package reports

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/alex-baker-cdm/erpnext/go-erpnext/pkg/db"
	"github.com/alex-baker-cdm/erpnext/go-erpnext/pkg/frappe"
)

// ---------------------------------------------------------------------------
// 2D.2a — Item Prices
// ---------------------------------------------------------------------------

// ItemPricesReport implements the Item Prices report.
type ItemPricesReport struct {
	DB *db.DB
}

// Execute runs the Item Prices report.
func (r *ItemPricesReport) Execute(filters map[string]interface{}) (*ReportResult, error) {
	columns := ParseColumnShorthands([]string{
		"Item:Link/Item:100",
		"Item Name::150",
		"Item Group:Link/Item Group:125",
		"Brand::100",
		"Description::150",
		"UOM:Link/UOM:80",
		"Last Purchase Rate:Currency:90",
		"Valuation Rate:Currency:80",
		"Sales Price List::180",
		"Purchase Price List::180",
		"BOM Rate:Currency:90",
	})

	itemMap, err := r.getItemDetails(filters)
	if err != nil {
		return nil, err
	}

	pl, err := r.getPriceList()
	if err != nil {
		return nil, err
	}

	lastPurchaseRate, err := r.getLastPurchaseRate()
	if err != nil {
		return nil, err
	}

	bomRate, err := r.getItemBOMRate()
	if err != nil {
		return nil, err
	}

	valRateMap, err := r.getValuationRate()
	if err != nil {
		return nil, err
	}

	precision := 2

	// Sort item codes
	var itemCodes []string
	for code := range itemMap {
		itemCodes = append(itemCodes, code)
	}
	sort.Strings(itemCodes)

	var data [][]interface{}
	for _, item := range itemCodes {
		detail := itemMap[item]
		lpr := frappe.Flt(lastPurchaseRate[item], precision)
		vr := frappe.Flt(valRateMap[item], precision)
		br := frappe.Flt(bomRate[item], precision)

		sellingPL := ""
		if pli, ok := pl[item]; ok {
			sellingPL = pli["Selling"]
		}
		buyingPL := ""
		if pli, ok := pl[item]; ok {
			buyingPL = pli["Buying"]
		}

		data = append(data, []interface{}{
			item,
			detail["item_name"],
			detail["item_group"],
			detail["brand"],
			detail["description"],
			detail["stock_uom"],
			lpr,
			vr,
			sellingPL,
			buyingPL,
			br,
		})
	}

	return &ReportResult{Columns: columns, Result: data}, nil
}

func (r *ItemPricesReport) getItemDetails(filters map[string]interface{}) (map[string]map[string]string, error) {
	query := "SELECT `name`, `item_group`, `item_name`, `description`, `brand`, `stock_uom` FROM `tabItem`"
	var args []interface{}

	itemsFilter, _ := filters["items"].(string)
	switch itemsFilter {
	case "Enabled Items only":
		query += " WHERE `disabled` = 0"
	case "Disabled Items only":
		query += " WHERE `disabled` = 1"
	}

	query += " ORDER BY `item_code`, `item_group`"

	rows, err := r.DB.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("item details query: %w", err)
	}
	defer rows.Close()

	itemMap := make(map[string]map[string]string)
	for rows.Next() {
		var name, itemGroup, itemName, description, brand, stockUOM string
		if err := rows.Scan(&name, &itemGroup, &itemName, &description, &brand, &stockUOM); err != nil {
			return nil, err
		}
		itemMap[name] = map[string]string{
			"item_name":   itemName,
			"item_group":  itemGroup,
			"description": description,
			"brand":       brand,
			"stock_uom":   stockUOM,
		}
	}
	return itemMap, nil
}

func (r *ItemPricesReport) getPriceList() (map[string]map[string]string, error) {
	query := `SELECT ip.item_code, ip.buying, ip.selling,
		IFNULL(cu.symbol, ip.currency) AS currency,
		ip.price_list_rate, ip.price_list
		FROM ` + "`tabItem Price`" + ` ip, ` + "`tabPrice List`" + ` pl, ` + "`tabCurrency`" + ` cu
		WHERE ip.price_list = pl.name AND pl.currency = cu.name AND pl.enabled = 1`

	rows, err := r.DB.Query(query)
	if err != nil {
		return nil, fmt.Errorf("price list query: %w", err)
	}
	defer rows.Close()

	// rate[item_code]["Buying"|"Selling"] = []string of price entries
	rate := make(map[string]map[string][]string)

	for rows.Next() {
		var itemCode, currency, priceList string
		var buying, selling int
		var priceListRate float64
		if err := rows.Scan(&itemCode, &buying, &selling, &currency, &priceListRate, &priceList); err != nil {
			return nil, err
		}
		price := fmt.Sprintf("%s %.2f - %s", currency, priceListRate, priceList)
		key := "Selling"
		if buying == 1 {
			key = "Buying"
		}
		if rate[itemCode] == nil {
			rate[itemCode] = make(map[string][]string)
		}
		rate[itemCode][key] = append(rate[itemCode][key], price)
	}

	result := make(map[string]map[string]string)
	for item, byType := range rate {
		result[item] = make(map[string]string)
		for key, prices := range byType {
			result[item][key] = strings.Join(prices, ", ")
		}
	}
	return result, nil
}

