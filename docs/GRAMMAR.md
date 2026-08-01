# Za Syntax Reference (de facto EBNF)

Version: za 1.3.0
Source: `docs/handbook/handbook.md`, `za_tests/**`, interpreter sources (`lexer/`, `phraser.go`, `actor.go`), and empirical `za -e` probes.

This is a **syntax-level** reference. It describes the surface grammar the interpreter
accepts — it does not describe type rules, scoping semantics, or runtime behaviour.
It is written as readable EBNF prose so it can be reused (e.g. to drive a syntax checker,
editor highlighting, or documentation) independent of any particular tooling.

## Notation

```
A ::= B C      sequence
A ::= B | C    choice
[ B ]          optional
B*             zero or more
B+             one or more
"kw"           literal keyword / terminal
<x>            terminal class described in words
-- comment
```

Terminals are case-insensitive keywords unless quoted as identifiers. Whitespace is
insignificant between tokens. Newlines are significant only where a statement boundary
matters; Za otherwise treats end-of-line as a statement terminator.

Note: Za blocks are **keyword-delimited**, never brace-delimited (`if ... endif`,
`def ... end`, `for ... endfor`, ...). Literal curly braces appear in Za only inside
string interpolation (`{name}`, `{=expr}`, `{name:fmt}`) and shell substitution
(`${...}`).

## 1. Lexical structure

```
comment        ::= "#" <any text to end of line>
identifier     ::= <letter|"_"> (<letter|digit|"_">)*
keyword        ::= "var" | "if" | "endif" | ...   -- see statement keyword list (§3.3)
```

### 1.1 String literals

```
string_literal ::= `"` <char | escape | interpolation>* `"`
                 | "`" <char | escape | interpolation>* "`"
escape         ::= "\" <any char>          -- C-like sprintf escapes
interpolation  ::= "{" identifier "}"
                 | "{" identifier ":" <format> "}"
                 | "{=" expression "}"
                 | "{=" expression ":" <format> "}"
```

Double-quoted and backtick strings may span multiple source lines. Backtick strings avoid
escaping inner double quotes. Interpolation may be disabled via policy controls.

Unterminated strings (missing closing quote) are a syntax error.

### 1.2 Numeric literals

```
int_literal     ::= [ "0x" | "0o" | "0b" ] <digits>
float_literal   ::= <digits> "." <digits>
                  | <digits> "f"           -- float64 suffix
                  | <digits> "h"           -- float32 suffix
bigi_literal    ::= <digits> "n"
bigf_literal    ::= <digits> "." <digits> "n"
```

Literal type is determined by suffix/decimal point: plain digits → int; `.` or `f` →
float; `h` → float32; `n` → bigi; `n` with `.` → bigf.

### 1.3 Built-in constants

```
builtin_const   ::= "true" | "false" | "nil" | "NaN"
```

## 2. Expressions

```
expression       ::= ternary_expr
ternary_expr     ::= or_expr [ "?" or_expr ":" or_expr ]   -- `?` / `:` ternary
or_expr          ::= and_expr ( ( "or" | "||" | "|" ) and_expr )*   -- `|` also set-union on maps
and_expr         ::= not_expr ( ( "and" | "&&" | "&" ) not_expr )*  -- `&` also set-intersection on maps
not_expr         ::= [ "not" | "!" ] comparison
comparison       ::= additive ( ( "==" | "!=" | "<" | "<=" | ">" | ">=" | "~" | "~i" ) additive )*
additive         ::= multiplicative ( ( "+" | "-" | "|" ) multiplicative )*
multiplicative   ::= unary ( ( "*" | "/" | "%" ) unary )*
unary            ::= unary_op unary | postfix
unary_op         ::= "$pa" | "$pp" | "$pb" | "$pn" | "$pe"    -- path ops
                   | "$uc" | "$lc" | "$st" | "$lt" | "$rt"    -- string ops
                   | "$in" | "$out"                            -- file read / write
                   | "-" | "+" | "&" | "@"                     -- negate / ref / global
                   | "?" "|" ...
postfix          ::= primary postfix_op*
postfix_op       ::= "." identifier            -- field / UFCS
                   | "::" identifier           -- namespace qualifier
                   | "[" index_expr "]"        -- index / slice / clamp (§2.3)
                   | "(" args ")"              -- call
                   | "++" | "--"
                   | "." ... 
primary          ::= literal | identifier | builtin_const | array_literal | map_literal
                   | "(" expression ")" | string_literal | shell_subst | expr_string
```

Note: mapping/filtering operators `->` and `?>` bind loosely (near assignment) so that
pipelines read naturally.

### 2.1 Literals

