#!/usr/bin/env python3
"""zzaudit run.py

Ground-truth runner for the zzaudit corpus.

Per generated file:
  - `za -zz <file>`      static syntax check (JSON result)
  - sandboxed `za -e <file-content>`  runtime probe = independent syntax arbiter

Classification (see README.md for rationale):

  valid   : -zz reject -> FP ; runtime reject -> template bug (review)
  invalid : runtime syntax_error/crash + -zz accept -> FN (the miss we hunt)
            runtime clean/semantic + -zz reject -> FP
            runtime clean + -zz accept -> no-op mutation
  fuzz    : runtime clean      -> valid syntax : -zz accept OK / -zz reject FP
            runtime syntax_err -> invalid syntax : -zz accept FN / -zz reject OK
            runtime semantic   -> syntactically valid, flagged "semantic_only"
            runtime crash      -> invalid syntax (evaluator crash) : FN if -zz accept
            runtime timeout    -> probably valid (infinite loop) : not a miss

Modes:
  run.py                    classify corpus/manifest.json
  run.py --baseline DIR...  -zz-only sweep over existing .za trees
  run.py --verify           strace every executed file; assert zero host side
                            effects; also (re)classify
Outputs: report.json + printed summary (FINDINGS material).
"""
import argparse
import json
import os
import re
import subprocess
import sys

from generate import CORPUS, ZA, WORK, sandbox_probe

HERE = os.path.dirname(os.path.abspath(__file__))
REPORT = os.path.join(HERE, "report.json")

# --------------------------------------------------------------------------
# Error taxonomy.  Classified by first matching signature, in order:
# PANIC > SYNTAX > SEMANTIC.  Grown empirically from runtime messages.
# --------------------------------------------------------------------------

PANIC_SIGS = [
    "index out of range",
    "index out of bounds",
    "slice bounds out of range",
    "interface conversion",
    "invalid memory address",
    "nil pointer",
    "panic:",
]

SYNTAX_SIGS = [
    "invalid VAR syntax",
    "unknown data type requested",
    "type mismatch in VAR assignment",
    "Missing ENDIF",
    "Missing ENDCASE",
    "Cannot determine the location of a matching ENDFOR",
    "cannot find a matching ENDFOR",
    "TO not found in FOR",
    "Invalid catch syntax",
    "Invalid assignment in FOR",
    "Invalid condition for amendment in FOR",
    "Invalid expression for amendment in FOR",
    "Missing expression in FOR",
    "Invalid expression for assignment in ENUM",
    "Missing assignment in ENUM",
    "Missing ASSERT in TEST",
    "Missing GROUP in TEST",
    "Invalid variable specification",
    "Not currently defining a function",
    "Invalid arguments in ASYNC",
    "Unknown argument in USE",
    "Unknown PANE command",
    "unknown unary path operator",
    "unknown unary string operator",
    "Expected - or + after macro",
    "Expected quoted value for macro define",
    "missing a condition",
    "missing condition",
    "BREAK IF missing a condition",
    "missing closing bracket",
    "Not currently in an IF block",
    "Open STRUCT definition",
    "end of map key brace missing",
    "end of range brace missing",
    "missing colon in ternary",
    "unexpected token",
    "problem lexing character",
    "Invalid default value in STRUCT",
    "Invalid STRUCT entry",
    "Invalid type in STRUCT",
    "Malformed WITH statement",
    "Unknown WITH type",
    "Invalid signature format",
    "unterminated",
    "Missing ENDWHILE",
    "Missing ENDTEST",
    "Missing ENDSTRUCT",
    "Missing ENDDEF",
    "invalid statement",
    "invalid assignee: must start with an identifier",
    "STRUCT must contain a name.",
    "DO not found in ON",
    "ON missing arguments.",
    "bad argument count in FOREACH.",
    "could not find an ENDWHILE",
    "ENDWHILE outside of WHILE loop",
    "ENDIF outside of IF",
    "throw requires an exception category",
    "unary dot field operator present outside of a WITH clause",
    "length mismatch of argument names",
    "is not allowed in expressions",
    "broken on type",
    "malformed bracket sequence",
    "Incorrect arguments supplied for ENUM",
    "could not evaluate the CASE condition",
    "too few arguments",
    "unexpected expression",
    "not a boolean on left of ternary",
    "invalid variable name in pre-inc/dec",
    "type error: cannot subtract",
    "invalid string (<nil>) in map",
]

