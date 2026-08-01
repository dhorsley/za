#!/usr/bin/env python3
"""zzaudit generate.py

Generates the zzaudit corpus from the Za grammar (see docs/GRAMMAR.md):

  corpus/valid/*.za    - self-contained programs that are known-valid and run clean
  corpus/invalid/*.za  - deliberate syntactic mutations of each grammar rule
  corpus/fuzz/*.za     - random token mutations / token soup over the valid templates
  corpus/static/*.za   - valid statements that are NOT executed (module/use/lib/shell/
                         require/exit/pause/input) - -zz only
  corpus/manifest.json - metadata for every generated file

Self-check: `generate.py --selfcheck` runs every valid template through the sandboxed
`za -e` probe and reports any template that does not run clean (rc==0, no error text),
so generator bugs are caught before classification.

Run:  python3 generate.py [--seed N] [--fuzz N] [--selfcheck]
"""
import argparse
import hashlib
import json
import os
import random
import re
import subprocess
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
CORPUS = os.path.join(HERE, "corpus")
ZA = os.environ.get("ZA", os.path.expanduser("~/go/src/za/za"))
WORK = os.environ.get("ZZAUDIT_WORK", "/tmp/opencode/zzaudit-work")

# --------------------------------------------------------------------------
# Grammar rule table.  Each rule: name -> (valid programs, invalid mutations)
# where an invalid mutation is (label, program).  Programs are the literal text
# written to a .za file.  Every valid program is self-contained and pure:
# it defines every identifier it uses and performs no filesystem / network /
# process side effects.
# --------------------------------------------------------------------------

