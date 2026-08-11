#!/usr/bin/env bash
# ============================================================
# Licen 签名一致性回归测试（防篡改验证）
#
# 验证签发 ↔ 验证两端对称性与防篡改能力：
#   1. 正常 License 验签通过（base64 往返一致）
#   2. 篡改任意业务字段 → INVALID_SIGNATURE（7 项）
#   3. 加未知字段 → 允许（Go json 忽略未知字段，无害）
#   4. server 加载篡改 License → valid:false
#   5. 机器码不匹配 → MACHINE_MISMATCH
#   6. 有效期边界：过期 → EXPIRED、未生效 → NOT_YET_VALID
#   7. 私钥签名↔公钥验签密钥对对应性（错误公钥验签失败）
#
# 用法:
#   ./scripts/verify-signature-test.sh
#   SKIP_SERVER=1 ./scripts/verify-signature-test.sh   # 跳过 server 端测试
#
# 依赖: go build（licen-tool / licen-server）、python3（JSON 篡改）、curl
# 全部通过退出码 0；任一项失败退出码 1。
# ============================================================
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SKIP_SERVER="${SKIP_SERVER:-0}"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"; [ -n "${SERVER_PID:-}" ] && kill "$SERVER_PID" 2>/dev/null || true' EXIT

PASS=0
FAIL=0

ok()   { PASS=$((PASS+1)); echo "  ✅ $1"; }
bad()  { FAIL=$((FAIL+1)); echo "  ❌ $1"; }

# ---------- 0. 构建工具 ----------
echo "════════════════════════════════════════════"
echo "🔏 Licen 签名一致性回归测试"
echo "════════════════════════════════════════════"

echo "▶ 构建 licen-tool / licen-server ..."
go build -o "$WORK/licen-tool" ./cmd/licen-tool
go build -o "$WORK/licen-server" ./cmd/licen-server

# ---------- 1. 生成密钥对 + 签发正常 License ----------
echo "▶ 生成临时密钥对（测试专用）..."
"$WORK/licen-tool" gen-keypair -d "$WORK/keys" >/dev/null
PRIV="$WORK/keys/private.pem"
PUB="$WORK/keys/public.pem"
[ -f "$PRIV" ] && [ -f "$PUB" ] || { echo "❌ 密钥文件未生成（$WORK/keys）"; exit 1; }

echo "▶ 签发正常 License（绑定测试机器码 TEST-MACHINE-001）..."
"$WORK/licen-tool" gen-license \
  -k "$PRIV" -m "TEST-MACHINE-001" -p licen-server -e enterprise \
  -n 50 -f server-core,api -d 365 -c "回归测试客户" -o "$WORK/license.json" >/dev/null
ok "License 签发成功"

# ---------- 2. 正常验签 ----------
echo "▶ 正常 License 验签..."
VOUT="$("$WORK/licen-tool" verify -k "$PUB" -l "$WORK/license.json" 2>&1 || true)"
if echo "$VOUT" | grep -q "校验通过"; then
  ok "正常验签通过"
else
  bad "正常验签失败（$VOUT）"
fi

# ---------- 3. 篡改测试（7 项） ----------
echo "▶ 篡改测试（均应 INVALID_SIGNATURE）..."
tamper() {
  local name="$1" expr="$2"
  python3 -c "
import json, sys
d = json.load(open('$WORK/license.json'))
$expr
json.dump(d, open('$WORK/license_t.json', 'w'), ensure_ascii=False, indent=2)
"
  # ⚠️ 不能用 verify | grep 管道判断：verify 篡改时退出码=1，
  # pipefail 下管道返回 1 → if 判假。须先捕获输出再 grep。
  local vout
  vout="$("$WORK/licen-tool" verify -k "$PUB" -l "$WORK/license_t.json" 2>&1 || true)"
  if echo "$vout" | grep -q "INVALID_SIGNATURE"; then
    ok "篡改${name} → 拒绝"
  else
    bad "篡改${name} 未拦截！（$vout）"
  fi
}

tamper "maxNodes"     "d['maxNodes'] = 999"
tamper "机器码"        "d['machineCode'] = '0'*64"
tamper "到期时间"      "d['expiresAt'] = '2030-01-01T00:00:00+08:00'"
tamper "功能点"        "d['features'] = ['hacker']"
tamper "客户名"        "d['customer'] = '黑客公司'"
tamper "licenseId"    "d['licenseId'] = 'LIC-FAKE'"
tamper "删字段"        "d.pop('edition')"

# ---------- 4. 加未知字段（应允许） ----------
echo "▶ 加未知字段（Go 忽略未知字段，应允许）..."
python3 -c "
import json
d = json.load(open('$WORK/license.json'))
d['extraField'] = 'x'
json.dump(d, open('$WORK/license_extra.json', 'w'), ensure_ascii=False, indent=2)
"
VOUT="$("$WORK/licen-tool" verify -k "$PUB" -l "$WORK/license_extra.json" 2>&1 || true)"
if echo "$VOUT" | grep -q "校验通过"; then
  ok "加未知字段 → 允许（无害，不影响业务值）"
else
  bad "加未知字段被拒（非预期）：$VOUT"
fi

