package handlers

// ws.go handles the HTTP → WebSocket upgrade for a world connection.
//
// The upgrade is a standard HTTP request that the client asks to "upgrade"
// to a persistent WebSocket connection. gorilla/websocket's Upgrader handles
// the handshake; after that we hand off to the room's actor loop.
//
// Auth: we rely on auth.Middleware wrapping this handler, which reads the
// JWT from the cookie, Authorization header, or ?token= query param.
// The ?token= form is what Godot's WebSocketPeer uses since it can't set
// arbitrary headers during the WebSocket handshake.
//
// The handler blocks in client.ReadPump() for the lifetime of the connection.
// WritePump runs in its own goroutine started just before.

import (
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/gorilla/websocket"
	"github.com/munster-bunkum/bunkum-api/internal/auth"
	"github.com/munster-bunkum/bunkum-api/internal/db"
	"github.com/munster-bunkum/bunkum-api/internal/hub"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// CheckOrigin controls which origins can open a WebSocket connection.
	// During development we allow everything; tighten this in production.
	CheckOrigin: func(r *http.Request) bool { return true },
}

// WorldSocket upgrades the connection and registers the client with the room.
func WorldSocket(h *hub.Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := auth.ClaimsFromContext(r.Context())
		worldName := strings.ToUpper(r.PathValue("name"))

		// Get the running room, or start one by loading the world from DB.
		room, err := h.GetOrCreate(r.Context(), worldName)
		if err != nil {
			if errors.Is(err, db.ErrNotFound) {
				http.Error(w, `{"error":"world not found"}`, http.StatusNotFound)
				return
			}
			log.Printf("[ws] GetOrCreate %s: %v", worldName, err)
			http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
			return
		}

		// Upgrade HTTP → WebSocket.
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			// Upgrade writes its own error response; just log and return.
			log.Printf("[ws] upgrade failed for %s: %v", claims.Username, err)
			return
		}

		client := &hub.Client{
			Room:      room,
			Conn:      conn,
			Send:      make(chan []byte, 256),
			Username:  claims.Username,
			Inventory: make([]hub.InventorySlot, 24), // 4 hotbar + 20 inventory
		}

		// Register the client with the room. This triggers world_init.
		room.Register <- client

		// WritePump gets its own goroutine; ReadPump blocks this handler
		// goroutine until the connection closes, which is the correct pattern —
		// the HTTP server's goroutine is "parked" here for the session lifetime.
		go client.WritePump()
		client.ReadPump()
	}
}
