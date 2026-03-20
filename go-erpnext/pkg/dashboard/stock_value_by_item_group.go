package dashboard

import (
	"net/http"

	"github.com/alex-baker-cdm/erpnext/go-erpnext/pkg/db"
)

// itemGroupStockValue holds an item group name and its aggregated stock value.
type itemGroupStockValue struct {
	ItemGroup  string
	StockValue float64
}

// StockValueByItemGroup handles the stock value by item group dashboard chart.
// It JOINs tabBin with tabItem to aggregate stock value by item_group,
// optionally filtered by company via warehouse filtering.
//
// Python source: erpnext/stock/dashboard_chart_source/stock_value_by_item_group/stock_value_by_item_group.py
func StockValueByItemGroup(dbConn *db.DB, w http.ResponseWriter, r *http.Request) {
	filters, err := ParseFilters(r)
	if err != nil {
		writeError(w, "invalid filters: "+err.Error(), http.StatusBadRequest)
		return
	}

	company, _ := filters["company"].(string)

	results, err := getStockValueByItemGroup(dbConn, company)
	if err != nil {
		writeError(w, "failed to query stock value by item group: "+err.Error(), http.StatusInternalServerError)
		return
	}

	labels := make([]string, 0, len(results))
	values := make([]float64, 0, len(results))
	for _, row := range results {
		if row.StockValue == 0 {
			continue
		}
		labels = append(labels, row.ItemGroup)
		values = append(values, row.StockValue)
	}

	WriteJSON(w, ChartResult{
		Labels: labels,
		Datasets: []Dataset{
			{Name: "Stock Value", Values: values},
		},
	})
}

// getStockValueByItemGroup queries tabBin joined with tabItem to get stock value
// grouped by item_group. If company is provided, warehouses are filtered by company.
// Returns up to 10 item groups sorted by stock value descending.
func getStockValueByItemGroup(dbConn *db.DB, company string) ([]itemGroupStockValue, error) {
	var query string
	var args []interface{}

	if company != "" {
		// Get warehouses for the company first
		whRows, err := dbConn.RawQuery(
			"SELECT name FROM `tabWarehouse` WHERE is_group = 0 AND company = ?", company,
		)
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
		args = append(args, warehouseNames[0])
		for i := 1; i < len(warehouseNames); i++ {
			placeholders += ", ?"
			args = append(args, warehouseNames[i])
		}

		query = "SELECT i.item_group, SUM(b.stock_value) as stock_value " +
			"FROM `tabBin` b " +
			"INNER JOIN `tabItem` i ON b.item_code = i.name " +
			"WHERE b.warehouse IN (" + placeholders + ") " +
			"GROUP BY i.item_group " +
			"ORDER BY stock_value DESC " +
			"LIMIT 10"
	} else {
		query = "SELECT i.item_group, SUM(b.stock_value) as stock_value " +
			"FROM `tabBin` b " +
			"INNER JOIN `tabItem` i ON b.item_code = i.name " +
			"GROUP BY i.item_group " +
			"ORDER BY stock_value DESC " +
			"LIMIT 10"
	}

	rows, err := dbConn.RawQuery(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []itemGroupStockValue
	for rows.Next() {
		var ig itemGroupStockValue
		if err := rows.Scan(&ig.ItemGroup, &ig.StockValue); err != nil {
			return nil, err
		}
		results = append(results, ig)
	}
	return results, rows.Err()
}
