#ifndef TEST_UNION_LIB_H
#define TEST_UNION_LIB_H

#include <stdint.h>

typedef union {
    int int_value;
    float float_value;
    unsigned char bytes[4];
} test_union_t;

int get_union_as_int(test_union_t u);
float get_union_as_float(test_union_t u);
test_union_t make_union_from_int(int value);
test_union_t make_union_from_float(double value);
unsigned char get_first_byte(test_union_t u);

#endif
