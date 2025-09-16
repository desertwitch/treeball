package main

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

// Expectation: A sorted list should be produced on standard output (stdout).
func Test_Program_List_Sorted_Success(t *testing.T) {
	fs := afero.NewMemMapFs()

	require.NoError(t, afero.WriteFile(fs, "/archive.tar.gz", createTar([]string{"z.txt", "a.txt", "dir/"}), 0o644))

	var stdoutBuf bytes.Buffer

	prog := NewProgram(fs, &stdoutBuf, io.Discard, nil, nil)
	require.NoError(t, prog.List(t.Context(), "/archive.tar.gz", true))

	paths := strings.Split(strings.TrimSpace(stdoutBuf.String()), "\n")
	require.Equal(t, []string{"a.txt", "dir/", "z.txt"}, paths)
}

// Expectation: An unsorted list should be produced on standard output (stdout).
func Test_Program_List_Unsorted_Success(t *testing.T) {
	fs := afero.NewMemMapFs()

	require.NoError(t, afero.WriteFile(fs, "/archive.tar.gz", createTar([]string{"z.txt", "a.txt", "dir/"}), 0o644))

	var stdoutBuf bytes.Buffer

	prog := NewProgram(fs, &stdoutBuf, io.Discard, nil, nil)
	require.NoError(t, prog.List(t.Context(), "/archive.tar.gz", false))

	paths := strings.Split(strings.TrimSpace(stdoutBuf.String()), "\n")
	require.Equal(t, []string{"z.txt", "a.txt", "dir/"}, paths)
}

// Expectation: A context cancellation should be respected.
func Test_Program_List_CtxCancel_Error(t *testing.T) {
	fs := afero.NewMemMapFs()

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	require.NoError(t, afero.WriteFile(fs, "/archive.tar.gz", createTar([]string{"a.txt", "b.txt"}), 0o644))

	var stdoutBuf, stderrBuf bytes.Buffer

	prog := NewProgram(fs, &stdoutBuf, &stderrBuf, nil, nil)
	require.ErrorIs(t, prog.List(ctx, "/archive.tar.gz", false), context.Canceled)
}
