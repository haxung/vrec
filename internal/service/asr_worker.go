package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
	"vrec/internal/config"
	"vrec/internal/model"
)

type ASRWorker struct {
	orderService *OrderService
	asrService   *ASRService
	s3Service    *S3Service
	cfg          *config.Config
	logger       *zap.Logger
	stopCh       chan struct{}
}

func NewASRWorker(orderService *OrderService, asrService *ASRService, s3Service *S3Service, cfg *config.Config, logger *zap.Logger) *ASRWorker {
	return &ASRWorker{
		orderService: orderService,
		asrService:   asrService,
		s3Service:    s3Service,
		cfg:          cfg,
		logger:       logger,
		stopCh:       make(chan struct{}),
	}
}

func (w *ASRWorker) Start() {
	w.logger.Info("asr worker started")
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-w.stopCh:
			w.logger.Info("asr worker stopped")
			return
		case <-ticker.C:
			w.poll()
		}
	}
}

func (w *ASRWorker) Stop() {
	close(w.stopCh)
}

func (w *ASRWorker) poll() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	orders, err := w.orderService.GetProcessingOrders(ctx)
	if err != nil {
		w.logger.Error("failed to get processing orders", zap.Error(err))
		return
	}

	if len(orders) == 0 {
		return
	}

	w.logger.Info("polling asr tasks", zap.Int("count", len(orders)))

	for _, order := range orders {
		if order.TaskID == "" {
			continue
		}

		status, resultURL, err := w.asrService.GetStatus(ctx, order.TaskID)
		if err != nil {
			w.logger.Error("failed to get task status",
				zap.String("task_id", order.TaskID),
				zap.Error(err),
			)
			continue
		}

		switch status {
		case TaskStatusSUCCEEDED:
			w.handleSuccess(ctx, order, resultURL)
		case TaskStatusFAILED, TaskStatusCANCELED:
			w.handleFailed(ctx, order.OrderNo)
		}
	}
}

func (w *ASRWorker) handleSuccess(ctx context.Context, order *model.Order, resultURL string) {
	// 0. 检查用户余额是否充足
	balance, err := w.orderService.CheckUserBalance(ctx, order.UserID)
	if err != nil || balance.LessThanOrEqual(decimal.Zero) {
		// 余额不足，标记为 insufficient 状态
		w.logger.Warn("insufficient balance, marking order as insufficient",
			zap.String("order_no", order.OrderNo.String()),
			zap.String("user_id", fmt.Sprintf("%d", order.UserID)),
		)
		if err := w.orderService.UpdateStatus(ctx, order.OrderNo, model.OrderStatusInsufficient); err != nil {
			w.logger.Error("failed to update order status to insufficient",
				zap.String("order_no", order.OrderNo.String()),
				zap.Error(err),
			)
		}
		// 取消 ASR 任务
		if order.TaskID != "" {
			w.asrService.CancelTask(ctx, order.TaskID)
		}
		return
	}

	// 1. 下载转写结果内容
	resultText, err := w.downloadTranscription(ctx, resultURL)
	if err != nil {
		w.logger.Error("failed to download transcription",
			zap.String("order_no", order.OrderNo.String()),
			zap.Error(err),
		)
		// 下载失败时仍保存 ASR 返回的 URL
		resultText = ""
	}

	var s3Key string
	// 2. 上传到 S3
	if resultText != "" {
		s3Key = w.s3Service.GenerateKeyWithFilename("transcription", "transcription.json")
		_, err := w.s3Service.Upload(ctx, s3Key, bytes.NewReader([]byte(resultText)), int64(len(resultText)), "application/json")
		if err != nil {
			w.logger.Warn("failed to upload to s3, will store locally",
				zap.String("order_no", order.OrderNo.String()),
				zap.Error(err),
			)
			// S3 上传失败时检查阈值
			if int64(len(resultText)) < w.cfg.Pricing.LocalStorageThreshold {
				s3Key = "" // 内容小于阈值，存储在数据库
			} else {
				// 内容大于阈值，上传失败，返回错误
				w.logger.Error("transcription too large to store locally",
					zap.String("order_no", order.OrderNo.String()),
					zap.Int64("size", int64(len(resultText))),
					zap.Int64("threshold", w.cfg.Pricing.LocalStorageThreshold),
				)
				return
			}
		}
	}

	// 3. 保存转写结果
	if err := w.orderService.CreateTranscriptionResult(ctx, order.OrderNo, s3Key, resultText); err != nil {
		w.logger.Error("failed to save transcription result",
			zap.String("order_no", order.OrderNo.String()),
			zap.Error(err),
		)
		return
	}

	// 4. 更新订单状态
	if err := w.orderService.UpdateStatus(ctx, order.OrderNo, model.OrderStatusSuccess); err != nil {
		w.logger.Error("failed to update order status",
			zap.String("order_no", order.OrderNo.String()),
			zap.Error(err),
		)
		return
	}

	// 5. 如果有回调地址，主动推送转写结果
	if order.CallbackURL != "" {
		w.deliverCallback(ctx, order, s3Key, resultText)
	}

	w.logger.Info("asr task succeeded",
		zap.String("order_no", order.OrderNo.String()),
		zap.String("s3_key", s3Key),
	)
}

