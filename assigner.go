package main

import (
    "fmt"
    "math"
    "math/big"
    "reflect"
    "strconv"
    "unsafe"
)


// intStrCache provides a cache for small integer to string conversions to reduce allocations.
var intStrCache [1024]string

// intToString is a faster version of strconv.Itoa for small, non-negative integers.
func intToString(i int) string {
    if i >= 0 && i < len(intStrCache) {
        return intStrCache[i]
    }
    return strconv.Itoa(i)
}

func unsafeSet(dest reflect.Value, obj any) {
    if !dest.CanAddr() {
        panic("unsafeSet called on non-addressable destination")
    }
    r := reflect.ValueOf(obj)
    dest = reflect.New(r.Type()).Elem()
    dest.Set(r)
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
    Typemap["mixed"] = reflect.TypeOf((*any)(nil)).Elem()
    Typemap["any"] = reflect.TypeOf((*any)(nil)).Elem()
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

    // If the incoming value is invalid (e.g., from a nil interface), it means
    // we are performing indexed access on something that doesn't exist yet.
    // We must "auto-vivify" a new slice to hold the value. The type of the
    // new slice is inferred from the value being assigned.
    if !slice.IsValid() {
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

    // If we get an interface, it's likely from a container like []any. If it's
    // nil, we can safely replace it with a new slice (auto-vivification).
    if slice.Kind() == reflect.Interface {
        if slice.IsNil() {
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
            newSlice := reflect.MakeSlice(reflect.TypeOf([]any{}), index+1, index+1)
            return newSlice, nil
        }
        return slice, fmt.Errorf("cannot perform array access on non-slice type: %v", slice.Type())
    }

    if index >= slice.Len() {
        newSize := index + 1
        newCap := slice.Cap()
        if newSize > newCap {
            newCap = newSize
            if newCap < 2*slice.Cap() {
                newCap = 2 * slice.Cap()
            }
        }
        newSlice := reflect.MakeSlice(slice.Type(), newSize, newCap)
        for i := 0; i < slice.Len(); i++ {
            newSlice.Index(i).Set(reflect.ValueOf(recCopy(slice.Index(i).Interface())))
        }
        return newSlice, nil
    }
    return slice, nil
}

// convertAssignmentValue handles type conversions and checking for assignments.
func convertAssignmentValue(targetType reflect.Type, value any) (any, error) {

    // If targetType is nil, it signifies assignment to a new key in a map or a new
    // element in a slice that is being auto-vivified. In this case, there's no
    // existing type to convert to, so we accept the new value as-is.
    if targetType == nil {
        return value, nil
    }

    // Handle nil value - always allowed for auto-vivified containers
    if value == nil {
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
        if valueType != nil && valueType.AssignableTo(targetType) {
            return value, nil
        }
        // Allow any value to be assigned to an empty interface (`any` or `any`)
        if targetType.NumMethod() == 0 {
            return value, nil
        }
        return nil, fmt.Errorf("type %v does not implement interface %v", valueType, targetType)
    }

    // Handle array/slice element assignment
    if targetKind == reflect.Slice || targetKind == reflect.Array {
        valueKind := valueReflect.Kind()
        // pf("[cav] slice assignment to %v with %+v\n",targetKind,value)

        // Case 1: Assigning a whole slice/array to another slice/array
        if valueKind == reflect.Slice || valueKind == reflect.Array {
            sourceType := valueReflect.Type()
            targetElemType := targetType.Elem()
            sourceElemType := sourceType.Elem()

            // Fast path: if element types are directly assignable, just return the original value.
            if sourceElemType.AssignableTo(targetElemType) {
                return value, nil
            }

            // Slow path: element-by-element conversion is needed.
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
        if valueType != nil {
            valueKind := valueType.Kind()
            // For whole map assignment
            if valueKind == reflect.Map {
                // For untyped, allow if key/value types are compatible
                if valueType.Key().AssignableTo(targetType.Key()) &&
                    valueType.Elem().AssignableTo(targetType.Elem()) {
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
        return value, nil
    }

    // 2. Try conversion for basic types only
    if valueType != nil && valueType.ConvertibleTo(targetType) {
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
            return reflect.ValueOf(value).Convert(targetType).Interface(), nil
        }
    }

    // FINAL FALLBACK: If no other conversion rule matches, return the original value.
    // The subsequent assignment logic (e.g., reflect.Set) will be the final arbiter
    // of whether the assignment is valid. This allows for more dynamic assignments.
    return value, nil
}

/*
handleFieldAssignment is a helper function to handle direct field assignments
on a struct variable (e.g., `variable.field = value`). It fetches the struct,
creates a mutable copy, sets the field on the copy, and writes the modified
struct back.
*/
func handleFieldAssignment(lfs, rfs uint32, lident *[]Variable, varToken Token, fieldName string, value any) error {

    // ts is target struct
    ts, found := vget(&varToken, lfs, lident, varToken.tokText)
    if !found {
        return fmt.Errorf("record variable %v not found", varToken.tokText)
    }

    // reflection of target struct value:
    val := reflect.ValueOf(ts)

    if val.Kind() == reflect.Map {
        vsetElement(nil, lfs, lident, varToken.tokText, fieldName, value)
        // pf("setting map element %s\n",fieldName)
        return nil
    }

    if val.Kind() != reflect.Struct {
        return fmt.Errorf("variable %v is not a struct", varToken.tokText)
    }

    // enforce casing
    fieldName=renameSF(fieldName)

    // make a new temporary target struct type to work with and populate it
    tmp := reflect.New(val.Type()).Elem()
    tmp.Set(val)

    // get a ref to the required field:
    field := tmp.FieldByName(fieldName)
    disableRO(&field)

    if !field.IsValid() {
        return fmt.Errorf("field %v not found in struct %v", fieldName, varToken.tokText)
    }

    // if the field is not public then change ref to a new instance of the field
    // this may not be right, it's an attempt to remove the taint (that clearly is not working!)

    if !field.CanSet() {
        field = reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem()
    }

    // populate field value from required value in fn arguments
    if value == nil {
        field.Set(reflect.Zero(field.Type()))
    } else {
        valToSet := reflect.ValueOf(value)
        if valToSet.Type().AssignableTo(field.Type()) {
            field.Set(valToSet)
            // pf("latest dest field value: %+v\n",field.Interface())
            // pf("latest dest tmp value  : %+v\n",tmp.Interface())
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

    // uppercase initial
    fieldName=renameSF(fieldName)

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
    disableRO(&field)

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
        switch value.(type) {
        case string:
            value = interpolate(p.namespace, rfs, rident, value.(string))
        }

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
        if la == 4 && assignee[1].tokType == LeftSBrace && assignee[3].tokType == RightSBrace {
            // Fast path for simple var[key] = value assignments
            key, err := p.Eval(rfs, assignee[2:3])
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
            return
        }
        if la > 3 && assignee[1].tokType == LeftSBrace {
            // Find the MATCHING right bracket for the opening '[' at position 1
            bracketDepth := 1
            matchingRbPos := -1
            for i := 2; i < la && bracketDepth > 0; i++ {
                if assignee[i].tokType == LeftSBrace {
                    bracketDepth++
                } else if assignee[i].tokType == RightSBrace {
                    bracketDepth--
                    if bracketDepth == 0 {
                        matchingRbPos = i
                        break
                    }
                }
            }

            // Check for additional '[' tokens AFTER the matching closing ']' to detect multi-dimensional access
            hasSecondBracket := false
            if matchingRbPos != -1 && matchingRbPos+1 < la {
                for i := matchingRbPos + 1; i < la; i++ {
                    if assignee[i].tokType == LeftSBrace {
                        hasSecondBracket = true
                        break
                    }
                }
            }

            // If we have multiple bracket pairs, use complex assignment path
            if hasSecondBracket {
                if F_EnableComplexAssignments {
                    chain, err := p.parseAccessChain(assignee, lfs, lident, rfs, rident)
                    if err != nil {
                        expr.errVal = err
                        expr.evalError = true
                        return
                    }
                    err = p.processAssignment(chain, value, lident)
                    if err != nil {
                        expr.errVal = err
                        expr.evalError = true
                    }
                    return
                } else {
                    expr.errVal = fmt.Errorf("deeply nested assignments are not enabled; use F_EnableComplexAssignments=true")
                    expr.evalError = true
                    return
                }
            }

            // For single-dimensional assignment, find the LAST right bracket
            rbPos := -1
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
                return
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
            // Find the MATCHING right bracket for the opening '[' at position 1
            bracketDepth := 1
            matchingRbPos := -1
            for i := 2; i < la && bracketDepth > 0; i++ {
                if assignee[i].tokType == LeftSBrace {
                    bracketDepth++
                } else if assignee[i].tokType == RightSBrace {
                    bracketDepth--
                    if bracketDepth == 0 {
                        matchingRbPos = i
                        break
                    }
                }
            }

            // Check for additional '[' tokens AFTER the matching closing ']' to detect multi-dimensional access
            hasSecondBracket := false
            if matchingRbPos != -1 && matchingRbPos+1 < la {
                for i := matchingRbPos + 1; i < la; i++ {
                    if assignee[i].tokType == LeftSBrace {
                        hasSecondBracket = true
                        break
                    }
                }
            }

            // If we have multiple bracket pairs, use complex assignment path
            if hasSecondBracket {
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
                    continue
                } else {
                    expr.errVal = fmt.Errorf("deeply nested assignments are not enabled; use F_EnableComplexAssignments=true")
                    expr.evalError = true
                    return
                }
            }

            // For single-dimensional assignment, find the LAST right bracket
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

// processAssignment is the main refactored assignment function.
func (p *leparser) processAssignment(chain Chain, valueToSet any, lident *[]Variable) error {
    // Ensure the root variable exists.
    if chain.BindPos >= uint64(len(*lident)) {
        newident := make([]Variable, chain.BindPos+identGrowthSize)
        copy(newident, *lident)
        *lident = newident
    }

    rootVar := &(*lident)[chain.BindPos]
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

    // Call recursive assign, skipping the first access (which is just the variable itself)
    finalVal, err = p.recursiveAssign(reflect.ValueOf(rootVar.IValue), chain.Accesses[1:], valueToSet)
    // pf("(pa) final val -> %#v\n",finalVal)
    if err != nil {
        return err
    }

    if finalVal.IsValid() {
        rootVar.IValue = finalVal.Interface()
    } else {
        rootVar.IValue = nil
    }

    return nil
}

func (p *leparser) recursiveAssign(currentVal reflect.Value, accesses []Access, valueToSet any) (reflect.Value, error) {

    // pf("inside RA with acs -> %+v\n",accesses)
    // pf("inside RA with cv  -> [#6]%#v[#-]\n",currentVal)
    // pf("inside RA with val -> %+v\n",valueToSet)

    // Base case: If there are no more access steps, we have the final container.
    // We just need to convert the value we're setting and return it.
    if len(accesses) == 0 {
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
    // pf("Current access -> %+v\n",access)

    // If the current value is an interface, recurse into the value it contains.
    if currentVal.IsValid() && currentVal.Kind() == reflect.Interface {
        // If the interface is nil, we can't recurse. The next step (e.g., AccessMap)
        // will handle auto-vivification of a new container.
        if ! currentVal.IsNil() {
            return p.recursiveAssign(currentVal.Elem(), accesses, valueToSet)
        }
    }

    switch access.Type {
    case AccessArray:
        index := access.Key.(int)

        // Grow slice if necessary. This might create a new slice value.
        // pf("(aa) slice size %v\n",currentVal.Len())
        grownSlice, err := growSlice(currentVal, index, valueToSet)
        if err != nil {
            return reflect.Value{}, err
        }
        // pf("(aa) new size %v\n",grownSlice.Len())
        currentVal = grownSlice

        // Recursively call on the element.
        elem := currentVal.Index(index)
        // pf("(aa) elem : %+v\n",elem)
        modifiedElem, err := p.recursiveAssign(elem, remainingAccesses, valueToSet)
        // pf("(aa) mod elem : %+v\n",modifiedElem)
        if err != nil {
            return reflect.Value{}, err
        }

		// copy slice
        newSlice := reflect.MakeSlice(currentVal.Type(), currentVal.Len(), currentVal.Cap())
        // newSlice.Set(currentVal)
        for i := 0; i < currentVal.Len(); i++ {
            newSlice.Index(i).Set(reflect.ValueOf(recCopy(currentVal.Index(i).Interface())))
        }

		// update slice with new elem
        newSlice.Index(index).Set(modifiedElem)
        // pf("(aa) new slice -> %#v\n",newSlice)
        return newSlice, nil

    case AccessMap:

        var newContainer reflect.Value

        // pf("cvk -> %+v\n",currentVal.Kind())
        // pf("(am) accessing %v | ",access.Key)
        // pf("(am) access list: %#v\n",accesses)

        var isMap bool

        if currentVal.Kind() == reflect.Map {
            isMap=true
        }

        if currentVal.Kind() == reflect.Invalid {
            newContainer=reflect.MakeMap(reflect.TypeOf(make(map[string]any)))
            isMap=true
        } else {
            newContainer=currentVal
        }

        // copy from below
        // All non-string keys are converted to strings.
        var skey string
        var ikey int
        switch k := access.Key.(type) {
        case string:
            skey = k
        case int:
            skey = intToString(k)
            ikey = k
        case uint:
            skey = strconv.FormatUint(uint64(k), 10)
            ikey = int(k)
        case int64:
            skey = strconv.FormatInt(k, 10)
            ikey = int(k)
        case uint64:
            skey = strconv.FormatUint(k, 10)
            ikey = int(k)
        case float64:
            skey = strconv.FormatFloat(k, 'f', -1, 64)
        case *big.Int:
            skey = k.String()
        case *big.Float:
            skey = k.String()
        default:
            skey = fmt.Sprintf("%v", k)
        }
        rkey := reflect.ValueOf(skey)

        // Recursively call on the element.
        var elem reflect.Value
        if isMap {
            elem = newContainer.MapIndex(rkey)
        } else {
            // Grow slice if necessary. This might create a new slice value.
            grownSlice, err := growSlice(newContainer, ikey, valueToSet)
            if err != nil {
                return reflect.Value{}, err
            }
            newContainer = grownSlice
            elem = newContainer.Index(ikey)
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
        if isMap {
            newContainer.SetMapIndex(rkey, modifiedElem)
            return newContainer, nil
        }

        newContainer.Index(ikey).Set(modifiedElem)
        return newContainer, nil

    case AccessField:
        // enforce field casing
        access.Field=renameSF(access.Field)
        if !currentVal.IsValid() {
            return reflect.Value{}, fmt.Errorf("cannot access field on nil value")
        }

        /*  
        // If the struct is not addressable (e.g., from `[]any`), we MUST make a copy
        if !currentVal.CanAddr() {
            currentVal = reflect.ValueOf(recCopy(currentVal.Interface()))
        }
        */
        currentVal = reflect.ValueOf(recCopy(currentVal.Interface()))

        var field reflect.Value
        // pf("cv entry -> %#v\n",currentVal)

    	tmp := reflect.New(currentVal.Type()).Elem()
    	tmp.Set(currentVal)

		field=tmp.FieldByName(access.Field)
        disableRO(&field)

        if !field.IsValid() {
            return reflect.Value{}, fmt.Errorf("field '%s' not found in struct %v", access.Field, currentVal.Type())
        }

        // Recursively call on the field.
        finalField, err := p.recursiveAssign(field, remainingAccesses, valueToSet)
        if err != nil {
            return reflect.Value{}, err
        }
        disableRO(&finalField)

        // Set the field on our (now addressable) struct.
		field.Set(finalField)
		currentVal=tmp

        // pf("field is -> %#v\n",field)
        // pf("cv is    -> %#v\n",currentVal)
        return currentVal, nil
    }

    return reflect.Value{}, fmt.Errorf("unsupported access type in chain")
}

func (p *leparser) parseAccessChain(tokens []Token, lfs uint32, lident *[]Variable, rfs uint32, rident *[]Variable) (Chain, error) {
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
        switch tokens[i].tokType {
        case LeftSBrace:
            // Handle array/map access with [] notation
            // Find the matching right bracket
            rbPos := -1
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

            i = rbPos + 1

        case SYM_DOT:
            i++
            if i >= len(tokens) || tokens[i].tokType != Identifier {
                return Chain{}, fmt.Errorf("invalid field access: unexpected token after dot")
            }

            field := renameSF(tokens[i].tokText)

            // Add struct access to chain
            chain.Accesses = append(chain.Accesses, Access{
                Type:  AccessField,
                Field: field,
            })
            i++

        default:
            return Chain{}, fmt.Errorf("unexpected token type in assignment chain: %v", tokens[i].tokText)
        }
    }

    return chain, nil
}


func recCopy(original any) (copy any) {
    if original == nil {
        return nil
    }
    value := reflect.ValueOf(original)
    return rcopy(value).Interface()
}

func rcopy(original reflect.Value) reflect.Value {
    switch original.Kind() {
    case reflect.Slice:
        return rcopySlice(original)
    case reflect.Map:
        return rcopyMap(original)
    case reflect.Ptr:
        return rcopyPointer(original)
    case reflect.Struct:
        return rcopyStruct(original)
    case reflect.Chan:
        return rcopyChan(original)
    case reflect.Array:
        return rcopyArray(original)
    default:
        return forceCopyValue(original)
    }
}

func forceCopyValue(original reflect.Value) reflect.Value {
    originalType := original.Type()
    newPointer := reflect.New(originalType)
    newPointer.Elem().Set(original)
    return newPointer.Elem()
}

func rcopySlice(original reflect.Value) reflect.Value {
    if original.IsNil() {
        return original
    }
    copy := reflect.MakeSlice(original.Type(), 0, 0)
    for i := 0; i < original.Len(); i++ {
        elementCopy := rcopy(original.Index(i))
        copy = reflect.Append(copy, elementCopy)
    }
    return copy
}

func rcopyArray(original reflect.Value) reflect.Value {
    if original.Len() == 0 {
        return original
    }
    elementType := original.Index(0).Type()
    arrayType := reflect.ArrayOf(original.Len(), elementType)
    newPointer := reflect.New(arrayType)
    copy := newPointer.Elem()
    for i := 0; i < original.Len(); i++ {
        subCopy := rcopy(original.Index(i))
        copy.Index(i).Set(subCopy)
    }
    return copy
}

func rcopyMap(original reflect.Value) reflect.Value {
    if original.IsNil() {
        return original
    }
    keyType := original.Type().Key()
    valueType := original.Type().Elem()
    mapType := reflect.MapOf(keyType, valueType)
    copy := reflect.MakeMap(mapType)
    for _, key := range original.MapKeys() {
        value := rcopy(original.MapIndex(key))
        copy.SetMapIndex(key, value)
    }
    return copy
}

func rcopyPointer(original reflect.Value) reflect.Value {
    if original.IsNil() {
        return original
    }
    element := original.Elem()
    copy := reflect.New(element.Type())
    copyElement := rcopy(element)
    copy.Elem().Set(copyElement)
    return copy
}

func rcopyStruct(original reflect.Value) reflect.Value {
    // pf("rcs: original->%#v\n",original)
    copy := reflect.New(original.Type()).Elem()
    for i := 0; i < original.NumField(); i++ {
        fieldValue := original.Field(i)
        disableRO(&fieldValue)
        destField := copy.Field(i)
        disableRO(&destField)
        destField.Set(rcopy(fieldValue))
    }
    return copy
}

func rcopyChan(original reflect.Value) reflect.Value {
    return reflect.MakeChan(original.Type(), original.Cap())
}


func setupRO() {
    t:=reflect.TypeOf(reflect.Value{})
    for i:=0;i<t.NumField();i++ {
        f:=t.Field(i)
        if f.Name=="flag" {
            flagOffset=f.Offset
            return
        }
    }
    panic("No flag field found in reflect.Value")
}

var flagOffset uintptr // set by a call to setupRO() in main

const (
    // Lifted from go/src/reflect/value.go.
    flagStickyRO uintptr = 1 << 5
    flagEmbedRO  uintptr = 1 << 6
    flagRO       uintptr = flagStickyRO | flagEmbedRO
)

func disableRO(v *reflect.Value) {
    flags:=(*uintptr)(unsafe.Pointer(uintptr(unsafe.Pointer(v))+flagOffset))
    *flags &^= flagRO
}




