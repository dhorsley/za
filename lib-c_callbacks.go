package main

/*
#include <stdint.h>

// Trampoline functions for callbacks with context parameter
// These are exported Go functions that C can call

// Signature: int compar(void *a, void *b, void *context)
// Used by: qsort_r, bsearch_r
extern int za_callback_with_context_ptr_ptr_int(void *arg1, void *arg2, uintptr_t context);

// Signature: int compar(int a, int b, void *context)
extern int za_callback_with_context_int_int_int(int arg1, int arg2, uintptr_t context);

// Signature: void* start_routine(void *arg)
// Used by: pthread_create (arg is passed separately, not in context)
extern void* za_callback_ptr_ptr(void *arg);

// Signature: void handler(int signum, siginfo_t *info, void *context)
// Used by: sigaction with SA_SIGINFO
extern void za_callback_sigaction(int signum, void *info, void *context);

// Signature: void handler(int signum)
// Used by: signal (simple, no context)
extern void za_callback_int_void(int signum);

// NEW SIGNATURES - Additional common patterns

// Signature: double func(double x, void *context)
// Used by: Math transformations, numerical algorithms
extern double za_callback_double_double(double arg, uintptr_t context);

// Signature: int compar(void *a, void *b, void *c, void *context)
// Used by: Complex comparators with 3 data pointers
extern int za_callback_ptr_ptr_ptr_int(void *arg1, void *arg2, void *arg3, uintptr_t context);

// Signature: void func(void *context)
// Used by: Simple callbacks, thread_cleanup, atexit-style handlers
extern void za_callback_void_void(uintptr_t context);

// Signature: void func(void *ptr, void *context)
// Used by: Cleanup/destructor callbacks, iteration callbacks
extern void za_callback_ptr_void(void *arg, uintptr_t context);

// HIGH PRIORITY ADDITIONS

// Signature: int func(int x, void *context)
// Used by: Hash functions, error code mappers, simple transforms
extern int za_callback_int_int(int arg, uintptr_t context);

// Signature: void func(void *data, size_t length, void *context)
// Used by: Buffer processors, data handlers with length
extern void za_callback_ptr_int_void(void *arg1, int arg2, uintptr_t context);

// Signature: int func(void *data, size_t length, void *context)
// Used by: Validators with length, return status
extern int za_callback_ptr_int_int(void *arg1, int arg2, uintptr_t context);

// Signature: void func(void *key, void *value, void *context)
// Used by: Iteration callbacks without return (tree traversal, foreach)
extern void za_callback_ptr_ptr_void(void *arg1, void *arg2, uintptr_t context);

// Signature: float func(float x, void *context)
// Used by: Single-precision math, 32-bit float processing
extern float za_callback_float_float(float arg, uintptr_t context);

// Signature: void func(const char *msg, void *context)
// Used by: Logging callbacks, error handlers
extern void za_callback_string_void(char *arg, uintptr_t context);

// Signature: double func(double x, double y, void *context)
// Used by: Binary math operations, distance functions
extern double za_callback_double_double_double(double arg1, double arg2, uintptr_t context);

// ADDITIONAL USEFUL SIGNATURES

// Signature: int func(const char *str, void *context)
// Used by: String validators, parsers, hash functions
extern int za_callback_string_int(char *arg, uintptr_t context);

// Signature: void func(int a, int b, void *context)
// Used by: Progress callbacks, range handlers
extern void za_callback_int_int_void(int arg1, int arg2, uintptr_t context);

// Signature: int func(void *a, void *b, void *context) returning bool as int
// Used by: Predicate functions for filtering
extern int za_callback_ptr_ptr_bool(void *arg1, void *arg2, uintptr_t context);

*/
import "C"

import (
    "context"
    "fmt"
    "runtime/cgo"
    "strings"
    "sync"
    "sync/atomic"
    "unsafe"
)

