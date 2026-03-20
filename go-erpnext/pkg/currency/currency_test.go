package currency

import (
	"math"
	"testing"
)

const tolerance = 1e-4

func almostEqual(a, b, tol float64) bool {
	return math.Abs(a-b) < tol
}

func TestGetPeggedRate_Case1_SameBase(t *testing.T) {
	pm := PeggedMap{
		"AED": {PeggedAgainst: "USD", Ratio: 3.67},
		"SAR": {PeggedAgainst: "USD", Ratio: 3.75},
	}
	result := GetPeggedRate(pm, "AED", "SAR", nil)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	expected := (1 / 3.67) * 3.75
	if !almostEqual(*result, expected, tolerance) {
		t.Errorf("expected ~%.4f, got %.4f", expected, *result)
	}
}

func TestGetPeggedRate_Case2_DifferentBases(t *testing.T) {
	pm := PeggedMap{
		"AED": {PeggedAgainst: "USD", Ratio: 3.67},
		"HKD": {PeggedAgainst: "EUR", Ratio: 8.5},
	}
	baseRate := 0.85
	fn := func(from, to string) *float64 {
		if from == "USD" && to == "EUR" {
			return &baseRate
		}
		return nil
	}
	result := GetPeggedRate(pm, "AED", "HKD", fn)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	expected := (1 / 3.67) * 0.85 * 8.5
	if !almostEqual(*result, expected, tolerance) {
		t.Errorf("expected ~%.4f, got %.4f", expected, *result)
	}
}

func TestGetPeggedRate_Case2_BaseRateNotFound(t *testing.T) {
	pm := PeggedMap{
		"AED": {PeggedAgainst: "USD", Ratio: 3.67},
		"HKD": {PeggedAgainst: "EUR", Ratio: 8.5},
	}
	fn := func(from, to string) *float64 {
		return nil
	}
	result := GetPeggedRate(pm, "AED", "HKD", fn)
	if result != nil {
		t.Errorf("expected nil, got %f", *result)
	}
}

func TestGetPeggedRate_Case3_FromPeggedToTo(t *testing.T) {
	pm := PeggedMap{
		"AED": {PeggedAgainst: "USD", Ratio: 3.67},
	}
	result := GetPeggedRate(pm, "AED", "USD", nil)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if !almostEqual(*result, 3.67, tolerance) {
		t.Errorf("expected 3.67, got %.4f", *result)
	}
}

func TestGetPeggedRate_Case4_ToPeggedToFrom(t *testing.T) {
	pm := PeggedMap{
		"AED": {PeggedAgainst: "USD", Ratio: 3.67},
	}
	result := GetPeggedRate(pm, "USD", "AED", nil)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	expected := 1 / 3.67
	if !almostEqual(*result, expected, tolerance) {
		t.Errorf("expected ~%.4f, got %.4f", expected, *result)
	}
}

func TestGetPeggedRate_NotFound_NeitherInMap(t *testing.T) {
	pm := PeggedMap{}
	result := GetPeggedRate(pm, "GBP", "JPY", nil)
	if result != nil {
		t.Errorf("expected nil, got %f", *result)
	}
}

func TestGetPeggedRate_OnlyOneEntry_NoMatch(t *testing.T) {
	pm := PeggedMap{
		"AED": {PeggedAgainst: "USD", Ratio: 3.67},
	}
	result := GetPeggedRate(pm, "AED", "EUR", nil)
	if result != nil {
		t.Errorf("expected nil, got %f", *result)
	}
}

func TestGetPeggedRate_ZeroRatio_FromEntry_SameBase(t *testing.T) {
	pm := PeggedMap{
		"AED": {PeggedAgainst: "USD", Ratio: 0},
		"SAR": {PeggedAgainst: "USD", Ratio: 3.75},
	}
	result := GetPeggedRate(pm, "AED", "SAR", nil)
	if result != nil {
		t.Errorf("expected nil when fromEntry.Ratio is 0, got %f", *result)
	}
}

func TestGetPeggedRate_ZeroRatio_ToEntry_Case4(t *testing.T) {
	pm := PeggedMap{
		"AED": {PeggedAgainst: "USD", Ratio: 0},
	}
	result := GetPeggedRate(pm, "USD", "AED", nil)
	if result != nil {
		t.Errorf("expected nil when toEntry.Ratio is 0, got %f", *result)
	}
}

func TestGetPeggedRate_ZeroRatio_BothZero(t *testing.T) {
	pm := PeggedMap{
		"AED": {PeggedAgainst: "USD", Ratio: 0},
		"SAR": {PeggedAgainst: "USD", Ratio: 0},
	}
	result := GetPeggedRate(pm, "AED", "SAR", nil)
	if result != nil {
		t.Errorf("expected nil when both ratios are 0, got %f", *result)
	}
}

func TestGetPeggedRate_ZeroRatio_FromEntry_DifferentBases(t *testing.T) {
	pm := PeggedMap{
		"AED": {PeggedAgainst: "USD", Ratio: 0},
		"HKD": {PeggedAgainst: "EUR", Ratio: 8.5},
	}
	baseRate := 0.85
	fn := func(from, to string) *float64 {
		if from == "USD" && to == "EUR" {
			return &baseRate
		}
		return nil
	}
	result := GetPeggedRate(pm, "AED", "HKD", fn)
	if result != nil {
		t.Errorf("expected nil when fromEntry.Ratio is 0 (different bases), got %f", *result)
	}
}

func TestGetPeggedRate_EdgeCase_RatioOne(t *testing.T) {
	pm := PeggedMap{
		"BSD": {PeggedAgainst: "USD", Ratio: 1.0},
	}
	// Case 3: from pegged to to
	result := GetPeggedRate(pm, "BSD", "USD", nil)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if !almostEqual(*result, 1.0, tolerance) {
		t.Errorf("expected 1.0, got %.4f", *result)
	}

	// Case 4: to pegged to from
	result = GetPeggedRate(pm, "USD", "BSD", nil)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if !almostEqual(*result, 1.0, tolerance) {
		t.Errorf("expected 1.0, got %.4f", *result)
	}
}
