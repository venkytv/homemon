package netatmo

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"

	"github.com/venkytv/homemon/backend"
)

const (
	NetatmoRefreshTokenFile = "netatmo-refresh-token"
	NetatmoConfigFile       = "netatmo-config.yaml"
	DeviceID                = "netatmo"
)

type RefreshTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
}

type NetatmoHomeCoachData struct {
	Body struct {
		Devices []struct {
			DashboardData struct {
				Temperature float64 `json:"Temperature"`
				CO2         int     `json:"CO2"`
				Humidity    int     `json:"Humidity"`
				Noise       int     `json:"Noise"`
				Pressure    float64 `json:"Pressure"`
			} `json:"dashboard_data"`
		} `json:"devices"`
	} `json:"body"`
}

func RecordMetrics(ctx context.Context, config *backend.Config) {
	// Get the access token
	refreshTokenFile := path.Join(config.ConfigDir, NetatmoRefreshTokenFile)
	accessToken, expiresIn, err := getAccessToken(ctx, config.RestyClient, refreshTokenFile)
	if err != nil {
		slog.Error("Error getting access token", "error", err)
		os.Exit(1)
	}
	slog.Debug("Access Token", "expiresIn", expiresIn)

	// Load mac IDs
	configFile := path.Join(config.ConfigDir, NetatmoConfigFile)
	var k = koanf.New(".")

	if err := k.Load(file.Provider(configFile), yaml.Parser()); err != nil {
		slog.Error("Error loading config file", "error", err)
		os.Exit(1)
	}

	// Record the initial metrics
	recordMetricsRoutine(ctx, config, k, accessToken)

	// Start the ticker to record metrics
	metricsTicker := time.NewTicker(3 * time.Minute)

	// Start the ticker to refresh the access token
	accessTokenTicker := time.NewTicker(time.Duration(expiresIn-1800) * time.Second)

	// Start the ticker to run the cleanup routine
	cleanupTicker := time.NewTicker(15 * time.Second)

	for {
		select {
		case <-metricsTicker.C:
			slog.Debug("Recording metrics")
			recordMetricsRoutine(ctx, config, k, accessToken)
		case <-accessTokenTicker.C:
			slog.Info("Refreshing access token")
			accessToken, expiresIn, err = getAccessToken(ctx, config.RestyClient, refreshTokenFile)
			if err != nil {
				slog.Error("Error getting access token", "error", err)
				os.Exit(1)
			}
		case <-cleanupTicker.C:
			slog.Debug("Cleaning up metrics")
			backend.CleanupMetrics(ctx, config, false)
		}
	}
}

func recordMetricsRoutine(ctx context.Context, config *backend.Config, k *koanf.Koanf, accessToken string) {
	// Load mac IDs
	macIdMap := k.MustStringMap("mac-ids")

	// Load metric ranges
	var humitidyRanges, temperatureRanges, co2Ranges, noiseRanges []backend.Range
	k.Unmarshal("metrics.humidity", &humitidyRanges)
	k.Unmarshal("metrics.temperature", &temperatureRanges)
	k.Unmarshal("metrics.co2", &co2Ranges)
	k.Unmarshal("metrics.noise", &noiseRanges)

	// Build metrics generators for each room
	var humidityMetricGeneratorMap = make(map[string]func(int, string) backend.Metric)
	var temperatureMetricGeneratorMap = make(map[string]func(int, string) backend.Metric)
	var co2MetricGeneratorMap = make(map[string]func(int, string) backend.Metric)
	var noiseMetricGeneratorMap = make(map[string]func(int, string) backend.Metric)

	for room, _ := range macIdMap {
		humidityMetricGeneratorMap[room] = backend.MetricGenerator("humidity:"+room, 5*time.Minute)
		temperatureMetricGeneratorMap[room] = backend.MetricGenerator("temperature:"+room, 5*time.Minute)
		co2MetricGeneratorMap[room] = backend.MetricGenerator("co2:"+room, 5*time.Minute)
		noiseMetricGeneratorMap[room] = backend.MetricGenerator("noise:"+room, 5*time.Minute)
	}

	for room, mac_id := range macIdMap {

		homeCoachData := NetatmoHomeCoachData{}
		resp, err := config.RestyClient.R().
			SetContext(ctx).
			EnableGenerateCurlOnDebug().
			SetHeader("Authorization", "Bearer "+accessToken).
			SetQueryParam("device_id", mac_id).
			SetResult(&homeCoachData).
			Get("https://api.netatmo.com/api/gethomecoachsdata")
		if err != nil {
			slog.Error("Error getting home coach data", "error", err)
			os.Exit(1)
		}
		if resp.IsError() {
			slog.Error("Error getting home coach data", "error", resp.Status()+" "+string(resp.Body()))
			os.Exit(1)
		}
		slog.Debug("Home Coach Data", "data", homeCoachData)

		dashboardData := homeCoachData.Body.Devices[0].DashboardData

		// Publish raw metrics
		rawMetrics := []backend.RawMetric{
			{
				Name:     "sensor.environmental.temperature",
				DeviceID: DeviceID,
				Location: room,
				Value:    dashboardData.Temperature,
			},
			{
				Name:     "sensor.environmental.humidity",
				DeviceID: DeviceID,
				Location: room,
				Value:    float64(dashboardData.Humidity),
			},
			{
				Name:     "sensor.environmental.co2",
				DeviceID: DeviceID,
				Location: room,
				Value:    float64(dashboardData.CO2),
			},
			{
				Name:     "sensor.acoustic.noise",
				DeviceID: DeviceID,
				Location: room,
				Value:    float64(dashboardData.Noise),
			},
		}

		for _, rawMetric := range rawMetrics {
			slog.Info("Publishing raw metric", "metric", rawMetric)
			err = config.RawPublisher.Publish(ctx, rawMetric)
			if err != nil {
				slog.Error("Error publishing raw metric", "error", err)
			}
		}

		// Generate metrics

		// Humidity
		for _, metricRange := range humitidyRanges {
			if float64(dashboardData.Humidity) >= metricRange.From && float64(dashboardData.Humidity) < metricRange.To {
				humidityMetric := humidityMetricGeneratorMap[room](metricRange.Priority, metricRange.Colour)
				slog.Info("Publishing metric", "humidity", humidityMetric, "current", dashboardData.Humidity)
				err = config.Publisher.Publish(ctx, humidityMetric)
				if err != nil {
					slog.Error("Error publishing metric", "error", err)
				}
				break
			}
		}

		// Temperature
		for _, metricRange := range temperatureRanges {
			if float64(dashboardData.Temperature) >= metricRange.From && float64(dashboardData.Temperature) < metricRange.To {
				temperatureMetric := temperatureMetricGeneratorMap[room](metricRange.Priority, metricRange.Colour)
				slog.Info("Publishing metric", "temperature", temperatureMetric, "current", dashboardData.Temperature)
				err = config.Publisher.Publish(ctx, temperatureMetric)
				if err != nil {
					slog.Error("Error publishing metric", "error", err)
				}
				break
			}
		}

		// CO2
		for _, metricRange := range co2Ranges {
			if float64(dashboardData.CO2) >= metricRange.From && float64(dashboardData.CO2) < metricRange.To {
				co2Metric := co2MetricGeneratorMap[room](metricRange.Priority, metricRange.Colour)
				slog.Info("Publishing metric", "co2", co2Metric, "current", dashboardData.CO2)
				err = config.Publisher.Publish(ctx, co2Metric)
				if err != nil {
					slog.Error("Error publishing metric", "error", err)
				}
				break
			}
		}

		// Noise
		for _, metricRange := range noiseRanges {
			if float64(dashboardData.Noise) >= metricRange.From && float64(dashboardData.Noise) < metricRange.To {
				noiseMetric := noiseMetricGeneratorMap[room](metricRange.Priority, metricRange.Colour)
				slog.Info("Publishing metric", "noise", noiseMetric, "current", dashboardData.Noise)
				err = config.Publisher.Publish(ctx, noiseMetric)
				if err != nil {
					slog.Error("Error publishing metric", "error", err)
				}
				break
			}
		}
	}
}

