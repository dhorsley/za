/* Main header that includes types */

#ifndef TEST_INCLUDE_MAIN_H
#define TEST_INCLUDE_MAIN_H

#include "test_include_types.h"

#define MAIN_FEATURE_ENABLED 1
#define MAIN_BUFFER_SIZE (TYPES_BUFFER_SIZE + 128)

/* This should get BASE_CONSTANT from base.h via types.h */
#define DERIVED_VALUE (BASE_CONSTANT + 500)

#ifdef MAIN_FEATURE_ENABLED
#define CONDITIONAL_VALUE 42
#endif

#endif /* TEST_INCLUDE_MAIN_H */
