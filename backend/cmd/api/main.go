package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mattcarp12/mdq/internal/api"
	"github.com/mattcarp12/mdq/internal/db"
	"github.com/mattcarp12/mdq/internal/queue"
	"github.com/mattcarp12/mdq/internal/repository"
)

func main() {
	// 1. Parse CLI Flags
	runMigrations := flag.Bool("migrate", false, "Run database migrations and exit")
	flag.Parse()

	// 2. Load Environment Variables
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
	producer := queue.NewRedisProducer(redisClient, "mdq:jobs")
	server := api.NewServer(jobRepo, producer)

	// 6. Setup Routing
	mux := http.NewServeMux()
	server.RegisterRoutes(mux)

	httpServer := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}

	// 7. Start Server
	go func() {
		log.Printf("MDQ API Server starting on port %s...", port)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
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