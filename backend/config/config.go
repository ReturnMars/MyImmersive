package config

import (
	"log"
	"os"
	"sync"

	"github.com/joho/godotenv"
)

// Config 应用配置结构体
type Config struct {
	DeepSeekAPIKey string
	DeepSeekURL    string
	ServerPort     string
}

var (
	cfg  *Config
	once sync.Once
)

// Load 加载配置 (单例模式)
func Load() *Config {
	once.Do(func() {
		// 尝试加载 .env 文件 (开发环境)
		if err := godotenv.Load(); err != nil {
			log.Println("[Config] No .env file found, using environment variables")
		}

		cfg = &Config{
			DeepSeekAPIKey: getEnv("DEEPSEEK_API_KEY", ""),
			DeepSeekURL:    getEnv("DEEPSEEK_URL", "https://api.deepseek.com"),
			ServerPort:     getEnv("SERVER_PORT", "8080"),
		}

		// 必须配置的校验
		if cfg.DeepSeekAPIKey == "" {
			log.Fatal("[Config] DEEPSEEK_API_KEY is required but not set!")
		}

		log.Printf("[Config] Loaded: URL=%s, Port=%s\n", cfg.DeepSeekURL, cfg.ServerPort)
	})

	return cfg
}

// getEnv 获取环境变量，支持默认值
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