// CallbackInfo stores information about a registered callback
type CallbackInfo struct {
    ZaFuncName     string   // Name of Za function to call
    CallerEvalfs   uint32   // evalfs of the context where callback was registered
    Signature      string   // e.g., "int,int->int"
    CallbackID     int      // Unique ID for this callback
    ClosureCleanup func()   // Optional cleanup for dynamic closures (nil for hardcoded trampolines)
}

// Global callback registry using cgo.Handle for safe passing through C
var callbackHandles = make(map[int]cgo.Handle) // callbackID → cgo.Handle
var callbackMutex sync.RWMutex
var callbackCounter int32

// Mutex to serialize callback invocations for thread safety
var callbackInvocationMutex sync.Mutex

// parseCallbackSignature parses a signature string like "int,ptr->double"
// Returns: (paramTypes []string, returnType string, error)
func parseCallbackSignature(sig string) ([]string, string, error) {
    // Check for unsupported patterns
    if strings.Contains(sig, "struct<") {
        return nil, "", fmt.Errorf("struct-by-value callbacks not yet supported: %s", sig)
    }
    if strings.Contains(sig, "...") {
        return nil, "", fmt.Errorf("variadic callbacks not supported via closures: %s", sig)
    }

    // Split on "->"
    parts := strings.Split(sig, "->")
    if len(parts) != 2 {
        return nil, "", fmt.Errorf("invalid signature format (expected 'params->return'): %s", sig)
    }

    returnType := strings.TrimSpace(parts[1])

    // Handle void parameters (no params)
    paramsPart := strings.TrimSpace(parts[0])
    if paramsPart == "" || paramsPart == "void" {
        return []string{}, returnType, nil
    }

    // Split parameters
    params := strings.Split(paramsPart, ",")
    paramTypes := make([]string, len(params))
    for i, p := range params {
        paramTypes[i] = strings.TrimSpace(p)
    }

    return paramTypes, returnType, nil
}

// invokeZaCallback invokes a Za function from a C callback
// Follows the pattern from task() in actor.go and function calls in eval_ops.go
func invokeZaCallback(info *CallbackInfo, args ...any) (any, error) {
    // Serialize all callback invocations to avoid Za interpreter race conditions
    callbackInvocationMutex.Lock()
    defer callbackInvocationMutex.Unlock()

    // Look up function base address
    lmv, isfunc := fnlookup.lmget(info.ZaFuncName)
    if !isfunc {
        return nil, fmt.Errorf("callback function %s not found", info.ZaFuncName)
    }

    // Get function space
    loc, _ := GetNextFnSpace(true, info.ZaFuncName+"@callback",
        call_s{prepared: true, base: lmv, caller: info.CallerEvalfs})

    // Create fresh ident array (same as async tasks and normal calls)
    var ident = make([]Variable, identInitialSize)

    // Set call line (0 for callbacks, like async)
    atomic.StoreInt32(&calltable[loc].callLine, 0)

    // Call the function
    ctx := context.Background()
    rcount, _, _, _, callErr := Call(
        ctx,
        MODE_NEW,
        &ident,
        loc,
        ciCallback, // Callback registrant
        false,      // not a method
        nil,        // no method value
        "",         // no kind override
        []string{}, // no arg names
        nil,        // no captured vars
        args...,
    )

    if callErr != nil {
        return nil, callErr
    }

    // Get return value
    calllock.Lock()
    res := calltable[loc].retvals
    calltable[loc].gcShyness = 100
    calltable[loc].gc = true
    calllock.Unlock()

    // Extract single return value
    if rcount == 0 {
        return nil, nil
    }

    if resArray, ok := res.([]any); ok && len(resArray) > 0 {
        return resArray[0], nil
    }

    return res, nil
}

// CGO Trampoline Functions
// These are the actual C-callable functions that bridge to Za

