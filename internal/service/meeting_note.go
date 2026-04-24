package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
	"vrec/internal/config"
	"vrec/internal/model"
	"vrec/internal/repository"
)

type MeetingNoteService struct {
	cfg         *config.Config
	llmService  *LLMService
	s3Service   *S3Service
	summaryRepo *repository.MeetingSummaryRepository
	resultRepo  *repository.TranscriptionResultRepository
	logger      *zap.Logger
}

func NewMeetingNoteService(cfg *config.Config, llmService *LLMService, s3Service *S3Service, summaryRepo *repository.MeetingSummaryRepository, resultRepo *repository.TranscriptionResultRepository, logger *zap.Logger) *MeetingNoteService {
	return &MeetingNoteService{
		cfg:         cfg,
		llmService:  llmService,
		s3Service:   s3Service,
		summaryRepo: summaryRepo,
		resultRepo:  resultRepo,
		logger:      logger,
	}
}

type GenerateMeetingNoteRequest struct {
	OrderNo uuid.UUID
	Text    string
}

// formatTime 将毫秒转换为 HH:MM:SS 格式
func formatTime(ms int64) string {
	totalSec := ms / 1000
	hours := totalSec / 3600
	minutes := (totalSec % 3600) / 60
	seconds := totalSec % 60
	return fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds)
}

// parseTranscriptionAndFormat 解析 ASR JSON 结果并格式化为对话格式
func parseTranscriptionAndFormat(asrJSON string) (string, error) {
	var resp model.TranscriptionResponse
	if err := json.Unmarshal([]byte(asrJSON), &resp); err != nil {
		// 如果解析失败，返回原始文本
		return asrJSON, err
	}

	var lines []string
	for _, transcript := range resp.Transcripts {
		for _, sentence := range transcript.Sentences {
			speaker := fmt.Sprintf("说话人%d", sentence.SpeakerID)
			timeStr := formatTime(sentence.BeginTime)
			line := fmt.Sprintf("[%s] %s: %s", timeStr, speaker, sentence.Text)
			lines = append(lines, line)
		}
	}

	if len(lines) == 0 {
		// 如果没有句子级结果，尝试使用段落级结果
		for _, transcript := range resp.Transcripts {
			if transcript.Text != "" {
				lines = append(lines, transcript.Text)
			}
		}
	}

	return strings.Join(lines, "\n"), nil
}

func (s *MeetingNoteService) GenerateMeetingNote(ctx context.Context, req *GenerateMeetingNoteRequest) (*model.MeetingSummary, error) {
	if !s.llmService.IsEnabled() {
		return nil, fmt.Errorf("llm is not enabled, please enable it in config")
	}

	// 获取转写结果
	result, err := s.resultRepo.GetByOrderNo(ctx, req.OrderNo)
	if err != nil {
		return nil, fmt.Errorf("get transcription result failed: %w", err)
	}
	if result == nil {
		return nil, fmt.Errorf("transcription result not found")
	}

	// 解析 ASR 结果并格式化为对话格式（时间/说话人/内容）
	dialogueText, err := parseTranscriptionAndFormat(result.ResultText)
	if err != nil {
		s.logger.Warn("failed to parse ASR result as structured format, using raw text",
			zap.String("order_no", req.OrderNo.String()),
			zap.Error(err))
		dialogueText = result.ResultText
	}

	// 调用 LLM 生成会议纪要
	summaryText, err := s.llmService.GenerateMeetingSummary(ctx, &GenerateSummaryRequest{
		Text: dialogueText,
	})
	if err != nil {
		return nil, fmt.Errorf("generate meeting summary failed: %w", err)
	}

	// 上传会议纪要到 S3
	summaryS3Key := s.s3Service.GenerateKeyWithFilename("meeting_summary", "meeting_summary.md")
	_, err = s.s3Service.Upload(ctx, summaryS3Key, &readerProvider{text: summaryText}, int64(len(summaryText)), "text/markdown")
	if err != nil {
		s.logger.Warn("failed to upload meeting summary to s3",
			zap.String("order_no", req.OrderNo.String()),
			zap.Error(err),
		)
		// S3 上传失败，summaryText 会保存在数据库
		summaryS3Key = ""
	}

	// 计算会议纪要费用：按 token 数计费
	tokenCount := float64(len(summaryText)) / 2
	cost := decimal.NewFromFloat(tokenCount / 1000 * s.cfg.Pricing.MeetingNotePerToken)

	// 保存会议纪要
	summary := &model.MeetingSummary{
		OrderNo:      req.OrderNo,
		SummaryS3Key: summaryS3Key,
		SummaryText:  summaryText,
		Cost:         cost,
	}

	if err := s.summaryRepo.Create(ctx, summary); err != nil {
		return nil, fmt.Errorf("save meeting summary failed: %w", err)
	}

	s.logger.Info("meeting note generated",
		zap.String("order_no", req.OrderNo.String()),
		zap.String("cost", summary.Cost.String()),
	)

	return summary, nil
}

func (s *MeetingNoteService) GetMeetingNote(ctx context.Context, orderNo uuid.UUID) (*model.MeetingSummary, error) {
	return s.summaryRepo.GetByOrderNo(ctx, orderNo)
}

func (s *MeetingNoteService) GetMeetingNoteURL(ctx context.Context, orderNo uuid.UUID) (string, error) {
	summary, err := s.summaryRepo.GetByOrderNo(ctx, orderNo)
	if err != nil {
		return "", err
	}
	if summary == nil {
		return "", fmt.Errorf("meeting summary not found")
	}
	if summary.SummaryS3Key == "" {
		return "", nil
	}
	return s.s3Service.GeneratePresignedURLByS3Key(ctx, summary.SummaryS3Key)
}

func (s *MeetingNoteService) GetMeetingNoteContent(ctx context.Context, orderNo uuid.UUID) (string, error) {
	summary, err := s.summaryRepo.GetByOrderNo(ctx, orderNo)
	if err != nil {
		return "", err
	}
	if summary == nil {
		return "", fmt.Errorf("meeting summary not found")
	}
	// 优先返回数据库中的内容
	if summary.SummaryText != "" {
		return summary.SummaryText, nil
	}
	// 否则从 S3 下载
	if summary.SummaryS3Key == "" {
		return "", nil
	}
	presignedURL, err := s.s3Service.GeneratePresignedURLByS3Key(ctx, summary.SummaryS3Key)
	if err != nil || presignedURL == "" {
		return "", err
	}
	resp, err := http.Get(presignedURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func (s *MeetingNoteService) IsLLMEnabled() bool {
	return s.llmService.IsEnabled()
}

type readerProvider struct {
	text string
	pos  int
}

func (r *readerProvider) Read(p []byte) (n int, err error) {
	if r.pos >= len(r.text) {
		return 0, nil
	}
	n = copy(p, r.text[r.pos:])
	r.pos += n
	return n, nil
}
