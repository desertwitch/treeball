package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/lanrat/extsort"
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

	_ = tw.Close()
	_ = gz.Close()

	return buf.Bytes()
}

// A helper function for tests to create a directory tree with empty dummy files ("x/" for a folder).
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

// Expectation: A difference between the tarballs should be found, the correct error returned and the output file exist.
func Test_Program_Diff_TarVsTar_DiffsFound_Success(t *testing.T) {
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
func Test_Program_Diff_TarVsTar_NoDiffsFound_Success(t *testing.T) {
	fs := afero.NewMemMapFs()

	require.NoError(t, afero.WriteFile(fs, "/old.tar.gz", createTar([]string{"a.txt", "b/", "b/x.txt"}), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/new.tar.gz", createTar([]string{"a.txt", "b/", "b/x.txt"}), 0o644))

	prog := NewProgram(fs, io.Discard, io.Discard, nil, nil)
	_, err := prog.Diff(t.Context(), "/old.tar.gz", "/new.tar.gz", "/diff.tar.gz", nil)
	require.NoError(t, err)

	_, err = fs.Stat("/diff.tar.gz")
	require.ErrorIs(t, err, os.ErrNotExist)
}

// Expectation: No differences found between two tarballs when exclusions are applied, and the output file removed.
func Test_Program_Diff_TarVsTar_NoDiffsFound_Excludes_Success(t *testing.T) {
	fs := afero.NewMemMapFs()

	oldTar := createTar([]string{
		"app/",
		"app/main.go",
		"app/internal/",
		"app/internal/util.go",
		"app/vendor/",
		"app/vendor/github.com/",
		"app/vendor/github.com/lib/",
		"app/vendor/github.com/lib/lib.go",
	})
	require.NoError(t, afero.WriteFile(fs, "/old.tar.gz", oldTar, 0o644))

	newTar := createTar([]string{
		"app/",
		"app/main.go",
		"app/internal/",
		"app/internal/util.go",
		"app/vendor/",
		"app/vendor/github.com/",
		"app/vendor/github.com/lib/",
		"app/vendor/github.com/lib/lib_v2.go",
	})
	require.NoError(t, afero.WriteFile(fs, "/new.tar.gz", newTar, 0o644))

	prog := NewProgram(fs, io.Discard, io.Discard, nil, nil)

	_, err := prog.Diff(t.Context(), "/old.tar.gz", "/new.tar.gz", "/diff.tar.gz", []string{"**/vendor/**"})
	require.NoError(t, err)

	_, err = fs.Stat("/diff.tar.gz")
	require.ErrorIs(t, err, os.ErrNotExist)
}

// Expectation: A difference between the tarball and folder should be found, the correct error returned and the output file exist.
func Test_Program_Diff_TarVsFolder_DiffsFound_Success(t *testing.T) {
	fs := afero.NewMemMapFs()

	require.NoError(t, afero.WriteFile(fs, "/cmpOld.tar.gz", createTar([]string{
		"a.txt",
		"b/",
		"b/x.txt",
	}), 0o644))

	require.NoError(t, fs.MkdirAll("/cmpNew/b", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/cmpNew/a.txt", []byte{}, 0o644))
	require.NoError(t, afero.WriteFile(fs, "/cmpNew/b/y.txt", []byte{}, 0o644))

	prog := NewProgram(fs, io.Discard, io.Discard, nil, nil)
	_, err := prog.Diff(t.Context(), "/cmpOld.tar.gz", "/cmpNew", "/diff.tar.gz", nil)
	require.ErrorIs(t, err, ErrDiffsFound)

	f, err := fs.Open("/diff.tar.gz")
	require.NoError(t, err)

	gzr, err := gzip.NewReader(f)
	require.NoError(t, err)

	tr := tar.NewReader(gzr)
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

// Expectation: No difference between the tarball and folder should be found, no error returned and the output file removed.
func Test_Program_Diff_TarVsFolder_NoDiffsFound_Success(t *testing.T) {
	fs := afero.NewMemMapFs()

	require.NoError(t, afero.WriteFile(fs, "/cmpOld.tar.gz", createTar([]string{
		"a.txt",
		"b/",
		"b/x.txt",
	}), 0o644))

	require.NoError(t, fs.MkdirAll("/cmpNew/b", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/cmpNew/a.txt", []byte{}, 0o644))
	require.NoError(t, afero.WriteFile(fs, "/cmpNew/b/x.txt", []byte{}, 0o644))

	prog := NewProgram(fs, io.Discard, io.Discard, nil, nil)
	_, err := prog.Diff(t.Context(), "/cmpOld.tar.gz", "/cmpNew", "/diff.tar.gz", nil)
	require.NoError(t, err)

	_, err = fs.Stat("/diff.tar.gz")
	require.ErrorIs(t, err, os.ErrNotExist)
}

// Expectation: No differences found between tarball and folder when exclusions are applied, and the output file removed.
func Test_Program_Diff_TarVsFolder_NoDiffsFound_Excludes_Success(t *testing.T) {
	fs := afero.NewMemMapFs()

	require.NoError(t, afero.WriteFile(fs, "/cmpOld.tar.gz", createTar([]string{
		"app/",
		"app/main.go",
		"app/internal/",
		"app/internal/util.go",
		"app/vendor/",
		"app/vendor/github.com/",
		"app/vendor/github.com/lib/",
		"app/vendor/github.com/lib/lib.go",
	}), 0o644))

	require.NoError(t, fs.MkdirAll("/cmpNew/app/internal", 0o755))
	require.NoError(t, fs.MkdirAll("/cmpNew/app/vendor/github.com/lib", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/cmpNew/app/main.go", []byte{}, 0o644))
	require.NoError(t, afero.WriteFile(fs, "/cmpNew/app/internal/util.go", []byte{}, 0o644))
	require.NoError(t, afero.WriteFile(fs, "/cmpNew/app/vendor/github.com/lib/lib_v2.go", []byte{}, 0o644))

	prog := NewProgram(fs, io.Discard, io.Discard, nil, nil)
	_, err := prog.Diff(t.Context(), "/cmpOld.tar.gz", "/cmpNew", "/diff.tar.gz", []string{"**/vendor/**"})
	require.NoError(t, err)

	_, err = fs.Stat("/diff.tar.gz")
	require.ErrorIs(t, err, os.ErrNotExist)
}

// Expectation: A difference between the folder and tarball should be found, the correct error returned and the output file exist.
func Test_Program_Diff_FolderVsTar_DiffsFound_Success(t *testing.T) {
	fs := afero.NewMemMapFs()

	require.NoError(t, fs.MkdirAll("/cmpOld/b", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/cmpOld/a.txt", []byte{}, 0o644))
	require.NoError(t, afero.WriteFile(fs, "/cmpOld/b/x.txt", []byte{}, 0o644))

	require.NoError(t, afero.WriteFile(fs, "/cmpNew.tar.gz", createTar([]string{
		"a.txt",
		"b/",
		"b/y.txt",
	}), 0o644))

	prog := NewProgram(fs, io.Discard, io.Discard, nil, nil)
	_, err := prog.Diff(t.Context(), "/cmpOld", "/cmpNew.tar.gz", "/diff.tar.gz", nil)
	require.ErrorIs(t, err, ErrDiffsFound)

	f, err := fs.Open("/diff.tar.gz")
	require.NoError(t, err)

	gzr, err := gzip.NewReader(f)
	require.NoError(t, err)

	tr := tar.NewReader(gzr)
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

// Expectation: No difference between the folder and tarball should be found, no error returned and the output file removed.
func Test_Program_Diff_FolderVsTar_NoDiffsFound_Success(t *testing.T) {
	fs := afero.NewMemMapFs()

	require.NoError(t, fs.MkdirAll("/cmpOld/b", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/cmpOld/a.txt", []byte{}, 0o644))
	require.NoError(t, afero.WriteFile(fs, "/cmpOld/b/x.txt", []byte{}, 0o644))

	require.NoError(t, afero.WriteFile(fs, "/cmpNew.tar.gz", createTar([]string{
		"a.txt",
		"b/",
		"b/x.txt",
	}), 0o644))

	prog := NewProgram(fs, io.Discard, io.Discard, nil, nil)
	_, err := prog.Diff(t.Context(), "/cmpOld", "/cmpNew.tar.gz", "/diff.tar.gz", nil)
	require.NoError(t, err)

	_, err = fs.Stat("/diff.tar.gz")
	require.ErrorIs(t, err, os.ErrNotExist)
}

// Expectation: No differences found between folder and tarball when exclusions are applied, and the output file removed.
func Test_Program_Diff_FolderVsTar_NoDiffsFound_Excludes_Success(t *testing.T) {
	fs := afero.NewMemMapFs()

	require.NoError(t, fs.MkdirAll("/cmpOld/app/internal", 0o755))
	require.NoError(t, fs.MkdirAll("/cmpOld/app/vendor/github.com/lib", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/cmpOld/app/main.go", []byte{}, 0o644))
	require.NoError(t, afero.WriteFile(fs, "/cmpOld/app/internal/util.go", []byte{}, 0o644))
	require.NoError(t, afero.WriteFile(fs, "/cmpOld/app/vendor/github.com/lib/lib.go", []byte{}, 0o644))

	require.NoError(t, afero.WriteFile(fs, "/cmpNew.tar.gz", createTar([]string{
		"app/",
		"app/main.go",
		"app/internal/",
		"app/internal/util.go",
		"app/vendor/",
		"app/vendor/github.com/",
		"app/vendor/github.com/lib/",
		"app/vendor/github.com/lib/lib_v2.go",
	}), 0o644))

	prog := NewProgram(fs, io.Discard, io.Discard, nil, nil)
	_, err := prog.Diff(t.Context(), "/cmpOld", "/cmpNew.tar.gz", "/diff.tar.gz", []string{"**/vendor/**"})
	require.NoError(t, err)

	_, err = fs.Stat("/diff.tar.gz")
	require.ErrorIs(t, err, os.ErrNotExist)
}

// Expectation: A difference between the folders should be found, the correct error returned and the output file exist.
func Test_Program_Diff_FolderVsFolder_DiffsFound_Success(t *testing.T) {
	fs := afero.NewMemMapFs()

	require.NoError(t, fs.MkdirAll("/old/b", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/old/a.txt", []byte{}, 0o644))
	require.NoError(t, afero.WriteFile(fs, "/old/b/x.txt", []byte{}, 0o644))

	require.NoError(t, fs.MkdirAll("/new/b", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/new/a.txt", []byte{}, 0o644))
	require.NoError(t, afero.WriteFile(fs, "/new/b/y.txt", []byte{}, 0o644))

	prog := NewProgram(fs, io.Discard, io.Discard, nil, nil)
	_, err := prog.Diff(t.Context(), "/old", "/new", "/diff.tar.gz", nil)
	require.ErrorIs(t, err, ErrDiffsFound)

	f, err := fs.Open("/diff.tar.gz")
	require.NoError(t, err)

	gzr, err := gzip.NewReader(f)
	require.NoError(t, err)

	tr := tar.NewReader(gzr)
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

// Expectation: No difference between the folders should be found, no error returned and the output file removed.
func Test_Program_Diff_FolderVsFolder_NoDiffsFound_Success(t *testing.T) {
	fs := afero.NewMemMapFs()

	require.NoError(t, fs.MkdirAll("/old/b", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/old/a.txt", []byte{}, 0o644))
	require.NoError(t, afero.WriteFile(fs, "/old/b/x.txt", []byte{}, 0o644))

	require.NoError(t, fs.MkdirAll("/new/b", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/new/a.txt", []byte{}, 0o644))
	require.NoError(t, afero.WriteFile(fs, "/new/b/x.txt", []byte{}, 0o644))

	prog := NewProgram(fs, io.Discard, io.Discard, nil, nil)
	_, err := prog.Diff(t.Context(), "/old", "/new", "/diff.tar.gz", nil)
	require.NoError(t, err)

	_, err = fs.Stat("/diff.tar.gz")
	require.ErrorIs(t, err, os.ErrNotExist)
}

// Expectation: No differences found between folders when exclusions are applied, and the output file removed.
func Test_Program_Diff_FolderVsFolder_NoDiffsFound_Excludes_Success(t *testing.T) {
	fs := afero.NewMemMapFs()

	require.NoError(t, fs.MkdirAll("/old/app/internal", 0o755))
	require.NoError(t, fs.MkdirAll("/old/app/vendor/github.com/lib", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/old/app/main.go", []byte{}, 0o644))
	require.NoError(t, afero.WriteFile(fs, "/old/app/internal/util.go", []byte{}, 0o644))
	require.NoError(t, afero.WriteFile(fs, "/old/app/vendor/github.com/lib/lib.go", []byte{}, 0o644))

	require.NoError(t, fs.MkdirAll("/new/app/internal", 0o755))
	require.NoError(t, fs.MkdirAll("/new/app/vendor/github.com/lib", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/new/app/main.go", []byte{}, 0o644))
	require.NoError(t, afero.WriteFile(fs, "/new/app/internal/util.go", []byte{}, 0o644))
	require.NoError(t, afero.WriteFile(fs, "/new/app/vendor/github.com/lib/lib_v2.go", []byte{}, 0o644))

	prog := NewProgram(fs, io.Discard, io.Discard, nil, nil)
	_, err := prog.Diff(t.Context(), "/old", "/new", "/diff.tar.gz", []string{"**/vendor/**"})
	require.NoError(t, err)

	_, err = fs.Stat("/diff.tar.gz")
	require.ErrorIs(t, err, os.ErrNotExist)
}

// Expectation: Diffing a tarball against the folder it was created from should never report differences,
// including names that need special tar encodings (long, non-UTF-8, spaces, nesting, empty dirs).
func Test_Program_Diff_RoundTrip_Success(t *testing.T) {
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

// Expectation: Diff results should be identical when sorting spills to disk and merges many chunks.
func Test_Program_Diff_SpillToDisk_Success(t *testing.T) {
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

// Expectation: The printed differences should be exactly those in the diff tarball, also beyond the output buffer size.
func Test_Program_Diff_StdoutMatchesArchive_Success(t *testing.T) {
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

// Expectation: A diff without differences should not destroy a pre-existing file at the output path.
func Test_Program_Diff_NoDiffsFound_PreservesExistingOutput_Success(t *testing.T) {
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

// Expectation: A device like /dev/null should be usable as output and never be replaced or removed.
func Test_Program_Diff_DevNullOutput_Success(t *testing.T) {
	if _, err := os.Stat(os.DevNull); err != nil || os.DevNull != "/dev/null" {
		t.Skip("no /dev/null on this platform")
	}

	dir := t.TempDir()
	writeTree(t, afero.NewOsFs(), dir, []string{"a/x", "b/x", "b/y"})
	a, b := filepath.Join(dir, "a"), filepath.Join(dir, "b")
	prog := NewProgram(afero.NewOsFs(), &bytes.Buffer{}, &bytes.Buffer{}, nil, nil)

	_, err := prog.Diff(t.Context(), a, b, os.DevNull, nil)
	require.ErrorIs(t, err, ErrDiffsFound)
	_, err = prog.Diff(t.Context(), a, a, os.DevNull, nil)
	require.NoError(t, err)
	_, err = prog.Diff(t.Context(), a, filepath.Join(dir, "missing"), os.DevNull, nil)
	require.Error(t, err)
	require.NotErrorIs(t, err, ErrDiffsFound)

	info, err := os.Lstat(os.DevNull)
	require.NoError(t, err)
	require.Equal(t, os.ModeDevice|os.ModeCharDevice, info.Mode().Type())
}

// Expectation: No temporary files should be left behind, whichever way the diff ends.
func Test_Program_Diff_NoTempFilesLeftBehind_Table(t *testing.T) {
	cancelled, cancel := context.WithCancel(t.Context())
	cancel()

	tests := []struct {
		name   string
		cmpNew string
		cancel bool
	}{
		{"diffs found", "/new", false},
		{"no diffs found", "/old", false},
		{"failure", "/missing", false},
		{"cancel", "/new", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := afero.NewMemMapFs()
			require.NoError(t, afero.WriteFile(fs, "/old/a.txt", nil, 0o644))
			require.NoError(t, afero.WriteFile(fs, "/new/b.txt", nil, 0o644))
			require.NoError(t, fs.MkdirAll("/out", 0o755))

			ctx := t.Context() //nolint:contextcheck
			if tt.cancel {
				ctx = cancelled
			}

			_, _ = NewProgram(fs, &bytes.Buffer{}, &bytes.Buffer{}, nil, nil).Diff(ctx, "/old", tt.cmpNew, "/out/d.tar.gz", nil)

			leftovers, err := afero.Glob(fs, "/out/.*.tmp")
			require.NoError(t, err)
			require.Empty(t, leftovers)
		})
	}
}

// Expectation: An error should be thrown when the old path is not existent.
func Test_Program_Diff_OldPathMissing_Stat_Error(t *testing.T) {
	fs := afero.NewMemMapFs()

	require.NoError(t, afero.WriteFile(fs, "/old.tar.gz", createTar([]string{"a.txt", "b/", "b/x.txt"}), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/new.tar.gz", createTar([]string{"a.txt", "b/", "b/x.txt"}), 0o644))

	prog := NewProgram(fs, io.Discard, io.Discard, nil, nil)
	_, err := prog.Diff(t.Context(), "/old1.tar.gz", "/new.tar.gz", "/diff.tar.gz", nil)

	require.Error(t, err)
	require.ErrorContains(t, err, "stat")

	_, err = fs.Stat("/diff.tar.gz")
	require.ErrorIs(t, err, os.ErrNotExist)
}

// Expectation: An error should be thrown when the new path is not existent.
func Test_Program_Diff_NewPathMissing_Stat_Error(t *testing.T) {
	fs := afero.NewMemMapFs()

	require.NoError(t, afero.WriteFile(fs, "/old.tar.gz", createTar([]string{"a.txt", "b/", "b/x.txt"}), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/new.tar.gz", createTar([]string{"a.txt", "b/", "b/x.txt"}), 0o644))

	prog := NewProgram(fs, io.Discard, io.Discard, nil, nil)
	_, err := prog.Diff(t.Context(), "/old.tar.gz", "/new1.tar.gz", "/diff.tar.gz", nil)

	require.Error(t, err)
	require.ErrorContains(t, err, "stat")

	_, err = fs.Stat("/diff.tar.gz")
	require.ErrorIs(t, err, os.ErrNotExist)
}

// Expectation: An invalid exclude pattern should produce an error.
func Test_Program_Diff_InvalidExcludePattern_Error(t *testing.T) {
	fs := afero.NewMemMapFs()

	require.NoError(t, afero.WriteFile(fs, "/old.tar.gz", createTar([]string{"a.txt", "b/", "b/x.txt"}), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/new.tar.gz", createTar([]string{"a.txt", "b/", "b/y.txt"}), 0o644))

	prog := NewProgram(fs, io.Discard, io.Discard, nil, nil)
	_, err := prog.Diff(t.Context(), "/old.tar.gz", "/new.tar.gz", "/diff.tar.gz", []string{"a["})

	require.Error(t, err)
	require.ErrorContains(t, err, "exclude")
}

// Expectation: An invalid configuration should raise the appropriate error and the output file be removed.
func Test_Program_Diff_InvalidCompressorCompLevel_Error(t *testing.T) {
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

// Expectation: A write failure while flushing the diff tarball should be reported, not swallowed.
func Test_Program_Diff_FlushFails_Error(t *testing.T) {
	base := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(base, "/old/a.txt", nil, 0o644))
	require.NoError(t, afero.WriteFile(base, "/new/b.txt", nil, 0o644))
	fs := fullDiskFs{base}

	prog := NewProgram(fs, &bytes.Buffer{}, &bytes.Buffer{}, nil, nil)

	_, err := prog.Diff(t.Context(), "/old", "/new", "/diff.tar.gz", nil)
	require.ErrorContains(t, err, "simulated disk full")
	require.NotErrorIs(t, err, ErrDiffsFound)
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