//export za_callback_with_context_ptr_ptr_int
func za_callback_with_context_ptr_ptr_int(arg1, arg2 unsafe.Pointer, context C.uintptr_t) C.int {
    // Restore CallbackInfo from cgo.Handle
    h := cgo.Handle(context)
    info := h.Value().(*CallbackInfo)

    // Convert pointers to Za values
    ptr1 := NewCPointer(arg1, "callback_arg")
    ptr2 := NewCPointer(arg2, "callback_arg")

    // Call Za function
    result, err := invokeZaCallback(info, ptr1, ptr2)
    if err != nil {
        // Return 0 on error (safe default for comparators)
        return 0
    }

    // Convert result to int
    if result == nil {
        return 0
    }

    switch v := result.(type) {
    case int:
        return C.int(v)
    case int64:
        return C.int(v)
    case uint:
        return C.int(v)
    case uint64:
        return C.int(v)
    case float64:
        return C.int(v)
    default:
        return 0
    }
}

//export za_callback_with_context_int_int_int
func za_callback_with_context_int_int_int(arg1, arg2 C.int, context C.uintptr_t) C.int {
    // Restore CallbackInfo from cgo.Handle
    h := cgo.Handle(context)
    info := h.Value().(*CallbackInfo)

    // Call Za function with int arguments
    result, err := invokeZaCallback(info, int(arg1), int(arg2))
    if err != nil {
        return 0
    }

    if result == nil {
        return 0
    }

    switch v := result.(type) {
    case int:
        return C.int(v)
    case int64:
        return C.int(v)
    case uint:
        return C.int(v)
    case uint64:
        return C.int(v)
    case float64:
        return C.int(v)
    default:
        return 0
    }
}

//export za_callback_ptr_ptr
func za_callback_ptr_ptr(arg unsafe.Pointer) unsafe.Pointer {
    // For pthread_create, the context is passed as the arg itself
    // Extract handle from arg
    h := cgo.Handle(uintptr(arg))
    info := h.Value().(*CallbackInfo)

    // Call Za function
    result, err := invokeZaCallback(info, NewCPointer(arg, "thread_arg"))
    if err != nil {
        return nil
    }

    // Convert result back to pointer
    if result == nil {
        return nil
    }

    if ptr, ok := result.(*CPointerValue); ok {
        return ptr.Ptr
    }

    return nil
}

//export za_callback_sigaction
func za_callback_sigaction(signum C.int, info unsafe.Pointer, context unsafe.Pointer) {
    // For sigaction with SA_SIGINFO, context might contain our handle
    // This is more complex - for now, we'll use a simpler approach
    // where the handle is stored in a global map keyed by signal number

    // TODO: Implement proper sigaction support with context handling
    // For now, just call with signum
    _ = info
    _ = context
}

//export za_callback_int_void
func za_callback_int_void(signum C.int) {
    // Simple signal handler without context
    // This won't work with our context-based approach
    // User must use sigaction with SA_SIGINFO instead

    // TODO: Could use global map: signal number → callback handle
    _ = signum
}

// NEW CALLBACK TRAMPOLINES - Additional common signatures

//export za_callback_double_double
func za_callback_double_double(arg C.double, context C.uintptr_t) C.double {
    // Restore CallbackInfo from cgo.Handle
    h := cgo.Handle(context)
    info := h.Value().(*CallbackInfo)

    // Call Za function with double argument
    result, err := invokeZaCallback(info, float64(arg))
    if err != nil {
        return 0.0 // Safe default for math functions
    }

    if result == nil {
        return 0.0
    }

    // Convert result to double
    switch v := result.(type) {
    case float64:
        return C.double(v)
    case float32:
        return C.double(v)
    case int:
        return C.double(v)
    case int64:
        return C.double(v)
    case uint:
        return C.double(v)
    case uint64:
        return C.double(v)
    default:
        return 0.0
    }
}

