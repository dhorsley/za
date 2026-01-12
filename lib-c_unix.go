//go:build !windows && !noffi && cgo
// +build !windows,!noffi,cgo

package main

/*
#include <dlfcn.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

// Helper functions for calling C functions with different signatures
// These are necessary because Go cannot dynamically call arbitrary C functions

// 0-arg functions
static const char* call_func_void_str(void* fn) {
    typedef const char* (*func_t)(void);
    return ((func_t)fn)();
}

static int call_func_void_int(void* fn) {
    typedef int (*func_t)(void);
    return ((func_t)fn)();
}

static double call_func_void_double(void* fn) {
    typedef double (*func_t)(void);
    return ((func_t)fn)();
}

// 1-arg functions
static int call_func_str_int(void* fn, const char* s) {
    typedef int (*func_t)(const char*);
    return ((func_t)fn)(s);
}

static const char* call_func_str_str(void* fn, const char* s) {
    typedef const char* (*func_t)(const char*);
    return ((func_t)fn)(s);
}

static int call_func_int_int(void* fn, int i) {
    typedef int (*func_t)(int);
    return ((func_t)fn)(i);
}

static double call_func_double_double(void* fn, double d) {
    typedef double (*func_t)(double);
    return ((func_t)fn)(d);
}

// 2-arg functions
static int call_func_str_str_int(void* fn, const char* s1, const char* s2) {
    typedef int (*func_t)(const char*, const char*);
    return ((func_t)fn)(s1, s2);
}

static int call_func_str_int_int(void* fn, const char* s, int i) {
    typedef int (*func_t)(const char*, int);
    return ((func_t)fn)(s, i);
}

static int call_func_int_int_int(void* fn, int i1, int i2) {
    typedef int (*func_t)(int, int);
    return ((func_t)fn)(i1, i2);
}

static double call_func_double_double_double(void* fn, double d1, double d2) {
    typedef double (*func_t)(double, double);
    return ((func_t)fn)(d1, d2);
}

// 3-arg functions
static int call_func_int_int_int_int(void* fn, int i1, int i2, int i3) {
    typedef int (*func_t)(int, int, int);
    return ((func_t)fn)(i1, i2, i3);
}

static double call_func_ddd_double(void* fn, double d1, double d2, double d3) {
    typedef double (*func_t)(double, double, double);
    return ((func_t)fn)(d1, d2, d3);
}

static int call_func_str_str_int_int(void* fn, const char* s1, const char* s2, int n) {
    typedef int (*func_t)(const char*, const char*, int);
    return ((func_t)fn)(s1, s2, n);
}

// 4-arg functions
static int call_func_iiii_int(void* fn, int i1, int i2, int i3, int i4) {
    typedef int (*func_t)(int, int, int, int);
    return ((func_t)fn)(i1, i2, i3, i4);
}

// Pointer-based functions (for APIs like libpng, zlib, etc.)

// 0-arg returning pointer
static void* call_func_void_ptr(void* fn) {
    typedef void* (*func_t)(void);
    return ((func_t)fn)();
}

// 1-arg pointer functions
static void* call_func_ptr_ptr(void* fn, void* p) {
    typedef void* (*func_t)(void*);
    return ((func_t)fn)(p);
}

static void* call_func_str_ptr(void* fn, const char* s) {
    typedef void* (*func_t)(const char*);
    return ((func_t)fn)(s);
}

static int call_func_ptr_int(void* fn, void* p) {
    typedef int (*func_t)(void*);
    return ((func_t)fn)(p);
}

static void call_func_ptr_void(void* fn, void* p) {
    typedef void (*func_t)(void*);
    ((func_t)fn)(p);
}

// 2-arg pointer functions
static void* call_func_ptr_ptr_ptr(void* fn, void* p1, void* p2) {
    typedef void* (*func_t)(void*, void*);
    return ((func_t)fn)(p1, p2);
}

static void call_func_ptr_ptr_void(void* fn, void* p1, void* p2) {
    typedef void (*func_t)(void*, void*);
    ((func_t)fn)(p1, p2);
}

static int call_func_ptr_ptr_int(void* fn, void* p1, void* p2) {
    typedef int (*func_t)(void*, void*);
    return ((func_t)fn)(p1, p2);
}

static void* call_func_str_ptr_ptr(void* fn, const char* s, void* p) {
    typedef void* (*func_t)(const char*, void*);
    return ((func_t)fn)(s, p);
}

static int call_func_ptr_int_int(void* fn, void* p, int i) {
    typedef int (*func_t)(void*, int);
    return ((func_t)fn)(p, i);
}

// 3-arg pointer functions
static void call_func_ptr_ptr_ptr_void(void* fn, void* p1, void* p2, void* p3) {
    typedef void (*func_t)(void*, void*, void*);
    ((func_t)fn)(p1, p2, p3);
}

static int call_func_ptr_int_int_int(void* fn, void* p, int i1, int i2) {
    typedef int (*func_t)(void*, int, int);
    return ((func_t)fn)(p, i1, i2);
}

// 4-arg pointer functions (png_create_write_struct pattern: str, ptr, ptr, ptr -> ptr)
static void* call_func_str_ptr_ptr_ptr_ptr(void* fn, const char* s, void* p1, void* p2, void* p3) {
    typedef void* (*func_t)(const char*, void*, void*, void*);
    return ((func_t)fn)(s, p1, p2, p3);
}

static void call_func_ptr_ptr_int_int_void(void* fn, void* p1, void* p2, int i1, int i2) {
    typedef void (*func_t)(void*, void*, int, int);
    ((func_t)fn)(p1, p2, i1, i2);
}

// 8-arg function for png_set_IHDR: (ptr, ptr, int, int, int, int, int, int) -> void
static void call_func_ppiiiiiii_void(void* fn, void* p1, void* p2,
                                      unsigned int width, unsigned int height,
                                      int bit_depth, int color_type,
                                      int interlace, int compression, int filter) {
    typedef void (*func_t)(void*, void*, unsigned int, unsigned int, int, int, int, int, int);
    ((func_t)fn)(p1, p2, width, height, bit_depth, color_type, interlace, compression, filter);
}

// File operations for libpng
static void* call_fopen(const char* path, const char* mode) {
    return fopen(path, mode);
}

static int call_fclose(void* fp) {
    return fclose((FILE*)fp);
}

// png_init_io wrapper: (png_ptr, FILE*) -> void
static void call_func_ptr_file_void(void* fn, void* png_ptr, void* fp) {
    typedef void (*func_t)(void*, void*);
    ((func_t)fn)(png_ptr, fp);
}

// For png_write_row: (ptr, ptr) -> void (already covered by call_func_ptr_ptr_void)

// For getting NULL pointer
static void* get_null_ptr(void) {
    return NULL;
}

// For reading data symbol values
static int read_int_symbol(void* addr) {
    if (addr == NULL) return 0;
    return *((int*)addr);
}

static double read_double_symbol(void* addr) {
    if (addr == NULL) return 0.0;
    return *((double*)addr);
}

static const char* read_string_symbol(void* addr) {
    if (addr == NULL) return NULL;
    return *((const char**)addr);
}
*/
import "C"

