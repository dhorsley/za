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

// Pointer-based functions

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

// 4-arg pointer functions
static void* call_func_str_ptr_ptr_ptr_ptr(void* fn, const char* s, void* p1, void* p2, void* p3) {
    typedef void* (*func_t)(const char*, void*, void*, void*);
    return ((func_t)fn)(s, p1, p2, p3);
}

static void call_func_ptr_ptr_int_int_void(void* fn, void* p1, void* p2, int i1, int i2) {
    typedef void (*func_t)(void*, void*, int, int);
    ((func_t)fn)(p1, p2, i1, i2);
}

// 8-arg function: (ptr, ptr, int, int, int, int, int, int) -> void
static void call_func_ppiiiiiii_void(void* fn, void* p1, void* p2,
                                      unsigned int width, unsigned int height,
                                      int bit_depth, int color_type,
                                      int interlace, int compression, int filter) {
    typedef void (*func_t)(void*, void*, unsigned int, unsigned int, int, int, int, int, int);
    ((func_t)fn)(p1, p2, width, height, bit_depth, color_type, interlace, compression, filter);
}

// File operations
static void* call_fopen(const char* path, const char* mode) {
    return fopen(path, mode);
}

static int call_fclose(void* fp) {
    return fclose((FILE*)fp);
}

// Generic (ptr, ptr) -> void wrapper
static void call_func_ptr_file_void(void* fn, void* p1, void* fp) {
    typedef void (*func_t)(void*, void*);
    ((func_t)fn)(p1, fp);
}

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
    "strings"
    "unsafe"
)

// LoadCLibrary loads a C shared library using dlopen
func LoadCLibrary(path string) (*CLibrary, error) {
    pathC := C.CString(path)
    defer C.free(unsafe.Pointer(pathC))

    // Try RTLD_NOW | RTLD_GLOBAL for better symbol resolution
    handle := C.dlopen(pathC, C.RTLD_NOW|C.RTLD_GLOBAL)
    if handle == nil {
        errMsg := C.GoString(C.dlerror())
        return nil, fmt.Errorf("failed to load library %s: %s", path, errMsg)
    }

    return &CLibrary{
        Name:    path, // Store full path for man page lookup
        Handle:  unsafe.Pointer(handle),
        Symbols: make(map[string]*CSymbol),
        Structs: make(map[string]*CLibraryStruct),
    }, nil
}

// LoadCLibraryWithAlias loads a C library with a specific alias name
func LoadCLibraryWithAlias(path string, alias string) (*CLibrary, error) {
    // On first C library load, try to initialize libffi
    if !libffiChecked {
        InitLibFFI()
    }

    // Check if libffi is available
    if !IsLibFFIAvailable() {
        return nil, fmt.Errorf(
            "C FFI requires libffi but it was not found on this system.\n\n" +
                "To use C library FFI, install libffi:\n\n" +
                "  Debian/Ubuntu:  sudo apt install libffi8\n" +
                "  RHEL/Fedora:    sudo dnf install libffi\n" +
                "  Arch Linux:     sudo pacman -S libffi\n" +
                "  Alpine Linux:   sudo apk add libffi\n" +
                "  FreeBSD:        sudo pkg install libffi\n\n" +
                "After installation, restart your Za program.")
    }

    lib, err := LoadCLibrary(path)
    if err != nil {
        return nil, err
    }
    // Keep lib.Name as the full path (set in LoadCLibrary)
    // but also store the alias for namespace lookups
    lib.Alias = alias             // Set alias field for LIB declaration lookup
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
            // Skip imported symbols (undefined in this library)
            if sym.Section == elf.SHN_UNDEF {
                continue
            }

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
        return nil, []string{"ERROR: Library handle is nil - cannot call function"}
    }

    // Get function pointer from library
    funcNameC := C.CString(functionName)
    defer C.free(unsafe.Pointer(funcNameC))

    funcPtr := C.dlsym(lib.Handle, funcNameC)
    if funcPtr == nil {
        errMsg := C.GoString(C.dlerror())
        return nil, []string{fmt.Sprintf("ERROR: Failed to resolve symbol '%s': %s", functionName, errMsg)}
    }

    // Check if function signature was declared via LIB keyword
    sig, declared := GetDeclaredSignature(lib.Alias, functionName)
    if !declared {
        return nil, []string{fmt.Sprintf(
            "ERROR: Function '%s' not declared. Use: LIB %s::%s(...) -> <return_type>",
            functionName, lib.Alias, functionName)}
    }

    // Validate argument count matches declaration
    if sig.HasVarargs {
        // Variadic function - require at least fixed args count
        if len(args) < sig.FixedArgCount {
            return nil, []string{fmt.Sprintf(
                "ERROR: %s expects at least %d arguments (declared in LIB %s::%s), got %d",
                functionName, sig.FixedArgCount, lib.Alias, functionName, len(args))}
        }
    } else {
        // Non-variadic function - require exact match
        if len(args) != len(sig.ParamTypes) {
            return nil, []string{fmt.Sprintf(
                "ERROR: %s expects %d arguments (declared in LIB %s::%s), got %d",
                functionName, len(sig.ParamTypes), lib.Alias, functionName, len(args))}
        }
    }

    // Use libffi if available
    if IsLibFFIAvailable() {
        // Call via libffi with declared signature
        result, err := CallCFunctionViaLibFFI(funcPtr, functionName, args, sig)
        if err != nil {
            return nil, []string{fmt.Sprintf("ERROR: libffi call failed: %v", err)}
        }

        return result, nil
    }

    // Fallback if libffi not available
    return nil, []string{"ERROR: libffi not available - this should have been caught during library loading"}
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
