package config

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"strconv"
	"time"
)

// Config holds all application configuration
type Config struct {
	// Database
	DBPath string

	// Security
	JWTSecret        string
	JWTSecretFromEnv bool // 是否从环境变量读取
	CORSOrigins      string

	// n2n Management
	MgmtAddr string

	// Cache
	IPCacheTTL  time.Duration
	IPCacheSize int

	// Server
	Port string

	// Features
	DisableNetTools bool // 禁用网络诊断工具
}

var cfg *Config

// Get returns the global configuration
func Get() *Config {
	if cfg == nil {
		cfg = Load()
	}
	return cfg
}

// generateRandomSecret 生成随机密钥
func generateRandomSecret() string {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		panic("crypto/rand failed: " + err.Error())
	}
	return hex.EncodeToString(bytes)
}

// Load reads configuration from environment variables with defaults
func Load() *Config {
	jwtSecret := os.Getenv("N2N_ADMIN_SECRET")
	jwtFromEnv := jwtSecret != ""
	if !jwtFromEnv {
		// 未设置环境变量时生成随机密钥（每次重启会变化）
		jwtSecret = generateRandomSecret()
	}

	return &Config{
		DBPath:           getEnv("N2N_DB_PATH", "n2n_admin.db"),
		JWTSecret:        jwtSecret,
		JWTSecretFromEnv: jwtFromEnv,
		CORSOrigins:      getEnv("N2N_CORS_ORIGINS", ""),
		MgmtAddr:         getEnv("N2N_MGMT_ADDR", "127.0.0.1:56440"),
		IPCacheTTL:       getDurationEnv("N2N_IP_CACHE_TTL", 24*time.Hour),
		IPCacheSize:      getIntEnv("N2N_IP_CACHE_SIZE", 1000),
		Port:             getEnv("N2N_PORT", "8080"),
		DisableNetTools:  !getBoolEnv("N2N_ENABLE_NET_TOOLS", false), // 默认禁用，设置 N2N_ENABLE_NET_TOOLS=true 启用
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getIntEnv(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}

func getDurationEnv(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}

func getBoolEnv(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		return value == "true" || value == "1" || value == "yes"
	}
	return defaultValue
}
