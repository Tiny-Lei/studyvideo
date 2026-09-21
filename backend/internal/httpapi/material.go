package httpapi

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"studyvideo/internal/store"
)

// 资料区：独立于视频区的内容库，支持 PDF / Markdown / Office 文档等的上传、浏览与下载。

var allowedMaterialExts = map[string]bool{
	".pdf": true, ".md": true, ".markdown": true,
	".doc": true, ".docx": true, ".ppt": true, ".pptx": true,
	".xls": true, ".xlsx": true, ".txt": true, ".zip": true,
}

// ---------------- 公开接口 ----------------

func (s *API) handleMaterialHome(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	categories, err := s.store.ListMaterialCategories(ctx)
	if err != nil {
		slog.Error("查询资料分类失败", "error", err)
		writeErr(w, http.StatusInternalServerError, "查询失败")
		return
	}
	total, categoryCount, err := s.store.MaterialStats(ctx)
	if err != nil {
		slog.Error("查询资料统计失败", "error", err)
		writeErr(w, http.StatusInternalServerError, "查询失败")
		return
	}
	latest, _, err := s.store.ListMaterials(ctx, store.MaterialFilter{Page: 1, PageSize: 8})
	if err != nil {
		slog.Error("查询最新资料失败", "error", err)
		writeErr(w, http.StatusInternalServerError, "查询失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"categories": categories,
		"latest":     latest,
		"stats":      map[string]int64{"materials": total, "categories": categoryCount},
	})
}

func (s *API) handleMaterialCategory(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "参数错误")
		return
	}
	ctx := r.Context()
	category, err := s.store.GetMaterialCategory(ctx, id)
	if errors.Is(err, store.ErrNotFound) {
		writeErr(w, http.StatusNotFound, "资料分类不存在")
		return
	}
	if err != nil {
		slog.Error("查询资料分类失败", "error", err)
		writeErr(w, http.StatusInternalServerError, "查询失败")
		return
	}
	materials, _, err := s.store.ListMaterials(ctx, store.MaterialFilter{CategoryID: id, Page: 1, PageSize: 200})
	if err != nil {
		slog.Error("查询资料失败", "error", err)
		writeErr(w, http.StatusInternalServerError, "查询失败")
		return
	}
	groups, err := s.store.MaterialGroups(ctx, id)
	if err != nil {
		slog.Error("查询分组失败", "error", err)
		writeErr(w, http.StatusInternalServerError, "查询失败")
		return
	}
	tags, err := s.store.MaterialTagStats(ctx, id)
	if err != nil {
		slog.Error("查询标签统计失败", "error", err)
		writeErr(w, http.StatusInternalServerError, "查询失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"category":  category,
		"materials": materials,
		"groups":    groups,
		"tags":      tags,
	})
}

func (s *API) handleMaterials(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	categoryID, _ := strconv.ParseInt(q.Get("category_id"), 10, 64)
	page, _ := strconv.Atoi(q.Get("page"))
	if page <= 0 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(q.Get("page_size"))
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}
	f := store.MaterialFilter{
		CategoryID: categoryID,
		GroupName:  trim(q.Get("group")),
		Keyword:    trim(q.Get("q")),
		FileExt:    strings.ToLower(trim(q.Get("ext"))),
		Page:       page,
		PageSize:   pageSize,
	}
	materials, total, err := s.store.ListMaterials(r.Context(), f)
	if err != nil {
		slog.Error("查询资料失败", "error", err)
		writeErr(w, http.StatusInternalServerError, "查询失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"materials": materials, "total": total, "page": page, "page_size": pageSize,
	})
}

// handleMaterialSearch 资料独立搜索：只返回资料，不涉及视频。
func (s *API) handleMaterialSearch(w http.ResponseWriter, r *http.Request) {
	q := trim(r.URL.Query().Get("q"))
	if q == "" {
		writeJSON(w, http.StatusOK, map[string]any{"q": "", "materials": []store.Material{}})
		return
	}
	materials, err := s.store.SearchMaterials(r.Context(), q, 100)
	if err != nil {
		slog.Error("资料搜索失败", "error", err)
		writeErr(w, http.StatusInternalServerError, "搜索失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"q": q, "materials": materials})
}

func (s *API) handleMaterialDetail(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "参数错误")
		return
	}
	material, err := s.store.GetMaterialPublic(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeErr(w, http.StatusNotFound, "资料不存在")
		return
	}
	if err != nil {
		slog.Error("查询资料失败", "error", err)
		writeErr(w, http.StatusInternalServerError, "查询失败")
		return
	}
	category, err := s.store.GetMaterialCategory(r.Context(), material.CategoryID)
	if err != nil {
		slog.Error("查询资料分类失败", "error", err)
		writeErr(w, http.StatusInternalServerError, "查询失败")
		return
	}
	// 同分组（同套题）下的其它资料，便于看「试卷 / 解析卷」切换
	var siblings []store.Material
	if material.GroupName != "" {
		all, _, err := s.store.ListMaterials(r.Context(), store.MaterialFilter{
			CategoryID: material.CategoryID, GroupName: material.GroupName, Page: 1, PageSize: 50,
		})
		if err == nil {
			for _, m := range all {
				if m.ID != material.ID {
					siblings = append(siblings, m)
				}
			}
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"material": material, "category": category, "siblings": siblings,
	})
}

