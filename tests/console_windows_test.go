// console_windows_test.go
// +build windows

package main

import "testing"

func TestGetRowCol_NoPanic(t *testing.T) {
    // Calling GetRowCol on fd=0 (stdin) again may return an error on non-Windows or if unavailable,
    // but it must not panic.
    col, row, err := GetRowCol(0)
    if err != nil {
        t.Logf("GetRowCol(0) returned error: %v", err)
    }
    // If no error, col and row should be >= 0
    if err == nil && (col < 0 || row < 0) {
        t.Fatalf("GetRowCol(0) returned invalid cursor position: (%d, %d)", col, row)
    }
}

