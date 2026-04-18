package database

import (
	"encoding/json"
	"errors"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"zcopy-server-backend/config"
	"zcopy-server-backend/logger"
	"zcopy-server-backend/models"
)

var (
	ErrUserNotFound   = errors.New("user not found")
	ErrUserDuplicated = errors.New("user duplicated")
	DB                *Store
)

type Store struct {
	mu     sync.RWMutex
	path   string
	nextID uint
	users  []models.User
}

type persistData struct {
	NextID uint          `json:"nextId"`
	Users  []models.User `json:"users"`
}

func InitDB() {
	dbPath := config.AppConfig.Database.Path
	dbDir := filepath.Dir(dbPath)

	if _, err := os.Stat(dbDir); os.IsNotExist(err) {
		if err := os.MkdirAll(dbDir, 0755); err != nil {
			log.Fatalf("Failed to create database directory: %v", err)
		}
	}

	store := &Store{
		path:   dbPath,
		nextID: 1,
	}

	if err := store.load(); err != nil {
		log.Fatalf("Failed to load database: %v", err)
	}

	DB = store
	log.Println("Database initialized successfully")
}

func (s *Store) Close() error {
	return nil
}

func (s *Store) load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, err := os.Stat(s.path); errors.Is(err, os.ErrNotExist) {
		return s.flushLocked()
	}

	content, err := os.ReadFile(s.path)
	if err != nil {
		return err
	}

	if len(content) == 0 {
		return s.flushLocked()
	}

	var data persistData
	if err := json.Unmarshal(content, &data); err != nil {
		return err
	}

	s.users = data.Users
	if data.NextID == 0 {
		s.nextID = uint(len(data.Users) + 1)
	} else {
		s.nextID = data.NextID
	}

	return nil
}

func (s *Store) flushLocked() error {
	data := persistData{
		NextID: s.nextID,
		Users:  s.users,
	}

	content, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		logger.Error("database flush failed", "error", err.Error())
		return err
	}

	err = os.WriteFile(s.path, content, 0644)
	if err != nil {
		logger.Error("database flush failed", "error", err.Error())
	}
	return err
}

func (s *Store) UserExists(username, email string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, user := range s.users {
		if strings.EqualFold(user.Username, username) || strings.EqualFold(user.Email, email) {
			return true
		}
	}

	return false
}

func (s *Store) CreateUser(user *models.User) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, item := range s.users {
		if strings.EqualFold(item.Username, user.Username) || strings.EqualFold(item.Email, user.Email) {
			return ErrUserDuplicated
		}
	}

	now := time.Now()
	user.ID = s.nextID
	user.CreatedAt = now
	user.UpdatedAt = now
	s.nextID++
	s.users = append(s.users, *user)

	err := s.flushLocked()
	if err == nil {
		logger.Info("user created", "user_id", user.ID, "username", user.Username, "email", user.Email)
	}
	return err
}

func (s *Store) FindUserByAccount(account string) (models.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, user := range s.users {
		if strings.EqualFold(user.Username, account) || strings.EqualFold(user.Email, account) {
			return user, nil
		}
	}

	return models.User{}, ErrUserNotFound
}

func (s *Store) FindUserByID(userID uint) (models.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, user := range s.users {
		if user.ID == userID {
			return user, nil
		}
	}

	return models.User{}, ErrUserNotFound
}
