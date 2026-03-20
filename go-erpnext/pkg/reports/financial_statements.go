package reports

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/alex-baker-cdm/erpnext/go-erpnext/pkg/db"
	"github.com/alex-baker-cdm/erpnext/go-erpnext/pkg/frappe"
)

// Account represents a GL account with its tree position and accumulated values.
type Account struct {
	Name           string             `json:"name"`
	AccountNumber  string             `json:"account_number"`
	ParentAccount  string             `json:"parent_account"`
	Lft            int                `json:"lft"`
	Rgt            int                `json:"rgt"`
	RootType       string             `json:"root_type"`
	ReportType     string             `json:"report_type"`
	AccountName    string             `json:"account_name"`
	IncludeInGross bool               `json:"include_in_gross"`
	AccountType    string             `json:"account_type"`
	IsGroup        bool               `json:"is_group"`
	Indent         float64            `json:"indent"`
	Values         map[string]float64 `json:"values"`
	OpeningBalance float64            `json:"opening_balance"`
}

// GLEntry represents a single General Ledger entry.
type GLEntry struct {
	Account                 string    `json:"account"`
	PostingDate             time.Time `json:"posting_date"`
	Debit                   float64   `json:"debit"`
	Credit                  float64   `json:"credit"`
	DebitInAccountCurrency  float64   `json:"debit_in_account_currency"`
	CreditInAccountCurrency float64   `json:"credit_in_account_currency"`
	AccountCurrency         string    `json:"account_currency"`
	FiscalYear              string    `json:"fiscal_year"`
}

// Period represents a reporting period with date boundaries and labels.
type Period struct {
	FromDate                    time.Time `json:"from_date"`
	ToDate                      time.Time `json:"to_date"`
	Key                         string    `json:"key"`
	Label                       string    `json:"label"`
	YearStartDate               time.Time `json:"year_start_date"`
	YearEndDate                 time.Time `json:"year_end_date"`
	ToDateFiscalYear            string    `json:"to_date_fiscal_year"`
	FromDateFiscalYearStartDate time.Time `json:"from_date_fiscal_year_start_date"`
}

// ReportRow represents a single row in a financial report output.
type ReportRow struct {
	Account        string             `json:"account"`
	ParentAccount  string             `json:"parent_account"`
	Indent         float64            `json:"indent"`
	YearStartDate  string             `json:"year_start_date"`
	YearEndDate    string             `json:"year_end_date"`
	Currency       string             `json:"currency"`
	IncludeInGross bool               `json:"include_in_gross"`
	AccountType    string             `json:"account_type"`
	IsGroup        bool               `json:"is_group"`
	OpeningBalance float64            `json:"opening_balance"`
	AccountName    string             `json:"account_name"`
	HasValue       bool               `json:"has_value"`
	Total          float64            `json:"total"`
	Values         map[string]float64 `json:"values"`
}

// FilterAccounts builds a parent-child tree from a flat list of accounts,
// sorts children by lft/rgt order, and assigns indent levels.
// Returns the filtered (tree-ordered) accounts, a map of accounts by name,
// and a parent-children map.
func FilterAccounts(accounts []Account) ([]Account, map[string]*Account, map[string][]*Account) {
	accountsByName := make(map[string]*Account)
	parentChildMap := make(map[string][]*Account)

	for i := range accounts {
		accounts[i].Values = make(map[string]float64)
		accountsByName[accounts[i].Name] = &accounts[i]
		parent := accounts[i].ParentAccount
		parentChildMap[parent] = append(parentChildMap[parent], &accounts[i])
	}

	var filtered []Account
	var addToList func(parent string, level int)
	addToList = func(parent string, level int) {
		children := parentChildMap[parent]
		sortAccounts(children, parent == "")
		for _, child := range children {
			child.Indent = float64(level)
			filtered = append(filtered, *child)
			addToList(child.Name, level+1)
		}
	}

	addToList("", 0)

	return filtered, accountsByName, parentChildMap
}

