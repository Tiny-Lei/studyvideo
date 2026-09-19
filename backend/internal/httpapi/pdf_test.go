package httpapi

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"studyvideo/internal/alert"
	"studyvideo/internal/auth"
	"studyvideo/internal/config"
	"studyvideo/internal/filestore"
	"studyvideo/internal/health"
	"studyvideo/internal/risk"
	"studyvideo/internal/store"

	"github.com/go-sql-driver/mysql"
)

type testEnv struct {
	api        *API
	mux        *http.ServeMux
	store      *store.Store
	files      *filestore.Store
	cfg        *config.Config
	db         *sql.DB
	topicID    int64
	categoryID int64
	videoID    int64
}

// isolatedHTTPDSN 使用独立的 <库名>_http 数据库，避免与其它测试包相互干扰。
func isolatedHTTPDSN(t *testing.T, dsn string) string {
	t.Helper()
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		t.Fatalf("解析 TEST_DB_DSN 失败: %v", err)
	}
	if cfg.DBName == "" {
		cfg.DBName = "studyvideo"
	}
	cfg.DBName += "_http"
	return cfg.FormatDSN()
}

// newTestEnv 搭建带真实 MySQL 与临时文件目录的测试环境；未设置 TEST_DB_DSN 时跳过。
func newTestEnv(t *testing.T) *testEnv {
	t.Helper()
	dsn := os.Getenv("TEST_DB_DSN")
	if dsn == "" {
		t.Skip("未设置 TEST_DB_DSN，跳过 HTTP 集成测试")
	}
	db, err := store.Open(isolatedHTTPDSN(t, dsn))
	if err != nil {
		t.Fatalf("连接测试库失败: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	ctx := context.Background()
	for _, table := range []string{"video_stats", "pdfs", "videos", "categories", "topics", "ip_daily", "blocked_ips"} {
		if _, err := db.ExecContext(ctx, "DELETE FROM "+table); err != nil {
			t.Fatalf("清理表 %s 失败: %v", table, err)
		}
	}

	st := store.New(db)
	cfg := &config.Config{
		AdminPassword:   "test-pass",
		SessionSecret:   []byte("test-secret"),
		BurstTotalLimit: 1000,
		BurstVideoLimit: 1000,
		BurstWindow:     time.Minute,
		DailyVideoLimit: 100000,
		DailyTotalLimit: 100000,
		BlockMinutes:    1,
		LoginAttempts:   100,
		LoginWindow:     time.Minute,
		CheckTimeout:    2 * time.Second,
		MaxPDFSizeBytes: 1 << 20,
	}
	auther := auth.New(cfg.AdminPassword, cfg.SessionSecret, false)
	alerter := alert.New("")
	riskMgr := risk.New(st, cfg, auther, alerter)
	checker := health.New(st, cfg, alerter)
	files, err := filestore.New(t.TempDir())
	if err != nil {
		t.Fatalf("初始化文件存储失败: %v", err)
	}
	api := New(st, cfg, auther, riskMgr, checker, files)
	mux := http.NewServeMux()
	api.Register(mux)

	topic := &store.Topic{Name: "测试主题"}
	if err := st.CreateTopic(ctx, topic); err != nil {
		t.Fatalf("创建主题失败: %v", err)
	}
	category := &store.Category{TopicID: topic.ID, Name: "测试分类"}
	if err := st.CreateCategory(ctx, category); err != nil {
		t.Fatalf("创建分类失败: %v", err)
	}
	video := &store.Video{TopicID: topic.ID, CategoryID: category.ID, Title: "测试视频", URL: "https://example.com/v.mp4"}
	if err := st.CreateVideo(ctx, video); err != nil {
		t.Fatalf("创建视频失败: %v", err)
	}
	return &testEnv{api: api, mux: mux, store: st, files: files, cfg: cfg, db: db, topicID: topic.ID, categoryID: category.ID, videoID: video.ID}
}

func (e *testEnv) do(t *testing.T, method, path string, body io.Reader, admin bool) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, body)
	if admin {
		req.Header.Set("X-Admin-Token", e.cfg.AdminPassword)
	}
	rec := httptest.NewRecorder()
	e.mux.ServeHTTP(rec, req)
	return rec
}

