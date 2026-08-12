package license

import (
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/BrianApple/Licen/internal/machine"
)

// Manager 授权状态管理器：加载 License、采集机器码、校验、热重载。
//
// 支持两种部署形态：
//   - 单文件模式：licenseFile 指向一个 .json 文件（如 license.json），
//     激活/校验使用该文件（向后兼容旧部署）。
//   - 多产品目录模式：licenseFile 指向一个目录（如 licenses/），
//     目录下每个 <product>.json 对应一个产品的 License，各产品独立激活、
//     独立校验、互不影响（私有化部署同时使用多个产品的场景）。
type Manager struct {
	mu sync.RWMutex

	PublicKey *rsa.PublicKey

	machineCode string
	salt        string
	licenseFile string

	// 单文件模式字段（向后兼容）
	license *Model
	result  ValidationResult

	// 多产品模式字段：product → License + 校验结果
	licenses map[string]*Model
	results  map[string]ValidationResult

	hardware machine.HardwareInfo
}

// NewManager 创建管理器并加载 License（公钥优先取外部文件 ./keys/public.pem，否则内置）
func NewManager(salt, licenseFile, embeddedPublicKey string) (*Manager, error) {
	hw := machine.Collect(salt)
	m := &Manager{
		salt:        salt,
		licenseFile: licenseFile,
		machineCode: hw.MachineCode,
		hardware:    hw,
		licenses:    make(map[string]*Model),
		results:     make(map[string]ValidationResult),
	}

	// 公钥加载：外部文件优先，其次内置（构建时 -ldflags 注入或默认）
	if embeddedPublicKey != "" {
		pub, err := ParsePublicKey(embeddedPublicKey)
		if err != nil {
			return nil, fmt.Errorf("内置公钥解析失败: %w", err)
		}
		m.PublicKey = pub
		slog.Info("🔐 使用内置公钥")
	} else {
		return nil, fmt.Errorf("未提供授权服务公钥（构建时通过 -ldflags 注入）")
	}

	slog.Info("🔑 本机机器码", "machineCode", m.machineCode)
	if m.machineCode == "" {
		slog.Warn("⚠️ 无法采集到任何硬件指纹，License 将永远 MACHINE_MISMATCH", "hint", "请以 root 运行 licen-server，检查 /sys/class/dmi/id 与 /sys/block 权限")
	}
	return m, nil
}

// isDirMode licenseFile 是否多产品目录模式
func (m *Manager) isDirMode() bool {
	fi, err := os.Stat(m.licenseFile)
	if err == nil && fi.IsDir() {
		return true
	}
	// 目录尚未创建时：以 .json 结尾视为单文件，否则视为目录
	return !strings.HasSuffix(m.licenseFile, ".json")
}

// productFile 目录模式下某产品的 License 文件路径
func (m *Manager) productFile(product string) string {
	if product == "" {
		product = "default"
	}
	return filepath.Join(m.licenseFile, product+".json")
}

// Reload 重新加载并校验 License 文件（单文件或目录下全部）
func (m *Manager) Reload() bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.isDirMode() {
		lic, err := Load(m.licenseFile)
		if err != nil {
			if !os.IsNotExist(err) {
				slog.Warn("⚠️ License 文件读取失败", "err", err.Error())
			}
			m.license = nil
			m.result = Missing
			return false
		}
		m.license = lic
		m.result = Validate(lic, m.PublicKey, m.machineCode, time.Now())
		slog.Info("📄 License 加载", "file", m.licenseFile, "result", m.result.String())
		return m.result == Valid
	}

	// 多产品目录模式：加载目录下所有 *.json
	m.licenses = make(map[string]*Model)
	m.results = make(map[string]ValidationResult)
	entries, err := os.ReadDir(m.licenseFile)
	if err != nil {
		if os.IsNotExist(err) {
			return false // 目录未创建，视为未激活
		}
		slog.Warn("⚠️ License 目录读取失败", "err", err.Error())
		return false
	}
	anyValid := false
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		lic, err := Load(filepath.Join(m.licenseFile, e.Name()))
		if err != nil {
			slog.Warn("⚠️ License 文件解析失败", "file", e.Name(), "err", err.Error())
			continue
		}
		res := Validate(lic, m.PublicKey, m.machineCode, time.Now())
		// 以签名内容 product 为唯一事实（文件名仅作展示；不一致时告警，防改名错乱）
		key := lic.Product
		if key == "" {
			key = strings.TrimSuffix(e.Name(), ".json") // 兼容旧 License 无 product 字段
		}
		if key != strings.TrimSuffix(e.Name(), ".json") {
			slog.Warn("⚠️ License 文件名与内容 product 不一致（以签名内容为准）", "file", e.Name(), "contentProduct", lic.Product)
		}
		m.licenses[key] = lic
		m.results[key] = res
		if res == Valid {
			anyValid = true
		}
		slog.Info("📄 License 加载", "file", e.Name(), "product", key, "result", res.String())
	}
	if len(m.licenses) == 0 {
		slog.Info("📄 License 目录为空", "dir", m.licenseFile)
	}
	return anyValid
}