//export za_callback_ptr_ptr_ptr_int
func za_callback_ptr_ptr_ptr_int(arg1, arg2, arg3 unsafe.Pointer, context C.uintptr_t) C.int {
    // Restore CallbackInfo from cgo.Handle
    h := cgo.Handle(context)
    info := h.Value().(*CallbackInfo)

    // Convert pointers to Za values
    ptr1 := NewCPointer(arg1, "callback_arg")
    ptr2 := NewCPointer(arg2, "callback_arg")
    ptr3 := NewCPointer(arg3, "callback_arg")

    // Call Za function
    result, err := invokeZaCallback(info, ptr1, ptr2, ptr3)
    if err != nil {
        return 0 // Safe default for comparators
    }

    if result == nil {
        return 0
    }

    // Convert result to int
    switch v := result.(type) {
    case int:
        return C.int(v)
    case int64:
        return C.int(v)
    case uint:
        return C.int(v)
    case uint64:
        return C.int(v)
    case float64:
        return C.int(v)
    default:
        return 0
    }
}

//export za_callback_void_void
func za_callback_void_void(context C.uintptr_t) {
    // Restore CallbackInfo from cgo.Handle
    h := cgo.Handle(context)
    info := h.Value().(*CallbackInfo)

    // Call Za function with no arguments
    _, _ = invokeZaCallback(info)
    // Ignore errors and return value for void->void callbacks
}

//export za_callback_ptr_void
func za_callback_ptr_void(arg unsafe.Pointer, context C.uintptr_t) {
    // Restore CallbackInfo from cgo.Handle
    h := cgo.Handle(context)
    info := h.Value().(*CallbackInfo)

    // Convert pointer to Za value
    ptr := NewCPointer(arg, "callback_arg")

    // Call Za function
    _, _ = invokeZaCallback(info, ptr)
    // Ignore errors and return value for ptr->void callbacks
}

// HIGH PRIORITY CALLBACK TRAMPOLINES

//export za_callback_int_int
func za_callback_int_int(arg C.int, context C.uintptr_t) C.int {
    // Restore CallbackInfo from cgo.Handle
    h := cgo.Handle(context)
    info := h.Value().(*CallbackInfo)

    // Call Za function with int argument
    result, err := invokeZaCallback(info, int(arg))
    if err != nil {
        return 0 // Safe default
    }

    if result == nil {
        return 0
    }

    // Convert result to int
    switch v := result.(type) {
    case int:
        return C.int(v)
    case int64:
        return C.int(v)
    case uint:
        return C.int(v)
    case uint64:
        return C.int(v)
    case float64:
        return C.int(v)
    default:
        return 0
    }
}

//export za_callback_ptr_int_void
func za_callback_ptr_int_void(arg1 unsafe.Pointer, arg2 C.int, context C.uintptr_t) {
    // Restore CallbackInfo from cgo.Handle
    h := cgo.Handle(context)
    info := h.Value().(*CallbackInfo)

    // Convert arguments to Za values
    ptr := NewCPointer(arg1, "callback_arg")
    length := int(arg2)

    // Call Za function
    _, _ = invokeZaCallback(info, ptr, length)
    // Ignore errors and return value for ptr,int->void callbacks
}

//export za_callback_ptr_int_int
func za_callback_ptr_int_int(arg1 unsafe.Pointer, arg2 C.int, context C.uintptr_t) C.int {
    // Restore CallbackInfo from cgo.Handle
    h := cgo.Handle(context)
    info := h.Value().(*CallbackInfo)

    // Convert arguments to Za values
    ptr := NewCPointer(arg1, "callback_arg")
    length := int(arg2)

    // Call Za function
    result, err := invokeZaCallback(info, ptr, length)
    if err != nil {
        return 0 // Safe default (failure/invalid)
    }

    if result == nil {
        return 0
    }

    // Convert result to int
    switch v := result.(type) {
    case int:
        return C.int(v)
    case int64:
        return C.int(v)
    case uint:
        return C.int(v)
    case uint64:
        return C.int(v)
    case bool:
        if v {
            return 1
        }
        return 0
    default:
        return 0
    }
}

