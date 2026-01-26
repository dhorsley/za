#ifndef TEST_INLINE_UNION_H
#define TEST_INLINE_UNION_H

// Test 1: Basic inline union
typedef struct {
    int type;
    union {
        int int_val;
        float float_val;
    } data;
} BasicInlineUnion;

// Test 2: Multiple inline unions
typedef struct {
    union {
        int x;
        float fx;
    } first;
    int middle;
    union {
        int y;
        float fy;
    } second;
} MultipleInlineUnions;

// Test 3: Inline struct (not union)
typedef struct {
    struct {
        int x;
        int y;
    } point;
    int z;
} InlineStructTest;

// Function declarations
BasicInlineUnion process_basic_union(BasicInlineUnion input);
int get_union_int_val(BasicInlineUnion input);
float get_union_float_val(BasicInlineUnion input);
BasicInlineUnion make_union_with_int(int type, int val);
BasicInlineUnion make_union_with_float(int type, float val);
MultipleInlineUnions process_multiple_unions(MultipleInlineUnions input);
InlineStructTest process_inline_struct(InlineStructTest input);

#endif
