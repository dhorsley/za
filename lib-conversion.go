//go:build !test
// +build !test

package main

import (
    "bufio"
    "bytes"
    "encoding/base64"
    "encoding/gob"
    "encoding/json"
    "errors"
    "fmt"
    "math"
    "math/big"
    "os"
    "reflect"
    "strconv"
    "sort"
    "strings"
    str "strings"
    "unsafe"

    "github.com/itchyny/gojq"
)

func kind(kind_override string, args ...any) (ret any, err error) {

    // pf("(inside kind call) with args... %#v\n",args)
    if len(args) != 1 {
        return -1, errors.New("invalid arguments provided to kind()")
    }

    if kind_override != "" {
        // pf("[k] passed an override of [%s]\n",kind_override)
        return kind_override, nil
    }

    repl := str.Replace(sf("%T", args[0]), "float64", "float", -1)
    repl = str.Replace(repl, "interface {}", "any", -1)
    return repl, nil
}

// struct to map
func s2m(val any) map[string]any {

    m := make(map[string]any)

    rs := reflect.ValueOf(val)
    rt := rs.Type()
    rs2 := reflect.New(rs.Type()).Elem()
    rs2.Set(rs)

    for i := 0; i < rs.NumField(); i++ {
        rf := rs2.Field(i)
        rf = reflect.NewAt(rf.Type(), unsafe.Pointer(rf.UnsafeAddr())).Elem()
        name := rt.Field(i).Name
        m[name] = rf.Interface()
    }

    return m
}

// map to struct: requires type information of receiver.
func m2s(m map[string]any, rcvr any) any {

    // get underlying type of rcvr
    rs := reflect.ValueOf(rcvr)
    rt := rs.Type()

    rs2 := reflect.New(rt).Elem()
    rs2.Set(rs)

    // populate rcvr through reflection
    for i := 0; i < rs.NumField(); i++ {
        rf := rs2.Field(i)
        rf = reflect.NewAt(rf.Type(), unsafe.Pointer(rf.UnsafeAddr())).Elem()
        name := rt.Field(i).Name
        switch tm := m[name].(type) {
        case bool, int, int64, uint, uint8, uint64, float64, string, any:
            rf.Set(reflect.ValueOf(tm))
        case []bool, []int, []int64, []uint, []uint8, []uint64, []float64, []string, []any:
            rf.Set(reflect.ValueOf(tm))
        default:
            pf("unknown type in m2s '%T'\n", tm)
        }
    }

    return rs2.Interface()
}

// generateTypeString converts a reflect.Type to a string that can be parsed by parseAndConstructType
func generateTypeString(t reflect.Type) string {
    switch t.Kind() {
    case reflect.Slice:
        elemTypeStr := generateTypeString(t.Elem())
        return "[]" + elemTypeStr
    case reflect.Array:
        elemTypeStr := generateTypeString(t.Elem())
        return sf("[%d]%s", t.Len(), elemTypeStr)
    case reflect.Map:
        keyTypeStr := generateTypeString(t.Key())
        valueTypeStr := generateTypeString(t.Elem())
        return sf("map[%s]%s", keyTypeStr, valueTypeStr)
    case reflect.String:
        return "string"
    case reflect.Int:
        return "int"
    case reflect.Uint:
        return "uint"
    case reflect.Float64:
        return "float64"
    case reflect.Bool:
        return "bool"
    case reflect.Interface:
        // For interface{} types, use "interface{}"
        if t.NumMethod() == 0 {
            return "interface{}"
        }
        return t.String()
    default:
        return t.String()
    }
}

