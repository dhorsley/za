package main

import (
    "errors"
    "fmt"
    "math"
    "math/big"
    "sort"
)

func buildArrayLib() {
    features["array"] = Feature{version: 1, category: "data"}
    categories["array"] = []string{
        "reshape", "zeros", "ones", "flatten", "mean",
        "median", "std", "variance", "identity", "trace",
        "argmax", "argmin", "find", "where", "stack",
        "concatenate", "squeeze", "det", "det_big", "inverse", "inverse_big", "rank",
    }

    // reshape() - Change array dimensions
    slhelp["reshape"] = LibHelp{in: "array,new_dims", out: "array", action: "Reshape array to new dimensions. Total elements must remain constant."}
    stdlib["reshape"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("reshape", args, 2, "2", "any", "any"); !ok {
            return nil, err
        }

        array := args[0]
        newDims := args[1]

        // Validate that array is actually a slice
        if !isSlice(array) {
            return nil, errors.New("reshape: first argument must be an array")
        }

        // Handle new_dims as either slice or individual arguments
        var dims []int
        switch newDims.(type) {
        case []int:
            dims = newDims.([]int)
        case []any:
            dimSlice := newDims.([]any)
            dims = make([]int, len(dimSlice))
            for i, d := range dimSlice {
                if dimInt, ok := d.(int); ok {
                    dims[i] = dimInt
                } else {
                    return nil, errors.New("reshape: dimensions must be integers")
                }
            }
        default:
            return nil, errors.New("reshape: second argument must be array of dimensions")
        }

        // Calculate total elements in original array
        originalFlat := flattenSlice(array)
        originalCount := len(originalFlat)

        // Calculate required elements for new dimensions
        requiredCount := 1
        for _, dim := range dims {
            if dim < 0 {
                return nil, errors.New("reshape: dimensions must be non-negative")
            }
            requiredCount *= dim
        }

        // Validate element count preservation
        if originalCount != requiredCount {
            return nil, errors.New(fmt.Sprintf("reshape: cannot reshape array of %d elements to %d elements", originalCount, requiredCount))
        }

        // Create reshaped array
        return reshapeArray(originalFlat, dims), nil
    }

    // zeros() - Create zero-filled arrays
    slhelp["zeros"] = LibHelp{in: "dims...", out: "array", action: "Create array filled with zeros. Dimensions can be provided as separate arguments or as a single array."}
    stdlib["zeros"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if len(args) == 0 {
            return nil, errors.New("zeros: at least one dimension required")
        }

        var dims []int
        if len(args) == 1 {
            // Single argument - could be array of dimensions or single dimension
            switch args[0].(type) {
            case []int:
                dims = args[0].([]int)
            case []any:
                dimSlice := args[0].([]any)
                dims = make([]int, len(dimSlice))
                for i, d := range dimSlice {
                    if dimInt, ok := d.(int); ok {
                        dims[i] = dimInt
                    } else {
                        return nil, errors.New("zeros: dimensions must be integers")
                    }
                }
            case int:
                dims = []int{args[0].(int)}
            default:
                return nil, errors.New("zeros: dimensions must be integers")
            }
        } else {
            // Multiple arguments - treat each as a dimension
            dims = make([]int, len(args))
            for i, arg := range args {
                if dim, ok := arg.(int); ok {
                    dims[i] = dim
                } else {
                    return nil, errors.New("zeros: dimensions must be integers")
                }
            }
        }

        // Validate dimensions
        for _, dim := range dims {
            if dim < 0 {
                return nil, errors.New("zeros: dimensions must be non-negative")
            }
        }

        return createFilledArray(dims, 0), nil
    }

    // ones() - Create one-filled arrays
    slhelp["ones"] = LibHelp{in: "dims...", out: "array", action: "Create array filled with ones. Dimensions can be provided as separate arguments or as a single array."}
    stdlib["ones"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if len(args) == 0 {
            return nil, errors.New("ones: at least one dimension required")
        }

        var dims []int
        if len(args) == 1 {
            // Single argument - could be array of dimensions or single dimension
            switch args[0].(type) {
            case []int:
                dims = args[0].([]int)
            case []any:
                dimSlice := args[0].([]any)
                dims = make([]int, len(dimSlice))
                for i, d := range dimSlice {
                    if dimInt, ok := d.(int); ok {
                        dims[i] = dimInt
                    } else {
                        return nil, errors.New("ones: dimensions must be integers")
                    }
                }
            case int:
                dims = []int{args[0].(int)}
            default:
                return nil, errors.New("ones: dimensions must be integers")
            }
        } else {
            // Multiple arguments - treat each as a dimension
            dims = make([]int, len(args))
            for i, arg := range args {
                if dim, ok := arg.(int); ok {
                    dims[i] = dim
                } else {
                    return nil, errors.New("ones: dimensions must be integers")
                }
            }
        }

        // Validate dimensions
        for _, dim := range dims {
            if dim < 0 {
                return nil, errors.New("ones: dimensions must be non-negative")
            }
        }

        return createFilledArray(dims, 1), nil
    }

    // flatten() - Convert N-D to 1-D
    slhelp["flatten"] = LibHelp{in: "array", out: "array", action: "Flatten multi-dimensional array to 1-D array."}
    stdlib["flatten"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("flatten", args, 1, "1", "any"); !ok {
            return nil, err
        }

        array := args[0]
        if !isSlice(array) {
            return nil, errors.New("flatten: argument must be an array")
        }

        return flattenSlice(array), nil
    }

    // mean() - Statistical mean (alias to avg)
    slhelp["mean"] = LibHelp{in: "array", out: "number", action: "Calculate the arithmetic mean of values in an array. Supports multi-dimensional arrays."}
    stdlib["mean"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("mean", args, 1, "1", "any"); !ok {
            return nil, err
        }

        // Use existing avg function
        return avg_multi(args[0]), nil
    }

    // median() - Statistical median
    slhelp["median"] = LibHelp{in: "array", out: "number", action: "Calculate the median value in an array. Supports multi-dimensional arrays."}
    stdlib["median"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("median", args, 1, "1", "any"); !ok {
            return nil, err
        }

        array := args[0]
        if !isSlice(array) {
            return nil, errors.New("median: argument must be an array")
        }

        // Flatten array and convert to float64 slice for sorting
        flat := flattenSlice(array)
        if len(flat) == 0 {
            return 0.0, nil
        }

        // Convert to float64 for sorting
        values := make([]float64, len(flat))
        for i, val := range flat {
            f, hasError := GetAsFloat(val)
            if hasError {
                return math.NaN(), nil
            }
            values[i] = f
        }

        // Sort values
        sort.Float64s(values)

        // Calculate median
        n := len(values)
        if n%2 == 1 {
            // Odd number of values - return middle value
            return values[n/2], nil
        } else {
            // Even number of values - return average of two middle values
            return (values[n/2-1] + values[n/2]) / 2.0, nil
        }
    }

    // std() - Standard deviation
    slhelp["std"] = LibHelp{in: "array", out: "number", action: "Calculate the standard deviation of values in an array. Supports multi-dimensional arrays."}
    stdlib["std"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("std", args, 1, "1", "any"); !ok {
            return nil, err
        }

        array := args[0]
        if !isSlice(array) {
            return nil, errors.New("std: argument must be an array")
        }

        // Calculate variance first
        variance, err := calculateVariance(array)
        if err != nil {
            return nil, err
        }

        return math.Sqrt(variance), nil
    }

    // variance() - Variance
    slhelp["variance"] = LibHelp{in: "array", out: "number", action: "Calculate the variance of values in an array. Supports multi-dimensional arrays."}
    stdlib["variance"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("variance", args, 1, "1", "any"); !ok {
            return nil, err
        }

        array := args[0]
        if !isSlice(array) {
            return nil, errors.New("variance: argument must be an array")
        }

        return calculateVariance(array)
    }

    // identity() - Create identity matrix
    slhelp["identity"] = LibHelp{in: "size", out: "array", action: "Create identity matrix of specified size."}
    stdlib["identity"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("identity", args, 1, "1", "int"); !ok {
            return nil, err
        }

        size := args[0].(int)
        if size < 0 {
            return nil, errors.New("identity: size must be non-negative")
        }

        result := make([][]any, size)
        for i := 0; i < size; i++ {
            row := make([]any, size)
            for j := 0; j < size; j++ {
                if i == j {
                    row[j] = 1
                } else {
                    row[j] = 0
                }
            }
            result[i] = row
        }
        return result, nil
    }

    // trace() - Sum of diagonal elements
    slhelp["trace"] = LibHelp{in: "matrix", out: "number", action: "Calculate trace (sum of diagonal elements) of a square matrix."}
    stdlib["trace"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("trace", args, 1, "1", "any"); !ok {
            return nil, err
        }

        matrix := args[0]

        // Unwrap interface and convert to [][]any using same pattern as matmul
        var rows [][]any
        if slice, ok := matrix.([]interface{}); ok {
            // Convert []interface{} to [][]interface{}
            rows = make([][]any, len(slice))
            for i, val := range slice {
                if subSlice, ok := val.([]interface{}); ok {
                    rows[i] = make([]any, len(subSlice))
                    for j, subVal := range subSlice {
                        rows[i][j] = subVal
                    }
                } else {
                    return nil, errors.New("trace: matrix must be 2D")
                }
            }
        } else if slice, ok := matrix.([]any); ok {
            // Handle []any case
            rows = make([][]any, len(slice))
            for i, val := range slice {
                if subSlice, ok := val.([]any); ok {
                    rows[i] = subSlice
                } else {
                    return nil, errors.New("trace: matrix must be 2D")
                }
            }
        } else {
            return nil, errors.New("trace: matrix must be 2D")
        }

        if len(rows) == 0 {
            return 0.0, nil
        }

        // Check if square
        for _, row := range rows {
            if len(row) != len(rows) {
                return nil, errors.New("trace: matrix must be square")
            }
        }

        // Calculate trace
        sum := 0.0
        for i := 0; i < len(rows); i++ {
            val, hasError := GetAsFloat(rows[i][i])
            if hasError {
                return math.NaN(), errors.New("trace: non-numeric value in matrix")
            }
            sum += val
        }

        return sum, nil
    }

    // argmax() - Index of maximum value
    slhelp["argmax"] = LibHelp{in: "array", out: "number", action: "Find index of maximum value in array. Supports multi-dimensional arrays via flattening."}
    stdlib["argmax"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("argmax", args, 1, "1", "any"); !ok {
            return nil, err
        }

        array := args[0]
        if !isSlice(array) {
            return nil, errors.New("argmax: argument must be an array")
        }

        flat := flattenSlice(array)
        if len(flat) == 0 {
            return -1, nil
        }

        maxIndex := 0
        maxVal, _ := GetAsFloat(flat[0])

        for i := 1; i < len(flat); i++ {
            val, hasError := GetAsFloat(flat[i])
            if hasError {
                continue // Skip non-numeric values
            }
            if val > maxVal {
                maxVal = val
                maxIndex = i
            }
        }

        return maxIndex, nil
    }

    // argmin() - Index of minimum value
    slhelp["argmin"] = LibHelp{in: "array", out: "number", action: "Find index of minimum value in array. Supports multi-dimensional arrays via flattening."}
    stdlib["argmin"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("argmin", args, 1, "1", "any"); !ok {
            return nil, err
        }

        array := args[0]
        if !isSlice(array) {
            return nil, errors.New("argmin: argument must be an array")
        }

        flat := flattenSlice(array)
        if len(flat) == 0 {
            return -1, nil
        }

        minIndex := 0
        minVal, _ := GetAsFloat(flat[0])

        for i := 1; i < len(flat); i++ {
            val, hasError := GetAsFloat(flat[i])
            if hasError {
                continue // Skip non-numeric values
            }
            if val < minVal {
                minVal = val
                minIndex = i
            }
        }

        return minIndex, nil
    }

    // find() - Find indices of non-zero or matching elements
    slhelp["find"] = LibHelp{in: "array,value", out: "array", action: "Find indices of non-zero elements or elements matching specified value."}
    stdlib["find"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if len(args) < 1 || len(args) > 2 {
            return nil, errors.New("find: requires 1 or 2 arguments")
        }

        array := args[0]
        if !isSlice(array) {
            return nil, errors.New("find: first argument must be an array")
        }

        flat := flattenSlice(array)
        var indices []any

        if len(args) == 1 {
            // Find non-zero elements
            for i, val := range flat {
                if num, hasError := GetAsFloat(val); !hasError && num != 0 {
                    indices = append(indices, i)
                }
            }
        } else {
            // Find elements matching specific value
            target := args[1]
            for i, val := range flat {
                if val == target {
                    indices = append(indices, i)
                }
            }
        }

        return indices, nil
    }

    // where() - Conditional selection
    slhelp["where"] = LibHelp{in: "condition,x,y", out: "array", action: "Return elements from x where condition is true, from y where false."}
    stdlib["where"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("where", args, 3, "3", "any", "any", "any"); !ok {
            return nil, err
        }

        condition := args[0]
        x := args[1]
        y := args[2]

        if !isSlice(condition) {
            return nil, errors.New("where: condition must be an array")
        }

        condFlat := flattenSlice(condition)
        var xFlat, yFlat []any

        if isSlice(x) {
            xFlat = flattenSlice(x)
        } else {
            xFlat = []any{x}
        }

        if isSlice(y) {
            yFlat = flattenSlice(y)
        } else {
            yFlat = []any{y}
        }

        result := make([]any, len(condFlat))
        for i, cond := range condFlat {
            condVal, hasError := GetAsFloat(cond)
            if !hasError && condVal != 0 {
                // True - take from x
                if i < len(xFlat) {
                    result[i] = xFlat[i]
                } else {
                    result[i] = xFlat[len(xFlat)-1]
                }
            } else {
                // False - take from y
                if i < len(yFlat) {
                    result[i] = yFlat[i]
                } else {
                    result[i] = yFlat[len(yFlat)-1]
                }
            }
        }

        return result, nil
    }

    // stack() - Stack arrays along new axis
    slhelp["stack"] = LibHelp{in: "arrays...", out: "array", action: "Stack arrays along a new axis to create a higher-dimensional array."}
    stdlib["stack"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if len(args) == 0 {
            return nil, errors.New("stack: at least one array required")
        }

        // Validate all arguments are arrays
        for i, arg := range args {
            if !isSlice(arg) {
                return nil, fmt.Errorf("stack: argument %d must be an array", i+1)
            }
        }

        // Flatten all arrays
        flattened := make([][]any, len(args))
        for i, arg := range args {
            flattened[i] = flattenSlice(arg)
        }

        // Check if all arrays have same length
        if len(flattened) > 1 {
            firstLen := len(flattened[0])
            for i := 1; i < len(flattened); i++ {
                if len(flattened[i]) != firstLen {
                    return nil, errors.New("stack: all arrays must have same length")
                }
            }
        }

        return flattened, nil
    }

    // concatenate() - Join arrays along existing axis
    slhelp["concatenate"] = LibHelp{in: "arrays...", out: "array", action: "Join arrays along an existing axis."}
    stdlib["concatenate"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if len(args) == 0 {
            return nil, errors.New("concatenate: at least one array required")
        }

        // Validate all arguments are arrays
        for i, arg := range args {
            if !isSlice(arg) {
                return nil, fmt.Errorf("concatenate: argument %d must be an array", i+1)
            }
        }

        // Flatten all arrays and concatenate
        var result []any
        for _, arg := range args {
            flat := flattenSlice(arg)
            result = append(result, flat...)
        }

        return result, nil
    }

    // squeeze() - Remove singleton dimensions
    slhelp["squeeze"] = LibHelp{in: "array", out: "array", action: "Remove singleton dimensions from array."}
    stdlib["squeeze"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("squeeze", args, 1, "1", "any"); !ok {
            return nil, err
        }

        array := args[0]
        if !isSlice(array) {
            return nil, errors.New("squeeze: argument must be an array")
        }

        return squeezeArray(array), nil
    }

    // det() - Matrix determinant
    slhelp["det"] = LibHelp{in: "matrix", out: "number", action: "Calculate determinant of a square matrix."}
    stdlib["det"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("det", args, 1, "1", "any"); !ok {
            return nil, err
        }

        matrix := args[0]
        if !isSlice(matrix) {
            return nil, errors.New("det: argument must be a matrix")
        }

        // Convert to [][]any using same pattern as matmul
        var rows [][]any
        if slice, ok := matrix.([]interface{}); ok {
            // Convert []interface{} to [][]interface{}
            rows = make([][]any, len(slice))
            for i, val := range slice {
                if subSlice, ok := val.([]interface{}); ok {
                    rows[i] = make([]any, len(subSlice))
                    for j, subVal := range subSlice {
                        rows[i][j] = subVal
                    }
                } else {
                    return nil, errors.New("det: matrix must be 2D")
                }
            }
        } else if slice, ok := matrix.([]any); ok {
            // Handle []any case
            rows = make([][]any, len(slice))
            for i, val := range slice {
                if subSlice, ok := val.([]any); ok {
                    rows[i] = subSlice
                } else {
                    return nil, errors.New("det: matrix must be 2D")
                }
            }
        } else {
            return nil, errors.New("det: matrix must be 2D")
        }

        if len(rows) == 0 {
            return 1.0, nil
        }

        // Check if square
        for _, row := range rows {
            if len(row) != len(rows) {
                return nil, errors.New("det: matrix must be square")
            }
        }

        // Convert to float64 matrix
        n := len(rows)
        mat := make([][]float64, n)
        for i := 0; i < n; i++ {
            mat[i] = make([]float64, n)
            for j := 0; j < n; j++ {
                val, hasError := GetAsFloat(rows[i][j])
                if hasError {
                    return math.NaN(), errors.New("det: non-numeric value in matrix")
                }
                mat[i][j] = val
            }
        }

        return calculateDeterminant(mat), nil
    }

    // det_big() - Matrix determinant with arbitrary precision
    slhelp["det_big"] = LibHelp{in: "matrix", out: "big.Float", action: "Calculate determinant of a square matrix with arbitrary precision using big.Float."}
    stdlib["det_big"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("det_big", args, 1, "1", "any"); !ok {
            return nil, err
        }

        matrix := args[0]
        if !isSlice(matrix) {
            return nil, errors.New("det_big: argument must be a matrix")
        }

        // Convert to [][]any using same pattern as matmul
        var rows [][]any
        if slice, ok := matrix.([][]interface{}); ok {
            // Direct case: [][]interface{}
            rows = make([][]any, len(slice))
            for i, val := range slice {
                rows[i] = make([]any, len(val))
                for j, subVal := range val {
                    rows[i][j] = subVal
                }
            }
        } else if slice, ok := matrix.([][]any); ok {
            // Direct case: [][]any
            rows = slice
        } else if slice, ok := matrix.([]interface{}); ok {
            // Convert []interface{} to [][]interface{}
            rows = make([][]any, len(slice))
            for i, val := range slice {
                if subSlice, ok := val.([]interface{}); ok {
                    rows[i] = make([]any, len(subSlice))
                    for j, subVal := range subSlice {
                        rows[i][j] = subVal
                    }
                } else if subSlice, ok := val.([]any); ok {
                    // Handle case where inner rows are []any
                    rows[i] = subSlice
                } else {
                    return nil, errors.New("det_big: matrix must be 2D")
                }
            }
        } else if slice, ok := matrix.([]any); ok {
            // Handle []any case
            rows = make([][]any, len(slice))
            for i, val := range slice {
                if subSlice, ok := val.([]any); ok {
                    rows[i] = subSlice
                } else if subSlice, ok := val.([]interface{}); ok {
                    // Handle case where inner rows are []interface{}
                    rows[i] = make([]any, len(subSlice))
                    for j, subVal := range subSlice {
                        rows[i][j] = subVal
                    }
                } else {
                    return nil, errors.New("det_big: matrix must be 2D")
                }
            }
        } else {
            return nil, errors.New("det_big: matrix must be 2D")
        }

        if len(rows) == 0 {
            return big.NewFloat(1).SetPrec(128), nil
        }

        // Check if square
        for _, row := range rows {
            if len(row) != len(rows) {
                return nil, errors.New("det_big: matrix must be square")
            }
        }

        // Convert to big.Float matrix with higher precision
        n := len(rows)
        precision := uint(128) // Higher precision than float64
        mat := make([][]big.Float, n)
        for i := 0; i < n; i++ {
            mat[i] = make([]big.Float, n)
            for j := 0; j < n; j++ {
                val, hasError := GetAsFloat(rows[i][j])
                if hasError {
                    return nil, errors.New("det_big: non-numeric value in matrix")
                }
                mat[i][j] = *big.NewFloat(val).SetPrec(precision)
            }
        }

        result := calculateDeterminantBigFloat(mat)
        // Set reasonable precision to eliminate floating-point artifacts
        result.SetPrec(64) // Reduce to standard double precision
        return result, nil
    }

    // inverse() - Matrix inverse
    slhelp["inverse"] = LibHelp{in: "matrix", out: "array", action: "Calculate inverse of a square matrix."}
    stdlib["inverse"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("inverse", args, 1, "1", "any"); !ok {
            return nil, err
        }

        matrix := args[0]
        if !isSlice(matrix) {
            return nil, errors.New("inverse: argument must be a matrix")
        }

        // Convert to [][]any using same pattern as matmul
        var rows [][]any
        if slice, ok := matrix.([]interface{}); ok {
            // Convert []interface{} to [][]interface{}
            rows = make([][]any, len(slice))
            for i, val := range slice {
                if subSlice, ok := val.([]interface{}); ok {
                    rows[i] = make([]any, len(subSlice))
                    for j, subVal := range subSlice {
                        rows[i][j] = subVal
                    }
                } else {
                    return nil, errors.New("inverse: matrix must be 2D")
                }
            }
        } else if slice, ok := matrix.([]any); ok {
            // Handle []any case
            rows = make([][]any, len(slice))
            for i, val := range slice {
                if subSlice, ok := val.([]any); ok {
                    rows[i] = subSlice
                } else {
                    return nil, errors.New("inverse: matrix must be 2D")
                }
            }
        } else {
            return nil, errors.New("inverse: matrix must be 2D")
        }

        if len(rows) == 0 {
            return nil, errors.New("inverse: cannot invert empty matrix")
        }

        // Check if square
        for _, row := range rows {
            if len(row) != len(rows) {
                return nil, errors.New("inverse: matrix must be square")
            }
        }

        // Convert to float64 matrix
        n := len(rows)
        mat := make([][]float64, n)
        for i := 0; i < n; i++ {
            mat[i] = make([]float64, n)
            for j := 0; j < n; j++ {
                val, hasError := GetAsFloat(rows[i][j])
                if hasError {
                    return nil, errors.New("inverse: non-numeric value in matrix")
                }
                mat[i][j] = val
            }
        }

        // Check if matrix is singular
        det := calculateDeterminant(mat)
        if math.Abs(det) < 1e-10 {
            return nil, errors.New("inverse: matrix is singular (determinant is zero)")
        }

        inverse, err := calculateInverse(mat)
        if err != nil {
            return nil, err
        }

        // Convert back to [][]any
        result := make([][]any, n)
        for i := 0; i < n; i++ {
            result[i] = make([]any, n)
            for j := 0; j < n; j++ {
                result[i][j] = inverse[i][j]
            }
        }

        return result, nil
    }

    // rank() - Matrix rank
    slhelp["rank"] = LibHelp{in: "matrix", out: "number", action: "Calculate rank of a matrix."}
    stdlib["rank"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("rank", args, 1, "1", "any"); !ok {
            return nil, err
        }

        matrix := args[0]
        if !isSlice(matrix) {
            return nil, errors.New("rank: argument must be a matrix")
        }

        // Convert to [][]any using same pattern as matmul
        var rows [][]any
        if slice, ok := matrix.([]interface{}); ok {
            // Convert []interface{} to [][]interface{}
            rows = make([][]any, len(slice))
            for i, val := range slice {
                if subSlice, ok := val.([]interface{}); ok {
                    rows[i] = make([]any, len(subSlice))
                    for j, subVal := range subSlice {
                        rows[i][j] = subVal
                    }
                } else {
                    return nil, errors.New("rank: matrix must be 2D")
                }
            }
        } else if slice, ok := matrix.([]any); ok {
            // Handle []any case
            rows = make([][]any, len(slice))
            for i, val := range slice {
                if subSlice, ok := val.([]any); ok {
                    rows[i] = subSlice
                } else {
                    return nil, errors.New("rank: matrix must be 2D")
                }
            }
        } else {
            return nil, errors.New("rank: matrix must be 2D")
        }

        if len(rows) == 0 {
            return 0, nil
        }

        // Convert to float64 matrix
        m := len(rows)
        n := len(rows[0])
        mat := make([][]float64, m)
        for i := 0; i < m; i++ {
            mat[i] = make([]float64, n)
            for j := 0; j < n; j++ {
                if j < len(rows[i]) {
                    val, hasError := GetAsFloat(rows[i][j])
                    if hasError {
                        return nil, errors.New("rank: non-numeric value in matrix")
                    }
                    mat[i][j] = val
                } else {
                    mat[i][j] = 0.0
                }
            }
        }

        return calculateRank(mat), nil
    }

    // inverse_big() - Matrix inverse with arbitrary precision
    slhelp["inverse_big"] = LibHelp{in: "matrix", out: "array", action: "Calculate inverse of a square matrix with arbitrary precision using big.Float."}
    stdlib["inverse_big"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("inverse_big", args, 1, "1", "any"); !ok {
            return nil, err
        }

        matrix := args[0]
        if !isSlice(matrix) {
            return nil, errors.New("inverse_big: argument must be a matrix")
        }

        // Convert to [][]any using same pattern as matmul
        var rows [][]any
        if slice, ok := matrix.([][]interface{}); ok {
            // Direct case: [][]interface{}
            rows = make([][]any, len(slice))
            for i, val := range slice {
                rows[i] = make([]any, len(val))
                for j, subVal := range val {
                    rows[i][j] = subVal
                }
            }
        } else if slice, ok := matrix.([][]any); ok {
            // Direct case: [][]any
            rows = slice
        } else if slice, ok := matrix.([]interface{}); ok {
            // Convert []interface{} to [][]interface{}
            rows = make([][]any, len(slice))
            for i, val := range slice {
                if subSlice, ok := val.([]interface{}); ok {
                    rows[i] = make([]any, len(subSlice))
                    for j, subVal := range subSlice {
                        rows[i][j] = subVal
                    }
                } else if subSlice, ok := val.([]any); ok {
                    // Handle case where inner rows are []any
                    rows[i] = subSlice
                } else {
                    return nil, errors.New("inverse_big: matrix must be 2D")
                }
            }
        } else if slice, ok := matrix.([]any); ok {
            // Handle []any case
            rows = make([][]any, len(slice))
            for i, val := range slice {
                if subSlice, ok := val.([]any); ok {
                    rows[i] = subSlice
                } else if subSlice, ok := val.([]interface{}); ok {
                    // Handle case where inner rows are []interface{}
                    rows[i] = make([]any, len(subSlice))
                    for j, subVal := range subSlice {
                        rows[i][j] = subVal
                    }
                } else {
                    return nil, errors.New("inverse_big: matrix must be 2D")
                }
            }
        } else {
            return nil, errors.New("inverse_big: matrix must be 2D")
        }

        if len(rows) == 0 {
            return nil, errors.New("inverse_big: cannot invert empty matrix")
        }

        // Check if square
        for _, row := range rows {
            if len(row) != len(rows) {
                return nil, errors.New("inverse_big: matrix must be square")
            }
        }

        // Convert to big.Float matrix with higher precision
        n := len(rows)
        precision := uint(128) // Higher precision than float64
        mat := make([][]big.Float, n)
        for i := 0; i < n; i++ {
            mat[i] = make([]big.Float, n)
            for j := 0; j < n; j++ {
                val, hasError := GetAsFloat(rows[i][j])
                if hasError {
                    return nil, errors.New("inverse_big: non-numeric value in matrix")
                }
                mat[i][j] = *big.NewFloat(val).SetPrec(precision)
            }
        }

        // Check if matrix is singular using big.Float arithmetic
        det := calculateDeterminantBigFloat(mat)
        if det.Sign() == 0 {
            return nil, errors.New("inverse_big: matrix is singular (determinant is zero)")
        }

        inverse, err := calculateInverseBigFloat(mat)
        if err != nil {
            return nil, err
        }

        // Convert back to [][]any with reasonable precision
        result := make([][]any, n)
        for i := 0; i < n; i++ {
            result[i] = make([]any, n)
            for j := 0; j < n; j++ {
                // Set reasonable precision to eliminate floating-point artifacts
                inverse[i][j].SetPrec(64) // Reduce to standard double precision
                result[i][j] = inverse[i][j]
            }
        }

        return result, nil
    }
}

