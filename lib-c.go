package main

import (
    "fmt"
    "io/ioutil"
    "os"
    "os/exec"
    "path/filepath"
    "plugin"
    "regexp"
    "runtime"
    "strconv"
    "strings"
    "sync"
    "unsafe"
)

// CType represents C data types that can be mapped to Za types
type CType int

const (
    CVoid CType = iota
    CInt
    CFloat
    CDouble
    CChar
    CString
    CBool
    CPointer
    CStruct
    CUInt
    CInt16
    CUInt16
    CInt64
    CUInt64
    CLongDouble
    CInt8
    CUInt8
)

// CSymbol represents a C function or variable symbol
type CSymbol struct {
    Name         string
    ReturnType   CType
    Parameters   []CParameter
    IsFunction   bool
    Address      uintptr
    Library      string
    SupportNotes []string
}

// CParameter represents a function parameter
type CParameter struct {
    Name           string
    Type           CType
    StructTypeName string // For CStruct types, the Za struct type name
}

// StructField represents a field in a C struct
type StructField struct {
    Name        string
    Type        CType
    Offset      uintptr
    ArraySize   int   // 0 for non-arrays, >0 for fixed-size arrays
    ElementType CType // For arrays, the type of array elements
    IsUnion     bool  // true if this field is a union type
    UnionDef    *CLibraryStruct // Union definition if IsUnion=true
    StructName  string // For nested struct fields (Type==CStruct), the struct type name
    StructDef   *CLibraryStruct // For nested struct fields, the struct definition
}

// CLibraryStruct represents a C struct or union definition
type CLibraryStruct struct {
    Name    string
    Fields  []StructField
    Size    uintptr
    IsUnion bool // true for unions, false for structs
}

// CLibrary represents a loaded C library
type CLibrary struct {
    Name    string
    Alias   string         // Namespace alias for this library (e.g., "c" for libc)
    Plugin  *plugin.Plugin
    Symbols map[string]*CSymbol
    Structs map[string]*CLibraryStruct
    Handle  unsafe.Pointer // dlopen handle for C libraries
}

// Close library and cleanup
func (lib *CLibrary) Close() error {
    // For now, just clear the maps
    lib.Symbols = nil
    lib.Structs = nil
    return nil
}

// Global registry of loaded C libraries
var loadedCLibraries = make(map[string]*CLibrary)

// CFunctionSignature represents an explicitly declared C function signature
type CFunctionSignature struct {
    ParamTypes       []CType  // Types of fixed parameters
    ParamStructNames []string // Parallel array for struct type names (empty for non-struct params)
    ReturnType       CType    // Return type
    ReturnStructName string   // For CStruct return values, the Za struct type name
    HasVarargs       bool     // True if function is variadic (takes variable arguments)
    FixedArgCount    int      // Number of fixed arguments before varargs
}

// Global registry of declared function signatures
// Map structure: libraryAlias -> functionName -> signature
var declaredSignatures = make(map[string]map[string]CFunctionSignature)

// DeclareCFunction stores an explicit function signature declaration
func DeclareCFunction(libraryAlias, functionName string, paramTypes []CType, paramStructNames []string, returnType CType, returnStructName string, hasVarargs bool) {
    if declaredSignatures[libraryAlias] == nil {
        declaredSignatures[libraryAlias] = make(map[string]CFunctionSignature)
    }
    fixedArgCount := len(paramTypes)
    declaredSignatures[libraryAlias][functionName] = CFunctionSignature{
        ParamTypes:       paramTypes,
        ParamStructNames: paramStructNames,
        ReturnType:       returnType,
        ReturnStructName: returnStructName,
        HasVarargs:       hasVarargs,
        FixedArgCount:    fixedArgCount,
    }
}

// GetDeclaredSignature retrieves a previously declared function signature
func GetDeclaredSignature(libraryAlias, functionName string) (CFunctionSignature, bool) {
    if declaredSignatures[libraryAlias] == nil {
        return CFunctionSignature{}, false
    }
    sig, ok := declaredSignatures[libraryAlias][functionName]
    return sig, ok
}

// CPointerValue represents an opaque C pointer that can be passed between FFI calls
type CPointerValue struct {
    Ptr     unsafe.Pointer
    TypeTag string // Optional type hint (e.g., "png_structp", "FILE*")
}

// IsNull returns true if the pointer is null
func (p *CPointerValue) IsNull() bool {
    return p == nil || p.Ptr == nil
}

// ToInt converts the pointer value to an integer
// This is useful for size_t, uintptr_t, and other integer-valued pointer returns
// Equivalent to c_ptr_to_int() but can be called as a method: ptr.ToInt()
func (p *CPointerValue) ToInt() int {
    if p == nil || p.Ptr == nil {
        return 0
    }
    return int(uintptr(p.Ptr))
}

// String representation for debugging
func (p *CPointerValue) String() string {
    if p == nil || p.Ptr == nil {
        return "CPointer(null)"
    }
    return fmt.Sprintf("CPointer(%s:%p)", p.TypeTag, p.Ptr)
}

// NullPointer returns a null CPointerValue
func NullPointer() *CPointerValue {
    return &CPointerValue{Ptr: nil, TypeTag: "null"}
}

// NewCPointer creates a new CPointerValue from an unsafe.Pointer
func NewCPointer(ptr unsafe.Pointer, typeTag string) *CPointerValue {
    return &CPointerValue{Ptr: ptr, TypeTag: typeTag}
}

// ConvertZaToCType maps Za types to C types
func ConvertZaToCType(zaType uint8) (CType, []string) {
    switch zaType {
    case kint:
        return CInt, nil
    case kuint:
        return CInt, nil
    case kfloat:
        return CFloat, nil
    case kstring:
        return CString, nil
    case kbool:
        return CBool, nil
    case kpointer:
        return CPointer, nil
    default:
        return CVoid, []string{fmt.Sprintf("[UNSUPPORTED: Cannot convert Za type %d to C type]", zaType)}
    }
}

// StringToCType converts type name strings to CType enum (for LIB declarations)
// Returns: CType, struct name (if applicable), error
func StringToCType(typeName string) (CType, string, error) {
    // Remove all spaces for parsing
    typeName = strings.ReplaceAll(typeName, " ", "")

    // Handle struct<typename> syntax
    if strings.HasPrefix(typeName, "struct<") && strings.HasSuffix(typeName, ">") {
        structName := typeName[7:len(typeName)-1] // Extract name from struct<name>
        return CStruct, structName, nil
    }

    switch strings.ToLower(typeName) {
    case "void":
        return CVoid, "", nil
    case "int":
        return CInt, "", nil
    case "uint":
        return CUInt, "", nil
    case "int16":
        return CInt16, "", nil
    case "uint16":
        return CUInt16, "", nil
    case "int64":
        return CInt64, "", nil
    case "uint64":
        return CUInt64, "", nil
    case "int8":
        return CInt8, "", nil
    case "uint8", "byte":
        return CUInt8, "", nil
    case "intptr":
        return CInt64, "intptr mapped to int64", nil
    case "uintptr":
        return CUInt64, "uintptr mapped to uint64", nil
    case "float":
        return CFloat, "", nil
    case "double":
        return CDouble, "", nil
    case "longdouble":
        return CLongDouble, "", nil
    case "char":
        return CChar, "", nil
    case "string":
        return CString, "", nil
    case "bool":
        return CBool, "", nil
    case "pointer", "ptr":
        return CPointer, "", nil
    case "struct":
        return CStruct, "", nil // Generic opaque struct pointer
    default:
        return CVoid, "", fmt.Errorf("unknown type name: %s", typeName)
    }
}

// ConvertZaToCTypes converts Za values to C values based on expected types
func ConvertZaToCTypes(args []any, params []CParameter) ([]any, error) {
    if len(args) != len(params) {
        return nil, fmt.Errorf("argument count mismatch: expected %d, got %d", len(params), len(args))
    }

    cArgs := make([]any, len(args))
    for i, arg := range args {
        expectedType := params[i].Type
        converted, err := ConvertZaToCValue(arg, expectedType)
        if err != nil {
            return nil, fmt.Errorf("parameter %d: %v", i, err)
        }
        cArgs[i] = converted
    }
    return cArgs, nil
}

