package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPingEndpoint(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/method/go_erpnext.ping", nil)
	w := httptest.NewRecorder()

	pingHandler(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}

	if body["message"] != "pong" {
		t.Errorf("expected message 'pong', got '%s'", body["message"])
	}
}

func TestUnknownRoute(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/method/unknown", nil)
	w := httptest.NewRecorder()

	catchAllHandler(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", resp.StatusCode)
	}

	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}

	if body["error"] != "not implemented in Go service" {
		t.Errorf("expected error 'not implemented in Go service', got '%s'", body["error"])
	}
}

func TestHealthResponseFormat(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/method/go_erpnext.ping", nil)
	w := httptest.NewRecorder()

	pingHandler(w, req)

	resp := w.Result()
	contentType := resp.Header.Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("expected Content-Type 'application/json', got '%s'", contentType)
	}
}

func TestPingAcceptsPOST(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/method/go_erpnext.ping", nil)
	w := httptest.NewRecorder()

	pingHandler(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200 for POST, got %d", resp.StatusCode)
	}

	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}

	if body["message"] != "pong" {
		t.Errorf("expected message 'pong', got '%s'", body["message"])
	}
}
