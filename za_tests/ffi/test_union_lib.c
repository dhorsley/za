#include <stdint.h>

typedef union {
    int int_value;
    float float_value;
    unsigned char bytes[4];
} test_union_t;

// Function that takes a union and returns the int interpretation
int get_union_as_int(test_union_t u) {
    return u.int_value;
}

// Function that takes a union and returns the float interpretation
float get_union_as_float(test_union_t u) {
    return u.float_value;
}

// Function that creates a union from an int
test_union_t make_union_from_int(int value) {
    test_union_t u;
    u.int_value = value;
    return u;
}

// Function that creates a union from a float
// Note: Za passes floating point literals as double, so we accept double and cast to float
test_union_t make_union_from_float(double value) {
    test_union_t u;
    u.float_value = (float)value;
    return u;
}

// Function to get first byte
unsigned char get_first_byte(test_union_t u) {
    return u.bytes[0];
}
