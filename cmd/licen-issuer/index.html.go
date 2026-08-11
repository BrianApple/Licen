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
  .wrap { max-width: 860px; margin: 40px auto; padding: 0 20px; }
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
</script>
</body>
</html>`
