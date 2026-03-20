package queries

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/alex-baker-cdm/erpnext/go-erpnext/pkg/db"
)

// TaxAccountQuery handles the tax_account_query endpoint.
// Two-pass query: first with account_type filter, fallback without.
// Queries Account with company currency match, is_group=0, disabled filter.
// Python equivalent: erpnext.controllers.queries.tax_account_query
func TaxAccountQuery(dbConn *db.DB, w http.ResponseWriter, r *http.Request) {
	params, err := ParseSearchParams(r)
	if err != nil {
		WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	if params.Filters == nil {
		WriteError(w, http.StatusBadRequest, "filters required")
		return
	}

	company, _ := params.Filters["company"].(string)
	disabled := "0"
	if d, ok := params.Filters["disabled"]; ok {
		disabled = fmt.Sprintf("%v", d)
	}

	searchfield := params.Searchfield
	if searchfield == "" {
		searchfield = "name"
	}
	if err := db.SanitizeIdentifier(searchfield, "searchfield"); err != nil {
		WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Get company currency
	var companyCurrency string
	row := dbConn.RawQueryRow("SELECT `default_currency` FROM `tabCompany` WHERE `name` = ?", company)
	row.Scan(&companyCurrency)

	mcond := GetMatchCond("Account")

	getAccounts := func(withAccountTypeFilter bool) ([][]interface{}, error) {
		var args []interface{}
		accountTypeCond := ""
		if withAccountTypeFilter {
			if accountTypes, ok := params.Filters["account_type"]; ok {
				switch at := accountTypes.(type) {
				case []interface{}:
					if len(at) > 0 {
						placeholders := make([]string, len(at))
						for i, v := range at {
							placeholders[i] = "?"
							args = append(args, v)
						}
						accountTypeCond = "AND `account_type` IN (" + strings.Join(placeholders, ",") + ")"
					}
				case string:
					accountTypeCond = "AND `account_type` = ?"
					args = append(args, at)
				}
			}
		}

		query := fmt.Sprintf(`SELECT name, parent_account
			FROM `+"`tabAccount`"+`
			WHERE docstatus != 2
			%s
			AND is_group = 0
			AND company = ?
			AND disabled = ?
			AND (account_currency = ? OR IFNULL(account_currency, '') = '')
			AND `+"`%s`"+` LIKE ?
			%s
			ORDER BY idx DESC, name
			LIMIT ? OFFSET ?`,
			accountTypeCond,
			searchfield,
			mcond)

		args = append(args, company, disabled, companyCurrency, "%"+params.Txt+"%", params.PageLen, params.Start)

		rows, err := dbConn.RawQuery(query, args...)
		if err != nil {
			return nil, fmt.Errorf("querying tax accounts: %w", err)
		}
		defer rows.Close()

		return ScanRows(rows)
	}

	// First pass: with account_type filter
	results, err := getAccounts(true)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, fmt.Sprintf("query error: %v", err))
		return
	}

	// Fallback: without account_type filter
	if len(results) == 0 {
		results, err = getAccounts(false)
		if err != nil {
			WriteError(w, http.StatusInternalServerError, fmt.Sprintf("query error: %v", err))
			return
		}
	}

	WriteJSON(w, results)
}

