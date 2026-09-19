package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"studyvideo/internal/alert"
	"studyvideo/internal/auth"
	"studyvideo/internal/compress"
	"studyvideo/internal/config"
	"studyvideo/internal/filestore"
	"studyvideo/internal/health"
	"studyvideo/internal/httpapi"
	"studyvideo/internal/risk"
	"studyvideo/internal/store"
	"studyvideo/internal/web"
)

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	cfg := config.Load()
	db, err := store.Open(cfg.DSN)
	if err != nil {
		slog.Error("数据库初始化失败", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	st := store.New(db)
	auther := auth.New(cfg.AdminPassword, cfg.SessionSecret, cfg.CookieSecure)
	alerter := alert.New(cfg.AlertWebhookURL)
	riskMgr := risk.New(st, cfg, auther, alerter)
	checker := health.New(st, cfg, alerter)
	files, err := filestore.New(cfg.DataDir)
	if err != nil {
		slog.Error("初始化文件存储失败", "error", err)
		os.Exit(1)
	}
	api := httpapi.New(st, cfg, auther, riskMgr, checker, files)
	slog.Info("资料存储目录", "dir", files.Root(), "max_pdf_mb", cfg.MaxPDFSizeBytes>>20)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	apiMux := http.NewServeMux()
	api.Register(apiMux)

	root := http.NewServeMux()
	root.Handle("/api/", riskMgr.Wrap(apiMux))
	root.Handle("/", web.Handler())

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           httpapi.CommonMiddleware(compress.Middleware(root)),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	riskMgr.Start(ctx)
	checker.Start(ctx)

	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("HTTP 服务异常退出", "error", err)
			stop()
		}
	}()
	slog.Info("StudyVideo 已启动", "addr", cfg.Addr,
		"daily_video_limit", cfg.DailyVideoLimit, "burst_video_limit", cfg.BurstVideoLimit)

	<-ctx.Done()
	slog.Info("正在关闭服务…")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("关闭服务失败", "error", err)
	}
}
