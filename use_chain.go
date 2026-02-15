package main

import (
    "slices"
    "strings"
    "sync"
)

var chainlock = &sync.RWMutex{} // chain access lock

var uchain = make([]string, 0)
var ustack = make([][]string, 0)

func uc_add(s string) bool {
    chainlock.Lock()
    defer chainlock.Unlock()

    if slices.Contains(uchain, s) {
        return false
    }

    uchain = append(uchain, s)
    return true
}

func uc_remove(s string) bool {
    chainlock.Lock()
    defer chainlock.Unlock()
    for p := 0; p < len(uchain); p += 1 {
        if uchain[p] == s {
            na := make([]string, len(uchain)-1)
            if p != 0 {
                copy(na, uchain[:p])
            }
            if p < len(uchain)-1 {
                na = append(na[:p], uchain[p+1:]...)
            }
            uchain = na
            return true
        }
    }
    return false
}

func uc_top(s string) bool {
    uc_remove(s)
    chainlock.Lock()
    defer chainlock.Unlock()
    na := make([]string, 0, len(uchain)+1)
    na = append(na, s)
    for p := 0; p < len(uchain); p += 1 {
        na = append(na, uchain[p])
    }
    uchain = na
    return true
}

func uc_reset() {
    chainlock.Lock()
    defer chainlock.Unlock()
    na := make([]string, 0)
    uchain = na
}

func ucs_push() bool {
    chainlock.Lock()
    defer chainlock.Unlock()
    na := make([][]string, 0, len(ustack))
    na = append(na, uchain)
    for p := 0; p < len(ustack); p += 1 {
        na = append(na, ustack[p])
    }
    ustack = na
    return true
}

func ucs_pop() bool {
    chainlock.Lock()
    defer chainlock.Unlock()
    if len(ustack) == 0 {
        return false
    }
    uchain = ustack[0]
    ustack = ustack[1:]
    return true
}

func uc_show() {
    chainlock.RLock()
    defer chainlock.RUnlock()
    pf("USE chain\n")
    if len(uchain) == 0 {
        pf("[#2]Empty[#-]\n")
    }
    for p := 0; p < len(uchain); p += 1 {
        pf("[%2d] %s\n", p, uchain[p])
    }
}

func uc_match_func(s string) string {
    chainlock.RLock()
    defer chainlock.RUnlock()
    for p := 0; p < len(uchain); p += 1 {
        if fnlookup.lmexists(uchain[p] + "::" + s) {
            return uchain[p]
        }
        // Check if this is a C library module and has the function
        if lib, exists := loadedCLibraries[uchain[p]]; exists {
            if _, symbolExists := lib.Symbols[s]; symbolExists {
                return uchain[p]
            }
        }
    }
    return ""
}

func uc_match_enum(s string) string {
    chainlock.RLock()
    globlock.RLock()
    defer globlock.RUnlock()
    defer chainlock.RUnlock()
    for p := 0; p < len(uchain); p += 1 {
        if _, found := enum[uchain[p]+"::"+s]; found {
            return uchain[p]
        }
    }
    return ""
}

func uc_match_constant(s string) (string, any, bool) {
    chainlock.RLock()
    defer chainlock.RUnlock()

    /*
    *  @note(DH):
    *  we may need to re-enable this lock if we see
    *  any unusual behaviour with C constants.
    *  it is disabled for now as pprof was showing
    *  a huge spike in time spent in uc_match_constant,
    *  RUnlock, RLock and sync/Atomic.Add causing a
    *  degradation in za performance of around 20-30%
    *  (or I write better code, but that ain't happening.)
    */
    // moduleConstantsLock.RLock()
    // defer moduleConstantsLock.RUnlock()

    for p := 0; p < len(uchain); p += 1 {
        if constMap, exists := moduleConstants[uchain[p]]; exists {
            if val, found := constMap[s]; found {
                return uchain[p], val, true
            }
        }
    }
    return "", nil, false
}

func uc_match_struct(s string) string {
    chainlock.RLock()
    globlock.RLock()
    defer globlock.RUnlock()
    defer chainlock.RUnlock()
    for p := 0; p < len(uchain); p += 1 {
        // Check Za-defined structs
        structmapslock.RLock()
        if _, exists := structmaps[uchain[p]+"::"+s]; exists {
            structmapslock.RUnlock()
            return uchain[p]
        }
        structmapslock.RUnlock()

        // Check FFI structs from AUTO imports
        ffiStructLock.RLock()
        if _, exists := ffiStructDefinitions[uchain[p]+"::"+s]; exists {
            ffiStructLock.RUnlock()
            return uchain[p]
        }
        ffiStructLock.RUnlock()
    }
    return ""
}

func uc_match_c_func(s string) string {
    chainlock.RLock()
    defer chainlock.RUnlock()

    // Single pass: iterate use chain in order
    // declaredSignatures is the single source of truth for both MANUAL and AUTO LIB declarations
    // This ensures use chain order is respected, not random map iteration
    for p := 0; p < len(uchain); p += 1 {
        if _, exists := GetDeclaredSignature(uchain[p], s); exists {
            return uchain[p]
        }
    }
    return ""
}

// uc_match_ffi_struct resolves a bare struct name through the use chain
// and returns the fully qualified name (namespace::structName).
// Returns empty string if not found in any namespace.
func uc_match_ffi_struct(s string) string {
    if strings.Contains(s, "::") {
        return s  // Already qualified
    }

    if namespace := uc_match_struct(s); namespace != "" {
        return namespace + "::" + s
    }

    return ""  // Not found in any namespace
}

// uc_match_typedef searches for a typedef through the use chain
// Returns the library namespace where the typedef is defined, or empty string if not found
func uc_match_typedef(typeName string) string {
    chainlock.RLock()
    // Make a copy of the chain so we can release the lock before checking typedefs
    searchChain := make([]string, len(uchain))
    copy(searchChain, uchain)
    chainlock.RUnlock()

    // Now check each library in the use chain order
    for _, alias := range searchChain {
        moduleTypedefsLock.RLock()
        if moduleTypedefs[alias] != nil {
            if _, exists := moduleTypedefs[alias][typeName]; exists {
                moduleTypedefsLock.RUnlock()
                return alias
            }
        }
        moduleTypedefsLock.RUnlock()
    }

    return ""  // Not found in any namespace
}
