// Package app builds the HTTP handler shared by the standalone server and the
// Vercel function.
package app

import (
	"net/http"

	"myapp/internal/auth"
	"myapp/internal/config"
	"myapp/internal/store"
)

func NewHandler(cfg config.Config, db *store.Postgres) http.Handler {
	mux := http.NewServeMux()
	// Readiness probe for load balancers / orchestrators.
	mux.HandleFunc("GET /api/healthz", func(w http.ResponseWriter, r *http.Request) {
		if err := db.Ping(r.Context()); err != nil {
			http.Error(w, "database unavailable", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	tokens := auth.NewTokens(cfg.JWTSecret, cfg.TokenTTL)
	auth.New(db, tokens, cfg.SecureCookie, int(cfg.TokenTTL.Seconds())).Register(mux)
	return mux
}
