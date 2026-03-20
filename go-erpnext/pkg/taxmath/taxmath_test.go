package taxmath

import (
	"math"
	"testing"
)

const epsilon = 1e-9

func almostEqual(a, b float64) bool {
	return math.Abs(a-b) < epsilon
}

// ---------------------------------------------------------------------------
// GetTaxRate
// ---------------------------------------------------------------------------

func TestGetTaxRate_CustomRateInMap(t *testing.T) {
	itemTaxMap := map[string]float64{"VAT - TC": 12.5}
	got := GetTaxRate("VAT - TC", itemTaxMap, 18.0, 2)
	if !almostEqual(got, 12.5) {
		t.Errorf("expected 12.5, got %v", got)
	}
}

func TestGetTaxRate_NotInMap(t *testing.T) {
	itemTaxMap := map[string]float64{"VAT - TC": 12.5}
	got := GetTaxRate("CGST - TC", itemTaxMap, 18.0, 2)
	if !almostEqual(got, 18.0) {
		t.Errorf("expected 18.0, got %v", got)
	}
}

func TestGetTaxRate_EmptyMap(t *testing.T) {
	got := GetTaxRate("VAT - TC", map[string]float64{}, 9.0, 2)
	if !almostEqual(got, 9.0) {
		t.Errorf("expected 9.0, got %v", got)
	}
}

func TestGetTaxRate_PrecisionRounding(t *testing.T) {
	itemTaxMap := map[string]float64{"VAT - TC": 12.345}
	got := GetTaxRate("VAT - TC", itemTaxMap, 0, 2)
	if !almostEqual(got, 12.35) {
		t.Errorf("expected 12.35, got %v", got)
	}
}

// ---------------------------------------------------------------------------
// GetCurrentTaxFraction
// ---------------------------------------------------------------------------

func TestGetCurrentTaxFraction_OnNetTotal(t *testing.T) {
	frac, amt := GetCurrentTaxFraction("On Net Total", 10.0, true, "", 0, 0)
	if !almostEqual(frac, 0.1) {
		t.Errorf("expected fraction 0.1, got %v", frac)
	}
	if !almostEqual(amt, 0.0) {
		t.Errorf("expected inclusiveTaxAmountPerQty 0, got %v", amt)
	}
}

func TestGetCurrentTaxFraction_OnPreviousRowAmount(t *testing.T) {
	frac, amt := GetCurrentTaxFraction("On Previous Row Amount", 5.0, true, "", 0.1, 0)
	if !almostEqual(frac, 0.005) {
		t.Errorf("expected fraction 0.005, got %v", frac)
	}
	if !almostEqual(amt, 0.0) {
		t.Errorf("expected inclusiveTaxAmountPerQty 0, got %v", amt)
	}
}

func TestGetCurrentTaxFraction_OnPreviousRowTotal(t *testing.T) {
	frac, amt := GetCurrentTaxFraction("On Previous Row Total", 5.0, true, "", 0, 1.1)
	if !almostEqual(frac, 0.055) {
		t.Errorf("expected fraction 0.055, got %v", frac)
	}
	if !almostEqual(amt, 0.0) {
		t.Errorf("expected inclusiveTaxAmountPerQty 0, got %v", amt)
	}
}

func TestGetCurrentTaxFraction_OnItemQuantity(t *testing.T) {
	frac, amt := GetCurrentTaxFraction("On Item Quantity", 50.0, true, "", 0, 0)
	if !almostEqual(frac, 0.0) {
		t.Errorf("expected fraction 0, got %v", frac)
	}
	if !almostEqual(amt, 50.0) {
		t.Errorf("expected inclusiveTaxAmountPerQty 50, got %v", amt)
	}
}

func TestGetCurrentTaxFraction_Deduct(t *testing.T) {
	frac, amt := GetCurrentTaxFraction("On Net Total", 10.0, true, "Deduct", 0, 0)
	if !almostEqual(frac, -0.1) {
		t.Errorf("expected fraction -0.1, got %v", frac)
	}
	if !almostEqual(amt, 0.0) {
		t.Errorf("expected inclusiveTaxAmountPerQty 0, got %v", amt)
	}
}

func TestGetCurrentTaxFraction_DeductOnItemQuantity(t *testing.T) {
	frac, amt := GetCurrentTaxFraction("On Item Quantity", 50.0, true, "Deduct", 0, 0)
	if !almostEqual(frac, 0.0) {
		t.Errorf("expected fraction 0, got %v", frac)
	}
	if !almostEqual(amt, -50.0) {
		t.Errorf("expected inclusiveTaxAmountPerQty -50, got %v", amt)
	}
}

