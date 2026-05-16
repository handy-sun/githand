package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfigDir(t *testing.T) {
	// with XDG set
	t.Setenv("XDG_CONFIG_HOME", "/tmp/xdg-test")
	dir, err := DefaultConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	if dir != "/tmp/xdg-test/githand" {
		t.Fatalf("expected /tmp/xdg-test/githand, got %s", dir)
	}

	// without XDG — falls back to ~/.config
	t.Setenv("XDG_CONFIG_HOME", "")
	dir, err = DefaultConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	home, _ := os.UserHomeDir()
	expected := filepath.Join(home, ".config", "githand")
	if dir != expected {
		t.Fatalf("expected %s, got %s", expected, dir)
	}
}

func TestConfigRoundTrip(t *testing.T) {
	dir, err := os.MkdirTemp("", "githand-config-test-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	cfg := DefaultConfig()
	cfg.Scan.Recursive = false
	cfg.Status.Workers = 4

	if err := SaveConfig(dir, cfg); err != nil {
		t.Fatal(err)
	}

	loaded, err := LoadConfig(dir)
	if err != nil {
		t.Fatal(err)
	}

	if loaded.Scan.Recursive != false {
		t.Error("Scan.Recursive should be false")
	}
	if loaded.Status.Workers != 4 {
		t.Errorf("Status.Workers should be 4, got %d", loaded.Status.Workers)
	}
	if loaded.Version != 1 {
		t.Errorf("Version should be 1, got %d", loaded.Version)
	}
}

func TestLoadConfigMissing(t *testing.T) {
	dir, err := os.MkdirTemp("", "githand-config-test-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	cfg, err := LoadConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	def := DefaultConfig()
	if cfg != def {
		t.Error("missing config should return defaults")
	}
}

func TestRegistryRoundTrip(t *testing.T) {
	dir, err := os.MkdirTemp("", "githand-registry-test-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	reg := Registry{
		Version:  1,
		BasePath: "/Users/qi/work",
		Repos: []Repo{
			{Name: "githand", Path: "/Users/qi/work/githand", Group: "agent-switch"},
			{Name: "expnix", Path: "/Users/qi/work/nix/expnix", Group: "nix"},
		},
		Groups: map[string][]string{
			"agent-switch": {"githand"},
			"nix":          {"expnix"},
		},
	}

	if err := SaveRegistry(dir, reg); err != nil {
		t.Fatal(err)
	}

	loaded, err := LoadRegistry(dir)
	if err != nil {
		t.Fatal(err)
	}

	if loaded.BasePath != reg.BasePath {
		t.Errorf("BasePath: expected %s, got %s", reg.BasePath, loaded.BasePath)
	}
	if len(loaded.Repos) != 2 {
		t.Fatalf("expected 2 repos, got %d", len(loaded.Repos))
	}
	if loaded.Repos[0].Name != "githand" {
		t.Errorf("first repo name: expected githand, got %s", loaded.Repos[0].Name)
	}
	if loaded.Repos[0].Group != "agent-switch" {
		t.Errorf("first repo group: expected agent-switch, got %s", loaded.Repos[0].Group)
	}
	if len(loaded.Groups["nix"]) != 1 || loaded.Groups["nix"][0] != "expnix" {
		t.Errorf("nix group: expected [expnix], got %v", loaded.Groups["nix"])
	}
}

func TestLoadRegistryMissing(t *testing.T) {
	dir, err := os.MkdirTemp("", "githand-registry-test-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	reg, err := LoadRegistry(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(reg.Repos) != 0 {
		t.Error("missing registry should have no repos")
	}
}

func TestRegistryFindRepo(t *testing.T) {
	reg := Registry{
		Repos: []Repo{
			{Name: "alpha", Path: "/a"},
			{Name: "beta", Path: "/b"},
		},
	}
	if r := reg.FindRepo("alpha"); r == nil || r.Path != "/a" {
		t.Error("FindRepo alpha failed")
	}
	if r := reg.FindRepo("missing"); r != nil {
		t.Error("FindRepo missing should return nil")
	}
}

func TestRegistryRemoveRepo(t *testing.T) {
	reg := Registry{
		Repos: []Repo{
			{Name: "alpha", Path: "/a"},
			{Name: "beta", Path: "/b"},
		},
	}
	if !reg.RemoveRepo("alpha") {
		t.Error("RemoveRepo alpha should succeed")
	}
	if len(reg.Repos) != 1 || reg.Repos[0].Name != "beta" {
		t.Error("after removing alpha, only beta should remain")
	}
	if reg.RemoveRepo("missing") {
		t.Error("RemoveRepo missing should return false")
	}
}

func TestRegistryReposInGroup(t *testing.T) {
	reg := Registry{
		Repos: []Repo{
			{Name: "a", Path: "/a", Group: "x"},
			{Name: "b", Path: "/b", Group: "y"},
			{Name: "c", Path: "/c", Group: "x"},
			{Name: "d", Path: "/d"},
		},
		Groups: map[string][]string{
			"x": {"e"}, // e is in groups map but not in repos
		},
	}

	inX := reg.ReposInGroup("x")
	if len(inX) != 2 {
		t.Fatalf("group x should have 2 repos, got %d", len(inX))
	}
	names := map[string]bool{}
	for _, r := range inX {
		names[r.Name] = true
	}
	if !names["a"] || !names["c"] {
		t.Errorf("group x should contain a and c, got %v", names)
	}
}