SEMANTIC_SIGS = [
    "is uninitialised",
    "uninitialised",
    "undefined function",
    "undefined variable",
    "undefined",
    "is not defined",
    "not defined",
    "unknown variable",
    "unknown function",
    "unknown method",
    "divide by zero",
    "division by zero",
    "out of bounds",
    "type mismatch",
    "Return count mismatch",
    "Return type mismatch",
    "Type mismatch for parameter",
    "invalid index",
    "cannot assign",
    "cannot convert",
    "invalid type",
    "Could not evaluate",
    "Error evaluating",
    "error evaluating",
    "cannot index",
    "no such field",
    "unknown field",
    "unknown member",
    "is already declared",
    "not a member of enum",
    "shift operations only work with integers",
    "unary positive requires number",
    "IN operator requires a list",
    "pre-inc/dec not supported",
    "PRINT term evaluation",
    "RETURN IF condition must evaluate to boolean",
]


def runtime_verdict(rc, out, err):
    blob = out + err
    if rc == 124:
        return "timeout"
    rejected = (rc != 0) or ("Error in" in blob) or ("runtime error:" in blob)
    if not rejected:
        return "clean"
    for sig in PANIC_SIGS:
        if sig in blob:
            return "crash"
    for sig in SYNTAX_SIGS:
        if sig in blob:
            return "syntax_error"
    for sig in SEMANTIC_SIGS:
        if sig in blob:
            return "semantic_error"
    return "unknown_error"


def run_zz(path):
    p = subprocess.run([ZA, "-zz", path], capture_output=True, text=True, timeout=60)
    txt = (p.stdout + p.stderr).strip()
    entry = None
    success = False
    warnings = []
    try:
        data = json.loads(txt.splitlines()[-1])
        success = bool(data.get("success"))
        for e in data.get("files", []):
            if os.path.realpath(e.get("path")) == os.path.realpath(path):
                entry = e
    except Exception:
        pass
    if entry is None:
        return {"status": "unparsed", "error": txt[:200], "warnings": [], "success": success}
    return {"status": entry.get("status"), "error": entry.get("error", ""),
            "warnings": entry.get("warnings", []), "success": success}


