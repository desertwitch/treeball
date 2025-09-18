package main

import (
	"archive/tar"
	"context"
	"fmt"
	"io/fs"
	"path/filepath"

	pgzip "github.com/klauspost/pgzip"
)

// Create produces a tarball of a target directory structure.
// Any encountered files are replaced with zero-byte empty dummies.
//
// The input parameter specifies the root directory to package. The output
// parameter is the path of the tarball file to create. Any paths matching the
// excludes slice are skipped. The ctx parameter controls early cancellation.
func (prog *Program) Create(ctx context.Context, input string, output string, excludes []string) error {
	var creationDone bool

	out, err := prog.fs.Create(output)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}

	defer func() {
		if !creationDone {
			_ = prog.fs.Remove(output)
		}
	}()
	defer out.Close()

	gw, err := pgzip.NewWriterLevel(out, prog.gzipConfig.CompressionLevel)
	if err != nil {
		return fmt.Errorf("failed to initialize gzip writer: %w", err)
	}
	defer gw.Close()

	if err := gw.SetConcurrency(prog.gzipConfig.BlockSize, prog.gzipConfig.BlockCount); err != nil {
		return fmt.Errorf("failed to set gzip writer settings: %w", err)
	}

	tw := tar.NewWriter(gw)
	defer tw.Close()

	if err := prog.fsWalker.WalkDir(input, func(path string, d fs.DirEntry, err error) error {
		if err := ctx.Err(); err != nil {
			return ctx.Err()
		}

		if err != nil {
			return fmt.Errorf("failed to walk filesystem: %w", err)
		}

		if path == input {
			return nil
		}

		relPath, err := filepath.Rel(input, path)
		if err != nil {
			return fmt.Errorf("failed to obtain relative path: %w", err)
		}

		if excluded, err := isExcluded(relPath, d.IsDir(), excludes); err != nil {
			return fmt.Errorf("invalid exclude pattern: %w", err)
		} else if excluded && d.IsDir() {
			return filepath.SkipDir
		} else if excluded {
			return nil
		}

		if err := writeDummyFile(tw, relPath, d.IsDir()); err != nil {
			return fmt.Errorf("failed to write dummy file: %w", err)
		}

		fmt.Fprintln(prog.stdout, path)

		return nil
	}); err != nil {
		return fmt.Errorf("failure during create: %w", err)
	}

	creationDone = true

	return nil
}
