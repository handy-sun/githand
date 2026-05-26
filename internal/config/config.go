// Package config handles loading and saving githand configuration files.
//
// Two TOML files live under ~/.config/githand/ by default:
//   - githand.toml: user preferences (defaults, worker count, etc.)
//   - repos.toml:   the repo registry (base_path, repos, groups)
//
// Set GITHAND_HOME to use a different configuration directory.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"
)

const (
	EnvCache         = "XDG_CACHE_HOME"
	EnvHome          = "GITHAND_HOME"
	ConfigFileName   = "githand.toml"
	RegistryFileName = "repos.toml"
)

// DefaultConfigDir returns the githand config directory.
// Honors GITHAND_HOME; falls back to ~/.config/githand.
func DefaultConfigDir() (string, error) {
	if dir := os.Getenv(EnvHome); dir != "" {
		return dir, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(home, ".config", "githand"), nil
}

func defaultSnapshotOutputDir() string {
	if dir := os.Getenv(EnvCache); dir != "" {
		return filepath.Join(dir, "githand")
	}
	return "~/.cache/githand"
}

// ---------- githand.toml ----------

// Config represents the contents of githand.toml.
type Config struct {
	Version  int         `toml:"version"`
	Scan     ScanConfig  `toml:"scan"`
	Status   StatusConf  `toml:"status"`
	Snapshot SnapConfig  `toml:"snapshot"`
	Restore  RestoreConf `toml:"restore"`
}

type ScanConfig struct {
	Recursive bool `toml:"recursive"`
	AutoGroup bool `toml:"auto_group"`
}

type StatusConf struct {
	Workers  int  `toml:"workers"`
	JSON     bool `toml:"json"`
	AutoSync bool `toml:"auto_sync"`
}

type SnapConfig struct {
	OutputDir    string `toml:"output_dir"`
	IncludeClean bool   `toml:"include_clean"`
}

type RestoreConf struct {
	DryRun bool `toml:"dry_run"`
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		Version: 1,
		Scan: ScanConfig{
			Recursive: false,
			AutoGroup: true,
		},
		Status: StatusConf{
			Workers:  8,
			JSON:     false,
			AutoSync: true,
		},
		Snapshot: SnapConfig{
			OutputDir:    defaultSnapshotOutputDir(),
			IncludeClean: true,
		},
		Restore: RestoreConf{
			DryRun: false,
		},
	}
}

// LoadConfig reads githand.toml from the given directory.
// Returns DefaultConfig if the file does not exist.
func LoadConfig(dir string) (Config, error) {
	path := filepath.Join(dir, ConfigFileName)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return DefaultConfig(), nil
		}
		return Config{}, fmt.Errorf("read config: %w", err)
	}
	cfg := DefaultConfig()
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config: %w", err)
	}
	return cfg, nil
}

// SaveConfig writes cfg to githand.toml in the given directory.
func SaveConfig(dir string, cfg Config) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	data, err := toml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	path := filepath.Join(dir, ConfigFileName)
	return os.WriteFile(path, data, 0o644)
}

// ---------- repos.toml ----------

// Repo represents a single registered git repository.
type Repo struct {
	Name  string `toml:"name"`
	Path  string `toml:"path"`
	Group string `toml:"group,omitempty"`
}

// Registry represents the contents of repos.toml.
type Registry struct {
	Version  int                 `toml:"version"`
	BasePath string              `toml:"base_path"`
	Repos    []Repo              `toml:"repos"`
	Groups   map[string][]string `toml:"groups"`
}

// LoadRegistry reads repos.toml from the given directory.
func LoadRegistry(dir string) (Registry, error) {
	path := filepath.Join(dir, RegistryFileName)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Registry{}, nil
		}
		return Registry{}, fmt.Errorf("read registry: %w", err)
	}
	var reg Registry
	if err := toml.Unmarshal(data, &reg); err != nil {
		return Registry{}, fmt.Errorf("parse registry: %w", err)
	}
	return reg, nil
}

// SaveRegistry writes reg to repos.toml in the given directory.
func SaveRegistry(dir string, reg Registry) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	data, err := toml.Marshal(reg)
	if err != nil {
		return fmt.Errorf("marshal registry: %w", err)
	}
	path := filepath.Join(dir, RegistryFileName)
	return os.WriteFile(path, data, 0o644)
}

// FindRepo returns the repo with the given name, or nil.
func (r *Registry) FindRepo(name string) *Repo {
	for i := range r.Repos {
		if r.Repos[i].Name == name {
			return &r.Repos[i]
		}
	}
	return nil
}

// RemoveRepo removes the repo with the given name. Returns false if not found.
func (r *Registry) RemoveRepo(name string) bool {
	for i, repo := range r.Repos {
		if repo.Name == name {
			r.Repos = append(r.Repos[:i], r.Repos[i+1:]...)
			return true
		}
	}
	return false
}

// ReposInGroup returns all repos belonging to a group.
// A repo belongs if its Group field matches or its name appears in Groups[group].
func (r *Registry) ReposInGroup(group string) []Repo {
	seen := make(map[string]bool)
	var result []Repo

	// by explicit group field
	for i := range r.Repos {
		if r.Repos[i].Group == group {
			seen[r.Repos[i].Name] = true
			result = append(result, r.Repos[i])
		}
	}

	// by Groups map membership
	names := r.Groups[group]
	for _, name := range names {
		if !seen[name] {
			if repo := r.FindRepo(name); repo != nil {
				seen[name] = true
				result = append(result, *repo)
			}
		}
	}

	return result
}
