package queries

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/alex-baker-cdm/erpnext/go-erpnext/pkg/db"
)

// ItemQuery handles the item_query endpoint — the most complex search endpoint.
// Features: party-specific item filtering via Party Specific Item, barcode search via Item Barcode,
// description scan only if table row count < 50000, LOCATE-based ordering.
// Python equivalent: erpnext.controllers.queries.item_query
func ItemQuery(dbConn *db.DB, w http.ResponseWriter, r *http.Request) {
	params, err := ParseSearchParams(r)
	if err != nil {
		WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	searchfield := params.Searchfield
	if searchfield == "" {
		searchfield = "name"
	}
	if err := db.SanitizeIdentifier(searchfield, "searchfield"); err != nil {
		WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	filters := params.Filters
	if filters == nil {
		filters = make(map[string]interface{})
	}

	// Handle party-specific item filtering
	if customer, ok := filters["customer"].(string); ok && customer != "" {
		applyPartySpecificFilters(dbConn, filters, customer, "Customer")
		delete(filters, "customer")
	} else if supplier, ok := filters["supplier"].(string); ok && supplier != "" {
		applyPartySpecificFilters(dbConn, filters, supplier, "Supplier")
		delete(filters, "supplier")
	} else {
		delete(filters, "customer")
		delete(filters, "supplier")
	}

	// Build searchfields list
	searchfields := []string{searchfield, "item_code", "item_group", "item_name"}
	searchfields = uniqueStrings(searchfields)

	var searchConds []string
	for _, f := range searchfields {
		if err := db.SanitizeIdentifier(f, "searchfield"); err != nil {
			continue
		}
		searchConds = append(searchConds, fmt.Sprintf("`tabItem`.`%s` LIKE ?", f))
	}

	txt := "%" + params.Txt + "%"
	cleanTxt := strings.ReplaceAll(params.Txt, "%", "")
	today := time.Now().Format("2006-01-02")

	// Check if description search should be enabled
	descriptionCond := ""
	count, err := dbConn.EstimateCount("Item")
	if err == nil && count < 50000 {
		descriptionCond = "OR `tabItem`.`description` LIKE ?"
	}

	// Build filter conditions
	var filterArgs []interface{}
	fcond := GetFiltersCond(filters, &filterArgs)
	mcond := GetMatchCond("Item")

	// Build columns (simplified — full implementation would read meta search fields)
	columns := "`tabItem`.`name`"

	// Build the query
	query := fmt.Sprintf(`SELECT %s
		FROM `+"`tabItem`"+`
		WHERE `+"`tabItem`"+`.docstatus < 2
		AND `+"`tabItem`"+`.disabled = 0
		AND `+"`tabItem`"+`.has_variants = 0
		AND (`+"`tabItem`"+`.end_of_life > ? OR IFNULL(`+"`tabItem`"+`.end_of_life, '0000-00-00') = '0000-00-00')
		AND (%s
			OR `+"`tabItem`"+`.item_code IN (SELECT parent FROM `+"`tabItem Barcode`"+` WHERE barcode LIKE ?)
			%s)
		%s %s
		ORDER BY
			IF(LOCATE(?, name), LOCATE(?, name), 99999),
			IF(LOCATE(?, item_name), LOCATE(?, item_name), 99999),
			idx DESC, name, item_name
		LIMIT ?, ?`,
		columns,
		strings.Join(searchConds, " OR "),
		descriptionCond,
		fcond, mcond)

	// Build args
	var args []interface{}
	args = append(args, today) // end_of_life comparison

	// Search condition args
	for range searchConds {
		args = append(args, txt)
	}
	args = append(args, txt) // barcode LIKE

	// Description condition arg
	if descriptionCond != "" {
		args = append(args, txt)
	}

	args = append(args, filterArgs...) // filter args

	// LOCATE args
	args = append(args, cleanTxt, cleanTxt) // LOCATE name
	args = append(args, cleanTxt, cleanTxt) // LOCATE item_name

	args = append(args, params.Start, params.PageLen) // LIMIT offset, count

	rows, err := dbConn.RawQuery(query, args...)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, fmt.Sprintf("query error: %v", err))
		return
	}
	defer rows.Close()

	results, err := ScanRows(rows)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, fmt.Sprintf("scan error: %v", err))
		return
	}

	WriteJSON(w, results)
}

