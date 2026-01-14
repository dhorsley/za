//go:build !windows && !noffi && cgo
// +build !windows,!noffi,cgo

package main

/*
#include <dlfcn.h>
#include <stdlib.h>
#include <stdio.h>

// No #include <ffi.h> - we load symbols dynamically!

// libffi type definitions (match libffi's actual structs)
typedef enum {
    FFI_OK = 0,
    FFI_BAD_TYPEDEF,
    FFI_BAD_ABI
} ffi_status;

typedef enum {
    FFI_SYSV = 1,
    FFI_UNIX64 = 2,
    FFI_DEFAULT_ABI = 2  // On x86-64 Linux, use Unix64 ABI
} ffi_abi;

// Forward declarations (actual structs are opaque)
typedef struct ffi_type_s ffi_type;
typedef struct ffi_cif_s ffi_cif;

// External type descriptors (provided by libffi)
// These are loaded via dlsym
static ffi_type* libffi_type_void;
static ffi_type* libffi_type_uint8;
static ffi_type* libffi_type_sint8;
static ffi_type* libffi_type_uint16;
static ffi_type* libffi_type_sint16;
static ffi_type* libffi_type_uint32;
static ffi_type* libffi_type_sint32;
static ffi_type* libffi_type_uint64;
static ffi_type* libffi_type_sint64;
static ffi_type* libffi_type_float;
static ffi_type* libffi_type_double;
static ffi_type* libffi_type_pointer;
static ffi_type* libffi_type_longdouble;

// Function pointer types for libffi functions
// Use void* for all parameters to match actual dlsym signatures
typedef ffi_status (*ffi_prep_cif_func)(void *cif,
                                         ffi_abi abi,
                                         unsigned int nargs,
                                         void *rtype,
                                         void **atypes);

typedef void (*ffi_call_func)(void *cif,
                               void *fn,
                               void *rvalue,
                               void **avalue);

typedef ffi_status (*ffi_prep_cif_var_func)(ffi_cif *cif,
                                             ffi_abi abi,
                                             unsigned int nfixedargs,
                                             unsigned int ntotalargs,
                                             ffi_type *rtype,
                                             ffi_type **atypes);

// Global function pointers (loaded from libffi.so)
static ffi_prep_cif_func libffi_prep_cif = NULL;
static ffi_call_func libffi_call = NULL;
static ffi_prep_cif_var_func libffi_prep_cif_var = NULL;

// libffi handle
static void* libffi_handle = NULL;

// Load libffi dynamically
static int load_libffi(void) {
    if (libffi_handle != NULL) {
        return 1; // Already loaded
    }

    // Try common paths for libffi
    const char* paths[] = {
        "libffi.so.8",                           // Generic
        "/usr/lib/x86_64-linux-gnu/libffi.so.8", // Debian/Ubuntu
        "/usr/lib64/libffi.so.8",                // RHEL/Fedora
        "/usr/lib/libffi.so.8",                  // Arch/Alpine
        "/usr/local/lib/libffi.so.8",            // FreeBSD
        "libffi.so.7",                           // Older systems
        "libffi.so.6",                           // Very old systems
        NULL
    };

    for (int i = 0; paths[i] != NULL; i++) {
        libffi_handle = dlopen(paths[i], RTLD_LAZY | RTLD_LOCAL);
        if (libffi_handle != NULL) {
            break;
        }
    }

    if (libffi_handle == NULL) {
        return 0; // Failed to load
    }

    // Load function symbols
    libffi_prep_cif = (ffi_prep_cif_func)dlsym(libffi_handle, "ffi_prep_cif");
    libffi_call = (ffi_call_func)dlsym(libffi_handle, "ffi_call");
    libffi_prep_cif_var = (ffi_prep_cif_var_func)dlsym(libffi_handle, "ffi_prep_cif_var");

    // Load type descriptors
    libffi_type_void = (ffi_type*)dlsym(libffi_handle, "ffi_type_void");
    libffi_type_uint8 = (ffi_type*)dlsym(libffi_handle, "ffi_type_uint8");
    libffi_type_sint8 = (ffi_type*)dlsym(libffi_handle, "ffi_type_sint8");
    libffi_type_uint16 = (ffi_type*)dlsym(libffi_handle, "ffi_type_uint16");
    libffi_type_sint16 = (ffi_type*)dlsym(libffi_handle, "ffi_type_sint16");
    libffi_type_uint32 = (ffi_type*)dlsym(libffi_handle, "ffi_type_uint32");
    libffi_type_sint32 = (ffi_type*)dlsym(libffi_handle, "ffi_type_sint32");
    libffi_type_uint64 = (ffi_type*)dlsym(libffi_handle, "ffi_type_uint64");
    libffi_type_sint64 = (ffi_type*)dlsym(libffi_handle, "ffi_type_sint64");
    libffi_type_float = (ffi_type*)dlsym(libffi_handle, "ffi_type_float");
    libffi_type_double = (ffi_type*)dlsym(libffi_handle, "ffi_type_double");
    libffi_type_pointer = (ffi_type*)dlsym(libffi_handle, "ffi_type_pointer");
    libffi_type_longdouble = (ffi_type*)dlsym(libffi_handle, "ffi_type_longdouble");

    // Verify all symbols loaded
    if (libffi_prep_cif == NULL || libffi_call == NULL ||
        libffi_type_void == NULL || libffi_type_pointer == NULL) {
        dlclose(libffi_handle);
        libffi_handle = NULL;
        return 0;
    }

    return 1; // Success
}

// Check if libffi is available
static int is_libffi_available(void) {
    return libffi_handle != NULL;
}

// Generic FFI call wrapper
static int call_via_libffi(
    void* fn_ptr,
    int n_args,
    int* arg_types,      // Za CType enum values
    void** arg_values,   // Pointers to actual argument values
    int return_type,     // Za CType enum value
    void* return_value,  // Pointer to return value storage
    int is_variadic,     // 1 if variadic function, 0 otherwise
    int n_fixed_args     // Number of fixed arguments (for variadic functions)
) {
    if (!is_libffi_available()) {
        return -1; // libffi not loaded
    }

    // Allocate ffi_cif on heap with proper alignment
    ffi_cif* cif = (ffi_cif*)malloc(128);
    if (cif == NULL) {
        return -6; // Memory allocation failed
    }

    // Allocate argument type array (use malloc for safety with any n_args)
    ffi_type** ffi_arg_types = NULL;
    if (n_args > 0) {
        ffi_arg_types = (ffi_type**)malloc(sizeof(ffi_type*) * n_args);
        if (ffi_arg_types == NULL) {
            free(cif);
            return -4; // Memory allocation failed
        }
    }

    // Map Za types to libffi types
    for (int i = 0; i < n_args; i++) {
        switch (arg_types[i]) {
            case 0: // CVoid (shouldn't happen for args)
                ffi_arg_types[i] = libffi_type_void;
                break;
            case 1: // CInt
                ffi_arg_types[i] = libffi_type_sint32;
                break;
            case 2: // CFloat
                ffi_arg_types[i] = libffi_type_float;
                break;
            case 3: // CDouble
                ffi_arg_types[i] = libffi_type_double;
                break;
            case 4: // CChar
                ffi_arg_types[i] = libffi_type_sint8;
                break;
            case 5: // CString (pointer)
                ffi_arg_types[i] = libffi_type_pointer;
                break;
            case 6: // CBool
                ffi_arg_types[i] = libffi_type_uint8;
                break;
            case 7: // CPointer
                ffi_arg_types[i] = libffi_type_pointer;
                break;
            case 8: // CStruct (pointer)
                ffi_arg_types[i] = libffi_type_pointer;
                break;
            case 9: // CUInt
                ffi_arg_types[i] = libffi_type_uint32;
                break;
            case 10: // CInt16
                ffi_arg_types[i] = libffi_type_sint16;
                break;
            case 11: // CUInt16
                ffi_arg_types[i] = libffi_type_uint16;
                break;
            case 12: // CInt64
                ffi_arg_types[i] = libffi_type_sint64;
                break;
            case 13: // CUInt64
                ffi_arg_types[i] = libffi_type_uint64;
                break;
            case 14: // CLongDouble
                ffi_arg_types[i] = libffi_type_longdouble;
                break;
            case 15: // CInt8
                ffi_arg_types[i] = libffi_type_sint8;
                break;
            case 16: // CUInt8
                ffi_arg_types[i] = libffi_type_uint8;
                break;
            default:
                if (ffi_arg_types != NULL) free(ffi_arg_types);
                free(cif);
                return -2; // Unknown type
        }
    }

    // Map return type
    ffi_type* ffi_return_type;
    switch (return_type) {
        case 0: // CVoid
            ffi_return_type = libffi_type_void;
            break;
        case 1: // CInt
            ffi_return_type = libffi_type_sint32;
            break;
        case 2: // CFloat
            ffi_return_type = libffi_type_float;
            break;
        case 3: // CDouble
            ffi_return_type = libffi_type_double;
            break;
        case 4: // CChar
            ffi_return_type = libffi_type_sint8;
            break;
        case 5: // CString (pointer)
            ffi_return_type = libffi_type_pointer;
            break;
        case 6: // CBool
            ffi_return_type = libffi_type_uint8;
            break;
        case 7: // CPointer
            ffi_return_type = libffi_type_pointer;
            break;
        case 8: // CStruct (pointer)
            ffi_return_type = libffi_type_pointer;
            break;
        case 9: // CUInt
            ffi_return_type = libffi_type_uint32;
            break;
        case 10: // CInt16
            ffi_return_type = libffi_type_sint16;
            break;
        case 11: // CUInt16
            ffi_return_type = libffi_type_uint16;
            break;
        case 12: // CInt64
            ffi_return_type = libffi_type_sint64;
            break;
        case 13: // CUInt64
            ffi_return_type = libffi_type_uint64;
            break;
        case 14: // CLongDouble
            ffi_return_type = libffi_type_longdouble;
            break;
        case 15: // CInt8
            ffi_return_type = libffi_type_sint8;
            break;
        case 16: // CUInt8
            ffi_return_type = libffi_type_uint8;
            break;
        default:
            if (ffi_arg_types != NULL) free(ffi_arg_types);
            free(cif);
            return -2; // Unknown type
    }

    // Prepare call interface
    ffi_status status;
    if (is_variadic) {
        // Use ffi_prep_cif_var for variadic functions
        if (libffi_prep_cif_var == NULL) {
            fprintf(stderr, "ERROR: ffi_prep_cif_var not available\n");
            if (ffi_arg_types != NULL) free(ffi_arg_types);
            free(cif);
            return -7; // ffi_prep_cif_var not available
        }
        status = libffi_prep_cif_var(cif, FFI_DEFAULT_ABI, n_fixed_args, n_args,
                                      ffi_return_type, ffi_arg_types);
        if (status != FFI_OK) {
            fprintf(stderr, "ERROR: ffi_prep_cif_var failed with status %d (n_fixed=%d, n_total=%d)\n",
                   status, n_fixed_args, n_args);
            if (ffi_arg_types != NULL) free(ffi_arg_types);
            free(cif);
            return -3; // prep_cif_var failed
        }
    } else {
        // Use ffi_prep_cif for non-variadic functions
        status = libffi_prep_cif(cif, FFI_DEFAULT_ABI, n_args,
                                 (void*)ffi_return_type, (void**)ffi_arg_types);
        if (status != FFI_OK) {
            fprintf(stderr, "ERROR: ffi_prep_cif failed with status %d (n_args=%d)\n", status, n_args);
            if (ffi_arg_types != NULL) free(ffi_arg_types);
            free(cif);
            return -3; // prep_cif failed
        }
    }

    // Make the call
    if (libffi_call == NULL) {
        fprintf(stderr, "ERROR: libffi_call is NULL!\n");
        if (ffi_arg_types != NULL) free(ffi_arg_types);
        free(cif);
        return -5;
    }

    // Make the FFI call
    libffi_call(cif, fn_ptr, return_value, arg_values);

    // Cleanup
    if (ffi_arg_types != NULL) free(ffi_arg_types);
    free(cif);

    return 0; // Success
}
*/
import "C"
import (
    "fmt"
    "unsafe"
)

