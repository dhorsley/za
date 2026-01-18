#!/bin/bash
# FFI Test Suite Runner
# Runs all FFI and AUTO tests and provides summary report

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
BOLD='\033[1m'
NC='\033[0m' # No Color

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ZA_BIN="$(cd "$SCRIPT_DIR/../.." && pwd)/za"
VERBOSE=0
SHOW_FAILURES=0

# Test counters
TOTAL_TESTS=0
PASSED_TESTS=0
FAILED_TESTS=0
declare -a FAILED_TEST_LIST

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -v|--verbose)
            VERBOSE=1
            shift
            ;;
        -f|--show-failures)
            SHOW_FAILURES=1
            shift
            ;;
        -h|--help)
            echo "Usage: $0 [OPTIONS]"
            echo ""
            echo "Options:"
            echo "  -v, --verbose        Show full output for each test"
            echo "  -f, --show-failures  Show error output for failed tests"
            echo "  -h, --help           Show this help message"
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            exit 1
            ;;
    esac
done

# Verify Za compiler exists
if [ ! -x "$ZA_BIN" ]; then
    echo -e "${RED}Error: Za compiler not found at $ZA_BIN${NC}"
    echo "Please run: cd $(dirname "$ZA_BIN") && go build -o za ."
    exit 1
fi

# Change to test directory
cd "$SCRIPT_DIR"

# Ensure compiled library is available
if [ ! -f "test_union_struct.so" ]; then
    echo -e "${YELLOW}Compiling test_union_struct.so...${NC}"
    gcc -shared -fPIC -o test_union_struct.so test_union_struct_lib.c 2>/dev/null || {
        echo -e "${RED}Failed to compile test_union_struct.so${NC}"
        exit 1
    }
    # Also copy to /tmp for tests that reference it there
    cp test_union_struct.so /tmp/test_union_struct.so
fi

echo -e "${BOLD}${BLUE}═════════════════════════════════════════════${NC}"
echo -e "${BOLD}${BLUE}     FFI/AUTO Test Suite Runner${NC}"
echo -e "${BOLD}${BLUE}═════════════════════════════════════════════${NC}"
echo ""

# Run all tests
for test_file in test_*.za; do
    TOTAL_TESTS=$((TOTAL_TESTS + 1))

    if [ $VERBOSE -eq 1 ]; then
        echo -e "${BOLD}Testing: $test_file${NC}"
        if "$ZA_BIN" -a "$test_file" 2>&1; then
            echo -e "${GREEN}✓ PASS${NC}  $test_file\n"
            PASSED_TESTS=$((PASSED_TESTS + 1))
        else
            echo -e "${RED}✗ FAIL${NC}  $test_file\n"
            FAILED_TESTS=$((FAILED_TESTS + 1))
            FAILED_TEST_LIST+=("$test_file")
        fi
    else
        if output=$("$ZA_BIN" -a "$test_file" 2>&1); then
            echo -e "${GREEN}✓${NC} $test_file"
            PASSED_TESTS=$((PASSED_TESTS + 1))
        else
            echo -e "${RED}✗${NC} $test_file"
            if [ $SHOW_FAILURES -eq 1 ]; then
                echo -e "${RED}  Error output:${NC}"
                echo "$output" | sed 's/^/    /'
            fi
            FAILED_TESTS=$((FAILED_TESTS + 1))
            FAILED_TEST_LIST+=("$test_file")
        fi
    fi
done

echo ""
echo -e "${BOLD}${BLUE}═════════════════════════════════════════════${NC}"
echo -e "${BOLD}${BLUE}Test Results Summary${NC}"
echo -e "${BOLD}${BLUE}═════════════════════════════════════════════${NC}"
echo -e "Total tests:     ${BOLD}$TOTAL_TESTS${NC}"
echo -e "Passed:          ${GREEN}${BOLD}$PASSED_TESTS${NC}"
echo -e "Failed:          ${RED}${BOLD}$FAILED_TESTS${NC}"

if [ $FAILED_TESTS -eq 0 ]; then
    echo -e "\n${GREEN}${BOLD}✓ All tests passed!${NC}"
    exit 0
else
    echo -e "\n${RED}${BOLD}✗ Some tests failed:${NC}"
    for failed_test in "${FAILED_TEST_LIST[@]}"; do
        echo -e "  ${RED}•${NC} $failed_test"
    done

    if [ $SHOW_FAILURES -eq 0 ]; then
        echo -e "\nRun with ${BOLD}-f${NC} or ${BOLD}--show-failures${NC} to see error output"
    fi

    exit 1
fi
