package notify

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"sync"
	"time"

	"sentrygo/internal/config"
)

type SMSService struct {
	accounts []config.SMSAccount // لیست حساب‌های پیامکی
	lastSent time.Time
	mu       sync.Mutex
	Cooldown time.Duration
}

func NewSMSService(accounts []config.SMSAccount) *SMSService {
	return &SMSService{
		accounts: accounts,
		Cooldown: 3 * time.Minute,
	}
}

func (s *SMSService) IsReady() bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if time.Since(s.lastSent) < s.Cooldown {
		return false
	}
	s.lastSent = time.Now()
	return true
}

func (s *SMSService) SendSMS(message string) {
	for _, acc := range s.accounts {
		// هر پیامک را در یک گوروتین جدا می‌فرستیم تا معطل هم نشوند
		go func(account config.SMSAccount) {
			apiURL := fmt.Sprintf("https://api.kavenegar.com/v1/%s/sms/send.json", account.APIKey)
			params := url.Values{}
			params.Add("receptor", account.TargetPhone)
			params.Add("message", message)
			if account.Sender != "" {
				params.Add("sender", account.Sender)
			}

			resp, err := http.PostForm(apiURL, params)
			if err == nil && resp.StatusCode == http.StatusOK {
				log.Printf("SMS sent successfully to %s", account.TargetPhone)
			} else {
				log.Printf("SMS failed to %s. Status: %d", account.TargetPhone, resp.StatusCode)
			}
			if err == nil {
				resp.Body.Close()
			}
		}(acc)
	}
}

func (s *SMSService) MakeCall(message string) {
	for _, acc := range s.accounts {
		go func(account config.SMSAccount) {
			apiURL := fmt.Sprintf("https://api.kavenegar.com/v1/%s/call/maketts.json", account.APIKey)
			params := url.Values{}
			params.Add("receptor", account.TargetPhone)
			params.Add("message", message)

			resp, err := http.PostForm(apiURL, params)
			if err == nil && resp.StatusCode == http.StatusOK {
				log.Printf("Voice call initiated to %s", account.TargetPhone)
			}
			if err == nil {
				resp.Body.Close()
			}
		}(acc)
	}
}
