package agent

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCourseSessionsKeepsCourseBoundariesAndNewestFirst(t *testing.T) {
	root := t.TempDir()
	folder := filepath.Join(root, "Sessions")
	if err := os.MkdirAll(folder, 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(id, course, kind, section string) {
		body := "---\nid: " + id + "\nkind: " + kind + "\ntermination: completed\ncurriculum: \"" + course + "\"\n---\n\n## 起点\n\n章节：" + section + "  \n"
		if err := os.WriteFile(filepath.Join(folder, id+".md"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("session-20261014-090000.000000001", "ddia", "lesson", "1 复制")
	write("session-20261014-100000.000000002", "math", "lesson", "1 极限")
	write("session-20261014-110000.000000003", "ddia", "review", "复习")
	write("session-20261014-120000.000000004", "ddia", "lesson", "2 分区")

	s := NewServer(root)
	rows := s.CourseSessions("ddia", false, 20)
	if len(rows) != 2 || mstr(rows[0], "id") != "session-20261014-120000.000000004" || mstr(rows[0], "section") != "2 分区" {
		t.Fatalf("unexpected lesson history: %#v", rows)
	}
	withReviews := s.CourseSessions("ddia", true, 20)
	if len(withReviews) != 3 {
		t.Fatalf("review history was not included: %#v", withReviews)
	}
}

func TestFrontmatterValueAcceptsQuotedScalars(t *testing.T) {
	text := "---\ncurriculum: \"ddia-core\"\nkind: lesson\n---\n"
	if got := frontmatterValue(text, "curriculum"); got != "ddia-core" {
		t.Fatalf("curriculum = %q", got)
	}
}
