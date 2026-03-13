package hub

// ── Client → Server action types ─────────────────────────────────────────────

const (
	ActionBlockBreak  = "block_break"
	ActionBlockPlace  = "block_place"
	ActionPlayerMove  = "player_move"
	ActionPickup      = "pickup"       // pick up a drop by ID
	ActionDropItem    = "drop_item"    // drop item from inventory to world
	ActionDestroyItem = "destroy_item" // permanently remove item from inventory
)

// InboundMsg is the generic envelope for every client action.
type InboundMsg struct {
	Type      string  `json:"type"`
	X         int     `json:"x"`
	Y         int     `json:"y"`
	BlockID   string  `json:"block_id,omitempty"`
	PosX      float64 `json:"pos_x,omitempty"`
	PosY      float64 `json:"pos_y,omitempty"`
	DropID    string  `json:"drop_id,omitempty"`
	SlotIndex int     `json:"slot_index,omitempty"`
	Qty       int     `json:"qty,omitempty"`
}

// ── Server → Client event types ──────────────────────────────────────────────

const (
	EventWorldInit      = "world_init"
	EventBlockUpdate    = "block_update"
	EventBlockDamage    = "block_damage"
	EventDropSpawn      = "drop_spawn"
	EventDropDespawn    = "drop_despawn"
	EventInventoryUpdate = "inventory_update"
	EventPlayerJoin     = "player_join"
	EventPlayerMove     = "player_move"
	EventPlayerLeave    = "player_leave"
	EventError          = "error"
)

// WorldInitMsg is sent once to each client when they connect.
type WorldInitMsg struct {
	Type    string      `json:"type"`
	Width   int         `json:"width"`
	Height  int         `json:"height"`
	SpawnX  int         `json:"spawn_x"`
	SpawnY  int         `json:"spawn_y"`
	Cells   []CellMsg   `json:"cells"`
	Drops   []DropMsg   `json:"drops"`
	Players []PlayerMsg `json:"players"`
}

type CellMsg struct {
	X     int        `json:"x"`
	Y     int        `json:"y"`
	Stack []BlockMsg `json:"stack"`
}

// BlockMsg is a block as sent to clients — no health value, only ratio on damage.
type BlockMsg struct {
	ID   string                 `json:"id"`
	Meta map[string]interface{} `json:"meta,omitempty"`
}

type BlockUpdateMsg struct {
	Type  string     `json:"type"`
	X     int        `json:"x"`
	Y     int        `json:"y"`
	Stack []BlockMsg `json:"stack"`
}

// BlockDamageMsg carries a 0–1 ratio for the break-animation overlay.
type BlockDamageMsg struct {
	Type        string  `json:"type"`
	X           int     `json:"x"`
	Y           int     `json:"y"`
	HealthRatio float64 `json:"health_ratio"`
}

// DropMsg describes a drop lying in the world.
type DropMsg struct {
	Type   string  `json:"type"`
	DropID string  `json:"drop_id"`
	ItemID string  `json:"item_id"`
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Qty    int     `json:"qty"`
}

type DropDespawnMsg struct {
	Type   string `json:"type"`
	DropID string `json:"drop_id"`
}

// InventorySlot is one slot in a player's inventory.
type InventorySlot struct {
	ItemID string `json:"item_id"`
	Qty    int    `json:"qty"`
}

// InventoryUpdateMsg replaces the client's full inventory view.
type InventoryUpdateMsg struct {
	Type  string          `json:"type"`
	Slots []InventorySlot `json:"slots"`
}

type PlayerMsg struct {
	Username string  `json:"username"`
	X        float64 `json:"x"`
	Y        float64 `json:"y"`
}

type PlayerJoinMsg struct {
	Type     string  `json:"type"`
	Username string  `json:"username"`
	X        float64 `json:"x"`
	Y        float64 `json:"y"`
}

type PlayerMoveMsg struct {
	Type     string  `json:"type"`
	Username string  `json:"username"`
	X        float64 `json:"x"`
	Y        float64 `json:"y"`
}

type PlayerLeaveMsg struct {
	Type     string `json:"type"`
	Username string `json:"username"`
}

type ErrMsg struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}
