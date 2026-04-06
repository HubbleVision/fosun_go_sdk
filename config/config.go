// Package config provides configuration loading for FosunXCZ SDK
package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config 客户端配置
type Config struct {
	BaseURL            string // API基础地址
	APIKey             string // API密钥
	ClientPrivateKey   string // 客户端私钥(PEM格式)
	ServerPublicKey    string // 服务端公钥(PEM格式)
	SDKType            string // SDK类型: "" 或 "ops"
	RequestTimeout     int    // 请求超时时间(秒), 默认15
	MaxRetries         int    // 最大重试次数, 默认3
	RateLimitRequests  int    // 每秒允许请求数 (RPS), 0 表示不限速
	RateLimitBurst     int    // 突发请求数 (桶容量), 默认为 RateLimitRequests
}

// LoadConfig 从YAML文件加载配置
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	applyEnvOverrides(&cfg)
	cfg.SetDefaults()
	return &cfg, nil
}

// LoadConfigFromEnv 从环境变量加载配置
func LoadConfigFromEnv() *Config {
	cfg := &Config{}
	applyEnvOverrides(cfg)
	cfg.SetDefaults()
	return cfg
}

func applyEnvOverrides(cfg *Config) {
	if v := os.Getenv("FOSUN_BASE_URL"); v != "" {
		cfg.BaseURL = v
	}
	if v := os.Getenv("FOSUN_API_KEY"); v != "" {
		cfg.APIKey = v
	}
	if v := os.Getenv("FSOPENAPI_CLIENT_PRIVATE_KEY"); v != "" {
		cfg.ClientPrivateKey = v
	}
	if v := os.Getenv("FSOPENAPI_SERVER_PUBLIC_KEY"); v != "" {
		cfg.ServerPublicKey = v
	}
	if v := os.Getenv("SDK_TYPE"); v != "" {
		cfg.SDKType = v
	}
	if v := os.Getenv("FOSUN_RATE_LIMIT_REQUESTS"); v != "" {
		if val, err := parseInt(v); err == nil {
			cfg.RateLimitRequests = val
		}
	}
	if v := os.Getenv("FOSUN_RATE_LIMIT_BURST"); v != "" {
		if val, err := parseInt(v); err == nil {
			cfg.RateLimitBurst = val
		}
	}
}

func parseInt(s string) (int, error) {
	var val int
	_, err := fmt.Sscanf(s, "%d", &val)
	return val, err
}

func (c *Config) SetDefaults() {
	if c.RequestTimeout == 0 {
		c.RequestTimeout = 15
	}
	if c.MaxRetries == 0 {
		c.MaxRetries = 3
	}
}