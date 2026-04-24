package service

import (
	"testing"

	"vrec/internal/config"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
)

func TestOrderService_CalculateCosts(t *testing.T) {
	cfg := &config.Config{
		Pricing: config.PricingConfig{
			StoragePerGBDay:     0.01,
			ASRPerMinute:        0.1,
			SubtitlePerMinute:   0.1,
			MeetingNotePerToken: 0.1,
		},
	}

	svc := &OrderService{cfg: cfg}

	tests := []struct {
		name            string
		duration        int64
		sizeBytes       int64
		wantSubtitle    bool
		wantMeetingNote bool
		wantStorage     decimal.Decimal
		wantASR         decimal.Decimal
		wantTotal       decimal.Decimal
	}{
		{
			name:        "1分钟音频, 1MB",
			duration:    60,
			sizeBytes:   1024 * 1024,
			wantStorage: decimal.NewFromFloat(0.01 / 1024 * 3), // ~0.00003
			wantASR:     decimal.NewFromFloat(0.1),
			wantTotal:   decimal.NewFromFloat(0.10003),
		},
		{
			name:        "5分钟音频, 5MB",
			duration:    300,
			sizeBytes:   5 * 1024 * 1024,
			wantStorage: decimal.NewFromFloat(5.0 * 0.01 / 1024 * 3), // ~0.00015
			wantASR:     decimal.NewFromFloat(0.5),
			wantTotal:   decimal.NewFromFloat(0.50015),
		},
		{
			name:        "0秒音频",
			duration:    0,
			sizeBytes:   1024 * 1024,
			wantStorage: decimal.NewFromFloat(0.01 / 1024 * 3),
			wantASR:     decimal.NewFromInt(0),
			wantTotal:   decimal.NewFromFloat(0.01 / 1024 * 3),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage, asr, subtitle, meeting, total := svc.CalculateCosts(tt.duration, tt.sizeBytes, tt.wantSubtitle, tt.wantMeetingNote)

			assert.True(t, tt.wantStorage.Sub(storage).Abs().LessThan(decimal.NewFromFloat(0.00001)),
				"storage cost: got %s, want %s", storage, tt.wantStorage)
			assert.True(t, tt.wantASR.Sub(asr).Abs().LessThan(decimal.NewFromFloat(0.00001)),
				"asr cost: got %s, want %s", asr, tt.wantASR)
			assert.True(t, tt.wantTotal.Sub(total).Abs().LessThan(decimal.NewFromFloat(0.00001)),
				"total cost: got %s, want %s", total, tt.wantTotal)

			// subtitle and meeting costs should be zero for these tests
			assert.True(t, subtitle.IsZero(), "subtitle cost should be zero")
			assert.True(t, meeting.IsZero(), "meeting cost should be zero")
		})
	}
}
