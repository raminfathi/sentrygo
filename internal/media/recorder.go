package media

import (
	"bytes"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// RecordVideo connects to the RTSP stream and records a short video clip.
func RecordVideo(cameraIP, username, password string, durationSeconds int) (string, error) {
	if err := os.MkdirAll("snapshots", os.ModePerm); err != nil {
		return "", fmt.Errorf("failed to create directory: %w", err)
	}

	filename := fmt.Sprintf("snapshots/alert_%s.mp4", time.Now().Format("20060102_150405"))
	absPath, err := filepath.Abs(filename)
	if err != nil {
		absPath = filename
	}

	// URL Encode the password to safely handle special characters like '@'
	safePassword := url.QueryEscape(password)

	// ساخت URL استریم RTSP در هایک‌ویژن
	rtspURL := fmt.Sprintf("rtsp://%s:%s@%s:554/Streaming/Channels/101", username, safePassword, cameraIP)

	durationArg := fmt.Sprintf("%d", durationSeconds)
	cmd := exec.Command("ffmpeg",
		"-y",
		"-rtsp_transport", "tcp",
		"-i", rtspURL,
		"-t", durationArg,
		"-c:v", "copy", "-an",
		"-c:a", "aac", // صدا را به فرمت استاندارد MP4 تبدیل کن
		absPath,
	)

	// گرفتن لاگ‌های خطای FFmpeg برای دیباگ دقیق‌تر
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	log.Printf("FFmpeg: Starting video recording for %d seconds...", durationSeconds)

	if err := cmd.Run(); err != nil {
		// حالا اگر خطا بدهد، دلیل دقیق آن را در ترمینال می‌بینیم
		return "", fmt.Errorf("ffmpeg recording failed: %v | Details: %s", err, stderr.String())
	}

	log.Printf("FFmpeg: Video successfully recorded: %s", absPath)
	return absPath, nil
}
