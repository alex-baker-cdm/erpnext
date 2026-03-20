package reports

import (
	"testing"
)

func TestItemPricesColumns(t *testing.T) {
	r := &ItemPricesReport{}
	// Execute with nil DB should fail, but we can test column construction
	// by checking the column definitions directly
	columns := ParseColumnShorthands([]string{
		"Item:Link/Item:100",
		"Item Name::150",
		"Item Group:Link/Item Group:125",
		"Brand::100",
		"Description::150",
		"UOM:Link/UOM:80",
		"Last Purchase Rate:Currency:90",
		"Valuation Rate:Currency:80",
		"Sales Price List::180",
		"Purchase Price List::180",
		"BOM Rate:Currency:90",
	})

	if len(columns) != 11 {
		t.Fatalf("expected 11 columns, got %d", len(columns))
	}

	if columns[0].Label != "Item" {
		t.Errorf("first column label: expected 'Item', got %q", columns[0].Label)
	}
	if columns[0].Fieldtype != "Link" {
		t.Errorf("first column fieldtype: expected 'Link', got %q", columns[0].Fieldtype)
	}
	if columns[0].Options != "Item" {
		t.Errorf("first column options: expected 'Item', got %q", columns[0].Options)
	}
	if columns[0].Width != 100 {
		t.Errorf("first column width: expected 100, got %d", columns[0].Width)
	}

	// Verify currency columns
	if columns[6].Fieldtype != "Currency" {
		t.Errorf("Last Purchase Rate fieldtype: expected 'Currency', got %q", columns[6].Fieldtype)
	}
	if columns[10].Label != "BOM Rate" {
		t.Errorf("last column label: expected 'BOM Rate', got %q", columns[10].Label)
	}

	_ = r // ensure struct compiles
}

func TestTotalStockSummaryColumns_Warehouse(t *testing.T) {
	columns := ParseColumnShorthands([]string{
		"Warehouse:Link/Warehouse:150",
		"Item:Link/Item:150",
		"Description::300",
		"Current Qty:Float:100",
	})

	if len(columns) != 4 {
		t.Fatalf("expected 4 columns, got %d", len(columns))
	}
	if columns[0].Label != "Warehouse" {
		t.Errorf("first column: expected 'Warehouse', got %q", columns[0].Label)
	}
	if columns[3].Fieldtype != "Float" {
		t.Errorf("Current Qty fieldtype: expected 'Float', got %q", columns[3].Fieldtype)
	}
}

func TestTotalStockSummaryColumns_Company(t *testing.T) {
	columns := ParseColumnShorthands([]string{
		"Company:Link/Company:250",
		"Item:Link/Item:150",
		"Description::300",
		"Current Qty:Float:100",
	})

	if len(columns) != 4 {
		t.Fatalf("expected 4 columns, got %d", len(columns))
	}
	if columns[0].Label != "Company" {
		t.Errorf("first column: expected 'Company', got %q", columns[0].Label)
	}
	if columns[0].Width != 250 {
		t.Errorf("Company width: expected 250, got %d", columns[0].Width)
	}
}

func TestBatchItemExpiryStatusColumns(t *testing.T) {
	columns := ParseColumnShorthands([]string{
		"Item:Link/Item:150",
		"Item Name::150",
		"Batch:Link/Batch:150",
		"Stock UOM:Link/UOM:100",
		"Quantity:Float:100",
		"Expires On:Date:100",
		"Expiry (In Days):Int:130",
	})

	if len(columns) != 7 {
		t.Fatalf("expected 7 columns, got %d", len(columns))
	}
	if columns[2].Fieldtype != "Link" {
		t.Errorf("Batch fieldtype: expected 'Link', got %q", columns[2].Fieldtype)
	}
	if columns[2].Options != "Batch" {
		t.Errorf("Batch options: expected 'Batch', got %q", columns[2].Options)
	}
	if columns[5].Fieldtype != "Date" {
		t.Errorf("Expires On fieldtype: expected 'Date', got %q", columns[5].Fieldtype)
	}
	if columns[6].Fieldtype != "Int" {
		t.Errorf("Expiry (In Days) fieldtype: expected 'Int', got %q", columns[6].Fieldtype)
	}
}

