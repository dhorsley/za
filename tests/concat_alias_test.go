package main

// Regression tests for the concat/append slice-aliasing hazard.
//
// za's concat() is documented as pure ("Concatenates two lists and returns
// the result"), but was implemented as a bare Go append: when the first
// argument had spare capacity the append wrote in place, silently clobbering
// live slices sharing the backing. In particular
// concat(concat(A,B),C) corrupted C whenever C aliased A's backing and the
// inner append fit capacity (eager slice-header evaluation + in-place
// write). concat() and 2-arg append() must now always return fresh backing.

import (
    "testing"
)

func init() {
    buildStandardLib()
}

// makeBackedFloat32 returns n sequential floats with extra capacity, so
// appends fit in place (the precondition for the old corruption).
func makeBackedFloat32(n int) []float32 {
    s := make([]float32, n, n+64)
    for i := range s {
        s[i] = float32(i)
    }
    return s
}

func callConcat(t *testing.T, a, b any) any {
    t.Helper()
    fn, ok := stdlib["concat"]
    if !ok {
        t.Fatal("stdlib has no concat")
    }
    ret, err := fn("", 0, nil, a, b)
    if err != nil {
        t.Fatalf("concat failed: %v", err)
    }
    return ret
}

func TestConcatNestedAliased(t *testing.T) {
    base := makeBackedFloat32(100)
    quad := []float32{1000, 1001, 1002, 1003}
    a := base[0:20]
    c := base[20:]
    inner, err := stdlib["concat"]("", 0, nil, a, quad)
    if err != nil {
        t.Fatalf("inner concat failed: %v", err)
    }
    got := callConcat(t, inner, c).([]float32)
    if len(got) != 104 {
        t.Fatalf("len = %d, want 104", len(got))
    }
    for i := 0; i < 20; i++ {
        if got[i] != float32(i) {
            t.Fatalf("prefix mismatch at %d: got %v want %v", i, got[i], i)
        }
    }
    for i := 0; i < 4; i++ {
        if got[20+i] != quad[i] {
            t.Fatalf("insert mismatch at %d: got %v want %v", 20+i, got[20+i], quad[i])
        }
    }
    for i := 0; i < 80; i++ {
        if got[24+i] != float32(20+i) {
            t.Fatalf("suffix mismatch at %d: got %v want %v", 24+i, got[24+i], 20+i)
        }
    }
    // source backing must be untouched
    for i := 0; i < 100; i++ {
        if base[i] != float32(i) {
            t.Fatalf("source clobbered at %d: got %v want %v", i, base[i], i)
        }
    }
}

func TestConcatTempVarForm(t *testing.T) {
    // Same hazard with the calls split across statements (no nesting).
    base := makeBackedFloat32(100)
    quad := []float32{1000, 1001, 1002, 1003}
    inner := callConcat(t, base[0:20], quad)
    got := callConcat(t, inner, base[20:]).([]float32)
    if len(got) != 104 {
        t.Fatalf("len = %d, want 104", len(got))
    }
    if got[20] != 1000 || got[23] != 1003 {
        t.Fatalf("insert corrupted: %v %v", got[20], got[23])
    }
    if got[24] != 20 {
        t.Fatalf("suffix head corrupted: got %v want 20", got[24])
    }
    for i := 0; i < 100; i++ {
        if base[i] != float32(i) {
            t.Fatalf("source clobbered at %d: got %v want %v", i, base[i], i)
        }
    }
}

func TestConcatReadAfterAppend(t *testing.T) {
    // Non-nested form: reading a[2:] after y = concat(a[0:2], big).
    base := makeBackedFloat32(10)
    big := []float32{50, 51, 52, 53, 54, 55, 56, 57}
    y := callConcat(t, base[0:2], big).([]float32)
    if len(y) != 10 || y[0] != 0 || y[2] != 50 || y[9] != 57 {
        t.Fatalf("concat result wrong: %v", y)
    }
    for i := 0; i < 10; i++ {
        if base[i] != float32(i) {
            t.Fatalf("source clobbered at %d: got %v want %v", i, base[i], i)
        }
    }
}

func TestAppendAliasedPrefix(t *testing.T) {
    // 2-arg append onto a short prefix must not clobber a longer live view.
    base := makeBackedFloat32(10)
    fn, ok := stdlib["append"]
    if !ok {
        t.Fatal("stdlib has no append")
    }
    ret, err := fn("", 0, nil, base[0:3], float32(99))
    if err != nil {
        t.Fatalf("append failed: %v", err)
    }
    got := ret.([]float32)
    if len(got) != 4 || got[3] != 99 {
        t.Fatalf("append result wrong: %v", got)
    }
    for i := 0; i < 10; i++ {
        if base[i] != float32(i) {
            t.Fatalf("source clobbered at %d: got %v want %v", i, base[i], i)
        }
    }
}

func TestConcatAnyType(t *testing.T) {
    // Same guarantee for []any.
    base := make([]any, 10, 74)
    for i := range base {
        base[i] = i
    }
    inner := callConcat(t, base[0:2], []any{100, 101})
    got := callConcat(t, inner, base[2:]).([]any)
    if len(got) != 12 {
        t.Fatalf("len = %d, want 12", len(got))
    }
    want := []any{0, 1, 100, 101, 2, 3, 4, 5, 6, 7, 8, 9}
    for i, w := range want {
        if got[i] != w {
            t.Fatalf("mismatch at %d: got %v want %v", i, got[i], w)
        }
    }
    for i := 0; i < 10; i++ {
        if base[i] != i {
            t.Fatalf("source clobbered at %d: got %v want %v", i, base[i], i)
        }
    }
}
