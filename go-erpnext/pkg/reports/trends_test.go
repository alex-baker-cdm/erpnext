package reports

import (
	"testing"
	"time"
)

func TestValidateFilters_AllPresent(t *testing.T) {
	filters := TrendFilters{
		FiscalYear: "2024",
		BasedOn:    "Item",
		Period:     "Monthly",
		Company:    "Test Co",
	}
	if err := ValidateFilters(filters); err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
}

func TestValidateFilters_MissingFiscalYear(t *testing.T) {
	filters := TrendFilters{
		BasedOn: "Item",
		Period:  "Monthly",
		Company: "Test Co",
	}
	err := ValidateFilters(filters)
	if err == nil {
		t.Error("expected error for missing Fiscal Year")
	}
}

func TestValidateFilters_MissingBasedOn(t *testing.T) {
	filters := TrendFilters{
		FiscalYear: "2024",
		Period:     "Monthly",
		Company:    "Test Co",
	}
	err := ValidateFilters(filters)
	if err == nil {
		t.Error("expected error for missing Based On")
	}
}

func TestValidateFilters_MissingPeriod(t *testing.T) {
	filters := TrendFilters{
		FiscalYear: "2024",
		BasedOn:    "Item",
		Company:    "Test Co",
	}
	err := ValidateFilters(filters)
	if err == nil {
		t.Error("expected error for missing Period")
	}
}

func TestValidateFilters_MissingCompany(t *testing.T) {
	filters := TrendFilters{
		FiscalYear: "2024",
		BasedOn:    "Item",
		Period:     "Monthly",
	}
	err := ValidateFilters(filters)
	if err == nil {
		t.Error("expected error for missing Company")
	}
}

func TestValidateFilters_SameBasedOnAndGroupBy(t *testing.T) {
	filters := TrendFilters{
		FiscalYear: "2024",
		BasedOn:    "Item",
		Period:     "Monthly",
		Company:    "Test Co",
		GroupBy:    "Item",
	}
	err := ValidateFilters(filters)
	if err == nil {
		t.Error("expected error when Based On equals Group By")
	}
}

func TestValidateFilters_DifferentBasedOnAndGroupBy(t *testing.T) {
	filters := TrendFilters{
		FiscalYear: "2024",
		BasedOn:    "Item",
		Period:     "Monthly",
		Company:    "Test Co",
		GroupBy:    "Customer",
	}
	if err := ValidateFilters(filters); err != nil {
		t.Errorf("expected no error for different Based On and Group By, got: %v", err)
	}
}

func TestGetPeriodDateRanges_Monthly(t *testing.T) {
	start := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC)

	ranges := GetPeriodDateRanges("Monthly", start, end)

	if len(ranges) != 12 {
		t.Fatalf("expected 12 monthly ranges, got %d", len(ranges))
	}

	// First month: Jan 1 to Jan 31
	if ranges[0][0] != start {
		t.Errorf("first range start: expected %v, got %v", start, ranges[0][0])
	}
	expectedEnd := time.Date(2024, 1, 31, 0, 0, 0, 0, time.UTC)
	if ranges[0][1] != expectedEnd {
		t.Errorf("first range end: expected %v, got %v", expectedEnd, ranges[0][1])
	}

	// Last month ends at year end
	if ranges[11][1] != end {
		t.Errorf("last range end: expected %v, got %v", end, ranges[11][1])
	}
}

func TestGetPeriodDateRanges_Quarterly(t *testing.T) {
	start := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC)

	ranges := GetPeriodDateRanges("Quarterly", start, end)

	if len(ranges) != 4 {
		t.Fatalf("expected 4 quarterly ranges, got %d", len(ranges))
	}

	// Q1: Jan 1 to Mar 31
	expectedQ1End := time.Date(2024, 3, 31, 0, 0, 0, 0, time.UTC)
	if ranges[0][1] != expectedQ1End {
		t.Errorf("Q1 end: expected %v, got %v", expectedQ1End, ranges[0][1])
	}

	// Q2: Apr 1 to Jun 30
	expectedQ2Start := time.Date(2024, 4, 1, 0, 0, 0, 0, time.UTC)
	expectedQ2End := time.Date(2024, 6, 30, 0, 0, 0, 0, time.UTC)
	if ranges[1][0] != expectedQ2Start {
		t.Errorf("Q2 start: expected %v, got %v", expectedQ2Start, ranges[1][0])
	}
	if ranges[1][1] != expectedQ2End {
		t.Errorf("Q2 end: expected %v, got %v", expectedQ2End, ranges[1][1])
	}
}

func TestGetPeriodDateRanges_HalfYearly(t *testing.T) {
	start := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC)

	ranges := GetPeriodDateRanges("Half-Yearly", start, end)

	if len(ranges) != 2 {
		t.Fatalf("expected 2 half-yearly ranges, got %d", len(ranges))
	}

	expectedH1End := time.Date(2024, 6, 30, 0, 0, 0, 0, time.UTC)
	if ranges[0][1] != expectedH1End {
		t.Errorf("H1 end: expected %v, got %v", expectedH1End, ranges[0][1])
	}
}

func TestGetPeriodDateRanges_Yearly(t *testing.T) {
	start := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC)

	ranges := GetPeriodDateRanges("Yearly", start, end)

	if len(ranges) != 1 {
		t.Fatalf("expected 1 yearly range, got %d", len(ranges))
	}
	if ranges[0][0] != start {
		t.Errorf("yearly start: expected %v, got %v", start, ranges[0][0])
	}
	if ranges[0][1] != end {
		t.Errorf("yearly end: expected %v, got %v", end, ranges[0][1])
	}
}

