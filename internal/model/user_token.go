package model

import (
	"time"

	"github.com/google/uuid"
)

type UserToken struct {
	ID        int64       `json:"id"`
	UserID    int64       `json:"user_id"`
	Token     uuid.UUID   `json:"token"`
	CreatedAt time.Time   `json:"created_at"`
	ExpiresAt time.Time   `json:"expires_at"`
	UpdatedAt time.Time   `json:"updated_at"`
}
