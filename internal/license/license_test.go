package license

import (
	"crypto/rsa"
	"testing"
	"time"
)

func buildLicense(t *testing.T, priv *rsa.PrivateKey, machineCode, expiresAt string) *Model {
	t.Helper()
	m := &Model{
		LicenseID:   "LIC-TEST-0001",
		Product:     "ai-engine",
		Edition:     "enterprise",
		MachineCode: machineCode,
		MaxNodes:    10,
		Features:    []string{"ai-inference", "nlp"},
		IssuedAt:    time.Now().Add(-24 * time.Hour).Format(time.RFC3339Nano),
		ExpiresAt:   expiresAt,
		Customer:    "测试客户",
	}
	if err := m.SignWith(priv); err != nil {
		t.Fatalf("签名失败: %v", err)
	}
	return m
}

func newKeyPair(t *testing.T) (*rsa.PrivateKey, *rsa.PublicKey) {
	t.Helper()
	priv, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("密钥生成失败: %v", err)
	}
	return priv, &priv.PublicKey
}

func TestSignAndVerify(t *testing.T) {
	priv, pub := newKeyPair(t)
	lic := buildLicense(t, priv, "abc123", time.Now().Add(365*24*time.Hour).Format(time.RFC3339Nano))

	if !lic.Verify(pub) {
		t.Fatal("验签应通过")
	}
	if result := Validate(lic, pub, "abc123", time.Now()); result != Valid {
		t.Fatalf("校验应通过，得到: %s", result)
	}
}

func TestTamperedFieldFails(t *testing.T) {
	priv, pub := newKeyPair(t)
	lic := buildLicense(t, priv, "abc123", time.Now().Add(365*24*time.Hour).Format(time.RFC3339Nano))

	lic.MaxNodes = 999 // 篡改
	if lic.Verify(pub) {
		t.Fatal("篡改后验签应失败")
	}
	if result := Validate(lic, pub, "abc123", time.Now()); result != InvalidSignature {
		t.Fatalf("应为 INVALID_SIGNATURE，得到: %s", result)
	}
}

func TestMachineMismatch(t *testing.T) {
	priv, pub := newKeyPair(t)
	lic := buildLicense(t, priv, "machine-A", time.Now().Add(365*24*time.Hour).Format(time.RFC3339Nano))

	if result := Validate(lic, pub, "machine-B", time.Now()); result != MachineMismatch {
		t.Fatalf("应为 MACHINE_MISMATCH，得到: %s", result)
	}
}

func TestExpired(t *testing.T) {
	priv, pub := newKeyPair(t)
	lic := buildLicense(t, priv, "abc123", time.Now().Add(-24*time.Hour).Format(time.RFC3339Nano))

	if result := Validate(lic, pub, "abc123", time.Now()); result != Expired {
		t.Fatalf("应为 EXPIRED，得到: %s", result)
	}
}

func TestWrongKeyFails(t *testing.T) {
	_, pub := newKeyPair(t)          // 正规公钥
	attackerPriv, _ := newKeyPair(t) // 攻击者密钥
	lic := buildLicense(t, attackerPriv, "abc123",
		time.Now().Add(365*24*time.Hour).Format(time.RFC3339Nano))

	if lic.Verify(pub) {
		t.Fatal("攻击者自签的 License 用正规公钥验签应失败")
	}
}

func TestKeyRoundTrip(t *testing.T) {
	priv, _ := newKeyPair(t)
	pubB64 := PublicKeyBase64(priv)
	parsed, err := ParsePublicKey(pubB64)
	if err != nil {
		t.Fatalf("公钥解析失败: %v", err)
	}
	lic := buildLicense(t, priv, "abc123", time.Now().Add(365*24*time.Hour).Format(time.RFC3339Nano))
	if !lic.Verify(parsed) {
		t.Fatal("往返后验签应通过")
	}
}

func TestSkipMachineCheckWhenEmpty(t *testing.T) {
	priv, pub := newKeyPair(t)
	lic := buildLicense(t, priv, "abc123", time.Now().Add(365*24*time.Hour).Format(time.RFC3339Nano))

	// currentMachineCode 为空（licen-tool verify 场景）→ 跳过机器码匹配
	if result := Validate(lic, pub, "", time.Now()); result != Valid {
		t.Fatalf("空机器码应跳过匹配，得到: %s", result)
	}
}
