package agent

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Model is one OpenAI-compatible chat/completions endpoint with tool calling.
type Model struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	BaseURL string `json:"base_url"`
	ModelID string `json:"model"`
	Key     string `json:"key,omitempty"`
	KeyFile string `json:"key_file,omitempty"`
}

// KeyFileDefault holds the DeepSeek key; it is read by this process and never
// sent to any frontend.
var KeyFileDefault = filepath.Join(os.Getenv("HOME"), ".config", "deepseek", "key")

// ModelsFile keeps models added in the app (mode 600, never leaves the machine).
var ModelsFile = filepath.Join(os.Getenv("HOME"), ".config", "learning-os-proto", "models.json")

// builtin models resolve their key file lazily so mobile Configure() can
// redirect KeyFileDefault after package init.
var builtinModels = []Model{
	{ID: "ds-pro", Name: "DeepSeek v4-pro", BaseURL: "https://api.deepseek.com", ModelID: "deepseek-v4-pro"},
	{ID: "ds-flash", Name: "DeepSeek flash", BaseURL: "https://api.deepseek.com", ModelID: "deepseek-flash"},
}

// CallLimit is how long one model call may take in total, however steadily it
// trickles.
const CallLimit = 240 * time.Second

type modelConf struct {
	Active string  `json:"active"`
	Custom []Model `json:"custom"`
}

func loadModelConf() modelConf {
	var d modelConf
	if b, err := os.ReadFile(ModelsFile); err == nil {
		_ = json.Unmarshal(b, &d)
	}
	if d.Active == "" {
		d.Active = "ds-pro"
	}
	return d
}

func saveModelConf(d modelConf) error {
	if err := os.MkdirAll(filepath.Dir(ModelsFile), 0o755); err != nil {
		return err
	}
	b, _ := json.MarshalIndent(d, "", " ")
	return os.WriteFile(ModelsFile, b, 0o600)
}

func allModels() []Model {
	return append(append([]Model{}, builtinModels...), loadModelConf().Custom...)
}

// ActiveModel returns the configured tutor, defaulting to the first builtin.
func ActiveModel() Model {
	conf := loadModelConf()
	for _, m := range allModels() {
		if m.ID == conf.Active {
			return m
		}
	}
	return builtinModels[0]
}

func (m Model) key() (string, error) {
	if m.Key != "" {
		return m.Key, nil
	}
	kf := m.KeyFile
	if kf == "" {
		kf = KeyFileDefault
	}
	b, err := os.ReadFile(kf)
	return strings.TrimSpace(string(b)), err
}

// PublicModels is the /api/models payload: no keys, ever.
func PublicModels() map[string]any {
	active := ActiveModel()
	list := []map[string]any{}
	for _, m := range allModels() {
		builtin := m.Key == ""
		list = append(list, map[string]any{
			"id": m.ID, "name": m.Name, "base_url": m.BaseURL, "model": m.ModelID, "builtin": builtin,
		})
	}
	return map[string]any{"active": active.ID, "models": list}
}

// CheckModel makes a tiny call to prove a newly added model answers, before it
// is saved.
func CheckModel(m Model) error {
	body, _ := json.Marshal(map[string]any{
		"model":      m.ModelID,
		"messages":   []map[string]any{{"role": "user", "content": "ping"}},
		"max_tokens": 8,
	})
	req, err := http.NewRequest("POST", strings.TrimRight(m.BaseURL, "/")+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+m.Key)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("HTTP %s", resp.Status)
	}
	var sink map[string]any
	return json.NewDecoder(resp.Body).Decode(&sink)
}

// Message is one chat message as stored in the conversation history.
type Message = map[string]any

// ChatAnswer is one completed model turn.
type ChatAnswer struct {
	Content   string
	Reasoning string
	ToolCalls []ToolCall
	Usage     map[string]any
}

// ToolCall is the model's request to run one tool.
type ToolCall struct {
	ID        string
	Name      string
	Arguments string
}

