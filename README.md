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

This turns what's normally a giant wall of text into a portable, well organized snapshot.  
Directory trees are reshaped as artifacts - something you can archive, compare, and extract.  

### FEATURES

#### Core commands:
- **Create** a tree tarball from any directory tree
- **Diff** two tree tarballs to detect added/removed paths
- **List** the contents of a tree tarball (sorted or original order)

#### Operational strengths:
- Works efficiently even with **millions of files** (tested up to **50M+**)
- Streams data and uses external sorting for a **low resource profile**
- Clear, **scriptable output** via `stdout` / `stderr` (no useless chatter)
- Fully **tested** (including exclusion logic, signal handling, edge cases)

### COMMANDS

#### `treeball create`

Build a `.tar.gz` archive from a directory tree.

```bash
treeball create <root-folder> <output.tar.gz> [--exclude=PATH --exclude=PATH...]
```

Beware `--exclude` paths must be written in the same absolute/relative form as the `<root-folder>`.

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

Beware the `diff` archive contains synthetic `+++` and `---` directories to reflect both additions and removals.

> **Performance considerations with massive archives:**
> The external sorting mechanism may off-load excess data to on-disk locations (controllable with `--tmpdir`) to conserve RAM.
> Ensure that a suitable location is provided (in terms of speed and available space), as such data can peak at multiple gigabytes.
> If none is provided, the intelligent mechanism will try choose one for you, falling back to the system's default temporary file location.

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

> **Performance considerations with massive archives:**
> The external sorting mechanism may off-load excess data to on-disk locations (controllable with `--tmpdir`) to conserve RAM.
> Ensure that a suitable location is provided (in terms of speed and available space), as such data can peak at multiple gigabytes.
> If none is provided, the intelligent mechanism will try choose one for you, falling back to the system's default temporary file location.

### ADVANCED OPTIONS

These optional options allow for more granular control with advanced workloads or environments.

#### `treeball create`

| Flag           | Description                                         | Default      |
|----------------|-----------------------------------------------------|--------------|
| `--blocksize`  | Compression block size                              | 1048576      |
| `--blockcount` | Number of compression blocks processed in parallel  | `GOMAXPROCS` |

#### `treeball create` / `treeball diff`

| Flag            | Description                                          | Default |
|-----------------|------------------------------------------------------|---------|
| `--compression` | Targeted level of compression (0: none - 9: highest) | 9       |

#### `treeball diff` / `treeball list`

| Flag          | Description                                                    | Default                      |
|---------------|----------------------------------------------------------------|------------------------------|
| `--tmpdir`    | On-disk directory for external sorting                         | `""` (auto) $^{1}$ $^{2}$    |
| `--workers`   | Number of parallel worker threads used for sorting/diffing     | `GOMAXPROCS` (max. 4) $^{3}$ |
| `--chunksize` | Maximum in-memory records per worker (before spilling to disk) | 100000                       |

> $^{1}$ You should use `--tmpdir` to point to high-speed storage (e.g., NVMe scratch disk) for best performance.  
> $^{2}$ You should ensure `--tmpdir` has sufficient free space of up to several gigabytes for advanced workloads.  
> $^{3}$ When `GOMAXPROCS` is smaller than 4, that will be chosen as _default_ - otherwise `--workers` will _default_ to 4.  

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

### BENCHMARKS

#### Environment A - Low-Performance VM:
Intel i7-10710U 1.10GHz (VM: 2 cores), 2GB RAM, Samsung 970 EVO Plus NVMe (EXT4), Ubuntu 24.04.3  
Average path length: ~80 characters / Maximum directory depth: 5 levels / Defaults  

