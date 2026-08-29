/* Windows FFI test fixture DLL.
   Exercises the hardcoded DLL call path, Win64 ABI, structs, variadics,
   function pointers and libffi closures (callbacks).
   MinGW auto-exports all symbols - no __declspec needed. */

#include "za_ffi_test.h"
#include <stdarg.h>
#include <string.h>

/* Phase 2 - simple native calls. */
int za_test_add(int a, int b) {
    return a + b;
}

double za_test_multiply(double a, double b) {
    return a * b;
}

const char *za_test_hello(void) {
    return "hello from za_ffi_test.dll";
}

void *za_test_identity(void *p) {
    return p;
}

/* Phase 3 - ABI-sensitive calls. */
long long za_test_int64_echo(long long v) {
    return v;
}

int za_test_uint64_topbit(unsigned long long v) {
    /* Return the top bit so uint64 handling is verified end to end. */
    return (v & 0x8000000000000000ULL) ? 1 : 0;
}

float za_test_float_echo(float f) {
    return f;
}

double za_test_double_echo(double d) {
    return d;
}

double za_test_mixed(double a, int b, double c) {
    return a + (double)b + c;
}

int za_test_string_len(const char *s) {
    return s == NULL ? -1 : (int)strlen(s);
}

/* Phase 4 - structs. */
ZaPoint za_test_make_point(int x, int y) {
    ZaPoint p;
    p.x = x;
    p.y = y;
    return p;
}

int za_test_point_sum(ZaPoint p) {
    return p.x + p.y;
}

int za_test_point_sum_ptr(ZaPoint *p) {
    if (p == NULL) {
        return -1;
    }
    return p->x + p->y;
}

int za_test_point_fill(ZaPoint *p, int x, int y) {
    if (p == NULL) {
        return -1;
    }
    p->x = x;
    p->y = y;
    return x + y;
}

/* Phase 5 - variadic calls. */
int za_test_sum_n(int count, ...) {
    va_list ap;
    int i, total = 0;
    va_start(ap, count);
    for (i = 0; i < count; i++) {
        total += va_arg(ap, int);
    }
    va_end(ap);
    return total;
}

/* Phase 6 - function pointers. */
int za_test_plus1(int v) {
    return v + 1;
}

za_int_fn_t za_test_get_plus1_ptr(void) {
    return za_test_plus1;
}

int za_test_apply(za_int_fn_t fn, int v) {
    return fn(v);
}

/* Phase 7 - callbacks (C calls a Za-provided closure). */
int za_test_invoke_callback(za_int_fn_t fn, int value) {
    return fn(value);
}