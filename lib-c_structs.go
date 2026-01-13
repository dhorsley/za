//go:build !windows && cgo
// +build !windows,cgo

package main

/*
#include <stdlib.h>
#include <string.h>
*/
import "C"

import (
    "fmt"
    "reflect"
    "unsafe"
)

// MarshalStructToC converts a Za struct to C memory layout
// Returns: pointer to C memory, cleanup function, error
func MarshalStructToC(zaStruct any, structDef *CLibraryStruct) (unsafe.Pointer, func(), error) {
    if structDef == nil {
        return nil, nil, fmt.Errorf("struct definition is nil")
    }

    // Allocate C memory for the struct
    ptr := C.malloc(C.size_t(structDef.Size))
    if ptr == nil {
        return nil, nil, fmt.Errorf("failed to allocate C memory for struct (size: %d)", structDef.Size)
    }

    // Zero-initialize the memory
    C.memset(ptr, 0, C.size_t(structDef.Size))

    // Track allocated string pointers for cleanup
    var allocatedStrings []unsafe.Pointer

    cleanup := func() {
        // Free all allocated string pointers
        for _, strPtr := range allocatedStrings {
            C.free(strPtr)
        }
        // Free the struct memory
        C.free(ptr)
    }

    // Use reflection to read Za struct fields
    val := reflect.ValueOf(zaStruct)
    if val.Kind() == reflect.Ptr {
        val = val.Elem()
    }

    if val.Kind() != reflect.Struct {
        cleanup()
        return nil, nil, fmt.Errorf("expected struct, got %v", val.Kind())
    }

    // Copy each field to C memory at the correct offset
    for _, field := range structDef.Fields {
        zaField := val.FieldByName(field.Name)
        if !zaField.IsValid() {
            cleanup()
            return nil, nil, fmt.Errorf("Za struct missing field: %s", field.Name)
        }

        // Calculate C memory address for this field
        fieldPtr := unsafe.Pointer(uintptr(ptr) + field.Offset)

        // Marshal based on field type
        switch field.Type {
        case CInt:
            // Handle both int and uint types
            var intVal int64
            switch zaField.Kind() {
            case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
                intVal = zaField.Int()
            case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
                intVal = int64(zaField.Uint())
            default:
                cleanup()
                return nil, nil, fmt.Errorf("field %s: expected int/uint, got %v", field.Name, zaField.Kind())
            }
            *(*C.int)(fieldPtr) = C.int(intVal)

        case CFloat:
            if zaField.Kind() != reflect.Float32 && zaField.Kind() != reflect.Float64 {
                cleanup()
                return nil, nil, fmt.Errorf("field %s: expected float, got %v", field.Name, zaField.Kind())
            }
            *(*C.float)(fieldPtr) = C.float(zaField.Float())

        case CDouble:
            if zaField.Kind() != reflect.Float32 && zaField.Kind() != reflect.Float64 {
                cleanup()
                return nil, nil, fmt.Errorf("field %s: expected float, got %v", field.Name, zaField.Kind())
            }
            *(*C.double)(fieldPtr) = C.double(zaField.Float())

        case CBool:
            if zaField.Kind() != reflect.Bool {
                cleanup()
                return nil, nil, fmt.Errorf("field %s: expected bool, got %v", field.Name, zaField.Kind())
            }
            var boolVal C.int
            if zaField.Bool() {
                boolVal = 1
            } else {
                boolVal = 0
            }
            *(*C.int)(fieldPtr) = boolVal

        case CString:
            // Handle string field
            if zaField.Kind() != reflect.String {
                cleanup()
                return nil, nil, fmt.Errorf("field %s: expected string, got %v", field.Name, zaField.Kind())
            }
            str := zaField.String()
            cstr := C.CString(str)
            allocatedStrings = append(allocatedStrings, unsafe.Pointer(cstr))
            // Store the pointer in the struct field
            *(*unsafe.Pointer)(fieldPtr) = unsafe.Pointer(cstr)

        case CPointer:
            // Handle pointer field (CPointerValue)
            if ptrVal, ok := zaField.Interface().(*CPointerValue); ok {
                *(*unsafe.Pointer)(fieldPtr) = ptrVal.Ptr
            } else if zaField.Kind() == reflect.Ptr || zaField.Kind() == reflect.UnsafePointer {
                *(*unsafe.Pointer)(fieldPtr) = unsafe.Pointer(zaField.Pointer())
            } else {
                // For interface{}/any fields, try to extract pointer value
                *(*unsafe.Pointer)(fieldPtr) = nil
            }

        case CStruct:
            // Handle nested struct as pointer
            cleanup()
            return nil, nil, fmt.Errorf("nested struct fields not yet supported (field: %s)", field.Name)

        default:
            cleanup()
            return nil, nil, fmt.Errorf("unsupported field type %v for field %s", field.Type, field.Name)
        }
    }

    return ptr, cleanup, nil
}

