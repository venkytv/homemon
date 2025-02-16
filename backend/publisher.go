package backend

import (
	"context"
	"log"
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

// List metrics
func (p *Publisher) ListMetrics(ctx context.Context, config *Config) ([]Metric, error) {
	// Get all metrics
	priority_key := p.prefix + ":priority"
	colour_key := p.prefix + ":colour"
	ttl_key := p.prefix + ":ttl"
	metrics := []Metric{}

	// Get all members with scores ordered by priority in reverse order
	members, err := p.redisClient.ZRevRangeByScoreWithScores(ctx, priority_key, &redis.ZRangeBy{
		Min: "-inf",
		Max: "+inf",
	}).Result()
	if err != nil {
		return nil, err
	}
	for _, member := range members {
		colour, err := p.redisClient.HGet(ctx, colour_key, member.Member.(string)).Result()
		if err != nil {
			return nil, err
		}
		priority := int(member.Score)
		ttl, err := p.redisClient.ZScore(ctx, ttl_key, member.Member.(string)).Result()
		if err != nil {
			return nil, err
		}
		metrics = append(metrics, Metric{
			Name:     member.Member.(string),
			Priority: priority,
			Colour:   colour,
			TTL:      time.Unix(int64(ttl), 0),
		})
	}

	return metrics, nil
}

// Delete metric
func (p *Publisher) DeleteMetric(ctx context.Context, name string) error {
	// Ensure metric exists
	priority_key := p.prefix + ":priority"
	if _, err := p.redisClient.ZRank(ctx, priority_key, name).Result(); err != nil {
		// Metric does not exist
		log.Fatalf("Metric %s does not exist", name)
	}

	// Delete priority
	if err := p.redisClient.ZRem(ctx, priority_key, name).Err(); err != nil {
		return err
	}

	// Delete colour
	colour_key := p.prefix + ":colour"
	if err := p.redisClient.HDel(ctx, colour_key, name).Err(); err != nil {
		return err
	}

	// Delete TTL
	ttl_key := p.prefix + ":ttl"
	if err := p.redisClient.ZRem(ctx, ttl_key, name).Err(); err != nil {
		return err
	}

	return nil
}
