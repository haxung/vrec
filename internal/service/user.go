package service

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
	"vrec/internal/model"
	"vrec/internal/repository"
	"vrec/pkg/errors"
)

type UserService struct {
	userRepo     *repository.UserRepository
	tokenRepo    *repository.UserTokenRepository
	rechargeRepo *repository.UserRechargeRepository
	logger       *zap.Logger
}

func NewUserService(userRepo *repository.UserRepository, tokenRepo *repository.UserTokenRepository, rechargeRepo *repository.UserRechargeRepository, logger *zap.Logger) *UserService {
	return &UserService{userRepo: userRepo, tokenRepo: tokenRepo, rechargeRepo: rechargeRepo, logger: logger}
}

func (s *UserService) Register(ctx context.Context, username, password string) (*model.User, error) {
	existing, err := s.userRepo.GetByUsername(ctx, username)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, errors.ErrUserAlreadyExists
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	user := &model.User{
		Username: username,
		Password: string(hash),
		Balance:  decimal.NewFromInt(0),
		QPSLimit: 10,
	}
	if err := s.userRepo.Create(ctx, user); err != nil {
		return nil, err
	}
	return user, nil
}

func (s *UserService) Login(ctx context.Context, username, password string) (*model.User, *model.UserToken, error) {
	user, err := s.userRepo.GetByUsername(ctx, username)
	if err != nil {
		return nil, nil, err
	}
	if user == nil {
		return nil, nil, errors.ErrUserNotFound
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		return nil, nil, errors.ErrInvalidPassword
	}

	token := &model.UserToken{
		UserID:    user.ID,
		Token:     uuid.New(),
		ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
	}
	if err := s.tokenRepo.Create(ctx, token); err != nil {
		return nil, nil, err
	}

	return user, token, nil
}

func (s *UserService) ValidateToken(ctx context.Context, tokenStr string) (int64, int64, error) {
	token, err := uuid.Parse(tokenStr)
	if err != nil {
		return 0, 0, errors.ErrAuthInvalidToken
	}

	userToken, err := s.tokenRepo.GetByToken(ctx, token)
	if err != nil {
		return 0, 0, err
	}
	if userToken == nil {
		return 0, 0, errors.ErrAuthInvalidToken
	}

	if userToken.ExpiresAt.Before(time.Now()) {
		return 0, 0, errors.ErrAuthTokenExpired
	}

	return userToken.UserID, userToken.ID, nil
}

func (s *UserService) Logout(ctx context.Context, tokenStr string) error {
	token, err := uuid.Parse(tokenStr)
	if err != nil {
		return errors.ErrAuthInvalidToken
	}
	return s.tokenRepo.DeleteByToken(ctx, token)
}

func (s *UserService) LogoutAll(ctx context.Context, userID int64) error {
	return s.tokenRepo.DeleteByUserID(ctx, userID)
}

func (s *UserService) GetUserTokens(ctx context.Context, userID int64) ([]*model.UserToken, error) {
	return s.tokenRepo.GetByUserID(ctx, userID)
}

func (s *UserService) DeleteToken(ctx context.Context, userID int64, tokenID int64) error {
	token, err := s.tokenRepo.GetByID(ctx, tokenID)
	if err != nil {
		return err
	}
	if token == nil || token.UserID != userID {
		return errors.ErrAuthInvalidToken
	}
	return s.tokenRepo.DeleteByID(ctx, tokenID)
}

func (s *UserService) GetByID(ctx context.Context, id int64) (*model.User, error) {
	user, err := s.userRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, errors.ErrUserNotFound
	}
	return user, nil
}

func (s *UserService) DeductBalance(ctx context.Context, userID int64, amount string) error {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return err
	}
	if user == nil {
		return errors.ErrUserNotFound
	}

	userBalance, _ := decimal.NewFromString(user.Balance.String())
	deductAmount, _ := decimal.NewFromString(amount)
	if userBalance.LessThan(deductAmount) {
		return errors.ErrInsufficientBalance
	}

	newBalance := userBalance.Sub(deductAmount)
	return s.userRepo.UpdateBalance(ctx, userID, newBalance.String())
}

// CheckBalanceForOrder 检查余额是否足够创建订单（欠费用户不能创建订单）
func (s *UserService) CheckBalanceForOrder(ctx context.Context, userID int64, amount string) error {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return err
	}
	if user == nil {
		return errors.ErrUserNotFound
	}

	userBalance, _ := decimal.NewFromString(user.Balance.String())
	deductAmount, _ := decimal.NewFromString(amount)
	if userBalance.LessThan(deductAmount) {
		return errors.ErrInsufficientBalance
	}
	return nil
}

// GetBalance 获取用户余额
func (s *UserService) GetBalance(ctx context.Context, userID int64) (decimal.Decimal, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return decimal.Zero, err
	}
	if user == nil {
		return decimal.Zero, errors.ErrUserNotFound
	}
	return user.Balance, nil
}

func (s *UserService) GetBalanceFloat(ctx context.Context, userID int64) (float64, error) {
	balance, err := s.GetBalance(ctx, userID)
	if err != nil {
		return 0, err
	}
	f, _ := balance.Float64()
	return f, nil
}

func (s *UserService) AddBalance(ctx context.Context, userID int64, tokenID int64, amount string) (*model.UserRecharge, error) {
	rechargeAmount, _ := decimal.NewFromString(amount)

	recharge := &model.UserRecharge{
		UserID:  userID,
		TokenID: tokenID,
		Amount:  rechargeAmount,
	}
	if err := s.rechargeRepo.Create(ctx, recharge); err != nil {
		return nil, err
	}

	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, errors.ErrUserNotFound
	}

	userBalance, _ := decimal.NewFromString(user.Balance.String())
	newBalance := userBalance.Add(rechargeAmount)
	if err := s.userRepo.UpdateBalance(ctx, userID, newBalance.String()); err != nil {
		return nil, err
	}

	return recharge, nil
}
