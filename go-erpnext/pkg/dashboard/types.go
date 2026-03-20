// Package dashboard provides Go HTTP handlers for ERPNext dashboard chart sources
// and dashboard page data endpoints. Each handler mirrors its Python counterpart
// and returns JSON in the format expected by the Frappe frontend.
package dashboard

import (
	"encoding/json"
	"net/http"

	"github.com/alex-baker-cdm/erpnext/go-erpnext/pkg/db"
)

// ChartResult is the standard response format for dashboard chart sources.
// Frappe frontend expects {labels: [...], datasets: [{name, values}]}.
type ChartResult struct {
	Labels   []string  `json:"labels"`
	Datasets []Dataset `json:"datasets"`
}

// Dataset represents a single data series within a ChartResult.
type Dataset struct {
	Name   string    `json:"name"`
	Values []float64 `json:"values"`
}

// HandlerFunc is the signature for dashboard endpoint handlers that need DB access.
type HandlerFunc func(dbConn *db.DB, w http.ResponseWriter, r *http.Request)

// MakeHandler wraps a HandlerFunc with the shared DB connection to produce
// a standard http.HandlerFunc suitable for use with http.ServeMux.
func MakeHandler(dbConn *db.DB, fn HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		fn(dbConn, w, r)
	}
}

// WriteJSON serialises data into the Frappe standard response envelope
// {"message": data} and writes it to the ResponseWriter.
func WriteJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	envelope := map[string]interface{}{"message": data}
	if err := json.NewEncoder(w).Encode(envelope); err != nil {
		http.Error(w, `{"error":"failed to encode response"}`, http.StatusInternalServerError)
	}
}

// writeError writes a JSON error response with the given status code.
func writeError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

// ParseFilters extracts the "filters" query parameter from the request and
// returns it as a map. The parameter is expected to be a JSON-encoded object.
// If the parameter is missing or empty, an empty map is returned.
func ParseFilters(r *http.Request) (map[string]interface{}, error) {
	raw := r.URL.Query().Get("filters")
	if raw == "" {
		// Also try form body for POST requests
		if r.Method == http.MethodPost {
			if err := r.ParseForm(); err == nil {
				raw = r.FormValue("filters")
			}
		}
	}
	if raw == "" {
		return map[string]interface{}{}, nil
	}

	var filters map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &filters); err != nil {
		return nil, err
	}
	return filters, nil
}

// parseStringParam returns the named query/form parameter as a string, or
// the default value if it is missing or empty.
func parseStringParam(r *http.Request, name, defaultVal string) string {
	val := r.URL.Query().Get(name)
	if val == "" && r.Method == http.MethodPost {
		_ = r.ParseForm()
		val = r.FormValue(name)
	}
	if val == "" {
		return defaultVal
	}
	return val
}

// parseIntParam returns the named query/form parameter as an int, or the
// default value if it is missing, empty, or not a valid integer.
func parseIntParam(r *http.Request, name string, defaultVal int) int {
	raw := parseStringParam(r, name, "")
	if raw == "" {
		return defaultVal
	}
	var v int
	if _, err := json.Number(raw).Int64(); err == nil {
		n, _ := json.Number(raw).Int64()
		v = int(n)
	} else {
		v = defaultVal
	}
	return v
}

// validSortColumns lists the column names that are allowed in ORDER BY clauses
// for the item dashboard and warehouse capacity dashboard endpoints.
var validSortColumns = map[string]bool{
	"actual_qty":                      true,
	"projected_qty":                   true,
	"reserved_qty":                    true,
	"reserved_qty_for_production":     true,
	"reserved_qty_for_sub_contract":   true,
	"valuation_rate":                  true,
	"stock_capacity":                  true,
	"item_code":                       true,
	"warehouse":                       true,
	"company":                         true,
	"percent_occupied":                true,
}

// validSortOrders lists the allowed sort order keywords.
var validSortOrders = map[string]bool{
	"asc":  true,
	"desc": true,
}

// validateSortParams checks that sort_by and sort_order are safe for SQL injection.
func validateSortParams(sortBy, sortOrder string) bool {
	return validSortColumns[sortBy] && validSortOrders[sortOrder]
}
