package sync

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	logpkg "zcopy-client-backend/log"
	"zcopy-client-backend/models"
)

func SnapshotPath(snapshotDir, taskID string) string {
	return filepath.Join(snapshotDir, taskID+".json")
}

func LoadSnapshot(snapshotDir, taskID string, logs logpkg.LogStore, task models.BackupTask) models.TaskSnapshot {
	path := SnapshotPath(snapshotDir, taskID)
	buf, err := os.ReadFile(path)
	if err != nil {
		logs.Push("debug", task, "", "加载快照：文件不存在")
		return models.TaskSnapshot{Files: map[string]models.FileFingerprint{}}
	}
	var snapshot models.TaskSnapshot
	if err := json.Unmarshal(buf, &snapshot); err != nil {
		logs.Push("debug", task, "", "加载快照：文件不存在")
		return models.TaskSnapshot{Files: map[string]models.FileFingerprint{}}
	}
	if snapshot.Files == nil {
		snapshot.Files = map[string]models.FileFingerprint{}
	}
	logs.Push("debug", task, "", fmt.Sprintf("加载快照：%d 个文件", len(snapshot.Files)))
	return snapshot
}

func SaveSnapshot(snapshotDir, taskID string, snapshot models.TaskSnapshot, logs logpkg.LogStore, task models.BackupTask) error {
	path := SnapshotPath(snapshotDir, taskID)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		logs.Push("error", task, "", "快照保存失败："+err.Error())
		return err
	}
	buf, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		logs.Push("error", task, "", "快照保存失败："+err.Error())
		return err
	}
	err = os.WriteFile(path, buf, 0644)
	if err != nil {
		logs.Push("error", task, "", "快照保存失败："+err.Error())
		return err
	}
	logs.Push("info", task, "", fmt.Sprintf("快照已保存：%d 个文件", len(snapshot.Files)))
	return nil
}
