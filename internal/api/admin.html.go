// adminHTML licen-server 内置 Web 管理平台（零外部依赖，内嵌单文件）。
// 功能：授权状态总览 / License 热重载 / 节点管理 / Apps 管理 / 审计日志。
// 鉴权：首次使用时输入管理 Token（X-Admin-Token），保存在浏览器 localStorage。
package api

const adminHTML = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Licen 授权管理平台</title>
<style>
  * { margin: 0; padding: 0; box-sizing: border-box; }
  body { font-family: -apple-system, "PingFang SC", "Microsoft YaHei", sans-serif; background: #f0f2f5; color: #1f2329; }
  .wrap { max-width: 1080px; margin: 30px auto; padding: 0 20px; }
  .header { display: flex; align-items: center; gap: 12px; margin-bottom: 20px; }
  .logo { width: 44px; height: 44px; border-radius: 10px; background: linear-gradient(135deg, #2563eb, #7c3aed); display: flex; align-items: center; justify-content: center; color: #fff; font-size: 22px; font-weight: 700; }
  .header h1 { font-size: 22px; }
  .header p { color: #8a919f; font-size: 13px; margin-top: 2px; }
  .header .right { margin-left: auto; display: flex; align-items: center; gap: 10px; }
  .card { background: #fff; border-radius: 12px; padding: 22px; box-shadow: 0 1px 4px rgba(0,0,0,.06); margin-bottom: 16px; }
  .card h2 { font-size: 15px; margin-bottom: 16px; color: #333; border-left: 3px solid #2563eb; padding-left: 10px; }
  .badge { display: inline-block; padding: 3px 12px; border-radius: 20px; font-size: 13px; font-weight: 700; }
  .badge.ok { background: #e6f7ec; color: #168a2f; }
  .badge.bad { background: #fef3f3; color: #d92d20; }
  .badge.warn { background: #fff7e6; color: #b45309; }
  .stat-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(160px, 1fr)); gap: 14px; }
  .stat { background: #f8fafc; border: 1px solid #e8ebf0; border-radius: 10px; padding: 14px; }
  .stat .k { font-size: 12px; color: #8a919f; }
  .stat .v { font-size: 18px; font-weight: 700; margin-top: 6px; word-break: break-all; }
  .stat .v.small { font-size: 13px; }
  table { width: 100%; border-collapse: collapse; font-size: 13px; }
  th { text-align: left; padding: 10px 8px; color: #8a919f; font-weight: 600; border-bottom: 1px solid #e8ebf0; white-space: nowrap; }
  td { padding: 10px 8px; border-bottom: 1px solid #f1f2f4; vertical-align: middle; }
  tr:hover td { background: #f8fafc; }
  .dot { display: inline-block; width: 8px; height: 8px; border-radius: 50%; margin-right: 6px; }
  .dot.on { background: #22c55e; }
  .dot.off { background: #cbd5e1; }
  .btn { padding: 7px 16px; border: none; border-radius: 8px; font-size: 13px; cursor: pointer; font-weight: 600; }
  .btn-primary { background: #2563eb; color: #fff; }
  .btn-primary:hover { background: #1d4ed8; }
  .btn-ghost { background: #f1f2f4; color: #333; }
  .btn-ghost:hover { background: #e5e7eb; }
  .btn-danger { background: #fee2e2; color: #d92d20; }
  .btn-danger:hover { background: #fecaca; }
  .btn:disabled { opacity: .5; cursor: not-allowed; }
  .input { padding: 8px 12px; border: 1px solid #d5d9e0; border-radius: 8px; font-size: 13px; outline: none; width: 100%; }
  .input:focus { border-color: #2563eb; box-shadow: 0 0 0 3px rgba(37,99,235,.1); }
  .form-row { display: flex; gap: 10px; margin-bottom: 12px; flex-wrap: wrap; }
  .form-row .input { flex: 1; min-width: 140px; }
  .mono { font-family: ui-monospace, "SF Mono", Consolas, monospace; font-size: 12px; }
  .token-bar { display: flex; gap: 10px; align-items: center; background: #fff; border: 1px solid #e8ebf0; border-radius: 10px; padding: 10px 14px; margin-bottom: 16px; }
  .token-bar .input { max-width: 320px; }
  .muted { color: #8a919f; font-size: 12px; }
  .actions { display: flex; gap: 8px; }
  .refresh { margin-left: auto; }
  .toast { position: fixed; top: 20px; right: 20px; padding: 12px 20px; border-radius: 10px; color: #fff; font-size: 14px; display: none; z-index: 99; box-shadow: 0 4px 16px rgba(0,0,0,.15); }
  .toast.ok { background: #168a2f; }
  .toast.err { background: #d92d20; }
  pre { background: #f8fafc; border: 1px solid #e8ebf0; border-radius: 8px; padding: 10px; font-size: 12px; overflow-x: auto; max-height: 240px; }
  .section-title { display: flex; align-items: center; }
  .section-title .actions { margin-left: auto; }
</style>
</head>
<body>
<div class="wrap">
  <div class="header">
    <div class="logo">🔑</div>
    <div>
      <h1>Licen 授权管理平台</h1>
      <p>客户侧授权服务 · License 状态 / 节点 / 应用 / 审计 一站式管理</p>
    </div>
    <div class="right">
      <button class="btn btn-ghost" onclick="loadAll()">🔄 刷新</button>
    </div>
  </div>

  <div class="token-bar">
    <span>🔐 管理 Token：</span>
    <input class="input" id="token" type="password" placeholder="输入 X-Admin-Token（首次使用需配置）">
    <button class="btn btn-primary" onclick="saveToken()">保存</button>
    <span class="muted" id="tokenHint"></span>
  </div>

  <div class="card">
    <div class="section-title"><h2>📋 授权状态</h2><div class="actions"><button class="btn btn-ghost refresh" onclick="reloadLicense()">♻ 热重载 License</button></div></div>
    <div id="licStatus" class="stat-grid">
      <div class="stat"><div class="k">授权状态</div><div class="v" id="stValid">-</div></div>
      <div class="stat"><div class="k">机器码</div><div class="v small mono" id="stMachine">-</div></div>
      <div class="stat"><div class="k">客户</div><div class="v" id="stCustomer">-</div></div>
      <div class="stat"><div class="k">产品 / 版本</div><div class="v small" id="stProduct">-</div></div>
      <div class="stat"><div class="k">并发节点</div><div class="v" id="stNodes">-</div></div>
      <div class="stat"><div class="k">功能点</div><div class="v small" id="stFeatures">-</div></div>
      <div class="stat"><div class="k">签发 / 到期</div><div class="v small" id="stDates">-</div></div>
    </div>
  </div>

  <div class="card">
    <div class="section-title"><h2>🖥 节点管理（并发控制）</h2></div>
    <table>
      <thead><tr><th>状态</th><th>节点名称</th><th>NodeID</th><th>AppKey</th><th>IP</th><th>版本</th><th>注册时间</th><th>最后心跳</th><th>操作</th></tr></thead>
      <tbody id="nodeBody"><tr><td colspan="9" class="muted">加载中...</td></tr></tbody>
    </table>
  </div>

  <div class="card">
    <div class="section-title"><h2>📦 应用管理（AppKey/Secret）</h2><div class="actions"><button class="btn btn-primary" onclick="showCreateApp()">+ 新建应用</button></div></div>
    <div id="createAppForm" style="display:none; margin-bottom:14px; background:#f8fafc; border:1px solid #e8ebf0; border-radius:10px; padding:14px;">
      <div class="form-row">
        <input class="input" id="appName" placeholder="应用名称，如 AI推理节点">
        <input class="input" id="appProduct" placeholder="产品标识，如 licen-server">
        <input class="input" id="appKey" placeholder="AppKey（必填）">
        <input class="input" id="appSecret" placeholder="AppSecret（留空自动生成）">
        <button class="btn btn-primary" onclick="createApp()">创建</button>
        <button class="btn btn-ghost" onclick="hideCreateApp()">取消</button>
      </div>
    </div>
    <table>
      <thead><tr><th>ID</th><th>名称</th><th>产品</th><th>AppKey</th><th>AppSecret</th><th>状态</th><th>创建时间</th><th>操作</th></tr></thead>
      <tbody id="appBody"><tr><td colspan="8" class="muted">加载中...</td></tr></tbody>
    </table>
  </div>

  <div class="card">
    <div class="section-title"><h2>📜 审计日志</h2></div>
    <table>
      <thead><tr><th>时间</th><th>动作</th><th>详情</th></tr></thead>
      <tbody id="auditBody"><tr><td colspan="3" class="muted">加载中...</td></tr></tbody>
    </table>
  </div>
</div>

<div class="toast" id="toast"></div>

<script>
const LS_KEY = 'licenAdminToken';

function token() { return document.getElementById('token').value.trim() || localStorage.getItem(LS_KEY) || ''; }
function saveToken() {
  const t = document.getElementById('token').value.trim();
  if (t) { localStorage.setItem(LS_KEY, t); }
  document.getElementById('tokenHint').textContent = t ? '✅ Token 已保存（存于本机浏览器）' : '⚠️ 未设置 Token';
  loadAll();
}
window.addEventListener('DOMContentLoaded', () => {
  const t = localStorage.getItem(LS_KEY);
  if (t) { document.getElementById('token').value = t; document.getElementById('tokenHint').textContent = '✅ Token 已从本机读取'; }
  loadAll();
});

async function api(path, opts = {}) {
  const headers = Object.assign({ 'X-Admin-Token': token() }, opts.headers || {});
  const resp = await fetch(path, Object.assign({}, opts, { headers }));
  const data = await resp.json().catch(() => ({}));
  if (resp.status === 401) { toast('❌ 管理 Token 无效', 'err'); }
  return data;
}

async function loadAll() {
  loadLicense(); loadNodes(); loadApps(); loadAudits();
}

async function loadLicense() {
  const d = await api('/api/v1/admin/license/status');
  if (!d || d.license === undefined) { document.getElementById('stValid').innerHTML = '<span class="badge bad">未激活</span>'; return; }
  const lic = d.license || {};
  const valid = d.valid;
  document.getElementById('stValid').innerHTML = valid
    ? '<span class="badge ok">✅ VALID</span>' : '<span class="badge bad">' + (d.result || 'INVALID') + '</span>';
  document.getElementById('stMachine').textContent = d.machineCode || '-';
  document.getElementById('stCustomer').textContent = lic.customer || '-';
  document.getElementById('stProduct').textContent = (lic.product || '-') + ' / ' + (lic.edition || '-');
  document.getElementById('stNodes').textContent = (lic.maxNodes ?? '-') + ' 个';
  document.getElementById('stFeatures').textContent = (lic.features && lic.features.length ? lic.features.join(', ') : '（全部）');
  document.getElementById('stDates').textContent = fmtTime(lic.issuedAt) + ' → ' + fmtTime(lic.expiresAt);
}

async function loadNodes() {
  const nodes = await api('/api/v1/admin/nodes?size=100');
  const body = document.getElementById('nodeBody');
  if (!Array.isArray(nodes) || nodes.length === 0) { body.innerHTML = '<tr><td colspan="9" class="muted">暂无节点</td></tr>'; return; }
  body.innerHTML = nodes.map(n => {
    const on = n.status === 'ONLINE';
    return '<tr>' +
      '<td><span class="dot ' + (on ? 'on' : 'off') + '"></span>' + (on ? '在线' : '离线') + '</td>' +
      '<td>' + esc(n.nodeName || '-') + '</td>' +
      '<td class="mono">' + esc(n.nodeId || '-') + '</td>' +
      '<td class="mono">' + esc(n.appKey || '-') + '</td>' +
      '<td>' + esc(n.ip || '-') + '</td>' +
      '<td>' + esc(n.version || '-') + '</td>' +
      '<td>' + fmtTime(n.registeredAt) + '</td>' +
      '<td>' + fmtTime(n.lastHeartbeatAt) + '</td>' +
      '<td><button class="btn btn-danger" onclick="revokeNode(' + n.id + ')">吊销</button></td>' +
      '</tr>';
  }).join('');
}

async function loadApps() {
  const apps = await api('/api/v1/admin/apps');
  const body = document.getElementById('appBody');
  if (!Array.isArray(apps) || apps.length === 0) { body.innerHTML = '<tr><td colspan="8" class="muted">暂无应用</td></tr>'; return; }
  body.innerHTML = apps.map(a =>
    '<tr>' +
    '<td>' + a.id + '</td>' +
    '<td>' + esc(a.name || '-') + '</td>' +
    '<td>' + esc(a.product || '-') + '</td>' +
    '<td class="mono">' + esc(a.appKey || '-') + '</td>' +
    '<td class="mono">' + esc(a.appSecret || '-') + '</td>' +
    '<td>' + (a.enabled ? '<span class="badge ok">启用</span>' : '<span class="badge warn">停用</span>') + '</td>' +
    '<td>' + fmtTime(a.createdAt) + '</td>' +
    '<td><button class="btn btn-danger" onclick="deleteApp(' + a.id + ')">删除</button></td>' +
    '</tr>'
  ).join('');
}

async function loadAudits() {
  const audits = await api('/api/v1/admin/audits?size=100');
  const body = document.getElementById('auditBody');
  if (!Array.isArray(audits) || audits.length === 0) { body.innerHTML = '<tr><td colspan="3" class="muted">暂无审计日志</td></tr>'; return; }
  body.innerHTML = audits.map(a =>
    '<tr><td class="mono">' + fmtTime(a.time) + '</td><td><code>' + esc(a.action || '-') + '</code></td><td class="mono">' + esc(a.detail || '') + '</td></tr>'
  ).join('');
}

async function revokeNode(id) {
  if (!confirm('确认吊销该节点？它将立即失去访问资格。')) return;
  const d = await api('/api/v1/admin/nodes/' + id, { method: 'DELETE' });
  toast(d.success ? '✅ 节点已吊销' : '❌ 吊销失败', d.success ? 'ok' : 'err');
  loadNodes();
}

async function reloadLicense() {
  const d = await api('/api/v1/admin/license/reload', { method: 'POST' });
  toast(d.success ? '✅ License 已热重载：' + d.result : '❌ 热重载失败：' + (d.result || ''), d.success ? 'ok' : 'err');
  loadLicense();
}

function showCreateApp() { document.getElementById('createAppForm').style.display = 'block'; }
function hideCreateApp() { document.getElementById('createAppForm').style.display = 'none'; }

async function createApp() {
  const payload = {
    name: document.getElementById('appName').value.trim(),
    product: document.getElementById('appProduct').value.trim(),
    appKey: document.getElementById('appKey').value.trim(),
    appSecret: document.getElementById('appSecret').value.trim()
  };
  if (!payload.appKey) { toast('❌ AppKey 必填', 'err'); return; }
  const d = await api('/api/v1/admin/apps', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(payload) });
  toast(d.success ? '✅ 应用已创建' : '❌ 创建失败：' + (d.message || ''), d.success ? 'ok' : 'err');
  hideCreateApp();
  loadApps();
}

async function deleteApp(id) {
  if (!confirm('确认删除该应用？关联节点将无法注册。')) return;
  const d = await api('/api/v1/admin/apps/' + id, { method: 'DELETE' });
  toast(d.success ? '✅ 应用已删除' : '❌ 删除失败', d.success ? 'ok' : 'err');
  loadApps();
}

function fmtTime(t) {
  if (!t) return '-';
  const d = new Date(t);
  if (isNaN(d)) return String(t);
  const p = n => String(n).padStart(2, '0');
  return d.getFullYear() + '-' + p(d.getMonth()+1) + '-' + p(d.getDate()) + ' ' + p(d.getHours()) + ':' + p(d.getMinutes());
}

function esc(s) { return String(s ?? '').replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;').replace(/"/g,'&quot;'); }

function toast(msg, type) {
  const el = document.getElementById('toast');
  el.textContent = msg;
  el.className = 'toast ' + (type || 'ok');
  el.style.display = 'block';
  setTimeout(() => { el.style.display = 'none'; }, 2600);
}
</script>
</body>
</html>`
