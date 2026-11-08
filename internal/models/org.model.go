package models

import (
	"time"

	"github.com/google/uuid"
)

type Org struct {
	Id               uuid.UUID `json:"id" db:"id" redis:"id"`
	Name             string    `json:"name" db:"name" redis:"name"`
	PrivateKey       string    `json:"private_key" db:"private_key" redis:"private_key"`
	AccountNo        string    `json:"account_no" db:"account_no" redis:"account_no"`
	Balance          int64     `json:"balance" db:"balance" redis:"balance"`
	AvailableBalance int64     `json:"available_balance" db:"available_balance" redis:"available_balance"`
	LockedBalance    int64     `json:"locked_balance" db:"locked_balance" redis:"locked_balance"`
	Currency         string    `json:"currency" db:"currency" redis:"currency"`
	Email            string    `json:"email" db:"email" redis:"email"`
	Password         string    `json:"-" db:"password" redis:"password"`
	CreatedAt        time.Time `json:"created_at" db:"created_at" redis:"created_at"`
	UpdatedAt        time.Time `json:"updated_at" db:"updated_at" redis:"updated_at"`
}
