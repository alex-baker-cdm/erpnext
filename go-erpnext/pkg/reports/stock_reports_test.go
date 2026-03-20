package reports

import (
	"math"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Stock Ledger Report Tests
// ---------------------------------------------------------------------------

func TestStockLedgerReport_Columns(t *testing.T) {
	r := &StockLedgerReport{}
	filters := map[string]interface{}{}
	columns := r.getColumns(filters)

	if len(columns) == 0 {
		t.Fatal("expected non-empty columns")
	}

	// Verify first column
	if columns[0].Label != "Date" || columns[0].Fieldname != "date" || columns[0].Fieldtype != "Datetime" {
		t.Errorf("unexpected first column: %+v", columns[0])
	}

	// Verify valuation field type defaults to Currency
	found := false
	for _, col := range columns {
		if col.Fieldname == "valuation_rate" {
			if col.Fieldtype != "Currency" {
				t.Errorf("expected Currency fieldtype for valuation_rate, got %s", col.Fieldtype)
			}
			found = true
			break
		}
	}
	if !found {
		t.Error("valuation_rate column not found")
	}
}

func TestStockLedgerReport_ColumnsCustomValuation(t *testing.T) {
	r := &StockLedgerReport{}
	filters := map[string]interface{}{
		"valuation_field_type": "Float",
	}
	columns := r.getColumns(filters)

	for _, col := range columns {
		if col.Fieldname == "valuation_rate" {
			if col.Fieldtype != "Float" {
				t.Errorf("expected Float fieldtype for valuation_rate, got %s", col.Fieldtype)
			}
			return
		}
	}
	t.Error("valuation_rate column not found")
}

func TestStockLedgerReport_ExecuteNilDB(t *testing.T) {
	r := &StockLedgerReport{}
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

func TestStockLedgerReport_OpeningBalanceNilDB(t *testing.T) {
	r := &StockLedgerReport{}
	filters := map[string]interface{}{
		"item_code": "ITEM-001",
		"warehouse": "Stores - TC",
		"from_date": "2024-01-01",
	}
	opening := r.getOpeningBalance(filters, nil)
	if opening == nil {
		t.Fatal("expected non-nil opening balance")
	}
	if opening["item_code"] != "'Opening'" {
		t.Errorf("unexpected item_code: %v", opening["item_code"])
	}
}

func TestStockLedgerReport_InOutQtyComputation(t *testing.T) {
	// Test the in/out qty computation logic
	tests := []struct {
		actualQty float64
		expInQty  float64
		expOutQty float64
	}{
		{10.0, 10.0, 0.0},
		{-5.0, 0.0, -5.0},
		{0.0, 0.0, 0.0},
		{0.5, 0.5, 0.0},
		{-0.25, 0.0, -0.25},
	}

	for _, tc := range tests {
		inQty := math.Max(tc.actualQty, 0)
		outQty := math.Min(tc.actualQty, 0)
		if inQty != tc.expInQty {
			t.Errorf("actualQty=%f: expected in_qty=%f, got %f", tc.actualQty, tc.expInQty, inQty)
		}
		if outQty != tc.expOutQty {
			t.Errorf("actualQty=%f: expected out_qty=%f, got %f", tc.actualQty, tc.expOutQty, outQty)
		}
	}
}

func TestStockLedgerReport_InOutRateComputation(t *testing.T) {
	// in_out_rate = stock_value_difference / actual_qty
	tests := []struct {
		svd       float64
		actualQty float64
		expected  float64
	}{
		{100.0, 10.0, 10.0},
		{-50.0, -5.0, 10.0},
		{0.0, 10.0, 0.0},
	}

	for _, tc := range tests {
		if tc.actualQty == 0 {
			continue
		}
		rate := tc.svd / tc.actualQty
		if math.Abs(rate-tc.expected) > 0.000001 {
			t.Errorf("svd=%f, qty=%f: expected rate=%f, got %f", tc.svd, tc.actualQty, tc.expected, rate)
		}
	}
}

// ---------------------------------------------------------------------------
// Stock Projected Qty Report Tests
// ---------------------------------------------------------------------------

func TestStockProjectedQtyReport_Columns(t *testing.T) {
	r := &StockProjectedQtyReport{}
	columns := r.getColumns()

	if len(columns) != 20 {
		t.Errorf("expected 20 columns, got %d", len(columns))
	}

	// Verify projected_qty column exists
	found := false
	for _, col := range columns {
		if col.Fieldname == "projected_qty" {
			found = true
			if col.Fieldtype != "Float" {
				t.Errorf("expected Float fieldtype for projected_qty, got %s", col.Fieldtype)
			}
			break
		}
	}
	if !found {
		t.Error("projected_qty column not found")
	}
}

func TestStockProjectedQtyReport_ExecuteNilDB(t *testing.T) {
	r := &StockProjectedQtyReport{}
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
// Item Shortage Report Tests
// ---------------------------------------------------------------------------

func TestItemShortageReport_Columns(t *testing.T) {
	r := &ItemShortageReport{}
	columns := r.getColumns()

	if len(columns) != 11 {
		t.Errorf("expected 11 columns, got %d", len(columns))
	}

	// Verify first column is warehouse
	if columns[0].Fieldname != "warehouse" {
		t.Errorf("expected first column to be warehouse, got %s", columns[0].Fieldname)
	}
}

func TestItemShortageReport_ChartData(t *testing.T) {
	r := &ItemShortageReport{}

	data := []map[string]interface{}{
		{"item_code": "ITEM-001", "projected_qty": -10.0},
		{"item_code": "ITEM-002", "projected_qty": -5.0},
	}

	chart := r.getChartData(data)
	if chart == nil {
		t.Fatal("expected non-nil chart data")
	}

	chartData, ok := chart["data"].(map[string]interface{})
	if !ok {
		t.Fatal("expected chart data to be a map")
	}

	labels, ok := chartData["labels"].([]interface{})
	if !ok {
		t.Fatal("expected labels to be a slice")
	}

	if len(labels) != 2 {
		t.Errorf("expected 2 labels, got %d", len(labels))
	}
}

func TestItemShortageReport_ChartDataLimit(t *testing.T) {
	r := &ItemShortageReport{}

	// Create 15 items to test the limit of 10
	data := make([]map[string]interface{}, 15)
	for i := 0; i < 15; i++ {
		data[i] = map[string]interface{}{"item_code": "ITEM", "projected_qty": -1.0}
	}

	chart := r.getChartData(data)
	chartData := chart["data"].(map[string]interface{})
	labels := chartData["labels"].([]interface{})
	if len(labels) != 10 {
		t.Errorf("expected 10 labels (max), got %d", len(labels))
	}
}

func TestItemShortageReport_ExecuteNilDB(t *testing.T) {
	r := &ItemShortageReport{}
	result, err := r.Execute(map[string]interface{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}

// ---------------------------------------------------------------------------
// Reserved Stock Report Tests
// ---------------------------------------------------------------------------

func TestReservedStockReport_Columns(t *testing.T) {
	r := &ReservedStockReport{}
	columns := r.getColumns()

	if len(columns) != 16 {
		t.Errorf("expected 16 columns, got %d", len(columns))
	}
}

func TestReservedStockReport_RequiredFilters(t *testing.T) {
	r := &ReservedStockReport{}
	_, err := r.Execute(map[string]interface{}{})
	if err == nil {
		t.Fatal("expected error for missing required filters")
	}

	_, err = r.Execute(map[string]interface{}{
		"company":   "Test Co",
		"from_date": "2024-01-01",
	})
	if err == nil {
		t.Fatal("expected error for missing to_date")
	}

	_, err = r.Execute(map[string]interface{}{
		"company":   "Test Co",
		"from_date": "2024-01-01",
		"to_date":   "2024-12-31",
	})
	if err != nil {
		t.Fatalf("unexpected error with all required filters: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Stock Analytics Report Tests
// ---------------------------------------------------------------------------

func TestStockAnalyticsReport_PeriodDateRanges(t *testing.T) {
	r := &StockAnalyticsReport{}

	filters := map[string]interface{}{
		"from_date": "2024-01-01",
		"to_date":   "2024-03-31",
		"range":     "Monthly",
	}

	ranges := r.getPeriodDateRanges(filters)
	if len(ranges) != 3 {
		t.Errorf("expected 3 monthly ranges, got %d", len(ranges))
	}
}

func TestStockAnalyticsReport_GetPeriod(t *testing.T) {
	r := &StockAnalyticsReport{}

	tests := []struct {
		dateStr  string
		rangeStr string
		expected string
	}{
		{"2024-01-15", "Monthly", "Jan 2024"},
		{"2024-06-15", "Monthly", "Jun 2024"},
		{"2024-03-31", "Quarterly", "Quarter 1 2024"},
		{"2024-06-30", "Quarterly", "Quarter 2 2024"},
		{"2024-01-15", "Yearly", "2024"},
	}

	for _, tc := range tests {
		date, _ := time.Parse("2006-01-02", tc.dateStr)
		period := r.getPeriod(date, map[string]interface{}{"range": tc.rangeStr})
		if period != tc.expected {
			t.Errorf("date=%s, range=%s: expected %q, got %q", tc.dateStr, tc.rangeStr, tc.expected, period)
		}
	}
}

func TestStockAnalyticsReport_ExecuteNilDB(t *testing.T) {
	r := &StockAnalyticsReport{}
	result, err := r.Execute(map[string]interface{}{
		"from_date": "2024-01-01",
		"to_date":   "2024-12-31",
		"range":     "Monthly",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}

// ---------------------------------------------------------------------------
// Serial and Batch Summary Report Tests
// ---------------------------------------------------------------------------

func TestSerialAndBatchSummaryReport_Columns(t *testing.T) {
	r := &SerialAndBatchSummaryReport{}

	// Without filters - should have all columns
	columns := r.getColumns(map[string]interface{}{})
	hasVoucherNo := false
	hasItemCode := false
	hasWarehouse := false
	for _, col := range columns {
		if col.Fieldname == "voucher_no" {
			hasVoucherNo = true
		}
		if col.Fieldname == "item_code" {
			hasItemCode = true
		}
		if col.Fieldname == "warehouse" {
			hasWarehouse = true
		}
	}
	if !hasVoucherNo {
		t.Error("expected voucher_no column")
	}
	if !hasItemCode {
		t.Error("expected item_code column")
	}
	if !hasWarehouse {
		t.Error("expected warehouse column")
	}

	// With voucher_no filter - should NOT have voucher_no/voucher_type columns
	columnsFiltered := r.getColumns(map[string]interface{}{"voucher_no": "INV-001"})
	for _, col := range columnsFiltered {
		if col.Fieldname == "voucher_no" {
			t.Error("did not expect voucher_no column when voucher_no filter is set")
		}
	}
}

func TestSerialAndBatchSummaryReport_ExecuteNilDB(t *testing.T) {
	r := &SerialAndBatchSummaryReport{}
	result, err := r.Execute(map[string]interface{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}

// ---------------------------------------------------------------------------
// Serial No Ledger Report Tests
// ---------------------------------------------------------------------------

func TestSerialNoLedgerReport_Columns(t *testing.T) {
	r := &SerialNoLedgerReport{}
	columns := r.getColumns()

	if len(columns) != 12 {
		t.Errorf("expected 12 columns, got %d", len(columns))
	}

	// Verify column names
	expectedFields := []string{"posting_date", "posting_time", "voucher_type", "voucher_no",
		"company", "warehouse", "status", "serial_no", "valuation_rate", "qty", "party_type", "party"}

	for i, expected := range expectedFields {
		if columns[i].Fieldname != expected {
			t.Errorf("column %d: expected fieldname %q, got %q", i, expected, columns[i].Fieldname)
		}
	}
}

func TestSerialNoLedgerReport_VoucherTypeClassification(t *testing.T) {
	// Test buying vs selling voucher type classification
	buyingTypes := []string{"Purchase Invoice", "Purchase Receipt", "Subcontracting Receipt"}
	sellingTypes := []string{"Sales Invoice", "Delivery Note"}
	otherTypes := []string{"Stock Entry", "Stock Reconciliation"}

	for _, vt := range buyingTypes {
		if !buyingVoucherTypes[vt] {
			t.Errorf("expected %q to be a buying voucher type", vt)
		}
	}

	for _, vt := range sellingTypes {
		if !sellingVoucherTypes[vt] {
			t.Errorf("expected %q to be a selling voucher type", vt)
		}
	}

	for _, vt := range otherTypes {
		if buyingVoucherTypes[vt] || sellingVoucherTypes[vt] {
			t.Errorf("expected %q to be neither buying nor selling", vt)
		}
	}
}

func TestSerialNoLedgerReport_ExecuteNilDB(t *testing.T) {
	r := &SerialNoLedgerReport{}
	result, err := r.Execute(map[string]interface{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}

// ---------------------------------------------------------------------------
// Warehouse Wise Stock Balance Report Tests
// ---------------------------------------------------------------------------

func TestWarehouseWiseStockBalanceReport_Columns(t *testing.T) {
	r := &WarehouseWiseStockBalanceReport{}

	// Without show_disabled_warehouses filter
	columns := r.getColumns(map[string]interface{}{})
	if len(columns) != 2 {
		t.Errorf("expected 2 columns, got %d", len(columns))
	}

	// With show_disabled_warehouses filter
	columns = r.getColumns(map[string]interface{}{"show_disabled_warehouses": 1})
	if len(columns) != 3 {
		t.Errorf("expected 3 columns with disabled filter, got %d", len(columns))
	}
}

func TestWarehouseWiseStockBalanceReport_SetBalanceInParent(t *testing.T) {
	r := &WarehouseWiseStockBalanceReport{}

	warehouses := []map[string]interface{}{
		{"name": "All Warehouses", "parent_warehouse": "", "is_group": int64(1), "indent": 0.0, "stock_balance": 0.0},
		{"name": "Stores - TC", "parent_warehouse": "All Warehouses", "is_group": int64(0), "indent": 1.0, "stock_balance": 100.0},
		{"name": "Finished Goods - TC", "parent_warehouse": "All Warehouses", "is_group": int64(0), "indent": 1.0, "stock_balance": 200.0},
	}

	r.setBalanceInParent(warehouses)

	parentBalance := warehouses[0]["stock_balance"].(float64)
	if parentBalance != 300.0 {
		t.Errorf("expected parent balance 300.0, got %f", parentBalance)
	}
}

func TestWarehouseWiseStockBalanceReport_ExecuteNilDB(t *testing.T) {
	r := &WarehouseWiseStockBalanceReport{}
	result, err := r.Execute(map[string]interface{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}

// ---------------------------------------------------------------------------
// Helper: scanRowsToMaps / scanRowsToSlices (tested via mock rows)
// ---------------------------------------------------------------------------

// mockRows implements the interface needed by scanRowsToMaps/scanRowsToSlices.
type mockRows struct {
	cols    []string
	data    [][]interface{}
	cursor  int
}

func (m *mockRows) Columns() ([]string, error) {
	return m.cols, nil
}

func (m *mockRows) Next() bool {
	if m.cursor < len(m.data) {
		m.cursor++
		return true
	}
	return false
}

func (m *mockRows) Scan(dest ...interface{}) error {
	row := m.data[m.cursor-1]
	for i, v := range row {
		ptr := dest[i].(*interface{})
		*ptr = v
	}
	return nil
}

func TestScanRowsToMaps(t *testing.T) {
	rows := &mockRows{
		cols: []string{"item_code", "qty"},
		data: [][]interface{}{
			{"ITEM-001", 10.0},
			{"ITEM-002", 20.0},
		},
	}

	result, err := scanRowsToMaps(rows)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(result))
	}

	if result[0]["item_code"] != "ITEM-001" {
		t.Errorf("expected ITEM-001, got %v", result[0]["item_code"])
	}
	if result[1]["qty"] != 20.0 {
		t.Errorf("expected 20.0, got %v", result[1]["qty"])
	}
}

func TestScanRowsToSlices(t *testing.T) {
	rows := &mockRows{
		cols: []string{"name", "amount"},
		data: [][]interface{}{
			{"DN-001", 500.0},
		},
	}

	result, err := scanRowsToSlices(rows)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) != 1 {
		t.Fatalf("expected 1 row, got %d", len(result))
	}
	if result[0][0] != "DN-001" {
		t.Errorf("expected DN-001, got %v", result[0][0])
	}
}

func TestScanRowsToMaps_ByteConversion(t *testing.T) {
	rows := &mockRows{
		cols: []string{"name"},
		data: [][]interface{}{
			{[]byte("hello")},
		},
	}

	result, err := scanRowsToMaps(rows)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result[0]["name"] != "hello" {
		t.Errorf("expected 'hello', got %v", result[0]["name"])
	}
}
