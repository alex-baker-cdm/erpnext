package reports

import (
	"testing"
	"time"
)

func TestFilterAccounts_TreeBuilding(t *testing.T) {
	accounts := []Account{
		{Name: "Assets", ParentAccount: "", Lft: 1, Rgt: 10, RootType: "Asset", ReportType: "Balance Sheet", IsGroup: true},
		{Name: "Bank", ParentAccount: "Assets", Lft: 2, Rgt: 5, RootType: "Asset", ReportType: "Balance Sheet"},
		{Name: "Cash", ParentAccount: "Assets", Lft: 6, Rgt: 9, RootType: "Asset", ReportType: "Balance Sheet"},
		{Name: "Petty Cash", ParentAccount: "Cash", Lft: 7, Rgt: 8, RootType: "Asset", ReportType: "Balance Sheet"},
	}

	filtered, byName, parentChildMap := FilterAccounts(accounts)

	// Verify all accounts are present
	if len(filtered) != 4 {
		t.Fatalf("expected 4 filtered accounts, got %d", len(filtered))
	}

	// Verify byName map
	if _, ok := byName["Assets"]; !ok {
		t.Error("byName should contain 'Assets'")
	}
	if _, ok := byName["Bank"]; !ok {
		t.Error("byName should contain 'Bank'")
	}

	// Verify parent-child map
	if children, ok := parentChildMap[""]; ok {
		if len(children) != 1 {
			t.Errorf("expected 1 root account, got %d", len(children))
		}
	} else {
		t.Error("parentChildMap should have root entry")
	}

	assetChildren := parentChildMap["Assets"]
	if len(assetChildren) != 2 {
		t.Errorf("expected 2 children of Assets, got %d", len(assetChildren))
	}

	// Verify indent levels
	if filtered[0].Name != "Assets" {
		t.Errorf("first account should be 'Assets', got %q", filtered[0].Name)
	}
	if filtered[0].Indent != 0 {
		t.Errorf("root account indent should be 0, got %f", filtered[0].Indent)
	}

	// Find Petty Cash and check indent
	for _, acc := range filtered {
		if acc.Name == "Petty Cash" {
			if acc.Indent != 2 {
				t.Errorf("Petty Cash indent should be 2, got %f", acc.Indent)
			}
		}
	}
}

func TestFilterAccounts_EmptyInput(t *testing.T) {
	filtered, byName, parentChildMap := FilterAccounts([]Account{})
	if len(filtered) != 0 {
		t.Errorf("expected 0 filtered accounts, got %d", len(filtered))
	}
	if len(byName) != 0 {
		t.Errorf("expected empty byName map, got %d entries", len(byName))
	}
	if len(parentChildMap) != 0 {
		t.Errorf("expected empty parentChildMap, got %d entries", len(parentChildMap))
	}
}

func TestFilterOutZeroValueRows(t *testing.T) {
	accounts := []Account{
		{Name: "Income"},
		{Name: "Sales", ParentAccount: "Income"},
		{Name: "Other Income", ParentAccount: "Income"},
	}

	_, _, parentChildMap := FilterAccounts(accounts)

	data := []ReportRow{
		{Account: "Income", HasValue: false, Values: map[string]float64{}},
		{Account: "Sales", HasValue: true, Values: map[string]float64{"jan_2024": 100}},
		{Account: "Other Income", HasValue: false, Values: map[string]float64{}},
	}

	result := FilterOutZeroValueRows(data, parentChildMap, false)

	// Should keep Sales (has value) and Income (parent of Sales), but not Other Income
	if len(result) != 2 {
		t.Fatalf("expected 2 rows after filtering, got %d", len(result))
	}

	names := make(map[string]bool)
	for _, row := range result {
		names[row.Account] = true
	}

	if !names["Income"] {
		t.Error("expected Income to be kept (parent of non-zero child)")
	}
	if !names["Sales"] {
		t.Error("expected Sales to be kept (has value)")
	}
	if names["Other Income"] {
		t.Error("expected Other Income to be filtered out")
	}
}

