//go:build integration

package service

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"testing"
	"time"

	"vrec/internal/config"

	"go.uber.org/zap"
)

func TestS3Service_Upload(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	svc := NewS3Service(&config.Config{
		S3: config.S3Config{
			Endpoint:      "http://127.0.0.1:9000",
			AccessKey:     "admin",
			SecretKey:     "admin",
			Bucket:        "vrec",
			Region:        "auto",
			URLExpireDays: 3,
		},
	}, logger)

	ctx := context.Background()
	key := svc.GenerateKeyWithFilename("test", "test.txt")
	content := "0~9A~Za~z"

	result, err := svc.Upload(ctx, key, bytes.NewReader([]byte(content)), int64(len(content)), "text/plain")
	if err != nil {
		t.Fatalf("upload failed: %v", err)
	}

	t.Logf("Key: %s", result.Key)
	t.Logf("Size: %d", result.SizeBytes)
	t.Logf("ContentType: %s", result.ContentType)
	t.Logf("URL: %s", result.URL)
}

func TestS3Service_GeneratePresignedURL_15Seconds(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	svc := NewS3Service(&config.Config{
		S3: config.S3Config{
			Endpoint:      "http://127.0.0.1:9000",
			AccessKey:     "admin",
			SecretKey:     "admin",
			Bucket:        "vrec",
			Region:        "auto",
			URLExpireDays: 3,
		},
	}, logger)

	ctx := context.Background()
	key := svc.GenerateKeyWithFilename("test", "test.txt")
	content := "0~9A~Za~z"

	_, err := svc.Upload(ctx, key, bytes.NewReader([]byte(content)), int64(len(content)), "text/plain")
	if err != nil {
		t.Fatalf("upload failed: %v", err)
	}

	url, expiresAt, err := svc.GeneratePresignedURL(ctx, key, 15*time.Second)
	if err != nil {
		t.Fatalf("generate presigned url failed: %v", err)
	}

	t.Logf("URL: %s", url)
	t.Logf("ExpiresAt: %s", expiresAt)

	// 验证 X-Amz-Expires=15
	if !bytes.Contains([]byte(url), []byte("X-Amz-Expires=15")) {
		t.Errorf("URL should contain X-Amz-Expires=15")
	}
}

func TestS3Service_Download_WithExpiry(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	svc := NewS3Service(&config.Config{
		S3: config.S3Config{
			Endpoint:      "http://127.0.0.1:9000",
			AccessKey:     "admin",
			SecretKey:     "admin",
			Bucket:        "vrec",
			Region:        "auto",
			URLExpireDays: 3,
		},
	}, logger)

	ctx := context.Background()
	key := svc.GenerateKeyWithFilename("test", "test.txt")
	content := "0~9A~Za~z"

	_, err := svc.Upload(ctx, key, bytes.NewReader([]byte(content)), int64(len(content)), "text/plain")
	if err != nil {
		t.Fatalf("upload failed: %v", err)
	}

	// 生成 15s 过期的预签名 URL
	url, expiresAt, err := svc.GeneratePresignedURL(ctx, key, 15*time.Second)
	if err != nil {
		t.Fatalf("generate presigned url failed: %v", err)
	}
	t.Logf("Generated URL with 15s expiry, expires at: %s", expiresAt)

	// 1. 过期前下载（应该成功）
	t.Log("=== Before expiry ===")
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("download before expiry failed: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if string(body) != content {
		t.Errorf("expected content %q, got %q", content, string(body))
	}
	t.Logf("Download before expiry succeeded, content: %s", string(body))

	// 2. 等待 16 秒让 URL 过期
	t.Log("=== Waiting 16 seconds for URL to expire ===")
	time.Sleep(16 * time.Second)

	// 3. 过期后下载（应该失败或返回错误）
	resp, err = http.Get(url)
	if err != nil {
		t.Logf("download after expiry failed (expected): %v", err)
		t.Log("=== PASS: URL expired as expected ===")
		return
	}
	defer resp.Body.Close()

	// MinIO 返回 403 或其他错误表示过期
	if resp.StatusCode >= 400 {
		t.Logf("download after expiry returned status %d (expected for expired URL)", resp.StatusCode)
		t.Log("=== PASS: URL expired as expected ===")
		return
	}

	body, _ = io.ReadAll(resp.Body)
	t.Errorf("download after expiry should have failed, got status %d and body %s", resp.StatusCode, string(body))
}

func TestS3Service_Download_UploadDownload(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	svc := NewS3Service(&config.Config{
		S3: config.S3Config{
			Endpoint:      "http://127.0.0.1:9000",
			AccessKey:     "admin",
			SecretKey:     "admin",
			Bucket:        "vrec",
			Region:        "auto",
			URLExpireDays: 3,
		},
	}, logger)

	ctx := context.Background()
	key := svc.GenerateKeyWithFilename("test", "test.txt")
	content := "Hello S3 Download!"

	// 上传
	_, err := svc.Upload(ctx, key, bytes.NewReader([]byte(content)), int64(len(content)), "text/plain")
	if err != nil {
		t.Fatalf("upload failed: %v", err)
	}

	// 下载
	data, err := svc.Download(ctx, key)
	if err != nil {
		t.Fatalf("download failed: %v", err)
	}

	if string(data) != content {
		t.Errorf("expected content %q, got %q", content, string(data))
	}

	t.Logf("Download succeeded, content: %s", string(data))
}

func TestS3Service_Download_NotFound(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	svc := NewS3Service(&config.Config{
		S3: config.S3Config{
			Endpoint:      "http://127.0.0.1:9000",
			AccessKey:     "admin",
			SecretKey:     "admin",
			Bucket:        "vrec",
			Region:        "auto",
			URLExpireDays: 3,
		},
	}, logger)

	ctx := context.Background()
	_, err := svc.Download(ctx, "nonexistent-key-12345")

	if err == nil {
		t.Error("expected error for nonexistent key, got nil")
	} else {
		t.Logf("Expected error: %v", err)
	}
}
