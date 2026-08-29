#ifndef ZA_FFI_TEST_H
#define ZA_FFI_TEST_H

/* Windows FFI test fixture header.
   Used by the MODULE ... AUTO test (test_win_auto.za). Kept as plain,
   portable prototypes so Za's AUTO header parser handles every declaration.
   (MinGW auto-exports all DLL symbols, so no __declspec is needed.)

   za_test_uint64_topbit and za_test_sum_n (varargs) are intentionally not
   declared here - the test scripts declare them via LIB statements. */

/* Macro/constant fixtures for the AUTO preprocessor tests. */
#define ZA_TEST_VALUE 42
#define ZA_TEST_SHIFT (1 << 4)

typedef struct {
    int x;
    int y;
} ZaPoint;

/* Phase 2 - simple native calls. */
int za_test_add(int a, int b);
double za_test_multiply(double a, double b);
const char *za_test_hello(void);
void *za_test_identity(void *p);

/* Phase 3 - ABI-sensitive calls. */
long long za_test_int64_echo(long long v);
float za_test_float_echo(float f);
double za_test_double_echo(double d);
double za_test_mixed(double a, int b, double c);
int za_test_string_len(const char *s);

/* Phase 4 - structs. */
ZaPoint za_test_make_point(int x, int y);
int za_test_point_sum(ZaPoint p);
int za_test_point_sum_ptr(ZaPoint *p);
int za_test_point_fill(ZaPoint *p, int x, int y);

/* Phase 6 - function pointers. */
typedef int (*za_int_fn_t)(int);
int za_test_plus1(int v);
za_int_fn_t za_test_get_plus1_ptr(void);
int za_test_apply(za_int_fn_t fn, int v);

/* Phase 7 - callbacks (C calls a Za-provided closure). */
int za_test_invoke_callback(za_int_fn_t fn, int value);

#endif /* ZA_FFI_TEST_H */