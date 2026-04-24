//go:build integration

package service

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"
	"testing"

	"vrec/internal/config"

	"go.uber.org/zap"
)

var (
	queryTaskID = flag.String("task_id", "", "ASR Task ID to query")
)

func TestASRStatusAndDownload(t *testing.T) {
	if *asrAPIKey == "" {
		t.Fatal("asr-api-key flag is required")
	}
	if *queryTaskID == "" {
		t.Fatal("task_id flag is required")
	}

	logger, _ := zap.NewDevelopment()
	svc := NewASRService(&config.ASRConfig{
		APIKey: *asrAPIKey,
		Model:  "fun-asr",
	}, logger)

	ctx := context.Background()

	// 1. 查询任务状态
	status, resultURL, err := svc.GetStatus(ctx, *queryTaskID)
	if err != nil {
		t.Fatalf("GetStatus failed: %v", err)
	}

	t.Logf("TaskID: %s, Status: %s", *queryTaskID, status)

	if status == TaskStatusSUCCEEDED && resultURL != "" {
		// 2. 下载转写结果
		t.Logf("Result URL: %s", resultURL)

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, resultURL, nil)
		if err != nil {
			t.Fatalf("create request failed: %v", err)
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("download failed: %v", err)
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("read body failed: %v", err)
		}

		t.Logf("Downloaded %d bytes", len(body))

		// 打印前 500 字符预览
		preview := body
		if len(preview) > 500 {
			preview = preview[:500]
		}
		t.Logf("=== Preview (first 500 chars) ===\n%s", preview)
	} else {
		fmt.Printf("Task status: %s (resultURL: %s)\n", status, resultURL)
	}
}
