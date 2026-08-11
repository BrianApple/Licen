// licen-issuer：厂商侧 License 签发服务（Web 单二进制）。
//
// 厂商部署后，客户把「机器码」发过来，通过 Web 界面或 API 一键签发 License，
// 生成的 license.json 回传给客户，客户上传到 licen-server 即激活全部功能。
//
// 用法：
//
//	licen-issuer -c config.yaml
//
// 配置（config.yaml）：
//
//	server:
//	  port: 8099
//	issuer:
//	  private-key-file: ./keys/private.pem   # 厂商私钥（必填，勿外泄）
//	  admin-token:       change-me           # 签发 API 鉴权 Token（必填）
package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/BrianApple/Licen/internal/license"
	"gopkg.in/yaml.v3"
)

// ---------- 配置 ----------

type Config struct {
	Server struct {
		Port int `yaml:"port"`
	} `yaml:"server"`
	Issuer struct {
		PrivateKeyFile string `yaml:"private-key-file"`
		AdminToken     string `yaml:"admin-token"`
	} `yaml:"issuer"`
}

func loadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	if cfg.Server.Port == 0 {
		cfg.Server.Port = 8099
	}
	return &cfg, nil
}

// ---------- 签发服务 ----------

type IssuerServer struct {
	priv       *rsa.PrivateKey
	adminToken string
}

func NewIssuer(priv *rsa.PrivateKey, adminToken string) *IssuerServer {
	return &IssuerServer{priv: priv, adminToken: adminToken}
}

// ---------- 签发请求/响应 ----------

type issueReq struct {
	MachineCode string   `json:"machineCode"` // 客户 VM 机器码（必填）
	Product     string   `json:"product"`     // 产品标识（必填）
	Edition     string   `json:"edition"`     // 版本/套餐，默认 enterprise
	MaxNodes    int      `json:"maxNodes"`    // 并发节点数，默认 1
	Features    []string `json:"features"`    // 功能点列表
	Days        int      `json:"days"`        // 有效期（天），与 ExpiresAt 二选一
	ExpiresAt   string   `json:"expiresAt"`   // 到期时间 ISO-8601
	Customer    string   `json:"customer"`    // 客户名称
}

type issueResp struct {
	Success bool           `json:"success"`
	Message string         `json:"message,omitempty"`
	License *license.Model `json:"license,omitempty"`
	// LicenseJSON 完整 license.json 文本（可直接保存为文件上传激活）
	LicenseJSON string `json:"licenseJson,omitempty"`
}

// issue 生成 License
func (s *IssuerServer) issue(req issueReq) (*license.Model, error) {
	if req.MachineCode == "" {
		return nil, fmt.Errorf("machineCode（机器码）不能为空")
	}
	if req.Product == "" {
		return nil, fmt.Errorf("product（产品标识）不能为空")
	}
	if req.Edition == "" {
		req.Edition = "enterprise"
	}
	if req.MaxNodes <= 0 {
		req.MaxNodes = 1
	}
	if req.Days <= 0 && req.ExpiresAt == "" {
		return nil, fmt.Errorf("必须指定 days（有效期天数）或 expiresAt（到期时间）")
	}

	m := &license.Model{
		LicenseID:   "LIC-" + randomID(8) + "-" + fmt.Sprint(time.Now().UnixMilli()),
		Product:     req.Product,
		Edition:     req.Edition,
		MachineCode: req.MachineCode,
		MaxNodes:    req.MaxNodes,
		Features:    req.Features,
		IssuedAt:    time.Now().Format(time.RFC3339Nano),
		Customer:    req.Customer,
	}
	if req.ExpiresAt != "" {
		t, err := time.Parse(time.RFC3339Nano, req.ExpiresAt)
		if err != nil {
			return nil, fmt.Errorf("expiresAt 格式错误（应为 ISO-8601 如 2027-08-11T00:00:00）: %w", err)
		}
		m.ExpiresAt = t.Format(time.RFC3339Nano)
	} else {
		m.ExpiresAt = time.Now().AddDate(0, 0, req.Days).Format(time.RFC3339Nano)
	}
	if err := m.SignWith(s.priv); err != nil {
		return nil, fmt.Errorf("签名失败: %w", err)
	}
	return m, nil
}

// ---------- HTTP ----------

// handler 返回 HTTP 处理器
func (s *IssuerServer) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", s.handleIndex) // Web UI
	mux.HandleFunc("GET /api/v1/health", s.handleHealth)
	mux.HandleFunc("POST /api/v1/issue", s.auth(s.handleIssue))
	mux.HandleFunc("POST /api/v1/issue-text", s.auth(s.handleIssueText)) // 兼容 form 提交
	return mux
}

