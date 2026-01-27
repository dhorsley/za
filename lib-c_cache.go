package main

import (
    "crypto/sha256"
    "encoding/gob"
    "encoding/hex"
    "encoding/json"
    "fmt"
    "io"
    "os"
    "path/filepath"
    "reflect"
    "runtime"
    "sort"
    "strings"
    "sync"
    "time"
    "unsafe"
)

// FFICacheKey is the complete key used to validate cache validity
type FFICacheKey struct {
    // Platform (caches are NOT portable across architectures)
    OS          string // runtime.GOOS
    Arch        string // runtime.GOARCH
    PointerSize int    // 32-bit vs 64-bit

    // Library identification
    LibraryPath string // Absolute path to .so
    LibraryHash string // SHA256 of .so file

    // Header identification (all recursively #included headers)
    HeaderPaths []string // Sorted list of all headers
    HeaderHashes []string // SHA256 of each header content

    // Za version (for format compatibility)
    ZaVersion string // BuildVersion
}

// EnumData is a cacheable representation of enum_s (which has unexported fields)
type EnumData struct {
    Members   map[string]any
    Ordered   []string
    Namespace string
}

// FFICacheData contains all the data to cache from header parsing
type FFICacheData struct {
    Version int // Cache format version (currently 1)

    // Global maps from parsing
    ModuleConstants        map[string]map[string]any
    ModuleMacros           map[string]map[string]string
    ModuleMacrosOriginal   map[string]map[string]string
    ModuleTypedefs         map[string]map[string]string
    ModuleFuncPtrSigs      map[string]map[string]CFunctionSignature
    FFIStructs             map[string]*CLibraryStruct
    EnumsData              map[string]EnumData // Serializable version of Enums
    DeclaredSignatures     map[string]map[string]CFunctionSignature
    AutoImportErrors       map[string][]string

    // Metadata
    CacheKey  FFICacheKey
    CreatedAt time.Time
}

const FFICacheVersion = 1
const FFICacheDir = ".cache/za/ffi"

// registerGobTypes registers types for gob encoding
// This is called on first cache access to ensure all types are defined
var gobTypesRegistered = false
var registerGobOnce sync.Once

func registerGobTypes() {
    registerGobOnce.Do(func() {
        gob.Register(&CLibraryStruct{})
        gob.Register(&StructField{})
        gob.Register(&CFunctionSignature{})
        gob.Register(&enum_s{})
        gob.Register(CType(0))
        gob.Register(map[string]interface{}{})
        gob.Register(map[string]*CLibraryStruct{})
        gob.Register(map[string]*enum_s{})
        gob.Register(map[string]map[string]CFunctionSignature{})
        gob.Register([]StructField{})
        gob.Register([]*CLibraryStruct{})
        gobTypesRegistered = true
    })
}

// getCachePath returns the cache directory path
func getCachePath() (string, error) {
    home, err := os.UserHomeDir()
    if err != nil {
        return "", err
    }
    cachePath := filepath.Join(home, FFICacheDir)
    return cachePath, nil
}

// hashFile computes SHA256 of a file
func hashFile(filePath string) (string, error) {
    f, err := os.Open(filePath)
    if err != nil {
        return "", err
    }
    defer f.Close()

    h := sha256.New()
    if _, err := io.Copy(h, f); err != nil {
        return "", err
    }

    return hex.EncodeToString(h.Sum(nil)), nil
}

// hashString computes SHA256 of a string
func hashString(s string) string {
    h := sha256.Sum256([]byte(s))
    return hex.EncodeToString(h[:])
}

// hashObject computes SHA256 of a JSON-encoded object
func hashObject(obj interface{}) (string, error) {
    data, err := json.Marshal(obj)
    if err != nil {
        return "", err
    }
    return hashString(string(data)), nil
}

