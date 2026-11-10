package repository

import (
	"context"
	"time"

	"github.com/ajaka/triage/internal/models"
	"github.com/jackc/pgx/v5"
)

func (r *Repository) GetActiveCurrencies(ctx context.Context) ([]models.Currency, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	query := `
	SELECT id, code, last_known_rate_to_dollars
	FROM currencies
	WHERE active = true
	`
	rows, err := r.pool.Query(ctx, query)

	if err != nil {
		return nil, err
	}

	currencies, err := pgx.CollectRows(rows, pgx.RowToStructByNameLax[models.Currency])
	return currencies, err
}

func (r *Repository) UpdateCurrencies(ctx context.Context, currencies []models.Currency) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	batch := pgx.Batch{}

	for _, c := range currencies {
		batch.Queue(`
			UPDATE currencies
		 	SET last_known_rate_to_dollars = $1
			WHERE id = $2
			`, c.LastKnownRateToDollars, c.Id)
	}

	resp := r.pool.SendBatch(ctx, &batch)
	return resp.Close()
}