// ConvertZaToCValue converts a single Za value to C value
func ConvertZaToCValue(zval any, expectedType CType) (any, error) {
    switch expectedType {
    case CInt:
        switch v := zval.(type) {
        case int:
            return v, nil
        case float64:
            return int(v), nil
        case string:
            ival, err := strconv.Atoi(v)
            if err != nil {
                return nil, fmt.Errorf("cannot convert string '%s' to int", v)
            }
            return ival, nil
        case bool:
            if v {
                return 1, nil
            }
            return 0, nil
        default:
            return nil, fmt.Errorf("cannot convert %T to C int", zval)
        }
    case CUInt:
        switch v := zval.(type) {
        case int:
            if v < 0 {
                return nil, fmt.Errorf("cannot convert negative int %d to C uint", v)
            }
            return uint(v), nil
        case uint:
            return v, nil
        case uint64:
            return uint(v), nil
        case float64:
            if v < 0 {
                return nil, fmt.Errorf("cannot convert negative float %f to C uint", v)
            }
            return uint(v), nil
        case string:
            uival, err := strconv.ParseUint(v, 10, 32)
            if err != nil {
                return nil, fmt.Errorf("cannot convert string '%s' to uint", v)
            }
            return uint(uival), nil
        case bool:
            if v {
                return uint(1), nil
            }
            return uint(0), nil
        default:
            return nil, fmt.Errorf("cannot convert %T to C uint", zval)
        }
    case CFloat:
        switch v := zval.(type) {
        case int:
            return float64(v), nil
        case float64:
            return v, nil
        case string:
            fval, err := strconv.ParseFloat(v, 64)
            if err != nil {
                return nil, fmt.Errorf("cannot convert string '%s' to float", v)
            }
            return fval, nil
        case bool:
            if v {
                return 1.0, nil
            }
            return 0.0, nil
        default:
            return nil, fmt.Errorf("cannot convert %T to C float", zval)
        }
    case CString:
        switch v := zval.(type) {
        case string:
            return v, nil
        default:
            str := fmt.Sprintf("%v", zval)
            return str, nil
        }
    case CBool:
        switch v := zval.(type) {
        case bool:
            return v, nil
        case int:
            return v != 0, nil
        case float64:
            return v != 0.0, nil
        case string:
            return v != "" && v != "0" && v != "false", nil
        default:
            return false, nil
        }
    case CInt16:
        switch v := zval.(type) {
        case int:
            if v < -32768 || v > 32767 {
                return nil, fmt.Errorf("value %d out of range for int16 (-32768 to 32767)", v)
            }
            return int16(v), nil
        case int64:
            if v < -32768 || v > 32767 {
                return nil, fmt.Errorf("value %d out of range for int16 (-32768 to 32767)", v)
            }
            return int16(v), nil
        case uint:
            if v > 32767 {
                return nil, fmt.Errorf("value %d out of range for int16 (-32768 to 32767)", v)
            }
            return int16(v), nil
        case uint64:
            if v > 32767 {
                return nil, fmt.Errorf("value %d out of range for int16 (-32768 to 32767)", v)
            }
            return int16(v), nil
        case uint8:
            return int16(v), nil // uint8 always fits in int16
        case float64:
            if v < -32768 || v > 32767 {
                return nil, fmt.Errorf("value %f out of range for int16 (-32768 to 32767)", v)
            }
            return int16(v), nil
        case string:
            ival, err := strconv.ParseInt(v, 10, 16)
            if err != nil {
                return nil, fmt.Errorf("cannot convert string '%s' to int16", v)
            }
            return int16(ival), nil
        case bool:
            if v {
                return int16(1), nil
            }
            return int16(0), nil
        default:
            return nil, fmt.Errorf("cannot convert %T to C int16", zval)
        }
    case CUInt16:
        switch v := zval.(type) {
        case int:
            if v < 0 || v > 65535 {
                return nil, fmt.Errorf("value %d out of range for uint16 (0 to 65535)", v)
            }
            return uint16(v), nil
        case int64:
            if v < 0 || v > 65535 {
                return nil, fmt.Errorf("value %d out of range for uint16 (0 to 65535)", v)
            }
            return uint16(v), nil
        case uint:
            if v > 65535 {
                return nil, fmt.Errorf("value %d out of range for uint16 (0 to 65535)", v)
            }
            return uint16(v), nil
        case uint64:
            if v > 65535 {
                return nil, fmt.Errorf("value %d out of range for uint16 (0 to 65535)", v)
            }
            return uint16(v), nil
        case uint8:
            return uint16(v), nil // uint8 always fits in uint16
        case float64:
            if v < 0 || v > 65535 {
                return nil, fmt.Errorf("value %f out of range for uint16 (0 to 65535)", v)
            }
            return uint16(v), nil
        case string:
            uival, err := strconv.ParseUint(v, 10, 16)
            if err != nil {
                return nil, fmt.Errorf("cannot convert string '%s' to uint16", v)
            }
            return uint16(uival), nil
        case bool:
            if v {
                return uint16(1), nil
            }
            return uint16(0), nil
        default:
            return nil, fmt.Errorf("cannot convert %T to C uint16", zval)
        }
    case CInt64:
        switch v := zval.(type) {
        case int64:
            return v, nil // Za native int64 type
        case int:
            return int64(v), nil
        case uint:
            return int64(v), nil
        case uint64:
            if v > 9223372036854775807 { // Max int64
                return nil, fmt.Errorf("value %d out of range for int64", v)
            }
            return int64(v), nil
        case uint8:
            return int64(v), nil
        case float64:
            return int64(v), nil
        case string:
            ival, err := strconv.ParseInt(v, 10, 64)
            if err != nil {
                return nil, fmt.Errorf("cannot convert string '%s' to int64", v)
            }
            return ival, nil
        case bool:
            if v {
                return int64(1), nil
            }
            return int64(0), nil
        default:
            return nil, fmt.Errorf("cannot convert %T to C int64", zval)
        }
    case CUInt64:
        switch v := zval.(type) {
        case uint64:
            return v, nil // Za native uint64 type
        case uint:
            return uint64(v), nil // Za native uint type
        case uint8:
            return uint64(v), nil // Za native uint8 type
        case int:
            if v < 0 {
                return nil, fmt.Errorf("cannot convert negative int %d to C uint64", v)
            }
            return uint64(v), nil
        case int64:
            if v < 0 {
                return nil, fmt.Errorf("cannot convert negative int64 %d to C uint64", v)
            }
            return uint64(v), nil
        case float64:
            if v < 0 {
                return nil, fmt.Errorf("cannot convert negative float %f to C uint64", v)
            }
            return uint64(v), nil
        case string:
            uival, err := strconv.ParseUint(v, 10, 64)
            if err != nil {
                return nil, fmt.Errorf("cannot convert string '%s' to uint64", v)
            }
            return uival, nil
        case bool:
            if v {
                return uint64(1), nil
            }
            return uint64(0), nil
        default:
            return nil, fmt.Errorf("cannot convert %T to C uint64", zval)
        }
    case CLongDouble:
        // Go doesn't have native long double, use float64
        switch v := zval.(type) {
        case int:
            return float64(v), nil
        case float64:
            return v, nil
        case string:
            fval, err := strconv.ParseFloat(v, 64)
            if err != nil {
                return nil, fmt.Errorf("cannot convert string '%s' to long double", v)
            }
            return fval, nil
        case bool:
            if v {
                return 1.0, nil
            }
            return 0.0, nil
        default:
            return nil, fmt.Errorf("cannot convert %T to C long double", zval)
        }
    case CInt8:
        switch v := zval.(type) {
        case int:
            if v < -128 || v > 127 {
                return nil, fmt.Errorf("value %d out of range for int8 (-128 to 127)", v)
            }
            return int8(v), nil
        case int64:
            if v < -128 || v > 127 {
                return nil, fmt.Errorf("value %d out of range for int8 (-128 to 127)", v)
            }
            return int8(v), nil
        case uint:
            if v > 127 {
                return nil, fmt.Errorf("value %d out of range for int8 (-128 to 127)", v)
            }
            return int8(v), nil
        case uint64:
            if v > 127 {
                return nil, fmt.Errorf("value %d out of range for int8 (-128 to 127)", v)
            }
            return int8(v), nil
        case uint8:
            if v > 127 {
                return nil, fmt.Errorf("value %d out of range for int8 (-128 to 127)", v)
            }
            return int8(v), nil
        case float64:
            if v < -128 || v > 127 {
                return nil, fmt.Errorf("value %f out of range for int8 (-128 to 127)", v)
            }
            return int8(v), nil
        case string:
            ival, err := strconv.ParseInt(v, 10, 8)
            if err != nil {
                return nil, fmt.Errorf("cannot convert string '%s' to int8", v)
            }
            return int8(ival), nil
        case bool:
            if v {
                return int8(1), nil
            }
            return int8(0), nil
        default:
            return nil, fmt.Errorf("cannot convert %T to C int8", zval)
        }
    case CUInt8:
        switch v := zval.(type) {
        case uint8:
            return v, nil // Za native uint8 type - direct mapping
        case int:
            if v < 0 || v > 255 {
                return nil, fmt.Errorf("value %d out of range for uint8 (0 to 255)", v)
            }
            return uint8(v), nil
        case int64:
            if v < 0 || v > 255 {
                return nil, fmt.Errorf("value %d out of range for uint8 (0 to 255)", v)
            }
            return uint8(v), nil
        case uint:
            if v > 255 {
                return nil, fmt.Errorf("value %d out of range for uint8 (0 to 255)", v)
            }
            return uint8(v), nil
        case uint64:
            if v > 255 {
                return nil, fmt.Errorf("value %d out of range for uint8 (0 to 255)", v)
            }
            return uint8(v), nil
        case float64:
            if v < 0 || v > 255 {
                return nil, fmt.Errorf("value %f out of range for uint8 (0 to 255)", v)
            }
            return uint8(v), nil
        case string:
            uival, err := strconv.ParseUint(v, 10, 8)
            if err != nil {
                return nil, fmt.Errorf("cannot convert string '%s' to uint8", v)
            }
            return uint8(uival), nil
        case bool:
            if v {
                return uint8(1), nil
            }
            return uint8(0), nil
        default:
            return nil, fmt.Errorf("cannot convert %T to C uint8", zval)
        }
    default:
        return zval, nil // Pass through for unsupported types
    }
}

