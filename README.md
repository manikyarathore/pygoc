# PyGoC

**A statically-typed Python-subset language, compiled to idiomatic Go — built entirely from scratch.**

PyGoC is a hand-written compiler with no parser generators, no compiler-compiler tooling, and no external codegen frameworks. Every stage — lexing, parsing, type inference, semantic analysis, IR construction, optimization, and Go code generation — is implemented directly, in readable, modular Go.

It is not trying to be GCC or LLVM. It is trying to be a **small, correct, real compiler** for a deliberately scoped language — built to the standard of craftsmanship of a serious systems project, not a toy.

---

## What is "PyGo"?

PyGo looks like Python but is a **strict, statically-typed subset** of it. If you know Python, you already know PyGo's syntax:

```python
def add(a: int, b: int) -> int:
    return a + b

def factorial(n: int) -> int:
    if n <= 1:
        return 1
    return n * factorial(n - 1)

x = add(3, 4)
print(x)
```

Every PyGo program is type-checked at compile time, then compiled down to a standalone Go program you can `go build` and run natively — no interpreter, no runtime dependency on PyGoC itself.

**Included:** variables with type inference, `int/float/bool/string`, `list<T>`, `dict<K,V>`, `tuple<T...>`, `if/elif/else`, `while`, `for ... in range(...)`, functions with recursion and multiple return values, restricted (read-only) closures, and a mapped standard library (`fmt`, `strconv`, `strings`, `math`, `sort`, `bufio`).