// applyPartySpecificFilters applies party-specific item restrictions to the filters map.
func applyPartySpecificFilters(dbConn *db.DB, filters map[string]interface{}, party, partyType string) {
	// Query Party Specific Item rules for OTHER parties (to exclude their restricted items)
	query := `SELECT restrict_based_on, based_on_value
		FROM ` + "`tabParty Specific Item`" + `
		WHERE party != ? AND party_type = ?`

	rows, err := dbConn.RawQuery(query, party, partyType)
	if err != nil {
		return
	}
	defer rows.Close()

	filtersDict := make(map[string][]interface{})
	for rows.Next() {
		var restrictBasedOn, basedOnValue string
		if err := rows.Scan(&restrictBasedOn, &basedOnValue); err != nil {
			continue
		}
		if restrictBasedOn == "Item" {
			restrictBasedOn = "name"
		}
		// Convert to snake_case (scrub equivalent)
		key := strings.ToLower(strings.ReplaceAll(restrictBasedOn, " ", "_"))
		filtersDict[key] = append(filtersDict[key], basedOnValue)
	}

	for key, vals := range filtersDict {
		filters[key] = []interface{}{"not in", vals}
	}
}

// GetBatchNo handles the get_batch_no endpoint.
// Combines results from stock ledger entries + serial/batch bundles, merges quantities, filters positive qty.
// Python equivalent: erpnext.controllers.queries.get_batch_no
func GetBatchNo(dbConn *db.DB, w http.ResponseWriter, r *http.Request) {
	params, err := ParseSearchParams(r)
	if err != nil {
		WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	if params.Filters == nil {
		WriteError(w, http.StatusBadRequest, "filters required")
		return
	}

	// Override page_len to 300 as in Python
	pageLen := 300

	// Get search fields (simplified — would read from meta in full implementation)
	searchfields := []string{}

	batches, err := GetBatchesFromStockLedgerEntries(dbConn, searchfields, params.Txt, params.Filters, params.Start, pageLen)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, fmt.Sprintf("SLE query error: %v", err))
		return
	}

	bundleBatches, err := GetBatchesFromSerialAndBatchBundle(dbConn, searchfields, params.Txt, params.Filters, params.Start, pageLen)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, fmt.Sprintf("bundle query error: %v", err))
		return
	}

	if bundleBatches != nil {
		batches = append(batches, bundleBatches...)
	}

	filteredBatches := GetFilteredBatches(batches)

	// Add empty batches if is_inward
	if isInward, ok := params.Filters["is_inward"]; ok {
		if isInwardBool, ok := isInward.(bool); ok && isInwardBool {
			excludeNames := make([]string, len(filteredBatches))
			for i, b := range filteredBatches {
				if name, ok := b[0].(string); ok {
					excludeNames[i] = name
				}
			}
			emptyBatches, err := GetEmptyBatches(dbConn, params.Filters, params.Start, pageLen, excludeNames, params.Txt)
			if err == nil && emptyBatches != nil {
				filteredBatches = append(filteredBatches, emptyBatches...)
			}
		}
	}

	WriteJSON(w, filteredBatches)
}

