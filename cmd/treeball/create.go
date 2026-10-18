package main

import (
	"archive/tar"
	"bufio"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	pgzip "github.com/klauspost/pgzip"
	"github.com/spf13/afero"
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

// atomicFile is an output file that is written to a temporary sibling and
// only moved into place by Commit, so that a failed run never leaves a
// truncated archive behind nor destroys a pre-existing file at that path.
//
// Destinations that already exist but are not regular files (devices such
// as /dev/null, FIFOs, symlinks) are written directly instead: a temporary
// sibling may not be creatable next to them, and a rename would replace the
// device, FIFO or symlink itself rather than write through it.
type atomicFile struct {
	afero.File

	fs        afero.Fs
	dest      string
	direct    bool
	committed bool
}

// createAtomicTries bounds the retries on (unlikely) temporary name collisions.
const createAtomicTries = 100

// createAtomic creates a hidden temporary sibling of dest, e.g. ".out.tar.gz.123456.tmp".
func createAtomic(fsys afero.Fs, dest string) (*atomicFile, error) {
	if info, err := lstat(fsys, dest); err == nil && !info.Mode().IsRegular() {
		f, err := fsys.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, os.FileMode(baseFilePerms))
		if err != nil {
			return nil, fmt.Errorf("failed to create output file: %w", err)
		}

		return &atomicFile{File: f, fs: fsys, dest: dest, direct: true}, nil
	}

	prefix := filepath.Join(filepath.Dir(dest), "."+filepath.Base(dest)+".")

	for range createAtomicTries {
		name := prefix + strconv.FormatUint(rand.Uint64(), 36) + ".tmp" //nolint:gosec

		f, err := fsys.OpenFile(name, os.O_RDWR|os.O_CREATE|os.O_EXCL, os.FileMode(baseFilePerms))
		if errors.Is(err, fs.ErrExist) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("failed to create output file: %w", err)
		}

		return &atomicFile{File: f, fs: fsys, dest: dest}, nil
	}

	return nil, fmt.Errorf("failed to create output file: %w: no unused temporary name", fs.ErrExist)
}

// Commit closes the temporary file and moves it to its destination.
func (a *atomicFile) Commit() error {
	if err := a.Close(); err != nil {
		return fmt.Errorf("failed to close output file: %w", err)
	}
	if a.direct {
		a.committed = true

		return nil
	}
	if err := a.fs.Rename(a.Name(), a.dest); err != nil {
		return fmt.Errorf("failed to move output file into place: %w", err)
	}
	a.committed = true

	return nil
}

// Abort discards the temporary file; it is a no-op after Commit.
// A directly written destination is closed but never removed.
func (a *atomicFile) Abort() {
	if a.committed {
		return
	}
	_ = a.Close()
	if !a.direct {
		_ = a.fs.Remove(a.Name())
	}
}

// lstat returns the info of name itself, not following a final symlink where supported.
func lstat(fsys afero.Fs, name string) (fs.FileInfo, error) {
	if l, ok := fsys.(afero.Lstater); ok {
		info, _, err := l.LstatIfPossible(name)

		return info, err //nolint:wrapcheck
	}

	return fsys.Stat(name) //nolint:wrapcheck
}

// relToRoot returns target relative to root if target lies inside root
// (compared lexically on absolute paths), or an empty string otherwise.
func relToRoot(root, target string) string {
	absRoot, err1 := filepath.Abs(root)
	absTarget, err2 := filepath.Abs(target)
	if err1 != nil || err2 != nil {
		return ""
	}

	rel, err := filepath.Rel(absRoot, absTarget)
	// IsLocal rejects anything escaping root; "." (target == root) is still local.
	if err != nil || rel == "." || !filepath.IsLocal(rel) {
		return ""
	}

	return rel
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
