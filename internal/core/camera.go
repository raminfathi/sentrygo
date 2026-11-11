package core

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"time"
)

// CaptureSilentBurst سه عکس سریع در زمان تشخیص حرکت می‌گیرد
func CaptureSilentBurst(rtspURL string) {
	// ۱. ساخت پوشه بر اساس تاریخ امروز
	today := time.Now().Format("2006-01-02") // خروجی: 2026-11-11
	saveDir := fmt.Sprintf("archive/%s", today)

	if err := os.MkdirAll(saveDir, 0755); err != nil {
		log.Printf("❌ خطا در ساخت پوشه روزانه: %v", err)
		return
	}

	// ۲. الگوی نام‌گذاری عکس‌ها (مثال: event_15-30-22_01.jpg)
	timestamp := time.Now().Format("15-04-05")
	outputPattern := fmt.Sprintf("%s/event_%s_%%02d.jpg", saveDir, timestamp)

	// اجرای این فرآیند در پس‌زمینه تا برنامه متوقف نشود
	go func() {
		// دستور FFmpeg برای استخراج ۳ فریم با سرعت ۳ فریم بر ثانیه
		cmd := exec.Command("ffmpeg", "-y", "-hide_banner", "-loglevel", "error",
			"-rtsp_transport", "tcp",
			"-i", rtspURL,
			"-vframes", "3", // تعداد کل عکس‌ها
			"-r", "3", // سرعت استخراج (۳ عکس در ثانیه)
			"-q:v", "2", // کیفیت بالا برای عکس
			outputPattern,
		)

		if err := cmd.Run(); err != nil {
			log.Printf("❌ خطا در ثبت عکس‌های سریع (Silent): %v", err)
		} else {
			log.Printf("📸 ۳ عکس از حرکت تشخیص داده شده در پوشه %s ذخیره شد.", today)
		}
	}()
}
