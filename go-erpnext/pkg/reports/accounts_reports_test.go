package reports

import (
	"testing"
)

// ---------------------------------------------------------------------------
// Sales Register Report Tests
// ---------------------------------------------------------------------------

func TestSalesRegisterReport_Columns(t *testing.T) {
	r := &SalesRegisterReport{}
	columns := r.getColumns(nil, nil)

	if len(columns) == 0 {
		t.Fatal("expected non-empty columns")
	}

	// Check base columns exist
	expectedBase := []string{"voucher_type", "voucher_no", "posting_date", "customer"}
	for _, expected := range expectedBase {
		found := false
		for _, col := range columns {
			if col.Fieldname == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected column %q not found", expected)
		}
	}
}

func TestSalesRegisterReport_ColumnsDynamic(t *testing.T) {
	r := &SalesRegisterReport{}
	incomeAccounts := []string{"Sales - TC", "Service Revenue - TC"}
	taxAccounts := []string{"VAT - TC"}

	columns := r.getColumns(incomeAccounts, taxAccounts)

	// Should have income columns
	foundSales := false
	foundVAT := false
	for _, col := range columns {
		if col.Fieldname == scrubAccountName("Sales - TC") {
			foundSales = true
		}
		if col.Fieldname == scrubAccountName("VAT - TC") {
			foundVAT = true
		}
	}
	if !foundSales {
		t.Error("expected dynamic income account column for Sales - TC")
	}
	if !foundVAT {
		t.Error("expected dynamic tax account column for VAT - TC")
	}
}

func TestSalesRegisterReport_ExecuteNilDB(t *testing.T) {
	r := &SalesRegisterReport{}
	result, err := r.Execute(map[string]interface{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if len(result.Columns) == 0 {
		t.Error("expected non-empty columns")
	}
}

// ---------------------------------------------------------------------------
// Purchase Register Report Tests
// ---------------------------------------------------------------------------

func TestPurchaseRegisterReport_Columns(t *testing.T) {
	r := &PurchaseRegisterReport{}
	columns := r.getColumns(nil, nil)

	if len(columns) == 0 {
		t.Fatal("expected non-empty columns")
	}

	expectedBase := []string{"voucher_type", "voucher_no", "posting_date", "supplier_id"}
	for _, expected := range expectedBase {
		found := false
		for _, col := range columns {
			if col.Fieldname == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected column %q not found", expected)
		}
	}
}

func TestPurchaseRegisterReport_ExecuteNilDB(t *testing.T) {
	r := &PurchaseRegisterReport{}
	result, err := r.Execute(map[string]interface{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}

// ---------------------------------------------------------------------------
// Item Wise Sales Register Report Tests
// ---------------------------------------------------------------------------

func TestItemWiseSalesRegisterReport_Columns(t *testing.T) {
	r := &ItemWiseSalesRegisterReport{}
	columns := r.getColumns(nil)

	if len(columns) == 0 {
		t.Fatal("expected non-empty columns")
	}

	expectedFields := []string{"item_code", "item_name", "item_group", "posting_date", "customer"}
	for _, expected := range expectedFields {
		found := false
		for _, col := range columns {
			if col.Fieldname == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected column %q not found", expected)
		}
	}
}

func TestItemWiseSalesRegisterReport_ExecuteNilDB(t *testing.T) {
	r := &ItemWiseSalesRegisterReport{}
	result, err := r.Execute(map[string]interface{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}

// ---------------------------------------------------------------------------
// Item Wise Purchase Register Report Tests
// ---------------------------------------------------------------------------

func TestItemWisePurchaseRegisterReport_Columns(t *testing.T) {
	r := &ItemWisePurchaseRegisterReport{}
	columns := r.getColumns(nil)

	if len(columns) == 0 {
		t.Fatal("expected non-empty columns")
	}

	expectedFields := []string{"item_code", "item_name", "item_group", "posting_date", "supplier"}
	for _, expected := range expectedFields {
		found := false
		for _, col := range columns {
			if col.Fieldname == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected column %q not found", expected)
		}
	}
}

func TestItemWisePurchaseRegisterReport_ExecuteNilDB(t *testing.T) {
	r := &ItemWisePurchaseRegisterReport{}
	result, err := r.Execute(map[string]interface{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}

// ---------------------------------------------------------------------------
// Delivered Items To Be Billed Report Tests
// ---------------------------------------------------------------------------

func TestDeliveredItemsToBeBilledReport_Columns(t *testing.T) {
	r := &DeliveredItemsToBeBilledReport{}
	columns := r.getColumns()

	if len(columns) != 12 {
		t.Errorf("expected 12 columns, got %d", len(columns))
	}

	// First column fieldname is "name" with label "Delivery Note"
	if columns[0].Fieldname != "name" || columns[0].Label != "Delivery Note" {
		t.Errorf("expected first column to be name/Delivery Note, got %s/%s", columns[0].Fieldname, columns[0].Label)
	}
}

func TestDeliveredItemsToBeBilledReport_ExecuteNilDB(t *testing.T) {
	r := &DeliveredItemsToBeBilledReport{}
	result, err := r.Execute(map[string]interface{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if len(result.Columns) == 0 {
		t.Error("expected non-empty columns")
	}
}

// ---------------------------------------------------------------------------
// Billed Items To Be Received Report Tests
// ---------------------------------------------------------------------------

func TestBilledItemsToBeReceivedReport_Columns(t *testing.T) {
	r := &BilledItemsToBeReceivedReport{}
	columns := r.getColumns()

	if len(columns) != 10 {
		t.Errorf("expected 10 columns, got %d", len(columns))
	}

	// First column fieldname is "name" with label "Purchase Invoice"
	if columns[0].Fieldname != "name" || columns[0].Label != "Purchase Invoice" {
		t.Errorf("expected first column to be name/Purchase Invoice, got %s/%s", columns[0].Fieldname, columns[0].Label)
	}
}

func TestBilledItemsToBeReceivedReport_ExecuteNilDB(t *testing.T) {
	r := &BilledItemsToBeReceivedReport{}
	result, err := r.Execute(map[string]interface{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if len(result.Columns) == 0 {
		t.Error("expected non-empty columns")
	}
}

// ---------------------------------------------------------------------------
// Utility function tests
// ---------------------------------------------------------------------------

func TestScrubAccountName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Sales - TC", "sales___tc"},
		{"VAT 15% - TC", "vat_15___tc"},
		{"Cost of Goods Sold", "cost_of_goods_sold"},
		{"", ""},
		{"Simple", "simple"},
		{"Multiple   Spaces", "multiple___spaces"},
	}

	for _, tc := range tests {
		result := scrubAccountName(tc.input)
		if result != tc.expected {
			t.Errorf("scrubAccountName(%q): expected %q, got %q", tc.input, tc.expected, result)
		}
	}
}

func TestAppendUnique(t *testing.T) {
	tests := []struct {
		slice    []string
		val      string
		expected int
	}{
		{nil, "a", 1},
		{[]string{"a"}, "a", 1},
		{[]string{"a"}, "b", 2},
		{[]string{"a", "b"}, "c", 3},
		{[]string{"a", "b", "c"}, "b", 3},
	}

	for _, tc := range tests {
		result := appendUnique(tc.slice, tc.val)
		if len(result) != tc.expected {
			t.Errorf("appendUnique(%v, %q): expected len %d, got %d", tc.slice, tc.val, tc.expected, len(result))
		}
	}
}
