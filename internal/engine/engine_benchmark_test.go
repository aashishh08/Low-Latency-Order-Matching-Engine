package engine_test

import (
	"math/rand"
	"testing"
	"time"

	"order-matching-engine/internal/common"
	"order-matching-engine/internal/engine"
)

var symbols = []string{"AAPL", "GOOGL", "MSFT", "TSLA", "AMZN"}

func init() {
	rand.Seed(time.Now().UnixNano())
}

func randomSymbol() string {
	return symbols[rand.Intn(len(symbols))]
}

func randomSide() common.Side {
	if rand.Intn(2) == 0 {
		return common.SideBuy
	}
	return common.SideSell
}

func randomOrderType() common.OrderType {
	// MARKET only sometimes to avoid unrealistic failure rates
	if rand.Intn(10) < 2 {
		return common.OrderTypeMarket
	}
	return common.OrderTypeLimit
}

func randomPrice() int64 {
	// Prices distributed around $100-$200 range
	return 10000 + rand.Int63n(10000)
}

func randomQty() int64 {
	return int64(1 + rand.Intn(500))
}

// Benchmark with realistic preloaded liquidity
func BenchmarkMatchingEngine(b *testing.B) {
	eng := engine.NewMatchingEngine()

	// Preload order books with realistic liquidity
	for i := 0; i < 5000; i++ {
		eng.PlaceOrder(&common.Order{
			Symbol:   randomSymbol(),
			Side:     randomSide(),
			Type:     common.OrderTypeLimit,
			Price:    randomPrice(),
			Quantity: randomQty(),
		})
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		symbol := randomSymbol()
		side := randomSide()
		typ := randomOrderType()
		qty := randomQty()

		price := int64(0)
		if typ == common.OrderTypeLimit {
			price = randomPrice()
		}

		_, _, _ = eng.PlaceOrder(&common.Order{
			Symbol:   symbol,
			Side:     side,
			Type:     typ,
			Price:    price,
			Quantity: qty,
		})
	}
}
