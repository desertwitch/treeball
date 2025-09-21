package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

// A helper writer for tests to simulate writing errors.
type errorWriter struct{}

// A helper function for tests to simulate writing failure.
func (errorWriter) Write(p []byte) (int, error) {
	return 0, errors.New("simulated write failure")
}

// Expectation: The function should stream paths from a directory using fsPathStream.
func Test_Program_multiPathStream_Dir_Success(t *testing.T) {
	fs := afero.NewMemMapFs()

	require.NoError(t, fs.MkdirAll("/project/assets", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/project/a.txt", []byte("a"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/project/assets/b.txt", []byte("b"), 0o644))

	prog := NewProgram(fs, io.Discard, io.Discard, nil, nil)
	paths, errs, err := prog.multiPathStream(t.Context(), "/project", true, []string{"assets/b.txt"})

	require.NoError(t, err)
	require.NotNil(t, paths)
	require.NotNil(t, errs)

	got := []string{}
	for p := range paths {
		got = append(got, p)
	}

	for err := range errs {
		require.NoError(t, err)
	}

	require.Equal(t, []string{"a.txt", "assets/"}, got)
}

// Expectation: The function should stream paths from a tar using tarPathStream.
func Test_Program_multiPathStream_File_Success(t *testing.T) {
	fs := afero.NewMemMapFs()

	tarData := createTar([]string{"alpha.txt", "zeta/", "zeta/beta.txt"})
	require.NoError(t, afero.WriteFile(fs, "/archive.tar.gz", tarData, 0o644))

	prog := NewProgram(fs, io.Discard, io.Discard, nil, nil)
	paths, errs, err := prog.multiPathStream(t.Context(), "/archive.tar.gz", true, nil)

	require.NoError(t, err)
	require.NotNil(t, paths)
	require.NotNil(t, errs)

	got := []string{}
	for p := range paths {
		got = append(got, p)
	}

	for err := range errs {
		require.NoError(t, err)
	}

	require.Equal(t, []string{"alpha.txt", "zeta/", "zeta/beta.txt"}, got)
}

// Expectation: The function should return an error if the input path cannot be stat'ed.
func Test_Program_multiPathStream_Stat_Error(t *testing.T) {
	fs := afero.NewMemMapFs()

	prog := NewProgram(fs, io.Discard, io.Discard, nil, nil)
	paths, errs, err := prog.multiPathStream(t.Context(), "/missing", false, nil)

	require.Error(t, err)
	require.Nil(t, paths)
	require.Nil(t, errs)

	require.Contains(t, err.Error(), "failed to stat")
}

// Expecation: The channels should contain the correct ordered paths and no errors.
func Test_Program_tarPathStream_Sorted_Success(t *testing.T) {
	fs := afero.NewMemMapFs()

	tarData := createTar([]string{"z.txt", "b/", "b/c.txt"})
	require.NoError(t, afero.WriteFile(fs, "/archive.tar.gz", tarData, 0o644))

	prog := NewProgram(fs, io.Discard, io.Discard, nil, nil)
	paths, errs := prog.tarPathStream(t.Context(), "/archive.tar.gz", true, nil)

	got := []string{}
	for p := range paths {
		got = append(got, p)
	}

	for err := range errs {
		require.NoError(t, err)
	}

	require.Equal(t, []string{"b/", "b/c.txt", "z.txt"}, got)
}

// Expecation: The channels should contain the correct ordered paths and no errors.
func Test_Program_tarPathStream_Unsorted_Success(t *testing.T) {
	fs := afero.NewMemMapFs()

	tarData := createTar([]string{"z.txt", "b/", "b/c.txt"})
	require.NoError(t, afero.WriteFile(fs, "/archive.tar.gz", tarData, 0o644))

	prog := NewProgram(fs, io.Discard, io.Discard, nil, nil)
	paths, errs := prog.tarPathStream(t.Context(), "/archive.tar.gz", false, nil)

	got := []string{}
	for p := range paths {
		got = append(got, p)
	}

	for err := range errs {
		require.NoError(t, err)
	}

	require.Equal(t, []string{"z.txt", "b/", "b/c.txt"}, got)
}

