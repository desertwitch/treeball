package main

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// Expectation: Only targets inside the root should resolve, not look-alike siblings or the root itself.
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

// Expectation: Relative and absolute spellings of the same location should be recognized as such.
func Test_relToRoot_RelativePaths_Success(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	require.Equal(t, "out.tar.gz", relToRoot("src", filepath.Join(dir, "src", "out.tar.gz")))
	require.Equal(t, "out.tar.gz", relToRoot(filepath.Join(dir, "src"), "./src/out.tar.gz"))
	require.Empty(t, relToRoot("src", "out.tar.gz"))
}