func (e *testEnv) upload(t *testing.T, path, title, filename string, content []byte) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	if title != "" {
		_ = mw.WriteField("title", title)
	}
	_ = mw.WriteField("sort", "3")
	fw, err := mw.CreateFormFile("file", filename)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fw.Write(content); err != nil {
		t.Fatal(err)
	}
	mw.Close()

	req := httptest.NewRequest(http.MethodPost, path, &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("X-Admin-Token", e.cfg.AdminPassword)
	rec := httptest.NewRecorder()
	e.mux.ServeHTTP(rec, req)
	return rec
}

const minimalPDF = "%PDF-1.4\n1 0 obj\n<< /Type /Catalog >>\nendobj\ntrailer\n<< /Root 1 0 R >>\n%%EOF\n"

func TestPDFUploadPreviewDownload(t *testing.T) {
	e := newTestEnv(t)

	// 未登录不能上传
	rec := e.do(t, http.MethodGet, fmt.Sprintf("/api/admin/videos/%d/pdfs", e.videoID), nil, false)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("未登录应返回 401, got %d", rec.Code)
	}

	// 上传 PDF
	rec = e.upload(t, fmt.Sprintf("/api/admin/videos/%d/pdfs", e.videoID), "真题试卷", "真题试卷.pdf", []byte(minimalPDF))
	if rec.Code != http.StatusOK {
		t.Fatalf("上传失败: %d %s", rec.Code, rec.Body.String())
	}
	var uploadResp struct {
		PDF store.PDF `json:"pdf"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &uploadResp); err != nil {
		t.Fatal(err)
	}
	if uploadResp.PDF.ID == 0 || uploadResp.PDF.FileName != "真题试卷.pdf" || uploadResp.PDF.FileSize == 0 {
		t.Fatalf("上传响应字段不完整: %+v", uploadResp.PDF)
	}
	if uploadResp.PDF.FilePath != "" {
		t.Fatal("上传响应不应暴露服务器文件路径")
	}

	// 磁盘上应有文件，且文件名不含中文/原始名
	entries, err := filepath.Glob(filepath.Join(e.files.Root(), "*", "*", "*.pdf"))
	if err != nil || len(entries) != 1 {
		t.Fatalf("磁盘文件异常: %v %v", err, entries)
	}

	// 非 PDF 内容应被拒绝
	rec = e.upload(t, fmt.Sprintf("/api/admin/videos/%d/pdfs", e.videoID), "伪装文件", "fake.pdf", []byte("hello world"))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("伪装 PDF 应被拒绝, got %d %s", rec.Code, rec.Body.String())
	}

	// 非 .pdf 扩展名应被拒绝
	rec = e.upload(t, fmt.Sprintf("/api/admin/videos/%d/pdfs", e.videoID), "文本", "note.txt", []byte(minimalPDF))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("非 PDF 扩展名应被拒绝, got %d", rec.Code)
	}

	// 空文件应被拒绝
	rec = e.upload(t, fmt.Sprintf("/api/admin/videos/%d/pdfs", e.videoID), "空", "empty.pdf", nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("空文件应被拒绝, got %d", rec.Code)
	}

	// 超出大小限制应被拒绝
	e.cfg.MaxPDFSizeBytes = 16
	rec = e.upload(t, fmt.Sprintf("/api/admin/videos/%d/pdfs", e.videoID), "超大", "big.pdf", bytes.Repeat([]byte("A"), 64))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("超限文件应被拒绝, got %d", rec.Code)
	}
	e.cfg.MaxPDFSizeBytes = 1 << 20

	// 公开详情包含资料元数据但不含文件路径
	rec = e.do(t, http.MethodGet, fmt.Sprintf("/api/videos/%d", e.videoID), nil, false)
	if rec.Code != http.StatusOK {
		t.Fatalf("公开详情失败: %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "真题试卷") {
		t.Fatal("公开详情应包含资料标题")
	}
	if strings.Contains(body, "file_path") || strings.Contains(body, e.files.Root()) {
		t.Fatal("公开详情不应包含文件路径")
	}

	pdfID := uploadResp.PDF.ID

	// 在线预览：inline + Range 支持
	rec = e.do(t, http.MethodGet, fmt.Sprintf("/api/pdfs/%d/file", pdfID), nil, false)
	if rec.Code != http.StatusOK {
		t.Fatalf("预览失败: %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/pdf" {
		t.Fatalf("Content-Type 错误: %s", ct)
	}
	if cd := rec.Header().Get("Content-Disposition"); !strings.HasPrefix(cd, "inline") {
		t.Fatalf("预览应为 inline: %s", cd)
	}
	if !strings.Contains(rec.Header().Get("Content-Disposition"), "filename*=UTF-8''") {
		t.Fatalf("中文文件名应使用 RFC 5987 编码: %s", rec.Header().Get("Content-Disposition"))
	}
	if rec.Body.String() != minimalPDF {
		t.Fatal("预览内容与上传内容不一致")
	}

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/pdfs/%d/file", pdfID), nil)
	req.Header.Set("Range", "bytes=0-9")
	rangeRec := httptest.NewRecorder()
	e.mux.ServeHTTP(rangeRec, req)
	if rangeRec.Code != http.StatusPartialContent {
		t.Fatalf("Range 请求应返回 206, got %d", rangeRec.Code)
	}
	if cr := rangeRec.Header().Get("Content-Range"); !strings.HasPrefix(cr, "bytes 0-9/") {
		t.Fatalf("Content-Range 错误: %s", cr)
	}

	// 下载：attachment
	rec = e.do(t, http.MethodGet, fmt.Sprintf("/api/pdfs/%d/download", pdfID), nil, false)
	if rec.Code != http.StatusOK {
		t.Fatalf("下载失败: %d", rec.Code)
	}
	if cd := rec.Header().Get("Content-Disposition"); !strings.HasPrefix(cd, "attachment") {
		t.Fatalf("下载应为 attachment: %s", cd)
	}
	if rec.Body.String() != minimalPDF {
		t.Fatal("下载内容与上传内容不一致")
	}

	// 旧 /open 链接兼容跳转
	rec = e.do(t, http.MethodGet, fmt.Sprintf("/api/pdfs/%d/open", pdfID), nil, false)
	if rec.Code != http.StatusFound || rec.Header().Get("Location") != fmt.Sprintf("/api/pdfs/%d/file", pdfID) {
		t.Fatalf("open 兼容跳转异常: %d %s", rec.Code, rec.Header().Get("Location"))
	}

	// 只改名称不换文件
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	_ = mw.WriteField("title", "改名后的试卷")
	_ = mw.WriteField("sort", "9")
	mw.Close()
	req = httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/admin/pdfs/%d", pdfID), &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("X-Admin-Token", e.cfg.AdminPassword)
	rec = httptest.NewRecorder()
	e.mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("更新失败: %d %s", rec.Code, rec.Body.String())
	}
	got, _ := e.store.GetPDF(context.Background(), pdfID)
	if got.Title != "改名后的试卷" || got.Sort != 9 {
		t.Fatalf("更新未生效: %+v", got)
	}
	if !e.files.Exists(got.FilePath) {
		t.Fatal("只改名称时文件应保留")
	}

	// 替换文件后旧文件应被清理
	oldPath := got.FilePath
	rec = e.uploadUpdate(t, pdfID, "替换后", "新试卷.pdf", []byte(minimalPDF+"% v2\n"))
	if rec.Code != http.StatusOK {
		t.Fatalf("替换文件失败: %d %s", rec.Code, rec.Body.String())
	}
	got, _ = e.store.GetPDF(context.Background(), pdfID)
	if got.FilePath == oldPath {
		t.Fatal("替换后文件路径应变化")
	}
	if e.files.Exists(oldPath) {
		t.Fatal("被替换的旧文件应被删除")
	}
	if got.FileName != "新试卷.pdf" {
		t.Fatalf("替换后文件名未更新: %s", got.FileName)
	}

	// 删除资料应同时删除文件
	filePath := got.FilePath
	rec = e.do(t, http.MethodDelete, fmt.Sprintf("/api/admin/pdfs/%d", pdfID), nil, true)
	if rec.Code != http.StatusOK {
		t.Fatalf("删除资料失败: %d", rec.Code)
	}
	if e.files.Exists(filePath) {
		t.Fatal("删除资料后文件应被清理")
	}
	if _, err := e.store.GetPDF(context.Background(), pdfID); err == nil {
		t.Fatal("删除资料后记录应消失")
	}
}

func (e *testEnv) uploadUpdate(t *testing.T, pdfID int64, title, filename string, content []byte) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	_ = mw.WriteField("title", title)
	_ = mw.WriteField("sort", "1")
	fw, _ := mw.CreateFormFile("file", filename)
	_, _ = fw.Write(content)
	mw.Close()

	req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/admin/pdfs/%d", pdfID), &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("X-Admin-Token", e.cfg.AdminPassword)
	rec := httptest.NewRecorder()
	e.mux.ServeHTTP(rec, req)
	return rec
}

func TestPDFCleanupOnVideoAndTopicDelete(t *testing.T) {
	e := newTestEnv(t)
	ctx := context.Background()

	rec := e.upload(t, fmt.Sprintf("/api/admin/videos/%d/pdfs", e.videoID), "随视频删除", "a.pdf", []byte(minimalPDF))
	if rec.Code != http.StatusOK {
		t.Fatalf("上传失败: %d %s", rec.Code, rec.Body.String())
	}
	pdfs, _ := e.store.PDFsByVideo(ctx, e.videoID)
	if len(pdfs) != 1 {
		t.Fatalf("资料数量异常: %d", len(pdfs))
	}
	filePath := pdfs[0].FilePath

	rec = e.do(t, http.MethodDelete, fmt.Sprintf("/api/admin/videos/%d", e.videoID), nil, true)
	if rec.Code != http.StatusOK {
		t.Fatalf("删除视频失败: %d", rec.Code)
	}
	if e.files.Exists(filePath) {
		t.Fatal("删除视频应清理资料文件")
	}

	// 主题级联
	video := &store.Video{TopicID: e.topicID, CategoryID: e.categoryID, Title: "级联视频", URL: "https://example.com/b.mp4"}
	if err := e.store.CreateVideo(ctx, video); err != nil {
		t.Fatal(err)
	}
	rec = e.upload(t, fmt.Sprintf("/api/admin/videos/%d/pdfs", video.ID), "随主题删除", "b.pdf", []byte(minimalPDF))
	if rec.Code != http.StatusOK {
		t.Fatalf("上传失败: %d %s", rec.Code, rec.Body.String())
	}
	pdfs, _ = e.store.PDFsByVideo(ctx, video.ID)
	filePath = pdfs[0].FilePath

	rec = e.do(t, http.MethodDelete, fmt.Sprintf("/api/admin/topics/%d", e.topicID), nil, true)
	if rec.Code != http.StatusOK {
		t.Fatalf("删除主题失败: %d", rec.Code)
	}
	if e.files.Exists(filePath) {
		t.Fatal("删除主题应清理资料文件")
	}
}

func TestPDFTraversalSafety(t *testing.T) {
	e := newTestEnv(t)
	ctx := context.Background()
	p := &store.PDF{VideoID: e.videoID, Title: "恶意路径", FilePath: "../../etc/passwd", FileName: "x.pdf", FileSize: 1, MimeType: "application/pdf"}
	if err := e.store.CreatePDF(ctx, p); err != nil {
		t.Fatal(err)
	}
	rec := e.do(t, http.MethodGet, fmt.Sprintf("/api/pdfs/%d/file", p.ID), nil, false)
	if rec.Code != http.StatusInternalServerError && rec.Code != http.StatusNotFound {
		t.Fatalf("路径穿越应被拒绝, got %d", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "root:") {
		t.Fatal("不应读取到系统文件")
	}
}

func TestVideoStatsEndpoints(t *testing.T) {
	e := newTestEnv(t)
	ctx := context.Background()

	// 访问详情页应记录访问（按 IP 去重）
	for i := 0; i < 2; i++ {
		rec := e.do(t, http.MethodGet, fmt.Sprintf("/api/videos/%d", e.videoID), nil, false)
		if rec.Code != http.StatusOK {
			t.Fatalf("详情页请求失败: %d", rec.Code)
		}
	}
	// 模拟不同 IP
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/videos/%d", e.videoID), nil)
	req.RemoteAddr = "127.0.0.1:12345" // 回环来源才会信任 X-Forwarded-For
	req.Header.Set("X-Forwarded-For", "198.51.100.10")
	rec := httptest.NewRecorder()
	e.mux.ServeHTTP(rec, req)

	stats, err := e.store.VideoStatsFor(ctx, []int64{e.videoID})
	if err != nil {
		t.Fatal(err)
	}
	st := stats[e.videoID]
	if st.VisitUV != 2 {
		t.Fatalf("期望 2 个独立访客，实际 %d", st.VisitUV)
	}
	if st.VisitPV != 3 {
		t.Fatalf("期望 3 次访问 PV，实际 %d", st.VisitPV)
	}
	if st.WatchUV != 0 || st.WatchPV != 0 {
		t.Fatalf("尚未播放不应有观看数据: %+v", st)
	}

	// 上报播放
	rec = e.do(t, http.MethodPost, fmt.Sprintf("/api/videos/%d/watch", e.videoID), nil, false)
	if rec.Code != http.StatusOK {
		t.Fatalf("上报观看失败: %d %s", rec.Code, rec.Body.String())
	}
	// 重复上报：UV 不变，PV 增加
	e.do(t, http.MethodPost, fmt.Sprintf("/api/videos/%d/watch", e.videoID), nil, false)
	stats, _ = e.store.VideoStatsFor(ctx, []int64{e.videoID})
	st = stats[e.videoID]
	if st.WatchUV != 1 || st.WatchPV != 2 {
		t.Fatalf("观看统计不正确: %+v", st)
	}
	if st.VisitUV != 2 || st.VisitPV != 3 {
		t.Fatalf("观看上报不应影响访问统计: %+v", st)
	}

	// 不存在的视频
	rec = e.do(t, http.MethodPost, "/api/videos/999999/watch", nil, false)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("不存在视频应返回 404, got %d", rec.Code)
	}

	// 管理端列表应附带统计
	rec = e.do(t, http.MethodGet, "/api/admin/videos?page=1&page_size=20", nil, true)
	if rec.Code != http.StatusOK {
		t.Fatalf("管理端列表失败: %d", rec.Code)
	}
	var listResp struct {
		Videos []store.Video `json:"videos"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &listResp); err != nil {
		t.Fatal(err)
	}
	if len(listResp.Videos) != 1 || listResp.Videos[0].Stats.WatchUV != 1 || listResp.Videos[0].Stats.VisitUV != 2 {
		t.Fatalf("管理端列表统计未附带: %+v", listResp.Videos)
	}
}
