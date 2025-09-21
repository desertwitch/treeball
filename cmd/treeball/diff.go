package main

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"path/filepath"
	"strings"

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
func (prog *Program) Diff(ctx context.Context, cmpOld string, cmpNew string, output string, excludes []string) (*diff.Result, error) { //nolint:unparam
	var hasDifferences bool
	var oldStream, newStream <-chan string
	var oldErrs, newErrs <-chan error

	out, err := prog.fs.Create(output)
	if err != nil {
		return nil, fmt.Errorf("failed to create output file: %w", err)
	}

	defer func() {
		if !hasDifferences {
			_ = prog.fs.Remove(output)
		}
	}()
	defer out.Close()

	gw, err := gzip.NewWriterLevel(out, prog.gzipConfig.CompressionLevel)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize gzip writer: %w", err)
	}
	defer gw.Close()

	tw := tar.NewWriter(gw)
	defer tw.Close()

	if oldStream, oldErrs, err = prog.multiPathStream(ctx, cmpOld, true, excludes); err != nil {
		return nil, fmt.Errorf("failed to establish stream: %w", err)
	}
	if newStream, newErrs, err = prog.multiPathStream(ctx, cmpNew, true, excludes); err != nil {
		return nil, fmt.Errorf("failed to establish stream: %w", err)
	}

	result, err := diff.Strings(
		ctx,
		oldStream, newStream,
		oldErrs, newErrs,
		func(delta diff.Delta, item string) error {
			switch delta {
			case diff.OLD:
				fmt.Fprintf(prog.stdout, "--- %s\n", item)

				isDir := strings.HasSuffix(item, "/")

				return writeDummyFile(tw, filepath.Join("---", item), isDir)
			case diff.NEW:
				fmt.Fprintf(prog.stdout, "+++ %s\n", item)

				isDir := strings.HasSuffix(item, "/")

				return writeDummyFile(tw, filepath.Join("+++", item), isDir)
			}

			return nil
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failure during diff: %w", err)
	}

	if result.ExtraA > 0 || result.ExtraB > 0 {
		hasDifferences = true

		return &result, ErrDiffsFound
	}

	return &result, nil
}
