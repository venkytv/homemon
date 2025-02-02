package backend

import (
	"github.com/go-resty/resty/v2"
	"github.com/redis/go-redis/v9"
)

type Config struct {
	ConfigDir    string
	RestyClient  *resty.Client
	RedisClient  *redis.Client
	Publisher    *Publisher
	RawPublisher *RawPublisher
}

type Range struct {
	From     float64 `koanf:"from"`
	To       float64 `koanf:"to"`
	Priority int     `koanf:"priority"`
	Colour   string  `koanf:"colour"`
}
