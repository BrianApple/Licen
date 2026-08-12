// 产品库 + SDK 分发：
//   - products.json 存储产品信息（id=License.product 唯一标识，签发下拉来源）
//   - 内嵌 4 语言 SDK 源码（go:embed），按产品×语言打包 zip 下载
//   - 历史台账中出现的 product 自动导入产品库（迁移，保证签发/台账/产品三者一致）
package main

import (
	"archive/zip"
	"bytes"
	"embed"
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

// ---------- SDK 内嵌 ----------

//go:embed sdk/go sdk/java sdk/python sdk/c
var sdkFS embed.FS

// sdkLanguages 支持的语言 SDK 与 zip 包名
var sdkLanguages = map[string]string{
	"go":     "licen-sdk-go-1.0.0",
	"java":   "licen-sdk-java-1.0.0",
	"python": "licen-sdk-python-1.0.0",
	"c":      "licen-sdk-c-1.0.0",
}

// ---------- 产品信息 ----------

// ProductInfo 一个可授权产品（id 与 License.product 一致）
type ProductInfo struct {
	ID           string    `json:"id"`          // 产品标识（唯一，= License.product）
	Name         string    `json:"name"`        // 产品名称
	Description  string    `json:"description"` // 产品描述
	SDKs         []string  `json:"sdks"`        // 支持的语言 SDK（go/java/python/c）
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
	LicenseCount int       `json:"licenseCount,omitempty"` // 台账授权数（列表时填充）
}

// ProductStore 产品库存储（持久化 data/products.json）
type ProductStore struct {
	mu    sync.Mutex
	path  string
	prods map[string]ProductInfo
}

// seedProducts 空产品库的预置样例产品（HXAPIGate 智能网关 / IOTGate 物联网关）
var seedProducts = []ProductInfo{
	{ID: "hxapigate", Name: "HXAPIGate 智能网关", Description: "HXAPIGate 智能 API 网关：统一流量接入、API 管理与企业服务集成", SDKs: []string{"go", "java", "python", "c"}},
	{ID: "iotgate", Name: "IOTGate 物联网关", Description: "基于 Netty 的高性能物联网关：多协议设备接入与边缘计算", SDKs: []string{"go", "java", "python", "c"}},
}

// NewProductStore 加载或初始化产品库（空库时预置样例产品）
func NewProductStore(path string) (*ProductStore, error) {
	s := &ProductStore{path: path, prods: make(map[string]ProductInfo)}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("创建产品库目录失败: %w", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// 空库预置样例产品
			for _, p := range seedProducts {
				p.CreatedAt = time.Now()
				p.UpdatedAt = time.Now()
				s.prods[p.ID] = p
			}
			if err := s.save(); err != nil {
				return nil, fmt.Errorf("产品库样例初始化失败: %w", err)
			}
			return s, nil
		}
		return nil, fmt.Errorf("读取产品库失败: %w", err)
	}
	if len(data) == 0 {
		return s, nil
	}
	var list []ProductInfo
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, fmt.Errorf("产品库解析失败: %w", err)
	}
	for _, p := range list {
		s.prods[p.ID] = p
	}
	return s, nil
}

func (s *ProductStore) save() error {
	list := make([]ProductInfo, 0, len(s.prods))
	for _, p := range s.prods {
		list = append(list, p)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].ID < list[j].ID })
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

