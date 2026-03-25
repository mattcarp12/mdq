package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mattcarp12/mdq/internal/models"
)

type UserRepository interface {
	GetUserByEmail(ctx context.Context, email string) (models.User, error)
}

type postgresUserRepo struct {
	db *pgxpool.Pool
}

func NewPostgresUserRepository(db *pgxpool.Pool) UserRepository {
	return &postgresUserRepo{db: db}
}

func (r *postgresUserRepo) GetUserByEmail(ctx context.Context, email string) (models.User, error) {
	var user models.User
	query := `SELECT id, email, password_hash FROM users WHERE email = $1`

	err := r.db.QueryRow(ctx, query, email).Scan(&user.ID, &user.Email, &user.PasswordHash)
	if err != nil {
		if err == pgx.ErrNoRows {
			return models.User{}, fmt.Errorf("user not found")
		}
		return models.User{}, fmt.Errorf("database error: %w", err)
	}

	return user, nil
}