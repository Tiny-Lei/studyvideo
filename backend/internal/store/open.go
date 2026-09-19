package store

import (
	"database/sql"
	_ "embed"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
)

//go:embed schema.sql
var schemaSQL string

// Open 连接 MySQL 并自动建库、建表。
func Open(dsn string) (*sql.DB, error) {
	mysqlCfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		return nil, fmt.Errorf("解析 DB_DSN 失败: %w", err)
	}

	bootstrapCfg := *mysqlCfg
	bootstrapCfg.DBName = ""
	bootstrapCfg.Timeout = 5 * time.Second
	bs, err := sql.Open("mysql", bootstrapCfg.FormatDSN())
	if err != nil {
		return nil, fmt.Errorf("连接 MySQL 失败: %w", err)
	}
	if err := bs.Ping(); err != nil {
		bs.Close()
		return nil, fmt.Errorf("连接 MySQL 失败（请检查 DB_DSN）: %w", err)
	}
	if mysqlCfg.DBName != "" {
		_, err = bs.Exec("CREATE DATABASE IF NOT EXISTS `" + strings.ReplaceAll(mysqlCfg.DBName, "`", "``") +
			"` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci")
		if err != nil {
			slog.Warn("自动创建数据库失败（若数据库已存在可忽略）", "error", err)
		}
	}
	bs.Close()

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("打开 MySQL 失败: %w", err)
	}
	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(time.Hour)
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("Ping MySQL 失败: %w", err)
	}
	if err := migrate(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("初始化表结构失败: %w", err)
	}
	return db, nil
}

func migrate(db *sql.DB) error {
	for _, stmt := range strings.Split(schemaSQL, ";") {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("%w (statement: %.80s)", err, stmt)
		}
	}
	if err := migratePDFsToFileStorage(db); err != nil {
		return err
	}
	return migrateVideosToCategories(db)
}

// migrateVideosToCategories 为旧库补充 categories 结构：
//  1. 给 videos 增加 category_id 列（如缺失）；
//  2. 为每个已有视频的主题创建默认分类「未分类」，把该主题下没有分类的视频归入其中。
//
// 新库（无历史数据）不受影响。
func migrateVideosToCategories(db *sql.DB) error {
	cols, err := tableColumns(db, "videos")
	if err != nil {
		return err
	}
	if _, ok := cols["category_id"]; !ok {
		if _, err := db.Exec("ALTER TABLE videos ADD COLUMN category_id BIGINT UNSIGNED NOT NULL DEFAULT 0 AFTER topic_id"); err != nil {
			return fmt.Errorf("为 videos 添加 category_id 失败: %w", err)
		}
		if _, err := db.Exec("ALTER TABLE videos ADD INDEX idx_category_sort (category_id, sort)"); err != nil {
			slog.Warn("添加 category_id 索引失败（可忽略）", "error", err)
		}
	}

	// 找出有「未归类视频」的主题
	rows, err := db.Query(`SELECT DISTINCT topic_id FROM videos WHERE category_id = 0`)
	if err != nil {
		return err
	}
	var topicIDs []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		topicIDs = append(topicIDs, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	for _, topicID := range topicIDs {
		var categoryID int64
		err := db.QueryRow(`SELECT id FROM categories WHERE topic_id = ? AND name = '未分类' LIMIT 1`, topicID).Scan(&categoryID)
		if errors.Is(err, sql.ErrNoRows) {
			res, err := db.Exec(`INSERT INTO categories (topic_id, name, description, sort) VALUES (?, '未分类', '历史数据自动归入的分类，可重命名或移动', 0)`, topicID)
			if err != nil {
				return fmt.Errorf("创建默认分类失败: %w", err)
			}
			categoryID, err = res.LastInsertId()
			if err != nil {
				return err
			}
		} else if err != nil {
			return err
		}
		if _, err := db.Exec(`UPDATE videos SET category_id = ? WHERE topic_id = ? AND category_id = 0`, categoryID, topicID); err != nil {
			return fmt.Errorf("归类历史视频失败: %w", err)
		}
		slog.Info("已将历史视频归入默认分类", "topic_id", topicID, "category_id", categoryID)
	}
	return nil
}

// migratePDFsToFileStorage 把早期「PDF 链接版」的 pdfs 表升级为「本站文件存储版」。
// 旧数据中的外链无法转成本地文件，迁移时保留记录（file_path 为空），
// 管理端会标记为「文件缺失」，由运营重新上传。
func migratePDFsToFileStorage(db *sql.DB) error {
	cols, err := tableColumns(db, "pdfs")
	if err != nil {
		return err
	}
	if len(cols) == 0 {
		return nil
	}

	additions := map[string]string{
		"file_path": "VARCHAR(512) NOT NULL DEFAULT '' AFTER title",
		"file_name": "VARCHAR(255) NOT NULL DEFAULT '' AFTER file_path",
		"file_size": "BIGINT UNSIGNED NOT NULL DEFAULT 0 AFTER file_name",
		"mime_type": "VARCHAR(100) NOT NULL DEFAULT '' AFTER file_size",
	}
	for name, def := range additions {
		if _, ok := cols[name]; ok {
			continue
		}
		if _, err := db.Exec("ALTER TABLE pdfs ADD COLUMN " + name + " " + def); err != nil {
			return fmt.Errorf("为 pdfs 添加列 %s 失败: %w", name, err)
		}
	}

	// 旧的直链列在新结构下不再使用，且为 NOT NULL，必须移除。
	for _, name := range []string{"url", "url_hash"} {
		if _, ok := cols[name]; !ok {
			continue
		}
		if _, err := db.Exec("ALTER TABLE pdfs DROP COLUMN " + name); err != nil {
			return fmt.Errorf("移除 pdfs 旧列 %s 失败: %w", name, err)
		}
	}
	slog.Info("pdfs 表已升级为本地文件存储结构")
	return nil
}

func tableColumns(db *sql.DB, table string) (map[string]struct{}, error) {
	rows, err := db.Query(`SELECT COLUMN_NAME FROM information_schema.COLUMNS
		WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ?`, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	cols := make(map[string]struct{}, 8)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		cols[name] = struct{}{}
	}
	return cols, rows.Err()
}
