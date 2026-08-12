// 客户维度归档：证书与 SDK 按「客户 / 产品」落盘归档，厂商可随时调取交付记录。
//
// 目录结构（默认 data/archive/）：
//
//	{root}/
//	  {customer}/                       # 客户（目录名 sanitize：保留字母数字/中文/-_.，其余转 _）
//	    {product}/                      # 产品
//	      licenses/                     # 证书（license.json 全文）
//	        {licenseId}.json
//	        {licenseId}.revoked         # 吊销标记（空文件，吊销时创建）
//	      sdk/                          # 该客户下载的 SDK 副本
//	        licen-sdk-go-1.0.0-{product}.zip
//	        .downloads.json             # 下载记录（追加式）
//
// 线程安全：sync.Mutex；写文件用临时文件 + rename 原子替换。
package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"
)

// ArchiveStore 客户维度归档存储
type ArchiveStore struct {
	mu   sync.Mutex
	root string
}

// NewArchiveStore 初始化归档目录（不存在则创建）
func NewArchiveStore(root string) (*ArchiveStore, error) {
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, fmt.Errorf("创建归档目录失败: %w", err)
	}
	return &ArchiveStore{root: root}, nil
}

// sanitizeName 目录名安全化：保留 Unicode 字母/数字与 -_.，其余替换为 _；
// 空值或纯符号回退 unknown；同时破坏可能的路径穿越序列（..）。
func sanitizeName(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "unknown"
	}
	var b strings.Builder
	for _, r := range s {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_' || r == '.':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	out := strings.ReplaceAll(b.String(), "..", "_")
	if out == "" || out == "." {
		return "unknown"
	}
	return out
}

// safePath 客户/产品/文件 → 归档内绝对路径（sanitize + 防穿越）
func (a *ArchiveStore) safePath(parts ...string) (string, error) {
	segs := make([]string, 0, len(parts))
	for _, p := range parts {
		p = sanitizeName(p)
		if p == "" || p == "." || strings.Contains(p, "/") || strings.Contains(p, "\\") {
			return "", fmt.Errorf("非法路径片段: %q", p)
		}
		segs = append(segs, p)
	}
	return filepath.Join(append([]string{a.root}, segs...)...), nil
}

// SaveLicense 保存证书文件到 archive/{customer}/{product}/licenses/{licenseId}.json
func (a *ArchiveStore) SaveLicense(customer, product, licenseID string, content []byte) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	dir, err := a.safePath(customer, product, "licenses")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return atomicWrite(filepath.Join(dir, sanitizeName(licenseID)+".json"), content)
}

// MarkRevoked 吊销标记：archive/{customer}/{product}/licenses/{licenseId}.revoked
func (a *ArchiveStore) MarkRevoked(customer, product, licenseID string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	dir, err := a.safePath(customer, product, "licenses")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return atomicWrite(filepath.Join(dir, sanitizeName(licenseID)+".revoked"), []byte("revoked "+time.Now().Format(time.RFC3339)+"\n"))
}

// SDKDownloadRecord 一条 SDK 下载记录
type SDKDownloadRecord struct {
	File         string    `json:"file"`    // zip 文件名
	Lang         string    `json:"lang"`    // 语言
	Product      string    `json:"product"` // 产品
	Size         int64     `json:"size"`    // 字节数
	DownloadedAt time.Time `json:"downloadedAt"`
}

// SaveSDK 归档 SDK zip 副本到 archive/{customer}/{product}/sdk/ 并追加下载记录。
// 同名文件已存在时跳过文件写入（避免重复占盘），但记录仍追加。
func (a *ArchiveStore) SaveSDK(customer, product, lang, filename string, content []byte) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	dir, err := a.safePath(customer, product, "sdk")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	fp := filepath.Join(dir, sanitizeName(filename))
	if _, err := os.Stat(fp); os.IsNotExist(err) {
		if err := atomicWrite(fp, content); err != nil {
			return err
		}
	}
	// 下载记录（.downloads.json 追加）
	recs := a.readDownloadRecords(dir)
	recs = append(recs, SDKDownloadRecord{
		File:         sanitizeName(filename),
		Lang:         lang,
		Product:      product,
		Size:         int64(len(content)),
		DownloadedAt: time.Now(),
	})
	return atomicWrite(filepath.Join(dir, ".downloads.json"), mustJSON(recs))
}

