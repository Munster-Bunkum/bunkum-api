package models

import "time"

type PrivateMessage struct {
	ID          int64     `json:"id"`
	SenderID    int64     `json:"sender_id"`
	RecipientID int64     `json:"recipient_id"`
	Body        string    `json:"body"`
	ReadAt      *time.Time `json:"read_at"`
	CreatedAt   time.Time `json:"created_at"`

	// Populated on read for convenience
	SenderUsername    string `json:"sender_username,omitempty"`
	RecipientUsername string `json:"recipient_username,omitempty"`
}
