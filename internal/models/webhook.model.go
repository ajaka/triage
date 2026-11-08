package models

import (
	"time"

	"github.com/google/uuid"
)

type Webhook struct {
	Id                int       `json:"id" db:"id" redis:"id"`
	Url               string    `json:"url" db:"url" redis:"url"`
	SecretHeaderKey   string    `json:"secret_header_key" db:"secret_header_key" redis:"secret_header_key"`
	OrgId             uuid.UUID `json:"org_id" db:"org_id" redis:"org_id"`
	SecretHeaderValue string    `json:"secret_header_value" db:"secret_header_value" redis:"secret_header_value"`
	CreatedAt         time.Time `json:"created_at" db:"created_at" redis:"created_at"`
	UpdatedAt         time.Time `json:"updated_at" db:"updated_at" redis:"updated_at"`
}
