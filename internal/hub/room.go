package hub

import (
	"context"
	"encoding/json"
	"log"
	"math"
	"math/rand"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/munster-bunkum/bunkum-api/internal/db"
)

const (
	punchDamage      = 1
	autosaveTick     = 60 * time.Second
	healTick         = 1 * time.Second / 2 // how often to check for block heals
	healDelay        = 3 * time.Second     // time after last hit before health restores
	pickupRadiusPx   = 14.0               // max pixel distance to pick up a drop
)

// ClientMsg pairs an inbound action with the client who sent it.
type ClientMsg struct {
	Client *Client
	Msg    InboundMsg
}

type Room struct {
	Name       string
	State      *WorldState
	clients    map[*Client]bool
	Register   chan *Client
	Unregister chan *Client
	Inbound    chan ClientMsg
	pool       *pgxpool.Pool
}

func (r *Room) Run() {
	autosaver := time.NewTicker(autosaveTick)
	healer    := time.NewTicker(healTick)
	defer autosaver.Stop()
	defer healer.Stop()

	for {
		select {
		case client := <-r.Register:
			r.clients[client] = true
			log.Printf("[room:%s] %s joined (%d online)", r.Name, client.Username, len(r.clients))
			r.sendInitTo(client)
			r.broadcastExcept(client, PlayerJoinMsg{
				Type:     EventPlayerJoin,
				Username: client.Username,
				X:        client.PosX,
				Y:        client.PosY,
			})

		case client := <-r.Unregister:
			if _, ok := r.clients[client]; !ok {
				continue
			}
			delete(r.clients, client)
			close(client.Send)
			log.Printf("[room:%s] %s left (%d online)", r.Name, client.Username, len(r.clients))
			r.broadcast(PlayerLeaveMsg{Type: EventPlayerLeave, Username: client.Username})
			if len(r.clients) == 0 {
				r.save()
				log.Printf("[room:%s] empty, shutting down", r.Name)
				return
			}

		case cm := <-r.Inbound:
			r.handle(cm)

		case <-healer.C:
			r.healBlocks()

		case <-autosaver.C:
			if len(r.clients) > 0 {
				r.save()
			}
		}
	}
}

func (r *Room) handle(cm ClientMsg) {
	switch cm.Msg.Type {
	case ActionBlockBreak:
		r.handleBreak(cm.Client, cm.Msg.X, cm.Msg.Y)
	case ActionBlockPlace:
		r.handlePlace(cm.Client, cm.Msg.X, cm.Msg.Y, cm.Msg.BlockID)
	case ActionPlayerMove:
		r.handleMove(cm.Client, cm.Msg.PosX, cm.Msg.PosY)
	case ActionPickup:
		r.handlePickup(cm.Client, cm.Msg.DropID)
	case ActionDropItem:
		r.handleDropItem(cm.Client, cm.Msg.SlotIndex, cm.Msg.Qty)
	case ActionDestroyItem:
		r.handleDestroyItem(cm.Client, cm.Msg.SlotIndex, cm.Msg.Qty)
	}
}

// handleBreak processes a block_break action.
// On a clean break it also spawns a drop according to the loot table.
func (r *Room) handleBreak(client *Client, x, y int) {
	key := CellKey{x, y}
	stack, exists := r.State.Cells[key]
	if !exists || len(stack) == 0 {
		return
	}

	top := stack[len(stack)-1]
	if top.Health < 0 {
		return // indestructible
	}

	top.Health -= punchDamage

	if top.Health <= 0 {
		// Remove top block from stack.
		stack = stack[:len(stack)-1]
		if len(stack) == 0 {
			delete(r.State.Cells, key)
		} else {
			r.State.Cells[key] = stack
		}
		delete(r.State.LastHitAt, key)

		r.broadcast(BlockUpdateMsg{
			Type:  EventBlockUpdate,
			X:     x,
			Y:     y,
			Stack: stackToMsgs(stack),
		})

		// Spawn a drop if the loot table has an entry for this block.
		if itemID, ok := blockDrops[top.ID]; ok {
			// Random offset within the cell so multiple drops on the same tile
			// are visually distinct. Range: ±10 px on X, ±8 px on Y.
			dropX := float64(x*32) + 16 + float64(rand.Intn(21)-10)
			dropY := float64(y*32) + 16 + float64(rand.Intn(17)-8)
			dropID, isNew := r.State.trySpawnDrop(itemID, dropX, dropY, 1)
			if dropID != "" {
				if isNew {
					// Brand new stack — tell all clients to render it.
					r.broadcast(DropMsg{
						Type:   EventDropSpawn,
						DropID: dropID,
						ItemID: itemID,
						X:      dropX,
						Y:      dropY,
						Qty:    1,
					})
				} else {
					// Merged into existing stack — update the qty on that node.
					d := r.State.Drops[dropID]
					r.broadcast(DropMsg{
						Type:   EventDropSpawn, // reuse drop_spawn; client updates qty
						DropID: dropID,
						ItemID: d.ItemID,
						X:      d.X,
						Y:      d.Y,
						Qty:    d.Qty,
					})
				}
			}
		}
	} else {
		stack[len(stack)-1] = top
		r.State.Cells[key] = stack
		r.State.LastHitAt[key] = time.Now()

		maxHP := float64(blockMaxHealth[top.ID])
		r.broadcast(BlockDamageMsg{
			Type:        EventBlockDamage,
			X:           x,
			Y:           y,
			HealthRatio: float64(top.Health) / maxHP,
		})
	}
}