// auth 签发 API 鉴权
func (s *IssuerServer) auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("X-Issuer-Token")
		if subtle.ConstantTimeCompare([]byte(token), []byte(s.adminToken)) != 1 {
			writeJSON(w, http.StatusUnauthorized, map[string]any{"success": false, "message": "签发 Token 无效"})
			return
		}
		next(w, r)
	}
}

func (s *IssuerServer) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "UP", "time": time.Now().UnixMilli()})
}

// handleIssue REST API 签发（JSON）
func (s *IssuerServer) handleIssue(w http.ResponseWriter, r *http.Request) {
	var req issueReq
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, issueResp{Success: false, Message: "请求体解析失败: " + err.Error()})
		return
	}
	s.doIssue(w, req)
}

// handleIssueText 兼容表单/文本提交（JSON body 或 machineCode=xxx 表单）
func (s *IssuerServer) handleIssueText(w http.ResponseWriter, r *http.Request) {
	req := issueReq{}
	ct := r.Header.Get("Content-Type")
	if strings.Contains(ct, "application/json") {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, issueResp{Success: false, Message: "请求体解析失败"})
			return
		}
	} else if err := r.ParseForm(); err != nil {
		writeJSON(w, http.StatusBadRequest, issueResp{Success: false, Message: "表单解析失败"})
		return
	} else {
		req.MachineCode = r.FormValue("machineCode")
		req.Product = r.FormValue("product")
		req.Edition = r.FormValue("edition")
		req.Customer = r.FormValue("customer")
		req.ExpiresAt = r.FormValue("expiresAt")
		fmt.Sscanf(r.FormValue("maxNodes"), "%d", &req.MaxNodes)
		fmt.Sscanf(r.FormValue("days"), "%d", &req.Days)
		if f := r.FormValue("features"); f != "" {
			for _, x := range strings.Split(f, ",") {
				if x = strings.TrimSpace(x); x != "" {
					req.Features = append(req.Features, x)
				}
			}
		}
	}
	s.doIssue(w, req)
}

func (s *IssuerServer) doIssue(w http.ResponseWriter, req issueReq) {
	m, err := s.issue(req)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, issueResp{Success: false, Message: err.Error()})
		return
	}
	// 序列化为可下载的 license.json
	jsonBytes, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, issueResp{Success: false, Message: "License 序列化失败"})
		return
	}
	slog.Info("🔑 License 已签发", "licenseId", m.LicenseID, "product", m.Product, "customer", m.Customer)
	writeJSON(w, http.StatusOK, issueResp{
		Success:     true,
		Message:     "License 签发成功",
		License:     m,
		LicenseJSON: string(jsonBytes),
	})
}

// ---------- Web UI ----------

func (s *IssuerServer) handleIndex(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(indexHTML))
}

// ---------- 工具 ----------

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func randomID(n int) string {
	const chars = "0123456789ABCDEF"
	b := make([]byte, n)
	randBytes := make([]byte, n)
	if _, err := rand.Read(randBytes); err != nil {
		for i := range b {
			b[i] = chars[(time.Now().UnixNano()>>(uint(i)*3))%16]
		}
		return string(b)
	}
	for i := range b {
		b[i] = chars[randBytes[i]%16]
	}
	return string(b)
}

// ---------- main ----------

func main() {
	configPath := flag.String("c", "config.yaml", "配置文件路径")
	flag.Parse()

	cfg, err := loadConfig(*configPath)
	if err != nil {
		slog.Error("配置加载失败", "err", err)
		os.Exit(1)
	}
	if cfg.Issuer.PrivateKeyFile == "" || cfg.Issuer.AdminToken == "" || cfg.Issuer.AdminToken == "change-me" {
		slog.Error("配置错误：必须设置 issuer.private-key-file（厂商私钥）和 issuer.admin-token（签发鉴权 Token，勿用默认值）")
		os.Exit(1)
	}
	priv, err := license.LoadPrivateKeyFile(cfg.Issuer.PrivateKeyFile)
	if err != nil {
		slog.Error("私钥加载失败", "err", err)
		os.Exit(1)
	}

	issuer := NewIssuer(priv, cfg.Issuer.AdminToken)
	httpServer := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Server.Port),
		Handler: issuer.handler(),
	}
	go func() {
		slog.Info("🚀 licen-issuer 启动（厂商签发服务）", "port", cfg.Server.Port)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("HTTP 服务异常退出", "err", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	slog.Info("🛑 正在停止...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = httpServer.Shutdown(ctx)
}
