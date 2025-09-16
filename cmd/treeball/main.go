/*
treeball creates, diffs, and lists directory trees as archives.

It treats directory trees as artifacts that can be archived, compared, and extracted.
Entire filesystem structures are replicated into tarballs, with the actual files being
replaced with zero byte dummy placeholders, but preserving their exact paths. This allows
for replacing long textual 'tree'-style lists with single small, browseable .tar.gz files.

The program works efficiently even with millions of files, intelligently off-loading data to
disk when system resources would otherwise become too constrained. It supports these commands:

	create - build a tarball from a given directory tree
	diff   - generate a diff tarball containing only the changes between two tarballs
	list   - produce a sorted or unsorted listing of all the contents of a given tarball

All commands print their primary results (such as file paths or differences) to standard output
(stdout). Any encountered errors and operational messages are printed to standard error (stderr).

Exit Codes:

	0 - Success
	1 - Differences found (only for 'diff')
	2 - General failure (invalid input, I/O errors, etc.)
*/
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/lanrat/extsort"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

const (
	baseFilePerms   = 0o666
	baseFolderPerms = 0o777

	tarStreamBuffer = 1000

	exitTimeout        = 10 * time.Second
	exitCodeSuccess    = 0
	exitCodeDiffsFound = 1
	exitCodeFailure    = 2
)

var (
	// Version is automatically populated by the build process (Makefile).
	Version string

	//nolint:mnd
	pgzipConfigDefault = PgzipConfig{
		BlockSize:  1 << 20,               // Approximate size of blocks
		BlockCount: runtime.GOMAXPROCS(0), // Amount of blocks processing in parallel
	}

	//nolint:mnd
	extSortConfigDefault = extsort.Config{
		ChunkSize:          100_000,                       // Records per chunk (default: 1M)
		NumWorkers:         min(4, runtime.GOMAXPROCS(0)), // Parallel sorting/merging workers (default: 2)
		ChanBuffSize:       1,                             // Channel buffer size (default: 1)
		SortedChanBuffSize: 1000,                          // Output channel buffer (default: 1000)
		TempFilesDir:       "",                            // Temporary files directory (default: intelligent selection)
	}

	// ErrDiffsFound is an exit-code relevant sentinel error.
	ErrDiffsFound = errors.New("differences were found")
)

// Program is the primary structure of the application.
type Program struct {
	fs       afero.Fs
	fsWalker Walker

	stdout io.Writer
	stderr io.Writer

	pgzipConfig   *PgzipConfig
	extSortConfig *extsort.Config
}

// NewProgram returns a pointer to a new [Program].
func NewProgram(fs afero.Fs, stdout io.Writer, stderr io.Writer, pgzipConfig *PgzipConfig, extsortConfig *extsort.Config) *Program {
	var walker Walker

	if fs == nil {
		fs = afero.NewOsFs()
	}

	if stdout == nil {
		stdout = os.Stdout
	}

	if stderr == nil {
		stderr = os.Stderr
	}

	if pgzipConfig == nil {
		cfg := pgzipConfigDefault
		pgzipConfig = &cfg
	}

	if extsortConfig == nil {
		cfg := extSortConfigDefault
		extsortConfig = &cfg
	}

	if _, ok := fs.(*afero.OsFs); ok {
		walker = OSWalker{}
	} else {
		walker = AferoWalker{FS: fs}
	}

	return &Program{
		fs:            fs,
		fsWalker:      walker,
		stdout:        stdout,
		stderr:        stderr,
		pgzipConfig:   pgzipConfig,
		extSortConfig: extsortConfig,
	}
}