func (r *ItemPricesReport) getLastPurchaseRate() (map[string]float64, error) {
	query := `SELECT item_code, posting_date, base_rate FROM (
		SELECT poi.item_code, po.transaction_date AS posting_date, poi.base_rate
		FROM ` + "`tabPurchase Order`" + ` po, ` + "`tabPurchase Order Item`" + ` poi
		WHERE po.name = poi.parent AND po.docstatus = 1
		UNION ALL
		SELECT pri.item_code, pr.posting_date, pri.base_rate
		FROM ` + "`tabPurchase Receipt`" + ` pr, ` + "`tabPurchase Receipt Item`" + ` pri
		WHERE pr.name = pri.parent AND pr.docstatus = 1
		UNION ALL
		SELECT pii.item_code, pi.posting_date, pii.base_rate
		FROM ` + "`tabPurchase Invoice`" + ` pi, ` + "`tabPurchase Invoice Item`" + ` pii
		WHERE pi.name = pii.parent AND pi.docstatus = 1 AND pi.update_stock = 1
	) AS combined ORDER BY item_code, posting_date`

	rows, err := r.DB.Query(query)
	if err != nil {
		return nil, fmt.Errorf("last purchase rate query: %w", err)
	}
	defer rows.Close()

	result := make(map[string]float64)
	for rows.Next() {
		var itemCode, postingDate string
		var baseRate float64
		if err := rows.Scan(&itemCode, &postingDate, &baseRate); err != nil {
			return nil, err
		}
		// Last row per item wins due to ORDER BY
		result[itemCode] = baseRate
	}
	return result, nil
}

func (r *ItemPricesReport) getItemBOMRate() (map[string]float64, error) {
	query := "SELECT `item`, `total_cost` / `quantity` AS bom_rate FROM `tabBOM` WHERE `is_active` = 1 AND `is_default` = 1"
	rows, err := r.DB.Query(query)
	if err != nil {
		return nil, fmt.Errorf("bom rate query: %w", err)
	}
	defer rows.Close()

	result := make(map[string]float64)
	for rows.Next() {
		var item string
		var bomRate float64
		if err := rows.Scan(&item, &bomRate); err != nil {
			return nil, err
		}
		if _, exists := result[item]; !exists {
			result[item] = bomRate
		}
	}
	return result, nil
}

func (r *ItemPricesReport) getValuationRate() (map[string]float64, error) {
	query := "SELECT `item_code`, SUM(`actual_qty` * `valuation_rate`) / SUM(`actual_qty`) AS val_rate" +
		" FROM `tabBin` WHERE `actual_qty` > 0 GROUP BY `item_code`"
	rows, err := r.DB.Query(query)
	if err != nil {
		return nil, fmt.Errorf("valuation rate query: %w", err)
	}
	defer rows.Close()

	result := make(map[string]float64)
	for rows.Next() {
		var itemCode string
		var valRate float64
		if err := rows.Scan(&itemCode, &valRate); err != nil {
			return nil, err
		}
		result[itemCode] = valRate
	}
	return result, nil
}

// ---------------------------------------------------------------------------
// 2D.2b — Total Stock Summary
// ---------------------------------------------------------------------------

// TotalStockSummaryReport implements the Total Stock Summary report.
type TotalStockSummaryReport struct {
	DB *db.DB
}

// Execute runs the Total Stock Summary report.
func (r *TotalStockSummaryReport) Execute(filters map[string]interface{}) (*ReportResult, error) {
	groupBy, _ := filters["group_by"].(string)

	var columns []Column
	if groupBy == "Warehouse" {
		columns = ParseColumnShorthands([]string{
			"Warehouse:Link/Warehouse:150",
			"Item:Link/Item:150",
			"Description::300",
			"Current Qty:Float:100",
		})
	} else {
		columns = ParseColumnShorthands([]string{
			"Company:Link/Company:250",
			"Item:Link/Item:150",
			"Description::300",
			"Current Qty:Float:100",
		})
	}

	var query string
	var args []interface{}

	if groupBy == "Warehouse" {
		query = "SELECT b.warehouse, i.item_code, i.description, SUM(b.actual_qty)" +
			" FROM `tabBin` b" +
			" INNER JOIN `tabItem` i ON b.item_code = i.item_code" +
			" INNER JOIN `tabWarehouse` w ON w.name = b.warehouse" +
			" WHERE b.actual_qty != 0"

		if company, ok := filters["company"].(string); ok && company != "" {
			query += " AND w.company = ?"
			args = append(args, company)
		}
		query += " GROUP BY b.warehouse, i.item_code"
	} else {
		query = "SELECT w.company, i.item_code, i.description, SUM(b.actual_qty)" +
			" FROM `tabBin` b" +
			" INNER JOIN `tabItem` i ON b.item_code = i.item_code" +
			" INNER JOIN `tabWarehouse` w ON w.name = b.warehouse" +
			" WHERE b.actual_qty != 0" +
			" GROUP BY w.company, i.item_code"
	}

	data, err := queryToRows(r.DB, query, args...)
	if err != nil {
		return nil, err
	}

	return &ReportResult{Columns: columns, Result: data}, nil
}

