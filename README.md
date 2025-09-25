<div align="center">
    <img alt="Logo" src="assets/treeball.png" width="260">
    <h1>treeball</h1>
    <p>Keeping inventory is half the recovery.</p>
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

`treeball` is a command-line utility for preserving directory trees as compressed archives, **replacing all files with zero-byte placeholder files**. This creates lightweight tarballs that are portable, navigable, and diffable. Think of **browsable inventory-type backups** of e.g. media libraries, but without the overhead of preserving file contents.

### RATIONALE

An important step in recovering from catastrophic data loss is knowing what you had in the first place.  
But have you ever tried to find something specific in a `tree`-produced list, only to drown in all that text?  
Wouldn't it be nice to just browse that as if it were your regular filesystem - but packed into a single file?  

`treeball` solves this by converting directory trees into `.tar.gz` archives that:

- **Preserve full structure** (all paths, directories, and filenames)
- Replace actual files with **zero-byte placeholder files** (saving a lot of space)
- Can easily be **browsed with any archive viewer**
- Support fast, efficient **diffing** between two trees
- Can be **listed** within the CLI in sorted or original order
- Enable **recovery planning** (extract stubs first, replace files later)

This turns what's normally a giant wall of text into a portable, well organized snapshot.  
Directory trees are reshaped as artifacts - something you can archive, compare, and extract.  

### FEATURES

#### Core commands:
- **Create** a tree tarball from any directory tree
- **Diff** two tree sources to detect added/removed paths
- **List** the contents of a tree tarball (sorted or original order)

