# PyGoC — The Complete Hardest-Tier Roadmap
### A Python-Subset Compiler Targeting Go — Full Technical Blueprint

> Goal: build the strongest, most complete student compiler project realistically achievable — deep enough to be a legitimate systems project, modular enough to actually learn from, and documented well enough to defend in a viva and show off on a CV/GitHub.

---

## 0. Project Identity

**Name:** PyGoC
**Tagline:** *A statically-typed Python-subset language ("PyGo"), compiled to idiomatic Go, built entirely from scratch — lexer through optimizer through code generator.*

**Difficulty tier chosen:** Hardest realistic (not "impossible," but the ceiling of what one student can build solo in a semester with disciplined scoping).

**What makes this the "hardest" version vs. the earlier medium plan:**
- Full indentation-sensitive lexer (INDENT/DEDENT/NEWLINE) — not simplified.
- Rich type system: `int, float, bool, string, void, list<T>, tuple<T...>, dict<K,V>, function types`.
- Control flow: `if/elif/else`, `while`, `for ... in range(...)`, `break`, `continue`, nested loops.
- Functions: parameters, return values (including multiple returns), recursion, nested function scopes, closures (restricted — no captured mutation, to keep semantics sane).
- Expressions: full precedence-climbing expression grammar, unary ops, ternary (`x if cond else y`), boolean short-circuiting.
- Collections: `list<T>`, `dict<K,V>`, `tuple<T...>` with indexing, slicing (basic), and common methods.
- Standard library mapping layer: `fmt`, `strconv`, `strings`, `math`, `sort`, `bufio`, `errors`.
- Real IR: three-address code (TAC) lowered into a **Control Flow Graph (CFG)** of basic blocks — not just a flat instruction list.
- Multi-pass optimizer: constant folding, constant propagation, dead-code elimination, common subexpression elimination, algebraic simplification, and (stretch) loop-invariant code motion.
- Full diagnostics system with **error recovery** (the compiler keeps parsing after an error to report multiple issues per run, like a real compiler) instead of stopping at the first error.
- Golden-file end-to-end tests, benchmark suite, and a debug CLI exposing every internal stage.

---

## 1. Why This Scope Is "Hardest But Feasible"

| Component | Why it's hard | Why it's still finishable |
|---|---|---|
| Indentation lexer | Python's tokenizer algorithm has real edge cases (tabs vs spaces, blank lines, comments mid-block) | Algorithm is public, well-documented, and mechanical once understood |
| Type system w/ generics-lite (`list<T>`) | Needs monomorphization-style reasoning without full generics | You only need *one level* of parametrization, not a general generics system |
| CFG-based IR + optimizer | Real compilers do this; conceptually deep | You control the language, so CFGs stay small and predictable |
| Error recovery | Naive parsers die on first error | Panic-mode recovery (skip to next statement boundary) is a known, teachable technique |
| Closures | Genuinely hard in general | You restrict to read-only capture — cuts 80% of the complexity |

**Explicitly excluded even at hardest tier** (to avoid the "huge incomplete compiler" trap your original doc warned about):
classes/OOP, exceptions (`try/except`), generators/`yield`, decorators, multiple inheritance, full dynamic typing, metaclasses, `*args`/`**kwargs`, imports/modules, async.

---

## 2. Repository Structure (Modular by Design)

Every folder = one compiler concept, one Go package, independently testable and independently *readable*. This is deliberate: you should be able to open any single folder and understand that phase without reading the rest of the compiler.

