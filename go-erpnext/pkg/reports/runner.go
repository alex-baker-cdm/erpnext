package reports

import (
	"encoding/json"
	"net/http"
	"sync"

	"github.com/alex-baker-cdm/erpnext/go-erpnext/pkg/db"
)

// Report is the interface that all Go report implementations must satisfy.
type Report interface {
	Execute(filters map[string]interface{}) (*ReportResult, error)
}

// ReportResult holds the output of a report execution.
type ReportResult struct {
	Columns []Column        `json:"columns"`
	Result  [][]interface{} `json:"result"`
	Chart   interface{}     `json:"chart,omitempty"`
}

// registry holds all registered Go report implementations, keyed by report name.
var registry = map[string]Report{}

// registryMu protects concurrent access to the registry.
var registryMu sync.RWMutex

// Register adds a report implementation to the global registry.
// It should be called during init() or startup for each Go-ported report.
func Register(name string, r Report) {
	registryMu.Lock()
	defer registryMu.Unlock()
	registry[name] = r
}

// GetReport looks up a report by name in the registry.
// Returns nil if not found.
func GetReport(name string) Report {
	registryMu.RLock()
	defer registryMu.RUnlock()
	return registry[name]
}

// RunReportHandler returns an http.HandlerFunc that serves report execution requests.
// It accepts GET/POST with parameters: report_name (string), filters (JSON string).
// The handler looks up the report by name in the registry, executes it, and returns
// JSON in the format Frappe's frontend expects.
func RunReportHandler(database *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodPost {
			writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}

		// Parse parameters from query string or form body
		if err := r.ParseForm(); err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid form data")
			return
		}

		reportName := r.FormValue("report_name")
		if reportName == "" {
			writeJSONError(w, http.StatusBadRequest, "report_name is required")
			return
		}

		// Look up the report in registry
		report := GetReport(reportName)
		if report == nil {
			writeJSONError(w, http.StatusNotFound, "report not found: "+reportName)
			return
		}

		// Parse filters JSON
		filters := make(map[string]interface{})
		filtersStr := r.FormValue("filters")
		if filtersStr != "" {
			if err := json.Unmarshal([]byte(filtersStr), &filters); err != nil {
				writeJSONError(w, http.StatusBadRequest, "invalid filters JSON: "+err.Error())
				return
			}
		}

		// Execute the report
		result, err := report.Execute(filters)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "report execution failed: "+err.Error())
			return
		}

		// Return in Frappe's expected format
		response := map[string]interface{}{
			"message": result,
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}
}

// writeJSONError writes a JSON error response.
func writeJSONError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}