// ---------------------------------------------------------------------------
// 2D.2c — Batch Item Expiry Status
// ---------------------------------------------------------------------------

// BatchItemExpiryStatusReport implements the Batch Item Expiry Status report.
type BatchItemExpiryStatusReport struct {
	DB *db.DB
}

// Execute runs the Batch Item Expiry Status report.
func (r *BatchItemExpiryStatusReport) Execute(filters map[string]interface{}) (*ReportResult, error) {
	fromDate, _ := filters["from_date"].(string)
	toDate, _ := filters["to_date"].(string)

	if fromDate == "" {
		return nil, fmt.Errorf("'From Date' is required")
	}
	if toDate == "" {
		return nil, fmt.Errorf("'To Date' is required")
	}

	columns := ParseColumnShorthands([]string{
		"Item:Link/Item:150",
		"Item Name::150",
		"Batch:Link/Batch:150",
		"Stock UOM:Link/UOM:100",
		"Quantity:Float:100",
		"Expires On:Date:100",
		"Expiry (In Days):Int:130",
	})

	query := "SELECT `item`, `item_name`, `name`, `stock_uom`, `batch_qty`, `expiry_date`" +
		" FROM `tabBatch`" +
		" WHERE `disabled` = 0 AND `batch_qty` > 0" +
		" AND DATE(`creation`) >= ? AND DATE(`creation`) <= ?" +
		" ORDER BY `creation`"
	args := []interface{}{fromDate, toDate}

	if item, ok := filters["item"].(string); ok && item != "" {
		query = "SELECT `item`, `item_name`, `name`, `stock_uom`, `batch_qty`, `expiry_date`" +
			" FROM `tabBatch`" +
			" WHERE `disabled` = 0 AND `batch_qty` > 0" +
			" AND DATE(`creation`) >= ? AND DATE(`creation`) <= ?" +
			" AND `item` = ?" +
			" ORDER BY `creation`"
		args = append(args, item)
	}

	rows, err := r.DB.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("batch query: %w", err)
	}
	defer rows.Close()

	today := time.Now().UTC().Truncate(24 * time.Hour)
	var data [][]interface{}

	for rows.Next() {
		var itemCode, itemName, batchName, stockUOM string
		var batchQty float64
		var expiryDate *string

		if err := rows.Scan(&itemCode, &itemName, &batchName, &stockUOM, &batchQty, &expiryDate); err != nil {
			return nil, err
		}

		var expiresOn interface{}
		var expiryDays interface{}

		if expiryDate != nil && *expiryDate != "" {
			ed := frappe.Getdate(*expiryDate)
			if !ed.IsZero() {
				expiresOn = *expiryDate
				days := int(ed.Sub(today).Hours() / 24)
				if days < 0 {
					days = 0
				}
				expiryDays = days
			}
		}

		data = append(data, []interface{}{
			itemCode, itemName, batchName, stockUOM, batchQty, expiresOn, expiryDays,
		})
	}

	return &ReportResult{Columns: columns, Result: data}, nil
}

// ---------------------------------------------------------------------------
// 2D.2d — Share Ledger
// ---------------------------------------------------------------------------

// ShareLedgerReport implements the Share Ledger report.
type ShareLedgerReport struct {
	DB *db.DB
}

// Execute runs the Share Ledger report.
func (r *ShareLedgerReport) Execute(filters map[string]interface{}) (*ReportResult, error) {
	date, _ := filters["date"].(string)
	if date == "" {
		return nil, fmt.Errorf("please select date")
	}

	shareholder, _ := filters["shareholder"].(string)

	columns := ParseColumnShorthands([]string{
		"Shareholder:Link/Shareholder:150",
		"Date:Date:100",
		"Transfer Type::140",
		"Share Type::90",
		"No of Shares::90",
		"Rate:Currency:90",
		"Amount:Currency:90",
		"Company::150",
		"Share Transfer:Link/Share Transfer:90",
	})

	var data [][]interface{}
	if shareholder == "" {
		return &ReportResult{Columns: columns, Result: data}, nil
	}

	query := "SELECT * FROM `tabShare Transfer`" +
		" WHERE ((DATE(`date`) <= ? AND `from_shareholder` = ?)" +
		" OR (DATE(`date`) <= ? AND `to_shareholder` = ?))" +
		" AND `docstatus` = 1" +
		" ORDER BY `date`"

	rows, err := r.DB.Query(query, date, shareholder, date, shareholder)
	if err != nil {
		return nil, fmt.Errorf("share ledger query: %w", err)
	}
	defer rows.Close()

	colNames, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	colIndex := make(map[string]int)
	for i, name := range colNames {
		colIndex[name] = i
	}

	for rows.Next() {
		vals := make([]interface{}, len(colNames))
		ptrs := make([]interface{}, len(colNames))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}

		getStr := func(field string) string {
			if idx, ok := colIndex[field]; ok && vals[idx] != nil {
				if b, ok := vals[idx].([]byte); ok {
					return string(b)
				}
				return fmt.Sprintf("%v", vals[idx])
			}
			return ""
		}
		getVal := func(field string) interface{} {
			if idx, ok := colIndex[field]; ok {
				if b, bOK := vals[idx].([]byte); bOK {
					return string(b)
				}
				return vals[idx]
			}
			return nil
		}

		transferType := getStr("transfer_type")
		if transferType == "Transfer" {
			fromSH := getStr("from_shareholder")
			toSH := getStr("to_shareholder")
			if fromSH == shareholder {
				transferType += " to " + toSH
			} else {
				transferType += " from " + fromSH
			}
		}

		row := []interface{}{
			shareholder,
			getVal("date"),
			transferType,
			getVal("share_type"),
			getVal("no_of_shares"),
			getVal("rate"),
			getVal("amount"),
			getVal("company"),
			getVal("name"),
		}
		data = append(data, row)
	}

	return &ReportResult{Columns: columns, Result: data}, nil
}