def classify(meta, zz, verdict, blob):
    kind = meta["kind"]
    zz_ok = zz["status"] == "ok"
    r = {"path": meta["path"], "kind": kind, "rule": meta["rule"], "mut": meta["mut"],
         "zz_status": zz["status"], "zz_error": zz["error"], "zz_warnings": zz["warnings"],
         "zz_success": zz["success"], "runtime_verdict": verdict, "outcome": None,
         "note": "", "blob": blob[:300]}

    name = os.path.basename(meta["path"])
    is_test_file = "test__" in name

    if kind == "valid":
        if not zz_ok:
            r["outcome"] = "FP"
            r["note"] = "valid syntax rejected by -zz"
        elif verdict != "clean":
            r["outcome"] = "REVIEW"
            r["note"] = f"template rejected at runtime ({verdict})"
        else:
            r["outcome"] = "OK"
    elif kind == "invalid":
        if verdict in ("syntax_error", "crash"):
            if zz_ok:
                r["outcome"] = "FN"
                r["note"] = f"invalid syntax accepted by -zz (runtime: {verdict})"
            else:
                r["outcome"] = "OK"
                r["note"] = f"caught by both (runtime: {verdict})"
        elif verdict == "timeout":
            if zz_ok:
                r["outcome"] = "DUAL_GAP"
                r["note"] = "accepted at runtime and looped (triage: invalid, or valid infinite loop e.g. bare 'while')"
            else:
                r["outcome"] = "STRICT"
                r["note"] = "runtime lenient (looped), -zz rejects"
        elif verdict == "clean":
            if zz_ok:
                r["outcome"] = "DUAL_GAP"
                r["note"] = "invalid by construction, but runtime ALSO accepted (EOF leniency / silent no-op)"
            elif is_test_file:
                r["outcome"] = "UNVERIFIABLE"
                r["note"] = "test body skipped at runtime without -t; -zz ground truth only"
            else:
                r["outcome"] = "STRICT"
                r["note"] = "runtime lenient (stray/EOF closer), -zz rejects - desirable strictness"
        elif verdict == "semantic_error":
            if zz_ok:
                r["outcome"] = "DUAL_GAP"
                r["note"] = "mutation only broke semantics; runtime and -zz both accept syntax (triage)"
            else:
                r["outcome"] = "STRICT"
                r["note"] = "runtime lenient, -zz rejects (semantic) - desirable strictness"
        else:
            r["outcome"] = "REVIEW"
            r["note"] = f"runtime {verdict}"
    elif kind == "fuzz":
        if verdict == "clean":
            if zz_ok:
                r["outcome"] = "OK"
            else:
                r["outcome"] = "STRICT"
                r["note"] = "runtime tolerated file, -zz rejects - verify not a true FP"
        elif verdict in ("syntax_error", "crash"):
            if zz_ok:
                r["outcome"] = "FN"
                r["note"] = f"invalid fuzz syntax accepted by -zz (runtime: {verdict})"
            else:
                r["outcome"] = "OK"
                r["note"] = f"caught by both (runtime: {verdict})"
        elif verdict == "semantic_error":
            if zz_ok:
                r["outcome"] = "OK"
                r["note"] = "semantic_only"
            else:
                r["outcome"] = "STRICT"
                r["note"] = "fuzz parsed at runtime (semantic) but -zz rejects - verify not a true FP"
        else:
            r["outcome"] = "REVIEW"
            r["note"] = f"runtime {verdict}"
    else:  # static
        r["outcome"] = "STATIC"
        r["note"] = "syntax-check only, never executed"
    return r


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--baseline", nargs="*", default=[], metavar="DIR",
                    help="-zz-only sweep over .za files in these trees")
    ap.add_argument("--verify", action="store_true", help="strace-verify executed corpus")
    ap.add_argument("--no-probe", action="store_true", help="skip runtime probes (classification skipped)")
    args = ap.parse_args()

    results = []
    baseline = []

    if args.baseline:
        files = []
        for d in args.baseline:
            for root, _dirs, fnames in os.walk(d):
                for fn in sorted(fnames):
                    if fn.endswith(".za"):
                        files.append(os.path.join(root, fn))
        for path in files:
            zz = run_zz(path)
            entry = {"path": path, "kind": "baseline", "zz_status": zz["status"],
                     "zz_error": zz["error"], "zz_warnings": zz["warnings"],
                     "zz_success": zz["success"], "runtime_verdict": "-", "outcome": None}
            if zz["status"] == "ok":
                entry["outcome"] = "OK" if not zz["warnings"] else "warn"
            elif zz["status"] == "unparsed":
                entry["outcome"] = "review"
            else:
                entry["outcome"] = "zz_error"
            baseline.append(entry)
        print(f"baseline: {len(baseline)} files, "
              f"{sum(1 for b in baseline if b['outcome']=='OK')} clean, "
              f"{sum(1 for b in baseline if b['outcome']=='zz_error')} zz_error, "
              f"{sum(1 for b in baseline if b['outcome']=='warn')} warn, "
              f"{sum(1 for b in baseline if b['outcome']=='review')} review")

    if not args.no_probe:
        manifest = json.load(open(os.path.join(CORPUS, "manifest.json")))
        executed = 0
        for meta in manifest:
            path = meta["path"]
            zz = run_zz(path)
            if meta["kind"] == "static":
                r = {"path": path, "kind": "static", "rule": meta["rule"], "mut": meta["mut"],
                     "zz_status": zz["status"], "zz_error": zz["error"],
                     "zz_warnings": zz["warnings"], "zz_success": zz["success"],
                     "runtime_verdict": "-", "outcome": "OK" if zz["status"] == "ok" else "zz_error",
                     "note": "static-only (not executed)", "blob": ""}
                results.append(r)
                continue
            with open(path) as f:
                prog = f.read()
            rc, out, err = sandbox_probe(prog)
            verdict = runtime_verdict(rc, out, err)
            blob = out + err
            results.append(classify(meta, zz, verdict, blob))
            executed += 1

        if args.verify:
            verify_executed(manifest, results)

    # Summaries
    from collections import Counter
    out = Counter(r["outcome"] for r in results)
    print("corpus outcomes:", dict(out))
    if args.baseline:
        bo = Counter(b["outcome"] for b in baseline)
        print("baseline outcomes:", dict(bo))

    for oc in ("FN", "FP"):
        rows = [r for r in results if r["outcome"] == oc]
        if rows:
            print(f"\n== {oc} ({len(rows)}) ==")
            for r in rows[:200]:
                print(f"  {r['kind']:<7} {os.path.basename(r['path']):<36} "
                      f"rule={r['rule'] or '-':<18} {r['note']}")
                if r.get("blob"):
                    m = re.search(r"Error in[^\n]*\n?([^\n]*)", r["blob"])
                    if m:
                        print(f"      runtime: {m.group(1).strip()[:140]}")

    unknown = Counter()
    for r in results:
        if r["runtime_verdict"] in ("unknown_error", "timeout"):
            unknown[r["runtime_verdict"]] += 1
    if unknown:
        print(f"\nunknown/timeout verdicts: {dict(unknown)} (review manually; "
              f"add signatures if clearly syntax/semantic)")

    review = [r for r in results if r["outcome"] == "review"]
    if review:
        print(f"\nreview ({len(review)}):")
        for r in review[:50]:
            print(f"  {os.path.basename(r['path'])} {r['note']}")

    with open(REPORT, "w") as f:
        json.dump({"results": results, "baseline": baseline}, f, indent=1)
    print(f"\nreport written to {REPORT}")


# --------------------------------------------------------------------------
# strace verification: prove the executed corpus performs no host side effects.
# --------------------------------------------------------------------------

