#ifndef TEST_FUNCPTR_LIB_H
#define TEST_FUNCPTR_LIB_H

// Basic arithmetic operations
int add_ints(int a, int b);
int multiply_ints(int a, int b);
int subtract_ints(int a, int b);

// Function pointer type for binary operations
typedef int (*binary_op_t)(int, int);

// Functions that return function pointers
binary_op_t get_add_ptr(void);
binary_op_t get_multiply_ptr(void);
binary_op_t get_subtract_ptr(void);

// Pointer comparison function
typedef int (*ptr_compar_t)(const void *, const void *);
int ptr_compare_ints(const void *a, const void *b);
ptr_compar_t get_ptr_compare(void);

// Struct with function pointer field
typedef struct {
    int (*op)(int, int);
    const char *name;
} operation_t;

operation_t create_operation(int (*func)(int, int), const char *name);
int call_operation(operation_t *op, int a, int b);

#endif