**Deliberately excluded** (documented, not accidental — see [Scope Decisions](#scope-decisions) below): classes, exceptions, generators, decorators, full dynamic typing, `*args`/`**kwargs`, imports/modules, async.

---

## Why This Project Exists

Most "toy compiler" projects either stay so small they teach nothing, or try to be too ambitious and never finish. PyGoC is scoped deliberately at the hardest *tractable* point: a real type system, a real CFG-based IR, real multi-pass optimization, and real error recovery — without wandering into the parts of language design (classes, exceptions, dynamic typing) that turn a semester project into a multi-year one.

---

## Architecture — The Full Pipeline

```text
 source (.py)
      │
      ▼
 ┌─────────┐   token stream (with synthesized INDENT/DEDENT/NEWLINE)
 │  lexer  │
 └────┬────┘
      ▼
 ┌─────────┐   Abstract Syntax Tree, built with panic-mode error recovery
 │ parser  │
 └────┬────┘
      ▼
 ┌───────────┐  scoped symbol table (global → function → block)
 │  symbol   │
 └────┬──────┘
      ▼
 ┌───────────┐  static type inference + operator type rules
 │  types    │
 └────┬──────┘
      ▼
 ┌───────────┐  fully validated, typed AST + diagnostics
 │ semantic  │
 └────┬──────┘
      ▼
 ┌───────────┐  three-address code lowered into a control-flow graph
 │    ir     │
 └────┬──────┘
      ▼
 ┌───────────┐  constant folding → propagation → algebraic simplification
 │ optimizer │  → common subexpression elimination → dead code elimination
 └────┬──────┘
      ▼
 ┌───────────┐  idiomatic, gofmt-clean Go source + auto-resolved imports
 │  codegen  │
 └────┬──────┘
      ▼
   go build  →  native executable
```

Every arrow above is a real package boundary in the repo — each stage only depends on the stages before it, never after. You can open any single folder and understand that phase in isolation.

---

## Repository Structure

```text
pygoc/
├── cmd/pygoc/            # CLI entrypoint only — no compiler logic lives here
├── internal/
│   ├── token/            # shared token vocabulary (used by lexer AND parser)
│   ├── lexer/             # Phase 1 — source → tokens
│   ├── ast/               # AST node definitions (data only, no logic)
│   ├── parser/             # Phase 2 — tokens → AST, with error recovery
│   ├── symbol/             # Phase 3a — scopes & symbol tables
│   ├── types/              # Phase 3b — type system & inference
│   ├── semantic/           # Phase 3c — validation, using symbol + types
│   ├── ir/                 # Phase 4 — typed AST → three-address code → CFG
│   ├── optimizer/          # Phase 5 — multi-pass CFG optimization
│   ├── codegen/            # Phase 6 — CFG → Go source + stdlib mapping
│   └── diagnostics/        # cross-cutting: shared error/reporting types
├── tests/
│   ├── golden/             # input.py + expected output pairs
│   └── e2e/                 # compile + execute + verify stdout
├── examples/                # showcase PyGo programs
├── benchmarks/               # compiler performance benchmarks
├── docs/
│   ├── language-spec.md
│   ├── grammar.ebnf
│   ├── architecture.md
│   └── viva-question-bank.md
├── go.mod
└── README.md
```

> **Design note on `internal/token/`:** token types live in their own package, separate from `lexer`, specifically so the parser can import token types without creating a circular dependency between `lexer` and `parser`.

---

## Build & Run

Requires Go 1.22+.

```bash
git clone <your-repo-url> pygoc
cd pygoc
go build ./...          # build everything
go test ./... -v        # run the full test suite
```

Planned CLI (built out phase-by-phase, see roadmap):

```bash
pygoc lex file.py        # print token stream
pygoc ast file.py        # print parsed AST
pygoc sema file.py       # print symbol table + inferred types
pygoc ir file.py         # print unoptimized IR/CFG
pygoc optimize file.py   # print optimized IR/CFG (before/after)
pygoc compile file.py    # emit Go source
pygoc run file.py        # compile + go run in one step
pygoc check file.py      # semantic check only (like a linter)
```

Every debug command is a partial pipeline run stopped early and printed — reflecting the same "each phase is independently inspectable" principle the architecture is built on.

---

## Current Status

| Phase | Package | Status |
|---|---|---|
| 1. Lexer | `internal/token`, `internal/lexer` | ✅ Implemented — indentation handling, all literals/operators, error recovery, 16-case test suite |
| 2. Parser | `internal/ast`, `internal/parser` | ⏳ Next up |
| 3a. Symbol Table | `internal/symbol` | ☐ Not started |
| 3b. Type System | `internal/types` | ☐ Not started |
| 3c. Semantic Analysis | `internal/semantic` | ☐ Not started |
| 4. IR (TAC + CFG) | `internal/ir` | ☐ Not started |
| 5. Optimizer | `internal/optimizer` | ☐ Not started |
| 6. Code Generator | `internal/codegen` | ☐ Not started |
| CLI | `cmd/pygoc` | ☐ Not started |

See [`docs/ROADMAP.md`](docs/ROADMAP.md) for the full phase-by-phase plan, timeline, and design rationale behind every decision.

---

## Scope Decisions

Every exclusion below was a deliberate tradeoff, not a limitation discovered late:

| Excluded | Why |
|---|---|
| Classes / OOP | Would require a whole method-dispatch and inheritance model — out of scope for a semester |
| Exceptions (`try/except`) | Needs unwind semantics threaded through the entire IR and codegen layer |
| Generators / `yield` | Requires coroutine-style state machines in codegen — a project on its own |
| Full dynamic typing | Defeats the point of static type inference, PyGoC's core teaching value |
| Closures with mutation | Restricted to **read-only capture** — keeps scoping tractable without losing the "functions as values" flavor |
| Imports / modules | Single-file compilation keeps the symbol table design simple |

---

## Testing Philosophy

Every phase is tested in isolation, and the whole pipeline is tested end-to-end:

- **Unit tests** per package (table-driven, one file per source file).
- **Golden tests** — `input.py` paired with an `expected.go` (or expected IR dump), so regressions show up as an exact diff.
- **End-to-end tests** — compile a PyGo program, actually run the resulting Go binary, and check its output.
- **Benchmarks** — track compiler performance itself, not just correctness.

Run everything with:

```bash
go test ./... -v
go test ./... -bench=.
```

---

## Contributing / Working On This Yourself

This project is built phase-by-phase, in order, deliberately — later phases depend on earlier ones being correct, so skipping ahead tends to produce bugs that are hard to trace back to their real cause. If you're extending it:

1. Read the relevant `docs/*.md` before touching code.
2. Write the test cases for a feature before (or alongside) the implementation.
3. Keep each package's dependency direction one-way (never import a "later" phase from an "earlier" one).

---

## License

Add your chosen license here (MIT is a common, permissive choice for a portfolio/teaching compiler).
