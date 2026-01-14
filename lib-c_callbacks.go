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
    ZaFuncName   string // Name of Za function to call
    CallerEvalfs uint32 // evalfs of the context where callback was registered
    Signature    string // e.g., "int,int->int"
    CallbackID   int    // Unique ID for this callback
}

// Global callback registry using cgo.Handle for safe passing through C
var callbackHandles = make(map[int]cgo.Handle) // callbackID → cgo.Handle
var callbackMutex sync.RWMutex
var callbackCounter int32

// Mutex to serialize callback invocations for thread safety
var callbackInvocationMutex sync.Mutex

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
            // Cleanup on error
            callbackMutex.Lock()
            delete(callbackHandles, callbackID)
            callbackMutex.Unlock()
            h.Delete()
            return nil, err
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

    default:
        return nil, fmt.Errorf("no trampoline available for signature %s (supported: ptr,ptr->int, int,int->int, ptr->ptr, int,ptr,ptr->void, int->void)", sig)
    }
}

