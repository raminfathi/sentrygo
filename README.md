
```markdown
# SentryGo 🛡️

A lightweight, self-hosted smart security system written in Go. SentryGo connects to RTSP-enabled security cameras (specifically optimized for Hikvision), provides a real-time web dashboard, and sends instant alerts via Telegram, Bale (Iranian Messenger), and FarazSMS.

## ✨ Features

*   **Real-Time Web Dashboard:** Watch live camera feeds directly in your browser with near-zero latency (using FFmpeg and MPJPEG streaming).
*   **Multi-Channel Alerts:** Instant notifications with snapshots sent to Telegram, Bale, and via SMS.
*   **Remote Control:** Arm or disarm the alarm system directly from the web dashboard or via bot commands.
*   **Media Archive & Gallery:** Manually or automatically capture snapshots and 25-second video clips. View and manage them through the built-in web gallery.
*   **Dynamic Configuration:** Edit sensitive configurations (like bot tokens, passwords, and phone numbers) directly from the secure web UI without touching the server files.
*   **Secure Access:** The web dashboard is protected by HTTP Basic Authentication to prevent unauthorized access.

## 📋 Prerequisites

Before running this project, ensure you have the following installed on your system:
*   [Go](https://go.dev/dl/) (v1.20 or higher)
*   [FFmpeg](https://ffmpeg.org/download.html) (Required for RTSP to Web streaming and media recording)
*   A Hikvision camera (or any IP camera supporting RTSP streams)

## 🚀 Installation

1. **Clone the repository:**
   ```bash
   git clone [https://github.com/raminfathi/sentrygo.git](https://github.com/raminfathi/sentrygo.git)
   cd sentrygo

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
# or if you have a Makefile configured:
make run

```

Once the server is running, the Web Dashboard will be available at:
**http://localhost:8080**

* **Login:** Use the `DASHBOARD_USER` and `DASHBOARD_PASS` defined in your `.env` file.
* **Settings:** Navigate to the "Settings" tab in the dashboard to update API keys, phone numbers, or passwords on the fly. After saving, use the "Restart" button in the UI to apply changes.

## 🔒 Security Notes

* **Basic Auth:** The dashboard is protected via Basic Authentication. Never expose port `8080` directly to the public internet without a reverse proxy (like Nginx) and HTTPS (SSL/TLS).
* `.env` **Protection:** Ensure your `.env` and `archive/` directory remain in your `.gitignore` to prevent leaking sensitive data and personal footage.

## 🤝 Contributing

Pull requests are welcome. For major changes, please open an issue first to discuss what you would like to change.

```