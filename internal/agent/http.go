package agent

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/hedykan/learning-system/internal/vault"
	"github.com/spf13/cobra"
)

// ServeCommand is wired into the learn CLI by main (agent must not import the
// app package, so the app cannot register it itself). The vault comes from the
// root's persistent --vault flag, falling back to discovery from the cwd.
func ServeCommand() *cobra.Command {
	var port int
	var staticDir string
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Serve the mobile prototypes with the real tutor (local JSON API)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			explicit, _ := cmd.Flags().GetString("vault")
			start := explicit
			if start == "" {
				wd, err := os.Getwd()
				if err != nil {
					return err
				}
				start = wd
			}
			root, err := vault.Discover(start)
			if err != nil {
				return err
			}
			return Serve(root, port, staticDir)
		},
	}
	cmd.Flags().IntVar(&port, "port", 8765, "listen port (127.0.0.1 only)")
	cmd.Flags().StringVar(&staticDir, "static", "", "prototype directory to serve (e.g. prototype/); / redirects to /mobile/island/")
	return cmd
}

// Serve runs the local app backend until the process dies. It listens on
// 127.0.0.1 only; the tutor key never leaves this process.
func Serve(vaultRoot string, port int, staticDir string) error {
	if _, err := os.Stat(KeyFileDefault); err != nil && len(loadModelConf().Custom) == 0 {
		return fmt.Errorf("missing DeepSeek key file and no other tutor model configured")
	}
	tmp := filepath.Join(vaultRoot, ".learning", "tmp")
	if err := os.MkdirAll(filepath.Join(tmp, "app-logs"), 0o755); err != nil {
		return err
	}
	migrateLegacyState(vaultRoot)
	s := NewServer(vaultRoot)
	mux := http.NewServeMux()
	h := &handler{s: s, static: staticDir}

	mux.HandleFunc("GET /api/ping", h.json(func() (any, int) {
		return map[string]any{"ok": true, "model": ActiveModel().Name}, 200
	}))
	mux.HandleFunc("GET /api/models", h.json(func() (any, int) { return PublicModels(), 200 }))
	mux.HandleFunc("GET /api/overview", h.json(func() (any, int) { return s.Overview(), 200 }))
	mux.HandleFunc("GET /api/review", h.json(func() (any, int) { return s.Due(), 200 }))
	mux.HandleFunc("GET /api/notes", h.json(func() (any, int) { return s.Notes(), 200 }))
	mux.HandleFunc("GET /api/source", h.json(func() (any, int) { return s.SourceText(), 200 }))
	mux.HandleFunc("GET /api/turns", h.json(func() (any, int) { return s.Turns(), 200 }))
	mux.HandleFunc("GET /api/courses", h.json(func() (any, int) { return s.Courses(), 200 }))
	mux.HandleFunc("GET /api/course", h.json(func() (any, int) { return s.CourseState(), 200 }))
	mux.HandleFunc("GET /api/concept/{id}", h.jsonWith(func(r *http.Request) (any, int) {
		d := s.Concept(r.PathValue("id"))
		if d == nil {
			return map[string]any{"error": "not found"}, 404
		}
		return d, 200
	}))

	mux.HandleFunc("POST /api/config", h.plainJSON(configModel))
	mux.HandleFunc("POST /api/models", h.plainJSON(addModel))
	mux.HandleFunc("POST /api/models/remove", h.plainJSON(removeModel))

	mux.HandleFunc("POST /api/chat", h.stream(func(r *http.Request, b map[string]any, emit Emit) (map[string]any, error) {
		return s.Chat(r.Context(), str(b, "message"), emit)
	}))
	mux.HandleFunc("POST /api/review/ask", h.stream(func(r *http.Request, b map[string]any, emit Emit) (map[string]any, error) {
		return s.ReviewAsk(r.Context(), str(b, "concept"), orStr(b, "label", str(b, "concept")), emit)
	}))
	mux.HandleFunc("POST /api/review/answer", h.stream(func(r *http.Request, b map[string]any, emit Emit) (map[string]any, error) {
		return s.ReviewAnswer(r.Context(), str(b, "concept"), orStr(b, "label", str(b, "concept")), str(b, "answer"), emit)
	}))
	mux.HandleFunc("POST /api/review/end", h.stream(func(r *http.Request, _ map[string]any, emit Emit) (map[string]any, error) {
		return s.ReviewEnd(r.Context(), emit)
	}))
	mux.HandleFunc("POST /api/course/goal", h.stream(func(r *http.Request, b map[string]any, emit Emit) (map[string]any, error) {
		return s.NewGoal(r.Context(), str(b, "goal"), emit)
	}))
	mux.HandleFunc("POST /api/course/confirm", h.stream(func(r *http.Request, _ map[string]any, emit Emit) (map[string]any, error) {
		return s.ConfirmOutline(r.Context(), emit)
	}))
	mux.HandleFunc("POST /api/session/end", h.stream(func(r *http.Request, _ map[string]any, emit Emit) (map[string]any, error) {
		return s.EndSession(r.Context(), emit)
	}))
	mux.HandleFunc("POST /api/curriculum/activate", h.stream(func(r *http.Request, b map[string]any, emit Emit) (map[string]any, error) {
		return s.Activate(r.Context(), str(b, "id"), emit)
	}))

	mux.HandleFunc("GET /", h.staticOrRedirect())
	fmt.Printf("Learning OS prototype: http://127.0.0.1:%d/  (vault: %s)\n", port, filepath.Base(vaultRoot))
	return http.ListenAndServe(fmt.Sprintf("127.0.0.1:%d", port), mux)
}

