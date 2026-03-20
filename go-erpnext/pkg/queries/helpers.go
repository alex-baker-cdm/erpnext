package queries

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/alex-baker-cdm/erpnext/go-erpnext/pkg/db"
)

// GetFields returns the search fields for a doctype, merging defaults with
// the doctype's configured search_fields and title_field.
// This is a Go equivalent of the Python get_fields() helper.
// TODO: Read search_fields and title_field from doctype meta when DB is available.
// For now returns the provided default fields.
func GetFields(dbConn *db.DB, doctype string, defaultFields []string) []string {
	// In the full implementation, this would read from DocType meta:
	//   meta = frappe.get_meta(doctype)
	//   fields.extend(meta.get_search_fields())
	//   if meta.title_field: fields.insert(1, meta.title_field)
	// For now, return deduplicated default fields.
	return uniqueStrings(defaultFields)
}

// HasIgnoredField checks if a reference doctype has an ignore_user_permissions
// Link or Dynamic Link field pointing to the given doctype.
// TODO: Requires reading doctype meta from JSON/DB. Returns false as placeholder.
func HasIgnoredField(dbConn *db.DB, referenceDoctype, doctype string) bool {
	return false
}

// GetDoctypeWiseFilters separates a list-of-lists filter set by doctype name.
// Input format: [[doctype, field, op, value], ...]
// Returns a map from doctype name to its filters.
func GetDoctypeWiseFilters(filters [][]interface{}) map[string][][]interface{} {
	result := make(map[string][][]interface{})
	for _, row := range filters {
		if len(row) < 1 {
			continue
		}
		dt, ok := row[0].(string)
		if !ok {
			continue
		}
		result[dt] = append(result[dt], row)
	}
	return result
}

// GetFilteredBatches deduplicates batch data by batch name, sums quantities,
// and filters out batches with non-positive total quantity.
// Input rows are expected as [batch_name, qty, ...extra_fields].
func GetFilteredBatches(data [][]interface{}) [][]interface{} {
	type batchEntry struct {
		order int
		data  []interface{}
	}

	batches := make(map[string]*batchEntry)
	var order []string

	for _, row := range data {
		if len(row) < 2 {
			continue
		}
		name, ok := row[0].(string)
		if !ok {
			continue
		}
		qty := toFloat64(row[1])

		if existing, found := batches[name]; found {
			existingQty := toFloat64(existing.data[1])
			existing.data[1] = existingQty + qty
		} else {
			// Copy the row
			rowCopy := make([]interface{}, len(row))
			copy(rowCopy, row)
			batches[name] = &batchEntry{order: len(order), data: rowCopy}
			order = append(order, name)
		}
	}

	var result [][]interface{}
	for _, name := range order {
		entry := batches[name]
		qty := toFloat64(entry.data[1])
		if qty > 0 {
			result = append(result, entry.data)
		}
	}
	return result
}

// GetBatchesFromStockLedgerEntries queries batch quantities from Stock Ledger Entries.
// Joins SLE with Batch table, groups by batch_no and warehouse.
func GetBatchesFromStockLedgerEntries(dbConn *db.DB, searchfields []string, txt string, filters map[string]interface{}, start, pageLen int) ([][]interface{}, error) {
	itemCode, _ := filters["item_code"].(string)
	if itemCode == "" {
		return nil, nil
	}

	var args []interface{}

	// Build SELECT clause
	selectCols := "`sle`.`batch_no`, SUM(`sle`.`actual_qty`) AS `qty`"
	selectCols += ", CONCAT('MFG-', `b`.`manufacturing_date`) AS `manufacturing_date`"
	selectCols += ", CONCAT('EXP-', `b`.`expiry_date`) AS `expiry_date`"
	for _, field := range searchfields {
		if err := db.SanitizeIdentifier(field, "search field"); err != nil {
			continue
		}
		selectCols += fmt.Sprintf(", `b`.`%s`", field)
	}

	query := fmt.Sprintf(`SELECT %s
		FROM `+"`tabStock Ledger Entry`"+` sle
		INNER JOIN `+"`tabBatch`"+` b ON b.name = sle.batch_no
		WHERE sle.is_cancelled = 0
		AND sle.item_code = ?
		AND b.disabled = 0
		AND sle.batch_no IS NOT NULL`, selectCols)
	args = append(args, itemCode)

	// Expiry date filter
	includeExpired, _ := filters["include_expired_batches"].(bool)
	if !includeExpired {
		postingDate, _ := filters["posting_date"].(string)
		if postingDate == "" {
			postingDate = "CURDATE()"
			query += " AND (b.expiry_date >= CURDATE() OR b.expiry_date IS NULL)"
		} else {
			query += " AND (b.expiry_date >= ? OR b.expiry_date IS NULL)"
			args = append(args, postingDate)
		}
	}

	// Warehouse filter
	if warehouse, ok := filters["warehouse"].(string); ok && warehouse != "" {
		query += " AND sle.warehouse = ?"
		args = append(args, warehouse)
	}

	// Text search
	if txt != "" {
		txtConditions := []string{"`b`.`name` LIKE ?"}
		args = append(args, "%"+txt+"%")
		for _, field := range searchfields {
			if err := db.SanitizeIdentifier(field, "search field"); err != nil {
				continue
			}
			txtConditions = append(txtConditions, fmt.Sprintf("`b`.`%s` LIKE ?", field))
			args = append(args, "%"+txt+"%")
		}
		// Also search by name again (already included above in the first condition)
		query += " AND (" + strings.Join(txtConditions, " OR ") + ")"
	}

	query += " GROUP BY `sle`.`batch_no`, `sle`.`warehouse`"
	query += " HAVING SUM(`sle`.`actual_qty`) != 0"
	query += fmt.Sprintf(" LIMIT %d OFFSET %d", pageLen, start)

	rows, err := dbConn.RawQuery(query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying batches from SLE: %w", err)
	}
	defer rows.Close()

	return ScanRows(rows)
}

