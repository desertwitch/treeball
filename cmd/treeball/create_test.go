package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

// A helper filesystem for tests to simulate filesystem errors.
type errorFs struct {
	afero.Fs
}

// A helper function for tests to simulate file creation failure.
func (e errorFs) Create(name string) (afero.File, error) {
	return nil, errors.New("simulated create failure")
}

// A helper function for tests to simulate file creation failure (via OpenFile).
func (e errorFs) OpenFile(name string, flag int, perm os.FileMode) (afero.File, error) {
	if flag&os.O_CREATE != 0 {
		return nil, errors.New("simulated create failure")
	}

	return e.Fs.OpenFile(name, flag, perm) //nolint:wrapcheck
}

// A helper function for tests to simulate file opening failure.
func (e errorFs) Open(name string) (afero.File, error) {
	return nil, errors.New("simulated open failure")
}

// A helper filesystem walker for tests to simulate filesystem walk errors.
type errorWalker struct{}

// A helper function for tests to simulate filesystem walk failure.
func (errorWalker) WalkDir(path string, fn fs.WalkDirFunc) error {
	return fn(path, nil, errors.New("simulated walk failure"))
}

// A helper filesystem for tests to simulate a full disk (all writes to created files fail).
type fullDiskFs struct {
	afero.Fs
}

// A helper file for tests that fails every write.
type fullDiskFile struct {
	afero.File
}

// A helper function for tests to simulate write failure.
func (f fullDiskFile) Write(p []byte) (int, error) {
	return 0, errors.New("simulated disk full")
}

// A helper function for tests to hand out created files that fail every write.
func (e fullDiskFs) OpenFile(name string, flag int, perm os.FileMode) (afero.File, error) {
	f, err := e.Fs.OpenFile(name, flag, perm)
	if err != nil || flag&os.O_CREATE == 0 {
		return f, err //nolint:wrapcheck
	}

	return fullDiskFile{f}, nil
}

// A helper writer for tests to simulate writing errors.
type errorWriter struct{}

// A helper function for tests to simulate writing failure.
func (errorWriter) Write(p []byte) (int, error) {
	return 0, errors.New("simulated write failure")
}

