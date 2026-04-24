package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/joho/godotenv"
)

func TestLoad_EnvVars(t *testing.T) {
	// 设置环境变量
	os.Setenv("MEMBROWSER_AUTH_KEY", "test-auth-key")
	os.Setenv("MEMBROWSER_HAIKU_API_KEY", "test-haiku-key")
	os.Setenv("MEMBROWSER_SONNET_API_KEY", "test-sonnet-key")
	os.Setenv("MEMBROWSER_OPUS_API_KEY", "test-opus-key")
	defer func() {
		os.Unsetenv("MEMBROWSER_AUTH_KEY")
		os.Unsetenv("MEMBROWSER_HAIKU_API_KEY")
		os.Unsetenv("MEMBROWSER_SONNET_API_KEY")
		os.Unsetenv("MEMBROWSER_OPUS_API_KEY")
	}()

	cfg := Load()

	if cfg.AuthKey != "test-auth-key" {
		t.Errorf("AuthKey = %q, want %q", cfg.AuthKey, "test-auth-key")
	}
	if cfg.Haiku.APIKey != "test-haiku-key" {
		t.Errorf("Haiku.APIKey = %q, want %q", cfg.Haiku.APIKey, "test-haiku-key")
	}
	if cfg.Sonnet.APIKey != "test-sonnet-key" {
		t.Errorf("Sonnet.APIKey = %q, want %q", cfg.Sonnet.APIKey, "test-sonnet-key")
	}
	if cfg.Opus.APIKey != "test-opus-key" {
		t.Errorf("Opus.APIKey = %q, want %q", cfg.Opus.APIKey, "test-opus-key")
	}
}

func TestLoad_Defaults(t *testing.T) {
	// 清除所有环境变量
	os.Unsetenv("MEMBROWSER_AUTH_KEY")
	os.Unsetenv("MEMBROWSER_PORT")
	os.Unsetenv("MEMBROWSER_DB_PATH")

	cfg := Load()

	if cfg.Port != 8080 {
		t.Errorf("Port = %d, want %d", cfg.Port, 8080)
	}
	if cfg.DBPath != "./data/membrowser.db" {
		t.Errorf("DBPath = %q, want %q", cfg.DBPath, "./data/membrowser.db")
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("LogLevel = %q, want %q", cfg.LogLevel, "debug")
	}
}

func TestLoad_DotEnvFile(t *testing.T) {
	// 清除环境变量
	os.Unsetenv("MEMBROWSER_AUTH_KEY")
	os.Unsetenv("MEMBROWSER_HAIKU_API_KEY")
	os.Unsetenv("MEMBROWSER_SONNET_API_KEY")
	os.Unsetenv("MEMBROWSER_OPUS_API_KEY")

	// 创建临时 .env 文件
	tmpDir := t.TempDir()
	envFile := filepath.Join(tmpDir, ".env")
	content := "MEMBROWSER_AUTH_KEY=dotenv-auth-key\n" +
		"MEMBROWSER_HAIKU_API_KEY=dotenv-haiku-key\n" +
		"MEMBROWSER_SONNET_API_KEY=dotenv-sonnet-key\n" +
		"MEMBROWSER_OPUS_API_KEY=dotenv-opus-key\n"
	if err := os.WriteFile(envFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// 用 godotenv 加载临时 .env
	if err := godotenv.Load(envFile); err != nil {
		t.Fatal("godotenv.Load failed:", err)
	}

	cfg := Load()

	if cfg.AuthKey != "dotenv-auth-key" {
		t.Errorf("AuthKey = %q, want %q", cfg.AuthKey, "dotenv-auth-key")
	}
	if cfg.Haiku.APIKey != "dotenv-haiku-key" {
		t.Errorf("Haiku.APIKey = %q, want %q", cfg.Haiku.APIKey, "dotenv-haiku-key")
	}
	if cfg.Sonnet.APIKey != "dotenv-sonnet-key" {
		t.Errorf("Sonnet.APIKey = %q, want %q", cfg.Sonnet.APIKey, "dotenv-sonnet-key")
	}
	if cfg.Opus.APIKey != "dotenv-opus-key" {
		t.Errorf("Opus.APIKey = %q, want %q", cfg.Opus.APIKey, "dotenv-opus-key")
	}
}
