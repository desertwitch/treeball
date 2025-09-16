package main

import (
	"archive/tar"
	"bytes"
	"errors"
	"io"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

// Expectation: The function should handle the exclusions according to the table's expectations.
func Test_isExcluded_Table(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		excludes []string
		expected bool
	}{
		{
			name:     "Exact match",
			path:     "/home/user/docs",
			excludes: []string{"/home/user/docs", "/tmp/cache"},
			expected: true,
		},
		{
			name:     "Sub-path match",
			path:     "/home/user/docs/file.txt",
			excludes: []string{"/home/user/docs"},
			expected: true,
		},
		{
			name:     "Not excluded",
			path:     "/home/user/pictures",
			excludes: []string{"/home/user/docs"},
			expected: false,
		},
		{
			name:     "Empty exclude list",
			path:     "/any/path",
			excludes: []string{},
			expected: false,
		},
		{
			name:     "Trailing slash in exclude",
			path:     "/var/log/syslog",
			excludes: []string{"/var/log/"},
			expected: true,
		},
		{
			name:     "Path with whitespace",
			path:     "/home/user/my documents/file.txt",
			excludes: []string{"/home/user/my documents"},
			expected: true,
		},
		{
			name:     "Unclean path with double slash",
			path:     "/tmp//cache/log.txt",
			excludes: []string{"/tmp/cache"},
			expected: true,
		},
		{
			name:     "Unclean path with whitespace and double slash",
			path:     " /tmp//cache/log.txt ",
			excludes: []string{"/tmp/cache"},
			expected: true,
		},
		{
			name:     "Absolute path with absolute exclude (match)",
			path:     "/src/a/file.txt",
			excludes: []string{"/src/a"},
			expected: true,
		},
		{
			name:     "Absolute path with relative exclude (no match)",
			path:     "/src/a/file.txt",
			excludes: []string{"src/a"},
			expected: false,
		},
		{
			name:     "Relative path with relative exclude (match)",
			path:     "src/a/file.txt",
			excludes: []string{"src/a"},
			expected: true,
		},
		{
			name:     "Relative path with absolute exclude (no match)",
			path:     "src/a/file.txt",
			excludes: []string{"/src/a"},
			expected: false,
		},
		{
			name:     "Different absolute root (no match)",
			path:     "/home/user/docs/file.txt",
			excludes: []string{"/other/docs"},
			expected: false,
		},
		{
			name:     "Exclude parent directory should not match sibling",
			path:     "/src/other/file.txt",
			excludes: []string{"/src/a"},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := isExcluded(tt.path, tt.excludes)
			require.Equal(t, tt.expected, result)
		})
	}
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
		names = append(names, hdr.Name)
	}

	require.Equal(t, []string{"foo.txt", "bar/"}, names)
}

// Expecation: The channels should contain the correct ordered paths and no errors.
func Test_tarPathStream_Sorted_Success(t *testing.T) {
	fs := afero.NewMemMapFs()

	tarData := createTar([]string{"z.txt", "b/", "b/c.txt"})
	require.NoError(t, afero.WriteFile(fs, "/archive.tar.gz", tarData, 0o644))

	prog := NewProgram(fs, io.Discard, io.Discard, nil, nil)
	paths, errs := prog.tarPathStream(t.Context(), "/archive.tar.gz", true)

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
func Test_tarPathStream_Unsorted_Success(t *testing.T) {
	fs := afero.NewMemMapFs()

	tarData := createTar([]string{"z.txt", "b/", "b/c.txt"})
	require.NoError(t, afero.WriteFile(fs, "/archive.tar.gz", tarData, 0o644))

	prog := NewProgram(fs, io.Discard, io.Discard, nil, nil)
	paths, errs := prog.tarPathStream(t.Context(), "/archive.tar.gz", false)

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
func Test_tarPathStream_Open_Error(t *testing.T) {
	baseFs := afero.NewMemMapFs()

	require.NoError(t, afero.WriteFile(baseFs, "/archive.tar.gz", []byte("test"), 0o644))

	fs := errorFs{Fs: baseFs}

	prog := NewProgram(fs, io.Discard, io.Discard, nil, nil)
	paths, errs := prog.tarPathStream(t.Context(), "/archive.tar.gz", false)

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
func Test_extsortStrings_extErrs_Error(t *testing.T) {
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