func (w *ASRWorker) downloadTranscription(ctx context.Context, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", err
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(body), nil
}

func (w *ASRWorker) deliverCallback(ctx context.Context, order *model.Order, s3Key string, resultText string) {
	// 获取实际内容：优先使用 resultText，否则从 S3 下载
	content := resultText
	if content == "" && s3Key != "" {
		presignedURL, err := w.s3Service.GeneratePresignedURLByS3Key(ctx, s3Key)
		if err == nil && presignedURL != "" {
			content, _ = w.downloadTranscription(ctx, presignedURL)
		}
	}

	// 解析 ASR 原始结果，返回结构化数据
	var asrResult model.TranscriptionResponse
	if err := json.Unmarshal([]byte(content), &asrResult); err != nil {
		// 解析失败时返回原始文本
		callbackPayload := map[string]interface{}{
			"order_no":    order.OrderNo.String(),
			"status":      "success",
			"result_text": content,
			"need_subtitle":    order.NeedSubtitle,
			"need_meeting_note": order.NeedMeetingNote,
		}
		body, _ := json.Marshal(callbackPayload)
		w.sendCallback(ctx, order, body)
		return
	}

	// 返回与 ASR 格式一致的结构化结果
	callbackPayload := map[string]interface{}{
		"order_no":    order.OrderNo.String(),
		"status":      "success",
		"file_url":    asrResult.FileURL,
		"properties":  asrResult.Properties,
		"transcripts": asrResult.Transcripts,
		"need_subtitle":    order.NeedSubtitle,
		"need_meeting_note": order.NeedMeetingNote,
	}

	body, err := json.Marshal(callbackPayload)
	if err != nil {
		w.logger.Error("failed to marshal callback payload",
			zap.String("order_no", order.OrderNo.String()),
			zap.Error(err),
		)
		return
	}

	w.sendCallback(ctx, order, body)
}

func (w *ASRWorker) sendCallback(ctx context.Context, order *model.Order, body []byte) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, order.CallbackURL, bytes.NewReader(body))
	if err != nil {
		w.logger.Error("failed to create callback request",
			zap.String("order_no", order.OrderNo.String()),
			zap.Error(err),
		)
		return
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		w.logger.Error("failed to deliver callback",
			zap.String("order_no", order.OrderNo.String()),
			zap.String("callback_url", order.CallbackURL),
			zap.Error(err),
		)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		w.logger.Info("callback delivered successfully",
			zap.String("order_no", order.OrderNo.String()),
			zap.String("callback_url", order.CallbackURL),
		)
	} else {
		w.logger.Warn("callback returned non-200 status",
			zap.String("order_no", order.OrderNo.String()),
			zap.String("callback_url", order.CallbackURL),
			zap.Int("status", resp.StatusCode),
		)
	}
}

func (w *ASRWorker) handleFailed(ctx context.Context, orderNo uuid.UUID) {
	if err := w.orderService.UpdateStatus(ctx, orderNo, model.OrderStatusFailed); err != nil {
		w.logger.Error("failed to update order status",
			zap.String("order_no", orderNo.String()),
			zap.Error(err),
		)
		return
	}

	w.logger.Info("asr task failed", zap.String("order_no", orderNo.String()))
}
