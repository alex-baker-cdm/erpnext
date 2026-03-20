package queries

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/alex-baker-cdm/erpnext/go-erpnext/pkg/db"
)

// EmployeeQuery handles the employee_query endpoint.
// Searches for active employees by name/employee_name with LOCATE-based relevance ordering.
// Python equivalent: erpnext.controllers.queries.employee_query
func EmployeeQuery(dbConn *db.DB, w http.ResponseWriter, r *http.Request) {
	params, err := ParseSearchParams(r)
	if err != nil {
		WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	fields := GetFields(dbConn, "Employee", []string{"name", "employee_name"})
	searchfield := params.Searchfield
	if searchfield == "" {
		searchfield = "name"
	}
	if err := db.SanitizeIdentifier(searchfield, "searchfield"); err != nil {
		WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Build search conditions
	var searchConds []string
	for _, f := range fields {
		if err := db.SanitizeIdentifier(f, "field"); err != nil {
			continue
		}
		searchConds = append(searchConds, fmt.Sprintf("`%s` LIKE ?", f))
	}

	txt := "%" + params.Txt + "%"
	cleanTxt := strings.ReplaceAll(params.Txt, "%", "")

	var args []interface{}

	// Build filter conditions
	fcond := ""
	if params.Filters != nil {
		fcond = GetFiltersCond(params.Filters, &args)
	}

	mcond := GetMatchCond("Employee")

	fieldList := make([]string, len(fields))
	for i, f := range fields {
		fieldList[i] = fmt.Sprintf("`%s`", f)
	}

	query := fmt.Sprintf(`SELECT %s FROM `+"`tabEmployee`"+`
		WHERE status IN ('Active', 'Suspended')
		AND docstatus < 2
		AND (`+"`%s`"+` LIKE ? OR %s)
		%s %s
		ORDER BY
			(CASE WHEN LOCATE(?, name) > 0 THEN LOCATE(?, name) ELSE 99999 END),
			(CASE WHEN LOCATE(?, employee_name) > 0 THEN LOCATE(?, employee_name) ELSE 99999 END),
			idx DESC, name, employee_name
		LIMIT ? OFFSET ?`,
		strings.Join(fieldList, ", "),
		searchfield,
		strings.Join(searchConds, " OR "),
		fcond, mcond)

	// Build args: search conditions args first, then filter args, then order/limit
	var queryArgs []interface{}
	queryArgs = append(queryArgs, txt) // for searchfield LIKE
	for range searchConds {
		queryArgs = append(queryArgs, txt)
	}
	queryArgs = append(queryArgs, args...)      // filter args
	queryArgs = append(queryArgs, cleanTxt)     // LOCATE name
	queryArgs = append(queryArgs, cleanTxt)     // LOCATE name
	queryArgs = append(queryArgs, cleanTxt)     // LOCATE employee_name
	queryArgs = append(queryArgs, cleanTxt)     // LOCATE employee_name
	queryArgs = append(queryArgs, params.PageLen)
	queryArgs = append(queryArgs, params.Start)

	rows, err := dbConn.RawQuery(query, queryArgs...)
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

// LeadQuery handles the lead_query endpoint.
// Searches for leads that are not converted, with text search on name/lead_name/company_name.
// Python equivalent: erpnext.controllers.queries.lead_query
func LeadQuery(dbConn *db.DB, w http.ResponseWriter, r *http.Request) {
	params, err := ParseSearchParams(r)
	if err != nil {
		WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	fields := GetFields(dbConn, "Lead", []string{"name", "lead_name", "company_name"})
	searchfield := params.Searchfield
	if searchfield == "" {
		searchfield = "name"
	}
	if err := db.SanitizeIdentifier(searchfield, "searchfield"); err != nil {
		WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	txt := "%" + params.Txt + "%"
	cleanTxt := strings.ReplaceAll(params.Txt, "%", "")
	mcond := GetMatchCond("Lead")

	fieldList := make([]string, len(fields))
	for i, f := range fields {
		fieldList[i] = fmt.Sprintf("`%s`", f)
	}

	query := fmt.Sprintf(`SELECT %s FROM `+"`tabLead`"+`
		WHERE docstatus < 2
		AND IFNULL(status, '') != 'Converted'
		AND (`+"`%s`"+` LIKE ?
			OR lead_name LIKE ?
			OR company_name LIKE ?)
		%s
		ORDER BY
			(CASE WHEN LOCATE(?, name) > 0 THEN LOCATE(?, name) ELSE 99999 END),
			(CASE WHEN LOCATE(?, lead_name) > 0 THEN LOCATE(?, lead_name) ELSE 99999 END),
			(CASE WHEN LOCATE(?, company_name) > 0 THEN LOCATE(?, company_name) ELSE 99999 END),
			idx DESC, name, lead_name
		LIMIT ? OFFSET ?`,
		strings.Join(fieldList, ", "),
		searchfield,
		mcond)

	args := []interface{}{
		txt, txt, txt, // LIKE conditions
		cleanTxt, cleanTxt, // LOCATE name
		cleanTxt, cleanTxt, // LOCATE lead_name
		cleanTxt, cleanTxt, // LOCATE company_name
		params.PageLen, params.Start,
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

// BOMQuery handles the bom endpoint.
// Searches for active, submitted BOMs with text search and LOCATE-based ordering.
// Python equivalent: erpnext.controllers.queries.bom
func BOMQuery(dbConn *db.DB, w http.ResponseWriter, r *http.Request) {
	params, err := ParseSearchParams(r)
	if err != nil {
		WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	fields := GetFields(dbConn, "BOM", []string{"name", "item"})
	searchfield := params.Searchfield
	if searchfield == "" {
		searchfield = "name"
	}
	if err := db.SanitizeIdentifier(searchfield, "searchfield"); err != nil {
		WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	txt := "%" + params.Txt + "%"
	cleanTxt := strings.ReplaceAll(params.Txt, "%", "")

	var filterArgs []interface{}
	fcond := GetFiltersCond(params.Filters, &filterArgs)
	mcond := GetMatchCond("BOM")

	fieldList := make([]string, len(fields))
	for i, f := range fields {
		fieldList[i] = fmt.Sprintf("`%s`", f)
	}

	query := fmt.Sprintf(`SELECT %s FROM `+"`tabBOM`"+`
		WHERE `+"`tabBOM`"+`.docstatus = 1
		AND `+"`tabBOM`"+`.is_active = 1
		AND `+"`tabBOM`"+`.`+"`%s`"+` LIKE ?
		%s %s
		ORDER BY
			(CASE WHEN LOCATE(?, name) > 0 THEN LOCATE(?, name) ELSE 99999 END),
			idx DESC, name
		LIMIT ? OFFSET ?`,
		strings.Join(fieldList, ", "),
		searchfield,
		fcond, mcond)

	args := []interface{}{txt}
	args = append(args, filterArgs...)
	args = append(args, cleanTxt, cleanTxt)
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

// GetBatchNumbers handles the get_batch_numbers endpoint.
// Queries batches where disabled=0, expiry_date >= CURRENT_DATE or NULL, name LIKE txt.
// Python equivalent: erpnext.controllers.queries.get_batch_numbers
func GetBatchNumbers(dbConn *db.DB, w http.ResponseWriter, r *http.Request) {
	params, err := ParseSearchParams(r)
	if err != nil {
		WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	var args []interface{}
	query := "SELECT `batch_id` FROM `tabBatch` WHERE `disabled` = 0 AND (`expiry_date` >= CURRENT_DATE OR `expiry_date` IS NULL) AND `name` LIKE ?"
	args = append(args, "%"+params.Txt+"%")

	if params.Filters != nil {
		if item, ok := params.Filters["item"].(string); ok && item != "" {
			query += " AND `item` = ?"
			args = append(args, item)
		}
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

// ItemManufacturerQuery handles the item_manufacturer_query endpoint.
// Queries Item Manufacturer filtered by item_code, manufacturer LIKE txt.
// Python equivalent: erpnext.controllers.queries.item_manufacturer_query
func ItemManufacturerQuery(dbConn *db.DB, w http.ResponseWriter, r *http.Request) {
	params, err := ParseSearchParams(r)
	if err != nil {
		WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	var args []interface{}
	query := "SELECT `manufacturer`, `manufacturer_part_no` FROM `tabItem Manufacturer` WHERE `manufacturer` LIKE ? AND `item_code` = ? LIMIT ? OFFSET ?"
	args = append(args, "%"+params.Txt+"%")

	itemCode := ""
	if params.Filters != nil {
		if ic, ok := params.Filters["item_code"].(string); ok {
			itemCode = ic
		}
	}
	args = append(args, itemCode, params.PageLen, params.Start)

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

// GetPurchaseReceipts handles the get_purchase_receipts endpoint.
// Joins Purchase Receipt with Purchase Receipt Item, docstatus=1.
// Python equivalent: erpnext.controllers.queries.get_purchase_receipts
func GetPurchaseReceipts(dbConn *db.DB, w http.ResponseWriter, r *http.Request) {
	params, err := ParseSearchParams(r)
	if err != nil {
		WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	var args []interface{}
	query := "SELECT `pr`.`name` FROM `tabPurchase Receipt` `pr`, `tabPurchase Receipt Item` `pritem` WHERE `pr`.`docstatus` = 1 AND `pritem`.`parent` = `pr`.`name` AND `pr`.`name` LIKE ?"
	args = append(args, "%"+params.Txt+"%")

	if params.Filters != nil {
		if itemCode, ok := params.Filters["item_code"].(string); ok && itemCode != "" {
			query += " AND `pritem`.`item_code` = ?"
			args = append(args, itemCode)
		}
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

// GetPurchaseInvoices handles the get_purchase_invoices endpoint.
// Joins Purchase Invoice with Purchase Invoice Item, docstatus=1.
// Python equivalent: erpnext.controllers.queries.get_purchase_invoices
func GetPurchaseInvoices(dbConn *db.DB, w http.ResponseWriter, r *http.Request) {
	params, err := ParseSearchParams(r)
	if err != nil {
		WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	var args []interface{}
	query := "SELECT `pi`.`name` FROM `tabPurchase Invoice` `pi`, `tabPurchase Invoice Item` `piitem` WHERE `pi`.`docstatus` = 1 AND `piitem`.`parent` = `pi`.`name` AND `pi`.`name` LIKE ?"
	args = append(args, "%"+params.Txt+"%")

	if params.Filters != nil {
		if itemCode, ok := params.Filters["item_code"].(string); ok && itemCode != "" {
			query += " AND `piitem`.`item_code` = ?"
			args = append(args, itemCode)
		}
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

// periodClosingDoctypes is the hardcoded list of doctypes used for period closing.
var periodClosingDoctypes = []string{
	"Sales Invoice",
	"Purchase Invoice",
	"Sales Order",
	"Purchase Order",
	"Quotation",
	"Delivery Note",
	"Purchase Receipt",
}

// GetDoctypesForClosing handles the get_doctypes_for_closing endpoint.
// Returns a hardcoded list of period_closing_doctypes, filtered by txt.
// Python equivalent: erpnext.controllers.queries.get_doctypes_for_closing
func GetDoctypesForClosing(dbConn *db.DB, w http.ResponseWriter, r *http.Request) {
	params, err := ParseSearchParams(r)
	if err != nil {
		WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	var results [][]interface{}
	seen := make(map[string]bool)
	txt := strings.ToLower(params.Txt)

	for _, d := range periodClosingDoctypes {
		if seen[d] {
			continue
		}
		seen[d] = true
		if txt == "" || strings.Contains(strings.ToLower(d), txt) {
			results = append(results, []interface{}{d})
		}
	}

	WriteJSON(w, results)
}

// GetPaymentTermsForReferences handles the get_payment_terms_for_references endpoint.
// Queries Payment Schedule filtered by parent=reference.
// Python equivalent: erpnext.controllers.queries.get_payment_terms_for_references
func GetPaymentTermsForReferences(dbConn *db.DB, w http.ResponseWriter, r *http.Request) {
	params, err := ParseSearchParams(r)
	if err != nil {
		WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	if params.Filters == nil {
		WriteJSON(w, []interface{}{})
		return
	}

	reference, _ := params.Filters["reference"].(string)
	if reference == "" {
		WriteJSON(w, []interface{}{})
		return
	}

	query := "SELECT `payment_term` FROM `tabPayment Schedule` WHERE `parent` = ? LIMIT ?"
	args := []interface{}{reference, params.PageLen}

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

// GetItemUOMQuery handles the get_item_uom_query endpoint.
// Queries UOM Conversion Detail or UOM based on stock settings.
// Python equivalent: erpnext.controllers.queries.get_item_uom_query
func GetItemUOMQuery(dbConn *db.DB, w http.ResponseWriter, r *http.Request) {
	params, err := ParseSearchParams(r)
	if err != nil {
		WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Check stock settings for allow_uom_with_conversion_rate_defined_in_item
	// TODO: Read from Stock Settings singleton when available
	// For now, check via raw query
	var allowUOM string
	row := dbConn.RawQueryRow("SELECT `value` FROM `tabSingles` WHERE `doctype` = 'Stock Settings' AND `field` = 'allow_uom_with_conversion_rate_defined_in_item'")
	row.Scan(&allowUOM)

	if allowUOM == "1" {
		// Query UOM Conversion Detail
		var args []interface{}
		query := "SELECT `uom`, `conversion_factor` FROM `tabUOM Conversion Detail` WHERE `parent` = ?"

		itemCode := ""
		if params.Filters != nil {
			if ic, ok := params.Filters["item_code"].(string); ok {
				itemCode = ic
			}
		}
		args = append(args, itemCode)

		if params.Txt != "" {
			query += " AND `uom` LIKE ?"
			args = append(args, "%"+params.Txt+"%")
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
		return
	}

	// Default: query UOM table
	var args []interface{}
	query := "SELECT `name` FROM `tabUOM` WHERE `enabled` = 1 AND `name` LIKE ?"
	args = append(args, "%"+params.Txt+"%")
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
