package reports

import (
	"fmt"
	"time"

	"github.com/alex-baker-cdm/erpnext/go-erpnext/pkg/frappe"
)

// TrendConditions holds the column definitions and SQL fragments built by GetColumns.
type TrendConditions struct {
	BasedOnSelect        string   `json:"based_on_select"`
	PeriodWiseSelect     string   `json:"period_wise_select"`
	Columns              []string `json:"columns"`
	GroupBy              string   `json:"group_by"`
	Grbc                 []string `json:"grbc"`
	Trans                string   `json:"trans"`
	AddlTables           string   `json:"addl_tables"`
	AddlTablesRelCond    string   `json:"addl_tables_relational_cond"`
}

// TrendFilters represents the filter parameters for trend reports.
type TrendFilters struct {
	FiscalYear    string `json:"fiscal_year"`
	BasedOn       string `json:"based_on"`
	Period        string `json:"period"`
	Company       string `json:"company"`
	GroupBy       string `json:"group_by"`
	PeriodBasedOn string `json:"period_based_on"`
}

// ValidateFilters checks that all required trend report filters are present.
// Returns an error if any mandatory filter is missing or if based_on == group_by.
func ValidateFilters(filters TrendFilters) error {
	if filters.FiscalYear == "" {
		return fmt.Errorf("Fiscal Year is mandatory")
	}
	if filters.BasedOn == "" {
		return fmt.Errorf("Based On is mandatory")
	}
	if filters.Period == "" {
		return fmt.Errorf("Period is mandatory")
	}
	if filters.Company == "" {
		return fmt.Errorf("Company is mandatory")
	}
	if filters.BasedOn == filters.GroupBy && filters.GroupBy != "" {
		return fmt.Errorf("'Based On' and 'Group By' can not be same")
	}
	return nil
}

// GetPeriodDateRanges generates period date ranges for the given periodicity and fiscal year dates.
// Returns a slice of [startDate, endDate] pairs for each period.
func GetPeriodDateRanges(period string, yearStartDate, yearEndDate time.Time) [][2]time.Time {
	incrementMap := map[string]int{
		"Monthly":     1,
		"Quarterly":   3,
		"Half-Yearly": 6,
		"Yearly":      12,
	}

	increment, ok := incrementMap[period]
	if !ok {
		increment = 12
	}

	var ranges [][2]time.Time
	currentStart := yearStartDate

	for i := 1; i <= 12; i += increment {
		periodEnd := frappe.AddDays(frappe.AddMonths(currentStart, increment), -1)

		if periodEnd.After(yearEndDate) {
			periodEnd = yearEndDate
		}

		ranges = append(ranges, [2]time.Time{currentStart, periodEnd})

		currentStart = frappe.AddDays(periodEnd, 1)

		if periodEnd.Equal(yearEndDate) {
			break
		}
	}

	return ranges
}

