// Package serverless exposes the app to the Vercel function in api/. Vercel
// compiles that function outside this module's path, so it cannot import
// internal packages directly; this package can, and re-exports what it needs.
package serverless

import (
	"context"
	"net/http"

	"myapp/internal/app"
	"myapp/internal/config"
	"myapp/internal/store"
)

// New loads configuration from the environment, connects to the database and
// returns the shared HTTP handler.
func New(ctx context.Context) (http.Handler, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	db, err := store.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, err
	}
	return app.NewHandler(cfg, db), nil
}
