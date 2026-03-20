package reports

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/alex-baker-cdm/erpnext/go-erpnext/pkg/db"
	"github.com/alex-baker-cdm/erpnext/go-erpnext/pkg/frappe"
)

// ---------------------------------------------------------------------------
// 2E.1a  Stock Ledger Report (most complex)
// Source: erpnext/stock/report/stock_ledger/stock_ledger.py
// ---------------------------------------------------------------------------

// StockLedgerReport implements the Stock Ledger report.
type StockLedgerReport struct {
	DB *db.DB
}

func (r *StockLedgerReport) Execute(filters map[string]interface{}) (*ReportResult, error) {
	_ = IsRepostingItemValuationInProgress()

	columns := r.getColumns(filters)
	items := r.getItems(filters)
	slEntries, err := r.getStockLedgerEntries(filters, items)
	if err != nil {
		return nil, fmt.Errorf("stock ledger entries: %w", err)
	}

	itemDetails, err := r.getItemDetails(items, slEntries)
	if err != nil {
		return nil, fmt.Errorf("item details: %w", err)
	}

	// TODO: get_inventory_dimensions() - skip for now, return empty
	// TODO: get_inv_dimension_wise_value() - skip for now

	openingRow := r.getOpeningBalance(filters, slEntries)

	precision := 6 // default float precision

	data := []map[string]interface{}{}
	if openingRow != nil {
		data = append(data, openingRow)
	}

	actualQty := 0.0
	stockValue := 0.0
	if openingRow != nil {
		actualQty = frappe.FltFromAny(openingRow["qty_after_transaction"], -1)
		stockValue = frappe.FltFromAny(openingRow["stock_value"], -1)
	}

	batchBalanceDict := map[string][2]float64{}
	batchNoFilter, _ := filters["batch_no"].(string)
	if actualQty != 0 && batchNoFilter != "" {
		batchBalanceDict[batchNoFilter] = [2]float64{actualQty, stockValue}
	}

	for _, sle := range slEntries {
		itemCode, _ := sle["item_code"].(string)
		if detail, ok := itemDetails[itemCode]; ok {
			for k, v := range detail {
				if _, exists := sle[k]; !exists {
					sle[k] = v
				}
			}
		}

		sleActualQty := frappe.FltFromAny(sle["actual_qty"], -1)

		if batchNoFilter != "" {
			actualQty += frappe.Flt(sleActualQty, precision)
			stockValue += frappe.FltFromAny(sle["stock_value_difference"], -1)

			sleBatchNo, _ := sle["batch_no"].(string)
			if sleBatchNo != "" {
				bal := batchBalanceDict[sleBatchNo]
				bal[0] += sleActualQty
				bal[1] += stockValue
				batchBalanceDict[sleBatchNo] = bal
			}

			voucherType, _ := sle["voucher_type"].(string)
			if voucherType == "Stock Reconciliation" && sleActualQty == 0 {
				actualQty = frappe.FltFromAny(sle["qty_after_transaction"], -1)
				stockValue = frappe.FltFromAny(sle["stock_value"], -1)
			}

			sle["qty_after_transaction"] = actualQty
			sle["stock_value"] = stockValue
		}

		inQty := math.Max(sleActualQty, 0)
		outQty := math.Min(sleActualQty, 0)
		sle["in_qty"] = inQty
		sle["out_qty"] = outQty

		if sleActualQty != 0 {
			svd := frappe.FltFromAny(sle["stock_value_difference"], -1)
			sle["in_out_rate"] = frappe.Flt(svd/sleActualQty, precision)
		} else {
			voucherType, _ := sle["voucher_type"].(string)
			if voucherType == "Stock Reconciliation" {
				sle["in_out_rate"] = sle["valuation_rate"]
			}
		}

		data = append(data, sle)
	}

	// Convert to [][]interface{} for ReportResult
	result := make([][]interface{}, len(data))
	for i, row := range data {
		rowData := make([]interface{}, len(columns))
		for j, col := range columns {
			rowData[j] = row[col.Fieldname]
		}
		result[i] = rowData
	}

	return &ReportResult{
		Columns: columns,
		Result:  result,
	}, nil
}

func (r *StockLedgerReport) getColumns(filters map[string]interface{}) []Column {
	valuationFieldType := "Currency"
	if vft, ok := filters["valuation_field_type"].(string); ok && vft != "" {
		valuationFieldType = vft
	}

	columns := []Column{
		{Label: "Date", Fieldname: "date", Fieldtype: "Datetime", Width: 150},
		{Label: "Item", Fieldname: "item_code", Fieldtype: "Link", Options: "Item", Width: 100},
		{Label: "Item Name", Fieldname: "item_name", Width: 100},
		{Label: "Stock UOM", Fieldname: "stock_uom", Fieldtype: "Link", Options: "UOM", Width: 90},
		// TODO: inventory dimension columns from get_inventory_dimensions()
		{Label: "In Qty", Fieldname: "in_qty", Fieldtype: "Float", Width: 80},
		{Label: "Out Qty", Fieldname: "out_qty", Fieldtype: "Float", Width: 80},
		{Label: "Balance Qty", Fieldname: "qty_after_transaction", Fieldtype: "Float", Width: 100},
		{Label: "Warehouse", Fieldname: "warehouse", Fieldtype: "Link", Options: "Warehouse", Width: 150},
		{Label: "Item Group", Fieldname: "item_group", Fieldtype: "Link", Options: "Item Group", Width: 100},
		{Label: "Brand", Fieldname: "brand", Fieldtype: "Link", Options: "Brand", Width: 100},
		{Label: "Description", Fieldname: "description", Width: 200},
		{Label: "Incoming Rate", Fieldname: "incoming_rate", Fieldtype: "Currency", Options: "Company:company:default_currency", Width: 110},
		{Label: "Avg Rate (Balance Stock)", Fieldname: "valuation_rate", Fieldtype: valuationFieldType, Width: 180},
		{Label: "Valuation Rate", Fieldname: "in_out_rate", Fieldtype: valuationFieldType, Width: 140},
		{Label: "Balance Value", Fieldname: "stock_value", Fieldtype: "Currency", Options: "Company:company:default_currency", Width: 110},
		{Label: "Value Change", Fieldname: "stock_value_difference", Fieldtype: "Currency", Options: "Company:company:default_currency", Width: 110},
		{Label: "Voucher Type", Fieldname: "voucher_type", Width: 110},
		{Label: "Voucher #", Fieldname: "voucher_no", Fieldtype: "Dynamic Link", Options: "voucher_type", Width: 100},
		{Label: "Batch", Fieldname: "batch_no", Fieldtype: "Link", Options: "Batch", Width: 100},
		{Label: "Serial No", Fieldname: "serial_no", Fieldtype: "Link", Options: "Serial No", Width: 100},
		{Label: "Serial and Batch Bundle", Fieldname: "serial_and_batch_bundle", Fieldtype: "Link", Options: "Serial and Batch Bundle", Width: 100},
		{Label: "Project", Fieldname: "project", Fieldtype: "Link", Options: "Project", Width: 100},
		{Label: "Company", Fieldname: "company", Fieldtype: "Link", Options: "Company", Width: 110},
	}
	return columns
}

func (r *StockLedgerReport) getItems(filters map[string]interface{}) []string {
	if r.DB == nil {
		return nil
	}

	var items []string
	var conditions []string
	var args []interface{}

	if itemCodes, ok := filters["item_code"]; ok && itemCodes != nil {
		switch v := itemCodes.(type) {
		case string:
			if v != "" {
				conditions = append(conditions, "`name` = ?")
				args = append(args, v)
			}
		case []interface{}:
			if len(v) > 0 {
				placeholders := strings.Repeat("?,", len(v))
				placeholders = placeholders[:len(placeholders)-1]
				conditions = append(conditions, "`name` IN ("+placeholders+")")
				args = append(args, v...)
			}
		}
	} else {
		if brand, ok := filters["brand"].(string); ok && brand != "" {
			conditions = append(conditions, "`brand` = ?")
			args = append(args, brand)
		}
		if itemGroup, ok := filters["item_group"].(string); ok && itemGroup != "" {
			conditions = append(conditions, "`item_group` = ?")
			args = append(args, itemGroup)
		}
	}

	if len(conditions) == 0 {
		return nil
	}

	query := "SELECT `name` FROM `tabItem` WHERE " + strings.Join(conditions, " AND ")
	rows, err := r.DB.Query(query, args...)
	if err != nil {
		return nil
	}
	defer rows.Close()

	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err == nil {
			items = append(items, name)
		}
	}
	return items
}