// BasedWiseColumnsQuery returns SQL fragments and column definitions per based_on type.
// This mirrors the Python based_wise_columns_query function.
func BasedWiseColumnsQuery(basedOn, trans string) map[string]interface{} {
	details := map[string]interface{}{
		"based_on_cols":              []string{},
		"based_on_select":           "",
		"based_on_group_by":         "",
		"addl_tables":               "",
		"addl_tables_relational_cond": "",
	}

	switch basedOn {
	case "Item":
		details["based_on_cols"] = []string{"Item:Link/Item:120", "Item Name:Data:120"}
		details["based_on_select"] = "t2.item_code, t2.item_name,"
		details["based_on_group_by"] = "t2.item_code"
		details["addl_tables"] = ""

	case "Item Group":
		details["based_on_cols"] = []string{"Item Group:Link/Item Group:120"}
		details["based_on_select"] = "t2.item_group,"
		details["based_on_group_by"] = "t2.item_group"
		details["addl_tables"] = ""

	case "Customer":
		if trans == "Quotation" {
			details["based_on_cols"] = []string{
				"Party:Link/Customer:120",
				"Party Name:Data:120",
				"Territory:Link/Territory:120",
			}
			details["based_on_select"] = "t1.party_name, t1.customer_name, t1.territory,"
		} else {
			details["based_on_cols"] = []string{
				"Customer:Link/Customer:120",
				"Customer Name:Data:120",
				"Territory:Link/Territory:120",
			}
			details["based_on_select"] = "t1.customer, t1.customer_name, t1.territory,"
		}
		details["based_on_group_by"] = "t1.customer"
		if trans == "Quotation" {
			details["based_on_group_by"] = "t1.party_name"
		}
		details["addl_tables"] = ""

	case "Customer Group":
		details["based_on_cols"] = []string{"Customer Group:Link/Customer Group"}
		details["based_on_select"] = "t1.customer_group,"
		details["based_on_group_by"] = "t1.customer_group"
		details["addl_tables"] = ""

	case "Supplier":
		details["based_on_cols"] = []string{
			"Supplier:Link/Supplier:120",
			"Supplier Name:Data:120",
			"Supplier Group:Link/Supplier Group:140",
		}
		details["based_on_select"] = "t1.supplier, t1.supplier_name, t3.supplier_group,"
		details["based_on_group_by"] = "t1.supplier"
		details["addl_tables"] = ",`tabSupplier` t3"
		details["addl_tables_relational_cond"] = " and t1.supplier = t3.name"

	case "Supplier Group":
		details["based_on_cols"] = []string{"Supplier Group:Link/Supplier Group:140"}
		details["based_on_select"] = "t3.supplier_group,"
		details["based_on_group_by"] = "t3.supplier_group"
		details["addl_tables"] = ",`tabSupplier` t3"
		details["addl_tables_relational_cond"] = " and t1.supplier = t3.name"

	case "Territory":
		details["based_on_cols"] = []string{"Territory:Link/Territory:120"}
		details["based_on_select"] = "t1.territory,"
		details["based_on_group_by"] = "t1.territory"
		details["addl_tables"] = ""

	case "Project":
		if trans == "Sales Invoice" || trans == "Delivery Note" || trans == "Sales Order" {
			details["based_on_cols"] = []string{"Project:Link/Project:120"}
			details["based_on_select"] = "t1.project,"
			details["based_on_group_by"] = "t1.project"
			details["addl_tables"] = ""
		} else if trans == "Purchase Order" || trans == "Purchase Invoice" || trans == "Purchase Receipt" {
			details["based_on_cols"] = []string{"Project:Link/Project:120"}
			details["based_on_select"] = "t2.project,"
			details["based_on_group_by"] = "t2.project"
			details["addl_tables"] = ""
		}
	}

	// Add currency column (common to all)
	if cols, ok := details["based_on_cols"].([]string); ok {
		details["based_on_cols"] = append(cols, "Currency:Link/Currency:120")
	}

	if sel, ok := details["based_on_select"].(string); ok {
		details["based_on_select"] = sel + "t4.default_currency as currency,"
	}

	if tables, ok := details["addl_tables"].(string); ok {
		details["addl_tables"] = tables + ", `tabCompany` t4"
	}

	relCond := ""
	if rc, ok := details["addl_tables_relational_cond"].(string); ok {
		relCond = rc
	}
	details["addl_tables_relational_cond"] = relCond + " and t1.company = t4.name"

	return details
}

// CalculateTotalRow computes a total row by summing all numeric columns in the data.
// Float and Currency columns are identified by their column definition strings.
func CalculateTotalRow(data [][]interface{}, columns []string) []interface{} {
	// Identify numeric column indices
	numericCols := make(map[int]bool)
	for i, col := range columns {
		if containsAny(col, "Float", "Currency/currency") {
			numericCols[i] = true
		}
	}

	// Initialize totals
	totals := make(map[int]float64)
	for i := range numericCols {
		totals[i] = 0
	}

	// Sum values
	for _, row := range data {
		for i := range numericCols {
			if i < len(row) && row[i] != nil {
				totals[i] += toFloat64(row[i])
			}
		}
	}

	// Build total row
	totalRow := make([]interface{}, len(columns))
	totalRow[0] = "'Total'"
	for i := 1; i < len(columns); i++ {
		if numericCols[i] {
			totalRow[i] = totals[i]
		} else {
			totalRow[i] = nil
		}
	}

	return totalRow
}

// containsAny checks if s contains any of the given substrings.
func containsAny(s string, substrs ...string) bool {
	for _, sub := range substrs {
		if len(s) >= len(sub) {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
		}
	}
	return false
}

// toFloat64 converts an interface{} to float64, returning 0 for non-numeric types.
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
	default:
		return 0
	}
}
