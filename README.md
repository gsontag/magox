# magox

[![ci](https://github.com/gsontag/magox/actions/workflows/ci.yml/badge.svg)](https://github.com/gsontag/magox/actions/workflows/ci.yml)

A layer over [mage](https://magefile.org) for the projects in Go: the targets
call the toolchain in Go, not in shell, and each step shows as one line, its
state and its duration. The output of a step shows only if it fails.

```go
//go:build mage

package main

import "github.com/gsontag/magox/gox"

// Test runs the tests under the race detector.
func Test() error {
	return gox.Test{Race: true, Cover: "coverage.out", ExcludeGenerated: true}.Run()
}

// Lint runs golangci-lint.
func Lint() error { return gox.Lint{}.Run() }
```

```
✓ lint      golangci-lint run  0 issues  8.2s
✗ test      ./...  66 packages · 915 passed · 2 failed · coverage 81.3%  30.4s
  │ --- FAIL  TestTemplate (0.00s)  github.com/gsontag/bolt/locale
  │     po_test.go:47: po/bolt.pot is not that of the texts
```

- `gox`: `Build` (several platforms at once), `Test` (through `go test -json`),
  `Coverage`, `CoverHTML`, `Lint`, `Fmt`, `FmtCheck`, `Vet`, `Tidy`,
  `TidyCheck`, `Generate`, `Fix` and `FixCheck` (the modernizers of go fix,
  cached: fast), `Hints` (the hints of gopls that go fix has not, as the type
  arguments it can infer: slow, gopls loads the whole module).
- `targets`: the targets every project needs, to import with `//mage:import`
  (build, cross, test, coverage, coverageHTML, lint, lintFix, fmt, fix, hints,
  vet, tidy, check, all), set in the `init` of the magefile through `targets.Config`.
  A target of the magefile may not have the name of one of them, unless the
  import has an alias: `//mage:import go` gives `go:test`, `go:lint`...
- `magox.Run` makes a step of any function; `magox.Sh` of any command.
- The tools come from the `tool` directives of go.mod, else from the PATH.

The rendering follows the environment: live in a terminal, plain lines in a
pipe or a CI, annotations on GitHub Actions, everything with `mage -v`.
`MAGOX_OUTPUT=live|plain|verbose|github` forces one.

`examples/bolt` is the Makefile of bolt, rewritten:

```sh
go tool mage -d examples/bolt -w ../bolt -l
```

magox builds itself, from `magefiles/`:

```sh
go tool mage        # all: checks, tests, coverage
go tool mage -l     # the targets
```
