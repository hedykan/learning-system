package conversation_test

import (
	"strings"
	"testing"
	"time"

	"github.com/hedykan/learning-system/internal/conversation"
)

const legacy = "---\nid: session-1\nkind: lesson\ncurriculum: \"ddia\"\n---\n\n# Raw Conversation\n\n## 2026-09-24T09:00:00Z — assistant\n\n> 你怎么看？\n\n## 2026-09-24T09:01:00Z — user\n\n> 第一行\n> \n> 第二行\n"

func TestParseLegacyAssignsOrdinalIDs(t *testing.T) {
	conv, err := conversation.Parse(legacy)
	if err != nil {
		t.Fatal(err)
	}
	if conv.SessionID != "session-1" || conv.Curriculum != "ddia" || conv.Kind != "lesson" {
		t.Fatalf("frontmatter = %+v", conv)
	}
	if len(conv.Turns) != 2 || conv.Turns[1].ID != "t0002" || conv.Turns[1].Role != "user" || conv.Turns[1].Anchored {
		t.Fatalf("turns = %+v", conv.Turns)
	}
	if conv.Turns[1].Text != "第一行\n\n第二行" {
		t.Fatalf("text = %q", conv.Turns[1].Text)
	}
}

func TestFormatEntryRoundTripsWithAnchor(t *testing.T) {
	text := legacy + conversation.FormatEntry(3, "user", "不能只看平均值\n还要看 p99", time.Date(2026, 9, 24, 9, 2, 0, 0, time.UTC))
	conv, err := conversation.Parse(text)
	if err != nil {
		t.Fatal(err)
	}
	turn, ok := conv.Turn("t0003")
	if !ok || !turn.Anchored || turn.Role != "user" || turn.Text != "不能只看平均值\n还要看 p99" {
		t.Fatalf("turn = %+v", turn)
	}
	if !strings.Contains(conv.RoleText("user"), "还要看 p99") {
		t.Fatal("role text missing new turn")
	}
}

func TestParseRejectsMismatchedIDs(t *testing.T) {
	bad := legacy + "\n## t0009 — 2026-09-24T09:02:00Z — user\n\n> x\n"
	if _, err := conversation.Parse(bad); err == nil {
		t.Fatal("expected mismatch error")
	}
	badAnchor := legacy + "\n## t0003 — 2026-09-24T09:02:00Z — user\n\n> x\n\n^t0004\n"
	if _, err := conversation.Parse(badAnchor); err == nil {
		t.Fatal("expected anchor mismatch error")
	}
}

func TestQuoteMatches(t *testing.T) {
	if err := conversation.QuoteMatches("不能只看  平均值，还要看 p99", "不能只看 平均值"); err != nil {
		t.Fatal(err)
	}
	if err := conversation.QuoteMatches("abc", "ab"); err == nil {
		t.Fatal("short quote accepted")
	}
	if err := conversation.QuoteMatches("不能只看平均值", "必须看平均值"); err == nil {
		t.Fatal("foreign quote accepted")
	}
}

func TestParseCRLF(t *testing.T) {
	crlf := strings.ReplaceAll(legacy, "\n", "\r\n")
	conv, err := conversation.Parse(crlf)
	if err != nil || len(conv.Turns) != 2 || conv.Turns[1].Text != "第一行\n\n第二行" || conv.Curriculum != "ddia" {
		t.Fatalf("crlf parse: %+v err=%v", conv, err)
	}
}
