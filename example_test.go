package autobreaker_test

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/1mb-dev/autobreaker"
)

// The README Quick Start is this function's body; TestReadmeQuickStart enforces it.
// No Output comment: it calls the network, so go test compiles it but does not run it.
func Example() {
	client := &http.Client{Timeout: 5 * time.Second}

	breaker := autobreaker.New(autobreaker.Settings{
		Name:              "api-client",
		Timeout:           10 * time.Second, // Open -> HalfOpen after 10s
		AdaptiveThreshold: true,             // trip on failure rate instead of 6 consecutive failures
		Interval:          10 * time.Second, // rate measured over fixed 10s windows
	})

	result, err := breaker.Execute(func() (interface{}, error) {
		resp, err := client.Get("https://api.example.com/data")
		if err != nil {
			return nil, err
		}
		if resp.StatusCode >= 500 {
			_ = resp.Body.Close()
			return nil, fmt.Errorf("server error: %s", resp.Status)
		}
		return resp, nil
	})

	switch {
	case errors.Is(err, autobreaker.ErrOpenState), errors.Is(err, autobreaker.ErrTooManyRequests):
		fmt.Println("circuit open, using fallback")
		return
	case err != nil:
		fmt.Println("request failed:", err)
		return
	}

	resp := result.(*http.Response)
	defer func() { _ = resp.Body.Close() }()
	fmt.Println("status:", resp.Status)
}

// TestReadmeQuickStart keeps the README Quick Start identical to Example's body.
func TestReadmeQuickStart(t *testing.T) {
	readme := readFile(t, "README.md")
	start := strings.Index(readme, "## Quick Start\n")
	if start < 0 {
		t.Fatal("README.md has no Quick Start section")
	}
	got := funcBody(t, "README.md main", readme[start:], "func main() {\n", "\n}\n```")
	got = tabIndent(got)

	src := readFile(t, "example_test.go")
	want := funcBody(t, "Example", src, "func Example() {\n", "\n}\n")

	if got != want {
		t.Errorf("README Quick Start differs from Example in example_test.go.\nREADME:\n%s\nExample:\n%s", got, want)
	}

	// The body match leaves the package clause and imports unchecked; build the whole block.
	if testing.Short() {
		t.Skip("builds the README block with the go command")
	}
	block := funcBody(t, "README.md Quick Start block", readme[start:], "```go\n", "```")
	root, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	goLine := regexp.MustCompile(`(?m)^go .*$`).FindString(readFile(t, "go.mod"))
	gomod := "module quickstart\n\n" + goLine + "\n\nrequire github.com/1mb-dev/autobreaker v0.0.0\n\n" +
		"replace github.com/1mb-dev/autobreaker => " + strconv.Quote(root) + "\n"
	for name, data := range map[string]string{"go.mod": gomod, "main.go": block} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	cmd := exec.Command("go", "vet", ".")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOFLAGS=", "GOWORK=off")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Errorf("README Quick Start does not build: %v\n%s", err, out)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func funcBody(t *testing.T, name, s, open, closing string) string {
	t.Helper()
	i := strings.Index(s, open)
	if i < 0 {
		t.Fatalf("%s: %q not found", name, open)
	}
	s = s[i+len(open):]
	j := strings.Index(s, closing)
	if j < 0 {
		t.Fatalf("%s: end %q not found", name, closing)
	}
	return s[:j]
}

// tabIndent turns the README's 4-space indentation into gofmt tabs, leaving alignment inside lines alone.
func tabIndent(s string) string {
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		trimmed := strings.TrimLeft(l, " ")
		lines[i] = strings.Repeat("\t", (len(l)-len(trimmed))/4) + trimmed
	}
	return strings.Join(lines, "\n")
}