| Files | CREATE (Time / RAM / CPU) | DIFF (Time / RAM / CPU)    | LIST (Time / RAM / CPU)   | Treeball Size |
|-------|---------------------------|----------------------------|---------------------------|---------------|
| 5K    | 0.05 s / 10.6 MB / 120%   | 0.05 s / 16.6 MB / 140%    | 0.04 s / 9.9 MB / 100%    | 25 KB         |
| 10K   | 0.09 s / 14.5 MB / 167%   | 0.08 s / 18.5 MB / 175%    | 0.07 s / 11.3 MB / 100%   | 49 KB         |
| 50K   | 0.34 s / 28.4 MB / 188%   | 0.21 s / 26.2 MB / 186%    | 0.24 s / 15.9 MB / 113%   | 242 KB        |
| 100K  | 0.67 s / 34.9 MB / 194%   | 0.47 s / 57.8 MB / 177%    | 0.56 s / 29.8 MB / 123%   | 483 KB        |
| 500K  | 3.42 s / 40.7 MB / 189%   | 2.09 s / 85.1 MB / 190%    | 2.31 s / 40.3 MB / 125%   | 2.4 MB        |
| 1M    | 7.48 s / 41.3 MB / 190%   | 4.58 s / 80.4 MB / 193%    | 4.66 s / 39.6 MB / 127%   | 4.8 MB        |
| 5M    | 39.23 s / 42.4 MB / 185%  | 22.19 s / 75.8 MB / 195%   | 22.99 s / 39.6 MB / 128%  | 24 MB         |
| 10M   | 77.21 s / 42.8 MB / 185%  | 45.39 s / 78.8 MB / 194%   | 45.45 s / 39.4 MB / 128%  | 48 MB         |
| 25M   | 194.08 s / 43.0 MB / 184% | 113.89 s / 80.4 MB / 193%  | 115.03 s / 40.6 MB / 128% | 119 MB        |
| 50M   | 388.17 s / 42.9 MB / 184% | 227.51 s / 136.7 MB / 193% | 231.13 s / 71.3 MB / 128% | 237 MB        |

#### Environment B - High-Performance VM:
Intel i5-12600K 3.69 GHz (VM: 16 cores), 32GB RAM, Samsung 980 Pro NVMe (EXT4), Ubuntu 24.04.2  
Average path length: ~80 characters / Maximum directory depth: 5 levels / Defaults  

| Files | CREATE (Time / RAM / CPU)  | DIFF (Time / RAM / CPU)    | LIST (Time / RAM / CPU)   | Treeball Size |
|-------|----------------------------|----------------------------|---------------------------|---------------|
| 5K    | 0.03 s / 21.38 MB / 100%   | 0.02 s / 17.23 MB / 150%   | 0.02 s / 10.93 MB / 100%  | 25 KB         |
| 10K   | 0.04 s / 26.63 MB / 200%   | 0.04 s / 14.73 MB / 175%   | 0.03 s / 13.02 MB / 100%  | 49 KB         |
| 50K   | 0.12 s / 43.55 MB / 342%   | 0.11 s / 24.84 MB / 200%   | 0.10 s / 19.25 MB / 120%  | 242 KB        |
| 100K  | 0.22 s / 52.78 MB / 373%   | 0.24 s / 52.11 MB / 217%   | 0.21 s / 32.43 MB / 133%  | 483 KB        |
| 500K  | 0.95 s / 55.22 MB / 425%   | 1.05 s / 81.90 MB / 255%   | 0.95 s / 43.38 MB / 148%  | 2.4 MB        |
| 1M    | 1.94 s / 57.23 MB / 422%   | 1.97 s / 81.84 MB / 253%   | 1.87 s / 43.13 MB / 151%  | 4.8 MB        |
| 5M    | 12.99 s / 62.99 MB / 317%  | 9.97 s / 82.31 MB / 252%   | 9.32 s / 47.24 MB / 151%  | 24 MB         |
| 10M   | 29.78 s / 58.88 MB / 277%  | 19.37 s / 84.13 MB / 260%  | 18.80 s / 45.23 MB / 150% | 48 MB         |
| 25M   | 87.34 s / 58.86 MB / 240%  | 47.95 s / 92.37 MB / 268%  | 46.10 s / 44.89 MB / 152% | 119 MB        |
| 50M   | 172.75 s / 61.81 MB / 241% | 99.35 s / 142.35 MB / 265% | 92.54 s / 74.84 MB / 152% | 237 MB        |

> CPU usage above 100% indicates that the program is **multi-threaded** and effectively parallelized.  

### SECURITY, CONTRIBUTIONS, AND LICENSE

Please report any issues via the GitHub Issues tracker. While no major features are currently planned, contributions are welcome. Contributions should be submitted through GitHub and, if possible, should pass the test suite and comply with the project's linting rules. All code is licensed under the MIT license.