// ConvertCToZaValue converts a C value to Za value
func ConvertCToZaValue(cval any, cType CType) (any, error) {
    switch cType {
    case CInt, CChar:
        switch v := cval.(type) {
        case int:
            return v, nil
        case int32:
            return int(v), nil
        default:
            return cval, nil
        }
    case CUInt:
        switch v := cval.(type) {
        case uint:
            return int(v), nil // Convert to Za int
        case uint32:
            return int(v), nil // Convert to Za int
        default:
            return cval, nil
        }
    case CFloat, CDouble:
        switch v := cval.(type) {
        case float64:
            return v, nil
        case float32:
            return float64(v), nil
        default:
            return cval, nil
        }
    case CString:
        switch v := cval.(type) {
        case string:
            return v, nil
        default:
            return fmt.Sprintf("%v", cval), nil
        }
    case CBool:
        switch v := cval.(type) {
        case bool:
            return v, nil
        case int:
            return v != 0, nil
        default:
            return cval != nil, nil
        }
    case CInt16:
        switch v := cval.(type) {
        case int16:
            return int(v), nil
        case int:
            return v, nil
        default:
            return cval, nil
        }
    case CUInt16:
        switch v := cval.(type) {
        case uint16:
            return int(v), nil
        case uint:
            return int(v), nil
        default:
            return cval, nil
        }
    case CInt64:
        switch v := cval.(type) {
        case int64:
            return int(v), nil // May lose precision on 32-bit platforms
        case int:
            return v, nil
        default:
            return cval, nil
        }
    case CUInt64:
        switch v := cval.(type) {
        case uint64:
            return int(v), nil // May lose precision or overflow
        case uint:
            return int(v), nil
        default:
            return cval, nil
        }
    case CLongDouble:
        switch v := cval.(type) {
        case float64:
            return v, nil
        case float32:
            return float64(v), nil
        default:
            return cval, nil
        }
    case CInt8:
        switch v := cval.(type) {
        case int8:
            return int(v), nil
        case int:
            return v, nil
        default:
            return cval, nil
        }
    case CUInt8:
        switch v := cval.(type) {
        case uint8:
            return v, nil // Return as uint8 (Za native type)
        case uint:
            return uint8(v), nil
        case int:
            if v >= 0 && v <= 255 {
                return uint8(v), nil
            }
            return cval, nil
        default:
            return cval, nil
        }
    case CPointer, CVoid:
        return cval, nil // Pointers and void as-is for now
    default:
        return cval, nil
    }
}

// RegisterCSymbol adds a C symbol to global registry
func RegisterCSymbol(symbol *CSymbol) {
    if library, exists := loadedCLibraries[symbol.Library]; exists {
        library.Symbols[symbol.Name] = symbol
    }
}

// CallCFunction executes a C function via FFI using dlsym
func CallCFunction(library string, functionName string, args []any) (any, []string) {
    lib, exists := loadedCLibraries[library]
    if !exists {
        return nil, []string{fmt.Sprintf("[ERROR: C library '%s' not loaded]", library)}
    }

    symbol, exists := lib.Symbols[functionName]
    if !exists {
        return nil, []string{fmt.Sprintf("[ERROR: Function '%s' not found in library '%s']", functionName, library)}
    }

    if !symbol.IsFunction {
        return nil, []string{fmt.Sprintf("[ERROR: '%s' is not a function]", functionName)}
    }

    // Delegate to platform-specific implementation
    return callCFunctionPlatform(lib, functionName, args)
}

// GetCLibrarySymbols returns all symbols from a loaded C library
func GetCLibrarySymbols(library string) ([]*CSymbol, error) {
    lib, exists := loadedCLibraries[library]
    if !exists {
        return nil, fmt.Errorf("C library '%s' not loaded", library)
    }

    symbols := make([]*CSymbol, 0, len(lib.Symbols))
    for _, symbol := range lib.Symbols {
        symbols = append(symbols, symbol)
    }
    return symbols, nil
}

// FindCFunction finds the first C library that contains a function and returns the namespace name
// Returns empty string if function not found in any C library
func FindCFunction(functionName string) string {
    // Strip namespace prefix if present (e.g., "png::func" -> "func")
    cleanName := functionName
    if strings.Contains(functionName, "::") {
        parts := strings.SplitN(functionName, "::", 2)
        if len(parts) == 2 {
            cleanName = parts[1]
        }
    }

    // Search all loaded C libraries for this function
    for libName, lib := range loadedCLibraries {
        if symbol, exists := lib.Symbols[cleanName]; exists {
            if symbol.IsFunction {
                return libName
            }
        }
    }
    return ""
}

// isCFunction checks if a namespace and function name correspond to a loaded C library function
func isCFunction(namespace, functionName string) bool {
    if lib, exists := loadedCLibraries[namespace]; exists {
        if symbol, symbolExists := lib.Symbols[functionName]; symbolExists {
            return symbol.IsFunction
        }
    }
    return false
}

// GetCFunctionSignature generates a readable signature for a C function
func GetCFunctionSignature(symbol *CSymbol) string {
    if !symbol.IsFunction {
        return fmt.Sprintf("%s (data)", symbol.Name)
    }

    // Build parameter list
    var paramStr strings.Builder
    for i, param := range symbol.Parameters {
        if i > 0 {
            paramStr.WriteString(", ")
        }
        paramStr.WriteString(fmt.Sprintf("%s %s", param.Name, CTypeToString(param.Type)))
    }

    // Return type string
    returnType := CTypeToString(symbol.ReturnType)

    return fmt.Sprintf("%s(%s) -> %s", symbol.Name, paramStr.String(), returnType)
}

// FormatDeclaredSignature formats a declared function signature for display
func FormatDeclaredSignature(functionName string, sig CFunctionSignature) string {
    var paramStr strings.Builder

    // Build parameter list
    for i, paramType := range sig.ParamTypes {
        if i > 0 {
            paramStr.WriteString(", ")
        }

        // Use generic param names (arg1, arg2, etc.) since we don't store param names
        paramStr.WriteString(fmt.Sprintf("arg%d:%s", i+1, CTypeToString(paramType)))

        // Add struct type name if present
        if len(sig.ParamStructNames) > i && sig.ParamStructNames[i] != "" {
            paramStr.WriteString(fmt.Sprintf("<%s>", sig.ParamStructNames[i]))
        }
    }

    // Add varargs indicator
    if sig.HasVarargs {
        if len(sig.ParamTypes) > 0 {
            paramStr.WriteString(", ")
        }
        paramStr.WriteString("...args")
    }

    // Format return type
    returnType := CTypeToString(sig.ReturnType)
    if sig.ReturnStructName != "" {
        returnType += fmt.Sprintf("<%s>", sig.ReturnStructName)
    }

    return fmt.Sprintf("%s(%s) -> %s", functionName, paramStr.String(), returnType)
}

// CTypeToString converts C type enum to string
func CTypeToString(cType CType) string {
    switch cType {
    case CVoid:
        return "void"
    case CInt:
        return "int"
    case CInt8:
        return "int8"
    case CInt16:
        return "int16"
    case CInt64:
        return "int64"
    case CUInt:
        return "uint"
    case CUInt8:
        return "uint8"
    case CUInt16:
        return "uint16"
    case CUInt64:
        return "uint64"
    case CFloat:
        return "float"
    case CDouble:
        return "double"
    case CLongDouble:
        return "long double"
    case CChar:
        return "char"
    case CString:
        return "char*"
    case CBool:
        return "bool"
    case CPointer:
        return "void*"
    case CStruct:
        return "struct"
    default:
        return "unknown"
    }
}