// WarehouseQuery handles the warehouse_query endpoint.
// LEFT JOINs Warehouse with Bin, shows actual_qty, handles title field.
// Python equivalent: erpnext.controllers.queries.warehouse_query
func WarehouseQuery(dbConn *db.DB, w http.ResponseWriter, r *http.Request) {
	params, err := ParseSearchParams(r)
	if err != nil {
		WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	searchfield := params.Searchfield
	if searchfield == "" {
		searchfield = "name"
	}
	if err := db.SanitizeIdentifier(searchfield, "searchfield"); err != nil {
		WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	warehouseField := "name"
	// TODO: Check meta for show_title_field_in_link and title_field
	// For now, use "name" as default

	// Parse list filters for warehouse query
	var binConditions string
	var warehouseConditions string
	var condArgs []interface{}

	if params.RawFilters != "" {
		listFilters, err := ParseListFilters(params.RawFilters)
		if err == nil {
			filterMap := GetDoctypeWiseFilters(listFilters)
			// Build bin conditions
			if binFilters, ok := filterMap["Bin"]; ok {
				for _, f := range binFilters {
					if len(f) >= 4 {
						field, _ := f[1].(string)
						op, _ := f[2].(string)
						val := f[3]
						if err := db.SanitizeIdentifier(field, "bin filter field"); err != nil {
							continue
						}
						safeOp, err := SanitizeOperator(op)
						if err != nil {
							continue
						}
						binConditions += fmt.Sprintf(" AND `tabBin`.`%s` %s ?", field, safeOp)
						condArgs = append(condArgs, val)
					}
				}
			}
			// Build warehouse conditions
			if whFilters, ok := filterMap["Warehouse"]; ok {
				for _, f := range whFilters {
					if len(f) >= 4 {
						field, _ := f[1].(string)
						op, _ := f[2].(string)
						val := f[3]
						if err := db.SanitizeIdentifier(field, "warehouse filter field"); err != nil {
							continue
						}
						safeOp, err := SanitizeOperator(op)
						if err != nil {
							continue
						}
						warehouseConditions += fmt.Sprintf(" AND `tabWarehouse`.`%s` %s ?", field, safeOp)
						condArgs = append(condArgs, val)
					}
				}
			}
		}
	}

	mcond := GetMatchCond("Warehouse")

	if err := db.SanitizeIdentifier(warehouseField, "warehouse field"); err != nil {
		WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	query := fmt.Sprintf(`SELECT `+"`tabWarehouse`"+`.`+"`%s`"+`,
		CONCAT_WS(' : ', 'Actual Qty', IFNULL(ROUND(`+"`tabBin`"+`.actual_qty, 2), 0)) AS actual_qty
		FROM `+"`tabWarehouse`"+` LEFT JOIN `+"`tabBin`"+`
		ON `+"`tabBin`"+`.warehouse = `+"`tabWarehouse`"+`.name %s
		WHERE `+"`tabWarehouse`"+`.`+"`%s`"+` LIKE ?
		%s %s
		ORDER BY IFNULL(`+"`tabBin`"+`.actual_qty, 0) DESC, `+"`tabWarehouse`"+`.`+"`%s`"+` ASC
		LIMIT ? OFFSET ?`,
		warehouseField,
		binConditions,
		searchfield,
		warehouseConditions, mcond,
		warehouseField)

	var args []interface{}
	args = append(args, condArgs...)
	args = append(args, "%"+params.Txt+"%")
	args = append(args, params.PageLen, params.Start)

	rows, err := dbConn.RawQuery(query, args...)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, fmt.Sprintf("query error: %v", err))
		return
	}
	defer rows.Close()

	results, err := ScanRows(rows)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, fmt.Sprintf("scan error: %v", err))
		return
	}

	WriteJSON(w, results)
}

