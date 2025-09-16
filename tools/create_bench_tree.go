// create_bench_tree is a benchmark helper tool for file tree creation.
//
//nolint:mnd
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"

	"github.com/spf13/afero"
)

const filesPerDir = 100

var workers = runtime.NumCPU() * 2

func buildPath(base string, d int) string {
	level1 := fmt.Sprintf("dept_%02d", d/1000)
	level2 := fmt.Sprintf("proj_%03d", d/100)
	level3 := fmt.Sprintf("batch_%04d", d/10)
	level4 := fmt.Sprintf("group_%06d", d)

	return filepath.Join(base, level1, level2, level3, level4)
}

func createDirAndFiles(fs afero.Fs, base string, d int, totalFiles int) error {
	subdir := buildPath(base, d)

	if err := fs.MkdirAll(subdir, 0o755); err != nil {
		return fmt.Errorf("error creating dir: %w", err)
	}

	for f := range filesPerDir {
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

func createDummyTree(fs afero.Fs, base string, totalFiles int) error {
	var once sync.Once
	var wg sync.WaitGroup

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tasks := make(chan int, workers)
	errCh := make(chan error, 1)

	dirsNeeded := (totalFiles / filesPerDir) + 1

	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for d := range tasks {
				if err := createDirAndFiles(fs, base, d, totalFiles); err != nil {
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

	return nil
}

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintf(os.Stderr, "usage: create_bench_tree <base_dir> <file_count>")
		os.Exit(1)
	}

	baseDir := os.Args[1]

	totalFiles, err := strconv.Atoi(os.Args[2])
	if err != nil || totalFiles <= 0 {
		fmt.Fprintf(os.Stderr, "error: invalid file count: %v\n", err)
		os.Exit(1)
	}

	if err := createDummyTree(afero.NewOsFs(), baseDir, totalFiles); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
