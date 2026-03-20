package db

import (
	"strings"
	"testing"
)

func TestSanitizeOrderBy(t *testing.T) {
	validCases := []struct {
		name       string
		orderBy    string
		wantClause string
	}{
		{"field only defaults to ASC", "name", " ORDER BY `name` ASC"},
		{"field with ASC", "creation ASC", " ORDER BY `creation` ASC"},
		{"field with DESC", "modified DESC", " ORDER BY `modified` DESC"},
		{"field with lowercase asc", "creation asc", " ORDER BY `creation` ASC"},
		{"field with lowercase desc", "modified desc", " ORDER BY `modified` DESC"},
		{"field with mixed case Desc", "modified Desc", " ORDER BY `modified` DESC"},
		{"field with underscores", "custom_field DESC", " ORDER BY `custom_field` DESC"},
	}

	for _, tc := range validCases {
		t.Run("valid/"+tc.name, func(t *testing.T) {
			got, err := sanitizeOrderBy(tc.orderBy)
			if err != nil {
				t.Fatalf("unexpected error for OrderBy %q: %v", tc.orderBy, err)
			}
			if got != tc.wantClause {
				t.Errorf("sanitizeOrderBy(%q) = %q, want %q", tc.orderBy, got, tc.wantClause)
			}
		})
	}

	invalidCases := []struct {
		name    string
		orderBy string
		wantMsg string
	}{
		{"SQL injection with semicolon", "name; DROP TABLE foo", "invalid order_by clause"},
		{"SQL injection with OR", "name OR 1=1", "invalid order_by clause"},
		{"too many parts", "name DESC extra", "invalid order_by clause"},
		{"invalid direction keyword", "name ASCENDING", "invalid order direction"},
		{"special characters in field", "name$bad DESC", "invalid order by field identifier"},
		{"backtick injection", "name` DESC--", "invalid order by field identifier"},
		{"parenthesis injection", "name) UNION SELECT", "invalid order_by clause"},
	}

	for _, tc := range invalidCases {
		t.Run("invalid/"+tc.name, func(t *testing.T) {
			_, err := sanitizeOrderBy(tc.orderBy)
			if err == nil {
				t.Fatalf("expected error for OrderBy %q, got nil", tc.orderBy)
			}
			if !strings.Contains(err.Error(), tc.wantMsg) {
				t.Errorf("OrderBy %q: expected error containing %q, got %q", tc.orderBy, tc.wantMsg, err.Error())
			}
		})
	}
}
