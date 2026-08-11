// Package store SQLite 持久化：节点注册表、应用配置、审计日志。
package store

import (
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

// Node 节点记录
type Node struct {
	ID              int64     `json:"id"`
	NodeID          string    `json:"nodeId"`
	AppKey          string    `json:"appKey"`
	NodeName        string    `json:"nodeName"`
	IP              string    `json:"ip"`
	Version         string    `json:"version"`
	Status          string    `json:"status"` // ONLINE / OFFLINE
	RegisteredAt    time.Time `json:"registeredAt"`
	LastHeartbeatAt time.Time `json:"lastHeartbeatAt"`
}

// App 应用配置
type App struct {
	ID        int64     `json:"id"`
	AppKey    string    `json:"appKey"`
	AppSecret string    `json:"appSecret"`
	Product   string    `json:"product"`
	Name      string    `json:"name"`
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"createdAt"`
}

// Audit 审计日志
type Audit struct {
	ID     int64     `json:"id"`
	Time   time.Time `json:"time"`
	Action string    `json:"action"`
	Detail string    `json:"detail"`
}

// Store SQLite 存储
type Store struct {
	db *sql.DB
}

// Open 打开（自动建表）
func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("SQLite 打开失败: %w", err)
	}
	db.SetMaxOpenConns(1) // SQLite 单写者，避免锁冲突
	schema := `
CREATE TABLE IF NOT EXISTS licen_node (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    node_id TEXT NOT NULL UNIQUE,
    app_key TEXT NOT NULL,
    node_name TEXT,
    ip TEXT,
    version TEXT,
    status TEXT NOT NULL DEFAULT 'ONLINE',
    registered_at TIMESTAMP NOT NULL,
    last_heartbeat_at TIMESTAMP NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_node_lastbeat ON licen_node(last_heartbeat_at);
CREATE TABLE IF NOT EXISTS licen_app (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    app_key TEXT NOT NULL UNIQUE,
    app_secret TEXT NOT NULL,
    product TEXT,
    name TEXT,
    enabled INTEGER NOT NULL DEFAULT 1,
    created_at TIMESTAMP NOT NULL
);
CREATE TABLE IF NOT EXISTS licen_audit (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    time TIMESTAMP NOT NULL,
    action TEXT NOT NULL,
    detail TEXT
);
`
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("建表失败: %w", err)
	}
	return &Store{db: db}, nil
}

// Close 关闭
func (s *Store) Close() error { return s.db.Close() }

// ---------- 审计 ----------

// AuditLog 记录审计
func (s *Store) AuditLog(action, detail string) {
	_, _ = s.db.Exec(`INSERT INTO licen_audit(time, action, detail) VALUES (?, ?, ?)`,
		time.Now(), action, detail)
}

// ListAudits 审计列表（倒序）
func (s *Store) ListAudits(limit int) ([]Audit, error) {
	rows, err := s.db.Query(`SELECT id, time, action, detail FROM licen_audit ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Audit, 0)
	for rows.Next() {
		var a Audit
		if err := rows.Scan(&a.ID, &a.Time, &a.Action, &a.Detail); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// ---------- 应用 ----------

// FindApp 按 appKey 查应用
func (s *Store) FindApp(appKey string) (*App, error) {
	row := s.db.QueryRow(`SELECT id, app_key, app_secret, product, name, enabled, created_at
		FROM licen_app WHERE app_key = ?`, appKey)
	var a App
	err := row.Scan(&a.ID, &a.AppKey, &a.AppSecret, &a.Product, &a.Name, &a.Enabled, &a.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &a, nil
}

// ListApps 应用列表
func (s *Store) ListApps() ([]App, error) {
	rows, err := s.db.Query(`SELECT id, app_key, app_secret, product, name, enabled, created_at
		FROM licen_app ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]App, 0)
	for rows.Next() {
		var a App
		if err := rows.Scan(&a.ID, &a.AppKey, &a.AppSecret, &a.Product, &a.Name, &a.Enabled, &a.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// AppCount 应用数量
func (s *Store) AppCount() (int64, error) {
	var n int64
	err := s.db.QueryRow(`SELECT COUNT(*) FROM licen_app`).Scan(&n)
	return n, err
}

// CreateApp 创建应用
func (s *Store) CreateApp(a *App) error {
	res, err := s.db.Exec(`INSERT INTO licen_app(app_key, app_secret, product, name, enabled, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`, a.AppKey, a.AppSecret, a.Product, a.Name, a.Enabled, time.Now())
	if err != nil {
		return err
	}
	a.ID, _ = res.LastInsertId()
	return nil
}

// DeleteApp 删除应用
func (s *Store) DeleteApp(id int64) error {
	_, err := s.db.Exec(`DELETE FROM licen_app WHERE id = ?`, id)
	return err
}

// ---------- 节点 ----------

// FindNode 按 nodeId 查节点
func (s *Store) FindNode(nodeID string) (*Node, error) {
	row := s.db.QueryRow(`SELECT id, node_id, app_key, node_name, ip, version, status, registered_at, last_heartbeat_at
		FROM licen_node WHERE node_id = ?`, nodeID)
	var n Node
	err := row.Scan(&n.ID, &n.NodeID, &n.AppKey, &n.NodeName, &n.IP, &n.Version,
		&n.Status, &n.RegisteredAt, &n.LastHeartbeatAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &n, nil
}

// FindNodeByID 按主键查节点
func (s *Store) FindNodeByID(id int64) (*Node, error) {
	row := s.db.QueryRow(`SELECT id, node_id, app_key, node_name, ip, version, status, registered_at, last_heartbeat_at
		FROM licen_node WHERE id = ?`, id)
	var n Node
	err := row.Scan(&n.ID, &n.NodeID, &n.AppKey, &n.NodeName, &n.IP, &n.Version,
		&n.Status, &n.RegisteredAt, &n.LastHeartbeatAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &n, nil
}

// UpsertNode 注册/续约节点
func (s *Store) UpsertNode(n *Node) error {
	existing, err := s.FindNode(n.NodeID)
	if err != nil {
		return err
	}
	now := time.Now()
	if existing != nil {
		_, err = s.db.Exec(`UPDATE licen_node SET node_name=?, ip=?, version=?, status=?, last_heartbeat_at=?
			WHERE node_id=?`, n.NodeName, n.IP, n.Version, "ONLINE", now, n.NodeID)
		n.ID = existing.ID
		return err
	}
	res, err := s.db.Exec(`INSERT INTO licen_node(node_id, app_key, node_name, ip, version, status, registered_at, last_heartbeat_at)
		VALUES (?, ?, ?, ?, ?, 'ONLINE', ?, ?)`, n.NodeID, n.AppKey, n.NodeName, n.IP, n.Version, now, now)
	if err != nil {
		return err
	}
	n.ID, _ = res.LastInsertId()
	return nil
}

// TouchHeartbeat 更新心跳时间
func (s *Store) TouchHeartbeat(nodeID string) error {
	return s.TouchHeartbeatAt(nodeID, time.Now())
}

// TouchHeartbeatAt 将节点心跳时间置为指定时间（强制下线时置 epoch 立即回收名额）
func (s *Store) TouchHeartbeatAt(nodeID string, at time.Time) error {
	_, err := s.db.Exec(`UPDATE licen_node SET last_heartbeat_at=? WHERE node_id=?`, at, nodeID)
	return err
}

// CountOnline 统计最近窗口内在线节点数
func (s *Store) CountOnline(since time.Time) (int64, error) {
	var n int64
	err := s.db.QueryRow(`SELECT COUNT(*) FROM licen_node WHERE last_heartbeat_at > ?`, since).Scan(&n)
	return n, err
}

// ListStale 查询超过指定时间未心跳的节点
func (s *Store) ListStale(before time.Time) ([]Node, error) {
	rows, err := s.db.Query(`SELECT id, node_id, app_key, node_name, ip, version, status, registered_at, last_heartbeat_at
		FROM licen_node WHERE last_heartbeat_at < ?`, before)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Node, 0)
	for rows.Next() {
		var n Node
		if err := rows.Scan(&n.ID, &n.NodeID, &n.AppKey, &n.NodeName, &n.IP, &n.Version,
			&n.Status, &n.RegisteredAt, &n.LastHeartbeatAt); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

// SetNodeOffline 标记节点离线
func (s *Store) SetNodeOffline(nodeID string) error {
	_, err := s.db.Exec(`UPDATE licen_node SET status='OFFLINE' WHERE node_id=?`, nodeID)
	return err
}

// DeleteNode 删除节点（彻底回收）
func (s *Store) DeleteNode(id int64) error {
	_, err := s.db.Exec(`DELETE FROM licen_node WHERE id = ?`, id)
	return err
}

// ListNodes 节点列表（倒序）
func (s *Store) ListNodes(limit int) ([]Node, error) {
	rows, err := s.db.Query(`SELECT id, node_id, app_key, node_name, ip, version, status, registered_at, last_heartbeat_at
		FROM licen_node ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Node, 0)
	for rows.Next() {
		var n Node
		if err := rows.Scan(&n.ID, &n.NodeID, &n.AppKey, &n.NodeName, &n.IP, &n.Version,
			&n.Status, &n.RegisteredAt, &n.LastHeartbeatAt); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}
