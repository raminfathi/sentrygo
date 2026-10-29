package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"sentrygo/internal/config"
	"sentrygo/internal/core"
	"sentrygo/internal/isapi"
	"sentrygo/internal/media"
	"sentrygo/internal/notify"
	"sentrygo/internal/vision"
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

	// راه‌اندازی تلگرام در پس‌زمینه (بدون توقف برنامه)
	telegramService := notify.NewTelegramService(cfg.TelegramBotToken, cfg.TelegramProxy, cfg.TelegramChatIDs, 60*time.Second)

	// راه‌اندازی بله در پس‌زمینه
	var baleService *notify.BaleService
	if cfg.BaleBotToken != "" {
		baleService = notify.NewBaleService(cfg.BaleBotToken, cfg.BaleChatIDs, 60*time.Second)
	} else {
		log.Println("Bale bot token not provided, running with Telegram only.")
	}

	// --- راه‌اندازی پیامک و تماس ---
	var smsService *notify.SMSService
	if len(cfg.SMSAccounts) > 0 {
		smsService = notify.NewSMSService(cfg.SMSAccounts)

		// ارسال پیام تست در زمان روشن شدن برنامه
		// نکته: برای غیرفعال کردنش در آینده، فقط کافیه اول این خط دوتا اسلش (//) بذاری تا کامنت بشه
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

	onDemandAction := func() {
		log.Println("On-Demand action triggered via bot...")
		images, err := mediaService.CaptureBurst(context.Background(), 1, 0)
		if err == nil && len(images) > 0 {
			telegramService.QueueBurstAlert(images)
			if baleService != nil {
				baleService.QueueAlert(images)
			}
		}

		go func() {
			videoPath, err := media.RecordVideo(cfg.CameraIP, cfg.CameraUser, cfg.CameraPass, 5)
			if err != nil {
				return
			}
			defer os.Remove(videoPath)
			telegramService.SendVideoAlert(videoPath)
			if baleService != nil {
				baleService.SendVideo(videoPath)
			}
		}()
	}

	// استارت ربات‌ها (اگر وصل باشند)
	go telegramService.StartListening(systemState, onDemandAction)
	if baleService != nil {
		go baleService.StartListening(systemState, onDemandAction)
	}

	// استارت فوری دوربین بدون معطلی برای پراکسی!
	go listener.Start(ctx)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down SentryGo gracefully...")
	cancel()
	time.Sleep(1 * time.Second)
}