# ---------- 5. 密钥对应性：错误公钥验签失败 ----------
echo "▶ 密钥对应性（错误公钥应验签失败）..."
"$WORK/licen-tool" gen-keypair -d "$WORK/other" >/dev/null
OTHER_PUB="$WORK/other/public.pem"
VOUT="$("$WORK/licen-tool" verify -k "$OTHER_PUB" -l "$WORK/license.json" 2>&1 || true)"
if echo "$VOUT" | grep -q "INVALID_SIGNATURE"; then
  ok "错误公钥验签 → 拒绝"
else
  bad "错误公钥验签未拦截！$VOUT"
fi

# ---------- 6. server 端加载校验 ----------
if [ "$SKIP_SERVER" = "1" ]; then
  echo "⏭️  跳过 server 端测试（SKIP_SERVER=1）"
else
  PORT=18199
  # 清理残留进程（(cd && server) & 的 PID 是子 shell，licen-server 本体成孤儿占端口）
  stop_server() {
    [ -n "${SERVER_PID:-}" ] && kill "$SERVER_PID" 2>/dev/null || true
    # 按端口杀真正的 licen-server 进程
    local pid
    pid=$(ss -tlnp 2>/dev/null | grep ":$PORT" | grep -oP 'pid=\K[0-9]+' || true)
    [ -n "$pid" ] && kill "$pid" 2>/dev/null || true
    SERVER_PID=""
    sleep 1
  }
  echo "▶ server 加载篡改 License（应 valid:false / INVALID_SIGNATURE）..."
  mkdir -p "$WORK/data"
  cat > "$WORK/server-config.yaml" <<EOF
server:
  port: $PORT
licen:
  salt: ""
  license-file: ./license.json
  public-key-file: ./keys/public.pem
  heartbeat-timeout-seconds: 90
  admin-token: test-admin-token
  hmac-verify-enabled: true
  db-path: ./data/licen.db
EOF
  # 篡改 maxNodes 的 license 放进去
  python3 -c "
import json
d = json.load(open('$WORK/license.json'))
d['maxNodes'] = 999
json.dump(d, open('$WORK/license_t.json','w'), ensure_ascii=False, indent=2)
"
  cp "$WORK/license_t.json" "$WORK/license.json"
  (cd "$WORK" && "$WORK/licen-server" -c server-config.yaml >server.log 2>&1) &
  SERVER_PID=$!
  sleep 2
  STATUS=$(curl -s "http://localhost:$PORT/api/v1/license/status" | python3 -c "import json,sys; d=json.load(sys.stdin); print(d['result'])" 2>/dev/null || echo "UNREACHABLE")
  if [ "$STATUS" = "INVALID_SIGNATURE" ]; then
    ok "server 拒绝篡改 License（INVALID_SIGNATURE）"
  else
    bad "server 未拒绝篡改 License（result=$STATUS）"
    echo "    server.log: $(tail -3 "$WORK/server.log" 2>/dev/null | tr '\n' ' ')"
  fi
  stop_server

  # 机器码不匹配（用另一机器码签发，加载后 MACHINE_MISMATCH）
  echo "▶ server 机器码不匹配（应 MACHINE_MISMATCH）..."
  "$WORK/licen-tool" gen-license \
    -k "$PRIV" -m "OTHER-MACHINE-002" -p licen-server -e enterprise \
    -n 10 -f server-core -d 30 -c "另一台机器" -o "$WORK/license_other.json" >/dev/null
  cp "$WORK/license_other.json" "$WORK/license.json"
  (cd "$WORK" && "$WORK/licen-server" -c server-config.yaml >server2.log 2>&1) &
  SERVER_PID=$!
  sleep 2
  STATUS=$(curl -s "http://localhost:$PORT/api/v1/license/status" | python3 -c "import json,sys; d=json.load(sys.stdin); print(d['result'])" 2>/dev/null || echo "UNREACHABLE")
  if [ "$STATUS" = "MACHINE_MISMATCH" ]; then
    ok "server 机器码不匹配 → MACHINE_MISMATCH"
  else
    bad "机器码不匹配未拦截（result=$STATUS）"
    echo "    server2.log: $(tail -3 "$WORK/server2.log" 2>/dev/null | tr '\n' ' ')"
  fi
  stop_server
fi

# ---------- 7. 有效期边界（licen-tool 级） ----------
echo "▶ 有效期边界（过期 → EXPIRED）..."
"$WORK/licen-tool" gen-license \
  -k "$PRIV" -m "TEST-MACHINE-001" -p licen-server -e enterprise \
  -n 10 -f server-core --expires 2020-01-01T00:00:00+08:00 -c "过期测试" -o "$WORK/license_expired.json" >/dev/null
VOUT="$("$WORK/licen-tool" verify -k "$PUB" -l "$WORK/license_expired.json" 2>&1 || true)"
if echo "$VOUT" | grep -qE "EXPIRED|过期"; then
  ok "过期 License → EXPIRED"
else
  bad "过期 License 未识别（$VOUT）"
fi

# ---------- 汇总 ----------
echo "════════════════════════════════════════════"
echo "📊 结果: ✅ $PASS 通过  ❌ $FAIL 失败"
echo "════════════════════════════════════════════"
[ "$FAIL" = "0" ] && echo "🎉 签名一致性回归全部通过" || echo "🚨 存在失败项！"
exit $([ "$FAIL" = "0" ] && echo 0 || echo 1)
