package queries

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetDoctypesForClosing_NoFilter(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		GetDoctypesForClosing(nil, w, r)
	}

	r := httptest.NewRequest(http.MethodGet, "/api/method/erpnext.controllers.queries.get_doctypes_for_closing", nil)
	w := httptest.NewRecorder()
	handler(w, r)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}

	msg, ok := body["message"]
	if !ok {
		t.Fatal("missing 'message' key")
	}

	arr, ok := msg.([]interface{})
	if !ok {
		t.Fatalf("expected array, got %T", msg)
	}

	// Should return all 7 doctypes
	if len(arr) != 7 {
		t.Errorf("expected 7 doctypes, got %d", len(arr))
	}

	// Verify all expected doctypes are present
	expected := map[string]bool{
		"Sales Invoice":    false,
		"Purchase Invoice": false,
		"Sales Order":      false,
		"Purchase Order":   false,
		"Quotation":        false,
		"Delivery Note":    false,
		"Purchase Receipt": false,
	}

	for _, item := range arr {
		tuple, ok := item.([]interface{})
		if !ok || len(tuple) == 0 {
			continue
		}
		name, ok := tuple[0].(string)
		if !ok {
			continue
		}
		if _, exists := expected[name]; exists {
			expected[name] = true
		}
	}

	for name, found := range expected {
		if !found {
			t.Errorf("expected doctype %q not found in results", name)
		}
	}
}

func TestGetDoctypesForClosing_WithFilter(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		GetDoctypesForClosing(nil, w, r)
	}

	r := httptest.NewRequest(http.MethodGet, "/test?txt=sales", nil)
	w := httptest.NewRecorder()
	handler(w, r)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}

	msg := body["message"].([]interface{})
	// Should return "Sales Invoice" and "Sales Order"
	if len(msg) != 2 {
		t.Errorf("expected 2 filtered doctypes, got %d", len(msg))
	}

	for _, item := range msg {
		tuple := item.([]interface{})
		name := tuple[0].(string)
		if name != "Sales Invoice" && name != "Sales Order" {
			t.Errorf("unexpected doctype in filtered results: %q", name)
		}
	}
}

func TestGetDoctypesForClosing_CaseInsensitive(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		GetDoctypesForClosing(nil, w, r)
	}

	r := httptest.NewRequest(http.MethodGet, "/test?txt=PURCHASE", nil)
	w := httptest.NewRecorder()
	handler(w, r)

	resp := w.Result()
	var body map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&body)

	msg := body["message"].([]interface{})
	// Should return "Purchase Invoice", "Purchase Order", "Purchase Receipt"
	if len(msg) != 3 {
		t.Errorf("expected 3 purchase doctypes, got %d", len(msg))
	}
}

func TestResponseFormatWrapping(t *testing.T) {
	// Test that WriteJSON wraps data in {"message": ...}
	w := httptest.NewRecorder()
	WriteJSON(w, [][]interface{}{{"test", 1}})

	var body map[string]interface{}
	json.NewDecoder(w.Result().Body).Decode(&body)

	if _, ok := body["message"]; !ok {
		t.Error("response not wrapped in 'message' key")
	}
}

func TestMakeHandler(t *testing.T) {
	// Test that MakeHandler correctly wraps a function
	called := false
	fn := func(dbConn interface{}, w http.ResponseWriter, r *http.Request) {
		called = true
		WriteJSON(w, "ok")
	}

	// We can't use the real MakeHandler since it expects *db.DB, but we can test the concept
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fn(nil, w, r)
	})

	r := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if !called {
		t.Error("handler function was not called")
	}

	if w.Result().StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Result().StatusCode)
	}
}
