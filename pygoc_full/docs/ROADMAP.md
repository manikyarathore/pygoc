# PyGoC — Full Roadmap & Technical Framework

> This is the authoritative, up-to-date plan for the project — the single document to re-read before starting any new phase. It reflects the actual repo structure in use, current progress, and the finalized 6-phase-visible CLI specification.

---

## 0. Project Identity

**Name:** PyGoC
**What it is:** a hand-written, educational compiler for "PyGo" — a statically-typed Python-subset language — targeting Go as the output, with every phase visible and independently inspectable.
**Standard we're holding it to:** not GCC's *scope*, but GCC's *standard of correctness and craftsmanship*, at PyGo's scope.
**Core teaching goal:** given a PyGo program, see exactly how it changes at every phase — source → tokens → AST → typed AST → IR/CFG → optimized IR → generated Go → executable.
**Timeline:** 7 weeks (see §9).

---

## 1. The Six Phases (User-Facing) vs. Package Structure (Implementation)

The CLI and the `pipeline` command present the compiler as **exactly six phases**. Internally, Phase 3 ("Semantic Analysis") is implemented as three separate, independently-tested packages — this is a deliberate split for buildability, not a difference in what the user sees.

```text
[1] LEXICAL ANALYSIS     internal/lexer (+ internal/token)
[2] SYNTAX ANALYSIS      internal/parser (+ internal/ast)
[3] SEMANTIC ANALYSIS    internal/symbol + internal/types + internal/semantic
[4] INTERMEDIATE CODE    internal/ir
[5] CODE OPTIMIZATION    internal/optimizer
[6] CODE GENERATION      internal/codegen (+ internal/numpy for the library subset)
```

Every phase must produce output that can be printed to the terminal in isolation via its own CLI subcommand, AND as one labeled section of `pygoc pipeline`.

---

## 2. Full Pipeline (Reference)

```text
source (.py)
   ↓  [internal/lexer, using internal/token]
Token Stream (INDENT/DEDENT/NEWLINE synthesized)
   ↓  [internal/parser, building internal/ast]
AST (built with panic-mode error recovery)
   ↓  [internal/symbol]
Scoped Symbol Table
   ↓  [internal/types]
Type Inference
   ↓  [internal/semantic]
Typed, Validated AST + Diagnostics
   ↓  [internal/ir]
Three-Address Code → Control Flow Graph
   ↓  [internal/optimizer]
Optimized CFG (5 independently visible passes)
   ↓  [internal/codegen, using internal/numpy for the library subset]
Go Source (+ auto-resolved imports)
   ↓
go build → Executable
```

---

## 3. Repository Structure (Authoritative)

