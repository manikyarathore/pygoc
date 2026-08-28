# PyGoC — Full Roadmap & Technical Framework

> This is the authoritative, up-to-date plan for the project — the single document to re-read before starting any new phase. It reflects the actual repo structure in use (including `internal/token` as its own package) and current progress.

---

## 0. Project Identity

**Name:** PyGoC
**What it is:** a hand-written compiler for "PyGo" — a statically-typed Python-subset language — targeting Go as the output.
**Standard we're holding it to:** not GCC's *scope*, but GCC's *standard of correctness and craftsmanship*, at PyGo's scope.

---

## 1. Full Pipeline (Reference)

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
Optimized CFG (multi-pass)
   ↓  [internal/codegen]
Go Source (+ auto-resolved imports)
   ↓
go build → Executable
```

---

## 2. Repository Structure (Authoritative — matches actual repo)

```text
pygoc/
├── cmd/
│   └── pygoc/                  # main.go — CLI entrypoint only, no logic
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
│   ├── parser/                  # Phase 2 — tokens → AST
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
│   │   └── semantic_test.go
│   ├── ir/                      # Phase 4 — typed AST → IR
│   │   ├── tac.go                 # three-address code instruction set
│   │   ├── cfg.go                 # basic blocks + control flow graph
│   │   ├── builder.go             # AST → IR lowering
│   │   └── ir_test.go
│   ├── optimizer/               # Phase 5 — IR → optimized IR
│   │   ├── constfold.go
│   │   ├── constprop.go
│   │   ├── deadcode.go
│   │   ├── cse.go                 # common subexpression elimination
│   │   ├── algebraic.go
│   │   ├── pipeline.go            # pass ordering/orchestration
│   │   └── optimizer_test.go
│   ├── codegen/                 # Phase 6 — IR → Go source
│   │   ├── generator.go
│   │   ├── stdlib_map.go          # PyGo builtins → Go stdlib calls
│   │   ├── imports.go             # auto-import resolution
│   │   └── codegen_test.go
│   └── diagnostics/              # cross-cutting: used by every phase
│       ├── error.go
│       ├── reporter.go            # pretty terminal error output w/ source spans
│       └── diagnostics_test.go
├── tests/
│   ├── golden/                   # input.py + expected.go pairs
│   └── e2e/                       # compile + run + check output
├── examples/                      # showcase PyGo programs
├── benchmarks/
├── docs/
│   ├── language-spec.md
│   ├── grammar.ebnf
│   ├── architecture.md
│   └── viva-question-bank.md
├── go.mod
└── README.md
```

**Dependency rule enforced throughout:** each package only imports packages *earlier* in the pipeline. `parser` may import `lexer`/`token`/`ast`; `lexer` must never import `parser`. This keeps the dependency graph a straight line, mirroring the pipeline itself.

---

## 3. Language Specification (Full Feature Set)

### 3.1 Types
```text
int, float, bool, string, void
list<T>
dict<K,V>
tuple<T1,T2,...>
function types: (T1,T2)->T3
```

### 3.2 Literals
```python
10          # int
3.14        # float
True False  # bool
"hello"     # string
[1,2,3]     # list<int>
{"a":1}     # dict<string,int>
(1,"x")     # tuple<int,string>
```

### 3.3 Operators
```text
Arithmetic:   + - * / % //
Comparison:   == != < > <= >=
Boolean:      and or not
Unary:        - not
Assignment:   = += -= *= /=
```

### 3.4 Control Flow
```python
if cond: ... elif cond: ... else: ...
while cond: ...
for x in range(a, b): ...
break
continue
```

### 3.5 Functions
```python
def add(a: int, b: int) -> int:
    return a + b

def divmod2(a: int, b: int) -> (int, int):
    return a // b, a % b
```
Recursion supported. Nested function definitions supported. Closures are **read-only capture only** — a nested function may read an outer variable, never reassign it.

### 3.6 Collections
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

### 3.7 Standard Library Mapping
```text
print()      → fmt.Println
input()      → bufio.Scanner
str/int/float/bool()  → strconv.*
len()        → len()
sorted()     → sort.*
abs, pow, sqrt, floor → math.*
string methods (.upper .split .join .contains) → strings.*
```

### 3.8 Stretch (only after core is stable end-to-end)
```python
y = x if x > 0 else -x
squares = [i*i for i in range(10)]
```

Full grammar lives in `docs/grammar.ebnf`; type rules matrix lives in `internal/types/rules.go` and mirrored in `docs/language-spec.md`.

---

## 4. Phase-by-Phase Plan

### Phase 1 — Lexer ✅ DONE
- `internal/token`: token vocabulary, keyword table.
- `internal/lexer`: rune scanner, indentation stack (INDENT/DEDENT/NEWLINE), string/number/identifier scanning, operator scanning, lexer-level error recovery.
- 16 table-driven tests covering literals, keywords, nested blocks, blank/comment lines, tab-rejection, mismatched dedents, bracket-continuation, no-trailing-newline, illegal-character recovery.
- **Action item before moving on:** run `go test ./... -v` locally and confirm all green — this hasn't been executed in a real Go environment yet.

### Phase 2 — Parser (NEXT)
- `internal/ast`: node types only (`expr.go`, `stmt.go`) — no behavior.
- `internal/parser/parser.go`: statement-level recursive descent.
- `internal/parser/expr_parser.go`: precedence-climbing expression parser, isolated from statement parsing.
- `internal/parser/recovery.go`: panic-mode recovery — on a syntax error, skip to the next statement boundary (NEWLINE at the correct indent level) and keep parsing, so one run reports every syntax error in the file.
- `internal/ast/printer.go`: pretty-printer, powers the future `pygoc ast` debug command.
- Tests: one grammar rule at a time, plus deliberately malformed inputs to exercise recovery.

### Phase 3a — Symbol Table
- `internal/symbol`: `Scope` with parent pointers (global → function → block), `Table` for lookups.
- Each entry: name, type, declared-at span, mutability (needed for closure read-only enforcement later).

### Phase 3b — Type System
- `internal/types/types.go`: `Type` interface + primitives + `ListType`, `DictType`, `TupleType`, `FuncType`.
- `internal/types/rules.go`: explicit operator type rules (e.g. `int + float → float`, `int / int → float`, `int // int → int`).
- `internal/types/inference.go`: local, statement-by-statement inference — deliberately not full Hindley-Milner, since PyGo doesn't need polymorphism.

