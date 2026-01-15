/* Test header for string and float constant parsing */

// ===== STRING CONSTANTS =====

// Simple strings
#define TEST_STRING_1 "hello"
#define TEST_STRING_2 "Hello, World!"

// Strings with spaces and punctuation
#define TEST_VERSION "1.2.3"
#define TEST_AUTHOR "John Doe <john@example.com>"

// Strings with escape sequences
#define TEST_NEWLINE "line1\nline2"
#define TEST_TAB "col1\tcol2"
#define TEST_QUOTE "He said \"hi\""

// Empty string
#define TEST_EMPTY ""

// Should be skipped (not simple strings)
#define TEST_CONCAT "part1" "part2"  // Concatenation - won't parse
#define TEST_MACRO(x) #x  // Function-like macro - will be skipped

// ===== FLOAT CONSTANTS =====

// Simple floats
#define TEST_PI 3.14159265358979323846
#define TEST_E 2.71828182845904523536

// Scientific notation
#define TEST_AVOGADRO 6.02214076e23
#define TEST_PLANCK 6.62607015E-34

// Integer-looking floats
#define TEST_TIMEOUT 30.0
#define TEST_RATIO 0.5

// Small precision
#define TEST_SMALL 1.5

// ===== INTEGER CONSTANTS (existing support) =====

// Decimal
#define TEST_INT_DEC 42

// Hex
#define TEST_INT_HEX 0xFF

// Bit shift
#define TEST_INT_SHIFT (1<<5)

// ===== EXPRESSION CONSTANTS (bonus feature via ev()) =====

// Arithmetic
#define TEST_EXPR_ARITH (100 + 50)

// Bitwise
#define TEST_EXPR_BITWISE (0xFF00 | 0x00FF)

// ===== CONSTANTS REFERENCING OTHER CONSTANTS =====

// Later constants can reference earlier ones
#define BASE_SIZE 1024
#define BUFFER_SIZE (BASE_SIZE * 2)
#define LARGE_BUFFER (BUFFER_SIZE * 4)
