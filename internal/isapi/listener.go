package isapi

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"sentrygo/internal/core"
	"sentrygo/internal/media" // پکیج جدید ایمپورت شد
	"sentrygo/internal/vision"

	"sentrygo/internal/notify" // Import the new notify package

	"github.com/icholy/digest"
)

type Listener struct {
	ip       string
	username string
	password string
	state    *core.SystemState
	media    *media.Service          // فیلد جدید اضافه شد
	notifier *notify.TelegramService // Add the notifier field
	vision   *vision.FaceDetector    // Add the vision module
}

// NewListener initializes a new ISAPI stream listener.
func NewListener(ip, username, password string, state *core.SystemState, mediaSvc *media.Service, notifier *notify.TelegramService, faceDetector *vision.FaceDetector) *Listener {
	return &Listener{
		ip:       ip,
		username: username,
		password: password,
		state:    state,
		media:    mediaSvc,
		notifier: notifier,
		vision:   faceDetector, // <--- دلیل کرش خالی بودن این فیلد بود!
	}
}

func (l *Listener) Start(ctx context.Context) {
	url := fmt.Sprintf("http://%s/ISAPI/Event/notification/alertStream", l.ip)

	client := &http.Client{
		Transport: &digest.Transport{
			Username: l.username,
			Password: l.password,
		},
		Timeout: 0,
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		log.Printf("Failed to create request: %v", err)
		return
	}

	log.Println("Connecting to camera event stream...")
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Connection failed: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("Camera returned status: %d", resp.StatusCode)
		return
	}

	log.Println("Successfully connected to ISAPI event stream!")

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			log.Println("Stopping ISAPI listener...")
			return
		default:
		}

		line := scanner.Text()
		// if strings.Contains(line, "<eventType>") {
		// 	log.Printf("🔍 RAW EVENT FROM CAMERA: %s", strings.TrimSpace(line))
		// }
		if strings.Contains(line, "<targetType>human</targetType>") {
			log.Println("Event Triggered: Human detected by the camera.")

			if l.state.IsArmed() {
				log.Println("ALERT: System is ARMED. Capturing burst snapshots...")

				// Take 5 photos with 800ms delay between them
				imagePaths, err := l.media.CaptureBurst(ctx, 5, 800*time.Millisecond)
				if err != nil {
					log.Printf("Failed to capture burst snapshots: %v", err)
					continue
				}
				// AI Check: Scan all captured frames for a face
				faceFound := false
				for _, path := range imagePaths {
					if l.vision.ContainsFace(path) {
						faceFound = true
						break
					}
				}

				if faceFound {
					log.Println("AI Confirmation: Face detected! Queuing for Telegram...")
					l.notifier.QueueBurstAlert(imagePaths)
				} else {
					log.Println("AI Confirmation: No face detected. Dropping alert and cleaning up.")
					for _, path := range imagePaths {
						os.Remove(path)
					}
				}
			} else {
				log.Println("INFO: System is DISARMED. Logging event only, no alerts sent.")
			}
		}
	}

	if err := scanner.Err(); err != nil {
		log.Printf("Stream read error: %v", err)
	}
}
