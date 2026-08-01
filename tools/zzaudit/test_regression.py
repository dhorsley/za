#!/usr/bin/env python3
"""
Regression test for the 12 genuine gaps closed in this pass, plus the 2 FP fixes.
Run after any future change to confirm the syntax checker still catches them.
"""
import json, os, subprocess, sys, tempfile

ZA = os.environ.get("ZA", os.path.expanduser("~/go/src/za/za"))

def run_zz(src: str) -> dict:
    with tempfile.NamedTemporaryFile(mode="w", suffix=".za", delete=False) as f:
        f.write(src)
        path = f.name
    try:
        p = subprocess.run([ZA, "-S", "-zz", "-f", path],
                           capture_output=True, text=True, timeout=30)
        txt = (p.stdout + p.stderr).strip()
        try:
            data = json.loads(txt.splitlines()[-1])
            for e in data.get("files", []):
                if os.path.realpath(e.get("path", "")) == os.path.realpath(path):
                    return {"status": e.get("status"), "error": e.get("error", "")}
        except Exception:
            pass
        return {"status": "unparsed", "error": txt[:200]}
    finally:
        os.unlink(path)

# fmt: off
CASES = [
    # (name, source, expected_status)
    # 2 FP fixes
    ("inline_def_end",       "def f() end\n",                          "ok"),
    ("inline_try_endtry",    "try endtry\n",                           "ok"),
    # 4 statement-shape genuine gaps
    ("missing_rhs",          "x =\n",                                  "error"),
    ("compound_missing_rhs", "x +=\n",                                 "error"),
    ("var_comma_tail",       "var x, int\n",                           "error"),
    ("bare_double_colon",    "a = ::\n",                               "error"),
    ("chained_range",        "x = 1..2..3\n",                          "error"),
    # 7 EOF-leniency gaps
    ("unterminated_string",  's = "abc\n',                             "error"),
    ("unclosed_array",       "a = [1,2,3\n",                           "error"),
    ("unclosed_map",         "m = map(\n",                             "error"),
    ("unclosed_enum",        "enum E(\n",                             "error"),
    ("unclosed_def_paren",   "def f(\n",                              "error"),
    ("missing_endwith",      "with enum x\n  y = 1\n",                 "error"),
    # stray endif (runtime fix; -zz already caught it)
    ("stray_endif",          "endif\n",                                "error"),
    # control: valid code must stay ok
    ("valid_use_dash",       "use -\n",                                "ok"),
    ("valid_while_true",     "while\n  break\nendwhile\n",               "ok"),
    ("valid_for_comma",      "for ,,\n  break\nendfor\n",               "ok"),
    ("valid_print_bare",     "print\n",                                "ok"),
    ("valid_return_if",      "def f()\n  return if\nend\n",             "ok"),
]
# fmt: on

def main():
    failed = 0
    for name, src, expected in CASES:
        result = run_zz(src)
        status = result.get("status")
        if status != expected:
            print(f"FAIL  {name}: expected {expected}, got {status} ({result.get('error')})")
            failed += 1
        else:
            print(f"PASS  {name}")
    print(f"\n{failed}/{len(CASES)} failures")
    return 1 if failed else 0

if __name__ == "__main__":
    sys.exit(main())
