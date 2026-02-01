//go:build !windows && !noffi && cgo
// +build !windows,!noffi,cgo

package main

/*
#include <dlfcn.h>
#include <stdlib.h>
#include <stdio.h>
#include <string.h>
#include <wchar.h>

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

// Full ffi_type structure definition (from libffi)
struct ffi_type_s {
    size_t size;               // Size of type (libffi computes for structs)
    unsigned short alignment;  // Alignment (libffi computes for structs)
    unsigned short type;       // Type identifier (FFI_TYPE_STRUCT for structs)
    ffi_type **elements;       // NULL-terminated array of field types (for structs)
};

// ffi_type constants (from libffi)
#define FFI_TYPE_VOID       0
#define FFI_TYPE_INT        1
#define FFI_TYPE_FLOAT      2
#define FFI_TYPE_DOUBLE     3
#define FFI_TYPE_UINT8      5
#define FFI_TYPE_SINT8      6
#define FFI_TYPE_UINT16     7
#define FFI_TYPE_SINT16     8
#define FFI_TYPE_UINT32     9
#define FFI_TYPE_SINT32     10
#define FFI_TYPE_UINT64     11
#define FFI_TYPE_SINT64     12
#define FFI_TYPE_STRUCT     13
#define FFI_TYPE_POINTER    14

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

// Map Za CType enum to libffi type pointer
// Returns NULL for CStruct (caller must create custom type)
static ffi_type* map_ctype_to_ffi_type(int ctype) {
    switch (ctype) {
        case 0: return libffi_type_void;      // CVoid
        case 1: return libffi_type_sint32;    // CInt
        case 2: return libffi_type_float;     // CFloat
        case 3: return libffi_type_double;    // CDouble
        case 4: return libffi_type_sint8;     // CChar
        case 5: return libffi_type_pointer;   // CString
        case 6: return libffi_type_uint8;     // CBool
        case 7: return libffi_type_pointer;   // CPointer
        case 8: return NULL;                  // CStruct - needs custom type
        case 9: return libffi_type_uint32;    // CUInt
        case 10: return libffi_type_sint16;   // CInt16
        case 11: return libffi_type_uint16;   // CUInt16
        case 12: return libffi_type_sint64;   // CInt64
        case 13: return libffi_type_uint64;   // CUInt64
        case 14: return libffi_type_longdouble; // CLongDouble
        case 15: return libffi_type_sint8;    // CInt8
        case 16: return libffi_type_uint8;    // CUInt8
        default: return NULL;
    }
}

// Create a custom ffi_type for a struct
// field_types: array of ffi_type* for each field (Go will create these)
// num_fields: number of fields
// Returns: dynamically allocated ffi_type*, caller must free
static ffi_type* create_struct_ffi_type(ffi_type** field_types, int num_fields) {
    if (field_types == NULL || num_fields < 0) {
        return NULL;
    }

    // Allocate the ffi_type structure
    ffi_type* struct_type = (ffi_type*)malloc(sizeof(ffi_type));
    if (struct_type == NULL) {
        return NULL;
    }

    // Allocate elements array (num_fields + 1 for NULL terminator)
    ffi_type** elements = (ffi_type**)malloc(sizeof(ffi_type*) * (num_fields + 1));
    if (elements == NULL) {
        free(struct_type);
        return NULL;
    }

    // Copy field types
    for (int i = 0; i < num_fields; i++) {
        elements[i] = field_types[i];
    }
    elements[num_fields] = NULL; // Terminator

    // Initialize struct type
    struct_type->size = 0;        // libffi computes this
    struct_type->alignment = 0;   // libffi computes this
    struct_type->type = FFI_TYPE_STRUCT;
    struct_type->elements = elements;

    return struct_type;
}

// Cleanup a custom struct ffi_type
static void free_struct_ffi_type(ffi_type* struct_type) {
    if (struct_type == NULL) {
        return;
    }
    if (struct_type->elements != NULL) {
        free(struct_type->elements);
    }
    free(struct_type);
}

// Global to store the computed return type size from the last ffi_prep_cif
// This allows Go code to retrieve the actual size libffi computed
static size_t last_computed_return_size = 0;

// Getter function for Go to retrieve the computed size
static size_t get_last_computed_return_size(void) {
    return last_computed_return_size;
}

// Generic FFI call wrapper
static int call_via_libffi(
    void* fn_ptr,
    int n_args,
    int* arg_types,         // Za CType enum values
    void** arg_values,      // Pointers to actual argument values
    int return_type,        // Za CType enum value
    void* return_value,     // Pointer to return value storage
    int is_variadic,        // 1 if variadic function, 0 otherwise
    int n_fixed_args,       // Number of fixed arguments (for variadic functions)
    ffi_type** custom_arg_types,  // Custom ffi_type* for each arg (NULL = use arg_types enum)
    ffi_type* custom_return_type  // Custom ffi_type* for return (NULL = use return_type enum)
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
        // Use custom type if provided, otherwise map from enum
        if (custom_arg_types != NULL && custom_arg_types[i] != NULL) {
            ffi_arg_types[i] = custom_arg_types[i];
        } else {
            ffi_arg_types[i] = map_ctype_to_ffi_type(arg_types[i]);
            if (ffi_arg_types[i] == NULL) {
                if (ffi_arg_types != NULL) free(ffi_arg_types);
                free(cif);
                return -2; // Unknown type
            }
        }
    }

    // Map return type
    ffi_type* ffi_return_type;
    if (custom_return_type != NULL) {
        ffi_return_type = custom_return_type;
    } else {
        ffi_return_type = map_ctype_to_ffi_type(return_type);
        if (ffi_return_type == NULL) {
            if (ffi_arg_types != NULL) free(ffi_arg_types);
            free(cif);
            return -2; // Unknown type
        }
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

    // Store the computed return type size for Go to retrieve later
    // This is the actual size libffi computed, which may differ from the struct definition size
    if (ffi_return_type != NULL && ffi_return_type->size > 0) {
        last_computed_return_size = ffi_return_type->size;
    } else {
        last_computed_return_size = 0;
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
    "math/big"
    "reflect"
    "runtime"
    "runtime/cgo"
    "strings"
    "sync"
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

        // Detect wchar_t size for platform
        wcharSize = unsafe.Sizeof(C.wchar_t(0))
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

// Global cache for struct ffi_type* pointers (to avoid recreating them)
var (
    structFFITypeCache     = make(map[string]unsafe.Pointer)
    structFFITypeCacheLock sync.RWMutex
)

// Platform-detected wchar_t size (set during init)
var wcharSize uintptr

// getLastComputedReturnSize retrieves the actual size libffi computed for the return type
// This is called AFTER the libffi call to get the size that was actually used
func getLastComputedReturnSize() C.size_t {
    return C.get_last_computed_return_size()
}

// createFFITypeForStruct creates a custom ffi_type for a struct definition
// Returns the ffi_type* pointer and any error
func createFFITypeForStruct(structDef *CLibraryStruct) (unsafe.Pointer, error) {
    if structDef == nil {
        return nil, fmt.Errorf("struct definition is nil")
    }

    // Check cache first
    structFFITypeCacheLock.RLock()
    if cached, ok := structFFITypeCache[structDef.Name]; ok {
        structFFITypeCacheLock.RUnlock()
        return cached, nil
    }
    structFFITypeCacheLock.RUnlock()

    // Special handling for unions: represent as integer-classified struct
    // Per x86-64 ABI, unions are classified using merged classification of all members.
    // When a union contains both integer and float members, the result is INTEGER class.
    // To ensure correct ABI, represent unions as generic integer types (uint64/uint32)
    // rather than using the largest member's specific type.
    if structDef.IsUnion {
        if structDef.Size == 0 {
            return nil, fmt.Errorf("union %s has zero size", structDef.Name)
        }

        // Represent union to match x86-64 ABI mixed classification
        // GCC uses: first eightbyte (0-7) in RAX (INTEGER), second eightbyte (8-15) in XMM0 (SSE)
        // To match this, use: uint64 (RAX) + float (XMM0) for 12-byte unions
        var fieldTypes []*C.ffi_type
        remainingSize := structDef.Size

        // First eightbyte: use uint64 to get INTEGER classification (RAX)
        if remainingSize >= 8 {
            fieldTypes = append(fieldTypes, C.map_ctype_to_ffi_type(C.int(CUInt64)))
            remainingSize -= 8
        } else if remainingSize >= 4 {
            fieldTypes = append(fieldTypes, C.map_ctype_to_ffi_type(C.int(CUInt)))
            remainingSize -= 4
        } else if remainingSize >= 2 {
            fieldTypes = append(fieldTypes, C.map_ctype_to_ffi_type(C.int(CUInt16)))
            remainingSize -= 2
        } else if remainingSize >= 1 {
            fieldTypes = append(fieldTypes, C.map_ctype_to_ffi_type(C.int(CUInt8)))
            remainingSize -= 1
        }

        // Second eightbyte: use float to get SSE classification (XMM0)
        if remainingSize >= 4 {
            fieldTypes = append(fieldTypes, C.map_ctype_to_ffi_type(C.int(CFloat)))
            remainingSize -= 4
        } else if remainingSize >= 2 {
            fieldTypes = append(fieldTypes, C.map_ctype_to_ffi_type(C.int(CUInt16)))
            remainingSize -= 2
        } else if remainingSize >= 1 {
            fieldTypes = append(fieldTypes, C.map_ctype_to_ffi_type(C.int(CUInt8)))
            remainingSize -= 1
        }

        if len(fieldTypes) == 0 {
            return nil, fmt.Errorf("union %s: could not create field types", structDef.Name)
        }


        // Create C array for field types
        numFields := len(fieldTypes)
        fieldTypesArray := (**C.ffi_type)(C.malloc(C.size_t(numFields) * C.size_t(unsafe.Sizeof(uintptr(0)))))
        if fieldTypesArray == nil {
            return nil, fmt.Errorf("failed to allocate field types array for union")
        }

        fieldTypesSlice := (*[1 << 30]*C.ffi_type)(unsafe.Pointer(fieldTypesArray))[:numFields:numFields]
        for i, ft := range fieldTypes {
            if ft == nil {
                C.free(unsafe.Pointer(fieldTypesArray))
                return nil, fmt.Errorf("union %s: nil field type at index %d", structDef.Name, i)
            }
            fieldTypesSlice[i] = ft
        }

        unionType := C.create_struct_ffi_type(fieldTypesArray, C.int(numFields))
        if unionType == nil {
            C.free(unsafe.Pointer(fieldTypesArray))
            return nil, fmt.Errorf("failed to create union ffi_type")
        }

        // Cache the union's ffi_type
        structFFITypeCacheLock.Lock()
        structFFITypeCache[structDef.Name] = unsafe.Pointer(unionType)
        structFFITypeCacheLock.Unlock()

        return unsafe.Pointer(unionType), nil
    }

    // Count total ffi_type elements (arrays expand to multiple elements)
    totalElements := 0
    for _, field := range structDef.Fields {
        if field.ArraySize > 0 {
            totalElements += field.ArraySize  // Each array element is a separate ffi_type
        } else {
            totalElements++
        }
    }

    if totalElements == 0 {
        return nil, fmt.Errorf("struct %s has no fields", structDef.Name)
    }

    // Allocate C array for field types (expanded for arrays)
    fieldTypesArray := (**C.ffi_type)(C.malloc(C.size_t(totalElements) * C.size_t(unsafe.Sizeof(uintptr(0)))))
    if fieldTypesArray == nil {
        return nil, fmt.Errorf("failed to allocate field types array")
    }

    // Convert to Go slice for easier indexing
    fieldTypesSlice := (*[1 << 30]*C.ffi_type)(unsafe.Pointer(fieldTypesArray))[:totalElements:totalElements]

    // Map each field to its ffi_type* (expanding arrays)
    idx := 0
    for _, field := range structDef.Fields {
        if field.ArraySize > 0 {
            // Array field - expand to multiple ffi_type elements
            elemType := C.map_ctype_to_ffi_type(C.int(field.ElementType))
            if elemType == nil {
                C.free(unsafe.Pointer(fieldTypesArray))
                return nil, fmt.Errorf("field %s: unsupported array element type %d", field.Name, field.ElementType)
            }
            // Add element type for each array element
            for j := 0; j < field.ArraySize; j++ {
                fieldTypesSlice[idx] = elemType
                idx++
            }
        } else {
            // Regular field
            var fieldType *C.ffi_type

            // Handle inline union/struct fields (both use UnionDef)
            if field.UnionDef != nil {
                // Recursively create ffi_type for inline union or struct
                nestedFFIType, err := createFFITypeForStruct(field.UnionDef)
                if err != nil {
                    C.free(unsafe.Pointer(fieldTypesArray))
                    return nil, fmt.Errorf("field %s: %w", field.Name, err)
                }
                fieldType = (*C.ffi_type)(nestedFFIType)
            } else if field.Type == CStruct && field.StructDef != nil {
                // Handle regular nested structs (non-inline)
                nestedFFIType, err := createFFITypeForStruct(field.StructDef)
                if err != nil {
                    C.free(unsafe.Pointer(fieldTypesArray))
                    return nil, fmt.Errorf("field %s: %w", field.Name, err)
                }
                fieldType = (*C.ffi_type)(nestedFFIType)
            } else {
                // Use standard type mapping
                fieldType = C.map_ctype_to_ffi_type(C.int(field.Type))
                if fieldType == nil {
                    C.free(unsafe.Pointer(fieldTypesArray))
                    return nil, fmt.Errorf("field %s: unsupported type %d", field.Name, field.Type)
                }
            }

            fieldTypesSlice[idx] = fieldType
            idx++
        }
    }

    // Create the struct ffi_type
    structType := C.create_struct_ffi_type(fieldTypesArray, C.int(totalElements))
    if structType == nil {
        C.free(unsafe.Pointer(fieldTypesArray))
        return nil, fmt.Errorf("failed to create struct ffi_type")
    }

    // Cache it
    structFFITypeCacheLock.Lock()
    structFFITypeCache[structDef.Name] = unsafe.Pointer(structType)
    structFFITypeCacheLock.Unlock()

    return unsafe.Pointer(structType), nil
}

// CallCFunctionViaLibFFI calls a C function using libffi
func CallCFunctionViaLibFFI(funcPtr unsafe.Pointer, funcName string, args []any, sig CFunctionSignature) (any, error) {
    if !IsLibFFIAvailable() {
        return nil, fmt.Errorf("libffi not available")
    }

    expectedRetType := sig.ReturnType

    // Detect and unwrap mutable arguments
    var mutableArgs []*MutableArg
    mutableArgIndices := make(map[int]*MutableArg)  // Index -> MutableArg mapping for marshaling
    unwrappedArgs := make([]any, len(args))

    for i, arg := range args {
        if mutArg, ok := arg.(*MutableArg); ok {
            // This is a mutable argument - track it
            mutableArgs = append(mutableArgs, mutArg)
            mutableArgIndices[i] = mutArg

            // Check if this is an output parameter (koutparam) that needs auto-allocation
            bin := mutArg.Binding
            if bin < uint64(len(*mutArg.IdentPtr)) && (*mutArg.IdentPtr)[bin].IKind == koutparam {
                // Auto-allocate memory for output parameters with unknown type
                // Allocate generously (64 bytes aligned) for output pointers/structs
                allocSize := uintptr(64)
                if i < len(sig.ParamTypes) && sig.ParamTypes[i] != CPointer {
                    // For non-pointer types, use the actual size
                    typeSize := getSizeForType(sig.ParamTypes[i])
                    if typeSize > 0 && typeSize < 64 {
                        allocSize = typeSize
                    }
                }
                // Allocate memory with malloc and zero it
                allocPtr := C.malloc(C.size_t(allocSize))
                if allocPtr == nil {
                    return nil, fmt.Errorf("failed to allocate memory for output parameter at index %d", i)
                }
                // Zero-initialize the allocated memory
                C.memset(allocPtr, 0, C.size_t(allocSize))
                mutArg.CPtr = allocPtr
                mutArg.IsAutoAllocated = true  // Mark for cleanup
                // Store the pointer as CPointerValue for proper marshaling
                // But mark it so we know NOT to add extra indirection in marshaling
                pval := &CPointerValue{Ptr: allocPtr}
                unwrappedArgs[i] = pval
            } else {
                // Regular mutable argument - use the wrapped value for marshaling
                unwrappedArgs[i] = mutArg.Value
            }
        } else {
            unwrappedArgs[i] = arg
        }
    }

    // Use unwrappedArgs for the rest of the function
    args = unwrappedArgs

    // For variadic functions, validate minimum argument count
    if sig.HasVarargs {
        if len(args) < sig.FixedArgCount {
            return nil, fmt.Errorf("variadic function %s requires at least %d arguments, got %d",
                funcName, sig.FixedArgCount, len(args))
        }
    }

    // Handle zero-arg functions
    if len(args) == 0 {
        // Allocate return value buffer with malloc for proper alignment
        returnBuf := C.malloc(C.size_t(unsafe.Sizeof(C.longlong(0))))
        defer C.free(returnBuf)

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
            returnBuf,
            C.int(isVariadic),
            C.int(0), // n_fixed_args (0 for zero-arg functions)
            nil,      // custom_arg_types
            nil,      // custom_return_type
        )

        if result != 0 {
            return nil, fmt.Errorf("libffi call failed with code %d", result)
        }

        return convertReturnValue(returnBuf, sig)
    }

    // Convert Za arguments to C values with type checking and range validation
    convertedArgs := make([]any, len(args))
    for i, arg := range args {
        // Check if this is an array - if so, skip ConvertZaToCValue and pass through
        if _, ok := arg.([]interface{}); ok {
            // Array argument (including empty arrays) - pass through without conversion
            convertedArgs[i] = arg
            continue
        }

        // Check if this is a koutparam pointer that was auto-allocated
        // It's passed as CPointerValue and needs special handling
        if mutArg, ok := mutableArgIndices[i]; ok {
            bin := mutArg.Binding
            if bin < uint64(len(*mutArg.IdentPtr)) && (*mutArg.IdentPtr)[bin].IKind == koutparam && mutArg.IsAutoAllocated {
                // This is an auto-allocated koutparam pointer
                // Wrap it as CPointerValue so it gets marshaled as a simple pointer
                convertedArgs[i] = &CPointerValue{Ptr: mutArg.CPtr}
                continue
            }
        }

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
    // Keep as unsafe.Pointer, don't cast to (*unsafe.Pointer) yet
    argValuesPtr := C.malloc(C.size_t(len(convertedArgs)) * C.size_t(unsafe.Sizeof(unsafe.Pointer(nil))))
    defer C.free(argValuesPtr)

    // Temporary storage for converted arguments (must stay alive during call)
    var cstrings []unsafe.Pointer
    var allocatedMem []unsafe.Pointer
    var cleanupFuncs []func()

    // Array to hold custom ffi_type* for struct/union parameters
    var customArgTypes []unsafe.Pointer
    if len(convertedArgs) > 0 {
        customArgTypes = make([]unsafe.Pointer, len(convertedArgs))
    }

    defer func() {
        for _, cs := range cstrings {
            C.free(cs)
        }
        for _, mem := range allocatedMem {
            C.free(mem)
        }
        // Execute cleanup functions (in reverse order for safety)
        for i := len(cleanupFuncs) - 1; i >= 0; i-- {
            cleanupFuncs[i]()
        }
    }()

    // Convert to slices for easier indexing
    argTypesSlice := (*[1 << 30]C.int)(unsafe.Pointer(argTypes))[:len(convertedArgs):len(convertedArgs)]
    argValuesSlice := (*[1 << 30]unsafe.Pointer)(unsafe.Pointer(argValuesPtr))[:len(convertedArgs):len(convertedArgs)]

    // Zero-initialize the argument values array
    if len(convertedArgs) > 0 {
        C.memset(argValuesPtr, 0, C.size_t(len(convertedArgs))*C.size_t(unsafe.Sizeof(unsafe.Pointer(nil))))
    }

    for i, arg := range convertedArgs {
        // Get expected parameter type to handle float vs double correctly
        var expectedParamType CType
        if i < len(sig.ParamTypes) {
            expectedParamType = sig.ParamTypes[i]
        }

        switch v := arg.(type) {
        case int:
            argTypesSlice[i] = 1 // CInt
            // Allocate C memory for int value
            intPtr := C.malloc(C.size_t(unsafe.Sizeof(C.int(0))))
            allocatedMem = append(allocatedMem, intPtr)
            *(*C.int)(intPtr) = C.int(v)
            argValuesSlice[i] = intPtr
            // Capture C pointer for mutable arguments (but not auto-allocated koutparam)
            if mutArg, ok := mutableArgIndices[i]; ok && !mutArg.IsAutoAllocated {
                mutArg.CPtr = intPtr
            }

        case uint:
            argTypesSlice[i] = 9 // CUInt
            // Allocate C memory for uint value
            uintPtr := C.malloc(C.size_t(unsafe.Sizeof(C.uint(0))))
            allocatedMem = append(allocatedMem, uintPtr)
            *(*C.uint)(uintPtr) = C.uint(v)
            argValuesSlice[i] = uintPtr
            // Capture C pointer for mutable arguments (but not auto-allocated koutparam)
            if mutArg, ok := mutableArgIndices[i]; ok && !mutArg.IsAutoAllocated {
                mutArg.CPtr = uintPtr
            }

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
            // Capture C pointer for mutable arguments (but not auto-allocated koutparam)
            if mutArg, ok := mutableArgIndices[i]; ok && !mutArg.IsAutoAllocated {
                mutArg.CPtr = int64Ptr
            }

        case uint64:
            argTypesSlice[i] = 13 // CUInt64
            // Allocate C memory for uint64 value
            uint64Ptr := C.malloc(C.size_t(unsafe.Sizeof(C.ulonglong(0))))
            allocatedMem = append(allocatedMem, uint64Ptr)
            *(*C.ulonglong)(uint64Ptr) = C.ulonglong(v)
            argValuesSlice[i] = uint64Ptr
            // Capture C pointer for mutable arguments (but not auto-allocated koutparam)
            if mutArg, ok := mutableArgIndices[i]; ok && !mutArg.IsAutoAllocated {
                mutArg.CPtr = uint64Ptr
            }

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
            // Check if this should be float (32-bit) or double (64-bit)
            if expectedParamType == CFloat {
                argTypesSlice[i] = 2 // CFloat
                // Allocate C memory for float value (convert float64 to float32)
                fltPtr := C.malloc(C.size_t(unsafe.Sizeof(C.float(0))))
                allocatedMem = append(allocatedMem, fltPtr)
                *(*C.float)(fltPtr) = C.float(v)
                argValuesSlice[i] = fltPtr
                // Capture C pointer for mutable arguments (but not auto-allocated koutparam)
                if mutArg, ok := mutableArgIndices[i]; ok && !mutArg.IsAutoAllocated {
                    mutArg.CPtr = fltPtr
                }
            } else {
                argTypesSlice[i] = 3 // CDouble
                // Allocate C memory for double value
                dblPtr := C.malloc(C.size_t(unsafe.Sizeof(C.double(0))))
                allocatedMem = append(allocatedMem, dblPtr)
                *(*C.double)(dblPtr) = C.double(v)
                argValuesSlice[i] = dblPtr
                // Capture C pointer for mutable arguments (but not auto-allocated koutparam)
                if mutArg, ok := mutableArgIndices[i]; ok && !mutArg.IsAutoAllocated {
                    mutArg.CPtr = dblPtr
                }
            }

        case string:
            argTypesSlice[i] = 5 // CString
            cstr := C.CString(v)
            cstrings = append(cstrings, unsafe.Pointer(cstr))
            // Allocate C memory for pointer to string
            ptrPtr := C.malloc(C.size_t(unsafe.Sizeof(unsafe.Pointer(nil))))
            allocatedMem = append(allocatedMem, ptrPtr)
            *(*unsafe.Pointer)(ptrPtr) = unsafe.Pointer(cstr)
            argValuesSlice[i] = ptrPtr
            // Capture C pointer for mutable arguments (but not auto-allocated koutparam)
            if mutArg, ok := mutableArgIndices[i]; ok && !mutArg.IsAutoAllocated {
                mutArg.CPtr = unsafe.Pointer(cstr)
            }

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
            // Capture C pointer for mutable arguments (but not auto-allocated koutparam)
            if mutArg, ok := mutableArgIndices[i]; ok && !mutArg.IsAutoAllocated {
                mutArg.CPtr = boolPtr
            }

        case []int:
            argTypesSlice[i] = 7 // CPointer
            arrayLen := len(v)

            // Handle empty arrays
            if arrayLen == 0 {
                ptrPtr := C.malloc(C.size_t(unsafe.Sizeof(unsafe.Pointer(nil))))
                allocatedMem = append(allocatedMem, ptrPtr)
                *(*unsafe.Pointer)(ptrPtr) = nil
                argValuesSlice[i] = ptrPtr
                continue
            }

            // Allocate C array
            elementSize := unsafe.Sizeof(C.int(0))
            totalSize := C.size_t(arrayLen) * C.size_t(elementSize)
            arrayPtr := C.malloc(totalSize)
            if arrayPtr == nil {
                return nil, fmt.Errorf("argument %d: failed to allocate int array", i)
            }
            allocatedMem = append(allocatedMem, arrayPtr)
            C.memset(arrayPtr, 0, totalSize)

            // Copy elements
            for idx, elem := range v {
                elemPtr := unsafe.Pointer(uintptr(arrayPtr) + uintptr(idx)*uintptr(elementSize))
                *(*C.int)(elemPtr) = C.int(elem)
            }

            // Create pointer slot for libffi
            ptrPtr := C.malloc(C.size_t(unsafe.Sizeof(unsafe.Pointer(nil))))
            allocatedMem = append(allocatedMem, ptrPtr)
            *(*unsafe.Pointer)(ptrPtr) = arrayPtr
            argValuesSlice[i] = ptrPtr

            // Track for mutable arrays
            if mutArg, ok := mutableArgIndices[i]; ok {
                mutArg.CPtr = arrayPtr
                mutArg.ArrayLen = arrayLen
                mutArg.ArrayElemType = "int"
            }

        case []float64:
            argTypesSlice[i] = 7 // CPointer
            arrayLen := len(v)

            // Handle empty arrays
            if arrayLen == 0 {
                ptrPtr := C.malloc(C.size_t(unsafe.Sizeof(unsafe.Pointer(nil))))
                allocatedMem = append(allocatedMem, ptrPtr)
                *(*unsafe.Pointer)(ptrPtr) = nil
                argValuesSlice[i] = ptrPtr
                continue
            }

            // Allocate C array
            elementSize := unsafe.Sizeof(C.double(0))
            totalSize := C.size_t(arrayLen) * C.size_t(elementSize)
            arrayPtr := C.malloc(totalSize)
            if arrayPtr == nil {
                return nil, fmt.Errorf("argument %d: failed to allocate float64 array", i)
            }
            allocatedMem = append(allocatedMem, arrayPtr)
            C.memset(arrayPtr, 0, totalSize)

            // Copy elements
            for idx, elem := range v {
                elemPtr := unsafe.Pointer(uintptr(arrayPtr) + uintptr(idx)*uintptr(elementSize))
                *(*C.double)(elemPtr) = C.double(elem)
            }

            // Create pointer slot for libffi
            ptrPtr := C.malloc(C.size_t(unsafe.Sizeof(unsafe.Pointer(nil))))
            allocatedMem = append(allocatedMem, ptrPtr)
            *(*unsafe.Pointer)(ptrPtr) = arrayPtr
            argValuesSlice[i] = ptrPtr

            // Track for mutable arrays
            if mutArg, ok := mutableArgIndices[i]; ok {
                mutArg.CPtr = arrayPtr
                mutArg.ArrayLen = arrayLen
                mutArg.ArrayElemType = "float64"
            }

        case []uint8:
            argTypesSlice[i] = 7 // CPointer
            arrayLen := len(v)

            // Handle empty arrays
            if arrayLen == 0 {
                ptrPtr := C.malloc(C.size_t(unsafe.Sizeof(unsafe.Pointer(nil))))
                allocatedMem = append(allocatedMem, ptrPtr)
                *(*unsafe.Pointer)(ptrPtr) = nil
                argValuesSlice[i] = ptrPtr
                continue
            }

            // Allocate C array
            elementSize := unsafe.Sizeof(C.uchar(0))
            totalSize := C.size_t(arrayLen) * C.size_t(elementSize)
            arrayPtr := C.malloc(totalSize)
            if arrayPtr == nil {
                return nil, fmt.Errorf("argument %d: failed to allocate uint8 array", i)
            }
            allocatedMem = append(allocatedMem, arrayPtr)
            C.memset(arrayPtr, 0, totalSize)

            // Copy elements
            for idx, elem := range v {
                elemPtr := unsafe.Pointer(uintptr(arrayPtr) + uintptr(idx)*uintptr(elementSize))
                *(*C.uchar)(elemPtr) = C.uchar(elem)
            }

            // Create pointer slot for libffi
            ptrPtr := C.malloc(C.size_t(unsafe.Sizeof(unsafe.Pointer(nil))))
            allocatedMem = append(allocatedMem, ptrPtr)
            *(*unsafe.Pointer)(ptrPtr) = arrayPtr
            argValuesSlice[i] = ptrPtr

            // Track for mutable arrays
            if mutArg, ok := mutableArgIndices[i]; ok {
                mutArg.CPtr = arrayPtr
                mutArg.ArrayLen = arrayLen
                mutArg.ArrayElemType = "uint8"
            }

        case []int64:
            argTypesSlice[i] = 7 // CPointer
            arrayLen := len(v)

            // Handle empty arrays
            if arrayLen == 0 {
                ptrPtr := C.malloc(C.size_t(unsafe.Sizeof(unsafe.Pointer(nil))))
                allocatedMem = append(allocatedMem, ptrPtr)
                *(*unsafe.Pointer)(ptrPtr) = nil
                argValuesSlice[i] = ptrPtr
                continue
            }

            // Allocate C array
            elementSize := unsafe.Sizeof(C.longlong(0))
            totalSize := C.size_t(arrayLen) * C.size_t(elementSize)
            arrayPtr := C.malloc(totalSize)
            if arrayPtr == nil {
                return nil, fmt.Errorf("argument %d: failed to allocate int64 array", i)
            }
            allocatedMem = append(allocatedMem, arrayPtr)
            C.memset(arrayPtr, 0, totalSize)

            // Copy elements
            for idx, elem := range v {
                elemPtr := unsafe.Pointer(uintptr(arrayPtr) + uintptr(idx)*uintptr(elementSize))
                *(*C.longlong)(elemPtr) = C.longlong(elem)
            }

            // Create pointer slot for libffi
            ptrPtr := C.malloc(C.size_t(unsafe.Sizeof(unsafe.Pointer(nil))))
            allocatedMem = append(allocatedMem, ptrPtr)
            *(*unsafe.Pointer)(ptrPtr) = arrayPtr
            argValuesSlice[i] = ptrPtr

            // Track for mutable arrays
            if mutArg, ok := mutableArgIndices[i]; ok {
                mutArg.CPtr = arrayPtr
                mutArg.ArrayLen = arrayLen
                mutArg.ArrayElemType = "int64"
            }

        case []uint:
            argTypesSlice[i] = 7 // CPointer
            arrayLen := len(v)

            // Handle empty arrays
            if arrayLen == 0 {
                ptrPtr := C.malloc(C.size_t(unsafe.Sizeof(unsafe.Pointer(nil))))
                allocatedMem = append(allocatedMem, ptrPtr)
                *(*unsafe.Pointer)(ptrPtr) = nil
                argValuesSlice[i] = ptrPtr
                continue
            }

            // Allocate C array
            elementSize := unsafe.Sizeof(C.uint(0))
            totalSize := C.size_t(arrayLen) * C.size_t(elementSize)
            arrayPtr := C.malloc(totalSize)
            if arrayPtr == nil {
                return nil, fmt.Errorf("argument %d: failed to allocate uint array", i)
            }
            allocatedMem = append(allocatedMem, arrayPtr)
            C.memset(arrayPtr, 0, totalSize)

            // Copy elements
            for idx, elem := range v {
                elemPtr := unsafe.Pointer(uintptr(arrayPtr) + uintptr(idx)*uintptr(elementSize))
                *(*C.uint)(elemPtr) = C.uint(elem)
            }

            // Create pointer slot for libffi
            ptrPtr := C.malloc(C.size_t(unsafe.Sizeof(unsafe.Pointer(nil))))
            allocatedMem = append(allocatedMem, ptrPtr)
            *(*unsafe.Pointer)(ptrPtr) = arrayPtr
            argValuesSlice[i] = ptrPtr

            // Track for mutable arrays
            if mutArg, ok := mutableArgIndices[i]; ok {
                mutArg.CPtr = arrayPtr
                mutArg.ArrayLen = arrayLen
                mutArg.ArrayElemType = "uint"
            }

        case []bool:
            argTypesSlice[i] = 7 // CPointer
            arrayLen := len(v)

            // Handle empty arrays
            if arrayLen == 0 {
                ptrPtr := C.malloc(C.size_t(unsafe.Sizeof(unsafe.Pointer(nil))))
                allocatedMem = append(allocatedMem, ptrPtr)
                *(*unsafe.Pointer)(ptrPtr) = nil
                argValuesSlice[i] = ptrPtr
                continue
            }

            // Allocate C array
            elementSize := unsafe.Sizeof(C.uchar(0))
            totalSize := C.size_t(arrayLen) * C.size_t(elementSize)
            arrayPtr := C.malloc(totalSize)
            if arrayPtr == nil {
                return nil, fmt.Errorf("argument %d: failed to allocate bool array", i)
            }
            allocatedMem = append(allocatedMem, arrayPtr)
            C.memset(arrayPtr, 0, totalSize)

            // Copy elements
            for idx, elem := range v {
                elemPtr := unsafe.Pointer(uintptr(arrayPtr) + uintptr(idx)*uintptr(elementSize))
                if elem {
                    *(*C.uchar)(elemPtr) = 1
                } else {
                    *(*C.uchar)(elemPtr) = 0
                }
            }

            // Create pointer slot for libffi
            ptrPtr := C.malloc(C.size_t(unsafe.Sizeof(unsafe.Pointer(nil))))
            allocatedMem = append(allocatedMem, ptrPtr)
            *(*unsafe.Pointer)(ptrPtr) = arrayPtr
            argValuesSlice[i] = ptrPtr

            // Track for mutable arrays
            if mutArg, ok := mutableArgIndices[i]; ok {
                mutArg.CPtr = arrayPtr
                mutArg.ArrayLen = arrayLen
                mutArg.ArrayElemType = "bool"
            }

        case []int16:
            argTypesSlice[i] = 7 // CPointer
            arrayLen := len(v)

            // Handle empty arrays
            if arrayLen == 0 {
                ptrPtr := C.malloc(C.size_t(unsafe.Sizeof(unsafe.Pointer(nil))))
                allocatedMem = append(allocatedMem, ptrPtr)
                *(*unsafe.Pointer)(ptrPtr) = nil
                argValuesSlice[i] = ptrPtr
                continue
            }

            // Allocate C array
            elementSize := unsafe.Sizeof(C.short(0))
            totalSize := C.size_t(arrayLen) * C.size_t(elementSize)
            arrayPtr := C.malloc(totalSize)
            if arrayPtr == nil {
                return nil, fmt.Errorf("argument %d: failed to allocate int16 array", i)
            }
            allocatedMem = append(allocatedMem, arrayPtr)
            C.memset(arrayPtr, 0, totalSize)

            // Copy elements
            for idx, elem := range v {
                elemPtr := unsafe.Pointer(uintptr(arrayPtr) + uintptr(idx)*uintptr(elementSize))
                *(*C.short)(elemPtr) = C.short(elem)
            }

            // Create pointer slot for libffi
            ptrPtr := C.malloc(C.size_t(unsafe.Sizeof(unsafe.Pointer(nil))))
            allocatedMem = append(allocatedMem, ptrPtr)
            *(*unsafe.Pointer)(ptrPtr) = arrayPtr
            argValuesSlice[i] = ptrPtr

            // Track for mutable arrays
            if mutArg, ok := mutableArgIndices[i]; ok {
                mutArg.CPtr = arrayPtr
                mutArg.ArrayLen = arrayLen
                mutArg.ArrayElemType = "int16"
            }

        case []uint16:
            argTypesSlice[i] = 7 // CPointer
            arrayLen := len(v)

            // Handle empty arrays
            if arrayLen == 0 {
                ptrPtr := C.malloc(C.size_t(unsafe.Sizeof(unsafe.Pointer(nil))))
                allocatedMem = append(allocatedMem, ptrPtr)
                *(*unsafe.Pointer)(ptrPtr) = nil
                argValuesSlice[i] = ptrPtr
                continue
            }

            // Allocate C array
            elementSize := unsafe.Sizeof(C.ushort(0))
            totalSize := C.size_t(arrayLen) * C.size_t(elementSize)
            arrayPtr := C.malloc(totalSize)
            if arrayPtr == nil {
                return nil, fmt.Errorf("argument %d: failed to allocate uint16 array", i)
            }
            allocatedMem = append(allocatedMem, arrayPtr)
            C.memset(arrayPtr, 0, totalSize)

            // Copy elements
            for idx, elem := range v {
                elemPtr := unsafe.Pointer(uintptr(arrayPtr) + uintptr(idx)*uintptr(elementSize))
                *(*C.ushort)(elemPtr) = C.ushort(elem)
            }

            // Create pointer slot for libffi
            ptrPtr := C.malloc(C.size_t(unsafe.Sizeof(unsafe.Pointer(nil))))
            allocatedMem = append(allocatedMem, ptrPtr)
            *(*unsafe.Pointer)(ptrPtr) = arrayPtr
            argValuesSlice[i] = ptrPtr

            // Track for mutable arrays
            if mutArg, ok := mutableArgIndices[i]; ok {
                mutArg.CPtr = arrayPtr
                mutArg.ArrayLen = arrayLen
                mutArg.ArrayElemType = "uint16"
            }

        case []uint64:
            argTypesSlice[i] = 7 // CPointer
            arrayLen := len(v)

            // Handle empty arrays
            if arrayLen == 0 {
                ptrPtr := C.malloc(C.size_t(unsafe.Sizeof(unsafe.Pointer(nil))))
                allocatedMem = append(allocatedMem, ptrPtr)
                *(*unsafe.Pointer)(ptrPtr) = nil
                argValuesSlice[i] = ptrPtr
                continue
            }

            // Allocate C array
            elementSize := unsafe.Sizeof(C.ulonglong(0))
            totalSize := C.size_t(arrayLen) * C.size_t(elementSize)
            arrayPtr := C.malloc(totalSize)
            if arrayPtr == nil {
                return nil, fmt.Errorf("argument %d: failed to allocate uint64 array", i)
            }
            allocatedMem = append(allocatedMem, arrayPtr)
            C.memset(arrayPtr, 0, totalSize)

            // Copy elements
            for idx, elem := range v {
                elemPtr := unsafe.Pointer(uintptr(arrayPtr) + uintptr(idx)*uintptr(elementSize))
                *(*C.ulonglong)(elemPtr) = C.ulonglong(elem)
            }

            // Create pointer slot for libffi
            ptrPtr := C.malloc(C.size_t(unsafe.Sizeof(unsafe.Pointer(nil))))
            allocatedMem = append(allocatedMem, ptrPtr)
            *(*unsafe.Pointer)(ptrPtr) = arrayPtr
            argValuesSlice[i] = ptrPtr

            // Track for mutable arrays
            if mutArg, ok := mutableArgIndices[i]; ok {
                mutArg.CPtr = arrayPtr
                mutArg.ArrayLen = arrayLen
                mutArg.ArrayElemType = "uint64"
            }

        case []int8:
            argTypesSlice[i] = 7 // CPointer
            arrayLen := len(v)

            // Handle empty arrays
            if arrayLen == 0 {
                ptrPtr := C.malloc(C.size_t(unsafe.Sizeof(unsafe.Pointer(nil))))
                allocatedMem = append(allocatedMem, ptrPtr)
                *(*unsafe.Pointer)(ptrPtr) = nil
                argValuesSlice[i] = ptrPtr
                continue
            }

            // Allocate C array
            elementSize := unsafe.Sizeof(C.char(0))
            totalSize := C.size_t(arrayLen) * C.size_t(elementSize)
            arrayPtr := C.malloc(totalSize)
            if arrayPtr == nil {
                return nil, fmt.Errorf("argument %d: failed to allocate int8 array", i)
            }
            allocatedMem = append(allocatedMem, arrayPtr)
            C.memset(arrayPtr, 0, totalSize)

            // Copy elements
            for idx, elem := range v {
                elemPtr := unsafe.Pointer(uintptr(arrayPtr) + uintptr(idx)*uintptr(elementSize))
                *(*C.char)(elemPtr) = C.char(elem)
            }

            // Create pointer slot for libffi
            ptrPtr := C.malloc(C.size_t(unsafe.Sizeof(unsafe.Pointer(nil))))
            allocatedMem = append(allocatedMem, ptrPtr)
            *(*unsafe.Pointer)(ptrPtr) = arrayPtr
            argValuesSlice[i] = ptrPtr

            // Track for mutable arrays
            if mutArg, ok := mutableArgIndices[i]; ok {
                mutArg.CPtr = arrayPtr
                mutArg.ArrayLen = arrayLen
                mutArg.ArrayElemType = "int8"
            }

        case []string:
            argTypesSlice[i] = 7 // CPointer
            arrayLen := len(v)

            // Handle empty arrays
            if arrayLen == 0 {
                ptrPtr := C.malloc(C.size_t(unsafe.Sizeof(unsafe.Pointer(nil))))
                allocatedMem = append(allocatedMem, ptrPtr)
                *(*unsafe.Pointer)(ptrPtr) = nil
                argValuesSlice[i] = ptrPtr
                continue
            }

            // Allocate array of char* pointers
            ptrArraySize := C.size_t(arrayLen) * C.size_t(unsafe.Sizeof(unsafe.Pointer(nil)))
            ptrArray := C.malloc(ptrArraySize)
            if ptrArray == nil {
                return nil, fmt.Errorf("argument %d: failed to allocate string array", i)
            }
            allocatedMem = append(allocatedMem, ptrArray)
            C.memset(ptrArray, 0, ptrArraySize)

            // Create slice view of pointer array
            ptrSlice := (*[1 << 30]*C.char)(unsafe.Pointer(ptrArray))[:arrayLen:arrayLen]

            // Allocate each string
            for idx, str := range v {
                cstr := C.CString(str)
                cstrings = append(cstrings, unsafe.Pointer(cstr))
                ptrSlice[idx] = cstr
            }

            // Create pointer slot
            ptrPtr := C.malloc(C.size_t(unsafe.Sizeof(unsafe.Pointer(nil))))
            allocatedMem = append(allocatedMem, ptrPtr)
            *(*unsafe.Pointer)(ptrPtr) = ptrArray
            argValuesSlice[i] = ptrPtr

        case []interface{}:
            // Handle generic Za arrays by inspecting element types
            if len(v) == 0 {
                // Empty array
                argTypesSlice[i] = 7 // CPointer
                ptrPtr := C.malloc(C.size_t(unsafe.Sizeof(unsafe.Pointer(nil))))
                allocatedMem = append(allocatedMem, ptrPtr)
                *(*unsafe.Pointer)(ptrPtr) = nil
                argValuesSlice[i] = ptrPtr
                continue
            }

            // Determine element type from first element
            elemType := reflect.TypeOf(v[0])
            argTypesSlice[i] = 7 // CPointer

            switch elemType.Kind() {
            case reflect.Int:
                // Convert to []int and marshal
                intArray := make([]C.int, len(v))
                for j, elem := range v {
                    if val, ok := elem.(int); ok {
                        intArray[j] = C.int(val)
                    } else {
                        return nil, fmt.Errorf("argument %d: array element %d is not int", i, j)
                    }
                }
                arrayPtr := C.malloc(C.size_t(len(v)) * C.size_t(unsafe.Sizeof(C.int(0))))
                if arrayPtr == nil {
                    return nil, fmt.Errorf("argument %d: failed to allocate int array", i)
                }
                allocatedMem = append(allocatedMem, arrayPtr)
                C.memcpy(arrayPtr, unsafe.Pointer(unsafe.SliceData(intArray)), C.size_t(len(intArray))*C.size_t(unsafe.Sizeof(C.int(0))))

                ptrPtr := C.malloc(C.size_t(unsafe.Sizeof(unsafe.Pointer(nil))))
                allocatedMem = append(allocatedMem, ptrPtr)
                *(*unsafe.Pointer)(ptrPtr) = arrayPtr
                argValuesSlice[i] = ptrPtr

                if mutArg, ok := mutableArgIndices[i]; ok {
                    mutArg.CPtr = arrayPtr
                    mutArg.ArrayLen = len(v)
                    mutArg.ArrayElemType = "int"
                }

            case reflect.Float64:
                // Convert to []float64 and marshal
                floatArray := make([]C.double, len(v))
                for j, elem := range v {
                    if val, ok := elem.(float64); ok {
                        floatArray[j] = C.double(val)
                    } else {
                        return nil, fmt.Errorf("argument %d: array element %d is not float64", i, j)
                    }
                }
                arrayPtr := C.malloc(C.size_t(len(v)) * C.size_t(unsafe.Sizeof(C.double(0))))
                if arrayPtr == nil {
                    return nil, fmt.Errorf("argument %d: failed to allocate float64 array", i)
                }
                allocatedMem = append(allocatedMem, arrayPtr)
                C.memcpy(arrayPtr, unsafe.Pointer(unsafe.SliceData(floatArray)), C.size_t(len(floatArray))*C.size_t(unsafe.Sizeof(C.double(0))))

                ptrPtr := C.malloc(C.size_t(unsafe.Sizeof(unsafe.Pointer(nil))))
                allocatedMem = append(allocatedMem, ptrPtr)
                *(*unsafe.Pointer)(ptrPtr) = arrayPtr
                argValuesSlice[i] = ptrPtr

                if mutArg, ok := mutableArgIndices[i]; ok {
                    mutArg.CPtr = arrayPtr
                    mutArg.ArrayLen = len(v)
                    mutArg.ArrayElemType = "float64"
                }

            case reflect.String:
                // String array handling
                arrayLen := len(v)
                ptrArraySize := C.size_t(arrayLen) * C.size_t(unsafe.Sizeof(unsafe.Pointer(nil)))
                ptrArray := C.malloc(ptrArraySize)
                if ptrArray == nil {
                    return nil, fmt.Errorf("argument %d: failed to allocate string array", i)
                }
                allocatedMem = append(allocatedMem, ptrArray)
                C.memset(ptrArray, 0, ptrArraySize)

                ptrSlice := (*[1 << 30]*C.char)(unsafe.Pointer(ptrArray))[:arrayLen:arrayLen]
                for j, elem := range v {
                    if str, ok := elem.(string); ok {
                        cstr := C.CString(str)
                        cstrings = append(cstrings, unsafe.Pointer(cstr))
                        ptrSlice[j] = cstr
                    } else {
                        return nil, fmt.Errorf("argument %d: array element %d is not string", i, j)
                    }
                }

                ptrPtr := C.malloc(C.size_t(unsafe.Sizeof(unsafe.Pointer(nil))))
                allocatedMem = append(allocatedMem, ptrPtr)
                *(*unsafe.Pointer)(ptrPtr) = ptrArray
                argValuesSlice[i] = ptrPtr

            case reflect.Bool:
                // Bool array handling
                boolArray := make([]C.uchar, len(v))
                for j, elem := range v {
                    if val, ok := elem.(bool); ok {
                        if val {
                            boolArray[j] = 1
                        } else {
                            boolArray[j] = 0
                        }
                    } else {
                        return nil, fmt.Errorf("argument %d: array element %d is not bool", i, j)
                    }
                }
                arrayPtr := C.malloc(C.size_t(len(v)) * C.size_t(unsafe.Sizeof(C.uchar(0))))
                if arrayPtr == nil {
                    return nil, fmt.Errorf("argument %d: failed to allocate bool array", i)
                }
                allocatedMem = append(allocatedMem, arrayPtr)
                C.memcpy(arrayPtr, unsafe.Pointer(unsafe.SliceData(boolArray)), C.size_t(len(boolArray))*C.size_t(unsafe.Sizeof(C.uchar(0))))

                ptrPtr := C.malloc(C.size_t(unsafe.Sizeof(unsafe.Pointer(nil))))
                allocatedMem = append(allocatedMem, ptrPtr)
                *(*unsafe.Pointer)(ptrPtr) = arrayPtr
                argValuesSlice[i] = ptrPtr

                if mutArg, ok := mutableArgIndices[i]; ok {
                    mutArg.CPtr = arrayPtr
                    mutArg.ArrayLen = len(v)
                    mutArg.ArrayElemType = "bool"
                }

            default:
                return nil, fmt.Errorf("argument %d: unsupported array element type %v. Supported: int, float64, string, bool", i, elemType)
            }

        case *CPointerValue:
            argTypesSlice[i] = 7 // CPointer
            //var expectedType string
            //if i < len(sig.ParamTypes) {
            //    expectedType = fmt.Sprintf("%d", sig.ParamTypes[i])
            //} else {
            //    expectedType = "unknown"
            //}

            // Check if this is a koutparam auto-allocated pointer
            isKoutparamPtr := false
            if mutArg, ok := mutableArgIndices[i]; ok {
                bin := mutArg.Binding
                if bin < uint64(len(*mutArg.IdentPtr)) && (*mutArg.IdentPtr)[bin].IKind == koutparam && mutArg.IsAutoAllocated {
                    isKoutparamPtr = true
                }
            }

            if isKoutparamPtr {
                // For koutparam pointers, pass the pointer value directly (no extra indirection)
                // libffi gets the pointer value itself, not a pointer-to-pointer
                ptrPtr := C.malloc(C.size_t(unsafe.Sizeof(unsafe.Pointer(nil))))
                if ptrPtr == nil {
                    return nil, fmt.Errorf("failed to allocate memory for marshaling koutparam at index %d", i)
                }
                allocatedMem = append(allocatedMem, ptrPtr)
                *(*unsafe.Pointer)(ptrPtr) = v.Ptr
                argValuesSlice[i] = ptrPtr
                // Verify what we wrote
                // stored := *(*unsafe.Pointer)(ptrPtr)
            } else {
                // For regular CPointerValue (struct output parameters), add extra indirection
                // v.Ptr is the buffer address (like &db in the C test where db is the result variable)
                // libffi needs: arg_values[i] points to the argument VALUE
                // The argument value for a void** parameter IS the address (&db)
                // So we just need to store v.Ptr and point to it
                ptrPtr := C.malloc(C.size_t(unsafe.Sizeof(unsafe.Pointer(nil))))
                allocatedMem = append(allocatedMem, ptrPtr)
                *(*unsafe.Pointer)(ptrPtr) = v.Ptr
                argValuesSlice[i] = ptrPtr
            }

        case *CFunctionPointer:
            // Function pointer parameter
            argTypesSlice[i] = 7 // CPointer

            if v.IsNull() {
                // Null function pointer
                ptrPtr := C.malloc(C.size_t(unsafe.Sizeof(unsafe.Pointer(nil))))
                if ptrPtr == nil {
                    return nil, fmt.Errorf("argument %d: failed to allocate memory for function pointer", i)
                }
                allocatedMem = append(allocatedMem, ptrPtr)
                *(*unsafe.Pointer)(ptrPtr) = nil
                argValuesSlice[i] = ptrPtr
            } else {
                // Non-null function pointer
                ptrPtr := C.malloc(C.size_t(unsafe.Sizeof(unsafe.Pointer(nil))))
                if ptrPtr == nil {
                    return nil, fmt.Errorf("argument %d: failed to allocate memory for function pointer", i)
                }
                allocatedMem = append(allocatedMem, ptrPtr)
                *(*unsafe.Pointer)(ptrPtr) = v.Ptr
                argValuesSlice[i] = ptrPtr
            }

        default:
            // Check if this is a struct/union type that needs marshaling
            // Get the struct type name from signature if available
            var structTypeName string
            if i < len(sig.ParamStructNames) {
                structTypeName = sig.ParamStructNames[i]
            }

            if structTypeName != "" {
                // Check if pointer-to-struct (has trailing *)
                isPointer := strings.HasSuffix(structTypeName, "*")
                baseStructName := strings.TrimSuffix(structTypeName, "*")

                // Strip "struct" and "union" keywords from the lookup name
                baseStructName = strings.TrimSpace(baseStructName)
                baseStructName = strings.TrimPrefix(baseStructName, "struct ")
                baseStructName = strings.TrimPrefix(baseStructName, "union ")
                baseStructName = strings.TrimSpace(baseStructName)

                // Look up the struct/union definition using base name
                // First try as union (from AUTO parsing), then as Za struct
                var structDef *CLibraryStruct
                var err error

                // Check if it's a union from C headers using use_chain resolution
                qualifiedName := uc_match_ffi_struct(baseStructName)
                if qualifiedName != "" {
                    ffiStructLock.RLock()
                    if def, ok := ffiStructDefinitions[qualifiedName]; ok {
                        structDef = def
                    }
                    ffiStructLock.RUnlock()
                }

                // If not found in FFI registry, try Za struct definition
                if structDef == nil {
                    structDef, err = getStructLayoutFromZa(baseStructName)
                    if err != nil {
                        return nil, fmt.Errorf("argument %d: failed to get struct/union layout for %s: %v", i, baseStructName, err)
                    }
                }

                // CRITICAL: Validate struct definition before marshaling
                if structDef == nil {
                    return nil, fmt.Errorf("argument %d: struct/union %s is not defined", i, baseStructName)
                }
                if structDef.Size == 0 {
                    return nil, fmt.Errorf("argument %d: struct/union %s has zero size - invalid definition", i, baseStructName)
                }

                // Check if this is a union type
                if structDef.IsUnion {
                    // Union parameter - expect a map literal
                    if isPointer {
                        // Pass union by pointer
                        argTypesSlice[i] = 7 // CPointer

                        // Arg should be a map
                        argMap, ok := arg.(map[string]any)
                        if !ok {
                            return nil, fmt.Errorf("argument %d: union pointer parameter expects map literal, got %T", i, arg)
                        }

                        // Allocate C memory for union
                        cPtr := C.malloc(C.size_t(structDef.Size))
                        if cPtr == nil {
                            return nil, fmt.Errorf("argument %d: failed to allocate C memory for union (size: %d)", i, structDef.Size)
                        }

                        // Zero-initialize
                        C.memset(cPtr, 0, C.size_t(structDef.Size))

                        // Track allocated strings
                        var allocatedStrings []unsafe.Pointer

                        // Marshal union
                        err = marshalUnion(argMap, structDef, cPtr, &allocatedStrings)
                        if err != nil {
                            C.free(cPtr)
                            for _, strPtr := range allocatedStrings {
                                C.free(strPtr)
                            }
                            return nil, fmt.Errorf("argument %d: failed to marshal union: %v", i, err)
                        }

                        // Add cleanup function for union pointer (will be called after libffi call)
                        allocatedStringsCopy := make([]unsafe.Pointer, len(allocatedStrings))
                        copy(allocatedStringsCopy, allocatedStrings)
                        cPtrCopy := cPtr
                        cleanupFuncs = append(cleanupFuncs, func() {
                            for _, strPtr := range allocatedStringsCopy {
                                C.free(strPtr)
                            }
                            C.free(cPtrCopy)
                        })

                        // For pointer unions, allocate a pointer slot and store cPtr address
                        ptrPtr := C.malloc(C.size_t(unsafe.Sizeof(unsafe.Pointer(nil))))
                        if ptrPtr == nil {
                            return nil, fmt.Errorf("argument %d: failed to allocate pointer slot for union", i)
                        }
                        allocatedMem = append(allocatedMem, ptrPtr)
                        *(*unsafe.Pointer)(ptrPtr) = cPtr
                        argValuesSlice[i] = ptrPtr
                    } else {
                        // Union parameter passed by value
                        argTypesSlice[i] = 8 // CStruct

                        // Arg should be a map
                        argMap, ok := arg.(map[string]any)
                        if !ok {
                            return nil, fmt.Errorf("argument %d: union parameter expects map literal, got %T", i, arg)
                        }

                        // Allocate C memory for union
                        cPtr := C.malloc(C.size_t(structDef.Size))
                        if cPtr == nil {
                            return nil, fmt.Errorf("argument %d: failed to allocate C memory for union (size: %d)", i, structDef.Size)
                        }

                        // Zero-initialize
                        C.memset(cPtr, 0, C.size_t(structDef.Size))

                        // Track allocated strings
                        var allocatedStrings []unsafe.Pointer

                        // Marshal union
                        err = marshalUnion(argMap, structDef, cPtr, &allocatedStrings)
                        if err != nil {
                            C.free(cPtr)
                            for _, strPtr := range allocatedStrings {
                                C.free(strPtr)
                            }
                            return nil, fmt.Errorf("argument %d: failed to marshal union: %v", i, err)
                        }

                        // Add cleanup function for union (will be called after libffi call)
                        allocatedStringsCopy := make([]unsafe.Pointer, len(allocatedStrings))
                        copy(allocatedStringsCopy, allocatedStrings)
                        cPtrCopy := cPtr
                        cleanupFuncs = append(cleanupFuncs, func() {
                            for _, strPtr := range allocatedStringsCopy {
                                C.free(strPtr)
                            }
                            C.free(cPtrCopy)
                        })

                        // Create custom ffi_type for the union (needed for libffi by-value passing)
                        customFFIType, err := createFFITypeForStruct(structDef)
                        if err != nil {
                            // Clean up before returning error
                            C.free(cPtr)
                            for _, strPtr := range allocatedStrings {
                                C.free(strPtr)
                            }
                            return nil, fmt.Errorf("argument %d: failed to create ffi_type for union %s: %v",
                                i, baseStructName, err)
                        }
                        customArgTypes[i] = customFFIType

                        // For by-value unions, argValuesSlice should point directly to the union data
                        // (not pointer-to-pointer, since we're passing the union by value)
                        argValuesSlice[i] = cPtr
                    }
                } else {
                    // Regular struct
                    if isPointer {
                        // Pass struct by pointer
                        argTypesSlice[i] = 7 // CPointer

                        cPtr, cleanup, err := MarshalStructToC(arg, structDef)
                        if err != nil {
                            return nil, fmt.Errorf("argument %d: failed to marshal struct: %v", i, err)
                        }

                        // Capture C pointer for mutable arguments
                        if mutArg, ok := mutableArgIndices[i]; ok {
                            mutArg.CPtr = cPtr
                            mutArg.StructDef = structDef
                        }

                        // Add cleanup function (will be called after libffi call)
                        cleanupFuncs = append(cleanupFuncs, cleanup)

                        // For pointer structs, allocate a pointer slot and store cPtr address
                        ptrPtr := C.malloc(C.size_t(unsafe.Sizeof(unsafe.Pointer(nil))))
                        if ptrPtr == nil {
                            return nil, fmt.Errorf("argument %d: failed to allocate pointer slot for struct", i)
                        }
                        allocatedMem = append(allocatedMem, ptrPtr)
                        *(*unsafe.Pointer)(ptrPtr) = cPtr
                        argValuesSlice[i] = ptrPtr
                    } else {
                        // Pass struct by value - use custom ffi_type for struct
                        argTypesSlice[i] = 8 // CStruct

                        customFFIType, err := createFFITypeForStruct(structDef)
                        if err != nil {
                            return nil, fmt.Errorf("argument %d: failed to create ffi_type for %s: %v", i, baseStructName, err)
                        }
                        customArgTypes[i] = customFFIType

                        cPtr, cleanup, err := MarshalStructToC(arg, structDef)
                        if err != nil {
                            return nil, fmt.Errorf("argument %d: failed to marshal struct: %v", i, err)
                        }

                        // Capture C pointer for mutable arguments
                        if mutArg, ok := mutableArgIndices[i]; ok {
                            mutArg.CPtr = cPtr
                            mutArg.StructDef = structDef
                        }

                        // Add cleanup function (will be called after libffi call)
                        cleanupFuncs = append(cleanupFuncs, cleanup)

                        // For by-value structs, argValuesSlice should point directly to the struct data
                        // (not pointer-to-pointer, since we're passing the struct by value)
                        argValuesSlice[i] = cPtr
                    }
                }
            } else {
                return nil, fmt.Errorf("unsupported argument type: %T", arg)
            }
        }
    }

    // Prepare return value storage
    // Allocate return value buffer with malloc for proper alignment on all platforms
    returnBuf := C.malloc(C.size_t(unsafe.Sizeof(C.longlong(0))))
    defer C.free(returnBuf)

    var returnPtr unsafe.Pointer
    var structReturnBuf unsafe.Pointer
    var structReturnBufSize C.size_t

    // Prepare custom ffi_type for struct/union return (if needed)
    // This applies to both CStruct explicit returns AND CPointer returns with a struct/union name
    var customReturnType unsafe.Pointer
    var initialStructSize int
    shouldCreateCustomReturnType := false

    if expectedRetType == CStruct && sig.ReturnStructName != "" {
        shouldCreateCustomReturnType = true
    } else if expectedRetType == CPointer && sig.ReturnStructName != "" && !strings.HasSuffix(sig.ReturnStructName, "*") {
        // CPointer return with a non-pointer struct name = struct/union returned by value
        shouldCreateCustomReturnType = true
    }

    if shouldCreateCustomReturnType {
        // Look up struct definition using use_chain resolution
        qualifiedName := uc_match_ffi_struct(sig.ReturnStructName)
        ffiStructLock.RLock()
        structDef, exists := ffiStructDefinitions[qualifiedName]
        ffiStructLock.RUnlock()

        if !exists || structDef == nil {
            return nil, fmt.Errorf("struct return type %s is not defined", sig.ReturnStructName)
        }
        if structDef.Size == 0 {
            return nil, fmt.Errorf("struct %s has zero size", sig.ReturnStructName)
        }

        // Create custom ffi_type for this struct
        var err error
        customReturnType, err = createFFITypeForStruct(structDef)
        if err != nil {
            return nil, fmt.Errorf("failed to create ffi_type for return struct %s: %v", sig.ReturnStructName, err)
        }

        initialStructSize = int(structDef.Size)

        returnPtr = returnBuf // Will be reassigned below after computing size
    } else {
        returnPtr = returnBuf
    }

    // Determine if variadic and fixed args count
    isVariadic := 0
    nFixedArgs := len(args)
    if sig.HasVarargs {
        isVariadic = 1
        nFixedArgs = sig.FixedArgCount
    }

    // Prepare custom_arg_types array for C (if we have any custom types)
    var customArgTypesC **C.ffi_type
    if len(customArgTypes) > 0 {
        // Check if any custom types exist
        hasCustomTypes := false
        for _, ct := range customArgTypes {
            if ct != nil {
                hasCustomTypes = true
                break
            }
        }

        if hasCustomTypes {
            // Allocate C array
            customArgTypesC = (**C.ffi_type)(C.malloc(C.size_t(len(customArgTypes)) * C.size_t(unsafe.Sizeof(uintptr(0)))))
            defer C.free(unsafe.Pointer(customArgTypesC))

            // Copy pointers
            slice := (*[1 << 30]*C.ffi_type)(unsafe.Pointer(customArgTypesC))[:len(customArgTypes):len(customArgTypes)]
            for i, ct := range customArgTypes {
                slice[i] = (*C.ffi_type)(ct)
            }
        }
    }

    // Call via libffi with custom types

    // Verify all argValuesSlice entries are valid pointers
    for i := 0; i < len(argValuesSlice); i++ {
        if argValuesSlice[i] == nil {
            return nil, fmt.Errorf("argument %d: argValuesSlice entry is nil", i)
        }
    }

/*
    // Debug: show argument types and values
    for i := 0; i < len(args); i++ {
        if i < len(argTypesSlice) {
            var typeStr string
            switch argTypesSlice[i] {
            case 1:
                typeStr = "CInt"
            case 2:
                typeStr = "CFloat"
            case 3:
                typeStr = "CDouble"
            case 6:
                typeStr = "CBool"
            case 7:
                typeStr = "CPointer"
            case 9:
                typeStr = "CUInt"
            case 12:
                typeStr = "CInt64"
            case 13:
                typeStr = "CUInt64"
            case 14:
                typeStr = "CString"
            default:
                typeStr = fmt.Sprintf("Type%d", argTypesSlice[i])
            }

            // Show what's in argValuesSlice[i]
            if i < len(argValuesSlice) {
                argPtr := argValuesSlice[i]
                if argPtr != nil {

                    // For pointer types, show what it points to (be careful not to dereference null)
                    if argTypesSlice[i] == 1 { // CInt
                        val := *(*C.int)(argPtr)
                    } else if argTypesSlice[i] == 5 { // CString
                        ptrVal := *(*unsafe.Pointer)(argPtr)
                        if ptrVal != nil {
                            cstr := C.GoString((*C.char)(ptrVal))
                        } else {
                        }
                    } else if argTypesSlice[i] == 12 { // CInt64
                        val := *(*C.longlong)(argPtr)
                    } else if argTypesSlice[i] == 7 { // CPointer
                        ptrVal := *(*unsafe.Pointer)(argPtr)
                    }
                } else {
                }
            }
        }
    }

    // Debug: show auto-allocated koutparam pointers
    for _, mutArg := range mutableArgs {
        if mutArg.IsAutoAllocated {
        }
    }
*/

    // For struct/union returns, allocate a buffer with extra padding for libffi's computed size
    // libffi may add padding to struct sizes due to ABI requirements
    // This applies when we have a custom return type (which could be for CStruct or CPointer with struct/union name)
    if customReturnType != nil && initialStructSize > 0 {
        // Allocate buffer with 2x padding to ensure libffi has enough space for any ABI padding
        // This is necessary because libffi may add padding based on x86-64 ABI rules
        allocSize := C.size_t(initialStructSize * 2)
        if allocSize < 128 {
            allocSize = 128
        }

        // Allocate the struct return buffer with padding
        structReturnBuf = C.malloc(allocSize)
        if structReturnBuf == nil {
            return nil, fmt.Errorf("failed to allocate return buffer for struct %s (size: %d)", sig.ReturnStructName, allocSize)
        }
        defer C.free(structReturnBuf)
        structReturnBufSize = allocSize
        C.memset(structReturnBuf, 0, structReturnBufSize)

        returnPtr = structReturnBuf
    }

    // Call libffi
    var status int
    status = int(C.call_via_libffi(
        funcPtr,
        C.int(len(args)),
        argTypes,
        (*unsafe.Pointer)(argValuesPtr),  // Cast the malloc'd array pointer to (*unsafe.Pointer) for libffi
        C.int(expectedRetType),
        returnPtr,
        C.int(isVariadic),
        C.int(nFixedArgs),
        customArgTypesC,     // custom_arg_types
        (*C.ffi_type)(customReturnType), // custom_return_type
    ))

    if status != 0 {
        return nil, fmt.Errorf("libffi call failed with code %d", status)
    }

    // CRITICAL: Unmarshal mutable arguments BEFORE cleanup runs (defer cleanup below)
    for _, mutArg := range mutableArgs {
        if mutArg.CPtr == nil {
            continue  // No C memory to read
        }

        var newValue any
        var err error

        // Handle different types
        if mutArg.StructDef != nil {
            // Struct type - use UnmarshalStructFromC
            newValue, err = UnmarshalStructFromC(mutArg.CPtr, mutArg.StructDef, "")
        } else if mutArg.ArrayLen > 0 {
            // Array type - unmarshal from C
            newValue, err = unmarshalArrayFromC(mutArg.CPtr, mutArg.ArrayLen, mutArg.ArrayElemType)
        } else {
            // Primitive type - read directly from C memory

            // For koutparam (output parameters with unknown type), determine type from signature
            bin := mutArg.Binding
            if bin < uint64(len(*mutArg.IdentPtr)) && (*mutArg.IdentPtr)[bin].IKind == koutparam {
                // Use the expected parameter type from function signature
                var paramType CType
                paramIndex := -1
                for idx, marg := range mutableArgIndices {
                    if marg == mutArg {
                        paramIndex = idx
                        break
                    }
                }

                if paramIndex >= 0 && paramIndex < len(sig.ParamTypes) {
                    paramType = sig.ParamTypes[paramIndex]
                } else {
                    // Fallback to pointer type if we can't determine
                    paramType = CPointer
                }

                // Read value based on expected parameter type
                switch paramType {
                case CInt:
                    newValue = int(*(*C.int)(mutArg.CPtr))
                case CUInt:
                    newValue = uint(*(*C.uint)(mutArg.CPtr))
                case CInt64:
                    newValue = int64(*(*C.longlong)(mutArg.CPtr))
                case CUInt64:
                    newValue = uint64(*(*C.ulonglong)(mutArg.CPtr))
                case CInt16:
                    newValue = int(*(*C.short)(mutArg.CPtr))
                case CUInt16:
                    newValue = uint(*(*C.ushort)(mutArg.CPtr))
                case CInt8:
                    newValue = int(*(*C.char)(mutArg.CPtr))
                case CUInt8:
                    newValue = uint(*(*C.uchar)(mutArg.CPtr))
                case CFloat:
                    newValue = float64(*(*C.float)(mutArg.CPtr))
                case CDouble:
                    newValue = float64(*(*C.double)(mutArg.CPtr))
                case CBool:
                    newValue = *(*C.int)(mutArg.CPtr) != 0  // 0 → false, non-zero → true
                case CPointer:
                    // For pointers, dereference the buffer to get the pointer value written by the function
                    ptrVal := *(*unsafe.Pointer)(mutArg.CPtr)
                    newValue = int64(uintptr(ptrVal))
                case CString:
                    newValue = C.GoString((*C.char)(mutArg.CPtr))
                default:
                    // Unknown type, skip
                    continue
                }
            } else {
                // Regular mutable argument - determine type from original value
                switch mutArg.Value.(type) {
                case int:
                    newValue = int(*(*C.int)(mutArg.CPtr))
                case uint:
                    newValue = uint(*(*C.uint)(mutArg.CPtr))
                case int64:
                    newValue = int64(*(*C.longlong)(mutArg.CPtr))
                case uint64:
                    newValue = uint64(*(*C.ulonglong)(mutArg.CPtr))
                case float64:
                    newValue = float64(*(*C.double)(mutArg.CPtr))
                case bool:
                    newValue = *(*C.int)(mutArg.CPtr) != 0  // 0 → false, non-zero → true
                case string:
                    // For strings, assume it's a char buffer we filled
                    newValue = C.GoString((*C.char)(mutArg.CPtr))
                default:
                    // Unknown type, skip
                    continue
                }
            }
        }

        if err != nil {
            continue  // Log warning but don't fail
        }

        // Update the original Za variable
        bin := mutArg.Binding
        if bin < uint64(len(*mutArg.IdentPtr)) {
            // Check if this is an output parameter with unknown type
            if (*mutArg.IdentPtr)[bin].IKind == koutparam {
                // Determine the appropriate kind based on the actual value type
                var newKind uint8
                switch newValue.(type) {
                case nil:
                    newKind = knil
                case bool:
                    newKind = kbool
                case int:
                    newKind = kint
                case int64:
                    newKind = kint64
                case uint:
                    newKind = kuint
                case uint64:
                    newKind = kuint64
                case float64:
                    newKind = kfloat
                case string:
                    newKind = kstring
                case *big.Int:
                    newKind = kbigi
                case *big.Float:
                    newKind = kbigf
                case []bool:
                    newKind = ksbool
                case []int:
                    newKind = ksint
                case []int64:
                    newKind = ksint64
                case []uint:
                    newKind = ksuint
                case []uint64:
                    newKind = ksuint64
                case []float64:
                    newKind = ksfloat
                case []string:
                    newKind = ksstring
                case []*big.Int:
                    newKind = ksbigi
                case []*big.Float:
                    newKind = ksbigf
                case []uint8:
                    newKind = kbyte
                case map[string]any:
                    newKind = kmap
                default:
                    // For unknown types (likely structs), use kany
                    newKind = kany
                }

                if mutArg.IsGlobal {
                    glock.Lock()
                    (*mutArg.IdentPtr)[bin].IValue = newValue
                    (*mutArg.IdentPtr)[bin].IKind = newKind
                    (*mutArg.IdentPtr)[bin].ITyped = true
                    glock.Unlock()
                } else {
                    (*mutArg.IdentPtr)[bin].IValue = newValue
                    (*mutArg.IdentPtr)[bin].IKind = newKind
                    (*mutArg.IdentPtr)[bin].ITyped = true
                }
            } else {
                // Normal update for already-typed variables
                if mutArg.IsGlobal {
                    glock.Lock()
                    (*mutArg.IdentPtr)[bin].IValue = newValue
                    glock.Unlock()
                } else {
                    (*mutArg.IdentPtr)[bin].IValue = newValue
                }
            }
        }
    }

    // Free allocated memory for koutparam variables (after unmarshaling is complete)
    // At this point, the variable has been updated with the dereferenced pointer value,
    // so the temporary buffer is no longer needed
    for _, mutArg := range mutableArgs {
        if mutArg.IsAutoAllocated && mutArg.CPtr != nil {
            C.free(mutArg.CPtr)
        }
    }

    // For struct returns, use the allocated buffer; otherwise use returnBuf
    if structReturnBuf != nil {
        // Get the actual computed size that libffi used
        computedSize := getLastComputedReturnSize()
        return convertReturnValueWithSize(structReturnBuf, sig, int(computedSize))
    }
    return convertReturnValue(returnBuf, sig)
}