// GetTaxTemplate handles the get_tax_template endpoint.
// Item tax template hierarchy: check item taxes, walk up item_group tree, fallback to all templates.
// Python equivalent: erpnext.controllers.queries.get_tax_template
func GetTaxTemplate(dbConn *db.DB, w http.ResponseWriter, r *http.Request) {
	params, err := ParseSearchParams(r)
	if err != nil {
		WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	if params.Filters == nil {
		WriteError(w, http.StatusBadRequest, "filters required")
		return
	}

	itemCode, _ := params.Filters["item_code"].(string)
	company, _ := params.Filters["company"].(string)
	itemGroup, _ := params.Filters["item_group"].(string)

	// Get item taxes
	var itemTaxes []string
	itemTaxRows, err := dbConn.RawQuery(
		"SELECT `item_tax_template` FROM `tabItem Tax` WHERE `parent` = ?", itemCode)
	if err == nil {
		defer itemTaxRows.Close()
		for itemTaxRows.Next() {
			var tmpl string
			if err := itemTaxRows.Scan(&tmpl); err == nil && tmpl != "" {
				itemTaxes = append(itemTaxes, tmpl)
			}
		}
	}

	// Walk up item_group tree
	currentGroup := itemGroup
	for currentGroup != "" {
		groupTaxRows, err := dbConn.RawQuery(
			"SELECT `item_tax_template` FROM `tabItem Tax` WHERE `parenttype` = 'Item Group' AND `parent` = ?",
			currentGroup)
		if err != nil {
			break
		}
		for groupTaxRows.Next() {
			var tmpl string
			if err := groupTaxRows.Scan(&tmpl); err == nil && tmpl != "" {
				itemTaxes = append(itemTaxes, tmpl)
			}
		}
		groupTaxRows.Close()

		// Get parent item_group
		var parentGroup string
		row := dbConn.RawQueryRow(
			"SELECT `parent_item_group` FROM `tabItem Group` WHERE `name` = ?",
			currentGroup)
		if err := row.Scan(&parentGroup); err != nil || parentGroup == currentGroup {
			break
		}
		currentGroup = parentGroup
	}

	if len(itemTaxes) == 0 {
		// Fallback: return all templates for the company
		var args []interface{}
		query := "SELECT `name` FROM `tabItem Tax Template` WHERE `disabled` = 0 AND `company` = ?"
		args = append(args, company)

		if params.Txt != "" {
			query += " AND `name` LIKE ?"
			args = append(args, "%"+params.Txt+"%")
		}

		rows, err := dbConn.RawQuery(query, args...)
		if err != nil {
			WriteError(w, http.StatusInternalServerError, fmt.Sprintf("query error: %v", err))
			return
		}
		defer rows.Close()

		results, err := ScanRows(rows)
		if err != nil {
			WriteError(w, http.StatusInternalServerError, fmt.Sprintf("scan error: %v", err))
			return
		}

		WriteJSON(w, results)
		return
	}

	// Filter item taxes by txt
	txt := strings.ToLower(params.Txt)
	seen := make(map[string]bool)
	var results [][]interface{}
	for _, tmpl := range itemTaxes {
		if seen[tmpl] {
			continue
		}
		seen[tmpl] = true
		if txt == "" || strings.Contains(strings.ToLower(tmpl), txt) {
			results = append(results, []interface{}{tmpl})
		}
	}

	WriteJSON(w, results)
}

// GetFilteredDimensions handles the get_filtered_dimensions endpoint.
// Accounting dimension filter with allow/restrict logic.
// Python equivalent: erpnext.controllers.queries.get_filtered_dimensions
func GetFilteredDimensions(dbConn *db.DB, w http.ResponseWriter, r *http.Request) {
	params, err := ParseSearchParams(r)
	if err != nil {
		WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	doctype := params.Doctype
	if doctype == "" {
		WriteError(w, http.StatusBadRequest, "doctype required")
		return
	}
	if err := db.SanitizeIdentifier(doctype, "doctype"); err != nil {
		WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	var conditions []string
	var args []interface{}

	// TODO: Check meta for is_tree, has_field("is_group"), has_field("disabled"), has_field("company")
	// For now, add common conditions

	if params.Filters != nil {
		if company, ok := params.Filters["company"].(string); ok && company != "" {
			conditions = append(conditions, "`company` = ?")
			args = append(args, company)
		}
	}

	// Text search on name
	var orConditions []string
	if params.Txt != "" {
		orConditions = append(orConditions, "`name` LIKE ?")
		args = append(args, "%"+params.Txt+"%")
	}

	// Dimension filter logic
	if params.Filters != nil {
		dimension, _ := params.Filters["dimension"].(string)
		account, _ := params.Filters["account"].(string)
		if dimension != "" && account != "" {
			// Query accounting dimension filter map
			dimFilterRows, err := dbConn.RawQuery(
				`SELECT adf.allow_or_restrict, adfa.dimension_value
				FROM `+"`tabAccounting Dimension Filter`"+` adf
				INNER JOIN `+"`tabAllowed Dimension`"+` adfa ON adfa.parent = adf.name
				WHERE adf.accounting_dimension = ?
				AND adf.disabled = 0
				AND (adf.apply_to_all_doctypes = 1 OR adf.name IN (
					SELECT parent FROM `+"`tabApplicable On Account`"+` WHERE applicable_on_account = ?
				))`,
				dimension, account)
			if err == nil {
				defer dimFilterRows.Close()

				var allowOrRestrict string
				var dimensionValues []interface{}
				for dimFilterRows.Next() {
					var aor, dv string
					if err := dimFilterRows.Scan(&aor, &dv); err == nil {
						allowOrRestrict = aor
						dimensionValues = append(dimensionValues, dv)
					}
				}

				if len(dimensionValues) > 0 {
					var selector string
					if allowOrRestrict == "Allow" {
						selector = "IN"
					} else {
						selector = "NOT IN"
					}
					placeholders := make([]string, len(dimensionValues))
					for i := range dimensionValues {
						placeholders[i] = "?"
					}
					conditions = append(conditions, fmt.Sprintf("`name` %s (%s)", selector, strings.Join(placeholders, ",")))
					args = append(args, dimensionValues...)
				}
			}
		}
	}

	query := fmt.Sprintf("SELECT `name` FROM `tab%s`", doctype)
	if len(conditions) > 0 || len(orConditions) > 0 {
		query += " WHERE "
		var allConds []string
		allConds = append(allConds, conditions...)
		if len(orConditions) > 0 {
			allConds = append(allConds, "("+strings.Join(orConditions, " OR ")+")")
		}
		query += strings.Join(allConds, " AND ")
	}

	rows, err := dbConn.RawQuery(query, args...)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, fmt.Sprintf("query error: %v", err))
		return
	}
	defer rows.Close()

	results, err := ScanRows(rows)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, fmt.Sprintf("scan error: %v", err))
		return
	}

	// Deduplicate results
	seen := make(map[string]bool)
	var deduped [][]interface{}
	for _, row := range results {
		if len(row) > 0 {
			key := fmt.Sprintf("%v", row[0])
			if !seen[key] {
				seen[key] = true
				deduped = append(deduped, row)
			}
		}
	}

	WriteJSON(w, deduped)
}