func (r *StockLedgerReport) getStockLedgerEntries(filters map[string]interface{}, items []string) ([]map[string]interface{}, error) {
	if r.DB == nil {
		return nil, nil
	}

	fromDate, _ := filters["from_date"].(string)
	toDate, _ := filters["to_date"].(string)
	if fromDate != "" {
		fromDate += " 00:00:00"
	}
	if toDate != "" {
		toDate += " 23:59:59"
	}

	query := `SELECT ` + "`item_code`, `posting_datetime` AS `date`, `warehouse`, `posting_date`, `posting_time`," +
		"`actual_qty`, `incoming_rate`, `valuation_rate`, `company`, `voucher_type`," +
		"`qty_after_transaction`, `stock_value_difference`, `serial_and_batch_bundle`," +
		"`voucher_no`, `stock_value`, `batch_no`, `serial_no`, `project`" +
		" FROM `tabStock Ledger Entry`" +
		" WHERE `docstatus` < 2 AND `is_cancelled` = 0"

	var args []interface{}

	if fromDate != "" && toDate != "" {
		query += " AND `posting_datetime` BETWEEN ? AND ?"
		args = append(args, fromDate, toDate)
	}

	if len(items) > 0 {
		placeholders := strings.Repeat("?,", len(items))
		placeholders = placeholders[:len(placeholders)-1]
		query += " AND `item_code` IN (" + placeholders + ")"
		for _, item := range items {
			args = append(args, item)
		}
	}

	for _, field := range []string{"voucher_no", "project", "company"} {
		if val, ok := filters[field].(string); ok && val != "" {
			query += fmt.Sprintf(" AND `%s` = ?", field)
			args = append(args, val)
		}
	}

	if batchNo, ok := filters["batch_no"].(string); ok && batchNo != "" {
		query += " AND `batch_no` = ?"
		args = append(args, batchNo)
	}

	if warehouse, ok := filters["warehouse"].(string); ok && warehouse != "" {
		query += " AND `warehouse` = ?"
		args = append(args, warehouse)
	}

	query += " ORDER BY `posting_datetime`, `creation`"

	rows, err := r.DB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanRowsToMaps(rows)
}

func (r *StockLedgerReport) getItemDetails(items []string, slEntries []map[string]interface{}) (map[string]map[string]interface{}, error) {
	result := make(map[string]map[string]interface{})
	if r.DB == nil {
		return result, nil
	}

	itemSet := make(map[string]bool)
	if len(items) > 0 {
		for _, item := range items {
			itemSet[item] = true
		}
	} else {
		for _, sle := range slEntries {
			if ic, ok := sle["item_code"].(string); ok {
				itemSet[ic] = true
			}
		}
	}

	if len(itemSet) == 0 {
		return result, nil
	}

	itemList := make([]string, 0, len(itemSet))
	for item := range itemSet {
		itemList = append(itemList, item)
	}

	placeholders := strings.Repeat("?,", len(itemList))
	placeholders = placeholders[:len(placeholders)-1]

	query := "SELECT `name`, `item_name`, `description`, `item_group`, `brand`, `stock_uom`" +
		" FROM `tabItem` WHERE `name` IN (" + placeholders + ")"

	args := make([]interface{}, len(itemList))
	for i, item := range itemList {
		args[i] = item
	}

	rows, err := r.DB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var name, itemName, description, itemGroup, brand, stockUOM string
		if err := rows.Scan(&name, &itemName, &description, &itemGroup, &brand, &stockUOM); err != nil {
			return nil, err
		}
		result[name] = map[string]interface{}{
			"item_name":   itemName,
			"description": description,
			"item_group":  itemGroup,
			"brand":       brand,
			"stock_uom":   stockUOM,
		}
	}
	return result, nil
}

func (r *StockLedgerReport) getOpeningBalance(filters map[string]interface{}, slEntries []map[string]interface{}) map[string]interface{} {
	itemCode, _ := filters["item_code"].(string)
	warehouse, _ := filters["warehouse"].(string)
	fromDate, _ := filters["from_date"].(string)

	if itemCode == "" || warehouse == "" || fromDate == "" {
		return nil
	}

	if r.DB == nil {
		return map[string]interface{}{
			"item_code":             "'Opening'",
			"qty_after_transaction": 0.0,
			"valuation_rate":        0.0,
			"stock_value":           0.0,
		}
	}

	query := "SELECT `qty_after_transaction`, `valuation_rate`, `stock_value`" +
		" FROM `tabStock Ledger Entry`" +
		" WHERE `item_code` = ? AND `warehouse` = ?" +
		" AND `posting_date` < ? AND `docstatus` < 2 AND `is_cancelled` = 0" +
		" ORDER BY `posting_datetime` DESC, `creation` DESC LIMIT 1"

	rows, err := r.DB.Query(query, itemCode, warehouse, fromDate)
	if err != nil {
		return nil
	}
	defer rows.Close()

	qtyAfter := 0.0
	valRate := 0.0
	stkValue := 0.0
	if rows.Next() {
		rows.Scan(&qtyAfter, &valRate, &stkValue)
	}

	return map[string]interface{}{
		"item_code":             "'Opening'",
		"qty_after_transaction": qtyAfter,
		"valuation_rate":        valRate,
		"stock_value":           stkValue,
	}
}

// ---------------------------------------------------------------------------
// 2E.1b  Stock Projected Qty Report
// Source: erpnext/stock/report/stock_projected_qty/stock_projected_qty.py
// ---------------------------------------------------------------------------

// StockProjectedQtyReport implements the Stock Projected Qty report.
type StockProjectedQtyReport struct {
	DB *db.DB
}

func (r *StockProjectedQtyReport) Execute(filters map[string]interface{}) (*ReportResult, error) {
	_ = IsRepostingItemValuationInProgress()

	columns := r.getColumns()
	binList, err := r.getBinList(filters)
	if err != nil {
		return nil, fmt.Errorf("bin list: %w", err)
	}

	itemMap, err := r.getItemMap(filters)
	if err != nil {
		return nil, fmt.Errorf("item map: %w", err)
	}

	var data [][]interface{}
	for _, bin := range binList {
		itemCode, _ := bin["item_code"].(string)
		item, ok := itemMap[itemCode]
		if !ok {
			continue
		}

		if brandFilter, ok := filters["brand"].(string); ok && brandFilter != "" {
			if itemBrand, _ := item["brand"].(string); itemBrand != brandFilter {
				continue
			}
		}

		if companyFilter, ok := filters["company"].(string); ok && companyFilter != "" {
			wh, _ := bin["warehouse"].(string)
			whCompany := r.getWarehouseCompany(wh)
			if whCompany != companyFilter {
				continue
			}
		}

		actualQty := frappe.FltFromAny(bin["actual_qty"], -1)
		plannedQty := frappe.FltFromAny(bin["planned_qty"], -1)
		indentedQty := frappe.FltFromAny(bin["indented_qty"], -1)
		orderedQty := frappe.FltFromAny(bin["ordered_qty"], -1)
		reservedQty := frappe.FltFromAny(bin["reserved_qty"], -1)
		reservedQtyProd := frappe.FltFromAny(bin["reserved_qty_for_production"], -1)
		reservedQtyProdPlan := frappe.FltFromAny(bin["reserved_qty_for_production_plan"], -1)
		reservedQtySub := frappe.FltFromAny(bin["reserved_qty_for_sub_contract"], -1)
		projectedQty := frappe.FltFromAny(bin["projected_qty"], -1)

		shortageQty := 0.0
		// shortage_qty calculation would need reorder_level from Item Reorder

		row := []interface{}{
			itemCode,
			item["item_name"],
			item["description"],
			item["item_group"],
			item["brand"],
			bin["warehouse"],
			item["stock_uom"],
			actualQty,
			plannedQty,
			indentedQty,
			orderedQty,
			reservedQty,
			reservedQtyProd,
			reservedQtyProdPlan,
			reservedQtySub,
			0.0, // reserved_qty_for_pos - TODO: get_pos_reserved_qty
			projectedQty,
			0.0, // re_order_level
			0.0, // re_order_qty
			shortageQty,
		}
		data = append(data, row)
	}

	return &ReportResult{
		Columns: columns,
		Result:  data,
	}, nil
}

