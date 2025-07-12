//go:build !test
// +build !test

package main

import (
    "fmt"
    "strconv"
    "strings"

    "gopkg.in/yaml.v3"
)

func buildYamlLib() {
    features["yaml"] = Feature{version: 1, category: "data"}
    categories["yaml"] = []string{"yaml_parse", "yaml_marshal", "yaml_get", "yaml_set", "yaml_delete"}

    slhelp["yaml_parse"] = LibHelp{in: "yaml_string", out: "any", action: "Parse YAML string to Za data structures."}
    stdlib["yaml_parse"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("yaml_parse", args, 1, "1", "string"); !ok {
            return nil, err
        }
        yamlString := args[0].(string)
        result, err := parseYAML(yamlString)
        if err != nil {
            return nil, fmt.Errorf("yaml_parse error: %v", err)
        }
        return result, nil
    }

    slhelp["yaml_marshal"] = LibHelp{in: "data", out: "string", action: "Convert Za data to YAML string."}
    stdlib["yaml_marshal"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("yaml_marshal", args, 1, "1", "any"); !ok {
            return nil, err
        }
        data := args[0]
        yamlString, err := marshalYAML(data)
        if err != nil {
            return nil, fmt.Errorf("yaml_marshal error: %v", err)
        }
        return yamlString, nil
    }

    slhelp["yaml_get"] = LibHelp{in: "data, path", out: "any", action: "Get value from YAML data using dot notation path (e.g., 'spec.containers[0].image')."}
    stdlib["yaml_get"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("yaml_get", args, 2, "2", "any", "string"); !ok {
            return nil, err
        }
        data := args[0]
        path := args[1].(string)
        result, err := yamlGet(data, path)
        if err != nil {
            return nil, fmt.Errorf("yaml_get error: %v", err)
        }
        return result, nil
    }

    slhelp["yaml_set"] = LibHelp{in: "data, path, value", out: "any", action: "Set value in YAML data using dot notation path. Returns modified data."}
    stdlib["yaml_set"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("yaml_set", args, 3, "3", "any", "string", "any"); !ok {
            return nil, err
        }
        data := args[0]
        path := args[1].(string)
        value := args[2]
        result, err := yamlSet(data, path, value)
        if err != nil {
            return nil, fmt.Errorf("yaml_set error: %v", err)
        }
        return result, nil
    }

    slhelp["yaml_delete"] = LibHelp{in: "data, path", out: "any", action: "Delete value from YAML data using dot notation path. Returns modified data."}
    stdlib["yaml_delete"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("yaml_delete", args, 2, "2", "any", "string"); !ok {
            return nil, err
        }
        data := args[0]
        path := args[1].(string)
        result, err := yamlDelete(data, path)
        if err != nil {
            return nil, fmt.Errorf("yaml_delete error: %v", err)
        }
        return result, nil
    }
}

func parseYAML(input string) (any, error) {
    var result any
    err := yaml.Unmarshal([]byte(input), &result)
    if err != nil {
        return nil, err
    }
    return result, nil
}

func marshalYAML(data any) (string, error) {
    yamlBytes, err := yaml.Marshal(data)
    if err != nil {
        return "", err
    }
    return string(yamlBytes), nil
}

// yamlGet retrieves a value from YAML data using dot notation path
func yamlGet(data any, path string) (any, error) {
    parts := parsePath(path)
    current := data

    for _, part := range parts {
        switch v := current.(type) {
        case map[string]any:
            if value, exists := v[part.key]; exists {
                current = value
            } else {
                return nil, fmt.Errorf("key '%s' not found", part.key)
            }
        case []any:
            if part.index >= 0 && part.index < len(v) {
                current = v[part.index]
            } else {
                return nil, fmt.Errorf("index %d out of bounds (length: %d)", part.index, len(v))
            }
        default:
            return nil, fmt.Errorf("cannot access '%s' in type %T", part.key, current)
        }
    }

    return current, nil
}

