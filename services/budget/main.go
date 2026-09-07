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
	publicKeyPath := mustGetenv("JWT_PUBLIC_KEY_PATH")
	authBaseURL := mustGetenv("AUTH_BASE_URL")

	ctx := context.Background()

	db, err := connectDB(ctx, databaseURL)
	if err != nil {
		log.Fatalf("db setup failed: %v", err)
	}
	defer db.Close()

	publicKey, err := loadPublicKey(publicKeyPath)
	if err != nil {
		log.Fatalf("load public key failed: %v", err)
	}

	categories := &categoryHandler{db: db}
	transactions := &transactionHandler{db: db, auth: newAuthClient(authBaseURL)}

	auth := func(h http.HandlerFunc) http.HandlerFunc { return requireAuth(publicKey, h) }

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	mux.HandleFunc("POST /categories", instrument("/categories", auth(categories.create)))
	mux.HandleFunc("GET /categories", instrument("/categories", auth(categories.list)))
	mux.HandleFunc("DELETE /categories/{id}", instrument("/categories/{id}", auth(categories.delete)))

	mux.HandleFunc("POST /transactions", instrument("/transactions", auth(transactions.create)))
	mux.HandleFunc("GET /transactions", instrument("/transactions", auth(transactions.list)))
	mux.HandleFunc("GET /transactions/{id}", instrument("/transactions/{id}", auth(transactions.get)))
	mux.HandleFunc("PUT /transactions/{id}", instrument("/transactions/{id}", auth(transactions.update)))
	mux.HandleFunc("DELETE /transactions/{id}", instrument("/transactions/{id}", auth(transactions.delete)))

	mux.Handle("GET /metrics", promhttp.Handler())

	httpServer := &http.Server{Addr: ":" + port, Handler: mux}

	ctx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Printf("budget service listening on :%s", port)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	}()

	<-ctx.Done()
	log.Print("shutting down budget service")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("graceful shutdown failed: %v", err)
	}
}
