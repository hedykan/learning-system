package curriculum

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/hedykan/learning-system/internal/web"
)

// SandboxAdvice explains the usual reason a fetch cannot connect.
const SandboxAdvice = "the Runtime could not connect. In an Agent sandbox (Codex blocks network access by default) either set `[sandbox_workspace_write] network_access = true` in the Vault's .codex/config.toml, or ask the learner to run this command in a normal terminal"

// snapshotInto fetches a web snapshot into Sources/<id>/original/snapshot-NNNN.
func snapshotInto(root, id string, spec FetchSpec, n int, now time.Time) (Revision, int, error) {
	if now.IsZero() {
		now = time.Now()
	}
	name := fmt.Sprintf("snapshot-%04d", n)
	dir := filepath.Join(root, "Sources", id, "original", name)
	res, err := web.Snapshot(web.Options{URL: spec.URL, Sitemap: spec.Sitemap, Prefix: spec.Prefix, MaxPages: spec.MaxPages}, dir)
	if err != nil {
		os.RemoveAll(dir)
		if errors.Is(err, web.ErrNetwork) {
			return Revision{}, 0, fmt.Errorf("%v: %s", err, SandboxAdvice)
		}
		return Revision{}, 0, err
	}
	return Revision{Snapshot: name, SHA256: res.SHA256, CreatedAt: now.UTC().Format(time.RFC3339)}, len(res.Pages), nil
}

// refreshWeb takes a new snapshot; an identical one is discarded.
func refreshWeb(root, id string, ref SourceRef, now time.Time) (RefreshResult, error) {
	var spec *FetchSpec
	var count int
	if m, err := LoadManifest(root, id); err == nil {
		spec, count = m.Fetch, len(m.Revisions)
	} else if r, err := LoadResource(root, id); err == nil {
		spec, count = r.Fetch, len(r.Revisions)
	}
	if spec == nil {
		return RefreshResult{}, fmt.Errorf("%s has no recorded fetch settings", id)
	}
	rev, _, err := snapshotInto(root, id, *spec, count+1, now)
	if err != nil {
		return RefreshResult{}, err
	}
	res, err := updateRevisions(root, id, ref.Primary, rev, func(a, b Revision) bool { return a.SHA256 == b.SHA256 })
	if err == nil && !res.Changed {
		os.RemoveAll(filepath.Join(root, "Sources", id, "original", rev.Snapshot))
	}
	return res, err
}
