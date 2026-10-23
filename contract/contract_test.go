package contract

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Session 05 — contract test against the NEIGHBOURING TEAM's running sandbox.
//
// This is not a mock. It sends real HTTP to a service you did not write and
// checks the answer against the OpenAPI document both teams agreed on.
//
// Run it:
//     NEIGHBOR_BASE_URL=http://10.0.0.42:8080 make contract-test
//
// With no NEIGHBOR_BASE_URL set, every test here skips, so `make test` stays
// green on a laptop with no neighbour in reach. A skipped contract test is not
// a passing one — your report must show it running against a real address.
//
// TODO(team): two things to change, and nothing else.
//   1. neighborOrdersPath / neighborOrderPath below, if your neighbour's
//      service names its routes differently from the shared contract.
//   2. validOrderBody, if their CreateOrderRequest carries extra fields.
// ---------------------------------------------------------------------------

const (
	neighborOrdersPath = "/orders" // TODO(team): confirm against their OpenAPI
	specPath           = "../openapi.json"
	requestTimeout     = 10 * time.Second
)

func validOrderBody() map[string]any {
	// TODO(team): match your neighbour's CreateOrderRequest schema.
	return map[string]any{"item": "contract-test-item", "quantity": 2}
}

func baseURL(t *testing.T) string {
	t.Helper()
	url := os.Getenv("NEIGHBOR_BASE_URL")
	if url == "" {
		t.Skip("NEIGHBOR_BASE_URL is not set — skipping the contract test. " +
			"Set it to your neighbour's sandbox, e.g. http://10.0.0.42:8080")
	}
	return url
}

func document(t *testing.T) *Document {
	t.Helper()
	doc, err := LoadDocument(specPath)
	if err != nil {
		t.Fatalf("load the agreed OpenAPI document: %v", err)
	}
	return doc
}

func send(t *testing.T, method, url string, body any) (*http.Response, map[string]any) {
	t.Helper()

	var payload []byte
	if body != nil {
		var err error
		if payload, err = json.Marshal(body); err != nil {
			t.Fatalf("encode request body: %v", err)
		}
	}
	request, err := http.NewRequest(method, url, bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}

	client := &http.Client{Timeout: requestTimeout}
	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("%s %s failed to reach the neighbour's sandbox: %v\n"+
			"Is their service running, listening on 0.0.0.0 and allowed through the firewall?",
			method, url, err)
	}
	t.Cleanup(func() { _ = response.Body.Close() })

	// A timestamp belongs in the report, so print one with every call.
	t.Logf("%s  %s %s -> %d", time.Now().Format(time.RFC3339), method, url, response.StatusCode)

	var decoded map[string]any
	if err := json.NewDecoder(response.Body).Decode(&decoded); err != nil {
		t.Fatalf("%s %s did not return a JSON object: %v", method, url, err)
	}
	return response, decoded
}

func assertMatches(t *testing.T, doc *Document, schemaName string, value any) {
	t.Helper()
	problems := doc.Validate(schemaName, value)
	for _, problem := range problems {
		t.Errorf("response does not match the agreed %s schema: %s", schemaName, problem)
	}
}

// --- the contract -----------------------------------------------------------

func TestNeighbor_CreateOrder_Returns201AndAnOrderMatchingTheContract(t *testing.T) {
	base := baseURL(t)
	doc := document(t)

	response, body := send(t, http.MethodPost, base+neighborOrdersPath, validOrderBody())

	if response.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d — body: %v", response.StatusCode, body)
	}
	assertMatches(t, doc, "Order", body)
}

func TestNeighbor_CreateOrder_WithQuantityZero_Returns400AndAnError(t *testing.T) {
	base := baseURL(t)
	doc := document(t)

	invalid := validOrderBody()
	invalid["quantity"] = 0

	response, body := send(t, http.MethodPost, base+neighborOrdersPath, invalid)

	// The contract fixes the status code and the error shape. It says nothing
	// about the wording of the message — that is their business, not ours.
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for an invalid quantity, got %d — body: %v",
			response.StatusCode, body)
	}
	assertMatches(t, doc, "Error", body)
}

func TestNeighbor_GetOrder_AfterCreating_ReturnsTheSameOrder(t *testing.T) {
	base := baseURL(t)
	doc := document(t)

	_, created := send(t, http.MethodPost, base+neighborOrdersPath, validOrderBody())
	id, ok := created["id"].(string)
	if !ok || id == "" {
		t.Fatalf("the create response carried no usable id: %v", created)
	}

	response, body := send(t, http.MethodGet, base+neighborOrdersPath+"/"+id, nil)

	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 when reading back order %q, got %d", id, response.StatusCode)
	}
	assertMatches(t, doc, "Order", body)
	if body["id"] != id {
		t.Errorf("expected to read back id %q, got %v", id, body["id"])
	}
}

func TestNeighbor_GetOrder_WithAnUnknownID_Returns404(t *testing.T) {
	base := baseURL(t)
	doc := document(t)

	response, body := send(t, http.MethodGet, base+neighborOrdersPath+"/ord_definitely_not_real", nil)

	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 for an unknown id, got %d — body: %v",
			response.StatusCode, body)
	}
	assertMatches(t, doc, "Error", body)
}
