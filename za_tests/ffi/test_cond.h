#ifndef TEST_COND_H
#define TEST_COND_H

#define OPENSSL_CONFIGURED_API 30600
#define OPENSSL_VERSION_MAJOR 3
#define OPENSSL_VERSION_MINOR 6

#ifndef OPENSSL_API_LEVEL
# if OPENSSL_CONFIGURED_API > 0
#  define OPENSSL_API_LEVEL (OPENSSL_CONFIGURED_API)
# else
#  define OPENSSL_API_LEVEL (OPENSSL_VERSION_MAJOR * 10000 + OPENSSL_VERSION_MINOR * 100)
# endif
#endif

#if OPENSSL_API_LEVEL > OPENSSL_CONFIGURED_API
#  error "impossible"
#endif

#if OPENSSL_API_LEVEL < 30000 && OPENSSL_API_LEVEL >= 20000
#  error "OpenSSL version 2 not supported"
#endif

// Ternary operator in #define/#if (previously #if ternaries were forced to false)
#define TEST_TERNARY_VALUE (OPENSSL_API_LEVEL >= 20000 ? 1 : 0)

#if (OPENSSL_API_LEVEL > 0 ? 1 : 0)
#  define TEST_DIRECT_TERNARY 1
#else
#  define TEST_DIRECT_TERNARY 0
#endif

#if (0 ? 1 : 0)
#  define TEST_FALSE_TERNARY 1
#else
#  define TEST_FALSE_TERNARY 0
#endif

#if TEST_TERNARY_VALUE == 1
#  define TEST_INDIRECT_TERNARY 1
#else
#  define TEST_INDIRECT_TERNARY 0
#endif

// Test function
int test_func(void);

#endif // TEST_COND_H