// sortAccounts sorts accounts using Frappe's convention: numbered accounts by name,
// root accounts by report_type/root_type ordering, others alphabetically.
func sortAccounts(accounts []*Account, isRoot bool) {
	sort.SliceStable(accounts, func(i, j int) bool {
		a, b := accounts[i], accounts[j]

		// If chart of accounts is numbered, sort by number
		if len(a.Name) > 0 && a.Name[0] >= '0' && a.Name[0] <= '9' {
			return a.Name < b.Name
		}

		if isRoot {
			if a.ReportType != b.ReportType && a.ReportType == "Balance Sheet" {
				return true
			}
			if a.RootType != b.RootType && a.RootType == "Asset" {
				return true
			}
			if a.RootType == "Liability" && b.RootType == "Equity" {
				return true
			}
			if a.RootType == "Income" && b.RootType == "Expense" {
				return true
			}
			return false
		}

		return a.Name < b.Name
	})
}

// FilterOutZeroValueRows removes rows with zero values while keeping parents
// of non-zero children. If showZeroValues is true, all rows are kept.
func FilterOutZeroValueRows(data []ReportRow, parentChildMap map[string][]*Account, showZeroValues bool) []ReportRow {
	if showZeroValues {
		return data
	}

	accountsToShow := make(map[string]bool)

	var getAllParents func(account string)
	getAllParents = func(account string) {
		for parent, children := range parentChildMap {
			for _, child := range children {
				if child.Name == account && parent != "" {
					accountsToShow[parent] = true
					getAllParents(parent)
				}
			}
		}
	}

	for _, d := range data {
		if d.HasValue {
			accountsToShow[d.Account] = true
			getAllParents(d.Account)
		}
	}

	var result []ReportRow
	for _, d := range data {
		if accountsToShow[d.Account] {
			result = append(result, d)
		}
	}
	return result
}

// SetGLEntriesByAccount queries GL entries grouped by account for the given parameters.
// Returns a map of account name to GL entries for that account.
func SetGLEntriesByAccount(
	database *db.DB,
	company string,
	fromDate string,
	toDate string,
	rootLft int,
	rootRgt int,
	rootType string,
	ignoreClosingEntries bool,
) (map[string][]GLEntry, error) {
	result := make(map[string][]GLEntry)

	// Build query with parameterized values
	query := `SELECT gl.account, gl.posting_date, gl.debit, gl.credit,
		gl.debit_in_account_currency, gl.credit_in_account_currency,
		gl.account_currency, gl.fiscal_year
		FROM ` + "`tabGL Entry`" + ` gl
		WHERE gl.company = ? AND gl.is_cancelled = 0 AND gl.posting_date <= ?`

	args := []interface{}{company, toDate}

	if fromDate != "" {
		query += " AND gl.posting_date >= ?"
		args = append(args, fromDate)
	}

	if rootLft > 0 && rootRgt > 0 {
		query += ` AND EXISTS (
			SELECT 1 FROM ` + "`tabAccount`" + ` acc
			WHERE acc.name = gl.account AND acc.lft >= ? AND acc.rgt <= ?
		)`
		args = append(args, rootLft, rootRgt)
	}

	if rootType != "" {
		query += ` AND EXISTS (
			SELECT 1 FROM ` + "`tabAccount`" + ` acc
			WHERE acc.name = gl.account AND acc.root_type = ?
		)`
		args = append(args, rootType)
	}

	if ignoreClosingEntries {
		query += " AND gl.voucher_type != 'Period Closing Voucher'"
	}

	query += " ORDER BY gl.posting_date, gl.account"

	rows, err := database.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying GL entries: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var entry GLEntry
		var postingDateStr string
		if err := rows.Scan(
			&entry.Account, &postingDateStr,
			&entry.Debit, &entry.Credit,
			&entry.DebitInAccountCurrency, &entry.CreditInAccountCurrency,
			&entry.AccountCurrency, &entry.FiscalYear,
		); err != nil {
			return nil, fmt.Errorf("scanning GL entry: %w", err)
		}
		entry.PostingDate = frappe.Getdate(postingDateStr)
		result[entry.Account] = append(result[entry.Account], entry)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating GL entries: %w", err)
	}

	return result, nil
}

