package notify

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/raminfathi/sentrygo/internal/core"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type BaleService struct {
	bot         *tgbotapi.BotAPI
	chatIDs     []int64
	cooldown    time.Duration
	lastSent    time.Time
	mu          sync.Mutex
	Queue       chan []string
	isConnected bool
}

func (b *BaleService) isAuthorized(id int64) bool {
	for _, authID := range b.chatIDs {
		if id == authID {
			return true
		}
	}
	return false
}

func NewBaleService(token string, chatIDs []int64, cooldown time.Duration) *BaleService {
	svc := &BaleService{
		chatIDs:  chatIDs,
		cooldown: cooldown,
		Queue:    make(chan []string, 50),
	}
	go svc.connectLoop(token)
	return svc
}

func (b *BaleService) connectLoop(token string) {
	client := &http.Client{}
	baleEndpoint := "https://tapi.bale.ai/bot%s/%s"

	for {
		bot, err := tgbotapi.NewBotAPIWithClient(token, baleEndpoint, client)
		if err == nil {
			log.Printf("Authorized on Bale account: %s", bot.Self.UserName)
			b.mu.Lock()
			b.bot = bot
			b.isConnected = true
			b.mu.Unlock()
			go b.startWorker()
			b.SendControlMenu()
			return
		}
		log.Printf("Bale Network down, retrying in 10s...")
		time.Sleep(10 * time.Second)
	}
}

func (b *BaleService) QueueAlert(imagePaths []string) {
	b.mu.Lock()
	if time.Since(b.lastSent) < b.cooldown {
		b.mu.Unlock()
		return
	}
	b.lastSent = time.Now()
	b.mu.Unlock()

	select {
	case b.Queue <- imagePaths:
	default:
	}
}

func (b *BaleService) startWorker() {
	for paths := range b.Queue {
		for {
			err := b.sendAlbum(paths)
			if err == nil {
				break
			}
			if strings.Contains(err.Error(), "no such file") {
				break
			}
			time.Sleep(10 * time.Second)
		}
	}
}

func (b *BaleService) sendAlbum(imagePaths []string) error {
	if len(imagePaths) == 0 {
		return nil
	}

	// دکمه‌های عملیاتی برای بله
	alertKeyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📷 دریافت عکس زنده", "status"),
			tgbotapi.NewInlineKeyboardButtonData("🔊 پخش آژیر خطر", "play_alarm"),
		),
	)

	var lastErr error
	for _, chatID := range b.chatIDs {
		// حالت عکس تکی
		if len(imagePaths) == 1 {
			photo := tgbotapi.NewPhoto(chatID, tgbotapi.FilePath(imagePaths[0]))
			photo.Caption = "🚨 فوری: یک شخص رویت شد!\n🕒 زمان: " + time.Now().Format("2006-01-02 15:04:05")
			photo.ReplyMarkup = alertKeyboard // چسباندن دکمه به عکس

			if _, err := b.bot.Send(photo); err != nil {
				lastErr = err
			}
			continue
		}

		// حالت آلبوم (چند عکس)
		var mediaGroup []interface{}
		for i, path := range imagePaths {
			photo := tgbotapi.NewInputMediaPhoto(tgbotapi.FilePath(path))
			if i == 0 {
				photo.Caption = "📸 تصاویر تکمیلی\n🕒 زمان: " + time.Now().Format("2006-01-02 15:04:05")
			}
			mediaGroup = append(mediaGroup, photo)
		}

		msg := tgbotapi.MediaGroupConfig{ChatID: chatID, Media: mediaGroup}
		if _, err := b.bot.SendMediaGroup(msg); err != nil {
			lastErr = err
		}

		// ارسال پیام دکمه‌دار بعد از آلبوم
		if lastErr == nil {
			actionMsg := tgbotapi.NewMessage(chatID, "فرمان عملیاتی برای تصاویر بالا:")
			actionMsg.ReplyMarkup = alertKeyboard
			b.bot.Send(actionMsg)
		}
	}
	return lastErr
}

func (b *BaleService) SendVideo(videoPath string) {
	b.mu.Lock()
	if !b.isConnected {
		b.mu.Unlock()
		return
	}
	b.mu.Unlock()

	for _, chatID := range b.chatIDs {
		video := tgbotapi.NewVideo(chatID, tgbotapi.FilePath(videoPath))
		video.Caption = "🎥 SentryGo Bale Video Alert"
		b.bot.Send(video)
	}
}

