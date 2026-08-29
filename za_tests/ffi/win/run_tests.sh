#!/bin/bash
# Windows FFI test suite runner.
# Runs every test_win_*.za under Proton/standalone Wine (see setup_wineprefix.sh).
# A test only PASSes when: (1) exit code is 0, AND (2) its output contains the
# script's own "[WIN-OK] <name>" trailer (printed after the last assert).
# A negative control (test_win_neg.za) must FAIL, proving failures are caught.
#
# Usage: ./run_tests.sh [-v|--verbose] [-f|--show-failures]
# Env:   ZA_WINEPREFIX, PROTON_ROOT / ZA_WINE64 (see setup_wineprefix.sh)

set -o pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ZA_ROOT="$(cd "$SCRIPT_DIR/../../.." && pwd)"

VERBOSE=0
SHOW_FAILURES=0
while [[ $# -gt 0 ]]; do
    case $1 in
        -v|--verbose) VERBOSE=1; shift ;;
        -f|--show-failures) SHOW_FAILURES=1; shift ;;
        -h|--help)
            echo "Usage: $0 [-v] [-f]"; exit 0 ;;
        *) echo "Unknown option: $1"; exit 1 ;;
    esac
done

COLOR() { printf "\033[%sm%s\033[0m" "$1" "$2"; }

# --- Wine prefix ------------------------------------------------------------
source "$SCRIPT_DIR/setup_wineprefix.sh" || exit 1

# --- Build za.exe (fresh cross-build into the prefix dir, outside the repo) --
ZA_EXE="$WINEPREFIX/za.exe"
if [ ! -f "$ZA_EXE" ] || find "$ZA_ROOT" -maxdepth 1 -name '*.go' -newer "$ZA_EXE" | grep -q .; then
    echo "Building Windows za.exe ..."
    ( cd "$ZA_ROOT" && \
      CGO_ENABLED=1 CC=x86_64-w64-mingw32-gcc CXX=x86_64-w64-mingw32-g++ \
      GOOS=windows GOARCH=amd64 go build -o "$ZA_EXE" . ) || exit 1
fi

# --- Build fixture DLLs -----------------------------------------------------
cd "$SCRIPT_DIR"
if [ ! -f za_ffi_test.dll ] || [ ! -f test_wchar_lib.dll ]; then
    ./build_dlls.sh || exit 1
fi

declare -A FAILED_OUTPUTS=()
TOTAL=0 PASSED=0 FAILED=0

record_fail() {  # record_fail <desc> <output>
    FAILED=$((FAILED + 1))
    FAILED_OUTPUTS["$1"]="$2"
}

report() {  # report <desc> <ok...>
    local desc="$1"; shift
    if [ "$1" -eq 0 ]; then
        echo "  $(COLOR 32 '✓ PASS')  $desc"
        PASSED=$((PASSED + 1))
    else
        echo "  $(COLOR 31 '✗ FAIL')  $desc"
        FAILED=$((FAILED + 1))
    fi
    TOTAL=$((TOTAL + 1))
}

# run_positive <desc> <test.za> - must exit 0 AND emit [WIN-OK]
run_positive() {
    local desc="$1" test_file="$2"
    local output
    output=$(WINEDEBUG=-all "$WINE64" "$ZA_EXE" -a "$test_file" 2>&1)
    local rc=$?

    if [ "$VERBOSE" -eq 1 ] || [ $rc -ne 0 ]; then
        [ $rc -eq 0 ] && echo -e "$output" | sed 's/^/    /'
    fi

    if [ $rc -eq 0 ] && printf '%s' "$output" | grep -q '\[WIN-OK\]'; then
        local trail
        trail=$(printf '%s' "$output" | grep '\[WIN-OK\]' | head -1 | sed 's/^[[:space:]]*//')
        report "$desc :: ${trail:-ok}" 0
    else
        echo "  $(COLOR 31 '✗ FAIL')  $desc (exit=$rc)"
        if [ "$SHOW_FAILURES" -eq 1 ]; then
            echo -e "$output" | sed 's/^/      /'
        fi
        record_fail "$desc" "$output"
        TOTAL=$((TOTAL + 1))
    fi
}

