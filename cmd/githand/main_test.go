package main

import (
	"testing"

	"github.com/handy-sun/githand/internal/i18n"
)

func TestStatusRemoteFlagIsBoolean(t *testing.T) {
	flag := statusCmd.Flags().Lookup("remote")
	if flag == nil {
		t.Fatal("status command should define --remote")
	}
	if flag.Value.Type() != "bool" {
		t.Fatalf("--remote type = %q, want bool", flag.Value.Type())
	}
	if flag.NoOptDefVal != "true" {
		t.Fatalf("--remote should not require a value, NoOptDefVal = %q", flag.NoOptDefVal)
	}
}

func TestApplyTranslationsTranslatesStatusRemoteAndSyncFlags(t *testing.T) {
	t.Cleanup(func() {
		i18n.SetLocale("en")
		applyTranslations(rootCmd)
	})

	for _, locale := range []string{"en", "zh"} {
		i18n.SetLocale(locale)
		applyTranslations(rootCmd)

		for _, name := range []string{"remote", "sync"} {
			flag := statusCmd.Flags().Lookup(name)
			want := i18n.T("status.flag." + name)
			if flag.Usage != want {
				t.Fatalf("locale %s: status --%s usage = %q, want %q", locale, name, flag.Usage, want)
			}
		}
	}
}

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
