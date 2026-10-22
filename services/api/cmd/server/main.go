package main

import (
	"api/db"
	"api/document"
	"api/queue"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	ctx := context.Background()

	pool, err := db.NewPool(ctx)
	if err != nil {
		log.Fatalf("Error: %v", err)
	}
	defer pool.Close()

	rdb, err := queue.NewClient()
	if err != nil {
		log.Fatalf("redis: %v", err)
	}
	defer rdb.Close()

	port := os.Getenv("API_PORT")

	if port == "" {
		port = "8080"
	}

	mux := http.NewServeMux()

	repo := document.NewRepository(pool)
	svc := document.NewService(repo, rdb)
	document.RegisterRoutes(mux, svc)

	// Liveness
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-type", "text/plain")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, " ok")
	})

	// Readyness
	mux.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		if err := pool.Ping(r.Context()); err != nil {
			WriteJson(w, http.StatusServiceUnavailable, map[string]string{"status": "not ready", "postgres": err.Error()})
			return
		}

		if err := rdb.Ping(r.Context()).Err(); err != nil {
			WriteJson(w, http.StatusServiceUnavailable, map[string]string{"status": "not ready", "redis": err.Error()})
			return
		}

		WriteJson(w, http.StatusOK, map[string]string{"status": "ready"})
	})

	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Printf("API listening on %s", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Error: %v", err)
		}
	}()

	// Graceuful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	log.Println("Shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Error: %v", err)
	}
	log.Println("stopped")
}

func WriteJson(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
