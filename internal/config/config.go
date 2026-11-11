package config

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type SMSAccount struct {
	APIKey      string
	TargetPhone string
	Sender      string
}

type Config struct {
	TelegramBotToken string
	TelegramChatIDs  []int64
	TelegramProxy    string
	CameraIP         string
	CameraUser       string
	CameraPass       string
	BaleBotToken     string
	BaleChatIDs      []int64
	RecordingPath    string
	RetentionDays    int
	SilentDetection  bool

	SMSAccounts []SMSAccount
}

func Load() *Config {
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: No .env file found. Using environment variables.")
	}
	retentionStr := os.Getenv("RETENTION_DAYS")
	retentionDays := 7
	if retentionStr != "" {
		fmt.Sscanf(retentionStr, "%d", &retentionDays)
	}
	silentDetection := os.Getenv("SILENT_DETECTION") == "true"
	return &Config{
		TelegramBotToken: os.Getenv("TELEGRAM_BOT_TOKEN"),
		TelegramChatIDs:  parseChatIDs(os.Getenv("TELEGRAM_CHAT_ID")),
		TelegramProxy:    os.Getenv("TELEGRAM_PROXY"),
		CameraIP:         os.Getenv("CAMERA_IP"),
		CameraUser:       os.Getenv("CAMERA_USER"),
		CameraPass:       os.Getenv("CAMERA_PASS"),
		BaleBotToken:     os.Getenv("BALE_BOT_TOKEN"),
		BaleChatIDs:      parseChatIDs(os.Getenv("BALE_CHAT_ID")),
		SMSAccounts:      parseSMS(os.Getenv("SMS_ACCOUNTS")),
		RecordingPath:    os.Getenv("RECORDING_PATH"),
		RetentionDays:    retentionDays,
		SilentDetection:  silentDetection,
	}
}

// تابع کمکی برای خواندن چند آیدی که با کاما جدا شده‌اند
func parseChatIDs(val string) []int64 {
	var ids []int64
	for _, p := range strings.Split(val, ",") {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if id, err := strconv.ParseInt(p, 10, 64); err == nil {
			ids = append(ids, id)
		}
	}
	return ids
}

// تابع کمکی برای خواندن تنظیمات چندگانه پیامک
func parseSMS(val string) []SMSAccount {
	var accounts []SMSAccount
	for _, p := range strings.Split(val, ",") {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		// فرمت: token:phone:sender
		parts := strings.Split(p, ":")
		if len(parts) == 3 {
			accounts = append(accounts, SMSAccount{
				APIKey:      strings.TrimSpace(parts[0]),
				TargetPhone: strings.TrimSpace(parts[1]),
				Sender:      strings.TrimSpace(parts[2]),
			})
		}
	}
	return accounts
}
