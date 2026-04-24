package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"vrec/internal/model"
)

type OrderRepository struct {
	db *pgxpool.Pool
}

func NewOrderRepository(db *pgxpool.Pool) *OrderRepository {
	return &OrderRepository{db: db}
}

func (r *OrderRepository) Create(ctx context.Context, order *model.Order) error {
	query := `
		INSERT INTO orders (
			order_no, user_id, token_id, status, task_id, callback_url,
			original_url, source,
			audio_duration, audio_format, sample_rate, channels, bit_rate, codec,
			s3_key, s3_url, s3_expires_at,
			storage_cost, asr_cost, total_cost,
			need_subtitle, need_meeting_note
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22)
		RETURNING id, created_at, updated_at`
	return r.db.QueryRow(ctx, query,
		order.OrderNo, order.UserID, order.TokenID, order.Status, order.TaskID, order.CallbackURL,
		order.OriginalURL, order.Source,
		order.AudioDuration, order.AudioFormat, order.SampleRate, order.Channels, order.BitRate, order.Codec,
		order.S3Key, order.S3URL, order.S3ExpiresAt,
		order.StorageCost, order.ASRCost, order.TotalCost,
		order.NeedSubtitle, order.NeedMeetingNote,
	).Scan(&order.ID, &order.CreatedAt, &order.UpdatedAt)
}

func (r *OrderRepository) GetByOrderNo(ctx context.Context, orderNo uuid.UUID) (*model.Order, error) {
	query := `
		SELECT id, order_no, user_id, token_id, status, task_id, callback_url,
			original_url, source,
			audio_duration, audio_format, sample_rate, channels, bit_rate, codec,
			s3_key, s3_url, s3_expires_at,
			storage_cost, asr_cost, total_cost, subtitle_cost, meeting_cost,
			need_subtitle, need_meeting_note,
			created_at, updated_at
		FROM orders WHERE order_no = $1`
	order := &model.Order{}
	err := r.db.QueryRow(ctx, query, orderNo).
		Scan(
			&order.ID, &order.OrderNo, &order.UserID, &order.TokenID, &order.Status, &order.TaskID, &order.CallbackURL,
			&order.OriginalURL, &order.Source,
			&order.AudioDuration, &order.AudioFormat, &order.SampleRate, &order.Channels, &order.BitRate, &order.Codec,
			&order.S3Key, &order.S3URL, &order.S3ExpiresAt,
			&order.StorageCost, &order.ASRCost, &order.TotalCost, &order.SubtitleCost, &order.MeetingCost,
			&order.NeedSubtitle, &order.NeedMeetingNote,
			&order.CreatedAt, &order.UpdatedAt,
		)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return order, err
}

