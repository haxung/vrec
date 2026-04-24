package service

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"vrec/internal/config"

	"go.uber.org/zap"
)

func TestS3Service_GenerateKey(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	svc := &S3Service{
		cfg: &config.S3Config{
			Bucket: "test-bucket",
		},
		logger: logger,
	}

	key := svc.GenerateKeyWithFilename("audio", "test.mp3")
	parts := strings.Split(key, "/")
	if len(parts) != 4 {
		t.Errorf("key should have 4 parts, got %d: %s", len(parts), key)
	}

	// 验证格式: {bucket}/audio/YYMMDD/test-{uuid}.mp3
	bucket := parts[0]
	prefix := parts[1]
	datePart := parts[2]
	filenamePart := parts[3]

	if bucket != "test-bucket" {
		t.Errorf("first part should be bucket name, got %s", bucket)
	}
	if prefix != "audio" {
		t.Errorf("second part should be 'audio', got %s", prefix)
	}
	if len(datePart) != 6 { // YYMMDD
		t.Errorf("third part should be YYMMDD format (6 chars), got %s", datePart)
	}
	if !strings.HasPrefix(filenamePart, "test-") || !strings.HasSuffix(filenamePart, ".mp3") {
		t.Errorf("filename part should be test-{{uuid}}.mp3, got %s", filenamePart)
	}
}

func TestS3Service_GenerateKey_Uniqueness(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	svc := &S3Service{
		cfg: &config.S3Config{
			Bucket: "test",
		},
		logger: logger,
	}

	keys := make(map[string]bool)
	for range 100 {
		key := svc.GenerateKeyWithFilename("test", "audio.mp3")
		if keys[key] {
			t.Errorf("duplicate key generated: %s", key)
		}
		keys[key] = true
	}
}

func TestS3Service_GetURLExpireDays(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	svc := &S3Service{
		cfg: &config.S3Config{
			URLExpireDays: 5,
		},
		logger: logger,
	}

	days := svc.GetURLExpireDays()
	if days != 5 {
		t.Errorf("expected 5, got %d", days)
	}
}

func TestUploadResult_Fields(t *testing.T) {
	now := time.Now()
	result := &UploadResult{
		Key:         "bucket/audio/260425/test-uuid.mp3",
		URL:         "https://example.com/bucket/audio/260425/test-uuid.mp3",
		ExpiresAt:   now,
		SizeBytes:   1024,
		ContentType: "audio/mpeg",
	}

	if result.Key != "bucket/audio/260425/test-uuid.mp3" {
		t.Errorf("expected Key, got %s", result.Key)
	}
	if result.SizeBytes != 1024 {
		t.Errorf("expected SizeBytes 1024, got %d", result.SizeBytes)
	}
	if result.ContentType != "audio/mpeg" {
		t.Errorf("expected ContentType audio/mpeg, got %s", result.ContentType)
	}
}

func TestGeneratePresignedURLByS3Key_EmptyKey(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	svc := &S3Service{
		cfg:    &config.S3Config{},
		logger: logger,
	}

	ctx := context.Background()
	url, err := svc.GeneratePresignedURLByS3Key(ctx, "")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if url != "" {
		t.Errorf("expected empty URL for empty key, got %s", url)
	}
}

func TestS3Service_GenerateKeyWithFilename(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	svc := &S3Service{
		cfg: &config.S3Config{
			Bucket: "test-bucket",
		},
		logger: logger,
	}

	key := svc.GenerateKeyWithFilename("audio", "testRecording.mp3")
	parts := strings.Split(key, "/")
	if len(parts) != 4 {
		t.Errorf("key should have 4 parts, got %d: %s", len(parts), key)
	}

	// 验证格式: {bucket}/audio/YYMMDD/filename-{uuid}.ext
	bucket := parts[0]
	prefix := parts[1]
	datePart := parts[2]
	filenamePart := parts[3]

	if bucket != "test-bucket" {
		t.Errorf("first part should be bucket, got %s", bucket)
	}
	if prefix != "audio" {
		t.Errorf("second part should be 'audio', got %s", prefix)
	}
	if len(datePart) != 6 { // YYMMDD
		t.Errorf("third part should be YYMMDD format (6 chars), got %s", datePart)
	}

	// filenamePart should be: testRecording-{uuid}.mp3
	if !strings.HasPrefix(filenamePart, "testRecording-") {
		t.Errorf("filename part should start with 'testRecording-', got %s", filenamePart)
	}
	if !strings.HasSuffix(filenamePart, ".mp3") {
		t.Errorf("filename part should end with '.mp3', got %s", filenamePart)
	}
	// UUID should contain dashes
	if !strings.Contains(filenamePart, "-") {
		t.Errorf("uuid part should contain dashes, got %s", filenamePart)
	}
}

func TestS3Service_GenerateKeyWithFilename_Sanitize(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	svc := &S3Service{
		cfg: &config.S3Config{
			Bucket: "test-bucket",
		},
		logger: logger,
	}

	tests := []struct {
		name     string
		filename string
		want     string
	}{
		{"spaces", "my audio file.mp3", "my_audio_file"},   // filepath.Base 不处理空格
		{"path", "dir/my audio.wav", "my_audio"},           // filepath.Base("dir/my audio.wav") = "my audio.wav", ext=".wav, nameWithoutExt="my audio"
		{"special_chars", "test:file?.wav", "test_file_"},  // filepath.Base("test:file?.wav") = "test:file?.wav", ext=".wav", nameWithoutExt="test:file?"
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := svc.GenerateKeyWithFilename("audio", tt.filename)
			parts := strings.Split(key, "/")
			filenamePart := parts[3] // {bucket}/audio/{date}/{filename}

			// filenamePart 格式: name-{uuid}.ext
			// 检查文件名部分是否以期望的干净名字开头（后面跟 - 和 uuid）
			if !strings.HasPrefix(filenamePart, tt.want+"-") {
				t.Errorf("expected prefix %s-, got %s", tt.want, filenamePart)
			}
			// 检查扩展名
			ext := filepath.Ext(filenamePart)
			if ext != filepath.Ext(tt.filename) {
				t.Errorf("expected ext %s, got %s", filepath.Ext(tt.filename), ext)
			}
		})
	}
}

func TestS3Service_GenerateKeyWithFilename_Uniqueness(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	svc := &S3Service{
		cfg: &config.S3Config{
			Bucket: "test-bucket",
		},
		logger: logger,
	}

	keys := make(map[string]bool)
	for range 100 {
		key := svc.GenerateKeyWithFilename("audio", "test.mp3")
		if keys[key] {
			t.Errorf("duplicate key generated: %s", key)
		}
		keys[key] = true
	}
}
