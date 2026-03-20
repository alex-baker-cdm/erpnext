package stock

import (
	"fmt"
	"strings"

	"github.com/alex-baker-cdm/erpnext/go-erpnext/pkg/db"
)

// toFloat64 converts a database value to float64.
func toFloat64(v interface{}) float64 {
	switch val := v.(type) {
	case float64:
		return val
	case float32:
		return float64(val)
	case int64:
		return float64(val)
	case int:
		return float64(val)
	case string:
		var f float64
		fmt.Sscanf(val, "%f", &f)
		return f
	case []byte:
		var f float64
		fmt.Sscanf(string(val), "%f", &f)
		return f
	case nil:
		return 0.0
	default:
		return 0.0
	}
}

// ToString converts a database value to string.
func ToString(v interface{}) string {
	if v == nil {
		return ""
	}
	switch val := v.(type) {
	case string:
		return val
	case []byte:
		return string(val)
	default:
		return fmt.Sprintf("%v", val)
	}
}

// ToBool checks whether a form parameter is truthy.
func ToBool(s string) bool {
	s = strings.TrimSpace(strings.ToLower(s))
	return s != "" && s != "0" && s != "false" && s != "no" && s != "none"
}

// GetStockBalance returns the stock balance quantity (and optionally the
// valuation rate) at the given warehouse on the given posting date/time.
//
// Mirrors erpnext/stock/utils.py:97-160.
func GetStockBalance(d *db.DB, itemCode, warehouse, postingDate, postingTime string, withValuationRate bool) (interface{}, error) {
	if postingDate == "" {
		postingDate = "2099-12-31"
	}
	if postingTime == "" {
		postingTime = "23:59:59"
	}

	lastEntry, err := GetPreviousSLE(d, PreviousSLEArgs{
		ItemCode:    itemCode,
		Warehouse:   warehouse,
		PostingDate: postingDate,
		PostingTime: postingTime,
	})
	if err != nil {
		return nil, err
	}

	if lastEntry == nil {
		if withValuationRate {
			return []float64{0.0, 0.0}, nil
		}
		return 0.0, nil
	}

	qty := toFloat64(lastEntry["qty_after_transaction"])
	if withValuationRate {
		rate := toFloat64(lastEntry["valuation_rate"])
		return []float64{qty, rate}, nil
	}
	return qty, nil
}

// GetLatestStockQty returns the latest actual quantity for the given item,
// optionally filtered by warehouse (including group warehouse expansion).
//
// Mirrors erpnext/stock/utils.py:170-191.
func GetLatestStockQty(d *db.DB, itemCode string, warehouse string) (float64, error) {
	if warehouse != "" {
		row, err := d.RawQueryRow(
			"SELECT is_group, lft, rgt FROM `tabWarehouse` WHERE name = ?", warehouse,
		)
		if err != nil {
			return 0, fmt.Errorf("checking warehouse: %w", err)
		}

		if row != nil && toFloat64(row["is_group"]) == 1 {
			lft := row["lft"]
			rgt := row["rgt"]
			result, err := d.RawQueryRow(
				"SELECT SUM(actual_qty) as total FROM `tabBin` WHERE item_code = ? AND EXISTS (SELECT name FROM `tabWarehouse` wh WHERE wh.name = `tabBin`.warehouse AND wh.lft >= ? AND wh.rgt <= ?)",
				itemCode, lft, rgt,
			)
			if err != nil {
				return 0, fmt.Errorf("getting group stock qty: %w", err)
			}
			if result == nil {
				return 0, nil
			}
			return toFloat64(result["total"]), nil
		}

		result, err := d.RawQueryRow(
			"SELECT SUM(actual_qty) as total FROM `tabBin` WHERE item_code = ? AND warehouse = ?",
			itemCode, warehouse,
		)
		if err != nil {
			return 0, fmt.Errorf("getting stock qty: %w", err)
		}
		if result == nil {
			return 0, nil
		}
		return toFloat64(result["total"]), nil
	}

	result, err := d.RawQueryRow(
		"SELECT SUM(actual_qty) as total FROM `tabBin` WHERE item_code = ?",
		itemCode,
	)
	if err != nil {
		return 0, fmt.Errorf("getting stock qty: %w", err)
	}
	if result == nil {
		return 0, nil
	}
	return toFloat64(result["total"]), nil
}

