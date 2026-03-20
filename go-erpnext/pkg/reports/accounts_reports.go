package reports

import (
	"fmt"
	"strings"

	"github.com/alex-baker-cdm/erpnext/go-erpnext/pkg/db"
	"github.com/alex-baker-cdm/erpnext/go-erpnext/pkg/frappe"
)

// ---------------------------------------------------------------------------
// 2E.2a  Sales Register Report
// Source: erpnext/accounts/report/sales_register/sales_register.py
// ---------------------------------------------------------------------------

// SalesRegisterReport implements the Sales Register report.
type SalesRegisterReport struct {
	DB *db.DB
}

func (r *SalesRegisterReport) Execute(filters map[string]interface{}) (*ReportResult, error) {
	invoiceList, err := r.getInvoices(filters)
	if err != nil {
		return nil, fmt.Errorf("sales register invoices: %w", err)
	}

	incomeAccounts, taxAccounts := r.getAccountColumns(invoiceList)
	columns := r.getColumns(incomeAccounts, taxAccounts)

	if len(invoiceList) == 0 {
		return &ReportResult{Columns: columns, Result: nil}, nil
	}

	invoiceIncomeMap := r.getInvoiceIncomeMap(invoiceList)
	invoiceTaxMap := r.getInvoiceTaxMap(invoiceList)
	invoiceSODNMap := r.getInvoiceSODNMap(invoiceList)
	modeOfPayments := r.getModeOfPayments(invoiceList)

	var data [][]interface{}
	for _, inv := range invoiceList {
		invName, _ := inv["name"].(string)

		salesOrders := ""
		deliveryNotes := ""
		if sodn, ok := invoiceSODNMap[invName]; ok {
			if sos, ok := sodn["sales_order"]; ok {
				salesOrders = strings.Join(sos, ", ")
			}
			if dns, ok := sodn["delivery_note"]; ok {
				deliveryNotes = strings.Join(dns, ", ")
			}
		}

		mop := ""
		if mops, ok := modeOfPayments[invName]; ok {
			mop = strings.Join(mops, ", ")
		}

		baseNetTotal := 0.0
		for _, acc := range incomeAccounts {
			if incMap, ok := invoiceIncomeMap[invName]; ok {
				baseNetTotal += frappe.FltFromAny(incMap[acc], -1)
			}
		}

		totalTax := 0.0
		for _, acc := range taxAccounts {
			if taxMap, ok := invoiceTaxMap[invName]; ok {
				totalTax += frappe.FltFromAny(taxMap[acc], -1)
			}
		}

		netTotal := baseNetTotal
		if netTotal == 0 {
			netTotal = frappe.FltFromAny(inv["base_net_total"], -1)
		}

		row := make([]interface{}, len(columns))
		colIdx := map[string]int{}
		for i, col := range columns {
			colIdx[col.Fieldname] = i
		}

		setVal := func(fieldname string, val interface{}) {
			if idx, ok := colIdx[fieldname]; ok {
				row[idx] = val
			}
		}

		setVal("voucher_type", "Sales Invoice")
		setVal("voucher_no", invName)
		setVal("posting_date", inv["posting_date"])
		setVal("customer", inv["customer"])
		setVal("customer_name", inv["customer_name"])
		setVal("customer_group", inv["customer_group"])
		setVal("territory", inv["territory"])
		setVal("tax_id", inv["tax_id"])
		setVal("receivable_account", inv["debit_to"])
		setVal("mode_of_payment", mop)
		setVal("project", inv["project"])
		setVal("owner", inv["owner"])
		setVal("sales_order", salesOrders)
		setVal("delivery_note", deliveryNotes)
		setVal("remarks", inv["remarks"])

		// Income account values
		for _, acc := range incomeAccounts {
			fieldname := scrubAccountName(acc)
			amount := 0.0
			if incMap, ok := invoiceIncomeMap[invName]; ok {
				amount = frappe.FltFromAny(incMap[acc], -1)
			}
			setVal(fieldname, amount)
		}

		setVal("net_total", netTotal)

		// Tax account values
		for _, acc := range taxAccounts {
			fieldname := scrubAccountName(acc)
			amount := 0.0
			if taxMap, ok := invoiceTaxMap[invName]; ok {
				amount = frappe.FltFromAny(taxMap[acc], -1)
			}
			setVal(fieldname, amount)
		}

		setVal("tax_total", totalTax)
		setVal("grand_total", inv["base_grand_total"])
		setVal("rounded_total", inv["base_rounded_total"])
		setVal("outstanding_amount", inv["outstanding_amount"])

		data = append(data, row)
	}

	return &ReportResult{
		Columns: columns,
		Result:  data,
	}, nil
}

func (r *SalesRegisterReport) getColumns(incomeAccounts, taxAccounts []string) []Column {
	columns := []Column{
		{Label: "Voucher Type", Fieldname: "voucher_type", Width: 120},
		{Label: "Voucher", Fieldname: "voucher_no", Fieldtype: "Dynamic Link", Options: "voucher_type", Width: 120},
		{Label: "Posting Date", Fieldname: "posting_date", Fieldtype: "Date", Width: 80},
		{Label: "Customer", Fieldname: "customer", Fieldtype: "Link", Options: "Customer", Width: 120},
		{Label: "Customer Name", Fieldname: "customer_name", Fieldtype: "Data", Width: 120},
		{Label: "Customer Group", Fieldname: "customer_group", Fieldtype: "Link", Options: "Customer Group", Width: 120},
		{Label: "Territory", Fieldname: "territory", Fieldtype: "Link", Options: "Territory", Width: 80},
		{Label: "Tax Id", Fieldname: "tax_id", Fieldtype: "Data", Width: 80},
		{Label: "Receivable Account", Fieldname: "receivable_account", Fieldtype: "Link", Options: "Account", Width: 100},
		{Label: "Mode Of Payment", Fieldname: "mode_of_payment", Fieldtype: "Data", Width: 120},
		{Label: "Project", Fieldname: "project", Fieldtype: "Link", Options: "Project", Width: 80},
		{Label: "Owner", Fieldname: "owner", Fieldtype: "Data", Width: 100},
		{Label: "Sales Order", Fieldname: "sales_order", Fieldtype: "Link", Options: "Sales Order", Width: 100},
		{Label: "Delivery Note", Fieldname: "delivery_note", Fieldtype: "Link", Options: "Delivery Note", Width: 100},
		{Label: "Cost Center", Fieldname: "cost_center", Fieldtype: "Link", Options: "Cost Center", Width: 100},
		{Label: "Warehouse", Fieldname: "warehouse", Fieldtype: "Link", Options: "Warehouse", Width: 100},
		{Label: "Currency", Fieldname: "currency", Fieldtype: "Data", Width: 80},
	}

	// Income account columns
	for _, acc := range incomeAccounts {
		columns = append(columns, Column{
			Label: acc, Fieldname: scrubAccountName(acc), Fieldtype: "Currency", Options: "currency", Width: 120,
		})
	}

	columns = append(columns, Column{
		Label: "Net Total", Fieldname: "net_total", Fieldtype: "Currency", Options: "currency", Width: 120,
	})

	// Tax account columns
	for _, acc := range taxAccounts {
		columns = append(columns, Column{
			Label: acc, Fieldname: scrubAccountName(acc), Fieldtype: "Currency", Options: "currency", Width: 120,
		})
	}

	columns = append(columns,
		Column{Label: "Tax Total", Fieldname: "tax_total", Fieldtype: "Currency", Options: "currency", Width: 120},
		Column{Label: "Grand Total", Fieldname: "grand_total", Fieldtype: "Currency", Options: "currency", Width: 120},
		Column{Label: "Rounded Total", Fieldname: "rounded_total", Fieldtype: "Currency", Options: "currency", Width: 120},
		Column{Label: "Outstanding Amount", Fieldname: "outstanding_amount", Fieldtype: "Currency", Options: "currency", Width: 120},
		Column{Label: "Remarks", Fieldname: "remarks", Fieldtype: "Data", Width: 150},
	)

	return columns
}

