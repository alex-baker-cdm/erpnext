package queries

import (
	"testing"
)

func TestGetFilteredBatches_Dedup(t *testing.T) {
	// Test deduplication and sum behavior
	data := [][]interface{}{
		{"BATCH-001", float64(10), "extra1"},
		{"BATCH-002", float64(5), "extra2"},
		{"BATCH-001", float64(15), "extra1"},  // duplicate, should sum
		{"BATCH-003", float64(-5), "extra3"},  // negative qty, should be filtered
	}

	result := GetFilteredBatches(data)

	if len(result) != 2 {
		t.Fatalf("expected 2 batches, got %d", len(result))
	}

	// BATCH-001 should have qty=25
	if result[0][0] != "BATCH-001" {
		t.Errorf("expected first batch BATCH-001, got %v", result[0][0])
	}
	qty := toFloat64(result[0][1])
	if qty != 25 {
		t.Errorf("expected BATCH-001 qty=25, got %v", qty)
	}

	// BATCH-002 should have qty=5
	if result[1][0] != "BATCH-002" {
		t.Errorf("expected second batch BATCH-002, got %v", result[1][0])
	}
	qty2 := toFloat64(result[1][1])
	if qty2 != 5 {
		t.Errorf("expected BATCH-002 qty=5, got %v", qty2)
	}
}

func TestGetFilteredBatches_AllNegative(t *testing.T) {
	data := [][]interface{}{
		{"BATCH-001", float64(-10)},
		{"BATCH-002", float64(-5)},
	}

	result := GetFilteredBatches(data)
	if result != nil && len(result) != 0 {
		t.Errorf("expected empty result for all negative batches, got %d", len(result))
	}
}

func TestGetFilteredBatches_Empty(t *testing.T) {
	result := GetFilteredBatches(nil)
	if result != nil && len(result) != 0 {
		t.Errorf("expected empty result for nil input, got %d", len(result))
	}
}

func TestGetFilteredBatches_ZeroQty(t *testing.T) {
	data := [][]interface{}{
		{"BATCH-001", float64(10)},
		{"BATCH-001", float64(-10)}, // sum to zero
	}

	result := GetFilteredBatches(data)
	if len(result) != 0 {
		t.Errorf("expected 0 batches (zero qty filtered out), got %d", len(result))
	}
}

func TestGetDoctypeWiseFilters(t *testing.T) {
	filters := [][]interface{}{
		{"Warehouse", "company", "=", "TestCo"},
		{"Bin", "item_code", "=", "ITEM-001"},
		{"Warehouse", "is_group", "=", "0"},
		{"Bin", "warehouse", "=", "Main Store"},
	}

	result := GetDoctypeWiseFilters(filters)

	if len(result) != 2 {
		t.Fatalf("expected 2 doctype groups, got %d", len(result))
	}

	if len(result["Warehouse"]) != 2 {
		t.Errorf("expected 2 Warehouse filters, got %d", len(result["Warehouse"]))
	}
	if len(result["Bin"]) != 2 {
		t.Errorf("expected 2 Bin filters, got %d", len(result["Bin"]))
	}
}

func TestGetDoctypeWiseFilters_Empty(t *testing.T) {
	result := GetDoctypeWiseFilters(nil)
	if len(result) != 0 {
		t.Errorf("expected empty map for nil input, got %d", len(result))
	}
}

func TestGetDoctypeWiseFilters_InvalidRow(t *testing.T) {
	filters := [][]interface{}{
		{}, // empty row
		{"Warehouse", "company", "=", "TestCo"},
		{123, "field", "=", "val"}, // non-string doctype
	}

	result := GetDoctypeWiseFilters(filters)
	if len(result) != 1 {
		t.Errorf("expected 1 doctype group, got %d", len(result))
	}
	if len(result["Warehouse"]) != 1 {
		t.Errorf("expected 1 Warehouse filter, got %d", len(result["Warehouse"]))
	}
}

func TestUniqueStrings(t *testing.T) {
	input := []string{"name", "item_name", "name", "description", "item_name"}
	result := uniqueStrings(input)

	if len(result) != 3 {
		t.Fatalf("expected 3 unique strings, got %d: %v", len(result), result)
	}
	if result[0] != "name" || result[1] != "item_name" || result[2] != "description" {
		t.Errorf("unexpected order: %v", result)
	}
}

func TestToFloat64(t *testing.T) {
	tests := []struct {
		input    interface{}
		expected float64
	}{
		{float64(10.5), 10.5},
		{int(42), 42.0},
		{int64(100), 100.0},
		{"3.14", 3.14},
		{"42", 42.0},
		{"not a number", 0.0},
		{nil, 0.0},
	}

	for _, tc := range tests {
		got := toFloat64(tc.input)
		if got != tc.expected {
			t.Errorf("toFloat64(%v) = %v, want %v", tc.input, got, tc.expected)
		}
	}
}
