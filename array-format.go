package main

import (
    "fmt"
    "math/big"
    "reflect"
    "strconv"
    str "strings"
)

// goLiteralToZaLiteral converts Go literal representations to Za literal format
func goLiteralToZaLiteral(v any) string {
    switch val := v.(type) {
    case string:
        return `"` + val + `"`
    case int:
        return strconv.FormatInt(int64(val), 10)
    case int8:
        return strconv.FormatInt(int64(val), 10)
    case int16:
        return strconv.FormatInt(int64(val), 10)
    case int32:
        return strconv.FormatInt(int64(val), 10)
    case int64:
        return strconv.FormatInt(val, 10)
    case uint:
        return strconv.FormatUint(uint64(val), 10)
    case uint8:
        return strconv.FormatUint(uint64(val), 10)
    case uint16:
        return strconv.FormatUint(uint64(val), 10)
    case uint32:
        return strconv.FormatUint(uint64(val), 10)
    case uint64:
        return strconv.FormatUint(val, 10)
    case float64:
        return strconv.FormatFloat(val, 'g', -1, 64)
    case bool:
        return strconv.FormatBool(val)
    case nil:
        return "nil"
    case *big.Int:
        return val.String()
    case *big.Float:
        return val.String()
    case map[string]any:
        return mapToZaLiteral(val)
    case []any:
        return arrayToZaLiteral(val)
    case []int:
        return intArrayToZaLiteral(val)
    case []int8:
        return int8ArrayToZaLiteral(val)
    case []int16:
        return int16ArrayToZaLiteral(val)
    case []int32:
        return int32ArrayToZaLiteral(val)
    case []int64:
        return int64ArrayToZaLiteral(val)
    case []uint:
        return uintArrayToZaLiteral(val)
    case []uint8:
        return uint8ArrayToZaLiteral(val)
    case []uint16:
        return uint16ArrayToZaLiteral(val)
    case []uint32:
        return uint32ArrayToZaLiteral(val)
    case []uint64:
        return uint64ArrayToZaLiteral(val)
    case []float64:
        return float64ArrayToZaLiteral(val)
    case []string:
        return stringArrayToZaLiteral(val)
    case []bool:
        return boolArrayToZaLiteral(val)
    case []*big.Int:
        return bigIntArrayToZaLiteral(val)
    case []*big.Float:
        return bigFloatArrayToZaLiteral(val)
    case [][]int:
        return multiIntArrayToZaLiteral(val)
    case [][]float64:
        return multiFloat64ArrayToZaLiteral(val)
    case [][]string:
        return multiStringArrayToZaLiteral(val)
    case [][]bool:
        return multiBoolArrayToZaLiteral(val)
    case [][]any:
        return multiAnyArrayToZaLiteral(val)
    default:
        // Handle user-defined structs and other complex types with reflection
        vVal := reflect.ValueOf(v)
        switch vVal.Kind() {
        case reflect.Struct:
            return structToZaLiteral(v)
        case reflect.Map:
            return genericMapToZaLiteral(v)
        case reflect.Slice, reflect.Array:
            return genericArrayToZaLiteral(v)
        default:
            panic(fmt.Sprintf("goLiteralToZaLiteral: unsupported type %T", val))
        }
    }
}

func mapToZaLiteral(m map[string]any) string {
    if len(m) == 0 {
        return "map()"
    }

    var parts []string
    for key, value := range m {
        valueStr := goLiteralToZaLiteral(value)
        parts = append(parts, sf(".%s %s", key, valueStr))
    }
    return "map(" + str.Join(parts, ", ") + ")"
}

func arrayToZaLiteral(arr []any) string {
    if len(arr) == 0 {
        return "[]"
    }

    var parts []string
    for _, item := range arr {
        itemStr := goLiteralToZaLiteral(item)
        parts = append(parts, itemStr)
    }
    return "[" + str.Join(parts, ", ") + "]"
}

func bigIntArrayToZaLiteral(arr []*big.Int) string {
    if len(arr) == 0 {
        return "[]"
    }

    var parts []string
    for _, item := range arr {
        parts = append(parts, item.String())
    }
    return "[" + str.Join(parts, ", ") + "]"
}

func bigFloatArrayToZaLiteral(arr []*big.Float) string {
    if len(arr) == 0 {
        return "[]"
    }

    var parts []string
    for _, item := range arr {
        parts = append(parts, item.String())
    }
    return "[" + str.Join(parts, ", ") + "]"
}

func intArrayToZaLiteral(arr []int) string {
    if len(arr) == 0 {
        return "[]"
    }

    var parts []string
    for _, item := range arr {
        parts = append(parts, strconv.FormatInt(int64(item), 10))
    }
    return "[" + str.Join(parts, ", ") + "]"
}

