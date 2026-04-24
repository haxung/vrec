//go:build integration

package service

import (
	"context"
	"flag"
	"testing"
	"time"

	"vrec/internal/config"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

var (
	asrAPIKey = flag.String("asr-api-key", "", "Alibaba ASR API Key")
	asrModel  = flag.String("asr-model", "fun-asr-2025-11-07", "ASR Model Name")
	audioURL  = flag.String("audio-url", "", "Audio file URL for testing")
)

func TestASRService_Transcribe(t *testing.T) {
	if *asrAPIKey == "" {
		t.Fatal("asr-api-key flag is required")
	}
	if *audioURL == "" {
		panic("audio url is empty")
	}

	logger, _ := zap.NewDevelopment()
	svc := NewASRService(&config.ASRConfig{
		APIKey: *asrAPIKey,
		Model:  *asrModel,
	}, logger)

	ctx := context.Background()
	resp, err := svc.Transcribe(ctx, &TranscribeRequest{
		AudioURL: *audioURL,
		OrderNo:  uuid.New(),
	})
	if err != nil {
		t.Fatalf("Transcribe failed: %v", err)
	}

	t.Logf("TaskID: %s", resp.TaskID)
	t.Logf("Status: %s", resp.Status)
}

func TestASRService_TranscribeAndWait(t *testing.T) {
	if *asrAPIKey == "" {
		t.Fatal("asr-api-key flag is required")
	}
	if *audioURL == "" {
		t.Fatal("audio-url flag is required")
	}

	logger, _ := zap.NewDevelopment()
	svc := NewASRService(&config.ASRConfig{
		APIKey: *asrAPIKey,
		Model:  *asrModel,
	}, logger)

	ctx := context.Background()

	// 1. 提交转写任务
	resp, err := svc.Transcribe(ctx, &TranscribeRequest{
		AudioURL: *audioURL,
		OrderNo:  uuid.New(),
	})
	if err != nil {
		t.Fatalf("Transcribe failed: %v", err)
	}
	t.Logf("TaskID: %s, Status: %s", resp.TaskID, resp.Status)

	// 2. 轮询等待结果
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	maxWait := 10 * time.Minute
	timeout := time.After(maxWait)

	for {
		select {
		case <-timeout:
			t.Fatalf("timeout after %v", maxWait)
		case <-ticker.C:
			status, resultURL, err := svc.GetStatus(ctx, resp.TaskID)
			if err != nil {
				t.Fatalf("GetStatus failed: %v", err)
			}
			t.Logf("TaskID: %s, Status: %s", resp.TaskID, status)

			switch status {
			case TaskStatusSUCCEEDED:
				t.Logf("Result URL: %s", resultURL)
				t.Log("=== SUCCESS ===")
				return
			case TaskStatusFAILED:
				t.Fatalf("ASR task failed")
			case TaskStatusCANCELED:
				t.Fatalf("ASR task canceled")
			}
		}
	}
}