func (r *StockProjectedQtyReport) getColumns() []Column {
	return []Column{
		{Label: "Item Code", Fieldname: "item_code", Fieldtype: "Link", Options: "Item", Width: 140},
		{Label: "Item Name", Fieldname: "item_name", Width: 100},
		{Label: "Description", Fieldname: "description", Width: 200},
		{Label: "Item Group", Fieldname: "item_group", Fieldtype: "Link", Options: "Item Group", Width: 100},
		{Label: "Brand", Fieldname: "brand", Fieldtype: "Link", Options: "Brand", Width: 100},
		{Label: "Warehouse", Fieldname: "warehouse", Fieldtype: "Link", Options: "Warehouse", Width: 120},
		{Label: "UOM", Fieldname: "stock_uom", Fieldtype: "Link", Options: "UOM", Width: 100},
		{Label: "Actual Qty", Fieldname: "actual_qty", Fieldtype: "Float", Width: 100},
		{Label: "Planned Qty", Fieldname: "planned_qty", Fieldtype: "Float", Width: 100},
		{Label: "Requested Qty", Fieldname: "indented_qty", Fieldtype: "Float", Width: 110},
		{Label: "Ordered Qty", Fieldname: "ordered_qty", Fieldtype: "Float", Width: 100},
		{Label: "Reserved Qty", Fieldname: "reserved_qty", Fieldtype: "Float", Width: 100},
		{Label: "Reserved for Production", Fieldname: "reserved_qty_for_production", Fieldtype: "Float", Width: 100},
		{Label: "Reserved for Production Plan", Fieldname: "reserved_qty_for_production_plan", Fieldtype: "Float", Width: 100},
		{Label: "Reserved for Sub Contracting", Fieldname: "reserved_qty_for_sub_contract", Fieldtype: "Float", Width: 100},
		{Label: "Reserved for POS Transactions", Fieldname: "reserved_qty_for_pos", Fieldtype: "Float", Width: 100},
		{Label: "Projected Qty", Fieldname: "projected_qty", Fieldtype: "Float", Width: 100},
		{Label: "Reorder Level", Fieldname: "re_order_level", Fieldtype: "Float", Width: 100},
		{Label: "Reorder Qty", Fieldname: "re_order_qty", Fieldtype: "Float", Width: 100},
		{Label: "Shortage Qty", Fieldname: "shortage_qty", Fieldtype: "Float", Width: 100},
	}
}

func (r *StockProjectedQtyReport) getBinList(filters map[string]interface{}) ([]map[string]interface{}, error) {
	if r.DB == nil {
		return nil, nil
	}

	query := "SELECT `item_code`, `warehouse`, `actual_qty`, `planned_qty`, `indented_qty`," +
		" `ordered_qty`, `reserved_qty`, `reserved_qty_for_production`," +
		" `reserved_qty_for_sub_contract`, `reserved_qty_for_production_plan`, `projected_qty`" +
		" FROM `tabBin`"

	var conditions []string
	var args []interface{}

	if itemCode, ok := filters["item_code"].(string); ok && itemCode != "" {
		conditions = append(conditions, "`item_code` = ?")
		args = append(args, itemCode)
	}
	if warehouse, ok := filters["warehouse"].(string); ok && warehouse != "" {
		conditions = append(conditions, "`warehouse` = ?")
		args = append(args, warehouse)
	}

	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += " ORDER BY `item_code`, `warehouse`"

	rows, err := r.DB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanRowsToMaps(rows)
}

func (r *StockProjectedQtyReport) getItemMap(filters map[string]interface{}) (map[string]map[string]interface{}, error) {
	result := make(map[string]map[string]interface{})
	if r.DB == nil {
		return result, nil
	}

	query := "SELECT `name`, `item_name`, `description`, `item_group`, `brand`, `stock_uom`" +
		" FROM `tabItem` WHERE `is_stock_item` = 1 AND `disabled` = 0"

	var args []interface{}
	if itemCode, ok := filters["item_code"].(string); ok && itemCode != "" {
		query += " AND `name` = ?"
		args = append(args, itemCode)
	}

	rows, err := r.DB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var name, itemName, description, itemGroup, brand, stockUOM string
		if err := rows.Scan(&name, &itemName, &description, &itemGroup, &brand, &stockUOM); err != nil {
			return nil, err
		}
		result[name] = map[string]interface{}{
			"item_name":   itemName,
			"description": description,
			"item_group":  itemGroup,
			"brand":       brand,
			"stock_uom":   stockUOM,
		}
	}
	return result, nil
}

func (r *StockProjectedQtyReport) getWarehouseCompany(warehouse string) string {
	if r.DB == nil || warehouse == "" {
		return ""
	}
	rows, err := r.DB.Query("SELECT `company` FROM `tabWarehouse` WHERE `name` = ? LIMIT 1", warehouse)
	if err != nil {
		return ""
	}
	defer rows.Close()
	var company string
	if rows.Next() {
		rows.Scan(&company)
	}
	return company
}

// ---------------------------------------------------------------------------
// 2E.1c  Item Shortage Report
// Source: erpnext/stock/report/item_shortage_report/item_shortage_report.py
// ---------------------------------------------------------------------------

// ItemShortageReport implements the Item Shortage Report.
type ItemShortageReport struct {
	DB *db.DB
}

func (r *ItemShortageReport) Execute(filters map[string]interface{}) (*ReportResult, error) {
	columns := r.getColumns()
	data, err := r.getData(filters)
	if err != nil {
		return nil, fmt.Errorf("item shortage data: %w", err)
	}

	var chart interface{}
	if len(data) > 0 {
		chart = r.getChartData(data)
	}

	result := make([][]interface{}, len(data))
	for i, row := range data {
		result[i] = []interface{}{
			row["warehouse"], row["item_code"], row["actual_qty"],
			row["ordered_qty"], row["planned_qty"], row["reserved_qty"],
			row["reserved_qty_for_production"], row["projected_qty"],
			row["company"], row["item_name"], row["description"],
		}
	}

	return &ReportResult{
		Columns: columns,
		Result:  result,
		Chart:   chart,
	}, nil
}

func (r *ItemShortageReport) getColumns() []Column {
	return []Column{
		{Label: "Warehouse", Fieldname: "warehouse", Fieldtype: "Link", Options: "Warehouse", Width: 150},
		{Label: "Item", Fieldname: "item_code", Fieldtype: "Link", Options: "Item", Width: 150},
		{Label: "Actual Quantity", Fieldname: "actual_qty", Fieldtype: "Float", Width: 120},
		{Label: "Ordered Quantity", Fieldname: "ordered_qty", Fieldtype: "Float", Width: 120},
		{Label: "Planned Quantity", Fieldname: "planned_qty", Fieldtype: "Float", Width: 120},
		{Label: "Reserved Quantity", Fieldname: "reserved_qty", Fieldtype: "Float", Width: 120},
		{Label: "Reserved Quantity for Production", Fieldname: "reserved_qty_for_production", Fieldtype: "Float", Width: 120},
		{Label: "Projected Quantity", Fieldname: "projected_qty", Fieldtype: "Float", Width: 120},
		{Label: "Company", Fieldname: "company", Fieldtype: "Link", Options: "Company", Width: 120},
		{Label: "Item Name", Fieldname: "item_name", Fieldtype: "Data", Width: 100},
		{Label: "Description", Fieldname: "description", Fieldtype: "Data", Width: 120},
	}
}

