package currency

// PeggedEntry represents a currency's peg relationship.
type PeggedEntry struct {
	PeggedAgainst string
	Ratio         float64
}

// PeggedMap maps currency codes to their peg entries.
type PeggedMap map[string]PeggedEntry

// BaseRateFn is a callback to look up exchange rates between base currencies.
// Used when two currencies are pegged to different bases (Case 2).
type BaseRateFn func(fromCurrency, toCurrency string) *float64

// GetPeggedRate calculates the exchange rate between two currencies using peg relationships.
// Returns nil if the rate cannot be determined from the peg map.
//
// Handles 4 cases:
//  1. Both currencies pegged to same base: rate = (1/from_ratio) * to_ratio
//  2. Both pegged to different bases: rate = (1/from_ratio) * base_rate * to_ratio
//  3. from_currency is pegged to to_currency: rate = from_ratio
//  4. to_currency is pegged to from_currency: rate = 1/to_ratio
func GetPeggedRate(peggedMap PeggedMap, fromCurrency, toCurrency string, baseRateFn BaseRateFn) *float64 {
	fromEntry, fromOk := peggedMap[fromCurrency]
	toEntry, toOk := peggedMap[toCurrency]

	if fromOk && toOk {
		// Case 1: Both are present and pegged to same base
		if fromEntry.PeggedAgainst == toEntry.PeggedAgainst {
			rate := (1 / fromEntry.Ratio) * toEntry.Ratio
			return &rate
		}

		// Case 2: Both are present but pegged to different bases
		if baseRateFn == nil {
			return nil
		}
		baseRate := baseRateFn(fromEntry.PeggedAgainst, toEntry.PeggedAgainst)
		if baseRate == nil {
			return nil
		}
		rate := (1 / fromEntry.Ratio) * (*baseRate) * toEntry.Ratio
		return &rate
	}

	// Case 3: from_currency is pegged to to_currency
	if fromOk && fromEntry.PeggedAgainst == toCurrency {
		rate := fromEntry.Ratio
		return &rate
	}

	// Case 4: to_currency is pegged to from_currency
	if toOk && toEntry.PeggedAgainst == fromCurrency {
		rate := 1 / toEntry.Ratio
		return &rate
	}

	return nil
}