// convertValue recursively converts a value to the specified type string
func convertValue(value any, targetTypeStr string) (any, error) {
    // Convert "any" alias to "interface{}" for compatibility
    targetTypeStr = str.Replace(targetTypeStr, "any", "interface{}", -1)

    // Use parseAndConstructType to get the target type
    targetType := parseAndConstructType(targetTypeStr)
    if targetType == nil {
        return nil, errors.New(sf("convertValue: invalid type string '%s'", targetTypeStr))
    }

    // If value is nil, create zero value of target type
    if value == nil {
        return reflect.Zero(targetType).Interface(), nil
    }

    sourceType := reflect.TypeOf(value)

    // If types are already the same, return as-is
    if sourceType == targetType {
        return value, nil
    }

    // Direct assignment check
    if sourceType.AssignableTo(targetType) {
        return value, nil
    }

    // Try conversion for slices
    if sourceType.Kind() == reflect.Slice && targetType.Kind() == reflect.Slice {
        sourceSlice := reflect.ValueOf(value)
        targetElemType := targetType.Elem()

        // Handle empty slice
        if sourceSlice.Len() == 0 {
            newSlice := reflect.MakeSlice(targetType, 0, 0)
            return newSlice.Interface(), nil
        }

        // Check if all elements can be converted
        newSlice := reflect.MakeSlice(targetType, sourceSlice.Len(), sourceSlice.Len())
        for i := 0; i < sourceSlice.Len(); i++ {
            elem := sourceSlice.Index(i)

            // If the element is an interface{}, get the concrete value
            if elem.Kind() == reflect.Interface {
                elem = elem.Elem()
            }

            elemValue := elem.Interface()

            if elem.Type().AssignableTo(targetElemType) {
                // Use the unwrapped value, not the reflect.Value
                valueToSet := reflect.ValueOf(elemValue)
                newSlice.Index(i).Set(valueToSet)
            } else if elem.Type().ConvertibleTo(targetElemType) {
                newSlice.Index(i).Set(elem.Convert(targetElemType))
            } else {
                // Try recursive conversion for nested slices
                targetElemTypeStr := generateTypeString(targetElemType)
                convertedElem, err := convertValue(elemValue, targetElemTypeStr)
                if err != nil {
                    return nil, errors.New(sf("to_typed: cannot convert element %d of type %T to %v: %v", i, elemValue, targetElemType, err))
                }
                newSlice.Index(i).Set(reflect.ValueOf(convertedElem))
            }
        }

        return newSlice.Interface(), nil
    }

    // Try direct conversion
    sourceValue := reflect.ValueOf(value)
    if sourceType.ConvertibleTo(targetType) {
        return sourceValue.Convert(targetType).Interface(), nil
    }

    return nil, errors.New(sf("convertValue: cannot convert value of type %T to type %s", value, targetTypeStr))
}

// Helper function for pretty printing
func pp(input any, maxDepth int, indent string) (string, error) {
    // Define color codes using ZA's sparkle system
    colours := map[string]string{
        "key":         "[#5]", // map keys
        "string":      "[#4]", // string values
        "number":      "[#6]", // numeric values
        "boolean":     "[#3]", // boolean values
        "null":        "[#2]", // null values and errors
        "map_start":   "[#1]", // map braces
        "slice_start": "[#1]", // slice brackets
        "reset":       "[#-]",
    }

    // Use reflection to handle all types dynamically
    val := reflect.ValueOf(input)
    if !val.IsValid() {
        return sparkle(colours["null"] + "null" + colours["reset"]), nil
    }

    result := prettyPrintValue(val, "", 0, maxDepth, indent, colours)
    return result, nil
}

