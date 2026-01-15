// Test header for preprocessor directive parsing
#ifndef TEST_PREPROCESSOR_H
#define TEST_PREPROCESSOR_H

// Simple ifdef
#ifdef __linux__
#define LINUX_VALUE 100
int linux_only_func(void);
#endif

// Simple ifndef
#ifndef __WINDOWS__
#define NOT_WINDOWS 1
#endif

// If-else branches
#ifdef __LP64__
#define POINTER_SIZE 8
typedef unsigned long size_t;
#else
#define POINTER_SIZE 4
typedef unsigned int size_t;
#endif

// Nested conditionals
#ifdef __linux__
  #ifdef __LP64__
    #define PLATFORM_ID "Linux 64-bit"
  #else
    #define PLATFORM_ID "Linux 32-bit"
  #endif
#endif

// Feature macro checks
#ifdef __USE_MISC
#define HAVE_MISC 1
int misc_func(void);
#endif

#endif  // TEST_PREPROCESSOR_H
