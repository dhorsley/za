#ifndef TEST_STRUCT_ARRAY_LIB_H
#define TEST_STRUCT_ARRAY_LIB_H

#include <stdint.h>

typedef struct {
    uint8_t rgb[3];
    uint8_t alpha;
} color_t;

typedef struct {
    int id;
    int values[5];
    int count;
} data_block_t;

// Function to create a color
color_t make_color(uint8_t r, uint8_t g, uint8_t b, uint8_t a);

// Function to get average RGB value
int get_avg_rgb(color_t c);

// Function to sum all values in a data block
int sum_data_block(data_block_t block);

// Function to create a data block
data_block_t make_data_block(int id, int v0, int v1, int v2, int v3, int v4);

#endif
