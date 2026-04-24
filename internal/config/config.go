package config

import (
	"os"

	"github.com/pelletier/go-toml/v2"
)

type Config struct {
	Server  ServerConfig  `toml:"server"`
	DB      DBConfig      `toml:"database"`
	S3      S3Config      `toml:"s3"`
	ASR     ASRConfig     `toml:"asr"`
	LLM     LLMConfig     `toml:"llm"`
	Pricing PricingConfig `toml:"pricing"`
	Logger  LoggerConfig  `toml:"logger"`
	Auth    AuthConfig    `toml:"auth"`
	Input   InputConfig   `toml:"input"`
}

type InputConfig struct {
	Path      string `toml:"path"`
	OutputDir string `toml:"output_dir"`
}

type ServerConfig struct {
	Port string `toml:"port"` // 服务端口，默认 8080
}

type DBConfig struct {
	Host     string `toml:"host"`     // 数据库地址
	Port     string `toml:"port"`     // 数据库端口
	Username string `toml:"username"` // 用户名
	Password string `toml:"password"` // 密码
	Name     string `toml:"name"`     // 数据库名
}

type S3Config struct {
	Endpoint      string `toml:"endpoint"`        // S3 端点（如 MinIO 地址）
	AccessKey     string `toml:"access_key"`      // 访问密钥
	SecretKey     string `toml:"secret_key"`      // 密钥
	Bucket        string `toml:"bucket"`          // Bucket 名称
	Region        string `toml:"region"`          // 区域
	URLExpireDays int    `toml:"url_expire_days"` // 预签名 URL 有效期（天）
}

type ASRConfig struct {
	APIKey      string `toml:"api_key"`      // 阿里云 FunAudio ASR API Key
	CallbackURL string `toml:"callback_url"` // 回调地址（当前通过轮询实现，可不配置）
	Model       string `toml:"model"`        // ASR 模型名称，默认 FunAudioASR
}

type LLMConfig struct {
	Enabled              bool    `toml:"enabled"`               // 是否启用 LLM 功能
	APIURL               string  `toml:"api_url"`              // LLM API 地址
	APIKey               string  `toml:"api_key"`              // LLM API Key
	Model                string  `toml:"model"`                // 模型名称
	MeetingSummaryPrompt string  `toml:"meeting_summary_prompt"` // 会议纪要 prompt 模板
	Temperature          float64 `toml:"temperature"`           // 生成温度，0-2 之间，越高越随机
}

type PricingConfig struct {
	StoragePerGBDay       float64 `toml:"storage_per_gb_day"`      // S3 存储价格（元/GB/天）
	ASRPerMinute          float64 `toml:"asr_per_minute"`          // ASR 转写价格（元/分钟）
	SubtitlePerMinute     float64 `toml:"subtitle_per_minute"`     // 字幕按分钟计费（元/分钟）
	MeetingNotePerToken   float64 `toml:"meeting_note_per_token"`  // 会议纪要按 token 计费（元/千token）
	LowBalanceThreshold   float64 `toml:"low_balance_threshold"`   // 低余额阈值（低于此值 QPS 降至 low_balance_qps，0 表示不启用）
	LowBalanceQPS         int     `toml:"low_balance_qps"`         // 低余额时的 QPS 限制
	LocalStorageThreshold int64   `toml:"local_storage_threshold"` // 本地存储阈值（字节），S3 上传失败且内容小于此值时存储在数据库（默认 1MB）
}

type LoggerConfig struct {
	Level      string `toml:"level"`       // 日志级别：debug/info/warn/error
	Format     string `toml:"format"`      // 日志格式：json/console
	Path       string `toml:"path"`        // 日志文件路径（为空则仅输出到 stderr）
	MaxSize    int    `toml:"max_size"`    // 单个日志文件最大大小（MB），默认 100
	MaxBackups int    `toml:"max_backups"` // 保留的旧日志文件数量，默认 30
	MaxAge     int    `toml:"max_age"`     // 旧日志文件保留天数，默认 7
	Compress   bool   `toml:"compress"`    // 是否压缩旧日志文件，默认 true
}

type AuthConfig struct {
	TokenExpireDays int    `toml:"token_expire_days"` // Token 有效期（天），默认 7 天
	JWTEnabled      bool   `toml:"jwt_enabled"`       // 是否启用 JWT 模式（默认 false）
	JWTSecret       string `toml:"jwt_secret"`        // JWT 签名密钥
	SidSecret       string `toml:"sid_secret"`        // SID 生成密钥
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	setDefaults(&cfg)
	return &cfg, nil
}

func setDefaults(cfg *Config) {
	if cfg.Server.Port == "" {
		cfg.Server.Port = "8080"
	}
	if cfg.DB.Host == "" {
		cfg.DB.Host = "localhost"
	}
	if cfg.DB.Port == "" {
		cfg.DB.Port = "5432"
	}
	if cfg.DB.Username == "" {
		cfg.DB.Username = "postgres"
	}
	if cfg.DB.Name == "" {
		cfg.DB.Name = "vrec"
	}
	if cfg.S3.Bucket == "" {
		cfg.S3.Bucket = "vrec"
	}
	if cfg.S3.URLExpireDays == 0 {
		cfg.S3.URLExpireDays = 3
	}
	if cfg.Pricing.StoragePerGBDay == 0 {
		cfg.Pricing.StoragePerGBDay = 0.01
	}
	if cfg.Pricing.ASRPerMinute == 0 {
		cfg.Pricing.ASRPerMinute = 0.1
	}
	if cfg.Logger.Level == "" {
		cfg.Logger.Level = "info"
	}
	if cfg.Logger.Format == "" {
		cfg.Logger.Format = "json"
	}
	if cfg.Logger.MaxSize == 0 {
		cfg.Logger.MaxSize = 100
	}
	if cfg.Logger.MaxBackups == 0 {
		cfg.Logger.MaxBackups = 30
	}
	if cfg.Logger.MaxAge == 0 {
		cfg.Logger.MaxAge = 7
	}
	if cfg.Auth.TokenExpireDays == 0 {
		cfg.Auth.TokenExpireDays = 7
	}
	if cfg.Auth.SidSecret == "" {
		cfg.Auth.SidSecret = "vrec-default-sid-secret-change-in-production"
	}
	if cfg.Pricing.SubtitlePerMinute == 0 {
		cfg.Pricing.SubtitlePerMinute = 0.01 // 默认 0.01 元/分钟
	}
	if cfg.Pricing.MeetingNotePerToken == 0 {
		cfg.Pricing.MeetingNotePerToken = 0.001 // 默认 0.001 元/千token
	}
	if cfg.Pricing.LowBalanceQPS == 0 {
		cfg.Pricing.LowBalanceQPS = 3 // 默认低余额 QPS 为 3
	}
	if cfg.Pricing.LocalStorageThreshold == 0 {
		cfg.Pricing.LocalStorageThreshold = 1048576 // 默认 1MB
	}
	if cfg.ASR.Model == "" {
		cfg.ASR.Model = "fun-asr" // 默认模型
	}
	if cfg.LLM.MeetingSummaryPrompt == "" {
		cfg.LLM.MeetingSummaryPrompt = "请根据以下会议录音转写文本，提取关键信息，生成会议纪要：\n{{text}}"
	}
	if cfg.LLM.Temperature == 0 {
		cfg.LLM.Temperature = 0.7 // 默认温度
	}
}
