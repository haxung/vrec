package repository

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"vrec/internal/model"
)

type UserRechargeRepository struct {
	db *pgxpool.Pool
}

func NewUserRechargeRepository(db *pgxpool.Pool) *UserRechargeRepository {
	return &UserRechargeRepository{db: db}
}

func (r *UserRechargeRepository) Create(ctx context.Context, recharge *model.UserRecharge) error {
	query := `
		INSERT INTO user_recharges (user_id, token_id, amount)
		VALUES ($1, $2, $3)
		RETURNING id, created_at`
	return r.db.QueryRow(ctx, query, recharge.UserID, recharge.TokenID, recharge.Amount).
		Scan(&recharge.ID, &recharge.CreatedAt)
}

func (r *UserRechargeRepository) GetByUserID(ctx context.Context, userID int64, limit, offset int) ([]*model.UserRecharge, error) {
	query := `
		SELECT id, user_id, token_id, amount, created_at
		FROM user_recharges WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`
	rows, err := r.db.Query(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var recharges []*model.UserRecharge
	for rows.Next() {
		recharge := &model.UserRecharge{}
		if err := rows.Scan(&recharge.ID, &recharge.UserID, &recharge.TokenID, &recharge.Amount, &recharge.CreatedAt); err != nil {
			return nil, err
		}
		recharges = append(recharges, recharge)
	}
	return recharges, rows.Err()
}
