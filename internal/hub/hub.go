package hub

// hub.go is the global registry of active world rooms.
//
// The Hub is created once at startup and injected wherever it's needed
// (the WS handler). It maps world names to running Room instances.
//
// Thread safety: multiple WebSocket connections can arrive simultaneously,
// each in their own goroutine. The Hub uses a RWMutex so many goroutines
// can read (look up an existing room) concurrently, but only one can write
// (create a new room) at a time.
//
// Double-checked locking: we take a read lock first for the common case
// (room already exists), then upgrade to a write lock only if we need to
// create one. We re-check inside the write lock to handle the race where
// two goroutines both saw "no room" and raced to create one.
//
// Room cleanup: each room's Run() goroutine exits when the last player
// leaves. A wrapper goroutine started here removes the entry from the map
// after Run() returns, keeping the hub tidy.

import (
	"context"
	"errors"
	"log"
	"sync"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/munster-bunkum/bunkum-api/internal/db"
)

// Hub holds all currently active world rooms.
type Hub struct {
	rooms map[string]*Room
	mu    sync.RWMutex
	pool  *pgxpool.Pool
}

// New creates a Hub. Pass in the DB pool so rooms can load/save worlds.
func New(pool *pgxpool.Pool) *Hub {
	return &Hub{
		rooms: make(map[string]*Room),
		pool:  pool,
	}
}

// GetOrCreate returns the running Room for worldName, starting one if needed.
// Returns an error if the world doesn't exist in the database — callers must
// ensure the world was created first (via POST /worlds/enter).
func (h *Hub) GetOrCreate(ctx context.Context, worldName string) (*Room, error) {
	// Fast path: room is already running.
	h.mu.RLock()
	room, ok := h.rooms[worldName]
	h.mu.RUnlock()
	if ok {
		return room, nil
	}

	// Slow path: we may need to create the room.
	h.mu.Lock()
	defer h.mu.Unlock()

	// Re-check — another goroutine may have created it while we waited.
	if room, ok = h.rooms[worldName]; ok {
		return room, nil
	}

	// Load the world from the database.
	world, err := db.FindWorldByName(ctx, h.pool, worldName)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			return nil, db.ErrNotFound
		}
		return nil, err
	}

	// Deserialise the stored JSONB into the in-memory WorldState.
	state, err := worldStateFromDB(world)
	if err != nil {
		return nil, err
	}

	room = &Room{
		Name:       worldName,
		State:      state,
		clients:    make(map[*Client]bool),
		Register:   make(chan *Client),
		Unregister: make(chan *Client),
		Inbound:    make(chan ClientMsg, 256),
		pool:       h.pool,
	}
	h.rooms[worldName] = room

	// Start the room's actor goroutine. When it exits (world empty),
	// remove the room from the map so the next visitor starts fresh.
	go func() {
		room.Run()
		h.mu.Lock()
		delete(h.rooms, worldName)
		h.mu.Unlock()
		log.Printf("[hub] room %s removed", worldName)
	}()

	log.Printf("[hub] room %s started", worldName)
	return room, nil
}
