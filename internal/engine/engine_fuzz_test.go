package engine_test

import (
	"testing"

	"order-matching-engine/internal/common"
	"order-matching-engine/internal/engine"
)

// Fuzz test: random order placement should never panic
func FuzzPlaceOrder(f *testing.F) {
	// Seed corpus
	f.Add("AAPL", int64(10000), int64(100))
	f.Add("GOOGL", int64(15000), int64(50))

	f.Fuzz(func(t *testing.T, symbol string, price int64, qty int64) {
		eng := engine.NewMatchingEngine()

		// Normalize inputs
		if price < 0 {
			price = -price
		}
		if qty < 0 {
			qty = -qty
		}

		side := common.SideBuy
		if qty%2 == 0 {
			side = common.SideSell
		}

		order := &common.Order{
			Symbol:   symbol,
			Side:     side,
			Type:     common.OrderTypeLimit,
			Price:    price,
			Quantity: qty,
		}

		// Should not panic
		_, _, _ = eng.PlaceOrder(order)
	})
}

// Fuzz test: random cancellations should never panic
func FuzzCancelOrder(f *testing.F) {
	f.Add("order-123")
	f.Add("")
	f.Add("non-existent")

	f.Fuzz(func(t *testing.T, orderID string) {
		eng := engine.NewMatchingEngine()

		// Should not panic even with invalid order IDs
		_ = eng.CancelOrder(orderID)
	})
}
