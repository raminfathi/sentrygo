package notify

import (
	"fmt"
	"log"
	"net/http"
	"sentrygo/internal/core"
	"strings"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type BaleService struct {
	bot      *tgbotapi.BotAPI
	chatID   int64
	cooldown time.Duration
	lastSent time.Time
	mu       sync.Mutex
	Queue    chan []string
}

func NewBaleService(token string, chatID int64, cooldown time.Duration) (*BaleService, error) {
	client := &http.Client{}
	baleEndpoint := "https://tapi.bale.ai/bot%s/%s"

	bot, err := tgbotapi.NewBotAPIWithClient(token, baleEndpoint, client)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to bale: %w", err)
	}

	log.Printf("Authorized on Bale account: %s", bot.Self.UserName)

	svc := &BaleService{
		bot:      bot,
		chatID:   chatID,
		cooldown: cooldown,
		Queue:    make(chan []string, 50),
	}

	go svc.startWorker()
	return svc, nil
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
		log.Println("Bale: Alert added to queue.")
	default:
		log.Println("Bale: Queue is full!")
	}
}

func (b *BaleService) startWorker() {
	for paths := range b.Queue {
		for {
			err := b.sendAlbum(paths)
			if err == nil {
				log.Println("Bale: Alert album sent successfully.")
				break
			}
			log.Printf("Bale network error, retrying in 10s: %v", err)
			time.Sleep(10 * time.Second)
		}
	}
}

func (b *BaleService) sendAlbum(imagePaths []string) error {
	if len(imagePaths) == 0 {
		return nil
	}

	// حالت اول: ارسال عکس تکی در بله
	if len(imagePaths) == 1 {
		photo := tgbotapi.NewPhoto(b.chatID, tgbotapi.FilePath(imagePaths[0]))
		photo.Caption = "🚨 فوری: یک شخص رویت شد!\n🕒 زمان: " + time.Now().Format("2006-01-02 15:04:05")
		_, err := b.bot.Send(photo)
		return err
	}

	// حالت دوم: ارسال آلبوم در بله
	var mediaGroup []interface{}
	for i, path := range imagePaths {
		photo := tgbotapi.NewInputMediaPhoto(tgbotapi.FilePath(path))
		if i == 0 {
			photo.Caption = "📸 تصاویر تکمیلی\n🕒 زمان: " + time.Now().Format("2006-01-02 15:04:05")
		}
		mediaGroup = append(mediaGroup, photo)
	}

	msg := tgbotapi.MediaGroupConfig{
		ChatID: b.chatID,
		Media:  mediaGroup,
	}

	_, err := b.bot.SendMediaGroup(msg)
	return err
}

func (b *BaleService) SendVideo(videoPath string) {
	video := tgbotapi.NewVideo(b.chatID, tgbotapi.FilePath(videoPath))
	video.Caption = "🎥 SentryGo Bale Video Alert"
	if _, err := b.bot.Send(video); err != nil {
		log.Printf("Failed to send video to Bale: %v", err)
	} else {
		log.Println("Bale video sent successfully.")
	}
}

// تابع کمکی برای ارسال پیام متنی در بله
func (b *BaleService) sendMessage(text string) {
	msg := tgbotapi.NewMessage(b.chatID, text)
	b.bot.Send(msg)
}

// --- بخش کنترل و تعامل دوطرفه بله ---
func (b *BaleService) StartListening(state *core.SystemState, onDemand func()) {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := b.bot.GetUpdatesChan(u)

	for update := range updates {
		if update.CallbackQuery != nil {
			if update.CallbackQuery.Message.Chat.ID != b.chatID {
				continue
			}

			callback := tgbotapi.NewCallback(update.CallbackQuery.ID, "")

			switch update.CallbackQuery.Data {
			case "arm":
				state.Arm()
				callback.Text = "سیستم مسلح شد! 🔴"
				b.sendMessage("🔴 سیستم امنیتی **فعال** شد.")
			case "disarm":
				state.Disarm()
				callback.Text = "سیستم غیرفعال شد! 🟢"
				b.sendMessage("🟢 سیستم امنیتی **غیرفعال** شد.")
			case "status":
				callback.Text = "در حال دریافت تصویر..."
				b.sendMessage("⏳ در حال دریافت عکس و فیلم فعلی...")
				go onDemand() // اجرای دستور
			}

			b.bot.Request(callback)
			continue
		}

		if update.Message != nil {
			if update.Message.Chat.ID != b.chatID {
				continue
			}
			text := strings.ToLower(update.Message.Text)
			if update.Message.Command() == "start" || update.Message.Command() == "menu" || text == "منو" || text == "menu" {
				b.SendControlMenu()
			}
		}
	}
}

func (b *BaleService) SendControlMenu() {
	msg := tgbotapi.NewMessage(b.chatID, "🛡 **کنترل پنل SentryGo (بله)**\nوضعیت را انتخاب کنید:")

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
	b.bot.Send(msg)
}
