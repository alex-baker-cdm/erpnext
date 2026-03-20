package stock

import (
	"testing"
)

func TestPreviousSLEArgs_PostingDatetime(t *testing.T) {
	// Test that the args struct correctly holds the data we expect.
	args := PreviousSLEArgs{
		ItemCode:    "ITEM-001",
		Warehouse:   "Stores - WH",
		PostingDate: "2024-01-15",
		PostingTime: "14:30:00",
	}

	if args.ItemCode != "ITEM-001" {
		t.Errorf("expected ItemCode 'ITEM-001', got %q", args.ItemCode)
	}
	if args.Warehouse != "Stores - WH" {
		t.Errorf("expected Warehouse 'Stores - WH', got %q", args.Warehouse)
	}
	if args.PostingDate != "2024-01-15" {
		t.Errorf("expected PostingDate '2024-01-15', got %q", args.PostingDate)
	}
	if args.PostingTime != "14:30:00" {
		t.Errorf("expected PostingTime '14:30:00', got %q", args.PostingTime)
	}
}

func TestPreviousSLEArgs_Defaults(t *testing.T) {
	// Test that empty args have expected zero values.
	args := PreviousSLEArgs{}

	if args.PostingDate != "" {
		t.Errorf("expected empty PostingDate, got %q", args.PostingDate)
	}
	if args.PostingTime != "" {
		t.Errorf("expected empty PostingTime, got %q", args.PostingTime)
	}
	if args.ExtraCond != "" {
		t.Errorf("expected empty ExtraCond, got %q", args.ExtraCond)
	}
}
