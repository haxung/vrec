package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"vrec/internal/model"
)

type RechargeOrderRepository struct {
	db *pgxpool.Pool
}

func NewRechargeOrderRepository(db *pgxpool.Pool) *RechargeOrderRepository {
	return &RechargeOrderRepository{db: db}
}

func (r *RechargeOrderRepository) Create(ctx context.Context, order *model.RechargeOrder) error {
	query := `
		INSERT INTO recharge_orders (recharge_no, user_id, token_id, amount, pay_channel, status, pay_url, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, created_at, updated_at`
	return r.db.QueryRow(ctx, query,
		order.RechargeNo, order.UserID, order.TokenID, order.Amount, order.PayChannel, order.Status, order.PayURL, order.ExpiresAt,
	).Scan(&order.ID, &order.CreatedAt, &order.UpdatedAt)
}

func (r *RechargeOrderRepository) GetByRechargeNo(ctx context.Context, rechargeNo uuid.UUID) (*model.RechargeOrder, error) {
	query := `
		SELECT id, recharge_no, user_id, token_id, amount, pay_channel, status, trade_no, pay_url, expires_at, paid_at, created_at, updated_at
		FROM recharge_orders WHERE recharge_no = $1`
	order := &model.RechargeOrder{}
	err := r.db.QueryRow(ctx, query, rechargeNo).
		Scan(&order.ID, &order.RechargeNo, &order.UserID, &order.TokenID, &order.Amount, &order.PayChannel,
			&order.Status, &order.TradeNo, &order.PayURL, &order.ExpiresAt, &order.PaidAt, &order.CreatedAt, &order.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return order, err
}

func (r *RechargeOrderRepository) GetByTradeNo(ctx context.Context, tradeNo string) (*model.RechargeOrder, error) {
	query := `
		SELECT id, recharge_no, user_id, token_id, amount, pay_channel, status, trade_no, pay_url, expires_at, paid_at, created_at, updated_at
		FROM recharge_orders WHERE trade_no = $1`
	order := &model.RechargeOrder{}
	err := r.db.QueryRow(ctx, query, tradeNo).
		Scan(&order.ID, &order.RechargeNo, &order.UserID, &order.TokenID, &order.Amount, &order.PayChannel,
			&order.Status, &order.TradeNo, &order.PayURL, &order.ExpiresAt, &order.PaidAt, &order.CreatedAt, &order.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return order, err
}

func (r *RechargeOrderRepository) UpdateStatus(ctx context.Context, rechargeNo uuid.UUID, status model.RechargeStatus, tradeNo string) error {
	var paidAt *time.Time
	if status == model.RechargeStatusPaid {
		now := time.Now()
		paidAt = &now
	}
	query := `UPDATE recharge_orders SET status = $1, trade_no = $2, paid_at = $3 WHERE recharge_no = $4`
	_, err := r.db.Exec(ctx, query, status, tradeNo, paidAt, rechargeNo)
	return err
}

func (r *RechargeOrderRepository) GetByUserID(ctx context.Context, userID int64, limit int, afterOrderNo string, afterCreatedAt time.Time) ([]*model.RechargeOrder, error) {
	var query string
	var rows pgx.Rows
	var err error

	if afterOrderNo != "" {
		query = `
			SELECT id, recharge_no, user_id, token_id, amount, pay_channel, status, trade_no, pay_url, expires_at, paid_at, created_at, updated_at
			FROM recharge_orders WHERE user_id = $1 AND (created_at, recharge_no) < ($2, $3)
			ORDER BY created_at DESC, recharge_no DESC
			LIMIT $4`
		rows, err = r.db.Query(ctx, query, userID, afterCreatedAt, afterOrderNo, limit)
	} else {
		query = `
			SELECT id, recharge_no, user_id, token_id, amount, pay_channel, status, trade_no, pay_url, expires_at, paid_at, created_at, updated_at
			FROM recharge_orders WHERE user_id = $1
			ORDER BY created_at DESC, recharge_no DESC
			LIMIT $2`
		rows, err = r.db.Query(ctx, query, userID, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orders []*model.RechargeOrder
	for rows.Next() {
		order := &model.RechargeOrder{}
		if err := rows.Scan(&order.ID, &order.RechargeNo, &order.UserID, &order.TokenID, &order.Amount, &order.PayChannel,
			&order.Status, &order.TradeNo, &order.PayURL, &order.ExpiresAt, &order.PaidAt, &order.CreatedAt, &order.UpdatedAt); err != nil {
			return nil, err
		}
		orders = append(orders, order)
	}
	return orders, rows.Err()
}

func (r *RechargeOrderRepository) GetByUserIDAndTimeRange(ctx context.Context, userID int64, startTime, endTime time.Time, limit int, afterOrderNo string, afterCreatedAt time.Time) ([]*model.RechargeOrder, error) {
	var query string
	var rows pgx.Rows
	var err error

	if afterOrderNo != "" {
		query = `
			SELECT id, recharge_no, user_id, token_id, amount, pay_channel, status, trade_no, pay_url, expires_at, paid_at, created_at, updated_at
			FROM recharge_orders WHERE user_id = $1 AND created_at >= $2 AND created_at <= $3 AND (created_at, recharge_no) < ($4, $5)
			ORDER BY created_at DESC, recharge_no DESC
			LIMIT $6`
		rows, err = r.db.Query(ctx, query, userID, startTime, endTime, afterCreatedAt, afterOrderNo, limit)
	} else {
		query = `
			SELECT id, recharge_no, user_id, token_id, amount, pay_channel, status, trade_no, pay_url, expires_at, paid_at, created_at, updated_at
			FROM recharge_orders WHERE user_id = $1 AND created_at >= $2 AND created_at <= $3
			ORDER BY created_at DESC, recharge_no DESC
			LIMIT $4`
		rows, err = r.db.Query(ctx, query, userID, startTime, endTime, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orders []*model.RechargeOrder
	for rows.Next() {
		order := &model.RechargeOrder{}
		if err := rows.Scan(&order.ID, &order.RechargeNo, &order.UserID, &order.TokenID, &order.Amount, &order.PayChannel,
			&order.Status, &order.TradeNo, &order.PayURL, &order.ExpiresAt, &order.PaidAt, &order.CreatedAt, &order.UpdatedAt); err != nil {
			return nil, err
		}
		orders = append(orders, order)
	}
	return orders, rows.Err()
}

func (r *RechargeOrderRepository) UpdateExpiredOrders(ctx context.Context) (int64, error) {
	query := `UPDATE recharge_orders SET status = 'expired' WHERE status = 'pending' AND expires_at < NOW()`
	result, err := r.db.Exec(ctx, query)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected(), nil
}

// GetByUserIDAndTimeRangeForBill 查询账单用，不返回大字段
func (r *RechargeOrderRepository) GetByUserIDAndTimeRangeForBill(ctx context.Context, userID int64, startTime, endTime time.Time) ([]*model.RechargeOrder, error) {
	query := `
		SELECT id, recharge_no, user_id, amount, pay_channel, status, created_at
		FROM recharge_orders WHERE user_id = $1 AND created_at >= $2 AND created_at <= $3
		ORDER BY created_at DESC`
	rows, err := r.db.Query(ctx, query, userID, startTime, endTime)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orders []*model.RechargeOrder
	for rows.Next() {
		order := &model.RechargeOrder{}
		if err := rows.Scan(&order.ID, &order.RechargeNo, &order.UserID, &order.Amount, &order.PayChannel, &order.Status, &order.CreatedAt); err != nil {
			return nil, err
		}
		orders = append(orders, order)
	}
	return orders, rows.Err()
}

// CountByUserIDAndTimeRange 统计时间范围内充值订单数
func (r *RechargeOrderRepository) CountByUserIDAndTimeRange(ctx context.Context, userID int64, startTime, endTime time.Time) (int64, error) {
	query := `SELECT COUNT(*) FROM recharge_orders WHERE user_id = $1 AND created_at >= $2 AND created_at <= $3`
	var count int64
	err := r.db.QueryRow(ctx, query, userID, startTime, endTime).Scan(&count)
	return count, err
}
