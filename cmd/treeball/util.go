package main

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"strings"
	"time"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/lanrat/extsort"
	"github.com/spf13/afero"
)

// GzipConfig is the configuration for concurrent gzip operations.
type GzipConfig struct {
	BlockSize        int // Approximate size of blocks (pgzip operations)
	BlockCount       int // Amount of blocks processing in parallel (pgzip operations)
	CompressionLevel int // Target level for compression (0: none to 9: highest)
}

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

func (prog *Program) mergeExcludes(excludeSlice []string, excludeFile string) ([]string, error) {
	excludes := []string{}

	if excludeFile != "" {
		file, err := prog.fs.Open(excludeFile)
		if err != nil {
			return nil, fmt.Errorf("failed to open exclude file: %w", err)
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())

			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}

			excludes = append(excludes, line)
		}

		if err := scanner.Err(); err != nil {
			return nil, fmt.Errorf("failed reading exclude file: %w", err)
		}
	}

	excludes = append(excludes, excludeSlice...)

	return excludes, nil
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

func (prog *Program) multiPathStream(ctx context.Context, path string, sort bool, excludes []string) (<-chan string, <-chan error, error) {
	info, err := prog.fs.Stat(path)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to stat: %w", err)
	}

	if info.IsDir() {
		paths, errs := prog.fsPathStream(ctx, path, sort, excludes)

		return paths, errs, nil
	}

	paths, errs := prog.tarPathStream(ctx, path, sort, excludes)

	return paths, errs, nil
}

func (prog *Program) fsPathStream(ctx context.Context, path string, sort bool, excludes []string) (<-chan string, <-chan error) {
	paths := make(chan string, fsStreamBuffer)
	errs := make(chan error, 1)

	go func() {
		defer close(paths)
		defer close(errs)

		if err := prog.fsWalker.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
			if err := ctx.Err(); err != nil {
				return fmt.Errorf("failed to walk filesystem: %w", err)
			}

			if err != nil {
				return fmt.Errorf("failed to walk filesystem: %w", err)
			}

			if p == path {
				return nil
			}

			relPath, err := filepath.Rel(path, p)
			if err != nil {
				return fmt.Errorf("failed to obtain relative path: %w", err)
			}

			if excluded, err := isExcluded(relPath, d.IsDir(), excludes); err != nil {
				return fmt.Errorf("failed to check for exclusion: %w", err)
			} else if excluded && d.IsDir() {
				return filepath.SkipDir
			} else if excluded {
				return nil
			}

			relPath = filepath.ToSlash(relPath)
			if d.IsDir() && !strings.HasSuffix(relPath, "/") {
				relPath += "/"
			}

			paths <- relPath

			return nil
		}); err != nil {
			errs <- fmt.Errorf("failed to stream from fs: %w", err)
		}
	}()

	if !sort {
		return paths, errs
	}

	return extsortStrings(ctx, paths, errs, prog.extSortConfig)
}

func (prog *Program) tarPathStream(ctx context.Context, path string, sort bool, excludes []string) (<-chan string, <-chan error) {
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
				errs <- fmt.Errorf("failed to stream from tar: %w", err)

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

			if excluded, err := isExcluded(hdr.Name, strings.HasSuffix(hdr.Name, "/"), excludes); err != nil {
				errs <- fmt.Errorf("failed to check for exclusion: %w", err)

				return
			} else if !excluded {
				paths <- hdr.Name
			}
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
