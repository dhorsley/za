#include <stddef.h>
#include "test_funcptr_typedefs.h"

// qsort_like - simple wrapper around memcmp style comparison
int qsort_like(void *base, int nmemb, compar_t compar) {
    if (base == NULL || compar == NULL) {
        return -1;
    }
    // Just return the comparison of first two elements
    if (nmemb >= 2) {
        int *arr = (int *)base;
        return compar(&arr[0], &arr[1]);
    }
    return 0;
}

// apply_binary_op - applies a binary operation
int apply_binary_op(int a, int b, binary_op_t op) {
    if (op == NULL) {
        return -1;
    }
    return op(a, b);
}

// apply_unary_op - applies a unary operation
int apply_unary_op(int x, unary_op_t op) {
    if (op == NULL) {
        return -1;
    }
    return op(x);
}
