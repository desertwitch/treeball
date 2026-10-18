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

	"github.com/lanrat/extsort"
	"github.com/lanrat/extsort/diff"
)

// Diff compares the contents of two sources (directories or tarballs) and
// produces a synthetic tarball representing only the differences between them.
//
// The input paths cmpOld and cmpNew can each be either a tarball (*.tar.gz) or
// a directory. The produced diff tarball has the following internal structure:
//   - Added paths are placed under a synthetic "+++" directory.
//   - Removed paths are placed under a synthetic "---" directory.
//
// Each differing file or folder is represented as a dummy entry to avoid
// including real file contents. Any paths matching the excludes slice are
// skipped on both sides of the input and for resulting diff-consideration.
//
// This function returns:
//   - (*diff.Result, ErrDiffsFound): if any differences are found
//   - (*diff.Result, nil): if the sources are identical (no output file)
//   - (nil, error): for any other failure (I/O, gzip, comparison error, etc.)
//
// The ctx parameter controls early cancellation.
func (prog *Program) Diff(ctx context.Context, cmpOld string, cmpNew string, output string, excludes []string) (*diff.Result, error) {
	var oldStream, newStream <-chan string
	var oldErrs, newErrs <-chan error

	out, err := createAtomic(prog.fs, output)
	if err != nil {
		return nil, err
	}
	defer out.Abort()

	gw, err := gzip.NewWriterLevel(out, prog.gzipConfig.CompressionLevel)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize gzip writer: %w", err)
	}
	defer gw.Close() // only for early returns; checked explicitly below

	tw := tar.NewWriter(gw)
	defer tw.Close() // only for early returns; checked explicitly below

	stdout := bufio.NewWriterSize(prog.stdout, 64<<10) //nolint:mnd
	defer stdout.Flush()

	if oldStream, oldErrs, err = prog.multiPathStream(ctx, cmpOld, true, excludes); err != nil {
		return nil, fmt.Errorf("failed to establish old stream: %w", err)
	}
	if newStream, newErrs, err = prog.multiPathStream(ctx, cmpNew, true, excludes); err != nil {
		return nil, fmt.Errorf("failed to establish new stream: %w", err)
	}

	result, err := diff.Strings(
		ctx,
		oldStream, newStream,
		oldErrs, newErrs,
		func(delta diff.Delta, item string) error {
			switch delta {
			case diff.OLD:
				fmt.Fprintf(stdout, "--- %s\n", item)

				isDir := strings.HasSuffix(item, "/")

				return writeDummyFile(tw, filepath.Join("---", item), isDir)
			case diff.NEW:
				fmt.Fprintf(stdout, "+++ %s\n", item)

				isDir := strings.HasSuffix(item, "/")

				return writeDummyFile(tw, filepath.Join("+++", item), isDir)
			}

			return nil
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failure during diff: %w", err)
	}

	if result.ExtraA == 0 && result.ExtraB == 0 {
		return &result, nil // no differences: temporary file is discarded
	}

	// Closing flushes the remaining buffered data; errors here mean a broken archive.
	if err := tw.Close(); err != nil {
		return nil, fmt.Errorf("failed to finalize tar stream: %w", err)
	}
	if err := gw.Close(); err != nil {
		return nil, fmt.Errorf("failed to finalize gzip stream: %w", err)
	}
	if err := out.Commit(); err != nil {
		return nil, err
	}

	return &result, ErrDiffsFound
}

// multiPathStream streams the relative paths of a source, choosing the source
// type by itself: directories are streamed with [Program.fsPathStream], anything
// else is treated as a tarball and streamed with [Program.tarPathStream].
// An error is only returned directly when the source cannot be stat'ed.
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

// fsPathStream walks the directory tree at path and streams the paths of all
// contained entries relative to it, with directories ending in "/". Excluded
// directories are skipped with all of their contents. If sort is true, the paths
// are passed through [extsortStrings] before streaming. The first walk error is
// sent on the error channel; both channels are closed once streaming ends.
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

// tarPathStream reads the tarball at path and streams the names of all contained
// entries, in archive order or sorted through [extsortStrings] if sort is true.
// Excludes are applied with a [treeExcluder], so they select the same entries as
// on a filesystem walk. The first read error is sent on the error channel; both
// channels are closed once streaming ends.
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
		tarExcl := &treeExcluder{excludes: excludes}
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

			if excluded, err := tarExcl.isExcluded(hdr.Name, strings.HasSuffix(hdr.Name, "/")); err != nil {
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
