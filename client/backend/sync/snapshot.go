package sync

import (
	"encoding/json"
	"os"
	"path/filepath"

	"zcopy-client-backend/models"
)

func SnapshotPath(snapshotDir, taskID string) string {
	return filepath.Join(snapshotDir, taskID+".json")
}

func LoadSnapshot(snapshotDir, taskID string) models.TaskSnapshot {
	path := SnapshotPath(snapshotDir, taskID)
	buf, err := os.ReadFile(path)
	if err != nil {
		return models.TaskSnapshot{Files: map[string]models.FileFingerprint{}}
	}
	var snapshot models.TaskSnapshot
	if err := json.Unmarshal(buf, &snapshot); err != nil {
		return models.TaskSnapshot{Files: map[string]models.FileFingerprint{}}
	}
	if snapshot.Files == nil {
		snapshot.Files = map[string]models.FileFingerprint{}
	}
	return snapshot
}

func SaveSnapshot(snapshotDir, taskID string, snapshot models.TaskSnapshot) error {
	path := SnapshotPath(snapshotDir, taskID)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	buf, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, buf, 0644)
}
