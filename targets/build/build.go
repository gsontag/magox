// Package build gives the targets that build, to import in a magefile under
// a namespace:
//
//	import (
//		//mage:import build
//		"github.com/gsontag/magox/targets/build"
//	)
//
// They are then build:run and build:cross; the magefile adds its own to the
// namespace with a type Build mg.Namespace.
//
// Mage makes a target of each exported function here: the package has no
// other.
package build

import "github.com/gsontag/magox/gox"

// Settings tells the targets what the project is. The magefile changes
// Config in its init: mage runs the targets after.
type Settings struct {
	Pkgs []string // the packages built; ./... if empty
	// Builds are the binaries that Run writes; empty, Run compiles Pkgs
	Builds []gox.Build
	// Platforms are the os/arch that Cross builds CrossPkgs for
	Platforms []string
	CrossPkgs []string // Pkgs if empty
}

// Config is what the targets run with.
var Config Settings

// Run compiles the packages, or writes the binaries of Config.Builds.
func Run() error {
	if len(Config.Builds) == 0 {
		return gox.Build{Pkgs: pkgs()}.Run()
	}
	for _, b := range Config.Builds {
		if err := b.Run(); err != nil {
			return err
		}
	}
	return nil
}

// Cross builds the packages for the platforms of Config.Platforms: a
// dependency on one OS slipped into them breaks there.
func Cross() error {
	if len(Config.Platforms) == 0 {
		return nil
	}
	p := Config.CrossPkgs
	if len(p) == 0 {
		p = pkgs()
	}
	return gox.Build{Pkgs: p, Platforms: Config.Platforms}.Run()
}

func pkgs() []string {
	if len(Config.Pkgs) == 0 {
		return []string{"./..."}
	}
	return Config.Pkgs
}
