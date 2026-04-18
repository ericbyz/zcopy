package config

import (
	"github.com/spf13/viper"
	"log"
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

func LoadConfig() {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("./config")

	viper.SetDefault("server.port", "8890")
	viper.SetDefault("server.mode", "debug")
	viper.SetDefault("database.path", "./data/zcopy.db")
	viper.SetDefault("auth.secret_key", "zcopy-secret-key-change-in-production")
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
		log.Fatalf("Failed to unmarshal config: %v", err)
	}
}
