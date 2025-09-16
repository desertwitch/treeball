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

// Diff produces a tarball containing the differences between two given
// tarballs. Any encountered files are replaced with zero-byte empty dummies.
//
// The cmpOld parameter is the path to the original tarball, and cmpNew is the
// path to the new tarball. The output parameter specifies the path of the
// resulting diff tarball, which contains synthetic folders marking added and
// removed paths (+++ and ---). The ctx parameter controls early cancellation.
//
// If differences are found, Diff returns a non-nil *diff.Result along with
// ErrDiffsFound. If no differences are found, the output file is removed before
// returning. Any other returned error indicates a generic failure (I/O, sorting, etc).
func (prog *Program) Diff(ctx context.Context, cmpOld string, cmpNew string, output string) (*diff.Result, error) { //nolint:unparam
	var hasDifferences bool

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

	gw, err := gzip.NewWriterLevel(out, gzip.BestCompression)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize gzip writer: %w", err)
	}
	defer gw.Close()

	tw := tar.NewWriter(gw)
	defer tw.Close()

	oldStream, oldErrs := prog.tarPathStream(ctx, cmpOld, true)
	newStream, newErrs := prog.tarPathStream(ctx, cmpNew, true)

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