func (r *OrderRepository) GetByUserID(ctx context.Context, userID int64, limit int, afterOrderNo string, afterCreatedAt time.Time) ([]*model.Order, error) {
	var query string
	var rows pgx.Rows
	var err error

	if afterOrderNo != "" {
		query = `
			SELECT id, order_no, user_id, token_id, status, task_id, callback_url,
				original_url, source,
				audio_duration, audio_format, sample_rate, channels, bit_rate, codec,
				s3_key, s3_url, s3_expires_at,
				storage_cost, asr_cost, total_cost, subtitle_cost, meeting_cost,
				need_subtitle, need_meeting_note,
				created_at, updated_at
			FROM orders WHERE user_id = $1 AND (created_at, order_no) < ($2, $3)
			ORDER BY created_at DESC, order_no DESC
			LIMIT $4`
		rows, err = r.db.Query(ctx, query, userID, afterCreatedAt, afterOrderNo, limit)
	} else {
		query = `
			SELECT id, order_no, user_id, token_id, status, task_id, callback_url,
				original_url, source,
				audio_duration, audio_format, sample_rate, channels, bit_rate, codec,
				s3_key, s3_url, s3_expires_at,
				storage_cost, asr_cost, total_cost, subtitle_cost, meeting_cost,
				need_subtitle, need_meeting_note,
				created_at, updated_at
			FROM orders WHERE user_id = $1
			ORDER BY created_at DESC, order_no DESC
			LIMIT $2`
		rows, err = r.db.Query(ctx, query, userID, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orders []*model.Order
	for rows.Next() {
		order := &model.Order{}
		if err := rows.Scan(
			&order.ID, &order.OrderNo, &order.UserID, &order.TokenID, &order.Status, &order.TaskID,
			&order.OriginalURL, &order.Source,
			&order.AudioDuration, &order.AudioFormat, &order.SampleRate, &order.Channels, &order.BitRate, &order.Codec,
			&order.S3Key, &order.S3URL, &order.S3ExpiresAt,
			&order.StorageCost, &order.ASRCost, &order.TotalCost, &order.SubtitleCost, &order.MeetingCost,
			&order.NeedSubtitle, &order.NeedMeetingNote,
			&order.CreatedAt, &order.UpdatedAt,
		); err != nil {
			return nil, err
		}
		orders = append(orders, order)
	}
	return orders, rows.Err()
}

func (r *OrderRepository) GetByUserIDAndTimeRange(ctx context.Context, userID int64, startTime, endTime time.Time, limit int, afterOrderNo string, afterCreatedAt time.Time) ([]*model.Order, error) {
	var query string
	var rows pgx.Rows
	var err error

	if afterOrderNo != "" {
		query = `
			SELECT id, order_no, user_id, token_id, status, task_id, callback_url,
				original_url, source,
				audio_duration, audio_format, sample_rate, channels, bit_rate, codec,
				s3_key, s3_url, s3_expires_at,
				storage_cost, asr_cost, total_cost, subtitle_cost, meeting_cost,
				need_subtitle, need_meeting_note,
				created_at, updated_at
			FROM orders WHERE user_id = $1 AND created_at >= $2 AND created_at <= $3 AND (created_at, order_no) < ($4, $5)
			ORDER BY created_at DESC, order_no DESC
			LIMIT $6`
		rows, err = r.db.Query(ctx, query, userID, startTime, endTime, afterCreatedAt, afterOrderNo, limit)
	} else {
		query = `
			SELECT id, order_no, user_id, token_id, status, task_id, callback_url,
				original_url, source,
				audio_duration, audio_format, sample_rate, channels, bit_rate, codec,
				s3_key, s3_url, s3_expires_at,
				storage_cost, asr_cost, total_cost, subtitle_cost, meeting_cost,
				need_subtitle, need_meeting_note,
				created_at, updated_at
			FROM orders WHERE user_id = $1 AND created_at >= $2 AND created_at <= $3
			ORDER BY created_at DESC, order_no DESC
			LIMIT $4`
		rows, err = r.db.Query(ctx, query, userID, startTime, endTime, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orders []*model.Order
	for rows.Next() {
		order := &model.Order{}
		if err := rows.Scan(
			&order.ID, &order.OrderNo, &order.UserID, &order.TokenID, &order.Status, &order.TaskID,
			&order.OriginalURL, &order.Source,
			&order.AudioDuration, &order.AudioFormat, &order.SampleRate, &order.Channels, &order.BitRate, &order.Codec,
			&order.S3Key, &order.S3URL, &order.S3ExpiresAt,
			&order.StorageCost, &order.ASRCost, &order.TotalCost, &order.SubtitleCost, &order.MeetingCost,
			&order.NeedSubtitle, &order.NeedMeetingNote,
			&order.CreatedAt, &order.UpdatedAt,
		); err != nil {
			return nil, err
		}
		orders = append(orders, order)
	}
	return orders, rows.Err()
}

// GetByUserIDAndTimeRangeForBill 查询账单用，不返回大字段
func (r *OrderRepository) GetByUserIDAndTimeRangeForBill(ctx context.Context, userID int64, startTime, endTime time.Time) ([]*model.Order, error) {
	query := `
		SELECT id, order_no, user_id, status,
			storage_cost, asr_cost, total_cost, subtitle_cost, meeting_cost,
			created_at
		FROM orders WHERE user_id = $1 AND created_at >= $2 AND created_at <= $3
		ORDER BY created_at DESC`
	rows, err := r.db.Query(ctx, query, userID, startTime, endTime)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orders []*model.Order
	for rows.Next() {
		order := &model.Order{}
		if err := rows.Scan(
			&order.ID, &order.OrderNo, &order.UserID, &order.Status,
			&order.StorageCost, &order.ASRCost, &order.TotalCost, &order.SubtitleCost, &order.MeetingCost,
			&order.CreatedAt,
		); err != nil {
			return nil, err
		}
		orders = append(orders, order)
	}
	return orders, rows.Err()
}

// CountByUserIDAndTimeRange 统计时间范围内订单数
func (r *OrderRepository) CountByUserIDAndTimeRange(ctx context.Context, userID int64, startTime, endTime time.Time) (int64, error) {
	query := `SELECT COUNT(*) FROM orders WHERE user_id = $1 AND created_at >= $2 AND created_at <= $3`
	var count int64
	err := r.db.QueryRow(ctx, query, userID, startTime, endTime).Scan(&count)
	return count, err
}

func (r *OrderRepository) UpdateStatus(ctx context.Context, orderNo uuid.UUID, status model.OrderStatus) error {
	query := `UPDATE orders SET status = $1 WHERE order_no = $2`
	_, err := r.db.Exec(ctx, query, status, orderNo)
	return err
}

func (r *OrderRepository) UpdateS3Info(ctx context.Context, orderNo uuid.UUID, s3URL, s3Key string, expiresAt *time.Time) error {
	query := `UPDATE orders SET s3_url = $1, s3_key = $2, s3_expires_at = $3 WHERE order_no = $4`
	_, err := r.db.Exec(ctx, query, s3URL, s3Key, expiresAt, orderNo)
	return err
}

func (r *OrderRepository) UpdateTaskID(ctx context.Context, orderNo uuid.UUID, taskID string) error {
	query := `UPDATE orders SET task_id = $1 WHERE order_no = $2`
	_, err := r.db.Exec(ctx, query, taskID, orderNo)
	return err
}

func (r *OrderRepository) GetProcessingOrders(ctx context.Context) ([]*model.Order, error) {
	query := `
		SELECT id, order_no, user_id, token_id, status, task_id, callback_url,
			original_url, source,
			audio_duration, audio_format, sample_rate, channels, bit_rate, codec,
			s3_key, s3_url, s3_expires_at,
			storage_cost, asr_cost, total_cost, subtitle_cost, meeting_cost,
			need_subtitle, need_meeting_note,
			created_at, updated_at
		FROM orders
		WHERE status = 'processing' AND task_id != ''
		ORDER BY created_at ASC
		LIMIT 100`
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orders []*model.Order
	for rows.Next() {
		order := &model.Order{}
		if err := rows.Scan(
			&order.ID, &order.OrderNo, &order.UserID, &order.TokenID, &order.Status, &order.TaskID,
			&order.OriginalURL, &order.Source,
			&order.AudioDuration, &order.AudioFormat, &order.SampleRate, &order.Channels, &order.BitRate, &order.Codec,
			&order.S3Key, &order.S3URL, &order.S3ExpiresAt,
			&order.StorageCost, &order.ASRCost, &order.TotalCost,
			&order.CreatedAt, &order.UpdatedAt,
		); err != nil {
			return nil, err
		}
		orders = append(orders, order)
	}
	return orders, rows.Err()
}

func (r *OrderRepository) CancelOrder(ctx context.Context, orderNo uuid.UUID) error {
	query := `UPDATE orders SET status = 'canceled' WHERE order_no = $1 AND status = 'processing'`
	_, err := r.db.Exec(ctx, query, orderNo)
	return err
}

func (r *OrderRepository) GetInsufficientOrdersByUserID(ctx context.Context, userID int64, limit int, afterOrderNo string, afterCreatedAt time.Time) ([]*model.Order, error) {
	var query string
	var rows pgx.Rows
	var err error

	if afterOrderNo != "" {
		query = `
			SELECT id, order_no, user_id, token_id, status, task_id, callback_url,
				original_url, source,
				audio_duration, audio_format, sample_rate, channels, bit_rate, codec,
				s3_key, s3_url, s3_expires_at,
				storage_cost, asr_cost, total_cost, subtitle_cost, meeting_cost,
				need_subtitle, need_meeting_note,
				created_at, updated_at
			FROM orders WHERE user_id = $1 AND status = 'insufficient' AND (created_at, order_no) < ($2, $3)
			ORDER BY created_at DESC, order_no DESC
			LIMIT $4`
		rows, err = r.db.Query(ctx, query, userID, afterCreatedAt, afterOrderNo, limit)
	} else {
		query = `
			SELECT id, order_no, user_id, token_id, status, task_id, callback_url,
				original_url, source,
				audio_duration, audio_format, sample_rate, channels, bit_rate, codec,
				s3_key, s3_url, s3_expires_at,
				storage_cost, asr_cost, total_cost, subtitle_cost, meeting_cost,
				need_subtitle, need_meeting_note,
				created_at, updated_at
			FROM orders WHERE user_id = $1 AND status = 'insufficient'
			ORDER BY created_at DESC, order_no DESC
			LIMIT $2`
		rows, err = r.db.Query(ctx, query, userID, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orders []*model.Order
	for rows.Next() {
		order := &model.Order{}
		if err := rows.Scan(
			&order.ID, &order.OrderNo, &order.UserID, &order.TokenID, &order.Status, &order.TaskID,
			&order.OriginalURL, &order.Source,
			&order.AudioDuration, &order.AudioFormat, &order.SampleRate, &order.Channels, &order.BitRate, &order.Codec,
			&order.S3Key, &order.S3URL, &order.S3ExpiresAt,
			&order.StorageCost, &order.ASRCost, &order.TotalCost, &order.SubtitleCost, &order.MeetingCost,
			&order.NeedSubtitle, &order.NeedMeetingNote,
			&order.CreatedAt, &order.UpdatedAt,
		); err != nil {
			return nil, err
		}
		orders = append(orders, order)
	}
	return orders, rows.Err()
}

func (r *OrderRepository) ResetInsufficientOrder(ctx context.Context, orderNo uuid.UUID) error {
	query := `UPDATE orders SET status = 'pending', task_id = '' WHERE order_no = $1 AND status = 'insufficient'`
	_, err := r.db.Exec(ctx, query, orderNo)
	return err
}