// handleMaterialFile 提供在线预览（inline）与下载（attachment）：
//   - /api/materials/{id}/file     → 在线查看（MD 前端拉取原文渲染）
//   - /api/materials/{id}/download → 下载
func (s *API) handleMaterialFile(download bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := pathID(r)
		if err != nil {
			writeErr(w, http.StatusBadRequest, "参数错误")
			return
		}
		material, err := s.store.GetMaterial(r.Context(), id)
		if errors.Is(err, store.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "资料不存在")
			return
		}
		if err != nil {
			slog.Error("查询资料失败", "error", err)
			writeErr(w, http.StatusInternalServerError, "查询失败")
			return
		}
		s.serveStoredFile(w, r, material.FilePath, material.FileName, material.MimeType, download)
	}
}

// ---------------- 管理端接口 ----------------

type materialCategoryInput struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Tags        string `json:"tags"`
	Sort        int    `json:"sort"`
}

func (s *API) handleAdminMaterialCategories(w http.ResponseWriter, r *http.Request) {
	categories, err := s.store.ListMaterialCategories(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "查询失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"categories": categories})
}

func (s *API) handleCreateMaterialCategory(w http.ResponseWriter, r *http.Request) {
	var req materialCategoryInput
	if err := readJSON(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	req.Name = trim(req.Name)
	if req.Name == "" {
		writeErr(w, http.StatusBadRequest, "分类名称不能为空")
		return
	}
	c := &store.MaterialCategory{Name: req.Name, Description: trim(req.Description), Tags: normalizeTags(req.Tags), Sort: req.Sort}
	if err := s.store.CreateMaterialCategory(r.Context(), c); err != nil {
		slog.Error("创建资料分类失败", "error", err)
		writeErr(w, http.StatusInternalServerError, "创建失败")
		return
	}
	if created, err := s.store.GetMaterialCategory(r.Context(), c.ID); err == nil {
		c = created
	}
	writeJSON(w, http.StatusOK, map[string]any{"category": c})
}

func (s *API) handleUpdateMaterialCategory(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "参数错误")
		return
	}
	var req materialCategoryInput
	if err := readJSON(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	req.Name = trim(req.Name)
	if req.Name == "" {
		writeErr(w, http.StatusBadRequest, "分类名称不能为空")
		return
	}
	c := &store.MaterialCategory{ID: id, Name: req.Name, Description: trim(req.Description), Tags: normalizeTags(req.Tags), Sort: req.Sort}
	err = s.store.UpdateMaterialCategory(r.Context(), c)
	if errors.Is(err, store.ErrNotFound) {
		writeErr(w, http.StatusNotFound, "分类不存在")
		return
	}
	if err != nil {
		slog.Error("更新资料分类失败", "error", err)
		writeErr(w, http.StatusInternalServerError, "更新失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// handleDeleteMaterialCategory 删除资料分类：mode=delete 时连同文件一起删除，默认拒绝删除非空分类。
func (s *API) handleDeleteMaterialCategory(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "参数错误")
		return
	}
	category, err := s.store.GetMaterialCategory(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeErr(w, http.StatusNotFound, "分类不存在")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "查询失败")
		return
	}
	mode := trim(r.URL.Query().Get("mode"))
	if category.MaterialCount > 0 && mode != "delete" {
		writeErr(w, http.StatusConflict,
			fmt.Sprintf("该分类下还有 %d 份资料，请先移动或使用「连同资料一起删除」", category.MaterialCount))
		return
	}

	var paths []string
	if mode == "delete" {
		paths, err = s.store.PDFFilePathsByMaterialCategory(r.Context(), id)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "删除失败")
			return
		}
	}
	if err := s.store.DeleteMaterialCategory(r.Context(), id); err != nil {
		slog.Error("删除资料分类失败", "error", err)
		writeErr(w, http.StatusInternalServerError, "删除失败")
		return
	}
	for _, p := range paths {
		if err := s.files.Remove(p); err != nil {
			slog.Warn("删除资料文件失败", "path", p, "error", err)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "removed_files": len(paths)})
}

