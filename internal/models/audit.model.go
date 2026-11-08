package models

import (
	"time"

	"github.com/google/uuid"
)

type AuditAction string

const (
	AuditActionUserUser AuditAction = "user_user"
	AuditActionOrgUser  AuditAction = "org_user"
)

type AuditActionType string

const (
	AuditActionTypeDebit    AuditActionType = "D"
	AuditActionTypeCredit   AuditActionType = "C"
	AuditActionTypeReversal AuditActionType = "R"
)

type Audit struct {
	Id             int             `json:"id" db:"id" redis:"id"`
	ReferenceId    uuid.UUID       `json:"reference_id" db:"reference_id" redis:"reference_id"`
	Action         AuditAction     `json:"action" db:"action" redis:"action"`
	ActionType     AuditActionType `json:"action_type" db:"action_type" redis:"action_type"`
	Party1         uuid.UUID       `json:"party1" db:"party1" redis:"party1"`
	Party2         uuid.UUID       `json:"party2" db:"party2" redis:"party2"`
	Party1Currency string          `json:"party1_currency" db:"party1_currency" redis:"party1_currency"`
	Party2Currency string          `json:"party2_currency" db:"party2_currency" redis:"party2_currency"`
	ExchangeRate   float64         `json:"exchange_rate" db:"exchange_rate" redis:"exchange_rate"`
	Amount         int64           `json:"amount" db:"amount" redis:"amount"`
	CreatedAt      time.Time       `json:"created_at" db:"created_at" redis:"created_at"`
}