func float64ArrayToZaLiteral(arr []float64) string {
    if len(arr) == 0 {
        return "[]"
    }

    var parts []string
    for _, item := range arr {
        parts = append(parts, strconv.FormatFloat(item, 'g', -1, 64))
    }
    return "[" + str.Join(parts, ", ") + "]"
}

func stringArrayToZaLiteral(arr []string) string {
    if len(arr) == 0 {
        return "[]"
    }

    var parts []string
    for _, item := range arr {
        parts = append(parts, `"`+item+`"`)
    }
    return "[" + str.Join(parts, ", ") + "]"
}

func boolArrayToZaLiteral(arr []bool) string {
    if len(arr) == 0 {
        return "[]"
    }

    var parts []string
    for _, item := range arr {
        parts = append(parts, strconv.FormatBool(item))
    }
    return "[" + str.Join(parts, ", ") + "]"
}

func multiIntArrayToZaLiteral(arr [][]int) string {
    if len(arr) == 0 {
        return "[]"
    }

    var parts []string
    for _, item := range arr {
        parts = append(parts, intArrayToZaLiteral(item))
    }
    return "[" + str.Join(parts, ", ") + "]"
}

func multiFloat64ArrayToZaLiteral(arr [][]float64) string {
    if len(arr) == 0 {
        return "[]"
    }

    var parts []string
    for _, item := range arr {
        parts = append(parts, float64ArrayToZaLiteral(item))
    }
    return "[" + str.Join(parts, ", ") + "]"
}

func multiStringArrayToZaLiteral(arr [][]string) string {
    if len(arr) == 0 {
        return "[]"
    }

    var parts []string
    for _, item := range arr {
        parts = append(parts, stringArrayToZaLiteral(item))
    }
    return "[" + str.Join(parts, ", ") + "]"
}

func multiBoolArrayToZaLiteral(arr [][]bool) string {
    if len(arr) == 0 {
        return "[]"
    }

    var parts []string
    for _, item := range arr {
        parts = append(parts, boolArrayToZaLiteral(item))
    }
    return "[" + str.Join(parts, ", ") + "]"
}

func multiAnyArrayToZaLiteral(arr [][]any) string {
    if len(arr) == 0 {
        return "[]"
    }

    var parts []string
    for _, item := range arr {
        parts = append(parts, arrayToZaLiteral(item))
    }
    return "[" + str.Join(parts, ", ") + "]"
}

// Array formatting functions for pretty printing
func isArrayType(v any) bool {
    rv := reflect.ValueOf(v)
    return rv.Kind() == reflect.Slice || rv.Kind() == reflect.Array
}

func getArrayDimensions(v any) []int {
    var dimensions []int
    current := v

    for {
        rv := reflect.ValueOf(current)
        if rv.Kind() != reflect.Slice && rv.Kind() != reflect.Array {
            break
        }

        dimensions = append(dimensions, rv.Len())

        if rv.Len() == 0 {
            break
        }

        elem := rv.Index(0)

        // Check if element is a slice (handle both direct slices and interface-wrapped slices)
        var elemValue reflect.Value
        if elem.Kind() == reflect.Interface {
            elemValue = reflect.ValueOf(elem.Interface())
        } else {
            elemValue = elem
        }

        if elemValue.Kind() == reflect.Slice || elemValue.Kind() == reflect.Array {
            current = elem.Interface()
        } else {
            break
        }
    }

    return dimensions
}

func getElementType(v any) string {
    rv := reflect.ValueOf(v)
    if rv.Kind() == reflect.Slice || rv.Kind() == reflect.Array {
        elemType := rv.Type().Elem()

        // Handle []any case specifically
        if elemType.Kind() == reflect.Interface {
            // Check if we have elements and determine their actual type
            if rv.Len() > 0 {
                firstElem := rv.Index(0)
                switch firstElem.Kind() {
                case reflect.Int, reflect.Int64:
                    return "Int64"
                case reflect.Uint, reflect.Uint32, reflect.Uint64:
                    return "Uint64"
                case reflect.Float32, reflect.Float64:
                    return "Float64"
                case reflect.String:
                    return "String"
                case reflect.Bool:
                    return "Bool"
                default:
                    // Handle big.Int and big.Float
                    if firstElem.Type().String() == "*big.Int" {
                        return "BigInt"
                    } else if firstElem.Type().String() == "*big.Float" {
                        return "BigFloat"
                    }
                    return "Any"
                }
            }
            return "Any"
        }

        // Handle basic types
        switch elemType.Kind() {
        case reflect.Int:
            return "Int64"
        case reflect.Int64:
            return "Int64"
        case reflect.Uint, reflect.Uint32, reflect.Uint64:
            return "Uint64"
        case reflect.Float32, reflect.Float64:
            return "Float64"
        case reflect.String:
            return "String"
        case reflect.Bool:
            return "Bool"
        default:
            // Handle big.Int and big.Float
            if elemType.String() == "*big.Int" {
                return "BigInt"
            } else if elemType.String() == "*big.Float" {
                return "BigFloat"
            }
            return elemType.Name()
        }
    }
    return "Unknown"
}