func (r *SalesRegisterReport) getInvoices(filters map[string]interface{}) ([]map[string]interface{}, error) {
	if r.DB == nil {
		return nil, nil
	}

	query := "SELECT 'Sales Invoice' AS `doctype`, `name`, `posting_date`, `debit_to`," +
		" `project`, `customer`, `customer_name`, `owner`, `remarks`," +
		" `territory`, `tax_id`, `customer_group`," +
		" `base_net_total`, `base_grand_total`, `base_rounded_total`," +
		" `outstanding_amount`, `is_internal_customer`, `represents_company`, `company`" +
		" FROM `tabSales Invoice` WHERE `docstatus` = 1"

	var args []interface{}

	if customer, ok := filters["customer"].(string); ok && customer != "" {
		query += " AND `customer` = ?"
		args = append(args, customer)
	}
	if company, ok := filters["company"].(string); ok && company != "" {
		query += " AND `company` = ?"
		args = append(args, company)
	}
	if fromDate, ok := filters["from_date"].(string); ok && fromDate != "" {
		query += " AND `posting_date` >= ?"
		args = append(args, fromDate)
	}
	if toDate, ok := filters["to_date"].(string); ok && toDate != "" {
		query += " AND `posting_date` <= ?"
		args = append(args, toDate)
	}
	if owner, ok := filters["owner"].(string); ok && owner != "" {
		query += " AND `owner` = ?"
		args = append(args, owner)
	}

	query += " ORDER BY `posting_date` DESC, `name` DESC"

	rows, err := r.DB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanRowsToMaps(rows)
}

func (r *SalesRegisterReport) getAccountColumns(invoiceList []map[string]interface{}) ([]string, []string) {
	if r.DB == nil || len(invoiceList) == 0 {
		return nil, nil
	}

	invNames := make([]interface{}, len(invoiceList))
	for i, inv := range invoiceList {
		invNames[i] = inv["name"]
	}

	placeholders := strings.Repeat("?,", len(invNames))
	placeholders = placeholders[:len(placeholders)-1]

	// Income accounts
	incomeQuery := "SELECT DISTINCT `income_account` FROM `tabSales Invoice Item`" +
		" WHERE `docstatus` = 1 AND `parent` IN (" + placeholders + ")" +
		" ORDER BY `income_account`"
	incomeAccounts := r.queryStringList(incomeQuery, invNames)

	// Tax accounts
	taxQuery := "SELECT DISTINCT `account_head` FROM `tabSales Taxes and Charges`" +
		" WHERE `parent` IN (" + placeholders + ")" +
		" AND `category` IN ('Total', 'Valuation and Total')" +
		" AND `base_tax_amount_after_discount_amount` != 0" +
		" ORDER BY `account_head`"
	taxAccounts := r.queryStringList(taxQuery, invNames)

	// Remove income accounts from tax accounts
	incomeSet := make(map[string]bool)
	for _, acc := range incomeAccounts {
		incomeSet[acc] = true
	}
	var filteredTax []string
	for _, acc := range taxAccounts {
		if !incomeSet[acc] {
			filteredTax = append(filteredTax, acc)
		}
	}

	return incomeAccounts, filteredTax
}

func (r *SalesRegisterReport) getInvoiceIncomeMap(invoiceList []map[string]interface{}) map[string]map[string]float64 {
	result := make(map[string]map[string]float64)
	if r.DB == nil || len(invoiceList) == 0 {
		return result
	}

	invNames := make([]interface{}, len(invoiceList))
	for i, inv := range invoiceList {
		invNames[i] = inv["name"]
	}

	placeholders := strings.Repeat("?,", len(invNames))
	placeholders = placeholders[:len(placeholders)-1]

	query := "SELECT `parent`, `income_account`, SUM(`base_net_amount`) AS `amount`" +
		" FROM `tabSales Invoice Item` WHERE `parent` IN (" + placeholders + ")" +
		" GROUP BY `parent`, `income_account`"

	rows, err := r.DB.Query(query, invNames...)
	if err != nil {
		return result
	}
	defer rows.Close()

	for rows.Next() {
		var parent, account string
		var amount float64
		if rows.Scan(&parent, &account, &amount) == nil {
			if _, ok := result[parent]; !ok {
				result[parent] = make(map[string]float64)
			}
			result[parent][account] = amount
		}
	}
	return result
}

func (r *SalesRegisterReport) getInvoiceTaxMap(invoiceList []map[string]interface{}) map[string]map[string]float64 {
	result := make(map[string]map[string]float64)
	if r.DB == nil || len(invoiceList) == 0 {
		return result
	}

	invNames := make([]interface{}, len(invoiceList))
	for i, inv := range invoiceList {
		invNames[i] = inv["name"]
	}

	placeholders := strings.Repeat("?,", len(invNames))
	placeholders = placeholders[:len(placeholders)-1]

	query := "SELECT `parent`, `account_head`," +
		" SUM(CASE WHEN `add_deduct_tax` = 'Add' THEN `base_tax_amount_after_discount_amount`" +
		" ELSE -`base_tax_amount_after_discount_amount` END) AS `tax_amount`" +
		" FROM `tabSales Taxes and Charges`" +
		" WHERE `parent` IN (" + placeholders + ")" +
		" AND `category` IN ('Total', 'Valuation and Total')" +
		" AND `base_tax_amount_after_discount_amount` != 0" +
		" GROUP BY `parent`, `account_head`, `add_deduct_tax`"

	rows, err := r.DB.Query(query, invNames...)
	if err != nil {
		return result
	}
	defer rows.Close()

	for rows.Next() {
		var parent, account string
		var amount float64
		if rows.Scan(&parent, &account, &amount) == nil {
			if _, ok := result[parent]; !ok {
				result[parent] = make(map[string]float64)
			}
			result[parent][account] += amount
		}
	}
	return result
}

