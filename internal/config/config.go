package config

import (
	"errors"
	"os"
	"time"
)

type Config struct {
	Addr         string
	DatabaseURL  string
	JWTSecret    []byte
	TokenTTL     time.Duration
	SecureCookie bool
}

// Load reads configuration from the environment. Every instance of the
// server reads the same variables, so running more replicas behind a load
// balancer needs no extra coordination.
func Load() (Config, error) {
	c := Config{
		Addr:         getenv("ADDR", ":8080"),
		DatabaseURL:  os.Getenv("DATABASE_URL"),
		JWTSecret:    []byte(os.Getenv("JWT_SECRET")),
		TokenTTL:     24 * time.Hour,
		SecureCookie: os.Getenv("COOKIE_SECURE") == "true",
	}
	if v := os.Getenv("TOKEN_TTL"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return c, errors.New("TOKEN_TTL must be a duration like 24h")
		}
		c.TokenTTL = d
	}
	if c.DatabaseURL == "" {
		return c, errors.New("DATABASE_URL is required")
	}
	if len(c.JWTSecret) < 32 {
		return c, errors.New("JWT_SECRET is required and must be at least 32 bytes")
	}
	return c, nil
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