func getDepthColour(depth int) string {
    if len(depthColours) == 0 {
        return ""
    }
    return depthColours[depth%len(depthColours)]
}

func formatArrayPretty(v any) string {
    dimensions := getArrayDimensions(v)
    elemType := getElementType(v)

    if len(dimensions) == 0 {
        return fmt.Sprintf("%v", v) // Not an array
    }

    if len(dimensions) == 1 {
        return formatArray1D(v, elemType, dimensions[0])
    } else if len(dimensions) == 2 {
        return formatArray2D(v, elemType, dimensions)
    } else {
        return formatArrayND(v, elemType, dimensions)
    }
}

func formatArray1D(v any, elemType string, length int) string {
    rv := reflect.ValueOf(v)
    if length == 0 {
        return fmt.Sprintf("0-element Vector{%s}: []", elemType)
    }

    var elements []string
    for i := 0; i < length; i++ {
        elem := rv.Index(i).Interface()
        // Apply display precision reduction when array_format() is enabled
        if prettyArrays {
            if bf, ok := elem.(*big.Float); ok {
                // Create display copy with reduced precision
                displayCopy := new(big.Float).Copy(bf)
                displayCopy.SetPrec(64) // Reduce to display precision
                elem = displayCopy
            }
        }
        colour := getDepthColour(0)
        reset := ""
        if colour != "" {
            reset = "[#-]"
        }
        elements = append(elements, fmt.Sprintf("%s%v%s", colour, elem, reset))
    }

    return fmt.Sprintf("%d-element Vector{%s}: [%s]", length, elemType, str.Join(elements, ", "))
}

func formatArray2D(v any, elemType string, dimensions []int) string {
    rows := dimensions[0]
    cols := dimensions[1]
    rv := reflect.ValueOf(v)

    if rows == 0 || cols == 0 {
        return fmt.Sprintf("%dx%d Matrix{%s}: []", rows, cols, elemType)
    }

    var lines []string
    for i := 0; i < rows; i++ {
        var rowElements []string
        row := rv.Index(i)

        // Handle case where row is an interface containing a slice
        var rowSlice reflect.Value
        if row.Kind() == reflect.Interface {
            rowSlice = reflect.ValueOf(row.Interface())
        } else {
            rowSlice = row
        }

        for j := 0; j < cols; j++ {
            elem := rowSlice.Index(j).Interface()
            // Apply display precision reduction when array_format() is enabled
            if prettyArrays {
                if bf, ok := elem.(*big.Float); ok {
                    // Create display copy with reduced precision
                    displayCopy := new(big.Float).Copy(bf)
                    displayCopy.SetPrec(64) // Reduce to display precision
                    elem = displayCopy
                }
            }
            colour := getDepthColour(1) // Elements in 2D array are at depth 1
            reset := ""
            if colour != "" {
                reset = "[#-]"
            }
            rowElements = append(rowElements, fmt.Sprintf("%s%v%s", colour, elem, reset))
        }
        lines = append(lines, str.Join(rowElements, " "))
    }

    return fmt.Sprintf("%dx%d Matrix{%s}:\n%s", rows, cols, elemType, str.Join(lines, "\n"))
}

func formatArrayPage(v any, elemType string, dimensions []int, pageDim int, pageIndex int) string {
    // Extract the slice for this page by recursively indexing
    rv := reflect.ValueOf(v)
    currentValue := rv

    // Navigate to the page by indexing through dimensions before pageDim
    for d := 0; d < pageDim; d++ {
        if currentValue.Kind() == reflect.Interface {
            currentValue = reflect.ValueOf(currentValue.Interface())
        }
        currentValue = currentValue.Index(pageIndex)
    }

    // Now format the remaining dimensions (which should be pageDim+1 to len(dimensions)-1)
    remainingDims := dimensions[pageDim+1:]
    if len(remainingDims) == 0 {
        // This shouldn't happen as pageDim should be < len(dimensions)-1
        return ""
    }

    if len(remainingDims) == 1 {
        // 1D page - show as vector
        return formatArray1D(currentValue.Interface(), elemType, remainingDims[0])
    } else if len(remainingDims) == 2 {
        // 2D page - show as matrix
        return formatArray2DFromValue(currentValue, elemType, remainingDims)
    } else {
        // 3D+ page - recursively format
        return formatArrayNDRecursive(currentValue.Interface(), elemType, remainingDims, 0, []int{})
    }
}

