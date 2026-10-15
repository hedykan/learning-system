package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
)

// Server is the local app backend: the tutor driver plus the read models the
// screens render. One tutor, one conversation at a time.
type Server struct {
	Vault string
	mu    sync.Mutex
	tools *Tools // holds PendingUser between messages
}

func NewServer(vault string) *Server {
	return &Server{Vault: vault, tools: &Tools{Vault: vault}}
}

// ---- small map helpers ----

func mmap(v any) map[string]any {
	m, _ := v.(map[string]any)
	return m
}

func mstr(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if s, ok := m[k].(string); ok && s != "" {
			return s
		}
	}
	return ""
}

// learnerSays records a learner message (now, or when the model starts a
// session) and adds it to the history.
func (s *Server) learnerSays(history []Message, text string) []Message {
	if activeSession(s.Vault) != nil {
		turn := appendTurn(s.Vault, "user", text)
		return append(history, Message{"role": "user", "content": "[" + turn + "] " + text})
	}
	s.tools.PendingUser = text
	return append(history, Message{"role": "user", "content": "[recorded when a session starts] " + text})
}

// startReview opens the review screen's own review session. A lesson in
// progress is suspended and resumes when the review ends, so the two never mix.
func (s *Server) startReview() error {
	if reviewing(s.Vault) {
		return nil
	}
	r := runCLI(s.Vault, "", "session", "start", "--kind", "review", "--json")
	if !r.OK {
		if strings.Contains(r.Output, "baseline") {
			return fmt.Errorf("摸底还没结束，先在学习页完成摸底再来复习")
		}
		return fmt.Errorf("%s", truncate(r.Output, 200))
	}
	forgetHistory(s.Vault, "", "review")
	forgetReviewQuestions(s.Vault, "")
	return nil
}

// endReview ends the review session, if one is open; the suspended lesson
// resumes. It reports whether a review was open.
func (s *Server) endReview(ctx context.Context, emit Emit) bool {
	if !reviewing(s.Vault) {
		forgetReviewQuestions(s.Vault, "")
		return false
	}
	r := runCLI(s.Vault, "", "session", "end", "--json")
	if !r.OK {
		if strings.Contains(r.Output, "needs --analysis-file") {
			// nothing was answered
			runCLI(s.Vault, "", "session", "abort", "--reason", "the learner left the review screen before answering", "--json")
		} else {
			// the runtime wants more, e.g. relations: let the tutor finish it
			h := loadHistory(s.Vault, "", "review")
			h = append(h, Message{"role": "user", "content": "[app] The learner left the review screen. Ending the review failed: " + truncate(r.Output, 600) + " Fix it and end the session with session_end. Then reply with one short sentence."})
			_, h2, _, _ := RunAgent(ctx, s.tools, h, emit, false)
			if h2 != nil {
				h = h2
			}
			saveHistory(s.Vault, "", "review", h)
			if reviewing(s.Vault) {
				runCLI(s.Vault, "", "session", "abort", "--reason", "the review could not be ended", "--json")
			}
		}
	}
	forgetHistory(s.Vault, "", "review")
	forgetReviewQuestions(s.Vault, "")
	return true
}

// ---- read models for the screens ----

// Overview is the learning page's state.
func (s *Server) Overview() map[string]any {
	st := learnJSON(s.Vault, "status")
	pos := mmap(st["position"])
	outline := learnJSON(s.Vault, "curriculum", "outline", "show")
	nodes := []map[string]any{}
	for _, n := range asSlice(outline["nodes"]) {
		m := mmap(n)
		depth, _ := m["depth"].(float64)
		if depth == 0 {
			depth = 1
		}
		nodes = append(nodes, map[string]any{"id": m["id"], "title": m["title"], "depth": depth, "status": m["status"]})
	}
	return map[string]any{
		"curriculum":     st["current_learning"],
		"section":        mstr(pos, "section", "chapter"),
		"node":           pos["node"],
		"active_session": st["active_session"],
		"outline":        nodes,
		"outline_status": st["outline"],
		"resources":      orEmpty(st["node_resources"]),
		"model":          ActiveModel().ID,
	}
}

func orEmpty(v any) any {
	if v == nil {
		return []any{}
	}
	return v
}