```text
pygoc/
├── cmd/
│   └── pygoc/                  # main.go — CLI dispatch only, no compiler logic
│       # subcommands: lex, ast, sema, ir, optimize, compile, run, check, pipeline
├── internal/
│   ├── token/                   # shared token vocabulary
│   │   └── token.go
│   ├── lexer/                   # Phase 1 ✅ — source → tokens
│   │   ├── lexer.go
│   │   ├── indent.go             # INDENT/DEDENT/NEWLINE logic, isolated
│   │   └── lexer_test.go
│   ├── ast/                     # Phase 2 data structures (types only, no logic)
│   │   ├── expr.go
│   │   ├── stmt.go
│   │   └── printer.go            # AST pretty-printer for `pygoc ast`
│   ├── parser/                  # Phase 2 ✅ — tokens → AST
│   │   ├── parser.go
│   │   ├── expr_parser.go         # precedence climbing, isolated
│   │   ├── recovery.go            # panic-mode error recovery
│   │   └── parser_test.go
│   ├── symbol/                  # Phase 3a — scopes & symbol tables
│   │   ├── table.go
│   │   ├── scope.go
│   │   └── symbol_test.go
│   ├── types/                   # Phase 3b — type system
│   │   ├── types.go               # Type interface + primitives
│   │   ├── inference.go           # type inference engine
│   │   ├── rules.go               # operator type rules (int+int→int, etc.)
│   │   └── types_test.go
│   ├── semantic/                # Phase 3c — semantic analysis
│   │   ├── analyzer.go
│   │   ├── checks.go              # undefined vars, scope errors, return checks
│   │   ├── printer.go             # prints symbol table + typed AST for `pygoc sema`
│   │   └── semantic_test.go
│   ├── ir/                      # Phase 4 — typed AST → IR
│   │   ├── tac.go                 # three-address code instruction set
│   │   ├── cfg.go                 # basic blocks + control flow graph
│   │   ├── builder.go             # AST → IR lowering
│   │   ├── printer.go             # prints TAC + CFG for `pygoc ir`
│   │   └── ir_test.go
│   ├── optimizer/               # Phase 5 — IR → optimized IR
│   │   ├── constfold.go
│   │   ├── constprop.go
│   │   ├── deadcode.go
│   │   ├── cse.go                 # common subexpression elimination
│   │   ├── algebraic.go
│   │   ├── pipeline.go            # pass ordering/orchestration, before/after per pass
│   │   └── optimizer_test.go
│   ├── codegen/                 # Phase 6 — IR → Go source
│   │   ├── generator.go
│   │   ├── stdlib_map.go          # PyGo builtins → Go stdlib calls
│   │   ├── imports.go             # auto-import resolution
│   │   └── codegen_test.go
│   ├── numpy/                    # NumPy-subset recognition & lowering
│   │   ├── recognize.go            # detects `import numpy as np` + np.* calls
│   │   ├── lower.go                 # np.array/+/-/* → Go slice + loop codegen
│   │   └── numpy_test.go
│   └── diagnostics/              # cross-cutting: used by every phase
│       ├── error.go
│       ├── reporter.go            # pretty terminal error output w/ source spans
│       └── diagnostics_test.go
├── tests/
│   ├── golden/                   # input.py + expected.go / expected IR-dump pairs
│   └── e2e/                       # looping harness: every .py here is compiled,
│                                   # run, and diffed against a matching .expected.txt
├── examples/                      # showcase PyGo programs, incl. NumPy-subset ones
├── benchmarks/
├── docs/
│   ├── language-spec.md
│   ├── grammar.ebnf
│   ├── architecture.md
│   ├── numpy-subset.md            # exact, authoritative list of supported np.* ops
│   └── viva-question-bank.md
├── go.mod
└── README.md
```