// buildFfiLib registers FFI helper functions in Za's stdlib
func buildFfiLib() {
    features["ffi"] = Feature{version: 1, category: "ffi"}
    categories["ffi"] = []string{"c_null", "c_fopen", "c_fclose", "c_ptr_is_null", "c_ptr_to_int", "c_alloc", "c_free", "c_set_byte", "c_get_byte", "c_get_uint16", "c_get_uint32", "c_get_int16", "c_get_int32", "c_get_symbol", "c_alloc_struct", "c_free_struct", "c_unmarshal_struct"}

    slhelp["c_null"] = LibHelp{in: "", out: "cpointer", action: "Returns a null C pointer for use in FFI calls."}
    stdlib["c_null"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        return NullPointer(), nil
    }

    slhelp["c_fopen"] = LibHelp{in: "path,mode", out: "cpointer", action: "Opens a file and returns a FILE* pointer. Mode is like C fopen (\"r\", \"w\", \"rb\", \"wb\", etc)."}
    stdlib["c_fopen"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("c_fopen", args, 1, "2", "string", "string"); !ok {
            return nil, err
        }
        return CFopen(args[0].(string), args[1].(string)), nil
    }

    slhelp["c_fclose"] = LibHelp{in: "file_ptr", out: "int", action: "Closes a FILE* pointer. Returns 0 on success."}
    stdlib["c_fclose"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if len(args) != 1 {
            return -1, fmt.Errorf("c_fclose requires 1 argument")
        }
        if p, ok := args[0].(*CPointerValue); ok {
            return CFclose(p), nil
        }
        return -1, fmt.Errorf("c_fclose requires a C pointer argument")
    }

    slhelp["c_ptr_is_null"] = LibHelp{in: "ptr", out: "bool", action: "Returns true if the C pointer is null."}
    stdlib["c_ptr_is_null"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if len(args) != 1 {
            return true, fmt.Errorf("c_ptr_is_null requires 1 argument")
        }
        if p, ok := args[0].(*CPointerValue); ok {
            return CPtrIsNull(p), nil
        }
        // Non-pointer values (like unmarshalled structs/maps) are not null
        return false, nil
    }

    slhelp["c_alloc"] = LibHelp{in: "size", out: "cpointer", action: "Allocates a zero-initialized byte buffer of the given size."}
    stdlib["c_alloc"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("c_alloc", args, 1, "1", "int"); !ok {
            return nil, err
        }
        return CAllocBytes(args[0].(int)), nil
    }

    slhelp["c_free"] = LibHelp{in: "ptr", out: "", action: "Frees a C pointer allocated by c_alloc."}
    stdlib["c_free"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if len(args) != 1 {
            return nil, fmt.Errorf("c_free requires 1 argument")
        }
        if p, ok := args[0].(*CPointerValue); ok {
            CFreePtr(p)
        }
        return nil, nil
    }

    slhelp["c_set_byte"] = LibHelp{in: "ptr,offset,value", out: "", action: "Sets a byte at the given offset in a buffer."}
    stdlib["c_set_byte"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("c_set_byte", args, 1, "3", "any", "int", "int"); !ok {
            return nil, err
        }
        if p, ok := args[0].(*CPointerValue); ok {
            CSetByte(p, args[1].(int), byte(args[2].(int)))
        }
        return nil, nil
    }

    slhelp["c_get_byte"] = LibHelp{in: "ptr,offset", out: "int", action: "Reads a byte at the given offset in a buffer."}
    stdlib["c_get_byte"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("c_get_byte", args, 1, "2", "any", "int"); !ok {
            return nil, err
        }
        if p, ok := args[0].(*CPointerValue); ok {
            return int(CGetByte(p, args[1].(int))), nil
        }
        return 0, fmt.Errorf("c_get_byte: first argument must be a C pointer")
    }

    slhelp["c_get_uint16"] = LibHelp{in: "ptr,offset", out: "int", action: "Reads a uint16 at the given offset in a buffer."}
    stdlib["c_get_uint16"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("c_get_uint16", args, 1, "2", "any", "int"); !ok {
            return nil, err
        }
        if p, ok := args[0].(*CPointerValue); ok {
            return int(CGetUint16(p, args[1].(int))), nil
        }
        return 0, fmt.Errorf("c_get_uint16: first argument must be a C pointer")
    }

    slhelp["c_get_uint32"] = LibHelp{in: "ptr,offset", out: "int", action: "Reads a uint32 at the given offset in a buffer."}
    stdlib["c_get_uint32"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("c_get_uint32", args, 1, "2", "any", "int"); !ok {
            return nil, err
        }
        if p, ok := args[0].(*CPointerValue); ok {
            return int(CGetUint32(p, args[1].(int))), nil
        }
        return 0, fmt.Errorf("c_get_uint32: first argument must be a C pointer")
    }

    slhelp["c_get_int16"] = LibHelp{in: "ptr,offset", out: "int", action: "Reads an int16 at the given offset in a buffer."}
    stdlib["c_get_int16"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("c_get_int16", args, 1, "2", "any", "int"); !ok {
            return nil, err
        }
        if p, ok := args[0].(*CPointerValue); ok {
            return int(CGetInt16(p, args[1].(int))), nil
        }
        return 0, fmt.Errorf("c_get_int16: first argument must be a C pointer")
    }

    slhelp["c_get_int32"] = LibHelp{in: "ptr,offset", out: "int", action: "Reads an int32 at the given offset in a buffer."}
    stdlib["c_get_int32"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("c_get_int32", args, 1, "2", "any", "int"); !ok {
            return nil, err
        }
        if p, ok := args[0].(*CPointerValue); ok {
            return int(CGetInt32(p, args[1].(int))), nil
        }
        return 0, fmt.Errorf("c_get_int32: first argument must be a C pointer")
    }

    slhelp["c_ptr_to_int"] = LibHelp{in: "ptr", out: "int", action: "Converts a C pointer to an integer. Useful for size_t, uintptr_t, and other integer-valued returns. Tip: Can also use ptr.ToInt() method for convenience."}
    stdlib["c_ptr_to_int"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("c_ptr_to_int", args, 1, "1", "any"); !ok {
            return nil, err
        }
        if p, ok := args[0].(*CPointerValue); ok {
            return p.ToInt(), nil
        }
        return nil, fmt.Errorf("c_ptr_to_int: argument is not a C pointer")
    }

    slhelp["c_get_uint64"] = LibHelp{in: "ptr,offset", out: "uint", action: "Reads a uint64 at the given offset in a buffer."}
    stdlib["c_get_uint64"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("c_get_uint64", args, 1, "2", "any", "int"); !ok {
            return nil, err
        }
        if p, ok := args[0].(*CPointerValue); ok {
            return CGetUint64(p, args[1].(int)), nil
        }
        return uint64(0), fmt.Errorf("c_get_uint64: first argument must be a C pointer")
    }

    slhelp["c_get_int64"] = LibHelp{in: "ptr,offset", out: "int", action: "Reads an int64 at the given offset in a buffer."}
    stdlib["c_get_int64"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("c_get_int64", args, 1, "2", "any", "int"); !ok {
            return nil, err
        }
        if p, ok := args[0].(*CPointerValue); ok {
            return CGetInt64(p, args[1].(int)), nil
        }
        return int64(0), fmt.Errorf("c_get_int64: first argument must be a C pointer")
    }

    slhelp["c_get_symbol"] = LibHelp{in: "library_alias,symbol_name", out: "any", action: "Reads a data symbol (constant/variable) from a loaded C library. Note: C preprocessor #defines are NOT symbols and cannot be read this way."}
    stdlib["c_get_symbol"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("c_get_symbol", args, 1, "2", "string", "string"); !ok {
            return nil, err
        }
        return CGetDataSymbol(args[0].(string), args[1].(string))
    }

    slhelp["c_alloc_struct"] = LibHelp{in: "struct_type_name", out: "cpointer", action: "Allocates memory for a C struct of the given Za struct type. The struct must be defined with the 'struct' keyword."}
    stdlib["c_alloc_struct"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("c_alloc_struct", args, 1, "1", "string"); !ok {
            return nil, err
        }
        structName := args[0].(string)

        // Get struct layout from Za struct definition
        structDef, err := getStructLayoutFromZa(structName)
        if err != nil {
            return nil, fmt.Errorf("c_alloc_struct: %v", err)
        }

        // Allocate memory for the struct
        return CAllocBytes(int(structDef.Size)), nil
    }

    slhelp["c_free_struct"] = LibHelp{in: "struct_ptr", out: "", action: "Frees a C struct pointer allocated by c_alloc_struct."}
    stdlib["c_free_struct"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if len(args) != 1 {
            return nil, fmt.Errorf("c_free_struct requires 1 argument")
        }
        if p, ok := args[0].(*CPointerValue); ok {
            CFreePtr(p)
        }
        return nil, nil
    }

    slhelp["c_unmarshal_struct"] = LibHelp{in: "struct_ptr,struct_type_name", out: "struct", action: "Reads C struct data from memory and converts it to a Za struct. Use this for 'out' parameters that C functions fill with data."}
    stdlib["c_unmarshal_struct"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("c_unmarshal_struct", args, 1, "2", "any", "string"); !ok {
            return nil, err
        }

        // Get the C pointer
        ptr, ok := args[0].(*CPointerValue)
        if !ok {
            return nil, fmt.Errorf("c_unmarshal_struct: first argument must be a C pointer")
        }

        if ptr.Ptr == nil {
            return nil, fmt.Errorf("c_unmarshal_struct: pointer is null")
        }

        structName := args[1].(string)

        // Get struct layout from Za struct definition
        structDef, err := getStructLayoutFromZa(structName)
        if err != nil {
            return nil, fmt.Errorf("c_unmarshal_struct: %v", err)
        }

        // Unmarshal C memory to Za struct or union
        if structDef.IsUnion {
            return unmarshalUnion(ptr.Ptr, structDef)
        }
        return UnmarshalStructFromC(ptr.Ptr, structDef, structName)
    }
}
// FunctionSignature represents a parsed C function signature from man pages
type FunctionSignature struct {
    ReturnType         CType
    ReturnStructName   string // For struct/union return types, the type name
    Parameters         []CParameter
    IsVariadic         bool
    RawSignature       string // Original C signature
}

