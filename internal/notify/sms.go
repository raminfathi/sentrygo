package notify

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/raminfathi/sentrygo/internal/config"
)

type SMSService struct {
	accounts []config.SMSAccount
	lastSent time.Time
	mu       sync.Mutex
}

func NewSMSService(accounts []config.SMSAccount) *SMSService {
	return &SMSService{
		accounts: accounts,
	}
}

func (s *SMSService) IsReady() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	// جلوگیری از ارسال رگباری (ارسال حداکثر هر ۳ دقیقه یکبار)
	if time.Since(s.lastSent) < 3*time.Minute {
		return false
	}
	s.lastSent = time.Now()
	return true
}

func (s *SMSService) SendSMS(message string) {
	patternCode := os.Getenv("SMS_PATTERN_CODE")

	for _, acc := range s.accounts {
		go func(account config.SMSAccount) {
			// ۱. ابتدا تلاش برای ارسال سریع با پترن
			if patternCode != "" && patternCode != "YOUR_PATTERN_CODE" {
				if s.trySendPattern(account, patternCode, message) {
					return // در صورت موفقیت، از تابع خارج می‌شود
				}
			}

			// ۲. در صورت خطا در پترن (یا تایید نشدن آن)، تغییر مسیر به پیامک عادی
			log.Printf("Pattern pending or failed. Falling back to standard SMS for %s...", account.TargetPhone)
			s.trySendStandard(account, message)
		}(acc)
	}
}

// ارسال پیامک عادی (در صورت شکست پترن) منطبق با مستندات ارسال چندگانه
func (s *SMSService) trySendStandard(account config.SMSAccount, message string) {
	// تغییر آدرس به آدرس صحیح ارسال پیامک ساده
	apiURL := "https://api.iranpayamak.com/ws/v1/sms/simple"

	alertText := "🚨 هشدار SentryGo\nسیستم متوجه حضور یک شخص شد!\nساعت: " + time.Now().Format("15:04:05")
	if message == "تست سیستم" {
		alertText = "✅ تست: سیستم دزدگیر با موفقیت روشن شد."
	}

	// اصلاح نام کلیدها (text و line_number) مطابق مستندات فراز
	payload := map[string]interface{}{
		"text":          alertText,
		"line_number":   account.Sender,
		"recipients":    []string{account.TargetPhone}, // در ارسال ساده، گیرنده آرایه است
		"number_format": "english",
	}

	jsonData, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", apiURL, bytes.NewBuffer(jsonData))
	req.Header.Set("Api-Key", account.APIKey)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Standard SMS Network Error: %v", err)
		return
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated {
		log.Printf("Standard SMS sent successfully to %s! Response: %s", account.TargetPhone, string(bodyBytes))
	} else {
		log.Printf("Standard SMS FAILED to %s. Status: %d, Response: %s", account.TargetPhone, resp.StatusCode, string(bodyBytes))
	}
}

func (s *SMSService) MakeCall(message string) {}

func (s *SMSService) SendStartupMessage() {
	log.Println("Sending startup test SMS...")
	s.SendSMS("تست سیستم")
}
func (s *SMSService) trySendPattern(account config.SMSAccount, defaultPattern string, message string) bool {
	apiURL := "https://api.iranpayamak.com/ws/v1/sms/pattern"

	// انتخاب خودکار پترن بر اساس نوع پیام
	patternCode := defaultPattern
	if message == "تست سیستم" {
		startupPattern := os.Getenv("SMS_STARTUP_PATTERN")
		if startupPattern != "" {
			patternCode = startupPattern
		} else {
			patternCode = ""
		}
	}

	payload := map[string]interface{}{
		"code":          patternCode,
		"recipient":     account.TargetPhone,
		"line_number":   account.Sender,
		"number_format": "english",
		"attributes": map[string]string{
			"time": time.Now().Format("15:04:05"),
		},
	}

	jsonData, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", apiURL, bytes.NewBuffer(jsonData))
	req.Header.Set("Api-Key", account.APIKey)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		return false
	}

	bodyBytes, _ := io.ReadAll(resp.Body)
	log.Printf("SMS (PATTERN) sent successfully to %s! Response: %s", account.TargetPhone, string(bodyBytes))
	return true
}
