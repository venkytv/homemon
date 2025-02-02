package backend

import (
	"context"
	"encoding/json"

	"github.com/nats-io/nats.go"
)

type RawMetric struct {
	Name     string  `json:"name"`
	DeviceID string  `json:"device_id"`
	Location string  `json:"location"`
	Value    float64 `json:"value"`
}

// RawPublisher publishes raw metrics to NATS
type RawPublisher struct {
	natsClient *nats.Conn
}

// NewNATSPublisher creates a new NATSPublisher
func NewNATSPublisher(address string) (*RawPublisher, error) {
	natsClient, err := nats.Connect(address)
	if err != nil {
		return nil, err
	}
	return &RawPublisher{
		natsClient: natsClient,
	}, nil
}

// Publish publishes the data to the backend
func (p *RawPublisher) Publish(ctx context.Context, metric RawMetric) error {
	data, err := json.Marshal(metric)
	if err != nil {
		return err
	}

	return p.natsClient.Publish(metric.Name, data)
}
