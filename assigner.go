// Version: 2.0.0
// Last Change: Refactor assignment logic to be unified, correct, and maintainable.
// Changes:
// - Removed the fast/slow path distinction. `assigner_fast.go` is now obsolete.
// - Deleted unused `checkStructCompatibility` and other dead code.
// - Replaced the monolithic `executeAssignment` with a cleaner, more modular implementation.
// - Correctly handles struct-in-container assignments to preserve type, fixing the core bug.
package main

import (
    "fmt"
    "math"
    "math/big"
    "reflect"
    "strconv"
    "unsafe"
)

// debug_assignment is a helper to print debug messages only when the feature flag is enabled.
func debug_assignment(format string, a ...any) {
    if F_EnableComplexAssignments {
        pf(format, a...)
    }
}

// dumpVal provides a detailed debug print of a reflect.Value's state.
func dumpVal(name string, v reflect.Value) {
    if !F_EnableComplexAssignments {
        return
    }
    if !v.IsValid() {
        debug_assignment("  - DUMP %s: Invalid reflect.Value\n", name)
        return
    }
    debug_assignment("  - DUMP %s: Type: %v, Kind: %v, IsValid: %v, CanAddr: %v, CanSet: %v\n",
        name, v.Type(), v.Kind(), v.IsValid(), v.CanAddr(), v.CanSet())
}

// intStrCache provides a cache for small integer to string conversions to reduce allocations.
var intStrCache [1024]string

// intToString is a faster version of strconv.Itoa for small, non-negative integers.
func intToString(i int) string {
    if i >= 0 && i < len(intStrCache) {
        return intStrCache[i]
    }
    return strconv.Itoa(i)
}

// unsafeSet bypasses Go's visibility rules by performing a raw memory copy.
// It is the key to handling unexported struct fields. It works by constructing
// an interface{} header for the source value to get a pointer to its underlying
// data, then copying that data to the destination's address.
func unsafeSet(dest, src reflect.Value) {
    if !dest.CanAddr() {
        // This should be caught by the caller, but as a safeguard.
        panic("unsafeSet called on non-addressable destination")
    }
    destPtr := dest.UnsafeAddr()

    var srcDataPtr unsafe.Pointer

    // To get a pointer to the source data, we use a robust whitelist. The fragile
    // "interface header trick" is only used for types that are known to be
    // pointer-based. All other types (structs, primitives, etc.) use the safe
    // method of creating a temporary variable to get a stable data pointer.
    // This prevents memory corruption from the header trick being used on an
    // unsupported type.
    switch src.Kind() {
    case reflect.Chan, reflect.Func, reflect.Map, reflect.Ptr, reflect.Slice, reflect.String:
        var srcIface any
        ifacePtr := (*[2]unsafe.Pointer)(unsafe.Pointer(&srcIface))
        srcTyp := src.Type()
        ifacePtr[0] = (*[2]unsafe.Pointer)(unsafe.Pointer(&srcTyp))[1]
        ifacePtr[1] = (*[2]unsafe.Pointer)(unsafe.Pointer(&src))[1]
        srcDataPtr = (*[2]unsafe.Pointer)(unsafe.Pointer(&srcIface))[1]
    default:
        tempVal := reflect.New(src.Type()).Elem()
        tempVal.Set(src)
        srcDataPtr = tempVal.Addr().UnsafePointer()
    }

    size := src.Type().Size()
    if size > 0 {
        destSlice := unsafe.Slice((*byte)(unsafe.Pointer(destPtr)), size)
        srcSlice := unsafe.Slice((*byte)(srcDataPtr), size)
        copy(destSlice, srcSlice)
    }
}

// AccessType represents different ways to access data structures (array indexing, map lookup, etc)
type AccessType int

const (
    AccessVariable AccessType = iota
    AccessArray
    AccessMap
    AccessField
)

// Access represents a single step in accessing nested data structures
// Key is used for array indices and map keys
// Field is used for struct field access
type Access struct {
    Type  AccessType
    Key   any    // For array/map access
    Field string // For struct access
}

func (a Access) String() string {
    t := []string{"variable", "array", "map", "field"}[a.Type]
    return sf("Access(t %s,k %v,f %s)", t, a.Key, a.Field)
}

// Chain represents the complete path to access a nested value
// Name is the root variable name
// Accesses contains the sequence of operations to reach the target
// FinalType is the expected type after all accesses
type Chain struct {
    Name      string
    Accesses  []Access
    BindPos   uint64 // Store bind position to avoid passing lident around.
    FinalType reflect.Type
}

// Typemap provides type mapping for Za types to Go reflect.Types.
// This is initialized once at package level to avoid recreation and locking issues.
var Typemap map[string]reflect.Type

func init() {
    for i := 0; i < len(intStrCache); i++ {
        intStrCache[i] = strconv.Itoa(i)
    }

    var tb bool
    var tu uint
    var tu8 uint8
    var tu32 uint32
    var tu64 uint64
    var ti int
    var tf64 float64
    var ts string
    var tbi *big.Int
    var tbf *big.Float
    var tmixed any

    var stb []bool
    var stu []uint
    var stu8 []uint8
    var stu32 []uint32
    var stu64 []uint64
    var sti []int
    var stf64 []float64
    var sts []string
    var stbi []*big.Int
    var stbf []*big.Float
    var stmixed []any

    Typemap = make(map[string]reflect.Type)
    Typemap["bool"] = reflect.TypeOf(tb)
    Typemap["uint"] = reflect.TypeOf(tu)
    Typemap["uint8"] = reflect.TypeOf(tu8)
    Typemap["uint32"] = reflect.TypeOf(tu32)
    Typemap["uint64"] = reflect.TypeOf(tu64)
    Typemap["ulong"] = reflect.TypeOf(tu32)
    Typemap["uxlong"] = reflect.TypeOf(tu64)
    Typemap["byte"] = reflect.TypeOf(tu8)
    Typemap["int"] = reflect.TypeOf(ti)
    Typemap["float"] = reflect.TypeOf(tf64)
    Typemap["bigi"] = reflect.TypeOf(tbi)
    Typemap["bigf"] = reflect.TypeOf(tbf)
    Typemap["string"] = reflect.TypeOf(ts)
    Typemap["mixed"] = reflect.TypeOf(tmixed)
    Typemap["any"] = reflect.TypeOf(tmixed)
    Typemap["[]bool"] = reflect.TypeOf(stb)
    Typemap["[]uint"] = reflect.TypeOf(stu)
    Typemap["[]uint8"] = reflect.TypeOf(stu8)
    Typemap["[]byte"] = reflect.TypeOf(stu8)
    Typemap["[]int"] = reflect.TypeOf(sti)
    Typemap["[]uint32"] = reflect.TypeOf(stu32)
    Typemap["[]uint64"] = reflect.TypeOf(stu64)
    Typemap["[]float"] = reflect.TypeOf(stf64)
    Typemap["[]string"] = reflect.TypeOf(sts)
    Typemap["[]bigi"] = reflect.TypeOf(stbi)
    Typemap["[]bigf"] = reflect.TypeOf(stbf)
    Typemap["[]mixed"] = reflect.TypeOf(stmixed)
    Typemap["[]any"] = reflect.TypeOf(stmixed)
    Typemap["[]"] = reflect.TypeOf(stmixed)
    Typemap["map"] = nil
}

