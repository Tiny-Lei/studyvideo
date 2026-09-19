package alert

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// Alerter 记录告警日志，并在配置了 webhook 时推送（企业微信/钉钉群机器人等）。
type Alerter struct {
	webhookURL string
	client     *http.Client
}

func New(webhookURL string) *Alerter {
	return &Alerter{
		webhookURL: webhookURL,
		client:     &http.Client{Timeout: 5 * time.Second},
	}
}

func (a *Alerter) Send(msg string) {
	slog.Warn("告警", "message", msg)
	if a.webhookURL == "" {
		return
	}
	go func() {
		body := fmt.Sprintf(`{"msgtype":"text","text":{"content":%q}}`, msg)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.webhookURL, strings.NewReader(body))
		if err != nil {
			return
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := a.client.Do(req)
		if err != nil {
			slog.Error("发送 webhook 告警失败", "error", err)
			return
		}
		_, _ = io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}()
}
