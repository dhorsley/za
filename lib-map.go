package main

// Deep merge operations for maps
// Pure functional approach - always return new maps, never modify inputs

// Helper function to check if a value is a map
func isMap(val any) bool {
    _, ok := val.(map[string]any)
    return ok
}

// Helper function to deep copy any value, especially nested maps
func deepCopyValue(val any) any {
    if m, isMap := val.(map[string]any); isMap {
        return cloneMap(m)
    }
    // For primitive types (string, int, bool, etc.) - no copy needed
    return val
}

// Helper function to clone a map with deep copying of nested structures
func cloneMap(m map[string]any) map[string]any {
    if m == nil {
        return make(map[string]any)
    }
    result := make(map[string]any, len(m))
    for k, v := range m {
        result[k] = deepCopyValue(v)
    }
    return result
}

// Deep merge maps - recursively merge maps, second operand overwrites first on conflicts
func deepMergeMaps(map1, map2 map[string]any) map[string]any {
    // Always create new map - pure functional approach
    result := make(map[string]any, len(map1)+len(map2))

    // Deep copy all values from map1
    for key, val := range map1 {
        result[key] = deepCopyValue(val)
    }

    // Merge values from map2
    for key, val2 := range map2 {
        if val1, exists := result[key]; exists {
            // Both maps have this key
            if isMap(val1) && isMap(val2) {
                // Both values are maps - merge recursively
                result[key] = deepMergeMaps(val1.(map[string]any), val2.(map[string]any))
            } else {
                // Conflict - second operand overwrites (deep copy)
                result[key] = deepCopyValue(val2)
            }
        } else {
            // Key only exists in map2
            result[key] = deepCopyValue(val2)
        }
    }

    return result
}

// Intersect maps - keep only keys present in both maps, recursively merge nested maps
func intersectMaps(map1, map2 map[string]any) map[string]any {
    result := make(map[string]any)

    // Find keys present in both maps
    for key, val1 := range map1 {
        if val2, exists := map2[key]; exists {
            // Key exists in both maps
            if isMap(val1) && isMap(val2) {
                // Both values are maps - intersect recursively
                result[key] = intersectMaps(val1.(map[string]any), val2.(map[string]any))
            } else {
                // Use value from first operand (deep copy)
                result[key] = deepCopyValue(val1)
            }
        }
    }

    return result
}

// Difference maps - keep keys from first map that are not in second map
func differenceMaps(map1, map2 map[string]any) map[string]any {
    result := make(map[string]any)

    // Keep keys from map1 that don't exist in map2
    for key, val := range map1 {
        if _, exists := map2[key]; !exists {
            // Key only exists in map1
            result[key] = deepCopyValue(val)
        }
    }

    return result
}

// Symmetric difference maps - keep keys that exist in exactly one map
func symmetricDifferenceMaps(map1, map2 map[string]any) map[string]any {
    result := make(map[string]any)

    // Add keys from map1 that don't exist in map2
    for key, val := range map1 {
        if _, exists := map2[key]; !exists {
            result[key] = deepCopyValue(val)
        }
    }

    // Add keys from map2 that don't exist in map1
    for key, val := range map2 {
        if _, exists := map1[key]; !exists {
            result[key] = deepCopyValue(val)
        }
    }

    return result
}

// Library function for deep merge (same as | operator)
func lib_merge(args []any) any {
    expect_args("merge", args, 1, "2", "map", "map")
    map1, ok1 := args[0].(map[string]any)
    map2, ok2 := args[1].(map[string]any)
    if !ok1 || !ok2 {
        panic("merge() requires two map arguments")
    }
    return deepMergeMaps(map1, map2)
}

// Library function for map intersection (same as & operator)
func lib_intersect(args []any) any {
    expect_args("intersect", args, 1, "2", "map", "map")
    map1, ok1 := args[0].(map[string]any)
    map2, ok2 := args[1].(map[string]any)
    if !ok1 || !ok2 {
        panic("intersect() requires two map arguments")
    }
    return intersectMaps(map1, map2)
}

// Library function for map difference (same as - operator)
func lib_difference(args []any) any {
    expect_args("difference", args, 1, "2", "map", "map")
    map1, ok1 := args[0].(map[string]any)
    map2, ok2 := args[1].(map[string]any)
    if !ok1 || !ok2 {
        panic("difference() requires two map arguments")
    }
    return differenceMaps(map1, map2)
}

// Library function for map symmetric difference (same as ^ operator)
func lib_symmetric_difference(args []any) any {
    expect_args("symmetric_difference", args, 1, "2", "map", "map")
    map1, ok1 := args[0].(map[string]any)
    map2, ok2 := args[1].(map[string]any)
    if !ok1 || !ok2 {
        panic("symmetric_difference() requires two map arguments")
    }
    return symmetricDifferenceMaps(map1, map2)
}

// Initialize map library functions
func init() {
    // Register map operation functions
    slhelp["merge"] = LibHelp{
        in:     "map1, map2",
        out:    "map",
        action: "Deep merge two maps (same as | operator)",
    }

    slhelp["intersect"] = LibHelp{
        in:     "map1, map2",
        out:    "map",
        action: "Keep only keys present in both maps (same as & operator)",
    }

    slhelp["difference"] = LibHelp{
        in:     "map1, map2",
        out:    "map",
        action: "Keep keys from first map that are not in second map (same as - operator)",
    }

    slhelp["symmetric_difference"] = LibHelp{
        in:     "map1, map2",
        out:    "map",
        action: "Keep keys that exist in exactly one map (same as ^ operator)",
    }
}
