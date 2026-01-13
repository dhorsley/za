package main

import (
    "fmt"
    "io/ioutil"
    "os/exec"
    "path/filepath"
    "plugin"
    "regexp"
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
    ParamTypes    []CType  // Types of fixed parameters
    ReturnType    CType    // Return type
    HasVarargs    bool     // True if function is variadic (takes variable arguments)
    FixedArgCount int      // Number of fixed arguments before varargs
}

// Global registry of declared function signatures
// Map structure: libraryAlias -> functionName -> signature
var declaredSignatures = make(map[string]map[string]CFunctionSignature)

// DeclareCFunction stores an explicit function signature declaration
func DeclareCFunction(libraryAlias, functionName string, paramTypes []CType, returnType CType, hasVarargs bool) {
    if declaredSignatures[libraryAlias] == nil {
        declaredSignatures[libraryAlias] = make(map[string]CFunctionSignature)
    }
    fixedArgCount := len(paramTypes)
    declaredSignatures[libraryAlias][functionName] = CFunctionSignature{
        ParamTypes:    paramTypes,
        ReturnType:    returnType,
        HasVarargs:    hasVarargs,
        FixedArgCount: fixedArgCount,
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

// StringToCType converts type name strings to CType enum (for LIB declarations)
func StringToCType(typeName string) (CType, error) {
    switch strings.ToLower(typeName) {
    case "void":
        return CVoid, nil
    case "int":
        return CInt, nil
    case "float":
        return CFloat, nil
    case "double":
        return CDouble, nil
    case "char":
        return CChar, nil
    case "string":
        return CString, nil
    case "bool":
        return CBool, nil
    case "pointer", "ptr":
        return CPointer, nil
    default:
        return CVoid, fmt.Errorf("unknown type name: %s", typeName)
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
    categories["ffi"] = []string{"c_null", "c_fopen", "c_fclose", "c_ptr_is_null", "c_ptr_to_int", "c_alloc", "c_free", "c_set_byte", "c_get_symbol"}

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

    slhelp["c_ptr_to_int"] = LibHelp{in: "ptr", out: "int", action: "Converts a C pointer to an integer. Useful for size_t values returned as pointers."}
    stdlib["c_ptr_to_int"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("c_ptr_to_int", args, 1, "1", "any"); !ok {
            return nil, err
        }
        if p, ok := args[0].(*CPointerValue); ok {
            return int(uintptr(p.Ptr)), nil
        }
        return nil, fmt.Errorf("c_ptr_to_int: argument is not a C pointer")
    }

    slhelp["c_get_symbol"] = LibHelp{in: "library_alias,symbol_name", out: "any", action: "Reads a data symbol (constant/variable) from a loaded C library. Note: C preprocessor #defines are NOT symbols and cannot be read this way."}
    stdlib["c_get_symbol"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("c_get_symbol", args, 1, "2", "string", "string"); !ok {
            return nil, err
        }
        return CGetDataSymbol(args[0].(string), args[1].(string))
    }
}
// FunctionSignature represents a parsed C function signature from man pages
type FunctionSignature struct {
    ReturnType   CType
    Parameters   []CParameter
    IsVariadic   bool
    RawSignature string // Original C signature
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

// mapCTypeStringToZa converts a C type string to Za CType enum
// Handles common C types including modifiers and pointers
func mapCTypeStringToZa(cTypeStr string) (CType, string, error) {
    // Remove leading/trailing whitespace
    cTypeStr = strings.TrimSpace(cTypeStr)

    // Split on * to separate pointers from types and modifiers
    // e.g., "char *restrict" -> handle separately
    parts := strings.Split(cTypeStr, "*")
    isPointer := len(parts) > 1

    // Clean up the base type (everything before *)
    baseTypePart := strings.TrimSpace(parts[0])

    // Remove modifiers from base type
    modifiers := []string{"const", "volatile", "static", "inline", "restrict", "unsigned", "signed"}
    for _, mod := range modifiers {
        baseTypePart = strings.Replace(baseTypePart, mod+" ", "", -1)
        baseTypePart = strings.Replace(baseTypePart, mod, "", -1)
    }
    baseType := strings.TrimSpace(baseTypePart)

    // Map C types to Za types
    switch baseType {
    case "void":
        if isPointer {
            return CPointer, "", nil // void* → pointer
        }
        return CVoid, "", nil

    case "char":
        if isPointer {
            return CString, "char* mapped to string", nil // char* → string
        }
        return CChar, "", nil

    case "int", "long", "short", "int8_t", "int16_t", "int32_t", "int64_t",
        "uint8_t", "uint16_t", "uint32_t", "uint64_t",
        "size_t", "ssize_t", "off_t", "pid_t", "uid_t", "gid_t", "mode_t", "time_t":
        if baseType == "size_t" || baseType == "ssize_t" {
            return CInt, baseType + " mapped to int", nil
        }
        return CInt, "", nil

    case "float":
        return CFloat, "", nil

    case "double":
        return CDouble, "", nil

    case "bool", "_Bool":
        return CBool, "", nil

    default:
        // Unknown types or struct/union pointers default to pointer
        if isPointer {
            return CPointer, baseType + "* mapped to pointer", nil
        }
        return CPointer, "unknown type '" + baseType + "' mapped to pointer", nil
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
        if strings.Contains(trimmed, functionName+"(") ||
            strings.Contains(trimmed, functionName+" (") {
            inFunction = true
            signatureLines = append(signatureLines, trimmed)

            // Check if signature ends on same line (with semicolon or closing paren)
            if strings.Contains(trimmed, ");") {
                break
            }
        } else if inFunction {
            // Continue collecting multi-line signature
            signatureLines = append(signatureLines, trimmed)
            if strings.Contains(trimmed, ");") {
                break
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
func parseCFunctionSignature(sigStr string, functionName string) (*FunctionSignature, error) {
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
    returnType, _, err := mapCTypeStringToZa(returnTypeStr)
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
            zaType, _, err := mapCTypeStringToZa(paramType)
            if err != nil {
                zaType = CPointer // Default to pointer if unknown
            }

            parameters = append(parameters, CParameter{
                Name: paramName,
                Type: zaType,
            })
        }
    }

    return &FunctionSignature{
        ReturnType:   returnType,
        Parameters:   parameters,
        IsVariadic:   isVariadic,
        RawSignature: sigStr,
    }, nil
}

// cTypeToLIBString converts CType to the string used in LIB declarations
func cTypeToLIBString(cType CType) string {
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
        sig, parseErr := parseCFunctionSignature(sigStr, functionName)
        if parseErr == nil {
            return sig, nil
        }
        // Fall through to online if parsing failed
    }

    // Try online fallback via man7.org
    sigStr, err = parseManPageOnline(functionName)
    if err == nil {
        // Successfully got signature from online, parse it
        sig, parseErr := parseCFunctionSignature(sigStr, functionName)
        if parseErr == nil {
            return sig, nil
        }
    }

    return nil, fmt.Errorf("could not find signature for function '%s'", functionName)
}
// parseManPageOnline fetches function signature from man7.org
func parseManPageOnline(functionName string) (string, error) {
    url := fmt.Sprintf("https://man7.org/linux/man-pages/man3/%s.3.html", functionName)

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

        for _, line := range lines {
            trimmedLine := strings.TrimSpace(line)

            // Skip #include lines
            if strings.Contains(trimmedLine, "#include") {
                continue
            }

            // Skip empty lines
            if trimmedLine == "" {
                continue
            }

            // Look for function name with opening parenthesis (and not in a comment/header)
            if strings.Contains(trimmedLine, functionName) && strings.Contains(trimmedLine, "(") {
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
                        break // Get only the first matching signature
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