// growSlice ensures a slice has enough capacity and length for a given index.
// It returns a new slice if growth was necessary.
func growSlice(slice reflect.Value, index int, valueForTyping any) (reflect.Value, error) {
    debug_assignment("[DEBUG] ENTER: growSlice. index: %d\n", index)

    // If the incoming value is invalid (e.g., from a nil interface), it means
    // we are performing indexed access on something that doesn't exist yet.
    // We must "auto-vivify" a new slice to hold the value. The type of the
    // new slice is inferred from the value being assigned.
    if !slice.IsValid() {
        debug_assignment("  - growSlice: auto-vivifying a new slice from invalid value\n")
        elemType := reflect.TypeOf(valueForTyping)
        // This handles the case where a nil is being assigned to a new variable.
        // We default to creating a slice of interfaces ([]any).
        if elemType == nil {
            elemType = reflect.TypeOf((*any)(nil)).Elem()
        }
        sliceType := reflect.SliceOf(elemType)
        newSlice := reflect.MakeSlice(sliceType, index+1, index+1)
        return newSlice, nil
    }

    // Guard against calling Len/Cap on non-slice types in debug logging.
    if slice.Kind() == reflect.Slice {
        debug_assignment("  - slice: Type: %v, Kind: %v, Len: %d, Cap: %d\n", slice.Type(), slice.Kind(), slice.Len(), slice.Cap())
    } else {
        debug_assignment("  - slice: Type: %v, Kind: %v\n", slice.Type(), slice.Kind())
    }

    // If we get an interface, it's likely from a container like []any. If it's
    // nil, we can safely replace it with a new slice (auto-vivification).
    if slice.Kind() == reflect.Interface {
        if slice.IsNil() {
            debug_assignment("  - growSlice: auto-vivifying a new []any slice from nil interface\n")
            newSlice := reflect.MakeSlice(reflect.TypeOf([]any{}), index+1, index+1)
            return newSlice, nil
        }
        // If the interface is not nil, unwrap it and proceed.
        slice = slice.Elem()
    }

    if slice.Kind() != reflect.Slice {
        // Attempt to handle array-like access on a nil or empty map. This is a
        // common case when a variable is declared but not initialized.
        if slice.Kind() == reflect.Map && slice.Len() == 0 {
            debug_assignment("  - growSlice: auto-vivifying a new []any slice from empty map\n")
            newSlice := reflect.MakeSlice(reflect.TypeOf([]any{}), index+1, index+1)
            return newSlice, nil
        }
        return slice, fmt.Errorf("cannot perform array access on non-slice type: %v", slice.Type())
    }

    if index >= slice.Len() {
        debug_assignment("  - growSlice: growing slice\n")
        newSize := index + 1
        newCap := slice.Cap()
        if newSize > newCap {
            newCap = newSize
            if newCap < 2*slice.Cap() {
                newCap = 2 * slice.Cap()
            }
        }
        newSlice := reflect.MakeSlice(slice.Type(), newSize, newCap)
        reflect.Copy(newSlice, slice)
        debug_assignment("  - growSlice: new slice: Len: %d, Cap: %d\n", newSlice.Len(), newSlice.Cap())
        return newSlice, nil
    }
    debug_assignment("  - growSlice: no growth needed\n")
    return slice, nil
}

