// Command bundle-openapi rewrites a multi-file OpenAPI spec into a
// single self-contained JSON file.
//
// It walks the spec recursively; every `$ref` that points to an
// external relative file is replaced with an internal
// `#/components/schemas/<Name>` (or `.../responses/<Name>`) pointer.
// The ref target itself is loaded, its own refs resolved, and the
// content hoisted into the matching component bucket at the root.
//
// Motivation: oapi-codegen's underlying OpenAPI loader can't follow
// external `$ref`s across sibling directories, which is exactly how
// the OpenSubsonic spec is laid out. Running this bundler as a
// `go generate` step ahead of oapi-codegen gives the generator the
// single-file input it expects while keeping us on a pure-Go
// toolchain.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: bundle-openapi <in.json> <out.json>")
		os.Exit(2)
	}
	if err := run(os.Args[1], os.Args[2]); err != nil {
		fmt.Fprintln(os.Stderr, "bundle-openapi:", err)
		os.Exit(1)
	}
}

func run(inPath, outPath string) error {
	rootDir, err := filepath.Abs(filepath.Dir(inPath))
	if err != nil {
		return err
	}
	raw, err := os.ReadFile(inPath)
	if err != nil {
		return err
	}
	var root map[string]any
	if err := json.Unmarshal(stripTrailingCommas(raw), &root); err != nil {
		return fmt.Errorf("parse %s: %w", inPath, err)
	}

	comps := ensureMap(root, "components")
	schemas := ensureMap(comps, "schemas")
	resps := ensureMap(comps, "responses")

	b := &bundler{
		schemas: schemas,
		resps:   resps,
		byAbs:   map[string]string{},
	}
	b.registerBucket(schemas, rootDir, "#/components/schemas/")
	b.registerBucket(resps, rootDir, "#/components/responses/")

	// Materialize: replace every $ref bucket entry with its resolved
	// file content (itself walked for nested refs). Done first so the
	// buckets are concrete by the time the later walk reaches the rest
	// of the spec.
	if err := b.materialize(schemas, rootDir); err != nil {
		return err
	}
	if err := b.materialize(resps, rootDir); err != nil {
		return err
	}

	// Walk everything outside the two component buckets, rewriting
	// external refs to the internal pointers we just registered.
	newRoot := make(map[string]any, len(root))
	for k, v := range root {
		if k == "components" {
			continue
		}
		w, err := b.walk(v, rootDir)
		if err != nil {
			return err
		}
		newRoot[k] = w
	}
	newComps := make(map[string]any, len(comps))
	for k, v := range comps {
		if k == "schemas" || k == "responses" {
			newComps[k] = v
			continue
		}
		w, err := b.walk(v, rootDir)
		if err != nil {
			return err
		}
		newComps[k] = w
	}
	newRoot["components"] = newComps

	data, err := json.MarshalIndent(newRoot, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(outPath, data, 0o644)
}

type bundler struct {
	schemas map[string]any    // components.schemas (mutated in place)
	resps   map[string]any    // components.responses (mutated in place)
	byAbs   map[string]string // absolute file path → internal $ref string
}

func (b *bundler) registerBucket(bucket map[string]any, baseDir, prefix string) {
	for key, val := range bucket {
		ref := asRef(val)
		if ref == "" {
			continue
		}
		abs, err := resolveRef(ref, baseDir)
		if err != nil || abs == "" {
			continue
		}
		b.byAbs[abs] = prefix + key
	}
}

// materialize replaces each `$ref` entry in bucket with the loaded
// content of the referenced file.
func (b *bundler) materialize(bucket map[string]any, baseDir string) error {
	for key, val := range bucket {
		ref := asRef(val)
		if ref == "" {
			continue
		}
		abs, err := resolveRef(ref, baseDir)
		if err != nil {
			return err
		}
		content, err := b.load(abs)
		if err != nil {
			return fmt.Errorf("bucket %q -> %s: %w", key, ref, err)
		}
		bucket[key] = content
	}
	return nil
}

func (b *bundler) load(abs string) (any, error) {
	raw, err := os.ReadFile(abs)
	if err != nil {
		return nil, err
	}
	var node any
	if err := json.Unmarshal(stripTrailingCommas(raw), &node); err != nil {
		return nil, fmt.Errorf("parse %s: %w", abs, err)
	}
	return b.walk(node, filepath.Dir(abs))
}

// stripTrailingCommas removes JSON trailing commas (a `,` immediately
// before `}` or `]`, with optional whitespace in between). The
// OpenSubsonic spec includes them in several schema files — standard
// JSON doesn't allow them but hand-edited specs commonly do. Commas
// inside string literals are preserved via a tiny parser state
// machine.
func stripTrailingCommas(b []byte) []byte {
	out := make([]byte, 0, len(b))
	inString := false
	escape := false
	for i := 0; i < len(b); i++ {
		c := b[i]
		if inString {
			out = append(out, c)
			switch {
			case escape:
				escape = false
			case c == '\\':
				escape = true
			case c == '"':
				inString = false
			}
			continue
		}
		if c == '"' {
			inString = true
			out = append(out, c)
			continue
		}
		if c == ',' {
			j := i + 1
			for j < len(b) && (b[j] == ' ' || b[j] == '\t' || b[j] == '\n' || b[j] == '\r') {
				j++
			}
			if j < len(b) && (b[j] == '}' || b[j] == ']') {
				continue
			}
		}
		out = append(out, c)
	}
	return out
}

// walk returns a copy of node with every external `$ref` rewritten to
// an internal `#/components/...` pointer. External refs that weren't
// registered in byAbs are loaded, hoisted into schemas, and then
// rewritten.
func (b *bundler) walk(node any, dir string) (any, error) {
	switch v := node.(type) {
	case map[string]any:
		if r, isRef := v["$ref"].(string); isRef && len(v) == 1 {
			if strings.HasPrefix(r, "#/") {
				return v, nil
			}
			abs, err := resolveRef(r, dir)
			if err != nil {
				return nil, err
			}
			if internal, ok := b.byAbs[abs]; ok {
				return map[string]any{"$ref": internal}, nil
			}
			// Orphan external ref — inline the loaded content in place
			// rather than hoisting. Hoisting into components.schemas
			// would be wrong for PathItem refs (the `paths` section
			// points at ./endpoints/*.json files which are operation
			// objects, not schemas) and spurious for anything else
			// that isn't already a named top-level component.
			return b.load(abs)
		}
		out := make(map[string]any, len(v))
		for k, child := range v {
			c, err := b.walk(child, dir)
			if err != nil {
				return nil, err
			}
			out[k] = c
		}
		return out, nil
	case []any:
		out := make([]any, len(v))
		for i, child := range v {
			c, err := b.walk(child, dir)
			if err != nil {
				return nil, err
			}
			out[i] = c
		}
		return out, nil
	default:
		return v, nil
	}
}

func asRef(v any) string {
	m, ok := v.(map[string]any)
	if !ok || len(m) != 1 {
		return ""
	}
	r, _ := m["$ref"].(string)
	return r
}

// resolveRef turns a JSON-Schema `$ref` value into an absolute
// filesystem path. Any JSON-pointer fragment is stripped — this
// bundler assumes refs point at whole files (which matches the
// OpenSubsonic layout).
func resolveRef(ref, baseDir string) (string, error) {
	if i := strings.Index(ref, "#"); i >= 0 {
		ref = ref[:i]
	}
	if ref == "" {
		return "", nil
	}
	return filepath.Abs(filepath.Join(baseDir, filepath.FromSlash(ref)))
}

func ensureMap(parent map[string]any, key string) map[string]any {
	if m, ok := parent[key].(map[string]any); ok {
		return m
	}
	m := map[string]any{}
	parent[key] = m
	return m
}
