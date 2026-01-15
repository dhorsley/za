/* Level 2 header that includes level1 */

#ifndef TEST_INCLUDE_LEVEL2_H
#define TEST_INCLUDE_LEVEL2_H

#include "test_include_level1.h"

#define LEVEL2_CONSTANT 200
#define COMBINED_VALUE (LEVEL1_CONSTANT + LEVEL2_CONSTANT)

#endif /* TEST_INCLUDE_LEVEL2_H */