// convertReturnValueWithSize is like convertReturnValue but knows the actual computed size from libffi
// This is important for struct/union returns where libffi may compute a different size than the struct definition
func convertReturnValueWithSize(returnValuePtr unsafe.Pointer, sig CFunctionSignature, computedSize int) (any, error) {
    expectedRetType := sig.ReturnType

    // For struct returns, pass the computed size to unmarshal
    if expectedRetType == CStruct && sig.ReturnStructName != "" {
        isPointerReturn := strings.HasSuffix(sig.ReturnStructName, "*")
        baseStructName := strings.TrimSuffix(sig.ReturnStructName, "*")

        qualifiedName := uc_match_ffi_struct(baseStructName)
        var structDef *CLibraryStruct
        if qualifiedName != "" {
            ffiStructLock.RLock()
            if def, ok := ffiStructDefinitions[qualifiedName]; ok {
                structDef = def
            }
            ffiStructLock.RUnlock()
        }

        if structDef == nil {
            structDef, _ = getStructLayoutFromZa(baseStructName)
        }

        if structDef != nil && !isPointerReturn {
            // Value return - unmarshal with knowledge of computed size
            resultMap, err := UnmarshalStructFromC(returnValuePtr, structDef, "")
            if err != nil {
                return nil, fmt.Errorf("failed to unmarshal struct %s: %v", baseStructName, err)
            }
            return resultMap, nil
        }
    }

    // For non-struct returns, use the regular converter
    return convertReturnValue(returnValuePtr, sig)
}

