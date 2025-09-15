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
Directory trees are reshaped as artifacts - something you can archive, compare, and extract.

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

Beware excludes must be written in the same absolute/relative form as the `<root-folder>`.

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

Beware the diff archive contains synthetic `+++` and `---` folders to reflect both additions and removals.

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

#### Benchmarks:

| Files     | CREATE (Time / RAM / CPU)  | DIFF (Time / RAM / CPU)  | LIST (Time / RAM / CPU)  | Treeball Size |
|-----------|----------------------------|--------------------------|--------------------------|---------------|
| 5K        | 0.06 s / 12.8 MB / 133%    | 0.06 s / 17.4 MB / 167%  | 0.05 s / 10.4 MB / 100%  | 25 KB         |
| 10K       | 0.12 s / 14.2 MB / 150%    | 0.09 s / 18.7 MB / 200%  | 0.08 s / 11.5 MB / 113%  | 49 KB         |
| 50K       | 0.47 s / 30.5 MB / 181%    | 0.27 s / 27.6 MB / 215%  | 0.29 s / 15.9 MB / 135%  | 242 KB        |
| 100K      | 1.00 s / 35.7 MB / 169%    | 0.55 s / 50.4 MB / 215%  | 0.59 s / 28.4 MB / 137%  | 483 KB        |
| 500K      | 5.22 s / 38.6 MB / 163%    | 2.58 s / 83.5 MB / 226%  | 2.70 s / 41.9 MB / 145%  | 2.4 MB        |
| 1M        | 10.84 s / 36.2 MB / 161%   | 5.59 s / 82.8 MB / 215%  | 5.65 s / 41.1 MB / 143%  | 4.8 MB        |
| 5M        | 55.44 s / 39.7 MB / 157%   | 25.76 s / 82.9 MB / 230% | 26.90 s / 41.1 MB / 146% | 24 MB         |

> CPU usage above 100% indicates that the program is **multi-threaded** and effectively parallelized.  
> 200% CPU usage on a system with 3 cores means the process is using **two full cores concurrently**.  

#### Benchmark Environment:

- **CPU**: Intel® Core™ i3-9100 @ 3.60GHz
- **Cores available to VM**: 3
- **Memory**: 2 GB RAM
- **Filesystem**: XFS
- **Disk**: Samsung SSD 980 NVMe
- **OS**: Ubuntu 24.04.2 LTS (noble) 
- **Average path length**: ~85 characters
- **Maximum directory depth**: 5 levels

### SECURITY, CONTRIBUTIONS, AND LICENSE

Please report any issues via the GitHub Issues tracker. While no major features are currently planned, contributions are welcome. Contributions should be submitted through GitHub and, if possible, should pass the test suite and comply with the project's linting rules. All code is licensed under the MIT license.
