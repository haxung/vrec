package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"vrec/internal/model"
)

type UserTokenRepository struct {
	db *pgxpool.Pool
}

func NewUserTokenRepository(db *pgxpool.Pool) *UserTokenRepository {
	return &UserTokenRepository{db: db}
}

func (r *UserTokenRepository) Create(ctx context.Context, token *model.UserToken) error {
	query := `
		INSERT INTO user_tokens (user_id, token, expires_at)
		VALUES ($1, $2, $3)
		RETURNING id, created_at`
	return r.db.QueryRow(ctx, query, token.UserID, token.Token, token.ExpiresAt).
		Scan(&token.ID, &token.CreatedAt)
}

func (r *UserTokenRepository) GetByToken(ctx context.Context, token uuid.UUID) (*model.UserToken, error) {
	query := `
		SELECT id, user_id, token, created_at, expires_at
		FROM user_tokens
		WHERE token = $1 AND expires_at > $2`
	userToken := &model.UserToken{}
	err := r.db.QueryRow(ctx, query, token, time.Now()).
		Scan(&userToken.ID, &userToken.UserID, &userToken.Token, &userToken.CreatedAt, &userToken.ExpiresAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return userToken, err
}

func (r *UserTokenRepository) GetByID(ctx context.Context, id int64) (*model.UserToken, error) {
	query := `
		SELECT id, user_id, token, created_at, expires_at
		FROM user_tokens
		WHERE id = $1`
	userToken := &model.UserToken{}
	err := r.db.QueryRow(ctx, query, id).
		Scan(&userToken.ID, &userToken.UserID, &userToken.Token, &userToken.CreatedAt, &userToken.ExpiresAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return userToken, err
}

func (r *UserTokenRepository) DeleteByToken(ctx context.Context, token uuid.UUID) error {
	query := `DELETE FROM user_tokens WHERE token = $1`
	_, err := r.db.Exec(ctx, query, token)
	return err
}

func (r *UserTokenRepository) DeleteByUserID(ctx context.Context, userID int64) error {
	query := `DELETE FROM user_tokens WHERE user_id = $1`
	_, err := r.db.Exec(ctx, query, userID)
	return err
}

func (r *UserTokenRepository) DeleteExpired(ctx context.Context) error {
	query := `DELETE FROM user_tokens WHERE expires_at < $1`
	_, err := r.db.Exec(ctx, query, time.Now())
	return err
}

func (r *UserTokenRepository) GetByUserID(ctx context.Context, userID int64) ([]*model.UserToken, error) {
	query := `
		SELECT id, user_id, token, created_at, expires_at
		FROM user_tokens
		WHERE user_id = $1 AND expires_at > $2
		ORDER BY created_at DESC`
	rows, err := r.db.Query(ctx, query, userID, time.Now())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tokens []*model.UserToken
	for rows.Next() {
		t := &model.UserToken{}
		if err := rows.Scan(&t.ID, &t.UserID, &t.Token, &t.CreatedAt, &t.ExpiresAt); err != nil {
			return nil, err
		}
		tokens = append(tokens, t)
	}
	return tokens, rows.Err()
}

func (r *UserTokenRepository) DeleteByID(ctx context.Context, id int64) error {
	query := `DELETE FROM user_tokens WHERE id = $1`
	_, err := r.db.Exec(ctx, query, id)
	return err
}