### Phase 3c — Semantic Analysis
- `internal/semantic/analyzer.go` + `checks.go`: undefined-variable checks, scope-rule enforcement, return-path checks (does every path in a `-> T` function return?), closure mutation rejection.
- Produces the fully typed, validated AST that Phase 4 consumes.

### Phase 4 — IR (Three-Address Code + CFG)
- `internal/ir/tac.go`: instruction set (`t1 = a + b`, `if t2 goto L1 else L2`, ...).
- `internal/ir/cfg.go`: `BasicBlock` + `CFG` (entry/exit blocks, successor/predecessor edges).
- `internal/ir/builder.go`: lowers each AST construct (`if`/`while`/`for`/calls) into blocks and jumps.

### Phase 5 — Optimizer (multi-pass, in order)
1. `constfold.go` — constant folding (`2 + 3 → 5`).
2. `constprop.go` — constant propagation.
3. `algebraic.go` — algebraic simplification (`x*1→x`, `x+0→x`, `x*0→0`).
4. `cse.go` — common subexpression elimination.
5. `deadcode.go` — dead code elimination.
6. `pipeline.go` — orchestrates pass order; each pass independently unit-tested with before/after IR fixtures.

### Phase 6 — Code Generator
- `internal/codegen/generator.go`: walks the optimized CFG, emits Go source via direct string-building.
- `internal/codegen/imports.go`: tracks which stdlib packages were actually used, emits only those imports.
- `internal/codegen/stdlib_map.go`: PyGo builtin → Go stdlib call mapping.
- Runtime representation: `list<T>` → Go slice, `dict<K,V>` → Go map, `tuple<...>` → struct or multiple return values depending on context.
- Generated Go must be `gofmt`-clean and pass `go vet` — enforced as a test.

### Cross-cutting — Diagnostics
- `internal/diagnostics`: one shared `Reporter` used by every phase — no phase prints its own ad-hoc errors.
- Every error carries file, line, column, span, severity, message.

### CLI (`cmd/pygoc`)
```bash
pygoc lex file.py
pygoc ast file.py
pygoc sema file.py
pygoc ir file.py
pygoc optimize file.py
pygoc compile file.py
pygoc run file.py
pygoc check file.py
```
Built incrementally — each debug command becomes available as soon as its corresponding phase exists.

---

## 5. Testing Strategy

| Layer | What's tested | How |
|---|---|---|
| Lexer | tokens, indentation edge cases, invalid chars | table-driven unit tests ✅ |
| Parser | grammar rules, precedence, malformed syntax, recovery | unit tests + malformed-input cases |
| Semantic | undefined vars, scope errors, type mismatches, bad returns | unit tests with expected diagnostics |
| IR | AST → CFG correctness | golden IR-dump comparisons |
| Optimizer | before/after IR per pass, combined pipeline | golden before/after fixtures |
| Codegen | generated Go compiles + `go vet` clean | golden `.go` file comparisons |
| End-to-end | compile + execute + check stdout | `tests/e2e/*.py` + expected output |
| Benchmarks | timing per phase on representative programs | `go test -bench` |

---

## 6. Risks & Guardrails

| Risk | Mitigation |
|---|---|
| Closures spiral into full lexical-scope complexity | Hard rule: read-only capture only, enforced in `semantic/checks.go` |
| Optimizer becomes "just constant folding" | Each pass gets its own dedicated implementation + test file, not bundled |
| Codegen becomes string-spaghetti | One function per AST/IR node type, tested independently |
| Documentation left to the last week | `docs/*.md` written incrementally, one per completed phase |
| Skipping ahead before a phase is verified | **Hard rule: don't start Phase N+1 until Phase N's tests are confirmed passing in a real Go environment** |

---

## 7. Immediate Next Actions

1. **Verify Phase 1**: run `go test ./... -v` in a real Go environment, confirm all 16 lexer tests pass. This has not yet been executed — the code was hand-traced for correctness, not machine-verified.
2. **Start Phase 2**: define `internal/ast` node types first (data only), then `internal/parser/parser.go` for statements, then `internal/parser/expr_parser.go` for expressions, then `internal/parser/recovery.go`.
3. Update the status table in `README.md` as each phase completes.

---

## 8. Viva / Defense Topics (grows per-phase)

- Why indentation-based lexing is genuinely hard, and how INDENT/DEDENT solves it.
- Why `internal/token` is its own package instead of living inside `lexer`.
- Recursive descent vs. parser generators — tradeoffs.
- Why symbol tables need scope chains, not a single flat map.
- How type inference differs from type checking.
- Why TAC + CFG (basic blocks) instead of a flat IR instruction list.
- What each optimization pass actually proves/preserves (correctness of transformations).
- Why panic-mode error recovery matters for usability.
- Design decisions mapping PyGo collections to Go's slice/map runtime types.
- What was deliberately excluded (classes, exceptions, generators) and why.
