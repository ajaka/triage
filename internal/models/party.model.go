package models

import (
	"time"

	"github.com/google/uuid"
)

type PartyType string

const (
	PartyTypeUser PartyType = "user"
	PartyTypeOrg  PartyType = "org"
)

type Party struct {
	Id        uuid.UUID `json:"id" db:"id" redis:"id"`
	PartyType PartyType `json:"party_type" db:"party_type" redis:"party_type"`
	CreatedAt time.Time `json:"created_at" db:"created_at" redis:"created_at"`
}
