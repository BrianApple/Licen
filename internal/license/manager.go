package license

import (
	"crypto/rsa"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/BrianApple/Licen/internal/machine"
)

// Manager 授权状态管理器：加载 License、采集机器码、校验、热重载。
type Manager struct {
	mu sync.RWMutex

	PublicKey *rsa.PublicKey

	machineCode string
	salt        string
	licenseFile string
	license     *Model
	result      ValidationResult
	hardware    machine.HardwareInfo
}

// NewManager 创建管理器并加载 License（公钥优先取外部文件 ./keys/public.pem，否则内置）
func NewManager(salt, licenseFile, embeddedPublicKey string) (*Manager, error) {
	hw := machine.Collect(salt)
	m := &Manager{
		salt:        salt,
		licenseFile: licenseFile,
		machineCode: hw.MachineCode,
		hardware:    hw,
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
	return m, nil
}

// Reload 重新加载并校验 License 文件
func (m *Manager) Reload() bool {
	m.mu.Lock()
	defer m.mu.Unlock()

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

// Activate 通过上传的 License 内容激活：解析 → 验签 → 机器码匹配 → 有效期，
// 全部通过后写入 license 文件并立即生效。
// 该接口不依赖管理 Token——安全性由 RSA 签名 + 机器码绑定保证（只有厂商能签发）。
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
	if err := os.WriteFile(m.licenseFile, content, 0o644); err != nil {
		slog.Error("❌ License 激活失败：写入文件错误", "err", err.Error())
		return Missing
	}
	m.license = lic
	m.result = result
	slog.Info("✅ License 激活成功", "licenseId", lic.LicenseID, "file", m.licenseFile)
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

// IsValid License 是否有效
func (m *Manager) IsValid() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.result == Valid
}

// License 当前 License（只读副本）
func (m *Manager) License() *Model {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.license
}

// Result 当前校验结果
func (m *Manager) Result() ValidationResult {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.result
}

// MachineCode 本机机器码
func (m *Manager) MachineCode() string {
	return m.machineCode
}

// Hardware 返回采集到的硬件明细（主板/CPU/MAC/磁盘/UUID，供展示与排查）
func (m *Manager) Hardware() machine.HardwareInfo {
	return m.hardware
}

// MaxNodes 最大并发节点数（License 未加载为 0）
func (m *Manager) MaxNodes() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.license == nil {
		return 0
	}
	return m.license.MaxNodes
}

// HasFeature License 是否包含指定功能点
func (m *Manager) HasFeature(feature string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.license == nil || m.license.Features == nil {
		return false
	}
	for _, f := range m.license.Features {
		if f == feature {
			return true
		}
	}
	return false
}
