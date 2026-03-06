package db

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/munster-bunkum/bunkum-api/internal/models"
)

// CreatePrivateMessage stores a message and returns it with sender/recipient usernames populated.
func CreatePrivateMessage(ctx context.Context, pool *pgxpool.Pool, senderID, recipientID int64, body string) (*models.PrivateMessage, error) {
	m := &models.PrivateMessage{}
	err := pool.QueryRow(ctx,
		`INSERT INTO private_messages (sender_id, recipient_id, body)
		 VALUES ($1, $2, $3)
		 RETURNING id, sender_id, recipient_id, body, read_at, created_at`,
		senderID, recipientID, body,
	).Scan(&m.ID, &m.SenderID, &m.RecipientID, &m.Body, &m.ReadAt, &m.CreatedAt)
	return m, err
}

// GetConversation returns messages between two users, newest last.
func GetConversation(ctx context.Context, pool *pgxpool.Pool, userA, userB int64) ([]*models.PrivateMessage, error) {
	rows, err := pool.Query(ctx,
		`SELECT pm.id, pm.sender_id, pm.recipient_id, pm.body, pm.read_at, pm.created_at,
		        s.username AS sender_username, r.username AS recipient_username
		 FROM private_messages pm
		 JOIN users s ON s.id = pm.sender_id
		 JOIN users r ON r.id = pm.recipient_id
		 WHERE (pm.sender_id = $1 AND pm.recipient_id = $2)
		    OR (pm.sender_id = $2 AND pm.recipient_id = $1)
		 ORDER BY pm.created_at ASC`,
		userA, userB,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []*models.PrivateMessage
	for rows.Next() {
		m := &models.PrivateMessage{}
		if err := rows.Scan(
			&m.ID, &m.SenderID, &m.RecipientID, &m.Body, &m.ReadAt, &m.CreatedAt,
			&m.SenderUsername, &m.RecipientUsername,
		); err != nil {
			return nil, err
		}
		messages = append(messages, m)
	}
	return messages, rows.Err()
}

// MarkRead marks all messages sent to userID in a conversation as read.
func MarkRead(ctx context.Context, pool *pgxpool.Pool, recipientID, senderID int64) error {
	_, err := pool.Exec(ctx,
		`UPDATE private_messages SET read_at = NOW()
		 WHERE recipient_id = $1 AND sender_id = $2 AND read_at IS NULL`,
		recipientID, senderID,
	)
	return err
}
