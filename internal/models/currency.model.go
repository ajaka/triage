package models

import (
	"time"
)

type Currency struct {
	Id                     int       `json:"id" db:"id" redis:"id"`
	Currency               string    `json:"currency" db:"currency" redis:"currency"`
	Code                   string    `json:"code" db:"code" redis:"code"`
	Active                 bool      `json:"active" db:"active" redis:"active"`
	LastKnownRateToDollars float64   `json:"last_known_rate_to_dollars" db:"last_known_rate_to_dollars" redis:"last_known_rate_to_dollars"`
	CreatedAt              time.Time `json:"created_at" db:"created_at" redis:"created_at"`
	UpdatedAt              time.Time `json:"updated_at" db:"updated_at" redis:"updated_at"`
}
