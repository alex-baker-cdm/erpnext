// Package stock provides Go implementations of ERPNext stock query functions.
package stock

import (
	"fmt"

	"github.com/alex-baker-cdm/erpnext/go-erpnext/pkg/db"
)

// GetChildWarehouses returns the list of all descendant warehouses (including
// the warehouse itself), matching the Python behavior at
// erpnext/stock/doctype/warehouse/warehouse.py:241-245.
func GetChildWarehouses(d *db.DB, warehouse string) ([]string, error) {
	row, err := d.RawQueryRow(
		"SELECT lft, rgt FROM `tabWarehouse` WHERE name = ?", warehouse,
	)
	if err != nil {
		return nil, fmt.Errorf("getting warehouse lft/rgt: %w", err)
	}
	if row == nil {
		return nil, fmt.Errorf("warehouse %q not found", warehouse)
	}

	lft, rgt := row["lft"], row["rgt"]

	rows, err := d.RawQuery(
		"SELECT name FROM `tabWarehouse` WHERE lft >= ? AND rgt <= ? ORDER BY lft",
		lft, rgt,
	)
	if err != nil {
		return nil, fmt.Errorf("getting child warehouses: %w", err)
	}

	names := make([]string, 0, len(rows))
	for _, r := range rows {
		if n, ok := r["name"].(string); ok {
			names = append(names, n)
		}
	}
	return names, nil
}
