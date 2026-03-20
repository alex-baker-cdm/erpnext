package reports

import (
	"testing"
	"time"
)

func TestParseTrendFilters(t *testing.T) {
	filters := map[string]interface{}{
		"fiscal_year":    "2024",
		"based_on":       "Item",
		"period":         "Monthly",
		"company":        "Test Co",
		"group_by":       "Customer",
		"period_based_on": "posting_date",
	}

	tf := parseTrendFilters(filters)

	if tf.FiscalYear != "2024" {
		t.Errorf("FiscalYear: expected '2024', got %q", tf.FiscalYear)
	}
	if tf.BasedOn != "Item" {
		t.Errorf("BasedOn: expected 'Item', got %q", tf.BasedOn)
	}
	if tf.Period != "Monthly" {
		t.Errorf("Period: expected 'Monthly', got %q", tf.Period)
	}
	if tf.Company != "Test Co" {
		t.Errorf("Company: expected 'Test Co', got %q", tf.Company)
	}
	if tf.GroupBy != "Customer" {
		t.Errorf("GroupBy: expected 'Customer', got %q", tf.GroupBy)
	}
	if tf.PeriodBasedOn != "posting_date" {
		t.Errorf("PeriodBasedOn: expected 'posting_date', got %q", tf.PeriodBasedOn)
	}
}

func TestParseTrendFilters_Empty(t *testing.T) {
	tf := parseTrendFilters(map[string]interface{}{})

	if tf.FiscalYear != "" {
		t.Errorf("expected empty FiscalYear, got %q", tf.FiscalYear)
	}
	if tf.BasedOn != "" {
		t.Errorf("expected empty BasedOn, got %q", tf.BasedOn)
	}
}

func TestGetPeriodWiseColumns_Monthly(t *testing.T) {
	dt := [2]time.Time{
		time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2024, 1, 31, 0, 0, 0, 0, time.UTC),
	}
	cols := getPeriodWiseColumns(dt, "Monthly")

	if len(cols) != 2 {
		t.Fatalf("expected 2 columns, got %d", len(cols))
	}
	if cols[0] != "Jan (Qty):Float:120" {
		t.Errorf("expected 'Jan (Qty):Float:120', got %q", cols[0])
	}
	if cols[1] != "Jan (Amt):Currency/currency:120" {
		t.Errorf("expected 'Jan (Amt):Currency/currency:120', got %q", cols[1])
	}
}

func TestGetPeriodWiseColumns_Quarterly(t *testing.T) {
	dt := [2]time.Time{
		time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2024, 3, 31, 0, 0, 0, 0, time.UTC),
	}
	cols := getPeriodWiseColumns(dt, "Quarterly")

	if len(cols) != 2 {
		t.Fatalf("expected 2 columns, got %d", len(cols))
	}
	if cols[0] != "Jan-Mar (Qty):Float:120" {
		t.Errorf("expected 'Jan-Mar (Qty):Float:120', got %q", cols[0])
	}
	if cols[1] != "Jan-Mar (Amt):Currency/currency:120" {
		t.Errorf("expected 'Jan-Mar (Amt):Currency/currency:120', got %q", cols[1])
	}
}

func TestGetPeriodWiseQuery(t *testing.T) {
	dt := [2]time.Time{
		time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2024, 1, 31, 0, 0, 0, 0, time.UTC),
	}
	result := getPeriodWiseQuery(dt, "posting_date", "")

	expected := "SUM(IF(t1.posting_date BETWEEN '2024-01-01' AND '2024-01-31', t2.stock_qty, NULL))," +
		"SUM(IF(t1.posting_date BETWEEN '2024-01-01' AND '2024-01-31', t2.base_net_amount, NULL)),"
	if result != expected {
		t.Errorf("unexpected query fragment:\ngot:  %q\nwant: %q", result, expected)
	}
}

