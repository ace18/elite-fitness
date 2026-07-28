package model

import "time"

type User struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"createdAt"`
}

type MagicLinkToken struct {
	ID        string
	Email     string
	Token     string
	ExpiresAt time.Time
	UsedAt    *time.Time
}
