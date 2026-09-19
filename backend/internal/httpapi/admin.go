package httpapi

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"studyvideo/internal/store"
)

// ---------------- 登录 ----------------

func (s *API) handleLogin(w http.ResponseWriter, r *http.Request) {
	ip := s.risk.ClientIP(r)
	if ok, retry := s.risk.LoginAllowed(ip); !ok {
		w.Header().Set("Retry-After", strconv.Itoa(retry))
		writeErr(w, http.StatusTooManyRequests, "尝试次数过多，请稍后再试")
		return
	}
	var req struct {
		Password string `json:"password"`
	}
	if err := readJSON(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if !s.auth.CheckPassword(req.Password) {
		s.risk.LoginFailed(ip)
		writeErr(w, http.StatusUnauthorized, "密码错误")
		return
	}
	s.risk.LoginSucceeded(ip)
	s.auth.SetCookie(w)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *API) handleLogout(w http.ResponseWriter, r *http.Request) {
	s.auth.ClearCookie(w)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *API) handleSession(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"admin": s.auth.IsAdmin(r)})
}

// ---------------- 概览 ----------------

func (s *API) handleOverview(w http.ResponseWriter, r *http.Request) {
	stats, err := s.store.Stats(r.Context())
	if err != nil {
		slog.Error("查询统计失败", "error", err)
		writeErr(w, http.StatusInternalServerError, "查询失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"stats": stats})
}

func (s *API) handleConfig(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"daily_video_limit":      s.cfg.DailyVideoLimit,
		"daily_total_limit":      s.cfg.DailyTotalLimit,
		"burst_video_limit":      s.cfg.BurstVideoLimit,
		"burst_total_limit":      s.cfg.BurstTotalLimit,
		"burst_window_seconds":   int(s.cfg.BurstWindow.Seconds()),
		"block_minutes":          s.cfg.BlockMinutes,
		"check_interval_minutes": int(s.cfg.CheckInterval.Minutes()),
		"trust_proxy":            s.cfg.TrustProxy,
	})
}

// ---------------- 主题 ----------------

type topicInput struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Sort        int    `json:"sort"`
}