// computeCacheKey builds the complete cache key
func computeCacheKey(libraryPath, alias string, explicitPaths []string) (FFICacheKey, error) {
    debugCache := os.Getenv("ZA_FFI_CACHE_DEBUG") != ""

    key := FFICacheKey{
        OS:          runtime.GOOS,
        Arch:        runtime.GOARCH,
        PointerSize: int(unsafe.Sizeof(uintptr(0))) * 8,
        LibraryPath: libraryPath,
        ZaVersion:   BuildVersion,
    }

    // Hash the library file
    libHash, err := hashFile(libraryPath)
    if err != nil {
        if debugCache {
            fmt.Fprintf(os.Stderr, "[FFI-CACHE] Failed to hash library %s: %v\n", libraryPath, err)
        }
        return key, err
    }
    key.LibraryHash = libHash

    if debugCache {
        fmt.Fprintf(os.Stderr, "[FFI-CACHE] Library hash: %s\n", libHash)
    }

    // Discover header paths if not explicit
    var headerPaths []string
    if len(explicitPaths) > 0 {
        headerPaths = explicitPaths
    } else {
        headerPaths = discoverHeaders(libraryPath)
    }

    // Scan headers recursively to get all included headers
    allHeaders := make(map[string]bool)
    for _, hpath := range headerPaths {
        scanHeadersRecursive(hpath, allHeaders)
    }

    // Convert to sorted list
    var sortedHeaders []string
    for h := range allHeaders {
        sortedHeaders = append(sortedHeaders, h)
    }
    sort.Strings(sortedHeaders)

    // Hash each header
    var headerHashes []string
    for _, hpath := range sortedHeaders {
        hHash, err := hashFile(hpath)
        if err != nil {
            if debugCache {
                fmt.Fprintf(os.Stderr, "[FFI-CACHE] Failed to hash header %s: %v\n", hpath, err)
            }
            // Continue with other headers - don't fail entire cache on one missing header
            hHash = "error"
        }
        headerHashes = append(headerHashes, hHash)
    }

    key.HeaderPaths = sortedHeaders
    key.HeaderHashes = headerHashes

    if debugCache {
        fmt.Fprintf(os.Stderr, "[FFI-CACHE] Scanning %d headers\n", len(sortedHeaders))
    }

    return key, nil
}

// scanHeadersRecursive recursively scans a header file and all its includes
func scanHeadersRecursive(filePath string, visited map[string]bool) {
    // Avoid infinite recursion and system headers
    if visited[filePath] {
        return
    }

    // Only cache user headers, skip system headers
    if strings.Contains(filePath, "/usr/include/") {
        visited[filePath] = true
        return
    }

    visited[filePath] = true

    data, err := os.ReadFile(filePath)
    if err != nil {
        return
    }

    content := string(data)
    lines := strings.Split(content, "\n")

    for _, line := range lines {
        line = strings.TrimSpace(line)
        if strings.HasPrefix(line, "#include") {
            // Extract include path
            var includePath string
            if strings.Contains(line, "\"") {
                // Local include: #include "path.h"
                start := strings.Index(line, "\"")
                end := strings.LastIndex(line, "\"")
                if start >= 0 && end > start {
                    includePath = line[start+1 : end]
                    // Resolve relative to current file's directory
                    dir := filepath.Dir(filePath)
                    fullPath := filepath.Join(dir, includePath)
                    if _, err := os.Stat(fullPath); err == nil {
                        scanHeadersRecursive(fullPath, visited)
                    }
                }
            }
        }
    }
}

// getCacheFileName computes the cache filename from the cache key
func getCacheFileName(cacheKey FFICacheKey) (string, error) {
    keyJSON, err := json.Marshal(cacheKey)
    if err != nil {
        return "", err
    }
    keyHash := hashString(string(keyJSON))
    return keyHash + ".cache", nil
}

// tryLoadFFICache attempts to load cached data
func tryLoadFFICache(libraryPath, alias string, explicitPaths []string) (*FFICacheData, bool) {
    registerGobTypes()

    debugCache := os.Getenv("ZA_FFI_CACHE_DEBUG") != ""
    noCache := os.Getenv("ZA_FFI_NOCACHE") != ""

    if noCache {
        if debugCache {
            fmt.Fprintf(os.Stderr, "[FFI-CACHE] Cache disabled via ZA_FFI_NOCACHE\n")
        }
        return nil, false
    }

    // Compute cache key
    cacheKey, err := computeCacheKey(libraryPath, alias, explicitPaths)
    if err != nil {
        if debugCache {
            fmt.Fprintf(os.Stderr, "[FFI-CACHE] Failed to compute cache key: %v\n", err)
        }
        return nil, false
    }

    // Get cache filename
    fileName, err := getCacheFileName(cacheKey)
    if err != nil {
        return nil, false
    }

    cachePath, err := getCachePath()
    if err != nil {
        return nil, false
    }

    fullPath := filepath.Join(cachePath, fileName)

    // Check if cache file exists
    f, err := os.Open(fullPath)
    if err != nil {
        if debugCache {
            fmt.Fprintf(os.Stderr, "[FFI-CACHE] Cache miss: %s\n", fileName)
        }
        return nil, false
    }
    defer f.Close()

    // Decode cache data
    decoder := gob.NewDecoder(f)
    var data FFICacheData
    if err := decoder.Decode(&data); err != nil {
        if debugCache {
            fmt.Fprintf(os.Stderr, "[FFI-CACHE] Cache decode failed: %v (will regenerate)\n", err)
        }
        // Delete corrupted cache
        os.Remove(fullPath)
        return nil, false
    }

    // Validate cache key matches
    if !cacheKeysEqual(data.CacheKey, cacheKey) {
        if debugCache {
            fmt.Fprintf(os.Stderr, "[FFI-CACHE] Cache key mismatch (will regenerate)\n")
        }
        return nil, false
    }

    if debugCache {
        fmt.Fprintf(os.Stderr, "[FFI-CACHE] Cache hit: %s\n", fileName)
        fmt.Fprintf(os.Stderr, "[FFI-CACHE] Loaded: %d constants, %d macros, %d structs, %d functions\n",
            len(moduleConstants[alias]),
            len(moduleMacros[alias]),
            len(ffiStructDefinitions),
            len(declaredSignatures[alias]))
    }

    return &data, true
}

