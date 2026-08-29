#ifndef ZA_MINGW_CHECK_H
#define ZA_MINGW_CHECK_H

#ifdef __MINGW32__
#define ZA_TEST_MINGW32 1
#else
#define ZA_TEST_MINGW32 0
#endif

#ifdef __MINGW64__
#define ZA_TEST_MINGW64 1
#else
#define ZA_TEST_MINGW64 0
#endif

#endif
