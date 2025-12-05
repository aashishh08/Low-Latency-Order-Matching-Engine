package common

// Side represents the direction of an order (buy or sell)
type Side string

const (
	SideBuy  Side = "BUY"
	SideSell Side = "SELL"
)

// OrderType represents the type of order
type OrderType string

const (
	OrderTypeLimit  OrderType = "LIMIT"
	OrderTypeMarket OrderType = "MARKET"
)

// OrderStatus represents the current status of an order
type OrderStatus string

const (
	OrderStatusAccepted  OrderStatus = "ACCEPTED"
	OrderStatusPartial   OrderStatus = "PARTIAL_FILL"
	OrderStatusFilled    OrderStatus = "FILLED"
	OrderStatusCancelled OrderStatus = "CANCELLED"
)

// Order represents a trading order in the system
type Order struct {
	ID        string      `json:"order_id"`
	Symbol    string      `json:"symbol"`
	Side      Side        `json:"side"`
	Type      OrderType   `json:"type"`
	Price     int64       `json:"price"`           // cents; required only for LIMIT
	Quantity  int64       `json:"quantity"`        // total quantity
	FilledQty int64       `json:"filled_quantity"` // quantity filled so far
	Status    OrderStatus `json:"status"`
	Timestamp int64       `json:"timestamp"` // unix ms
}

// Trade represents an executed trade between two orders
type Trade struct {
	TradeID   string `json:"trade_id"`
	BuyOrder  string `json:"buy_order"`
	SellOrder string `json:"sell_order"`
	Price     int64  `json:"price"`
	Quantity  int64  `json:"quantity"`
	Timestamp int64  `json:"timestamp"`
}
