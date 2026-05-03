package models

import (
	"time"
)

type User struct {
	ID        uint   `gorm:"primary_key"`
	Username  string `gorm:"size:100;unique_index;not null"`
	Email     string `gorm:"size:255;unique_index;not null"`
	Password  string `gorm:"size:255;not null"`
	Nickname  string `gorm:"size:100"`
	Avatar    string `gorm:"size:255"`
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time
}

type File struct {
	ID          uint   `gorm:"primary_key"`
	UserID      uint   `gorm:"not null"`
	FileName    string `gorm:"size:255;not null"`
	FilePath    string `gorm:"size:500;not null"`
	FileSize    int64  `gorm:"default:0"`
	FileHash    string `gorm:"size:64"`
	FileType    string `gorm:"size:50"`
	IsDirectory bool   `gorm:"default:false"`
	ParentID    uint   `gorm:"default:0"`
	Permissions string `gorm:"size:20;default:'public'"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   *time.Time
}

type FileVersion struct {
	ID        uint   `gorm:"primary_key"`
	FileID    uint   `gorm:"not null"`
	Version   int    `gorm:"default:1"`
	FileHash  string `gorm:"size:64"`
	FileSize  int64  `gorm:"default:0"`
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time
}

type Share struct {
	ID          uint   `gorm:"primary_key"`
	FileID      uint   `gorm:"not null"`
	UserID      uint   `gorm:"not null"`
	ShareCode   string `gorm:"size:20;unique_index;not null"`
	ExpireTime  time.Time
	Permissions string `gorm:"size:20;default:'read'"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   *time.Time
}
