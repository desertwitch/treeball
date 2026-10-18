package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

// Expectation: No temporary files may be left behind, whatever way a run ends.
func Test_Program_NoTempFilesLeftBehind(t *testing.T) {
	cancelled, cancel := context.WithCancel(t.Context())
	cancel()

	runs := map[string]func(prog *Program) error{
		"create success": func(p *Program) error { return p.Create(t.Context(), "/old", "/out/a.tar.gz", nil) },
		"create failure": func(p *Program) error { return p.Create(t.Context(), "/missing", "/out/a.tar.gz", nil) },
		"create cancel":  func(p *Program) error { return p.Create(cancelled, "/old", "/out/a.tar.gz", nil) },
		"diff diffs": func(p *Program) error {
			_, err := p.Diff(t.Context(), "/old", "/new", "/out/d.tar.gz", nil)

			return err
		},
		"diff no diffs": func(p *Program) error {
			_, err := p.Diff(t.Context(), "/old", "/old", "/out/d.tar.gz", nil)

			return err
		},
		"diff failure": func(p *Program) error {
			_, err := p.Diff(t.Context(), "/old", "/missing", "/out/d.tar.gz", nil)

			return err
		},
		"diff cancel": func(p *Program) error {
			_, err := p.Diff(cancelled, "/old", "/new", "/out/d.tar.gz", nil)

			return err
		},
	}

	for name, run := range runs {
		t.Run(name, func(t *testing.T) {
			fs := afero.NewMemMapFs()
			require.NoError(t, afero.WriteFile(fs, "/old/a.txt", nil, 0o644))
			require.NoError(t, afero.WriteFile(fs, "/new/b.txt", nil, 0o644))
			require.NoError(t, fs.MkdirAll("/out", 0o755))

			_ = run(NewProgram(fs, &bytes.Buffer{}, &bytes.Buffer{}, nil, nil))

			leftovers, err := afero.Glob(fs, "/out/.*.tmp")
			require.NoError(t, err)
			require.Empty(t, leftovers)
		})
	}
}

// Expectation: A successful run replaces an existing output file on a real filesystem.
func Test_Program_Create_ReplacesExistingOutput_OsFs(t *testing.T) {
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

// Expectation: What create prints must be exactly what lands in the archive, also past the stdout buffer size.
func Test_Program_Create_StdoutMatchesArchive(t *testing.T) {
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

// Expectation: What diff prints must be exactly what lands in the diff archive, also past the stdout buffer size.
func Test_Program_Diff_StdoutMatchesArchive(t *testing.T) {
	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/old", 0o755))
	for i := range 5000 {
		require.NoError(t, afero.WriteFile(fs, fmt.Sprintf("/new/file-%05d.txt", i), nil, 0o644))
	}

	var diffed, listed bytes.Buffer
	_, err := NewProgram(fs, &diffed, &bytes.Buffer{}, nil, nil).Diff(t.Context(), "/old", "/new", "/d.tar.gz", nil)
	require.ErrorIs(t, err, ErrDiffsFound)
	require.NoError(t, NewProgram(fs, &listed, &bytes.Buffer{}, nil, nil).List(t.Context(), "/d.tar.gz", false, nil))

	require.Greater(t, diffed.Len(), 64<<10)
	require.Equal(t, strings.ReplaceAll(listed.String(), "+++/", "+++ "), diffed.String())
}

// Expectation: Only targets truly inside root resolve; look-alike siblings and root itself do not.
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

// Expectation: Relative and absolute spellings of the same location are recognized as such.
func Test_relToRoot_RelativePaths(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	require.Equal(t, "out.tar.gz", relToRoot("src", filepath.Join(dir, "src", "out.tar.gz")))
	require.Equal(t, "out.tar.gz", relToRoot(filepath.Join(dir, "src"), "./src/out.tar.gz"))
	require.Empty(t, relToRoot("src", "out.tar.gz"))
}
