<div align="center">
    <img alt="Logo" src="assets/treeball.png" width="260">
    <h1>treeball</h1>
    <p>Being able to remember is half the recovery.</p>
</div>

<div align="center">
    <a href="https://github.com/desertwitch/treeball/releases"><img alt="Release" src="https://img.shields.io/github/release/desertwitch/treeball.svg"></a>
    <a href="https://go.dev/"><img alt="Go Version" src="https://img.shields.io/badge/Go-%3E%3D%201.24.1-%23007d9c"></a>
    <a href="https://pkg.go.dev/github.com/desertwitch/treeball"><img alt="Go Reference" src="https://pkg.go.dev/badge/github.com/desertwitch/treeball.svg"></a>
    <a href="https://goreportcard.com/report/github.com/desertwitch/treeball"><img alt="Go Report" src="https://goreportcard.com/badge/github.com/desertwitch/treeball"></a>
    <a href="./LICENSE"><img alt="License" src="https://img.shields.io/github/license/desertwitch/treeball"></a>
    <br>
    <a href="https://app.codecov.io/gh/desertwitch/treeball"><img alt="Codecov" src="https://codecov.io/github/desertwitch/treeball/graph/badge.svg?token=5CR32ES41N"></a>
    <a href="https://github.com/desertwitch/treeball/actions/workflows/golangci-lint.yml"><img alt="Lint" src="https://github.com/desertwitch/treeball/actions/workflows/golangci-lint.yml/badge.svg"></a>
    <a href="https://github.com/desertwitch/treeball/actions/workflows/golang-tests.yml"><img alt="Tests" src="https://github.com/desertwitch/treeball/actions/workflows/golang-tests.yml/badge.svg"></a>
    <a href="https://github.com/desertwitch/treeball/actions/workflows/golang-build.yml"><img alt="Build" src="https://github.com/desertwitch/treeball/actions/workflows/golang-build.yml/badge.svg"></a>
</div><br>

### OVERVIEW

**treeball creates, diffs, and lists directory trees as archives.**

An important step in recovering from catastrophic data loss is knowing what you had in the first place. `treeball` is a command-line utility for preserving directory trees as compressed archives, **replacing all files with zero-byte placeholders**. This creates lightweight, navigable tarballs that are portable, inspectable, and diffable - without scrolling through endless `tree`-style textual lists ever again.

### RATIONALE

Have you ever tried browsing a large directory with `tree` or `find`, only to drown in endless text? Wouldn't it be nice to explore those massive lists as if they were your filesystem - but all packed into a single file?

`treeball` solves this by converting directory trees into `.tar.gz` archives that:

- **Preserve full structure** (all paths, directories, and filenames)
- Replace actual files with **empty dummy files** (saving a lot of space)
- Can easily be **browsed with any archive viewer**
- Support fast, efficient **diffing** between two trees
- Can be **listed** within the CLI in sorted or original order
- Enable **recovery planning** (extract dummies first, replace files later)

This turns what's normally a giant wall of text into a portable, organized snapshot.
It treats directory trees as artifacts - something you can archive, compare, and extract.

### FEATURES

#### Core commands:
- **Create** a tree tarball from any directory tree
- **Diff** two tree tarballs to detect added/removed paths
- **List** the contents of a tree tarball (sorted or original order)

#### Operational strengths:
- Works efficiently even with **millions of files** (tested up to 5M+)
- Streams data and uses external sorting for a **low resource profile**
- Clear, **scriptable output** via `stdout` / `stderr` (no useless chatter)
- Fully **tested** (including exclusion logic, signal handling, edge cases)

### COMMANDS

#### `treeball create`

Build a `.tar.gz` archive from a directory tree.

```bash
treeball create <root-folder> <output.tar.gz> [--exclude=PATH --exclude=PATH...]
```

**Examples:**

```bash
# Archive the current directory:
treeball create . tree.tar.gz

# Archive a directory with exclusions:
treeball create /data/full archive.tar.gz --exclude=/data/full/tmp --exclude=/data/full/.git
```

#### `treeball diff`

Compare two tarballs and create a diff archive reflecting structural changes (added/removed files and directories).

```bash
treeball diff <old.tar.gz> <new.tar.gz> <diff.tar.gz> [--tmpdir=PATH]
```

**Examples:**

```bash
# Basic usage of the command:
treeball diff base.tar.gz updated.tar.gz changes.tar.gz

# Just see the diff in the terminal (without file output):
treeball diff base.tar.gz updated.tar.gz /dev/null

# Use of an on-disk temporary directory (for massive archives):
treeball diff old.tar.gz new.tar.gz diff.tar.gz --tmpdir=/mnt/largedisk
```

The diff archive contains dummy entries under `+++` and `---` folders to reflect additions and removals.

#### `treeball list`

List the contents of a `.tar.gz` tree archive (sorted or unsorted).

```bash
treeball list <input.tar.gz> [--sort=false] [--tmpdir=PATH]
```

**Examples:**

```bash
# List the contents as sorted (default):
treeball list archive.tar.gz

# List the contents in their original archive order:
treeball list archive.tar.gz --sort=false

# Use of an on-disk temporary directory (for massive archives):
treeball list archive.tar.gz --tmpdir=/mnt/largedisk
```

### EXIT CODES
  - `0` - Success
  - `1` - Differences found (only for `diff`)
  - `2` - General failure (invalid input, I/O errors, etc.)

### INSTALLATION

#### Building from source:

```bash
git clone https://github.com/desertwitch/treeball.git
cd treeball
make
```

#### Running a built executable:

```bash
./treeball --help
```

### PERFORMANCE NOTES

- Designed for efficiency with millions of files - streams I/O to avoid high memory usage and pressure.
- Intelligently off-loads temporary data to disk-based locations in order to conserve system resources.
- `--tmpdir` allows full control over where temporary data is off-loaded to (e.g., to high-speed storage).

### SECURITY, CONTRIBUTIONS, AND LICENSE

Please report any issues via the GitHub Issues tracker. While no major features are currently planned, contributions are welcome. Contributions should be submitted through GitHub and, if possible, should pass the test suite and comply with the project's linting rules. All code is licensed under the MIT license.
