package stock

import (
	"fmt"

	"github.com/alex-baker-cdm/erpnext/go-erpnext/pkg/db"
)

// GetItemDefaults returns the item record merged with the matching Item Default
// row for the given company.
//
// Mirrors erpnext/stock/doctype/item/item.py:1315-1325.
func GetItemDefaults(d *db.DB, itemCode, company string) (map[string]interface{}, error) {
	item, err := d.RawQueryRow(
		"SELECT * FROM `tabItem` WHERE name = ?", itemCode,
	)
	if err != nil {
		return nil, fmt.Errorf("getting item: %w", err)
	}
	if item == nil {
		return map[string]interface{}{}, nil
	}

	// Merge matching Item Default row
	defaults, err := d.RawQuery(
		"SELECT * FROM `tabItem Default` WHERE parent = ? AND company = ?",
		itemCode, company,
	)
	if err != nil {
		return nil, fmt.Errorf("getting item defaults: %w", err)
	}
	if len(defaults) > 0 {
		for k, v := range defaults[0] {
			if k != "name" && v != nil && ToString(v) != "" {
				item[k] = v
			}
		}
	}

	return item, nil
}

// GetItemGroupDefaults returns the Item Default row for the item's item_group
// and the given company.
//
// Mirrors erpnext/stock/get_item_details.py logic for item_group defaults.
func GetItemGroupDefaults(d *db.DB, itemCode, company string) (map[string]interface{}, error) {
	itemRow, err := d.RawQueryRow(
		"SELECT item_group FROM `tabItem` WHERE name = ?", itemCode,
	)
	if err != nil {
		return nil, fmt.Errorf("getting item group: %w", err)
	}
	if itemRow == nil || itemRow["item_group"] == nil {
		return map[string]interface{}{}, nil
	}

	itemGroup := ToString(itemRow["item_group"])
	defaults, err := d.RawQuery(
		"SELECT * FROM `tabItem Default` WHERE parent = ? AND company = ?",
		itemGroup, company,
	)
	if err != nil {
		return nil, fmt.Errorf("getting item group defaults: %w", err)
	}
	if len(defaults) > 0 {
		return defaults[0], nil
	}
	return map[string]interface{}{}, nil
}

// GetBrandDefaults returns the Item Default row for the item's brand
// and the given company.
//
// Mirrors erpnext/stock/get_item_details.py logic for brand defaults.
func GetBrandDefaults(d *db.DB, itemCode, company string) (map[string]interface{}, error) {
	itemRow, err := d.RawQueryRow(
		"SELECT brand FROM `tabItem` WHERE name = ?", itemCode,
	)
	if err != nil {
		return nil, fmt.Errorf("getting item brand: %w", err)
	}
	if itemRow == nil || itemRow["brand"] == nil || ToString(itemRow["brand"]) == "" {
		return map[string]interface{}{}, nil
	}

	brand := ToString(itemRow["brand"])
	defaults, err := d.RawQuery(
		"SELECT * FROM `tabItem Default` WHERE parent = ? AND company = ?",
		brand, company,
	)
	if err != nil {
		return nil, fmt.Errorf("getting brand defaults: %w", err)
	}
	if len(defaults) > 0 {
		return defaults[0], nil
	}
	return map[string]interface{}{}, nil
}
