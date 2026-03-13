package db

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/munster-bunkum/bunkum-api/internal/models"
)

func FindWorldByName(ctx context.Context, pool *pgxpool.Pool, name string) (models.World, error) {
	var w models.World
	var dataBytes []byte
	err := pool.QueryRow(ctx,
		`SELECT name, width, height, data, created_at, updated_at
		 FROM worlds WHERE name = $1`,
		name,
	).Scan(&w.Name, &w.Width, &w.Height, &dataBytes, &w.CreatedAt, &w.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return w, ErrNotFound
	}
	if err != nil {
		return w, err
	}
	w.Data = json.RawMessage(dataBytes)
	return w, nil
}

// InsertWorld creates a new world. Returns ErrConflict if the name is taken
// (concurrent creation race); callers should fall back to FindWorldByName.
func InsertWorld(ctx context.Context, pool *pgxpool.Pool, name string, width, height int, data models.WorldData) error {
	dataBytes, err := json.Marshal(data)
	if err != nil {
		return err
	}
	_, err = pool.Exec(ctx,
		`INSERT INTO worlds (name, width, height, data)
		 VALUES ($1, $2, $3, $4)
		 ON CONFLICT (name) DO NOTHING`,
		name, width, height, dataBytes,
	)
	return err
}

// SaveWorldData replaces a world's data payload with raw JSON from the client.
// Health values, meta fields, drops — everything passes through untouched.
func SaveWorldData(ctx context.Context, pool *pgxpool.Pool, name string, data json.RawMessage) error {
	result, err := pool.Exec(ctx,
		`UPDATE worlds SET data = $2, updated_at = NOW() WHERE name = $1`,
		name, []byte(data),
	)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func ListWorlds(ctx context.Context, pool *pgxpool.Pool) ([]models.WorldSummary, error) {
	rows, err := pool.Query(ctx,
		`SELECT name, width, height, created_at FROM worlds ORDER BY RANDOM()`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var worlds []models.WorldSummary
	for rows.Next() {
		var w models.WorldSummary
		if err := rows.Scan(&w.Name, &w.Width, &w.Height, &w.CreatedAt); err != nil {
			return nil, err
		}
		worlds = append(worlds, w)
	}
	return worlds, rows.Err()
}
