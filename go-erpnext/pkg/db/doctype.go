// Package db provides doctype schema reading from Frappe JSON files.
package db

import (
	"encoding/json"
	"fmt"
	"os"
)

// DoctypeField represents a single field in a Frappe doctype.
type DoctypeField struct {
	Fieldname string `json:"fieldname"`
	Fieldtype string `json:"fieldtype"`
	Label     string `json:"label"`
	Options   string `json:"options"`
	Reqd      int    `json:"reqd"`
	InFilter  int    `json:"in_filter"`
}

// DoctypeSchema represents the parsed structure of a Frappe doctype JSON file.
type DoctypeSchema struct {
	Name       string         `json:"name"`
	Module     string         `json:"module"`
	Fields     []DoctypeField `json:"fields"`
	IsTree     int            `json:"is_tree"`
	Autoname   string         `json:"autoname"`
	IsSingle   int            `json:"issingle"`
	IsTable    int            `json:"istable"`
	TableName  string         // Computed: `tabName`
}

// ReadDoctypeSchema reads and parses a Frappe doctype JSON file.
// The path should point to the JSON file, e.g.,
// "erpnext/stock/doctype/item/item.json"
func ReadDoctypeSchema(jsonPath string) (*DoctypeSchema, error) {
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		return nil, fmt.Errorf("reading doctype JSON: %w", err)
	}

	var schema DoctypeSchema
	if err := json.Unmarshal(data, &schema); err != nil {
		return nil, fmt.Errorf("parsing doctype JSON: %w", err)
	}

	schema.TableName = fmt.Sprintf("tab%s", schema.Name)
	return &schema, nil
}

// GetFieldNames returns a list of all fieldnames in the doctype.
func (ds *DoctypeSchema) GetFieldNames() []string {
	names := make([]string, 0, len(ds.Fields))
	for _, f := range ds.Fields {
		if f.Fieldname != "" {
			names = append(names, f.Fieldname)
		}
	}
	return names
}

// GetField returns the field definition for the given fieldname, or nil if not found.
func (ds *DoctypeSchema) GetField(fieldname string) *DoctypeField {
	for i := range ds.Fields {
		if ds.Fields[i].Fieldname == fieldname {
			return &ds.Fields[i]
		}
	}
	return nil
}

// IsLinkField returns true if the field is a Link field (references another doctype).
func (f *DoctypeField) IsLinkField() bool {
	return f.Fieldtype == "Link"
}

// IsTableField returns true if the field is a Table field (child table).
func (f *DoctypeField) IsTableField() bool {
	return f.Fieldtype == "Table" || f.Fieldtype == "Table MultiSelect"
}
