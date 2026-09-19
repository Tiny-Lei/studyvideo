package filestore

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func newStore(t *testing.T) *Store {
	t.Helper()
	s, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("创建存储失败: %v", err)
	}
	return s
}

func TestSaveOpenRemove(t *testing.T) {
	s := newStore(t)
	content := []byte("%PDF-1.7 test content")

	rel, size, err := s.Save(bytes.NewReader(content), ".pdf", 1024)
	if err != nil {
		t.Fatalf("保存失败: %v", err)
	}
	if size != int64(len(content)) {
		t.Fatalf("写入大小错误: %d", size)
	}
	if !strings.HasSuffix(rel, ".pdf") || strings.Contains(rel, "..") {
		t.Fatalf("相对路径不符合预期: %s", rel)
	}

	if !s.Exists(rel) {
		t.Fatal("保存后文件应存在")
	}
	f, info, err := s.Open(rel)
	if err != nil {
		t.Fatalf("打开失败: %v", err)
	}
	defer f.Close()
	if info.Size() != int64(len(content)) {
		t.Fatalf("文件大小错误: %d", info.Size())
	}
	got := make([]byte, len(content))
	if _, err := f.Read(got); err != nil {
		t.Fatalf("读取失败: %v", err)
	}
	if !bytes.Equal(got, content) {
		t.Fatalf("内容不一致: %q", got)
	}

	if err := s.Remove(rel); err != nil {
		t.Fatalf("删除失败: %v", err)
	}
	if s.Exists(rel) {
		t.Fatal("删除后文件不应存在")
	}
	if err := s.Remove(rel); err != nil {
		t.Fatalf("重复删除应幂等: %v", err)
	}
	if _, _, err := s.Open(rel); !errors.Is(err, ErrNotFound) {
		t.Fatalf("打开不存在的文件应返回 ErrNotFound, got %v", err)
	}
}

func TestSaveLimits(t *testing.T) {
	s := newStore(t)
	if _, _, err := s.Save(bytes.NewReader(nil), ".pdf", 10); err == nil {
		t.Fatal("空文件应被拒绝")
	}
	big := bytes.Repeat([]byte("x"), 100)
	if _, _, err := s.Save(bytes.NewReader(big), ".pdf", 10); err == nil {
		t.Fatal("超过大小限制应被拒绝")
	}
	entries, _ := os.ReadDir(filepath.Join(s.Root(), "2006"))
	if len(entries) > 0 {
		t.Fatal("失败的上传不应残留目录/文件")
	}
}

func TestResolveRejectsTraversal(t *testing.T) {
	s := newStore(t)
	for _, bad := range []string{"../etc/passwd", "/etc/passwd", "..", "a/../../b.pdf", ""} {
		if _, err := s.resolve(bad); err == nil {
			t.Errorf("路径 %q 应被拒绝", bad)
		}
	}
	if _, err := s.resolve("2026/01/abc.pdf"); err != nil {
		t.Errorf("合法路径被拒绝: %v", err)
	}
}

func TestNormalizeExt(t *testing.T) {
	cases := map[string]string{
		"pdf":  ".pdf",
		".PDF": ".pdf",
		"exe":  ".pdf",
		"docx": ".docx",
		"":     ".pdf",
	}
	for in, want := range cases {
		if got := normalizeExt(in); got != want {
			t.Errorf("normalizeExt(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestLooksLikePDF(t *testing.T) {
	s := newStore(t)
	pdfRel, _, err := s.Save(bytes.NewReader([]byte("%PDF-1.7 content")), ".pdf", 1024)
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := s.LooksLikePDF(pdfRel); err != nil || !ok {
		t.Fatalf("PDF 文件应被识别, ok=%v err=%v", ok, err)
	}
	fakeRel, _, err := s.Save(bytes.NewReader([]byte("just text")), ".pdf", 1024)
	if err != nil {
		t.Fatal(err)
	}
	if ok, _ := s.LooksLikePDF(fakeRel); ok {
		t.Fatal("非 PDF 内容不应被识别")
	}
	if _, err := s.LooksLikePDF("2026/01/missing.pdf"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("不存在的文件应返回 ErrNotFound, got %v", err)
	}
}