// GetCostCentersWithChildren expands a list of cost center names to include
// all their children using the lft/rgt nested set model.
func GetCostCentersWithChildren(database *db.DB, costCenters []string) ([]string, error) {
	allCostCenters := make(map[string]bool)

	for _, cc := range costCenters {
		cc = strings.TrimSpace(cc)
		if cc == "" {
			continue
		}

		// Get lft, rgt for this cost center
		rows, err := database.Query(
			"SELECT `lft`, `rgt` FROM `tabCost Center` WHERE `name` = ?", cc,
		)
		if err != nil {
			return nil, fmt.Errorf("querying cost center %s: %w", cc, err)
		}

		var lft, rgt int
		found := false
		if rows.Next() {
			if err := rows.Scan(&lft, &rgt); err != nil {
				rows.Close()
				return nil, fmt.Errorf("scanning cost center %s: %w", cc, err)
			}
			found = true
		}
		rows.Close()

		if !found {
			return nil, fmt.Errorf("cost center %s does not exist", cc)
		}

		// Get all children
		childRows, err := database.Query(
			"SELECT `name` FROM `tabCost Center` WHERE `lft` >= ? AND `rgt` <= ?",
			lft, rgt,
		)
		if err != nil {
			return nil, fmt.Errorf("querying cost center children: %w", err)
		}

		for childRows.Next() {
			var name string
			if err := childRows.Scan(&name); err != nil {
				childRows.Close()
				return nil, fmt.Errorf("scanning cost center child: %w", err)
			}
			allCostCenters[name] = true
		}
		childRows.Close()
	}

	result := make([]string, 0, len(allCostCenters))
	for cc := range allCostCenters {
		result = append(result, cc)
	}
	sort.Strings(result)
	return result, nil
}

// GetPeriodList generates a list of period date ranges based on the given parameters.
// Periodicity can be "Yearly", "Half-Yearly", "Quarterly", or "Monthly".
func GetPeriodList(
	yearStartDate time.Time,
	yearEndDate time.Time,
	periodicity string,
	accumulatedValues bool,
) []Period {
	monthsToAdd := map[string]int{
		"Yearly":      12,
		"Half-Yearly": 6,
		"Quarterly":   3,
		"Monthly":     1,
	}

	increment, ok := monthsToAdd[periodicity]
	if !ok {
		increment = 12
	}

	months := getMonths(yearStartDate, yearEndDate)
	numPeriods := int(math.Ceil(float64(months) / float64(increment)))

	var periodList []Period
	startDate := yearStartDate

	for i := 0; i < numPeriods; i++ {
		period := Period{
			FromDate:      startDate,
			YearStartDate: yearStartDate,
			YearEndDate:   yearEndDate,
		}

		toDate := frappe.AddMonths(startDate, increment)
		startDate = toDate

		// Subtract one day
		toDate = frappe.AddDays(toDate, -1)

		if !toDate.After(yearEndDate) {
			period.ToDate = toDate
		} else {
			period.ToDate = yearEndDate
		}

		periodList = append(periodList, period)

		if period.ToDate.Equal(yearEndDate) {
			break
		}
	}

	// Assign keys and labels
	for i := range periodList {
		key := strings.ToLower(periodList[i].ToDate.Format("Jan_2006"))
		key = strings.ReplaceAll(key, " ", "_")
		key = strings.ReplaceAll(key, "-", "_")
		periodList[i].Key = key

		if periodicity == "Monthly" && !accumulatedValues {
			periodList[i].Label = periodList[i].ToDate.Format("Jan 2006")
		} else {
			periodList[i].Label = getPeriodLabel(
				periodicity,
				periodList[i].FromDate,
				periodList[i].ToDate,
			)
		}
	}

	return periodList
}

// getMonths returns the number of months between two dates (inclusive).
func getMonths(startDate, endDate time.Time) int {
	diff := (12*endDate.Year() + int(endDate.Month())) -
		(12*startDate.Year() + int(startDate.Month()))
	return diff + 1
}

// getPeriodLabel generates a label for a reporting period.
func getPeriodLabel(periodicity string, fromDate, toDate time.Time) string {
	if periodicity == "Yearly" {
		fromYear := fromDate.Format("2006")
		toYear := toDate.Format("2006")
		if fromYear == toYear {
			return fromYear
		}
		return fromYear + "-" + toYear
	}
	return fromDate.Format("Jan 06") + "-" + toDate.Format("Jan 06")
}