// convertAssignmentValue handles type conversions and checking for assignments.
func convertAssignmentValue(targetType reflect.Type, value any) (any, error) {
    debug_assignment("[DEBUG] ENTER: convertAssignmentValue. targetType: %v, value: %#v (%T)\n", targetType, value, value)

    // If targetType is nil, it signifies assignment to a new key in a map or a new
    // element in a slice that is being auto-vivified. In this case, there's no
    // existing type to convert to, so we accept the new value as-is.
    if targetType == nil {
        debug_assignment("  - convert: nil targetType, accepting value as-is\n")
        return value, nil
    }

    // Handle nil value - always allowed for auto-vivified containers
    if value == nil {
        debug_assignment("  - convert: nil path\n")
        switch targetType.Kind() {
        case reflect.Interface, reflect.Slice, reflect.Map, reflect.Ptr, reflect.Chan:
            return nil, nil
        default:
            return nil, fmt.Errorf("cannot assign nil to %v", targetType)
        }
    }

    // Cache reflection value to avoid repeated calls
    valueReflect := reflect.ValueOf(value)
    valueKind := valueReflect.Kind()

    // Fast path for big number types using direct type comparison
    if targetType == Typemap["bigi"] {
        debug_assignment("  - convert: bigi path\n")
        if value == nil {
            return nil, fmt.Errorf("cannot convert nil to big.Int")
        }
        if valueKind == reflect.Int || valueKind == reflect.Int32 || valueKind == reflect.Int64 {
            return GetAsBigInt(valueReflect.Interface()), nil
        }
        if valueKind == reflect.Interface {
            return GetAsBigInt(valueReflect.Elem().Interface()), nil
        }
        return GetAsBigInt(value), nil
    }
    if targetType == Typemap["bigf"] {
        debug_assignment("  - convert: bigf path\n")
        if value == nil {
            return nil, fmt.Errorf("cannot convert nil to big.Float")
        }
        if valueKind == reflect.Float64 || valueKind == reflect.Float32 ||
            valueKind == reflect.Int || valueKind == reflect.Int32 || valueKind == reflect.Int64 {
            return GetAsBigFloat(valueReflect.Interface()), nil
        }
        if valueKind == reflect.Interface {
            return GetAsBigFloat(valueReflect.Elem().Interface()), nil
        }
        return GetAsBigFloat(value), nil
    }

    // Cache reflection values
    valueType := reflect.TypeOf(value)
    targetKind := targetType.Kind()

    // Handle interface special cases
    if targetKind == reflect.Interface {
        debug_assignment("  - convert: interface path\n")
        if valueType != nil && valueType.AssignableTo(targetType) {
            debug_assignment("    - assignable to interface\n")
            return value, nil
        }
        // Allow any value to be assigned to an empty interface (`any` or `interface{}`)
        if targetType.NumMethod() == 0 {
            debug_assignment("    - empty interface\n")
            return value, nil
        }
        return nil, fmt.Errorf("type %v does not implement interface %v", valueType, targetType)
    }

    // Handle array/slice element assignment
    if targetKind == reflect.Slice || targetKind == reflect.Array {
        debug_assignment("  - convert: slice/array path\n")
        valueKind := valueReflect.Kind()

        // Case 1: Assigning a whole slice/array to another slice/array
        if valueKind == reflect.Slice || valueKind == reflect.Array {
            debug_assignment("    - whole slice/array assignment path\n")
            sourceType := valueReflect.Type()
            targetElemType := targetType.Elem()
            sourceElemType := sourceType.Elem()

            // Fast path: if element types are directly assignable, just return the original value.
            // The caller (reflect.Set) will handle the assignment.
            if sourceElemType.AssignableTo(targetElemType) {
                debug_assignment("      - elements are directly assignable\n")
                return value, nil
            }

            // Slow path: element-by-element conversion is needed.
            debug_assignment("      - elements require conversion\n")
            sourceLen := valueReflect.Len()
            newSlice := reflect.MakeSlice(targetType, sourceLen, sourceLen)

            for i := 0; i < sourceLen; i++ {
                sourceElem := valueReflect.Index(i).Interface()
                // Recursively call convertAssignmentValue for each element.
                convertedElem, err := convertAssignmentValue(targetElemType, sourceElem)
                if err != nil {
                    return nil, fmt.Errorf("error converting slice element at index %d: %w", i, err)
                }
                // reflect.ValueOf(nil) is not valid, so handle that case by setting a zero value.
                if convertedElem == nil {
                    newSlice.Index(i).Set(reflect.Zero(targetElemType))
                } else {
                    val := reflect.ValueOf(convertedElem)
                    if val.Type().AssignableTo(targetElemType) {
                        newSlice.Index(i).Set(val)
                    } else {
                        return nil, fmt.Errorf("converted element of type %v is not assignable to slice element type %v", val.Type(), targetElemType)
                    }
                }
            }
            return newSlice.Interface(), nil
        }
        // If the target is a slice but the value is not, we fall through to the
        // generic assignability checks below. This correctly handles invalid
        // assignments like `slice = map`.
    }

    // Handle map element assignment
    if targetKind == reflect.Map {
        debug_assignment("  - convert: map path\n")
        if valueType != nil {
            valueKind := valueType.Kind()
            // For whole map assignment
            if valueKind == reflect.Map {
                // For untyped, allow if key/value types are compatible
                if valueType.Key().AssignableTo(targetType.Key()) &&
                    valueType.Elem().AssignableTo(targetType.Elem()) {
                    debug_assignment("    - whole map assignable\n")
                    return value, nil
                }
            }
        }
        // For element assignment (m[k] = v), use element type
        elemType := targetType.Elem()
        return convertAssignmentValue(elemType, value)
    }

    // For untyped variables:
    // 1. Check direct assignability
    if valueType != nil && valueType.AssignableTo(targetType) {
        debug_assignment("  - convert: direct assignable path\n")
        return value, nil
    }

    // 2. Try conversion for basic types only
    if valueType != nil && valueType.ConvertibleTo(targetType) {
        debug_assignment("  - convert: convertible path\n")
        // Special handling for uint32
        if targetType.Kind() == reflect.Uint32 {
            switch v := value.(type) {
            case int:
                if v >= 0 && v <= math.MaxUint32 {
                    return uint32(v), nil
                }
                return nil, fmt.Errorf("int value %d out of range for uint32", v)
            case uint32:
                return v, nil
            default:
                return nil, fmt.Errorf("cannot convert %T to uint32", value)
            }
        }

        // Other basic type conversions
        switch targetType.Kind() {
        case reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
            reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint64,
            reflect.Float32, reflect.Float64, reflect.String:
            debug_assignment("    - basic type convertible\n")
            return reflect.ValueOf(value).Convert(targetType).Interface(), nil
        }
    }

    // FINAL FALLBACK: If no other conversion rule matches, return the original value.
    // The subsequent assignment logic (e.g., reflect.Set) will be the final arbiter
    // of whether the assignment is valid. This allows for more dynamic assignments.
    debug_assignment("  - convert: FALLBACK, returning original value\n")
    return value, nil
}

// resolveStructLiteral checks if a value is an anonymous struct literal that
// matches a single, known struct definition. If so, it replaces the literal
// with a new instance of that a properly-typed, named struct. This is the
// key to fixing errors caused by the mismatch between anonymous literals and
// the interpreter's internal struct representation.
func resolveStructLiteral(val any) any {
    if val == nil {
        return nil
    }
    if reflect.TypeOf(val).Kind() != reflect.Struct {
        return val
    }

    if name, count := struct_match(val); count == 1 {
        // We have a unique match. Create a new instance of the correct type.
        fieldDefs := structmaps[name]
        if fieldDefs == nil {
            return val // Should not happen if struct_match passed.
        }

        sfields := make([]reflect.StructField, 0, len(fieldDefs)/4)
        for i := 0; i < len(fieldDefs)/4; i++ {
            sfields = append(sfields, reflect.StructField{
                Name: fieldDefs[i*4].(string),
                Type: Typemap[fieldDefs[i*4+1].(string)],
            })
        }
        namedStructType := reflect.StructOf(sfields)
        newInstance := reflect.New(namedStructType).Elem()

        // Copy values from the anonymous literal to the new named instance.
        literal := reflect.ValueOf(val)
        for i := 0; i < literal.NumField(); i++ {
            // The field in the new instance is unexported, so we must use unsafeSet.
            unsafeSet(newInstance.Field(i), literal.Field(i))
        }
        return newInstance.Interface()
    }

    return val
}

/*
handleFieldAssignment is a helper function to handle direct field assignments
on a struct variable (e.g., `variable.field = value`). It fetches the struct,
creates a mutable copy, sets the field on the copy, and writes the modified
struct back. This "copy-modify-replace" strategy avoids Go's reflection errors
with unexported fields.
*/
func handleFieldAssignment(lfs, rfs uint32, lident *[]Variable, varToken Token, fieldName string, value any) error {
    ts, found := vget(&varToken, lfs, lident, varToken.tokText)
    if !found {
        return fmt.Errorf("record variable %v not found", varToken.tokText)
    }

    val := reflect.ValueOf(ts)
    if val.Kind() != reflect.Struct {
        return fmt.Errorf("variable %v is not a struct", varToken.tokText)
    }

    tmp := reflect.New(val.Type()).Elem()
    tmp.Set(val)

    field := tmp.FieldByName(fieldName)
    if !field.IsValid() {
        return fmt.Errorf("field %v not found in struct %v", fieldName, varToken.tokText)
    }

    if !field.CanSet() {
        field = reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem()
    }

    if value == nil {
        field.Set(reflect.Zero(field.Type()))
    } else {
        valToSet := reflect.ValueOf(value)
        if valToSet.Type().AssignableTo(field.Type()) {
            field.Set(valToSet)
        } else {
            return fmt.Errorf("cannot assign result (%T) to %v.%v (%v)", value, varToken.tokText, fieldName, field.Type())
        }
    }

    var tok *Token
    if lfs == rfs {
        tok = &varToken
    }
    vset(tok, lfs, lident, varToken.tokText, tmp.Interface())
    return nil
}

