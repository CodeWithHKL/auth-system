package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newTestMux() *http.ServeMux {
	mux := http.NewServeMux()
	NewTodos().Register(mux)
	return mux
}

func do(mux *http.ServeMux, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func TestUpdateTodoText(t *testing.T) {
	mux := newTestMux()
	do(mux, "POST", "/api/todos", `{"text":"old"}`)

	rec := do(mux, "PUT", "/api/todos/1", `{"text":"  new  "}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"text":"new"`) {
		t.Errorf("body = %s, want trimmed text", rec.Body.String())
	}
}

func TestUpdateTodoErrors(t *testing.T) {
	mux := newTestMux()
	do(mux, "POST", "/api/todos", `{"text":"a"}`)

	cases := []struct {
		name, path, body string
		want             int
	}{
		{"bad id", "/api/todos/x", `{"text":"b"}`, http.StatusBadRequest},
		{"bad json", "/api/todos/1", `{`, http.StatusBadRequest},
		{"empty text", "/api/todos/1", `{"text":"  "}`, http.StatusBadRequest},
		{"missing", "/api/todos/99", `{"text":"b"}`, http.StatusNotFound},
	}
	for _, c := range cases {
		if got := do(mux, "PUT", c.path, c.body).Code; got != c.want {
			t.Errorf("%s: status = %d, want %d", c.name, got, c.want)
		}
	}
}