func (r *SalesRegisterReport) getInvoiceSODNMap(invoiceList []map[string]interface{}) map[string]map[string][]string {
	result := make(map[string]map[string][]string)
	if r.DB == nil || len(invoiceList) == 0 {
		return result
	}

	invNames := make([]interface{}, len(invoiceList))
	for i, inv := range invoiceList {
		invNames[i] = inv["name"]
	}

	placeholders := strings.Repeat("?,", len(invNames))
	placeholders = placeholders[:len(placeholders)-1]

	query := "SELECT `parent`, `sales_order`, `delivery_note`" +
		" FROM `tabSales Invoice Item`" +
		" WHERE `parent` IN (" + placeholders + ")" +
		" AND (`sales_order` != '' OR `delivery_note` != '')"

	rows, err := r.DB.Query(query, invNames...)
	if err != nil {
		return result
	}
	defer rows.Close()

	for rows.Next() {
		var parent, so, dn string
		if rows.Scan(&parent, &so, &dn) == nil {
			if _, ok := result[parent]; !ok {
				result[parent] = make(map[string][]string)
			}
			if so != "" {
				result[parent]["sales_order"] = appendUnique(result[parent]["sales_order"], so)
			}
			if dn != "" {
				result[parent]["delivery_note"] = appendUnique(result[parent]["delivery_note"], dn)
			}
		}
	}
	return result
}

func (r *SalesRegisterReport) getModeOfPayments(invoiceList []map[string]interface{}) map[string][]string {
	result := make(map[string][]string)
	if r.DB == nil || len(invoiceList) == 0 {
		return result
	}

	invNames := make([]interface{}, len(invoiceList))
	for i, inv := range invoiceList {
		invNames[i] = inv["name"]
	}

	placeholders := strings.Repeat("?,", len(invNames))
	placeholders = placeholders[:len(placeholders)-1]

	query := "SELECT `parent`, `mode_of_payment`" +
		" FROM `tabSales Invoice Payment`" +
		" WHERE `parent` IN (" + placeholders + ")"

	rows, err := r.DB.Query(query, invNames...)
	if err != nil {
		return result
	}
	defer rows.Close()

	for rows.Next() {
		var parent, mop string
		if rows.Scan(&parent, &mop) == nil && mop != "" {
			result[parent] = appendUnique(result[parent], mop)
		}
	}
	return result
}

func (r *SalesRegisterReport) queryStringList(query string, args []interface{}) []string {
	if r.DB == nil {
		return nil
	}
	rows, err := r.DB.Query(query, args...)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var result []string
	for rows.Next() {
		var val string
		if rows.Scan(&val) == nil {
			result = append(result, val)
		}
	}
	return result
}

// ---------------------------------------------------------------------------
// 2E.2b  Purchase Register Report
// Source: erpnext/accounts/report/purchase_register/purchase_register.py
// ---------------------------------------------------------------------------

// PurchaseRegisterReport implements the Purchase Register report.
type PurchaseRegisterReport struct {
	DB *db.DB
}

func (r *PurchaseRegisterReport) Execute(filters map[string]interface{}) (*ReportResult, error) {
	invoiceList, err := r.getInvoices(filters)
	if err != nil {
		return nil, fmt.Errorf("purchase register invoices: %w", err)
	}

	expenseAccounts, taxAccounts := r.getAccountColumns(invoiceList)
	columns := r.getColumns(expenseAccounts, taxAccounts)

	if len(invoiceList) == 0 {
		return &ReportResult{Columns: columns, Result: nil}, nil
	}

	invoiceExpenseMap := r.getInvoiceExpenseMap(invoiceList)
	invoiceTaxMap := r.getInvoiceTaxMap(invoiceList)
	invoicePOPRMap := r.getInvoicePOPRMap(invoiceList)

	var data [][]interface{}
	for _, inv := range invoiceList {
		invName, _ := inv["name"].(string)

		purchaseOrders := ""
		purchaseReceipts := ""
		if popr, ok := invoicePOPRMap[invName]; ok {
			if pos, ok := popr["purchase_order"]; ok {
				purchaseOrders = strings.Join(pos, ", ")
			}
			if prs, ok := popr["purchase_receipt"]; ok {
				purchaseReceipts = strings.Join(prs, ", ")
			}
		}

		baseNetTotal := 0.0
		for _, acc := range expenseAccounts {
			if expMap, ok := invoiceExpenseMap[invName]; ok {
				baseNetTotal += frappe.FltFromAny(expMap[acc], -1)
			}
		}

		totalTax := 0.0
		for _, acc := range taxAccounts {
			if taxMap, ok := invoiceTaxMap[invName]; ok {
				totalTax += frappe.FltFromAny(taxMap[acc], -1)
			}
		}

		netTotal := baseNetTotal
		if netTotal == 0 {
			netTotal = frappe.FltFromAny(inv["base_net_total"], -1)
		}

		row := make([]interface{}, len(columns))
		colIdx := map[string]int{}
		for i, col := range columns {
			colIdx[col.Fieldname] = i
		}

		setVal := func(fieldname string, val interface{}) {
			if idx, ok := colIdx[fieldname]; ok {
				row[idx] = val
			}
		}

		setVal("voucher_type", "Purchase Invoice")
		setVal("voucher_no", invName)
		setVal("posting_date", inv["posting_date"])
		setVal("supplier_id", inv["supplier"])
		setVal("supplier_name", inv["supplier_name"])
		setVal("tax_id", inv["tax_id"])
		setVal("payable_account", inv["credit_to"])
		setVal("mode_of_payment", inv["mode_of_payment"])
		setVal("bill_no", inv["bill_no"])
		setVal("bill_date", inv["bill_date"])
		setVal("remarks", inv["remarks"])
		setVal("purchase_order", purchaseOrders)
		setVal("purchase_receipt", purchaseReceipts)

		// Expense account values
		for _, acc := range expenseAccounts {
			fieldname := scrubAccountName(acc)
			amount := 0.0
			if expMap, ok := invoiceExpenseMap[invName]; ok {
				amount = frappe.FltFromAny(expMap[acc], -1)
			}
			setVal(fieldname, amount)
		}

		setVal("net_total", netTotal)

		// Tax account values
		for _, acc := range taxAccounts {
			fieldname := scrubAccountName(acc)
			amount := 0.0
			if taxMap, ok := invoiceTaxMap[invName]; ok {
				amount = frappe.FltFromAny(taxMap[acc], -1)
			}
			setVal(fieldname, amount)
		}

		setVal("total_tax", totalTax)
		setVal("grand_total", inv["base_grand_total"])
		setVal("rounded_total", inv["base_rounded_total"])
		setVal("outstanding_amount", inv["outstanding_amount"])

		data = append(data, row)
	}

	return &ReportResult{
		Columns: columns,
		Result:  data,
	}, nil
}

