# StudyVideo 构建与运维入口
#
# 常用命令：
#   make build            本地构建（前端 + 后端，产物 bin/studyvideo）
#   make test             运行全部单元/集成测试
#   make release          交叉编译多平台发布包到 dist/
#   make version          查看当前版本信息

SHELL := /bin/bash

# ---------- 版本信息（可由 CI 覆盖）----------
VERSION    ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT     ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
BUILD_TIME ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

LDFLAGS := -s -w \
  -X studyvideo/internal/version.Version=$(VERSION) \
  -X studyvideo/internal/version.Commit=$(COMMIT) \
  -X studyvideo/internal/version.BuildTime=$(BUILD_TIME)

# ---------- 路径与工具 ----------
BACKEND_DIR  := backend
FRONTEND_DIR := frontend
BIN          := bin/studyvideo
DIST         := dist
NPM_REGISTRY ?= https://registry.npmmirror.com
TEST_DB_DSN  ?=

# 数据库相关目标（test）使用该 DSN；未设置则跳过集成测试
export TEST_DB_DSN

# ---------- 元信息 ----------
.PHONY: help
help: ## 显示所有可用命令
	@echo "StudyVideo Makefile"
	@echo ""
	@grep -hE '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
	  awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}'
	@echo ""
	@echo "当前版本: $(VERSION) (commit $(COMMIT))"

.PHONY: version
version: ## 打印版本信息
	@echo "Version:    $(VERSION)"
	@echo "Commit:     $(COMMIT)"
	@echo "BuildTime:  $(BUILD_TIME)"

# ---------- 依赖 ----------
.PHONY: deps
deps: ## 安装前端依赖
	cd $(FRONTEND_DIR) && npm install --registry=$(NPM_REGISTRY) --no-audit --no-fund

# ---------- 构建 ----------
.PHONY: frontend
frontend: ## 构建前端（输出到 backend/internal/web/dist）
	cd $(FRONTEND_DIR) && npm run build

.PHONY: backend
backend: ## 编译后端（自动包含已构建的前端产物）
	cd $(BACKEND_DIR) && go build -trimpath -ldflags "$(LDFLAGS)" -o ../$(BIN) ./cmd/studyvideo

.PHONY: build
build: frontend backend ## 完整构建（前端 + 后端）

.PHONY: release
release: ## 交叉编译多平台发布包（linux/darwin, amd64/arm64）到 dist/
	@mkdir -p $(DIST)
	@set -e; for target in linux/amd64 linux/arm64 darwin/amd64 darwin/arm64; do \
	  os=$${target%/*}; arch=$${target#*/}; \
	  name=studyvideo_$(VERSION)_$${os}_$${arch}; \
	  echo ">> 构建 $$name"; \
	  (cd $(BACKEND_DIR) && CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch \
	    go build -trimpath -tags netgo -ldflags "$(LDFLAGS)" -o ../$(DIST)/$$name/studyvideo ./cmd/studyvideo); \
	  cp README.md .env.example $(DIST)/$$name/ 2>/dev/null || true; \
	  (cd $(DIST) && tar czf $$name.tar.gz $$name && rm -rf $$name); \
	done
	@cd $(DIST) && sha256sum *.tar.gz > checksums.txt && cat checksums.txt
	@echo "发布包已生成于 $(DIST)/（二进制已内嵌前端资源）"

.PHONY: clean
clean: ## 清理构建产物
	rm -rf $(BIN) $(DIST)
	rm -rf $(BACKEND_DIR)/internal/web/dist/assets

# ---------- 质量 ----------
.PHONY: test
test: ## 运行测试（设置 TEST_DB_DSN 时包含 MySQL 集成测试）
	cd $(BACKEND_DIR) && go test ./...

.PHONY: test-race
test-race: ## 带竞态检测的测试
	cd $(BACKEND_DIR) && go test -race ./...

.PHONY: cover
cover: ## 生成测试覆盖率报告 coverage.out
	cd $(BACKEND_DIR) && go test -coverprofile=../coverage.out ./...

.PHONY: fmt
fmt: ## 格式化 Go 代码
	cd $(BACKEND_DIR) && gofmt -w .

.PHONY: vet
vet: ## go vet 静态检查
	cd $(BACKEND_DIR) && go vet ./...

.PHONY: lint
lint: fmt vet ## 格式化 + 静态检查

.PHONY: check
check: lint test ## CI 等价检查（lint + test）

# ---------- 运行 ----------
.PHONY: run
run: ## 本地运行（读取 .env，需先 make build）
	./run.sh