//export za_callback_ptr_ptr_void
func za_callback_ptr_ptr_void(arg1, arg2 unsafe.Pointer, context C.uintptr_t) {
    // Restore CallbackInfo from cgo.Handle
    h := cgo.Handle(context)
    info := h.Value().(*CallbackInfo)

    // Convert pointers to Za values
    ptr1 := NewCPointer(arg1, "callback_arg")
    ptr2 := NewCPointer(arg2, "callback_arg")

    // Call Za function
    _, _ = invokeZaCallback(info, ptr1, ptr2)
    // Ignore errors and return value for ptr,ptr->void callbacks
}

//export za_callback_float_float
func za_callback_float_float(arg C.float, context C.uintptr_t) C.float {
    // Restore CallbackInfo from cgo.Handle
    h := cgo.Handle(context)
    info := h.Value().(*CallbackInfo)

    // Call Za function with float argument (promoted to float64 in Za)
    result, err := invokeZaCallback(info, float64(arg))
    if err != nil {
        return 0.0 // Safe default
    }

    if result == nil {
        return 0.0
    }

    // Convert result to float
    switch v := result.(type) {
    case float64:
        return C.float(v)
    case float32:
        return C.float(v)
    case int:
        return C.float(v)
    case int64:
        return C.float(v)
    case uint:
        return C.float(v)
    case uint64:
        return C.float(v)
    default:
        return 0.0
    }
}

//export za_callback_string_void
func za_callback_string_void(arg *C.char, context C.uintptr_t) {
    // Restore CallbackInfo from cgo.Handle
    h := cgo.Handle(context)
    info := h.Value().(*CallbackInfo)

    // Convert C string to Go string
    msg := C.GoString(arg)

    // Call Za function
    _, _ = invokeZaCallback(info, msg)
    // Ignore errors and return value for string->void callbacks
}

//export za_callback_double_double_double
func za_callback_double_double_double(arg1, arg2 C.double, context C.uintptr_t) C.double {
    // Restore CallbackInfo from cgo.Handle
    h := cgo.Handle(context)
    info := h.Value().(*CallbackInfo)

    // Call Za function with two double arguments
    result, err := invokeZaCallback(info, float64(arg1), float64(arg2))
    if err != nil {
        return 0.0 // Safe default
    }

    if result == nil {
        return 0.0
    }

    // Convert result to double
    switch v := result.(type) {
    case float64:
        return C.double(v)
    case float32:
        return C.double(v)
    case int:
        return C.double(v)
    case int64:
        return C.double(v)
    case uint:
        return C.double(v)
    case uint64:
        return C.double(v)
    default:
        return 0.0
    }
}

// ADDITIONAL USEFUL CALLBACK TRAMPOLINES

//export za_callback_string_int
func za_callback_string_int(arg *C.char, context C.uintptr_t) C.int {
    // Restore CallbackInfo from cgo.Handle
    h := cgo.Handle(context)
    info := h.Value().(*CallbackInfo)

    // Convert C string to Go string
    str := C.GoString(arg)

    // Call Za function
    result, err := invokeZaCallback(info, str)
    if err != nil {
        return 0 // Safe default
    }

    if result == nil {
        return 0
    }

    // Convert result to int
    switch v := result.(type) {
    case int:
        return C.int(v)
    case int64:
        return C.int(v)
    case uint:
        return C.int(v)
    case uint64:
        return C.int(v)
    case bool:
        if v {
            return 1
        }
        return 0
    default:
        return 0
    }
}

//export za_callback_int_int_void
func za_callback_int_int_void(arg1, arg2 C.int, context C.uintptr_t) {
    // Restore CallbackInfo from cgo.Handle
    h := cgo.Handle(context)
    info := h.Value().(*CallbackInfo)

    // Call Za function with two int arguments
    _, _ = invokeZaCallback(info, int(arg1), int(arg2))
    // Ignore errors and return value for int,int->void callbacks
}

