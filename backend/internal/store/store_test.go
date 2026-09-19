package store

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"
)

// 需要真实 MySQL 的集成测试；设置 TEST_DB_DSN 后运行，例如：
//
//	TEST_DB_DSN='studyvideo:密码@tcp(127.0.0.1:3306)/studyvideo?parseTime=true&charset=utf8mb4&loc=Local' go test ./...
//
// 测试会使用独立的 <库名>_store 数据库，避免与其它测试包相互干扰。
func newTestStore(t *testing.T) *Store {
	t.Helper()
	dsn := os.Getenv("TEST_DB_DSN")
	if dsn == "" {
		t.Skip("未设置 TEST_DB_DSN，跳过 MySQL 集成测试")
	}
	dsn = isolatedDSN(t, dsn, "_store")
	db, err := Open(dsn)
	if err != nil {
		t.Fatalf("连接测试库失败: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	s := New(db)
	ctx := context.Background()
	for _, table := range []string{"video_stats", "pdfs", "videos", "categories", "topics", "ip_daily", "blocked_ips"} {
		if _, err := db.ExecContext(ctx, "DELETE FROM "+table); err != nil {
			t.Fatalf("清理表 %s 失败: %v", table, err)
		}
	}
	return s
}

// isolatedDSN 在 DSN 的库名后追加后缀，得到该测试包专用的数据库。
func isolatedDSN(t *testing.T, dsn, suffix string) string {
	t.Helper()
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		t.Fatalf("解析 TEST_DB_DSN 失败: %v", err)
	}
	if cfg.DBName == "" {
		cfg.DBName = "studyvideo"
	}
	cfg.DBName += suffix
	return cfg.FormatDSN()
}

func TestStoreTopicsAndVideos(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	topic := &Topic{Name: "GESP 真题讲解", Description: "GESP 各级真题", Sort: 10}
	if err := s.CreateTopic(ctx, topic); err != nil {
		t.Fatalf("创建主题失败: %v", err)
	}
	if topic.ID == 0 {
		t.Fatal("创建主题后应返回 ID")
	}

	category := &Category{TopicID: topic.ID, Name: "一级真题", Description: "GESP 一级", Sort: 5}
	if err := s.CreateCategory(ctx, category); err != nil {
		t.Fatalf("创建分类失败: %v", err)
	}
	if category.ID == 0 {
		t.Fatal("创建分类后应返回 ID")
	}

	url := "https://static.hetaoimg.com/crmFiles/b3fdb68074174e58b2408b20f4185281.mp4"
	v := &Video{TopicID: topic.ID, CategoryID: category.ID, Title: "GESP 一级第 1 题", URL: url, Tags: "GESP,一级"}
	if err := s.CreateVideo(ctx, v); err != nil {
		t.Fatalf("创建视频失败: %v", err)
	}

	if err := s.CreateVideo(ctx, &Video{TopicID: topic.ID, CategoryID: category.ID, Title: "重复链接", URL: url}); !errors.Is(err, ErrDuplicate) {
		t.Fatalf("重复链接应返回 ErrDuplicate, got %v", err)
	}

	videos, err := s.PublicVideosByCategory(ctx, category.ID, "")
	if err != nil || len(videos) != 1 {
		t.Fatalf("分类视频查询失败: %v, len=%d", err, len(videos))
	}
	if videos[0].URL != "" {
		t.Fatal("公开列表不应返回视频直链")
	}
	if videos[0].CategoryName != "一级真题" || videos[0].TopicName != "GESP 真题讲解" {
		t.Fatalf("视频应带分类与主题名称: %+v", videos[0])
	}

	topicVideos, err := s.PublicVideosByTopic(ctx, topic.ID, "")
	if err != nil || len(topicVideos) != 1 {
		t.Fatalf("主题视频查询失败: %v, len=%d", err, len(topicVideos))
	}

	cats, err := s.ListCategories(ctx, topic.ID)
	if err != nil || len(cats) != 1 || cats[0].VideoCount != 1 {
		t.Fatalf("分类列表异常: %v %+v", err, cats)
	}

	found, err := s.SearchPublicVideos(ctx, "一级", 10)
	if err != nil || len(found) != 1 {
		t.Fatalf("搜索失败: %v, len=%d", err, len(found))
	}
	found, _ = s.SearchPublicVideos(ctx, "GESP 真题", 10)
	if len(found) != 1 {
		t.Fatalf("按主题名搜索应命中, len=%d", len(found))
	}

	if _, err := s.GetVideoPublic(ctx, v.ID); err != nil {
		t.Fatalf("公开详情查询失败: %v", err)
	}
	full, err := s.GetVideo(ctx, v.ID)
	if err != nil || full.URL != url {
		t.Fatalf("管理端查询应带直链: %v", err)
	}

	if err := s.UpdateVideoCheck(ctx, v.ID, "fail", 403, 120, "链接被拒绝访问", time.Now()); err != nil {
		t.Fatalf("更新检测状态失败: %v", err)
	}
	got, _ := s.GetVideo(ctx, v.ID)
	if got.Status != "fail" || got.StatusCode != 403 {
		t.Fatalf("检测状态未生效: %+v", got)
	}

	// 链接不变时保持状态；链接变化时重置为未检测
	if err := s.UpdateVideo(ctx, &Video{ID: v.ID, TopicID: topic.ID, CategoryID: category.ID, Title: "改名", URL: url}); err != nil {
		t.Fatalf("更新视频失败: %v", err)
	}
	got, _ = s.GetVideo(ctx, v.ID)
	if got.Status != "fail" {
		t.Fatalf("标题/排序变化不应重置状态, got %s", got.Status)
	}
	newURL := "https://static.hetaoimg.com/crmFiles/other.mp4"
	if err := s.UpdateVideo(ctx, &Video{ID: v.ID, TopicID: topic.ID, CategoryID: category.ID, Title: "换链接", URL: newURL}); err != nil {
		t.Fatalf("更新视频失败: %v", err)
	}
	got, _ = s.GetVideo(ctx, v.ID)
	if got.Status != "unchecked" || got.CheckedAt != nil {
		t.Fatalf("链接变化应重置状态, got %+v", got)
	}

	stats, err := s.Stats(ctx)
	if err != nil || stats.Topics != 1 || stats.Categories != 1 || stats.Videos != 1 || stats.Unchecked != 1 {
		t.Fatalf("统计异常: %+v, err=%v", stats, err)
	}

	// 配套资料（本站存储）增删改查与级联
	p := &PDF{VideoID: v.ID, Title: "真题试卷", FilePath: "2026/09/abc.pdf", FileName: "试卷.pdf", FileSize: 1024, MimeType: "application/pdf", Sort: 2}
	if err := s.CreatePDF(ctx, p); err != nil {
		t.Fatalf("创建资料失败: %v", err)
	}
	if p.ID == 0 {
		t.Fatal("创建资料后应返回 ID")
	}
	pub, err := s.ListPDFsPublic(ctx, v.ID)
	if err != nil || len(pub) != 1 {
		t.Fatalf("公开资料查询失败: %v len=%d", err, len(pub))
	}
	if pub[0].FilePath != "" {
		t.Fatal("公开资料不应返回文件路径")
	}
	if pub[0].FileName != "试卷.pdf" || pub[0].FileSize != 1024 {
		t.Fatalf("公开资料字段错误: %+v", pub[0])
	}
	if err := s.UpdatePDFMeta(ctx, p.ID, "改名后的试卷", 5); err != nil {
		t.Fatalf("更新资料元信息失败: %v", err)
	}
	gotPDF, _ := s.GetPDF(ctx, p.ID)
	if gotPDF.Title != "改名后的试卷" || gotPDF.Sort != 5 || gotPDF.FilePath != "2026/09/abc.pdf" {
		t.Fatalf("资料更新未生效: %+v", gotPDF)
	}
	paths, err := s.PDFFilePathsByTopic(ctx, topic.ID)
	if err != nil || len(paths) != 1 || paths[0] != "2026/09/abc.pdf" {
		t.Fatalf("按主题查询文件路径失败: %v %v", err, paths)
	}
	all, err := s.AllPDFFilePaths(ctx)
	if err != nil || len(all) != 1 {
		t.Fatalf("查询全部文件路径失败: %v %v", err, all)
	}

	// 删除分类：视频保留并移动到默认分类
	if _, err := s.DeleteCategory(ctx, category.ID, true); err != nil {
		t.Fatalf("删除分类失败: %v", err)
	}
	got, _ = s.GetVideo(ctx, v.ID)
	if got.CategoryID == 0 || got.CategoryID == category.ID {
		t.Fatalf("视频应被移动到默认分类, got category_id=%d", got.CategoryID)
	}
	fallback, err := s.GetCategory(ctx, got.CategoryID)
	if err != nil || fallback.Name != defaultCategoryName {
		t.Fatalf("默认分类异常: %v %+v", err, fallback)
	}

	// 重新建一个分类用于级联删除验证
	category2 := &Category{TopicID: topic.ID, Name: "二级真题", Sort: 1}
	if err := s.CreateCategory(ctx, category2); err != nil {
		t.Fatal(err)
	}
	if err := s.UpdateVideo(ctx, &Video{ID: v.ID, TopicID: topic.ID, CategoryID: category2.ID, Title: "换分类", URL: newURL}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DeleteCategory(ctx, category2.ID, false); err != nil {
		t.Fatalf("删除分类(含视频)失败: %v", err)
	}
	if _, err := s.GetVideo(ctx, v.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal("mode=delete 时视频应被删除")
	}

	if err := s.DeleteTopic(ctx, topic.ID); err != nil {
		t.Fatalf("删除主题失败: %v", err)
	}
	if _, err := s.GetVideo(ctx, v.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal("删除主题后视频应级联删除")
	}
	if _, err := s.GetPDF(ctx, p.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal("删除主题后资料应级联删除")
	}
}
func TestStoreDailyAndBlocked(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	ip := "203.0.113.7"

	vh, th, err := s.IncrDaily(ctx, ip, true)
	if err != nil || vh != 1 || th != 1 {
		t.Fatalf("首次计数异常: vh=%d th=%d err=%v", vh, th, err)
	}
	vh, th, _ = s.IncrDaily(ctx, ip, true)
	if vh != 2 || th != 2 {
		t.Fatalf("累加计数异常: vh=%d th=%d", vh, th)
	}
	_, th, _ = s.IncrDaily(ctx, ip, false)
	if th != 3 {
		t.Fatalf("总请求计数异常: th=%d", th)
	}

	until := time.Now().Add(time.Hour).Truncate(time.Second)
	if err := s.SetBlocked(ctx, ip, "测试封禁", until); err != nil {
		t.Fatalf("封禁失败: %v", err)
	}
	blocked, gotUntil, err := s.IsBlocked(ctx, ip)
	if err != nil || !blocked || !gotUntil.Equal(until) {
		t.Fatalf("封禁查询异常: blocked=%v until=%v err=%v", blocked, gotUntil, err)
	}
	list, err := s.ListBlocked(ctx)
	if err != nil || len(list) != 1 {
		t.Fatalf("封禁列表异常: %v len=%d", err, len(list))
	}
	if err := s.DeleteBlocked(ctx, ip); err != nil {
		t.Fatalf("解封失败: %v", err)
	}
	if blocked, _, _ := s.IsBlocked(ctx, ip); blocked {
		t.Fatal("解封后不应再命中")
	}

	// 过期记录应被清理
	if err := s.SetBlocked(ctx, "198.51.100.9", "过期", time.Now().Add(-time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := s.CleanupBlocked(ctx); err != nil {
		t.Fatal(err)
	}
	if blocked, _, _ := s.IsBlocked(ctx, "198.51.100.9"); blocked {
		t.Fatal("过期封禁应被清理")
	}
}

func TestVideoStatsDedup(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	topic := &Topic{Name: "统计主题"}
	if err := s.CreateTopic(ctx, topic); err != nil {
		t.Fatal(err)
	}
	category := &Category{TopicID: topic.ID, Name: "统计分类"}
	if err := s.CreateCategory(ctx, category); err != nil {
		t.Fatal(err)
	}
	v := &Video{TopicID: topic.ID, CategoryID: category.ID, Title: "统计视频", URL: "https://example.com/stat.mp4"}
	if err := s.CreateVideo(ctx, v); err != nil {
		t.Fatal(err)
	}

	// 同一 IP 同一天：UV 只计 1，PV 累加
	for i := 0; i < 3; i++ {
		if err := s.RecordVisit(ctx, v.ID, "203.0.113.1"); err != nil {
			t.Fatalf("记录访问失败: %v", err)
		}
	}
	for i := 0; i < 2; i++ {
		if err := s.RecordWatch(ctx, v.ID, "203.0.113.1"); err != nil {
			t.Fatalf("记录观看失败: %v", err)
		}
	}
	// 第二个 IP 只看不访问（边界：先观看后访问）
	if err := s.RecordWatch(ctx, v.ID, "203.0.113.2"); err != nil {
		t.Fatal(err)
	}
	stats, err := s.VideoStatsFor(ctx, []int64{v.ID})
	if err != nil {
		t.Fatal(err)
	}
	st := stats[v.ID]
	if st.VisitUV != 1 || st.WatchUV != 2 {
		t.Fatalf("UV 去重不正确: %+v", st)
	}
	if st.VisitPV != 3 || st.WatchPV != 3 {
		t.Fatalf("PV 累加不正确: %+v", st)
	}

	// 观看后再访问，不应把 visited 清零
	if err := s.RecordVisit(ctx, v.ID, "203.0.113.2"); err != nil {
		t.Fatal(err)
	}
	stats, _ = s.VideoStatsFor(ctx, []int64{v.ID})
	st = stats[v.ID]
	if st.VisitUV != 2 || st.WatchUV != 2 {
		t.Fatalf("混合记录后 UV 不正确: %+v", st)
	}

	// 批量查询与空列表
	stats, err = s.VideoStatsFor(ctx, []int64{v.ID, 999999})
	if err != nil || len(stats) != 1 {
		t.Fatalf("批量查询异常: %v len=%d", err, len(stats))
	}
	empty, err := s.VideoStatsFor(ctx, nil)
	if err != nil || len(empty) != 0 {
		t.Fatalf("空查询应返回空 map: %v %v", err, empty)
	}

	// 删除视频应级联删除统计
	if err := s.DeleteVideo(ctx, v.ID); err != nil {
		t.Fatal(err)
	}
	after, _ := s.VideoStatsFor(ctx, []int64{v.ID})
	if len(after) != 0 {
		t.Fatal("删除视频后统计应级联清理")
	}
}