// Recursive pretty printer that works with any reflected type
func prettyPrintValue(val reflect.Value, currentIndent string, depth int, maxDepth int, indent string, colours map[string]string) string {

    if depth > maxDepth {
        return colours["null"] + "... (max depth reached)" + colours["reset"]
    }
    // Handle nil values
    if !val.IsValid() {
        return colours["null"] + "null" + colours["reset"]
    }
    if val.Kind() == reflect.Ptr || val.Kind() == reflect.Interface ||
        val.Kind() == reflect.Chan || val.Kind() == reflect.Func ||
        val.Kind() == reflect.Map || val.Kind() == reflect.Slice {
        if val.IsNil() {
            return colours["null"] + "null" + colours["reset"]
        }
    }

    // Handle primitive types and special cases
    var interfaceValue any
    if val.CanInterface() {
        interfaceValue = val.Interface()
    } else {
        // Handle unexported fields using unsafe
        ptr := unsafe.Pointer(val.UnsafeAddr())
        rv := reflect.NewAt(val.Type(), ptr).Elem()
        interfaceValue = rv.Interface()
    }

    switch v := interfaceValue.(type) {
    case string:
        return colours["string"] + "\"" + v + "\"" + colours["reset"]
    case bool:
        return colours["boolean"] + fmt.Sprintf("%v", v) + colours["reset"]
    case int, int8, int16, int32, int64:
        return colours["number"] + fmt.Sprintf("%v", v) + colours["reset"]
    case uint, uint8, uint16, uint32, uint64:
        return colours["number"] + fmt.Sprintf("%v", v) + colours["reset"]
    case float32, float64:
        return colours["number"] + fmt.Sprintf("%v", v) + colours["reset"]
    case *big.Int:
        return colours["number"] + v.String() + colours["reset"]
    case *big.Float:
        return colours["number"] + v.String() + colours["reset"]
    }

    switch val.Kind() {
    case reflect.Map:
        var result strings.Builder
        result.WriteString(colours["map_start"] + "{" + colours["reset"] + "\n")

        keys := val.MapKeys()
        if len(keys) == 0 {
            result.WriteString(currentIndent + colours["map_start"] + "}" + colours["reset"])
            return result.String()
        }

        // For stable output, sort keys by their string representation, but print the original key value
        type keyPair struct {
            key reflect.Value
            str string
        }
        var keyPairs []keyPair
        for _, key := range keys {
            // Use fmt.Sprintf for stable string sort, but keep original key
            var keyValue any
            if key.CanInterface() {
                keyValue = key.Interface()
            }
            keyPairs = append(keyPairs, keyPair{key, fmt.Sprintf("%v", keyValue)})
        }

        sort.Slice(keyPairs, func(i, j int) bool {
            return keyPairs[i].str < keyPairs[j].str
        })

        for i, kp := range keyPairs {
            key := kp.key
            result.WriteString(currentIndent + indent + colours["key"])
            if key.Kind() == reflect.String {
                result.WriteString("\"" + key.String() + "\"")
            } else {
                result.WriteString(fmt.Sprintf("%v", key.Interface()))
            }
            result.WriteString(colours["reset"] + ": ")

            result.WriteString(prettyPrintValue(val.MapIndex(key), currentIndent+indent, depth+1, maxDepth, indent, colours))

            if i < len(keyPairs)-1 {
                result.WriteString(",")
            }
            result.WriteString("\n")
        }
        result.WriteString(currentIndent + colours["map_start"] + "}" + colours["reset"])
        return result.String()

    case reflect.Slice, reflect.Array:
        var result strings.Builder
        result.WriteString(colours["slice_start"] + "[" + colours["reset"] + "\n")
        for i := 0; i < val.Len(); i++ {
            result.WriteString(currentIndent + indent)
            result.WriteString(prettyPrintValue(val.Index(i), currentIndent+indent, depth+1, maxDepth, indent, colours))
            if i < val.Len()-1 {
                result.WriteString(",")
            }
            result.WriteString("\n")
        }
        result.WriteString(currentIndent + colours["slice_start"] + "]" + colours["reset"])
        return result.String()

    case reflect.Interface:
        if val.IsNil() {
            return colours["null"] + "null" + colours["reset"]
        }
        var result strings.Builder
        result.WriteString(prettyPrintValue(val.Elem(), currentIndent, depth+1, maxDepth, indent, colours))
        return result.String()

    case reflect.Ptr:
        if val.IsNil() {
            return colours["null"] + "null" + colours["reset"]
        }
        return prettyPrintValue(val.Elem(), currentIndent, depth+1, maxDepth, indent, colours)
    case reflect.Struct:
        var result strings.Builder
        result.WriteString(colours["map_start"] + "{" + colours["reset"] + "\n")
        typ := val.Type()
        for i := 0; i < val.NumField(); i++ {
            field := val.Field(i)
            fieldName := typ.Field(i).Name
            result.WriteString(currentIndent + indent + colours["key"] + "\"" + fieldName + "\"" + colours["reset"] + ": ")

            // Handle unexported fields using the same approach as the codebase
            var fieldValue any
            if field.CanInterface() {
                fieldValue = field.Interface()
            } else {
                // Use unsafe to access unexported fields
                ptr := unsafe.Pointer(field.UnsafeAddr())
                rv := reflect.NewAt(field.Type(), ptr).Elem()
                fieldValue = rv.Interface()
            }

            // Create a new reflect.Value for the field value
            fieldVal := reflect.ValueOf(fieldValue)
            result.WriteString(prettyPrintValue(fieldVal, currentIndent+indent, depth+1, maxDepth, indent, colours))
            if i < val.NumField()-1 {
                result.WriteString(",")
            }
            result.WriteString("\n")
        }
        result.WriteString(currentIndent + colours["map_start"] + "}" + colours["reset"])
        return result.String()
    default:
        var interfaceValue any
        if val.CanInterface() {
            interfaceValue = val.Interface()
        } else {
            // Handle unexported fields using unsafe
            ptr := unsafe.Pointer(val.UnsafeAddr())
            rv := reflect.NewAt(val.Type(), ptr).Elem()
            interfaceValue = rv.Interface()
        }
        return colours["string"] + fmt.Sprintf("%v", interfaceValue) + colours["reset"]
    }
}

