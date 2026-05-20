package main

import "testing"

func TestNormalizeBuildInfoStripsDescribeHashFromVersion(t *testing.T) {
	displayVersion, displayCommit := normalizeBuildInfo("v0.1.0-5-g8da94f0", "8da94f0")

	if displayVersion != "v0.1.0-5" {
		t.Fatalf("expected version v0.1.0-5, got %s", displayVersion)
	}
	if displayCommit != "8da94f0" {
		t.Fatalf("expected commit 8da94f0, got %s", displayCommit)
	}
}

func TestNormalizeBuildInfoMovesDirtySuffixToCommit(t *testing.T) {
	displayVersion, displayCommit := normalizeBuildInfo("v0.1.0-5-g8da94f0-dirty", "8da94f0")

	if displayVersion != "v0.1.0-5" {
		t.Fatalf("expected version v0.1.0-5, got %s", displayVersion)
	}
	if displayCommit != "8da94f0-dirty" {
		t.Fatalf("expected commit 8da94f0-dirty, got %s", displayCommit)
	}
}

func TestVersionTemplateUsesNormalizedBuildInfo(t *testing.T) {
	got := versionTemplate("v0.1.0-5-g8da94f0-dirty", "8da94f0", "2026-05-20T12:23:20+0800")
	want := "githand v0.1.0-5 (commit: 8da94f0-dirty, built: 2026-05-20T12:23:20+0800)\n"

	if got != want {
		t.Fatalf("version template mismatch\nwant: %q\n got: %q", want, got)
	}
}
