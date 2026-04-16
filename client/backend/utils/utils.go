package utils

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"zcopy-client-backend/models"
)

func ParseJSONMessage(data []byte) string {
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		return ""
	}
	if msg, ok := payload["message"].(string); ok {
		return msg
	}
	return ""
}

func NormalizeRemote(path string) string {
	p := strings.TrimSpace(filepath.ToSlash(path))
	p = strings.TrimPrefix(p, "/")
	p = strings.TrimSuffix(p, "/")
	if p == "." {
		return ""
	}
	return p
}

func CalcSpeed(transferred int64, startedAt time.Time) int64 {
	elapsed := time.Since(startedAt).Seconds()
	if elapsed <= 0 {
		return 0
	}
	return int64(float64(transferred) / elapsed)
}

func CollectLocalFiles(root string) ([]models.LocalFileItem, error) {
	items := make([]models.LocalFileItem, 0)
	err := filepath.Walk(root, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if rel == "." {
			return nil
		}
		if info.IsDir() {
			return nil
		}
		items = append(items, models.LocalFileItem{
			AbsPath: path,
			RelPath: rel,
			Size:    info.Size(),
			ModUnix: info.ModTime().Unix(),
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	return items, nil
}
