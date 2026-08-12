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
  /* 客户档案 */
  .toolbar select { padding: 7px 12px; border: 1px solid #d5d9e0; border-radius: 8px; font-size: 13px; outline: none; background: #fff; }
  .arch-prod { border: 1px solid #eef0f3; border-radius: 10px; padding: 12px 16px; margin-bottom: 10px; background: #fafbfd; }
  .arch-prod h4 { font-size: 13px; margin-bottom: 8px; color: #2563eb; }
  .arch-row { display: flex; align-items: center; gap: 8px; font-size: 12px; padding: 5px 0; border-bottom: 1px dashed #eceef2; flex-wrap: wrap; }
  .arch-row:last-child { border-bottom: none; }
  .arch-row .dim { color: #8a919f; font-size: 11px; }

  /* 图标（内联 SVG，antd @ant-design/icons 风格，零依赖） */
  .icon { width: 1em; height: 1em; vertical-align: -0.12em; fill: currentColor; display: inline-block; }
  .logo .icon { width: 26px; height: 26px; }
  .card h2 .icon, .modal h3 .icon { vertical-align: -0.15em; margin-right: 2px; }
  .close-x .icon { width: 14px; height: 14px; }
  .op-btn .icon { vertical-align: -0.1em; margin-right: 2px; }
  .tip .icon { vertical-align: -0.15em; margin-right: 2px; }
  .arch-row .icon { vertical-align: -0.12em; }
  .search-wrap { position: relative; }
  .search-wrap .icon { position: absolute; left: 10px; top: 50%; transform: translateY(-50%); color: #8a919f; }
  .search-wrap input { padding-left: 30px !important; }
  /* 状态圆点 */
  .dot { display: inline-block; width: 8px; height: 8px; border-radius: 50%; margin-right: 4px; vertical-align: 1px; }
  .dot.green { background: #168a2f; }
  .dot.orange { background: #b76e00; }
  .dot.red { background: #d92d20; }
  .dot.gray { background: #8a919f; }
</style>
</head>
<body>
<!-- 图标库：内联 antd @ant-design/icons 风格 SVG（Apache-2.0），零外部依赖 -->
<svg xmlns="http://www.w3.org/2000/svg" style="display:none" aria-hidden="true">
  <symbol id="icon-key" viewBox="0 0 1024 1024"><path d="M608 112c-167.9 0-304 136.1-304 304 0 70.3 23.9 135 63.9 186.5l-41.1 41.1c-3.1 3.1-8.2 3.1-11.3 0L296.2 624c-3.1-3.1-8.2-3.1-11.3 0l-36.7 36.7c-3.1 3.1-3.1 8.2 0 11.3L267 690.9c3.1 3.1 3.1 8.2 0 11.3l-36.7 36.7c-3.1 3.1-3.1 8.2 0 11.3L249 770.1c3.1 3.1 3.1 8.2 0 11.3l-36.7 36.7a8.2 8.2 0 0 0 0 11.3l18.7 18.7c3.1 3.1 8.2 3.1 11.3 0l223-223c51.5 40 116 63.9 186.7 63.9 167.9 0 304-136.1 304-304S775.9 112 608 112zm161.2 465.2C726.2 620.3 668.9 644 608 644c-60.9 0-118.2-23.7-161.2-66.8-43.1-43-66.8-100.3-66.8-161.2 0-60.9 23.7-118.2 66.8-161.2 43-43.1 100.3-66.8 161.2-66.8 60.9 0 118.2 23.7 161.2 66.8 43.1 43 66.8 100.3 66.8 161.2 0 60.9-23.7 118.2-66.8 161.2z"/></symbol>
  <symbol id="icon-table" viewBox="0 0 1024 1024"><path d="M928 160H96c-17.7 0-32 14.3-32 32v640c0 17.7 14.3 32 32 32h832c17.7 0 32-14.3 32-32V192c0-17.7-14.3-32-32-32zm-40 208H676V232h212v136zm0 224H676V432h212v160zm0 224H676V656h212v136zM416 232h212v136H416V232zm0 224h212v160H416V456zm0 224h212v136H416V680zM236 232h132v136H236V232zm0 224h132v160H236V456zm0 224h132v136H236V680z"/></symbol>
  <symbol id="icon-appstore" viewBox="0 0 1024 1024"><path d="M464 144H160c-8.8 0-16 7.2-16 16v304c0 8.8 7.2 16 16 16h304c8.8 0 16-7.2 16-16V160c0-8.8-7.2-16-16-16zm-52 268H212V212h200v200zm452-268H560c-8.8 0-16 7.2-16 16v304c0 8.8 7.2 16 16 16h304c8.8 0 16-7.2 16-16V160c0-8.8-7.2-16-16-16zm-52 268H612V212h200v200zM464 544H160c-8.8 0-16 7.2-16 16v304c0 8.8 7.2 16 16 16h304c8.8 0 16-7.2 16-16V560c0-8.8-7.2-16-16-16zm-52 268H212V612h200v200zm452-268H560c-8.8 0-16 7.2-16 16v304c0 8.8 7.2 16 16 16h304c8.8 0 16-7.2 16-16V560c0-8.8-7.2-16-16-16zm-52 268H612V612h200v200z"/></symbol>
  <symbol id="icon-folder" viewBox="0 0 1024 1024"><path d="M880 298.4H521L403.7 186.2a8.15 8.15 0 0 0-5.5-2.2H144c-17.7 0-32 14.3-32 32v592c0 17.7 14.3 32 32 32h736c17.7 0 32-14.3 32-32V330.4c0-17.7-14.3-32-32-32zM840 768H184V256h188.5l119.6 114.4H840V768z"/></symbol>
  <symbol id="icon-history" viewBox="0 0 1024 1024"><path d="M512 64C264.6 64 64 264.6 64 512s200.6 448 448 448 448-200.6 448-448S759.4 64 512 64zm0 820c-205.4 0-372-166.6-372-372s166.6-372 372-372 372 166.6 372 372-166.6 372-372 372zm140.8-302.5l-100-58.1V268c0-4.4-3.6-8-8-8h-56c-4.4 0-8 3.6-8 8v290.4c0 3.1 1.7 5.9 4.4 7.4l113.3 65.8c3.8 2.2 8.5 1 10.7-2.8l29.7-51.2c2.2-3.8 1-8.5-2.9-10.7z"/></symbol>
  <symbol id="icon-download" viewBox="0 0 1024 1024"><path d="M505.7 661a8 8 0 0 0 12.6 0l112-141.7c4.1-5.2.4-12.9-6.3-12.9h-74.1V168c0-4.4-3.6-8-8-8h-60c-4.4 0-8 3.6-8 8v338.3H400c-6.7 0-10.4 7.7-6.3 12.9l112 141.8zM878 626h-60c-4.4 0-8 3.6-8 8v154H214V634c0-4.4-3.6-8-8-8h-60c-4.4 0-8 3.6-8 8v198c0 17.7 14.3 32 32 32h684c17.7 0 32-14.3 32-32V634c0-4.4-3.6-8-8-8z"/></symbol>
  <symbol id="icon-plus" viewBox="0 0 1024 1024"><path d="M482 152h60q8 0 8 8v704q0 8-8 8h-60q-8 0-8-8V160q0-8 8-8z"/><path d="M176 474h672q8 0 8 8v60q0 8-8 8H176q-8 0-8-8v-60q0-8 8-8z"/></symbol>
  <symbol id="icon-close" viewBox="0 0 1024 1024"><path d="M563.8 512l262.5-312.9c4.4-5.2.7-13.1-6.1-13.1h-79.8c-4.7 0-9.2 2.1-12.3 5.7L511.6 449.8 295.1 191.7c-3-3.6-7.5-5.7-12.3-5.7H203c-6.8 0-10.5 7.9-6.1 13.1L459.4 512 196.9 824.9A7.95 7.95 0 0 0 203 838h79.8c4.7 0 9.2-2.1 12.3-5.7l216.5-258.1 216.5 258.1c3 3.6 7.5 5.7 12.3 5.7h79.8c6.8 0 10.5-7.9 6.1-13.1L563.8 512z"/></symbol>
  <symbol id="icon-search" viewBox="0 0 1024 1024"><path d="M909.6 854.5L649.9 594.8C690.2 542.7 712 479 712 412c0-80.2-31.3-155.4-87.9-212.1-56.6-56.7-132-87.9-212.1-87.9s-155.5 31.3-212.1 87.9C143.2 256.5 112 331.8 112 412c0 80.1 31.3 155.5 87.9 212.1C256.5 680.8 331.8 712 412 712c67 0 130.6-21.8 182.7-62l259.7 259.6a8.2 8.2 0 0 0 11.6 0l43.6-43.5a8.2 8.2 0 0 0 0-11.6zM570.4 570.4C528 612.7 471.8 636 412 636s-116-23.3-158.4-65.6C211.3 528 188 471.8 188 412s23.3-116.1 65.6-158.4C296 211.3 352.2 188 412 188s116.1 23.2 158.4 65.6S636 352.2 636 412s-23.3 116.1-65.6 158.4z"/></symbol>
  <symbol id="icon-info" viewBox="0 0 1024 1024"><path d="M512 64C264.6 64 64 264.6 64 512s200.6 448 448 448 448-200.6 448-448S759.4 64 512 64zm0 820c-205.4 0-372-166.6-372-372s166.6-372 372-372 372 166.6 372 372-166.6 372-372 372z"/><path d="M464 336a48 48 0 1 0 96 0 48 48 0 1 0-96 0zm72 112h-48c-4.4 0-8 3.6-8 8v272c0 4.4 3.6 8 8 8h48c4.4 0 8-3.6 8-8V456c0-4.4-3.6-8-8-8z"/></symbol>
  <symbol id="icon-reload" viewBox="0 0 1024 1024"><path d="M909.1 209.3l-56.4 44.1C775.8 155.1 656.2 92 521.9 92 290 92 102.3 279.5 102 511.5 101.7 743.7 289.8 932 521.9 932c186.7 0 344.9-120 403.4-285.4 9.1-25.7-10-53-37.3-53h-47.8c-18.4 0-34.6 11.8-39.5 29.5C776.1 739.1 659 824 521.9 824c-172.7 0-313.9-141.2-313.9-313.9S349.2 196.2 521.9 196.2c106.2 0 199.9 52.8 257.3 133.7L634 437.2c-14 11.2-6.4 32.8 12.8 32.8h201.3c9 0 16.9-5.4 20.2-13.6 5.7-14.3 2.3-31.3-8.4-42l-40.8-38.2z"/></symbol>
  <symbol id="icon-edit" viewBox="0 0 1024 1024"><path d="M257.7 752c2 0 4-.2 6-.5L431.9 722c2-.4 3.9-1.3 5.3-2.8l423.9-423.9a9.96 9.96 0 0 0 0-14.1L694.9 114.9c-1.9-1.9-4.4-2.9-7.1-2.9s-5.2 1-7.1 2.9L256.8 538.8c-1.5 1.5-2.4 3.3-2.8 5.3l-29.5 168.2a33.5 33.5 0 0 0 9.4 29.8c6.6 6.4 14.9 9.9 23.8 9.9zm67.4-174.4L687.8 215l73.3 73.3-362.7 362.6-88.9 15.7 15.6-89zM880 836H144c-17.7 0-32 14.3-32 32v36c0 4.4 3.6 8 8 8h784c4.4 0 8-3.6 8-8v-36c0-17.7-14.3-32-32-32z"/></symbol>
  <symbol id="icon-delete" viewBox="0 0 1024 1024"><path d="M360 184h-8c4.4 0 8-3.6 8-8v8h304v-8c0 4.4 3.6 8 8 8h-8v72h72v-80c0-35.3-28.7-64-64-64H352c-35.3 0-64 28.7-64 64v80h72v-72zm504 72H160c-17.7 0-32 14.3-32 32v32c0 4.4 3.6 8 8 8h60.4l24.7 523c1.6 34.1 29.8 61 63.9 61h454c34.2 0 62.3-26.8 63.9-61l24.7-523H888c4.4 0 8-3.6 8-8v-32c0-17.7-14.3-32-32-32zM731.3 840H292.7l-24.2-512h487l-24.2 512z"/></symbol>
  <symbol id="icon-tag" viewBox="0 0 1024 1024"><path d="M938 458.8l-29.6-29.2c-4.4-4.4-10.6-6.6-16.8-6.6h-235c-16.2 0-29.4 13.2-29.4 29.4 0 8.1 3.3 15.5 8.6 20.9l29.2 29.2c-1.5 2.2-2.9 4.5-4.3 6.8-24.6 43-66.9 76.4-117.6 89.8-70.6 18.7-145.4-8.7-191.3-70.2-36.1-48.4-45.7-111.1-25.6-168.3 20.9-59.4 71.9-104.3 132.5-116.9 41.7-8.7 84.6-1.9 119 18.7 11.8 7.1 22.5 15.8 31.9 25.8l56.1 56.1c3 3 7.1 4.7 11.3 4.7 4.4 0 8.5-1.8 11.5-4.8l120.6-120.6c3-3 4.7-7.1 4.7-11.5s-1.8-8.5-4.8-11.5L585.2 89.9C563 67.7 533.5 56 503.1 56c-23.1 0-45.7 6.5-65.3 18.8L95 277.8c-35.5 22.3-56.6 60.6-56.6 102.3 0 41.6 21.1 79.9 56.6 102.3l392.4 245.7c38.9 24.4 87.4 24.7 126.6.5l269.4-160.4c13.9-8.3 23-23.3 23-40 0-17.5-9-32.5-23.4-40.7zM640 328c-30.9 0-56-25.1-56-56s25.1-56 56-56 56 25.1 56 56-25.1 56-56 56z"/></symbol>
  <symbol id="icon-check-circle" viewBox="0 0 1024 1024"><path d="M699 353h-56.9c-4.9 0-9.2 3.1-11.3 7.6l-42.2 87.2-42.2-87.2c-2-4.5-6.4-7.6-11.3-7.6H472c-6.6 0-12 5.4-12 12v342c0 6.6 5.4 12 12 12h56c6.6 0 12-5.4 12-12V519.9l45.2 93.4c1.9 3.9 5.7 6.6 10 6.6h14.4c4.3 0 8.1-2.7 10-6.6l45.2-93.4V707c0 6.6 5.4 12 12 12h56c6.6 0 12-5.4 12-12V365c0-6.6-5.4-12-12-12zM512 64C264.6 64 64 264.6 64 512s200.6 448 448 448 448-200.6 448-448S759.4 64 512 64zm0 820c-205.4 0-372-166.6-372-372s166.6-372 372-372 372 166.6 372 372-166.6 372-372 372z"/></symbol>
  <symbol id="icon-warning" viewBox="0 0 1024 1024"><path d="M464 720a48 48 0 1 0 96 0 48 48 0 1 0-96 0zm16-304v184c0 4.4 3.6 8 8 8h48c4.4 0 8-3.6 8-8V416c0-4.4-3.6-8-8-8h-48c-4.4 0-8 3.6-8 8zm475.7 440l-416-720c-6.2-10.7-16.9-16-27.7-16s-21.6 5.3-27.7 16l-416 720C16 878.4 31.4 904 56 904h832c24.6 0 40-25.6 27.7-48zm-783.5-27.9L512 239.5l339.8 588.6H172.2z"/></symbol>
  <symbol id="icon-file-text" viewBox="0 0 1024 1024"><path d="M854.6 288.6L639.4 73.4c-6-6-14.1-9.4-22.6-9.4H192c-17.7 0-32 14.3-32 32v832c0 17.7 14.3 32 32 32h640c17.7 0 32-14.3 32-32V311.3c0-8.5-3.4-16.7-9.4-22.7zM790.2 326H602V137.8L790.2 326zm1.8 562H232V136h302v216a42 42 0 0 0 42 42h216v494zM504 618H320c-4.4 0-8 3.6-8 8v48c0 4.4 3.6 8 8 8h184c4.4 0 8-3.6 8-8v-48c0-4.4-3.6-8-8-8zM312 490v48c0 4.4 3.6 8 8 8h384c4.4 0 8-3.6 8-8v-48c0-4.4-3.6-8-8-8H320c-4.4 0-8 3.6-8 8z"/></symbol>
  <symbol id="icon-team" viewBox="0 0 1024 1024"><path d="M734 318c-61.9 0-112 50.1-112 112s50.1 112 112 112 112-50.1 112-112-50.1-112-112-112zm0 160c-26.5 0-48-21.5-48-48s21.5-48 48-48 48 21.5 48 48-21.5 48-48 48zm0-256c-79.5 0-144 64.5-144 144s64.5 144 144 144 144-64.5 144-144-64.5-144-144-144zm132.8 329.2c-14.3-10.9-30.1-19.4-47.1-25.3 17.3 21.7 27.3 48.4 27.3 77.1v40h64v-40c0-20.1-16-36.4-36.4-42.8-1.9-2.4-4.4-5.1-7.8-9zM290 318c-61.9 0-112 50.1-112 112s50.1 112 112 112 112-50.1 112-112-50.1-112-112-112zm0 160c-26.5 0-48-21.5-48-48s21.5-48 48-48 48 21.5 48 48-21.5 48-48 48zm0-256c-79.5 0-144 64.5-144 144s64.5 144 144 144 144-64.5 144-144-64.5-144-144-144zm132.8 329.2c-14.3-10.9-30.1-19.4-47.1-25.3 17.3 21.7 27.3 48.4 27.3 77.1v40h64v-40c0-20.1-16-36.4-36.4-42.8-1.9-2.4-4.4-5.1-7.8-9zM512 598c-79.5 0-144 64.5-144 144v56c0 4.4 3.6 8 8 8h272c4.4 0 8-3.6 8-8v-56c0-79.5-64.5-144-144-144zm96 160H416v-16c0-52.9 43.1-96 96-96s96 43.1 96 96v16z"/></symbol>
  <symbol id="icon-link" viewBox="0 0 1024 1024"><path d="M574 665.4c-3.5-.7-6.9-.9-10.4-.9-26.7 0-52.1 10.4-71.1 29.3L384 802.4c-39.2 39.2-102.9 39.2-142.1 0-39.2-39.2-39.2-102.9 0-142.1l108.4-108.4c8.2-8.2 8.2-21.6 0-29.8L322 493.8c-8.2-8.2-21.6-8.2-29.8 0L183.8 602.2c-58.8 58.8-58.8 154.3 0 213.1 58.8 58.8 154.3 58.8 213.1 0L505.3 707c22.6-22.6 53.1-35.1 85.3-35.1 32.3 0 62.7 12.5 85.3 35.1l108.4 108.4c58.8 58.8 154.3 58.8 213.1 0 58.8-58.8 58.8-154.3 0-213.1L792 694.8c-8.2 8.2-21.6 8.2-29.8 0l-28.3-28.3c-8.2-8.2-8.2-21.6 0-29.8l-1.2-1.2c21.6-36.4 33.8-78.6 33.8-123.4 0-36.7-7.4-72-20.8-104.1 9.7-9.6 19-19.3 28.4-28.7l108.4-108.4c58.8-58.8 58.8-154.3 0-213.1s-154.3-58.8-213.1 0L634.3 319c-22.6 22.6-35.1 53.1-35.1 85.3s12.5 62.7 35.1 85.3l28.3 28.3c3.5 3.5 5.4 8.2 5.4 13.2 0 5-1.9 9.7-5.4 13.2l-22.7 22.7c-3.5 3.5-8.2 5.4-13.2 5.4-5 0-9.7-1.9-13.2-5.4l-28.3-28.3c-22.6-22.6-53.1-35.1-85.3-35.1s-62.7 12.5-85.3 35.1L310.1 519.3c-8.2 8.2-8.2 21.6 0 29.8l28.3 28.3c8.2 8.2 21.6 8.2 29.8 0l28.3-28.3c22.6-22.6 53.1-35.1 85.3-35.1s62.7 12.5 85.3 35.1L634 636.8c3.5 3.5 5.4 8.2 5.4 13.2 0 5-1.9 9.7-5.4 13.2l-60 60z"/></symbol>
  <symbol id="icon-arrow-up" viewBox="0 0 1024 1024"><path d="M862 465.3h-81c-4.6 0-9 2-12.1 5.5L550 723.1V160c0-4.4-3.6-8-8-8h-60c-4.4 0-8 3.6-8 8v563.1L255.1 470.8c-3-3.5-7.4-5.5-12.1-5.5h-81c-6.8 0-10.5 8.1-6 13.2L487.9 861a31.96 31.96 0 0 0 48.3 0L868 478.5c4.5-5.2.8-13.2-6-13.2z"/></symbol>
</svg>

<div class="wrap">
  <div class="header">
    <div class="logo"><svg class="icon" aria-hidden="true"><use href="#icon-key"/></svg></div>
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
        <input id="product" value="hxapigate" list="productList" placeholder="例如: hxapigate">
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
        <input id="customer" list="customerList" placeholder="例如: 公司一（已有客户可下拉选择，自动带出最近签发参数）" oninput="onCustomerInput()">
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
      <svg class="icon" aria-hidden="true"><use href="#icon-info"/></svg> 生成后请将 <code>license.json</code> 发给客户，客户上传到 licen-server：
      <code>curl -X POST http://&lt;server&gt;:&lt;port&gt;/api/v1/activate -d @license.json -H "Content-Type: application/json"</code><br>
      上传成功即激活全部功能；License 与客户机器码强绑定，拷到其他机器无效（MACHINE_MISMATCH）。
    </p>
  </div>

  <div class="result" id="result"></div>

  <div class="alert" id="expireAlert"></div>

  <div class="card">
    <h2><svg class="icon" aria-hidden="true"><use href="#icon-table"/></svg> 已签发授权台账</h2>
    <div class="toolbar">
      <span class="search-wrap"><svg class="icon" aria-hidden="true"><use href="#icon-search"/></svg><input id="searchBox" placeholder="搜索客户 / 产品 / License ID" oninput="renderLicenses()"></span>
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
      <svg class="icon" aria-hidden="true"><use href="#icon-info"/></svg> 状态说明：<span class="badge valid">有效</span> 正常授权
      · <span class="badge expiring">即将到期</span> 30 天内到期，请及时续期
      · <span class="badge expired">已过期</span> 超过到期时间
      · <span class="badge revoked">已吊销</span> 已作废（可「重新签发」生成新 License 续期/替换）
      <br><svg class="icon" aria-hidden="true"><use href="#icon-reload"/></svg> 重新签发：用原参数（客户/产品/节点/功能点/机器码）生成新 License，有效期从今天重新起算，旧 License 自动标记吊销。
      <br><svg class="icon" aria-hidden="true"><use href="#icon-history"/></svg> 时序：点击「时序」查看该授权从最初签发至今的完整续签链（每次续签/吊销留痕）。
      <br><svg class="icon" aria-hidden="true"><use href="#icon-tag"/></svg> 预填：选择已有客户自动带出该客户最近一次签发参数（产品/版本/节点/功能点/机器码），仅需修改差异项。
    </p>
  </div>

  <div class="card">
    <h2><svg class="icon" aria-hidden="true"><use href="#icon-appstore"/></svg> 产品库 &amp; SDK 下载</h2>
    <div class="toolbar">
      <button class="op-btn" onclick="showProductForm()"><svg class="icon" aria-hidden="true"><use href="#icon-plus"/></svg> 新建产品</button>
      <span class="count" id="prodCount"></span>
    </div>
    <div id="prodTableWrap">
      <table>
        <thead>
          <tr>
            <th>产品 ID</th>
            <th>名称</th>
            <th>描述</th>
            <th>授权数</th>
            <th>支持 SDK</th>
            <th>操作</th>
          </tr>
        </thead>
        <tbody id="prodTable"></tbody>
      </table>
      <div class="empty" id="prodEmpty" style="display:none">暂无产品（签发 License 时会自动登记产品）</div>
    </div>
    <p class="tip">
      <svg class="icon" aria-hidden="true"><use href="#icon-info"/></svg> 产品 ID 与 License 的 <code>product</code> 字段一致，是授权的唯一标识。
      <br><svg class="icon" aria-hidden="true"><use href="#icon-download"/></svg> SDK 包内嵌了全部源码与示例，下载时<b>选择产品</b>：下载的 SDK 已内置该产品标识（注册/心跳自动携带，服务端三层校验：SDK 声明 == App 凭证 == License 签名授权）。
      <br><svg class="icon" aria-hidden="true"><use href="#icon-folder"/></svg> 下载时选择客户，SDK 副本与签发的证书会自动归档到「客户档案」，按 客户/产品 随时调取。
    </p>
  </div>

  <div class="card">
    <h2><svg class="icon" aria-hidden="true"><use href="#icon-team"/></svg> 客户-产品对应</h2>
    <div class="toolbar">
      <select id="cpCustomer" onchange="renderCP()">
        <option value="">— 选择客户 —</option>
      </select>
      <span>
        <button class="op-btn" onclick="showCustomerForm()"><svg class="icon" aria-hidden="true"><use href="#icon-plus"/></svg> 新建客户</button>
        <button class="op-btn" id="btnBindProduct" onclick="showBindForm()" style="display:none"><svg class="icon" aria-hidden="true"><use href="#icon-link"/></svg> 绑定产品</button>
        <button class="op-btn danger" id="btnDelCustomer" onclick="deleteCustomer()" style="display:none">删除客户</button>
        <span class="count" id="cpCount"></span>
      </span>
    </div>
    <div id="cpDetail"></div>
    <p class="tip">
      <svg class="icon" aria-hidden="true"><use href="#icon-info"/></svg> 维护「客户 × 产品」对应关系（节点上限 / 版本 / 状态）。
      签发 License 时自动登记对应关系；被绑定或有授权记录的产品/客户禁止删除。
    </p>
  </div>

  <div class="card">
    <h2><svg class="icon" aria-hidden="true"><use href="#icon-folder"/></svg> 客户档案（按客户归档）</h2>
    <div class="toolbar">
      <select id="archiveCustomer" onchange="renderArchive()">
        <option value="">— 选择客户 —</option>
      </select>
      <span class="count" id="archiveCount"></span>
    </div>
    <div id="archiveBody">
      <div class="empty"><svg class="icon" aria-hidden="true"><use href="#icon-arrow-up"/></svg> 选择客户，查看其名下的产品 SDK 与证书归档</div>
    </div>
    <p class="tip">
      <svg class="icon" aria-hidden="true"><use href="#icon-info"/></svg> 同一客户使用的产品 SDK 与证书按 <code>客户 / 产品</code> 归档：
      签发的 License 证书落盘 <code>archive/{客户}/{产品}/licenses/</code>，
      下载的定制 SDK 落盘 <code>archive/{客户}/{产品}/sdk/</code>，可随时下载回传交付。
    </p>
  </div>
</div>

<!-- 产品编辑弹窗 -->
<div class="modal-mask" id="prodMask" onclick="if(event.target===this)closeProdForm()">
  <div class="modal" style="width:520px">
    <span class="close-x" onclick="closeProdForm()"><svg class="icon" aria-hidden="true"><use href="#icon-close"/></svg></span>
    <h3 id="prodFormTitle"><svg class="icon" aria-hidden="true"><use href="#icon-plus"/></svg> 新建产品</h3>
    <div class="grid">
      <div class="field">
        <label>产品 ID <span style="color:#d92d20">*</span>（= License.product，仅字母/数字/-/_/.）</label>
        <input id="pId" placeholder="例如: hxapigate">
      </div>
      <div class="field">
        <label>产品名称</label>
        <input id="pName" placeholder="例如: AI 推理引擎">
      </div>
      <div class="field full">
        <label>产品描述</label>
        <input id="pDesc" placeholder="例如: 私有化大模型推理服务">
      </div>
      <div class="field full">
        <label>支持 SDK（勾选可下载的语言）</label>
        <div style="display:flex;gap:16px;padding-top:6px">
          <label style="font-size:13px"><input type="checkbox" class="pSdk" value="go" checked> Go</label>
          <label style="font-size:13px"><input type="checkbox" class="pSdk" value="java" checked> Java</label>
          <label style="font-size:13px"><input type="checkbox" class="pSdk" value="python" checked> Python</label>
          <label style="font-size:13px"><input type="checkbox" class="pSdk" value="c" checked> C</label>
        </div>
      </div>
    </div>
    <div class="row">
      <button class="btn btn-primary" onclick="saveProduct()">保存</button>
      <button class="btn btn-ghost" onclick="closeProdForm()">取消</button>
    </div>
  </div>
</div>

<!-- 客户表单弹窗（新建/编辑） -->
<div class="modal-mask" id="custMask" onclick="if(event.target===this)closeCustomerForm()">
  <div class="modal" style="width:560px">
    <span class="close-x" onclick="closeCustomerForm()"><svg class="icon" aria-hidden="true"><use href="#icon-close"/></svg></span>
    <h3 id="custFormTitle"><svg class="icon" aria-hidden="true"><use href="#icon-plus"/></svg> 新建客户</h3>
    <div class="grid">
      <div class="field">
        <label>客户名称 <span style="color:#d92d20">*</span>（与签发时 License 的 customer 一致）</label>
        <input id="cName" placeholder="例如: 公司一">
      </div>
      <div class="field">
        <label>联系人</label>
        <input id="cContact" placeholder="例如: 张工">
      </div>
      <div class="field">
        <label>电话</label>
        <input id="cPhone" placeholder="例如: 138****8888">
      </div>
      <div class="field">
        <label>邮箱</label>
        <input id="cEmail" placeholder="例如: zhang@example.com">
      </div>
      <div class="field full">
        <label>地址</label>
        <input id="cAddr" placeholder="例如: 北京市海淀区">
      </div>
      <div class="field full">
        <label>备注</label>
        <input id="cNote" placeholder="可选">
      </div>
    </div>
    <div class="row">
      <button class="btn btn-primary" onclick="saveCustomer()">保存</button>
      <button class="btn btn-ghost" onclick="closeCustomerForm()">取消</button>
    </div>
  </div>
</div>

<!-- 绑定产品弹窗（新增/编辑） -->
<div class="modal-mask" id="bindMask" onclick="if(event.target===this)closeBindForm()">
  <div class="modal" style="width:560px">
    <span class="close-x" onclick="closeBindForm()"><svg class="icon" aria-hidden="true"><use href="#icon-close"/></svg></span>
    <h3 id="bindFormTitle"><svg class="icon" aria-hidden="true"><use href="#icon-link"/></svg> 绑定产品</h3>
    <div class="grid">
      <div class="field">
        <label>产品 <span style="color:#d92d20">*</span></label>
        <input id="bProduct" list="bindProductList" placeholder="选择产品（来自产品库）">
        <datalist id="bindProductList"></datalist>
      </div>
      <div class="field">
        <label>版本/套餐</label>
        <input id="bEdition" value="enterprise" placeholder="enterprise">
      </div>
      <div class="field">
        <label>节点上限</label>
        <input id="bMaxNodes" type="number" value="50" min="1" placeholder="0=不限制">
      </div>
      <div class="field">
        <label>状态</label>
        <select id="bStatus">
          <option value="active">有效</option>
          <option value="paused">暂停</option>
          <option value="terminated">终止</option>
        </select>
      </div>
      <div class="field full">
        <label>备注</label>
        <input id="bNote" placeholder="可选">
      </div>
    </div>
    <div class="row">
      <button class="btn btn-primary" onclick="saveBind()">保存</button>
      <button class="btn btn-ghost" onclick="closeBindForm()">取消</button>
    </div>
  </div>
</div>

<!-- SDK 下载弹窗 -->
<div class="modal-mask" id="sdkMask" onclick="if(event.target===this)closeSdkDl()">
  <div class="modal" style="width:520px">
    <span class="close-x" onclick="closeSdkDl()"><svg class="icon" aria-hidden="true"><use href="#icon-close"/></svg></span>
    <h3><svg class="icon" aria-hidden="true"><use href="#icon-download"/></svg> 下载定制 SDK</h3>
    <div class="grid">
      <div class="field">
        <label>产品</label>
        <input id="sdkProduct" readonly>
      </div>
      <div class="field">
        <label>语言</label>
        <input id="sdkLang" readonly>
      </div>
      <div class="field full">
        <label>客户（可选：归档到客户档案）</label>
        <input id="sdkCustomer" list="archiveCustomerList" placeholder="选择或输入客户名称，留空 = 仅下载不归档">
        <datalist id="archiveCustomerList"></datalist>
      </div>
    </div>
    <p class="tip" style="margin-top:8px">定制版 SDK 已内置产品标识（注册/心跳自动携带）；填写客户后将同时归档 SDK 副本到客户档案。</p>
    <div class="row">
      <button class="btn btn-primary" onclick="doSdkDownload()"><svg class="icon" aria-hidden="true"><use href="#icon-download"/></svg> 下载并归档</button>
      <button class="btn btn-ghost" onclick="closeSdkDl()">取消</button>
    </div>
  </div>
</div>

<!-- 时间线弹窗 -->
<div class="modal-mask" id="tlMask" onclick="if(event.target===this)closeTl()">
  <div class="modal">
    <span class="close-x" onclick="closeTl()"><svg class="icon" aria-hidden="true"><use href="#icon-close"/></svg></span>
    <h3><svg class="icon" aria-hidden="true"><use href="#icon-history"/></svg> 授权时序</h3>
    <div id="tlBody"></div>
  </div>
</div>

<script>
// 图标渲染（内联 SVG sprite，antd 风格）
function icon(name, extra) {
  return '<svg class="icon' + (extra ? ' ' + extra : '') + '" aria-hidden="true"><use href="#icon-' + name + '"/></svg>';
}

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
      res.innerHTML = '<h3>' + icon('close') + ' 签发失败</h3><div>' + escapeHtml(data.message || '未知错误') + '</div>';
    } else {
      res.className = 'result ok';
      res.innerHTML =
        '<h3>' + icon('check-circle') + ' 签发成功</h3>' +
        '<pre>' + escapeHtml(data.licenseJson) + '</pre>' +
        '<a class="dl" download="license.json" href="data:text/plain;charset=utf-8,' + encodeURIComponent(data.licenseJson) + '">' + icon('download') + ' 下载 license.json</a>';
    }
  } catch (e) {
    res.className = 'result err';
    res.innerHTML = '<h3>' + icon('close') + ' 请求失败</h3><div>' + escapeHtml(String(e)) + '</div>';
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
  if (expiring > 0) parts.push(icon('warning') + ' <b>' + expiring + '</b> 条授权 30 天内到期，请及时续期');
  if (expired > 0) parts.push('<span class="expired-alert"><span class="dot red"></span><b>' + expired + '</b> 条授权已过期</span>');
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
  hint.innerHTML = icon('file-text') + ' 已识别客户：共 ' + c.licenses + ' 条授权（有效 ' + c.activeCount + '，其中 ' + c.expiringCount + ' 条即将到期），已预填最近签发参数：' +
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
    '<span class="dot green"></span><b>' + licStats.valid + '</b> 有效'
    + '&nbsp;<span class="dot orange"></span><b>' + licStats.expiring + '</b> 即将到期'
    + '&nbsp;<span class="dot red"></span><b>' + licStats.expired + '</b> 已过期'
    + '&nbsp;<span class="dot gray"></span><b>' + licStats.revoked + '</b> 已吊销';
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

// ---------- 产品库 & SDK ----------
let productCache = [];
let sdkLangNames = {};

async function loadProducts() {
  try {
    const token = sessionStorage.getItem('issuerToken') || '';
    const resp = await fetch('/api/v1/products', { headers: { 'X-Issuer-Token': token } });
    if (resp.status !== 200) return;
    const data = await resp.json();
    if (!data.success) return;
    productCache = data.products || [];
    sdkLangNames = data.sdkLanguages || {};
    renderProducts();
    initProductDatalist();
  } catch (e) { /* 静默 */ }
}

function renderProducts() {
  const tbody = document.getElementById('prodTable');
  const empty = document.getElementById('prodEmpty');
  document.getElementById('prodCount').textContent = '共 ' + productCache.length + ' 个产品';
  if (productCache.length === 0) {
    tbody.innerHTML = '';
    empty.style.display = 'block';
    return;
  }
  empty.style.display = 'none';
  tbody.innerHTML = productCache.map(p => {
    const sdks = (p.sdks || []).map(l => {
      const name = sdkLangNames[l] || l;
      return '<button class="op-btn" onclick="openSdkDl(\'' + l + '\',\'' + escapeHtml(p.id) + '\')" title="下载 ' + name + ' 定制版（内置产品标识）">' + icon('download') + ' ' + l + '</button>';
    }).join(' ');
    return '<tr>'
      + '<td class="mono" style="font-weight:600">' + escapeHtml(p.id) + '</td>'
      + '<td>' + escapeHtml(p.name || '-') + '</td>'
      + '<td>' + escapeHtml(p.description || '-') + '</td>'
      + '<td>' + (p.licenseCount || 0) + '</td>'
      + '<td>' + sdks + '</td>'
      + '<td><button class="op-btn" onclick="editProduct(\'' + escapeHtml(p.id) + '\')">编辑</button>'
      + '<button class="op-btn danger" onclick="deleteProduct(\'' + escapeHtml(p.id) + '\')">删除</button></td>'
      + '</tr>';
  }).join('');
}

function showProductForm(id) {
  document.getElementById('pId').disabled = !!id;
  document.getElementById('pId').value = '';
  document.getElementById('pName').value = '';
  document.getElementById('pDesc').value = '';
  document.querySelectorAll('.pSdk').forEach(c => c.checked = true);
  document.getElementById('prodFormTitle').innerHTML = id ? icon('edit') + ' 编辑产品' : icon('plus') + ' 新建产品';
  if (id) {
    const p = productCache.find(x => x.id === id);
    if (p) {
      document.getElementById('pId').value = p.id;
      document.getElementById('pName').value = p.name || '';
      document.getElementById('pDesc').value = p.description || '';
      document.querySelectorAll('.pSdk').forEach(c => c.checked = (p.sdks || []).includes(c.value));
    }
  }
  document.getElementById('prodMask').className = 'modal-mask show';
}

function closeProdForm() {
  document.getElementById('prodMask').className = 'modal-mask';
}

async function saveProduct() {
  const id = document.getElementById('pId').value.trim();
  if (!id) { alert('产品 ID 不能为空'); return; }
  const sdks = [...document.querySelectorAll('.pSdk:checked')].map(c => c.value);
  const body = {
    id: id,
    name: document.getElementById('pName').value.trim(),
    description: document.getElementById('pDesc').value.trim(),
    sdks: sdks
  };
  const isEdit = document.getElementById('pId').disabled;
  try {
    const resp = await fetch('/api/v1/products' + (isEdit ? '/' + encodeURIComponent(id) : ''), {
      method: isEdit ? 'PUT' : 'POST',
      headers: { 'Content-Type': 'application/json', 'X-Issuer-Token': getToken() },
      body: JSON.stringify(body)
    });
    const data = await resp.json();
    alert(data.message || (data.success ? '保存成功' : '保存失败'));
    if (data.success) { closeProdForm(); await loadProducts(); }
  } catch (e) { alert('请求失败: ' + e); }
}

async function deleteProduct(id) {
  if (!confirm('确定删除产品 ' + id + ' ？有授权记录的产品禁止删除。')) return;
  try {
    const resp = await fetch('/api/v1/products/' + encodeURIComponent(id), {
      method: 'DELETE',
      headers: { 'X-Issuer-Token': getToken() }
    });
    const data = await resp.json();
    alert(data.message || '删除失败');
    if (data.success) await loadProducts();
  } catch (e) { alert('请求失败: ' + e); }
}

// ---------- SDK 定制下载 ----------

let sdkDlLang = '', sdkDlProduct = '';

function openSdkDl(lang, product) {
  sdkDlLang = lang;
  sdkDlProduct = product;
  document.getElementById('sdkProduct').value = product;
  document.getElementById('sdkLang').value = (sdkLangNames[lang] || lang) + ' SDK';
  document.getElementById('sdkCustomer').value = '';
  document.getElementById('sdkMask').className = 'modal-mask show';
}

function closeSdkDl() {
  document.getElementById('sdkMask').className = 'modal-mask';
}

async function doSdkDownload() {
  const customer = document.getElementById('sdkCustomer').value.trim();
  let url = '/api/v1/sdk/' + encodeURIComponent(sdkDlLang) + '/download?product=' + encodeURIComponent(sdkDlProduct);
  if (customer) url += '&customer=' + encodeURIComponent(customer);
  const r = await fetch(url, { headers: { 'X-Issuer-Token': getToken() } });
  if (!r.ok) {
    const e = await r.json().catch(() => ({}));
    alert('下载失败: ' + (e.message || ('HTTP ' + r.status)));
    return;
  }
  const blob = await r.blob();
  const cd = r.headers.get('Content-Disposition') || '';
  const m = cd.match(/filename="?([^";]+)"?/);
  const filename = m ? m[1] : ('licen-sdk-' + sdkDlLang + '-1.0.0-' + sdkDlProduct + '.zip');
  const a = document.createElement('a');
  a.href = URL.createObjectURL(blob);
  a.download = filename;
  a.click();
  URL.revokeObjectURL(a.href);
  closeSdkDl();
  if (customer) await loadArchive();
}

// ---------- 客户档案 ----------

let archiveCache = [];

async function loadArchive() {
  try {
    const resp = await fetch('/api/v1/archive', { headers: { 'X-Issuer-Token': getToken() } });
    if (resp.status !== 200) return;
    const data = await resp.json();
    if (!data.success) return;
    archiveCache = data.customers || [];
    // 客户下拉（保留当前选择）
    const sel = document.getElementById('archiveCustomer');
    const cur = sel.value;
    sel.innerHTML = '<option value="">— 选择客户 —</option>'
      + archiveCache.map(c => '<option value="' + escapeHtml(c.customer) + '">' + escapeHtml(c.customer) + '</option>').join('');
    if (archiveCache.some(c => c.customer === cur)) sel.value = cur;
    // SDK 弹窗客户 datalist
    document.getElementById('archiveCustomerList').innerHTML = archiveCache.map(c => '<option value="' + escapeHtml(c.customer) + '">').join('');
    renderArchive();
  } catch (e) { /* 静默 */ }
}

function renderArchive() {
  const c = document.getElementById('archiveCustomer').value;
  const body = document.getElementById('archiveBody');
  document.getElementById('archiveCount').textContent = archiveCache.length ? '共 ' + archiveCache.length + ' 个客户' : '';
  if (!c) {
    body.innerHTML = '<div class="empty">' + icon('arrow-up') + ' 选择客户，查看其名下的产品 SDK 与证书归档</div>';
    return;
  }
  const cust = archiveCache.find(x => x.customer === c);
  if (!cust || !cust.products.length) {
    body.innerHTML = '<div class="empty">该客户暂无归档（签发带客户名的 License 或下载定制 SDK 时选择客户后自动归档）</div>';
    return;
  }
  body.innerHTML = cust.products.map(p => {
    const encC = encodeURIComponent(c), encP = encodeURIComponent(p.product);
    const lics = (p.licenses || []).map(l =>
      '<div class="arch-row">' + icon('history') + ' <span class="mono">' + escapeHtml(l.licenseId) + '</span> '
      + (l.revoked ? '<span class="badge revoked">已吊销</span>' : '<span class="badge valid">有效</span>')
      + '<span class="dim">' + escapeHtml(l.file) + ' · ' + fmtSize(l.size) + ' · ' + (l.modified || '').replace('T', ' ').slice(0, 16) + '</span>'
      + '<button class="op-btn" onclick="dlFile(\'/api/v1/archive/' + encC + '/' + encP + '/licenses/' + encodeURIComponent(l.file) + '\',\'' + l.file.replace(/'/g, "\\'") + '\')">' + icon('download') + ' 证书</button></div>'
    ).join('');
    const sdks = (p.sdks || []).map(s =>
      '<div class="arch-row">' + icon('appstore') + ' <span class="mono">' + escapeHtml(s.file) + '</span> '
      + '<span class="dim">(' + (s.lang || '-') + ' · ' + fmtSize(s.size) + ' · ' + (s.modified || '').replace('T', ' ').slice(0, 16) + ')</span>'
      + '<button class="op-btn" onclick="dlFile(\'/api/v1/archive/' + encC + '/' + encP + '/sdk/' + encodeURIComponent(s.file) + '\',\'' + s.file.replace(/'/g, "\\'") + '\')">' + icon('download') + ' SDK</button></div>'
    ).join('');
    return '<div class="arch-prod"><h4>' + icon('appstore') + ' ' + escapeHtml(p.product) + '</h4>'
      + (lics ? '<div style="margin:2px 0 4px;font-size:12px;color:#555"><b>证书（' + (p.licenses || []).length + '）</b></div>' + lics : '')
      + (sdks ? '<div style="margin:8px 0 4px;font-size:12px;color:#555"><b>SDK 归档（' + (p.sdks || []).length + '）</b></div>' + sdks : '')
      + '</div>';
  }).join('');
}

// 带鉴权头下载归档文件（证书/SDK 副本）
async function dlFile(url, filename) {
  try {
    const r = await fetch(url, { headers: { 'X-Issuer-Token': getToken() } });
    if (!r.ok) { alert('下载失败: HTTP ' + r.status); return; }
    const blob = await r.blob();
    const a = document.createElement('a');
    a.href = URL.createObjectURL(blob);
    a.download = filename;
    a.click();
    URL.revokeObjectURL(a.href);
  } catch (e) { alert('下载失败: ' + e); }
}

function fmtSize(n) {
  if (n < 1024) return n + 'B';
  if (n < 1048576) return (n / 1024).toFixed(1) + 'KB';
  return (n / 1048576).toFixed(1) + 'MB';
}

// 产品下拉（签发表单 product 改为从产品库选择）
function initProductDatalist() {
  const ids = productCache.map(p => p.id).filter(Boolean);
  document.getElementById('productList').innerHTML = ids.map(id => '<option value="' + escapeHtml(id) + '">').join('');
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
      res.innerHTML = '<h3>' + icon('check-circle') + ' 重新签发成功（新 License）</h3>'
        + '<pre>' + escapeHtml(data.licenseJson || '') + '</pre>'
        + '<a class="dl" download="license.json" href="data:text/plain;charset=utf-8,' + encodeURIComponent(data.licenseJson || '') + '">' + icon('download') + ' 下载 license.json</a>';
    } else {
      alert(data.message || '重新签发失败');
    }
    await loadLicenses();
  } catch (e) { alert('请求失败: ' + e); }
}

function downloadLic() {
  alert('台账仅存授权记录（不含签名文件）。如需再次下载完整 license.json，请使用「重新签发」生成新 License。');
}


// ---------- 客户-产品对应 ----------

let cpCache = [];

async function loadCP() {
  try {
    const resp = await fetch('/api/v1/customer-products', { headers: { 'X-Issuer-Token': getToken() } });
    if (resp.status !== 200) return;
    const data = await resp.json();
    if (!data.success) return;
    cpCache = data.customers || [];
    // 客户下拉（保留当前选择）
    const sel = document.getElementById('cpCustomer');
    const cur = sel.value;
    sel.innerHTML = '<option value="">— 选择客户 —</option>'
      + cpCache.map(c => '<option value="' + escapeHtml(c.customer) + '">' + escapeHtml(c.customer) + '（授权 ' + (c.totalLicenses || 0) + ' / 有效 ' + (c.activeLicenses || 0) + '）</option>').join('');
    if (cpCache.some(c => c.customer === cur)) sel.value = cur;
    // 绑定弹窗产品下拉（来自产品库）
    document.getElementById('bindProductList').innerHTML = productCache
      .map(p => '<option value="' + escapeHtml(p.id) + '">' + escapeHtml(p.name || p.id) + '</option>').join('');
    renderCP();
  } catch (e) { /* 静默 */ }
}

function renderCP() {
  const name = document.getElementById('cpCustomer').value;
  const detail = document.getElementById('cpDetail');
  document.getElementById('cpCount').textContent = cpCache.length ? '共 ' + cpCache.length + ' 个客户' : '';
  const btnBind = document.getElementById('btnBindProduct');
  const btnDel = document.getElementById('btnDelCustomer');
  if (!name) {
    detail.innerHTML = '<div class="empty"><svg class="icon" aria-hidden="true"><use href="#icon-arrow-up"/></svg> 选择客户，维护其产品绑定关系</div>';
    btnBind.style.display = 'none';
    btnDel.style.display = 'none';
    return;
  }
  const c = cpCache.find(x => x.customer === name);
  btnBind.style.display = 'inline-block';
  btnDel.style.display = 'inline-block';
  if (!c) { detail.innerHTML = '<div class="empty">客户不存在</div>'; return; }
  // 客户信息 + 绑定表
  const info = '<div class="arch-prod" style="margin-bottom:10px">'
    + '<div style="font-size:13px;font-weight:600;color:#2563eb">' + escapeHtml(c.customer) + '</div>'
    + '<div class="dim" style="font-size:12px;color:#8a919f;margin-top:4px">'
    + '联系人: ' + escapeHtml(c.contact || '-') + ' · 电话: ' + escapeHtml(c.phone || '-')
    + ' · 邮箱: ' + escapeHtml(c.email || '-') + ' · 地址: ' + escapeHtml(c.address || '-')
    + (c.note ? ' · 备注: ' + escapeHtml(c.note) : '')
    + ' <button class="op-btn" onclick="showCustomerForm(\'' + escapeHtml(c.customer) + '\')"><svg class="icon" aria-hidden="true"><use href="#icon-edit"/></svg> 编辑</button>'
    + '</div></div>';
  const binds = (c.products || []);
  if (binds.length === 0) {
    detail.innerHTML = info + '<div class="empty">该客户暂无绑定产品，点击「绑定产品」添加</div>';
    return;
  }
  const rows = binds.map(b => {
    const stMap = { active: '<span class="badge valid">有效</span>', paused: '<span class="badge expiring">暂停</span>', terminated: '<span class="badge revoked">终止</span>' };
    const prod = productCache.find(p => p.id === b.product);
    return '<tr>'
      + '<td class="mono" style="font-weight:600">' + escapeHtml(b.product) + '</td>'
      + '<td>' + escapeHtml(prod ? prod.name : '-') + '</td>'
      + '<td>' + escapeHtml(b.edition || 'enterprise') + '</td>'
      + '<td>' + (b.maxNodes > 0 ? b.maxNodes : '不限') + '</td>'
      + '<td>' + (stMap[b.status] || stMap.active) + '</td>'
      + '<td>' + (b.licenseCount || 0) + ' / ' + (b.activeCount || 0) + '</td>'
      + '<td><button class="op-btn" onclick="showBindForm(\'' + escapeHtml(b.product) + '\')"><svg class="icon" aria-hidden="true"><use href="#icon-edit"/></svg> 编辑</button>'
      + '<button class="op-btn danger" onclick="unbindProduct(\'' + escapeHtml(b.product) + '\')">解绑</button></td>'
      + '</tr>';
  }).join('');
  detail.innerHTML = info
    + '<table><thead><tr><th>产品</th><th>名称</th><th>版本</th><th>节点上限</th><th>状态</th><th>授权数/有效</th><th>操作</th></tr></thead>'
    + '<tbody>' + rows + '</tbody></table>';
}

// 客户表单
let cpEditName = '';
function showCustomerForm(name) {
  cpEditName = name || '';
  document.getElementById('cName').disabled = !!name;
  document.getElementById('cName').value = name || '';
  document.getElementById('cContact').value = '';
  document.getElementById('cPhone').value = '';
  document.getElementById('cEmail').value = '';
  document.getElementById('cAddr').value = '';
  document.getElementById('cNote').value = '';
  if (name) {
    const c = cpCache.find(x => x.customer === name);
    if (c) {
      document.getElementById('cContact').value = c.contact || '';
      document.getElementById('cPhone').value = c.phone || '';
      document.getElementById('cEmail').value = c.email || '';
      document.getElementById('cAddr').value = c.address || '';
      document.getElementById('cNote').value = c.note || '';
    }
  }
  document.getElementById('custFormTitle').innerHTML = icon(name ? 'edit' : 'plus') + (name ? ' 编辑客户' : ' 新建客户');
  document.getElementById('custMask').className = 'modal-mask show';
}
function closeCustomerForm() { document.getElementById('custMask').className = 'modal-mask'; }
async function saveCustomer() {
  const name = document.getElementById('cName').value.trim();
  if (!name) { alert('客户名称不能为空'); return; }
  const body = {
    customer: name,
    contact: document.getElementById('cContact').value.trim(),
    phone: document.getElementById('cPhone').value.trim(),
    email: document.getElementById('cEmail').value.trim(),
    address: document.getElementById('cAddr').value.trim(),
    note: document.getElementById('cNote').value.trim()
  };
  const isEdit = !!cpEditName;
  try {
    const resp = await fetch('/api/v1/customer-products' + (isEdit ? '/' + encodeURIComponent(cpEditName) : ''), {
      method: isEdit ? 'PUT' : 'POST',
      headers: { 'Content-Type': 'application/json', 'X-Issuer-Token': getToken() },
      body: JSON.stringify(body)
    });
    const data = await resp.json();
    alert(data.message || (data.success ? '保存成功' : '保存失败'));
    if (data.success) { closeCustomerForm(); await loadCP(); }
  } catch (e) { alert('请求失败: ' + e); }
}
async function deleteCustomer() {
  const name = document.getElementById('cpCustomer').value;
  if (!name) return;
  if (!confirm('确定删除客户 ' + name + ' ？有授权记录的客户禁止删除。')) return;
  try {
    const resp = await fetch('/api/v1/customer-products/' + encodeURIComponent(name), {
      method: 'DELETE', headers: { 'X-Issuer-Token': getToken() }
    });
    const data = await resp.json();
    alert(data.message || '删除失败');
    if (data.success) { document.getElementById('cpCustomer').value = ''; await loadCP(); }
  } catch (e) { alert('请求失败: ' + e); }
}

// 绑定表单
let bindEditProduct = '';
function showBindForm(product) {
  const cust = document.getElementById('cpCustomer').value;
  if (!cust) { alert('请先选择客户'); return; }
  bindEditProduct = product || '';
  document.getElementById('bProduct').disabled = !!product;
  document.getElementById('bProduct').value = product || '';
  document.getElementById('bEdition').value = 'enterprise';
  document.getElementById('bMaxNodes').value = 50;
  document.getElementById('bStatus').value = 'active';
  document.getElementById('bNote').value = '';
  if (product) {
    const c = cpCache.find(x => x.customer === cust);
    const b = c && c.products.find(x => x.product === product);
    if (b) {
      document.getElementById('bEdition').value = b.edition || 'enterprise';
      document.getElementById('bMaxNodes').value = b.maxNodes > 0 ? b.maxNodes : 50;
      document.getElementById('bStatus').value = b.status || 'active';
      document.getElementById('bNote').value = b.note || '';
    }
  }
  document.getElementById('bindFormTitle').innerHTML = icon(product ? 'edit' : 'link') + (product ? ' 编辑绑定' : ' 绑定产品');
  document.getElementById('bindMask').className = 'modal-mask show';
}
function closeBindForm() { document.getElementById('bindMask').className = 'modal-mask'; }
async function saveBind() {
  const cust = document.getElementById('cpCustomer').value;
  const product = document.getElementById('bProduct').value.trim();
  if (!cust || !product) { alert('客户与产品不能为空'); return; }
  const body = {
    product: product,
    edition: document.getElementById('bEdition').value.trim() || 'enterprise',
    maxNodes: parseInt(document.getElementById('bMaxNodes').value) || 0,
    status: document.getElementById('bStatus').value,
    note: document.getElementById('bNote').value.trim()
  };
  const isEdit = !!bindEditProduct;
  const url = '/api/v1/customer-products/' + encodeURIComponent(cust) + '/products'
    + (isEdit ? '/' + encodeURIComponent(bindEditProduct) : '');
  try {
    const resp = await fetch(url, {
      method: isEdit ? 'PUT' : 'POST',
      headers: { 'Content-Type': 'application/json', 'X-Issuer-Token': getToken() },
      body: JSON.stringify(body)
    });
    const data = await resp.json();
    alert(data.message || (data.success ? '保存成功' : '保存失败'));
    if (data.success) { closeBindForm(); await loadCP(); }
  } catch (e) { alert('请求失败: ' + e); }
}
async function unbindProduct(product) {
  const cust = document.getElementById('cpCustomer').value;
  if (!confirm('确定解绑 ' + cust + ' × ' + product + ' ？该产品有授权记录时禁止解绑。')) return;
  try {
    const resp = await fetch('/api/v1/customer-products/' + encodeURIComponent(cust) + '/products/' + encodeURIComponent(product), {
      method: 'DELETE', headers: { 'X-Issuer-Token': getToken() }
    });
    const data = await resp.json();
    alert(data.message || '解绑失败');
    if (data.success) await loadCP();
  } catch (e) { alert('请求失败: ' + e); }
}

// 页面加载时拉取台账
loadLicenses();
loadProducts();
loadArchive();
loadCP();
</script>
</body>
</html>`
