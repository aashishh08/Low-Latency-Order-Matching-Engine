package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"order-matching-engine/internal/api"
	"order-matching-engine/internal/engine"
)

// Test all bonus features integration
func TestBonusFeaturesIntegration(t *testing.T) {
	eng := engine.NewMatchingEngine()
	apiLayer := api.NewAPI(eng)
	router := apiLayer.Router()

	// Test 1: Place orders to generate trades
	buyOrder := map[string]any{
		"symbol":   "AAPL",
		"side":     "BUY",
		"type":     "LIMIT",
		"price":    10000,
		"quantity": 100,
	}

	sellOrder := map[string]any{
		"symbol":   "AAPL",
		"side":     "SELL",
		"type":     "LIMIT",
		"price":    10000,
		"quantity": 50,
	}

	// Place buy order
	body, _ := json.Marshal(buyOrder)
	req := httptest.NewRequest("POST", "/api/v1/orders", bytes.NewReader(body))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("Expected 201, got %d", w.Code)
	}

	// Place sell order (should match)
	body, _ = json.Marshal(sellOrder)
	req = httptest.NewRequest("POST", "/api/v1/orders", bytes.NewReader(body))
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200 (filled), got %d", w.Code)
	}

	// Test 2: Check OHLCV data
	req = httptest.NewRequest("GET", "/api/v1/market/ohlcv/AAPL", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200 for OHLCV, got %d", w.Code)
	}

	var ohlcv map[string]any
	json.Unmarshal(w.Body.Bytes(), &ohlcv)
	if ohlcv["symbol"] != "AAPL" {
		t.Fatalf("Expected AAPL, got %v", ohlcv["symbol"])
	}

	// Test 3: Check trade history
	req = httptest.NewRequest("GET", "/api/v1/market/trades/AAPL?limit=10", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200 for trades, got %d", w.Code)
	}

	var tradesResp map[string]any
	json.Unmarshal(w.Body.Bytes(), &tradesResp)
	trades := tradesResp["trades"].([]any)
	if len(trades) != 1 {
		t.Fatalf("Expected 1 trade, got %d", len(trades))
	}

	// Test 4: Check market depth
	req = httptest.NewRequest("GET", "/api/v1/market/depth/AAPL?levels=5", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200 for depth, got %d", w.Code)
	}

	// Test 5: Prometheus metrics
	req = httptest.NewRequest("GET", "/metrics/prometheus", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200 for prometheus, got %d", w.Code)
	}

	if w.Header().Get("Content-Type") != "text/plain; version=0.0.4" {
		t.Fatalf("Wrong content-type for prometheus")
	}

	// Test 6: Health endpoints
	req = httptest.NewRequest("GET", "/health/live", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200 for /health/live, got %d", w.Code)
	}

	req = httptest.NewRequest("GET", "/health/ready", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200 for /health/ready, got %d", w.Code)
	}
}

// Test edge cases
func TestBonusFeaturesEdgeCases(t *testing.T) {
	eng := engine.NewMatchingEngine()
	apiLayer := api.NewAPI(eng)
	router := apiLayer.Router()

	// Test 1: OHLCV for non-existent symbol
	req := httptest.NewRequest("GET", "/api/v1/market/ohlcv/NONEXISTENT", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200 even for non-existent symbol, got %d", w.Code)
	}

	// Test 2: Empty symbol
	req = httptest.NewRequest("GET", "/api/v1/market/ohlcv/", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Should get 404 or 400 (depending on router behavior)
	if w.Code == http.StatusOK {
		t.Fatalf("Should not return 200 for empty symbol")
	}

	// Test 3: Invalid limit
	req = httptest.NewRequest("GET", "/api/v1/market/trades/AAPL?limit=-999", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Should handle invalid limit gracefully, got %d", w.Code)
	}

	// Test 4: Extremely large limit
	req = httptest.NewRequest("GET", "/api/v1/market/trades/AAPL?limit=999999", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Should cap large limit gracefully, got %d", w.Code)
	}
}
