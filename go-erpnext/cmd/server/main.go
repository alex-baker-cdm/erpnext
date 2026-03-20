package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/alex-baker-cdm/erpnext/go-erpnext/pkg/dashboard"
	"github.com/alex-baker-cdm/erpnext/go-erpnext/pkg/db"
	"github.com/alex-baker-cdm/erpnext/go-erpnext/pkg/queries"
	"github.com/alex-baker-cdm/erpnext/go-erpnext/pkg/reports"
)

// loggingMiddleware wraps an http.Handler and logs each request with method, path, status, and duration.
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(rw, r)
		duration := time.Since(start)
		log.Printf("%s %s %d %s", r.Method, r.URL.Path, rw.statusCode, duration)
	})
}

// responseWriter wraps http.ResponseWriter to capture the status code for logging.
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// pingHandler responds with {"message": "pong"} for the health check endpoint.
func pingHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "pong"})
}

// catchAllHandler responds with 404 for any route not yet implemented in the Go service.
func catchAllHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotFound)
	json.NewEncoder(w).Encode(map[string]string{"error": "not implemented in Go service"})
}

func main() {
	port := os.Getenv("GO_ERPNEXT_PORT")
	if port == "" {
		port = "8001"
	}

	// Initialize database connection (optional — handlers will return errors if DB is nil)
	sitePath := os.Getenv("FRAPPE_SITE_PATH")
	var dbConn *db.DB
	if sitePath != "" {
		cfg, err := db.ReadSiteConfig(sitePath)
		if err != nil {
			log.Printf("Warning: could not read site config: %v (DB-dependent routes will fail)", err)
		} else {
			dbConn, err = db.New(cfg)
			if err != nil {
				log.Printf("Warning: could not connect to database: %v (DB-dependent routes will fail)", err)
			} else {
				defer dbConn.Close()
			}
		}
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/method/go_erpnext.ping", pingHandler)

	// Register all reports — Phase 2D & 2E
	reports.RegisterAllReports(dbConn)
	reports.RegisterTier2Reports(dbConn)

	// Report runner — Phase 2A
	mux.HandleFunc("/api/method/frappe.desk.query_report.run", reports.RunReportHandler(dbConn))

	// Search query endpoints — Phase 2B
	// Simple queries
	mux.HandleFunc("/api/method/erpnext.controllers.queries.employee_query", queries.MakeHandler(dbConn, queries.EmployeeQuery))
	mux.HandleFunc("/api/method/erpnext.controllers.queries.lead_query", queries.MakeHandler(dbConn, queries.LeadQuery))
	mux.HandleFunc("/api/method/erpnext.controllers.queries.bom", queries.MakeHandler(dbConn, queries.BOMQuery))
	mux.HandleFunc("/api/method/erpnext.controllers.queries.get_batch_numbers", queries.MakeHandler(dbConn, queries.GetBatchNumbers))
	mux.HandleFunc("/api/method/erpnext.controllers.queries.item_manufacturer_query", queries.MakeHandler(dbConn, queries.ItemManufacturerQuery))
	mux.HandleFunc("/api/method/erpnext.controllers.queries.get_purchase_receipts", queries.MakeHandler(dbConn, queries.GetPurchaseReceipts))
	mux.HandleFunc("/api/method/erpnext.controllers.queries.get_purchase_invoices", queries.MakeHandler(dbConn, queries.GetPurchaseInvoices))
	mux.HandleFunc("/api/method/erpnext.controllers.queries.get_doctypes_for_closing", func(w http.ResponseWriter, r *http.Request) {
		queries.GetDoctypesForClosing(nil, w, r) // No DB needed
	})
	mux.HandleFunc("/api/method/erpnext.controllers.queries.get_payment_terms_for_references", queries.MakeHandler(dbConn, queries.GetPaymentTermsForReferences))
	mux.HandleFunc("/api/method/erpnext.controllers.queries.get_item_uom_query", queries.MakeHandler(dbConn, queries.GetItemUOMQuery))

	// Account queries
	mux.HandleFunc("/api/method/erpnext.controllers.queries.tax_account_query", queries.MakeHandler(dbConn, queries.TaxAccountQuery))
	mux.HandleFunc("/api/method/erpnext.controllers.queries.get_account_list", queries.MakeHandler(dbConn, queries.GetAccountList))
	mux.HandleFunc("/api/method/erpnext.controllers.queries.get_income_account", queries.MakeHandler(dbConn, queries.GetIncomeAccount))
	mux.HandleFunc("/api/method/erpnext.controllers.queries.get_expense_account", queries.MakeHandler(dbConn, queries.GetExpenseAccount))
	mux.HandleFunc("/api/method/erpnext.controllers.queries.get_blanket_orders", queries.MakeHandler(dbConn, queries.GetBlanketOrders))

	// Complex queries
	mux.HandleFunc("/api/method/erpnext.controllers.queries.item_query", queries.MakeHandler(dbConn, queries.ItemQuery))
	mux.HandleFunc("/api/method/erpnext.controllers.queries.get_batch_no", queries.MakeHandler(dbConn, queries.GetBatchNo))
	mux.HandleFunc("/api/method/erpnext.controllers.queries.warehouse_query", queries.MakeHandler(dbConn, queries.WarehouseQuery))
	mux.HandleFunc("/api/method/erpnext.controllers.queries.get_tax_template", queries.MakeHandler(dbConn, queries.GetTaxTemplate))
	mux.HandleFunc("/api/method/erpnext.controllers.queries.get_filtered_dimensions", queries.MakeHandler(dbConn, queries.GetFilteredDimensions))
	mux.HandleFunc("/api/method/erpnext.controllers.queries.get_filtered_child_rows", queries.MakeHandler(dbConn, queries.GetFilteredChildRows))
	mux.HandleFunc("/api/method/erpnext.controllers.queries.get_project_name", queries.MakeHandler(dbConn, queries.GetProjectName))
	mux.HandleFunc("/api/method/erpnext.controllers.queries.get_delivery_notes_to_be_billed", queries.MakeHandler(dbConn, queries.GetDeliveryNotesToBeBilled))

	// Dashboard chart sources — Phase 2C
	mux.HandleFunc("/api/method/erpnext.accounts.dashboard_chart_source.account_balance_timeline.account_balance_timeline.get", dashboard.MakeHandler(dbConn, dashboard.AccountBalanceTimeline))
	mux.HandleFunc("/api/method/erpnext.stock.dashboard_chart_source.warehouse_wise_stock_value.warehouse_wise_stock_value.get", dashboard.MakeHandler(dbConn, dashboard.WarehouseWiseStockValue))
	mux.HandleFunc("/api/method/erpnext.stock.dashboard_chart_source.stock_value_by_item_group.stock_value_by_item_group.get", dashboard.MakeHandler(dbConn, dashboard.StockValueByItemGroup))

	// Dashboard pages
	mux.HandleFunc("/api/method/erpnext.stock.dashboard.item_dashboard.get_data", dashboard.MakeHandler(dbConn, dashboard.ItemDashboardGetData))
	mux.HandleFunc("/api/method/erpnext.stock.dashboard.warehouse_capacity_dashboard.get_data", dashboard.MakeHandler(dbConn, dashboard.WarehouseCapacityGetData))

	mux.HandleFunc("/", catchAllHandler)

	handler := loggingMiddleware(mux)

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: handler,
	}

	// Channel to listen for OS signals for graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Printf("Go ERPNext server starting on port %s", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed to start: %v", err)
		}
	}()

	// Block until a signal is received
	sig := <-quit
	log.Printf("Received signal %v, shutting down gracefully...", sig)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server stopped")
}
