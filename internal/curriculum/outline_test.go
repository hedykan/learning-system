package curriculum_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hedykan/learning-system/internal/curriculum"
	"github.com/hedykan/learning-system/internal/vault"
)

const ddiaOutline = `nodes:
  - {id: "1", title: 数据系统架构中的权衡, pages: [10, 29]}
  - {id: "1.1", title: 分析型与事务型系统, pages: [11, 16]}
  - {id: "1.2", title: 云服务与自托管, pages: [16, 20]}
  - {id: "2", title: 定义非功能性需求, pages: [30, 60]}
  - {id: "2.1", title: 案例研究：社交网络首页时间线, pages: [31, 34]}
  - {id: "2.2", title: 描述性能, pages: [34, 38]}
  - {id: "2.3", title: 可伸缩性, pages: [45, 50]}
  - {id: "3", title: 数据模型与查询语言, pages: [61, 100]}
`

func importBook(t *testing.T, content, name string) (string, time.Time) {
	t.Helper()
	root := filepath.Join(t.TempDir(), "vault")
	now := time.Date(2026, 9, 26, 10, 0, 0, 0, time.UTC)
	if _, err := vault.Init(root, now); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(src, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := curriculum.Import(root, curriculum.ImportOptions{SourcePath: src, ID: "book", Title: "Book", Activate: true, Confirmed: true, Now: now}); err != nil {
		t.Fatal(err)
	}
	return root, now
}

func TestMarkdownImportCreatesDraftOutline(t *testing.T) {
	root, _ := importBook(t, "# 极限\n\n## 数列极限\n\n```\n# not a heading\n```\n\n## 函数极限\n\n# 导数\n", "calc.md")
	o, err := curriculum.LoadOutline(root, "book")
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, n := range o.Nodes {
		ids = append(ids, n.ID+":"+n.Title)
	}
	if o.Status != "draft" || strings.Join(ids, ",") != "1:极限,1.1:数列极限,1.2:函数极限,2:导数" {
		t.Fatalf("outline = %s %v", o.Status, ids)
	}
}

func TestOutlineValidationRejectsBadInput(t *testing.T) {
	root, _ := importBook(t, "plain text", "book.txt")
	if o, _ := curriculum.LoadOutline(root, "book"); o.Status != "missing" {
		t.Fatalf("text import status = %s", o.Status)
	}
	bad := map[string]string{
		"duplicate":     "nodes: [{id: '1', title: a}, {id: '1', title: b}]",
		"orphan":        "nodes: [{id: '1', title: a}, {id: '2.1', title: b}]",
		"order":         "nodes: [{id: '2', title: a}, {id: '1', title: b}]",
		"pages":         "nodes: [{id: '1', title: a, pages: [9, 3]}]",
		"page order":    "nodes: [{id: '1', title: a, pages: [10, 20]}, {id: '2', title: b, pages: [5, 8]}]",
		"out of parent": "nodes: [{id: '1', title: a, pages: [10, 20]}, {id: '1.1', title: b, pages: [15, 25]}]",
		"bad id":        "nodes: [{id: 'I', title: a}]",
		"empty title":   "nodes: [{id: '1', title: ''}]",
	}
	for name, input := range bad {
		if _, err := curriculum.SetOutline(root, "book", []byte(input), false); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
	if o, _ := curriculum.LoadOutline(root, "book"); o.Status != "missing" {
		t.Fatal("rejected outline was written")
	}
	if _, err := curriculum.SetOutline(root, "book", []byte(ddiaOutline), true); err != nil {
		t.Fatal(err)
	}
	if o, _ := curriculum.LoadOutline(root, "book"); o.Status != "missing" {
		t.Fatal("dry run wrote the outline")
	}
}

func TestPositionCompletionUncoveredAndNext(t *testing.T) {
	root, now := importBook(t, "plain text", "book.txt")
	if err := os.WriteFile(filepath.Join(root, "Curriculum", "book", "progress.md"), []byte("# Progress — Book\n\nNo completed steps yet.\n\n- 2026-09-25: completed session `s1` at the recorded position.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := curriculum.SetOutline(root, "book", []byte(ddiaOutline), false); err != nil {
		t.Fatal(err)
	}
	o, err := curriculum.ConfirmOutline(root, "book")
	if err != nil || o.Status != "confirmed" {
		t.Fatalf("confirm: %v %s", err, o.Status)
	}
	pos, _ := curriculum.LoadPosition(root, "book")
	if curriculum.PositionVerified(o, pos) {
		t.Fatal("empty position reported verified")
	}
	if _, err := curriculum.PositionAtNode(o, pos, "9.9"); err == nil {
		t.Fatal("unknown node accepted")
	}
	pos, err = curriculum.PositionAtNode(o, pos, "2.3")
	if err != nil || pos.Chapter != "2 定义非功能性需求" || pos.Section != "2.3 可伸缩性" || !curriculum.PositionVerified(o, pos) {
		t.Fatalf("position = %+v err=%v", pos, err)
	}
	if err := curriculum.SavePosition(root, "book", pos); err != nil {
		t.Fatal(err)
	}
	entries, _ := curriculum.LoadProgress(root, "book")
	st := curriculum.Statuses(o, entries, pos, nil)
	var unc []string
	for _, s := range curriculum.Uncovered(st) {
		unc = append(unc, s.ID)
	}
	if strings.Join(unc, ",") != "1,2.1,2.2" {
		t.Fatalf("uncovered = %v", unc)
	}
	if _, err := curriculum.Mark(root, "book", "1", "skipped", "", "", now); err == nil {
		t.Fatal("mark without reason accepted")
	}
	if _, err := curriculum.Mark(root, "book", "1", "skipped", "学习者已在工作中掌握，选择跳过", "", now); err != nil {
		t.Fatal(err)
	}
	for _, n := range []string{"2.1", "2.2", "2.3"} {
		if _, err := curriculum.Mark(root, "book", n, "completed", "完成迁移题", "s2", now); err != nil {
			t.Fatal(err)
		}
	}
	entries, _ = curriculum.LoadProgress(root, "book")
	st = curriculum.Statuses(o, entries, pos, nil)
	status := map[string]string{}
	for _, s := range st {
		status[s.ID] = s.Status
	}
	if status["1"] != "skipped" || status["1.1"] != "skipped" || status["2"] != "completed" || status["3"] != "not_started" || len(curriculum.Uncovered(st)) != 0 {
		t.Fatalf("statuses = %v", status)
	}
	next, ok := curriculum.NextNode(o, st, pos)
	if !ok || next.ID != "3" {
		t.Fatalf("next = %+v %v", next, ok)
	}
	md, _ := os.ReadFile(filepath.Join(root, "Curriculum", "book", "progress.md"))
	for _, want := range []string{"generated_by: learn", "↷ 已跳过 1 数据系统架构中的权衡", "✅ 已完成 2.3 可伸缩性", "旧记录：- 2026-09-25: completed session `s1`"} {
		if !strings.Contains(string(md), want) {
			t.Fatalf("progress.md missing %q:\n%s", want, md)
		}
	}
	if entries[0].Kind != "legacy" {
		t.Fatalf("legacy progress not adopted: %+v", entries[0])
	}
}

func TestPartialStatusAndNext(t *testing.T) {
	root, now := importBook(t, "plain text", "book.txt")
	if _, err := curriculum.SetOutline(root, "book", []byte(ddiaOutline), false); err != nil {
		t.Fatal(err)
	}
	o, _ := curriculum.ConfirmOutline(root, "book")
	pos, _ := curriculum.PositionAtNode(o, curriculum.Position{Book: "book"}, "2.3")
	if err := curriculum.SavePosition(root, "book", pos); err != nil {
		t.Fatal(err)
	}
	if err := curriculum.RecordSession(root, "book", "s1", now); err != nil {
		t.Fatal(err)
	}
	// Move back to 2.1: 2.3 now has a session but is not complete.
	pos, _ = curriculum.PositionAtNode(o, pos, "2.1")
	entries, _ := curriculum.LoadProgress(root, "book")
	st := curriculum.Statuses(o, entries, pos, map[string]bool{"1.2": true})
	status := map[string]string{}
	for _, s := range st {
		status[s.ID] = s.Status
	}
	if status["2.3"] != "partial" || status["1.2"] != "partial" || status["1"] != "partial" || status["1.1"] != "uncovered" || status["2.2"] != "not_started" {
		t.Fatalf("statuses = %v", status)
	}
	var partial, uncovered []string
	for _, s := range curriculum.Partial(st) {
		partial = append(partial, s.ID)
	}
	for _, s := range curriculum.Uncovered(st) {
		uncovered = append(uncovered, s.ID)
	}
	if strings.Join(partial, ",") != "1,2.3" || strings.Join(uncovered, ",") != "1.1" {
		t.Fatalf("partial=%v uncovered=%v", partial, uncovered)
	}
	pos, _ = curriculum.PositionAtNode(o, pos, "3")
	st = curriculum.Statuses(o, entries, pos, map[string]bool{"1.2": true})
	if next, ok := curriculum.NextNode(o, st, pos); !ok || next.ID != "1.2" {
		t.Fatalf("next should return to the partial entry first, got %+v", next)
	}
}

func TestArchiveRestorePurgeAndReimport(t *testing.T) {
	root, now := importBook(t, "# 极限\n", "calc.md")
	plan, err := curriculum.PlanRemoval(root, "book", now)
	if err != nil || !plan.Active || plan.Blocked != "" {
		t.Fatalf("plan = %+v err=%v", plan, err)
	}
	if _, err := os.Stat(filepath.Join(root, "Sources", "book")); err != nil {
		t.Fatal("dry-run plan moved files")
	}
	if _, err := curriculum.Remove(root, "book", "", now); err == nil {
		t.Fatal("remove without reason accepted")
	}
	a, err := curriculum.Remove(root, "book", "换一本教材", now)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"Sources/book", "Curriculum/book"} {
		if _, err := os.Stat(filepath.Join(root, p)); !os.IsNotExist(err) {
			t.Fatalf("%s still present", p)
		}
	}
	if ig, _ := os.ReadFile(filepath.Join(root, ".learning", "archive", ".gitignore")); !strings.Contains(string(ig), "source/original") {
		t.Fatal("archive originals not ignored")
	}
	if title := curriculum.ArchivedTitle(root, "book"); title != "Book" {
		t.Fatalf("archived title = %s", title)
	}
	src := filepath.Join(t.TempDir(), "calc.md")
	if err := os.WriteFile(src, []byte("# 极限\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := curriculum.Import(root, curriculum.ImportOptions{SourcePath: src, ID: "book", Title: "Book 2", Confirmed: true, Now: now})
	if err != nil || p.ArchivedAs != a.ArchiveID {
		t.Fatalf("reimport = %+v err=%v", p, err)
	}
	if _, err := curriculum.Restore(root, a.ArchiveID); err == nil {
		t.Fatal("restore over an existing id accepted")
	}
	b, err := curriculum.Remove(root, "book", "重复导入", now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := curriculum.Restore(root, a.ArchiveID); err != nil {
		t.Fatal(err)
	}
	if m, err := curriculum.LoadManifest(root, "book"); err != nil || m.Title != "Book" {
		t.Fatalf("restored manifest = %+v err=%v", m, err)
	}
	if _, err := curriculum.Purge(root, b.ArchiveID, "wrong"); err == nil {
		t.Fatal("purge without matching confirmation accepted")
	}
	if _, err := curriculum.Purge(root, b.ArchiveID, b.ArchiveID); err != nil {
		t.Fatal(err)
	}
	if list, _ := curriculum.ListArchives(root); len(list) != 0 {
		t.Fatalf("archives left: %+v", list)
	}
	if _, err := curriculum.Purge(root, "../x", "../x"); err == nil {
		t.Fatal("path traversal accepted")
	}
}

func TestOutlineCRLF(t *testing.T) {
	root, _ := importBook(t, "plain text", "book.txt")
	if _, err := curriculum.SetOutline(root, "book", []byte(strings.ReplaceAll(ddiaOutline, "\n", "\r\n")), false); err != nil {
		t.Fatal(err)
	}
	if o, _ := curriculum.LoadOutline(root, "book"); len(o.Nodes) != 8 {
		t.Fatalf("crlf outline nodes = %d", len(o.Nodes))
	}
}
