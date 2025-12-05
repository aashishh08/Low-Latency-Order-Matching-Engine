package orderbook

import (
	"order-matching-engine/internal/common"
)

// PriceLevel represents all orders at a specific price level
type PriceLevel struct {
	Price  int64
	Orders []*common.Order // FIFO queue of orders at this price
}

// NewPriceLevel creates a new price level
func NewPriceLevel(price int64) *PriceLevel {
	return &PriceLevel{
		Price:  price,
		Orders: make([]*common.Order, 0, 8),
	}
}

// Enqueue adds an order to the end of the queue (FIFO)
func (pl *PriceLevel) Enqueue(o *common.Order) {
	pl.Orders = append(pl.Orders, o)
}

// Dequeue removes and returns the first order (FIFO)
func (pl *PriceLevel) Dequeue() (*common.Order, bool) {
	if len(pl.Orders) == 0 {
		return nil, false
	}
	o := pl.Orders[0]
	pl.Orders = pl.Orders[1:]
	return o, true
}

// IsEmpty returns true if there are no orders at this price level
func (pl *PriceLevel) IsEmpty() bool {
	return len(pl.Orders) == 0
}

// SideBook manages all price levels for one side (BUY or SELL)
type SideBook struct {
	IsBuy  bool
	Levels map[int64]*PriceLevel // key = price
	Prices []int64               // sorted slice of prices
}

// NewSideBook creates a new side book
func NewSideBook(isBuy bool) *SideBook {
	return &SideBook{
		IsBuy:  isBuy,
		Levels: make(map[int64]*PriceLevel),
		Prices: make([]int64, 0, 32),
	}
}

// InsertPrice ensures a price level exists and is tracked in sorted order
func (sb *SideBook) InsertPrice(price int64) *PriceLevel {
	if level, ok := sb.Levels[price]; ok {
		return level
	}

	level := NewPriceLevel(price)
	sb.Levels[price] = level
	sb.insertPriceSorted(price)
	return level
}

// insertPriceSorted inserts a price into the sorted list
// BUY side: highest price first (descending)
// SELL side: lowest price first (ascending)
func (sb *SideBook) insertPriceSorted(price int64) {
	inserted := false
	for i, p := range sb.Prices {
		if sb.IsBuy {
			if price > p {
				sb.Prices = append(sb.Prices[:i], append([]int64{price}, sb.Prices[i:]...)...)
				inserted = true
				break
			}
		} else {
			if price < p {
				sb.Prices = append(sb.Prices[:i], append([]int64{price}, sb.Prices[i:]...)...)
				inserted = true
				break
			}
		}
	}
	if !inserted {
		sb.Prices = append(sb.Prices, price)
	}
}

// BestPrice returns the best price for this side
func (sb *SideBook) BestPrice() (int64, bool) {
	if len(sb.Prices) == 0 {
		return 0, false
	}
	return sb.Prices[0], true
}

// BestLevel returns the price level at the best price
func (sb *SideBook) BestLevel() (*PriceLevel, bool) {
	price, ok := sb.BestPrice()
	if !ok {
		return nil, false
	}
	return sb.Levels[price], true
}

// OrderBook represents the full order book for a symbol
type OrderBook struct {
	Symbol string
	Bids   *SideBook // BUY orders
	Asks   *SideBook // SELL orders
}

// NewOrderBook creates a new order book for a symbol
func NewOrderBook(symbol string) *OrderBook {
	return &OrderBook{
		Symbol: symbol,
		Bids:   NewSideBook(true),
		Asks:   NewSideBook(false),
	}
}