func (r *PurchaseRegisterReport) getColumns(expenseAccounts, taxAccounts []string) []Column {
	columns := []Column{
		{Label: "Voucher Type", Fieldname: "voucher_type", Width: 120},
		{Label: "Voucher", Fieldname: "voucher_no", Fieldtype: "Dynamic Link", Options: "voucher_type", Width: 120},
		{Label: "Posting Date", Fieldname: "posting_date", Fieldtype: "Date", Width: 80},
		{Label: "Supplier", Fieldname: "supplier_id", Fieldtype: "Link", Options: "Supplier", Width: 120},
		{Label: "Supplier Name", Fieldname: "supplier_name", Fieldtype: "Data", Width: 120},
		{Label: "Supplier Group", Fieldname: "supplier_group", Fieldtype: "Link", Options: "Supplier Group", Width: 120},
		{Label: "Tax Id", Fieldname: "tax_id", Fieldtype: "Data", Width: 80},
		{Label: "Payable Account", Fieldname: "payable_account", Fieldtype: "Link", Options: "Account", Width: 100},
		{Label: "Mode Of Payment", Fieldname: "mode_of_payment", Fieldtype: "Data", Width: 120},
		{Label: "Project", Fieldname: "project", Fieldtype: "Link", Options: "Project", Width: 80},
		{Label: "Bill No", Fieldname: "bill_no", Fieldtype: "Data", Width: 120},
		{Label: "Bill Date", Fieldname: "bill_date", Fieldtype: "Date", Width: 80},
		{Label: "Purchase Order", Fieldname: "purchase_order", Fieldtype: "Link", Options: "Purchase Order", Width: 100},
		{Label: "Purchase Receipt", Fieldname: "purchase_receipt", Fieldtype: "Link", Options: "Purchase Receipt", Width: 100},
		{Label: "Currency", Fieldname: "currency", Fieldtype: "Data", Width: 80},
	}

	// Expense account columns
	for _, acc := range expenseAccounts {
		columns = append(columns, Column{
			Label: acc, Fieldname: scrubAccountName(acc), Fieldtype: "Currency", Options: "currency", Width: 120,
		})
	}

	columns = append(columns, Column{
		Label: "Net Total", Fieldname: "net_total", Fieldtype: "Currency", Options: "currency", Width: 120,
	})

	// Tax account columns
	for _, acc := range taxAccounts {
		columns = append(columns, Column{
			Label: acc, Fieldname: scrubAccountName(acc), Fieldtype: "Currency", Options: "currency", Width: 120,
		})
	}

	columns = append(columns,
		Column{Label: "Total Tax", Fieldname: "total_tax", Fieldtype: "Currency", Options: "currency", Width: 120},
		Column{Label: "Grand Total", Fieldname: "grand_total", Fieldtype: "Currency", Options: "currency", Width: 120},
		Column{Label: "Rounded Total", Fieldname: "rounded_total", Fieldtype: "Currency", Options: "currency", Width: 120},
		Column{Label: "Outstanding Amount", Fieldname: "outstanding_amount", Fieldtype: "Currency", Options: "currency", Width: 120},
		Column{Label: "Remarks", Fieldname: "remarks", Fieldtype: "Data", Width: 120},
	)

	return columns
}

func (r *PurchaseRegisterReport) getInvoices(filters map[string]interface{}) ([]map[string]interface{}, error) {
	if r.DB == nil {
		return nil, nil
	}

	query := "SELECT 'Purchase Invoice' AS `doctype`, `name`, `posting_date`, `credit_to`," +
		" `supplier`, `supplier_name`, `tax_id`, `bill_no`, `bill_date`," +
		" `remarks`, `base_net_total`, `base_grand_total`, `base_rounded_total`," +
		" `outstanding_amount`, `mode_of_payment`," +
		" `is_internal_supplier`, `represents_company`, `company`" +
		" FROM `tabPurchase Invoice` WHERE `docstatus` = 1"

	var args []interface{}

	if supplier, ok := filters["supplier"].(string); ok && supplier != "" {
		query += " AND `supplier` = ?"
		args = append(args, supplier)
	}
	if company, ok := filters["company"].(string); ok && company != "" {
		query += " AND `company` = ?"
		args = append(args, company)
	}
	if fromDate, ok := filters["from_date"].(string); ok && fromDate != "" {
		query += " AND `posting_date` >= ?"
		args = append(args, fromDate)
	}
	if toDate, ok := filters["to_date"].(string); ok && toDate != "" {
		query += " AND `posting_date` <= ?"
		args = append(args, toDate)
	}

	query += " ORDER BY `posting_date` DESC, `name` DESC"

	rows, err := r.DB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanRowsToMaps(rows)
}

func (r *PurchaseRegisterReport) getAccountColumns(invoiceList []map[string]interface{}) ([]string, []string) {
	if r.DB == nil || len(invoiceList) == 0 {
		return nil, nil
	}

	invNames := make([]interface{}, len(invoiceList))
	for i, inv := range invoiceList {
		invNames[i] = inv["name"]
	}

	placeholders := strings.Repeat("?,", len(invNames))
	placeholders = placeholders[:len(placeholders)-1]

	// Expense accounts
	expQuery := "SELECT DISTINCT `expense_account` FROM `tabPurchase Invoice Item`" +
		" WHERE `docstatus` = 1 AND (`expense_account` IS NOT NULL AND `expense_account` != '')" +
		" AND `parenttype` = 'Purchase Invoice'" +
		" AND `parent` IN (" + placeholders + ")" +
		" ORDER BY `expense_account`"
	expenseAccounts := r.queryStringList(expQuery, invNames)

	// Tax accounts
	taxQuery := "SELECT DISTINCT `account_head` FROM `tabPurchase Taxes and Charges`" +
		" WHERE `parent` IN (" + placeholders + ")" +
		" AND `category` IN ('Total', 'Valuation and Total')" +
		" AND `base_tax_amount_after_discount_amount` != 0" +
		" ORDER BY `account_head`"
	taxAccounts := r.queryStringList(taxQuery, invNames)

	// Remove expense accounts from tax accounts
	expenseSet := make(map[string]bool)
	for _, acc := range expenseAccounts {
		expenseSet[acc] = true
	}
	var filteredTax []string
	for _, acc := range taxAccounts {
		if !expenseSet[acc] {
			filteredTax = append(filteredTax, acc)
		}
	}

	return expenseAccounts, filteredTax
}

func (r *PurchaseRegisterReport) getInvoiceExpenseMap(invoiceList []map[string]interface{}) map[string]map[string]float64 {
	result := make(map[string]map[string]float64)
	if r.DB == nil || len(invoiceList) == 0 {
		return result
	}

	invNames := make([]interface{}, len(invoiceList))
	for i, inv := range invoiceList {
		invNames[i] = inv["name"]
	}

	placeholders := strings.Repeat("?,", len(invNames))
	placeholders = placeholders[:len(placeholders)-1]

	query := "SELECT `parent`, `expense_account`, SUM(`base_net_amount`) AS `amount`" +
		" FROM `tabPurchase Invoice Item`" +
		" WHERE `parent` IN (" + placeholders + ") AND `parenttype` = 'Purchase Invoice'" +
		" GROUP BY `parent`, `expense_account`"

	rows, err := r.DB.Query(query, invNames...)
	if err != nil {
		return result
	}
	defer rows.Close()

	for rows.Next() {
		var parent, account string
		var amount float64
		if rows.Scan(&parent, &account, &amount) == nil {
			if _, ok := result[parent]; !ok {
				result[parent] = make(map[string]float64)
			}
			result[parent][account] = amount
		}
	}
	return result
}