func (r *ItemShortageReport) getData(filters map[string]interface{}) ([]map[string]interface{}, error) {
	if r.DB == nil {
		return nil, nil
	}

	query := "SELECT b.`warehouse`, b.`item_code`, b.`actual_qty`, b.`ordered_qty`," +
		" b.`planned_qty`, b.`reserved_qty`, b.`reserved_qty_for_production`," +
		" b.`projected_qty`, w.`company`, i.`item_name`, i.`description`" +
		" FROM `tabBin` b, `tabWarehouse` w, `tabItem` i" +
		" WHERE i.`disabled` = 0 AND b.`projected_qty` < 0" +
		" AND w.`name` = b.`warehouse` AND b.`item_code` = i.`name`"

	var args []interface{}
	if warehouse, ok := filters["warehouse"].(string); ok && warehouse != "" {
		query += " AND b.`warehouse` = ?"
		args = append(args, warehouse)
	}
	if company, ok := filters["company"].(string); ok && company != "" {
		query += " AND w.`company` = ?"
		args = append(args, company)
	}

	query += " ORDER BY b.`projected_qty`"

	rows, err := r.DB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanRowsToMaps(rows)
}

func (r *ItemShortageReport) getChartData(data []map[string]interface{}) map[string]interface{} {
	labels := []interface{}{}
	datapoints := []interface{}{}

	limit := len(data)
	if limit > 10 {
		limit = 10
	}

	for i := 0; i < limit; i++ {
		labels = append(labels, data[i]["item_code"])
		datapoints = append(datapoints, data[i]["projected_qty"])
	}

	return map[string]interface{}{
		"data": map[string]interface{}{
			"labels":   labels,
			"datasets": []map[string]interface{}{{"name": "Projected Qty", "values": datapoints}},
		},
		"type": "bar",
	}
}

// ---------------------------------------------------------------------------
// 2E.1d  Reserved Stock Report
// Source: erpnext/stock/report/reserved_stock/reserved_stock.py
// ---------------------------------------------------------------------------

// ReservedStockReport implements the Reserved Stock report.
type ReservedStockReport struct {
	DB *db.DB
}

func (r *ReservedStockReport) Execute(filters map[string]interface{}) (*ReportResult, error) {
	// Validate filters
	for _, field := range []string{"company", "from_date", "to_date"} {
		if val, ok := filters[field].(string); !ok || val == "" {
			return nil, fmt.Errorf("%s is required", field)
		}
	}

	columns := r.getColumns()
	data, err := r.getData(filters)
	if err != nil {
		return nil, fmt.Errorf("reserved stock data: %w", err)
	}

	return &ReportResult{
		Columns: columns,
		Result:  data,
	}, nil
}

func (r *ReservedStockReport) getColumns() []Column {
	return []Column{
		{Label: "Date", Fieldname: "date", Fieldtype: "Datetime", Width: 150},
		{Label: "Warehouse", Fieldname: "warehouse", Fieldtype: "Link", Options: "Warehouse", Width: 150},
		{Label: "Item", Fieldname: "item_code", Fieldtype: "Link", Options: "Item", Width: 100},
		{Label: "Stock UOM", Fieldname: "stock_uom", Fieldtype: "Link", Options: "UOM", Width: 100},
		{Label: "Voucher Qty", Fieldname: "voucher_qty", Fieldtype: "Float", Width: 110},
		{Label: "Reserved Qty", Fieldname: "reserved_qty", Fieldtype: "Float", Width: 110},
		{Label: "Delivered Qty", Fieldname: "delivered_qty", Fieldtype: "Float", Width: 110},
		{Label: "Available Qty to Reserve", Fieldname: "available_qty", Fieldtype: "Float", Width: 120},
		{Label: "Voucher Type", Fieldname: "voucher_type", Fieldtype: "Data", Width: 110},
		{Label: "Voucher No", Fieldname: "voucher_no", Fieldtype: "Dynamic Link", Options: "voucher_type", Width: 120},
		{Label: "From Voucher Type", Fieldname: "from_voucher_type", Fieldtype: "Data", Width: 110},
		{Label: "From Voucher No", Fieldname: "from_voucher_no", Fieldtype: "Dynamic Link", Options: "from_voucher_type", Width: 120},
		{Label: "Stock Reservation Entry", Fieldname: "stock_reservation_entry", Fieldtype: "Link", Options: "Stock Reservation Entry", Width: 150},
		{Label: "Status", Fieldname: "status", Fieldtype: "Data", Width: 120},
		{Label: "Project", Fieldname: "project", Fieldtype: "Link", Options: "Project", Width: 100},
		{Label: "Company", Fieldname: "company", Fieldtype: "Link", Options: "Company", Width: 110},
	}
}

func (r *ReservedStockReport) getData(filters map[string]interface{}) ([][]interface{}, error) {
	if r.DB == nil {
		return nil, nil
	}

	company, _ := filters["company"].(string)
	fromDate, _ := filters["from_date"].(string)
	toDate, _ := filters["to_date"].(string)

	query := "SELECT `creation`, `warehouse`, `item_code`, `stock_uom`," +
		" `voucher_qty`, `reserved_qty`, `delivered_qty`," +
		" (`available_qty` - `reserved_qty`) AS `available_qty`," +
		" `voucher_type`, `voucher_no`, `from_voucher_type`, `from_voucher_no`," +
		" `name` AS `stock_reservation_entry`, `status`, `project`, `company`" +
		" FROM `tabStock Reservation Entry`" +
		" WHERE `docstatus` = 1 AND `company` = ?" +
		" AND DATE(`creation`) >= ? AND DATE(`creation`) <= ?"

	args := []interface{}{company, fromDate, toDate}

	for _, field := range []string{"item_code", "warehouse", "voucher_type", "voucher_no",
		"from_voucher_type", "from_voucher_no", "status", "project"} {
		if val, ok := filters[field].(string); ok && val != "" {
			query += fmt.Sprintf(" AND `%s` = ?", field)
			args = append(args, val)
		}
	}

	if val, ok := filters["stock_reservation_entry"].(string); ok && val != "" {
		query += " AND `name` = ?"
		args = append(args, val)
	}

	rows, err := r.DB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanRowsToSlices(rows)
}

// ---------------------------------------------------------------------------
// 2E.1e  Stock Analytics Report
// Source: erpnext/stock/report/stock_analytics/stock_analytics.py
// ---------------------------------------------------------------------------

// StockAnalyticsReport implements the Stock Analytics report.
type StockAnalyticsReport struct {
	DB *db.DB
}

func (r *StockAnalyticsReport) Execute(filters map[string]interface{}) (*ReportResult, error) {
	_ = IsRepostingItemValuationInProgress()

	periodColumns := r.getPeriodColumns(filters)
	columns := r.getColumns(periodColumns)

	data, err := r.getData(filters, periodColumns)
	if err != nil {
		return nil, fmt.Errorf("stock analytics data: %w", err)
	}

	chart := r.getChartData(periodColumns)

	return &ReportResult{
		Columns: columns,
		Result:  data,
		Chart:   chart,
	}, nil
}

func (r *StockAnalyticsReport) getPeriodColumns(filters map[string]interface{}) []Column {
	ranges := r.getPeriodDateRanges(filters)
	var periodCols []Column
	for _, dateRange := range ranges {
		endDate := dateRange[1]
		period := r.getPeriod(endDate, filters)
		fieldname := strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(period, " ", "_"), "-", "_"))
		periodCols = append(periodCols, Column{
			Label:     period,
			Fieldname: fieldname,
			Fieldtype: "Float",
			Width:     120,
		})
	}
	return periodCols
}

