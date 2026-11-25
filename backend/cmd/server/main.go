package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/rafael/monitor-de-rede-backend/internal/api"
	"github.com/rafael/monitor-de-rede-backend/internal/scheduler"
	"github.com/rafael/monitor-de-rede-backend/internal/storage"
	"github.com/rafael/monitor-de-rede-backend/internal/ws"
)

func main() {
	port := getEnv("PORT", "8080")
	dbPath := getEnv("DB_PATH", "./data/netpulse.db")

	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		log.Fatalf("failed to create data directory: %v", err)
	}

	store, err := storage.New(dbPath)
	if err != nil {
		log.Fatalf("failed to initialize database: %v", err)
	}
	defer store.Close()

	if err := store.SeedDefaultsIfEmpty(); err != nil {
		log.Printf("warning: failed seeding default monitors: %v", err)
	}

	hub := ws.NewHub()
	sched := scheduler.New(store, hub)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := sched.Start(ctx); err != nil {
		log.Fatalf("failed to start scheduler: %v", err)
	}

	handler := api.NewServer(store, sched, hub)
	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf("NetPulse daemon listening on :%s (db: %s)", port, dbPath)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("shutting down gracefully...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
