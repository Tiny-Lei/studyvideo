package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
)

var (
	ErrNotFound  = errors.New("记录不存在")
	ErrDuplicate = errors.New("该视频链接已存在")
)

const defaultCategoryName = "未分类"

type Store struct {
	db *sql.DB
}

func New(db *sql.DB) *Store { return &Store{db: db} }

func hashURL(u string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(u)))
	return hex.EncodeToString(sum[:])
}

func isDuplicateErr(err error) bool {
	var me *mysql.MySQLError
	return errors.As(err, &me) && me.Number == 1062
}

func likePattern(q string) string {
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return "%" + r.Replace(strings.TrimSpace(q)) + "%"
}

// ---------------- topics ----------------

func (s *Store) ListTopics(ctx context.Context) ([]Topic, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT t.id, t.name, COALESCE(t.description,''), t.sort, t.created_at, t.updated_at,
		       (SELECT COUNT(*) FROM videos v WHERE v.topic_id = t.id),
		       (SELECT COUNT(*) FROM videos v WHERE v.topic_id = t.id AND v.status = 'fail'),
		       (SELECT COUNT(*) FROM categories c WHERE c.topic_id = t.id)
		FROM topics t
		ORDER BY t.sort DESC, t.id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := make([]Topic, 0, 16)
	for rows.Next() {
		var t Topic
		if err := rows.Scan(&t.ID, &t.Name, &t.Description, &t.Sort, &t.CreatedAt, &t.UpdatedAt,
			&t.VideoCount, &t.FailCount, &t.CategoryCount); err != nil {
			return nil, err
		}
		list = append(list, t)
	}
	return list, rows.Err()
}