// Helper functions

// reshapeArray reshapes a flat array into the specified dimensions
func reshapeArray(flat []any, dims []int) any {
    if len(dims) == 0 {
        return []any{}
    }
    if len(dims) == 1 {
        return flat
    }

    // Create multi-dimensional array recursively
    return reshapeRecursive(flat, dims, 0)
}

// reshapeRecursive recursively builds the multi-dimensional array
func reshapeRecursive(flat []any, dims []int, dimIndex int) any {
    if dimIndex == len(dims)-1 {
        // Last dimension - return slice of values
        start := 0
        for i := 0; i < dimIndex; i++ {
            start += dims[i]
        }
        end := start + dims[dimIndex]
        return flat[start:end]
    }

    // Need to go deeper - create slice of recursive calls
    result := make([]any, dims[dimIndex])

    // Calculate the size of each sub-array at this level
    subArraySize := 1
    for i := dimIndex + 1; i < len(dims); i++ {
        subArraySize *= dims[i]
    }

    for i := 0; i < dims[dimIndex]; i++ {
        // Calculate the portion of flat array for this sub-array
        start := i * subArraySize
        end := start + subArraySize
        if end > len(flat) {
            end = len(flat)
        }
        subFlat := flat[start:end]
        // Pass remaining dimensions to recursive call
        remainingDims := make([]int, len(dims)-dimIndex-1)
        for j := dimIndex + 1; j < len(dims); j++ {
            remainingDims[j-dimIndex-1] = dims[j]
        }
        result[i] = reshapeRecursive(subFlat, remainingDims, 0)
    }

    return result
}

