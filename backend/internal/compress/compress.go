// Package compress 为 HTTP 响应提供 gzip 压缩。
//
// 设计要点：
//   - 仅压缩文本类响应（HTML/CSS/JS/JSON/XML/SVG），图片、PDF、视频等本身已压缩的内容跳过；
//   - 跳过 Range 请求（206 部分内容）与 HEAD，避免破坏断点续传语义；
//   - 已知 Content-Length 且小于阈值的响应不压缩，省去无谓开销；
//   - gzip.Writer 通过 sync.Pool 复用，降低 GC 压力。
package compress

import (
	"compress/gzip"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
)

const minCompressSize = 512

var writerPool = sync.Pool{
	New: func() any {
		w, _ := gzip.NewWriterLevel(io.Discard, gzip.DefaultCompression)
		return w
	},
}

// Middleware 返回带 gzip 压缩能力的 Handler 包装器。
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead ||
			r.Header.Get("Range") != "" ||
			strings.Contains(strings.ToLower(r.Header.Get("Connection")), "upgrade") ||
			!acceptsGzip(r) {
			next.ServeHTTP(w, r)
			return
		}
		w.Header().Add("Vary", "Accept-Encoding")
		cw := &compressWriter{ResponseWriter: w}
		defer cw.Close()
		next.ServeHTTP(cw, r)
	})
}

type compressWriter struct {
	http.ResponseWriter
	gz          *gzip.Writer
	wroteHeader bool
	compress    bool
}

func (w *compressWriter) WriteHeader(status int) {
	if w.wroteHeader {
		return
	}
	w.wroteHeader = true

	h := w.Header()
	if status >= 200 && status < 300 &&
		h.Get("Content-Encoding") == "" &&
		compressible(h.Get("Content-Type")) &&
		!tooSmall(h.Get("Content-Length")) {
		w.compress = true
		h.Del("Content-Length")
		h.Set("Content-Encoding", "gzip")
		gz := writerPool.Get().(*gzip.Writer)
		gz.Reset(w.ResponseWriter)
		w.gz = gz
	}
	w.ResponseWriter.WriteHeader(status)
}

func (w *compressWriter) Write(b []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	if w.compress {
		return w.gz.Write(b)
	}
	return w.ResponseWriter.Write(b)
}

// Flush 在流式场景下先冲出 gzip 缓冲，再调用底层 Flush。
func (w *compressWriter) Flush() {
	if w.compress && w.gz != nil {
		_ = w.gz.Flush()
	}
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (w *compressWriter) Close() {
	if w.gz == nil {
		return
	}
	_ = w.gz.Close()
	writerPool.Put(w.gz)
	w.gz = nil
}

func tooSmall(contentLength string) bool {
	if contentLength == "" {
		return false
	}
	n, err := strconv.ParseInt(contentLength, 10, 64)
	return err == nil && n < minCompressSize
}

func compressible(contentType string) bool {
	if contentType == "" {
		return false
	}
	ct, _, _ := strings.Cut(contentType, ";")
	ct = strings.ToLower(strings.TrimSpace(ct))
	switch ct {
	case "text/html", "text/css", "text/plain", "text/javascript", "application/javascript",
		"application/json", "application/manifest+json", "application/xml", "image/svg+xml":
		return true
	}
	return strings.HasPrefix(ct, "text/") ||
		strings.HasSuffix(ct, "+json") ||
		strings.HasSuffix(ct, "+xml")
}

func acceptsGzip(r *http.Request) bool {
	for _, part := range strings.Split(r.Header.Get("Accept-Encoding"), ",") {
		name, params, _ := strings.Cut(part, ";")
		if !strings.EqualFold(strings.TrimSpace(name), "gzip") {
			continue
		}
		q := 1.0
		for _, p := range strings.Split(params, ";") {
			if v, ok := strings.CutPrefix(strings.TrimSpace(p), "q="); ok {
				if f, err := strconv.ParseFloat(v, 64); err == nil {
					q = f
				}
			}
		}
		return q > 0
	}
	return false
}
