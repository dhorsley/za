// eval_ops_test.go
package main

import (
    "math/big"
    "testing"
)

func TestSliceOnString(t *testing.T) {
    // slice("hello", 1, 4) → "ell"
    out := slice("hello", 1, 4)
    str, ok := out.(string)
    if !ok {
        t.Fatalf("expected string, got %T", out)
    }
    if str != "ell" {
        t.Fatalf("slice(\"hello\",1,4) expected \"ell\", got %q", str)
    }
}

func TestSliceOnBoolArray(t *testing.T) {
    bArr := []bool{true, false, true}
    out := slice(bArr, 1, 3)
    arr, ok := out.([]bool)
    if !ok {
        t.Fatalf("expected []bool, got %T", out)
    }
    // Now expect [false, true], since elements at indices 1 and 2 are false, true
    if len(arr) != 2 || arr[0] != false || arr[1] != true {
        t.Fatalf("slice([]bool{true,false,true},1,3) expected [false,true], got %v", arr)
    }
}

func TestSliceOnIntArray(t *testing.T) {
    iArr := []int{1, 2, 3, 4}
    out := slice(iArr, 1, 3)
    arr, ok := out.([]int)
    if !ok {
        t.Fatalf("expected []int, got %T", out)
    }
    if len(arr) != 2 || arr[0] != 2 || arr[1] != 3 {
        t.Fatalf("slice([]int{1,2,3,4},1,3) expected [2,3], got %v", arr)
    }
}

func TestSliceOnBigIntArray(t *testing.T) {
    bi1 := big.NewInt(5)
    bi2 := big.NewInt(10)
    bArr := []*big.Int{bi1, bi2}
    out := slice(bArr, 0, 1)
    arr, ok := out.([]*big.Int)
    if !ok {
        t.Fatalf("expected []*big.Int, got %T", out)
    }
    if len(arr) != 1 || arr[0].Cmp(bi1) != 0 {
        t.Fatalf("slice([]*big.Int{5,10},0,1) expected [5], got %v", arr)
    }
}

