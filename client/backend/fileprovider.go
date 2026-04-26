package main

import (
	fileproviderpkg "zcopy-client-backend/fileprovider"
	"zcopy-client-backend/models"
)

type fileProviderBridgeStatus = fileproviderpkg.BridgeStatus

func (a *AppState) fileProviderAvailable() bool {
	return a.fp.Available()
}

func (a *AppState) initTaskFileProvider(task models.BackupTask) (fileProviderBridgeStatus, error) {
	return a.fp.InitTask(task)
}

func (a *AppState) getTaskFileProviderStatus(task models.BackupTask) (fileProviderBridgeStatus, error) {
	return a.fp.GetTaskStatus(task)
}

func (a *AppState) signalTaskFileProvider(task models.BackupTask, changedPath string) error {
	return a.fp.SignalTask(task, changedPath)
}
