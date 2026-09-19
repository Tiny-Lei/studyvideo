// Package version 保存构建期注入的版本信息。
//
// 通过 -ldflags 注入，例如：
//
//	go build -ldflags "-X studyvideo/internal/version.Version=1.2.3 \
//	  -X studyvideo/internal/version.Commit=$(git rev-parse --short HEAD) \
//	  -X studyvideo/internal/version.BuildTime=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
package version

import "fmt"

var (
	// Version 语义化版本号，未注入时为 dev。
	Version = "dev"
	// Commit 构建对应的 git commit（短哈希）。
	Commit = "none"
	// BuildTime UTC 构建时间。
	BuildTime = "unknown"
)

// String 返回形如 "1.2.3 (commit abc1234, built 2026-01-02T15:04:05Z)" 的描述。
func String() string {
	return fmt.Sprintf("%s (commit %s, built %s)", Version, Commit, BuildTime)
}