// handlePickup removes a drop from the world and adds it to the client's inventory.
func (r *Room) handlePickup(client *Client, dropID string) {
	drop, ok := r.State.Drops[dropID]
	if !ok {
		return // already picked up or invalid
	}

	dx := client.PosX - drop.X
	dy := client.PosY - drop.Y
	if math.Sqrt(dx*dx+dy*dy) > pickupRadiusPx {
		client.send(ErrMsg{Type: EventError, Message: "too far away"})
		return
	}

	if !client.addItem(drop.ItemID, drop.Qty) {
		client.send(ErrMsg{Type: EventError, Message: "inventory full"})
		return
	}

	delete(r.State.Drops, dropID)
	r.broadcast(DropDespawnMsg{Type: EventDropDespawn, DropID: dropID})
	client.send(client.inventoryMsg())
}

// handlePlace validates the client has the item, consumes it, and places the block.
func (r *Room) handlePlace(client *Client, x, y int, blockID string) {
	if _, ok := blockMaxHealth[blockID]; !ok {
		client.send(ErrMsg{Type: EventError, Message: "unknown block: " + blockID})
		return
	}

	key := CellKey{x, y}
	if len(r.State.Cells[key]) > 0 {
		client.send(ErrMsg{Type: EventError, Message: "cell already occupied"})
		return
	}

	if !client.consumeItem(blockID, 1) {
		client.send(ErrMsg{Type: EventError, Message: "you don't have that block"})
		return
	}

	health := blockMaxHealth[blockID]
	r.State.Cells[key] = append(r.State.Cells[key], Block{ID: blockID, Health: health})

	r.broadcast(BlockUpdateMsg{
		Type:  EventBlockUpdate,
		X:     x,
		Y:     y,
		Stack: stackToMsgs(r.State.Cells[key]),
	})
	client.send(client.inventoryMsg())
}

// handleDropItem removes qty from an inventory slot and spawns it as a world drop near the player.
func (r *Room) handleDropItem(client *Client, slotIndex, qty int) {
	if slotIndex < 0 || slotIndex >= len(client.Inventory) || qty <= 0 {
		return
	}
	slot := &client.Inventory[slotIndex]
	if slot.ItemID == "" || slot.Qty < qty {
		client.send(ErrMsg{Type: EventError, Message: "not enough items"})
		return
	}

	itemID := slot.ItemID

	// Drop near player's current position with a small random spread.
	dropX := client.PosX + float64(rand.Intn(33)-16)
	dropY := client.PosY + float64(rand.Intn(33)-16)
	dropID, isNew := r.State.trySpawnDrop(itemID, dropX, dropY, qty)
	if dropID == "" {
		// Tile is full — item stays in inventory.
		client.send(ErrMsg{Type: EventError, Message: "Can't drop that here!"})
		return
	}

	slot.Qty -= qty
	if slot.Qty == 0 {
		*slot = InventorySlot{}
	}

	if dropID != "" {
		d := r.State.Drops[dropID]
		r.broadcast(DropMsg{
			Type:   EventDropSpawn,
			DropID: dropID,
			ItemID: d.ItemID,
			X:      d.X,
			Y:      d.Y,
			Qty:    d.Qty,
		})
		_ = isNew
	}
	client.send(client.inventoryMsg())
}

