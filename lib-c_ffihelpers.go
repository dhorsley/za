//go:build !noffi && cgo
// +build !noffi,cgo

package main

/*
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

// Cross-platform C helpers used by the shared FFI memory/pointer functions.
// These avoid duplicating the CFopen/CAlloc/... helpers per platform.
static void* call_fopen(const char* path, const char* mode) {
    return fopen(path, mode);
}

static int call_fclose(void* fp) {
    return fclose((FILE*)fp);
}
*/
import "C"

import (
	"fmt"
	"unsafe"
)

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

// CAllocBytes allocates a zero-initialized byte buffer and returns it as a pointer
func CAllocBytes(size int) *CPointerValue {
	ptr := C.malloc(C.size_t(size))
	if ptr == nil {
		return NullPointer()
	}
	// Zero the memory
	C.memset(ptr, 0, C.size_t(size))
	return NewCPointer(ptr, "byte_buffer")
}

// CAllocBytesUninit allocates a raw byte buffer (NOT zeroed) and returns it as a pointer.
// The caller must write every byte before reading. Useful for performance-critical
// buffers that are completely overwritten before use.
func CAllocBytesUninit(size int) *CPointerValue {
	ptr := C.malloc(C.size_t(size))
	if ptr == nil {
		return NullPointer()
	}
	return NewCPointer(ptr, "byte_buffer_uninit")
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

// CSetUint16 writes a uint16 at an offset in a buffer
func CSetUint16(p *CPointerValue, offset int, value uint16) {
	if p != nil && p.Ptr != nil {
		uint16Ptr := (*uint16)(unsafe.Pointer(uintptr(p.Ptr) + uintptr(offset)))
		*uint16Ptr = value
	}
}

// CSetInt16 writes an int16 at an offset in a buffer
func CSetInt16(p *CPointerValue, offset int, value int16) {
	if p != nil && p.Ptr != nil {
		int16Ptr := (*int16)(unsafe.Pointer(uintptr(p.Ptr) + uintptr(offset)))
		*int16Ptr = value
	}
}

// CSetUint32 writes a uint32 at an offset in a buffer
func CSetUint32(p *CPointerValue, offset int, value uint32) {
	if p != nil && p.Ptr != nil {
		uint32Ptr := (*uint32)(unsafe.Pointer(uintptr(p.Ptr) + uintptr(offset)))
		*uint32Ptr = value
	}
}

// CSetInt32 writes an int32 at an offset in a buffer
func CSetInt32(p *CPointerValue, offset int, value int32) {
	if p != nil && p.Ptr != nil {
		int32Ptr := (*int32)(unsafe.Pointer(uintptr(p.Ptr) + uintptr(offset)))
		*int32Ptr = value
	}
}

// CSetUint64 writes a uint64 at an offset in a buffer
func CSetUint64(p *CPointerValue, offset int, value uint64) {
	if p != nil && p.Ptr != nil {
		uint64Ptr := (*uint64)(unsafe.Pointer(uintptr(p.Ptr) + uintptr(offset)))
		*uint64Ptr = value
	}
}

// CSetInt64 writes an int64 at an offset in a buffer
func CSetInt64(p *CPointerValue, offset int, value int64) {
	if p != nil && p.Ptr != nil {
		int64Ptr := (*int64)(unsafe.Pointer(uintptr(p.Ptr) + uintptr(offset)))
		*int64Ptr = value
	}
}

// CGetByte reads a byte at an offset in a buffer
func CGetByte(p *CPointerValue, offset int) byte {
	if p != nil && p.Ptr != nil {
		bytePtr := (*byte)(unsafe.Pointer(uintptr(p.Ptr) + uintptr(offset)))
		return *bytePtr
	}
	return 0
}

// CGetUint16 reads a uint16 at an offset in a buffer
func CGetUint16(p *CPointerValue, offset int) uint16 {
	if p != nil && p.Ptr != nil {
		uint16Ptr := (*uint16)(unsafe.Pointer(uintptr(p.Ptr) + uintptr(offset)))
		return *uint16Ptr
	}
	return 0
}

// CGetUint32 reads a uint32 at an offset in a buffer
func CGetUint32(p *CPointerValue, offset int) uint32 {
	if p != nil && p.Ptr != nil {
		uint32Ptr := (*uint32)(unsafe.Pointer(uintptr(p.Ptr) + uintptr(offset)))
		return *uint32Ptr
	}
	return 0
}

// CGetInt16 reads an int16 at an offset in a buffer
func CGetInt16(p *CPointerValue, offset int) int16 {
	if p != nil && p.Ptr != nil {
		int16Ptr := (*int16)(unsafe.Pointer(uintptr(p.Ptr) + uintptr(offset)))
		return *int16Ptr
	}
	return 0
}

// CGetInt32 reads an int32 at an offset in a buffer
func CGetInt32(p *CPointerValue, offset int) int32 {
	if p != nil && p.Ptr != nil {
		int32Ptr := (*int32)(unsafe.Pointer(uintptr(p.Ptr) + uintptr(offset)))
		return *int32Ptr
	}
	return 0
}

// CGetUint64 reads a uint64 at an offset in a buffer
func CGetUint64(p *CPointerValue, offset int) uint64 {
	if p != nil && p.Ptr != nil {
		uint64Ptr := (*uint64)(unsafe.Pointer(uintptr(p.Ptr) + uintptr(offset)))
		return *uint64Ptr
	}
	return 0
}

// CGetInt64 reads an int64 at an offset in a buffer
func CGetInt64(p *CPointerValue, offset int) int64 {
	if p != nil && p.Ptr != nil {
		int64Ptr := (*int64)(unsafe.Pointer(uintptr(p.Ptr) + uintptr(offset)))
		return *int64Ptr
	}
	return 0
}

// CGetByteAtAddr reads a byte at an int64 address + offset
// This allows working with opaque pointers returned as int64 from FFI calls
func CGetByteAtAddr(addr int64, offset int) byte {
	if addr != 0 {
		bytePtr := (*byte)(unsafe.Pointer(uintptr(addr) + uintptr(offset)))
		return *bytePtr
	}
	return 0
}

// CGetUint16AtAddr reads a uint16 at an int64 address + offset
func CGetUint16AtAddr(addr int64, offset int) uint16 {
	if addr != 0 {
		uint16Ptr := (*uint16)(unsafe.Pointer(uintptr(addr) + uintptr(offset)))
		return *uint16Ptr
	}
	return 0
}

// CGetInt16AtAddr reads an int16 at an int64 address + offset
func CGetInt16AtAddr(addr int64, offset int) int16 {
	if addr != 0 {
		int16Ptr := (*int16)(unsafe.Pointer(uintptr(addr) + uintptr(offset)))
		return *int16Ptr
	}
	return 0
}

// CGetUint32AtAddr reads a uint32 at an int64 address + offset
func CGetUint32AtAddr(addr int64, offset int) uint32 {
	if addr != 0 {
		uint32Ptr := (*uint32)(unsafe.Pointer(uintptr(addr) + uintptr(offset)))
		return *uint32Ptr
	}
	return 0
}

// CGetInt32AtAddr reads an int32 at an int64 address + offset
func CGetInt32AtAddr(addr int64, offset int) int32 {
	if addr != 0 {
		int32Ptr := (*int32)(unsafe.Pointer(uintptr(addr) + uintptr(offset)))
		return *int32Ptr
	}
	return 0
}

// CGetUint64AtAddr reads a uint64 at an int64 address + offset
func CGetUint64AtAddr(addr int64, offset int) uint64 {
	if addr != 0 {
		uint64Ptr := (*uint64)(unsafe.Pointer(uintptr(addr) + uintptr(offset)))
		return *uint64Ptr
	}
	return 0
}

// CGetInt64AtAddr reads an int64 at an int64 address + offset
func CGetInt64AtAddr(addr int64, offset int) int64 {
	if addr != 0 {
		int64Ptr := (*int64)(unsafe.Pointer(uintptr(addr) + uintptr(offset)))
		return *int64Ptr
	}
	return 0
}

// CSetByteAtAddr writes a byte at an int64 address + offset
func CSetByteAtAddr(addr int64, offset int, value byte) {
	if addr != 0 {
		bytePtr := (*byte)(unsafe.Pointer(uintptr(addr) + uintptr(offset)))
		*bytePtr = value
	}
}

// CSetUint16AtAddr writes a uint16 at an int64 address + offset
func CSetUint16AtAddr(addr int64, offset int, value uint16) {
	if addr != 0 {
		uint16Ptr := (*uint16)(unsafe.Pointer(uintptr(addr) + uintptr(offset)))
		*uint16Ptr = value
	}
}

// CSetInt16AtAddr writes an int16 at an int64 address + offset
func CSetInt16AtAddr(addr int64, offset int, value int16) {
	if addr != 0 {
		int16Ptr := (*int16)(unsafe.Pointer(uintptr(addr) + uintptr(offset)))
		*int16Ptr = value
	}
}

// CSetUint32AtAddr writes a uint32 at an int64 address + offset
func CSetUint32AtAddr(addr int64, offset int, value uint32) {
	if addr != 0 {
		uint32Ptr := (*uint32)(unsafe.Pointer(uintptr(addr) + uintptr(offset)))
		*uint32Ptr = value
	}
}

// CSetInt32AtAddr writes an int32 at an int64 address + offset
func CSetInt32AtAddr(addr int64, offset int, value int32) {
	if addr != 0 {
		int32Ptr := (*int32)(unsafe.Pointer(uintptr(addr) + uintptr(offset)))
		*int32Ptr = value
	}
}

// CSetUint64AtAddr writes a uint64 at an int64 address + offset
func CSetUint64AtAddr(addr int64, offset int, value uint64) {
	if addr != 0 {
		uint64Ptr := (*uint64)(unsafe.Pointer(uintptr(addr) + uintptr(offset)))
		*uint64Ptr = value
	}
}

// CSetInt64AtAddr writes an int64 at an int64 address + offset
func CSetInt64AtAddr(addr int64, offset int, value int64) {
	if addr != 0 {
		int64Ptr := (*int64)(unsafe.Pointer(uintptr(addr) + uintptr(offset)))
		*int64Ptr = value
	}
}

// CGetFloat reads a float32 at an offset in a buffer and returns as float64
func CGetFloat(p *CPointerValue, offset int) float64 {
	if p != nil && p.Ptr != nil {
		floatPtr := (*float32)(unsafe.Pointer(uintptr(p.Ptr) + uintptr(offset)))
		return float64(*floatPtr)
	}
	return 0.0
}

// CSetFloat writes a float64 as float32 at an offset in a buffer
func CSetFloat(p *CPointerValue, offset int, value float64) {
	if p != nil && p.Ptr != nil {
		floatPtr := (*float32)(unsafe.Pointer(uintptr(p.Ptr) + uintptr(offset)))
		*floatPtr = float32(value)
	}
}

// CGetFloat32 reads a float32 at an offset in a buffer and returns as float32
func CGetFloat32(p *CPointerValue, offset int) float32 {
	if p != nil && p.Ptr != nil {
		floatPtr := (*float32)(unsafe.Pointer(uintptr(p.Ptr) + uintptr(offset)))
		return *floatPtr
	}
	return 0.0
}

// CSetFloat32 writes a float32 at an offset in a buffer
func CSetFloat32(p *CPointerValue, offset int, value float32) {
	if p != nil && p.Ptr != nil {
		floatPtr := (*float32)(unsafe.Pointer(uintptr(p.Ptr) + uintptr(offset)))
		*floatPtr = value
	}
}

// CGetFloat32AtAddr reads a float32 at an int64 address + offset and returns as float32
func CGetFloat32AtAddr(addr int64, offset int) float32 {
	if addr != 0 {
		floatPtr := (*float32)(unsafe.Pointer(uintptr(addr) + uintptr(offset)))
		return *floatPtr
	}
	return 0.0
}

// CSetFloat32AtAddr writes a float32 at an int64 address + offset
func CSetFloat32AtAddr(addr int64, offset int, value float32) {
	if addr != 0 {
		floatPtr := (*float32)(unsafe.Pointer(uintptr(addr) + uintptr(offset)))
		*floatPtr = value
	}
}

// CGetDouble reads a float64 at an offset in a buffer
func CGetDouble(p *CPointerValue, offset int) float64 {
	if p != nil && p.Ptr != nil {
		doublePtr := (*float64)(unsafe.Pointer(uintptr(p.Ptr) + uintptr(offset)))
		return *doublePtr
	}
	return 0.0
}

// CSetDouble writes a float64 at an offset in a buffer
func CSetDouble(p *CPointerValue, offset int, value float64) {
	if p != nil && p.Ptr != nil {
		doublePtr := (*float64)(unsafe.Pointer(uintptr(p.Ptr) + uintptr(offset)))
		*doublePtr = value
	}
}

// CGetFloatAtAddr reads a float32 at an int64 address + offset and returns as float64
func CGetFloatAtAddr(addr int64, offset int) float64 {
	if addr != 0 {
		floatPtr := (*float32)(unsafe.Pointer(uintptr(addr) + uintptr(offset)))
		return float64(*floatPtr)
	}
	return 0.0
}

// CSetFloatAtAddr writes a float64 as float32 at an int64 address + offset
func CSetFloatAtAddr(addr int64, offset int, value float64) {
	if addr != 0 {
		floatPtr := (*float32)(unsafe.Pointer(uintptr(addr) + uintptr(offset)))
		*floatPtr = float32(value)
	}
}

// CGetDoubleAtAddr reads a float64 at an int64 address + offset
func CGetDoubleAtAddr(addr int64, offset int) float64 {
	if addr != 0 {
		doublePtr := (*float64)(unsafe.Pointer(uintptr(addr) + uintptr(offset)))
		return *doublePtr
	}
	return 0.0
}

// CSetDoubleAtAddr writes a float64 at an int64 address + offset
func CSetDoubleAtAddr(addr int64, offset int, value float64) {
	if addr != 0 {
		doublePtr := (*float64)(unsafe.Pointer(uintptr(addr) + uintptr(offset)))
		*doublePtr = value
	}
}

// CSetString copies a Za string to a C buffer at the given pointer
func CSetString(ptr *CPointerValue, s string) error {
	if ptr == nil || ptr.Ptr == nil {
		return fmt.Errorf("c_set_string: pointer is null")
	}
	cstr := C.CString(s)
	defer C.free(unsafe.Pointer(cstr))
	C.memcpy(ptr.Ptr, unsafe.Pointer(cstr), C.size_t(len(s)+1))
	return nil
}

// CNewString allocates a new C string from a Za string
func CNewString(s string) *CPointerValue {
	cstr := C.CString(s)
	if cstr == nil {
		return NullPointer()
	}
	return NewCPointer(unsafe.Pointer(cstr), "char*")
}

// CPtrToString converts a C string pointer to a Za string
func CPtrToString(ptr *CPointerValue) (string, error) {
	if ptr == nil || ptr.Ptr == nil {
		return "", fmt.Errorf("c_ptr_to_string: pointer is null")
	}
	return C.GoString((*C.char)(ptr.Ptr)), nil
}