func TestGetPeriodWiseQuery_Appends(t *testing.T) {
	dt := [2]time.Time{
		time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2024, 2, 29, 0, 0, 0, 0, time.UTC),
	}
	existing := "EXISTING,"
	result := getPeriodWiseQuery(dt, "transaction_date", existing)

	if result[:len(existing)] != existing {
		t.Errorf("should preserve existing query details")
	}
	if len(result) <= len(existing) {
		t.Errorf("should append new query details")
	}
}

func TestGroupWiseColumn(t *testing.T) {
	cols := groupWiseColumn("Customer")
	if len(cols) != 1 {
		t.Fatalf("expected 1 column, got %d", len(cols))
	}
	if cols[0] != "Customer:Link/Customer:120" {
		t.Errorf("expected 'Customer:Link/Customer:120', got %q", cols[0])
	}
}

func TestGroupWiseColumn_Empty(t *testing.T) {
	cols := groupWiseColumn("")
	if len(cols) != 0 {
		t.Errorf("expected 0 columns for empty group_by, got %d", len(cols))
	}
}

func TestBuildTopNChartData(t *testing.T) {
	r := &TrendReport{
		DocType:    "Delivery Note",
		ChartLabel: "Total Delivered Amount",
		ChartType:  "bar",
	}

	data := [][]interface{}{
		{"Item A", 100.0, 500.0},
		{"Item B", 50.0, 300.0},
		{"'Total'", 150.0, 800.0},
		{"Item C", 200.0, 1000.0},
	}
	columns := []string{
		"Item:Link/Item:120",
		"Total(Qty):Float:120",
		"Total(Amt):Currency/currency:120",
	}

	chart := r.buildTopNChartData(data, columns)
	if chart == nil {
		t.Fatal("chart should not be nil")
	}

	chartMap, ok := chart.(map[string]interface{})
	if !ok {
		t.Fatal("chart should be a map")
	}

	if chartMap["type"] != "bar" {
		t.Errorf("chart type: expected 'bar', got %v", chartMap["type"])
	}

	chartData, ok := chartMap["data"].(map[string]interface{})
	if !ok {
		t.Fatal("chart data should be a map")
	}

	labels, ok := chartData["labels"].([]string)
	if !ok {
		t.Fatal("labels should be []string")
	}

	// Should have 3 labels (Total row excluded)
	if len(labels) != 3 {
		t.Errorf("expected 3 labels, got %d", len(labels))
	}

	// First label should be the item with highest total (Item C = 1000)
	if labels[0] != "Item C" {
		t.Errorf("first label should be 'Item C', got %q", labels[0])
	}
}