// createFilledArray creates an array filled with the specified value
func createFilledArray(dims []int, fillValue any) any {
    if len(dims) == 0 {
        return []any{}
    }
    if len(dims) == 1 {
        result := make([]any, dims[0])
        for i := range result {
            result[i] = fillValue
        }
        return result
    }

    // Create multi-dimensional array recursively
    return createFilledRecursive(dims, 0, fillValue)
}

// createFilledRecursive recursively builds the filled multi-dimensional array
func createFilledRecursive(dims []int, dimIndex int, fillValue any) any {
    if dimIndex == len(dims)-1 {
        // Last dimension - create filled slice
        result := make([]any, dims[dimIndex])
        for i := range result {
            result[i] = fillValue
        }
        return result
    }

    // Need to go deeper - create slice of recursive calls
    result := make([]any, dims[dimIndex])
    for i := range result {
        result[i] = createFilledRecursive(dims, dimIndex+1, fillValue)
    }

    return result
}

// calculateVariance calculates the variance of values in an array
func calculateVariance(array any) (float64, error) {
    // Flatten array and convert to float64
    flat := flattenSlice(array)
    if len(flat) == 0 {
        return 0.0, nil
    }

    // Convert to float64 and calculate mean
    values := make([]float64, len(flat))
    sum := 0.0
    for i, val := range flat {
        f, hasError := GetAsFloat(val)
        if hasError {
            return math.NaN(), errors.New("variance: non-numeric value in array")
        }
        values[i] = f
        sum += f
    }

    mean := sum / float64(len(values))

    // Calculate variance
    varianceSum := 0.0
    for _, f := range values {
        diff := f - mean
        varianceSum += diff * diff
    }

    return varianceSum / float64(len(values)), nil
}

