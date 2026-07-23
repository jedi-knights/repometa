package repometa

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// finding is the per-directory result a detector produces. Scan attaches
// dv.rel as the Component's Root — detectors do not need to fill it in.
type finding struct {
	Kind       Kind
	Evidence   []Evidence
	Confidence float64
	Workspaces []Workspace
	Attributes map[string]string
}

// detector inspects a single directory. Detectors do NOT recurse; the
// walker handles traversal so bounds and skip lists are respected in one
// place.
type detector interface {
	detect(dv dirVisit, cfg options) []finding
}

func allDetectors() []detector {
	return []detector{
		goDetector{},
		rustDetector{},
		pythonDetector{},
		jsDetector{},
		dotnetDetector{},
		javaDetector{},
		cmakeDetector{},
		makeDetector{},
		cDetector{},
		asmDetector{},
	}
}

// ---- shared helpers ----

// hasFile reports whether the directory contains a regular file with
// exactly the given name (case-sensitive).
func hasFile(files []fs.DirEntry, name string) bool {
	for _, f := range files {
		if !f.IsDir() && f.Name() == name {
			return true
		}
	}
	return false
}

// firstFile returns the first name from names present in files, and true.
// If none are present it returns "", false.
func firstFile(files []fs.DirEntry, names ...string) (string, bool) {
	present := make(map[string]bool, len(files))
	for _, f := range files {
		if !f.IsDir() {
			present[f.Name()] = true
		}
	}
	for _, n := range names {
		if present[n] {
			return n, true
		}
	}
	return "", false
}

// countByExt counts files ending in any of the given extensions.
func countByExt(files []fs.DirEntry, exts ...string) int {
	n := 0
	for _, f := range files {
		if f.IsDir() {
			continue
		}
		name := f.Name()
		for _, ext := range exts {
			if strings.HasSuffix(name, ext) {
				n++
				break
			}
		}
	}
	return n
}

// relJoin joins a directory's relative path with a filename, using "/"
// for evidence paths so they are stable across platforms.
func relJoin(dir, name string) string {
	if dir == "." || dir == "" {
		return name
	}
	return dir + "/" + name
}

// Confidence values used when a manifest file is present but its
// contents could not be fully interpreted. The gradation reflects how
// much the detector still knows: an unreadable file (stat succeeded but
// open/read failed, or the size cap fired) means we saw the file existed
// but couldn't inspect its contents at all; an unparsable file means we
// did read the bytes but the unmarshaler rejected them. The component
// is still reported in both cases — its presence is a real signal — but
// downstream consumers get a hint that the shape data is missing.
const (
	confidenceUnreadable = 0.8
	confidenceUnparsable = 0.7
)

// evidenceUnreadable formats the Evidence entry used when readManifest
// returns an error. The filename is repeated in Path and Reason so a
// consumer scanning Evidence text for a specific manifest can match on
// either field.
func evidenceUnreadable(rel, filename string, err error) Evidence {
	return Evidence{
		Path:   relJoin(rel, filename),
		Reason: filename + " unreadable: " + err.Error(),
	}
}

// evidenceUnparsable formats the Evidence entry used when a detector's
// unmarshaler rejects the read bytes. Mirrors [evidenceUnreadable]'s
// shape so the two cases are visually parallel in Evidence output.
func evidenceUnparsable(rel, filename string, err error) Evidence {
	return Evidence{
		Path:   relJoin(rel, filename),
		Reason: filename + " parse error: " + err.Error(),
	}
}

// readManifestOrNil returns the manifest bytes at path, or nil if
// readManifest fails for any reason (stat failure, size cap exceeded,
// read failure). Use it in the workspace-member parsers whose contract
// is "best-effort: on any read failure I return an empty member list,
// no error." Callers that need to distinguish read failures from empty
// results should use [readManifest] directly.
func readManifestOrNil(path string, cfg options) []byte {
	data, err := readManifest(path, cfg)
	if err != nil {
		return nil
	}
	return data
}

// readManifest reads a file at path, capped at cfg.maxFileSize. On
// failure it returns nil and an error explaining the reason (stat
// failure, size cap exceeded, or read failure) so callers can attribute
// the specific cause in Evidence rather than reporting a generic
// "unparsed" state.
func readManifest(path string, cfg options) ([]byte, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("stat: %w", err)
	}
	if info.Size() > cfg.maxFileSize {
		return nil, fmt.Errorf("size %d bytes exceeds cap of %d", info.Size(), cfg.maxFileSize)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read: %w", err)
	}
	return data, nil
}

// expandMembers takes a base directory (absolute), a list of glob patterns
// (as they appear in a workspace manifest), and the Manifest-relative root
// of the workspace. It returns the resolved member paths as Manifest-
// relative paths, sorted and de-duplicated.
//
// Only single-star globs are supported. Patterns containing "**" are
// dropped rather than returned as literal paths — a broken entry
// (a directory a consumer would try to stat but that does not exist)
// is worse than a missing entry. Support for "**" is tracked in TODO.md.
func expandMembers(baseAbs string, patterns []string, wsRel string) []string {
	seen := make(map[string]struct{})
	var out []string
	for _, p := range patterns {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if strings.Contains(p, "**") {
			continue
		}
		matches, err := filepath.Glob(filepath.Join(baseAbs, p))
		if err != nil || len(matches) == 0 {
			rel := joinRel(wsRel, p)
			if _, ok := seen[rel]; !ok {
				seen[rel] = struct{}{}
				out = append(out, rel)
			}
			continue
		}
		for _, m := range matches {
			info, err := os.Stat(m)
			if err != nil || !info.IsDir() {
				continue
			}
			rel, err := filepath.Rel(baseAbs, m)
			if err != nil {
				continue
			}
			joined := joinRel(wsRel, filepath.ToSlash(rel))
			if _, ok := seen[joined]; !ok {
				seen[joined] = struct{}{}
				out = append(out, joined)
			}
		}
	}
	return out
}

// joinRel joins two Manifest-relative paths using "/" separators.
func joinRel(base, sub string) string {
	sub = strings.TrimPrefix(sub, "./")
	if base == "." || base == "" {
		return sub
	}
	return base + "/" + sub
}
