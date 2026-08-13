package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_Default(t *testing.T) {
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Server.Port != 8080 {
		t.Errorf("默认端口 = %d, want 8080", cfg.Server.Port)
	}
	if cfg.Lua.TimeoutSeconds != 5 {
		t.Errorf("默认超时 = %d, want 5", cfg.Lua.TimeoutSeconds)
	}
	if cfg.Storage.RulesDir != "data/rules" {
		t.Errorf("默认规则目录 = %s", cfg.Storage.RulesDir)
	}
}

func TestLoad_FromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := `
server:
  port: 9090
  static_dir: "public"
storage:
  rules_dir: "myrules"
lua:
  timeout_seconds: 12
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Server.Port != 9090 || cfg.Server.StaticDir != "public" {
		t.Errorf("server 配置错误: %+v", cfg.Server)
	}
	if cfg.Storage.RulesDir != "myrules" {
		t.Errorf("storage 配置错误: %+v", cfg.Storage)
	}
	if cfg.Lua.TimeoutSeconds != 12 {
		t.Errorf("lua 配置错误: %+v", cfg.Lua)
	}
}

func TestLoad_EnvOverride(t *testing.T) {
	t.Setenv("RULEGO_SERVER_PORT", "7777")
	t.Setenv("RULEGO_LUA_TIMEOUT_SECONDS", "3")
	t.Setenv("RULEGO_STORAGE_VERSION_LIMIT", "7")
	cfg, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Server.Port != 7777 {
		t.Errorf("环境变量覆盖端口 = %d, want 7777", cfg.Server.Port)
	}
	if cfg.Lua.TimeoutSeconds != 3 {
		t.Errorf("环境变量覆盖超时 = %d, want 3", cfg.Lua.TimeoutSeconds)
	}
	if cfg.Storage.VersionLimit != 7 {
		t.Errorf("环境变量覆盖版本保留数 = %d, want 7", cfg.Storage.VersionLimit)
	}
}

func TestLoad_MissingFile(t *testing.T) {
	cfg, err := Load(filepath.Join(t.TempDir(), "nope.yaml"))
	if err != nil {
		t.Fatalf("缺失文件应回退默认值, err = %v", err)
	}
	if cfg.Server.Port != 8080 {
		t.Errorf("端口 = %d, want 8080", cfg.Server.Port)
	}
}

func TestNormalize_InvalidPort(t *testing.T) {
	cfg := Default()
	cfg.Server.Port = 99999
	if err := cfg.Normalize(); err == nil {
		t.Error("非法端口应报错")
	}
	cfg.Server.Port = 0
	cfg.Lua.TimeoutSeconds = -1
	if err := cfg.Normalize(); err == nil {
		t.Error("端口 0 应校验失败")
	}
	cfg.Server.Port = 8080
	if err := cfg.Normalize(); err != nil {
		t.Fatalf("合法配置应通过: %v", err)
	}
	if cfg.Lua.TimeoutSeconds != 5 {
		t.Errorf("超时应回退默认 5, got %d", cfg.Lua.TimeoutSeconds)
	}
}