// Expectation: A tarball should be created with all given paths contained.
func Test_Program_Create_Success(t *testing.T) {
	fs := afero.NewMemMapFs()

	require.NoError(t, afero.WriteFile(fs, "/src/a.txt", []byte("a"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/src/b/c.txt", []byte("c"), 0o644))

	prog := NewProgram(fs, io.Discard, io.Discard, nil, nil)
	require.NoError(t, prog.Create(t.Context(), "/src", "/out.tar.gz", []string{}))

	f, err := fs.Open("/out.tar.gz")
	require.NoError(t, err)

	gzr, err := gzip.NewReader(f)
	require.NoError(t, err)

	tr := tar.NewReader(gzr)
	require.NotNil(t, tr)

	var names []string
	for {
		hdr, err := tr.Next()

		if err == io.EOF {
			break
		}

		names = append(names, hdr.Name)
	}

	require.Equal(t, []string{"a.txt", "b/", "b/c.txt"}, names)
}

// Expectation: A tarball should be created with all given paths contained, except the excluded folder.
func Test_Program_Create_WithExcludes_Success(t *testing.T) {
	fs := afero.NewMemMapFs()

	require.NoError(t, afero.WriteFile(fs, "/src/a.txt", []byte("a"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/src/b/c.txt", []byte("c"), 0o644))

	prog := NewProgram(fs, io.Discard, io.Discard, nil, nil)
	require.NoError(t, prog.Create(t.Context(), "/src", "/out.tar.gz", []string{"b"}))

	f, err := fs.Open("/out.tar.gz")
	require.NoError(t, err)

	gzr, err := gzip.NewReader(f)
	require.NoError(t, err)

	tr := tar.NewReader(gzr)
	require.NotNil(t, tr)

	var names []string
	for {
		hdr, err := tr.Next()

		if err == io.EOF {
			break
		}

		names = append(names, hdr.Name)
	}

	require.Equal(t, []string{"a.txt"}, names)
}

// Expectation: A tarball should be created with all given paths contained, except the excluded file.
func Test_Program_Create_WithFileExcluded_Success(t *testing.T) {
	fs := afero.NewMemMapFs()

	require.NoError(t, afero.WriteFile(fs, "/src/a.txt", []byte("a"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/src/b/c.txt", []byte("c"), 0o644))

	prog := NewProgram(fs, io.Discard, io.Discard, nil, nil)
	require.NoError(t, prog.Create(t.Context(), "/src", "/out.tar.gz", []string{"b/*.txt"}))

	f, err := fs.Open("/out.tar.gz")
	require.NoError(t, err)

	gzr, err := gzip.NewReader(f)
	require.NoError(t, err)

	tr := tar.NewReader(gzr)
	require.NotNil(t, tr)

	var names []string
	for {
		hdr, err := tr.Next()

		if err == io.EOF {
			break
		}

		names = append(names, hdr.Name)
	}

	require.Equal(t, []string{"a.txt", "b/"}, names)
}

// Expectation: The printed paths should be exactly those in the tarball, also beyond the output buffer size.
func Test_Program_Create_StdoutMatchesArchive_Success(t *testing.T) {
	fs := afero.NewMemMapFs()
	for i := range 5000 { // ~100KB of output, more than the 64KB stdout buffer
		require.NoError(t, afero.WriteFile(fs, fmt.Sprintf("/src/dir%d/file-%05d.txt", i%10, i), nil, 0o644))
	}

	var created, listed bytes.Buffer
	require.NoError(t, NewProgram(fs, &created, &bytes.Buffer{}, nil, nil).Create(t.Context(), "/src", "/out.tar.gz", nil))
	require.NoError(t, NewProgram(fs, &listed, &bytes.Buffer{}, nil, nil).List(t.Context(), "/out.tar.gz", false, nil))

	require.Greater(t, created.Len(), 64<<10)
	require.Equal(t, strings.ReplaceAll(listed.String(), "/\n", "\n"), created.String())
}

// Expectation: Symlinks should be recorded as entries (not followed) and round-trip cleanly on a real filesystem.
func Test_Program_Create_SymlinksNotFollowed_Success(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks need privileges on windows")
	}

	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	require.NoError(t, os.MkdirAll(filepath.Join(src, "real"), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(src, "real", "f"), nil, 0o600))
	require.NoError(t, os.Symlink("real", filepath.Join(src, "link")))
	require.NoError(t, os.Symlink("/nonexistent", filepath.Join(src, "dangling")))

	fs := afero.NewOsFs()
	archive := filepath.Join(dir, "src.tar.gz")
	require.NoError(t, NewProgram(fs, &bytes.Buffer{}, &bytes.Buffer{}, nil, nil).Create(t.Context(), src, archive, nil))

	require.Equal(t, []string{"dangling", "link", "real/", "real/f"}, listArchive(t, fs, archive, true, nil, nil))

	_, err := NewProgram(fs, &bytes.Buffer{}, &bytes.Buffer{}, nil, nil).Diff(t.Context(), archive, src, filepath.Join(dir, "d.tar.gz"), nil)
	require.NoError(t, err)
}

// Expectation: The output file should not archive itself when placed inside the input tree.
func Test_Program_Create_OutputInsideInput_Success(t *testing.T) {
	fs := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(fs, "/src/a.txt", nil, 0o644))
	require.NoError(t, afero.WriteFile(fs, "/src/out.tar.gz", []byte("previous"), 0o644))

	var stdout bytes.Buffer
	prog := NewProgram(fs, &stdout, &bytes.Buffer{}, nil, nil)

	require.NoError(t, prog.Create(t.Context(), "/src", "/src/out.tar.gz", nil))
	require.Equal(t, "a.txt\n", stdout.String())
}

// Expectation: The output file should get the same permissions as a plain os.Create would give it.
func Test_Program_Create_OutputPermissions_Success(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "src"), 0o750))

	ref, err := os.Create(filepath.Join(dir, "reference"))
	require.NoError(t, err)
	require.NoError(t, ref.Close())

	prog := NewProgram(afero.NewOsFs(), &bytes.Buffer{}, &bytes.Buffer{}, nil, nil)
	require.NoError(t, prog.Create(t.Context(), filepath.Join(dir, "src"), filepath.Join(dir, "out.tar.gz"), nil))

	refInfo, err := os.Stat(filepath.Join(dir, "reference"))
	require.NoError(t, err)
	outInfo, err := os.Stat(filepath.Join(dir, "out.tar.gz"))
	require.NoError(t, err)

	require.Equal(t, refInfo.Mode().Perm(), outInfo.Mode().Perm())
}

// Expectation: A successful creation should replace an existing output file on a real filesystem.
func Test_Program_Create_ReplacesExistingOutput_Success(t *testing.T) {
	dir := t.TempDir()
	src, out := filepath.Join(dir, "src"), filepath.Join(dir, "out.tar.gz")
	require.NoError(t, os.MkdirAll(src, 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(src, "a.txt"), nil, 0o600))
	require.NoError(t, os.WriteFile(out, []byte("not an archive"), 0o600))

	prog := NewProgram(afero.NewOsFs(), &bytes.Buffer{}, &bytes.Buffer{}, nil, nil)
	require.NoError(t, prog.Create(t.Context(), src, out, nil))

	var list bytes.Buffer
	prog = NewProgram(afero.NewOsFs(), &list, &bytes.Buffer{}, nil, nil)
	require.NoError(t, prog.List(t.Context(), out, true, nil))
	require.Equal(t, "a.txt\n", list.String())
}

// Expectation: An output path that is a symlink should be written through, keeping the symlink.
func Test_Program_Create_SymlinkOutput_Success(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, afero.NewOsFs(), dir, []string{"src/a.txt"})
	target, link := filepath.Join(dir, "target.tar.gz"), filepath.Join(dir, "link.tar.gz")
	require.NoError(t, os.WriteFile(target, []byte("old"), 0o600))
	if err := os.Symlink(target, link); err != nil {
		t.Skip("symlinks not supported")
	}

	fs := afero.NewOsFs()
	require.NoError(t, NewProgram(fs, &bytes.Buffer{}, &bytes.Buffer{}, nil, nil).Create(t.Context(), filepath.Join(dir, "src"), link, nil))

	info, err := os.Lstat(link)
	require.NoError(t, err)
	require.Equal(t, os.ModeSymlink, info.Mode().Type(), "symlink must not be replaced")
	require.Equal(t, []string{"a.txt"}, listArchive(t, fs, target, true, nil, nil))
}

// Expectation: A device like /dev/null should be usable as output and never be replaced or removed.
func Test_Program_Create_DevNullOutput_Success(t *testing.T) {
	if _, err := os.Stat(os.DevNull); err != nil || os.DevNull != "/dev/null" {
		t.Skip("no /dev/null on this platform")
	}

	dir := t.TempDir()
	writeTree(t, afero.NewOsFs(), dir, []string{"a/x"})
	prog := NewProgram(afero.NewOsFs(), &bytes.Buffer{}, &bytes.Buffer{}, nil, nil)

	require.NoError(t, prog.Create(t.Context(), filepath.Join(dir, "a"), os.DevNull, nil))
	require.Error(t, prog.Create(t.Context(), filepath.Join(dir, "missing"), os.DevNull, nil))

	info, err := os.Lstat(os.DevNull)
	require.NoError(t, err)
	require.Equal(t, os.ModeDevice|os.ModeCharDevice, info.Mode().Type())
}

// Expectation: No temporary files should be left behind, whichever way the creation ends.
func Test_Program_Create_NoTempFilesLeftBehind_Table(t *testing.T) {
	cancelled, cancel := context.WithCancel(t.Context())
	cancel()

	tests := []struct {
		name   string
		input  string
		cancel bool
	}{
		{"success", "/old", false},
		{"failure", "/missing", false},
		{"cancel", "/old", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := afero.NewMemMapFs()
			require.NoError(t, afero.WriteFile(fs, "/old/a.txt", nil, 0o644))
			require.NoError(t, fs.MkdirAll("/out", 0o755))

			ctx := t.Context() //nolint:contextcheck
			if tt.cancel {
				ctx = cancelled
			}

			_ = NewProgram(fs, &bytes.Buffer{}, &bytes.Buffer{}, nil, nil).Create(ctx, tt.input, "/out/a.tar.gz", nil)

			leftovers, err := afero.Glob(fs, "/out/.*.tmp")
			require.NoError(t, err)
			require.Empty(t, leftovers)
		})
	}
}

// Expectation: An invalid exclude pattern should produce an error.
func Test_Program_Create_InvalidExcludePattern_Error(t *testing.T) {
	fs := afero.NewMemMapFs()

	require.NoError(t, afero.WriteFile(fs, "/src/a.txt", []byte("a"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/src/b/c.txt", []byte("c"), 0o644))

	prog := NewProgram(fs, io.Discard, io.Discard, nil, nil)
	err := prog.Create(t.Context(), "/src", "/out.tar.gz", []string{"b["})

	require.Error(t, err)
	require.ErrorContains(t, err, "exclude")
}

// Expectation: An invalid configuration should raise the appropriate error and the output file be removed.
func Test_Program_Create_InvalidCompressorBlockCount_Error(t *testing.T) {
	fs := afero.NewMemMapFs()

	require.NoError(t, afero.WriteFile(fs, "/src/a.txt", []byte("a"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/src/b/c.txt", []byte("c"), 0o644))

	cfg := gzipConfigDefault
	cfg.BlockCount = -1

	prog := NewProgram(fs, io.Discard, io.Discard, &cfg, nil)
	require.Error(t, prog.Create(t.Context(), "/src", "/out.tar.gz", []string{}))

	_, err := fs.Stat("/out.tar.gz")
	require.ErrorIs(t, err, os.ErrNotExist)
}

// Expectation: An invalid configuration should raise the appropriate error and the output file be removed.
func Test_Program_Create_InvalidCompressorBlockSize_Error(t *testing.T) {
	fs := afero.NewMemMapFs()

	require.NoError(t, afero.WriteFile(fs, "/src/a.txt", []byte("a"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/src/b/c.txt", []byte("c"), 0o644))

	cfg := gzipConfigDefault
	cfg.BlockSize = -1

	prog := NewProgram(fs, io.Discard, io.Discard, &cfg, nil)
	require.Error(t, prog.Create(t.Context(), "/src", "/out.tar.gz", []string{}))

	_, err := fs.Stat("/out.tar.gz")
	require.ErrorIs(t, err, os.ErrNotExist)
}

// Expectation: An invalid configuration should raise the appropriate error and the output file be removed.
func Test_Program_Create_InvalidCompressorCompLevel_Error(t *testing.T) {
	fs := afero.NewMemMapFs()

	require.NoError(t, afero.WriteFile(fs, "/src/a.txt", []byte("a"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/src/b/c.txt", []byte("c"), 0o644))

	cfg := gzipConfigDefault
	cfg.CompressionLevel = -17

	prog := NewProgram(fs, io.Discard, io.Discard, &cfg, nil)
	require.Error(t, prog.Create(t.Context(), "/src", "/out.tar.gz", []string{}))

	_, err := fs.Stat("/out.tar.gz")
	require.ErrorIs(t, err, os.ErrNotExist)
}

// Expectation: A create failure should raise the appropriate error and the output file be removed.
func Test_Program_Create_CreateFile_Error(t *testing.T) {
	baseFs := afero.NewMemMapFs()

	require.NoError(t, baseFs.MkdirAll("/src", 0o755))
	require.NoError(t, afero.WriteFile(baseFs, "/src/file.txt", []byte("test"), 0o644))

	fs := errorFs{Fs: baseFs}

	prog := NewProgram(fs, io.Discard, io.Discard, nil, nil)

	err := prog.Create(t.Context(), "/src", "/out.tar.gz", nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "simulated create failure")

	_, statErr := fs.Stat("/out.tar.gz")
	require.ErrorIs(t, statErr, os.ErrNotExist)
}

// Expectation: A walk failure should raise the appropriate error and the output file be removed.
func Test_Program_Create_WalkDir_Error(t *testing.T) {
	fs := afero.NewMemMapFs()

	require.NoError(t, afero.WriteFile(fs, "/src/file.txt", []byte("test"), 0o644))

	prog := NewProgram(fs, io.Discard, io.Discard, nil, nil)
	prog.fsWalker = errorWalker{}

	err := prog.Create(t.Context(), "/src", "/out.tar.gz", nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "simulated walk failure")

	_, statErr := fs.Stat("/out.tar.gz")
	require.ErrorIs(t, statErr, os.ErrNotExist)
}

// Expectation: A write failure while flushing the tarball should be reported and no broken tarball left behind.
func Test_Program_Create_FlushFails_Error(t *testing.T) {
	base := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(base, "/src/a.txt", nil, 0o644))
	fs := fullDiskFs{base}

	prog := NewProgram(fs, &bytes.Buffer{}, &bytes.Buffer{}, nil, nil)

	err := prog.Create(t.Context(), "/src", "/out.tar.gz", nil)
	require.ErrorContains(t, err, "simulated disk full")

	exists, _ := afero.Exists(fs, "/out.tar.gz")
	require.False(t, exists, "no broken archive must be left behind")
}

// Expectation: A failed creation should not destroy a pre-existing file at the output path.
func Test_Program_Create_PreservesExistingOutput_Error(t *testing.T) {
	fs := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(fs, "/out.tar.gz", []byte("previous backup"), 0o644))

	prog := NewProgram(fs, &bytes.Buffer{}, &bytes.Buffer{}, nil, nil)

	require.Error(t, prog.Create(t.Context(), "/does-not-exist", "/out.tar.gz", nil))

	data, err := afero.ReadFile(fs, "/out.tar.gz")
	require.NoError(t, err)
	require.Equal(t, "previous backup", string(data))
}

// Expectation: A context cancellation should be respected and the output file removed.
func Test_Program_Create_CtxCancel_Error(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	fs := afero.NewMemMapFs()

	require.NoError(t, afero.WriteFile(fs, "/src/a.txt", []byte("a"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/src/b/c.txt", []byte("c"), 0o644))

	cancel()

	prog := NewProgram(fs, io.Discard, io.Discard, nil, nil)
	require.ErrorIs(t, prog.Create(ctx, "/src", "/out.tar.gz", []string{}), context.Canceled)

	_, err := fs.Stat("/out.tar.gz")
	require.ErrorIs(t, err, os.ErrNotExist)
}

// Expectation: Only targets inside the root should resolve, not look-alike siblings or the root itself.
func Test_relToRoot_Table(t *testing.T) {
	tests := []struct {
		root, target, want string
	}{
		{"/src", "/src/out.tar.gz", "out.tar.gz"},
		{"/src", "/src/sub/out.tar.gz", filepath.Join("sub", "out.tar.gz")},
		{"/src/", "/src/./sub/../out.tar.gz", "out.tar.gz"},
		{"/src", "/src", ""},
		{"/src", "/out.tar.gz", ""},
		{"/src", "/src2/out.tar.gz", ""}, // shared string prefix, but a sibling
		{"/src", "/src/../out.tar.gz", ""},
		{"/", "/out.tar.gz", "out.tar.gz"},
	}

	for _, tt := range tests {
		require.Equal(t, tt.want, relToRoot(tt.root, tt.target), "relToRoot(%q, %q)", tt.root, tt.target)
	}
}

// Expectation: Relative and absolute spellings of the same location should be recognized as such.
func Test_relToRoot_RelativePaths_Success(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	require.Equal(t, "out.tar.gz", relToRoot("src", filepath.Join(dir, "src", "out.tar.gz")))
	require.Equal(t, "out.tar.gz", relToRoot(filepath.Join(dir, "src"), "./src/out.tar.gz"))
	require.Empty(t, relToRoot("src", "out.tar.gz"))
}

// Expectation: The tar buffer should contain the appropriate files and folders.
func Test_writeDummyFile_Success(t *testing.T) {
	var buf bytes.Buffer

	tw := tar.NewWriter(&buf)
	require.NotNil(t, tw)

	require.NoError(t, writeDummyFile(tw, "foo.txt", false))
	require.NoError(t, writeDummyFile(tw, "bar", true))
	require.NoError(t, tw.Close())

	tr := tar.NewReader(&buf)
	require.NotNil(t, tr)

	var names []string
	for {
		hdr, err := tr.Next()

		if err == io.EOF {
			break
		}

		require.NoError(t, err)
		require.Zero(t, hdr.Size)

		if strings.HasSuffix(hdr.Name, "/") {
			require.Equal(t, tar.TypeDir, rune(hdr.Typeflag))
			require.Equal(t, baseFolderPerms, hdr.Mode)
		} else {
			require.Equal(t, tar.TypeReg, rune(hdr.Typeflag))
			require.Equal(t, baseFilePerms, hdr.Mode)
		}

		names = append(names, hdr.Name)
	}

	require.Equal(t, []string{"foo.txt", "bar/"}, names)
}

// Expectation: The function should return the correct error on header write failure.
func Test_writeDummyFile_WriteHeader_Error(t *testing.T) {
	tw := tar.NewWriter(errorWriter{})
	err := writeDummyFile(tw, "fail.txt", false)

	require.Error(t, err)
	require.Contains(t, err.Error(), "header")
}