func (r *StockAnalyticsReport) getColumns(periodColumns []Column) []Column {
	columns := []Column{
		{Label: "Item", Fieldname: "name", Fieldtype: "Link", Options: "Item", Width: 140},
		{Label: "Item Name", Fieldname: "item_name", Fieldtype: "Link", Options: "Item", Width: 140},
		{Label: "Item Group", Fieldname: "item_group", Fieldtype: "Link", Options: "Item Group", Width: 140},
		{Label: "Brand", Fieldname: "brand", Fieldtype: "Data", Width: 120},
		{Label: "UOM", Fieldname: "uom", Fieldtype: "Data", Width: 120},
	}
	columns = append(columns, periodColumns...)
	return columns
}

func (r *StockAnalyticsReport) getPeriodDateRanges(filters map[string]interface{}) [][2]time.Time {
	fromDateStr, _ := filters["from_date"].(string)
	toDateStr, _ := filters["to_date"].(string)
	rangeType, _ := filters["range"].(string)

	fromDate := frappe.Getdate(fromDateStr)
	toDate := frappe.Getdate(toDateStr)

	if fromDate.IsZero() || toDate.IsZero() {
		return nil
	}

	// Round down to nearest frequency
	switch rangeType {
	case "Monthly":
		fromDate = time.Date(fromDate.Year(), fromDate.Month(), 1, 0, 0, 0, 0, time.UTC)
	case "Quarterly":
		q := (int(fromDate.Month()) - 1) / 3
		fromDate = time.Date(fromDate.Year(), time.Month(q*3+1), 1, 0, 0, 0, 0, time.UTC)
	case "Yearly":
		fromDate = time.Date(fromDate.Year(), 1, 1, 0, 0, 0, 0, time.UTC)
	case "Weekly":
		weekday := int(fromDate.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		fromDate = fromDate.AddDate(0, 0, -(weekday - 1))
	}

	incrementMap := map[string]int{
		"Monthly":     1,
		"Quarterly":   3,
		"Half-Yearly": 6,
		"Yearly":      12,
	}
	increment := incrementMap[rangeType]
	if increment == 0 {
		increment = 1
	}

	var ranges [][2]time.Time
	for i := 0; i < 53; i++ {
		var periodEnd time.Time
		if rangeType == "Weekly" {
			periodEnd = fromDate.AddDate(0, 0, 6)
		} else {
			periodEnd = frappe.AddDays(frappe.AddMonths(fromDate, increment), -1)
		}

		if periodEnd.After(toDate) {
			periodEnd = toDate
		}
		ranges = append(ranges, [2]time.Time{fromDate, periodEnd})

		fromDate = periodEnd.AddDate(0, 0, 1)
		if !periodEnd.Before(toDate) {
			break
		}
	}

	return ranges
}

func (r *StockAnalyticsReport) getPeriod(date time.Time, filters map[string]interface{}) string {
	rangeType, _ := filters["range"].(string)
	months := []string{"Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"}

	switch rangeType {
	case "Weekly":
		_, week := date.ISOWeek()
		return fmt.Sprintf("Week %d %d", week, date.Year())
	case "Monthly":
		return fmt.Sprintf("%s %d", months[date.Month()-1], date.Year())
	case "Quarterly":
		quarter := (int(date.Month())-1)/3 + 1
		return fmt.Sprintf("Quarter %d %d", quarter, date.Year())
	default: // Yearly
		return fmt.Sprintf("%d", date.Year())
	}
}

func (r *StockAnalyticsReport) getData(filters map[string]interface{}, periodColumns []Column) ([][]interface{}, error) {
	if r.DB == nil {
		return nil, nil
	}

	items := r.getItems(filters)
	slEntries, err := r.getStockLedgerEntries(filters, items)
	if err != nil {
		return nil, err
	}

	itemDetails, err := r.getItemDetails(items, slEntries)
	if err != nil {
		return nil, err
	}

	periodicData := r.getPeriodicData(slEntries, filters)
	ranges := r.getPeriodDateRanges(filters)

	today := time.Now().UTC()
	today = time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.UTC)

	var data [][]interface{}
	for itemCode, detail := range itemDetails {
		row := make([]interface{}, 5+len(periodColumns))
		row[0] = itemCode
		row[1] = detail["item_name"]
		row[2] = detail["item_group"]
		row[3] = detail["brand"]
		row[4] = detail["stock_uom"]

		previousValue := 0.0
		for i, dateRange := range ranges {
			startDate := dateRange[0]
			endDate := dateRange[1]
			period := r.getPeriod(endDate, filters)
			fieldname := strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(period, " ", "_"), "-", "_"))
			_ = fieldname

			if itemPeriodData, ok := periodicData[itemCode]; ok {
				if periodData, ok := itemPeriodData[period]; ok {
					total := 0.0
					for _, v := range periodData {
						total += v
					}
					previousValue = total
					row[5+i] = total
				} else {
					if !today.Before(startDate) {
						row[5+i] = previousValue
					}
				}
			} else {
				if !today.Before(startDate) {
					row[5+i] = previousValue
				}
			}
		}
		data = append(data, row)
	}

	return data, nil
}

func (r *StockAnalyticsReport) getItems(filters map[string]interface{}) []string {
	if itemCode, ok := filters["item_code"].(string); ok && itemCode != "" {
		return []string{itemCode}
	}
	if r.DB == nil {
		return nil
	}

	query := "SELECT `name` FROM `tabItem` WHERE `is_stock_item` = 1"
	var args []interface{}

	if brand, ok := filters["brand"].(string); ok && brand != "" {
		query += " AND `brand` = ?"
		args = append(args, brand)
	}
	if itemGroup, ok := filters["item_group"].(string); ok && itemGroup != "" {
		query += " AND `item_group` = ?"
		args = append(args, itemGroup)
	}

	rows, err := r.DB.Query(query, args...)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var items []string
	for rows.Next() {
		var name string
		if rows.Scan(&name) == nil {
			items = append(items, name)
		}
	}
	return items
}

func (r *StockAnalyticsReport) getStockLedgerEntries(filters map[string]interface{}, items []string) ([]map[string]interface{}, error) {
	if r.DB == nil {
		return nil, nil
	}

	toDateStr, _ := filters["to_date"].(string)

	query := "SELECT `item_code`, `warehouse`, `posting_date`, `actual_qty`," +
		" `valuation_rate`, `company`, `voucher_type`, `qty_after_transaction`," +
		" `stock_value_difference`, `item_code` AS `name`, `voucher_no`," +
		" `stock_value`, `batch_no`" +
		" FROM `tabStock Ledger Entry`" +
		" WHERE `docstatus` < 2 AND `is_cancelled` = 0"

	var args []interface{}

	if len(items) > 0 {
		placeholders := strings.Repeat("?,", len(items))
		placeholders = placeholders[:len(placeholders)-1]
		query += " AND `item_code` IN (" + placeholders + ")"
		for _, item := range items {
			args = append(args, item)
		}
	}

	if toDateStr != "" {
		toDate := toDateStr + " 23:59:59"
		query += " AND `posting_datetime` <= ?"
		args = append(args, toDate)
	}

	if company, ok := filters["company"].(string); ok && company != "" {
		query += " AND `company` = ?"
		args = append(args, company)
	}

	if warehouse, ok := filters["warehouse"].(string); ok && warehouse != "" {
		query += " AND `warehouse` = ?"
		args = append(args, warehouse)
	}

	query += " ORDER BY `posting_datetime`, `creation`"

	rows, err := r.DB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanRowsToMaps(rows)
}

