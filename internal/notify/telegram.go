package notify

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/raminfathi/sentrygo/internal/core"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type TelegramService struct {
	bot         *tgbotapi.BotAPI
	chatIDs     []int64
	cooldown    time.Duration
	lastSent    time.Time
	mu          sync.Mutex
	queue       chan []string
	isConnected bool
}

func (t *TelegramService) isAuthorized(id int64) bool {
	for _, authID := range t.chatIDs {
		if id == authID {
			return true
		}
	}
	return false
}

func NewTelegramService(token, proxyURL string, chatIDs []int64, cooldown time.Duration) *TelegramService {
	svc := &TelegramService{
		chatIDs:  chatIDs,
		cooldown: cooldown,
		queue:    make(chan []string, 50),
	}
	go svc.connectLoop(token, proxyURL)
	return svc
}

func (t *TelegramService) connectLoop(token, proxyURL string) {
	var client *http.Client
	if proxyURL != "" {
		parsedURL, _ := url.Parse(proxyURL)
		client = &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(parsedURL)}}
	} else {
		client = &http.Client{}
	}

	for {
		bot, err := tgbotapi.NewBotAPIWithClient(token, tgbotapi.APIEndpoint, client)
		if err == nil {
			log.Printf("Authorized on Telegram account: %s", bot.Self.UserName)
			t.mu.Lock()
			t.bot = bot
			t.isConnected = true
			t.mu.Unlock()
			go t.startWorker()
			t.SendControlMenu()
			return
		}
		log.Printf("Telegram Proxy down, retrying in 10s...")
		time.Sleep(10 * time.Second)
	}
}

func (t *TelegramService) QueueBurstAlert(imagePaths []string) {
	t.mu.Lock()
	if time.Since(t.lastSent) < t.cooldown {
		t.mu.Unlock()
		for _, path := range imagePaths {
			os.Remove(path)
		}
		return
	}
	t.lastSent = time.Now()
	t.mu.Unlock()

	select {
	case t.queue <- imagePaths:
		log.Println("Alert added to offline queue. Telegram will process it when connected.")
	default:
	}
}

func (t *TelegramService) startWorker() {
	for paths := range t.queue {
		for {
			err := t.sendAlbum(paths)
			if err == nil {
				go func(files []string) {
					time.Sleep(5 * time.Minute)
					for _, path := range files {
						os.Remove(path)
					}
				}(paths)
				break
			}
			if strings.Contains(err.Error(), "no such file") {
				break
			}
			time.Sleep(10 * time.Second)
		}
	}
}

// تابع ارسال تصاویر همراه با دکمه‌های شیشه‌ای
func (t *TelegramService) sendAlbum(imagePaths []string) error {
	if len(imagePaths) == 0 {
		return nil
	}

	// دکمه‌هایی که زیر عکس خطر ظاهر می‌شوند
	alertKeyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📷 دریافت عکس زنده", "status"),
			tgbotapi.NewInlineKeyboardButtonData("🔊 پخش آژیر خطر", "play_alarm"),
		),
	)

	var lastErr error
	for _, chatID := range t.chatIDs {
		// اگر فقط یک عکس بود
		if len(imagePaths) == 1 {
			photo := tgbotapi.NewPhoto(chatID, tgbotapi.FilePath(imagePaths[0]))
			photo.Caption = "🚨 **فوری:** یک شخص رویت شد!\n🕒 زمان: " + time.Now().Format("2006-01-02 15:04:05")
			photo.ParseMode = "Markdown"
			photo.ReplyMarkup = alertKeyboard // چسباندن دکمه به عکس

			if _, err := t.bot.Send(photo); err != nil {
				lastErr = err
			}
			continue
		}

		// اگر چند عکس (آلبوم) بود
		var mediaGroup []interface{}
		for i, path := range imagePaths {
			photo := tgbotapi.NewInputMediaPhoto(tgbotapi.FilePath(path))
			if i == 0 {
				photo.Caption = "📸 تصاویر تکمیلی\n🕒 زمان: " + time.Now().Format("2006-01-02 15:04:05")
			}
			mediaGroup = append(mediaGroup, photo)
		}

		msg := tgbotapi.MediaGroupConfig{ChatID: chatID, Media: mediaGroup}
		if _, err := t.bot.SendMediaGroup(msg); err != nil {
			lastErr = err
		}

		// در آلبوم‌ها نمی‌توان دکمه را مستقیم به عکس چسباند، پس یک پیام جداگانه بلافاصله می‌فرستیم
		if lastErr == nil {
			actionMsg := tgbotapi.NewMessage(chatID, "فرمان عملیاتی برای تصاویر بالا:")
			actionMsg.ReplyMarkup = alertKeyboard
			t.bot.Send(actionMsg)
		}
	}
	return lastErr
}

