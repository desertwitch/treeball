package main

import (
	"bytes"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

// Expectation: An exclude pattern should select the same paths whether the source is a folder or a tarball,
// so diffing a tarball against the folder it came from stays clean under any exclude.
func Test_treeExcluder_FolderAndTarballAgree_Table(t *testing.T) {
	fs := afero.NewMemMapFs()
	writeTree(t, fs, "/src", []string{
		"movies/m.mkv",
		"movies/m.nfo",
		"@eaDir/thumbs/t.jpg",
		"movies/@eaDir/x.jpg",
		"tv/show/s01/e01.mkv",
		"tv/show/s01/e01.nfo",
		"tv/@eaDir", // a FILE named like the excluded dir
	})
	require.NoError(t, NewProgram(fs, &bytes.Buffer{}, &bytes.Buffer{}, nil, nil).Create(t.Context(), "/src", "/full.tar.gz", nil))

	patterns := []string{
		"@eaDir", "@eaDir/", "**/@eaDir", "**/@eaDir/", "**/@eaDir/**",
		"**/*.nfo", "tv/show", "tv/*/s01/", "movies/**",
	}

	for _, pattern := range patterns {
		t.Run(pattern, func(t *testing.T) {
			excl := []string{pattern}

			// folder side: what create archives with the exclude
			require.NoError(t, NewProgram(fs, &bytes.Buffer{}, &bytes.Buffer{}, nil, nil).Create(t.Context(), "/src", "/excl.tar.gz", excl))
			fromFolder := listArchive(t, fs, "/excl.tar.gz", true, nil, nil)

			// tarball side: what list shows from the full archive with the same exclude
			fromTarball := listArchive(t, fs, "/full.tar.gz", true, excl, nil)

			require.Equal(t, fromFolder, fromTarball)

			var out bytes.Buffer
			_, err := NewProgram(fs, &out, &bytes.Buffer{}, nil, nil).Diff(t.Context(), "/full.tar.gz", "/src", "/d.tar.gz", excl)
			require.NoError(t, err, "phantom differences:\n%s", out.String())
		})
	}
}

// Expectation: The exclusions from the table should meet their respective expectations.
func Test_isExcluded_Table(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		isDir    bool
		excludes []string
		expected bool
	}{
		// === Exact literal matches ===
		{"Exact file match", "foo.txt", false, []string{"foo.txt"}, true},
		{"Exact path match", "src/main.go", false, []string{"src/main.go"}, true},
		{"Exact dir match", "build", true, []string{"build"}, true},
		{"Exact dir only match", "build", true, []string{"build/"}, true},
		{"Exact dir only match", "build", false, []string{"build/"}, false},
		{"No match different name", "foo.txt", false, []string{"bar.txt"}, false},
		{"No match different path", "lib/main.go", false, []string{"src/main.go"}, false},

		// === Single star (*) - matches within single path component ===
		{"Star matches single component", "test.go", false, []string{"*.go"}, true},
		{"Star in middle", "test_file.go", false, []string{"test_*.go"}, true},
		{"Star at start", "main.go", false, []string{"*.go"}, true},
		{"Star matches empty", "test.go", false, []string{"test*.go"}, true},
		{"Multiple stars in pattern", "test_main_file.go", false, []string{"*_*_*.go"}, true},
		{"Star does NOT cross slash", "src/main.go", false, []string{"*.go"}, false},
		{"Star in path component", "src/test.go", false, []string{"src/*.go"}, true},
		{"Star no match across dirs", "src/lib/main.go", false, []string{"src/*.go"}, false},

		// === Double star (**) - recursive directory matching ===
		{"Doublestar matches anything", "a/b/c/file.go", false, []string{"**"}, true},
		{"Doublestar with extension", "deep/nested/file.go", false, []string{"**/*.go"}, true},
		{"Doublestar at start", "any/path/main.go", false, []string{"**/*.go"}, true},
		{"Doublestar in middle", "src/any/deep/main.go", false, []string{"src/**/main.go"}, true},
		{"Doublestar matches zero dirs", "src/main.go", false, []string{"src/**/main.go"}, true},
		{"Doublestar matches multiple levels", "a/b/c/d/e.txt", false, []string{"a/**/e.txt"}, true},
		{"Multiple doublestars", "a/b/c/d/e.txt", false, []string{"a/**/c/**/e.txt"}, true},
		{"Doublestar exact subpath", "vendor/pkg", true, []string{"vendor/**"}, true},

		// === Question mark (?) - single character ===
		{"Question mark matches one char", "a.txt", false, []string{"?.txt"}, true},
		{"Question mark no match multiple", "ab.txt", false, []string{"?.txt"}, false},
		{"Question mark no match zero", ".txt", false, []string{"?.txt"}, false},
		{"Multiple question marks", "ab.txt", false, []string{"??.txt"}, true},
		{"Question mark in path", "src/a.go", false, []string{"src/?.go"}, true},
		{"Question mark does not match slash", "a/b", true, []string{"a?b"}, false},

		// === Character classes [...] ===
		{"Char class range", "a.txt", false, []string{"[a-z].txt"}, true},
		{"Char class range no match", "A.txt", false, []string{"[a-z].txt"}, false},
		{"Char class explicit", "a.txt", false, []string{"[abc].txt"}, true},
		{"Char class explicit no match", "d.txt", false, []string{"[abc].txt"}, false},
		{"Negated char class", "d.txt", false, []string{"[!abc].txt"}, true},
		{"Negated char class no match", "a.txt", false, []string{"[!abc].txt"}, false},
		{"Char class with numbers", "1.txt", false, []string{"[0-9].txt"}, true},
		{"Char class mixed", "a.txt", false, []string{"[a-z0-9].txt"}, true},

		// === Brace expansion {a,b,c} ===
		{"Brace expansion first option", "file.go", false, []string{"file.{go,py,js}"}, true},
		{"Brace expansion second option", "file.py", false, []string{"file.{go,py,js}"}, true},
		{"Brace expansion third option", "file.js", false, []string{"file.{go,py,js}"}, true},
		{"Brace expansion no match", "file.txt", false, []string{"file.{go,py,js}"}, false},
		{"Brace in path", "src/main.go", false, []string{"{src,lib}/main.go"}, true},
		{"Nested braces", "a1.txt", false, []string{"{a,b}{1,2}.txt"}, true},
		{"Empty brace option", "file.", false, []string{"file.{go,}"}, true},

		// === Dotfiles and hidden files ===
		{"Hidden file match", ".gitignore", false, []string{".gitignore"}, true},
		{"Hidden dir match", ".git", true, []string{".git"}, true},
		{"Hidden file with star", ".bashrc", false, []string{".*"}, true},
		{"Nested hidden file", "project/.git/config", false, []string{"**/.git/config"}, true},
		{"Hidden dir recursive", "a/.hidden/b/file", false, []string{"**/.hidden/**"}, true},
		{"Star matches dot prefix", ".hidden", false, []string{"*hidden"}, true},
		{"Doublestar matches dot prefix", "a/.hidden/file", false, []string{"a/**"}, true},

		// === Edge cases with slashes ===
		{"Pattern with internal double slash", "a/b/c", false, []string{"a//b/c"}, false},
		{"Empty path component in pattern", "a/c", false, []string{"a//c"}, false},
		{"Sub-path", "sub/foo/bar", true, []string{"foo/bar"}, false},
		{"Non-sub-path dir", "foo/bar", true, []string{"foo/bar"}, true},
		{"Non-sub-path file", "foo/bar", false, []string{"foo/bar"}, true},
		{"Leading slash stripped dir", "sub/foo/bar", true, []string{"/foo/bar"}, false},
		{"Leading slash stripped file", "sub/foo/bar", false, []string{"/foo/bar"}, false},
		{"Leading slash stripped", "vendor/lib.go", false, []string{"/vendor/**"}, true},

		// === Complex real-world patterns ===
		{"node_modules anywhere", "project/node_modules/pkg/index.js", false, []string{"**/node_modules/**"}, true},
		{"Specific file in any node_modules", "a/node_modules/pkg/package.json", false, []string{"**/node_modules/**/package.json"}, true},
		{"Build artifacts", "target/release/binary", false, []string{"target/**"}, true},
		{"Test files", "src/utils_test.go", false, []string{"**/*_test.go"}, true},
		{"Backup files", "config.bak", false, []string{"*.bak"}, true},
		{"Temp files", "file.tmp", false, []string{"*.tmp"}, true},
		{"Log files anywhere", "app/logs/app.log", false, []string{"**/*.log"}, true},
		{"OS specific", ".DS_Store", false, []string{".DS_Store"}, true},
		{"Editor files", "main.go.swp", false, []string{"*.swp"}, true},

		// === Multiple patterns (OR logic) ===
		{"First pattern matches", "test.go", false, []string{"*.go", "*.py"}, true},
		{"Second pattern matches", "test.py", false, []string{"*.go", "*.py"}, true},
		{"Neither pattern matches", "test.txt", false, []string{"*.go", "*.py"}, false},
		{"Complex multiple patterns", "src/test.go", false, []string{"docs/**", "src/**/*.go", "*.tmp"}, true},

		// === Unicode and special characters ===
		{"Unicode filename", "файл.txt", false, []string{"файл.txt"}, true},
		{"Unicode in pattern", "test/файл.go", false, []string{"test/*.go"}, true},
		{"Spaces in filename", "my file.txt", false, []string{"my file.txt"}, true},
		{"Spaces with wildcard", "my test.txt", false, []string{"my *.txt"}, true},

		// === Escaping and special characters ===
		{"Brackets as char class", "test1.txt", false, []string{"test[1].txt"}, true},
		{"Literal star in filename", "test*.txt", false, []string{"test\\*.txt"}, true},
		{"Literal question in filename", "what?.txt", false, []string{"what\\?.txt"}, true},
		{"Literal star in filename", "test*a.txt", false, []string{"test\\*.txt"}, false},
		{"Literal question in filename", "what?a.txt", false, []string{"what\\?.txt"}, false},

		// === Performance and edge cases ===
		{"Empty pattern", "anything", false, []string{""}, false},
		{"Only wildcards", "anything", false, []string{"**"}, true},
		{"Deep nesting", "a/b/c/d/e/f/g/h.txt", false, []string{"a/**/h.txt"}, true},
		{"Many alternatives", "test.go", false, []string{"*.{go,py,js,cpp,c,h,hpp,java,kt,scala,clj}"}, true},

		// === Negative cases - common mistakes ===
		{"Star doesn't match path separator", "a/b", false, []string{"a*b"}, false},
		{"Question doesn't match path separator", "a/b", false, []string{"a?b"}, false},
		{"Single star is not recursive", "a/b/c.txt", false, []string{"*.txt"}, false},
		{"Doublestar in middle of word", "abc/def.txt", false, []string{"a**/def.txt"}, true},
		{"Char class doesn't match multiple", "ab.txt", false, []string{"[ab].txt"}, false},

		// === Case sensitivity (Unix is case-sensitive) ===
		{"Case sensitive match", "File.TXT", false, []string{"File.TXT"}, true},
		{"Case sensitive no match", "file.txt", false, []string{"File.TXT"}, false},
		{"Case sensitive wildcards", "FILE.txt", false, []string{"*.txt"}, true},
		{"Case sensitive char class", "A.txt", false, []string{"[A-Z].txt"}, true},
		{"Case sensitive char class no match", "a.txt", false, []string{"[A-Z].txt"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := isExcluded(tt.path, tt.isDir, tt.excludes)
			require.NoError(t, err)
			require.Equalf(t, tt.expected, got,
				"path=%q, isDir=%v, patterns=%v",
				tt.path, tt.isDir, tt.excludes)
		})
	}
}

// Expectation: The function should reject an invalid pattern and return an error.
func Test_isExcluded_InvalidExcludePattern_Error(t *testing.T) {
	result, err := isExcluded("/a/b/c", true, []string{"a["})

	require.Error(t, err)
	require.ErrorContains(t, err, "pattern")
	require.False(t, result)
}
