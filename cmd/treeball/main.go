/*
treeball creates, diffs, and lists directory trees as archives.

It preserves directory trees as compressed archives, replacing all files with zero-byte
placeholder files. This creates lightweight tarballs that are portable, navigable, and
diffable. Think of browsable inventory-type backups of e.g. media libraries, but without
the overhead of preserving file contents.

The program works efficiently even with millions of files, intelligently off-loading data to
disk when system resources would otherwise become too constrained. It supports these commands:

	create - build a tarball from a given directory tree
	diff   - generate a diff tarball containing only the changes between two sources
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
	"compress/gzip"
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
	baseFilePerms   int64 = 0o666
	baseFolderPerms int64 = 0o777

	tarStreamBuffer  = 1000
	fsStreamBuffer   = 1000
	stackTraceBuffer = 1 << 24

	exitTimeout        = 10 * time.Second
	exitCodeSuccess    = 0
	exitCodeDiffsFound = 1
	exitCodeFailure    = 2
)

var (
	// Version is automatically populated by the build process (Makefile).
	Version string

	//nolint:mnd
	gzipConfigDefault = GzipConfig{
		BlockSize:        1 << 20,               // Approximate size of blocks (pgzip operations)
		BlockCount:       runtime.GOMAXPROCS(0), // Amount of blocks processing in parallel (pgzip operations)
		CompressionLevel: gzip.BestCompression,  // Target level for compression (0: none to 9: highest)
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

	gzipConfig    *GzipConfig
	extSortConfig *extsort.Config
}

// NewProgram returns a pointer to a new [Program].
func NewProgram(fs afero.Fs, stdout io.Writer, stderr io.Writer, gzipConfig *GzipConfig, extsortConfig *extsort.Config) *Program {
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

	if gzipConfig == nil {
		cfg := gzipConfigDefault
		gzipConfig = &cfg
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
		gzipConfig:    gzipConfig,
		extSortConfig: extsortConfig,
	}
}

func newRootCmd(ctx context.Context, fs afero.Fs, stdout io.Writer, stderr io.Writer) *cobra.Command {
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

	createCmd := newCreateCmd(ctx, fs, stdout, stderr)
	diffCmd := newDiffCmd(ctx, fs, stdout, stderr)
	listCmd := newListCmd(ctx, fs, stdout, stderr)

	rootCmd.AddCommand(createCmd, diffCmd, listCmd)

	return rootCmd
}

func newCreateCmd(ctx context.Context, fs afero.Fs, stdout io.Writer, stderr io.Writer) *cobra.Command {
	var excludes []string
	var excludesFile string

	compressorConfig := gzipConfigDefault

	createCmd := &cobra.Command{
		Use:     "create <root-folder> <output.tar.gz>",
		Short:   createHelpShort,
		Long:    createHelpLong,
		Example: createExample,
		Args:    cobra.ExactArgs(2), //nolint:mnd
		RunE: func(_ *cobra.Command, args []string) error {
			prog := NewProgram(fs, stdout, stderr, &compressorConfig, nil)

			excl, err := prog.mergeExcludes(excludes, excludesFile)
			if err != nil {
				return fmt.Errorf("failed to evaluate exclude arguments: %w", err)
			}

			return prog.Create(ctx, args[0], args[1], excl)
		},
	}

	createCmd.Flags().StringArrayVar(&excludes, "exclude", nil, "pattern to exclude; can be repeated multiple times")
	createCmd.Flags().StringVar(&excludesFile, "excludes-from", "", "path to a file containing exclude patterns")
	createCmd.Flags().IntVar(&compressorConfig.CompressionLevel, "compression", gzipConfigDefault.CompressionLevel, "level of compression (0: none - 9: highest)")
	createCmd.Flags().IntVar(&compressorConfig.BlockSize, "blocksize", gzipConfigDefault.BlockSize, "block size for compressing")
	createCmd.Flags().IntVar(&compressorConfig.BlockCount, "blockcount", gzipConfigDefault.BlockCount, "blocks to compress in parallel")

	return createCmd
}

func newDiffCmd(ctx context.Context, fs afero.Fs, stdout io.Writer, stderr io.Writer) *cobra.Command {
	var excludes []string
	var excludesFile string

	sorterConfig := extSortConfigDefault
	compressorConfig := gzipConfigDefault

	diffCmd := &cobra.Command{
		Use:     "diff <old> <new> <diff.tar.gz>",
		Short:   diffHelpShort,
		Long:    diffHelpLong,
		Example: diffExample,
		Args:    cobra.ExactArgs(3), //nolint:mnd
		RunE: func(_ *cobra.Command, args []string) error {
			prog := NewProgram(fs, stdout, stderr, &compressorConfig, &sorterConfig)

			excl, err := prog.mergeExcludes(excludes, excludesFile)
			if err != nil {
				return fmt.Errorf("failed to evaluate exclude arguments: %w", err)
			}

			_, err = prog.Diff(ctx, args[0], args[1], args[2], excl)

			return err
		},
	}

	diffCmd.Flags().StringArrayVar(&excludes, "exclude", nil, "pattern to exclude; can be repeated multiple times")
	diffCmd.Flags().StringVar(&excludesFile, "excludes-from", "", "path to a file containing exclude patterns")
	diffCmd.Flags().StringVar(&sorterConfig.TempFilesDir, "tmpdir", extSortConfigDefault.TempFilesDir, "on-disk location for intermediate files")
	diffCmd.Flags().IntVar(&compressorConfig.CompressionLevel, "compression", gzipConfigDefault.CompressionLevel, "level of compression (0: none - 9: highest)")
	diffCmd.Flags().IntVar(&sorterConfig.NumWorkers, "workers", extSortConfigDefault.NumWorkers, "workers for concurrent operations")
	diffCmd.Flags().IntVar(&sorterConfig.ChunkSize, "chunksize", extSortConfigDefault.ChunkSize, "max records per worker before spilling to disk")

	return diffCmd
}

func newListCmd(ctx context.Context, fs afero.Fs, stdout io.Writer, stderr io.Writer) *cobra.Command {
	sort := true
	sorterConfig := extSortConfigDefault

	listCmd := &cobra.Command{
		Use:     "list <input.tar.gz>",
		Short:   listHelpShort,
		Long:    listHelpLong,
		Example: listExample,
		Args:    cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			prog := NewProgram(fs, stdout, stderr, nil, &sorterConfig)

			return prog.List(ctx, args[0], sort)
		},
	}

	listCmd.Flags().BoolVar(&sort, "sort", true, "sort the output list; for better comparability")
	listCmd.Flags().StringVar(&sorterConfig.TempFilesDir, "tmpdir", extSortConfigDefault.TempFilesDir, "on-disk location for intermediate files")
	listCmd.Flags().IntVar(&sorterConfig.NumWorkers, "workers", extSortConfigDefault.NumWorkers, "workers for concurrent operations")
	listCmd.Flags().IntVar(&sorterConfig.ChunkSize, "chunksize", extSortConfigDefault.ChunkSize, "max records per worker before spilling to disk")

	return listCmd
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

	sigChan2 := make(chan os.Signal, 1)
	signal.Notify(sigChan2, syscall.SIGUSR1)

	go func() {
		for range sigChan2 {
			buf := make([]byte, stackTraceBuffer)
			stacklen := runtime.Stack(buf, true)
			os.Stderr.Write(buf[:stacklen])
		}
	}()

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
