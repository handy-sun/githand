package main

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/handy-sun/githand/internal/snapshot"
)

func TestSnapshotOutputFlagCreatesTimestampedJSONFileWhenNoPayloadFiles(t *testing.T) {
	oldCfgDir := cfgDir
	oldSnapshotOutput := snapshotOutput
	oldSnapshotGroup := snapshotGroup
	oldSnapshotFilter := snapshotFilter
	oldSnapshotArchive := snapshotArchive
	t.Cleanup(func() {
		cfgDir = oldCfgDir
		snapshotOutput = oldSnapshotOutput
		snapshotGroup = oldSnapshotGroup
		snapshotFilter = oldSnapshotFilter
		snapshotArchive = oldSnapshotArchive
		rootCmd.SetArgs(nil)
	})

	cfgDir = t.TempDir()
	snapshotOutput = ""
	snapshotGroup = ""
	snapshotFilter = ""
	snapshotArchive = false

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
		t.Fatalf("expected one snapshot JSON file under output parent, got %d entries", len(entries))
	}
	if entries[0].IsDir() {
		t.Fatalf("expected JSON-only snapshot output entry to be a file, got directory %s", entries[0].Name())
	}
	if !regexp.MustCompile(`^githand-snapshot\.\d{4}-\d{6}\.json$`).MatchString(entries[0].Name()) {
		t.Fatalf("expected fixed githand-snapshot.MMDD-HHmmss.json file name, got %s", entries[0].Name())
	}
	if _, err := snapshot.ResolveSnapshotPath(filepath.Join(outputParent, entries[0].Name())); err != nil {
		t.Fatalf("expected direct snapshot JSON to resolve: %v", err)
	}
}
