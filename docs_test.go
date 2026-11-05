package autobreaker_test

import (
	"bufio"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Phrasings that state a minimum Go version. A version named for another purpose
// (e.g. the toolchain a benchmark ran on) must not use one of these forms.
var goRequirement = regexp.MustCompile(
	`Go (1\.\d+)(?:\.\d+)? or later|Go (1\.\d+)(?:\.\d+)?\+|Requires Go (1\.\d+)|^\s*[-*] Go (1\.\d+)`)

// TestDocumentedGoVersion keeps every documented Go requirement equal to go.mod's
// go line, which is the floor consumers get.
func TestDocumentedGoVersion(t *testing.T) {
	want := goModVersion(t)

	files := []string{"README.md", ".github/CONTRIBUTING.md"}
	err := filepath.WalkDir("docs/content", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(path, ".md") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	found := 0
	for _, path := range files {
		f, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		sc := bufio.NewScanner(f)
		for line := 1; sc.Scan(); line++ {
			for _, m := range goRequirement.FindAllStringSubmatch(sc.Text(), -1) {
				found++
				if got := strings.Join(m[1:], ""); got != want {
					t.Errorf("%s:%d: documents Go %s, go.mod says %s: %q", path, line, got, want, m[0])
				}
			}
		}
		if err := sc.Err(); err != nil {
			t.Fatal(err)
		}
		_ = f.Close()
	}
	if found == 0 {
		t.Fatal("no Go version requirement found in the docs; the patterns no longer match how the docs state it")
	}
}

func goModVersion(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile("go.mod")
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`(?m)^go (1\.\d+)`).FindSubmatch(data)
	if m == nil {
		t.Fatal("go.mod has no go line")
	}
	return string(m[1])
}
