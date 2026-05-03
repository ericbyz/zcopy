package config

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Database DatabaseConfig `mapstructure:"database"`
	Auth     AuthConfig     `mapstructure:"auth"`
	Storage  StorageConfig  `mapstructure:"storage"`
	Log      LogConfig      `mapstructure:"log"`
}

type ServerConfig struct {
	Port string `mapstructure:"port"`
	Mode string `mapstructure:"mode"`
}

type DatabaseConfig struct {
	Path string `mapstructure:"path"`
}

type AuthConfig struct {
	SecretKey        string `mapstructure:"secret_key"`
	TokenExpireHours int    `mapstructure:"token_expire_hours"`
}

type StorageConfig struct {
	RootDir string `mapstructure:"root_dir"`
}

type LogConfig struct {
	Level         string `mapstructure:"level"`
	Dir           string `mapstructure:"dir"`
	MaxAgeDays    int    `mapstructure:"max_age_days"`
	MaxSizeMB     int    `mapstructure:"max_size_mb"`
	MaxFileSizeMB int    `mapstructure:"max_file_size_mb"`
}

var AppConfig Config

func LoadConfig() error {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")

	if cfgPath := strings.TrimSpace(os.Getenv("ZCOPY_SERVER_CONFIG")); cfgPath != "" {
		viper.SetConfigFile(cfgPath)
	} else {
		viper.AddConfigPath(".")
		viper.AddConfigPath("./config")
	}

	viper.SetDefault("server.port", "8890")
	viper.SetDefault("server.mode", "debug")
	viper.SetDefault("database.path", "./data/zcopy.db")
	// JWT secret must be explicitly configured — no insecure default
	viper.SetDefault("auth.token_expire_hours", 72)
	viper.SetDefault("storage.root_dir", "./storage")
	viper.SetDefault("log.level", "info")
	viper.SetDefault("log.dir", "./data/logs")
	viper.SetDefault("log.max_age_days", 30)
	viper.SetDefault("log.max_size_mb", 500)
	viper.SetDefault("log.max_file_size_mb", 100)

	if err := viper.ReadInConfig(); err != nil {
		log.Printf("Config file not found, using default settings: %v", err)
	}

	if err := viper.Unmarshal(&AppConfig); err != nil {
		return fmt.Errorf("failed to unmarshal config: %w", err)
	}

	if strings.TrimSpace(AppConfig.Auth.SecretKey) == "" {
		return errors.New("auth.secret_key is required — set it in config.yaml or AUTH_SECRET_KEY env var")
	}

	return nil
}