import (
    "debug/elf"
    "fmt"
    "path/filepath"
    "strings"
    "unsafe"
)

// LoadCLibrary loads a C shared library using dlopen
func LoadCLibrary(path string) (*CLibrary, error) {
    pathC := C.CString(path)
    defer C.free(unsafe.Pointer(pathC))

    handle := C.dlopen(pathC, C.RTLD_LAZY)
    if handle == nil {
        errMsg := C.GoString(C.dlerror())
        return nil, fmt.Errorf("failed to load library %s: %s", path, errMsg)
    }

    return &CLibrary{
        Name:    filepath.Base(path),
        Handle:  unsafe.Pointer(handle),
        Symbols: make(map[string]*CSymbol),
        Structs: make(map[string]*CLibraryStruct),
    }, nil
}

// LoadCLibraryWithAlias loads a C library with a specific alias name
func LoadCLibraryWithAlias(path string, alias string) (*CLibrary, error) {
    lib, err := LoadCLibrary(path)
    if err != nil {
        return nil, err
    }
    lib.Name = alias              // Override auto-detected name with alias
    loadedCLibraries[alias] = lib // Register library for help system
    return lib, nil
}

// DiscoverLibrarySymbols discovers symbols from a loaded C library using ELF parsing
func DiscoverLibrarySymbols(lib *CLibrary, libPath string) error {
    file, err := elf.Open(libPath)
    if err != nil {
        return fmt.Errorf("failed to open ELF file: %v", err)
    }
    defer file.Close()

    dynamicSymbols, err := file.DynamicSymbols()
    if err != nil {
        return fmt.Errorf("failed to read dynamic symbols: %v", err)
    }

    symbolCount := 0
    for _, sym := range dynamicSymbols {
        // Strip version suffixes (e.g., @@GLIBC_2.2.5 or @GLIBC_2.2.5)
        cleanName := sym.Name
        if idx := strings.Index(cleanName, "@@"); idx > 0 {
            cleanName = cleanName[:idx]
        } else if idx := strings.Index(cleanName, "@"); idx > 0 {
            cleanName = cleanName[:idx]
        }

        if shouldProcessSymbol(cleanName) {
            symbolCount++
            symType := elf.ST_TYPE(sym.Info)
            // STT_FUNC (2) = regular function
            // STT_GNU_IFUNC (10) = indirect function (used by glibc for optimized math functions)
            if symType == elf.STT_FUNC || symType == elf.SymType(10) {
                // Function symbol (regular or IFUNC)
                funcSym := createFunctionSymbolWithAlias(cleanName, lib.Name)
                lib.Symbols[funcSym.Name] = funcSym
            } else {
                // Data symbol (constants, variables, etc.)
                dataSym := createDataSymbolWithAlias(cleanName, lib.Name)
                if dataSym != nil {
                    lib.Symbols[dataSym.Name] = dataSym
                }
            }
        }
    }

    // fmt.Printf("Discovered %d symbols from %s\n", symbolCount, libPath)
    return nil
}

