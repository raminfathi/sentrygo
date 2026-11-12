package web

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/raminfathi/sentrygo/internal/core"
)

type Server struct {
	port         string
	checkStatus  func() bool
	toggleStatus func() bool
}

func NewServer(port string, check func() bool, toggle func() bool) *Server {
	if port == "" {
		port = ":8080"
	}
	return &Server{port: port, checkStatus: check, toggleStatus: toggle}
}

func (s *Server) Start() {
	// پوشاندن تمام APIها با لایه امنیتی
	http.HandleFunc("/", s.basicAuth(s.handleIndex))
	http.HandleFunc("/api/settings", s.basicAuth(s.handleSettings))
	http.HandleFunc("/api/stream", s.basicAuth(s.handleStream))
	http.HandleFunc("/api/action/snapshot", s.basicAuth(s.handleSnapshot))
	http.HandleFunc("/api/action/record", s.basicAuth(s.handleRecord))
	http.HandleFunc("/api/archive", s.basicAuth(s.handleArchiveList))
	http.HandleFunc("/api/restart", s.basicAuth(s.handleRestart))
	http.HandleFunc("/api/status", s.basicAuth(s.handleStatus))

	// قفل کردن دسترسی مستقیم به فایل‌های پوشه archive
	archiveHandler := http.StripPrefix("/archive/", http.FileServer(http.Dir("archive")))
	http.HandleFunc("/archive/", s.basicAuth(func(w http.ResponseWriter, r *http.Request) {
		archiveHandler.ServeHTTP(w, r)
	}))

	log.Printf("🌐 Web Dashboard running at http://localhost%s", s.port)
	if err := http.ListenAndServe(s.port, nil); err != nil {
		log.Fatalf("Web server failed: %v", err)
	}
}
func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	tmpl, err := template.ParseFiles("templates/index.html")
	if err != nil {
		http.Error(w, "Error loading dashboard: "+err.Error(), http.StatusInternalServerError)
		return
	}
	tmpl.Execute(w, nil)
}

// ---------------------------------------------------------
// سیستم ارسال فوری فریم‌ها برای جلوگیری از بافر شدن ویدیو در Go
// ---------------------------------------------------------
type flushWriter struct {
	w http.ResponseWriter
	f http.Flusher
}

func (fw *flushWriter) Write(p []byte) (n int, err error) {
	n, err = fw.w.Write(p)
	if fw.f != nil {
		fw.f.Flush() // شلیک فوری اطلاعات به مرورگر کاربر
	}
	return
}

func (s *Server) handleStream(w http.ResponseWriter, r *http.Request) {
	cameraIP := os.Getenv("CAMERA_IP")
	cameraUser := os.Getenv("CAMERA_USER")
	cameraPass := os.Getenv("CAMERA_PASS")

	encodedPass := url.QueryEscape(cameraPass)
	streamURL := fmt.Sprintf("rtsp://%s:%s@%s:554/Streaming/Channels/101", cameraUser, encodedPass, cameraIP)

	// کلمه boundary باید دقیقاً با استانداردی که FFmpeg می‌فرستد یکی باشد
	w.Header().Set("Content-Type", "multipart/x-mixed-replace; boundary=ffmpeg")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Connection", "close")

	flusher, ok := w.(http.Flusher)
	if ok {
		flusher.Flush()
	}

	// استفاده از FlushWriter اختصاصی که بالا ساختیم
	fw := &flushWriter{w: w, f: flusher}

	cmd := exec.Command("ffmpeg",
		"-rtsp_transport", "tcp",
		"-fflags", "nobuffer",
		"-i", streamURL,
		"-f", "mpjpeg", // خروجی استاندارد استریم وب
		"-q:v", "7", // کیفیت تصویر
		"-r", "15", // فریم‌ریت منطقی برای وب
		"-",
	)

	// حالا خروجی به جای w، به FlushWriter ما متصل می‌شود
	cmd.Stdout = fw

	// اجرای استریم (تا زمانی که کاربر صفحه را ببندد اجرا می‌ماند)
	if err := cmd.Run(); err != nil {
		log.Println("Stream connection closed by user or error:", err)
	}
}

// ---------------------------------------------------------
// مدیریت تنظیمات .env
// ---------------------------------------------------------

func updateEnvFile(updates map[string]string) {
	content, err := os.ReadFile(".env")
	if err != nil {
		log.Println("Error reading .env file:", err)
		return
	}

	lines := strings.Split(string(content), "\n")
	for i, line := range lines {
		for key, val := range updates {
			if strings.HasPrefix(strings.TrimSpace(line), key+"=") {
				lines[i] = key + "=" + val
			}
		}
	}
	os.WriteFile(".env", []byte(strings.Join(lines, "\n")), 0644)
}
func (s *Server) handleSnapshot(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	cameraIP := os.Getenv("CAMERA_IP")
	cameraUser := os.Getenv("CAMERA_USER")
	cameraPass := url.QueryEscape(os.Getenv("CAMERA_PASS"))
	streamURL := fmt.Sprintf("rtsp://%s:%s@%s:554/Streaming/Channels/101", cameraUser, cameraPass, cameraIP)

	os.MkdirAll("archive", os.ModePerm)
	fileName := fmt.Sprintf("archive/snap_%s.jpg", time.Now().Format("15-04-05"))

	cmd := exec.Command("ffmpeg", "-y", "-rtsp_transport", "tcp", "-i", streamURL, "-vframes", "1", "-q:v", "2", fileName)

	if err := cmd.Run(); err != nil {
		json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "خطا در ثبت عکس"})
		return
	}
	json.NewEncoder(w).Encode(map[string]string{"status": "success", "message": "عکس با موفقیت ذخیره شد"})
}

