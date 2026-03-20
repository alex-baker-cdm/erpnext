package main

import (
	"encoding/json"
	"net/http"

	"github.com/alex-baker-cdm/erpnext/go-erpnext/pkg/db"
	"github.com/alex-baker-cdm/erpnext/go-erpnext/pkg/stock"
)

// valuationRateHandler handles GET/POST /api/method/erpnext.stock.get_item_details.get_valuation_rate
//
// Query/form params: item_code, company, warehouse.
func valuationRateHandler(database *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()

		itemCode := r.FormValue("item_code")
		company := r.FormValue("company")
		warehouse := r.FormValue("warehouse")

		result, err := stock.GetValuationRate(database, itemCode, company, warehouse)
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
