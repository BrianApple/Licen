// Package api REST API（/api/v1/*），Go 1.22+ 标准库路由。
package api

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/BrianApple/Licen/internal/license"
	"github.com/BrianApple/Licen/internal/node"
	"github.com/BrianApple/Licen/internal/store"
)

// Server HTTP 服务
type Server struct {
	store      *store.Store
	licMgr     *license.Manager
	nodeSvc    *node.Service
	adminToken string
	mux        *http.ServeMux
}

// New 创建 API 服务并注册路由
func New(st *store.Store, licMgr *license.Manager, nodeSvc *node.Service, adminToken string) *Server {
	s := &Server{
		store:      st,
		licMgr:     licMgr,
		nodeSvc:    nodeSvc,
		adminToken: adminToken,
		mux:        http.NewServeMux(),
	}

	// 公开接口（license 未激活也可用：健康检查/机器码采集/状态/激活）
	s.mux.HandleFunc("GET /api/v1/health", s.handleHealth)
	s.mux.HandleFunc("GET /api/v1/machine-code", s.handleMachineCode)
	s.mux.HandleFunc("GET /api/v1/license/status", s.handleLicenseStatus)
	s.mux.HandleFunc("POST /api/v1/activate", s.handleActivate)

	// 业务接口（需 license 激活）
	s.mux.HandleFunc("POST /api/v1/nodes/register", s.requireActive(s.handleRegister))
	s.mux.HandleFunc("POST /api/v1/nodes/heartbeat", s.requireActive(s.handleHeartbeat))

	// 管理接口（X-Admin-Token 鉴权 + license 激活）
	s.mux.HandleFunc("GET /api/v1/admin/license/status", s.requireActive(s.admin(s.handleAdminLicenseStatus)))
	s.mux.HandleFunc("POST /api/v1/admin/license/reload", s.requireActive(s.admin(s.handleAdminLicenseReload)))
	s.mux.HandleFunc("GET /api/v1/admin/nodes", s.requireActive(s.admin(s.handleAdminNodes)))
	s.mux.HandleFunc("DELETE /api/v1/admin/nodes/{id}", s.requireActive(s.admin(s.handleAdminRevokeNode)))
	s.mux.HandleFunc("GET /api/v1/admin/apps", s.requireActive(s.admin(s.handleAdminListApps)))
	s.mux.HandleFunc("POST /api/v1/admin/apps", s.requireActive(s.admin(s.handleAdminCreateApp)))
	s.mux.HandleFunc("DELETE /api/v1/admin/apps/{id}", s.requireActive(s.admin(s.handleAdminDeleteApp)))
	s.mux.HandleFunc("GET /api/v1/admin/audits", s.requireActive(s.admin(s.handleAdminAudits)))

	// Web 管理平台（内嵌单文件页面）
	s.mux.HandleFunc("GET /admin", s.handleAdminPage)
	s.mux.HandleFunc("GET /admin/assets/{file}", s.handleAdminAsset)

	return s
}

// handleAdminPage 返回内置 Web 管理平台页面（Vue 3 + Ant Design Vue，静态资源走 /admin/assets/）
func (s *Server) handleAdminPage(w http.ResponseWriter, _ *http.Request) {
	page := adminHTML
	if tpl, err := assetsFS.ReadFile("root.template.html"); err == nil {
		// 将模板转义为 JS 单引号字符串后注入（模板含换行/引号，需转义）
		jsTpl := strings.ReplaceAll(string(tpl), `\`, `\\`)
		jsTpl = strings.ReplaceAll(jsTpl, `'`, `\'`)
		jsTpl = strings.ReplaceAll(jsTpl, "\r", "")
		jsTpl = strings.ReplaceAll(jsTpl, "\n", `\n`)
		page = strings.ReplaceAll(page, "__ROOT_TEMPLATE__", "'"+jsTpl+"'")
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(page))
}

// handleAdminAsset 提供内嵌的前端静态资源（react/antd/icons 等 UMD 文件）
func (s *Server) handleAdminAsset(w http.ResponseWriter, r *http.Request) {
	file := r.PathValue("file")
	if file == "" || file == ".." || len(file) > 80 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"success": false, "message": "文件无效"})
		return
	}
	data, err := assetsFS.ReadFile("assets/" + file)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"success": false, "message": "资源不存在"})
		return
	}
	ct := "application/javascript"
	switch {
	case strings.HasSuffix(file, ".css"):
		ct = "text/css; charset=utf-8"
	case strings.HasSuffix(file, ".js"):
		ct = "application/javascript; charset=utf-8"
	case strings.HasSuffix(file, ".svg"):
		ct = "image/svg+xml"
	}
	w.Header().Set("Content-Type", ct)
	w.Header().Set("Cache-Control", "public, max-age=86400")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

// Handler 返回 http.Handler
func (s *Server) Handler() http.Handler { return s.mux }

// requireActive License 激活门控：未激活时业务/管理接口一律拒绝
func (s *Server) requireActive(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.licMgr.IsValid() {
			writeJSON(w, http.StatusForbidden, map[string]any{
				"success": false,
				"code":    "LICENSE_NOT_ACTIVATED",
				"message": "License 未激活，仅可使用基础功能（机器码采集/状态查询/激活）。请将机器码发送给厂商获取 License 后调用 POST /api/v1/activate 激活",
			})
			return
		}
		next(w, r)
	}
}

