// Package paymentterms provides Go equivalents of ERPNext payment term
// date-calculation helpers originally found in
// erpnext/controllers/accounts_controller.py (get_due_date,
// get_discount_date, get_payment_term_details).
package paymentterms

import (
	"time"

	"github.com/alex-baker-cdm/erpnext/go-erpnext/pkg/frappe"
)

// PaymentTerm mirrors the fields of an ERPNext "Payment Term" DocType row
// that are relevant for date and amount calculations.
type PaymentTerm struct {
	PaymentTerm             string
	Description             string
	DueDateBasedOn          string  // "Day(s) after invoice date", "Day(s) after the end of the invoice month", "Month(s) after the end of the invoice month"
	CreditDays              int
	CreditMonths            int
	DiscountValidityBasedOn string  // same options as DueDateBasedOn
	DiscountValidity        int
	InvoicePortion          float64
	DiscountType            string
	Discount                float64
	ModeOfPayment           string
}

// PaymentTermDetails holds the computed output for a single payment term,
// including copied descriptor fields, calculated amounts, and resolved dates.
type PaymentTermDetails struct {
	PaymentTerm             string
	Description             string
	InvoicePortion          float64
	DiscountType            string
	Discount                float64
	ModeOfPayment           string
	DueDateBasedOn          string
	CreditDays              int
	CreditMonths            int
	DiscountValidityBasedOn string
	DiscountValidity        int
	PaymentAmount           float64
	BasePaymentAmount       float64
	Outstanding             float64
	BaseOutstanding         float64
	DueDate                 time.Time
	DiscountDate            time.Time
}

// GetDueDate calculates the due date for a payment term.
// billDate is preferred over postingDate when non-zero.
// Returns the zero time if no matching rule is found or both dates are zero.
func GetDueDate(term PaymentTerm, postingDate, billDate time.Time) time.Time {
	date := pickDate(billDate, postingDate)
	if date.IsZero() {
		return time.Time{}
	}

	switch term.DueDateBasedOn {
	case "Day(s) after invoice date":
		return frappe.AddDays(date, term.CreditDays)
	case "Day(s) after the end of the invoice month":
		return frappe.AddDays(frappe.GetLastDay(date), term.CreditDays)
	case "Month(s) after the end of the invoice month":
		return frappe.GetLastDay(frappe.AddMonths(date, term.CreditMonths))
	default:
		return time.Time{}
	}
}

// GetDiscountDate calculates the discount validity date for a payment term.
// billDate is preferred over postingDate when non-zero.
// Returns the zero time if no matching rule is found or both dates are zero.
func GetDiscountDate(term PaymentTerm, postingDate, billDate time.Time) time.Time {
	date := pickDate(billDate, postingDate)
	if date.IsZero() {
		return time.Time{}
	}

	switch term.DiscountValidityBasedOn {
	case "Day(s) after invoice date":
		return frappe.AddDays(date, term.DiscountValidity)
	case "Day(s) after the end of the invoice month":
		return frappe.AddDays(frappe.GetLastDay(date), term.DiscountValidity)
	case "Month(s) after the end of the invoice month":
		return frappe.GetLastDay(frappe.AddMonths(date, term.DiscountValidity))
	default:
		return time.Time{}
	}
}

// GetPaymentTermDetails computes the full payment term details including
// amounts and dates, mirroring the Python get_payment_term_details function.
// billDate is preferred over postingDate for date calculations when non-zero.
// If the computed due date is before postingDate, it is clamped to postingDate.
func GetPaymentTermDetails(term PaymentTerm, postingDate time.Time, grandTotal, baseGrandTotal float64, billDate time.Time) PaymentTermDetails {
	details := PaymentTermDetails{
		PaymentTerm:             term.PaymentTerm,
		Description:             term.Description,
		InvoicePortion:          term.InvoicePortion,
		DiscountType:            term.DiscountType,
		Discount:                term.Discount,
		ModeOfPayment:           term.ModeOfPayment,
		DueDateBasedOn:          term.DueDateBasedOn,
		CreditDays:              term.CreditDays,
		CreditMonths:            term.CreditMonths,
		DiscountValidityBasedOn: term.DiscountValidityBasedOn,
		DiscountValidity:        term.DiscountValidity,
	}

	details.PaymentAmount = frappe.Flt(term.InvoicePortion, -1) * frappe.Flt(grandTotal, -1) / 100
	details.BasePaymentAmount = frappe.Flt(term.InvoicePortion, -1) * frappe.Flt(baseGrandTotal, -1) / 100
	details.Outstanding = details.PaymentAmount
	details.BaseOutstanding = details.BasePaymentAmount

	if !billDate.IsZero() {
		details.DueDate = GetDueDate(term, billDate, time.Time{})
		details.DiscountDate = GetDiscountDate(term, billDate, time.Time{})
	} else if !postingDate.IsZero() {
		details.DueDate = GetDueDate(term, postingDate, time.Time{})
		details.DiscountDate = GetDiscountDate(term, postingDate, time.Time{})
	}

	// Clamp: if due date is before posting date, set due date to posting date.
	if !postingDate.IsZero() && !details.DueDate.IsZero() && details.DueDate.Before(postingDate) {
		details.DueDate = postingDate
	}

	return details
}

// pickDate returns billDate if non-zero, otherwise postingDate.
func pickDate(billDate, postingDate time.Time) time.Time {
	if !billDate.IsZero() {
		return billDate
	}
	return postingDate
}
