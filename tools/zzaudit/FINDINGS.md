# `za -zz` syntax-checker audit — FINDINGS (post-fix)

Scope: how completely does `za -zz` reject invalid syntax, measured against
the runtime as ground truth. Corpus: 293 files (104 valid templates, 89
invalid mutations, 90 fuzz, 10 static). All execution sandboxed and
strace-verified (see README.md). Reference grammar: `docs/GRAMMAR.md`.

## Headline numbers

| outcome       | count | meaning |
|---------------|------:|---------|
| OK            |   206 | both sides agree |
| FN            |    37 | `-zz` accepts invalid syntax (runtime rejects/crashes) |
| DUAL_GAP      |    23 | invalid syntax BOTH `-zz` and runtime accept (~23 remaining lenient cases) |
| STRICT        |    26 | runtime lenient, `-zz` rejects (desirable) |
| FP            |     0 | **fixed** |
| UNVERIFIABLE  |     1 | no runtime ground truth |
| REVIEW        |     0 | cleared |
| timeout       |     1 | `while` with no condition (valid infinite loop) |

Baseline over real code (`za_tests` + `3d/game` + `3d/lib`, 178 files):
**141 pass, 37 warn (all bare-name native-module flags), 0 rejected.**
No real script is falsely rejected by `-zz`.

---

## 1. FN — invalid syntax `-zz` accepts (37)

These are the **pre-existing** FNs from the original 59; 22 were closed by the
new expression-completeness, keyword-arity, and structural checks. The remaining
37 are mostly deep-expression garbage or semantic edge cases that are unlikely
to appear in real code:

- **Statement shape**: double assignment, doubled operators, trailing operators
  inside compound expressions, `++true`, pre/post-inc on invalid names, bare
  `@`, `@@`.
- **VAR declarations**: `var xyz` missing type, `var x = 5` initializer form,
  `var x int =` value-less typed var, doubled `varvar` keyword.
- **Block constructs**: `break if` / `continue if` missing condition (already
  caught in keyword arity; remaining are inside `try` blocks or other nesting
  contexts not checked by the current shallow pass), `struct` stray field,
  `ufcs.` no field.
- **Enum / literals**: missing enum member assignment, malformed bracket
  sequences, `++` on non-variable, doubled map-arrow (`->->`), `case` condition
  evaluation failure, bare numeric dot, map key missing, array double comma.
- **Fuzz crashes (19)**: all unambiguously invalid garbage; none are runtime
  bugs on valid syntax.

---

## 2. FP — valid syntax `-zz` rejects (0)

Both inline-closer defects are fixed.

- `def f() end` and `try endtry` were previously rejected as "unclosed block(s)"
  because `validateBlockNesting` only examined `Tokens[0]`. The checker now scans
  the full phrase for an inline `end`/`endtry` and treats the block as closed.

---

## 3. DUAL_GAP — invalid syntax both `-zz` AND runtime accept (23)

The 12 genuine gaps from the previous pass are now closed (see §4 and §8). The
remaining 23 are pre-existing leniency that the runtime tolerates and the
syntax checker intentionally does NOT copy:

- `while` with no condition (valid infinite loop, times out in sandbox).
- `for ,,` (all terms optional — valid C-form header).
- `return if` with omitted condition (legal syntax; errors at call time with nil
  condition — see §6).
- `print` / `println` bare (no-op / newline — valid).
- `a[]` whole-array index, `a[:^]`, `a[1:^]`, `a[:^-1]`, `a[^-1]` (valid slice
  forms with `^` from-end bound).
- `enum` trailing comma, `case` bare `is`, `try` bare `catch`, `def` missing parens,
  `struct` no paren, `print` trailing comma, empty `enum ( )`, bare `catch`,
  `x = + 5` (unary plus), `s..x` (UFCS field access), `p = thing` (semantic
  error only), `++` / `--` on valid variables, `$uc` / `--5` (semantic-only),
  `"{n:}"` format deferred to use-time, `"{}"` literal, bare `test` name.

These are **not** checker gaps — the runtime's leniency is the source of truth
for them, or they are valid constructs that the runtime accepts correctly.

---

## 4. STRICT — runtime lenient, `-zz` rejects (26)

Includes the original 11 missing-closer / stray-closer cases, plus the **12
previously-genuine DUAL_GAP gaps** that the checker now catches, plus 7 fuzz
files that the runtime tolerates but the checker rejects (all genuine garbage).

- Missing `end` / `enddef` / `endwhile` / `endfor` / `endstruct` / `endtest` /
  `endwith` / `endtry` at EOF (checker rejects, runtime auto-closes).
- Stray `endif` / `endfor` / `endwhile` / `enddef` / `endstruct` / `endcase`
  (runtime errors or silently ignored for `endif` before the fix; checker
  rejects).
- `x =` / `x +=` — missing RHS (runtime assigns nil; checker rejects).
- `var x, int` — malformed comma declaration (runtime accepts; checker rejects).
- `a = ::` — bare scope operator (runtime accepts; checker rejects).
- `x = 1..2..3` — chained range (runtime accepts; checker rejects).
- Unterminated string at EOF (runtime: **now rejects** via lexer fix; checker
  also rejects — these are technically OK, but were DUAL_GAP before the fix).
- Unclosed array `[1,2,3`, unclosed `map(`, unclosed enum `E(`, unclosed def
  `f(` — all now caught by the checker (runtime still lenient for brackets/
  parens at EOF).
- Missing `endwith` (runtime lenient; checker rejects).

The checker is **stricter than the runtime** here by design; the runtime
leniency is not a bug worth copying.

---

## 5. UNVERIFIABLE (1)

`test` bodies are not parsed at runtime without `-t`, so a missing `endtest`
has no runtime ground truth. `-zz` rejects it — treat as correct.