var libffiAvailable bool = false
var libffiChecked bool = false

// InitLibFFI attempts to load libffi dynamically
func InitLibFFI() bool {
    if libffiChecked {
        return libffiAvailable
    }
    libffiChecked = true

    result := C.load_libffi()
    libffiAvailable = (result == 1)

    return libffiAvailable
}

// IsLibFFIAvailable checks if libffi was successfully loaded
func IsLibFFIAvailable() bool {
    if !libffiChecked {
        InitLibFFI()
    }
    return libffiAvailable
}

// CallCFunctionViaLibFFI calls a C function using libffi
func CallCFunctionViaLibFFI(funcPtr unsafe.Pointer, funcName string, args []any, sig CFunctionSignature) (any, error) {
    if !IsLibFFIAvailable() {
        return nil, fmt.Errorf("libffi not available")
    }

    expectedRetType := sig.ReturnType

    // For variadic functions, validate minimum argument count
    if sig.HasVarargs {
        if len(args) < sig.FixedArgCount {
            return nil, fmt.Errorf("variadic function %s requires at least %d arguments, got %d",
                funcName, sig.FixedArgCount, len(args))
        }
    }

    // Handle zero-arg functions
    if len(args) == 0 {
        var returnValue C.longlong
        isVariadic := 0
        if sig.HasVarargs {
            isVariadic = 1
        }
        result := C.call_via_libffi(
            funcPtr,
            C.int(0),
            nil,
            nil,
            C.int(expectedRetType),
            unsafe.Pointer(&returnValue),
            C.int(isVariadic),
            C.int(0), // n_fixed_args (0 for zero-arg functions)
        )

        if result != 0 {
            return nil, fmt.Errorf("libffi call failed with code %d", result)
        }

        return convertReturnValue(returnValue, sig)
    }

    // Convert Za arguments to C values with type checking and range validation
    convertedArgs := make([]any, len(args))
    for i, arg := range args {
        var expectedType CType
        if i < len(sig.ParamTypes) {
            expectedType = sig.ParamTypes[i]
        } else {
            // Variadic argument - infer type from value
            switch arg.(type) {
            case int:
                expectedType = CInt
            case uint:
                expectedType = CUInt
            case float64:
                expectedType = CDouble
            case string:
                expectedType = CString
            case bool:
                expectedType = CBool
            case *CPointerValue:
                expectedType = CPointer
            default:
                expectedType = CInt // Default fallback
            }
        }

        // Convert with range validation
        converted, err := ConvertZaToCValue(arg, expectedType)
        if err != nil {
            return nil, fmt.Errorf("argument %d: %v", i, err)
        }
        convertedArgs[i] = converted
    }

    // Build argument type and value arrays
    // Allocate C memory for argument type array
    argTypes := (*C.int)(C.malloc(C.size_t(len(convertedArgs)) * C.size_t(unsafe.Sizeof(C.int(0)))))
    defer C.free(unsafe.Pointer(argTypes))

    // Allocate C memory for argument value pointer array
    argValues := (*unsafe.Pointer)(C.malloc(C.size_t(len(convertedArgs)) * C.size_t(unsafe.Sizeof(unsafe.Pointer(nil)))))
    defer C.free(unsafe.Pointer(argValues))

    // Temporary storage for converted arguments (must stay alive during call)
    var cstrings []unsafe.Pointer
    var allocatedMem []unsafe.Pointer

    defer func() {
        for _, cs := range cstrings {
            C.free(cs)
        }
        for _, mem := range allocatedMem {
            C.free(mem)
        }
    }()

    // Convert to slices for easier indexing
    argTypesSlice := (*[1 << 30]C.int)(unsafe.Pointer(argTypes))[:len(convertedArgs):len(convertedArgs)]
    argValuesSlice := (*[1 << 30]unsafe.Pointer)(unsafe.Pointer(argValues))[:len(convertedArgs):len(convertedArgs)]

    for i, arg := range convertedArgs {
        switch v := arg.(type) {
        case int:
            argTypesSlice[i] = 1 // CInt
            // Allocate C memory for int value
            intPtr := C.malloc(C.size_t(unsafe.Sizeof(C.int(0))))
            allocatedMem = append(allocatedMem, intPtr)
            *(*C.int)(intPtr) = C.int(v)
            argValuesSlice[i] = intPtr

        case uint:
            argTypesSlice[i] = 9 // CUInt
            // Allocate C memory for uint value
            uintPtr := C.malloc(C.size_t(unsafe.Sizeof(C.uint(0))))
            allocatedMem = append(allocatedMem, uintPtr)
            *(*C.uint)(uintPtr) = C.uint(v)
            argValuesSlice[i] = uintPtr

        case int16:
            argTypesSlice[i] = 10 // CInt16
            // Allocate C memory for int16 value
            int16Ptr := C.malloc(C.size_t(unsafe.Sizeof(C.short(0))))
            allocatedMem = append(allocatedMem, int16Ptr)
            *(*C.short)(int16Ptr) = C.short(v)
            argValuesSlice[i] = int16Ptr

        case uint16:
            argTypesSlice[i] = 11 // CUInt16
            // Allocate C memory for uint16 value
            uint16Ptr := C.malloc(C.size_t(unsafe.Sizeof(C.ushort(0))))
            allocatedMem = append(allocatedMem, uint16Ptr)
            *(*C.ushort)(uint16Ptr) = C.ushort(v)
            argValuesSlice[i] = uint16Ptr

        case int64:
            argTypesSlice[i] = 12 // CInt64
            // Allocate C memory for int64 value
            int64Ptr := C.malloc(C.size_t(unsafe.Sizeof(C.longlong(0))))
            allocatedMem = append(allocatedMem, int64Ptr)
            *(*C.longlong)(int64Ptr) = C.longlong(v)
            argValuesSlice[i] = int64Ptr

        case uint64:
            argTypesSlice[i] = 13 // CUInt64
            // Allocate C memory for uint64 value
            uint64Ptr := C.malloc(C.size_t(unsafe.Sizeof(C.ulonglong(0))))
            allocatedMem = append(allocatedMem, uint64Ptr)
            *(*C.ulonglong)(uint64Ptr) = C.ulonglong(v)
            argValuesSlice[i] = uint64Ptr

        case int8:
            argTypesSlice[i] = 15 // CInt8
            // Allocate C memory for int8 value
            int8Ptr := C.malloc(C.size_t(unsafe.Sizeof(C.char(0))))
            allocatedMem = append(allocatedMem, int8Ptr)
            *(*C.char)(int8Ptr) = C.char(v)
            argValuesSlice[i] = int8Ptr

        case uint8:
            argTypesSlice[i] = 16 // CUInt8
            // Allocate C memory for uint8 value
            uint8Ptr := C.malloc(C.size_t(unsafe.Sizeof(C.uchar(0))))
            allocatedMem = append(allocatedMem, uint8Ptr)
            *(*C.uchar)(uint8Ptr) = C.uchar(v)
            argValuesSlice[i] = uint8Ptr

        case float64:
            argTypesSlice[i] = 3 // CDouble
            // Allocate C memory for double value
            dblPtr := C.malloc(C.size_t(unsafe.Sizeof(C.double(0))))
            allocatedMem = append(allocatedMem, dblPtr)
            *(*C.double)(dblPtr) = C.double(v)
            argValuesSlice[i] = dblPtr

        case string:
            argTypesSlice[i] = 5 // CString
            cstr := C.CString(v)
            cstrings = append(cstrings, unsafe.Pointer(cstr))
            // Allocate C memory for pointer to string
            ptrPtr := C.malloc(C.size_t(unsafe.Sizeof(unsafe.Pointer(nil))))
            allocatedMem = append(allocatedMem, ptrPtr)
            *(*unsafe.Pointer)(ptrPtr) = unsafe.Pointer(cstr)
            argValuesSlice[i] = ptrPtr

        case bool:
            argTypesSlice[i] = 6 // CBool
            // Allocate C memory for bool value
            boolPtr := C.malloc(C.size_t(unsafe.Sizeof(C.uchar(0))))
            allocatedMem = append(allocatedMem, boolPtr)
            if v {
                *(*C.uchar)(boolPtr) = 1
            } else {
                *(*C.uchar)(boolPtr) = 0
            }
            argValuesSlice[i] = boolPtr

        case *CPointerValue:
            argTypesSlice[i] = 7 // CPointer
            // v.Ptr is the buffer address (like &db in the C test where db is the result variable)
            // libffi needs: arg_values[i] points to the argument VALUE
            // The argument value for a void** parameter IS the address (&db)
            // So we just need to store v.Ptr and point to it
            ptrPtr := C.malloc(C.size_t(unsafe.Sizeof(unsafe.Pointer(nil))))
            allocatedMem = append(allocatedMem, ptrPtr)
            *(*unsafe.Pointer)(ptrPtr) = v.Ptr
            argValuesSlice[i] = ptrPtr

        default:
            // Check if this is a struct type that needs marshaling
            // Get the struct type name from signature if available
            var structTypeName string
            if i < len(sig.ParamStructNames) {
                structTypeName = sig.ParamStructNames[i]
            }

            if structTypeName != "" {
                // This is a typed struct - marshal to C memory
                argTypesSlice[i] = 8 // CStruct

                // Get C struct layout from Za struct definition
                structDef, err := getStructLayoutFromZa(structTypeName)
                if err != nil {
                    return nil, fmt.Errorf("argument %d: failed to get struct layout for %s: %v", i, structTypeName, err)
                }

                // Marshal Za struct to C memory
                cPtr, cleanup, err := MarshalStructToC(arg, structDef)
                if err != nil {
                    return nil, fmt.Errorf("argument %d: failed to marshal struct: %v", i, err)
                }

                // Clean up allocated C memory after the FFI call completes
                // cleanup() will free the struct memory and any allocated strings
                defer cleanup()

                // Store pointer for libffi (don't add cPtr to allocatedMem, cleanup() handles it)
                ptrPtr := C.malloc(C.size_t(unsafe.Sizeof(unsafe.Pointer(nil))))
                allocatedMem = append(allocatedMem, ptrPtr)
                *(*unsafe.Pointer)(ptrPtr) = cPtr
                argValuesSlice[i] = ptrPtr
            } else {
                return nil, fmt.Errorf("unsupported argument type: %T", arg)
            }
        }
    }

    // Prepare return value storage
    var returnValue C.longlong

    // Determine if variadic and fixed args count
    isVariadic := 0
    nFixedArgs := len(args)
    if sig.HasVarargs {
        isVariadic = 1
        nFixedArgs = sig.FixedArgCount
    }

    // Call via libffi
    result := C.call_via_libffi(
        funcPtr,
        C.int(len(args)),
        argTypes,
        (*unsafe.Pointer)(unsafe.Pointer(argValues)),
        C.int(expectedRetType),
        unsafe.Pointer(&returnValue),
        C.int(isVariadic),
        C.int(nFixedArgs),
    )

    if result != 0 {
        return nil, fmt.Errorf("libffi call failed with code %d", result)
    }

    return convertReturnValue(returnValue, sig)
}

