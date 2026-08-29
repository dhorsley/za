//go:build !windows && !noffi && cgo
// +build !windows,!noffi,cgo

package main

/*
#include <signal.h>
#include <sys/types.h>

// POSIX signal-handler trampolines. The siginfo_t-based sigaction handler is
// Unix-only, so this file is excluded from Windows builds. Libffi closure
// callbacks (the general callback facility) remain available on Windows.
extern void za_callback_sigaction(int signum, void *info, void *context);
extern void za_callback_int_void(int signum);
*/
import "C"

import (
    "fmt"
    "os"
    "runtime/cgo"
    "strings"
    "sync"
    "sync/atomic"
    "unsafe"
)

// Global registry for signal handlers (int->void callbacks)
// Maps signal number to callback handle
var signalCallbacks = make(map[int]cgo.Handle) // signal number → cgo.Handle
var signalCallbacksMutex sync.RWMutex

//export za_callback_sigaction
func za_callback_sigaction(signum C.int, info unsafe.Pointer, context unsafe.Pointer) {
    // sigaction with SA_SIGINFO: void (*)(int, siginfo_t*, void*)
    // Lookup callback handle from signal number (similar to int->void)

    signalCallbacksMutex.RLock()
    h, exists := signalCallbacks[int(signum)]
    signalCallbacksMutex.RUnlock()

    if !exists {
        // No handler registered for this signal
        return
    }

    // Restore CallbackInfo from cgo.Handle
    callbackInfo := h.Value().(*CallbackInfo)

    // Extract siginfo_t fields
    // Cast to C siginfo_t pointer
    siginfoPtr := (*C.siginfo_t)(info)

    // Create Za map to represent siginfo_t
    // Extract common fields that work across different signal types
    siginfoMap := map[string]any{
        "si_signo": int(siginfoPtr.si_signo),
        "si_errno": int(siginfoPtr.si_errno),
        "si_code":  int(siginfoPtr.si_code),
    }

    // Extract fields that are valid for certain signal types
    // Note: We use unsafe pointer arithmetic to access union members
    // This is platform-specific but works on Linux

    // For signals with process ID (SIGCHLD, SIGTERM from kill(), etc.)
    // We access si_pid which is typically at the same offset across architectures
    // Cast the internal data area to get common fields
    type sigval_union struct {
        sival_int int32
        _         [4]byte // padding to make it pointer-sized
    }

    // Extract si_pid, si_uid (valid for many signal types)
    // These are in the _sifields union but at consistent offsets
    // We'll use the raw bytes approach to stay safe
    dataPtr := (*[128]byte)(unsafe.Pointer(uintptr(info) + unsafe.Sizeof(C.int(0))*3))

    // si_pid is typically the first int in the union (offset 12 bytes on 64-bit)
    si_pid := *(*C.int)(unsafe.Pointer(&dataPtr[0]))
    si_uid := *(*C.uint)(unsafe.Pointer(&dataPtr[4]))

    siginfoMap["si_pid"] = int(si_pid)
    siginfoMap["si_uid"] = int(si_uid)

    // For SIGSEGV, SIGBUS, etc. - si_addr contains faulting address
    // This is also in the union at a specific offset
    si_addr := *(*unsafe.Pointer)(unsafe.Pointer(&dataPtr[8]))
    siginfoMap["si_addr"] = NewCPointer(si_addr, "fault_address")

    // For SIGCHLD - si_status contains exit status/signal
    si_status := *(*C.int)(unsafe.Pointer(&dataPtr[8]))
    siginfoMap["si_status"] = int(si_status)

    // si_value union - includes both int and pointer interpretations
    // This allows user code to access whichever makes sense
    si_value_int := *(*C.int)(unsafe.Pointer(&dataPtr[16]))
    si_value_ptr := *(*unsafe.Pointer)(unsafe.Pointer(&dataPtr[16]))

    siginfoMap["si_value"] = map[string]any{
        "sival_int": int(si_value_int),
        "sival_ptr": NewCPointer(si_value_ptr, "signal_value"),
    }

    // Create context pointer wrapper
    contextPtr := NewCPointer(context, "signal_context")

    // Call Za function with three arguments: signum, siginfo map, context
    _, err := invokeZaCallback(callbackInfo, int(signum), siginfoMap, contextPtr)
    if err != nil {
        // Signal handlers can't return errors, log to stderr
        fmt.Fprintf(os.Stderr, "Error in sigaction handler for signal %d: %v\n", signum, err)
    }
}

