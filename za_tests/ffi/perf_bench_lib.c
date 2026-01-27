/*
 * Performance Benchmark Library for Za FFI
 * Minimal C functions designed to isolate FFI overhead
 */

#include <string.h>
#include <stdlib.h>

/* ===== SIMPLE INTEGER FUNCTIONS (minimal C work) ===== */

/* Baseline: identity function - measures pure FFI overhead */
int bench_identity_int(int x) {
    return x;
}

/* Return hardcoded value - tests return value handling */
int bench_const_int(void) {
    return 42;
}

/* Simple arithmetic - minimal C work */
int bench_add_int(int a, int b) {
    return a + b;
}

/* Two parameters, two returns: tests parameter marshaling cost */
int bench_sum_two_ints(int a, int b) {
    return a + b;
}

/* Five integer parameters - tests marshaling of multiple args */
int bench_sum_five_ints(int a, int b, int c, int d, int e) {
    return a + b + c + d + e;
}

/* ===== FLOAT FUNCTIONS ===== */

/* Float identity - baseline for float overhead */
double bench_identity_float(double x) {
    return x;
}

/* Return float constant */
double bench_const_float(void) {
    return 3.14159;
}

/* Float arithmetic */
double bench_add_float(double a, double b) {
    return a + b;
}

/* Mixed int and float parameters */
int bench_mixed_types(int i, double f, int j) {
    return i + (int)f + j;
}

/* ===== POINTER/STRING FUNCTIONS ===== */

/* String length - tests string marshaling */
int bench_strlen(const char *str) {
    int len = 0;
    while (str[len]) len++;
    return len;
}

/* String copy - tests string allocation and copying */
void bench_strcpy_fixed(const char *src, char *dst) {
    while (*src) {
        *dst++ = *src++;
    }
    *dst = 0;
}

/* Void pointer identity - tests pointer handling */
void* bench_ptr_identity(void *p) {
    return p;
}

/* ===== STRUCT FUNCTIONS ===== */

typedef struct {
    int x;
    int y;
} Point;

/* Pass struct by value - tests struct marshaling */
int bench_struct_sum(Point p) {
    return p.x + p.y;
}

/* Struct with modification - tests struct copy cost */
Point bench_struct_add_offset(Point p, int offset) {
    Point result;
    result.x = p.x + offset;
    result.y = p.y + offset;
    return result;
}

typedef struct {
    double a;
    double b;
    double c;
} FloatTriple;

/* Larger struct - tests marshaling cost at scale */
double bench_float_triple_sum(FloatTriple t) {
    return t.a + t.b + t.c;
}

/* ===== ARRAY FUNCTIONS ===== */

/* Array sum - tests array iteration overhead */
int bench_sum_array(int *arr, int len) {
    int sum = 0;
    for (int i = 0; i < len; i++) {
        sum += arr[i];
    }
    return sum;
}

/* Array iteration without computation - pure overhead test */
void bench_iterate_array(int *arr, int len) {
    for (int i = 0; i < len; i++) {
        (void)arr[i];  /* Use variable to prevent optimization */
    }
}

/* Count array elements matching value */
int bench_count_value(int *arr, int len, int value) {
    int count = 0;
    for (int i = 0; i < len; i++) {
        if (arr[i] == value) count++;
    }
    return count;
}

/* ===== MEMORY OPERATIONS ===== */

/* Allocate and initialize - tests memory allocation overhead */
int* bench_alloc_init_array(int size, int value) {
    int *arr = (int *)malloc(size * sizeof(int));
    for (int i = 0; i < size; i++) {
        arr[i] = value;
    }
    return arr;
}

/* Free allocated memory */
void bench_free_array(int *arr) {
    free(arr);
}

/* ===== VOID FUNCTIONS ===== */

/* No parameters, no return - tests minimal overhead */
void bench_noop(void) {
    /* Do nothing */
}

/* Parameter but no return - tests parameter-only overhead */
void bench_consume_int(int x) {
    (void)x;  /* Use variable to prevent optimization */
}

/* ===== VARIADIC-LIKE (simulated with fixed args) ===== */

/* Simulate variadic: count non-zero arguments */
int bench_count_nonzero(int a, int b, int c, int d, int e) {
    int count = 0;
    if (a != 0) count++;
    if (b != 0) count++;
    if (c != 0) count++;
    if (d != 0) count++;
    if (e != 0) count++;
    return count;
}

/* ===== STRESS TEST FUNCTIONS ===== */

/* Very simple tight loop - tests absolute minimum latency */
int bench_tight_loop_noop(int iterations) {
    for (int i = 0; i < iterations; i++) {
        /* Empty loop body */
    }
    return iterations;
}

/* Simple accumulation in tight loop */
int bench_tight_loop_add(int iterations) {
    int sum = 0;
    for (int i = 0; i < iterations; i++) {
        sum += i;
    }
    return sum;
}
