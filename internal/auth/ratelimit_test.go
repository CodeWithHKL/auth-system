package auth

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"myapp/internal/store"
)

// memLimiter is an in-memory store.LoginLimiter. It ignores the window's
// passage of time; window expiry is SQL and is covered by the store tests.
type memLimiter struct {
	mu     sync.Mutex
	counts map[string]int
}

func (m *memLimiter) RetryAfter(_ context.Context, window time.Duration, limits []store.Limit) (time.Duration, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, l := range limits {
		if m.counts[l.Key] >= l.Max {
			return window, nil
		}
	}
	return 0, nil
}

func (m *memLimiter) RecordFailure(_ context.Context, _ time.Duration, keys ...string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, k := range keys {
		m.counts[k]++
	}
	return nil
}

func (m *memLimiter) Clear(_ context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.counts, key)
	return nil
}

func newLimitedServer() http.Handler {
	mux := http.NewServeMux()
	New(&memUsers{}, NewTokens([]byte(strings.Repeat("k", 32)), time.Hour), false, 3600).
		WithLoginLimiter(&memLimiter{counts: map[string]int{}}, false).
		Register(mux)
	return mux
}

func login(h http.Handler, email, password string) *httptest.ResponseRecorder {
	return do(h, "POST", "/api/auth/login", fmt.Sprintf(`{"email":%q,"password":%q}`, email, password))
}

func TestLoginLockedAfterTooManyFailuresForEmail(t *testing.T) {
	h := newLimitedServer()
	do(h, "POST", "/api/auth/signup", validSignup)

	for i := 0; i < maxFailuresPerEmail; i++ {
		if rec := login(h, "ada@example.com", "wrong password"); rec.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d: got %d, want 401", i+1, rec.Code)
		}
	}
	// Even the correct password is refused while locked, and it is told when to retry.
	rec := login(h, "ada@example.com", "correct horse")
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("got %d, want 429: %s", rec.Code, rec.Body)
	}
	if rec.Header().Get("Retry-After") != "900" {
		t.Errorf("Retry-After = %q, want 900", rec.Header().Get("Retry-After"))
	}
	// The email is matched case-insensitively, so changing case does not dodge the lock.
	if rec := login(h, "ADA@example.com", "correct horse"); rec.Code != http.StatusTooManyRequests {
		t.Errorf("uppercase email: got %d, want 429", rec.Code)
	}
	// Another account from the same IP is unaffected below the IP limit.
	if rec := login(h, "bob@example.com", "whatever"); rec.Code != http.StatusUnauthorized {
		t.Errorf("other email: got %d, want 401", rec.Code)
	}
}

func TestLoginLockedAfterTooManyFailuresForIP(t *testing.T) {
	h := newLimitedServer()
	for i := 0; i < maxFailuresPerIP; i++ {
		if rec := login(h, fmt.Sprintf("user%d@example.com", i), "guess"); rec.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d: got %d, want 401", i+1, rec.Code)
		}
	}
	if rec := login(h, "fresh@example.com", "guess"); rec.Code != http.StatusTooManyRequests {
		t.Errorf("got %d, want 429", rec.Code)
	}
}

func TestSuccessfulLoginResetsEmailCounter(t *testing.T) {
	h := newLimitedServer()
	do(h, "POST", "/api/auth/signup", validSignup)

	for i := 0; i < maxFailuresPerEmail-1; i++ {
		login(h, "ada@example.com", "wrong password")
	}
	if rec := login(h, "ada@example.com", "correct horse"); rec.Code != http.StatusOK {
		t.Fatalf("got %d, want 200", rec.Code)
	}
	for i := 0; i < maxFailuresPerEmail-1; i++ {
		if rec := login(h, "ada@example.com", "wrong password"); rec.Code != http.StatusUnauthorized {
			t.Fatalf("after reset, attempt %d: got %d, want 401", i+1, rec.Code)
		}
	}
}

func TestClientIP(t *testing.T) {
	req := httptest.NewRequest("POST", "/", nil)
	req.RemoteAddr = "10.0.0.9:5555"
	req.Header.Set("X-Forwarded-For", "203.0.113.7, 10.0.0.1")

	if got := clientIP(req, false); got != "10.0.0.9" {
		t.Errorf("untrusted: got %q, want the socket address, not the spoofable header", got)
	}
	if got := clientIP(req, true); got != "203.0.113.7" {
		t.Errorf("trusted XFF: got %q", got)
	}
	req.Header.Set("X-Real-IP", "198.51.100.4")
	if got := clientIP(req, true); got != "198.51.100.4" {
		t.Errorf("trusted X-Real-IP: got %q", got)
	}
}
