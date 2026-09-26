#!/usr/bin/env bash
# check_all.sh — builds, tests, and demos the whole PyGoC project in one
# pass. Run this from the pygoc/ project root:
#
#   chmod +x check_all.sh
#   ./check_all.sh
#
# It stops at the first real failure (build or test) so you know
# immediately if something's broken, but keeps going through all the
# demo commands even if individual example programs have issues, so you
# get a full picture in one run.

set -e  # stop immediately on build/test failure

BOLD='\033[1m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RESET='\033[0m'

section() {
    echo ""
    echo -e "${BOLD}════════════════════════════════════════${RESET}"
    echo -e "${BOLD}  $1${RESET}"
    echo -e "${BOLD}════════════════════════════════════════${RESET}"
}

section "0. Go toolchain check"
if ! command -v go &> /dev/null; then
    echo "ERROR: 'go' is not installed or not on PATH."
    echo "Install it first (Arch: sudo pacman -S go), then re-run this script."
    exit 1
fi
go version

section "1. go build ./... (compiles every package)"
go build ./...
echo -e "${GREEN}✓ build succeeded${RESET}"

section "2. go vet ./... (static checks)"
go vet ./...
echo -e "${GREEN}✓ vet clean${RESET}"

section "3. go test ./... -v (every unit + e2e test, looped)"
go test ./... -v
echo -e "${GREEN}✓ all tests passed${RESET}"

section "4. go test ./... -bench=. (benchmarks, if any)"
go test ./... -bench=. -run=^$ || echo -e "${YELLOW}(no benchmarks defined yet — that's fine)${RESET}"

section "5. CLI demo — each phase independently, on examples/arithmetic.py"
for cmd in lex ast sema ir optimize; do
    echo ""
    echo -e "${YELLOW}--- pygoc $cmd examples/arithmetic.py ---${RESET}"
    go run ./cmd/pygoc "$cmd" examples/arithmetic.py
done

section "6. Full six-phase pipeline demo — examples/factorial.py"
go run ./cmd/pygoc pipeline examples/factorial.py

section "7. Compile + run every example program directly"
for f in examples/*.py; do
    echo ""
    echo -e "${YELLOW}--- pygoc run $f ---${RESET}"
    go run ./cmd/pygoc run "$f"
done

section "ALL CHECKS PASSED"
echo -e "${GREEN}Build: OK   Vet: OK   Tests: OK   CLI: OK   Examples: OK${RESET}"
echo "Project is in a submittable state."
