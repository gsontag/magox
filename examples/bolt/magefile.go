//go:build mage

// The Makefile of bolt, in magox: the common targets come from
// magox/targets, the magefile keeps what is of bolt.
package main

import (
	"time"

	"github.com/gsontag/magox/gox"
	//mage:import
	"github.com/gsontag/magox/targets"
)

var Default = targets.Build

func init() {
	targets.Config.Timeout = 2 * time.Minute
	targets.Config.Platforms = []string{"windows/amd64", "js/wasm"}
	targets.Config.HintsIgnore = []string{"[darwin]"}
	// The packages that must build for every target of Go, not only Linux
	// (specification, section 4.10)
	targets.Config.CrossPkgs = []string{
		"./reactive/...", "./internal/...", "./gfx/...", "./style/...", "./text/...",
		"./platform", "./platform/keysym", "./platform/loop/...", "./platform/headless/...",
		"./core/...", "./widget/...", "./showcase/...", "./cmd/showcase", ".",
	}
}

// Generate runs the generators of the Wayland protocol and of the xkb table.
func Generate() error {
	return gox.Generate("./internal/wayland/protocol", "./internal/xkb")
}