// describeType provides a plain English description of a Go type
func describeType(t reflect.Type) string {
    switch t.Kind() {
    case reflect.String:
        return "text string"
    case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
        return "integer number"
    case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
        return "positive integer number"
    case reflect.Float32, reflect.Float64:
        return "decimal number"
    case reflect.Bool:
        return "true/false value"
    case reflect.Slice:
        elemDesc := describeType(t.Elem())
        return sf("list of %s", elemDesc)
    case reflect.Map:
        keyDesc := describeType(t.Key())
        valueDesc := describeType(t.Elem())
        return sf("dictionary mapping %s to %s", keyDesc, valueDesc)
    case reflect.Struct:
        return sf("struct with %d fields", t.NumField())
    case reflect.Ptr:
        return sf("pointer to %s", describeType(t.Elem()))
    case reflect.Interface:
        return "any type of value"
    case reflect.Array:
        elemDesc := describeType(t.Elem())
        return sf("fixed-size array of %d %s", t.Len(), elemDesc)
    case reflect.Chan:
        return sf("channel of %s", describeType(t.Elem()))
    case reflect.Func:
        return "function"
    default:
        return t.String()
    }
}

func buildConversionLib() {

    // conversion

    features["conversion"] = Feature{version: 1, category: "os"}
    categories["conversion"] = []string{
        "byte", "as_int", "as_int64", "as_bigi", "as_bigf", "as_float", "as_bool", "as_string", "maxuint", "char", "asc", "as_uint",
        "is_number", "base64e", "base64d", "json_decode", "json_format", "json_query", "pp",
        "write_struct", "read_struct",
        "btoi", "itob", "dtoo", "otod", "s2m", "m2s", "f2n", "to_typed",
    }

    slhelp["f2n"] = LibHelp{in: "any", out: "nil_or_any", action: "Converts false to nil or returns true."}
    stdlib["f2n"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("f2n", args, 1, "1", "bool"); !ok {
            return nil, err
        }
        if args[0].(bool) == false {
            return nil, nil
        }
        return args[0], nil
    }

    slhelp["s2m"] = LibHelp{in: "struct", out: "map", action: "Convert a struct to map."}
    stdlib["s2m"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("s2m", args, 1, "1", "any"); !ok {
            return nil, err
        }
        if reflect.TypeOf(args[0]).Kind() != reflect.Struct {
            return nil, errors.New("s2m: expected struct argument")
        }
        return s2m(args[0]), nil
    }

    slhelp["m2s"] = LibHelp{in: "map,struct_example", out: "struct", action: "Convert a map to struct following field form of [#i1]struct_example[#i0]."}
    stdlib["m2s"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("m2s", args, 1, "2", "map[string]interface {}", "any"); !ok {
            return nil, err
        }
        if reflect.TypeOf(args[1]).Kind() != reflect.Struct {
            return nil, errors.New("m2s: expected second argument to be struct")
        }
        m := m2s(args[0].(map[string]any), args[1])
        return m, nil
    }

    /*
    slhelp["explain"] = LibHelp{in: "struct", out: "string", action: "Returns a plain English description of a data structure's layout and types."}
    stdlib["explain"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("explain", args, 1, "1", "any"); !ok {
            return nil, err
        }

        obj := args[0]
        if reflect.TypeOf(obj).Kind() != reflect.Struct {
            return nil, errors.New("explain: expected struct argument")
        }

        val := reflect.ValueOf(obj)
        typ := val.Type()

        var result strings.Builder

        // Get struct name
        structName := "Unknown"
        if name, count := struct_match(obj); count == 1 {
            structName = name
        } else {
            structName = typ.String()
        }

        result.WriteString(sf("Struct '%s' contains %d fields:\n\n", structName, val.NumField()))

        // Describe each field
        for i := 0; i < val.NumField(); i++ {
            field := val.Field(i)
            fieldType := typ.Field(i)

            // Field name
            result.WriteString(sf("  %d. %s: ", i+1, fieldType.Name))

            // Field type description
            typeDesc := describeType(field.Type())
            result.WriteString(typeDesc)

            // Current value (if simple)
            if field.Kind() == reflect.String || field.Kind() == reflect.Int || field.Kind() == reflect.Float64 || field.Kind() == reflect.Bool {
                var fieldValue any
                if field.CanInterface() {
                    fieldValue = field.Interface()
                } else {
                    ptr := unsafe.Pointer(field.UnsafeAddr())
                    rv := reflect.NewAt(field.Type(), ptr).Elem()
                    fieldValue = rv.Interface()
                }
                result.WriteString(sf(" (current value: %v)", fieldValue))
            } else if field.Kind() == reflect.Slice {
                result.WriteString(sf(" (length: %d)", field.Len()))
            }

            result.WriteString("\n")
        }

        return result.String(), nil
    }
    */

    slhelp["write_struct"] = LibHelp{in: "filename,name_of_struct", out: "size", action: "Sends a struct to file. Returns byte size written."}
    stdlib["write_struct"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("write_struct", args, 1, "2", "string", "string"); !ok {
            return nil, err
        }

        fn := args[0].(string)
        vn := args[1].(string)

        // convert struct to map
        v, _ := vget(nil, evalfs, ident, vn)
        m := s2m(v)

        // encode with gob
        b := new(bytes.Buffer)
        e := gob.NewEncoder(b)
        err = e.Encode(m)
        if err != nil {
            return false, err
        }

        // start writer
        f, err := os.Create(fn)
        w := bufio.NewWriter(f)
        w.Write(b.Bytes())
        w.Flush()
        f.Close()

        return true, nil

    }

    slhelp["read_struct"] = LibHelp{in: "filename,name_of_destination_struct", out: "bool_success", action: "Read a struct from a file."}
    stdlib["read_struct"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("read_struct", args, 1, "2", "string", "string"); !ok {
            return nil, err
        }

        fn := args[0].(string)
        vn := args[1].(string)

        v, success := vget(nil, evalfs, ident, vn)
        if !success {
            return false, errors.New(sf("could not find '%v'", vn))
        }

        r := reflect.ValueOf(v)

        // confirm this is a struct
        if reflect.ValueOf(r).Kind().String() != "struct" {
            return false, errors.New(sf("'%v' is not a STRUCT", vn))
        }

        // retrieve the packed file
        f, err := os.Open(fn)
        if err != nil {
            return nil, err
        }

        // unpack
        var m = new(map[string]any)
        d := gob.NewDecoder(f)
        err = d.Decode(&m)
        f.Close()

        if err != nil {
            return false, errors.New("unpacking error")
        }

        // write to Za variable.
        bin := bind_int(evalfs, vn)
        (*ident)[bin] = Variable{IName: vn, IValue: m2s(*m, v), IKind: 0, ITyped: false, declared: true}

        return true, nil

    }

    slhelp["char"] = LibHelp{in: "int", out: "string", action: "Return a string representation of ASCII char [#i1]int[#i0]. Representations above 127 are empty."}
    stdlib["char"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("char", args, 1, "1", "int"); !ok {
            return nil, err
        }

        if args[0].(int) < 0 || args[0].(int) > 127 {
            return "", nil
        }
        return sf("%c", args[0].(int)), nil
    }

    slhelp["asc"] = LibHelp{in: "string", out: "int", action: "Return a numeric representation of the first char in [#i1]string[#i0]."}
    stdlib["asc"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("asc", args, 1, "1", "string"); !ok {
            return nil, err
        }
        return int([]rune(args[0].(string))[0]), nil
    }

    slhelp["itob"] = LibHelp{in: "int", out: "bool", action: "Return a boolean which is set to true when [#i1]int[#i0] is non-zero."}
    stdlib["itob"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("itob", args, 1, "1", "int"); !ok {
            return nil, err
        }
        return args[0].(int) != 0, nil
    }

    slhelp["btoi"] = LibHelp{in: "bool", out: "int", action: "Return an int which is either 1 when [#i1]bool[#i0] is true or else 0 when [#i1]bool[#i0] is false."}
    stdlib["btoi"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("btoi", args, 1, "1", "bool"); !ok {
            return nil, err
        }
        switch args[0].(bool) {
        case true:
            return 1, nil
        }
        return 0, nil
    }

    slhelp["dtoo"] = LibHelp{in: "int", out: "string", action: "Convert decimal int to octal string."}
    stdlib["dtoo"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("dtoo", args, 1, "1", "int"); !ok {
            return nil, err
        }
        return strconv.FormatInt(int64(args[0].(int)), 8), nil
    }

    slhelp["otod"] = LibHelp{in: "string", out: "int", action: "Convert octal string to decimal int."}
    stdlib["otod"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("otod", args, 1, "1", "string"); !ok {
            return nil, err
        }
        return strconv.ParseInt(args[0].(string), 8, 64)
    }

    /*
       // kind stub
       slhelp["kind"] = LibHelp{in: "var", out: "string", action: "Return a string indicating the type of the variable [#i1]var[#i0]."}
       stdlib["kind"] = func(ns string,evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
           return ret,err
       }
    */

    slhelp["kind"] = LibHelp{in: "var", out: "string", action: "Return a string indicating the type of the variable [#i1]var[#i0]."}
    stdlib["kind"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        // pf("k-argtype:[#2]%T[#-]\n",args[0])
        if ok, err := expect_args("kind", args, 1, "1", "any"); !ok {
            return nil, err
        }
        if len(args) != 1 {
            return -1, errors.New("invalid arguments provided to kind()")
        }

        repl := str.Replace(sf("%T", args[0]), "float64", "float", -1)
        repl = str.Replace(repl, "interface {}", "any", -1)
        return repl, nil
    }

    slhelp["base64e"] = LibHelp{in: "string", out: "string", action: "Return a string of the base64 encoding of [#i1]string[#i0]"}
    stdlib["base64e"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("base64e", args, 1, "1", "string"); !ok {
            return nil, err
        }
        enc := base64.StdEncoding.EncodeToString([]byte(args[0].(string)))
        return enc, nil
    }

    slhelp["base64d"] = LibHelp{in: "string", out: "string", action: "Return a string of the base64 decoding of [#i1]string[#i0]"}
    stdlib["base64d"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("base64d", args, 1, "1", "string"); !ok {
            return nil, err
        }
        dec, e := base64.StdEncoding.DecodeString(args[0].(string))
        if e != nil {
            return "", errors.New(sf("could not convert '%s' in base64d()", args[0].(string)))
        }
        return string(dec), nil
    }

    slhelp["json_decode"] = LibHelp{in: "string", out: "[]any", action: "Return a mixed type array representing a JSON string."}
    stdlib["json_decode"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("json_decode", args, 1, "1", "string"); !ok {
            return nil, err
        }

        var v map[string]any
        dec := json.NewDecoder(str.NewReader(args[0].(string)))

        if err := dec.Decode(&v); err != nil {
            return "", errors.New(sf("could not convert value '%v' in json_decode()", args[0].(string)))
        }

        return v, nil

    }

    slhelp["json_format"] = LibHelp{in: "string", out: "string", action: "Return a formatted JSON representation of [#i1]string[#i0], or an empty string on error."}
    stdlib["json_format"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("json_format", args, 1, "1", "string"); !ok {
            return nil, err
        }
        var pj bytes.Buffer
        if err := json.Indent(&pj, []byte(args[0].(string)), "", "\t"); err != nil {
            return "", errors.New(sf("could not format string in json_format()"))
        }
        return string(pj.Bytes()), nil
    }

    slhelp["json_query"] = LibHelp{in: "input_string,query_string[,map_bool]", out: "string",
        action: "Returns the result of processing [#i1]input_string[#i0] using the gojq library.\n" +
            "[#i1]query_string[#i0] is a jq-like query to operate with. If [#i1]map_bool[#i0] is false (default)\n" +
            "then a string is returned, otherwise an iterable list is returned."}
    stdlib["json_query"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("json_query", args, 2,
            "2", "string", "string",
            "3", "string", "string", "bool"); !ok {
            return nil, err
        }

        var complex bool
        if len(args) == 3 {
            switch args[2].(type) {
            case bool:
                complex = args[2].(bool)
            default:
                return nil, errors.New("argument 3 must be a boolean when present in json_query()")
            }
        }

        // first parse query string
        q, e := gojq.Parse(args[1].(string))
        if e != nil {
            return "", errors.New("invalid query string in json_query()")
        }

        // then decode json to map suitable for gojq.Run
        var iv map[string]any
        dec := json.NewDecoder(str.NewReader(args[0].(string)))
        if err := dec.Decode(&iv); err != nil {
            return "", errors.New("could not convert JSON in json_query()")
        }

        // process query
        var newstring str.Builder
        var retlist []any

        iter := q.Run(iv)

        for {
            v, ok := iter.Next()
            if !ok {
                break
            }
            if complex {
                retlist = append(retlist, v)
            } else {
                newstring.WriteString(sf("%v\n", v))
            }
        }

        if complex {
            return retlist, nil
        }
        return newstring.String(), nil

    }

    slhelp["pp"] = LibHelp{in: "map|slice, [max_depth], [indent_string]", out: "string", action: "Pretty print a map or slice with optional indentation, depth limit, and color-coded section headings."}
    stdlib["pp"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("pp", args, 3,
            "1", "any",
            "2", "any", "int",
            "3", "any", "int", "string"); !ok {
            return nil, err
        }

        input := args[0]
        maxDepth := 50
        indent := "  "

        if len(args) > 1 {
            maxDepth = args[1].(int)
        }
        if len(args) > 2 {
            indent = args[2].(string)
        }

        return pp(input, maxDepth, indent)
    }

    slhelp["as_bigi"] = LibHelp{in: "expr", out: "big_int", action: "Convert [#i1]expr[#i0] to a big integer. Also ensures this is a copy."}
    stdlib["as_bigi"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if len(args) != 1 {
            return -1, errors.New("invalid arguments provided to as_bigi()")
        }
        return GetAsBigInt(args[0]), nil
    }

    slhelp["as_bigf"] = LibHelp{in: "expr", out: "big_float", action: "Convert [#i1]expr[#i0] to a float. Also ensures this is a copy."}
    stdlib["as_bigf"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if len(args) != 1 {
            return -1, errors.New("invalid arguments provided to as_bigf()")
        }
        return GetAsBigFloat(args[0]), nil
    }

    slhelp["as_float"] = LibHelp{in: "var", out: "float", action: "Convert [#i1]var[#i0] to a float. Returns NaN on error."}
    stdlib["as_float"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if len(args) != 1 {
            return -1, errors.New("invalid arguments provided to as_float()")
        }
        i, e := GetAsFloat(args[0])
        if e {
            return math.NaN(), nil
        }
        return i, nil
    }

    slhelp["byte"] = LibHelp{in: "var", out: "byte", action: "Convert to a uint8 sized integer, or errors."}
    stdlib["byte"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if len(args) != 1 {
            return -1, errors.New("invalid arguments provided to byte()")
        }
        i, invalid := GetAsInt(args[0])
        if !invalid {
            return byte(i), nil
        }
        return byte(0), err
    }

    slhelp["as_bool"] = LibHelp{in: "string", out: "bool", action: "Convert [#i1]string[#i0] to a boolean value, or errors"}
    stdlib["as_bool"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if len(args) != 1 {
            return -1, errors.New("invalid arguments provided to as_bool()")
        }
        switch args[0].(type) {
        case bool:
            return args[0].(bool), nil
        case uint:
            return args[0].(uint) != 0, nil
        case int:
            return args[0].(int) != 0, nil
        case string:
            if args[0] == "" {
                args[0] = "false"
            }
            b, err := strconv.ParseBool(args[0].(string))
            if err == nil {
                return b, nil
            }
        }
        return false, errors.New(sf("could not convert [%T] (%v) to bool in as_bool()", args[0], args[0]))
    }

    slhelp["as_int"] = LibHelp{in: "var", out: "integer", action: "Convert [#i1]var[#i0] to an integer, or errors."}
    stdlib["as_int"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if len(args) != 1 {
            return -1, errors.New("invalid arguments provided to as_int()")
        }
        i, invalid := GetAsInt(args[0])
        if !invalid {
            return i, nil
        }
        return 0, errors.New(sf("could not convert [%T] (%v) to integer in as_int()", args[0], args[0]))
    }

    slhelp["as_uint"] = LibHelp{in: "var", out: "unsigned_integer", action: "Convert [#i1]var[#i0] to a uint type, or errors."}
    stdlib["as_uint"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if len(args) != 1 {
            return -1, errors.New("invalid arguments provided to as_uint()")
        }
        i, invalid := GetAsUint64(args[0])
        if !invalid {
            return i, nil
        }
        return uint(0), errors.New(sf("could not convert [%T] (%v) to integer in as_uint()", args[0], args[0]))
    }

    slhelp["maxuint"] = LibHelp{in: "var", out: "uint64", action: "Represents the maximum possible uint value."}
    stdlib["maxuint"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if len(args) != 0 {
            return nil, errors.New("invalid arguments provided to maxuint()")
        }
        return uint64(math.MaxUint), nil
    }

    slhelp["as_int64"] = LibHelp{in: "var", out: "integer", action: "Convert [#i1]var[#i0] to an int64 type, or errors."}
    stdlib["as_int64"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if len(args) != 1 {
            return -1, errors.New("invalid arguments provided to as_int64()")
        }
        i, invalid := GetAsInt(args[0])
        if !invalid {
            return int64(i), nil
        }
        return int64(0), errors.New(sf("could not convert [%T] (%v) to integer in as_int64()", args[0], args[0]))
    }

    slhelp["as_string"] = LibHelp{in: "value[,precision]", out: "string", action: "Converts [#i1]value[#i0] to a string."}
    stdlib["as_string"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("as_string", args, 2,
            "2", "any", "int",
            "1", "any"); !ok {
            return nil, err
        }
        var i string
        if len(args) == 2 {
            switch args[0].(type) {
            case *big.Float:
                f := args[0].(*big.Float)
                i = f.Text('g', args[1].(int))
            default:
                return "", errors.New(sf("as_string() was expecting a bigf type, but got a [%T]", args[0]))
            }
        } else {
            switch args[0].(type) {
            case *big.Int:
                n := args[0].(*big.Int)
                i = n.String()
            case *big.Float:
                f := args[0].(*big.Float)
                i = f.String()
            default:
                i = sf("%v", args[0])
            }
        }
        return i, nil
    }

    slhelp["is_number"] = LibHelp{in: "expression", out: "bool", action: "Returns true if [#i1]expression[#i0] can evaluate to a numeric value."}
    stdlib["is_number"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if len(args) != 1 {
            return -1, errors.New("invalid arguments provided to is_number()")
        }
        switch args[0].(type) {
        case uint, uint8, uint64, int, int64, float64:
            return isNumber(args[0]), nil
        case string:
            if len(args[0].(string)) == 0 {
                return false, nil
            }
            _, invalid := GetAsFloat(args[0])
            if invalid {
                return false, nil
            } else {
                _, invalid := GetAsInt(args[0])
                if invalid {
                    return false, nil
                }
            }
            return true, nil
        default:
            return false, nil
        }
    }

    slhelp["to_typed"] = LibHelp{in: "value,type_string", out: "typed_value", action: "Convert [#i1]value[#i0] to the specified type [#i1]type_string[#i0]. Supports multi-dimensional arrays like '[][]int', '[][][]string', '[5][3]int', etc."}
    stdlib["to_typed"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("to_typed", args, 1, "2", "any", "string"); !ok {
            return nil, err
        }

        value := args[0]
        typeString := args[1].(string)

        // Convert "any" alias to "interface{}" for compatibility
        typeString = str.Replace(typeString, "any", "interface{}", -1)

        // Use parseAndConstructType to get the target type
        targetType := parseAndConstructType(typeString)
        if targetType == nil {
            return nil, errors.New(sf("to_typed: invalid type string '%s'", typeString))
        }

        // If value is nil, create zero value of target type
        if value == nil {
            return reflect.Zero(targetType).Interface(), nil
        }

        sourceType := reflect.TypeOf(value)

        // If types are already the same, return as-is
        if sourceType == targetType {
            return value, nil
        }

        // Direct assignment check
        if sourceType.AssignableTo(targetType) {
            return value, nil
        }

        // Try conversion for slices using convertValue helper
        if sourceType.Kind() == reflect.Slice && targetType.Kind() == reflect.Slice {
            return convertValue(value, typeString)
        }

        // Try direct conversion
        sourceValue := reflect.ValueOf(value)
        if sourceType.ConvertibleTo(targetType) {
            return sourceValue.Convert(targetType).Interface(), nil
        }

        return nil, errors.New(sf("to_typed: cannot convert value of type %T to type %s", value, typeString))
    }

}
