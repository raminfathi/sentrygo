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
	telegramService, err := notify.NewTelegramService(cfg.TelegramBotToken, cfg.TelegramProxy, cfg.TelegramChatID, 60*time.Second)
	if err != nil {
		log.Fatalf("Failed to initialize Telegram service: %v", err)
	}

	// Initialize Bale Service (Optional / Direct connection)
	var baleService *notify.BaleService
	if cfg.BaleBotToken != "" {
		baleService, err = notify.NewBaleService(cfg.BaleBotToken, cfg.BaleChatID, 60*time.Second)
		if err != nil {
			log.Printf("Warning: Failed to initialize Bale service: %v", err)
		}
	} else {
		log.Println("Bale bot token not provided, running with Telegram only.")
	}

	// Initialize Media Service (Hikvision Snapshot)
	mediaService := media.NewService(cfg.CameraIP, cfg.CameraUser, cfg.CameraPass)

	// Initialize ISAPI Listener (Passing both notification services and AI)
	// Initialize ISAPI Listener
	listener := isapi.NewListener(
		cfg.CameraIP,
		cfg.CameraUser,
		cfg.CameraPass,
		systemState,
		mediaService,
		telegramService,
		baleService,
		faceDetector,
	)
	// Context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

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
