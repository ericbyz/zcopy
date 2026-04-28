package serverinfo

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"zcopy-server-backend/models"
)

var (
	instance *models.ServerInfo
	mu       sync.RWMutex
)

func generateUUID() (string, error) {
	uuid := make([]byte, 16)
	_, err := rand.Read(uuid)
	if err != nil {
		return "", err
	}

	uuid[6] = (uuid[6] & 0x0f) | 0x40
	uuid[8] = (uuid[8] & 0x3f) | 0x80

	return fmt.Sprintf("%02x%02x%02x%02x-%02x%02x-%02x%02x-%02x%02x-%02x%02x%02x%02x%02x%02x",
		uuid[0], uuid[1], uuid[2], uuid[3],
		uuid[4], uuid[5],
		uuid[6], uuid[7],
		uuid[8], uuid[9],
		uuid[10], uuid[11], uuid[12], uuid[13], uuid[14], uuid[15]), nil
}

func InitServerInfo(dataDir string, address string) (*models.ServerInfo, error) {
	mu.Lock()
	defer mu.Unlock()

	if instance != nil {
		return instance, nil
	}

	filePath := filepath.Join(dataDir, "server_info.json")

	if _, err := os.Stat(dataDir); os.IsNotExist(err) {
		if err := os.MkdirAll(dataDir, 0755); err != nil {
			return nil, err
		}
	}

	if _, err := os.Stat(filePath); !os.IsNotExist(err) {
		content, err := os.ReadFile(filePath)
		if err != nil {
			return nil, err
		}

		var info models.ServerInfo
		if err := json.Unmarshal(content, &info); err != nil {
			return nil, err
		}

		instance = &info
		return instance, nil
	}

	uuid, err := generateUUID()
	if err != nil {
		return nil, err
	}

	loc, _ := time.LoadLocation("Asia/Shanghai")
	now := time.Now().In(loc)

	instance = &models.ServerInfo{
		UUID:      uuid,
		Name:      "ZCopy Server",
		Version:   "1.0.0",
		Address:   address,
		CreatedAt: now,
	}

	content, err := json.MarshalIndent(instance, "", "  ")
	if err != nil {
		return nil, err
	}

	if err := os.WriteFile(filePath, content, 0644); err != nil {
		return nil, err
	}

	return instance, nil
}

func GetServerInfo() *models.ServerInfo {
	mu.RLock()
	defer mu.RUnlock()
	return instance
}
