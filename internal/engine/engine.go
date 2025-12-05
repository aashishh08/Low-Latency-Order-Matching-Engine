package engine

import (
	"sync"
	"time"

	"order-matching-engine/internal/common"
	"order-matching-engine/internal/orderbook"

	"github.com/google/uuid"
)

// MatchingEngine is the core engine that manages order books and executes trades
type MatchingEngine struct {
	mu       sync.RWMutex
	books    map[string]*orderbook.OrderBook // per-symbol books
	orders   map[string]*common.Order        // global order lookup
	tradesMu sync.Mutex                      // protects trade aggregation
	trades   []*common.Trade
}

// NewMatchingEngine creates a new matching engine instance
func NewMatchingEngine() *MatchingEngine {
	return &MatchingEngine{
		books:  make(map[string]*orderbook.OrderBook),
		orders: make(map[string]*common.Order),
		trades: make([]*common.Trade, 0, 1024),
	}
}

// ensureBook returns the order book for the symbol, creating one if needed
func (m *MatchingEngine) ensureBook(symbol string) *orderbook.OrderBook {
	m.mu.Lock()
	defer m.mu.Unlock()

	if book, ok := m.books[symbol]; ok {
		return book
	}

	book := orderbook.NewOrderBook(symbol)
	m.books[symbol] = book
	return book
}

// createOrder generates a new order with server timestamp + UUID
func (m *MatchingEngine) createOrder(req *common.Order) *common.Order {
	o := &common.Order{
		ID:        uuid.NewString(),
		Symbol:    req.Symbol,
		Side:      req.Side,
		Type:      req.Type,
		Price:     req.Price,
		Quantity:  req.Quantity,
		FilledQty: 0,
		Status:    common.OrderStatusAccepted,
		Timestamp: time.Now().UnixMilli(),
	}
	return o
}

// PlaceOrder is the entry point for new incoming orders
// Matching logic will be added in later steps
func (m *MatchingEngine) PlaceOrder(req *common.Order) (*common.Order, []*common.Trade, error) {
	// Full logic to be implemented later
	return nil, nil, nil
}

// CancelOrder removes an existing and non-filled order from the book
func (m *MatchingEngine) CancelOrder(orderID string) error {
	// Logic to be implemented in a later step
	return nil
}

// GetOrder returns an order's full state
func (m *MatchingEngine) GetOrder(orderID string) (*common.Order, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	o, ok := m.orders[orderID]
	return o, ok
}

// GetOrderBook exposes the book (read-only usage by API)
func (m *MatchingEngine) GetOrderBook(symbol string) (*orderbook.OrderBook, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	b, ok := m.books[symbol]
	return b, ok
}

// addTrade safely appends a new trade to history
func (m *MatchingEngine) addTrade(t *common.Trade) {
	m.tradesMu.Lock()
	m.trades = append(m.trades, t)
	m.tradesMu.Unlock()
}
