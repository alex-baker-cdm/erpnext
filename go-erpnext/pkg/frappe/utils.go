// Package frappe provides Go equivalents of common Frappe utility functions.
package frappe

import (
	"strconv"
	"strings"
	"time"
)

// Flt converts a value to float64 and rounds to the given precision.
// This is the Go equivalent of frappe.utils.flt.
// If precision is < 0, no rounding is applied.
// Uses strconv.FormatFloat with 'f' verb to match Python's round() behaviour,
// which performs banker's rounding on the decimal representation rather than
// on the binary floating-point value multiplied by a power of 10.
func Flt(value float64, precision int) float64 {
	if precision < 0 {
		return value
	}
	s := strconv.FormatFloat(value, 'f', precision, 64)
	result, _ := strconv.ParseFloat(s, 64)
	return result
}

// FltFromAny converts an arbitrary value to float64 and rounds to precision.
// Handles string, int, int64, float32, float64, and nil.
func FltFromAny(value interface{}, precision int) float64 {
	var f float64
	switch v := value.(type) {
	case float64:
		f = v
	case float32:
		f = float64(v)
	case int:
		f = float64(v)
	case int64:
		f = float64(v)
	case string:
		v = strings.TrimSpace(v)
		if v == "" {
			f = 0.0
		} else {
			parsed, err := strconv.ParseFloat(v, 64)
			if err != nil {
				f = 0.0
			} else {
				f = parsed
			}
		}
	case nil:
		f = 0.0
	default:
		f = 0.0
	}
	return Flt(f, precision)
}

// Cint converts an arbitrary value to int.
// This is the Go equivalent of frappe.utils.cint.
func Cint(value interface{}) int {
	switch v := value.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	case float32:
		return int(v)
	case string:
		v = strings.TrimSpace(v)
		if v == "" {
			return 0
		}
		// Try parsing as float first to handle "3.5" -> 3
		f, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return 0
		}
		return int(f)
	case bool:
		if v {
			return 1
		}
		return 0
	case nil:
		return 0
	default:
		return 0
	}
}

// Getdate parses a date string in "YYYY-MM-DD" format and returns a time.Time.
// This is the Go equivalent of frappe.utils.getdate.
// If parsing fails, returns the zero time.
func Getdate(value string) time.Time {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}
	}

	// Try common formats
	formats := []string{
		"2006-01-02",
		"2006-01-02 15:04:05",
		"01-02-2006",
		"02-01-2006",
		"2006/01/02",
	}

	for _, format := range formats {
		t, err := time.Parse(format, value)
		if err == nil {
			// Return date-only (zero time portion)
			return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
		}
	}
	return time.Time{}
}

// AddDays adds the specified number of days to a date.
// This is the Go equivalent of frappe.utils.add_days.
func AddDays(date time.Time, days int) time.Time {
	return date.AddDate(0, 0, days)
}

// AddMonths adds the specified number of months to a date.
// This is the Go equivalent of frappe.utils.add_months.
// Handles month-end clamping (e.g., Jan 31 + 1 month = Feb 28/29).
// Go's time.AddDate overflows (Jan 31 + 1 month = Mar 3), so we clamp
// to the last day of the target month when the original day exceeds it.
func AddMonths(date time.Time, months int) time.Time {
	y, m, d := date.Date()
	loc := date.Location()

	// Target month and year — use separate paths for positive and negative
	// offsets so the logic is explicit and not fragile.
	var targetMonth time.Month
	var targetYear int

	offset := int(m) - 1 + months
	if offset >= 0 {
		targetMonth = time.Month(offset%12 + 1)
		targetYear = y + offset/12
	} else {
		rem := offset % 12
		if rem < 0 {
			rem += 12
		}
		targetMonth = time.Month(rem + 1)
		// Integer division in Go truncates toward zero, so for negative
		// offsets we adjust by subtracting 1 when there is a remainder.
		targetYear = y + (offset-rem)/12
	}

	// Last day of target month
	lastDay := time.Date(targetYear, targetMonth+1, 0, 0, 0, 0, 0, loc).Day()

	// Clamp day to last day of target month
	if d > lastDay {
		d = lastDay
	}

	return time.Date(targetYear, targetMonth, d, 0, 0, 0, 0, loc)
}

// GetLastDay returns the last day of the month for the given date.
// This is the Go equivalent of frappe.utils.get_last_day.
func GetLastDay(date time.Time) time.Time {
	// Go to first day of next month, then subtract one day
	y, m, _ := date.Date()
	firstOfNextMonth := time.Date(y, m+1, 1, 0, 0, 0, 0, date.Location())
	return firstOfNextMonth.AddDate(0, 0, -1)
}
