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

// Expectation: An error should be thrown when the old path is not existent.
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
func Test_Program_Diff_InvalidCompressorConfig_Error(t *testing.T) {
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
