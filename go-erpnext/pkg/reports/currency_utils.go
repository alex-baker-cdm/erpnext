package reports

import (
	"sync"

	"github.com/alex-baker-cdm/erpnext/go-erpnext/pkg/frappe"
)

// CurrencyMap holds currency information for report rendering.
type CurrencyMap struct {
	Company                string `json:"company"`
	CompanyCurrency        string `json:"company_currency"`
	PresentationCurrency   string `json:"presentation_currency"`
	ReportDate             string `json:"report_date"`
}

// exchangeRateCache memoizes exchange rates to avoid repeated lookups.
var exchangeRateCache = struct {
	sync.RWMutex
	rates map[string]float64
}{rates: make(map[string]float64)}

// GetCurrency returns currency information for the given report filters.
// It determines the company, company currency, presentation currency, and report date.
func GetCurrency(filters map[string]interface{}) CurrencyMap {
	company, _ := filters["company"].(string)
	companyCurrency, _ := filters["company_currency"].(string)

	presentationCurrency := companyCurrency
	if pc, ok := filters["presentation_currency"].(string); ok && pc != "" {
		presentationCurrency = pc
	}

	reportDate := ""
	if rd, ok := filters["to_date"].(string); ok && rd != "" {
		reportDate = rd
	} else if rd, ok := filters["period_end_date"].(string); ok && rd != "" {
		reportDate = rd
	}

	return CurrencyMap{
		Company:              company,
		CompanyCurrency:      companyCurrency,
		PresentationCurrency: presentationCurrency,
		ReportDate:           reportDate,
	}
}

// ConvertToPresentationCurrency converts GL entry debit/credit values from
// company currency to presentation currency using exchange rates.
func ConvertToPresentationCurrency(glEntries []GLEntry, currencyInfo CurrencyMap) []GLEntry {
	presentationCurrency := currencyInfo.PresentationCurrency
	companyCurrency := currencyInfo.CompanyCurrency
	reportDate := currencyInfo.ReportDate

	// Collect unique account currencies
	accountCurrencies := make(map[string]bool)
	for _, entry := range glEntries {
		accountCurrencies[entry.AccountCurrency] = true
	}

	converted := make([]GLEntry, len(glEntries))
	for i, entry := range glEntries {
		converted[i] = entry

		if len(accountCurrencies) == 1 && entry.AccountCurrency == presentationCurrency {
			// If single account currency matches presentation currency, use account currency values
			converted[i].Debit = entry.DebitInAccountCurrency
			converted[i].Credit = entry.CreditInAccountCurrency
		} else {
			// Convert using exchange rates
			if entry.Debit != 0 {
				converted[i].Debit = Convert(entry.Debit, presentationCurrency, companyCurrency, reportDate)
			}
			if entry.Credit != 0 {
				converted[i].Credit = Convert(entry.Credit, presentationCurrency, companyCurrency, reportDate)
			}
		}
	}

	return converted
}

// Convert converts a monetary value from one currency to another using the
// exchange rate at the given date. Results are memoized for performance.
func Convert(value float64, from, to, date string) float64 {
	if from == to {
		return value
	}

	rate := GetRateAsAt(date, from, to)
	if rate == 0 {
		rate = 1
	}

	return frappe.Flt(value, -1) / rate
}

// GetRateAsAt retrieves the exchange rate for the given currency pair at the
// specified date. Returns the cached rate if available, otherwise returns 1.0
// as a default (actual rate fetching requires Frappe's exchange rate API).
func GetRateAsAt(date, fromCurrency, toCurrency string) float64 {
	if fromCurrency == toCurrency {
		return 1.0
	}

	key := fromCurrency + "-" + toCurrency + "@" + date

	exchangeRateCache.RLock()
	rate, ok := exchangeRateCache.rates[key]
	exchangeRateCache.RUnlock()

	if ok {
		return rate
	}

	// TODO: Integrate with actual exchange rate source (frappe's get_exchange_rate).
	// For now, return 1.0 as default.
	rate = 1.0

	exchangeRateCache.Lock()
	exchangeRateCache.rates[key] = rate
	exchangeRateCache.Unlock()

	return rate
}

// SetExchangeRate sets an exchange rate in the cache. This is useful for testing
// and for pre-loading rates from the database.
func SetExchangeRate(fromCurrency, toCurrency, date string, rate float64) {
	key := fromCurrency + "-" + toCurrency + "@" + date
	exchangeRateCache.Lock()
	exchangeRateCache.rates[key] = rate
	exchangeRateCache.Unlock()
}

// ClearExchangeRateCache clears all cached exchange rates.
func ClearExchangeRateCache() {
	exchangeRateCache.Lock()
	exchangeRateCache.rates = make(map[string]float64)
	exchangeRateCache.Unlock()
}
