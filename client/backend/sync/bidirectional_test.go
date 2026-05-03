package sync

import (
	"testing"

	"zcopy-client-backend/models"
)

func TestChooseSyncAction(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		task       models.BackupTask
		snapshot   models.FileFingerprint
		localMap   map[string]models.LocalFileItem
		remoteMap  map[string]models.FileFingerprint
		wantAction string
	}{
		{
			name:       "new local file uploads to remote",
			task:       models.BackupTask{ConflictMode: models.ConflictLatestPriority},
			localMap:   map[string]models.LocalFileItem{"a.txt": {RelPath: "a.txt", Size: 10, ModUnix: 100}},
			remoteMap:  map[string]models.FileFingerprint{},
			wantAction: "upload",
		},
		{
			name:       "new remote file downloads to local",
			task:       models.BackupTask{ConflictMode: models.ConflictLatestPriority},
			localMap:   map[string]models.LocalFileItem{},
			remoteMap:  map[string]models.FileFingerprint{"a.txt": {Size: 10, ModUnix: 100}},
			wantAction: "download",
		},
		{
			name:       "local changed after snapshot uploads",
			task:       models.BackupTask{ConflictMode: models.ConflictLatestPriority},
			snapshot:   models.FileFingerprint{Size: 10, ModUnix: 100},
			localMap:   map[string]models.LocalFileItem{"a.txt": {RelPath: "a.txt", Size: 10, ModUnix: 200}},
			remoteMap:  map[string]models.FileFingerprint{"a.txt": {Size: 10, ModUnix: 100}},
			wantAction: "upload",
		},
		{
			name:       "remote changed after snapshot downloads",
			task:       models.BackupTask{ConflictMode: models.ConflictLatestPriority},
			snapshot:   models.FileFingerprint{Size: 10, ModUnix: 100},
			localMap:   map[string]models.LocalFileItem{"a.txt": {RelPath: "a.txt", Size: 10, ModUnix: 100}},
			remoteMap:  map[string]models.FileFingerprint{"a.txt": {Size: 10, ModUnix: 200}},
			wantAction: "download",
		},
		{
			name:       "conflict prefers local when configured",
			task:       models.BackupTask{ConflictMode: models.ConflictLocalPriority},
			snapshot:   models.FileFingerprint{Size: 10, ModUnix: 100},
			localMap:   map[string]models.LocalFileItem{"a.txt": {RelPath: "a.txt", Size: 20, ModUnix: 200}},
			remoteMap:  map[string]models.FileFingerprint{"a.txt": {Size: 30, ModUnix: 300}},
			wantAction: "upload",
		},
		{
			name:       "remote deletion removes unchanged local file",
			task:       models.BackupTask{ConflictMode: models.ConflictLatestPriority},
			snapshot:   models.FileFingerprint{Size: 10, ModUnix: 100},
			localMap:   map[string]models.LocalFileItem{"a.txt": {RelPath: "a.txt", Size: 10, ModUnix: 100}},
			remoteMap:  map[string]models.FileFingerprint{},
			wantAction: "delete_local",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := chooseSyncAction(tt.task, "a.txt", tt.snapshot, tt.localMap, tt.remoteMap)
			if got != tt.wantAction {
				t.Fatalf("chooseSyncAction() = %q, want %q", got, tt.wantAction)
			}
		})
	}
}
