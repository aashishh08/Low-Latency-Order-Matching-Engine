package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"order-matching-engine/internal/common"
	"order-matching-engine/internal/engine"
)

type API struct {
	Engine *engine.MatchingEngine
}

func NewAPI(e *engine.MatchingEngine) *API {
	return &API{Engine: e}
}

func (a *API) Router() http.Handler {
	r := chi.NewRouter()

	r.Get("/health", a.health)
	r.Get("/metrics", a.metrics)

	r.Post("/api/v1/orders", a.placeOrder)
	r.Delete("/api/v1/orders/{id}", a.cancelOrder)
	r.Get("/api/v1/orders/{id}", a.getOrder)

	r.Get("/api/v1/orderbook/{symbol}", a.getOrderBook)

	return r
}

// ---------------------------
// Handlers
// ---------------------------

func (a *API) health(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]any{"status": "healthy"})
}

func (a *API) metrics(w http.ResponseWriter, r *http.Request) {
	// In a later step we will wire real metrics.
	json.NewEncoder(w).Encode(map[string]any{
		"orders_received":   0,
		"orders_matched":    0,
		"orders_cancelled":  0,
		"orders_in_book":    0,
		"trades_executed":   0,
		"latency_p50_ms":    0,
		"latency_p99_ms":    0,
		"latency_p999_ms":   0,
		"throughput_orders": 0,
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
