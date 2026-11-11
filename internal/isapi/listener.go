package isapi

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/raminfathi/sentrygo/internal/core"
	"github.com/raminfathi/sentrygo/internal/media"
	"github.com/raminfathi/sentrygo/internal/notify"
	"github.com/raminfathi/sentrygo/internal/vision"

	"github.com/icholy/digest"
)

type Listener struct {
	ip           string
	username     string
	password     string
	state        *core.SystemState
	media        *media.Service
	notifier     *notify.TelegramService
	baleNotifier *notify.BaleService
	sms          *notify.SMSService
	vision       *vision.FaceDetector

	lastTrigger time.Time
	mu          sync.Mutex
}

// NewListener initializes a new ISAPI stream listener.
func NewListener(
	ip, username, password string,
	state *core.SystemState,
	mediaSvc *media.Service,
	notifier *notify.TelegramService,
	baleNotifier *notify.BaleService,
	smsSvc *notify.SMSService,
	faceDetector *vision.FaceDetector,
) *Listener {
	return &Listener{
		ip:           ip,
		username:     username,
		password:     password,
		state:        state,
		media:        mediaSvc,
		notifier:     notifier,
		baleNotifier: baleNotifier,
		sms:          smsSvc,
		vision:       faceDetector,
	}
}

func (l *Listener) Start(ctx context.Context) {
	isapiURL := fmt.Sprintf("http://%s/ISAPI/Event/notification/alertStream", l.ip)

	client := &http.Client{
		Transport: &digest.Transport{
			Username: l.username,
			Password: l.password,
		},
		Timeout: 0,
	}

	req, err := http.NewRequestWithContext(ctx, "GET", isapiURL, nil)
	if err != nil {
		log.Printf("Failed to create request: %v", err)
		return
	}

	log.Println("Connecting to camera event stream...")
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Connection failed: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("Camera returned status: %d", resp.StatusCode)
		return
	}

	log.Println("Successfully connected to ISAPI event stream!")

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			log.Println("Stopping ISAPI listener...")
			return
		default:
		}

		line := scanner.Text()

		// بررسی رویداد تشخیص انسان
		// بررسی رویداد تشخیص انسان
		if strings.Contains(line, "<targetType>human</targetType>") {

			// ۱. تایمر ۱۵ ثانیه‌ای را برای همه حالت‌ها (چه دزدگیر روشن، چه خاموش) اعمال می‌کنیم
			// تا در ضبط بی‌صدا هم هارد پر از عکس‌های تکراری نشود
			l.mu.Lock()
			if time.Since(l.lastTrigger) < 15*time.Second {
				l.mu.Unlock()
				continue
			}
			l.lastTrigger = time.Now()
			l.mu.Unlock()

			log.Println("Event Triggered: Human detected by the camera.")

			if l.state.IsArmed() {
				log.Println("ALERT: System is ARMED. Executing real-time parallel response...")

				// --- شاخه اول: شروع فوری ضبط ویدیو به صورت موازی ---
				go func() {
					for i := 1; i <= 2; i++ {
						videoPath, err := media.RecordVideo(l.ip, l.username, l.password, 15)
						if err != nil {
							log.Printf("Failed to record video chunk %d: %v", i, err)
							continue
						}

						go func(vp string) {
							defer os.Remove(vp)
							if err := l.notifier.SendVideoAlert(vp); err != nil {
								log.Printf("Failed to send video alert to Telegram: %v", err)
							}
							if l.baleNotifier != nil {
								l.baleNotifier.SendVideo(vp)
							}
						}(videoPath)
					}
				}()

				go func() {
					firstImages, err := l.media.CaptureBurst(ctx, 1, 0)
					if err != nil || len(firstImages) == 0 {
						log.Printf("Failed to capture initial snapshot: %v", err)
						return
					}

					faceFound := l.vision.ContainsFace(firstImages[0])
					if faceFound {
						log.Println("AI: Face detected INSTANTLY! Sending first photo...")
					} else {
						log.Println("AI: No clear face detected, but Human triggered. Sending photo anyway...")
					}

					if l.sms != nil {
						go func() {
							if l.sms.IsReady() {
								msg := "هشدار سیستم امنیتی! یک شخص در دوربین مشاهده شد. لطفا تلگرام خود را چک کنید."
								log.Println("SMS Cooldown passed. Triggering Call and SMS...")
								l.sms.SendSMS(msg)
								l.sms.MakeCall(msg)
							} else {
								log.Println("SMS throttled: 3-minute cooldown is active.")
							}
						}()
					}

					l.notifier.QueueBurstAlert(firstImages)
					if l.baleNotifier != nil {
						l.baleNotifier.QueueAlert(firstImages)
					}

					// گرفتن ۳ عکس تکمیلی برای آلبوم
					go func() {
						log.Println("Capturing additional burst photos...")
						moreImages, _ := l.media.CaptureBurst(ctx, 3, 800*time.Millisecond)
						if len(moreImages) > 0 {
							l.notifier.QueueBurstAlert(moreImages)
							if l.baleNotifier != nil {
								l.baleNotifier.QueueAlert(moreImages)
							}
						}
					}()
				}()

			} else {
				// --- سیستم غیرفعال است (DISARMED) ---
				// بررسی می‌کنیم که آیا کاربر ضبط بی‌صدا را در داشبورد/env روشن کرده است؟
				if os.Getenv("SILENT_DETECTION") == "true" {
					log.Println("🕵️ دزدگیر خاموش است اما Silent Detection فعال است. ثبت عکس‌های مخفیانه...")

					// ساخت لینک RTSP ایمن
					encodedPass := url.QueryEscape(l.password)
					rtspURL := fmt.Sprintf("rtsp://%s:%s@%s:554/Streaming/Channels/101", l.username, encodedPass, l.ip)

					// فراخوانی تابع ذخیره عکس رگباری در پوشه روزانه (که در مرحله قبل ساختیم)
					core.CaptureSilentBurst(rtspURL)
				} else {
					log.Println("INFO: System is DISARMED and Silent Detection is OFF. Ignored.")
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		log.Printf("Stream read error: %v", err)
	}
}
