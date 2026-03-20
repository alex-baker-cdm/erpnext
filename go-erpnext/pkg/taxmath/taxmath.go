// Package taxmath provides pure tax calculation functions ported from
// erpnext/controllers/taxes_and_totals.py.
package taxmath

import (
	"github.com/alex-baker-cdm/erpnext/go-erpnext/pkg/frappe"
)

// GetTaxRate returns the item-specific tax rate from itemTaxMap if the
// accountHead exists there (rounded to precision), otherwise returns the
// default tax rate. Mirrors _get_tax_rate (lines 364-368).
func GetTaxRate(accountHead string, itemTaxMap map[string]float64, defaultRate float64, precision int) float64 {
	if rate, ok := itemTaxMap[accountHead]; ok {
		return frappe.Flt(rate, precision)
	}
	return defaultRate
}

// GetCurrentTaxFraction computes the tax fraction and inclusive tax amount per
// quantity for a single tax row when calculating tax-exclusive amounts from
// tax-inclusive prices. Mirrors get_current_tax_fraction (lines 331-362).
func GetCurrentTaxFraction(
	chargeType string,
	taxRate float64,
	includedInPrintRate bool,
	addDeductTax string,
	prevRowTaxFraction, prevRowGrandTotalFraction float64,
) (taxFraction, inclusiveTaxAmountPerQty float64) {
	if includedInPrintRate {
		switch chargeType {
		case "On Net Total":
			taxFraction = taxRate / 100.0
		case "On Previous Row Amount":
			taxFraction = (taxRate / 100.0) * prevRowTaxFraction
		case "On Previous Row Total":
			taxFraction = (taxRate / 100.0) * prevRowGrandTotalFraction
		case "On Item Quantity":
			inclusiveTaxAmountPerQty = taxRate
		}
	}

	if addDeductTax == "Deduct" {
		taxFraction *= -1.0
		inclusiveTaxAmountPerQty *= -1.0
	}

	return taxFraction, inclusiveTaxAmountPerQty
}

// GetCurrentTaxAndNetAmount computes the current tax amount and net amount for
// a single item-tax combination. Mirrors get_current_tax_and_net_amount
// (lines 567-595).
func GetCurrentTaxAndNetAmount(
	chargeType string,
	taxRate float64,
	itemNetAmount, itemQty,
	prevRowTaxAmount, prevRowGrandTotal,
	actualTaxAmount, docNetTotal float64,
	accountHeadInItemTaxMap bool,
) (netAmount, taxAmount float64) {
	switch chargeType {
	case "Actual":
		netAmount = itemNetAmount
		if docNetTotal != 0.0 {
			taxAmount = itemNetAmount * actualTaxAmount / docNetTotal
		}
	case "On Net Total":
		if accountHeadInItemTaxMap {
			netAmount = itemNetAmount
		}
		taxAmount = (taxRate / 100.0) * itemNetAmount
	case "On Previous Row Amount":
		netAmount = prevRowTaxAmount
		taxAmount = (taxRate / 100.0) * netAmount
	case "On Previous Row Total":
		netAmount = prevRowGrandTotal
		taxAmount = (taxRate / 100.0) * netAmount
	case "On Item Quantity":
		taxAmount = taxRate * itemQty
	}
	return netAmount, taxAmount
}

// SetInCompanyCurrency converts a document field value to company currency by
// multiplying by conversionRate and rounding to precision.
// Mirrors _set_in_company_currency (lines 238-244).
func SetInCompanyCurrency(value, conversionRate float64, precision int) float64 {
	return frappe.Flt(value*conversionRate, precision)
}

// ItemValues holds the computed monetary values for a single line item.
type ItemValues struct {
	Rate               float64
	Amount             float64
	NetRate            float64
	NetAmount          float64
	DiscountAmount     float64
	BaseRate           float64
	BaseAmount         float64
	BaseNetRate        float64
	BaseNetAmount      float64
	BasePriceListRate  float64
	RateWithMargin     float64
	BaseRateWithMargin float64
}

// CalculateItemValues computes item-level monetary values (rate, amount,
// net rate/amount, base values, margin) from pricing inputs.
// Mirrors the core math of calculate_item_values (lines 162-235).
func CalculateItemValues(
	priceListRate, discountPercentage, discountAmount, marginRateOrAmount float64,
	marginType string,
	qty, conversionRate float64,
	ratePrecision, amountPrecision, discountPrecision,
	baseRatePrecision, baseAmountPrecision int,
) ItemValues {
	var iv ItemValues

	rate := priceListRate

	// Apply margin first, if present.
	var rateWithMargin float64
	if marginType != "" && marginRateOrAmount != 0 {
		var marginValue float64
		if marginType == "Amount" {
			marginValue = marginRateOrAmount
		} else if marginType == "Percentage" {
			marginValue = priceListRate * marginRateOrAmount / 100.0
		}
		rateWithMargin = frappe.Flt(priceListRate+marginValue, ratePrecision)
		rate = rateWithMargin
	}

	// Apply discount.
	if discountPercentage == 100 {
		rate = 0.0
		iv.DiscountAmount = frappe.Flt(priceListRate, discountPrecision)
		if rateWithMargin != 0 {
			iv.DiscountAmount = frappe.Flt(rateWithMargin, discountPrecision)
		}
	} else if discountPercentage > 0 {
		baseForDiscount := priceListRate
		if rateWithMargin != 0 {
			baseForDiscount = rateWithMargin
		}
		rate = frappe.Flt(baseForDiscount*(1.0-(discountPercentage/100.0)), ratePrecision)
		iv.DiscountAmount = frappe.Flt(baseForDiscount-rate, discountPrecision)
	} else if discountAmount > 0 {
		baseForDiscount := priceListRate
		if rateWithMargin != 0 {
			baseForDiscount = rateWithMargin
		}
		rate = frappe.Flt(baseForDiscount-discountAmount, ratePrecision)
		iv.DiscountAmount = frappe.Flt(discountAmount, discountPrecision)
	}

	iv.Rate = frappe.Flt(rate, ratePrecision)
	iv.Amount = frappe.Flt(rate*qty, amountPrecision)
	iv.NetRate = iv.Rate
	iv.NetAmount = iv.Amount
	iv.RateWithMargin = frappe.Flt(rateWithMargin, ratePrecision)
	iv.BaseRateWithMargin = frappe.Flt(rateWithMargin*conversionRate, baseRatePrecision)

	// Base (company currency) values.
	iv.BasePriceListRate = frappe.Flt(priceListRate*conversionRate, baseRatePrecision)
	iv.BaseRate = frappe.Flt(iv.Rate*conversionRate, baseRatePrecision)
	iv.BaseAmount = frappe.Flt(iv.Amount*conversionRate, baseAmountPrecision)
	iv.BaseNetRate = frappe.Flt(iv.NetRate*conversionRate, baseRatePrecision)
	iv.BaseNetAmount = frappe.Flt(iv.NetAmount*conversionRate, baseAmountPrecision)

	return iv
}
