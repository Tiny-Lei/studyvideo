package store

import (
	"context"
	"database/sql"
	"errors"
	"sort"
	"strings"
)

// ---------------- 资料分类 ----------------

const materialCategoryCols = `c.id, c.name, COALESCE(c.description,''), COALESCE(c.tags,''), c.sort, c.created_at, c.updated_at,
	(SELECT COUNT(*) FROM materials m WHERE m.category_id = c.id)`

func (s *Store) ListMaterialCategories(ctx context.Context) ([]MaterialCategory, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+materialCategoryCols+`
		FROM material_categories c
		ORDER BY c.sort DESC, c.id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := make([]MaterialCategory, 0, 16)
	for rows.Next() {
		var c MaterialCategory
		if err := rows.Scan(&c.ID, &c.Name, &c.Description, &c.Tags, &c.Sort, &c.CreatedAt, &c.UpdatedAt, &c.MaterialCount); err != nil {
			return nil, err
		}
		list = append(list, c)
	}
	return list, rows.Err()
}

func (s *Store) GetMaterialCategory(ctx context.Context, id int64) (*MaterialCategory, error) {
	var c MaterialCategory
	err := s.db.QueryRowContext(ctx, `SELECT `+materialCategoryCols+`
		FROM material_categories c WHERE c.id = ?`, id).
		Scan(&c.ID, &c.Name, &c.Description, &c.Tags, &c.Sort, &c.CreatedAt, &c.UpdatedAt, &c.MaterialCount)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (s *Store) CreateMaterialCategory(ctx context.Context, c *MaterialCategory) error {
	res, err := s.db.ExecContext(ctx, `INSERT INTO material_categories (name, description, tags, sort) VALUES (?,?,?,?)`,
		c.Name, c.Description, c.Tags, c.Sort)
	if err != nil {
		return err
	}
	c.ID, err = res.LastInsertId()
	return err
}

func (s *Store) UpdateMaterialCategory(ctx context.Context, c *MaterialCategory) error {
	res, err := s.db.ExecContext(ctx, `UPDATE material_categories SET name=?, description=?, tags=?, sort=? WHERE id=?`,
		c.Name, c.Description, c.Tags, c.Sort, c.ID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		if _, err := s.GetMaterialCategory(ctx, c.ID); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) DeleteMaterialCategory(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM material_categories WHERE id=?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) MaterialCategoryExists(ctx context.Context, id int64) (bool, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM material_categories WHERE id=?`, id).Scan(&n)
	return n > 0, err
}

// ---------------- 资料 ----------------

const materialCols = `m.id, m.category_id, c.name, m.group_name, m.title, COALESCE(m.description,''), m.tags,
	m.file_path, m.file_name, m.file_size, m.mime_type, m.file_ext, m.sort, m.created_at, m.updated_at`

func scanMaterials(rows *sql.Rows) ([]Material, error) {
	list := make([]Material, 0, 32)
	for rows.Next() {
		var m Material
		if err := rows.Scan(&m.ID, &m.CategoryID, &m.CategoryName, &m.GroupName, &m.Title, &m.Description,
			&m.Tags, &m.FilePath, &m.FileName, &m.FileSize, &m.MimeType, &m.FileExt, &m.Sort,
			&m.CreatedAt, &m.UpdatedAt); err != nil {
			return nil, err
		}
		list = append(list, m)
	}
	return list, rows.Err()
}