// GetBatchesFromSerialAndBatchBundle queries batch quantities from Serial and Batch Entries
// joined with Stock Ledger Entries and Batch table.
func GetBatchesFromSerialAndBatchBundle(dbConn *db.DB, searchfields []string, txt string, filters map[string]interface{}, start, pageLen int) ([][]interface{}, error) {
	itemCode, _ := filters["item_code"].(string)
	if itemCode == "" {
		return nil, nil
	}

	var args []interface{}

	// Build SELECT clause
	selectCols := "`sbe`.`batch_no`, SUM(`sbe`.`qty`) AS `qty`"
	selectCols += ", CONCAT('MFG-', `b`.`manufacturing_date`) AS `manufacturing_date`"
	selectCols += ", CONCAT('EXP-', `b`.`expiry_date`) AS `expiry_date`"
	for _, field := range searchfields {
		if err := db.SanitizeIdentifier(field, "search field"); err != nil {
			continue
		}
		selectCols += fmt.Sprintf(", `b`.`%s`", field)
	}

	query := fmt.Sprintf(`SELECT %s
		FROM `+"`tabSerial and Batch Entry`"+` sbe
		INNER JOIN `+"`tabStock Ledger Entry`"+` sle ON sbe.parent = sle.serial_and_batch_bundle
		INNER JOIN `+"`tabBatch`"+` b ON b.name = sbe.batch_no
		WHERE sle.is_cancelled = 0
		AND sle.item_code = ?
		AND b.disabled = 0
		AND sle.serial_and_batch_bundle IS NOT NULL`, selectCols)
	args = append(args, itemCode)

	// Expiry date filter
	includeExpired, _ := filters["include_expired_batches"].(bool)
	if !includeExpired {
		postingDate, _ := filters["posting_date"].(string)
		if postingDate == "" {
			query += " AND (b.expiry_date >= CURDATE() OR b.expiry_date IS NULL)"
		} else {
			query += " AND (b.expiry_date >= ? OR b.expiry_date IS NULL)"
			args = append(args, postingDate)
		}
	}

	// Warehouse filter
	if warehouse, ok := filters["warehouse"].(string); ok && warehouse != "" {
		query += " AND sle.warehouse = ?"
		args = append(args, warehouse)
	}

	// Text search
	if txt != "" {
		txtConditions := []string{"`b`.`name` LIKE ?"}
		args = append(args, "%"+txt+"%")
		for _, field := range searchfields {
			if err := db.SanitizeIdentifier(field, "search field"); err != nil {
				continue
			}
			txtConditions = append(txtConditions, fmt.Sprintf("`b`.`%s` LIKE ?", field))
			args = append(args, "%"+txt+"%")
		}
		query += " AND (" + strings.Join(txtConditions, " OR ") + ")"
	}

	query += " GROUP BY `sbe`.`batch_no`, `sbe`.`warehouse`"
	query += " HAVING SUM(`sbe`.`qty`) != 0"
	query += fmt.Sprintf(" LIMIT %d OFFSET %d", pageLen, start)

	rows, err := dbConn.RawQuery(query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying batches from serial and batch bundle: %w", err)
	}
	defer rows.Close()

	return ScanRows(rows)
}

// GetEmptyBatches queries batches that have no stock ledger entries (empty batches).
// Excludes batches already found in previous queries.
func GetEmptyBatches(dbConn *db.DB, filters map[string]interface{}, start, pageLen int, excludeBatches []string, txt string) ([][]interface{}, error) {
	var args []interface{}
	query := "SELECT `name`, `batch_qty` FROM `tabBatch` WHERE `disabled` = 0"

	if itemCode, ok := filters["item_code"].(string); ok && itemCode != "" {
		query += " AND `item` = ?"
		args = append(args, itemCode)
	}

	if txt != "" {
		query += " AND `name` LIKE ?"
		args = append(args, "%"+txt+"%")
	}

	if len(excludeBatches) > 0 {
		placeholders := make([]string, len(excludeBatches))
		for i, b := range excludeBatches {
			placeholders[i] = "?"
			args = append(args, b)
		}
		query += " AND `name` NOT IN (" + strings.Join(placeholders, ",") + ")"
	}

	query += fmt.Sprintf(" LIMIT %d OFFSET %d", pageLen, start)

	rows, err := dbConn.RawQuery(query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying empty batches: %w", err)
	}
	defer rows.Close()

	return ScanRows(rows)
}

// uniqueStrings returns a deduplicated slice preserving order.
func uniqueStrings(input []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, s := range input {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	return result
}

// toFloat64 converts an interface{} to float64.
func toFloat64(v interface{}) float64 {
	switch val := v.(type) {
	case float64:
		return val
	case float32:
		return float64(val)
	case int:
		return float64(val)
	case int64:
		return float64(val)
	case string:
		f, err := strconv.ParseFloat(val, 64)
		if err != nil {
			return 0
		}
		return f
	default:
		return 0
	}
}
