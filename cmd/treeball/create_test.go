package main

import (
	"archive/tar"
	"context"
	"io"
	"os"
	"testing"

	pgzip "github.com/klauspost/pgzip"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

// Expectation: A tarball should be created with all given paths contained.
func Test_Program_Create_Success(t *testing.T) {
	fs := afero.NewMemMapFs()

	require.NoError(t, afero.WriteFile(fs, "/src/a.txt", []byte("a"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/src/b/c.txt", []byte("c"), 0o644))

	prog := NewProgram(fs, io.Discard, io.Discard, nil)
	require.NoError(t, prog.Create(t.Context(), "/src", "/out.tar.gz", []string{}))

	f, err := fs.Open("/out.tar.gz")
	require.NoError(t, err)

	gzr, err := pgzip.NewReader(f)
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

	prog := NewProgram(fs, io.Discard, io.Discard, nil)
	require.NoError(t, prog.Create(t.Context(), "/src", "/out.tar.gz", []string{"/src/b"}))

	f, err := fs.Open("/out.tar.gz")
	require.NoError(t, err)

	gzr, err := pgzip.NewReader(f)
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

	prog := NewProgram(fs, io.Discard, io.Discard, nil)
	require.NoError(t, prog.Create(t.Context(), "/src", "/out.tar.gz", []string{"/src/b/c.txt"}))

	f, err := fs.Open("/out.tar.gz")
	require.NoError(t, err)

	gzr, err := pgzip.NewReader(f)
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

	prog := NewProgram(fs, io.Discard, io.Discard, nil)
	require.ErrorIs(t, prog.Create(ctx, "/src", "/out.tar.gz", []string{}), context.Canceled)

	_, err := fs.Stat("/out.tar.gz")
	require.ErrorIs(t, err, os.ErrNotExist)
}
