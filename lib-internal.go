//go:build !test

package main

import (
    "context"
    "encoding/binary"
    "errors"
    "fmt"
    "io/ioutil"
    "math/big"
    "net/http" // for key()
    "os"
    "reflect"
    "regexp"
    "runtime"
    "sort"
    "strings"
    str "strings"
    "sync/atomic"
)

var execMode bool // used by report() for errors
var execFs uint32 // used by report() for errors

const (
    _AT_NULL             = 0
    _AT_CLKTCK           = 17
    _SYSTEM_CLK_TCK      = 100
    uintSize        uint = 32 << (^uint(0) >> 63)
)

func ulen(args any) (int, error) {
    switch args := args.(type) { // i'm getting fed up of typing these case statements!!
    case nil:
        return 0, nil
    case string:
        return len(args), nil
        // return utf8.RuneCountInString(args), nil
    case []string:
        return len(args), nil
    case []int:
        return len(args), nil
    case []int64:
        return len(args), nil
    case []*big.Int:
        return len(args), nil
    case []*big.Float:
        return len(args), nil
    case []uint8:
        return len(args), nil
    case []float64:
        return len(args), nil
    case []bool:
        return len(args), nil
    case []dirent:
        return len(args), nil
    case map[string]float64:
        return len(args), nil
    case map[string]string:
        return len(args), nil
    case map[string][]string:
        return len(args), nil
    case map[string]int:
        return len(args), nil
    case map[string]bool:
        return len(args), nil
    case map[string]int64:
        return len(args), nil
    case map[string]uint8:
        return len(args), nil
    case []map[string]any:
        return len(args), nil
    case map[string]any:
        return len(args), nil
    case [][]int:
        return len(args), nil
    case []any:
        return len(args), nil
    case []SlabInfo:
        return len(args), nil
    case []ProcessInfo:
        return len(args), nil
    case []SystemResources:
        return len(args), nil
    case []MemoryInfo:
        return len(args), nil
    case []CPUInfo:
        return len(args), nil
    case []NetworkIOStats:
        return len(args), nil
    case []DiskIOStats:
        return len(args), nil
    case []ProcessTree:
        return len(args), nil
    case []ProcessMap:
        return len(args), nil
    case []ResourceUsage:
        return len(args), nil
    case []ResourceSnapshot:
        return len(args), nil
    case map[string]SlabInfo:
        return len(args), nil
    case map[string]ProcessInfo:
        return len(args), nil
    case map[string]SystemResources:
        return len(args), nil
    case map[string]MemoryInfo:
        return len(args), nil
    case map[string]CPUInfo:
        return len(args), nil
    case map[string]NetworkIOStats:
        return len(args), nil
    case map[string]DiskIOStats:
        return len(args), nil
    case map[string]ProcessTree:
        return len(args), nil
    case map[string]ProcessMap:
        return len(args), nil
    case map[string]ResourceUsage:
        return len(args), nil
    case map[string]ResourceSnapshot:
        return len(args), nil
    case ProcessMap:
        return len(args.Relations), nil
    }

    // Reflection-based fallback for dynamically constructed types
    v := reflect.ValueOf(args)
    if v.IsValid() {
        switch v.Kind() {
        case reflect.Slice, reflect.Array, reflect.Map, reflect.Chan, reflect.String:
            return v.Len(), nil
        }
    }

    return -1, errors.New(sf("Cannot determine length of unknown type '%T' in len()", args))
}

func getMemUsage() (uint64, uint64) {
    var m runtime.MemStats
    runtime.ReadMemStats(&m)
    return m.Alloc, m.Sys
}

func enum_names(ns string, e string) []string {
    globlock.RLock()
    defer globlock.RUnlock()
    l := []string{}
    if !str.Contains(e, "::") {
        if found := uc_match_enum(e); found != "" {
            e = found + "::" + e
        } else {
            e = ns + "::" + e
        }
    }
    if _, found := enum[e]; !found {
        return l
    }
    if len(enum[e].members) == 0 {
        return l
    }
    for _, m := range enum[e].ordered {
        l = append(l, m)
    }
    return l
}

func enum_all(ns string, e string) []any {
    globlock.RLock()
    defer globlock.RUnlock()
    l := []any{}
    if !str.Contains(e, "::") {
        if found := uc_match_enum(e); found != "" {
            e = found + "::" + e
        } else {
            e = ns + "::" + e
        }
    }
    if _, found := enum[e]; !found {
        return l
    }
    if len(enum[e].members) == 0 {
        return l
    }
    for _, m := range enum[e].ordered {
        l = append(l, enum[e].members[m])
    }
    return l
}

// eregister - internal function for registering exceptions in the ex enum
// Used by both system initialization and the exreg() stdlib function
func eregister(name string, severity string) bool {
    globlock.Lock()
    defer globlock.Unlock()

    enumName := "main::ex"

    // Check if enum exists, create if not
    if enum[enumName] == nil {
        enum[enumName] = &enum_s{
            members:   make(map[string]any),
            ordered:   []string{},
            namespace: "main",
        }
    }

    // Check if member already exists
    if _, exists := enum[enumName].members[name]; exists {
        return false // Silent failure - member already exists
    }

    // Add new exception with severity
    enum[enumName].members[name] = severity
    enum[enumName].ordered = append(enum[enumName].ordered, name)

    return true
}

// initializeExceptionEnum - sets up the ex enum at startup
func initializeExceptionEnum() {
    // Create the ex enum first
    globlock.Lock()
    enumName := "main::ex"
    enum[enumName] = &enum_s{
        members:   make(map[string]any),
        ordered:   []string{},
        namespace: "main",
    }
    globlock.Unlock()

    // Add all categories[] keys with default severity
    for categoryKey := range categories {
        eregister(categoryKey, "error") // Default severity for categories
    }

    // Add common runtime exceptions with appropriate severities
    eregister("divide_by_zero", "error")
    eregister("null_pointer", "error")
    eregister("index_out_of_bounds", "error")
    eregister("stack_overflow", "error")
    eregister("memory_exhausted", "error")
    eregister("invalid_operation", "error")
    eregister("not_implemented", "warn")
    eregister("timeout", "warn")
    eregister("cancelled", "info")
    eregister("access_denied", "error")
    eregister("configuration_error", "error")
    eregister("validation_failed", "warn")
    eregister("unknown", "error") // Default severity for unknown exceptions
}

// GetAst(): returns a representation of the tokenised
// phrases in a function. this is not an ast, but serves
// the same purpose for us.
func GetAst(fn string) (ast string) {
    var ifn uint32
    var present bool
    if ifn, present = fnlookup.lmget(fn); !present {
        return
    }

    if ifn < uint32(len(functionspaces)) {

        var falist []string
        for _, fav := range functionArgs[ifn].args {
            falist = append(falist, fav)
        }

        first := true

        indent := 0
        istring := ""

        for q := range functionspaces[ifn] {
            if first == true {
                first = false
            }

            switch functionspaces[ifn][q].Tokens[0].tokType {
            case C_Endfor, C_Endwhile, C_Endif, C_Endcase:
                indent--
            }

            istring = str.Repeat("....", indent)

            ast += sf("%sLine (bytes:%d)  :\n", istring, Of(functionspaces[ifn][q]))

            for tk, tv := range functionspaces[ifn][q].Tokens {
                ast += sf("%s%6d : ", istring, 1+tk)
                subast1 := sf("(%s", tokNames[tv.tokType])
                if tv.subtype != 0 {
                    subast1 += sf(",subtype:%s", subtypeNames[tv.subtype])
                }
                subast1 += sf(")")
                ast += sf("%29s", subast1)
                show := str.TrimSpace(tr(str.Replace(tv.tokText, "\n", " ", -1), SQUEEZE, " ", ""))
                ast += sparkle(sf(" [#1]%+v[#-]", show))
                switch tv.tokVal.(type) {
                default:
                    if tv.tokVal != nil {
                        ast += sf(" Value : %+v (%T)", tv.tokVal, tv.tokVal)
                    }
                }
                ast += "\n"

            }
            ast += "\n"

            switch functionspaces[ifn][q].Tokens[0].tokType {
            case C_For, C_Foreach, C_While, C_If, C_Case:
                indent++
            }

        }

    }

    return ast
}

