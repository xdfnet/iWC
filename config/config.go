package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// Config 顶层配置
type Config struct {
	WeChat WeChatConfig `toml:"wechat"`
	Claude ClaudeConfig `toml:"claude"`
	System SystemConfig `toml:"system"`
}

// WeChatConfig 微信 ilink 配置
type WeChatConfig struct {
	Token      string `toml:"token"`
	BaseURL    string `toml:"base_url"`
	AllowFrom  string `toml:"allow_from"`
	LongPollMS int    `toml:"long_poll_timeout_ms"`
}

// ClaudeConfig Claude Code 配置
type ClaudeConfig struct {
	CLIPath string `toml:"cli_path"`
	WorkDir string `toml:"work_dir"`
}

// SystemConfig 系统配置
type SystemConfig struct {
	DataDir string `toml:"data_dir"`
}

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	home, _ := os.UserHomeDir()
	return &Config{
		WeChat: WeChatConfig{
			BaseURL:    "https://ilinkai.weixin.qq.com",
			LongPollMS: 35000,
		},
		Claude: ClaudeConfig{
			CLIPath: "claude",
			WorkDir: home,
		},
		System: SystemConfig{
			DataDir: filepath.Join(home, ".config", "iwc"),
		},
	}
}

// ConfigPath 返回配置文件路径
func ConfigPath() string {
	if p := os.Getenv("IWC_CONFIG"); p != "" {
		return p
	}
	// Backward compatibility for previous env var name.
	if p := os.Getenv("ICC_CONFIG"); p != "" {
		return p
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "iwc", "config.toml")
}

// Load 加载配置文件
func Load(path string) (*Config, error) {
	cfg := DefaultConfig()

	if path == "" {
		path = ConfigPath()
	}

	// 迁移旧配置到新位置
	if _, err := os.Stat(path); os.IsNotExist(err) {
		oldPath := filepath.Join(os.Getenv("HOME"), ".iwc", "config.toml")
		if _, err := os.Stat(oldPath); err == nil {
			fmt.Fprintf(os.Stderr, "ℹ️ 检测到旧配置，正在迁移到 %s\n", path)
			if data, err := os.ReadFile(oldPath); err == nil {
				os.MkdirAll(filepath.Dir(path), 0755)
				os.WriteFile(path, data, 0644)
			}
		}
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return cfg, fmt.Errorf("配置文件不存在: %s\n请先运行 `iwc setup`", path)
	}

	md, err := toml.DecodeFile(path, cfg)
	if err != nil {
		return nil, fmt.Errorf("解析配置失败: %w", err)
	}
	if len(md.Undecoded()) > 0 {
		fmt.Fprintf(os.Stderr, "⚠️ 配置中存在未知字段: %v\n", md.Undecoded())
	}

	// 确保数据目录存在
	if cfg.System.DataDir != "" {
		os.MkdirAll(cfg.System.DataDir, 0755)
	}

	return cfg, nil
}

// Save 保存配置到文件（原子写入）
func Save(cfg *Config, path string) error {
	if path == "" {
		path = ConfigPath()
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建配置目录失败: %w", err)
	}
	// 先写临时文件，再 rename 保证原子性
	tmp := path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return fmt.Errorf("创建配置文件失败: %w", err)
	}
	err = toml.NewEncoder(f).Encode(cfg)
	f.Close()
	if err != nil {
		os.Remove(tmp)
		return fmt.Errorf("写入配置失败: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("保存配置失败: %w", err)
	}
	return nil
}
