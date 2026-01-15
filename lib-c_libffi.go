//go:build !windows && !noffi && cgo
// +build !windows,!noffi,cgo

package main

/*
#include <dlfcn.h>
#include <stdlib.h>
#include <stdio.h>
#include <string.h>

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
    FFI_WIN64 = 4,
    FFI_DEFAULT_ABI = 2  // Will be overridden by get_platform_abi()
} ffi_abi;

// Platform and architecture-specific ABI detection
// Returns the appropriate ABI value for the current platform
static ffi_abi detected_abi = 0;

static ffi_abi get_platform_abi(const char* arch, const char* os) {
    // Cache the result
    if (detected_abi != 0) {
        return detected_abi;
    }

    // x86-64 / amd64
    if (strcmp(arch, "amd64") == 0) {
        detected_abi = FFI_UNIX64;  // Unix64 for all 64-bit Unix-like systems
        return detected_abi;
    }

    // ARM64 / aarch64
    if (strcmp(arch, "arm64") == 0) {
        detected_abi = FFI_UNIX64;  // ARM64 also uses Unix64 ABI on Unix systems
        return detected_abi;
    }

    // 32-bit x86
    if (strcmp(arch, "386") == 0) {
        detected_abi = FFI_SYSV;  // 32-bit x86 uses SYSV ABI
        return detected_abi;
    }

    // ARM 32-bit
    if (strcmp(arch, "arm") == 0) {
        detected_abi = FFI_SYSV;  // 32-bit ARM uses SYSV ABI
        return detected_abi;
    }

    // RISC-V 64-bit
    if (strcmp(arch, "riscv64") == 0) {
        detected_abi = FFI_UNIX64;  // RISC-V 64 uses Unix64 ABI
        return detected_abi;
    }

    // PowerPC 64-bit
    if (strcmp(arch, "ppc64") == 0 || strcmp(arch, "ppc64le") == 0) {
        detected_abi = FFI_UNIX64;  // PPC64 uses Unix64 ABI
        return detected_abi;
    }

    // Default fallback based on pointer size
    // If we don't recognize the architecture, use pointer size as heuristic
    if (sizeof(void*) == 8) {
        detected_abi = FFI_UNIX64;  // 64-bit pointer = Unix64 ABI
    } else {
        detected_abi = FFI_SYSV;    // 32-bit pointer = SYSV ABI
    }

    return detected_abi;
}

// Forward declarations
typedef struct ffi_type_s ffi_type;

// ffi_cif structure (from libffi) - needed for closure handler
typedef struct ffi_cif_s {
    ffi_abi abi;
    unsigned int nargs;
    ffi_type **arg_types;
    ffi_type *rtype;
    unsigned int bytes;
    unsigned int flags;
} ffi_cif;

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

// Closure API function pointers (for dynamic callback support)
// These enable runtime-generated callbacks with arbitrary signatures
static void* (*libffi_closure_alloc)(size_t size, void** code) = NULL;
static void (*libffi_closure_free)(void* closure) = NULL;
static ffi_status (*libffi_prep_closure_loc)(void* closure, void* cif, void (*fun)(void*, void*, void**, void*), void* user_data, void* codeloc) = NULL;

// libffi handle
static void* libffi_handle = NULL;

// Load libffi dynamically
static int load_libffi(void) {
    if (libffi_handle != NULL) {
        return 1; // Already loaded
    }

    // Try common paths for libffi
    // Priority order: generic names first (let system find it), then specific paths
    const char* paths[] = {
        // Generic names (system ld.so will search standard paths)
        "libffi.so.8",                           // Generic, version 8
        "libffi.so.7",                           // Generic, version 7
        "libffi.so.6",                           // Generic, version 6
        "libffi.so",                             // Generic, unversioned (BSD)

        // Linux-specific paths
        "/usr/lib/x86_64-linux-gnu/libffi.so.8", // Debian/Ubuntu x86_64
        "/usr/lib/aarch64-linux-gnu/libffi.so.8",// Debian/Ubuntu ARM64
        "/usr/lib64/libffi.so.8",                // RHEL/Fedora/CentOS
        "/usr/lib/libffi.so.8",                  // Arch/Alpine/Gentoo
        "/usr/lib/libffi.so",                    // Arch/Alpine unversioned

        // FreeBSD paths
        "/usr/local/lib/libffi.so.8",            // FreeBSD ports (versioned)
        "/usr/local/lib/libffi.so.7",            // FreeBSD ports (older)
        "/usr/local/lib/libffi.so",              // FreeBSD ports (unversioned)
        "/usr/lib/libffi.so",                    // FreeBSD base (if exists)

        // OpenBSD paths
        "/usr/local/lib/libffi.so",              // OpenBSD ports (unversioned)

        // NetBSD paths
        "/usr/pkg/lib/libffi.so.8",              // NetBSD pkgsrc (versioned)
        "/usr/pkg/lib/libffi.so",                // NetBSD pkgsrc (unversioned)

        // Additional fallback paths
        "/lib/libffi.so.8",                      // Some minimal systems
        "/lib64/libffi.so.8",                    // Some minimal systems

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

    // Load closure API symbols (optional - for dynamic callback support)
    // If these are unavailable, we'll still work with hardcoded trampolines
    libffi_closure_alloc = dlsym(libffi_handle, "ffi_closure_alloc");
    libffi_closure_free = dlsym(libffi_handle, "ffi_closure_free");
    libffi_prep_closure_loc = dlsym(libffi_handle, "ffi_prep_closure_loc");

    // Note: We don't fail if closure API is unavailable - it's optional
    // Dynamic callbacks will fall back to "not supported" error if needed

    // Verify all symbols loaded
    if (libffi_prep_cif == NULL || libffi_call == NULL ||
        libffi_type_void == NULL || libffi_type_pointer == NULL) {
        dlclose(libffi_handle);
        libffi_handle = NULL;
        return 0;
    }

    return 1; // Success
}

// Initialize ABI detection (called from Go with runtime.GOARCH and runtime.GOOS)
static void init_platform_abi(const char* arch, const char* os) {
    get_platform_abi(arch, os);
}

// Check if libffi is available
static int is_libffi_available(void) {
    return libffi_handle != NULL;
}

// Type accessor functions - allow Go code to get FFI type pointers
static void* get_ffi_type_void(void) { return libffi_type_void; }
static void* get_ffi_type_uint8(void) { return libffi_type_uint8; }
static void* get_ffi_type_sint8(void) { return libffi_type_sint8; }
static void* get_ffi_type_uint16(void) { return libffi_type_uint16; }
static void* get_ffi_type_sint16(void) { return libffi_type_sint16; }
static void* get_ffi_type_uint32(void) { return libffi_type_uint32; }
static void* get_ffi_type_sint32(void) { return libffi_type_sint32; }
static void* get_ffi_type_uint64(void) { return libffi_type_uint64; }
static void* get_ffi_type_sint64(void) { return libffi_type_sint64; }
static void* get_ffi_type_float(void) { return libffi_type_float; }
static void* get_ffi_type_double(void) { return libffi_type_double; }
static void* get_ffi_type_pointer(void) { return libffi_type_pointer; }

// Forward declaration of universal_closure_handler (defined in Go)
extern void universal_closure_handler(ffi_cif*, void*, void**, void*);

// Get pointer to universal_closure_handler
static void* get_universal_closure_handler(void) {
    return (void*)universal_closure_handler;
}

// Map type name string to FFI type pointer
// Returns NULL if type name is not recognized
static void* map_type_name_to_ffi_type(const char* type_name) {
    if (type_name == NULL) return NULL;

    if (strcmp(type_name, "void") == 0) return libffi_type_void;
    if (strcmp(type_name, "int") == 0 || strcmp(type_name, "int32") == 0) return libffi_type_sint32;
    if (strcmp(type_name, "uint") == 0 || strcmp(type_name, "uint32") == 0) return libffi_type_uint32;
    if (strcmp(type_name, "int64") == 0) return libffi_type_sint64;
    if (strcmp(type_name, "uint64") == 0) return libffi_type_uint64;
    if (strcmp(type_name, "int8") == 0) return libffi_type_sint8;
    if (strcmp(type_name, "uint8") == 0 || strcmp(type_name, "byte") == 0) return libffi_type_uint8;
    if (strcmp(type_name, "int16") == 0) return libffi_type_sint16;
    if (strcmp(type_name, "uint16") == 0) return libffi_type_uint16;
    if (strcmp(type_name, "float") == 0) return libffi_type_float;
    if (strcmp(type_name, "double") == 0) return libffi_type_double;
    if (strcmp(type_name, "bool") == 0) return libffi_type_sint32; // bool as int
    if (strcmp(type_name, "ptr") == 0 || strcmp(type_name, "pointer") == 0 || strcmp(type_name, "string") == 0) {
        return libffi_type_pointer;
    }

    return NULL; // Unknown type
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

    // Detect platform ABI (passed from Go code)
    // Note: arch and os strings are passed from callCFunctionWithFFI
    // For now, we use a default detection based on architecture

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
        status = libffi_prep_cif_var(cif, detected_abi, n_fixed_args, n_args,
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
        status = libffi_prep_cif(cif, detected_abi, n_args,
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

// Structure to hold closure creation result
typedef struct {
    void* codeloc;       // Executable code location
    void* closure;       // Closure pointer (for cleanup)
    void* cif;           // CIF pointer (for cleanup)
    void** arg_types;    // Arg types array (for cleanup)
    int status;          // 0 = success, negative = error
} closure_result_t;

// Create an FFI closure for dynamic callbacks
// param_types: array of type name strings (e.g., ["int", "ptr"])
// param_count: number of parameters
// return_type: return type name string
// user_data: opaque pointer to pass to handler (cgo.Handle)
static closure_result_t create_ffi_closure(
    const char** param_types,
    int param_count,
    const char* return_type,
    void* user_data
) {
    closure_result_t result = {NULL, NULL, NULL, NULL, -1};

    // Check if closure API is available
    if (libffi_closure_alloc == NULL || libffi_prep_closure_loc == NULL) {
        result.status = -1; // Closure API not available
        return result;
    }

    // Map return type
    ffi_type* ffi_ret_type = (ffi_type*)map_type_name_to_ffi_type(return_type);
    if (ffi_ret_type == NULL) {
        result.status = -2; // Invalid return type
        return result;
    }

    // Map parameter types
    ffi_type** ffi_arg_types = NULL;
    if (param_count > 0) {
        ffi_arg_types = (ffi_type**)malloc(sizeof(ffi_type*) * param_count);
        if (ffi_arg_types == NULL) {
            result.status = -3; // Memory allocation failed
            return result;
        }

        for (int i = 0; i < param_count; i++) {
            ffi_arg_types[i] = (ffi_type*)map_type_name_to_ffi_type(param_types[i]);
            if (ffi_arg_types[i] == NULL) {
                free(ffi_arg_types);
                result.status = -4; // Invalid parameter type
                return result;
            }
        }
    }

    // Create CIF
    ffi_cif* cif = (ffi_cif*)malloc(128);
    if (cif == NULL) {
        if (ffi_arg_types != NULL) free(ffi_arg_types);
        result.status = -5; // Memory allocation failed
        return result;
    }

    ffi_status status = libffi_prep_cif(
        cif,
        detected_abi,
        param_count,
        ffi_ret_type,
        (void*)ffi_arg_types
    );

    if (status != FFI_OK) {
        free(cif);
        if (ffi_arg_types != NULL) free(ffi_arg_types);
        result.status = -6; // prep_cif failed
        return result;
    }

    // Allocate closure
    void* codeloc = NULL;
    void* closure = libffi_closure_alloc(128, &codeloc);
    if (closure == NULL) {
        free(cif);
        if (ffi_arg_types != NULL) free(ffi_arg_types);
        result.status = -7; // closure_alloc failed
        return result;
    }

    // Prepare closure with universal handler
    void* handler = get_universal_closure_handler();
    status = libffi_prep_closure_loc(
        closure,
        cif,
        handler,
        user_data,
        codeloc
    );

    if (status != FFI_OK) {
        libffi_closure_free(closure);
        free(cif);
        if (ffi_arg_types != NULL) free(ffi_arg_types);
        result.status = -8; // prep_closure_loc failed
        return result;
    }

    // Success - populate result
    result.codeloc = codeloc;
    result.closure = closure;
    result.cif = cif;
    result.arg_types = (void**)ffi_arg_types;
    result.status = 0;

    return result;
}

// Cleanup closure resources
static void cleanup_ffi_closure(void* closure, void* cif, void** arg_types) {
    if (closure != NULL && libffi_closure_free != NULL) {
        libffi_closure_free(closure);
    }
    if (cif != NULL) {
        free(cif);
    }
    if (arg_types != NULL) {
        free(arg_types);
    }
}
*/
import "C"
import (
    "fmt"
    "runtime"
    "runtime/cgo"
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

    // Initialize platform-specific ABI detection if libffi loaded successfully
    if libffiAvailable {
        archCStr := C.CString(runtime.GOARCH)
        osCStr := C.CString(runtime.GOOS)
        C.init_platform_abi(archCStr, osCStr)
        C.free(unsafe.Pointer(archCStr))
        C.free(unsafe.Pointer(osCStr))
    }

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

// ============================================================================
// CLOSURE SUPPORT - Universal callback handler for dynamic signatures
// ============================================================================

//export universal_closure_handler
func universal_closure_handler(cif *C.ffi_cif, ret unsafe.Pointer, args *unsafe.Pointer, userdata unsafe.Pointer) {
    // Restore callback info from handle
    h := cgo.Handle(uintptr(userdata))
    info := h.Value().(*CallbackInfo)

    // Extract argument count
    nargs := int(cif.nargs)

    var zaArgs []any
    if nargs > 0 {
        // Access cif->arg_types array
        argTypesSlice := (*[1 << 28]*C.ffi_type)(unsafe.Pointer(cif.arg_types))[:nargs:nargs]
        argsSlice := (*[1 << 28]unsafe.Pointer)(unsafe.Pointer(args))[:nargs:nargs]

        // Unpack each argument
        zaArgs = make([]any, nargs)
        for i := 0; i < nargs; i++ {
            zaArgs[i] = unpackFFIArg(argsSlice[i], argTypesSlice[i])
        }
    }

    // Call Za function
    result, err := invokeZaCallback(info, zaArgs...)
    if err != nil {
        // Set zero/null return value on error
        packFFIReturn(ret, nil, cif.rtype)
        return
    }

    // Pack return value
    packFFIReturn(ret, result, cif.rtype)
}

// unpackFFIArg converts an FFI argument pointer to a Za value
func unpackFFIArg(argPtr unsafe.Pointer, ffiType *C.ffi_type) any {
    // Compare against loaded type pointers
    // Get pointers to compare
    typeVoid := C.get_ffi_type_void()
    typeSint32 := C.get_ffi_type_sint32()
    typeUint32 := C.get_ffi_type_uint32()
    typeSint64 := C.get_ffi_type_sint64()
    typeUint64 := C.get_ffi_type_uint64()
    typeSint8 := C.get_ffi_type_sint8()
    typeUint8 := C.get_ffi_type_uint8()
    typeSint16 := C.get_ffi_type_sint16()
    typeUint16 := C.get_ffi_type_uint16()
    typeFloat := C.get_ffi_type_float()
    typeDouble := C.get_ffi_type_double()
    typePointer := C.get_ffi_type_pointer()

    switch unsafe.Pointer(ffiType) {
    case typeSint32:
        return int(*(*C.int)(argPtr))
    case typeUint32:
        return uint(*(*C.uint)(argPtr))
    case typeSint64:
        return int64(*(*C.longlong)(argPtr))
    case typeUint64:
        return uint64(*(*C.ulonglong)(argPtr))
    case typeSint8:
        return int(*(*C.char)(argPtr))
    case typeUint8:
        return uint8(*(*C.uchar)(argPtr))
    case typeSint16:
        return int(*(*C.short)(argPtr))
    case typeUint16:
        return uint16(*(*C.ushort)(argPtr))
    case typeFloat:
        return float64(*(*C.float)(argPtr))
    case typeDouble:
        return float64(*(*C.double)(argPtr))
    case typePointer:
        ptr := *(*unsafe.Pointer)(argPtr)
        if ptr == nil {
            return NullPointer()
        }
        return NewCPointer(ptr, "closure_arg")
    case typeVoid:
        return nil
    default:
        // Unknown type - return as opaque pointer
        return NewCPointer(argPtr, "unknown_ffi_type")
    }
}

// packFFIReturn packs a Za value into an FFI return pointer
func packFFIReturn(retPtr unsafe.Pointer, value any, ffiType *C.ffi_type) {
    // Get pointers to compare
    typeVoid := C.get_ffi_type_void()
    typeSint32 := C.get_ffi_type_sint32()
    typeUint32 := C.get_ffi_type_uint32()
    typeSint64 := C.get_ffi_type_sint64()
    typeUint64 := C.get_ffi_type_uint64()
    typeSint8 := C.get_ffi_type_sint8()
    typeUint8 := C.get_ffi_type_uint8()
    typeSint16 := C.get_ffi_type_sint16()
    typeUint16 := C.get_ffi_type_uint16()
    typeFloat := C.get_ffi_type_float()
    typeDouble := C.get_ffi_type_double()
    typePointer := C.get_ffi_type_pointer()

    if value == nil && unsafe.Pointer(ffiType) != typeVoid {
        // Set zero value for non-void returns
        C.memset(retPtr, 0, C.size_t(8)) // Zero out return slot
        return
    }

    switch unsafe.Pointer(ffiType) {
    case typeSint32:
        intVal, _ := GetAsInt(value)
        *(*C.int)(retPtr) = C.int(intVal)
    case typeUint32:
        intVal, _ := GetAsInt(value)
        *(*C.uint)(retPtr) = C.uint(intVal)
    case typeSint64:
        intVal, _ := GetAsInt(value)
        *(*C.longlong)(retPtr) = C.longlong(intVal)
    case typeUint64:
        intVal, _ := GetAsInt(value)
        *(*C.ulonglong)(retPtr) = C.ulonglong(intVal)
    case typeSint8:
        intVal, _ := GetAsInt(value)
        *(*C.char)(retPtr) = C.char(intVal)
    case typeUint8:
        intVal, _ := GetAsInt(value)
        *(*C.uchar)(retPtr) = C.uchar(intVal)
    case typeSint16:
        intVal, _ := GetAsInt(value)
        *(*C.short)(retPtr) = C.short(intVal)
    case typeUint16:
        intVal, _ := GetAsInt(value)
        *(*C.ushort)(retPtr) = C.ushort(intVal)
    case typeFloat:
        floatVal, _ := GetAsFloat(value)
        *(*C.float)(retPtr) = C.float(floatVal)
    case typeDouble:
        floatVal, _ := GetAsFloat(value)
        *(*C.double)(retPtr) = C.double(floatVal)
    case typePointer:
        if ptr, ok := value.(*CPointerValue); ok {
            *(*unsafe.Pointer)(retPtr) = ptr.Ptr
        } else {
            *(*unsafe.Pointer)(retPtr) = nil
        }
    case typeVoid:
        // No return value to pack
    default:
        // Unknown type - set to null/zero
        C.memset(retPtr, 0, C.size_t(8))
    }
}

// createFFIClosure creates a dynamic libffi closure for a callback signature
// Returns: (executable code pointer, cleanup function, error)
func createFFIClosure(signature string, handle cgo.Handle) (unsafe.Pointer, func(), error) {
    // 1. Parse signature
    paramTypes, returnType, err := parseCallbackSignature(signature)
    if err != nil {
        return nil, nil, err
    }

    // 2. Convert parameter types to C strings
    var paramTypeCStrs []*C.char
    var paramTypePtrs **C.char
    nargs := len(paramTypes)

    if nargs > 0 {
        paramTypeCStrs = make([]*C.char, nargs)
        for i, typeName := range paramTypes {
            paramTypeCStrs[i] = C.CString(typeName)
        }
        // Create array of pointers
        paramTypePtrs = (**C.char)(C.malloc(C.size_t(nargs) * C.size_t(unsafe.Sizeof(uintptr(0)))))
        if paramTypePtrs == nil {
            // Cleanup allocated strings
            for _, cstr := range paramTypeCStrs {
                C.free(unsafe.Pointer(cstr))
            }
            return nil, nil, fmt.Errorf("failed to allocate param types array")
        }
        paramTypePtrsSlice := (*[1 << 28]*C.char)(unsafe.Pointer(paramTypePtrs))[:nargs:nargs]
        for i, cstr := range paramTypeCStrs {
            paramTypePtrsSlice[i] = cstr
        }
    }

    returnTypeCStr := C.CString(returnType)

    // 3. Call C function to create closure
    result := C.create_ffi_closure(paramTypePtrs, C.int(nargs), returnTypeCStr, unsafe.Pointer(uintptr(handle)))

    // Cleanup temporary C strings
    C.free(unsafe.Pointer(returnTypeCStr))
    for _, cstr := range paramTypeCStrs {
        C.free(unsafe.Pointer(cstr))
    }
    if paramTypePtrs != nil {
        C.free(unsafe.Pointer(paramTypePtrs))
    }

    // 4. Check result
    if result.status != 0 {
        var errMsg string
        switch result.status {
        case -1:
            errMsg = "libffi closure API not available"
        case -2:
            errMsg = fmt.Sprintf("invalid return type: %s", returnType)
        case -3:
            errMsg = "memory allocation failed for arg types"
        case -4:
            errMsg = "invalid parameter type"
        case -5:
            errMsg = "memory allocation failed for CIF"
        case -6:
            errMsg = "ffi_prep_cif failed"
        case -7:
            errMsg = "ffi_closure_alloc failed"
        case -8:
            errMsg = "ffi_prep_closure_loc failed"
        default:
            errMsg = fmt.Sprintf("unknown error (status %d)", result.status)
        }
        return nil, nil, fmt.Errorf("closure creation failed: %s", errMsg)
    }

    // 5. Create cleanup function
    closure := result.closure
    cif := result.cif
    argTypes := result.arg_types

    cleanup := func() {
        C.cleanup_ffi_closure(closure, cif, argTypes)
    }

    return result.codeloc, cleanup, nil
}
