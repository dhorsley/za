//go:build noffi || (linux && !cgo)
// +build noffi linux,!cgo

package main

import (
    "fmt"
)

// LoadCLibrary - FFI disabled in this build
func LoadCLibrary(path string) (*CLibrary, error) {
    return nil, fmt.Errorf("C FFI disabled in this build (use CGO-enabled build for FFI support)")
}

// LoadCLibraryWithAlias - FFI disabled in this build
func LoadCLibraryWithAlias(path string, alias string) (*CLibrary, error) {
    return nil, fmt.Errorf("C FFI disabled in this build (use CGO-enabled build for FFI support)")
}

// DiscoverLibrarySymbols - FFI disabled in this build
func DiscoverLibrarySymbols(lib *CLibrary, libPath string) error {
    return fmt.Errorf("C FFI disabled in this build")
}

// DiscoverSymbolsWithAlias - FFI disabled in this build
func DiscoverSymbolsWithAlias(libPath string, alias string, existingLib *CLibrary) ([]*CSymbol, error) {
    return nil, fmt.Errorf("C FFI disabled in this build")
}

// callCFunctionPlatform - FFI disabled in this build
func callCFunctionPlatform(lib *CLibrary, functionName string, args []any) (any, []string) {
    return nil, []string{"[ERROR: C FFI disabled in this build]"}
}

// shouldProcessSymbol - FFI disabled in this build
func shouldProcessSymbol(name string) bool {
    return false
}

// createFunctionSymbolWithAlias - FFI disabled in this build
func createFunctionSymbolWithAlias(name string, alias string) *CSymbol {
    return nil
}

// createDataSymbolWithAlias - FFI disabled in this build
func createDataSymbolWithAlias(name string, alias string) *CSymbol {
    return nil
}

// FFI helper function stubs for noffi builds

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

func CGetByte(p *CPointerValue, offset int) byte {
    return 0
}

func CGetUint16(p *CPointerValue, offset int) uint16 {
    return 0
}

func CGetUint32(p *CPointerValue, offset int) uint32 {
    return 0
}

func CGetInt16(p *CPointerValue, offset int) int16 {
    return 0
}

func CGetInt32(p *CPointerValue, offset int) int32 {
    return 0
}

func CGetUint64(p *CPointerValue, offset int) uint64 {
    return 0
}

func CGetInt64(p *CPointerValue, offset int) int64 {
    return 0
}

func CGetDataSymbol(libName, symbolName string) (any, error) {
    return nil, fmt.Errorf("FFI disabled in this build")
}
