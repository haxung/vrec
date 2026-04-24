//go:build integration

package service

import (
	"context"
	"flag"
	"os"
	"testing"

	"vrec/internal/config"

	"go.uber.org/zap"
)

var (
	llmAPIKey = flag.String("llm-api-key", "", "LLM API Key")
	llmAPIURL = flag.String("llm-api-url", "", "LLM API URL")
	llmModel  = flag.String("llm-model", "", "LLM Model Name")
)

// TestSubtitleGeneration 测试字幕生成功能
func TestSubtitleGeneration(t *testing.T) {
	asrJSON, err := os.ReadFile("asr_result.json")
	if err != nil {
		t.Fatalf("failed to read asr_result.json: %v", err)
	}

	logger, _ := zap.NewDevelopment()
	cfg := &config.Config{
		Pricing: config.PricingConfig{
			SubtitlePerMinute: 0.01,
		},
	}

	svc := &SubtitleService{cfg: cfg, logger: logger}
	srtContent, err := svc.generateSRTFromASR(string(asrJSON))
	if err != nil {
		t.Fatalf("generateSRTFromASR failed: %v", err)
	}

	t.Logf("SRT content length: %d", len(srtContent))
	t.Log("=== SRT Preview (first 500 chars) ===")
	if len(srtContent) > 500 {
		t.Log(srtContent[:500])
	} else {
		t.Log(srtContent)
	}

	// 统计字幕块数量
	blockCount := 0
	emptyLines := 0
	for _, line := range srtContent {
		if line == '\n' {
			emptyLines++
			if emptyLines == 2 {
				blockCount++
				emptyLines = 0
			}
		} else {
			emptyLines = 0
		}
	}
	t.Logf("=== Total subtitle blocks: %d ===", blockCount)
}

// TestMeetingNoteGeneration 测试会议纪要生成功能
func TestMeetingNoteGeneration(t *testing.T) {
	if *llmAPIKey == "" {
		t.Fatal("llm-api-key flag is required")
	}
	if *llmAPIURL == "" {
		t.Fatal("llm-api-url flag is required")
	}
	if *llmModel == "" {
		t.Fatal("llm-model flag is required")
	}

	asrJSON, err := os.ReadFile("asr_result.json")
	if err != nil {
		t.Fatalf("failed to read asr_result.json: %v", err)
	}

	logger, _ := zap.NewDevelopment()
	cfg := &config.Config{
		Pricing: config.PricingConfig{
			MeetingNotePerToken: 0.001,
		},
		LLM: config.LLMConfig{
			Enabled: true,
			APIURL:  *llmAPIURL,
			APIKey:  *llmAPIKey,
			Model:   *llmModel,
		},
	}

	llmSvc := NewLLMService(&cfg.LLM, logger)

	dialogueText, err := parseTranscriptionAndFormat(string(asrJSON))
	if err != nil {
		t.Fatalf("parseTranscriptionAndFormat failed: %v", err)
	}

	t.Log("=== Dialogue Format Preview (first 1000 chars) ===")
	if len(dialogueText) > 1000 {
		t.Log(dialogueText[:1000])
	} else {
		t.Log(dialogueText)
	}

	ctx := context.Background()
	summaryText, err := llmSvc.GenerateMeetingSummary(ctx, &GenerateSummaryRequest{
		Text: dialogueText,
	})
	if err != nil {
		t.Fatalf("GenerateMeetingSummary failed: %v", err)
	}

	t.Log("=== Meeting Summary ===")
	t.Log(summaryText)
}

// TestParseDialogueWithSpeakers 测试带说话人信息的对话解析
func TestParseDialogueWithSpeakers(t *testing.T) {
	asrJSON, err := os.ReadFile("asr_result.json")
	if err != nil {
		t.Fatalf("failed to read asr_result.json: %v", err)
	}

	dialogueText, err := parseTranscriptionAndFormat(string(asrJSON))
	if err != nil {
		t.Fatalf("parseTranscriptionAndFormat failed: %v", err)
	}

	t.Log("=== Dialogue with Speakers ===")
	t.Log(dialogueText)
}