RULES = [
    ("assign_scalar", [
        "x = 1\ny = x + 2\nz = y * 3\n",
        "name = \"hello\"\ntext = name + \" world\"\n",
        "a, b = 5, 10\ntotal = a + b\n",
    ], [
        ("no_rhs", "x =\n"),
        ("trailing_op", "x = 5 +\n"),
        ("double_assign", "x = 5 ==\n"),
        ("leading_op", "x = + 5\n"),
    ]),
    ("assign_op", [
        "x = 10\nx += 5\nx -= 2\nx *= 3\nx /= 2\nx %= 4\n",
    ], [
        ("bare_plus_eq", "x = 1\nx +=\n"),
        ("double_plus", "x = 1\nx ++= 2\n"),
    ]),
    ("assign_multi_target", [
        "def pair()\n  return 1, 2\nend\na, b = pair()\n",
        "def trio()\n  return [3, 4, 5]\nend\na, b, c = trio()\n",
    ], [
        ("multi_no_targets", "def pair()\n  return 1, 2\nend\n, = pair()\n"),
    ]),
    ("assign_global", [
        "g = 0\ndef bump()\n  @g = 5\nend\nbump()\n",
    ], [
        ("bare_at", "@ = 5\n"),
        ("double_at", "@@x = 5\n"),
    ]),
    ("var_scalar", [
        "var x int\nx = 10\n",
        "var x int = 10\n",
        "var y float32\nvar z string\nvar b bool\n",
        "var x, y int\n",
        "struct person\n  name string\nendstruct\nvar p person\np.name = \"x\"\n",
        "struct thing\n  x int\nendstruct\nvar c thing\n",
        "var m [100] int\n",
        "var grid [2][3]int\n",
        "var matrix [][]int\n",
        "var cube [][][]string\n",
        "var s []int\n",
    ], [
        ("missing_type", "var xyz\n"),
        ("assign_form", "var x = 5\n"),
        ("no_rhs_typed", "var x int =\n"),
        ("no_rhs_no_type", "var x\n"),
        ("unclosed_array", "var arr [3\n"),
        ("comma_tail", "var x, int\n"),
    ]),
    ("struct", [
        "struct person\n  name string\n  age int\nendstruct\nvar p person\np.name = \"Billy\"\n",
        "struct thing\n  x int = 5\n  y string = \"d\"\nendstruct\n",
    ], [
        ("missing_endstruct", "struct person\n  name string\n"),
        ("stray_field", "struct person\n  name string = = 5\nendstruct\n"),
        ("no_identifier", "struct\n  name string\nendstruct\n"),
    ]),
    ("enum", [
        "enum color ( red, green, blue )\nx = color.red\n",
        "enum status ( idle = 0, running = 5, done )\n",
    ], [
        ("unclosed_paren", "enum color ( red, green\n"),
        ("no_members", "enum color ( )\n"),
        ("trailing_comma", "enum color ( red, )\n"),
    ]),
    ("if", [
        "x = 1\nif x > 0\n  y = 2\nendif\n",
        "x = 5\nif x == 1\n  y = 1\nelse if x == 2\n  y = 2\nelse\n  y = 9\nendif\n",
        "x = 3\nif x == 1; y = 1; else if x == 3; y = 3; else; y = 9; endif\n",
    ], [
        ("missing_endif", "x = 1\nif x > 0\n  y = 2\n"),
        ("inline_endif", "x = 1\nif x > 0 endif\n"),
        ("no_condition", "x = 1\nif\n  y = 2\nendif\n"),
        ("bare_else", "x = 1\nif x > 0\n  y = 2\nelse\n"),
        ("stray_endif", "endif\n"),
        ("mismatched_closer", "x = 1\nif x > 0\n  y = 2\nendwhile\n"),
    ]),
    ("on_do", [
        "x = 6\non x > 5 do y = 1\n",
    ], [
        ("no_do", "x = 6\non x > 5\n"),
        ("bare_on", "on\n"),
        ("no_condition", "on do\n"),
    ]),
    ("stmt_modifier", [
        "for i=0 to 3\n  break if i == 2\nendfor\n",
        "for i=0 to 3\n  continue if i == 1\nendfor\n",
        "def f(x)\n  return 5 if x > 0\n  return x\nend\nf(1)\n",
        "def f(x)\n  return 1, 2 if x > 0\n  return x\nend\nf(1)\n",
    ], [
        ("break_no_cond", "for i=0 to 3\n  break if\nendfor\n"),
        ("continue_no_cond", "for i=0 to 3\n  continue if\nendfor\n"),
        ("return_no_cond", "def f()\n  return if\nend\nf()\n"),
    ]),
    ("for_to", [
        "for i=0 to 3\n  x = i\nendfor\n",
        "for i=0 to 10 step 2\n  x = i\nendfor\n",
    ], [
        ("missing_endfor", "for i=0 to 3\n  x = i\n"),
        ("no_to", "for i=0 3\n  x = i\nendfor\n"),
        ("inline_endfor", "for i=0 to 1 endfor\n"),
    ]),
    ("for_c", [
        "for i=0, i<=3, i++\n  x = i\nendfor\n",
        "for i=0, i<3, i+=2\n  x = i\nendfor\n",
    ], [
        ("no_terms", "for ,, \n  x = i\nendfor\n"),
        ("missing_endfor", "for i=0, i<3, i++\n  x = i\n"),
    ]),
    ("foreach", [
        "items = [1, 2, 3]\nforeach item in items\n  x = item\nendfor\n",
    ], [
        ("missing_iterable", "foreach item in\n  x = 1\nendfor\n"),
        ("missing_endfor", "items = [1]\nforeach item in items\n  x = item\n"),
        ("no_in", "items = [1]\nforeach item items\n  x = item\nendfor\n"),
    ]),
    ("while", [
        "x = 0\nwhile x < 3\n  x += 1\nendwhile\n",
    ], [
        ("missing_endwhile", "x = 0\nwhile x < 3\n  x += 1\n"),
        ("inline_endwhile", "x = 0\nwhile x < 3 endwhile\n"),
        ("no_condition", "x = 0\nwhile\n  x += 1\nendwhile\n"),
    ]),
    ("case", [
        "x = 1\ncase x\nis 1\n  y = 10\nis 2\n  y = 20\nor\n  y = 99\nendcase\n",
        "x = \"hi\"\ncase x\ncontains \"h\"\n  y = 1\nor\n  y = 2\nendcase\n",
        "x = 5\ncase x\nhas x > 3\n  y = 1\nor\n  y = 2\nendcase\n",
    ], [
        ("missing_endcase", "x = 1\ncase x\nis 1\n  y = 10\n"),
        ("bare_is", "x = 1\ncase x\nis\n  y = 10\nendcase\n"),
        ("inline_endcase", "x = 1\ncase x endcase\n"),
    ]),
    ("with_enum", [
        "enum status ( idle, done )\ns = status.done\nwith enum status\n  case s full\n  is idle\n    y = 1\n  is done\n    y = 2\n  endcase\nendwith\n",
    ], [
        ("with_no_enum", "with p\n  y = 1\nendwith\n"),
        ("missing_endwith", "enum status ( idle, done )\nwith enum status\n  y = 1\n"),
    ]),
    ("def", [
        "def f(x)\n  return x*2\nend\nf(4)\n",
        "def f() end\n",
        "def f(a,b)\n  return b,a\nend\na, b = f(1,2)\n",
        "def noargs()\n  return\nend\nnoargs()\n",
    ], [
        ("missing_paren", "def f\n  return 1\nend\n"),
        ("missing_end", "def f(x)\n  return x\n"),
        ("unclosed_paren", "def f(x\n  return x\nend\n"),
        ("stray_end", "end\n"),
        ("bare_end_in_def", "def f()\nend\nend\n"),
    ]),
    ("return_multi", [
        "def f(a,b,c)\n  return b,c,a\nend\nx,y,z = f(1,2,3)\n",
        "def g(a,b)\n  return [b,a]\nend\nx,y = g(1,2)\n",
    ], [
        ("return_pack_mismatch", "def f()\n  return [1,2,3]\nend\nx,y = f()\n"),
    ]),
    ("async", [
        "def t()\n  return 1\nend\nasync h t()\n",
    ], [
        ("bare_async", "async\n"),
        ("async_no_call", "async 5\n"),
    ]),
    ("try_catch", [
        "try\n  x = 1\ncatch err\n  x = 2\nendtry\n",
        "try\n  throw \"boom\"\ncatch err\n  x = 2\nendtry\n",
        "try throws \"boom\"\n  x = 1\nthen\n  x = 2\nendtry\n",
        "try\n  x = 1\ncatch err is \"boom\"\n  x = 2\nendtry\n",
        "try endtry\n",
    ], [
        ("missing_endtry", "try\n  x = 1\ncatch err\n  x = 2\n"),
        ("stray_endtry", "endtry\n"),
        ("bare_catch", "try\n  x = 1\ncatch\nendtry\n"),
        ("no_try_body", "try\nendtry\n"),
    ]),
    ("throw", [
        "try\n  throw \"boom\"\ncatch err\n  x = 1\nendtry\n",
    ], [
        ("throw_no_expr", "try\n  throw\ncatch err\n  x = 1\nendtry\n"),
    ]),
    ("test", [
        "test \"t1\" GROUP \"g\"\n  assert 1 == 1, \"ok\"\n  doc \"d\"\nendtest\n",
    ], [
        ("missing_endtest", "test \"t1\" GROUP \"g\"\n  assert 1 == 1\n"),
        ("missing_quote", "test t1 GROUP g\n  assert 1 == 1\nendtest\n"),
    ]),
    ("string_literal", [
        "s = \"hello\"\nx = 1\ns2 = \"value: {x}\"\n",
        "t = `expr with \"quotes\"`\n",
        "n = 42\nf = \"{n:04d}\"\n",
        "e = \"{=1 + 2}\"\n",
    ], [
        ("unterminated", "s = \"abc\n"),
        ("empty_expr_string", "s = \"{}\"\n"),
        ("bad_fmt", "s = \"{n:}\"\n"),
    ]),
    ("numeric_literal", [
        "a = 10\nb = 10.5\nc = 0xFF\nd = 0o755\ne = 0b1010\nf = 42n\ng = 3.14h\n",
    ], [
        ("double_dot", "x = 1..2..3\n"),
        ("bare_dot", "x = .\n"),
    ]),
    ("map_literal", [
        "m = map(.a 1, .b 2)\n",
        "m = map(.name \"Alice\", .age 30)\n",
        "s = anon(.x 1, .y 2)\n",
    ], [
        ("map_no_key", "m = map(1, 2)\n"),
        ("map_unclosed", "m = map(.a 1\n"),
    ]),
    ("struct_literal", [
        "struct person\n  name string\n  age int\nendstruct\np = person(\"Bob\", 21)\n",
        "struct thing\n  x int\nendstruct\np = thing(.x 5)\n",
    ], [
        ("struct_no_paren", "struct thing\n  x int\nendstruct\np = thing\n"),
    ]),
    ("array_literal", [
        "a = [1, 2, 3]\n",
        "b = []\n",
        "c = [1, 2, 3]\nx = c[1]\n",
        "d = [[1, 2], [3, 4]]\n",
    ], [
        ("array_no_close", "a = [1, 2, 3\n"),
        ("double_comma", "a = [1,, 2]\n"),
    ]),
    ("index_slice_clamp", [
        "a = [1, 2, 3, 4]\nx = a[1:3]\n",
        "n = 5\nc = n[0:10]\n",
        "a = [1, 2, 3, 4]\nx = a[:2]\n",
    ], [
        ("slice_no_bounds", "a = [1, 2, 3]\nx = a[:]\n"),
        ("empty_index", "a = [1, 2, 3]\nx = a[]\n"),
    ]),
    ("operators", [
        "a = 2 ** 3\n",
        "a = 10 / 3\n",
        "a = 17 % 5\n",
        "a = 1..10\n",
        "a = 5 < 6 and 6 < 7\n",
        "a = 5 > 4 or 3 > 4\n",
        "a = 1 == 1 and 2 != 1\n",
        "a = 0b101 & 0b011\n",
        "a = 0b101 | 0b010\n",
        "a = 1 << 2\n",
        "a = 4 >> 1\n",
        "a = 5 ^ 3\n",
        "a = not false\n",
        "a = 1 == 1 && 2 == 2\n",
        "a = 1 == 2 || 2 == 2\n",
        "s = \"abc\"\nu = $uc s\n",
        "s = \"  x  \"\nt = $st s\n",
        "p = \"/tmp/a/b.txt\"\nn = $pn p\n",
        "m = map(.a 1)\n",
        "x = \"hello\"\nif x ~ \"ell\"\n  y = 1\nendif\n",
        "x = \"HELLO\"\nif x ~i \"hello\"\n  y = 1\nendif\n",
    ], [
        ("trailing_and", "a = 1 and\n"),
        ("double_and_op", "a = 1 and and 2\n"),
        ("range_no_end", "a = 1..\n"),
        ("bare_double_colon", "a = ::\n"),
        ("ternary_no_then", "a = 1 ? 2\n"),
        ("ternary_no_else", "a = 1 ? 2 :\n"),
    ]),
    ("unary_ops", [
        "s = \"ab\"\nx = $lc s\n",
        "f = \"/tmp/x.txt\"\nb = $pb f\n",
        "n = 5\nx = -n\n",
        "n = 5\nx = +n\n",
    ], [
        ("bare_unary", "x = $uc\n"),
        ("double_minus", "x = --5\n"),
    ]),
    ("expr_string", [
        "a = [1, 2, 3, 4, 5]\nf = find(a, \"#>3\")\n",
        "a = [1, 2, 3, 4, 5]\nw = where(a, \"#%2==0\")\n",
        "a = [1, 2, 3]\nbad = a ?> \"#>2\"\n",
        "a = [1, 2, 3]\nnames = a -> \"#*2\"\n",
    ], [
        ("expr_string_no_hash", "a = [1, 2, 3]\nf = find(a, \">3\")\n"),
    ]),
    ("ufcs", [
        "s = \"123\"\nn = s.as_int\n",
        "s = \"a.b\"\nx = s.replace(\".\", \"-\")\n",
    ], [
        ("ufcs_no_field", "s = \"123\"\nn = s.\n"),
        ("ufcs_double_dot", "s = \"123\"\nn = s..x\n"),
    ]),
    ("print_family", [
        "print \"a\"\nprintln \"b\"\n",
        "x = 1\nprintln x\n",
    ], [
        ("print_no_arg", "print\n"),
        ("print_trailing_comma", "print \"a\",\n"),
    ]),
]

