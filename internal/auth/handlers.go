package auth

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"net/mail"
	"strings"
	"unicode/utf8"

	"golang.org/x/crypto/bcrypt"

	"myapp/internal/store"
)

const (
	cookieName  = "session"
	minPassword = 8
	maxPassword = 72 // bcrypt ignores everything past 72 bytes
	maxName     = 80
	maxBody     = 1 << 20
)

// dummyHash lets login spend the same bcrypt time for unknown emails as for
// known ones, so response time doesn't reveal which emails are registered.
var dummyHash, _ = bcrypt.GenerateFromPassword([]byte("not-a-real-password"), bcrypt.DefaultCost)

type Handler struct {
	users  store.Users
	tokens *Tokens
	secure bool
	ttlSec int
}

func New(users store.Users, tokens *Tokens, secureCookie bool, ttlSeconds int) *Handler {
	return &Handler{users: users, tokens: tokens, secure: secureCookie, ttlSec: ttlSeconds}
}

// Register wires the auth routes onto mux (method+path patterns need Go 1.22+).
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/auth/signup", h.signup)
	mux.HandleFunc("POST /api/auth/login", h.login)
	mux.HandleFunc("POST /api/auth/logout", h.logout)
	mux.HandleFunc("GET /api/auth/me", h.me)
}

type credentials struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *Handler) signup(w http.ResponseWriter, r *http.Request) {
	var in credentials
	if !decode(w, r, &in) {
		return
	}
	in.Name = strings.TrimSpace(in.Name)
	in.Email = strings.TrimSpace(in.Email)

	if fields := validateSignup(in); len(fields) > 0 {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"error": "validation failed", "fields": fields})
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		serverError(w, err)
		return
	}
	u, err := h.users.Create(r.Context(), in.Name, in.Email, string(hash))
	if errors.Is(err, store.ErrEmailTaken) {
		writeJSON(w, http.StatusConflict, map[string]any{
			"error":  "email already registered",
			"fields": map[string]string{"email": "This email is already registered"},
		})
		return
	}
	if err != nil {
		serverError(w, err)
		return
	}
	if !h.startSession(w, u.ID) {
		return
	}
	writeJSON(w, http.StatusCreated, u)
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var in credentials
	if !decode(w, r, &in) {
		return
	}
	in.Email = strings.TrimSpace(in.Email)

	u, err := h.users.ByEmail(r.Context(), in.Email)
	hash := dummyHash
	if err == nil {
		hash = []byte(u.PasswordHash)
	} else if !errors.Is(err, store.ErrNotFound) {
		serverError(w, err)
		return
	}
	pwErr := bcrypt.CompareHashAndPassword(hash, []byte(in.Password))
	if err != nil || pwErr != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid email or password"})
		return
	}
	if !h.startSession(w, u.ID) {
		return
	}
	writeJSON(w, http.StatusOK, u)
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name: cookieName, Value: "", Path: "/", MaxAge: -1,
		HttpOnly: true, Secure: h.secure, SameSite: http.SameSiteLaxMode,
	})
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	u, ok := h.currentUser(r.Context(), r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "not signed in"})
		return
	}
	writeJSON(w, http.StatusOK, u)
}

func (h *Handler) currentUser(ctx context.Context, r *http.Request) (store.User, bool) {
	c, err := r.Cookie(cookieName)
	if err != nil {
		return store.User{}, false
	}
	id, err := h.tokens.Verify(c.Value)
	if err != nil {
		return store.User{}, false
	}
	u, err := h.users.ByID(ctx, id)
	return u, err == nil
}

func (h *Handler) startSession(w http.ResponseWriter, userID int64) bool {
	tok, err := h.tokens.Issue(userID)
	if err != nil {
		serverError(w, err)
		return false
	}
	http.SetCookie(w, &http.Cookie{
		Name: cookieName, Value: tok, Path: "/", MaxAge: h.ttlSec,
		HttpOnly: true, Secure: h.secure, SameSite: http.SameSiteLaxMode,
	})
	return true
}

func validateSignup(in credentials) map[string]string {
	fields := map[string]string{}
	if in.Name == "" {
		fields["name"] = "Name is required"
	} else if utf8.RuneCountInString(in.Name) > maxName {
		fields["name"] = "Name is too long"
	}
	if addr, err := mail.ParseAddress(in.Email); err != nil || addr.Address != in.Email {
		fields["email"] = "Enter a valid email address"
	}
	switch {
	case len(in.Password) < minPassword:
		fields["password"] = "Password must be at least 8 characters"
	case len(in.Password) > maxPassword:
		fields["password"] = "Password must be at most 72 bytes"
	}
	return fields
}

func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxBody)
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return false
	}
	return true
}

func serverError(w http.ResponseWriter, err error) {
	log.Printf("internal error: %v", err)
	writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
