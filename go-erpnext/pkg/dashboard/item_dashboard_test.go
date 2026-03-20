package dashboard

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestItemDashboardSortValidation(t *testing.T) {
	tests := []struct {
		name      string
		sortBy    string
		sortOrder string
		wantCode  int
	}{
		{"valid sort", "actual_qty", "desc", http.StatusInternalServerError}, // Internal error because no DB
		{"valid sort asc", "projected_qty", "asc", http.StatusInternalServerError},
		{"invalid sort_by", "malicious_column", "desc", http.StatusBadRequest},
		{"invalid sort_order", "actual_qty", "INVALID", http.StatusBadRequest},
		{"sql injection sort_by", "1=1", "desc", http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := MakeHandler(nil, ItemDashboardGetData)
			url := "/test?sort_by=" + tt.sortBy + "&sort_order=" + tt.sortOrder
			req := httptest.NewRequest(http.MethodGet, url, nil)
			rec := httptest.NewRecorder()

			defer func() {
				if r := recover(); r != nil {
					// nil DB panic is acceptable for non-400 cases
					if tt.wantCode == http.StatusBadRequest {
						t.Errorf("expected 400 response, but got panic")
					}
				}
			}()

			handler.ServeHTTP(rec, req)

			if rec.Code != tt.wantCode {
				t.Errorf("expected status %d, got %d", tt.wantCode, rec.Code)
			}
		})
	}
}

func TestItemDashboardParamParsing(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet,
		"/test?item_code=ITEM-001&warehouse=Main&item_group=Products&start=20&sort_by=actual_qty&sort_order=asc",
		nil)

	itemCode := parseStringParam(req, "item_code", "")
	warehouse := parseStringParam(req, "warehouse", "")
	itemGroup := parseStringParam(req, "item_group", "")
	start := parseIntParam(req, "start", 0)
	sortBy := parseStringParam(req, "sort_by", "actual_qty")
	sortOrder := parseStringParam(req, "sort_order", "desc")

	if itemCode != "ITEM-001" {
		t.Errorf("expected item_code 'ITEM-001', got %q", itemCode)
	}
	if warehouse != "Main" {
		t.Errorf("expected warehouse 'Main', got %q", warehouse)
	}
	if itemGroup != "Products" {
		t.Errorf("expected item_group 'Products', got %q", itemGroup)
	}
	if start != 20 {
		t.Errorf("expected start 20, got %d", start)
	}
	if sortBy != "actual_qty" {
		t.Errorf("expected sort_by 'actual_qty', got %q", sortBy)
	}
	if sortOrder != "asc" {
		t.Errorf("expected sort_order 'asc', got %q", sortOrder)
	}
}

func TestItemDashboardDefaultParams(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)

	start := parseIntParam(req, "start", 0)
	sortBy := parseStringParam(req, "sort_by", "actual_qty")
	sortOrder := parseStringParam(req, "sort_order", "desc")

	if start != 0 {
		t.Errorf("expected default start 0, got %d", start)
	}
	if sortBy != "actual_qty" {
		t.Errorf("expected default sort_by 'actual_qty', got %q", sortBy)
	}
	if sortOrder != "desc" {
		t.Errorf("expected default sort_order 'desc', got %q", sortOrder)
	}
}
