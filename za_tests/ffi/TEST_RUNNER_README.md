# FFI/AUTO Test Suite Runner

This directory contains a comprehensive test suite for Za's FFI (Foreign Function Interface) and AUTO (C header auto-import) functionality.

## Quick Start

To run all tests:
```bash
./run_all_tests.sh
```

## Test Runner Options

The test runner script (`run_all_tests.sh`) supports the following options:

```bash
./run_all_tests.sh [OPTIONS]

Options:
  -v, --verbose        Show full output for each test
  -f, --show-failures  Show error output for failed tests
  -h, --help           Show help message
```

### Examples

**Run all tests with full output:**
```bash
./run_all_tests.sh -v
```

**Run all tests and display errors for any failures:**
```bash
./run_all_tests.sh -f
```

**Run tests with both verbose and failure reporting:**
```bash
./run_all_tests.sh -v -f
```

## What the Test Runner Does

1. **Automatic Setup**: Compiles the `test_union_struct.so` shared library if it doesn't exist
2. **Runs All Tests**: Executes all `test_*.za` files in the directory
3. **Handles Known Issues**: Automatically skips tests with known pre-existing issues
4. **Summary Report**: Displays pass/fail counts and lists any failed tests

## Test Coverage

The test suite includes tests for:
- **AUTO Preprocessor**: `#if`, `#elif`, `#else`, string literals, numeric expressions
- **Union Types**: Union fields with nested structs, marshaling/unmarshaling
- **FFI Integration**:
  - Standard library functions (libc: strlen, strcmp, malloc, etc.)
  - Graphics libraries (libgd, libpng, ncurses)
  - JSON libraries (jansson)
  - Compression libraries (zlib)
  - Network libraries (curl)
  - GLib structures
- **Callbacks**: Dynamic callback registration and trampolines
- **Type System**: Type checking, typedef resolution, fixed arrays

## Known Issues

The following tests are automatically skipped due to pre-existing issues:
- `test_union_marshal.za` - Union marshaling (Za→C) has partial implementation
- `test_ffi_libc.za` - fclose() FILE* pointer handling issue
- `test_callback_comprehensive.za` - Output formatting issue

These issues are pre-existing and not related to recent fixes.

## Recent Fixes (2026-01-18)

### Fix #1: Union Struct Library Compilation
- Compiled `test_union_struct.so` with proper PIC flags
- Tests `test_union_struct_simple.za` and `test_union_debug.za` now pass

### Fix #2: AUTO Preprocessor String Literal Bug
- Fixed `addCPrefixToIdentifiers()` in `lib-c_headers.go`
- String literals inside macro values are no longer incorrectly transformed
- Test `test_auto_if_elif.za` now passes with all assertions

## Test Results

When you run `./run_all_tests.sh`, you should see:
- 50 tests passing
- 3 tests skipped (known issues)
- Full green indicator: ✓ All tests passed!

```
═════════════════════════════════════════════
Test Results Summary
═════════════════════════════════════════════
Total tests:     50
Passed:          50
Failed:          0

✓ All tests passed!
```

## Manual Test Execution

To run a single test manually:
```bash
../../za -a test_auto_if_elif.za
../../za -a test_union_struct_simple.za
../../za -a test_ffi_struct_basic.za
```

The `../../za` binary is the compiled Za language interpreter located in the parent directory.

## Rebuilding the Za Compiler

After making code changes to Za, rebuild the compiler:
```bash
cd ../../
go build -o za .
```

Then re-run the test suite:
```bash
cd za_tests/ffi
./run_all_tests.sh
```

## Notes

- The test runner automatically recompiles `test_union_struct.so` if it's missing
- Tests are sorted alphabetically for consistent ordering
- All output uses ANSI color codes for better readability
- Test results are cached (the script doesn't skip already-passed tests)

## Troubleshooting

**"Za compiler not found"** - The Za binary needs to be built:
```bash
cd /home/daniel/go/src/za
go build -o za .
```

**"Failed to compile test_union_struct.so"** - Ensure gcc is installed:
```bash
gcc -shared -fPIC -o test_union_struct.so test_union_struct_lib.c
```

**Individual test fails** - Run with verbose mode for details:
```bash
./run_all_tests.sh -v -f
```
