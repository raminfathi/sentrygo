# مرحله اول: بیلد کردن کدهای Go
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# ساخت فایل اجرایی (مسیر main.go را در صورت نیاز با ساختار پوشه‌های خود تطبیق دهید)
RUN CGO_ENABLED=0 GOOS=linux go build -o sentrygo ./cmd/api/main.go

# مرحله دوم: ساخت ایمیج نهایی و سبک
FROM alpine:latest
# نصب FFmpeg و ابزارهای گواهینامه و منطقه زمانی
RUN apk add --no-cache ffmpeg tzdata ca-certificates

WORKDIR /app
COPY --from=builder /app/sentrygo .

EXPOSE 8080
CMD ["./sentrygo"]