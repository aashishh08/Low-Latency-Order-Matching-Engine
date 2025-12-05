package engine

import (
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"

	"order-matching-engine/internal/common"
	"order-matching-engine/internal/orderbook"
)

var (
	ErrInvalidOrderData      = errors.New("invalid order data")
	ErrInsufficientLiquidity = errors.New("insufficient liquidity")
	ErrOrderNotFound         = errors.New("order not found")
	ErrOrderAlreadyFinalized = errors.New("cannot cancel: order already filled or cancelled")
)

type MatchingEngine struct {
	mu     sync.RWMutex
	books  map[string]*orderbook.OrderBook // per-symbol books
	orders map[string]*common.Order        // global order lookup

	tradesMu sync.Mutex
	trades   []*common.Trade
}

func NewMatchingEngine() *MatchingEngine {
	return &MatchingEngine{
		books:  make(map[string]*orderbook.OrderBook),
		orders: make(map[string]*common.Order),
		trades: make([]*common.Trade, 0, 1024),
	}
}

// ensureBook assumes the caller holds m.mu (for write).
func (m *MatchingEngine) ensureBook(symbol string) *orderbook.OrderBook {
	if book, ok := m.books[symbol]; ok {
		return book
	}
	book := orderbook.NewOrderBook(symbol)
	m.books[symbol] = book
	return book
}

