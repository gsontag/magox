package tool

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
)

// module makes a module in a directory of its own, and runs the test there.
func module(t *testing.T, gomod string) {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(gomod), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	declared = sync.OnceValues(goModTools)
	t.Cleanup(func() { declared = sync.OnceValues(goModTools) })
}

// pathWith leaves on the PATH only go, and an executable name if not empty.
func pathWith(t *testing.T, name string) string {
	t.Helper()
	gobin, err := exec.LookPath("go")
	if err != nil {
		t.Skip("go is not on the PATH")
	}
	dir := t.TempDir()
	exe := filepath.Join(dir, name)
	if name != "" {
		if err := os.WriteFile(exe, []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+filepath.Dir(gobin))
	return exe
}

func TestDeclaredToolRunsThroughGoTool(t *testing.T) {
	module(t, "module example.com/m\n\ngo 1.24\n\ntool example.com/linters/v2/cmd/golint\n")
	pathWith(t, "golint") // go.mod wins over the PATH
	got, err := Command("golint", "example.com/linters/v2/cmd/golint")
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"go", "tool", "golint"}; !slices.Equal(got, want) {
		t.Errorf("Command = %q, want %q", got, want)
	}
}

func TestToolOfThePath(t *testing.T) {
	module(t, "module example.com/m\n\ngo 1.24\n")
	exe := pathWith(t, "golint")
	got, err := Command("golint", "example.com/cmd/golint")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got, []string{exe}) {
		t.Errorf("Command = %q, want %q", got, exe)
	}
}

func TestToolMissing(t *testing.T) {
	module(t, "module example.com/m\n\ngo 1.24\n")
	pathWith(t, "")
	_, err := Command("golint", "example.com/cmd/golint")
	if err == nil || !strings.Contains(err.Error(), "go get -tool example.com/cmd/golint@latest") {
		t.Errorf("err = %v, want how to declare the tool", err)
	}
}
