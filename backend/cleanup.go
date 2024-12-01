package backend

import (
	"context"
	"log/slog"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// Pick metrics from the backend where the TTL has expired and remove them
// from the priority and colour sets and the TTL sorted set itself.
func CleanupMetrics(ctx context.Context, config *Config, dryRun bool) error {
	p := config.Publisher

	// Get the current timestamp
	now := time.Now().Unix()

	ttl_key := p.prefix + ":ttl"

	// Get the metrics that have expired
	metrics, err := p.redisClient.ZRangeByScore(ctx, ttl_key, &redis.ZRangeBy{
		Min:    "-inf",
		Max:    strconv.FormatInt(now, 10),
		Offset: 0,
		Count:  -1,
	}).Result()

	if err != nil {
		slog.Error("Failed to get metrics to cleanup", "error", err)
		return err
	}

	slog.Debug("Metrics to cleanup", "metrics", metrics)
	if dryRun {
		slog.Info("Dry run, not removing metrics")
		return nil
	}

	// Remove the metrics from the priority sorted set and colour hash map
	for _, metric := range metrics {
		priority_key := p.prefix + ":priority"
		if err := p.redisClient.ZRem(ctx, priority_key, metric).Err(); err != nil {
			slog.Error("Failed to remove metric from priority", "metric", metric, "error", err)
			return err
		}

		colour_key := p.prefix + ":colour"
		if err := p.redisClient.HDel(ctx, colour_key, metric).Err(); err != nil {
			slog.Error("Failed to remove metric from colour", "metric", metric, "error", err)
			return err
		}

		if err := p.redisClient.ZRem(ctx, ttl_key, metric).Err(); err != nil {
			slog.Error("Failed to remove metric from TTL", "metric", metric, "error", err)
			return err
		}
	}

	return nil
}
