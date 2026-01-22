#include "test_array_lib.h"
#include <string.h>

// Sum an array of integers
int sum_int_array(int *arr, int len) {
    int sum = 0;
    for (int i = 0; i < len; i++) {
        sum += arr[i];
    }
    return sum;
}

// Average of float array
double average_float_array(double *arr, int len) {
    if (len == 0) return 0.0;
    double sum = 0.0;
    for (int i = 0; i < len; i++) {
        sum += arr[i];
    }
    return sum / len;
}

// Double all values in an int array (mutates in place)
void double_int_array(int *arr, int len) {
    for (int i = 0; i < len; i++) {
        arr[i] = arr[i] * 2;
    }
}

// Double all values in a float array (mutates in place)
void double_float_array(double *arr, int len) {
    for (int i = 0; i < len; i++) {
        arr[i] = arr[i] * 2.0;
    }
}

// Count bytes matching a value
int count_bytes(unsigned char *data, int len, unsigned char value) {
    int count = 0;
    for (int i = 0; i < len; i++) {
        if (data[i] == value) count++;
    }
    return count;
}

// Sum uint8 values
int sum_uint8_array(unsigned char *arr, int len) {
    int sum = 0;
    for (int i = 0; i < len; i++) {
        sum += arr[i];
    }
    return sum;
}

// Concatenate string array into buffer
int concat_strings(char **strings, int count, char *buffer, int bufsize) {
    int pos = 0;
    for (int i = 0; i < count && pos < bufsize; i++) {
        const char *str = strings[i];
        int len = strlen(str);
        if (pos + len + 1 > bufsize) break;
        if (pos > 0) {
            buffer[pos++] = ' ';
        }
        strcpy(buffer + pos, str);
        pos += len;
    }
    return pos;
}

// Count true values in bool array
int count_true_bool(unsigned char *bools, int len) {
    int count = 0;
    for (int i = 0; i < len; i++) {
        if (bools[i]) count++;
    }
    return count;
}

// Fill array with sequential values
void fill_sequence(int *arr, int len, int start) {
    for (int i = 0; i < len; i++) {
        arr[i] = start + i;
    }
}