// handleDestroyItem permanently removes qty from an inventory slot with no world drop.
func (r *Room) handleDestroyItem(client *Client, slotIndex, qty int) {
	if slotIndex < 0 || slotIndex >= len(client.Inventory) || qty <= 0 {
		return
	}
	slot := &client.Inventory[slotIndex]
	if slot.ItemID == "" || slot.Qty < qty {
		client.send(ErrMsg{Type: EventError, Message: "not enough items"})
		return
	}
	slot.Qty -= qty
	if slot.Qty == 0 {
		*slot = InventorySlot{}
	}
	client.send(client.inventoryMsg())
}

func (r *Room) handleMove(client *Client, posX, posY float64) {
	client.PosX = posX
	client.PosY = posY
	r.broadcastExcept(client, PlayerMoveMsg{
		Type:     EventPlayerMove,
		Username: client.Username,
		X:        posX,
		Y:        posY,
	})
}

// healBlocks restores full health to any block that hasn't been hit in healDelay.
func (r *Room) healBlocks() {
	now := time.Now()
	for key, hitAt := range r.State.LastHitAt {
		if now.Sub(hitAt) < healDelay {
			continue
		}
		stack, exists := r.State.Cells[key]
		if !exists || len(stack) == 0 {
			delete(r.State.LastHitAt, key)
			continue
		}
		top := &stack[len(stack)-1]
		top.Health = blockMaxHealth[top.ID]
		r.State.Cells[key] = stack
		delete(r.State.LastHitAt, key)

		// Tell clients the block is back to full health (same as a block_update).
		r.broadcast(BlockUpdateMsg{
			Type:  EventBlockUpdate,
			X:     key.X,
			Y:     key.Y,
			Stack: stackToMsgs(stack),
		})
	}
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func (r *Room) sendInitTo(client *Client) {
	cells := make([]CellMsg, 0, len(r.State.Cells))
	for key, stack := range r.State.Cells {
		cells = append(cells, toCellMsg(key, stack))
	}

	drops := make([]DropMsg, 0, len(r.State.Drops))
	for id, d := range r.State.Drops {
		drops = append(drops, DropMsg{
			Type:   EventDropSpawn,
			DropID: id,
			ItemID: d.ItemID,
			X:      d.X,
			Y:      d.Y,
			Qty:    d.Qty,
		})
	}

	players := make([]PlayerMsg, 0, len(r.clients))
	for c := range r.clients {
		if c == client {
			continue
		}
		players = append(players, PlayerMsg{Username: c.Username, X: c.PosX, Y: c.PosY})
	}

	client.send(WorldInitMsg{
		Type:    EventWorldInit,
		Width:   r.State.Width,
		Height:  r.State.Height,
		SpawnX:  r.State.SpawnCell.X,
		SpawnY:  r.State.SpawnCell.Y,
		Cells:   cells,
		Drops:   drops,
		Players: players,
	})
}

func (r *Room) broadcast(v any) {
	b, err := json.Marshal(v)
	if err != nil {
		log.Printf("[room:%s] broadcast marshal error: %v", r.Name, err)
		return
	}
	for client := range r.clients {
		select {
		case client.Send <- b:
		default:
			delete(r.clients, client)
			close(client.Send)
		}
	}
}

func (r *Room) broadcastExcept(except *Client, v any) {
	b, err := json.Marshal(v)
	if err != nil {
		return
	}
	for client := range r.clients {
		if client == except {
			continue
		}
		select {
		case client.Send <- b:
		default:
			delete(r.clients, client)
			close(client.Send)
		}
	}
}

func (r *Room) save() {
	data := r.State.toWorldData()
	b, err := json.Marshal(data)
	if err != nil {
		log.Printf("[room:%s] save marshal error: %v", r.Name, err)
		return
	}
	if err := db.SaveWorldData(context.Background(), r.pool, r.Name, b); err != nil {
		log.Printf("[room:%s] save error: %v", r.Name, err)
	} else {
		log.Printf("[room:%s] saved (%d cells)", r.Name, len(r.State.Cells))
	}
}

func stackToMsgs(stack []Block) []BlockMsg {
	msgs := make([]BlockMsg, len(stack))
	for i, b := range stack {
		msgs[i] = BlockMsg{ID: b.ID, Meta: b.Meta}
	}
	return msgs
}
