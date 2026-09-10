package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"sentrygo/internal/config"
	"sentrygo/internal/core"
	"sentrygo/internal/isapi"
	"sentrygo/internal/media"
	"sentrygo/internal/notify"
	"sentrygo/internal/vision"
)

func main() {
	os.Setenv("HTTP_PROXY", "socks5://127.0.0.1:2080")
	os.Setenv("HTTPS_PROXY", "socks5://127.0.0.1:2080")
	log.Println("Starting SentryGo Security System...")

	// Load configuration from .env file
	cfg := config.Load()

	systemState := core.NewSystemState()
	systemState.SetArmed(true)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	faceDetector, err := vision.NewFaceDetector("cascade/facefinder")
	if err != nil {
		log.Fatalf("Failed to initialize Face Detector: %v", err)
	}
	mediaService := media.NewService(cfg.CameraIP, cfg.CameraUser, cfg.CameraPass)

	telegramService, err := notify.NewTelegramService(cfg.TelegramBotToken, cfg.ProxyURL, cfg.TelegramChatID, cfg.AlertCooldownSeconds)
	if err != nil {
		log.Fatalf("Failed to initialize Telegram service: %v", err)
	}

	listener := isapi.NewListener(cfg.CameraIP, cfg.CameraUser, cfg.CameraPass, systemState, mediaService, telegramService, faceDetector)
	go listener.Start(ctx)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down SentryGo gracefully...")
	cancel()
}