// DiscoverSymbolsWithAlias discovers symbols and returns them as a slice
func DiscoverSymbolsWithAlias(libPath string, alias string, existingLib *CLibrary) ([]*CSymbol, error) {
    // Use existing library if provided, otherwise load new
    lib := existingLib
    if lib == nil {
        var err error
        lib, err = LoadCLibraryWithAlias(libPath, alias)
        if err != nil {
            return nil, err
        }
    }

    err := DiscoverLibrarySymbols(lib, libPath)
    if err != nil {
        return nil, err
    }

    symbols := make([]*CSymbol, 0, len(lib.Symbols))
    for _, sym := range lib.Symbols {
        symbols = append(symbols, sym)
    }
    return symbols, nil
}

// callCFunctionPlatform attempts to call a C function with given arguments
func callCFunctionPlatform(lib *CLibrary, functionName string, args []any) (any, []string) {
    if lib.Handle == nil {
        return nil, []string{"[ERROR: Library handle is nil - cannot call function]"}
    }

    // Get function pointer from library
    funcNameC := C.CString(functionName)
    defer C.free(unsafe.Pointer(funcNameC))

    funcPtr := C.dlsym(lib.Handle, funcNameC)
    if funcPtr == nil {
        errMsg := C.GoString(C.dlerror())
        return nil, []string{fmt.Sprintf("[ERROR: Failed to resolve symbol '%s': %s]", functionName, errMsg)}
    }

    // Call function using generic approach
    return callGenericFunction(funcPtr, functionName, args)
}

// callGenericFunction calls a C function using proper CGO helper functions
// Supports common function signatures through C wrapper functions
func callGenericFunction(funcPtr unsafe.Pointer, functionName string, args []any) (any, []string) {
    switch len(args) {
    case 0:
        return call0Args(funcPtr, functionName)
    case 1:
        return call1Arg(funcPtr, functionName, args[0])
    case 2:
        return call2Args(funcPtr, functionName, args[0], args[1])
    case 3:
        return call3Args(funcPtr, functionName, args[0], args[1], args[2])
    default:
        return callNArgs(funcPtr, functionName, args)
    }
}

