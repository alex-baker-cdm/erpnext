package paymentterms

import (
	"testing"
	"time"
)

// helper to build a date quickly.
func d(year int, month time.Month, day int) time.Time {
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

// --- GetDueDate tests ---

func TestGetDueDate_DaysAfterInvoiceDate(t *testing.T) {
	term := PaymentTerm{
		DueDateBasedOn: "Day(s) after invoice date",
		CreditDays:     30,
	}
	got := GetDueDate(term, d(2024, time.January, 15), time.Time{})
	want := d(2024, time.February, 14)
	if !got.Equal(want) {
		t.Errorf("GetDueDate (days after invoice) = %v, want %v", got, want)
	}
}

func TestGetDueDate_DaysAfterEndOfInvoiceMonth(t *testing.T) {
	term := PaymentTerm{
		DueDateBasedOn: "Day(s) after the end of the invoice month",
		CreditDays:     15,
	}
	got := GetDueDate(term, d(2024, time.January, 15), time.Time{})
	want := d(2024, time.February, 15)
	if !got.Equal(want) {
		t.Errorf("GetDueDate (days after end of month) = %v, want %v", got, want)
	}
}

func TestGetDueDate_MonthsAfterEndOfInvoiceMonth(t *testing.T) {
	term := PaymentTerm{
		DueDateBasedOn: "Month(s) after the end of the invoice month",
		CreditMonths:   2,
	}
	got := GetDueDate(term, d(2024, time.January, 15), time.Time{})
	want := d(2024, time.March, 31)
	if !got.Equal(want) {
		t.Errorf("GetDueDate (months after end of month) = %v, want %v", got, want)
	}
}

func TestGetDueDate_BillDateTakesPrecedence(t *testing.T) {
	term := PaymentTerm{
		DueDateBasedOn: "Day(s) after invoice date",
		CreditDays:     30,
	}
	got := GetDueDate(term, d(2024, time.January, 15), d(2024, time.January, 1))
	want := d(2024, time.January, 31)
	if !got.Equal(want) {
		t.Errorf("GetDueDate (bill date precedence) = %v, want %v", got, want)
	}
}

func TestGetDueDate_ZeroTime(t *testing.T) {
	term := PaymentTerm{
		DueDateBasedOn: "Day(s) after invoice date",
		CreditDays:     30,
	}
	got := GetDueDate(term, time.Time{}, time.Time{})
	if !got.IsZero() {
		t.Errorf("GetDueDate (zero time) = %v, want zero time", got)
	}
}

// --- GetDiscountDate tests ---

func TestGetDiscountDate_DaysAfterInvoiceDate(t *testing.T) {
	term := PaymentTerm{
		DiscountValidityBasedOn: "Day(s) after invoice date",
		DiscountValidity:        10,
	}
	got := GetDiscountDate(term, d(2024, time.January, 15), time.Time{})
	want := d(2024, time.January, 25)
	if !got.Equal(want) {
		t.Errorf("GetDiscountDate (days after invoice) = %v, want %v", got, want)
	}
}

func TestGetDiscountDate_DaysAfterEndOfInvoiceMonth(t *testing.T) {
	term := PaymentTerm{
		DiscountValidityBasedOn: "Day(s) after the end of the invoice month",
		DiscountValidity:        5,
	}
	got := GetDiscountDate(term, d(2024, time.January, 15), time.Time{})
	want := d(2024, time.February, 5)
	if !got.Equal(want) {
		t.Errorf("GetDiscountDate (days after end of month) = %v, want %v", got, want)
	}
}

func TestGetDiscountDate_MonthsAfterEndOfInvoiceMonth(t *testing.T) {
	term := PaymentTerm{
		DiscountValidityBasedOn: "Month(s) after the end of the invoice month",
		DiscountValidity:        1,
	}
	got := GetDiscountDate(term, d(2024, time.January, 15), time.Time{})
	want := d(2024, time.February, 29) // 2024 is a leap year
	if !got.Equal(want) {
		t.Errorf("GetDiscountDate (months after end of month) = %v, want %v", got, want)
	}
}

func TestGetDiscountDate_ZeroTime(t *testing.T) {
	term := PaymentTerm{
		DiscountValidityBasedOn: "Day(s) after invoice date",
		DiscountValidity:        10,
	}
	got := GetDiscountDate(term, time.Time{}, time.Time{})
	if !got.IsZero() {
		t.Errorf("GetDiscountDate (zero time) = %v, want zero time", got)
	}
}

// --- GetPaymentTermDetails tests ---

func TestGetPaymentTermDetails_AmountCalculation(t *testing.T) {
	term := PaymentTerm{
		PaymentTerm:    "50% upfront",
		Description:    "Half payment",
		InvoicePortion: 50,
		DueDateBasedOn: "Day(s) after invoice date",
		CreditDays:     30,
		DiscountType:   "Percentage",
		Discount:       2,
		ModeOfPayment:  "Cash",
	}
	details := GetPaymentTermDetails(term, d(2024, time.January, 15), 1000, 1200, time.Time{})

	if details.PaymentAmount != 500 {
		t.Errorf("PaymentAmount = %v, want 500", details.PaymentAmount)
	}
	if details.BasePaymentAmount != 600 {
		t.Errorf("BasePaymentAmount = %v, want 600", details.BasePaymentAmount)
	}
	if details.Outstanding != 500 {
		t.Errorf("Outstanding = %v, want 500", details.Outstanding)
	}
	if details.BaseOutstanding != 600 {
		t.Errorf("BaseOutstanding = %v, want 600", details.BaseOutstanding)
	}
	if details.PaymentTerm != "50% upfront" {
		t.Errorf("PaymentTerm = %v, want '50%% upfront'", details.PaymentTerm)
	}
	if details.DiscountType != "Percentage" {
		t.Errorf("DiscountType = %v, want 'Percentage'", details.DiscountType)
	}
	if details.Discount != 2 {
		t.Errorf("Discount = %v, want 2", details.Discount)
	}
}

func TestGetPaymentTermDetails_DueDateClamping(t *testing.T) {
	// Use a bill date far in the past so computed due date < posting date.
	term := PaymentTerm{
		DueDateBasedOn: "Day(s) after invoice date",
		CreditDays:     5,
		InvoicePortion: 100,
	}
	postingDate := d(2024, time.March, 1)
	billDate := d(2024, time.February, 1)
	// Computed due date = 2024-02-01 + 5 = 2024-02-06 which < 2024-03-01
	details := GetPaymentTermDetails(term, postingDate, 1000, 1000, billDate)

	if !details.DueDate.Equal(postingDate) {
		t.Errorf("DueDate (clamped) = %v, want %v", details.DueDate, postingDate)
	}
}

func TestGetPaymentTermDetails_ZeroTime(t *testing.T) {
	term := PaymentTerm{
		DueDateBasedOn:          "Day(s) after invoice date",
		CreditDays:              30,
		DiscountValidityBasedOn: "Day(s) after invoice date",
		DiscountValidity:        10,
		InvoicePortion:          100,
	}
	details := GetPaymentTermDetails(term, time.Time{}, 1000, 1000, time.Time{})

	if !details.DueDate.IsZero() {
		t.Errorf("DueDate (zero time) = %v, want zero time", details.DueDate)
	}
	if !details.DiscountDate.IsZero() {
		t.Errorf("DiscountDate (zero time) = %v, want zero time", details.DiscountDate)
	}
}