```text
pygoc/
├── cmd/
│   └── pygoc/                # main.go — CLI entrypoint only, no logic
├── internal/
│   ├── lexer/                 # Phase 1: source → tokens
│   │   ├── token.go
│   │   ├── lexer.go
│   │   ├── indent.go          # INDENT/DEDENT/NEWLINE logic, isolated
│   │   └── lexer_test.go
│   ├── ast/                   # Phase 2 data structures (no logic, just types)
│   │   ├── expr.go
│   │   ├── stmt.go
│   │   └── printer.go         # AST pretty-printer for `pygoc ast` debug cmd
│   ├── parser/                # Phase 2: tokens → AST
│   │   ├── parser.go
│   │   ├── expr_parser.go     # precedence climbing, isolated from statement parsing
│   │   ├── recovery.go        # panic-mode error recovery
│   │   └── parser_test.go
│   ├── symbol/                # Phase 3a: scopes & symbol tables
│   │   ├── table.go
│   │   ├── scope.go
│   │   └── symbol_test.go
│   ├── types/                 # Phase 3b: type system
│   │   ├── types.go            # Type interface + primitives
│   │   ├── inference.go        # type inference engine
│   │   ├── rules.go            # operator type rules (int+int→int, etc.)
│   │   └── types_test.go
│   ├── semantic/              # Phase 3c: semantic analysis (uses symbol + types)
│   │   ├── analyzer.go
│   │   ├── checks.go           # undefined vars, scope errors, return checks
│   │   └── semantic_test.go
│   ├── ir/                    # Phase 4: typed AST → IR
│   │   ├── tac.go              # three-address code instruction set
│   │   ├── cfg.go              # basic blocks + control flow graph
│   │   ├── builder.go          # AST → IR lowering
│   │   └── ir_test.go
│   ├── optimizer/             # Phase 5: IR → optimized IR
│   │   ├── constfold.go
│   │   ├── constprop.go
│   │   ├── deadcode.go
│   │   ├── cse.go               # common subexpression elimination
│   │   ├── algebraic.go
│   │   ├── pipeline.go          # pass ordering/orchestration
│   │   └── optimizer_test.go
│   ├── codegen/               # Phase 6: IR → Go source
│   │   ├── generator.go
│   │   ├── stdlib_map.go        # PyGo builtins → Go stdlib calls
│   │   ├── imports.go           # auto-import resolution
│   │   └── codegen_test.go
│   └── diagnostics/            # cross-cutting: used by every phase
│       ├── error.go
│       ├── reporter.go          # pretty terminal error output w/ source spans
│       └── diagnostics_test.go
├── tests/
│   ├── golden/                 # input.py + expected.go pairs
│   └── e2e/                    # compile + run + check output
├── examples/                   # showcase PyGo programs
├── benchmarks/
├── docs/
│   ├── language-spec.md
│   ├── grammar.ebnf
│   ├── architecture.md
│   └── viva-question-bank.md
├── go.mod
└── README.md
```

**Design rule enforced throughout:** each `internal/X` package only imports packages *earlier* in the pipeline (parser imports lexer+ast, never the reverse). This keeps the dependency graph a straight line — mirrors the compiler pipeline itself and makes the codebase self-documenting.

---

## 3. Full Compiler Pipeline

```text
Source (.py)
   ↓
[lexer]      → Token Stream (with INDENT/DEDENT/NEWLINE)
   ↓
[parser]     → AST (with panic-mode error recovery)
   ↓
[symbol]     → Scoped Symbol Table
   ↓
[types]      → Type Inference
   ↓
[semantic]   → Typed, Validated AST + Diagnostics
   ↓
[ir/builder] → Three-Address Code
   ↓
[ir/cfg]     → Control Flow Graph (basic blocks)
   ↓
[optimizer]  → Optimized CFG (multi-pass)
   ↓
[codegen]    → Go Source (+ auto imports)
   ↓
go build     → Executable
```

---

## 4. Language Specification — "PyGo" (Full Feature Set)

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

### 4.3 Operators (with defined precedence and type rules)
```text
Arithmetic:   + - * / % //
Comparison:   == != < > <= >=
Boolean:      and or not
Unary:        - not
Assignment:   = += -= *= /=
```
Full precedence table and type-promotion rules go in `docs/language-spec.md` (defined *before* the parser is written — see Phase 0 deliverable).

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

def divmod2(a: int, b: int) -> (int, int):   # multiple return
    return a // b, a % b
