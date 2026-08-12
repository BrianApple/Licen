// 客户-产品对应关系管理：
//   - 客户是独立实体（与台账 License.customer 同名即同一客户），可绑定多个产品
//   - 每个绑定记录：产品 / 节点上限 / 版本套餐 / 状态（active/paused/terminated）/ 备注
//   - 与台账联动：签发时自动登记（EnsureCustomer）；列表附带授权统计
//   - 持久化 data/customer-products.json（原子写盘）
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Binding 客户对一个产品的授权绑定
type Binding struct {
	Product  string    `json:"product"`        // 产品 ID（须存在于产品库）
	MaxNodes int       `json:"maxNodes"`       // 授权节点上限（0=不限制/默认）
	Edition  string    `json:"edition"`        // 版本/套餐（如 enterprise）
	Status   string    `json:"status"`         // active / paused / terminated
	Note     string    `json:"note,omitempty"` // 备注
	BoundAt  time.Time `json:"boundAt"`        // 绑定时间
}

// CustomerProduct 一个客户及其产品绑定
type CustomerProduct struct {
	Customer  string    `json:"customer"`          // 客户名称（与台账一致，唯一）
	Contact   string    `json:"contact,omitempty"` // 联系人
	Phone     string    `json:"phone,omitempty"`   // 电话
	Email     string    `json:"email,omitempty"`   // 邮箱
	Address   string    `json:"address,omitempty"` // 地址
	Note      string    `json:"note,omitempty"`    // 客户备注
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
	Products  []Binding `json:"products"`
}

// BindingStats 绑定台账统计（列表时填充）
type BindingStats struct {
	LicenseCount int `json:"licenseCount"` // 该客户×产品授权总数
	ActiveCount  int `json:"activeCount"`  // 当前有效（含即将到期）
}

// Binding 台账统计（列表时填充到 Binding.LicenseCount/ActiveCount）
type BindingWithStats struct {
	Binding
	LicenseCount int `json:"licenseCount"` // 该客户×该产品授权总数
	ActiveCount  int `json:"activeCount"`  // 当前有效（含即将到期）
}

// CustomerProductStore 客户-产品对应关系存储
type CustomerProductStore struct {
	mu    sync.Mutex
	path  string
	custs map[string]*CustomerProduct
}

// NewCustomerProductStore 加载或初始化（文件不存在返回空库）
func NewCustomerProductStore(path string) (*CustomerProductStore, error) {
	s := &CustomerProductStore{path: path, custs: make(map[string]*CustomerProduct)}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("创建对应关系目录失败: %w", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return s, nil
		}
		return nil, fmt.Errorf("读取客户-产品对应关系失败: %w", err)
	}
	if len(data) == 0 {
		return s, nil
	}
	var list []CustomerProduct
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, fmt.Errorf("客户-产品对应关系解析失败: %w", err)
	}
	for i := range list {
		c := &list[i]
		if c.Customer != "" {
			s.custs[c.Customer] = c
		}
	}
	return s, nil
}

func (s *CustomerProductStore) save() error {
	list := make([]CustomerProduct, 0, len(s.custs))
	for _, c := range s.custs {
		list = append(list, *c)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Customer < list[j].Customer })
	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

// All 客户列表（按客户名排序）
func (s *CustomerProductStore) All() []CustomerProduct {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]CustomerProduct, 0, len(s.custs))
	for _, c := range s.custs {
		out = append(out, *c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Customer < out[j].Customer })
	return out
}

// Get 按客户名查
func (s *CustomerProductStore) Get(customer string) (CustomerProduct, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.custs[customer]
	if !ok {
		return CustomerProduct{}, false
	}
	return *c, true
}

// UpsertCustomer 新建或更新客户（force=true 覆盖已有；否则已存在报错）
func (s *CustomerProductStore) UpsertCustomer(c CustomerProduct, force bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	c.Customer = strings.TrimSpace(c.Customer)
	if c.Customer == "" {
		return errors.New("客户名称不能为空")
	}
	if !force {
		if _, exists := s.custs[c.Customer]; exists {
			return errors.New("客户已存在: " + c.Customer)
		}
	} else if old, exists := s.custs[c.Customer]; exists {
		c.CreatedAt = old.CreatedAt
	}
	if c.CreatedAt.IsZero() {
		c.CreatedAt = time.Now()
	}
	c.UpdatedAt = time.Now()
	// 规范化绑定
	c.Products = normalizeBindings(c.Products)
	s.custs[c.Customer] = &c
	return s.save()
}

// DeleteCustomer 删除客户；hasLicense 时拒绝（保护台账）
func (s *CustomerProductStore) DeleteCustomer(customer string, hasLicense bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if hasLicense {
		return errors.New("该客户已有授权记录，禁止删除（请先吊销/清理相关 License）")
	}
	if _, ok := s.custs[customer]; !ok {
		return errors.New("客户不存在: " + customer)
	}
	delete(s.custs, customer)
	return s.save()
}

