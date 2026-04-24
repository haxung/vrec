package model

import (
	"time"

	"github.com/shopspring/decimal"
)

type User struct {
	ID        int64           `json:"id"`
	Username  string          `json:"username"`
	Password  string          `json:"-"`
	Balance   decimal.Decimal `json:"balance"`
	QPSLimit  int             `json:"qps_limit"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}