func newRootCmd(ctx context.Context, fs afero.Fs, stdout io.Writer, stderr io.Writer) *cobra.Command {
	var createExcludes []string

	rootCmd := &cobra.Command{
		Use:               "treeball",
		Short:             rootHelpShort,
		Long:              rootHelpLong,
		Version:           Version,
		SilenceErrors:     true,
		SilenceUsage:      true,
		CompletionOptions: cobra.CompletionOptions{DisableDefaultCmd: true},
	}
	rootCmd.SetOut(stdout)
	rootCmd.SetErr(stderr)

	createCompressorConfig := pgzipConfigDefault
	createCmd := &cobra.Command{
		Use:     "create <root-folder> <output.tar.gz>",
		Short:   createHelpShort,
		Long:    createHelpLong,
		Example: createExample,
		Args:    cobra.ExactArgs(2), //nolint:mnd
		RunE: func(_ *cobra.Command, args []string) error {
			prog := NewProgram(fs, stdout, stderr, &createCompressorConfig, nil)

			return prog.Create(ctx, args[0], args[1], createExcludes)
		},
	}
	createCmd.Flags().StringArrayVar(&createExcludes, "exclude", nil, "path to exclude; can be repeated multiple times")
	createCmd.Flags().IntVar(&createCompressorConfig.BlockSize, "blocksize", pgzipConfigDefault.BlockSize, "block size for compressing")
	createCmd.Flags().IntVar(&createCompressorConfig.BlockCount, "blockcount", pgzipConfigDefault.BlockCount, "blocks to compress in parallel")

	diffSorterConfig := extSortConfigDefault
	diffCmd := &cobra.Command{
		Use:     "diff <old.tar.gz> <new.tar.gz> <diff.tar.gz>",
		Short:   diffHelpShort,
		Long:    diffHelpLong,
		Example: diffExample,
		Args:    cobra.ExactArgs(3), //nolint:mnd
		RunE: func(_ *cobra.Command, args []string) error {
			prog := NewProgram(fs, stdout, stderr, nil, &diffSorterConfig)
			_, err := prog.Diff(ctx, args[0], args[1], args[2])

			return err
		},
	}
	diffCmd.Flags().StringVar(&diffSorterConfig.TempFilesDir, "tmpdir", extSortConfigDefault.TempFilesDir, "on-disk location for intermediate files")
	diffCmd.Flags().IntVar(&diffSorterConfig.NumWorkers, "workers", extSortConfigDefault.NumWorkers, "workers for concurrent operations")
	diffCmd.Flags().IntVar(&diffSorterConfig.ChunkSize, "chunksize", extSortConfigDefault.ChunkSize, "max records per worker before spilling to disk")

	listSort := true
	listSorterConfig := extSortConfigDefault
	listCmd := &cobra.Command{
		Use:     "list <input.tar.gz>",
		Short:   listHelpShort,
		Long:    listHelpLong,
		Example: listExample,
		Args:    cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			prog := NewProgram(fs, stdout, stderr, nil, &listSorterConfig)

			return prog.List(ctx, args[0], listSort)
		},
	}
	listCmd.Flags().BoolVar(&listSort, "sort", true, "sort the output list; for better comparability")
	listCmd.Flags().StringVar(&listSorterConfig.TempFilesDir, "tmpdir", extSortConfigDefault.TempFilesDir, "on-disk location for intermediate files")
	listCmd.Flags().IntVar(&listSorterConfig.NumWorkers, "workers", extSortConfigDefault.NumWorkers, "workers for concurrent operations")
	listCmd.Flags().IntVar(&listSorterConfig.ChunkSize, "chunksize", extSortConfigDefault.ChunkSize, "max records per worker before spilling to disk")

	rootCmd.AddCommand(createCmd, diffCmd, listCmd)

	return rootCmd
}

func main() {
	var exitCode int

	defer func() {
		os.Exit(exitCode)
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	errChan := make(chan error, 1)
	go func() {
		rootCmd := newRootCmd(ctx, afero.NewOsFs(), os.Stdout, os.Stderr)
		errChan <- rootCmd.Execute()
	}()

	select {
	case err := <-errChan:
		if err != nil {
			if errors.Is(err, ErrDiffsFound) {
				exitCode = exitCodeDiffsFound
			} else {
				exitCode = exitCodeFailure
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
			}
		} else {
			exitCode = exitCodeSuccess
		}

	case <-sigChan:
		fmt.Fprintln(os.Stderr, "interrupting...")
		cancel()

		select {
		case <-errChan:
			exitCode = exitCodeFailure
			fmt.Fprintln(os.Stderr, "interrupted (exited)")
		case <-time.After(exitTimeout):
			exitCode = exitCodeFailure
			fmt.Fprintln(os.Stderr, "interrupted (killed)")
		}
	}
}
