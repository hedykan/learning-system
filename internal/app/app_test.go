package app_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hedykan/learning-system/internal/app"
)

func TestCLIEndToEnd(t *testing.T) {
	root := filepath.Join(t.TempDir(), "Learning Vault")
	fixed := time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
	t.Setenv("LEARN_NOW", fixed.Format(time.RFC3339))
	run := func(stdin string, args ...string) string {
		t.Helper()
		var out bytes.Buffer
		a := app.New()
		a.Out = &out
		a.Err = &out
		a.In = bytes.NewBufferString(stdin)
		a.Now = func() time.Time { return fixed }
		cmd := a.RootCommand()
		cmd.SetArgs(args)
		if err := cmd.ExecuteContext(context.Background()); err != nil {
			t.Fatalf("learn %v: %v\n%s", args, err, out.String())
		}
		return out.String()
	}
	run("", "init", root, "--json")
	source := filepath.Join(t.TempDir(), "textbook.md")
	if err := os.WriteFile(source, []byte("# Textbook"), 0o644); err != nil {
		t.Fatal(err)
	}
	dryRun := run("", "--vault", root, "curriculum", "import", source, "--id", "textbook", "--activate", "--dry-run", "--json")
	var plan map[string]any
	if err := json.Unmarshal([]byte(dryRun), &plan); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, dryRun)
	}
	if plan["dry_run"] != true {
		t.Fatalf("dry_run = %#v", plan["dry_run"])
	}
	run("", "--vault", root, "curriculum", "import", source, "--id", "textbook", "--activate", "--yes")
	run("", "--vault", root, "curriculum", "position", "set", "--chapter", "1", "--section", "1.1", "--concept", "limits")
	run("", "--vault", root, "session", "start", "--kind", "lesson", "--skip-baseline")
	run("我理解了吗？", "--vault", root, "session", "append", "--role", "user")
	run("请先解释一个变式。", "--vault", root, "session", "append", "--role", "assistant")
	run("", "--vault", root, "session", "end", "--no-analysis", "--reason", "smoke test")
	statusJSON := run("", "--vault", root, "status", "--json")
	var status map[string]any
	if err := json.Unmarshal([]byte(statusJSON), &status); err != nil {
		t.Fatal(err)
	}
	if status["active_session"] != nil {
		t.Fatalf("active session was not cleared: %#v", status["active_session"])
	}
	if status["current_learning"] != "textbook" {
		t.Fatalf("current learning = %#v", status["current_learning"])
	}
}

