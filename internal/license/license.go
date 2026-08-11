// Package license License 数据模型 + RSA 验签 + 有效期校验。
package license

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"os"
	"time"
)

// Model License 数据模型。
// JSON 字段顺序固定（与签名 JSON 一致，签名=去掉 Sign 字段后的规范 JSON）。
type Model struct {
	LicenseID   string   `json:"licenseId"`
	Product     string   `json:"product"`
	Edition     string   `json:"edition"`
	MachineCode string   `json:"machineCode"`
	MaxNodes    int      `json:"maxNodes"`
	Features    []string `json:"features"`
	IssuedAt    string   `json:"issuedAt"`
	ExpiresAt   string   `json:"expiresAt"`
	Customer    string   `json:"customer,omitempty"`
	Sign        string   `json:"sign,omitempty"`
}

// CanonicalJSON 生成待签名内容：去掉 Sign 字段的规范 JSON（字段顺序固定）
func (m *Model) CanonicalJSON() ([]byte, error) {
	sign := m.Sign
	m.Sign = ""
	defer func() { m.Sign = sign }()
	return json.Marshal(m)
}

// Verify 用公钥验签（SHA256withRSA = PKCS1v15 + SHA-256）
func (m *Model) Verify(pub *rsa.PublicKey) bool {
	if m.Sign == "" {
		return false
	}
	canonical, err := m.CanonicalJSON()
	if err != nil {
		return false
	}
	sig, err := base64.StdEncoding.DecodeString(m.Sign)
	if err != nil {
		return false
	}
	digest := sha256.Sum256(canonical)
	return rsa.VerifyPKCS1v15(pub, crypto.SHA256, digest[:], sig) == nil
}

// Sign 用私钥签名（licen-tool 签发时调用）
func (m *Model) SignWith(priv *rsa.PrivateKey) error {
	canonical, err := m.CanonicalJSON()
	if err != nil {
		return err
	}
	digest := sha256.Sum256(canonical)
	sig, err := rsa.SignPKCS1v15(nil, priv, crypto.SHA256, digest[:])
	if err != nil {
		return err
	}
	m.Sign = base64.StdEncoding.EncodeToString(sig)
	return nil
}

// ValidationResult 校验结果
type ValidationResult int

const (
	Valid ValidationResult = iota
	Expired
	NotYetValid
	MachineMismatch
	InvalidSignature
	ClockReversed
	Missing
)

func (r ValidationResult) String() string {
	switch r {
	case Valid:
		return "VALID"
	case Expired:
		return "EXPIRED"
	case NotYetValid:
		return "NOT_YET_VALID"
	case MachineMismatch:
		return "MACHINE_MISMATCH"
	case InvalidSignature:
		return "INVALID_SIGNATURE"
	case ClockReversed:
		return "CLOCK_REVERSED"
	default:
		return "MISSING"
	}
}

// clockSkewTolerance 时钟偏差容忍（5 分钟，避免 NTP 抖动误判）
const clockSkewTolerance = 5 * time.Minute

// Validate 完整校验：验签 → 机器码 → 有效期。
// currentMachineCode 传 "" 时跳过机器码匹配（licen-tool verify 场景）。
func Validate(m *Model, pub *rsa.PublicKey, currentMachineCode string, now time.Time) ValidationResult {
	if m == nil {
		return Missing
	}
	if !m.Verify(pub) {
		return InvalidSignature
	}
	if m.MachineCode != "" && currentMachineCode != "" && m.MachineCode != currentMachineCode {
		return MachineMismatch
	}
	issued, err1 := time.Parse(time.RFC3339Nano, m.IssuedAt)
	expires, err2 := time.Parse(time.RFC3339Nano, m.ExpiresAt)
	if err1 != nil || err2 != nil {
		return InvalidSignature
	}
	if now.Before(issued.Add(-clockSkewTolerance)) {
		return NotYetValid
	}
	if now.After(expires.Add(clockSkewTolerance)) {
		return Expired
	}
	return Valid
}

// Load 从文件加载 License
func Load(path string) (*Model, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m Model
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

// Save 保存 License 到文件
func Save(m *Model, path string) error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// ParsePublicKey 解析 Base64 编码的 X.509 公钥（与 licen-tool gen-keypair 输出兼容）
func ParsePublicKey(b64 string) (*rsa.PublicKey, error) {
	der, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, fmt.Errorf("公钥 Base64 解码失败: %w", err)
	}
	pub, err := x509.ParsePKIXPublicKey(der)
	if err != nil {
		return nil, fmt.Errorf("公钥解析失败: %w", err)
	}
	rsaPub, ok := pub.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("非 RSA 公钥")
	}
	return rsaPub, nil
}

// ParsePrivateKey 解析 Base64 编码的 PKCS#8 私钥
func ParsePrivateKey(b64 string) (*rsa.PrivateKey, error) {
	der, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, fmt.Errorf("私钥 Base64 解码失败: %w", err)
	}
	key, err := x509.ParsePKCS8PrivateKey(der)
	if err != nil {
		return nil, fmt.Errorf("私钥解析失败: %w", err)
	}
	rsaPriv, ok := key.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("非 RSA 私钥")
	}
	return rsaPriv, nil
}

// LoadPublicKeyFile 从文件加载公钥（Base64 纯文本或 PEM 均可）
func LoadPublicKeyFile(path string) (*rsa.PublicKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	content := string(data)
	if block, _ := pem.Decode(data); block != nil {
		content = base64.StdEncoding.EncodeToString(block.Bytes)
	}
	return ParsePublicKey(content)
}

// LoadPrivateKeyFile 从文件加载私钥（Base64 纯文本或 PEM 均可）
func LoadPrivateKeyFile(path string) (*rsa.PrivateKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	content := string(data)
	if block, _ := pem.Decode(data); block != nil {
		content = base64.StdEncoding.EncodeToString(block.Bytes)
	}
	return ParsePrivateKey(content)
}

// GenerateKeyPair 生成 RSA-2048 密钥对（licen-tool gen-keypair）
func GenerateKeyPair() (*rsa.PrivateKey, error) {
	return rsa.GenerateKey(rand.Reader, 2048)
}

// PublicKeyBase64 私钥 → 对应公钥的 Base64（X.509 PKIX）
func PublicKeyBase64(priv *rsa.PrivateKey) string {
	der, _ := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	return base64.StdEncoding.EncodeToString(der)
}

// PrivateKeyBase64 私钥 → Base64（PKCS#8）
func PrivateKeyBase64(priv *rsa.PrivateKey) string {
	der, _ := x509.MarshalPKCS8PrivateKey(priv)
	return base64.StdEncoding.EncodeToString(der)
}
