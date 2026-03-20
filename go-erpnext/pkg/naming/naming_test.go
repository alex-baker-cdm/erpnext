package naming

import (
	"testing"
	"time"
)

func makeDate(year, month, day int) time.Time {
	return time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
}

func TestFormatYY(t *testing.T) {
	date := makeDate(2024, 1, 15)
	got := FormatNamingSeriesVariable("YY", date)
	if got != "24" {
		t.Errorf("YY for 2024-01-15: got %q, want %q", got, "24")
	}
}

func TestFormatYYYY(t *testing.T) {
	date := makeDate(2024, 1, 15)
	got := FormatNamingSeriesVariable("YYYY", date)
	if got != "2024" {
		t.Errorf("YYYY for 2024-01-15: got %q, want %q", got, "2024")
	}
}

func TestFormatMM(t *testing.T) {
	tests := []struct {
		date time.Time
		want string
	}{
		{makeDate(2024, 1, 15), "01"},
		{makeDate(2024, 12, 1), "12"},
	}
	for _, tc := range tests {
		got := FormatNamingSeriesVariable("MM", tc.date)
		if got != tc.want {
			t.Errorf("MM for %v: got %q, want %q", tc.date.Format("2006-01-02"), got, tc.want)
		}
	}
}

func TestFormatDD(t *testing.T) {
	tests := []struct {
		date time.Time
		want string
	}{
		{makeDate(2024, 1, 5), "05"},
		{makeDate(2024, 1, 15), "15"},
	}
	for _, tc := range tests {
		got := FormatNamingSeriesVariable("DD", tc.date)
		if got != tc.want {
			t.Errorf("DD for %v: got %q, want %q", tc.date.Format("2006-01-02"), got, tc.want)
		}
	}
}

func TestFormatJJJ(t *testing.T) {
	tests := []struct {
		date time.Time
		want string
	}{
		{makeDate(2024, 1, 1), "001"},
		{makeDate(2024, 2, 1), "032"},
		{makeDate(2024, 12, 31), "366"}, // 2024 is a leap year
	}
	for _, tc := range tests {
		got := FormatNamingSeriesVariable("JJJ", tc.date)
		if got != tc.want {
			t.Errorf("JJJ for %v: got %q, want %q", tc.date.Format("2006-01-02"), got, tc.want)
		}
	}
}

func TestFormatWW(t *testing.T) {
	tests := []struct {
		date time.Time
		want string
	}{
		{makeDate(2024, 1, 1), "01"},
		{makeDate(2024, 6, 15), "24"},
	}
	for _, tc := range tests {
		got := FormatNamingSeriesVariable("WW", tc.date)
		if got != tc.want {
			t.Errorf("WW for %v: got %q, want %q", tc.date.Format("2006-01-02"), got, tc.want)
		}
	}
}

func TestFYWithCallback(t *testing.T) {
	fyLookup := func(date time.Time, truncate bool) string {
		return "2024-2025"
	}
	got := FormatNamingSeriesVariableWithDB("FY", makeDate(2024, 6, 15), fyLookup, nil)
	if got != "2024-2025" {
		t.Errorf("FY with callback: got %q, want %q", got, "2024-2025")
	}
}

func TestTFYWithCallback(t *testing.T) {
	fyLookup := func(date time.Time, truncate bool) string {
		if truncate {
			return "24-25"
		}
		return "2024-2025"
	}
	got := FormatNamingSeriesVariableWithDB("TFY", makeDate(2024, 6, 15), fyLookup, nil)
	if got != "24-25" {
		t.Errorf("TFY with callback: got %q, want %q", got, "24-25")
	}
}

func TestABBRWithCallback(t *testing.T) {
	abbrLookup := func() string {
		return "TC"
	}
	got := FormatNamingSeriesVariableWithDB("ABBR", makeDate(2024, 1, 1), nil, abbrLookup)
	if got != "TC" {
		t.Errorf("ABBR with callback: got %q, want %q", got, "TC")
	}
}

func TestFYWithoutCallback(t *testing.T) {
	got := FormatNamingSeriesVariableWithDB("FY", makeDate(2024, 1, 1), nil, nil)
	if got != "" {
		t.Errorf("FY without callback: got %q, want %q", got, "")
	}
}

func TestTFYWithoutCallback(t *testing.T) {
	got := FormatNamingSeriesVariableWithDB("TFY", makeDate(2024, 1, 1), nil, nil)
	if got != "" {
		t.Errorf("TFY without callback: got %q, want %q", got, "")
	}
}

func TestABBRWithoutCallback(t *testing.T) {
	got := FormatNamingSeriesVariableWithDB("ABBR", makeDate(2024, 1, 1), nil, nil)
	if got != "" {
		t.Errorf("ABBR without callback: got %q, want %q", got, "")
	}
}

