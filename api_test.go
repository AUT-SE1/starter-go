package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// These exercise routing, JSON encoding and the store together, so they are
// integration tests even though nothing leaves the process.

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(NewMux(NewOrderStore()))
	t.Cleanup(server.Close)
	return server
}

func postJSON(t *testing.T, server *httptest.Server, path, body string) *http.Response {
	t.Helper()
	response, err := server.Client().Post(server.URL+path, "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("POST %s: %v", path, err)
	}
	t.Cleanup(func() { _ = response.Body.Close() })
	return response
}

func TestPostOrders_WithAValidBody_Returns201AndTheOrder(t *testing.T) {
	server := newTestServer(t)

	response := postJSON(t, server, "/orders", `{"item":"notebook","quantity":3}`)

	if response.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", response.StatusCode)
	}
	if contentType := response.Header.Get("Content-Type"); !strings.HasPrefix(contentType, "application/json") {
		t.Errorf("expected a JSON content type, got %q", contentType)
	}
	var order Order
	if err := json.NewDecoder(response.Body).Decode(&order); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if order.Item != "notebook" || order.Quantity != 3 || order.Status != StatusCreated {
		t.Errorf("unexpected order: %+v", order)
	}
	if order.ID == "" {
		t.Error("expected the response to carry an id")
	}
}

func TestPostOrders_WithAnInvalidQuantity_Returns400(t *testing.T) {
	server := newTestServer(t)

	response := postJSON(t, server, "/orders", `{"item":"notebook","quantity":0}`)

	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", response.StatusCode)
	}
	var failure APIError
	if err := json.NewDecoder(response.Body).Decode(&failure); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if failure.Error != ErrValidation || failure.Field != "quantity" {
		t.Errorf("expected a quantity validation error, got %+v", failure)
	}
}

func TestPostOrders_WithMalformedJSON_Returns400(t *testing.T) {
	server := newTestServer(t)

	response := postJSON(t, server, "/orders", `{"item":`)

	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", response.StatusCode)
	}
}

func TestGetOrder_AfterCreating_ReturnsTheSameOrder(t *testing.T) {
	server := newTestServer(t)

	created := postJSON(t, server, "/orders", `{"item":"کتاب","quantity":7}`)
	var want Order
	if err := json.NewDecoder(created.Body).Decode(&want); err != nil {
		t.Fatalf("decode create body: %v", err)
	}

	response, err := server.Client().Get(server.URL + "/orders/" + want.ID)
	if err != nil {
		t.Fatalf("GET order: %v", err)
	}
	defer func() { _ = response.Body.Close() }()

	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", response.StatusCode)
	}
	var got Order
	if err := json.NewDecoder(response.Body).Decode(&got); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if got != want {
		t.Errorf("expected %+v, got %+v", want, got)
	}
}

func TestGetOrder_WhenIDIsUnknown_Returns404(t *testing.T) {
	server := newTestServer(t)

	response, err := server.Client().Get(server.URL + "/orders/ord_missing")
	if err != nil {
		t.Fatalf("GET order: %v", err)
	}
	defer func() { _ = response.Body.Close() }()

	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", response.StatusCode)
	}
	var failure APIError
	if err := json.NewDecoder(response.Body).Decode(&failure); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if failure.Error != ErrNotFound {
		t.Errorf("expected error %q, got %q", ErrNotFound, failure.Error)
	}
}

func TestHealth_Returns200AndStatusOK(t *testing.T) {
	server := newTestServer(t)

	response, err := server.Client().Get(server.URL + "/health")
	if err != nil {
		t.Fatalf("GET health: %v", err)
	}
	defer func() { _ = response.Body.Close() }()

	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", response.StatusCode)
	}
	var body map[string]string
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["status"] != "ok" {
		t.Errorf("expected status ok, got %q", body["status"])
	}
}
