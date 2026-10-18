package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/lanrat/extsort"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

// writeTree creates the given entries below root ("x/" = directory, else empty file).
func writeTree(t *testing.T, fs afero.Fs, root string, entries []string) {
	t.Helper()

	for _, e := range entries {
		p := filepath.Join(root, e)
		if strings.HasSuffix(e, "/") {
			require.NoError(t, fs.MkdirAll(p, 0o755))

			continue
		}
		require.NoError(t, fs.MkdirAll(filepath.Dir(p), 0o755))
		require.NoError(t, afero.WriteFile(fs, p, nil, 0o644))
	}
}

// listArchive returns the archive's entries in archive order.
func listArchive(t *testing.T, fs afero.Fs, archive string, sorted bool, excludes []string, cfg *extsort.Config) []string {
	t.Helper()

	var out bytes.Buffer
	require.NoError(t, NewProgram(fs, &out, &bytes.Buffer{}, nil, cfg).List(t.Context(), archive, sorted, excludes))

	if out.Len() == 0 {
		return nil
	}

	return strings.Split(strings.TrimSuffix(out.String(), "\n"), "\n")
}

// Expectation: Diffing a tarball against the folder it was created from never reports differences,
// including names that need special tar encodings (long, non-UTF-8, spaces, nesting, empty dirs).
func Test_Behavior_CreateThenDiff_RoundTrip(t *testing.T) {
	long := strings.Repeat("x", 150)

	fs := afero.NewMemMapFs()
	writeTree(t, fs, "/src", []string{
		"plain.mkv",
		"with space/and more.mkv",
		long + "/" + long + ".mkv",
		"caf\xe9 latin1.mkv",
		"caf\xe9-" + long + ".mkv",
		"empty/nested/",
		"a-b/f", "a.b/f", "a/f", // sort-order neighbours of "a/"
	})

	prog := NewProgram(fs, &bytes.Buffer{}, &bytes.Buffer{}, nil, nil)
	require.NoError(t, prog.Create(t.Context(), "/src", "/src.tar.gz", nil))

	for _, pair := range [][2]string{{"/src.tar.gz", "/src"}, {"/src", "/src.tar.gz"}, {"/src.tar.gz", "/src.tar.gz"}} {
		var out bytes.Buffer
		res, err := NewProgram(fs, &out, &bytes.Buffer{}, nil, nil).Diff(t.Context(), pair[0], pair[1], "/d.tar.gz", nil)
		require.NoError(t, err, "diff %s %s:\n%s", pair[0], pair[1], out.String())
		require.Positive(t, res.Common)
	}
}

