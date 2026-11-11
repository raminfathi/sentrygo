package core

import (
	"context" // کتابخانه جدید
	"log"
	"os"
	"os/exec"
	"path/filepath"
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
func StartAutoCleanup(saveDir string, retentionDays int) {
	if retentionDays <= 0 {
		log.Println("⚠️ سیستم پاکسازی خودکار غیرفعال است (RetentionDays <= 0)")
		return
	}

	go func() {
		for {
			log.Printf("🧹 شروع بررسی و پاکسازی ویدیوهای قدیمی‌تر از %d روز...", retentionDays)

			// محاسبه زمانی که فایل‌ها قبل از آن باید حذف شوند
			cutoffTime := time.Now().AddDate(0, 0, -retentionDays)
			deletedCount := 0

			// اسکن پوشه ضبط
			err := filepath.Walk(saveDir, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return nil // در صورت خطا، از این فایل بگذر
				}

				// فقط فایل‌های mkv یا mp4 را بررسی کن (نه پوشه‌ها را)
				if !info.IsDir() && (strings.HasSuffix(info.Name(), ".mkv") || strings.HasSuffix(info.Name(), ".mp4")) {
					// اگر تاریخ تغییر فایل قدیمی‌تر از زمان تعیین شده بود
					if info.ModTime().Before(cutoffTime) {
						if err := os.Remove(path); err == nil {
							deletedCount++
						} else {
							log.Printf("❌ خطا در حذف فایل قدیمی: %s", path)
						}
					}
				}
				return nil
			})

			if err != nil {
				log.Printf("❌ خطا در اسکن پوشه برای پاکسازی: %v", err)
			} else if deletedCount > 0 {
				log.Printf("✅ عملیات پاکسازی تمام شد. %d فایل قدیمی حذف شد.", deletedCount)
			}

			// صبر برای 1 ساعت قبل از بررسی مجدد
			time.Sleep(1 * time.Hour)
		}
	}()
}