```
array_literal    ::= "[" [ expression ( "," expression )* ] "]"
map_literal      ::= "map(" [ map_pair ( "," map_pair )* ] ")"
map_pair         ::= "." identifier expression
struct_literal   ::= type_name "(" [ expression ( "," expression )* ] ")"
                   | type_name "(" [ map_pair ( "," map_pair )* ] ")"
anon_struct      ::= "anon(" [ map_pair ( "," map_pair )* ] ")"
```

### 2.2 Shell substitution

```
shell_subst      ::= "${" <shell command text> "}"
                   | "`" <expression string> "`"        -- backtick expr string
cmd_assign       ::= identifier "=|" <shell command to EOL>
cmd_pipe         ::= "|" <shell command to EOL>
```

`=|` captures command output into the identifier; a leading `|` pipes (discards) output.

### 2.3 Indexing, slicing, and clamping

```
index_expr       ::= expression
                   | expression ":" expression          -- slice: both bounds
                   | ":" expression                     -- slice/clamp: upper bound
                   | expression ":"                     -- slice/clamp: lower bound
                   | ""                                 -- whole array (`a[]`)
```

On arrays this is indexing/slicing; on numeric values `[start:end]` is clamping.
An empty index `a[]` is the whole array, identical to `a` and `a[:]`. `^` is a
from-end bound usable inside slices: `a[:^]`, `a[1:^]`, `a[:^-1]` and single
index `a[^-1]` are valid (`a[^]` alone and arithmetic like `a[^2]` are not).

### 2.4 Expression strings

```
expr_string      ::= <double-quoted or backtick string used where a condition/callback is expected>
expr_sub         ::= "#"                                -- current element (filter/map/find/where)
                   | "$idx"                             -- current index/key
```

Used by `?>` (filter), `->` (map), and `find()`/`where()` conditions; `#` and `$idx`
substitute the current element and index respectively. `.field` and `.nested.field`
access map/struct fields inside the expression string.

## 3. Statements

```
program          ::= ( statement stmt_sep )* [ statement ]
statement        ::= assignment | var_decl | control_stmt | call_stmt
                   | shell_stmt | misc_stmt
stmt_sep         ::= newline | ";"
```

Statements are terminated by a newline **or** a semicolon (`;`). Block constructs are
multi-line: the opening keyword line, each body statement, and the closing keyword each
occupy their own line (indentation is conventional, not required). The following
productions show the canonical newline form; a `;` may replace any `stmt_sep`.

Two closers are also recognised directly after a statement with no separator:
`end` (closing `def`) and `endtry` (closing `try`). All other closers (`endif`,
`endfor`, `endwhile`, `endcase`, `endstruct`, `endwith`, `endtest`) must be preceded by
a `stmt_sep`.

### 3.1 Assignment

```
assignment       ::= target_list assign_op expression
target_list      ::= lvalue ( "," lvalue )*
lvalue           ::= [ "@" ] identifier            -- `@` marks global target inside a function
                   | lvalue "." identifier
                   | lvalue "[" expression "]"
assign_op        ::= "=" | "+=" | "-=" | "*=" | "/=" | "%="
```

Multiple targets unpack multiple return values: `a,b,c = f(...)`.

Implicit creation: assigning to an undeclared name creates the variable (dynamically
typed). Auto-vivification creates intermediate containers when assigning through an
access path.

### 3.2 `var` declarations

```
var_decl         ::= "var" name_list [ type_spec ] [ "=" expression ]
name_list        ::= identifier ( "," identifier )*
type_spec        ::= identifier                       -- scalar, struct, or namespaced type
                   | namespace "::" identifier
                   | array_type
array_type       ::= "[" [ "]" ] ( "[" [ "]" ] )* [ type_spec ]    -- fixed and/or dynamic dims
```

Validated forms (note: `var x = 5` is **not** valid — the type token is mandatory;
there is no type-inference form of `var`):

```
var z int
var x float32
var p Point
var c ea::struct_type
var cow,pig,sheep animal
var arr [1000] int
var grid [2][3]int
var matrix [][]int
var cube [][][]string
var result []int
```

Runtime usage message: `VAR varname1 [,...varnameX] [optional_size] type [=expression]`.
A bare `var xyz` (no type) fails at runtime with `unknown data type requested 'main::'`
but passes the syntax checker (`-zz`) — see tools/zzaudit/FINDINGS.md.

### 3.3 Statement keywords

Keywords begin a statement when they appear at statement position:

```
var, setglob, init, in, pause, help, nop, hist, debug, require, exit, version,
quiet, loud, unset, input, prompt, log, print, println, logging, cls, at, define,
showdef, enddef, return, async, lib, module, namespace, use, uses, while, endwhile,
for, foreach, endfor, continue, break, if, else, endif, case, is, contains, has,
or, endcase, with, endwith, struct, endstruct, showstruct, pane, doc, test, endtest,
assert, on, to, step, as, do, macro, enum, try, catch, then, throws, throw, endtry,
permit, trap, test, assert, doc, on
```

## 4. Control flow

### 4.1 Conditionals

```
if_stmt          ::= "if" expression stmt_sep
                     ( statement stmt_sep )*
                     ( "else" "if" expression stmt_sep ( statement stmt_sep )* )*
                     [ "else" stmt_sep ( statement stmt_sep )* ]
                     "endif" [ stmt_sep ]
on_do            ::= "on" expression "do" statement
 stmt_modifier    ::= ( "break" | "continue" | "return" [ expr_list ] ) "if" [ expression ]
```


`else if` chains attach without nested `endif`. `on … do` executes exactly one
statement when the condition is true. Statement-modifier conditionals apply to
`break`, `continue`, and `return` only. The condition is optional for
`return` (bare `return if` and `return 5 if` are legal); `break if` and
`continue if` require it.

### 4.2 Loops

```
for_to           ::= "for" assignment "to" expression [ "step" expression ] stmt_sep
                     ( statement stmt_sep )* "endfor" [ stmt_sep ]
for_c            ::= "for" [ assignment ] "," [ expression ] "," [ update ] stmt_sep
                     ( statement stmt_sep )* "endfor" [ stmt_sep ]
update           ::= expression (typically "i++" | "i--" | "i+=n")
foreach          ::= "foreach" identifier "in" expression stmt_sep
                     ( statement stmt_sep )* "endfor" [ stmt_sep ]
while            ::= "while" [ expression ] stmt_sep
                     ( statement stmt_sep )* "endwhile" [ stmt_sep ]
break            ::= "break" [ loop_type ]          -- optional construct type
continue         ::= "continue" [ loop_type ]
loop_type        ::= identifier
```
```

A `while` condition is optional and defaults to `true` (`while` alone is an
infinite loop). The C-form `for` header terms are all optional, so `for ,,`
is legal.

### 4.3 `case` (switch / pattern match)

```
case_stmt        ::= "case" [ expression ] [ "full" ] stmt_sep
                     ( case_clause stmt_sep ( statement stmt_sep )* )*
                     [ "or" stmt_sep ( statement stmt_sep )* ]
                     "endcase" [ stmt_sep ]
case_clause      ::= "is" expression            -- equality/pattern (enum member)
                   | "has" expression           -- condition on payload
                   | "contains" string_literal  -- regex match
```

`full` (used with enum values) requires exhaustive coverage of all enum members.

### 4.4 `with`

```
with_stmt        ::= "with" ( "enum" | "struct" ) identifier stmt_sep
                     ( statement stmt_sep )* "endwith" [ stmt_sep ]
```

`with enum <type>` opens an enum-matching block; the body is normally a `case` block
closed by `endcase` before the outer `endwith`. `with struct <type>` opens a struct-
scoped block.

## 5. Functions

```
def_stmt         ::= "def" identifier "(" [ param_list ] ")" [ stmt_sep ]
                     ( statement [ stmt_sep ] )*    -- "end" may follow directly
                     "end" [ stmt_sep ]
param_list       ::= identifier ( "," identifier )*
return           ::= "return" [ expr_list ]
expr_list        ::= expression ( "," expression )*
                   | "[" expression ( "," expression )* "]"   -- packed multi-return
async_call       ::= "async" identifier call_expr [ identifier ]
call_expr        ::= [ namespace "::" ] identifier "(" [ args ] ")"   -- function call
```

Functions may return multiple comma-separated values; callers unpack them with
multi-target assignment. `return if condition` (and `return value if condition`,
or with the condition omitted) are statement-modifier forms. `end` closes a
`def`. Struct-associated functions use `self` inside the body to refer to the
current instance.

Note: the function body begins on the **next line**. Any tokens after the `def`
header on the same line are discarded as freeform trailing content, so `def f() end`
(and `def f() print(1) end`) are accepted with an empty body.

## 6. Structs and enums

```
struct_stmt      ::= "struct" identifier stmt_sep
                     ( struct_field stmt_sep )* "endstruct" [ stmt_sep ]
