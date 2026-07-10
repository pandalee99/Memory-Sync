package memorysync

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

var ErrConfigNotFound = errors.New("config not found")

// Config holds the parsed .memory-sync.toml.
type Config struct {
	ProjectID string      `toml:"project_id"`
	Store     StoreConfig `toml:"store"`
}

// StoreConfig holds the [store] table of .memory-sync.toml.
type StoreConfig struct {
	Backend string `toml:"backend"`
	URL     string `toml:"url"`
}

// LoadConfig parses a .memory-sync.toml file. Named LoadConfig (not Load) to
// avoid a name conflict with manifest.go's Load.
func LoadConfig(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config %s: %w", path, err)
	}
	var cfg Config
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config %s: %w", path, err)
	}
	return cfg, nil
}

// FindConfig resolves the config path by priority:
// flag > MSYNC_CONFIG env > ~/.memory-sync.toml > ./.memory-sync.toml.
func FindConfig(flagPath string) (string, error) {
	candidates := []string{flagPath}
	if env := os.Getenv("MSYNC_CONFIG"); env != "" {
		candidates = append(candidates, env)
	}
	if home, err := os.UserHomeDir(); err == nil {
		candidates = append(candidates, filepath.Join(home, ".memory-sync.toml"))
	}
	candidates = append(candidates, ".memory-sync.toml")
	for _, c := range candidates {
		if c == "" {
			continue
		}
		if _, err := os.Stat(c); err == nil {
			return c, nil
		}
	}
	return "", fmt.Errorf("%w: no .memory-sync.toml found (tried: %v). Run `memory-sync install` to create one, or use --config <path>", ErrConfigNotFound, candidates)
}

// SaveConfig writes cfg to path as TOML (plaintext schema: project_id + [store]).
func SaveConfig(path string, cfg Config) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create config %s: %w", path, err)
	}
	defer f.Close()
	if err := toml.NewEncoder(f).Encode(cfg); err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	return nil
}
