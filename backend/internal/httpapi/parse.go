package httpapi

import (
	"fmt"
	"net/url"
	"path"
	"strings"
	"unicode/utf8"
)

// BatchItem 是批量录入中的一行解析结果。
type BatchItem struct {
	Title       string `json:"title"`
	Tags        string `json:"tags"`
	URL         string `json:"url"`
	Description string `json:"description"`
	Line        int    `json:"line"`
	Error       string `json:"error,omitempty"`
}

// ParseBatchLines 解析批量录入文本，每行一条，支持格式：
//
//	链接
//	标题 | 链接
//	标题 | 标签1,标签2 | 链接
//	标题 | 标签1,标签2 | 链接 | 备注
//
// 链接可以出现在任意位置（以 http:// 或 https:// 开头的那一段）。
func ParseBatchLines(text string) []BatchItem {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	items := make([]BatchItem, 0, len(lines))
	for i, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		item := BatchItem{Line: i + 1}
		parts := strings.Split(line, "|")
		urlIdx := -1
		for j, p := range parts {
			p = strings.TrimSpace(p)
			parts[j] = p
			if urlIdx == -1 && isHTTPURL(p) {
				urlIdx = j
			}
		}
		if urlIdx == -1 {
			item.Error = "该行未找到以 http(s):// 开头的视频链接"
			items = append(items, item)
			continue
		}
		item.URL = parts[urlIdx]
		before := parts[:urlIdx]
		after := parts[urlIdx+1:]

		for _, p := range before {
			if p == "" {
				continue
			}
			if item.Title == "" {
				item.Title = p
			} else if item.Tags == "" {
				item.Tags = p
			} else {
				item.Description = joinNonEmpty(item.Description, p)
			}
		}
		for _, p := range after {
			if p == "" {
				continue
			}
			item.Description = joinNonEmpty(item.Description, p)
		}
		if item.Title == "" {
			item.Title = titleFromURL(item.URL)
		}
		items = append(items, item)
	}
	return items
}

func joinNonEmpty(a, b string) string {
	if a == "" {
		return b
	}
	return a + "\n" + b
}

func isHTTPURL(s string) bool {
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")
}

func titleFromURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return "未命名视频"
	}
	base := path.Base(u.Path)
	base = strings.TrimSuffix(base, path.Ext(base))
	if base == "" || base == "/" || base == "." {
		return "未命名视频"
	}
	if utf8.RuneCountInString(base) > 60 {
		base = string([]rune(base)[:60])
	}
	return base
}

// ValidateVideoURL 校验链接合法性。
func ValidateVideoURL(raw string) error {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return fmt.Errorf("链接格式不合法")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("链接必须以 http:// 或 https:// 开头")
	}
	if u.Host == "" {
		return fmt.Errorf("链接缺少域名")
	}
	return nil
}
