package httpapi

import (
	"errors"
	"fmt"
	"log/slog"
	"mime"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"

	"studyvideo/internal/filestore"
	"studyvideo/internal/store"
)

// ---------------- 配套资料（本站存储的 PDF 等文件） ----------------

const maxUploadMemory = 16 << 20 // 16MB 内存缓冲，超过部分落临时文件

func (s *API) handleAdminPDFs(w http.ResponseWriter, r *http.Request) {
	videoID, err := pathID(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "参数错误")
		return
	}
	if ok, _ := s.store.VideoExists(r.Context(), videoID); !ok {
		writeErr(w, http.StatusNotFound, "视频不存在")
		return
	}
	pdfs, err := s.store.ListPDFs(r.Context(), videoID)
	if err != nil {
		slog.Error("查询资料失败", "error", err)
		writeErr(w, http.StatusInternalServerError, "查询失败")
		return
	}
	for i := range pdfs {
		pdfs[i].FilePath = ""
	}
	writeJSON(w, http.StatusOK, map[string]any{"pdfs": pdfs})
}

// handleUploadPDF 接收 multipart 上传，文件落盘后写入数据库。
func (s *API) handleUploadPDF(w http.ResponseWriter, r *http.Request) {
	videoID, err := pathID(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "参数错误")
		return
	}
	if ok, _ := s.store.VideoExists(r.Context(), videoID); !ok {
		writeErr(w, http.StatusNotFound, "视频不存在")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, s.cfg.MaxPDFSizeBytes+maxUploadMemory)
	if err := r.ParseMultipartForm(maxUploadMemory); err != nil {
		writeErr(w, http.StatusBadRequest, fmt.Sprintf("解析上传内容失败（单文件不能超过 %d MB）", s.cfg.MaxPDFSizeBytes>>20))
		return
	}
	defer func() {
		if r.MultipartForm != nil {
			_ = r.MultipartForm.RemoveAll()
		}
	}()

	file, header, err := r.FormFile("file")
	if err != nil {
		writeErr(w, http.StatusBadRequest, "请选择要上传的文件")
		return
	}
	defer file.Close()

	title := trim(r.FormValue("title"))
	if title == "" {
		title = strings.TrimSuffix(header.Filename, filepath.Ext(header.Filename))
	}
	if title == "" {
		title = "未命名资料"
	}
	sort, _ := strconv.Atoi(trim(r.FormValue("sort")))

	ext := filepath.Ext(header.Filename)
	if !strings.EqualFold(ext, ".pdf") {
		writeErr(w, http.StatusBadRequest, "只支持上传 PDF 文件")
		return
	}
	relPath, size, err := s.files.Save(file, ext, s.cfg.MaxPDFSizeBytes)
	if err != nil {
		slog.Error("保存上传文件失败", "error", err)
		writeErr(w, http.StatusBadRequest, "保存文件失败："+err.Error())
		return
	}
	if ok, err := s.files.LooksLikePDF(relPath); err != nil || !ok {
		_ = s.files.Remove(relPath)
		writeErr(w, http.StatusBadRequest, "文件内容不是有效的 PDF")
		return
	}

	p := &store.PDF{
		VideoID:  videoID,
		Title:    title,
		FilePath: relPath,
		FileName: sanitizeFileName(header.Filename),
		FileSize: size,
		MimeType: "application/pdf",
		Sort:     sort,
	}
	if err := s.store.CreatePDF(r.Context(), p); err != nil {
		_ = s.files.Remove(relPath)
		slog.Error("写入资料记录失败", "error", err)
		writeErr(w, http.StatusInternalServerError, "保存记录失败")
		return
	}
	if saved, err := s.store.GetPDF(r.Context(), p.ID); err == nil {
		p = saved
	}
	p.FilePath = ""
	writeJSON(w, http.StatusOK, map[string]any{"pdf": p})
}

