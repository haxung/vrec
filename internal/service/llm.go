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

	"go.uber.org/zap"
)

type LLMService struct {
	cfg        *config.LLMConfig
	logger     *zap.Logger
	httpClient *http.Client
}

func NewLLMService(cfg *config.LLMConfig, logger *zap.Logger) *LLMService {
	return &LLMService{
		cfg:    cfg,
		logger: logger,
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

type GenerateSummaryRequest struct {
	Text   string
	Model  string
	APIURL string
	APIKey string
}

func (s *LLMService) GenerateMeetingSummary(ctx context.Context, req *GenerateSummaryRequest) (string, error) {
	if !s.cfg.Enabled {
		return "", fmt.Errorf("llm is not enabled")
	}

	model := req.Model
	if model == "" {
		model = s.cfg.Model
	}

	apiURL := req.APIURL
	if apiURL == "" {
		apiURL = s.cfg.APIURL
	}

	apiKey := req.APIKey
	if apiKey == "" {
		apiKey = s.cfg.APIKey
	}

	systemPrompt := s.cfg.MeetingSummaryPrompt
	if systemPrompt == "" {
		systemPrompt = `你是一个专业的会议纪要生成助手。请根据提供的会议转写文本，生成结构化的会议纪要，包括：
1. 会议主题/标题
2. 会议时间（如果提到）
3. 与会人员（如果提到）
4. 会议要点/讨论内容
5. 会议决议/下一步行动

请用中文回复，只输出会议纪要内容，不要有其他解释。`
	}

	temperature := s.cfg.Temperature
	if temperature == 0 {
		temperature = 0.7
	}

	payload := map[string]any{
		"model": model,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": req.Text},
		},
		"temperature": temperature,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal payload failed: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create request failed: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("call llm api failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("llm api failed, status: %d, body: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("parse response failed: %w", err)
	}

	if len(result.Choices) == 0 {
		return "", fmt.Errorf("llm returned no choices")
	}

	s.logger.Info("meeting summary generated successfully")
	return result.Choices[0].Message.Content, nil
}

func (s *LLMService) IsEnabled() bool {
	return s.cfg.Enabled
}
