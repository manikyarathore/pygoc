# PyGoC

**A statically-typed Python-subset language, compiled to idiomatic Go — built entirely from scratch.**

PyGoC is a hand-written, educational compiler with no parser generators, no compiler-compiler tooling, and no external codegen frameworks. Every stage — lexing, parsing, type inference, semantic analysis, IR construction, optimization, and Go code generation — is implemented directly, in readable, modular Go.

**The core design goal:** given a PyGo program, you should be able to see *exactly* how it changes at every compiler phase — source → tokens → AST → typed AST → IR/CFG → optimized IR → generated Go → executable — with each stage independently inspectable from the command line. Visibility and correctness matter more here than supporting all of Python.

It is not trying to be GCC or LLVM. It is trying to be a **small, correct, real compiler** for a deliberately scoped language — built to the standard of craftsmanship of a serious systems project, not a toy, and not a Python interpreter.

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

**Included:** variables with type inference, `int/float/bool/string`, `list<T>`, `dict<K,V>`, `tuple<T...>`, `if/elif/else`, `while`, `for ... in range(...)`, functions with recursion and multiple return values, restricted (read-only) closures, a mapped standard library (`fmt`, `strconv`, `strings`, `math`, `sort`, `bufio`), and a small, explicitly documented subset of **NumPy** (see [Library Support](#library-support) below).

**Deliberately excluded** (documented, not accidental — see [Scope Decisions](#scope-decisions) below): classes, exceptions, generators, decorators, full dynamic typing, `*args`/`**kwargs`, general imports/modules, async.

---

## The Six Compiler Phases

Every PyGo program passes through exactly six phases, and **every phase produces visible, inspectable output** — that visibility is the project's main teaching goal, not an afterthought.

```text
                 PyGo Source Code
                        │
                        ▼
              ┌──────────────────┐
              │ 1. Lexer         │   Source → Tokens
              └────────┬─────────┘   (INDENT/DEDENT/NEWLINE synthesized)
                        ▼
              ┌──────────────────┐
              │ 2. Parser        │   Tokens → AST
              └────────┬─────────┘   (panic-mode error recovery)
                        ▼
              ┌──────────────────┐
              │ 3. Semantic      │   AST → Typed, Validated AST
              │    Analysis      │   (symbol table + type inference + checks)
              └────────┬─────────┘
                        ▼
              ┌──────────────────┐
              │ 4. IR Generation │   AST → Three-Address Code + CFG
              └────────┬─────────┘
                        ▼
              ┌──────────────────┐
              │ 5. Optimization  │   IR → Optimized IR
              └────────┬─────────┘   (5 independently visible passes)
                        ▼
              ┌──────────────────┐
              │ 6. Code Gen      │   IR → Go source
              └────────┬─────────┘
                        ▼
                 Go Source Code
                        │
                        ▼
                    go build
                        │
                        ▼
                 Native Program
```

**Note on Phase 3:** internally, "Semantic Analysis" is implemented as three cooperating packages (`internal/symbol`, `internal/types`, `internal/semantic`) built and tested as separate sub-phases — 3a, 3b, 3c — because that's the right granularity for building and testing them correctly. But from the CLI's and the pipeline's point of view, they present as **one** phase: `pygoc sema` shows the finished symbol table and typed AST, not three separate outputs. The repo structure below reflects the implementation split; the phase numbering above reflects what you actually see.

---

## Architecture — Package Map

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
 ┌───────────┐  scoped symbol table (global → function; if/while/for share scope)
 │  symbol   │
 └────┬──────┘
      ▼                                    ── these three packages together
 ┌───────────┐  static type inference           implement Phase 3,
 │  types    │  + operator type rules            "Semantic Analysis"
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
 │  codegen  │  (includes the NumPy-subset → Go slice-loop mapping)
 └────┬──────┘
      ▼
   go build  →  native executable
```

Every arrow above is a real package boundary in the repo — each stage only depends on the stages before it, never after. You can open any single folder and understand that phase in isolation.

---

## Repository Structure

```text
pygoc/
├── cmd/pygoc/              # CLI entrypoint — dispatches to lex/ast/sema/ir/optimize/compile/run/pipeline
├── internal/
│   ├── token/              # shared token vocabulary (used by lexer AND parser)
│   ├── lexer/               # Phase 1 — source → tokens
│   ├── ast/                 # AST node definitions (data only, no logic)
│   ├── parser/               # Phase 2 — tokens → AST, with error recovery
│   ├── symbol/               # Phase 3a — scopes & symbol tables
│   ├── types/                # Phase 3b — type system & inference
│   ├── semantic/             # Phase 3c — validation, using symbol + types
│   ├── ir/                   # Phase 4 — typed AST → three-address code → CFG
│   ├── optimizer/            # Phase 5 — 5 independent, visible passes
│   ├── codegen/              # Phase 6 — CFG → Go source + stdlib mapping
│   ├── numpy/                 # NumPy-subset recognition & lowering (see below)
│   └── diagnostics/          # cross-cutting: shared error/reporting types
├── tests/
│   ├── golden/               # input.py + expected output pairs, per phase
│   └── e2e/                   # compile + execute + verify stdout
├── examples/                  # showcase PyGo programs, incl. NumPy-subset examples
├── benchmarks/                 # compiler performance benchmarks
├── docs/
│   ├── language-spec.md
│   ├── grammar.ebnf
│   ├── architecture.md
│   ├── numpy-subset.md         # exact list of supported NumPy operations
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

### CLI — inspect any phase independently

```bash
pygoc lex program.py        # print token stream
pygoc ast program.py        # print parsed AST (tree format)
pygoc sema program.py       # print symbol table + typed AST
pygoc ir program.py         # print unoptimized TAC + CFG
pygoc optimize program.py   # print IR after each of the 5 optimization passes
pygoc compile program.py    # emit Go source
pygoc run program.py        # compile + go run in one step
pygoc check program.py      # semantic check only, no codegen (like a linter)
```

### `pygoc pipeline` — the full six-phase run in one command

This is the main demonstration command: it runs a program through all six phases and prints every stage's output to the terminal, clearly labeled.

```bash
pygoc pipeline program.py
```

```text
════════════════════════════════════
        PyGoC Compiler Pipeline
════════════════════════════════════

[1] LEXICAL ANALYSIS
--------------------
Tokens:
...

[2] SYNTAX ANALYSIS
-------------------
AST:
...

[3] SEMANTIC ANALYSIS
---------------------
Symbol Table:
...
Typed AST:
...

[4] INTERMEDIATE CODE
---------------------
TAC:
...
CFG:
...

[5] CODE OPTIMIZATION
---------------------
Before:
...
Constant Folding:        ...
Constant Propagation:    ...
Algebraic Simplification:...
CSE:                     ...
Dead Code Elimination:   ...
Final IR:
...

[6] CODE GENERATION
-------------------
Generated Go:
...

════════════════════════════════════
          Compilation Complete
════════════════════════════════════
```

No files are required for this — terminal output is the deliverable. `pygoc compile`/`pygoc run` are the only commands that touch disk (to produce/run the `.go` file).

---

## Current Status

| Phase | Package(s) | Status |
|---|---|---|
| 1. Lexer | `internal/token`, `internal/lexer` | ✅ Done — 16/16 tests passing |
| 2. Parser | `internal/ast`, `internal/parser` | ✅ Done — 16/16 tests passing |
| 3. Semantic Analysis | `internal/symbol` (3a), `internal/types` (3b), `internal/semantic` (3c) | ⏳ 3a in progress |
| 4. IR (TAC + CFG) | `internal/ir` | ☐ Not started |
| 5. Optimizer | `internal/optimizer` | ☐ Not started |
| 6. Code Generator | `internal/codegen` | ☐ Not started |
| NumPy subset | `internal/numpy` | ☐ Not started (after core Phase 6) |
| CLI | `cmd/pygoc` | ☐ Not started (built incrementally per phase) |

See [`docs/ROADMAP.md`](docs/ROADMAP.md) for the full phase-by-phase plan and design rationale.

---

## Library Support

The compiler also demonstrates handling a real external library, without pretending to implement all of it. **NumPy** was chosen over Pandas: NumPy's core operations (fixed-shape numeric arrays, elementwise arithmetic) map directly onto Go slices and loops with no extra runtime machinery; Pandas would require a labeled, heterogeneous `DataFrame` type — a much larger scope, not realistic for this project's timeline.

**Explicitly supported subset:**
```python
import numpy as np

a = np.array([1, 2, 3])
b = np.array([4, 5, 6])
c = a + b        # elementwise add
d = a * b        # elementwise multiply
print(c)
```

`import numpy as np` is recognized as a **single special-cased exception** to the "no imports" rule — the compiler does not implement a general import system; it recognizes exactly this one line as declaring "the NumPy subset is in use" and nothing else. `np.array(...)` lowers to a Go slice; elementwise `+`/`-`/`*` lower to a small generated loop. The full, exact list of supported functions/operators lives in `docs/numpy-subset.md` and is kept authoritative there — this README will not attempt to duplicate it in full as the subset grows.

**Not supported, on purpose:** broadcasting between differently-shaped arrays, multi-dimensional arrays, linear algebra (`np.dot`, `np.linalg.*`), random number generation, any NumPy dtype other than PyGo's own `int`/`float`.

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
| General imports/modules | Single-file compilation keeps the symbol table design simple. `import numpy as np` is one narrowly special-cased exception, not a general import system |

---

## Testing Philosophy

Every phase is tested in isolation, and the whole pipeline is tested end-to-end — **looping is used deliberately at two levels**, not just single hand-picked examples:

- **Table-driven unit tests per package** — each test file loops over a table of `{input, expected}` cases (this is already how the lexer's and parser's 32 tests are written), so adding a new case is one line, not a new function.
- **A looping end-to-end harness** (`tests/e2e`) — iterates over every `.py` file in `tests/e2e/`, runs it through all six phases, `go build`s the generated Go, executes it, and diffs actual vs. expected stdout for each one in turn. One test function, many programs — this is what actually proves the whole pipeline works, not just each phase in isolation.
- **Golden tests** (`tests/golden`) — `input.py` paired with an `expected.go` (or an expected IR/AST dump), so regressions show up as an exact diff.
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