```
- Recursion supported.
- Nested function definitions supported.
- Closures: **read-only capture only** (a nested function may read an outer variable, not reassign it) — this is the deliberate simplification that keeps closures tractable.

### 4.6 Collections
```python
nums = [1,2,3]
nums.append(4)
len(nums)
nums[0]
nums[1:3]          # basic slicing

d = {"a": 1}
d["b"] = 2

t = (1, "x")
a, b = t            # tuple unpacking
```

### 4.7 Builtins mapped to Go stdlib
```text
print()      → fmt.Println
input()      → bufio.Scanner
str/int/float/bool()  → strconv.*
len()        → len()
sorted()     → sort.*
abs, pow, sqrt, floor → math.*
string methods (.upper .split .join .contains) → strings.*
```

### 4.8 Ternary & comprehensions (stretch, only after core is stable)
```python
y = x if x > 0 else -x
squares = [i*i for i in range(10)]
```

---

## 5. Grammar (Initial EBNF Sketch)

```ebnf
program        := statement* ;
statement      := simple_stmt | compound_stmt ;
simple_stmt    := (assign | expr_stmt | return_stmt | break | continue) NEWLINE ;
compound_stmt  := if_stmt | while_stmt | for_stmt | func_def ;

if_stmt        := "if" expr ":" block ("elif" expr ":" block)* ("else" ":" block)? ;
while_stmt     := "while" expr ":" block ;
for_stmt       := "for" IDENT "in" "range" "(" expr ("," expr)? ")" ":" block ;
func_def       := "def" IDENT "(" params? ")" ("->" type)? ":" block ;
block          := NEWLINE INDENT statement+ DEDENT ;

expr           := ternary ;
ternary        := or_expr ("if" or_expr "else" or_expr)? ;
or_expr        := and_expr ("or" and_expr)* ;
and_expr       := not_expr ("and" not_expr)* ;
not_expr       := "not" not_expr | comparison ;
comparison     := arith (("==" | "!=" | "<" | ">" | "<=" | ">=") arith)* ;
arith          := term (("+" | "-") term)* ;
term           := unary (("*" | "/" | "%" | "//") unary)* ;
unary          := "-" unary | primary ;
primary        := literal | IDENT | call | index | "(" expr ")" ;
```
(Full grammar finalized in `docs/grammar.ebnf` before Phase 2 begins — per your existing "design before code" rule.)

---

## 6. Type System Design

- **Type interface**: every type (`IntType`, `FloatType`, `ListType{Elem}`, `FuncType{Params,Return}`, etc.) implements a common `Type` interface with `String()` and `Equals()`.
- **Inference algorithm**: local, statement-by-statement inference (not full Hindley-Milner — deliberately simpler, since PyGo doesn't need polymorphism). Each `x = expr` infers `expr`'s type and binds it in the symbol table.
- **Type rules matrix** (defined explicitly, e.g.):
```text
int + int   → int
float + float → float
int + float → float   (implicit widening)
int / int   → float    (true division, like Python)
int // int  → int      (floor division)
string + string → string  (concatenation)
```
- **Type errors** are semantic errors, reported with source spans, not panics.

---

## 7. Symbol Table & Scoping

- **Scope chain**: global scope → function scope → block scope (if/while/for don't introduce new *variable* scopes in Python semantics — only functions do; this must be modeled correctly).
- Each `Scope` has a parent pointer; lookups walk up the chain.
- Function scopes track: parameters, locals, return type, whether all paths return (for `-> T` functions).
- Symbol table entries store: name, type, declared-at span, mutability info (for closures).

---

## 8. IR Design — Three-Address Code + CFG

**Why TAC + CFG instead of a flat instruction list:** basic blocks are what make real optimizations (dead-code elimination, CSE) tractable — you need "does this value get used before it's redefined" reasoning, which requires block/edge structure, not just a linear list.

```text
Instruction forms:
  t1 = a + b
  t2 = t1 * c
  if t2 goto L1 else L2
  L1:
  x = t2
  goto L3
  L2:
  ...