// UpsertBinding 客户绑定/更新产品
func (s *CustomerProductStore) UpsertBinding(customer string, b Binding) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.custs[customer]
	if !ok {
		return errors.New("客户不存在: " + customer)
	}
	b.Product = strings.TrimSpace(b.Product)
	if b.Product == "" {
		return errors.New("产品不能为空")
	}
	if b.Status == "" {
		b.Status = "active"
	}
	if b.MaxNodes <= 0 {
		b.MaxNodes = 0
	}
	// 已绑定则更新，否则追加
	idx := -1
	for i := range c.Products {
		if c.Products[i].Product == b.Product {
			idx = i
			break
		}
	}
	if idx >= 0 {
		b.BoundAt = c.Products[idx].BoundAt
		if b.BoundAt.IsZero() {
			b.BoundAt = time.Now()
		}
		c.Products[idx] = b
	} else {
		b.BoundAt = time.Now()
		c.Products = append(c.Products, b)
	}
	c.UpdatedAt = time.Now()
	return s.save()
}

// RemoveBinding 解绑产品；hasLicense 时拒绝
func (s *CustomerProductStore) RemoveBinding(customer, product string, hasLicense bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.custs[customer]
	if !ok {
		return errors.New("客户不存在: " + customer)
	}
	if hasLicense {
		return errors.New("该客户对此产品已有授权记录，禁止解绑（请先吊销/清理相关 License）")
	}
	idx := -1
	for i := range c.Products {
		if c.Products[i].Product == product {
			idx = i
			break
		}
	}
	if idx < 0 {
		return errors.New("绑定不存在: " + customer + " × " + product)
	}
	c.Products = append(c.Products[:idx], c.Products[idx+1:]...)
	c.UpdatedAt = time.Now()
	return s.save()
}

// Ensure 确保客户存在并绑定产品（签发时自动登记；无则自动创建）
func (s *CustomerProductStore) Ensure(customer, product string) {
	if customer == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.custs[customer]
	if !ok {
		c = &CustomerProduct{
			Customer:  customer,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		s.custs[customer] = c
	}
	if product != "" {
		found := false
		for i := range c.Products {
			if c.Products[i].Product == product {
				found = true
				break
			}
		}
		if !found {
			c.Products = append(c.Products, Binding{
				Product: product, Edition: "enterprise",
				Status: "active", BoundAt: time.Now(),
			})
		}
	}
	c.UpdatedAt = time.Now()
	_ = s.save()
}

// normalizeBindings 绑定列表规范化：去重、默认值
func normalizeBindings(bs []Binding) []Binding {
	var out []Binding
	seen := map[string]bool{}
	for _, b := range bs {
		b.Product = strings.TrimSpace(b.Product)
		if b.Product == "" || seen[b.Product] {
			continue
		}
		seen[b.Product] = true
		if b.Status == "" {
			b.Status = "active"
		}
		if b.BoundAt.IsZero() {
			b.BoundAt = time.Now()
		}
		out = append(out, b)
	}
	return out
}

// ProductBoundCount 某产品被多少客户绑定（产品删除保护用）
func (s *CustomerProductStore) ProductBoundCount(product string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for _, c := range s.custs {
		for _, b := range c.Products {
			if b.Product == product {
				n++
			}
		}
	}
	return n
}

// ---------- HTTP ----------

// handleCustomerProducts 客户-产品对应关系列表（含绑定台账统计）
func (s *IssuerServer) handleCustomerProducts(w http.ResponseWriter, _ *http.Request) {
	list := s.cpStore.All()
	now := time.Now()
	byKey := map[string]*BindingStats{}
	for _, r := range s.store.All() {
		key := r.Customer + "\x00" + r.Product
		st := byKey[key]
		if st == nil {
			st = &BindingStats{}
			byKey[key] = st
		}
		st.LicenseCount++
		status := r.Status(now)
		if status == "valid" || status == "expiring" {
			st.ActiveCount++
		}
	}
	type item struct {
		CustomerProduct
		TotalLicenses  int                `json:"totalLicenses"`
		ActiveLicenses int                `json:"activeLicenses"`
		Products       []BindingWithStats `json:"products"`
	}
	items := make([]item, 0, len(list))
	for _, c := range list {
		it := item{CustomerProduct: c}
		it.Products = make([]BindingWithStats, 0, len(c.Products))
		for _, b := range c.Products {
			bws := BindingWithStats{Binding: b}
			if st := byKey[c.Customer+"\x00"+b.Product]; st != nil {
				bws.LicenseCount = st.LicenseCount
				bws.ActiveCount = st.ActiveCount
				it.TotalLicenses += st.LicenseCount
				it.ActiveLicenses += st.ActiveCount
			}
			it.Products = append(it.Products, bws)
		}
		items = append(items, it)
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "total": len(items), "customers": items})
}

