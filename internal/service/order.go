package service

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
	"vrec/internal/config"
	"vrec/internal/model"
	"vrec/internal/repository"
	"vrec/pkg/errors"
)

type OrderService struct {
	cfg          *config.Config
	orderRepo    *repository.OrderRepository
	resultRepo   *repository.TranscriptionResultRepository
	userService  *UserService
	s3Service    *S3Service
	asrService    *ASRService
	logger       *zap.Logger
}

func NewOrderService(cfg *config.Config, orderRepo *repository.OrderRepository, resultRepo *repository.TranscriptionResultRepository, userService *UserService, s3Service *S3Service, asrService *ASRService, logger *zap.Logger) *OrderService {
	return &OrderService{
		cfg:         cfg,
		orderRepo:   orderRepo,
		resultRepo:  resultRepo,
		userService: userService,
		s3Service:   s3Service,
		asrService:  asrService,
		logger:      logger,
	}
}

// CalculateCosts 计算费用（存储费用 + ASR 费用 + 可选功能费用）
func (s *OrderService) CalculateCosts(duration int64, sizeBytes int64, needSubtitle, needMeetingNote bool) (storageCost, asrCost, subtitleCost, meetingCost, totalCost decimal.Decimal) {
	// ASR 费用 = 时长(分钟) × 单价
	durationMinutes := decimal.NewFromInt(duration).Div(decimal.NewFromInt(60))
	asrCost = durationMinutes.Mul(decimal.NewFromFloat(s.cfg.Pricing.ASRPerMinute))

	// 存储费用 = 大小(GB) × 天数 × 单价（假设存储 3 天）
	sizeGB := decimal.NewFromInt(sizeBytes).Div(decimal.NewFromInt(1024 * 1024 * 1024))
	storageCost = sizeGB.Mul(decimal.NewFromInt(3)).Mul(decimal.NewFromFloat(s.cfg.Pricing.StoragePerGBDay))

	// 字幕费用 = 时长(分钟) × 单价
	if needSubtitle {
		subtitleCost = durationMinutes.Mul(decimal.NewFromFloat(s.cfg.Pricing.SubtitlePerMinute))
	}

	// 会议纪要费用暂不计入创建订单时的费用，按需计算
	// meetingCost 在生成会议纪要时计算

	totalCost = storageCost.Add(asrCost).Add(subtitleCost)
	return
}

// CalculateMeetingNoteCost 根据文本 token 数计算会议纪要费用
func (s *OrderService) CalculateMeetingNoteCost(text string) decimal.Decimal {
	// 简单估算：按字符数估算 token（1 token ≈ 2 字符），然后按千 token 计费
	tokenCount := float64(len(text)) / 2
	return decimal.NewFromFloat(tokenCount / 1000 * s.cfg.Pricing.MeetingNotePerToken)
}

func (s *OrderService) CreateOrder(ctx context.Context, userID int64, tokenID int64, originalURL string, source model.OrderSource, audioInfo *AudioInfo, sizeBytes int64, needSubtitle, needMeetingNote bool, callbackURL string) (*model.Order, error) {
	storageCost, asrCost, subtitleCost, _, totalCost := s.CalculateCosts(audioInfo.Duration, sizeBytes, needSubtitle, needMeetingNote)

	// 检查余额是否充足（欠费用户不能创建订单）
	if err := s.userService.CheckBalanceForOrder(ctx, userID, totalCost.String()); err != nil {
		return nil, err
	}

	// 扣除总费用
	if err := s.userService.DeductBalance(ctx, userID, totalCost.String()); err != nil {
		return nil, err
	}

	order := &model.Order{
		OrderNo:        uuid.New(),
		UserID:         userID,
		TokenID:        tokenID,
		Status:         model.OrderStatusPending,
		OriginalURL:    originalURL,
		Source:         source,
		CallbackURL:    callbackURL,
		AudioDuration:  audioInfo.Duration,
		AudioFormat:    audioInfo.Format,
		SampleRate:     audioInfo.SampleRate,
		Channels:       audioInfo.Channels,
		BitRate:        audioInfo.BitRate,
		Codec:          audioInfo.Codec,
		StorageCost:    storageCost,
		ASRCost:        asrCost,
		SubtitleCost:   subtitleCost,
		TotalCost:      totalCost,
		NeedSubtitle:   needSubtitle,
		NeedMeetingNote: needMeetingNote,
	}

	if err := s.orderRepo.Create(ctx, order); err != nil {
		return nil, err
	}

	return order, nil
}

func (s *OrderService) GetByOrderNo(ctx context.Context, orderNo uuid.UUID) (*model.Order, error) {
	order, err := s.orderRepo.GetByOrderNo(ctx, orderNo)
	if err != nil {
		return nil, err
	}
	if order == nil {
		return nil, errors.ErrOrderNotFound
	}
	return order, nil
}

