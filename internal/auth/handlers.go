package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/mail"
	"strconv"
	"strings"
	"time"
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

	// Failed logins are counted per client IP and per email over loginWindow.
	// The IP limit is higher so one address (an office, a school) is not locked
	// out by a few typos; the email limit slows guessing against one account.
	loginWindow         = 15 * time.Minute
	maxFailuresPerIP    = 20
	maxFailuresPerEmail = 10
)

// dummyHash lets login spend the same bcrypt time for unknown emails as for
// known ones, so response time doesn't reveal which emails are registered.
var dummyHash, _ = bcrypt.GenerateFromPassword([]byte("not-a-real-password"), bcrypt.DefaultCost)

type Handler struct {
	users      store.Users
	tokens     *Tokens
	secure     bool
	ttlSec     int
	limiter    store.LoginLimiter // nil disables login rate limiting
	trustProxy bool
}

func New(users store.Users, tokens *Tokens, secureCookie bool, ttlSeconds int) *Handler {
	return &Handler{users: users, tokens: tokens, secure: secureCookie, ttlSec: ttlSeconds}
}

// WithLoginLimiter turns on login rate limiting. Set trustProxy only when the
// server sits behind a proxy that overwrites X-Real-IP / X-Forwarded-For (as
// Vercel does); otherwise clients could pick their own IP and dodge the limit.
func (h *Handler) WithLoginLimiter(l store.LoginLimiter, trustProxy bool) *Handler {
	h.limiter, h.trustProxy = l, trustProxy
	return h
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

	ipKey := "ip:" + clientIP(r, h.trustProxy)
	emailKey := "email:" + hashEmail(in.Email)
	if h.limiter != nil {
		// Checked before bcrypt so blocked guesses cost no CPU and, being
		// rejected here, are not counted again (the window does not extend).
		wait, err := h.limiter.RetryAfter(r.Context(), loginWindow, []store.Limit{
			{Key: ipKey, Max: maxFailuresPerIP},
			{Key: emailKey, Max: maxFailuresPerEmail},
		})
		if err != nil {
			serverError(w, err)
			return
		}
		if wait > 0 {
			tooManyAttempts(w, wait)
			return
		}
	}

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
		if h.limiter != nil {
			// Best effort: a counting error should not turn a 401 into a 500.
			if err := h.limiter.RecordFailure(r.Context(), loginWindow, ipKey, emailKey); err != nil {
				log.Printf("record login failure: %v", err)
			}
		}
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid email or password"})
		return
	}
	if h.limiter != nil {
		// A correct password resets that account's counter, not the IP's.
		if err := h.limiter.Clear(r.Context(), emailKey); err != nil {
			log.Printf("clear login failures: %v", err)
		}
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

func tooManyAttempts(w http.ResponseWriter, wait time.Duration) {
	mins := int((wait + time.Minute - 1) / time.Minute)
	w.Header().Set("Retry-After", strconv.Itoa(int((wait+time.Second-1)/time.Second)))
	unit := "minutes"
	if mins == 1 {
		unit = "minute"
	}
	writeJSON(w, http.StatusTooManyRequests, map[string]string{
		"error": fmt.Sprintf("Too many failed attempts. Try again in %d %s.", mins, unit),
	})
}

// hashEmail gives a fixed-size, case-insensitive key without storing the email.
func hashEmail(email string) string {
	sum := sha256.Sum256([]byte(strings.ToLower(email)))
	return hex.EncodeToString(sum[:])
}

func clientIP(r *http.Request, trustProxy bool) string {
	if trustProxy {
		if v := strings.TrimSpace(r.Header.Get("X-Real-IP")); v != "" {
			return v
		}
		if first, _, _ := strings.Cut(r.Header.Get("X-Forwarded-For"), ","); strings.TrimSpace(first) != "" {
			return strings.TrimSpace(first)
		}
	}
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
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
