#ifndef ZA_LLP64_CHECK_H
#define ZA_LLP64_CHECK_H

/* LLP64: long is 32-bit on Win64, so __LP64__ must not be defined. */
#ifdef __LP64__
#define ZA_LONG_SIZE 8
#else
#define ZA_LONG_SIZE 4
#endif

#endif
