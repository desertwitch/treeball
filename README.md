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

An important step in recovering from catastrophic data loss is knowing what you had in the first place.

`treeball` is a command-line utility for preserving directory trees as compressed archives, **replacing all files with zero-byte placeholders**. This creates lightweight, navigable tarballs that are portable, inspectable, and diffable - without scrolling through endless `tree`-style textual lists ever again.

### RATIONALE

**Have you ever tried to find something specific in a `tree`-produced list, only to drown in all that text?**  
**Wouldn't it be nice to just browse that as if it were your regular filesystem - but packed into a single file?**  

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

Beware the diff archive contains synthetic `+++` and `---` directories to reflect both additions and removals.

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

### ADVANCED OPTIONS

These flags are optional and intended for advanced users working with large-scale directories (multiple millions of files) or otherwise tuning `treeball` for specific hardware setups. Most users can safely ignore these unless dealing with performance constraints or custom environments.

#### `treeball create`

| Flag           | Description                                         | Default      |
|----------------|-----------------------------------------------------|--------------|
| `--blocksize`  | Compression block size                              | 1048576      |
| `--blockcount` | Number of compression blocks processed in parallel  | `GOMAXPROCS` |

#### `treeball diff` / `treeball list`

| Flag          | Description                                                    | Default                |
|---------------|----------------------------------------------------------------|------------------------|
| `--tmpdir`    | On-disk directory for external sorting                         | `""` (auto)            |
| `--workers`   | Number of parallel worker threads used for sorting/diffing     | `GOMAXPROCS` (max. 4)* |
| `--chunksize` | Maximum in-memory records per worker (before spilling to disk) | 100000                 |

You should use `--tmpdir` to point to high-speed local storage (e.g., NVMe scratch disk) for best performance.  
> *: When `GOMAXPROCS` is smaller than 4, that will be chosen as default, otherwise `--workers` defaults to 4.

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

**Environment A - Medium-Performance VM:**
> Intel i3-9100 3.60GHz (VM: 3 cores), 2GB RAM, Samsung SSD 980 NVMe, Ubuntu 24.04.2  
> Average path length: ~85 characters / Maximum directory depth: 5 levels / Default settings  

| Files | CREATE (Time / RAM / CPU) | DIFF (Time / RAM / CPU)   | LIST (Time / RAM / CPU)   | Treeball Size |
|-------|---------------------------|---------------------------|---------------------------|---------------|
| 5K    | 0.06 s / 11.50 MB / 133%  | 0.05 s / 16.63 MB / 180%  | 0.05 s / 10.25 MB / 100%  | 25 KB         |
| 10K   | 0.10 s / 15.38 MB / 160%  | 0.08 s / 17.75 MB / 188%  | 0.08 s / 11.25 MB / 125%  | 49 KB         |
| 50K   | 0.42 s / 28.38 MB / 198%  | 0.27 s / 26.50 MB / 215%  | 0.29 s / 15.75 MB / 135%  | 242 KB        |
| 100K  | 1.00 s / 31.37 MB / 170%  | 0.57 s / 53.13 MB / 207%  | 0.59 s / 28.63 MB / 137%  | 483 KB        |
| 500K  | 5.37 s / 34.96 MB / 161%  | 2.60 s / 85.00 MB / 224%  | 2.74 s / 41.48 MB / 144%  | 2.4 MB        |
| 1M    | 11.04 s / 36.07 MB / 158% | 5.27 s / 79.25 MB / 222%  | 5.60 s / 40.88 MB / 143%  | 4.8 MB        |
| 5M    | 54.45 s / 37.39 MB / 160% | 25.40 s / 81.12 MB / 229% | 27.08 s / 41.25 MB / 145% | 24 MB         |

> CPU usage above 100% indicates that the program is **multi-threaded** and effectively parallelized.  

**Environment B - High-Performance VM:**
> Intel i5-12600K 3.69 GHz (VM: 16 cores), 32GB RAM, Samsung SSD 980 Pro NVMe, Ubuntu 24.04.2  
> Average path length: ~85 characters / Maximum directory depth: 5 levels / Default settings  

| Files | CREATE (Time / RAM / CPU) | DIFF (Time / RAM / CPU)   | LIST (Time / RAM / CPU)  | Treeball Size |
|-------|---------------------------|---------------------------|--------------------------|---------------|
| 5K    | 0.03 s / 21.45 MB / 133%  | 0.02 s / 13.36 MB / 200%  | 0.02 s / 10.99 MB / 100% | 25 KB         |
| 10K   | 0.05 s / 24.09 MB / 180%  | 0.03 s / 21.47 MB / 200%  | 0.03 s / 13.09 MB / 100% | 49 KB         |
| 50K   | 0.25 s / 36.99 MB / 172%  | 0.11 s / 25.87 MB / 209%  | 0.10 s / 17.32 MB / 130% | 242 KB        |
| 100K  | 0.47 s / 37.82 MB / 177%  | 0.23 s / 51.90 MB / 213%  | 0.21 s / 31.37 MB / 123% | 483 KB        |
| 500K  | 2.71 s / 38.79 MB / 156%  | 1.09 s / 82.40 MB / 255%  | 0.99 s / 45.21 MB / 139% | 2.4 MB        |
| 1M    | 5.91 s / 38.57 MB / 144%  | 2.06 s / 81.76 MB / 255%  | 2.04 s / 44.82 MB / 144% | 4.8 MB        |
| 5M    | 30.48 s / 42.98 MB / 140% | 10.04 s / 82.45 MB / 257% | 9.71 s / 44.50 MB / 148% | 24 MB         |

> CPU usage above 100% indicates that the program is **multi-threaded** and effectively parallelized.  

### SECURITY, CONTRIBUTIONS, AND LICENSE

Please report any issues via the GitHub Issues tracker. While no major features are currently planned, contributions are welcome. Contributions should be submitted through GitHub and, if possible, should pass the test suite and comply with the project's linting rules. All code is licensed under the MIT license.