func (r *PurchaseRegisterReport) getInvoiceTaxMap(invoiceList []map[string]interface{}) map[string]map[string]float64 {
	result := make(map[string]map[string]float64)
	if r.DB == nil || len(invoiceList) == 0 {
		return result
	}

	invNames := make([]interface{}, len(invoiceList))
	for i, inv := range invoiceList {
		invNames[i] = inv["name"]
	}

	placeholders := strings.Repeat("?,", len(invNames))
	placeholders = placeholders[:len(placeholders)-1]

	query := "SELECT `parent`, `account_head`," +
		" SUM(CASE WHEN `add_deduct_tax` = 'Add' THEN `base_tax_amount_after_discount_amount`" +
		" ELSE -`base_tax_amount_after_discount_amount` END) AS `tax_amount`" +
		" FROM `tabPurchase Taxes and Charges`" +
		" WHERE `parent` IN (" + placeholders + ")" +
		" AND `category` IN ('Total', 'Valuation and Total')" +
		" AND `base_tax_amount_after_discount_amount` != 0" +
		" GROUP BY `parent`, `account_head`, `add_deduct_tax`"

	rows, err := r.DB.Query(query, invNames...)
	if err != nil {
		return result
	}
	defer rows.Close()

	for rows.Next() {
		var parent, account string
		var amount float64
		if rows.Scan(&parent, &account, &amount) == nil {
			if _, ok := result[parent]; !ok {
				result[parent] = make(map[string]float64)
			}
			result[parent][account] += amount
		}
	}
	return result
}

func (r *PurchaseRegisterReport) getInvoicePOPRMap(invoiceList []map[string]interface{}) map[string]map[string][]string {
	result := make(map[string]map[string][]string)
	if r.DB == nil || len(invoiceList) == 0 {
		return result
	}

	invNames := make([]interface{}, len(invoiceList))
	for i, inv := range invoiceList {
		invNames[i] = inv["name"]
	}

	placeholders := strings.Repeat("?,", len(invNames))
	placeholders = placeholders[:len(placeholders)-1]

	query := "SELECT `parent`, `purchase_order`, `purchase_receipt`, `project`" +
		" FROM `tabPurchase Invoice Item`" +
		" WHERE `parent` IN (" + placeholders + ")" +
		" AND (`purchase_order` != '' OR `purchase_receipt` != '' OR `project` != '')"

	rows, err := r.DB.Query(query, invNames...)
	if err != nil {
		return result
	}
	defer rows.Close()

	for rows.Next() {
		var parent, po, pr, project string
		if rows.Scan(&parent, &po, &pr, &project) == nil {
			if _, ok := result[parent]; !ok {
				result[parent] = make(map[string][]string)
			}
			if po != "" {
				result[parent]["purchase_order"] = appendUnique(result[parent]["purchase_order"], po)
			}
			if pr != "" {
				result[parent]["purchase_receipt"] = appendUnique(result[parent]["purchase_receipt"], pr)
			}
			if project != "" {
				result[parent]["project"] = appendUnique(result[parent]["project"], project)
			}
		}
	}
	return result
}

func (r *PurchaseRegisterReport) queryStringList(query string, args []interface{}) []string {
	if r.DB == nil {
		return nil
	}
	rows, err := r.DB.Query(query, args...)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var result []string
	for rows.Next() {
		var val string
		if rows.Scan(&val) == nil {
			result = append(result, val)
		}
	}
	return result
}

// ---------------------------------------------------------------------------
// 2E.2c  Item Wise Sales Register Report
// Source: erpnext/accounts/report/item_wise_sales_register/item_wise_sales_register.py
// ---------------------------------------------------------------------------

// ItemWiseSalesRegisterReport implements the Item Wise Sales Register report.
type ItemWiseSalesRegisterReport struct {
	DB *db.DB
}

func (r *ItemWiseSalesRegisterReport) Execute(filters map[string]interface{}) (*ReportResult, error) {
	columns := r.getColumns(filters)

	items, err := r.getItems(filters)
	if err != nil {
		return nil, fmt.Errorf("item wise sales register items: %w", err)
	}

	if len(items) == 0 {
		return &ReportResult{Columns: columns, Result: nil}, nil
	}

	// TODO: get_tax_accounts for dynamic tax columns
	// TODO: get_delivery_notes_against_sales_order

	var data [][]interface{}
	for _, d := range items {
		row := make([]interface{}, len(columns))
		colIdx := map[string]int{}
		for i, col := range columns {
			colIdx[col.Fieldname] = i
		}

		setVal := func(fieldname string, val interface{}) {
			if idx, ok := colIdx[fieldname]; ok {
				row[idx] = val
			}
		}

		setVal("item_code", d["item_code"])
		setVal("item_name", d["item_name"])
		setVal("item_group", d["item_group"])
		setVal("description", d["description"])
		setVal("invoice", d["parent"])
		setVal("posting_date", d["posting_date"])
		setVal("customer", d["customer"])
		setVal("customer_name", d["customer_name"])
		setVal("customer_group", d["customer_group"])
		setVal("debit_to", d["debit_to"])
		setVal("territory", d["territory"])
		setVal("project", d["project"])
		setVal("company", d["company"])
		setVal("sales_order", d["sales_order"])
		setVal("delivery_note", d["delivery_note"])
		setVal("income_account", d["income_account"])
		setVal("cost_center", d["cost_center"])
		setVal("stock_qty", d["stock_qty"])
		setVal("stock_uom", d["stock_uom"])

		baseNetRate := frappe.FltFromAny(d["base_net_rate"], -1)
		baseNetAmount := frappe.FltFromAny(d["base_net_amount"], -1)

		setVal("rate", baseNetRate)
		setVal("amount", baseNetAmount)
		setVal("total_tax", 0.0)
		setVal("total", baseNetAmount)

		data = append(data, row)
	}

	return &ReportResult{
		Columns: columns,
		Result:  data,
	}, nil
}