// Activate 通过上传的 License 内容激活：解析 → 验签 → 机器码匹配 → 有效期，
// 全部通过后写入 license 文件并立即生效。
// 该接口不依赖管理 Token——安全性由 RSA 签名 + 机器码绑定保证（只有厂商能签发）。
// 目录模式下按产品写入 <product>.json，各产品独立共存互不覆盖。
func (m *Manager) Activate(content []byte) ValidationResult {
	lic, err := LoadBytes(content)
	if err != nil {
		slog.Warn("⚠️ License 激活失败：内容解析错误", "err", err.Error())
		return Missing
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	result := Validate(lic, m.PublicKey, m.machineCode, time.Now())
	if result != Valid {
		slog.Warn("⚠️ License 激活被拒绝", "licenseId", lic.LicenseID, "result", result.String())
		return result
	}

	if !m.isDirMode() {
		if err := os.WriteFile(m.licenseFile, content, 0o644); err != nil {
			slog.Error("❌ License 激活失败：写入文件错误", "err", err.Error())
			return Missing
		}
		m.license = lic
		m.result = result
		slog.Info("✅ License 激活成功", "licenseId", lic.LicenseID, "file", m.licenseFile)
		return result
	}

	// 目录模式：按产品写入独立文件（先确保目录存在）
	dir := m.licenseFile
	if err := os.MkdirAll(dir, 0o755); err != nil {
		slog.Error("❌ License 激活失败：创建目录错误", "err", err.Error())
		return Missing
	}
	product := lic.Product
	if product == "" {
		product = "default"
	}
	f := m.productFile(product)
	if err := os.WriteFile(f, content, 0o644); err != nil {
		slog.Error("❌ License 激活失败：写入文件错误", "err", err.Error())
		return Missing
	}
	m.licenses[product] = lic
	m.results[product] = result
	slog.Info("✅ License 激活成功", "licenseId", lic.LicenseID, "product", product, "file", f)
	return result
}

// ActivateFromFile 从文件路径读取并激活（管理接口 reload 的增强版）
func (m *Manager) ActivateFromFile(path string) ValidationResult {
	data, err := os.ReadFile(path)
	if err != nil {
		slog.Warn("⚠️ License 文件读取失败", "err", err.Error())
		return Missing
	}
	return m.Activate(data)
}

// IsValid 是否有任一有效 License（单文件=该文件有效；目录=任一产品有效）
func (m *Manager) IsValid() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.result == Valid || m.anyValidLocked()
}

func (m *Manager) anyValidLocked() bool {
	for _, res := range m.results {
		if res == Valid {
			return true
		}
	}
	return false
}

// License 当前 License（只读副本）。多产品模式下返回第一个有效产品（无则任意一个）。
func (m *Manager) License() *Model {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.license != nil {
		return m.license
	}
	// 优先返回有效产品（确定性：按 product 排序取第一个有效，避免 map 随机）
	var valid []*Model
	for _, lic := range m.licenses {
		if res, ok := m.results[lic.Product]; ok && res == Valid {
			valid = append(valid, lic)
		}
	}
	if len(valid) > 0 {
		sort.Slice(valid, func(i, j int) bool { return valid[i].Product < valid[j].Product })
		return valid[0]
	}
	for _, lic := range m.licenses {
		return lic
	}
	return nil
}