// squeezeArray removes singleton dimensions from an array
func squeezeArray(array any) any {
    // If it's not a slice, return as-is
    if !isSlice(array) {
        return array
    }

    // Handle []interface{} case
    if slice, ok := array.([]interface{}); ok {
        // Check if this is a 2D array with single row
        if len(slice) == 1 {
            if row, ok := slice[0].([]interface{}); ok {
                return row
            }
        }

        // Check if this is a 2D array with single column
        if len(slice) > 0 {
            if firstRow, ok := slice[0].([]interface{}); ok && len(firstRow) == 1 {
                result := make([]interface{}, len(slice))
                for i, val := range slice {
                    if row, ok := val.([]interface{}); ok && len(row) == 1 {
                        result[i] = row[0]
                    } else {
                        return array // Not a proper single column, return as-is
                    }
                }
                return result
            }
        }

        return array
    }

    // Handle []any case
    if slice, ok := array.([]any); ok {
        // Check if this is a 2D array with single row
        if len(slice) == 1 {
            if row, ok := slice[0].([]any); ok {
                return row
            }
        }

        // Check if this is a 2D array with single column
        if len(slice) > 0 {
            if firstRow, ok := slice[0].([]any); ok && len(firstRow) == 1 {
                result := make([]any, len(slice))
                for i, val := range slice {
                    if row, ok := val.([]any); ok && len(row) == 1 {
                        result[i] = row[0]
                    } else {
                        return array // Not a proper single column, return as-is
                    }
                }
                return result
            }
        }

        return array
    }

    // For 1D arrays, return as-is (no singleton dimensions to remove)
    return array
}

