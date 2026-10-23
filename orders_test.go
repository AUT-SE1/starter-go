package main

import (
	"strings"
	"testing"
)

// Helpers so each case reads as data rather than as pointer plumbing.
func str(value string) *string { return &value }
func num(value int) *int       { return &value }

func TestValidateCreateOrder_WhenRequestIsValid_ReturnsCleanedValues(t *testing.T) {
	item, quantity, failure := ValidateCreateOrder(CreateOrderRequest{
		Item:     str("  notebook  "),
		Quantity: num(3),
	})

	if failure != nil {
		t.Fatalf("expected no failure, got %+v", failure)
	}
	if item != "notebook" {
		t.Errorf("expected the item to be trimmed to %q, got %q", "notebook", item)
	}
	if quantity != 3 {
		t.Errorf("expected quantity 3, got %d", quantity)
	}
}

func TestValidateCreateOrder_WhenInputIsInvalid_ReturnsTheOffendingField(t *testing.T) {
	cases := []struct {
		name          string
		request       CreateOrderRequest
		expectedField string
	}{
		{"item is missing", CreateOrderRequest{Quantity: num(1)}, "item"},
		{"item is empty", CreateOrderRequest{Item: str(""), Quantity: num(1)}, "item"},
		{"item is only whitespace", CreateOrderRequest{Item: str("   "), Quantity: num(1)}, "item"},
		{"item is too long", CreateOrderRequest{Item: str(strings.Repeat("a", 65)), Quantity: num(1)}, "item"},
		{"quantity is missing", CreateOrderRequest{Item: str("book")}, "quantity"},
		{"quantity is zero", CreateOrderRequest{Item: str("book"), Quantity: num(0)}, "quantity"},
		{"quantity is negative", CreateOrderRequest{Item: str("book"), Quantity: num(-5)}, "quantity"},
		{"quantity is above the maximum", CreateOrderRequest{Item: str("book"), Quantity: num(1001)}, "quantity"},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			_, _, failure := ValidateCreateOrder(testCase.request)

			if failure == nil {
				t.Fatal("expected validation to fail, it succeeded")
			}
			if failure.Error != ErrValidation {
				t.Errorf("expected error %q, got %q", ErrValidation, failure.Error)
			}
			if failure.Field != testCase.expectedField {
				t.Errorf("expected field %q, got %q", testCase.expectedField, failure.Field)
			}
		})
	}
}

func TestValidateCreateOrder_AtTheQuantityBoundaries_Succeeds(t *testing.T) {
	for _, quantity := range []int{MinQuantity, MaxQuantity} {
		if _, _, failure := ValidateCreateOrder(CreateOrderRequest{
			Item: str("book"), Quantity: num(quantity),
		}); failure != nil {
			t.Errorf("quantity %d is inside the contract but was rejected: %+v", quantity, failure)
		}
	}
}

func TestValidateCreateOrder_WhenItemIsNonASCII_KeepsItIntact(t *testing.T) {
	// 64 runes is the limit, not 64 bytes — a Persian item name must fit.
	item, _, failure := ValidateCreateOrder(CreateOrderRequest{
		Item: str("کتاب"), Quantity: num(1),
	})

	if failure != nil {
		t.Fatalf("expected a non-ASCII item to be accepted, got %+v", failure)
	}
	if item != "کتاب" {
		t.Errorf("expected the item to survive unchanged, got %q", item)
	}
}

func TestOrderStore_AfterCreate_ReturnsTheSameOrder(t *testing.T) {
	store := NewOrderStore()

	created := store.Create("notebook", 2)
	found, ok := store.Get(created.ID)

	if !ok {
		t.Fatalf("expected to find order %q", created.ID)
	}
	if found != created {
		t.Errorf("expected %+v, got %+v", created, found)
	}
	if !strings.HasPrefix(created.ID, "ord_") {
		t.Errorf("expected the id to start with %q, got %q", "ord_", created.ID)
	}
	if created.Status != StatusCreated {
		t.Errorf("expected status %q, got %q", StatusCreated, created.Status)
	}
}

func TestOrderStore_WhenIDIsUnknown_ReportsNotFound(t *testing.T) {
	store := NewOrderStore()

	if _, ok := store.Get("ord_does_not_exist"); ok {
		t.Error("expected an unknown id to be reported as missing")
	}
}

func TestOrderStore_GivesEachOrderItsOwnID(t *testing.T) {
	store := NewOrderStore()

	first := store.Create("book", 1)
	second := store.Create("book", 1)

	if first.ID == second.ID {
		t.Errorf("expected two different ids, both were %q", first.ID)
	}
}