func TestGetCurrentTaxFraction_NotIncludedInPrintRate(t *testing.T) {
	frac, amt := GetCurrentTaxFraction("On Net Total", 10.0, false, "", 0, 0)
	if !almostEqual(frac, 0.0) {
		t.Errorf("expected fraction 0, got %v", frac)
	}
	if !almostEqual(amt, 0.0) {
		t.Errorf("expected inclusiveTaxAmountPerQty 0, got %v", amt)
	}
}

// ---------------------------------------------------------------------------
// GetCurrentTaxAndNetAmount
// ---------------------------------------------------------------------------

func TestGetCurrentTaxAndNetAmount_Actual(t *testing.T) {
	net, tax := GetCurrentTaxAndNetAmount("Actual", 0, 500, 0, 0, 0, 100, 1000, false)
	if !almostEqual(tax, 50.0) {
		t.Errorf("expected tax 50, got %v", tax)
	}
	if !almostEqual(net, 500.0) {
		t.Errorf("expected net 500, got %v", net)
	}
}

func TestGetCurrentTaxAndNetAmount_ActualZeroDocNet(t *testing.T) {
	net, tax := GetCurrentTaxAndNetAmount("Actual", 0, 500, 0, 0, 0, 100, 0, false)
	if !almostEqual(tax, 0.0) {
		t.Errorf("expected tax 0, got %v", tax)
	}
	if !almostEqual(net, 500.0) {
		t.Errorf("expected net 500, got %v", net)
	}
}

func TestGetCurrentTaxAndNetAmount_OnNetTotal(t *testing.T) {
	net, tax := GetCurrentTaxAndNetAmount("On Net Total", 10, 500, 0, 0, 0, 0, 0, true)
	if !almostEqual(tax, 50.0) {
		t.Errorf("expected tax 50, got %v", tax)
	}
	if !almostEqual(net, 500.0) {
		t.Errorf("expected net 500, got %v", net)
	}
}

func TestGetCurrentTaxAndNetAmount_OnNetTotal_NotInMap(t *testing.T) {
	net, tax := GetCurrentTaxAndNetAmount("On Net Total", 10, 500, 0, 0, 0, 0, 0, false)
	if !almostEqual(tax, 50.0) {
		t.Errorf("expected tax 50, got %v", tax)
	}
	if !almostEqual(net, 0.0) {
		t.Errorf("expected net 0, got %v", net)
	}
}

func TestGetCurrentTaxAndNetAmount_OnPreviousRowAmount(t *testing.T) {
	net, tax := GetCurrentTaxAndNetAmount("On Previous Row Amount", 5, 0, 0, 100, 0, 0, 0, false)
	if !almostEqual(tax, 5.0) {
		t.Errorf("expected tax 5, got %v", tax)
	}
	if !almostEqual(net, 100.0) {
		t.Errorf("expected net 100, got %v", net)
	}
}

func TestGetCurrentTaxAndNetAmount_OnPreviousRowTotal(t *testing.T) {
	net, tax := GetCurrentTaxAndNetAmount("On Previous Row Total", 5, 0, 0, 0, 1100, 0, 0, false)
	if !almostEqual(tax, 55.0) {
		t.Errorf("expected tax 55, got %v", tax)
	}
	if !almostEqual(net, 1100.0) {
		t.Errorf("expected net 1100, got %v", net)
	}
}

func TestGetCurrentTaxAndNetAmount_OnItemQuantity(t *testing.T) {
	net, tax := GetCurrentTaxAndNetAmount("On Item Quantity", 10, 0, 5, 0, 0, 0, 0, false)
	if !almostEqual(tax, 50.0) {
		t.Errorf("expected tax 50, got %v", tax)
	}
	if !almostEqual(net, 0.0) {
		t.Errorf("expected net 0, got %v", net)
	}
}

// ---------------------------------------------------------------------------
// SetInCompanyCurrency
// ---------------------------------------------------------------------------

func TestSetInCompanyCurrency_Basic(t *testing.T) {
	got := SetInCompanyCurrency(100.0, 1.5, 2)
	if !almostEqual(got, 150.0) {
		t.Errorf("expected 150.00, got %v", got)
	}
}

func TestSetInCompanyCurrency_Rounding(t *testing.T) {
	got := SetInCompanyCurrency(33.333, 1.0, 2)
	if !almostEqual(got, 33.33) {
		t.Errorf("expected 33.33, got %v", got)
	}
}

func TestSetInCompanyCurrency_ZeroRate(t *testing.T) {
	got := SetInCompanyCurrency(100.0, 0.0, 2)
	if !almostEqual(got, 0.0) {
		t.Errorf("expected 0, got %v", got)
	}
}

// ---------------------------------------------------------------------------
// CalculateItemValues
// ---------------------------------------------------------------------------

