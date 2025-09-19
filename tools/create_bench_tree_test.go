package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

// A helper filesystem for tests to simulate filesystem errors.
type failingFs struct {
	afero.Fs
	failMkdirAll bool
	failCreate   bool
}

// A helper function for tests to simulate folder creation failure.
func (f *failingFs) MkdirAll(path string, perm os.FileMode) error {
	if f.failMkdirAll {
		return errors.New("simulated mkdirall error")
	}

	return f.Fs.MkdirAll(path, perm) //nolint:wrapcheck
}

// A helper function for tests to simulate file creation failure.
func (f *failingFs) Create(name string) (afero.File, error) {
	if f.failCreate {
		return nil, errors.New("simulated create error")
	}

	return f.Fs.Create(name) //nolint:wrapcheck
}

// Expectation: The requested tree should be produced without errors.
func Test_Tool_createDummyTree_Success(t *testing.T) {
	fs := afero.NewMemMapFs()

	base := "/testroot"
	totalFiles := 250
	expectedDepth := 5 // dept/proj/batch/group/file

	err := createDummyTree(fs, base, totalFiles)
	require.NoError(t, err)

	var fileCount int
	err = afero.Walk(fs, base, func(path string, info os.FileInfo, err error) error {
		require.NoError(t, err)

		if info.Mode().IsRegular() {
			fileCount++

			require.True(t, strings.HasPrefix(info.Name(), "data_") && strings.HasSuffix(info.Name(), ".txt"))

			relPath, relErr := filepath.Rel(base, path)
			require.NoError(t, relErr)

			depth := len(strings.Split(relPath, string(filepath.Separator)))
			require.Equal(t, expectedDepth, depth)
		}

		return nil
	})
	require.NoError(t, err)

	require.Equal(t, totalFiles, fileCount)
}

// Expectation: The requested tree creation should fail with the correct error.
func Test_Tool_createDummyTree_MkDirAll_Error(t *testing.T) {
	fs := &failingFs{
		Fs:           afero.NewMemMapFs(),
		failMkdirAll: true,
	}

	err := createDummyTree(fs, "/fail", 100000)
	require.Error(t, err)
	require.Contains(t, err.Error(), "mkdirall")
}

// Expectation: The requested tree creation should fail with the correct error.
func Test_Tool_createDummyTree_CreateFile_Error(t *testing.T) {
	fs := &failingFs{
		Fs:         afero.NewMemMapFs(),
		failCreate: true,
	}

	err := createDummyTree(fs, "/fail", 100000)
	require.Error(t, err)
	require.Contains(t, err.Error(), "creating file")
}