func formatArray2DFromValue(rv reflect.Value, elemType string, dimensions []int) string {
    rows := dimensions[0]
    cols := dimensions[1]

    if rows == 0 || cols == 0 {
        return fmt.Sprintf("%dx%d Matrix{%s}: []", rows, cols, elemType)
    }

    var lines []string
    for i := 0; i < rows; i++ {
        var rowElements []string
        row := rv.Index(i)

        // Handle case where row is an interface containing a slice
        var rowSlice reflect.Value
        if row.Kind() == reflect.Interface {
            rowSlice = reflect.ValueOf(row.Interface())
        } else {
            rowSlice = row
        }

        // Validate that rowSlice is actually a slice/array before proceeding
        if rowSlice.Kind() != reflect.Slice && rowSlice.Kind() != reflect.Array {
            // Fallback to safe formatting for mixed-type arrays
            return fmt.Sprintf("Mixed-type array (cannot format uniformly): %v", rv.Interface())
        }

        for j := 0; j < cols; j++ {
            elem := rowSlice.Index(j).Interface()
            // Apply display precision reduction when array_format() is enabled
            if prettyArrays {
                if bf, ok := elem.(*big.Float); ok {
                    // Create display copy with reduced precision
                    displayCopy := new(big.Float).Copy(bf)
                    displayCopy.SetPrec(64) // Reduce to display precision
                    elem = displayCopy
                }
            }
            colour := getDepthColour(len(dimensions)) // Adjust depth based on context
            reset := ""
            if colour != "" {
                reset = "[#-]"
            }
            rowElements = append(rowElements, fmt.Sprintf("%s%v%s", colour, elem, reset))
        }
        lines = append(lines, str.Join(rowElements, " "))
    }

    return str.Join(lines, "\n")
}

func formatArrayNDRecursive(v any, elemType string, dimensions []int, depth int, indices []int) string {
    rv := reflect.ValueOf(v)

    // Handle interface unwrapping at current level
    var currentValue reflect.Value
    if rv.Kind() == reflect.Interface {
        currentValue = reflect.ValueOf(rv.Interface())
    } else {
        currentValue = rv
    }

    // Base case: we've reached the innermost dimension (should be elements)
    if depth == len(dimensions)-1 {
        var elements []string
        for i := 0; i < dimensions[depth]; i++ {
            elem := currentValue.Index(i).Interface()
            // Apply display precision reduction when array_format() is enabled
            if prettyArrays {
                if bf, ok := elem.(*big.Float); ok {
                    // Create display copy with reduced precision
                    displayCopy := new(big.Float).Copy(bf)
                    displayCopy.SetPrec(64) // Reduce to display precision
                    elem = displayCopy
                }
            }
            colour := getDepthColour(depth)
            reset := ""
            if colour != "" {
                reset = "[#-]"
            }
            elements = append(elements, fmt.Sprintf("%s%v%s", colour, elem, reset))
        }
        return str.Join(elements, " ")
    }

    // Recursive case: we need to go deeper
    var result []string
    for i := 0; i < dimensions[depth]; i++ {
        nextLevel := currentValue.Index(i)

        // Get the actual interface value for recursion
        var nextValue any
        if nextLevel.Kind() == reflect.Interface {
            nextValue = nextLevel.Interface()
        } else {
            nextValue = nextLevel.Interface()
        }

        // Recursively format the next level
        newIndices := append([]int{i}, indices...)
        subResult := formatArrayNDRecursive(nextValue, elemType, dimensions, depth+1, newIndices)

        if len(dimensions)-depth > 2 {
            // For more than 2 levels remaining, show with index
            indexParts := append([]string{}, fmt.Sprintf("%d", i+1))
            for _, idx := range indices {
                indexParts = append([]string{fmt.Sprintf("%d", idx+1)}, indexParts...)
            }
            indexStr := fmt.Sprintf("[%s]", str.Join(indexParts, ", "))
            result = append(result, fmt.Sprintf("%s = %s", indexStr, subResult))
        } else {
            // For the last 2 levels, just show the content
            result = append(result, subResult)
        }
    }

    return str.Join(result, "\n")
}