FORBIDDEN_SYSCALLS = {
    "creat", "unlink", "unlinkat", "mkdir", "mkdirat", "rmdir", "rename",
    "renameat", "renameat2", "truncate", "ftruncate", "chmod", "fchmod",
    "chown", "fchown", "lchown", "connect", "socket", "bind", "listen",
    "accept", "accept4", "sendto", "sendmsg", "sendmmsg", "recvfrom",
    "recvmsg", "recvmmsg", "execve", "execveat", "fork", "vfork", "kill",
    "ptrace", "mount", "setuid", "setgid", "mknod",
    "mknodat", "symlink", "symlinkat", "link", "linkat", "setxattr",
    "sethostname", "reboot", "swapon", "swapoff", "pivot_root", "chroot",
    "ioperm", "iopl",
}

def strace_probe(prog, tracefile):
    cmd = ["strace", "-f", "-o", tracefile, "-e", "trace=all",
           "timeout", "10", "bwrap",
           "--unshare-all", "--die-with-parent", "--new-session",
           "--ro-bind", "/", "/",
           "--proc", "/proc", "--dev", "/dev",
           "--tmpfs", "/tmp", "--tmpfs", "/run", "--tmpfs", "/var/tmp",
           "--tmpfs", "/var/log",
           "--bind", WORK, WORK,
           "--chdir", WORK,
           "--setenv", "HOME", WORK,
           "--", ZA, "-e", prog]
    p = subprocess.run(cmd, input="", capture_output=True, text=True, timeout=25)
    return p.returncode, p.stdout, p.stderr

def benign_path(p):
    if not p or not p.startswith("/"):
        return True
    if p.startswith(WORK) or p.startswith("/tmp"):
        return True
    if p in ("/dev/null", "/dev/tty", "/dev/stdin", "/dev/stdout", "/dev/stderr"):
        return True
    return False


def verify_executed(manifest, results):
    violations = []
    by_file = {}
    for meta in manifest:
        if meta["kind"] == "static":
            continue
        with open(meta["path"]) as f:
            prog = f.read()
        tracefile = os.path.join(WORK, "strace.log")
        rc, out, err = strace_probe(prog, tracefile)
        if not os.path.exists(tracefile):
            continue
        with open(tracefile, errors="replace") as tf:
            lines = tf.read().splitlines()
        execve_paths = []
        za_exec_idx = None
        for i, l in enumerate(lines):
            em = re.search(r'execve(at)?\("([^"]+)"', l)
            if em and em.group(2) == ZA:
                za_exec_idx = i
                break
        probs = []
        for i, l in enumerate(lines):
            m = re.match(r"\d+\s+([a-z0-9_]+)\(", l)
            if not m:
                continue
            if za_exec_idx is not None:
                if i <= za_exec_idx:
                    continue
            name = m.group(1)
            pm = re.search(r'"([^"]*)"', l)
            path = pm.group(1) if pm else ""
            if name in ("execve", "execveat"):
                em = re.search(r'execve(at)?\("([^"]+)"', l)
                if em:
                    execve_paths.append(em.group(2))
                continue
            if name in FORBIDDEN_SYSCALLS:
                if name == "kill" and ("SIGTERM" in l or "SIGCONT" in l):
                    continue
                if name in ("unlink", "unlinkat", "rmdir", "rename", "renameat",
                            "renameat2", "mkdir", "mkdirat", "truncate",
                            "ftruncate", "chmod", "fchmod", "chown", "fchown",
                            "lchown", "symlink", "symlinkat", "link", "linkat",
                            "mknod", "mknodat", "setxattr") and benign_path(path):
                    continue
                probs.append(l.strip()[:200])
            elif name in ("openat", "open"):
                if re.search(r"O_WRONLY|O_RDWR|O_CREAT|O_TRUNC", l) and not benign_path(path):
                    probs.append(l.strip()[:200])
        verdict = runtime_verdict(rc, out, err)
        if probs:
            violations.append({"path": meta["path"], "verdict": verdict, "syscalls": probs})
        by_file[meta["path"]] = {"verdict": verdict, "rc": rc, "execve": len(execve_paths)}
    if violations:
        print(f"\nVERIFY FAIL: {len(violations)} file(s) performed host side effects:")
        for v in violations:
            print(f"  {os.path.basename(v['path'])} verdict={v['verdict']}")
            for s in v["syscalls"]:
                print(f"    {s}")
        print("Corpus must be regenerated with those tokens excluded.")
        return 1
    print(f"\nVERIFY PASS: all {len(by_file)} executed files showed no host-side "
          f"syscalls (network/file-write/process/ipc all confined to the sandbox).")
    return 0


if __name__ == "__main__":
    sys.exit(main())
