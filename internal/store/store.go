package store

import (
	"context"
	"errors"
	"time"
)

var (
	ErrEmailTaken = errors.New("email already registered")
	ErrNotFound   = errors.New("user not found")
)

type User struct {
	ID           int64     `json:"id"`
	Name         string    `json:"name"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"createdAt"`
}

// Users is the persistence boundary for accounts. Emails are matched
// case-insensitively.
type Users interface {
	Create(ctx context.Context, name, email, passwordHash string) (User, error)
	ByEmail(ctx context.Context, email string) (User, error)
	ByID(ctx context.Context, id int64) (User, error)
}

// Limit says a key may fail at most Max times per window.
type Limit struct {
	Key string
	Max int
}

// LoginLimiter counts failed logins across all server instances. Keys are
// opaque strings chosen by the caller.
type LoginLimiter interface {
	// RetryAfter returns how long to wait if any limit has been reached, else 0.
	RetryAfter(ctx context.Context, window time.Duration, limits []Limit) (time.Duration, error)
	RecordFailure(ctx context.Context, window time.Duration, keys ...string) error
	Clear(ctx context.Context, key string) error
}
