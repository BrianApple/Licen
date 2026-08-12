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
// product：SDK 自声明的产品标识（旧 SDK 可传空串，走宽松模式）
func (s *Service) Register(appKey, appSecret, product, nodeID, nodeName, ip, version string) Result {
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

	// 2. 三锁校验（产品对应关系：SDK 声明 == App 凭证绑定 == License 签名授权）
	// 锁1：SDK 自声明产品必须与 App 凭证绑定的产品一致（防凭证串用到其他产品）
	if product != "" && app.Product != "" && product != app.Product {
		s.store.AuditLog("NODE_REJECT", "nodeId="+nodeID+" appKey="+appKey+" product="+product+" appProduct="+app.Product)
		return fail("PRODUCT_MISMATCH")
	}
	// 产品标识确定：SDK 声明优先，其次 App 凭证绑定
	prod := product
	if prod == "" {
		prod = app.Product
	}
	// 锁2：该产品必须有有效授权（精确匹配，不再全局随机）
	lic := s.licMgr.LicenseFor(prod)
	if lic == nil || s.licMgr.ResultFor(prod) != license.Valid {
		s.store.AuditLog("NODE_REJECT", "nodeId="+nodeID+" appKey="+appKey+" product="+prod+" 无有效授权")
		return fail("PRODUCT_NOT_LICENSED")
	}

	now := time.Now()
	timeoutBefore := now.Add(-time.Duration(s.timeoutSec) * time.Second)
	// 锁3：限额按本产品（MaxNodesFor），不用所有产品之和
	maxNodes := s.licMgr.MaxNodesFor(prod)

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
		s.store.AuditLog("NODE_RENEW", "nodeId="+nodeID+" appKey="+appKey+" product="+prod)
		online, _ := s.store.CountOnline(timeoutBefore)
		slog.Info("🔄 节点续约", "nodeId", nodeID, "name", nodeName, "product", prod)
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
	// 返回本产品 License 信息（通过 node 所属 App 的产品标识精确匹配，避免串产品）
	licID, expiresAt := "", ""
	maxNodes := 0
	if app, err := s.store.FindApp(node.AppKey); err == nil && app != nil {
		lic := s.licMgr.LicenseFor(app.Product)
		if lic != nil {
			licID, expiresAt = lic.LicenseID, lic.ExpiresAt
			maxNodes = s.licMgr.MaxNodesFor(app.Product)
		}
	}
	online, _ := s.store.CountOnline(time.Now().Add(-time.Duration(s.timeoutSec) * time.Second))
	return ok(nodeID, licID, expiresAt, int(online), maxNodes)
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
