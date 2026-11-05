package autobreaker_test

import (
	"os/exec"
	"path/filepath"
	"testing"
)

// TestExampleModules vets the examples that are separate Go modules: go test ./... in the
// root module does not reach them, so a stale go.mod or a vet error would otherwise ship.
// Needs module downloads (prometheus) on a cold cache; skipped under -short.
func TestExampleModules(t *testing.T) {
	if testing.Short() {
		t.Skip("runs the go command per example module")
	}
	mods, err := filepath.Glob("examples/*/go.mod")
	if err != nil {
		t.Fatal(err)
	}
	if len(mods) == 0 {
		t.Fatal("no example modules found")
	}
	for _, mod := range mods {
		dir := filepath.Dir(mod)
		t.Run(filepath.Base(dir), func(t *testing.T) {
			cmd := exec.Command("go", "vet", "./...")
			cmd.Dir = dir
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Errorf("go vet in %s: %v\n%s", dir, err, out)
			}
		})
	}
}
