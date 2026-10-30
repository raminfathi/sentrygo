package isapi

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"sentrygo/internal/core"
	"sentrygo/internal/media"
	"sentrygo/internal/notify"
	"sentrygo/internal/vision"

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
	url := fmt.Sprintf("http://%s/ISAPI/Event/notification/alertStream", l.ip)

	client := &http.Client{
		Transport: &digest.Transport{
			Username: l.username,
			Password: l.password,
		},
		Timeout: 0,
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
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
		if strings.Contains(line, "<targetType>human</targetType>") {

			// جلوگیری از ارسال رگباری هشدار (حداقل ۱۵ ثانیه فاصله بین هر تشخیص)
			if !l.state.IsArmed() {
				// (اختیاری: برای اینکه ترمینال پر از لاگ نشه، این لاگ رو کامنت کن یا بذار بمونه)
				// log.Println("INFO: System is DISARMED. Ignored.")
				continue
			}

			// ۲. حالا اگر سیستم روشن بود، تایمر ۱۵ ثانیه‌ای را اعمال کن
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
					// گرفتن دو ویدیوی ۱۵ ثانیه‌ای پشت سر هم
					for i := 1; i <= 2; i++ {
						videoPath, err := media.RecordVideo(l.ip, l.username, l.password, 15)
						if err != nil {
							log.Printf("Failed to record video chunk %d: %v", i, err)
							continue
						}

						// ارسال ویدیو در یک Goroutine مستقل برای جلوگیری از توقف ضبط بعدی
						go func(vp string) {
							defer os.Remove(vp) // حذف فایل به صورت امن بعد از ارسال

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
					// فقط ۱ عکس می‌گیریم (با تاخیر 0)
					firstImages, err := l.media.CaptureBurst(ctx, 1, 0)
					if err != nil || len(firstImages) == 0 {
						log.Printf("Failed to capture initial snapshot: %v", err)
						return
					}

					// بررسی فوری چهره با متد ContainsFace
					faceFound := l.vision.ContainsFace(firstImages[0])

					if faceFound {
						log.Println("AI: Face detected INSTANTLY! Sending first photo...")

					} else {
						// تغییر مهم: عکس رو پاک نمی‌کنیم! چون دوربین "انسان" تشخیص داده
						log.Println("AI: No clear face detected in first frame, but Human triggered. Sending photo anyway...")
					}
					if l.sms != nil {
						go func() {
							// چک می‌کنیم که آیا زمان خنک‌شدن (Cooldown) پیامک تمام شده یا نه؟
							if l.sms.IsReady() {
								msg := "هشدار سیستم امنیتی! یک شخص در دوربین مشاهده شد. لطفا تلگرام خود را چک کنید."
								log.Println("SMS Cooldown passed. Triggering Call and SMS...")
								l.sms.SendSMS(msg)
								l.sms.MakeCall(msg)
							} else {
								log.Println("SMS throttled: 3-minute cooldown is active to save credit.")
							}
						}()
					}
					// ارسال فوری عکس اول (چه چهره باشه چه نباشه، چون سنسور انسان رو تایید کرده)
					l.notifier.QueueBurstAlert(firstImages)
					if l.baleNotifier != nil {
						l.baleNotifier.QueueAlert(firstImages)
					}

					// --- شاخه سوم: گرفتن ۳ عکس تکمیلی برای آلبوم ---
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
				log.Println("INFO: System is DISARMED. Logging event only, no alerts sent.")
			}
		}
	}

	if err := scanner.Err(); err != nil {
		log.Printf("Stream read error: %v", err)
	}
}
