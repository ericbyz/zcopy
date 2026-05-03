package models

import (
	"time"
)

type ServerInfo struct {
	UUID      string    `json:"uuid"`
	Name      string    `json:"name"`
	Version   string    `json:"version"`
	Address   string    `json:"address"`
	CreatedAt time.Time `json:"createdAt"`
}