// UnmarshalStructFromC creates a Za struct from C memory
// Returns the Za struct instance
func UnmarshalStructFromC(cPtr unsafe.Pointer, structDef *CLibraryStruct, zaStructName string) (any, error) {
    if cPtr == nil {
        return nil, fmt.Errorf("C pointer is nil")
    }

    if structDef == nil {
        return nil, fmt.Errorf("struct definition is nil")
    }

    // Build reflect.StructField array from C struct definition
    var sfields []reflect.StructField
    for _, field := range structDef.Fields {
        var fieldType reflect.Type

        // Map CType to Go reflect.Type
        switch field.Type {
        case CInt:
            fieldType = reflect.TypeOf(int(0))
        case CFloat:
            fieldType = reflect.TypeOf(float32(0))
        case CDouble:
            fieldType = reflect.TypeOf(float64(0))
        case CBool:
            fieldType = reflect.TypeOf(false)
        case CString:
            fieldType = reflect.TypeOf("")
        case CPointer:
            fieldType = reflect.TypeOf((*CPointerValue)(nil))
        case CStruct:
            // Nested struct as pointer
            fieldType = reflect.TypeOf((*CPointerValue)(nil))
        default:
            fieldType = reflect.TypeOf((*any)(nil)).Elem() // interface{}
        }

        sfields = append(sfields, reflect.StructField{
            Name: field.Name,
            Type: fieldType,
        })
    }

    // Create struct dynamically
    structType := reflect.StructOf(sfields)
    structVal := reflect.New(structType).Elem()

    // Read each field from C memory
    for i, field := range structDef.Fields {
        fieldPtr := unsafe.Pointer(uintptr(cPtr) + field.Offset)
        zaField := structVal.Field(i)

        switch field.Type {
        case CInt:
            cIntVal := *(*C.int)(fieldPtr)
            zaField.SetInt(int64(cIntVal))

        case CFloat:
            cFloatVal := *(*C.float)(fieldPtr)
            zaField.SetFloat(float64(cFloatVal))

        case CDouble:
            cDoubleVal := *(*C.double)(fieldPtr)
            zaField.SetFloat(float64(cDoubleVal))

        case CBool:
            cBoolVal := *(*C.int)(fieldPtr)
            zaField.SetBool(cBoolVal != 0)

        case CString:
            // Read char* pointer and convert to Go string
            cstr := *(*unsafe.Pointer)(fieldPtr)
            if cstr != nil {
                goStr := C.GoString((*C.char)(cstr))
                zaField.SetString(goStr)
            } else {
                zaField.SetString("")
            }

        case CPointer, CStruct:
            // Read pointer value
            ptr := *(*unsafe.Pointer)(fieldPtr)
            zaField.Set(reflect.ValueOf(&CPointerValue{Ptr: ptr, TypeTag: field.Name}))

        default:
            // Unknown type - leave as zero value
        }
    }

    return structVal.Interface(), nil
}
