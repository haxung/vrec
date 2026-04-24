package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"vrec/internal/model"
)

type MeetingSummaryRepository struct {
	db *pgxpool.Pool
}

func NewMeetingSummaryRepository(db *pgxpool.Pool) *MeetingSummaryRepository {
	return &MeetingSummaryRepository{db: db}
}

func (r *MeetingSummaryRepository) Create(ctx context.Context, summary *model.MeetingSummary) error {
	query := `
		INSERT INTO meeting_summaries (order_no, summary_s3_key, summary_text, cost)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at`
	return r.db.QueryRow(ctx, query, summary.OrderNo, summary.SummaryS3Key, summary.SummaryText, summary.Cost).
		Scan(&summary.ID, &summary.CreatedAt)
}

func (r *MeetingSummaryRepository) GetByOrderNo(ctx context.Context, orderNo uuid.UUID) (*model.MeetingSummary, error) {
	query := `
		SELECT id, order_no, summary_s3_key, summary_text, cost, created_at
		FROM meeting_summaries WHERE order_no = $1`
	summary := &model.MeetingSummary{}
	err := r.db.QueryRow(ctx, query, orderNo).
		Scan(&summary.ID, &summary.OrderNo, &summary.SummaryS3Key, &summary.SummaryText, &summary.Cost, &summary.CreatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return summary, err
}