/* for future use:
func sttyFlag(flags string,state bool) (okay bool) {
    termios, err := unix.IoctlGetTermios(0, ioctlReadTermios)
    newState := *termios
    if err!=nil {
        return false
    }
    for fp:=0;fp<len(flags); fp++ {
        f:=flags[fp]
        if state {
            switch f {
            case 'n':
                newState.Iflag |= unix.ICRNL
            case 'i':
                newState.Iflag |= unix.INLCR
                newState.Iflag |= unix.IGNCR
            case 'u':
                newState.Iflag |= unix.IUCLC
            case 's':
                newState.Lflag |= unix.ISIG
            case 'c':
                newState.Lflag |= unix.ICANON
            case 'e':
                newState.Lflag |= unix.ECHO
            }
        } else {
            switch f {
            case 'n':
                newState.Iflag &^= unix.ICRNL
            case 'i':
                newState.Iflag &^= unix.INLCR
                newState.Iflag &^= unix.IGNCR
            case 'u':
                newState.Iflag &^= unix.IUCLC
            case 's':
                newState.Lflag &^= unix.ISIG
            case 'c':
                newState.Lflag &^= unix.ICANON
            case 'e':
                newState.Lflag &^= unix.ECHO
            }
        }
        unix.IoctlSetTermios(0, ioctlWriteTermios, &newState)
    }
    return true
}
*/

