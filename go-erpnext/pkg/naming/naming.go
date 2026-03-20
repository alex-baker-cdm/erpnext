// Package naming provides Go equivalents of ERPNext's naming series utilities.
package naming

import (
	"fmt"
	"time"
)

// FYLookupFunc is a callback for fiscal year lookup (DB-dependent).
// Takes a date and a truncate flag, and returns the fiscal year string.
type FYLookupFunc func(date time.Time, truncate bool) string

// ABBRLookupFunc is a callback for company abbreviation lookup (DB-dependent).
type ABBRLookupFunc func() string

// FormatNamingSeriesVariable formats a naming series variable using the given date.
// Supported variables:
//   - "YY"   -> 2-digit year (e.g., "24")
//   - "YYYY" -> 4-digit year (e.g., "2024")
//   - "MM"   -> 2-digit month (e.g., "01")
//   - "DD"   -> 2-digit day (e.g., "15")
//   - "JJJ"  -> day of year, zero-padded to 3 digits (e.g., "015")
//   - "WW"   -> ISO week number, zero-padded to 2 digits (e.g., "03")
//   - "FY"   -> fiscal year (requires DB lookup, returns empty without callback)
//   - "TFY"  -> truncated fiscal year (requires DB lookup, returns empty without callback)
//   - "ABBR" -> company abbreviation (requires DB lookup, returns empty without callback)
//
// For FY, TFY, and ABBR variables, use FormatNamingSeriesVariableWithDB instead.
// This function returns an empty string for those variables.
func FormatNamingSeriesVariable(variable string, date time.Time) string {
	return FormatNamingSeriesVariableWithDB(variable, date, nil, nil)
}

// FormatNamingSeriesVariableWithDB formats variables that may need DB access.
// For date variables (YY, YYYY, MM, DD, JJJ, WW), works without callbacks.
// For FY/TFY, requires fyLookup. For ABBR, requires abbrLookup.
// If the required callback is nil, returns an empty string.
// For unknown variables, returns an empty string.
func FormatNamingSeriesVariableWithDB(variable string, date time.Time, fyLookup FYLookupFunc, abbrLookup ABBRLookupFunc) string {
	switch variable {
	case "YY":
		return date.Format("06")
	case "YYYY":
		return date.Format("2006")
	case "MM":
		return date.Format("01")
	case "DD":
		return date.Format("02")
	case "JJJ":
		return fmt.Sprintf("%03d", date.YearDay())
	case "WW":
		return DetermineConsecutiveWeekNumber(date)
	case "FY":
		if fyLookup == nil {
			return ""
		}
		return fyLookup(date, false)
	case "TFY":
		if fyLookup == nil {
			return ""
		}
		return fyLookup(date, true)
	case "ABBR":
		if abbrLookup == nil {
			return ""
		}
		return abbrLookup()
	default:
		return ""
	}
}

// DetermineConsecutiveWeekNumber returns the ISO week number for the given date,
// zero-padded to 2 digits.
func DetermineConsecutiveWeekNumber(date time.Time) string {
	_, week := date.ISOWeek()
	return fmt.Sprintf("%02d", week)
}