echo ""
echo "════════════════════════════════════════════════"
echo "     Windows FFI Test Suite"
echo "════════════════════════════════════════════════"
echo ""

# --- Normal suite (test_win_*.za except the negative control) ---------------
for test_file in test_win_*.za; do
    case "$test_file" in
        *_neg.za) continue ;;
    esac
    run_positive "$test_file" "$test_file"
done

# --- Negative control: must FAIL -------------------------------------------
output=$(WINEDEBUG=-all "$WINE64" "$ZA_EXE" -a test_win_neg.za 2>&1)
rc=$?
if [ $rc -ne 0 ] && ! printf '%s' "$output" | grep -q '\[WIN-OK\]'; then
    report "negative control test_win_neg.za (assert failure caught)" 0
else
    echo "  $(COLOR 31 '✗ FAIL')  negative control: expected assert failure (rc=$rc)"
    record_fail "negative control" "$output"
    TOTAL=$((TOTAL + 1))
fi

# --- Phase 1: libffi discovery (script-dir-first) ---------------------------
DLL="$SCRIPT_DIR/libffi-8.dll"
if [ -f "$DLL" ]; then
    run_positive "libffi discovery (libffi-8.dll beside script)" test_win_add.za

    mv "$DLL" "$DLL.hidden"
    trap 'mv "$DLL.hidden" "$DLL" 2>/dev/null' EXIT
    output=$(WINEDEBUG=-all "$WINE64" "$ZA_EXE" -a test_win_add.za 2>&1)
    rc=$?
    trap - EXIT
    mv "$DLL.hidden" "$DLL"

    if [ $rc -ne 0 ] && printf '%s' "$output" | grep -qi 'libffi'; then
        echo "  $(COLOR 32 '✓ PASS')  libffi DLL absent -> clean FFI error (rc=$rc)"
        PASSED=$((PASSED + 1))
    else
        echo "  $(COLOR 31 '✗ FAIL')  libffi DLL absent -> expected clean libffi error (rc=$rc)"
        record_fail "libffi absent" "$output"
    fi
    TOTAL=$((TOTAL + 1))
else
    echo "  $(COLOR 33 'skipped')  libffi-8.dll not found in $SCRIPT_DIR"
fi

# --- Phase 19: AUTO cache behaviour ----------------------------------------
run_positive "AUTO cache: first parse (creates cache)" test_win_auto.za
run_positive "AUTO cache: second load (from cache)" test_win_auto.za

output=$(ZA_FFI_NOCACHE=1 WINEDEBUG=-all "$WINE64" "$ZA_EXE" -a test_win_auto.za 2>&1)
rc=$?
if [ $rc -eq 0 ] && printf '%s' "$output" | grep -q '\[WIN-OK\]'; then
    report "AUTO cache: ZA_FFI_NOCACHE=1 re-parse" 0
else
    echo "  $(COLOR 31 '✗ FAIL')  AUTO cache: ZA_FFI_NOCACHE=1 re-parse (rc=$rc)"
    record_fail "nocache" "$output"
    TOTAL=$((TOTAL + 1))
fi

# --- Phase 19c: ZA_FFI_CACHE_CLEAR (deletes cache, forces fresh parse) ---
output=$(ZA_FFI_CACHE_CLEAR=1 WINEDEBUG=-all "$WINE64" "$ZA_EXE" -a test_win_auto.za 2>&1)
rc=$?
if [ $rc -eq 0 ] && printf '%s' "$output" | grep -q '\[WIN-OK\]' \
    && ! printf '%s' "$output" | grep -q 'loaded from cache'; then
    report "AUTO cache: ZA_FFI_CACHE_CLEAR=1 (cleared + re-parsed)" 0
else
    echo "  $(COLOR 31 '✗ FAIL')  AUTO cache: ZA_FFI_CACHE_CLEAR=1 (rc=$rc)"
    record_fail "cache_clear" "$output"
    TOTAL=$((TOTAL + 1))
fi

# After clear + rebuild, a plain run loads from cache again (rebuild worked)
run_positive "AUTO cache: load after ZA_FFI_CACHE_CLEAR (rebuilt)" test_win_auto.za