// extractLibraryBaseName extracts base library name from file path
// Examples:
//   "/usr/lib/libpng16.so.16" → "libpng"
//   "/usr/lib/x86_64-linux-gnu/libjson-c.so.5" → "libjson-c"
//   "/lib64/libc.so.6" → "libc"
func extractLibraryBaseName(libraryPath string) string {
    // 1. Get basename: /usr/lib/libpng16.so.16 → libpng16.so.16
    base := filepath.Base(libraryPath)

    // 2. Remove .so* suffix: libpng16.so.16 → libpng16
    if idx := strings.Index(base, ".so"); idx != -1 {
        base = base[:idx]
    }

    // 3. Remove version numbers from end: libpng16 → libpng
    // Match pattern: lib<name><version_numbers>
    re := regexp.MustCompile(`^(lib[a-zA-Z_-]+?)(\d+)?$`)
    if matches := re.FindStringSubmatch(base); len(matches) > 1 {
        return matches[1] // Return lib<name> without version
    }

    return base // Return as-is if no pattern match
}
// stripNonTypeTokens removes API macros and qualifiers, keeping only valid C type components
func stripNonTypeTokens(typeStr string) string {
    // Known C type keywords that should be preserved
    validTypeKeywords := map[string]bool{
        // Basic types
        "void": true, "char": true, "short": true, "int": true, "long": true,
        "float": true, "double": true,
        "signed": true, "unsigned": true,
        "_Bool": true, "_Complex": true, "_Imaginary": true,

        // Type qualifiers (usually stripped later, but valid)
        "const": true, "volatile": true, "restrict": true, "_Atomic": true,

        // Type prefixes
        "struct": true, "union": true, "enum": true,
    }

    // Storage class and other specifiers to strip
    stripKeywords := map[string]bool{
        "static": true, "extern": true, "auto": true, "register": true,
        "typedef": true, "inline": true, "__inline": true, "__inline__": true,
    }

    // Tokenize preserving * as separate tokens
    var tokens []string
    currentToken := ""
    for _, ch := range typeStr {
        if ch == '*' {
            if currentToken != "" {
                tokens = append(tokens, strings.TrimSpace(currentToken))
                currentToken = ""
            }
            tokens = append(tokens, "*")
        } else if ch == ' ' || ch == '\t' {
            if currentToken != "" {
                tokens = append(tokens, currentToken)
                currentToken = ""
            }
        } else {
            currentToken += string(ch)
        }
    }
    if currentToken != "" {
        tokens = append(tokens, currentToken)
    }

    // Filter tokens
    var kept []string
    expectTypeNameAfter := false // true after struct/union/enum

    for _, token := range tokens {
        // Always keep pointers
        if token == "*" {
            kept = append(kept, token)
            continue
        }

        // Keep valid type keywords
        if validTypeKeywords[token] {
            kept = append(kept, token)
            // If this is struct/union/enum, next identifier is the type name
            if token == "struct" || token == "union" || token == "enum" {
                expectTypeNameAfter = true
            }
            continue
        }

        // Strip known non-type keywords
        if stripKeywords[token] {
            continue
        }

        // If we're expecting a type name after struct/union/enum, keep it
        if expectTypeNameAfter {
            kept = append(kept, token)
            expectTypeNameAfter = false
            continue
        }

        // For remaining identifiers:
        // - If it's all uppercase or has underscores at start/end, likely a macro -> strip
        // - Otherwise, might be a typedef -> keep it
        isLikelyMacro := false
        if strings.ToUpper(token) == token && len(token) > 2 {
            // All uppercase and longer than 2 chars -> likely macro
            isLikelyMacro = true
        }
        if strings.HasPrefix(token, "__") || strings.HasSuffix(token, "__") {
            // Double underscore prefix/suffix -> likely compiler macro
            isLikelyMacro = true
        }
        if strings.HasSuffix(token, "_t") {
            // POSIX typedef convention (uint32_t, size_t, etc.) -> keep
            isLikelyMacro = false
        }

        if !isLikelyMacro {
            kept = append(kept, token)
        }
        // else: strip this token (it's likely a macro)
    }

    result := strings.Join(kept, " ")

    // Handle edge case: if we're left with only pointer(s) and no type
    // e.g., "XMLPUBFUN*" → ["*"] after filtering
    // Default to void* in this case
    result = strings.TrimSpace(result)
    if result == "*" || result == "* *" || result == "**" {
        result = "void *"
    } else if strings.HasPrefix(result, "* ") {
        // Starts with dangling pointer (no type before it)
        result = "void " + result
    }

    return result
}

// mapCTypeStringToZa converts a C type string to Za CType enum
// Handles common C types including modifiers and pointers
func mapCTypeStringToZa(cTypeStr string, alias string) (CType, string, error) {
    // Remove leading/trailing whitespace first
    cTypeStr = strings.TrimSpace(cTypeStr)

    // Strip storage class specifiers BEFORE typedef resolution
    // These keywords are not part of the type name and prevent typedef matching
    storageClassSpecifiers := []string{"extern", "static", "auto", "register", "typedef"}
    for _, keyword := range storageClassSpecifiers {
        cTypeStr = strings.TrimPrefix(cTypeStr, keyword+" ")
        cTypeStr = strings.TrimSpace(cTypeStr)
    }

    // Check if this is a known struct/union typedef BEFORE resolution
    // This allows us to preserve the original type name for struct/union references
    if alias != "" {
        cleanType := strings.TrimSpace(cTypeStr)
        cleanType = strings.TrimPrefix(cleanType, "const ")
        cleanType = strings.TrimPrefix(cleanType, "struct ")
        cleanType = strings.TrimPrefix(cleanType, "union ")
        cleanType = strings.TrimSpace(cleanType)

        // Check if this is a pointer-to-struct type (ends with *)
        // For pointer-to-struct, strip the * for lookup but keep track that it's a pointer
        isPointerType := strings.HasSuffix(cleanType, "*")
        lookupName := cleanType
        if isPointerType {
            // Strip all trailing * for struct lookup
            lookupName = strings.TrimRight(cleanType, "*")
            lookupName = strings.TrimSpace(lookupName)
        }

        // Check if it's a known struct/union from AUTO parsing
        ffiStructLock.RLock()
        if def, exists := ffiStructDefinitions[lookupName]; exists {
            ffiStructLock.RUnlock()
            if os.Getenv("ZA_DEBUG_AUTO") != "" {
                fmt.Fprintf(os.Stderr, "[AUTO] Type %s is known %s, using as struct reference\n",
                    lookupName, map[bool]string{true: "union", false: "struct"}[def.IsUnion])
            }

            // For pointer-to-struct types, return as CPointer with the struct name preserved
            // This allows auto-unmarshalling in convertReturnValue
            // For value struct types, return as CStruct
            if isPointerType {
                // Return CPointer but preserve the struct name (with * suffix) for later unmarshalling
                return CPointer, cleanType, nil
            } else {
                return CStruct, lookupName, nil
            }
        }
        ffiStructLock.RUnlock()
    }

    // Try typedef resolution for other types
    if alias != "" {
        if resolved := resolveTypedef(cTypeStr, alias, 0); resolved != "" {
            if os.Getenv("ZA_DEBUG_AUTO") != "" {
                fmt.Printf("[AUTO] Resolved typedef: %s → %s (alias: %s)\n", cTypeStr, resolved, alias)
            }
            cTypeStr = resolved  // Replace with resolved type
        }
    }

    // Split on * to separate pointers from types and modifiers
    // e.g., "char *restrict" -> handle separately
    parts := strings.Split(cTypeStr, "*")
    isPointer := len(parts) > 1

    // Clean up the base type (everything before *)
    baseTypePart := strings.TrimSpace(parts[0])

    // Strip non-type tokens (API macros, qualifiers, etc.) using whitelist approach
    baseTypePart = stripNonTypeTokens(baseTypePart)

    // Remove type qualifiers and storage class specifiers
    // NOTE: Do NOT remove "unsigned" or "signed" - they are part of the type name!
    // "unsigned int" is a different type than "int"
    qualifiers := []string{"const", "volatile", "static", "inline", "restrict"}
    for _, qual := range qualifiers {
        baseTypePart = strings.Replace(baseTypePart, qual+" ", "", -1)
        baseTypePart = strings.Replace(baseTypePart, qual, "", -1)
    }
    baseType := strings.TrimSpace(baseTypePart)

    // Check for common typedef patterns that indicate pointers
    // Many libraries use 'p' or 'ptr' suffix for pointer typedefs
    lowerType := strings.ToLower(baseType)

    // Pattern 1: Types ending in "charp", "*charp", "char_p" are char pointers (strings)
    // Examples: png_charp, png_const_charp, json_charp, etc.
    if strings.HasSuffix(lowerType, "charp") || strings.HasSuffix(lowerType, "char_p") ||
       strings.HasSuffix(lowerType, "char_ptr") || strings.Contains(lowerType, "charp") {
        return CString, baseType, nil
    }

    // Pattern 2: Types containing "string" are likely strings
    // Examples: string_t, StringPtr, etc.
    if strings.Contains(lowerType, "string") {
        return CString, baseType, nil
    }

    // Pattern 3: Types ending in 'p' or 'ptr' (except common non-pointer types)
    // Examples: png_structp, voidp, intp, json_objectp, etc.
    // Exclude: "exp" (exponential), "tmp" (temporary), "cmp" (compare), etc.
    if !isPointer && (strings.HasSuffix(lowerType, "p") || strings.HasSuffix(lowerType, "ptr")) {
        // Check if it's not a common false positive
        falsePositives := []string{"exp", "tmp", "cmp", "amp"}
        isFalsePositive := false
        for _, fp := range falsePositives {
            if lowerType == fp || strings.HasSuffix(lowerType, "_"+fp) {
                isFalsePositive = true
                break
            }
        }

        if !isFalsePositive && len(lowerType) > 1 {
            // This is likely a pointer typedef
            isPointer = true
            // Note: We don't know what it points to, so treat as generic pointer
            // unless we can infer from the name
        }
    }

    // Map C types to Za types
    switch baseType {
    case "void":
        if isPointer {
            return CPointer, "", nil // void* → pointer
        }
        return CVoid, "", nil

    case "char":
        if isPointer {
            return CString, "", nil // char* → string
        }
        return CChar, "", nil

    // Signed integer types
    // Note: "signed" alone means "signed int", "long" alone means "long int"
    case "int", "signed", "signed int",
        "long", "long int", "signed long", "signed long int",
        "short", "short int", "signed short", "signed short int",
        "long long", "long long int", "signed long long", "signed long long int",
        "int8_t", "int16_t", "int32_t", "int64_t",
        "ssize_t", "off_t", "pid_t", "time_t",
        "intptr_t", "ptrdiff_t":
        // 8-bit signed integers
        if baseType == "int8_t" {
            return CInt8, baseType + " mapped to int8", nil
        }
        // 16-bit signed integers
        if baseType == "short" || baseType == "short int" ||
           baseType == "signed short" || baseType == "signed short int" ||
           baseType == "int16_t" {
            return CInt16, baseType + " mapped to int16", nil
        }
        // 64-bit signed integers
        // Note: On LP64 systems (Linux, macOS, BSD), "long" is 64-bit
        if baseType == "long long" || baseType == "long long int" ||
           baseType == "signed long long" || baseType == "signed long long int" ||
           baseType == "long" || baseType == "long int" ||
           baseType == "signed long" || baseType == "signed long int" ||
           baseType == "int64_t" || baseType == "off_t" ||
           baseType == "intptr_t" || baseType == "ptrdiff_t" {
            return CInt64, baseType + " mapped to int64", nil
        }
        // Default to 32-bit for int, signed, signed int, pid_t, time_t
        if baseType == "ssize_t" {
            return CInt64, baseType + " mapped to int64", nil
        }
        return CInt, "", nil

    // Unsigned integer types
    // Note: "unsigned" alone means "unsigned int"
    case "unsigned", "unsigned int",
        "unsigned long", "unsigned long int",
        "unsigned short", "unsigned short int",
        "unsigned long long", "unsigned long long int",
        "uint8_t", "uint16_t", "uint32_t", "uint64_t",
        "size_t", "uid_t", "gid_t", "mode_t",
        "uintptr_t":
        // 8-bit unsigned integers
        if baseType == "uint8_t" {
            return CUInt8, baseType + " mapped to uint8", nil
        }
        // 16-bit unsigned integers
        if baseType == "unsigned short" || baseType == "unsigned short int" ||
           baseType == "uint16_t" {
            return CUInt16, baseType + " mapped to uint16", nil
        }
        // 64-bit unsigned integers
        // Note: On LP64 systems (Linux, macOS, BSD), "unsigned long" is 64-bit
        if baseType == "unsigned long long" || baseType == "unsigned long long int" ||
           baseType == "unsigned long" || baseType == "unsigned long int" ||
           baseType == "uint64_t" || baseType == "uintptr_t" {
            return CUInt64, baseType + " mapped to uint64", nil
        }
        // size_t handling: typically 64-bit on 64-bit systems, 32-bit on 32-bit systems
        // For simplicity, map to uint64 to avoid truncation issues
        if baseType == "size_t" {
            return CUInt64, baseType + " mapped to uint64", nil
        }
        // Default to 32-bit for unsigned, unsigned int, uid_t, gid_t, mode_t
        return CUInt, "", nil

    case "float":
        return CFloat, "", nil

    case "double":
        return CDouble, "", nil

    case "long double":
        return CLongDouble, "", nil

    case "bool", "_Bool":
        return CBool, "", nil

    case "wchar_t":
        // wchar_t* is a pointer (wide character string)
        if isPointer {
            return CPointer, "wchar_t* mapped to pointer", nil
        }
        // Platform-dependent size detected at init
        if wcharSize == 2 {
            return CUInt16, "wchar_t mapped to uint16", nil
        } else if wcharSize == 4 {
            return CUInt, "wchar_t mapped to uint32", nil
        }
        // Fallback if size unknown
        return CPointer, "wchar_t (unknown size) mapped to pointer", nil

    default:
        // Check for enum types (should map to int)
        if strings.HasPrefix(baseType, "enum ") ||
           strings.HasPrefix(baseType, "enum{") ||
           baseType == "enum" {
            return CInt, "", nil
        }

        // Heuristic: types containing "uint" are likely unsigned integers
        if strings.Contains(strings.ToLower(baseType), "uint") {
            return CUInt, baseType, nil
        }

        // Unknown types or struct/union pointers default to pointer
        if isPointer {
            return CPointer, baseType + "*", nil
        }
        return CPointer, baseType, nil
    }
}