// Expecation: The channels should contain the correct error and no paths.
func Test_Program_tarPathStream_Open_Error(t *testing.T) {
	baseFs := afero.NewMemMapFs()

	require.NoError(t, afero.WriteFile(baseFs, "/archive.tar.gz", []byte("test"), 0o644))

	fs := errorFs{Fs: baseFs}

	prog := NewProgram(fs, io.Discard, io.Discard, nil, nil)
	paths, errs := prog.tarPathStream(t.Context(), "/archive.tar.gz", false, nil)

	for range paths {
		t.Fatal("should not emit paths")
	}

	select {
	case err := <-errs:
		require.Error(t, err)
		require.Contains(t, err.Error(), "simulated open failure")
	default:
		t.Fatal("expected error from tarPathStream")
	}
}

// Expecation: The channels should contain the correct error and no paths.
func Test_Program_tarPathStream_GzipDecode_Error(t *testing.T) {
	fs := afero.NewMemMapFs()

	require.NoError(t, afero.WriteFile(fs, "/archive.tar.gz", []byte("not a gzip file"), 0o644))

	prog := NewProgram(fs, io.Discard, io.Discard, nil, nil)
	paths, errs := prog.tarPathStream(t.Context(), "/archive.tar.gz", false, nil)

	for range paths {
		t.Fatal("should not emit any paths")
	}

	select {
	case err := <-errs:
		require.Error(t, err)
		require.Contains(t, err.Error(), "gzip")
	default:
		t.Fatal("expected gzip error from tarPathStream")
	}
}

// Expecation: The channels should contain the correct error and no paths.
func Test_Program_tarPathStream_TarDecode_Error(t *testing.T) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)

	_, err := gz.Write([]byte("not a valid tarball"))
	require.NoError(t, err)

	err = gz.Close()
	require.NoError(t, err)

	fs := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(fs, "/archive.tar.gz", buf.Bytes(), 0o644))

	prog := NewProgram(fs, io.Discard, io.Discard, nil, nil)
	paths, errs := prog.tarPathStream(t.Context(), "/archive.tar.gz", false, nil)

	for range paths {
		t.Fatal("should not emit any paths")
	}

	select {
	case err := <-errs:
		require.Error(t, err)
		require.Contains(t, err.Error(), "tar")
	default:
		t.Fatal("expected tar error from tarPathStream")
	}
}

// Expectation: Should cancel decompression when context is canceled.
func Test_Program_tarPathStream_CtxCancel_Error(t *testing.T) {
	fs := afero.NewMemMapFs()

	tarData := createTar([]string{"z.txt", "b/", "b/c.txt"})
	require.NoError(t, afero.WriteFile(fs, "/archive.tar.gz", tarData, 0o644))

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	prog := NewProgram(fs, io.Discard, io.Discard, nil, nil)
	paths, errs := prog.tarPathStream(ctx, "/archive.tar.gz", false, nil)

	for range paths {
		t.Fatal("should not emit any paths")
	}

	select {
	case err := <-errs:
		require.ErrorIs(t, err, context.Canceled)
	default:
		t.Fatal("expected ctx error from tarPathStream")
	}
}

// Expectation: tarPathStream should return an error when given an invalid exclude pattern.
func Test_Program_tarPathStream_InvalidExcludePattern_Error(t *testing.T) {
	fs := afero.NewMemMapFs()

	tarData := createTar([]string{"foo.txt"})
	require.NoError(t, afero.WriteFile(fs, "/archive.tar.gz", tarData, 0o644))

	prog := NewProgram(fs, io.Discard, io.Discard, nil, nil)
	paths, errs := prog.tarPathStream(t.Context(), "/archive.tar.gz", false, []string{"invalid["})

	for range paths {
		t.Fatal("should not emit any paths")
	}

	select {
	case err := <-errs:
		require.Error(t, err)
		require.Contains(t, err.Error(), "exclude")
	default:
		t.Fatal("expected error from tarPathStream")
	}
}

