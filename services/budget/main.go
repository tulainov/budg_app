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
	transactions := &transactionHandler{db: db}

	auth := func(h http.HandlerFunc) http.HandlerFunc { return requireAuth(publicKey, h) }

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	mux.HandleFunc("POST /categories", auth(categories.create))
	mux.HandleFunc("GET /categories", auth(categories.list))
	mux.HandleFunc("DELETE /categories/{id}", auth(categories.delete))

	mux.HandleFunc("POST /transactions", auth(transactions.create))
	mux.HandleFunc("GET /transactions", auth(transactions.list))
	mux.HandleFunc("GET /transactions/{id}", auth(transactions.get))
	mux.HandleFunc("PUT /transactions/{id}", auth(transactions.update))
	mux.HandleFunc("DELETE /transactions/{id}", auth(transactions.delete))

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
