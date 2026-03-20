package reports

import (
	"github.com/alex-baker-cdm/erpnext/go-erpnext/pkg/db"
)

// RegisterAllReports registers all Go-ported reports in the global registry.
// Call this from main.go after DB initialization.
func RegisterAllReports(database *db.DB) {
	// Trend reports
	Register("Delivery Note Trends", &TrendReport{
		DocType:    "Delivery Note",
		ChartLabel: "Total Delivered Amount",
		ChartType:  "bar",
		DB:         database,
	})
	Register("Purchase Receipt Trends", &TrendReport{
		DocType:     "Purchase Receipt",
		ChartLabel:  "Total Received Amount",
		ChartType:   "bar",
		ChartColors: []string{"#5e64ff"},
		DB:          database,
	})
	Register("Sales Invoice Trends", &TrendReport{
		DocType: "Sales Invoice",
		DB:      database,
	})
	Register("Purchase Invoice Trends", &TrendReport{
		DocType: "Purchase Invoice",
		DB:      database,
	})
	Register("Sales Order Trends", &TrendReport{
		DocType:    "Sales Order",
		ChartLabel: "{period} Sales Value",
		ChartType:  "line",
		DB:         database,
	})
	Register("Quotation Trends", &TrendReport{
		DocType:    "Quotation",
		ChartLabel: "{period} Quoted Amount",
		ChartType:  "line",
		DB:         database,
	})
	Register("Purchase Order Trends", &TrendReport{
		DocType:    "Purchase Order",
		ChartLabel: "{period} Purchase Value",
		ChartType:  "line",
		DB:         database,
	})

	// Simple list reports
	Register("Item Prices", &ItemPricesReport{DB: database})
	Register("Total Stock Summary", &TotalStockSummaryReport{DB: database})
	Register("Batch Item Expiry Status", &BatchItemExpiryStatusReport{DB: database})
	Register("Share Ledger", &ShareLedgerReport{DB: database})
	Register("Share Balance", &ShareBalanceReport{DB: database})
	Register("Available Serial No", &AvailableSerialNoReport{DB: database})
	Register("Inactive Sales Items", &InactiveSalesItemsReport{DB: database})
	Register("Bank Clearance Summary", &BankClearanceSummaryReport{DB: database})
	Register("BOM Stock Report", &BOMStockReport{DB: database})
}
