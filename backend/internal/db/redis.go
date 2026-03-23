package db

import (
	"context"
	"fmt"
	"log"

	"github.com/redis/go-redis/v9"
)

// NewRedisClient initializes a connection pool to Redis.
func NewRedisClient(ctx context.Context, redisURL string) (*redis.Client, error) {
	// ParseURL handles standard connection strings like "redis://user:pass@localhost:6379/0"
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse redis URL: %w", err)
	}

	client := redis.NewClient(opts)

	// Ping to verify the connection
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis ping failed: %w", err)
	}

	log.Println("Successfully connected to Redis")
	return client, nil
}