func (s *Server) handleRecord(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	cameraIP := os.Getenv("CAMERA_IP")
	cameraUser := os.Getenv("CAMERA_USER")
	cameraPass := url.QueryEscape(os.Getenv("CAMERA_PASS"))
	streamURL := fmt.Sprintf("rtsp://%s:%s@%s:554/Streaming/Channels/101", cameraUser, cameraPass, cameraIP)

	os.MkdirAll("archive", os.ModePerm)
	fileName := fmt.Sprintf("archive/vid_%s.mp4", time.Now().Format("15-04-05"))

	// اجرای ضبط در پس‌زمینه تا سایت برای ۲۵ ثانیه قفل نشود
	go func() {
		cmd := exec.Command("ffmpeg", "-y", "-rtsp_transport", "tcp", "-i", streamURL, "-t", "10", "-c:v", "copy", "-an", fileName)
		cmd.Run()
	}()

	json.NewEncoder(w).Encode(map[string]string{"status": "success", "message": "ضبط ۲۵ ثانیه‌ای شروع شد"})
}

func (s *Server) handleArchiveList(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	files, err := os.ReadDir("archive")
	if err != nil {
		json.NewEncoder(w).Encode([]map[string]string{})
		return
	}

	var fileList []map[string]string
	for _, f := range files {
		if f.IsDir() {
			continue
		}

		fileType := "image"
		if strings.HasSuffix(f.Name(), ".mp4") {
			fileType = "video"
		}

		fileList = append(fileList, map[string]string{
			"name": f.Name(),
			"url":  "/archive/" + f.Name(),
			"type": fileType,
		})
	}

	// برعکس کردن لیست برای اینکه عکس‌های جدیدتر اول نمایش داده شوند
	for i, j := 0, len(fileList)-1; i < j; i, j = i+1, j-1 {
		fileList[i], fileList[j] = fileList[j], fileList[i]
	}

	json.NewEncoder(w).Encode(fileList)
}
func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method == http.MethodPost {
		newState := s.toggleStatus()
		json.NewEncoder(w).Encode(map[string]bool{"armed": newState})
		return
	}
	json.NewEncoder(w).Encode(map[string]bool{"armed": s.checkStatus()})
}
func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method == http.MethodGet {
		settings := map[string]string{
			"CAMERA_IP":           os.Getenv("CAMERA_IP"),
			"CAMERA_USER":         os.Getenv("CAMERA_USER"),
			"CAMERA_PASS":         os.Getenv("CAMERA_PASS"),
			"SMS_ACCOUNTS":        os.Getenv("SMS_ACCOUNTS"), // تغییر به متغیر شما
			"SMS_PATTERN_CODE":    os.Getenv("SMS_PATTERN_CODE"),
			"SMS_STARTUP_PATTERN": os.Getenv("SMS_STARTUP_PATTERN"),
			"SMS_PHONES":          os.Getenv("SMS_PHONES"), // اگر در env ندارید اضافه کنید
			"TELEGRAM_BOT_TOKEN":  os.Getenv("TELEGRAM_BOT_TOKEN"),
			"BALE_BOT_TOKEN":      os.Getenv("BALE_BOT_TOKEN"),
		}
		json.NewEncoder(w).Encode(settings)
		return
	}

	if r.Method == http.MethodPost {
		var newSettings map[string]string
		if err := json.NewDecoder(r.Body).Decode(&newSettings); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		log.Printf("📥 داده‌های دریافت شده از سایت: %v", newSettings)
		updateEnvFile(newSettings)
		for key, val := range newSettings {
			os.Setenv(key, val)
		}
		json.NewEncoder(w).Encode(map[string]string{"status": "success", "message": "تنظیمات با موفقیت ذخیره شد"})
	}
}

// تابع ری‌استارت سیستم
func (s *Server) handleRestart(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"message": "سیستم در حال راه‌اندازی مجدد است..."})

		// یک ثانیه تاخیر برای رسیدن پیام به مرورگر، سپس توقف برنامه
		go func() {
			core.StopContinuousRecording()
			time.Sleep(1 * time.Second)
			os.Exit(0) // در لینوکس، سرویس systemd به صورت خودکار برنامه را دوباره روشن می‌کند
		}()
	}
}

// لایه امنیتی برای بررسی نام کاربری و رمز عبور مرورگر
func (s *Server) basicAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()

		expectedUser := os.Getenv("DASHBOARD_USER")
		expectedPass := os.Getenv("DASHBOARD_PASS")

		if expectedUser == "" {
			expectedUser = "admin"
		}
		if expectedPass == "" {
			expectedPass = "admin"
		}

		if !ok || user != expectedUser || pass != expectedPass {
			w.Header().Set("WWW-Authenticate", `Basic realm="SentryGo Security Dashboard"`)
			http.Error(w, "Unauthorized Access", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	}
}
