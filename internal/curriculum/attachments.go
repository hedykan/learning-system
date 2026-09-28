package curriculum

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/hedykan/learning-system/internal/fsutil"
	"github.com/hedykan/learning-system/internal/i18n"
	"github.com/hedykan/learning-system/internal/locator"
	"gopkg.in/yaml.v3"
)

// ResourceSet lists the supplementary resources of a curriculum and where
// they attach to its outline (CR-2026-027). The curriculum's own material is
// its implicit primary resource and covers every entry; a curriculum without
// this file simply has no supplementary resources.
type ResourceSet struct {
	Version     int             `yaml:"version" json:"version"`
	Resources   []ResourceEntry `yaml:"resources" json:"resources"`
	Attachments []Attachment    `yaml:"attachments" json:"attachments"`
}

// ResourceEntry is one resource used by a curriculum.
type ResourceEntry struct {
	ID   string `yaml:"id" json:"id"`
	Role string `yaml:"role" json:"role"` // supplementary
}

// Attachment places a resource locator on an outline entry.
type Attachment struct {
	Node    string          `yaml:"node" json:"node"`
	Locator locator.Locator `yaml:"locator" json:"locator"`
}

// NodeResource is one resource position of an outline entry, for learners
// and Agents.
type NodeResource struct {
	Resource string          `json:"resource"`
	Title    string          `json:"title"`
	Kind     string          `json:"kind"`
	Primary  bool            `json:"primary"`
	Locator  locator.Locator `json:"locator"`
	Label    string          `json:"label"`
}

func resourceSetPath(root, id string) string {
	return filepath.Join(root, "Curriculum", id, "resources.yaml")
}

// LoadResourceSet reads a curriculum's resources; missing means none.
func LoadResourceSet(root, id string) (ResourceSet, error) {
	set := ResourceSet{Version: 1}
	data, err := os.ReadFile(resourceSetPath(root, id))
	if os.IsNotExist(err) {
		return set, nil
	}
	if err != nil {
		return set, err
	}
	if err := yaml.Unmarshal(data, &set); err != nil {
		return set, fmt.Errorf("parse resources of %s: %w", id, err)
	}
	return set, nil
}

func saveResourceSet(root, id string, set ResourceSet) error {
	data, err := yaml.Marshal(set)
	if err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(resourceSetPath(root, id), data, 0o644)
}

// Attach places a resource locator on a confirmed outline entry. An empty
// locator resource means the curriculum's own material.
func Attach(root, id, node string, loc locator.Locator) (NodeResource, error) {
	o, err := LoadOutline(root, id)
	if err != nil {
		return NodeResource{}, err
	}
	if o.Status != "confirmed" {
		return NodeResource{}, fmt.Errorf("confirm the outline of %s before attaching resources", id)
	}
	if _, ok := o.Find(node); !ok {
		return NodeResource{}, fmt.Errorf("outline of %s has no entry %q", id, node)
	}
	if loc.Resource == "" {
		loc.Resource = id
	}
	ref, err := Resolve(root, loc.Resource)
	if err != nil {
		return NodeResource{}, err
	}
	if ref.Primary && loc.Resource != id {
		return NodeResource{}, fmt.Errorf("%s is the material of another curriculum; add the file again as a resource with `learn source add` to share it", loc.Resource)
	}
	if err := ref.Validate(loc); err != nil {
		return NodeResource{}, fmt.Errorf("locator for %s: %w", loc.Resource, err)
	}
	set, err := LoadResourceSet(root, id)
	if err != nil {
		return NodeResource{}, err
	}
	if !ref.Primary && !hasResource(set, loc.Resource) {
		set.Resources = append(set.Resources, ResourceEntry{ID: loc.Resource, Role: "supplementary"})
	}
	a := Attachment{Node: node, Locator: loc}
	exists := false
	for _, e := range set.Attachments {
		exists = exists || e == a
	}
	if !exists {
		set.Attachments = append(set.Attachments, a)
	}
	if err := saveResourceSet(root, id, set); err != nil {
		return NodeResource{}, err
	}
	return nodeResource(ref, loc, i18n.Default), nil
}

