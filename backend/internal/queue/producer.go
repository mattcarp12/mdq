package queue

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
)

// Producer defines the interface for pushing jobs to our message broker.
type Producer interface {
	Enqueue(ctx context.Context, jobID string, jobType string) error
}

type redisProducer struct {
	client *redis.Client
	stream string
}

// NewRedisProducer creates a new Redis stream publisher.
func NewRedisProducer(client *redis.Client, stream string) Producer {
	return &redisProducer{
		client: client,
		stream: stream,
	}
}

// Enqueue pushes the minimal required data (the ID and Type) to the Stream.
// The workers will use the ID to fetch the full JSON payload from PostgreSQL.
func (r *redisProducer) Enqueue(ctx context.Context, jobID string, jobType string) error {
	err := r.client.XAdd(ctx, &redis.XAddArgs{
		Stream: r.stream,
		// We use standard map mapping for Redis field-value pairs
		Values: map[string]interface{}{
			"job_id": jobID,
			"type":   jobType,
		},
	}).Err()

	if err != nil {
		return fmt.Errorf("failed to enqueue job to redis stream: %w", err)
	}

	return nil
}