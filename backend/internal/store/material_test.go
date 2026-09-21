package store

import (
	"context"
	"errors"
	"testing"
)

func TestMaterialStore(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	// 分类 CRUD
	cat := &MaterialCategory{Name: "GESP真题", Description: "GESP 历年真题", Sort: 10}
	if err := s.CreateMaterialCategory(ctx, cat); err != nil {
		t.Fatalf("创建资料分类失败: %v", err)
	}
	if cat.ID == 0 {
		t.Fatal("创建分类后应返回 ID")
	}
	cats, err := s.ListMaterialCategories(ctx)
	if err != nil || len(cats) != 1 || cats[0].MaterialCount != 0 {
		t.Fatalf("分类列表异常: %v %+v", err, cats)
	}

	// 创建资料（同分组：试卷 + 解析卷）
	paper := &Material{
		CategoryID: cat.ID, GroupName: "2024-03 一级", Title: "2024-03 一级 试卷",
		Tags: "GESP,试卷", FilePath: "2026/09/paper.pdf", FileName: "试卷.pdf",
		FileSize: 2048, MimeType: "application/pdf", FileExt: "pdf", Sort: 1,
	}
	if err := s.CreateMaterial(ctx, paper); err != nil {
		t.Fatalf("创建资料失败: %v", err)
	}
	answer := &Material{
		CategoryID: cat.ID, GroupName: "2024-03 一级", Title: "2024-03 一级 解析卷",
		Tags: "GESP,解析", FilePath: "2026/09/answer.md", FileName: "解析.md",
		FileSize: 512, MimeType: "text/markdown; charset=utf-8", FileExt: "md",
	}
	if err := s.CreateMaterial(ctx, answer); err != nil {
		t.Fatalf("创建资料失败: %v", err)
	}
	other := &Material{
		CategoryID: cat.ID, Title: "课件：循环与数组",
		FilePath: "2026/09/slides.pptx", FileName: "循环与数组.pptx",
		FileSize: 10240, MimeType: "application/vnd.openxmlformats-officedocument.presentationml.presentation", FileExt: "pptx",
	}
	if err := s.CreateMaterial(ctx, other); err != nil {
		t.Fatalf("创建资料失败: %v", err)
	}

	// 分类计数
	cats, _ = s.ListMaterialCategories(ctx)
	if cats[0].MaterialCount != 3 {
		t.Fatalf("分类资料数应为 3, got %d", cats[0].MaterialCount)
	}

	// 按分类列表（公开：不含路径）
	list, total, err := s.ListMaterials(ctx, MaterialFilter{CategoryID: cat.ID, Page: 1, PageSize: 20})
	if err != nil || total != 3 || len(list) != 3 {
		t.Fatalf("资料列表异常: %v total=%d len=%d", err, total, len(list))
	}
	for _, m := range list {
		if m.FilePath != "" {
			t.Fatal("公开资料列表不应返回服务器路径")
		}
	}

	// 按分组过滤 + 分组列表
	groupList, total, err := s.ListMaterials(ctx, MaterialFilter{CategoryID: cat.ID, GroupName: "2024-03 一级", Page: 1, PageSize: 20})
	if err != nil || total != 2 {
		t.Fatalf("分组过滤异常: %v total=%d", err, total)
	}
	_ = groupList
	groups, err := s.MaterialGroups(ctx, cat.ID)
	if err != nil || len(groups) != 1 || groups[0] != "2024-03 一级" {
		t.Fatalf("分组列表异常: %v %v", err, groups)
	}

	// 按格式过滤
	mdList, total, err := s.ListMaterials(ctx, MaterialFilter{CategoryID: cat.ID, FileExt: "md", Page: 1, PageSize: 20})
	if err != nil || total != 1 || mdList[0].ID != answer.ID {
		t.Fatalf("格式过滤异常: %v total=%d", err, total)
	}

	// 独立搜索（只搜资料表）
	found, err := s.SearchMaterials(ctx, "解析", 10)
	if err != nil || len(found) != 1 || found[0].ID != answer.ID {
		t.Fatalf("资料搜索异常: %v len=%d", err, len(found))
	}
	found, _ = s.SearchMaterials(ctx, "课件", 10)
	if len(found) != 1 {
		t.Fatalf("按标题搜索应命中, len=%d", len(found))
	}

	// 详情与更新
	got, err := s.GetMaterialPublic(ctx, paper.ID)
	if err != nil || got.FilePath != "" || got.CategoryName != "GESP真题" {
		t.Fatalf("公开详情异常: %v %+v", err, got)
	}
	full, err := s.GetMaterial(ctx, paper.ID)
	if err != nil || full.FilePath != "2026/09/paper.pdf" {
		t.Fatalf("管理端详情应含路径: %v", err)
	}
	if err := s.UpdateMaterial(ctx, &Material{
		ID: paper.ID, CategoryID: cat.ID, GroupName: "2024-03 一级",
		Title: "2024-03 一级 试卷（修订）", Tags: "GESP,试卷", FilePath: full.FilePath,
		FileName: full.FileName, FileSize: full.FileSize, MimeType: full.MimeType, FileExt: full.FileExt,
	}); err != nil {
		t.Fatalf("更新资料失败: %v", err)
	}
	got, _ = s.GetMaterial(ctx, paper.ID)
	if got.Title != "2024-03 一级 试卷（修订）" {
		t.Fatalf("更新未生效: %+v", got)
	}

	// 统计
	materials, categoryCount, err := s.MaterialStats(ctx)
	if err != nil || materials != 3 || categoryCount != 1 {
		t.Fatalf("统计异常: %v materials=%d categories=%d", err, materials, categoryCount)
	}

	// 分类下文件路径收集（删除前清理用）
	paths, err := s.PDFFilePathsByMaterialCategory(ctx, cat.ID)
	if err != nil || len(paths) != 3 {
		t.Fatalf("文件路径收集异常: %v %v", err, paths)
	}

	// 删除单个资料
	if err := s.DeleteMaterial(ctx, other.ID); err != nil {
		t.Fatalf("删除资料失败: %v", err)
	}
	if _, err := s.GetMaterial(ctx, other.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal("删除后应查不到")
	}

	// 删除分类 → 级联删除资料
	if err := s.DeleteMaterialCategory(ctx, cat.ID); err != nil {
		t.Fatalf("删除分类失败: %v", err)
	}
	if _, err := s.GetMaterial(ctx, paper.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal("删除分类后资料应级联删除")
	}
	if cats, _ := s.ListMaterialCategories(ctx); len(cats) != 0 {
		t.Fatal("分类应已删除")
	}
}