func (r *StockAnalyticsReport) getItemDetails(items []string, slEntries []map[string]interface{}) (map[string]map[string]interface{}, error) {
	result := make(map[string]map[string]interface{})
	if r.DB == nil {
		return result, nil
	}

	itemSet := make(map[string]bool)
	if len(items) > 0 {
		for _, item := range items {
			itemSet[item] = true
		}
	} else {
		for _, sle := range slEntries {
			if ic, ok := sle["item_code"].(string); ok {
				itemSet[ic] = true
			}
		}
	}

	if len(itemSet) == 0 {
		return result, nil
	}

	itemList := make([]string, 0, len(itemSet))
	for item := range itemSet {
		itemList = append(itemList, item)
	}

	placeholders := strings.Repeat("?,", len(itemList))
	placeholders = placeholders[:len(placeholders)-1]

	query := "SELECT `name`, `item_name`, `description`, `item_group`, `brand`, `stock_uom`" +
		" FROM `tabItem` WHERE `name` IN (" + placeholders + ")"

	args := make([]interface{}, len(itemList))
	for i, item := range itemList {
		args[i] = item
	}

	rows, err := r.DB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var name, itemName, description, itemGroup, brand, stockUOM string
		if err := rows.Scan(&name, &itemName, &description, &itemGroup, &brand, &stockUOM); err != nil {
			return nil, err
		}
		result[name] = map[string]interface{}{
			"item_name":   itemName,
			"description": description,
			"item_group":  itemGroup,
			"brand":       brand,
			"stock_uom":   stockUOM,
		}
	}
	return result, nil
}

func (r *StockAnalyticsReport) getPeriodicData(entries []map[string]interface{}, filters map[string]interface{}) map[string]map[string]map[string]float64 {
	valueQuantity, _ := filters["value_quantity"].(string)
	if valueQuantity == "" {
		valueQuantity = "Quantity"
	}

	periodicData := make(map[string]map[string]map[string]float64)

	for _, d := range entries {
		itemCode, _ := d["item_code"].(string)
		warehouse, _ := d["warehouse"].(string)
		postingDateStr, _ := d["posting_date"].(string)
		postingDate := frappe.Getdate(postingDateStr)
		period := r.getPeriod(postingDate, filters)

		actualQty := frappe.FltFromAny(d["actual_qty"], -1)
		stockValueDiff := frappe.FltFromAny(d["stock_value_difference"], -1)

		var value float64
		if valueQuantity == "Quantity" {
			value = actualQty
		} else {
			value = stockValueDiff
		}

		if _, ok := periodicData[itemCode]; !ok {
			periodicData[itemCode] = make(map[string]map[string]float64)
			periodicData[itemCode]["balance"] = make(map[string]float64)
		}

		if _, ok := periodicData[itemCode][period]; !ok {
			// Copy balance
			prevBalance := make(map[string]float64)
			for k, v := range periodicData[itemCode]["balance"] {
				prevBalance[k] = v
			}
			periodicData[itemCode][period] = prevBalance
		}

		periodicData[itemCode]["balance"][warehouse] += value
		periodicData[itemCode][period][warehouse] = periodicData[itemCode]["balance"][warehouse]
	}

	return periodicData
}

func (r *StockAnalyticsReport) getChartData(periodColumns []Column) map[string]interface{} {
	labels := make([]interface{}, len(periodColumns))
	for i, col := range periodColumns {
		labels[i] = col.Label
	}

	return map[string]interface{}{
		"data": map[string]interface{}{
			"labels":   labels,
			"datasets": []interface{}{},
		},
		"type": "line",
	}
}

// ---------------------------------------------------------------------------
// 2E.1f  Serial and Batch Summary Report
// Source: erpnext/stock/report/serial_and_batch_summary/serial_and_batch_summary.py
// ---------------------------------------------------------------------------

// SerialAndBatchSummaryReport implements the Serial and Batch Summary report.
type SerialAndBatchSummaryReport struct {
	DB *db.DB
}

func (r *SerialAndBatchSummaryReport) Execute(filters map[string]interface{}) (*ReportResult, error) {
	data, err := r.getData(filters)
	if err != nil {
		return nil, fmt.Errorf("serial batch summary data: %w", err)
	}

	columns := r.getColumns(filters)

	result := make([][]interface{}, len(data))
	for i, row := range data {
		rowData := make([]interface{}, len(columns))
		for j, col := range columns {
			rowData[j] = row[col.Fieldname]
		}
		result[i] = rowData
	}

	return &ReportResult{
		Columns: columns,
		Result:  result,
	}, nil
}

func (r *SerialAndBatchSummaryReport) getColumns(filters map[string]interface{}) []Column {
	columns := []Column{
		{Label: "Company", Fieldname: "company", Fieldtype: "Link", Options: "Company", Width: 120},
		{Label: "Serial and Batch Bundle", Fieldname: "name", Fieldtype: "Link", Options: "Serial and Batch Bundle", Width: 110},
		{Label: "Posting Date", Fieldname: "posting_date", Fieldtype: "Date", Width: 100},
	}

	if _, ok := filters["voucher_no"]; !ok {
		columns = append(columns,
			Column{Label: "Voucher Type", Fieldname: "voucher_type", Width: 120},
			Column{Label: "Voucher No", Fieldname: "voucher_no", Fieldtype: "Dynamic Link", Options: "voucher_type", Width: 160},
		)
	}

	if _, ok := filters["item_code"]; !ok {
		columns = append(columns,
			Column{Label: "Item Code", Fieldname: "item_code", Fieldtype: "Link", Options: "Item", Width: 120},
			Column{Label: "Item Name", Fieldname: "item_name", Fieldtype: "Data", Width: 120},
		)
	}

	if _, ok := filters["warehouse"]; !ok {
		columns = append(columns,
			Column{Label: "Warehouse", Fieldname: "warehouse", Fieldtype: "Link", Options: "Warehouse", Width: 120},
		)
	}

	// Always include serial_no and batch_no unless we know the item doesn't have them
	columns = append(columns,
		Column{Label: "Serial No", Fieldname: "serial_no", Fieldtype: "Link", Options: "Serial No", Width: 120},
		Column{Label: "Batch No", Fieldname: "batch_no", Fieldtype: "Data", Width: 120},
		Column{Label: "Batch Qty", Fieldname: "qty", Fieldtype: "Float", Width: 120},
	)

	columns = append(columns,
		Column{Label: "Incoming Rate", Fieldname: "incoming_rate", Fieldtype: "Float", Width: 120},
		Column{Label: "Change in Stock Value", Fieldname: "stock_value_difference", Fieldtype: "Float", Width: 120},
	)

	return columns
}

func (r *SerialAndBatchSummaryReport) getData(filters map[string]interface{}) ([]map[string]interface{}, error) {
	if r.DB == nil {
		return nil, nil
	}

	query := "SELECT sbb.`voucher_type`, sbb.`posting_datetime` AS `posting_date`," +
		" sbb.`name`, sbb.`company`, sbb.`voucher_no`, sbb.`item_code`, sbb.`item_name`," +
		" sbe.`serial_no`, sbe.`batch_no`, sbe.`warehouse`," +
		" sbe.`incoming_rate`, sbe.`stock_value_difference`, sbe.`qty`" +
		" FROM `tabSerial and Batch Bundle` sbb" +
		" INNER JOIN `tabSerial and Batch Entry` sbe ON sbe.`parent` = sbb.`name`" +
		" WHERE sbb.`docstatus` = 1 AND sbb.`is_cancelled` = 0"

	var args []interface{}

	for _, field := range []string{"voucher_type", "item_code", "warehouse", "company"} {
		if val, ok := filters[field].(string); ok && val != "" {
			query += fmt.Sprintf(" AND sbb.`%s` = ?", field)
			args = append(args, val)
		}
	}

	if voucherNo, ok := filters["voucher_no"].(string); ok && voucherNo != "" {
		query += " AND sbb.`voucher_no` = ?"
		args = append(args, voucherNo)
	}

	if fromDate, ok := filters["from_date"].(string); ok && fromDate != "" {
		if toDate, ok := filters["to_date"].(string); ok && toDate != "" {
			query += " AND sbb.`posting_datetime` BETWEEN ? AND ?"
			args = append(args, fromDate, toDate+" 23:59:59")
		}
	}

	for _, field := range []string{"serial_no", "batch_no"} {
		if val, ok := filters[field].(string); ok && val != "" {
			query += fmt.Sprintf(" AND sbe.`%s` = ?", field)
			args = append(args, val)
		}
	}

	query += " ORDER BY sbb.`posting_datetime`"

	rows, err := r.DB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanRowsToMaps(rows)
}

