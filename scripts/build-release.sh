#!/usr/bin/env bash
# ============================================================
# Licen 加固构建脚本（P7）
# 产物：静态链接 + strip 符号 + garble 混淆（L1+L2 防逆向）
#
# 用法:
#   ./scripts/build-release.sh                # 构建当前平台
#   ./scripts/build-release.sh linux amd64    # 交叉构建指定平台
#   SKIP_GARBLE=1 ./scripts/build-release.sh  # 仅 L1（静态+strip），跳过 garble
#
# 产物输出到: dist/<version>/
# ============================================================
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

VERSION="${VERSION:-$(grep -m1 'v[0-9]' go.mod >/dev/null 2>&1 && echo "3.0.0" || echo "1.0.0")}"
if [ -z "${VERSION:-}" ]; then VERSION="1.0.0"; fi

TARGET_OS="${1:-$(go env GOOS)}"
TARGET_ARCH="${2:-$(go env GOARCH)}"
SKIP_GARBLE="${SKIP_GARBLE:-0}"

OUT_DIR="dist/${VERSION}/${TARGET_OS}-${TARGET_ARCH}"
mkdir -p "$OUT_DIR"

echo "════════════════════════════════════════════"
echo "🔒 Licen 加固构建  v${VERSION}"
echo "   平台: ${TARGET_OS}/${TARGET_ARCH}"
echo "   garble: $([ "$SKIP_GARBLE" = "1" ] && echo '跳过(L1 only)' || echo '启用(L1+L2)')"
echo "════════════════════════════════════════════"

# ---------- 检查工具 ----------
if ! command -v go >/dev/null; then echo "❌ go 未安装"; exit 1; fi
GARBLE=""
if [ "$SKIP_GARBLE" != "1" ]; then
  GARBLE="$(command -v garble || true)"
  if [ -z "$GARBLE" ]; then
    echo "⚠️  garble 未安装，尝试安装..."
    go install mvdan.cc/garble@latest
    GARBLE="$(command -v garble)"
  fi
fi

# ---------- 构建公共参数 ----------
# -s -w: 剥离符号表与 DWARF 调试信息（L1）
# -buildid=: 移除构建 ID
# -X main.version=: 注入版本号
LDFLAGS="-s -w -buildid= -X main.version=${VERSION}"
export CGO_ENABLED=0   # 静态链接（L1）
export GOOS="$TARGET_OS"
export GOARCH="$TARGET_ARCH"

build_one() {
  local pkg="$1" bin="$2" extra_ldflags="${3:-}"
  local out="${OUT_DIR}/${bin}"
  echo "▶ 构建 ${bin} ..."

  if [ -n "$GARBLE" ]; then
    # L2: garble 混淆
    #   -literals: 加密字符串字面量
    #   -tiny:     进一步压缩符号名（缩短混淆名）
    #   -seed=random: 每次构建混淆结果不同（防比对）
    # 注意: garble flags 在 build 之前，go build flags 在 build 之后
    CGO_ENABLED=0 GOOS="$GOOS" GOARCH="$GOARCH" \
      "$GARBLE" -literals -tiny -seed=random build \
      -ldflags "${LDFLAGS}${extra_ldflags}" \
      -o "$out" "$pkg"
  else
    go build -trimpath -ldflags "${LDFLAGS}${extra_ldflags}" -o "$out" "$pkg"
  fi

  # 二次 strip（保险，garble 已含 -s -w 时此步几乎无操作）
  strip --strip-unneeded "$out" 2>/dev/null || true
  echo "   ✅ $(ls -lh "$out" | awk '{print $5}')  $(file -b "$out" | cut -d, -f1-2)"
}

build_one ./cmd/licen-server licen-server
build_one ./cmd/licen-tool   licen-tool

# ---------- 拷贝配置模板 ----------
cp cmd/licen-server/config.yaml "$OUT_DIR/config.yaml.example" 2>/dev/null || true
cp docs/protocol.md "$OUT_DIR/PROTOCOL.md" 2>/dev/null || true

# ---------- 校验 ----------
echo "────────────────────────────────────────────"
echo "🔍 加固校验:"
for b in licen-server licen-tool; do
  f="$OUT_DIR/$b"
  [ -f "$f" ] || { echo "❌ $b 缺失"; exit 1; }
  # 1. 静态链接校验
  if file "$f" | grep -q "statically linked"; then
    echo "   ✅ $b 静态链接"
  else
    echo "   ⚠️  $b 非纯静态（可能仍依赖 glibc）"
  fi
  # 2. 符号表校验
  if nm "$f" 2>&1 | grep -q "no symbols"; then
    echo "   ✅ $b 符号已剥离"
  else
    echo "   ⚠️  $b 仍有符号: $(nm "$f" 2>/dev/null | wc -l) 个"
  fi
done

echo "────────────────────────────────────────────"
echo "📦 产物目录: $OUT_DIR"
ls -lh "$OUT_DIR"
echo "✅ 加固构建完成"
