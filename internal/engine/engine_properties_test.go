package engine_test

import (
	"testing"

	"order-matching-engine/internal/common"
	"order-matching-engine/internal/engine"
)

// Property: Total filled quantity never exceeds original quantity
func TestProperty_FilledNeverExceedsTotal(t *testing.T) {
	eng := engine.NewMatchingEngine()

	// Place a large sell order
	eng.PlaceOrder(&common.Order{
		Symbol:   "AAPL",
		Side:     common.SideSell,
		Type:     common.OrderTypeLimit,
		Price:    10000,
		Quantity: 1000,
	})

	// Place multiple buy orders that will fill parts of it
	for i := 0; i < 20; i++ {
		order, trades, _ := eng.PlaceOrder(&common.Order{
			Symbol:   "AAPL",
			Side:     common.SideBuy,
			Type:     common.OrderTypeLimit,
			Price:    10000,
			Quantity: 100,
		})

		// Property: filled quantity must not exceed total quantity
		if order.FilledQty > order.Quantity {
			t.Fatalf("Filled qty (%d) exceeds total (%d)", order.FilledQty, order.Quantity)
		}

		// Property: sum of trade quantities equals filled quantity
		tradeSum := int64(0)
		for _, trade := range trades {
			tradeSum += trade.Quantity
		}
		if tradeSum != order.FilledQty {
			t.Fatalf("Trade sum (%d) != FilledQty (%d)", tradeSum, order.FilledQty)
		}
	}
}

// Property: Trades always use resting order price
func TestProperty_TradesUseRestingPrice(t *testing.T) {
	eng := engine.NewMatchingEngine()

	// Place resting sell at 10000
	eng.PlaceOrder(&common.Order{
		Symbol:   "AAPL",
		Side:     common.SideSell,
		Type:     common.OrderTypeLimit,
		Price:    10000,
		Quantity: 100,
	})

	// Incoming buy at higher price (10500)
	_, trades, _ := eng.PlaceOrder(&common.Order{
		Symbol:   "AAPL",
		Side:     common.SideBuy,
		Type:     common.OrderTypeLimit,
		Price:    10500,
		Quantity: 50,
	})

	// Property: trade should execute at resting order price (10000)
	for _, trade := range trades {
		if trade.Price != 10000 {
			t.Fatalf("Trade executed at %d, expected resting price 10000", trade.Price)
		}
	}
}

// Property: Order book maintains sorted price levels
func TestProperty_OrderBookSorted(t *testing.T) {
	eng := engine.NewMatchingEngine()

	// Place random orders
	prices := []int64{10500, 10000, 10200, 10100, 10300}
	for _, price := range prices {
		eng.PlaceOrder(&common.Order{
			Symbol:   "AAPL",
			Side:     common.SideSell,
			Type:     common.OrderTypeLimit,
			Price:    price,
			Quantity: 100,
		})
	}

	book, _ := eng.GetOrderBook("AAPL")

	// Property: Asks must be sorted ascending
	for i := 1; i < len(book.Asks.Prices); i++ {
		if book.Asks.Prices[i] < book.Asks.Prices[i-1] {
			t.Fatalf("Asks not sorted: %d comes after %d", book.Asks.Prices[i], book.Asks.Prices[i-1])
		}
	}

	// Add bid orders
	for _, price := range prices {
		eng.PlaceOrder(&common.Order{
			Symbol:   "AAPL",
			Side:     common.SideBuy,
			Type:     common.OrderTypeLimit,
			Price:    price,
			Quantity: 100,
		})
	}

	book, _ = eng.GetOrderBook("AAPL")

	// Property: Bids must be sorted descending
	for i := 1; i < len(book.Bids.Prices); i++ {
		if book.Bids.Prices[i] > book.Bids.Prices[i-1] {
			t.Fatalf("Bids not sorted: %d comes after %d", book.Bids.Prices[i], book.Bids.Prices[i-1])
		}
	}
}