func TestBasedWiseColumnsQuery_Item(t *testing.T) {
	details := BasedWiseColumnsQuery("Item", "Sales Order")

	cols, ok := details["based_on_cols"].([]string)
	if !ok {
		t.Fatal("based_on_cols should be []string")
	}

	// Item has 2 base cols + 1 currency col = 3
	if len(cols) != 3 {
		t.Errorf("expected 3 columns for Item, got %d", len(cols))
	}

	sel, _ := details["based_on_select"].(string)
	if sel == "" {
		t.Error("based_on_select should not be empty")
	}

	groupBy, _ := details["based_on_group_by"].(string)
	if groupBy != "t2.item_code" {
		t.Errorf("expected group_by 't2.item_code', got %q", groupBy)
	}
}

func TestBasedWiseColumnsQuery_Supplier(t *testing.T) {
	details := BasedWiseColumnsQuery("Supplier", "Purchase Order")

	cols, ok := details["based_on_cols"].([]string)
	if !ok {
		t.Fatal("based_on_cols should be []string")
	}

	// Supplier has 3 base cols + 1 currency col = 4
	if len(cols) != 4 {
		t.Errorf("expected 4 columns for Supplier, got %d", len(cols))
	}

	addlTables, _ := details["addl_tables"].(string)
	if addlTables == "" {
		t.Error("Supplier should have additional tables")
	}

	relCond, _ := details["addl_tables_relational_cond"].(string)
	if relCond == "" {
		t.Error("Supplier should have relational condition")
	}
}

func TestBasedWiseColumnsQuery_Customer(t *testing.T) {
	details := BasedWiseColumnsQuery("Customer", "Sales Order")

	cols, ok := details["based_on_cols"].([]string)
	if !ok {
		t.Fatal("based_on_cols should be []string")
	}

	// Customer has 3 base cols + 1 currency col = 4
	if len(cols) != 4 {
		t.Errorf("expected 4 columns for Customer, got %d", len(cols))
	}

	groupBy, _ := details["based_on_group_by"].(string)
	if groupBy != "t1.customer" {
		t.Errorf("expected group_by 't1.customer', got %q", groupBy)
	}
}

func TestBasedWiseColumnsQuery_CustomerQuotation(t *testing.T) {
	details := BasedWiseColumnsQuery("Customer", "Quotation")

	groupBy, _ := details["based_on_group_by"].(string)
	if groupBy != "t1.party_name" {
		t.Errorf("expected group_by 't1.party_name' for Quotation, got %q", groupBy)
	}
}

func TestCalculateTotalRow(t *testing.T) {
	columns := []string{
		"Item:Link/Item:120",
		"Jan (Qty):Float:120",
		"Jan (Amt):Currency/currency:120",
		"Total(Qty):Float:120",
		"Total(Amt):Currency/currency:120",
	}

	data := [][]interface{}{
		{"Widget A", 10.0, 100.0, 10.0, 100.0},
		{"Widget B", 20.0, 200.0, 20.0, 200.0},
		{"Widget C", 5.0, 50.0, 5.0, 50.0},
	}

	total := CalculateTotalRow(data, columns)

	if len(total) != len(columns) {
		t.Fatalf("total row length: expected %d, got %d", len(columns), len(total))
	}

	if total[0] != "'Total'" {
		t.Errorf("first column should be 'Total', got %v", total[0])
	}

	// Sum of Qty column (index 1): 10 + 20 + 5 = 35
	if total[1] != 35.0 {
		t.Errorf("total qty: expected 35, got %v", total[1])
	}

	// Sum of Amt column (index 2): 100 + 200 + 50 = 350
	if total[2] != 350.0 {
		t.Errorf("total amt: expected 350, got %v", total[2])
	}

	// Total(Qty) sum: 35
	if total[3] != 35.0 {
		t.Errorf("total total_qty: expected 35, got %v", total[3])
	}

	// Total(Amt) sum: 350
	if total[4] != 350.0 {
		t.Errorf("total total_amt: expected 350, got %v", total[4])
	}
}

func TestCalculateTotalRow_EmptyData(t *testing.T) {
	columns := []string{
		"Item:Link/Item:120",
		"Qty:Float:120",
	}

	total := CalculateTotalRow([][]interface{}{}, columns)

	if len(total) != 2 {
		t.Fatalf("total row length: expected 2, got %d", len(total))
	}

	if total[0] != "'Total'" {
		t.Errorf("first column should be 'Total', got %v", total[0])
	}

	// Float column should be 0
	if total[1] != 0.0 {
		t.Errorf("expected 0, got %v", total[1])
	}
}

func TestCalculateTotalRow_NilValues(t *testing.T) {
	columns := []string{
		"Item:Link/Item:120",
		"Qty:Float:120",
		"Amt:Currency/currency:120",
	}

	data := [][]interface{}{
		{"A", 10.0, nil},
		{"B", nil, 200.0},
	}

	total := CalculateTotalRow(data, columns)

	if total[1] != 10.0 {
		t.Errorf("qty total: expected 10, got %v", total[1])
	}
	if total[2] != 200.0 {
		t.Errorf("amt total: expected 200, got %v", total[2])
	}
}
