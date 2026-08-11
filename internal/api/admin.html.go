// adminHTML licen-server 内置 Web 管理平台（Ant Design v5 + @ant-design/icons）。
// 资源策略：React/ReactDOM/dayjs/antd/icons 五个 UMD 文件 go:embed 内嵌进二进制，
// 零外部 CDN 依赖，客户隔离内网离线可用。
// 功能：授权状态总览 / License 热重载 / 节点管理 / Apps 管理 / 审计日志。
// 鉴权：首次使用时输入管理 Token（X-Admin-Token），保存在浏览器 localStorage。
package api

import "embed"

//go:embed assets/*
var assetsFS embed.FS

const adminHTML = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Licen 授权管理平台</title>
<style>
  * { margin: 0; padding: 0; box-sizing: border-box; }
  body { background: #f5f5f5; font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, "PingFang SC", "Microsoft YaHei", sans-serif; }
  .app-header { background: #fff; padding: 0 24px; height: 56px; display: flex; align-items: center; gap: 12px; box-shadow: 0 1px 4px rgba(0,0,0,.08); position: sticky; top: 0; z-index: 10; }
  .app-header .logo { font-size: 22px; color: #1677ff; display: flex; align-items: center; }
  .app-header h1 { font-size: 17px; font-weight: 600; color: rgba(0,0,0,.88); }
  .app-header .sub { font-size: 12px; color: rgba(0,0,0,.45); margin-left: 8px; }
  .app-header .spacer { flex: 1; }
  .content { max-width: 1200px; margin: 20px auto; padding: 0 20px 40px; }
  .token-bar { background: #fff; border-radius: 8px; padding: 16px 20px; margin-bottom: 16px; display: flex; align-items: center; gap: 12px; box-shadow: 0 1px 2px rgba(0,0,0,.06); }
  .token-bar .tip { color: rgba(0,0,0,.45); font-size: 12px; }
  .stat-grid { display: grid; grid-template-columns: repeat(4, 1fr); gap: 12px; }
  @media (max-width: 900px) { .stat-grid { grid-template-columns: repeat(2, 1fr); } }
  .mono { font-family: "SF Mono", "Cascadia Code", Consolas, monospace; font-size: 12px; }
  .section-card { margin-bottom: 16px; }
</style>
</head>
<body>
<div id="root"></div>

<script src="/admin/assets/react.production.min.js"></script>
<script src="/admin/assets/react-dom.production.min.js"></script>
<script src="/admin/assets/dayjs.min.js"></script>
<script src="/admin/assets/antd.min.js"></script>
<script src="/admin/assets/icons.min.js"></script>

<script>
(function () {
  "use strict";
  var React = window.React, ReactDOM = window.ReactDOM;
  var antd = window.antd, icons = window.icons;
  var h = React.createElement;
  var LS_KEY = 'licenAdminToken';

  var Button = antd.Button, Card = antd.Card, Table = antd.Table, Tag = antd.Tag,
      Badge = antd.Badge, Modal = antd.Modal, Form = antd.Form, Input = antd.Input,
      Popconfirm = antd.Popconfirm, Space = antd.Space, Typography = antd.Typography,
      Row = antd.Row, Col = antd.Col, Statistic = antd.Statistic, Tooltip = antd.Tooltip,
      message = antd.message, ConfigProvider = antd.ConfigProvider;

  var KeyOutlined = icons.KeyOutlined, SafetyOutlined = icons.SafetyCertificateOutlined,
      DashboardOutlined = icons.DashboardOutlined, ReloadOutlined = icons.ReloadOutlined,
      CloudServerOutlined = icons.CloudServerOutlined, AppstoreOutlined = icons.AppstoreOutlined,
      FileSearchOutlined = icons.FileSearchOutlined, PlusOutlined = icons.PlusOutlined,
      DeleteOutlined = icons.DeleteOutlined, CheckCircleFilled = icons.CheckCircleFilled,
      CloseCircleFilled = icons.CloseCircleFilled, SyncOutlined = icons.SyncOutlined,
      LockOutlined = icons.LockOutlined, NodeIndexOutlined = icons.NodeIndexOutlined,
      ApiOutlined = icons.ApiOutlined, HistoryOutlined = icons.HistoryOutlined;

  function getToken() {
    var t = document.getElementById('token-input');
    return (t && t.value ? t.value : localStorage.getItem(LS_KEY) || '').trim();
  }

  function api(path, opts) {
    opts = opts || {};
    var headers = Object.assign({ 'X-Admin-Token': getToken() }, opts.headers || {});
    return fetch(path, Object.assign({}, opts, { headers: headers })).then(function (resp) {
      if (resp.status === 401) { message.error('管理 Token 无效，请检查后重试'); throw new Error('UNAUTHORIZED'); }
      return resp.json().catch(function () { return {}; });
    });
  }

  function fmtTime(t) {
    if (!t) return '-';
    var d = new Date(t);
    if (isNaN(d.getTime())) return String(t);
    function p(n) { return n < 10 ? '0' + n : '' + n; }
    return d.getFullYear() + '-' + p(d.getMonth() + 1) + '-' + p(d.getDate()) + ' ' + p(d.getHours()) + ':' + p(d.getMinutes());
  }

  var App = function () {
    var _s = React.useState(false), loading = _s[0], setLoading = _s[1];
    var _l = React.useState(null), licData = _l[0], setLic = _l[1];
    var _n = React.useState([]), nodes = _n[0], setNodes = _n[1];
    var _a = React.useState([]), apps = _a[0], setApps = _a[1];
    var _u = React.useState([]), audits = _u[0], setAudits = _u[1];
    var _m = React.useState(false), modalOpen = _m[0], setModal = _m[1];
    var _f = React.useState(null), form = _f[0], setForm = _f[1];
    var _t = React.useState(localStorage.getItem(LS_KEY) || ''), tokenVal = _t[0], setTokenVal = _t[1];

    function loadAll() {
      setLoading(true);
      Promise.all([
        api('/api/v1/admin/license/status').then(function (d) { setLic(d); return d; }).catch(function () { setLic(null); }),
        api('/api/v1/admin/nodes?size=100').then(function (d) { setNodes(Array.isArray(d) ? d : []); }).catch(function () { setNodes([]); }),
        api('/api/v1/admin/apps').then(function (d) { setApps(Array.isArray(d) ? d : []); }).catch(function () { setApps([]); }),
        api('/api/v1/admin/audits?size=100').then(function (d) { setAudits(Array.isArray(d) ? d : []); }).catch(function () { setAudits([]); })
      ]).then(function () { setLoading(false); });
    }

    React.useEffect(function () { if (localStorage.getItem(LS_KEY)) loadAll(); }, []);

    function saveToken() {
      var v = getToken();
      if (!v) { message.warning('请输入管理 Token'); return; }
      localStorage.setItem(LS_KEY, v);
      setTokenVal(v);
      message.success('Token 已保存（存于本机浏览器）');
      loadAll();
    }

    function reloadLicense() {
      api('/api/v1/admin/license/reload', { method: 'POST' }).then(function (d) {
        if (d.success) { message.success('License 已热重载：' + d.result); } else { message.error('热重载失败：' + (d.result || '')); }
        loadAll();
      }).catch(function () {});
    }

    function revokeNode(id) {
      api('/api/v1/admin/nodes/' + id, { method: 'DELETE' }).then(function (d) {
        d.success ? message.success('节点已吊销') : message.error('吊销失败');
        loadAll();
      }).catch(function () {});
    }

    function createApp(values) {
      api('/api/v1/admin/apps', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(values) }).then(function (d) {
        if (d.success) { message.success('应用已创建'); setModal(false); form.resetFields(); } else { message.error('创建失败：' + (d.message || '')); }
        loadAll();
      }).catch(function () {});
    }

    function deleteApp(id) {
      api('/api/v1/admin/apps/' + id, { method: 'DELETE' }).then(function (d) {
        d.success ? message.success('应用已删除') : message.error('删除失败');
        loadAll();
      }).catch(function () {});
    }

    // -------- 授权状态卡片 --------
    var lic = licData && licData.license ? licData.license : null;
    var valid = licData ? licData.valid : false;
    var statusBadge = lic && valid
      ? h(Tag, { color: 'success', style: { fontSize: 14, padding: '3px 14px', borderRadius: 20 } }, h(CheckCircleFilled), ' VALID')
      : h(Tag, { color: 'error', style: { fontSize: 14, padding: '3px 14px', borderRadius: 20 } }, h(CloseCircleFilled), ' ' + ((licData && licData.result) || 'INVALID'));

    var statCards = h(Row, { gutter: [12, 12] },
      h(Col, { span: 6, xs: 12 }, h(Card, { size: 'small' }, h(Statistic, { title: '授权状态', valueRender: function () { return statusBadge; } }))),
      h(Col, { span: 6, xs: 12 }, h(Card, { size: 'small' }, h(Statistic, { title: '并发节点', value: lic ? lic.maxNodes : 0, suffix: '个' }))),
      h(Col, { span: 6, xs: 12 }, h(Card, { size: 'small' }, h(Statistic, { title: '客户', valueRender: function () { return h(Typography.Text, { ellipsis: true, style: { maxWidth: 160 } }, lic ? lic.customer : '-'); } }))),
      h(Col, { span: 6, xs: 12 }, h(Card, { size: 'small' }, h(Statistic, { title: '在线节点', value: Array.isArray(nodes) ? nodes.filter(function (n) { return n.status === 'ONLINE'; }).length : 0, suffix: '/' + (lic ? lic.maxNodes : 0) })))
    );

    var licDetail = h(Card, { className: 'section-card', title: h('span', null, h(DashboardOutlined, { style: { marginRight: 8, color: '#1677ff' } }), '授权状态') },
      lic ? h(antd.Descriptions, { size: 'small', column: { xs: 1, sm: 2, md: 3 }, bordered: true,
        items: [
          { key: 'licenseId', label: 'License ID', children: h('span', { className: 'mono' }, lic.licenseId) },
          { key: 'product', label: '产品 / 版本', children: (lic.product || '-') + ' / ' + (lic.edition || '-') },
          { key: 'machineCode', label: '机器码', children: h(Tooltip, { title: lic.machineCode }, h(Typography.Text, { className: 'mono', ellipsis: true, style: { maxWidth: 320 } }, lic.machineCode)) },
          { key: 'features', label: '功能点', children: lic.features && lic.features.length ? lic.features.map(function (f) { return h(Tag, { color: 'blue', key: f }, f); }) : h(Tag, null, '全部') },
          { key: 'issuedAt', label: '签发时间', children: fmtTime(lic.issuedAt) },
          { key: 'expiresAt', label: '到期时间', children: h('span', { style: { color: new Date(lic.expiresAt) < new Date() ? '#cf1322' : 'inherit', fontWeight: 600 } }, fmtTime(lic.expiresAt)) }
        ] }) : h(antd.Empty, { description: 'License 未激活' })
    );

    // -------- 节点表格 --------
    var nodeColumns = [
      { title: '状态', dataIndex: 'status', width: 90, render: function (s) { return s === 'ONLINE' ? h(Badge, { status: 'success', text: '在线' }) : h(Badge, { status: 'default', text: '离线' }); } },
      { title: '节点名称', dataIndex: 'nodeName', render: function (v, r) { return h('span', null, h(NodeIndexOutlined, { style: { marginRight: 6, color: '#1677ff' } }), v || '-'); } },
      { title: 'NodeID', dataIndex: 'nodeId', className: 'mono' },
      { title: 'AppKey', dataIndex: 'appKey', className: 'mono' },
      { title: 'IP', dataIndex: 'ip', width: 130 },
      { title: '版本', dataIndex: 'version', width: 90 },
      { title: '注册时间', dataIndex: 'registeredAt', width: 150, render: fmtTime },
      { title: '最后心跳', dataIndex: 'lastHeartbeatAt', width: 150, render: fmtTime },
      { title: '操作', width: 90, render: function (_, r) {
          return h(Popconfirm, { title: '确认吊销该节点？', onConfirm: function () { revokeNode(r.id); } },
            h(Button, { type: 'link', danger: true, size: 'small', icon: h(DeleteOutlined) }, '吊销'));
        } }
    ];

    var nodeTable = h(Card, { className: 'section-card', title: h('span', null, h(CloudServerOutlined, { style: { marginRight: 8, color: '#1677ff' } }), '节点管理（并发控制）') },
      h(Table, { rowKey: 'id', size: 'small', columns: nodeColumns, dataSource: nodes, loading: loading, pagination: { pageSize: 8, showTotal: function (t) { return '共 ' + t + ' 个节点'; } }, scroll: { x: 1100 } })
    );

    // -------- Apps 表格 --------
    var appColumns = [
      { title: 'ID', dataIndex: 'id', width: 60 },
      { title: '名称', dataIndex: 'name', render: function (v, r) { return h('span', null, h(AppstoreOutlined, { style: { marginRight: 6, color: '#1677ff' } }), v || '-'); } },
      { title: '产品', dataIndex: 'product', width: 140 },
      { title: 'AppKey', dataIndex: 'appKey', className: 'mono' },
      { title: 'AppSecret', dataIndex: 'appSecret', className: 'mono', ellipsis: true },
      { title: '状态', dataIndex: 'enabled', width: 80, render: function (v) { return v ? h(Tag, { color: 'success' }, '启用') : h(Tag, { color: 'warning' }, '停用'); } },
      { title: '创建时间', dataIndex: 'createdAt', width: 150, render: fmtTime },
      { title: '操作', width: 90, render: function (_, r) {
          return h(Popconfirm, { title: '确认删除该应用？', onConfirm: function () { deleteApp(r.id); } },
            h(Button, { type: 'link', danger: true, size: 'small', icon: h(DeleteOutlined) }, '删除'));
        } }
    ];

    var appTable = h(Card, { className: 'section-card',
      title: h('span', null, h(ApiOutlined, { style: { marginRight: 8, color: '#1677ff' } }), '应用管理（AppKey/Secret）'),
      extra: h(Button, { type: 'primary', icon: h(PlusOutlined), onClick: function () { setModal(true); } }, '新建应用'),
      children: h(Table, { rowKey: 'id', size: 'small', columns: appColumns, dataSource: apps, loading: loading, pagination: { pageSize: 6, showTotal: function (t) { return '共 ' + t + ' 个应用'; } }, scroll: { x: 1000 } })
    });

    // -------- 审计表格 --------
    var auditColumns = [
      { title: '时间', dataIndex: 'time', width: 170, render: fmtTime },
      { title: '动作', dataIndex: 'action', width: 200, render: function (v) { return h(Tag, { color: 'geekblue' }, v); } },
      { title: '详情', dataIndex: 'detail', className: 'mono' }
    ];

    var auditTable = h(Card, { className: 'section-card', title: h('span', null, h(HistoryOutlined, { style: { marginRight: 8, color: '#1677ff' } }), '审计日志') },
      h(Table, { rowKey: 'id', size: 'small', columns: auditColumns, dataSource: audits, loading: loading, pagination: { pageSize: 8, showTotal: function (t) { return '共 ' + t + ' 条'; } }, scroll: { x: 800 } })
    );

    // -------- 新建应用 Modal --------
    var createModal = h(Modal, { title: h('span', null, h(AppstoreOutlined, { style: { marginRight: 8, color: '#1677ff' } }), '新建应用'), open: modalOpen, onCancel: function () { setModal(false); }, footer: null, destroyOnHidden: true },
      h(Form, { ref: function (f) { if (f && f !== form) setForm(f); }, layout: 'vertical', onFinish: createApp, initialValues: { product: 'licen-server', enabled: true },
        children: [
          h(Form.Item, { label: '应用名称', name: 'name', rules: [{ required: true, message: '请输入应用名称' }] }, h(Input, { placeholder: '如：AI推理节点集群' })),
          h(Form.Item, { label: '产品标识', name: 'product' }, h(Input, { placeholder: 'licen-server' })),
          h(Form.Item, { label: 'AppKey', name: 'appKey', rules: [{ required: true, message: '请输入 AppKey' }] }, h(Input, { placeholder: '唯一标识，如 licen-server-app' })),
          h(Form.Item, { label: 'AppSecret', name: 'appSecret' }, h(Input.Password, { placeholder: '留空自动生成' })),
          h(Form.Item, { children: h(Space, null, h(Button, { type: 'primary', htmlType: 'submit' }, '创建'), h(Button, { onClick: function () { setModal(false); } }, '取消')) })
        ] })
    );

    // -------- 顶部 Token 栏 --------
    var tokenBar = h('div', { className: 'token-bar' },
      h(LockOutlined, { style: { fontSize: 16, color: '#1677ff' } }),
      h('span', { style: { fontWeight: 600 } }, '管理 Token：'),
      h(Input.Password, { id: 'token-input', value: tokenVal, onChange: function (e) { setTokenVal(e.target.value); }, placeholder: '输入 X-Admin-Token', style: { width: 260 }, onPressEnter: saveToken }),
      h(Button, { type: 'primary', onClick: saveToken }, '保存并加载'),
      tokenVal ? h('span', { className: 'tip' }, '✅ Token 已保存（存于本机浏览器）') : h('span', { className: 'tip' }, 'Token 仅存于本机 localStorage，不随请求之外的任何地方传输')
    );

    return h('div', null,
      h('div', { className: 'app-header' },
        h('span', { className: 'logo' }, h(SafetyOutlined)),
        h('h1', null, 'Licen 授权管理平台'),
        h('span', { className: 'sub' }, '客户侧授权服务 · 状态 / 节点 / 应用 / 审计 一站式管理'),
        h('div', { className: 'spacer' }),
        h(Button, { icon: h(SyncOutlined), onClick: loadAll, loading: loading }, '刷新')
      ),
      h('div', { className: 'content' },
        tokenBar,
        statCards,
        h(Row, { gutter: [12, 12] }, h(Col, { span: 24 }, licDetail)),
        nodeTable,
        appTable,
        auditTable
      ),
      createModal
    );
  };

  ReactDOM.render(
    h(ConfigProvider, { theme: { token: { colorPrimary: '#1677ff' } } },
      h(antd.App, null, h(App))
    ),
    document.getElementById('root')
  );
})();
</script>
</body>
</html>`