// ---------------------------------------------------------------------------
// 2E.1g  Serial No Ledger Report
// Source: erpnext/stock/report/serial_no_ledger/serial_no_ledger.py
// ---------------------------------------------------------------------------

// SerialNoLedgerReport implements the Serial No Ledger report.
type SerialNoLedgerReport struct {
	DB *db.DB
}

var buyingVoucherTypes = map[string]bool{
	"Purchase Invoice":      true,
	"Purchase Receipt":      true,
	"Subcontracting Receipt": true,
}

var sellingVoucherTypes = map[string]bool{
	"Sales Invoice": true,
	"Delivery Note": true,
}

func (r *SerialNoLedgerReport) Execute(filters map[string]interface{}) (*ReportResult, error) {
	columns := r.getColumns()
	data, err := r.getData(filters)
	if err != nil {
		return nil, fmt.Errorf("serial no ledger data: %w", err)
	}

	result := make([][]interface{}, len(data))
	for i, row := range data {
		rowData := make([]interface{}, len(columns))
		for j, col := range columns {
			rowData[j] = row[col.Fieldname]
		}
		result[i] = rowData
	}

	return &ReportResult{
		Columns: columns,
		Result:  result,
	}, nil
}

func (r *SerialNoLedgerReport) getColumns() []Column {
	return []Column{
		{Label: "Posting Date", Fieldname: "posting_date", Fieldtype: "Date", Width: 120},
		{Label: "Posting Time", Fieldname: "posting_time", Fieldtype: "Time", Width: 90},
		{Label: "Voucher Type", Fieldname: "voucher_type", Fieldtype: "Data", Width: 160},
		{Label: "Voucher No", Fieldname: "voucher_no", Fieldtype: "Dynamic Link", Options: "voucher_type", Width: 230},
		{Label: "Company", Fieldname: "company", Fieldtype: "Link", Options: "Company", Width: 120},
		{Label: "Warehouse", Fieldname: "warehouse", Fieldtype: "Link", Options: "Warehouse", Width: 120},
		{Label: "Status", Fieldname: "status", Fieldtype: "Data", Width: 90},
		{Label: "Serial No", Fieldname: "serial_no", Fieldtype: "Link", Options: "Serial No", Width: 130},
		{Label: "Valuation Rate", Fieldname: "valuation_rate", Fieldtype: "Float", Width: 130},
		{Label: "Qty", Fieldname: "qty", Fieldtype: "Float", Width: 150},
		{Label: "Party Type", Fieldname: "party_type", Fieldtype: "Data", Width: 90},
		{Label: "Party", Fieldname: "party", Fieldtype: "Dynamic Link", Options: "party_type", Width: 120},
	}
}

func (r *SerialNoLedgerReport) getData(filters map[string]interface{}) ([]map[string]interface{}, error) {
	if r.DB == nil {
		return nil, nil
	}

	itemCode, _ := filters["item_code"].(string)
	serialNo, _ := filters["serial_no"].(string)
	fromDate, _ := filters["from_date"].(string)
	toDate, _ := filters["to_date"].(string)

	// Get stock ledger entries
	query := "SELECT `posting_date`, `posting_time`, `voucher_type`, `voucher_no`," +
		" `company`, `warehouse`, `actual_qty`, `serial_no`," +
		" `serial_and_batch_bundle`, `stock_value_difference`" +
		" FROM `tabStock Ledger Entry`" +
		" WHERE `docstatus` < 2 AND `is_cancelled` = 0"

	var args []interface{}
	if itemCode != "" {
		query += " AND `item_code` = ?"
		args = append(args, itemCode)
	}
	if fromDate != "" {
		query += " AND `posting_date` >= ?"
		args = append(args, fromDate)
	}
	if toDate != "" {
		query += " AND `posting_date` <= ?"
		args = append(args, toDate)
	}

	query += " ORDER BY `posting_datetime` ASC, `creation` ASC"

	rows, err := r.DB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	slEntries, err := scanRowsToMaps(rows)
	if err != nil {
		return nil, err
	}

	if len(slEntries) == 0 {
		return nil, nil
	}

	// Get serial nos from bundles
	var bundleIDs []string
	for _, sle := range slEntries {
		if bundleID, ok := sle["serial_and_batch_bundle"].(string); ok && bundleID != "" {
			bundleIDs = append(bundleIDs, bundleID)
		}
	}

	bundleWiseSerialNos := make(map[string][]map[string]interface{})
	if len(bundleIDs) > 0 {
		bundleWiseSerialNos, _ = r.getSerialNosFromBundles(bundleIDs, serialNo)
	}

	var data []map[string]interface{}
	for _, sle := range slEntries {
		actualQty := frappe.FltFromAny(sle["actual_qty"], -1)
		status := "Delivered"
		qty := -1.0
		if actualQty > 0 {
			status = "Active"
			qty = 1.0
		}

		voucherType, _ := sle["voucher_type"].(string)
		partyType := ""
		if buyingVoucherTypes[voucherType] {
			partyType = "Supplier"
		} else if sellingVoucherTypes[voucherType] {
			partyType = "Customer"
		}

		baseArgs := map[string]interface{}{
			"posting_date": sle["posting_date"],
			"posting_time": sle["posting_time"],
			"voucher_type": voucherType,
			"voucher_no":   sle["voucher_no"],
			"status":       status,
			"company":      sle["company"],
			"warehouse":    sle["warehouse"],
			"qty":          qty,
			"party_type":   partyType,
			"party":        nil, // TODO: fetch party from voucher
		}

		var serialNos []map[string]interface{}
		if sleSerialNo, ok := sle["serial_no"].(string); ok && sleSerialNo != "" {
			parts := strings.Split(sleSerialNo, "\n")
			svd := frappe.FltFromAny(sle["stock_value_difference"], -1)
			valRate := 0.0
			if actualQty != 0 {
				valRate = math.Abs(svd / actualQty)
			}
			for _, sn := range parts {
				sn = strings.TrimSpace(sn)
				if sn == "" {
					continue
				}
				if serialNo != "" && serialNo != sn {
					continue
				}
				serialNos = append(serialNos, map[string]interface{}{
					"serial_no":      sn,
					"valuation_rate": valRate,
				})
			}
		}

		if bundleID, ok := sle["serial_and_batch_bundle"].(string); ok && bundleID != "" {
			if bundleSerials, ok := bundleWiseSerialNos[bundleID]; ok {
				serialNos = append(serialNos, bundleSerials...)
			}
		}

		for idx, snData := range serialNos {
			if idx == 0 {
				entry := make(map[string]interface{})
				for k, v := range baseArgs {
					entry[k] = v
				}
				entry["serial_no"] = snData["serial_no"]
				entry["valuation_rate"] = snData["valuation_rate"]
				data = append(data, entry)
			} else {
				data = append(data, map[string]interface{}{
					"serial_no":      snData["serial_no"],
					"valuation_rate": snData["valuation_rate"],
					"qty":            qty,
				})
			}
		}
	}

	return data, nil
}

func (r *SerialNoLedgerReport) getSerialNosFromBundles(bundleIDs []string, serialNoFilter string) (map[string][]map[string]interface{}, error) {
	result := make(map[string][]map[string]interface{})
	if r.DB == nil || len(bundleIDs) == 0 {
		return result, nil
	}

	placeholders := strings.Repeat("?,", len(bundleIDs))
	placeholders = placeholders[:len(placeholders)-1]

	query := "SELECT `serial_no`, `parent`, `stock_value_difference` AS `valuation_rate`" +
		" FROM `tabSerial and Batch Entry`" +
		" WHERE `parent` IN (" + placeholders + ")"

	args := make([]interface{}, len(bundleIDs))
	for i, id := range bundleIDs {
		args[i] = id
	}

	if serialNoFilter != "" {
		query += " AND `serial_no` = ?"
		args = append(args, serialNoFilter)
	}

	query += " ORDER BY `idx` ASC"

	rows, err := r.DB.Query(query, args...)
	if err != nil {
		return result, err
	}
	defer rows.Close()

	for rows.Next() {
		var serialNo, parent string
		var valRate float64
		if err := rows.Scan(&serialNo, &parent, &valRate); err != nil {
			continue
		}
		result[parent] = append(result[parent], map[string]interface{}{
			"serial_no":      serialNo,
			"valuation_rate": math.Abs(valRate),
		})
	}
	return result, nil
}

