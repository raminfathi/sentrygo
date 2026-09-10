package notify

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"sentrygo/internal/core"
	"strings"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type TelegramService struct {
	bot      *tgbotapi.BotAPI
	chatIDs  []int64 // لیست آیدی ادمین‌ها
	cooldown time.Duration
	lastSent time.Time
	mu       sync.Mutex
	queue    chan []string // صف آفلاین برای نگهداری مسیر عکس‌ها
}

// بررسی می‌کند که آیا فرستنده پیام در لیست ادمین‌ها هست یا خیر
func (t *TelegramService) isAuthorized(id int64) bool {
	for _, authID := range t.chatIDs {
		if id == authID {
			return true
		}
	}
	return false
}

// NewTelegramService initializes the Telegram bot and starts the background worker.
func NewTelegramService(token, proxyURL string, chatIDs []int64, cooldown time.Duration) (*TelegramService, error) {
	var client *http.Client

	if proxyURL != "" {
		parsedURL, err := url.Parse(proxyURL)
		if err != nil {
			return nil, fmt.Errorf("invalid proxy url: %w", err)
		}
		client = &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(parsedURL)}}
		log.Printf("Connecting to Telegram using proxy: %s", proxyURL)
	} else {
		client = &http.Client{}
	}

	bot, err := tgbotapi.NewBotAPIWithClient(token, tgbotapi.APIEndpoint, client)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to telegram: %w", err)
	}

	log.Printf("Authorized on Telegram account: %s", bot.Self.UserName)

	svc := &TelegramService{
		bot:      bot,
		chatIDs:  chatIDs,
		cooldown: cooldown,
		queue:    make(chan []string, 50), // ظرفیت صف آفلاین
	}

	// استارت کردن کارگر پس‌زمینه به صورت ناهمگام
	go svc.startWorker()

	return svc, nil
}

// QueueBurstAlert adds the alert to the channel instead of blocking the main thread.
func (t *TelegramService) QueueBurstAlert(imagePaths []string) {
	t.mu.Lock()
	if time.Since(t.lastSent) < t.cooldown {
		t.mu.Unlock()
		log.Println("Alert throttled: Cooldown active, dropping alert.")
		// پاکسازی عکس‌ها برای جلوگیری از پر شدن هارد
		for _, path := range imagePaths {
			os.Remove(path)
		}
		return
	}
	t.lastSent = time.Now()
	t.mu.Unlock()

	select {
	case t.queue <- imagePaths:
		log.Println("Alert added to offline queue. Worker will process it.")
	default:
		log.Println("CRITICAL: Queue is full! Dropping alert.")
	}
}

// startWorker constantly checks the queue and retries on network failure.
func (t *TelegramService) startWorker() {
	for paths := range t.queue {
		for {
			err := t.sendAlbum(paths)
			if err == nil {
				log.Println("Telegram burst alert (album) sent successfully to all admins.")
				// پاکسازی موفق عکس‌ها از روی هارد
				for _, path := range paths {
					os.Remove(path)
				}
				break // خروج از حلقه تلاش مجدد و رفتن به پیام بعدی
			}

			log.Printf("Network error, retrying in 10 seconds... Error: %v", err)
			time.Sleep(10 * time.Second)
		}
	}
}

func (t *TelegramService) sendAlbum(imagePaths []string) error {
	if len(imagePaths) == 0 {
		return nil
	}

	var lastErr error

	// ارسال آلبوم برای تک تک ادمین‌ها
	for _, chatID := range t.chatIDs {
		// حالت اول: اگر فقط یک عکس داریم (ارسال فوریِ ثانیه صفر)
		if len(imagePaths) == 1 {
			photo := tgbotapi.NewPhoto(chatID, tgbotapi.FilePath(imagePaths[0]))
			photo.Caption = "🚨 **فوری:** یک شخص رویت شد!\n🕒 زمان: " + time.Now().Format("2006-01-02 15:04:05")
			photo.ParseMode = "Markdown"
			if _, err := t.bot.Send(photo); err != nil {
				log.Printf("Failed to send photo to %d: %v", chatID, err)
				lastErr = err
			}
			continue
		}

		// حالت دوم: اگر چند عکس داریم (آلبوم تکمیلی)
		var mediaGroup []interface{}
		for i, path := range imagePaths {
			photo := tgbotapi.NewInputMediaPhoto(tgbotapi.FilePath(path))
			if i == 0 {
				photo.Caption = "📸 تصاویر تکمیلی از زوایای دیگر\n🕒 زمان: " + time.Now().Format("2006-01-02 15:04:05")
			}
			mediaGroup = append(mediaGroup, photo)
		}

		msg := tgbotapi.MediaGroupConfig{
			ChatID: chatID,
			Media:  mediaGroup,
		}

		if _, err := t.bot.SendMediaGroup(msg); err != nil {
			log.Printf("Failed to send album to %d: %v", chatID, err)
			lastErr = err
		}
	}

	return lastErr
}

