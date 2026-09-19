package health

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"studyvideo/internal/alert"
	"studyvideo/internal/config"
	"studyvideo/internal/store"
)

type Result struct {
	Code      int    `json:"code"`
	LatencyMS int64  `json:"latency_ms"`
	Status    string `json:"status"`
	ErrMsg    string `json:"err_msg"`
}

type Progress struct {
	Running    bool       `json:"running"`
	Trigger    string     `json:"trigger"`
	Total      int        `json:"total"`
	Done       int        `json:"done"`
	OK         int        `json:"ok"`
	Fail       int        `json:"fail"`
	StartedAt  time.Time  `json:"started_at"`
	FinishedAt *time.Time `json:"finished_at"`
}

type Checker struct {
	store  *store.Store
	cfg    *config.Config
	alert  *alert.Alerter
	client *http.Client

	baseCtx  context.Context
	mu       sync.Mutex
	running  bool
	progress Progress
}

func New(st *store.Store, cfg *config.Config, al *alert.Alerter) *Checker {
	return &Checker{
		store: st,
		cfg:   cfg,
		alert: al,
		client: &http.Client{
			Timeout: cfg.CheckTimeout,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 10 {
					return fmt.Errorf("重定向次数过多")
				}
				return nil
			},
		},
	}
}

func (c *Checker) Start(ctx context.Context) {
	c.baseCtx = ctx
	go func() {
		if c.cfg.CheckOnStart {
			select {
			case <-time.After(10 * time.Second):
				c.RunAsync("启动检查")
			case <-ctx.Done():
				return
			}
		}
		if c.cfg.CheckInterval <= 0 {
			return
		}
		ticker := time.NewTicker(c.cfg.CheckInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				c.RunAsync("定时检查")
			}
		}
	}()
}

func (c *Checker) Progress() Progress {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.progress
}

// RunAsync 启动一次全量检查；若已有检查在执行则返回 false。
func (c *Checker) RunAsync(trigger string) bool {
	c.mu.Lock()
	if c.running {
		c.mu.Unlock()
		return false
	}
	c.running = true
	c.progress = Progress{Running: true, Trigger: trigger, StartedAt: time.Now()}
	c.mu.Unlock()

	go func() {
		ctx := c.baseCtx
		if ctx == nil {
			ctx = context.Background()
		}
		defer func() {
			c.mu.Lock()
			c.running = false
			c.progress.Running = false
			now := time.Now()
			c.progress.FinishedAt = &now
			p := c.progress
			c.mu.Unlock()
			slog.Info("链接检查完成", "trigger", trigger, "total", p.Total, "ok", p.OK, "fail", p.Fail)
		}()

		videos, err := c.store.VideosForCheck(ctx)
		if err != nil {
			slog.Error("读取待检查视频失败", "error", err)
			return
		}
		c.mu.Lock()
		c.progress.Total = len(videos)
		c.mu.Unlock()

		workers := c.cfg.CheckWorkers
		if workers < 1 {
			workers = 4
		}
		sem := make(chan struct{}, workers)
		var wg sync.WaitGroup
		for _, v := range videos {
			select {
			case <-ctx.Done():
				return
			case sem <- struct{}{}:
			}
			wg.Add(1)
			go func(v store.Video) {
				defer wg.Done()
				defer func() { <-sem }()
				c.checkOne(ctx, v, true)
			}(v)
		}
		wg.Wait()
	}()
	return true
}

func (c *Checker) CheckOne(ctx context.Context, v store.Video) Result {
	return c.checkOne(ctx, v, false)
}

// CheckOneAsync 后台异步探测单个链接（使用独立超时，不受请求上下文影响）。
func (c *Checker) CheckOneAsync(v store.Video) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), c.cfg.CheckTimeout+10*time.Second)
		defer cancel()
		c.checkOne(ctx, v, false)
	}()
}

func (c *Checker) checkOne(ctx context.Context, v store.Video, track bool) Result {
	res := c.probe(ctx, v.URL)
	checkedAt := time.Now()
	if err := c.store.UpdateVideoCheck(ctx, v.ID, res.Status, res.Code, res.LatencyMS, res.ErrMsg, checkedAt); err != nil {
		slog.Error("更新链接状态失败", "video_id", v.ID, "error", err)
	}
	if track {
		c.mu.Lock()
		c.progress.Done++
		if res.Status == "ok" {
			c.progress.OK++
		} else {
			c.progress.Fail++
		}
		c.mu.Unlock()
	}
	if res.Status == "fail" && v.Status != "fail" {
		c.alert.Send(fmt.Sprintf("【StudyVideo 链接告警】视频 #%d 链接失效：%s（%s）", v.ID, res.ErrMsg, v.URL))
	}
	return res
}

func (c *Checker) probe(ctx context.Context, rawURL string) Result {
	start := time.Now()
	code, ctype, err := c.do(ctx, http.MethodHead, rawURL, "")
	if err == nil && (code == http.StatusForbidden || code == http.StatusMethodNotAllowed ||
		code == http.StatusNotImplemented || code == http.StatusBadRequest) {
		code, ctype, err = c.do(ctx, http.MethodGet, rawURL, "bytes=0-0")
	}
	latency := time.Since(start).Milliseconds()
	res := Result{Code: code, LatencyMS: latency}
	if err != nil {
		res.Status = "fail"
		res.ErrMsg = "请求失败: " + err.Error()
		return res
	}
	status, msg := classifyProbe(code, ctype)
	res.Status = status
	res.ErrMsg = msg
	return res
}

func (c *Checker) do(ctx context.Context, method, rawURL, byteRange string) (int, string, error) {
	req, err := http.NewRequestWithContext(ctx, method, rawURL, nil)
	if err != nil {
		return 0, "", err
	}
	req.Header.Set("User-Agent", "StudyVideo-LinkChecker/1.0")
	req.Header.Set("Accept", "video/*,*/*;q=0.8")
	if byteRange != "" {
		req.Header.Set("Range", byteRange)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close()
	if method == http.MethodGet {
		_, _ = io.CopyN(io.Discard, resp.Body, 1024)
	}
	return resp.StatusCode, resp.Header.Get("Content-Type"), nil
}

func classifyProbe(code int, contentType string) (string, string) {
	switch code {
	case http.StatusOK, http.StatusPartialContent:
		if strings.HasPrefix(strings.ToLower(contentType), "text/html") {
			return "fail", fmt.Sprintf("返回的是网页而非视频文件 (HTTP %d)", code)
		}
		return "ok", ""
	case http.StatusNotFound:
		return "fail", "链接不存在 (HTTP 404)"
	case http.StatusForbidden:
		return "fail", "链接被拒绝访问 (HTTP 403)，可能已开启防盗链"
	case http.StatusUnauthorized:
		return "fail", "链接需要鉴权 (HTTP 401)，可能已改为签名地址"
	default:
		return "fail", fmt.Sprintf("异常状态码 HTTP %d", code)
	}
}
