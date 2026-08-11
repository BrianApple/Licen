// Package node 节点注册/心跳/名额控制/超时回收。
package node

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"strconv"
	"time"

	"github.com/BrianApple/Licen/internal/license"
	"github.com/BrianApple/Licen/internal/store"
)

// Service 节点服务
type Service struct {
	store       *store.Store
	licMgr      *license.Manager
	timeoutSec  int64
	hmacEnabled bool
}

// New 创建节点服务
func New(st *store.Store, licMgr *license.Manager, timeoutSec int64, hmacEnabled bool) *Service {
	return &Service{
		store:       st,
		licMgr:      licMgr,
		timeoutSec:  timeoutSec,
		hmacEnabled: hmacEnabled,
	}
}

// Result 注册/心跳结果
type Result struct {
	Success     bool   `json:"success"`
	Message     string `json:"message"`
	NodeID      string `json:"nodeId,omitempty"`
	LicenseID   string `json:"licenseId,omitempty"`
	ExpiresAt   string `json:"expiresAt,omitempty"`
	OnlineNodes int    `json:"onlineNodes"`
	MaxNodes    int    `json:"maxNodes"`
}

func fail(msg string) Result { return Result{Success: false, Message: msg} }

func ok(nodeID, licID, expiresAt string, online, max int) Result {
	return Result{Success: true, Message: "registered", NodeID: nodeID,
		LicenseID: licID, ExpiresAt: expiresAt, OnlineNodes: online, MaxNodes: max}
}

// Register 客户端注册（首次接入或重启续约）
func (s *Service) Register(appKey, appSecret, nodeID, nodeName, ip, version string) Result {
	// 1. 应用凭证校验
	app, err := s.store.FindApp(appKey)
	if err != nil || app == nil || !app.Enabled {
		s.store.AuditLog("NODE_REJECT", "nodeId="+nodeID+" appKey="+appKey+" 应用不存在或未启用")
		return fail("APP_NOT_FOUND")
	}
	if !hmacEqual(app.AppSecret, appSecret) {
		s.store.AuditLog("NODE_REJECT", "nodeId="+nodeID+" 密钥校验失败")
		return fail("APP_AUTH_FAILED")
	}

	// 2. License 有效性
	if !s.licMgr.IsValid() {
		return fail("LICENSE_INVALID:" + s.licMgr.Result().String())
	}

	// 3. 产品匹配（License.product 与 app.product 均非空且不一致时拒绝）
	lic := s.licMgr.License()
	if lic.Product != "" && app.Product != "" && lic.Product != app.Product {
		return fail("PRODUCT_MISMATCH")
	}

	now := time.Now()
	timeoutBefore := now.Add(-time.Duration(s.timeoutSec) * time.Second)
	maxNodes := s.licMgr.MaxNodes()

	// 4. 已注册节点 → 续约（不占新名额）
	existing, _ := s.store.FindNode(nodeID)
	if existing != nil {
		existing.NodeName = nodeName
		existing.IP = ip
		existing.Version = version
		if err := s.store.UpsertNode(existing); err != nil {
			slog.Error("节点续约失败", "err", err)
			return fail("STORE_ERROR")
		}
		s.store.AuditLog("NODE_RENEW", "nodeId="+nodeID+" appKey="+appKey)
		online, _ := s.store.CountOnline(timeoutBefore)
		slog.Info("🔄 节点续约", "nodeId", nodeID, "name", nodeName)
		return ok(nodeID, lic.LicenseID, lic.ExpiresAt, int(online), maxNodes)
	}

	// 5. 名额控制
	online, _ := s.store.CountOnline(timeoutBefore)
	if maxNodes > 0 && int(online) >= maxNodes {
		s.store.AuditLog("NODE_REJECT", "nodeId="+nodeID+" online="+strconv.FormatInt(online, 10)+" max="+strconv.Itoa(maxNodes))
		slog.Warn("⛔ 节点名额已满", "online", online, "max", maxNodes, "nodeId", nodeID)
		return Result{Success: false, Message: "NODE_LIMIT_REACHED", OnlineNodes: int(online), MaxNodes: maxNodes}
	}

	// 6. 新节点注册
	n := &store.Node{
		NodeID:   nodeID,
		AppKey:   appKey,
		NodeName: nodeName,
		IP:       ip,
		Version:  version,
		Status:   "ONLINE",
	}
	if err := s.store.UpsertNode(n); err != nil {
		slog.Error("节点注册失败", "err", err)
		return fail("STORE_ERROR")
	}
	s.store.AuditLog("NODE_REGISTER", "nodeId="+nodeID+" appKey="+appKey+" name="+nodeName+" ip="+ip)
	slog.Info("✅ 节点注册", "nodeId", nodeID, "name", nodeName, "ip", ip)
	return ok(nodeID, lic.LicenseID, lic.ExpiresAt, int(online)+1, maxNodes)
}

