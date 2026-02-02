package main

import (
    "crypto/sha256"
    "encoding/gob"
    "encoding/hex"
    "encoding/json"
    "fmt"
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

const FFICacheVersion = 2
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

// hashFile computes a fast hash based on file mtime and size
// This is much faster than SHA256 and sufficient for cache invalidation
func hashFile(filePath string) (string, error) {
    info, err := os.Stat(filePath)
    if err != nil {
        return "", err
    }
    return fmt.Sprintf("%d-%d", info.ModTime().Unix(), info.Size()), nil
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
        return key, err
    }
    key.LibraryHash = libHash

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
            // Continue with other headers - don't fail entire cache on one missing header
            hHash = "error"
        }
        headerHashes = append(headerHashes, hHash)
    }

    key.HeaderPaths = sortedHeaders
    key.HeaderHashes = headerHashes

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
func tryLoadFFICache(cacheKey FFICacheKey) (*FFICacheData, bool) {
    registerGobTypes()

    noCache := os.Getenv("ZA_FFI_NOCACHE") != ""

    if noCache {
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
        return nil, false
    }
    defer f.Close()

    // Decode cache data
    decoder := gob.NewDecoder(f)
    var data FFICacheData
    if err := decoder.Decode(&data); err != nil {
        // Delete corrupted cache
        os.Remove(fullPath)
        return nil, false
    }

    // Validate cache key matches
    if !cacheKeysEqual(data.CacheKey, cacheKey) {
        return nil, false
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
// IMPORTANT: Only restores data for the specific alias to prevent namespace pollution
func populateGlobalMapsFromCache(data *FFICacheData, alias string, fs uint32) error {
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

    // Copy module-specific maps for THIS ALIAS ONLY
    if data.ModuleConstants != nil && data.ModuleConstants[alias] != nil {
        moduleConstants[alias] = data.ModuleConstants[alias]
    }
    if data.ModuleMacros != nil && data.ModuleMacros[alias] != nil {
        moduleMacros[alias] = data.ModuleMacros[alias]
    }
    if data.ModuleMacrosOriginal != nil && data.ModuleMacrosOriginal[alias] != nil {
        moduleMacrosOriginal[alias] = data.ModuleMacrosOriginal[alias]
    }
    if data.ModuleTypedefs != nil && data.ModuleTypedefs[alias] != nil {
        moduleTypedefs[alias] = data.ModuleTypedefs[alias]
    }
    if data.ModuleFuncPtrSigs != nil && data.ModuleFuncPtrSigs[alias] != nil {
        moduleFunctionPointerSignatures[alias] = data.ModuleFuncPtrSigs[alias]
    }

    // Copy global enums (module-independent in this cache)
    // enum_s has unexported fields, so we store EnumData and reconstruct using reflection
    if data.EnumsData != nil {
        for name, enumData := range data.EnumsData {
            reconstructed, err := reconstructEnum(enumData)
            if err != nil {
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

    // Copy declared signatures for THIS ALIAS ONLY
    // Check for existing manual declarations before overwriting
    if data.DeclaredSignatures != nil && data.DeclaredSignatures[alias] != nil {
        if declaredSignatures[alias] == nil {
            declaredSignatures[alias] = make(map[string]CFunctionSignature)
        }
        for funcName, sig := range data.DeclaredSignatures[alias] {
            // Only restore the signature if:
            // 1. It doesn't exist yet, OR
            // 2. The existing one is not manual (can overwrite AUTO with AUTO)
            if existing, hasExisting := declaredSignatures[alias][funcName]; !hasExisting {
                // New function, add it
                declaredSignatures[alias][funcName] = sig
            } else if existing.Source != SourceManual {
                // Existing is AUTO, can be updated
                declaredSignatures[alias][funcName] = sig
            }
            // If existing.Source == SourceManual, skip - don't overwrite manual with cached AUTO
        }
    }

    // Copy auto import errors for THIS ALIAS ONLY
    if data.AutoImportErrors != nil && data.AutoImportErrors[alias] != nil {
        autoImportErrors[alias] = data.AutoImportErrors[alias]
    }

    return nil
}

// saveFFICache collects parsed data and saves to cache
// IMPORTANT: Only saves data for the specific alias to prevent namespace pollution
func saveFFICache(cacheKey FFICacheKey, alias string) error {
    registerGobTypes()

    noCache := os.Getenv("ZA_FFI_NOCACHE") != ""

    if noCache {
        return nil
    }

    // Collect current global state for THIS ALIAS ONLY
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
                continue
            }
            enumsData[name] = enumData
        }
    }

    // Only save data for the current alias
    aliasOnlyConstants := make(map[string]map[string]any)
    if moduleConstants[alias] != nil {
        aliasOnlyConstants[alias] = moduleConstants[alias]
    }

    aliasOnlyMacros := make(map[string]map[string]string)
    if moduleMacros[alias] != nil {
        aliasOnlyMacros[alias] = moduleMacros[alias]
    }

    aliasOnlyMacrosOriginal := make(map[string]map[string]string)
    if moduleMacrosOriginal[alias] != nil {
        aliasOnlyMacrosOriginal[alias] = moduleMacrosOriginal[alias]
    }

    aliasOnlyTypedefs := make(map[string]map[string]string)
    if moduleTypedefs[alias] != nil {
        aliasOnlyTypedefs[alias] = moduleTypedefs[alias]
    }

    aliasOnlyFuncPtrSigs := make(map[string]map[string]CFunctionSignature)
    if moduleFunctionPointerSignatures[alias] != nil {
        aliasOnlyFuncPtrSigs[alias] = moduleFunctionPointerSignatures[alias]
    }

    aliasOnlyDeclaredSigs := make(map[string]map[string]CFunctionSignature)
    if declaredSignatures[alias] != nil {
        aliasOnlyDeclaredSigs[alias] = declaredSignatures[alias]
    }

    aliasOnlyErrors := make(map[string][]string)
    if autoImportErrors[alias] != nil {
        aliasOnlyErrors[alias] = autoImportErrors[alias]
    }

    data := &FFICacheData{
        Version:             FFICacheVersion,
        ModuleConstants:     copyMapMapAny(aliasOnlyConstants),
        ModuleMacros:        copyMapMapString(aliasOnlyMacros),
        ModuleMacrosOriginal: copyMapMapString(aliasOnlyMacrosOriginal),
        ModuleTypedefs:      copyMapMapString(aliasOnlyTypedefs),
        ModuleFuncPtrSigs:   copyMapMapSignature(aliasOnlyFuncPtrSigs),
        FFIStructs:          copyMapStruct(ffiStructDefinitions),
        EnumsData:           enumsData,
        DeclaredSignatures:  copyMapMapSignature(aliasOnlyDeclaredSigs),
        AutoImportErrors:    copyMapSliceString(aliasOnlyErrors),
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
        return nil
    }

    if err := os.MkdirAll(cachePath, 0755); err != nil {
        return nil
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
        return nil
    }

    encoder := gob.NewEncoder(f)
    if err := encoder.Encode(data); err != nil {
        f.Close()
        os.Remove(tempPath)
        return nil
    }
    f.Close()

    // Atomic rename
    if err := os.Rename(tempPath, fullPath); err != nil {
        os.Remove(tempPath)
        return nil
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
