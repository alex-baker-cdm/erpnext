package main

import (
	"encoding/json"
	"net/http"

	"github.com/alex-baker-cdm/erpnext/go-erpnext/pkg/db"
	"github.com/alex-baker-cdm/erpnext/go-erpnext/pkg/stock"
)

// stockItemHandler handles GET/POST
// /api/method/erpnext.stock.doctype.quick_stock_balance.quick_stock_balance.get_stock_item_details
//
// Query/form params: warehouse, date, item, barcode.
//
// Mirrors erpnext/stock/doctype/quick_stock_balance/quick_stock_balance.py:36-51.
func stockItemHandler(database *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()

		warehouse := r.FormValue("warehouse")
		date := r.FormValue("date")
		item := r.FormValue("item")
		barcode := r.FormValue("barcode")

		out := make(map[string]interface{})

		// Resolve item from barcode if provided
		if barcode != "" {
			row, err := database.RawQueryRow(
				"SELECT parent FROM `tabItem Barcode` WHERE barcode = ?", barcode,
			)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			if row == nil {
				writeError(w, http.StatusBadRequest, "Invalid Barcode. There is no Item attached to this barcode.")
				return
			}
			out["item"] = stock.ToString(row["parent"])
		} else {
			out["item"] = item
		}

		resolvedItem := stock.ToString(out["item"])

		// Get barcodes for the item
		barcodeRows, err := database.RawQuery(
			"SELECT barcode FROM `tabItem Barcode` WHERE parent = ?", resolvedItem,
		)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		barcodes := make([]string, 0, len(barcodeRows))
		for _, br := range barcodeRows {
			barcodes = append(barcodes, stock.ToString(br["barcode"]))
		}
		out["barcodes"] = barcodes

		// Get stock balance (qty only)
		qtyResult, err := stock.GetStockBalance(database, resolvedItem, warehouse, date, "", false)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		out["qty"] = qtyResult

		// Get stock value
		value, err := stock.GetStockValueOn(database, []string{warehouse}, date, resolvedItem, "")
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		out["value"] = value

		// Get item image
		imgRow, err := database.RawQueryRow(
			"SELECT image FROM `tabItem` WHERE name = ?", resolvedItem,
		)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if imgRow != nil {
			out["image"] = stock.ToString(imgRow["image"])
		} else {
			out["image"] = nil
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"message": out})
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
