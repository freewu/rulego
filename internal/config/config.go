// Package config 负责规则引擎的配置管理：
// 支持 YAML 配置文件 + 环境变量（RULEGO_*）覆盖 + 内置默认值三级机制。
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config 是规则引擎的完整配置。
type Config struct {
	Server  ServerConfig  `yaml:"server" json:"server"`
	Storage StorageConfig `yaml:"storage" json:"storage"`
	Lua     LuaConfig     `yaml:"lua" json:"lua"`
}

// ServerConfig HTTP 服务配置。
type ServerConfig struct {
	Host      string `yaml:"host" json:"host"`
	Port      int    `yaml:"port" json:"port"`
	StaticDir string `yaml:"static_dir" json:"static_dir"`
}

// StorageConfig 规则存储配置。
type StorageConfig struct {
	RulesDir string `yaml:"rules_dir" json:"rules_dir"`
}

// LuaConfig Lua 运行时配置。
type LuaConfig struct {
	TimeoutSeconds int `yaml:"timeout_seconds" json:"timeout_seconds"`
}

// Default 返回默认配置。
func Default() *Config {
	return &Config{
		Server: ServerConfig{
			Host:      "0.0.0.0",
			Port:      8080,
			StaticDir: "web",
		},
		Storage: StorageConfig{
			RulesDir: "data/rules",
		},
		Lua: LuaConfig{
			TimeoutSeconds: 5,
		},
	}
}

// Load 从 YAML 文件加载配置；文件不存在时返回默认配置。
// 加载完成后应用 RULEGO_* 环境变量覆盖。
func Load(path string) (*Config, error) {
	cfg := Default()
	if path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				fmt.Fprintf(os.Stderr, "[config] 配置文件 %s 不存在，使用默认配置\n", path)
				return cfg, nil
			}
			return nil, fmt.Errorf("读取配置文件失败: %w", err)
		}
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("解析配置文件失败: %w", err)
		}
	}
	cfg.applyEnv()
	return cfg, nil
}

// applyEnv 用 RULEGO_<SECTION>_<KEY> 形式的环境变量覆盖配置。
// 例如 RULEGO_SERVER_PORT=9090、RULEGO_LUA_TIMEOUT_SECONDS=10。
func (c *Config) applyEnv() {
	if v := os.Getenv("RULEGO_SERVER_HOST"); v != "" {
		c.Server.Host = v
	}
	if v := os.Getenv("RULEGO_SERVER_PORT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			c.Server.Port = n
		}
	}
	if v := os.Getenv("RULEGO_SERVER_STATIC_DIR"); v != "" {
		c.Server.StaticDir = v
	}
	if v := os.Getenv("RULEGO_STORAGE_RULES_DIR"); v != "" {
		c.Storage.RulesDir = v
	}
	if v := os.Getenv("RULEGO_LUA_TIMEOUT_SECONDS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			c.Lua.TimeoutSeconds = n
		}
	}
}

// Addr 返回 HTTP 监听地址，如 "0.0.0.0:8080"。
func (c *Config) Addr() string {
	return fmt.Sprintf("%s:%d", c.Server.Host, c.Server.Port)
}

// Normalize 对配置做归一化与校验。
func (c *Config) Normalize() error {
	if c.Server.Port <= 0 || c.Server.Port > 65535 {
		return fmt.Errorf("无效端口: %d", c.Server.Port)
	}
	if c.Lua.TimeoutSeconds <= 0 {
		c.Lua.TimeoutSeconds = 5
	}
	return nil
}

// String 返回用于启动日志的配置摘要。
func (c *Config) String() string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "server: %s\n", c.Addr())
	fmt.Fprintf(&sb, "static_dir: %s\n", c.Server.StaticDir)
	fmt.Fprintf(&sb, "rules_dir: %s\n", c.Storage.RulesDir)
	fmt.Fprintf(&sb, "lua_timeout: %ds\n", c.Lua.TimeoutSeconds)
	return sb.String()
}
