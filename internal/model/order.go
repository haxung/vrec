package model

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type OrderStatus string

const (
	OrderStatusPending       OrderStatus = "pending"
	OrderStatusProcessing   OrderStatus = "processing"
	OrderStatusSuccess      OrderStatus = "success"
	OrderStatusFailed      OrderStatus = "failed"
	OrderStatusCanceled    OrderStatus = "canceled"
	OrderStatusExpired     OrderStatus = "expired"
	OrderStatusInsufficient OrderStatus = "insufficient" // 余额不足
)

type OrderSource string

const (
	OrderSourceLocal   OrderSource = "local"
	OrderSourceRemote  OrderSource = "remote"
	OrderSourceStream  OrderSource = "stream"
)

type Order struct {
	ID            int64           `json:"id"`
	OrderNo       uuid.UUID       `json:"order_no"`
	UserID        int64           `json:"user_id"`
	TokenID       int64           `json:"token_id"`
	Status        OrderStatus     `json:"status"`
	TaskID        string          `json:"task_id,omitempty"`
	CallbackURL   string          `json:"callback_url,omitempty"`

	// 原始信息
	OriginalURL   string         `json:"original_url"`
	Source        OrderSource    `json:"source"`

	// 音频元数据
	AudioDuration int64          `json:"audio_duration"`
	AudioFormat   string         `json:"audio_format"`
	SampleRate    int64          `json:"sample_rate"`
	Channels      int            `json:"channels"`
	BitRate       int64          `json:"bit_rate"`
	Codec         string         `json:"codec"`

	// S3 存储
	S3Key         string         `json:"s3_key"`
	S3URL         string         `json:"s3_url"`
	S3ExpiresAt   *time.Time    `json:"s3_expires_at,omitempty"`

	// 费用
	StorageCost    decimal.Decimal `json:"storage_cost"`
	ASRCost        decimal.Decimal `json:"asr_cost"`
	SubtitleCost   decimal.Decimal `json:"subtitle_cost,omitempty"`
	MeetingCost    decimal.Decimal `json:"meeting_cost,omitempty"`
	TotalCost      decimal.Decimal `json:"total_cost"`

	// 可选功能
	NeedSubtitle   bool           `json:"need_subtitle,omitempty"`
	NeedMeetingNote bool          `json:"need_meeting_note,omitempty"`

	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
}

type TranscriptionResult struct {
	ID            int64     `json:"id"`
	OrderNo       uuid.UUID `json:"order_no"`
	ResultS3Key   string    `json:"result_s3_key,omitempty"` // S3 存储 key（转写结果）
	ResultText    string    `json:"result_text"`       // ASR 原始 JSON 结果（小于阈值时直接存储）
	SubtitleS3Key string    `json:"subtitle_s3_key,omitempty"` // 字幕 S3 key
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// TranscriptionResponse ASR 转写结果（与阿里云 FunASR 格式一致）
type TranscriptionResponse struct {
	FileURL     string            `json:"file_url"`
	Properties  AudioProperties   `json:"properties"`
	Transcripts []Transcript      `json:"transcripts"`
}

type AudioProperties struct {
	AudioFormat                   string  `json:"audio_format"`
	Channels                     []int  `json:"channels"`
	OriginalSamplingRate          int64  `json:"original_sampling_rate"`
	OriginalDurationMilliseconds int64  `json:"original_duration_in_milliseconds"`
}

type Transcript struct {
	ChannelID                      int       `json:"channel_id"`
	ContentDurationMilliseconds    int64     `json:"content_duration_in_milliseconds"`
	Text                           string    `json:"text"`
	Sentences                      []Sentence `json:"sentences"`
}

type Sentence struct {
	BeginTime  int64   `json:"begin_time"`
	EndTime    int64   `json:"end_time"`
	Text       string  `json:"text"`
	SentenceID int64   `json:"sentence_id"`
	SpeakerID  int64   `json:"speaker_id,omitempty"`
	Words      []Word  `json:"words,omitempty"`
	Punctuation string `json:"punctuation,omitempty"`
}

type Word struct {
	BeginTime  int64  `json:"begin_time"`
	EndTime    int64  `json:"end_time"`
	Text       string `json:"text"`
	Punctuation string `json:"punctuation,omitempty"`
}

type MeetingSummary struct {
	ID           int64           `json:"id"`
	OrderNo      uuid.UUID       `json:"order_no"`
	SummaryS3Key string          `json:"summary_s3_key,omitempty"` // S3 key
	SummaryText  string          `json:"summary_text"`
	Cost         decimal.Decimal `json:"cost"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
}