func (s *Store) ListMaterials(ctx context.Context, f MaterialFilter) ([]Material, int64, error) {
	where := []string{"1=1"}
	args := []any{}
	if f.CategoryID > 0 {
		where = append(where, "m.category_id = ?")
		args = append(args, f.CategoryID)
	}
	if f.GroupName != "" {
		where = append(where, "m.group_name = ?")
		args = append(args, f.GroupName)
	}
	if f.FileExt != "" {
		where = append(where, "m.file_ext = ?")
		args = append(args, f.FileExt)
	}
	if f.Keyword != "" {
		like := likePattern(f.Keyword)
		where = append(where, `(m.title LIKE ? ESCAPE '\\' OR m.tags LIKE ? ESCAPE '\\'
			OR COALESCE(m.description,'') LIKE ? ESCAPE '\\' OR m.group_name LIKE ? ESCAPE '\\'
			OR c.name LIKE ? ESCAPE '\\')`)
		args = append(args, like, like, like, like, like)
	}
	cond := strings.Join(where, " AND ")

	var total int64
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM materials m
		JOIN material_categories c ON c.id = m.category_id WHERE `+cond, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	if f.Page <= 0 {
		f.Page = 1
	}
	if f.PageSize <= 0 || f.PageSize > 200 {
		f.PageSize = 20
	}
	qArgs := append(append([]any{}, args...), f.PageSize, (f.Page-1)*f.PageSize)
	rows, err := s.db.QueryContext(ctx, `SELECT `+materialCols+`
		FROM materials m JOIN material_categories c ON c.id = m.category_id
		WHERE `+cond+`
		ORDER BY m.sort DESC, m.group_name DESC, m.id DESC
		LIMIT ? OFFSET ?`, qArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	list, err := scanMaterials(rows)
	if err != nil {
		return nil, 0, err
	}
	// 公开列表不暴露服务器路径
	for i := range list {
		list[i].FilePath = ""
	}
	return list, total, nil
}

// SearchMaterials 仅在资料库内搜索（与视频搜索完全独立）。
func (s *Store) SearchMaterials(ctx context.Context, q string, limit int) ([]Material, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	like := likePattern(q)
	rows, err := s.db.QueryContext(ctx, `SELECT `+materialCols+`
		FROM materials m JOIN material_categories c ON c.id = m.category_id
		WHERE m.title LIKE ? ESCAPE '\\' OR m.tags LIKE ? ESCAPE '\\'
		   OR COALESCE(m.description,'') LIKE ? ESCAPE '\\' OR m.group_name LIKE ? ESCAPE '\\'
		   OR c.name LIKE ? ESCAPE '\\'
		ORDER BY m.sort DESC, m.id DESC
		LIMIT ?`, like, like, like, like, like, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list, err := scanMaterials(rows)
	if err != nil {
		return nil, err
	}
	for i := range list {
		list[i].FilePath = ""
	}
	return list, nil
}

// GetMaterialPublic 公开详情（不含文件路径）。
func (s *Store) GetMaterialPublic(ctx context.Context, id int64) (*Material, error) {
	m, err := s.GetMaterial(ctx, id)
	if err != nil {
		return nil, err
	}
	m.FilePath = ""
	return m, nil
}

func (s *Store) GetMaterial(ctx context.Context, id int64) (*Material, error) {
	var m Material
	err := s.db.QueryRowContext(ctx, `SELECT `+materialCols+`
		FROM materials m JOIN material_categories c ON c.id = m.category_id
		WHERE m.id = ?`, id).
		Scan(&m.ID, &m.CategoryID, &m.CategoryName, &m.GroupName, &m.Title, &m.Description,
			&m.Tags, &m.FilePath, &m.FileName, &m.FileSize, &m.MimeType, &m.FileExt, &m.Sort,
			&m.CreatedAt, &m.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &m, nil
}

func (s *Store) CreateMaterial(ctx context.Context, m *Material) error {
	res, err := s.db.ExecContext(ctx, `INSERT INTO materials
		(category_id, group_name, title, description, tags, file_path, file_name, file_size, mime_type, file_ext, sort)
		VALUES (?,?,?,?,?,?,?,?,?,?,?)`,
		m.CategoryID, m.GroupName, m.Title, m.Description, m.Tags,
		m.FilePath, m.FileName, m.FileSize, m.MimeType, m.FileExt, m.Sort)
	if err != nil {
		return err
	}
	m.ID, err = res.LastInsertId()
	return err
}

func (s *Store) UpdateMaterial(ctx context.Context, m *Material) error {
	res, err := s.db.ExecContext(ctx, `UPDATE materials SET
		category_id=?, group_name=?, title=?, description=?, tags=?,
		file_path=?, file_name=?, file_size=?, mime_type=?, file_ext=?, sort=?
		WHERE id=?`,
		m.CategoryID, m.GroupName, m.Title, m.Description, m.Tags,
		m.FilePath, m.FileName, m.FileSize, m.MimeType, m.FileExt, m.Sort, m.ID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		if _, err := s.GetMaterial(ctx, m.ID); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) DeleteMaterial(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM materials WHERE id=?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// MaterialsByCategory 返回分类下全部资料（含路径），用于删除分类前收集文件。
func (s *Store) MaterialsByCategory(ctx context.Context, categoryID int64) ([]Material, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, file_path FROM materials WHERE category_id=?`, categoryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := make([]Material, 0, 16)
	for rows.Next() {
		var m Material
		if err := rows.Scan(&m.ID, &m.FilePath); err != nil {
			return nil, err
		}
		list = append(list, m)
	}
	return list, rows.Err()
}

