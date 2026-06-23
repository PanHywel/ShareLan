#!/bin/bash
# 将 ShareLan 编译为 macOS .app 包（双击运行，无终端窗口）
set -e

cd "$(dirname "$0")/.."
APP_NAME="ShareLan.app"
BINARY="backend/sharelan"

if [ ! -f "$BINARY" ]; then
  echo "❌ 未找到 $BINARY，请先运行 build.sh"
  exit 1
fi

echo "📦 打包 $APP_NAME ..."

# 创建 .app 目录结构
rm -rf "$APP_NAME"
mkdir -p "$APP_NAME/Contents/MacOS"

# 复制二进制
cp "$BINARY" "$APP_NAME/Contents/MacOS/sharelan"

# 创建 Info.plist
cat > "$APP_NAME/Contents/Info.plist" << EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>CFBundleExecutable</key>
  <string>sharelan</string>
  <key>CFBundleIdentifier</key>
  <string>com.sharelan.app</string>
  <key>CFBundleName</key>
  <string>ShareLan</string>
  <key>CFBundleVersion</key>
  <string>0.1.0</string>
  <key>CFBundlePackageType</key>
  <string>APPL</string>
  <key>LSMinimumSystemVersion</key>
  <string>10.15</string>
  <key>NSHighResolutionCapable</key>
  <true/>
</dict>
</plist>
EOF

# 设置可执行权限
chmod +x "$APP_NAME/Contents/MacOS/sharelan"

echo "✅ 打包完成: $APP_NAME"
echo "双击 $APP_NAME 即可运行（无终端窗口）"