// All 产品列表（按 ID 排序）
func (s *ProductStore) All() []ProductInfo {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]ProductInfo, 0, len(s.prods))
	for _, p := range s.prods {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// Get 按 ID 查产品
func (s *ProductStore) Get(id string) (ProductInfo, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.prods[id]
	return p, ok
}

// Upsert 新建或更新产品；force=true 时允许覆盖
func (s *ProductStore) Upsert(p ProductInfo, force bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if p.ID == "" {
		return errors.New("产品 ID 不能为空")
	}
	if !force {
		if _, exists := s.prods[p.ID]; exists {
			return errors.New("产品已存在: " + p.ID)
		}
	} else if old, exists := s.prods[p.ID]; exists {
		p.CreatedAt = old.CreatedAt
	}
	if p.CreatedAt.IsZero() {
		p.CreatedAt = time.Now()
	}
	p.UpdatedAt = time.Now()
	// 规范化 SDK 语言列表
	var sdks []string
	seen := map[string]bool{}
	for _, l := range p.SDKs {
		l = strings.ToLower(strings.TrimSpace(l))
		if l != "" && !seen[l] {
			if _, ok := sdkLanguages[l]; ok {
				seen[l] = true
				sdks = append(sdks, l)
			}
		}
	}
	if len(sdks) == 0 {
		sdks = []string{"go", "java", "python", "c"}
	}
	p.SDKs = sdks
	s.prods[p.ID] = p
	return s.save()
}

// Delete 删除产品；hasLicense 时拒绝（保护台账完整性）
func (s *ProductStore) Delete(id string, hasLicense bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if hasLicense {
		return errors.New("该产品已有授权记录，禁止删除（请先吊销/清理相关 License）")
	}
	if _, ok := s.prods[id]; !ok {
		return errors.New("产品不存在: " + id)
	}
	delete(s.prods, id)
	return s.save()
}

// EnsureProduct 确保产品存在（历史台账迁移：自动导入）
func (s *ProductStore) EnsureProduct(id string) {
	if id == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.prods[id]; ok {
		return
	}
	now := time.Now()
	s.prods[id] = ProductInfo{
		ID: id, Name: id, Description: "（历史台账自动导入，请补充产品信息）",
		SDKs:      []string{"go", "java", "python", "c"},
		CreatedAt: now, UpdatedAt: now,
	}
	_ = s.save()
}

// ---------- SDK zip 打包 ----------

// sdkProductPlaceholder SDK 源码中的产品占位符：打包时替换为所选产品 ID
// （通用版替换为空字符串；定制版替换为产品 ID，SDK 注册/心跳默认携带该产品）
const sdkProductPlaceholder = "__LICEN_DEFAULT_PRODUCT__"

// sdkInfoFile zip 根目录的定制信息文件（标注产品/版本/下载时间）
const sdkInfoFile = "sdk-info.json"

// buildSDKZip 将内嵌 SDK 源码打包为 zip 字节流。
// product 为空 = 通用 SDK（占位符替换为空，与旧版一致）；非空 = 定制 SDK：
//   - SDK 源码中默认产品标识注入为所选产品（注册/心跳自动携带）
//   - zip 根目录附加 sdk-info.json 定制信息
func buildSDKZip(lang, product string) ([]byte, error) {
	dir, ok := sdkLanguages[lang]
	if !ok {
		return nil, fmt.Errorf("不支持的语言: %s（可用: go/java/python/c）", lang)
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	prefix := "sdk/" + lang
	root := dir + "/"
	// 替换目标：定制版注入产品 ID，通用版清空占位符
	repl := ""
	if product != "" {
		repl = product
	}
	err := walkEmbed(sdkFS, prefix, func(path string, isDir bool) error {
		if isDir {
			return nil
		}
		rel := strings.TrimPrefix(path, prefix)
		rel = strings.TrimPrefix(rel, "/")
		data, err := sdkFS.ReadFile(path)
		if err != nil {
			return err
		}
		// 文本文件做占位符替换（SDK 均为源码文本；二进制文件直接原样打入）
		if isTextFile(rel) {
			data = []byte(strings.ReplaceAll(string(data), sdkProductPlaceholder, repl))
		}
		w, err := zw.Create(root + rel)
		if err != nil {
			return err
		}
		_, err = w.Write(data)
		return err
	})
	if err != nil {
		return nil, err
	}
	// 定制 SDK：zip 根目录附加 sdk-info.json
	if product != "" {
		info := map[string]any{
			"lang":         lang,
			"version":      strings.TrimPrefix(dir, "licen-sdk-"+lang+"-"),
			"product":      product,
			"productName":  product,
			"downloadedAt": time.Now().Format(time.RFC3339),
			"note":         "该 SDK 已内置产品标识，注册/心跳默认携带，与签发 License 的 product 一致。",
		}
		infoBytes, _ := json.MarshalIndent(info, "", "  ")
		w, err := zw.Create(root + sdkInfoFile)
		if err != nil {
			return nil, err
		}
		if _, err := w.Write(infoBytes); err != nil {
			return nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// isTextFile 判断 zip 内文件是否文本（仅对文本文件做占位符替换）
func isTextFile(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".go", ".java", ".py", ".c", ".h", ".md", ".txt", ".json", ".xml", ".yml", ".yaml", ".properties", ".imports", ".gitignore", ".sh", ".bat":
		return true
	}
	return false
}

// walkEmbed 遍历 embed.FS 目录
func walkEmbed(fs embed.FS, root string, fn func(path string, isDir bool) error) error {
	entries, err := fs.ReadDir(root)
	if err != nil {
		return err
	}
	for _, e := range entries {
		p := root + "/" + e.Name()
		if e.IsDir() {
			if err := fn(p, true); err != nil {
				return err
			}
			if err := walkEmbed(fs, p, fn); err != nil {
				return err
			}
		} else {
			if err := fn(p, false); err != nil {
				return err
			}
		}
	}
	return nil
}

// ---------- HTTP ----------

// handleProducts 产品列表（含台账授权数）
func (s *IssuerServer) handleProducts(w http.ResponseWriter, _ *http.Request) {
	// 历史台账 product 自动导入产品库（迁移）
	seen := map[string]bool{}
	for _, r := range s.store.All() {
		if r.Product == "" || seen[r.Product] {
			continue
		}
		seen[r.Product] = true
		s.products.EnsureProduct(r.Product)
	}
	list := s.products.All()
	byProduct := map[string]int{}
	for _, r := range s.store.All() {
		if r.Product != "" {
			byProduct[r.Product]++
		}
	}
	for i := range list {
		list[i].LicenseCount = byProduct[list[i].ID]
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "total": len(list), "products": list, "sdkLanguages": sdkLanguages})
}

type productReq struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	SDKs        []string `json:"sdks"`
}

// handleCreateProduct 新建产品
func (s *IssuerServer) handleCreateProduct(w http.ResponseWriter, r *http.Request) {
	var req productReq
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"success": false, "message": "请求体解析失败: " + err.Error()})
		return
	}
	req.ID = strings.TrimSpace(req.ID)
	if req.ID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"success": false, "message": "产品 ID 不能为空"})
		return
	}
	// 产品 ID 约束：字母数字-，防止与路径/其他产品冲突
	for _, ch := range req.ID {
		if !(ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z' || ch >= '0' && ch <= '9' || ch == '-' || ch == '_' || ch == '.') {
			writeJSON(w, http.StatusBadRequest, map[string]any{"success": false, "message": "产品 ID 仅允许字母/数字/'-'/'_'/'.'"})
			return
		}
	}
	if req.Name == "" {
		req.Name = req.ID
	}
	p := ProductInfo{ID: req.ID, Name: req.Name, Description: req.Description, SDKs: req.SDKs}
	if err := s.products.Upsert(p, false); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"success": false, "message": err.Error()})
		return
	}
	// 返回规范化后的产品（SDKs 已按产品库规则回填）
	saved, _ := s.products.Get(req.ID)
	slog.Info("🆕 产品已创建", "id", req.ID)
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "message": "产品已创建", "product": saved})
}

