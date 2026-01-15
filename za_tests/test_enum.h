/* Test header for enum parsing */

#define TEST_CONST_1 42
#define TEST_CONST_2 0x100
#define TEST_CONST_3 (1 << 5)

enum TestColors {
    RED,
    GREEN = 5,
    BLUE,
    ALPHA = 0xFF
};

enum {
    ANON_FIRST = 10,
    ANON_SECOND
};
