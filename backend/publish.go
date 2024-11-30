package backend

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

type Metric struct {
	Name     string
	Priority int
	Colour   string
	TTL      time.Time
}

// Metric generator closure
func MetricGenerator(name string, ttl time.Duration) func(int, string) Metric {
	return func(priority int, colour string) Metric {
		return Metric{
			Name:     name,
			Priority: priority,
			Colour:   colour,
			TTL:      time.Now().Add(ttl),
		}
	}
}

// Publish publishes the data to the backend
type Publisher struct {
	redisClient *redis.Client
	prefix      string
}

// NewPublisher creates a new Publisher
func NewPublisher(address, prefix string) *Publisher {
	redisClient := redis.NewClient(&redis.Options{
		Addr: address,
	})
	return &Publisher{
		redisClient: redisClient,
		prefix:      prefix,
	}
}

// Publish publishes the data to the backend
func (p *Publisher) Publish(ctx context.Context, metric Metric) error {
	// Push priority to a sorted set
	priority_key := p.prefix + ":priority"
	if err := p.redisClient.ZAdd(ctx, priority_key, redis.Z{Score: float64(metric.Priority), Member: metric.Name}).Err(); err != nil {
		return err
	}

	// Push colour to a hash
	colour_key := p.prefix + ":colour"
	if err := p.redisClient.HSet(ctx, colour_key, metric.Name, metric.Colour).Err(); err != nil {
		return err
	}

	// Push TTL to a sorted set
	ttl_key := p.prefix + ":ttl"
	if err := p.redisClient.ZAdd(ctx, ttl_key, redis.Z{Score: float64(metric.TTL.Unix()), Member: metric.Name}).Err(); err != nil {
		return err
	}

	return nil
}
