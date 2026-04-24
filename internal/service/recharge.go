package service

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
	"vrec/internal/model"
	"vrec/internal/repository"
	"vrec/pkg/errors"
)

type RechargeService struct {
	rechargeOrderRepo *repository.RechargeOrderRepository
	rechargeRepo      *repository.UserRechargeRepository
	userRepo          *repository.UserRepository
	paymentService    *PaymentService
	logger            *zap.Logger
}

func NewRechargeService(
	rechargeOrderRepo *repository.RechargeOrderRepository,
	rechargeRepo *repository.UserRechargeRepository,
	userRepo *repository.UserRepository,
	paymentService *PaymentService,
	logger *zap.Logger,
) *RechargeService {
	return &RechargeService{
		rechargeOrderRepo: rechargeOrderRepo,
		rechargeRepo:      rechargeRepo,
		userRepo:          userRepo,
		paymentService:    paymentService,
		logger:            logger,
	}
}

func (s *RechargeService) CreatePayOrder(ctx context.Context, userID, tokenID int64, amount string, payChannel model.PayChannel) (*model.RechargeOrder, error) {
	amountDec, err := decimal.NewFromString(amount)
	if err != nil {
		return nil, err
	}

	rechargeOrder := &model.RechargeOrder{
		RechargeNo: uuid.New(),
		UserID:    userID,
		TokenID:   tokenID,
		Amount:    amountDec,
		PayChannel: payChannel,
		Status:    model.RechargeStatusPending,
		ExpiresAt: time.Now().Add(30 * time.Minute),
	}

	payResp, err := s.paymentService.CreatePayOrder(ctx, &PayRequest{
		UserID:     userID,
		TokenID:    tokenID,
		Amount:     amount,
		PayChannel: payChannel,
	})
	if err != nil {
		return nil, err
	}

	rechargeOrder.PayURL = payResp.PayURL

	if err := s.rechargeOrderRepo.Create(ctx, rechargeOrder); err != nil {
		return nil, err
	}

	return rechargeOrder, nil
}

func (s *RechargeService) HandleCallback(ctx context.Context, tradeNo string, paid bool) error {
	order, err := s.rechargeOrderRepo.GetByTradeNo(ctx, tradeNo)
	if err != nil {
		return err
	}
	if order == nil {
		return errors.ErrRechargeNotFound
	}

	if order.Status == model.RechargeStatusPaid {
		return nil
	}

	if order.Status == model.RechargeStatusExpired || time.Now().After(order.ExpiresAt) {
		return errors.ErrRechargeExpired
	}

	if paid {
		if err := s.rechargeOrderRepo.UpdateStatus(ctx, order.RechargeNo, model.RechargeStatusPaid, tradeNo); err != nil {
			return err
		}

		recharge := &model.UserRecharge{
			UserID:  order.UserID,
			TokenID: order.TokenID,
			Amount:  order.Amount,
		}
		if err := s.rechargeRepo.Create(ctx, recharge); err != nil {
			return err
		}

		user, err := s.userRepo.GetByID(ctx, order.UserID)
		if err != nil {
			return err
		}

		userBalance, _ := decimal.NewFromString(user.Balance.String())
		newBalance := userBalance.Add(order.Amount)
		return s.userRepo.UpdateBalance(ctx, order.UserID, newBalance.String())
	} else {
		return s.rechargeOrderRepo.UpdateStatus(ctx, order.RechargeNo, model.RechargeStatusCancelled, tradeNo)
	}
}

func (s *RechargeService) GetByRechargeNo(ctx context.Context, rechargeNo uuid.UUID) (*model.RechargeOrder, error) {
	order, err := s.rechargeOrderRepo.GetByRechargeNo(ctx, rechargeNo)
	if err != nil {
		return nil, err
	}
	if order == nil {
		return nil, errors.ErrRechargeNotFound
	}
	return order, nil
}

func (s *RechargeService) GetUserRecharges(ctx context.Context, userID int64, limit, offset int) ([]*model.UserRecharge, error) {
	return s.rechargeRepo.GetByUserID(ctx, userID, limit, offset)
}

func (s *RechargeService) GetUserRechargeOrders(ctx context.Context, userID int64, limit int, afterOrderNo string, afterCreatedAt time.Time) ([]*model.RechargeOrder, error) {
	return s.rechargeOrderRepo.GetByUserID(ctx, userID, limit, afterOrderNo, afterCreatedAt)
}

func (s *RechargeService) GetRechargeOrdersByUserIDAndTimeRange(ctx context.Context, userID int64, startTime, endTime time.Time, limit int, afterOrderNo string, afterCreatedAt time.Time) ([]*model.RechargeOrder, error) {
	return s.rechargeOrderRepo.GetByUserIDAndTimeRange(ctx, userID, startTime, endTime, limit, afterOrderNo, afterCreatedAt)
}

func (s *RechargeService) GetRechargeOrdersForBill(ctx context.Context, userID int64, startTime, endTime time.Time) ([]*model.RechargeOrder, error) {
	return s.rechargeOrderRepo.GetByUserIDAndTimeRangeForBill(ctx, userID, startTime, endTime)
}

func (s *RechargeService) CountRechargeOrdersByTimeRange(ctx context.Context, userID int64, startTime, endTime time.Time) (int64, error) {
	return s.rechargeOrderRepo.CountByUserIDAndTimeRange(ctx, userID, startTime, endTime)
}
