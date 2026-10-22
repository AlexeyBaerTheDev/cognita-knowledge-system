package queue

import (
	"fmt"
	"os"

	"github.com/redis/go-redis/v9"
)

func NewClient() (*redis.Client, error) {
	raw := os.Getenv("REDIS_URL")
	if raw == "" {
		return nil, fmt.Errorf("REDIS_URL is not set")
	}

	opt, err := redis.ParseURL(raw)
	if err != nil {
		return nil, fmt.Errorf("parse REDIS_URL: %w", err)
	}

	return redis.NewClient(opt), nil
}