// calculateDeterminant calculates determinant using Gaussian elimination
func calculateDeterminant(mat [][]float64) float64 {
    n := len(mat)
    if n == 0 {
        return 1.0
    }

    // Create a copy to avoid modifying original
    temp := make([][]float64, n)
    for i := 0; i < n; i++ {
        temp[i] = make([]float64, n)
        for j := 0; j < n; j++ {
            temp[i][j] = mat[i][j]
        }
    }

    det := 1.0

    for i := 0; i < n; i++ {
        // Find pivot
        pivot := i
        for j := i + 1; j < n; j++ {
            if math.Abs(temp[j][i]) > math.Abs(temp[pivot][i]) {
                pivot = j
            }
        }

        // Swap rows if needed
        if pivot != i {
            temp[i], temp[pivot] = temp[pivot], temp[i]
            det *= -1
        }

        // If pivot is zero, determinant is zero
        if math.Abs(temp[i][i]) < 1e-10 {
            return 0.0
        }

        // Multiply diagonal elements
        det *= temp[i][i]

        // Eliminate column
        for j := i + 1; j < n; j++ {
            factor := temp[j][i] / temp[i][i]
            for k := i; k < n; k++ {
                temp[j][k] -= factor * temp[i][k]
            }
        }
    }

    return det
}