// Courses lists every curriculum with a one-line progress.
func (s *Server) Courses() map[string]any {
	active, _ := mmap(learnJSON(s.Vault, "status"))["current_learning"].(string)
	out := []map[string]any{}
	for _, c := range learnJSONList(s.Vault, "curriculum", "list") {
		m := mmap(c)
		id := mstr(m, "id")
		o := learnJSON(s.Vault, "curriculum", "outline", "show", id)
		nodes := asSlice(o["nodes"])
		done := 0
		for _, n := range nodes {
			if mmap(n)["status"] == "completed" {
				done++
			}
		}
		meta := "待整理大纲"
		if len(nodes) > 0 {
			meta = fmt.Sprintf("%d / %d 节", done, len(nodes))
		}
		title := mstr(m, "title")
		if title == "" {
			title = id
		}
		out = append(out, map[string]any{"id": id, "title": title, "meta": meta})
	}
	return map[string]any{"active": active, "courses": out}
}

func frontmatterValue(text, key string) string {
	re := regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(key) + `:\s*(.*?)\s*$`)
	m := re.FindStringSubmatch(text)
	if len(m) < 2 {
		return ""
	}
	return strings.Trim(strings.TrimSpace(m[1]), `"'`)
}

func sessionTime(id string) string {
	re := regexp.MustCompile(`^session-(\d{4})(\d{2})(\d{2})-(\d{2})(\d{2})(\d{2})`)
	m := re.FindStringSubmatch(id)
	if len(m) == 0 {
		return ""
	}
	return fmt.Sprintf("%s-%s-%sT%s:%s:%s", m[1], m[2], m[3], m[4], m[5], m[6])
}

// CourseSessions indexes finished session pages and the current runtime session
// without activating the requested course.
func (s *Server) CourseSessions(cid string, includeReviews bool, limit int) []map[string]any {
	rows := []map[string]any{}
	seen := map[string]bool{}
	folder := filepath.Join(s.Vault, "Sessions")
	entries, _ := os.ReadDir(folder)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(folder, entry.Name()))
		if err != nil {
			continue
		}
		head := string(b)
		if len(head) > 6000 {
			head = head[:6000]
		}
		if frontmatterValue(head, "curriculum") != cid {
			continue
		}
		kind := frontmatterValue(head, "kind")
		if kind == "" {
			kind = "lesson"
		}
		if kind == "review" && !includeReviews {
			continue
		}
		id := frontmatterValue(head, "id")
		if id == "" {
			id = strings.TrimSuffix(entry.Name(), ".md")
		}
		section := ""
		if m := regexp.MustCompile(`(?m)^章节：\s*(.*?)\s*$`).FindStringSubmatch(head); len(m) > 1 && strings.TrimSpace(m[1]) != "未设置" {
			section = strings.TrimSpace(m[1])
		}
		status := frontmatterValue(head, "termination")
		if status == "" {
			status = "completed"
		}
		rows = append(rows, map[string]any{"id": id, "course": cid, "kind": kind, "status": status,
			"at": sessionTime(id), "section": section, "active": false})
		seen[id] = true
	}
	st := learnJSON(s.Vault, "status")
	for _, raw := range []any{st["active_session"], st["suspended_session"]} {
		a := mmap(raw)
		id, kind := mstr(a, "id"), mstr(a, "kind")
		if id == "" || mstr(a, "curriculum") != cid || seen[id] || (kind == "review" && !includeReviews) {
			continue
		}
		rows = append(rows, map[string]any{"id": id, "course": cid, "kind": kind, "status": "active",
			"at": mstr(a, "started_at"), "section": mstr(a, "starting_chapter"), "active": true})
	}
	sort.Slice(rows, func(i, j int) bool { return mstr(rows[i], "at", "id") > mstr(rows[j], "at", "id") })
	if limit > 0 && len(rows) > limit {
		rows = rows[:limit]
	}
	return rows
}

