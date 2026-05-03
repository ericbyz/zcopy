package registry

import "zcopy-client-backend/models"

type TaskLister interface {
	List() []models.BackupTask
}

type TaskUpdater interface {
	TaskLister
	Upsert(task models.BackupTask) error
}

func MigrateTasksToServer(store TaskUpdater, defaultServerID string) error {
	tasks := store.List()
	for _, task := range tasks {
		if task.ServerID == "" {
			task.ServerID = defaultServerID
			task.ServerIDs = []string{defaultServerID}
			if err := store.Upsert(task); err != nil {
				return err
			}
		}
	}
	return nil
}