func (s *Store) GetTopic(ctx context.Context, id int64) (*Topic, error) {
	var t Topic
	err := s.db.QueryRowContext(ctx, `
		SELECT t.id, t.name, COALESCE(t.description,''), t.sort, t.created_at, t.updated_at,
		       (SELECT COUNT(*) FROM videos v WHERE v.topic_id = t.id),
		       (SELECT COUNT(*) FROM videos v WHERE v.topic_id = t.id AND v.status = 'fail'),
		       (SELECT COUNT(*) FROM categories c WHERE c.topic_id = t.id)
		FROM topics t WHERE t.id = ?`, id).
		Scan(&t.ID, &t.Name, &t.Description, &t.Sort, &t.CreatedAt, &t.UpdatedAt,
			&t.VideoCount, &t.FailCount, &t.CategoryCount)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (s *Store) CreateTopic(ctx context.Context, t *Topic) error {
	res, err := s.db.ExecContext(ctx, `INSERT INTO topics (name, description, sort) VALUES (?,?,?)`,
		t.Name, t.Description, t.Sort)
	if err != nil {
		return err
	}
	t.ID, err = res.LastInsertId()
	return err
}

func (s *Store) UpdateTopic(ctx context.Context, t *Topic) error {
	res, err := s.db.ExecContext(ctx, `UPDATE topics SET name=?, description=?, sort=? WHERE id=?`,
		t.Name, t.Description, t.Sort, t.ID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		if ok, _ := s.TopicExists(ctx, t.ID); !ok {
			return ErrNotFound
		}
	}
	return nil
}

func (s *Store) DeleteTopic(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM topics WHERE id=?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) TopicExists(ctx context.Context, id int64) (bool, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM topics WHERE id=?`, id).Scan(&n)
	return n > 0, err
}

// ---------------- categories ----------------

const categoryCols = `c.id, c.topic_id, t.name, c.name, COALESCE(c.description,''), c.sort, c.created_at, c.updated_at,
	(SELECT COUNT(*) FROM videos v WHERE v.category_id = c.id),
	(SELECT COUNT(*) FROM videos v WHERE v.category_id = c.id AND v.status = 'fail')`

func scanCategories(rows *sql.Rows) ([]Category, error) {
	list := make([]Category, 0, 16)
	for rows.Next() {
		var c Category
		if err := rows.Scan(&c.ID, &c.TopicID, &c.TopicName, &c.Name, &c.Description, &c.Sort,
			&c.CreatedAt, &c.UpdatedAt, &c.VideoCount, &c.FailCount); err != nil {
			return nil, err
		}
		list = append(list, c)
	}
	return list, rows.Err()
}

func (s *Store) ListCategories(ctx context.Context, topicID int64) ([]Category, error) {
	where := ""
	args := []any{}
	if topicID > 0 {
		where = "WHERE c.topic_id = ?"
		args = append(args, topicID)
	}
	rows, err := s.db.QueryContext(ctx, `SELECT `+categoryCols+`
		FROM categories c JOIN topics t ON t.id = c.topic_id
		`+where+`
		ORDER BY c.sort DESC, c.id ASC`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanCategories(rows)
}

func (s *Store) GetCategory(ctx context.Context, id int64) (*Category, error) {
	var c Category
	err := s.db.QueryRowContext(ctx, `SELECT `+categoryCols+`
		FROM categories c JOIN topics t ON t.id = c.topic_id
		WHERE c.id = ?`, id).
		Scan(&c.ID, &c.TopicID, &c.TopicName, &c.Name, &c.Description, &c.Sort,
			&c.CreatedAt, &c.UpdatedAt, &c.VideoCount, &c.FailCount)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (s *Store) CreateCategory(ctx context.Context, c *Category) error {
	res, err := s.db.ExecContext(ctx, `INSERT INTO categories (topic_id, name, description, sort) VALUES (?,?,?,?)`,
		c.TopicID, c.Name, c.Description, c.Sort)
	if err != nil {
		return err
	}
	c.ID, err = res.LastInsertId()
	return err
}

func (s *Store) UpdateCategory(ctx context.Context, c *Category) error {
	res, err := s.db.ExecContext(ctx, `UPDATE categories SET topic_id=?, name=?, description=?, sort=? WHERE id=?`,
		c.TopicID, c.Name, c.Description, c.Sort, c.ID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		if _, err := s.GetCategory(ctx, c.ID); err != nil {
			return err
		}
	}
	return nil
}

// DeleteCategory 删除分类；分类下的视频若 keepVideos 为 true 则移到同主题的默认分类，否则一并删除。
func (s *Store) DeleteCategory(ctx context.Context, id int64, keepVideos bool) (removedVideoIDs []int64, err error) {
	cat, err := s.GetCategory(ctx, id)
	if err != nil {
		return nil, err
	}
	if keepVideos {
		target, err := s.ensureDefaultCategory(ctx, cat.TopicID)
		if err != nil {
			return nil, err
		}
		if _, err := s.db.ExecContext(ctx, `UPDATE videos SET category_id=? WHERE category_id=?`, target, id); err != nil {
			return nil, err
		}
	} else {
		rows, err := s.db.QueryContext(ctx, `SELECT id FROM videos WHERE category_id=?`, id)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var vid int64
			if err := rows.Scan(&vid); err != nil {
				rows.Close()
				return nil, err
			}
			removedVideoIDs = append(removedVideoIDs, vid)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, err
		}
		if _, err := s.db.ExecContext(ctx, `DELETE FROM videos WHERE category_id=?`, id); err != nil {
			return nil, err
		}
	}
	res, err := s.db.ExecContext(ctx, `DELETE FROM categories WHERE id=?`, id)
	if err != nil {
		return nil, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, ErrNotFound
	}
	return removedVideoIDs, nil
}

// ensureDefaultCategory 返回主题下的默认分类（不存在则创建）。
func (s *Store) ensureDefaultCategory(ctx context.Context, topicID int64) (int64, error) {
	var id int64
	err := s.db.QueryRowContext(ctx, `SELECT id FROM categories WHERE topic_id=? AND name=? LIMIT 1`, topicID, defaultCategoryName).Scan(&id)
	if err == nil {
		return id, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return 0, err
	}
	res, err := s.db.ExecContext(ctx, `INSERT INTO categories (topic_id, name, description, sort) VALUES (?,?,?,0)`,
		topicID, defaultCategoryName, "系统自动创建的默认分类")
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Store) CategoryExists(ctx context.Context, id int64) (bool, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM categories WHERE id=?`, id).Scan(&n)
	return n > 0, err
}