// SessionDetail returns one immutable transcript by id.
func (s *Server) SessionDetail(id string) map[string]any {
	if !regexp.MustCompile(`^session-[A-Za-z0-9.-]+$`).MatchString(id) {
		return nil
	}
	d := learnJSON(s.Vault, "session", "turns", "--session", id)
	if d == nil {
		return nil
	}
	var summary map[string]any
	path := filepath.Join(s.Vault, "Sessions", id+".md")
	if b, err := os.ReadFile(path); err == nil {
		course := frontmatterValue(string(b), "curriculum")
		for _, row := range s.CourseSessions(course, true, 0) {
			if mstr(row, "id") == id {
				summary = row
				break
			}
		}
	}
	if summary == nil {
		st := learnJSON(s.Vault, "status")
		for _, raw := range []any{st["active_session"], st["suspended_session"]} {
			a := mmap(raw)
			if mstr(a, "id") == id {
				summary = map[string]any{"id": id, "course": mstr(a, "curriculum"), "kind": mstr(a, "kind"),
					"status": "active", "at": mstr(a, "started_at"), "section": mstr(a, "starting_chapter"), "active": true}
				break
			}
		}
	}
	turns := []map[string]any{}
	for _, raw := range asSlice(d["turns"]) {
		t := mmap(raw)
		role := mstr(t, "role")
		if role == "user" || role == "assistant" {
			turns = append(turns, map[string]any{"id": t["id"], "role": role, "text": mstr(t, "text"), "at": t["at"]})
		}
	}
	if summary == nil {
		summary = map[string]any{"id": id, "at": sessionTime(id)}
	}
	return map[string]any{"session": summary, "turns": turns}
}

func (s *Server) CourseOutline(cid string) map[string]any {
	d := learnJSON(s.Vault, "curriculum", "outline", "show", cid)
	return map[string]any{"course": cid, "status": d["status"], "type": d["type"], "nodes": orEmpty(d["nodes"])}
}

func (s *Server) CourseConcepts(cid string) map[string]any {
	rows := []any{}
	for _, raw := range asSlice(learnJSON(s.Vault, "state")["concepts"]) {
		c := mmap(raw)
		for _, cur := range asSlice(c["curricula"]) {
			if cur == cid {
				rows = append(rows, c)
				break
			}
		}
	}
	return map[string]any{"course": cid, "concepts": rows}
}

func (s *Server) CourseSources(cid string, outline map[string]any) map[string]any {
	ids := map[string]bool{cid: true}
	for _, raw := range asSlice(outline["nodes"]) {
		for _, res := range asSlice(mmap(raw)["resources"]) {
			if id, ok := res.(string); ok {
				ids[id] = true
				continue
			}
			r := mmap(res)
			id := mstr(r, "resource")
			if id == "" {
				id = mstr(mmap(r["locator"]), "resource")
			}
			ids[id] = id != ""
		}
	}
	rows := []any{}
	for _, raw := range asSlice(learnJSON(s.Vault, "source", "list")["sources"]) {
		if ids[mstr(mmap(raw), "id")] {
			rows = append(rows, raw)
		}
	}
	return map[string]any{"course": cid, "sources": rows}
}

func (s *Server) CourseMoments(cid string) map[string]any {
	rows := []any{}
	for _, raw := range asSlice(s.Home()["moments"]) {
		if mstr(mmap(raw), "course") == cid {
			rows = append(rows, raw)
		}
	}
	return map[string]any{"course": cid, "moments": rows}
}

// CourseDetail is a read-only snapshot and never activates the course.
func (s *Server) courseRow(cid string) map[string]any {
	var course map[string]any
	for _, raw := range learnJSONList(s.Vault, "curriculum", "list") {
		if mstr(mmap(raw), "id") == cid {
			course = mmap(raw)
			break
		}
	}
	return course
}

func (s *Server) CourseExists(cid string) bool { return s.courseRow(cid) != nil }

func (s *Server) CourseDetail(cid string) map[string]any {
	course := s.courseRow(cid)
	if course == nil {
		return nil
	}
	outline := s.CourseOutline(cid)
	nodes, done := asSlice(outline["nodes"]), 0
	for _, raw := range nodes {
		if mstr(mmap(raw), "status") == "completed" {
			done++
		}
	}
	percent := 0
	if len(nodes) > 0 {
		percent = done * 100 / len(nodes)
	}
	position := map[string]any{}
	if b, err := os.ReadFile(filepath.Join(s.Vault, "Curriculum", cid, "current-position.md")); err == nil {
		for _, key := range []string{"node", "chapter", "section", "current_concept", "last_completed"} {
			position[key] = frontmatterValue(string(b), key)
		}
	}
	goal := mmap(learnJSON(s.Vault, "curriculum", "goal", "show", cid))["goal"]
	sessions := s.CourseSessions(cid, false, 0)
	return map[string]any{"id": cid, "title": mstr(course, "title", "id"), "kind": course["kind"],
		"active": mstr(learnJSON(s.Vault, "status"), "current_learning") == cid, "goal": goal,
		"outline": outline, "position": position, "progress": map[string]any{"completed": done, "total": len(nodes), "percent": percent},
		"concepts": s.CourseConcepts(cid)["concepts"], "sources": s.CourseSources(cid, outline)["sources"],
		"sessions": sessions, "session_count": len(sessions)}
}

