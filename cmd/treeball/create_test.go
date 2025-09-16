package main

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
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
	require.NoError(t, prog.Create(t.Context(), "/src", "/out.tar.gz", []string{"/src/b"}))

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
func Test_Program_Create_WithFileExcludes_Success(t *testing.T) {
	fs := afero.NewMemMapFs()

	require.NoError(t, afero.WriteFile(fs, "/src/a.txt", []byte("a"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/src/b/c.txt", []byte("c"), 0o644))

	prog := NewProgram(fs, io.Discard, io.Discard, nil, nil)
	require.NoError(t, prog.Create(t.Context(), "/src", "/out.tar.gz", []string{"/src/b/c.txt"}))

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

// Expectation: An invalid compressor configuration should raise an error at runtime.
func Test_Program_Create_InvalidConfig_Error(t *testing.T) {
	fs := afero.NewMemMapFs()

	require.NoError(t, afero.WriteFile(fs, "/src/a.txt", []byte("a"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/src/b/c.txt", []byte("c"), 0o644))

	cfg := PgzipConfig{
		BlockSize:  -1,
		BlockCount: -1,
	}

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
