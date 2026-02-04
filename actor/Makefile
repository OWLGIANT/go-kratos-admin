.PHONY: all build run proto test clean docker-up docker-down docker-logs help

# ========================================
# 项目配置
# ========================================
APP_NAME := actor
VERSION := last
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME := $(shell date +"%Y-%m-%d_%H:%M:%S")

# Docker 配置
IMAGE_NAME := wwttxx2/$(APP_NAME)
IMAGE_TAG := $(VERSION)

# 构建配置
GOOS ?= linux
GOARCH ?= amd64
export GOROOT := /usr/local/go
export PATH := $(GOROOT)/bin:$(PATH)

# 旧变量 (兼容)
BINARY_DIR=bin
PROTO_DIR=protos
PROTO_OUTPUT_DIR=pkg/proto
GO=go
PROTOC=protoc
#export https_proxy=http://127.0.0.1:33210 http_proxy=http://127.0.0.1:33210 all_proxy=socks5://127.0.0.1:33211
# 颜色
GREEN := \033[0;32m
YELLOW := \033[1;33m
RED := \033[0;31m
NC := \033[0m

# ========================================
# 帮助信息
# ========================================
help:
	@echo ""
	@echo "$(GREEN)Actor 构建工具$(NC)"
	@echo ""
	@echo "$(YELLOW)Docker 相关命令:$(NC)"
	@echo "  $(GREEN)make docker-build$(NC)    - 编译并构建 Docker 镜像"
	@echo "  $(GREEN)make docker-push$(NC)     - 推送 Docker 镜像到仓库"
	@echo "  $(GREEN)make deploy$(NC)          - 一键部署 (编译 + 构建 + 推送)"
	@echo "  $(GREEN)make docker-run$(NC)      - 本地运行 Docker 容器"
	@echo "  $(GREEN)make docker-stop$(NC)     - 停止本地 Docker 容器"
	@echo ""
	@echo "$(YELLOW)开发相关命令:$(NC)"
	@echo "  $(GREEN)make build-bin$(NC)       - 编译 Go 二进制文件"
	@echo "  $(GREEN)make build-mac$(NC)       - 编译 macOS 二进制文件"
	@echo "  $(GREEN)make test$(NC)            - 运行测试"
	@echo "  $(GREEN)make clean$(NC)           - 清理构建产物"
	@echo ""
	@echo "$(YELLOW)环境变量:$(NC)"
	@echo "  REGISTRY         - Docker 仓库地址 (例: docker.io/username)"
	@echo "  VERSION          - 版本号 (默认: $(VERSION))"
	@echo "  BACKEND_WS_URL   - Backend WebSocket 地址"
	@echo "  BACKEND_WS_TOKEN - Backend 认证 Token"
	@echo ""
	@echo "$(YELLOW)示例:$(NC)"
	@echo "  make deploy REGISTRY=docker.io/myuser"
	@echo "  make docker-run BACKEND_WS_URL=ws://43.134.189.161:7790/ws"
	@echo ""

# ========================================
# Docker 镜像构建和推送
# ========================================

# 编译二进制文件 (Linux)
build-bin:
	@chmod +x ./build.sh && ./build.sh

# 构建 Docker 镜像
docker-build: build-bin
	@echo ""
	@echo "$(GREEN)========================================$(NC)"
	@echo "$(GREEN)  构建 Docker 镜像$(NC)"
	@echo "$(GREEN)  镜像: $(IMAGE_NAME):$(IMAGE_TAG)$(NC)"
	@echo "$(GREEN)========================================$(NC)"
	@docker build --platform linux/amd64 -t $(IMAGE_NAME):$(IMAGE_TAG) .
	@docker tag $(IMAGE_NAME):$(IMAGE_TAG) $(IMAGE_NAME):latest
	@echo "$(GREEN)镜像构建完成!$(NC)"
	@docker images | grep $(IMAGE_NAME)

# 仅构建 Docker 镜像 (不重新编译)
docker-build-only:
	@echo "$(GREEN)构建 Docker 镜像: $(IMAGE_NAME):$(IMAGE_TAG)$(NC)"
	@docker build -t $(IMAGE_NAME):$(IMAGE_TAG) .
	@docker tag $(IMAGE_NAME):$(IMAGE_TAG) $(IMAGE_NAME):latest
	@echo "$(GREEN)镜像构建完成!$(NC)"

# 推送 Docker 镜像
docker-push:
	@echo "$(GREEN)推送镜像到: $(IMAGE_NAME):$(IMAGE_TAG)$(NC)"
	@docker push $(IMAGE_NAME):$(IMAGE_TAG)
	@docker push $(IMAGE_NAME):latest
	@echo "$(GREEN)推送完成!$(NC)"

# 一键部署: 编译 + 构建镜像 + 推送
deploy: docker-build docker-push
	@echo ""
	@echo "$(GREEN)========================================$(NC)"
	@echo "$(GREEN)  部署完成!$(NC)"
	@echo "$(GREEN)  镜像: $(IMAGE_NAME):$(IMAGE_TAG)$(NC)"
	@echo "$(GREEN)========================================$(NC)"

# 本地运行 Docker 容器
docker-run:
	@echo "$(GREEN)启动 $(APP_NAME) 容器...$(NC)"
	@docker run -d --name $(APP_NAME) \
		-e BACKEND_WS_URL="$(BACKEND_WS_URL)" \
		-e BACKEND_WS_TOKEN="$(BACKEND_WS_TOKEN)" \
		-p 8080:8080 \
		$(IMAGE_NAME):latest
	@echo "$(GREEN)容器已启动!$(NC)"
	@docker ps | grep $(APP_NAME)

# 停止 Docker 容器
docker-stop:
	@echo "$(YELLOW)停止 $(APP_NAME) 容器...$(NC)"
	@docker stop $(APP_NAME) 2>/dev/null || true
	@docker rm $(APP_NAME) 2>/dev/null || true
	@echo "$(GREEN)容器已停止$(NC)"

# 查看容器日志
docker-logs:
	@docker logs -f $(APP_NAME)

# ========================================
# 旧命令 (兼容)
# ========================================

all: proto build

# Generate protobuf files
proto:
	@echo "Generating protobuf files..."
	@mkdir -p $(PROTO_OUTPUT_DIR)
	$(PROTOC) --proto_path=$(PROTO_DIR) \
		--go_out=$(PROTO_OUTPUT_DIR) \
		--go_opt=paths=source_relative \
		--go-grpc_out=$(PROTO_OUTPUT_DIR) \
		--go-grpc_opt=paths=source_relative \
		$(PROTO_DIR)/*.proto 2>/dev/null || true

# Build all services
build: proto
	@echo "Building services..."
	@mkdir -p $(BINARY_DIR)
	$(GO) build -o $(BINARY_DIR)/actor-service ./cmd/server

# Run actor service
run-actor:
	$(GO) run ./cmd/server/main.go

# Run tests
test:
	$(GO) test -v ./...

# Run tests with coverage
test-coverage:
	$(GO) test -v -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out -o coverage.html

# Clean build artifacts
clean:
	@echo "$(YELLOW)清理构建产物...$(NC)"
	@rm -rf $(BINARY_DIR)
	@rm -f $(APP_NAME)
	@rm -f coverage.out coverage.html
	@docker rmi $(IMAGE_NAME):$(IMAGE_TAG) 2>/dev/null || true
	@docker rmi $(IMAGE_NAME):latest 2>/dev/null || true
	@echo "$(GREEN)清理完成$(NC)"

# Docker compose commands
docker-up:
	docker-compose up -d

docker-down:
	docker-compose down

# Install dependencies
deps:
	$(GO) mod download
	$(GO) mod tidy

# Lint
lint:
	golangci-lint run

# Format code
fmt:
	$(GO) fmt ./...

# 显示当前配置
info:
	@echo "$(GREEN)当前配置:$(NC)"
	@echo "  APP_NAME:    $(APP_NAME)"
	@echo "  VERSION:     $(VERSION)"
	@echo "  GIT_COMMIT:  $(GIT_COMMIT)"
	@echo "  IMAGE_NAME:  $(IMAGE_NAME)"
	@echo "  IMAGE_TAG:   $(IMAGE_TAG)"
	@echo "  REGISTRY:    $(REGISTRY)"
	@echo "  GOOS:        $(GOOS)"
	@echo "  GOARCH:      $(GOARCH)"

.DEFAULT_GOAL := help
