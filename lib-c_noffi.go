//go:build noffi || (linux && !cgo)
// +build noffi linux,!cgo

package main

import (
    "fmt"
    "unsafe"
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

func CGetByteAtAddr(addr int64, offset int) byte {
    return 0
}

func CGetUint16AtAddr(addr int64, offset int) uint16 {
    return 0
}

func CGetInt16AtAddr(addr int64, offset int) int16 {
    return 0
}

func CGetUint32AtAddr(addr int64, offset int) uint32 {
    return 0
}

func CGetInt32AtAddr(addr int64, offset int) int32 {
    return 0
}

func CGetUint64AtAddr(addr int64, offset int) uint64 {
    return 0
}

func CGetInt64AtAddr(addr int64, offset int) int64 {
    return 0
}

func CGetDataSymbol(libName, symbolName string) (any, error) {
    return nil, fmt.Errorf("FFI disabled in this build")
}

// ============================================================================
// STRUCT/UNION MARSHALING STUBS
// ============================================================================

// unmarshalUnion - FFI disabled in this build
func unmarshalUnion(ptr unsafe.Pointer, unionDef *CLibraryStruct) (map[string]any, error) {
    return nil, fmt.Errorf("C FFI disabled in this build (struct/union unmarshaling not available)")
}

// UnmarshalStructFromC - FFI disabled in this build
func UnmarshalStructFromC(cPtr unsafe.Pointer, structDef *CLibraryStruct, zaStructName string) (any, error) {
    return nil, fmt.Errorf("C FFI disabled in this build (struct/union unmarshaling not available)")
}

// CSetString - FFI disabled in this build
func CSetString(ptr *CPointerValue, s string) error {
    return fmt.Errorf("C FFI disabled in this build (c_set_string not available)")
}

// CNewString - FFI disabled in this build
func CNewString(s string) *CPointerValue {
    return NullPointer()
}

// CPtrToString - FFI disabled in this build
func CPtrToString(ptr *CPointerValue) (string, error) {
    return "", fmt.Errorf("C FFI disabled in this build (c_ptr_to_string not available)")
}

// wcharSize - Platform-detected wchar_t size (unavailable in noffi builds)
var wcharSize uintptr = 0
