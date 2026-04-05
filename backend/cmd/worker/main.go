package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"

	"github.com/mattcarp12/mdq/internal/db"
	"github.com/mattcarp12/mdq/internal/metrics"
	"github.com/mattcarp12/mdq/internal/repository" // Ensure this matches your module
	"github.com/mattcarp12/mdq/internal/telemetry"
	"github.com/mattcarp12/mdq/internal/worker"
)

func main() {
	// 1. Setup Logging & Chaos Seeding
	env := os.Getenv("ENV")
	var logger *slog.Logger
	if env == "production" {
		logger = slog.New(slog.NewJSONHandler(os.Stdout, nil))
	} else {
		logger = slog.New(slog.NewTextHandler(os.Stdout, nil))
	}
	slog.SetDefault(logger)

	// Register all Prometheus metrics before the processor starts.
	metrics.Register()

	slog.Info("Booting up MDQ Task Worker...", slog.String("env", env))

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://devuser:devpassword@localhost:5432/taskqueue?sslmode=disable"
	}
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		redisURL = "redis://localhost:6379/0"
	}
	jaegerUrl := os.Getenv("JAEGER_URL")
	if jaegerUrl == "" {
		jaegerUrl = "localhost:4318"
	}

	// Inside main():
	tp, err := telemetry.InitProvider("mdq-worker", "localhost:4318")
	if err != nil {
		slog.Error("Failed to initialize telemetry", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer func() {
		if err := tp.Shutdown(context.Background()); err != nil {
			slog.Error("Error shutting down tracer provider", slog.String("error", err.Error()))
		}
	}()

	// 2. Connect to Database (using pgxpool)
	ctx := context.Background()
	dbPool, err := db.NewPostgresPool(ctx, dbURL)
	if err != nil {
		slog.Error("failed to connect to database", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer dbPool.Close()

	jobRepo := repository.NewPostgresJobRepository(dbPool)

	// 3. Connect to Redis
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		slog.Error("invalid REDIS_URL", slog.String("error", err.Error()))
		os.Exit(1)
	}
	redisClient := redis.NewClient(opts)
	defer redisClient.Close()

	// 4. Identify this specific container instance
	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "unknown-worker"
	}

	// 5. Initialize the Processor
	// IMPORTANT: The streamName "mdq:jobs" must exactly match the string used in QueueProducer.Enqueue!
	processor := worker.NewProcessor(
		redisClient,
		jobRepo,
		"mdq:jobs",    // The stream key
		"mdq-workers", // The Consumer Group
		hostname,      // The unique ID for this instance
	)

	// 6. Start the Metrics HTTP Server for Prometheus
	metricsMux := http.NewServeMux()
	// Expose the default Prometheus registry
	metricsMux.Handle("/metrics", promhttp.Handler())

	metricsServer := &http.Server{
		Addr:    "0.0.0.0:8081",
		Handler: metricsMux,
	}

	// Run the HTTP server in its own background goroutine
	go func() {
		slog.Info("Starting Worker Metrics server...", slog.String("port", "8081"))
		if err := metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("Metrics server crashed", slog.String("error", err.Error()))
		}
	}()

	// 6. Graceful Shutdown Management
	stopCtx, cancel := context.WithCancel(ctx)

	// Start the infinite processing loop in a background goroutine
	go processor.Start(stopCtx)

	// Listen for OS Interrupts (e.g., Ctrl+C or AWS Fargate SIGTERM)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	sig := <-quit
	slog.Info("Received shutdown signal", slog.String("signal", sig.String()))

	// Cancel the context. This breaks the XReadGroup block in the processor loop.
	cancel()

	// Shut down the metrics HTTP server gracefully too
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	metricsServer.Shutdown(shutdownCtx)

	// Give the worker 5 seconds to finish its current handler before forcefully exiting.
	slog.Info("Waiting 5 seconds for graceful shutdown...")
	time.Sleep(5 * time.Second)
	slog.Info("Worker shut down successfully.")
}
