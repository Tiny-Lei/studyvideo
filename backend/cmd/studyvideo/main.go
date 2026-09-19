package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
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
	"studyvideo/internal/version"
	"studyvideo/internal/web"
)

func main() {
	showVersion := flag.Bool("version", false, "打印版本信息后退出")
	printConfig := flag.Bool("print-config", false, "打印当前生效配置（隐藏敏感值）后退出")
	flag.Parse()

	if *showVersion {
		fmt.Printf("StudyVideo %s\n", version.String())
		return
	}

	cfg := config.Load()
	initLogger(cfg)

	if *printConfig {
		cfg.Print(os.Stdout)
		return
	}

	slog.Info("StudyVideo 启动中", "version", version.Version, "commit", version.Commit, "built", version.BuildTime)

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
	go cleanupStats(ctx, st, cfg.StatsKeepDays)

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

// initLogger 按配置初始化全局日志（支持 text/json 与日志级别）。
func initLogger(cfg *config.Config) {
	opts := &slog.HandlerOptions{Level: cfg.LogLevelValue()}
	var handler slog.Handler
	if cfg.LogFormat == "json" {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}
	slog.SetDefault(slog.New(handler))
}

// cleanupStats 每天清理一次过早的访问/观看明细，避免统计表无限增长。
// keepDays <= 0 表示永久保留。
func cleanupStats(ctx context.Context, st *store.Store, keepDays int) {
	if keepDays <= 0 {
		slog.Info("视频统计明细永久保留（STATS_KEEP_DAYS=0）")
		return
	}
	cleanup := func() {
		if err := st.CleanupVideoStats(ctx, keepDays); err != nil {
			slog.Error("清理视频统计明细失败", "error", err)
		}
	}
	cleanup()
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			cleanup()
		}
	}
}