// GetFilteredChildRows handles the get_filtered_child_rows endpoint.
// Dynamic doctype query with idx/item_code display.
// Python equivalent: erpnext.controllers.queries.get_filtered_child_rows
func GetFilteredChildRows(dbConn *db.DB, w http.ResponseWriter, r *http.Request) {
	params, err := ParseSearchParams(r)
	if err != nil {
		WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	doctype := params.Doctype
	if doctype == "" {
		WriteError(w, http.StatusBadRequest, "doctype required")
		return
	}
	if err := db.SanitizeIdentifier(doctype, "doctype"); err != nil {
		WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	var args []interface{}
	query := fmt.Sprintf("SELECT `name`, CONCAT('#', `idx`, ', ', `item_code`) FROM `tab%s`", doctype)

	var conditions []string
	if params.Filters != nil {
		for field, value := range params.Filters {
			if err := db.SanitizeIdentifier(field, "filter field"); err != nil {
				continue
			}
			conditions = append(conditions, fmt.Sprintf("`%s` = ?", field))
			args = append(args, value)
		}
	}

	if params.Txt != "" {
		txtSearch := params.Txt + "%"
		txtNoHash := strings.ReplaceAll(params.Txt, "#", "") + "%"
		txtCond := "(`idx` LIKE ? OR `item_code` LIKE ? OR `name` LIKE ?)"
		conditions = append(conditions, txtCond)
		args = append(args, txtNoHash, txtSearch, txtSearch)
	}

	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}

	query += " ORDER BY `idx`"
	query += fmt.Sprintf(" LIMIT %d OFFSET %d", params.PageLen, params.Start)

	rows, err := dbConn.RawQuery(query, args...)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, fmt.Sprintf("query error: %v", err))
		return
	}
	defer rows.Close()

	results, err := ScanRows(rows)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, fmt.Sprintf("scan error: %v", err))
		return
	}

	WriteJSON(w, results)
}

