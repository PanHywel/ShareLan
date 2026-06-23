#!/bin/bash
set -e

echo "🔨 构建 ShareLan..."

# 1. 构建前端
echo "📦 构建前端..."
cd "$(dirname "$0")/frontend"
npm run build

# 2. 复制前端产物到后端
echo "📋 复制前端产物..."
rm -rf ../backend/dist
cp -r dist ../backend/dist

# 3. 编译 Go 后端
echo "⚙️  编译 Go 后端..."
cd ../backend
CGO_ENABLED=1 go build -o sharelan .

# 同时编译 arm64 版本（Apple Silicon 兼容）
CGO_ENABLED=1 GOARCH=arm64 go build -o sharelan-darwin-arm64 . || true

echo "✅ 构建完成: backend/sharelan"

# 4. macOS 打包
if [ "$(uname)" = "Darwin" ]; then
  echo "📦 打包 macOS .app..."
  bash ../scripts/package-macos-app.sh
fi
