package reports

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/alex-baker-cdm/erpnext/go-erpnext/pkg/db"
	"github.com/alex-baker-cdm/erpnext/go-erpnext/pkg/frappe"
)

// TrendReport is a generic report implementation for all trend-style reports.
// It wraps the shared trend utilities (ValidateFilters, BasedWiseColumnsQuery,
// GetPeriodDateRanges, CalculateTotalRow) to produce period-wise aggregated data.
type TrendReport struct {
	DocType     string   // e.g. "Delivery Note", "Sales Invoice"
	ChartLabel  string   // e.g. "Total Delivered Amount", "" if no chart
	ChartType   string   // "bar" or "line"
	ChartColors []string // optional, e.g. ["#5e64ff"]
	DB          *db.DB
}

// Execute runs the trend report with the given filters.
func (r *TrendReport) Execute(filters map[string]interface{}) (*ReportResult, error) {
	tf := parseTrendFilters(filters)

	if err := ValidateFilters(tf); err != nil {
		return nil, err
	}

	// Get based_on column/SQL details
	details := BasedWiseColumnsQuery(tf.BasedOn, r.DocType)
	basedOnCols, _ := details["based_on_cols"].([]string)
	basedOnSelect, _ := details["based_on_select"].(string)
	basedOnGroupBy, _ := details["based_on_group_by"].(string)
	addlTables, _ := details["addl_tables"].(string)
	addlTablesRelCond, _ := details["addl_tables_relational_cond"].(string)

	// Determine posting date field
	postingDate := "t1.transaction_date"
	if r.DocType == "Sales Invoice" || r.DocType == "Purchase Invoice" ||
		r.DocType == "Purchase Receipt" || r.DocType == "Delivery Note" {
		postingDate = "t1.posting_date"
		if tf.PeriodBasedOn != "" &&
			(r.DocType == "Sales Invoice" || r.DocType == "Purchase Invoice") {
			postingDate = "t1." + tf.PeriodBasedOn
		}
	}

	// Fetch fiscal year dates
	yearStartDate, yearEndDate, err := r.getFiscalYearDates(tf.FiscalYear)
	if err != nil {
		return nil, fmt.Errorf("fetching fiscal year dates: %w", err)
	}

	// Build period-wise columns and SELECT fragments
	periodCols, periodSelect := r.periodWiseColumnsQuery(tf, yearStartDate, yearEndDate)

	// Build group_by columns
	groupByCols := groupWiseColumn(tf.GroupBy)

	// Assemble final columns list
	var columns []string
	if len(groupByCols) > 0 {
		columns = append(columns, basedOnCols...)
		columns = append(columns, groupByCols...)
		columns = append(columns, periodCols...)
	} else {
		columns = append(columns, basedOnCols...)
		columns = append(columns, periodCols...)
	}
	columns = append(columns, "Total(Qty):Float:120", "Total(Amt):Currency/currency:120")

	// Build additional conditions
	cond := ""
	if basedOnSelect == "t1.project," || basedOnSelect == "t2.project," {
		cond = " AND " + strings.TrimSuffix(basedOnSelect, ",") + " IS NOT NULL"
	}

	includeClosedOrders := false
	if v, ok := filters["include_closed_orders"]; ok {
		if b, ok := v.(bool); ok {
			includeClosedOrders = b
		}
	}
	if !includeClosedOrders {
		if r.DocType == "Sales Order" || r.DocType == "Purchase Order" {
			cond += " AND t1.status != 'Closed'"
		}
	}
	if r.DocType == "Quotation" && tf.GroupBy == "Customer" {
		cond += " AND t1.quotation_to = 'Customer'"
	}

	// Build query
	queryDetails := basedOnSelect + periodSelect
	queryDetails += "SUM(t2.stock_qty), SUM(t2.base_net_amount)"

	query := fmt.Sprintf(
		"SELECT %s FROM `tab%s` t1, `tab%s Item` t2 %s"+
			" WHERE t2.parent = t1.name AND t1.company = ? AND %s BETWEEN ? AND ?"+
			" AND t1.docstatus = 1 %s %s"+
			" GROUP BY %s",
		queryDetails,
		r.DocType, r.DocType,
		addlTables,
		postingDate,
		cond, addlTablesRelCond,
		basedOnGroupBy,
	)

	rows, err := r.DB.Query(query, tf.Company, yearStartDate, yearEndDate)
	if err != nil {
		return nil, fmt.Errorf("executing trend query: %w", err)
	}
	defer rows.Close()

	colTypes, err := rows.ColumnTypes()
	if err != nil {
		return nil, fmt.Errorf("getting column types: %w", err)
	}
	numCols := len(colTypes)

	var data [][]interface{}
	for rows.Next() {
		vals := make([]interface{}, numCols)
		ptrs := make([]interface{}, numCols)
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, fmt.Errorf("scanning row: %w", err)
		}
		row := make([]interface{}, numCols)
		for i, v := range vals {
			if b, ok := v.([]byte); ok {
				row[i] = string(b)
			} else {
				row[i] = v
			}
		}
		data = append(data, row)
	}

	// Calculate total row and append
	totalRow := CalculateTotalRow(data, columns)
	data = append(data, totalRow)

	// Parse columns to Column structs
	parsedCols := ParseColumnShorthands(columns)

	// Build chart data
	var chart interface{}
	if r.ChartLabel != "" {
		chart = r.buildChartData(data, columns, tf)
	}

	return &ReportResult{
		Columns: parsedCols,
		Result:  data,
		Chart:   chart,
	}, nil
}