// readDownloadRecords 读取目录下载记录（.downloads.json，不存在返回空）
func (a *ArchiveStore) readDownloadRecords(dir string) []SDKDownloadRecord {
	data, err := os.ReadFile(filepath.Join(dir, ".downloads.json"))
	if err != nil {
		return nil
	}
	var recs []SDKDownloadRecord
	_ = json.Unmarshal(data, &recs)
	return recs
}

// ---------- 归档树 ----------

// ArchiveLicense 归档证书项
type ArchiveLicense struct {
	File      string    `json:"file"` // 文件名（xxx.json 或 xxx.json.revoked）
	LicenseID string    `json:"licenseId"`
	Revoked   bool      `json:"revoked"`
	Size      int64     `json:"size"`
	Modified  time.Time `json:"modified"`
}

// ArchiveSDK 归档 SDK 项
type ArchiveSDK struct {
	File         string    `json:"file"`
	Lang         string    `json:"lang"`
	Size         int64     `json:"size"`
	Modified     time.Time `json:"modified"`
	DownloadedAt time.Time `json:"downloadedAt"`
}

// ArchiveProduct 客户下某产品的归档
type ArchiveProduct struct {
	Product  string           `json:"product"`
	Licenses []ArchiveLicense `json:"licenses"`
	SDKs     []ArchiveSDK     `json:"sdks"`
}

// ArchiveCustomer 一个客户的归档
type ArchiveCustomer struct {
	Customer string           `json:"customer"`
	Products []ArchiveProduct `json:"products"`
}

// Tree 返回完整归档树（客户 → 产品 → 证书/SDK，按名称排序）
func (a *ArchiveStore) Tree() ([]ArchiveCustomer, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	customers, err := os.ReadDir(a.root)
	if err != nil {
		return nil, err
	}
	var out []ArchiveCustomer
	for _, c := range customers {
		if !c.IsDir() || strings.HasPrefix(c.Name(), ".") {
			continue
		}
		ac := ArchiveCustomer{Customer: c.Name()}
		prodDirs, err := os.ReadDir(filepath.Join(a.root, c.Name()))
		if err != nil {
			continue
		}
		for _, p := range prodDirs {
			if !p.IsDir() || strings.HasPrefix(p.Name(), ".") {
				continue
			}
			ap := ArchiveProduct{Product: p.Name()}
			base := filepath.Join(a.root, c.Name(), p.Name())
			// licenses
			if licDir, err := os.ReadDir(filepath.Join(base, "licenses")); err == nil {
				for _, lf := range licDir {
					if lf.IsDir() {
						continue
					}
					name := lf.Name()
					revoked := strings.HasSuffix(name, ".revoked")
					licenseID := strings.TrimSuffix(strings.TrimSuffix(name, ".json"), ".revoked")
					if !strings.HasSuffix(name, ".json") && !revoked {
						continue
					}
					info, _ := lf.Info()
					ap.Licenses = append(ap.Licenses, ArchiveLicense{
						File:      name,
						LicenseID: licenseID,
						Revoked:   revoked,
						Size:      info.Size(),
						Modified:  info.ModTime(),
					})
				}
				sort.Slice(ap.Licenses, func(i, j int) bool { return ap.Licenses[i].File < ap.Licenses[j].File })
			}
			// sdk（zip 副本；.downloads.json 仅作记录，不列出为 SDK 项）
			if sdkDir, err := os.ReadDir(filepath.Join(base, "sdk")); err == nil {
				for _, sf := range sdkDir {
					if sf.IsDir() || strings.HasPrefix(sf.Name(), ".") {
						continue
					}
					if !strings.HasSuffix(sf.Name(), ".zip") {
						continue
					}
					info, _ := sf.Info()
					ap.SDKs = append(ap.SDKs, ArchiveSDK{
						File:     sf.Name(),
						Lang:     sdkLangFromZip(sf.Name()),
						Size:     info.Size(),
						Modified: info.ModTime(),
					})
				}
				sort.Slice(ap.SDKs, func(i, j int) bool { return ap.SDKs[i].File < ap.SDKs[j].File })
			}
			if len(ap.Licenses) > 0 || len(ap.SDKs) > 0 {
				ac.Products = append(ac.Products, ap)
			}
		}
		sort.Slice(ac.Products, func(i, j int) bool { return ac.Products[i].Product < ac.Products[j].Product })
		if len(ac.Products) > 0 {
			out = append(out, ac)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Customer < out[j].Customer })
	return out, nil
}

