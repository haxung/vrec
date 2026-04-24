package service

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"vrec/internal/model"
)

type PaymentService struct {
	logger *zap.Logger
}

func NewPaymentService(logger *zap.Logger) *PaymentService {
	return &PaymentService{logger: logger}
}

type PayRequest struct {
	UserID     int64
	TokenID    int64
	Amount     string
	PayChannel model.PayChannel
}

type PayResponse struct {
	RechargeNo string `json:"recharge_no"`
	PayURL     string `json:"pay_url"`
	ExpiresAt  string `json:"expires_at"`
}

func (s *PaymentService) CreatePayOrder(ctx context.Context, req *PayRequest) (*PayResponse, error) {
	switch req.PayChannel {
	case model.PayChannelAlipay:
		return s.createAlipayOrder(ctx, req)
	case model.PayChannelWechat:
		return s.createWechatOrder(ctx, req)
	default:
		return nil, fmt.Errorf("unsupported pay channel: %s", req.PayChannel)
	}
}

func (s *PaymentService) createAlipayOrder(ctx context.Context, req *PayRequest) (*PayResponse, error) {
	return &PayResponse{
		RechargeNo: uuid.New().String(),
		PayURL:     "https://alipay.com/qr/not-implemented",
		ExpiresAt:  "",
	}, nil
}

func (s *PaymentService) createWechatOrder(ctx context.Context, req *PayRequest) (*PayResponse, error) {
	return &PayResponse{
		RechargeNo: uuid.New().String(),
		PayURL:     "https://wechat.com/qr/not-implemented",
		ExpiresAt:  "",
	}, nil
}

func (s *PaymentService) QueryOrder(ctx context.Context, rechargeNo string) (*model.RechargeOrder, error) {
	return nil, fmt.Errorf("not implemented: query order")
}

func (s *PaymentService) CloseOrder(ctx context.Context, rechargeNo string) error {
	return fmt.Errorf("not implemented: close order")
}
