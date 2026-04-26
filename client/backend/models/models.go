package models

import (
	"sync"
	"time"
)

type AppConfig struct {
	Server struct {
		Port string `yaml:"port"`
		Mode string `yaml:"mode"`
	} `yaml:"server"`
	FileServer struct {
		BaseURL string `yaml:"base_url"`
	} `yaml:"file_server"`
	Storage struct {
		DataDir string `yaml:"data_dir"`
	} `yaml:"storage"`
}

type BackupTask struct {
	ID           string      `json:"id"`
	Name         string      `json:"name"`
	TaskMode     string      `json:"taskMode,omitempty"`
	ConflictMode string      `json:"conflictMode,omitempty"`
	LocalPath    string      `json:"localPath"`
	RemotePath   string      `json:"remotePath"`
	AutoBackup   bool        `json:"autoBackup"`
	OnDemandSync bool        `json:"onDemandSync"`
	CloudOnly    bool        `json:"cloudOnly"`
	Status       string      `json:"status"`
	LastError    string      `json:"lastError"`
	LastSyncAt   *time.Time  `json:"lastSyncAt"`
	SyncReport   *SyncReport `json:"syncReport,omitempty"`
	CreatedAt    time.Time   `json:"createdAt"`
	UpdatedAt    time.Time   `json:"updatedAt"`
}

const (
	TaskModeBackup = "backup"
	TaskModeSync   = "sync"

	ConflictLatestPriority = "latest"
	ConflictLocalPriority  = "local"
	ConflictRemotePriority = "remote"
)

type SyncReport struct {
	State            string     `json:"state"`
	Mode             string     `json:"mode"`
	Message          string     `json:"message"`
	TotalFiles       int        `json:"totalFiles"`
	UploadedFiles    int        `json:"uploadedFiles"`
	FailedFiles      int        `json:"failedFiles"`
	TotalBytes       int64      `json:"totalBytes"`
	TransferredBytes int64      `json:"transferredBytes"`
	SpeedBytesPerSec int64      `json:"speedBytesPerSec"`
	FailedFilePaths  []string   `json:"failedFilePaths,omitempty"`
	StartedAt        time.Time  `json:"startedAt"`
	FinishedAt       *time.Time `json:"finishedAt,omitempty"`
}

type TransferLog struct {
	ID        string    `json:"id"`
	TaskID    string    `json:"taskId"`
	TaskName  string    `json:"taskName"`
	Level     string    `json:"level"`
	FilePath  string    `json:"filePath,omitempty"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"createdAt"`
}

type TaskSnapshot struct {
	Files map[string]FileFingerprint `json:"files"`
}

type FileFingerprint struct {
	Size    int64 `json:"size"`
	ModUnix int64 `json:"modUnix"`
}

type LocalFileItem struct {
	AbsPath string
	RelPath string
	Size    int64
	ModUnix int64
}

type WatchController struct {
	StopCh chan struct{}
	DoneCh chan struct{}
}

type TaskStore struct {
	Mu    sync.RWMutex
	Path  string
	Tasks []BackupTask
}