// admin 管理端鉴权包装
func (s *Server) admin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("X-Admin-Token")
		if subtle.ConstantTimeCompare([]byte(token), []byte(s.adminToken)) != 1 {
			writeJSON(w, http.StatusUnauthorized, map[string]any{"success": false, "message": "管理Token无效"})
			return
		}
		next(w, r)
	}
}

// ---------- 公开接口 ----------

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":       "UP",
		"time":         time.Now().UnixMilli(),
		"licenseValid": s.licMgr.IsValid(),
	})
}

func (s *Server) handleMachineCode(w http.ResponseWriter, _ *http.Request) {
	hw := s.licMgr.Hardware()
	writeJSON(w, http.StatusOK, map[string]any{
		"machineCode": s.licMgr.MachineCode(),
		"hardware": map[string]string{
			"motherboardSerial": hw.MotherboardSerial,
			"cpuSerial":         hw.CPUSerial,
			"macAddress":        hw.MacAddress,
			"diskSerial":        hw.DiskSerial,
			"systemUUID":        hw.SystemUUID,
		},
		"hint": "请将此机器码发送给厂商用于签发 License",
	})
}

func (s *Server) handleLicenseStatus(w http.ResponseWriter, _ *http.Request) {
	body := map[string]any{
		"valid":       s.licMgr.IsValid(),
		"result":      s.licMgr.Result().String(),
		"machineCode": s.licMgr.MachineCode(),
		"onlineNodes": s.nodeSvc.OnlineCount(),
	}
	if lic := s.licMgr.License(); lic != nil {
		body["licenseId"] = lic.LicenseID
		body["product"] = lic.Product
		body["edition"] = lic.Edition
		body["customer"] = lic.Customer
		body["maxNodes"] = lic.MaxNodes
		body["features"] = lic.Features
		body["issuedAt"] = lic.IssuedAt
		body["expiresAt"] = lic.ExpiresAt
	}
	// 多产品模式：列出全部产品授权状态
	if prods := s.licMgr.Products(); len(prods) > 0 {
		list := make([]map[string]any, 0, len(prods))
		for _, p := range prods {
			item := map[string]any{
				"product": p.Product,
				"result":  p.Result.String(),
				"valid":   p.Result == license.Valid,
			}
			if p.License != nil {
				item["licenseId"] = p.License.LicenseID
				item["edition"] = p.License.Edition
				item["customer"] = p.License.Customer
				item["maxNodes"] = p.License.MaxNodes
				item["features"] = p.License.Features
				item["issuedAt"] = p.License.IssuedAt
				item["expiresAt"] = p.License.ExpiresAt
			}
			list = append(list, item)
		}
		body["products"] = list
	}
	writeJSON(w, http.StatusOK, body)
}

type registerReq struct {
	AppKey    string `json:"appKey"`
	AppSecret string `json:"appSecret"`
	NodeID    string `json:"nodeId"`
	NodeName  string `json:"nodeName"`
	IP        string `json:"ip"`
	Version   string `json:"version"`
}

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req registerReq
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"success": false, "message": "请求体解析失败"})
		return
	}
	if req.NodeID == "" {
		req.NodeID = newUUID()
	}
	result := s.nodeSvc.Register(req.AppKey, req.AppSecret, req.NodeID, req.NodeName, req.IP, req.Version)
	writeJSON(w, http.StatusOK, result)
}

type heartbeatReq struct {
	AppKey    string `json:"appKey"`
	NodeID    string `json:"nodeId"`
	Timestamp string `json:"timestamp"`
	Sign      string `json:"sign"`
}

func (s *Server) handleHeartbeat(w http.ResponseWriter, r *http.Request) {
	var req heartbeatReq
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"success": false, "message": "请求体解析失败"})
		return
	}
	result := s.nodeSvc.Heartbeat(req.NodeID, req.AppKey, req.Timestamp, req.Sign)
	writeJSON(w, http.StatusOK, result)
}

// ---------- 激活 ----------

type activateReq struct {
	// LicenseContent 支持两种上传方式：
	//   1) 请求体直接为 license.json 内容（{"licenseId":...,"sign":...}）
	//   2) 包装格式：{"licenseContent":"<license.json 原始字符串>"}
	LicenseContent string `json:"licenseContent"`
}