func formatArrayNDPages(v any, elemType string, dimensions []int) string {
    rv := reflect.ValueOf(v)

    // Validate that we have a slice/array at the top level
    if rv.Kind() != reflect.Slice && rv.Kind() != reflect.Array {
        return fmt.Sprintf("Not an array: %v", v)
    }

    // Build dimension string like "2x3x4x5"
    var dimStrs []string
    for _, dim := range dimensions {
        dimStrs = append(dimStrs, fmt.Sprintf("%d", dim))
    }
    dimStr := str.Join(dimStrs, "x")

    var result []string

    // For 4D arrays, show each 3D slice as a page
    if len(dimensions) == 4 {
        d1, d2, d3, d4 := dimensions[0], dimensions[1], dimensions[2], dimensions[3]

        for k := 0; k < d1; k++ {
            // Get the 3D slice for this page
            slice := rv.Index(k)

            // Handle case where slice is an interface containing a slice
            var sliceValue reflect.Value
            if slice.Kind() == reflect.Interface {
                sliceValue = reflect.ValueOf(slice.Interface())
            } else {
                sliceValue = slice
            }

            // Validate that sliceValue is actually a slice/array before proceeding
            if sliceValue.Kind() != reflect.Slice && sliceValue.Kind() != reflect.Array {
                // Fallback to safe formatting for mixed-type arrays
                return fmt.Sprintf("Mixed-type array (cannot format uniformly): %v", v)
            }

            // Format this 3D slice using the existing 3D formatting logic
            pageContent := format3DFromValue(sliceValue, elemType, []int{d2, d3, d4})

            // Add page header
            result = append(result, fmt.Sprintf("[:, :, :, %d] =", k+1))
            result = append(result, pageContent)

            if k < d1-1 {
                result = append(result, "") // Add blank line between pages
            }
        }

        return fmt.Sprintf("%dx%dx%dx%d Array{%s, 4}:\n%s", d1, d2, d3, d4, elemType, str.Join(result, "\n"))
    }

    // For 5D+ arrays, recursively show each (N-1)D slice as a page
    if len(dimensions) >= 5 {
        firstDim := dimensions[0]
        remainingDims := dimensions[1:]

        for k := 0; k < firstDim; k++ {
            // Get the (N-1)D slice for this page
            slice := rv.Index(k)

            // Handle case where slice is an interface containing a slice
            var sliceValue reflect.Value
            if slice.Kind() == reflect.Interface {
                sliceValue = reflect.ValueOf(slice.Interface())
            } else {
                sliceValue = slice
            }

            // Validate that sliceValue is actually a slice/array before proceeding
            if sliceValue.Kind() != reflect.Slice && sliceValue.Kind() != reflect.Array {
                // Fallback to safe formatting for mixed-type arrays
                return fmt.Sprintf("Mixed-type array (cannot format uniformly): %v", v)
            }

            // Build the page notation with appropriate number of colons
            var colons []string
            for i := 0; i < len(dimensions)-1; i++ {
                colons = append(colons, ":")
            }
            pageNotation := fmt.Sprintf("[%s, %d]", str.Join(colons, ", "), k+1)

            // Recursively format this (N-1)D slice
            pageContent := formatArrayNDPages(sliceValue.Interface(), elemType, remainingDims)

            // Add page header
            result = append(result, fmt.Sprintf("%s =", pageNotation))
            result = append(result, pageContent)

            if k < firstDim-1 {
                result = append(result, "") // Add blank line between pages
            }
        }

        return fmt.Sprintf("%s Array{%s, %d}:\n%s", dimStr, elemType, len(dimensions), str.Join(result, "\n"))
    }

    // Shouldn't reach here
    return fmt.Sprintf("%s Array{%s}: []", dimStr, elemType)
}

func format3DFromValue(rv reflect.Value, elemType string, dimensions []int) string {
    d1, d2, d3 := dimensions[0], dimensions[1], dimensions[2]

    var result []string

    // Show each 2D slice within this 3D array
    for k := 0; k < d1; k++ {
        slice := rv.Index(k)

        // Handle case where slice is an interface containing a slice
        var sliceValue reflect.Value
        if slice.Kind() == reflect.Interface {
            sliceValue = reflect.ValueOf(slice.Interface())
        } else {
            sliceValue = slice
        }

        // Validate that sliceValue is actually a slice/array before proceeding
        if sliceValue.Kind() != reflect.Slice && sliceValue.Kind() != reflect.Array {
            // Fallback to safe formatting for mixed-type arrays
            return fmt.Sprintf("Mixed-type array (cannot format uniformly): %v", rv.Interface())
        }

        var sliceLines []string

        for i := 0; i < d2; i++ {
            var rowElements []string
            row := sliceValue.Index(i)

            // Handle case where row is an interface containing a slice
            var rowSlice reflect.Value
            if row.Kind() == reflect.Interface {
                rowSlice = reflect.ValueOf(row.Interface())
            } else {
                rowSlice = row
            }

            // Validate that rowSlice is actually a slice/array before proceeding
            if rowSlice.Kind() != reflect.Slice && rowSlice.Kind() != reflect.Array {
                // Fallback to safe formatting for mixed-type arrays
                return fmt.Sprintf("Mixed-type array (cannot format uniformly): %v", rv.Interface())
            }

            for j := 0; j < d3; j++ {
                elem := rowSlice.Index(j).Interface()
                // Apply display precision reduction when array_format() is enabled
                if prettyArrays {
                    if bf, ok := elem.(*big.Float); ok {
                        // Create display copy with reduced precision
                        displayCopy := new(big.Float).Copy(bf)
                        displayCopy.SetPrec(64) // Reduce to display precision
                        elem = displayCopy
                    }
                }
                colour := getDepthColour(2) // Elements in 3D array are at depth 2
                reset := ""
                if colour != "" {
                    reset = "[#-]"
                }
                rowElements = append(rowElements, fmt.Sprintf("%s%v%s", colour, elem, reset))
            }
            sliceLines = append(sliceLines, str.Join(rowElements, " "))
        }

        if k < d1-1 {
            result = append(result, fmt.Sprintf("[:, :, %d] =", k+1))
        } else {
            result = append(result, fmt.Sprintf("[:, :, %d] =", k+1))
        }
        result = append(result, sliceLines...)
        if k < d1-1 {
            result = append(result, "")
        }
    }

    return str.Join(result, "\n")
}

