package mdblock_test

import (
	"strings"
	"testing"

	"github.com/hedykan/learning-system/internal/mdblock"
)

func TestUpsertAndPreserve(t *testing.T) {
	text := mdblock.Upsert("", "analysis", "v1", "# S\n")
	text = strings.Replace(text, "<!-- learn:user:begin -->\n", "<!-- learn:user:begin -->\n我自己的笔记\n", 1)
	text = mdblock.Upsert(text, "analysis", "v2", "# S\n")
	if !strings.Contains(text, "v2") || strings.Contains(text, "v1") {
		t.Fatalf("block not replaced:\n%s", text)
	}
	if got := mdblock.PreservedUser(text, true); got != "我自己的笔记" {
		t.Fatalf("user block = %q", got)
	}
	if got := mdblock.PreservedUser("旧的手写文件", false); got != "旧的手写文件" {
		t.Fatalf("unowned file content lost: %q", got)
	}
	if got := mdblock.PreservedUser("runtime text", true); got != "" {
		t.Fatalf("owned file content kept: %q", got)
	}
	legacy := mdblock.Upsert("# Legacy session\n\nbody", "analysis", "new", "")
	if !strings.HasPrefix(legacy, "# Legacy session\n\nbody\n") || !strings.Contains(legacy, "new") {
		t.Fatalf("append failed:\n%s", legacy)
	}
}
