package hub

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/munster-bunkum/bunkum-api/internal/models"
)

// blockMaxHealth is the server's source of truth for block durability.
// -1 = indestructible.
var blockMaxHealth = map[string]int{
	"dirt":       5,
	"bedrock":    -1,
	"door":       10,
	"world_door": -1,
}

// blockDrops maps block ID → item ID it drops when broken.
// Blocks absent from this table drop nothing.
var blockDrops = map[string]string{
	"dirt": "dirt",
	"door": "door",
}

type CellKey struct{ X, Y int }

type Block struct {
	ID     string
	Health int
	Meta   map[string]interface{}
}

// Drop is an item lying on the ground waiting to be picked up.
// X/Y are pixel coordinates (centre of the cell it dropped from).
// Drops are ephemeral — they are not persisted to the DB. When the
// last player leaves a world, all drops disappear.
type Drop struct {
	ItemID string
	X, Y   float64
	Qty    int
}

type WorldState struct {
	Width      int
	Height     int
	SpawnCell  CellKey
	Cells      map[CellKey][]Block
	Drops      map[string]*Drop // drop ID → drop
	NextDropID int
	LastHitAt  map[CellKey]time.Time // for block-heal tracking
}

const maxDropStacksPerTile = 12

// trySpawnDrop attempts to add qty of itemID at pixel position (x,y).
// If a stack of the same item already exists on that tile it merges into it
// and returns ("", false). If a new stack is created it returns (dropID, true).
// If the tile already has maxDropStacksPerTile different stacks the drop is lost
// and ("", false) is returned.
func (ws *WorldState) trySpawnDrop(itemID string, x, y float64, qty int) (string, bool) {
	// Count stacks on this tile and look for a matching item to merge into.
	var mergeID string
	count := 0
	for id, d := range ws.Drops {
		if d.X == x && d.Y == y {
			count++
			if d.ItemID == itemID {
				mergeID = id
			}
		}
	}

	if mergeID != "" {
		ws.Drops[mergeID].Qty += qty
		return mergeID, false // merged, no new drop node needed
	}
	if count >= maxDropStacksPerTile {
		return "", false // tile full, drop lost
	}

	id := fmt.Sprintf("%d", ws.NextDropID)
	ws.NextDropID++
	ws.Drops[id] = &Drop{ItemID: itemID, X: x, Y: y, Qty: qty}
	return id, true
}

// rawWorldData mirrors WorldData with float64 coordinates to survive JSONB
// round-trips that may store integers as 0.0.
type rawWorldData struct {
	Cells []struct {
		X     float64            `json:"x"`
		Y     float64            `json:"y"`
		Stack []models.BlockData `json:"stack"`
	} `json:"cells"`
	Drops []models.DropData `json:"drops"`
}

func worldStateFromDB(world models.World) (*WorldState, error) {
	var data rawWorldData
	if err := json.Unmarshal(world.Data, &data); err != nil {
		return nil, err
	}

	ws := &WorldState{
		Width:     world.Width,
		Height:    world.Height,
		Cells:     make(map[CellKey][]Block),
		Drops:     make(map[string]*Drop),
		LastHitAt: make(map[CellKey]time.Time),
	}

	for _, cd := range data.Cells {
		key := CellKey{int(cd.X), int(cd.Y)}
		stack := make([]Block, 0, len(cd.Stack))
		for _, bd := range cd.Stack {
			health := blockMaxHealth[bd.ID]
			if bd.Health != nil {
				health = *bd.Health
			}
			stack = append(stack, Block{ID: bd.ID, Health: health, Meta: bd.Meta})
			if bd.ID == "world_door" {
				ws.SpawnCell = key
			}
		}
		ws.Cells[key] = stack
	}

	// Load persisted drops. IDs are reassigned sequentially — clients always
	// receive the authoritative list via world_init so stale IDs don't matter.
	for _, dd := range data.Drops {
		id := fmt.Sprintf("%d", ws.NextDropID)
		ws.NextDropID++
		ws.Drops[id] = &Drop{ItemID: dd.ID, X: dd.X, Y: dd.Y, Qty: dd.Qty}
	}

	return ws, nil
}

func (ws *WorldState) toWorldData() models.WorldData {
	cells := make([]models.CellData, 0, len(ws.Cells))
	for key, stack := range ws.Cells {
		entries := make([]models.BlockData, 0, len(stack))
		for _, b := range stack {
			entry := models.BlockData{ID: b.ID, Meta: b.Meta}
			maxHP := blockMaxHealth[b.ID]
			if maxHP > 0 && b.Health != maxHP {
				h := b.Health
				entry.Health = &h
			}
			entries = append(entries, entry)
		}
		cells = append(cells, models.CellData{X: key.X, Y: key.Y, Stack: entries})
	}
	drops := make([]models.DropData, 0, len(ws.Drops))
	for _, d := range ws.Drops {
		drops = append(drops, models.DropData{ID: d.ItemID, X: d.X, Y: d.Y, Qty: d.Qty})
	}

	return models.WorldData{
		Cells:  cells,
		Drops:  drops,
		Config: map[string]interface{}{},
	}
}

func toCellMsg(key CellKey, stack []Block) CellMsg {
	msgs := make([]BlockMsg, len(stack))
	for i, b := range stack {
		msgs[i] = BlockMsg{ID: b.ID, Meta: b.Meta}
	}
	return CellMsg{X: key.X, Y: key.Y, Stack: msgs}
}
