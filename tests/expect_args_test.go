// expect_args_test.go
package main

import (
    "math/big"
    "testing"
)

func TestExpectArgsSuccess(t *testing.T) {
    // Case 1: no args, no variants → should succeed
    ok, err := expect_args("foo", []any{}, 0)
    if !ok || err != nil {
        t.Fatalf("expect_args(\"foo\", [], 0) expected (true, nil), got (%v, %v)", ok, err)
    }

    // Case 2: two numeric arguments
    args := []any{3, 4.5}
    ok, err = expect_args("add", args, 1, "2", "number", "number")
    if !ok || err != nil {
        t.Fatalf("expect_args(\"add\", [3, 4.5], 1, \"2\", \"number\", \"number\") failed: %v", err)
    }

    // Case 3: big‐int variant
    bigVal := big.NewInt(42)
    ok, err = expect_args("bigcalc", []any{bigVal}, 1, "1", "bignumber")
    if !ok || err != nil {
        t.Fatalf("expect_args(\"bigcalc\", [*big.Int], 1, \"1\", \"bignumber\") failed: %v", err)
    }

    // Case 4: slice variant
    sliceArg := []any{1, "x", true}
    ok, err = expect_args("scan", []any{sliceArg}, 1, "1", "[]any")
    if !ok || err != nil {
        t.Fatalf("expect_args(\"scan\", [[]any], 1, \"1\", \"[]any\") failed: %v", err)
    }
}

func TestExpectArgsFailure(t *testing.T) {
    // Wrong count
    ok, err := expect_args("foo", []any{1}, 1, "2", "number", "number")
    if ok || err == nil {
        t.Fatalf("expect_args should fail on wrong arg count, got (%v, %v)", ok, err)
    }

    // Nil value inside args
    args := []any{nil}
    ok, err = expect_args("foo", args, 1, "1", "number")
    if ok || err == nil {
        t.Fatalf("expect_args should reject nil value, got (%v, %v)", ok, err)
    }

    // Type mismatch
    args2 := []any{1, "not a number"}
    ok, err = expect_args("foo", args2, 1, "2", "number", "number")
    if ok || err == nil {
        t.Fatalf("expect_args should reject wrong type, got (%v, %v)", ok, err)
    }
}

