NPM_REGISTRY ?= https://registry.npmmirror.com
BIN := bin/studyvideo

.PHONY: all build backend frontend deps run test fmt vet clean

all: build

# 安装前端依赖（仅首次构建需要）
deps:
	cd frontend && npm install --registry=$(NPM_REGISTRY) --no-audit --no-fund

# 构建前端，产物输出到 backend/internal/web/dist（由 Go embed 内嵌）
frontend:
	cd frontend && npm run build

# 编译后端（自动包含前端产物）
backend:
	cd backend && go build -o ../$(BIN) ./cmd/studyvideo

build: frontend backend

run:
	./run.sh

test:
	cd backend && go test ./...

fmt:
	cd backend && gofmt -w .

vet:
	cd backend && go vet ./...

clean:
	rm -rf bin
