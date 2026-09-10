package media

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/icholy/digest"
)

type Service struct {
	ip       string
	username string
	password string
}

func NewService(ip, username, password string) *Service {
	return &Service{
		ip:       ip,
		username: username,
		password: password,
	}
}

// CaptureBurst takes multiple snapshots sequentially.
func (s *Service) CaptureBurst(ctx context.Context, count int, interval time.Duration) ([]string, error) {
	if err := os.MkdirAll("snapshots", os.ModePerm); err != nil {
		return nil, fmt.Errorf("failed to create directory: %w", err)
	}

	var capturedFiles []string

	for i := 1; i <= count; i++ {
		url := fmt.Sprintf("http://%s/ISAPI/Streaming/channels/1/picture", s.ip)

		client := &http.Client{
			Transport: &digest.Transport{
				Username: s.username,
				Password: s.password,
			},
			Timeout: 5 * time.Second,
		}

		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			log.Printf("Burst: failed to create request for frame %d: %v", i, err)
			continue
		}

		resp, err := client.Do(req)
		if err != nil {
			log.Printf("Burst: failed to fetch frame %d: %v", i, err)
			continue
		}

		if resp.StatusCode == http.StatusOK {
			filename := fmt.Sprintf("snapshots/alert_%s_%d.jpg", time.Now().Format("20060102_150405"), i)
			file, err := os.Create(filename)
			if err == nil {
				io.Copy(file, resp.Body)
				file.Close()
				capturedFiles = append(capturedFiles, filename)
			}
		}
		resp.Body.Close()

		// Wait before taking the next photo
		if i < count {
			time.Sleep(interval)
		}
	}

	if len(capturedFiles) == 0 {
		return nil, fmt.Errorf("failed to capture any frames")
	}

	return capturedFiles, nil
}
