# zzaudit

Audits the completeness of the `za -zz` static syntax checker against the
runtime's ground truth, to inform building a real syntax-checking feature.
Companion doc: `../../docs/GRAMMAR.md` (the de facto syntax reference).

## Layout

- `generate.py` - builds the corpus: 104 valid templates, 89 invalid
  mutations, 90 fuzz files (delete/dup/swap/insert mutations + token soup over
  the valid templates), 10 static files (module/use/lib/namespace/permit
  constructs - `-zz` only, never executed). Pinned RNG seed (default 7).
- `run.py` - executes each file with `za -zz` AND in a bwrap sandbox,
  classifies results, writes `report.json`.
- `corpus/` - generated files + `manifest.json`.
- `FINDINGS.md` - the audit results.

## Reproduce

```sh
python3 generate.py --selfcheck     # regenerate corpus; verify all 104 valid templates run clean
python3 run.py                      # full classification sweep (sandboxed execution)
python3 run.py --baseline ~/go/src/za/za_tests ~/3d/game ~/3d/lib   # -zz-only sweep over real code
python3 run.py --verify             # strace every executed file; assert zero host side effects
```

`report.json` is overwritten by the last run. Env `ZA` overrides the
interpreter path (default `~/go/src/za/za`).

## Safety model

Real scripts are never executed - only `-zz` checked. Corpus files are
executed in a sandbox to establish runtime ground truth, with three
independent guarantees:

1. **Construction** - the fuzz/mutation token pool is grammar tokens +
   invented identifiers (`foo`, `bar`, `items`, ...) only. Stdlib and real
   identifiers are never in the pool, so there is no hardcoded sanitize list
   to go stale.
2. **Containment** - every runtime probe:
   `bwrap --unshare-all --die-with-parent --new-session --ro-bind / / --proc
   /proc --dev /dev --tmpfs /tmp --tmpfs /run --tmpfs /var/tmp --tmpfs
   /var/log --bind <WORK> <WORK> --chdir <WORK> --setenv HOME <WORK>` +
   `timeout 10` + stdin `/dev/null`.
3. **Verification** - `--verify` re-runs all executed files under
   `strace -f -e trace=all` and asserts no host-side syscalls (network,
   file-write, process, ipc). It analyzes only syscalls made after the `za`
   execve, and exempts sandbox machinery (bwrap setup, Go runtime SIGURG,
   `timeout` watchdog signals) and benign sandbox-local paths (`<WORK>`,
   `/tmp`, `/dev/null`, `/dev/tty`, relative paths).

## Runtime ground truth semantics

- `za -zz FILE` outputs JSON: `{"files":[{...,"status":"ok|error","error","warnings"}],"success":bool}`.
- Runtime errors print `Error in ...` to **stdout**; many syntax errors exit
  rc=0, so a verdict is `rc != 0 OR error-text present`, never rc alone.
- Caught exceptions exit rc=0; `exit N` exits rc=N.
- Runtime is itself lenient in places (see FINDINGS.md) - when both sides
  accept invalid input the case is classified `DUAL_GAP` and must be triaged
  manually.

## Outcomes

| outcome       | meaning                                                        |
|---------------|----------------------------------------------------------------|
| `OK`          | both sides agree (valid accepted, invalid caught, or semantic-only) |
| `FN`          | `-zz` accepts syntax the runtime rejects/crashes on (a miss)     |
| `DUAL_GAP`    | invalid by construction but runtime ALSO accepted (triage in FINDINGS) |
| `STRICT`      | runtime lenient, `-zz` rejects - desirable strictness            |
| `FP`          | valid syntax rejected by `-zz` (over-rejection)                  |
| `UNVERIFIABLE`| no runtime ground truth (e.g. test blocks skip without `-t`)     |
| `REVIEW`      | ambiguous runtime verdict (timeout/unknown)                      |
| `STATIC`      | static construct, never executed                                 |