func (s *OrderService) GetUserOrders(ctx context.Context, userID int64, limit int, afterOrderNo string, afterCreatedAt time.Time) ([]*model.Order, error) {
	return s.orderRepo.GetByUserID(ctx, userID, limit, afterOrderNo, afterCreatedAt)
}

func (s *OrderService) GetOrdersByUserIDAndTimeRange(ctx context.Context, userID int64, startTime, endTime time.Time, limit int, afterOrderNo string, afterCreatedAt time.Time) ([]*model.Order, error) {
	return s.orderRepo.GetByUserIDAndTimeRange(ctx, userID, startTime, endTime, limit, afterOrderNo, afterCreatedAt)
}

func (s *OrderService) GetOrdersForBill(ctx context.Context, userID int64, startTime, endTime time.Time) ([]*model.Order, error) {
	return s.orderRepo.GetByUserIDAndTimeRangeForBill(ctx, userID, startTime, endTime)
}

func (s *OrderService) CountOrdersByTimeRange(ctx context.Context, userID int64, startTime, endTime time.Time) (int64, error) {
	return s.orderRepo.CountByUserIDAndTimeRange(ctx, userID, startTime, endTime)
}

func (s *OrderService) UpdateStatus(ctx context.Context, orderNo uuid.UUID, status model.OrderStatus) error {
	return s.orderRepo.UpdateStatus(ctx, orderNo, status)
}

func (s *OrderService) UpdateTaskID(ctx context.Context, orderNo uuid.UUID, taskID string) error {
	return s.orderRepo.UpdateTaskID(ctx, orderNo, taskID)
}

func (s *OrderService) UpdateS3Info(ctx context.Context, orderNo uuid.UUID, s3URL, s3Key string, expiresAt *time.Time) error {
	return s.orderRepo.UpdateS3Info(ctx, orderNo, s3URL, s3Key, expiresAt)
}

func (s *OrderService) IsS3URLExpired(order *model.Order) bool {
	if order.S3URL == "" || order.S3ExpiresAt == nil {
		return true
	}
	return time.Now().After(*order.S3ExpiresAt)
}

func (s *OrderService) CancelOrder(ctx context.Context, orderNo uuid.UUID, taskID string) error {
	// 先调用 ASR 取消任务
	if taskID != "" {
		if err := s.asrService.CancelTask(ctx, taskID); err != nil {
			s.logger.Warn("failed to cancel asr task",
				zap.String("task_id", taskID),
				zap.String("order_no", orderNo.String()),
				zap.Error(err),
			)
			// ASR 取消失败不影响订单取消，继续更新订单状态
		}
	}

	// 更新订单状态为 canceled
	return s.orderRepo.CancelOrder(ctx, orderNo)
}

func (s *OrderService) GetInsufficientOrders(ctx context.Context, userID int64, limit int, afterOrderNo string, afterCreatedAt time.Time) ([]*model.Order, error) {
	return s.orderRepo.GetInsufficientOrdersByUserID(ctx, userID, limit, afterOrderNo, afterCreatedAt)
}

func (s *OrderService) RetryInsufficientOrder(ctx context.Context, orderNo uuid.UUID) error {
	// 获取订单
	order, err := s.orderRepo.GetByOrderNo(ctx, orderNo)
	if err != nil {
		return err
	}
	if order == nil {
		return errors.ErrOrderNotFound
	}
	if order.Status != model.OrderStatusInsufficient {
		return errors.ErrOrderStatusInvalid
	}

	// 检查余额是否充足
	if err := s.userService.CheckBalanceForOrder(ctx, order.UserID, order.TotalCost.String()); err != nil {
		return err
	}

	// 扣除费用
	if err := s.userService.DeductBalance(ctx, order.UserID, order.TotalCost.String()); err != nil {
		return err
	}

	// 重置订单状态为 pending
	return s.orderRepo.ResetInsufficientOrder(ctx, orderNo)
}

func (s *OrderService) CheckUserBalance(ctx context.Context, userID int64) (decimal.Decimal, error) {
	return s.userService.GetBalance(ctx, userID)
}

func (s *OrderService) GetProcessingOrders(ctx context.Context) ([]*model.Order, error) {
	return s.orderRepo.GetProcessingOrders(ctx)
}

func (s *OrderService) CreateTranscriptionResult(ctx context.Context, orderNo uuid.UUID, resultS3Key, resultText string) error {
	result := &model.TranscriptionResult{
		OrderNo:     orderNo,
		ResultS3Key: resultS3Key,
		ResultText:  resultText,
	}
	return s.resultRepo.Create(ctx, result)
}
