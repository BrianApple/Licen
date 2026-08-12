// 台账存储：记录已签发的 License（签发管理/审计）。
// 持久化到 data/licenses.json（JSON 数组，追加式），进程重启不丢。
// 线程安全：sync.Mutex 保护读写；写盘用原子替换（临时文件+rename）防损坏。
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// LicenseRecord 一条已签发 License 的台账记录
type LicenseRecord struct {
	LicenseID   string     `json:"licenseId"`
	Product     string     `json:"product"`
	Edition     string     `json:"edition"`
	Customer    string     `json:"customer"`
	MachineCode string     `json:"machineCode"`
	MaxNodes    int        `json:"maxNodes"`
	Features    []string   `json:"features"`
	IssuedAt    time.Time  `json:"issuedAt"`
	ExpiresAt   time.Time  `json:"expiresAt"`
	Revoked     bool       `json:"revoked"`
	RevokedAt   *time.Time `json:"revokedAt,omitempty"`
	RevokeNote  string     `json:"revokeNote,omitempty"`
	// ReissuedTo 重新签发后的新 LicenseID（吊销作废后替换）
	ReissuedTo string `json:"reissuedTo,omitempty"`
	// ReissuedFrom 本 License 由哪个旧 License 重新签发而来
	ReissuedFrom string `json:"reissuedFrom,omitempty"`
}

// Status 派生状态：有效 / 即将到期（≤30天）/ 已过期 / 已吊销
func (r LicenseRecord) Status(now time.Time) string {
	if r.Revoked {
		return "revoked"
	}
	if now.After(r.ExpiresAt) {
		return "expired"
	}
	if now.Add(30 * 24 * time.Hour).After(r.ExpiresAt) {
		return "expiring"
	}
	return "valid"
}

// DaysLeft 剩余天数（到期 - 当前；负数表示已过期天数）
func (r LicenseRecord) DaysLeft(now time.Time) int {
	return int(r.ExpiresAt.Sub(now).Hours() / 24)
}

// Store License 台账存储
type Store struct {
	mu   sync.Mutex
	path string
	recs []LicenseRecord
}

// NewStore 加载或初始化台账（目录不存在则创建）
func NewStore(path string) (*Store, error) {
	s := &Store{path: path}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("创建台账目录失败: %w", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			s.recs = []LicenseRecord{}
			return s, nil
		}
		return nil, fmt.Errorf("读取台账失败: %w", err)
	}
	if len(data) == 0 {
		s.recs = []LicenseRecord{}
		return s, nil
	}
	if err := json.Unmarshal(data, &s.recs); err != nil {
		return nil, fmt.Errorf("台账解析失败（%s）: %w", path, err)
	}
	return s, nil
}

// Add 追加一条签发记录并持久化
func (s *Store) Add(rec LicenseRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.recs = append(s.recs, rec)
	return s.save()
}

// All 返回全部记录（新→旧）
func (s *Store) All() []LicenseRecord {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]LicenseRecord, len(s.recs))
	copy(out, s.recs)
	sort.Slice(out, func(i, j int) bool { return out[i].IssuedAt.After(out[j].IssuedAt) })
	return out
}

// Get 按 LicenseID 查记录
func (s *Store) Get(licenseID string) (LicenseRecord, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, r := range s.recs {
		if r.LicenseID == licenseID {
			return r, true
		}
	}
	return LicenseRecord{}, false
}

// Update 更新一条记录（吊销/重签），持久化
func (s *Store) Update(rec LicenseRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.recs {
		if s.recs[i].LicenseID == rec.LicenseID {
			s.recs[i] = rec
			return s.save()
		}
	}
	return errors.New("记录不存在: " + rec.LicenseID)
}

// save 原子写盘（临时文件 + rename，防止写一半崩溃损坏台账）
func (s *Store) save() error {
	data, err := json.MarshalIndent(s.recs, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

// CustomerSummary 客户维度汇总（预填签发表单用）
type CustomerSummary struct {
	Customer      string    `json:"customer"`
	LastProduct   string    `json:"lastProduct"`
	LastEdition   string    `json:"lastEdition"`
	LastNodes     int       `json:"lastNodes"`
	LastFeatures  []string  `json:"lastFeatures"`
	MachineCode   string    `json:"machineCode"`
	LastIssued    time.Time `json:"lastIssued"`
	Licenses      int       `json:"licenses"`
	ActiveCount   int       `json:"activeCount"`   // 当前有效（含即将到期）的授权数
	ExpiringCount int       `json:"expiringCount"` // 30 天内到期数
}

// Customers 客户列表（按客户分组汇总，供签发表单预填下拉）
func (s *Store) Customers(now time.Time) []CustomerSummary {
	s.mu.Lock()
	defer s.mu.Unlock()
	byCustomer := make(map[string]*CustomerSummary)
	var order []string
	for _, r := range s.recs {
		if r.Customer == "" {
			continue
		}
		c, ok := byCustomer[r.Customer]
		if !ok {
			c = &CustomerSummary{Customer: r.Customer}
			byCustomer[r.Customer] = c
			order = append(order, r.Customer)
		}
		c.Licenses++
		st := r.Status(now)
		if st == "valid" || st == "expiring" {
			c.ActiveCount++
		}
		if st == "expiring" {
			c.ExpiringCount++
		}
		// 最近一次签发作为预填参考
		if r.IssuedAt.After(c.LastIssued) {
			c.LastIssued = r.IssuedAt
			c.LastProduct = r.Product
			c.LastEdition = r.Edition
			c.LastNodes = r.MaxNodes
			c.LastFeatures = append([]string(nil), r.Features...)
			c.MachineCode = r.MachineCode
		}
	}
	out := make([]CustomerSummary, 0, len(order))
	for _, name := range order {
		out = append(out, *byCustomer[name])
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].LastIssued.After(out[j].LastIssued)
	})
	return out
}

// Timeline 沿 reissuedFrom/reissuedTo 双向遍历，返回该授权链完整时序（按签发时间排序）。
// 返回从最初签发到当前记录的全部节点（含被吊销的旧记录）。
func (s *Store) Timeline(licenseID string) []LicenseRecord {
	s.mu.Lock()
	defer s.mu.Unlock()
	byID := make(map[string]LicenseRecord, len(s.recs))
	for _, r := range s.recs {
		byID[r.LicenseID] = r
	}
	// 向前回溯到链首
	head := licenseID
	for {
		rec, ok := byID[head]
		if !ok {
			break
		}
		if rec.ReissuedFrom == "" {
			break
		}
		head = rec.ReissuedFrom
	}
	// 从链首向后收集
	var chain []LicenseRecord
	cur := head
	for {
		rec, ok := byID[cur]
		if !ok {
			break
		}
		chain = append(chain, rec)
		if rec.ReissuedTo == "" {
			break
		}
		cur = rec.ReissuedTo
	}
	sort.Slice(chain, func(i, j int) bool { return chain[i].IssuedAt.Before(chain[j].IssuedAt) })
	return chain
}
