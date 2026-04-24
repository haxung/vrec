package model

import (
	"time"

	"github.com/shopspring/decimal"
)

type UserRecharge struct {
	ID        int64           `json:"id"`
	UserID    int64           `json:"user_id"`
	TokenID   int64           `json:"token_id"`
	Amount    decimal.Decimal `json:"amount"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}