// Expectation: Symlinks are recorded as entries (not followed) and round-trip cleanly on a real filesystem.
func Test_Behavior_Symlinks_RoundTrip_OsFs(t *testing.T) {
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

// Expectation: When sorting spills to disk and merges many chunks, results are identical to in-memory sorting.
func Test_Behavior_ExternalSort_SpillToDisk(t *testing.T) {
	var oldEntries, newEntries []string //nolint:prealloc
	for i := range 2000 {               // unique names, scattered across 40 dirs in non-sorted order
		name := fmt.Sprintf("d%02d/f%05d", (i*31)%40, (i*7919)%100000)
		oldEntries = append(oldEntries, name)
		if i%7 != 0 { // remove every 7th file (never empties a whole dir)
			newEntries = append(newEntries, name)
		}
	}
	newEntries = append(newEntries, "added/one", "added/two")

	fs := afero.NewMemMapFs()
	writeTree(t, fs, "/old", oldEntries)
	writeTree(t, fs, "/new", newEntries)
	require.NoError(t, NewProgram(fs, &bytes.Buffer{}, &bytes.Buffer{}, nil, nil).Create(t.Context(), "/old", "/old.tar.gz", nil))

	tiny := extSortConfigDefault
	tiny.ChunkSize = 7 // forces hundreds of on-disk chunks
	tiny.TempFilesDir = t.TempDir()

	// list: sorted output must be a correct sort of the unsorted output
	unsorted := listArchive(t, fs, "/old.tar.gz", false, nil, nil)
	sorted := listArchive(t, fs, "/old.tar.gz", true, nil, &tiny)
	expected := slices.Clone(unsorted)
	slices.Sort(expected)
	require.Equal(t, expected, sorted)

	// diff: spilled and in-memory sorting must produce byte-identical results
	run := func(cfg *extsort.Config) string {
		var out bytes.Buffer
		_, err := NewProgram(fs, &out, &bytes.Buffer{}, nil, cfg).Diff(t.Context(), "/old.tar.gz", "/new", "/d.tar.gz", nil)
		require.ErrorIs(t, err, ErrDiffsFound)

		return out.String()
	}
	inMemory, spilled := run(nil), run(&tiny)
	require.Equal(t, inMemory, spilled)
	require.Equal(t, 286, strings.Count(spilled, "--- ")) // ceil(2000/7)
	require.Equal(t, "+++ added/\n+++ added/one\n+++ added/two\n", strings.Join(slices.DeleteFunc(
		strings.SplitAfter(spilled, "\n"), func(l string) bool { return !strings.HasPrefix(l, "+++ ") }), ""))
}

// Expectation: An exclude pattern selects the same entries whether the source is a folder or a tarball,
// so diffing a tarball against the folder it came from stays clean under any exclude.
func Test_Behavior_Excludes_FolderAndTarballAgree(t *testing.T) {
	fs := afero.NewMemMapFs()
	writeTree(t, fs, "/src", []string{
		"movies/m.mkv",
		"movies/m.nfo",
		"@eaDir/thumbs/t.jpg",
		"movies/@eaDir/x.jpg",
		"tv/show/s01/e01.mkv",
		"tv/show/s01/e01.nfo",
		"tv/@eaDir", // a FILE named like the excluded dir
	})
	require.NoError(t, NewProgram(fs, &bytes.Buffer{}, &bytes.Buffer{}, nil, nil).Create(t.Context(), "/src", "/full.tar.gz", nil))

	patterns := []string{
		"@eaDir", "@eaDir/", "**/@eaDir", "**/@eaDir/", "**/@eaDir/**",
		"**/*.nfo", "tv/show", "tv/*/s01/", "movies/**",
	}

	for _, pattern := range patterns {
		t.Run(pattern, func(t *testing.T) {
			excl := []string{pattern}

			// folder side: what create archives with the exclude
			require.NoError(t, NewProgram(fs, &bytes.Buffer{}, &bytes.Buffer{}, nil, nil).Create(t.Context(), "/src", "/excl.tar.gz", excl))
			fromFolder := listArchive(t, fs, "/excl.tar.gz", true, nil, nil)

			// tarball side: what list shows from the full archive with the same exclude
			fromTarball := listArchive(t, fs, "/full.tar.gz", true, excl, nil)

			require.Equal(t, fromFolder, fromTarball)

			var out bytes.Buffer
			_, err := NewProgram(fs, &out, &bytes.Buffer{}, nil, nil).Diff(t.Context(), "/full.tar.gz", "/src", "/d.tar.gz", excl)
			require.NoError(t, err, "phantom differences:\n%s", out.String())
		})
	}
}

// Expectation: The documented exit codes (0 success, 1 differences, 2 failure) are what the process returns.
func Test_Behavior_ExitCodes(t *testing.T) {
	if os.Getenv("TREEBALL_TEST_RUN_MAIN") == "1" {
		os.Args = append([]string{"treeball"}, strings.Split(os.Getenv("TREEBALL_TEST_ARGS"), "\x1f")...)
		main()

		return
	}

	dir := t.TempDir()
	writeTree(t, afero.NewOsFs(), dir, []string{"a/x", "b/x", "b/y"})
	a, b := filepath.Join(dir, "a"), filepath.Join(dir, "b")

	tests := []struct {
		args []string
		want int
	}{
		{[]string{"diff", a, a, filepath.Join(dir, "d1.tar.gz")}, exitCodeSuccess},
		{[]string{"diff", a, b, filepath.Join(dir, "d2.tar.gz")}, exitCodeDiffsFound},
		{[]string{"create", filepath.Join(dir, "missing"), filepath.Join(dir, "o.tar.gz")}, exitCodeFailure},
		{[]string{"list"}, exitCodeFailure},
		{[]string{"create", a, filepath.Join(dir, "o.tar.gz")}, exitCodeSuccess},
	}

	for _, tt := range tests {
		cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^Test_Behavior_ExitCodes$") //nolint:gosec
		cmd.Env = append(os.Environ(), "TREEBALL_TEST_RUN_MAIN=1", "TREEBALL_TEST_ARGS="+strings.Join(tt.args, "\x1f"))

		err := cmd.Run()

		code := 0
		if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
			code = exitErr.ExitCode()
		} else {
			require.NoError(t, err)
		}
		require.Equal(t, tt.want, code, "treeball %v", tt.args)
	}
}
