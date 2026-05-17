package i18n

import (
	"os"
	"sync"
	"testing"
)

func TestDetectLocaleFromEnv(t *testing.T) {
	tests := []struct {
		env  string
		want string
	}{
		{"en_US.UTF-8", "en"},
		{"zh_CN.UTF-8", "zh"},
		{"ja_JP.UTF-8", "en"}, // unsupported falls back to en
		{"C", "en"},
		{"", "en"},
	}

	for _, tt := range tests {
		t.Run(tt.env, func(t *testing.T) {
			os.Setenv("GITHAND_LANG", "")
			os.Setenv("LANG", tt.env)
			once = sync.Once{}
			current = "en"
			if got := Locale(); got != tt.want {
				t.Errorf("Locale() with LANG=%s = %q, want %q", tt.env, got, tt.want)
			}
		})
	}
}

func TestDetectLocaleFromGithandLang(t *testing.T) {
	tests := []struct {
		env  string
		want string
	}{
		{"zh", "zh"},
		{"en", "en"},
		{"EN", "en"}, // case-insensitive
		{"fr", "en"}, // unsupported falls back to en
	}

	for _, tt := range tests {
		t.Run(tt.env, func(t *testing.T) {
			os.Setenv("GITHAND_LANG", tt.env)
			os.Setenv("LANG", "")
			once = sync.Once{}
			current = "en"
			if got := Locale(); got != tt.want {
				t.Errorf("Locale() with GITHAND_LANG=%s = %q, want %q", tt.env, got, tt.want)
			}
		})
	}
	os.Unsetenv("GITHAND_LANG")
}

func TestSetLocale(t *testing.T) {
	once = sync.Once{}
	current = "en"

	SetLocale("zh")
	if current != "zh" {
		t.Errorf("SetLocale(zh) = %q, want zh", current)
	}

	SetLocale("en")
	if current != "en" {
		t.Errorf("SetLocale(en) = %q, want en", current)
	}

	SetLocale("unsupported")
	if current != "en" {
		t.Errorf("SetLocale(unsupported) = %q, want en (fallback)", current)
	}
}

func TestT(t *testing.T) {
	once = sync.Once{}
	current = "en"

	// English
	if got := T("root.short"); got != "Git workspace sync and migration CLI" {
		t.Errorf("T(root.short) en = %q, want English", got)
	}

	// Switch to Chinese
	SetLocale("zh")
	if got := T("root.short"); got != "Git 工作区同步与迁移工具" {
		t.Errorf("T(root.short) zh = %q, want Chinese", got)
	}

	// Unknown key falls back to key itself
	if got := T("nonexistent.key"); got != "nonexistent.key" {
		t.Errorf("T(nonexistent.key) = %q, want key itself", got)
	}
}

func TestTf(t *testing.T) {
	once = sync.Once{}
	current = "en"

	SetLocale("zh")
	got := Tf("scan.result", "/Users/qi/work", 35, 3)
	want := "已扫描 /Users/qi/work：发现 35 个仓库，新增 3 个。"
	if got != want {
		t.Errorf("Tf(scan.result) zh = %q, want %q", got, want)
	}
}