func TestBuildPeriodicChartData(t *testing.T) {
	r := &TrendReport{
		DocType:    "Sales Order",
		ChartLabel: "{period} Sales Value",
		ChartType:  "line",
	}

	tf := TrendFilters{
		BasedOn: "Item",
		Period:  "Monthly",
	}

	// Item based_on has start=2, so columns[2:-2] picked at every other for Amt
	columns := []string{
		"Item:Link/Item:120",
		"Item Name:Data:120",
		"Jan (Qty):Float:120",
		"Jan (Amt):Currency/currency:120",
		"Feb (Qty):Float:120",
		"Feb (Amt):Currency/currency:120",
		"Total(Qty):Float:120",
		"Total(Amt):Currency/currency:120",
	}

	data := [][]interface{}{
		{"Widget A", "Widget A Name", 10.0, 100.0, 20.0, 200.0, 30.0, 300.0},
		{"Widget B", "Widget B Name", 5.0, 50.0, 15.0, 150.0, 20.0, 200.0},
	}

	chart := r.buildPeriodicChartData(data, columns, tf)
	if chart == nil {
		t.Fatal("chart should not be nil")
	}

	chartMap, ok := chart.(map[string]interface{})
	if !ok {
		t.Fatal("chart should be a map")
	}

	if chartMap["type"] != "line" {
		t.Errorf("chart type: expected 'line', got %v", chartMap["type"])
	}

	chartData, ok := chartMap["data"].(map[string]interface{})
	if !ok {
		t.Fatal("chart data should be a map")
	}

	labels, ok := chartData["labels"].([]string)
	if !ok {
		t.Fatal("labels should be []string")
	}

	// Should have 2 period labels (Jan, Feb)
	if len(labels) != 2 {
		t.Fatalf("expected 2 labels, got %d: %v", len(labels), labels)
	}
	if labels[0] != "Jan" {
		t.Errorf("first label: expected 'Jan', got %q", labels[0])
	}
	if labels[1] != "Feb" {
		t.Errorf("second label: expected 'Feb', got %q", labels[1])
	}

	datasets, ok := chartData["datasets"].([]map[string]interface{})
	if !ok || len(datasets) == 0 {
		t.Fatal("should have at least one dataset")
	}

	values, ok := datasets[0]["values"].([]float64)
	if !ok {
		t.Fatal("values should be []float64")
	}

	// Sum: Jan Amt = 100+50=150, Feb Amt = 200+150=350
	if values[0] != 150.0 {
		t.Errorf("Jan sum: expected 150, got %v", values[0])
	}
	if values[1] != 350.0 {
		t.Errorf("Feb sum: expected 350, got %v", values[1])
	}

	// Check chart label has period substituted
	if datasets[0]["name"] != "Monthly Sales Value" {
		t.Errorf("dataset name: expected 'Monthly Sales Value', got %v", datasets[0]["name"])
	}
}

func TestBuildPeriodicChartData_CustomerBasedOn(t *testing.T) {
	r := &TrendReport{
		DocType:    "Sales Order",
		ChartLabel: "{period} Sales Value",
		ChartType:  "line",
	}

	tf := TrendFilters{
		BasedOn: "Customer",
		Period:  "Monthly",
	}

	// Customer based_on has start=3
	columns := []string{
		"Customer:Link/Customer:120",
		"Customer Name:Data:120",
		"Territory:Link/Territory:120",
		"Jan (Qty):Float:120",
		"Jan (Amt):Currency/currency:120",
		"Total(Qty):Float:120",
		"Total(Amt):Currency/currency:120",
	}

	data := [][]interface{}{
		{"Cust A", "Cust A Name", "US", 10.0, 100.0, 10.0, 100.0},
	}

	chart := r.buildPeriodicChartData(data, columns, tf)
	if chart == nil {
		t.Fatal("chart should not be nil")
	}

	chartMap := chart.(map[string]interface{})
	chartData := chartMap["data"].(map[string]interface{})
	labels := chartData["labels"].([]string)

	if len(labels) != 1 {
		t.Fatalf("expected 1 label, got %d: %v", len(labels), labels)
	}
	if labels[0] != "Jan" {
		t.Errorf("label: expected 'Jan', got %q", labels[0])
	}
}

func TestTrendReportRegistration(t *testing.T) {
	// Verify that TrendReport struct fields are set correctly
	tr := &TrendReport{
		DocType:     "Delivery Note",
		ChartLabel:  "Total Delivered Amount",
		ChartType:   "bar",
		ChartColors: []string{"#ff0000"},
	}

	if tr.DocType != "Delivery Note" {
		t.Errorf("DocType: expected 'Delivery Note', got %q", tr.DocType)
	}
	if tr.ChartLabel != "Total Delivered Amount" {
		t.Errorf("ChartLabel: expected 'Total Delivered Amount', got %q", tr.ChartLabel)
	}
	if tr.ChartType != "bar" {
		t.Errorf("ChartType: expected 'bar', got %q", tr.ChartType)
	}
	if len(tr.ChartColors) != 1 || tr.ChartColors[0] != "#ff0000" {
		t.Errorf("ChartColors unexpected: %v", tr.ChartColors)
	}
}
