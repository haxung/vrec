package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"vrec/internal/config"
	"vrec/internal/model"
	"vrec/internal/repository"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

type SubtitleService struct {
	cfg        *config.Config
	s3Service  *S3Service
	resultRepo *repository.TranscriptionResultRepository
	logger     *zap.Logger
}

func NewSubtitleService(cfg *config.Config, s3Service *S3Service, resultRepo *repository.TranscriptionResultRepository, logger *zap.Logger) *SubtitleService {
	return &SubtitleService{
		cfg:        cfg,
		s3Service:  s3Service,
		resultRepo: resultRepo,
		logger:     logger,
	}
}

type GenerateSubtitleRequest struct {
	OrderNo  uuid.UUID
	AudioURL string
	Format   string
}

func (s *SubtitleService) GenerateSubtitle(ctx context.Context, req *GenerateSubtitleRequest) (*model.TranscriptionResult, error) {
	// 获取转写结果
	result, err := s.resultRepo.GetByOrderNo(ctx, req.OrderNo)
	if err != nil {
		return nil, fmt.Errorf("get transcription result failed: %w", err)
	}
	if result == nil {
		return nil, fmt.Errorf("transcription result not found")
	}

	// 生成字幕文件（SRT格式）
	srtContent, err := s.generateSRTFromASR(result.ResultText)
	if err != nil {
		return nil, err
	}

	// 上传字幕到 S3
	subtitleS3Key := s.s3Service.GenerateKeyWithFilename("subtitle", "subtitle.srt")
	_, err = s.s3Service.Upload(ctx, subtitleS3Key, strings.NewReader(srtContent), int64(len(srtContent)), "application/x-subrip")
	if err != nil {
		s.logger.Warn("failed to upload subtitle to s3",
			zap.String("order_no", req.OrderNo.String()),
			zap.Error(err),
		)
		// S3 上传失败，内容小于阈值时存储在 ResultText 中（这里字幕一般较小，不做本地存储）
		subtitleS3Key = ""
	}

	// 更新 transcription_results 的 subtitle_s3_key
	err = s.resultRepo.UpdateSubtitle(ctx, req.OrderNo, subtitleS3Key)
	if err != nil {
		return nil, fmt.Errorf("update subtitle failed: %w", err)
	}

	result.SubtitleS3Key = subtitleS3Key

	s.logger.Info("subtitle generated",
		zap.String("order_no", req.OrderNo.String()),
		zap.String("subtitle_s3_key", subtitleS3Key),
	)

	return result, nil
}

func (s *SubtitleService) GetSubtitleContent(ctx context.Context, orderNo uuid.UUID) (string, error) {
	result, err := s.resultRepo.GetByOrderNo(ctx, orderNo)
	if err != nil {
		return "", err
	}
	if result == nil {
		return "", fmt.Errorf("transcription result not found")
	}

	// 优先从 S3 下载字幕
	if result.SubtitleS3Key != "" {
		data, err := s.s3Service.Download(ctx, result.SubtitleS3Key)
		if err == nil && len(data) > 0 {
			return string(data), nil
		}
	}

	// S3 没有或下载失败，直接从 ASR 结果生成
	return s.generateSRTFromASR(result.ResultText)
}

func (s *SubtitleService) GetSubtitleCost(durationMinutes float64) float64 {
	return durationMinutes * s.cfg.Pricing.SubtitlePerMinute
}

func formatSRTTime(d time.Duration) string {
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60
	millis := int(d.Milliseconds()) % 1000
	return fmt.Sprintf("%02d:%02d:%02d,%03d", hours, minutes, seconds, millis)
}

// generateSRTFromASR 从 ASR 结果生成 SRT 字幕（使用精确时间戳）
func (s *SubtitleService) generateSRTFromASR(asrJSON string) (string, error) {
	if asrJSON == "" {
		return "", nil
	}

	var transcription model.TranscriptionResponse
	if err := json.Unmarshal([]byte(asrJSON), &transcription); err != nil {
		// ASR 结果解析失败，返回错误
		return "", err
	}

	var srt strings.Builder
	index := 1

	for _, transcript := range transcription.Transcripts {
		for _, sentence := range transcript.Sentences {
			if sentence.Text == "" {
				continue
			}

			startTime := time.Duration(sentence.BeginTime) * time.Millisecond
			endTime := time.Duration(sentence.EndTime) * time.Millisecond

			fmt.Fprintf(&srt, "%d\n", index)
			fmt.Fprintf(&srt, "%s --> %s\n", formatSRTTime(startTime), formatSRTTime(endTime))
			srt.WriteString(sentence.Text)
			srt.WriteString("\n\n")
			index++
		}
	}

	return srt.String(), nil
}