// GetAccountList handles the get_account_list endpoint.
// Dynamic filter building from dict/list filters, default is_group=0.
// Python equivalent: erpnext.controllers.queries.get_account_list
func GetAccountList(dbConn *db.DB, w http.ResponseWriter, r *http.Request) {
	params, err := ParseSearchParams(r)
	if err != nil {
		WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Build filter list from dict or list format
	type filterEntry struct {
		doctype string
		field   string
		op      string
		value   interface{}
	}

	var filterList []filterEntry

	if params.Filters != nil {
		// Dict format
		for key, val := range params.Filters {
			switch v := val.(type) {
			case []interface{}:
				if len(v) == 2 {
					op, _ := v[0].(string)
					filterList = append(filterList, filterEntry{"Account", key, op, v[1]})
				}
			default:
				filterList = append(filterList, filterEntry{"Account", key, "=", val})
			}
		}
	} else if params.RawFilters != "" {
		// Try list format
		listFilters, err := ParseListFilters(params.RawFilters)
		if err == nil {
			for _, f := range listFilters {
				if len(f) >= 4 {
					dt, _ := f[0].(string)
					field, _ := f[1].(string)
					op, _ := f[2].(string)
					filterList = append(filterList, filterEntry{dt, field, op, f[3]})
				}
			}
		}
	}

	// Add default is_group=0 if not present
	hasIsGroup := false
	for _, f := range filterList {
		if f.field == "is_group" {
			hasIsGroup = true
			break
		}
	}
	if !hasIsGroup {
		filterList = append(filterList, filterEntry{"Account", "is_group", "=", "0"})
	}

	// Add searchfield filter
	if params.Searchfield != "" && params.Txt != "" {
		filterList = append(filterList, filterEntry{"Account", params.Searchfield, "like", "%" + params.Txt + "%"})
	}

	// Build query
	var conditions []string
	var args []interface{}

	for _, f := range filterList {
		if err := db.SanitizeIdentifier(f.field, "filter field"); err != nil {
			continue
		}
		op := strings.ToUpper(strings.TrimSpace(f.op))
		switch op {
		case "LIKE":
			conditions = append(conditions, fmt.Sprintf("`%s` LIKE ?", f.field))
			args = append(args, f.value)
		case "=":
			conditions = append(conditions, fmt.Sprintf("`%s` = ?", f.field))
			args = append(args, f.value)
		case "!=":
			conditions = append(conditions, fmt.Sprintf("`%s` != ?", f.field))
			args = append(args, f.value)
		case "IN":
			if vals, ok := f.value.([]interface{}); ok && len(vals) > 0 {
				placeholders := make([]string, len(vals))
				for i, v := range vals {
					placeholders[i] = "?"
					args = append(args, v)
				}
				conditions = append(conditions, fmt.Sprintf("`%s` IN (%s)", f.field, strings.Join(placeholders, ",")))
			}
		case "NOT IN":
			if vals, ok := f.value.([]interface{}); ok && len(vals) > 0 {
				placeholders := make([]string, len(vals))
				for i, v := range vals {
					placeholders[i] = "?"
					args = append(args, v)
				}
				conditions = append(conditions, fmt.Sprintf("`%s` NOT IN (%s)", f.field, strings.Join(placeholders, ",")))
			}
		default:
			conditions = append(conditions, fmt.Sprintf("`%s` = ?", f.field))
			args = append(args, f.value)
		}
	}

	query := "SELECT `name`, `parent_account` FROM `tabAccount`"
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
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

// GetIncomeAccount handles the get_income_account endpoint.
// Account where report_type='Profit and Loss' OR account_type IN ('Income Account','Temporary'),
// is_group=0, disabled=0.
// Python equivalent: erpnext.controllers.queries.get_income_account
func GetIncomeAccount(dbConn *db.DB, w http.ResponseWriter, r *http.Request) {
	params, err := ParseSearchParams(r)
	if err != nil {
		WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	var args []interface{}
	query := `SELECT name FROM ` + "`tabAccount`" + `
		WHERE (report_type = 'Profit and Loss' OR account_type IN ('Income Account', 'Temporary'))
		AND is_group = 0
		AND disabled = 0`

	if params.Txt != "" {
		query += " AND `name` LIKE ?"
		args = append(args, "%"+params.Txt+"%")
	}

	if params.Filters != nil {
		if company, ok := params.Filters["company"].(string); ok && company != "" {
			query += " AND `company` = ?"
			args = append(args, company)
		}
	}

	// TODO: build_qb_match_conditions for permission filtering
	query += " ORDER BY idx DESC, name"

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

// GetExpenseAccount handles the get_expense_account endpoint.
// Account where report_type='Profit and Loss' OR account_type IN
// ('Expense Account','Fixed Asset','Temporary','Asset Received But Not Billed','Capital Work in Progress'),
// is_group=0, disabled=0.
// Python equivalent: erpnext.controllers.queries.get_expense_account
func GetExpenseAccount(dbConn *db.DB, w http.ResponseWriter, r *http.Request) {
	params, err := ParseSearchParams(r)
	if err != nil {
		WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	var args []interface{}
	query := `SELECT name FROM ` + "`tabAccount`" + `
		WHERE (report_type = 'Profit and Loss' OR account_type IN ('Expense Account', 'Fixed Asset', 'Temporary', 'Asset Received But Not Billed', 'Capital Work in Progress'))
		AND is_group = 0
		AND disabled = 0`

	if params.Txt != "" {
		query += " AND `name` LIKE ?"
		args = append(args, "%"+params.Txt+"%")
	}

	if params.Filters != nil {
		if company, ok := params.Filters["company"].(string); ok && company != "" {
			query += " AND `company` = ?"
			args = append(args, company)
		}
	}

	// TODO: build_qb_match_conditions for permission filtering

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

// GetBlanketOrders handles the get_blanket_orders endpoint.
// Joins Blanket Order with Blanket Order Item, filtered by item, blanket_order_type, company, docstatus=1.
// Python equivalent: erpnext.controllers.queries.get_blanket_orders
func GetBlanketOrders(dbConn *db.DB, w http.ResponseWriter, r *http.Request) {
	params, err := ParseSearchParams(r)
	if err != nil {
		WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	if params.Filters == nil {
		WriteError(w, http.StatusBadRequest, "filters required")
		return
	}

	item, _ := params.Filters["item"].(string)
	blanketOrderType, _ := params.Filters["blanket_order_type"].(string)
	company, _ := params.Filters["company"].(string)

	query := `SELECT DISTINCT bo.name, bo.blanket_order_type, bo.to_date
		FROM ` + "`tabBlanket Order`" + ` bo, ` + "`tabBlanket Order Item`" + ` boi
		WHERE boi.parent = bo.name
		AND boi.item_code = ?
		AND bo.blanket_order_type = ?
		AND bo.company = ?
		AND bo.docstatus = 1`

	args := []interface{}{item, blanketOrderType, company}

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
