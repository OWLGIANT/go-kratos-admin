#!/bin/bash
set -e

EXECUTE_NAME="go-wind-admin"

# 获取版本号
VERSION=$(git rev-parse --short HEAD 2>/dev/null || echo "dev")
echo "构建版本: $VERSION"

# 编译
echo "编译二进制文件..."
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build \
    -ldflags "-w -s -X main.version=${VERSION}" \
    -o $EXECUTE_NAME \
    ./app/admin/service/cmd/server/

echo "构建完成: $EXECUTE_NAME"
