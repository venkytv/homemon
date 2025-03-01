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
	natsPrefix string
}

// NewNATSPublisher creates a new NATSPublisher
func NewNATSPublisher(address string, prefix string) (*RawPublisher, error) {
	natsClient, err := nats.Connect(address)
	if err != nil {
		return nil, err
	}
	if len(prefix) > 0 {
		prefix += "."
	}
	return &RawPublisher{
		natsClient: natsClient,
		natsPrefix: prefix,
	}, nil
}

// Publish publishes the data to the backend
func (p *RawPublisher) Publish(ctx context.Context, metric RawMetric) error {
	data, err := json.Marshal(metric)
	if err != nil {
		return err
	}

	name := p.natsPrefix + metric.Name
	return p.natsClient.Publish(name, data)
}