// parseTrendFilters extracts TrendFilters from a generic filters map.
func parseTrendFilters(filters map[string]interface{}) TrendFilters {
	getString := func(key string) string {
		if v, ok := filters[key]; ok {
			if s, ok := v.(string); ok {
				return s
			}
		}
		return ""
	}
	return TrendFilters{
		FiscalYear:    getString("fiscal_year"),
		BasedOn:       getString("based_on"),
		Period:        getString("period"),
		Company:       getString("company"),
		GroupBy:       getString("group_by"),
		PeriodBasedOn: getString("period_based_on"),
	}
}

// getFiscalYearDates retrieves the start and end dates for a fiscal year from the DB.
func (r *TrendReport) getFiscalYearDates(fiscalYear string) (time.Time, time.Time, error) {
	rows, err := r.DB.Query(
		"SELECT `year_start_date`, `year_end_date` FROM `tabFiscal Year` WHERE `name` = ? LIMIT 1",
		fiscalYear,
	)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	defer rows.Close()

	if !rows.Next() {
		return time.Time{}, time.Time{}, fmt.Errorf("fiscal year %q not found", fiscalYear)
	}

	var startStr, endStr string
	if err := rows.Scan(&startStr, &endStr); err != nil {
		return time.Time{}, time.Time{}, err
	}

	startDate := frappe.Getdate(startStr)
	endDate := frappe.Getdate(endStr)
	return startDate, endDate, nil
}

// periodWiseColumnsQuery builds period-wise column definitions and SQL SELECT fragments.
func (r *TrendReport) periodWiseColumnsQuery(tf TrendFilters, yearStart, yearEnd time.Time) ([]string, string) {
	var pwc []string
	queryDetails := ""

	transDate := "transaction_date"
	if r.DocType == "Purchase Receipt" || r.DocType == "Delivery Note" ||
		r.DocType == "Purchase Invoice" || r.DocType == "Sales Invoice" {
		transDate = "posting_date"
		if tf.PeriodBasedOn != "" &&
			(r.DocType == "Purchase Invoice" || r.DocType == "Sales Invoice") {
			transDate = tf.PeriodBasedOn
		}
	}

	betDates := GetPeriodDateRanges(tf.Period, yearStart, yearEnd)

	if tf.Period != "Yearly" {
		for _, dt := range betDates {
			pwc = append(pwc, getPeriodWiseColumns(dt, tf.Period)...)
			queryDetails = getPeriodWiseQuery(dt, transDate, queryDetails)
		}
	} else {
		pwc = []string{
			tf.FiscalYear + " (Qty):Float:120",
			tf.FiscalYear + " (Amt):Currency/currency:120",
		}
		queryDetails = " SUM(t2.stock_qty), SUM(t2.base_net_amount),"
	}

	return pwc, queryDetails
}

// getPeriodWiseColumns returns column definition strings for a single period.
func getPeriodWiseColumns(dt [2]time.Time, period string) []string {
	if period == "Monthly" {
		mon := dt[0].Format("Jan")
		return []string{
			mon + " (Qty):Float:120",
			mon + " (Amt):Currency/currency:120",
		}
	}
	startMon := dt[0].Format("Jan")
	endMon := dt[1].Format("Jan")
	return []string{
		startMon + "-" + endMon + " (Qty):Float:120",
		startMon + "-" + endMon + " (Amt):Currency/currency:120",
	}
}

