package repometa

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
)

// dirVisit is the payload the walker hands each detector for a single
// directory. Files contains only non-directory entries (regular files
// and other special entries), sorted lexicographically by name.
type dirVisit struct {
	abs   string
	rel   string
	files []fs.DirEntry
}

type walker struct {
	root  string
	cfg   options
	stats ScanStats
}

func newWalker(root string, cfg options) *walker {
	return &walker{root: root, cfg: cfg}
}

type visitFunc func(dirVisit)

func (w *walker) walk(visit visitFunc) error {
	return w.walkDir(w.root, ".", 0, visit)
}

func (w *walker) walkDir(abs, rel string, depth int, visit visitFunc) error {
	if depth > w.cfg.maxDepth {
		w.stats.DepthCapHits++
		return nil
	}
	if w.stats.DirsVisited >= w.cfg.maxDirs {
		w.stats.DirCapHits++
		return nil
	}

	entries, err := os.ReadDir(abs)
	if err != nil {
		// A permission error on a subdirectory should not abort the
		// whole scan; skip it and continue. Any other error at the root
		// bubbles up because Scan already stat'd the root.
		if rel == "." {
			return err
		}
		return nil
	}

	files := make([]fs.DirEntry, 0, len(entries))
	subdirs := make([]fs.DirEntry, 0)
	for _, e := range entries {
		name := e.Name()
		if skipDirs[name] {
			continue
		}
		if e.IsDir() {
			subdirs = append(subdirs, e)
			continue
		}
		if e.Type()&fs.ModeSymlink != 0 {
			w.stats.SymlinksSkipped++
			continue
		}
		files = append(files, e)
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Name() < files[j].Name() })
	sort.Slice(subdirs, func(i, j int) bool { return subdirs[i].Name() < subdirs[j].Name() })

	w.stats.DirsVisited++
	w.stats.FilesSeen += len(files)

	visit(dirVisit{abs: abs, rel: rel, files: files})

	for _, sd := range subdirs {
		if sd.Type()&fs.ModeSymlink != 0 {
			w.stats.SymlinksSkipped++
			continue
		}
		childAbs := filepath.Join(abs, sd.Name())
		childRel := sd.Name()
		if rel != "." {
			childRel = filepath.Join(rel, sd.Name())
		}
		if err := w.walkDir(childAbs, childRel, depth+1, visit); err != nil {
			return err
		}
	}
	return nil
}

// skipDirs is the hardcoded set of directory names never descended into.
// These are unambiguous ecosystem artifacts / caches / tool state that
// would explode the walk time without adding component information.
// Names that are commonly legitimate source directories in some
// codebases (bin, obj, out, env) are intentionally NOT listed here.
var skipDirs = map[string]bool{
	".git":          true,
	".hg":           true,
	".svn":          true,
	".idea":         true,
	".vscode":       true,
	".gradle":       true,
	".mvn":          true,
	".next":         true,
	".nuxt":         true,
	".angular":      true,
	".pytest_cache": true,
	".mypy_cache":   true,
	".ruff_cache":   true,
	".tox":          true,
	".terraform":    true,
	".direnv":       true,
	"__pycache__":   true,
	"node_modules":  true,
	".venv":         true,
	"venv":          true,
	"vendor":        true,
	"target":        true,
	"dist":          true,
	"build":         true,
}
