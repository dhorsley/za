/* Test header for union support */

#ifndef TEST_UNION_H
#define TEST_UNION_H

/* Simple union with different field types */
typedef union {
    int int_value;
    float float_value;
    unsigned char bytes[4];
} simple_union_t;

/* Union with larger types */
typedef union {
    long long int64_value;
    double double_value;
    unsigned char bytes[8];
} large_union_t;

/* Struct containing a union field (common pattern) */
typedef struct {
    int type_tag;
    union {
        int as_int;
        float as_float;
        char as_char;
    } data;
} tagged_union_t;

#endif /* TEST_UNION_H */