func (s *API) handleAdminTopics(w http.ResponseWriter, r *http.Request) {
	topics, err := s.store.ListTopics(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "查询失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"topics": topics})
}

func (s *API) handleCreateTopic(w http.ResponseWriter, r *http.Request) {
	var req topicInput
	if err := readJSON(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	req.Name = trim(req.Name)
	if req.Name == "" {
		writeErr(w, http.StatusBadRequest, "主题名称不能为空")
		return
	}
	t := &store.Topic{Name: req.Name, Description: trim(req.Description), Sort: req.Sort}
	if err := s.store.CreateTopic(r.Context(), t); err != nil {
		slog.Error("创建主题失败", "error", err)
		writeErr(w, http.StatusInternalServerError, "创建失败")
		return
	}
	if created, err := s.store.GetTopic(r.Context(), t.ID); err == nil {
		t = created
	}
	writeJSON(w, http.StatusOK, map[string]any{"topic": t})
}

func (s *API) handleUpdateTopic(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "参数错误")
		return
	}
	var req topicInput
	if err := readJSON(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	req.Name = trim(req.Name)
	if req.Name == "" {
		writeErr(w, http.StatusBadRequest, "主题名称不能为空")
		return
	}
	t := &store.Topic{ID: id, Name: req.Name, Description: trim(req.Description), Sort: req.Sort}
	err = s.store.UpdateTopic(r.Context(), t)
	if errors.Is(err, store.ErrNotFound) {
		writeErr(w, http.StatusNotFound, "主题不存在")
		return
	}
	if err != nil {
		slog.Error("更新主题失败", "error", err)
		writeErr(w, http.StatusInternalServerError, "更新失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *API) handleDeleteTopic(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "参数错误")
		return
	}
	paths, err := s.store.PDFFilePathsByTopic(r.Context(), id)
	if err != nil {
		slog.Error("查询主题资料失败", "error", err)
		writeErr(w, http.StatusInternalServerError, "删除失败")
		return
	}
	err = s.store.DeleteTopic(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeErr(w, http.StatusNotFound, "主题不存在")
		return
	}
	if err != nil {
		slog.Error("删除主题失败", "error", err)
		writeErr(w, http.StatusInternalServerError, "删除失败")
		return
	}
	for _, p := range paths {
		if err := s.files.Remove(p); err != nil {
			slog.Warn("删除资料文件失败", "path", p, "error", err)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// ---------------- 分类 ----------------

type categoryInput struct {
	TopicID     int64  `json:"topic_id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Sort        int    `json:"sort"`
}

func (s *API) handleAdminCategories(w http.ResponseWriter, r *http.Request) {
	topicID, _ := strconv.ParseInt(trim(r.URL.Query().Get("topic_id")), 10, 64)
	categories, err := s.store.ListCategories(r.Context(), topicID)
	if err != nil {
		slog.Error("查询分类失败", "error", err)
		writeErr(w, http.StatusInternalServerError, "查询失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"categories": categories})
}

func (s *API) handleCreateCategory(w http.ResponseWriter, r *http.Request) {
	var req categoryInput
	if err := readJSON(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	req.Name = trim(req.Name)
	if req.TopicID <= 0 {
		writeErr(w, http.StatusBadRequest, "请选择所属主题")
		return
	}
	if req.Name == "" {
		writeErr(w, http.StatusBadRequest, "分类名称不能为空")
		return
	}
	if ok, _ := s.store.TopicExists(r.Context(), req.TopicID); !ok {
		writeErr(w, http.StatusBadRequest, "所属主题不存在")
		return
	}
	c := &store.Category{TopicID: req.TopicID, Name: req.Name, Description: trim(req.Description), Sort: req.Sort}
	if err := s.store.CreateCategory(r.Context(), c); err != nil {
		slog.Error("创建分类失败", "error", err)
		writeErr(w, http.StatusInternalServerError, "创建失败")
		return
	}
	if created, err := s.store.GetCategory(r.Context(), c.ID); err == nil {
		c = created
	}
	writeJSON(w, http.StatusOK, map[string]any{"category": c})
}

func (s *API) handleUpdateCategory(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "参数错误")
		return
	}
	var req categoryInput
	if err := readJSON(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	req.Name = trim(req.Name)
	if req.TopicID <= 0 {
		writeErr(w, http.StatusBadRequest, "请选择所属主题")
		return
	}
	if req.Name == "" {
		writeErr(w, http.StatusBadRequest, "分类名称不能为空")
		return
	}
	c := &store.Category{ID: id, TopicID: req.TopicID, Name: req.Name, Description: trim(req.Description), Sort: req.Sort}
	err = s.store.UpdateCategory(r.Context(), c)
	if errors.Is(err, store.ErrNotFound) {
		writeErr(w, http.StatusNotFound, "分类不存在")
		return
	}
	if err != nil {
		slog.Error("更新分类失败", "error", err)
		writeErr(w, http.StatusInternalServerError, "更新失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// handleDeleteCategory 删除分类；mode=keep（默认）时分类下视频移动到「未分类」，mode=delete 时连同视频一起删除。
func (s *API) handleDeleteCategory(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "参数错误")
		return
	}
	keep := trim(r.URL.Query().Get("mode")) != "delete"

	var removedPDFs []string
	if !keep {
		videos, err := s.store.VideosByCategory(r.Context(), id)
		if err != nil {
			slog.Error("查询分类视频失败", "error", err)
			writeErr(w, http.StatusInternalServerError, "删除失败")
			return
		}
		for _, v := range videos {
			pdfs, err := s.store.PDFsByVideo(r.Context(), v.ID)
			if err != nil {
				slog.Error("查询资料失败", "error", err)
				writeErr(w, http.StatusInternalServerError, "删除失败")
				return
			}
			for _, p := range pdfs {
				if p.FilePath != "" {
					removedPDFs = append(removedPDFs, p.FilePath)
				}
			}
		}
	}

	removedVideoIDs, err := s.store.DeleteCategory(r.Context(), id, keep)
	if errors.Is(err, store.ErrNotFound) {
		writeErr(w, http.StatusNotFound, "分类不存在")
		return
	}
	if err != nil {
		slog.Error("删除分类失败", "error", err)
		writeErr(w, http.StatusInternalServerError, "删除失败")
		return
	}
	for _, p := range removedPDFs {
		if err := s.files.Remove(p); err != nil {
			slog.Warn("删除资料文件失败", "path", p, "error", err)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "removed_videos": len(removedVideoIDs), "kept_videos": keep})
}

// ---------------- 视频 ----------------

type videoInput struct {
	TopicID     int64  `json:"topic_id"`
	CategoryID  int64  `json:"category_id"`
	Title       string `json:"title"`
	URL         string `json:"url"`
	Description string `json:"description"`
	Tags        string `json:"tags"`
	Sort        int    `json:"sort"`
}

func (in *videoInput) normalize() error {
	in.Title = trim(in.Title)
	in.URL = trim(in.URL)
	in.Description = trim(in.Description)
	in.Tags = normalizeTags(in.Tags)
	if in.TopicID <= 0 {
		return errors.New("请选择所属主题")
	}
	if in.CategoryID <= 0 {
		return errors.New("请选择所属分类")
	}
	if in.Title == "" {
		return errors.New("标题不能为空")
	}
	return ValidateVideoURL(in.URL)
}

func normalizeTags(tags string) string {
	parts := strings.FieldsFunc(tags, func(r rune) bool { return r == ',' || r == '，' || r == ';' || r == '；' })
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = trim(p); p != "" {
			out = append(out, p)
		}
	}
	return strings.Join(out, ",")
}

func (s *API) handleAdminVideos(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page, _ := strconv.Atoi(q.Get("page"))
	topicID, _ := strconv.ParseInt(q.Get("topic_id"), 10, 64)
	categoryID, _ := strconv.ParseInt(q.Get("category_id"), 10, 64)
	f := store.VideoFilter{
		TopicID:    topicID,
		CategoryID: categoryID,
		Keyword:    trim(q.Get("q")),
		Status:     trim(q.Get("status")),
		Page:       page,
		PageSize:   atoi(q.Get("page_size"), 20),
	}
	videos, total, err := s.store.AdminListVideos(r.Context(), f)
	if err != nil {
		slog.Error("查询视频失败", "error", err)
		writeErr(w, http.StatusInternalServerError, "查询失败")
		return
	}
	// 附上访问 / 观看统计（仅本页，避免大范围扫描）
	if len(videos) > 0 {
		ids := make([]int64, 0, len(videos))
		for _, v := range videos {
			ids = append(ids, v.ID)
		}
		stats, err := s.store.VideoStatsFor(r.Context(), ids)
		if err != nil {
			slog.Error("查询视频统计失败", "error", err)
		} else {
			for i := range videos {
				if st, ok := stats[videos[i].ID]; ok {
					videos[i].Stats = st
				}
			}
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"videos": videos, "total": total, "page": f.Page, "page_size": f.PageSize})
}

func (s *API) handleCreateVideo(w http.ResponseWriter, r *http.Request) {
	var req videoInput
	if err := readJSON(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := req.normalize(); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if ok, _ := s.store.TopicExists(r.Context(), req.TopicID); !ok {
		writeErr(w, http.StatusBadRequest, "所属主题不存在")
		return
	}
	if ok, _ := s.store.CategoryExists(r.Context(), req.CategoryID); !ok {
		writeErr(w, http.StatusBadRequest, "所属分类不存在")
		return
	}
	v := &store.Video{TopicID: req.TopicID, CategoryID: req.CategoryID, Title: req.Title, URL: req.URL, Description: req.Description, Tags: req.Tags, Sort: req.Sort}
	if err := s.store.CreateVideo(r.Context(), v); err != nil {
		if errors.Is(err, store.ErrDuplicate) {
			writeErr(w, http.StatusConflict, "该视频链接已存在")
			return
		}
		slog.Error("创建视频失败", "error", err)
		writeErr(w, http.StatusInternalServerError, "创建失败")
		return
	}
	s.health.CheckOneAsync(*v)
	if created, err := s.store.GetVideo(r.Context(), v.ID); err == nil {
		v = created
	}
	writeJSON(w, http.StatusOK, map[string]any{"video": v})
}

func (s *API) handleUpdateVideo(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "参数错误")
		return
	}
	var req videoInput
	if err := readJSON(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := req.normalize(); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if ok, _ := s.store.TopicExists(r.Context(), req.TopicID); !ok {
		writeErr(w, http.StatusBadRequest, "所属主题不存在")
		return
	}
	if ok, _ := s.store.CategoryExists(r.Context(), req.CategoryID); !ok {
		writeErr(w, http.StatusBadRequest, "所属分类不存在")
		return
	}
	v := &store.Video{ID: id, TopicID: req.TopicID, CategoryID: req.CategoryID, Title: req.Title, URL: req.URL, Description: req.Description, Tags: req.Tags, Sort: req.Sort}
	err = s.store.UpdateVideo(r.Context(), v)
	if errors.Is(err, store.ErrNotFound) {
		writeErr(w, http.StatusNotFound, "视频不存在")
		return
	}
	if errors.Is(err, store.ErrDuplicate) {
		writeErr(w, http.StatusConflict, "该视频链接已存在")
		return
	}
	if err != nil {
		slog.Error("更新视频失败", "error", err)
		writeErr(w, http.StatusInternalServerError, "更新失败")
		return
	}
	updated, _ := s.store.GetVideo(r.Context(), id)
	writeJSON(w, http.StatusOK, map[string]any{"video": updated})
}

func (s *API) handleDeleteVideo(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "参数错误")
		return
	}
	// 先取出资料文件路径，删除记录后同步清理磁盘
	pdfs, err := s.store.PDFsByVideo(r.Context(), id)
	if err != nil {
		slog.Error("查询配套资料失败", "error", err)
		writeErr(w, http.StatusInternalServerError, "删除失败")
		return
	}
	err = s.store.DeleteVideo(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeErr(w, http.StatusNotFound, "视频不存在")
		return
	}
	if err != nil {
		slog.Error("删除视频失败", "error", err)
		writeErr(w, http.StatusInternalServerError, "删除失败")
		return
	}
	for _, p := range pdfs {
		if p.FilePath == "" {
			continue
		}
		if err := s.files.Remove(p.FilePath); err != nil {
			slog.Warn("删除资料文件失败", "path", p.FilePath, "error", err)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *API) handleBatchCreate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TopicID     int64  `json:"topic_id"`
		CategoryID  int64  `json:"category_id"`
		Text        string `json:"text"`
		DefaultTags string `json:"default_tags"`
	}
	if err := readJSON(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.TopicID <= 0 {
		writeErr(w, http.StatusBadRequest, "请选择所属主题")
		return
	}
	if req.CategoryID <= 0 {
		writeErr(w, http.StatusBadRequest, "请选择所属分类")
		return
	}
	if ok, _ := s.store.TopicExists(r.Context(), req.TopicID); !ok {
		writeErr(w, http.StatusBadRequest, "所属主题不存在")
		return
	}
	if ok, _ := s.store.CategoryExists(r.Context(), req.CategoryID); !ok {
		writeErr(w, http.StatusBadRequest, "所属分类不存在")
		return
	}
	if trim(req.Text) == "" {
		writeErr(w, http.StatusBadRequest, "请粘贴视频链接")
		return
	}
	defaultTags := normalizeTags(req.DefaultTags)
	items := ParseBatchLines(req.Text)

	created, skipped, failed := 0, 0, 0
	results := make([]BatchItem, 0, len(items))
	for _, it := range items {
		if it.Error == "" {
			if err := ValidateVideoURL(it.URL); err != nil {
				it.Error = err.Error()
			}
		}
		if it.Error != "" {
			failed++
			results = append(results, it)
			continue
		}
		if it.Tags == "" {
			it.Tags = defaultTags
		}
		v := &store.Video{TopicID: req.TopicID, CategoryID: req.CategoryID, Title: it.Title, URL: it.URL, Description: it.Description, Tags: normalizeTags(it.Tags)}
		err := s.store.CreateVideo(r.Context(), v)
		switch {
		case err == nil:
			created++
			it.Title = v.Title
		case errors.Is(err, store.ErrDuplicate):
			skipped++
			it.Error = "链接已存在，已跳过"
		default:
			failed++
			it.Error = "写入失败: " + err.Error()
		}
		results = append(results, it)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"created": created,
		"skipped": skipped,
		"failed":  failed,
		"results": results,
	})
	if created > 0 {
		s.health.RunAsync("新增后检查")
	}
}

// ---------------- 链接检查 ----------------

func (s *API) handleCheckVideo(w http.ResponseWriter, r *http.Request) {
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
		writeErr(w, http.StatusInternalServerError, "查询失败")
		return
	}
	res := s.health.CheckOne(r.Context(), *video)
	writeJSON(w, http.StatusOK, map[string]any{"result": res})
}

func (s *API) handleCheckAll(w http.ResponseWriter, r *http.Request) {
	started := s.health.RunAsync("手动检查")
	writeJSON(w, http.StatusOK, map[string]any{"started": started, "progress": s.health.Progress()})
}

func (s *API) handleCheckStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"progress": s.health.Progress()})
}

// ---------------- 风控 ----------------

func (s *API) handleBlockedList(w http.ResponseWriter, r *http.Request) {
	list, err := s.store.ListBlocked(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "查询失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"blocked": list})
}

func (s *API) handleUnblock(w http.ResponseWriter, r *http.Request) {
	ip := trim(r.PathValue("ip"))
	if ip == "" {
		writeErr(w, http.StatusBadRequest, "参数错误")
		return
	}
	if err := s.risk.Unblock(r.Context(), ip); err != nil {
		writeErr(w, http.StatusInternalServerError, "操作失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *API) handleUsage(w http.ResponseWriter, r *http.Request) {
	list, err := s.store.ListRecentUsage(r.Context(), atoi(r.URL.Query().Get("limit"), 50))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "查询失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"usage": list})
}
