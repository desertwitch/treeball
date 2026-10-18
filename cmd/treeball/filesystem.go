package main

import (
	"fmt"
	"io/fs"
	"path"
	"path/filepath"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/spf13/afero"
)

// Walker is an interface describing a filesystem walking function.
type Walker interface {
	WalkDir(root string, fn fs.WalkDirFunc) error
}

// AferoWalker is an adapter to turn the [afero.Walk] into a [filepath.WalkDir] signature.
type AferoWalker struct {
	FS afero.Fs
}

// WalkDir is a method that adapts [afero.Walk] into a [filepath.WalkDir] compatible signature.
func (w AferoWalker) WalkDir(root string, fn fs.WalkDirFunc) error {
	return afero.Walk(w.FS, root, func(path string, info fs.FileInfo, err error) error { //nolint:wrapcheck
		var entry fs.DirEntry
		if info != nil {
			entry = fileInfoDirEntry{info}
		}

		return fn(path, entry, err)
	})
}

// OSWalker is a wrapper structure for the native [filepath.WalkDir] function.
type OSWalker struct{}

// WalkDir is a wrapper method for the native [filepath.WalkDir] function.
func (w OSWalker) WalkDir(root string, fn fs.WalkDirFunc) error {
	return filepath.WalkDir(root, fn) //nolint:wrapcheck
}

// fileInfoDirEntry is an adapter to turn an [fs.FileInfo] into an [fs.DirEntry].
type fileInfoDirEntry struct {
	fs.FileInfo
}

// Type returns the type bits of the underlying [fs.FileInfo] mode.
func (fi fileInfoDirEntry) Type() fs.FileMode {
	return fi.Mode().Type()
}

// Info returns the underlying [fs.FileInfo]; it never fails.
func (fi fileInfoDirEntry) Info() (fs.FileInfo, error) {
	return fi.FileInfo, nil
}

// IsDir reports whether the underlying [fs.FileInfo] describes a directory.
func (fi fileInfoDirEntry) IsDir() bool {
	return fi.Mode().IsDir()
}

// Name returns the base name of the underlying [fs.FileInfo].
func (fi fileInfoDirEntry) Name() string {
	return fi.FileInfo.Name()
}

// treeExcluder applies excludes to a flat list of archive paths the same way
// they apply to a filesystem walk: on a walk, an excluded directory is skipped
// with all of its contents, so here an entry is also excluded when any of its
// parent directories is. The verdict for the most recent parent directory is
// cached, as consecutive archive entries mostly share the same parent.
type treeExcluder struct {
	excludes []string

	lastParent         string
	lastParentExcluded bool
}

// isExcluded reports whether the archive entry name is excluded, either by itself
// or because one of its parent directories is. Without any excludes, it always
// reports false without further work.
func (e *treeExcluder) isExcluded(name string, isDir bool) (bool, error) {
	if len(e.excludes) == 0 {
		return false, nil
	}

	parent := path.Dir(strings.TrimSuffix(name, "/"))

	if parent != e.lastParent {
		excluded, err := e.anyAncestorExcluded(parent)
		if err != nil {
			return false, err
		}
		e.lastParent, e.lastParentExcluded = parent, excluded
	}

	if e.lastParentExcluded {
		return true, nil
	}

	return isExcluded(name, isDir, e.excludes)
}

// anyAncestorExcluded reports whether dir ("a/b/c") or any of its ancestors ("a", "a/b") is excluded.
func (e *treeExcluder) anyAncestorExcluded(dir string) (bool, error) {
	if dir == "." || dir == "/" {
		return false, nil
	}

	for i := 0; i <= len(dir); i++ {
		if i < len(dir) && dir[i] != '/' {
			continue
		}
		if i == 0 {
			continue
		}

		excluded, err := isExcluded(dir[:i], true, e.excludes)
		if err != nil || excluded {
			return excluded, err
		}
	}

	return false, nil
}

// isExcluded reports whether path matches any of the exclude patterns, which
// follow the doublestar format and are matched against the whole cleaned path.
// A leading "/" in a pattern is ignored; a trailing "/" restricts it to
// directories. An error is returned for an invalid pattern.
func isExcluded(path string, isDir bool, excludes []string) (bool, error) {
	path = filepath.ToSlash(filepath.Clean(path))

	for _, rawPattern := range excludes {
		pattern := filepath.ToSlash(rawPattern)

		needDirMatch := strings.HasSuffix(pattern, "/")
		pattern = strings.TrimPrefix(strings.TrimSuffix(pattern, "/"), "/")

		matched, err := doublestar.Match(pattern, path)
		if err != nil {
			return false, fmt.Errorf("invalid exclude pattern: %w", err)
		}
		if matched {
			if needDirMatch && !isDir {
				continue
			}

			return true, nil
		}
	}

	return false, nil
}
