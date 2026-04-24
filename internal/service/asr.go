package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"vrec/internal/config"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

type ASRService struct {
	cfg        *config.ASRConfig
	logger     *zap.Logger
	httpClient *http.Client
}

func NewASRService(cfg *config.ASRConfig, logger *zap.Logger) *ASRService {
	return &ASRService{
		cfg:    cfg,
		logger: logger,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

type TranscribeRequest struct {
	AudioURL    string
	CallbackURL string
	OrderNo     uuid.UUID

	// FunASR parameters
	ChannelID          []int    // 音轨索引，从0开始，默认 [0]
	SpeakerCount       int      // 说话人数量参考值，范围2-100
	LanguageHints      []string // 语言提示，如 ["zh", "en"]
	SpecialWordFilter  bool     // 是否启用特殊词汇过滤
	VocabularyID       string   // 热词ID
}

type TranscribeResponse struct {
	TaskID string `json:"task_id"`
	Status string `json:"task_status"`
}

type taskStatusResponse struct {
	Output struct {
		TaskID     string `json:"task_id"`
		TaskStatus string `json:"task_status"`
		Results    []struct {
			FileURL          string `json:"file_url"`
			TranscriptionURL string `json:"transcription_url"`
			SubtaskStatus    string `json:"subtask_status"`
			Code             string `json:"code,omitempty"`
			Message          string `json:"message,omitempty"`
		} `json:"results"`
		TaskMetrics struct {
			Total     int `json:"TOTAL"`
			Succeeded int `json:"SUCCEEDED"`
			Failed    int `json:"FAILED"`
		} `json:"task_metrics"`
	} `json:"output"`
	Usage struct {
		Duration int `json:"duration"`
	} `json:"usage"`
}

const (
	TaskStatusSubmitting = "SUBMITTING"
	TaskStatusPending    = "PENDING"
	TaskStatusProcessing = "PROCESSING"
	TaskStatusSUCCEEDED  = "SUCCEEDED"
	TaskStatusFAILED     = "FAILED"
	TaskStatusCANCELED   = "CANCELED"
)

func (s *ASRService) Transcribe(ctx context.Context, req *TranscribeRequest) (*TranscribeResponse, error) {
	submitURL := "https://dashscope.aliyuncs.com/api/v1/services/audio/asr/transcription"

	payload := map[string]any{
		"model": s.cfg.Model,
		"input": map[string]any{
			"file_urls": []string{req.AudioURL},
		},
		"parameters": map[string]any{
			"diarization_enabled": true,
		},
	}

	if req.CallbackURL != "" {
		payload["callback_url"] = req.CallbackURL
	}
	if len(req.ChannelID) > 0 {
		payload["parameters"].(map[string]any)["channel_id"] = req.ChannelID
	}
	if req.SpeakerCount >= 2 && req.SpeakerCount <= 100 {
		payload["parameters"].(map[string]any)["speaker_count"] = req.SpeakerCount
	}
	if len(req.LanguageHints) > 0 {
		payload["parameters"].(map[string]any)["language_hints"] = req.LanguageHints
	}
	if req.SpecialWordFilter {
		payload["parameters"].(map[string]any)["special_word_filter"] = true
	}
	if req.VocabularyID != "" {
		payload["input"].(map[string]any)["vocabulary_id"] = req.VocabularyID
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal payload failed: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, submitURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request failed: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+s.cfg.APIKey)
	httpReq.Header.Set("X-DashScope-Async", "enable")

	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("submit transcription failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("submit failed, status: %d, body: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Output struct {
			TaskID     string `json:"task_id"`
			TaskStatus string `json:"task_status"`
		} `json:"output"`
		RequestId string `json:"request_id"`
	}

	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("parse response failed: %w", err)
	}

	s.logger.Info("asr task submitted",
		zap.String("request_id", result.RequestId),
		zap.String("task_id", result.Output.TaskID),
		zap.String("status", result.Output.TaskStatus),
		zap.String("order_no", req.OrderNo.String()),
	)

	return &TranscribeResponse{
		TaskID: result.Output.TaskID,
		Status: result.Output.TaskStatus,
	}, nil
}

func (s *ASRService) GetStatus(ctx context.Context, taskID string) (string, string, error) {
	queryURL := fmt.Sprintf("https://dashscope.aliyuncs.com/api/v1/tasks/%s", taskID)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, queryURL, nil)
	if err != nil {
		return "", "", fmt.Errorf("create request failed: %w", err)
	}

	httpReq.Header.Set("Authorization", "Bearer "+s.cfg.APIKey)

	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		return "", "", fmt.Errorf("query task failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", fmt.Errorf("read response failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("query failed, status: %d, body: %s", resp.StatusCode, string(respBody))
	}

	var result taskStatusResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", "", fmt.Errorf("parse response failed: %w", err)
	}

	var resultURL string
	if len(result.Output.Results) > 0 {
		resultURL = result.Output.Results[0].TranscriptionURL
	}

	return result.Output.TaskStatus, resultURL, nil
}

func (s *ASRService) CancelTask(ctx context.Context, taskID string) error {
	cancelURL := fmt.Sprintf("https://dashscope.aliyuncs.com/api/v1/tasks/%s", taskID)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodDelete, cancelURL, nil)
	if err != nil {
		return fmt.Errorf("create request failed: %w", err)
	}

	httpReq.Header.Set("Authorization", "Bearer "+s.cfg.APIKey)

	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("cancel task failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("cancel failed, status: %d, body: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

func (s *ASRService) GetCallbackURL() string {
	return s.cfg.CallbackURL
}