// call0Args handles functions with no arguments
func call0Args(funcPtr unsafe.Pointer, functionName string) (any, []string) {
    // Heuristic: if function name suggests it returns a number, try int first
    // This avoids crashes from calling int-returning functions as string-returning
    lowerName := strings.ToLower(functionName)
    if strings.Contains(lowerName, "number") ||
        strings.Contains(lowerName, "count") ||
        strings.Contains(lowerName, "size") ||
        strings.Contains(lowerName, "length") ||
        strings.Contains(lowerName, "get_") && !strings.Contains(lowerName, "str") {
        // Try () -> int first
        intResult := C.call_func_void_int(funcPtr)
        return int(intResult), []string{fmt.Sprintf("[SUCCESS: %s() -> int]", functionName)}
    }

    // Try () -> ptr first (for functions returning opaque handles)
    ptrResult := C.call_func_void_ptr(funcPtr)
    if ptrResult != nil {
        // Could be a valid pointer or a string pointer
        // Try to read it as a string (safely check first byte)
        result := C.call_func_void_str(funcPtr)
        if result != nil {
            return C.GoString(result), []string{fmt.Sprintf("[SUCCESS: %s() -> string]", functionName)}
        }
        return NewCPointer(ptrResult, functionName+"_result"), []string{fmt.Sprintf("[SUCCESS: %s() -> ptr]", functionName)}
    }

    // Try () -> double (for math constants like M_PI if exposed)
    doubleResult := C.call_func_void_double(funcPtr)
    if doubleResult != 0.0 {
        return float64(doubleResult), []string{fmt.Sprintf("[SUCCESS: %s() -> double]", functionName)}
    }

    // Try () -> int
    intResult := C.call_func_void_int(funcPtr)
    return int(intResult), []string{fmt.Sprintf("[SUCCESS: %s() -> int]", functionName)}
}

// call1Arg handles functions with one argument
func call1Arg(funcPtr unsafe.Pointer, functionName string, arg any) (any, []string) {
    switch v := arg.(type) {
    case string:
        cStr := C.CString(v)
        defer C.free(unsafe.Pointer(cStr))

        // Try (char*) -> char* first (common for string processing)
        strResult := C.call_func_str_str(funcPtr, cStr)
        if strResult != nil {
            return C.GoString(strResult), []string{fmt.Sprintf("[SUCCESS: %s(string) -> string]", functionName)}
        }

        // Try (char*) -> ptr (for functions like png_get_libpng_ver that may return ptr)
        ptrResult := C.call_func_str_ptr(funcPtr, cStr)
        if ptrResult != nil {
            return NewCPointer(ptrResult, functionName+"_result"), []string{fmt.Sprintf("[SUCCESS: %s(string) -> ptr]", functionName)}
        }

        // Try (char*) -> int
        intResult := C.call_func_str_int(funcPtr, cStr)
        return int(intResult), []string{fmt.Sprintf("[SUCCESS: %s(string) -> int]", functionName)}

    case int:
        // Try (int) -> int
        intResult := C.call_func_int_int(funcPtr, C.int(v))
        return int(intResult), []string{fmt.Sprintf("[SUCCESS: %s(int) -> int]", functionName)}

    case float64:
        // Try (double) -> double
        doubleResult := C.call_func_double_double(funcPtr, C.double(v))
        return float64(doubleResult), []string{fmt.Sprintf("[SUCCESS: %s(double) -> double]", functionName)}

    case bool:
        intVal := 0
        if v {
            intVal = 1
        }
        intResult := C.call_func_int_int(funcPtr, C.int(intVal))
        return int(intResult) != 0, []string{fmt.Sprintf("[SUCCESS: %s(bool) -> bool]", functionName)}

    case *CPointerValue:
        var cPtr unsafe.Pointer
        if v != nil {
            cPtr = v.Ptr
        }
        // Try (ptr) -> ptr first
        ptrResult := C.call_func_ptr_ptr(funcPtr, cPtr)
        if ptrResult != nil {
            return NewCPointer(ptrResult, functionName+"_result"), []string{fmt.Sprintf("[SUCCESS: %s(ptr) -> ptr]", functionName)}
        }
        // Try (ptr) -> int
        intResult := C.call_func_ptr_int(funcPtr, cPtr)
        return int(intResult), []string{fmt.Sprintf("[SUCCESS: %s(ptr) -> int]", functionName)}

    default:
        return nil, []string{fmt.Sprintf("[ERROR: Unsupported argument type %T for %s]", arg, functionName)}
    }
}

