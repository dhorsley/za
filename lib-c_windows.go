//go:build windows && !noffi && cgo
// +build windows,!noffi,cgo

package main

/*
#include <windows.h>
#include <stdio.h>

extern void *za_dlopen(const char *path);
extern void *za_dlsym(void *handle, const char *symbol);
extern void  za_dlclose(void *handle);

// Last Win32 error code for loader diagnostics
static int za_getlasterror(void) {
    return (int)GetLastError();
}

// Full module file path so PE export parsing can open the DLL on disk even
// when the caller supplied a bare DLL name (normal search already resolved it).
static int za_module_path(void *handle, char *buf, int buflen) {
    if (handle == NULL || buf == NULL || buflen <= 0) {
        return 0;
    }
    DWORD n = GetModuleFileNameA((HMODULE)handle, buf, (DWORD)buflen);
    if (n == 0 || n >= (DWORD)buflen) {
        return 0;
    }
    return (int)n;
}

// Read a data symbol value (constants, etc.)
static int read_int_symbol_win(void* addr) {
    if (addr == NULL) return 0;
    return *((int*)addr);
}
*/
import "C"

import (
    "bytes"
    "context"
    debugpe "debug/pe" // not aliased "pe" - main.go declares a package var named pe
    "encoding/binary"
    "fmt"
    "path/filepath"
    "strings"
    "unsafe"
)

// LoadCLibrary loads a C shared library (DLL) using LoadLibraryA.
// A bare DLL name uses the OS normal search; a full path loads that file.
// No .so -> .dll translation is performed - scripts specify the native name.
func LoadCLibrary(path string) (*CLibrary, error) {
    pathC := C.CString(path)
    defer C.free(unsafe.Pointer(pathC))

    handle := C.za_dlopen(pathC)
    if handle == nil {
        errCode := int(C.za_getlasterror())
        return nil, fmt.Errorf("failed to load library %s (Windows error %d / 0x%X)", path, errCode, errCode)
    }

    // Resolve the module's full path (for PE export discovery) when possible.
    name := path
    if filepath.IsAbs(path) {
        name = path
    } else {
        var buf [4096]C.char
        n := C.za_module_path(handle, &buf[0], 4096)
        if n > 0 {
            name = C.GoString(&buf[0])
        }
    }

    return &CLibrary{
        Name:    name, // Full path where known, for PE export parsing
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
            "C FFI requires the libffi DLL but it was not found on this system.\n\n" +
                "Place libffi-8.dll next to this script (or in the DLL search path), e.g.:\n\n" +
                "  libffi-8.dll   (libffi 3.4.x/3.8.x)\n\n" +
                "The DLL is supplied with Za at shared/win/libffi-8.dll.\n" +
                "After placing it, restart your Za program.")
    }

    lib, err := LoadCLibrary(path)
    if err != nil {
        return nil, err
    }
    lib.Alias = alias             // Set alias field for LIB declaration lookup
    loadedCLibraries[alias] = lib // Register library for help system
    return lib, nil
}

// CanResolveSymbol checks if a named symbol can actually be resolved at runtime
// via GetProcAddress on the loaded library handle.
func CanResolveSymbol(lib *CLibrary, name string) bool {
    if lib == nil || lib.Handle == nil {
        return false
    }
    nameC := C.CString(name)
    defer C.free(unsafe.Pointer(nameC))
    return C.za_dlsym(lib.Handle, nameC) != nil
}