// ---------------------------------------------------------------------------
// 2D.2e — Share Balance
// ---------------------------------------------------------------------------

// ShareBalanceReport implements the Share Balance report.
type ShareBalanceReport struct {
	DB *db.DB
}

// Execute runs the Share Balance report.
func (r *ShareBalanceReport) Execute(filters map[string]interface{}) (*ReportResult, error) {
	date, _ := filters["date"].(string)
	if date == "" {
		return nil, fmt.Errorf("please select date")
	}

	shareholder, _ := filters["shareholder"].(string)

	columns := ParseColumnShorthands([]string{
		"Shareholder:Link/Shareholder:150",
		"Share Type::90",
		"No of Shares::90",
		"Average Rate:Currency:90",
		"Amount:Currency:90",
	})

	var data [][]interface{}
	if shareholder == "" {
		return &ReportResult{Columns: columns, Result: data}, nil
	}

	// Fetch share balance from Shareholder child table
	query := "SELECT `share_type`, `no_of_shares`, `rate`, `amount`" +
		" FROM `tabShare Balance`" +
		" WHERE `parent` = ? AND `parenttype` = 'Shareholder'"

	rows, err := r.DB.Query(query, shareholder)
	if err != nil {
		return nil, fmt.Errorf("share balance query: %w", err)
	}
	defer rows.Close()

	type shareEntry struct {
		shareType  string
		noOfShares float64
		rate       float64
		amount     float64
	}

	var entries []shareEntry
	for rows.Next() {
		var e shareEntry
		if err := rows.Scan(&e.shareType, &e.noOfShares, &e.rate, &e.amount); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}

	// Aggregate by share type
	for _, entry := range entries {
		found := false
		for i, row := range data {
			if len(row) > 1 {
				if st, ok := row[1].(string); ok && st == entry.shareType {
					nos := toFloat64(row[2]) + entry.noOfShares
					amt := toFloat64(row[4]) + entry.amount
					var avgRate float64
					if nos != 0 {
						avgRate = amt / nos
					}
					data[i][2] = nos
					data[i][3] = avgRate
					data[i][4] = amt
					found = true
					break
				}
			}
		}
		if !found {
			data = append(data, []interface{}{
				shareholder,
				entry.shareType,
				entry.noOfShares,
				entry.rate,
				entry.amount,
			})
		}
	}

	return &ReportResult{Columns: columns, Result: data}, nil
}

// ---------------------------------------------------------------------------
// 2D.2f — Available Serial No
// ---------------------------------------------------------------------------

// AvailableSerialNoReport implements the Available Serial No report.
type AvailableSerialNoReport struct {
	DB *db.DB
}

