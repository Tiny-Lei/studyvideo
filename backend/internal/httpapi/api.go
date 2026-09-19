package httpapi

import (
	"net/http"
	"strconv"

	"studyvideo/internal/auth"
	"studyvideo/internal/config"
	"studyvideo/internal/filestore"
	"studyvideo/internal/health"
	"studyvideo/internal/risk"
	"studyvideo/internal/store"
)

type API struct {
	store  *store.Store
	cfg    *config.Config
	auth   *auth.Auth
	risk   *risk.Manager
	health *health.Checker
	files  *filestore.Store
}

func New(st *store.Store, cfg *config.Config, a *auth.Auth, rm *risk.Manager, hc *health.Checker, fs *filestore.Store) *API {
	return &API{store: st, cfg: cfg, auth: a, risk: rm, health: hc, files: fs}
}

func (s *API) Register(mux *http.ServeMux) {
	// 公开接口
	mux.HandleFunc("GET /api/health", s.handleHealth)
	mux.HandleFunc("GET /api/home", s.handleHome)
	mux.HandleFunc("GET /api/topics", s.handleTopics)
	mux.HandleFunc("GET /api/topics/{id}", s.handleTopic)
	mux.HandleFunc("GET /api/categories/{id}", s.handleCategory)
	mux.HandleFunc("GET /api/videos/{id}", s.handleVideo)
	mux.HandleFunc("GET /api/videos/{id}/play", s.handlePlay)
	mux.HandleFunc("GET /api/pdfs/{id}/file", s.handlePDFFile(false))
	mux.HandleFunc("GET /api/pdfs/{id}/download", s.handlePDFFile(true))
	mux.HandleFunc("GET /api/pdfs/{id}/open", s.handleOpenPDF)
	mux.HandleFunc("GET /api/search", s.handleSearch)

	// 管理端接口
	mux.HandleFunc("POST /api/admin/login", s.handleLogin)
	mux.HandleFunc("POST /api/admin/logout", s.handleLogout)
	mux.HandleFunc("GET /api/admin/session", s.handleSession)

	mux.HandleFunc("GET /api/admin/overview", s.requireAdmin(s.handleOverview))
	mux.HandleFunc("GET /api/admin/topics", s.requireAdmin(s.handleAdminTopics))
	mux.HandleFunc("POST /api/admin/topics", s.requireAdmin(s.handleCreateTopic))
	mux.HandleFunc("PUT /api/admin/topics/{id}", s.requireAdmin(s.handleUpdateTopic))
	mux.HandleFunc("DELETE /api/admin/topics/{id}", s.requireAdmin(s.handleDeleteTopic))

	mux.HandleFunc("GET /api/admin/categories", s.requireAdmin(s.handleAdminCategories))
	mux.HandleFunc("POST /api/admin/categories", s.requireAdmin(s.handleCreateCategory))
	mux.HandleFunc("PUT /api/admin/categories/{id}", s.requireAdmin(s.handleUpdateCategory))
	mux.HandleFunc("DELETE /api/admin/categories/{id}", s.requireAdmin(s.handleDeleteCategory))

	mux.HandleFunc("GET /api/admin/videos", s.requireAdmin(s.handleAdminVideos))
	mux.HandleFunc("POST /api/admin/videos", s.requireAdmin(s.handleCreateVideo))
	mux.HandleFunc("PUT /api/admin/videos/{id}", s.requireAdmin(s.handleUpdateVideo))
	mux.HandleFunc("DELETE /api/admin/videos/{id}", s.requireAdmin(s.handleDeleteVideo))
	mux.HandleFunc("POST /api/admin/videos/batch", s.requireAdmin(s.handleBatchCreate))
	mux.HandleFunc("POST /api/admin/videos/{id}/check", s.requireAdmin(s.handleCheckVideo))

	mux.HandleFunc("GET /api/admin/videos/{id}/pdfs", s.requireAdmin(s.handleAdminPDFs))
	mux.HandleFunc("POST /api/admin/videos/{id}/pdfs", s.requireAdmin(s.handleUploadPDF))
	mux.HandleFunc("PUT /api/admin/pdfs/{id}", s.requireAdmin(s.handleUpdatePDF))
	mux.HandleFunc("DELETE /api/admin/pdfs/{id}", s.requireAdmin(s.handleDeletePDF))
	mux.HandleFunc("GET /api/admin/pdfs/{id}/file", s.requireAdmin(s.handlePDFFile(false)))

	mux.HandleFunc("GET /api/admin/check/status", s.requireAdmin(s.handleCheckStatus))
	mux.HandleFunc("POST /api/admin/check", s.requireAdmin(s.handleCheckAll))

	mux.HandleFunc("GET /api/admin/blocked", s.requireAdmin(s.handleBlockedList))
	mux.HandleFunc("DELETE /api/admin/blocked/{ip}", s.requireAdmin(s.handleUnblock))
	mux.HandleFunc("GET /api/admin/usage", s.requireAdmin(s.handleUsage))
	mux.HandleFunc("GET /api/admin/config", s.requireAdmin(s.handleConfig))

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		writeErr(w, http.StatusNotFound, "接口不存在")
	})
}

func (s *API) requireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.auth.IsAdmin(r) {
			writeErr(w, http.StatusUnauthorized, "未登录或登录已过期")
			return
		}
		next(w, r)
	}
}

func pathID(r *http.Request) (int64, error) {
	return strconv.ParseInt(r.PathValue("id"), 10, 64)
}

func atoi(s string, def int) int {
	if n, err := strconv.Atoi(s); err == nil {
		return n
	}
	return def
}
