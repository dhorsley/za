package main

import (
    "fmt"
    "reflect"
)

// checkShape verifies that two reflect.Values that are slices have identical
// dimensions at every depth. It returns an error if a mismatch is found,
// ensuring that element‑wise operations only run on rectangular arrays.
func checkShape(a, b reflect.Value) error {
    if a.Kind() != reflect.Slice || b.Kind() != reflect.Slice {
        return fmt.Errorf("checkShape: both arguments must be slices")
    }

    if a.Len() != b.Len() { // don't break on empty arrays
        return fmt.Errorf("shape mismatch: outer dimensions differ (%d vs %d)", a.Len(), b.Len())
    }

    for i := 0; i < a.Len(); i++ {
        ai := a.Index(i)
        bi := b.Index(i)

        // If both are slices, recurse
        if ai.Kind() == reflect.Slice && bi.Kind() == reflect.Slice {
            if err := checkShape(ai, bi); err != nil {
                return err
            }
        } else if ai.Kind() == reflect.Slice || bi.Kind() == reflect.Slice {
            // One is slice, other is not - shape mismatch
            return fmt.Errorf("shape mismatch at index %d: one side is slice, other is scalar", i)
        }
        // If neither is slice, they're leaf elements - no further checking needed
    }

    return nil
}

// isSlice checks if a value is a slice (including nested slices)
func isSlice(v any) bool {
    return reflect.TypeOf(v).Kind() == reflect.Slice
}

// getSliceDimensions returns the dimensions of a nested slice as a slice of ints
func getSliceDimensions(v any) []int {
    var dims []int
    rv := reflect.ValueOf(v)
    for rv.Kind() == reflect.Slice {
        dims = append(dims, rv.Len())
        if rv.Len() > 0 {
            rv = rv.Index(0)
        } else {
            break
        }
    }
    return dims
}

// ApplyElementwiseBinaryOp recursively applies a binary scalar operation
// to two operands that may be scalars, slices, or a mix of both.
//
//  a, b – the operands (any Go value)
//  op   – a function that takes two scalar values and returns the result.
//
// The function:
//  1. Checks whether a and b are slices using reflection.
//  2. If both are slices, verifies that their shapes match (using checkShape).
//  3. Allocates a new slice of the same concrete element type.
//  4. Recursively processes each element pair, calling op on leaf scalars.
//  5. If one operand is a scalar and the other a slice, broadcasts the scalar.
//  6. Returns the newly‑created slice (or the scalar result when both are scalars).
func ApplyElementwiseBinaryOp(a, b any, op func(x, y any) any) any {
    // Handle scalar-scalar case
    if !isSlice(a) && !isSlice(b) {
        return op(a, b)
    }

    // Handle scalar-slice cases (broadcasting)
    if !isSlice(a) && isSlice(b) {
        return broadcastScalarToSlice(a, b, op)
    }
    if isSlice(a) && !isSlice(b) {
        return broadcastScalarToSlice(b, a, func(x, y any) any { return op(y, x) })
    }

    // Handle slice-slice case
    ra := reflect.ValueOf(a)
    rb := reflect.ValueOf(b)

    // Check shape compatibility
    if err := checkShape(ra, rb); err != nil {
        panic(fmt.Errorf("shape mismatch: %v", err))
    }

    // Determine the result type by performing a sample operation
    var resultElemType reflect.Type
    if ra.Len() > 0 {
        sampleA := ra.Index(0).Interface()
        sampleB := rb.Index(0).Interface()
        sampleResult := ApplyElementwiseBinaryOp(sampleA, sampleB, op)
        resultElemType = reflect.TypeOf(sampleResult)
    } else {
        // For empty slices, use the first slice's element type as fallback
        resultElemType = reflect.TypeOf(a).Elem()
    }

    resultType := reflect.SliceOf(resultElemType)
    result := reflect.MakeSlice(resultType, ra.Len(), ra.Len())

    for i := 0; i < ra.Len(); i++ {
        ai := ra.Index(i).Interface()
        bi := rb.Index(i).Interface()
        resultElem := ApplyElementwiseBinaryOp(ai, bi, op)
        result.Index(i).Set(reflect.ValueOf(resultElem))
    }

    return result.Interface()
}

// broadcastScalarToSlice applies a scalar operation to each element of a slice
func broadcastScalarToSlice(scalar, slice any, op func(x, y any) any) any {
    rs := reflect.ValueOf(slice)

    // Determine the result type by performing a sample operation
    var resultElemType reflect.Type
    if rs.Len() > 0 {
        sampleElem := rs.Index(0).Interface()
        sampleResult := op(scalar, sampleElem)
        resultElemType = reflect.TypeOf(sampleResult)
    } else {
        // For empty slices, use the slice element type as fallback
        resultElemType = reflect.TypeOf(slice).Elem()
    }

    resultType := reflect.SliceOf(resultElemType)
    result := reflect.MakeSlice(resultType, rs.Len(), rs.Len())

    for i := 0; i < rs.Len(); i++ {
        elem := rs.Index(i).Interface()
        resultElem := op(scalar, elem)
        result.Index(i).Set(reflect.ValueOf(resultElem))
    }

    return result.Interface()
}