// Heartbeat 心跳保活（HMAC 签名校验）
func (s *Service) Heartbeat(nodeID, appKey, timestamp, sign string) Result {
	node, err := s.store.FindNode(nodeID)
	if err != nil || node == nil {
		return fail("NODE_NOT_FOUND")
	}
	if s.hmacEnabled {
		app, err := s.store.FindApp(appKey)
		if err != nil || app == nil {
			return fail("APP_NOT_FOUND")
		}
		expected := HmacSHA256Hex(app.AppSecret, nodeID+":"+timestamp)
		if !hmacEqual(expected, sign) {
			return fail("SIGN_INVALID")
		}
		// 时间戳防重放（±5 分钟）
		ts, err := strconv.ParseInt(timestamp, 10, 64)
		if err != nil {
			return fail("TIMESTAMP_INVALID")
		}
		if diff := abs(time.Now().UnixMilli() - ts); diff > 5*60*1000 {
			return fail("TIMESTAMP_REJECTED")
		}
	}
	if err := s.store.TouchHeartbeat(nodeID); err != nil {
		return fail("STORE_ERROR")
	}
	lic := s.licMgr.License()
	licID, expiresAt := "", ""
	if lic != nil {
		licID, expiresAt = lic.LicenseID, lic.ExpiresAt
	}
	online, _ := s.store.CountOnline(time.Now().Add(-time.Duration(s.timeoutSec) * time.Second))
	return ok(nodeID, licID, expiresAt, int(online), s.licMgr.MaxNodes())
}

// ListNodes 节点列表
func (s *Service) ListNodes(limit int) ([]store.Node, error) {
	return s.store.ListNodes(limit)
}

// OnlineCount 当前在线节点数
func (s *Service) OnlineCount() int {
	n, _ := s.store.CountOnline(time.Now().Add(-time.Duration(s.timeoutSec) * time.Second))
	return int(n)
}

// RevokeNode 强制下线（释放名额）
func (s *Service) RevokeNode(id int64) bool {
	node, err := s.store.FindNodeByID(id)
	if err != nil || node == nil {
		return false
	}
	if err := s.store.SetNodeOffline(node.NodeID); err != nil {
		return false
	}
	// 置为 epoch 立即回收名额
	s.store.TouchHeartbeatAt(node.NodeID, time.Unix(0, 0))
	s.store.AuditLog("NODE_REVOKE", "nodeId="+node.NodeID)
	slog.Info("🗑️ 节点强制下线", "nodeId", node.NodeID)
	return true
}

// RecycleLoop 定时回收：超时未心跳标记离线，2 倍超时彻底清理
func (s *Service) RecycleLoop(stop <-chan struct{}) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			s.recycleOnce()
		}
	}
}

func (s *Service) recycleOnce() {
	now := time.Now()
	timeout := time.Duration(s.timeoutSec) * time.Second
	// 标记离线
	stale, _ := s.store.ListStale(now.Add(-timeout))
	for _, n := range stale {
		if n.Status != "OFFLINE" {
			_ = s.store.SetNodeOffline(n.NodeID)
			s.store.AuditLog("NODE_OFFLINE", "nodeId="+n.NodeID)
			slog.Info("📴 节点离线", "nodeId", n.NodeID, "last", n.LastHeartbeatAt)
		}
	}
	// 彻底清理 2 倍超时以上
	dead, _ := s.store.ListStale(now.Add(-2 * timeout))
	for _, n := range dead {
		_ = s.store.DeleteNode(n.ID)
		s.store.AuditLog("NODE_RECYCLE", "nodeId="+n.NodeID)
		slog.Info("🧹 节点记录已清理", "nodeId", n.NodeID)
	}
}

// HmacSHA256Hex HMAC-SHA256 十六进制摘要
func HmacSHA256Hex(secret, data string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(data))
	return hex.EncodeToString(mac.Sum(nil))
}

func hmacEqual(a, b string) bool {
	return hmac.Equal([]byte(a), []byte(b))
}

func abs(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}
