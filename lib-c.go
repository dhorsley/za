package main

import (
    "fmt"
    "plugin"
    "strconv"
    "strings"
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
    Name string
    Type CType
}

// StructField represents a field in a C struct
type StructField struct {
    Name   string
    Type   CType
    Offset uintptr
}

// CLibraryStruct represents a C struct definition
type CLibraryStruct struct {
    Name   string
    Fields []StructField
    Size   uintptr
}

// CLibrary represents a loaded C library
type CLibrary struct {
    Name    string
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

// CPointerValue represents an opaque C pointer that can be passed between FFI calls
type CPointerValue struct {
    Ptr     unsafe.Pointer
    TypeTag string // Optional type hint (e.g., "png_structp", "FILE*")
}

// IsNull returns true if the pointer is null
func (p *CPointerValue) IsNull() bool {
    return p == nil || p.Ptr == nil
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
    default:
        return CVoid, []string{fmt.Sprintf("[UNSUPPORTED: Cannot convert Za type %d to C type]", zaType)}
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

// CTypeToString converts C type enum to string
func CTypeToString(cType CType) string {
    switch cType {
    case CVoid:
        return "void"
    case CInt:
        return "int"
    case CFloat:
        return "float"
    case CDouble:
        return "double"
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
    categories["ffi"] = []string{"c_null", "c_fopen", "c_fclose", "c_ptr_is_null", "c_alloc", "c_free", "c_set_byte", "c_get_symbol"}

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
        return true, nil
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

    slhelp["c_get_symbol"] = LibHelp{in: "library_alias,symbol_name", out: "any", action: "Reads a data symbol (constant/variable) from a loaded C library. Note: C preprocessor #defines are NOT symbols and cannot be read this way."}
    stdlib["c_get_symbol"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("c_get_symbol", args, 1, "2", "string", "string"); !ok {
            return nil, err
        }
        return CGetDataSymbol(args[0].(string), args[1].(string))
    }
}
