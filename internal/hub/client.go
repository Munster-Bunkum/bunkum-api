package hub

// client.go manages a single WebSocket connection.
//
// Each client runs two goroutines:
//
//   ReadPump  — blocks on conn.ReadMessage(), parses JSON, forwards to Room.
//   WritePump — blocks on the send channel, writes outgoing JSON to the socket.
//
// Why two goroutines instead of one?
// The WebSocket spec allows simultaneous reads and writes on a connection,
// but gorilla/websocket is not safe for concurrent writes. By funneling all
// writes through a single goroutine reading from a channel, we guarantee
// that only one write happens at a time without needing a mutex.
//
// The send channel acts as a small buffer. If the client falls too far behind
// (channel full), WritePump closes the connection rather than letting memory
// grow unboundedly.
//
// Ping/pong keepalive: gorilla/websocket handles pong frames automatically
// when we set a PongHandler. We send Ping frames from WritePump on a timer.
// If the client doesn't pong within pongWait, the read deadline fires and
// ReadPump exits, triggering the unregister flow.

import (
	"encoding/json"
	"log"
	"time"

	"github.com/gorilla/websocket"
)

const (
	writeWait      = 10 * time.Second           // max time to write a message
	pongWait       = 60 * time.Second           // max time between pongs
	pingPeriod     = (pongWait * 9) / 10        // how often to send pings
	maxMessageSize = 4096                        // max bytes per incoming message
	sendBufSize    = 256                         // send channel buffer depth
)

// Client represents one connected player.
type Client struct {
	Room      *Room
	Conn      *websocket.Conn
	Send      chan []byte // outgoing messages; WritePump drains this
	Username  string
	PosX, PosY float64
	Inventory []InventorySlot // 4-slot hotbar + 20 inventory; index = slot number
}

// addItem adds qty of itemID to the first available or matching slot.
// Slot 0 is reserved for the client-side fist/wrench tool and is never used.
// Returns true if all items were placed (inventory not full).
func (c *Client) addItem(itemID string, qty int) bool {
	// First pass: stack onto existing slot (skip slot 0).
	for i := 1; i < len(c.Inventory); i++ {
		if c.Inventory[i].ItemID == itemID {
			c.Inventory[i].Qty += qty
			return true
		}
	}
	// Second pass: find empty slot (skip slot 0).
	for i := 1; i < len(c.Inventory); i++ {
		if c.Inventory[i].ItemID == "" {
			c.Inventory[i] = InventorySlot{ItemID: itemID, Qty: qty}
			return true
		}
	}
	return false // inventory full
}

// consumeItem removes qty from the first slot holding itemID.
// Returns true if the full amount was available.
func (c *Client) consumeItem(itemID string, qty int) bool {
	for i := 1; i < len(c.Inventory); i++ {
		if c.Inventory[i].ItemID == itemID && c.Inventory[i].Qty >= qty {
			c.Inventory[i].Qty -= qty
			if c.Inventory[i].Qty == 0 {
				c.Inventory[i] = InventorySlot{}
			}
			return true
		}
	}
	return false
}

// inventoryMsg builds the InventoryUpdateMsg to send to this client.
func (c *Client) inventoryMsg() InventoryUpdateMsg {
	slots := make([]InventorySlot, len(c.Inventory))
	copy(slots, c.Inventory)
	return InventoryUpdateMsg{Type: EventInventoryUpdate, Slots: slots}
}

// ReadPump runs until the WebSocket closes. It forwards valid JSON messages
// to the room's Inbound channel for processing.
func (c *Client) ReadPump() {
	// When ReadPump exits the client is effectively dead. Unregistering
	// removes it from the room and closes c.Send so WritePump also exits.
	defer func() {
		c.Room.Unregister <- c
		c.Conn.Close()
	}()

	c.Conn.SetReadLimit(maxMessageSize)
	c.Conn.SetReadDeadline(time.Now().Add(pongWait))
	c.Conn.SetPongHandler(func(string) error {
		// Each pong resets the deadline, keeping the connection alive.
		c.Conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, raw, err := c.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err,
				websocket.CloseGoingAway,
				websocket.CloseAbnormalClosure,
			) {
				log.Printf("[ws] %s: read error: %v", c.Username, err)
			}
			return
		}

		var msg InboundMsg
		if err := json.Unmarshal(raw, &msg); err != nil {
			// Malformed JSON — ignore, don't disconnect
			continue
		}

		c.Room.Inbound <- ClientMsg{Client: c, Msg: msg}
	}
}

// WritePump drains c.Send and writes messages to the WebSocket. It also
// sends periodic pings to detect dead connections.
func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()

	for {
		select {
		case msg, ok := <-c.Send:
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// Room closed the channel — send a clean close frame.
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.Conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}

		case <-ticker.C:
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// send is a helper that marshals v to JSON and enqueues it on c.Send.
// If the buffer is full the client is too slow; we close them rather than
// blocking the room goroutine.
func (c *Client) send(v any) {
	b, err := json.Marshal(v)
	if err != nil {
		log.Printf("[ws] marshal error for %s: %v", c.Username, err)
		return
	}
	select {
	case c.Send <- b:
	default:
		log.Printf("[ws] %s: send buffer full, closing", c.Username)
		close(c.Send)
		c.Conn.Close()
	}
}
