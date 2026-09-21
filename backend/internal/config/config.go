package config

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
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

	// 全局登录失败保护：窗口内全站累计失败达到阈值后，无条件锁定登录一段时间
	LoginGlobalLimit  int
	LoginGlobalWindow time.Duration
	LoginLockout      time.Duration

	CheckInterval time.Duration
	CheckOnStart  bool
	CheckTimeout  time.Duration
	CheckWorkers  int

	AlertWebhookURL string

	// 本地上传文件存储
	DataDir              string
	MaxPDFSizeBytes      int64 // 视频配套 PDF 上限
	MaxMaterialSizeBytes int64 // 资料区文件上限

	// 视频访问/观看明细保留天数（0 表示永久保留）
	StatsKeepDays int

	// 日志
	LogLevel  string // debug / info / warn / error
	LogFormat string // text / json
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
		Addr:              getenv("HTTP_ADDR", ":8080"),
		DSN:               getenv("DB_DSN", "studyvideo:studyvideo123@tcp(127.0.0.1:3306)/studyvideo?parseTime=true&charset=utf8mb4&loc=Local&timeout=5s"),
		AdminPassword:     password,
		SessionSecret:     []byte(secret),
		CookieSecure:      getenvBool("COOKIE_SECURE", false),
		TrustProxy:        getenv("TRUST_PROXY", "auto"),
		DailyVideoLimit:   getenvInt64("DAILY_VIDEO_LIMIT", 800),
		DailyTotalLimit:   getenvInt64("DAILY_TOTAL_LIMIT", 8000),
		BurstWindow:       time.Duration(getenvInt("BURST_WINDOW_SECONDS", 10)) * time.Second,
		BurstVideoLimit:   getenvInt("BURST_VIDEO_LIMIT", 20),
		BurstTotalLimit:   getenvInt("BURST_TOTAL_LIMIT", 120),
		BlockMinutes:      getenvInt("BLOCK_MINUTES", 30),
		LoginAttempts:     getenvInt("LOGIN_ATTEMPTS", 10),
		LoginWindow:       time.Duration(getenvInt("LOGIN_WINDOW_SECONDS", 600)) * time.Second,
		LoginGlobalLimit:  getenvInt("LOGIN_GLOBAL_LIMIT", 30),
		LoginGlobalWindow: time.Duration(getenvInt("LOGIN_GLOBAL_WINDOW_SECONDS", 900)) * time.Second,
		LoginLockout:      time.Duration(getenvInt("LOGIN_LOCKOUT_SECONDS", 1800)) * time.Second,
		CheckInterval:     time.Duration(getenvInt("CHECK_INTERVAL_MINUTES", 360)) * time.Minute,
		CheckOnStart:      getenvBool("CHECK_ON_START", true),
		CheckTimeout:      time.Duration(getenvInt("CHECK_TIMEOUT_SECONDS", 15)) * time.Second,
		CheckWorkers:      getenvInt("CHECK_WORKERS", 6),
		AlertWebhookURL:   strings.TrimSpace(os.Getenv("ALERT_WEBHOOK_URL")),
		DataDir:           getenv("DATA_DIR", "./data"),
		MaxPDFSizeBytes:   getenvInt64("MAX_PDF_SIZE_MB", 50) * 1024 * 1024,
		StatsKeepDays:     getenvInt("STATS_KEEP_DAYS", 730),
		LogLevel:          strings.ToLower(getenv("LOG_LEVEL", "info")),
		LogFormat:         strings.ToLower(getenv("LOG_FORMAT", "text")),
	}
}

// LogLevelValue 把配置的日志级别映射为 slog 级别。
func (c *Config) LogLevelValue() slog.Level {
	switch c.LogLevel {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// Print 输出当前生效配置（敏感值仅显示是否已设置），用于部署排查。
func (c *Config) Print(w io.Writer) {
	mask := func(v string) string {
		if strings.TrimSpace(v) == "" {
			return "(未设置)"
		}
		return "(已设置)"
	}
	dsn := c.DSN
	if i := strings.LastIndex(dsn, "@"); i > 0 {
		if j := strings.Index(dsn, ":"); j > 0 && j < i {
			dsn = dsn[:j+1] + "******" + dsn[i:]
		}
	}
	fmt.Fprintf(w, `StudyVideo 生效配置
  HTTP_ADDR              %s
  DB_DSN                 %s
  ADMIN_PASSWORD         %s
  SESSION_SECRET         %s
  COOKIE_SECURE          %v
  TRUST_PROXY            %s
  DATA_DIR               %s
  MAX_PDF_SIZE_MB        %d
  MAX_MATERIAL_SIZE_MB   %d
  STATS_KEEP_DAYS        %d
  DAILY_VIDEO_LIMIT      %d
  DAILY_TOTAL_LIMIT      %d
  BURST_VIDEO_LIMIT      %d
  BURST_TOTAL_LIMIT      %d
  BURST_WINDOW_SECONDS   %d
  BLOCK_MINUTES          %d
  LOGIN_ATTEMPTS         %d（单 IP，窗口 %d 秒）
  LOGIN_GLOBAL_LIMIT     %d（全站，窗口 %d 秒，锁定 %d 秒；0 = 禁用）
  CHECK_INTERVAL_MINUTES %d
  CHECK_ON_START         %v
  CHECK_TIMEOUT_SECONDS  %d
  CHECK_WORKERS          %d
  LOG_LEVEL              %s
  LOG_FORMAT             %s
  ALERT_WEBHOOK_URL      %s
`,
		c.Addr, dsn, mask(c.AdminPassword), mask(string(c.SessionSecret)), c.CookieSecure, c.TrustProxy,
		c.DataDir, c.MaxPDFSizeBytes>>20, c.MaxMaterialSizeBytes>>20, c.StatsKeepDays,
		c.DailyVideoLimit, c.DailyTotalLimit, c.BurstVideoLimit, c.BurstTotalLimit, int(c.BurstWindow.Seconds()),
		c.BlockMinutes,
		c.LoginAttempts, int(c.LoginWindow.Seconds()),
		c.LoginGlobalLimit, int(c.LoginGlobalWindow.Seconds()), int(c.LoginLockout.Seconds()),
		int(c.CheckInterval.Minutes()), c.CheckOnStart, int(c.CheckTimeout.Seconds()), c.CheckWorkers,
		c.LogLevel, c.LogFormat, c.AlertWebhookURL)
}
