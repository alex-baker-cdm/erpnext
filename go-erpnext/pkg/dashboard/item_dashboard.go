package dashboard

import (
	"net/http"

	"github.com/alex-baker-cdm/erpnext/go-erpnext/pkg/db"
	"github.com/alex-baker-cdm/erpnext/go-erpnext/pkg/frappe"
)

// itemDashboardRow holds one row returned by the item dashboard endpoint.
type itemDashboardRow struct {
	ItemCode                   string  `json:"item_code"`
	Warehouse                  string  `json:"warehouse"`
	ProjectedQty               float64 `json:"projected_qty"`
	ReservedQty                float64 `json:"reserved_qty"`
	ReservedQtyForProduction   float64 `json:"reserved_qty_for_production"`
	ReservedQtyForSubContract  float64 `json:"reserved_qty_for_sub_contract"`
	ActualQty                  float64 `json:"actual_qty"`
	ValuationRate              float64 `json:"valuation_rate"`
	ItemName                   string  `json:"item_name"`
	ReservedStock              float64 `json:"reserved_stock"`
}

// ItemDashboardGetData handles the item dashboard data endpoint.
// It queries tabBin with optional filters for item_code, warehouse, and item_group,
// and enriches results with item names and stock reservation details.
//
// Python source: erpnext/stock/dashboard/item_dashboard.py
func ItemDashboardGetData(dbConn *db.DB, w http.ResponseWriter, r *http.Request) {
	itemCode := parseStringParam(r, "item_code", "")
	warehouse := parseStringParam(r, "warehouse", "")
	itemGroup := parseStringParam(r, "item_group", "")
	start := parseIntParam(r, "start", 0)
	sortBy := parseStringParam(r, "sort_by", "actual_qty")
	sortOrder := parseStringParam(r, "sort_order", "desc")

	if !validateSortParams(sortBy, sortOrder) {
		writeError(w, "invalid sort_by or sort_order parameter", http.StatusBadRequest)
		return
	}

	items, err := getItemDashboardData(dbConn, itemCode, warehouse, itemGroup, start, sortBy, sortOrder)
	if err != nil {
		writeError(w, "failed to query item dashboard data: "+err.Error(), http.StatusInternalServerError)
		return
	}

	WriteJSON(w, items)
}

// getItemDashboardData queries tabBin for item dashboard data with optional filters.
func getItemDashboardData(dbConn *db.DB, itemCode, warehouse, itemGroup string, start int, sortBy, sortOrder string) ([]itemDashboardRow, error) {
	query := "SELECT b.item_code, b.warehouse, b.projected_qty, b.reserved_qty, " +
		"b.reserved_qty_for_production, b.reserved_qty_for_sub_contract, " +
		"b.actual_qty, b.valuation_rate, IFNULL(i.item_name, '') as item_name " +
		"FROM `tabBin` b " +
		"JOIN `tabItem` i ON b.item_code = i.name " +
		"WHERE (b.projected_qty != 0 OR b.reserved_qty != 0 " +
		"OR b.reserved_qty_for_production != 0 OR b.reserved_qty_for_sub_contract != 0 " +
		"OR b.actual_qty != 0)"

	var args []interface{}

	if itemCode != "" {
		query += " AND b.item_code = ?"
		args = append(args, itemCode)
	}
	if warehouse != "" {
		query += " AND b.warehouse = ?"
		args = append(args, warehouse)
	}
	if itemGroup != "" {
		// Use lft/rgt tree traversal to include child item groups
		items, err := getItemsInGroup(dbConn, itemGroup)
		if err != nil {
			return nil, err
		}
		if len(items) > 0 {
			placeholders := "?"
			args = append(args, items[0])
			for j := 1; j < len(items); j++ {
				placeholders += ", ?"
				args = append(args, items[j])
			}
			query += " AND b.item_code IN (" + placeholders + ")"
		} else {
			// No items in group — return empty
			return []itemDashboardRow{}, nil
		}
	}

	// TODO: Permission filtering placeholder — add warehouse permission conditions here
	// if build_match_conditions("Warehouse", user) returns conditions,
	// filter warehouses accordingly.

	// ORDER BY — sortBy and sortOrder are validated by the caller
	query += " ORDER BY " + sortBy + " " + sortOrder
	query += " LIMIT 21 OFFSET ?"
	args = append(args, start)

	rows, err := dbConn.RawQuery(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Get float precision from System Settings (default to 3)
	precision := getFloatPrecision(dbConn)

	var results []itemDashboardRow
	for rows.Next() {
		var row itemDashboardRow
		if err := rows.Scan(
			&row.ItemCode, &row.Warehouse, &row.ProjectedQty, &row.ReservedQty,
			&row.ReservedQtyForProduction, &row.ReservedQtyForSubContract,
			&row.ActualQty, &row.ValuationRate, &row.ItemName,
		); err != nil {
			return nil, err
		}

		// Apply precision
		row.ProjectedQty = frappe.Flt(row.ProjectedQty, precision)
		row.ReservedQty = frappe.Flt(row.ReservedQty, precision)
		row.ReservedQtyForProduction = frappe.Flt(row.ReservedQtyForProduction, precision)
		row.ReservedQtyForSubContract = frappe.Flt(row.ReservedQtyForSubContract, precision)
		row.ActualQty = frappe.Flt(row.ActualQty, precision)

		// Enrich with stock reservation details
		reserved, _ := getReservedStockQty(dbConn, row.ItemCode, row.Warehouse)
		row.ReservedStock = frappe.Flt(reserved, precision)

		results = append(results, row)
	}

	if results == nil {
		results = []itemDashboardRow{}
	}

	return results, rows.Err()
}

// getItemsInGroup returns all item names that belong to the given item group
// or any of its descendants (using lft/rgt tree traversal).
func getItemsInGroup(dbConn *db.DB, itemGroup string) ([]string, error) {
	var lft, rgt int
	err := dbConn.RawQueryRow(
		"SELECT lft, rgt FROM `tabItem Group` WHERE name = ?", itemGroup,
	).Scan(&lft, &rgt)
	if err != nil {
		return nil, err
	}

	rows, err := dbConn.RawQuery(
		"SELECT i.name FROM `tabItem` i "+
			"WHERE EXISTS (SELECT 1 FROM `tabItem Group` WHERE name = i.item_group AND lft >= ? AND rgt <= ?)",
		lft, rgt,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		items = append(items, name)
	}
	return items, rows.Err()
}

// getReservedStockQty queries the Stock Reservation Entry table to get the
// reserved stock quantity for a specific item_code and warehouse combination.
func getReservedStockQty(dbConn *db.DB, itemCode, warehouse string) (float64, error) {
	var qty float64
	err := dbConn.RawQueryRow(
		"SELECT IFNULL(SUM(reserved_qty - delivered_qty), 0) "+
			"FROM `tabStock Reservation Entry` "+
			"WHERE item_code = ? AND warehouse = ? "+
			"AND docstatus = 1 AND status NOT IN ('Delivered', 'Cancelled')",
		itemCode, warehouse,
	).Scan(&qty)
	if err != nil {
		return 0, err
	}
	return qty, nil
}

// getFloatPrecision reads the float_precision setting from System Settings.
// Returns 3 as the default if the lookup fails.
func getFloatPrecision(dbConn *db.DB) int {
	val, err := dbConn.GetValue("System Settings", "System Settings", "float_precision")
	if err != nil {
		return 3
	}
	return frappe.Cint(val)
}
