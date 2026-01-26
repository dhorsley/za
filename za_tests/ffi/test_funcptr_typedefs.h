#ifndef TEST_FUNCPTR_TYPEDEFS_H
#define TEST_FUNCPTR_TYPEDEFS_H

// Function pointer typedefs for testing AUTO import

// Standard comparator typedef (like qsort)
typedef int (*compar_t)(const void *, const void *);

// Simple binary operation
typedef int (*binary_op_t)(int, int);

// Unary operation
typedef int (*unary_op_t)(int);

// Pointer operation
typedef void* (*alloc_func_t)(int size);

// Destructor
typedef void (*free_func_t)(void *ptr);

// Generic callback with context
typedef void (*callback_t)(void *context);

// Data processor
typedef int (*process_data_t)(const char *data, int length);

// Functions that work with these typedefs
int qsort_like(void *base, int nmemb, compar_t compar);
int apply_binary_op(int a, int b, binary_op_t op);
int apply_unary_op(int x, unary_op_t op);

#endif
