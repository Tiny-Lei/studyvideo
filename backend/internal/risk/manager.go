package risk

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"studyvideo/internal/alert"
	"studyvideo/internal/auth"
	"studyvideo/internal/config"
	"studyvideo/internal/store"
)

type Manager struct {
	store *store.Store
	cfg   *config.Config
	auth  *auth.Auth
	alert *alert.Alerter

	total *slidingLimiter
	video *slidingLimiter
	login *slidingLimiter

	mu         sync.Mutex
	blockCache map[string]time.Time
	lastCheck  map[string]time.Time
}

func New(st *store.Store, cfg *config.Config, a *auth.Auth, al *alert.Alerter) *Manager {
	return &Manager{
		store:      st,
		cfg:        cfg,
		auth:       a,
		alert:      al,
		total:      newSlidingLimiter(cfg.BurstTotalLimit, cfg.BurstWindow),
		video:      newSlidingLimiter(cfg.BurstVideoLimit, cfg.BurstWindow),
		login:      newSlidingLimiter(cfg.LoginAttempts, cfg.LoginWindow),
		blockCache: make(map[string]time.Time),
		lastCheck:  make(map[string]time.Time),
	}
}

func (m *Manager) Start(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				m.total.Cleanup()
				m.video.Cleanup()
				m.login.Cleanup()
				m.refreshBlockCache()
				if err := m.store.CleanupBlocked(ctx); err != nil {
					slog.Error("清理过期封禁失败", "error", err)
				}
			}
		}
	}()
}

func (m *Manager) refreshBlockCache() {
	list, err := m.store.ListBlocked(context.Background())
	if err != nil {
		slog.Error("刷新封禁缓存失败", "error", err)
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.blockCache = make(map[string]time.Time, len(list))
	for _, b := range list {
		m.blockCache[b.IP] = b.BlockedUntil
	}
}

// Wrap 对公开 API 做统一风控；管理端接口自行鉴权，不经风控计数。
func (m *Manager) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/admin/") || r.URL.Path == "/api/health" {
			next.ServeHTTP(w, r)
			return
		}
		isVideo := strings.HasSuffix(r.URL.Path, "/play") ||
			strings.HasSuffix(r.URL.Path, "/file") ||
			strings.HasSuffix(r.URL.Path, "/download")
		if m.guard(w, r, isVideo) {
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (m *Manager) guard(w http.ResponseWriter, r *http.Request, isVideo bool) bool {
	if m.auth.IsAdmin(r) {
		return false
	}
	ip := m.ClientIP(r)
	if ip == "" {
		return false
	}

	if blocked, until := m.isBlocked(r.Context(), ip); blocked {
		m.reject(w, blockedMessage(until), until)
		return true
	}

	if ok, _ := m.total.Allow("t:" + ip); !ok {
		until := time.Now().Add(time.Duration(m.cfg.BlockMinutes) * time.Minute)
		m.block(r.Context(), ip, "短时间内请求过于频繁", until)
		m.reject(w, "请求过于频繁，已被临时限制访问", until)
		return true
	}
	if isVideo {
		if ok, _ := m.video.Allow("v:" + ip); !ok {
			until := time.Now().Add(time.Duration(m.cfg.BlockMinutes) * time.Minute)
			m.block(r.Context(), ip, fmt.Sprintf("短时间播放请求过于频繁（%d 秒内超过 %d 次）", int(m.cfg.BurstWindow.Seconds()), m.cfg.BurstVideoLimit), until)
			m.reject(w, "播放请求过于频繁，已被临时限制访问", until)
			return true
		}
	}

	videoHits, totalHits, err := m.store.IncrDaily(r.Context(), ip, isVideo)
	if err != nil {
		slog.Error("记录访问量失败", "error", err)
		return false
	}
	if isVideo && m.cfg.DailyVideoLimit > 0 && videoHits > m.cfg.DailyVideoLimit {
		until := endOfDay()
		m.block(r.Context(), ip, fmt.Sprintf("单日播放请求超过 %d 次", m.cfg.DailyVideoLimit), until)
		m.reject(w, fmt.Sprintf("今日播放请求已达上限（%d 次），请明日再试", m.cfg.DailyVideoLimit), until)
		return true
	}
	if m.cfg.DailyTotalLimit > 0 && totalHits > m.cfg.DailyTotalLimit {
		until := endOfDay()
		m.block(r.Context(), ip, fmt.Sprintf("单日总请求超过 %d 次", m.cfg.DailyTotalLimit), until)
		m.reject(w, "今日访问次数已达上限，请明日再试", until)
		return true
	}
	return false
}

func blockedMessage(until time.Time) string {
	return fmt.Sprintf("因异常访问已被限制，解除时间 %s", until.Format("01-02 15:04"))
}

func (m *Manager) reject(w http.ResponseWriter, msg string, until time.Time) {
	retry := int(time.Until(until).Seconds())
	if retry < 1 {
		retry = 1
	}
	w.Header().Set("Retry-After", fmt.Sprintf("%d", retry))
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusTooManyRequests)
	fmt.Fprintf(w, `{"error":%q}`+"\n", msg)
}