// handleUpdatePDF 更新资料名称与排序；若携带文件则替换原文件。
func (s *API) handleUpdatePDF(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "参数错误")
		return
	}
	existing, err := s.store.GetPDF(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeErr(w, http.StatusNotFound, "资料不存在")
		return
	}
	if err != nil {
		slog.Error("查询资料失败", "error", err)
		writeErr(w, http.StatusInternalServerError, "查询失败")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, s.cfg.MaxPDFSizeBytes+maxUploadMemory)
	if err := r.ParseMultipartForm(maxUploadMemory); err != nil {
		writeErr(w, http.StatusBadRequest, "解析上传内容失败")
		return
	}
	defer func() {
		if r.MultipartForm != nil {
			_ = r.MultipartForm.RemoveAll()
		}
	}()

	title := trim(r.FormValue("title"))
	if title == "" {
		title = existing.Title
	}
	sort, _ := strconv.Atoi(trim(r.FormValue("sort")))

	updated := &store.PDF{
		ID:       id,
		Title:    title,
		FilePath: existing.FilePath,
		FileName: existing.FileName,
		FileSize: existing.FileSize,
		MimeType: existing.MimeType,
		Sort:     sort,
	}

	file, header, ferr := r.FormFile("file")
	if ferr == nil {
		defer file.Close()
		ext := filepath.Ext(header.Filename)
		if !strings.EqualFold(ext, ".pdf") {
			writeErr(w, http.StatusBadRequest, "只支持上传 PDF 文件")
			return
		}
		relPath, size, err := s.files.Save(file, ext, s.cfg.MaxPDFSizeBytes)
		if err != nil {
			slog.Error("保存上传文件失败", "error", err)
			writeErr(w, http.StatusBadRequest, "保存文件失败："+err.Error())
			return
		}
		if ok, err := s.files.LooksLikePDF(relPath); err != nil || !ok {
			_ = s.files.Remove(relPath)
			writeErr(w, http.StatusBadRequest, "文件内容不是有效的 PDF")
			return
		}
		updated.FilePath = relPath
		updated.FileName = sanitizeFileName(header.Filename)
		updated.FileSize = size
		updated.MimeType = "application/pdf"
	}

	if err := s.store.UpdatePDF(r.Context(), updated); err != nil {
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
	if saved, err := s.store.GetPDF(r.Context(), id); err == nil {
		updated = saved
	}
	updated.FilePath = ""
	writeJSON(w, http.StatusOK, map[string]any{"pdf": updated})
}

func (s *API) handleDeletePDF(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "参数错误")
		return
	}
	pdf, err := s.store.GetPDF(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeErr(w, http.StatusNotFound, "资料不存在")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "查询失败")
		return
	}
	if err := s.store.DeletePDF(r.Context(), id); err != nil {
		slog.Error("删除资料失败", "error", err)
		writeErr(w, http.StatusInternalServerError, "删除失败")
		return
	}
	if err := s.files.Remove(pdf.FilePath); err != nil {
		slog.Warn("删除资料文件失败", "path", pdf.FilePath, "error", err)
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// handlePDFFile 提供在线预览与下载；支持 Range，方便浏览器内置 PDF 阅读器翻页。
//   - /api/pdfs/{id}/file  → 在线预览（inline）
//   - /api/pdfs/{id}/download → 下载（attachment）
func (s *API) handlePDFFile(download bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := pathID(r)
		if err != nil {
			writeErr(w, http.StatusBadRequest, "参数错误")
			return
		}
		pdf, err := s.store.GetPDF(r.Context(), id)
		if errors.Is(err, store.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "资料不存在")
			return
		}
		if err != nil {
			slog.Error("查询资料失败", "error", err)
			writeErr(w, http.StatusInternalServerError, "查询失败")
			return
		}
		if pdf.FilePath == "" {
			writeErr(w, http.StatusNotFound, "该资料文件缺失，请在后台重新上传")
			return
		}
		f, info, err := s.files.Open(pdf.FilePath)
		if errors.Is(err, filestore.ErrNotFound) {
			slog.Error("资料文件丢失", "pdf_id", pdf.ID, "path", pdf.FilePath)
			writeErr(w, http.StatusNotFound, "资料文件不存在")
			return
		}
		if err != nil {
			slog.Error("打开资料文件失败", "error", err)
			writeErr(w, http.StatusInternalServerError, "读取失败")
			return
		}
		defer f.Close()

		ctype := pdf.MimeType
		if ctype == "" {
			ctype = mime.TypeByExtension(filepath.Ext(pdf.FileName))
		}
		if ctype == "" {
			ctype = "application/octet-stream"
		}
		disposition := "inline"
		if download {
			disposition = "attachment"
		}
		w.Header().Set("Content-Type", ctype)
		w.Header().Set("Content-Disposition", fmt.Sprintf(`%s; filename="%s"; filename*=UTF-8''%s`,
			disposition, asciiFallback(pdf.FileName), url.PathEscape(pdf.FileName)))
		w.Header().Set("Cache-Control", "private, max-age=3600")
		// http.ServeContent 自动处理 Range / If-Modified-Since，满足阅读器翻页与断点续传
		http.ServeContent(w, r, pdf.FileName, info.ModTime(), f)
	}
}

func sanitizeFileName(name string) string {
	name = strings.TrimSpace(filepath.Base(strings.ReplaceAll(name, "\\", "/")))
	if name == "" || name == "." || name == ".." {
		return "资料.pdf"
	}
	if len([]rune(name)) > 120 {
		ext := filepath.Ext(name)
		name = string([]rune(strings.TrimSuffix(name, ext))[:100]) + ext
	}
	return name
}

// asciiFallback 生成仅含 ASCII 的回退文件名；纯中文名会退化为 "document.pdf" 这类可读形式。
func asciiFallback(name string) string {
	ext := filepath.Ext(name)
	var b strings.Builder
	for _, r := range strings.TrimSuffix(name, ext) {
		if r < 128 && r != '"' && r != '\\' && r >= 32 && r != '/' {
			b.WriteRune(r)
		}
	}
	base := strings.TrimSpace(b.String())
	if base == "" {
		base = "document"
	}
	if ext == "" {
		ext = ".pdf"
	}
	return base + ext
}
