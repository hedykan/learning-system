package i18n

import (
	"regexp"
	"strings"
	"testing"
)

var verb = regexp.MustCompile(`%[-+# 0]*\d*(?:\.\d+)?[a-zA-Z%]`)

func TestTranslationsKeepFormatVerbsAndLayout(t *testing.T) {
	for _, lang := range Languages {
		for src, dst := range Keys(lang) {
			if a, b := verb.FindAllString(src, -1), verb.FindAllString(dst, -1); strings.Join(a, " ") != strings.Join(b, " ") {
				t.Errorf("%s: %q has verbs %v, translation %q has %v", lang, src, a, dst, b)
			}
			if strings.Count(src, "\n") != strings.Count(dst, "\n") {
				t.Errorf("%s: %q and %q differ in line breaks", lang, src, dst)
			}
		}
	}
}

func TestFallbacks(t *testing.T) {
	if T("", "学习进度") != "学习进度" || T("zh", "学习进度") != "学习进度" {
		t.Fatal("Chinese must be the identity")
	}
	if T("en", "学习进度") != "Progress" || T("fr", "学习进度") != "Progress" {
		t.Fatal("English and unknown languages must use the English table")
	}
	if T("en", "未收录") != "未收录" {
		t.Fatal("untranslated text must be returned unchanged")
	}
}