// calculateInverse calculates matrix inverse using Gaussian elimination
func calculateInverse(mat [][]float64) ([][]float64, error) {
    n := len(mat)

    // Create augmented matrix [A|I]
    aug := make([][]float64, n)
    for i := 0; i < n; i++ {
        aug[i] = make([]float64, 2*n)
        for j := 0; j < n; j++ {
            aug[i][j] = mat[i][j]
        }
        aug[i][i+n] = 1.0 // Identity matrix
    }

    // Gaussian elimination
    for i := 0; i < n; i++ {
        // Find pivot
        pivot := i
        for j := i + 1; j < n; j++ {
            if math.Abs(aug[j][i]) > math.Abs(aug[pivot][i]) {
                pivot = j
            }
        }

        // Swap rows if needed
        if pivot != i {
            aug[i], aug[pivot] = aug[pivot], aug[i]
        }

        // Check for singular matrix
        if math.Abs(aug[i][i]) < 1e-10 {
            return nil, errors.New("matrix is singular")
        }

        // Normalize pivot row
        pivotVal := aug[i][i]
        for j := 0; j < 2*n; j++ {
            aug[i][j] /= pivotVal
        }

        // Eliminate other rows
        for j := 0; j < n; j++ {
            if i != j {
                factor := aug[j][i]
                for k := 0; k < 2*n; k++ {
                    aug[j][k] -= factor * aug[i][k]
                }
            }
        }
    }

    // Extract inverse matrix
    inverse := make([][]float64, n)
    for i := 0; i < n; i++ {
        inverse[i] = make([]float64, n)
        for j := 0; j < n; j++ {
            inverse[i][j] = aug[i][j+n]
        }
    }

    return inverse, nil
}

