package main

import (
	"context"
	_ "embed"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed schema.sql
var schemaSQL string

// connectDB retries the initial ping instead of failing on the first
// attempt: in Kubernetes, this pod's Deployment can be applied and start
// before Postgres's readiness probe passes, since kubectl doesn't order
// resource types by dependency.
func connectDB(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("connect db: %w", err)
	}

	// 40 attempts * 3s = up to 2 minutes: generous enough to cover a cold
	// `postgres:16-alpine` image pull on a brand new node (our own images
	// are pre-loaded via `kind load docker-image`, but Postgres's is not).
	const maxAttempts = 40
	var pingErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if pingErr = pool.Ping(ctx); pingErr == nil {
			break
		}
		log.Printf("waiting for database (attempt %d/%d): %v", attempt, maxAttempts, pingErr)
		time.Sleep(3 * time.Second)
	}
	if pingErr != nil {
		return nil, fmt.Errorf("ping db: %w", pingErr)
	}

	if _, err := pool.Exec(ctx, schemaSQL); err != nil {
		return nil, fmt.Errorf("apply schema: %w", err)
	}
	return pool, nil
}
