package main

import (
	"archive/tar"
	"context"
	"fmt"
	"path/filepath"
	"strings"

	pgzip "github.com/klauspost/pgzip"
	"github.com/lanrat/extsort/diff"
)

// Diff produces a tarball consisting of the differences between two given tarballs.
func (prog *Program) Diff(ctx context.Context, cmpOld string, cmpNew string, output string) (*diff.Result, error) {
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

	gw, err := pgzip.NewWriterLevel(out, pgzip.BestCompression)
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
