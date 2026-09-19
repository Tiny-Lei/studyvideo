package health

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"studyvideo/internal/alert"
	"studyvideo/internal/config"
)

func newTestChecker() *Checker {
	cfg := &config.Config{CheckTimeout: 3 * time.Second, CheckWorkers: 2}
	return New(nil, cfg, alert.New(""))
}

func TestClassifyProbe(t *testing.T) {
	cases := []struct {
		code int
		ct   string
		want string
	}{
		{200, "video/mp4", "ok"},
		{206, "application/octet-stream", "ok"},
		{200, "text/html; charset=utf-8", "fail"},
		{404, "", "fail"},
		{403, "", "fail"},
		{500, "", "fail"},
	}
	for _, c := range cases {
		got, _ := classifyProbe(c.code, c.ct)
		if got != c.want {
			t.Errorf("classifyProbe(%d, %q) = %q, want %q", c.code, c.ct, got, c.want)
		}
	}
}

func TestProbe(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/ok.mp4", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "video/mp4")
		if r.Method == http.MethodGet && r.Header.Get("Range") != "" {
			w.WriteHeader(http.StatusPartialContent)
			_, _ = w.Write([]byte{0})
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/head405.mp4", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "video/mp4")
		w.WriteHeader(http.StatusPartialContent)
	})
	mux.HandleFunc("/fake.mp4", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/missing.mp4", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := newTestChecker()
	ctx := context.Background()

	if res := c.probe(ctx, srv.URL+"/ok.mp4"); res.Status != "ok" || res.Code != 200 {
		t.Errorf("正常链接应 ok, got %+v", res)
	}
	if res := c.probe(ctx, srv.URL+"/head405.mp4"); res.Status != "ok" || res.Code != 206 {
		t.Errorf("不支持 HEAD 时应回退 Range GET, got %+v", res)
	}
	if res := c.probe(ctx, srv.URL+"/fake.mp4"); res.Status != "fail" {
		t.Errorf("返回 HTML 应判为失效, got %+v", res)
	}
	if res := c.probe(ctx, srv.URL+"/missing.mp4"); res.Status != "fail" || res.Code != 404 {
		t.Errorf("404 应判为失效, got %+v", res)
	}
}
