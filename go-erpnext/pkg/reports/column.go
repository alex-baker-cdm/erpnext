// Package reports provides the report runner framework and shared report utilities
// for the Go ERPNext migration. It includes column definitions, financial statement
// helpers, currency conversion, trend analysis, and stock report utilities.
package reports

import (
	"strings"
)

// Column represents a single column definition in a Frappe report.
// It matches the JSON schema expected by the Frappe frontend.
type Column struct {
	Fieldname string `json:"fieldname"`
	Fieldtype string `json:"fieldtype"`
	Label     string `json:"label"`
	Width     int    `json:"width,omitempty"`
	Options   string `json:"options,omitempty"`
}

// ParseColumnShorthand parses Frappe's shorthand column format string into a Column struct.
// The shorthand format is "Label:Fieldtype/Options:Width" or "Label:Fieldtype:Width".
// Examples:
//
//	"Item:Link/Item:120"       -> Column{Label:"Item", Fieldtype:"Link", Options:"Item", Width:120}
//	"Amount:Currency:150"      -> Column{Label:"Amount", Fieldtype:"Currency", Width:150}
//	"Name:Data:200"            -> Column{Label:"Name", Fieldtype:"Data", Width:200}
//	"Total(Qty):Float:120"     -> Column{Label:"Total(Qty)", Fieldtype:"Float", Width:120}
//	"Amt:Currency/currency:120"-> Column{Label:"Amt", Fieldtype:"Currency", Options:"currency", Width:120}
func ParseColumnShorthand(shorthand string) Column {
	col := Column{}

	parts := strings.SplitN(shorthand, ":", 3)
	if len(parts) == 0 {
		return col
	}

	col.Label = parts[0]
	// Default fieldname: lowercase label with spaces replaced by underscores
	col.Fieldname = strings.ToLower(strings.ReplaceAll(col.Label, " ", "_"))

	if len(parts) >= 2 {
		fieldtypePart := parts[1]
		// Check if fieldtype contains options (e.g., "Link/Item")
		if slashIdx := strings.Index(fieldtypePart, "/"); slashIdx >= 0 {
			col.Fieldtype = fieldtypePart[:slashIdx]
			col.Options = fieldtypePart[slashIdx+1:]
		} else {
			col.Fieldtype = fieldtypePart
		}
	}

	if len(parts) >= 3 {
		col.Width = parseWidth(parts[2])
	}

	return col
}

// parseWidth converts a width string to int. Returns 0 on failure.
func parseWidth(s string) int {
	s = strings.TrimSpace(s)
	width := 0
	for _, ch := range s {
		if ch >= '0' && ch <= '9' {
			width = width*10 + int(ch-'0')
		} else {
			break
		}
	}
	return width
}

// ParseColumnShorthands parses a slice of Frappe shorthand column strings into Column structs.
func ParseColumnShorthands(shorthands []string) []Column {
	cols := make([]Column, len(shorthands))
	for i, s := range shorthands {
		cols[i] = ParseColumnShorthand(s)
	}
	return cols
}
