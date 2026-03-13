package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/munster-bunkum/bunkum-api/internal/db"
	"github.com/munster-bunkum/bunkum-api/internal/models"
)

var validWorldName = regexp.MustCompile(`^[A-Z_]{5,8}$`)

type enterWorldRequest struct {
	Name string `json:"name"`
}

// ListWorlds returns a random selection of worlds for the browser.
func ListWorlds(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		worlds, err := db.ListWorlds(r.Context(), pool)
		if err != nil {
			jsonError(w, "internal server error", http.StatusInternalServerError)
			return
		}
		if worlds == nil {
			worlds = []models.WorldSummary{}
		}
		jsonResponse(w, http.StatusOK, map[string]any{"worlds": worlds})
	}
}

// EnterWorld validates the world name, creates the world in the DB if it
// doesn't exist yet, and returns only the metadata (name, width, height).
//
// The actual world state (cells, players) is delivered via WebSocket after
// the client opens a connection to /ws/worlds/{name}. We deliberately keep
// cell data out of this HTTP response — the WebSocket is the authoritative
// channel and this endpoint is just a gatekeeper.
func EnterWorld(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req enterWorldRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			jsonError(w, "invalid request body", http.StatusBadRequest)
			return
		}

		name := strings.ToUpper(strings.TrimSpace(req.Name))
		if !validWorldName.MatchString(name) {
			jsonError(w, "world name must be 5–8 letters (A–Z or _)", http.StatusUnprocessableEntity)
			return
		}
		if blockedWorldNames[name] {
			jsonError(w, "that world name is not allowed", http.StatusUnprocessableEntity)
			return
		}

		world, err := db.FindWorldByName(r.Context(), pool, name)
		if errors.Is(err, db.ErrNotFound) {
			world, err = createWorld(r, pool, name)
			if err != nil {
				jsonError(w, "internal server error", http.StatusInternalServerError)
				return
			}
		} else if err != nil {
			jsonError(w, "internal server error", http.StatusInternalServerError)
			return
		}

		// Return only the lightweight summary — the WebSocket delivers state.
		jsonResponse(w, http.StatusOK, map[string]any{"world": models.WorldSummary{
			Name:      world.Name,
			Width:     world.Width,
			Height:    world.Height,
			CreatedAt: world.CreatedAt,
		}})
	}
}

// createWorld generates a fresh world, persists it, and returns it.
// Handles the concurrent-creation race: if another request beats us to the
// INSERT, we fall back to a SELECT.
func createWorld(r *http.Request, pool *pgxpool.Pool, name string) (models.World, error) {
	width, height := worldDimensions(name)
	data := generateWorldData(width, height)

	if err := db.InsertWorld(r.Context(), pool, name, width, height, data); err != nil {
		return models.World{}, err
	}

	// Re-fetch so we return the canonical DB row (handles the ON CONFLICT case).
	return db.FindWorldByName(r.Context(), pool, name)
}

// worldDimensions computes width and height from the world name.
// This is where name-based easter eggs live — add new modifiers here.
func worldDimensions(name string) (width, height int) {
	width, height = 101, 51
	if strings.EqualFold(name, "TALL") {
		height *= 2
	}
	if strings.EqualFold(name, "WIDE") {
		width *= 2
	}
	if strings.EqualFold(name, "BIG") {
		height *= 2
		width *= 2
	}
	if strings.EqualFold(name, "HUGE") {
		height *= 4
		width *= 4
	}
	if strings.EqualFold(name, "TINY") {
		height /= 2
		width /= 2
	}
	if strings.EqualFold(name, "MICRO") {
		height /= 4
		width /= 4
	}
	return
}

// generateWorldData builds the initial cell layout matching Godot's
// generate_empty_world: bedrock floor, dirt fill, WorldDoor at spawn.
func generateWorldData(width, height int) models.WorldData {
	surfaceRow := height - 30
	spawnX := width / 2

	cells := make([]models.CellData, 0, width*31)

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			var stack []models.BlockData

			if y == height-1 {
				stack = []models.BlockData{{ID: "bedrock"}}
			} else if y > surfaceRow {
				// Spawn support column gets bedrock on top of dirt so the
				// WorldDoor has a solid floor beneath it.
				if x == spawnX && y == surfaceRow+1 {
					stack = []models.BlockData{{ID: "dirt"}, {ID: "bedrock"}}
				} else {
					stack = []models.BlockData{{ID: "dirt"}}
				}
			}

			if len(stack) > 0 {
				cells = append(cells, models.CellData{X: x, Y: y, Stack: stack})
			}
		}
	}

	// WorldDoor at the surface — this is the spawn point and the exit portal.
	cells = append(cells, models.CellData{
		X:     spawnX,
		Y:     surfaceRow,
		Stack: []models.BlockData{{ID: "world_door"}},
	})

	return models.WorldData{
		Cells:  cells,
		Drops:  []models.DropData{},
		Config: map[string]interface{}{},
	}
}
