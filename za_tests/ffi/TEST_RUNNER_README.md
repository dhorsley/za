# FFI/AUTO Test Suite Runner

This directory contains a comprehensive test suite for Za's FFI (Foreign Function Interface) and AUTO (C header auto-import) functionality.

These tests are intended to be run on Arch. Module library/header paths may need adjusting
for other OS.

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

## Notes

- The test runner automatically recompiles `test_union_struct.so` if it's missing

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