// handleActivate 上传 License 激活：验签 + 机器码 + 有效期通过后写入并生效。
// 不需要管理 Token——伪造风险由 RSA 签名 + 机器码绑定杜绝。
func (s *Server) handleActivate(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20)) // 1MB 上限
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"success": false, "message": "请求体读取失败"})
		return
	}

	content := body
	// 尝试识别包装格式
	var wrap activateReq
	if err := json.Unmarshal(body, &wrap); err == nil && wrap.LicenseContent != "" {
		content = []byte(wrap.LicenseContent)
	}

	result := s.licMgr.Activate(content)
	if result != license.Valid {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"success": false,
			"result":  result.String(),
			"message": "License 激活失败：" + result.String() + "（请确认 License 由厂商签发且绑定本机机器码）",
		})
		return
	}
	s.store.AuditLog("LICENSE_ACTIVATE", "result=VALID licenseId="+s.licMgr.License().LicenseID)
	lic := s.licMgr.License()
	writeJSON(w, http.StatusOK, map[string]any{
		"success":   true,
		"message":   "License 激活成功，全部功能已启用",
		"licenseId": lic.LicenseID,
		"product":   lic.Product,
		"edition":   lic.Edition,
		"customer":  lic.Customer,
		"maxNodes":  lic.MaxNodes,
		"features":  lic.Features,
		"expiresAt": lic.ExpiresAt,
	})
}

// ---------- 管理接口 ----------

func (s *Server) handleAdminLicenseStatus(w http.ResponseWriter, _ *http.Request) {
	body := map[string]any{
		"valid":       s.licMgr.IsValid(),
		"result":      s.licMgr.Result().String(),
		"machineCode": s.licMgr.MachineCode(),
		"license":     s.licMgr.License(),
	}
	if prods := s.licMgr.Products(); len(prods) > 0 {
		list := make([]map[string]any, 0, len(prods))
		for _, p := range prods {
			item := map[string]any{
				"product": p.Product,
				"result":  p.Result.String(),
				"valid":   p.Result == license.Valid,
			}
			if p.License != nil {
				item["licenseId"] = p.License.LicenseID
				item["edition"] = p.License.Edition
				item["customer"] = p.License.Customer
				item["maxNodes"] = p.License.MaxNodes
				item["features"] = p.License.Features
				item["issuedAt"] = p.License.IssuedAt
				item["expiresAt"] = p.License.ExpiresAt
			}
			list = append(list, item)
		}
		body["products"] = list
	}
	writeJSON(w, http.StatusOK, body)
}

func (s *Server) handleAdminLicenseReload(w http.ResponseWriter, _ *http.Request) {
	ok := s.licMgr.Reload()
	s.store.AuditLog("ADMIN_LICENSE_RELOAD", "result="+s.licMgr.Result().String())
	writeJSON(w, http.StatusOK, map[string]any{"success": ok, "result": s.licMgr.Result().String()})
}

func (s *Server) handleAdminNodes(w http.ResponseWriter, r *http.Request) {
	limit := queryInt(r, "size", 100)
	if limit > 500 {
		limit = 500
	}
	nodes, err := s.store.ListNodes(limit)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"success": false, "message": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, nodes)
}

func (s *Server) handleAdminRevokeNode(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"success": false, "message": "id 无效"})
		return
	}
	ok := s.nodeSvc.RevokeNode(id)
	writeJSON(w, http.StatusOK, map[string]any{"success": ok})
}

func (s *Server) handleAdminListApps(w http.ResponseWriter, _ *http.Request) {
	apps, err := s.store.ListApps()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"success": false, "message": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, apps)
}

type createAppReq struct {
	Name      string `json:"name"`
	Product   string `json:"product"`
	AppKey    string `json:"appKey"`
	AppSecret string `json:"appSecret"`
}

func (s *Server) handleAdminCreateApp(w http.ResponseWriter, r *http.Request) {
	var req createAppReq
	if err := decodeJSON(r, &req); err != nil || req.AppKey == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"success": false, "message": "appKey 不能为空"})
		return
	}
	if existing, _ := s.store.FindApp(req.AppKey); existing != nil {
		writeJSON(w, http.StatusOK, map[string]any{"success": false, "message": "appKey 已存在: " + req.AppKey})
		return
	}
	if req.AppSecret == "" {
		req.AppSecret = newUUID() + newUUID()
	}
	app := &store.App{
		AppKey:    req.AppKey,
		AppSecret: req.AppSecret,
		Product:   req.Product,
		Name:      req.Name,
		Enabled:   true,
	}
	if err := s.store.CreateApp(app); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"success": false, "message": err.Error()})
		return
	}
	s.store.AuditLog("ADMIN_APP_CREATE", "appKey="+req.AppKey)
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "app": app})
}

func (s *Server) handleAdminDeleteApp(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"success": false, "message": "id 无效"})
		return
	}
	err = s.store.DeleteApp(id)
	s.store.AuditLog("ADMIN_APP_DELETE", "id="+strconv.FormatInt(id, 10))
	writeJSON(w, http.StatusOK, map[string]any{"success": err == nil})
}

func (s *Server) handleAdminAudits(w http.ResponseWriter, r *http.Request) {
	limit := queryInt(r, "size", 100)
	if limit > 500 {
		limit = 500
	}
	audits, err := s.store.ListAudits(limit)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"success": false, "message": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, audits)
}

// ---------- 工具 ----------

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		slog.Error("JSON 写入失败", "err", err)
	}
}

func decodeJSON(r *http.Request, v any) error {
	dec := json.NewDecoder(r.Body)
	return dec.Decode(v)
}

func queryInt(r *http.Request, key string, def int) int {
	v := r.URL.Query().Get(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

// newUUID 生成 UUID v4
func newUUID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 36)
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