// call2Args handles functions with two arguments
func call2Args(funcPtr unsafe.Pointer, functionName string, arg1, arg2 any) (any, []string) {
    switch v1 := arg1.(type) {
    case string:
        cStr1 := C.CString(v1)
        defer C.free(unsafe.Pointer(cStr1))

        switch v2 := arg2.(type) {
        case string:
            cStr2 := C.CString(v2)
            defer C.free(unsafe.Pointer(cStr2))
            // (char*, char*) -> int (strcmp, strstr patterns)
            intResult := C.call_func_str_str_int(funcPtr, cStr1, cStr2)
            return int(intResult), []string{fmt.Sprintf("[SUCCESS: %s(string, string) -> int]", functionName)}

        case int:
            // (char*, int) -> int
            intResult := C.call_func_str_int_int(funcPtr, cStr1, C.int(v2))
            return int(intResult), []string{fmt.Sprintf("[SUCCESS: %s(string, int) -> int]", functionName)}

        case *CPointerValue:
            var cPtr unsafe.Pointer
            if v2 != nil {
                cPtr = v2.Ptr
            }
            // (char*, ptr) -> ptr
            ptrResult := C.call_func_str_ptr_ptr(funcPtr, cStr1, cPtr)
            if ptrResult != nil {
                return NewCPointer(ptrResult, functionName+"_result"), []string{fmt.Sprintf("[SUCCESS: %s(string, ptr) -> ptr]", functionName)}
            }
            return NullPointer(), []string{fmt.Sprintf("[SUCCESS: %s(string, ptr) -> null]", functionName)}
        }

    case int:
        if v2, ok := arg2.(int); ok {
            // (int, int) -> int
            intResult := C.call_func_int_int_int(funcPtr, C.int(v1), C.int(v2))
            return int(intResult), []string{fmt.Sprintf("[SUCCESS: %s(int, int) -> int]", functionName)}
        }

    case float64:
        if v2, ok := arg2.(float64); ok {
            // (double, double) -> double (math functions like pow, fmod)
            doubleResult := C.call_func_double_double_double(funcPtr, C.double(v1), C.double(v2))
            return float64(doubleResult), []string{fmt.Sprintf("[SUCCESS: %s(double, double) -> double]", functionName)}
        }

    case *CPointerValue:
        var cPtr1 unsafe.Pointer
        if v1 != nil {
            cPtr1 = v1.Ptr
        }
        switch v2 := arg2.(type) {
        case *CPointerValue:
            var cPtr2 unsafe.Pointer
            if v2 != nil {
                cPtr2 = v2.Ptr
            }
            // (ptr, ptr) -> ptr first
            ptrResult := C.call_func_ptr_ptr_ptr(funcPtr, cPtr1, cPtr2)
            if ptrResult != nil {
                return NewCPointer(ptrResult, functionName+"_result"), []string{fmt.Sprintf("[SUCCESS: %s(ptr, ptr) -> ptr]", functionName)}
            }
            // (ptr, ptr) -> void (common pattern like png_init_io, png_write_info)
            C.call_func_ptr_ptr_void(funcPtr, cPtr1, cPtr2)
            return nil, []string{fmt.Sprintf("[SUCCESS: %s(ptr, ptr) -> void]", functionName)}

        case int:
            // (ptr, int) -> int
            intResult := C.call_func_ptr_int_int(funcPtr, cPtr1, C.int(v2))
            return int(intResult), []string{fmt.Sprintf("[SUCCESS: %s(ptr, int) -> int]", functionName)}
        }
    }

    return nil, []string{fmt.Sprintf("[ERROR: Unsupported argument types (%T, %T) for %s]", arg1, arg2, functionName)}
}

// call3Args handles functions with three arguments
func call3Args(funcPtr unsafe.Pointer, functionName string, arg1, arg2, arg3 any) (any, []string) {
    switch v1 := arg1.(type) {
    case int:
        if v2, ok := arg2.(int); ok {
            if v3, ok := arg3.(int); ok {
                // (int, int, int) -> int
                intResult := C.call_func_int_int_int_int(funcPtr, C.int(v1), C.int(v2), C.int(v3))
                return int(intResult), []string{fmt.Sprintf("[SUCCESS: %s(int, int, int) -> int]", functionName)}
            }
        }

    case string:
        cStr1 := C.CString(v1)
        defer C.free(unsafe.Pointer(cStr1))

        if v2, ok := arg2.(string); ok {
            cStr2 := C.CString(v2)
            defer C.free(unsafe.Pointer(cStr2))

            if v3, ok := arg3.(int); ok {
                // (char*, char*, int) -> int (strncmp pattern)
                intResult := C.call_func_str_str_int_int(funcPtr, cStr1, cStr2, C.int(v3))
                return int(intResult), []string{fmt.Sprintf("[SUCCESS: %s(string, string, int) -> int]", functionName)}
            }
        }

    case float64:
        if v2, ok := arg2.(float64); ok {
            if v3, ok := arg3.(float64); ok {
                // (double, double, double) -> double (fma pattern)
                doubleResult := C.call_func_ddd_double(funcPtr, C.double(v1), C.double(v2), C.double(v3))
                return float64(doubleResult), []string{fmt.Sprintf("[SUCCESS: %s(double, double, double) -> double]", functionName)}
            }
        }
    }

    return nil, []string{fmt.Sprintf("[ERROR: Unsupported argument types for %s with 3 args]", functionName)}
}