func TestCLIInterpretationLoop(t *testing.T) {
	root := filepath.Join(t.TempDir(), "vault")
	fixed := time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
	t.Setenv("LEARN_NOW", fixed.Format(time.RFC3339))
	run := func(stdin string, wantError bool, args ...string) string {
		t.Helper()
		var out bytes.Buffer
		a := app.New()
		a.Out, a.Err, a.In, a.Now = &out, &out, bytes.NewBufferString(stdin), func() time.Time { return fixed }
		cmd := a.RootCommand()
		cmd.SetArgs(append([]string{"--vault", root}, args...))
		err := cmd.ExecuteContext(context.Background())
		if wantError != (err != nil) {
			t.Fatalf("learn %v: err=%v\n%s", args, err, out.String())
		}
		if err != nil {
			return err.Error()
		}
		return out.String()
	}
	run("", false, "init", root)
	source := filepath.Join(t.TempDir(), "ddia.md")
	if err := os.WriteFile(source, []byte("# DDIA"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("", false, "curriculum", "import", source, "--id", "ddia", "--activate", "--yes")
	run("", false, "curriculum", "position", "set", "--chapter", "1", "--concept", "尾延迟")
	run("", false, "session", "start", "--kind", "lesson", "--skip-baseline")
	run("哪个系统更能扛住？", false, "session", "append", "--role", "assistant")
	appended := run("B 平均更快，所以 B 更好", false, "session", "append", "--role", "user", "--json")
	if !strings.Contains(appended, `"turn": "t0002"`) {
		t.Fatalf("append output: %s", appended)
	}
	turns := run("", false, "session", "turns", "--role", "user", "--json")
	if !strings.Contains(turns, "B 平均更快") || strings.Contains(turns, "哪个系统") {
		t.Fatalf("turns output: %s", turns)
	}
	rec := `{"schema":"learning-os/interpretation@1","curriculum":"ddia",
	  "concepts":[{"id":"tail-latency","label":"尾延迟","source_ref":{"chapter":"1"}}],
	  "events":[{"id":"e1","type":"misconception","concept":"tail-latency","summary":"只看平均值","evidence":[{"turn":"t0002","quote":"B 平均更快"}]}]}`
	run(`{"schema":"learning-os/interpretation@1","curriculum":"ddia","bogus":1}`, true, "session", "checkpoint", "--analysis-file", "-")
	checkpoint := run(rec, false, "session", "checkpoint", "--analysis-file", "-", "--json")
	if !strings.Contains(checkpoint, `"status": "accepted"`) {
		t.Fatalf("checkpoint: %s", checkpoint)
	}
	next := run("", false, "next", "--json")
	if !strings.Contains(next, `"action": "repair_misconception"`) || !strings.Contains(next, `"strategy": "counterexample"`) {
		t.Fatalf("next: %s", next)
	}
	state := run("", false, "state", "--json")
	if !strings.Contains(state, `"available": true`) || !strings.Contains(state, `"tail-latency"`) {
		t.Fatalf("state: %s", state)
	}
	if concept := run("", false, "state", "concept", "尾延迟", "--json"); !strings.Contains(concept, "open_misconceptions") {
		t.Fatalf("concept: %s", concept)
	}
	run("", false, "curriculum", "detour", "start", "--topic", "百分位数", "--reason", "不清楚 p99 的含义", "--return-condition", "能解释 p99")
	status := run("", false, "status", "--json")
	if !strings.Contains(status, `"projections": "current"`) || !strings.Contains(status, `"topic": "百分位数"`) {
		t.Fatalf("status: %s", status)
	}
	run("", false, "curriculum", "detour", "end", "--outcome", "completed", "--learned", "p99 是第 99 百分位")
	end := `{"schema":"learning-os/interpretation@1","curriculum":"ddia","progress_decision":{"decision":"stay","reason":"误解尚未修正"}}`
	run(end, false, "session", "end", "--analysis-file", "-")
	rebuild := run("", false, "model", "rebuild", "--dry-run", "--json")
	if strings.Contains(rebuild, `"action": "update"`) || strings.Contains(rebuild, `"action": "create"`) {
		t.Fatalf("rebuild should be a no-op after end: %s", rebuild)
	}
	if !strings.Contains(run("", false, "version"), "v0.1.9") {
		t.Fatal("version not bumped")
	}
}

func TestBaselineLocksCurriculumPosition(t *testing.T) {
	root := filepath.Join(t.TempDir(), "vault")
	fixed := time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
	t.Setenv("LEARN_NOW", fixed.Format(time.RFC3339))
	run := func(stdin string, wantError bool, args ...string) string {
		t.Helper()
		var out bytes.Buffer
		a := app.New()
		a.Out, a.Err, a.In, a.Now = &out, &out, bytes.NewBufferString(stdin), func() time.Time { return fixed }
		cmd := a.RootCommand()
		cmd.SetArgs(args)
		err := cmd.ExecuteContext(context.Background())
		if wantError && err == nil {
			t.Fatalf("learn %v: expected error", args)
		}
		if !wantError && err != nil {
			t.Fatalf("learn %v: %v\n%s", args, err, out.String())
		}
		if err != nil {
			return err.Error()
		}
		return out.String()
	}
	run("", false, "init", root)
	source := filepath.Join(t.TempDir(), "book.md")
	if err := os.WriteFile(source, []byte("# Book"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("", false, "--vault", root, "curriculum", "import", source, "--id", "book", "--activate", "--yes")
	started := run("", false, "--vault", root, "session", "start", "--json")
	if !strings.Contains(started, `"kind": "baseline"`) || !strings.Contains(started, `"depth": "standard"`) {
		t.Fatalf("unexpected start output: %s", started)
	}
	errText := run("", true, "--vault", root, "curriculum", "position", "set", "--chapter", "1")
	if !strings.Contains(errText, "locked during a baseline") {
		t.Fatalf("unexpected position error: %s", errText)
	}
	position := run("", false, "--vault", root, "curriculum", "position", "--json")
	if strings.Contains(position, `"chapter": "1"`) {
		t.Fatalf("position advanced during baseline: %s", position)
	}
}

func TestCLICurriculumFidelity(t *testing.T) {
	root := filepath.Join(t.TempDir(), "vault")
	fixed := time.Date(2026, 9, 26, 10, 0, 0, 0, time.UTC)
	t.Setenv("LEARN_NOW", fixed.Format(time.RFC3339))
	run := func(stdin string, wantError bool, args ...string) string {
		t.Helper()
		var out bytes.Buffer
		a := app.New()
		a.Out, a.Err, a.In, a.Now = &out, &out, bytes.NewBufferString(stdin), func() time.Time { return fixed }
		cmd := a.RootCommand()
		cmd.SetArgs(append([]string{"--vault", root}, args...))
		err := cmd.ExecuteContext(context.Background())
		if wantError != (err != nil) {
			t.Fatalf("learn %v: err=%v\n%s", args, err, out.String())
		}
		if err != nil {
			return err.Error()
		}
		return out.String()
	}
	run("", false, "init", root)
	if _, err := os.Stat(filepath.Join(root, ".learning", "tmp", ".gitignore")); err != nil {
		t.Fatal("init did not create .learning/tmp")
	}
	src := filepath.Join(t.TempDir(), "ddia.pdf")
	if err := os.WriteFile(src, []byte("%PDF-1.4 fake"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("", false, "curriculum", "import", src, "--id", "ddia", "--activate", "--yes")
	run("", false, "curriculum", "position", "set", "--chapter", "第1章 可靠、可扩展与可维护的应用", "--concept", "负载")
	status := run("", false, "status", "--json")
	if !strings.Contains(status, `"outline": "missing"`) || !strings.Contains(status, `"position_verified": false`) {
		t.Fatalf("status before outline: %s", status)
	}
	outline := `{"nodes":[{"id":"1","title":"数据系统架构中的权衡","pages":[10,29]},{"id":"2","title":"定义非功能性需求","pages":[30,60]},
	  {"id":"2.1","title":"描述性能","pages":[34,38]},{"id":"2.2","title":"可伸缩性","pages":[45,50]},{"id":"3","title":"数据模型与查询语言","pages":[61,100]}]}`
	run(`{"nodes":[{"id":"2","title":"x"},{"id":"1","title":"y"}]}`, true, "curriculum", "outline", "set", "--file", "-")
	run(outline, false, "curriculum", "outline", "set", "--file", "-")
	run("", false, "curriculum", "outline", "confirm")
	if got := run("", true, "curriculum", "position", "set", "--chapter", "第2章"); !strings.Contains(got, "--node") {
		t.Fatalf("label set after confirm: %s", got)
	}
	run("", true, "curriculum", "position", "set", "--node", "9")
	run("", false, "curriculum", "position", "set", "--node", "2.2", "--concept", "负载参数")
	status = run("", false, "status", "--json")
	for _, want := range []string{`"position_verified": true`, `"chapter": "2 定义非功能性需求"`, `"id": "1"`, `"id": "2.1"`} {
		if !strings.Contains(status, want) {
			t.Fatalf("status missing %s: %s", want, status)
		}
	}
	run("", false, "curriculum", "skip", "1", "--reason", "学习者选择先学第 2 章，第 1 章稍后补")
	if done := run("", false, "curriculum", "complete", "2.1", "--reason", "能解释 p99 与平均值", "--json"); !strings.Contains(done, `"next_node"`) || !strings.Contains(done, `"id": "2.2"`) {
		t.Fatalf("complete should name the next entry: %s", done)
	}
	if status = run("", false, "status", "--json"); !strings.Contains(status, `"uncovered": []`) {
		t.Fatalf("uncovered after marking: %s", status)
	}
	run("", false, "session", "start", "--kind", "lesson", "--skip-baseline")
	turn := filepath.Join(root, ".learning", "tmp", "turn.txt")
	if err := os.WriteFile(turn, []byte("请预测：加机器能解决热点吗？"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("", false, "session", "append", "--role", "assistant", "--file", turn)
	run("加机器也分不过去，热点都在一个分片上", false, "session", "append", "--role", "user")
	rec := `{"schema":"learning-os/interpretation@1","curriculum":"ddia",
	  "concepts":[{"id":"hotspot","label":"热点"}],
	  "events":[{"id":"e1","type":"prediction","concept":"hotspot","summary":"预测加机器无法分散热点","evidence":[{"turn":"t0002","quote":"热点都在一个分片上"}]}],
	  "state_updates":[{"id":"u1","concept":"hotspot","state":"developing","capabilities":["predicted","applied"],"summary":"能预测热点","evidence":[{"turn":"t0002","quote":"加机器也分不过去"},{"turn":"t0002","quote":"加机器也分不过去，热点都在一个分片上"}]}]}`
	run(rec, false, "session", "checkpoint", "--analysis-file", "-")
	concept := run("", false, "state", "concept", "hotspot", "--json")
	if !strings.Contains(concept, `"node": "2.2"`) {
		t.Fatalf("concept not anchored to the outline node: %s", concept)
	}
	note, _ := os.ReadFile(filepath.Join(root, "Concepts", "热点.md"))
	if strings.Count(string(note), "- 「加机器也分不过去") != 1 {
		t.Fatalf("overlapping quotes not merged:\n%s", note)
	}
	end := `{"schema":"learning-os/interpretation@1","curriculum":"ddia","progress_decision":{"decision":"advance","reason":"热点已能应用"}}`
	run(end, false, "session", "end", "--analysis-file", "-")
	if done := run("", false, "curriculum", "complete", "2.2", "--reason", "能分析热点分片", "--json"); !strings.Contains(done, `"moved_to": {`) || !strings.Contains(done, `"id": "3"`) {
		t.Fatalf("completing the current entry should advance to 3: %s", done)
	}
	if pos := run("", false, "curriculum", "position", "--json"); !strings.Contains(pos, `"node": "3"`) {
		t.Fatalf("position not advanced: %s", pos)
	}
	run("", false, "curriculum", "position", "set", "--node", "2.2") // back for a review
	next := run("", false, "next", "--json")
	if !strings.Contains(next, `"rule": "R4b-consolidate"`) {
		t.Fatalf("next session should consolidate first: %s", next)
	}
	index, _ := os.ReadFile(filepath.Join(root, "Curriculum", "ddia", "ddia.md"))
	for _, want := range []string{"## 目录", "↷ 已跳过 1 数据系统架构中的权衡", "✅ 已完成 2.2 可伸缩性", "○ 未开始 3 数据模型与查询语言"} {
		if !strings.Contains(string(index), want) {
			t.Fatalf("index missing %q:\n%s", want, index)
		}
	}
	progress, _ := os.ReadFile(filepath.Join(root, "Curriculum", "ddia", "学习进度.md"))
	if !strings.Contains(string(progress), "完成 2.2 可伸缩性：能分析热点分片") || !strings.Contains(string(progress), "结束学习") {
		t.Fatalf("progress log:\n%s", progress)
	}
	if out := run("", false, "commit", "--message", "manual"); !strings.Contains(out, "Committed") && !strings.Contains(out, "Nothing") {
		t.Fatalf("commit: %s", out)
	}
}

func TestCLIHomeTextbookPointsAndArchive(t *testing.T) {
	root := filepath.Join(t.TempDir(), "vault")
	fixed := time.Date(2026, 9, 26, 10, 0, 0, 0, time.UTC)
	t.Setenv("LEARN_NOW", fixed.Format(time.RFC3339))
	clock := fixed
	run := func(stdin string, wantError bool, args ...string) string {
		t.Helper()
		var out bytes.Buffer
		a := app.New()
		a.Out, a.Err, a.In, a.Now = &out, &out, bytes.NewBufferString(stdin), func() time.Time { return clock }
		cmd := a.RootCommand()
		cmd.SetArgs(append([]string{"--vault", root}, args...))
		err := cmd.ExecuteContext(context.Background())
		if wantError != (err != nil) {
			t.Fatalf("learn %v: err=%v\n%s", args, err, out.String())
		}
		if err != nil {
			return err.Error()
		}
		return out.String()
	}
	if err := os.MkdirAll(filepath.Join(root, "Ideas"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "Ideas", "mine.md"), []byte("我的想法"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("我自己写的说明"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("", false, "init", root)
	home := read(t, filepath.Join(root, "README.md"))
	for _, want := range []string{"generated_by: learn", "还没有导入教材", "我自己写的说明", "## 怎么用"} {
		if !strings.Contains(home, want) {
			t.Fatalf("empty home missing %q:\n%s", want, home)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "Insights")); !os.IsNotExist(err) {
		t.Fatal("init still creates legacy directories")
	}
	// Simulate an old Vault with an empty legacy directory.
	if err := os.MkdirAll(filepath.Join(root, "Questions"), 0o755); err != nil {
		t.Fatal(err)
	}
	plan := run("", false, "agent", "update", "--dry-run", "--json")
	if !strings.Contains(plan, `"path": "Questions/"`) || !strings.Contains(plan, "keep-dir-with-user-files") {
		t.Fatalf("tidy plan: %s", plan)
	}
	if _, err := os.Stat(filepath.Join(root, "Questions")); err != nil {
		t.Fatal("dry run removed a directory")
	}
	run("", false, "agent", "update", "--yes")
	if _, err := os.Stat(filepath.Join(root, "Questions")); !os.IsNotExist(err) {
		t.Fatal("empty legacy directory kept")
	}
	if _, err := os.Stat(filepath.Join(root, "Ideas", "mine.md")); err != nil {
		t.Fatal("user file in legacy directory removed")
	}

	src := filepath.Join(t.TempDir(), "ddia.pdf")
	if err := os.WriteFile(src, []byte("%PDF-1.4 fake"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("", false, "curriculum", "import", src, "--id", "ddia", "--title", "DDIA", "--activate", "--yes")
	run(`{"nodes":[{"id":"2","title":"定义非功能性需求","pages":[30,60]},{"id":"2.2","title":"描述性能","pages":[34,38]},{"id":"2.3","title":"可靠性与容错","pages":[39,44]}]}`, false, "curriculum", "outline", "set", "--file", "-")
	run("", false, "curriculum", "outline", "confirm")
	run("", false, "curriculum", "position", "set", "--node", "2.2")
	run("", false, "session", "start", "--kind", "lesson", "--skip-baseline")
	run("B 的平均值更低", false, "session", "append", "--role", "user")
	clock = fixed.Add(47 * time.Minute)
	run("不能只看平均值，还要看最慢的那批请求", false, "session", "append", "--role", "user")
	bad := `{"schema":"learning-os/interpretation@1","curriculum":"ddia","concepts":[{"id":"tail","label":"尾延迟","textbook_points":{"pages":[50,52],"points":["响应时间是一个分布"]}}],
	  "events":[{"id":"e1","type":"insight","concept":"tail","summary":"看尾部","evidence":[{"turn":"t0002","quote":"不能只看平均值"}]}]}`
	if got := run(bad, true, "session", "checkpoint", "--analysis-file", "-"); !strings.Contains(got, "outside outline entry 2.2") {
		t.Fatalf("out-of-range points: %s", got)
	}
	good := strings.Replace(bad, "[50,52]", "[34,36]", 1)
	run(good, false, "session", "checkpoint", "--analysis-file", "-")
	if got := run("", true, "curriculum", "remove", "ddia", "--reason", "x", "--yes"); !strings.Contains(got, "active") {
		t.Fatalf("remove during session: %s", got)
	}
	run("", false, "session", "end")
	run("", false, "curriculum", "position", "set", "--node", "2.3")
	note := read(t, filepath.Join(root, "Concepts", "尾延迟.md"))
	wantTime := fixed.Add(47*time.Minute).Local().Format("2006-01-02 15:04") + " · t0002"
	for _, want := range []string{"## 教材要点", "AI 根据原书第 34–36 页概括", "- 响应时间是一个分布", wantTime} {
		if !strings.Contains(note, want) {
			t.Fatalf("concept note missing %q:\n%s", want, note)
		}
	}
	home = read(t, filepath.Join(root, "README.md"))
	for _, want := range []string{"[[Curriculum/ddia/DDIA|DDIA]]", "学到：2 定义非功能性需求 / 2.3 可靠性与容错", "完成 0 / 2 节", "形成中 0 个", "[[Sessions/session-", "我自己写的说明"} {
		if !strings.Contains(home, want) {
			t.Fatalf("home missing %q:\n%s", want, home)
		}
	}
	status := run("", false, "status", "--json")
	if !strings.Contains(status, `"partial": [`) || !strings.Contains(status, `"id": "2.2"`) {
		t.Fatalf("status partial: %s", status)
	}
	index := read(t, filepath.Join(root, "Curriculum", "ddia", "DDIA.md"))
	if !strings.Contains(index, "◐ 学过一部分 2.2 描述性能") {
		t.Fatalf("index lacks partial:\n%s", index)
	}
	before := home
	if dry := run("", false, "model", "rebuild", "--dry-run", "--json"); strings.Contains(dry, `"action": "update"`) || strings.Contains(dry, `"action": "create"`) {
		t.Fatalf("rebuild would rewrite unchanged projections: %s", dry)
	}
	if err := os.Remove(filepath.Join(root, "README.md")); err != nil {
		t.Fatal(err)
	}
	run("", false, "model", "rebuild")
	if got := read(t, filepath.Join(root, "README.md")); !strings.Contains(got, "## 正在学习") || strings.Contains(got, "我自己写的说明") {
		t.Fatalf("home not rebuilt from records:\n%s", got)
	}
	_ = before
	run("", false, "curriculum", "remove", "ddia", "--dry-run", "--json")
	archived := run("", false, "curriculum", "remove", "ddia", "--reason", "测试归档", "--yes", "--json")
	if !strings.Contains(archived, `"archive_id": "ddia-`) {
		t.Fatalf("archive: %s", archived)
	}
	if _, err := os.Stat(filepath.Join(root, "Curriculum", "ddia")); !os.IsNotExist(err) {
		t.Fatal("projection recreated the archived curriculum directory")
	}
	if note := read(t, filepath.Join(root, "Concepts", "尾延迟.md")); !strings.Contains(note, "DDIA（已归档）") {
		t.Fatalf("archived source label:\n%s", note)
	}
	if list := run("", false, "curriculum", "archives", "--json"); !strings.Contains(list, "测试归档") {
		t.Fatalf("archives: %s", list)
	}
}

func read(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestCLIReviewAndGraph(t *testing.T) {
	root := filepath.Join(t.TempDir(), "vault")
	day1 := time.Date(2026, 10, 1, 4, 0, 0, 0, time.UTC)
	clock := day1
	t.Setenv("LEARN_NOW", day1.Format(time.RFC3339))
	run := func(stdin string, wantError bool, args ...string) string {
		t.Helper()
		var out bytes.Buffer
		a := app.New()
		a.Out, a.Err, a.In, a.Now = &out, &out, bytes.NewBufferString(stdin), func() time.Time { return clock }
		cmd := a.RootCommand()
		cmd.SetArgs(append([]string{"--vault", root}, args...))
		err := cmd.ExecuteContext(context.Background())
		if wantError != (err != nil) {
			t.Fatalf("learn %v: err=%v\n%s", args, err, out.String())
		}
		if err != nil {
			return err.Error()
		}
		return out.String()
	}
	advance := func(d time.Duration) {
		clock = clock.Add(d)
		t.Setenv("LEARN_NOW", clock.Format(time.RFC3339))
	}
	run("", false, "init", root)
	src := filepath.Join(t.TempDir(), "ddia.md")
	if err := os.WriteFile(src, []byte("# 定义非功能性需求\n\n## 描述性能\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("", false, "curriculum", "import", src, "--id", "ddia", "--title", "DDIA", "--activate", "--yes")
	run("", false, "curriculum", "outline", "confirm")
	run("", false, "curriculum", "position", "set", "--node", "1.1")
	run("", false, "session", "start", "--kind", "lesson", "--skip-baseline")
	run("哪个更能扛住？", false, "session", "append", "--role", "assistant")
	run("不能只看平均值，还要看最慢的请求；吞吐量也要看", false, "session", "append", "--role", "user")
	rec := `{"schema":"learning-os/interpretation@1","curriculum":"ddia",
	  "concepts":[{"id":"tail","label":"尾延迟","aliases":["p99 延迟"]},{"id":"throughput","label":"吞吐量","related":[{"concept":"tail","note":"一起描述性能"}]}],
	  "state_updates":[{"id":"u1","concept":"tail","state":"developing","capabilities":["explained"],"summary":"能解释","evidence":[{"turn":"t0002","quote":"不能只看平均值"}]},
	                   {"id":"u2","concept":"throughput","state":"developing","capabilities":["explained"],"summary":"知道要看","evidence":[{"turn":"t0002","quote":"吞吐量也要看"}]}]}`
	run(rec, false, "session", "checkpoint", "--analysis-file", "-")
	run("", false, "session", "end")
	if due := run("", false, "review", "--json"); !strings.Contains(due, `"due": []`) {
		t.Fatalf("nothing should be due on the learning day: %s", due)
	}
	tail := read(t, filepath.Join(root, "Concepts", "尾延迟.md"))
	for _, want := range []string{"aliases:\n  - \"p99 延迟\"", "  - learning/state/developing", "  - learning/curriculum/ddia", "## 相关概念", "[[Concepts/吞吐量|吞吐量]]：一起描述性能", "## 复习计划", "下次复习：2026-10-02"} {
		if !strings.Contains(tail, want) {
			t.Fatalf("tail note missing %q:\n%s", want, tail)
		}
	}
	if !strings.Contains(read(t, filepath.Join(root, "Concepts", "吞吐量.md")), "[[Concepts/尾延迟|尾延迟]]") {
		t.Fatal("relation not shown on the other end")
	}

	advance(48 * time.Hour)
	due := run("", false, "review", "--json")
	if !strings.Contains(due, `"concept": "tail"`) || !strings.Contains(due, `"today": "2026-10-03"`) {
		t.Fatalf("review due: %s", due)
	}
	if next := run("", false, "next", "--json"); !strings.Contains(next, `"rule": "R3b-review-due"`) {
		t.Fatalf("next: %s", next)
	}
	run("", false, "session", "start", "--kind", "review")
	run("还记得尾延迟吗？", false, "session", "append", "--role", "assistant")
	run("要看最慢那批请求的延迟，比如p99", false, "session", "append", "--role", "user")
	result := `{"schema":"learning-os/interpretation@1","curriculum":"ddia","review_results":[{"id":"r1","concept":"tail","outcome":"recalled","action_turn":"t0001","evidence":[{"turn":"t0002","quote":"要看最慢那批请求的延迟"}]}]}`
	run(result, false, "session", "checkpoint", "--analysis-file", "-")
	home := read(t, filepath.Join(root, "README.md"))
	if !strings.Contains(home, "## 今天该复习（2026-10-03）") || !strings.Contains(home, "[[Concepts/吞吐量|吞吐量]]（2026-10-02 到期") {
		t.Fatalf("home review section:\n%s", home)
	}
	if strings.Contains(home, "[[Concepts/尾延迟|尾延迟]]（") {
		t.Fatalf("reviewed concept still listed as due:\n%s", home)
	}
	if !strings.Contains(read(t, filepath.Join(root, "Concepts", "尾延迟.md")), "下次复习：2026-10-05") {
		t.Fatal("recalled review did not advance the interval")
	}
	if !strings.Contains(read(t, filepath.Join(root, "Profile", "学习者总览.md")), "## 复习日程") {
		t.Fatal("overview lacks review schedule")
	}
}

func TestCLIRelationGateAtSessionEnd(t *testing.T) {
	root := filepath.Join(t.TempDir(), "vault")
	fixed := time.Date(2026, 10, 1, 4, 0, 0, 0, time.UTC)
	t.Setenv("LEARN_NOW", fixed.Format(time.RFC3339))
	run := func(stdin string, wantError bool, args ...string) string {
		t.Helper()
		var out bytes.Buffer
		a := app.New()
		fixed = fixed.Add(time.Minute)
		now := fixed
		a.Out, a.Err, a.In, a.Now = &out, &out, bytes.NewBufferString(stdin), func() time.Time { return now }
		cmd := a.RootCommand()
		cmd.SetArgs(append([]string{"--vault", root}, args...))
		err := cmd.ExecuteContext(context.Background())
		if wantError != (err != nil) {
			t.Fatalf("learn %v: err=%v\n%s", args, err, out.String())
		}
		if err != nil {
			return err.Error()
		}
		return out.String()
	}
	run("", false, "init", root)
	src := filepath.Join(t.TempDir(), "b.md")
	if err := os.WriteFile(src, []byte("# 一\n\n## 甲\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("", false, "curriculum", "import", src, "--id", "b", "--activate", "--yes")
	lesson := func(concept, label, answer string) {
		run("", false, "session", "start", "--kind", "lesson", "--skip-baseline")
		run("请解释。", false, "session", "append", "--role", "assistant")
		run(answer, false, "session", "append", "--role", "user")
		rec := fmt.Sprintf(`{"schema":"learning-os/interpretation@1","curriculum":"b","concepts":[{"id":"%s","label":"%s"}],
		  "state_updates":[{"id":"u1","concept":"%s","state":"developing","capabilities":["explained"],"summary":"能解释","evidence":[{"turn":"t0002","quote":"%s"}]}]}`, concept, label, concept, answer)
		run(rec, false, "session", "checkpoint", "--analysis-file", "-")
	}
	lesson("alpha", "阿尔法", "阿尔法就是第一个")
	run("", false, "session", "end") // only one concept: nothing to link to
	lesson("beta", "贝塔", "贝塔是第二个")
	got := run("", true, "session", "end")
	if !strings.Contains(got, "beta（贝塔）: existing concepts alpha（阿尔法）") {
		t.Fatalf("gate message: %s", got)
	}
	if status := run("", false, "status", "--json"); !strings.Contains(status, `"active_session": {`) {
		t.Fatal("rejected end closed the session")
	}
	run(`{"schema":"learning-os/interpretation@1","curriculum":"b","no_related":["beta"]}`, false, "session", "end", "--analysis-file", "-")
	lesson("gamma", "伽马", "伽马是第三个")
	run(`{"schema":"learning-os/interpretation@1","curriculum":"b","concepts":[{"id":"gamma","related":[{"concept":"alpha","note":"同属一组"}]}]}`, false, "session", "end", "--analysis-file", "-")
	lesson2 := `{"schema":"learning-os/interpretation@1","curriculum":"b","state_updates":[{"id":"u2","concept":"beta","state":"developing","capabilities":["predicted"],"summary":"再次","evidence":[{"turn":"t0002","quote":"贝塔还是第二个"}]}]}`
	run("", false, "session", "start", "--kind", "lesson", "--skip-baseline")
	run("再说一次？", false, "session", "append", "--role", "assistant")
	run("贝塔还是第二个", false, "session", "append", "--role", "user")
	run(lesson2, false, "session", "end", "--analysis-file", "-") // beta was declared unrelated before
	if note := read(t, filepath.Join(root, "Concepts", "阿尔法.md")); !strings.Contains(note, "[[Concepts/伽马|伽马]]：同属一组") {
		t.Fatalf("alpha note lacks relation:\n%s", note)
	}
}

func TestCLIStableGateAndKeyQuestions(t *testing.T) {
	root := filepath.Join(t.TempDir(), "vault")
	clock := time.Date(2026, 10, 1, 1, 0, 0, 0, time.UTC)
	run := func(stdin string, wantError bool, args ...string) string {
		t.Helper()
		clock = clock.Add(time.Minute)
		t.Setenv("LEARN_NOW", clock.Format(time.RFC3339))
		now := clock
		var out bytes.Buffer
		a := app.New()
		a.Out, a.Err, a.In, a.Now = &out, &out, bytes.NewBufferString(stdin), func() time.Time { return now }
		cmd := a.RootCommand()
		cmd.SetArgs(append([]string{"--vault", root}, args...))
		err := cmd.ExecuteContext(context.Background())
		if wantError != (err != nil) {
			t.Fatalf("learn %v: err=%v\n%s", args, err, out.String())
		}
		if err != nil {
			return err.Error()
		}
		return out.String()
	}
	nextDay := func() { clock = clock.Add(24 * time.Hour) }
	run("", false, "init", root)
	src := filepath.Join(t.TempDir(), "c.md")
	if err := os.WriteFile(src, []byte("# 缓存\n\n## 强缓存\n\n## 协商缓存\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("", false, "curriculum", "import", src, "--id", "c", "--activate", "--yes")
	run("", false, "curriculum", "outline", "confirm")
	run("", false, "curriculum", "position", "set", "--node", "1.1")
	lesson := func(answer, rec string, wantError bool) string {
		t.Helper()
		run("", false, "session", "start", "--kind", "lesson", "--skip-baseline")
		run("请回答。", false, "session", "append", "--role", "assistant")
		run(answer, false, "session", "append", "--role", "user")
		return run(rec, wantError, "session", "checkpoint", "--analysis-file", "-")
	}
	header := `{"schema":"learning-os/interpretation@1","curriculum":"c",`
	lesson("有效期内直接用本地副本不发请求", header+`"concepts":[{"id":"strong","label":"强缓存"}],
	  "state_updates":[{"id":"u1","concept":"strong","state":"developing","capabilities":["explained"],"summary":"能解释","evidence":[{"turn":"t0002","quote":"有效期内直接用本地副本"}]}]}`, false)
	run(header+`"no_related":["strong"]}`, false, "session", "end", "--analysis-file", "-")
	nextDay()
	lesson("还记得，有效期内不发请求", header+`"state_updates":[{"id":"u2","concept":"strong","state":"stable","capabilities":["retrieved"],"summary":"能独立回忆","evidence":[{"turn":"t0002","quote":"有效期内不发请求"}]}]}`, false)
	run("", false, "session", "end")
	nextDay()
	relapse := header + `"events":[{"id":"e1","type":"misconception","concept":"strong","summary":"以为每次都问服务器","evidence":[{"turn":"t0002","quote":"每次都会先问服务器"}]}]`
	got := lesson("浏览器每次都会先问服务器", relapse+`}`, true)
	if !strings.Contains(got, "strong（强缓存）was stable, but this record shows a misconception") {
		t.Fatalf("stable gate message: %s", got)
	}
	if st := run("", false, "state", "concept", "strong", "--json"); !strings.Contains(st, `"state": "stable"`) {
		t.Fatalf("rejected record changed the model: %s", st)
	}
	run(relapse+`,"state_updates":[{"id":"u3","concept":"strong","state":"fragile","capabilities":["explained"],"summary":"复发误解","evidence":[{"turn":"t0002","quote":"每次都会先问服务器"}]}]}`, false, "session", "checkpoint", "--analysis-file", "-")
	st := run("", false, "state", "concept", "strong", "--json")
	if !strings.Contains(st, `"state": "fragile"`) || !strings.Contains(st, `"state": "stable"`) {
		t.Fatalf("history should keep stable and end fragile: %s", st)
	}

	// Key questions.
	run("那为什么内容没变还要问一次？", false, "session", "append", "--role", "user")
	q := func(node, quote string) string {
		return header + fmt.Sprintf(`"questions":[{"id":"why-ask-if-unchanged","question":"内容没变为什么还要问一次","concept":"strong","node":"%s","evidence":[{"turn":"t0003","quote":"%s"}]}]}`, node, quote)
	}
	run(q("9.9", "为什么内容没变还要问一次"), true, "session", "checkpoint", "--analysis-file", "-")
	run(q("1.2", "这句话不存在"), true, "session", "checkpoint", "--analysis-file", "-")
	run(q("1.2", "为什么内容没变还要问一次"), false, "session", "checkpoint", "--analysis-file", "-")
	note := read(t, filepath.Join(root, "Questions", "内容没变为什么还要问一次.md"))
	for _, want := range []string{"# 内容没变为什么还要问一次", "learning/question/open", "预计在目录条目 1.2 回答", "尚未解决"} {
		if !strings.Contains(note, want) {
			t.Fatalf("question note missing %q:\n%s", want, note)
		}
	}
	if home := read(t, filepath.Join(root, "README.md")); !strings.Contains(home, "## 我提出的问题") {
		t.Fatalf("home lacks open questions:\n%s", home)
	}
	if next := run("", false, "next", "--json"); !strings.Contains(next, `"open_questions": [`) {
		t.Fatalf("next lacks open questions: %s", next)
	}
	run("有效期内其实根本不问服务器", false, "session", "append", "--role", "user")
	run(header+`"events":[{"id":"e2","type":"correction","concept":"strong","summary":"纠正误解","evidence":[{"turn":"t0004","quote":"有效期内其实根本不问服务器"}]}]}`, false, "session", "checkpoint", "--analysis-file", "-")
	run("", false, "curriculum", "position", "set", "--node", "1.2")
	if next := run("", false, "next", "--json"); !strings.Contains(next, `"rule": "R2b-open-question"`) {
		t.Fatalf("next should address the question at its node: %s", next)
	}
	run("因为强缓存过期后浏览器不知道内容变没变，必须问一次", false, "session", "append", "--role", "user")
	resolve := header + `"question_resolutions":[{"question":"why-ask-if-unchanged","summary":"过期后浏览器无法得知内容是否变化，所以必须问","evidence":[{"turn":"t0005","quote":"过期后浏览器不知道内容变没变"}]}]}`
	run(resolve, false, "session", "checkpoint", "--analysis-file", "-")
	if note := read(t, filepath.Join(root, "Questions", "内容没变为什么还要问一次.md")); !strings.Contains(note, "learning/question/resolved") || !strings.Contains(note, "过期后浏览器无法得知") {
		t.Fatalf("resolved note:\n%s", note)
	}
	again := strings.Replace(resolve, "所以必须问", "换个说法", 1)
	run(again, true, "session", "checkpoint", "--analysis-file", "-")
	if next := run("", false, "next", "--json"); strings.Contains(next, "R2b-open-question") {
		t.Fatalf("resolved question still drives next: %s", next)
	}
}
