package compress

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func gzipHandler(contentType string, body []byte) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", contentType)
		w.Write(body)
	})
}

func doRequest(t *testing.T, h http.Handler, method, accept string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, "/", nil)
	if accept != "" {
		req.Header.Set("Accept-Encoding", accept)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	Middleware(h).ServeHTTP(rec, req)
	return rec
}

func decompress(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	zr, err := gzip.NewReader(rec.Body)
	if err != nil {
		t.Fatalf("解压失败: %v", err)
	}
	defer zr.Close()
	out, err := io.ReadAll(zr)
	if err != nil {
		t.Fatalf("读取失败: %v", err)
	}
	return string(out)
}

func TestCompressJSON(t *testing.T) {
	body := []byte(strings.Repeat(`{"message":"题目讲解视频"}`, 100))
	rec := doRequest(t, gzipHandler("application/json; charset=utf-8", body), http.MethodGet, "gzip, deflate, br", nil)
	if enc := rec.Header().Get("Content-Encoding"); enc != "gzip" {
		t.Fatalf("应返回 gzip 编码, got %q", enc)
	}
	if rec.Header().Get("Content-Length") != "" {
		t.Fatal("压缩后不应保留 Content-Length")
	}
	if !strings.Contains(rec.Header().Get("Vary"), "Accept-Encoding") {
		t.Fatal("应设置 Vary: Accept-Encoding")
	}
	if got := decompress(t, rec); got != string(body) {
		t.Fatal("解压内容与原始内容不一致")
	}
	if rec.Body.Len() >= len(body) {
		t.Fatalf("压缩后应更小: %d >= %d", rec.Body.Len(), len(body))
	}
}

func TestNoGzipWhenNotAccepted(t *testing.T) {
	body := []byte(strings.Repeat("hello world ", 100))
	rec := doRequest(t, gzipHandler("text/html", body), http.MethodGet, "", nil)
	if rec.Header().Get("Content-Encoding") != "" {
		t.Fatal("客户端不支持时不应压缩")
	}
	if rec.Body.String() != string(body) {
		t.Fatal("内容应原样返回")
	}
	if strings.Contains(rec.Header().Get("Vary"), "Accept-Encoding") {
		t.Fatal("未压缩时不必设置 Vary")
	}
}

func TestNoGzipForAlreadyCompressedTypes(t *testing.T) {
	for _, ct := range []string{"application/pdf", "image/png", "video/mp4", "application/octet-stream", "application/zip"} {
		rec := doRequest(t, gzipHandler(ct, bytes.Repeat([]byte("x"), 4096)), http.MethodGet, "gzip", nil)
		if rec.Header().Get("Content-Encoding") != "" {
			t.Errorf("%s 不应被 gzip 压缩", ct)
		}
	}
}

func TestNoGzipForRangeAndHead(t *testing.T) {
	body := bytes.Repeat([]byte("range content "), 100)
	rec := doRequest(t, gzipHandler("text/plain", body), http.MethodGet, "gzip", map[string]string{"Range": "bytes=0-99"})
	if rec.Header().Get("Content-Encoding") != "" {
		t.Fatal("Range 请求不应压缩")
	}
	rec = doRequest(t, gzipHandler("text/plain", body), http.MethodHead, "gzip", nil)
	if rec.Header().Get("Content-Encoding") != "" {
		t.Fatal("HEAD 请求不应压缩")
	}
}

func TestNoGzipForTinyResponse(t *testing.T) {
	small := []byte(`{"ok":true}`)
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Length", "11")
		w.Write(small)
	})
	rec := doRequest(t, h, http.MethodGet, "gzip", nil)
	if rec.Header().Get("Content-Encoding") != "" {
		t.Fatal("小于阈值且已知长度时不应压缩")
	}
	if rec.Body.String() != string(small) {
		t.Fatal("小响应应原样返回")
	}
}

func TestAcceptsGzipQuality(t *testing.T) {
	cases := map[string]bool{
		"gzip":              true,
		"deflate, gzip;q=1": true,
		"gzip;q=0.5":        true,
		"gzip;q=0":          false,
		"gzip;q=0.0":        false,
		"br, deflate":       false,
		"":                  false,
	}
	for header, want := range cases {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		if header != "" {
			req.Header.Set("Accept-Encoding", header)
		}
		if got := acceptsGzip(req); got != want {
			t.Errorf("acceptsGzip(%q) = %v, want %v", header, got, want)
		}
	}
}

func TestConcurrentCompression(t *testing.T) {
	body := []byte(strings.Repeat("并发压缩测试内容", 500))
	h := Middleware(gzipHandler("text/html", body))
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Header.Set("Accept-Encoding", "gzip")
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Header().Get("Content-Encoding") != "gzip" {
				t.Errorf("并发下应返回 gzip")
			}
		}()
	}
	wg.Wait()
}