// Expectation: The channels should contain the correct ordered paths and no errors.
func Test_Program_fsPathStream_Sorted_Success(t *testing.T) {
	fs := afero.NewMemMapFs()

	require.NoError(t, fs.MkdirAll("/testdir/subdir", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/testdir/a.txt", []byte("a"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/testdir/subdir/b.txt", []byte("b"), 0o644))

	prog := NewProgram(fs, io.Discard, io.Discard, nil, nil)
	paths, errs := prog.fsPathStream(t.Context(), "/testdir", true, nil)

	got := []string{}
	for p := range paths {
		got = append(got, p)
	}

	for err := range errs {
		require.NoError(t, err)
	}

	require.Equal(t, []string{"a.txt", "subdir/", "subdir/b.txt"}, got)
}

// Expectation: Should return error if the filesystem walk fails.
func Test_Program_fsPathStream_WalkDir_Error(t *testing.T) {
	fs := afero.NewMemMapFs()

	require.NoError(t, afero.WriteFile(fs, "/somefile", []byte("data"), 0o644))

	prog := NewProgram(fs, io.Discard, io.Discard, nil, nil)
	prog.fsWalker = errorWalker{}

	paths, errs := prog.fsPathStream(t.Context(), "/somefile", false, nil)

	for range paths {
		t.Fatal("should not emit paths")
	}

	select {
	case err := <-errs:
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to stream from fs")
	default:
		t.Fatal("expected error from fsPathStream")
	}
}