// calculateRank calculates matrix rank using Gaussian elimination
func calculateRank(mat [][]float64) int {
    m := len(mat)
    if m == 0 {
        return 0
    }
    n := len(mat[0])

    // Create a copy to avoid modifying original
    temp := make([][]float64, m)
    for i := 0; i < m; i++ {
        temp[i] = make([]float64, n)
        for j := 0; j < n; j++ {
            temp[i][j] = mat[i][j]
        }
    }

    rank := 0
    row := 0
    col := 0

    for row < m && col < n {
        // Find pivot
        pivot := row
        for i := row + 1; i < m; i++ {
            if math.Abs(temp[i][col]) > math.Abs(temp[pivot][col]) {
                pivot = i
            }
        }

        // If pivot is zero, move to next column
        if math.Abs(temp[pivot][col]) < 1e-10 {
            col++
            continue
        }

        // Swap rows if needed
        if pivot != row {
            temp[row], temp[pivot] = temp[pivot], temp[row]
        }

        // Normalize pivot row
        pivotVal := temp[row][col]
        for j := col; j < n; j++ {
            temp[row][j] /= pivotVal
        }

        // Eliminate other rows
        for i := 0; i < m; i++ {
            if i != row && math.Abs(temp[i][col]) > 1e-10 {
                factor := temp[i][col]
                for j := col; j < n; j++ {
                    temp[i][j] -= factor * temp[row][j]
                }
            }
        }

        rank++
        row++
        col++
    }

    return rank
}

// luDecompositionBigFloat performs LU decomposition with partial pivoting
// Returns P (permutation vector), L (lower triangular), U (upper triangular)
func luDecompositionBigFloat(mat [][]big.Float) ([]int, [][]big.Float, [][]big.Float, error) {
    n := len(mat)
    precision := uint(128)

    // Initialize matrices
    P := make([]int, n)
    L := make([][]big.Float, n)
    U := make([][]big.Float, n)

    // Copy matrix to U and initialize L as identity
    for i := 0; i < n; i++ {
        P[i] = i
        L[i] = make([]big.Float, n)
        U[i] = make([]big.Float, n)
        for j := 0; j < n; j++ {
            U[i][j] = mat[i][j]
            if i == j {
                L[i][j] = *big.NewFloat(1).SetPrec(precision)
            } else {
                L[i][j] = *big.NewFloat(0).SetPrec(precision)
            }
        }
    }

    // Perform LU decomposition with partial pivoting
    for i := 0; i < n; i++ {
        // Find pivot row
        pivotRow := i
        maxAbs := new(big.Float).Abs(&U[i][i])
        for k := i + 1; k < n; k++ {
            if new(big.Float).Abs(&U[k][i]).Cmp(maxAbs) > 0 {
                maxAbs = new(big.Float).Abs(&U[k][i])
                pivotRow = k
            }
        }

        // Swap rows in U and update permutation vector
        if pivotRow != i {
            P[i], P[pivotRow] = P[pivotRow], P[i]
            for j := 0; j < n; j++ {
                U[i][j], U[pivotRow][j] = U[pivotRow][j], U[i][j]
            }
        }

        // Eliminate column i
        for j := i + 1; j < n; j++ {
            factor := new(big.Float).Quo(&U[j][i], &U[i][i])
            L[j][i] = *factor // Store the multiplier in L matrix
            for k := i; k < n; k++ {
                product := new(big.Float).Mul(factor, &U[i][k])
                U[j][k].Sub(&U[j][k], product)
            }
        }
    }

    return P, L, U, nil
}

