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
  .badge.expired { background: #fef3e6; color: #b76e00; }
  .badge.revoked { background: #fdeeee; color: #d92d20; }
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
        <input id="product" value="licen-server" placeholder="例如: ai-engine">
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
        <input id="customer" placeholder="例如: 某某电力集团">
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

  <div class="card">
    <h2>📒 已签发授权台账</h2>
    <div class="toolbar">
      <input id="searchBox" placeholder="🔍 搜索客户 / 产品 / License ID" oninput="renderLicenses()">
      <span class="count" id="licCount"></span>
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
            <th>操作</th>
          </tr>
        </thead>
        <tbody id="licTable"></tbody>
      </table>
      <div class="empty" id="licEmpty" style="display:none">暂无签发记录</div>
    </div>
    <p class="tip">
      💡 状态说明：<span class="badge valid">有效</span> 正常授权
      · <span class="badge expired">已过期</span> 超过到期时间
      · <span class="badge revoked">已吊销</span> 已作废（可「重新签发」生成新 License 续期/替换）
      <br>🔁 重新签发：用原参数（客户/产品/节点/功能点/机器码）生成新 License，有效期从今天重新起算，旧 License 自动标记吊销。
    </p>
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

async function loadLicenses() {
  try {
    const token = sessionStorage.getItem('issuerToken') || '';
    const resp = await fetch('/api/v1/licenses', { headers: { 'X-Issuer-Token': token } });
    if (resp.status === 401) { return; } // 未鉴权不显示，等签发时输入
    const data = await resp.json();
    if (data.success) { licCache = data.licenses || []; }
  } catch (e) { /* 静默 */ }
  renderLicenses();
}

function badge(status) {
  const map = { valid: ['有效','valid'], expired: ['已过期','expired'], revoked: ['已吊销','revoked'] };
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
    const opReissue = (r.status === 'valid' || r.status === 'expired')
      ? '<button class="op-btn" onclick="reissueLic(\'' + r.licenseId + '\')">重新签发</button>'
      : '';
    const dl = '<button class="op-btn" onclick="downloadLic(\'' + r.licenseId + '\')">下载</button>';
    return '<tr>'
      + '<td>' + badge(r.status) + '</td>'
      + '<td class="mono" title="' + escapeHtml(r.licenseId || '') + '">' + escapeHtml(shortId(r.licenseId)) + '</td>'
      + '<td>' + escapeHtml(r.customer || '-') + '</td>'
      + '<td>' + escapeHtml(r.product || '-') + ' / ' + escapeHtml(r.edition || '-') + '</td>'
      + '<td>' + (r.maxNodes || 0) + '</td>'
      + '<td>' + escapeHtml((r.features || []).join(',')) + '</td>'
      + '<td>' + fmtTime(r.issuedAt) + '</td>'
      + '<td>' + fmtTime(r.expiresAt) + '</td>'
      + '<td>' + dl + opReissue + opRevoke + '</td>'
      + '</tr>';
  }).join('');
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
