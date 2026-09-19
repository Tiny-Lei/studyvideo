package risk

import (
	"testing"
	"time"

	"studyvideo/internal/alert"
	"studyvideo/internal/auth"
	"studyvideo/internal/config"
)

func newTestRisk(cfg *config.Config) *Manager {
	if cfg == nil {
		cfg = &config.Config{}
	}
	if cfg.LoginAttempts == 0 {
		cfg.LoginAttempts = 10
	}
	if cfg.LoginWindow == 0 {
		cfg.LoginWindow = time.Minute
	}
	if cfg.LoginGlobalLimit == 0 {
		cfg.LoginGlobalLimit = 5
	}
	if cfg.LoginGlobalWindow == 0 {
		cfg.LoginGlobalWindow = time.Minute
	}
	if cfg.LoginLockout == 0 {
		cfg.LoginLockout = time.Minute
	}
	auther := auth.New("test-password", []byte("test-secret"), false)
	return New(nil, cfg, auther, alert.New(""))
}

// 多 IP 分布式失败也应触发全局锁定（单 IP 限流挡不住的情况）。
func TestGlobalLoginLockoutAcrossIPs(t *testing.T) {
	m := newTestRisk(nil)

	for i := 0; i < 4; i++ {
		ip := "203.0.113." + string(rune('1'+i))
		if ok, _ := m.LoginAllowed(ip); !ok {
			t.Fatalf("第 %d 个 IP 首次尝试应放行", i+1)
		}
		m.LoginFailed(ip)
	}

	// 第 5 次失败触发阈值
	m.LoginFailed("198.51.100.99")

	// 全局锁定后，任何新 IP 都不允许尝试
	if ok, retry := m.LoginAllowed("192.0.2.200"); ok || retry < 1 {
		t.Fatalf("达到全局阈值后应锁定所有 IP, ok=%v retry=%d", ok, retry)
	}
	if locked, until := m.loginGloballyLocked(); !locked || until.Before(time.Now()) {
		t.Fatalf("应处于锁定状态, locked=%v until=%v", locked, until)
	}
}

// 成功登录应清除全局失败记录并解除锁定。
func TestLoginSuccessResetsGlobalState(t *testing.T) {
	m := newTestRisk(nil)

	for i := 0; i < 4; i++ {
		m.LoginFailed("203.0.113.10")
	}
	m.LoginSucceeded("203.0.113.10")

	// 重置后重新计数，未达阈值不应锁定
	m.LoginFailed("203.0.113.11")
	if locked, _ := m.loginGloballyLocked(); locked {
		t.Fatal("成功登录后不应保留全局锁定")
	}
	if ok, _ := m.LoginAllowed("192.0.2.1"); !ok {
		t.Fatal("重置后其它 IP 应可继续尝试")
	}
}

// 锁定到期后自动解锁。
func TestGlobalLoginLockoutExpires(t *testing.T) {
	m := newTestRisk(&config.Config{
		LoginAttempts:     100,
		LoginWindow:       time.Minute,
		LoginGlobalLimit:  2,
		LoginGlobalWindow: time.Minute,
		LoginLockout:      30 * time.Millisecond,
	})

	m.LoginFailed("203.0.113.1")
	m.LoginFailed("203.0.113.2")
	if ok, _ := m.LoginAllowed("192.0.2.1"); ok {
		t.Fatal("应处于锁定状态")
	}

	time.Sleep(50 * time.Millisecond)
	if ok, _ := m.LoginAllowed("192.0.2.1"); !ok {
		t.Fatal("锁定到期后应自动放行")
	}
}

// 关闭全局保护（阈值 0）时不应锁定。
func TestGlobalLoginLockoutDisabled(t *testing.T) {
	cfg := &config.Config{
		LoginAttempts:     100,
		LoginWindow:       time.Minute,
		LoginGlobalLimit:  0, // 0 = 禁用
		LoginGlobalWindow: time.Minute,
		LoginLockout:      time.Minute,
	}
	m := New(nil, cfg, auth.New("test-password", []byte("test-secret"), false), alert.New(""))
	for i := 0; i < 20; i++ {
		m.LoginFailed("203.0.113.1")
	}
	if locked, _ := m.loginGloballyLocked(); locked {
		t.Fatal("阈值设为 0 时应禁用全局锁定")
	}
	if ok, _ := m.LoginAllowed("203.0.113.9"); !ok {
		t.Fatal("未锁定时应放行")
	}
}
