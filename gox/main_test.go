package gox

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/pterm/pterm"

	"github.com/gsontag/magox/internal/report"
)

var out syncBuffer

func TestMain(m *testing.M) {
	// Read once, by the first step
	if err := os.Setenv("MAGOX_OUTPUT", "plain"); err != nil {
		panic(err)
	}
	pterm.DisableColor()
	report.Output = &out
	os.Exit(m.Run())
}

type syncBuffer struct {
	mu sync.Mutex
	b  strings.Builder
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.b.Write(p)
}

// take returns what the steps wrote since the last call.
func (b *syncBuffer) take() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	s := b.b.String()
	b.b.Reset()
	return s
}

// module writes the files of a module in a directory of its own, and runs
// the test there.
func module(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Chdir(dir)
	t.Setenv("GOWORK", "off")
	t.Setenv("GOFLAGS", "")
	out.take()
	return dir
}
