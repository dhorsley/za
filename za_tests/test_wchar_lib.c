#include "test_wchar_lib.h"

// Return wchar_t size for verification
int get_wchar_size(void) {
    return sizeof(wchar_t);
}

// Simple wchar_t echo function
wchar_t wchar_echo(wchar_t c) {
    return c;
}

// Array test
void wchar_fill_array(wchar_t *arr, int len) {
    for (int i = 0; i < len; i++) {
        arr[i] = (wchar_t)(65 + i);  // 'A', 'B', 'C', ...
    }
}