```

- `BasicBlock`: list of instructions + successor/predecessor edges.
- `CFG`: entry block, exit block(s), full block graph per function.
- IR builder walks the typed AST and lowers each construct (if/while/for/function-call) into blocks + jumps — this is one of the richest teaching moments in the whole project (control flow → graph structure).

---

## 9. Optimization Passes (in pipeline order)

1. **Constant folding** — `2 + 3` → `5` at compile time.
2. **Constant propagation** — replace uses of a known-constant variable with the constant.
3. **Algebraic simplification** — `x * 1 → x`, `x + 0 → x`, `x * 0 → 0`.
4. **Common subexpression elimination (CSE)** — reuse a previously computed value instead of recomputing.
5. **Dead-code elimination** — remove instructions/blocks whose results are never used or are unreachable.
6. *(Stretch)* **Loop-invariant code motion** — hoist computations that don't change across loop iterations.

Each pass is its own file/function, takes a CFG in, returns a (possibly changed) CFG out, and is independently unit-testable with "before IR / after IR" fixtures — this is what `pygoc optimize` will show side-by-side.

---

## 10. Code Generation Strategy

- Walk the optimized CFG per function, emit Go source using a `text/template`-free, direct string-builder approach (easier to debug than templates for a first version).
- **Auto-import resolution**: `codegen/imports.go` tracks which stdlib packages were actually used (`fmt`, `strconv`, `strings`, `math`, `sort`, `bufio`) and emits only those import lines.
- **Runtime representation choices** (documented explicitly, since this is a real design decision):
  - PyGo `list<T>` → Go slice `[]T`
  - PyGo `dict<K,V>` → Go `map[K]V`
  - PyGo `tuple<...>` → Go struct or multiple return values, depending on context
- Generated Go must be `gofmt`-clean and pass `go vet` — this is itself a test.

---

## 11. Diagnostics & Error Recovery

- Every phase reports errors through one shared `diagnostics.Reporter`, not ad-hoc `fmt.Println`.
- Errors carry: source file, line, column, span, severity, message, and (where possible) a suggested fix.
- **Panic-mode recovery in the parser**: on a syntax error, skip tokens until the next statement boundary (NEWLINE at the right indent level) and keep parsing — so one `pygoc compile` run can report *all* syntax errors in a file, not just the first. This is a legitimate, well-known compiler-design technique worth a full viva section.

---

## 12. CLI Design

```bash
pygoc lex file.py        # print token stream
pygoc ast file.py        # print AST
pygoc sema file.py       # print symbol table + inferred types
pygoc ir file.py         # print unoptimized IR/CFG
pygoc optimize file.py   # print optimized IR/CFG (before/after diff)
pygoc compile file.py    # emit Go source
pygoc run file.py        # compile + go run, single command
pygoc check file.py      # semantic check only, no codegen (like a linter)
```
Every debug command is essentially a partial pipeline run stopped early and printed — reinforcing the "each phase is independently inspectable" design goal.

---

## 13. Testing Strategy

| Layer | What's tested | How |
|---|---|---|
| Lexer | tokens, indentation edge cases, invalid chars | table-driven unit tests |
| Parser | grammar rules, precedence, malformed syntax, recovery | unit tests + fuzz-style malformed inputs |
| Semantic | undefined vars, scope errors, type mismatches, bad returns | unit tests with expected diagnostics |
| IR | AST → CFG correctness | golden IR-dump comparisons |
| Optimizer | before/after IR per pass, combined pipeline | golden before/after fixtures |
| Codegen | generated Go compiles + `go vet` clean | golden `.go` file comparisons |
| End-to-end | compile + execute + check stdout | `tests/e2e/*.py` + expected output |
| Benchmarks | timing per phase on representative programs | `go test -bench` |

Golden test format:
```text
tests/golden/factorial.py
tests/golden/factorial.expected.go
tests/e2e/factorial.py
tests/e2e/factorial.expected.txt
```

---

## 14. Semester Timeline & Milestones

```text
Week 1-2   Phase 0: language spec, grammar, architecture doc (no code)
Week 3-4   v0.1  Lexer + indentation handling, full test suite
Week 5-6   v0.2  Parser (recursive descent) + error recovery
Week 6     v0.3  AST module + pretty printer
Week 7-8   v0.4  Symbol table + scoping
Week 8-9   v0.5  Type system + inference + semantic analysis
Week 10    v0.6  IR (TAC) + CFG builder
Week 11    v0.7  Optimizer passes (start with constant folding, add incrementally)
Week 12-13 v0.8  Code generator + stdlib mapping
Week 13    v0.9  CLI polish, all debug commands working
Week 14    v1.0  End-to-end tests, golden tests, benchmarks, docs
Week 15    Buffer: report writing, viva prep, README polish
```

**Hard rule (yours, and a good one):** correctness and architecture come before advanced features. If week 11 arrives and the optimizer isn't started, cut list comprehensions/ternary before cutting CSE — core pipeline completeness beats feature count.

---

## 15. Risks & How to Avoid Scope Creep

| Risk | Mitigation |
|---|---|
| Indentation lexer eats too much time | Timebox to 1.5 weeks; reference Python's public tokenizer algorithm rather than inventing your own |
| Closures spiral into full lexical-scope complexity | Hard rule: read-only capture only, enforced by a semantic check that rejects reassignment of captured vars |
| Optimizer becomes "just constant folding" | Each pass gets its own week slot in the timeline, not "later" |
| Codegen becomes string-spaghetti | One function per AST/IR node type, tested independently |
| Report/documentation left to the last week | Write `docs/*.md` incrementally, one per completed phase, not all at the end |

---

## 16. Final Deliverables Checklist

```text
☐ Compiler source (fully modular, per structure in §2)
☐ CLI with all 8 debug/run commands
☐ Language specification document
☐ Formal grammar (EBNF)
☐ Type system documentation
☐ IR + CFG design documentation
☐ Optimizer pass documentation (before/after examples)
☐ Unit tests for every phase
☐ Golden tests (input/output pairs)
☐ End-to-end example programs
☐ Benchmark suite + results
☐ Architecture diagram
☐ README (professional, 20-section structure)
☐ Final project report
☐ Viva question bank
```

---

## 17. CV Positioning (once actually built — only claim what's real)

> **PyGoC — Python-Subset Compiler Targeting Go**
> Designed and implemented a full compiler pipeline from lexical analysis through a CFG-based IR, multi-pass optimizer, and Go code generation. Built a hand-written recursive-descent parser with panic-mode error recovery, an indentation-sensitive lexer, a scoped symbol table, a static type inference engine, and a three-address-code IR lowered to basic blocks. Implemented constant folding, constant propagation, common subexpression elimination, algebraic simplification, and dead-code elimination as independent, composable optimization passes. Shipped as a CLI exposing every internal compiler stage for inspection, with golden-file regression tests, end-to-end execution tests, and compiler performance benchmarks.

---

## 18. Viva Topics (headline list — full question bank built per-phase as we go)

- Why indentation-based lexing is genuinely hard (and how INDENT/DEDENT solves it)
- Recursive descent vs. parser generators — tradeoffs
- Why symbol tables need scope chains, not a single flat map
- How type inference differs from type checking
- Why TAC + CFG (basic blocks) instead of a flat IR list
- What each optimization pass actually proves/preserves (correctness of transformations)
- Why panic-mode error recovery matters for usability
- Design decisions in mapping PyGo collections to Go's slice/map runtime types
- What was deliberately excluded (classes, exceptions, generators) and why

---

## 19. What We Build First

**Next immediate step:** Phase 0 is *already done* by this document, except one thing — the **finalized, word-for-word language specification and grammar** should be split into their own `docs/language-spec.md` and `docs/grammar.ebnf` files before a single line of lexer code is written, per your own rule.

**Recommended first coding milestone:** `internal/lexer` — token types, then the indentation algorithm, then a full test suite — since every later phase depends on a correct token stream.

---

*End of blueprint. Do not begin Phase 1 implementation until this document is confirmed.*