func (t *TelegramService) SendVideoAlert(videoPath string) error {
	t.mu.Lock()
	if !t.isConnected {
		t.mu.Unlock()
		return fmt.Errorf("telegram not connected")
	}
	t.mu.Unlock()

	var lastErr error
	for _, chatID := range t.chatIDs {
		video := tgbotapi.NewVideo(chatID, tgbotapi.FilePath(videoPath))
		video.Caption = fmt.Sprintf("🎥 VIDEO ALERT: Intrusion Detected!\n🕒 Time: %s", time.Now().Format("2006-01-02 15:04:05"))
		if _, err := t.bot.Send(video); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

func (t *TelegramService) SendMarkdownMessage(text string) {
	t.mu.Lock()
	if !t.isConnected {
		t.mu.Unlock()
		return
	}
	t.mu.Unlock()

	for _, chatID := range t.chatIDs {
		msg := tgbotapi.NewMessage(chatID, text)
		msg.ParseMode = "Markdown"
		t.bot.Send(msg)
	}
}

func (t *TelegramService) StartListening(state *core.SystemState, onDemand func(chatID int64)) {
	for {
		t.mu.Lock()
		ready := t.isConnected
		t.mu.Unlock()
		if ready {
			break
		}
		time.Sleep(2 * time.Second)
	}

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := t.bot.GetUpdatesChan(u)

	for update := range updates {
		if update.CallbackQuery != nil {
			if !t.isAuthorized(update.CallbackQuery.Message.Chat.ID) {
				continue
			}
			callback := tgbotapi.NewCallback(update.CallbackQuery.ID, "")

			// استخراج نام کاربر تلگرام
			userName := update.CallbackQuery.From.FirstName
			if update.CallbackQuery.From.LastName != "" {
				userName += " " + update.CallbackQuery.From.LastName
			}

			switch update.CallbackQuery.Data {
			case "arm":
				state.Arm(userName) // پاس دادن نام کاربر
				callback.Text = "سیستم مسلح شد! 🔴"
				t.SendMarkdownMessage(fmt.Sprintf("🔴 سیستم امنیتی توسط **%s** (تلگرام) **فعال** شد.", userName))

			case "disarm":
				state.Disarm(userName) // پاس دادن نام کاربر
				callback.Text = "سیستم غیرفعال شد! 🟢"
				t.SendMarkdownMessage(fmt.Sprintf("🟢 سیستم امنیتی توسط **%s** (تلگرام) **غیرفعال** شد.", userName))

			case "check_state": // منطق دکمه جدید
				statusTxt := "🟢 غیرفعال (Disarmed)"
				if state.IsArmed() {
					statusTxt = "🔴 فعال (Armed)"
				}
				actor := state.GetLastActor()
				if actor == "" {
					actor = "نامشخص (احتمالاً ری‌استارت شده)"
				}
				callback.Text = "وضعیت دریافت شد"
				t.SendMarkdownMessage(fmt.Sprintf("📊 **وضعیت سیستم:** %s\n👤 **آخرین تغییر توسط:** %s", statusTxt, actor))

			case "status":
				callback.Text = "در حال دریافت فیلم..."
				go onDemand(update.CallbackQuery.Message.Chat.ID)
			case "play_alarm":
				callback.Text = "فرمان آژیر صادر شد!"
				t.SendMarkdownMessage("🔊 **آژیر خطر فعال شد!**")
			}
			t.bot.Request(callback)
			continue
		}

		// بخش مربوط به پیام‌های متنی و دستورات
		if update.Message != nil {
			if !t.isAuthorized(update.Message.Chat.ID) {
				continue
			}

			text := strings.ToLower(update.Message.Text)

			// ثبت پترن جدید از راه دور
			if update.Message.IsCommand() && update.Message.Command() == "setpattern" {
				code := update.Message.CommandArguments()
				if code == "" {
					t.SendMarkdownMessage("❌ لطفاً کد پترن را وارد کنید. مثال:\n`/setpattern k3j5g7f9`")
					continue
				}

				err := UpdateEnvFile("SMS_PATTERN_CODE", code)
				if err != nil {
					t.SendMarkdownMessage("❌ خطا در ذخیره فایل تنظیمات: " + err.Error())
				} else {
					t.SendMarkdownMessage(fmt.Sprintf("✅ کد پترن با موفقیت به `%s` تغییر یافت و سیستم پیامک روی حالت ارسال سریع تنظیم شد!", code))
				}
				continue
			}

			if update.Message.Command() == "start" || text == "منو" {
				t.SendControlMenu()
			}
		}
	}
}

func (t *TelegramService) SendControlMenu() {
	t.mu.Lock()
	if !t.isConnected {
		t.mu.Unlock()
		return
	}
	t.mu.Unlock()

	for _, chatID := range t.chatIDs {
		msg := tgbotapi.NewMessage(chatID, "🛡 **کنترل پنل SentryGo**\nفرمان مورد نظر را انتخاب کنید:")
		msg.ParseMode = "Markdown"
		keyboard := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("🔴 فعال کردن (Arm)", "arm"),
				tgbotapi.NewInlineKeyboardButtonData("🟢 غیرفعال کردن (Disarm)", "disarm"),
			),
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("❓ وضعیت فعلی سیستم", "check_state"), // دکمه جدید
				tgbotapi.NewInlineKeyboardButtonData("📸🎥 عکس زنده", "status"),
			),
		)
		msg.ReplyMarkup = keyboard
		t.bot.Send(msg)
	}
}