/*
handleMapOrArrayFieldAssignment handles field assignments on structs that are
elements of a map or array (e.g., `map[key].field = value`). It retrieves the
enclosing map/array, creates a mutable copy of the target struct, modifies the
field on the copy, and then replaces the original struct in the container with
the modified version.
*/
func handleMapOrArrayFieldAssignment(lfs, rfs uint32, lident *[]Variable, varToken Token, key any, fieldName string, value any) error {
    container, found := vget(&varToken, lfs, lident, varToken.tokText)
    if !found {
        return fmt.Errorf("container variable %v not found", varToken.tokText)
    }

    var skey string
    var ikey int
    isMap := false

    switch k := key.(type) {
    case string:
        skey = k
        isMap = true
    case int:
        ikey = k
    default:
        return fmt.Errorf("unsupported key type %T", key)
    }

    containerVal := reflect.ValueOf(container)
    var targetStruct reflect.Value

    if isMap {
        if containerVal.Kind() != reflect.Map {
            return fmt.Errorf("variable %s is not a map", varToken.tokText)
        }
        mapVal := containerVal.MapIndex(reflect.ValueOf(skey))
        if !mapVal.IsValid() {
            return fmt.Errorf("key %s not found in map %s", skey, varToken.tokText)
        }
        targetStruct = mapVal.Elem()
    } else {
        if containerVal.Kind() != reflect.Slice && containerVal.Kind() != reflect.Array {
            return fmt.Errorf("variable %s is not an array or slice", varToken.tokText)
        }
        if ikey >= containerVal.Len() {
            return fmt.Errorf("index %d out of bounds for %s", ikey, varToken.tokText)
        }
        targetStruct = containerVal.Index(ikey).Elem()
    }

    if targetStruct.Kind() != reflect.Struct {
        return fmt.Errorf("element at %v is not a struct", key)
    }

    tmp := reflect.New(targetStruct.Type()).Elem()
    tmp.Set(targetStruct)

    field := tmp.FieldByName(fieldName)
    if !field.IsValid() {
        return fmt.Errorf("field %v not found in struct", fieldName)
    }

    if !field.CanSet() {
        field = reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem()
    }

    if value == nil {
        field.Set(reflect.Zero(field.Type()))
    } else {
        valToSet := reflect.ValueOf(value)
        if valToSet.Type().AssignableTo(field.Type()) {
            field.Set(valToSet)
        } else {
            return fmt.Errorf("cannot assign result (%T) to field %s (%v)", value, fieldName, field.Type())
        }
    }

    var tok *Token
    if lfs == rfs {
        tok = &varToken
    }
    vsetElement(tok, lfs, lident, varToken.tokText, key, tmp.Interface())
    return nil
}

func (p *leparser) doAssign(lfs uint32, lident *[]Variable, rfs uint32, rident *[]Variable, tks []Token, expr *ExpressionCarton, eqPos int, hasComma bool) {
    // --- Path A: Single Assignment ---
    if !hasComma {
        assignee := tks[:eqPos]
        value := expr.result

        la := len(assignee)

        if la == 1 { // a = val
            vset(&assignee[0], lfs, lident, assignee[0].tokText, value)
            return
        }
        if la == 3 && assignee[1].tokType == SYM_DOT { // a.f = val
            err := handleFieldAssignment(lfs, rfs, lident, assignee[0], assignee[2].tokText, value)
            if err != nil {
                expr.errVal = err
                expr.evalError = true
            }
            return
        }
        if la > 3 && assignee[1].tokType == LeftSBrace {
            rbPos := -1 // Find right bracket by searching backwards
            for i := la - 1; i >= 2; i-- {
                if assignee[i].tokType == RightSBrace {
                    rbPos = i
                    break
                }
            }

            if rbPos != -1 {
                // We've found a ']', so this is likely an indexed assignment.
                // We delay key evaluation until we know which path we're on
                // to create a smaller scope for the 'key' variable, which helps
                // the compiler's escape analysis.

                // Check for a[k].f = val
                if rbPos+2 < la && assignee[rbPos+1].tokType == SYM_DOT {
                    key, err := p.Eval(rfs, assignee[2:rbPos])
                    if err != nil {
                        expr.errVal = fmt.Errorf("could not evaluate index/key: %w", err)
                        expr.evalError = true
                        return
                    }
                    fieldName := assignee[rbPos+2].tokText
                    err = handleMapOrArrayFieldAssignment(lfs, rfs, lident, assignee[0], key, fieldName, value)
                    if err != nil {
                        expr.errVal = err
                        expr.evalError = true
                    }
                } else { // a[k] = val
                    key, err := p.Eval(rfs, assignee[2:rbPos])
                    if err != nil {
                        expr.errVal = fmt.Errorf("could not evaluate index/key: %w", err)
                        expr.evalError = true
                        return
                    }

                    switch k := key.(type) {
                    case int:
                        if k < 0 {
                            expr.errVal = fmt.Errorf("negative element index: %d", k)
                            expr.evalError = true
                            return
                        }
                        if lfs == rfs {
                            vsetElement(&assignee[0], lfs, lident, assignee[0].tokText, k, value)
                        } else {
                            vsetElement(nil, lfs, lident, assignee[0].tokText, k, value)
                        }
                    case string:
                        skey := interpolate(p.namespace, rfs, rident, k)
                        if lfs == rfs {
                            vsetElement(&assignee[0], lfs, lident, assignee[0].tokText, skey, value)
                        } else {
                            vsetElement(nil, lfs, lident, assignee[0].tokText, skey, value)
                        }
                    case int64:
                        if k < 0 {
                            expr.errVal = fmt.Errorf("negative element index: %d", k)
                            expr.evalError = true
                            return
                        }
                        if lfs == rfs {
                            vsetElement(&assignee[0], lfs, lident, assignee[0].tokText, int(k), value)
                        } else {
                            vsetElement(nil, lfs, lident, assignee[0].tokText, int(k), value)
                        }
                    default:
                        // Fallback for other types, convert to string
                        skey := GetAsString(key)
                        if lfs == rfs {
                            vsetElement(&assignee[0], lfs, lident, assignee[0].tokText, skey, value)
                        } else {
                            vsetElement(nil, lfs, lident, assignee[0].tokText, skey, value)
                        }
                    }
                }
                return // We handled this assignment.
            }
        }

        // Fallback for deeply nested assignments
        if F_EnableComplexAssignments {
            chain, err := p.parseAccessChain(assignee, lfs, lident, rfs, rident)
            if err != nil {
                expr.errVal = err
                expr.evalError = true
            }
            err = p.processAssignment(chain, value, lident)
            if err != nil {
                expr.errVal = err
                expr.evalError = true
            }
        } else {
            expr.errVal = fmt.Errorf("deeply nested assignments are not enabled; use F_EnableComplexAssignments=true")
            expr.evalError = true
        }
        return
    }

    // --- Path B: Multiple Assignment (Slower, requires allocation) ---
    largs := p.splitCommaArray(tks[:eqPos])
    val := reflect.ValueOf(expr.result)
    isSlice := val.IsValid() && val.Kind() == reflect.Slice
    numResults := 1
    if isSlice {
        numResults = val.Len()
    }
    if len(largs) > numResults && numResults > 1 {
        expr.errVal = fmt.Errorf("not enough values to unpack for assignment")
        expr.evalError = true
        return
    }

    for i, tokens := range largs {
        valueToSet := any(nil)
        if i < numResults {
            if isSlice {
                valueToSet = val.Index(i).Interface()
            } else {
                valueToSet = expr.result
            }
        }
        assignee := tokens
        la := len(assignee)

        if la == 1 { // a = val
            vset(&assignee[0], lfs, lident, assignee[0].tokText, valueToSet)
            continue
        }
        if la == 3 && assignee[1].tokType == SYM_DOT { // a.f = val
            err := handleFieldAssignment(lfs, rfs, lident, assignee[0], assignee[2].tokText, valueToSet)
            if err != nil {
                expr.errVal = err
                expr.evalError = true
            }
            continue
        }
        if la > 3 && assignee[1].tokType == LeftSBrace {
            rbPos := -1
            for i := la - 1; i >= 2; i-- {
                if assignee[i].tokType == RightSBrace {
                    rbPos = i
                    break
                }
            }

            if rbPos != -1 {
                // Check for a[k].f = val
                if rbPos+2 < la && assignee[rbPos+1].tokType == SYM_DOT {
                    key, err := p.Eval(rfs, assignee[2:rbPos])
                    if err != nil {
                        expr.errVal = fmt.Errorf("could not evaluate index/key: %w", err)
                        expr.evalError = true
                        return
                    }
                    fieldName := assignee[rbPos+2].tokText
                    err = handleMapOrArrayFieldAssignment(lfs, rfs, lident, assignee[0], key, fieldName, valueToSet)
                    if err != nil {
                        expr.errVal = err
                        expr.evalError = true
                    }
                } else { // a[k] = val
                    key, err := p.Eval(rfs, assignee[2:rbPos])
                    if err != nil {
                        expr.errVal = fmt.Errorf("could not evaluate index/key: %w", err)
                        expr.evalError = true
                        return
                    }

                    switch k := key.(type) {
                    case int:
                        if k < 0 {
                            expr.errVal = fmt.Errorf("negative element index: %d", k)
                            expr.evalError = true
                            return
                        }
                        if lfs == rfs {
                            vsetElement(&assignee[0], lfs, lident, assignee[0].tokText, k, valueToSet)
                        } else {
                            vsetElement(nil, lfs, lident, assignee[0].tokText, k, valueToSet)
                        }
                    case string:
                        skey := interpolate(p.namespace, rfs, rident, k)
                        if lfs == rfs {
                            vsetElement(&assignee[0], lfs, lident, assignee[0].tokText, skey, valueToSet)
                        } else {
                            vsetElement(nil, lfs, lident, assignee[0].tokText, skey, valueToSet)
                        }
                    case int64:
                        if k < 0 {
                            expr.errVal = fmt.Errorf("negative element index: %d", k)
                            expr.evalError = true
                            return
                        }
                        if lfs == rfs {
                            vsetElement(&assignee[0], lfs, lident, assignee[0].tokText, int(k), valueToSet)
                        } else {
                            vsetElement(nil, lfs, lident, assignee[0].tokText, int(k), valueToSet)
                        }
                    default:
                        skey := GetAsString(key)
                        if lfs == rfs {
                            vsetElement(&assignee[0], lfs, lident, assignee[0].tokText, skey, valueToSet)
                        } else {
                            vsetElement(nil, lfs, lident, assignee[0].tokText, skey, valueToSet)
                        }
                    }
                }
                continue
            }
        }

        // Fallback for deeply nested assignments
        if F_EnableComplexAssignments {
            chain, err := p.parseAccessChain(assignee, lfs, lident, rfs, rident)
            if err != nil {
                expr.errVal = err
                expr.evalError = true
                return
            }
            err = p.processAssignment(chain, valueToSet, lident)
            if err != nil {
                expr.errVal = err
                expr.evalError = true
                return
            }
        } else {
            expr.errVal = fmt.Errorf("deeply nested assignments are not enabled; use F_EnableComplexAssignments=true")
            expr.evalError = true
            return
        }
    }
}