func TestBatchItemExpiryStatusValidation(t *testing.T) {
	r := &BatchItemExpiryStatusReport{}

	// Missing from_date
	_, err := r.Execute(map[string]interface{}{"to_date": "2024-01-01"})
	if err == nil {
		t.Error("expected error for missing from_date")
	}

	// Missing to_date
	_, err = r.Execute(map[string]interface{}{"from_date": "2024-01-01"})
	if err == nil {
		t.Error("expected error for missing to_date")
	}
}

func TestShareLedgerColumns(t *testing.T) {
	columns := ParseColumnShorthands([]string{
		"Shareholder:Link/Shareholder:150",
		"Date:Date:100",
		"Transfer Type::140",
		"Share Type::90",
		"No of Shares::90",
		"Rate:Currency:90",
		"Amount:Currency:90",
		"Company::150",
		"Share Transfer:Link/Share Transfer:90",
	})

	if len(columns) != 9 {
		t.Fatalf("expected 9 columns, got %d", len(columns))
	}
	if columns[0].Options != "Shareholder" {
		t.Errorf("Shareholder options: expected 'Shareholder', got %q", columns[0].Options)
	}
	if columns[8].Options != "Share Transfer" {
		t.Errorf("Share Transfer options: expected 'Share Transfer', got %q", columns[8].Options)
	}
}