// tryManPage attempts to get signature from a man page
// If manPageName == functionName, looks for exact function signature
// If manPageName is a library name, searches within the page for functionName
func tryManPage(manPageName string, functionName string) (string, error) {
    // Execute man command for section 3
    cmd := exec.Command("man", "3", manPageName)
    output, err := cmd.Output()
    if err != nil {
        return "", fmt.Errorf("man page not found for %s", manPageName)
    }

    manText := string(output)

    // Find SYNOPSIS section
    synopsisRegex := regexp.MustCompile(`(?m)^SYNOPSIS\s*$`)
    synopsisIdx := synopsisRegex.FindStringIndex(manText)
    if synopsisIdx == nil {
        return "", fmt.Errorf("SYNOPSIS section not found")
    }

    // Find next section after SYNOPSIS
    sectionEndRegex := regexp.MustCompile(`(?m)^[A-Z][A-Z\s]+$`)
    remainingText := manText[synopsisIdx[1]:]
    sectionEndIdx := sectionEndRegex.FindStringIndex(remainingText)

    var synopsisText string
    if sectionEndIdx != nil {
        synopsisText = remainingText[:sectionEndIdx[0]]
    } else {
        synopsisText = remainingText
    }

    // Extract lines containing the function signature
    lines := strings.Split(synopsisText, "\n")
    var signatureLines []string
    inFunction := false

    for _, line := range lines {
        // Skip #include lines
        if strings.Contains(line, "#include") {
            continue
        }

        trimmed := strings.TrimSpace(line)

        // Look for the function name
        if !inFunction && (strings.Contains(trimmed, functionName+"(") ||
            strings.Contains(trimmed, functionName+" (")) {
            inFunction = true
            signatureLines = append(signatureLines, trimmed)
        } else if inFunction && trimmed != "" {
            // Continue collecting multi-line signature
            signatureLines = append(signatureLines, trimmed)
        }

        // Check if we have a complete signature
        if inFunction && len(signatureLines) > 0 {
            // Join current lines to check for completion
            joined := strings.Join(signatureLines, " ")

            // Signature is complete when:
            // 1. Has opening parenthesis AND
            // 2. Has at least as many closing parens as opening parens AND
            // 3. Either ends with ; or has balanced parens
            openCount := strings.Count(joined, "(")
            closeCount := strings.Count(joined, ")")

            if openCount > 0 && closeCount >= openCount {
                // Check if this looks complete (ends with ) or );)
                if strings.HasSuffix(strings.TrimSpace(joined), ")") ||
                   strings.HasSuffix(strings.TrimSpace(joined), ");") {
                    break
                }
            }
        }
    }

    if len(signatureLines) == 0 {
        return "", fmt.Errorf("function '%s' not found in man page '%s'", functionName, manPageName)
    }

    // Join multi-line signatures and clean up
    signature := strings.Join(signatureLines, " ")
    signature = strings.TrimSuffix(signature, ";")
    signature = strings.TrimSpace(signature)

    return signature, nil
}

// parseManPageLocal executes `man 3 <function>` and extracts the function signature
// Now supports library-level man pages by accepting library context
func parseManPageLocal(functionName string, libraryAlias string, libraryPath string) (string, error) {
    // Extract library base name from path
    libraryBaseName := extractLibraryBaseName(libraryPath)

    // Try man pages in order of specificity
    manPageCandidates := []string{
        functionName,    // e.g., "png_set_text"
        libraryAlias,    // e.g., "png"
        libraryBaseName, // e.g., "libpng"
    }

    // Remove duplicates
    seen := make(map[string]bool)
    var uniqueCandidates []string
    for _, candidate := range manPageCandidates {
        if candidate != "" && !seen[candidate] {
            seen[candidate] = true
            uniqueCandidates = append(uniqueCandidates, candidate)
        }
    }

    // Try each candidate
    for _, candidate := range uniqueCandidates {
        sigStr, err := tryManPage(candidate, functionName)
        if err == nil {
            return sigStr, nil
        }
    }

    return "", fmt.Errorf("man page not found for function '%s'", functionName)
}