// Get a new access token using the refresh token in file
func getAccessToken(ctx context.Context, client *resty.Client, refreshTokenFile string) (string, int, error) {
	refreshToken, err := readRefreshTokenFromFile(refreshTokenFile)
	if err != nil {
		return "", 0, err
	}
	refreshTokenResponse, err := refreshAccessToken(ctx, client, refreshToken)
	if err != nil {
		return "", 0, err
	}

	// Write the new refresh token to file
	err = os.WriteFile(refreshTokenFile, []byte(refreshTokenResponse.RefreshToken), 0600)
	if err != nil {
		return "", 0, err
	}
	return refreshTokenResponse.AccessToken, refreshTokenResponse.ExpiresIn, nil
}

// Read the refresh token from file
func readRefreshTokenFromFile(refreshTokenFile string) (string, error) {
	// Read the refresh token from file
	refreshToken, err := os.ReadFile(refreshTokenFile)
	if err != nil {
		return "", err
	}
	// Trim the trailing newline
	newRefreshToken := string(refreshToken)
	return strings.TrimSuffix(newRefreshToken, "\n"), nil

}

// Read client ID and secret from environment variables
func readClientIDAndSecretFromEnv() (string, string, error) {
	clientID := os.Getenv("NETATMO_CLIENT_ID")
	clientSecret := os.Getenv("NETATMO_CLIENT_SECRET")
	if clientID == "" || clientSecret == "" {
		return "", "", fmt.Errorf("NETATMO_CLIENT_ID or NETATMO_CLIENT_SECRET is not set")
	}
	return clientID, clientSecret, nil
}

func refreshAccessToken(ctx context.Context, client *resty.Client, refreshToken string) (RefreshTokenResponse, error) {
	// Get a new access token using the refresh token

	refreshTokenResponse := RefreshTokenResponse{}

	clientID, clientSecret, err := readClientIDAndSecretFromEnv()
	if err != nil {
		return refreshTokenResponse, err
	}

	resp, err := client.R().
		SetContext(ctx).
		EnableGenerateCurlOnDebug().
		SetFormData(map[string]string{
			"grant_type":    "refresh_token",
			"refresh_token": refreshToken,
			"client_id":     clientID,
			"client_secret": clientSecret,
		}).
		SetResult(&refreshTokenResponse).
		Post("https://api.netatmo.com/oauth2/token")
	if err != nil {
		return refreshTokenResponse, err
	}
	if resp.IsError() {
		return refreshTokenResponse, fmt.Errorf("error: %s %s", resp.Status(), string(resp.Body()))
	}

	return refreshTokenResponse, nil
}
