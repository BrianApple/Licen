package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config 授权服务配置（config.yaml）
type Config struct {
	Server struct {
		Port int `yaml:"port"`
	} `yaml:"server"`
	Licen struct {
		// 机器码盐（与厂商签发 License 时一致，可选）
		Salt string `yaml:"salt"`
		// License 文件路径
		LicenseFile string `yaml:"license-file"`
		// 公钥文件路径（部署时替换；不存在则用构建内嵌公钥）
		PublicKeyFile string `yaml:"public-key-file"`
		// 心跳超时（秒）：超过此时长未心跳视为离线并回收名额
		HeartbeatTimeoutSeconds int64 `yaml:"heartbeat-timeout-seconds"`
		// 管理端 API Token（/api/v1/admin/** 需要 X-Admin-Token）
		AdminToken string `yaml:"admin-token"`
		// 客户端心跳 HMAC 签名校验开关
		HmacVerifyEnabled bool `yaml:"hmac-verify-enabled"`
		// SQLite 数据库文件路径
		DBPath string `yaml:"db-path"`
	} `yaml:"licen"`
}

// Default 返回默认配置
func Default() *Config {
	c := &Config{}
	c.Server.Port = 8090
	c.Licen.LicenseFile = "./license.json"
	c.Licen.PublicKeyFile = "./keys/public.pem"
	c.Licen.HeartbeatTimeoutSeconds = 90
	c.Licen.AdminToken = "licen-admin-2026"
	c.Licen.HmacVerifyEnabled = true
	c.Licen.DBPath = "./data/licen.db"
	return c
}

// Load 从 YAML 文件加载配置（文件不存在则用默认值）
func Load(path string) (*Config, error) {
	c := Default()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return c, nil
		}
		return nil, err
	}
	if err := yaml.Unmarshal(data, c); err != nil {
		return nil, fmt.Errorf("配置文件解析失败: %w", err)
	}
	return c, nil
}