// tryElementAssign attempts a fast-path assignment for slice or map elements.
// It handles auto-vivification of nil containers. Returns true on success.
func tryElementAssign(container any, key any, value any) bool {
    debug_assignment("[DEBUG] ENTER: tryElementAssign. container: %T, key: %#v, value: %#v\n", container, key, value)
    switch c := container.(type) {
    case map[string]any:
        debug_assignment("  - tryElementAssign: map[string]any case\n")
        var skey string
        switch k := key.(type) {
        case string:
            skey = k
        case int:
            skey = intToString(k)
        case uint:
            skey = strconv.FormatUint(uint64(k), 10)
        case int64:
            skey = strconv.FormatInt(k, 10)
        case uint64:
            skey = strconv.FormatUint(k, 10)
        case float64:
            skey = strconv.FormatFloat(k, 'f', -1, 64)
        case *big.Int:
            skey = k.String()
        case *big.Float:
            skey = k.String()
        default:
            skey = fmt.Sprintf("%v", k)
        }
        debug_assignment("    - skey: %s\n", skey)
        c[skey] = value
        return true
    case []any:
        debug_assignment("  - tryElementAssign: []any case\n")
        idx, ok := key.(int)
        if !ok {
            return false // Index must be an integer for a slice.
        }
        if idx < 0 {
            return false // Negative index is invalid.
        }
        debug_assignment("    - idx: %d\n", idx)

        if idx >= len(c) {
            // This path should not be taken for `[]any`, as `growSlice` in the
            // recursive path handles it. But for safety, we'll leave this check.
            return false
        }
        c[idx] = value
        return true
    }
    return false
}

// funcOf is a reflection helper that takes any slice, array, or pointer to one,
// and returns a generic accessor function. This allows the caller to treat
// them as a generic list without needing complex type switches.
func funcOf(slice any) (res struct {
    Len func() int
    Get func(i int) any
}, ok bool) {
    v := reflect.ValueOf(slice)
    for v.Kind() == reflect.Ptr || v.Kind() == reflect.Interface {
        if !v.Elem().IsValid() {
            return res, false
        }
        v = v.Elem()
    }
    if v.Kind() != reflect.Slice && v.Kind() != reflect.Array {
        return res, false
    }
    res.Len = v.Len
    res.Get = func(i int) any { return v.Index(i).Interface() }
    return res, true
}