func buildInternalLib() {

    features["internal"] = Feature{version: 1, category: "debug"}
    categories["internal"] = []string{"last", "last_err", "zsh_version", "bash_version", "bash_versinfo", "user", "os", "home", "lang",
        "release_name", "release_version", "release_id", "winterm", "hostname", "argc", "argv",
        "funcs", "keypress", "tokens", "key", "clear_line", "pid", "ppid", "system",
        "func_inputs", "func_outputs", "func_descriptions", "func_categories",
        "local", "clktck", "funcref", "thisfunc", "thisref", "cursoron", "cursoroff", "cursorx",
        "eval", "exec", "term_w", "term_h", "pane_h", "pane_w", "pane_r", "pane_c", "utf8supported", "execpath", "trap", "coproc",
        "capture_shell", "ansi", "interpol", "shell_pid", "has_shell", "has_term", "term", "has_colour",
        "len", "rlen", "echo", "get_row", "get_col", "unmap", "await", "get_mem", "zainfo", "get_cores", "permit",
        "enum_names", "enum_all", "dump", "mdump", "sysvar", "expect",
        "ast", "varbind", "sizeof", "dup", "defined", "log_queue_status",
        "set_depth",
        "logging_stats", "exreg", "format_stack_trace", "panic", "array_format", "array_colours",
        "import_errors", "import_has_errors",
        // "suppress_prompt", "conread","conwrite","conset","conclear", : for future use.
    }

    slhelp["expect"] = LibHelp{in: "[]arguments,variant_count,[]variants", out: "bool", action: "returns true if the arguments satisfy the type list.\na variant is an argument count followed by a list of type_strings."}
    stdlib["expect"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("expect", args, 1, "3", "[]interface {}", "int", "[]string"); !ok {
            return nil, err
        }
        // find caller's name and remove trailing func space instance id:
        myName, _ := numlookup.lmget(evalfs)
        replAny, _ := stdlib["replace"](ns, evalfs, ident, myName, "@.*$", "")
        myName = replAny.(string)
        // some type name conversion for "float"==float64 and "interface {}"==any
        variants := args[2].([]string)
        for k, v := range variants {
            switch v {
            case "float":
                variants[k] = "float64"
            case "any":
                variants[k] = "interface {}"
            }
        }
        // make check:
        ok, err := expect_args(myName, args[0].([]any), args[1].(int), variants...)
        if err != nil {
            return false, nil
        }
        return ok, nil
    }

    slhelp["gdump"] = LibHelp{in: "function_name", out: "", action: "Displays system variable list."}
    stdlib["gdump"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("gdump", args, 1, "0"); !ok {
            return nil, err
        }
        for e := 0; e < len(gident); e++ {
            if gident[e].declared {
                pf("%s = %v\n", gident[e].IName, gident[e].IValue)
            }
        }
        return nil, nil
    }
    slhelp["mdump"] = LibHelp{in: "function_name", out: "", action: "Displays global variable list."}
    stdlib["mdump"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("mdump", args, 1, "0"); !ok {
            return nil, err
        }
        for e := 0; e < len(mident); e++ {
            if mident[e].declared {
                pf("%s = %v\n", mident[e].IName, mident[e].IValue)
            }
        }
        return nil, nil
    }

    slhelp["dump"] = LibHelp{in: "function_name", out: "", action: "Displays in-scope variable list."}
    stdlib["dump"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("dump", args, 1, "0"); !ok {
            return nil, err
        }
        for e := 0; e < len(*ident); e++ {
            if (*ident)[e].declared {
                pf("%s = %v\n", (*ident)[e].IName, (*ident)[e].IValue)
            }
        }
        return nil, nil
    }

    slhelp["format_stack_trace"] = LibHelp{in: "[]stackFrame", out: "string", action: "formats a stack trace array into a readable string with numbered frames."}
    stdlib["format_stack_trace"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("format_stack_trace", args, 1, "1", "[]stackFrame"); !ok {
            return nil, err
        }
        stackTrace := args[0].([]stackFrame)
        return formatStackTrace(stackTrace), nil
    }

    slhelp["dup"] = LibHelp{in: "map/array", out: "copy", action: "returns a duplicate copy of [#i1]argument 1[#i0]."}
    stdlib["dup"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("dup", args, 17,
            "1", "map[string]string",
            "1", "map[string]bool",
            "1", "map[string]int",
            "1", "map[string]uint",
            "1", "map[string]float64",
            "1", "map[string]*big.Int",
            "1", "map[string]*big.Float",
            "1", "map[string]interface {}",
            "1", "string",
            "1", "[]string",
            "1", "[]bool",
            "1", "[]int",
            "1", "[]uint",
            "1", "[]float64",
            "1", "[]*big.Int",
            "1", "[]*big.Float",
            "1", "[]interface {}"); !ok {
            return nil, err
        }

        switch m := args[0].(type) {
        case map[string]string:
            m2 := make(map[string]string)
            for id, v := range m {
                m2[id] = v
            }
            return m2, nil
        case map[string]bool:
            m2 := make(map[string]bool)
            for id, v := range m {
                m2[id] = v
            }
            return m2, nil
        case map[string]int:
            m2 := make(map[string]int)
            for id, v := range m {
                m2[id] = v
            }
            return m2, nil
        case map[string]uint:
            m2 := make(map[string]uint)
            for id, v := range m {
                m2[id] = v
            }
            return m2, nil
        case map[string]float64:
            m2 := make(map[string]float64)
            for id, v := range m {
                m2[id] = v
            }
            return m2, nil
        case map[string]*big.Int:
            m2 := make(map[string]*big.Int)
            for id, v := range m {
                m2[id] = v
            }
            return m2, nil
        case map[string]*big.Float:
            m2 := make(map[string]*big.Float)
            for id, v := range m {
                m2[id] = v
            }
            return m2, nil
        case map[string]interface{}:
            m2 := make(map[string]interface{})
            for id, v := range m {
                m2[id] = v
            }
            return m2, nil
        case []bool:
            a2 := make([]bool, len(m), cap(m))
            copy(a2, m)
            return a2, nil
        case []int:
            a2 := make([]int, len(m), cap(m))
            copy(a2, m)
            return a2, nil
        case []uint:
            a2 := make([]uint, len(m), cap(m))
            copy(a2, m)
            return a2, nil
        case []float64:
            a2 := make([]float64, len(m), cap(m))
            copy(a2, m)
            return a2, nil
        case string:
            return str.Clone(m), nil
        case []string:
            a2 := make([]string, len(m), cap(m))
            copy(a2, m)
            return a2, nil
        case []*big.Int:
            a2 := make([]*big.Int, len(m), cap(m))
            copy(a2, m)
            return a2, nil
        case []*big.Float:
            a2 := make([]*big.Float, len(m), cap(m))
            copy(a2, m)
            return a2, nil
        case []interface{}:
            a2 := make([]interface{}, len(m), cap(m))
            copy(a2, m)
            return a2, nil

        default:
            return nil, errors.New(sf("dup requires a map, not a %T", args[0]))
        }
    }

    slhelp["sizeof"] = LibHelp{in: "string", out: "uint", action: "returns the size of an object."}
    stdlib["sizeof"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("sizeof", args, 1, "1", "any"); !ok {
            return nil, err
        }
        return Of(args[0]), nil
    }

    slhelp["varbind"] = LibHelp{in: "string", out: "uint", action: "returns the name binding uint for a variable."}
    stdlib["varbind"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("varbind", args, 1, "1", "string"); !ok {
            return nil, err
        }
        return bind_int(evalfs, args[0].(string)), nil
    }

    slhelp["enum_names"] = LibHelp{in: "enum", out: "[]string", action: "returns the name labels associated with enumeration [#i1]enum[#i0]"}
    stdlib["enum_names"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("enum_names", args, 1, "1", "string"); !ok {
            return nil, err
        }
        return enum_names(ns, args[0].(string)), nil
    }

    slhelp["enum_all"] = LibHelp{in: "enum", out: "[]mixed", action: "returns the values associated with enumeration [#i1]enum[#i0]"}
    stdlib["enum_all"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("enum_all", args, 1, "1", "string"); !ok {
            return nil, err
        }
        return enum_all(ns, args[0].(string)), nil
    }

    /*
       slhelp["conread"] = LibHelp{in: "", out: "termios_struct", action: "reads console state struct."}
       stdlib["conread"] = func(ns string,evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
           if ok,err:=expect_args("conread",args,1,"0"); !ok { return nil,err }
           termios, err := unix.IoctlGetTermios(0, ioctlReadTermios)
           if err!=nil {
               return nil,err
           }
           return termios,nil
       }

       slhelp["conwrite"] = LibHelp{in: "", out: "int", action: "writes console state struct."}
       stdlib["conwrite"] = func(ns string,evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
           if ok,err:=expect_args("conwrite",args,1,"1","*unix.Termios"); !ok { return nil,err }
           return nil,unix.IoctlSetTermios(0, ioctlWriteTermios, args[0].(*unix.Termios))
       }

       slhelp["conclear"] = LibHelp{in: "string", out: "bool", action: "resets console state bits. returns success flag.\nFlags are n:ICRNL i:IGNCR u:IUCLC s:ISIG c:ICANON e:ECHO\nSee man page termios (3) for further details."}
       stdlib["conclear"] = func(ns string,evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
           if ok,err:=expect_args("conclear",args,1,"1","string"); !ok { return nil,err }
           return sttyFlag(args[0].(string),false),nil
       }

       slhelp["conset"] = LibHelp{in: "string", out: "bool", action: "sets console state bits. returns success flag.\nFlags are n:ICRNL i:IGNCR u:IUCLC s:ISIG c:ICANON e:ECHO\nSee man page termios (3) for further details."}
       stdlib["conset"] = func(ns string,evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
           if ok,err:=expect_args("conset",args,1,"1","string"); !ok { return nil,err }
           return sttyFlag(args[0].(string),true),nil
       }
    */

    /*
       slhelp["suppress_prompt"] = LibHelp{in: "bool", out: "", action: "Disable/Enable command/eval prompt."}
       stdlib["suppress_prompt"] = func(ns string,evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
           if ok,err:=expect_args("suppress_prompt",args,1,"1","bool"); !ok { return nil,err }
           prev:=squelch_prompt
           squelch_prompt=args[0].(bool)
           return prev,nil
       }
    */

    slhelp["set_depth"] = LibHelp{in: "int_max_depth", out: "", action: "Sets the maximum directory recurse depth in interactive help mode."}
    stdlib["set_depth"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("set_depth", args, 1, "1", "int"); !ok {
            return nil, err
        }
        gvset("context_dir_depth", args[0].(int))
        return nil, nil
    }

    // Map operations
    slhelp["merge"] = LibHelp{
        in:     "map1, map2",
        out:    "map",
        action: "Deep merge two maps (same as | operator)",
    }
    stdlib["merge"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("merge", args, 1, "2", "map", "map"); !ok {
            return nil, err
        }
        return deepMergeMaps(args[0].(map[string]any), args[1].(map[string]any)), nil
    }

    slhelp["intersect"] = LibHelp{
        in:     "map1, map2",
        out:    "map",
        action: "Keep only keys present in both maps (same as & operator)",
    }
    stdlib["intersect"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("intersect", args, 1, "2", "map", "map"); !ok {
            return nil, err
        }
        return intersectMaps(args[0].(map[string]any), args[1].(map[string]any)), nil
    }

    slhelp["difference"] = LibHelp{
        in:     "map1, map2",
        out:    "map",
        action: "Keep keys from first map that are not in second map (same as - operator)",
    }
    stdlib["difference"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("difference", args, 1, "2", "map", "map"); !ok {
            return nil, err
        }
        return differenceMaps(args[0].(map[string]any), args[1].(map[string]any)), nil
    }

    slhelp["symmetric_difference"] = LibHelp{
        in:     "map1, map2",
        out:    "map",
        action: "Keep keys that exist in exactly one map (same as ^ operator)",
    }
    stdlib["symmetric_difference"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("symmetric_difference", args, 1, "2", "map", "map"); !ok {
            return nil, err
        }
        return symmetricDifferenceMaps(args[0].(map[string]any), args[1].(map[string]any)), nil
    }

    // Set predicate functions
    slhelp["is_subset"] = LibHelp{
        in:     "map1, map2",
        out:    "bool",
        action: "Check if map1 is a subset of map2 (all keys in map1 exist in map2)",
    }
    stdlib["is_subset"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("is_subset", args, 1, "2", "map", "map"); !ok {
            return nil, err
        }
        map1 := args[0].(map[string]any)
        map2 := args[1].(map[string]any)
        intersection := intersectMaps(map1, map2)
        return len(intersection) == len(map1), nil
    }

    slhelp["is_superset"] = LibHelp{
        in:     "map1, map2",
        out:    "bool",
        action: "Check if map1 is a superset of map2 (all keys in map2 exist in map1)",
    }
    stdlib["is_superset"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("is_superset", args, 1, "2", "map", "map"); !ok {
            return nil, err
        }
        map1 := args[0].(map[string]any)
        map2 := args[1].(map[string]any)
        intersection := intersectMaps(map1, map2)
        return len(intersection) == len(map2), nil
    }

    slhelp["is_disjoint"] = LibHelp{
        in:     "map1, map2",
        out:    "bool",
        action: "Check if two maps are disjoint (no keys in common)",
    }
    stdlib["is_disjoint"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("is_disjoint", args, 1, "2", "map", "map"); !ok {
            return nil, err
        }
        map1 := args[0].(map[string]any)
        map2 := args[1].(map[string]any)
        intersection := intersectMaps(map1, map2)
        return len(intersection) == 0, nil
    }

    slhelp["log_queue_status"] = LibHelp{in: "", out: "struct", action: "Returns logging queue status: [#i1].used[#i0]: current queue usage, [#i1].total[#i0]: queue size, [#i1].running[#i0]: worker status, [#i1].percentage[#i0]: usage percentage"}
    stdlib["log_queue_status"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("log_queue_status", args, 0); !ok {
            return nil, err
        }
        used, total, running := getLogQueueUsage()
        percentage := float64(0)
        if total > 0 {
            percentage = (float64(used) / float64(total)) * 100.0
        }
        return map[string]any{
            "used":       used,
            "total":      total,
            "running":    running,
            "percentage": percentage,
        }, nil
    }

    slhelp["zainfo"] = LibHelp{in: "", out: "struct", action: "internal info: [#i1].version[#i0]: semantic version number, [#i1].name[#i0]: language name, [#i1].build[#i0]: build type"}
    stdlib["zainfo"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("zainfo", args, 0); !ok {
            return nil, err
        }
        v, _ := gvget("@version")
        l, _ := gvget("@language")
        c, _ := gvget("@ct_info")
        return zainfo{Version: v.(string), Name: l.(string), Build: c.(string)}, nil
    }

    slhelp["sysvar"] = LibHelp{in: "system_variable_name", out: "struct", action: "Returns the value of a system variable."}
    stdlib["sysvar"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("sysvar", args, 1, "1", "string"); !ok {
            return nil, err
        }
        v, _ := gvget(args[0].(string))
        return v, nil
    }

    slhelp["dinfo"] = LibHelp{in: "var", out: "struct", action: "(debug) show var info."}
    stdlib["dinfo"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("dinfo", args, 0); !ok {
            return nil, err
        }
        bindlock.RLock()
        pf("EvalFS  : %d\n", evalfs)
        pf("Bindings:\n%#v\n", bindings[evalfs])
        pf("Ident   :\n")
        bindlock.RUnlock()
        for k, i := range *ident {
            pf("%3d : %+v\n", k, i)
        }
        pf("\n")
        return nil, nil
    }

    slhelp["utf8supported"] = LibHelp{in: "", out: "bool", action: "Is the current language utf-8 compliant? This only works if the environmental variable LANG is available."}
    stdlib["utf8supported"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("utf8supported", args, 0); !ok {
            return nil, err
        }
        return str.HasSuffix(str.ToLower(os.Getenv("LANG")), ".utf-8"), nil
    }

    slhelp["wininfo"] = LibHelp{in: "", out: "int", action: "(windows only) Returns the console geometry."}
    stdlib["wininfo"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("wininfo", args, 2,
            "1", "int",
            "0"); !ok {
            return nil, err
        }
        hnd := 1
        if len(args) == 1 {
            hnd = args[0].(int)
        }
        return GetWinInfo(hnd), nil
    }

    slhelp["get_mem"] = LibHelp{in: "", out: "struct",
        action: "Returns the current heap allocated memory and total system memory usage in MB.\n" +
            "[#SOL]Structure fields are [#i1].alloc[#i0] and [#i1].system[#i0] for allocated space and total system space respectively."}
    stdlib["get_mem"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("get_mem", args, 0); !ok {
            return nil, err
        }
        a, s := getMemUsage()
        return struct {
            Alloc  uint64
            System uint64
        }{a / 1024 / 1024, s / 1024 / 1024}, nil
    }

    slhelp["get_cores"] = LibHelp{in: "", out: "int", action: "Returns the CPU core count."}
    stdlib["get_cores"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("get_cores", args, 0); !ok {
            return nil, err
        }
        return runtime.NumCPU(), nil
    }

    slhelp["defined"] = LibHelp{in: "string", out: "bool", action: "checks if a constant, variable, or map key is defined in the current scope or USE chain"}
    stdlib["defined"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if len(args) != 1 {
            return nil, fmt.Errorf("defined() requires exactly 1 argument (constant/variable name or map access)")
        }

        exprStr := GetAsString(args[0])

        // Check if this is a map key access expression (contains '[')
        if strings.Contains(exprStr, "[") {
            // Parse map access: varname["key"] or varname[index]
            bracketPos := strings.Index(exprStr, "[")
            if bracketPos > 0 {
                varName := strings.TrimSpace(exprStr[:bracketPos])

                // Get the variable
                bin := bind_int(evalfs, varName)
                if bin >= uint64(len(*ident)) || !(*ident)[bin].declared {
                    return false, nil // Variable doesn't exist
                }

                varValue := (*ident)[bin].IValue

                // Check if it's a map
                if m, ok := varValue.(map[string]any); ok {
                    // Extract the key from the brackets
                    keyPart := exprStr[bracketPos+1:]
                    if endBracket := strings.Index(keyPart, "]"); endBracket > 0 {
                        keyExpr := strings.TrimSpace(keyPart[:endBracket])
                        // Remove surrounding quotes if present
                        keyExpr = strings.Trim(keyExpr, "\"")

                        // Check if key exists in map
                        _, exists := m[keyExpr]
                        return exists, nil
                    }
                }

                // Not a map, return false
                return false, nil
            }
        }

        // Original behavior: check constants and simple variables
        constantName := exprStr

        // Check module constants in USE chain
        chainlock.RLock()
        moduleConstantsLock.RLock()
        defer moduleConstantsLock.RUnlock()
        defer chainlock.RUnlock()

        for p := 0; p < len(uchain); p += 1 {
            if constMap, exists := moduleConstants[uchain[p]]; exists {
                if _, found := constMap[constantName]; found {
                    return true, nil
                }
            }
        }

        // Check local variables directly in ident table
        bin := bind_int(evalfs, constantName)
        if bin < uint64(len(*ident)) && (*ident)[bin].declared {
            return true, nil
        }

        return false, nil
    }

    slhelp["term_h"] = LibHelp{in: "", out: "int", action: "Returns the current terminal height."}
    stdlib["term_h"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("term_h", args, 0); !ok {
            return nil, err
        }
        return MH, nil
    }

    slhelp["term_w"] = LibHelp{in: "", out: "int", action: "Returns the current terminal width."}
    stdlib["term_w"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("term_w", args, 0); !ok {
            return nil, err
        }
        return MW, nil
    }

    slhelp["pane_h"] = LibHelp{in: "", out: "int", action: "Returns the current pane height."}
    stdlib["pane_h"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("pane_h", args, 0); !ok {
            return nil, err
        }
        return panes[currentpane].h, nil
    }

    slhelp["pane_w"] = LibHelp{in: "", out: "int", action: "Returns the current pane width."}
    stdlib["pane_w"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("pane_w", args, 0); !ok {
            return nil, err
        }
        return panes[currentpane].w, nil
    }

    slhelp["pane_r"] = LibHelp{in: "", out: "int", action: "Returns the current pane start row."}
    stdlib["pane_r"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("pane_r", args, 0); !ok {
            return nil, err
        }
        return panes[currentpane].row, nil
    }

    slhelp["pane_c"] = LibHelp{in: "", out: "int", action: "Returns the current pane start column."}
    stdlib["pane_c"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("pane_c", args, 0); !ok {
            return nil, err
        }
        return panes[currentpane].col, nil
    }

    slhelp["interpolate"] = LibHelp{in: "string", out: "int", action: "Returns [#i1]string[#i0] with interpolated content."}
    stdlib["interpolate"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("interpolate", args, 1, "1", "string"); !ok {
            return nil, err
        }
        return interpolate(ns, evalfs, ident, args[0].(string)), nil
    }

    slhelp["system"] = LibHelp{in: "string[,bool]", out: "string", action: "Executes command [#i1]string[#i0] and returns a command structure (bool==false) or displays (bool==true) the output."}
    stdlib["system"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("system", args, 2,
            "2", "string", "bool",
            "1", "string"); !ok {
            return nil, err
        }

        cmd := interpolate(ns, evalfs, ident, args[0].(string))
        if len(args) == 2 && args[1] == true {
            system(cmd, true)
            return nil, nil
        }

        return system(cmd, false), nil
    }

    slhelp["argv"] = LibHelp{in: "", out: "[]string", action: "CLI arguments as an array."}
    stdlib["argv"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("argv", args, 2,
            "1", "int",
            "0"); !ok {
            return nil, err
        }
        if len(args) == 1 {
            return cmdargs[args[0].(int)], nil
        }
        return cmdargs, nil
    }

    slhelp["argc"] = LibHelp{in: "", out: "int", action: "CLI argument count."}
    stdlib["argc"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("argc", args, 0); !ok {
            return nil, err
        }
        return len(cmdargs), nil
    }

    slhelp["eval"] = LibHelp{in: "string", out: "[mixed]", action: "evaluate expression in [#i1]string[#i0]."}
    stdlib["eval"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("eval", args, 1, "1", "string"); !ok {
            return nil, err
        }

        if !permit_eval {
            panic(fmt.Errorf("eval() not permitted!"))
        }

        p := &leparser{}
        calllock.RLock()
        p.ident = ident
        p.fs = evalfs
        p.namespace = ns
        p.ctx = withProfilerContext(context.Background())
        calllock.RUnlock()

        // pf("-- [eval] ns %s fs %s q:|%s|\n",ns,evalfs,args[0].(string))
        return ev(p, evalfs, args[0].(string))
    }

    slhelp["exec"] = LibHelp{in: "string", out: "return_values", action: "execute code in [#i1]string[#i0]."}
    stdlib["exec"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {

        if !permit_eval {
            panic(fmt.Errorf("exec() not permitted!"))
        }

        execMode = true // racey: don't care, error reporting stuff
        execFs = evalfs

        var code string
        if len(args) > 0 {
            switch args[0].(type) {
            case string:
                code = args[0].(string) + "\n"
            default:
                return nil, errors.New("exec requires a string to lex.")
            }
        }

        ctx := withProfilerContext(context.Background())

        // allocate function space for source
        sloc, sfn := GetNextFnSpace(true, "exec_@", call_s{prepared: true, caller: evalfs})

        // parse
        badword, _ := phraseParse(ctx, sfn, code, 0, 0)
        if badword {
            return nil, errors.New("exec could not lex input.")
        }

        // allocate function space for execution
        basemodmap[sloc] = "main"
        eloc, efn := GetNextFnSpace(true, sfn+"@", call_s{prepared: true})
        cs := calltable[eloc]
        cs.caller = evalfs
        cs.base = sloc
        cs.retvals = nil
        cs.fs = efn
        calltable[eloc] = cs
        var instance_ident = make([]Variable, identInitialSize)

        // pf("[#5](debug-exec) : sloc -> %d eloc -> %d[#-]\n", sloc, eloc)
        // pf("[#5](debug-exec) : executing -> [%+v][#-]\n", code)

        // execute code
        atomic.AddInt32(&concurrent_funcs, 1)

        var rcount uint8
        // Set the callLine field in the calltable entry before calling the function
        // For eval calls, we don't have parser context, so use 0
        atomic.StoreInt32(&calltable[eloc].callLine, 0) // Eval calls don't have parser context

        if len(args) > 1 {
            rcount, _, _, _, err = Call(ctx, MODE_NEW, &instance_ident, eloc, ciEval, false, nil, "", []string{}, nil, args[1:]...)
        } else {
            rcount, _, _, _, err = Call(ctx, MODE_NEW, &instance_ident, eloc, ciEval, false, nil, "", []string{}, nil)
        }

        if err != nil {
            return nil, err
        }

        execMode = false
        atomic.AddInt32(&concurrent_funcs, -1)

        // get return values
        calllock.Lock()
        res := calltable[eloc].retvals
        calltable[eloc].gcShyness = 50
        calltable[eloc].gc = true
        calltable[eloc].disposable = true
        calllock.Unlock()

        // throw away the tokenised source
        fnlookup.lmdelete(sfn)
        numlookup.lmdelete(sloc)

        // throw away the code instance block
        fnlookup.lmdelete(efn)
        numlookup.lmdelete(eloc)

        // parse return'ed values
        switch rcount {
        case 0:
            return nil, nil
        case 1:
            return res.([]any)[0], nil
        default:
            return res, nil
        }
        // @unreachable:
        // return res,nil

    }

    slhelp["get_row"] = LibHelp{in: "", out: "int", action: "reads the row position of console text cursor."}
    stdlib["get_row"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("get_row", args, 0); !ok {
            return nil, err
        }
        r, _ := GetCursorPos()
        if runtime.GOOS == "windows" {
            r++
        }
        return r, nil
    }

    slhelp["get_col"] = LibHelp{in: "", out: "int", action: "reads the column position of console text cursor."}
    stdlib["get_col"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("get_col", args, 0); !ok {
            return nil, err
        }
        _, c := GetCursorPos()
        if runtime.GOOS == "windows" {
            c++
        }
        return c, nil
    }

    slhelp["echo"] = LibHelp{in: "[bool[,mask]]", out: "bool",
        action: "Enable or disable local echo. Optionally, set the mask character to be used during input.\n[#SOL]" +
            "Current visibility state is returned when no arguments are provided."}
    stdlib["echo"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("echo", args, 2,
            "2", "bool", "string",
            "1", "bool"); !ok {
            return nil, err
        }

        se := true
        if args[0].(bool) {
            gvset("@echo", true)
        } else {
            se = false
            gvset("@echo", false)
        }

        mask, _ := gvget("@echomask")
        if len(args) > 1 {
            mask = args[1].(string)
        }

        setEcho(se)
        gvset("@echomask", mask)
        v, _ := gvget("@echo")

        return v, nil
    }

    slhelp["permit"] = LibHelp{in: "behaviour_string,various_types", out: "", action: "Set a run-time behaviour... [#2]uninit[#-]: should stop for uninitialised variables / [#2]dupmod[#-]: ignore duplicate imports\n" +
        "[#SOL][#2]exitquiet[#-]: shorter error message / [#2]shell[#-]: permit shell commands / [#2]eval[#-]: permit eval() calls\n" +
        "[#SOL][#2]interpol[#-]: permit string interpolation / [#2]cmdfallback[#-]: shell call on eval failure (interactive)\n" +
        "[#SOL][#2]permit[#-]: enable/disable permit() / [#2]exception_strictness[#-]: enable/disable exception_strictness call\n" +
        "[#SOL][#2]macro[#-]: enable/disable macro statement / [#2]sanitisation[#-]: enable/disable the sanitisation_enable() call\n",
    }
    stdlib["permit"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("permit", args, 4,
            "2", "string", "bool",
            "2", "string", "int",
            "2", "string", "float64",
            "2", "string", "string"); !ok {
            return nil, err
        }

        if !permit_permit {
            panic(fmt.Errorf("permit() not permitted!"))
        }

        lastlock.Lock()
        defer lastlock.Unlock()

        switch str.ToLower(args[0].(string)) {
        case "uninit":
            switch args[1].(type) {
            case bool:
                permit_uninit = args[1].(bool)
                return nil, nil
            default:
                return nil, errors.New("permit(uninit) accepts a boolean value only.")
            }
        case "dupmod":
            switch args[1].(type) {
            case bool:
                permit_dupmod = args[1].(bool)
                return nil, nil
            default:
                return nil, errors.New("permit(dupmod) accepts a boolean value only.")
            }
        case "exitquiet":
            switch args[1].(type) {
            case bool:
                permit_exitquiet = args[1].(bool)
                return nil, nil
            default:
                return nil, errors.New("permit(exitquiet) accepts a boolean value only.")
            }
        case "shell":
            switch args[1].(type) {
            case bool:
                permit_shell = args[1].(bool)
                return nil, nil
            default:
                return nil, errors.New("permit(shell) accepts a boolean value only.")
            }

        case "cmdfallback":
            switch args[1].(type) {
            case bool:
                permit_cmd_fallback = args[1].(bool)
                return nil, nil
            default:
                return nil, errors.New("permit(cmdfallback) accepts a boolean value only.")
            }

        case "eval":
            switch args[1].(type) {
            case bool:
                permit_eval = args[1].(bool)
                return nil, nil
            default:
                return nil, errors.New("permit(eval) accepts a boolean value only.")
            }
        case "interpol":
            switch args[1].(type) {
            case bool:
                interpolation = args[1].(bool)
                return nil, nil
            default:
                return nil, errors.New("permit(interpol) accepts a boolean value only.")
            }
        case "permit":
            switch args[1].(type) {
            case bool:
                permit_permit = args[1].(bool)
                return nil, nil
            default:
                return nil, errors.New("permit(permit) accepts a boolean value only.")
            }
        case "error_exit":
            switch args[1].(type) {
            case bool:
                permit_error_exit = args[1].(bool)
                return nil, nil
            default:
                return nil, errors.New("permit(error_exit) accepts a boolean value only.")
            }
        case "exception_strictness":
            switch args[1].(type) {
            case bool:
                permit_exception_strictness = args[1].(bool)
                return nil, nil
            default:
                return nil, errors.New("permit(exception_strictness) accepts a boolean value only.")
            }
        case "macro":
            switch args[1].(type) {
            case bool:
                permit_macro = args[1].(bool)
                return nil, nil
            default:
                return nil, errors.New("permit(macro) accepts a boolean value only.")
            }
        case "sanitisation":
            switch args[1].(type) {
            case bool:
                sanitisationMutex.Lock()
                permit_sanitisation = args[1].(bool)
                sanitisationMutex.Unlock()
                return nil, nil
            default:
                return nil, errors.New("permit(sanitisation) accepts a boolean value only.")
            }
        }

        return nil, errors.New("unrecognised behaviour provided in permit() argument 1. Available behaviours: permit, error_exit, exception_strictness, macro, sanitisization.")
    }

    slhelp["ansi"] = LibHelp{in: "bool", out: "previous_bool", action: "Enable (default) or disable ANSI colour support at runtime. Returns the previous state."}
    stdlib["ansi"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("ansi", args, 1, "1", "bool"); !ok {
            return nil, err
        }
        lastam := ansiMode
        lastlock.Lock()
        ansiMode = args[0].(bool)
        lastlock.Unlock()
        setupAnsiPalette()
        return lastam, nil
    }

    slhelp["feed"] = LibHelp{in: "bool", out: "bool", action: "(debug) Toggle for enforced interactive mode line feed."}
    stdlib["feed"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("feed", args, 1, "1", "bool"); !ok {
            return nil, err
        }
        lastlock.Lock()
        interactiveFeed = args[0].(bool)
        lastlock.Unlock()
        return nil, nil
    }

    slhelp["interpol"] = LibHelp{in: "bool", out: "bool",
        action: "Enable (default) or disable string interpolation at runtime.\n" +
            "[#SOL]This is useful for ensuring that braced phrases remain unmolested. Returns the previous state."}
    stdlib["interpol"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("interpol", args, 1, "1", "bool"); !ok {
            return nil, err
        }
        lastlock.Lock()
        prev := interpolation
        interpolation = args[0].(bool)
        lastlock.Unlock()
        return prev, nil
    }

    slhelp["coproc"] = LibHelp{in: "bool", out: "", action: "Select if | and =| commands should execute in the coprocess (true) or the current Za process (false)."}
    stdlib["coproc"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("bool", args, 1, "1", "bool"); !ok {
            return nil, err
        }
        gvset("@runInParent", !args[0].(bool))
        return nil, nil
    }

    slhelp["trap"] = LibHelp{in: "trap_type_string,function_call_string", out: "", action: "set a responding function for a given trap type.\nCurrently supported trap types: int, error"}
    stdlib["trap"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("trap", args, 1, "2", "string", "string"); !ok {
            return nil, err
        }
        switch str.ToLower(args[0].(string)) {
        case "int":
            gvset("@trapInt", args[1].(string))
        case "error":
            gvset("@trapError", args[1].(string))
        }
        return nil, nil
    }

    slhelp["capture_shell"] = LibHelp{in: "bool", out: "", action: "Select if | and =| commands should capture output."}
    stdlib["capture_shell"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("capture_shell", args, 1, "1", "bool"); !ok {
            return nil, err
        }
        gvset("@commandCapture", args[0].(bool))
        return nil, nil
    }

    slhelp["funcref"] = LibHelp{in: "name", out: "func_ref_num", action: "Find a function handle."}
    stdlib["funcref"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("funcref", args, 1, "1", "string"); !ok {
            return nil, err
        }
        lmv, _ := fnlookup.lmget(args[0].(string))
        return lmv, nil
    }

    slhelp["thisfunc"] = LibHelp{in: "", out: "string", action: "Find this function's name."}
    stdlib["thisfunc"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("thisfunc", args, 0); !ok {
            return nil, err
        }
        nv, _ := numlookup.lmget(evalfs)
        return nv, nil
    }

    slhelp["thisref"] = LibHelp{in: "", out: "func_ref_num", action: "Find this function's handle."}
    stdlib["thisref"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("thisref", args, 0); !ok {
            return nil, err
        }
        i, _ := GetAsInt(evalfs)
        return i, nil
    }

    slhelp["local"] = LibHelp{in: "string", out: "value", action: "Return this local variable's value."}
    stdlib["local"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("local", args, 1, "1", "string"); !ok {
            return nil, err
        }
        name := args[0].(string)
        v, found := vget(nil, evalfs, ident, name)
        if found {
            return v, nil
        }
        return nil, errors.New(sf("'%v' does not exist!", name))
    }

    slhelp["len"] = LibHelp{in: "various_types", out: "integer", action: "Returns length of string or list."}
    stdlib["len"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if len(args) == 1 {
            return ulen(args[0])
        }
        return nil, errors.New("Bad argument in len()")
    }

    slhelp["rlen"] = LibHelp{in: "string", out: "integer", action: "Returns length of string in runes."}
    stdlib["rlen"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("rlen", args, 1, "1", "string"); !ok {
            return nil, err
        }
        return rlen(args[0].(string)), nil
    }

    slhelp["await"] = LibHelp{in: "handle_map[,all_flag]", out: "[]result", action: "Checks for async completion."}
    stdlib["await"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("await", args, 2,
            "2", "string", "bool",
            "1", "string"); !ok {
            return nil, err
        }

        waitForAll := false
        if len(args) > 1 {
            waitForAll = args[1].(bool)
        }

        bin := bind_int(evalfs, args[0].(string))

        switch args[0].(type) {
        case string:
            if !(*ident)[bin].declared {
                return nil, errors.New("await requires the name of a local handle map")
            }
        }

        var results = make(map[string]any)

        keepWaiting := true

        for keepWaiting {

            // Have to lock this as the results may be updated
            // concurrently while this loop is running.

            vlock.Lock()
            for k, v := range (*ident)[bin].IValue.(map[string]any) {

                select {
                case retval := <-v.(chan any):

                    if retval == nil { // shouldn't happen
                        fmt.Printf("[await] received result for key: %s → %#v\n", k, retval.(struct {
                            l uint32
                            r any
                        }).r)
                        pf("(k %v) is nil. still waiting for it.\n", k)
                        os.Exit(1) // but you never know!
                    }

                    results[k] = retval.(struct {
                        l uint32
                        r any
                    }).r

                    // close the channel, yes i know, not at the client end, etc
                    close(v.(chan any))

                    // remove async/await pair from handle list
                    delete((*ident)[bin].IValue.(map[string]any), k)

                default:
                }

            }
            vlock.Unlock()

            keepWaiting = false

            if waitForAll {
                if len((*ident)[bin].IValue.(map[string]any)) != 0 {
                    keepWaiting = true
                }
            }
        }
        return results, nil
    }

    slhelp["unmap"] = LibHelp{in: "ary_name,key_name", out: "bool", action: "Remove a map key. Returns true on successful removal."}
    stdlib["unmap"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("unmap", args, 1, "2", "string", "string"); !ok {
            return nil, err
        }

        var v any
        var found bool

        if v, found = vget(nil, evalfs, ident, args[0].(string)); !found {
            return false, nil
        }

        switch v.(type) {
        case map[string]any, map[string]int, map[string]float64, map[string]int64:
        case map[string]bool, map[string]uint:
        default:
            return false, errors.New("unmap requires a map")
        }

        if _, found = v.(map[string]any)[args[1].(string)].(any); found {
            vdelete(evalfs, ident, args[0].(string), args[1].(string))
            return true, nil
        }
        return false, nil
    }

    slhelp["key"] = LibHelp{in: "ary_name,key_name", out: "bool", action: "Does key [#i1]key_name[#i0] exist in associative array [#i1]ary_name[#i0]?"}
    stdlib["key"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("key", args, 1, "2", "string", "string"); !ok {
            return nil, err
        }

        var v any
        var found bool

        if v, found = vget(nil, evalfs, ident, args[0].(string)); !found {
            var mloc uint32
            if interactive {
                mloc = 1
            } else {
                mloc = 2
            }
            if v, found = vget(nil, mloc, &mident, args[0].(string)); !found {
                return false, nil
            }
        }

        key := interpolate(ns, evalfs, ident, args[1].(string))

        switch v := v.(type) {
        case http.Header:
            if _, found = v[key]; found {
                return true, nil
            }
        case map[string]float64:
            if _, found = v[key]; found {
                return true, nil
            }
        case map[string]uint8:
            if _, found = v[key]; found {
                return true, nil
            }
        case map[string]uint:
            if _, found = v[key]; found {
                return true, nil
            }
        case map[string]uint64:
            if _, found = v[key]; found {
                return true, nil
            }
        case map[string]int64:
            if _, found = v[key]; found {
                return true, nil
            }
        case map[string]int:
            if _, found = v[key]; found {
                return true, nil
            }
        case map[string]bool:
            if _, found = v[key]; found {
                return true, nil
            }
        case map[string]string:
            if _, found = v[key]; found {
                return true, nil
            }
        case map[string][]string:
            if _, found = v[key]; found {
                return true, nil
            }
        case map[string]any:
            if _, found = v[key]; found {
                return true, nil
            }
        case map[string][]any:
            if _, found = v[key]; found {
                return true, nil
            }
        default:
            return false, errors.New("key() requires a map")
        }
        return false, nil
    }

    slhelp["last"] = LibHelp{in: "", out: "int", action: "Returns the last received error code from a co-process command."}
    stdlib["last"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("last", args, 0); !ok {
            return nil, err
        }
        v, found := gvget("@last")
        if found {
            i := v.(int)
            return i, nil
        }
        return -1, errors.New("no co-process command has been executed yet.")
    }

    slhelp["execpath"] = LibHelp{in: "", out: "string", action: "Returns the initial working directory."}
    stdlib["execpath"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("execpath", args, 0); !ok {
            return nil, err
        }
        v, _ := gvget("@execpath")
        return string(v.(string)), err
    }

    slhelp["last_err"] = LibHelp{in: "", out: "string", action: "Returns the last received error text from the co-process."}
    stdlib["last_err"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("last_err", args, 0); !ok {
            return nil, err
        }
        v, found := gvget("@last_err")
        if found {
            return v.(string), err
        }
        return "", errors.New("No co-process error has been detected yet.")
    }

    slhelp["zsh_version"] = LibHelp{in: "", out: "string", action: "Returns the zsh version string if present."}
    stdlib["zsh_version"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("zsh_version", args, 0); !ok {
            return nil, err
        }
        v, found := gvget("@zsh_version")
        if !found {
            v = ""
        }
        return v.(string), err
    }

    slhelp["bash_version"] = LibHelp{in: "", out: "string", action: "Returns the full release string of the Bash co-process."}
    stdlib["bash_version"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("bash_version", args, 0); !ok {
            return nil, err
        }
        v, _ := gvget("@bash_version")
        return v.(string), err
    }

    slhelp["bash_versinfo"] = LibHelp{in: "", out: "string", action: "Returns the major version number of the Bash co-process."}
    stdlib["bash_versinfo"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("bash_versinfo", args, 0); !ok {
            return nil, err
        }
        v, _ := gvget("@bash_versinfo")
        return v.(string), err
    }

    slhelp["powershell_version"] = LibHelp{in: "", out: "string", action: "Returns the PowerShell version (Windows only)."}
    stdlib["powershell_version"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("powershell_version", args, 0); !ok {
            return nil, err
        }
        v, found := gvget("@powershell_version")
        if !found {
            return "", nil
        }
        return v.(string), err
    }

    slhelp["cmd_version"] = LibHelp{in: "", out: "string", action: "Returns the CMD version (Windows only)."}
    stdlib["cmd_version"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("cmd_version", args, 0); !ok {
            return nil, err
        }
        v, found := gvget("@cmd_version")
        if !found {
            return "", nil
        }
        return v.(string), err
    }

    slhelp["keypress"] = LibHelp{in: "[timeout_ms]", out: "int", action: "Returns an integer corresponding with a keypress.\n" +
        "[#SOL]Internally, the minimum timeout value is currently 1 decisecond.\n" +
        "[#SOL]See the termios(3) man page for reasoning about VMIN/VTIME."}
    stdlib["keypress"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("keypress", args, 3,
            "2", "int", "bool",
            "1", "int",
            "0"); !ok {
            return nil, err
        }
        timeo := int64(0)
        if len(args) > 0 {
            switch args[0].(type) {
            case string, int:
                ttmp, terr := GetAsInt(args[0])
                timeo = int64(ttmp)
                if terr {
                    return "", errors.New("Invalid timeout value.")
                }
            }
        }

        disp := false
        if len(args) > 1 {
            disp = args[1].(bool)
        }

        k := wrappedGetCh(int(timeo), disp)

        if k == 3 { // ctrl-c
            lastlock.RLock()
            sig_int = true
            lastlock.RUnlock()
        }

        return k, nil
    }

    slhelp["cursoroff"] = LibHelp{in: "", out: "", action: "Disables cursor display."}
    stdlib["cursoroff"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("cursoroff", args, 0); !ok {
            return nil, err
        }
        hideCursor()
        return nil, nil
    }

    slhelp["cursorx"] = LibHelp{in: "n", out: "", action: "Moves cursor to horizontal position [#i1]n[#i0]."}
    stdlib["cursorx"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("cursorx", args, 1, "1", "int"); !ok {
            return nil, err
        }
        cursorX(args[0].(int))
        return nil, nil
    }

    slhelp["cursoron"] = LibHelp{in: "", out: "", action: "Enables cursor display."}
    stdlib["cursoron"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("cursoron", args, 0); !ok {
            return nil, err
        }
        showCursor()
        return nil, nil
    }

    slhelp["ppid"] = LibHelp{in: "", out: "int", action: "Return the pid of parent process."}
    stdlib["ppid"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("ppid", args, 0); !ok {
            return nil, err
        }
        return os.Getppid(), nil
    }

    slhelp["pid"] = LibHelp{in: "", out: "int", action: "Return the pid of the current process."}
    stdlib["pid"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("pid", args, 0); !ok {
            return nil, err
        }
        return os.Getpid(), nil
    }

    slhelp["clear_line"] = LibHelp{in: "row,col", out: "", action: "Clear to the end of the line, starting at row,col in the current pane."}
    stdlib["clear_line"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("clear_line", args, 1, "2", "int", "int"); !ok {
            return nil, err
        }
        atlock.Lock()
        row, rerr := GetAsInt(args[0])
        col, cerr := GetAsInt(args[1])
        atlock.Unlock()
        if !(cerr || rerr) {
            clearToEOPane(row, col)
        }
        return nil, nil
    }

    slhelp["user"] = LibHelp{in: "", out: "string", action: "Returns the parent user of the Bash co-process."}
    stdlib["user"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("user", args, 0); !ok {
            return nil, err
        }
        v, _ := gvget("@user")
        return v.(string), err
    }

    slhelp["os"] = LibHelp{in: "", out: "string", action: "Returns the kernel version name as reported by the coprocess."}
    stdlib["os"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("os", args, 0); !ok {
            return nil, err
        }
        v, _ := gvget("@os")
        return v.(string), err
    }

    slhelp["home"] = LibHelp{in: "", out: "string", action: "Returns the home directory of the user that launched Za as reported by the coprocess."}
    stdlib["home"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("home", args, 0); !ok {
            return nil, err
        }
        v, _ := gvget("@home")
        return v.(string), err
    }

    slhelp["lang"] = LibHelp{in: "", out: "string", action: "Returns the locale name used within the coprocess."}
    stdlib["lang"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("lang", args, 0); !ok {
            return nil, err
        }
        if v, found := gvget("@lang"); found {
            return v.(string), nil
        }
        return "", nil
    }

    slhelp["release_name"] = LibHelp{in: "", out: "string", action: "Returns the OS release name as reported by the coprocess."}
    stdlib["release_name"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("release_name", args, 0); !ok {
            return nil, err
        }
        v, _ := gvget("@release_name")
        return v.(string), err
    }

    slhelp["hostname"] = LibHelp{in: "", out: "string", action: "Returns the current hostname."}
    stdlib["hostname"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("hostname", args, 0); !ok {
            return nil, err
        }
        z, _ := os.Hostname()
        gvset("@hostname", z)
        return z, err
    }

    slhelp["tokens"] = LibHelp{in: "string", out: "struct", action: "Returns a structure containing a list of tokens ([#i1].tokens[#i0]) in a string and a list ([#i1].types[#i0]) of token types."}
    stdlib["tokens"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("tokens", args, 1, "1", "string"); !ok {
            return nil, err
        }
        var toks []string
        var toktypes []string
        var cl int16
        for p := 0; p < len(args[0].(string)); {
            t := nextToken(args[0].(string), evalfs, &cl, p)
            if t.tokPos != -1 {
                p = t.tokPos
            }
            toks = append(toks, t.carton.tokText)
            toktypes = append(toktypes, tokNames[t.carton.tokType])
            if t.eof || t.eol {
                break
            }
        }
        return token_result{Tokens: toks, Types: toktypes}, err
    }

    slhelp["release_version"] = LibHelp{in: "", out: "string", action: "Returns the OS version number."}
    stdlib["release_version"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("release_version", args, 0); !ok {
            return nil, err
        }
        v, _ := gvget("@release_version")
        return v.(string), err
    }

    slhelp["release_id"] = LibHelp{in: "", out: "string", action: "Returns the /etc derived release name."}
    stdlib["release_id"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("release_id", args, 0); !ok {
            return nil, err
        }
        v, _ := gvget("@release_id")
        return v.(string), err
    }

    slhelp["winterm"] = LibHelp{in: "", out: "bool", action: "Is this a WSL terminal?"}
    stdlib["winterm"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("winterm", args, 0); !ok {
            return nil, err
        }
        v, _ := gvget("@winterm")
        return v.(bool), err
    }

    slhelp["func_inputs"] = LibHelp{in: "", out: "[]string", action: "Returns a list of standard library function inputs."}
    stdlib["func_inputs"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("func_inputs", args, 0); !ok {
            return nil, err
        }
        var fm = make(map[string]string)
        for k, i := range slhelp {
            fm[k] = i.in
        }
        return fm, nil
    }

    slhelp["func_outputs"] = LibHelp{in: "", out: "[]string", action: "Returns a list of standard library function outputs."}
    stdlib["func_outputs"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("func_outputs", args, 0); !ok {
            return nil, err
        }
        var fm = make(map[string]string)
        for k, i := range slhelp {
            fm[k] = i.out
        }
        return fm, nil
    }

    slhelp["func_descriptions"] = LibHelp{in: "", out: "[]string", action: "Returns a list of standard library function descriptions."}
    stdlib["func_descriptions"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("func_descriptions", args, 0); !ok {
            return nil, err
        }
        var fm = make(map[string]string)
        for k, i := range slhelp {
            fm[k] = i.action
        }
        return fm, nil
    }

    slhelp["func_categories"] = LibHelp{in: "", out: "[]string", action: "Returns a list of standard library functions."}
    stdlib["func_categories"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("func_categories", args, 0); !ok {
            return nil, err
        }
        return categories, nil
    }

    slhelp["funcs"] = LibHelp{in: "[partial_match[,bool_return]]", out: "string", action: "Returns a list of standard library functions."}
    stdlib["funcs"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("funcs", args, 3,
            "2", "string", "bool",
            "1", "string",
            "0"); !ok {
            return nil, err
        }

        if len(args) == 0 {
            args = append(args, "")
        }

        retstring := false
        if len(args) == 2 {
            retstring = args[1].(bool)
        }

        regex := ""
        funclist := ""
        if args[0].(string) != "" {
            regex = args[0].(string)
        }

        if !regexWillCompile(regex) {
            return nil, fmt.Errorf("invalid match rule in funcs() : %s", regex)
        }

        // sort the keys
        var keys []string
        for k := range categories {
            keys = append(keys, k)
        }
        sort.Strings(keys)

        for _, k := range keys {
            c := k
            v := categories[k]
            matchList := ""
            foundOne := false
            for _, q := range v {
                show := false

                if matched, _ := regexp.MatchString(regex, q); matched {
                    show = true
                }
                if matched, _ := regexp.MatchString(regex, k); matched {
                    show = true
                }

                if show {
                    if _, ok := slhelp[q]; ok {
                        lhs := slhelp[q].out
                        colour := "2"
                        if slhelp[q].out != "" {
                            lhs += " = "
                            colour = "3"
                        }
                        params := slhelp[q].in
                        s_inset, _ := stdlib["inset"](ns, evalfs, ident, sparkle(slhelp[q].action), 8)
                        matchList += sf(sparkle("\n  [#6]Function : [#"+colour+"]%s%s(%s)[#-]\n"), lhs, q, params)
                        matchList += sf(sparkle("[#7]%s[#-]\n"), s_inset)
                    }
                    foundOne = true
                }
            }
            if foundOne {
                funclist += sf("Category: %s\n%s\n", c, matchList)
            }
        }
        if !retstring {
            pf(funclist)
            return nil, nil
        }
        return funclist, nil
    }

    slhelp["ast"] = LibHelp{in: "fn_name", out: "string", action: "Return tokenised phrase representation."}
    stdlib["ast"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("ast", args, 1, "1", "string"); !ok {
            return nil, err
        }
        fname := args[0].(string)
        if fname == "" {
            return "", nil
        }
        _, found := fnlookup.lmget(fname)
        if found {
            return GetAst(fname), nil
        } else {
            return "", nil
        }
    }

    slhelp["has_term"] = LibHelp{in: "", out: "bool", action: "Check if executing with a tty."}
    stdlib["has_term"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("has_term", args, 0); !ok {
            return false, err
        }
        return isatty(), nil
    }

    slhelp["term"] = LibHelp{in: "", out: "string", action: "Returns the OS reported terminal type."}
    stdlib["term"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("term", args, 0); !ok {
            return false, err
        }
        term := os.Getenv("TERM")
        return term, nil
    }

    slhelp["has_colour"] = LibHelp{in: "", out: "bool", action: "Check if tty supports at least 16 colours."}
    stdlib["has_colour"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("has_colour", args, 0); !ok {
            return false, err
        }
        term := os.Getenv("TERM")
        cterms := regexp.MustCompile("(?i)^xterm|^vt100|^vt220|^rxvt|^screen|colour|ansi|cygwin|linux")
        return ansiMode && cterms.MatchString(term), nil
    }

    slhelp["has_shell"] = LibHelp{in: "", out: "bool", action: "Check if a child co-process has been launched."}
    stdlib["has_shell"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("has_shell", args, 0); !ok {
            return nil, err
        }
        v, _ := gvget("@noshell")
        return !v.(bool), nil
    }

    slhelp["shell_pid"] = LibHelp{in: "", out: "int", action: "Get process ID of the launched child co-process."}
    stdlib["shell_pid"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("shell_pid", args, 0); !ok {
            return nil, err
        }
        v, _ := gvget("@shell_pid")
        return v, nil
    }

    slhelp["clktck"] = LibHelp{in: "", out: "int", action: "Get clock ticks from aux file."}
    stdlib["clktck"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("clktck", args, 0); !ok {
            return nil, err
        }
        return getclktck(), nil
    }

    slhelp["logging_stats"] = LibHelp{in: "", out: "struct", action: "Returns comprehensive logging statistics: [#i1].queue_used[#i0]: current queue usage, [#i1].queue_total[#i0]: queue size, [#i1].queue_running[#i0]: worker status, [#i1].main_processed[#i0]: main log requests processed, [#i1].web_processed[#i0]: web access log requests processed"}
    stdlib["logging_stats"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("logging_stats", args, 0); !ok {
            return nil, err
        }
        used, total, running, webRequests, mainRequests := getLogQueueStats()
        return map[string]any{
            "queue_used":     used,
            "queue_total":    total,
            "queue_running":  running,
            "main_processed": mainRequests,
            "web_processed":  webRequests,
        }, nil
    }

    slhelp["exception_strictness"] = LibHelp{in: "mode_string", out: "", action: "Set exception handling strictness mode.\n" +
        "[#SOL]strict: fatal termination on unhandled exceptions (default)\n" +
        "[#SOL]permissive: converts unhandled exceptions to normal panics\n" +
        "[#SOL]warn: prints warning but continues execution\n" +
        "[#SOL]disabled: completely disable try..catch processing",
    }
    stdlib["exception_strictness"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("exception_strictness", args, 1, "1", "string"); !ok {
            return nil, err
        }

        // Security check - must be explicitly permitted
        if !permit_exception_strictness {
            return nil, fmt.Errorf("exception_strictness() not permitted!")
        }

        mode := args[0].(string)
        switch mode {
        case "strict", "permissive", "warn", "disabled":
            exceptionStrictness = mode
            if mode == "disabled" {
                fmt.Println("Warning: Exception handling disabled - try..catch blocks will be ignored")
            }
            return nil, nil
        default:
            return nil, fmt.Errorf("Invalid exception strictness mode: %s (use: strict, permissive, warn, disabled)", mode)
        }
    }

    slhelp["exreg"] = LibHelp{in: "name_string,severity_string", out: "bool", action: "Register a new exception type in the ex enum with severity level. Returns true if successful, false if exception already exists. Severity levels: emerg, alert, crit, err, warn, notice, info, debug. Used to override default LOG level during exception handling."}
    stdlib["exreg"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("exreg", args, 1, "2", "string", "string"); !ok {
            return nil, err
        }

        name := args[0].(string)
        severity := args[1].(string)

        // Validate exception name
        if name == "" {
            return nil, fmt.Errorf("Exception name cannot be empty")
        }

        // Validate severity level
        validSeverities := []string{"emerg", "emergency", "alert", "crit", "critical", "err", "error", "warn", "warning", "notice", "info", "debug"}
        isValid := false
        for _, valid := range validSeverities {
            if strings.ToLower(severity) == valid {
                isValid = true
                break
            }
        }
        if !isValid {
            return nil, fmt.Errorf("Invalid severity level: %s (use: emerg, alert, crit, err, warn, notice, info, debug)", severity)
        }

        // Register the exception using internal function
        success := eregister(name, severity)
        return success, nil
    }

    slhelp["panic"] = LibHelp{in: "message_string", out: "", action: "Calls Go's built-in panic() with the specified message. This tests the panic-to-exception conversion system when error_style() is set to 'exception' or 'mixed'."}
    stdlib["panic"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("panic", args, 1, "1", "string"); !ok {
            return nil, err
        }

        message := args[0].(string)

        // Call Go's built-in panic() with an error type for proper conversion
        panic(fmt.Errorf("%s", message))
    }

    slhelp["array_format"] = LibHelp{in: "bool", out: "", action: "Enable/disable pretty array formatting. Default: false, enabled in interactive mode."}
    stdlib["array_format"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("array_format", args, 1, "1", "any"); !ok {
            return nil, err
        }
        if enabled, ok := args[0].(bool); ok {
            prettyArrays = enabled
            return true, nil
        }
        return nil, errors.New("array_format: expected boolean argument")
    }

    slhelp["array_colours"] = LibHelp{in: "[]string", out: "[]string", action: "Set colour scheme for array depth formatting and return previous colours."}
    stdlib["array_colours"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("array_colours", args, 1, "1", "any"); !ok {
            return nil, err
        }

        // Save current colours to return
        previous := make([]string, len(depthColours))
        copy(previous, depthColours)

        // Handle []string or []any (cast to strings)
        var colours []string
        switch args[0].(type) {
        case []string:
            colours = args[0].([]string)
        case []any:
            anySlice := args[0].([]any)
            colours = make([]string, len(anySlice))
            for i, item := range anySlice {
                if str, ok := item.(string); ok {
                    colours[i] = str
                } else {
                    return nil, errors.New("array_colours: all elements must be strings")
                }
            }
        default:
            return nil, errors.New("array_colours: expected []string or []any")
        }

        // Basic validation - check if colours look like Za colour codes
        for _, colour := range colours {
            if !str.HasPrefix(colour, "[#") || !str.HasSuffix(colour, "]") {
                return nil, errors.New("array_colours: invalid colour format, expected [#colour_name]")
            }
        }

        depthColours = colours
        return previous, nil
    }

    slhelp["import_errors"] = LibHelp{in: "module_alias_string", out: "[]string", action: "Returns a list of import errors for an AUTO module. Each error message describes a struct/union that was skipped due to unresolvable fields. Returns empty list if no errors."}
    stdlib["import_errors"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("import_errors", args, 1, "1", "string"); !ok {
            return nil, err
        }

        aliasVal := args[0].(string)

        autoImportErrorsLock.RLock()
        errors, hasErrors := autoImportErrors[aliasVal]
        autoImportErrorsLock.RUnlock()

        if !hasErrors {
            return []any{}, nil  // Return empty list if no errors
        }

        // Convert []string to []any for Za list
        result := make([]any, len(errors))
        for i, e := range errors {
            result[i] = e
        }
        return result, nil
    }

    slhelp["import_has_errors"] = LibHelp{in: "module_alias_string", out: "bool", action: "Returns true if an AUTO module import had any errors (structs skipped). Useful for quick checks before using optional structs."}
    stdlib["import_has_errors"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("import_has_errors", args, 1, "1", "string"); !ok {
            return nil, err
        }

        aliasVal := args[0].(string)

        autoImportErrorsLock.RLock()
        _, hasErrors := autoImportErrors[aliasVal]
        autoImportErrorsLock.RUnlock()

        return hasErrors, nil
    }

}

func getclktck() int {

    if runtime.GOOS == "windows" {
        return -1
    }

    buf, err := ioutil.ReadFile("/proc/self/auxv")
    if err == nil {
        pb := int(uintSize / 8)
        for i := 0; i < len(buf)-pb*2; i += pb * 2 {
            var tag, val uint
            switch uintSize {
            case 32:
                tag = uint(binary.LittleEndian.Uint32(buf[i:]))
                val = uint(binary.LittleEndian.Uint32(buf[i+pb:]))
            case 64:
                tag = uint(binary.LittleEndian.Uint64(buf[i:]))
                val = uint(binary.LittleEndian.Uint64(buf[i+pb:]))
            }

            switch tag {
            case _AT_CLKTCK:
                if val != 0 {
                    return int(val)
                }
            }
        }
    }
    return int(_SYSTEM_CLK_TCK)
}