// convertReturnValue converts C return value to Za type
func convertReturnValue(returnValue C.longlong, sig CFunctionSignature) (any, error) {
    expectedRetType := sig.ReturnType

    switch expectedRetType {
    case CVoid:
        return nil, nil

    case CInt:
        return int(*(*C.int)(unsafe.Pointer(&returnValue))), nil

    case CUInt:
        return int(*(*C.uint)(unsafe.Pointer(&returnValue))), nil

    case CInt16:
        return int(*(*C.short)(unsafe.Pointer(&returnValue))), nil

    case CUInt16:
        return int(*(*C.ushort)(unsafe.Pointer(&returnValue))), nil

    case CInt64:
        return int(*(*C.longlong)(unsafe.Pointer(&returnValue))), nil

    case CUInt64:
        return int(*(*C.ulonglong)(unsafe.Pointer(&returnValue))), nil

    case CLongDouble:
        // Note: Go doesn't have native long double support
        // libffi stores it in returnValue, we read it as double (may lose precision)
        return float64(*(*C.double)(unsafe.Pointer(&returnValue))), nil

    case CInt8:
        return int(*(*C.char)(unsafe.Pointer(&returnValue))), nil

    case CUInt8:
        return uint8(*(*C.uchar)(unsafe.Pointer(&returnValue))), nil

    case CDouble:
        return float64(*(*C.double)(unsafe.Pointer(&returnValue))), nil

    case CString:
        cstr := *(*unsafe.Pointer)(unsafe.Pointer(&returnValue))
        if cstr == nil {
            return "", nil
        }
        return C.GoString((*C.char)(cstr)), nil

    case CBool:
        bVal := *(*C.uchar)(unsafe.Pointer(&returnValue))
        return bVal != 0, nil

    case CPointer:
        ptr := *(*unsafe.Pointer)(unsafe.Pointer(&returnValue))
        return &CPointerValue{Ptr: ptr}, nil

    case CStruct:
        ptr := *(*unsafe.Pointer)(unsafe.Pointer(&returnValue))
        if ptr == nil {
            return nil, nil
        }

        if sig.ReturnStructName != "" {
            // Typed struct - unmarshal from C memory
            structDef, err := getStructLayoutFromZa(sig.ReturnStructName)
            if err != nil {
                return nil, fmt.Errorf("failed to get struct layout for %s: %v", sig.ReturnStructName, err)
            }

            // Unmarshal C memory to Za struct
            return UnmarshalStructFromC(ptr, structDef, sig.ReturnStructName)
        } else {
            // Generic struct - return as opaque pointer
            return &CPointerValue{Ptr: ptr}, nil
        }

    default:
        return nil, fmt.Errorf("unsupported return type: %d", expectedRetType)
    }
}