// ---------------- videos ----------------

const publicVideoCols = `v.id, v.topic_id, t.name, v.category_id, COALESCE(c.name,''), v.title,
	COALESCE(v.description,''), v.tags, v.sort, v.status, v.created_at, v.updated_at`

const publicVideoJoin = `FROM videos v
	JOIN topics t ON t.id = v.topic_id
	LEFT JOIN categories c ON c.id = v.category_id`

func scanPublicVideos(rows *sql.Rows) ([]Video, error) {
	list := make([]Video, 0, 32)
	for rows.Next() {
		var v Video
		if err := rows.Scan(&v.ID, &v.TopicID, &v.TopicName, &v.CategoryID, &v.CategoryName, &v.Title,
			&v.Description, &v.Tags, &v.Sort, &v.Status, &v.CreatedAt, &v.UpdatedAt); err != nil {
			return nil, err
		}
		list = append(list, v)
	}
	return list, rows.Err()
}

// PublicVideosByCategory 按分类查询视频；order 支持 newest（最新发布）/ oldest（最早发布），
// 其它值使用默认排序（sort 降序，其次新加入的在前）。
func (s *Store) PublicVideosByCategory(ctx context.Context, categoryID int64, order string) ([]Video, error) {
	orderBy := "v.sort DESC, v.id DESC"
	switch order {
	case "newest":
		orderBy = "v.created_at DESC, v.id DESC"
	case "oldest":
		orderBy = "v.created_at ASC, v.id ASC"
	}
	rows, err := s.db.QueryContext(ctx, `SELECT `+publicVideoCols+`
		`+publicVideoJoin+`
		WHERE v.category_id = ?
		ORDER BY `+orderBy, categoryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPublicVideos(rows)
}

// PublicVideosByTopic 按主题查询全部视频（跨分类），用于相关视频与主题级检索。
func (s *Store) PublicVideosByTopic(ctx context.Context, topicID int64, order string) ([]Video, error) {
	orderBy := "v.sort DESC, v.id DESC"
	switch order {
	case "newest":
		orderBy = "v.created_at DESC, v.id DESC"
	case "oldest":
		orderBy = "v.created_at ASC, v.id ASC"
	}
	rows, err := s.db.QueryContext(ctx, `SELECT `+publicVideoCols+`
		`+publicVideoJoin+`
		WHERE v.topic_id = ?
		ORDER BY `+orderBy, topicID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPublicVideos(rows)
}

func (s *Store) SearchPublicVideos(ctx context.Context, q string, limit int) ([]Video, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	like := likePattern(q)
	rows, err := s.db.QueryContext(ctx, `SELECT `+publicVideoCols+`
		`+publicVideoJoin+`
		WHERE v.title LIKE ? ESCAPE '\\' OR v.tags LIKE ? ESCAPE '\\'
		   OR COALESCE(v.description,'') LIKE ? ESCAPE '\\' OR t.name LIKE ? ESCAPE '\\'
		   OR COALESCE(c.name,'') LIKE ? ESCAPE '\\'
		ORDER BY v.sort DESC, v.id DESC
		LIMIT ?`, like, like, like, like, like, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPublicVideos(rows)
}

func (s *Store) GetVideoPublic(ctx context.Context, id int64) (*Video, error) {
	v, err := s.getVideo(ctx, id, false)
	if err != nil {
		return nil, err
	}
	v.URL = ""
	return v, nil
}

func (s *Store) GetVideo(ctx context.Context, id int64) (*Video, error) {
	return s.getVideo(ctx, id, true)
}

func (s *Store) getVideo(ctx context.Context, id int64, withURL bool) (*Video, error) {
	var v Video
	var checked sql.NullTime
	err := s.db.QueryRowContext(ctx, `SELECT v.id, v.topic_id, t.name, v.category_id, COALESCE(c.name,''),
			v.title, v.url, COALESCE(v.description,''), v.tags, v.sort, v.status, v.status_code, v.latency_ms,
			COALESCE(v.error_msg,''), v.checked_at, v.created_at, v.updated_at
		`+publicVideoJoin+`
		WHERE v.id = ?`, id).
		Scan(&v.ID, &v.TopicID, &v.TopicName, &v.CategoryID, &v.CategoryName, &v.Title, &v.URL,
			&v.Description, &v.Tags, &v.Sort, &v.Status, &v.StatusCode, &v.LatencyMS, &v.ErrorMsg,
			&checked, &v.CreatedAt, &v.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if checked.Valid {
		t := checked.Time
		v.CheckedAt = &t
	}
	if !withURL {
		v.URL = ""
	}
	return &v, nil
}

func (s *Store) AdminListVideos(ctx context.Context, f VideoFilter) ([]Video, int64, error) {
	where := []string{"1=1"}
	args := []any{}
	if f.TopicID > 0 {
		where = append(where, "v.topic_id = ?")
		args = append(args, f.TopicID)
	}
	if f.CategoryID > 0 {
		where = append(where, "v.category_id = ?")
		args = append(args, f.CategoryID)
	}
	if f.Keyword != "" {
		like := likePattern(f.Keyword)
		where = append(where, "(v.title LIKE ? ESCAPE '\\' OR v.tags LIKE ? ESCAPE '\\' OR v.url LIKE ? ESCAPE '\\')")
		args = append(args, like, like, like)
	}
	if f.Status == "ok" || f.Status == "fail" || f.Status == "unchecked" {
		where = append(where, "v.status = ?")
		args = append(args, f.Status)
	}
	cond := strings.Join(where, " AND ")

	var total int64
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) `+publicVideoJoin+` WHERE `+cond, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	if f.Page <= 0 {
		f.Page = 1
	}
	if f.PageSize <= 0 || f.PageSize > 200 {
		f.PageSize = 20
	}
	qArgs := append(append([]any{}, args...), f.PageSize, (f.Page-1)*f.PageSize)
	rows, err := s.db.QueryContext(ctx, `SELECT v.id, v.topic_id, t.name, v.category_id, COALESCE(c.name,''),
			v.title, v.url, COALESCE(v.description,''), v.tags, v.sort, v.status, v.status_code, v.latency_ms,
			COALESCE(v.error_msg,''), v.checked_at, v.created_at, v.updated_at
		`+publicVideoJoin+`
		WHERE `+cond+`
		ORDER BY v.sort DESC, v.id DESC
		LIMIT ? OFFSET ?`, qArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	list := make([]Video, 0, f.PageSize)
	for rows.Next() {
		var v Video
		var checked sql.NullTime
		if err := rows.Scan(&v.ID, &v.TopicID, &v.TopicName, &v.CategoryID, &v.CategoryName, &v.Title, &v.URL,
			&v.Description, &v.Tags, &v.Sort, &v.Status, &v.StatusCode, &v.LatencyMS, &v.ErrorMsg,
			&checked, &v.CreatedAt, &v.UpdatedAt); err != nil {
			return nil, 0, err
		}
		if checked.Valid {
			t := checked.Time
			v.CheckedAt = &t
		}
		list = append(list, v)
	}
	return list, total, rows.Err()
}

func (s *Store) CreateVideo(ctx context.Context, v *Video) error {
	if v.Status == "" {
		v.Status = "unchecked"
	}
	res, err := s.db.ExecContext(ctx, `INSERT INTO videos
		(topic_id, category_id, title, url, url_hash, description, tags, sort, status)
		VALUES (?,?,?,?,?,?,?,?,?)`,
		v.TopicID, v.CategoryID, v.Title, v.URL, hashURL(v.URL), v.Description, v.Tags, v.Sort, v.Status)
	if err != nil {
		if isDuplicateErr(err) {
			return ErrDuplicate
		}
		return err
	}
	v.ID, err = res.LastInsertId()
	return err
}

func (s *Store) UpdateVideo(ctx context.Context, v *Video) error {
	// 链接变化时重置健康状态，等待重新检测。
	// 注意：MySQL UPDATE 的赋值按顺序求值，以下 IF 条件均引用赋值前的 url 列。
	res, err := s.db.ExecContext(ctx, `UPDATE videos SET
		status = IF(url <> ?, 'unchecked', status),
		status_code = IF(url <> ?, 0, status_code),
		latency_ms = IF(url <> ?, 0, latency_ms),
		error_msg = IF(url <> ?, '', error_msg),
		checked_at = IF(url <> ?, NULL, checked_at),
		topic_id=?, category_id=?, title=?, url=?, url_hash=?, description=?, tags=?, sort=?
		WHERE id=?`,
		v.URL, v.URL, v.URL, v.URL, v.URL,
		v.TopicID, v.CategoryID, v.Title, v.URL, hashURL(v.URL), v.Description, v.Tags, v.Sort, v.ID)
	if err != nil {
		if isDuplicateErr(err) {
			return ErrDuplicate
		}
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		if _, err := s.GetVideo(ctx, v.ID); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) DeleteVideo(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM videos WHERE id=?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// ---------------- pdfs ----------------

func (s *Store) ListPDFsPublic(ctx context.Context, videoID int64) ([]PDF, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, video_id, title, file_name, file_size, mime_type, sort, created_at, updated_at
		FROM pdfs WHERE video_id=? ORDER BY sort DESC, id ASC`, videoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := make([]PDF, 0, 4)
	for rows.Next() {
		var p PDF
		if err := rows.Scan(&p.ID, &p.VideoID, &p.Title, &p.FileName, &p.FileSize, &p.MimeType, &p.Sort, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		list = append(list, p)
	}
	return list, rows.Err()
}

func (s *Store) ListPDFs(ctx context.Context, videoID int64) ([]PDF, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, video_id, title, file_path, file_name, file_size, mime_type, sort, created_at, updated_at
		FROM pdfs WHERE video_id=? ORDER BY sort DESC, id ASC`, videoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPDFs(rows)
}

func scanPDFs(rows *sql.Rows) ([]PDF, error) {
	list := make([]PDF, 0, 4)
	for rows.Next() {
		var p PDF
		if err := rows.Scan(&p.ID, &p.VideoID, &p.Title, &p.FilePath, &p.FileName, &p.FileSize, &p.MimeType, &p.Sort, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		list = append(list, p)
	}
	return list, rows.Err()
}

func (s *Store) GetPDF(ctx context.Context, id int64) (*PDF, error) {
	var p PDF
	err := s.db.QueryRowContext(ctx, `SELECT id, video_id, title, file_path, file_name, file_size, mime_type, sort, created_at, updated_at
		FROM pdfs WHERE id=?`, id).
		Scan(&p.ID, &p.VideoID, &p.Title, &p.FilePath, &p.FileName, &p.FileSize, &p.MimeType, &p.Sort, &p.CreatedAt, &p.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (s *Store) CreatePDF(ctx context.Context, p *PDF) error {
	res, err := s.db.ExecContext(ctx, `INSERT INTO pdfs (video_id, title, file_path, file_name, file_size, mime_type, sort)
		VALUES (?,?,?,?,?,?,?)`,
		p.VideoID, p.Title, p.FilePath, p.FileName, p.FileSize, p.MimeType, p.Sort)
	if err != nil {
		return err
	}
	p.ID, err = res.LastInsertId()
	return err
}

func (s *Store) UpdatePDF(ctx context.Context, p *PDF) error {
	res, err := s.db.ExecContext(ctx, `UPDATE pdfs SET title=?, file_path=?, file_name=?, file_size=?, mime_type=?, sort=? WHERE id=?`,
		p.Title, p.FilePath, p.FileName, p.FileSize, p.MimeType, p.Sort, p.ID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		if _, err := s.GetPDF(ctx, p.ID); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) UpdatePDFMeta(ctx context.Context, id int64, title string, sort int) error {
	res, err := s.db.ExecContext(ctx, `UPDATE pdfs SET title=?, sort=? WHERE id=?`, title, sort, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		if _, err := s.GetPDF(ctx, id); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) DeletePDF(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM pdfs WHERE id=?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// PDFsByVideo 返回某视频的全部资料（含文件路径），用于删除前收集待清理文件。
func (s *Store) PDFsByVideo(ctx context.Context, videoID int64) ([]PDF, error) {
	return s.ListPDFs(ctx, videoID)
}

// AllPDFFilePaths 返回所有资料的存储路径，用于启动时清理孤儿文件。
func (s *Store) AllPDFFilePaths(ctx context.Context) (map[string]struct{}, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT file_path FROM pdfs WHERE file_path <> ''`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	paths := make(map[string]struct{}, 32)
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, err
		}
		paths[p] = struct{}{}
	}
	return paths, rows.Err()
}

// PDFFilePathsByTopic 返回主题下所有资料的文件路径（删除主题前用于清理磁盘）。
func (s *Store) PDFFilePathsByTopic(ctx context.Context, topicID int64) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT p.file_path FROM pdfs p
		JOIN videos v ON v.id = p.video_id
		WHERE v.topic_id = ? AND p.file_path <> ''`, topicID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	paths := make([]string, 0, 16)
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, err
		}
		paths = append(paths, p)
	}
	return paths, rows.Err()
}

func (s *Store) VideoExists(ctx context.Context, id int64) (bool, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM videos WHERE id=?`, id).Scan(&n)
	return n > 0, err
}