// chatCompletion makes one streamed model call. Streaming lets us enforce a
// total time limit (a keep-alive trickle never trips a socket timeout) and
// notice at once when the page stops listening: beat is invoked every couple
// of seconds, and a beat error cancels the call.
func chatCompletion(ctx context.Context, m Model, messages []Message, tools []map[string]any, beat func() error) (ChatAnswer, error) {
	key, err := m.key()
	if err != nil {
		return ChatAnswer{}, err
	}
	body, _ := json.Marshal(map[string]any{
		"model": m.ModelID, "messages": messages, "tools": tools, "stream": true,
	})
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "POST", strings.TrimRight(m.BaseURL, "/")+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return ChatAnswer{}, err
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", "application/json")
	resp, err := (&http.Client{}).Do(req) // no client timeout: CallLimit governs the whole call
	if err != nil {
		return ChatAnswer{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		var sink bytes.Buffer
		_, _ = sink.ReadFrom(resp.Body)
		return ChatAnswer{}, fmt.Errorf("%s: HTTP %s: %s", m.Name, resp.Status, sink.String()[:min(300, sink.Len())])
	}

	start, lastBeat := time.Now(), time.Now()
	var content, reasoning strings.Builder
	slots := map[int]*ToolCall{}
	var usage map[string]any
	reader := bufio.NewReaderSize(resp.Body, 1<<16)
	for {
		if time.Since(start) > CallLimit {
			return ChatAnswer{}, fmt.Errorf("%s 超过 %d 分钟还没答完，已中断", m.Name, int(CallLimit.Minutes()))
		}
		if time.Since(lastBeat) > 2*time.Second && beat != nil {
			if err := beat(); err != nil {
				return ChatAnswer{}, err
			}
			lastBeat = time.Now()
		}
		line, err := reader.ReadBytes('\n')
		if len(line) > 0 {
			if done, err := consumeLine(line, &content, &reasoning, slots, &usage); err != nil {
				return ChatAnswer{}, err
			} else if done {
				break
			}
		}
		if err != nil {
			break // the stream ends on EOF or a cancelled context
		}
	}
	answer := ChatAnswer{Content: content.String(), Reasoning: reasoning.String(), Usage: usage}
	for i := 0; ; i++ {
		tc, ok := slots[i]
		if !ok {
			break
		}
		answer.ToolCalls = append(answer.ToolCalls, *tc)
	}
	return answer, nil
}

func asSlice(v any) []any {
	s, _ := v.([]any)
	return s
}

// consumeLine folds one SSE line into the answer; it reports "[DONE]".
func consumeLine(line []byte, content, reasoning *strings.Builder, slots map[int]*ToolCall, usage *map[string]any) (bool, error) {
	s := strings.TrimSpace(string(line))
	if !strings.HasPrefix(s, "data:") {
		return false, nil
	}
	data := strings.TrimSpace(s[len("data:"):])
	if data == "[DONE]" {
		return true, nil
	}
	var chunk map[string]any
	if json.Unmarshal([]byte(data), &chunk) != nil {
		return false, nil
	}
	if u, ok := chunk["usage"].(map[string]any); ok {
		*usage = u
	}
	for _, c := range asSlice(chunk["choices"]) {
		choice, _ := c.(map[string]any)
		delta, _ := choice["delta"].(map[string]any)
		if s, ok := delta["content"].(string); ok {
			content.WriteString(s)
		}
		if s, ok := delta["reasoning_content"].(string); ok {
			reasoning.WriteString(s)
		}
		for _, t := range asSlice(delta["tool_calls"]) {
			tc, _ := t.(map[string]any)
			idx := 0
			if f, ok := tc["index"].(float64); ok {
				idx = int(f)
			}
			slot := slots[idx]
			if slot == nil {
				slot = &ToolCall{}
				slots[idx] = slot
			}
			if id, ok := tc["id"].(string); ok && id != "" {
				slot.ID = id
			}
			fn, _ := tc["function"].(map[string]any)
			if s, ok := fn["name"].(string); ok {
				slot.Name += s
			}
			if s, ok := fn["arguments"].(string); ok {
				slot.Arguments += s
			}
		}
	}
	return false, nil
}