// SendVideoAlert sends a recorded video clip to all Telegram admins.
func (t *TelegramService) SendVideoAlert(videoPath string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	var lastErr error
	for _, chatID := range t.chatIDs {
		video := tgbotapi.NewVideo(chatID, tgbotapi.FilePath(videoPath))
		video.Caption = fmt.Sprintf("🎥 VIDEO ALERT: Intrusion Detected!\n🕒 Time: %s", time.Now().Format("2006-01-02 15:04:05"))

		if _, err := t.bot.Send(video); err != nil {
			log.Printf("Failed to send video to %d: %v", chatID, err)
			lastErr = err
		} else {
			log.Printf("Telegram video alert sent successfully to %d.", chatID)
		}
	}

	return lastErr
}

// تابع کمکی برای ارسال پیام‌های متنی ساده به تمامی ادمین‌ها در تلگرام
func (t *TelegramService) sendMarkdownMessage(text string) {
	for _, chatID := range t.chatIDs {
		msg := tgbotapi.NewMessage(chatID, text)
		msg.ParseMode = "Markdown"
		t.bot.Send(msg)
	}
}

// --- بخش کنترل و تعامل دوطرفه ---
func (t *TelegramService) StartListening(state *core.SystemState, onDemand func()) {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := t.bot.GetUpdatesChan(u)

	for update := range updates {
		if update.CallbackQuery != nil {
			// چک کردن اینکه آیا فرد کلیک‌کننده، ادمین است یا خیر
			if !t.isAuthorized(update.CallbackQuery.Message.Chat.ID) {
				continue
			}

			callback := tgbotapi.NewCallback(update.CallbackQuery.ID, "")

			switch update.CallbackQuery.Data {
			case "arm":
				state.Arm()
				callback.Text = "سیستم مسلح شد! 🔴"
				t.sendMarkdownMessage("🔴 سیستم امنیتی **فعال** شد. (هشدارها روشن)")
			case "disarm":
				state.Disarm()
				callback.Text = "سیستم غیرفعال شد! 🟢"
				t.sendMarkdownMessage("🟢 سیستم امنیتی **غیرفعال** شد. (هشدارها خاموش)")
			case "status":
				callback.Text = "در حال دریافت عکس و فیلم..."
				t.sendMarkdownMessage("⏳ در حال ارتباط با دوربین برای دریافت عکس و فیلم فعلی...")
				go onDemand() // اجرای تابع گرفتن عکس و فیلم
			}

			t.bot.Request(callback)
			continue
		}

		if update.Message != nil {
			// چک کردن دسترسی برای پیام‌های متنی
			if !t.isAuthorized(update.Message.Chat.ID) {
				continue
			}
			text := strings.ToLower(update.Message.Text)
			if update.Message.Command() == "start" || update.Message.Command() == "menu" || text == "منو" || text == "menu" {
				t.SendControlMenu()
			}
		}
	}
}

// SendControlMenu دکمه‌ها را برای تمامی ادمین‌ها می‌فرستد
func (t *TelegramService) SendControlMenu() {
	for _, chatID := range t.chatIDs {
		msg := tgbotapi.NewMessage(chatID, "🛡 **کنترل پنل SentryGo**\nوضعیت سیستم را انتخاب کنید:")
		msg.ParseMode = "Markdown"

		keyboard := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("🔴 خونه نیستم (فعال)", "arm"),
				tgbotapi.NewInlineKeyboardButtonData("🟢 خونه هستم (غیرفعال)", "disarm"),
			),
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("📸🎥 ارسال عکس و فیلم الان", "status"),
			),
		)
		msg.ReplyMarkup = keyboard
		t.bot.Send(msg)
	}
}