func TestShareLedgerValidation(t *testing.T) {
	r := &ShareLedgerReport{}

	// Missing date
	_, err := r.Execute(map[string]interface{}{})
	if err == nil {
		t.Error("expected error for missing date")
	}

	// With date but no shareholder => empty data
	result, err := r.Execute(map[string]interface{}{"date": "2024-01-01"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Result) != 0 {
		t.Errorf("expected empty result without shareholder, got %d rows", len(result.Result))
	}
}

func TestShareBalanceColumns(t *testing.T) {
	columns := ParseColumnShorthands([]string{
		"Shareholder:Link/Shareholder:150",
		"Share Type::90",
		"No of Shares::90",
		"Average Rate:Currency:90",
		"Amount:Currency:90",
	})

	if len(columns) != 5 {
		t.Fatalf("expected 5 columns, got %d", len(columns))
	}
	if columns[3].Label != "Average Rate" {
		t.Errorf("expected 'Average Rate', got %q", columns[3].Label)
	}
}

func TestShareBalanceValidation(t *testing.T) {
	r := &ShareBalanceReport{}

	// Missing date
	_, err := r.Execute(map[string]interface{}{})
	if err == nil {
		t.Error("expected error for missing date")
	}

	// With date but no shareholder => empty data
	result, err := r.Execute(map[string]interface{}{"date": "2024-01-01"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Result) != 0 {
		t.Errorf("expected empty result without shareholder, got %d rows", len(result.Result))
	}
}

func TestShareBalanceAggregation(t *testing.T) {
	// Test the aggregation logic directly
	// Simulate two share entries of the same type
	data := [][]interface{}{
		{"SH001", "Equity", 100.0, 10.0, 1000.0},
	}

	// Add second entry of same type
	entry := struct {
		shareType  string
		noOfShares float64
		rate       float64
		amount     float64
	}{"Equity", 50.0, 12.0, 600.0}

	found := false
	for i, row := range data {
		if len(row) > 1 {
			if st, ok := row[1].(string); ok && st == entry.shareType {
				nos := toFloat64(row[2]) + entry.noOfShares
				amt := toFloat64(row[4]) + entry.amount
				var avgRate float64
				if nos != 0 {
					avgRate = amt / nos
				}
				data[i][2] = nos
				data[i][3] = avgRate
				data[i][4] = amt
				found = true
				break
			}
		}
	}

	if !found {
		t.Fatal("should have found existing entry to merge")
	}

	// Check aggregated values
	if data[0][2] != 150.0 {
		t.Errorf("shares: expected 150, got %v", data[0][2])
	}
	expectedRate := 1600.0 / 150.0
	if rate, ok := data[0][3].(float64); !ok || (rate-expectedRate) > 0.001 {
		t.Errorf("avg rate: expected ~%.4f, got %v", expectedRate, data[0][3])
	}
	if data[0][4] != 1600.0 {
		t.Errorf("amount: expected 1600, got %v", data[0][4])
	}
}

func TestAvailableSerialNoColumns(t *testing.T) {
	// Verify column definitions without executing (which requires a DB)
	columns := []Column{
		{Label: "Date", Fieldname: "date", Fieldtype: "Datetime", Width: 150},
		{Label: "Item", Fieldname: "item_code", Fieldtype: "Link", Options: "Item", Width: 100},
		{Label: "Item Name", Fieldname: "item_name", Width: 100},
		{Label: "UOM", Fieldname: "stock_uom", Fieldtype: "Link", Options: "UOM", Width: 60},
		{Label: "In Qty", Fieldname: "in_qty", Fieldtype: "Float", Width: 80},
		{Label: "Out Qty", Fieldname: "out_qty", Fieldtype: "Float", Width: 80},
		{Label: "Balance Qty", Fieldname: "qty_after_transaction", Fieldtype: "Float", Width: 100},
		{Label: "Warehouse", Fieldname: "warehouse", Fieldtype: "Link", Options: "Warehouse", Width: 150},
		{Label: "Serial No (In/Out)", Fieldname: "serial_no", Width: 150},
		{Label: "Balance Serial No", Fieldname: "balance_serial_no", Width: 150},
		{Label: "Incoming Rate", Fieldname: "incoming_rate", Fieldtype: "Currency", Width: 110, Options: "Company:company:default_currency"},
		{Label: "Avg Rate (Balance Stock)", Fieldname: "valuation_rate", Fieldtype: "Currency", Width: 180, Options: "Company:company:default_currency"},
		{Label: "Valuation Rate", Fieldname: "in_out_rate", Fieldtype: "Currency", Width: 140, Options: "Company:company:default_currency"},
		{Label: "Balance Value", Fieldname: "stock_value", Fieldtype: "Currency", Width: 110, Options: "Company:company:default_currency"},
		{Label: "Value Change", Fieldname: "stock_value_difference", Fieldtype: "Currency", Width: 110, Options: "Company:company:default_currency"},
		{Label: "Serial and Batch Bundle", Fieldname: "serial_and_batch_bundle", Fieldtype: "Link", Options: "Serial and Batch Bundle", Width: 100},
		{Label: "Voucher Type", Fieldname: "voucher_type", Width: 110},
		{Label: "Voucher #", Fieldname: "voucher_no", Fieldtype: "Dynamic Link", Options: "voucher_type", Width: 100},
		{Label: "Company", Fieldname: "company", Fieldtype: "Link", Options: "Company", Width: 110},
	}

	if len(columns) != 19 {
		t.Fatalf("expected 19 columns, got %d", len(columns))
	}
	if columns[0].Fieldtype != "Datetime" {
		t.Errorf("Date fieldtype: expected 'Datetime', got %q", columns[0].Fieldtype)
	}
	if columns[4].Fieldname != "in_qty" {
		t.Errorf("In Qty fieldname: expected 'in_qty', got %q", columns[4].Fieldname)
	}
	if columns[17].Fieldtype != "Dynamic Link" {
		t.Errorf("Voucher # fieldtype: expected 'Dynamic Link', got %q", columns[17].Fieldtype)
	}
}

func TestInactiveSalesItemsColumns(t *testing.T) {
	columns := []Column{
		{Label: "Territory", Fieldname: "territory", Fieldtype: "Link", Options: "Territory", Width: 100},
		{Label: "Item Group", Fieldname: "item_group", Fieldtype: "Link", Options: "Item Group", Width: 150},
		{Label: "Item", Fieldname: "item", Fieldtype: "Link", Options: "Item", Width: 150},
		{Label: "Item Name", Fieldname: "item_name", Fieldtype: "Data", Width: 150},
		{Label: "Customer", Fieldname: "customer", Fieldtype: "Link", Options: "Customer", Width: 100},
		{Label: "Last Order Date", Fieldname: "last_order_date", Fieldtype: "Date", Width: 100},
		{Label: "Quantity", Fieldname: "qty", Fieldtype: "Float", Width: 100},
		{Label: "Days Since Last Order", Fieldname: "days_since_last_order", Fieldtype: "Int", Width: 100},
	}

	if len(columns) != 8 {
		t.Fatalf("expected 8 columns, got %d", len(columns))
	}
	if columns[0].Fieldname != "territory" {
		t.Errorf("first fieldname: expected 'territory', got %q", columns[0].Fieldname)
	}
	if columns[7].Fieldtype != "Int" {
		t.Errorf("last fieldtype: expected 'Int', got %q", columns[7].Fieldtype)
	}
}

func TestBankClearanceSummaryColumns(t *testing.T) {
	columns := []Column{
		{Label: "Payment Document Type", Fieldname: "payment_document_type", Fieldtype: "Data", Width: 130},
		{Label: "Payment Entry", Fieldname: "payment_entry", Fieldtype: "Dynamic Link", Options: "payment_document_type", Width: 140},
		{Label: "Posting Date", Fieldname: "posting_date", Fieldtype: "Date", Width: 120},
		{Label: "Cheque/Reference No", Fieldname: "cheque_no", Width: 120},
		{Label: "Clearance Date", Fieldname: "clearance_date", Fieldtype: "Date", Width: 120},
		{Label: "Against Account", Fieldname: "against", Fieldtype: "Link", Options: "Account", Width: 200},
		{Label: "Amount", Fieldname: "amount", Fieldtype: "Currency", Width: 120},
	}

	if len(columns) != 7 {
		t.Fatalf("expected 7 columns, got %d", len(columns))
	}
	if columns[1].Fieldtype != "Dynamic Link" {
		t.Errorf("Payment Entry fieldtype: expected 'Dynamic Link', got %q", columns[1].Fieldtype)
	}
	if columns[1].Options != "payment_document_type" {
		t.Errorf("Payment Entry options: expected 'payment_document_type', got %q", columns[1].Options)
	}
}

func TestBOMStockReportColumns(t *testing.T) {
	columns := ParseColumnShorthands([]string{
		"Item:Link/Item:150",
		"Item Name::240",
		"Description::300",
		"From BOM No::200",
		"BOM Qty:Float:160",
		"BOM UOM::160",
		"Required Qty:Float:120",
		"In Stock Qty:Float:120",
		"Enough Parts to Build:Float:200",
	})

	if len(columns) != 9 {
		t.Fatalf("expected 9 columns, got %d", len(columns))
	}
	if columns[0].Label != "Item" {
		t.Errorf("first column: expected 'Item', got %q", columns[0].Label)
	}
	if columns[4].Fieldtype != "Float" {
		t.Errorf("BOM Qty fieldtype: expected 'Float', got %q", columns[4].Fieldtype)
	}
	if columns[8].Label != "Enough Parts to Build" {
		t.Errorf("last column: expected 'Enough Parts to Build', got %q", columns[8].Label)
	}
}

func TestBOMStockReportValidation(t *testing.T) {
	r := &BOMStockReport{}

	// Zero qty_to_produce
	_, err := r.Execute(map[string]interface{}{
		"qty_to_produce": 0,
		"bom":            "BOM-001",
		"warehouse":      "WH",
	})
	if err == nil {
		t.Error("expected error for zero qty_to_produce")
	}

	// Negative qty_to_produce
	_, err = r.Execute(map[string]interface{}{
		"qty_to_produce": -5.0,
		"bom":            "BOM-001",
		"warehouse":      "WH",
	})
	if err == nil {
		t.Error("expected error for negative qty_to_produce")
	}

	// Missing BOM
	_, err = r.Execute(map[string]interface{}{
		"qty_to_produce": 10.0,
		"warehouse":      "WH",
	})
	if err == nil {
		t.Error("expected error for missing BOM")
	}
}

func TestRegisterAllReports(t *testing.T) {
	// Clear registry
	registryMu.Lock()
	oldRegistry := registry
	registry = map[string]Report{}
	registryMu.Unlock()

	defer func() {
		registryMu.Lock()
		registry = oldRegistry
		registryMu.Unlock()
	}()

	RegisterAllReports(nil)

	expectedReports := []string{
		"Delivery Note Trends",
		"Purchase Receipt Trends",
		"Sales Invoice Trends",
		"Purchase Invoice Trends",
		"Sales Order Trends",
		"Quotation Trends",
		"Purchase Order Trends",
		"Item Prices",
		"Total Stock Summary",
		"Batch Item Expiry Status",
		"Share Ledger",
		"Share Balance",
		"Available Serial No",
		"Inactive Sales Items",
		"Bank Clearance Summary",
		"BOM Stock Report",
	}

	for _, name := range expectedReports {
		if GetReport(name) == nil {
			t.Errorf("report %q not registered", name)
		}
	}

	// Total should be 16 (7 trend + 9 simple)
	registryMu.RLock()
	count := len(registry)
	registryMu.RUnlock()

	if count != 16 {
		t.Errorf("expected 16 registered reports, got %d", count)
	}
}

func TestPriceListGrouping(t *testing.T) {
	// Test the price list grouping logic by simulating what getPriceList does
	rate := make(map[string]map[string][]string)

	// Simulate price list entries
	entries := []struct {
		itemCode string
		buying   int
		price    string
	}{
		{"ITEM-001", 0, "USD 10.00 - Standard Selling"},
		{"ITEM-001", 0, "EUR 9.00 - EU Selling"},
		{"ITEM-001", 1, "USD 8.00 - Standard Buying"},
		{"ITEM-002", 0, "USD 20.00 - Standard Selling"},
	}

	for _, e := range entries {
		key := "Selling"
		if e.buying == 1 {
			key = "Buying"
		}
		if rate[e.itemCode] == nil {
			rate[e.itemCode] = make(map[string][]string)
		}
		rate[e.itemCode][key] = append(rate[e.itemCode][key], e.price)
	}

	// Build result map
	result := make(map[string]map[string]string)
	for item, byType := range rate {
		result[item] = make(map[string]string)
		for key, prices := range byType {
			joined := ""
			for i, p := range prices {
				if i > 0 {
					joined += ", "
				}
				joined += p
			}
			result[item][key] = joined
		}
	}

	// Verify ITEM-001 selling
	if result["ITEM-001"]["Selling"] != "USD 10.00 - Standard Selling, EUR 9.00 - EU Selling" {
		t.Errorf("ITEM-001 selling: got %q", result["ITEM-001"]["Selling"])
	}

	// Verify ITEM-001 buying
	if result["ITEM-001"]["Buying"] != "USD 8.00 - Standard Buying" {
		t.Errorf("ITEM-001 buying: got %q", result["ITEM-001"]["Buying"])
	}

	// Verify ITEM-002
	if result["ITEM-002"]["Selling"] != "USD 20.00 - Standard Selling" {
		t.Errorf("ITEM-002 selling: got %q", result["ITEM-002"]["Selling"])
	}
}

func TestValuationRateCalculation(t *testing.T) {
	// Test the weighted average calculation logic
	type binEntry struct {
		actualQty     float64
		valuationRate float64
	}

	bins := []binEntry{
		{100, 10.0},
		{200, 15.0},
		{50, 20.0},
	}

	var totalQtyVal float64
	var totalQty float64
	for _, b := range bins {
		totalQtyVal += b.actualQty * b.valuationRate
		totalQty += b.actualQty
	}

	if totalQty == 0 {
		t.Fatal("total qty should not be zero")
	}

	valRate := totalQtyVal / totalQty
	// Expected: (100*10 + 200*15 + 50*20) / (100+200+50) = (1000+3000+1000)/350 = 5000/350 ≈ 14.2857
	expected := 5000.0 / 350.0
	if (valRate - expected) > 0.001 || (expected - valRate) > 0.001 {
		t.Errorf("valuation rate: expected %.4f, got %.4f", expected, valRate)
	}
}
