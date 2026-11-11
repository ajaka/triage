package repository

import (
	"context"

	"github.com/ajaka/triage/internal/models"
	"github.com/jackc/pgx/v5/pgxpool"
)

type RepositoryInterface interface {
	GetActiveCurrencies(ctx context.Context) ([]models.Currency, error)
	UpdateCurrencies(ctx context.Context, currencies []models.Currency) error
}

type Repository struct {
	pool *pgxpool.Pool
}

func InitRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}