// parseCFunctionSignature parses a C function signature string
// Example input: "size_t strlen(const char *s)"
func parseCFunctionSignature(sigStr string, functionName string, alias string) (*FunctionSignature, error) {
    // Clean up the signature
    sigStr = strings.TrimSpace(sigStr)
    sigStr = strings.TrimSuffix(sigStr, ";")

    // Split on '(' to separate return type + function name from parameters
    parts := strings.SplitN(sigStr, "(", 2)
    if len(parts) != 2 {
        return nil, fmt.Errorf("invalid signature format: missing '('")
    }

    leftPart := strings.TrimSpace(parts[0])
    paramsPart := strings.TrimSuffix(strings.TrimSpace(parts[1]), ")")

    // Extract return type (everything before function name)
    // Function name is the last word in leftPart, but may have * attached
    leftWords := strings.Fields(leftPart)
    if len(leftWords) == 0 {
        return nil, fmt.Errorf("invalid signature: no return type")
    }

    // Last word might be "*functionName" or "functionName"
    lastWordInLeft := leftWords[len(leftWords)-1]
    var returnTypeStr string

    // If last word starts with *, it means pointer return type
    if strings.HasPrefix(lastWordInLeft, "*") {
        // e.g., "void *malloc" -> leftWords = ["void", "*malloc"]
        // Return type should be "void *"
        if len(leftWords) == 1 {
            // Just "*functionName", return type is pointer (void *)
            returnTypeStr = "void *"
        } else {
            // Combine all but last word, then add " *"
            returnTypeStr = strings.Join(leftWords[:len(leftWords)-1], " ") + " *"
        }
    } else if len(leftWords) == 1 {
        // Just function name, assume void return
        returnTypeStr = "void"
    } else {
        // Normal case: return type is all words before function name
        returnTypeStr = strings.Join(leftWords[:len(leftWords)-1], " ")
    }

    // Parse return type
    returnType, returnStructName, err := mapCTypeStringToZa(returnTypeStr, alias)
    if err != nil {
        return nil, fmt.Errorf("failed to parse return type '%s': %w", returnTypeStr, err)
    }

    // Parse parameters
    var parameters []CParameter
    isVariadic := false

    if paramsPart != "" && paramsPart != "void" {
        // Split parameters by comma (but be careful of commas inside nested types)
        paramStrs := strings.Split(paramsPart, ",")

        for i, paramStr := range paramStrs {
            paramStr = strings.TrimSpace(paramStr)

            // Check for variadic (...
            if paramStr == "..." || strings.HasPrefix(paramStr, "...") {
                isVariadic = true
                break
            }

            // Parse parameter: "type name" or just "type"
            // Handle cases like: "const char *s", "int *p", "char *", "void*ptr"
            paramWords := strings.Fields(paramStr)
            if len(paramWords) == 0 {
                continue
            }

            var paramType string
            var paramName string

            // Check if last word is like "*s" (pointer with name)
            lastWord := paramWords[len(paramWords)-1]
            if strings.HasPrefix(lastWord, "*") && len(lastWord) > 1 && !strings.HasPrefix(lastWord, "**") {
                // Last word is "*name" - split into pointer marker and name
                paramName = lastWord[1:] // Remove leading *
                paramType = strings.Join(paramWords[:len(paramWords)-1], " ") + " *"
            } else if len(paramWords) > 1 && !strings.Contains(lastWord, "*") && !strings.Contains(lastWord, "[") {
                // Last word is a plain identifier (parameter name)
                paramName = lastWord
                paramType = strings.Join(paramWords[:len(paramWords)-1], " ")
            } else {
                // No parameter name, or complex pointer syntax
                paramType = paramStr
                paramName = fmt.Sprintf("arg%d", i)
            }

            // Map C type to Za type
            zaType, structTypeName, err := mapCTypeStringToZa(paramType, alias)
            if err != nil {
                zaType = CPointer // Default to pointer if unknown
            }

            parameters = append(parameters, CParameter{
                Name:           paramName,
                Type:           zaType,
                StructTypeName: structTypeName,
            })
        }
    }

    // Apply function name heuristics to correct likely-wrong return types
    // This helps when man pages have incorrect/simplified type signatures
    lowerFuncName := strings.ToLower(functionName)

    // Functions with these patterns in their names likely return strings
    stringReturnPatterns := []string{
        "_ver",        // png_get_libpng_ver, SDL_GetVersion, etc.
        "_version",    // get_version, sqlite3_version, etc.
        "version",     // getversion, libversion, etc.
        "_string",     // to_string, get_string, etc.
        "tostring",    // json_to_string, etc.
        "_name",       // get_name, file_name, etc.
        "getname",     // getname, getName, etc.
        "_path",       // get_path, file_path, etc.
        "getpath",     // getpath, getPath, etc.
        "_error",      // get_error, error_string, etc.
        "geterror",    // geterror, getError, etc.
        "strerror",    // strerror, etc.
        "_message",    // get_message, error_message, etc.
        "getmessage",  // getmessage, getMessage, etc.
    }

    // Check if function name matches string return patterns
    // and if current return type is a non-string pointer/int
    shouldBeString := false
    for _, pattern := range stringReturnPatterns {
        if strings.Contains(lowerFuncName, pattern) {
            // Only override if it's currently not a string
            if returnType != CString && (returnType == CPointer || returnType == CInt || returnType == CChar) {
                shouldBeString = true
                break
            }
        }
    }

    if shouldBeString {
        returnType = CString
    }

    return &FunctionSignature{
        ReturnType:       returnType,
        ReturnStructName: returnStructName,
        Parameters:       parameters,
        IsVariadic:       isVariadic,
        RawSignature:     sigStr,
    }, nil
}

// cTypeToLIBString converts CType to the string used in LIB declarations
func cTypeToLIBString(cType CType) string {
    switch cType {
    case CVoid:
        return "void"
    case CInt:
        return "int"
    case CUInt:
        return "uint"
    case CInt16:
        return "int16"
    case CUInt16:
        return "uint16"
    case CInt64:
        return "int64"
    case CUInt64:
        return "uint64"
    case CFloat:
        return "float"
    case CDouble:
        return "double"
    case CLongDouble:
        return "longdouble"
    case CInt8:
        return "int8"
    case CUInt8:
        return "uint8"
    case CChar:
        return "char"
    case CString:
        return "string" // LIB uses "string", not "char*"
    case CBool:
        return "bool"
    case CPointer:
        return "pointer" // LIB uses "pointer", not "void*"
    case CStruct:
        return "struct"
    default:
        return "unknown"
    }
}

// generateLIBDeclaration creates a ready-to-use LIB statement from a function signature
func generateLIBDeclaration(libraryAlias, functionName string, sig *FunctionSignature) string {
    // Build parameter list
    var paramStrs []string
    for _, param := range sig.Parameters {
        typeStr := cTypeToLIBString(param.Type)
        paramStrs = append(paramStrs, fmt.Sprintf("%s:%s", param.Name, typeStr))
    }

    // Add varargs if present
    if sig.IsVariadic {
        paramStrs = append(paramStrs, "...args")
    }

    paramsStr := strings.Join(paramStrs, ", ")
    returnTypeStr := cTypeToLIBString(sig.ReturnType)

    return fmt.Sprintf("LIB %s::%s(%s) -> %s", libraryAlias, functionName, paramsStr, returnTypeStr)
}

// lookupFunctionSignature tries to find a function signature via man pages or online
// Returns the parsed signature or an error if not found
// Now accepts library context to enable library-level man page searches
func lookupFunctionSignature(functionName string, lib *CLibrary) (*FunctionSignature, error) {
    var libraryAlias, libraryPath string

    // Extract library info if provided
    if lib != nil {
        libraryAlias = lib.Alias
        libraryPath = lib.Name
    }

    // Try local man page first (with library context if available)
    var sigStr string
    var err error

    if lib != nil {
        // Library context available - try multi-stage lookup
        sigStr, err = parseManPageLocal(functionName, libraryAlias, libraryPath)
    } else {
        // No library context - try function-specific man page only
        sigStr, err = tryManPage(functionName, functionName)
    }

    if err == nil {
        // Successfully got signature from man page, parse it
        sig, parseErr := parseCFunctionSignature(sigStr, functionName, "")
        if parseErr == nil {
            return sig, nil
        }
        // Fall through to online if parsing failed
    }

    // Try online fallback via man7.org
    sigStr, err = parseManPageOnline(functionName)
    if err == nil {
        // Successfully got signature from online, parse it
        sig, parseErr := parseCFunctionSignature(sigStr, functionName, "")
        if parseErr == nil {
            return sig, nil
        }
    }

    return nil, fmt.Errorf("could not find signature for function '%s'", functionName)
}
// getPlatformManPageURL returns the appropriate online man page URL for the current platform
func getPlatformManPageURL(functionName string) string {
    switch runtime.GOOS {
    case "freebsd":
        // FreeBSD man pages
        return fmt.Sprintf("https://man.freebsd.org/cgi/man.cgi?query=%s&sektion=3", functionName)
    case "openbsd":
        // OpenBSD man pages
        return fmt.Sprintf("https://man.openbsd.org/%s.3", functionName)
    case "netbsd":
        // NetBSD man pages
        return fmt.Sprintf("https://man.netbsd.org/%s.3", functionName)
    case "dragonfly":
        // DragonFly BSD uses FreeBSD man pages as fallback
        return fmt.Sprintf("https://man.dragonflybsd.org/?command=%s&section=3", functionName)
    case "linux":
        fallthrough
    default:
        // Linux (default) - man7.org
        return fmt.Sprintf("https://man7.org/linux/man-pages/man3/%s.3.html", functionName)
    }
}

