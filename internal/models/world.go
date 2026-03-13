package models

import (
	"encoding/json"
	"time"
)

// World is the full DB row. Data holds the JSONB payload as raw JSON so it
// round-trips to the client without re-encoding (preserves all client fields).
// Ownership is handled in-world via lock blocks (meta: {owner: "username"}),
// not at the database level.
type World struct {
	Name      string          `json:"name"`
	Width     int             `json:"width"`
	Height    int             `json:"height"`
	Data      json.RawMessage `json:"data"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}

// WorldSummary is returned in list responses — no heavy data payload.
type WorldSummary struct {
	Name      string    `json:"name"`
	Width     int       `json:"width"`
	Height    int       `json:"height"`
	CreatedAt time.Time `json:"created_at"`
}

// WorldData is used only when generating a fresh world server-side.
// When clients save a world, the raw JSON is stored as-is without decoding.
//
// Extensibility: any new top-level key (npcs, events, etc.) the client adds
// passes through unchanged because the whole blob is stored as JSONB.
type WorldData struct {
	Cells  []CellData             `json:"cells"`
	Drops  []DropData             `json:"drops"`
	Config map[string]interface{} `json:"config"`
}

// CellData is one tile position. Stack holds blocks bottom-to-top.
type CellData struct {
	X     int         `json:"x"`
	Y     int         `json:"y"`
	Stack []BlockData `json:"stack"`
}

// BlockData is one block in a cell stack as stored in the database.
// Health is a pointer so nil means "use the block's max health" —
// freshly generated blocks don't need to store it explicitly.
// Meta holds block-specific state: gate open/close, planted_at for
// trees, lock owner, sign text, etc.
type BlockData struct {
	ID     string                 `json:"id"`
	Health *int                   `json:"health,omitempty"`
	Meta   map[string]interface{} `json:"meta,omitempty"`
}

// DropData is an item lying on the ground at world coordinates.
type DropData struct {
	ID  string  `json:"id"`
	X   float64 `json:"x"`
	Y   float64 `json:"y"`
	Qty int     `json:"qty"`
}
