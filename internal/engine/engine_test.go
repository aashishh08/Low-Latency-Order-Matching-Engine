package engine_test

import (
	"testing"

	"order-matching-engine/internal/common"
	"order-matching-engine/internal/engine"
)

// Helper to create order requests
func newReq(symbol string, side common.Side, typ common.OrderType, price, qty int64) *common.Order {
	return &common.Order{
		Symbol:   symbol,
		Side:     side,
		Type:     typ,
		Price:    price,
		Quantity: qty,
	}
}

// -------------------------
// FULL MATCH TEST
// -------------------------
func TestFullMatch(t *testing.T) {
	eng := engine.NewMatchingEngine()

	// Add resting SELL
	sell, _, err := eng.PlaceOrder(newReq("AAPL", common.SideSell, common.OrderTypeLimit, 15000, 100))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Incoming BUY fully matches
	buy, trades, err := eng.PlaceOrder(newReq("AAPL", common.SideBuy, common.OrderTypeLimit, 15000, 100))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if buy.Status != common.OrderStatusFilled {
		t.Fatalf("expected filled buy order")
	}
	if sell.Status != common.OrderStatusFilled {
		t.Fatalf("expected filled sell order")
	}
	if len(trades) != 1 || trades[0].Quantity != 100 {
		t.Fatalf("expected 1 trade of qty 100, got %#v", trades)
	}
}

// -------------------------
// PARTIAL FILL TEST
// -------------------------
func TestPartialFill(t *testing.T) {
	eng := engine.NewMatchingEngine()

	// Resting SELL = 100
	eng.PlaceOrder(newReq("AAPL", common.SideSell, common.OrderTypeLimit, 15000, 100))

	// Incoming BUY = 150 should partially fill
	buy, trades, err := eng.PlaceOrder(newReq("AAPL", common.SideBuy, common.OrderTypeLimit, 15000, 150))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if buy.Status != common.OrderStatusPartial {
		t.Fatalf("expected partial fill, got: %v", buy.Status)
	}
	if buy.FilledQty != 100 {
		t.Fatalf("expected 100 filled, got %d", buy.FilledQty)
	}
	if len(trades) != 1 || trades[0].Quantity != 100 {
		t.Fatalf("expected 1 trade of qty 100")
	}
}

// -------------------------
// MULTI-LEVEL (WALK THE BOOK)
// -------------------------
func TestWalkTheBook(t *testing.T) {
	eng := engine.NewMatchingEngine()

	// Resting SELL levels
	eng.PlaceOrder(newReq("AAPL", common.SideSell, common.OrderTypeLimit, 15000, 100)) // best
	eng.PlaceOrder(newReq("AAPL", common.SideSell, common.OrderTypeLimit, 15100, 200))

	// BUY walks through both levels
	buy, trades, err := eng.PlaceOrder(newReq("AAPL", common.SideBuy, common.OrderTypeLimit, 15100, 250))
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}

	if len(trades) != 2 {
		t.Fatalf("expected 2 trades, got %d", len(trades))
	}
	if buy.Status != common.OrderStatusFilled {
		t.Fatalf("expected filled, got %v", buy.Status)
	}
	if buy.FilledQty != 250 { // 100 + 150 from second level = 250 total
		t.Fatalf("wrong filled qty: %d", buy.FilledQty)
	}
}

// -------------------------
// MARKET ORDER FULL FILL
// -------------------------
func TestMarketOrderFullFill(t *testing.T) {
	eng := engine.NewMatchingEngine()

	eng.PlaceOrder(newReq("AAPL", common.SideSell, common.OrderTypeLimit, 10000, 100))
	eng.PlaceOrder(newReq("AAPL", common.SideSell, common.OrderTypeLimit, 10100, 50))

	mkt, trades, err := eng.PlaceOrder(&common.Order{
		Symbol:   "AAPL",
		Side:     common.SideBuy,
		Type:     common.OrderTypeMarket,
		Quantity: 150,
	})

	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}

	if mkt.Status != common.OrderStatusFilled {
		t.Fatalf("expected filled")
	}
	if len(trades) != 2 {
		t.Fatalf("expected 2 trades")
	}
}

// -------------------------
// MARKET ORDER INSUFFICIENT LIQUIDITY
// -------------------------
func TestMarketOrderInsufficient(t *testing.T) {
	eng := engine.NewMatchingEngine()

	// Only 50 available
	eng.PlaceOrder(newReq("AAPL", common.SideSell, common.OrderTypeLimit, 10000, 50))

	_, _, err := eng.PlaceOrder(&common.Order{
		Symbol:   "AAPL",
		Side:     common.SideBuy,
		Type:     common.OrderTypeMarket,
		Quantity: 100,
	})

	if err == nil {
		t.Fatalf("expected insufficient liquidity error")
	}
}

// -------------------------
// FIFO AT SAME PRICE
// -------------------------
func TestFIFO(t *testing.T) {
	eng := engine.NewMatchingEngine()

	// Two SELL orders at same price
	o1, _, _ := eng.PlaceOrder(newReq("AAPL", common.SideSell, common.OrderTypeLimit, 10000, 100))
	o2, _, _ := eng.PlaceOrder(newReq("AAPL", common.SideSell, common.OrderTypeLimit, 10000, 100))

	buy, trades, err := eng.PlaceOrder(newReq("AAPL", common.SideBuy, common.OrderTypeLimit, 10000, 150))
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}

	if len(trades) != 2 {
		t.Fatalf("expected 2 trades")
	}
	if trades[0].SellOrder != o1.ID {
		t.Fatalf("expected first order to match first (FIFO)")
	}
	if trades[1].SellOrder != o2.ID {
		t.Fatalf("expected second order next")
	}
	if buy.Status != common.OrderStatusFilled {
		t.Fatalf("expected filled")
	}
}

// -------------------------
// CANCEL ORDER
// -------------------------
func TestCancelOrder(t *testing.T) {
	eng := engine.NewMatchingEngine()

	o, _, _ := eng.PlaceOrder(newReq("AAPL", common.SideSell, common.OrderTypeLimit, 10000, 100))

	if err := eng.CancelOrder(o.ID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// BUY should not match because order is gone
	buy, trades, _ := eng.PlaceOrder(newReq("AAPL", common.SideBuy, common.OrderTypeLimit, 10000, 100))

	if buy.Status != common.OrderStatusAccepted {
		t.Fatalf("expected unfilled buy because cancel worked")
	}
	if len(trades) != 0 {
		t.Fatalf("expected 0 trades, got %d", len(trades))
	}
}