# --- Phase 20: -xx libffi bundling ----------------------------------------
if [ -f bundle_ffi_win.za ]; then
    # Build control (-x only) and -xx bundles from the current za.exe
    WINEDEBUG=-all "$WINE64" "$ZA_EXE" -x -n bundle_ctl.exe bundle_ffi_win.za >/dev/null 2>&1
    bctl=$?
    WINEDEBUG=-all "$WINE64" "$ZA_EXE" -xx -n bundle_xx.exe bundle_ffi_win.za >/dev/null 2>&1
    bxx=$?

    # Run bundles from a neutral cwd so the OS DLL search cannot accidentally
    # pick up the libffi-8.dll committed in this test directory.
    if [ $bctl -eq 0 ]; then
        output=$(cd "$ZA_ROOT" && WINEDEBUG=-all "$WINE64" "$SCRIPT_DIR/bundle_ctl.exe" 2>&1)
        rc=$?
        if [ $rc -ne 0 ] && printf '%s' "$output" | grep -qi 'libffi'; then
            report "bundle -x (control) fails cleanly without libffi" 0
        else
            echo "  $(COLOR 31 '✗ FAIL')  bundle -x (control) expected clean libffi error (rc=$rc)"
            record_fail "bundle control" "$output"
            TOTAL=$((TOTAL + 1))
        fi
    else
        echo "  $(COLOR 31 '✗ FAIL')  control bundle build failed (rc=$bctl)"
        TOTAL=$((TOTAL + 1)); FAILED=$((FAILED + 1))
    fi

    if [ $bxx -eq 0 ]; then
        output=$(cd "$ZA_ROOT" && WINEDEBUG=-all "$WINE64" "$SCRIPT_DIR/bundle_xx.exe" 2>&1)
        rc=$?
        if [ $rc -eq 0 ] && printf '%s' "$output" | grep -q '\[WIN-OK\]'; then
            report "bundle -xx (libffi embedded) FFI works in bundle" 0
        else
            echo "  $(COLOR 31 '✗ FAIL')  bundle -xx FFI failed (rc=$rc)"
            record_fail "bundle -xx" "$output"
            TOTAL=$((TOTAL + 1))
        fi

        s_ctl=$(stat -c %s bundle_ctl.exe 2>/dev/null || echo 0)
        s_xx=$(stat -c %s bundle_xx.exe 2>/dev/null || echo 0)
        s_dll=$(stat -c %s libffi-8.dll)
        if [ $((s_xx - s_ctl)) -ge $((s_dll / 2)) ]; then
            report "bundle -xx embeds libffi-8.dll (size +$((s_xx - s_ctl)) bytes)" 0
        else
            echo "  $(COLOR 31 '✗ FAIL')  bundle -xx size delta too small (${s_ctl}->${s_xx}, dll=${s_dll})"
            TOTAL=$((TOTAL + 1)); FAILED=$((FAILED + 1))
        fi
    else
        echo "  $(COLOR 31 '✗ FAIL')  -xx bundle build failed (rc=$bxx)"
        TOTAL=$((TOTAL + 1)); FAILED=$((FAILED + 1))
    fi
    rm -f bundle_ctl.exe bundle_xx.exe
else
    echo "  $(COLOR 33 'skipped')  bundle_ffi_win.za not found"
fi

# --- Report -----------------------------------------------------------------
echo ""
echo "════════════════════════════════════════════════"
echo "Test Results Summary"
echo "════════════════════════════════════════════════"
echo "Total:   $TOTAL"
echo "Passed:  $PASSED"
echo "Failed:  $FAILED"

if [ "$FAILED" -eq 0 ]; then
    echo ""
    echo "$(COLOR 32 '✓ All tests passed!')"
    exit 0
else
    if [ "$SHOW_FAILURES" -eq 1 ]; then
        for k in "${!FAILED_OUTPUTS[@]}"; do
            echo "--- $k"; echo -e "${FAILED_OUTPUTS[$k]}" | sed 's/^/    /'
        done
    fi
    echo "$(COLOR 31 '✗ Some tests failed')"
    exit 1
fi