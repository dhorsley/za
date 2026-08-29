# Windows FFI Test Suite (`za_tests/ffi/win`)

Validates the Windows build's C FFI (libffi-based) using the cross-built
`za.exe` running under **Steam's Proton** (a bundled, current Wine) or any
standalone Wine. Mirrors the doc's phased Windows FFI requirements and, for
each test, reuses the same `za_tests/ffi` conventions (`.c/.h` fixtures,
`module ... as x auto "....h"`, `LIB x::f(a:int)->int`, `assert`).

## Prerequisites
- `go` (with cgo) and the MinGW-w64 cross compiler `x86_64-w64-mingw32-gcc`
  (Arch: `pacman -S mingw-w64-gcc`).
- Either Steam/Proton (default lookup under `~/.steam/steam/steamapps/common/`
  `Proton - Experimental` / `Proton Hotfix`) or any standalone `wine64`.

## Usage
```bash
./build_dlls.sh                      # cross-compile za_ffi_test.dll, test_wchar_lib.dll
./run_tests.sh                       # runs every test_win_*.za + discovery + cache checks
./run_tests.sh -f                    # print output of failures
```

### Environment overrides (portable to other hosts)
- `ZA_WINEPREFIX`   – where the Wine prefix lives (default `$HOME/za-winprefix`)
- `PROTON_ROOT`     – directory of a Proton install (or set nothing; default paths)
- `ZA_WINE64`       – full path to a `wine64` binary (bypasses Proton lookup)
- `CC`              – cross compiler (default `x86_64-w64-mingw32-gcc`)

`run_tests.sh` sources `setup_wineprefix.sh` (idempotent prefix creation) and
cross-builds `za.exe` into the prefix directory (outside the repo) when stale.

A positive test only PASSes when `za.exe -a <script>` exits 0 **and** the
output contains the script's own `[WIN-OK] <name>` trailer (printed after the
last `assert`), so a silent failure can never read as a pass. The runner also
requires the negative control `test_win_neg.za` to fail, proving assert
failures are actually caught.

## Coverage (ran green: wine-10.0 / libffi-8.dll 3.8.0, mingw-w64 GCC 16.2.0)

| Doc phase | Script | Checks |
|-----------|--------|--------|
| 1 | `run_tests.sh` (present/absent) | libffi-8.dll loaded from the *script's* directory first; absent DLL → clean FFI error |
| 2 | `test_win_add.za` | int, double, const char*, void* calls |
| 3 | `test_win_abi.za` | int64, float, double, mixed int/double, strings (Win64 ABI) |
| 4 | `test_win_struct.za` | struct value return, **struct-literal** by-value arg, by-pointer arg, out-param writes |
| 5 | `test_win_variadic.za` | `ffi_prep_cif_var` path |
| 6 | `test_win_funcptr.za` | retrieve + wrap + call a C function pointer |
| 7 | `test_win_callback.za` | C DLL → libffi closure → Za function → return to C |
| 17 | `test_win_kernel32.za` | `kernel32.dll` → `GetCurrentProcessId` |
| 18 | `test_win_auto.za` | `MODULE ... AUTO`, macros, typedef fn-ptr, **struct-literal** calls |
| –  | `test_win_wchar.za` | 16-bit `wchar_t` runtime detection (2 bytes) |
| –  | `test_win_preproc_*.za` | platform/LLP64/glibc/MinGW predefined macros (`_WIN32`/`_WIN64` set; `__LP64__`/`__unix__`/glibc unset) |
| 19 | `run_tests.sh` (cache) | first parse → cache → cache load → `ZA_FFI_NOCACHE=1` |
| 20 | `run_tests.sh` + `run_bundle_tests.sh` | `-x`/`-xx` bundles: control fails cleanly without libffi, `-xx` runs with embedded libffi (Windows + Linux) |
| –  | `test_win_neg.za` | negative control: assert failure must be caught |

## Bundling (`-xx`; implies `-x`)
`za -xx` embeds the native libffi runtime (Windows: `shared/win/libffi-8.dll`;
Linux/BSD: system `libffi.so.8`/`.so.7`/`.so`), so FFI works in the bundle
without a system/wine libffi. On execution the library unpacks beside the
extracted script and is found by script-dir-first discovery. libffi is
MIT (`shared/win/LICENSE`). Windows bundles also required (and got) a fix so the
embedded binary extracts/runs as `za.exe`.

## Struct construction + cache note
AUTO-imported C structs (and unions) are registered as **typed Za structs**
(`registerStructInZa`, lib-c_headers.go) under `alias::Name`, so `ZaPoint(.x …,
.y …)` literals work; FFI value returns are exposed as dot-accessible maps. The
AUTO cache did **not** restore that registration on cache loads, breaking
struct literals after the first parse - fixed in `lib-c_cache.go`
(`populateGlobalMapsFromCache` now re-runs `registerStructInZa` for restored
structs). Note: under Wine/Windows the cache lives in the *virtual home*
(`$WINEPREFIX/drive_c/users/<user>/.cache/za/ffi`), not `~/.cache`.

### Cache control env vars (both work under Wine/Windows too)
- `ZA_FFI_NOCACHE=1` - bypass: skip reading and skip writing; existing cache
  files are left untouched.
- `ZA_FFI_CACHE_CLEAR=1` - delete this module's cache file(s), then force a
  fresh parse (the save path rebuilds them).

## ABI note
Windows ABI is selected at runtime via the loaded libffi's own
`ffi_get_default_abi()` (falling back to arch/OS mapping, `FFI_WIN64 = 3`).
The MSYS2 mingw-w64 libffi build reports `FFI_UNIX64` (2) as its default and
rejects `FFI_WIN64` (3)/`FFI_GNUW64` (4); hard-coding 3/4 would fail
`ffi_prep_cif` with `FFI_BAD_ABI`. Using the DLL's default works for every
checked call, so the constant is never assumed.

## Caveat
Wine is representative but not bit-identical to Windows. A final pass on a
real Windows host is still the acceptance gate (and the eventual OpenGL port
target).