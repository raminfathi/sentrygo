package config

import (
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	TelegramBotToken string
	TelegramChatID   int64
	TelegramProxy    string
	CameraIP         string
	CameraUser       string
	CameraPass       string
	BaleBotToken     string
	BaleChatID       int64
}

func Load() *Config {
	// Load .env file from the project root directory
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: .env file not found, reading from environment variables.")
	}

	return &Config{
		TelegramBotToken: os.Getenv("TELEGRAM_BOT_TOKEN"),
		TelegramChatID:   parseInt64(os.Getenv("TELEGRAM_CHAT_ID")),
		TelegramProxy:    os.Getenv("TELEGRAM_PROXY"),
		CameraIP:         os.Getenv("CAMERA_IP"),
		CameraUser:       os.Getenv("CAMERA_USER"),
		CameraPass:       os.Getenv("CAMERA_PASS"),
		BaleBotToken:     os.Getenv("BALE_BOT_TOKEN"),
		BaleChatID:       parseInt64(os.Getenv("BALE_CHAT_ID")),
	}
}

func parseInt64(s string) int64 {
	val, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0
	}
	return val
}
