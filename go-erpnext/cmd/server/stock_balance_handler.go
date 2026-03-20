package main

import (
	"encoding/json"
	"net/http"

	"github.com/alex-baker-cdm/erpnext/go-erpnext/pkg/db"
	"github.com/alex-baker-cdm/erpnext/go-erpnext/pkg/stock"
)

// stockBalanceHandler handles GET/POST /api/method/erpnext.stock.utils.get_stock_balance
//
// Query/form params: item_code, warehouse, posting_date, posting_time,
// with_valuation_rate, with_serial_no.
//
// If with_serial_no is truthy the request is returned as HTTP 501 so that
// nginx falls through to the Python backend.
func stockBalanceHandler(database *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()

		itemCode := r.FormValue("item_code")
		warehouse := r.FormValue("warehouse")
		postingDate := r.FormValue("posting_date")
		postingTime := r.FormValue("posting_time")
		withValuationRate := stock.ToBool(r.FormValue("with_valuation_rate"))
		withSerialNo := stock.ToBool(r.FormValue("with_serial_no"))

		if withSerialNo {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotImplemented)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "with_serial_no not implemented in Go service",
			})
			return
		}

		result, err := stock.GetStockBalance(database, itemCode, warehouse, postingDate, postingTime, withValuationRate)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"message": result})
	}
}