func TestCalculateItemValues_SimpleDiscount(t *testing.T) {
	iv := CalculateItemValues(100, 10, 0, 0, "", 5, 1.0, 2, 2, 2, 2, 2)
	if !almostEqual(iv.Rate, 90.0) {
		t.Errorf("expected rate 90, got %v", iv.Rate)
	}
	if !almostEqual(iv.Amount, 450.0) {
		t.Errorf("expected amount 450, got %v", iv.Amount)
	}
	if !almostEqual(iv.NetRate, 90.0) {
		t.Errorf("expected net_rate 90, got %v", iv.NetRate)
	}
	if !almostEqual(iv.NetAmount, 450.0) {
		t.Errorf("expected net_amount 450, got %v", iv.NetAmount)
	}
}

func TestCalculateItemValues_100PercentDiscount(t *testing.T) {
	iv := CalculateItemValues(100, 100, 0, 0, "", 5, 1.0, 2, 2, 2, 2, 2)
	if !almostEqual(iv.Rate, 0.0) {
		t.Errorf("expected rate 0, got %v", iv.Rate)
	}
	if !almostEqual(iv.Amount, 0.0) {
		t.Errorf("expected amount 0, got %v", iv.Amount)
	}
}

func TestCalculateItemValues_DiscountAmount(t *testing.T) {
	iv := CalculateItemValues(100, 0, 15, 0, "", 1, 1.0, 2, 2, 2, 2, 2)
	if !almostEqual(iv.Rate, 85.0) {
		t.Errorf("expected rate 85, got %v", iv.Rate)
	}
	if !almostEqual(iv.DiscountAmount, 15.0) {
		t.Errorf("expected discount_amount 15, got %v", iv.DiscountAmount)
	}
}

func TestCalculateItemValues_MarginPercentage(t *testing.T) {
	iv := CalculateItemValues(100, 0, 0, 10, "Percentage", 1, 1.0, 2, 2, 2, 2, 2)
	if !almostEqual(iv.RateWithMargin, 110.0) {
		t.Errorf("expected rate_with_margin 110, got %v", iv.RateWithMargin)
	}
	if !almostEqual(iv.Rate, 110.0) {
		t.Errorf("expected rate 110, got %v", iv.Rate)
	}
}

func TestCalculateItemValues_MarginAmount(t *testing.T) {
	iv := CalculateItemValues(100, 0, 0, 20, "Amount", 1, 1.0, 2, 2, 2, 2, 2)
	if !almostEqual(iv.RateWithMargin, 120.0) {
		t.Errorf("expected rate_with_margin 120, got %v", iv.RateWithMargin)
	}
	if !almostEqual(iv.Rate, 120.0) {
		t.Errorf("expected rate 120, got %v", iv.Rate)
	}
}

func TestCalculateItemValues_CurrencyConversion(t *testing.T) {
	iv := CalculateItemValues(100, 10, 0, 0, "", 5, 1.5, 2, 2, 2, 2, 2)
	if !almostEqual(iv.Rate, 90.0) {
		t.Errorf("expected rate 90, got %v", iv.Rate)
	}
	if !almostEqual(iv.BaseRate, 135.0) {
		t.Errorf("expected base_rate 135, got %v", iv.BaseRate)
	}
	if !almostEqual(iv.BaseAmount, 675.0) {
		t.Errorf("expected base_amount 675, got %v", iv.BaseAmount)
	}
	if !almostEqual(iv.BasePriceListRate, 150.0) {
		t.Errorf("expected base_price_list_rate 150, got %v", iv.BasePriceListRate)
	}
	if !almostEqual(iv.BaseNetRate, 135.0) {
		t.Errorf("expected base_net_rate 135, got %v", iv.BaseNetRate)
	}
	if !almostEqual(iv.BaseNetAmount, 675.0) {
		t.Errorf("expected base_net_amount 675, got %v", iv.BaseNetAmount)
	}
}

func TestCalculateItemValues_MarginWithDiscount(t *testing.T) {
	// Margin of 10% on priceListRate=100 -> rateWithMargin=110
	// Then 10% discount on 110 -> rate=99
	iv := CalculateItemValues(100, 10, 0, 10, "Percentage", 2, 1.0, 2, 2, 2, 2, 2)
	if !almostEqual(iv.RateWithMargin, 110.0) {
		t.Errorf("expected rate_with_margin 110, got %v", iv.RateWithMargin)
	}
	if !almostEqual(iv.Rate, 99.0) {
		t.Errorf("expected rate 99, got %v", iv.Rate)
	}
	if !almostEqual(iv.Amount, 198.0) {
		t.Errorf("expected amount 198, got %v", iv.Amount)
	}
}
