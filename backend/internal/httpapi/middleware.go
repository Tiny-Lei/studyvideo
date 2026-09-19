package httpapi

import (
	"log/slog"
	"net/http"
	"time"
)

// CommonMiddleware 提供 panic 恢复、访问日志与基础安全响应头。
func CommonMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		defer func() {
			if rec := recover(); rec != nil {
				slog.Error("请求 panic", "path", r.URL.Path, "panic", rec)
				writeErr(w, http.StatusInternalServerError, "服务器内部错误")
			}
			slog.Debug("请求完成", "method", r.Method, "path", r.URL.Path, "duration", time.Since(start).String())
		}()
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("X-Frame-Options", "SAMEORIGIN")
		next.ServeHTTP(w, r)
	})
}
