package main

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/handy-sun/githand/internal/snapshot"
)

func TestSnapshotOutputFlagCreatesTimestampedDirUnderOutputParent(t *testing.T) {
	oldCfgDir := cfgDir
	oldSnapshotOutput := snapshotOutput
	oldSnapshotGroup := snapshotGroup
	oldSnapshotFilter := snapshotFilter
	t.Cleanup(func() {
		cfgDir = oldCfgDir
		snapshotOutput = oldSnapshotOutput
		snapshotGroup = oldSnapshotGroup
		snapshotFilter = oldSnapshotFilter
		rootCmd.SetArgs(nil)
	})

	cfgDir = t.TempDir()
	snapshotOutput = ""
	snapshotGroup = ""
	snapshotFilter = ""

	outputParent := filepath.Join(t.TempDir(), "snapshots")
	rootCmd.SetArgs([]string{"snapshot", "-o", outputParent})

	if err := rootCmd.Execute(); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(outputParent, snapshot.SnapshotJSONName)); !os.IsNotExist(err) {
		t.Fatalf("snapshot.json should not be written directly under output parent")
	}

	entries, err := os.ReadDir(outputParent)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected one snapshot directory under output parent, got %d entries", len(entries))
	}
	if !entries[0].IsDir() {
		t.Fatalf("expected snapshot output entry to be a directory, got %s", entries[0].Name())
	}
	if !regexp.MustCompile(`^githand-snapshot\.\d{4}-\d{6}$`).MatchString(entries[0].Name()) {
		t.Fatalf("expected fixed githand-snapshot.MMDD-HHmmss directory name, got %s", entries[0].Name())
	}
	if _, err := os.Stat(filepath.Join(outputParent, entries[0].Name(), snapshot.SnapshotJSONName)); err != nil {
		t.Fatalf("expected snapshot manifest inside timestamped directory: %v", err)
	}
}
