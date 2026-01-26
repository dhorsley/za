#include <stddef.h>
#include "test_funcptr_struct.h"

math_ops_t create_math_ops(int (*compare_fn)(int, int), int (*transform_fn)(int)) {
    math_ops_t ops;
    ops.compare = compare_fn;
    ops.transform = transform_fn;
    ops.name = "custom_ops";
    return ops;
}

int use_math_ops(math_ops_t *ops, int a, int b) {
    if (ops == NULL || ops->compare == NULL) {
        return -1;
    }
    return ops->compare(a, b);
}

system_t create_system(void (*init_fn)(void), void (*cleanup_fn)(void), math_ops_t *ops) {
    system_t sys;
    sys.init = init_fn;
    sys.cleanup = cleanup_fn;
    sys.ops = ops;
    return sys;
}