# --------------------------------------------------------------------------
# Fuzz pool: grammar tokens only.  Side-effecting keywords/operators and real
# stdlib identifiers are deliberately absent, so generated files can never
# reach out of the interpreter (see run.py --verify for the independent
# syscall-level check).
# --------------------------------------------------------------------------

FUZZ_TOKENS = [
    "var", "if", "else", "endif", "for", "to", "step", "endfor", "foreach",
    "in", "while", "endwhile", "case", "is", "has", "contains", "or", "endcase",
    "struct", "endstruct", "enum", "def", "end", "return", "try", "catch",
    "then", "throws", "throw", "endtry", "with", "endwith", "test", "endtest",
    "assert", "doc", "print", "println", "and", "not", "break", "continue",
    "full", "as", "self", "nil", "true", "false", "NaN",
    "=", "+=", "-=", "*=", "/=", "%=", "==", "!=", "<", "<=", ">", ">=",
    "+", "-", "*", "/", "%", "**", "++", "--", "..", "~", "~i", "&&", "||",
    "!", "&", "|", "^", "<<", ">>", ":", "::", ".", ",", "(", ")", "[", "]",
    "?", "@",
]
FUZZ_IDS = ["foo", "bar", "x", "y", "z", "n", "i", "items", "val", "nums",
            "s", "data", "result", "obj", "k", "v", "item", "a", "b", "total"]
