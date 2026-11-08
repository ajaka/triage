package models

import (
	"time"

	"github.com/google/uuid"
)

type Role string

const (
	RoleUser       Role = "user"
	RoleAdmin      Role = "admin"
	RoleSuperadmin Role = "superadmin"
)

type User struct {
	Id               uuid.UUID `json:"id" db:"id" redis:"id"`
	AccountNo        string    `json:"account_no" db:"account_no" redis:"account_no"`
	FirstName        string    `json:"first_name" db:"first_name" redis:"first_name"`
	LastName         string    `json:"last_name" db:"last_name" redis:"last_name"`
	Email            string    `json:"email" db:"email" redis:"email"`
	Password         string    `json:"-" db:"password" redis:"password"`
	Verified         bool      `json:"verified" db:"verified" redis:"verified"`
	Active           bool      `json:"active" db:"active" redis:"active"`
	Balance          int64     `json:"balance" db:"balance" redis:"balance"`
	AvailableBalance int64     `json:"available_balance" db:"available_balance" redis:"available_balance"`
	LockedBalance    int64     `json:"locked_balance" db:"locked_balance" redis:"locked_balance"`
	Currency         string    `json:"currency" db:"currency" redis:"currency"`
	CreatedAt        time.Time `json:"created_at" db:"created_at" redis:"created_at"`
	UpdatedAt        time.Time `json:"updated_at" db:"updated_at" redis:"updated_at"`
}