func (r *ItemWiseSalesRegisterReport) getColumns(filters map[string]interface{}) []Column {
	columns := []Column{
		{Label: "Item Code", Fieldname: "item_code", Fieldtype: "Link", Options: "Item", Width: 120},
		{Label: "Item Name", Fieldname: "item_name", Fieldtype: "Data", Width: 120},
		{Label: "Item Group", Fieldname: "item_group", Fieldtype: "Link", Options: "Item Group", Width: 120},
		{Label: "Description", Fieldname: "description", Fieldtype: "Data", Width: 150},
		{Label: "Invoice", Fieldname: "invoice", Fieldtype: "Link", Options: "Sales Invoice", Width: 150},
		{Label: "Posting Date", Fieldname: "posting_date", Fieldtype: "Date", Width: 120},
		{Label: "Customer", Fieldname: "customer", Fieldtype: "Link", Options: "Customer", Width: 120},
		{Label: "Customer Name", Fieldname: "customer_name", Fieldtype: "Data", Width: 120},
		{Label: "Customer Group", Fieldname: "customer_group", Fieldtype: "Link", Options: "Customer Group", Width: 120},
		{Label: "Receivable Account", Fieldname: "debit_to", Fieldtype: "Link", Options: "Account", Width: 80},
		{Label: "Mode Of Payment", Fieldname: "mode_of_payment", Fieldtype: "Data", Width: 120},
		{Label: "Territory", Fieldname: "territory", Fieldtype: "Link", Options: "Territory", Width: 80},
		{Label: "Project", Fieldname: "project", Fieldtype: "Link", Options: "Project", Width: 80},
		{Label: "Company", Fieldname: "company", Fieldtype: "Link", Options: "Company", Width: 80},
		{Label: "Sales Order", Fieldname: "sales_order", Fieldtype: "Link", Options: "Sales Order", Width: 100},
		{Label: "Delivery Note", Fieldname: "delivery_note", Fieldtype: "Link", Options: "Delivery Note", Width: 100},
		{Label: "Income Account", Fieldname: "income_account", Fieldtype: "Link", Options: "Account", Width: 100},
		{Label: "Cost Center", Fieldname: "cost_center", Fieldtype: "Link", Options: "Cost Center", Width: 100},
		{Label: "Stock Qty", Fieldname: "stock_qty", Fieldtype: "Float", Width: 100},
		{Label: "Stock UOM", Fieldname: "stock_uom", Fieldtype: "Link", Options: "UOM", Width: 100},
		{Label: "Rate", Fieldname: "rate", Fieldtype: "Float", Options: "currency", Width: 100},
		{Label: "Amount", Fieldname: "amount", Fieldtype: "Currency", Options: "currency", Width: 100},
		// TODO: dynamic tax columns appended here
		{Label: "Total Tax", Fieldname: "total_tax", Fieldtype: "Currency", Options: "currency", Width: 120},
		{Label: "Total", Fieldname: "total", Fieldtype: "Currency", Options: "currency", Width: 120},
		{Label: "Currency", Fieldname: "currency", Fieldtype: "Data", Width: 80},
	}

	return columns
}

func (r *ItemWiseSalesRegisterReport) getItems(filters map[string]interface{}) ([]map[string]interface{}, error) {
	if r.DB == nil {
		return nil, nil
	}

	query := "SELECT sii.`name`, sii.`parent`, si.`posting_date`, si.`debit_to`," +
		" si.`customer`, si.`remarks`, si.`territory`, si.`company`," +
		" si.`base_net_total`, sii.`project`, sii.`item_code`, sii.`description`," +
		" sii.`item_name`, sii.`item_group`, sii.`sales_order`, sii.`delivery_note`," +
		" sii.`income_account`, sii.`cost_center`, sii.`stock_qty`, sii.`stock_uom`," +
		" sii.`base_net_rate`, sii.`base_net_amount`, si.`customer_name`," +
		" IFNULL(si.`customer_group`, 'Not Specified') AS `customer_group`" +
		" FROM `tabSales Invoice` si" +
		" INNER JOIN `tabSales Invoice Item` sii ON si.`name` = sii.`parent`" +
		" WHERE si.`docstatus` = 1 AND sii.`parenttype` = 'Sales Invoice'"

	var args []interface{}

	if company, ok := filters["company"].(string); ok && company != "" {
		query += " AND si.`company` = ?"
		args = append(args, company)
	}
	if customer, ok := filters["customer"].(string); ok && customer != "" {
		query += " AND si.`customer` = ?"
		args = append(args, customer)
	}
	if fromDate, ok := filters["from_date"].(string); ok && fromDate != "" {
		query += " AND si.`posting_date` >= ?"
		args = append(args, fromDate)
	}
	if toDate, ok := filters["to_date"].(string); ok && toDate != "" {
		query += " AND si.`posting_date` <= ?"
		args = append(args, toDate)
	}
	if itemCode, ok := filters["item_code"].(string); ok && itemCode != "" {
		query += " AND sii.`item_code` = ?"
		args = append(args, itemCode)
	}
	if itemGroup, ok := filters["item_group"].(string); ok && itemGroup != "" {
		query += " AND sii.`item_group` = ?"
		args = append(args, itemGroup)
	}
	if brand, ok := filters["brand"].(string); ok && brand != "" {
		query += " AND sii.`brand` = ?"
		args = append(args, brand)
	}
	if warehouse, ok := filters["warehouse"].(string); ok && warehouse != "" {
		query += " AND sii.`warehouse` = ?"
		args = append(args, warehouse)
	}

	query += " ORDER BY si.`posting_date` DESC, sii.`item_group` DESC"

	rows, err := r.DB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanRowsToMaps(rows)
}

// ---------------------------------------------------------------------------
// 2E.2d  Item Wise Purchase Register Report
// Source: erpnext/accounts/report/item_wise_purchase_register/item_wise_purchase_register.py
// ---------------------------------------------------------------------------

// ItemWisePurchaseRegisterReport implements the Item Wise Purchase Register report.
type ItemWisePurchaseRegisterReport struct {
	DB *db.DB
}

func (r *ItemWisePurchaseRegisterReport) Execute(filters map[string]interface{}) (*ReportResult, error) {
	columns := r.getColumns(filters)

	items, err := r.getItems(filters)
	if err != nil {
		return nil, fmt.Errorf("item wise purchase register items: %w", err)
	}

	if len(items) == 0 {
		return &ReportResult{Columns: columns, Result: nil}, nil
	}

	// TODO: get_tax_accounts for dynamic tax columns

	var data [][]interface{}
	for _, d := range items {
		row := make([]interface{}, len(columns))
		colIdx := map[string]int{}
		for i, col := range columns {
			colIdx[col.Fieldname] = i
		}

		setVal := func(fieldname string, val interface{}) {
			if idx, ok := colIdx[fieldname]; ok {
				row[idx] = val
			}
		}

		setVal("item_code", d["item_code"])
		setVal("item_name", d["item_name"])
		setVal("item_group", d["item_group"])
		setVal("description", d["description"])
		setVal("invoice", d["parent"])
		setVal("posting_date", d["posting_date"])
		setVal("supplier", d["supplier"])
		setVal("supplier_name", d["supplier_name"])
		setVal("credit_to", d["credit_to"])
		setVal("mode_of_payment", d["mode_of_payment"])
		setVal("project", d["project"])
		setVal("company", d["company"])
		setVal("purchase_order", d["purchase_order"])
		setVal("purchase_receipt", d["purchase_receipt"])
		setVal("expense_account", d["expense_account"])
		setVal("stock_qty", d["stock_qty"])
		setVal("stock_uom", d["stock_uom"])

		baseNetAmount := frappe.FltFromAny(d["base_net_amount"], -1)
		stockQty := frappe.FltFromAny(d["stock_qty"], -1)
		rate := baseNetAmount
		if stockQty != 0 {
			rate = baseNetAmount / stockQty
		}

		setVal("rate", rate)
		setVal("amount", baseNetAmount)
		setVal("total_tax", 0.0)
		setVal("total", baseNetAmount)

		data = append(data, row)
	}

	return &ReportResult{
		Columns: columns,
		Result:  data,
	}, nil
}

