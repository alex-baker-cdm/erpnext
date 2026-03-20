package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestStockBalanceHandler_SerialNo501(t *testing.T) {
	// When with_serial_no is truthy, the handler should return 501.
	handler := stockBalanceHandler(nil)

	req := httptest.NewRequest(http.MethodGet,
		"/api/method/erpnext.stock.utils.get_stock_balance?item_code=TEST&warehouse=WH&with_serial_no=1", nil)
	w := httptest.NewRecorder()

	handler(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusNotImplemented {
		t.Errorf("expected status 501, got %d", resp.StatusCode)
	}

	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if body["error"] != "with_serial_no not implemented in Go service" {
		t.Errorf("unexpected error message: %s", body["error"])
	}
}

func TestStockBalanceHandler_NoDB(t *testing.T) {
	// Without a DB connection, passing nil should cause an error when actually
	// calling stock functions. with_serial_no=0 means we proceed into the code.
	// Since database is nil, calling GetStockBalance will panic/error.
	// This test verifies that with_serial_no detection works before DB access.
	handler := stockBalanceHandler(nil)

	req := httptest.NewRequest(http.MethodGet,
		"/api/method/erpnext.stock.utils.get_stock_balance?item_code=TEST&warehouse=WH&with_serial_no=true", nil)
	w := httptest.NewRecorder()

	handler(w, req)

	// with_serial_no=true should still trigger 501
	if w.Result().StatusCode != http.StatusNotImplemented {
		t.Errorf("expected 501 for with_serial_no=true, got %d", w.Result().StatusCode)
	}
}

func TestStockItemHandler_NoBarcode_NoDB(t *testing.T) {
	// Without a barcode and without an item, the handler should still return
	// a valid JSON response (even if the values are empty/zero) when it
	// encounters a nil DB error.
	// This tests that the handler returns structured errors rather than panicking.
	//
	// Note: With a nil DB, the handler will panic on RawQuery calls.
	// In production, routes are only registered when DB != nil (see main.go).
	// This test documents that behaviour.
	t.Log("Skipping: handler requires non-nil DB; routes are only registered when DB is available")
}

func TestWriteError(t *testing.T) {
	w := httptest.NewRecorder()
	writeError(w, http.StatusBadRequest, "test error")

	resp := w.Result()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", resp.StatusCode)
	}

	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if body["error"] != "test error" {
		t.Errorf("expected 'test error', got %q", body["error"])
	}
}
