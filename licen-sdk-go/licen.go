// Package licen 客户端 SDK（Go）：注册授权服务、心跳保活、授权能力校验。
//
// 行为与 Java/Python/C SDK 一致（见 docs/protocol.md）：
//   - 启动注册，失败不阻塞（进入宽限期）
//   - 定时心跳（HMAC 签名），断联宽限期内视为有效
//   - 节点被清理自动重新注册（自愈）
//   - 提供 isValid / hasFeature 能力校验
//
// 零第三方依赖（仅标准库）。
package licen

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"sync"
	"time"
)

// Config SDK 配置
type Config struct {
	// 授权服务地址，如 http://10.0.0.10:8090
	ServerURL string
	// 应用标识（授权服务管理端创建）
	AppKey string
	// 应用密钥
	AppSecret string
	// 节点名称（默认本机 hostname）
	NodeName string
	// 心跳间隔（默认 30s）
	HeartbeatInterval time.Duration
	// 授权中心不可达宽限期（默认 300s）：超过后判定降级
	GracePeriod time.Duration
	// 连接超时（默认 3s）
	ConnectTimeout time.Duration
}

func (c *Config) fillDefaults() {
	if c.HeartbeatInterval <= 0 {
		c.HeartbeatInterval = 30 * time.Second
	}
	if c.GracePeriod <= 0 {
		c.GracePeriod = 300 * time.Second
	}
	if c.ConnectTimeout <= 0 {
		c.ConnectTimeout = 3 * time.Second
	}
	if c.NodeName == "" {
		c.NodeName = hostname()
	}
}

// Status 授权状态快照
type Status struct {
	Valid       bool     `json:"valid"`
	LicenseID   string   `json:"licenseId,omitempty"`
	ExpiresAt   string   `json:"expiresAt,omitempty"`
	MaxNodes    int      `json:"maxNodes"`
	OnlineNodes int      `json:"onlineNodes"`
	Features    []string `json:"features,omitempty"`
	LastSyncAt  time.Time
}

// Client Licen 客户端
type Client struct {
	cfg     Config
	nodeID  string
	http    *http.Client
	mu      sync.RWMutex
	status  Status
	degraded bool
	lastContact time.Time

	ctx    context.Context
	cancel context.CancelFunc
	done   chan struct{}
	wg     sync.WaitGroup

	onChange func(Status)
	onChangeMu sync.Mutex
}

// NewClient 创建客户端（不启动，需调用 Start）
func NewClient(cfg Config) (*Client, error) {
	if cfg.ServerURL == "" || cfg.AppKey == "" || cfg.AppSecret == "" {
		return nil, fmt.Errorf("licen: server-url / app-key / app-secret 为必填项")
	}
	cfg.fillDefaults()
	nodeID, err := uuid()
	if err != nil {
		return nil, fmt.Errorf("licen: 节点ID生成失败: %w", err)
	}
	c := &Client{
		cfg:    cfg,
		nodeID: nodeID,
		http: &http.Client{
			Timeout: cfg.ConnectTimeout,
		},
		done: make(chan struct{}),
	}
	return c, nil
}

// NodeID 客户端节点 ID
func (c *Client) NodeID() string { return c.nodeID }

// Start 启动：注册 + 定时心跳（非阻塞）
func (c *Client) Start(ctx context.Context) {
	c.ctx, c.cancel = context.WithCancel(ctx)
	// 首次注册（失败不阻断启动，进入宽限期重试）
	if err := c.register(); err != nil {
		c.markDegraded("注册失败: " + err.Error())
	} else {
		c.degraded = false
	}
	c.wg.Add(1)
	go c.heartbeatLoop()
}

// Stop 停止心跳
func (c *Client) Stop() {
	if c.cancel != nil {
		c.cancel()
	}
	select {
	case <-c.done:
	default:
		close(c.done)
	}
	c.wg.Wait()
}

// IsValid 是否持有有效授权（最近一次同步状态）
func (c *Client) IsValid() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.status.Valid
}

// HasFeature 是否拥有指定功能点
func (c *Client) HasFeature(feature string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if !c.status.Valid {
		return false
	}
	for _, f := range c.status.Features {
		if f == feature {
			return true
		}
	}
	return false
}

// IsDegraded 授权中心是否不可达且已过宽限期
func (c *Client) IsDegraded() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.degraded
}

// Status 当前授权状态
func (c *Client) Status() Status {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.status
}

// Refresh 主动向授权中心拉取最新状态
func (c *Client) Refresh() error {
	return c.refresh()
}

// OnStatusChange 注册状态变化回调（状态更新且与上次不同时触发）
func (c *Client) OnStatusChange(fn func(Status)) {
	c.onChangeMu.Lock()
	defer c.onChangeMu.Unlock()
	c.onChange = fn
}

// ---------- 内部实现 ----------

