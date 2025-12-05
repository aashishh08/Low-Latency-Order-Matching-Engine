package marketdata

import (
	"sync"
	"time"

	"order-matching-engine/internal/common"
)

type OHLCV struct {
	Symbol    string `json:"symbol"`
	Open      int64  `json:"open"`
	High      int64  `json:"high"`
	Low       int64  `json:"low"`
	Close     int64  `json:"close"`
	Volume    int64  `json:"volume"`
	Timestamp int64  `json:"timestamp"`
}

type MarketData struct {
	mu     sync.RWMutex
	ohlcv  map[string]*OHLCV          // symbol -> current OHLCV
	trades map[string][]*common.Trade // symbol -> recent trades
}

func NewMarketData() *MarketData {
	return &MarketData{
		ohlcv:  make(map[string]*OHLCV),
		trades: make(map[string][]*common.Trade),
	}
}

func (m *MarketData) RecordTrade(trade *common.Trade, symbol string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Update OHLCV
	candle, exists := m.ohlcv[symbol]
	if !exists {
		candle = &OHLCV{
			Symbol:    symbol,
			Open:      trade.Price,
			High:      trade.Price,
			Low:       trade.Price,
			Close:     trade.Price,
			Volume:    0,
			Timestamp: time.Now().UnixMilli(),
		}
		m.ohlcv[symbol] = candle
	}

	if trade.Price > candle.High {
		candle.High = trade.Price
	}
	if trade.Price < candle.Low || candle.Low == 0 {
		candle.Low = trade.Price
	}
	candle.Close = trade.Price
	candle.Volume += trade.Quantity

	// Store trade history (last 1000 trades per symbol)
	if m.trades[symbol] == nil {
		m.trades[symbol] = make([]*common.Trade, 0, 1000)
	}
	m.trades[symbol] = append(m.trades[symbol], trade)
	if len(m.trades[symbol]) > 1000 {
		m.trades[symbol] = m.trades[symbol][1:]
	}
}

func (m *MarketData) GetOHLCV(symbol string) *OHLCV {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if candle, ok := m.ohlcv[symbol]; ok {
		// Return copy
		copy := *candle
		return &copy
	}
	return nil
}

func (m *MarketData) GetRecentTrades(symbol string, limit int) []*common.Trade {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Validate limit
	if limit <= 0 {
		limit = 100
	}
	if limit > 10000 {
		limit = 10000 // Cap at 10k to prevent memory issues
	}

	trades := m.trades[symbol]
	if trades == nil {
		return []*common.Trade{}
	}

	start := 0
	if len(trades) > limit {
		start = len(trades) - limit
	}

	result := make([]*common.Trade, len(trades)-start)
	copy(result, trades[start:])
	return result
}
