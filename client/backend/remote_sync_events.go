package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"zcopy-client-backend/models"
)

type remoteSyncEvent struct {
	TaskID string `json:"taskId"`
	Path   string `json:"path"`
	Type   string `json:"type"`
}

func (a *AppState) startRemoteEventLoop() {
	go func() {
		for {
			token := a.getToken()
			if strings.TrimSpace(token) == "" {
				time.Sleep(2 * time.Second)
				continue
			}
			if err := a.consumeRemoteEventStream(token); err != nil && strings.TrimSpace(a.getToken()) != "" {
				a.pushLog("warn", models.BackupTask{Name: "system"}, "", "服务端同步事件流已断开: "+err.Error())
			}
			time.Sleep(2 * time.Second)
		}
	}()
}

func (a *AppState) consumeRemoteEventStream(token string) error {
	endpoint := strings.TrimRight(a.cfg.FileServer.BaseURL, "/") + "/sync/events/stream"
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "text/event-stream")

	resp, err := a.httpc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized {
		return nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return errors.New(resp.Status)
	}

	a.pushLog("info", models.BackupTask{Name: "system"}, "", "服务端同步事件流已连接")

	scanner := bufio.NewScanner(resp.Body)
	var eventName string
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			eventName = ""
			continue
		}
		if strings.HasPrefix(line, ":") {
			continue
		}
		if strings.HasPrefix(line, "event:") {
			eventName = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
			continue
		}
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		if eventName != "" && eventName != "sync" {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		var evt remoteSyncEvent
		if err := json.Unmarshal([]byte(payload), &evt); err != nil {
			continue
		}
		a.handleRemoteSyncEvent(evt)
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return nil
}

func (a *AppState) handleRemoteSyncEvent(evt remoteSyncEvent) {
	task, ok := a.store.Get(evt.TaskID)
	if !ok || !isSyncTask(task) {
		return
	}
	a.pushLog("info", task, evt.Path, "收到服务端变更事件: "+evt.Type)
	a.scheduleRemoteTaskSync(task, evt)
}

func (a *AppState) scheduleRemoteTaskSync(task models.BackupTask, evt remoteSyncEvent) {
	a.remoteSyncMu.Lock()
	if timer, ok := a.remoteSyncTimers[task.ID]; ok {
		timer.Stop()
	}
	a.remoteSyncTimers[task.ID] = time.AfterFunc(1500*time.Millisecond, func() {
		if err := a.syncTask(task.ID); err != nil {
			a.pushLog("error", task, evt.Path, "应用服务端变更失败: "+err.Error())
			_ = a.ackRemoteSyncEvent(evt, "failed", err.Error())
		} else {
			if err := a.signalTaskFileProvider(task, evt.Path); err != nil {
				a.pushLog("warn", task, evt.Path, "同步已完成，但刷新云文件夹失败: "+err.Error())
			}
			a.pushLog("info", task, evt.Path, "服务端变更已同步到客户端")
			_ = a.ackRemoteSyncEvent(evt, "success", "")
		}
		a.remoteSyncMu.Lock()
		delete(a.remoteSyncTimers, task.ID)
		a.remoteSyncMu.Unlock()
	})
	a.remoteSyncMu.Unlock()
}

func (a *AppState) ackRemoteSyncEvent(evt remoteSyncEvent, result string, detail string) error {
	token := a.getToken()
	if strings.TrimSpace(token) == "" {
		return nil
	}
	body, err := json.Marshal(map[string]string{
		"taskId": evt.TaskID,
		"path":   evt.Path,
		"type":   evt.Type,
		"result": result,
		"detail": detail,
	})
	if err != nil {
		return err
	}
	_, _, err = a.proxyRaw(http.MethodPost, "/sync/events/ack", bytes.NewReader(body), "application/json", token)
	return err
}