func (r *ItemWisePurchaseRegisterReport) getColumns(filters map[string]interface{}) []Column {
	columns := []Column{
		{Label: "Item Code", Fieldname: "item_code", Fieldtype: "Link", Options: "Item", Width: 120},
		{Label: "Item Name", Fieldname: "item_name", Fieldtype: "Data", Width: 120},
		{Label: "Item Group", Fieldname: "item_group", Fieldtype: "Link", Options: "Item Group", Width: 120},
		{Label: "Description", Fieldname: "description", Fieldtype: "Data", Width: 150},
		{Label: "Invoice", Fieldname: "invoice", Fieldtype: "Link", Options: "Purchase Invoice", Width: 150},
		{Label: "Posting Date", Fieldname: "posting_date", Fieldtype: "Date", Width: 120},
		{Label: "Supplier", Fieldname: "supplier", Fieldtype: "Link", Options: "Supplier", Width: 120},
		{Label: "Supplier Name", Fieldname: "supplier_name", Fieldtype: "Data", Width: 120},
		{Label: "Payable Account", Fieldname: "credit_to", Fieldtype: "Link", Options: "Account", Width: 80},
		{Label: "Mode Of Payment", Fieldname: "mode_of_payment", Fieldtype: "Link", Options: "Mode of Payment", Width: 120},
		{Label: "Project", Fieldname: "project", Fieldtype: "Link", Options: "Project", Width: 80},
		{Label: "Company", Fieldname: "company", Fieldtype: "Link", Options: "Company", Width: 80},
		{Label: "Purchase Order", Fieldname: "purchase_order", Fieldtype: "Link", Options: "Purchase Order", Width: 100},
		{Label: "Purchase Receipt", Fieldname: "purchase_receipt", Fieldtype: "Link", Options: "Purchase Receipt", Width: 100},
		{Label: "Expense Account", Fieldname: "expense_account", Fieldtype: "Link", Options: "Account", Width: 100},
		{Label: "Stock Qty", Fieldname: "stock_qty", Fieldtype: "Float", Width: 100},
		{Label: "Stock UOM", Fieldname: "stock_uom", Fieldtype: "Link", Options: "UOM", Width: 100},
		{Label: "Rate", Fieldname: "rate", Fieldtype: "Float", Options: "currency", Width: 100},
		{Label: "Amount", Fieldname: "amount", Fieldtype: "Currency", Options: "currency", Width: 100},
		// TODO: dynamic tax columns appended here
		{Label: "Total Tax", Fieldname: "total_tax", Fieldtype: "Currency", Options: "currency", Width: 120},
		{Label: "Total", Fieldname: "total", Fieldtype: "Currency", Options: "currency", Width: 120},
		{Label: "Currency", Fieldname: "currency", Fieldtype: "Data", Width: 80},
	}

	return columns
}

func (r *ItemWisePurchaseRegisterReport) getItems(filters map[string]interface{}) ([]map[string]interface{}, error) {
	if r.DB == nil {
		return nil, nil
	}

	query := "SELECT pii.`name`, pii.`parent`, pi.`posting_date`, pi.`credit_to`," +
		" pi.`company`, pi.`supplier`, pi.`remarks`, pi.`base_net_total`," +
		" pi.`unrealized_profit_loss_account`," +
		" pii.`item_code`, pii.`description`, pii.`item_name`, pii.`item_group`," +
		" pii.`project`, pii.`purchase_order`, pii.`purchase_receipt`," +
		" pii.`po_detail`, pii.`expense_account`, pii.`stock_qty`, pii.`stock_uom`," +
		" pii.`base_net_amount`, pi.`supplier_name`, pi.`mode_of_payment`" +
		" FROM `tabPurchase Invoice` pi" +
		" INNER JOIN `tabPurchase Invoice Item` pii ON pi.`name` = pii.`parent`" +
		" WHERE pi.`docstatus` = 1 AND pii.`parenttype` = 'Purchase Invoice'"

	var args []interface{}

	if company, ok := filters["company"].(string); ok && company != "" {
		query += " AND pi.`company` = ?"
		args = append(args, company)
	}
	if supplier, ok := filters["supplier"].(string); ok && supplier != "" {
		query += " AND pi.`supplier` = ?"
		args = append(args, supplier)
	}
	if fromDate, ok := filters["from_date"].(string); ok && fromDate != "" {
		query += " AND pi.`posting_date` >= ?"
		args = append(args, fromDate)
	}
	if toDate, ok := filters["to_date"].(string); ok && toDate != "" {
		query += " AND pi.`posting_date` <= ?"
		args = append(args, toDate)
	}
	if itemCode, ok := filters["item_code"].(string); ok && itemCode != "" {
		query += " AND pii.`item_code` = ?"
		args = append(args, itemCode)
	}
	if itemGroup, ok := filters["item_group"].(string); ok && itemGroup != "" {
		query += " AND pii.`item_group` = ?"
		args = append(args, itemGroup)
	}

	query += " ORDER BY pi.`posting_date` DESC, pii.`item_group` DESC"

	rows, err := r.DB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanRowsToMaps(rows)
}

// ---------------------------------------------------------------------------
// 2E.2e  Delivered Items To Be Billed Report
// Source: erpnext/accounts/report/delivered_items_to_be_billed/delivered_items_to_be_billed.py
// ---------------------------------------------------------------------------

// DeliveredItemsToBeBilledReport implements the Delivered Items To Be Billed report.
type DeliveredItemsToBeBilledReport struct {
	DB *db.DB
}

func (r *DeliveredItemsToBeBilledReport) Execute(filters map[string]interface{}) (*ReportResult, error) {
	columns := r.getColumns()
	data, err := r.getData(filters)
	if err != nil {
		return nil, fmt.Errorf("delivered items to be billed: %w", err)
	}

	return &ReportResult{
		Columns: columns,
		Result:  data,
	}, nil
}

