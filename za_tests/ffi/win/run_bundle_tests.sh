#!/bin/bash
# Linux bundle test for -xx (embedding native libffi in an -x bundle).
# Verifies:
#   - a -xx bundle run with LD_LIBRARY_PATH cleared still does FFI (extracted
#     script dir ships libffi.so.8, which the loader checks first)
#   - the -xx bundle is meaningfully larger (embeds the libffi .so)
#   - a plain -x bundle still works (regression)
# Env: ZA_LINUX (path to a linux za build; default: build one to /tmp)

set -o pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ZA_ROOT="$(cd "$SCRIPT_DIR/../../.." && pwd)"
cd "$SCRIPT_DIR"

COLOR() { printf "\033[%sm%s\033[0m" "$1" "$2"; }
TOTAL=0 PASSED=0 FAILED=0
report() {
    if [ "$1" -eq 0 ]; then echo "  $(COLOR 32 '✓ PASS')  $2"; PASSED=$((PASSED+1))
    else echo "  $(COLOR 31 '✗ FAIL')  $2"; FAILED=$((FAILED+1)); fi
    TOTAL=$((TOTAL+1))
}

ZA_LINUX="${ZA_LINUX:-/tmp/za-linux-bundle}"
if [ ! -x "$ZA_LINUX" ]; then
    echo "Building linux za ..."
    ( cd "$ZA_ROOT" && go build -o "$ZA_LINUX" . ) || exit 1
fi

# The bundle needs the local test .so (built from the shared fixture source)
if [ ! -f za_ffi_test.so ]; then
    echo "Building za_ffi_test.so ..."
    gcc -shared -fPIC -O2 -o za_ffi_test.so za_ffi_test.c || exit 1
fi

echo ""
echo "════════════════════════════════════════════════"
echo "     Linux bundle test (-xx libffi embedding)"
echo "════════════════════════════════════════════════"
echo ""

"$ZA_LINUX" -x -n bundle_ctl.gz bundle_ffi_linux.za >/dev/null 2>&1
rc=$?
if [ $rc -ne 0 ]; then echo "  $(COLOR 31 '✗ FAIL')  control bundle build failed"; exit 1; fi

"$ZA_LINUX" -xx -n bundle_xx.gz bundle_ffi_linux.za >/dev/null 2>&1
rc=$?
if [ $rc -ne 0 ]; then echo "  $(COLOR 31 '✗ FAIL')  -xx bundle build failed"; exit 1; fi

# -xx bundle with an empty LD_LIBRARY_PATH: embedded libffi beside script works
output=$(LD_LIBRARY_PATH= ./bundle_xx.gz 2>&1); rc=$?
if [ $rc -eq 0 ] && printf '%s' "$output" | grep -q '\[WIN-OK\]'; then
    report 0 "bundle -xx FFI works with empty LD_LIBRARY_PATH"
else
    echo "$output" | sed 's/^/    /'
    report 1 "bundle -xx FFI failed (rc=$rc)"
fi

# plain -x bundle regression (uses system libffi)
output=$(./bundle_ctl.gz 2>&1); rc=$?
if [ $rc -eq 0 ] && printf '%s' "$output" | grep -q '\[WIN-OK\]'; then
    report 0 "bundle -x (control) still works"
else
    echo "$output" | sed 's/^/    /'
    report 1 "bundle -x (control) failed (rc=$rc)"
fi

# size delta: -xx embeds the native libffi .so (real libffi .so files are
# tens of kilobytes, so require a meaningful margin)
s_ctl=$(stat -c %s bundle_ctl.gz)
s_xx=$(stat -c %s bundle_xx.gz)
if [ $((s_xx - s_ctl)) -ge 10000 ]; then
    report 0 "bundle -xx embeds native libffi (size +$((s_xx - s_ctl)) bytes)"
else
    report 1 "bundle -xx size delta too small (ctl=$s_ctl xx=$s_xx)"
fi

rm -f bundle_ctl.gz bundle_xx.gz

echo ""
echo "Total: $TOTAL  Passed: $PASSED  Failed: $FAILED"
[ "$FAILED" -eq 0 ] && echo "$(COLOR 32 '✓ All bundle tests passed!')" && exit 0
echo "$(COLOR 31 '✗ bundle tests failed')" && exit 1