// GetProjectName handles the get_project_name endpoint.
// Queries Project with customer/company/status filters, LOCATE ordering.
// Python equivalent: erpnext.controllers.queries.get_project_name
func GetProjectName(dbConn *db.DB, w http.ResponseWriter, r *http.Request) {
	params, err := ParseSearchParams(r)
	if err != nil {
		WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	fields := GetFields(dbConn, "Project", []string{"name", "project_name"})

	var conditions []string
	var orConditions []string
	var args []interface{}

	// AND conditions
	if params.Filters != nil {
		if customer, ok := params.Filters["customer"].(string); ok && customer != "" {
			conditions = append(conditions, "(`customer` = ? OR `customer` IS NULL OR `customer` = '')")
			args = append(args, customer)
		}
		if company, ok := params.Filters["company"].(string); ok && company != "" {
			conditions = append(conditions, "`company` = ?")
			args = append(args, company)
		}
	}

	conditions = append(conditions, "`status` NOT IN ('Completed', 'Cancelled')")

	// OR conditions for text search (exclude customer and status from search)
	if params.Txt != "" {
		// Search by name and project_name
		orConditions = append(orConditions, "`name` LIKE ?")
		args = append(args, "%"+params.Txt+"%")
		orConditions = append(orConditions, "`project_name` LIKE ?")
		args = append(args, "%"+params.Txt+"%")
	}

	fieldList := make([]string, len(fields))
	for i, f := range fields {
		if err := db.SanitizeIdentifier(f, "field"); err != nil {
			fieldList[i] = "`name`"
			continue
		}
		fieldList[i] = fmt.Sprintf("`%s`", f)
	}

	query := fmt.Sprintf("SELECT %s FROM `tabProject`", strings.Join(fieldList, ", "))

	var whereParts []string
	whereParts = append(whereParts, conditions...)
	if len(orConditions) > 0 {
		whereParts = append(whereParts, "("+strings.Join(orConditions, " OR ")+")")
	}

	if len(whereParts) > 0 {
		query += " WHERE " + strings.Join(whereParts, " AND ")
	}

	// Ordering
	cleanTxt := strings.ReplaceAll(params.Txt, "%", "")
	if params.Txt != "" {
		query += " ORDER BY IF(LOCATE(?, `project_name`) > 0, LOCATE(?, `project_name`), 99999), `idx` DESC, `name`"
		args = append(args, cleanTxt, cleanTxt)
	} else {
		query += " ORDER BY `idx` DESC, `name`"
	}

	if params.PageLen > 0 {
		query += fmt.Sprintf(" LIMIT %d", params.PageLen)
	}
	if params.Start > 0 {
		query += fmt.Sprintf(" OFFSET %d", params.Start)
	}

	rows, err := dbConn.RawQuery(query, args...)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, fmt.Sprintf("query error: %v", err))
		return
	}
	defer rows.Close()

	results, err := ScanRows(rows)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, fmt.Sprintf("scan error: %v", err))
		return
	}

	WriteJSON(w, results)
}

// GetDeliveryNotesToBeBilled handles the get_delivery_notes_to_be_billed endpoint.
// Complex billing status: per_billed < 100, is_return handling.
// Python equivalent: erpnext.controllers.queries.get_delivery_notes_to_be_billed
func GetDeliveryNotesToBeBilled(dbConn *db.DB, w http.ResponseWriter, r *http.Request) {
	params, err := ParseSearchParams(r)
	if err != nil {
		WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	searchfield := params.Searchfield
	if searchfield == "" {
		searchfield = "name"
	}
	if err := db.SanitizeIdentifier(searchfield, "searchfield"); err != nil {
		WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	fields := GetFields(dbConn, "Delivery Note", []string{"name", "customer", "posting_date"})

	fieldList := make([]string, len(fields))
	for i, f := range fields {
		if err := db.SanitizeIdentifier(f, "field"); err != nil {
			fieldList[i] = "`tabDelivery Note`.`name`"
			continue
		}
		fieldList[i] = fmt.Sprintf("`tabDelivery Note`.`%s`", f)
	}

	var filterArgs []interface{}
	fcond := GetFiltersCond(params.Filters, &filterArgs)
	mcond := GetMatchCond("Delivery Note")

	txt := "%" + params.Txt + "%"

	query := fmt.Sprintf(`SELECT %s
		FROM `+"`tabDelivery Note`"+`
		WHERE `+"`tabDelivery Note`"+`.`+"`%s`"+` LIKE ?
		AND `+"`tabDelivery Note`"+`.docstatus = 1
		AND status NOT IN ('Stopped', 'Closed')
		%s
		AND (
			(`+"`tabDelivery Note`"+`.is_return = 0 AND `+"`tabDelivery Note`"+`.per_billed < 100)
			OR (`+"`tabDelivery Note`"+`.grand_total = 0 AND `+"`tabDelivery Note`"+`.per_billed < 100)
			OR (
				`+"`tabDelivery Note`"+`.is_return = 1
				AND return_against IN (SELECT name FROM `+"`tabDelivery Note`"+` WHERE per_billed < 100)
			)
		)
		%s
		ORDER BY `+"`tabDelivery Note`"+`.`+"`%s`"+` ASC
		LIMIT ? OFFSET ?`,
		strings.Join(fieldList, ", "),
		searchfield,
		fcond,
		mcond,
		searchfield)

	var args []interface{}
	args = append(args, txt)
	args = append(args, filterArgs...)
	args = append(args, params.PageLen, params.Start)

	rows, err := dbConn.RawQuery(query, args...)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, fmt.Sprintf("query error: %v", err))
		return
	}
	defer rows.Close()

	results, err := ScanRows(rows)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, fmt.Sprintf("scan error: %v", err))
		return
	}

	WriteJSON(w, results)
}