// Execute runs the Available Serial No report.
func (r *AvailableSerialNoReport) Execute(filters map[string]interface{}) (*ReportResult, error) {
	columns := []Column{
		{Label: "Date", Fieldname: "date", Fieldtype: "Datetime", Width: 150},
		{Label: "Item", Fieldname: "item_code", Fieldtype: "Link", Options: "Item", Width: 100},
		{Label: "Item Name", Fieldname: "item_name", Width: 100},
		{Label: "UOM", Fieldname: "stock_uom", Fieldtype: "Link", Options: "UOM", Width: 60},
		{Label: "In Qty", Fieldname: "in_qty", Fieldtype: "Float", Width: 80},
		{Label: "Out Qty", Fieldname: "out_qty", Fieldtype: "Float", Width: 80},
		{Label: "Balance Qty", Fieldname: "qty_after_transaction", Fieldtype: "Float", Width: 100},
		{Label: "Warehouse", Fieldname: "warehouse", Fieldtype: "Link", Options: "Warehouse", Width: 150},
		{Label: "Serial No (In/Out)", Fieldname: "serial_no", Width: 150},
		{Label: "Balance Serial No", Fieldname: "balance_serial_no", Width: 150},
		{Label: "Incoming Rate", Fieldname: "incoming_rate", Fieldtype: "Currency", Width: 110, Options: "Company:company:default_currency"},
		{Label: "Avg Rate (Balance Stock)", Fieldname: "valuation_rate", Fieldtype: "Currency", Width: 180, Options: "Company:company:default_currency"},
		{Label: "Valuation Rate", Fieldname: "in_out_rate", Fieldtype: "Currency", Width: 140, Options: "Company:company:default_currency"},
		{Label: "Balance Value", Fieldname: "stock_value", Fieldtype: "Currency", Width: 110, Options: "Company:company:default_currency"},
		{Label: "Value Change", Fieldname: "stock_value_difference", Fieldtype: "Currency", Width: 110, Options: "Company:company:default_currency"},
		{Label: "Serial and Batch Bundle", Fieldname: "serial_and_batch_bundle", Fieldtype: "Link", Options: "Serial and Batch Bundle", Width: 100},
		{Label: "Voucher Type", Fieldname: "voucher_type", Width: 110},
		{Label: "Voucher #", Fieldname: "voucher_no", Fieldtype: "Dynamic Link", Options: "voucher_type", Width: 100},
		{Label: "Company", Fieldname: "company", Fieldtype: "Link", Options: "Company", Width: 110},
	}

	// Get items with serial nos
	itemQuery := "SELECT `name` FROM `tabItem` WHERE `has_serial_no` = 1"
	var itemArgs []interface{}
	if itemCode, ok := filters["item_code"].(string); ok && itemCode != "" {
		itemQuery += " AND `name` = ?"
		itemArgs = append(itemArgs, itemCode)
	}

	itemRows, err := r.DB.Query(itemQuery, itemArgs...)
	if err != nil {
		return nil, fmt.Errorf("items query: %w", err)
	}
	defer itemRows.Close()

	var items []string
	for itemRows.Next() {
		var name string
		if err := itemRows.Scan(&name); err != nil {
			return nil, err
		}
		items = append(items, name)
	}

	if len(items) == 0 {
		return &ReportResult{Columns: columns, Result: nil}, nil
	}

	// Get stock ledger entries for these items
	placeholders := make([]string, len(items))
	sleArgs := make([]interface{}, len(items))
	for i, item := range items {
		placeholders[i] = "?"
		sleArgs[i] = item
	}

	sleQuery := fmt.Sprintf(
		"SELECT `posting_date`, `item_code`, `warehouse`, `actual_qty`, `qty_after_transaction`,"+
			" `incoming_rate`, `valuation_rate`, `stock_value`, `stock_value_difference`,"+
			" `voucher_type`, `voucher_no`, `serial_no`, `serial_and_batch_bundle`, `company`"+
			" FROM `tabStock Ledger Entry`"+
			" WHERE `item_code` IN (%s) AND `is_cancelled` = 0"+
			" ORDER BY `posting_date`, `posting_time`, `creation`",
		strings.Join(placeholders, ","),
	)

	sleRows, err := r.DB.Query(sleQuery, sleArgs...)
	if err != nil {
		return nil, fmt.Errorf("stock ledger query: %w", err)
	}
	defer sleRows.Close()

	// Get item details
	itemDetailQuery := fmt.Sprintf(
		"SELECT `name`, `item_name`, `stock_uom` FROM `tabItem` WHERE `name` IN (%s)",
		strings.Join(placeholders, ","),
	)
	detailRows, err := r.DB.Query(itemDetailQuery, sleArgs...)
	if err != nil {
		return nil, fmt.Errorf("item detail query: %w", err)
	}
	defer detailRows.Close()

	itemDetails := make(map[string]map[string]string)
	for detailRows.Next() {
		var name, itemName, stockUOM string
		if err := detailRows.Scan(&name, &itemName, &stockUOM); err != nil {
			return nil, err
		}
		itemDetails[name] = map[string]string{
			"item_name": itemName,
			"stock_uom": stockUOM,
		}
	}

	var data [][]interface{}
	for sleRows.Next() {
		var postingDate, itemCode, warehouse, voucherType, voucherNo, company string
		var serialNo, sabb *string
		var actualQty, qtyAfterTxn, incomingRate, valuationRate, stockValue, stockValueDiff float64

		if err := sleRows.Scan(&postingDate, &itemCode, &warehouse, &actualQty, &qtyAfterTxn,
			&incomingRate, &valuationRate, &stockValue, &stockValueDiff,
			&voucherType, &voucherNo, &serialNo, &sabb, &company); err != nil {
			return nil, err
		}

		detail := itemDetails[itemCode]
		itemName := ""
		stockUOM := ""
		if detail != nil {
			itemName = detail["item_name"]
			stockUOM = detail["stock_uom"]
		}

		inQty := math.Max(actualQty, 0)
		outQty := math.Min(actualQty, 0)

		var inOutRate float64
		if actualQty != 0 {
			inOutRate = stockValueDiff / actualQty
		}

		serialNoStr := ""
		if serialNo != nil {
			serialNoStr = *serialNo
		}
		sabbStr := ""
		if sabb != nil {
			sabbStr = *sabb
		}

		data = append(data, []interface{}{
			postingDate, itemCode, itemName, stockUOM,
			inQty, outQty, qtyAfterTxn,
			warehouse, serialNoStr, "", // balance_serial_no computed separately if needed
			incomingRate, valuationRate, inOutRate,
			stockValue, stockValueDiff, sabbStr,
			voucherType, voucherNo, company,
		})
	}

	return &ReportResult{Columns: columns, Result: data}, nil
}

