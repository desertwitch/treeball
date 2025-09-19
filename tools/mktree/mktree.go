// mktree is a benchmark helper tool for synthetic tree creation.
//
//nolint:mnd
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
	"syscall"

	"github.com/spf13/afero"
)

const filesPerDir = 100

var workers = runtime.GOMAXPROCS(0) * 2

func buildPath(base string, d int) string {
	level1 := fmt.Sprintf("dept_%02d", d/1000)
	level2 := fmt.Sprintf("proj_%03d", d/100)
	level3 := fmt.Sprintf("batch_%04d", d/10)
	level4 := fmt.Sprintf("group_%06d", d)

	return filepath.Join(base, level1, level2, level3, level4)
}

func createDirAndFiles(ctx context.Context, fs afero.Fs, base string, d int, totalFiles int) error {
	subdir := buildPath(base, d)

	if err := fs.MkdirAll(subdir, 0o755); err != nil {
		return fmt.Errorf("error creating dir: %w", err)
	}

	for f := range filesPerDir {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("error during creation: %w", err)
		}

		index := d*filesPerDir + f
		if index >= totalFiles {
			break
		}

		fileName := fmt.Sprintf("data_%06d.txt", f)
		path := filepath.Join(subdir, fileName)

		fh, err := fs.Create(path)
		if err != nil {
			return fmt.Errorf("error creating file: %w", err)
		}
		fh.Close()
	}

	return nil
}

func createDummyTree(ctx context.Context, fs afero.Fs, base string, totalFiles int) error {
	var once sync.Once
	var wg sync.WaitGroup

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	tasks := make(chan int, workers)
	errCh := make(chan error, 1)

	dirsNeeded := (totalFiles + filesPerDir - 1) / filesPerDir

	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for d := range tasks {
				if err := createDirAndFiles(ctx, fs, base, d, totalFiles); err != nil {
					once.Do(func() {
						errCh <- err
						cancel()
					})

					return
				}
			}
		}()
	}

	go func() {
		defer close(tasks)
		for d := range dirsNeeded {
			select {
			case tasks <- d:
			case <-ctx.Done():
				return
			}
		}
	}()

	wg.Wait()
	close(errCh)

	if err, ok := <-errCh; ok && err != nil {
		return err
	}

	if err := ctx.Err(); err != nil {
		return fmt.Errorf("error during creation: %w", err)
	}

	return nil
}

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintf(os.Stderr, "usage: mktree <base_dir> <file_count>\n")
		os.Exit(1)
	}

	baseDir := os.Args[1]

	totalFiles, err := strconv.Atoi(os.Args[2])
	if err != nil || totalFiles <= 0 {
		fmt.Fprintf(os.Stderr, "error: invalid file count: %v\n", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	errChan := make(chan error, 1)
	go func() {
		defer close(errChan)
		if err := createDummyTree(ctx, afero.NewOsFs(), baseDir, totalFiles); err != nil {
			errChan <- fmt.Errorf("failed to create tree: %w", err)
		}
	}()

	for {
		select {
		case <-sigChan:
			cancel()
		case err := <-errChan:
			if err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				os.Exit(1)
			}
			os.Exit(0)
		}
	}
}
