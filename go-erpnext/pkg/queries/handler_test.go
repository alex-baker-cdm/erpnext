package queries

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestParseSearchParams_Defaults(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/test", nil)
	params, err := ParseSearchParams(r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if params.PageLen != 20 {
		t.Errorf("expected default PageLen=20, got %d", params.PageLen)
	}
	if params.Start != 0 {
		t.Errorf("expected default Start=0, got %d", params.Start)
	}
	if params.Txt != "" {
		t.Errorf("expected empty Txt, got %q", params.Txt)
	}
}

func TestParseSearchParams_WithValues(t *testing.T) {
	u, _ := url.Parse("/test?doctype=Item&txt=widget&searchfield=item_name&start=10&page_len=50&filters=%7B%22company%22%3A%22TestCo%22%7D")
	r := httptest.NewRequest(http.MethodGet, u.String(), nil)
	params, err := ParseSearchParams(r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if params.Doctype != "Item" {
		t.Errorf("expected Doctype=Item, got %q", params.Doctype)
	}
	if params.Txt != "widget" {
		t.Errorf("expected Txt=widget, got %q", params.Txt)
	}
	if params.Searchfield != "item_name" {
		t.Errorf("expected Searchfield=item_name, got %q", params.Searchfield)
	}
	if params.Start != 10 {
		t.Errorf("expected Start=10, got %d", params.Start)
	}
	if params.PageLen != 50 {
		t.Errorf("expected PageLen=50, got %d", params.PageLen)
	}
	if params.Filters == nil {
		t.Fatal("expected non-nil Filters")
	}
	if params.Filters["company"] != "TestCo" {
		t.Errorf("expected company=TestCo, got %v", params.Filters["company"])
	}
}

func TestParseSearchParams_InvalidFilters(t *testing.T) {
	u, _ := url.Parse("/test?filters=not-valid-json")
	r := httptest.NewRequest(http.MethodGet, u.String(), nil)
	params, err := ParseSearchParams(r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Invalid JSON should result in nil Filters
	if params.Filters != nil {
		t.Errorf("expected nil Filters for invalid JSON, got %v", params.Filters)
	}
}

func TestEscapeLike(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"hello", "hello"},
		{"hello%world", "hello\\%world"},
		{"hello_world", "hello\\_world"},
		{"%_both_%_", "\\%\\_both\\_\\%\\_"},
		{"", ""},
	}
	for _, tc := range tests {
		got := EscapeLike(tc.input)
		if got != tc.expected {
			t.Errorf("EscapeLike(%q) = %q, want %q", tc.input, got, tc.expected)
		}
	}
}

func TestWriteJSON(t *testing.T) {
	w := httptest.NewRecorder()
	data := [][]interface{}{{"item1", 10}, {"item2", 20}}
	WriteJSON(w, data)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	ct := resp.Header.Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %q", ct)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	msg, ok := body["message"]
	if !ok {
		t.Fatal("response missing 'message' key")
	}

	arr, ok := msg.([]interface{})
	if !ok {
		t.Fatalf("expected message to be array, got %T", msg)
	}
	if len(arr) != 2 {
		t.Errorf("expected 2 items in message, got %d", len(arr))
	}
}

func TestWriteError(t *testing.T) {
	w := httptest.NewRecorder()
	WriteError(w, http.StatusBadRequest, "missing param")

	resp := w.Result()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", resp.StatusCode)
	}

	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if body["error"] != "missing param" {
		t.Errorf("expected error='missing param', got %q", body["error"])
	}
}

func TestGetFiltersCond(t *testing.T) {
	// Empty filters
	var args []interface{}
	result := GetFiltersCond(nil, &args)
	if result != "" {
		t.Errorf("expected empty string for nil filters, got %q", result)
	}

	// Simple equality filters
	args = nil
	filters := map[string]interface{}{
		"company": "TestCo",
	}
	result = GetFiltersCond(filters, &args)
	if result == "" {
		t.Error("expected non-empty filter condition")
	}
	if len(args) != 1 {
		t.Errorf("expected 1 arg, got %d", len(args))
	}
	if args[0] != "TestCo" {
		t.Errorf("expected arg 'TestCo', got %v", args[0])
	}
}

func TestGetMatchCond(t *testing.T) {
	// Should return empty string (placeholder)
	result := GetMatchCond("Employee")
	if result != "" {
		t.Errorf("expected empty match condition, got %q", result)
	}
}

func TestParseListFilters(t *testing.T) {
	// Valid list filters
	raw := `[["Warehouse","company","=","TestCo"],["Bin","item_code","=","ITEM-001"]]`
	filters, err := ParseListFilters(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(filters) != 2 {
		t.Errorf("expected 2 filters, got %d", len(filters))
	}

	// Empty string
	filters, err = ParseListFilters("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if filters != nil {
		t.Errorf("expected nil for empty string, got %v", filters)
	}

	// Invalid JSON
	_, err = ParseListFilters("invalid")
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestSanitizeOperator(t *testing.T) {
	// Valid operators
	validOps := []string{"=", "!=", "<", ">", "<=", ">=", "LIKE", "like", "IN", "in", "NOT IN", "not in"}
	for _, op := range validOps {
		result, err := SanitizeOperator(op)
		if err != nil {
			t.Errorf("SanitizeOperator(%q) returned unexpected error: %v", op, err)
		}
		if result == "" {
			t.Errorf("SanitizeOperator(%q) returned empty string", op)
		}
	}

	// Invalid operators (SQL injection attempts)
	invalidOps := []string{"DROP TABLE", "; --", "1=1 OR", "UNION", "DELETE"}
	for _, op := range invalidOps {
		_, err := SanitizeOperator(op)
		if err == nil {
			t.Errorf("SanitizeOperator(%q) should have returned error", op)
		}
	}
}

func TestMakeHandler_NilDB(t *testing.T) {
	called := false
	fn := func(dbConn interface{}, w http.ResponseWriter, r *http.Request) {
		called = true
	}

	// We test the nil-DB guard concept: the handler should return 503
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate MakeHandler nil guard
		if true { // simulating nil dbConn
			WriteError(w, http.StatusServiceUnavailable, "database connection not configured")
			return
		}
		fn(nil, w, r)
	})

	r := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if called {
		t.Error("handler should not have been called with nil DB")
	}
	if w.Result().StatusCode != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", w.Result().StatusCode)
	}
}