// yamlSet sets a value in YAML data using dot notation path
func yamlSet(data any, path string, value any) (any, error) {
    parts := parsePath(path)

    // Handle root level assignment
    if len(parts) == 1 && parts[0].index == -1 {
        if mapData, ok := data.(map[string]any); ok {
            mapData[parts[0].key] = value
            return data, nil
        }
        return nil, fmt.Errorf("root data is not a map")
    }

    // Navigate to parent and set value
    parent, err := yamlGet(data, strings.Join(pathPartsToStrings(parts[:len(parts)-1]), "."))
    if err != nil {
        return nil, err
    }

    lastPart := parts[len(parts)-1]
    switch v := parent.(type) {
    case map[string]any:
        v[lastPart.key] = value
    case []any:
        if lastPart.index >= 0 && lastPart.index < len(v) {
            v[lastPart.index] = value
        } else {
            return nil, fmt.Errorf("index %d out of bounds (length: %d)", lastPart.index, len(v))
        }
    default:
        return nil, fmt.Errorf("cannot set value in type %T", parent)
    }

    return data, nil
}

// yamlDelete removes a value from YAML data using dot notation path
func yamlDelete(data any, path string) (any, error) {
    parts := parsePath(path)

    // Handle root level deletion
    if len(parts) == 1 && parts[0].index == -1 {
        if mapData, ok := data.(map[string]any); ok {
            delete(mapData, parts[0].key)
            return data, nil
        }
        return nil, fmt.Errorf("root data is not a map")
    }

    // Navigate to parent and delete value
    parent, err := yamlGet(data, strings.Join(pathPartsToStrings(parts[:len(parts)-1]), "."))
    if err != nil {
        return nil, err
    }

    lastPart := parts[len(parts)-1]
    switch v := parent.(type) {
    case map[string]any:
        delete(v, lastPart.key)
    case []any:
        if lastPart.index >= 0 && lastPart.index < len(v) {
            // Remove element from slice
            v = append(v[:lastPart.index], v[lastPart.index+1:]...)
            // Update the parent slice
            parentPath := strings.Join(pathPartsToStrings(parts[:len(parts)-1]), ".")
            yamlSet(data, parentPath, v)
        } else {
            return nil, fmt.Errorf("index %d out of bounds (length: %d)", lastPart.index, len(v))
        }
    default:
        return nil, fmt.Errorf("cannot delete from type %T", parent)
    }

    return data, nil
}

// pathPart represents a single part of a dot notation path
type pathPart struct {
    key   string
    index int // -1 for map keys, >= 0 for array indices
}

// parsePath parses a dot notation path into pathPart slices
func parsePath(path string) []pathPart {
    if path == "" {
        return []pathPart{}
    }

    var parts []pathPart
    segments := strings.Split(path, ".")

    for _, segment := range segments {
        // Check if this segment contains an array index
        if strings.Contains(segment, "[") && strings.Contains(segment, "]") {
            // Extract key and index
            openBracket := strings.Index(segment, "[")
            closeBracket := strings.Index(segment, "]")

            if openBracket > 0 && closeBracket > openBracket {
                key := segment[:openBracket]
                indexStr := segment[openBracket+1 : closeBracket]

                if index, err := strconv.Atoi(indexStr); err == nil {
                    parts = append(parts, pathPart{key: key, index: index})
                } else {
                    parts = append(parts, pathPart{key: segment, index: -1})
                }
            } else {
                parts = append(parts, pathPart{key: segment, index: -1})
            }
        } else {
            parts = append(parts, pathPart{key: segment, index: -1})
        }
    }

    return parts
}

// pathPartsToStrings converts pathPart slices back to string representation
func pathPartsToStrings(parts []pathPart) []string {
    var strings []string
    for _, part := range parts {
        if part.index >= 0 {
            strings = append(strings, fmt.Sprintf("%s[%d]", part.key, part.index))
        } else {
            strings = append(strings, part.key)
        }
    }
    return strings
}
