package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"zcopy-client-backend/models"
	"zcopy-client-backend/platform"
	"zcopy-client-backend/utils"
)

func normalizeTaskFields(task *models.BackupTask) {
	task.Name = strings.TrimSpace(task.Name)
	task.RemotePath = utils.NormalizeRemote(task.RemotePath)
	task.TaskMode = normalizedTaskMode(task.TaskMode)
	task.ConflictMode = normalizedConflictMode(task.ConflictMode)
}

func normalizedTaskMode(raw string) string {
	switch strings.TrimSpace(raw) {
	case models.TaskModeSync:
		return models.TaskModeSync
	default:
		return models.TaskModeBackup
	}
}

func normalizedConflictMode(raw string) string {
	switch strings.TrimSpace(raw) {
	case models.ConflictLocalPriority:
		return models.ConflictLocalPriority
	case models.ConflictRemotePriority:
		return models.ConflictRemotePriority
	default:
		return models.ConflictLatestPriority
	}
}

func isSyncTask(task models.BackupTask) bool {
	return normalizedTaskMode(task.TaskMode) == models.TaskModeSync
}

func shouldAutoCreateLocalPath(task models.BackupTask) bool {
	return isSyncTask(task) && task.OnDemandSync
}

func taskGeneratedLocalPath(taskName string) (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	safe := strings.TrimSpace(platform.SafeName(taskName))
	if safe == "" {
		safe = "zcopy-task"
	}
	return filepath.Join(homeDir, "ZCopy", safe), nil
}

func (a *AppState) validateAndPrepareTask(task *models.BackupTask) error {
	normalizeTaskFields(task)
	if task.Name == "" {
		return errors.New("任务名称不能为空")
	}

	if shouldAutoCreateLocalPath(*task) {
		localPath, err := taskGeneratedLocalPath(task.Name)
		if err != nil {
			return errors.New("生成本地目录失败: " + err.Error())
		}
		if err := os.MkdirAll(localPath, 0755); err != nil {
			return errors.New("创建本地目录失败: " + err.Error())
		}
		task.LocalPath = localPath
	}

	if isSyncTask(*task) {
		task.CloudOnly = false
		if task.LocalPath == "" {
			return errors.New("同步模式需要本地目录")
		}
		info, err := os.Stat(task.LocalPath)
		if err != nil || !info.IsDir() {
			return errors.New("本地目录不存在或不可用")
		}
		return nil
	}

	if task.CloudOnly {
		if !task.OnDemandSync {
			return errors.New("全新模式需要开启按需同步")
		}
		task.LocalPath = ""
		return nil
	}

	task.LocalPath = filepath.Clean(strings.TrimSpace(task.LocalPath))
	if task.LocalPath == "" {
		return errors.New("本地目录不能为空")
	}
	info, err := os.Stat(task.LocalPath)
	if err != nil || !info.IsDir() {
		return errors.New("本地目录不存在或不可用")
	}
	return nil
}

func (a *AppState) upsertRemoteSyncTask(task models.BackupTask) error {
	if !isSyncTask(task) {
		return nil
	}
	token := a.getToken()
	if token == "" {
		return errors.New("请先登录客户端")
	}
	body, err := json.Marshal(map[string]any{
		"clientTaskId": task.ID,
		"taskName":     task.Name,
		"remotePath":   task.RemotePath,
		"conflictMode": normalizedConflictMode(task.ConflictMode),
		"onDemandSync": task.OnDemandSync,
	})
	if err != nil {
		return err
	}
	data, status, err := a.proxyRaw(http.MethodPut, "/sync/tasks", bytes.NewReader(body), "application/json", token)
	if err != nil {
		return err
	}
	if status < 200 || status >= 300 {
		msg := utils.ParseJSONMessage(data)
		if msg == "" {
			msg = "同步任务注册失败"
		}
		return errors.New(msg)
	}
	return nil
}

func (a *AppState) deleteRemoteSyncTask(task models.BackupTask) error {
	if !isSyncTask(task) {
		return nil
	}
	token := a.getToken()
	if token == "" {
		return nil
	}
	data, status, err := a.proxyRaw(http.MethodDelete, "/sync/tasks/"+task.ID, nil, "", token)
	if err != nil {
		return err
	}
	if status == http.StatusNotFound {
		return nil
	}
	if status < 200 || status >= 300 {
		msg := utils.ParseJSONMessage(data)
		if msg == "" {
			msg = "删除服务端同步任务失败"
		}
		return errors.New(msg)
	}
	return nil
}

func (a *AppState) restoreRemoteSyncTasks() {
	for _, task := range a.store.List() {
		if !isSyncTask(task) {
			continue
		}
		if err := a.upsertRemoteSyncTask(task); err != nil {
			a.pushLog("warn", task, "", "恢复服务端同步任务失败: "+err.Error())
		}
	}
}