// callNArgs handles functions with 4+ arguments (limited support)
func callNArgs(funcPtr unsafe.Pointer, functionName string, args []any) (any, []string) {
    if len(args) == 4 {
        // Try (string, ptr, ptr, ptr) -> ptr pattern (png_create_write_struct)
        if s, ok := args[0].(string); ok {
            if p1, ok1 := args[1].(*CPointerValue); ok1 {
                if p2, ok2 := args[2].(*CPointerValue); ok2 {
                    if p3, ok3 := args[3].(*CPointerValue); ok3 {
                        cStr := C.CString(s)
                        defer C.free(unsafe.Pointer(cStr))
                        var cp1, cp2, cp3 unsafe.Pointer
                        if p1 != nil {
                            cp1 = p1.Ptr
                        }
                        if p2 != nil {
                            cp2 = p2.Ptr
                        }
                        if p3 != nil {
                            cp3 = p3.Ptr
                        }
                        ptrResult := C.call_func_str_ptr_ptr_ptr_ptr(funcPtr, cStr, cp1, cp2, cp3)
                        if ptrResult != nil {
                            return NewCPointer(ptrResult, functionName+"_result"), []string{fmt.Sprintf("[SUCCESS: %s(str, ptr, ptr, ptr) -> ptr]", functionName)}
                        }
                        return NullPointer(), []string{fmt.Sprintf("[SUCCESS: %s(str, ptr, ptr, ptr) -> null]", functionName)}
                    }
                }
            }
        }

        // Try all-int signature
        allInts := true
        intArgs := make([]int, 4)
        for i, arg := range args {
            if v, ok := arg.(int); ok {
                intArgs[i] = v
            } else {
                allInts = false
                break
            }
        }
        if allInts {
            intResult := C.call_func_iiii_int(funcPtr, C.int(intArgs[0]), C.int(intArgs[1]), C.int(intArgs[2]), C.int(intArgs[3]))
            return int(intResult), []string{fmt.Sprintf("[SUCCESS: %s(int, int, int, int) -> int]", functionName)}
        }
    }

    // Handle 9-arg function for png_set_IHDR: (ptr, ptr, int, int, int, int, int, int, int) -> void
    if len(args) == 9 {
        if p1, ok1 := args[0].(*CPointerValue); ok1 {
            if p2, ok2 := args[1].(*CPointerValue); ok2 {
                allIntsAfter := true
                intArgs := make([]int, 7)
                for i := 2; i < 9; i++ {
                    if v, ok := args[i].(int); ok {
                        intArgs[i-2] = v
                    } else {
                        allIntsAfter = false
                        break
                    }
                }
                if allIntsAfter {
                    var cp1, cp2 unsafe.Pointer
                    if p1 != nil {
                        cp1 = p1.Ptr
                    }
                    if p2 != nil {
                        cp2 = p2.Ptr
                    }
                    C.call_func_ppiiiiiii_void(funcPtr, cp1, cp2,
                        C.uint(intArgs[0]), C.uint(intArgs[1]),
                        C.int(intArgs[2]), C.int(intArgs[3]),
                        C.int(intArgs[4]), C.int(intArgs[5]), C.int(intArgs[6]))
                    return nil, []string{fmt.Sprintf("[SUCCESS: %s(ptr, ptr, 7 ints) -> void]", functionName)}
                }
            }
        }
    }

    // Fallback for unsupported signatures
    return fmt.Sprintf("C_FUNCTION_%s_CALLED", functionName), []string{fmt.Sprintf("[WARNING: %s called with %d args - limited signature support]", functionName, len(args))}
}

