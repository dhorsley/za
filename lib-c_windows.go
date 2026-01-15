//go:build windows
// +build windows

// Windows FFI Support Status:
// FFI (Foreign Function Interface) is NOT supported on Windows.
// Za's FFI feature is designed for and only available on Linux and BSD platforms.
// This file provides stub implementations that return clear error messages
// when MODULE or LIB statements attempt to use FFI on Windows.

package main

import (
    "fmt"
    "path/filepath"
    "strings"
)

// LoadCLibrary loads a C shared library using LoadLibrary on Windows
// Windows FFI support has been removed. Za focuses on Linux and BSD platforms.
func LoadCLibrary(path string) (*CLibrary, error) {
    return nil, fmt.Errorf("FFI is not supported on Windows.\nZa's FFI feature is only available on Linux and BSD platforms.\nLibrary path: %s", path)
}

// LoadCLibraryWithAlias loads a C library with a specific alias name
func LoadCLibraryWithAlias(path string, alias string) (*CLibrary, error) {
    lib, err := LoadCLibrary(path)
    if err != nil {
        return nil, err
    }
    lib.Name = alias
    loadedCLibraries[alias] = lib
    return lib, nil
}

// DiscoverLibrarySymbols discovers symbols from a loaded C library
// Windows FFI support has been removed.
func DiscoverLibrarySymbols(lib *CLibrary, libPath string) error {
    return fmt.Errorf("FFI is not supported on Windows. Za's FFI feature is only available on Linux and BSD platforms.")
}

// DiscoverSymbolsWithAlias discovers symbols and returns them as a slice
func DiscoverSymbolsWithAlias(libPath string, alias string, existingLib *CLibrary) ([]*CSymbol, error) {
    return nil, fmt.Errorf("FFI is not supported on Windows. Za's FFI feature is only available on Linux and BSD platforms.")
}

// callCFunctionPlatform attempts to call a C function with given arguments
func callCFunctionPlatform(lib *CLibrary, functionName string, args []any) (any, []string) {
    return nil, []string{"FFI is not supported on Windows. Za's FFI feature is only available on Linux and BSD platforms."}
}

// shouldProcessSymbol checks if a symbol should be processed
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

// createFunctionSymbolWithAlias creates a function symbol with custom alias
func createFunctionSymbolWithAlias(name string, alias string) *CSymbol {
    return &CSymbol{
        Name:         name,
        IsFunction:   true,
        Library:      alias,
        ReturnType:   CVoid,
        Parameters:   []CParameter{},
        SupportNotes: []string{"FFI not supported on Windows (Linux/BSD only)"},
    }
}

// createDataSymbolWithAlias creates a data symbol
func createDataSymbolWithAlias(name string, alias string) *CSymbol {
    return &CSymbol{
        Name:         name,
        IsFunction:   false,
        Library:      alias,
        ReturnType:   CVoid,
        SupportNotes: []string{"FFI not supported on Windows (Linux/BSD only)"},
    }
}

// FFI helper function stubs for Windows builds

func CNull() *CPointerValue {
    return NullPointer()
}

func CFopen(path, mode string) *CPointerValue {
    return NullPointer()
}

func CFclose(fp *CPointerValue) int {
    return -1
}

func CPtrIsNull(p *CPointerValue) bool {
    return true
}

func CAllocBytes(size int) *CPointerValue {
    return NullPointer()
}

func CFreePtr(p *CPointerValue) {
}

func CSetByte(p *CPointerValue, offset int, value byte) {
}

func CGetDataSymbol(libName, symbolName string) (any, error) {
    return nil, fmt.Errorf("FFI is not supported on Windows. Za's FFI feature is only available on Linux and BSD platforms.")
}
