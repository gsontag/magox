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
  arguments it can infer: slow, gopls loads the whole module),
  `FieldAlignment` (the structs that would take less memory in another order,
  in a table, the most wasteful first; only those that spare memory indeed,
  with the size classes of the allocator; `Fix` rearranges those without
  comments, that fieldalignment would drop).
- `targets`: the targets every project needs, a package by domain, each
  imported under its namespace with `//mage:import <namespace>`:
  `targets/build` (build:run, build:cross), `targets/quality`
  (quality:check, quality:lint, quality:lintFix, quality:fmt, quality:fix,
  quality:hints, quality:align, quality:alignFix, quality:vet, quality:tidy), `targets/test` (test:run,
  test:coverage, test:coverageHTML); `targets` itself gives all. Each
  package has its `Config`, set in the `init` of the magefile. The magefile
  adds its own targets to a namespace, and its checks to quality:check:

  ```go
  type Quality mg.Namespace

  // Chars fails on the invisible characters: quality:chars.
  func (Quality) Chars() error { ... }

  func init() { quality.Config.Checks = []any{Quality.Chars} }
  ```
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
go tool mage        # all: quality:check, test:coverage, build:cross
go tool mage -l     # the targets
```
