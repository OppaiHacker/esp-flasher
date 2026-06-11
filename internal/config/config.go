// Package config handles persistent application settings (config.json in tool directory).
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"

	"espflasher/internal/paths"
)

// Config represents the persisted settings.
type Config struct {
	Lang     string `json:"lang"`      // "en" / "pl"
	LastPort string `json:"last_port"` // last selected port
	Baud     int    `json:"baud"`      // monitor baud rate
}

// Default returns the default configuration.
func Default() Config {
	return Config{Lang: "en", Baud: 115200}
}

// Path returns the configuration file path.
func Path() string {
	return filepath.Join(paths.BaseDir(), "config.json")
}

// Load reads the configuration; returns default on error or missing file.
func Load() Config {
	cfg := Default()
	data, err := os.ReadFile(Path())
	if err != nil {
		return cfg
	}
	_ = json.Unmarshal(data, &cfg)
	if cfg.Baud <= 0 {
		cfg.Baud = 115200
	}
	if cfg.Lang != "pl" {
		cfg.Lang = "en"
	}
	return cfg
}

// Save writes the configuration to disk. Under sudo, it chowns the file
// to the session owner (SUDO_UID/SUDO_GID) so normal runs can overwrite it.
func (c Config) Save() error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(Path(), append(data, '\n'), 0o644); err != nil {
		return err
	}
	if uid, err1 := strconv.Atoi(os.Getenv("SUDO_UID")); err1 == nil {
		if gid, err2 := strconv.Atoi(os.Getenv("SUDO_GID")); err2 == nil {
			_ = os.Chown(Path(), uid, gid)
		}
	}
	return nil
}
