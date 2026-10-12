//go:build mage

// The Makefile of bolt, in magox: the common targets come from
// magox/targets, each in its namespace; the magefile adds what is of bolt.
package main

import (
	"time"

	"github.com/magefile/mage/mg"

	"github.com/gsontag/magox"
	"github.com/gsontag/magox/gox"

	//mage:import
	"github.com/gsontag/magox/targets"
	//mage:import build
	"github.com/gsontag/magox/targets/build"
	//mage:import quality
	"github.com/gsontag/magox/targets/quality"
	//mage:import test
	"github.com/gsontag/magox/targets/test"
)

var Default = targets.All

func init() {
	test.Config.Timeout = 2 * time.Minute
	build.Config.Platforms = []string{"windows/amd64", "js/wasm"}
	// The packages that must build for every target of Go, not only Linux
	// (specification, section 4.10)
	build.Config.CrossPkgs = []string{
		"./reactive/...", "./internal/...", "./gfx/...", "./style/...", "./text/...",
		"./platform", "./platform/keysym", "./platform/loop/...", "./platform/headless/...",
		"./core/...", "./widget/...", "./showcase/...", "./cmd/showcase", ".",
	}
	quality.Config.HintsIgnore = []string{"[darwin]"}
	quality.Config.Checks = []any{Quality.Chars}
}

// Quality takes the checks of bolt into the namespace of those of magox.
type Quality mg.Namespace

// Chars fails if a Go file has an invisible or bidi character.
func (Quality) Chars() error {
	return magox.Run("chars", "invisible and bidi characters", func(s *magox.Step) error {
		return s.Command("sh", "-c", `! LC_ALL=C.UTF-8 grep -rnP `+
			`'[\x{00A0}\x{00AD}\x{2000}-\x{200F}\x{2028}-\x{202F}\x{205F}-\x{206F}\x{FEFF}]' --include='*.go' .`).Run()
	})
}

// Generate runs the generators of the Wayland protocol and of the xkb table.
func Generate() error {
	return gox.Generate("./internal/wayland/protocol", "./internal/xkb")
}