// typedAssign performs a type-safe assignment for simple variables, mimicking the
// logic of the original `vset` function. It returns true on success.
func typedAssign(v *Variable, value any) bool {
    debug_assignment("[DEBUG] ENTER: typedAssign. var: %s, value: %#v (%T)\n", v.IName, value, value)
    var ok bool
    switch v.IKind {
    case kbool:
        debug_assignment("  - typedAssign: kbool case\n")
        if val, isType := value.(bool); isType {
            v.IValue = val
            ok = true
        }
    case kint, kint64:
        debug_assignment("  - typedAssign: kint/kint64 case\n")
        if val, isType := value.(int); isType {
            v.IValue = val
            ok = true
        }
    case kuint, kuint64:
        debug_assignment("  - typedAssign: kuint/kuint64 case\n")
        if val, isType := value.(uint); isType {
            v.IValue = val
            ok = true
        }
    case kfloat:
        debug_assignment("  - typedAssign: kfloat case\n")
        if val, isType := value.(float64); isType {
            v.IValue = val
            ok = true
        }
    case kstring:
        debug_assignment("  - typedAssign: kstring case\n")
        if val, isType := value.(string); isType {
            v.IValue = val
            ok = true
        }
    case kbyte:
        debug_assignment("  - typedAssign: kbyte case\n")
        if val, isType := value.(uint8); isType {
            v.IValue = val
            ok = true
        }
    case kbigi:
        debug_assignment("  - typedAssign: kbigi case\n")
        if val, isType := value.(*big.Int); isType {
            v.IValue.(*big.Int).Set(val)
            ok = true
        }
    case kbigf:
        debug_assignment("  - typedAssign: kbigf case\n")
        if val, isType := value.(*big.Float); isType {
            v.IValue.(*big.Float).Set(val)
            ok = true
        }
    case ksbool:
        debug_assignment("  - typedAssign: ksbool case\n")
        if val, isType := value.([]bool); isType {
            v.IValue = val
            ok = true
        }
    case ksint:
        debug_assignment("  - typedAssign: ksint case\n")
        if val, isType := value.([]int); isType {
            v.IValue = val
            ok = true
        }
    case ksuint:
        debug_assignment("  - typedAssign: ksuint case\n")
        if val, isType := value.([]uint); isType {
            v.IValue = val
            ok = true
        }
    case ksfloat:
        debug_assignment("  - typedAssign: ksfloat case\n")
        if val, isType := value.([]float64); isType {
            v.IValue = val
            ok = true
        }
    case ksstring:
        debug_assignment("  - typedAssign: ksstring case\n")
        if val, isType := value.([]string); isType {
            v.IValue = val
            ok = true
        }
    case ksbyte:
        debug_assignment("  - typedAssign: ksbyte case\n")
        if val, isType := value.([]uint8); isType {
            v.IValue = val
            ok = true
        }
    case ksbigi:
        debug_assignment("  - typedAssign: ksbigi case\n")
        if val, isType := value.([]*big.Int); isType {
            v.IValue = val
            ok = true
        }
    case ksbigf:
        debug_assignment("  - typedAssign: ksbigf case\n")
        if val, isType := value.([]*big.Float); isType {
            v.IValue = val
            ok = true
        }
    case ksany:
        debug_assignment("  - typedAssign: ksany case\n")
        if val, isType := value.([]any); isType {
            v.IValue = val
            ok = true
        }
    case kmap:
        debug_assignment("  - typedAssign: kmap case\n")
        if val, isType := value.(map[string]any); isType {
            v.IValue = val
            ok = true
        }
    case knil, kany:
        debug_assignment("  - typedAssign: knil/kany case\n")
        v.IValue = value
        ok = true
    }
    return ok
}

// processAssignment is the main refactored assignment function.
func (p *leparser) processAssignment(chain Chain, valueToSet any, lident *[]Variable) error {
    // Ensure the root variable exists.
    if chain.BindPos >= uint64(len(*lident)) {
        newident := make([]Variable, chain.BindPos+identGrowthSize)
        copy(newident, *lident)
        *lident = newident
    }

    rootVar := &(*lident)[chain.BindPos]
    debug_assignment("[DEBUG] ENTER: processAssignment. valueToSet: %#v, rootVar.IValue: %#v\n", valueToSet, rootVar.IValue)
    if !rootVar.declared {
        rootVar.IName = chain.Name
        rootVar.declared = true

        // struct type inference, as per the original working implementation.
        if valueToSet != nil {
            if reflect.TypeOf(valueToSet).Kind() == reflect.Struct {
                if structName, count := struct_match(valueToSet); count == 1 {
                    rootVar.Kind_override = structName
                }
            }
        }

        // Auto-vivify the root container based on the first access
        if len(chain.Accesses) > 1 {
            switch chain.Accesses[1].Type {
            case AccessMap:
                rootVar.IValue = make(map[string]any)
            case AccessArray:
                rootVar.IValue = make([]any, 0)
            }
        }
    }

    // It returns the (potentially new) value of the container it modified.
    var finalVal reflect.Value
    var err error

    debug_assignment("[DEBUG] processAssignment: Calling recursiveAssign\n")
    // Call recursive assign, skipping the first access (which is just the variable itself)
    finalVal, err = p.recursiveAssign(reflect.ValueOf(rootVar.IValue), chain.Accesses[1:], valueToSet)
    if err != nil {
        debug_assignment("[DEBUG] processAssignment: recursiveAssign returned err\n")
        return err
    }
    debug_assignment("[DEBUG] processAssignment: recursiveAssign returned. finalVal: %#v\n", finalVal)

    if finalVal.IsValid() {
        rootVar.IValue = finalVal.Interface()
    } else {
        rootVar.IValue = nil
    }

    return nil
}

