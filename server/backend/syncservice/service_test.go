package syncservice

import "testing"

func TestOccupiedBy(t *testing.T) {
	t.Parallel()

	manager := &Manager{
		store: &taskStore{
			tasks: []SyncTask{
				{UserID: 1, ClientTaskID: "task-1", TaskName: "任务一", RemotePath: "docs/a"},
				{UserID: 1, ClientTaskID: "task-2", TaskName: "任务二", RemotePath: "docs/b"},
				{UserID: 2, ClientTaskID: "task-3", TaskName: "其他用户", RemotePath: "docs/a"},
			},
		},
	}

	if _, ok := manager.OccupiedBy(1, "docs/a", ""); !ok {
		t.Fatalf("expected docs/a to be occupied for user 1")
	}
	if _, ok := manager.OccupiedBy(1, "docs/a", "task-1"); ok {
		t.Fatalf("expected excludeTaskID to ignore current task")
	}
	if _, ok := manager.OccupiedBy(1, "docs/c", ""); ok {
		t.Fatalf("expected docs/c to be free")
	}
	if _, ok := manager.OccupiedBy(2, "docs/a", ""); !ok {
		t.Fatalf("expected docs/a to be occupied for user 2")
	}
}
