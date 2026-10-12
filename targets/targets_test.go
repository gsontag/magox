package targets

import (
	"slices"
	"testing"
)

func TestDefaults(t *testing.T) {
	if !Config.Race || !Config.ExcludeGenerated || Config.Profile != "coverage.out" {
		t.Errorf("Config = %+v", Config)
	}
}

func TestPkgs(t *testing.T) {
	old := Config
	t.Cleanup(func() { Config = old })

	if got := pkgs(); !slices.Equal(got, []string{"./..."}) {
		t.Errorf("pkgs() = %q, want ./... by default", got)
	}
	Config.Pkgs = []string{"./cmd/...", "./lib"}
	if got := pkgs(); !slices.Equal(got, Config.Pkgs) {
		t.Errorf("pkgs() = %q, want Config.Pkgs", got)
	}
}

func TestCrossWithoutPlatforms(t *testing.T) {
	old := Config
	t.Cleanup(func() { Config = old })

	Config.Platforms = nil
	if err := Cross(); err != nil {
		t.Errorf("Cross() = %v, want nothing to do", err)
	}
}
