#ifndef TEST_FUNCPTR_STRUCT_H
#define TEST_FUNCPTR_STRUCT_H

// Struct with function pointer fields - vtable-style API

typedef struct {
    // Compare function: returns -1, 0, or 1
    int (*compare)(int a, int b);

    // Transform function: applies a transformation
    int (*transform)(int x);

    // Name of the operation
    const char *name;
} math_ops_t;

// Create a math operations struct
math_ops_t create_math_ops(int (*compare_fn)(int, int), int (*transform_fn)(int));

// Use a math operations struct
int use_math_ops(math_ops_t *ops, int a, int b);

// Nested struct with function pointers
typedef struct {
    void (*init)(void);
    void (*cleanup)(void);
    math_ops_t *ops;
} system_t;

system_t create_system(void (*init_fn)(void), void (*cleanup_fn)(void), math_ops_t *ops);

#endif