// cacheKeysEqual compares two cache keys
func cacheKeysEqual(a, b FFICacheKey) bool {
    if a.OS != b.OS || a.Arch != b.Arch || a.PointerSize != b.PointerSize {
        return false
    }
    if a.LibraryPath != b.LibraryPath || a.LibraryHash != b.LibraryHash {
        return false
    }
    if a.ZaVersion != b.ZaVersion {
        return false
    }
    if len(a.HeaderPaths) != len(b.HeaderPaths) || len(a.HeaderHashes) != len(b.HeaderHashes) {
        return false
    }
    for i := range a.HeaderPaths {
        if a.HeaderPaths[i] != b.HeaderPaths[i] || a.HeaderHashes[i] != b.HeaderHashes[i] {
            return false
        }
    }
    return true
}

// populateGlobalMapsFromCache restores global state from cached data
func populateGlobalMapsFromCache(data *FFICacheData, alias string, fs uint32) error {
    debugCache := os.Getenv("ZA_FFI_CACHE_DEBUG") != ""

    // Lock all global map locks
    moduleConstantsLock.Lock()
    moduleMacrosLock.Lock()
    moduleMacrosOriginalLock.Lock()
    moduleTypedefsLock.Lock()
    moduleFunctionPointerSignaturesLock.Lock()
    ffiStructLock.Lock()
    autoImportErrorsLock.Lock()
    defer func() {
        autoImportErrorsLock.Unlock()
        ffiStructLock.Unlock()
        moduleFunctionPointerSignaturesLock.Unlock()
        moduleTypedefsLock.Unlock()
        moduleMacrosOriginalLock.Unlock()
        moduleMacrosLock.Unlock()
        moduleConstantsLock.Unlock()
    }()

    // Copy module-specific maps
    if data.ModuleConstants != nil {
        moduleConstants[alias] = data.ModuleConstants[alias]
    }
    if data.ModuleMacros != nil {
        moduleMacros[alias] = data.ModuleMacros[alias]
    }
    if data.ModuleMacrosOriginal != nil {
        moduleMacrosOriginal[alias] = data.ModuleMacrosOriginal[alias]
    }
    if data.ModuleTypedefs != nil {
        moduleTypedefs[alias] = data.ModuleTypedefs[alias]
    }
    if data.ModuleFuncPtrSigs != nil {
        moduleFunctionPointerSignatures[alias] = data.ModuleFuncPtrSigs[alias]
    }

    // Copy global enums (module-independent in this cache)
    // enum_s has unexported fields, so we store EnumData and reconstruct using reflection
    if data.EnumsData != nil {
        for name, enumData := range data.EnumsData {
            reconstructed, err := reconstructEnum(enumData)
            if err != nil {
                if debugCache {
                    fmt.Fprintf(os.Stderr, "[FFI-CACHE] Warning: failed to reconstruct enum %s: %v\n", name, err)
                }
                continue
            }
            enum[name] = reconstructed
        }
    }

    // Copy FFI structs
    if data.FFIStructs != nil {
        for name, structDef := range data.FFIStructs {
            ffiStructDefinitions[name] = structDef
        }
    }

    // Copy declared signatures
    if data.DeclaredSignatures != nil {
        for lib, sigs := range data.DeclaredSignatures {
            if declaredSignatures[lib] == nil {
                declaredSignatures[lib] = make(map[string]CFunctionSignature)
            }
            for funcName, sig := range sigs {
                declaredSignatures[lib][funcName] = sig
            }
        }
    }

    // Copy auto import errors
    if data.AutoImportErrors != nil {
        for lib, errs := range data.AutoImportErrors {
            autoImportErrors[lib] = errs
        }
    }

    if debugCache {
        fmt.Fprintf(os.Stderr, "[FFI-CACHE] Populated global maps from cache\n")
    }

    return nil
}

