package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

// GET /api/v1/market/ohlcv/{symbol}
func (a *API) getOHLCV(w http.ResponseWriter, r *http.Request) {
	symbol := chi.URLParam(r, "symbol")
	if symbol == "" {
		http.Error(w, "symbol required", http.StatusBadRequest)
		return
	}

	ohlcv := a.MarketData.GetOHLCV(symbol)
	if ohlcv == nil {
		json.NewEncoder(w).Encode(map[string]any{
			"symbol": symbol,
			"data":   nil,
		})
		return
	}

	json.NewEncoder(w).Encode(ohlcv)
}

// GET /api/v1/market/trades/{symbol}?limit=100
func (a *API) getTrades(w http.ResponseWriter, r *http.Request) {
	symbol := chi.URLParam(r, "symbol")
	if symbol == "" {
		http.Error(w, "symbol required", http.StatusBadRequest)
		return
	}

	limitStr := r.URL.Query().Get("limit")
	limit := 100
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	trades := a.MarketData.GetRecentTrades(symbol, limit)
	json.NewEncoder(w).Encode(map[string]any{
		"symbol": symbol,
		"trades": trades,
	})
}

// GET /api/v1/market/depth/{symbol}?levels=10
func (a *API) getDepth(w http.ResponseWriter, r *http.Request) {
	symbol := chi.URLParam(r, "symbol")

	book, ok := a.Engine.GetOrderBook(symbol)
	if !ok {
		http.Error(w, "Symbol not found", http.StatusNotFound)
		return
	}

	levelsStr := r.URL.Query().Get("levels")
	levels := 10
	if levelsStr != "" {
		if l, err := strconv.Atoi(levelsStr); err == nil {
			levels = l
		}
	}

	// Build depth response
	bids := make([]map[string]any, 0, levels)
	asks := make([]map[string]any, 0, levels)

	for i, price := range book.Bids.Prices {
		if i >= levels {
			break
		}
		level := book.Bids.Levels[price]
		qty := int64(0)
		for _, o := range level.Orders {
			qty += o.Quantity - o.FilledQty
		}
		bids = append(bids, map[string]any{
			"price":    price,
			"quantity": qty,
		})
	}

	for i, price := range book.Asks.Prices {
		if i >= levels {
			break
		}
		level := book.Asks.Levels[price]
		qty := int64(0)
		for _, o := range level.Orders {
			qty += o.Quantity - o.FilledQty
		}
		asks = append(asks, map[string]any{
			"price":    price,
			"quantity": qty,
		})
	}

	json.NewEncoder(w).Encode(map[string]any{
		"symbol": symbol,
		"bids":   bids,
		"asks":   asks,
	})
}