FUZZ_LITERALS = ["0", "1", "2", "42", "3.14", "1000", "\"str\"", "\"hi\"", "\"x\""]
TOKEN_RE = re.compile(r"(?:==|!=|<=|>=|\*\*|->|\?>|\+=|-=|\*=|\/=|%=|&&|\|\||<<|>>|\.\.|::|\+\+|--|~i|[$][a-z][a-z]?|[()\[\]{},:;=+\-*/%<>&|^!?.@]|\"[^\"]*\"|`[^`]*`|\S+)")

def tokenize(prog):
    return TOKEN_RE.findall(prog)

def tokenize_keep_ws(prog):
    return re.findall(r"(\s+|==|!=|<=|>=|\*\*|->|\?>|\+=|-=|\*=|\/=|%=|&&|\|\||<<|>>|\.\.|::|\+\+|--|~i|[$][a-z][a-z]?|[()\[\]{},:;=+\-*/%<>&|^!?.@]|\"[^\"]*\"|`[^`]*`|\S+)", prog)

def detokenize(pieces):
    out = ""
    prev = ""
    for p in pieces:
        if p.strip():
            if prev.strip() and not p[0].isspace():
                no_space_before = prev[-1] in "([.,:+-/|&!=<>*"
                no_space_after = p[0] in ")].,;:"
                if not (no_space_before or no_space_after):
                    out += " "
        out += p
        prev = p
    return out