// GetStockValueOn returns the total stock value (sum of stock_value_difference)
// from the Stock Ledger Entry table up to the given posting date.
//
// Mirrors erpnext/stock/utils.py:60-93.
func GetStockValueOn(d *db.DB, warehouses []string, postingDate, itemCode, company string) (float64, error) {
	if postingDate == "" {
		postingDate = "2099-12-31"
	}

	var conditions []string
	var args []interface{}

	conditions = append(conditions, "posting_date <= ?")
	args = append(args, postingDate)
	conditions = append(conditions, "is_cancelled = 0")

	if len(warehouses) > 0 {
		// Expand group warehouses
		expanded := make(map[string]bool)
		for _, wh := range warehouses {
			row, err := d.RawQueryRow(
				"SELECT is_group FROM `tabWarehouse` WHERE name = ?", wh,
			)
			if err != nil {
				return 0, fmt.Errorf("checking warehouse group: %w", err)
			}
			if row != nil && toFloat64(row["is_group"]) == 1 {
				children, err := GetChildWarehouses(d, wh)
				if err != nil {
					return 0, err
				}
				for _, c := range children {
					expanded[c] = true
				}
			} else {
				expanded[wh] = true
			}
		}

		if len(expanded) > 0 {
			placeholders := make([]string, 0, len(expanded))
			for wh := range expanded {
				placeholders = append(placeholders, "?")
				args = append(args, wh)
			}
			conditions = append(conditions, "warehouse IN ("+strings.Join(placeholders, ", ")+")")
		}
	}

	if itemCode != "" {
		conditions = append(conditions, "item_code = ?")
		args = append(args, itemCode)
	}

	if company != "" {
		conditions = append(conditions, "company = ?")
		args = append(args, company)
	}

	query := fmt.Sprintf(
		"SELECT IFNULL(SUM(stock_value_difference), 0) as total FROM `tabStock Ledger Entry` WHERE %s",
		strings.Join(conditions, " AND "),
	)

	row, err := d.RawQueryRow(query, args...)
	if err != nil {
		return 0, fmt.Errorf("get stock value on: %w", err)
	}
	if row == nil {
		return 0, nil
	}
	return toFloat64(row["total"]), nil
}

// GetValuationRate returns the valuation rate for the given item/company/warehouse.
//
// Mirrors erpnext/stock/get_item_details.py:1652-1683.
func GetValuationRate(d *db.DB, itemCode, company, warehouse string) (map[string]interface{}, error) {
	// Check if warehouse is a group
	if warehouse != "" {
		whRow, err := d.RawQueryRow(
			"SELECT is_group FROM `tabWarehouse` WHERE name = ?", warehouse,
		)
		if err != nil {
			return nil, fmt.Errorf("checking warehouse: %w", err)
		}
		if whRow != nil && toFloat64(whRow["is_group"]) == 1 {
			return map[string]interface{}{"valuation_rate": 0.0}, nil
		}
	}

	item, err := GetItemDefaults(d, itemCode, company)
	if err != nil {
		return nil, err
	}
	itemGroup, err := GetItemGroupDefaults(d, itemCode, company)
	if err != nil {
		return nil, err
	}
	brand, err := GetBrandDefaults(d, itemCode, company)
	if err != nil {
		return nil, err
	}

	isStockItem := toFloat64(item["is_stock_item"]) == 1

	if isStockItem {
		if warehouse == "" {
			// Try to get default warehouse from item, item_group, or brand defaults
			if w := ToString(item["default_warehouse"]); w != "" {
				warehouse = w
			} else if w := ToString(itemGroup["default_warehouse"]); w != "" {
				warehouse = w
			} else if w := ToString(brand["default_warehouse"]); w != "" {
				warehouse = w
			}
		}

		binRow, err := d.RawQueryRow(
			"SELECT valuation_rate FROM `tabBin` WHERE item_code = ? AND warehouse = ?",
			itemCode, warehouse,
		)
		if err != nil {
			return nil, fmt.Errorf("getting bin valuation rate: %w", err)
		}
		if binRow != nil {
			return map[string]interface{}{"valuation_rate": toFloat64(binRow["valuation_rate"])}, nil
		}
		// Fall back to item valuation_rate
		rate := toFloat64(item["valuation_rate"])
		return map[string]interface{}{"valuation_rate": rate}, nil
	}

	// Non-stock item: get from Purchase Invoice
	piRow, err := d.RawQueryRow(
		"SELECT SUM(base_net_amount) / SUM(qty * conversion_factor) as valuation_rate FROM `tabPurchase Invoice Item` WHERE docstatus = 1 AND item_code = ?",
		itemCode,
	)
	if err != nil {
		return nil, fmt.Errorf("getting PI valuation rate: %w", err)
	}
	if piRow != nil {
		return map[string]interface{}{"valuation_rate": toFloat64(piRow["valuation_rate"])}, nil
	}

	return map[string]interface{}{"valuation_rate": 0.0}, nil
}