// MaterialGroups 返回某分类下的分组名列表（用于前台按套题分组展示）。
func (s *Store) MaterialGroups(ctx context.Context, categoryID int64) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT DISTINCT group_name FROM materials
		WHERE category_id=? AND group_name <> '' ORDER BY group_name DESC`, categoryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	groups := make([]string, 0, 16)
	for rows.Next() {
		var g string
		if err := rows.Scan(&g); err != nil {
			return nil, err
		}
		groups = append(groups, g)
	}
	return groups, rows.Err()
}

// MaterialStats 返回资料总数与分类数（用于资料区首页）。
func (s *Store) MaterialStats(ctx context.Context) (int64, int64, error) {
	var materials, categories int64
	err := s.db.QueryRowContext(ctx, `SELECT
		(SELECT COUNT(*) FROM materials),
		(SELECT COUNT(*) FROM material_categories)`).Scan(&materials, &categories)
	return materials, categories, err
}

// PDFFilePathsByMaterialCategory 返回分类下所有资料文件路径（删除分类前清理磁盘用）。
func (s *Store) PDFFilePathsByMaterialCategory(ctx context.Context, categoryID int64) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT file_path FROM materials WHERE category_id=? AND file_path <> ''`, categoryID)
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

// TagCount 标签及其资料数量。
type TagCount struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// MaterialTagStats 统计某分类下各标签的资料数量（预设但尚未使用的标签计 0）。
func (s *Store) MaterialTagStats(ctx context.Context, categoryID int64) ([]TagCount, error) {
	// 预设标签（保持后台设置的顺序）
	preset := make([]string, 0, 8)
	category, err := s.GetMaterialCategory(ctx, categoryID)
	if err != nil {
		return nil, err
	}
	if category.Tags != "" {
		for _, t := range strings.Split(category.Tags, ",") {
			if t = strings.TrimSpace(t); t != "" {
				preset = append(preset, t)
			}
		}
	}

	// 实际使用情况
	rows, err := s.db.QueryContext(ctx, `SELECT tags FROM materials WHERE category_id=? AND tags <> ''`, categoryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	counts := make(map[string]int, 16)
	for rows.Next() {
		var tags string
		if err := rows.Scan(&tags); err != nil {
			return nil, err
		}
		for _, t := range strings.Split(tags, ",") {
			if t = strings.TrimSpace(t); t != "" {
				counts[t]++
			}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// 预设优先，其余按数量倒序
	result := make([]TagCount, 0, len(counts)+len(preset))
	seen := make(map[string]bool, len(counts)+len(preset))
	for _, name := range preset {
		result = append(result, TagCount{Name: name, Count: counts[name]})
		seen[name] = true
	}
	rest := make([]TagCount, 0, len(counts))
	for name, n := range counts {
		if !seen[name] {
			rest = append(rest, TagCount{Name: name, Count: n})
		}
	}
	sort.Slice(rest, func(i, j int) bool {
		if rest[i].Count != rest[j].Count {
			return rest[i].Count > rest[j].Count
		}
		return rest[i].Name < rest[j].Name
	})
	return append(result, rest...), nil
}
