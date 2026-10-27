package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Todo struct {
	ID   int       `json:"id"`
	Text string    `json:"text"`
	Done bool      `json:"done"`
	Date time.Time `json:"date"`
}

// Todos is an in-memory todo store. The mutex guards it because
// every HTTP request is handled in its own goroutine.
type Todos struct {
	mu     sync.Mutex
	items  []Todo
	nextID int
}

func NewTodos() *Todos {
	return &Todos{items: []Todo{}, nextID: 1}
}

// Register wires the todo routes onto mux (method+path patterns need Go 1.22+).
func (t *Todos) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/todos", t.list)
	mux.HandleFunc("POST /api/todos", t.create)
	mux.HandleFunc("PATCH /api/todos/{id}", t.toggle)
	mux.HandleFunc("DELETE /api/todos/{id}", t.remove)
}

func (t *Todos) list(w http.ResponseWriter, r *http.Request) {
	t.mu.Lock()
	defer t.mu.Unlock()
	writeJSON(w, http.StatusOK, t.items)
}

func (t *Todos) create(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	text := strings.TrimSpace(in.Text)
	if text == "" {
		http.Error(w, "text is required", http.StatusBadRequest)
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()
	todo := Todo{ID: t.nextID, Text: text, Date: time.Now()}
	t.nextID++
	t.items = append(t.items, todo)
	writeJSON(w, http.StatusCreated, todo)
}

func (t *Todos) toggle(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()
	for i := range t.items {
		if t.items[i].ID == id {
			t.items[i].Done = !t.items[i].Done
			writeJSON(w, http.StatusOK, t.items[i])
			return
		}
	}
	http.NotFound(w, r)
}

func (t *Todos) remove(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()
	for i := range t.items {
		if t.items[i].ID == id {
			t.items = append(t.items[:i], t.items[i+1:]...)
			w.WriteHeader(http.StatusNoContent)
			return
		}
	}
	http.NotFound(w, r)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