func (b *BaleService) SendMessage(text string) {
	b.mu.Lock()
	if !b.isConnected {
		b.mu.Unlock()
		return
	}
	b.mu.Unlock()

	for _, chatID := range b.chatIDs {
		msg := tgbotapi.NewMessage(chatID, text)
		b.bot.Send(msg)
	}
}

func (b *BaleService) StartListening(state *core.SystemState, onDemand func(chatID int64)) {
	for {
		b.mu.Lock()
		ready := b.isConnected
		b.mu.Unlock()
		if ready {
			break
		}
		time.Sleep(2 * time.Second)
	}

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := b.bot.GetUpdatesChan(u)

	for update := range updates {
		// پردازش کلیک روی دکمه‌های شیشه‌ای
		if update.CallbackQuery != nil {
			if !b.isAuthorized(update.CallbackQuery.Message.Chat.ID) {
				continue
			}
			callback := tgbotapi.NewCallback(update.CallbackQuery.ID, "")
			switch update.CallbackQuery.Data {
			case "arm":
				state.Arm()
				callback.Text = "سیستم مسلح شد! 🔴"
				b.SendMessage("🔴 سیستم امنیتی فعال شد.")
			case "disarm":
				state.Disarm()
				callback.Text = "سیستم غیرفعال شد! 🟢"
				b.SendMessage("🟢 سیستم امنیتی غیرفعال شد.")
			case "status":
				callback.Text = "در حال دریافت فیلم..."
				go onDemand(update.CallbackQuery.Message.Chat.ID)
			case "play_alarm":
				callback.Text = "فرمان آژیر صادر شد!"
				b.SendMessage("🔊 **آژیر خطر فعال شد!** (در حال ارسال دستور به دوربین...)")
				// TODO: اتصال به تابع ISAPI هایک‌ویژن
			}
			b.bot.Request(callback)
			continue
		}

		// پردازش پیام‌های متنی و دستورات
		if update.Message != nil {
			if !b.isAuthorized(update.Message.Chat.ID) {
				continue
			}
			text := strings.ToLower(update.Message.Text)

			// تنظیم پترن جدید از داخل بله
			if update.Message.IsCommand() && update.Message.Command() == "setpattern" {
				code := update.Message.CommandArguments()
				if code == "" {
					b.SendMessage("❌ لطفاً کد پترن را وارد کنید. مثال:\n/setpattern k3j5g7f9")
					continue
				}

				// دقت کن که تابع UpdateEnvFile در پکیج notify و در فایل تلگرام تعریف شده
				// پس نیازی نیست دوباره اینجا تعریفش کنیم و مستقیم قابل استفاده است
				err := UpdateEnvFile("SMS_PATTERN_CODE", code)
				if err != nil {
					b.SendMessage("❌ خطا در ذخیره فایل تنظیمات: " + err.Error())
				} else {
					b.SendMessage(fmt.Sprintf("✅ کد پترن با موفقیت به `%s` تغییر یافت!", code))
				}
				continue
			}

			if update.Message.Command() == "start" || text == "منو" {
				b.SendControlMenu()
			}
		}
	}
}

func (b *BaleService) SendControlMenu() {
	b.mu.Lock()
	if !b.isConnected {
		b.mu.Unlock()
		return
	}
	b.mu.Unlock()

	for _, chatID := range b.chatIDs {
		msg := tgbotapi.NewMessage(chatID, "🛡 **کنترل پنل SentryGo (بله)**\nوضعیت را انتخاب کنید:")
		keyboard := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("🔴 خونه نیستم (فعال)", "arm"),
				tgbotapi.NewInlineKeyboardButtonData("🟢 خونه هستم (غیرفعال)", "disarm"),
			),
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("📸🎥 دریافت عکس زنده", "status"),
			),
		)
		msg.ReplyMarkup = keyboard
		b.bot.Send(msg)
	}
}

// ارسال ویدیو به یک شخص خاص (کسی که دکمه را زده)
func (b *BaleService) SendVideoToUser(chatID int64, videoPath string, isManual bool) error {
	b.mu.Lock()
	if !b.isConnected {
		b.mu.Unlock()
		return fmt.Errorf("telegram not connected")
	}
	b.mu.Unlock()

	video := tgbotapi.NewVideo(chatID, tgbotapi.FilePath(videoPath))
	if isManual {
		video.Caption = "🎥 ویدیوی درخواستی لحظه‌ای دوربین"
	} else {
		video.Caption = "🚨 فوری: شخص رویت شد!\n🕒 زمان: " + time.Now().Format("15:04:05")
	}

	_, err := b.bot.Send(video)
	return err
}
