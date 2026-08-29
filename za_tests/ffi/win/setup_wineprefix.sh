#!/bin/bash
# Initialize the Wine prefix used to run the Windows FFI test suite.
# Idempotent and host-portable:
#   - ZA_WINEPREFIX  (default $HOME/za-winprefix)  where the prefix lives
#   - ZA_WINE64      full path to a standalone wine64 binary (overrides Proton)
#   - PROTON_ROOT    dir of a Proton install (default: Steam Proton dirs)
# On another host: set PROTON_ROOT (or ZA_WINE64) and ZA_WINEPREFIX, then run.
# This script is safe to source from run_tests.sh (uses return on error).

set -u

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

ZA_WINEPREFIX="${ZA_WINEPREFIX:-$HOME/za-winprefix}"
export WINEPREFIX="$ZA_WINEPREFIX"
export WINEDLLOVERRIDES="${WINEDLLOVERRIDES:-winemenubuilder.exe=d}"

# Locate a wine64 binary.
WINE64="${ZA_WINE64:-}"
if [ -z "$WINE64" ]; then
    PROTON_ROOT="${PROTON_ROOT:-}"
    if [ -z "$PROTON_ROOT" ]; then
        for p in \
            "$HOME/.steam/steam/steamapps/common/Proton - Experimental" \
            "$HOME/.steam/steam/steamapps/common/Proton Hotfix" \
            "$HOME/.local/share/Steam/steamapps/common/Proton - Experimental"; do
            if [ -x "$p/files/bin/wine64" ]; then
                PROTON_ROOT="$p"
                break
            fi
        done
    fi
    if [ -n "$PROTON_ROOT" ] && [ -x "$PROTON_ROOT/files/bin/wine64" ]; then
        WINE64="$PROTON_ROOT/files/bin/wine64"
    else
        echo "Error: no wine64 found. Install Proton/Steam, set PROTON_ROOT to a" >&2
        echo "Proton install dir, or set ZA_WINE64 to a wine64 binary." >&2
        return 1 2>/dev/null || exit 1
    fi
fi
export WINE64

WINEDEBUG="${WINEDEBUG:--all}"
export WINEDEBUG

# Create/verify the prefix (only boots once - idempotent).
if [ ! -f "$WINEPREFIX/drive_c/windows/system32" ] && [ ! -d "$WINEPREFIX/drive_c" ]; then
    echo "Initializing Wine prefix at $WINEPREFIX ..."
    mkdir -p "$(dirname "$WINEPREFIX")"
    "$WINE64" wineboot -i >/dev/null 2>&1

    if [ ! -d "$WINEPREFIX/drive_c" ]; then
        echo "Error: Wine prefix creation failed." >&2
        return 1 2>/dev/null || exit 1
    fi
fi

echo "Wine:  $("$WINE64" --version 2>/dev/null)"
echo "Prefix: $WINEPREFIX"