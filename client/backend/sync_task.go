package main

func (a *AppState) syncTask(taskID string) error {
	return a.syncer.SyncTask(taskID)
}
