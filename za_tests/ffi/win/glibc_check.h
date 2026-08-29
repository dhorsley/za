#ifndef ZA_GLIBC_CHECK_H
#define ZA_GLIBC_CHECK_H

#ifdef __GLIBC__
#define ZA_TEST_GLIBC_BRANCH 1
#else
#define ZA_TEST_GLIBC_BRANCH 0
#endif

#endif