func (p *leparser) recursiveAssign(currentVal reflect.Value, accesses []Access, valueToSet any) (reflect.Value, error) {
    debug_assignment("[DEBUG] ENTER: recursiveAssign. accesses_len: %d, valueToSet: %#v (%T)\n", len(accesses), valueToSet, valueToSet)
    if currentVal.IsValid() {
        debug_assignment("  - currentVal: Type: %v, Kind: %v, CanAddr: %v, CanSet: %v\n", currentVal.Type(), currentVal.Kind(), currentVal.CanAddr(), currentVal.CanSet())
    } else {
        debug_assignment("  - currentVal: Invalid\n")
    }
    // Base case: If there are no more access steps, we have the final container.
    // We just need to convert the value we're setting and return it.
    if len(accesses) == 0 {
        debug_assignment("[DEBUG] recursiveAssign: Base case\n")
        var targetType reflect.Type
        if currentVal.IsValid() {
            targetType = currentVal.Type()
        }
        convertedValue, err := convertAssignmentValue(targetType, valueToSet)
        if err != nil {
            return reflect.Value{}, err
        }
        return reflect.ValueOf(convertedValue), nil
    }

    access := accesses[0] // The CURRENT access to process
    remainingAccesses := accesses[1:]

    // If the current value is an interface, recurse into the value it contains.
    // This is the key to preventing memory corruption from temporary copies.
    if currentVal.IsValid() && currentVal.Kind() == reflect.Interface {
        debug_assignment("[DEBUG] recursiveAssign: Recursing into interface\n")
        // If the interface is nil, we can't recurse. The next step (e.g., AccessMap)
        // will handle auto-vivification of a new container.
        if currentVal.IsNil() {
            debug_assignment("[DEBUG] recursiveAssign: Interface is nil, proceeding to auto-vivify\n")
        } else {
            return p.recursiveAssign(currentVal.Elem(), accesses, valueToSet)
        }
    }

    debug_assignment("about to switch on access.Type of %+v\n", access.Type)
    switch access.Type {
    case AccessArray:
        debug_assignment("[DEBUG] recursiveAssign: AccessArray case\n")
        index := access.Key.(int)

        // Grow slice if necessary. This might create a new slice value.
        grownSlice, err := growSlice(currentVal, index, valueToSet)
        if err != nil {
            return reflect.Value{}, err
        }
        currentVal = grownSlice

        // Recursively call on the element.
        elem := currentVal.Index(index)
        modifiedElem, err := p.recursiveAssign(elem, remainingAccesses, valueToSet)
        if err != nil {
            return reflect.Value{}, err
        }

        // To modify a slice, we must create a new copy, set the element in the
        // copy, and then return the new slice. Direct in-place modification
        // of slice elements via reflection is not possible in this context.
        // This was the source of the memory corruption.
        newSlice := reflect.MakeSlice(currentVal.Type(), currentVal.Len(), currentVal.Cap())
        reflect.Copy(newSlice, currentVal)
        // We must use unsafeSet here, not Set, because the slice may contain
        // dynamically created structs with unexported fields.
        unsafeSet(newSlice.Index(index), modifiedElem)
        return newSlice, nil

    case AccessMap:
        debug_assignment("[DEBUG] recursiveAssign: AccessMap case\n")
        // Consistent with AccessArray/AccessField, if the map is not addressable
        // (e.g., from a slice/interface), we must make a mutable copy.
        if !currentVal.CanAddr() {
            currentVal = deepCopyValue(currentVal)
        }
        if !currentVal.IsValid() || (currentVal.Kind() != reflect.Map) {
            // Auto-vivify map as string-keyed, as it's the only supported user-space map.
            currentVal = reflect.ValueOf(make(map[string]any))
        }

        // All non-string keys are converted to strings.
        var skey string
        switch k := access.Key.(type) {
        case string:
            skey = k
        case int:
            skey = intToString(k)
        case uint:
            skey = strconv.FormatUint(uint64(k), 10)
        case int64:
            skey = strconv.FormatInt(k, 10)
        case uint64:
            skey = strconv.FormatUint(k, 10)
        case float64:
            skey = strconv.FormatFloat(k, 'f', -1, 64)
        case *big.Int:
            skey = k.String()
        case *big.Float:
            skey = k.String()
        default:
            skey = fmt.Sprintf("%v", k)
        }
        debug_assignment("[DEBUG] recursiveAssign: AccessMap case. skey: %s\n", skey)
        rkey := reflect.ValueOf(skey)

        // Recursively call on the element.
        elem := currentVal.MapIndex(rkey)
        debug_assignment("[DEBUG] recursiveAssign: AccessMap recursing\n")
        if elem.IsValid() {
            debug_assignment("  - elem: Type: %v, Kind: %v, CanAddr: %v, CanSet: %v\n", elem.Type(), elem.Kind(), elem.CanAddr(), elem.CanSet())
        } else {
            debug_assignment("  - elem: Invalid\n")
        }

        // After retrieving an element, if it's an interface, we must unwrap it
        // before passing it to the next recursive step.
        if elem.IsValid() && elem.Kind() == reflect.Interface {
            elem = elem.Elem()
        }

        modifiedElem, err := p.recursiveAssign(elem, remainingAccesses, valueToSet)
        if err != nil {
            return reflect.Value{}, err
        }

        // Set the (potentially modified) element back into the map.
        currentVal.SetMapIndex(rkey, modifiedElem)
        return currentVal, nil

    case AccessField:
        debug_assignment("[DEBUG] recursiveAssign: AccessField case\n")
        if !currentVal.IsValid() {
            return reflect.Value{}, fmt.Errorf("cannot access field on nil value")
        }

        // If the struct is not addressable (e.g., from `[]any`), we MUST make a copy
        // to remove the taint before we can access its fields for modification.
        if !currentVal.CanAddr() {
            debug_assignment("[DEBUG] recursiveAssign: AccessField unaddressable struct\n")
            currentVal = deepCopyValue(currentVal)
        }

        field := currentVal.FieldByName(access.Field)
        if !field.IsValid() {
            return reflect.Value{}, fmt.Errorf("field '%s' not found in struct %v", access.Field, currentVal.Type())
        }
        debug_assignment("[DEBUG] recursiveAssign: AccessField recursing. field: %s\n", access.Field)
        if field.IsValid() {
            debug_assignment("  - field: Type: %v, Kind: %v, CanAddr: %v, CanSet: %v\n", field.Type(), field.Kind(), field.CanAddr(), field.CanSet())
        } else {
            debug_assignment("  - field: Invalid\n")
        }
        // Recursively call on the field.
        modifiedField, err := p.recursiveAssign(field, remainingAccesses, valueToSet)
        if err != nil {
            return reflect.Value{}, err
        }

        // BOXING LOGIC: If target is interface, ensure value is boxed.
        finalField := modifiedField
        if field.Type().Kind() == reflect.Interface && modifiedField.IsValid() && modifiedField.Type() != field.Type() {
            if modifiedField.Type().AssignableTo(field.Type()) {
                boxedVal := reflect.New(field.Type()).Elem()
                boxedVal.Set(modifiedField)
                finalField = boxedVal
            } else {
                return reflect.Value{}, fmt.Errorf("type mismatch: cannot assign %v to interface type %v", modifiedField.Type(), field.Type())
            }
        }

        // Set the field on our (now addressable) struct.
        unsafeSet(field, finalField)

        return currentVal, nil
    }

    return reflect.Value{}, fmt.Errorf("unsupported access type in chain")
}

