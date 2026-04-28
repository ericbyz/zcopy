package models

import (
	"time"
)

type ClientSession struct {
	SessionID     string    `json:"sessionId"`
	UserID        uint      `json:"userId"`
	Username      string    `json:"username"`
	ClientIP      string    `json:"clientIp"`
	ServerID      string    `json:"serverId"`
	ActiveTasks   int       `json:"activeTasks"`
	LastHeartbeat time.Time `json:"lastHeartbeat"`
	Status        string    `json:"status"`
	ConnectedAt   time.Time `json:"connectedAt"`
}
