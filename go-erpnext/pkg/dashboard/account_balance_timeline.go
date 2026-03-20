package dashboard

import (
	"net/http"
	"time"

	"github.com/alex-baker-cdm/erpnext/go-erpnext/pkg/db"
	"github.com/alex-baker-cdm/erpnext/go-erpnext/pkg/frappe"
)

// glEntry holds a single aggregated GL Entry row used by the account balance timeline.
type glEntry struct {
	PostingDate time.Time
	Debit       float64
	Credit      float64
}

// AccountBalanceTimeline handles the account balance timeline dashboard chart.
// It reads GL entries for the specified account and company, builds a cumulative
// balance over monthly date points, and returns a ChartResult.
//
// Python source: erpnext/accounts/dashboard_chart_source/account_balance_timeline/account_balance_timeline.py
func AccountBalanceTimeline(dbConn *db.DB, w http.ResponseWriter, r *http.Request) {
	filters, err := ParseFilters(r)
	if err != nil {
		writeError(w, "invalid filters: "+err.Error(), http.StatusBadRequest)
		return
	}

	account, _ := filters["account"].(string)
	company, _ := filters["company"].(string)

	if company == "" && account == "" {
		writeError(w, "Company and account filters not set!", http.StatusBadRequest)
		return
	}
	if company == "" {
		writeError(w, "Company filter not set!", http.StatusBadRequest)
		return
	}
	if account == "" {
		writeError(w, "Account filter not set!", http.StatusBadRequest)
		return
	}

	// Parse from_date / to_date from query params; default to_date = today
	toDateStr := parseStringParam(r, "to_date", "")
	fromDateStr := parseStringParam(r, "from_date", "")

	var toDate time.Time
	if toDateStr != "" {
		toDate = frappe.Getdate(toDateStr)
	}
	if toDate.IsZero() {
		toDate = time.Now().UTC().Truncate(24 * time.Hour)
	}

	var fromDate time.Time
	if fromDateStr != "" {
		fromDate = frappe.Getdate(fromDateStr)
	}
	if fromDate.IsZero() {
		// Default: 1 year before to_date
		fromDate = frappe.AddMonths(toDate, -12)
	}

	// Generate monthly date points
	dates := getDatesFromTimegrain(fromDate, toDate)

	// Fetch GL entries for the account
	entries, err := getGLEntries(dbConn, company, account, toDate)
	if err != nil {
		writeError(w, "failed to query GL entries: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Look up account root_type to determine sign and cumulation
	rootType, err := getAccountRootType(dbConn, account)
	if err != nil {
		// Default to Asset if lookup fails
		rootType = "Asset"
	}

	// Build cumulative balance result
	result := buildResult(account, dates, entries, rootType)

	labels := make([]string, len(result))
	values := make([]float64, len(result))
	for i, r := range result {
		labels[i] = r.date.Format("2006-01-02")
		values[i] = r.balance
	}

	WriteJSON(w, ChartResult{
		Labels: labels,
		Datasets: []Dataset{
			{Name: account, Values: values},
		},
	})
}

// dateBalance holds a date and its associated balance for the timeline.
type dateBalance struct {
	date    time.Time
	balance float64
}

// buildResult computes the cumulative balance timeline from GL entries and dates.
// It mirrors the Python build_result function, applying sign inversion for
// credit-type accounts and cumulation for balance sheet accounts.
func buildResult(account string, dates []time.Time, entries []glEntry, rootType string) []dateBalance {
	result := make([]dateBalance, len(dates))
	for i, d := range dates {
		result[i] = dateBalance{date: d, balance: 0.0}
	}

	// Accumulate balances in debit
	dateIndex := 0
	for _, entry := range entries {
		// Move pointer forward until entry date <= result date
		for dateIndex < len(result)-1 && entry.PostingDate.After(result[dateIndex].date) {
			dateIndex++
		}
		result[dateIndex].balance += entry.Debit - entry.Credit
	}

	// If account type is credit, switch balances
	if rootType != "Asset" && rootType != "Expense" {
		for i := range result {
			result[i].balance = -1 * result[i].balance
		}
	}

	// For balance sheet accounts, the totals are cumulative
	if rootType == "Asset" || rootType == "Liability" || rootType == "Equity" {
		for i := 1; i < len(result); i++ {
			result[i].balance = result[i].balance + result[i-1].balance
		}
	}

	return result
}

// getGLEntries fetches GL entries for the given account (and its descendants)
// before the specified toDate. Entries are aggregated by posting_date.
func getGLEntries(dbConn *db.DB, company, account string, toDate time.Time) ([]glEntry, error) {
	// Get descendant accounts using lft/rgt tree traversal
	accounts, err := getDescendantAccounts(dbConn, account)
	if err != nil {
		// Fall back to just the account itself
		accounts = []string{account}
	}

	if len(accounts) == 0 {
		return nil, nil
	}

	// Build IN clause placeholders
	placeholders := "?"
	args := make([]interface{}, 0, len(accounts)+1)
	args = append(args, accounts[0])
	for i := 1; i < len(accounts); i++ {
		placeholders += ", ?"
		args = append(args, accounts[i])
	}
	args = append(args, toDate.Format("2006-01-02"))

	query := "SELECT posting_date, SUM(debit) as debit, SUM(credit) as credit " +
		"FROM `tabGL Entry` " +
		"WHERE account IN (" + placeholders + ") " +
		"AND posting_date < ? " +
		"AND is_cancelled = 0 " +
		"AND voucher_type != 'Period Closing Voucher' " +
		"GROUP BY posting_date " +
		"ORDER BY posting_date ASC"

	rows, err := dbConn.RawQuery(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []glEntry
	for rows.Next() {
		var e glEntry
		if err := rows.Scan(&e.PostingDate, &e.Debit, &e.Credit); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// getDescendantAccounts returns the account and all its descendants using lft/rgt.
func getDescendantAccounts(dbConn *db.DB, account string) ([]string, error) {
	var lft, rgt int
	err := dbConn.RawQueryRow(
		"SELECT lft, rgt FROM `tabAccount` WHERE name = ?", account,
	).Scan(&lft, &rgt)
	if err != nil {
		return []string{account}, err
	}

	rows, err := dbConn.RawQuery(
		"SELECT name FROM `tabAccount` WHERE lft >= ? AND rgt <= ?", lft, rgt,
	)
	if err != nil {
		return []string{account}, err
	}
	defer rows.Close()

	var accounts []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return accounts, err
		}
		accounts = append(accounts, name)
	}
	if len(accounts) == 0 {
		accounts = append(accounts, account)
	}
	return accounts, rows.Err()
}

// getAccountRootType returns the root_type of an account (Asset, Liability, etc.).
func getAccountRootType(dbConn *db.DB, account string) (string, error) {
	var rootType string
	err := dbConn.RawQueryRow(
		"SELECT root_type FROM `tabAccount` WHERE name = ?", account,
	).Scan(&rootType)
	return rootType, err
}

// getDatesFromTimegrain generates monthly date points between fromDate and toDate.
// Each date is the last day of its month.
func getDatesFromTimegrain(fromDate, toDate time.Time) []time.Time {
	var dates []time.Time

	// Start with end of the from_date's month
	current := frappe.GetLastDay(fromDate)
	dates = append(dates, current)

	for current.Before(toDate) {
		// Add one month and get last day
		next := frappe.AddMonths(current, 1)
		current = frappe.GetLastDay(next)
		dates = append(dates, current)
	}

	return dates
}
