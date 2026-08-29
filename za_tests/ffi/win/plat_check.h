#ifndef ZA_PLAT_CHECK_H
#define ZA_PLAT_CHECK_H

#ifdef _WIN32
#define ZA_PLATFORM_WINDOWS 1
#else
#define ZA_PLATFORM_WINDOWS 0
#endif

#ifdef _WIN64
#define ZA_PLATFORM_WIN64 1
#else
#define ZA_PLATFORM_WIN64 0
#endif

#ifdef __unix__
#define ZA_PLATFORM_UNIX 1
#else
#define ZA_PLATFORM_UNIX 0
#endif

#ifdef __LP64__
#define ZA_PLATFORM_LP64 1
#else
#define ZA_PLATFORM_LP64 0
#endif

#ifdef __GLIBC__
#define ZA_PLATFORM_GLIBC 1
#else
#define ZA_PLATFORM_GLIBC 0
#endif

#endif
