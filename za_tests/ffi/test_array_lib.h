#ifndef TEST_ARRAY_LIB_H
#define TEST_ARRAY_LIB_H

// Sum an array of integers
int sum_int_array(int *arr, int len);

// Average of float array
double average_float_array(double *arr, int len);

// Double all values in an int array (mutates in place)
void double_int_array(int *arr, int len);

// Double all values in a float array (mutates in place)
void double_float_array(double *arr, int len);

// Count bytes matching a value
int count_bytes(unsigned char *data, int len, unsigned char value);

// Sum uint8 values
int sum_uint8_array(unsigned char *arr, int len);

// Concatenate string array into buffer
int concat_strings(char **strings, int count, char *buffer, int bufsize);

// Count true values in bool array
int count_true_bool(unsigned char *bools, int len);

// Fill array with sequential values
void fill_sequence(int *arr, int len, int start);

#endif // TEST_ARRAY_LIB_H