// ---------------------------------------------------------------------------
// 2E.1h  Warehouse Wise Stock Balance Report
// Source: erpnext/stock/report/warehouse_wise_stock_balance/warehouse_wise_stock_balance.py
// ---------------------------------------------------------------------------

// WarehouseWiseStockBalanceReport implements the Warehouse Wise Stock Balance report.
type WarehouseWiseStockBalanceReport struct {
	DB *db.DB
}

func (r *WarehouseWiseStockBalanceReport) Execute(filters map[string]interface{}) (*ReportResult, error) {
	columns := r.getColumns(filters)
	data, err := r.getData(filters)
	if err != nil {
		return nil, fmt.Errorf("warehouse stock balance data: %w", err)
	}

	result := make([][]interface{}, len(data))
	for i, row := range data {
		rowData := make([]interface{}, len(columns))
		for j, col := range columns {
			rowData[j] = row[col.Fieldname]
		}
		result[i] = rowData
	}

	return &ReportResult{
		Columns: columns,
		Result:  result,
	}, nil
}

func (r *WarehouseWiseStockBalanceReport) getColumns(filters map[string]interface{}) []Column {
	columns := []Column{
		{Label: "Warehouse", Fieldname: "name", Fieldtype: "Link", Options: "Warehouse", Width: 200},
		{Label: "Stock Balance", Fieldname: "stock_balance", Fieldtype: "Float", Width: 150},
	}

	if showDisabled, ok := filters["show_disabled_warehouses"]; ok && showDisabled != nil {
		v := frappe.Cint(showDisabled)
		if v != 0 {
			columns = append(columns, Column{
				Label: "Warehouse Disabled?", Fieldname: "disabled", Fieldtype: "Check", Width: 200,
			})
		}
	}

	return columns
}

func (r *WarehouseWiseStockBalanceReport) getData(filters map[string]interface{}) ([]map[string]interface{}, error) {
	if r.DB == nil {
		return nil, nil
	}

	warehouseBalance, err := r.getWarehouseWiseBalance(filters)
	if err != nil {
		return nil, err
	}

	warehouses, err := r.getWarehouses(filters)
	if err != nil {
		return nil, err
	}

	// Assign balances
	for i := range warehouses {
		wName, _ := warehouses[i]["name"].(string)
		if bal, ok := warehouseBalance[wName]; ok {
			warehouses[i]["stock_balance"] = bal
		} else {
			warehouses[i]["stock_balance"] = 0.0
		}
	}

	// Update indent
	r.updateIndent(warehouses)
	// Roll up balances to parents
	r.setBalanceInParent(warehouses)

	return warehouses, nil
}

func (r *WarehouseWiseStockBalanceReport) getWarehouseWiseBalance(filters map[string]interface{}) (map[string]float64, error) {
	result := make(map[string]float64)
	if r.DB == nil {
		return result, nil
	}

	query := "SELECT `warehouse`, SUM(`stock_value_difference`) AS `stock_balance`" +
		" FROM `tabStock Ledger Entry`" +
		" WHERE `docstatus` < 2 AND `is_cancelled` = 0"

	var args []interface{}
	if company, ok := filters["company"].(string); ok && company != "" {
		query += " AND `company` = ?"
		args = append(args, company)
	}
	query += " GROUP BY `warehouse`"

	rows, err := r.DB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var warehouse string
		var balance float64
		if err := rows.Scan(&warehouse, &balance); err != nil {
			continue
		}
		result[warehouse] = balance
	}
	return result, nil
}

func (r *WarehouseWiseStockBalanceReport) getWarehouses(filters map[string]interface{}) ([]map[string]interface{}, error) {
	if r.DB == nil {
		return nil, nil
	}

	query := "SELECT `name`, `parent_warehouse`, `is_group`, `disabled`" +
		" FROM `tabWarehouse` WHERE `disabled` = 0"

	var args []interface{}
	if company, ok := filters["company"].(string); ok && company != "" {
		query += " AND `company` = ?"
		args = append(args, company)
	}

	showDisabled := frappe.Cint(filters["show_disabled_warehouses"])
	if showDisabled != 0 {
		query = strings.Replace(query, "`disabled` = 0", "`disabled` IN (0, ?)", 1)
		args = append(args, showDisabled)
	}

	query += " ORDER BY `lft`"

	rows, err := r.DB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanRowsToMaps(rows)
}

func (r *WarehouseWiseStockBalanceReport) updateIndent(warehouses []map[string]interface{}) {
	for _, wh := range warehouses {
		isGroup, _ := wh["is_group"].(int64)
		if isGroup == 1 {
			whName, _ := wh["name"].(string)
			indent := frappe.FltFromAny(wh["indent"], -1)
			r.addIndent(warehouses, whName, indent)
		}
	}
}

func (r *WarehouseWiseStockBalanceReport) addIndent(warehouses []map[string]interface{}, parentName string, indent float64) {
	for i := range warehouses {
		parentWh, _ := warehouses[i]["parent_warehouse"].(string)
		if parentWh == parentName {
			warehouses[i]["indent"] = indent + 1
			whName, _ := warehouses[i]["name"].(string)
			isGroup, _ := warehouses[i]["is_group"].(int64)
			if isGroup == 1 {
				r.addIndent(warehouses, whName, indent+1)
			}
		}
	}
}

func (r *WarehouseWiseStockBalanceReport) setBalanceInParent(warehouses []map[string]interface{}) {
	// Process from highest indent to lowest
	maxIndent := 0.0
	for _, wh := range warehouses {
		ind := frappe.FltFromAny(wh["indent"], -1)
		if ind > maxIndent {
			maxIndent = ind
		}
	}

	for indent := maxIndent; indent >= 0; indent-- {
		for _, wh := range warehouses {
			whIndent := frappe.FltFromAny(wh["indent"], -1)
			if whIndent != indent {
				continue
			}
			parentName, _ := wh["parent_warehouse"].(string)
			if parentName == "" {
				continue
			}
			balance := frappe.FltFromAny(wh["stock_balance"], -1)
			for j := range warehouses {
				pName, _ := warehouses[j]["name"].(string)
				if pName == parentName {
					parentBal := frappe.FltFromAny(warehouses[j]["stock_balance"], -1)
					warehouses[j]["stock_balance"] = parentBal + balance
					break
				}
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Helper: scan SQL rows into []map[string]interface{}
// ---------------------------------------------------------------------------

func scanRowsToMaps(rows interface{ Next() bool; Columns() ([]string, error); Scan(...interface{}) error }) ([]map[string]interface{}, error) {
	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	var results []map[string]interface{}
	for rows.Next() {
		values := make([]interface{}, len(cols))
		valuePtrs := make([]interface{}, len(cols))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, err
		}

		row := make(map[string]interface{})
		for i, col := range cols {
			val := values[i]
			if b, ok := val.([]byte); ok {
				row[col] = string(b)
			} else {
				row[col] = val
			}
		}
		results = append(results, row)
	}
	return results, nil
}

// scanRowsToSlices scans SQL rows into [][]interface{}.
func scanRowsToSlices(rows interface{ Next() bool; Columns() ([]string, error); Scan(...interface{}) error }) ([][]interface{}, error) {
	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	var results [][]interface{}
	for rows.Next() {
		values := make([]interface{}, len(cols))
		valuePtrs := make([]interface{}, len(cols))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, err
		}

		row := make([]interface{}, len(cols))
		for i := range cols {
			val := values[i]
			if b, ok := val.([]byte); ok {
				row[i] = string(b)
			} else {
				row[i] = val
			}
		}
		results = append(results, row)
	}
	return results, nil
}