// Due is the review page's queue.
func (s *Server) Due() map[string]any {
	r := learnJSON(s.Vault, "review")
	active := mstr(learnJSON(s.Vault, "status"), "current_learning")
	allowed := map[string]bool{}
	for _, raw := range asSlice(learnJSON(s.Vault, "state")["concepts"]) {
		m := mmap(raw)
		for _, cur := range asSlice(m["curricula"]) {
			if cur == active {
				allowed[mstr(m, "id")] = true
			}
		}
	}
	due, upcoming := []any{}, []any{}
	prepared := loadReviewQuestions(s.Vault).Questions
	seen := map[string]bool{}
	for _, d := range asSlice(r["due"]) {
		m := mmap(d)
		if !allowed[mstr(m, "concept")] {
			continue
		}
		label := mstr(m, "label", "concept")
		entry := map[string]any{"concept": m["concept"], "label": label, "due": m["due"], "practice": false}
		if q, ok := prepared[mstr(m, "concept")]; ok && q.Recorded {
			entry["ask"] = q.Ask
		}
		due = append(due, entry)
		seen[mstr(m, "concept")] = true
	}
	for _, d := range asSlice(r["upcoming"]) {
		m := mmap(d)
		if !allowed[mstr(m, "concept")] {
			continue
		}
		label := mstr(m, "label", "concept")
		entry := map[string]any{"concept": m["concept"], "label": label, "due": m["due"], "practice": false}
		if q, ok := prepared[mstr(m, "concept")]; ok && q.Recorded {
			entry["ask"] = q.Ask
		}
		upcoming = append(upcoming, entry)
		if len(due) < 6 && !seen[mstr(m, "concept")] {
			practice := map[string]any{"concept": m["concept"], "label": label, "due": m["due"], "practice": true}
			if q, ok := prepared[mstr(m, "concept")]; ok && q.Recorded {
				practice["ask"] = q.Ask
			}
			due = append(due, practice)
			seen[mstr(m, "concept")] = true
		}
	}
	return map[string]any{"due": due, "upcoming": upcoming, "today": r["today"]}
}

// ReviewCurrent returns generated, unanswered questions for the active review
// session. It never calls the model.
func (s *Server) ReviewCurrent() map[string]any {
	set := loadReviewQuestions(s.Vault)
	questions := []ReviewQuestion{}
	for _, q := range set.Questions {
		questions = append(questions, q)
	}
	return map[string]any{"course": set.Course, "session": set.Session, "questions": questions}
}

var stateMap = map[string]string{"stable": "stable", "fragile": "fragile", "developing": "developing"}

// Notes is the notes page's concept list.
func (s *Server) Notes() map[string]any {
	st := learnJSON(s.Vault, "state")
	out := []map[string]any{}
	for _, c := range asSlice(st["concepts"]) {
		m := mmap(c)
		if st8, ok := stateMap[mstr(m, "state")]; ok {
			e := map[string]any{"id": m["id"], "label": m["label"], "state": st8}
			// 列表行要显示小节/摘要/下次复习，搜索要别名/原话/旧认知，从概念详情拼过来
			if d := s.Concept(mstr(m, "id")); d != nil {
				e["section"] = d["section"]
				e["summary"] = d["understanding"]
				e["next"] = d["next"]
				e["aliases"] = d["aliases"]
				e["quotes"] = d["quotes"]
				e["was"] = d["was"]
			}
			out = append(out, e)
		}
	}
	return map[string]any{"concepts": out}
}