// calculateDeterminantBigFloat calculates determinant using LU decomposition
func calculateDeterminantBigFloat(mat [][]big.Float) *big.Float {
    n := len(mat)
    if n == 0 {
        return big.NewFloat(1)
    }
    if n == 1 {
        return &mat[0][0]
    }
    if n == 2 {
        // Use adjugate method for 2x2 matrices (exact)
        det := new(big.Float).SetPrec(128)
        det.Mul(&mat[0][0], &mat[1][1])
        temp := new(big.Float).SetPrec(128)
        temp.Mul(&mat[0][1], &mat[1][0])
        det.Sub(det, temp)
        return det
    }

    // Use LU decomposition for larger matrices
    P, L, U, err := luDecompositionBigFloat(mat)
    if err != nil {
        return big.NewFloat(0) // Return 0 on error
    }

    // Use L and U to avoid compiler warning
    _ = L
    _ = U

    // Calculate determinant from LU: det(A) = det(P) * det(U)
    // det(U) is product of diagonal elements
    detU := big.NewFloat(1).SetPrec(128)
    for i := 0; i < n; i++ {
        detU.Mul(detU, &U[i][i])
    }

    // Calculate det(P) from permutation parity
    detP := big.NewFloat(1).SetPrec(128)
    visited := make([]bool, n)
    for i := 0; i < n; i++ {
        if !visited[i] {
            // Count cycle length
            cycleLen := 0
            j := i
            for !visited[j] {
                visited[j] = true
                j = P[j]
                cycleLen++
            }
            // Even cycle length contributes -1 to determinant (swap count = cycleLen - 1)
            if (cycleLen-1)%2 == 1 {
                detP.Neg(detP)
            }
        }
    }

    return new(big.Float).Mul(detP, detU)
}

// calculateInverseBigFloat calculates inverse using big.Float
func calculateInverseBigFloat(mat [][]big.Float) ([][]*big.Float, error) {
    n := len(mat)
    precision := uint(128)

    // For 2x2 matrices, use adjugate method which is more reliable
    if n == 2 {
        // Calculate determinant
        det := new(big.Float).SetPrec(precision)
        det.Mul(&mat[0][0], &mat[1][1])
        temp := new(big.Float).SetPrec(precision)
        temp.Mul(&mat[0][1], &mat[1][0])
        det.Sub(det, temp)

        if det.Sign() == 0 {
            return nil, errors.New("matrix is singular")
        }

        // Calculate inverse using formula: 1/det * [[d, -b], [-c, a]]
        invDet := new(big.Float).SetPrec(precision)
        invDet.Quo(big.NewFloat(1).SetPrec(precision), det)

        inverse := make([][]*big.Float, 2)
        inverse[0] = make([]*big.Float, 2)
        inverse[1] = make([]*big.Float, 2)

        // [0][0] = d/det
        inverse[0][0] = new(big.Float).Mul(&mat[1][1], invDet).SetPrec(precision)
        // [0][1] = -b/det
        inverse[0][1] = new(big.Float).Mul(&mat[0][1], invDet).SetPrec(precision)
        inverse[0][1].Neg(inverse[0][1])
        // [1][0] = -c/det
        inverse[1][0] = new(big.Float).Mul(&mat[1][0], invDet).SetPrec(precision)
        inverse[1][0].Neg(inverse[1][0])
        // [1][1] = a/det
        inverse[1][1] = new(big.Float).Mul(&mat[0][0], invDet).SetPrec(precision)

        return inverse, nil
    }

    // For larger matrices, use LU decomposition
    // Get LU decomposition
    P, L, U, err := luDecompositionBigFloat(mat)
    if err != nil {
        return nil, err
    }

    // Create identity matrix for solving
    identity := make([][]big.Float, n)
    for i := 0; i < n; i++ {
        identity[i] = make([]big.Float, n)
        for j := 0; j < n; j++ {
            if i == j {
                identity[i][j] = *big.NewFloat(1).SetPrec(precision)
            } else {
                identity[i][j] = *big.NewFloat(0).SetPrec(precision)
            }
        }
    }

    // Solve for each column of the inverse
    inverse := make([][]*big.Float, n)
    for i := 0; i < n; i++ {
        inverse[i] = make([]*big.Float, n)
        for j := 0; j < n; j++ {
            inverse[i][j] = new(big.Float).SetPrec(precision)
        }
    }

    // For each column of the identity matrix, solve Ax = b
    for col := 0; col < n; col++ {
        // Extract the column vector
        b := make([]big.Float, n)
        for i := 0; i < n; i++ {
            b[i] = identity[P[i]][col] // Apply permutation to RHS
        }

        // Forward substitution: solve Ly = b
        y := make([]big.Float, n)
        for i := 0; i < n; i++ {
            sum := big.NewFloat(0).SetPrec(precision)
            for j := 0; j < i; j++ {
                product := new(big.Float).Mul(&L[i][j], &y[j])
                sum.Add(sum, product)
            }
            y[i].Sub(&b[i], sum)
        }

        // Back substitution: solve Ux = y
        x := make([]big.Float, n)
        for i := n - 1; i >= 0; i-- {
            sum := big.NewFloat(0).SetPrec(precision)
            for j := i + 1; j < n; j++ {
                product := new(big.Float).Mul(&U[i][j], &x[j])
                sum.Add(sum, product)
            }
            temp := new(big.Float).Sub(&y[i], sum)
            x[i].Quo(temp, &U[i][i])
        }

        // Store the solution column in the inverse matrix
        for i := 0; i < n; i++ {
            inverse[i][col].Set(&x[i])
        }
    }

    return inverse, nil

}
