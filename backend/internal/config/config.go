package config

import (
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Addr          string
	DSN           string
	AdminPassword string
	SessionSecret []byte
	CookieSecure  bool

	// "auto" 时：请求来自回环地址（本机反向代理）才信任 X-Forwarded-For
	TrustProxy string

	DailyVideoLimit int64
	DailyTotalLimit int64
	BurstWindow     time.Duration
	BurstVideoLimit int
	BurstTotalLimit int
	BlockMinutes    int
	LoginAttempts   int
	LoginWindow     time.Duration

	CheckInterval time.Duration
	CheckOnStart  bool
	CheckTimeout  time.Duration
	CheckWorkers  int

	AlertWebhookURL string

	// PDF 资料本地存储
	DataDir         string
	MaxPDFSizeBytes int64
}

func getenv(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

func getenvInt(key string, def int) int {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func getenvInt64(key string, def int64) int64 {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			return n
		}
	}
	return def
}

func getenvBool(key string, def bool) bool {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		switch strings.ToLower(v) {
		case "1", "true", "yes", "on":
			return true
		case "0", "false", "no", "off":
			return false
		}
	}
	return def
}

func Load() *Config {
	secret := strings.TrimSpace(os.Getenv("SESSION_SECRET"))
	if secret == "" {
		buf := make([]byte, 32)
		_, _ = rand.Read(buf)
		secret = hex.EncodeToString(buf)
		slog.Warn("未设置 SESSION_SECRET，已随机生成；服务重启后管理员需要重新登录")
	}
	password := getenv("ADMIN_PASSWORD", "admin123")
	if password == "admin123" {
		slog.Warn("正在使用默认管理密码 admin123，请通过环境变量 ADMIN_PASSWORD 修改！")
	}
	return &Config{
		Addr:            getenv("HTTP_ADDR", ":8080"),
		DSN:             getenv("DB_DSN", "studyvideo:studyvideo123@tcp(127.0.0.1:3306)/studyvideo?parseTime=true&charset=utf8mb4&loc=Local&timeout=5s"),
		AdminPassword:   password,
		SessionSecret:   []byte(secret),
		CookieSecure:    getenvBool("COOKIE_SECURE", false),
		TrustProxy:      getenv("TRUST_PROXY", "auto"),
		DailyVideoLimit: getenvInt64("DAILY_VIDEO_LIMIT", 800),
		DailyTotalLimit: getenvInt64("DAILY_TOTAL_LIMIT", 8000),
		BurstWindow:     time.Duration(getenvInt("BURST_WINDOW_SECONDS", 10)) * time.Second,
		BurstVideoLimit: getenvInt("BURST_VIDEO_LIMIT", 20),
		BurstTotalLimit: getenvInt("BURST_TOTAL_LIMIT", 120),
		BlockMinutes:    getenvInt("BLOCK_MINUTES", 30),
		LoginAttempts:   getenvInt("LOGIN_ATTEMPTS", 10),
		LoginWindow:     time.Duration(getenvInt("LOGIN_WINDOW_SECONDS", 600)) * time.Second,
		CheckInterval:   time.Duration(getenvInt("CHECK_INTERVAL_MINUTES", 360)) * time.Minute,
		CheckOnStart:    getenvBool("CHECK_ON_START", true),
		CheckTimeout:    time.Duration(getenvInt("CHECK_TIMEOUT_SECONDS", 15)) * time.Second,
		CheckWorkers:    getenvInt("CHECK_WORKERS", 6),
		AlertWebhookURL: strings.TrimSpace(os.Getenv("ALERT_WEBHOOK_URL")),
		DataDir:         getenv("DATA_DIR", "./data"),
		MaxPDFSizeBytes: getenvInt64("MAX_PDF_SIZE_MB", 50) * 1024 * 1024,
	}
}
