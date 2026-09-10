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
	// Force the Go runtime to route proxy traffic and handle DNS properly
	os.Setenv("HTTP_PROXY", "socks5://127.0.0.1:2080")
	os.Setenv("HTTPS_PROXY", "socks5://127.0.0.1:2080")

	log.Println("Starting SentryGo Security System...")

	// Load configuration
	cfg := config.Load()

	// Initialize System State (ARMED by default)
	systemState := core.NewSystemState()
	systemState.Arm()

	// Initialize AI Vision Module (Pigo Face Detector)
	faceDetector, err := vision.NewFaceDetector("cascade/facefinder")
	if err != nil {
		log.Fatalf("Failed to initialize Face Detector: %v", err)
	}

	// Initialize Telegram Service with Proxy
	telegramService, err := notify.NewTelegramService(cfg.TelegramBotToken, cfg.TelegramProxy, cfg.TelegramChatIDs, 60*time.Second)
	if err != nil {
		log.Fatalf("Failed to initialize Telegram service: %v", err)
	}

	// Initialize Bale Service (Optional / Direct connection)
	var baleService *notify.BaleService

	if cfg.BaleBotToken != "" {
		baleService, err = notify.NewBaleService(cfg.BaleBotToken, cfg.BaleChatIDs, 60*time.Second)
		if err != nil {
			log.Printf("Warning: Failed to initialize Bale service: %v", err)
		}
	} else {
		log.Println("Bale bot token not provided, running with Telegram only.")
	}
	// Initialize SMS & Call Service
	var smsService *notify.SMSService
	if len(cfg.SMSAccounts) > 0 {
		smsService = notify.NewSMSService(cfg.SMSAccounts)
	} else {
		log.Println("SMS config missing. Running without SMS/Call support.")
	}
	// Initialize Media Service (Hikvision Snapshot)
	mediaService := media.NewService(cfg.CameraIP, cfg.CameraUser, cfg.CameraPass)

	// Initialize ISAPI Listener
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

	// Context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// تعریف منطقِ دکمه دریافت وضعیت (گرفتن همزمان عکس و فیلم)
	onDemandAction := func() {
		log.Println("On-Demand action triggered via bot...")
		// بخش اول: گرفتن یک عکس فوری و ارسال
		images, err := mediaService.CaptureBurst(context.Background(), 1, 0)
		if err == nil && len(images) > 0 {
			telegramService.QueueBurstAlert(images)
			if baleService != nil {
				baleService.QueueAlert(images)
			}
		} else {
			log.Printf("On-Demand Photo Error: %v", err)
		}

		// بخش دوم: گرفتن ۵ ثانیه فیلم و ارسال
		go func() {
			videoPath, err := media.RecordVideo(cfg.CameraIP, cfg.CameraUser, cfg.CameraPass, 5)
			if err != nil {
				log.Printf("On-Demand Video Error: %v", err)
				return
			}
			defer os.Remove(videoPath) // پاک کردن فیلم از هارد بعد از ارسال

			if err := telegramService.SendVideoAlert(videoPath); err != nil {
				log.Printf("Telegram video error: %v", err)
			}
			if baleService != nil {
				baleService.SendVideo(videoPath)
			}
		}()
	}

	// استارت زدن گوش‌دهنده‌های تلگرام و بله و پاس دادنِ تابع
	go telegramService.StartListening(systemState, onDemandAction)
	telegramService.SendControlMenu()

	if baleService != nil {
		go baleService.StartListening(systemState, onDemandAction)
		baleService.SendControlMenu()
	}

	// Start listening to Hikvision camera events
	go listener.Start(ctx)

	// Wait for termination signal (Ctrl+C)
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down SentryGo gracefully...")
	cancel()
	time.Sleep(1 * time.Second) // Give background workers a moment to clean up
}