// DiscoverLibrarySymbols discovers symbols from a loaded C library using
// Windows PE export-table parsing.
func DiscoverLibrarySymbols(lib *CLibrary, libPath string) error {
    file, err := debugpe.Open(libPath)
    if err != nil {
        return fmt.Errorf("failed to open PE file: %v", err)
    }
    defer file.Close()

    exports, err := parsePEExports(file)
    if err != nil {
        return fmt.Errorf("failed to parse PE exports: %v", err)
    }

    symbolCount := 0
    for _, exp := range exports {
        cleanName := exp.Name
        if !shouldProcessSymbol(cleanName) {
            continue
        }

        // Skip exported symbols we cannot actually resolve at runtime
        // (identical filtering to the Unix dlsym-based validation).
        if !CanResolveSymbol(lib, cleanName) {
            continue
        }

        symbolCount++
        libraryIdentifier := lib.Alias
        if libraryIdentifier == "" {
            libraryIdentifier = lib.Name
        }

        if exp.Function {
            funcSym := createFunctionSymbolWithAlias(cleanName, libraryIdentifier)
            lib.Symbols[funcSym.Name] = funcSym
        } else {
            dataSym := createDataSymbolWithAlias(cleanName, libraryIdentifier)
            if dataSym != nil {
                lib.Symbols[dataSym.Name] = dataSym
            }
        }
    }

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

    // Use lib.Name for symbol discovery (full module path where available)
    err := DiscoverLibrarySymbols(lib, lib.Name)
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
func callCFunctionPlatform(ctx context.Context, lib *CLibrary, functionName string, args []any) (any, []string) {
    if lib.Handle == nil {
        return nil, []string{"ERROR: Library handle is nil - cannot call function"}
    }

    // Get function pointer from library
    funcNameC := C.CString(functionName)
    defer C.free(unsafe.Pointer(funcNameC))

    funcPtr := C.za_dlsym(lib.Handle, funcNameC)
    if funcPtr == nil {
        return nil, []string{fmt.Sprintf("ERROR: Failed to resolve symbol '%s'", functionName)}
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
        result, err := CallCFunctionViaLibFFI(ctx, funcPtr, functionName, args, sig)
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

// getDefaultAlias extracts a default alias from a library path
func getDefaultAlias(path string) string {
    base := filepath.Base(path)
    // Remove .dll extension
    if strings.HasSuffix(base, ".dll") {
        base = strings.TrimSuffix(base, ".dll")
    }
    return base
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

    // Get symbol address via GetProcAddress
    cSymName := C.CString(symbolName)
    defer C.free(unsafe.Pointer(cSymName))

    addr := C.za_dlsym(lib.Handle, cSymName)
    if addr == nil {
        return nil, fmt.Errorf("symbol '%s' not found in library '%s'", symbolName, libName)
    }

    // Check if it's marked as a function (shouldn't read function addresses as data)
    if sym, ok := lib.Symbols[symbolName]; ok && sym.IsFunction {
        return nil, fmt.Errorf("'%s' is a function, not a data symbol", symbolName)
    }

    // Try to read as int (most common for constants)
    intVal := C.read_int_symbol_win(addr)
    return int(intVal), nil
}

// ============================================================================
// PE export-table parsing
// ============================================================================

// peExportDirectories contains the IndexOfExportDirectory (directory 0).
// WinNT IMAGE_EXPORT_DIRECTORY layout (little endian).
type peExportDirectory struct {
    Characteristics       uint32
    TimeDateStamp         uint32
    MajorVersion          uint16
    MinorVersion          uint16
    Name                  uint32
    Base                  uint32
    NumberOfFunctions     uint32
    NumberOfNames         uint32
    AddressOfFunctions    uint32
    AddressOfNames        uint32
    AddressOfNameOrdinals uint32
}

// peExportSymbol is one exported symbol from the PE export table.
type peExportSymbol struct {
    Name     string
    Function bool // true when the export points into an executable section
}

// peRvaToSection locates the section and offset-within-section for a relative
// virtual address. Returns false when the RVA is not backed by raw file data.
func peRvaToSection(f *debugpe.File, rva uint32) (*debugpe.Section, int64, bool) {
    for _, s := range f.Sections {
        size := s.VirtualSize
        if size == 0 {
            size = s.Size
        }
        if rva >= s.VirtualAddress && rva < s.VirtualAddress+size {
            off := int64(rva - s.VirtualAddress)
            if off >= int64(s.Size) {
                return nil, 0, false // in BSS/zero-fill, not on disk
            }
            return s, off, true
        }
    }
    return nil, 0, false
}

// peReadAt reads bytes from a section (offsets are section-relative).
func peReadAt(sec *debugpe.Section, off int64, buf []byte) bool {
    n, _ := sec.ReadAt(buf, off)
    return n == len(buf)
}

// peReadU32 reads a little-endian uint32 from a section at the given offset.
func peReadU32(sec *debugpe.Section, off int64) (uint32, bool) {
    var buf [4]byte
    if !peReadAt(sec, off, buf[:]) {
        return 0, false
    }
    return binary.LittleEndian.Uint32(buf[:]), true
}

// peReadU16 reads a little-endian uint16 from a section at the given offset.
func peReadU16(sec *debugpe.Section, off int64) (uint16, bool) {
    var buf [2]byte
    if !peReadAt(sec, off, buf[:]) {
        return 0, false
    }
    return binary.LittleEndian.Uint16(buf[:]), true
}

// peReadCString reads a NUL-terminated string from a section at an offset.
func peReadCString(sec *debugpe.Section, off int64, maxLen int) (string, bool) {
    buf := make([]byte, maxLen)
    n, _ := sec.ReadAt(buf, off)
    if n <= 0 {
        return "", false
    }
    data := buf[:n]
    if i := bytes.IndexByte(data, 0); i >= 0 {
        return string(data[:i]), true
    }
    return string(data), true
}

// parsePEExports enumerates all exported names of a PE file. For each export
// it records whether the export points into an executable section (heuristic
// for function vs data, since the PE format carries no type information).
func parsePEExports(f *debugpe.File) ([]peExportSymbol, error) {
    if len(f.Sections) == 0 {
        return nil, fmt.Errorf("no sections")
    }

    var exports []peExportSymbol

    // Directory entry 0 = IMAGE_DIRECTORY_ENTRY_EXPORT
    var dataDirs []debugpe.DataDirectory
    switch oh := f.OptionalHeader.(type) {
    case *debugpe.OptionalHeader64:
        dataDirs = oh.DataDirectory[:]
    case *debugpe.OptionalHeader32:
        dataDirs = oh.DataDirectory[:]
    default:
        return nil, fmt.Errorf("unsupported optional header format")
    }
    if len(dataDirs) == 0 || dataDirs[0].VirtualAddress == 0 || dataDirs[0].Size == 0 {
        return exports, nil // no export table
    }

    exportSec, exportSecOff, ok := peRvaToSection(f, dataDirs[0].VirtualAddress)
    if !ok || exportSec == nil {
        return nil, fmt.Errorf("export directory RVA not mapped to file data")
    }

    var dir peExportDirectory
    dirBuf := make([]byte, 40)
    if !peReadAt(exportSec, exportSecOff, dirBuf) {
        return nil, fmt.Errorf("truncated export directory")
    }
    if err := binary.Read(bytes.NewReader(dirBuf), binary.LittleEndian, &dir); err != nil {
        return nil, err
    }

    // Validate the arrays live in mapped data.
    namesSec, namesOff, ok := peRvaToSection(f, dir.AddressOfNames)
    if !ok || namesSec == nil {
        return nil, fmt.Errorf("export name table RVA not mapped")
    }

    funcsSec, funcsOff, ok := peRvaToSection(f, dir.AddressOfFunctions)
    if !ok || funcsSec == nil {
        return nil, fmt.Errorf("export function table RVA not mapped")
    }

    ordSec, ordOff, ok := peRvaToSection(f, dir.AddressOfNameOrdinals)
    if !ok || ordSec == nil {
        return nil, fmt.Errorf("export ordinal table RVA not mapped")
    }

    numNames := int(dir.NumberOfNames)
    for i := 0; i < numNames; i++ {
        nameRva, ok := peReadU32(namesSec, namesOff+int64(i)*4)
        if !ok {
            continue
        }
        strSec, strOff, ok := peRvaToSection(f, nameRva)
        if !ok || strSec == nil {
            continue
        }
        name, ok := peReadCString(strSec, strOff, 256)
        if !ok || name == "" {
            continue
        }

        function := true
        ord, ok := peReadU16(ordSec, ordOff+int64(i)*2)
        if ok && int(ord) < int(dir.NumberOfFunctions) {
            fnRva, ok := peReadU32(funcsSec, funcsOff+int64(ord)*4)
            if ok {
                // A forwarder points into the export table itself; treat as
                // function. Otherwise classify by the target section's flags.
                if fnRva >= dataDirs[0].VirtualAddress &&
                    fnRva < dataDirs[0].VirtualAddress+dataDirs[0].Size {
                    function = true
                } else if sec, _, ok := peRvaToSection(f, fnRva); ok && sec != nil {
                    function = sec.Characteristics&debugpe.IMAGE_SCN_MEM_EXECUTE != 0
                }
            }
        }

        exports = append(exports, peExportSymbol{Name: name, Function: function})
    }

    return exports, nil
}