func (m *Manager) block(ctx context.Context, ip, reason string, until time.Time) {
	if err := m.store.SetBlocked(ctx, ip, reason, until); err != nil {
		slog.Error("写入封禁失败", "ip", ip, "error", err)
	}
	m.mu.Lock()
	m.blockCache[ip] = until
	m.lastCheck[ip] = time.Now()
	m.mu.Unlock()
	slog.Warn("已封禁 IP", "ip", ip, "reason", reason, "until", until.Format(time.RFC3339))
	m.alert.Send(fmt.Sprintf("【StudyVideo 风控告警】IP %s 已被限制访问：%s，解封时间 %s", ip, reason, until.Format("2006-01-02 15:04:05")))
}

func (m *Manager) isBlocked(ctx context.Context, ip string) (bool, time.Time) {
	now := time.Now()
	m.mu.Lock()
	if until, ok := m.blockCache[ip]; ok && until.After(now) {
		m.mu.Unlock()
		return true, until
	}
	if t, ok := m.lastCheck[ip]; ok && now.Sub(t) < 30*time.Second {
		m.mu.Unlock()
		return false, time.Time{}
	}
	m.mu.Unlock()

	blocked, until, err := m.store.IsBlocked(ctx, ip)
	if err != nil {
		slog.Error("查询封禁状态失败", "error", err)
		return false, time.Time{}
	}
	m.mu.Lock()
	m.lastCheck[ip] = now
	if blocked {
		m.blockCache[ip] = until
	} else {
		delete(m.blockCache, ip)
	}
	m.mu.Unlock()
	return blocked, until
}

func (m *Manager) Unblock(ctx context.Context, ip string) error {
	err := m.store.DeleteBlocked(ctx, ip)
	m.mu.Lock()
	delete(m.blockCache, ip)
	delete(m.lastCheck, ip)
	m.mu.Unlock()
	m.total.Reset("t:" + ip)
	m.video.Reset("v:" + ip)
	return err
}

// LoginAllowed 判断该 IP 是否允许尝试登录。
func (m *Manager) LoginAllowed(ip string) (bool, int) {
	return m.login.Allow("l:" + ip)
}

// LoginSucceeded 登录成功后清空该 IP 的失败计数。
func (m *Manager) LoginSucceeded(ip string) {
	m.login.Reset("l:" + ip)
}

func (m *Manager) ClientIP(r *http.Request) string {
	remote := r.RemoteAddr
	if host, _, err := net.SplitHostPort(remote); err == nil {
		remote = host
	}
	trust := m.cfg.TrustProxy == "true" || (m.cfg.TrustProxy == "auto" && isLoopback(remote))
	if trust {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			parts := strings.Split(xff, ",")
			if ip := strings.TrimSpace(parts[0]); net.ParseIP(ip) != nil {
				return ip
			}
		}
		if xr := strings.TrimSpace(r.Header.Get("X-Real-IP")); net.ParseIP(xr) != nil {
			return xr
		}
	}
	return remote
}

func isLoopback(host string) bool {
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func endOfDay() time.Time {
	now := time.Now()
	return time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
}
