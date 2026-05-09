package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	JWT      JWTConfig
	RateLimit RateLimitConfig
}

type ServerConfig struct {
	Port         string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

type DatabaseConfig struct {
	Path string
}

type JWTConfig struct {
	Secret          string
	ExpireDuration  time.Duration
	RefreshExpire   time.Duration
}

type RateLimitConfig struct {
	FreeUserRPM     int
	VipUserRPM      int
	GlobalRPM       int
}

func Load() *Config {
	return &Config{
		Server: ServerConfig{
			Port:         getEnv("SERVER_PORT", ":8080"),
			ReadTimeout:  time.Second * 15,
			WriteTimeout: time.Second * 15,
		},
		Database: DatabaseConfig{
			Path: getEnv("DB_PATH", "./data/cleanmark.db"),
		},
		JWT: JWTConfig{
			Secret:         getEnv("JWT_SECRET", "cleanmark-secret-key-2024"),
			ExpireDuration: time.Hour * 24,
			RefreshExpire:  time.Hour * 720,
		},
		RateLimit: RateLimitConfig{
			FreeUserRPM: getEnvAsInt("FREE_USER_RPM", 10),
			VipUserRPM:  getEnvAsInt("VIP_USER_RPM", 60),
			GlobalRPM:   getEnvAsInt("GLOBAL_RPM", 1000),
		},
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvAsInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}