// Concept is one concept card.
func (s *Server) Concept(cid string) map[string]any {
	d := learnJSON(s.Vault, "state", "concept", cid)
	if d == nil {
		return nil
	}
	c := mmap(d["concept"])
	quotes := []any{}
	seen := map[string]bool{}
	items := asSlice(c["history"])
	items = append(items, asSlice(d["events"])...)
	for _, it := range items {
		m := mmap(it)
		for _, e := range asSlice(m["evidence"]) {
			ev := mmap(e)
			if q := mstr(ev, "quote"); q != "" && !seen[q] {
				seen[q] = true
				at := mstr(m, "at")
				quotes = append(quotes, []any{q, truncate(at, 10)})
			}
		}
	}
	was := []any{}
	for _, ch := range asSlice(d["cognitive_changes"]) {
		if om := mmap(ch)["old_model"]; om != nil {
			was = append(was, om)
		}
	}
	understanding := "还没有形成记录。"
	if h := asSlice(c["history"]); len(h) > 0 {
		if s8 := mstr(mmap(h[len(h)-1]), "summary"); s8 != "" {
			understanding = s8
		}
	}
	next := "—"
	due := s.Due()
	for _, x := range append(asSlice(due["due"]), asSlice(due["upcoming"])...) {
		if mmap(x)["concept"] == cid {
			next = mstr(mmap(x), "due")
			break
		}
	}
	related := []any{}
	for _, r := range asSlice(c["related"]) {
		related = append(related, mmap(r)["concept"])
	}
	if len(quotes) > 4 {
		quotes = quotes[len(quotes)-4:]
	}
	section := ""
	if refs := asSlice(c["source_refs"]); len(refs) > 0 {
		section = mstr(mmap(refs[len(refs)-1]), "section", "chapter")
	}
	return map[string]any{
		"id": cid, "label": c["label"], "state": stateMap[mstr(d, "state")],
		"understanding": understanding, "quotes": quotes, "was": was,
		"related": related, "next": next, "aliases": orEmpty(c["aliases"]),
		"section": section,
	}
}

// SourceText is the original text behind the current outline entry.
func (s *Server) SourceText() map[string]any {
	res := asSlice(s.Overview()["resources"])
	if len(res) == 0 {
		return map[string]any{"title": "", "text": "这一节没有挂载原文。"}
	}
	loc := mmap(res[0])["locator"]
	l := mmap(loc)
	d := learnJSON(s.Vault, "source", "read", mstr(l, "resource"), mstr(l, "kind"), mstr(l, "value"))
	title := mstr(mmap(res[0]), "title")
	if title == "" {
		title = mstr(l, "resource")
	}
	return map[string]any{"title": title, "text": mstr(mmap(d["content"]), "text")}
}

// latestSession finds the newest finished lesson of a course from the Sessions/
// page front matter (reviews live on their own page).
func (s *Server) latestSession(cur string) string {
	folder := filepath.Join(s.Vault, "Sessions")
	entries, err := os.ReadDir(folder)
	if err != nil {
		return ""
	}
	names := []string{}
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Sort(sort.Reverse(sort.StringSlice(names)))
	kindRe := regexp.MustCompile(`(?m)^kind:\s*review\s*$`)
	curRe := regexp.MustCompile(`(?m)^curriculum:\s*"?` + regexp.QuoteMeta(cur) + `"?\s*$`)
	idRe := regexp.MustCompile(`(?m)^id:\s*(\S+)`)
	for _, name := range names {
		b, err := os.ReadFile(filepath.Join(folder, name))
		if err != nil {
			continue
		}
		head := string(b)
		if len(head) > 2000 {
			head = head[:2000]
		}
		if kindRe.MatchString(head) {
			continue
		}
		if curRe.MatchString(head) {
			if m := idRe.FindStringSubmatch(head); m != nil {
				return m[1]
			}
		}
	}
	return ""
}

