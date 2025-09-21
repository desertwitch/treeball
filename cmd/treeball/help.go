package main

const (
	rootHelpShort = "treeball creates, diffs, and lists directory trees as archives."

	rootHelpLong = `treeball creates, diffs, and lists directory trees as archives.

It preserves directory trees as compressed archives, replacing all files with zero-byte
placeholder files. This creates lightweight tarballs that are portable, navigable, and
diffable. Think of browsable inventory-type backups of e.g. media libraries, but without
overhead of preserving the file contents.

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

For detailed help on a specific command, run:
  treeball help <command>`

	createHelpShort = "Create a tarball representing any given directory tree"

	createHelpLong = `Create a tarball representing any given directory tree.

The command will recursively include all files and directories under <root-folder>.
Files will be compressed as zero-byte placeholder files with their names preserved.

Excludes are expected as relative to <root-folder> and following 'doublestar' format:
https://github.com/bmatcuk/doublestar?tab=readme-ov-file#patterns

All paths written to the tarball will be printed to standard output (stdout), any errors
or other relevant operational output will be printed to standard error (stderr) respectively.
The command will return with an exit code 0 in case of success; an exit code 2 for any errors.`

	createExample = `
# Archive the current directory:
treeball create . output.tar.gz

# Archive a directory with exclusions:
treeball create /mnt/data output.tar.gz --exclude='src/**/main.go'

# Archive a directory with exclusions from a file:
treeball create /mnt/data output.tar.gz --excludes-from=./excludes.txt`

	diffHelpShort = "Create a diff tarball from any two pre-existing sources"

	diffHelpLong = `Create a diff tarball containing only the structural differences between any two sources.

The command will compare the structure of two existing (directory tree) sources and produce
a "diff" tarball reflecting any additions or removals, comparing the "old" and "new" sources.
This helps to identify which paths were recently added or lost (e.g., for recovery scenarios).

The command supports sources as either an existing directory or an existing tarball (.tar.gz).
This means you can compare tar vs. tar, tar vs. dir, dir vs. tar and dir vs. dir respectively.

Excludes are expected as relative to given sources and following 'doublestar' format:
https://github.com/bmatcuk/doublestar?tab=readme-ov-file#patterns

Any differences will also be written to standard output (stdout), while any other operational
output will be written to standard error (stderr). The program will return with an exit code
0 in case no differences were found; with an exit code 1 in case some differences were found.

Performance considerations with massive archives:
The external sorting mechanism may off-load excess data to on-disk locations to conserve RAM.
Ensure that a suitable --tmpdir is provided (in terms of speed and available space), as such
data can peak at multiple gigabytes. If none is provided, the intelligent mechanism will try
choose one for you, falling back to the system's default temporary file location on failure.`

	diffExample = `
# Basic usage of the command:
treeball diff old.tar.gz new.tar.gz diff.tar.gz

# Basic usage of the command with directory comparison:
treeball diff old.tar.gz /mnt/new diff.tar.gz

# Just see the diff in the terminal (without file output):
treeball diff old.tar.gz new.tar.gz /dev/null

# Use of an on-disk temporary directory (for massive archives):
treeball diff old.tar.gz new.tar.gz diff.tar.gz --tmpdir=/mnt/largedisk`

	listHelpShort = "List the paths contained in a tarball (sorted by default)"

	listHelpLong = `List all contained paths in a tarball, either sorted or in original order.

By default, the paths are sorted alphabetically, which improves readability and makes it
easier to 'diff' or otherwise compare. --sort=false preserves the original archive order,
if that would otherwise be needed.

All listed paths are printed to standard output (stdout), while any operational output and
encountered errors will be written to standard error (stderr) respectively. The command
returns with an exit code 0 upon success; an exit code 2 for any encountered errors.

Performance considerations with massive archives:
The external sorting mechanism may off-load excess data to on-disk locations to conserve RAM.
Ensure that a suitable --tmpdir is provided (in terms of speed and available space), as such
data can peak at multiple gigabytes. If none is provided, the intelligent mechanism will try
choose one for you, falling back to the system's default temporary file location on failure.`

	listExample = `
# List the contents as sorted (default):
treeball list input.tar.gz

# List the contents in their original archive order:
treeball list input.tar.gz --sort=false

# Use of an on-disk temporary directory (for massive archives):
treeball list input.tar.gz --tmpdir=/mnt/largedisk`
)
