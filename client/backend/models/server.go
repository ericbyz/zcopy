package models

import "time"

type ServerConfig struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	Address         string    `json:"address"`
	WebURL          string    `json:"webUrl,omitempty"`
	IsDefault       bool      `json:"isDefault"`
	LastConnectedAt time.Time `json:"lastConnectedAt"`
	Status          string    `json:"status"` // "online" / "offline" / "unknown"
	AddedAt         time.Time `json:"addedAt"`
}