func formatArrayND(v any, elemType string, dimensions []int) string {
    rv := reflect.ValueOf(v)

    // Build dimension string like "2x3x4"
    var dimStrs []string
    for _, dim := range dimensions {
        dimStrs = append(dimStrs, fmt.Sprintf("%d", dim))
    }
    dimStr := str.Join(dimStrs, "x")

    if len(dimensions) == 0 {
        return fmt.Sprintf("%s Array{%s}: []", dimStr, elemType)
    }

    // For 3D+ arrays, show slice by slice
    var result []string

    // Handle 3D case specially for better formatting
    if len(dimensions) == 3 {
        d1, d2, d3 := dimensions[0], dimensions[1], dimensions[2]

        // Show each 2D slice
        for k := 0; k < d1; k++ {
            slice := rv.Index(k)

            // Handle case where slice is an interface containing a slice
            var sliceValue reflect.Value
            if slice.Kind() == reflect.Interface {
                sliceValue = reflect.ValueOf(slice.Interface())
            } else {
                sliceValue = slice
            }

            var sliceLines []string

            for i := 0; i < d2; i++ {
                var rowElements []string
                row := sliceValue.Index(i)

                // Handle case where row is an interface containing a slice
                var rowSlice reflect.Value
                if row.Kind() == reflect.Interface {
                    rowSlice = reflect.ValueOf(row.Interface())
                } else {
                    rowSlice = row
                }

                for j := 0; j < d3; j++ {
                    elem := rowSlice.Index(j).Interface()
                    colour := getDepthColour(2) // Elements in 3D array are at depth 2
                    reset := ""
                    if colour != "" {
                        reset = "[#-]"
                    }
                    rowElements = append(rowElements, fmt.Sprintf("%s%v%s", colour, elem, reset))
                }
                sliceLines = append(sliceLines, str.Join(rowElements, " "))
            }

            if k < d1-1 {
                result = append(result, fmt.Sprintf("[:, :, %d] =", k+1))
            } else {
                result = append(result, fmt.Sprintf("[:, :, %d] =", k+1))
            }
            result = append(result, sliceLines...)
            if k < d1-1 {
                result = append(result, "")
            }
        }

        return fmt.Sprintf("%dx%dx%d Array{%s, 3}:\n%s", d1, d2, d3, elemType, str.Join(result, "\n"))
    }

    // For 4D+ arrays, use page-based formatting
    if len(dimensions) >= 4 {
        return formatArrayNDPages(v, elemType, dimensions)
    }

    // Fallback (shouldn't reach here)
    return formatArrayNDRecursive(v, elemType, dimensions, 0, []int{})
}

func structToZaLiteral(v any) string {
    vVal := reflect.ValueOf(v)
    if vVal.Kind() == reflect.Ptr {
        vVal = vVal.Elem()
    }

    if vVal.Kind() != reflect.Struct {
        panic(fmt.Sprintf("structToZaLiteral: expected struct, got %T", v))
    }

    var parts []string
    typ := vVal.Type()
    for i := 0; i < vVal.NumField(); i++ {
        field := vVal.Field(i)
        fieldType := typ.Field(i)

        // Skip unexported fields
        if !fieldType.IsExported() {
            continue
        }

        fieldName := fieldType.Name
        fieldValue := goLiteralToZaLiteral(field.Interface())
        parts = append(parts, sf("%s: %s", fieldName, fieldValue))
    }

    return "{" + str.Join(parts, ", ") + "}"
}

