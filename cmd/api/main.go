package main

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/raminfathi/sentrygo/internal/config"
	"github.com/raminfathi/sentrygo/internal/core"
	"github.com/raminfathi/sentrygo/internal/isapi"
	"github.com/raminfathi/sentrygo/internal/media"
	"github.com/raminfathi/sentrygo/internal/notify"
	"github.com/raminfathi/sentrygo/internal/vision"
	"github.com/raminfathi/sentrygo/internal/web"
)

func main() {
	log.Println("Starting SentryGo Security System...")

	cfg := config.Load()
	encodedPass := url.QueryEscape(cfg.CameraPass)
	rtspURL := fmt.Sprintf("rtsp://%s:%s@%s:554/Streaming/Channels/101",
		cfg.CameraUser,
		encodedPass,
		cfg.CameraIP,
	)

	recPath := cfg.RecordingPath
	if recPath == "" {
		recPath = "./archive/records"
	}
	core.StartContinuousRecording(rtspURL, recPath)
	core.StartAutoCleanup(recPath, cfg.RetentionDays)
	systemState := core.NewSystemState()
	systemState.Arm("سیستم (راه‌اندازی اولیه)") // پاس دادن مقدار پیش‌فرض

	faceDetector, err := vision.NewFaceDetector("cascade/facefinder")
	if err != nil {
		log.Fatalf("Failed to initialize Face Detector: %v", err)
	}

	// راه‌اندازی تلگرام
	telegramService := notify.NewTelegramService(cfg.TelegramBotToken, cfg.TelegramProxy, cfg.TelegramChatIDs, 60*time.Second)

	// راه‌اندازی بله
	var baleService *notify.BaleService
	if cfg.BaleBotToken != "" {
		baleService = notify.NewBaleService(cfg.BaleBotToken, cfg.BaleChatIDs, 60*time.Second)
	} else {
		log.Println("Bale bot token not provided, running with Telegram only.")
	}

	// راه‌اندازی پیامک
	var smsService *notify.SMSService
	if len(cfg.SMSAccounts) > 0 {
		smsService = notify.NewSMSService(cfg.SMSAccounts)
		smsService.SendStartupMessage()
	} else {
		log.Println("SMS config missing. Running without SMS/Call support.")
	}

	mediaService := media.NewService(cfg.CameraIP, cfg.CameraUser, cfg.CameraPass)

	listener := isapi.NewListener(
		cfg.CameraIP,
		cfg.CameraUser,
		cfg.CameraPass,
		systemState,
		mediaService,
		telegramService,
		baleService,
		smsService,
		faceDetector,
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// --- توابع اختصاصی برای درخواست فیلم لحظه‌ای ---
	// افزایش زمان ضبط از 5 به 25 ثانیه برای پوشش کامل اتفاقات

	onDemandTelegram := func(chatID int64) {
		log.Printf("Telegram on-demand video requested by ChatID: %d", chatID)
		go func() {
			// سیستم ضد-کرش: اگر خطایی پیش آمد برنامه بسته نشود
			defer func() {
				if r := recover(); r != nil {
					log.Println("Recovered from panic in onDemandTelegram:", r)
				}
			}()

			videoPath, err := media.RecordVideo(cfg.CameraIP, cfg.CameraUser, cfg.CameraPass, 10)
			if err != nil {
				log.Println("Error recording video:", err)
				return
			}
			defer os.Remove(videoPath)

			// ارسال فیلم فقط به کاربری که درخواست داده با کپشن اختصاصی
			telegramService.SendVideoToUser(chatID, videoPath, true)
		}()
	}

	onDemandBale := func(chatID int64) {
		log.Printf("Bale on-demand video requested by ChatID: %d", chatID)
		go func() {
			defer func() {
				if r := recover(); r != nil {
					log.Println("Recovered from panic in onDemandBale:", r)
				}
			}()

			videoPath, err := media.RecordVideo(cfg.CameraIP, cfg.CameraUser, cfg.CameraPass, 10)
			if err != nil {
				log.Println("Error recording video:", err)
				return
			}
			defer os.Remove(videoPath)

			if baleService != nil {
				baleService.SendVideoToUser(chatID, videoPath, true)
			}
		}()
	}

	go telegramService.StartListening(systemState, onDemandTelegram)
	if baleService != nil {
		go baleService.StartListening(systemState, onDemandBale)
	}

	webPort := os.Getenv("WEB_PORT")
	if webPort == "" {
		webPort = ":8080"
	}
	webServer := web.NewServer(webPort,
		// تابع خواندن وضعیت فعلی
		func() bool { return systemState.IsArmed() },

		// تابع تغییر وضعیت دزدگیر از روی وب
		func() bool {
			currentState := systemState.IsArmed()
			newState := !currentState

			// تعیین منبع تغییر دهنده
			source := "داشبورد وب (Admin Panel)"

			if newState {
				systemState.Arm(source) // استفاده از تابع جدید با ورودی نام
				msg := fmt.Sprintf("🔴 سیستم امنیتی توسط **%s** **فعال** شد.", source)
				telegramService.SendMarkdownMessage(msg)
				// baleService.SendMessage(msg) // اگر ربات بله هم فعال است، این خط را از کامنت در بیاورید
			} else {
				systemState.Disarm(source) // استفاده از تابع جدید با ورودی نام
				msg := fmt.Sprintf("🟢 سیستم امنیتی توسط **%s** **غیرفعال** شد.", source)
				telegramService.SendMarkdownMessage(msg)
				// baleService.SendMessage(msg) // اگر ربات بله هم فعال است، این خط را از کامنت در بیاورید
			}

			return newState
		},
	)
	go webServer.Start()
	go listener.Start(ctx)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down SentryGo gracefully...")
	cancel()
	time.Sleep(1 * time.Second)
}
