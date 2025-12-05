package api

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"

	"order-matching-engine/internal/common"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type WSMessage struct {
	Type    string `json:"type"` // "trade" | "orderbook"
	Symbol  string `json:"symbol"`
	Payload any    `json:"payload"`
}

type WSHub struct {
	mu          sync.RWMutex
	subscribers map[string]map[*websocket.Conn]bool // symbol -> connections
}

func NewWSHub() *WSHub {
	return &WSHub{
		subscribers: make(map[string]map[*websocket.Conn]bool),
	}
}

func (h *WSHub) Subscribe(symbol string, conn *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.subscribers[symbol] == nil {
		h.subscribers[symbol] = make(map[*websocket.Conn]bool)
	}
	h.subscribers[symbol][conn] = true
}

func (h *WSHub) Unsubscribe(symbol string, conn *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if subs, ok := h.subscribers[symbol]; ok {
		delete(subs, conn)
		if len(subs) == 0 {
			delete(h.subscribers, symbol)
		}
	}
}

func (h *WSHub) BroadcastTrade(symbol string, trade *common.Trade) {
	h.broadcast(symbol, WSMessage{
		Type:    "trade",
		Symbol:  symbol,
		Payload: trade,
	})
}

func (h *WSHub) BroadcastOrderBook(symbol string, bids, asks []map[string]any) {
	h.broadcast(symbol, WSMessage{
		Type:   "orderbook",
		Symbol: symbol,
		Payload: map[string]any{
			"bids": bids,
			"asks": asks,
		},
	})
}

func (h *WSHub) broadcast(symbol string, msg WSMessage) {
	h.mu.RLock()
	subs := h.subscribers[symbol]
	h.mu.RUnlock()

	if len(subs) == 0 {
		return
	}

	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("Failed to marshal WS message: %v", err)
		return
	}

	for conn := range subs {
		go func(c *websocket.Conn) {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("WebSocket broadcast panic: %v", r)
					h.Unsubscribe(symbol, c)
					c.Close()
				}
			}()

			if err := c.WriteMessage(websocket.TextMessage, data); err != nil {
				h.Unsubscribe(symbol, c)
				c.Close()
			}
		}(conn)
	}
}

func (a *API) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	symbol := chi.URLParam(r, "symbol")
	if symbol == "" {
		http.Error(w, "symbol required", http.StatusBadRequest)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}
	defer conn.Close()

	a.WSHub.Subscribe(symbol, conn)
	defer a.WSHub.Unsubscribe(symbol, conn)

	// Keep connection alive, read messages (ping/pong)
	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			break
		}
	}
}