// convertReturnValue converts C return value to Za type
func convertReturnValue(returnValuePtr unsafe.Pointer, sig CFunctionSignature) (any, error) {
    expectedRetType := sig.ReturnType

    // For non-struct returns, read the value from the pointer
    var returnValue C.longlong
    if expectedRetType != CStruct {
        returnValue = *(*C.longlong)(returnValuePtr)
    }

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

    case CUInt8, CChar:
        return uint8(*(*C.uchar)(unsafe.Pointer(&returnValue))), nil

    case CFloat:
        result := float64(*(*C.float)(unsafe.Pointer(&returnValue)))
        return result, nil

    case CDouble:
        result := float64(*(*C.double)(unsafe.Pointer(&returnValue)))
        return result, nil

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

        // Check if this is a pointer-to-struct and try to unmarshal it
        if sig.ReturnStructName != "" && strings.HasSuffix(sig.ReturnStructName, "*") && ptr != nil {
            baseStructName := strings.TrimSuffix(sig.ReturnStructName, "*")
            baseStructName = strings.TrimSpace(baseStructName)

            // Look up the struct definition using use_chain resolution
            var structDef *CLibraryStruct
            qualifiedName := uc_match_ffi_struct(baseStructName)
            if qualifiedName != "" {
                ffiStructLock.RLock()
                if def, ok := ffiStructDefinitions[qualifiedName]; ok {
                    structDef = def
                }
                ffiStructLock.RUnlock()
            }

            // If struct definition exists, auto-unmarshal the pointed-to struct
            if structDef != nil {
                if structDef.IsUnion {
                    return unmarshalUnion(ptr, structDef)
                } else {
                    return UnmarshalStructFromC(ptr, structDef, baseStructName)
                }
            }
        }

        return &CPointerValue{Ptr: ptr}, nil

    case CStruct:
        if sig.ReturnStructName != "" {
            // Check if pointer-to-struct return (has trailing *)
            isPointerReturn := strings.HasSuffix(sig.ReturnStructName, "*")
            baseStructName := strings.TrimSuffix(sig.ReturnStructName, "*")

            // Look up the struct/union definition to determine if it's a value or pointer return
            var structDef *CLibraryStruct
            var err error

            // Check if it's a union from C headers using use_chain resolution
            qualifiedName := uc_match_ffi_struct(baseStructName)
            if qualifiedName != "" {
                ffiStructLock.RLock()
                if def, ok := ffiStructDefinitions[qualifiedName]; ok {
                    structDef = def
                }
                ffiStructLock.RUnlock()
            }

            // If not found in FFI registry, try Za struct definition
            if structDef == nil {
                structDef, err = getStructLayoutFromZa(baseStructName)
                if err != nil {
                    return nil, fmt.Errorf("failed to get struct/union layout for %s: %v", baseStructName, err)
                }
            }

            // Validate struct definition before unmarshaling
            // For value returns, require struct definition; for pointer returns, allow fallback to raw pointer
            if structDef == nil && !isPointerReturn {
                return nil, fmt.Errorf("struct/union %s is not defined - cannot unmarshal return value", baseStructName)
            }
            if structDef != nil && structDef.Size == 0 {
                return nil, fmt.Errorf("struct/union %s has zero size - cannot unmarshal", baseStructName)
            }

            if isPointerReturn {
                // Struct returned as pointer - returnValuePtr contains a pointer to the struct
                ptr := *(*unsafe.Pointer)(returnValuePtr)
                if ptr == nil {
                    return &CPointerValue{Ptr: nil}, nil
                }

                // If struct definition exists, auto-unmarshal the pointed-to struct
                if structDef != nil {
                    if structDef.IsUnion {
                        return unmarshalUnion(ptr, structDef)
                    } else {
                        return UnmarshalStructFromC(ptr, structDef, baseStructName)
                    }
                }

                // Fallback: return raw pointer if no struct definition available
                return &CPointerValue{Ptr: ptr}, nil
            } else {
                // Check if this is a union type (unions are typically returned by value)
                if structDef.IsUnion {
                    // Union returned by value - data is in return buffer
                    return unmarshalUnion(returnValuePtr, structDef)
                } else {
                    // Regular struct - also returned by value (data is in return buffer)
                    return UnmarshalStructFromC(returnValuePtr, structDef, baseStructName)
                }
            }
        } else {
            // Generic struct - return as opaque pointer
            ptr := *(*unsafe.Pointer)(returnValuePtr)
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

// unmarshalArrayFromC copies data from C array back to Go slice
func unmarshalArrayFromC(cPtr unsafe.Pointer, length int, elemType string) (any, error) {
    switch elemType {
    case "int":
        result := make([]int, length)
        for i := 0; i < length; i++ {
            elemPtr := unsafe.Pointer(uintptr(cPtr) + uintptr(i)*unsafe.Sizeof(C.int(0)))
            result[i] = int(*(*C.int)(elemPtr))
        }
        return result, nil

    case "float64":
        result := make([]float64, length)
        for i := 0; i < length; i++ {
            elemPtr := unsafe.Pointer(uintptr(cPtr) + uintptr(i)*unsafe.Sizeof(C.double(0)))
            result[i] = float64(*(*C.double)(elemPtr))
        }
        return result, nil

    case "uint8":
        result := make([]uint8, length)
        for i := 0; i < length; i++ {
            elemPtr := unsafe.Pointer(uintptr(cPtr) + uintptr(i))
            result[i] = uint8(*(*C.uchar)(elemPtr))
        }
        return result, nil

    case "bool":
        result := make([]bool, length)
        for i := 0; i < length; i++ {
            elemPtr := unsafe.Pointer(uintptr(cPtr) + uintptr(i))
            result[i] = *(*C.uchar)(elemPtr) != 0
        }
        return result, nil

    case "int64":
        result := make([]int64, length)
        for i := 0; i < length; i++ {
            elemPtr := unsafe.Pointer(uintptr(cPtr) + uintptr(i)*unsafe.Sizeof(C.longlong(0)))
            result[i] = int64(*(*C.longlong)(elemPtr))
        }
        return result, nil

    case "uint64":
        result := make([]uint64, length)
        for i := 0; i < length; i++ {
            elemPtr := unsafe.Pointer(uintptr(cPtr) + uintptr(i)*unsafe.Sizeof(C.ulonglong(0)))
            result[i] = uint64(*(*C.ulonglong)(elemPtr))
        }
        return result, nil

    case "int16":
        result := make([]int16, length)
        for i := 0; i < length; i++ {
            elemPtr := unsafe.Pointer(uintptr(cPtr) + uintptr(i)*unsafe.Sizeof(C.short(0)))
            result[i] = int16(*(*C.short)(elemPtr))
        }
        return result, nil

    case "uint16":
        result := make([]uint16, length)
        for i := 0; i < length; i++ {
            elemPtr := unsafe.Pointer(uintptr(cPtr) + uintptr(i)*unsafe.Sizeof(C.ushort(0)))
            result[i] = uint16(*(*C.ushort)(elemPtr))
        }
        return result, nil

    case "uint":
        result := make([]uint, length)
        for i := 0; i < length; i++ {
            elemPtr := unsafe.Pointer(uintptr(cPtr) + uintptr(i)*unsafe.Sizeof(C.uint(0)))
            result[i] = uint(*(*C.uint)(elemPtr))
        }
        return result, nil

    case "int8":
        result := make([]int8, length)
        for i := 0; i < length; i++ {
            elemPtr := unsafe.Pointer(uintptr(cPtr) + uintptr(i))
            result[i] = int8(*(*C.char)(elemPtr))
        }
        return result, nil

    default:
        return nil, fmt.Errorf("unsupported array element type for unmarshal: %s", elemType)
    }
}