// Detach removes every locator of a resource from an outline entry.
func Detach(root, id, node, resource string) (int, error) {
	set, err := LoadResourceSet(root, id)
	if err != nil {
		return 0, err
	}
	if resource == "" {
		resource = id
	}
	kept := set.Attachments[:0]
	removed := 0
	for _, a := range set.Attachments {
		if a.Node == node && a.Locator.Resource == resource {
			removed++
			continue
		}
		kept = append(kept, a)
	}
	set.Attachments = kept
	if removed == 0 {
		return 0, nil
	}
	return removed, saveResourceSet(root, id, set)
}

func hasResource(set ResourceSet, id string) bool {
	for _, r := range set.Resources {
		if r.ID == id {
			return true
		}
	}
	return false
}

func nodeResource(ref SourceRef, loc locator.Locator, lang string) NodeResource {
	return NodeResource{Resource: ref.ID, Title: ref.Title, Kind: ref.Kind, Primary: ref.Primary, Locator: loc, Label: loc.Label(lang)}
}

// NodeResources lists where an outline entry can be studied: the entry's
// own position in the primary material, then attached resources.
func NodeResources(root, id, node, lang string) ([]NodeResource, error) {
	out := []NodeResource{}
	if node == "" {
		return out, nil
	}
	o, err := LoadOutline(root, id)
	if err != nil {
		return nil, err
	}
	if n, ok := o.Find(node); ok {
		if where, ok := n.Where(); ok {
			if ref, err := Resolve(root, id); err == nil {
				where.Resource = id
				out = append(out, nodeResource(ref, where, lang))
			}
		}
	}
	set, err := LoadResourceSet(root, id)
	if err != nil {
		return nil, err
	}
	for _, a := range set.Attachments {
		if a.Node != node {
			continue
		}
		ref, err := Resolve(root, a.Locator.Resource)
		if err != nil {
			out = append(out, NodeResource{Resource: a.Locator.Resource, Locator: a.Locator, Label: a.Locator.Label(lang)})
			continue
		}
		out = append(out, nodeResource(ref, a.Locator, lang))
	}
	return out, nil
}

// CheckIssue is one problem found by Check.
type CheckIssue struct {
	Node    string          `json:"node,omitempty"`
	Locator locator.Locator `json:"locator"`
	Error   string          `json:"error"`
}

// CheckReport summarizes the resources of a curriculum.
type CheckReport struct {
	Curriculum string       `json:"curriculum"`
	Resources  []string     `json:"resources"`
	Invalid    []CheckIssue `json:"invalid"`
	// Unsourced lists entries with no resource position at all; only
	// curricula without a readable primary file can have them.
	Unsourced []string `json:"unsourced"`
}

// Check validates every outline and attachment locator of a curriculum and
// lists entries that no resource covers.
func Check(root, id string) (CheckReport, error) {
	rep := CheckReport{Curriculum: id, Resources: []string{id}, Invalid: []CheckIssue{}, Unsourced: []string{}}
	primary, err := Resolve(root, id)
	if err != nil {
		return rep, err
	}
	o, err := LoadOutline(root, id)
	if err != nil {
		return rep, err
	}
	set, err := LoadResourceSet(root, id)
	if err != nil {
		return rep, err
	}
	for _, r := range set.Resources {
		rep.Resources = append(rep.Resources, r.ID)
	}
	covered := map[string]bool{}
	for _, n := range o.Nodes {
		if where, ok := n.Where(); ok {
			covered[n.ID] = true
			if err := primary.Validate(where); err != nil {
				rep.Invalid = append(rep.Invalid, CheckIssue{Node: n.ID, Locator: where, Error: err.Error()})
			}
		}
	}
	for _, a := range set.Attachments {
		covered[a.Node] = true
		if _, ok := o.Find(a.Node); !ok {
			rep.Invalid = append(rep.Invalid, CheckIssue{Node: a.Node, Locator: a.Locator, Error: "outline entry no longer exists"})
			continue
		}
		ref, err := Resolve(root, a.Locator.Resource)
		if err == nil {
			err = ref.Validate(a.Locator)
		}
		if err != nil {
			rep.Invalid = append(rep.Invalid, CheckIssue{Node: a.Node, Locator: a.Locator, Error: err.Error()})
		}
	}
	if primary.Caps.External {
		for _, n := range o.Nodes {
			if !covered[n.ID] && !hasChildNode(o, n.ID) {
				rep.Unsourced = append(rep.Unsourced, n.ID)
			}
		}
	}
	return rep, nil
}

func hasChildNode(o Outline, id string) bool {
	for _, n := range o.Nodes {
		if parentID(n.ID) == id {
			return true
		}
	}
	return false
}