#### Operational strengths:
- Works efficiently even with **millions of files** (see [benchmarks](#benchmarks))
- Streams data and uses external sorting for a **low resource profile**
- Clear, **scriptable output** via `stdout` / `stderr` (no useless chatter)
- Fully **tested** (including exclusion logic, signal handling, edge cases)

### COMMANDS

#### `treeball create`

Build a `.tar.gz` archive from a directory tree.

```bash
treeball create <root-folder> <output.tar.gz> [--exclude=PATTERN] [--excludes-from=PATH]
```

**Examples:**

```bash
# Archive the current directory:
treeball create . output.tar.gz

# Archive a directory with exclusions:
treeball create /mnt/data output.tar.gz --exclude='src/**/main.go'

# Archive a directory with exclusions from a file:
treeball create /mnt/data output.tar.gz --excludes-from=./excludes.txt
```

#### `treeball diff`

Compare two sources and create a diff archive reflecting structural changes (added/removed files and directories).

```bash
treeball diff <old> <new> <diff.tar.gz> [--tmpdir=PATH] [--exclude=PATTERN] [--excludes-from=PATH]
```

The command supports sources as either an existing directory or an existing tarball (`.tar.gz`).  
This means you can compare tar vs. tar, tar vs. dir, dir vs. tar and dir vs. dir respectively.  

**Examples:**

```bash
# Basic usage of the command:
treeball diff old.tar.gz new.tar.gz diff.tar.gz

# Basic usage of the command with directory comparison:
treeball diff old.tar.gz /mnt/new diff.tar.gz

# Just see the diff in the terminal (without file output):
treeball diff old.tar.gz new.tar.gz /dev/null

# Use of an on-disk temporary directory (for massive archives):
treeball diff old.tar.gz new.tar.gz diff.tar.gz --tmpdir=/mnt/largedisk
```

Beware the `diff` archive contains synthetic `+++` and `---` directories to reflect both additions and removals.

> **Performance considerations with massive archives:**
> The external sorting mechanism may off-load excess data to on-disk locations (controllable with `--tmpdir`) to conserve RAM.
> Ensure that a suitable location is provided (in terms of speed and available space), as such data can peak at multiple gigabytes.
> If none is provided, the intelligent mechanism will try choose one for you, falling back to the system's default temporary file location.

#### `treeball list`

List the contents of a `.tar.gz` tree archive (as sorted or unsorted).

```bash
treeball list <input.tar.gz> [--tmpdir=PATH] [--sort=false] [--exclude=PATTERN] [--excludes-from=PATH]
```

**Examples:**

```bash
# List the contents as sorted (default):
treeball list input.tar.gz

# List the contents in their original archive order:
treeball list input.tar.gz --sort=false

# Use of an on-disk temporary directory (for massive archives):
treeball list input.tar.gz --tmpdir=/mnt/largedisk
```

> **Performance considerations with massive archives:**
> The external sorting mechanism may off-load excess data to on-disk locations (controllable with `--tmpdir`) to conserve RAM.
> Ensure that a suitable location is provided (in terms of speed and available space), as such data can peak at multiple gigabytes.
> If none is provided, the intelligent mechanism will try choose one for you, falling back to the system's default temporary file location.

### EXCLUDE PATTERNS

Exclusion patterns are expected to always be relative to the given input directory tree.  
This means, passing `/mnt/user` to a command, `a.txt` would exclude `/mnt/user/a.txt`.  

`--exclude` arguments can be repeated multiple times, and/or a `--excludes-from` file be loaded.  
If either type of argument is given, all exclusion patterns are merged together at program runtime.  

All exclusion patterns are expected to follow the `doublestar`-format:  
https://github.com/bmatcuk/doublestar?tab=readme-ov-file#patterns

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

| Flag          | Description                                                    | Default                               |
|---------------|----------------------------------------------------------------|---------------------------------------|
| `--tmpdir`    | On-disk directory for external sorting                         | `""` (auto) <sup>1,</sup><sup>2</sup> |
| `--workers`   | Number of parallel worker threads used for sorting/diffing     | `GOMAXPROCS` <sup>3</sup>             |
| `--chunksize` | Maximum in-memory records per worker (before spilling to disk) | 100000                                |

> <sup>1</sup> You should use `--tmpdir` to point to high-speed storage (e.g., NVMe scratch disk) for best performance.  
> <sup>2</sup> You should ensure `--tmpdir` has sufficient free space of up to several gigabytes for advanced workloads.  
> <sup>3</sup> When `GOMAXPROCS` is smaller than 4, that will be chosen as _default_ - otherwise `--workers` will _default_ to 4.  

### EXIT CODES
  - `0` - Success
  - `1` - Differences found (only for `diff`)
  - `2` - General failure (invalid input, I/O errors, etc.)

### INSTALLATION

To build from source, a `Makefile` is included with the project's source code.
Running `make all` will compile the application and pull in any necessary
dependencies. `make check` runs the test suite and static analysis tools.

For convenience, precompiled static binaries for common architectures are
released through GitHub. These can be installed into `/usr/bin/` or respective
system locations; ensure they are executable by running `chmod +x` before use.

> All builds from source are designed to generate [reproducible builds](https://reproducible-builds.org/),
> meaning that they should compile as byte-identical to the respective released binaries and also have the
> exact same checksums upon integrity verification.

#### Building from source:

```bash
git clone https://github.com/desertwitch/treeball.git
cd treeball
make all
```

#### Running a built executable:

```bash
./treeball --help
```

### BENCHMARKS

Benchmarks demonstrate consistent [performance](./PERFORMANCE.md) across small to large directory trees.

| Files  | CREATE (Time / RAM / CPU)    | DIFF (Time / RAM / CPU)      | LIST (Time / RAM / CPU)      | Treeball Size |
|--------|------------------------------|------------------------------|------------------------------|---------------|
| 10K    | 0.04 s / 29.44 MB / 200%     | 0.04 s / 16.58 MB / 150%     | 0.04 s / 13.53 MB / 75%      | 49 KB         |
| 500K   | 0.94 s / 55.47 MB / 435%     | 1.39 s / 88.57 MB / 243%     | 1.31 s / 45.94 MB / 140%     | 2.4 MB        |
| **1M** | **1.77 s / 58.91 MB / 469%** | **2.44 s / 88.16 MB / 263%** | **2.17 s / 46.23 MB / 141%** | **4.8 MB**    |
| 5M     | 12.99 s / 62.83 MB / 321%    | 11.81 s / 84.08 MB / 250%    | 10.74 s / 46.04 MB / 146%    | 24 MB         |
| 10M    | 29.27 s / 59.39 MB / 291%    | 22.92 s / 86.21 MB / 256%    | 22.12 s / 46.03 MB / 140%    | 48 MB         |

> CPU usage above 100% indicates that the program is **multi-threaded** and effectively parallelized.  
> RAM usage per million files drops significantly with scale due to **external sorting** and streaming data.  
> Stress tests with trees of **up to 500 million files** have shown continued [low resource consumption](./PERFORMANCE.md) trends.  

**Benchmark Environment**:  
Average path length: ~80 characters / Maximum directory depth: 5 levels  
3x `--exclude` / `--tmpdir` (on same disk) / Maximum compression level (9)  
i5-12600K 3.69 GHz (16 cores), 32GB RAM, 980 Pro NVMe (EXT4), Ubuntu 24.04.2  

### ACKNOWLEDGEMENTS

This program would not be possible without Ian Foster's amazing `extsort` library:  
[https://github.com/lanrat/extsort](https://github.com/lanrat/extsort) / [https://pkg.go.dev/github.com/lanrat/extsort](https://pkg.go.dev/github.com/lanrat/extsort)

### SECURITY, CONTRIBUTIONS, AND LICENSE

Please report any issues via the GitHub Issues tracker. While no major features are currently planned, contributions are welcome. Contributions should be submitted through GitHub and, if possible, should pass the test suite and comply with the project's linting rules. All code is licensed under the MIT license.