// ---------------------------------------------------------------------------
// 2D.2g — Inactive Sales Items
// ---------------------------------------------------------------------------

// InactiveSalesItemsReport implements the Inactive Sales Items report.
type InactiveSalesItemsReport struct {
	DB *db.DB
}

// Execute runs the Inactive Sales Items report.
func (r *InactiveSalesItemsReport) Execute(filters map[string]interface{}) (*ReportResult, error) {
	columns := []Column{
		{Label: "Territory", Fieldname: "territory", Fieldtype: "Link", Options: "Territory", Width: 100},
		{Label: "Item Group", Fieldname: "item_group", Fieldtype: "Link", Options: "Item Group", Width: 150},
		{Label: "Item", Fieldname: "item", Fieldtype: "Link", Options: "Item", Width: 150},
		{Label: "Item Name", Fieldname: "item_name", Fieldtype: "Data", Width: 150},
		{Label: "Customer", Fieldname: "customer", Fieldtype: "Link", Options: "Customer", Width: 100},
		{Label: "Last Order Date", Fieldname: "last_order_date", Fieldtype: "Date", Width: 100},
		{Label: "Quantity", Fieldname: "qty", Fieldtype: "Float", Width: 100},
		{Label: "Days Since Last Order", Fieldname: "days_since_last_order", Fieldtype: "Int", Width: 100},
	}

	basedOn, _ := filters["based_on"].(string)
	if basedOn == "" {
		basedOn = "Sales Order"
	}

	days := frappe.Cint(filters["days"])

	// Get territories
	territories, err := r.getTerritories(filters)
	if err != nil {
		return nil, err
	}

	// Get items
	items, err := r.getItems(filters)
	if err != nil {
		return nil, err
	}

	// Get sales details
	salesData, err := r.getSalesDetails(basedOn)
	if err != nil {
		return nil, err
	}

	var data [][]interface{}
	for _, territory := range territories {
		for _, item := range items {
			row := []interface{}{
				territory,
				item["item_group"],
				item["item_code"],
				item["item_name"],
				nil, nil, nil, nil,
			}

			key := territory + "|" + item["item_code"]
			if sd, ok := salesData[key]; ok {
				if sd.daysSinceLastOrder > days {
					row[4] = sd.customer
					row[5] = sd.lastOrderDate
					row[6] = sd.qty
					row[7] = sd.daysSinceLastOrder
				} else {
					continue
				}
			}

			data = append(data, row)
		}
	}

	return &ReportResult{Columns: columns, Result: data}, nil
}

type salesDetail struct {
	territory          string
	customer           string
	lastOrderDate      string
	qty                float64
	daysSinceLastOrder int
}

func (r *InactiveSalesItemsReport) getSalesDetails(basedOn string) (map[string]salesDetail, error) {
	dateField := "s.posting_date"
	doctype := "Sales Invoice"
	if basedOn == "Sales Order" {
		dateField = "s.transaction_date"
		doctype = "Sales Order"
	}

	query := fmt.Sprintf(
		"SELECT s.territory, s.customer, si.item_code, si.qty, %s AS last_order_date,"+
			" DATEDIFF(CURRENT_DATE, %s) AS days_since_last_order"+
			" FROM `tab%s` s, `tab%s Item` si"+
			" WHERE s.name = si.parent AND s.docstatus = 1"+
			" ORDER BY days_since_last_order",
		dateField, dateField, doctype, doctype,
	)

	rows, err := r.DB.Query(query)
	if err != nil {
		return nil, fmt.Errorf("sales details query: %w", err)
	}
	defer rows.Close()

	result := make(map[string]salesDetail)
	for rows.Next() {
		var territory, customer, itemCode, lastOrderDate string
		var qty float64
		var daysSince int

		if err := rows.Scan(&territory, &customer, &itemCode, &qty, &lastOrderDate, &daysSince); err != nil {
			return nil, err
		}

		key := territory + "|" + itemCode
		if _, exists := result[key]; !exists {
			result[key] = salesDetail{
				territory:          territory,
				customer:           customer,
				lastOrderDate:      lastOrderDate,
				qty:                qty,
				daysSinceLastOrder: daysSince,
			}
		}
	}
	return result, nil
}

func (r *InactiveSalesItemsReport) getTerritories(filters map[string]interface{}) ([]string, error) {
	query := "SELECT `name` FROM `tabTerritory`"
	var args []interface{}
	if territory, ok := filters["territory"].(string); ok && territory != "" {
		query += " WHERE `name` = ?"
		args = append(args, territory)
	}

	rows, err := r.DB.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("territories query: %w", err)
	}
	defer rows.Close()

	var result []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		result = append(result, name)
	}
	return result, nil
}