// CalculateValues assigns GL entry values to accounts by period.
// For each GL entry, it checks if the posting date falls within each period
// and accumulates debit-credit values.
func CalculateValues(
	accountsByName map[string]*Account,
	glEntriesByAccount map[string][]GLEntry,
	periodList []Period,
	accumulatedValues bool,
	ignoreAccumulatedValuesForFY bool,
) {
	for _, entries := range glEntriesByAccount {
		for _, entry := range entries {
			acc, ok := accountsByName[entry.Account]
			if !ok {
				continue
			}

			for _, period := range periodList {
				if !entry.PostingDate.After(period.ToDate) {
					if (accumulatedValues || !entry.PostingDate.Before(period.FromDate)) &&
						(!ignoreAccumulatedValuesForFY ||
							entry.FiscalYear == period.ToDateFiscalYear) {
						acc.Values[period.Key] += frappe.Flt(entry.Debit, -1) - frappe.Flt(entry.Credit, -1)
					}
				}
			}

			if len(periodList) > 0 && entry.PostingDate.Before(periodList[0].YearStartDate) {
				acc.OpeningBalance += frappe.Flt(entry.Debit, -1) - frappe.Flt(entry.Credit, -1)
			}
		}
	}
}

// AccumulateValuesIntoParents rolls up child account values into their parent accounts.
func AccumulateValuesIntoParents(accounts []Account, accountsByName map[string]*Account, periodList []Period) {
	for i := len(accounts) - 1; i >= 0; i-- {
		d := accounts[i]
		if d.ParentAccount == "" {
			continue
		}
		parent, ok := accountsByName[d.ParentAccount]
		if !ok {
			continue
		}
		for _, period := range periodList {
			parent.Values[period.Key] += d.Values[period.Key]
		}
		parent.OpeningBalance += d.OpeningBalance
	}
}

// PrepareData formats account data into final report output rows.
func PrepareData(
	accounts []Account,
	balanceMustBe string,
	periodList []Period,
	companyCurrency string,
	accumulatedValues bool,
) []ReportRow {
	var data []ReportRow

	yearStartDate := ""
	yearEndDate := ""
	if len(periodList) > 0 {
		yearStartDate = periodList[0].YearStartDate.Format("2006-01-02")
		yearEndDate = periodList[len(periodList)-1].YearEndDate.Format("2006-01-02")
	}

	for i := range accounts {
		d := &accounts[i]
		hasValue := false
		total := 0.0

		row := ReportRow{
			Account:        d.Name,
			ParentAccount:  d.ParentAccount,
			Indent:         d.Indent,
			YearStartDate:  yearStartDate,
			YearEndDate:    yearEndDate,
			Currency:       companyCurrency,
			IncludeInGross: d.IncludeInGross,
			AccountType:    d.AccountType,
			IsGroup:        d.IsGroup,
			OpeningBalance:  d.OpeningBalance,
			AccountName:    d.AccountName,
			Values:         make(map[string]float64),
		}

		if balanceMustBe == "Debit" {
			row.OpeningBalance = d.OpeningBalance
		} else {
			row.OpeningBalance = -d.OpeningBalance
		}

		for _, period := range periodList {
			val := d.Values[period.Key]
			if val != 0 && balanceMustBe == "Credit" {
				val *= -1
			}

			row.Values[period.Key] = frappe.Flt(val, 3)

			if math.Abs(row.Values[period.Key]) >= 0.005 {
				hasValue = true
				total += frappe.Flt(row.Values[period.Key], -1)
			}
		}

		if accumulatedValues && len(periodList) > 0 {
			row.HasValue = hasValue
			row.Total = frappe.Flt(d.Values[periodList[len(periodList)-1].Key], 3)
			if balanceMustBe == "Credit" && row.Total != 0 {
				row.Total *= -1
			}
		} else {
			row.HasValue = hasValue
			row.Total = total
		}

		data = append(data, row)
	}

	return data
}

// AddTotalRow appends a total row that sums all root-level account values.
func AddTotalRow(
	out []ReportRow,
	rootType string,
	balanceMustBe string,
	periodList []Period,
	companyCurrency string,
) []ReportRow {
	totalRow := ReportRow{
		AccountName: fmt.Sprintf("'Total %s (%s)'", rootType, balanceMustBe),
		Account:     fmt.Sprintf("'Total %s (%s)'", rootType, balanceMustBe),
		Currency:    companyCurrency,
		Values:      make(map[string]float64),
	}

	hasTotalKey := false
	for _, row := range out {
		if row.ParentAccount == "" {
			for _, period := range periodList {
				totalRow.Values[period.Key] += row.Values[period.Key]
			}
			totalRow.Total += frappe.Flt(row.Total, -1)
			totalRow.OpeningBalance += row.OpeningBalance
			hasTotalKey = true
		}
	}

	if hasTotalKey {
		out = append(out, totalRow)
		// Blank row after total
		out = append(out, ReportRow{Values: make(map[string]float64)})
	}

	return out
}
