// constants_test.go
package main

import "testing"

func TestImportantConstantsAreNonZero(t *testing.T) {
    if MAX_LOOPS <= 0 {
        t.Fatalf("MAX_LOOPS should be >0, got %d", MAX_LOOPS)
    }
    if identInitialSize <= 0 {
        t.Fatalf("identInitialSize should be >0, got %d", identInitialSize)
    }
    if CALL_CAP <= 0 {
        t.Fatalf("CALL_CAP should be >0, got %d", CALL_CAP)
    }
    if MAX_CLIENTS <= 0 {
        t.Fatalf("MAX_CLIENTS should be >0, got %d", MAX_CLIENTS)
    }
}

