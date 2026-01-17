//go:build !windows && !noffi && cgo
// +build !windows,!noffi,cgo

package main

/*
#include <stdlib.h>
#include <string.h>
#include <stdint.h>
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

    // Handle both Go structs and maps (structs from AUTO are returned as maps)
    val := reflect.ValueOf(zaStruct)
    if val.Kind() == reflect.Ptr {
        val = val.Elem()
    }

    // Check if it's a map or a struct
    isMap := val.Kind() == reflect.Map
    isStruct := val.Kind() == reflect.Struct

    if !isMap && !isStruct {
        cleanup()
        return nil, nil, fmt.Errorf("expected struct or map, got %v", val.Kind())
    }

    // Copy each field to C memory at the correct offset
    for _, field := range structDef.Fields {
        var zaField reflect.Value

        if isMap {
            // Access map by key (use original C field name)
            mapVal := val.MapIndex(reflect.ValueOf(field.Name))
            if !mapVal.IsValid() {
                cleanup()
                return nil, nil, fmt.Errorf("Za struct missing field: %s", field.Name)
            }
            zaField = mapVal
        } else {
            // Access struct by field name (use capitalized Go field name)
            goFieldName := renameSF(field.Name)
            zaField = val.FieldByName(goFieldName)
            if !zaField.IsValid() {
                cleanup()
                return nil, nil, fmt.Errorf("Za struct missing field: %s (Go field: %s)", field.Name, goFieldName)
            }
        }

        // Handle interface{} wrapped values from maps
        if zaField.Kind() == reflect.Interface {
            zaField = zaField.Elem()
        }

        // Calculate C memory address for this field
        fieldPtr := unsafe.Pointer(uintptr(ptr) + field.Offset)

        // Handle fixed-size arrays
        if field.ArraySize > 0 {
            // Za field should be a slice or array
            if zaField.Kind() != reflect.Slice && zaField.Kind() != reflect.Array {
                cleanup()
                return nil, nil, fmt.Errorf("field %s: expected slice/array for C array field, got %v", field.Name, zaField.Kind())
            }

            // Check array length
            arrayLen := zaField.Len()
            if arrayLen != field.ArraySize {
                cleanup()
                return nil, nil, fmt.Errorf("field %s: array length mismatch (expected %d, got %d)", field.Name, field.ArraySize, arrayLen)
            }

            // Copy each array element
            for i := 0; i < field.ArraySize; i++ {
                elemVal := zaField.Index(i)
                elemPtr := unsafe.Pointer(uintptr(fieldPtr) + uintptr(i)*getSizeForType(field.ElementType))

                // Unwrap interface{} values from maps
                if elemVal.Kind() == reflect.Interface {
                    elemVal = elemVal.Elem()
                }

                // Marshal element based on type
                if err := marshalElementToC(elemPtr, elemVal, field.ElementType, field.Name, &allocatedStrings); err != nil {
                    cleanup()
                    return nil, nil, err
                }
            }
            continue
        }

        // Marshal based on field type
        switch field.Type {
        case CInt, CUInt:
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

        case CInt8:
            var intVal int64
            switch zaField.Kind() {
            case reflect.Int, reflect.Int8:
                intVal = zaField.Int()
            default:
                cleanup()
                return nil, nil, fmt.Errorf("field %s: expected int8, got %v", field.Name, zaField.Kind())
            }
            *(*C.int8_t)(fieldPtr) = C.int8_t(intVal)

        case CUInt8, CChar:
            var uintVal uint64
            switch zaField.Kind() {
            case reflect.Uint, reflect.Uint8:
                uintVal = zaField.Uint()
            case reflect.Int, reflect.Int8:
                uintVal = uint64(zaField.Int())
            default:
                cleanup()
                return nil, nil, fmt.Errorf("field %s: expected uint8, got %v", field.Name, zaField.Kind())
            }
            *(*C.uint8_t)(fieldPtr) = C.uint8_t(uintVal)

        case CInt16:
            var intVal int64
            switch zaField.Kind() {
            case reflect.Int, reflect.Int16:
                intVal = zaField.Int()
            default:
                cleanup()
                return nil, nil, fmt.Errorf("field %s: expected int16, got %v", field.Name, zaField.Kind())
            }
            *(*C.int16_t)(fieldPtr) = C.int16_t(intVal)

        case CUInt16:
            var uintVal uint64
            switch zaField.Kind() {
            case reflect.Uint, reflect.Uint16:
                uintVal = zaField.Uint()
            default:
                cleanup()
                return nil, nil, fmt.Errorf("field %s: expected uint16, got %v", field.Name, zaField.Kind())
            }
            *(*C.uint16_t)(fieldPtr) = C.uint16_t(uintVal)

        case CInt64:
            var intVal int64
            switch zaField.Kind() {
            case reflect.Int, reflect.Int64:
                intVal = zaField.Int()
            default:
                cleanup()
                return nil, nil, fmt.Errorf("field %s: expected int64, got %v", field.Name, zaField.Kind())
            }
            *(*C.int64_t)(fieldPtr) = C.int64_t(intVal)

        case CUInt64:
            var uintVal uint64
            switch zaField.Kind() {
            case reflect.Uint, reflect.Uint64:
                uintVal = zaField.Uint()
            default:
                cleanup()
                return nil, nil, fmt.Errorf("field %s: expected uint64, got %v", field.Name, zaField.Kind())
            }
            *(*C.uint64_t)(fieldPtr) = C.uint64_t(uintVal)

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
            // Handle nested struct by value (recursive marshaling)
            if field.StructDef == nil {
                // No struct definition available - treat as opaque bytes
                // User can still pass raw byte data if needed
                continue
            }

            // Check if Za field is a struct instance (map[string]any)
            if zaField.Kind() != reflect.Map {
                // Not a map - might be nil or some other value
                // Just zero out the nested struct memory
                C.memset(fieldPtr, 0, C.size_t(field.StructDef.Size))
                continue
            }

            zaStruct, ok := zaField.Interface().(map[string]any)
            if !ok {
                // Not a string-keyed map - zero out
                C.memset(fieldPtr, 0, C.size_t(field.StructDef.Size))
                continue
            }

            // Recursively marshal the nested struct
            nestedPtr, nestedCleanup, err := MarshalStructToC(zaStruct, field.StructDef)
            if err != nil {
                cleanup()
                return nil, nil, fmt.Errorf("failed to marshal nested struct field %s: %w", field.Name, err)
            }

            // Copy the marshaled nested struct into the parent struct's memory
            C.memcpy(fieldPtr, nestedPtr, C.size_t(field.StructDef.Size))

            // Clean up the temporary nested struct
            nestedCleanup()


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

    // Return a map with original C field names (like unmarshalUnion does)
    result := make(map[string]any)

    // Read each field from C memory using sequential offsets
    for _, field := range structDef.Fields {
        fieldPtr := unsafe.Pointer(uintptr(cPtr) + field.Offset)

        // Handle fixed-size arrays
        if field.ArraySize > 0 {
            slice := make([]any, field.ArraySize)
            for j := 0; j < field.ArraySize; j++ {
                elemPtr := unsafe.Pointer(uintptr(fieldPtr) + uintptr(j)*getSizeForType(field.ElementType))

                var elemVal any
                switch field.ElementType {
                case CInt, CUInt:
                    elemVal = int(*(*C.int)(elemPtr))
                case CInt8:
                    elemVal = int8(*(*C.int8_t)(elemPtr))
                case CUInt8, CChar:
                    elemVal = uint8(*(*C.uint8_t)(elemPtr))
                case CInt16:
                    elemVal = int16(*(*C.int16_t)(elemPtr))
                case CUInt16:
                    elemVal = uint16(*(*C.uint16_t)(elemPtr))
                case CInt64:
                    elemVal = int64(*(*C.int64_t)(elemPtr))
                case CUInt64:
                    elemVal = uint64(*(*C.uint64_t)(elemPtr))
                case CFloat:
                    elemVal = float64(*(*C.float)(elemPtr))
                case CDouble:
                    elemVal = float64(*(*C.double)(elemPtr))
                case CBool:
                    elemVal = (*(*C.int)(elemPtr)) != 0
                default:
                    elemVal = nil
                }
                slice[j] = elemVal
            }
            result[field.Name] = slice
            continue
        }

        // Read single value
        var fieldVal any
        switch field.Type {
        case CInt:
            fieldVal = int(*(*C.int)(fieldPtr))
        case CUInt:
            fieldVal = uint(*(*C.uint)(fieldPtr))
        case CInt8:
            fieldVal = int8(*(*C.int8_t)(fieldPtr))
        case CUInt8, CChar:
            fieldVal = uint8(*(*C.uint8_t)(fieldPtr))
        case CInt16:
            fieldVal = int16(*(*C.int16_t)(fieldPtr))
        case CUInt16:
            fieldVal = uint16(*(*C.uint16_t)(fieldPtr))
        case CInt64:
            fieldVal = int64(*(*C.int64_t)(fieldPtr))
        case CUInt64:
            fieldVal = uint64(*(*C.uint64_t)(fieldPtr))
        case CFloat:
            fieldVal = float64(*(*C.float)(fieldPtr))
        case CDouble:
            fieldVal = float64(*(*C.double)(fieldPtr))
        case CBool:
            fieldVal = (*(*C.int)(fieldPtr)) != 0
        case CString:
            // Read char* pointer and convert to Go string
            cstr := *(*unsafe.Pointer)(fieldPtr)
            if cstr != nil {
                fieldVal = C.GoString((*C.char)(cstr))
            } else {
                fieldVal = ""
            }
        case CPointer:
            // Read pointer value
            ptr := *(*unsafe.Pointer)(fieldPtr)
            fieldVal = &CPointerValue{Ptr: ptr, TypeTag: field.Name}
        case CStruct:
            // Handle nested struct by value (recursive unmarshaling)
            if field.StructDef != nil && field.StructName != "" {
                // Recursively unmarshal the nested struct
                nestedStruct, err := UnmarshalStructFromC(fieldPtr, field.StructDef, field.StructName)
                if err != nil {
                    // If recursive unmarshal fails, fall back to pointer
                    fieldVal = &CPointerValue{Ptr: fieldPtr, TypeTag: field.Name}
                } else {
                    fieldVal = nestedStruct
                }
            } else {
                // No struct definition - return as pointer
                fieldVal = &CPointerValue{Ptr: fieldPtr, TypeTag: field.Name}
            }
        default:
            fieldVal = nil
        }

        result[field.Name] = fieldVal
    }

    return result, nil
}

// getSizeForType returns the size in bytes for a CType
func getSizeForType(ctype CType) uintptr {
    switch ctype {
    case CInt, CUInt, CFloat:
        return 4
    case CDouble, CInt64, CUInt64:
        return 8
    case CInt16, CUInt16:
        return 2
    case CInt8, CUInt8, CChar, CBool:
        return 1
    case CString, CPointer:
        return unsafe.Sizeof(uintptr(0))
    default:
        return unsafe.Sizeof(uintptr(0))
    }
}

// marshalElementToC marshals a single array element to C memory
func marshalElementToC(ptr unsafe.Pointer, val reflect.Value, ctype CType, fieldName string, allocatedStrings *[]unsafe.Pointer) error {
    switch ctype {
    case CInt, CUInt:
        var intVal int64
        switch val.Kind() {
        case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
            intVal = val.Int()
        case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
            intVal = int64(val.Uint())
        default:
            return fmt.Errorf("field %s: expected int/uint, got %v", fieldName, val.Kind())
        }
        *(*C.int)(ptr) = C.int(intVal)

    case CInt8:
        if val.Kind() != reflect.Int && val.Kind() != reflect.Int8 {
            return fmt.Errorf("field %s: expected int8, got %v", fieldName, val.Kind())
        }
        *(*C.int8_t)(ptr) = C.int8_t(val.Int())

    case CUInt8, CChar:
        var uintVal uint64
        switch val.Kind() {
        case reflect.Uint, reflect.Uint8:
            uintVal = val.Uint()
        case reflect.Int, reflect.Int8:
            uintVal = uint64(val.Int())
        default:
            return fmt.Errorf("field %s: expected uint8, got %v", fieldName, val.Kind())
        }
        *(*C.uint8_t)(ptr) = C.uint8_t(uintVal)

    case CInt16:
        if val.Kind() != reflect.Int && val.Kind() != reflect.Int16 {
            return fmt.Errorf("field %s: expected int16, got %v", fieldName, val.Kind())
        }
        *(*C.int16_t)(ptr) = C.int16_t(val.Int())

    case CUInt16:
        if val.Kind() != reflect.Uint && val.Kind() != reflect.Uint16 {
            return fmt.Errorf("field %s: expected uint16, got %v", fieldName, val.Kind())
        }
        *(*C.uint16_t)(ptr) = C.uint16_t(val.Uint())

    case CInt64:
        if val.Kind() != reflect.Int64 && val.Kind() != reflect.Int {
            return fmt.Errorf("field %s: expected int64, got %v", fieldName, val.Kind())
        }
        *(*C.int64_t)(ptr) = C.int64_t(val.Int())

    case CUInt64:
        if val.Kind() != reflect.Uint64 && val.Kind() != reflect.Uint {
            return fmt.Errorf("field %s: expected uint64, got %v", fieldName, val.Kind())
        }
        *(*C.uint64_t)(ptr) = C.uint64_t(val.Uint())

    case CFloat:
        if val.Kind() != reflect.Float32 && val.Kind() != reflect.Float64 {
            return fmt.Errorf("field %s: expected float, got %v", fieldName, val.Kind())
        }
        *(*C.float)(ptr) = C.float(val.Float())

    case CDouble:
        if val.Kind() != reflect.Float32 && val.Kind() != reflect.Float64 {
            return fmt.Errorf("field %s: expected double, got %v", fieldName, val.Kind())
        }
        *(*C.double)(ptr) = C.double(val.Float())

    case CBool:
        if val.Kind() != reflect.Bool {
            return fmt.Errorf("field %s: expected bool, got %v", fieldName, val.Kind())
        }
        var boolVal C.int
        if val.Bool() {
            boolVal = 1
        } else {
            boolVal = 0
        }
        *(*C.int)(ptr) = boolVal

    case CString:
        if val.Kind() != reflect.String {
            return fmt.Errorf("field %s: expected string, got %v", fieldName, val.Kind())
        }
        str := val.String()
        cstr := C.CString(str)
        *allocatedStrings = append(*allocatedStrings, unsafe.Pointer(cstr))
        *(*unsafe.Pointer)(ptr) = unsafe.Pointer(cstr)

    case CPointer:
        if ptrVal, ok := val.Interface().(*CPointerValue); ok {
            *(*unsafe.Pointer)(ptr) = ptrVal.Ptr
        } else {
            *(*unsafe.Pointer)(ptr) = nil
        }

    case CStruct:
        // CStruct marshaling is handled by the caller (marshalUnion/MarshalStructToC)
        // since we need the StructDef which isn't available here
        return fmt.Errorf("field %s: CStruct marshaling should be handled by caller, not marshalElementToC", fieldName)

    default:
        return fmt.Errorf("field %s: unsupported array element type %v", fieldName, ctype)
    }

    return nil
}

// marshalUnion marshals a Za map literal to C union memory
// Expects a map with exactly 1 key (the active field)
// Returns error if map has 0 or >1 keys, or if field name is invalid
func marshalUnion(zaMap map[string]any, unionDef *CLibraryStruct, ptr unsafe.Pointer, allocatedStrings *[]unsafe.Pointer) error {
    if unionDef == nil || !unionDef.IsUnion {
        return fmt.Errorf("invalid union definition")
    }

    // Validate map has exactly 1 field
    if len(zaMap) != 1 {
        return fmt.Errorf("union map must have exactly 1 field (got %d)", len(zaMap))
    }

    // Find the field in the map
    for fieldName, value := range zaMap {
        // Find field in union definition
        var field *StructField
        for i := range unionDef.Fields {
            if unionDef.Fields[i].Name == fieldName {
                field = &unionDef.Fields[i]
                break
            }
        }

        if field == nil {
            return fmt.Errorf("unknown union field: %s", fieldName)
        }

        // Marshal value to offset 0 (all union fields share memory)
        // Note: All union fields have offset=0
        fieldPtr := ptr // All fields write to same location

        // Handle arrays
        if field.ArraySize > 0 {
            // Value should be a slice or array
            val := reflect.ValueOf(value)
            if val.Kind() != reflect.Slice && val.Kind() != reflect.Array {
                return fmt.Errorf("union field %s: expected slice/array for C array field, got %T", fieldName, value)
            }

            arrayLen := val.Len()
            if arrayLen != field.ArraySize {
                return fmt.Errorf("union field %s: array length mismatch (expected %d, got %d)", fieldName, field.ArraySize, arrayLen)
            }

            // Copy each array element
            for i := 0; i < field.ArraySize; i++ {
                elemVal := val.Index(i)
                elemPtr := unsafe.Pointer(uintptr(fieldPtr) + uintptr(i)*getSizeForType(field.ElementType))
                if err := marshalElementToC(elemPtr, elemVal, field.ElementType, fieldName, allocatedStrings); err != nil {
                    return err
                }
            }
            return nil
        }

        // Marshal single value
        // Handle CStruct types specially (nested struct marshaling)
        if field.Type == CStruct && field.StructDef != nil {
            // Value should be a map[string]any
            zaStruct, ok := value.(map[string]any)
            if !ok {
                return fmt.Errorf("union field %s: expected map for struct, got %T", fieldName, value)
            }

            // Recursively marshal the nested struct
            nestedPtr, nestedCleanup, err := MarshalStructToC(zaStruct, field.StructDef)
            if err != nil {
                return fmt.Errorf("failed to marshal nested struct in union field %s: %w", fieldName, err)
            }

            // Copy the marshaled nested struct into the union memory (at offset 0)
            C.memcpy(fieldPtr, nestedPtr, C.size_t(field.StructDef.Size))

            // Clean up the temporary nested struct
            nestedCleanup()
            return nil
        }

        return marshalElementToC(fieldPtr, reflect.ValueOf(value), field.Type, fieldName, allocatedStrings)
    }

    return nil
}

// unmarshalUnion unmarshals C union memory to a Za map with ALL interpretations
// Returns a map where each key is a field name and value is that field's interpretation of the memory
func unmarshalUnion(ptr unsafe.Pointer, unionDef *CLibraryStruct) (map[string]any, error) {
    if unionDef == nil || !unionDef.IsUnion {
        return nil, fmt.Errorf("invalid union definition")
    }


    result := make(map[string]any)

    // Read each field from offset 0 (all fields overlap)
    for _, field := range unionDef.Fields {
        fieldPtr := ptr // All fields read from same location (offset 0)

        // Handle arrays
        if field.ArraySize > 0 {
            slice := make([]any, field.ArraySize)
            for i := 0; i < field.ArraySize; i++ {
                elemPtr := unsafe.Pointer(uintptr(fieldPtr) + uintptr(i)*getSizeForType(field.ElementType))

                var elemVal any
                switch field.ElementType {
                case CInt, CUInt:
                    elemVal = int(*(*C.int)(elemPtr))
                case CInt8:
                    elemVal = int8(*(*C.int8_t)(elemPtr))
                case CUInt8, CChar:
                    elemVal = uint8(*(*C.uint8_t)(elemPtr))
                case CInt16:
                    elemVal = int16(*(*C.int16_t)(elemPtr))
                case CUInt16:
                    elemVal = uint16(*(*C.uint16_t)(elemPtr))
                case CInt64:
                    elemVal = int64(*(*C.int64_t)(elemPtr))
                case CUInt64:
                    elemVal = uint64(*(*C.uint64_t)(elemPtr))
                case CFloat:
                    elemVal = float64(*(*C.float)(elemPtr))
                case CDouble:
                    elemVal = float64(*(*C.double)(elemPtr))
                case CBool:
                    elemVal = (*(*C.int)(elemPtr)) != 0
                default:
                    elemVal = nil
                }
                slice[i] = elemVal
            }
            result[field.Name] = slice
            continue
        }

        // Read single value
        var fieldVal any
        switch field.Type {
        case CInt:
            fieldVal = int(*(*C.int)(fieldPtr))
        case CUInt:
            fieldVal = uint(*(*C.uint)(fieldPtr))
        case CInt8:
            fieldVal = int8(*(*C.int8_t)(fieldPtr))
        case CUInt8, CChar:
            fieldVal = uint8(*(*C.uint8_t)(fieldPtr))
        case CInt16:
            fieldVal = int16(*(*C.int16_t)(fieldPtr))
        case CUInt16:
            fieldVal = uint16(*(*C.uint16_t)(fieldPtr))
        case CInt64:
            fieldVal = int64(*(*C.int64_t)(fieldPtr))
        case CUInt64:
            fieldVal = uint64(*(*C.uint64_t)(fieldPtr))
        case CFloat:
            fieldVal = float64(*(*C.float)(fieldPtr))
        case CDouble:
            fieldVal = float64(*(*C.double)(fieldPtr))
        case CBool:
            fieldVal = (*(*C.int)(fieldPtr)) != 0
        case CString:
            cstr := *(*unsafe.Pointer)(fieldPtr)
            if cstr != nil {
                fieldVal = C.GoString((*C.char)(cstr))
            } else {
                fieldVal = ""
            }
        case CPointer:
            fieldVal = *(*unsafe.Pointer)(fieldPtr)
        case CStruct:
            // Recursively unmarshal nested struct
            if field.StructDef != nil {
                var err error
                fieldVal, err = UnmarshalStructFromC(fieldPtr, field.StructDef, field.StructName)
                if err != nil {
                    // Fall back to raw pointer on error
                    fieldVal = fieldPtr
                }
            } else {
                // No struct definition, return raw pointer
                fieldVal = fieldPtr
            }
        default:
            fieldVal = nil
        }

        result[field.Name] = fieldVal
    }

    return result, nil
}
