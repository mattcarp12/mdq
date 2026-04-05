package main

import (
	"context"
	"flag"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/mattcarp12/mdq/internal/api"
	"github.com/mattcarp12/mdq/internal/db"
	"github.com/mattcarp12/mdq/internal/metrics"
	"github.com/mattcarp12/mdq/internal/queue"
	"github.com/mattcarp12/mdq/internal/repository"
	"github.com/mattcarp12/mdq/internal/telemetry"
)

func main() {
	// 1. Parse CLI Flags
	runMigrations := flag.Bool("migrate", false, "Run database migrations and exit")
	flag.Parse()

	// 2. Load Environment Variables
	env := os.Getenv("ENV")
	if env == "" {
		env = "local"
	}
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://devuser:devpassword@localhost:5432/taskqueue?sslmode=disable"
	}
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		redisURL = "redis://localhost:6379/0"
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		if env == "local" {
			jwtSecret = "devsecretkey"
		} else {
			log.Fatal("FATAL: JWT_SECRET environment variable is required")
		}
	}
	allowedOriginsStr := os.Getenv("ALLOWED_ORIGINS") // Comma-separated list of allowed origins for CORS
	allowedOrigins := []string{}
	if allowedOriginsStr != "" {
		allowedOrigins = strings.Split(allowedOriginsStr, ",")
	}
	jaegerUrl := os.Getenv("JAEGER_URL")
	if jaegerUrl == "" {
		jaegerUrl = "localhost:4318"
	}

	var logger *slog.Logger
	if env == "production" {
		logger = slog.New(slog.NewJSONHandler(os.Stdout, nil))
	} else {
		logger = slog.New(slog.NewTextHandler(os.Stdout, nil))
	}

	slog.SetDefault(logger)

	// Initialize OpenTelemetry
	tp, err := telemetry.InitProvider("mdq-api", jaegerUrl)
	if err != nil {
		slog.Error("Failed to initialize telemetry", slog.String("error", err.Error()))
		os.Exit(1)
	}
	// Ensure all buffered spans are flushed before the application exits
	defer func() {
		if err := tp.Shutdown(context.Background()); err != nil {
			slog.Error("Error shutting down tracer provider", slog.String("error", err.Error()))
		}
	}()

	slog.Info("Starting MDQ API server...", slog.String("port", port), slog.String("env", env))

	// Register Prometheus Metrics
	metrics.Register()

	// 3. Handle Migrations
	if *runMigrations {
		if err := db.RunMigrations(dbURL); err != nil {
			log.Fatalf("Migration failed: %v", err)
		}
		os.Exit(0)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 4. Initialize Infrastructure Connections
	pgPool, err := db.NewPostgresPool(ctx, dbURL)
	if err != nil {
		log.Fatalf("Failed to connect to Postgres: %v", err)
	}
	defer pgPool.Close()

	redisClient, err := db.NewRedisClient(ctx, redisURL)
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	defer redisClient.Close()

	// 5. Inject Dependencies
	jobRepo := repository.NewPostgresJobRepository(pgPool)
	userRepo := repository.NewPostgresUserRepository(pgPool)
	producer := queue.NewRedisProducer(redisClient, "mdq:jobs")

	server := api.NewServer(jobRepo, userRepo, producer, []byte(jwtSecret), allowedOrigins)

	// 6. Setup Routing
	mux := http.NewServeMux()
	server.RegisterRoutes(mux)

	httpServer := &http.Server{
		Addr:    "0.0.0.0:" + port,
		Handler: server.SetupHandler(),
	}

	// 7. Start Server
	go func() {
		slog.Info("MDQ API Server starting on port %s...", slog.String("address", httpServer.Addr))
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("Server failed", slog.Any("error", err))
		}
	}()

	// 8. Graceful Shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down gracefully...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}
	log.Println("Server exited properly")
}
