package main

import (
	"log"
	"net/http"

	"myapp/internal/handlers"
)

func main() {
	mux := http.NewServeMux()
	handlers.NewTodos().Register(mux)

	addr := ":8080"
	log.Printf("API listening on http://localhost%s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}
