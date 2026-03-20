package dashboard

import (
	"math"
	"testing"
	"time"
)

func TestGetDatesFromTimegrain(t *testing.T) {
	from := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
	to := time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC)

	dates := getDatesFromTimegrain(from, to)

	if len(dates) == 0 {
		t.Fatal("expected at least one date point")
	}

	// First date should be end of January 2024
	first := dates[0]
	if first.Month() != time.January || first.Day() != 31 || first.Year() != 2024 {
		t.Errorf("expected first date 2024-01-31, got %s", first.Format("2006-01-02"))
	}

	// Each date should be the last day of its month
	for _, d := range dates {
		nextMonth := time.Date(d.Year(), d.Month()+1, 1, 0, 0, 0, 0, time.UTC)
		lastDay := nextMonth.AddDate(0, 0, -1)
		if d.Day() != lastDay.Day() {
			t.Errorf("date %s is not last day of month (expected day %d)", d.Format("2006-01-02"), lastDay.Day())
		}
	}

	// Last date should be >= toDate
	last := dates[len(dates)-1]
	if last.Before(to) {
		t.Errorf("expected last date >= %s, got %s", to.Format("2006-01-02"), last.Format("2006-01-02"))
	}
}

func TestGetDatesFromTimegrainSingleMonth(t *testing.T) {
	from := time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2024, 3, 15, 0, 0, 0, 0, time.UTC)

	dates := getDatesFromTimegrain(from, to)

	if len(dates) == 0 {
		t.Fatal("expected at least one date")
	}

	// Should include end of March
	found := false
	for _, d := range dates {
		if d.Month() == time.March && d.Day() == 31 && d.Year() == 2024 {
			found = true
		}
	}
	if !found {
		t.Error("expected to find 2024-03-31 in dates")
	}
}

func TestBuildResultDebitAccount(t *testing.T) {
	// Asset account (debit-type, cumulative)
	dates := []time.Time{
		time.Date(2024, 1, 31, 0, 0, 0, 0, time.UTC),
		time.Date(2024, 2, 29, 0, 0, 0, 0, time.UTC),
		time.Date(2024, 3, 31, 0, 0, 0, 0, time.UTC),
	}

	entries := []glEntry{
		{PostingDate: time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC), Debit: 1000, Credit: 0},
		{PostingDate: time.Date(2024, 2, 10, 0, 0, 0, 0, time.UTC), Debit: 500, Credit: 100},
		{PostingDate: time.Date(2024, 3, 5, 0, 0, 0, 0, time.UTC), Debit: 200, Credit: 50},
	}

	result := buildResult("Test Account", dates, entries, "Asset")

	if len(result) != 3 {
		t.Fatalf("expected 3 results, got %d", len(result))
	}

	// Jan: 1000 (debit - credit for Asset = positive)
	// Feb: 1000 + 400 = 1400 (cumulative for balance sheet)
	// Mar: 1400 + 150 = 1550
	if result[0].balance != 1000 {
		t.Errorf("expected Jan balance 1000, got %f", result[0].balance)
	}
	if result[1].balance != 1400 {
		t.Errorf("expected Feb balance 1400, got %f", result[1].balance)
	}
	if result[2].balance != 1550 {
		t.Errorf("expected Mar balance 1550, got %f", result[2].balance)
	}
}

func TestBuildResultCreditAccount(t *testing.T) {
	// Liability account (credit-type, cumulative, sign inverted)
	dates := []time.Time{
		time.Date(2024, 1, 31, 0, 0, 0, 0, time.UTC),
		time.Date(2024, 2, 29, 0, 0, 0, 0, time.UTC),
	}

	entries := []glEntry{
		{PostingDate: time.Date(2024, 1, 10, 0, 0, 0, 0, time.UTC), Debit: 0, Credit: 500},
		{PostingDate: time.Date(2024, 2, 15, 0, 0, 0, 0, time.UTC), Debit: 100, Credit: 300},
	}

	result := buildResult("Liability Account", dates, entries, "Liability")

	// Jan: debit-credit = 0-500 = -500, inverted = 500
	// Feb: cumulative: 500 + (-(100-300)) = 500 + 200 = 700
	if result[0].balance != 500 {
		t.Errorf("expected Jan balance 500, got %f", result[0].balance)
	}
	if result[1].balance != 700 {
		t.Errorf("expected Feb balance 700, got %f", result[1].balance)
	}
}

