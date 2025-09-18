package main

import (
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

// Expectation: The 'create' subcommand should not error with valid arguments and existing input.
func Test_CLI_CreateCommand_Success(t *testing.T) {
	fs := afero.NewMemMapFs()

	_ = fs.MkdirAll("/some/input", 0o755)
	_ = afero.WriteFile(fs, "/some/input/file.txt", []byte("test"), 0o644)

	cmd := newRootCmd(t.Context(), fs, nil, nil)
	cmd.SetArgs([]string{"create", "/some/input", "/some/output.tar.gz"})

	require.NoError(t, cmd.Execute())
}

// Expectation: The 'diff' subcommand should produce the correct error when differences are found.
func Test_CLI_DiffCommand_DiffsFound_Success(t *testing.T) {
	fs := afero.NewMemMapFs()

	_ = afero.WriteFile(fs, "/old.tar.gz", createTar([]string{"a.txt"}), 0o644)
	_ = afero.WriteFile(fs, "/new.tar.gz", createTar([]string{"a.txt", "b.txt"}), 0o644)

	cmd := newRootCmd(t.Context(), fs, nil, nil)
	cmd.SetArgs([]string{"diff", "/old.tar.gz", "/new.tar.gz", "/diff.tar.gz"})

	require.ErrorIs(t, cmd.Execute(), ErrDiffsFound)
}

// Expectation: The 'diff' subcommand should not produce an error when no differences are found.
func Test_CLI_DiffCommand_NoDiffsFound_Success(t *testing.T) {
	fs := afero.NewMemMapFs()

	_ = afero.WriteFile(fs, "/old.tar.gz", createTar([]string{"a.txt"}), 0o644)
	_ = afero.WriteFile(fs, "/new.tar.gz", createTar([]string{"a.txt"}), 0o644)

	cmd := newRootCmd(t.Context(), fs, nil, nil)
	cmd.SetArgs([]string{"diff", "/old.tar.gz", "/new.tar.gz", "/diff.tar.gz"})

	require.NoError(t, cmd.Execute())
}

// Expectation: The 'list' subcommand should not error when invoked with a valid tarball.
func Test_CLI_ListCommand_Success(t *testing.T) {
	fs := afero.NewMemMapFs()

	_ = afero.WriteFile(fs, "/input.tar.gz", createTar([]string{"a.txt", "b.txt"}), 0o644)

	cmd := newRootCmd(t.Context(), fs, nil, nil)
	cmd.SetArgs([]string{"list", "/input.tar.gz"})

	require.NoError(t, cmd.Execute())
}

// Expectation: The root command should error when given an unknown subcommand.
func Test_CLI_UnknownCommand_Error(t *testing.T) {
	fs := afero.NewMemMapFs()

	cmd := newRootCmd(t.Context(), fs, nil, nil)
	cmd.SetArgs([]string{"unknown-subcommand"})

	require.Error(t, cmd.Execute())
}

// Expectation: The 'create' subcommand should error when missing arguments.
func Test_CLI_CreateCommand_ArgCount_Error(t *testing.T) {
	fs := afero.NewMemMapFs()

	cmd := newRootCmd(t.Context(), fs, nil, nil)
	cmd.SetArgs([]string{"create", "/only-one"})

	require.Error(t, cmd.Execute())
}

// Expectation: The 'create' subcommand should error when the exclude file does not exist.
func Test_CLI_CreateCommand_ExcludeFile_Error(t *testing.T) {
	fs := afero.NewMemMapFs()

	cmd := newRootCmd(t.Context(), fs, nil, nil)
	cmd.SetArgs([]string{"create", "/one", "/two", "--excludes-from=/a.txt"})
	err := cmd.Execute()

	require.Error(t, err)
	require.ErrorContains(t, err, "exclude")
}

// Expectation: The 'diff' subcommand should error when missing arguments.
func Test_CLI_DiffCommand_ArgCount_Error(t *testing.T) {
	fs := afero.NewMemMapFs()

	cmd := newRootCmd(t.Context(), fs, nil, nil)
	cmd.SetArgs([]string{"diff", "/one", "/two"})

	require.Error(t, cmd.Execute())
}

// Expectation: The 'diff' subcommand should error when the exclude file does not exist.
func Test_CLI_DiffCommand_ExcludeFile_Error(t *testing.T) {
	fs := afero.NewMemMapFs()

	cmd := newRootCmd(t.Context(), fs, nil, nil)
	cmd.SetArgs([]string{"diff", "/one", "/two", "/three", "--excludes-from=/a.txt"})
	err := cmd.Execute()

	require.Error(t, err)
	require.ErrorContains(t, err, "exclude")
}

// Expectation: The 'list' subcommand should error when missing arguments.
func Test_CLI_ListCommand_ArgCount_Error(t *testing.T) {
	fs := afero.NewMemMapFs()

	cmd := newRootCmd(t.Context(), fs, nil, nil)
	cmd.SetArgs([]string{"list"})

	require.Error(t, cmd.Execute())
}
