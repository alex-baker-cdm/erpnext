package dashboard

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestChartResultJSON(t *testing.T) {
	cr := ChartResult{
		Labels: []string{"Jan", "Feb", "Mar"},
		Datasets: []Dataset{
			{Name: "Revenue", Values: []float64{100, 200, 300}},
		},
	}

	data, err := json.Marshal(cr)
	if err != nil {
		t.Fatalf("failed to marshal ChartResult: %v", err)
	}

	var decoded ChartResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal ChartResult: %v", err)
	}

	if len(decoded.Labels) != 3 {
		t.Errorf("expected 3 labels, got %d", len(decoded.Labels))
	}
	if decoded.Labels[0] != "Jan" {
		t.Errorf("expected first label 'Jan', got %q", decoded.Labels[0])
	}
	if len(decoded.Datasets) != 1 {
		t.Errorf("expected 1 dataset, got %d", len(decoded.Datasets))
	}
	if decoded.Datasets[0].Name != "Revenue" {
		t.Errorf("expected dataset name 'Revenue', got %q", decoded.Datasets[0].Name)
	}
	if len(decoded.Datasets[0].Values) != 3 {
		t.Errorf("expected 3 values, got %d", len(decoded.Datasets[0].Values))
	}
	if decoded.Datasets[0].Values[2] != 300 {
		t.Errorf("expected third value 300, got %f", decoded.Datasets[0].Values[2])
	}
}

func TestChartResultJSONEmpty(t *testing.T) {
	cr := ChartResult{
		Labels:   []string{},
		Datasets: []Dataset{},
	}

	data, err := json.Marshal(cr)
	if err != nil {
		t.Fatalf("failed to marshal empty ChartResult: %v", err)
	}

	expected := `{"labels":[],"datasets":[]}`
	if string(data) != expected {
		t.Errorf("expected %s, got %s", expected, string(data))
	}
}

func TestWriteJSON(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteJSON(rec, map[string]string{"key": "value"})

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	ct := rec.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %q", ct)
	}

	var envelope map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	msg, ok := envelope["message"]
	if !ok {
		t.Fatal("expected 'message' key in envelope")
	}

	msgMap, ok := msg.(map[string]interface{})
	if !ok {
		t.Fatal("expected message to be a map")
	}
	if msgMap["key"] != "value" {
		t.Errorf("expected message.key = 'value', got %v", msgMap["key"])
	}
}

func TestWriteJSONChartResult(t *testing.T) {
	rec := httptest.NewRecorder()
	cr := ChartResult{
		Labels: []string{"W1", "W2"},
		Datasets: []Dataset{
			{Name: "Stock Value", Values: []float64{1000, 2000}},
		},
	}
	WriteJSON(rec, cr)

	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("failed to unmarshal envelope: %v", err)
	}

	var result ChartResult
	if err := json.Unmarshal(envelope["message"], &result); err != nil {
		t.Fatalf("failed to unmarshal ChartResult from message: %v", err)
	}

	if len(result.Labels) != 2 {
		t.Errorf("expected 2 labels, got %d", len(result.Labels))
	}
	if result.Datasets[0].Name != "Stock Value" {
		t.Errorf("expected dataset name 'Stock Value', got %q", result.Datasets[0].Name)
	}
}

func TestParseFiltersEmpty(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	filters, err := ParseFilters(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(filters) != 0 {
		t.Errorf("expected empty filters, got %v", filters)
	}
}

func TestParseFiltersValid(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test?filters="+
		"%7B%22company%22%3A%22TestCo%22%2C%22account%22%3A%22Revenue%22%7D", nil)
	filters, err := ParseFilters(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if filters["company"] != "TestCo" {
		t.Errorf("expected company 'TestCo', got %v", filters["company"])
	}
	if filters["account"] != "Revenue" {
		t.Errorf("expected account 'Revenue', got %v", filters["account"])
	}
}

func TestParseFiltersInvalid(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, `/test?filters=not-json`, nil)
	_, err := ParseFilters(req)
	if err == nil {
		t.Error("expected error for invalid JSON, got nil")
	}
}

func TestParseStringParam(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test?item_code=ITEM-001&warehouse=", nil)
	if got := parseStringParam(req, "item_code", ""); got != "ITEM-001" {
		t.Errorf("expected 'ITEM-001', got %q", got)
	}
	if got := parseStringParam(req, "warehouse", "default"); got != "default" {
		t.Errorf("expected 'default', got %q", got)
	}
	if got := parseStringParam(req, "missing", "fallback"); got != "fallback" {
		t.Errorf("expected 'fallback', got %q", got)
	}
}

func TestParseIntParam(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test?start=10&bad=abc", nil)
	if got := parseIntParam(req, "start", 0); got != 10 {
		t.Errorf("expected 10, got %d", got)
	}
	if got := parseIntParam(req, "bad", 5); got != 5 {
		t.Errorf("expected 5 for non-numeric, got %d", got)
	}
	if got := parseIntParam(req, "missing", 42); got != 42 {
		t.Errorf("expected 42 for missing param, got %d", got)
	}
}

func TestValidateSortParams(t *testing.T) {
	tests := []struct {
		sortBy    string
		sortOrder string
		want      bool
	}{
		{"actual_qty", "desc", true},
		{"actual_qty", "asc", true},
		{"stock_capacity", "desc", true},
		{"item_code", "asc", true},
		{"invalid_col", "desc", false},
		{"actual_qty", "INVALID", false},
		{"", "", false},
		{"actual_qty; DROP TABLE--", "desc", false},
	}
	for _, tt := range tests {
		got := validateSortParams(tt.sortBy, tt.sortOrder)
		if got != tt.want {
			t.Errorf("validateSortParams(%q, %q) = %v, want %v", tt.sortBy, tt.sortOrder, got, tt.want)
		}
	}
}
