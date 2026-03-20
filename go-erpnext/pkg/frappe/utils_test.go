package frappe

import (
	"testing"
	"time"
)

func TestFlt(t *testing.T) {
	tests := []struct {
		name      string
		value     float64
		precision int
		expected  float64
	}{
		{"zero", 0.0, 2, 0.0},
		{"positive round", 1.2345, 2, 1.23},
		{"positive round up", 1.2355, 2, 1.24},
		{"negative precision", 1.2345, -1, 1.2345},
		{"large precision", 1.23456789, 6, 1.234568},
		{"negative value", -1.2345, 2, -1.23},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Flt(tt.value, tt.precision)
			if result != tt.expected {
				t.Errorf("Flt(%v, %d) = %v, want %v", tt.value, tt.precision, result, tt.expected)
			}
		})
	}
}

func TestFltFromAny(t *testing.T) {
	tests := []struct {
		name      string
		value     interface{}
		precision int
		expected  float64
	}{
		{"float64", 1.234, 2, 1.23},
		{"int", 5, 2, 5.0},
		{"string", "3.456", 2, 3.46},
		{"empty string", "", 2, 0.0},
		{"nil", nil, 2, 0.0},
		{"invalid string", "abc", 2, 0.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FltFromAny(tt.value, tt.precision)
			if result != tt.expected {
				t.Errorf("FltFromAny(%v, %d) = %v, want %v", tt.value, tt.precision, result, tt.expected)
			}
		})
	}
}

func TestCint(t *testing.T) {
	tests := []struct {
		name     string
		value    interface{}
		expected int
	}{
		{"int", 5, 5},
		{"float64", 3.7, 3},
		{"string int", "5", 5},
		{"string float", "3.5", 3},
		{"empty string", "", 0},
		{"nil", nil, 0},
		{"bool true", true, 1},
		{"bool false", false, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Cint(tt.value)
			if result != tt.expected {
				t.Errorf("Cint(%v) = %v, want %v", tt.value, result, tt.expected)
			}
		})
	}
}

func TestGetdate(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		expected time.Time
	}{
		{"standard format", "2024-01-15", time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)},
		{"with time", "2024-01-15 10:30:00", time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)},
		{"empty string", "", time.Time{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Getdate(tt.value)
			if !result.Equal(tt.expected) {
				t.Errorf("Getdate(%q) = %v, want %v", tt.value, result, tt.expected)
			}
		})
	}
}

func TestAddDays(t *testing.T) {
	date := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
	result := AddDays(date, 10)
	expected := time.Date(2024, 1, 25, 0, 0, 0, 0, time.UTC)
	if !result.Equal(expected) {
		t.Errorf("AddDays(%v, 10) = %v, want %v", date, result, expected)
	}
}

func TestAddMonths(t *testing.T) {
	tests := []struct {
		name     string
		date     time.Time
		months   int
		expected time.Time
	}{
		{"add 1 month", time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC), 1, time.Date(2024, 2, 15, 0, 0, 0, 0, time.UTC)},
		{"month end clamp", time.Date(2024, 1, 31, 0, 0, 0, 0, time.UTC), 1, time.Date(2024, 3, 2, 0, 0, 0, 0, time.UTC)},
		{"add 12 months", time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC), 12, time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := AddMonths(tt.date, tt.months)
			if !result.Equal(tt.expected) {
				t.Errorf("AddMonths(%v, %d) = %v, want %v", tt.date, tt.months, result, tt.expected)
			}
		})
	}
}

func TestGetLastDay(t *testing.T) {
	tests := []struct {
		name     string
		date     time.Time
		expected time.Time
	}{
		{"january", time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC), time.Date(2024, 1, 31, 0, 0, 0, 0, time.UTC)},
		{"february leap", time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC), time.Date(2024, 2, 29, 0, 0, 0, 0, time.UTC)},
		{"february non-leap", time.Date(2023, 2, 1, 0, 0, 0, 0, time.UTC), time.Date(2023, 2, 28, 0, 0, 0, 0, time.UTC)},
		{"december", time.Date(2024, 12, 25, 0, 0, 0, 0, time.UTC), time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetLastDay(tt.date)
			if !result.Equal(tt.expected) {
				t.Errorf("GetLastDay(%v) = %v, want %v", tt.date, result, tt.expected)
			}
		})
	}
}