// Expectation: Should cancel walking when context is canceled.
func Test_Program_fsPathStream_CtxCancel_Error(t *testing.T) {
	fs := afero.NewMemMapFs()

	require.NoError(t, fs.MkdirAll("/cancel", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/cancel/a.txt", []byte("a"), 0o644))

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	prog := NewProgram(fs, io.Discard, io.Discard, nil, nil)
	paths, errs := prog.fsPathStream(ctx, "/cancel", false, nil)

	for range paths {
		t.Fatal("should not emit paths")
	}

	for err := range errs {
		require.ErrorIs(t, err, context.Canceled)
	}
}

// Expectation: fsPathStream should return an error when given an invalid exclude pattern.
func Test_Program_fsPathStream_InvalidExcludePattern_Error(t *testing.T) {
	fs := afero.NewMemMapFs()

	require.NoError(t, fs.MkdirAll("/data", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/file.txt", []byte("x"), 0o644))

	prog := NewProgram(fs, io.Discard, io.Discard, nil, nil)
	paths, errs := prog.fsPathStream(t.Context(), "/data", false, []string{"invalid["})

	for range paths {
		t.Fatal("should not emit any paths")
	}

	select {
	case err := <-errs:
		require.Error(t, err)
		require.Contains(t, err.Error(), "exclude")
	default:
		t.Fatal("expected error from fsPathStream")
	}
}

// Expectation: Should return all entries from the exclude slice when no file is provided.
func Test_Program_mergeExcludes_SliceOnly_Success(t *testing.T) {
	fs := afero.NewMemMapFs()

	prog := NewProgram(fs, io.Discard, io.Discard, nil, nil)
	result, err := prog.mergeExcludes([]string{"foo", "bar"}, "")

	require.NoError(t, err)
	require.Equal(t, []string{"foo", "bar"}, result)
}

// Expectation: Should return entries from the file only when no slice is provided.
func Test_Program_mergeExcludes_FileOnly_Success(t *testing.T) {
	fs := afero.NewMemMapFs()

	content := "alpha\nbeta\n"
	require.NoError(t, afero.WriteFile(fs, "/excludes.txt", []byte(content), 0o644))

	prog := NewProgram(fs, io.Discard, io.Discard, nil, nil)
	result, err := prog.mergeExcludes(nil, "/excludes.txt")

	require.NoError(t, err)
	require.Equal(t, []string{"alpha", "beta"}, result)
}

// Expectation: Should return combined entries from both slice and file.
func Test_Program_mergeExcludes_FileAndSlice_Success(t *testing.T) {
	fs := afero.NewMemMapFs()

	content := "one\ntwo\n"
	require.NoError(t, afero.WriteFile(fs, "/ex.txt", []byte(content), 0o644))

	prog := NewProgram(fs, io.Discard, io.Discard, nil, nil)
	result, err := prog.mergeExcludes([]string{"three", "four"}, "/ex.txt")

	require.NoError(t, err)
	require.Equal(t, []string{"one", "two", "three", "four"}, result)
}

// Expectation: Should ignore blank lines and comment lines in the exclude file.
func Test_Program_mergeExcludes_FileWithCommentsAndBlanks_Success(t *testing.T) {
	fs := afero.NewMemMapFs()

	content := `
# this is a comment
foo

# another comment
bar
`
	require.NoError(t, afero.WriteFile(fs, "/ignore.txt", []byte(content), 0o644))

	prog := NewProgram(fs, io.Discard, io.Discard, nil, nil)
	result, err := prog.mergeExcludes(nil, "/ignore.txt")

	require.NoError(t, err)
	require.Equal(t, []string{"foo", "bar"}, result)
}

// Expectation: Should return a non-nil slice when no excludes are provided.
func Test_Program_mergeExcludes_NoExcludes_Success(t *testing.T) {
	fs := afero.NewMemMapFs()

	prog := NewProgram(fs, io.Discard, io.Discard, nil, nil)
	result, err := prog.mergeExcludes(nil, "")

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Empty(t, result)
}

// Expectation: Should return an error if the exclude file does not exist.
func Test_Program_mergeExcludes_ExcludeFileMissing_Error(t *testing.T) {
	fs := afero.NewMemMapFs()

	prog := NewProgram(fs, io.Discard, io.Discard, nil, nil)
	_, err := prog.mergeExcludes(nil, "/missing.txt")

	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to open exclude file")
}

// Expectation: The tar buffer should contain the appropriate files and folders.
func Test_writeDummyFile_Success(t *testing.T) {
	var buf bytes.Buffer

	tw := tar.NewWriter(&buf)
	require.NotNil(t, tw)

	require.NoError(t, writeDummyFile(tw, "foo.txt", false))
	require.NoError(t, writeDummyFile(tw, "bar", true))
	require.NoError(t, tw.Close())

	tr := tar.NewReader(&buf)
	require.NotNil(t, tr)

	var names []string
	for {
		hdr, err := tr.Next()

		if err == io.EOF {
			break
		}

		require.NoError(t, err)
		require.Zero(t, hdr.Size)

		if strings.HasSuffix(hdr.Name, "/") {
			require.Equal(t, tar.TypeDir, rune(hdr.Typeflag))
			require.Equal(t, baseFolderPerms, hdr.Mode)
		} else {
			require.Equal(t, tar.TypeReg, rune(hdr.Typeflag))
			require.Equal(t, baseFilePerms, hdr.Mode)
		}

		names = append(names, hdr.Name)
	}

	require.Equal(t, []string{"foo.txt", "bar/"}, names)
}

// Expectation: The function should return the correct error on header write failure.
func Test_writeDummyFile_WriteHeader_Error(t *testing.T) {
	tw := tar.NewWriter(errorWriter{})
	err := writeDummyFile(tw, "fail.txt", false)

	require.Error(t, err)
	require.Contains(t, err.Error(), "header")
}

// Expectation: The channels should contain the correct ordered paths and no errors.
func Test_extsortStrings_Success(t *testing.T) {
	in := make(chan string, 3)
	in <- "c"
	in <- "a"
	in <- "b"
	close(in)

	extErrs := make(chan error)
	close(extErrs)

	out, errs := extsortStrings(t.Context(), in, extErrs, &extSortConfigDefault)

	got := []string{}
	for p := range out {
		got = append(got, p)
	}

	for err := range errs {
		require.NoError(t, err)
	}

	require.Equal(t, []string{"a", "b", "c"}, got)
}

// Expectation: The channels should contain the correct error and no paths.
func Test_extsortStrings_ExternalChannel_Error(t *testing.T) {
	in := make(chan string)
	close(in)

	extErrs := make(chan error, 1)
	extErrs <- errors.New("simulated external error")
	close(extErrs)

	out, errs := extsortStrings(t.Context(), in, extErrs, &extSortConfigDefault)

	for range out {
		t.Fatal("should not receive any output")
	}

	select {
	case err := <-errs:
		require.Error(t, err)
		require.Contains(t, err.Error(), "simulated external error")
	default:
		t.Fatal("expected error from extsortStrings")
	}
}

// Expectation: A context cancellation should be respected and the sorting interrupted.
func Test_extsortStrings_CtxCancel_Error(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())

	in := make(chan string, 1)
	in <- "a"
	close(in)

	extErrs := make(chan error)
	close(extErrs)

	cancel()
	out, errs := extsortStrings(ctx, in, extErrs, &extSortConfigDefault)

	for range out {
		t.Fatal("should not emit output")
	}

	for err := range errs {
		require.ErrorIs(t, err, context.Canceled)
	}
}

// Expectation: The exclusions from the table should meet their respective expectations.
func Test_isExcluded_Table(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		isDir    bool
		excludes []string
		expected bool
	}{
		// === Exact literal matches ===
		{"Exact file match", "foo.txt", false, []string{"foo.txt"}, true},
		{"Exact path match", "src/main.go", false, []string{"src/main.go"}, true},
		{"Exact dir match", "build", true, []string{"build"}, true},
		{"Exact dir only match", "build", true, []string{"build/"}, true},
		{"Exact dir only match", "build", false, []string{"build/"}, false},
		{"No match different name", "foo.txt", false, []string{"bar.txt"}, false},
		{"No match different path", "lib/main.go", false, []string{"src/main.go"}, false},

		// === Single star (*) - matches within single path component ===
		{"Star matches single component", "test.go", false, []string{"*.go"}, true},
		{"Star in middle", "test_file.go", false, []string{"test_*.go"}, true},
		{"Star at start", "main.go", false, []string{"*.go"}, true},
		{"Star matches empty", "test.go", false, []string{"test*.go"}, true},
		{"Multiple stars in pattern", "test_main_file.go", false, []string{"*_*_*.go"}, true},
		{"Star does NOT cross slash", "src/main.go", false, []string{"*.go"}, false},
		{"Star in path component", "src/test.go", false, []string{"src/*.go"}, true},
		{"Star no match across dirs", "src/lib/main.go", false, []string{"src/*.go"}, false},

		// === Double star (**) - recursive directory matching ===
		{"Doublestar matches anything", "a/b/c/file.go", false, []string{"**"}, true},
		{"Doublestar with extension", "deep/nested/file.go", false, []string{"**/*.go"}, true},
		{"Doublestar at start", "any/path/main.go", false, []string{"**/*.go"}, true},
		{"Doublestar in middle", "src/any/deep/main.go", false, []string{"src/**/main.go"}, true},
		{"Doublestar matches zero dirs", "src/main.go", false, []string{"src/**/main.go"}, true},
		{"Doublestar matches multiple levels", "a/b/c/d/e.txt", false, []string{"a/**/e.txt"}, true},
		{"Multiple doublestars", "a/b/c/d/e.txt", false, []string{"a/**/c/**/e.txt"}, true},
		{"Doublestar exact subpath", "vendor/pkg", true, []string{"vendor/**"}, true},

		// === Question mark (?) - single character ===
		{"Question mark matches one char", "a.txt", false, []string{"?.txt"}, true},
		{"Question mark no match multiple", "ab.txt", false, []string{"?.txt"}, false},
		{"Question mark no match zero", ".txt", false, []string{"?.txt"}, false},
		{"Multiple question marks", "ab.txt", false, []string{"??.txt"}, true},
		{"Question mark in path", "src/a.go", false, []string{"src/?.go"}, true},
		{"Question mark does not match slash", "a/b", true, []string{"a?b"}, false},

		// === Character classes [...] ===
		{"Char class range", "a.txt", false, []string{"[a-z].txt"}, true},
		{"Char class range no match", "A.txt", false, []string{"[a-z].txt"}, false},
		{"Char class explicit", "a.txt", false, []string{"[abc].txt"}, true},
		{"Char class explicit no match", "d.txt", false, []string{"[abc].txt"}, false},
		{"Negated char class", "d.txt", false, []string{"[!abc].txt"}, true},
		{"Negated char class no match", "a.txt", false, []string{"[!abc].txt"}, false},
		{"Char class with numbers", "1.txt", false, []string{"[0-9].txt"}, true},
		{"Char class mixed", "a.txt", false, []string{"[a-z0-9].txt"}, true},

		// === Brace expansion {a,b,c} ===
		{"Brace expansion first option", "file.go", false, []string{"file.{go,py,js}"}, true},
		{"Brace expansion second option", "file.py", false, []string{"file.{go,py,js}"}, true},
		{"Brace expansion third option", "file.js", false, []string{"file.{go,py,js}"}, true},
		{"Brace expansion no match", "file.txt", false, []string{"file.{go,py,js}"}, false},
		{"Brace in path", "src/main.go", false, []string{"{src,lib}/main.go"}, true},
		{"Nested braces", "a1.txt", false, []string{"{a,b}{1,2}.txt"}, true},
		{"Empty brace option", "file.", false, []string{"file.{go,}"}, true},

		// === Dotfiles and hidden files ===
		{"Hidden file match", ".gitignore", false, []string{".gitignore"}, true},
		{"Hidden dir match", ".git", true, []string{".git"}, true},
		{"Hidden file with star", ".bashrc", false, []string{".*"}, true},
		{"Nested hidden file", "project/.git/config", false, []string{"**/.git/config"}, true},
		{"Hidden dir recursive", "a/.hidden/b/file", false, []string{"**/.hidden/**"}, true},
		{"Star matches dot prefix", ".hidden", false, []string{"*hidden"}, true},
		{"Doublestar matches dot prefix", "a/.hidden/file", false, []string{"a/**"}, true},

		// === Edge cases with slashes ===
		{"Pattern with internal double slash", "a/b/c", false, []string{"a//b/c"}, false},
		{"Empty path component in pattern", "a/c", false, []string{"a//c"}, false},
		{"Sub-path", "sub/foo/bar", true, []string{"foo/bar"}, false},
		{"Non-sub-path dir", "foo/bar", true, []string{"foo/bar"}, true},
		{"Non-sub-path file", "foo/bar", false, []string{"foo/bar"}, true},
		{"Leading slash stripped dir", "sub/foo/bar", true, []string{"/foo/bar"}, false},
		{"Leading slash stripped file", "sub/foo/bar", false, []string{"/foo/bar"}, false},
		{"Leading slash stripped", "vendor/lib.go", false, []string{"/vendor/**"}, true},

		// === Complex real-world patterns ===
		{"node_modules anywhere", "project/node_modules/pkg/index.js", false, []string{"**/node_modules/**"}, true},
		{"Specific file in any node_modules", "a/node_modules/pkg/package.json", false, []string{"**/node_modules/**/package.json"}, true},
		{"Build artifacts", "target/release/binary", false, []string{"target/**"}, true},
		{"Test files", "src/utils_test.go", false, []string{"**/*_test.go"}, true},
		{"Backup files", "config.bak", false, []string{"*.bak"}, true},
		{"Temp files", "file.tmp", false, []string{"*.tmp"}, true},
		{"Log files anywhere", "app/logs/app.log", false, []string{"**/*.log"}, true},
		{"OS specific", ".DS_Store", false, []string{".DS_Store"}, true},
		{"Editor files", "main.go.swp", false, []string{"*.swp"}, true},

		// === Multiple patterns (OR logic) ===
		{"First pattern matches", "test.go", false, []string{"*.go", "*.py"}, true},
		{"Second pattern matches", "test.py", false, []string{"*.go", "*.py"}, true},
		{"Neither pattern matches", "test.txt", false, []string{"*.go", "*.py"}, false},
		{"Complex multiple patterns", "src/test.go", false, []string{"docs/**", "src/**/*.go", "*.tmp"}, true},

		// === Unicode and special characters ===
		{"Unicode filename", "файл.txt", false, []string{"файл.txt"}, true},
		{"Unicode in pattern", "test/файл.go", false, []string{"test/*.go"}, true},
		{"Spaces in filename", "my file.txt", false, []string{"my file.txt"}, true},
		{"Spaces with wildcard", "my test.txt", false, []string{"my *.txt"}, true},

		// === Escaping and special characters ===
		{"Brackets as char class", "test1.txt", false, []string{"test[1].txt"}, true},
		{"Literal star in filename", "test*.txt", false, []string{"test\\*.txt"}, true},
		{"Literal question in filename", "what?.txt", false, []string{"what\\?.txt"}, true},
		{"Literal star in filename", "test*a.txt", false, []string{"test\\*.txt"}, false},
		{"Literal question in filename", "what?a.txt", false, []string{"what\\?.txt"}, false},

		// === Performance and edge cases ===
		{"Empty pattern", "anything", false, []string{""}, false},
		{"Only wildcards", "anything", false, []string{"**"}, true},
		{"Deep nesting", "a/b/c/d/e/f/g/h.txt", false, []string{"a/**/h.txt"}, true},
		{"Many alternatives", "test.go", false, []string{"*.{go,py,js,cpp,c,h,hpp,java,kt,scala,clj}"}, true},

		// === Negative cases - common mistakes ===
		{"Star doesn't match path separator", "a/b", false, []string{"a*b"}, false},
		{"Question doesn't match path separator", "a/b", false, []string{"a?b"}, false},
		{"Single star is not recursive", "a/b/c.txt", false, []string{"*.txt"}, false},
		{"Doublestar in middle of word", "abc/def.txt", false, []string{"a**/def.txt"}, true},
		{"Char class doesn't match multiple", "ab.txt", false, []string{"[ab].txt"}, false},

		// === Case sensitivity (Unix is case-sensitive) ===
		{"Case sensitive match", "File.TXT", false, []string{"File.TXT"}, true},
		{"Case sensitive no match", "file.txt", false, []string{"File.TXT"}, false},
		{"Case sensitive wildcards", "FILE.txt", false, []string{"*.txt"}, true},
		{"Case sensitive char class", "A.txt", false, []string{"[A-Z].txt"}, true},
		{"Case sensitive char class no match", "a.txt", false, []string{"[A-Z].txt"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := isExcluded(tt.path, tt.isDir, tt.excludes)
			require.NoError(t, err)
			require.Equalf(t, tt.expected, got,
				"path=%q, isDir=%v, patterns=%v",
				tt.path, tt.isDir, tt.excludes)
		})
	}
}

// Expectation: The function should reject an invalid pattern and return an error.
func Test_isExcluded_InvalidExcludePattern_Error(t *testing.T) {
	result, err := isExcluded("/a/b/c", true, []string{"a["})

	require.Error(t, err)
	require.ErrorContains(t, err, "pattern")
	require.False(t, result)
}
