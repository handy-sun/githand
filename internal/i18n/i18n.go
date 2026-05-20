// Package i18n provides lightweight internationalization for githand.
//
// Locale is detected from the LANG environment variable (prefix before dot/dash),
// falling back to "en". Override with the --lang flag or GITHAND_LANG env var.
//
// Usage:
//
//	i18n.T("scan.short")           → translated string
//	i18n.Tf("scan.result", path, n) → translated string with fmt.Sprintf
package i18n

import (
	"fmt"
	"os"
	"strings"
	"sync"
)

// current holds the active locale code ("en", "zh", etc.).
var current string

// once ensures locale detection runs only once.
var once sync.Once

// Locale returns the active locale code.
func Locale() string {
	once.Do(detectLocale)
	return current
}

// SetLocale forces the active locale. Call this before any T/Tf calls
// (e.g. from the --lang flag in cobra's PersistentPreRun).
func SetLocale(lang string) {
	once.Do(func() {}) // mark as done so detectLocale won't override
	lang = strings.ToLower(strings.TrimSpace(lang))
	if _, ok := catalogs[lang]; ok {
		current = lang
	} else {
		current = "en"
	}
}

// detectLocale picks a locale from env vars, preferring GITHAND_LANG over LANG.
// Called inside sync.Once — must not call SetLocale (which also calls sync.Once).
func detectLocale() {
	if v := os.Getenv("GITHAND_LANG"); v != "" {
		v = strings.ToLower(strings.TrimSpace(v))
		if _, ok := catalogs[v]; ok {
			current = v
		} else {
			current = "en"
		}
		return
	}
	lang := os.Getenv("LANG")
	if lang == "" {
		current = "en"
		return
	}
	// LANG is typically like "zh_CN.UTF-8" or "en_US"
	code := strings.ToLower(strings.SplitN(lang, ".", 2)[0])
	code = strings.SplitN(code, "_", 2)[0]
	if _, ok := catalogs[code]; ok {
		current = code
	} else {
		current = "en"
	}
}

// T returns the translated string for the given key.
// Falls back to English if the key is missing in the active locale.
// Falls back to the key itself if not found in any catalog.
func T(key string) string {
	once.Do(detectLocale)
	if cat, ok := catalogs[current]; ok {
		if s, ok := cat[key]; ok {
			return s
		}
	}
	// Fallback to English
	if cat, ok := catalogs["en"]; ok {
		if s, ok := cat[key]; ok {
			return s
		}
	}
	return key
}

// Tf returns the translated string with fmt.Sprintf applied.
func Tf(key string, args ...any) string {
	return fmt.Sprintf(T(key), args...)
}

// Keys returns all known translation keys (from the English catalog).
func Keys() []string {
	cat := catalogs["en"]
	keys := make([]string, 0, len(cat))
	for k := range cat {
		keys = append(keys, k)
	}
	return keys
}