func (c *Client) heartbeatLoop() {
	defer c.wg.Done()
	ticker := time.NewTicker(c.cfg.HeartbeatInterval)
	defer ticker.Stop()
	for {
		select {
		case <-c.ctx.Done():
			return
		case <-c.done:
			return
		case <-ticker.C:
			if err := c.heartbeat(); err != nil {
				// 节点被清理/服务重启 → 自动重新注册（自愈）
				if err == errNodeNotFound {
					if rerr := c.register(); rerr == nil {
						c.mu.Lock()
						c.degraded = false
						c.mu.Unlock()
						continue
					}
				}
				if time.Since(c.lastContact) > c.cfg.GracePeriod {
					c.markDegraded("心跳连续失败: " + err.Error())
				}
			} else {
				c.mu.Lock()
				c.degraded = false
				c.mu.Unlock()
			}
		}
	}
}

var errNodeNotFound = fmt.Errorf("NODE_NOT_FOUND")

func (c *Client) register() error {
	body, _ := json.Marshal(map[string]string{
		"appKey":    c.cfg.AppKey,
		"appSecret": c.cfg.AppSecret,
		"nodeId":    c.nodeID,
		"nodeName":  c.cfg.NodeName,
		"ip":        localIP(),
		"version":   "licen-sdk-go-1.0.0",
	})
	resp, err := c.post("/api/v1/nodes/register", body)
	if err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("注册被拒绝: %s", resp.Message)
	}
	c.updateFromRegister(resp)
	c.lastContact = time.Now()
	_ = c.refresh() // 拉取完整状态（含 features）
	return nil
}

func (c *Client) heartbeat() error {
	ts := time.Now().UnixMilli()
	sign := hmacSHA256Hex(c.cfg.AppSecret, fmt.Sprintf("%s:%d", c.nodeID, ts))
	body, _ := json.Marshal(map[string]string{
		"appKey":    c.cfg.AppKey,
		"nodeId":    c.nodeID,
		"timestamp": fmt.Sprint(ts),
		"sign":      sign,
	})
	resp, err := c.post("/api/v1/nodes/heartbeat", body)
	if err != nil {
		return err
	}
	if !resp.Success {
		if resp.Message == "NODE_NOT_FOUND" {
			return errNodeNotFound
		}
		return fmt.Errorf("心跳被拒绝: %s", resp.Message)
	}
	c.lastContact = time.Now()
	c.updateFromRegister(resp)
	return nil
}

func (c *Client) refresh() error {
	resp, err := c.get("/api/v1/license/status")
	if err != nil {
		return err
	}
	c.mu.Lock()
	st := Status{LastSyncAt: time.Now()}
	_ = json.Unmarshal(resp, &st)
	changed := st.Valid != c.status.Valid || st.LicenseID != c.status.LicenseID
	c.status = st
	c.mu.Unlock()
	if changed {
		c.notifyChange(st)
	}
	return nil
}

// registerResponse 注册/心跳响应结构
type registerResponse struct {
	Success     bool   `json:"success"`
	Message     string `json:"message"`
	NodeID      string `json:"nodeId"`
	LicenseID   string `json:"licenseId"`
	ExpiresAt   string `json:"expiresAt"`
	OnlineNodes int    `json:"onlineNodes"`
	MaxNodes    int    `json:"maxNodes"`
}

func (c *Client) updateFromRegister(resp *registerResponse) {
	c.mu.Lock()
	changed := c.status.LicenseID != resp.LicenseID || c.status.Valid != resp.Success
	st := c.status
	st.Valid = resp.Success
	st.LicenseID = resp.LicenseID
	st.ExpiresAt = resp.ExpiresAt
	st.OnlineNodes = resp.OnlineNodes
	st.MaxNodes = resp.MaxNodes
	st.LastSyncAt = time.Now()
	c.status = st
	c.mu.Unlock()
	if changed {
		c.notifyChange(st)
	}
}

func (c *Client) notifyChange(st Status) {
	c.onChangeMu.Lock()
	fn := c.onChange
	c.onChangeMu.Unlock()
	if fn != nil {
		fn(st)
	}
}

func (c *Client) markDegraded(reason string) {
	c.mu.Lock()
	was := c.degraded
	c.degraded = true
	c.mu.Unlock()
	if !was {
		fmt.Fprintf(os.Stderr, "[licen-sdk-go] ⚠️ 授权中心不可达，进入宽限期: %s\n", reason)
	}
}

func (c *Client) post(path string, body []byte) (*registerResponse, error) {
	req, err := http.NewRequest(http.MethodPost, c.cfg.ServerURL+path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	var out registerResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("响应解析失败: %w", err)
	}
	return &out, nil
}

func (c *Client) get(path string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, c.cfg.ServerURL+path, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	buf := new(bytes.Buffer)
	_, err = buf.ReadFrom(resp.Body)
	return buf.Bytes(), err
}

// ---------- 工具 ----------

func hmacSHA256Hex(secret, data string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(data))
	return hex.EncodeToString(mac.Sum(nil))
}

func uuid() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}

func hostname() string {
	h, err := os.Hostname()
	if err != nil || h == "" {
		return "unknown-host"
	}
	return h
}

func localIP() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return ""
	}
	defer conn.Close()
	addr, ok := conn.LocalAddr().(*net.UDPAddr)
	if !ok {
		return ""
	}
	return addr.IP.String()
}
