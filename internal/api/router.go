package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/go-chi/chi/v5"

	"order-matching-engine/internal/common"
	"order-matching-engine/internal/engine"
	"order-matching-engine/internal/marketdata"
)

type API struct {
	Engine     *engine.MatchingEngine
	WSHub      *WSHub
	MarketData *marketdata.MarketData
	startTime  time.Time
}

func NewAPI(e *engine.MatchingEngine) *API {
	return &API{
		Engine:     e,
		WSHub:      NewWSHub(),
		MarketData: marketdata.NewMarketData(),
		startTime:  time.Now(),
	}
}

func (a *API) Router() http.Handler {
	r := chi.NewRouter()

	// Health and metrics
	r.Get("/health", a.health)
	r.Get("/health/live", a.healthLive)
	r.Get("/health/ready", a.healthReady)
	r.Get("/metrics", a.metrics)
	r.Get("/metrics/prometheus", a.metricsPrometheus)

	// Core order endpoints
	r.Post("/api/v1/orders", a.placeOrder)
	r.Delete("/api/v1/orders/{id}", a.cancelOrder)
	r.Get("/api/v1/orders/{id}", a.getOrder)
	r.Get("/api/v1/orderbook/{symbol}", a.getOrderBook)

	// Market data endpoints
	r.Get("/api/v1/market/ohlcv/{symbol}", a.getOHLCV)
	r.Get("/api/v1/market/trades/{symbol}", a.getTrades)
	r.Get("/api/v1/market/depth/{symbol}", a.getDepth)

	// WebSocket endpoint
	r.Get("/ws/{symbol}", a.handleWebSocket)

	return r
}

// ---------------------------
// Handlers
// ---------------------------

func (a *API) health(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]any{
		"status": "healthy",
		"uptime": time.Since(a.startTime).Seconds(),
	})
}

func (a *API) healthLive(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]any{"status": "live"})
}

func (a *API) healthReady(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]any{"status": "ready"})
}

func (a *API) metricsPrometheus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	w.Write([]byte(a.Engine.Metrics.PrometheusFormat()))
}

func (a *API) metrics(w http.ResponseWriter, r *http.Request) {
	p50, p99, p999 := a.Engine.Metrics.Percentiles()

	json.NewEncoder(w).Encode(map[string]any{
		"orders_received":   atomic.LoadUint64(&a.Engine.Metrics.OrdersReceived),
		"orders_matched":    atomic.LoadUint64(&a.Engine.Metrics.OrdersMatched),
		"orders_cancelled":  atomic.LoadUint64(&a.Engine.Metrics.OrdersCancelled),
		"trades_executed":   atomic.LoadUint64(&a.Engine.Metrics.TradesExecuted),
		"orders_in_book":    a.Engine.OrdersInBook(),
		"latency_p50_ms":    p50,
		"latency_p99_ms":    p99,
		"latency_p999_ms":   p999,
		"throughput_orders": a.Engine.Metrics.Throughput(),
	})
}

// POST /api/v1/orders
func (a *API) placeOrder(w http.ResponseWriter, r *http.Request) {
	var req common.Order

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Malformed JSON", http.StatusBadRequest)
		return
	}

	order, trades, err := a.Engine.PlaceOrder(&req)
	if err != nil {
		switch err {
		case engine.ErrInvalidOrderData:
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		case engine.ErrInsufficientLiquidity:
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		default:
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	// Broadcast trades via WebSocket & record market data
	for _, trade := range trades {
		a.WSHub.BroadcastTrade(order.Symbol, trade)
		a.MarketData.RecordTrade(trade, order.Symbol)
	}

	resp := map[string]any{
		"order_id":           order.ID,
		"status":             order.Status,
		"filled_quantity":    order.FilledQty,
		"remaining_quantity": order.Quantity - order.FilledQty,
		"trades":             trades,
	}

	// Response code rules:
	// - 201: order accepted (no matches)
	// - 202: partial fill
	// - 200: fully filled

	if order.Status == common.OrderStatusAccepted {
		w.WriteHeader(http.StatusCreated)
	} else if order.Status == common.OrderStatusPartial {
		w.WriteHeader(http.StatusAccepted)
	} else if order.Status == common.OrderStatusFilled {
		w.WriteHeader(http.StatusOK)
	}

	json.NewEncoder(w).Encode(resp)
}

// DELETE /api/v1/orders/{id}
func (a *API) cancelOrder(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	err := a.Engine.CancelOrder(id)
	if err != nil {
		switch err {
		case engine.ErrOrderNotFound:
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		case engine.ErrOrderAlreadyFinalized:
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		default:
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	json.NewEncoder(w).Encode(map[string]any{
		"order_id": id,
		"status":   "CANCELLED",
	})
}

// GET /api/v1/orders/{id}
func (a *API) getOrder(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	o, ok := a.Engine.GetOrder(id)
	if !ok {
		http.Error(w, "Order not found", http.StatusNotFound)
		return
	}

	json.NewEncoder(w).Encode(o)
}

// GET /api/v1/orderbook/{symbol}?depth=10
func (a *API) getOrderBook(w http.ResponseWriter, r *http.Request) {
	symbol := chi.URLParam(r, "symbol")

	book, ok := a.Engine.GetOrderBook(symbol)
	if !ok {
		http.Error(w, "Symbol not found", http.StatusNotFound)
		return
	}

	depthStr := r.URL.Query().Get("depth")
	depth := 10
	if depthStr != "" {
		if d, err := strconv.Atoi(depthStr); err == nil {
			depth = d
		}
	}

	// Aggregate quantities per price level.
	bids := make([]map[string]any, 0, depth)
	asks := make([]map[string]any, 0, depth)

	for i, price := range book.Bids.Prices {
		if i >= depth {
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
		if i >= depth {
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
