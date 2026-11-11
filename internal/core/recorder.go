package core

import (
	"context" // کتابخانه جدید
	"log"
	"os"
	"os/exec"
	"strings"
	"time"
)

// متغیر سراسری برای لغو عملیات ضبط
var cancelRecording context.CancelFunc

func StartContinuousRecording(rtspURL, saveDir string) {
	if err := os.MkdirAll(saveDir, 0755); err != nil {
		log.Printf("❌ خطا در ساخت پوشه ضبط: %v", err)
		return
	}

	// ایجاد یک Context برای مدیریت توقف پروسه
	ctx, cancel := context.WithCancel(context.Background())
	cancelRecording = cancel

	go func() {
		for {
			select {
			case <-ctx.Done():
				log.Println("🛑 فرآیند ضبط ۲۴ ساعته متوقف شد.")
				return // خروج کامل از حلقه ضبط
			default:
				log.Println("🎥 در حال شروع ضبط ۲۴ ساعته دوربین...")

				cleanSaveDir := strings.ReplaceAll(saveDir, "\\", "/")
				cleanSaveDir = strings.TrimSuffix(cleanSaveDir, "/")
				outputPattern := cleanSaveDir + "/record_%Y-%m-%d_%H-%M-%S.mkv"

				// اجرای FFmpeg با استفاده از Context
				cmd := exec.CommandContext(ctx, "ffmpeg",
					"-rtsp_transport", "tcp",
					"-i", rtspURL,
					"-c:v", "copy",
					"-an",
					"-f", "segment",
					"-segment_time", "900",
					"-reset_timestamps", "1",
					"-strftime", "1",
					outputPattern,
				)

				cmd.Stderr = os.Stderr

				err := cmd.Run()
				if ctx.Err() != nil {
					return // اگر دلیل خروج، ری‌استارت دستی بوده است
				}
				log.Printf("⚠️ قطعی در ضبط ۲۴ ساعته: %v. تلاش مجدد...", err)
				time.Sleep(5 * time.Second)
			}
		}
	}()
}

// تابع جدید برای فراخوانی هنگام ری‌استارت
func StopContinuousRecording() {
	if cancelRecording != nil {
		cancelRecording()
		time.Sleep(2 * time.Second) // ۲ ثانیه صبر برای بسته شدن کامل فایل mkv
	}
}
