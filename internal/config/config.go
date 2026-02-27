package config

import (
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
)

type Config struct {
	App        AppConfig        `toml:"app"`
	Database   DatabaseConfig   `toml:"database"`
	Postmark   PostmarkConfig   `toml:"postmark"`
	Subscribe  SubscribeConfig  `toml:"subscribe"`
	Admin      AdminConfig      `toml:"admin"`
	Compliance ComplianceConfig `toml:"compliance"`
}

type ComplianceConfig struct {
	PhysicalAddress string `toml:"physical_address"`
}

type AdminConfig struct {
	Username     string `toml:"username"`
	PasswordHash string `toml:"password_hash"`
	SessionTTL   string `toml:"session_ttl"`
}

type AppConfig struct {
	ListenAddr  string   `toml:"listen_addr"`
	BaseURL     string   `toml:"base_url"`
	CORSOrigins []string `toml:"cors_origins"`
}

type DatabaseConfig struct {
	Path      string `toml:"path"`
	BackupDir string `toml:"backup_dir"`
}

type PostmarkConfig struct {
	ServerToken   string `toml:"server_token"`
	MessageStream string `toml:"message_stream"`
}

type SubscribeConfig struct {
	DefaultRedirect string `toml:"default_redirect"`
}

func Load(path string) (*Config, error) {
	cfg := &Config{
		App: AppConfig{
			ListenAddr: ":8090",
			BaseURL:    "http://localhost:8090",
		},
		Database: DatabaseConfig{
			Path: "broadwave.db",
		},
		Postmark: PostmarkConfig{
			MessageStream: "outbound",
		},
		Subscribe: SubscribeConfig{
			DefaultRedirect: "/",
		},
		Admin: AdminConfig{
			Username:   "admin",
			SessionTTL: "24h",
		},
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}

	if err := toml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	return cfg, nil
}
