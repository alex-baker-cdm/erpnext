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

	// Initialize database connection from Frappe site config.
	sitePath := os.Getenv("FRAPPE_SITE_PATH")
	if sitePath == "" {
		sitePath = "/home/frappe/frappe-bench/sites/frontend"
	}

	var database *db.DB

	cfg, err := db.ReadSiteConfig(sitePath)
	if err != nil {
		log.Printf("WARNING: Could not read site config from %s: %v (running without DB)", sitePath, err)
	} else {
		database, err = db.New(cfg)
		if err != nil {
			log.Printf("WARNING: Could not connect to database: %v (running without DB)", err)
		} else {
			defer database.Close()
			log.Printf("Connected to database %s@%s:%d/%s", cfg.DBUser, cfg.DBHost, cfg.DBPort, cfg.DBName)
		}
	}

	mux := http.NewServeMux()

	// Health check - no auth required
	mux.HandleFunc("/api/method/go_erpnext.ping", pingHandler)

	// Stock query endpoints - auth required
	if database != nil {
		auth := authMiddleware(database)

		mux.Handle("/api/method/erpnext.stock.utils.get_stock_balance",
			auth(http.HandlerFunc(stockBalanceHandler(database))))
		mux.Handle("/api/method/erpnext.stock.utils.get_latest_stock_qty",
			auth(http.HandlerFunc(stockQtyHandler(database))))
		mux.Handle("/api/method/erpnext.stock.get_item_details.get_valuation_rate",
			auth(http.HandlerFunc(valuationRateHandler(database))))
		mux.Handle("/api/method/erpnext.stock.doctype.quick_stock_balance.quick_stock_balance.get_stock_item_details",
			auth(http.HandlerFunc(stockItemHandler(database))))
	}

	// Catch-all for unimplemented routes
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