// UpdateEnvFile فایل کانفیگ را می‌خواند و مقدار جدید را جایگزین می‌کند
func UpdateEnvFile(key, newValue string) error {
	filePath := ".env"
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err // اگر فایل نبود خطا می‌دهد
	}

	lines := strings.Split(string(data), "\n")
	found := false
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), key+"=") {
			lines[i] = key + "=" + newValue
			found = true
			break
		}
	}

	if !found {
		lines = append(lines, key+"="+newValue)
	}

	output := strings.Join(lines, "\n")
	return os.WriteFile(filePath, []byte(output), 0644)
}

// ارسال ویدیو به یک شخص خاص (کسی که دکمه را زده)
func (t *TelegramService) SendVideoToUser(chatID int64, videoPath string, isManual bool) error {
	t.mu.Lock()
	if !t.isConnected {
		t.mu.Unlock()
		return fmt.Errorf("telegram not connected")
	}
	t.mu.Unlock()

	video := tgbotapi.NewVideo(chatID, tgbotapi.FilePath(videoPath))
	if isManual {
		video.Caption = "🎥 ویدیوی درخواستی لحظه‌ای دوربین"
	} else {
		video.Caption = "🚨 فوری: شخص رویت شد!\n🕒 زمان: " + time.Now().Format("15:04:05")
	}

	_, err := t.bot.Send(video)
	return err
}
