// Package filestore 负责本站自有文件（目前为配套 PDF 资料）的落盘、读取与删除。
//
// 设计要点：
//   - 文件按日期分目录存放，文件名使用随机 ID，避免中文/特殊字符与重名问题；
//   - 所有对外暴露的路径都经过校验，杜绝路径穿越；
//   - 删除是幂等操作，文件不存在不算错误。
package filestore

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ErrNotFound 表示存储的文件不存在。
var ErrNotFound = errors.New("文件不存在")

type Store struct {
	root string
}

func New(root string) (*Store, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("解析存储目录失败: %w", err)
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		return nil, fmt.Errorf("创建存储目录失败: %w", err)
	}
	return &Store{root: abs}, nil
}

// Root 返回存储根目录的绝对路径。
func (s *Store) Root() string { return s.root }

// Save 将 src 的内容写入存储，返回可持久化的相对路径（如 2026/09/ab12cd.pdf）。
// ext 会被规范化为允许的扩展名（PDF 场景下非白名单一律按 .pdf 处理）。
// size 为允许的最大字节数，<=0 表示不限制。
func (s *Store) Save(src io.Reader, ext string, size int64) (string, int64, error) {
	ext = normalizeExt(ext)
	relDir := time.Now().Format("2006/01")
	absDir := filepath.Join(s.root, filepath.FromSlash(relDir))
	if err := os.MkdirAll(absDir, 0o755); err != nil {
		return "", 0, fmt.Errorf("创建目录失败: %w", err)
	}

	name, err := randomName(ext)
	if err != nil {
		return "", 0, err
	}
	absPath := filepath.Join(absDir, name)
	relPath := relDir + "/" + name

	f, err := os.OpenFile(absPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return "", 0, fmt.Errorf("创建文件失败: %w", err)
	}

	var reader io.Reader = src
	if size > 0 {
		reader = io.LimitReader(src, size+1)
	}
	written, err := io.Copy(f, reader)
	closeErr := f.Close()
	if err != nil || closeErr != nil {
		_ = os.Remove(absPath)
		if err == nil {
			err = closeErr
		}
		return "", 0, fmt.Errorf("写入文件失败: %w", err)
	}
	if size > 0 && written > size {
		_ = os.Remove(absPath)
		return "", 0, fmt.Errorf("文件超过大小限制")
	}
	if written == 0 {
		_ = os.Remove(absPath)
		return "", 0, fmt.Errorf("文件内容为空")
	}
	return relPath, written, nil
}

// Open 打开存储中的文件，返回文件句柄与信息。
func (s *Store) Open(relPath string) (*os.File, os.FileInfo, error) {
	abs, err := s.resolve(relPath)
	if err != nil {
		return nil, nil, err
	}
	f, err := os.Open(abs)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, ErrNotFound
		}
		return nil, nil, err
	}
	info, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, nil, err
	}
	if info.IsDir() {
		f.Close()
		return nil, nil, ErrNotFound
	}
	return f, info, nil
}

// Exists 判断文件是否存在。
func (s *Store) Exists(relPath string) bool {
	abs, err := s.resolve(relPath)
	if err != nil {
		return false
	}
	info, err := os.Stat(abs)
	return err == nil && !info.IsDir()
}

// LooksLikePDF 通过文件头判断内容是否为 PDF，防止仅改扩展名的伪装文件。
func (s *Store) LooksLikePDF(relPath string) (bool, error) {
	abs, err := s.resolve(relPath)
	if err != nil {
		return false, err
	}
	f, err := os.Open(abs)
	if err != nil {
		if os.IsNotExist(err) {
			return false, ErrNotFound
		}
		return false, err
	}
	defer f.Close()
	head := make([]byte, 5)
	n, err := io.ReadFull(f, head)
	if err != nil && err != io.ErrUnexpectedEOF {
		return false, nil
	}
	return n >= 5 && string(head[:5]) == "%PDF-", nil
}

// Remove 删除文件（幂等）。
func (s *Store) Remove(relPath string) error {
	if strings.TrimSpace(relPath) == "" {
		return nil
	}
	abs, err := s.resolve(relPath)
	if err != nil {
		return err
	}
	if err := os.Remove(abs); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// resolve 将相对路径解析为绝对路径，并确保结果位于存储根目录内。
func (s *Store) resolve(relPath string) (string, error) {
	trimmed := strings.TrimSpace(relPath)
	if trimmed == "" || filepath.IsAbs(trimmed) || strings.HasPrefix(trimmed, "/") || strings.Contains(trimmed, "\\") {
		return "", fmt.Errorf("非法的文件路径: %s", relPath)
	}
	clean := filepath.Clean(filepath.FromSlash(trimmed))
	if clean == "." || clean == string(filepath.Separator) || strings.HasPrefix(clean, "..") {
		return "", fmt.Errorf("非法的文件路径: %s", relPath)
	}
	abs := filepath.Join(s.root, clean)
	if abs != s.root && !strings.HasPrefix(abs, s.root+string(filepath.Separator)) {
		return "", fmt.Errorf("非法的文件路径: %s", relPath)
	}
	return abs, nil
}

func normalizeExt(ext string) string {
	ext = strings.ToLower(strings.TrimSpace(ext))
	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	if len(ext) > 10 {
		ext = ".pdf"
	}
	switch ext {
	case ".pdf", ".doc", ".docx", ".ppt", ".pptx", ".xls", ".xlsx", ".zip", ".rar", ".7z", ".txt", ".md", ".png", ".jpg", ".jpeg", ".webp", ".mp3", ".mp4":
		return ext
	default:
		return ".pdf"
	}
}

func randomName(ext string) (string, error) {
	buf := make([]byte, 12)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("生成文件名失败: %w", err)
	}
	return hex.EncodeToString(buf) + ext, nil
}
