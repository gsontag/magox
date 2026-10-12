// Package tool finds the external tools: those that go.mod declares first,
// at the version it pins, else those of the PATH.
package tool

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"path"
	"sync"
)

// declared caches the tools of go.mod; the tests read it again.
var declared = sync.OnceValues(goModTools)

func goModTools() (map[string]bool, error) {
	out, err := exec.Command("go", "mod", "edit", "-json").Output()
	if err != nil {
		return nil, fmt.Errorf("reading go.mod: %w", err)
	}
	var mod struct{ Tool []struct{ Path string } }
	if err := json.Unmarshal(out, &mod); err != nil {
		return nil, fmt.Errorf("reading go.mod: %w", err)
	}
	tools := map[string]bool{}
	for _, t := range mod.Tool {
		tools[path.Base(t.Path)] = true
	}
	return tools, nil
}

// Command returns the command that runs the tool name, of the package pkg:
// through `go tool` if go.mod declares it, else from the PATH.
func Command(name, pkg string) ([]string, error) {
	if tools, err := declared(); err == nil && tools[name] {
		return []string{"go", "tool", name}, nil
	}
	if p, err := exec.LookPath(name); err == nil {
		return []string{p}, nil
	}
	return nil, fmt.Errorf("%s not found: declare it in go.mod with `go get -tool %s@latest`, or install it", name, pkg)
}
