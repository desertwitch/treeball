package main

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

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

// Expectation: The 'create' subcommand should not error with valid arguments and existing input.
func Test_NewRootCmd_CreateCommand_Success(t *testing.T) {
	fs := afero.NewMemMapFs()

	_ = fs.MkdirAll("/some/input", 0o755)
	_ = afero.WriteFile(fs, "/some/input/file.txt", []byte("test"), 0o644)

	cmd := newRootCmd(t.Context(), fs, nil, nil)
	cmd.SetArgs([]string{"create", "/some/input", "/some/output.tar.gz"})

	require.NoError(t, cmd.Execute())
}

// Expectation: The 'create' subcommand should error when missing arguments.
func Test_NewRootCmd_CreateCommand_ArgCount_Error(t *testing.T) {
	fs := afero.NewMemMapFs()

	cmd := newRootCmd(t.Context(), fs, nil, nil)
	cmd.SetArgs([]string{"create", "/only-one"})

	require.Error(t, cmd.Execute())
}

// Expectation: The 'create' subcommand should error when the exclude file does not exist.
func Test_NewRootCmd_CreateCommand_ExcludeFileMissing_Error(t *testing.T) {
	fs := afero.NewMemMapFs()

	cmd := newRootCmd(t.Context(), fs, nil, nil)
	cmd.SetArgs([]string{"create", "/one", "/two", "--excludes-from=/a.txt"})
	err := cmd.Execute()

	require.Error(t, err)
	require.ErrorContains(t, err, "exclude")
}

// Expectation: The 'diff' subcommand should produce the correct error when differences are found.
func Test_NewRootCmd_DiffCommand_DiffsFound_Success(t *testing.T) {
	fs := afero.NewMemMapFs()

	_ = afero.WriteFile(fs, "/old.tar.gz", createTar([]string{"a.txt"}), 0o644)
	_ = afero.WriteFile(fs, "/new.tar.gz", createTar([]string{"a.txt", "b.txt"}), 0o644)

	cmd := newRootCmd(t.Context(), fs, nil, nil)
	cmd.SetArgs([]string{"diff", "/old.tar.gz", "/new.tar.gz", "/diff.tar.gz"})

	require.ErrorIs(t, cmd.Execute(), ErrDiffsFound)
}

// Expectation: The 'diff' subcommand should not produce an error when no differences are found.
func Test_NewRootCmd_DiffCommand_NoDiffsFound_Success(t *testing.T) {
	fs := afero.NewMemMapFs()

	_ = afero.WriteFile(fs, "/old.tar.gz", createTar([]string{"a.txt"}), 0o644)
	_ = afero.WriteFile(fs, "/new.tar.gz", createTar([]string{"a.txt"}), 0o644)

	cmd := newRootCmd(t.Context(), fs, nil, nil)
	cmd.SetArgs([]string{"diff", "/old.tar.gz", "/new.tar.gz", "/diff.tar.gz"})

	require.NoError(t, cmd.Execute())
}

// Expectation: The 'diff' subcommand should error when missing arguments.
func Test_NewRootCmd_DiffCommand_ArgCount_Error(t *testing.T) {
	fs := afero.NewMemMapFs()

	cmd := newRootCmd(t.Context(), fs, nil, nil)
	cmd.SetArgs([]string{"diff", "/one", "/two"})

	require.Error(t, cmd.Execute())
}

// Expectation: The 'diff' subcommand should error when the exclude file does not exist.
func Test_NewRootCmd_DiffCommand_ExcludeFileMissing_Error(t *testing.T) {
	fs := afero.NewMemMapFs()

	cmd := newRootCmd(t.Context(), fs, nil, nil)
	cmd.SetArgs([]string{"diff", "/one", "/two", "/three", "--excludes-from=/a.txt"})
	err := cmd.Execute()

	require.Error(t, err)
	require.ErrorContains(t, err, "exclude")
}

// Expectation: The 'list' subcommand should not error when invoked with a valid tarball.
func Test_NewRootCmd_ListCommand_Success(t *testing.T) {
	fs := afero.NewMemMapFs()

	_ = afero.WriteFile(fs, "/input.tar.gz", createTar([]string{"a.txt", "b.txt"}), 0o644)

	cmd := newRootCmd(t.Context(), fs, nil, nil)
	cmd.SetArgs([]string{"list", "/input.tar.gz"})

	require.NoError(t, cmd.Execute())
}

// Expectation: The 'list' subcommand should error when missing arguments.
func Test_NewRootCmd_ListCommand_ArgCount_Error(t *testing.T) {
	fs := afero.NewMemMapFs()

	cmd := newRootCmd(t.Context(), fs, nil, nil)
	cmd.SetArgs([]string{"list"})

	require.Error(t, cmd.Execute())
}

// Expectation: The 'list' subcommand should error when the exclude file does not exist.
func Test_NewRootCmd_ListCommand_ExcludeFileMissing_Error(t *testing.T) {
	fs := afero.NewMemMapFs()

	_ = afero.WriteFile(fs, "/input.tar.gz", createTar([]string{"a.txt", "b.txt"}), 0o644)

	cmd := newRootCmd(t.Context(), fs, nil, nil)
	cmd.SetArgs([]string{"list", "/input.tar.gz", "--excludes-from=/e.txt"})
	err := cmd.Execute()

	require.Error(t, err)
	require.ErrorContains(t, err, "exclude")
}

// Expectation: The root command should error when given an unknown subcommand.
func Test_NewRootCmd_UnknownCommand_Error(t *testing.T) {
	fs := afero.NewMemMapFs()

	cmd := newRootCmd(t.Context(), fs, nil, nil)
	cmd.SetArgs([]string{"unknown-subcommand"})

	require.Error(t, cmd.Execute())
}

// Expectation: The process should return the documented exit codes (0 success, 1 differences, 2 failure).
func Test_main_ExitCodes_Table(t *testing.T) {
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
		cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^Test_main_ExitCodes_Table$") //nolint:gosec
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
