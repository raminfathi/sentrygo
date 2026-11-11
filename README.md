# SentryGo 🛡️

A lightweight, self-hosted smart security system written in Go. SentryGo connects to RTSP-enabled security cameras (specifically optimized for Hikvision), provides a real-time web dashboard, and sends instant alerts via Telegram, Bale (Iranian Messenger), and FarazSMS.

## ✨ Features

-   **24/7 Continuous Recording:** Reliable MKV segment recording with automatic storage management (Auto-Cleanup).
-   **Silent Detection (Burst Capture):** When disarmed, the system can silently capture high-quality burst photos in daily folders upon detecting motion, without triggering alarms.
-   **Real-Time Web Dashboard:** Watch live camera feeds directly in your browser with near-zero latency.
-   **Multi-Channel Alerts:** Instant notifications with snapshots and video clips sent to Telegram, Bale, and via SMS.
-   **Remote Control:** Arm or disarm the alarm system directly from the web dashboard.
-   **Media Archive & Gallery:** Browse daily captured events, manual snapshots, and continuous recordings.
-   **Dynamic Configuration:** Edit sensitive configurations and recording paths directly from the secure web UI without touching code.
-   **Secure Access:** The web dashboard is protected by HTTP Basic Authentication.
## 📋 Prerequisites

Before running this project, ensure you have the following installed on your system:
*   [Go](https://go.dev/dl/) (v1.20 or higher)
*   [FFmpeg](https://ffmpeg.org/download.html) (Required for RTSP to Web streaming)
*   A Hikvision camera (or any IP camera supporting RTSP streams)

## 🚀 Installation

1. **Clone the repository:**
   ```bash
   git clone https://github.com/raminfathi/sentrygo.git
   cd sentrygo
   ```

2. **Install Go dependencies:**
   ```bash
   go mod tidy
   ```

3. **Environment Setup:**
   Create your configuration file by copying the provided example:
   ```bash
   cp .env.example .env
   ```
   *Note: Open `.env` and fill in your camera credentials, bot tokens, and API keys. This file is ignored by Git to keep your data secure.*

## 💻 Usage

Start the SentryGo server:
```bash
go run cmd/api/main.go
```

Once the server is running, the Web Dashboard will be available at:
**http://localhost:8080**

## 🔒 Security Notes

*   **Basic Auth:** The dashboard is protected via Basic Authentication. Never expose port `8080` directly to the public internet without a reverse proxy.
*   **.env Protection:** Ensure your `.env` and `archive/` directory remain in your `.gitignore`.

## 🤝 Contributing

Pull requests are welcome. For major changes, please open an issue first to discuss what you would like to change.

## 📄 License

[MIT](https://choosealicense.com/licenses/mit/)