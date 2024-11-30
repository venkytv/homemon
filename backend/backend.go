package backend

import (
	"github.com/go-resty/resty/v2"
	"github.com/redis/go-redis/v9"
)

type Config struct {
	ConfigDir   string
	RestyClient *resty.Client
	RedisClient *redis.Client
	Publisher   *Publisher
}
