package main

import (
	"context"
	"fmt"
)

// List writes to standard output the contents of a tarball.
//
// The input parameter specifies the path to the tarball. If sort is true, the
// entries are written in sorted order; otherwise, they are written in the
// original archive's order. The ctx parameter controls early cancellation.
func (prog *Program) List(ctx context.Context, input string, sort bool) error {
	paths, errs := prog.tarPathStream(ctx, input, sort, nil)

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