func TestFilterOutZeroValueRows_ShowZeroValues(t *testing.T) {
	data := []ReportRow{
		{Account: "Income", HasValue: false, Values: map[string]float64{}},
		{Account: "Sales", HasValue: false, Values: map[string]float64{}},
	}

	result := FilterOutZeroValueRows(data, map[string][]*Account{}, true)

	if len(result) != 2 {
		t.Fatalf("expected all 2 rows when showZeroValues=true, got %d", len(result))
	}
}

func TestGetPeriodList_Monthly(t *testing.T) {
	start := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC)

	periods := GetPeriodList(start, end, "Monthly", false)

	if len(periods) != 12 {
		t.Fatalf("expected 12 monthly periods, got %d", len(periods))
	}

	// First period
	if periods[0].FromDate != start {
		t.Errorf("first period from_date: expected %v, got %v", start, periods[0].FromDate)
	}
	expectedFirstEnd := time.Date(2024, 1, 31, 0, 0, 0, 0, time.UTC)
	if periods[0].ToDate != expectedFirstEnd {
		t.Errorf("first period to_date: expected %v, got %v", expectedFirstEnd, periods[0].ToDate)
	}

	// Last period
	if periods[11].ToDate != end {
		t.Errorf("last period to_date: expected %v, got %v", end, periods[11].ToDate)
	}

	// Verify keys are set
	if periods[0].Key == "" {
		t.Error("period key should not be empty")
	}

	// Verify labels are set
	if periods[0].Label == "" {
		t.Error("period label should not be empty")
	}
}

func TestGetPeriodList_Quarterly(t *testing.T) {
	start := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC)

	periods := GetPeriodList(start, end, "Quarterly", false)

	if len(periods) != 4 {
		t.Fatalf("expected 4 quarterly periods, got %d", len(periods))
	}

	// Q1
	expectedQ1End := time.Date(2024, 3, 31, 0, 0, 0, 0, time.UTC)
	if periods[0].ToDate != expectedQ1End {
		t.Errorf("Q1 to_date: expected %v, got %v", expectedQ1End, periods[0].ToDate)
	}

	// Q4
	if periods[3].ToDate != end {
		t.Errorf("Q4 to_date: expected %v, got %v", end, periods[3].ToDate)
	}
}

func TestGetPeriodList_Yearly(t *testing.T) {
	start := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC)

	periods := GetPeriodList(start, end, "Yearly", false)

	if len(periods) != 1 {
		t.Fatalf("expected 1 yearly period, got %d", len(periods))
	}

	if periods[0].FromDate != start {
		t.Errorf("yearly from_date: expected %v, got %v", start, periods[0].FromDate)
	}
	if periods[0].ToDate != end {
		t.Errorf("yearly to_date: expected %v, got %v", end, periods[0].ToDate)
	}
}

func TestGetPeriodList_HalfYearly(t *testing.T) {
	start := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC)

	periods := GetPeriodList(start, end, "Half-Yearly", false)

	if len(periods) != 2 {
		t.Fatalf("expected 2 half-yearly periods, got %d", len(periods))
	}

	expectedH1End := time.Date(2024, 6, 30, 0, 0, 0, 0, time.UTC)
	if periods[0].ToDate != expectedH1End {
		t.Errorf("H1 to_date: expected %v, got %v", expectedH1End, periods[0].ToDate)
	}
}

func TestCalculateValues(t *testing.T) {
	acc := &Account{
		Name:   "Sales",
		Values: make(map[string]float64),
	}
	accountsByName := map[string]*Account{"Sales": acc}

	glEntries := map[string][]GLEntry{
		"Sales": {
			{
				Account:     "Sales",
				PostingDate: time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
				Debit:       0,
				Credit:      1000,
			},
			{
				Account:     "Sales",
				PostingDate: time.Date(2024, 2, 20, 0, 0, 0, 0, time.UTC),
				Debit:       0,
				Credit:      2000,
			},
		},
	}

	periodList := []Period{
		{
			FromDate:      time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			ToDate:        time.Date(2024, 1, 31, 0, 0, 0, 0, time.UTC),
			Key:           "jan_2024",
			YearStartDate: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			FromDate:      time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC),
			ToDate:        time.Date(2024, 2, 29, 0, 0, 0, 0, time.UTC),
			Key:           "feb_2024",
			YearStartDate: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		},
	}

	CalculateValues(accountsByName, glEntries, periodList, false, false)

	// Jan should have debit(0) - credit(1000) = -1000
	if acc.Values["jan_2024"] != -1000 {
		t.Errorf("expected jan_2024 = -1000, got %f", acc.Values["jan_2024"])
	}

	// Feb should have debit(0) - credit(2000) = -2000
	if acc.Values["feb_2024"] != -2000 {
		t.Errorf("expected feb_2024 = -2000, got %f", acc.Values["feb_2024"])
	}
}

