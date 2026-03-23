package db

import (
	"context"
	"fmt"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"
)

// NewPostgresPool creates and returns a highly-concurrent connection pool.
// In robust, distributed systems, connection pooling is critical to prevent 
// database connection exhaustion under heavy load.
func NewPostgresPool(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse database URL: %w", err)
	}

	// SOTA Tip: You can tune config.MaxConns here based on your worker count
	// and API load later when deploying to AWS.

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Verify the connection is actually alive
	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("database ping failed: %w", err)
	}

	log.Println("Successfully connected to PostgreSQL pool")
	return pool, nil
}