def mutate_delete(prog, rng):
    pieces = tokenize_keep_ws(prog)
    tok_idx = [i for i, p in enumerate(pieces) if p.strip()]
    if not tok_idx:
        return prog
    i = rng.choice(tok_idx)
    del pieces[i]
    return "".join(pieces)

def mutate_dup(prog, rng):
    pieces = tokenize_keep_ws(prog)
    tok_idx = [i for i, p in enumerate(pieces) if p.strip()]
    if not tok_idx:
        return prog
    i = rng.choice(tok_idx)
    pieces.insert(i, pieces[i])
    return "".join(pieces)

def mutate_swap(prog, rng):
    pieces = tokenize_keep_ws(prog)
    tok_idx = [i for i, p in enumerate(pieces) if p.strip()]
    if len(tok_idx) < 2:
        return prog
    i, j = rng.sample(tok_idx, 2)
    pieces[i], pieces[j] = pieces[j], pieces[i]
    return "".join(pieces)

def mutate_insert(prog, rng):
    pieces = tokenize_keep_ws(prog)
    tok_idx = [i for i, p in enumerate(pieces) if p.strip()]
    pool = FUZZ_TOKENS + FUZZ_IDS + FUZZ_LITERALS
    if not tok_idx:
        return prog
    i = rng.choice(tok_idx + [len(pieces)])
    pieces.insert(i, rng.choice(pool))
    return "".join(pieces)

def token_soup(rng):
    pool = FUZZ_TOKENS + FUZZ_IDS + FUZZ_LITERALS
    n = rng.randint(3, 8)
    toks = [rng.choice(pool) for _ in range(n)]
    return detokenize(toks) + "\n"

def fuzz_mutate(prog, rng):
    fns = [mutate_delete, mutate_dup, mutate_swap, mutate_insert]
    fn = rng.choice(fns)
    try:
        return fn(prog, rng)
    except Exception:
        return prog

# --------------------------------------------------------------------------

def sha(s):
    return hashlib.sha1(s.encode()).hexdigest()[:8]