func TestUnknownVariable(t *testing.T) {
	got := FormatNamingSeriesVariable("UNKNOWN", makeDate(2024, 1, 1))
	if got != "" {
		t.Errorf("Unknown variable: got %q, want %q", got, "")
	}
}

func TestDetermineConsecutiveWeekNumber(t *testing.T) {
	tests := []struct {
		date time.Time
		want string
	}{
		{makeDate(2024, 1, 1), "01"},
		{makeDate(2024, 6, 15), "24"},
		{makeDate(2024, 12, 31), "01"}, // ISO week: 2024-12-31 is week 1 of 2025
	}
	for _, tc := range tests {
		got := DetermineConsecutiveWeekNumber(tc.date)
		if got != tc.want {
			t.Errorf("DetermineConsecutiveWeekNumber(%v): got %q, want %q", tc.date.Format("2006-01-02"), got, tc.want)
		}
	}
}

func TestVariousDatesComprehensive(t *testing.T) {
	// Test multiple dates across different months/years for comprehensive coverage
	tests := []struct {
		name     string
		variable string
		date     time.Time
		want     string
	}{
		// Year boundaries
		{"YY 2000", "YY", makeDate(2000, 1, 1), "00"},
		{"YY 1999", "YY", makeDate(1999, 12, 31), "99"},
		{"YYYY 2000", "YYYY", makeDate(2000, 6, 15), "2000"},
		{"YYYY 2025", "YYYY", makeDate(2025, 3, 20), "2025"},

		// Month boundaries
		{"MM February", "MM", makeDate(2024, 2, 28), "02"},
		{"MM June", "MM", makeDate(2024, 6, 1), "06"},
		{"MM September", "MM", makeDate(2024, 9, 30), "09"},

		// Day boundaries
		{"DD first", "DD", makeDate(2024, 1, 1), "01"},
		{"DD last of month", "DD", makeDate(2024, 1, 31), "31"},
		{"DD mid", "DD", makeDate(2024, 7, 20), "20"},

		// Day of year - non-leap year
		{"JJJ Jan 1 non-leap", "JJJ", makeDate(2023, 1, 1), "001"},
		{"JJJ Dec 31 non-leap", "JJJ", makeDate(2023, 12, 31), "365"},
		{"JJJ Mar 1 non-leap", "JJJ", makeDate(2023, 3, 1), "060"},

		// Day of year - leap year
		{"JJJ Mar 1 leap", "JJJ", makeDate(2024, 3, 1), "061"},
		{"JJJ Feb 29 leap", "JJJ", makeDate(2024, 2, 29), "060"},

		// Week numbers
		{"WW mid-year", "WW", makeDate(2024, 7, 1), "27"},
		{"WW start of year", "WW", makeDate(2025, 1, 6), "02"},

		// FY/TFY/ABBR without callbacks via FormatNamingSeriesVariable
		{"FY no callback", "FY", makeDate(2024, 1, 1), ""},
		{"TFY no callback", "TFY", makeDate(2024, 1, 1), ""},
		{"ABBR no callback", "ABBR", makeDate(2024, 1, 1), ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := FormatNamingSeriesVariable(tc.variable, tc.date)
			if got != tc.want {
				t.Errorf("%s: FormatNamingSeriesVariable(%q, %v) = %q, want %q",
					tc.name, tc.variable, tc.date.Format("2006-01-02"), got, tc.want)
			}
		})
	}
}

func TestFYCallbackReceivesTruncateFlag(t *testing.T) {
	// Verify that FY passes truncate=false and TFY passes truncate=true
	var receivedTruncate bool
	fyLookup := func(date time.Time, truncate bool) string {
		receivedTruncate = truncate
		return "test"
	}

	FormatNamingSeriesVariableWithDB("FY", makeDate(2024, 1, 1), fyLookup, nil)
	if receivedTruncate != false {
		t.Errorf("FY should pass truncate=false, got true")
	}

	FormatNamingSeriesVariableWithDB("TFY", makeDate(2024, 1, 1), fyLookup, nil)
	if receivedTruncate != true {
		t.Errorf("TFY should pass truncate=true, got false")
	}
}

func TestFYCallbackReceivesDate(t *testing.T) {
	// Verify the date is passed through to the callback
	expectedDate := makeDate(2024, 6, 15)
	var receivedDate time.Time
	fyLookup := func(date time.Time, truncate bool) string {
		receivedDate = date
		return "2024-2025"
	}

	FormatNamingSeriesVariableWithDB("FY", expectedDate, fyLookup, nil)
	if !receivedDate.Equal(expectedDate) {
		t.Errorf("FY callback received date %v, expected %v", receivedDate, expectedDate)
	}
}
