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

	// Optional: connect to database if site path is configured
	var dbConn *db.DB
	sitePath := os.Getenv("FRAPPE_SITE_PATH")
	if sitePath != "" {
		cfg, err := db.ReadSiteConfig(sitePath)
		if err != nil {
			log.Printf("Warning: could not read site config: %v (dashboard endpoints will not work)", err)
		} else {
			dbConn, err = db.New(cfg)
			if err != nil {
				log.Printf("Warning: could not connect to database: %v (dashboard endpoints will not work)", err)
			} else {
				defer dbConn.Close()
			}
		}
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/method/go_erpnext.ping", pingHandler)

	// Dashboard chart sources
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
