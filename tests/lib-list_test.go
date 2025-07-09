package main

import (
    "testing"
)

type q struct {
    a int
    b string
}

func TestMultiSorted_SortsBySingleField(t *testing.T) {
    input := []any{
        q{a: 42, b: "test"},
        q{a: 3, b: "blah"},
        q{a: 1, b: "fooblah"},
    }

    sorted, err := MultiSorted(input, []string{"a"}, nil)
    if err != nil {
        t.Fatalf("MultiSorted failed: %v", err)
    }

    want := []int{1, 3, 42}
    for i, v := range sorted {
        qq := v.(q)
        if qq.a != want[i] {
            t.Errorf("Sort by 'a' failed at index %d: got %d, want %d", i, qq.a, want[i])
        }
    }
}

func TestMultiSorted_SortsByMultipleFields(t *testing.T) {
    input := []any{
        q{a: 3, b: "x"},
        q{a: 3, b: "a"},
        q{a: 1, b: "foo"},
        q{a: 2, b: "bar"},
    }

    sorted, err := MultiSorted(input, []string{"a", "b"}, nil)
    if err != nil {
        t.Fatalf("MultiSorted failed: %v", err)
    }

    want := []q{
        {1, "foo"},
        {2, "bar"},
        {3, "a"},
        {3, "x"},
    }

    for i, v := range sorted {
        qq := v.(q)
        if qq != want[i] {
            t.Errorf("Sort by 'a' then 'b' failed at index %d: got %+v, want %+v", i, qq, want[i])
        }
    }
}

func TestMultiSorted_DescendingSortOrder(t *testing.T) {
    input := []any{
        q{a: 1, b: "foo"},
        q{a: 2, b: "bar"},
        q{a: 3, b: "baz"},
    }

    sorted, err := MultiSorted(input, []string{"a"}, []bool{false})
    if err != nil {
        t.Fatalf("MultiSorted failed: %v", err)
    }

    want := []int{3, 2, 1}
    for i, v := range sorted {
        qq := v.(q)
        if qq.a != want[i] {
            t.Errorf("Descending sort by 'a' failed at index %d: got %d, want %d", i, qq.a, want[i])
        }
    }
}

func TestMultiSorted_FieldDoesNotExist(t *testing.T) {
    input := []any{
        q{a: 1, b: "x"},
        q{a: 2, b: "y"},
    }

    // Sort by a non-existent field "c"
    sorted, err := MultiSorted(input, []string{"c"}, nil)
    if err != nil {
        t.Fatalf("MultiSorted failed with non-existent field: %v", err)
    }

    // It should be no-op (original order unchanged)
    for i, v := range sorted {
        qq := v.(q)
        if qq != input[i] {
            t.Errorf("Sort with non-existent field changed order: got %+v, want %+v", qq, input[i])
        }
    }
}

func TestMultiSorted_EmptySlice(t *testing.T) {
    input := []any{}

    sorted, err := MultiSorted(input, []string{"a"}, nil)
    if err != nil {
        t.Fatalf("MultiSorted failed on empty slice: %v", err)
    }
    if len(sorted) != 0 {
        t.Errorf("Expected empty sorted slice, got %d elements", len(sorted))
    }
}

func TestMultiSorted_SliceWithNilValues(t *testing.T) {
    type qptr struct {
        a *int
        b string
    }

    one := 1
    two := 2

    input := []any{
        qptr{a: &two, b: "b"},
        qptr{a: nil, b: "a"},
        qptr{a: &one, b: "c"},
    }

    // Sort by "a" ascending (nil should be considered smallest or no-op)
    sorted, err := MultiSorted(input, []string{"a"}, nil)
    if err != nil {
        t.Fatalf("MultiSorted failed on nil fields: %v", err)
    }

    t.Logf("Sorted with nil values: %+v", sorted)
}

func TestMultiSorted_InputNotSlice(t *testing.T) {
    _, err := MultiSorted(123, []string{"a"}, nil)
    if err == nil {
        t.Fatal("MultiSorted should error on non-slice input")
    }
}

func TestMultiSorted_MismatchedSortKeysAndOrder(t *testing.T) {
    input := []any{
        q{a: 1, b: "x"},
        q{a: 2, b: "y"},
    }

    _, err := MultiSorted(input, []string{"a"}, []bool{true, false})
    if err == nil {
        t.Fatal("MultiSorted should error on mismatched sort keys and order lengths")
    }
}

