#include "test_struct_array_lib.h"

color_t make_color(uint8_t r, uint8_t g, uint8_t b, uint8_t a) {
    color_t c;
    c.rgb[0] = r;
    c.rgb[1] = g;
    c.rgb[2] = b;
    c.alpha = a;
    return c;
}

int get_avg_rgb(color_t c) {
    return (c.rgb[0] + c.rgb[1] + c.rgb[2]) / 3;
}

int sum_data_block(data_block_t block) {
    int sum = 0;
    for (int i = 0; i < block.count && i < 5; i++) {
        sum += block.values[i];
    }
    return sum;
}

data_block_t make_data_block(int id, int v0, int v1, int v2, int v3, int v4) {
    data_block_t block;
    block.id = id;
    block.values[0] = v0;
    block.values[1] = v1;
    block.values[2] = v2;
    block.values[3] = v3;
    block.values[4] = v4;
    block.count = 5;
    return block;
}