// VideosByCategory 返回分类下的全部视频（精简字段），用于删除分类前收集资料文件。
func (s *Store) VideosByCategory(ctx context.Context, categoryID int64) ([]Video, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, topic_id, category_id, title, url, status FROM videos WHERE category_id=?`, categoryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := make([]Video, 0, 16)
	for rows.Next() {
		var v Video
		if err := rows.Scan(&v.ID, &v.TopicID, &v.CategoryID, &v.Title, &v.URL, &v.Status); err != nil {
			return nil, err
		}
		list = append(list, v)
	}
	return list, rows.Err()
}

func (s *Store) VideosForCheck(ctx context.Context) ([]Video, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, url, status FROM videos ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := make([]Video, 0, 64)
	for rows.Next() {
		var v Video
		if err := rows.Scan(&v.ID, &v.URL, &v.Status); err != nil {
			return nil, err
		}
		list = append(list, v)
	}
	return list, rows.Err()
}

func (s *Store) UpdateVideoCheck(ctx context.Context, id int64, status string, code int, latencyMS int64, errMsg string, checkedAt time.Time) error {
	_, err := s.db.ExecContext(ctx, `UPDATE videos SET status=?, status_code=?, latency_ms=?, error_msg=?, checked_at=? WHERE id=?`,
		status, code, latencyMS, errMsg, checkedAt, id)
	return err
}

func (s *Store) Stats(ctx context.Context) (Stats, error) {
	var st Stats
	err := s.db.QueryRowContext(ctx, `SELECT
		(SELECT COUNT(*) FROM topics),
		(SELECT COUNT(*) FROM categories),
		(SELECT COUNT(*) FROM videos),
		(SELECT COUNT(*) FROM videos WHERE status='fail'),
		(SELECT COUNT(*) FROM videos WHERE status='unchecked'),
		(SELECT COUNT(*) FROM blocked_ips WHERE blocked_until > NOW()),
		(SELECT COALESCE(SUM(video_hits),0) FROM ip_daily WHERE day=CURDATE()),
		(SELECT COALESCE(SUM(total_hits),0) FROM ip_daily WHERE day=CURDATE())`).
		Scan(&st.Topics, &st.Categories, &st.Videos, &st.FailVideos, &st.Unchecked, &st.BlockedIPs, &st.TodayPlays, &st.TodayVisits)
	return st, err
}

// ---------------- traffic / blocking ----------------

func (s *Store) IncrDaily(ctx context.Context, ip string, isVideo bool) (int64, int64, error) {
	v := 0
	if isVideo {
		v = 1
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO ip_daily (ip, day, video_hits, total_hits)
		VALUES (?, CURDATE(), ?, ?) AS new
		ON DUPLICATE KEY UPDATE
			video_hits = ip_daily.video_hits + new.video_hits,
			total_hits = ip_daily.total_hits + new.total_hits`, ip, v, 1)
	if err != nil {
		return 0, 0, err
	}
	var videoHits, totalHits int64
	err = s.db.QueryRowContext(ctx, `SELECT video_hits, total_hits FROM ip_daily WHERE ip=? AND day=CURDATE()`, ip).
		Scan(&videoHits, &totalHits)
	return videoHits, totalHits, err
}

