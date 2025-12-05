package orderbook

import "order-matching-engine/internal/common"

// PriceLevel represents all orders at a given price, in FIFO order.
type PriceLevel struct {
	Price  int64
	Orders []*common.Order // FIFO queue
}

func NewPriceLevel(price int64) *PriceLevel {
	return &PriceLevel{
		Price:  price,
		Orders: make([]*common.Order, 0, 8),
	}
}

func (pl *PriceLevel) Enqueue(o *common.Order) {
	pl.Orders = append(pl.Orders, o)
}

func (pl *PriceLevel) Dequeue() (*common.Order, bool) {
	if len(pl.Orders) == 0 {
		return nil, false
	}
	o := pl.Orders[0]
	pl.Orders = pl.Orders[1:]
	return o, true
}

func (pl *PriceLevel) IsEmpty() bool {
	return len(pl.Orders) == 0
}

// SideBook holds the orders for one side of the book (BUY or SELL).
// - BUY: prices sorted descending (highest first)
// - SELL: prices sorted ascending (lowest first)
type SideBook struct {
	IsBuy         bool
	Levels        map[int64]*PriceLevel // price -> level
	Prices        []int64               // sorted list of prices
	TotalQuantity int64                 // total remaining quantity across all levels
}

func NewSideBook(isBuy bool) *SideBook {
	return &SideBook{
		IsBuy:  isBuy,
		Levels: make(map[int64]*PriceLevel),
		Prices: make([]int64, 0, 32),
	}
}

// InsertPrice ensures there is a price level, and inserts the price into
// the sorted Prices slice if it is new.
func (sb *SideBook) InsertPrice(price int64) *PriceLevel {
	if level, ok := sb.Levels[price]; ok {
		return level
	}
	level := NewPriceLevel(price)
	sb.Levels[price] = level
	sb.insertPriceSorted(price)
	return level
}

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

func (sb *SideBook) BestPrice() (int64, bool) {
	if len(sb.Prices) == 0 {
		return 0, false
	}
	return sb.Prices[0], true
}

func (sb *SideBook) BestLevel() (*PriceLevel, bool) {
	price, ok := sb.BestPrice()
	if !ok {
		return nil, false
	}
	level, ok := sb.Levels[price]
	if !ok {
		return nil, false
	}
	return level, true
}

// RemovePrice removes a price level entirely if present.
func (sb *SideBook) RemovePrice(price int64) {
	delete(sb.Levels, price)
	for i, p := range sb.Prices {
		if p == price {
			sb.Prices = append(sb.Prices[:i], sb.Prices[i+1:]...)
			break
		}
	}
}

// AddOrder inserts an order into the appropriate price level and updates
// the aggregate TotalQuantity with the order's remaining quantity.
func (sb *SideBook) AddOrder(o *common.Order) {
	remaining := o.Quantity - o.FilledQty
	if remaining <= 0 {
		return
	}
	level := sb.InsertPrice(o.Price)
	level.Enqueue(o)
	sb.TotalQuantity += remaining
}

type OrderBook struct {
	Symbol string
	Bids   *SideBook
	Asks   *SideBook
}

func NewOrderBook(symbol string) *OrderBook {
	return &OrderBook{
		Symbol: symbol,
		Bids:   NewSideBook(true),
		Asks:   NewSideBook(false),
	}
}
