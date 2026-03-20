package main

import (
	"encoding/json"
	"net/http"

	"github.com/alex-baker-cdm/erpnext/go-erpnext/pkg/db"
	"github.com/alex-baker-cdm/erpnext/go-erpnext/pkg/stock"
)

// stockQtyHandler handles GET/POST /api/method/erpnext.stock.utils.get_latest_stock_qty
//
// Query/form params: item_code, warehouse.
func stockQtyHandler(database *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()

		itemCode := r.FormValue("item_code")
		warehouse := r.FormValue("warehouse")

		qty, err := stock.GetLatestStockQty(database, itemCode, warehouse)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"message": qty})
	}
}
