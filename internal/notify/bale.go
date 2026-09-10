package notify

import (
	"fmt"
	"log"
	"net/http"
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
	var mediaGroup []interface{}
	for i, path := range imagePaths {
		photo := tgbotapi.NewInputMediaPhoto(tgbotapi.FilePath(path))
		if i == 0 {
			photo.Caption = fmt.Sprintf("🚨 SentryGo Bale Alert\n🕒 Time: %s", time.Now().Format("2006-01-02 15:04:05"))
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
