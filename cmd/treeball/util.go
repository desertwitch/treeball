package main

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"strings"
	"time"

	"github.com/lanrat/extsort"
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
	return filepath.WalkDir(root, fn)
}

type fileInfoDirEntry struct {
	fs.FileInfo
}

func (fi fileInfoDirEntry) Type() fs.FileMode {
	return fi.Mode().Type()
}

func (fi fileInfoDirEntry) Info() (fs.FileInfo, error) {
	return fi.FileInfo, nil
}

func (fi fileInfoDirEntry) IsDir() bool {
	return fi.Mode().IsDir()
}

func (fi fileInfoDirEntry) Name() string {
	return fi.FileInfo.Name()
}

func isExcluded(path string, excludes []string) bool {
	path = filepath.Clean(strings.TrimSpace(path))

	for _, excl := range excludes {
		if path == excl {
			return true
		}
		if rel, err := filepath.Rel(excl, path); err == nil && !strings.HasPrefix(rel, "..") {
			return true
		}
	}

	return false
}

func writeDummyFile(tw *tar.Writer, name string, isDir bool) error {
	name = filepath.ToSlash(name)

	hdr := &tar.Header{
		Name:    name,
		ModTime: time.Time{},
	}

	if isDir {
		hdr.Mode = baseFolderPerms
		hdr.Typeflag = tar.TypeDir

		if !strings.HasSuffix(hdr.Name, "/") {
			hdr.Name += "/"
		}
	} else {
		hdr.Mode = baseFilePerms
		hdr.Typeflag = tar.TypeReg
	}

	hdr.Size = 0

	if err := tw.WriteHeader(hdr); err != nil {
		return fmt.Errorf("failed to write tar header: %w", err)
	}

	return nil
}

func (prog *Program) tarPathStream(ctx context.Context, path string, sort bool) (<-chan string, <-chan error) {
	paths := make(chan string, tarStreamBuffer)
	errs := make(chan error, 1)

	go func() {
		defer close(paths)
		defer close(errs)

		f, err := prog.fs.Open(path)
		if err != nil {
			errs <- fmt.Errorf("failed to open input file: %w", err)

			return
		}
		defer f.Close()

		gz, err := gzip.NewReader(f)
		if err != nil {
			errs <- fmt.Errorf("failed to initialize gzip reader: %w", err)

			return
		}
		defer gz.Close()

		tr := tar.NewReader(gz)
		for {
			if err := ctx.Err(); err != nil {
				errs <- ctx.Err()

				return
			}

			hdr, err := tr.Next()
			if err != nil {
				if !errors.Is(err, io.EOF) {
					errs <- fmt.Errorf("failed to stream from tar: %w", err)

					return
				}

				break // EOF
			}

			paths <- hdr.Name
		}
	}()

	if !sort {
		return paths, errs
	}

	return extsortStrings(ctx, paths, errs, prog.extSortConfig)
}

// extsortStrings wraps [extsort.Strings] for internal use.
//
// It merges two possible error sources into a single channel:
//  1. Runtime sorting errors - any errors raised while sorting proceeds.
//  2. extErrs (optional) - errors from non-sorting work such as tar-reading.
//
// Do note that only the first error observed from these sources is sent downstream.
func extsortStrings(ctx context.Context, input <-chan string, extErrs <-chan error, config *extsort.Config) (<-chan string, <-chan error) {
	sorter, sorterOut, sorterErrs := extsort.Strings(input, config)

	if sorter != nil {
		go sorter.Sort(ctx)
	}

	mergedErrs := make(chan error, 1)
	go func() {
		defer close(mergedErrs)

		for extErrs != nil || sorterErrs != nil {
			select {
			case err, ok := <-extErrs:
				if ok && err != nil {
					mergedErrs <- err

					return
				}
				extErrs = nil // channel closed, disable case.

			case err, ok := <-sorterErrs:
				if ok && err != nil {
					mergedErrs <- err

					return
				}
				sorterErrs = nil // channel closed, disable case.
			}
		}
	}()

	return sorterOut, mergedErrs
}