// parseManPageOnline fetches function signature from platform-specific man page URL
func parseManPageOnline(functionName string) (string, error) {
    url := getPlatformManPageURL(functionName)

    // Fetch the HTML page
    resp, err := web_client.Get(url)
    if err != nil {
        return "", fmt.Errorf("failed to fetch man page from %s: %w", url, err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != 200 {
        return "", fmt.Errorf("man page not found online (HTTP %d)", resp.StatusCode)
    }

    // Read response body
    bodyBytes, err := ioutil.ReadAll(resp.Body)
    if err != nil {
        return "", fmt.Errorf("failed to read response body: %w", err)
    }

    html := string(bodyBytes)

    // Find <pre> tags which contain the SYNOPSIS
    preRegex := regexp.MustCompile(`(?s)<pre>(.*?)</pre>`)
    matches := preRegex.FindAllStringSubmatch(html, -1)

    if len(matches) == 0 {
        return "", fmt.Errorf("no <pre> sections found in HTML")
    }

    // Look through <pre> sections for the function signature
    for _, match := range matches {
        preContent := match[1]

        // Strip HTML tags but keep text
        tagRegex := regexp.MustCompile(`<[^>]+>`)
        cleanContent := tagRegex.ReplaceAllString(preContent, "")

        // Decode HTML entities
        cleanContent = strings.ReplaceAll(cleanContent, "&lt;", "<")
        cleanContent = strings.ReplaceAll(cleanContent, "&gt;", ">")
        cleanContent = strings.ReplaceAll(cleanContent, "&amp;", "&")
        cleanContent = strings.ReplaceAll(cleanContent, "&#160;", " ")

        // Skip if this looks like a header (contains "Library Functions Manual" etc.)
        if strings.Contains(cleanContent, "Library Functions Manual") ||
            strings.Contains(cleanContent, "Linux Programmer") {
            continue
        }

        // Look for #include to identify SYNOPSIS section
        if !strings.Contains(cleanContent, "#include") {
            continue
        }

        // Extract lines containing the function signature
        lines := strings.Split(cleanContent, "\n")
        var signatureLines []string
        foundMatch := false
        inSignature := false

        for _, line := range lines {
            trimmedLine := strings.TrimSpace(line)

            // Skip #include lines
            if strings.Contains(trimmedLine, "#include") {
                continue
            }

            // Skip empty lines when not in signature
            if !inSignature && trimmedLine == "" {
                continue
            }

            // Look for function name with opening parenthesis (and not in a comment/header)
            if !inSignature && strings.Contains(trimmedLine, functionName) && strings.Contains(trimmedLine, "(") {
                // Make sure it's not a header line
                if !strings.Contains(trimmedLine, "Manual") && !strings.Contains(trimmedLine, "(3)") {
                    // Check if this line starts with the function name (not just contains it)
                    // This helps us get "int printf(...)" and not "int fprintf(...)"
                    if strings.HasPrefix(trimmedLine, functionName+"(") ||
                        strings.Contains(trimmedLine, " "+functionName+"(") ||
                        strings.Contains(trimmedLine, "\t"+functionName+"(") ||
                        strings.Contains(trimmedLine, "*"+functionName+"(") {
                        signatureLines = append(signatureLines, trimmedLine)
                        foundMatch = true
                        inSignature = true
                    }
                }
            } else if inSignature {
                // Continue collecting lines that are part of the signature
                if trimmedLine != "" {
                    signatureLines = append(signatureLines, trimmedLine)
                }
            }

            // Check if signature is complete
            if inSignature && len(signatureLines) > 0 {
                joined := strings.Join(signatureLines, " ")
                openCount := strings.Count(joined, "(")
                closeCount := strings.Count(joined, ")")

                // Signature is complete when parentheses are balanced
                if openCount > 0 && closeCount >= openCount {
                    // Check if this looks complete
                    if strings.HasSuffix(strings.TrimSpace(joined), ")") ||
                       strings.HasSuffix(strings.TrimSpace(joined), ");") {
                        break
                    }
                }
            }
        }

        if foundMatch && len(signatureLines) > 0 {
            signature := strings.Join(signatureLines, " ")
            signature = strings.TrimSuffix(signature, ";")
            signature = strings.TrimSpace(signature)
            return signature, nil
        }
    }

    return "", fmt.Errorf("function signature not found in online man page")
}

// Global registry for FFI struct definitions (separate from Za's structmaps)
var ffiStructDefinitions = make(map[string]*CLibraryStruct)
var ffiStructLock sync.RWMutex

// lookupStructDefinition finds a struct definition by name
func lookupStructDefinition(name string) *CLibraryStruct {
    ffiStructLock.RLock()
    defer ffiStructLock.RUnlock()
    return ffiStructDefinitions[name]
}

// getStructLayoutFromZa converts a Za struct definition to a C struct layout
// It queries the global structmaps and calculates C-style field offsets
func getStructLayoutFromZa(structName string) (*CLibraryStruct, error) {
    // Check if already cached
    if cached := lookupStructDefinition(structName); cached != nil {
        return cached, nil
    }

    // Look up struct definition in Za's structmaps
    // Try with and without namespace prefixes
    structmapslock.RLock()
    structDef, found := structmaps[structName]

    if !found {
        // Try common namespace prefixes
        namespaces := []string{"main::", "global::", ""}
        for _, ns := range namespaces {
            qualifiedName := ns + structName
            if def, ok := structmaps[qualifiedName]; ok {
                structDef = def
                found = true
                break
            }
        }
    }

    if !found {
        // Search all keys for a match (handle cases where user provides full name or just base name)
        for k, v := range structmaps {
            // If structName has ::, do exact match; otherwise match the suffix
            if strings.Contains(structName, "::") {
                if k == structName {
                    structDef = v
                    found = true
                    break
                }
            } else {
                // Match suffix after ::
                if strings.HasSuffix(k, "::"+structName) || k == structName {
                    structDef = v
                    found = true
                    break
                }
            }
        }
    }
    structmapslock.RUnlock()

    if !found {
        return nil, fmt.Errorf("struct %s not defined", structName)
    }

    // structDef format: [fieldName1, fieldType1, hasDefault1, defaultVal1, fieldName2, fieldType2, ...]
    if len(structDef)%4 != 0 {
        return nil, fmt.Errorf("invalid struct definition for %s", structName)
    }

    var fields []StructField
    var currentOffset uintptr = 0

    // Process each field
    for i := 0; i < len(structDef); i += 4 {
        fieldName, ok := structDef[i].(string)
        if !ok {
            return nil, fmt.Errorf("invalid field name in struct %s", structName)
        }

        fieldTypeStr, ok := structDef[i+1].(string)
        if !ok {
            return nil, fmt.Errorf("invalid field type in struct %s", structName)
        }

        // Convert Za type string to CType
        var fieldCType CType
        var fieldSize uintptr
        var arraySize int = 0
        var elementType CType

        // Check for array syntax: type[size]
        if strings.Contains(fieldTypeStr, "[") && strings.HasSuffix(fieldTypeStr, "]") {
            openBracket := strings.Index(fieldTypeStr, "[")
            closeBracket := strings.LastIndex(fieldTypeStr, "]")

            if openBracket > 0 && closeBracket > openBracket {
                elementTypeStr := strings.TrimSpace(fieldTypeStr[:openBracket])
                arraySizeStr := strings.TrimSpace(fieldTypeStr[openBracket+1 : closeBracket])

                // Parse array size
                size, err := strconv.Atoi(arraySizeStr)
                if err != nil || size <= 0 {
                    return nil, fmt.Errorf("invalid array size '%s' for field %s", arraySizeStr, fieldName)
                }
                arraySize = size

                // Parse element type
                var elemSize uintptr
                switch strings.ToLower(elementTypeStr) {
                case "int", "uint":
                    elementType = CInt
                    elemSize = 4
                case "int8":
                    elementType = CInt8
                    elemSize = 1
                case "uint8", "byte":
                    elementType = CUInt8
                    elemSize = 1
                case "int16":
                    elementType = CInt16
                    elemSize = 2
                case "uint16":
                    elementType = CUInt16
                    elemSize = 2
                case "int64":
                    elementType = CInt64
                    elemSize = 8
                case "uint64":
                    elementType = CUInt64
                    elemSize = 8
                case "float":
                    elementType = CFloat
                    elemSize = 4
                case "double":
                    elementType = CDouble
                    elemSize = 8
                case "char":
                    elementType = CChar
                    elemSize = 1
                default:
                    return nil, fmt.Errorf("unsupported array element type '%s' for field %s", elementTypeStr, fieldName)
                }

                // For arrays, we use the element type as the field type
                // The marshaling code will use ArraySize to handle the array
                fieldCType = elementType
                fieldSize = elemSize * uintptr(arraySize)

                // Add this field and skip the normal type parsing
                fields = append(fields, StructField{
                    Name:        fieldName,
                    Type:        fieldCType,
                    Offset:      currentOffset,
                    ArraySize:   arraySize,
                    ElementType: elementType,
                })

                currentOffset += fieldSize
                continue
            }
        }

        switch strings.ToLower(fieldTypeStr) {
        case "int", "uint":
            fieldCType = CInt
            fieldSize = 4 // C int is 32-bit
        case "float":
            fieldCType = CDouble // Za float is float64
            fieldSize = 8
        case "double":
            fieldCType = CDouble
            fieldSize = 8
        case "string":
            fieldCType = CString // char* pointer
            fieldSize = uintptr(unsafe.Sizeof(uintptr(0)))
        case "bool":
            fieldCType = CBool
            fieldSize = 1
        case "pointer", "ptr":
            fieldCType = CPointer
            fieldSize = uintptr(unsafe.Sizeof(uintptr(0)))
        case "any", "interface{}":
            fieldCType = CPointer // Treat as opaque pointer
            fieldSize = uintptr(unsafe.Sizeof(uintptr(0)))
        default:
            // Check if it's a struct type
            if strings.HasPrefix(fieldTypeStr, "struct<") || structmaps[fieldTypeStr] != nil {
                fieldCType = CStruct // Nested struct as pointer
                fieldSize = uintptr(unsafe.Sizeof(uintptr(0)))
            } else {
                // Unknown type - treat as pointer
                fieldCType = CPointer
                fieldSize = uintptr(unsafe.Sizeof(uintptr(0)))
            }
        }

        fields = append(fields, StructField{
            Name:   fieldName,
            Type:   fieldCType,
            Offset: currentOffset,
        })

        currentOffset += fieldSize
    }

    // Create and cache the C struct layout
    cStruct := &CLibraryStruct{
        Name:   structName,
        Fields: fields,
        Size:   currentOffset,
    }

    ffiStructLock.Lock()
    ffiStructDefinitions[structName] = cStruct
    ffiStructLock.Unlock()

    return cStruct, nil
}