func genericMapToZaLiteral(v any) string {
    vVal := reflect.ValueOf(v)
    if vVal.Kind() == reflect.Ptr {
        vVal = vVal.Elem()
    }

    if vVal.Kind() != reflect.Map {
        panic(fmt.Sprintf("genericMapToZaLiteral: expected map, got %T", v))
    }

    if vVal.Len() == 0 {
        return "map()"
    }

    var parts []string
    iter := vVal.MapRange()
    for iter.Next() {
        key := iter.Key()
        value := iter.Value()

        // Convert key to string
        var keyStr string
        switch key.Kind() {
        case reflect.String:
            keyStr = key.String()
        default:
            keyStr = goLiteralToZaLiteral(key.Interface())
        }

        valueStr := goLiteralToZaLiteral(value.Interface())
        parts = append(parts, sf("%s %s", keyStr, valueStr))
    }

    return "map(" + str.Join(parts, ", ") + ")"
}

func int8ArrayToZaLiteral(arr []int8) string {
    if len(arr) == 0 {
        return "[]"
    }

    var parts []string
    for _, item := range arr {
        parts = append(parts, strconv.FormatInt(int64(item), 10))
    }
    return "[" + str.Join(parts, ", ") + "]"
}

func int16ArrayToZaLiteral(arr []int16) string {
    if len(arr) == 0 {
        return "[]"
    }

    var parts []string
    for _, item := range arr {
        parts = append(parts, strconv.FormatInt(int64(item), 10))
    }
    return "[" + str.Join(parts, ", ") + "]"
}

func int32ArrayToZaLiteral(arr []int32) string {
    if len(arr) == 0 {
        return "[]"
    }

    var parts []string
    for _, item := range arr {
        parts = append(parts, strconv.FormatInt(int64(item), 10))
    }
    return "[" + str.Join(parts, ", ") + "]"
}

func int64ArrayToZaLiteral(arr []int64) string {
    if len(arr) == 0 {
        return "[]"
    }

    var parts []string
    for _, item := range arr {
        parts = append(parts, strconv.FormatInt(item, 10))
    }
    return "[" + str.Join(parts, ", ") + "]"
}

func uintArrayToZaLiteral(arr []uint) string {
    if len(arr) == 0 {
        return "[]"
    }

    var parts []string
    for _, item := range arr {
        parts = append(parts, strconv.FormatUint(uint64(item), 10))
    }
    return "[" + str.Join(parts, ", ") + "]"
}

func uint8ArrayToZaLiteral(arr []uint8) string {
    if len(arr) == 0 {
        return "[]"
    }

    var parts []string
    for _, item := range arr {
        parts = append(parts, strconv.FormatUint(uint64(item), 10))
    }
    return "[" + str.Join(parts, ", ") + "]"
}

func uint16ArrayToZaLiteral(arr []uint16) string {
    if len(arr) == 0 {
        return "[]"
    }

    var parts []string
    for _, item := range arr {
        parts = append(parts, strconv.FormatUint(uint64(item), 10))
    }
    return "[" + str.Join(parts, ", ") + "]"
}

func uint32ArrayToZaLiteral(arr []uint32) string {
    if len(arr) == 0 {
        return "[]"
    }

    var parts []string
    for _, item := range arr {
        parts = append(parts, strconv.FormatUint(uint64(item), 10))
    }
    return "[" + str.Join(parts, ", ") + "]"
}

func uint64ArrayToZaLiteral(arr []uint64) string {
    if len(arr) == 0 {
        return "[]"
    }

    var parts []string
    for _, item := range arr {
        parts = append(parts, strconv.FormatUint(item, 10))
    }
    return "[" + str.Join(parts, ", ") + "]"
}

func genericArrayToZaLiteral(v any) string {
    vVal := reflect.ValueOf(v)
    if vVal.Kind() == reflect.Ptr {
        vVal = vVal.Elem()
    }

    if vVal.Kind() != reflect.Slice && vVal.Kind() != reflect.Array {
        panic(fmt.Sprintf("genericArrayToZaLiteral: expected slice/array, got %T", v))
    }

    if vVal.Len() == 0 {
        return "[]"
    }

    var parts []string
    for i := 0; i < vVal.Len(); i++ {
        element := vVal.Index(i).Interface()
        parts = append(parts, goLiteralToZaLiteral(element))
    }

    return "[" + str.Join(parts, ", ") + "]"
}