func (r *InactiveSalesItemsReport) getItems(filters map[string]interface{}) ([]map[string]string, error) {
	query := "SELECT `name`, `item_group`, `item_name`, `item_code` FROM `tabItem` WHERE `disabled` = 0 AND `is_stock_item` = 1"
	var args []interface{}

	if ig, ok := filters["item_group"].(string); ok && ig != "" {
		query += " AND `item_group` = ?"
		args = append(args, ig)
	}
	if item, ok := filters["item"].(string); ok && item != "" {
		query += " AND `name` = ?"
		args = append(args, item)
	}
	query += " ORDER BY `name`"

	rows, err := r.DB.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("items query: %w", err)
	}
	defer rows.Close()

	var result []map[string]string
	for rows.Next() {
		var name, itemGroup, itemName, itemCode string
		if err := rows.Scan(&name, &itemGroup, &itemName, &itemCode); err != nil {
			return nil, err
		}
		result = append(result, map[string]string{
			"name":       name,
			"item_group": itemGroup,
			"item_name":  itemName,
			"item_code":  itemCode,
		})
	}
	return result, nil
}

// ---------------------------------------------------------------------------
// 2D.2h — Bank Clearance Summary
// ---------------------------------------------------------------------------

// BankClearanceSummaryReport implements the Bank Clearance Summary report.
type BankClearanceSummaryReport struct {
	DB *db.DB
}

// Execute runs the Bank Clearance Summary report.
func (r *BankClearanceSummaryReport) Execute(filters map[string]interface{}) (*ReportResult, error) {
	columns := []Column{
		{Label: "Payment Document Type", Fieldname: "payment_document_type", Fieldtype: "Data", Width: 130},
		{Label: "Payment Entry", Fieldname: "payment_entry", Fieldtype: "Dynamic Link", Options: "payment_document_type", Width: 140},
		{Label: "Posting Date", Fieldname: "posting_date", Fieldtype: "Date", Width: 120},
		{Label: "Cheque/Reference No", Fieldname: "cheque_no", Width: 120},
		{Label: "Clearance Date", Fieldname: "clearance_date", Fieldtype: "Date", Width: 120},
		{Label: "Against Account", Fieldname: "against", Fieldtype: "Link", Options: "Account", Width: 200},
		{Label: "Amount", Fieldname: "amount", Fieldtype: "Currency", Width: 120},
	}

	account, _ := filters["account"].(string)
	fromDate, _ := filters["from_date"].(string)
	toDate, _ := filters["to_date"].(string)

	if account == "" || fromDate == "" || toDate == "" {
		return &ReportResult{Columns: columns, Result: nil}, nil
	}

	var allEntries [][]interface{}

	// Journal Entries
	jeQuery := "SELECT 'Journal Entry' AS payment_document, je.name AS payment_entry," +
		" je.posting_date, je.cheque_no, je.clearance_date," +
		" jea.against_account," +
		" (jea.debit_in_account_currency - jea.credit_in_account_currency) AS amount" +
		" FROM `tabJournal Entry Account` jea" +
		" INNER JOIN `tabJournal Entry` je ON jea.parent = je.name" +
		" WHERE jea.account = ? AND je.docstatus = 1" +
		" AND je.posting_date >= ? AND je.posting_date <= ?" +
		" AND (je.is_opening = 'No' OR je.is_opening IS NULL)" +
		" ORDER BY je.posting_date DESC, je.name DESC"
	jeData, err := queryToRows(r.DB, jeQuery, account, fromDate, toDate)
	if err != nil {
		return nil, err
	}
	allEntries = append(allEntries, jeData...)

	// Payment Entries
	peQuery := "SELECT 'Payment Entry' AS payment_document, pe.name AS payment_entry," +
		" pe.posting_date, pe.reference_no AS cheque_no, pe.clearance_date," +
		" pe.party AS against_account," +
		" CASE WHEN pe.paid_from = ? THEN ((pe.paid_amount * -1) - pe.total_taxes_and_charges) ELSE pe.received_amount END AS amount" +
		" FROM `tabPayment Entry` pe" +
		" WHERE (pe.paid_from = ? OR pe.paid_to = ?)" +
		" AND pe.docstatus = 1 AND pe.posting_date >= ? AND pe.posting_date <= ?" +
		" ORDER BY pe.posting_date DESC, pe.name DESC"
	peData, err := queryToRows(r.DB, peQuery, account, account, account, fromDate, toDate)
	if err != nil {
		return nil, err
	}
	allEntries = append(allEntries, peData...)

	// Purchase Invoices (paid)
	piQuery := "SELECT 'Purchase Invoice' AS payment_document, pi.name AS payment_entry," +
		" pi.posting_date, pi.bill_no AS cheque_no, pi.clearance_date," +
		" pi.supplier AS against_account," +
		" (pi.paid_amount * -1) AS amount" +
		" FROM `tabPurchase Invoice` pi" +
		" WHERE pi.docstatus = 1 AND pi.is_paid = 1 AND pi.cash_bank_account = ?" +
		" AND pi.posting_date >= ? AND pi.posting_date <= ?" +
		" ORDER BY pi.posting_date DESC, pi.name DESC"
	piData, err := queryToRows(r.DB, piQuery, account, fromDate, toDate)
	if err != nil {
		return nil, err
	}
	allEntries = append(allEntries, piData...)

	// Sort by posting_date (index 2)
	sort.SliceStable(allEntries, func(i, j int) bool {
		di := fmt.Sprintf("%v", allEntries[i][2])
		dj := fmt.Sprintf("%v", allEntries[j][2])
		return di < dj
	})

	return &ReportResult{Columns: columns, Result: allEntries}, nil
}