def write_corpus(fuzz_n=90, seed=7):
    if os.path.exists(CORPUS):
        import shutil
        shutil.rmtree(CORPUS)
    for d in ("valid", "invalid", "fuzz", "static"):
        os.makedirs(os.path.join(CORPUS, d))
    manifest = []
    rng = random.Random(seed)

    valid_seeds = []
    for rule, valids, invalids in RULES:
        for vi, prog in enumerate(valids):
            path = os.path.join(CORPUS, "valid", f"{rule}_{vi}.za")
            with open(path, "w") as f:
                f.write(prog)
            manifest.append({"path": path, "kind": "valid", "rule": rule, "mut": ""})
            valid_seeds.append(prog)
        for label, prog in invalids:
            path = os.path.join(CORPUS, "invalid", f"{rule}__{label}.za")
            with open(path, "w") as f:
                f.write(prog)
            manifest.append({"path": path, "kind": "invalid", "rule": rule, "mut": label})

    for fi in range(fuzz_n):
        seed_prog = rng.choice(valid_seeds)
        if fi % 5 == 4:
            prog = token_soup(rng)
            label = "soup"
        else:
            prog = fuzz_mutate(seed_prog, rng)
            label = "mut"
        path = os.path.join(CORPUS, "fuzz", f"fuzz_{fi:04d}.za")
        with open(path, "w") as f:
            f.write(prog)
        manifest.append({"path": path, "kind": "fuzz", "rule": "", "mut": label})

    static_files = [
        ("static_module", "module \"util\"\n"),
        ("static_module_as", "module \"util\" as u\n"),
        ("static_use", "use +tm\n"),
        ("static_use_clear", "use -\n"),
        ("static_lib", "lib libc::malloc(size:int) -> pointer\n"),
        ("static_namespace", "namespace myns\n"),
        ("static_require", "require ls\n"),
        ("static_shell_pipe", "| ls\n"),
        ("static_shell_assign", "files =| ls\n"),
        ("static_permit", "permit off\n"),
    ]
    for name, prog in static_files:
        path = os.path.join(CORPUS, "static", f"{name}.za")
        with open(path, "w") as f:
            f.write(prog)
        manifest.append({"path": path, "kind": "static", "rule": "", "mut": name})

    with open(os.path.join(CORPUS, "manifest.json"), "w") as f:
        json.dump(manifest, f, indent=1)
    counts = {}
    for m in manifest:
        counts[m["kind"]] = counts.get(m["kind"], 0) + 1
    print("generated:", counts, "total", len(manifest))
    return manifest

def sandbox_probe(prog, timeout=10):
    """Run prog under sandboxed `za -e`. Returns (rc, stdout, stderr)."""
    os.makedirs(WORK, exist_ok=True)
    cmd = ["timeout", str(timeout), "bwrap",
           "--unshare-all", "--die-with-parent", "--new-session",
           "--ro-bind", "/", "/",
           "--proc", "/proc", "--dev", "/dev",
           "--tmpfs", "/tmp", "--tmpfs", "/run", "--tmpfs", "/var/tmp",
           "--tmpfs", "/var/log",
           "--bind", WORK, WORK,
           "--chdir", WORK,
           "--setenv", "HOME", WORK,
           "--", ZA, "-e", prog]
    p = subprocess.run(cmd, input="", capture_output=True, text=True, timeout=timeout + 5)
    return p.returncode, p.stdout, p.stderr

def selfcheck():
    bad = 0
    for rule, valids, _invalids in RULES:
        for vi, prog in enumerate(valids):
            rc, out, err = sandbox_probe(prog)
            blob = out + err
            if rc != 0 or "Error in" in blob or "runtime error:" in blob:
                bad += 1
                print(f"SELFCHECK FAIL [{rule}_{vi}] rc={rc}")
                print("  prog:", prog.replace("\n", "\\n"))
                print("  out:", blob.strip().replace("\n", " | ")[:200])
    if bad:
        print(f"selfcheck: {bad} template(s) do not run clean")
        return 1
    print("selfcheck: all valid templates run clean (rc=0, no error text)")
    return 0

if __name__ == "__main__":
    ap = argparse.ArgumentParser()
    ap.add_argument("--seed", type=int, default=7)
    ap.add_argument("--fuzz", type=int, default=90)
    ap.add_argument("--selfcheck", action="store_true")
    args = ap.parse_args()
    write_corpus(fuzz_n=args.fuzz, seed=args.seed)
    if args.selfcheck:
        sys.exit(selfcheck())
