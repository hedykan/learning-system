// Package i18n holds the fixed text of generated pages (CR-2026-023). Chinese
// source strings are the keys; each other language is one lookup table, so
// adding a language means adding one table. Command output stays English.
package i18n

import "fmt"

const Default = "zh"

// Languages lists the supported interface languages.
var Languages = []string{"zh", "en"}

var tables = map[string]map[string]string{"en": en}

// Valid reports whether lang is a supported interface language.
func Valid(lang string) bool {
	for _, l := range Languages {
		if l == lang {
			return true
		}
	}
	return false
}

// Normalize maps an empty or unknown language to a supported one: empty is
// Chinese (the pre-v0.1.7 behavior), anything else unknown falls back to English.
func Normalize(lang string) string {
	switch {
	case lang == "":
		return Default
	case Valid(lang):
		return lang
	default:
		return "en"
	}
}

// T translates a Chinese source string; untranslated strings stay Chinese.
func T(lang, s string) string {
	if t, ok := tables[Normalize(lang)][s]; ok {
		return t
	}
	return s
}

// F translates a format string, then formats it.
func F(lang, format string, args ...any) string { return fmt.Sprintf(T(lang, format), args...) }

// Keys lists the source strings a language table translates (for tests).
func Keys(lang string) map[string]string { return tables[lang] }
