package model

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type PayChannel string

const (
	PayChannelAlipay PayChannel = "alipay"
	PayChannelWechat PayChannel = "wechat"
)

type RechargeStatus string

const (
	RechargeStatusPending   RechargeStatus = "pending"
	RechargeStatusPaid      RechargeStatus = "paid"
	RechargeStatusCancelled RechargeStatus = "cancelled"
	RechargeStatusExpired    RechargeStatus = "expired"
)

type RechargeOrder struct {
	ID          int64            `json:"id"`
	RechargeNo  uuid.UUID       `json:"recharge_no"`
	UserID      int64           `json:"user_id"`
	TokenID     int64           `json:"token_id"`
	Amount      decimal.Decimal `json:"amount"`
	PayChannel  PayChannel      `json:"pay_channel"`
	Status      RechargeStatus  `json:"status"`
	TradeNo     string          `json:"trade_no"`
	PayURL      string          `json:"pay_url"`
	ExpiresAt   time.Time      `json:"expires_at"`
	PaidAt      *time.Time     `json:"paid_at,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}
