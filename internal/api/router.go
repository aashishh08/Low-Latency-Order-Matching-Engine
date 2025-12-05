package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"order-matching-engine/internal/engine"
)

type API struct {
	Engine *engine.MatchingEngine
}

func NewAPI(e *engine.MatchingEngine) *API {
	return &API{
		Engine: e,
	}
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
// Handlers (logic added later)
// ---------------------------

func (a *API) health(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]any{
		"status": "healthy",
	})
}

func (a *API) metrics(w http.ResponseWriter, r *http.Request) {
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

func (a *API) placeOrder(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotImplemented)
	w.Write([]byte("order submission not implemented yet"))
}

func (a *API) cancelOrder(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotImplemented)
	w.Write([]byte("order cancellation not implemented yet"))
}

func (a *API) getOrder(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotImplemented)
	w.Write([]byte("get order not implemented yet"))
}

func (a *API) getOrderBook(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotImplemented)
	w.Write([]byte("orderbook not implemented yet"))
}
