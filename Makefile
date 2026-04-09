.PHONY: dev build run docker-up docker-down clean

# 本地开发
dev:
	go run ./cmd/server/

# 编译
build:
	go build -o dist/recalla-server ./cmd/server/

# 交叉编译 Linux
build-linux:
	GOOS=linux GOARCH=amd64 go build -o dist/recalla-server ./cmd/server/

# Docker 启动
docker-up:
	docker compose up -d

# Docker 停止
docker-down:
	docker compose down

# Docker 重建
docker-rebuild:
	docker compose up -d --build

# 清理
clean:
	rm -rf dist/
