package config

import (
	"log"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

// Config holds all the environment variables for the application.
type Config struct {
	CameraIP             string
	CameraUser           string
	CameraPass           string
	TelegramBotToken     string
	TelegramChatID       int64
	AlertCooldownSeconds time.Duration
	ProxyURL             string
}

// Load reads the .env file and populates the Config struct.
func Load() *Config {
	if err := godotenv.Load(); err != nil {
		log.Println("WARNING: No .env file found, relying on system environment variables.")
	}

	chatID, _ := strconv.ParseInt(getEnv("TELEGRAM_CHAT_ID", "0"), 10, 64)
	cooldown, _ := strconv.Atoi(getEnv("ALERT_COOLDOWN_SECONDS", "60"))

	return &Config{
		CameraIP:             getEnv("CAMERA_IP", "192.168.1.64"),
		CameraUser:           getEnv("CAMERA_USER", "admin"),
		CameraPass:           getEnv("CAMERA_PASS", ""),
		TelegramBotToken:     getEnv("TELEGRAM_BOT_TOKEN", ""),
		TelegramChatID:       chatID,
		AlertCooldownSeconds: time.Duration(cooldown) * time.Second,
		ProxyURL:             getEnv("PROXY_URL", ""),
	}
}

// getEnv retrieves the value of the environment variable named by the key.
// It returns the fallback value if the variable is not present.
func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}
