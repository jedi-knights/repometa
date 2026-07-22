// Package repometa scans a source repository and reports the components
// (buildable units) it contains, plus any monorepo workspace layouts it
// recognizes. It is intended to be imported by downstream tools that
// need to reason about arbitrary repositories without re-implementing
// discovery logic.
//
// The API is unstable. See the repo README for scope and non-goals.
package repometa

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Scan walks root and returns a [Manifest] describing every detected
// component. The returned Manifest is non-nil on success.
//
// The walk is bounded by the caps documented on [WithMaxDepth],
// [WithMaxDirs], and [WithMaxFileSize]; a caller who overrides none of
// them accepts the package defaults. Symlinks and a hardcoded skip list
// (.git, node_modules, vendor, target, dist, build, .next, .angular,
// .venv) are never traversed.
//
// Scan returns an error if root is empty, does not exist, or is not a
// directory. Errors surfaced by the underlying filesystem walk are
// wrapped and returned as-is; there are no exported sentinel errors.
//
// Scan is safe for concurrent use: it holds no package-level state and
// mutates only the Manifest it returns.
func Scan(root string, opts ...Option) (*Manifest, error) {
	if root == "" {
		return nil, errors.New("repometa: root path is empty")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, errors.New("repometa: root is not a directory")
	}

	cfg := defaultOptions()
	for _, opt := range opts {
		opt(&cfg)
	}

	w := newWalker(abs, cfg)
	detectors := allDetectors()

	var components []Component
	if err := w.walk(func(dv dirVisit) {
		for _, d := range detectors {
			for _, f := range d.detect(dv, cfg) {
				components = append(components, Component{
					Kind:       f.Kind,
					Root:       dv.rel,
					Evidence:   f.Evidence,
					Confidence: f.Confidence,
					Workspaces: f.Workspaces,
					Attributes: f.Attributes,
				})
			}
		}
	}); err != nil {
		return nil, err
	}

	components = suppressLooseSourceInsideStructured(components)
	sortComponents(components)

	return &Manifest{
		Root:       abs,
		Components: components,
		Stats:      w.stats,
	}, nil
}

// suppressLooseSourceInsideStructured drops KindCSource / KindAsmSource
// findings that live at, or below, a directory already reported as a
// Make or CMake project. Coverage from other ecosystems (Go, Rust, Node,
// Python) does not suppress loose C/asm — those languages don't own
// C/asm source files.
func suppressLooseSourceInsideStructured(cs []Component) []Component {
	var cBuildRoots []string
	for _, c := range cs {
		if isCBuildKind(c.Kind) {
			cBuildRoots = append(cBuildRoots, c.Root)
		}
	}
	kept := cs[:0]
	for _, c := range cs {
		if isLooseSource(c.Kind) && coveredBy(c.Root, cBuildRoots) {
			continue
		}
		kept = append(kept, c)
	}
	return kept
}

func isLooseSource(k Kind) bool {
	return k == KindCSource || k == KindAsmSource
}

func isCBuildKind(k Kind) bool {
	return k == KindCMakeProject || k == KindMakeProject
}

// coveredBy reports whether path equals, or is a proper descendant of,
// any of roots. Uses forward-slash separator because Component.Root uses
// slash-separated paths regardless of platform. A root of "." matches
// every path because Component.Root is canonicalized to strip any
// leading "./" — the HasPrefix check would otherwise miss all children.
func coveredBy(path string, roots []string) bool {
	for _, r := range roots {
		if r == "." {
			return true
		}
		if r == path {
			return true
		}
		if strings.HasPrefix(path, r+"/") {
			return true
		}
	}
	return false
}

// sortComponents orders components deterministically: by root path, then
// by kind. This makes serialized output diff-friendly.
func sortComponents(cs []Component) {
	sort.Slice(cs, func(i, j int) bool {
		if cs[i].Root != cs[j].Root {
			return cs[i].Root < cs[j].Root
		}
		return string(cs[i].Kind) < string(cs[j].Kind)
	})
}
