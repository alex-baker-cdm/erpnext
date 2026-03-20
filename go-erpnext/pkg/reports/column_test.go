package reports

import (
	"testing"
)

func TestParseColumnShorthand_LinkWithOptions(t *testing.T) {
	col := ParseColumnShorthand("Item:Link/Item:120")
	if col.Label != "Item" {
		t.Errorf("expected label 'Item', got %q", col.Label)
	}
	if col.Fieldtype != "Link" {
		t.Errorf("expected fieldtype 'Link', got %q", col.Fieldtype)
	}
	if col.Options != "Item" {
		t.Errorf("expected options 'Item', got %q", col.Options)
	}
	if col.Width != 120 {
		t.Errorf("expected width 120, got %d", col.Width)
	}
}

func TestParseColumnShorthand_CurrencyWithOptions(t *testing.T) {
	col := ParseColumnShorthand("Total(Amt):Currency/currency:120")
	if col.Label != "Total(Amt)" {
		t.Errorf("expected label 'Total(Amt)', got %q", col.Label)
	}
	if col.Fieldtype != "Currency" {
		t.Errorf("expected fieldtype 'Currency', got %q", col.Fieldtype)
	}
	if col.Options != "currency" {
		t.Errorf("expected options 'currency', got %q", col.Options)
	}
	if col.Width != 120 {
		t.Errorf("expected width 120, got %d", col.Width)
	}
}

func TestParseColumnShorthand_DataColumn(t *testing.T) {
	col := ParseColumnShorthand("Item Name:Data:120")
	if col.Label != "Item Name" {
		t.Errorf("expected label 'Item Name', got %q", col.Label)
	}
	if col.Fieldtype != "Data" {
		t.Errorf("expected fieldtype 'Data', got %q", col.Fieldtype)
	}
	if col.Options != "" {
		t.Errorf("expected empty options, got %q", col.Options)
	}
	if col.Width != 120 {
		t.Errorf("expected width 120, got %d", col.Width)
	}
}

func TestParseColumnShorthand_FloatColumn(t *testing.T) {
	col := ParseColumnShorthand("Total(Qty):Float:120")
	if col.Label != "Total(Qty)" {
		t.Errorf("expected label 'Total(Qty)', got %q", col.Label)
	}
	if col.Fieldtype != "Float" {
		t.Errorf("expected fieldtype 'Float', got %q", col.Fieldtype)
	}
	if col.Width != 120 {
		t.Errorf("expected width 120, got %d", col.Width)
	}
}

func TestParseColumnShorthand_NoWidth(t *testing.T) {
	col := ParseColumnShorthand("Name:Data")
	if col.Label != "Name" {
		t.Errorf("expected label 'Name', got %q", col.Label)
	}
	if col.Fieldtype != "Data" {
		t.Errorf("expected fieldtype 'Data', got %q", col.Fieldtype)
	}
	if col.Width != 0 {
		t.Errorf("expected width 0, got %d", col.Width)
	}
}

func TestParseColumnShorthand_LabelOnly(t *testing.T) {
	col := ParseColumnShorthand("OnlyLabel")
	if col.Label != "OnlyLabel" {
		t.Errorf("expected label 'OnlyLabel', got %q", col.Label)
	}
	if col.Fieldtype != "" {
		t.Errorf("expected empty fieldtype, got %q", col.Fieldtype)
	}
}

func TestParseColumnShorthand_EmptyString(t *testing.T) {
	col := ParseColumnShorthand("")
	if col.Label != "" {
		t.Errorf("expected empty label, got %q", col.Label)
	}
}

func TestParseColumnShorthand_FieldnameGeneration(t *testing.T) {
	col := ParseColumnShorthand("Item Name:Data:120")
	if col.Fieldname != "item_name" {
		t.Errorf("expected fieldname 'item_name', got %q", col.Fieldname)
	}

	col2 := ParseColumnShorthand("Customer Group:Link/Customer Group")
	if col2.Fieldname != "customer_group" {
		t.Errorf("expected fieldname 'customer_group', got %q", col2.Fieldname)
	}
}

func TestParseColumnShorthand_SupplierGroupWidth(t *testing.T) {
	col := ParseColumnShorthand("Supplier Group:Link/Supplier Group:140")
	if col.Fieldtype != "Link" {
		t.Errorf("expected fieldtype 'Link', got %q", col.Fieldtype)
	}
	if col.Options != "Supplier Group" {
		t.Errorf("expected options 'Supplier Group', got %q", col.Options)
	}
	if col.Width != 140 {
		t.Errorf("expected width 140, got %d", col.Width)
	}
}

func TestParseColumnShorthands(t *testing.T) {
	shorthands := []string{
		"Item:Link/Item:120",
		"Item Name:Data:120",
		"Qty:Float:100",
	}
	cols := ParseColumnShorthands(shorthands)
	if len(cols) != 3 {
		t.Fatalf("expected 3 columns, got %d", len(cols))
	}
	if cols[0].Fieldtype != "Link" {
		t.Errorf("first column: expected fieldtype 'Link', got %q", cols[0].Fieldtype)
	}
	if cols[1].Fieldtype != "Data" {
		t.Errorf("second column: expected fieldtype 'Data', got %q", cols[1].Fieldtype)
	}
	if cols[2].Fieldtype != "Float" {
		t.Errorf("third column: expected fieldtype 'Float', got %q", cols[2].Fieldtype)
	}
}