// ---------------------------------------------------------------------------
// 2D.2i — BOM Stock Report
// ---------------------------------------------------------------------------

// BOMStockReport implements the BOM Stock Report.
type BOMStockReport struct {
	DB *db.DB
}

// Execute runs the BOM Stock Report.
func (r *BOMStockReport) Execute(filters map[string]interface{}) (*ReportResult, error) {
	columns := ParseColumnShorthands([]string{
		"Item:Link/Item:150",
		"Item Name::240",
		"Description::300",
		"From BOM No::200",
		"BOM Qty:Float:160",
		"BOM UOM::160",
		"Required Qty:Float:120",
		"In Stock Qty:Float:120",
		"Enough Parts to Build:Float:200",
	})

	qtyToProduce := frappe.FltFromAny(filters["qty_to_produce"], -1)
	if qtyToProduce <= 0 {
		return nil, fmt.Errorf("Quantity to Produce should be greater than zero")
	}

	bom, _ := filters["bom"].(string)
	if bom == "" {
		return nil, fmt.Errorf("BOM is required")
	}

	warehouse, _ := filters["warehouse"].(string)
	showExploded := false
	if v, ok := filters["show_exploded_view"]; ok {
		if b, ok := v.(bool); ok {
			showExploded = b
		}
	}

	bomItemTable := "BOM Item"
	if showExploded {
		bomItemTable = "BOM Explosion Item"
	}

	// Build the query
	// Get warehouse details for tree-based filtering
	var warehouseCondition string
	var whArgs []interface{}

	if warehouse != "" {
		// Check if warehouse has lft/rgt for tree-based filtering
		whQuery := "SELECT `lft`, `rgt` FROM `tabWarehouse` WHERE `name` = ? LIMIT 1"
		whRows, err := r.DB.Query(whQuery, warehouse)
		if err != nil {
			return nil, fmt.Errorf("warehouse query: %w", err)
		}

		var lft, rgt int
		hasTree := false
		if whRows.Next() {
			if err := whRows.Scan(&lft, &rgt); err == nil && lft > 0 && rgt > 0 {
				hasTree = true
			}
		}
		whRows.Close()

		if hasTree {
			warehouseCondition = " INNER JOIN `tabWarehouse` wh ON bin.warehouse = wh.name WHERE wh.lft >= ? AND wh.rgt <= ?"
			whArgs = append(whArgs, lft, rgt)
		} else {
			warehouseCondition = " WHERE bin.warehouse = ?"
			whArgs = append(whArgs, warehouse)
		}
	}

	// Build the main query
	query := fmt.Sprintf(
		"SELECT bi.item_code, bi.item_name, bi.description, b.name AS bom_name,"+
			" SUM(bi.stock_qty) AS bom_qty, bi.stock_uom,"+
			" (SUM(bi.stock_qty) * ?) / b.quantity AS required_qty,"+
			" IFNULL(stock.actual_qty, 0) AS in_stock_qty,"+
			" FLOOR(IFNULL(stock.actual_qty, 0) / ((SUM(bi.stock_qty) * ?) / b.quantity)) AS enough_parts"+
			" FROM `tabBOM` b"+
			" INNER JOIN `tab%s` bi ON b.name = bi.parent"+
			" LEFT JOIN ("+
			"   SELECT bin.item_code, SUM(bin.actual_qty) AS actual_qty FROM `tabBin` bin%s GROUP BY bin.item_code"+
			" ) stock ON bi.item_code = stock.item_code"+
			" WHERE bi.parent = ? AND bi.parenttype = 'BOM'"+
			" GROUP BY bi.item_code"+
			" ORDER BY bi.idx",
		bomItemTable,
		warehouseCondition,
	)

	args := []interface{}{qtyToProduce, qtyToProduce}
	args = append(args, whArgs...)
	args = append(args, bom)

	data, err := queryToRows(r.DB, query, args...)
	if err != nil {
		return nil, err
	}

	return &ReportResult{Columns: columns, Result: data}, nil
}

// ---------------------------------------------------------------------------
// queryToRows is a helper that executes a query and returns all rows as [][]interface{}.
// ---------------------------------------------------------------------------
func queryToRows(database *db.DB, query string, args ...interface{}) ([][]interface{}, error) {
	rows, err := database.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query error: %w", err)
	}
	defer rows.Close()

	colTypes, err := rows.ColumnTypes()
	if err != nil {
		return nil, fmt.Errorf("getting column types: %w", err)
	}
	numCols := len(colTypes)

	var data [][]interface{}
	for rows.Next() {
		vals := make([]interface{}, numCols)
		ptrs := make([]interface{}, numCols)
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, fmt.Errorf("scanning row: %w", err)
		}
		row := make([]interface{}, numCols)
		for i, v := range vals {
			if b, ok := v.([]byte); ok {
				row[i] = string(b)
			} else {
				row[i] = v
			}
		}
		data = append(data, row)
	}
	return data, nil
}
