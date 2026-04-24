package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"vrec/internal/model"
)

type TranscriptionResultRepository struct {
	db *pgxpool.Pool
}

func NewTranscriptionResultRepository(db *pgxpool.Pool) *TranscriptionResultRepository {
	return &TranscriptionResultRepository{db: db}
}

func (r *TranscriptionResultRepository) Create(ctx context.Context, result *model.TranscriptionResult) error {
	query := `
		INSERT INTO transcription_results (order_no, result_s3_key, result_text, subtitle_s3_key)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at`
	return r.db.QueryRow(ctx, query, result.OrderNo, result.ResultS3Key, result.ResultText, result.SubtitleS3Key).
		Scan(&result.ID, &result.CreatedAt)
}

func (r *TranscriptionResultRepository) GetByOrderNo(ctx context.Context, orderNo uuid.UUID) (*model.TranscriptionResult, error) {
	query := `
		SELECT id, order_no, result_s3_key, result_text, subtitle_s3_key, created_at
		FROM transcription_results WHERE order_no = $1`
	result := &model.TranscriptionResult{}
	err := r.db.QueryRow(ctx, query, orderNo).
		Scan(&result.ID, &result.OrderNo, &result.ResultS3Key, &result.ResultText, &result.SubtitleS3Key, &result.CreatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return result, err
}

func (r *TranscriptionResultRepository) UpdateSubtitle(ctx context.Context, orderNo uuid.UUID, subtitleS3Key string) error {
	query := `UPDATE transcription_results SET subtitle_s3_key = $1 WHERE order_no = $2`
	_, err := r.db.Exec(ctx, query, subtitleS3Key, orderNo)
	return err
}