func orStr(m map[string]any, key, fallback string) string {
	if v := str(m, key); v != "" {
		return v
	}
	return fallback
}

type handler struct {
	s      *Server
	static string
}

// json answers a GET with a JSON payload.
func (h *handler) json(f func() (any, int)) http.HandlerFunc {
	return h.jsonWith(func(*http.Request) (any, int) { return f() })
}

func (h *handler) jsonWith(f func(*http.Request) (any, int)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data, code := f(r)
		body, _ := json.Marshal(data)
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(code)
		_, _ = w.Write(body)
	}
}

// plainJSON answers a POST that takes and returns plain JSON.
func (h *handler) plainJSON(f func(map[string]any) (any, int)) http.HandlerFunc {
	return h.jsonWith(func(r *http.Request) (any, int) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		return f(body)
	})
}

// stream handles an action POST: progress flows as server-sent events, then
// the result arrives in a done event. A client that stopped listening cancels
// the run through the request context and failed writes.
func (h *handler) stream(f func(*http.Request, map[string]any, Emit) (map[string]any, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		w.WriteHeader(200)
		flush, _ := w.(http.Flusher)
		emit := func(ev map[string]any) error {
			b, _ := json.Marshal(ev)
			if _, err := fmt.Fprintf(w, "data: %s\n\n", b); err != nil {
				return ErrCancelled
			}
			if flush != nil {
				flush.Flush()
			}
			select {
			case <-r.Context().Done():
				return ErrCancelled
			default:
				return nil
			}
		}
		result, err := f(r, body, emit)
		if err != nil {
			if err == ErrCancelled {
				return // nobody is listening any more; what was recorded stays
			}
			_ = emit(map[string]any{"type": "error", "message": truncate(err.Error(), 300)})
			return
		}
		result["type"] = "done"
		_ = emit(result)
	}
}

// staticOrRedirect serves the prototype directory; / jumps straight to the
// island prototype, as the Python devserver did.
func (h *handler) staticOrRedirect() http.HandlerFunc {
	if h.static == "" {
		return func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "no --static directory configured", 404)
		}
	}
	files := http.FileServer(http.Dir(h.static))
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.Redirect(w, r, "/mobile/island/", 302)
			return
		}
		files.ServeHTTP(w, r)
	}
}

var modelIDSanitizer = regexp.MustCompile(`[^a-z0-9]+`)

func configModel(body map[string]any) (any, int) {
	conf := loadModelConf()
	for _, m := range allModels() {
		if m.ID == str(body, "model") {
			conf.Active = m.ID
			_ = saveModelConf(conf)
			break
		}
	}
	return PublicModels(), 200
}

func addModel(body map[string]any) (any, int) {
	m := Model{
		Name:    strings.TrimSpace(str(body, "name")),
		BaseURL: strings.TrimSpace(str(body, "base_url")),
		ModelID: strings.TrimSpace(str(body, "model")),
		Key:     strings.TrimSpace(str(body, "key")),
	}
	if m.Name == "" || m.BaseURL == "" || m.ModelID == "" || m.Key == "" {
		return map[string]any{"ok": false, "message": "名称、接口地址、模型名、密钥都要填"}, 200
	}
	if err := CheckModel(m); err != nil {
		return map[string]any{"ok": false, "message": "连不上这个模型：" + truncate(err.Error(), 160)}, 200
	}
	m.ID = "m-" + strings.Trim(modelIDSanitizer.ReplaceAllString(strings.ToLower(m.Name+"-"+m.ModelID), "-"), "-")
	if len(m.ID) > 42 {
		m.ID = m.ID[:42]
	}
	conf := loadModelConf()
	kept := conf.Custom[:0]
	for _, c := range conf.Custom {
		if c.ID != m.ID {
			kept = append(kept, c)
		}
	}
	conf.Custom = append(kept, m)
	conf.Active = m.ID
	if err := saveModelConf(conf); err != nil {
		return map[string]any{"ok": false, "message": err.Error()}, 200
	}
	out := PublicModels()
	out["ok"] = true
	return out, 200
}

func removeModel(body map[string]any) (any, int) {
	conf := loadModelConf()
	kept := conf.Custom[:0]
	for _, c := range conf.Custom {
		if c.ID != str(body, "id") {
			kept = append(kept, c)
		}
	}
	conf.Custom = kept
	if conf.Active == str(body, "id") {
		conf.Active = builtinModels[0].ID
	}
	_ = saveModelConf(conf)
	return PublicModels(), 200
}