// saveFFICache collects parsed data and saves to cache
func saveFFICache(libraryPath, alias string, explicitPaths []string) error {
    registerGobTypes()

    debugCache := os.Getenv("ZA_FFI_CACHE_DEBUG") != ""
    noCache := os.Getenv("ZA_FFI_NOCACHE") != ""

    if debugCache {
        fmt.Fprintf(os.Stderr, "[FFI-CACHE] saveFFICache called for %s/%s\n", alias, libraryPath)
    }

    if noCache {
        if debugCache {
            fmt.Fprintf(os.Stderr, "[FFI-CACHE] Cache save skipped (noCache=true)\n")
        }
        return nil
    }

    // Compute cache key
    cacheKey, err := computeCacheKey(libraryPath, alias, explicitPaths)
    if err != nil {
        if debugCache {
            fmt.Fprintf(os.Stderr, "[FFI-CACHE] Failed to compute cache key for save: %v\n", err)
        }
        return nil // Don't fail compilation if cache save fails
    }

    // Collect current global state
    moduleConstantsLock.RLock()
    moduleMacrosLock.RLock()
    moduleMacrosOriginalLock.RLock()
    moduleTypedefsLock.RLock()
    moduleFunctionPointerSignaturesLock.RLock()
    ffiStructLock.RLock()
    autoImportErrorsLock.RLock()

    // Extract enum data using reflection (enum_s has unexported fields)
    enumsData := make(map[string]EnumData)
    for name, e := range enum {
        if e != nil {
            enumData, err := extractEnumData(e)
            if err != nil {
                if debugCache {
                    fmt.Fprintf(os.Stderr, "[FFI-CACHE] Warning: failed to extract enum %s: %v\n", name, err)
                }
                continue
            }
            enumsData[name] = enumData
        }
    }

    data := &FFICacheData{
        Version:             FFICacheVersion,
        ModuleConstants:     copyMapMapAny(moduleConstants),
        ModuleMacros:        copyMapMapString(moduleMacros),
        ModuleMacrosOriginal: copyMapMapString(moduleMacrosOriginal),
        ModuleTypedefs:      copyMapMapString(moduleTypedefs),
        ModuleFuncPtrSigs:   copyMapMapSignature(moduleFunctionPointerSignatures),
        FFIStructs:          copyMapStruct(ffiStructDefinitions),
        EnumsData:           enumsData,
        DeclaredSignatures:  copyMapMapSignature(declaredSignatures),
        AutoImportErrors:    copyMapSliceString(autoImportErrors),
        CacheKey:            cacheKey,
        CreatedAt:           time.Now(),
    }

    autoImportErrorsLock.RUnlock()
    ffiStructLock.RUnlock()
    moduleFunctionPointerSignaturesLock.RUnlock()
    moduleTypedefsLock.RUnlock()
    moduleMacrosOriginalLock.RUnlock()
    moduleMacrosLock.RUnlock()
    moduleConstantsLock.RUnlock()

    // Get cache path and ensure directory exists
    cachePath, err := getCachePath()
    if err != nil {
        if debugCache {
            fmt.Fprintf(os.Stderr, "[FFI-CACHE] Failed to get cache path: %v\n", err)
        }
        return nil
    }

    if debugCache {
        fmt.Fprintf(os.Stderr, "[FFI-CACHE] Cache path: %s\n", cachePath)
    }

    if err := os.MkdirAll(cachePath, 0755); err != nil {
        if debugCache {
            fmt.Fprintf(os.Stderr, "[FFI-CACHE] Failed to create cache directory %s: %v\n", cachePath, err)
        }
        return nil
    }

    if debugCache {
        fmt.Fprintf(os.Stderr, "[FFI-CACHE] Cache directory created/exists\n")
    }

    // Get cache filename
    fileName, err := getCacheFileName(cacheKey)
    if err != nil {
        return nil
    }

    fullPath := filepath.Join(cachePath, fileName)
    tempPath := fullPath + ".tmp"

    // Write to temporary file
    f, err := os.Create(tempPath)
    if err != nil {
        if debugCache {
            fmt.Fprintf(os.Stderr, "[FFI-CACHE] Failed to create temp cache file: %v\n", err)
        }
        return nil
    }

    encoder := gob.NewEncoder(f)
    if err := encoder.Encode(data); err != nil {
        f.Close()
        os.Remove(tempPath)
        if debugCache {
            fmt.Fprintf(os.Stderr, "[FFI-CACHE] Failed to encode cache: %v\n", err)
        }
        return nil
    }
    f.Close()

    // Atomic rename
    if err := os.Rename(tempPath, fullPath); err != nil {
        if debugCache {
            fmt.Fprintf(os.Stderr, "[FFI-CACHE] Failed to rename cache file: %v\n", err)
        }
        os.Remove(tempPath)
        return nil
    }

    if debugCache {
        fmt.Fprintf(os.Stderr, "[FFI-CACHE] Saved cache: %s\n", fileName)
    }

    return nil
}

