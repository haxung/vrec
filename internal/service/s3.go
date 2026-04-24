package service

import (
	"context"
	"fmt"
	"io"
	"mime"
	"path/filepath"
	"strings"
	"time"

	"vrec/internal/config"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type S3Service struct {
	cfg       *config.S3Config
	logger    *zap.Logger
	s3Client  *s3.Client
	presigner *s3.PresignClient
}

func NewS3Service(cfg *config.Config, logger *zap.Logger) *S3Service {
	awsCfg := aws.Config{
		Region: cfg.S3.Region,
		Credentials: credentials.NewStaticCredentialsProvider(
			cfg.S3.AccessKey,
			cfg.S3.SecretKey,
			"",
		),
	}

	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(cfg.S3.Endpoint)
		o.UsePathStyle = true
	})

	svc := &S3Service{
		cfg:       &cfg.S3,
		logger:    logger,
		s3Client:  client,
		presigner: s3.NewPresignClient(client),
	}

	// 确保 bucket 存在，不存在则创建
	if err := svc.EnsureBucketExists(context.Background()); err != nil {
		panic("failed to ensure bucket exists: " + err.Error())
	}

	return svc
}

// EnsureBucketExists 检查 bucket 是否存在，不存在则创建
func (s *S3Service) EnsureBucketExists(ctx context.Context) error {
	_, err := s.s3Client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(s.cfg.Bucket),
	})
	if err == nil {
		s.logger.Info("bucket exists",
			zap.String("bucket", s.cfg.Bucket),
		)
		return nil
	}

	// bucket 不存在，创建它
	s.logger.Info("creating bucket",
		zap.String("bucket", s.cfg.Bucket),
	)
	_, err = s.s3Client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String(s.cfg.Bucket),
	})
	if err != nil {
		return fmt.Errorf("create bucket failed: %w", err)
	}

	s.logger.Info("bucket created",
		zap.String("bucket", s.cfg.Bucket),
	)
	return nil
}

type UploadResult struct {
	Key         string
	URL         string
	ExpiresAt   time.Time
	SizeBytes   int64
	ContentType string
}

// Upload 上传文件到 S3，返回预签名 URL
// key 格式: {bucket}/{prefix}/{YYMMDD}/{filename}-{uuid}.{ext}
// contentType: MIME 类型，如 "audio/mpeg"，为空则根据文件名扩展名自动推断
func (s *S3Service) Upload(ctx context.Context, key string, body io.Reader, size int64, contentType string) (*UploadResult, error) {
	// 自动推断 contentType
	if contentType == "" {
		contentType = mime.TypeByExtension(filepath.Ext(key))
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	input := &s3.PutObjectInput{
		Bucket:             aws.String(s.cfg.Bucket),
		Key:                aws.String(key),
		Body:               body,
		ContentType:        aws.String(contentType),
		ContentDisposition: aws.String("inline"), // 浏览器预览而非下载
	}

	_, err := s.s3Client.PutObject(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("s3 upload failed: %w", err)
	}

	expiresAt := time.Now().Add(time.Duration(s.cfg.URLExpireDays) * 24 * time.Hour)
	presignResult, err := s.presigner.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.cfg.Bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(time.Until(expiresAt)))
	if err != nil {
		return nil, fmt.Errorf("generate presigned url failed: %w", err)
	}

	s.logger.Info("s3 upload success",
		zap.String("key", key),
		zap.Int64("size", size),
		zap.String("content_type", contentType),
	)

	return &UploadResult{
		Key:         key,
		URL:         presignResult.URL,
		ExpiresAt:   expiresAt,
		SizeBytes:   size,
		ContentType: contentType,
	}, nil
}

// GeneratePresignedURL 生成预签名 URL
func (s *S3Service) GeneratePresignedURL(ctx context.Context, key string, expires time.Duration) (string, time.Time, error) {
	expiresAt := time.Now().Add(expires)
	presignResult, err := s.presigner.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.cfg.Bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(expires))
	if err != nil {
		return "", time.Time{}, fmt.Errorf("generate presigned url failed: %w", err)
	}
	return presignResult.URL, expiresAt, nil
}

// GeneratePresignedURLByS3Key 根据 S3 key 生成预签名 URL（默认3天有效期）
func (s *S3Service) GeneratePresignedURLByS3Key(ctx context.Context, key string) (string, error) {
	if key == "" {
		return "", nil
	}
	url, _, err := s.GeneratePresignedURL(ctx, key, time.Duration(s.cfg.URLExpireDays)*24*time.Hour)
	return url, err
}

// Download 根据 S3 key 下载文件内容（内部调用）
func (s *S3Service) Download(ctx context.Context, key string) ([]byte, error) {
	result, err := s.s3Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.cfg.Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("s3 download failed: %w", err)
	}
	defer result.Body.Close()

	data, err := io.ReadAll(result.Body)
	if err != nil {
		return nil, fmt.Errorf("read s3 response failed: %w", err)
	}

	s.logger.Debug("s3 download success",
		zap.String("key", key),
		zap.Int("size", len(data)),
	)

	return data, nil
}

// Delete 删除 S3 对象
func (s *S3Service) Delete(ctx context.Context, key string) error {
	_, err := s.s3Client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.cfg.Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("s3 delete failed: %w", err)
	}
	s.logger.Info("s3 delete success", zap.String("key", key))
	return nil
}

// GenerateKeyWithFilename 生成包含 bucket 前缀和原始文件名的 S3 key
// key 格式: {bucket}/{prefix}/{YYMMDD}/{filename}-{uuid}.{ext}
func (s *S3Service) GenerateKeyWithFilename(prefix string, filename string) string {
	now := time.Now()
	// 提取文件名（去掉路径）和扩展名
	baseName := filepath.Base(filename)
	ext := filepath.Ext(baseName)
	nameWithoutExt := baseName[:len(baseName)-len(ext)]
	// 清理文件名中的非法字符
	cleanName := sanitizeFilename(nameWithoutExt)
	return fmt.Sprintf("%s/%s/%s/%s-%s%s", s.cfg.Bucket, prefix, now.Format("20060102")[2:], cleanName, uuid.New().String(), ext)
}

// sanitizeFilename 清理文件名中的非法字符
func sanitizeFilename(name string) string {
	// 替换非法字符为下划线
	replacer := strings.NewReplacer(
		"/", "_",
		"\\", "_",
		":", "_",
		"*", "_",
		"?", "_",
		"\"", "_",
		"<", "_",
		">", "_",
		"|", "_",
		" ", "_",
	)
	return replacer.Replace(name)
}

// GetURLExpireDays 返回 URL 过期天数
func (s *S3Service) GetURLExpireDays() int {
	return s.cfg.URLExpireDays
}
