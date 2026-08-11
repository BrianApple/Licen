// licen-tool：厂商授权 CLI（Go 单二进制）。
//
// 用法：
//
//	licen-tool gen-keypair -d ./keys                  # 生成 RSA 密钥对
//	licen-tool machinecode                            # 查看本机机器码
//	licen-tool gen-license -k keys/private.pem -m <机器码> -p ai-engine -n 10 -d 365 -c "客户名" -o license.json
//	licen-tool verify -k keys/public.pem -l license.json
package main

import (
	"crypto/rand"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/BrianApple/Licen/internal/license"
	"github.com/BrianApple/Licen/internal/machine"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}
	var err error
	switch os.Args[1] {
	case "gen-keypair":
		err = cmdGenKeypair(os.Args[2:])
	case "gen-license":
		err = cmdGenLicense(os.Args[2:])
	case "verify":
		err = cmdVerify(os.Args[2:])
	case "machinecode":
		err = cmdMachinecode(os.Args[2:])
	case "help", "-h", "--help":
		usage()
		return
	default:
		usage()
		os.Exit(1)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "❌", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Print(`Licen 厂商授权工具 v1.0.0

用法:
  licen-tool gen-keypair [-d 输出目录]             生成 RSA-2048 密钥对
  licen-tool machinecode [-s 盐]                  显示本机机器码（客户VM上执行后报给厂商）
  licen-tool gen-license -k 私钥 -m 机器码 [-p 产品] [-e 版本] [-n 节点数] [-f 功能点] [-d 天数|--expires 到期时间] [-c 客户] [-o 输出]
  licen-tool verify -k 公钥 -l license文件         验证 License

示例:
  licen-tool gen-keypair -d ./keys
  licen-tool machinecode
  licen-tool gen-license -k keys/private.pem -m <机器码> -p ai-engine -n 10 -d 365 -c "某公司" -o license.json
  licen-tool verify -k keys/public.pem -l license.json
`)
}

// ---------- gen-keypair ----------

func cmdGenKeypair(args []string) error {
	fs := flag.NewFlagSet("gen-keypair", flag.ExitOnError)
	dir := fs.String("d", "./keys", "输出目录")
	fs.Parse(args)

	priv, err := license.GenerateKeyPair()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(*dir, 0o755); err != nil {
		return err
	}
	privPath := *dir + "/private.pem"
	pubPath := *dir + "/public.pem"
	if err := os.WriteFile(privPath, []byte(license.PrivateKeyBase64(priv)), 0o600); err != nil {
		return err
	}
	if err := os.WriteFile(pubPath, []byte(license.PublicKeyBase64(priv)), 0o644); err != nil {
		return err
	}
	fmt.Println("✅ 密钥对已生成：")
	fmt.Println("   私钥（厂商自留，勿外泄）: " + privPath)
	fmt.Println("   公钥（内置到授权服务）: " + pubPath)
	return nil
}

// ---------- machinecode ----------

func cmdMachinecode(args []string) error {
	fs := flag.NewFlagSet("machinecode", flag.ExitOnError)
	salt := fs.String("s", "", "自定义盐（与授权服务配置一致）")
	fs.Parse(args)

	info := machine.Collect(*salt)
	if info.MachineCode == "" {
		fmt.Println("⚠️  无法采集到任何硬件指纹（/sys 不可读或均为空）！")
		fmt.Println("   请以 root 运行，或检查 /sys/class/dmi/id 与 /sys/block 权限后重试。")
		return nil
	}
	fmt.Println("🔑 本机机器码: " + info.MachineCode)
	fmt.Println("   硬件信息: " + info.String())
	fmt.Println("   请将此机器码发送给厂商用于签发 License")
	return nil
}

// ---------- gen-license ----------