type customerReq struct {
	Customer string    `json:"customer"`
	Contact  string    `json:"contact"`
	Phone    string    `json:"phone"`
	Email    string    `json:"email"`
	Address  string    `json:"address"`
	Note     string    `json:"note"`
	Products []Binding `json:"products"`
}

// handleCreateCustomer 新建客户（可带初始产品绑定）
func (s *IssuerServer) handleCreateCustomer(w http.ResponseWriter, r *http.Request) {
	var req customerReq
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"success": false, "message": "请求体解析失败: " + err.Error()})
		return
	}
	c := CustomerProduct{
		Customer: req.Customer, Contact: req.Contact, Phone: req.Phone,
		Email: req.Email, Address: req.Address, Note: req.Note, Products: req.Products,
	}
	if err := s.cpStore.UpsertCustomer(c, false); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"success": false, "message": err.Error()})
		return
	}
	slog.Info("🆕 客户已创建", "customer", c.Customer)
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "message": "客户已创建", "customer": c.Customer})
}

// handleUpdateCustomer 更新客户信息
func (s *IssuerServer) handleUpdateCustomer(w http.ResponseWriter, r *http.Request) {
	customer := r.PathValue("customer")
	var req customerReq
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"success": false, "message": "请求体解析失败: " + err.Error()})
		return
	}
	old, ok := s.cpStore.Get(customer)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{"success": false, "message": "客户不存在: " + customer})
		return
	}
	old.Contact = req.Contact
	old.Phone = req.Phone
	old.Email = req.Email
	old.Address = req.Address
	old.Note = req.Note
	if err := s.cpStore.UpsertCustomer(old, true); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"success": false, "message": err.Error()})
		return
	}
	slog.Info("✏️ 客户已更新", "customer", customer)
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "message": "客户已更新"})
}

// handleDeleteCustomer 删除客户（有授权记录拒绝）
func (s *IssuerServer) handleDeleteCustomer(w http.ResponseWriter, r *http.Request) {
	customer := r.PathValue("customer")
	hasLicense := false
	for _, rec := range s.store.All() {
		if rec.Customer == customer {
			hasLicense = true
			break
		}
	}
	if err := s.cpStore.DeleteCustomer(customer, hasLicense); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"success": false, "message": err.Error()})
		return
	}
	slog.Info("🗑️ 客户已删除", "customer", customer)
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "message": "客户已删除"})
}

type bindingReq struct {
	Product  string `json:"product"`
	MaxNodes int    `json:"maxNodes"`
	Edition  string `json:"edition"`
	Status   string `json:"status"`
	Note     string `json:"note"`
}

// handleBindProduct 客户绑定产品
func (s *IssuerServer) handleBindProduct(w http.ResponseWriter, r *http.Request) {
	customer := r.PathValue("customer")
	var req bindingReq
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"success": false, "message": "请求体解析失败: " + err.Error()})
		return
	}
	// 产品必须存在于产品库
	if _, ok := s.products.Get(req.Product); !ok {
		writeJSON(w, http.StatusBadRequest, map[string]any{"success": false, "message": "产品不存在: " + req.Product + "（请先在产品库创建）"})
		return
	}
	b := Binding{Product: req.Product, MaxNodes: req.MaxNodes, Edition: req.Edition, Status: req.Status, Note: req.Note}
	if err := s.cpStore.UpsertBinding(customer, b); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"success": false, "message": err.Error()})
		return
	}
	slog.Info("🔗 客户已绑定产品", "customer", customer, "product", req.Product)
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "message": "绑定成功"})
}

// handleUpdateBinding 更新绑定（节点上限/版本/状态/备注）
func (s *IssuerServer) handleUpdateBinding(w http.ResponseWriter, r *http.Request) {
	customer := r.PathValue("customer")
	product := r.PathValue("product")
	var req bindingReq
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"success": false, "message": "请求体解析失败: " + err.Error()})
		return
	}
	req.Product = product
	if err := s.cpStore.UpsertBinding(customer, req2Binding(req)); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"success": false, "message": err.Error()})
		return
	}
	slog.Info("✏️ 绑定已更新", "customer", customer, "product", product)
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "message": "绑定已更新"})
}

// handleUnbindProduct 解绑产品（有授权记录拒绝）
func (s *IssuerServer) handleUnbindProduct(w http.ResponseWriter, r *http.Request) {
	customer := r.PathValue("customer")
	product := r.PathValue("product")
	hasLicense := false
	for _, rec := range s.store.All() {
		if rec.Customer == customer && rec.Product == product {
			hasLicense = true
			break
		}
	}
	if err := s.cpStore.RemoveBinding(customer, product, hasLicense); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"success": false, "message": err.Error()})
		return
	}
	slog.Info("🔓 客户已解绑产品", "customer", customer, "product", product)
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "message": "已解绑"})
}

func req2Binding(req bindingReq) Binding {
	return Binding{Product: req.Product, MaxNodes: req.MaxNodes, Edition: req.Edition, Status: req.Status, Note: req.Note}
}