// Turns returns the active course's lesson: in progress (also while a review
// suspends it), or else its last finished one.
func (s *Server) Turns() map[string]any {
	st := learnJSON(s.Vault, "status")
	cur := mstr(st, "current_learning")
	active := mmap(st["active_session"])
	if active["kind"] == "review" {
		active = mmap(st["suspended_session"])
	}
	past := true
	var d map[string]any
	sid := ""
	if active != nil && active["curriculum"] == cur {
		sid = mstr(active, "id")
		d = learnJSON(s.Vault, "session", "turns", "--session", sid)
		past = false
	} else if sid = s.latestSession(cur); sid != "" {
		d = learnJSON(s.Vault, "session", "turns", "--session", sid)
	}
	items := []map[string]any{}
	for _, t := range asSlice(d["turns"]) {
		m := mmap(t)
		text := mstr(m, "text")
		items = append(items, map[string]any{"role": m["role"], "text": text})
	}
	if len(items) == 0 {
		// 课前对话（还没 session_start）：回退到 runner 保存的对话，
		// 这样应用重启后聊天内容还在。
		for _, m := range loadHistory(s.Vault, cur, "learn") {
			text, _ := m["content"].(string)
			if text == "" || strings.HasPrefix(text, "[app]") {
				continue
			}
			text = strings.TrimPrefix(text, "[recorded when a session starts] ")
			switch m["role"] {
			case "user", "assistant":
				items = append(items, map[string]any{"role": m["role"], "text": text})
			}
		}
	}
	return map[string]any{"session": sid, "turns": items, "past": past}
}

// CourseState is where a course being built stands: its goal card, and the
// draft outline once it can be confirmed.
func (s *Server) CourseState() map[string]any {
	st := learnJSON(s.Vault, "status")
	status, _ := st["outline"].(string)
	goal := mmap(learnJSON(s.Vault, "curriculum", "goal", "show"))["goal"]
	var draft any
	if status == "draft" {
		draft = learnJSON(s.Vault, "curriculum", "outline", "review")
	}
	return map[string]any{"building": status != "confirmed", "outline_status": status, "goal": goal, "draft": draft}
}

// ---- actions ----

// Chat answers one learner message on the learning page.
func (s *Server) Chat(ctx context.Context, text string, emit Emit) (map[string]any, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.endReview(ctx, emit) // the learner is back on the learning page
	h := loadHistory(s.Vault, "", "learn")
	h = s.learnerSays(h, text)
	reply, h2, _, err := RunAgent(ctx, s.tools, h, emit, true)
	if h2 != nil {
		h = h2
	}
	saveHistory(s.Vault, "", "learn", h)
	if err != nil {
		return nil, err
	}
	return map[string]any{"reply": reply, "course": s.CourseState()}, nil
}

// NewGoal: the learner tapped + and said what they want to learn; a new course
// and its intake begin.
func (s *Server) NewGoal(ctx context.Context, goal string, emit Emit) (map[string]any, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.finishSession(ctx, "the learner started a new course", emit, true); err != nil {
		return nil, err
	}
	s.tools.PendingUser = ""
	h := []Message{
		{"role": "system", "content": SystemPrompt(s.Vault)},
		{"role": "user", "content": "[app] The learner tapped 新建课程 and chose 说说目标; their goal is the next message. Create the course with curriculum_create_goal " +
			"(the goal in their words, a short lowercase id, a short Chinese title), then start the intake with session_start kind baseline " +
			"and ask your first interview question, one question only."},
	}
	h = s.learnerSays(h, goal)
	reply, h2, _, err := RunAgent(ctx, s.tools, h, emit, true)
	s.tools.PendingUser = ""
	if h2 != nil {
		h = h2
	}
	saveHistory(s.Vault, "", "learn", h) // now under the new course
	if err != nil {
		return nil, err
	}
	return map[string]any{"reply": reply, "course": s.CourseState()}, nil
}

// ConfirmOutline: the learner tapped 开始学习 on the draft card; the outline is
// confirmed and the tutor begins the first entry.
func (s *Server) ConfirmOutline(ctx context.Context, emit Emit) (map[string]any, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r := runCLI(s.Vault, "", "curriculum", "outline", "confirm", "--json")
	if !r.OK {
		return map[string]any{"ok": false, "message": truncate(r.Output, 300), "course": s.CourseState()}, nil
	}
	h := loadHistory(s.Vault, "", "learn")
	h = append(h, Message{"role": "user", "content": "[app] The learner reviewed the draft card and tapped 开始学习, so the app confirmed the outline. If the intake baseline is still open, " +
		"end it now with its assessment (goal_card included). Then set the position to the first entry you will teach with position_set, " +
		"start a lesson session on it and begin teaching it in one short message."})
	reply, h2, _, err := RunAgent(ctx, s.tools, h, emit, true)
	if h2 != nil {
		h = h2
	}
	saveHistory(s.Vault, "", "learn", h)
	if err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "reply": reply, "course": s.CourseState()}, nil
}