**Dependency rule enforced throughout:** each package only imports packages *earlier* in the pipeline. `parser` may import `lexer`/`token`/`ast`; `lexer` must never import `parser`. `numpy` is imported only by `codegen` (it's a codegen-time lowering concern, not a separate pipeline stage). This keeps the dependency graph a straight line, mirroring the pipeline itself.

---

## 4. Language Specification (Full Feature Set)

### 4.1 Types
```text
int, float, bool, string, void
list<T>
dict<K,V>
tuple<T1,T2,...>
function types: (T1,T2)->T3
```

### 4.2 Literals
```python
10          # int
3.14        # float
True False  # bool
"hello"     # string
[1,2,3]     # list<int>
{"a":1}     # dict<string,int>
(1,"x")     # tuple<int,string>
```

### 4.3 Operators
```text
Arithmetic:   + - * / % //
Comparison:   == != < > <= >=
Boolean:      and or not
Unary:        - not
Assignment:   = += -= *= /=
```

### 4.4 Control Flow
```python
if cond: ... elif cond: ... else: ...
while cond: ...
for x in range(a, b): ...
break
continue
```

### 4.5 Functions
```python
def add(a: int, b: int) -> int:
    return a + b

def divmod2(a: int, b: int) -> (int, int):
    return a // b, a % b
```
Recursion supported. Nested function definitions supported. Closures are **read-only capture only** — a nested function may read an outer variable, never reassign it.

### 4.6 Collections
```python
nums = [1,2,3]
nums.append(4)
len(nums)
nums[0]
nums[1:3]

d = {"a": 1}
d["b"] = 2

t = (1, "x")
a, b = t
```

### 4.7 Standard Library Mapping
```text
print()      → fmt.Println
input()      → bufio.Scanner
str/int/float/bool()  → strconv.*
len()        → len()
sorted()     → sort.*
abs, pow, sqrt, floor → math.*
string methods (.upper .split .join .contains) → strings.*
```

### 4.8 NumPy Subset (see also docs/numpy-subset.md — authoritative)
```python
import numpy as np      # the ONE recognized import statement — special-cased, not general

a = np.array([1, 2, 3])
b = np.array([4, 5, 6])
c = a + b                # elementwise add   → generated Go loop
d = a * b                # elementwise multiply
print(c)
```
`np.array([...])` lowers to a Go `[]int` or `[]float64` slice (element type inferred from the literal, same as any other PyGo list). Elementwise `+ - *` between two arrays of statically-known-equal length lower to a small generated `for` loop over a new result slice. **Not supported:** broadcasting, multi-dimensional arrays, `np.dot`/`np.linalg.*`, RNG functions, any dtype beyond PyGo's own `int`/`float`.

### 4.9 Stretch (only after core is stable end-to-end)
```python
y = x if x > 0 else -x
squares = [i*i for i in range(10)]
```

Full grammar lives in `docs/grammar.ebnf`; type rules matrix lives in `internal/types/rules.go` and mirrored in `docs/language-spec.md`.

---

## 5. Phase-by-Phase Plan

Each phase entry below states both its implementation deliverables AND its exact terminal-output contract — the output shape is not an afterthought, it's part of the phase's definition of done.

### Phase 1 — Lexer ✅ DONE
- `internal/token`, `internal/lexer`: rune scanner, indentation stack (INDENT/DEDENT/NEWLINE), literal/operator scanning, lexer-level error recovery.
- 16 table-driven tests, all passing (verified in a real Go environment).
- **`pygoc lex` output contract:** one token per line, `TYPE(lexeme)` format, e.g. `IDENTIFIER(x)`, `ASSIGN(=)`, `INTEGER(10)`, `NEWLINE`.

### Phase 2 — Parser ✅ DONE
- `internal/ast`, `internal/parser`: full statement + precedence-climbing expression grammar, panic-mode error recovery.
- 16 tests, all passing, including a hand-verified precedence trace and an error-recovery trace.
- **`pygoc ast` output contract:** indented tree format (already implemented as `ast.Print`/`ast.Sprint` in `internal/ast/printer.go`), e.g.:
  ```text
  Program
  ├── AssignStmt op==
  │   ├── Targets: Identifier(x)
  │   └── Value: BinaryExpr op=+
  │       ├── IntLiteral(10)
  │       └── IntLiteral(20)
  ```
- **Syntax error output contract:** `Syntax Error: Line L, Column C\nExpected X but found Y`.

### Phase 3a — Symbol Table (NEXT)
- `internal/symbol`: `Scope` with parent pointers (global → function only — if/while/for share the enclosing scope), `Table` for lookups, `IsLocal`/`IsCaptured` helpers for future closure checks.
- Each entry: name, type (`any` placeholder until 3b), declared-at span, kind (variable/parameter/function).

### Phase 3b — Type System
- `internal/types/types.go`: `Type` interface + primitives + `ListType`, `DictType`, `TupleType`, `FuncType`.
- `internal/types/rules.go`: explicit operator type rules (`int + float → float`, `int / int → float`, `int // int → int`, etc.).
- `internal/types/inference.go`: local, statement-by-statement inference.

### Phase 3c — Semantic Analysis
- `internal/semantic/analyzer.go` + `checks.go`: undefined-variable checks, scope-rule enforcement, return-path checks, closure mutation rejection (using `symbol.IsCaptured`).
- `internal/semantic/printer.go`: renders the symbol table + typed AST for `pygoc sema`.
- **`pygoc sema` output contract:**
  ```text
  Symbol Table
  ────────────────────────
  Name      Type      Scope
  ────────────────────────
  x         int       global
  ────────────────────────

  Type Analysis
  x = 10          → int
  print(x)        → valid
  ```
  **Semantic error contract:** `Semantic Error: Line L, Column C\n\nCannot add:\n    int + string\n\nExpected compatible numeric types.`
- Produces the fully typed, validated AST that Phase 4 consumes.

### Phase 4 — IR (Three-Address Code + CFG)
- `internal/ir/tac.go`: instruction set (`t1 = a + b`, `if t2 goto L1 else L2`, `call print, y`, ...).
- `internal/ir/cfg.go`: `BasicBlock` + `CFG` (entry/exit blocks, successor/predecessor edges).
- `internal/ir/builder.go`: lowers each AST construct (`if`/`while`/`for`/calls) into blocks and jumps.
- `internal/ir/printer.go`: renders TAC linearly and the CFG as labeled blocks (`B1:`, `B2:`, ...) with their jump targets.
- **`pygoc ir` output contract:** TAC listing followed by a CFG block listing, exactly matching the format in the language spec's IR examples.

### Phase 5 — Optimizer (multi-pass, in order, each independently visible)
1. `constfold.go` — constant folding (`2 + 3 → 5`).
2. `constprop.go` — constant propagation.
3. `algebraic.go` — algebraic simplification (`x*1→x`, `x+0→x`, `x*0→0`).
4. `cse.go` — common subexpression elimination.
5. `deadcode.go` — dead code elimination.
6. `pipeline.go` — orchestrates pass order; **critically, this is not a black box** — it retains the IR snapshot after each individual pass so the CLI can print all five stages plus the original and final IR.
- **`pygoc optimize` output contract:**
  ```text
  === Original IR ===
  ...
  === Constant Folding ===
  ...
  === Constant Propagation ===
  ...
  === Algebraic Simplification ===
  ...
  === Common Subexpression Elimination ===
  ...
  === Dead Code Elimination ===
  ...
  === Final Optimized IR ===
  ...
  ```
- Each pass independently unit-tested with before/after IR fixtures — never bundled into one opaque "optimize" test.

### Phase 6 — Code Generator
- `internal/codegen/generator.go`: walks the optimized CFG, emits Go source via direct string-building.
- `internal/codegen/imports.go`: tracks which stdlib packages were actually used, emits only those imports.
- `internal/codegen/stdlib_map.go`: PyGo builtin → Go stdlib call mapping.
- `internal/numpy/`: recognizes `import numpy as np` and `np.array`/elementwise-op calls during codegen and lowers them to Go slices + generated loops (see §4.8).
- Runtime representation: `list<T>` → Go slice, `dict<K,V>` → Go map, `tuple<...>` → struct or multiple return values depending on context.
- Generated Go must be `gofmt`-clean and pass `go vet` — enforced as a test.
- **`pygoc compile` output contract:** the generated `.go` source printed to terminal (and optionally written to disk); must compile and run to produce output matching the original PyGo program.

### Cross-cutting — Diagnostics
- `internal/diagnostics`: one shared `Reporter` used by every phase — no phase prints its own ad-hoc errors. All error formats above (syntax, semantic) are implemented once here, not duplicated per phase.

### CLI (`cmd/pygoc`)
```bash
pygoc lex program.py
pygoc ast program.py
pygoc sema program.py
pygoc ir program.py
pygoc optimize program.py
pygoc compile program.py
pygoc run program.py
pygoc check program.py
pygoc pipeline program.py   # runs and labels all six phases in one pass — see README
```
Built incrementally — each debug command becomes available as soon as its corresponding phase exists. `pipeline` is added last, once all six phases exist, since it's a thin orchestrator over the others (no new logic of its own beyond labeled printing).

---

## 6. Testing Strategy — Looping, at Every Level

"Looping to test" is a deliberate strategy, applied at three distinct levels rather than one:

| Level | What loops | How |
|---|---|---|
| **Unit (per package)** | A table of `{input, expected}` cases, one test function | Already how the 32 lexer + parser tests are written — table-driven, one line per new case |
| **Golden (per phase)** | Every `input.py`/`expected.*` pair found in `tests/golden/<phase>/` | A single test function `glob`s the directory and asserts each pair matches — adding a new golden case means adding two files, not new Go code |
| **End-to-end (whole pipeline)** | Every `.py` file in `tests/e2e/`, each paired with an `.expected.txt` | One test function loops the directory: compile → `go build` the result → execute → diff stdout against `.expected.txt`, for every example, reporting all failures in one run rather than stopping at the first |

| Layer | What's tested | How |
|---|---|---|
| Lexer | tokens, indentation edge cases, invalid chars | table-driven unit tests ✅ |
| Parser | grammar rules, precedence, malformed syntax, recovery | table-driven unit tests ✅ |
| Semantic | undefined vars, scope errors, type mismatches, bad returns | table-driven unit tests with expected diagnostics |
| IR | AST → CFG correctness | golden IR-dump comparisons (looped over `tests/golden/ir/`) |
| Optimizer | before/after IR per pass, combined pipeline | golden before/after fixtures (looped), one sub-test per pass |
| Codegen | generated Go compiles + `go vet` clean | golden `.go` file comparisons (looped) |
| NumPy subset | array creation + elementwise ops lower correctly | table-driven, plus one e2e example |
| End-to-end | compile + execute + check stdout | looped over every `tests/e2e/*.py` |
| Benchmarks | timing per phase on representative programs | `go test -bench`, looped over `benchmarks/*.py` |

---

## 7. Library Support — NumPy Subset (Rationale)

**Why NumPy, not Pandas:** NumPy's core need — fixed-shape numeric arrays with elementwise arithmetic — maps directly onto a Go slice plus a `for` loop, with zero new runtime types. Pandas' core need is a `DataFrame`: labeled, heterogeneous-column, indexable-by-name tabular data, which would require building an entire small runtime library (not just a codegen mapping) — meaningfully bigger scope, not realistic inside 7 weeks alongside the other five phases.

**Supported (documented exhaustively in `docs/numpy-subset.md`, kept authoritative there as it's extended):**
- `import numpy as np` — recognized as one special-cased statement, not a general import mechanism
- `np.array([...])` → Go slice, element type inferred same as any PyGo list literal
- Elementwise `a + b`, `a - b`, `a * b` between two same-length arrays → generated loop
- `print(array)` → `fmt.Println` on the underlying slice

**Not supported, explicitly:** broadcasting between different shapes, multi-dimensional arrays, `np.dot`/`np.linalg.*`, random number generation, non-`int`/`float` dtypes, any other `np.*` function not listed above.

**Where it sits in the pipeline:** NumPy recognition happens in `internal/numpy`, called only from `internal/codegen` — it is a codegen-time lowering concern (how do we generate Go for this array operation), not a separate compiler phase, exactly as shown in the pipeline diagram in §1/§2.

---

## 8. Risks & Guardrails

| Risk | Mitigation |
|---|---|
| Closures spiral into full lexical-scope complexity | Hard rule: read-only capture only, enforced in `semantic/checks.go` |
| Optimizer becomes "just constant folding" | Each pass gets its own dedicated implementation + test file, not bundled; `pipeline.go` retains every intermediate snapshot so this is externally verifiable, not just a claim |
| Codegen becomes string-spaghetti | One function per AST/IR node type, tested independently |
| NumPy subset scope-creeps toward "implement NumPy" | Hard cap: the 4 operations listed in §7, nothing more, until/unless everything else is done early |
| Documentation left to the last week | `docs/*.md` written incrementally, one per completed phase |
| Skipping ahead before a phase is verified | **Hard rule: don't start Phase N+1 until Phase N's tests are confirmed passing in a real Go environment** |
| 7-week timeline runs long | Cut order if behind schedule: NumPy subset first, then optimizer down to 2–3 passes (constant folding + dead code elimination are the highest-value pair), then dict/tuple support — the six core phases and their CLI visibility are never cut |

---

## 9. Timeline (7 Weeks)

| Week | Deliverable |
|---|---|
| 1 | Language spec, grammar design + Lexer (with tests) — **done** |
| 2 | Parser — recursive descent + expression grammar (with tests) — **done** |
| 3 | Symbol table + type system (Phase 3a/3b) |
| 4 | Semantic analysis (Phase 3c) + start of IR construction (Phase 4) |
| 5 | Complete IR (CFG) + Optimizer passes (Phase 5) |
| 6 | Code generator + CLI, including `pipeline` command (Phase 6) |
| 7 | NumPy subset, end-to-end/golden test loops, benchmarks, documentation, final report |

---

## 10. Immediate Next Actions

1. **Phase 3a — Symbol Table.** `internal/symbol/scope.go` (Scope + ScopeKind mechanics) and `internal/symbol/table.go` (Symbol type + Table facade), with a table-driven `symbol_test.go` covering scope chains, redefinition, shadowing, and the `IsLocal`/`IsCaptured` helpers needed later for closure checks.
2. Update the status table in `README.md` as each phase completes.
3. Do not start Phase 3b until 3a's tests are confirmed passing.

---

## 11. Viva / Defense Topics (grows per-phase)

- Why indentation-based lexing is genuinely hard, and how INDENT/DEDENT solves it.
- Why `internal/token` is its own package instead of living inside `lexer`.
- Recursive descent vs. parser generators — tradeoffs.
- Why symbol tables need scope chains, not a single flat map — and why `if`/`while`/`for` don't get their own scope in Python-family semantics.
- How type inference differs from type checking.
- Why TAC + CFG (basic blocks) instead of a flat IR instruction list.
- What each optimization pass actually proves/preserves (correctness of transformations), and why the pipeline keeps every intermediate snapshot instead of just the final result.
- Why panic-mode error recovery matters for usability.
- Design decisions mapping PyGo collections to Go's slice/map runtime types.
- Why NumPy was chosen over Pandas, and why only 3 operations are supported.
- What was deliberately excluded (classes, exceptions, generators) and why.
