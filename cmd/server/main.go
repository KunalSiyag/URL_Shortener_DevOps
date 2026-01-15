package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	"url_shortener/internal/handler"
	"url_shortener/internal/store"
)

func main() {
	// 1. Get port from environment variable, default to 8080
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "redis:6379"
	}
	// 2. Initialize the in-memory store (this holds our shortened URLs)
	urlStore := store.NewRedisStore(redisAddr)
	// urlStore := store.NewInMemoryStore()
	// 3. Create a new HTTP request multiplexer (router)
	mux := http.NewServeMux()

	// 4. Register route handlers
	// GET /health - health check endpoint for Kubernetes probes
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})
	// POST /shorten - creates a short URL from a long URL
	mux.HandleFunc("/shorten", handler.ShortenHandler(urlStore))
	// GET /{shortCode} - redirects to the original URL
	mux.HandleFunc("/", handler.RedirectHandler(urlStore))

	// 5. Configure the HTTP server
	server := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}

	// 6. Start server in a goroutine (non-blocking)
	go func() {
		log.Printf("Starting server on :%s\n", port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
		}
	}()

	// 7. Graceful shutdown setup
	// Create a channel to receive OS signals
	quit := make(chan os.Signal, 1)
	// Notify this channel when SIGINT (Ctrl+C) or SIGTERM is received
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// 8. Block until we receive a shutdown signal
	<-quit
	log.Println("Shutting down server...")

	// 9. Create a deadline for the shutdown (5 seconds max)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 10. Attempt graceful shutdown
	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exiting")
}