// handleUpdateProduct 更新产品
func (s *IssuerServer) handleUpdateProduct(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req productReq
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"success": false, "message": "请求体解析失败: " + err.Error()})
		return
	}
	old, ok := s.products.Get(id)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{"success": false, "message": "产品不存在: " + id})
		return
	}
	if req.Name != "" {
		old.Name = req.Name
	}
	if req.Description != "" || len(req.SDKs) > 0 {
		old.Description = req.Description
		old.SDKs = req.SDKs
	}
	if err := s.products.Upsert(old, true); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"success": false, "message": err.Error()})
		return
	}
	slog.Info("✏️ 产品已更新", "id", id)
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "message": "产品已更新", "product": old})
}

// handleDeleteProduct 删除产品（有授权记录或已被客户绑定时拒绝）
func (s *IssuerServer) handleDeleteProduct(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	hasLicense := false
	for _, rec := range s.store.All() {
		if rec.Product == id {
			hasLicense = true
			break
		}
	}
	if bound := s.cpStore.ProductBoundCount(id); bound > 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"success": false, "message": fmt.Sprintf("该产品已被 %d 个客户绑定，禁止删除（请先在「客户-产品对应」中解绑）", bound)})
		return
	}
	if err := s.products.Delete(id, hasLicense); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"success": false, "message": err.Error()})
		return
	}
	slog.Info("🗑️ 产品已删除", "id", id)
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "message": "产品已删除"})
}

// handleSDKDownload 下载指定语言 SDK zip。
// 查询参数（均可选，向后兼容）：
//
//	product:  产品 ID。传入则下载「定制版」：SDK 源码内嵌该产品默认标识
//	          （注册/心跳自动携带），zip 根目录附 sdk-info.json，文件名带产品。
//	customer: 客户名称。传入则归档 SDK 副本到 archive/{customer}/{product}/sdk/
//	          并记录下载日志（客户维度归档）。
func (s *IssuerServer) handleSDKDownload(w http.ResponseWriter, r *http.Request) {
	lang := strings.ToLower(strings.TrimSpace(r.PathValue("lang")))
	product := strings.TrimSpace(r.URL.Query().Get("product"))
	customer := strings.TrimSpace(r.URL.Query().Get("customer"))

	// 定制版必须校验产品存在于产品库
	if product != "" {
		if _, ok := s.products.Get(product); !ok {
			writeJSON(w, http.StatusBadRequest, map[string]any{"success": false, "message": "产品不存在: " + product + "（请先在产品库创建）"})
			return
		}
	}
	data, err := buildSDKZip(lang, product)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"success": false, "message": err.Error()})
		return
	}
	base, ok := sdkLanguages[lang]
	if !ok {
		writeJSON(w, http.StatusBadRequest, map[string]any{"success": false, "message": "不支持的语言: " + lang})
		return
	}
	filename := base + ".zip"
	if product != "" {
		filename = base + "-" + product + ".zip"
	}
	// 客户维度归档（需同时指定产品；customer 留空仅下载不归档）
	if customer != "" && product != "" && s.archive != nil {
		if err := s.archive.SaveSDK(customer, product, lang, filename, data); err != nil {
			slog.Warn("⚠️ SDK 归档失败", "customer", customer, "product", product, "lang", lang, "err", err)
		}
	}
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	w.Header().Set("Content-Length", fmt.Sprint(len(data)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}
