package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

// A helper filesystem whose newly created files fail every write (e.g. disk full).
type fullDiskFs struct {
	afero.Fs
}

type fullDiskFile struct {
	afero.File
}

func (f fullDiskFile) Write(p []byte) (int, error) {
	return 0, errors.New("simulated disk full")
}

func (e fullDiskFs) OpenFile(name string, flag int, perm os.FileMode) (afero.File, error) {
	f, err := e.Fs.OpenFile(name, flag, perm)
	if err != nil || flag&os.O_CREATE == 0 {
		return f, err //nolint:wrapcheck
	}

	return fullDiskFile{f}, nil
}

// Expectation: A write failure while flushing the archive must be reported, not swallowed.
func Test_Program_Create_FlushError_Reported(t *testing.T) {
	base := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(base, "/src/a.txt", nil, 0o644))
	fs := fullDiskFs{base}

	prog := NewProgram(fs, &bytes.Buffer{}, &bytes.Buffer{}, nil, nil)

	err := prog.Create(t.Context(), "/src", "/out.tar.gz", nil)
	require.ErrorContains(t, err, "simulated disk full")

	exists, _ := afero.Exists(fs, "/out.tar.gz")
	require.False(t, exists, "no broken archive must be left behind")
}

// Expectation: A write failure while flushing the diff must be reported, not swallowed.
func Test_Program_Diff_FlushError_Reported(t *testing.T) {
	base := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(base, "/old/a.txt", nil, 0o644))
	require.NoError(t, afero.WriteFile(base, "/new/b.txt", nil, 0o644))
	fs := fullDiskFs{base}

	prog := NewProgram(fs, &bytes.Buffer{}, &bytes.Buffer{}, nil, nil)

	_, err := prog.Diff(t.Context(), "/old", "/new", "/diff.tar.gz", nil)
	require.ErrorContains(t, err, "simulated disk full")
	require.NotErrorIs(t, err, ErrDiffsFound)
}

// Expectation: A failed create must not destroy a pre-existing file at the output path.
func Test_Program_Create_Failure_PreservesExistingOutput(t *testing.T) {
	fs := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(fs, "/out.tar.gz", []byte("previous backup"), 0o644))

	prog := NewProgram(fs, &bytes.Buffer{}, &bytes.Buffer{}, nil, nil)

	require.Error(t, prog.Create(t.Context(), "/does-not-exist", "/out.tar.gz", nil))

	data, err := afero.ReadFile(fs, "/out.tar.gz")
	require.NoError(t, err)
	require.Equal(t, "previous backup", string(data))
}

// Expectation: A diff without differences must not destroy a pre-existing file at the output path.
func Test_Program_Diff_NoDiffs_PreservesExistingOutput(t *testing.T) {
	fs := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(fs, "/old/a.txt", nil, 0o644))
	require.NoError(t, afero.WriteFile(fs, "/new/a.txt", nil, 0o644))
	require.NoError(t, afero.WriteFile(fs, "/diff.tar.gz", []byte("previous diff"), 0o644))

	prog := NewProgram(fs, &bytes.Buffer{}, &bytes.Buffer{}, nil, nil)

	_, err := prog.Diff(t.Context(), "/old", "/new", "/diff.tar.gz", nil)
	require.NoError(t, err)

	data, err := afero.ReadFile(fs, "/diff.tar.gz")
	require.NoError(t, err)
	require.Equal(t, "previous diff", string(data))
}

// Expectation: The output file must not archive itself when placed inside the input tree.
func Test_Program_Create_OutputInsideInput_NotArchived(t *testing.T) {
	fs := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(fs, "/src/a.txt", nil, 0o644))
	require.NoError(t, afero.WriteFile(fs, "/src/out.tar.gz", []byte("previous"), 0o644))

	var stdout bytes.Buffer
	prog := NewProgram(fs, &stdout, &bytes.Buffer{}, nil, nil)

	require.NoError(t, prog.Create(t.Context(), "/src", "/src/out.tar.gz", nil))
	require.Equal(t, "a.txt\n", stdout.String())
}

// Expectation: The output file must get the same permissions as a plain os.Create would give it.
func Test_Program_Create_OutputPermissions_MatchOsCreate(t *testing.T) {
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
