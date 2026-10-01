// Package learning is the gomobile facade over the learning core and the agent
// driver. Everything crosses the language boundary as JSON strings — the same
// payloads the web API serves — so Android and iOS share one contract.
//
// Reads return the JSON directly. Actions (Chat, NewGoal, …) block until the
// tutor is done and return the done payload; progress events meanwhile go to
// the EventHandler. CancelActive aborts the running action.
package learning

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/hedykan/learning-system/internal/agent"
)

// EventHandler receives progress events ({"type":"alive"|"tool"|...}) while an
// action runs. The final result is the action's return value instead.
type EventHandler interface {
	OnEvent(eventJSON string)
}

// Tutor is one open vault with its agent driver.
type Tutor struct {
	srv    *agent.Server
	mu     sync.Mutex
	cancel context.CancelFunc
}

// Configure points the model configuration at the app's private storage; call
// once before OpenVault (Android has no usable HOME).
func Configure(configDir string) {
	agent.KeyFileDefault = configDir + "/deepseek-key"
	agent.ModelsFile = configDir + "/models.json"
}

// OpenVault opens an existing vault.
func OpenVault(path string) (*Tutor, error) {
	return &Tutor{srv: agent.NewServer(path)}, nil
}

// InitVault initializes a new vault at path and opens it.
func InitVault(path string) (*Tutor, error) {
	if out, ok := agent.InitVault(path); !ok {
		return nil, &Error{Message: out}
	}
	return OpenVault(path)
}

// Error is a failed vault operation.
type Error struct{ Message string }

func (e *Error) Error() string { return e.Message }

func toJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

// emitter forwards driver events to the app.
func (t *Tutor) emitter(h EventHandler) agent.Emit {
	return func(ev map[string]any) error {
		if h != nil {
			h.OnEvent(toJSON(ev))
		}
		return nil
	}
}

// ctx registers the action's context for CancelActive.
func (t *Tutor) ctx() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	t.mu.Lock()
	t.cancel = cancel
	t.mu.Unlock()
	return ctx
}

// CancelActive aborts the running action, if any.
func (t *Tutor) CancelActive() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.cancel != nil {
		t.cancel()
	}
}

// ---- read models (the web API's GET endpoints) ----

func (t *Tutor) Ping() string    { return toJSON(map[string]any{"ok": true, "model": agent.ActiveModel().Name}) }
func (t *Tutor) Models() string  { return toJSON(agent.PublicModels()) }
func (t *Tutor) Overview() string { return toJSON(t.srv.Overview()) }
func (t *Tutor) Courses() string { return toJSON(t.srv.Courses()) }
func (t *Tutor) Review() string  { return toJSON(t.srv.Due()) }
func (t *Tutor) Notes() string   { return toJSON(t.srv.Notes()) }
func (t *Tutor) Turns() string   { return toJSON(t.srv.Turns()) }
func (t *Tutor) Course() string  { return toJSON(t.srv.CourseState()) }
func (t *Tutor) Source() string  { return toJSON(t.srv.SourceText()) }

func (t *Tutor) Concept(id string) string {
	d := t.srv.Concept(id)
	if d == nil {
		return `{"error":"not found"}`
	}
	return toJSON(d)
}

// ---- actions (the web API's SSE endpoints; results are returned, progress via handler) ----

func (t *Tutor) Chat(message string, h EventHandler) string {
	return t.result(func(ctx context.Context, emit agent.Emit) (map[string]any, error) {
		return t.srv.Chat(ctx, message, emit)
	}, h)
}

func (t *Tutor) NewGoal(goal string, h EventHandler) string {
	return t.result(func(ctx context.Context, emit agent.Emit) (map[string]any, error) {
		return t.srv.NewGoal(ctx, goal, emit)
	}, h)
}

func (t *Tutor) ConfirmOutline(h EventHandler) string {
	return t.result(func(ctx context.Context, emit agent.Emit) (map[string]any, error) {
		return t.srv.ConfirmOutline(ctx, emit)
	}, h)
}

func (t *Tutor) ReviewAsk(concept, label string, h EventHandler) string {
	return t.result(func(ctx context.Context, emit agent.Emit) (map[string]any, error) {
		return t.srv.ReviewAsk(ctx, concept, label, emit)
	}, h)
}

func (t *Tutor) ReviewAnswer(concept, label, answer string, h EventHandler) string {
	return t.result(func(ctx context.Context, emit agent.Emit) (map[string]any, error) {
		return t.srv.ReviewAnswer(ctx, concept, label, answer, emit)
	}, h)
}

func (t *Tutor) ReviewEnd(h EventHandler) string {
	return t.result(func(ctx context.Context, emit agent.Emit) (map[string]any, error) {
		return t.srv.ReviewEnd(ctx, emit)
	}, h)
}

func (t *Tutor) EndSession(h EventHandler) string {
	return t.result(func(ctx context.Context, emit agent.Emit) (map[string]any, error) {
		return t.srv.EndSession(ctx, emit)
	}, h)
}

func (t *Tutor) ActivateCourse(id string, h EventHandler) string {
	return t.result(func(ctx context.Context, emit agent.Emit) (map[string]any, error) {
		return t.srv.Activate(ctx, id, emit)
	}, h)
}

func (t *Tutor) result(f func(context.Context, agent.Emit) (map[string]any, error), h EventHandler) string {
	out, err := f(t.ctx(), t.emitter(h))
	if err != nil {
		return toJSON(map[string]any{"type": "error", "message": err.Error()})
	}
	out["type"] = "done"
	return toJSON(out)
}

// ---- tutor models ----

func (t *Tutor) SetModel(id string) { agent.SetActiveModel(id) }

// AddModel validates and saves a custom model; returns "" on success or the
// error message.
func (t *Tutor) AddModel(name, baseURL, modelID, key string) string {
	return agent.AddModel(name, baseURL, modelID, key)
}

func (t *Tutor) RemoveModel(id string) { agent.RemoveModel(id) }