func TestBuildResultIncomeAccount(t *testing.T) {
	// Income account (credit-type, NOT cumulative)
	dates := []time.Time{
		time.Date(2024, 1, 31, 0, 0, 0, 0, time.UTC),
		time.Date(2024, 2, 29, 0, 0, 0, 0, time.UTC),
	}

	entries := []glEntry{
		{PostingDate: time.Date(2024, 1, 10, 0, 0, 0, 0, time.UTC), Debit: 0, Credit: 1000},
		{PostingDate: time.Date(2024, 2, 15, 0, 0, 0, 0, time.UTC), Debit: 0, Credit: 2000},
	}

	result := buildResult("Income Account", dates, entries, "Income")

	// Jan: -(0-1000) = 1000, not cumulative
	// Feb: -(0-2000) = 2000
	if result[0].balance != 1000 {
		t.Errorf("expected Jan balance 1000, got %f", result[0].balance)
	}
	if result[1].balance != 2000 {
		t.Errorf("expected Feb balance 2000, got %f", result[1].balance)
	}
}

func TestBuildResultExpenseAccount(t *testing.T) {
	// Expense account (debit-type, NOT cumulative)
	dates := []time.Time{
		time.Date(2024, 1, 31, 0, 0, 0, 0, time.UTC),
		time.Date(2024, 2, 29, 0, 0, 0, 0, time.UTC),
	}

	entries := []glEntry{
		{PostingDate: time.Date(2024, 1, 10, 0, 0, 0, 0, time.UTC), Debit: 500, Credit: 0},
		{PostingDate: time.Date(2024, 2, 15, 0, 0, 0, 0, time.UTC), Debit: 300, Credit: 0},
	}

	result := buildResult("Expense Account", dates, entries, "Expense")

	// Jan: 500-0 = 500, no inversion, not cumulative
	// Feb: 300
	if result[0].balance != 500 {
		t.Errorf("expected Jan balance 500, got %f", result[0].balance)
	}
	if result[1].balance != 300 {
		t.Errorf("expected Feb balance 300, got %f", result[1].balance)
	}
}

func TestBuildResultEmptyEntries(t *testing.T) {
	dates := []time.Time{
		time.Date(2024, 1, 31, 0, 0, 0, 0, time.UTC),
		time.Date(2024, 2, 29, 0, 0, 0, 0, time.UTC),
	}

	result := buildResult("Empty Account", dates, nil, "Asset")

	if len(result) != 2 {
		t.Fatalf("expected 2 results, got %d", len(result))
	}
	for i, r := range result {
		if math.Abs(r.balance) > 0.001 {
			t.Errorf("expected zero balance at index %d, got %f", i, r.balance)
		}
	}
}

func TestBuildResultMultipleEntriesSameDate(t *testing.T) {
	dates := []time.Time{
		time.Date(2024, 1, 31, 0, 0, 0, 0, time.UTC),
	}

	entries := []glEntry{
		{PostingDate: time.Date(2024, 1, 10, 0, 0, 0, 0, time.UTC), Debit: 100, Credit: 0},
		{PostingDate: time.Date(2024, 1, 10, 0, 0, 0, 0, time.UTC), Debit: 200, Credit: 50},
		{PostingDate: time.Date(2024, 1, 20, 0, 0, 0, 0, time.UTC), Debit: 300, Credit: 0},
	}

	result := buildResult("Test", dates, entries, "Asset")

	// All entries fall in Jan: (100-0) + (200-50) + (300-0) = 550
	if result[0].balance != 550 {
		t.Errorf("expected balance 550, got %f", result[0].balance)
	}
}