// getPeriodWiseQuery appends SUM(IF(...)) SQL fragments for a single period.
func getPeriodWiseQuery(dt [2]time.Time, transDate, queryDetails string) string {
	sd := dt[0].Format("2006-01-02")
	ed := dt[1].Format("2006-01-02")
	queryDetails += fmt.Sprintf(
		"SUM(IF(t1.%s BETWEEN '%s' AND '%s', t2.stock_qty, NULL)),"+
			"SUM(IF(t1.%s BETWEEN '%s' AND '%s', t2.base_net_amount, NULL)),",
		transDate, sd, ed,
		transDate, sd, ed,
	)
	return queryDetails
}

// groupWiseColumn returns group_by column definitions.
func groupWiseColumn(groupBy string) []string {
	if groupBy != "" {
		return []string{groupBy + ":Link/" + groupBy + ":120"}
	}
	return nil
}

// buildChartData builds chart data based on the report's chart configuration.
func (r *TrendReport) buildChartData(data [][]interface{}, columns []string, tf TrendFilters) interface{} {
	if r.DocType == "Delivery Note" || r.DocType == "Purchase Receipt" {
		return r.buildTopNChartData(data, columns)
	}
	if r.DocType == "Sales Order" || r.DocType == "Quotation" || r.DocType == "Purchase Order" {
		return r.buildPeriodicChartData(data, columns, tf)
	}
	return nil
}

// buildTopNChartData builds chart data for DN/PR trends: top 10 by total, last column as value.
func (r *TrendReport) buildTopNChartData(data [][]interface{}, columns []string) interface{} {
	// Filter out total row and sort by last column descending
	var filtered [][]interface{}
	for _, row := range data {
		if len(row) > 0 {
			if s, ok := row[0].(string); ok && s == "'Total'" {
				continue
			}
		}
		filtered = append(filtered, row)
	}

	sort.Slice(filtered, func(i, j int) bool {
		vi := toFloat64(filtered[i][len(filtered[i])-1])
		vj := toFloat64(filtered[j][len(filtered[j])-1])
		return vi > vj
	})

	// Take top 10
	if len(filtered) > 10 {
		filtered = filtered[:10]
	}

	var labels []string
	var values []float64
	for _, row := range filtered {
		if len(row) > 0 {
			labels = append(labels, fmt.Sprintf("%v", row[0]))
			values = append(values, toFloat64(row[len(row)-1]))
		}
	}

	chartData := map[string]interface{}{
		"data": map[string]interface{}{
			"labels":   labels,
			"datasets": []map[string]interface{}{{"name": r.ChartLabel, "values": values}},
		},
		"type": r.ChartType,
	}
	if len(r.ChartColors) > 0 {
		chartData["colors"] = r.ChartColors
	}
	return chartData
}

// buildPeriodicChartData builds chart data for SO/Quotation/PO trends:
// sum periodic amounts across all rows.
func (r *TrendReport) buildPeriodicChartData(data [][]interface{}, columns []string, tf TrendFilters) interface{} {
	// Determine start offset based on based_on type
	start := 1
	switch tf.BasedOn {
	case "Customer":
		start = 3
	case "Item":
		start = 2
	}
	if tf.GroupBy != "" {
		start++
	}

	// Extract periodic amount columns (every other column starting from start+1, the Amt columns)
	// columns[start:-2] then pick every other one (Amt, not Qty)
	end := len(columns) - 2
	if end <= start {
		return nil
	}

	var amtIndices []int
	var amtLabels []string
	for i := start + 1; i < end; i += 2 {
		amtIndices = append(amtIndices, i)
		label := columns[i]
		// Remove " (Amt)" suffix and type info
		if colonIdx := strings.Index(label, ":"); colonIdx > 0 {
			label = label[:colonIdx]
		}
		label = strings.TrimSuffix(label, " (Amt)")
		amtLabels = append(amtLabels, label)
	}

	// Sum values across all data rows
	sums := make([]float64, len(amtIndices))
	for _, row := range data {
		if len(row) > 0 {
			if s, ok := row[0].(string); ok && s == "'Total'" {
				continue
			}
		}
		for j, idx := range amtIndices {
			if idx < len(row) {
				sums[j] += toFloat64(row[idx])
			}
		}
	}

	// Replace {period} in chart label
	chartLabel := strings.ReplaceAll(r.ChartLabel, "{period}", tf.Period)

	chartData := map[string]interface{}{
		"data": map[string]interface{}{
			"labels":   amtLabels,
			"datasets": []map[string]interface{}{{"name": chartLabel, "values": sums}},
		},
		"type": r.ChartType,
	}
	if len(r.ChartColors) > 0 {
		chartData["colors"] = r.ChartColors
	}
	return chartData
}
