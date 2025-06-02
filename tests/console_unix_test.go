// console_unix_test.go
// +build !windows

package main

import (
    "testing"
)

func TestGetSize_NoPanic(t *testing.T) {
    // Calling GetSize on fd=0 (stdin) may return an error if stdin isn't a TTY,
    // but it must not panic.
    cols, rows, err := GetSize(0)
    if err != nil {
        t.Logf("GetSize(0) returned error (likely not a TTY): %v", err)
    }
    // Columns and rows should be non-negative if no error; otherwise, ignore values.
    if err == nil && (cols < 0 || rows < 0) {
        t.Fatalf("GetSize(0) returned invalid dimensions: (%d, %d)", cols, rows)
    }
}

