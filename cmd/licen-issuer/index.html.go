// indexHTML 签发服务 Web 界面（零外部依赖，内嵌单文件）。
// 说明：客户把机器码发过来 → 厂商填表 → 生成 license.json → 下载回传客户。
package main

const indexHTML = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Licen 授权签发中心</title>
<style>
  * { margin: 0; padding: 0; box-sizing: border-box; }
  body { font-family: -apple-system, "PingFang SC", "Microsoft YaHei", sans-serif; background: #f0f2f5; color: #1f2329; }
  .wrap { max-width: 1100px; margin: 40px auto; padding: 0 20px; }
  .header { display: flex; align-items: center; gap: 12px; margin-bottom: 24px; }
  .logo { width: 44px; height: 44px; border-radius: 10px; background: linear-gradient(135deg, #2563eb, #7c3aed); display: flex; align-items: center; justify-content: center; color: #fff; font-size: 22px; font-weight: 700; }
  .header h1 { font-size: 22px; }
  .header p { color: #8a919f; font-size: 13px; margin-top: 2px; }
  .card { background: #fff; border-radius: 12px; padding: 24px; box-shadow: 0 1px 4px rgba(0,0,0,.06); margin-bottom: 16px; }
  .card h2 { font-size: 15px; margin-bottom: 16px; color: #333; border-left: 3px solid #2563eb; padding-left: 10px; }
  .grid { display: grid; grid-template-columns: 1fr 1fr; gap: 14px; }
  .field label { display: block; font-size: 13px; color: #555; margin-bottom: 5px; }
  .field input { width: 100%; padding: 9px 12px; border: 1px solid #d5d9e0; border-radius: 8px; font-size: 14px; outline: none; transition: border .2s; }
  .field input:focus { border-color: #2563eb; box-shadow: 0 0 0 3px rgba(37,99,235,.1); }
  .field.full { grid-column: 1 / -1; }
  .row { display: flex; gap: 12px; margin-top: 20px; }
  .btn { padding: 10px 28px; border: none; border-radius: 8px; font-size: 14px; cursor: pointer; font-weight: 600; }
  .btn-primary { background: #2563eb; color: #fff; }
  .btn-primary:hover { background: #1d4ed8; }
  .btn-ghost { background: #f1f2f4; color: #333; }
  .btn-ghost:hover { background: #e5e7eb; }
  .btn:disabled { opacity: .5; cursor: not-allowed; }
  .result { margin-top: 20px; display: none; }
  .result.ok { display: block; border: 1px solid #bbe7c0; background: #f3fbf4; border-radius: 10px; padding: 16px; }
  .result.err { display: block; border: 1px solid #f3c1c1; background: #fef3f3; border-radius: 10px; padding: 16px; }
  .result h3 { font-size: 14px; margin-bottom: 10px; }
  .result.ok h3 { color: #168a2f; }
  .result.err h3 { color: #d92d20; }
  pre { background: #0f172a; color: #e2e8f0; padding: 14px; border-radius: 8px; font-size: 12px; overflow-x: auto; max-height: 300px; line-height: 1.5; }
  .dl { display: inline-block; margin-top: 12px; padding: 8px 20px; background: #168a2f; color: #fff; border-radius: 8px; text-decoration: none; font-size: 13px; font-weight: 600; }
  .tip { font-size: 12px; color: #8a919f; margin-top: 16px; line-height: 1.8; }
  .tip code { background: #f1f2f4; padding: 2px 6px; border-radius: 4px; }
  /* 台账表格 */
  table { width: 100%; border-collapse: collapse; font-size: 13px; }
  th { text-align: left; padding: 10px 12px; background: #f8f9fb; color: #555; font-weight: 600; border-bottom: 2px solid #e5e7eb; white-space: nowrap; }
  td { padding: 10px 12px; border-bottom: 1px solid #f0f1f4; vertical-align: middle; }
  tr:hover td { background: #fafbfd; }
  .badge { display: inline-block; padding: 3px 10px; border-radius: 20px; font-size: 12px; font-weight: 600; }
  .badge.valid { background: #e6f7ec; color: #168a2f; }
  .badge.expiring { background: #fff7e6; color: #b76e00; }
  .badge.expired { background: #fef3e6; color: #b76e00; }
  .badge.revoked { background: #fdeeee; color: #d92d20; }
  /* 到期提醒条 */
  .alert { display: none; background: #fff7e6; border: 1px solid #f5d9a8; color: #b76e00; border-radius: 10px; padding: 12px 16px; margin-bottom: 16px; font-size: 13px; }
  .alert.show { display: flex; align-items: center; gap: 8px; }
  .alert .expired-alert { color: #d92d20; }
  /* 时间线弹窗 */
  .modal-mask { display: none; position: fixed; inset: 0; background: rgba(0,0,0,.45); z-index: 100; }
  .modal-mask.show { display: flex; align-items: flex-start; justify-content: center; padding: 60px 20px; }
  .modal { background: #fff; border-radius: 14px; width: 860px; max-width: 100%; max-height: 80vh; overflow-y: auto; padding: 24px; box-shadow: 0 8px 30px rgba(0,0,0,.18); }
  .modal h3 { font-size: 16px; margin-bottom: 16px; border-left: 3px solid #2563eb; padding-left: 10px; }
  .modal .close-x { float: right; cursor: pointer; color: #8a919f; font-size: 18px; }
  .tl { position: relative; padding-left: 24px; }
  .tl::before { content: ''; position: absolute; left: 7px; top: 8px; bottom: 8px; width: 2px; background: #e5e7eb; }
  .tl-item { position: relative; margin-bottom: 18px; }
  .tl-item::before { content: ''; position: absolute; left: -23px; top: 6px; width: 12px; height: 12px; border-radius: 50%; background: #2563eb; border: 2px solid #fff; box-shadow: 0 0 0 2px #2563eb; }
  .tl-item.revoked::before { background: #d92d20; box-shadow: 0 0 0 2px #d92d20; }
  .tl-item.expiring::before { background: #b76e00; box-shadow: 0 0 0 2px #b76e00; }
  .tl-item.expired::before { background: #8a919f; box-shadow: 0 0 0 2px #8a919f; }
  .tl-item .tl-title { font-size: 14px; font-weight: 600; }
  .tl-item .tl-meta { font-size: 12px; color: #8a919f; margin-top: 4px; line-height: 1.7; }
  .tl-item .tl-meta .mono { color: #555; }
  /* 客户下拉（datalist 样式提示） */
  .field .hint { font-size: 11px; color: #2563eb; margin-top: 4px; display: none; }
  .field .hint.show { display: block; }
  .counts { display: flex; gap: 14px; font-size: 12px; color: #8a919f; margin-left: auto; }
  .counts b { font-size: 13px; }
  .mono { font-family: "SF Mono", Consolas, monospace; font-size: 12px; color: #555; }
  .op-btn { padding: 4px 12px; border: 1px solid #d5d9e0; border-radius: 6px; background: #fff; font-size: 12px; cursor: pointer; margin-right: 6px; }
  .op-btn:hover { border-color: #2563eb; color: #2563eb; }
  .op-btn.danger:hover { border-color: #d92d20; color: #d92d20; }
  .empty { text-align: center; color: #8a919f; padding: 30px 0; font-size: 13px; }
  .toolbar { display: flex; justify-content: space-between; align-items: center; margin-bottom: 14px; }
  .toolbar input { padding: 7px 12px; border: 1px solid #d5d9e0; border-radius: 8px; font-size: 13px; outline: none; width: 240px; }
  .toolbar .count { font-size: 13px; color: #8a919f; }
</style>
</head>
<body>
<div class="wrap">
  <div class="header">
    <div class="logo">🔑</div>
    <div>
      <h1>Licen 授权签发中心</h1>
      <p>厂商侧工具 · 输入客户机器码，一键生成 License</p>
    </div>
  </div>

  <div class="card">
    <h2>签发 License</h2>
    <div class="grid">
      <div class="field full">
        <label>机器码 <span style="color:#d92d20">*</span>（客户在 licen-server 上执行 <code>curl /api/v1/machine-code</code> 或 <code>licen-tool machinecode</code> 获取）</label>
        <input id="machineCode" placeholder="例如: 079cee10e8a1bb2af162bf736813a1c5fc662d3366b75a97f2304a94d15df625">
      </div>
      <div class="field">
        <label>产品标识 <span style="color:#d92d20">*</span></label>
        <input id="product" value="licen-server" list="productList" placeholder="例如: ai-engine">
        <datalist id="productList"></datalist>
      </div>
      <div class="field">
        <label>版本/套餐</label>
        <input id="edition" value="enterprise" placeholder="enterprise">
      </div>
      <div class="field">
        <label>并发节点数</label>
        <input id="maxNodes" type="number" value="50" min="1">
      </div>
      <div class="field">
        <label>有效期（天）</label>
        <input id="days" type="number" value="365" min="1">
      </div>
      <div class="field full">
        <label>客户名称</label>
        <input id="customer" list="customerList" placeholder="例如: 某某电力集团（已有客户可下拉选择，自动带出最近签发参数）" oninput="onCustomerInput()">
        <datalist id="customerList"></datalist>
        <div class="hint" id="customerHint"></div>
      </div>
      <div class="field full">
        <label>功能点（逗号分隔，可选）</label>
        <input id="features" placeholder="例如: server-core,api,report">
      </div>
    </div>
    <div class="row">
      <button class="btn btn-primary" id="btnIssue" onclick="issue()">生成 License</button>
      <button class="btn btn-ghost" onclick="clearForm()">清空</button>
    </div>
    <p class="tip">
      💡 生成后请将 <code>license.json</code> 发给客户，客户上传到 licen-server：
      <code>curl -X POST http://&lt;server&gt;:&lt;port&gt;/api/v1/activate -d @license.json -H "Content-Type: application/json"</code><br>
      上传成功即激活全部功能；License 与客户机器码强绑定，拷到其他机器无效（MACHINE_MISMATCH）。
    </p>
  </div>

  <div class="result" id="result"></div>

  <div class="alert" id="expireAlert"></div>

  <div class="card">
    <h2>📒 已签发授权台账</h2>
    <div class="toolbar">
      <input id="searchBox" placeholder="🔍 搜索客户 / 产品 / License ID" oninput="renderLicenses()">
      <span class="count" id="licCount"></span>
      <span class="counts" id="licStats"></span>
    </div>
    <div id="licTableWrap">
      <table>
        <thead>
          <tr>
            <th>状态</th>
            <th>License ID</th>
            <th>客户</th>
            <th>产品 / 版本</th>
            <th>节点</th>
            <th>功能点</th>
            <th>签发时间</th>
            <th>到期时间</th>
            <th>剩余</th>
            <th>操作</th>
          </tr>
        </thead>
        <tbody id="licTable"></tbody>
      </table>
      <div class="empty" id="licEmpty" style="display:none">暂无签发记录</div>
    </div>
    <p class="tip">
      💡 状态说明：<span class="badge valid">有效</span> 正常授权
      · <span class="badge expiring">即将到期</span> 30 天内到期，请及时续期
      · <span class="badge expired">已过期</span> 超过到期时间
      · <span class="badge revoked">已吊销</span> 已作废（可「重新签发」生成新 License 续期/替换）
      <br>🔁 重新签发：用原参数（客户/产品/节点/功能点/机器码）生成新 License，有效期从今天重新起算，旧 License 自动标记吊销。
      <br>📜 时序：点击「时序」查看该授权从最初签发至今的完整续签链（每次续签/吊销留痕）。
      <br>🏷 预填：选择已有客户自动带出该客户最近一次签发参数（产品/版本/节点/功能点/机器码），仅需修改差异项。
    </p>
  </div>
</div>

<!-- 时间线弹窗 -->
<div class="modal-mask" id="tlMask" onclick="if(event.target===this)closeTl()">
  <div class="modal">
    <span class="close-x" onclick="closeTl()">✕</span>
    <h3>📜 授权时序</h3>
    <div id="tlBody"></div>
  </div>
</div>

<script>
async function issue() {
  const btn = document.getElementById('btnIssue');
  const res = document.getElementById('result');
  btn.disabled = true;
  btn.textContent = '签发中...';
  res.className = 'result';
  res.innerHTML = '';

  const payload = {
    machineCode: document.getElementById('machineCode').value.trim(),
    product: document.getElementById('product').value.trim(),
    edition: document.getElementById('edition').value.trim(),
    maxNodes: parseInt(document.getElementById('maxNodes').value) || 1,
    days: parseInt(document.getElementById('days').value) || 0,
    customer: document.getElementById('customer').value.trim(),
    features: document.getElementById('features').value.split(',').map(s => s.trim()).filter(Boolean)
  };

  try {
    const resp = await fetch('/api/v1/issue', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'X-Issuer-Token': prompt('请输入签发 Token：') || ''
      },
      body: JSON.stringify(payload)
    });
    const data = await resp.json();
    if (!data.success) {
      res.className = 'result err';
      res.innerHTML = '<h3>❌ 签发失败</h3><div>' + escapeHtml(data.message || '未知错误') + '</div>';
    } else {
      res.className = 'result ok';
      res.innerHTML =
        '<h3>✅ 签发成功</h3>' +
        '<pre>' + escapeHtml(data.licenseJson) + '</pre>' +
        '<a class="dl" download="license.json" href="data:text/plain;charset=utf-8,' + encodeURIComponent(data.licenseJson) + '">⬇ 下载 license.json</a>';
    }
  } catch (e) {
    res.className = 'result err';
    res.innerHTML = '<h3>❌ 请求失败</h3><div>' + escapeHtml(String(e)) + '</div>';
  } finally {
    btn.disabled = false;
    btn.textContent = '生成 License';
  }
}

function clearForm() {
  ['machineCode','product','edition','maxNodes','days','customer','features'].forEach(id => {
    const el = document.getElementById(id);
    if (el) { el.value = ''; }
  });
  document.getElementById('product').value = 'licen-server';
  document.getElementById('edition').value = 'enterprise';
  document.getElementById('maxNodes').value = '50';
  document.getElementById('days').value = '365';
  document.getElementById('result').className = 'result';
  document.getElementById('result').innerHTML = '';
}

function escapeHtml(s) {
  return String(s).replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;').replace(/"/g,'&quot;');
}

// ---------- 已签发台账 ----------
let licCache = [];
let licStats = { valid: 0, expiring: 0, expired: 0, revoked: 0 };

async function loadLicenses() {
  try {
    const token = sessionStorage.getItem('issuerToken') || '';
    const resp = await fetch('/api/v1/licenses', { headers: { 'X-Issuer-Token': token } });
    if (resp.status === 401) { return; } // 未鉴权不显示，等签发时输入
    const data = await resp.json();
    if (data.success) { licCache = data.licenses || []; licStats = data.stats || licStats; }
  } catch (e) { /* 静默 */ }
  renderLicenses();
  renderAlert();
  loadCustomers();
}

// 到期提醒条
function renderAlert() {
  const el = document.getElementById('expireAlert');
  const expiring = licStats.expiring || 0;
  const expired = licStats.expired || 0;
  const parts = [];
  if (expiring > 0) parts.push('⚠️ <b>' + expiring + '</b> 条授权 30 天内到期，请及时续期');
  if (expired > 0) parts.push('<span class="expired-alert">🔴 <b>' + expired + '</b> 条授权已过期</span>');
  if (parts.length === 0) { el.className = 'alert'; el.innerHTML = ''; return; }
  el.className = 'alert show';
  el.innerHTML = parts.join('&nbsp;&nbsp;');
}

// 客户列表（预填 datalist + 产品 datalist）
let customerCache = [];

async function loadCustomers() {
  try {
    const token = sessionStorage.getItem('issuerToken') || '';
    const resp = await fetch('/api/v1/customers', { headers: { 'X-Issuer-Token': token } });
    if (resp.status !== 200) return;
    const data = await resp.json();
    if (!data.success) return;
    customerCache = data.customers || [];
    document.getElementById('customerList').innerHTML = customerCache
      .map(c => '<option value="' + escapeHtml(c.customer) + '">').join('');
    const products = [...new Set(customerCache.map(c => c.lastProduct).filter(Boolean))];
    document.getElementById('productList').innerHTML = products
      .map(p => '<option value="' + escapeHtml(p) + '">').join('');
  } catch (e) { /* 静默 */ }
}

// 客户输入：匹配已有客户则提示并预填最近签发参数
function onCustomerInput() {
  const name = document.getElementById('customer').value.trim();
  const hint = document.getElementById('customerHint');
  if (!name) { hint.className = 'hint'; hint.innerHTML = ''; return; }
  const c = customerCache.find(x => x.customer === name);
  if (!c) {
    hint.className = 'hint';
    hint.innerHTML = '新客户，将新建授权记录';
    return;
  }
  hint.className = 'hint show';
  hint.innerHTML = '📋 已识别客户：共 ' + c.licenses + ' 条授权（有效 ' + c.activeCount + '，其中 ' + c.expiringCount + ' 条即将到期），已预填最近签发参数：' +
    escapeHtml(c.lastProduct + ' / ' + c.lastEdition + ' / ' + c.lastNodes + ' 节点');
  // 仅预填空白字段，避免覆盖用户已输入内容
  if (!document.getElementById('machineCode').value) document.getElementById('machineCode').value = c.machineCode || '';
  if (!document.getElementById('product').value || document.getElementById('product').value === 'licen-server') document.getElementById('product').value = c.lastProduct || '';
  if (!document.getElementById('edition').value) document.getElementById('edition').value = c.lastEdition || '';
  if (!document.getElementById('maxNodes').value) document.getElementById('maxNodes').value = c.lastNodes || '';
  if (!document.getElementById('features').value) document.getElementById('features').value = (c.lastFeatures || []).join(',');
}

function badge(status) {
  const map = { valid: ['有效','valid'], expiring: ['即将到期','expiring'], expired: ['已过期','expired'], revoked: ['已吊销','revoked'] };
  const [txt, cls] = map[status] || [status, 'expired'];
  return '<span class="badge ' + cls + '">' + txt + '</span>';
}

function fmtTime(t) {
  if (!t) return '-';
  const d = new Date(t);
  if (isNaN(d)) return String(t).slice(0, 10);
  return d.getFullYear() + '-' + String(d.getMonth()+1).padStart(2,'0') + '-' + String(d.getDate()).padStart(2,'0');
}

function shortId(id) {
  return id ? id.slice(0, 14) + '…' : '-';
}

function renderLicenses() {
  const q = (document.getElementById('searchBox').value || '').trim().toLowerCase();
  const rows = licCache.filter(r => {
    if (!q) return true;
    return (r.customer || '').toLowerCase().includes(q)
      || (r.product || '').toLowerCase().includes(q)
      || (r.licenseId || '').toLowerCase().includes(q);
  });
  const tbody = document.getElementById('licTable');
  const empty = document.getElementById('licEmpty');
  document.getElementById('licCount').textContent = '共 ' + rows.length + ' 条';
  document.getElementById('licStats').innerHTML =
    '🟢 <b>' + licStats.valid + '</b> 有效'
    + '&nbsp;🟠 <b>' + licStats.expiring + '</b> 即将到期'
    + '&nbsp;🔴 <b>' + licStats.expired + '</b> 已过期'
    + '&nbsp;⚫ <b>' + licStats.revoked + '</b> 已吊销';
  if (rows.length === 0) {
    tbody.innerHTML = '';
    empty.style.display = 'block';
    return;
  }
  empty.style.display = 'none';
  tbody.innerHTML = rows.map(r => {
    const opRevoke = r.status === 'valid'
      ? '<button class="op-btn danger" onclick="revokeLic(\'' + r.licenseId + '\')">吊销</button>'
      : '';
    const opReissue = (r.status === 'valid' || r.status === 'expiring' || r.status === 'expired')
      ? '<button class="op-btn" onclick="reissueLic(\'' + r.licenseId + '\')">重新签发</button>'
      : '';
    const dl = '<button class="op-btn" onclick="downloadLic(\'' + r.licenseId + '\')">下载</button>';
    const tl = '<button class="op-btn" onclick="showTimeline(\'' + r.licenseId + '\')">时序</button>';
    // 剩余天数：有效绿色/即将到期橙色/已过期红色
    let daysHtml = '-';
    if (r.status === 'valid') daysHtml = '<span style="color:#168a2f">' + r.daysLeft + ' 天</span>';
    else if (r.status === 'expiring') daysHtml = '<span style="color:#b76e00;font-weight:600">' + r.daysLeft + ' 天</span>';
    else if (r.status === 'expired') daysHtml = '<span style="color:#d92d20">已过期 ' + Math.abs(r.daysLeft) + ' 天</span>';
    return '<tr>'
      + '<td>' + badge(r.status) + '</td>'
      + '<td class="mono" title="' + escapeHtml(r.licenseId || '') + '">' + escapeHtml(shortId(r.licenseId)) + '</td>'
      + '<td>' + escapeHtml(r.customer || '-') + '</td>'
      + '<td>' + escapeHtml(r.product || '-') + ' / ' + escapeHtml(r.edition || '-') + '</td>'
      + '<td>' + (r.maxNodes || 0) + '</td>'
      + '<td>' + escapeHtml((r.features || []).join(',')) + '</td>'
      + '<td>' + fmtTime(r.issuedAt) + '</td>'
      + '<td>' + fmtTime(r.expiresAt) + '</td>'
      + '<td>' + daysHtml + '</td>'
      + '<td>' + dl + tl + opReissue + opRevoke + '</td>'
      + '</tr>';
  }).join('');
}

// ---------- 授权时序 ----------
async function showTimeline(id) {
  const mask = document.getElementById('tlMask');
  const body = document.getElementById('tlBody');
  body.innerHTML = '<div class="empty">加载中...</div>';
  mask.className = 'modal-mask show';
  try {
    const token = sessionStorage.getItem('issuerToken') || '';
    const resp = await fetch('/api/v1/licenses/' + encodeURIComponent(id) + '/timeline', { headers: { 'X-Issuer-Token': token } });
    const data = await resp.json();
    if (!data.success) { body.innerHTML = '<div class="empty">' + escapeHtml(data.message || '查询失败') + '</div>'; return; }
    const list = data.timeline || [];
    body.innerHTML = '<div class="tl">' + list.map((item, i) => {
      const cls = item.status;
      const reason = item.revoked
        ? (item.revokeNote ? ' · 原因：' + escapeHtml(item.revokeNote) : ' · 已吊销')
        : (item.reissuedTo ? ' · 已续签至 ' + escapeHtml(shortId(item.reissuedTo)) : '');
      const from = item.reissuedFrom ? '（续签自 ' + escapeHtml(shortId(item.reissuedFrom)) + '）' : '（最初签发）';
      return '<div class="tl-item ' + cls + '">'
        + '<div class="tl-title">#' + (list.length - i) + ' ' + badge(item.status) + ' ' + escapeHtml(item.licenseId) + '</div>'
        + '<div class="tl-meta">'
        + '客户：' + escapeHtml(item.customer || '-') + ' · 产品：' + escapeHtml(item.product || '-') + ' / ' + escapeHtml(item.edition || '-')
        + '<br>签发：' + fmtTime(item.issuedAt) + ' ' + from
        + '<br>到期：' + fmtTime(item.expiresAt)
        + (item.revoked ? '<br>吊销：' + fmtTime(item.revokedAt) : '')
        + '<br>节点：' + (item.maxNodes || 0) + ' · 功能点：' + escapeHtml((item.features || []).join(','))
        + reason
        + '</div></div>';
    }).join('') + '</div>';
  } catch (e) {
    body.innerHTML = '<div class="empty">请求失败: ' + escapeHtml(String(e)) + '</div>';
  }
}

function closeTl() {
  document.getElementById('tlMask').className = 'modal-mask';
}

function getToken() {
  let t = sessionStorage.getItem('issuerToken');
  if (!t) {
    t = prompt('请输入签发 Token：') || '';
    if (t) sessionStorage.setItem('issuerToken', t);
  }
  return t;
}

async function apiPost(url, body) {
  const resp = await fetch(url, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', 'X-Issuer-Token': getToken() },
    body: body ? JSON.stringify(body) : '{}'
  });
  return resp.json();
}

async function revokeLic(id) {
  if (!confirm('确定吊销 ' + id + ' ？吊销后该授权作废，可重新签发替换。')) return;
  try {
    const data = await apiPost('/api/v1/licenses/' + id + '/revoke', { note: '人工吊销' });
    alert(data.message || '吊销失败');
    await loadLicenses();
  } catch (e) { alert('请求失败: ' + e); }
}

async function reissueLic(id) {
  if (!confirm('重新签发 ' + id + ' ？将生成新 License（新 ID、有效期从今天起算），旧 License 自动吊销。')) return;
  try {
    const data = await apiPost('/api/v1/licenses/' + id + '/reissue', {});
    if (data.success) {
      const res = document.getElementById('result');
      res.className = 'result ok';
      res.innerHTML = '<h3>✅ 重新签发成功（新 License）</h3>'
        + '<pre>' + escapeHtml(data.licenseJson || '') + '</pre>'
        + '<a class="dl" download="license.json" href="data:text/plain;charset=utf-8,' + encodeURIComponent(data.licenseJson || '') + '">⬇ 下载 license.json</a>';
    } else {
      alert(data.message || '重新签发失败');
    }
    await loadLicenses();
  } catch (e) { alert('请求失败: ' + e); }
}

function downloadLic() {
  alert('台账仅存授权记录（不含签名文件）。如需再次下载完整 license.json，请使用「重新签发」生成新 License。');
}

// 页面加载时拉取台账
loadLicenses();
</script>
</body>
</html>`