func TestAccumulateValuesIntoParents(t *testing.T) {
	parent := Account{
		Name:   "Income",
		Values: map[string]float64{"jan_2024": 0},
	}
	child := Account{
		Name:          "Sales",
		ParentAccount: "Income",
		Values:        map[string]float64{"jan_2024": 500},
	}

	accounts := []Account{parent, child}
	accountsByName := map[string]*Account{
		"Income": &accounts[0],
		"Sales":  &accounts[1],
	}

	periodList := []Period{
		{Key: "jan_2024"},
	}

	AccumulateValuesIntoParents(accounts, accountsByName, periodList)

	if accountsByName["Income"].Values["jan_2024"] != 500 {
		t.Errorf("expected Income jan_2024 = 500, got %f", accountsByName["Income"].Values["jan_2024"])
	}
}

func TestPrepareData(t *testing.T) {
	accounts := []Account{
		{
			Name:        "Sales",
			AccountName: "Sales",
			Values:      map[string]float64{"jan_2024": -1000},
		},
	}

	periodList := []Period{
		{
			Key:           "jan_2024",
			Label:         "Jan 2024",
			YearStartDate: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			YearEndDate:   time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC),
		},
	}

	data := PrepareData(accounts, "Credit", periodList, "USD", false)

	if len(data) != 1 {
		t.Fatalf("expected 1 row, got %d", len(data))
	}

	// Credit balance_must_be should flip sign
	if data[0].Values["jan_2024"] != 1000 {
		t.Errorf("expected jan_2024 = 1000 (sign flipped for Credit), got %f", data[0].Values["jan_2024"])
	}

	if data[0].Currency != "USD" {
		t.Errorf("expected currency USD, got %s", data[0].Currency)
	}
}

func TestAddTotalRow(t *testing.T) {
	periodList := []Period{
		{Key: "jan_2024"},
		{Key: "feb_2024"},
	}

	data := []ReportRow{
		{
			Account:       "Income",
			ParentAccount: "",
			Values:        map[string]float64{"jan_2024": 1000, "feb_2024": 2000},
			Total:         3000,
		},
		{
			Account:       "Sales",
			ParentAccount: "Income",
			Values:        map[string]float64{"jan_2024": 1000, "feb_2024": 2000},
			Total:         3000,
		},
	}

	result := AddTotalRow(data, "Income", "Credit", periodList, "USD")

	// Should have original 2 rows + total row + blank row
	if len(result) != 4 {
		t.Fatalf("expected 4 rows (2 + total + blank), got %d", len(result))
	}

	totalRow := result[2]
	if totalRow.Values["jan_2024"] != 1000 {
		t.Errorf("total jan_2024: expected 1000, got %f", totalRow.Values["jan_2024"])
	}
	if totalRow.Values["feb_2024"] != 2000 {
		t.Errorf("total feb_2024: expected 2000, got %f", totalRow.Values["feb_2024"])
	}
	if totalRow.Total != 3000 {
		t.Errorf("total: expected 3000, got %f", totalRow.Total)
	}
}

func TestGetMonths(t *testing.T) {
	tests := []struct {
		start    time.Time
		end      time.Time
		expected int
	}{
		{
			time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC),
			12,
		},
		{
			time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			time.Date(2024, 3, 31, 0, 0, 0, 0, time.UTC),
			3,
		},
		{
			time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC),
			time.Date(2024, 6, 30, 0, 0, 0, 0, time.UTC),
			1,
		},
	}

	for _, tt := range tests {
		got := getMonths(tt.start, tt.end)
		if got != tt.expected {
			t.Errorf("getMonths(%v, %v) = %d, want %d", tt.start, tt.end, got, tt.expected)
		}
	}
}
