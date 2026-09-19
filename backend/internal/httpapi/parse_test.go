package httpapi

import "testing"

func TestParseBatchLines(t *testing.T) {
	text := `
# 注释行
https://static.hetaoimg.com/crmFiles/abc123.mp4
GESP 一级真题第 1 题 | https://static.hetaoimg.com/crmFiles/def456.mp4
GESP 一级真题第 2 题 | GESP,一级,选择题 | https://static.hetaoimg.com/crmFiles/ghi789.mp4 | 讲解二分查找
CSP-J 初赛第 3 题 | CSP | https://static.hetaoimg.com/crmFiles/jkl012.mp4
这一行没有链接
`
	items := ParseBatchLines(text)
	if len(items) != 5 {
		t.Fatalf("期望解析出 5 条，实际 %d 条", len(items))
	}
	if items[0].Title != "abc123" || items[0].URL != "https://static.hetaoimg.com/crmFiles/abc123.mp4" {
		t.Errorf("纯链接行解析错误: %+v", items[0])
	}
	if items[1].Title != "GESP 一级真题第 1 题" {
		t.Errorf("标题解析错误: %+v", items[1])
	}
	if items[2].Tags != "GESP,一级,选择题" || items[2].Description != "讲解二分查找" {
		t.Errorf("标签/备注解析错误: %+v", items[2])
	}
	if items[3].URL == "" || items[3].Error != "" {
		t.Errorf("链接位置解析错误: %+v", items[3])
	}
	if items[4].Error == "" {
		t.Errorf("缺少链接的行应报错: %+v", items[4])
	}
}

func TestTitleFromURL(t *testing.T) {
	cases := map[string]string{
		"https://static.hetaoimg.com/crmFiles/b3fdb68074174e58b2408b20f4185281.mp4": "b3fdb68074174e58b2408b20f4185281",
		"https://example.com/video/demo.mp4?x=1":                                    "demo",
		"https://example.com/":                                                      "未命名视频",
	}
	for in, want := range cases {
		if got := titleFromURL(in); got != want {
			t.Errorf("titleFromURL(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestValidateVideoURL(t *testing.T) {
	if err := ValidateVideoURL("https://static.hetaoimg.com/a.mp4"); err != nil {
		t.Errorf("合法链接被拒绝: %v", err)
	}
	for _, bad := range []string{"", "ftp://x/a.mp4", "static.hetaoimg.com/a.mp4", "javascript:alert(1)"} {
		if err := ValidateVideoURL(bad); err == nil {
			t.Errorf("非法链接 %q 应被拒绝", bad)
		}
	}
}
