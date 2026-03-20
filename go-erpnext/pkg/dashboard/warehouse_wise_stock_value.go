package dashboard

import (
	"net/http"

	"github.com/alex-baker-cdm/erpnext/go-erpnext/pkg/db"
)

// warehouseStockValue holds a warehouse name and its aggregated stock value.
type warehouseStockValue struct {
	Warehouse  string
	StockValue float64
}

// WarehouseWiseStockValue handles the warehouse-wise stock value dashboard chart.
// It queries tabBin aggregated by warehouse, optionally filtered by company,
// and returns the top 10 warehouses by stock value.
//
// Python source: erpnext/stock/dashboard_chart_source/warehouse_wise_stock_value/warehouse_wise_stock_value.py
func WarehouseWiseStockValue(dbConn *db.DB, w http.ResponseWriter, r *http.Request) {
	filters, err := ParseFilters(r)
	if err != nil {
		writeError(w, "invalid filters: "+err.Error(), http.StatusBadRequest)
		return
	}

	company, _ := filters["company"].(string)

	warehouses, err := getWarehouseWiseStockValues(dbConn, company)
	if err != nil {
		writeError(w, "failed to query warehouse stock values: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if len(warehouses) == 0 {
		WriteJSON(w, []interface{}{})
		return
	}

	labels := make([]string, len(warehouses))
	values := make([]float64, len(warehouses))
	for i, wh := range warehouses {
		labels[i] = wh.Warehouse
		values[i] = wh.StockValue
	}

	WriteJSON(w, ChartResult{
		Labels: labels,
		Datasets: []Dataset{
			{Name: "Stock Value", Values: values},
		},
	})
}

// getWarehouseWiseStockValues queries tabBin for stock values grouped by warehouse.
// If company is provided, only warehouses belonging to that company are included.
// Returns up to 10 warehouses sorted by stock value descending.
func getWarehouseWiseStockValues(dbConn *db.DB, company string) ([]warehouseStockValue, error) {
	// First get non-group warehouses, optionally filtered by company
	var warehouseQuery string
	var warehouseArgs []interface{}

	if company != "" {
		warehouseQuery = "SELECT name FROM `tabWarehouse` WHERE is_group = 0 AND company = ? ORDER BY name"
		warehouseArgs = append(warehouseArgs, company)
	} else {
		warehouseQuery = "SELECT name FROM `tabWarehouse` WHERE is_group = 0 ORDER BY name"
	}

	whRows, err := dbConn.RawQuery(warehouseQuery, warehouseArgs...)
	if err != nil {
		return nil, err
	}
	defer whRows.Close()

	var warehouseNames []string
	for whRows.Next() {
		var name string
		if err := whRows.Scan(&name); err != nil {
			return nil, err
		}
		warehouseNames = append(warehouseNames, name)
	}
	if err := whRows.Err(); err != nil {
		return nil, err
	}

	if len(warehouseNames) == 0 {
		return nil, nil
	}

	// Build IN clause
	placeholders := "?"
	args := make([]interface{}, 0, len(warehouseNames))
	args = append(args, warehouseNames[0])
	for i := 1; i < len(warehouseNames); i++ {
		placeholders += ", ?"
		args = append(args, warehouseNames[i])
	}

	query := "SELECT warehouse, SUM(stock_value) as stock_value " +
		"FROM `tabBin` " +
		"WHERE warehouse IN (" + placeholders + ") AND stock_value > 0 " +
		"GROUP BY warehouse " +
		"ORDER BY stock_value DESC " +
		"LIMIT 10"

	rows, err := dbConn.RawQuery(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []warehouseStockValue
	for rows.Next() {
		var wv warehouseStockValue
		if err := rows.Scan(&wv.Warehouse, &wv.StockValue); err != nil {
			return nil, err
		}
		results = append(results, wv)
	}
	return results, rows.Err()
}
