package main

import (
	"context"
	"fmt"
)

// List to standard output (stdout) either a sorted or unsorted list of another tarball.
func (prog *Program) List(ctx context.Context, input string, sort bool) error {
	paths, errs := prog.tarPathStream(ctx, input, sort)

	for path := range paths {
		fmt.Fprintln(prog.stdout, path)
	}

	for err := range errs {
		if err != nil {
			return fmt.Errorf("failure during listing: %w", err)
		}
	}

	return nil
}
