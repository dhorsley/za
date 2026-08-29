#!/bin/bash
# Build the Windows FFI test fixture DLLs (MinGW-w64 cross compiler).
# Usage: ./build_dlls.sh [clean]
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

CC="${CC:-x86_64-w64-mingw32-gcc}"

if [ "$1" = "clean" ]; then
    rm -f za_ffi_test.dll test_wchar_lib.dll
    echo "Removed fixture DLLs."
    exit 0
fi

if ! command -v "$CC" >/dev/null 2>&1; then
    echo "Error: cross compiler '$CC' not found." >&2
    echo "Install mingw-w64 (e.g. Arch: pacman -S mingw-w64-gcc) or set CC." >&2
    exit 1
fi

echo "Compiling za_ffi_test.dll ..."
"$CC" -shared -o za_ffi_test.dll za_ffi_test.c -O2

echo "Compiling test_wchar_lib.dll ..."
"$CC" -shared -o test_wchar_lib.dll test_wchar_lib.c -O2

echo "Done:"
ls -la *.dll