struct_field     ::= identifier type_spec [ "=" expression ]     -- optional default
enum_stmt        ::= "enum" identifier "(" enum_member ( "," enum_member )* ")"
enum_member      ::= identifier [ "=" expression ]
```

Struct fields default to a declared default value or zero-like value. Enum members
auto-increment unless given an explicit `= value`. `ex` is the predefined exception
enum.

## 7. Modules, namespaces, and C interop

```
module_stmt      ::= "module" string_literal [ "as" identifier ]
use_stmt         ::= "use" ("-" | "+" | "^") identifier
                   | "use" "push" | "use" "pop"
namespace_stmt   ::= "namespace" identifier          -- current namespace
lib_stmt         ::= "lib" lib_decl
lib_decl         ::= identifier "::" identifier "(" [ c_params ] ")" [ "->" c_type ]
c_params         ::= identifier ":" c_type ( "," identifier ":" c_type )*
c_type           ::= "int" | "uint" | "float" | "double" | "string" | "pointer" | ...
qualified_ref    ::= identifier "::" identifier      -- namespaced value/type/function
std_bypass       ::= "std::" identifier              -- force stdlib resolution
```

`module` imports a script module; `as` gives an alias. `use` manages the namespace
resolution chain (`-` clears/removes, `+` adds, `^` pushes to top, `push`/`pop` stack
the chain). `lib` declares a C library symbol for FFI. `std::` forces stdlib lookup,
bypassing the USE chain.

## 8. Errors and exceptions

```
try_stmt         ::= "try" [ "throws" category ] [ stmt_sep ]
                     ( statement [ stmt_sep ] )*          -- "endtry" may follow directly
                     ( catch_clause ( statement [ stmt_sep ] )* )*
                     [ "then" [ stmt_sep ] ( statement [ stmt_sep ] )* ]
                     "endtry" [ stmt_sep ]
catch_clause     ::= "catch" [ identifier ] [ catch_pred ] [ stmt_sep ]
catch_pred       ::= "is" expression
                   | "in" expression
                   | "contains" string_literal
throw            ::= "throw" expression
trap_stmt        ::= "trap" [ "on" | "off" ] [ expression ]   -- error trap registration
```

`then` is the cleanup section that runs regardless of exception. `error_*` functions
introspect error context inside a handler.

Note: like `def`, the `try` body begins on the **next line**; any tokens after the
`try` header on the same line are discarded as freeform trailing content, so
`try endtry` is accepted with an empty body.

## 9. Testing

```
test_stmt        ::= "test" string_literal [ "group" string_literal ]
                     [ "assert" ( "fail" | "continue" ) ] stmt_sep
                     ( statement stmt_sep )* "endtest" [ stmt_sep ]
assert           ::= "assert" expression [ "," string_literal ]
doc_stmt         ::= "doc" string_literal
```

Test blocks are ignored during normal script execution and run under `za -t`.

## 10. Misc statements

```
print_family     ::= ( "print" | "println" ) [ expression ( "," expression )* ]
                   | "printf" format_args
output_file      ::= expression "$out" string_literal    -- infix write
input_file       ::= "$in" string_literal                -- read file (unary)
require          ::= "require" identifier                -- require an executable
exit_stmt        ::= "exit" [ expression ]
unset            ::= "unset" identifier
pause            ::= "pause" [ expression ]
permit           ::= "permit" "on" | "permit" "off" [ permit_items ]
log_stmt         ::= "log" ...                            -- logging family
at_stmt          ::= "at" ...
```

`$out` is an infix operator: `content $out "/path"` writes to a file. `$in` reads a
file: `content = $in "/path"`.

The `print`/`println` argument list is optional: bare `print` prints nothing
outside the REPL (a bare line in the REPL), and bare `println` prints just a
newline.

## 11. Operators (terminal inventory)

Arithmetic: `+ - * / % **`
Assignment: `= += -= *= /= %=`
Comparison: `== != < <= > >= ~ ~i`
Boolean: `and or not && || !`
Bitwise: `& | ^ << >>` (also set ops on maps)
Unary: `$pa $pp $pb $pn $pe $uc $lc $st $lt $rt $in $out - + & @`
Range: `..`
Increment: `++ --`
Mapping/filtering: `-> ?>`
Shell: `|= | ${...}`
Punctuation: `( ) [ ] { } , : :: . ;`
Global: `@`
Path: `$pa $pp $pb $pn $pe`

## 12. Statement-layer gaps in `-zz`

The static syntax checker (`za -zz`) performs lexical analysis, phrase/block nesting
validation, and static module resolution. It does **not** compile expressions or
validate statement shapes. Consequences (detailed in tools/zzaudit/FINDINGS.md):

- Missing statement keywords/terminators (e.g. `var xyz` with no type) are not caught.
- Bare-name C module declarations that resolve only at runtime are flagged even though
  valid (false positives).
- Statement bodies are never type-checked; only nesting and lexical structure matter.
