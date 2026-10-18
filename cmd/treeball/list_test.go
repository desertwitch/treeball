package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"slices"
	"strings"
	"testing"

	"github.com/lanrat/extsort"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

// A helper function for tests to list the paths of a tarball (sorted or in archive order).
func listArchive(t *testing.T, fs afero.Fs, archive string, sorted bool, excludes []string, cfg *extsort.Config) []string {
	t.Helper()

	var out bytes.Buffer
	require.NoError(t, NewProgram(fs, &out, &bytes.Buffer{}, nil, cfg).List(t.Context(), archive, sorted, excludes))

	if out.Len() == 0 {
		return nil
	}

	return strings.Split(strings.TrimSuffix(out.String(), "\n"), "\n")
}

// Expectation: A sorted list should be produced on standard output (stdout).
func Test_Program_List_Sorted_Success(t *testing.T) {
	fs := afero.NewMemMapFs()

	require.NoError(t, afero.WriteFile(fs, "/archive.tar.gz", createTar([]string{"z.txt", "a.txt", "dir/"}), 0o644))

	var stdoutBuf bytes.Buffer

	prog := NewProgram(fs, &stdoutBuf, io.Discard, nil, nil)
	require.NoError(t, prog.List(t.Context(), "/archive.tar.gz", true, nil))

	paths := strings.Split(strings.TrimSpace(stdoutBuf.String()), "\n")
	require.Equal(t, []string{"a.txt", "dir/", "z.txt"}, paths)
}

// Expectation: A sorted list should be produced and the exclude patterns respected.
func Test_Program_List_Sorted_Excludes_Success(t *testing.T) {
	fs := afero.NewMemMapFs()

	require.NoError(t, afero.WriteFile(fs, "/archive.tar.gz", createTar([]string{"z.txt", "a.txt", "y.txt", "dir/"}), 0o644))

	var stdoutBuf bytes.Buffer

	prog := NewProgram(fs, &stdoutBuf, io.Discard, nil, nil)
	require.NoError(t, prog.List(t.Context(), "/archive.tar.gz", true, []string{"y.txt"}))

	paths := strings.Split(strings.TrimSpace(stdoutBuf.String()), "\n")
	require.Equal(t, []string{"a.txt", "dir/", "z.txt"}, paths)
}

// Expectation: A sorted list should be correct even when sorting spills to disk and merges many chunks.
func Test_Program_List_Sorted_SpillToDisk_Success(t *testing.T) {
	entries := make([]string, 0, 2000)
	for i := range 2000 { // unique names, scattered across 40 dirs in non-sorted order
		entries = append(entries, fmt.Sprintf("d%02d/f%05d", (i*31)%40, (i*7919)%100000))
	}

	fs := afero.NewMemMapFs()
	writeTree(t, fs, "/src", entries)
	require.NoError(t, NewProgram(fs, &bytes.Buffer{}, &bytes.Buffer{}, nil, nil).Create(t.Context(), "/src", "/src.tar.gz", nil))

	tiny := extSortConfigDefault
	tiny.ChunkSize = 7 // forces hundreds of on-disk chunks
	tiny.TempFilesDir = t.TempDir()

	unsorted := listArchive(t, fs, "/src.tar.gz", false, nil, nil)
	sorted := listArchive(t, fs, "/src.tar.gz", true, nil, &tiny)

	expected := slices.Clone(unsorted)
	slices.Sort(expected)
	require.Equal(t, expected, sorted)
}

// Expectation: An unsorted list should be produced on standard output (stdout).
func Test_Program_List_Unsorted_Success(t *testing.T) {
	fs := afero.NewMemMapFs()

	require.NoError(t, afero.WriteFile(fs, "/archive.tar.gz", createTar([]string{"z.txt", "a.txt", "dir/"}), 0o644))

	var stdoutBuf bytes.Buffer

	prog := NewProgram(fs, &stdoutBuf, io.Discard, nil, nil)
	require.NoError(t, prog.List(t.Context(), "/archive.tar.gz", false, nil))

	paths := strings.Split(strings.TrimSpace(stdoutBuf.String()), "\n")
	require.Equal(t, []string{"z.txt", "a.txt", "dir/"}, paths)
}

// Expectation: An unsorted list should be produced and the exclude patterns respected.
func Test_Program_List_Unsorted_Excludes_Success(t *testing.T) {
	fs := afero.NewMemMapFs()

	require.NoError(t, afero.WriteFile(fs, "/archive.tar.gz", createTar([]string{"z.txt", "y.txt", "a.txt", "dir/"}), 0o644))

	var stdoutBuf bytes.Buffer

	prog := NewProgram(fs, &stdoutBuf, io.Discard, nil, nil)
	require.NoError(t, prog.List(t.Context(), "/archive.tar.gz", false, []string{"y.txt"}))

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
	require.ErrorIs(t, prog.List(ctx, "/archive.tar.gz", false, nil), context.Canceled)
}