//export za_callback_ptr_ptr_bool
func za_callback_ptr_ptr_bool(arg1, arg2 unsafe.Pointer, context C.uintptr_t) C.int {
    // Restore CallbackInfo from cgo.Handle
    h := cgo.Handle(context)
    info := h.Value().(*CallbackInfo)

    // Convert pointers to Za values
    ptr1 := NewCPointer(arg1, "callback_arg")
    ptr2 := NewCPointer(arg2, "callback_arg")

    // Call Za function
    result, err := invokeZaCallback(info, ptr1, ptr2)
    if err != nil {
        return 0 // Safe default (false)
    }

    if result == nil {
        return 0
    }

    // Convert result to bool (as int)
    switch v := result.(type) {
    case bool:
        if v {
            return 1
        }
        return 0
    case int:
        if v != 0 {
            return 1
        }
        return 0
    case int64:
        if v != 0 {
            return 1
        }
        return 0
    default:
        return 0
    }
}

// Za stdlib functions

func init() {
    // Register callback functions in stdlib

    stdlib["c_register_callback"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (any, error) {
        if len(args) < 2 {
            return nil, fmt.Errorf("c_register_callback requires 2 arguments: function_name, signature")
        }

        funcName := GetAsString(args[0])
        signature := GetAsString(args[1])

        // If function name doesn't contain ::, prepend current namespace
        fullFuncName := funcName
        if !strings.Contains(funcName, "::") {
            fullFuncName = ns + "::" + funcName
        }

        // Verify function exists
        _, isfunc := fnlookup.lmget(fullFuncName)
        if !isfunc {
            return nil, fmt.Errorf("function %s not found", fullFuncName)
        }

        // Allocate callback ID
        callbackID := int(atomic.AddInt32(&callbackCounter, 1))

        // Create callback info
        info := &CallbackInfo{
            ZaFuncName:   fullFuncName,
            CallerEvalfs: evalfs,
            Signature:    signature,
            CallbackID:   callbackID,
        }

        // Create cgo.Handle to safely pass through C
        h := cgo.NewHandle(info)

        callbackMutex.Lock()
        callbackHandles[callbackID] = h
        callbackMutex.Unlock()

        // Get trampoline pointer for signature
        trampolinePtr, err := getTrampolineForSignature(signature)
        if err != nil {
            // Try creating a dynamic closure as fallback
            var cleanup func()
            var closureErr error
            trampolinePtr, cleanup, closureErr = createFFIClosure(signature, h)
            if closureErr != nil {
                // Both hardcoded trampoline and closure failed - cleanup and return error
                callbackMutex.Lock()
                delete(callbackHandles, callbackID)
                callbackMutex.Unlock()
                h.Delete()
                return nil, fmt.Errorf("no trampoline for signature '%s' and closure creation failed: %w", signature, closureErr)
            }
            // Closure created successfully - store cleanup function
            info.ClosureCleanup = cleanup
        }

        // Return map with both trampoline and handle (Option B from plan)
        return map[string]any{
            "trampoline": NewCPointer(trampolinePtr, "callback_trampoline"),
            "handle":     NewCPointer(unsafe.Pointer(uintptr(h)), "callback_handle"),
        }, nil
    }

    stdlib["c_unregister_callback"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (any, error) {
        if len(args) < 1 {
            return nil, fmt.Errorf("c_unregister_callback requires callback object")
        }

        // Extract handle from the map returned by c_register_callback
        cbMap, ok := args[0].(map[string]any)
        if !ok {
            return nil, fmt.Errorf("c_unregister_callback requires callback object from c_register_callback")
        }

        handleVal, ok := cbMap["handle"]
        if !ok {
            return nil, fmt.Errorf("callback object missing handle field")
        }

        ptr, ok := handleVal.(*CPointerValue)
        if !ok {
            return nil, fmt.Errorf("callback handle is not a pointer")
        }

        h := cgo.Handle(uintptr(ptr.Ptr))

        // Get CallbackInfo to find ID
        info := h.Value().(*CallbackInfo)

        // Call closure cleanup if present (for dynamic closures)
        if info.ClosureCleanup != nil {
            info.ClosureCleanup()
        }

        callbackMutex.Lock()
        delete(callbackHandles, info.CallbackID)
        callbackMutex.Unlock()

        // Delete handle to free resources
        h.Delete()

        return nil, nil
    }

    stdlib["c_get_trampoline"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (any, error) {
        if len(args) < 1 {
            return nil, fmt.Errorf("c_get_trampoline requires signature string")
        }

        signature := GetAsString(args[0])

        trampolinePtr, err := getTrampolineForSignature(signature)
        if err != nil {
            return nil, err
        }

        return NewCPointer(trampolinePtr, "callback_trampoline"), nil
    }
}

// getTrampolineForSignature returns the appropriate trampoline function pointer for a signature
func getTrampolineForSignature(sig string) (unsafe.Pointer, error) {
    switch sig {
    case "ptr,ptr->int":
        // qsort_r comparator: int (*)(const void*, const void*, void*)
        return C.za_callback_with_context_ptr_ptr_int, nil

    case "int,int->int":
        // Integer comparator with context
        return C.za_callback_with_context_int_int_int, nil

    case "ptr->ptr":
        // pthread_create: void* (*)(void*)
        return C.za_callback_ptr_ptr, nil

    case "int,ptr,ptr->void":
        // sigaction with SA_SIGINFO: void (*)(int, siginfo_t*, void*)
        return C.za_callback_sigaction, nil

    case "int->void":
        // Simple signal handler: void (*)(int)
        return C.za_callback_int_void, nil

    // NEW SIGNATURES
    case "double->double":
        // Math transformations: double (*)(double, void*)
        return C.za_callback_double_double, nil

    case "ptr,ptr,ptr->int":
        // 3-pointer comparator: int (*)(void*, void*, void*, void*)
        return C.za_callback_ptr_ptr_ptr_int, nil

    case "void->void":
        // Simple callback: void (*)(void*)
        return C.za_callback_void_void, nil

    case "ptr->void":
        // Cleanup/destructor: void (*)(void*, void*)
        return C.za_callback_ptr_void, nil

    // HIGH PRIORITY ADDITIONS
    case "int->int":
        // Hash functions, transforms: int (*)(int, void*)
        return C.za_callback_int_int, nil

    case "ptr,int->void":
        // Buffer processors: void (*)(void*, size_t, void*)
        return C.za_callback_ptr_int_void, nil

    case "ptr,int->int":
        // Validators with length: int (*)(void*, size_t, void*)
        return C.za_callback_ptr_int_int, nil

    case "ptr,ptr->void":
        // Iteration callbacks: void (*)(void*, void*, void*)
        return C.za_callback_ptr_ptr_void, nil

    case "float->float":
        // Single-precision math: float (*)(float, void*)
        return C.za_callback_float_float, nil

    case "string->void":
        // Logging callbacks: void (*)(const char*, void*)
        return C.za_callback_string_void, nil

    case "double,double->double":
        // Binary math operations: double (*)(double, double, void*)
        return C.za_callback_double_double_double, nil

    // ADDITIONAL USEFUL SIGNATURES
    case "string->int":
        // String validators/parsers: int (*)(const char*, void*)
        return C.za_callback_string_int, nil

    case "int,int->void":
        // Progress callbacks, range handlers: void (*)(int, int, void*)
        return C.za_callback_int_int_void, nil

    case "ptr,ptr->bool":
        // Predicate functions: int (*)(void*, void*, void*) returning bool as int
        return C.za_callback_ptr_ptr_bool, nil

    default:
        return nil, fmt.Errorf("no trampoline available for signature %s\nSupported signatures:\n  Core: ptr,ptr->int, int,int->int, ptr->ptr, int,ptr,ptr->void, int->void\n  Math: double->double, float->float, double,double->double\n  Pointers: ptr,ptr,ptr->int, void->void, ptr->void, ptr,ptr->void, ptr,int->void, ptr,int->int, ptr,ptr->bool\n  Primitives: int->int, string->void, string->int, int,int->void", sig)
    }
}

