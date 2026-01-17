#include "test_union_struct_lib.h"
#include <stdio.h>

Value make_int_value(int val) {
    Value v;
    v.as_int = val;
    return v;
}

Value make_point_value(int x, int y) {
    Value v;
    v.as_point.x = x;
    v.as_point.y = y;
    return v;
}

Value make_color_value(float r, float g, float b) {
    Value v;
    printf("[C] make_color_value called with r=%f, g=%f, b=%f\n", r, g, b);
    v.as_color.r = r;
    v.as_color.g = g;
    v.as_color.b = b;
    printf("[C] Returning v.as_color.r=%f, g=%f, b=%f\n", v.as_color.r, v.as_color.g, v.as_color.b);
    return v;
}

int test_float_arg(float f) {
    printf("[C] test_float_arg called with f=%f\n", f);
    return (int)(f * 100.0f);
}

Color make_color_direct(float r, float g, float b) {
    Color c;
    c.r = r;
    c.g = g;
    c.b = b;
    return c;
}

void make_color_value_ptr(Value *out, float r, float g, float b) {
    out->as_color.r = r;
    out->as_color.g = g;
    out->as_color.b = b;
}
