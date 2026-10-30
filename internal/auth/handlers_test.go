package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"myapp/internal/store"
)

// memUsers is an in-memory store.Users so handler tests need no database.
type memUsers struct {
	mu    sync.Mutex
	users []store.User
}

func (m *memUsers) Create(_ context.Context, name, email, hash string) (store.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, u := range m.users {
		if strings.EqualFold(u.Email, email) {
			return store.User{}, store.ErrEmailTaken
		}
	}
	u := store.User{ID: int64(len(m.users) + 1), Name: name, Email: email, PasswordHash: hash, CreatedAt: time.Now()}
	m.users = append(m.users, u)
	return u, nil
}

func (m *memUsers) ByEmail(_ context.Context, email string) (store.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, u := range m.users {
		if strings.EqualFold(u.Email, email) {
			return u, nil
		}
	}
	return store.User{}, store.ErrNotFound
}

func (m *memUsers) ByID(_ context.Context, id int64) (store.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, u := range m.users {
		if u.ID == id {
			return u, nil
		}
	}
	return store.User{}, store.ErrNotFound
}

func newServer() http.Handler {
	mux := http.NewServeMux()
	New(&memUsers{}, NewTokens([]byte(strings.Repeat("k", 32)), time.Hour), false, 3600).Register(mux)
	return mux
}

func do(h http.Handler, method, path, body string, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func sessionCookie(t *testing.T, rec *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()
	for _, c := range rec.Result().Cookies() {
		if c.Name == cookieName && c.Value != "" {
			if !c.HttpOnly {
				t.Error("session cookie must be HttpOnly")
			}
			return c
		}
	}
	t.Fatal("no session cookie set")
	return nil
}

const validSignup = `{"name":"Ada","email":"ada@example.com","password":"correct horse"}`

func TestSignupLoginMe(t *testing.T) {
	h := newServer()

	rec := do(h, "POST", "/api/auth/signup", validSignup)
	if rec.Code != http.StatusCreated {
		t.Fatalf("signup: got %d: %s", rec.Code, rec.Body)
	}
	if strings.Contains(rec.Body.String(), "correct horse") || strings.Contains(rec.Body.String(), "password") {
		t.Errorf("response leaks password data: %s", rec.Body)
	}
	cookie := sessionCookie(t, rec)

	me := do(h, "GET", "/api/auth/me", "", cookie)
	if me.Code != http.StatusOK {
		t.Fatalf("me after signup: got %d", me.Code)
	}
	var u store.User
	json.NewDecoder(me.Body).Decode(&u)
	if u.Email != "ada@example.com" || u.Name != "Ada" {
		t.Errorf("me returned %+v", u)
	}

	// Login is case-insensitive on the email.
	rec = do(h, "POST", "/api/auth/login", `{"email":"ADA@example.com","password":"correct horse"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("login: got %d: %s", rec.Code, rec.Body)
	}
	sessionCookie(t, rec)
}

func TestSignupValidation(t *testing.T) {
	h := newServer()
	rec := do(h, "POST", "/api/auth/signup", `{"name":"","email":"nope","password":"short"}`)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("got %d", rec.Code)
	}
	var out struct {
		Fields map[string]string `json:"fields"`
	}
	json.NewDecoder(rec.Body).Decode(&out)
	for _, f := range []string{"name", "email", "password"} {
		if out.Fields[f] == "" {
			t.Errorf("missing error for %q", f)
		}
	}
}

func TestSignupDuplicateEmail(t *testing.T) {
	h := newServer()
	do(h, "POST", "/api/auth/signup", validSignup)
	rec := do(h, "POST", "/api/auth/signup", `{"name":"Other","email":"ADA@example.com","password":"another pass"}`)
	if rec.Code != http.StatusConflict {
		t.Fatalf("got %d, want 409", rec.Code)
	}
}

func TestLoginRejectsBadCredentials(t *testing.T) {
	h := newServer()
	do(h, "POST", "/api/auth/signup", validSignup)
	for name, body := range map[string]string{
		"wrong password": `{"email":"ada@example.com","password":"wrong"}`,
		"unknown email":  `{"email":"who@example.com","password":"correct horse"}`,
	} {
		rec := do(h, "POST", "/api/auth/login", body)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s: got %d, want 401", name, rec.Code)
		}
		if len(rec.Result().Cookies()) != 0 {
			t.Errorf("%s: set a cookie", name)
		}
	}
}

func TestMeRequiresValidSession(t *testing.T) {
	h := newServer()
	if rec := do(h, "GET", "/api/auth/me", ""); rec.Code != http.StatusUnauthorized {
		t.Errorf("no cookie: got %d", rec.Code)
	}
	bad := &http.Cookie{Name: cookieName, Value: "garbage"}
	if rec := do(h, "GET", "/api/auth/me", "", bad); rec.Code != http.StatusUnauthorized {
		t.Errorf("bad token: got %d", rec.Code)
	}

	// A token signed with a different secret must be rejected.
	other, _ := NewTokens([]byte(strings.Repeat("x", 32)), time.Hour).Issue(1)
	if rec := do(h, "GET", "/api/auth/me", "", &http.Cookie{Name: cookieName, Value: other}); rec.Code != http.StatusUnauthorized {
		t.Errorf("foreign token: got %d", rec.Code)
	}
}

func TestExpiredTokenRejected(t *testing.T) {
	tk := NewTokens([]byte(strings.Repeat("k", 32)), -time.Minute)
	raw, _ := tk.Issue(1)
	if _, err := tk.Verify(raw); err == nil {
		t.Error("expired token verified")
	}
}

func TestLogoutClearsCookie(t *testing.T) {
	rec := do(newServer(), "POST", "/api/auth/logout", "")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("got %d", rec.Code)
	}
	cs := rec.Result().Cookies()
	if len(cs) != 1 || cs[0].MaxAge >= 0 {
		t.Errorf("cookie not cleared: %+v", cs)
	}
}
