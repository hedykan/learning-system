package learner

import (
	"encoding/json"
	"fmt"

	"path/filepath"
	"time"

	"github.com/hedykan/learning-system/internal/conversation"
	"github.com/hedykan/learning-system/internal/fsutil"
	"github.com/hedykan/learning-system/internal/record"
)

// VaultResolver reads turns from Vault conversations, caching parsed files.
type VaultResolver struct {
	Root  string
	cache map[string]*conversation.Conversation
}

func NewResolver(root string) *VaultResolver {
	return &VaultResolver{Root: root, cache: map[string]*conversation.Conversation{}}
}

// Conversation loads and caches a session's conversation.
func (r *VaultResolver) Conversation(session string) (*conversation.Conversation, error) {
	if conv, ok := r.cache[session]; ok {
		return conv, nil
	}
	conv, err := conversation.Load(conversation.Path(r.Root, session))
	if err != nil {
		return nil, fmt.Errorf("session %s: %w", session, err)
	}
	r.cache[session] = conv
	return conv, nil
}

// Forget drops a cached conversation, e.g. after new turns were appended.
func (r *VaultResolver) Forget(session string) { delete(r.cache, session) }

func (r *VaultResolver) Turn(session, turn string) (conversation.Turn, error) {
	conv, err := r.Conversation(session)
	if err != nil {
		return conversation.Turn{}, err
	}
	t, ok := conv.Turn(turn)
	if !ok {
		return conversation.Turn{}, fmt.Errorf("turn %s does not exist in session %s", turn, session)
	}
	return t, nil
}

// Replay builds the model from envelopes in order.
func Replay(envs []record.Envelope, resolver TurnResolver) (*Model, error) {
	m := New()
	for _, env := range envs {
		if err := m.Apply(env, resolver); err != nil {
			return nil, fmt.Errorf("replay %s/%04d: %w", env.Session, env.Seq, err)
		}
	}
	m.Generation = record.Generation(envs)
	return m, nil
}

// Load replays every stored record in the Vault.
func Load(root string) (*Model, []record.Envelope, error) {
	envs, err := record.LoadAll(root)
	if err != nil {
		return nil, nil, err
	}
	m, err := Replay(envs, NewResolver(root))
	if err != nil {
		return nil, nil, err
	}
	return m, envs, nil
}

// SubmitResult reports what a submission did.
type SubmitResult struct {
	Status string `json:"status"` // accepted or unchanged
	Record string `json:"record,omitempty"`
	Seq    int    `json:"seq,omitempty"`
	Model  *Model `json:"-"`
}

// Submit validates rec against the replayed model and stores it. Nothing is
// written unless the whole record is valid; identical resubmission is a no-op.
func Submit(root, sessionID, kind string, rec *record.Record, now time.Time) (SubmitResult, error) {
	return SubmitChecked(root, sessionID, kind, rec, now, nil)
}

// Check inspects a submission. before is the model without the new record
// (nil when an identical record already exists); after includes it.
type Check func(before, after *Model) error

// SubmitChecked is Submit with extra checks on the candidate model; the
// record is written only when every check passes.
func SubmitChecked(root, sessionID, kind string, rec *record.Record, now time.Time, check Check) (SubmitResult, error) {
	resolver := NewResolver(root)
	conv, err := resolver.Conversation(sessionID)
	if err != nil {
		return SubmitResult{}, err
	}
	if rec.Curriculum != conv.Curriculum {
		return SubmitResult{}, fmt.Errorf("record curriculum %q does not match session curriculum %q", rec.Curriculum, conv.Curriculum)
	}
	hash, err := record.Hash(rec)
	if err != nil {
		return SubmitResult{}, err
	}
	envs, err := record.LoadAll(root)
	if err != nil {
		return SubmitResult{}, err
	}
	seq := 0
	for _, env := range envs {
		if env.Session != sessionID {
			continue
		}
		if env.Hash == hash {
			m, err := Replay(envs, resolver)
			if err != nil {
				return SubmitResult{}, err
			}
			if check != nil {
				if err := check(nil, m); err != nil {
					return SubmitResult{}, err
				}
			}
			return SubmitResult{Status: "unchanged", Seq: env.Seq, Model: m}, nil
		}
		if env.Seq > seq {
			seq = env.Seq
		}
	}
	env := record.Envelope{Session: sessionID, Seq: seq + 1, Kind: kind, SubmittedAt: now.UTC().Format(time.RFC3339Nano), Hash: hash, Record: *rec}
	candidate := append(append([]record.Envelope{}, envs...), env)
	m, err := Replay(candidate, resolver)
	if err != nil {
		return SubmitResult{}, err
	}
	if check != nil {
		before, err := Replay(envs, resolver)
		if err != nil {
			return SubmitResult{}, err
		}
		if err := check(before, m); err != nil {
			return SubmitResult{}, err
		}
	}
	rel, err := record.Write(root, env)
	if err != nil {
		return SubmitResult{}, err
	}
	return SubmitResult{Status: "accepted", Record: rel, Seq: env.Seq, Model: m}, nil
}

// SaveCache writes the derived model cache, which Git ignores.
func SaveCache(root string, m *Model) error {
	dir := filepath.Join(root, ".learning", "model")
	if _, err := fsutil.WriteFileIfAbsent(filepath.Join(dir, ".gitignore"), []byte("*\n"), 0o644); err != nil {
		return err
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("encode learner model: %w", err)
	}
	return fsutil.WriteFileAtomic(filepath.Join(dir, "learner-model.json"), append(data, '\n'), 0o644)
}

// HasRecords reports whether a session has any accepted record.
func HasRecords(root, sessionID string) (bool, error) {
	envs, err := record.LoadSession(root, sessionID)
	if err != nil {
		return false, err
	}
	return len(envs) > 0, nil
}
