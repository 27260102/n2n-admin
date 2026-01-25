#!/bin/bash

# n2n-admin 一键构建脚本
# --------------------------------------------------
# 此脚本将自动构建前端，并将其嵌入到 Go 后端二进制文件中。

set -e # 遇到错误立即停止

echo "======================================="
echo "       n2n-admin Build System"
echo "======================================="

BASE_DIR=$(cd "$(dirname "$0")"; pwd)

# 1. 构建前端
echo "[1/2] Building Frontend (React)..."
cd "$BASE_DIR/frontend"
npm install
npm run build

# 2. 构建后端
echo "[2/2] Building Backend (Go)..."
cd "$BASE_DIR/backend"
go mod tidy
go build -ldflags="-s -w" -o n2n_admin main.go

echo ""
echo "======================================="
echo "Build Successful!"
echo "Binary location: $BASE_DIR/backend/n2n_admin"
echo ""
echo "To start the service:"
echo "  cd backend && ./n2n_admin"
echo "======================================="