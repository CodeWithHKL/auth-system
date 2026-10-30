// Package handler is the Vercel Function entry point. vercel.json rewrites
// every /api/* request here; the original path is preserved, so the shared mux
// routes it as usual.
package handler

import (
	"context"
	"log"
	"net/http"
	"sync"
	"time"

	"myapp/internal/app"
	"myapp/internal/config"
	"myapp/internal/store"
)

var (
	mu      sync.Mutex
	handler http.Handler
)

// get builds the app on first use and reuses it (and its connection pool) for
// later requests on the same function instance. A failed start is retried on
// the next request rather than cached.
func get() (http.Handler, error) {
	mu.Lock()
	defer mu.Unlock()
	if handler != nil {
		return handler, nil
	}
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	// Not the request context: the pool outlives this request.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	db, err := store.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, err
	}
	handler = app.NewHandler(cfg, db)
	return handler, nil
}

func Handler(w http.ResponseWriter, r *http.Request) {
	h, err := get()
	if err != nil {
		log.Printf("startup failed: %v", err)
		http.Error(w, "service unavailable", http.StatusServiceUnavailable)
		return
	}
	h.ServeHTTP(w, r)
}
