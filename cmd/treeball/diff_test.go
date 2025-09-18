package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

// A helper function for tests to create a tarball with empty dummy files.
func createTar(entries []string) []byte {
	var buf bytes.Buffer

	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)

	for _, name := range entries {
		_ = writeDummyFile(tw, name, strings.HasSuffix(name, "/"))
	}

	tw.Close()
	gz.Close()

	return buf.Bytes()
}

// Expectation: A difference between the tarballs should be found, the correct error returned and the output file exist.
func Test_Program_Diff_DiffsFound_Success(t *testing.T) {
	fs := afero.NewMemMapFs()

	require.NoError(t, afero.WriteFile(fs, "/old.tar.gz", createTar([]string{"a.txt", "b/", "b/x.txt"}), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/new.tar.gz", createTar([]string{"a.txt", "b/", "b/y.txt"}), 0o644))

	prog := NewProgram(fs, io.Discard, io.Discard, nil, nil)
	_, err := prog.Diff(t.Context(), "/old.tar.gz", "/new.tar.gz", "/diff.tar.gz", nil)
	require.ErrorIs(t, err, ErrDiffsFound)

	f, err := fs.Open("/diff.tar.gz")
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

	require.Equal(t, []string{"---/b/x.txt", "+++/b/y.txt"}, names)
}

// Expectation: No difference between the tarballs should be found, no error returned and the output file removed.
func Test_Program_Diff_NoDiffsFound_Success(t *testing.T) {
	fs := afero.NewMemMapFs()

	require.NoError(t, afero.WriteFile(fs, "/old.tar.gz", createTar([]string{"a.txt", "b/", "b/x.txt"}), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/new.tar.gz", createTar([]string{"a.txt", "b/", "b/x.txt"}), 0o644))

	prog := NewProgram(fs, io.Discard, io.Discard, nil, nil)
	_, err := prog.Diff(t.Context(), "/old.tar.gz", "/new.tar.gz", "/diff.tar.gz", nil)
	require.NoError(t, err)

	_, err = fs.Stat("/diff.tar.gz")
	require.ErrorIs(t, err, os.ErrNotExist)
}

// Expectation: A context cancellation should be respected and the output file removed.
func Test_Program_Diff_CtxCancel_Error(t *testing.T) {
	fs := afero.NewMemMapFs()

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	require.NoError(t, afero.WriteFile(fs, "/old.tar.gz", createTar([]string{"a.txt"}), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/new.tar.gz", createTar([]string{"a.txt", "b.txt"}), 0o644))

	prog := NewProgram(fs, io.Discard, io.Discard, nil, nil)
	_, err := prog.Diff(ctx, "/old.tar.gz", "/new.tar.gz", "/diff.tar.gz", nil)
	require.ErrorIs(t, err, context.Canceled)

	_, err = fs.Stat("/diff.tar.gz")
	require.ErrorIs(t, err, os.ErrNotExist)
}

// Expectation: A create failure should raise the appropriate error and the output file be removed.
func Test_Program_Diff_CreateFile_Error(t *testing.T) {
	baseFs := afero.NewMemMapFs()

	require.NoError(t, afero.WriteFile(baseFs, "/old.tar.gz", createTar([]string{"a.txt"}), 0o644))
	require.NoError(t, afero.WriteFile(baseFs, "/new.tar.gz", createTar([]string{"a.txt", "b.txt"}), 0o644))

	fs := errorFs{Fs: baseFs}
	prog := NewProgram(fs, io.Discard, io.Discard, nil, nil)

	_, err := prog.Diff(t.Context(), "/old.tar.gz", "/new.tar.gz", "/diff.tar.gz", nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "simulated create failure")

	_, statErr := fs.Stat("/diff.tar.gz")
	require.ErrorIs(t, statErr, os.ErrNotExist)
}

// Expectation: An invalid configuration should raise the appropriate error and the output file be removed.
func Test_Program_Diff_InvalidConfig_Error(t *testing.T) {
	fs := afero.NewMemMapFs()

	require.NoError(t, afero.WriteFile(fs, "/old.tar.gz", createTar([]string{"a.txt"}), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/new.tar.gz", createTar([]string{"a.txt", "b.txt"}), 0o644))

	cfg := gzipConfigDefault
	cfg.CompressionLevel = -17

	prog := NewProgram(fs, io.Discard, io.Discard, &cfg, nil)
	_, err := prog.Diff(t.Context(), "/old.tar.gz", "/new.tar.gz", "/diff.tar.gz", nil)
	require.Error(t, err)

	_, err = fs.Stat("/diff.tar.gz")
	require.ErrorIs(t, err, os.ErrNotExist)
}