//export za_callback_int_void
func za_callback_int_void(signum C.int) {
    // Simple signal handler: void (*)(int)
    // Lookup callback handle from signal number

    signalCallbacksMutex.RLock()
    h, exists := signalCallbacks[int(signum)]
    signalCallbacksMutex.RUnlock()

    if !exists {
        // No handler registered for this signal - this shouldn't happen
        // but fail silently to avoid crashing the program
        return
    }

    // Restore CallbackInfo from cgo.Handle
    info := h.Value().(*CallbackInfo)

    // Call Za function with signal number as argument
    _, err := invokeZaCallback(info, int(signum))
    if err != nil {
        // Signal handlers can't return errors, just log to stderr
        fmt.Fprintf(os.Stderr, "Error in signal handler for signal %d: %v\n", signum, err)
    }
}

func init() {
    stdlib["c_register_signal_handler"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (any, error) {
        if ok, err := expect_args("c_register_signal_handler", args, 2,
            "2", "int", "string",           // Variant 1: signum, function_name
            "3", "int", "string", "string"); // Variant 2: signum, function_name, handler_type
            !ok {
            return nil, err
        }

        signum, bad := GetAsInt(args[0])
        if bad {
            return nil, fmt.Errorf("signal_number must be an integer")
        }

        funcName := GetAsString(args[1])

        // Determine handler type
        handlerType := "simple" // default
        if len(args) >= 3 {
            handlerType = GetAsString(args[2])
            if handlerType != "simple" && handlerType != "sigaction" {
                return nil, fmt.Errorf("handler_type must be 'simple' or 'sigaction', got '%s'", handlerType)
            }
        }

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

        // Create callback info with appropriate signature
        var signature string
        var trampoline unsafe.Pointer

        if handlerType == "sigaction" {
            signature = "sigaction"
            trampoline = unsafe.Pointer(C.za_callback_sigaction)
        } else {
            signature = "int->void"
            trampoline = unsafe.Pointer(C.za_callback_int_void)
        }

        info := &CallbackInfo{
            ZaFuncName:   fullFuncName,
            CallerEvalfs: evalfs,
            Signature:    signature,
            CallbackID:   callbackID,
        }

        // Create cgo.Handle
        h := cgo.NewHandle(info)

        // Store in both registries
        callbackMutex.Lock()
        callbackHandles[callbackID] = h
        callbackMutex.Unlock()

        signalCallbacksMutex.Lock()
        signalCallbacks[int(signum)] = h
        signalCallbacksMutex.Unlock()

        // Return appropriate trampoline
        return NewCPointer(trampoline, "signal_handler"), nil
    }

    stdlib["c_unregister_signal_handler"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (any, error) {
        if len(args) < 1 {
            return nil, fmt.Errorf("c_unregister_signal_handler requires signal_number")
        }

        signum, bad := GetAsInt(args[0])
        if bad {
            return nil, fmt.Errorf("signal_number must be an integer")
        }

        signalCallbacksMutex.Lock()
        h, exists := signalCallbacks[int(signum)]
        if !exists {
            signalCallbacksMutex.Unlock()
            return nil, fmt.Errorf("no signal handler registered for signal %d", signum)
        }
        delete(signalCallbacks, int(signum))
        signalCallbacksMutex.Unlock()

        // Get CallbackInfo to find ID
        info := h.Value().(*CallbackInfo)

        callbackMutex.Lock()
        delete(callbackHandles, info.CallbackID)
        callbackMutex.Unlock()

        // Delete handle to free resources
        h.Delete()

        return nil, nil
    }
}