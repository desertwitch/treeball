package main

import (
	"archive/tar"
	"bufio"
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
	out, err := createAtomic(prog.fs, output)
	if err != nil {
		return err
	}
	defer out.Abort()

	gw, err := pgzip.NewWriterLevel(out, prog.gzipConfig.CompressionLevel)
	if err != nil {
		return fmt.Errorf("failed to initialize gzip writer: %w", err)
	}
	defer gw.Close() // only for early returns; checked explicitly below

	if err := gw.SetConcurrency(prog.gzipConfig.BlockSize, prog.gzipConfig.BlockCount); err != nil {
		return fmt.Errorf("failed to set gzip writer settings: %w", err)
	}

	// If the output lies inside the input tree, don't archive it (or its
	// temporary file, whose random name would show up in every diff).
	skipOut := relToRoot(input, output)
	skipTmp := relToRoot(input, out.Name())

	tw := tar.NewWriter(gw)
	defer tw.Close() // only for early returns; checked explicitly below

	stdout := bufio.NewWriterSize(prog.stdout, 64<<10) //nolint:mnd
	defer stdout.Flush()

	if err := prog.fsWalker.WalkDir(input, func(path string, d fs.DirEntry, err error) error {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("failed to walk filesystem: %w", err)
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

		if !d.IsDir() && (relPath == skipOut || relPath == skipTmp) {
			return nil
		}

		if excluded, err := isExcluded(relPath, d.IsDir(), excludes); err != nil {
			return fmt.Errorf("failed to check for exclusion: %w", err)
		} else if excluded && d.IsDir() {
			return filepath.SkipDir
		} else if excluded {
			return nil
		}

		if err := writeDummyFile(tw, relPath, d.IsDir()); err != nil {
			return fmt.Errorf("failed to write dummy file: %w", err)
		}

		fmt.Fprintln(stdout, relPath)

		return nil
	}); err != nil {
		return fmt.Errorf("failure during create: %w", err)
	}

	// Closing flushes the remaining buffered data; errors here mean a broken archive.
	if err := tw.Close(); err != nil {
		return fmt.Errorf("failed to finalize tar stream: %w", err)
	}
	if err := gw.Close(); err != nil {
		return fmt.Errorf("failed to finalize gzip stream: %w", err)
	}

	return out.Commit()
}
