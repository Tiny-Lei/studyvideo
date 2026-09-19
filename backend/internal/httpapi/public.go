package httpapi

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"studyvideo/internal/store"
)

func (s *API) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *API) handleHome(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	topics, err := s.store.ListTopics(ctx)
	if err != nil {
		slog.Error("查询主题失败", "error", err)
		writeErr(w, http.StatusInternalServerError, "查询失败")
		return
	}
	recent, err := s.store.SearchPublicVideos(ctx, "", 12)
	if err != nil {
		slog.Error("查询最新视频失败", "error", err)
		writeErr(w, http.StatusInternalServerError, "查询失败")
		return
	}
	var total int
	for _, t := range topics {
		total += t.VideoCount
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"topics": topics,
		"recent": recent,
		"stats":  map[string]int{"topics": len(topics), "videos": total},
	})
}

func (s *API) handleTopics(w http.ResponseWriter, r *http.Request) {
	topics, err := s.store.ListTopics(r.Context())
	if err != nil {
		slog.Error("查询主题失败", "error", err)
		writeErr(w, http.StatusInternalServerError, "查询失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"topics": topics})
}

func (s *API) handleTopic(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "参数错误")
		return
	}
	ctx := r.Context()
	topic, err := s.store.GetTopic(ctx, id)
	if errors.Is(err, store.ErrNotFound) {
		writeErr(w, http.StatusNotFound, "主题不存在")
		return
	}
	if err != nil {
		slog.Error("查询主题失败", "error", err)
		writeErr(w, http.StatusInternalServerError, "查询失败")
		return
	}
	categories, err := s.store.ListCategories(ctx, id)
	if err != nil {
		slog.Error("查询分类失败", "error", err)
		writeErr(w, http.StatusInternalServerError, "查询失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"topic": topic, "categories": categories})
}

func (s *API) handleCategory(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "参数错误")
		return
	}
	ctx := r.Context()
	category, err := s.store.GetCategory(ctx, id)
	if errors.Is(err, store.ErrNotFound) {
		writeErr(w, http.StatusNotFound, "分类不存在")
		return
	}
	if err != nil {
		slog.Error("查询分类失败", "error", err)
		writeErr(w, http.StatusInternalServerError, "查询失败")
		return
	}
	videos, err := s.store.PublicVideosByCategory(ctx, id, trim(r.URL.Query().Get("order")))
	if err != nil {
		slog.Error("查询视频失败", "error", err)
		writeErr(w, http.StatusInternalServerError, "查询失败")
		return
	}
	topic, err := s.store.GetTopic(ctx, category.TopicID)
	if err != nil {
		slog.Error("查询主题失败", "error", err)
		writeErr(w, http.StatusInternalServerError, "查询失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"category": category, "topic": topic, "videos": videos})
}

func (s *API) handleVideo(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "参数错误")
		return
	}
	ctx := r.Context()
	video, err := s.store.GetVideoPublic(ctx, id)
	if errors.Is(err, store.ErrNotFound) {
		writeErr(w, http.StatusNotFound, "视频不存在")
		return
	}
	if err != nil {
		slog.Error("查询视频失败", "error", err)
		writeErr(w, http.StatusInternalServerError, "查询失败")
		return
	}
	pdfs, err := s.store.ListPDFsPublic(ctx, id)
	if err != nil {
		slog.Error("查询配套资料失败", "error", err)
		writeErr(w, http.StatusInternalServerError, "查询失败")
		return
	}
	topic, err := s.store.GetTopic(ctx, video.TopicID)
	if err != nil {
		slog.Error("查询主题失败", "error", err)
		writeErr(w, http.StatusInternalServerError, "查询失败")
		return
	}
	// 相关视频：优先同分类，不足时补充同主题其它分类的视频
	related, err := s.store.PublicVideosByCategory(ctx, video.CategoryID, "")
	if err != nil {
		slog.Error("查询相关视频失败", "error", err)
		writeErr(w, http.StatusInternalServerError, "查询失败")
		return
	}
	if len(related) < 20 {
		siblings, err := s.store.PublicVideosByTopic(ctx, video.TopicID, "")
		if err != nil {
			slog.Error("查询同主题视频失败", "error", err)
			writeErr(w, http.StatusInternalServerError, "查询失败")
			return
		}
		seen := make(map[int64]struct{}, len(related))
		for _, v := range related {
			seen[v.ID] = struct{}{}
		}
		for _, v := range siblings {
			if len(related) >= 20 {
				break
			}
			if _, ok := seen[v.ID]; ok {
				continue
			}
			seen[v.ID] = struct{}{}
			related = append(related, v)
		}
	}
	filtered := related[:0]
	for _, v := range related {
		if v.ID != video.ID {
			filtered = append(filtered, v)
		}
	}
	related = filtered
	if len(related) > 20 {
		related = related[:20]
	}
	// 记录一次访问（同日同 IP 去重），失败不影响正常返回
	if ip := s.risk.ClientIP(r); ip != "" {
		if err := s.store.RecordVisit(ctx, video.ID, ip); err != nil {
			slog.Error("记录视频访问失败", "video_id", video.ID, "error", err)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"video": video, "topic": topic, "related": related, "pdfs": pdfs})
}

// handleReportWatch 由前端在视频真正开始播放时上报，用于统计「观看次数」。
// 同日同 IP 只计一次独立观看，PV 每次累加；不计入播放限流额度。
func (s *API) handleReportWatch(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "参数错误")
		return
	}
	if ok, _ := s.store.VideoExists(r.Context(), id); !ok {
		writeErr(w, http.StatusNotFound, "视频不存在")
		return
	}
	ip := s.risk.ClientIP(r)
	if ip == "" {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
		return
	}
	if err := s.store.RecordWatch(r.Context(), id, ip); err != nil {
		slog.Error("记录视频观看失败", "video_id", id, "error", err)
		writeErr(w, http.StatusInternalServerError, "记录失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// handleOpenPDF 兼容旧链接：302 跳转到在线预览地址。
func (s *API) handleOpenPDF(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "参数错误")
		return
	}
	if _, err := s.store.GetPDF(r.Context(), id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "资料不存在")
			return
		}
		writeErr(w, http.StatusInternalServerError, "查询失败")
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/api/pdfs/%d/file", id), http.StatusFound)
}

func (s *API) handlePlay(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "参数错误")
		return
	}
	video, err := s.store.GetVideo(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeErr(w, http.StatusNotFound, "视频不存在")
		return
	}
	if err != nil {
		slog.Error("查询视频失败", "error", err)
		writeErr(w, http.StatusInternalServerError, "查询失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id":    video.ID,
		"title": video.Title,
		"url":   video.URL,
	})
}

func (s *API) handleSearch(w http.ResponseWriter, r *http.Request) {
	q := trim(r.URL.Query().Get("q"))
	if q == "" {
		writeJSON(w, http.StatusOK, map[string]any{"q": "", "videos": []store.Video{}})
		return
	}
	videos, err := s.store.SearchPublicVideos(r.Context(), q, 100)
	if err != nil {
		slog.Error("搜索失败", "error", err)
		writeErr(w, http.StatusInternalServerError, "搜索失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"q": q, "videos": videos})
}