var outcomeLabel = map[string]string{"recalled": "想起", "partial": "部分想起", "forgotten": "忘记"}

// ReviewAsk opens (or continues) the review session and asks one retrieval
// question about the given concept.
func (s *Server) ReviewAsk(ctx context.Context, cid, label string, emit Emit) (map[string]any, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.startReview(); err != nil {
		return nil, err
	}
	if q, ok := loadReviewQuestions(s.Vault).Questions[cid]; ok && q.Ask != "" {
		if !q.Recorded {
			h := loadHistory(s.Vault, "", "review")
			h = append(h,
				Message{"role": "user", "content": "[app] Present the prepared retrieval question about " + label + " now."},
				Message{"role": "assistant", "content": q.Ask})
			appendTurn(s.Vault, "assistant", q.Ask)
			saveHistory(s.Vault, "", "review", h)
			q.Recorded = true
			saveReviewQuestion(s.Vault, q)
		}
		return map[string]any{"ask": q.Ask, "prepared": true}, nil
	}
	h := loadHistory(s.Vault, "", "review")
	h = append(h, Message{"role": "user", "content": "[app] The learner opened the review screen. Ask one retrieval question about the concept " + label + " (id " + cid + "), following the review workflow. Reply with the question only, one or two sentences, no greeting."})
	q, h2, _, err := RunAgent(ctx, s.tools, h, emit, true)
	if h2 != nil {
		h = h2
	}
	saveHistory(s.Vault, "", "review", h)
	if err != nil {
		return nil, err
	}
	q = strings.TrimSpace(q)
	saveReviewQuestion(s.Vault, ReviewQuestion{Concept: cid, Label: label, Ask: q, Recorded: true})
	return map[string]any{"ask": q, "prepared": true}, nil
}

// ReviewPrepare generates a question without adding it to the review
// transcript. ReviewAsk records it only when the card is actually shown, so a
// prefetched next question cannot get ahead of the learner's current answer.
func (s *Server) ReviewPrepare(ctx context.Context, cid, label string, emit Emit) (map[string]any, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.startReview(); err != nil {
		return nil, err
	}
	if q, ok := loadReviewQuestions(s.Vault).Questions[cid]; ok && q.Ask != "" {
		return map[string]any{"ask": q.Ask, "prepared": true}, nil
	}
	h := []Message{{"role": "system", "content": SystemPrompt(s.Vault)},
		{"role": "user", "content": "[app] Prepare one retrieval question about the concept " + label + " (id " + cid + "), following the review workflow. Reply with the question only, one or two sentences, no greeting. Do not use tools."}}
	q, _, _, err := RunAgent(ctx, s.tools, h, emit, false)
	if err != nil {
		return nil, err
	}
	q = strings.TrimSpace(q)
	saveReviewQuestion(s.Vault, ReviewQuestion{Concept: cid, Label: label, Ask: q})
	return map[string]any{"ask": q, "prepared": true}, nil
}

// ReviewAnswer judges the learner's answer and records the outcome.
func (s *Server) ReviewAnswer(ctx context.Context, cid, label, answer string, emit Emit) (map[string]any, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !reviewing(s.Vault) {
		return nil, fmt.Errorf("这道题的复习已经结束，请重新出题")
	}
	h := loadHistory(s.Vault, "", "review")
	if answer == "" {
		answer = "（没有作答）"
	}
	h = s.learnerSays(h, answer)
	h = append(h, Message{"role": "user", "content": "[app] That was the learner's answer to the review question about " + label + " (id " + cid + "). Judge it as recalled, partial or forgotten, submit it now with record_checkpoint as review_results (action_turn is your question, evidence is the answer), then reply with brief feedback in at most two sentences. Do not ask a follow-up question here."})
	reply, h2, calls, err := RunAgent(ctx, s.tools, h, emit, true)
	if h2 != nil {
		h = h2
	}
	saveHistory(s.Vault, "", "review", h)
	if err != nil {
		return nil, err
	}
	forgetReviewQuestion(s.Vault, cid)
	outcome := ""
	for _, call := range calls {
		if call.Name == "record_checkpoint" && call.OK {
			rec := mmap(call.Args["record"])
			if rec == nil {
				json.Unmarshal([]byte(str(call.Args, "record")), &rec)
			}
			for _, r := range asSlice(rec["review_results"]) {
				if mmap(r)["concept"] == cid {
					outcome = mstr(mmap(r), "outcome")
				}
			}
		}
	}
	next := "—"
	for _, x := range asSlice(s.Due()["upcoming"]) {
		if mmap(x)["concept"] == cid {
			next = mstr(mmap(x), "due")
			break
		}
	}
	verdict, ok := outcomeLabel[outcome]
	if !ok {
		verdict = "部分想起"
	}
	return map[string]any{"verdict": verdict, "recorded": outcome != "", "feedback": strings.TrimSpace(reply), "next": next}, nil
}

