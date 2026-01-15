// Test header for #if and #elif directives

#define LINUX 1
#define VERSION 2

// Test 1: Simple #if with numeric comparison
#if VERSION > 1
#define TEST1 "version_greater_than_1"
#endif

// Test 2: #if with defined() operator
#if defined(LINUX)
#define TEST2 "linux_defined"
#endif

// Test 3: #if with complex expression
#if defined(LINUX) && VERSION == 2
#define TEST3 "linux_and_version_2"
#endif

// Test 4: #if false, #elif true
#if VERSION == 1
#define TEST4 "version_1"
#elif VERSION == 2
#define TEST4 "version_2"
#else
#define TEST4 "version_other"
#endif

// Test 5: Multiple #elif with first one true
#if 0
#define TEST5 "zero"
#elif 1
#define TEST5 "one"
#elif 1
#define TEST5 "should_not_see_this"
#else
#define TEST5 "else"
#endif

// Test 6: All #if/#elif false, #else activates
#if 0
#define TEST6 "if"
#elif 0
#define TEST6 "elif"
#else
#define TEST6 "else"
#endif

// Test 7: #if with undefined macro (should be false)
#if UNDEFINED
#define TEST7 "undefined_was_true"
#else
#define TEST7 "undefined_was_false"
#endif

// Test 8: Complex boolean expression
#if (VERSION > 1) && (LINUX == 1)
#define TEST8 "complex_true"
#else
#define TEST8 "complex_false"
#endif
