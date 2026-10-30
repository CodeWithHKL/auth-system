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