func (s *API) handleAdminMaterials(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	categoryID, _ := strconv.ParseInt(q.Get("category_id"), 10, 64)
	f := store.MaterialFilter{
		CategoryID: categoryID,
		GroupName:  trim(q.Get("group")),
		Keyword:    trim(q.Get("q")),
		FileExt:    strings.ToLower(trim(q.Get("ext"))),
		Page:       atoi(q.Get("page"), 1),
		PageSize:   atoi(q.Get("page_size"), 20),
	}
	materials, total, err := s.store.ListMaterials(r.Context(), f)
	if err != nil {
		slog.Error("查询资料失败", "error", err)
		writeErr(w, http.StatusInternalServerError, "查询失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"materials": materials, "total": total, "page": f.Page, "page_size": f.PageSize,
	})
}

// handleUploadMaterial 上传资料（multipart）：文件 + 分类 + 分组 + 标题 + 标签 + 备注。
func (s *API) handleUploadMaterial(w http.ResponseWriter, r *http.Request) {
	maxBytes := s.cfg.MaxMaterialSizeBytes
	r.Body = http.MaxBytesReader(w, r.Body, maxBytes+maxUploadMemory)
	if err := r.ParseMultipartForm(maxUploadMemory); err != nil {
		writeErr(w, http.StatusBadRequest, fmt.Sprintf("解析上传内容失败（单文件不能超过 %d MB）", maxBytes>>20))
		return
	}
	defer func() {
		if r.MultipartForm != nil {
			_ = r.MultipartForm.RemoveAll()
		}
	}()

	categoryID, _ := strconv.ParseInt(trim(r.FormValue("category_id")), 10, 64)
	if categoryID <= 0 {
		writeErr(w, http.StatusBadRequest, "请选择资料分类")
		return
	}
	if ok, _ := s.store.MaterialCategoryExists(r.Context(), categoryID); !ok {
		writeErr(w, http.StatusBadRequest, "资料分类不存在")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeErr(w, http.StatusBadRequest, "请选择要上传的文件")
		return
	}
	defer file.Close()

	ext := strings.ToLower(filepath.Ext(header.Filename))
	if !allowedMaterialExts[ext] {
		writeErr(w, http.StatusBadRequest, "不支持的文件类型（支持 PDF / Markdown / Office 文档 / TXT / ZIP）")
		return
	}

	title := trim(r.FormValue("title"))
	if title == "" {
		title = strings.TrimSuffix(header.Filename, filepath.Ext(header.Filename))
	}
	if title == "" {
		title = "未命名资料"
	}
	sortN, _ := strconv.Atoi(trim(r.FormValue("sort")))

	relPath, size, err := s.files.Save(file, ext, maxBytes)
	if err != nil {
		slog.Error("保存资料文件失败", "error", err)
		writeErr(w, http.StatusBadRequest, "保存文件失败："+err.Error())
		return
	}
	// PDF 做文件头校验，防止改扩展名伪装
	if ext == ".pdf" {
		if ok, err := s.files.LooksLikePDF(relPath); err != nil || !ok {
			_ = s.files.Remove(relPath)
			writeErr(w, http.StatusBadRequest, "文件内容不是有效的 PDF")
			return
		}
	}

	m := &store.Material{
		CategoryID:  categoryID,
		GroupName:   trim(r.FormValue("group_name")),
		Title:       title,
		Description: trim(r.FormValue("description")),
		Tags:        normalizeTags(r.FormValue("tags")),
		FilePath:    relPath,
		FileName:    sanitizeFileName(header.Filename),
		FileSize:    size,
		MimeType:    materialMime(ext),
		FileExt:     strings.TrimPrefix(ext, "."),
		Sort:        sortN,
	}
	if err := s.store.CreateMaterial(r.Context(), m); err != nil {
		_ = s.files.Remove(relPath)
		slog.Error("写入资料记录失败", "error", err)
		writeErr(w, http.StatusInternalServerError, "保存记录失败")
		return
	}
	if saved, err := s.store.GetMaterial(r.Context(), m.ID); err == nil {
		m = saved
	}
	m.FilePath = ""
	writeJSON(w, http.StatusOK, map[string]any{"material": m})
}

