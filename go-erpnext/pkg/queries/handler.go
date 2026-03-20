// Package queries provides Go HTTP handlers for ERPNext search query endpoints.
// Each handler corresponds to a @frappe.whitelist() function in erpnext/controllers/queries.py.
package queries

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/alex-baker-cdm/erpnext/go-erpnext/pkg/db"
)

// SearchParams holds the common parameters passed to all search query endpoints.
type SearchParams struct {
	Doctype     string
	Txt         string
	Searchfield string
	Start       int
	PageLen     int
	Filters     map[string]interface{}
	// RawFilters holds the unparsed filters value for endpoints that need list-of-lists format.
	RawFilters string
}

// ParseSearchParams extracts search parameters from an HTTP request.
// Parameters can come from query string or form body (application/x-www-form-urlencoded).
func ParseSearchParams(r *http.Request) (*SearchParams, error) {
	if err := r.ParseForm(); err != nil {
		return nil, fmt.Errorf("parsing form: %w", err)
	}

	start, _ := strconv.Atoi(r.FormValue("start"))
	pageLen, _ := strconv.Atoi(r.FormValue("page_len"))
	if pageLen == 0 {
		pageLen = 20
	}

	sp := &SearchParams{
		Doctype:     r.FormValue("doctype"),
		Txt:         r.FormValue("txt"),
		Searchfield: r.FormValue("searchfield"),
		Start:       start,
		PageLen:     pageLen,
		RawFilters:  r.FormValue("filters"),
	}

	// Parse filters from JSON string
	filtersStr := r.FormValue("filters")
	if filtersStr != "" {
		var filters map[string]interface{}
		if err := json.Unmarshal([]byte(filtersStr), &filters); err != nil {
			// Filters might be a list-of-lists; store raw and leave map nil
			sp.Filters = nil
		} else {
			sp.Filters = filters
		}
	}

	return sp, nil
}

// ParseListFilters parses filters as a list of lists (used by warehouse_query etc.).
func ParseListFilters(raw string) ([][]interface{}, error) {
	if raw == "" {
		return nil, nil
	}
	var filters [][]interface{}
	if err := json.Unmarshal([]byte(raw), &filters); err != nil {
		return nil, fmt.Errorf("parsing list filters: %w", err)
	}
	return filters, nil
}

// WriteJSON writes a standard Frappe JSON response: {"message": data}.
func WriteJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	resp := map[string]interface{}{"message": data}
	json.NewEncoder(w).Encode(resp)
}

// WriteError writes an error response in JSON format.
func WriteError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	resp := map[string]string{"error": msg}
	json.NewEncoder(w).Encode(resp)
}

// EscapeLike escapes SQL LIKE wildcards (% and _) in a string.
func EscapeLike(txt string) string {
	txt = strings.ReplaceAll(txt, "%", "\\%")
	txt = strings.ReplaceAll(txt, "_", "\\_")
	return txt
}

// GetMatchCond returns a match condition string for permission filtering.
// TODO: Full permission integration requires Frappe session context.
// Returns empty string as placeholder.
func GetMatchCond(doctype string) string {
	return ""
}

// GetFiltersCond builds a WHERE clause fragment from filters.
// Returns a string like " AND field1 = ? AND field2 = ?" and appends
// the corresponding values to the args slice.
func GetFiltersCond(filters map[string]interface{}, args *[]interface{}) string {
	if len(filters) == 0 {
		return ""
	}

	var conditions []string
	for field, value := range filters {
		if err := db.SanitizeIdentifier(field, "filter field"); err != nil {
			continue
		}
		// Handle operator-value pairs like ["like", "%foo%"]
		if arr, ok := value.([]interface{}); ok && len(arr) == 2 {
			op, opOk := arr[0].(string)
			if opOk {
				op = strings.ToUpper(strings.TrimSpace(op))
				switch op {
				case "LIKE":
					conditions = append(conditions, fmt.Sprintf("`%s` LIKE ?", field))
					*args = append(*args, arr[1])
				case "NOT IN":
					if vals, ok := arr[1].([]interface{}); ok && len(vals) > 0 {
						placeholders := make([]string, len(vals))
						for i, v := range vals {
							placeholders[i] = "?"
							*args = append(*args, v)
						}
						conditions = append(conditions, fmt.Sprintf("`%s` NOT IN (%s)", field, strings.Join(placeholders, ",")))
					}
				case "IN":
					if vals, ok := arr[1].([]interface{}); ok && len(vals) > 0 {
						placeholders := make([]string, len(vals))
						for i, v := range vals {
							placeholders[i] = "?"
							*args = append(*args, v)
						}
						conditions = append(conditions, fmt.Sprintf("`%s` IN (%s)", field, strings.Join(placeholders, ",")))
					}
				case "!=":
					conditions = append(conditions, fmt.Sprintf("`%s` != ?", field))
					*args = append(*args, arr[1])
				case "=":
					conditions = append(conditions, fmt.Sprintf("`%s` = ?", field))
					*args = append(*args, arr[1])
				case ">":
					conditions = append(conditions, fmt.Sprintf("`%s` > ?", field))
					*args = append(*args, arr[1])
				case "<":
					conditions = append(conditions, fmt.Sprintf("`%s` < ?", field))
					*args = append(*args, arr[1])
				case ">=":
					conditions = append(conditions, fmt.Sprintf("`%s` >= ?", field))
					*args = append(*args, arr[1])
				case "<=":
					conditions = append(conditions, fmt.Sprintf("`%s` <= ?", field))
					*args = append(*args, arr[1])
				default:
					conditions = append(conditions, fmt.Sprintf("`%s` = ?", field))
					*args = append(*args, value)
				}
				continue
			}
		}
		conditions = append(conditions, fmt.Sprintf("`%s` = ?", field))
		*args = append(*args, value)
	}

	if len(conditions) == 0 {
		return ""
	}
	return " AND " + strings.Join(conditions, " AND ")
}

// MakeHandler wraps a query function into an http.HandlerFunc.
// If dbConn is nil, handlers that require a database will return 503.
func MakeHandler(dbConn *db.DB, fn func(*db.DB, http.ResponseWriter, *http.Request)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if dbConn == nil {
			WriteError(w, http.StatusServiceUnavailable, "database connection not configured")
			return
		}
		fn(dbConn, w, r)
	}
}

// validSQLOperators is the set of SQL operators allowed in user-supplied filters.
var validSQLOperators = map[string]bool{
	"=":      true,
	"!=":     true,
	"<":      true,
	">":      true,
	"<=":     true,
	">=":     true,
	"LIKE":   true,
	"IN":     true,
	"NOT IN": true,
}

// SanitizeOperator validates that an SQL operator is in the allowed set.
// Returns the uppercased operator or an error if it is not allowed.
func SanitizeOperator(op string) (string, error) {
	normalized := strings.ToUpper(strings.TrimSpace(op))
	if !validSQLOperators[normalized] {
		return "", fmt.Errorf("invalid SQL operator: %q", op)
	}
	return normalized, nil
}

// ScanRows scans sql.Rows into a slice of string slices (tuple-like format).
func ScanRows(rows interface{ Next() bool; Columns() ([]string, error); Scan(...interface{}) error }) ([][]interface{}, error) {
	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("getting columns: %w", err)
	}

	var results [][]interface{}
	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}
		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, fmt.Errorf("scanning row: %w", err)
		}

		row := make([]interface{}, len(columns))
		for i, val := range values {
			if b, ok := val.([]byte); ok {
				row[i] = string(b)
			} else {
				row[i] = val
			}
		}
		results = append(results, row)
	}
	return results, nil
}