func cmdGenLicense(args []string) error {
	fs := flag.NewFlagSet("gen-license", flag.ExitOnError)
	privateKey := fs.String("k", "", "私钥文件路径（必填）")
	machineCode := fs.String("m", "", "绑定机器码（必填）")
	product := fs.String("p", "", "产品标识，如 ai-engine（必填）")
	edition := fs.String("e", "enterprise", "版本/套餐")
	maxNodes := fs.Int("n", 1, "最大并发节点数")
	features := fs.String("f", "", "功能点列表，逗号分隔，如 ai-inference,nlp")
	days := fs.Int("d", 0, "有效期（天），与 --expires 二选一")
	expires := fs.String("expires", "", "到期时间 ISO-8601，如 2027-08-11T00:00:00")
	customer := fs.String("c", "", "客户名称")
	output := fs.String("o", "license.json", "输出 License 文件路径")
	fs.Parse(args)

	if *privateKey == "" || *machineCode == "" || *product == "" {
		return fmt.Errorf("必须指定 -k 私钥、-m 机器码、-p 产品")
	}
	if *days <= 0 && *expires == "" {
		return fmt.Errorf("必须指定 -d 天数 或 --expires 到期时间")
	}

	priv, err := license.LoadPrivateKeyFile(*privateKey)
	if err != nil {
		return fmt.Errorf("私钥加载失败: %w", err)
	}

	m := &license.Model{
		LicenseID:   "LIC-" + randomID(8) + "-" + fmt.Sprint(time.Now().UnixMilli()),
		Product:     *product,
		Edition:     *edition,
		MachineCode: *machineCode,
		MaxNodes:    *maxNodes,
		IssuedAt:    time.Now().Format(time.RFC3339Nano),
		Customer:    *customer,
	}
	if *features != "" {
		for _, f := range strings.Split(*features, ",") {
			if f = strings.TrimSpace(f); f != "" {
				m.Features = append(m.Features, f)
			}
		}
	}
	if *expires != "" {
		t, err := time.Parse(time.RFC3339Nano, *expires)
		if err != nil {
			return fmt.Errorf("到期时间格式错误（应为 ISO-8601 如 2027-08-11T00:00:00）: %w", err)
		}
		m.ExpiresAt = t.Format(time.RFC3339Nano)
	} else {
		m.ExpiresAt = time.Now().AddDate(0, 0, *days).Format(time.RFC3339Nano)
	}

	if err := m.SignWith(priv); err != nil {
		return fmt.Errorf("签名失败: %w", err)
	}
	if err := license.Save(m, *output); err != nil {
		return fmt.Errorf("保存失败: %w", err)
	}

	fmt.Println("✅ License 已签发: " + *output)
	fmt.Println("   licenseId  : " + m.LicenseID)
	fmt.Println("   产品        : " + *product + " (" + *edition + ")")
	fmt.Println("   客户        : " + ifEmpty(*customer, "-"))
	fmt.Println("   机器码      : " + *machineCode)
	fmt.Println("   并发节点数  : " + fmt.Sprint(*maxNodes))
	fmt.Println("   功能点      : " + ifEmpty(*features, "-"))
	fmt.Println("   到期时间    : " + m.ExpiresAt)
	return nil
}

// ---------- verify ----------

func cmdVerify(args []string) error {
	fs := flag.NewFlagSet("verify", flag.ExitOnError)
	publicKey := fs.String("k", "", "公钥文件路径（必填）")
	licenseFile := fs.String("l", "", "License 文件路径（必填）")
	fs.Parse(args)

	if *publicKey == "" || *licenseFile == "" {
		return fmt.Errorf("必须指定 -k 公钥、-l License 文件")
	}
	pub, err := license.LoadPublicKeyFile(*publicKey)
	if err != nil {
		return fmt.Errorf("公钥加载失败: %w", err)
	}
	m, err := license.Load(*licenseFile)
	if err != nil {
		return fmt.Errorf("License 加载失败: %w", err)
	}

	fmt.Println("License: " + m.LicenseID)
	fmt.Println("产品: " + m.Product + " (" + m.Edition + ")")
	fmt.Println("客户: " + ifEmpty(m.Customer, "-"))
	fmt.Println("机器码: " + m.MachineCode)
	fmt.Println("节点数: " + fmt.Sprint(m.MaxNodes))
	fmt.Println("功能点: " + strings.Join(m.Features, ","))
	fmt.Println("到期: " + m.ExpiresAt)

	result := license.Validate(m, pub, "", time.Now())
	if result == license.Valid {
		fmt.Println("✅ 校验通过（签名有效）")
		return nil
	}
	return fmt.Errorf("校验失败: %s", result.String())
}

// ---------- 工具 ----------

func randomID(n int) string {
	const chars = "0123456789ABCDEF"
	b := make([]byte, n)
	randBytes := make([]byte, n)
	if _, err := rand.Read(randBytes); err != nil {
		for i := range b {
			b[i] = chars[(time.Now().UnixNano()>>(uint(i)*3))%16]
		}
		return string(b)
	}
	for i := range b {
		b[i] = chars[randBytes[i]%16]
	}
	return string(b)
}

func ifEmpty(s, def string) string {
	if s == "" {
		return def
	}
	return s
}