// handleUpdateMaterial 更新资料元信息；携带 file 时替换文件。
func (s *API) handleUpdateMaterial(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "参数错误")
		return
	}
	existing, err := s.store.GetMaterial(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeErr(w, http.StatusNotFound, "资料不存在")
		return
	}
	if err != nil {
		slog.Error("查询资料失败", "error", err)
		writeErr(w, http.StatusInternalServerError, "查询失败")
		return
	}

	maxBytes := s.cfg.MaxMaterialSizeBytes
	r.Body = http.MaxBytesReader(w, r.Body, maxBytes+maxUploadMemory)
	if err := r.ParseMultipartForm(maxUploadMemory); err != nil {
		writeErr(w, http.StatusBadRequest, "解析上传内容失败")
		return
	}
	defer func() {
		if r.MultipartForm != nil {
			_ = r.MultipartForm.RemoveAll()
		}
	}()

	categoryID, _ := strconv.ParseInt(trim(r.FormValue("category_id")), 10, 64)
	if categoryID <= 0 {
		categoryID = existing.CategoryID
	}
	if ok, _ := s.store.MaterialCategoryExists(r.Context(), categoryID); !ok {
		writeErr(w, http.StatusBadRequest, "资料分类不存在")
		return
	}

	updated := &store.Material{
		ID:          id,
		CategoryID:  categoryID,
		GroupName:   trim(r.FormValue("group_name")),
		Title:       trim(r.FormValue("title")),
		Description: trim(r.FormValue("description")),
		Tags:        normalizeTags(r.FormValue("tags")),
		FilePath:    existing.FilePath,
		FileName:    existing.FileName,
		FileSize:    existing.FileSize,
		MimeType:    existing.MimeType,
		FileExt:     existing.FileExt,
		Sort:        atoi(trim(r.FormValue("sort")), existing.Sort),
	}
	if updated.Title == "" {
		updated.Title = existing.Title
	}

	file, header, ferr := r.FormFile("file")
	if ferr == nil {
		defer file.Close()
		ext := strings.ToLower(filepath.Ext(header.Filename))
		if !allowedMaterialExts[ext] {
			writeErr(w, http.StatusBadRequest, "不支持的文件类型")
			return
		}
		relPath, size, err := s.files.Save(file, ext, maxBytes)
		if err != nil {
			slog.Error("保存资料文件失败", "error", err)
			writeErr(w, http.StatusBadRequest, "保存文件失败："+err.Error())
			return
		}
		if ext == ".pdf" {
			if ok, err := s.files.LooksLikePDF(relPath); err != nil || !ok {
				_ = s.files.Remove(relPath)
				writeErr(w, http.StatusBadRequest, "文件内容不是有效的 PDF")
				return
			}
		}
		updated.FilePath = relPath
		updated.FileName = sanitizeFileName(header.Filename)
		updated.FileSize = size
		updated.MimeType = materialMime(ext)
		updated.FileExt = strings.TrimPrefix(ext, ".")
	}

	if err := s.store.UpdateMaterial(r.Context(), updated); err != nil {
		if ferr == nil {
			_ = s.files.Remove(updated.FilePath)
		}
		slog.Error("更新资料失败", "error", err)
		writeErr(w, http.StatusInternalServerError, "更新失败")
		return
	}
	if ferr == nil && existing.FilePath != "" && existing.FilePath != updated.FilePath {
		if err := s.files.Remove(existing.FilePath); err != nil {
			slog.Warn("删除被替换的旧文件失败", "path", existing.FilePath, "error", err)
		}
	}
	if saved, err := s.store.GetMaterial(r.Context(), id); err == nil {
		updated = saved
	}
	updated.FilePath = ""
	writeJSON(w, http.StatusOK, map[string]any{"material": updated})
}

func (s *API) handleDeleteMaterial(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "参数错误")
		return
	}
	material, err := s.store.GetMaterial(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeErr(w, http.StatusNotFound, "资料不存在")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "查询失败")
		return
	}
	if err := s.store.DeleteMaterial(r.Context(), id); err != nil {
		slog.Error("删除资料失败", "error", err)
		writeErr(w, http.StatusInternalServerError, "删除失败")
		return
	}
	if err := s.files.Remove(material.FilePath); err != nil {
		slog.Warn("删除资料文件失败", "path", material.FilePath, "error", err)
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func materialMime(ext string) string {
	switch ext {
	case ".pdf":
		return "application/pdf"
	case ".md", ".markdown":
		return "text/markdown; charset=utf-8"
	case ".txt":
		return "text/plain; charset=utf-8"
	case ".zip":
		return "application/zip"
	case ".doc":
		return "application/msword"
	case ".docx":
		return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	case ".ppt":
		return "application/vnd.ms-powerpoint"
	case ".pptx":
		return "application/vnd.openxmlformats-officedocument.presentationml.presentation"
	case ".xls":
		return "application/vnd.ms-excel"
	case ".xlsx":
		return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	default:
		return "application/octet-stream"
	}
}
