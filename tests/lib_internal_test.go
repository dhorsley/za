package main

import (
    "reflect"
    "testing"
)

func TestCheckShape(t *testing.T) {
    // Test matching 2D slices
    a := [][]int{{1, 2}, {3, 4}}
    b := [][]int{{5, 6}, {7, 8}}
    ra := reflect.ValueOf(a)
    rb := reflect.ValueOf(b)

    if err := checkShape(ra, rb); err != nil {
        t.Errorf("checkShape failed for matching 2D slices: %v", err)
    }

    // Test mismatched 2D slices
    c := [][]int{{1, 2, 3}, {4, 5, 6}}
    rc := reflect.ValueOf(c)
    if err := checkShape(ra, rc); err == nil {
        t.Error("checkShape should have failed for mismatched 2D slices")
    }

    // Test matching 3D slices
    d := [][][]int{{{1}, {2}}, {{3}, {4}}}
    e := [][][]int{{{5}, {6}}, {{7}, {8}}}
    rd := reflect.ValueOf(d)
    re := reflect.ValueOf(e)

    if err := checkShape(rd, re); err != nil {
        t.Errorf("checkShape failed for matching 3D slices: %v", err)
    }
}

func TestApplyElementwiseBinaryOp(t *testing.T) {
    // Test scalar addition
    result := ApplyElementwiseBinaryOp(2, 3, func(x, y any) any {
        return x.(int) + y.(int)
    })
    if result != 5 {
        t.Errorf("Expected 5, got %v", result)
    }

    // Test 1D slice addition
    a := []int{1, 2, 3}
    b := []int{4, 5, 6}
    result = ApplyElementwiseBinaryOp(a, b, func(x, y any) any {
        return x.(int) + y.(int)
    })
    expected := []int{5, 7, 9}
    if !reflect.DeepEqual(result, expected) {
        t.Errorf("Expected %v, got %v", expected, result)
    }

    // Test 2D slice addition
    c := [][]int{{1, 2}, {3, 4}}
    d := [][]int{{5, 6}, {7, 8}}
    result = ApplyElementwiseBinaryOp(c, d, func(x, y any) any {
        return x.(int) + y.(int)
    })
    expected2D := [][]int{{6, 8}, {10, 12}}
    if !reflect.DeepEqual(result, expected2D) {
        t.Errorf("Expected %v, got %v", expected2D, result)
    }

    // Test scalar broadcasting
    e := []int{1, 2, 3}
    result = ApplyElementwiseBinaryOp(5, e, func(x, y any) any {
        return x.(int) + y.(int)
    })
    expectedBroadcast := []int{6, 7, 8}
    if !reflect.DeepEqual(result, expectedBroadcast) {
        t.Errorf("Expected %v, got %v", expectedBroadcast, result)
    }
}

func TestIsSlice(t *testing.T) {
    if isSlice(5) {
        t.Error("isSlice should return false for scalar")
    }
    if !isSlice([]int{1, 2, 3}) {
        t.Error("isSlice should return true for slice")
    }
    if !isSlice([][]int{{1, 2}, {3, 4}}) {
        t.Error("isSlice should return true for nested slice")
    }
}

func TestGetSliceDimensions(t *testing.T) {
    // Test 1D slice
    a := []int{1, 2, 3}
    dims := getSliceDimensions(a)
    expected := []int{3}
    if !reflect.DeepEqual(dims, expected) {
        t.Errorf("Expected %v, got %v", expected, dims)
    }

    // Test 2D slice
    b := [][]int{{1, 2}, {3, 4}}
    dims = getSliceDimensions(b)
    expected = []int{2, 2}
    if !reflect.DeepEqual(dims, expected) {
        t.Errorf("Expected %v, got %v", expected, dims)
    }

    // Test 3D slice
    c := [][][]int{{{1}, {2}}, {{3}, {4}}}
    dims = getSliceDimensions(c)
    expected = []int{2, 2, 1}
    if !reflect.DeepEqual(dims, expected) {
        t.Errorf("Expected %v, got %v", expected, dims)
    }
}
