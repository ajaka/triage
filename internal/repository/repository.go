package repository

import "github.com/jackc/pgx/v5/pgxpool"

type Repository struct {
	pool *pgxpool.Pool
}

func InitRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}