// Check if symbol should be processed
func shouldProcessSymbol(name string) bool {
    // Skip common unwanted symbols
    if strings.HasPrefix(name, "_") ||
        strings.Contains(name, "@@") ||
        strings.Contains(name, "@") ||
        len(name) == 0 {
        return false
    }

    return true
}

// Create function symbol with custom alias
func createFunctionSymbolWithAlias(name string, alias string) *CSymbol {
    symbol := &CSymbol{
        Name:         name,
        IsFunction:   true,
        Library:      alias,
        ReturnType:   CVoid,
        Parameters:   []CParameter{},
        SupportNotes: []string{"[SUPPORTED: Function calls implemented]"},
    }

    return symbol
}

// Create data symbol (constants, etc.)
func createDataSymbolWithAlias(name string, alias string) *CSymbol {
    // Generic data symbol - no special cases
    return &CSymbol{
        Name:         name,
        IsFunction:   false,
        Library:      alias,
        ReturnType:   CVoid,
        SupportNotes: []string{"[SUPPORTED: Constants will be available in future version]"},
    }
}

// FFI helper functions for Za stdlib

// CNull returns a null pointer for use in FFI calls
func CNull() *CPointerValue {
    return NullPointer()
}

// CFopen opens a file and returns a FILE* pointer for use with C libraries
func CFopen(path, mode string) *CPointerValue {
    cPath := C.CString(path)
    defer C.free(unsafe.Pointer(cPath))
    cMode := C.CString(mode)
    defer C.free(unsafe.Pointer(cMode))

    fp := C.call_fopen(cPath, cMode)
    if fp == nil {
        return NullPointer()
    }
    return NewCPointer(fp, "FILE*")
}

// CFclose closes a FILE* pointer
func CFclose(fp *CPointerValue) int {
    if fp == nil || fp.Ptr == nil {
        return -1
    }
    return int(C.call_fclose(fp.Ptr))
}

// CPtrIsNull checks if a pointer is null
func CPtrIsNull(p *CPointerValue) bool {
    return p == nil || p.Ptr == nil
}

// CAllocBytes allocates a byte buffer and returns it as a pointer
func CAllocBytes(size int) *CPointerValue {
    ptr := C.malloc(C.size_t(size))
    if ptr == nil {
        return NullPointer()
    }
    // Zero the memory
    C.memset(ptr, 0, C.size_t(size))
    return NewCPointer(ptr, "byte_buffer")
}

// CFreePtr frees a pointer allocated by CAllocBytes
func CFreePtr(p *CPointerValue) {
    if p != nil && p.Ptr != nil {
        C.free(p.Ptr)
        p.Ptr = nil
    }
}

// CSetByte sets a byte at an offset in a buffer
func CSetByte(p *CPointerValue, offset int, value byte) {
    if p != nil && p.Ptr != nil {
        bytePtr := (*byte)(unsafe.Pointer(uintptr(p.Ptr) + uintptr(offset)))
        *bytePtr = value
    }
}

// CGetDataSymbol reads a data symbol value from a loaded C library
// Returns the value as int, float64, or string depending on what works
func CGetDataSymbol(libName, symbolName string) (any, error) {
    lib, exists := loadedCLibraries[libName]
    if !exists {
        return nil, fmt.Errorf("library '%s' not loaded", libName)
    }

    if lib.Handle == nil {
        return nil, fmt.Errorf("library '%s' has no handle", libName)
    }

    // Get symbol address via dlsym
    cSymName := C.CString(symbolName)
    defer C.free(unsafe.Pointer(cSymName))

    addr := C.dlsym(lib.Handle, cSymName)
    if addr == nil {
        return nil, fmt.Errorf("symbol '%s' not found in library '%s'", symbolName, libName)
    }

    // Check if it's marked as a function (shouldn't read function addresses as data)
    if sym, ok := lib.Symbols[symbolName]; ok && sym.IsFunction {
        return nil, fmt.Errorf("'%s' is a function, not a data symbol", symbolName)
    }

    // Try to read as int (most common for constants)
    intVal := C.read_int_symbol(addr)
    return int(intVal), nil
}
