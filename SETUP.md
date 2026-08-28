# Setup & Dependencies

PyGoC is built entirely on Go's standard library — **zero third-party
dependencies** is a deliberate design choice (keeps the project trivially
buildable anywhere Go is installed, no version-resolution surprises).

This file is the single place to check "do I have what I need" and "did
we add any new dependency" — Go's equivalent of a `requirements.txt`,
except most of it is auto-maintained by tooling instead of hand-edited.

## Prerequisites

| Tool | Version | Install (Arch Linux) | Why |
|---|---|---|---|
| Go | 1.22+ | `sudo pacman -S go` | compiler toolchain for building/running/testing PyGoC itself |

Verify:
```bash
go version
```

## Dependency tracking (auto-maintained — do not hand-edit)

- `go.mod` — module path, Go version, and (if any are ever added) every
  external package PyGoC depends on.
- `go.sum` — exact version hashes, auto-generated the first time it's
  needed. Not present yet, because there are zero external dependencies.

If a future phase genuinely needs an external package, the process is:
```bash
go get github.com/some/package
```
This updates `go.mod`/`go.sum` automatically — nothing to track by hand.
As of Phase 1, **no external packages have been added**, and none are
currently planned for any phase (see docs/ROADMAP.md — the whole
pipeline is designed around the standard library only: `fmt`, `strconv`,
`strings`, `math`, `sort`, `bufio`, `errors`).

## Build & test

```bash
go build ./...       # compile everything
go test ./... -v      # run all tests, verbose
```
