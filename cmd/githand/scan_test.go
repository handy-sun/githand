package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/handy-sun/githand/internal/config"
	"github.com/handy-sun/githand/internal/testutil"
)

func initRepo(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	testutil.InitGitRepo(t, dir)
}

// runScan executes the scan command against the given config directory.
func runScan(t *testing.T, home string, args ...string) {
	t.Helper()
	oldCfgDir := cfgDir
	t.Cleanup(func() { cfgDir = oldCfgDir })
	cfgDir = home

	rootCmd.SetArgs(append([]string{"scan"}, args...))
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatal(err)
	}
}

func loadRegistry(t *testing.T, dir string) config.Registry {
	t.Helper()
	reg, err := config.LoadRegistry(dir)
	if err != nil {
		t.Fatal(err)
	}
	return reg
}

func TestScanMultipleRootsRegistersBasePaths(t *testing.T) {
	home := testutil.TempDir(t)
	rootA := testutil.TempDir(t)
	rootB := testutil.TempDir(t)
	initRepo(t, filepath.Join(rootA, "alpha"))
	initRepo(t, filepath.Join(rootB, "beta"))

	runScan(t, home, rootA, rootB)

	reg := loadRegistry(t, home)
	if reg.BasePath != rootA {
		t.Errorf("primary base path should be the first scanned root %s, got %s", rootA, reg.BasePath)
	}
	if len(reg.BasePaths) != 2 || reg.BasePaths[0] != rootA || reg.BasePaths[1] != rootB {
		t.Errorf("expected base paths [%s %s], got %v", rootA, rootB, reg.BasePaths)
	}
	if reg.FindRepo("alpha") == nil || reg.FindRepo("beta") == nil {
		t.Errorf("repos from both roots should be registered, got %v", reg.Repos)
	}
}

func TestScanSecondRootPreservesPrimaryBasePath(t *testing.T) {
	home := testutil.TempDir(t)
	rootA := testutil.TempDir(t)
	rootB := testutil.TempDir(t)
	initRepo(t, filepath.Join(rootA, "alpha"))
	initRepo(t, filepath.Join(rootB, "beta"))

	runScan(t, home, rootA)
	runScan(t, home, rootB)

	reg := loadRegistry(t, home)
	if reg.BasePath != rootA {
		t.Errorf("re-scanning another root must not move the primary base path, got %s", reg.BasePath)
	}
	if len(reg.BasePaths) != 2 {
		t.Errorf("expected 2 base paths, got %v", reg.BasePaths)
	}
}

func TestScanSameRootTwiceDoesNotDuplicateBasePath(t *testing.T) {
	home := testutil.TempDir(t)
	rootA := testutil.TempDir(t)
	initRepo(t, filepath.Join(rootA, "alpha"))

	runScan(t, home, rootA)
	runScan(t, home, rootA)

	reg := loadRegistry(t, home)
	if len(reg.BasePaths) != 1 {
		t.Errorf("re-scanning the same root should not duplicate the base path, got %v", reg.BasePaths)
	}
	if len(reg.Repos) != 1 {
		t.Errorf("re-scanning the same root should not duplicate repos, got %v", reg.Repos)
	}
}

func TestScanRejectsFileArgument(t *testing.T) {
	home := testutil.TempDir(t)
	file := filepath.Join(testutil.TempDir(t), "not-a-dir")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	oldCfgDir := cfgDir
	t.Cleanup(func() { cfgDir = oldCfgDir; rootCmd.SetArgs(nil) })
	cfgDir = home
	rootCmd.SetArgs([]string{"scan", file})

	if err := rootCmd.Execute(); err == nil {
		t.Fatal("scanning a file should fail")
	}
}
