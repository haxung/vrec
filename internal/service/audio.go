package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"go.uber.org/zap"
	"vrec/internal/config"
)

type AudioService struct {
	cfg       *config.Config
	logger    *zap.Logger
	ffprobePath string
	ffmpegPath  string
}

func NewAudioService(cfg *config.Config, logger *zap.Logger) *AudioService {
	return &AudioService{
		cfg:        cfg,
		logger:     logger,
		ffprobePath: getEnv("FFPROBE_PATH", "ffprobe"),
		ffmpegPath:  getEnv("FFMPEG_PATH", "ffmpeg"),
	}
}

const (
	MaxAudioSizeBytes = 1 << 30 // 1GB
	MaxAudioDurationSec = 6 * 60 * 60 // 6 hours
)

type AudioInfo struct {
	Duration   int64   `json:"duration"`   // 时长（秒）
	SampleRate int64   `json:"sample_rate"` // 采样率 Hz
	Format     string  `json:"format"`     // 格式 mp3/wav/etc
	BitRate    int64   `json:"bit_rate"`   // 比特率 bps
	Size       int64   `json:"size"`       // 文件大小（字节）
	Channels   int     `json:"channels"`   // 声道数
	Codec      string  `json:"codec"`      // 编解码器
}

func (s *AudioService) ParseFromReader(ctx context.Context, reader io.Reader, filename string) (*AudioInfo, error) {
	// 创建临时文件
	tmpDir := os.TempDir()
	tmpFile, err := os.CreateTemp(tmpDir, "audio-*_"+filepath.Base(filename))
	if err != nil {
		return nil, fmt.Errorf("create temp file failed: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	// 写入文件
	if _, err := io.Copy(tmpFile, reader); err != nil {
		tmpFile.Close()
		return nil, fmt.Errorf("write temp file failed: %w", err)
	}
	tmpFile.Close()

	return s.probeFile(ctx, tmpPath)
}

func (s *AudioService) ParseFromURL(ctx context.Context, urlStr string) (*AudioInfo, io.Reader, error) {
	if !s.IsValidURL(urlStr) {
		return nil, nil, fmt.Errorf("invalid url: %s", urlStr)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlStr, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("create request failed: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("fetch url failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("fetch url failed: status %d", resp.StatusCode)
	}

	// 创建临时文件
	tmpDir := os.TempDir()
	tmpFile, err := os.CreateTemp(tmpDir, "audio-*")
	if err != nil {
		return nil, nil, fmt.Errorf("create temp file failed: %w", err)
	}
	tmpPath := tmpFile.Name()

	// 写入并保持reader可用
	writer := io.MultiWriter(tmpFile)
	reader := io.TeeReader(resp.Body, writer)

	if _, err := io.Copy(tmpFile, resp.Body); err != nil {
		os.Remove(tmpPath)
		tmpFile.Close()
		return nil, nil, fmt.Errorf("write temp file failed: %w", err)
	}
	tmpFile.Close()

	info, err := s.probeFile(ctx, tmpPath)
	if err != nil {
		os.Remove(tmpPath)
		return nil, nil, err
	}

	// 清理临时文件
	go func() {
		os.Remove(tmpPath)
	}()

	return info, reader, nil
}

type ffprobeOutput struct {
	Format struct {
		Filename   string `json:"filename"`
		Duration   string `json:"duration"`
		Size       string `json:"size"`
		BitRate    string `json:"bit_rate"`
		FormatName string `json:"format_name"`
	} `json:"format"`
	Streams []struct {
		CodecType string `json:"codec_type"`
		Channels  int    `json:"channels"`
		SampleRate string `json:"sample_rate"`
		CodecName  string `json:"codec_name"`
	} `json:"streams"`
}

// ProbeFile 探测音频文件信息（导出方法）
func (s *AudioService) ProbeFile(ctx context.Context, path string) (*AudioInfo, error) {
	return s.probeFile(ctx, path)
}

func (s *AudioService) probeFile(ctx context.Context, path string) (*AudioInfo, error) {
	args := []string{
		"-v", "quiet",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		path,
	}

	cmd := exec.CommandContext(ctx, s.ffprobePath, args...)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ffprobe failed: %w", err)
	}

	var result ffprobeOutput
	if err := json.Unmarshal(output, &result); err != nil {
		return nil, fmt.Errorf("parse ffprobe output failed: %w", err)
	}

	info := &AudioInfo{
		Format: result.Format.FormatName,
	}

	if result.Format.Duration != "" {
		if duration, err := strconv.ParseFloat(result.Format.Duration, 64); err == nil {
			info.Duration = int64(duration)
		}
	}

	if result.Format.Size != "" {
		if size, err := strconv.ParseInt(result.Format.Size, 10, 64); err == nil {
			info.Size = size
		}
	}

	if result.Format.BitRate != "" {
		if bitrate, err := strconv.ParseInt(result.Format.BitRate, 10, 64); err == nil {
			info.BitRate = bitrate
		}
	}

	// 查找音频流
	for _, stream := range result.Streams {
		if stream.CodecType == "audio" {
			if stream.SampleRate != "" {
				if sampleRate, err := strconv.ParseInt(stream.SampleRate, 10, 64); err == nil {
					info.SampleRate = sampleRate
				}
			}
			info.Channels = stream.Channels
			info.Codec = stream.CodecName
			break
		}
	}

	s.logger.Info("audio probed",
		zap.String("path", path),
		zap.Int64("duration", info.Duration),
		zap.Int64("sample_rate", info.SampleRate),
		zap.String("format", info.Format),
	)

	return info, nil
}

// ValidateAudio 检查音频文件和时长限制
func (s *AudioService) ValidateAudio(info *AudioInfo) error {
	if info.Size > MaxAudioSizeBytes {
		return fmt.Errorf("audio size %d bytes exceeds limit %d bytes", info.Size, MaxAudioSizeBytes)
	}
	if info.Duration > MaxAudioDurationSec {
		return fmt.Errorf("audio duration %d seconds exceeds limit %d seconds", info.Duration, MaxAudioDurationSec)
	}
	return nil
}

func (s *AudioService) DownloadToTemp(ctx context.Context, urlStr string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlStr, nil)
	if err != nil {
		return "", err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("http status: %d", resp.StatusCode)
	}

	tmpFile, err := os.CreateTemp(os.TempDir(), "audio-download-*")
	if err != nil {
		return "", err
	}
	tmpPath := tmpFile.Name()

	if _, err := io.Copy(tmpFile, resp.Body); err != nil {
		os.Remove(tmpPath)
		return "", err
	}
	tmpFile.Close()

	return tmpPath, nil
}

func (s *AudioService) CleanupTemp(path string) error {
	if path != "" {
		return os.Remove(path)
	}
	return nil
}

func (s *AudioService) DetectFormat(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".mp3":
		return "mp3"
	case ".wav":
		return "wav"
	case ".m4a":
		return "m4a"
	case ".ogg":
		return "ogg"
	case ".flac":
		return "flac"
	case ".aac":
		return "aac"
	case ".wma":
		return "wma"
	default:
		return strings.TrimPrefix(ext, ".")
	}
}

func (s *AudioService) IsValidURL(urlStr string) bool {
	u, err := url.Parse(urlStr)
	if err != nil {
		return false
	}
	return u.Scheme == "http" || u.Scheme == "https"
}

func (s *AudioService) GetFFmpegPath() string {
	return s.ffmpegPath
}

func (s *AudioService) GetFFprobePath() string {
	return s.ffprobePath
}

func getEnv(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}
