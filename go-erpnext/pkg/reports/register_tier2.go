package reports

import (
	"github.com/alex-baker-cdm/erpnext/go-erpnext/pkg/db"
)

// RegisterTier2Reports registers all Tier 2 report implementations
// (8 stock reports + 6 accounts reports) with the global report registry.
// It should be called during server startup after the database connection
// is established.
func RegisterTier2Reports(database *db.DB) {
	// Stock Reports
	Register("Stock Ledger", &StockLedgerReport{DB: database})
	Register("Stock Projected Qty", &StockProjectedQtyReport{DB: database})
	Register("Item Shortage Report", &ItemShortageReport{DB: database})
	Register("Reserved Stock", &ReservedStockReport{DB: database})
	Register("Stock Analytics", &StockAnalyticsReport{DB: database})
	Register("Serial and Batch Summary", &SerialAndBatchSummaryReport{DB: database})
	Register("Serial No Ledger", &SerialNoLedgerReport{DB: database})
	Register("Warehouse Wise Stock Balance", &WarehouseWiseStockBalanceReport{DB: database})

	// Accounts Reports
	Register("Sales Register", &SalesRegisterReport{DB: database})
	Register("Purchase Register", &PurchaseRegisterReport{DB: database})
	Register("Item Wise Sales Register", &ItemWiseSalesRegisterReport{DB: database})
	Register("Item Wise Purchase Register", &ItemWisePurchaseRegisterReport{DB: database})
	Register("Delivered Items To Be Billed", &DeliveredItemsToBeBilledReport{DB: database})
	Register("Billed Items To Be Received", &BilledItemsToBeReceivedReport{DB: database})
}
