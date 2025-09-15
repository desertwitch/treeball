package main

import (
	"archive/tar"
	"bytes"
	"io"
	"testing"

	"github.com/stretchr/testify/require"
)

// Expectation: The tar buffer should contain the appropriate files and folders.
func Test_WriteDummyFile_Success(t *testing.T) {
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

	require.ElementsMatch(t, []string{"foo.txt", "bar/"}, names)
}

// Expectation: The function should handle the exclusions according to the table's expectations.
func Test_IsExcluded_Table(t *testing.T) {
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
