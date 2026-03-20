package dashboard

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCalculateOccupancy(t *testing.T) {
	tests := []struct {
		name          string
		actualQty     float64
		stockCapacity float64
		want          float64
	}{
		{"full capacity", 100, 100, 100},
		{"half capacity", 50, 100, 50},
		{"zero capacity", 50, 0, 0},
		{"empty warehouse", 0, 100, 0},
		{"over capacity", 150, 100, 150},
		{"quarter capacity", 25, 100, 25},
		{"small fraction", 1, 1000, 0}, // Rounds to 0 with precision 0
		{"typical usage", 75, 200, 38}, // 37.5 rounds to 38 with Flt(..., 0)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculateOccupancy(tt.actualQty, tt.stockCapacity)
			if got != tt.want {
				t.Errorf("CalculateOccupancy(%f, %f) = %f, want %f",
					tt.actualQty, tt.stockCapacity, got, tt.want)
			}
		})
	}
}

func TestWarehouseCapacitySortValidation(t *testing.T) {
	tests := []struct {
		name      string
		sortBy    string
		sortOrder string
		wantCode  int
	}{
		{"valid sort", "stock_capacity", "desc", http.StatusInternalServerError}, // No DB
		{"invalid sort_by", "malicious_col", "desc", http.StatusBadRequest},
		{"invalid sort_order", "stock_capacity", "INVALID", http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := MakeHandler(nil, WarehouseCapacityGetData)
			url := "/test?sort_by=" + tt.sortBy + "&sort_order=" + tt.sortOrder
			req := httptest.NewRequest(http.MethodGet, url, nil)
			rec := httptest.NewRecorder()

			defer func() {
				if r := recover(); r != nil {
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

func TestSortCapacityData(t *testing.T) {
	data := []warehouseCapacityRow{
		{ItemCode: "A", PercentOccupied: 30},
		{ItemCode: "B", PercentOccupied: 80},
		{ItemCode: "C", PercentOccupied: 50},
	}

	sortCapacityData(data, "desc")

	if data[0].ItemCode != "B" {
		t.Errorf("expected first item 'B' (80%%), got %q (%f%%)", data[0].ItemCode, data[0].PercentOccupied)
	}
	if data[1].ItemCode != "C" {
		t.Errorf("expected second item 'C' (50%%), got %q (%f%%)", data[1].ItemCode, data[1].PercentOccupied)
	}
	if data[2].ItemCode != "A" {
		t.Errorf("expected third item 'A' (30%%), got %q (%f%%)", data[2].ItemCode, data[2].PercentOccupied)
	}
}

func TestSortCapacityDataAsc(t *testing.T) {
	data := []warehouseCapacityRow{
		{ItemCode: "A", PercentOccupied: 30},
		{ItemCode: "B", PercentOccupied: 80},
		{ItemCode: "C", PercentOccupied: 50},
	}

	sortCapacityData(data, "asc")

	if data[0].ItemCode != "A" {
		t.Errorf("expected first item 'A' (30%%), got %q", data[0].ItemCode)
	}
	if data[1].ItemCode != "C" {
		t.Errorf("expected second item 'C' (50%%), got %q", data[1].ItemCode)
	}
	if data[2].ItemCode != "B" {
		t.Errorf("expected third item 'B' (80%%), got %q", data[2].ItemCode)
	}
}

func TestSortCapacityDataEmpty(t *testing.T) {
	var data []warehouseCapacityRow
	// Should not panic on empty slice
	sortCapacityData(data, "desc")
	if len(data) != 0 {
		t.Errorf("expected empty data, got %d items", len(data))
	}
}

func TestSortCapacityDataSingleItem(t *testing.T) {
	data := []warehouseCapacityRow{
		{ItemCode: "A", PercentOccupied: 50},
	}
	sortCapacityData(data, "desc")
	if data[0].ItemCode != "A" {
		t.Errorf("expected item 'A', got %q", data[0].ItemCode)
	}
}