// ResultFor 指定产品的校验结果（目录模式；单文件模式仅当 product 匹配时返回）
func (m *Manager) ResultFor(product string) ValidationResult {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if !m.isDirMode() {
		if m.license != nil && (product == "" || m.license.Product == product) {
			return m.result
		}
		return Missing
	}
	res, ok := m.results[product]
	if !ok {
		return Missing
	}
	return res
}

// LicenseFor 指定产品的 License（目录模式；单文件模式仅当 product 匹配时返回）
func (m *Manager) LicenseFor(product string) *Model {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if !m.isDirMode() {
		if m.license != nil && (product == "" || m.license.Product == product) {
			return m.license
		}
		return nil
	}
	return m.licenses[product]
}

// Products 目录模式下全部已激活产品列表（含校验结果）
func (m *Manager) Products() []ProductStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]ProductStatus, 0, len(m.licenses))
	for p, lic := range m.licenses {
		out = append(out, ProductStatus{
			Product: p,
			License: lic,
			Result:  m.results[p],
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Product < out[j].Product })
	return out
}

// ProductStatus 单个产品的授权状态
type ProductStatus struct {
	Product string
	License *Model
	Result  ValidationResult
}

// Result 当前校验结果（单文件模式）。多产品模式返回任一有效=Valid，否则第一个非 Valid。
func (m *Manager) Result() ValidationResult {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.license != nil {
		return m.result
	}
	for _, res := range m.results {
		if res == Valid {
			return Valid
		}
	}
	for _, res := range m.results {
		return res
	}
	return Missing
}

// MachineCode 本机机器码
func (m *Manager) MachineCode() string {
	return m.machineCode
}

// Hardware 返回采集到的硬件明细（主板/CPU/MAC/磁盘/UUID，供展示与排查）
func (m *Manager) Hardware() machine.HardwareInfo {
	return m.hardware
}

// MaxNodes 最大并发节点数。
// 单文件=该 License；目录=所有有效产品中 MaxNodes 之和（多产品各自限额、总和不超）。
func (m *Manager) MaxNodes() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if !m.isDirMode() {
		if m.license == nil {
			return 0
		}
		return m.license.MaxNodes
	}
	total := 0
	for p, lic := range m.licenses {
		if m.results[p] == Valid {
			total += lic.MaxNodes
		}
	}
	return total
}

// MaxNodesFor 指定产品的最大并发节点数
func (m *Manager) MaxNodesFor(product string) int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if lic, ok := m.licenses[product]; ok && m.results[product] == Valid {
		return lic.MaxNodes
	}
	if !m.isDirMode() && m.license != nil && (product == "" || m.license.Product == product) && m.result == Valid {
		return m.license.MaxNodes
	}
	return 0
}

// HasFeature License 是否包含指定功能点（任意产品）
func (m *Manager) HasFeature(feature string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.license != nil && m.license.Features != nil {
		for _, f := range m.license.Features {
			if f == feature {
				return true
			}
		}
	}
	for p, lic := range m.licenses {
		if m.results[p] != Valid || lic.Features == nil {
			continue
		}
		for _, f := range lic.Features {
			if f == feature {
				return true
			}
		}
	}
	return false
}

// HasFeatureFor 指定产品是否包含功能点
func (m *Manager) HasFeatureFor(product, feature string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	lic, ok := m.licenses[product]
	if !ok {
		if !m.isDirMode() && m.license != nil && (product == "" || m.license.Product == product) {
			lic = m.license
		} else {
			return false
		}
	}
	if lic.Features == nil {
		return false
	}
	for _, f := range lic.Features {
		if f == feature {
			return true
		}
	}
	return false
}

// marshalUnused 防止 json 未使用告警（保留字段序列化能力）
var _ = json.Marshal
