package main

import (
	"crypto/rand"
	"encoding/hex"
	"strings"
	"sync"
	"unicode/utf8"
)

// The validation bounds are part of the shared contract in openapi.yaml.
// Keep them and the specification in step with each other.
const (
	MinQuantity   = 1
	MaxQuantity   = 1000
	MaxItemRunes  = 64
	StatusCreated = "created"
	ErrValidation = "validation_failed"
	ErrNotFound   = "not_found"
)

// CreateOrderRequest is the request body of POST /orders.
// The fields are pointers so that a missing field can be told apart from a
// zero value: {"quantity": 0} and {} must produce different messages.
type CreateOrderRequest struct {
	Item     *string `json:"item"`
	Quantity *int    `json:"quantity"`
}

// Order is the response body of POST /orders and GET /orders/{id}.
type Order struct {
	ID       string `json:"id"`
	Item     string `json:"item"`
	Quantity int    `json:"quantity"`
	Status   string `json:"status"`
}

// APIError is the body returned with every 4xx response.
type APIError struct {
	Error   string `json:"error"`
	Field   string `json:"field,omitempty"`
	Message string `json:"message,omitempty"`
}

// ValidateCreateOrder checks one request body and returns the cleaned values.
// It touches no network, no clock and no storage, which is what makes the
// tests for it unit tests.
func ValidateCreateOrder(request CreateOrderRequest) (item string, quantity int, failure *APIError) {
	if request.Item == nil {
		return "", 0, &APIError{Error: ErrValidation, Field: "item", Message: "item is required"}
	}
	item = strings.TrimSpace(*request.Item)
	if item == "" {
		return "", 0, &APIError{Error: ErrValidation, Field: "item", Message: "item must not be empty"}
	}
	if utf8.RuneCountInString(item) > MaxItemRunes {
		return "", 0, &APIError{Error: ErrValidation, Field: "item", Message: "item is longer than 64 characters"}
	}
	if request.Quantity == nil {
		return "", 0, &APIError{Error: ErrValidation, Field: "quantity", Message: "quantity is required"}
	}
	quantity = *request.Quantity
	if quantity < MinQuantity || quantity > MaxQuantity {
		return "", 0, &APIError{Error: ErrValidation, Field: "quantity", Message: "quantity must be between 1 and 1000"}
	}
	return item, quantity, nil
}

// OrderStore keeps orders in memory. A real service would use a database;
// swapping this out is what Session 06 onwards is for.
type OrderStore struct {
	mutex  sync.RWMutex
	orders map[string]Order
}

func NewOrderStore() *OrderStore {
	return &OrderStore{orders: make(map[string]Order)}
}

func (store *OrderStore) Create(item string, quantity int) Order {
	order := Order{ID: newOrderID(), Item: item, Quantity: quantity, Status: StatusCreated}
	store.mutex.Lock()
	defer store.mutex.Unlock()
	store.orders[order.ID] = order
	return order
}

func (store *OrderStore) Get(id string) (Order, bool) {
	store.mutex.RLock()
	defer store.mutex.RUnlock()
	order, found := store.orders[id]
	return order, found
}

func newOrderID() string {
	buffer := make([]byte, 8)
	if _, err := rand.Read(buffer); err != nil {
		// crypto/rand does not fail on the platforms we target; if it ever
		// does, failing loudly beats handing out a predictable id.
		panic("read random bytes: " + err.Error())
	}
	return "ord_" + hex.EncodeToString(buffer)
}
