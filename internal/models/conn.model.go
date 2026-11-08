package models

import (
	"time"

	"github.com/google/uuid"
)

type ConnStatus string

const (
	ConnStatusPending  ConnStatus = "pending"
	ConnStatusAccepted ConnStatus = "accepted"
	ConnStatusRejected ConnStatus = "rejected"
	ConnStatusExpired  ConnStatus = "expired"
)

type Conn struct {
	Id        uuid.UUID  `json:"id" db:"id" redis:"id"`
	UserId    uuid.UUID  `json:"user_id" db:"user_id" redis:"user_id"`
	OrgId     uuid.UUID  `json:"org_id" db:"org_id" redis:"org_id"`
	Status    ConnStatus `json:"status" db:"status" redis:"status"`
	CreatedAt time.Time  `json:"created_at" db:"created_at" redis:"created_at"`
	UpdatedAt time.Time  `json:"updated_at" db:"updated_at" redis:"updated_at"`
}
