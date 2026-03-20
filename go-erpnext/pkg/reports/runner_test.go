package reports

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// mockReport is a test implementation of the Report interface.
type mockReport struct {
	result *ReportResult
	err    error
}

func (m *mockReport) Execute(filters map[string]interface{}) (*ReportResult, error) {
	return m.result, m.err
}

func TestRegisterAndGetReport(t *testing.T) {
	// Clean up after test
	defer func() {
		registryMu.Lock()
		delete(registry, "Test Report")
		registryMu.Unlock()
	}()

	r := &mockReport{result: &ReportResult{
		Columns: []Column{{Fieldname: "name", Label: "Name", Fieldtype: "Data"}},
		Result:  [][]interface{}{{"row1"}},
	}}

	Register("Test Report", r)

	got := GetReport("Test Report")
	if got == nil {
		t.Fatal("expected to find registered report, got nil")
	}

	got = GetReport("Nonexistent Report")
	if got != nil {
		t.Fatal("expected nil for unregistered report")
	}
}

func TestRunReportHandler_Success(t *testing.T) {
	defer func() {
		registryMu.Lock()
		delete(registry, "Sales Report")
		registryMu.Unlock()
	}()

	Register("Sales Report", &mockReport{
		result: &ReportResult{
			Columns: []Column{
				{Fieldname: "item", Label: "Item", Fieldtype: "Data", Width: 120},
				{Fieldname: "qty", Label: "Qty", Fieldtype: "Float", Width: 100},
			},
			Result: [][]interface{}{
				{"Widget A", 10.0},
				{"Widget B", 25.5},
			},
		},
	})

	handler := RunReportHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/method/frappe.desk.query_report.run?report_name=Sales+Report&filters={}", nil)
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	msg, ok := resp["message"]
	if !ok {
		t.Fatal("response missing 'message' key")
	}

	msgMap, ok := msg.(map[string]interface{})
	if !ok {
		t.Fatalf("message is not a map: %T", msg)
	}

	if _, ok := msgMap["columns"]; !ok {
		t.Error("message missing 'columns'")
	}
	if _, ok := msgMap["result"]; !ok {
		t.Error("message missing 'result'")
	}
}

func TestRunReportHandler_UnknownReport404(t *testing.T) {
	handler := RunReportHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/method/frappe.desk.query_report.run?report_name=Unknown+Report", nil)
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for unknown report, got %d", rec.Code)
	}

	var resp map[string]string
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp["error"] == "" {
		t.Error("expected error message in response")
	}
}

func TestRunReportHandler_MissingReportName(t *testing.T) {
	handler := RunReportHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/method/frappe.desk.query_report.run", nil)
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing report_name, got %d", rec.Code)
	}
}

func TestRunReportHandler_InvalidFiltersJSON(t *testing.T) {
	defer func() {
		registryMu.Lock()
		delete(registry, "Some Report")
		registryMu.Unlock()
	}()

	Register("Some Report", &mockReport{result: &ReportResult{}})

	handler := RunReportHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/method/frappe.desk.query_report.run?report_name=Some+Report&filters={bad", nil)
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid JSON, got %d", rec.Code)
	}
}

func TestRunReportHandler_ReportError(t *testing.T) {
	defer func() {
		registryMu.Lock()
		delete(registry, "Error Report")
		registryMu.Unlock()
	}()

	Register("Error Report", &mockReport{err: fmt.Errorf("database error")})

	handler := RunReportHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/method/frappe.desk.query_report.run?report_name=Error+Report", nil)
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 for report error, got %d", rec.Code)
	}
}

func TestRunReportHandler_POSTMethod(t *testing.T) {
	defer func() {
		registryMu.Lock()
		delete(registry, "POST Report")
		registryMu.Unlock()
	}()

	Register("POST Report", &mockReport{
		result: &ReportResult{
			Columns: []Column{{Fieldname: "x", Label: "X", Fieldtype: "Data"}},
			Result:  [][]interface{}{{"val"}},
		},
	})

	handler := RunReportHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/api/method/frappe.desk.query_report.run?report_name=POST+Report", nil)
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for POST, got %d", rec.Code)
	}
}

func TestRunReportHandler_MethodNotAllowed(t *testing.T) {
	handler := RunReportHandler(nil)
	req := httptest.NewRequest(http.MethodDelete, "/api/method/frappe.desk.query_report.run?report_name=X", nil)
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405 for DELETE, got %d", rec.Code)
	}
}

func TestRunReportHandler_ResponseFormat(t *testing.T) {
	defer func() {
		registryMu.Lock()
		delete(registry, "Format Report")
		registryMu.Unlock()
	}()

	Register("Format Report", &mockReport{
		result: &ReportResult{
			Columns: []Column{
				{Fieldname: "name", Label: "Name", Fieldtype: "Data", Width: 200},
			},
			Result: [][]interface{}{{"test"}},
			Chart:  nil,
		},
	})

	handler := RunReportHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/method/frappe.desk.query_report.run?report_name=Format+Report", nil)
	rec := httptest.NewRecorder()

	handler(rec, req)

	// Verify Content-Type
	ct := rec.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %s", ct)
	}

	// Verify structure matches Frappe format
	var resp map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if _, ok := resp["message"]; !ok {
		t.Error("top-level response must have 'message' key")
	}
}
