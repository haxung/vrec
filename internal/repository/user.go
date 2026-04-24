package repository

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"vrec/internal/model"
)

type UserRepository struct {
	db *pgxpool.Pool
}

func NewUserRepository(db *pgxpool.Pool) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) Create(ctx context.Context, user *model.User) error {
	query := `
		INSERT INTO users (username, password, balance, qps_limit)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at, updated_at`
	return r.db.QueryRow(ctx, query, user.Username, user.Password, user.Balance, user.QPSLimit).
		Scan(&user.ID, &user.CreatedAt, &user.UpdatedAt)
}

func (r *UserRepository) GetByUsername(ctx context.Context, username string) (*model.User, error) {
	query := `
		SELECT id, username, password, balance, qps_limit, created_at, updated_at
		FROM users WHERE username = $1`
	user := &model.User{}
	err := r.db.QueryRow(ctx, query, username).
		Scan(&user.ID, &user.Username, &user.Password, &user.Balance, &user.QPSLimit, &user.CreatedAt, &user.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return user, err
}

func (r *UserRepository) GetByID(ctx context.Context, id int64) (*model.User, error) {
	query := `
		SELECT id, username, password, balance, qps_limit, created_at, updated_at
		FROM users WHERE id = $1`
	user := &model.User{}
	err := r.db.QueryRow(ctx, query, id).
		Scan(&user.ID, &user.Username, &user.Password, &user.Balance, &user.QPSLimit, &user.CreatedAt, &user.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return user, err
}

func (r *UserRepository) UpdateBalance(ctx context.Context, id int64, balance string) error {
	query := `UPDATE users SET balance = $1 WHERE id = $2`
	_, err := r.db.Exec(ctx, query, balance, id)
	return err
}
