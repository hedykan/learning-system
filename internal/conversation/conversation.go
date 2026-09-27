// Package conversation parses append-only Raw Conversation files and assigns
// stable turn IDs. Because conversations are append-only, a turn's ordinal is
// its identity; v0.1.2 also writes that ID into the heading and a block anchor.
package conversation

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

// Turn is one raw conversation entry.
type Turn struct {
	ID        string `json:"turn"`
	Ordinal   int    `json:"ordinal"`
	Role      string `json:"role"`
	Timestamp string `json:"timestamp"`
	Text      string `json:"text"`
	Anchored  bool   `json:"anchored"`
}

// Conversation is a parsed Raw Conversation file.
type Conversation struct {
	SessionID       string
	Curriculum      string
	Kind            string
	BaselineSkipped bool
	Turns           []Turn
}

var turnIDPattern = regexp.MustCompile(`^t[0-9]{4,}$`)
var anchorPattern = regexp.MustCompile(`^\^t[0-9]{4,}$`)

// TurnID formats the stable ID for a 1-based ordinal.
func TurnID(ordinal int) string { return fmt.Sprintf("t%04d", ordinal) }

// Ordinal parses a turn ID back into its 1-based ordinal.
func Ordinal(id string) (int, bool) {
	if !turnIDPattern.MatchString(id) {
		return 0, false
	}
	var n int
	if _, err := fmt.Sscanf(id, "t%d", &n); err != nil || n < 1 {
		return 0, false
	}
	return n, true
}

// Parse reads conversation text. It rejects files whose explicit turn IDs do
// not match their ordinal, because every evidence reference relies on them.
func Parse(text string) (*Conversation, error) {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	conv := &Conversation{}
	body := text
	if strings.HasPrefix(text, "---\n") {
		rest := strings.TrimPrefix(text, "---\n")
		if end := strings.Index(rest, "\n---\n"); end >= 0 {
			parseFrontmatter(rest[:end], conv)
			body = rest[end+len("\n---\n"):]
		}
	}
	var current *Turn
	var lines []string
	flush := func() {
		if current == nil {
			return
		}
		current.Text = strings.TrimSpace(strings.Join(lines, "\n"))
		conv.Turns = append(conv.Turns, *current)
		current, lines = nil, nil
	}
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(line, "## ") {
			flush()
			parts := strings.Split(strings.TrimPrefix(line, "## "), " — ")
			ordinal := len(conv.Turns) + 1
			turn := &Turn{ID: TurnID(ordinal), Ordinal: ordinal}
			switch len(parts) {
			case 2:
				turn.Timestamp, turn.Role = strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
			case 3:
				explicit := strings.TrimSpace(parts[0])
				if explicit != turn.ID {
					return nil, fmt.Errorf("conversation turn %d is labelled %q; the conversation is damaged", ordinal, explicit)
				}
				turn.Timestamp, turn.Role = strings.TrimSpace(parts[1]), strings.TrimSpace(parts[2])
			default:
				return nil, fmt.Errorf("conversation turn %d has an unrecognized heading", ordinal)
			}
			current = turn
			continue
		}
		if current == nil {
			continue
		}
		switch {
		case anchorPattern.MatchString(trimmed):
			if trimmed[1:] != current.ID {
				return nil, fmt.Errorf("conversation turn %s has mismatched anchor %q", current.ID, trimmed)
			}
			current.Anchored = true
		case trimmed == ">":
			lines = append(lines, "")
		case strings.HasPrefix(trimmed, "> "):
			lines = append(lines, strings.TrimPrefix(trimmed, "> "))
		}
	}
	flush()
	return conv, nil
}

func parseFrontmatter(frontmatter string, conv *Conversation) {
	for _, line := range strings.Split(frontmatter, "\n") {
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		value = strings.Trim(strings.TrimSpace(value), `"`)
		switch strings.TrimSpace(key) {
		case "id":
			conv.SessionID = value
		case "curriculum":
			conv.Curriculum = value
		case "kind":
			conv.Kind = value
		case "baseline_skipped":
			conv.BaselineSkipped = value == "true"
		}
	}
}

// Load parses a conversation file.
func Load(path string) (*Conversation, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read conversation: %w", err)
	}
	return Parse(string(data))
}

// Path returns the Vault path of a session's conversation.
func Path(root, sessionID string) string {
	return filepath.Join(root, "Conversations", sessionID+".md")
}

// BaselineSkipped reports whether any conversation of a curriculum was
// started with an explicit --skip-baseline.
func BaselineSkipped(root, curriculum string) (bool, error) {
	entries, err := os.ReadDir(filepath.Join(root, "Conversations"))
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("read conversations: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		conv, err := Load(filepath.Join(root, "Conversations", entry.Name()))
		if err != nil {
			continue
		}
		if conv.Curriculum == curriculum && conv.BaselineSkipped {
			return true, nil
		}
	}
	return false, nil
}

// Turn returns the turn with the given ID.
func (c *Conversation) Turn(id string) (Turn, bool) {
	n, ok := Ordinal(id)
	if !ok || n > len(c.Turns) {
		return Turn{}, false
	}
	return c.Turns[n-1], true
}

// RoleText concatenates every turn of one role, one line per turn line.
func (c *Conversation) RoleText(role string) string {
	var b strings.Builder
	for _, turn := range c.Turns {
		if turn.Role == role {
			b.WriteString(turn.Text)
			b.WriteByte('\n')
		}
	}
	return b.String()
}

// FormatEntry renders a new v0.1.2 turn entry with its heading ID and anchor.
func FormatEntry(ordinal int, role, content string, now time.Time) string {
	id := TurnID(ordinal)
	quoted := "> " + strings.ReplaceAll(content, "\n", "\n> ")
	return fmt.Sprintf("\n## %s — %s — %s\n\n%s\n\n^%s\n", id, now.UTC().Format(time.RFC3339Nano), role, quoted, id)
}

// Normalize trims and collapses whitespace for quote comparison.
func Normalize(s string) string { return strings.Join(strings.Fields(s), " ") }

// QuoteMatches reports whether quote is a normalized substring of text with a
// usable length.
func QuoteMatches(text, quote string) error {
	q := Normalize(quote)
	n := utf8.RuneCountInString(q)
	if n < 4 || n > 200 {
		return fmt.Errorf("quote must be 4-200 characters, got %d", n)
	}
	if !strings.Contains(Normalize(text), q) {
		return fmt.Errorf("quote %q is not in the cited turn", q)
	}
	return nil
}
