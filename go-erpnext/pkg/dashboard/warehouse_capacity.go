package dashboard

import (
	"net/http"

	"github.com/alex-baker-cdm/erpnext/go-erpnext/pkg/db"
	"github.com/alex-baker-cdm/erpnext/go-erpnext/pkg/frappe"
)

// warehouseCapacityRow holds one row returned by the warehouse capacity dashboard.
type warehouseCapacityRow struct {
	ItemCode        string  `json:"item_code"`
	Warehouse       string  `json:"warehouse"`
	Company         string  `json:"company"`
	StockCapacity   float64 `json:"stock_capacity"`
	ActualQty       float64 `json:"actual_qty"`
	PercentOccupied float64 `json:"percent_occupied"`
	ItemName        string  `json:"item_name"`
}

// WarehouseCapacityGetData handles the warehouse capacity dashboard data endpoint.
// It queries tabPutaway Rule for capacity info, enriches with actual stock from
// tabBin, and calculates occupancy percentage.
//
// Python source: erpnext/stock/dashboard/warehouse_capacity_dashboard.py
func WarehouseCapacityGetData(dbConn *db.DB, w http.ResponseWriter, r *http.Request) {
	itemCode := parseStringParam(r, "item_code", "")
	warehouse := parseStringParam(r, "warehouse", "")
	parentWarehouse := parseStringParam(r, "parent_warehouse", "")
	company := parseStringParam(r, "company", "")
	start := parseIntParam(r, "start", 0)
	sortBy := parseStringParam(r, "sort_by", "stock_capacity")
	sortOrder := parseStringParam(r, "sort_order", "desc")

	if !validateSortParams(sortBy, sortOrder) {
		writeError(w, "invalid sort_by or sort_order parameter", http.StatusBadRequest)
		return
	}

	data, err := getWarehouseCapacityData(dbConn, itemCode, warehouse, parentWarehouse, company, start, sortBy, sortOrder)
	if err != nil {
		writeError(w, "failed to query warehouse capacity data: "+err.Error(), http.StatusInternalServerError)
		return
	}

	WriteJSON(w, data)
}

// getWarehouseCapacityData queries tabPutaway Rule for warehouse capacity information
// and enriches with actual stock balance and occupancy percentage.
func getWarehouseCapacityData(dbConn *db.DB, itemCode, warehouse, parentWarehouse, company string, start int, sortBy, sortOrder string) ([]warehouseCapacityRow, error) {
	query := "SELECT p.item_code, p.warehouse, IFNULL(p.company, '') as company, " +
		"p.stock_capacity, " +
		"IFNULL(b.actual_qty, 0) as actual_qty, " +
		"IFNULL(i.item_name, '') as item_name " +
		"FROM `tabPutaway Rule` p " +
		"LEFT JOIN `tabBin` b ON p.item_code = b.item_code AND p.warehouse = b.warehouse " +
		"JOIN `tabItem` i ON p.item_code = i.name " +
		"WHERE p.disable = 0"

	var args []interface{}

	if itemCode != "" {
		query += " AND p.item_code = ?"
		args = append(args, itemCode)
	}
	if warehouse != "" {
		query += " AND p.warehouse = ?"
		args = append(args, warehouse)
	}
	if company != "" {
		query += " AND p.company = ?"
		args = append(args, company)
	}
	if parentWarehouse != "" {
		// Get descendant warehouses using lft/rgt
		warehouses, err := getDescendantWarehouses(dbConn, parentWarehouse)
		if err != nil {
			return nil, err
		}
		if len(warehouses) > 0 {
			placeholders := "?"
			args = append(args, warehouses[0])
			for j := 1; j < len(warehouses); j++ {
				placeholders += ", ?"
				args = append(args, warehouses[j])
			}
			query += " AND p.warehouse IN (" + placeholders + ")"
		}
	}

	// TODO: Permission filtering placeholder — add warehouse permission conditions here

	// Sort and paginate — sortBy and sortOrder are validated by the caller.
	// Since percent_occupied is calculated in Go (not in SQL), we sort by
	// the SQL columns when possible and apply Go-level sorting for percent_occupied.
	if sortBy == "percent_occupied" {
		// Sort in Go after fetching — use default SQL order
		query += " LIMIT 11 OFFSET ?"
	} else {
		query += " ORDER BY " + sortBy + " " + sortOrder + " LIMIT 11 OFFSET ?"
	}
	args = append(args, start)

	rows, err := dbConn.RawQuery(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []warehouseCapacityRow
	for rows.Next() {
		var row warehouseCapacityRow
		if err := rows.Scan(
			&row.ItemCode, &row.Warehouse, &row.Company,
			&row.StockCapacity, &row.ActualQty, &row.ItemName,
		); err != nil {
			return nil, err
		}

		// Calculate occupancy percentage
		row.PercentOccupied = CalculateOccupancy(row.ActualQty, row.StockCapacity)

		results = append(results, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Sort by percent_occupied in Go if requested
	if sortBy == "percent_occupied" {
		sortCapacityData(results, sortOrder)
	}

	if results == nil {
		results = []warehouseCapacityRow{}
	}

	return results, nil
}

// CalculateOccupancy computes the occupancy percentage: (actual_qty / stock_capacity) * 100.
// Returns 0 if stock_capacity is zero to avoid division by zero.
func CalculateOccupancy(actualQty, stockCapacity float64) float64 {
	if stockCapacity == 0 {
		return 0
	}
	return frappe.Flt((actualQty/stockCapacity)*100, 0)
}

// getDescendantWarehouses returns all warehouse names that are descendants of
// the given parent warehouse (using lft/rgt tree traversal).
func getDescendantWarehouses(dbConn *db.DB, parentWarehouse string) ([]string, error) {
	var lft, rgt int
	err := dbConn.RawQueryRow(
		"SELECT lft, rgt FROM `tabWarehouse` WHERE name = ?", parentWarehouse,
	).Scan(&lft, &rgt)
	if err != nil {
		return nil, err
	}

	rows, err := dbConn.RawQuery(
		"SELECT name FROM `tabWarehouse` WHERE lft >= ? AND rgt <= ?", lft, rgt,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var warehouses []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		warehouses = append(warehouses, name)
	}
	return warehouses, rows.Err()
}

// sortCapacityData sorts warehouse capacity data by percent_occupied.
func sortCapacityData(data []warehouseCapacityRow, sortOrder string) {
	// Simple insertion sort for small data sets (limit 11)
	for i := 1; i < len(data); i++ {
		key := data[i]
		j := i - 1
		for j >= 0 && shouldSwap(data[j], key, sortOrder) {
			data[j+1] = data[j]
			j--
		}
		data[j+1] = key
	}
}

// shouldSwap returns true if a should come after b for the given sort order.
func shouldSwap(a, b warehouseCapacityRow, sortOrder string) bool {
	if sortOrder == "desc" {
		return a.PercentOccupied < b.PercentOccupied
	}
	return a.PercentOccupied > b.PercentOccupied
}
