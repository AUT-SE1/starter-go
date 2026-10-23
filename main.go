package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"
)

// NewMux wires every route. Tests call it directly with httptest, so the
// routing they exercise is the same routing production uses.
func NewMux(store *OrderStore) *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		// Match only this path, not a prefix.
		if r.URL.Path != "/health" {
			http.NotFound(w, r)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	mux.HandleFunc("POST /orders", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/orders" {
			http.NotFound(w, r)
			return
		}
		var request CreateOrderRequest
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64*1024)).Decode(&request); err != nil {
			writeJSON(w, http.StatusBadRequest, APIError{
				Error:   ErrValidation,
				Field:   "body",
				Message: "request body is not valid JSON",
			})
			return
		}
		item, quantity, failure := ValidateCreateOrder(request)
		if failure != nil {
			writeJSON(w, http.StatusBadRequest, failure)
			return
		}
		writeJSON(w, http.StatusCreated, store.Create(item, quantity))
	})

	mux.HandleFunc("GET /orders/{id}", func(w http.ResponseWriter, r *http.Request) {
		order, found := store.Get(r.PathValue("id"))
		if !found {
			writeJSON(w, http.StatusNotFound, APIError{
				Error:   ErrNotFound,
				Message: "no order with that id",
			})
			return
		}
		writeJSON(w, http.StatusOK, order)
	})

	return mux
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		log.Printf("write response: %v", err)
	}
}

func main() {
	address := os.Getenv("LISTEN_ADDR")
	if address == "" {
		address = "127.0.0.1:8080"
	}

	server := &http.Server{
		Addr:              address,
		Handler:           NewMux(NewOrderStore()),
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	log.Printf("Listening on %s", address)
	log.Fatal(server.ListenAndServe())
}
