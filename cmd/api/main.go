package main

import (
	"context"
	"log"
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
	systemState := core.NewSystemState()
	systemState.Arm()

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

			videoPath, err := media.RecordVideo(cfg.CameraIP, cfg.CameraUser, cfg.CameraPass, 25)
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

			videoPath, err := media.RecordVideo(cfg.CameraIP, cfg.CameraUser, cfg.CameraPass, 25)
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
		// ۱. اضافه شدن پرانتز برای خواندن وضعیت
		func() bool { return systemState.IsArmed() },

		// ۲. تغییر منطق دکمه برای سوئیچ کردن وضعیت
		func() bool {
			currentState := systemState.IsArmed()
			newState := !currentState
			systemState.SetArmed(newState)

			// ایجاد متن پیام
			msg := "🟢 سیستم دزدگیر غیرفعال شد (Disarmed)"
			if newState {
				msg = "🔴 سیستم دزدگیر فعال شد (Armed)"
			}

			// ارسال پیام به ربات‌ها (نام متغیرهای ربات خود را اینجا قرار دهید)
			telegramService.SendMarkdownMessage(msg)
			baleService.SendMessage(msg)

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
