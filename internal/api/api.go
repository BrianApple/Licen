// Package api REST API（/api/v1/*），Go 1.22+ 标准库路由。
package api

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
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

	// 公开接口
	s.mux.HandleFunc("GET /api/v1/health", s.handleHealth)
	s.mux.HandleFunc("GET /api/v1/machine-code", s.handleMachineCode)
	s.mux.HandleFunc("GET /api/v1/license/status", s.handleLicenseStatus)
	s.mux.HandleFunc("POST /api/v1/nodes/register", s.handleRegister)
	s.mux.HandleFunc("POST /api/v1/nodes/heartbeat", s.handleHeartbeat)

	// 管理接口（X-Admin-Token 鉴权）
	s.mux.HandleFunc("GET /api/v1/admin/license/status", s.admin(s.handleAdminLicenseStatus))
	s.mux.HandleFunc("POST /api/v1/admin/license/reload", s.admin(s.handleAdminLicenseReload))
	s.mux.HandleFunc("GET /api/v1/admin/nodes", s.admin(s.handleAdminNodes))
	s.mux.HandleFunc("DELETE /api/v1/admin/nodes/{id}", s.admin(s.handleAdminRevokeNode))
	s.mux.HandleFunc("GET /api/v1/admin/apps", s.admin(s.handleAdminListApps))
	s.mux.HandleFunc("POST /api/v1/admin/apps", s.admin(s.handleAdminCreateApp))
	s.mux.HandleFunc("DELETE /api/v1/admin/apps/{id}", s.admin(s.handleAdminDeleteApp))
	s.mux.HandleFunc("GET /api/v1/admin/audits", s.admin(s.handleAdminAudits))

	return s
}

// Handler 返回 http.Handler
func (s *Server) Handler() http.Handler { return s.mux }

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
	writeJSON(w, http.StatusOK, map[string]any{
		"machineCode": s.licMgr.MachineCode(),
		"hint":        "请将此机器码发送给厂商用于签发 License",
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

// ---------- 管理接口 ----------

func (s *Server) handleAdminLicenseStatus(w http.ResponseWriter, _ *http.Request) {
	body := map[string]any{
		"valid":       s.licMgr.IsValid(),
		"result":      s.licMgr.Result().String(),
		"machineCode": s.licMgr.MachineCode(),
		"license":     s.licMgr.License(),
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