// replaceAllArrayIndexing replaces ALL #[index] patterns in a string with actual array elements
func replaceAllArrayIndexing(expr string, array any) (string, error) {
    result := expr

    for str.Contains(result, "#[") {
        hash_bracket_pos := str.Index(result, "#[")
        if hash_bracket_pos == -1 {
            break
        }

        close_bracket_pos := str.Index(result[hash_bracket_pos+2:], "]")
        if close_bracket_pos == -1 {
            break
        }

        index_str := result[hash_bracket_pos+2 : hash_bracket_pos+2+close_bracket_pos]
        index, err := strconv.Atoi(index_str)
        if err != nil || index < 0 {
            return "", fmt.Errorf("invalid array index: %s", index_str)
        }

        var element_str string
        var valid_index bool

        // Handle different slice types
        switch arr := array.(type) {
        case []any:
            if index < len(arr) {
                element := arr[index]
                switch elem := element.(type) {
                case string:
                    element_str = `"` + elem + `"`
                case int:
                    element_str = strconv.FormatInt(int64(elem), 10)
                case uint:
                    element_str = strconv.FormatUint(uint64(elem), 10)
                case float64:
                    element_str = strconv.FormatFloat(elem, 'g', -1, 64)
                case bool:
                    element_str = strconv.FormatBool(elem)
                case nil:
                    element_str = "nil"
                default:
                    element_str = goLiteralToZaLiteral(elem)
                }
                valid_index = true
            }
        case []int:
            if index < len(arr) {
                element := arr[index]
                element_str = strconv.FormatInt(int64(element), 10)
                valid_index = true
            }
        case []float64:
            if index < len(arr) {
                element := arr[index]
                element_str = strconv.FormatFloat(element, 'g', -1, 64)
                valid_index = true
            }
        case []string:
            if index < len(arr) {
                element := arr[index]
                element_str = `"` + element + `"`
                valid_index = true
            }
        case []bool:
            if index < len(arr) {
                element := arr[index]
                element_str = strconv.FormatBool(element)
                valid_index = true
            }
        case []*big.Int:
            if index < len(arr) {
                element := arr[index]
                element_str = element.String()
                valid_index = true
            }
        case []*big.Float:
            if index < len(arr) {
                element := arr[index]
                element_str = element.String()
                valid_index = true
            }
        case [][]int:
            if index < len(arr) {
                element := arr[index]
                element_str = goLiteralToZaLiteral(element)
                valid_index = true
            }
        }

        if !valid_index {
            return "", fmt.Errorf("array index %d out of bounds", index)
        }

        pattern_to_replace := result[hash_bracket_pos : hash_bracket_pos+2+close_bracket_pos+1]
        result = str.Replace(result, pattern_to_replace, element_str, -1)
    }

    return result, nil
}

func replaceMapFieldAccess(expr string, item any) (string, error) {
    // Parse and replace #.field patterns with actual values from the map
    result := expr

    // Check if item is a map
    mapValue, ok := item.(map[string]any)
    if !ok {
        return "", fmt.Errorf("expected map[string]any for field access, got %T", item)
    }

    for {
        // Find the next #. pattern
        start := str.Index(result, "#.")
        if start == -1 {
            break // No more field access patterns found
        }

        // Extract the field name - continue until we hit a non-identifier character
        fieldName := ""
        pos := start + 2 // Skip #.

        for pos < len(result) {
            ch := result[pos]
            // Allow alphanumeric, underscore, and hyphen in field names (like map literals)
            // Stop at whitespace, operators, dots, brackets, or backticks
            if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '_' || ch == '-' {
                fieldName += string(ch)
                pos++
            } else {
                break
            }
        }

        if fieldName == "" {
            return "", fmt.Errorf("empty field name in expression: %s", result)
        }

        // Capitalize field name for struct field access
        capitalizedFieldName := renameSF(fieldName)

        // Look up the field value in the map - try original first, then capitalized
        fieldValue, exists := mapValue[fieldName]
        if !exists {
            fieldValue, exists = mapValue[capitalizedFieldName]
            if !exists {
                return "", fmt.Errorf("field '%s' not found in map", fieldName)
            }
        }

        // Convert the field value to a Za literal
        var valueStr string
        switch fv := fieldValue.(type) {
        case string:
            valueStr = `"` + fv + `"`
        case int:
            valueStr = strconv.FormatInt(int64(fv), 10)
        case uint:
            valueStr = strconv.FormatUint(uint64(fv), 10)
        case float64:
            valueStr = strconv.FormatFloat(fv, 'g', -1, 64)
        case bool:
            valueStr = strconv.FormatBool(fv)
        case map[string]any:
            // Convert map to Za literal format
            valueStr = mapToZaLiteral(fv)
        case []any:
            // Convert array to Za literal format
            valueStr = arrayToZaLiteral(fv)
        case []*big.Int:
            // Convert big int array to Za literal format
            valueStr = bigIntArrayToZaLiteral(fv)
        case []*big.Float:
            // Convert big float array to Za literal format
            valueStr = bigFloatArrayToZaLiteral(fv)
        default:
            valueStr = goLiteralToZaLiteral(fv)
        }

        // Replace the #.field pattern with the actual value
        result = result[:start] + valueStr + result[pos:]
    }

    return result, nil
}
