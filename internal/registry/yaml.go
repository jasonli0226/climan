package registry

import (
	"bytes"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type fileShape struct {
	Version  int               `yaml:"version"`
	LastSync time.Time         `yaml:"last_sync"`
	Tools    map[string]*Entry `yaml:"tools"`
}

// Parse reads a registry file. name is used in error messages.
func Parse(name string, data []byte) (*Registry, error) {
	if len(bytes.TrimSpace(data)) == 0 {
		return New(), nil
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	var f fileShape
	if err := doc.Decode(&f); err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	if f.Version == 0 {
		f.Version = SchemaVersion
	}
	if f.Version > SchemaVersion {
		return nil, fmt.Errorf("%s: registry version %d is newer than this climan supports", name, f.Version)
	}
	if f.Tools == nil {
		f.Tools = map[string]*Entry{}
	}
	return &Registry{Version: f.Version, LastSync: f.LastSync, Tools: f.Tools, doc: &doc}, nil
}

// Marshal writes the registry back into its original document, so comments,
// key order and unknown keys survive. New tools are appended at the end.
func (r *Registry) Marshal() ([]byte, error) {
	if r.doc == nil {
		r.doc = &yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{{Kind: yaml.MappingNode}}}
	}
	root := r.doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("registry: top level is not a mapping")
	}
	if err := setValue(root, "version", SchemaVersion); err != nil {
		return nil, err
	}
	if !r.LastSync.IsZero() {
		if err := setValue(root, "last_sync", r.LastSync); err != nil {
			return nil, err
		}
	}
	tools := mappingValue(root, "tools")
	tools.Style = 0 // block style, so each tool sits on its own lines
	if err := mergeTools(tools, r.Tools); err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(r.doc); err != nil {
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func mergeTools(tools *yaml.Node, entries map[string]*Entry) error {
	existing := map[string]*yaml.Node{}
	for i := 0; i+1 < len(tools.Content); i += 2 {
		existing[tools.Content[i].Value] = tools.Content[i+1]
	}
	var added []string
	for key := range entries {
		if _, ok := existing[key]; !ok {
			added = append(added, key)
		}
	}
	sort.Strings(added)

	for key, node := range existing {
		if e, ok := entries[key]; ok {
			if err := mergeEntry(node, e); err != nil {
				return fmt.Errorf("tool %s: %w", key, err)
			}
		}
	}
	for _, key := range added {
		node := &yaml.Node{Kind: yaml.MappingNode}
		if err := mergeEntry(node, entries[key]); err != nil {
			return fmt.Errorf("tool %s: %w", key, err)
		}
		tools.Content = append(tools.Content, scalarKey(key), node)
	}
	return nil
}

// mergeEntry writes e's fields into the mapping node, replacing values in
// place (keeping their comments), appending new keys, and removing owned keys
// whose value is now empty.
func mergeEntry(node *yaml.Node, e *Entry) error {
	if node.Kind != yaml.MappingNode {
		node.Kind, node.Tag, node.Value, node.Content = yaml.MappingNode, "", "", nil
	}
	var fresh yaml.Node
	if err := fresh.Encode(e); err != nil {
		return err
	}
	want := map[string]*yaml.Node{}
	for i := 0; i+1 < len(fresh.Content); i += 2 {
		want[fresh.Content[i].Value] = fresh.Content[i+1]
	}

	var kept []*yaml.Node
	seen := map[string]bool{}
	for i := 0; i+1 < len(node.Content); i += 2 {
		k, v := node.Content[i], node.Content[i+1]
		nv, owned := want[k.Value]
		switch {
		case owned:
			kept = append(kept, k, withComments(nv, v))
			seen[k.Value] = true
		case ownedKeys[k.Value]:
			// climan field now empty: drop it
		default:
			kept = append(kept, k, v)
		}
	}
	for i := 0; i+1 < len(fresh.Content); i += 2 {
		if k := fresh.Content[i]; !seen[k.Value] {
			kept = append(kept, k, fresh.Content[i+1])
		}
	}
	node.Content = kept
	return nil
}

func withComments(n, old *yaml.Node) *yaml.Node {
	n.HeadComment, n.LineComment, n.FootComment = old.HeadComment, old.LineComment, old.FootComment
	return n
}

// ownedKeys are the YAML keys of Entry's fields.
var ownedKeys = func() map[string]bool {
	keys := map[string]bool{}
	t := reflect.TypeOf(Entry{})
	for i := 0; i < t.NumField(); i++ {
		name, _, _ := strings.Cut(t.Field(i).Tag.Get("yaml"), ",")
		keys[name] = true
	}
	return keys
}()

func setValue(m *yaml.Node, key string, v any) error {
	var n yaml.Node
	if err := n.Encode(v); err != nil {
		return err
	}
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			m.Content[i+1] = withComments(&n, m.Content[i+1])
			return nil
		}
	}
	m.Content = append(m.Content, scalarKey(key), &n)
	return nil
}

// mappingValue returns the mapping stored under key, creating it if absent.
func mappingValue(m *yaml.Node, key string) *yaml.Node {
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			v := m.Content[i+1]
			if v.Kind != yaml.MappingNode {
				v.Kind, v.Tag, v.Value, v.Content, v.Style = yaml.MappingNode, "", "", nil, 0
			}
			return v
		}
	}
	v := &yaml.Node{Kind: yaml.MappingNode}
	m.Content = append(m.Content, scalarKey(key), v)
	return v
}

func scalarKey(s string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: s}
}
