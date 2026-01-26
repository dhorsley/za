#include <stdlib.h>

// Simple test functions for function pointer testing

// Basic arithmetic operations - to be called through function pointers
int add_ints(int a, int b) {
    return a + b;
}

int multiply_ints(int a, int b) {
    return a * b;
}

int subtract_ints(int a, int b) {
    return a - b;
}

// Function that returns a function pointer
typedef int (*binary_op_t)(int, int);

binary_op_t get_add_ptr(void) {
    return add_ints;
}

binary_op_t get_multiply_ptr(void) {
    return multiply_ints;
}

binary_op_t get_subtract_ptr(void) {
    return subtract_ints;
}

// Pointer operations
typedef int (*ptr_compar_t)(const void *, const void *);

int ptr_compare_ints(const void *a, const void *b) {
    int ia = *(const int *)a;
    int ib = *(const int *)b;
    if (ia < ib) return -1;
    if (ia > ib) return 1;
    return 0;
}

ptr_compar_t get_ptr_compare(void) {
    return ptr_compare_ints;
}

// Struct with function pointer field
typedef struct {
    int (*op)(int, int);
    const char *name;
} operation_t;

operation_t create_operation(int (*func)(int, int), const char *name) {
    operation_t result;
    result.op = func;
    result.name = name;
    return result;
}

int call_operation(operation_t *op, int a, int b) {
    if (op == NULL || op->op == NULL) {
        return -1;
    }
    return op->op(a, b);
}