// Copy helper functions
func copyMapMapAny(m map[string]map[string]any) map[string]map[string]any {
    result := make(map[string]map[string]any)
    for k, v := range m {
        if v != nil {
            innerCopy := make(map[string]any)
            for k2, v2 := range v {
                innerCopy[k2] = v2
            }
            result[k] = innerCopy
        }
    }
    return result
}

func copyMapMapString(m map[string]map[string]string) map[string]map[string]string {
    result := make(map[string]map[string]string)
    for k, v := range m {
        if v != nil {
            innerCopy := make(map[string]string)
            for k2, v2 := range v {
                innerCopy[k2] = v2
            }
            result[k] = innerCopy
        }
    }
    return result
}

func copyMapMapSignature(m map[string]map[string]CFunctionSignature) map[string]map[string]CFunctionSignature {
    result := make(map[string]map[string]CFunctionSignature)
    for k, v := range m {
        if v != nil {
            innerCopy := make(map[string]CFunctionSignature)
            for k2, v2 := range v {
                innerCopy[k2] = v2
            }
            result[k] = innerCopy
        }
    }
    return result
}

func copyMapStruct(m map[string]*CLibraryStruct) map[string]*CLibraryStruct {
    result := make(map[string]*CLibraryStruct)
    for k, v := range m {
        result[k] = v
    }
    return result
}

func copyEnums(m map[string]*enum_s) map[string]*enum_s {
    result := make(map[string]*enum_s)
    for k, v := range m {
        result[k] = v
    }
    return result
}

func copyMapSliceString(m map[string][]string) map[string][]string {
    result := make(map[string][]string)
    for k, v := range m {
        if v != nil {
            resultSlice := make([]string, len(v))
            copy(resultSlice, v)
            result[k] = resultSlice
        }
    }
    return result
}

// extractEnumData extracts data from enum_s using reflection
// enum_s has unexported fields: members, ordered, namespace
func extractEnumData(e *enum_s) (EnumData, error) {
    if e == nil {
        return EnumData{}, nil
    }

    val := reflect.ValueOf(e).Elem()

    // Extract members field
    membersField := val.FieldByName("members")
    var members map[string]any
    if membersField.IsValid() && membersField.CanInterface() {
        members = membersField.Interface().(map[string]any)
    }

    // Extract ordered field
    orderedField := val.FieldByName("ordered")
    var ordered []string
    if orderedField.IsValid() && orderedField.CanInterface() {
        ordered = orderedField.Interface().([]string)
    }

    // Extract namespace field
    namespaceField := val.FieldByName("namespace")
    var namespace string
    if namespaceField.IsValid() && namespaceField.CanInterface() {
        namespace = namespaceField.Interface().(string)
    }

    return EnumData{
        Members:   members,
        Ordered:   ordered,
        Namespace: namespace,
    }, nil
}

// reconstructEnum reconstructs an enum_s from EnumData
// This uses reflection to set unexported fields
func reconstructEnum(data EnumData) (*enum_s, error) {
    // Create new enum_s instance
    e := &enum_s{}
    val := reflect.ValueOf(e).Elem()

    // Set members field
    membersField := val.FieldByName("members")
    if membersField.IsValid() && membersField.CanSet() {
        membersField.Set(reflect.ValueOf(data.Members))
    }

    // Set ordered field
    orderedField := val.FieldByName("ordered")
    if orderedField.IsValid() && orderedField.CanSet() {
        orderedField.Set(reflect.ValueOf(data.Ordered))
    }

    // Set namespace field
    namespaceField := val.FieldByName("namespace")
    if namespaceField.IsValid() && namespaceField.CanSet() {
        namespaceField.Set(reflect.ValueOf(data.Namespace))
    }

    return e, nil
}
