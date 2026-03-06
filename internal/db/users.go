package db

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/munster-bunkum/bunkum-api/internal/models"
)

var ErrNotFound = errors.New("not found")

func CreateUser(ctx context.Context, pool *pgxpool.Pool, username, email, passwordDigest string) (*models.User, error) {
	u := &models.User{}
	err := pool.QueryRow(ctx,
		`INSERT INTO users (username, email, password_digest)
		 VALUES ($1, $2, $3)
		 RETURNING id, username, email, created_at`,
		username, email, passwordDigest,
	).Scan(&u.ID, &u.Username, &u.Email, &u.CreatedAt)
	return u, err
}

func FindUserByUsername(ctx context.Context, pool *pgxpool.Pool, username string) (*models.User, error) {
	u := &models.User{}
	err := pool.QueryRow(ctx,
		`SELECT id, username, email, password_digest, created_at
		 FROM users WHERE username = $1`,
		username,
	).Scan(&u.ID, &u.Username, &u.Email, &u.PasswordDigest, &u.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return u, err
}

func FindUserByID(ctx context.Context, pool *pgxpool.Pool, id int64) (*models.User, error) {
	u := &models.User{}
	err := pool.QueryRow(ctx,
		`SELECT id, username, email, created_at FROM users WHERE id = $1`,
		id,
	).Scan(&u.ID, &u.Username, &u.Email, &u.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return u, err
}