func (p *leparser) parseAccessChain(tokens []Token, lfs uint32, lident *[]Variable, rfs uint32, rident *[]Variable) (Chain, error) {
    debug_assignment("[DEBUG] ENTER: parseAccessChain\n")
    if len(tokens) == 0 || tokens[0].tokType != Identifier {
        return Chain{}, fmt.Errorf("invalid assignee: must start with an identifier")
    }

    var bindpos uint64
    if lfs != rfs {
        bindpos = bind_int(lfs, tokens[0].tokText)
    } else {
        bindpos = tokens[0].bindpos
    }

    chain := Chain{
        Accesses: make([]Access, 0, (len(tokens)+1)/2),
        Name:     tokens[0].tokText,
        BindPos:  bindpos,
    }

    // First access is always a variable
    chain.Accesses = append(chain.Accesses, Access{
        Type:  AccessVariable,
        Field: tokens[0].tokText,
    })

    // Process tokens to build access chain
    i := 1
    for i < len(tokens) {
        debug_assignment("  - parseAccessChain: loop, token: %v\n", tokens[i])
        switch tokens[i].tokType {
        case LeftSBrace:
            // Handle array/map access with [] notation
            // Find the matching right bracket
            rbPos := -1
            // This is a complex path and brace counting is required here.
            // The user's instruction was specific to doAssign, where the end is known.
            // Here, we must search.
            braceCount := 1
            for j := i + 1; j < len(tokens); j++ {
                if tokens[j].tokType == LeftSBrace {
                    braceCount++
                } else if tokens[j].tokType == RightSBrace {
                    braceCount--
                    if braceCount == 0 {
                        rbPos = j
                        break
                    }
                }
            }
            if rbPos == -1 {
                return Chain{}, fmt.Errorf("missing closing bracket")
            }

            // Evaluate the index/key expression
            key, err := p.Eval(rfs, tokens[i+1:rbPos])
            if err != nil {
                return Chain{}, fmt.Errorf("invalid index/key expression: %v", err)
            }
            debug_assignment("    - parsed key: %#v\n", key)

            var accessType AccessType
            if idx, ok := key.(int); ok {
                accessType = AccessArray
                key = idx
            } else {
                accessType = AccessMap
                key = fmt.Sprintf("%v", key)
            }

            // Add array/map access to chain
            chain.Accesses = append(chain.Accesses, Access{
                Type: accessType,
                Key:  key,
            })
            debug_assignment("    - appended access: type %d, key %#v\n", accessType, key)

            i = rbPos + 1

        case SYM_DOT:
            i++
            if i >= len(tokens) || tokens[i].tokType != Identifier {
                return Chain{}, fmt.Errorf("invalid field access: unexpected token after dot")
            }

            field := tokens[i].tokText
            debug_assignment("    - parsed field: %s\n", field)

            // Add struct access to chain
            chain.Accesses = append(chain.Accesses, Access{
                Type:  AccessField,
                Field: field,
            })
            debug_assignment("    - appended access: type AccessField, field %s\n", field)
            i++

        default:
            return Chain{}, fmt.Errorf("unexpected token type in assignment chain: %v", tokens[i].tokText)
        }
    }

    debug_assignment("  - parseAccessChain: completed, returning chain: %+v\n", chain)
    return chain, nil
}

func (p *leparser) validateChain(chain Chain, value any) error {
    // Get initial variable
    if len(chain.Accesses) == 0 {
        return nil
    }

    // Walk the chain and validate each access
    for i, access := range chain.Accesses {
        isLast := i == len(chain.Accesses)-1

        switch access.Type {
        case AccessMap:
        // Key validation is handled during access.

        case AccessArray:
            // Validate array indices and bounds
            idx, ok := access.Key.(int)
            if !ok {
                return fmt.Errorf("array index must be integer")
            }
            if idx < 0 {
                return fmt.Errorf("array index out of bounds: %d", idx)
            }

            // For the last access, validate against RHS value
            if isLast {
                // If RHS is slice/array, validate size matches
                rhsVal := reflect.ValueOf(value)
                if rhsVal.Kind() == reflect.Array || rhsVal.Kind() == reflect.Slice {
                    if idx >= rhsVal.Len() {
                        return fmt.Errorf("array index out of bounds: %d (RHS size: %d)", idx, rhsVal.Len())
                    }
                }
            }

        case AccessField:
            if access.Field == "" {
                return fmt.Errorf("struct field name cannot be empty")
            }

        }
    }

    return nil
}

// boxValueIfNecessary handles the case where a concrete value (like an int) needs
// to be placed into a container that holds interfaces (like []any or a struct field
// of type any). It "boxes" the concrete value into an interface value.
func boxValueIfNecessary(targetType reflect.Type, valueToSet reflect.Value) (reflect.Value, error) {
    if !valueToSet.IsValid() {
        // If setting to nil/invalid, just return it. The caller handles creating a zero value.
        return valueToSet, nil
    }
    // If types already match, we're good.
    if targetType == valueToSet.Type() {
        return valueToSet, nil
    }

    // This handles boxing a concrete value (int, string, etc.) into an interface
    // when the target container expects an interface.
    if targetType.Kind() == reflect.Interface && valueToSet.Type().AssignableTo(targetType) {
        // Create a new reflect.Value of the target interface type.
        boxedVal := reflect.New(targetType).Elem()
        // Set the interface to hold the concrete value.
        boxedVal.Set(valueToSet)
        return boxedVal, nil
    }

    // Return a clear error for unhandled mismatches.
    return reflect.Value{}, fmt.Errorf(
        "internal error: type mismatch during recursive assignment. container expected %v, but received %v",
        targetType,
        valueToSet.Type(),
    )
}

// deepCopyValue creates a clean, mutable copy of a potentially tainted, unaddressable reflect.Value.
func deepCopyValue(v reflect.Value) reflect.Value {
    if !v.IsValid() {
        return v
    }
    // Create a new, addressable value of the same type to be our clean destination.
    dest := reflect.New(v.Type()).Elem()
    // Copy from the (potentially tainted) source `v` to the clean destination `dest`.
    copyHelper(dest, v)
    return dest
}

// copyHelper recursively copies from a source value to a settable destination value.
// It uses `unsafeSet` to bypass taint checks on all field types.
func copyHelper(dest, src reflect.Value) {
    if !src.IsValid() {
        return
    }

    switch src.Kind() {
    case reflect.Struct:
        for i := 0; i < src.NumField(); i++ {
            copyHelper(dest.Field(i), src.Field(i))
        }
    case reflect.Slice:
        if !src.IsNil() {
            // Create a new clean slice and recursively copy elements into it.
            newSlice := reflect.MakeSlice(src.Type(), src.Len(), src.Cap())
            for i := 0; i < src.Len(); i++ {
                copyHelper(newSlice.Index(i), src.Index(i))
            }
            dest.Set(newSlice)
        }

    case reflect.Map:
        if !src.IsNil() {
            newMap := reflect.MakeMap(src.Type())
            for _, key := range src.MapKeys() {
                newMap.SetMapIndex(deepCopyValue(key), deepCopyValue(src.MapIndex(key)))
            }
            dest.Set(newMap)
        }
    case reflect.Interface:
        if !src.IsNil() {
            // Create a clean copy of the value within the interface.
            copiedElem := deepCopyValue(src.Elem())
            // When the destination is an interface, we must use the standard Set operation.
            // This is safe because copiedElem is a new, clean value. Using unsafeSet
            // here would corrupt the interface's internal pointers.
            dest.Set(copiedElem)
        }
    case reflect.Ptr:
        if !src.IsNil() {
            // Create a new pointer of the same type as the original.
            newPtr := reflect.New(src.Type().Elem())
            // Recursively copy the value from the original pointer's target
            // to the new pointer's target.
            copyHelper(newPtr.Elem(), src.Elem())
            // Set the destination value to be the new pointer.
            dest.Set(newPtr)
        }
    default:
        // This handles primitive types (int, string, bool, etc.).
        unsafeSet(dest, src)
    }
}