func (m *MatchingEngine) createOrder(req *common.Order) *common.Order {
	return &common.Order{
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
}

func validateOrderRequest(req *common.Order) error {
	if req == nil {
		return ErrInvalidOrderData
	}
	if req.Symbol == "" {
		return ErrInvalidOrderData
	}
	if req.Quantity <= 0 {
		return ErrInvalidOrderData
	}
	if req.Side != common.SideBuy && req.Side != common.SideSell {
		return ErrInvalidOrderData
	}
	if req.Type != common.OrderTypeLimit && req.Type != common.OrderTypeMarket {
		return ErrInvalidOrderData
	}
	if req.Type == common.OrderTypeLimit && req.Price <= 0 {
		return ErrInvalidOrderData
	}
	return nil
}

// PlaceOrder is the main entry point for new incoming orders.
// It handles validation, matching, and book insertion for remaining quantities.
func (m *MatchingEngine) PlaceOrder(req *common.Order) (*common.Order, []*common.Trade, error) {
	if err := validateOrderRequest(req); err != nil {
		return nil, nil, err
	}

	// Create server-side order instance.
	incoming := m.createOrder(req)

	m.mu.Lock()
	defer m.mu.Unlock()

	book := m.ensureBook(incoming.Symbol)

	var trades []*common.Trade
	switch incoming.Type {
	case common.OrderTypeLimit:
		trades = m.executeLimitOrder(book, incoming)
	case common.OrderTypeMarket:
		// Market orders must either fully execute or be rejected.
		if !m.hasSufficientLiquidityForMarket(book, incoming) {
			return nil, nil, ErrInsufficientLiquidity
		}
		trades = m.executeMarketOrder(book, incoming)
	default:
		return nil, nil, ErrInvalidOrderData
	}

	// Determine final status and decide whether to keep the order in the book.
	remaining := incoming.Quantity - incoming.FilledQty

	switch incoming.Type {
	case common.OrderTypeLimit:
		if remaining > 0 {
			// Partially or not filled: add remaining to the book.
			if incoming.FilledQty > 0 {
				incoming.Status = common.OrderStatusPartial
			} else {
				incoming.Status = common.OrderStatusAccepted
			}
			if incoming.Side == common.SideBuy {
				book.Bids.AddOrder(incoming)
			} else {
				book.Asks.AddOrder(incoming)
			}
		} else {
			incoming.Status = common.OrderStatusFilled
		}
	case common.OrderTypeMarket:
		// By construction, market orders are either fully filled or rejected earlier.
		if remaining == 0 {
			incoming.Status = common.OrderStatusFilled
		}
	}

	// Store final order state in lookup map.
	m.orders[incoming.ID] = incoming

	return incoming, trades, nil
}

// hasSufficientLiquidityForMarket checks if the opposite side has enough quantity
// to fully fill the incoming market order.
func (m *MatchingEngine) hasSufficientLiquidityForMarket(book *orderbook.OrderBook, o *common.Order) bool {
	var opposite *orderbook.SideBook
	if o.Side == common.SideBuy {
		opposite = book.Asks
	} else {
		opposite = book.Bids
	}
	return opposite.TotalQuantity >= o.Quantity
}

// executeLimitOrder walks the opposite book side while prices cross and fills as much
// as possible, respecting price-time priority and partial fills.
func (m *MatchingEngine) executeLimitOrder(book *orderbook.OrderBook, o *common.Order) []*common.Trade {
	var opposite *orderbook.SideBook
	if o.Side == common.SideBuy {
		opposite = book.Asks
	} else {
		opposite = book.Bids
	}

	trades := make([]*common.Trade, 0, 4)
	remaining := o.Quantity - o.FilledQty

	for remaining > 0 {
		level, ok := opposite.BestLevel()
		if !ok {
			break // no liquidity
		}

		// Check price crossing condition.
		if o.Side == common.SideBuy && level.Price > o.Price {
			break
		}
		if o.Side == common.SideSell && level.Price < o.Price {
			break
		}

		// Consume orders at this price level in FIFO order.
		for remaining > 0 && !level.IsEmpty() {
			existing := level.Orders[0]
			existingRemaining := existing.Quantity - existing.FilledQty
			if existingRemaining <= 0 {
				_, _ = level.Dequeue()
				continue
			}

			qty := remaining
			if existingRemaining < qty {
				qty = existingRemaining
			}

			remaining -= qty
			o.FilledQty += qty
			existing.FilledQty += qty

			if existing.FilledQty == existing.Quantity {
				existing.Status = common.OrderStatusFilled
			} else {
				existing.Status = common.OrderStatusPartial
			}

			// Update aggregate liquidity.
			opposite.TotalQuantity -= qty

			// Record trade.
			buyID := o.ID
			sellID := existing.ID
			if o.Side == common.SideSell {
				buyID = existing.ID
				sellID = o.ID
			}

			trade := &common.Trade{
				TradeID:   uuid.NewString(),
				BuyOrder:  buyID,
				SellOrder: sellID,
				Price:     level.Price,
				Quantity:  qty,
				Timestamp: time.Now().UnixMilli(),
			}
			trades = append(trades, trade)
			m.addTrade(trade)

			if existing.FilledQty == existing.Quantity {
				_, _ = level.Dequeue()
			}
		}

		if level.IsEmpty() {
			opposite.RemovePrice(level.Price)
		}

		if remaining == 0 {
			break
		}
	}

	return trades
}

// executeMarketOrder walks the opposite book side regardless of price,
// assuming sufficient liquidity has already been validated.
func (m *MatchingEngine) executeMarketOrder(book *orderbook.OrderBook, o *common.Order) []*common.Trade {
	var opposite *orderbook.SideBook
	if o.Side == common.SideBuy {
		opposite = book.Asks
	} else {
		opposite = book.Bids
	}

	trades := make([]*common.Trade, 0, 4)
	remaining := o.Quantity - o.FilledQty

	for remaining > 0 {
		level, ok := opposite.BestLevel()
		if !ok {
			break // should not happen if liquidity was checked
		}

		for remaining > 0 && !level.IsEmpty() {
			existing := level.Orders[0]
			existingRemaining := existing.Quantity - existing.FilledQty
			if existingRemaining <= 0 {
				_, _ = level.Dequeue()
				continue
			}

			qty := remaining
			if existingRemaining < qty {
				qty = existingRemaining
			}

			remaining -= qty
			o.FilledQty += qty
			existing.FilledQty += qty

			if existing.FilledQty == existing.Quantity {
				existing.Status = common.OrderStatusFilled
			} else {
				existing.Status = common.OrderStatusPartial
			}

			opposite.TotalQuantity -= qty

			buyID := o.ID
			sellID := existing.ID
			if o.Side == common.SideSell {
				buyID = existing.ID
				sellID = o.ID
			}

			trade := &common.Trade{
				TradeID:   uuid.NewString(),
				BuyOrder:  buyID,
				SellOrder: sellID,
				Price:     level.Price,
				Quantity:  qty,
				Timestamp: time.Now().UnixMilli(),
			}
			trades = append(trades, trade)
			m.addTrade(trade)

			if existing.FilledQty == existing.Quantity {
				_, _ = level.Dequeue()
			}
		}

		if level.IsEmpty() {
			opposite.RemovePrice(level.Price)
		}

		if remaining == 0 {
			break
		}
	}

	return trades
}

// CancelOrder removes any remaining quantity of an order from the book and
// marks it as cancelled. It is synchronous: once it returns, the order will
// not participate in future matches.
func (m *MatchingEngine) CancelOrder(orderID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	o, ok := m.orders[orderID]
	if !ok {
		return ErrOrderNotFound
	}

	if o.Status == common.OrderStatusFilled || o.Status == common.OrderStatusCancelled {
		return ErrOrderAlreadyFinalized
	}

	book, ok := m.books[o.Symbol]
	if !ok {
		return ErrOrderNotFound
	}

	var sideBook *orderbook.SideBook
	if o.Side == common.SideBuy {
		sideBook = book.Bids
	} else {
		sideBook = book.Asks
	}

	level, ok := sideBook.Levels[o.Price]
	if !ok {
		// It might already be fully matched but status not updated; treat as finalized.
		return ErrOrderAlreadyFinalized
	}

	remaining := o.Quantity - o.FilledQty
	if remaining <= 0 {
		return ErrOrderAlreadyFinalized
	}

	// Remove the order from the level's FIFO queue.
	idx := -1
	for i, ord := range level.Orders {
		if ord.ID == o.ID {
			idx = i
			break
		}
	}
	if idx == -1 {
		return ErrOrderAlreadyFinalized
	}

	level.Orders = append(level.Orders[:idx], level.Orders[idx+1:]...)
	sideBook.TotalQuantity -= remaining

	if level.IsEmpty() {
		sideBook.RemovePrice(level.Price)
	}

	o.Status = common.OrderStatusCancelled
	return nil
}

func (m *MatchingEngine) GetOrder(orderID string) (*common.Order, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	o, ok := m.orders[orderID]
	return o, ok
}

func (m *MatchingEngine) GetOrderBook(symbol string) (*orderbook.OrderBook, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	b, ok := m.books[symbol]
	return b, ok
}

func (m *MatchingEngine) addTrade(t *common.Trade) {
	m.tradesMu.Lock()
	m.trades = append(m.trades, t)
	m.tradesMu.Unlock()
}
