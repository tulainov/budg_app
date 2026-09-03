package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func mustGetenv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("missing required env var %s", key)
	}
	return v
}

func main() {
	port := getenv("PORT", "8080")
	databaseURL := mustGetenv("DATABASE_URL")
	privateKeyPath := mustGetenv("JWT_PRIVATE_KEY_PATH")

	ctx := context.Background()

	db, err := connectDB(ctx, databaseURL)
	if err != nil {
		log.Fatalf("db setup failed: %v", err)
	}
	defer db.Close()

	privateKey, err := loadPrivateKey(privateKeyPath)
	if err != nil {
		log.Fatalf("load private key failed: %v", err)
	}

	s := &server{db: db, privateKey: privateKey}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("POST /signup", instrument("/signup", s.handleSignup))
	mux.HandleFunc("POST /login", instrument("/login", s.handleLogin))
	mux.Handle("GET /metrics", promhttp.Handler())

	httpServer := &http.Server{Addr: ":" + port, Handler: mux}

	ctx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Printf("auth service listening on :%s", port)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	}()

	<-ctx.Done()
	log.Print("shutting down auth service")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("graceful shutdown failed: %v", err)
	}
}