// finishSession ends the session; the course's next session starts a fresh
// conversation. Normally the tutor first records what is left. quick (switching
// or adding a course) skips the tutor: a wrap-up can take the model minutes,
// and records already checkpointed stay; the conversation itself is always kept.
func (s *Server) finishSession(ctx context.Context, why string, emit Emit, quick bool) (*string, error) {
	s.endReview(ctx, emit)
	sess := activeSession(s.Vault)
	if sess == nil {
		return nil, nil
	}
	cur := mstr(sess, "curriculum")
	if quick {
		r := runCLI(s.Vault, "", "session", "end", "--no-analysis", "--reason", why, "--json")
		if !r.OK { // e.g. a baseline cannot end without an assessment
			runCLI(s.Vault, "", "session", "abort", "--reason", why, "--json")
		}
		forgetHistory(s.Vault, cur, "learn")
		empty := ""
		return &empty, nil
	}
	h := loadHistory(s.Vault, cur, "learn")
	h = append(h, Message{"role": "user", "content": "[app] " + why + " Wrap up quickly: submit a short final record only if something clearly unrecorded remains, and end the session with session_end. " +
		"If this is an intake or baseline that is not finished, end it with a reason instead of writing an assessment. Then reply with a one-sentence goodbye."})
	reply, h2, _, err := RunAgent(ctx, s.tools, h, emit, false)
	if h2 != nil {
		h = h2
	}
	if errors.Is(err, ErrCancelled) {
		saveHistory(s.Vault, cur, "learn", h)
		return nil, err
	}
	if err != nil {
		reply = "" // a slow or failing model must not keep the learner from moving on
	}
	saveHistory(s.Vault, cur, "learn", h)
	if activeSession(s.Vault) != nil {
		runCLI(s.Vault, "", "session", "end", "--no-analysis", "--reason", "learner left the app", "--json")
	}
	if activeSession(s.Vault) != nil {
		// e.g. a baseline refuses to end without an assessment: keep the
		// conversation, drop the session
		runCLI(s.Vault, "", "session", "abort", "--reason", "learner left before the session could be wrapped up", "--json")
	}
	forgetHistory(s.Vault, cur, "learn")
	return &reply, nil
}

// ReviewEnd ends an open review session.
func (s *Server) ReviewEnd(ctx context.Context, emit Emit) (map[string]any, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return map[string]any{"ended": s.endReview(ctx, emit)}, nil
}

// EndSession ends the learning session because the learner is leaving.
func (s *Server) EndSession(ctx context.Context, emit Emit) (map[string]any, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	reply, err := s.finishSession(ctx, "The learner is leaving.", emit, false)
	if err != nil {
		return nil, err
	}
	out := map[string]any{"ended": reply != nil, "reply": ""}
	if reply != nil {
		out["reply"] = *reply
	}
	return out, nil
}

// Activate switches course; a lesson in progress belongs to the old course, so
// it is wrapped up first.
func (s *Server) Activate(ctx context.Context, cid string, emit Emit) (map[string]any, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.endReview(ctx, emit)
	if sess := activeSession(s.Vault); sess != nil && sess["curriculum"] != cid {
		if err := emit(map[string]any{"type": "tool", "name": "session_end"}); err != nil {
			return nil, err
		}
		if _, err := s.finishSession(ctx, "the learner switched to another course", emit, true); err != nil {
			return nil, err
		}
	}
	r := runCLI(s.Vault, "", "curriculum", "activate", cid)
	out := s.Overview()
	out["ok"] = r.OK
	out["message"] = truncate(r.Output, 300)
	return out, nil
}
