package reports

import (
	"github.com/alex-baker-cdm/erpnext/go-erpnext/pkg/frappe"
)

// ConvertibleColumnInfo tracks which columns need UOM conversion and how.
type ConvertibleColumnInfo struct {
	ConvertedCol string
	ForType      string // "rate" or "qty"
}

// AddAdditionalUOMColumns adds alternate UOM columns next to convertible columns
// in a report, and populates the alternate values in the result rows.
// Columns with a "convertible" property ("rate" or "qty") get a duplicate column
// appended with "_alt" suffix and values converted using the provided conversion factors.
func AddAdditionalUOMColumns(
	columns []map[string]interface{},
	result []map[string]interface{},
	includeUOM string,
	conversionFactors map[string]float64,
) ([]map[string]interface{}, []map[string]interface{}) {
	if includeUOM == "" || len(conversionFactors) == 0 {
		return columns, result
	}

	convertibleMap := make(map[string]ConvertibleColumnInfo)

	// Iterate in reverse to safely insert columns
	for colIdx := len(columns) - 1; colIdx >= 0; colIdx-- {
		col := columns[colIdx]
		convertible, _ := col["convertible"].(string)
		if convertible != "rate" && convertible != "qty" {
			continue
		}

		fieldname, _ := col["fieldname"].(string)

		// Create a copy for the alternate column
		altCol := make(map[string]interface{})
		for k, v := range col {
			altCol[k] = v
		}
		altCol["fieldname"] = fieldname + "_alt"

		label, _ := altCol["label"].(string)
		if convertible == "rate" {
			altCol["label"] = label + " (per " + includeUOM + ")"
		} else {
			altCol["label"] = label + " (" + includeUOM + ")"
		}

		// Insert after current column
		nextIdx := colIdx + 1
		newCols := make([]map[string]interface{}, 0, len(columns)+1)
		newCols = append(newCols, columns[:nextIdx]...)
		newCols = append(newCols, altCol)
		newCols = append(newCols, columns[nextIdx:]...)
		columns = newCols

		convertibleMap[fieldname] = ConvertibleColumnInfo{
			ConvertedCol: fieldname + "_alt",
			ForType:      convertible,
		}
	}

	// Populate alternate column values in result
	for rowIdx, row := range result {
		itemCode, _ := row["item_code"].(string)
		conversionFactor := conversionFactors[itemCode]
		if conversionFactor == 0 {
			conversionFactor = 1.0
		}

		for convertibleCol, info := range convertibleMap {
			valueBefore := frappe.FltFromAny(row[convertibleCol], -1)
			if info.ForType == "rate" {
				row[info.ConvertedCol] = frappe.Flt(valueBefore*conversionFactor, -1)
			} else {
				row[info.ConvertedCol] = frappe.Flt(valueBefore/conversionFactor, -1)
			}
		}
		result[rowIdx] = row
	}

	return columns, result
}

// IsRepostingItemValuationInProgress checks if there are pending item valuation
// reposting entries. Returns true if any "Repost Item Valuation" documents
// are in "Queued" or "In Progress" status.
//
// TODO: This requires database access to query the Repost Item Valuation doctype.
// For now, returns false as a no-op. Full implementation requires the DB layer
// to check: SELECT name FROM `tabRepost Item Valuation`
// WHERE docstatus=1 AND status IN ('Queued', 'In Progress') LIMIT 1
func IsRepostingItemValuationInProgress() bool {
	return false
}
