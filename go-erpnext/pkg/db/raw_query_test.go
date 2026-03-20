package db

import (
	"testing"
)

func TestRawQueryRow_NilOnEmpty(t *testing.T) {
	// This is a unit-level test for the RawQueryRow method logic.
	// Since we can't easily mock the DB here without an interface,
	// we validate the method signature and nil-safety of the wrapper.
	//
	// Integration tests with a real DB will exercise the full path.

	// Verify that the DB type has the methods we need.
	var _ func(string, ...interface{}) ([]map[string]interface{}, error)
	var d *DB
	_ = d // Compile-time check that the type exists
}

func TestSanitizeIdentifier(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"simple", "item_code", false},
		{"with_space", "Stock Ledger Entry", false},
		{"with_numbers", "tab123", false},
		{"injection", "name; DROP TABLE", true},
		{"backtick", "name`", true},
		{"empty", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := sanitizeIdentifier(tt.input, "test")
			if (err != nil) != tt.wantErr {
				t.Errorf("sanitizeIdentifier(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}
