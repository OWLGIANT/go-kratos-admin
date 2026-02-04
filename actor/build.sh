#!/bin/bash

# Actor 构建脚本
# 用于编译 Go 二进制文件

set -e

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 项目信息
APP_NAME="actor"
VERSION=${VERSION:-"1.0.0"}
BUILD_TIME=$(date +"%Y-%m-%d %H:%M:%S")
GIT_COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")

# 构建目标
GOOS=${GOOS:-"linux"}
GOARCH=${GOARCH:-"amd64"}

echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}  Building ${APP_NAME}${NC}"
echo -e "${GREEN}  Version: ${VERSION}${NC}"
echo -e "${GREEN}  Commit: ${GIT_COMMIT}${NC}"
echo -e "${GREEN}  OS/Arch: ${GOOS}/${GOARCH}${NC}"
echo -e "${GREEN}========================================${NC}"

# 清理旧的构建产物
echo -e "${YELLOW}Cleaning old build...${NC}"
rm -f ${APP_NAME}

# 设置 Go 环境变量
export CGO_ENABLED=0
export GOOS=${GOOS}
export GOARCH=${GOARCH}

# 构建参数
LDFLAGS="-s -w"
LDFLAGS="${LDFLAGS} -X 'actor.GitCommitHash=${GIT_COMMIT}'"
LDFLAGS="${LDFLAGS} -X 'actor.AppName=${APP_NAME}'"
LDFLAGS="${LDFLAGS} -X 'actor.BuildTime=${BUILD_TIME}'"

# 编译
echo -e "${YELLOW}Building binary...${NC}"
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags "${LDFLAGS}" -o ${APP_NAME} ./cmd/server

# 检查构建结果
if [ -f "${APP_NAME}" ]; then
    echo -e "${GREEN}Build successful!${NC}"
    echo -e "${GREEN}Binary: $(pwd)/${APP_NAME}${NC}"
    ls -lh ${APP_NAME}
else
    echo -e "${RED}Build failed!${NC}"
    exit 1
fi

echo ""
echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}  Build completed successfully!${NC}"
echo -e "${GREEN}========================================${NC}"
echo ""
echo -e "To build Docker image, run:"
echo -e "  ${YELLOW}docker build -t ${APP_NAME}:${VERSION} .${NC}"
echo ""
