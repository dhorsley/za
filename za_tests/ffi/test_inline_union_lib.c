#include "test_inline_union_lib.h"
#include <stdio.h>

// Test function: Takes BasicInlineUnion, returns modified version
BasicInlineUnion process_basic_union(BasicInlineUnion input) {
    BasicInlineUnion result;
    result.type = input.type + 1;  // Increment type

    // Access the union field - we'll read it as int and double the value
    result.data.int_val = input.data.int_val * 2;

    return result;
}

// Test function: Get int value from union
int get_union_int_val(BasicInlineUnion input) {
    return input.data.int_val;
}

// Test function: Get float value from union
float get_union_float_val(BasicInlineUnion input) {
    return input.data.float_val;
}

// Test function: Create BasicInlineUnion with int value
BasicInlineUnion make_union_with_int(int type, int val) {
    BasicInlineUnion result;
    result.type = type;
    result.data.int_val = val;
    return result;
}

// Test function: Create BasicInlineUnion with float value
BasicInlineUnion make_union_with_float(int type, float val) {
    BasicInlineUnion result;
    result.type = type;
    result.data.float_val = val;
    return result;
}

// Test function: Process MultipleInlineUnions
MultipleInlineUnions process_multiple_unions(MultipleInlineUnions input) {
    MultipleInlineUnions result;
    result.first.x = input.first.x + 1;
    result.middle = input.middle * 2;
    result.second.y = input.second.y + 10;
    return result;
}

// Test function: Process InlineStructTest
InlineStructTest process_inline_struct(InlineStructTest input) {
    InlineStructTest result;
    result.point.x = input.point.x + 5;
    result.point.y = input.point.y + 10;
    result.z = input.z + 15;
    return result;
}