// sdkLangFromZip 从 zip 文件名推断语言（licen-sdk-{lang}-{ver}[-{product}].zip）
func sdkLangFromZip(name string) string {
	base := strings.TrimSuffix(name, ".zip")
	parts := strings.Split(base, "-")
	if len(parts) >= 3 && parts[0] == "licen" && parts[1] == "sdk" {
		return parts[2]
	}
	return ""
}

// OpenLicense 读取归档证书文件（校验路径合法）
func (a *ArchiveStore) OpenLicense(customer, product, file string) ([]byte, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	dir, err := a.safePath(customer, product, "licenses")
	if err != nil {
		return nil, err
	}
	return readSafeFile(dir, file)
}

// OpenSDK 读取归档 SDK 副本
func (a *ArchiveStore) OpenSDK(customer, product, file string) ([]byte, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	dir, err := a.safePath(customer, product, "sdk")
	if err != nil {
		return nil, err
	}
	return readSafeFile(dir, file)
}

// readSafeFile 目录内安全读文件（拒绝子目录/穿越）
func readSafeFile(dir, name string) ([]byte, error) {
	name = sanitizeName(name)
	if name == "" || name == "." || strings.ContainsAny(name, "/\\") {
		return nil, fmt.Errorf("非法文件名: %q", name)
	}
	return os.ReadFile(filepath.Join(dir, name))
}

// ---------- HTTP ----------

// handleArchive 客户维度归档树（客户 → 产品 → 证书/SDK）
func (s *IssuerServer) handleArchive(w http.ResponseWriter, _ *http.Request) {
	tree, err := s.archive.Tree()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"success": false, "message": "归档读取失败: " + err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "customers": tree})
}

// handleArchiveLicenseDownload 下载归档证书文件
func (s *IssuerServer) handleArchiveLicenseDownload(w http.ResponseWriter, r *http.Request) {
	customer := r.PathValue("customer")
	product := r.PathValue("product")
	file := r.PathValue("file")
	data, err := s.archive.OpenLicense(customer, product, file)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"success": false, "message": "归档证书不存在: " + err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", sanitizeName(file)))
	w.Header().Set("Content-Length", fmt.Sprint(len(data)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

// handleArchiveSDKDownload 下载归档 SDK 副本
func (s *IssuerServer) handleArchiveSDKDownload(w http.ResponseWriter, r *http.Request) {
	customer := r.PathValue("customer")
	product := r.PathValue("product")
	file := r.PathValue("file")
	data, err := s.archive.OpenSDK(customer, product, file)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"success": false, "message": "归档 SDK 不存在: " + err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", sanitizeName(file)))
	w.Header().Set("Content-Length", fmt.Sprint(len(data)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

// ---------- 工具 ----------

// atomicWrite 临时文件 + rename 原子写
func atomicWrite(path string, content []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, content, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// mustJSON json 序列化（失败返回空数组文本，不影响归档主流程）
func mustJSON(v any) []byte {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return []byte("[]")
	}
	return b
}