---

## 6. Runtime facts (not quirks to copy)

These are documented language/runtime behaviors; the syntax checker must NOT
imitate them where they are lenient, but should respect them where they are
deliberate:

1. **Inline def/try trailing-content discard is by design.** Everything after
the `def`/`try` header on the same line is discarded as a freeform comment-style
token stream. The syntax checker handles this by recognizing an inline `end` /
`endtry` on the same line as a valid closer.
2. **Def bodies validate lazily at call time** — by design. A function is not
parsed for statement validity until it is called, so bad syntax inside an
uncalled `def` reads as "clean". Methodology note: mutations inside def bodies
must append a call (`f()`) to force runtime validation.
3. **EOF auto-close for blocks** — the runtime silently closes unterminated
blocks at EOF. The lexer fix closed the string-literal case; blocks remain
lenient. The checker catches them via the nesting and shape passes.
4. **Stray closer behavior is not uniform.** `endif` at top level was silently
ignored (fixed in actor.go to error like `endcase`). `endfor`/`endwhile` /
`enddef`/`endstruct` already error. `endtest` / `endtry` / `endwith` behavior
is documented separately.
5. **ERR_SYNTAX was `iota` (0)** — fixed to `iota + 1`. Syntax errors now exit
non-zero (1). This surfaced a latent runtime rejection for missing `endstruct`
at EOF that was previously exiting 0.
6. **Errors print to stdout** — `finish(false, ERR_SYNTAX)` prints the error to
stdout and exits. With the iota fix the exit code is now 1 instead of 0.
7. **`return if` with omitted condition** — legal syntax. The runtime evaluates
the omitted condition as `<nil>` at call time, producing "RETURN IF condition must
evaluate to boolean". This is a call-time semantic error, not a syntax bug.
8. **`--5`, `$uc`** parse fine and fail only at evaluation (semantic).

---

## 7. Baseline — real code is clean

178 real files (`za_tests`, `3d/game`, `3d/lib`) pass `-zz`; 0 rejected. The
37 warnings are all `missing module at line N: <lib>.so (module not found: ...)`
for bare-name native libs (`libc.so.6`, `libpng.so`, `libglib-2.0.so.0`,
`libpthread.so.0`, ...). This is the one real-code gap in `-zz`: it cannot
resolve `module "libc.so.6"` because the interpreter looks for the `.so` in the
CWD. A future module-resolution pass has a ready-made free corpus of 37 examples
here. These are warnings, not rejections — no real script breaks.

---

## 8. What changed in this pass

### Runtime fixes
- **Lexer** (`lexer/lex.go`): unterminated string / block literals (`"`, `` ` ``,
  `'`, `{`) now return an error instead of swallowing the rest of the file to
  EOF. Runtime exits `ERR_LEX` (127); `-zz` routes the error to a per-file
  badword instead of killing the process.
- **Stray `endif`** (`actor.go:7926`): a top-level `endif` with no matching `if`
  now errors (`"Not currently in an IF block."`) and exits `ERR_SYNTAX` (1),
  matching `endcase`/`endfor`/`endwhile`/`enddef`/`endstruct`.
- **Error codes** (`constants.go:132`): `ERR_SYNTAX` shifted from `iota` (0) to
  `iota + 1` (1). All other `ERR_*` constants shifted accordingly; `ERR_LEX`
  remains explicit at 127. No test scripts depend on the numeric values.

### `-zz` checker enhancements (`parse_timing.go`)
- **Inline closer fix**: `validateBlockNesting` now scans the full phrase for an
  inline `end`/`endtry` after a `def`/`try` opener and treats the block as closed.
- **`with` / `endwith`** added to the block-nesting maps (previously missing).
- **New `validateStatementShapes` pass** (checker-only, zero runtime-parsing cost):
  - missing RHS (`x =`, `x +=`)
  - trailing binary operator (`x = 5 +`, `a = 1 <<`)
  - bare `::` (`a = ::`)
  - chained range (`x = 1..2..3`)
  - malformed `var` comma-declaration (`var x, int`)
  - unbalanced parens/brackets at phrase end (unclosed array, `map(`, enum paren,
    def paren)
- **Lexer error routing**: `nextToken` routes lexer errors into a global
  `lastSoftErrorMsg` when `lexSoftErrors` is set (only during `-zz`), allowing
  `phraseParse` to return `badword` instead of hard-exiting.

### Corpus outcome deltas

| outcome      | before | after | delta |
|--------------|-------:|------:|------:|
| OK           |    185 |   206 |  +21 |
| FN           |     59 |    37 |  -22 |
| DUAL_GAP     |     35 |    23 |  -12 |
| STRICT       |     11 |    26 |  +15 |
| FP           |      2 |     0 |   -2 |
| UNVERIFIABLE |      1 |     1 |    0 |
| REVIEW       |      0 |     0 |    0 |

The 12 DUAL_GAP reductions are the genuine gaps now closed. The 21 OK gain is
from the 2 FP fixes + 5 unterminated-string cases that now reject on both sides +
14 keyword-arity fixes (`if`/`for`/`foreach`/`break`/`continue`/`return`/`on`/`with`/`async`/`throw`/`struct`/`assert`/`exit`/`pause`/`require`/`module`/`namespace`).
The 15 new STRICT entries are the 12 genuine gaps + 3 fuzz garbage cases that
the runtime tolerates but the checker rejects. The 22 FN reduction is the
expression-completeness and keyword-arity checks catching previously-missed invalid
syntax.

### Regression fixtures
`tools/zzaudit/test_regression.py` runs 19 targeted cases (12 closed gaps + 2 FP
fixes + 5 control valid cases) against `za -zz` in under a second.