func (r *DeliveredItemsToBeBilledReport) getColumns() []Column {
	return []Column{
		{Label: "Delivery Note", Fieldname: "name", Fieldtype: "Link", Options: "Delivery Note", Width: 160},
		{Label: "Date", Fieldname: "date", Fieldtype: "Date", Width: 100},
		{Label: "Customer", Fieldname: "customer", Fieldtype: "Link", Options: "Customer", Width: 120},
		{Label: "Customer Name", Fieldname: "customer_name", Fieldtype: "Data", Width: 120},
		{Label: "Item Code", Fieldname: "item_code", Fieldtype: "Link", Options: "Item", Width: 120},
		{Label: "Amount", Fieldname: "amount", Fieldtype: "Currency", Options: "Company:company:default_currency", Width: 100},
		{Label: "Billed Amount", Fieldname: "billed_amount", Fieldtype: "Currency", Options: "Company:company:default_currency", Width: 100},
		{Label: "Returned Amount", Fieldname: "returned_amount", Fieldtype: "Currency", Options: "Company:company:default_currency", Width: 120},
		{Label: "Pending Amount", Fieldname: "pending_amount", Fieldtype: "Currency", Options: "Company:company:default_currency", Width: 120},
		{Label: "Item Name", Fieldname: "item_name", Fieldtype: "Data", Width: 120},
		{Label: "Description", Fieldname: "description", Fieldtype: "Data", Width: 120},
		{Label: "Project", Fieldname: "project", Fieldtype: "Link", Options: "Project", Width: 120},
	}
}

func (r *DeliveredItemsToBeBilledReport) getData(filters map[string]interface{}) ([][]interface{}, error) {
	if r.DB == nil {
		return nil, nil
	}

	query := "SELECT dn.`name`, dn.`posting_date` AS `date`," +
		" dn.`customer`, dn.`customer_name`," +
		" dni.`item_code`, dni.`amount`, dni.`billed_amt` AS `billed_amount`," +
		" dni.`returned_qty` * dni.`rate` AS `returned_amount`," +
		" (dni.`amount` - dni.`billed_amt` - dni.`returned_qty` * dni.`rate`) AS `pending_amount`," +
		" dni.`item_name`, dni.`description`, dni.`project`" +
		" FROM `tabDelivery Note` dn" +
		" INNER JOIN `tabDelivery Note Item` dni ON dn.`name` = dni.`parent`" +
		" WHERE dn.`docstatus` = 1 AND dn.`status` NOT IN ('Stopped', 'Closed')" +
		" AND dni.`amount` > 0 AND dni.`billed_amt` < dni.`amount`"

	var args []interface{}

	if company, ok := filters["company"].(string); ok && company != "" {
		query += " AND dn.`company` = ?"
		args = append(args, company)
	}
	if customer, ok := filters["customer"].(string); ok && customer != "" {
		query += " AND dn.`customer` = ?"
		args = append(args, customer)
	}
	if fromDate, ok := filters["from_date"].(string); ok && fromDate != "" {
		query += " AND dn.`posting_date` >= ?"
		args = append(args, fromDate)
	}
	if toDate, ok := filters["to_date"].(string); ok && toDate != "" {
		query += " AND dn.`posting_date` <= ?"
		args = append(args, toDate)
	}

	query += " ORDER BY dn.`name` DESC"

	rows, err := r.DB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanRowsToSlices(rows)
}

// ---------------------------------------------------------------------------
// 2E.2f  Billed Items To Be Received Report
// Source: erpnext/accounts/report/billed_items_to_be_received/billed_items_to_be_received.py
// ---------------------------------------------------------------------------

// BilledItemsToBeReceivedReport implements the Billed Items To Be Received report.
type BilledItemsToBeReceivedReport struct {
	DB *db.DB
}

func (r *BilledItemsToBeReceivedReport) Execute(filters map[string]interface{}) (*ReportResult, error) {
	columns := r.getColumns()
	data, err := r.getData(filters)
	if err != nil {
		return nil, fmt.Errorf("billed items to be received: %w", err)
	}

	return &ReportResult{
		Columns: columns,
		Result:  data,
	}, nil
}

func (r *BilledItemsToBeReceivedReport) getColumns() []Column {
	return []Column{
		{Label: "Purchase Invoice", Fieldname: "name", Fieldtype: "Link", Options: "Purchase Invoice", Width: 170},
		{Label: "Supplier", Fieldname: "supplier", Fieldtype: "Link", Options: "Supplier", Width: 120},
		{Label: "Posting Date", Fieldname: "posting_date", Fieldtype: "Date", Width: 100},
		{Label: "Item Code", Fieldname: "item_code", Fieldtype: "Link", Options: "Item", Width: 100},
		{Label: "Item Name", Fieldname: "item_name", Fieldtype: "Data", Width: 100},
		{Label: "UOM", Fieldname: "uom", Fieldtype: "Link", Options: "UOM", Width: 100},
		{Label: "Invoiced Qty", Fieldname: "qty", Fieldtype: "Float", Width: 100},
		{Label: "Received Qty", Fieldname: "received_qty", Fieldtype: "Float", Width: 100},
		{Label: "Rate", Fieldname: "rate", Fieldtype: "Currency", Width: 100},
		{Label: "Amount", Fieldname: "amount", Fieldtype: "Currency", Width: 100},
	}
}

func (r *BilledItemsToBeReceivedReport) getData(filters map[string]interface{}) ([][]interface{}, error) {
	if r.DB == nil {
		return nil, nil
	}

	query := "SELECT pi.`name`, pi.`supplier`, pi.`posting_date`," +
		" pii.`item_code`, pii.`item_name`, pii.`uom`," +
		" pii.`qty`, pii.`received_qty`, pii.`rate`, pii.`amount`" +
		" FROM `tabPurchase Invoice` pi" +
		" INNER JOIN `tabPurchase Invoice Item` pii ON pi.`name` = pii.`parent`" +
		" WHERE pi.`docstatus` = 1 AND pi.`per_received` < 100" +
		" AND pi.`update_stock` = 0 AND pi.`is_opening` != 'Yes'"

	var args []interface{}

	if company, ok := filters["company"].(string); ok && company != "" {
		query += " AND pi.`company` = ?"
		args = append(args, company)
	}
	if postingDate, ok := filters["posting_date"].(string); ok && postingDate != "" {
		query += " AND pi.`posting_date` <= ?"
		args = append(args, postingDate)
	}

	rows, err := r.DB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanRowsToSlices(rows)
}

// ---------------------------------------------------------------------------
// Utility functions
// ---------------------------------------------------------------------------

// scrubAccountName converts an account name to a fieldname (similar to frappe.scrub).
// E.g., "Cost of Goods Sold - TC" -> "cost_of_goods_sold___tc"
func scrubAccountName(name string) string {
	result := strings.ToLower(name)
	result = strings.ReplaceAll(result, " ", "_")
	result = strings.ReplaceAll(result, "-", "_")
	// Remove characters that aren't alphanumeric or underscore
	var cleaned strings.Builder
	for _, ch := range result {
		if (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch == '_' {
			cleaned.WriteRune(ch)
		}
	}
	return cleaned.String()
}

// appendUnique appends a value to a string slice only if it's not already present.
func appendUnique(slice []string, val string) []string {
	for _, s := range slice {
		if s == val {
			return slice
		}
	}
	return append(slice, val)
}