func (s *Store) SetBlocked(ctx context.Context, ip, reason string, until time.Time) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO blocked_ips (ip, reason, blocked_until) VALUES (?,?,?)
		ON DUPLICATE KEY UPDATE reason=VALUES(reason), blocked_until=GREATEST(blocked_until, VALUES(blocked_until))`,
		ip, reason, until)
	return err
}

func (s *Store) ListBlocked(ctx context.Context) ([]BlockedIP, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT ip, reason, blocked_until, created_at FROM blocked_ips
		WHERE blocked_until > NOW() ORDER BY blocked_until DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := make([]BlockedIP, 0, 16)
	for rows.Next() {
		var b BlockedIP
		if err := rows.Scan(&b.IP, &b.Reason, &b.BlockedUntil, &b.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, b)
	}
	return list, rows.Err()
}

func (s *Store) IsBlocked(ctx context.Context, ip string) (bool, time.Time, error) {
	var until time.Time
	err := s.db.QueryRowContext(ctx, `SELECT blocked_until FROM blocked_ips WHERE ip=? AND blocked_until > NOW()`, ip).Scan(&until)
	if errors.Is(err, sql.ErrNoRows) {
		return false, time.Time{}, nil
	}
	if err != nil {
		return false, time.Time{}, err
	}
	return true, until, nil
}

func (s *Store) DeleteBlocked(ctx context.Context, ip string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM blocked_ips WHERE ip=?`, ip)
	return err
}

func (s *Store) CleanupBlocked(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM blocked_ips WHERE blocked_until < NOW()`)
	return err
}

func (s *Store) ListRecentUsage(ctx context.Context, limit int) ([]map[string]any, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx, `SELECT ip, video_hits, total_hits, updated_at FROM ip_daily
		WHERE day=CURDATE() ORDER BY video_hits DESC, total_hits DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := make([]map[string]any, 0, limit)
	for rows.Next() {
		var ip string
		var vh, th int64
		var updated time.Time
		if err := rows.Scan(&ip, &vh, &th, &updated); err != nil {
			return nil, err
		}
		list = append(list, map[string]any{"ip": ip, "video_hits": vh, "total_hits": th, "updated_at": updated})
	}
	return list, rows.Err()
}
