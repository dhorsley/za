package main

import (
    "context"
    "encoding/gob"
    "encoding/json"
    "errors"
    "fmt"
    "io/ioutil"
    "log"
    "math"
    "math/big"
    "os"
    "os/exec"
    "path"
    "path/filepath"
    "reflect"
    "regexp"
    "runtime"
    "sort"
    "strconv"
    "strings"
    str "strings"
    "sync"
    "sync/atomic"
    "time"
    "unsafe"
)

var debugger = &Debugger{
    breakpoints: make(map[uint64]string),
    watchList:   []string{},
    listContext: 10,
}

var activeDebugContext *leparser

// var currentPC int16

func showIdent(ident *[]Variable) {
    for k, e := range *ident {
        pf("%3d -- %s -> %+v -- decl -> %v\n", k, e.IName, e.IValue, e.declared)
    }
}

// populate a struct.
func fillStruct(t *Variable, structvalues []any, Typemap map[string]reflect.Type, hasAry bool, fieldNames []string) error {

    if len(structvalues) > 0 {
        var sfields []reflect.StructField
        offset := uintptr(0)
        nextNamePos := 0
        for svpos := 0; svpos < len(structvalues); svpos += 4 {
            // structvalues: [0] name [1] type [2] boolhasdefault [3] default_value
            nv := structvalues[svpos].(string)
            nt := structvalues[svpos+1].(string)

            if nt == "mixed" {
                nt = "any"
            }

            newtype := Typemap[nt]

            // override name if provided in fieldNames:
            if len(fieldNames) > 0 {
                // pf("Replacing field named '%s' with '%s'\n",nv,fieldNames[nextNamePos])
                nv = fieldNames[nextNamePos]
                if nt == "any" {
                    newtype = reflect.TypeOf((*any)(nil)).Elem()
                } else if nt == "[]" || nt == "[]any" || nt == "[]mixed" {
                    newtype = reflect.TypeOf([]any{})
                } else {
                    newtype = reflect.TypeOf(structvalues[svpos+3])
                    // newtype=Typemap[nt]
                }
                nextNamePos++
            }
            // pf("  ([#2]nv %s [#6]%v[#-]) \n",nv,newtype)

            // populate struct fields:
            sfields = append(sfields,
                reflect.StructField{
                    Name: nv, PkgPath: "main",
                    Type:      newtype,
                    Offset:    offset,
                    Anonymous: false,
                },
            )

            if nt == "any" {
                offset += 32 // interface size
            } else if nt == "[]any" || nt == "[]" {
                offset += reflect.TypeOf([]any{}).Size() // slice size (24 bytes)
            } else {
                offset += Typemap[nt].Size()
            }

        }
        // pf(" (inside fillStruct()) [ sf-> %#v ]\n",sfields)
        new_struct := reflect.StructOf(sfields)
        v := (reflect.New(new_struct).Elem()).Interface()

        if !hasAry {
            // default values setting:

            val := reflect.ValueOf(v)
            tmp := reflect.New(val.Type()).Elem()
            tmp.Set(val)

            nextNamePos := 0
            for svpos := 0; svpos < len(structvalues); svpos += 4 {
                // structvalues: [0] name [1] type [2] boolhasdefault [3] default_value
                nv := structvalues[svpos].(string)
                nhd := structvalues[svpos+2].(bool)
                ndv := structvalues[svpos+3]

                if len(fieldNames) > 0 {
                    nv = fieldNames[nextNamePos]
                    nextNamePos++
                }

                tf := tmp.FieldByName(nv)

                // Bodge: special case assignment of bigi/bigf to coerce type:
                switch tf.Type().String() {
                case "*big.Int":
                    ndv = GetAsBigInt(ndv)
                    nhd = true
                case "*big.Float":
                    ndv = GetAsBigFloat(ndv)
                    nhd = true
                }
                // end-bodge

                if nhd {

                    var intyp reflect.Type
                    if ndv != nil {
                        intyp = reflect.ValueOf(ndv).Type()
                    }

                    if intyp.AssignableTo(tf.Type()) {
                        tf = reflect.NewAt(tf.Type(), unsafe.Pointer(tf.UnsafeAddr())).Elem()
                        tf.Set(reflect.ValueOf(ndv))
                    } else {
                        return fmt.Errorf("cannot set field default (%T) for %v (%v)", ndv, nv, tf.Type())
                    }
                }
            }

            (*t).IValue = tmp.Interface()
            gob.Register((*t).IValue)
            // var tmpArray = reflect.ArrayOf(0,val.Type())
            // gob.Register(tmpArray)

        } else {
            (*t).IValue = []any{}
        }

    } // end-len>0

    return nil
}

func task(caller uint32, base uint32, endClose bool, callname string, iargs ...any) (chan any, string) {

    r := make(chan any)

    loc, id := GetNextFnSpace(true, callname+"@", call_s{prepared: true, base: base, caller: caller, gc: false, gcShyness: 100})
    // fmt.Printf("***** [task]  loc#%d caller#%d, recv cstab: %+v\n",loc,caller,calltable[loc])

    go func() {
        if endClose {
            defer close(r)
        }
        var ident = make([]Variable, identInitialSize)

        atomic.AddInt32(&concurrent_funcs, 1)

        var rcount byte
        var errVal error

        ctx := withProfilerContext(context.Background())
        // Set the callLine field in the calltable entry before calling the function
        // For async calls, we don't have parser context, so use 0
        atomic.StoreInt32(&calltable[loc].callLine, 0) // Async calls don't have parser context

        if enableProfiling {
            id_for_profiling := "async_task: " + str.Replace(id, "@", " instance ", -1)
            startTime := time.Now()
            startProfile(id_for_profiling)
            pushToCallChain(ctx, id_for_profiling)
            rcount, _, _, _, errVal = Call(ctx, MODE_NEW, &ident, loc, ciAsyn, false, nil, "", []string{}, nil, iargs...)
            popCallChain(ctx)
            recordExclusiveExecutionTime(ctx, []string{id_for_profiling}, time.Since(startTime))
        } else {
            rcount, _, _, _, errVal = Call(ctx, MODE_NEW, &ident, loc, ciAsyn, false, nil, "", []string{}, nil, iargs...)
        }
        if errVal != nil {
            panic(errors.New(sf("call error in async task %s", id)))
        }

        // fmt.Printf("[task] sending into chan for key %s: %p\n", id,r)

        switch rcount {
        case 0:
            // pf("[task] [rcount==0 case] sending result for loc %v: %+v\n", loc, nil)
            r <- struct {
                l uint32
                r any
            }{loc, nil}
            // pf("[#3]TASK RESULT : loc %d : no value (nil)[#-]\n",loc)
        case 1:
            calllock.RLock()
            v := calltable[loc].retvals
            // pf("[task] [rcount==1 case] sending result for loc %v: %+v\n", loc, v)
            calllock.RUnlock()
            if v == nil {
                r <- nil
                break
            }
            // pf("[#3]TASK RESULT : loc %d : val (%+v)[#-]\n",loc,v.([]any))
            r <- struct {
                l uint32
                r any
            }{loc, v.([]any)[0]}
        default:
            calllock.RLock()
            v := calltable[loc].retvals
            // pf("[task] [default case] sending result for loc %v: %+v\n", loc, v)
            calllock.RUnlock()
            r <- struct {
                l uint32
                r any
            }{loc, v}
            // pf("[#3]TASK RESULT : loc %d : val (%+v)[#-]\n",loc,v.([]any))
        }

        // Now mark for GC AFTER the send
        calllock.Lock()
        calltable[loc].gcShyness = 10000
        calltable[loc].gc = true
        calllock.Unlock()

        atomic.AddInt32(&concurrent_funcs, -1)

    }()
    return r, id
}

// finish : flag the machine state as okay or in error and
// optionally terminates execution.
func finish(hard bool, i int) {
    if permit_error_exit {

        if logWorkerRunning {
            stopLogWorker()
        }

        if hard {
            os.Exit(i)
        }

        if !interactive {
            os.Exit(i)
        }

        lastlock.Lock()
        sig_int = true
        lastlock.Unlock()
    }
}

func strcmp(a string, b string) bool {
    la := len(a)
    if la != len(b) {
        return false
    }
    if la == 0 {
        return true
    }
    for la > 0 {
        la -= 1
        if a[la] != b[la] {
            return false
        }
    }
    return true
}

func GetAsString(v any) (i string) {
    switch v.(type) {
    case *big.Int:
        n := v.(*big.Int)
        i = n.String()
    case *big.Float:
        f := v.(*big.Float)
        i = f.String()
    default:
        i = sf("%v", v)
    }
    return
}

func GetAsBigInt(i any) *big.Int {
    var ri big.Int
    switch i := i.(type) {
    case uint8:
        ri.SetInt64(int64(i))
    case int64:
        ri.SetInt64(i)
    case uint32:
        ri.SetUint64(uint64(i))
    case uint:
        ri.SetUint64(uint64(i))
    case uint64:
        ri.SetUint64(i)
    case int:
        ri.SetInt64(int64(i))
    case float64:
        ri.SetInt64(int64(i))
    case *big.Int:
        ri.Set(i)
    case *big.Float:
        i.Int(&ri)
    case string:
        ri.SetString(i, 0)
    }
    return &ri
}

func GetAsBigFloat(i any) *big.Float {
    var r big.Float
    switch i := i.(type) {
    case uint8:
        r.SetFloat64(float64(i))
    case int64:
        r.SetFloat64(float64(i))
    case uint32:
        r.SetFloat64(float64(i))
    case uint:
        r.SetFloat64(float64(i))
    case uint64:
        r.SetFloat64(float64(i))
    case int:
        r.SetFloat64(float64(i))
    case float64:
        r.SetFloat64(i)
    case *big.Int:
        r.SetInt(i)
    case *big.Float:
        r.Copy(i)
    case string:
        r.SetString(i)
    }
    return &r
}

// GetAsFloat : converts a variety of types to a float
func GetAsFloat(unk any) (float64, bool) {
    switch i := unk.(type) {
    case int:
        return float64(i), false
    case int16:
        return float64(i), false
    case int32:
        return float64(i), false
    case int64:
        return float64(i), false
    case uint:
        return float64(i), false
    case uint8:
        return float64(i), false
    case uint32:
        return float64(i), false
    case uint64:
        return float64(i), false
    case float64:
        return i, false
    case *big.Float:
        // Convert big.Float to float64 if within limits
        f64, accuracy := i.Float64()
        return f64, accuracy != big.Exact || math.IsInf(f64, 0) || math.IsNaN(f64)
    case string:
        p, e := strconv.ParseFloat(i, 64)
        return p, e != nil
    default:
        return math.NaN(), true
    }
}

// GetAsInt64 : converts a variety of types to int64
func GetAsInt64(expr any) (int64, bool) {
    switch i := expr.(type) {
    case float64:
        return int64(i), false
    case uint:
        return int64(i), false
    case int:
        return int64(i), false
    case int16:
        return int64(i), false
    case int32:
        return int64(i), false
    case int64:
        return i, false
    case uint32:
        return int64(i), false
    case uint64:
        return int64(i), false
    case uint8:
        return int64(i), false
    case string:
        p, e := strconv.ParseFloat(i, 64)
        if e == nil {
            return int64(p), false
        }
    }
    return 0, true
}

func GetAsInt(expr any) (int, bool) {
    switch i := expr.(type) {
    case float64:
        return int(i), false
    case bool:
        if !i {
            return int(0), false
        }
        return int(1), false
    case uint:
        return int(i), false
    case int64:
        return int(i), false
    case uint32:
        return int(i), false
    case uint64:
        return int(i), false
    case uint8:
        return int(i), false
    case int:
        return i, false
    case string:
        if i != "" {
            p, e := strconv.ParseFloat(i, 64)
            if e == nil {
                return int(p), false
            }
        }
    }
    return 0, true
}

func GetAsUint(expr any) (uint, bool) {
    switch i := expr.(type) {
    case float64:
        return uint(i), false
    case int:
        return uint(i), false
    case int64:
        return uint(i), false
    case uint32:
        return uint(i), false
    case uint64:
        return uint(i), false
    case uint8:
        return uint(i), false
    case uint:
        return i, false
    case string:
        p, e := strconv.ParseFloat(i, 64)
        if e == nil {
            return uint(p), false
        }
    default:
    }
    return uint(0), true
}

func GetAsUint64(expr any) (uint64, bool) {
    switch i := expr.(type) {
    case float64:
        return uint64(i), false
    case int:
        return uint64(i), false
    case int64:
        return uint64(i), false
    case uint32:
        return uint64(i), false
    case uint64:
        return i, false
    case uint8:
        return uint64(i), false
    case uint:
        return uint64(i), false
    case string:
        p, e := strconv.ParseFloat(i, 64)
        if e == nil {
            return uint64(p), false
        }
    default:
    }
    return uint64(0), true
}

// check for value in slice - used by lookahead()
func InSlice(a int64, list []int64) bool {
    for k, _ := range list {
        if list[k] == a {
            return true
        }
    }
    return false
}

//
// LOOK-AHEAD FUNCTIONS
//

// searchToken is used by FOR to check for occurrences of the loop variable.
// the presence of indirection always causes a return of true
func searchToken(source_base uint32, start int16, end int16, sval string) bool {

    if sval == "" {
        return false
    }

    range_fs := functionspaces[source_base][start:end]

    for _, v := range range_fs {
        if v.TokenCount == 0 {
            continue
        }
        for r := 0; r < len(v.Tokens); r += 1 {
            if v.Tokens[r].tokType == Identifier && v.Tokens[r].tokText == sval {
                return true
            }
            // check for direct reference
            if str.Contains(v.Tokens[r].tokText, sval) {
                return true
            }
            // on *any* indirect reference return true, as we can't be
            // sure without following the interpolation.
            if str.Contains(v.Tokens[r].tokText, "{{") {
                return true
            }
        }
    }
    return false
}

// lookahead used by if..else..endif and similar constructs for nesting
//
//  @note: lookahead only returns _,_,true when over dedented.
//

func lookahead(fs uint32, startLine int16, indent int, endlevel int, term int64, indenters []int64, dedenters []int64) (bool, int16, bool) {

    // pf("(la) searching for %s from statement #%d\n",tokNames[term],startLine)

    range_fs := functionspaces[fs][startLine:]

    for i, v := range range_fs {

        if len(v.Tokens) == 0 {
            continue
        }

        // indents and dedents
        if InSlice(v.Tokens[0].tokType, indenters) {
            indent += 1
        }
        if InSlice(v.Tokens[0].tokType, dedenters) {
            indent -= 1
        }
        if indent < endlevel {
            return false, 0, true
        }

        // found search term?
        if indent == endlevel && v.Tokens[0].tokType == term {
            return true, int16(i), false
        }
    }

    // return found, distance, nesting_fault_status
    // pf("token %s not found.\n",tokNames[term])
    return false, -1, false

}

// find the next available slot for a function or module
//
//  definition in the functionspace[] list.
//  do_lock normally only false during recursive user-defined fn calls.
func GetNextFnSpace(do_lock bool, requiredName string, cs call_s) (uint32, string) {

    // fmt.Printf("Entered gnfs\n")
    calllock.Lock()

    // : sets up a re-use value
    var reuse, e uint32
    if (globseq % globseq_disposal_freq) == 0 {
        for e = 0; e < globseq; e += 1 {
            if calltable[e].gc && calltable[e].disposable {
                if calltable[e].gcShyness > 0 {
                    calltable[e].gcShyness -= 1
                }
                if calltable[e].gcShyness == 0 {
                    reuse = e
                    // runtime.GC()
                    break
                }
            }
        }
    }

    // find a reservation
    for numlookup.lmexists(globseq) { // reserved
        globseq = (globseq + 1) % gnfsModulus
        if globseq == 0 {
            globseq = 2
        }
    }

    // resize calltable if needed
    for globseq >= uint32(cap(calltable)) {
        if cap(calltable) >= gnfsModulus {
            fmt.Printf("call table overgrown\n")
            finish(true, ERR_FATAL)
            calllock.Unlock()
            return 0, ""
        }
        ncs := make([]call_s, len(calltable)*2, cap(calltable)*2)
        copy(ncs, calltable)
        calltable = ncs
        // fmt.Printf("[gnfs] resized calltable.\n")
    }

    if reuse == 0 {
        reuse = globseq
    }

    // generate new tagged instance name
    newName := requiredName
    if newName[len(newName)-1] == '@' {
        newName += strconv.FormatUint(uint64(reuse), 10)
    }

    // allocate
    calltable[reuse].gc = false
    calltable[reuse].disposable = false
    calltable[reuse].gcShyness = 0
    calltable[reuse].isTryBlock = false // Reset try block flag for reused function spaces
    numlookup.lmset(reuse, newName)
    fnlookup.lmset(newName, reuse)
    if cs.prepared == true {
        cs.fs = newName
        cs.disposable = false
        calltable[reuse] = cs
        // fmt.Printf("[gnfs] populated call table entry # %d with: %+v\n",reuse,calltable[globseq])
    }

    // fmt.Printf("(gnf) allocated for %v with %d\n",newName,reuse)

    calllock.Unlock()
    return reuse, newName

}

// setup mutex locks
var calllock = &sync.RWMutex{}       // function call related
var lastlock = &sync.RWMutex{}       // cached globals
var farglock = &sync.RWMutex{}       // function args manipulation
var fspacelock = &sync.RWMutex{}     // token storage related
var globlock = &sync.RWMutex{}       // enum access lock
var sglock = &sync.RWMutex{}         // setglob lock
var structmapslock = &sync.RWMutex{} // structmaps access
var testlock = &sync.Mutex{}         // test state processing
var atlock = &sync.Mutex{}           // console cursor positioning

// for error reporting : keeps a list of parent->child function calls
//
//  will probably blow up during recursion.
//  errorChain tracks the full call stack (with caller/line info) for error reporting only.
var errorChain []chainInfo

// Stack frame structure for automated stack traces
type stackFrame struct {
    function       string
    line           int16
    caller         string
    filename       string // Add filename for enhanced stack traces
    namespace      string // Add namespace for enhanced stack traces
    calledFunction string // Name of the function that was called
}

// Generate stack trace from the current error chain
func generateStackTrace(currentFunctionName string, currentFunctionLoc uint32, throwLine int16) []stackFrame {
    var trace []stackFrame

    // First, add the current function (the one that threw the exception)
    if currentFunctionName != "" {
        // For the current function, we want the line where the exception was thrown
        currentLine := throwLine

        currentFrame := stackFrame{
            function:  stripNamespacePrefix(stripInstanceNumber(currentFunctionName, 0)), // Current function registrant unknown, assume sync
            line:      currentLine,
            caller:    "",
            filename:  "", // Will be populated if available
            namespace: "", // Will be populated if available
        }
        trace = append(trace, currentFrame)
    }

    // Now add all the callers from errorChain in reverse order (oldest to newest)
    // Each entry in errorChain represents a function that called another function
    for i := len(errorChain) - 1; i >= 0; i-- {
        chain := errorChain[i]

        // For each caller, we want to show the line where this function called the next function
        // The line number is now stored directly in the errorChain entry
        line := chain.line

        frame := stackFrame{
            function:       stripNamespacePrefix(stripInstanceNumber(chain.name, chain.registrant)),
            line:           line,
            caller:         "",                                                                                // chainInfo doesn't have caller field, use empty string
            filename:       chain.filename,                                                                    // Add filename from chainInfo
            namespace:      chain.namespace,                                                                   // Add namespace from chainInfo
            calledFunction: stripNamespacePrefix(stripInstanceNumber(chain.calledFunction, chain.registrant)), // Clean up the called function name
        }
        trace = append(trace, frame)
    }

    return trace
}

// stripInstanceNumber removes the @xx instance number from function names
// but preserves it for async calls (ciAsyn registrant)
func stripInstanceNumber(functionName string, registrant uint8) string {
    // For async calls (ciAsyn), keep the @ suffix to show concurrent instances
    if registrant == ciAsyn {
        return functionName
    }

    // For synchronous calls, strip the @ suffix for cleaner output
    if atIndex := strings.Index(functionName, "@"); atIndex != -1 {
        return functionName[:atIndex]
    }
    return functionName
}

// stripNamespacePrefix removes the namespace:: prefix from function names
func stripNamespacePrefix(functionName string) string {
    if colonIndex := strings.Index(functionName, "::"); colonIndex != -1 {
        return functionName[colonIndex+2:] // Skip "::"
    }
    return functionName
}

// getExceptionSeverity checks if there's an active exception and returns its registered severity level
func getExceptionSeverity(ifs uint32) (int, bool) {
    // Check if there's an active exception in this function space
    exceptionPtr := atomic.LoadPointer(&calltable[ifs].activeException)
    if exceptionPtr == nil {
        return 0, false
    }

    // Cast to exceptionInfo
    excInfo := (*exceptionInfo)(exceptionPtr)
    if excInfo == nil {
        return 0, false
    }

    // Get the exception category
    var category string
    switch cat := excInfo.category.(type) {
    case string:
        category = cat
    case int:
        // For enum values, we'd need to look up the name
        // For now, just return false for enum values
        return 0, false
    default:
        return 0, false
    }

    // Check if this exception has a registered severity in the ex enum
    globlock.RLock()
    defer globlock.RUnlock()

    enumName := "main::ex"
    if exEnum, exists := enum[enumName]; exists {
        if severity, exists := exEnum.members[category]; exists {
            // Convert severity string to log level
            switch strings.ToLower(severity.(string)) {
            case "emerg", "emergency":
                return LOG_EMERG, true
            case "alert":
                return LOG_ALERT, true
            case "crit", "critical":
                return LOG_CRIT, true
            case "err", "error":
                return LOG_ERR, true
            case "warn", "warning":
                return LOG_WARNING, true
            case "notice":
                return LOG_NOTICE, true
            case "info":
                return LOG_INFO, true
            case "debug":
                return LOG_DEBUG, true
            default:
                // Unknown severity, fall back to error
                return LOG_ERR, true
            }
        }
    }

    return 0, false
}

// formatStackTrace formats a stack trace into a readable string with colours
func formatStackTrace(stackTrace []stackFrame) string {
    if len(stackTrace) == 0 {
        return sparkle("[#fred]empty stack trace[#-]")
    }
    var result strings.Builder
    for i, frame := range stackTrace {
        displayLine := frame.line

        // Build the called function info if available
        calledInfo := ""
        if frame.calledFunction != "" {
            calledInfo = fmt.Sprintf(", called %s()", frame.calledFunction)
        }

        // Enhanced format with filename and namespace if available
        if frame.filename != "" && frame.namespace != "" {
            result.WriteString(sparkle(fmt.Sprintf("  [#fyellow]%d:[#-] [#fgreen]%s[#-] in [#fblue]%s[#-] at [#fcyan]%s[#-]:[#fred]%d[#-]%s[#-]\n", i+1, frame.function, frame.namespace, frame.filename, displayLine, calledInfo)))
        } else if frame.filename != "" {
            result.WriteString(sparkle(fmt.Sprintf("  [#fyellow]%d:[#-] [#fgreen]%s[#-] at [#fcyan]%s[#-]:[#fred]%d[#-]%s[#-]\n", i+1, frame.function, frame.filename, displayLine, calledInfo)))
        } else if frame.namespace != "" {
            result.WriteString(sparkle(fmt.Sprintf("  [#fyellow]%d:[#-] [#fgreen]%s[#-] in [#fblue]%s[#-]:[#fred]%d[#-]%s[#-]\n", i+1, frame.function, frame.namespace, displayLine, calledInfo)))
        } else {
            result.WriteString(sparkle(fmt.Sprintf("  [#fyellow]%d:[#-] [#fgreen]%s[#-]:[#fred]%d[#-]%s[#-]\n", i+1, frame.function, displayLine, calledInfo)))
        }
    }
    return result.String()
}

// logException logs an exception to the current logging target
func logException(category any, message string, line int, function string, stackTrace []stackFrame) {
    if !loggingEnabled {
        return
    }

    // Create snapshot of current fields for JSON logging
    var fieldsCopy map[string]any
    if jsonLoggingEnabled {
        fieldsCopy = make(map[string]any)
        for k, v := range logFields {
            fieldsCopy[k] = v
        }
        fieldsCopy["source_line"] = line
        fieldsCopy["function"] = function
        fieldsCopy["category"] = category

        // Add stack trace as structured data
        if len(stackTrace) > 0 {
            stackTraceData := make([]map[string]any, len(stackTrace))
            for i, frame := range stackTrace {
                stackTraceData[i] = map[string]any{
                    "function":  frame.function,
                    "line":      frame.line,
                    "caller":    frame.caller,
                    "filename":  frame.filename,
                    "namespace": frame.namespace,
                }
            }
            fieldsCopy["stack_trace"] = stackTraceData
        }
    }

    // Create the log message
    logMessage := sf("Exception '%v': %s", category, message)

    // Queue the exception log request
    request := LogRequest{
        Message:    logMessage,
        Fields:     fieldsCopy,
        IsJSON:     jsonLoggingEnabled,
        IsError:    true,
        SourceLine: int16(line),
        Level:      LOG_ERR, // Exception logs use ERROR level
        Timestamp:  time.Now(),
    }

    queueLogRequest(request)
}

func showUnhandled(header string, category map[string]any) {

    keys := make([]string, 0, len(category))
    for k := range category {
        if k == "line" {
            continue
        }
        if k == "stack_trace" {
            continue
        }
        keys = append(keys, k)
    }
    sort.Strings(keys)

    pf("%s\n", sparkle(header))
    for _, k := range keys {
        v := category[k]
        switch v := v.(type) {
        case int:
            if v == 0 {
                continue
            }
        case string:
            if v == "" {
                continue
            }
        default:
            if v == nil {
                continue
            }
        }
        if k == "function" {
            v = stripNamespacePrefix(stripInstanceNumber(v.(string), 0))
        }
        pf("  - %9v : %+v\n", k, v)
    }
}

// handleUnhandledExceptionCore applies the exception strictness policy with optional parameters
func handleUnhandledException(excInfo *exceptionInfo, ifs uint32) {
    // Log the exception first (regardless of strictness policy)
    var line int
    var function string
    var stackTrace []stackFrame
    var message string
    var category map[string]any
    var catString string
    function = "unknown"

    if excInfo != nil {
        line = excInfo.line
        function = excInfo.function
        stackTrace = excInfo.stackTrace
        switch excInfo.category.(type) {
        case string:
            catString = (excInfo.category).(string)
        case map[string]any:
            category = (excInfo.category).(map[string]any)
        default:
            catString = sf("%+v", category)
        }

        message = excInfo.message
    }

    // Log the exception
    logException(category, message, line, function, stackTrace)

    switch exceptionStrictness {
    case "strict":
        // Fatal termination with helpful message (default)
        header := "[#2]FATAL: Unhandled exception:"
        if catString == "" {
            showUnhandled(header, category)
        } else {
            pf("  - %9v : %v\n", "category", catString)
        }
        // Show location info if available
        if excInfo != nil {
            pf("[#fred]  at line %d in function %s[#-]\n", excInfo.line, excInfo.function)

            // Show stack trace if available
            if len(excInfo.stackTrace) > 0 {
                pf("[#fred]  Stack trace:[#-]\n")
                pf("%s", formatStackTrace(excInfo.stackTrace))
            }
        } else {
            pf("[#fred]  Program terminated due to unhandled exception[#-]\n")
        }

        finish(false, ERR_EXCEPTION)
    case "permissive":
        // Convert to normal panic
        header := "[#6]Converting unhandled exception to panic:"
        showUnhandled(header, category)
        panic(sf("Unhandled exception: %v - %s", category, message))
    case "warn":
        // Print warning but continue
        header := "[#6]WARNING: Unhandled exception:"
        showUnhandled(header, category)
        if excInfo != nil {
            pf("[#fyellow]  at line %d in function %s (continuing execution)[#-]\n", excInfo.line, excInfo.function)

            // Show stack trace if available
            if len(excInfo.stackTrace) > 0 {
                pf("[#fyellow]  Stack trace:[#-]\n")
                pf("%s", formatStackTrace(excInfo.stackTrace))
            }
        } else {
            pf("[#fyellow]  Program completed with unhandled exception[#-]\n")
        }

        // Clear state and continue atomically (only if ifs is valid)
        if ifs > 0 {
            atomic.StorePointer(&calltable[ifs].activeException, nil)
            atomic.StoreInt32(&calltable[ifs].currentCatchMatched, 0)
        }
    case "disabled":
        // Completely ignore - just clear state (only if ifs is valid)
        if ifs > 0 {
            atomic.StorePointer(&calltable[ifs].activeException, nil)
            atomic.StoreInt32(&calltable[ifs].currentCatchMatched, 0)
        }
    default:
        // Unknown strictness - default to strict
        header := sf("[#2]FATAL: Unhandled exception (unknown strictness '%s', defaulting to strict)", exceptionStrictness)
        showUnhandled(header, category)

        // Show location info if available
        if excInfo != nil {
            pf("[#fred]  at line %d in function %s[#-]\n", excInfo.line, excInfo.function)

            // Show stack trace if available
            if len(excInfo.stackTrace) > 0 {
                pf("[#fred]  Stack trace:[#-]\n")
                pf("%s", formatStackTrace(excInfo.stackTrace))
            }
        } else {
            pf("[#fred]  Program terminated due to unhandled exception[#-]\n")
        }

        finish(false, ERR_EXCEPTION)
    }
}

// defined function entry point
// everything about what is to be executed is contained in calltable[csloc]
func Call(ctx context.Context, varmode uint8, ident *[]Variable, csloc uint32, registrant uint8, method bool, method_value any, kind_override string, arg_names []string, captured_vars []any, va ...any) (retval_count uint8, endFunc bool, method_result any, captured_result []any, callErr error) {

    /*
       dispifs,_:=fnlookup.lmget(calltable[csloc].fs)
       pf("-- Call()\n  -func %s\n  - fs %d\n  - base %d\n  - ident addr : %v\n",
           calltable[csloc].fs,
           dispifs,
           calltable[csloc].base,
           &ident,
       )
    */

    display_fs, _ := numlookup.lmget(calltable[csloc].base)

    calllock.Lock()

    // register call
    caller_str, _ := numlookup.lmget(calltable[csloc].caller)
    if caller_str == "global" {
        caller_str = "main"
    }

    if calltable[csloc].caller != 0 {
        // Get filename for the caller
        var callerFilename string
        if fileMapValue, exists := fileMap.Load(calltable[csloc].caller); exists {
            callerFilename = fileMapValue.(string)
        }

        if len(errorChain) == 0 {
            errorChain = make([]chainInfo, 0, 3)
        }

        if enhancedErrorsEnabled {
            // If arg_names is empty (positional call), get parameter names from function definition
            paramNames := arg_names
            if len(paramNames) == 0 && len(va) > 0 {
                farglock.RLock()
                callerBase := calltable[csloc].base
                if callerBase < uint32(len(functionArgs)) {
                    paramNames = functionArgs[callerBase].args
                }
                farglock.RUnlock()
            }

            errorChain = append(errorChain, chainInfo{
                loc:            calltable[csloc].caller,
                name:           caller_str,
                line:           int16(calltable[csloc].callLine) + 1, // Store the callLine directly
                filename:       callerFilename,
                registrant:     registrant,
                argNames:       paramNames,
                argValues:      va,
                namespace:      currentModule,       // Add current namespace
                calledFunction: calltable[csloc].fs, // Store the name of the function being called
            })
        } else {
            errorChain = append(errorChain, chainInfo{
                loc:            calltable[csloc].caller,
                name:           caller_str,
                line:           int16(calltable[csloc].callLine) + 1, // Store the callLine directly
                filename:       callerFilename,
                registrant:     registrant,
                namespace:      currentModule,       // Add current namespace
                calledFunction: calltable[csloc].fs, // Store the name of the function being called
            })
        }
    }

    // profile setup

    if enableProfiling {
        pushToCallChain(ctx, display_fs)
        startProfile(caller_str)
    }
    startTime := time.Now()

    // set up evaluation parser - one per function
    parser := &leparser{}
    parser.ident = ident
    parser.kind_override = kind_override
    parser.ctx = ctx

    // Read isTryBlock flag while we already have the lock to avoid extra locking
    isTryBlock := calltable[csloc].isTryBlock

    calllock.Unlock()

    lastlock.Lock()
    interparse.ident = ident
    if interactive {
        parser.mident = 1
        interparse.mident = 1
    } else {
        parser.mident = 2
        interparse.mident = 2
    }
    lastlock.Unlock()

    var inbound *Phrase
    var basecode_entry *BaseCode
    var current_with_handle *os.File
    var source_base uint32 // location of the translated source tokens

    // error handler
    defer func() {
        if r := recover(); r != nil {
            // fall back to shell command?
            if interactive && !parser.hard_fault && !parser.std_call && permit_cmd_fallback {
                cmd := basecode[source_base][parser.pc].Original

                s := interpolate(currentModule, 1, &mident, cmd)
                s = str.TrimRight(s, "\n")
                if len(s) > 0 {
                    cop := Copper(s, false)
                    gvset("@last", cop.Code)
                    gvset("@last_err", cop.Err)
                    if !cop.Okay {
                        pf("Error: [%d] in shell command '%s'\n", cop.Code, str.TrimLeft(s, " \t"))
                        pf(cop.Out + "\n")
                        pf(cop.Err + "\n")
                    } else {
                        if len(cop.Out) > 0 {
                            if cop.Out[len(cop.Out)-1] != '\n' {
                                cop.Out += "\n"
                            }
                            pf("%s", cop.Out)
                        }
                    }
                }
                if row >= MH {
                    if row > MH {
                        row = MH
                    }
                    for past := row - MH; past > 0; past-- {
                        at(MH+1, 1)
                        fmt.Print("\n")
                    }
                    row = MH
                }
            } else {

                if !enforceError {
                    callErr = errors.New(sf("suppressed panic: %v", r))
                    return
                }

                // Check for try blocks using enhanced registry before standard error handling

                // Build execution path for context tracking
                executionPath := make([]uint32, 0)
                executionPath = append(executionPath, source_base)

                // Add call chain to execution path (in correct order - newest first)
                for i := len(errorChain) - 1; i >= 0; i-- {
                    executionPath = append(executionPath, errorChain[i].loc)
                }

                parser.hard_fault = false
                if _, ok := r.(runtime.Error); ok {
                    parser.report(inbound.SourceLine, sf("\n%v\n", r))
                    if debugMode {
                        err := r.(error)
                        panic(err)
                    }
                    finish(false, ERR_EVAL)
                }

                var err error
                if errVal, ok := r.(error); ok {
                    err = errVal
                } else {
                    err = errors.New(sf("%v", r))
                }

                // Check for panic-to-exception conversion based on error style mode
                errorStyleLock.RLock()
                currentErrorStyle := errorStyleMode
                errorStyleLock.RUnlock()

                // Check if we should convert panics to exceptions
                // This happens when:
                // 1. User has set error_style() to "exception" or "mixed" mode, OR
                // 2. We're currently inside a try..endtry block
                shouldConvertToException := currentErrorStyle == ERROR_STYLE_EXCEPTION || currentErrorStyle == ERROR_STYLE_MIXED

                // If not in exception mode, check if we're inside a try block
                if !shouldConvertToException {
                    // Look for try blocks in current execution context
                    executionPath := make([]uint32, 0)
                    executionPath = append(executionPath, source_base)

                    // Add call chain to execution path
                    for _, chainEntry := range errorChain {
                        executionPath = append(executionPath, chainEntry.loc)
                    }

                    // Check if there are applicable try blocks
                    applicableTryBlocks := findApplicableTryBlocks(ctx, source_base, executionPath)
                    shouldConvertToException = len(applicableTryBlocks) > 0
                }

                if shouldConvertToException {
                    // Check if this is an ExceptionThrow (already an exception)
                    if excThrow, ok := r.(ExceptionThrow); ok {
                        // This is already an exception, convert to exception state
                        excInfo := &exceptionInfo{
                            category:   excThrow.Category,
                            message:    excThrow.Message,
                            line:       int(inbound.SourceLine) + 1,
                            function:   calltable[csloc].fs,
                            fs:         csloc,
                            stackTrace: generateStackTrace(calltable[csloc].fs, csloc, inbound.SourceLine),
                            source:     "throw",
                        }
                        atomic.StorePointer(&calltable[csloc].activeException, unsafe.Pointer(excInfo))

                        // Set catch matched to false so try/catch blocks can see the exception
                        atomic.StoreInt32(&calltable[csloc].currentCatchMatched, 0)

                        // Return with exception state for try/catch handling
                        callErr = nil
                        return
                    } else {
                        // Convert regular panic to exception
                        category := "panic"
                        message := err.Error()

                        // Check if exceptionInfo is already present (set by dparse or other handler)
                        currentExceptionPtr := atomic.LoadPointer(&calltable[csloc].activeException)
                        currentException := (*exceptionInfo)(currentExceptionPtr)

                        if currentException != nil {
                            // Use the category and message from the existing exceptionInfo
                            category = GetAsString(currentException.category)
                            message = currentException.message
                        } else {
                            // Fallback: Extract category from ?? operator error message if possible
                            if strings.Contains(message, "?? operator failure:") {
                                parts := strings.Split(message, " -> ")
                                if len(parts) > 1 {
                                    category = parts[1]
                                    message = parts[0]
                                }
                            }
                        }

                        excInfo := &exceptionInfo{
                            category:   category,
                            message:    message,
                            line:       int(inbound.SourceLine) + 1,
                            function:   calltable[csloc].fs,
                            fs:         csloc,
                            stackTrace: generateStackTrace(calltable[csloc].fs, csloc, inbound.SourceLine),
                            source:     "panic",
                        }
                        atomic.StorePointer(&calltable[csloc].activeException, unsafe.Pointer(excInfo))

                        // Set catch matched to false so try/catch blocks can see the exception
                        atomic.StoreInt32(&calltable[csloc].currentCatchMatched, 0)

                        // Set exception state for try/catch handling
                        callErr = nil
                        // Don't return - let the function continue so try/catch blocks can see the exception
                    }
                }

                if enhancedErrorsEnabled {
                    // Capture current call arguments only when error occurs
                    // If arg_names is empty (positional call), get parameter names from function definition
                    paramNames := arg_names
                    if len(paramNames) == 0 && len(va) > 0 {
                        farglock.RLock()
                        if source_base < uint32(len(functionArgs)) {
                            paramNames = functionArgs[source_base].args
                        }
                        farglock.RUnlock()
                    }

                    // Get current function name
                    currentFunctionName := calltable[csloc].fs

                    // Check if this is an enhanced expect_args error for additional context
                    if enhancedErr, ok := err.(*EnhancedExpectArgsError); ok {
                        // Show enhanced error context with stdlib function details
                        showEnhancedExpectArgsErrorWithCallArgs(parser, inbound.SourceLine, enhancedErr, parser.fs, nil, currentFunctionName)
                    } else {
                        // Show enhanced error context for any other error type
                        showEnhancedErrorWithCallArgs(parser, inbound.SourceLine, err, parser.fs, nil, currentFunctionName)
                    }
                } else {
                    // Standard error reporting
                    parser.report(inbound.SourceLine, sf("\n%v\n", err))
                    if debugMode {
                        panic(r)
                    }
                    setEcho(true)
                }
                finish(false, ERR_EVAL)
            }
        }
    }()

    // some tracking variables for this function call
    var break_count int // usually 0. when >0 stops breakIn from resetting, used for multi-level breaks.
    var breakIn int64   // true during transition from break to outer.

    var forceEnd bool    // used by BREAK for skipping context checks when bailing from nested constructs.
    var retvalues []any  // return values to be passed back
    var finalline int16  // tracks end of tokens in the function
    var fs string        // current function space name
    var thisLoop *s_loop // pointer to loop information. used in FOR

    // set up the function space

    // -- get call details
    calllock.Lock()
    // unique name for this execution, pre-generated before call
    fs = calltable[csloc].fs

    // where the tokens are:
    source_base = calltable[csloc].base

    currentModule = basemodmap[source_base]
    parser.namespace = currentModule
    interparse.namespace = currentModule
    // pf("in call to %s currentModule set to : %s\n",fs,currentModule)

    // the uint32 id attached to fs name
    ifs, _ := fnlookup.lmget(fs)
    calllock.Unlock()

    fname, exists := fileMap.Load(calltable[source_base].base)
    if !exists {
        panic(errors.New(sf("fileMap entry not found for base=%d", calltable[source_base].base)))
    }

    moduleloc := fname.(string)
    fileMap.Store(ifs, moduleloc)

    // -- generate bindings

    bindlock.Lock()

    // reset bindings
    if ifs >= uint32(cap(bindings)) {
        bindResize()
    }
    if varmode == MODE_NEW {
        bindings[ifs] = make(map[string]uint64)
    }

    // copy bindings from source tokens
    for _, phrase := range functionspaces[source_base] {
        for _, tok := range phrase.Tokens {
            if tok.tokType == Identifier {
                bindings[ifs][tok.tokText] = tok.bindpos
            }
        }
    }
    // pf("Binding table from tokens is:\n%#v\n",bindings[ifs])

    bindlock.Unlock()

    if varmode == MODE_NEW {
        testlock.Lock()
        test_group = ""
        test_name = ""
        test_assert = ""
        testlock.Unlock()
    }

    // generic nesting indentation counter
    // this being local prevents re-entrance i guess
    var depth int

    // stores the active construct/loop types outer->inner
    //  for the break and continue statements
    var lastConstruct = []int64{}

    // initialise condition states: CASE stack depth
    // initialise the loop positions: FOR, FOREACH, WHILE

    // active CASE..ENDCASE statement meta info
    var wc = make([]caseCarton, CASE_CAP)

    // count of active CASE..ENDCASE statements
    var wccount int

    // counters per loop type
    var loops = make([]s_loop, MAX_LOOPS)

    // assign self from calling object
    if method {
        bin := bind_int(ifs, "self")
        vset(nil, ifs, ident, "self", method_value)
        t := (*ident)[bin]
        t.ITyped = false
        t.declared = true
        t.Kind_override = kind_override
        (*ident)[bin] = t
    }

    // assign captured variables from parent scope
    if captured_vars != nil && len(captured_vars) > 0 {
        // Get the captured variable names from the try block registry
        if isTryBlock {
            // Find the try block info to get captured variable names
            for _, tryBlock := range tryBlockRegistry {
                if tryBlock.functionSpace == csloc {
                    for i, varName := range tryBlock.capturedVars {
                        if i < len(captured_vars) {
                            bin := bind_int(ifs, varName)
                            vset(nil, ifs, ident, varName, captured_vars[i])
                            t := (*ident)[bin]
                            t.ITyped = false
                            t.declared = true
                            (*ident)[bin] = t
                        }
                    }
                    break
                }
            }
        }
    }

tco_reentry:

    // assign value to local vars named in functionArgs (the call parameters)
    //  from each va value.
    // - functionArgs[] created at definition time from the call signature

    farglock.RLock()

    if len(va) > 0 {
        if method {
            va = va[1:]
        }
    }

    for q, argName := range functionArgs[source_base].args {

        var value any
        if q < len(va) {
            // Use provided argument
            value = va[q]
        } else if functionArgs[source_base].hasDefault[q] {
            // Use default
            value = functionArgs[source_base].defaults[q]
        } else {
            farglock.RUnlock()
            if enforceError {
                parser.report(-1, sf("missing required argument: %s", argName))
                finish(false, ERR_SYNTAX)
                return
            } else {
                panic(errors.New(sf("missing required argument: %s", argName)))
            }
        }

        if s, ok := value.(string); ok {
            value = interpolate(currentModule, ifs, ident, s)
        }

        vset(nil, ifs, ident, argName, value)

        // Set up typed parameter AFTER vset (vset with nil tok overwrites the Variable)
        if len(functionArgs[source_base].argTypes) > q && functionArgs[source_base].argTypes[q] != "" {
            argType := functionArgs[source_base].argTypes[q]
            bin := bind_int(ifs, argName)
            t := &(*ident)[bin]
            t.ITyped = true
            t.Kind_override = argType
            // Type validation: check if value matches expected type
            if !isCompatibleType(value, argType, currentModule) {
                farglock.RUnlock()
                parser.report(-1, sf("Type mismatch for parameter '%s': expected %s, got %T", argName, argType, value))
                finish(false, ERR_SYNTAX)
                return
            }
        }
    }

    farglock.RUnlock()

    if len(functionspaces[source_base]) > 32767 {
        parser.report(-1, "function too long!")
        finish(true, ERR_SYNTAX)
        return
    }

    // pf("Call Args: %#v\n",va)

    finalline = int16(len(functionspaces[source_base]))

    inside_test := false // are we currently inside a test bock
    inside_with := false // WITH cannot be nested and remains local in scope.

    var structMode bool       // are we currently defining a struct
    var structName string     // name of struct currently being defined
    var structNode []any      // struct builder
    var defining bool         // are we currently defining a function. takes priority over structmode.
    var definitionName string // ... if we are, what is it called

    parser.pc = -1 // program counter : increments to zero at start of loop

    var si bool
    var we ExpressionCarton // pre-allocated for general expression results eval
    var expr any            // pre-allocated for wrapped expression results eval
    var err error

    typeInvalid := false // used during struct building for indicating type validity.
    statement := Error

    // debug mode stuff:

    activeDebugContext = parser
    if debugMode && ifs < 3 {
        pf("[#fgreen]Debugger is active. Pausing before startup.[#-]\n")
        debugger.enterDebugger(0, functionspaces[source_base], ident, &mident, &gident)
    }

    // main statement loop:

    for {

        parser.pc += 1

        if debugMode {

            // currentPC=parser.pc

            for {
                debugger.lock.RLock()
                isPaused := debugger.paused
                debugger.lock.RUnlock()

                if !isPaused {
                    break
                }
                time.Sleep(10 * time.Millisecond)
            }

            debugger.lock.Lock()
            key := (uint64(ifs) << 32) | uint64(parser.pc)
            cond, hasBP := debugger.breakpoints[key]
            debugger.lock.Unlock()

            if hasBP {
                if cond == "" {
                    debugger.enterDebugger(key, functionspaces[source_base], ident, &mident, &gident)
                } else {
                    result, err := ev(parser, ifs, cond)
                    if err != nil {
                        pf("[#fred]Error evaluating breakpoint condition: %v[#-]\n", err)
                        debugger.enterDebugger(key, functionspaces[source_base], ident, &mident, &gident)
                    } else if isTruthy(result) {
                        debugger.enterDebugger(key, functionspaces[source_base], ident, &mident, &gident)
                    }
                }
            }

            if debugger.stepMode {
                debugger.enterDebugger(key, functionspaces[source_base], ident, &mident, &gident)
            }

            if debugger.nextMode && len(errorChain) <= debugger.nextCallDepth {
                debugger.enterDebugger(key, functionspaces[source_base], ident, &mident, &gident)
            }
        }

        // @note: sig_int can be a race condition. alternatives?
        // if sig_int removed from below then user ctrl-c handler cannot
        // return a custom error code. also, having this cond check every
        // iteration slows down execution.

        if parser.pc >= finalline || endFunc || sig_int {
            break
        }

        // get the next Phrase
        inbound = &functionspaces[source_base][parser.pc]

        // Set the parser's line field to the current source line
        parser.line = inbound.SourceLine

        // Note: Line numbers are now captured at call time via callLine field
        // This ensures stack traces show where functions were called from

    ondo_reenter: // on..do re-enters here because it creates the new phrase in advance and
        //  we want to leave the program counter unaffected.

        statement = inbound.Tokens[0].tokType

        // finally... start processing the statement.

        /////// LINE DEBUG //////////////////////////////////////////////////////
        if lineDebug {
            clr := "2"
            if defining || statement == C_Define {
                clr = "4"
            }
            pf("[#dim][#7]%20s: %5d : [#"+clr+"]%+v[#-]\n", display_fs, inbound.SourceLine+1, basecode[source_base][parser.pc])
        }
        /////////////////////////////////////////////////////////////////////////

        // append statements to a function if currently inside a DEFINE block.
        if defining && statement != C_Enddef {
            lmv, _ := fnlookup.lmget(definitionName)
            fspacelock.Lock()
            functionspaces[lmv] = append(functionspaces[lmv], *inbound)
            basecode_entry = &basecode[source_base][parser.pc]
            basecode[lmv] = append(basecode[lmv], *basecode_entry)
            // although we have added all the tokens in to the new source_base,
            // we still have to add identifier bindings in the new source_base
            // for the replicated inbound lines.
            for _, itok := range inbound.Tokens {
                if itok.tokType == Identifier {
                    itok.bindpos = bind_int(lmv, itok.tokText)
                    itok.bound = true
                }
            }
            fspacelock.Unlock()
            continue
        }

        // struct building
        if structMode && statement != C_Endstruct {

            if statement != C_Define && statement != C_Enddef {

                // consume the statement as an identifier
                // as we are only accepting simple types currently, restrict validity
                //  to single type token.
                if inbound.TokenCount < 2 {
                    parser.report(inbound.SourceLine, sf("Invalid STRUCT entry '%v'", inbound.Tokens[0].tokText))
                    finish(false, ERR_SYNTAX)
                    break
                }

                // check for default value assignment:
                var eqPos int16
                var hasValue bool
                for eqPos = 2; eqPos < inbound.TokenCount; eqPos += 1 {
                    if inbound.Tokens[eqPos].tokType == O_Assign {
                        hasValue = true
                        break
                    }
                }

                var default_value ExpressionCarton
                if hasValue {
                    default_value = parser.wrappedEval(ifs, ident, ifs, ident, inbound.Tokens[eqPos+1:])
                    if default_value.evalError {
                        parser.report(inbound.SourceLine, sf("Invalid default value in STRUCT '%s'", inbound.Tokens[0].tokText))
                        finish(false, ERR_SYNTAX)
                        break
                    }
                }

                var cet ExpressionCarton
                if hasValue {
                    cet = crushEvalTokens(inbound.Tokens[1:eqPos])
                } else {
                    cet = crushEvalTokens(inbound.Tokens[1:])
                }

                // check for valid types:
                typeText := str.ToLower(cet.text)
                isValidType := false

                // Check for basic types
                switch typeText {
                case "int", "float", "string", "bool", "uint", "uint8", "bigi", "bigf", "byte", "mixed", "any", "[]":
                    isValidType = true
                case "[]int", "[]float", "[]string", "[]bool", "[]uint", "[]uint8", "[]bigi", "[]bigf", "[]byte":
                    isValidType = true
                case "int8", "int16", "int64", "uint16", "uint64", "double", "char":
                    isValidType = true
                default:
                    // Check for fixed-size array syntax: type[size]
                    if str.Contains(typeText, "[") && str.HasSuffix(typeText, "]") {
                        openBracket := str.Index(typeText, "[")
                        closeBracket := str.LastIndex(typeText, "]")
                        if openBracket > 0 && closeBracket > openBracket {
                            elemType := str.TrimSpace(typeText[:openBracket])
                            sizeStr := str.TrimSpace(typeText[openBracket+1 : closeBracket])

                            // Validate element type
                            switch elemType {
                            case "int", "uint", "int8", "uint8", "int16", "uint16", "int64", "uint64", "float", "double", "byte", "char":
                                // Validate size is a number
                                if _, err := strconv.Atoi(sizeStr); err == nil {
                                    isValidType = true
                                }
                            }
                        }
                    }
                }

                if !isValidType {
                    parser.report(inbound.SourceLine, sf("Invalid type in STRUCT '%s'", cet.text))
                    finish(false, ERR_SYNTAX)
                    typeInvalid = true
                    break
                }

                if typeInvalid {
                    break
                }

                structNode = append(structNode, renameSF(inbound.Tokens[0].tokText), cet.text, hasValue, default_value.result)
                // pf("current struct node build at :\n%#v\n",structNode)

                continue
            }
        }

        // show var references for -V arg
        if var_refs {
            switch statement {
            case C_Module, C_Define, C_Enddef:
            default:
                continue
            }
        }

        // abort this phrase if currently inside a TEST block but the test flag is not set.
        /*
         * these kind of tests really slow down interpretation.
         * just removing the stanza below can add ~ 9M ops/sec
         */
        if inside_test {
            if statement != C_Endtest && !under_test {
                continue
            }
        }

        ////////////////////////////////////////////////////////////////
        // BREAK here if required

        // a break effectively examines the construct end token type, e.g.
        // C_Endfor, C_Endwhile and if the current statement doesn't match
        // then keeps on looping until it hits the right type.
        // we should maybe have it do a lookahead instead and a direct
        // jump, but just haven't got around to that yet.
        // it would mean we could probably remove the stanza below and some
        // code further in (in the C_End* types) as well as speed up break/continues.

        // breakIn holds either Error or a token_type for ending the current construct
        if breakIn != Error {
            if (breakIn == C_For || breakIn == C_Foreach) && statement != C_Endfor {
                continue
            }
            if breakIn == C_While && statement != C_Endwhile {
                continue
            }
            if breakIn == C_Case && statement != C_Endcase {
                continue
            }
        }
        ////////////////////////////////////////////////////////////////

        // main parsing for statements starts here:

        switch statement {

        case C_Var: // permit declaration with a default value

            //   'VAR' name [ ',' ... nameX ] [ '[' [size] ']' ] type [ '=' expr ]
            // | 'VAR' name struct_name
            // | 'VAR' aryname []struct_name

            var name_list []string
            var name_pos []uint64
            var expectingComma bool
            var varSyntaxError bool
            var c int16

        var_comma_loop:
            for c = int16(1); c < inbound.TokenCount; c += 1 {
                switch inbound.Tokens[c].tokType {
                case Identifier:
                    if expectingComma { // syntax error
                        break var_comma_loop
                    }
                    inter := interpolate(currentModule, ifs, ident, inbound.Tokens[c].tokText)
                    name_list = append(name_list, inter)
                    name_pos = append(name_pos, uint64(c))
                    // pf("nl : %s , np : %d\n",inter,c)
                case O_Comma:
                    if !expectingComma {
                        varSyntaxError = true
                        break var_comma_loop
                    }
                default:
                    break var_comma_loop
                }
                expectingComma = !expectingComma
            }

            if len(name_list) == 0 {
                varSyntaxError = true
            }

            // set eqpos to either location of first equals sign
            // or zero, as well as bool to indicate success
            var eqPos int16
            var hasEqu bool
            for eqPos = c; eqPos < inbound.TokenCount; eqPos += 1 {
                if inbound.Tokens[eqPos].tokType == O_Assign {
                    hasEqu = true
                    break
                }
            }
            // eqPos remains as last token index on natural loop exit

            // look for ary setup or namespaced struct name

            var hasAry bool
            var size int
            found_namespace := ""

            if !varSyntaxError {
                // continue from last 'c' value

                // namespace check
                for dcpos := c; dcpos < eqPos; dcpos += 1 {
                    if inbound.Tokens[dcpos].tokType == SYM_DoubleColon {
                        found_namespace = inbound.Tokens[dcpos-1].tokText
                        break
                    }
                }

                if found_namespace == "" {
                    found_namespace = parser.namespace
                    if c+1 < inbound.TokenCount {
                        if found := uc_match_func(inbound.Tokens[c+1].tokText); found != "" {
                            found_namespace = found
                        }
                    }
                }

                // Validate bracket sequence before processing
                bracketDepth := 0
                for i := c; i < eqPos; i++ {
                    if inbound.Tokens[i].tokType == LeftSBrace {
                        bracketDepth++
                    } else if inbound.Tokens[i].tokType == RightSBrace {
                        bracketDepth--
                        if bracketDepth < 0 {
                            parser.report(inbound.SourceLine, "malformed bracket sequence: ']' found before matching '['")
                            finish(false, ERR_SYNTAX)
                            break
                        }
                    }
                }
                if bracketDepth > 0 {
                    parser.report(inbound.SourceLine, "malformed bracket sequence: unclosed '[' brackets")
                    finish(false, ERR_SYNTAX)
                }

                if inbound.Tokens[c].tokType == LeftSBrace {

                    // find RightSBrace - handle multiple [] pairs for multi-dimensional arrays
                    var d int16
                    bracketPairs := 0
                    for i := c; i < eqPos; i++ {
                        if inbound.Tokens[i].tokType == LeftSBrace {
                            // Find matching RightSBrace
                            bracketCount := 1
                            j := i + 1
                            for j < eqPos && bracketCount > 0 {
                                if inbound.Tokens[j].tokType == LeftSBrace {
                                    bracketCount++
                                } else if inbound.Tokens[j].tokType == RightSBrace {
                                    bracketCount--
                                }
                                j++
                            }
                            if bracketCount == 0 {
                                // Found a complete [] pair
                                bracketPairs++
                                if bracketPairs == 1 {
                                    // Handle size for the first [] pair only
                                    d = j - 1 // j-1 is the RightSBrace position
                                    hasAry = true
                                    if d > (i + 1) {
                                        // not an empty [] term, but includes a size expression
                                        se := parser.wrappedEval(ifs, ident, ifs, ident, inbound.Tokens[i+1:d])
                                        if se.evalError {
                                            parser.report(inbound.SourceLine, "could not evaluate size expression in VAR")
                                            finish(false, ERR_EVAL)
                                            break
                                        }
                                        switch se.result.(type) {
                                        case int:
                                            size = se.result.(int)
                                        case int64:
                                            size = int(se.result.(int64))
                                        case uint:
                                            size = int(se.result.(uint))
                                        case uint64:
                                            size = int(se.result.(uint64))
                                        default:
                                            parser.report(inbound.SourceLine, "size expression must evaluate to an integer")
                                            finish(false, ERR_EVAL)
                                            break
                                        }
                                    }
                                }
                                i = j - 1 // Continue after this bracket pair
                            } else {
                                break // Incomplete bracket pair
                            }
                        }
                    }
                }

            } else {
                parser.report(inbound.SourceLine, "invalid VAR syntax\nUsage: VAR varname1 [#i1][,...varnameX][#i0] [#i1][optional_size][#i0] type [#i1][=expression][#i0]")
                finish(false, ERR_SYNTAX)
            }

            if varSyntaxError {
                break
            }

            // eval the terms to assign to new vars
            hasValue := false
            if hasEqu {
                hasValue = true
                we = parser.wrappedEval(ifs, ident, ifs, ident, inbound.Tokens[eqPos+1:])
                if we.evalError {
                    parser.report(inbound.SourceLine, "could not evaluate VAR assignment expression")
                    finish(false, ERR_EVAL)
                    break
                }
            }

            // name iterations

            for nlp, vname := range name_list {

                var sid uint64
                if strcmp(vname, inbound.Tokens[name_pos[nlp]].tokText) { // no interpol done:
                    sid = inbound.Tokens[name_pos[nlp]].bindpos
                } else {
                    sid = bind_int(ifs, vname)
                }

                // resize ident if required:
                if sid >= uint64(len(*ident)) {
                    newIdent := make([]Variable, sid+identGrowthSize)
                    copy(newIdent, *ident)
                    *ident = newIdent
                }

                // Check for variable redeclaration
                if sid < uint64(len(*ident)) && (*ident)[sid].declared {
                    parser.report(inbound.SourceLine, sf("variable '%s' is already declared", vname))
                    finish(false, ERR_SYNTAX)
                    break
                }

                // Check if this is an output parameter (var name mut)
                var isOutParam bool
                for i := eqPos - 1; i >= c; i-- {
                    if inbound.Tokens[i].tokType == O_Mut {
                        isOutParam = true
                        break
                    }
                }

                // Handle output parameters first - skip normal type processing
                if isOutParam {
                    t := Variable{}
                    t.IName = vname
                    t.IKind = koutparam
                    t.IValue = nil
                    t.ITyped = false
                    t.declared = true
                    (*ident)[sid] = t
                    continue // Skip to next variable in the name list
                }

                // Build the complete type string from all tokens
                var new_type_token_string string

                // Find the base type - need to look backwards from eqPos to find the type token
                var baseType string
                for i := eqPos - 1; i >= c; i-- {
                    tokType := inbound.Tokens[i].tokType
                    if tokType == T_Map || tokType == T_Int || tokType == T_String || tokType == T_Float ||
                        tokType == T_Bool || tokType == T_Uint || tokType == T_Bigi || tokType == T_Bigf ||
                        tokType == T_Any || tokType == T_Array || tokType == T_Pointer {
                        baseType = inbound.Tokens[i].tokText
                        break
                    }
                    // Also look for identifier tokens that could be struct names
                    if tokType == Identifier {
                        baseType = inbound.Tokens[i].tokText
                        break
                    }
                }

                // Count all bracket pairs from the parsing phase and build the type string
                var bracketSuffix string
                var bracketCount int

                // The bracket detection already counted all [] pairs, so build the bracket notation
                if hasAry {
                    // Count how many [] pairs were detected by looking at the tokens
                    for i := c; i < eqPos; i++ {
                        if inbound.Tokens[i].tokType == LeftSBrace {
                            // Find matching RightSBrace to count complete pairs
                            depth := 1
                            j := i + 1
                            for j < eqPos && depth > 0 {
                                if inbound.Tokens[j].tokType == LeftSBrace {
                                    depth++
                                } else if inbound.Tokens[j].tokType == RightSBrace {
                                    depth--
                                }
                                j++
                            }
                            if depth == 0 {
                                bracketCount++
                                i = j - 1 // Skip past this bracket pair
                            }
                        }
                    }

                    // Build the brackets - for maps they go after, for others they go before
                    for i := 0; i < bracketCount; i++ {
                        bracketSuffix += "[]"
                    }
                }

                // Handle maps specially - brackets go after "map"
                if baseType == "map" {
                    new_type_token_string = baseType + bracketSuffix
                } else {
                    // For arrays/slices, brackets go before the type
                    new_type_token_string = bracketSuffix + baseType
                }

                // declaration and initialisation
                reflectType, found := Typemap[new_type_token_string]
                if !found {
                    // Try dynamic type construction for multi-dimensional arrays/slices/maps
                    reflectType = parseAndConstructType(new_type_token_string)
                    if reflectType != nil {
                        Typemap[new_type_token_string] = reflectType // Cache for future use
                        found = true
                    }
                }

                if found {

                    t := Variable{}

                    if new_type_token_string != "map" {
                        t.IValue = reflect.New(reflectType).Elem().Interface()
                    }

                    t.IName = vname
                    t.ITyped = true
                    t.declared = true

                    // Check if this is a dynamically constructed type (multi-dimensional)
                    // Use the bracketCount from token parsing instead of fragile string matching
                    isDynamic := bracketCount > 1

                    // Handle multi-dimensional maps specially - they're still kmap
                    isMultiDimMap := str.HasPrefix(new_type_token_string, "map[") && str.HasSuffix(new_type_token_string, "]")

                    // Handle multi-dimensional maps (they're all kmap at runtime)
                    if isMultiDimMap {
                        t.IKind = kmap
                        t.IValue = make(map[string]any, size)
                        gob.Register(t.IValue)
                    } else if !isDynamic {
                        switch new_type_token_string {
                        case "nil":
                            t.IKind = knil
                        case "bool":
                            t.IKind = kbool
                        case "int":
                            t.IKind = kint
                        case "uint":
                            t.IKind = kuint
                        case "float":
                            t.IKind = kfloat
                        case "string":
                            t.IKind = kstring
                        case "uint8", "byte":
                            t.IKind = kbyte
                        case "uint64", "uxlong":
                            t.IKind = kuint64
                        case "mixed":
                            t.IKind = kany
                        case "any":
                            t.IKind = kany
                        case "pointer":
                            t.IKind = kpointer
                        case "[]bool":
                            t.IKind = ksbool
                            t.IValue = make([]bool, size, size)
                        case "[]int":
                            t.IKind = ksint
                            t.IValue = make([]int, size, size)
                        case "[]uint":
                            t.IKind = ksuint
                            t.IValue = make([]uint, size, size)
                        case "[]int64":
                            t.IKind = ksint64
                            t.IValue = make([]int64, size, size)
                        case "[]uint64", "[]uxlong":
                            t.IKind = ksuint64
                            t.IValue = make([]uint64, size, size)
                        case "[]float":
                            t.IKind = ksfloat
                            t.IValue = make([]float64, size, size)
                        case "[]string":
                            t.IKind = ksstring
                            t.IValue = make([]string, size, size)
                        case "[]byte", "[]uint8":
                            t.IKind = ksbyte
                            t.IValue = make([]uint8, size, size)
                        case "[]", "[]mixed", "[]any", "[]interface {}":
                            t.IKind = ksany
                            t.IValue = make([]any, size, size)
                        case "map":
                            t.IKind = kmap
                            t.IValue = make(map[string]any, size)
                            gob.Register(t.IValue)
                        case "bigi":
                            t.IKind = kbigi
                            t.IValue = big.NewInt(0)
                        case "bigf":
                            t.IKind = kbigf
                            t.IValue = big.NewFloat(0)
                        case "[]bigi":
                            t.IKind = ksbigi
                            t.IValue = make([]*big.Int, size, size)
                        case "[]bigf":
                            t.IKind = ksbigf
                            t.IValue = make([]*big.Float, size, size)
                        }
                    } else {
                        // Dynamic multi-dimensional types use kdynamic
                        t.IKind = kdynamic
                    }

                    // if we had a default value, stuff it in here...

                    if hasValue && new_type_token_string != "map" {

                        // deal with bigs first:
                        var tmp any

                        if t.IKind == kbigi || t.IKind == kbigf {
                            switch t.IKind {
                            case kbigi:
                                tmp = GetAsBigInt(we.result)
                            case kbigf:
                                tmp = GetAsBigFloat(we.result)
                            }
                            switch tmp := tmp.(type) {
                            case *big.Int, *big.Float:
                                t.IValue = tmp
                            default:
                                parser.report(inbound.SourceLine, sf("type mismatch in VAR assignment (need a big, got %T)", tmp))
                                finish(false, ERR_EVAL)
                            }
                        } else {
                            // ... then other types:
                            new_type_token_string = str.Replace(new_type_token_string, "float", "float64", -1)
                            new_type_token_string = str.Replace(new_type_token_string, "any", "interface {}", -1)
                            if sf("%T", we.result) != new_type_token_string {
                                parser.report(inbound.SourceLine, sf("type mismatch in VAR assignment (need %s, got %T)", new_type_token_string, we.result))
                                finish(false, ERR_EVAL)
                                break
                            } else {
                                t.IValue = we.result
                            }
                        }
                    }

                    // Register dynamically constructed types with gob for serialization
                    if reflectType != nil && (str.Contains(new_type_token_string, "[][]") || str.Contains(new_type_token_string, "[") && str.Contains(new_type_token_string, "]")) {
                        gob.Register(t.IValue)
                    }

                    // write temp to ident
                    (*ident)[sid] = t
                    // pf("wrote var: %#v\n... with sid of #%d\n",t,sid)

                } else {
                    // unknown type: check if it is a struct name

                    isStruct := false
                    structvalues := []any{}

                    // handle namespace presence
                    checkstr := new_type_token_string
                    sname := found_namespace + "::" + checkstr
                    cpos := str.IndexByte(checkstr, ':')
                    if cpos != -1 {
                        if len(checkstr) > cpos+1 {
                            if checkstr[cpos+1] == ':' {
                                sname = checkstr
                            }
                        }
                    }

                    // structmap has list of field_name,field_type,... for each struct
                    // structvalues: [0] name [1] type [2] boolhasdefault [3] default_value

                    // First try to resolve through use_chain
                    resolvedName := uc_match_struct(sname)
                    lookupName := sname
                    if resolvedName != "" {
                        lookupName = resolvedName + "::" + sname
                    }

                    structmapslock.RLock()
                    if vals, found := structmaps[lookupName]; found {
                        isStruct = true
                        structvalues = vals
                    } else if vals, found := structmaps[sname]; found {
                        // Fallback: try exact lookup (already qualified names)
                        isStruct = true
                        structvalues = vals
                    } else {
                        // Fallback: loop through structmaps for a match (handles remaining edge cases)
                        for sn, _ := range structmaps {
                            if sn == sname {
                                isStruct = true
                                structvalues = structmaps[sn]
                                break
                            }
                        }
                    }
                    structmapslock.RUnlock()

                    // For C library types, check ffiStructDefinitions as fallback
                    // In VAR context, we want to use struct types even if a function shares the name
                    // (e.g., c::stat is both a function and a struct in libc)
                    if !isStruct && lookupName != "" {
                        ffiStructLock.RLock()
                        if structDef, exists := ffiStructDefinitions[lookupName]; exists && !structDef.IsUnion {
                            // Found FFI struct - register it in structmaps now for future use
                            ffiStructLock.RUnlock()
                            parts := str.SplitN(lookupName, "::", 2)
                            if len(parts) == 2 {
                                namespace := parts[0]
                                structName := parts[1]
                                registerStructInZa(namespace, structName, structDef)
                                // Now retrieve from structmaps
                                structmapslock.RLock()
                                if vals, found := structmaps[lookupName]; found {
                                    isStruct = true
                                    structvalues = vals
                                }
                                structmapslock.RUnlock()
                            }
                        } else {
                            ffiStructLock.RUnlock()
                        }
                    }

                    if isStruct {
                        t := (*ident)[sid]
                        err = fillStruct(&t, structvalues, Typemap, hasAry, []string{})
                        if err != nil {
                            parser.report(inbound.SourceLine, err.Error())
                            finish(false, ERR_EVAL)
                            break
                        }
                        t.IName = vname
                        t.ITyped = false
                        t.declared = true
                        t.Kind_override = sname
                        (*ident)[sid] = t

                    } else {
                        parser.report(inbound.SourceLine, sf("unknown data type requested '%v'", sname))
                        finish(false, ERR_SYNTAX)
                        break
                    }

                } // end-type-or-struct

            } // end-of-name-list

        case C_Use:

            switch inbound.TokenCount {
            case 1:
                uc_show()
            case 2:
                arg := inbound.Tokens[1]
                switch arg.tokType {
                case O_Minus:
                    uc_reset()
                case Identifier:
                    switch str.ToLower(arg.tokText) {
                    case "push":
                        ucs_push()
                    case "pop":
                        if ucs_pop() == false {
                            parser.report(inbound.SourceLine, sf("Cannot pop an empty stack in USE command."))
                        }
                    default:
                        parser.report(inbound.SourceLine, sf("Unknown argument in USE (%s).", arg.tokText))
                        finish(false, ERR_SYNTAX)
                    }
                default:
                    parser.report(inbound.SourceLine, sf("Unknown argument in USE (%s).", arg.tokText))
                    finish(false, ERR_SYNTAX)
                }
            case 3:
                arg1 := inbound.Tokens[1]
                arg2 := inbound.Tokens[2]
                switch arg1.tokType {
                case O_Minus:
                    uc_remove(arg2.tokText)
                case O_Plus:
                    uc_add(arg2.tokText)
                case SYM_Caret:
                    uc_top(arg2.tokText)
                default:
                    parser.report(inbound.SourceLine, sf("Unknown argument in USE (%s).", arg1.tokText))
                    finish(false, ERR_SYNTAX)
                }
            default:
                parser.report(inbound.SourceLine, sf("USE keyword has invalid arguments."))
                finish(false, ERR_SYNTAX)
            }

        // @note: use this at your own risk... (experimental)
        case C_Namespace:
            switch inbound.TokenCount {
            case 2:
                ns := inbound.Tokens[1].tokText
                parser.namespace = ns
                interparse.namespace = ns
                currentModule = ns
            default:
                parser.report(inbound.SourceLine, sf("NAMESPACE needs a single argument."))
                finish(false, ERR_SYNTAX)
            }

        case C_While:

            var endfound bool
            var enddistance int16

            endfound, enddistance, _ = lookahead(source_base, parser.pc, 0, 0, C_Endwhile, []int64{C_While}, []int64{C_Endwhile})
            if !endfound {
                parser.report(inbound.SourceLine, "could not find an ENDWHILE")
                finish(false, ERR_SYNTAX)
                break
            }

            // if cond false, then jump to end while
            // if true, stack the cond then continue

            // eval

            var res bool
            var etoks []Token

            if inbound.TokenCount == 1 {
                etoks = []Token{Token{tokType: Identifier, tokText: "true", subtype: subtypeConst, tokVal: true}}
                res = true
            } else {

                etoks = inbound.Tokens[1:]
                we = parser.wrappedEval(ifs, ident, ifs, ident, etoks)
                if we.evalError {
                    parser.report(inbound.SourceLine, "could not evaluate WHILE condition")
                    finish(false, ERR_EVAL)
                    break
                }

                switch we.result.(type) {
                case bool:
                    res = we.result.(bool)
                default:
                    parser.report(inbound.SourceLine, "WHILE condition must evaluate to boolean")
                    finish(false, ERR_EVAL)
                    break
                }

            }

            if isBool(res) && res {
                // while cond is true, stack, then continue loop
                depth += 1
                loops[depth] = s_loop{repeatFrom: parser.pc, whileContinueAt: parser.pc + enddistance, repeatCond: etoks, loopType: LT_WHILE}
                lastConstruct = append(lastConstruct, C_While)
                break
            } else {
                // -> endwhile
                parser.pc += enddistance
            }

        case C_Endwhile:

            // re-evaluate, on true jump back to start, on false, destack and continue

            cond := loops[depth]

            if !forceEnd && cond.loopType != LT_WHILE {
                parser.report(inbound.SourceLine, "ENDWHILE outside of WHILE loop")
                finish(false, ERR_SYNTAX)
                break
            }

            // time to die?
            if breakIn == C_While {
                depth -= 1
                lastConstruct = lastConstruct[:depth]
                breakIn = Error
                forceEnd = false
                break_count -= 1
                if break_count > 0 {
                    switch lastConstruct[depth-1] {
                    case C_For, C_Foreach, C_While, C_Case:
                        breakIn = lastConstruct[depth-1]
                    }
                }
                // pf("ENDWHILE-BREAK: bc %d\n",break_count)
                break
            }

            // evaluate condition
            we = parser.wrappedEval(ifs, ident, ifs, ident, cond.repeatCond)
            if we.evalError {
                parser.report(inbound.SourceLine, sf("eval fault in ENDWHILE\n%+v\n", we.errVal))
                finish(false, ERR_EVAL)
                break
            }

            if we.result.(bool) {
                // while still true, loop
                parser.pc = cond.repeatFrom
            } else {
                // was false, so leave the loop
                depth -= 1
                lastConstruct = lastConstruct[:depth]
            }

        case C_SetGlob: // set the value of a global variable.

            if inbound.TokenCount < 3 {
                parser.report(inbound.SourceLine, "missing value in setglob.")
                finish(false, ERR_SYNTAX)
                break
            }

            // fmt.Printf("(sg) in fs %d (mident->%d) eval -> %+v\n",ifs,parser.mident,inbound.Tokens[1:])
            atomic.StoreUint32(&has_global_lock, ifs)
            sglock.Lock()
            if res := parser.wrappedEval(parser.mident, &mident, ifs, ident, inbound.Tokens[1:]); res.evalError {
                parser.report(inbound.SourceLine, sf("Error in SETGLOB evaluation\n%+v\n", res.errVal))
                atomic.StoreUint32(&has_global_lock, 0)
                sglock.Unlock()
                finish(false, ERR_EVAL)
                break
            }
            sglock.Unlock()
            atomic.StoreUint32(&has_global_lock, 0)

        case C_Foreach:

            // FOREACH var [ : type ] IN expr
            // iterates over the result of expression expr as a list

            if inbound.TokenCount < 4 {
                parser.report(inbound.SourceLine, "bad argument count in FOREACH.")
                finish(false, ERR_SYNTAX)
                break
            }

            skip := 0
            it_type := ""
            if inbound.Tokens[2].tokType == SYM_COLON {
                it_type = inbound.Tokens[3].tokText
                skip = 2
                // valid type?
                // check if it_type is a key in either Typemap or structmaps
                otype := it_type
                if !str.Contains(it_type, "::") {
                    it_type = parser.namespace + "::" + it_type
                }

                found := false
                if _, found = structmaps[it_type]; !found {
                    _, found = Typemap[otype]
                }
                if !found {
                    parser.report(inbound.SourceLine, sf("invalid type [%s] for iterator in FOREACH.", otype))
                    finish(false, ERR_SYNTAX)
                    break
                }
            }

            if inbound.Tokens[2+skip].tokType != C_In {
                parser.report(inbound.SourceLine, "malformed FOREACH statement.")
                finish(false, ERR_SYNTAX)
                break
            }

            if inbound.Tokens[1].tokType != Identifier {
                parser.report(inbound.SourceLine, "parameter 2 must be an identifier.")
                finish(false, ERR_SYNTAX)
                break
            }

            var condEndPos int

            fid := inbound.Tokens[1].tokText

            switch inbound.Tokens[3+skip].tokType {

            // cause evaluation of all terms following IN
            case SYM_BOR, O_InFile, ResultBlock, Block, NumericLiteral, StringLiteral, LeftSBrace, LParen, Identifier:

                we = parser.wrappedEval(ifs, ident, ifs, ident, inbound.Tokens[3+skip:])
                if we.evalError {
                    parser.report(inbound.SourceLine, sf("error evaluating term in FOREACH statement '%v'\n%+v\n", we.text, we.errVal))
                    finish(false, ERR_EVAL)
                    break
                }

                // ensure result block has content:
                switch we.result.(type) {
                case struct {
                    Out  string
                    Err  string
                    Code int
                    Okay bool
                }:
                    // cast cmd results as their stdout string in loops
                    we.result = we.result.(struct {
                        Out  string
                        Err  string
                        Code int
                        Okay bool
                    }).Out
                case string:
                default:
                    if inbound.Tokens[3+skip].tokType == ResultBlock {
                        parser.report(inbound.SourceLine, "system command did not return a string in FOREACH statement\n")
                        finish(false, ERR_EVAL)
                        break
                    }
                }

                var l int
                switch lv := we.result.(type) {
                case string:
                    l = len(lv)
                case []string:
                    l = len(lv)
                case []uint:
                    l = len(lv)
                case []int:
                    l = len(lv)
                case []float64:
                    l = len(lv)
                case []bool:
                    l = len(lv)
                case []tui:
                    l = len(lv)
                case []stackFrame:
                    l = len(lv)
                case []*big.Int:
                    l = len(lv)
                case []*big.Float:
                    l = len(lv)
                case []dirent:
                    l = len(lv)
                case []ProcessInfo:
                    l = len(lv)
                case []SlabInfo:
                    l = len(lv)
                case []SystemResources:
                    l = len(lv)
                case []MemoryInfo:
                    l = len(lv)
                case []CPUInfo:
                    l = len(lv)
                case []NetworkIOStats:
                    l = len(lv)
                case []DiskIOStats:
                    l = len(lv)
                case []ProcessTree:
                    l = len(lv)
                case []ProcessMap:
                    l = len(lv)
                case []ResourceUsage:
                    l = len(lv)
                case []ResourceSnapshot:
                    l = len(lv)
                case []alloc_info:
                    l = len(lv)
                case map[string]alloc_info:
                    l = len(lv)
                case map[string]stackFrame:
                    l = len(lv)
                case map[string]dirent:
                    l = len(lv)
                case map[string]tui:
                    l = len(lv)
                case map[string]string:
                    l = len(lv)
                case map[string]uint:
                    l = len(lv)
                case map[string]int:
                    l = len(lv)
                case map[string]float64:
                    l = len(lv)
                case map[string]bool:
                    l = len(lv)
                case map[string][]string:
                    l = len(lv)
                case map[string][]uint:
                    l = len(lv)
                case map[string][]int:
                    l = len(lv)
                case map[string][]bool:
                    l = len(lv)
                case map[string][]float64:
                    l = len(lv)
                case map[string]ProcessInfo:
                    l = len(lv)
                case map[string]SlabInfo:
                    l = len(lv)
                case map[string]SystemResources:
                    l = len(lv)
                case map[string]MemoryInfo:
                    l = len(lv)
                case map[string]CPUInfo:
                    l = len(lv)
                case map[string]NetworkIOStats:
                    l = len(lv)
                case map[string]DiskIOStats:
                    l = len(lv)
                case map[string]ProcessTree:
                    l = len(lv)
                case map[string]ProcessMap:
                    l = len(lv)
                case map[string]ResourceUsage:
                    l = len(lv)
                case map[string]ResourceSnapshot:
                    l = len(lv)
                case []map[string]any:
                    l = len(lv)
                case map[string]any:
                    l = len(lv)
                case [][]int:
                    l = len(lv)
                case []any:
                    l = len(lv)
                default:
                    pf("Unknown loop type [%T]\n", lv)
                }

                endfound, enddistance, _ := lookahead(source_base, parser.pc, 0, 0, C_Endfor, []int64{C_For, C_Foreach}, []int64{C_Endfor})
                if !endfound {
                    parser.report(inbound.SourceLine, "Cannot determine the location of a matching ENDFOR.")
                    finish(false, ERR_SYNTAX)
                    break
                }

                // skip empty expressions
                if l == 0 {
                    parser.pc += enddistance
                    break
                }

                var iter *reflect.MapIter

                switch we.result.(type) {

                case string:

                    // split and treat as array if multi-line

                    // remove a single trailing \n from string
                    elast := len(we.result.(string)) - 1
                    if we.result.(string)[elast] == '\n' {
                        we.result = we.result.(string)[:elast]
                    }

                    // split up string at \n divisions into an array
                    if runtime.GOOS != "windows" {
                        we.result = str.Split(we.result.(string), "\n")
                    } else {
                        we.result = str.Split(str.Replace(we.result.(string), "\r\n", "\n", -1), "\n")
                    }

                    if len(we.result.([]string)) > 0 {
                        vset(nil, ifs, ident, "key_"+fid, 0)
                        vset(&inbound.Tokens[1], ifs, ident, fid, we.result.([]string)[0])
                        condEndPos = len(we.result.([]string)) - 1
                    }

                case map[string]float64:
                    if len(we.result.(map[string]float64)) > 0 {
                        // get iterator for this map
                        iter = reflect.ValueOf(we.result.(map[string]float64)).MapRange()

                        // set initial key and value
                        if iter.Next() {
                            vset(nil, ifs, ident, "key_"+fid, iter.Key().String())
                            vset(&inbound.Tokens[1], ifs, ident, fid, iter.Value().Interface())
                        }
                        condEndPos = len(we.result.(map[string]float64)) - 1
                    }

                case map[string]tui:
                    if len(we.result.(map[string]tui)) > 0 {
                        iter = reflect.ValueOf(we.result.(map[string]tui)).MapRange()
                        if iter.Next() {
                            vset(nil, ifs, ident, "key_"+fid, iter.Key().String())
                            vset(&inbound.Tokens[1], ifs, ident, fid, iter.Value().Interface())
                        }
                        condEndPos = len(we.result.(map[string]tui)) - 1
                    }

                case map[string]stackFrame:
                    if len(we.result.(map[string]stackFrame)) > 0 {
                        iter = reflect.ValueOf(we.result.(map[string]stackFrame)).MapRange()
                        if iter.Next() {
                            vset(nil, ifs, ident, "key_"+fid, iter.Key().String())
                            vset(&inbound.Tokens[1], ifs, ident, fid, iter.Value().Interface())
                        }
                        condEndPos = len(we.result.(map[string]stackFrame)) - 1
                    }

                case map[string]alloc_info:
                    if len(we.result.(map[string]alloc_info)) > 0 {
                        iter = reflect.ValueOf(we.result.(map[string]alloc_info)).MapRange()
                        if iter.Next() {
                            vset(nil, ifs, ident, "key_"+fid, iter.Key().String())
                            vset(&inbound.Tokens[1], ifs, ident, fid, iter.Value().Interface())
                        }
                        condEndPos = len(we.result.(map[string]alloc_info)) - 1
                    }

                case map[string]dirent:
                    if len(we.result.(map[string]dirent)) > 0 {
                        iter = reflect.ValueOf(we.result.(map[string]dirent)).MapRange()
                        if iter.Next() {
                            vset(nil, ifs, ident, "key_"+fid, iter.Key().String())
                            vset(&inbound.Tokens[1], ifs, ident, fid, iter.Value().Interface())
                        }
                        condEndPos = len(we.result.(map[string]dirent)) - 1
                    }

                case map[string]bool:
                    if len(we.result.(map[string]bool)) > 0 {
                        iter = reflect.ValueOf(we.result.(map[string]bool)).MapRange()
                        if iter.Next() {
                            vset(nil, ifs, ident, "key_"+fid, iter.Key().String())
                            vset(&inbound.Tokens[1], ifs, ident, fid, iter.Value().Interface())
                        }
                        condEndPos = len(we.result.(map[string]bool)) - 1
                    }

                case map[string]uint:
                    if len(we.result.(map[string]uint)) > 0 {
                        iter = reflect.ValueOf(we.result.(map[string]uint)).MapRange()
                        if iter.Next() {
                            vset(nil, ifs, ident, "key_"+fid, iter.Key().String())
                            vset(&inbound.Tokens[1], ifs, ident, fid, iter.Value().Interface())
                        }
                        condEndPos = len(we.result.(map[string]uint)) - 1
                    }

                case map[string]int:
                    if len(we.result.(map[string]int)) > 0 {
                        // get iterator for this map
                        iter = reflect.ValueOf(we.result.(map[string]int)).MapRange()

                        // set initial key and value
                        if iter.Next() {
                            vset(nil, ifs, ident, "key_"+fid, iter.Key().String())
                            vset(&inbound.Tokens[1], ifs, ident, fid, iter.Value().Interface())
                        }
                        condEndPos = len(we.result.(map[string]int)) - 1
                    }

                case map[string]string:

                    if len(we.result.(map[string]string)) > 0 {

                        // get iterator for this map
                        iter = reflect.ValueOf(we.result.(map[string]string)).MapRange()
                        // set initial key and value
                        if iter.Next() {
                            vset(nil, ifs, ident, "key_"+fid, iter.Key().String())
                            vset(&inbound.Tokens[1], ifs, ident, fid, iter.Value().Interface())
                        }
                        condEndPos = len(we.result.(map[string]string)) - 1
                    }

                case map[string][]string:

                    if len(we.result.(map[string][]string)) > 0 {

                        // get iterator for this map
                        iter = reflect.ValueOf(we.result.(map[string][]string)).MapRange()

                        // set initial key and value
                        if iter.Next() {
                            vset(nil, ifs, ident, "key_"+fid, iter.Key().String())
                            vset(&inbound.Tokens[1], ifs, ident, fid, iter.Value().Interface())
                        }
                        condEndPos = len(we.result.(map[string][]string)) - 1
                    }

                case []float64:

                    if len(we.result.([]float64)) > 0 {
                        vset(nil, ifs, ident, "key_"+fid, 0)
                        vset(&inbound.Tokens[1], ifs, ident, fid, we.result.([]float64)[0])
                        condEndPos = len(we.result.([]float64)) - 1
                    }

                case float64: // special case: float
                    we.result = []float64{we.result.(float64)}
                    if len(we.result.([]float64)) > 0 {
                        vset(nil, ifs, ident, "key_"+fid, 0)
                        vset(&inbound.Tokens[1], ifs, ident, fid, we.result.([]float64)[0])
                        condEndPos = len(we.result.([]float64)) - 1
                    }

                case []uint:
                    if len(we.result.([]uint)) > 0 {
                        vset(nil, ifs, ident, "key_"+fid, 0)
                        vset(&inbound.Tokens[1], ifs, ident, fid, we.result.([]uint)[0])
                        condEndPos = len(we.result.([]uint)) - 1
                    }

                case []bool:
                    if len(we.result.([]bool)) > 0 {
                        vset(nil, ifs, ident, "key_"+fid, 0)
                        vset(&inbound.Tokens[1], ifs, ident, fid, we.result.([]bool)[0])
                        condEndPos = len(we.result.([]bool)) - 1
                    }

                case []int:
                    if len(we.result.([]int)) > 0 {
                        vset(nil, ifs, ident, "key_"+fid, 0)
                        vset(&inbound.Tokens[1], ifs, ident, fid, we.result.([]int)[0])
                        condEndPos = len(we.result.([]int)) - 1
                    }

                case []*big.Int:
                    if len(we.result.([]*big.Int)) > 0 {
                        vset(nil, ifs, ident, "key_"+fid, 0)
                        vset(&inbound.Tokens[1], ifs, ident, fid, we.result.([]*big.Int)[0])
                        condEndPos = len(we.result.([]*big.Int)) - 1
                    }

                case []*big.Float:
                    if len(we.result.([]*big.Float)) > 0 {
                        vset(nil, ifs, ident, "key_"+fid, 0)
                        vset(&inbound.Tokens[1], ifs, ident, fid, we.result.([]*big.Float)[0])
                        condEndPos = len(we.result.([]*big.Float)) - 1
                    }

                case int: // special case: int
                    we.result = []int{we.result.(int)}
                    if len(we.result.([]int)) > 0 {
                        vset(nil, ifs, ident, "key_"+fid, 0)
                        vset(&inbound.Tokens[1], ifs, ident, fid, we.result.([]int)[0])
                        condEndPos = len(we.result.([]int)) - 1
                    }

                case []string:
                    if len(we.result.([]string)) > 0 {
                        vset(nil, ifs, ident, "key_"+fid, 0)
                        vset(&inbound.Tokens[1], ifs, ident, fid, we.result.([]string)[0])
                        condEndPos = len(we.result.([]string)) - 1
                    }

                case []tui:
                    if len(we.result.([]tui)) > 0 {
                        vset(nil, ifs, ident, "key_"+fid, 0)
                        vset(&inbound.Tokens[1], ifs, ident, fid, we.result.([]tui)[0])
                        condEndPos = len(we.result.([]tui)) - 1
                    }

                case []stackFrame:
                    if len(we.result.([]stackFrame)) > 0 {
                        vset(nil, ifs, ident, "key_"+fid, 0)
                        vset(&inbound.Tokens[1], ifs, ident, fid, we.result.([]stackFrame)[0])
                        condEndPos = len(we.result.([]stackFrame)) - 1
                    }

                case []SlabInfo:
                    if len(we.result.([]SlabInfo)) > 0 {
                        vset(nil, ifs, ident, "key_"+fid, 0)
                        vset(&inbound.Tokens[1], ifs, ident, fid, we.result.([]SlabInfo)[0])
                        condEndPos = len(we.result.([]SlabInfo)) - 1
                    }
                case []ProcessInfo:
                    if len(we.result.([]ProcessInfo)) > 0 {
                        vset(nil, ifs, ident, "key_"+fid, 0)
                        vset(&inbound.Tokens[1], ifs, ident, fid, we.result.([]ProcessInfo)[0])
                        condEndPos = len(we.result.([]ProcessInfo)) - 1
                    }
                case []SystemResources:
                    if len(we.result.([]SystemResources)) > 0 {
                        vset(nil, ifs, ident, "key_"+fid, 0)
                        vset(&inbound.Tokens[1], ifs, ident, fid, we.result.([]SystemResources)[0])
                        condEndPos = len(we.result.([]SystemResources)) - 1
                    }
                case []MemoryInfo:
                    if len(we.result.([]MemoryInfo)) > 0 {
                        vset(nil, ifs, ident, "key_"+fid, 0)
                        vset(&inbound.Tokens[1], ifs, ident, fid, we.result.([]MemoryInfo)[0])
                        condEndPos = len(we.result.([]MemoryInfo)) - 1
                    }
                case []CPUInfo:
                    if len(we.result.([]CPUInfo)) > 0 {
                        vset(nil, ifs, ident, "key_"+fid, 0)
                        vset(&inbound.Tokens[1], ifs, ident, fid, we.result.([]CPUInfo)[0])
                        condEndPos = len(we.result.([]CPUInfo)) - 1
                    }
                case []NetworkIOStats:
                    if len(we.result.([]NetworkIOStats)) > 0 {
                        vset(nil, ifs, ident, "key_"+fid, 0)
                        vset(&inbound.Tokens[1], ifs, ident, fid, we.result.([]NetworkIOStats)[0])
                        condEndPos = len(we.result.([]NetworkIOStats)) - 1
                    }
                case []DiskIOStats:
                    if len(we.result.([]DiskIOStats)) > 0 {
                        vset(nil, ifs, ident, "key_"+fid, 0)
                        vset(&inbound.Tokens[1], ifs, ident, fid, we.result.([]DiskIOStats)[0])
                        condEndPos = len(we.result.([]DiskIOStats)) - 1
                    }
                case []ProcessTree:
                    if len(we.result.([]ProcessTree)) > 0 {
                        vset(nil, ifs, ident, "key_"+fid, 0)
                        vset(&inbound.Tokens[1], ifs, ident, fid, we.result.([]ProcessTree)[0])
                        condEndPos = len(we.result.([]ProcessTree)) - 1
                    }
                case []ProcessMap:
                    if len(we.result.([]ProcessMap)) > 0 {
                        vset(nil, ifs, ident, "key_"+fid, 0)
                        vset(&inbound.Tokens[1], ifs, ident, fid, we.result.([]ProcessMap)[0])
                        condEndPos = len(we.result.([]ProcessMap)) - 1
                    }
                case []ResourceUsage:
                    if len(we.result.([]ResourceUsage)) > 0 {
                        vset(nil, ifs, ident, "key_"+fid, 0)
                        vset(&inbound.Tokens[1], ifs, ident, fid, we.result.([]ResourceUsage)[0])
                        condEndPos = len(we.result.([]ResourceUsage)) - 1
                    }
                case []ResourceSnapshot:
                    if len(we.result.([]ResourceSnapshot)) > 0 {
                        vset(nil, ifs, ident, "key_"+fid, 0)
                        vset(&inbound.Tokens[1], ifs, ident, fid, we.result.([]ResourceSnapshot)[0])
                        condEndPos = len(we.result.([]ResourceSnapshot)) - 1
                    }

                case []dirent:
                    if len(we.result.([]dirent)) > 0 {
                        vset(nil, ifs, ident, "key_"+fid, 0)
                        vset(&inbound.Tokens[1], ifs, ident, fid, we.result.([]dirent)[0])
                        condEndPos = len(we.result.([]dirent)) - 1
                    }

                case []alloc_info:
                    if len(we.result.([]alloc_info)) > 0 {
                        vset(nil, ifs, ident, "key_"+fid, 0)
                        vset(&inbound.Tokens[1], ifs, ident, fid, we.result.([]alloc_info)[0])
                        condEndPos = len(we.result.([]alloc_info)) - 1
                    }

                case [][]int:
                    if len(we.result.([][]int)) > 0 {
                        vset(nil, ifs, ident, "key_"+fid, 0)
                        vset(&inbound.Tokens[1], ifs, ident, fid, we.result.([][]int)[0])
                        condEndPos = len(we.result.([][]int)) - 1
                    }

                case []map[string]any:

                    if len(we.result.([]map[string]any)) > 0 {
                        vset(nil, ifs, ident, "key_"+fid, 0)
                        vset(&inbound.Tokens[1], ifs, ident, fid, we.result.([]map[string]any)[0])
                        condEndPos = len(we.result.([]map[string]any)) - 1
                    }

                case map[string]any:

                    if len(we.result.(map[string]any)) > 0 {

                        // get iterator for this map
                        iter = reflect.ValueOf(we.result.(map[string]any)).MapRange()

                        // set initial key and value
                        if iter.Next() {
                            vset(nil, ifs, ident, "key_"+fid, iter.Key().String())
                            vset(&inbound.Tokens[1], ifs, ident, fid, iter.Value().Interface())
                        }
                        condEndPos = len(we.result.(map[string]any)) - 1
                    }

                case map[string]SlabInfo:
                    if len(we.result.(map[string]SlabInfo)) > 0 {
                        iter = reflect.ValueOf(we.result.(map[string]SlabInfo)).MapRange()
                        if iter.Next() {
                            vset(nil, ifs, ident, "key_"+fid, iter.Key().String())
                            vset(&inbound.Tokens[1], ifs, ident, fid, iter.Value().Interface())
                        }
                        condEndPos = len(we.result.(map[string]SlabInfo)) - 1
                    }
                case map[string]ProcessInfo:
                    if len(we.result.(map[string]ProcessInfo)) > 0 {
                        iter = reflect.ValueOf(we.result.(map[string]ProcessInfo)).MapRange()
                        if iter.Next() {
                            vset(nil, ifs, ident, "key_"+fid, iter.Key().String())
                            vset(&inbound.Tokens[1], ifs, ident, fid, iter.Value().Interface())
                        }
                        condEndPos = len(we.result.(map[string]ProcessInfo)) - 1
                    }
                case map[string]SystemResources:
                    if len(we.result.(map[string]SystemResources)) > 0 {
                        iter = reflect.ValueOf(we.result.(map[string]SystemResources)).MapRange()
                        if iter.Next() {
                            vset(nil, ifs, ident, "key_"+fid, iter.Key().String())
                            vset(&inbound.Tokens[1], ifs, ident, fid, iter.Value().Interface())
                        }
                        condEndPos = len(we.result.(map[string]SystemResources)) - 1
                    }
                case map[string]MemoryInfo:
                    if len(we.result.(map[string]MemoryInfo)) > 0 {
                        iter = reflect.ValueOf(we.result.(map[string]MemoryInfo)).MapRange()
                        if iter.Next() {
                            vset(nil, ifs, ident, "key_"+fid, iter.Key().String())
                            vset(&inbound.Tokens[1], ifs, ident, fid, iter.Value().Interface())
                        }
                        condEndPos = len(we.result.(map[string]MemoryInfo)) - 1
                    }
                case map[string]CPUInfo:
                    if len(we.result.(map[string]CPUInfo)) > 0 {
                        iter = reflect.ValueOf(we.result.(map[string]CPUInfo)).MapRange()
                        if iter.Next() {
                            vset(nil, ifs, ident, "key_"+fid, iter.Key().String())
                            vset(&inbound.Tokens[1], ifs, ident, fid, iter.Value().Interface())
                        }
                        condEndPos = len(we.result.(map[string]CPUInfo)) - 1
                    }
                case map[string]NetworkIOStats:
                    if len(we.result.(map[string]NetworkIOStats)) > 0 {
                        iter = reflect.ValueOf(we.result.(map[string]NetworkIOStats)).MapRange()
                        if iter.Next() {
                            vset(nil, ifs, ident, "key_"+fid, iter.Key().String())
                            vset(&inbound.Tokens[1], ifs, ident, fid, iter.Value().Interface())
                        }
                        condEndPos = len(we.result.(map[string]NetworkIOStats)) - 1
                    }
                case map[string]DiskIOStats:
                    if len(we.result.(map[string]DiskIOStats)) > 0 {
                        iter = reflect.ValueOf(we.result.(map[string]DiskIOStats)).MapRange()
                        if iter.Next() {
                            vset(nil, ifs, ident, "key_"+fid, iter.Key().String())
                            vset(&inbound.Tokens[1], ifs, ident, fid, iter.Value().Interface())
                        }
                        condEndPos = len(we.result.(map[string]DiskIOStats)) - 1
                    }
                case map[string]ProcessTree:
                    if len(we.result.(map[string]ProcessTree)) > 0 {
                        iter = reflect.ValueOf(we.result.(map[string]ProcessTree)).MapRange()
                        if iter.Next() {
                            vset(nil, ifs, ident, "key_"+fid, iter.Key().String())
                            vset(&inbound.Tokens[1], ifs, ident, fid, iter.Value().Interface())
                        }
                        condEndPos = len(we.result.(map[string]ProcessTree)) - 1
                    }
                case map[string]ProcessMap:
                    if len(we.result.(map[string]ProcessMap)) > 0 {
                        iter = reflect.ValueOf(we.result.(map[string]ProcessMap)).MapRange()
                        if iter.Next() {
                            vset(nil, ifs, ident, "key_"+fid, iter.Key().String())
                            vset(&inbound.Tokens[1], ifs, ident, fid, iter.Value().Interface())
                        }
                        condEndPos = len(we.result.(map[string]ProcessMap)) - 1
                    }
                case map[string]ResourceUsage:
                    if len(we.result.(map[string]ResourceUsage)) > 0 {
                        iter = reflect.ValueOf(we.result.(map[string]ResourceUsage)).MapRange()
                        if iter.Next() {
                            vset(nil, ifs, ident, "key_"+fid, iter.Key().String())
                            vset(&inbound.Tokens[1], ifs, ident, fid, iter.Value().Interface())
                        }
                        condEndPos = len(we.result.(map[string]ResourceUsage)) - 1
                    }
                case map[string]ResourceSnapshot:
                    if len(we.result.(map[string]ResourceSnapshot)) > 0 {
                        iter = reflect.ValueOf(we.result.(map[string]ResourceSnapshot)).MapRange()
                        if iter.Next() {
                            vset(nil, ifs, ident, "key_"+fid, iter.Key().String())
                            vset(&inbound.Tokens[1], ifs, ident, fid, iter.Value().Interface())
                        }
                        condEndPos = len(we.result.(map[string]ResourceSnapshot)) - 1
                    }

                case []any:

                    if len(we.result.([]any)) > 0 {
                        vset(nil, ifs, ident, "key_"+fid, 0)
                        vset(&inbound.Tokens[1], ifs, ident, fid, we.result.([]any)[0])

                        bin := bind_int(ifs, fid)
                        if it_type != "" {
                            t := (*ident)[bin]
                            t.ITyped = true
                            t.declared = true
                            t.Kind_override = it_type
                            (*ident)[bin] = t
                        }

                        isStruct := reflect.TypeOf(we.result.([]any)[0]).Kind() == reflect.Struct
                        if isStruct && it_type == "" {
                            if s, count := struct_match(we.result.([]any)[0]); count == 1 {
                                (*ident)[bin].Kind_override = s
                            }
                        }

                        condEndPos = len(we.result.([]any)) - 1
                    }

                default:
                    parser.report(inbound.SourceLine, sf("Mishandled return of type '%T' from FOREACH expression '%v'\n", we.result, we.result))
                    finish(false, ERR_EVAL)
                    break
                }

                depth += 1
                lastConstruct = append(lastConstruct, C_Foreach)

                loops[depth] = s_loop{loopVar: fid, keyVar: "key_" + fid,
                    optNoUse:   Opt_LoopStart,
                    repeatFrom: parser.pc + 1, iterOverMap: iter, iterOverArray: we.result,
                    counter: 0, condEnd: condEndPos, forEndPos: enddistance + parser.pc,
                    loopType: LT_FOREACH, itType: it_type,
                }

            default:
                parser.report(inbound.SourceLine, "Unexpected expression type in FOREACH.")
                finish(false, ERR_SYNTAX)
                break

            }

        case C_For: // loop over an int64 range

            var iterAssignment []Token
            var iterCondition []Token
            var iterAmendment []Token
            customCond := false

            // check for custom FOR setup
            // e.g. for x=0,x<10,x+=1

            commaList := parser.splitCommaArray(inbound.Tokens[1:])
            if len(commaList) == 3 {
                iterAssignment = commaList[0]
                iterCondition = commaList[1]
                iterAmendment = commaList[2]
                foundAssign := false
                if len(iterAssignment) > 0 {
                    // has an equals? then do assignment
                    for eqPos := 0; eqPos < len(iterAssignment); eqPos += 1 {
                        if inbound.Tokens[eqPos].tokType == O_Assign {
                            foundAssign = true
                            default_value := parser.wrappedEval(ifs, ident, ifs, ident, iterAssignment)
                            if default_value.evalError {
                                foundAssign = false
                            }
                            break
                        }
                    }
                    if !foundAssign {
                        parser.report(inbound.SourceLine, sf("Invalid assignment in FOR (%+v)", iterAssignment))
                        finish(false, ERR_SYNTAX)
                        break
                    }
                }
                customCond = true

                // figure end position
                endfound, enddistance, _ := lookahead(source_base, parser.pc, 0, 0, C_Endfor, []int64{C_For, C_Foreach}, []int64{C_Endfor})
                if !endfound {
                    parser.report(inbound.SourceLine, "Cannot determine the location of a matching ENDFOR.")
                    finish(false, ERR_SYNTAX)
                    break
                }

                depth += 1
                loops[depth] = s_loop{
                    optNoUse: Opt_LoopStart,
                    loopType: LT_FOR, forEndPos: parser.pc + enddistance, repeatFrom: parser.pc + 1,
                    repeatCond: iterCondition, repeatAmendment: iterAmendment, repeatCustom: true,
                }

                lastConstruct = append(lastConstruct, C_For)

            }

            if !customCond {

                if inbound.TokenCount < 5 || inbound.Tokens[2].tokText != "=" {
                    // not a normal or custom for loop
                    parser.report(inbound.SourceLine, "Malformed FOR statement.")
                    finish(false, ERR_SYNTAX)
                    break
                }

                toAt := findDelim(inbound.Tokens, C_To, 2)
                if toAt == -1 {
                    parser.report(inbound.SourceLine, "TO not found in FOR")
                    finish(false, ERR_SYNTAX)
                    break
                }

                stepAt := findDelim(inbound.Tokens, C_Step, toAt)
                stepped := true
                if stepAt == -1 {
                    stepped = false
                }

                var fstart, fend, fstep int

                var err error

                if toAt > 3 {
                    expr, err = parser.Eval(ifs, inbound.Tokens[3:toAt])
                    if err == nil && isNumber(expr) {
                        fstart, _ = GetAsInt(expr)
                    } else {
                        parser.report(inbound.SourceLine, "Could not evaluate start expression in FOR")
                        finish(false, ERR_EVAL)
                        break
                    }
                } else {
                    parser.report(inbound.SourceLine, "Missing expression in FOR statement?")
                    finish(false, ERR_SYNTAX)
                    break
                }

                if inbound.TokenCount > toAt+1 {
                    if stepAt > 0 {
                        expr, err = parser.Eval(ifs, inbound.Tokens[toAt+1:stepAt])
                    } else {
                        expr, err = parser.Eval(ifs, inbound.Tokens[toAt+1:])
                    }
                    if err == nil && isNumber(expr) {
                        fend, _ = GetAsInt(expr)
                    } else {
                        parser.report(inbound.SourceLine, "Could not evaluate end expression in FOR")
                        finish(false, ERR_EVAL)
                        break
                    }
                } else {
                    parser.report(inbound.SourceLine, "Missing expression in FOR statement?")
                    finish(false, ERR_SYNTAX)
                    break
                }

                if stepped {
                    if inbound.TokenCount > stepAt+1 {
                        expr, err = parser.Eval(ifs, inbound.Tokens[stepAt+1:])
                        if err == nil && isNumber(expr) {
                            fstep, _ = GetAsInt(expr)
                        } else {
                            parser.report(inbound.SourceLine, "Could not evaluate STEP expression")
                            finish(false, ERR_EVAL)
                            break
                        }
                    } else {
                        parser.report(inbound.SourceLine, "Missing expression in FOR statement?")
                        finish(false, ERR_SYNTAX)
                        break
                    }
                }

                step := 1
                if stepped {
                    step = fstep
                }
                if step == 0 {
                    parser.report(inbound.SourceLine, "This is a road to nowhere. (STEP==0)")
                    finish(true, ERR_EVAL)
                    break
                }

                direction := ACT_INC
                if step < 0 {
                    direction = ACT_DEC
                }

                // figure end position
                endfound, enddistance, _ := lookahead(source_base, parser.pc, 0, 0, C_Endfor, []int64{C_For, C_Foreach}, []int64{C_Endfor})
                if !endfound {
                    parser.report(inbound.SourceLine, "Cannot determine the location of a matching ENDFOR.")
                    finish(false, ERR_SYNTAX)
                    break
                }

                // @note: if loop counter is never used between here and C_Endfor, then don't vset the local var

                // store loop data
                fid := inbound.Tokens[1].tokText

                // prepare loop counter binding
                bin := inbound.Tokens[1].bindpos

                depth += 1
                loops[depth] = s_loop{
                    loopVar:        fid,
                    keyVar:         "key_" + fid,
                    loopVarBinding: bin,
                    optNoUse:       Opt_LoopStart,
                    loopType:       LT_FOR, forEndPos: parser.pc + enddistance, repeatFrom: parser.pc + 1,
                    counter: fstart, condEnd: fend,
                    repeatAction: direction, repeatActionStep: step,
                }

                // store loop start condition
                vset(&inbound.Tokens[1], ifs, ident, fid, fstart)

                lastConstruct = append(lastConstruct, C_For)

                // make sure start is not more than end, if it is, send it to the endfor
                switch direction {
                case ACT_INC:
                    if fstart > fend {
                        parser.pc = parser.pc + enddistance - 1
                        break
                    }
                case ACT_DEC:
                    if fstart < fend {
                        parser.pc = parser.pc + enddistance - 1
                        break
                    }
                }

            } // end-not-custom-cond

        case C_Endfor: // terminate a FOR or FOREACH block

            //.. take address of loop info store entry
            thisLoop = &loops[depth]

            if (*thisLoop).optNoUse == Opt_LoopStart {
                if !forceEnd && lastConstruct[depth-1] != C_Foreach && lastConstruct[depth-1] != C_For {
                    parser.report(inbound.SourceLine, "ENDFOR without a FOR or FOREACH")
                    finish(false, ERR_SYNTAX)
                    break
                }
            }

            var loopEnd bool

            // perform cond action and check condition

            if breakIn != C_For && breakIn != C_Foreach {

                switch (*thisLoop).loopType {

                case LT_FOREACH: // move through range

                    (*thisLoop).counter += 1

                    // set only on first iteration, keeps optNoUse consistent with C_For
                    if (*thisLoop).optNoUse == Opt_LoopStart {
                        (*thisLoop).optNoUse = Opt_LoopSet
                    }

                    it_type := (*thisLoop).itType

                    if (*thisLoop).counter > (*thisLoop).condEnd {
                        loopEnd = true
                    } else {

                        // assign value back to local variable

                        switch (*thisLoop).iterOverArray.(type) {

                        // map ranges are randomly ordered!!

                        case map[string]any, map[string]alloc_info, map[string]stackFrame, map[string]tui, map[string]dirent:
                            if (*thisLoop).iterOverMap.Next() { // true means not exhausted
                                vset(nil, ifs, ident, (*thisLoop).keyVar, (*thisLoop).iterOverMap.Key().String())
                                vset(nil, ifs, ident, (*thisLoop).loopVar, (*thisLoop).iterOverMap.Value().Interface())
                            }

                        case map[string]int, map[string]uint, map[string]bool, map[string]float64, map[string]string:
                            if (*thisLoop).iterOverMap.Next() { // true means not exhausted
                                vset(nil, ifs, ident, (*thisLoop).keyVar, (*thisLoop).iterOverMap.Key().String())
                                vset(nil, ifs, ident, (*thisLoop).loopVar, (*thisLoop).iterOverMap.Value().Interface())
                            }

                        case map[string]ProcessInfo, map[string]SystemResources, map[string]MemoryInfo, map[string]CPUInfo:
                            if (*thisLoop).iterOverMap.Next() { // true means not exhausted
                                vset(nil, ifs, ident, (*thisLoop).keyVar, (*thisLoop).iterOverMap.Key().String())
                                vset(nil, ifs, ident, (*thisLoop).loopVar, (*thisLoop).iterOverMap.Value().Interface())
                            }

                        case map[string]NetworkIOStats, map[string]DiskIOStats, map[string]ProcessTree, map[string]ProcessMap:
                            if (*thisLoop).iterOverMap.Next() { // true means not exhausted
                                vset(nil, ifs, ident, (*thisLoop).keyVar, (*thisLoop).iterOverMap.Key().String())
                                vset(nil, ifs, ident, (*thisLoop).loopVar, (*thisLoop).iterOverMap.Value().Interface())
                            }

                        case map[string]ResourceUsage, map[string]ResourceSnapshot, map[string][]string, map[string]SlabInfo:
                            if (*thisLoop).iterOverMap.Next() { // true means not exhausted
                                vset(nil, ifs, ident, (*thisLoop).keyVar, (*thisLoop).iterOverMap.Key().String())
                                vset(nil, ifs, ident, (*thisLoop).loopVar, (*thisLoop).iterOverMap.Value().Interface())
                            }

                        case []bool:
                            vset(nil, ifs, ident, (*thisLoop).keyVar, (*thisLoop).counter)
                            vset(nil, ifs, ident, (*thisLoop).loopVar, (*thisLoop).iterOverArray.([]bool)[(*thisLoop).counter])
                        case []int:
                            vset(nil, ifs, ident, (*thisLoop).keyVar, (*thisLoop).counter)
                            vset(nil, ifs, ident, (*thisLoop).loopVar, (*thisLoop).iterOverArray.([]int)[(*thisLoop).counter])
                        case []uint:
                            vset(nil, ifs, ident, (*thisLoop).keyVar, (*thisLoop).counter)
                            vset(nil, ifs, ident, (*thisLoop).loopVar, (*thisLoop).iterOverArray.([]uint8)[(*thisLoop).counter])
                        case []uint64:
                            vset(nil, ifs, ident, (*thisLoop).keyVar, (*thisLoop).counter)
                            vset(nil, ifs, ident, (*thisLoop).loopVar, (*thisLoop).iterOverArray.([]uint64)[(*thisLoop).counter])
                        case []string:
                            vset(nil, ifs, ident, (*thisLoop).keyVar, (*thisLoop).counter)
                            vset(nil, ifs, ident, (*thisLoop).loopVar, (*thisLoop).iterOverArray.([]string)[(*thisLoop).counter])

                        case []SlabInfo:
                            vset(nil, ifs, ident, (*thisLoop).keyVar, (*thisLoop).counter)
                            vset(nil, ifs, ident, (*thisLoop).loopVar, (*thisLoop).iterOverArray.([]SlabInfo)[(*thisLoop).counter])
                        case []ProcessInfo:
                            vset(nil, ifs, ident, (*thisLoop).keyVar, (*thisLoop).counter)
                            vset(nil, ifs, ident, (*thisLoop).loopVar, (*thisLoop).iterOverArray.([]ProcessInfo)[(*thisLoop).counter])
                        case []SystemResources:
                            vset(nil, ifs, ident, (*thisLoop).keyVar, (*thisLoop).counter)
                            vset(nil, ifs, ident, (*thisLoop).loopVar, (*thisLoop).iterOverArray.([]SystemResources)[(*thisLoop).counter])
                        case []MemoryInfo:
                            vset(nil, ifs, ident, (*thisLoop).keyVar, (*thisLoop).counter)
                            vset(nil, ifs, ident, (*thisLoop).loopVar, (*thisLoop).iterOverArray.([]MemoryInfo)[(*thisLoop).counter])
                        case []CPUInfo:
                            vset(nil, ifs, ident, (*thisLoop).keyVar, (*thisLoop).counter)
                            vset(nil, ifs, ident, (*thisLoop).loopVar, (*thisLoop).iterOverArray.([]CPUInfo)[(*thisLoop).counter])

                        case []NetworkIOStats:
                            vset(nil, ifs, ident, (*thisLoop).keyVar, (*thisLoop).counter)
                            vset(nil, ifs, ident, (*thisLoop).loopVar, (*thisLoop).iterOverArray.([]NetworkIOStats)[(*thisLoop).counter])
                        case []DiskIOStats:
                            vset(nil, ifs, ident, (*thisLoop).keyVar, (*thisLoop).counter)
                            vset(nil, ifs, ident, (*thisLoop).loopVar, (*thisLoop).iterOverArray.([]DiskIOStats)[(*thisLoop).counter])
                        case []ProcessTree:
                            vset(nil, ifs, ident, (*thisLoop).keyVar, (*thisLoop).counter)
                            vset(nil, ifs, ident, (*thisLoop).loopVar, (*thisLoop).iterOverArray.([]ProcessTree)[(*thisLoop).counter])
                        case []ProcessMap:
                            vset(nil, ifs, ident, (*thisLoop).keyVar, (*thisLoop).counter)
                            vset(nil, ifs, ident, (*thisLoop).loopVar, (*thisLoop).iterOverArray.([]ProcessMap)[(*thisLoop).counter])

                        case []ResourceUsage:
                            vset(nil, ifs, ident, (*thisLoop).keyVar, (*thisLoop).counter)
                            vset(nil, ifs, ident, (*thisLoop).loopVar, (*thisLoop).iterOverArray.([]ResourceUsage)[(*thisLoop).counter])
                        case []ResourceSnapshot:
                            vset(nil, ifs, ident, (*thisLoop).keyVar, (*thisLoop).counter)
                            vset(nil, ifs, ident, (*thisLoop).loopVar, (*thisLoop).iterOverArray.([]ResourceSnapshot)[(*thisLoop).counter])

                        case []dirent:
                            vset(nil, ifs, ident, (*thisLoop).keyVar, (*thisLoop).counter)
                            vset(nil, ifs, ident, (*thisLoop).loopVar, (*thisLoop).iterOverArray.([]dirent)[(*thisLoop).counter])
                        case []tui:
                            vset(nil, ifs, ident, (*thisLoop).keyVar, (*thisLoop).counter)
                            vset(nil, ifs, ident, (*thisLoop).loopVar, (*thisLoop).iterOverArray.([]tui)[(*thisLoop).counter])
                        case []stackFrame:
                            vset(nil, ifs, ident, (*thisLoop).keyVar, (*thisLoop).counter)
                            vset(nil, ifs, ident, (*thisLoop).loopVar, (*thisLoop).iterOverArray.([]stackFrame)[(*thisLoop).counter])
                        case []alloc_info:
                            vset(nil, ifs, ident, (*thisLoop).keyVar, (*thisLoop).counter)
                            vset(nil, ifs, ident, (*thisLoop).loopVar, (*thisLoop).iterOverArray.([]alloc_info)[(*thisLoop).counter])
                        case []float64:
                            vset(nil, ifs, ident, (*thisLoop).keyVar, (*thisLoop).counter)
                            vset(nil, ifs, ident, (*thisLoop).loopVar, (*thisLoop).iterOverArray.([]float64)[(*thisLoop).counter])
                        case []*big.Int:
                            vset(nil, ifs, ident, (*thisLoop).keyVar, (*thisLoop).counter)
                            vset(nil, ifs, ident, (*thisLoop).loopVar, (*thisLoop).iterOverArray.([]*big.Int)[(*thisLoop).counter])
                        case []*big.Float:
                            vset(nil, ifs, ident, (*thisLoop).keyVar, (*thisLoop).counter)
                            vset(nil, ifs, ident, (*thisLoop).loopVar, (*thisLoop).iterOverArray.([]*big.Float)[(*thisLoop).counter])
                        case [][]int:
                            vset(nil, ifs, ident, (*thisLoop).keyVar, (*thisLoop).counter)
                            vset(nil, ifs, ident, (*thisLoop).loopVar, (*thisLoop).iterOverArray.([][]int)[(*thisLoop).counter])
                        case []map[string]any:
                            vset(nil, ifs, ident, (*thisLoop).keyVar, (*thisLoop).counter)
                            vset(nil, ifs, ident, (*thisLoop).loopVar, (*thisLoop).iterOverArray.([]map[string]any)[(*thisLoop).counter])
                        case []any:
                            vset(nil, ifs, ident, (*thisLoop).keyVar, (*thisLoop).counter)
                            vset(nil, ifs, ident, (*thisLoop).loopVar, (*thisLoop).iterOverArray.([]any)[(*thisLoop).counter])
                        default:
                            // @note: should put a proper exit in here.
                            pv, _ := vget(nil, ifs, ident, sf("%v", (*thisLoop).iterOverArray.([]float64)[(*thisLoop).counter]))
                            pf("Unknown type [%T] in END/Foreach\n", pv)
                        }

                        bin := bind_int(ifs, (*thisLoop).loopVar)
                        if it_type != "" {
                            t := (*ident)[bin]
                            t.ITyped = true
                            t.Kind_override = it_type
                            (*ident)[bin] = t
                        }

                        isStruct := reflect.TypeOf((*ident)[bin].IValue).Kind() == reflect.Struct
                        if isStruct && it_type == "" {
                            if s, count := struct_match((*ident)[bin].IValue); count == 1 {
                                (*ident)[bin].Kind_override = s
                            }
                        }

                    }

                case LT_FOR: // move through range

                    if (*thisLoop).repeatCustom {

                        // amend iterator
                        if len((*thisLoop).repeatAmendment) > 0 {
                            evAmendment := parser.wrappedEval(ifs, ident, ifs, ident, (*thisLoop).repeatAmendment)
                            if evAmendment.evalError {
                                parser.report(inbound.SourceLine, "Invalid expression for amendment in FOR")
                                finish(false, ERR_EVAL)
                                break
                            }
                        }

                        // check iterator
                        if len((*thisLoop).repeatCond) > 0 {
                            evCond := parser.wrappedEval(ifs, ident, ifs, ident, (*thisLoop).repeatCond)
                            if evCond.evalError {
                                parser.report(inbound.SourceLine, "Invalid condition for amendment in FOR")
                                finish(false, ERR_EVAL)
                                break
                            }
                            loopEnd = true
                            switch evCond.result.(type) {
                            case bool:
                                if evCond.result.(bool) {
                                    loopEnd = false
                                }
                            default:
                                parser.report(inbound.SourceLine, "Condition does not evaluate to a bool in FOR")
                                finish(false, ERR_EVAL)
                                break
                            }
                        }

                    } else {

                        (*thisLoop).counter += (*thisLoop).repeatActionStep

                        switch (*thisLoop).repeatAction {
                        case ACT_INC:
                            if (*thisLoop).counter > (*thisLoop).condEnd {
                                (*thisLoop).counter -= (*thisLoop).repeatActionStep
                                if (*thisLoop).optNoUse == Opt_LoopIgnore {
                                    (*ident)[(*thisLoop).loopVarBinding].IValue = (*thisLoop).counter
                                }
                                loopEnd = true
                            }
                        case ACT_DEC:
                            if (*thisLoop).counter < (*thisLoop).condEnd {
                                (*thisLoop).counter -= (*thisLoop).repeatActionStep
                                if (*thisLoop).optNoUse == Opt_LoopIgnore {
                                    (*ident)[(*thisLoop).loopVarBinding].IValue = (*thisLoop).counter
                                }
                                loopEnd = true
                            }
                        }

                        // check tokens once for loop var references, then set Opt_LoopSet if found.
                        if (*thisLoop).optNoUse == Opt_LoopStart {
                            (*thisLoop).optNoUse = Opt_LoopIgnore
                            if searchToken(source_base, (*thisLoop).repeatFrom, parser.pc, (*thisLoop).loopVar) {
                                (*thisLoop).optNoUse = Opt_LoopSet
                            }
                        }

                        // assign loop counter value back to local variable
                        if (*thisLoop).optNoUse == Opt_LoopSet {
                            (*ident)[(*thisLoop).loopVarBinding].IValue = (*thisLoop).counter
                        }

                    }

                }

            } else {
                // time to die, mr bond? C_Break reached
                if ((*thisLoop).loopType == LT_FOR && breakIn == C_For) || ((*thisLoop).loopType == LT_FOREACH && breakIn == C_Foreach) {
                    // pf("**reached break reset**\n")
                    breakIn = Error // reset to unbroken
                    forceEnd = false
                    loopEnd = true
                }
            }

            if loopEnd {
                // leave the loop
                depth -= 1
                lastConstruct = lastConstruct[:depth]
                breakIn = Error // reset to unbroken
                forceEnd = false
                if break_count > 0 {
                    break_count -= 1
                    if break_count > 0 {
                        switch lastConstruct[depth-1] {
                        case C_For, C_Foreach, C_While, C_Case:
                            breakIn = lastConstruct[depth-1]
                        }
                    }
                }
            } else {
                // jump back to start of block
                parser.pc = (*thisLoop).repeatFrom - 1 // start of loop will do pc++
            }

        case C_Continue:

            // Continue should work with FOR, FOREACH or WHILE.

            if depth == 0 {
                parser.report(inbound.SourceLine, "Attempting to CONTINUE without a valid surrounding construct.")
                finish(false, ERR_SYNTAX)
            } else {

                // add an IF clause to guard the continue against execution, optionally.

                var continueIgnore bool

                if inbound.TokenCount > 1 {
                    if inbound.Tokens[1].tokType == C_If {
                        if inbound.TokenCount == 2 {
                            parser.report(inbound.SourceLine, "missing condition in CONTINUE IF statement")
                            finish(false, ERR_EVAL)
                            break
                        } else {
                            we = parser.wrappedEval(ifs, ident, ifs, ident, inbound.Tokens[2:])
                            if we.evalError {
                                parser.report(inbound.SourceLine, "could not evaluate CONTINUE IF condition")
                                finish(false, ERR_EVAL)
                                break
                            }
                            switch we.result.(type) {
                            case bool:
                                if !we.result.(bool) {
                                    // condition not met so flag an ignore state on
                                    // the rest of the continue statement
                                    continueIgnore = true
                                }
                            default:
                                parser.report(inbound.SourceLine, "CONTINUE IF condition must evaluate to boolean")
                                finish(false, ERR_EVAL)
                                break
                            }
                        }
                    }
                }

                ////////////////////////////////////////////////////

                if !continueIgnore {
                    switch lastConstruct[depth-1] {
                    case C_For, C_Foreach:
                        thisLoop = &loops[depth]
                        parser.pc = (*thisLoop).forEndPos - 1

                    case C_While:
                        thisLoop = &loops[depth]
                        parser.pc = (*thisLoop).whileContinueAt - 1

                    case C_Case:
                        // mark this as an error for now, as we don't currently
                        //  backtrack through lastConstruct to check the actual
                        //  loop type so that it can be properly unwound.
                        parser.report(inbound.SourceLine, "Attempting to CONTINUE inside a CASE is not permitted.")
                        finish(false, ERR_SYNTAX)
                    }
                }

            }

        case C_Break:

            // Break should work with either FOR, FOREACH, WHILE or CASE.

            // We use lastConstruct to establish which is the innermost
            //  of these from which we need to break out.

            // The surrounding construct should set the
            //  lastConstruct[depth] on entry.

            // check for break depth argument

            break_count = 0

            if inbound.TokenCount > 1 {

                // break by construct type
                if inbound.TokenCount == 2 {
                    thisLoop = &loops[depth]
                    forceEnd = false

                    var efound, er, ifEr bool
                    switch inbound.Tokens[1].tokType {
                    case C_Case:
                        efound, _, er = lookahead(source_base, parser.pc, 1, 0, C_Endcase, []int64{C_Case}, []int64{C_Endcase})
                        breakIn = C_Case
                        forceEnd = true
                        parser.pc = wc[wccount].endLine - 1
                    case C_For:
                        efound, _, er = lookahead(source_base, parser.pc, 1, 0, C_Endfor, []int64{C_For, C_Foreach}, []int64{C_Endfor})
                        breakIn = C_For
                        forceEnd = true
                        parser.pc = (*thisLoop).forEndPos - 1
                    case C_Foreach:
                        efound, _, er = lookahead(source_base, parser.pc, 1, 0, C_Endfor, []int64{C_For, C_Foreach}, []int64{C_Endfor})
                        breakIn = C_Foreach
                        forceEnd = true
                        parser.pc = (*thisLoop).forEndPos - 1
                    case C_While:
                        efound, _, er = lookahead(source_base, parser.pc, 1, 0, C_Endwhile, []int64{C_While}, []int64{C_Endwhile})
                        breakIn = C_While
                        forceEnd = true
                        parser.pc = (*thisLoop).whileContinueAt - 1
                    case C_If:
                        ifEr = true
                    }
                    if ifEr {
                        parser.report(inbound.SourceLine, "BREAK IF missing a condition")
                        finish(false, ERR_SYNTAX)
                        break
                    }
                    if er {
                        // lookahead error
                        parser.report(inbound.SourceLine, sf("BREAK [%s] cannot find end of construct", tokNames[breakIn]))
                        finish(false, ERR_SYNTAX)
                        break
                    }
                    if efound {
                        // break jump point is set, so continue pc loop
                        continue
                    }

                }

                // break [n] if ... syntax

                var has_break_count bool
                if inbound.TokenCount > 2 {

                    if inbound.Tokens[1].tokType == C_If || (inbound.Tokens[1].tokType == NumericLiteral && inbound.Tokens[2].tokType == C_If) {

                        cond_start_pos := 2
                        if inbound.Tokens[1].tokType == NumericLiteral {
                            break_count, _ = GetAsInt(inbound.Tokens[1].tokVal)
                            cond_start_pos = 3
                            has_break_count = true
                        }

                        we = parser.wrappedEval(ifs, ident, ifs, ident, inbound.Tokens[cond_start_pos:])
                        if we.evalError {
                            parser.report(inbound.SourceLine, "could not evaluate BREAK IF condition")
                            finish(false, ERR_EVAL)
                            break
                        }

                        var hasIfErr bool
                        var breakIgnore bool

                        switch we.result.(type) {
                        case bool:
                            if !we.result.(bool) {
                                breakIgnore = true
                            }
                            if !has_break_count {
                                break_count = 1
                                has_break_count = true
                            }
                        default:
                            parser.report(inbound.SourceLine, "BREAK IF condition must evaluate to boolean")
                            finish(false, ERR_EVAL)
                            hasIfErr = true
                        }
                        if hasIfErr {
                            break
                        }
                        if breakIgnore {
                            continue
                        } // this skips all potential break processing and moves to next pc statement
                    }
                }

                /////////////////////////////////////

                if !forceEnd && !has_break_count {
                    // break by expression
                    break_depth := parser.wrappedEval(ifs, ident, ifs, ident, inbound.Tokens[1:])
                    switch break_depth.result.(type) {
                    case int:
                        break_count = break_depth.result.(int)
                        // pf("-- break/expr int->%v\n",break_count)
                    default:
                        parser.report(inbound.SourceLine, "Could not evaluate BREAK depth argument")
                        finish(false, ERR_EVAL)
                        break
                    }
                }

                if forceEnd { // IF clause cannot trigger this:
                    // set count of back tracking in end* statements
                    for break_count = 1; break_count <= depth; break_count += 1 {
                        // pf("(cbreak) increasing break_count to %v\n",break_count)
                        lce := lastConstruct[depth-break_count]
                        // pf("(cbreak) now processing lc type of %v\n",tokNames[lce])
                        if lce == C_Case {
                            wccount -= 1
                        }
                        if lce == C_While {
                        }
                        if lce == inbound.Tokens[1].tokType {
                            break
                        }
                    }
                    // pf("(cbreak) final break_count value is %v\n",break_count)
                }

            }

            // jump calc, depending on break context

            thisLoop = &loops[depth]

            switch lastConstruct[depth-1] {

            case C_For:
                parser.pc = (*thisLoop).forEndPos - 1
                breakIn = C_For

            case C_Foreach:
                parser.pc = (*thisLoop).forEndPos - 1
                breakIn = C_Foreach

            case C_While:
                parser.pc = (*thisLoop).whileContinueAt - 1
                breakIn = C_While

            case C_Case:
                parser.pc = wc[wccount].endLine - 1
                breakIn = C_Case

            default:
                parser.report(inbound.SourceLine, "A grue is attempting to BREAK out. (Breaking without a surrounding context!)")
                // pf("breakin->%v depth->%v wccount->%v thisloop->%#v\n",breakIn,depth,wccount,thisLoop)
                // pf("breakcount->%v lastConstruct->%#v\n",break_count,lastConstruct[depth-1])
                finish(false, ERR_SYNTAX)
                break
            }

        case C_Enum:

            if inbound.TokenCount < 4 || (!(inbound.Tokens[2].tokType == LParen && inbound.Tokens[inbound.TokenCount-1].tokType == RParen) &&
                !(inbound.Tokens[2].tokType == LeftCBrace && inbound.Tokens[inbound.TokenCount-1].tokType == RightCBrace)) {
                parser.report(inbound.SourceLine, "Incorrect arguments supplied for ENUM.")
                finish(false, ERR_SYNTAX)
                break
            }

            resu := parser.splitCommaArray(inbound.Tokens[3 : inbound.TokenCount-1])

            globlock.Lock()
            enum_name := parser.namespace + "::" + inbound.Tokens[1].tokText
            enum[enum_name] = &enum_s{}
            enum[enum_name].members = make(map[string]any)
            enum[enum_name].namespace = parser.namespace
            globlock.Unlock()

            var nextVal any
            nextVal = 0 // auto incs to 1 for first default value
            var member string
        enum_loop:
            for ea := range resu {

                if len(resu[ea]) == 1 {
                    switch nextVal.(type) {
                    case int:
                        nextVal = nextVal.(int) + 1
                    case uint:
                        nextVal = nextVal.(uint) + 1
                    case int64:
                        nextVal = nextVal.(int64) + 1
                    case float64:
                        nextVal = nextVal.(float64) + 1
                    default:
                        // non-incremental error
                        parser.report(inbound.SourceLine, "Cannot increment default value in ENUM")
                        finish(false, ERR_EVAL)
                        break enum_loop
                    }

                    globlock.Lock()
                    member = resu[ea][0].tokText
                    enum[enum_name].members[member] = nextVal
                    enum[enum_name].ordered = append(enum[enum_name].ordered, member)
                    globlock.Unlock()

                } else {
                    //   member = constant
                    // | member = expr
                    if len(resu[ea]) > 2 {
                        if resu[ea][1].tokType == O_Assign {

                            evEnum := parser.wrappedEval(ifs, ident, ifs, ident, resu[ea][2:])

                            if evEnum.evalError {
                                parser.report(inbound.SourceLine, "Invalid expression for assignment in ENUM")
                                finish(false, ERR_EVAL)
                                break enum_loop
                            }

                            nextVal = evEnum.result

                            globlock.Lock()
                            member = resu[ea][0].tokText
                            enum[enum_name].members[member] = nextVal
                            enum[enum_name].ordered = append(enum[enum_name].ordered, member)
                            globlock.Unlock()

                        } else {
                            // error
                            parser.report(inbound.SourceLine, "Missing assignment in ENUM")
                            finish(false, ERR_SYNTAX)
                            break enum_loop
                        }
                    }
                }
            }

        case C_Unset: // undeclare variables

            if inbound.TokenCount < 2 {
                parser.report(inbound.SourceLine, "Incorrect arguments supplied for UNSET.")
                finish(false, ERR_SYNTAX)
            } else {
                resu := parser.splitCommaArray(inbound.Tokens[1:])
                for e := 0; e < len(resu); e++ {
                    if len(resu[e]) == 1 {
                        removee := resu[e][0].tokText
                        if (*ident)[resu[e][0].bindpos].declared {
                            vunset(ifs, ident, removee)
                        } else {
                            /*
                               parser.report(inbound.SourceLine, sf("Variable %s does not exist.", removee))
                               finish(false, ERR_EVAL)
                               break
                            */
                        }
                    } else {
                        parser.report(inbound.SourceLine, sf("Invalid variable specification '%v' in UNSET.", resu[e]))
                        finish(false, ERR_EVAL)
                        break
                    }
                }
            }

        case C_Pane:

            if inbound.TokenCount == 1 {
                pf("Current  %-24s %3s %3s %3s %3s %s\n", "Name", "y", "x", "h", "w", "Title")
                for p, v := range panes {
                    def := ""
                    if p == currentpane {
                        def = "*"
                    }
                    pf("%6s   %-24s %3d %3d %3d %3d %s\n", def, p, v.row, v.col, v.h, v.w, v.title)
                }
                break
            }

            switch str.ToLower(inbound.Tokens[1].tokText) {
            case "off":
                if inbound.TokenCount != 2 {
                    parser.report(inbound.SourceLine, "Too many arguments supplied.")
                    finish(false, ERR_SYNTAX)
                    break
                }
                // disable
                panes = make(map[string]Pane)
                panes["global"] = Pane{row: 0, col: 0, h: MH, w: MW + 1}
                currentpane = "global"
                setPane("global")

            case "select":

                if inbound.TokenCount != 3 {
                    parser.report(inbound.SourceLine, "Invalid pane selection.")
                    finish(false, ERR_SYNTAX)
                    break
                }

                cp, _ := parser.Eval(ifs, inbound.Tokens[2:3])

                switch cp := cp.(type) {
                case string:

                    setPane(cp)
                    currentpane = cp

                default:
                    parser.report(inbound.SourceLine, "Warning: you must provide a string value to PANE SELECT.")
                    finish(false, ERR_EVAL)
                    break
                }

            case "define":

                var title = ""
                var boxed string = "round" // box style // none,round,square,double

                // Collect the expressions for each position
                //      pane define name , y , x , h , w [ , title [ , border ] ]

                nameCommaAt := findDelim(inbound.Tokens, O_Comma, 3)
                YCommaAt := findDelim(inbound.Tokens, O_Comma, nameCommaAt+1)
                XCommaAt := findDelim(inbound.Tokens, O_Comma, YCommaAt+1)
                HCommaAt := findDelim(inbound.Tokens, O_Comma, XCommaAt+1)
                WCommaAt := findDelim(inbound.Tokens, O_Comma, HCommaAt+1)
                TCommaAt := findDelim(inbound.Tokens, O_Comma, WCommaAt+1)

                if nameCommaAt == -1 || YCommaAt == -1 || XCommaAt == -1 || HCommaAt == -1 {
                    parser.report(inbound.SourceLine, "Bad delimiter in PANE DEFINE.")
                    finish(false, ERR_SYNTAX)
                    break
                }

                hasTitle := false
                hasBox := false
                if TCommaAt > -1 {
                    hasTitle = true
                    if TCommaAt < inbound.TokenCount-1 {
                        hasBox = true
                    }
                }

                // var ew,etit,ebox ExpressionCarton
                var ew, etit, ebox []Token

                if hasTitle {
                    ew = inbound.Tokens[HCommaAt+1 : WCommaAt]
                } else {
                    ew = inbound.Tokens[HCommaAt+1:]
                }

                if hasTitle && hasBox {
                    etit = inbound.Tokens[WCommaAt+1 : TCommaAt]
                    ebox = inbound.Tokens[TCommaAt+1:]
                } else {
                    if hasTitle {
                        etit = inbound.Tokens[WCommaAt+1:]
                    }
                }

                var ptitle, pbox ExpressionCarton
                pname := parser.wrappedEval(ifs, ident, ifs, ident, inbound.Tokens[2:nameCommaAt])
                py := parser.wrappedEval(ifs, ident, ifs, ident, inbound.Tokens[nameCommaAt+1:YCommaAt])
                px := parser.wrappedEval(ifs, ident, ifs, ident, inbound.Tokens[YCommaAt+1:XCommaAt])
                ph := parser.wrappedEval(ifs, ident, ifs, ident, inbound.Tokens[XCommaAt+1:HCommaAt])
                pw := parser.wrappedEval(ifs, ident, ifs, ident, ew)
                if hasTitle {
                    ptitle = parser.wrappedEval(ifs, ident, ifs, ident, etit)
                }
                if hasBox {
                    pbox = parser.wrappedEval(ifs, ident, ifs, ident, ebox)
                }

                if pname.evalError || py.evalError || px.evalError || ph.evalError || pw.evalError {
                    parser.report(inbound.SourceLine, "could not evaluate an argument in PANE DEFINE")
                    finish(false, ERR_EVAL)
                    break
                }

                name := sf("%v", pname.result)
                col, invalid1 := GetAsInt(px.result)
                row, invalid2 := GetAsInt(py.result)
                w, invalid3 := GetAsInt(pw.result)
                h, invalid4 := GetAsInt(ph.result)
                if hasTitle {
                    title = sf("%v", ptitle.result)
                }
                if hasBox {
                    boxed = sf("%v", pbox.result)
                }

                if invalid1 || invalid2 || invalid3 || invalid4 {
                    parser.report(inbound.SourceLine, sf("Could not use an argument in PANE DEFINE. [%T %T %T %T]", px.result, py.result, pw.result, ph.result))
                    finish(false, ERR_EVAL)
                    break
                }

                if pname.result.(string) == "global" {
                    parser.report(inbound.SourceLine, "Cannot redefine the global PANE.")
                    finish(false, ERR_SYNTAX)
                    break
                }

                panes[name] = Pane{row: row, col: col, w: w, h: h, title: title, boxed: boxed}
                paneBox(name)

            case "title":
                if inbound.TokenCount > 2 {
                    etit := inbound.Tokens[2:]
                    ptitle := parser.wrappedEval(ifs, ident, ifs, ident, etit)
                    p := panes[currentpane]
                    p.title = sf("%v", ptitle.result)
                    panes[currentpane] = p
                    paneBox(currentpane)
                }

            case "redraw":
                paneBox(currentpane)

            default:
                parser.report(inbound.SourceLine, "Unknown PANE command.")
                finish(false, ERR_SYNTAX)
            }

        case SYM_BOR: // Local Command

            bc := interpolate(currentModule, ifs, ident, basecode[source_base][parser.pc].borcmd)

            /*
               pf("\n")
               pf("In local command\nCalled with ifs:%d and tokens->%+v\n",ifs,inbound.Tokens)
               pf("  source_base -> %v\n",source_base)
               pf("  basecode    -> %v\n",basecode[source_base][parser.pc].Original)
               pf("  bor cmd     -> %#v\n",bc)
               pf("\n")
            */

            if inbound.TokenCount == 2 && hasOuter(inbound.Tokens[1].tokText, '`') {
                s := interpolate(currentModule, ifs, ident, stripOuter(inbound.Tokens[1].tokText, '`'))
                coprocCall(s)
            } else {
                coprocCall(bc)
            }

        case C_Pause:

            var i string

            if inbound.TokenCount < 2 {
                parser.report(inbound.SourceLine, "Not enough arguments in PAUSE.")
                finish(false, ERR_SYNTAX)
                break
            }

            we = parser.wrappedEval(ifs, ident, ifs, ident, inbound.Tokens[1:])

            if !we.evalError {

                if isNumber(we.result) {
                    i = sf("%v", we.result)
                } else {
                    i = we.result.(string)
                }

                dur, err := time.ParseDuration(i + "ms")

                if err != nil {
                    parser.report(inbound.SourceLine, sf("'%s' did not evaluate to a duration.", we.text))
                    finish(false, ERR_EVAL)
                    break
                }

                time.Sleep(dur)

            } else {
                parser.report(inbound.SourceLine, sf("could not evaluate PAUSE expression\n%+v", we.errVal))
                finish(false, ERR_EVAL)
                break
            }

        case C_Doc:

            // Check if HEREDOC
            isHeredoc := false
            if inbound.TokenCount > 1 {
                if tok := inbound.Tokens[1]; strings.ToLower(tok.tokText) == "gen" || strings.ToLower(tok.tokText) == "delim" || strings.ToLower(tok.tokText) == "var" {
                    isHeredoc = true
                }
            }

            if isHeredoc {
                // HEREDOC
                var varName, delim string
                var hasGen bool
                var content string
                for i, tok := range inbound.Tokens[1:] {
                    lowerText := strings.ToLower(tok.tokText)
                    if lowerText == "gen" || lowerText == "delim" || lowerText == "var" {
                        switch lowerText {
                        case "gen":
                            hasGen = true
                        case "delim":
                            if i+1 < len(inbound.Tokens[1:]) {
                                delim = inbound.Tokens[1+i+1].tokText
                            }
                        case "var":
                            if i+1 < len(inbound.Tokens[1:]) {
                                varName = inbound.Tokens[1+i+1].tokText
                            }
                        }
                    } else if tok.tokType == StringLiteral {
                        content = tok.tokText
                    }
                }
                if isHeredoc && delim == "" {
                    delim = "\n\n"
                }
                if delim != "" {
                    // get from registry
                    docRegistryLock.RLock()
                    for _, doc := range docRegistry {
                        if doc.line == inbound.SourceLine {
                            content = doc.content
                            break
                        }
                    }
                    docRegistryLock.RUnlock()
                }
                if varName != "" {
                    vset(nil, ifs, ident, varName, content)
                }
                if hasGen && testMode {
                    appendToTestReport(test_output_file, ifs, parser.pc,
                        interpolate(currentModule, ifs, ident, content),
                    )
                }
            } else {
                // old DOC
                if testMode {
                    if inbound.TokenCount > 1 {
                        evnest := 0
                        newstart := 0
                        docout := ""
                        for term := range inbound.Tokens[1:] {
                            nt := inbound.Tokens[1+term]
                            // pf("(doc) term %+v nt %+v\n",term,nt)
                            if nt.tokType == LParen || nt.tokType == LeftSBrace {
                                evnest += 1
                            }
                            if nt.tokType == RParen || nt.tokType == RightSBrace {
                                evnest -= 1
                            }
                            if evnest == 0 && (term == len(inbound.Tokens[1:])-1 || nt.tokType == O_Comma) {
                                v, _ := parser.Eval(ifs, inbound.Tokens[1+newstart:term+2])
                                newstart = term + 1
                                switch v.(type) {
                                case string:
                                    v = interpolate(currentModule, ifs, ident, v.(string))
                                }
                                docout += sparkle(sf(`%v`, v))
                                continue
                            }
                        }

                        appendToTestReport(test_output_file, ifs, parser.pc, docout)

                    }
                }
            }

        case C_Test:

            // TEST "name" GROUP "group_name" ASSERT FAIL|CONTINUE

            testlock.Lock()
            inside_test = true

            if testMode {

                if !(inbound.TokenCount == 4 || inbound.TokenCount == 6) {
                    parser.report(inbound.SourceLine, "Badly formatted TEST command.")
                    finish(false, ERR_SYNTAX)
                    testlock.Unlock()
                    break
                }

                if !str.EqualFold(inbound.Tokens[2].tokText, "group") {
                    parser.report(inbound.SourceLine, "Missing GROUP in TEST command.")
                    finish(false, ERR_SYNTAX)
                    testlock.Unlock()
                    break
                }

                test_assert = "fail"
                if inbound.TokenCount == 6 {
                    if !str.EqualFold(inbound.Tokens[4].tokText, "assert") {
                        parser.report(inbound.SourceLine, "Missing ASSERT in TEST command.")
                        finish(false, ERR_SYNTAX)
                        testlock.Unlock()
                        break
                    } else {
                        switch str.ToLower(inbound.Tokens[5].tokText) {
                        case "fail":
                            test_assert = "fail"
                        case "continue":
                            test_assert = "continue"
                        default:
                            parser.report(inbound.SourceLine, "Bad ASSERT type in TEST command.")
                            finish(false, ERR_SYNTAX)
                            testlock.Unlock()
                            break
                        }
                    }
                }

                test_name = interpolate(currentModule, ifs, ident, stripOuterQuotes(inbound.Tokens[1].tokText, 2))
                test_group = interpolate(currentModule, ifs, ident, stripOuterQuotes(inbound.Tokens[3].tokText, 2))

                under_test = false
                // if filter matches group
                if test_name_filter == "" {
                    if matched, _ := regexp.MatchString(test_group_filter, test_group); matched {
                        vset(nil, ifs, ident, "_test_group", test_group)
                        vset(nil, ifs, ident, "_test_name", test_name)
                        under_test = true
                        appendToTestReport(test_output_file, ifs, parser.pc, sf("\nTest Section : [#5][#bold]%s/%s[#boff][#-]", test_group, test_name))
                    }
                } else {
                    // if filter matches name
                    if matched, _ := regexp.MatchString(test_name_filter, test_name); matched {
                        vset(nil, ifs, ident, "_test_group", test_group)
                        vset(nil, ifs, ident, "_test_name", test_name)
                        under_test = true
                        appendToTestReport(test_output_file, ifs, parser.pc, sf("\nTest Section : [#5][#bold]%s/%s[#boff][#-]", test_group, test_name))
                    }
                }

            }
            testlock.Unlock()

        case C_Endtest:

            testlock.Lock()
            under_test = false
            inside_test = false
            testlock.Unlock()

        case C_On:
            // ON expr DO action
            // was false? - discard command tokens and continue
            // was true? - reform command without the 'ON condition' tokens and re-enter command switch

            if inbound.TokenCount > 2 {

                doAt := findDelim(inbound.Tokens, C_Do, 1)
                if doAt == -1 {
                    parser.report(inbound.SourceLine, "DO not found in ON")
                    finish(false, ERR_SYNTAX)
                } else {
                    // more tokens after the DO to form a command with?
                    if inbound.TokenCount >= doAt {

                        we = parser.wrappedEval(ifs, ident, ifs, ident, inbound.Tokens[1:doAt])
                        if we.evalError {
                            parser.report(inbound.SourceLine, sf("Could not evaluate expression '%v' in ON..DO statement.\n%+v", we.text, we.errVal))
                            finish(false, ERR_EVAL)
                            break
                        }

                        switch we.result.(type) {
                        case bool:
                            if we.result.(bool) {

                                // create a phrase
                                p := Phrase{}
                                b := BaseCode{}
                                p.Tokens = inbound.Tokens[doAt+1:]
                                p.TokenCount = inbound.TokenCount - (doAt + 1)
                                b.Original = basecode[source_base][parser.pc].Original

                                // action!
                                inbound = &p
                                basecode_entry = &b
                                goto ondo_reenter

                            }

                        default:
                            pf("Result Type -> %T expression was -> %v\n", we.text, we.result)
                            parser.report(inbound.SourceLine, "ON cannot operate without a condition.")
                            finish(false, ERR_EVAL)
                            break
                        }

                    }
                }

            } else {
                parser.report(inbound.SourceLine, "ON missing arguments.")
                finish(false, ERR_SYNTAX)
            }

        case C_Assert:

            if inbound.TokenCount < 2 {
                parser.report(inbound.SourceLine, "Insufficient arguments supplied to ASSERT")
                finish(false, ERR_ASSERT)
                break
            }

            // Determine if this is ASSERT ERROR or normal ASSERT
            isAssertError := inbound.TokenCount > 2 && inbound.Tokens[1].tokText == "ERROR"

            var exprTokens []Token
            var messageTokens []Token
            var hasCustomMessage bool

            if isAssertError {
                // Look for comma in ASSERT ERROR tokens
                commaAt := findDelim(inbound.Tokens[2:], O_Comma, 0)
                if commaAt != -1 {
                    exprTokens = inbound.Tokens[2 : 2+commaAt]
                    messageTokens = inbound.Tokens[2+commaAt+1:]
                    hasCustomMessage = true
                } else {
                    exprTokens = inbound.Tokens[2:]
                }
            } else {
                // Look for comma in normal ASSERT tokens
                commaAt := findDelim(inbound.Tokens[1:], O_Comma, 0)
                if commaAt != -1 {
                    exprTokens = inbound.Tokens[1 : 1+commaAt]
                    messageTokens = inbound.Tokens[1+commaAt+1:]
                    hasCustomMessage = true
                } else {
                    exprTokens = inbound.Tokens[1:]
                }
            }

            // Evaluate once
            oldEnforceError := enforceError
            enforceError = false
            we := parser.wrappedEval(ifs, ident, ifs, ident, exprTokens)

            // Evaluate custom message if present
            var customMessage string
            if hasCustomMessage && len(messageTokens) > 0 {
                msgEval := parser.wrappedEval(ifs, ident, ifs, ident, messageTokens)
                if !msgEval.evalError {
                    customMessage = GetAsString(msgEval.result)
                }
            }

            enforceError = oldEnforceError

            // Non-test mode: exit early with lightweight checks
            if !under_test {
                // Check if we're inside a try block using the state information
                insideTryBlock := calltable[ifs].isTryBlock

                var needsThrow bool
                var throwMessage string

                if isAssertError {
                    if !we.evalError {
                        throwMessage = "ASSERT ERROR: expression did not throw an error"
                        parser.report(inbound.SourceLine, throwMessage)
                        if insideTryBlock {
                            needsThrow = true
                        } else {
                            finish(false, ERR_ASSERT)
                        }
                    }
                    if !needsThrow {
                        // Passed: errored as expected
                        break
                    }
                } else {
                    // Normal ASSERT
                    if we.assign {
                        throwMessage = "Assert contained an assignment"
                        parser.report(inbound.SourceLine, "[#2][#bold]Warning! Assert contained an assignment![#-][#boff]")
                        if insideTryBlock {
                            needsThrow = true
                        } else {
                            finish(false, ERR_ASSERT)
                        }
                    } else if we.evalError {
                        throwMessage = "Could not evaluate expression in ASSERT statement"
                        parser.report(inbound.SourceLine, throwMessage)
                        if insideTryBlock {
                            needsThrow = true
                        } else {
                            finish(false, ERR_EVAL)
                        }
                    } else if b, ok := we.result.(bool); !ok || !b {
                        if customMessage != "" {
                            throwMessage = sf("Could not assert! (%s)", customMessage)
                        } else {
                            throwMessage = "Could not assert! (assertion failed)"
                        }
                        parser.report(inbound.SourceLine, throwMessage)
                        if insideTryBlock {
                            needsThrow = true
                        } else {
                            finish(false, ERR_ASSERT)
                        }
                    }
                }

                // Inject throw statement if needed
                if needsThrow {
                    // Create tokens for: throw "assert" with throwMessage
                    p := Phrase{}
                    p.Tokens = []Token{
                        {tokType: C_Throw, tokText: "throw"},
                        {tokType: StringLiteral, tokText: "\"assert\"", tokVal: "assert"},
                        {tokType: C_With, tokText: "with"},
                        {tokType: StringLiteral, tokText: sf("\"%s\"", throwMessage), tokVal: throwMessage},
                    }
                    p.TokenCount = int16(len(p.Tokens))
                    p.SourceLine = inbound.SourceLine

                    b := BaseCode{}
                    b.Original = basecode[source_base][parser.pc].Original

                    // Re-enter the switch with the throw statement
                    inbound = &p
                    basecode_entry = &b
                    goto ondo_reenter
                }
                break
            }

            // Under test: use full test reporting
            if isAssertError {
                if we.evalError {
                    if customMessage != "" {
                        handleTestResult(ifs, true, inbound.SourceLine, "ASSERT ERROR", customMessage)
                    } else {
                        handleTestResult(ifs, true, inbound.SourceLine, "ASSERT ERROR", "expression threw an error as expected")
                    }
                } else {
                    if customMessage != "" {
                        handleTestResult(ifs, false, inbound.SourceLine, "ASSERT ERROR", customMessage)
                    } else {
                        handleTestResult(ifs, false, inbound.SourceLine, "ASSERT ERROR", "expression did not throw an error")
                    }
                }
                break
            }

            // Regular ASSERT with full test reporting
            cet := crushEvalTokens(exprTokens)
            if we.assign {
                parser.report(inbound.SourceLine, "[#2][#bold]Warning! Assert contained an assignment![#-][#boff]")
                finish(false, ERR_ASSERT)
                break
            }
            if we.evalError {
                parser.report(inbound.SourceLine, "Could not evaluate expression in ASSERT statement")
                finish(false, ERR_EVAL)
                break
            }
            if b, ok := we.result.(bool); !ok || !b {
                if customMessage != "" {
                    handleTestResult(ifs, false, inbound.SourceLine, cet.text, customMessage)
                } else {
                    handleTestResult(ifs, false, inbound.SourceLine, cet.text, sf("Could not assert! (%s)", we.text))
                }
            } else {
                if customMessage != "" {
                    handleTestResult(ifs, true, inbound.SourceLine, cet.text, customMessage)
                } else {
                    handleTestResult(ifs, true, inbound.SourceLine, cet.text, cet.text)
                }
            }

        case C_Help:
            var hargs []string
            if inbound.TokenCount > 1 {
                for _, ht := range inbound.Tokens[1:] {
                    hargs = append(hargs, ht.tokText)
                }
            }
            ihelp(currentModule, hargs)

        case C_Nop:
            // time.Sleep(1 * time.Microsecond)

        case C_Async:

            // ASYNC IDENTIFIER (namespace :: ) IDENTIFIER LPAREN [EXPRESSION[,...]] RPAREN [IDENTIFIER]
            // async handles    (ns :: )        q          (      [e[,...]]          )      [key]

            if inbound.TokenCount < 5 {
                usage := "ASYNC [#i1]handle_map function_call([args]) [next_id][#i0]"
                parser.report(inbound.SourceLine, "Invalid arguments in ASYNC\n"+usage)
                finish(false, ERR_SYNTAX)
                break
            }

            handles := inbound.Tokens[1].tokText

            // namespace check
            skip := int16(0)
            found_namespace := parser.namespace
            if inbound.Tokens[3].tokType == SYM_DoubleColon {
                found_namespace = inbound.Tokens[2].tokText
                skip = 2
            }

            call := found_namespace + "::" + inbound.Tokens[2+skip].tokText

            if inbound.Tokens[3+skip].tokType != LParen {
                parser.report(inbound.SourceLine, "could not find '(' in ASYNC function call.")
                finish(false, ERR_SYNTAX)
            }

            // get arguments

            var rightParenLoc int16
            for ap := inbound.TokenCount - 1; ap > 3+skip; ap -= 1 {
                if inbound.Tokens[ap].tokType == RParen {
                    rightParenLoc = ap
                    break
                }
            }

            if rightParenLoc < 4 {
                parser.report(inbound.SourceLine, "could not find a valid ')' in ASYNC function call.")
                finish(false, ERR_SYNTAX)
            }

            resu, errs := parser.evalCommaArray(ifs, inbound.Tokens[4+skip:rightParenLoc])

            // find the optional key argument, for stipulating the key name to be used in handles
            var nival any
            if rightParenLoc != inbound.TokenCount-1 {
                var err error
                nival, err = parser.Eval(ifs, inbound.Tokens[rightParenLoc+1:])
                if err != nil {
                    parser.report(inbound.SourceLine, sf("could not evaluate handle key argument '%+v' in ASYNC.", inbound.Tokens[rightParenLoc+1:]))
                    finish(false, ERR_EVAL)
                    break
                }
            }

            lmv, isfunc := fnlookup.lmget(call)

            if isfunc {

                errClear := true
                for e := 0; e < len(errs); e += 1 {
                    if errs[e] != nil {
                        // error
                        pf("- arg %d: %+v\n", errs[e])
                        errClear = false
                    }
                }

                if !errClear {
                    parser.report(inbound.SourceLine, sf("problem evaluating arguments in function call. (fs=%v)\n", ifs))
                    finish(false, ERR_EVAL)
                    break
                }

                // make Za function call

                // construct a go call that includes a normal Call
                globlock.Lock()
                if handles == "nil" {
                    _, _ = task(ifs, lmv, true, call, resu...)
                } else {
                    h, id := task(ifs, lmv, false, call, resu...)
                    // assign channel h to handles map
                    if nival == nil {
                        // fmt.Printf("about to vsetElement() in ASYNC (no key name) : nival:%#v h:%#v\n",nival,h)
                        vsetElement(nil, ifs, ident, handles, sf("async_%v", id), h)
                    } else {
                        // fmt.Printf("about to vsetElement() in ASYNC : nival:%#v h:%#v\n",nival,h)
                        vsetElement(nil, ifs, ident, handles, sf("%v", nival), h)
                    }
                }
                globlock.Unlock()

            } else {
                // func not found
                parser.report(inbound.SourceLine, sf("invalid function '%s' in ASYNC call", call))
                finish(false, ERR_EVAL)
            }

        case C_Macro:

            if !permit_macro {
                parser.report(inbound.SourceLine, "macro() not permitted!")
                finish(false, ERR_EVAL)
                break
            }

            // macro [!] [-+] m_name "value" or macro [!] - [m_name] or macro list

            if inbound.TokenCount == 1 {
                pf("Usage: macro [!] [-+] name \"value\" | macro [!] - [name] | macro [!] list\n")
                break
            }

            var verbose bool
            var isDefine bool
            var name string
            var value string
            var errMsg string

            i := int16(1) // start after 'macro'

            // check for !
            if i < inbound.TokenCount && inbound.Tokens[i].tokText == "!" {
                verbose = true
                i++
            }

            // check for list
            if i < inbound.TokenCount && inbound.Tokens[i].tokText == "list" {
                // list macros
                var count int
                macroMap.Range(func(key, val any) bool {
                    count++
                    k := key.(string)
                    def := val.(MacroDef)
                    params := make([]string, len(def.Params))
                    copy(params, def.Params)
                    if def.HasVarargs && len(params) > 0 {
                        params[len(params)-1] += "..."
                    }
                    paramsStr := str.Join(params, ",")
                    if paramsStr != "" {
                        paramsStr = "(" + paramsStr + ")"
                    }
                    pf("[#1]#%s%s[#-] -> %s\n", k, paramsStr, def.Template)
                    return true
                })
                if count == 0 {
                    pf("No macros defined.\n")
                }
                break
            }

            // check for - or +
            if i < inbound.TokenCount {
                tok := inbound.Tokens[i]
                if tok.tokType == O_Plus {
                    isDefine = true
                    i++
                } else if tok.tokType == O_Minus {
                    isDefine = false
                    i++
                } else {
                    errMsg = "Expected - or + after macro"
                }
            } else {
                errMsg = "Incomplete macro statement"
            }

            if errMsg == "" {
                if isDefine {
                    // define: name "value"
                    name = ""
                    for i < inbound.TokenCount && inbound.Tokens[i].tokType != StringLiteral {
                        name += inbound.Tokens[i].tokText
                        i++
                    }
                    if i < inbound.TokenCount && inbound.Tokens[i].tokType == StringLiteral {
                        value = inbound.Tokens[i].tokText
                        i++
                    } else {
                        errMsg = "Expected quoted value for macro define"
                    }
                } else {
                    // undefine: [name]
                    name = ""
                    for i < inbound.TokenCount {
                        name += inbound.Tokens[i].tokText
                        i++
                    }
                }
            }

            if errMsg != "" {
                pf("Error: %s\n", errMsg)
                break
            }

            // execute
            if isDefine {
                macroDefine(name, value, verbose)
            } else {
                macroUndefine(name)
            }

        case C_Require: // @note: this keyword may be remove

            // require feat support in stdlib first. requires version-as-feat support and markup.

            if inbound.TokenCount < 2 {
                parser.report(inbound.SourceLine, "Malformed REQUIRE statement.")
                finish(true, ERR_SYNTAX)
                break
            }

            var reqfeat string
            var reqvers int
            var reqEnd bool

            switch inbound.TokenCount {
            case 2: // only by name
                reqfeat = inbound.Tokens[1].tokText
            case 3: // name + version
                reqfeat = inbound.Tokens[1].tokText
                reqvers, _ = strconv.Atoi(inbound.Tokens[2].tokText)
            default: // check for semver
                required := crushEvalTokens(inbound.Tokens[1:]).text
                required = str.Replace(required, " ", "", -1)
                _, e := vconvert(required)
                if e == nil {
                    // sem ver provided / compare to language version
                    lver, _ := gvget("@version")
                    lcmp, _ := vcmp(lver.(string), required)
                    if lcmp == -1 { // lang ver is lower than required ver
                        // error
                        pf("Language version of '%s' is too low (%s<%s). Quitting.\n", lver, lver, required)
                        finish(true, ERR_REQUIRE)
                    }
                    reqEnd = true
                }
            }

            if !reqEnd {
                if _, ok := features[reqfeat]; ok {
                    // feature exists
                    if features[reqfeat].version < reqvers {
                        // version too low
                        pf("Library version of '%s' is too low (%d<%d). Quitting.\n", reqfeat, features[reqfeat].version, reqvers)
                        finish(true, ERR_REQUIRE)
                    }
                } else {
                    pf("Library does not contain feature '%s'.\n", reqfeat)
                    finish(true, ERR_REQUIRE)
                }
            }

        case C_Version:
            version()

        case C_Exit:
            if inbound.TokenCount > 1 {
                resu, errs := parser.evalCommaArray(ifs, inbound.Tokens[1:])
                errmsg := ""
                if len(resu) > 1 && errs[1] == nil {
                    switch resu[1].(type) {
                    case string:
                        resu[1] = interpolate(currentModule, ifs, ident, resu[1].(string))
                        errmsg = sf("%v\n", resu[1])
                    }
                }
                if len(resu) > 0 && errs[0] == nil {
                    ec := resu[0]
                    pf(errmsg)
                    if isNumber(ec) {
                        finish(true, ec.(int))
                    } else {
                        parser.report(inbound.SourceLine, "Could not evaluate your EXIT expression")
                        finish(true, ERR_EVAL)
                    }
                }
            } else {
                finish(true, 0)
            }

        case C_Define:

            if inbound.TokenCount > 1 {

                if defining {
                    parser.report(inbound.SourceLine, "Already defining a function. Nesting not permitted.")
                    finish(true, ERR_SYNTAX)
                    break
                }

                defining = true
                definitionName = parser.namespace + "::" + inbound.Tokens[1].tokText

                parent := ""
                if structMode {
                    parent = structName
                    definitionName += "~" + parent
                }

                // pf("[#4]Now defining %s[#-]\n",definitionName)

                // loc, _ := GetNextFnSpace(true, definitionName, call_s{prepared: false})
                loc, _ := GetNextFnSpace(true, definitionName, call_s{prepared: true, base: source_base, caller: ifs, gc: false, gcShyness: 100})
                var dargs []string
                var argTypes []string
                var hasDefault []bool
                var defaults []any
                var returnTypes []string
                hasReturnTypes := false

                if inbound.TokenCount > 2 {
                    // process tokens directly (no string splitting!)
                    tokens := inbound.Tokens[2:]

                    // Check for -> return type(s) FIRST, before removing parens
                    paramTokens := tokens
                    mapPos := -1
                    for i, tok := range tokens {
                        if tok.tokType == O_Map {
                            mapPos = i
                            break
                        }
                    }
                    if mapPos != -1 {
                        paramTokens = tokens[:mapPos]
                        returnTokens := tokens[mapPos+1:]
                        hasReturnTypes = true
                        returnTypes = parseReturnTypes(returnTokens)
                    }

                    // Now remove outer parens from parameter tokens if present
                    if len(paramTokens) > 0 && paramTokens[0].tokType == LParen && paramTokens[len(paramTokens)-1].tokType == RParen {
                        paramTokens = paramTokens[1 : len(paramTokens)-1]
                    }

                    var currentArgTokens []Token
                    for _, tok := range paramTokens {
                        if tok.tokType == O_Comma {
                            parser.processArgumentTokens(currentArgTokens, &dargs, &argTypes, &hasDefault, &defaults, loc, ifs, ident)
                            currentArgTokens = nil
                        } else {
                            currentArgTokens = append(currentArgTokens, tok)
                        }
                    }
                    // process the final argument
                    if len(currentArgTokens) > 0 {
                        parser.processArgumentTokens(currentArgTokens, &dargs, &argTypes, &hasDefault, &defaults, loc, ifs, ident)
                    }
                }

                // error if it clashes with a stdlib name
                exMatchStdlib := false
                for n, _ := range slhelp {
                    if n == definitionName {
                        parser.report(inbound.SourceLine, "A library function already exists with the name '"+definitionName+"'")
                        finish(false, ERR_SYNTAX)
                        exMatchStdlib = true
                        break
                    }
                }
                if exMatchStdlib {
                    break
                }

                // register new func in funcmap
                funcmap[definitionName] = Funcdef{
                    name:   definitionName,
                    module: parser.namespace,
                    fs:     loc,
                    parent: parent,
                }

                basemodmap[loc] = parser.namespace
                sourceMap[loc] = source_base // relate defined base 'loc' to parent 'ifs' instance's 'base' source

                fspacelock.Lock()
                functionspaces[loc] = []Phrase{}
                basecode[loc] = []BaseCode{}
                fspacelock.Unlock()

                farglock.Lock()
                functionArgs[loc].args = dargs
                functionArgs[loc].argTypes = argTypes
                functionArgs[loc].hasDefault = hasDefault
                functionArgs[loc].defaults = defaults
                functionArgs[loc].returnTypes = returnTypes
                functionArgs[loc].hasReturnTypes = hasReturnTypes
                farglock.Unlock()

                // pf("defining new function %s (%d)\n",definitionName,loc)

            }

        case C_Showdef:

            if inbound.TokenCount == 2 {

                searchTerm := inbound.Tokens[1].tokText
                if val, found := modlist[searchTerm]; found {
                    if val == true {
                        pf("[#5]Module %s : Functions[#-]\n", searchTerm)
                        for _, fun := range funcmap {
                            if fun.module == searchTerm {
                                ShowDef(fun.name)
                            }
                        }
                    }
                } else {
                    fn := stripOuterQuotes(inbound.Tokens[1].tokText, 2)
                    fn = interpolate(currentModule, ifs, ident, fn)
                    if _, exists := fnlookup.lmget(fn); exists {
                        ShowDef(fn)
                    } else {
                        parser.report(inbound.SourceLine, "Module/function not found.")
                        finish(false, ERR_EVAL)
                    }
                }

            } else {

                fnlookup.m.Range(func(key, value interface{}) bool {
                    name := key.(string)
                    count := value.(uint32)
                    if count < 2 {
                        return true // continue
                    }
                    ShowDef(name)
                    return true // keep iterating
                })
                pf("\n")

                /*
                   for oq := range fnlookup.smap {
                       if fnlookup.smap[oq] < 2 {
                       continue
                       } // don't show global or main
                       ShowDef(oq)
                   }
                   pf("\n")
                */

            }

        case C_Return:

            // split return args by comma in evaluable lumps
            var rargs = make([][]Token, 1)
            var curArg uint8
            evnest := 0
            argtoks := inbound.Tokens[1:]

            rargs[0] = make([]Token, 0)
            ppos := 0
            for tok := range argtoks {
                nt := argtoks[tok]
                if nt.tokType == LParen {
                    evnest += 1
                }
                if nt.tokType == RParen {
                    evnest -= 1
                }
                if nt.tokType == LeftSBrace {
                    evnest += 1
                }
                if nt.tokType == RightSBrace {
                    evnest -= 1
                }
                if evnest == 0 && (tok == len(argtoks)-1 || nt.tokType == O_Comma) {
                    rargs[curArg] = argtoks[ppos : tok+1]
                    ppos = tok + 1
                    curArg += 1
                    if int(curArg) >= len(rargs) {
                        rargs = append(rargs, []Token{})
                    }
                }
            }
            retval_count = curArg
            // pf("call() %d : args -> [%+v]\n",ifs,rargs)

            // tail call recursion handling:
            if inbound.TokenCount > 2 {

                var bname string
                bname, _ = numlookup.lmget(source_base)
                //pf("[bname:%s,toktext:%s,current:%s]",bname,inbound.Tokens[1].tokText,currentModule)
                tco_check := false // deny tco until we check all is well

                if inbound.Tokens[1].tokType == Identifier && inbound.Tokens[2].tokType == LParen {
                    if strcmp(currentModule+"::"+inbound.Tokens[1].tokText, bname) {
                        rbraceAt := findDelim(inbound.Tokens, RParen, 2)
                        // pf("[rb@%d,tokcount:%d]",rbraceAt,inbound.TokenCount)
                        if rbraceAt == inbound.TokenCount-1 {
                            tco_check = true
                        }
                    }
                }

                if tco_check {
                    skip_reentry := false
                    resu, errs := parser.evalCommaArray(ifs, rargs[0][2:len(rargs[0])-1])
                    // populate var args for re-entry. should check errs here too...
                    for q := 0; q < len(errs); q += 1 {
                        va[q] = resu[q]
                        if errs[q] != nil {
                            skip_reentry = true
                            break
                        }
                    }
                    // no args/wrong arg count check
                    if len(errs) != len(va) {
                        skip_reentry = true
                    }

                    // set tco flag if required, and perform.
                    if !skip_reentry {
                        wccount = 0
                        depth = 0
                        parser.pc = -1
                        goto tco_reentry
                    }
                }
            }

            // evaluate each expr and stuff the results in an array
            var ev_er error
            retvalues = make([]any, curArg)
            for q := 0; q < int(curArg); q += 1 {
                retvalues[q], ev_er = parser.Eval(ifs, rargs[q])
                if ev_er != nil {
                    parser.report(inbound.SourceLine, "Could not evaluate RETURN arguments")
                    finish(true, ERR_EVAL)
                    break
                }
            }

            // Validate return types if specified
            farglock.RLock()
            if functionArgs[source_base].hasReturnTypes {
                expectedTypes := functionArgs[source_base].returnTypes
                if len(expectedTypes) != int(retval_count) {
                    farglock.RUnlock()
                    parser.report(inbound.SourceLine, sf("Return count mismatch: expected %d value(s), got %d", len(expectedTypes), retval_count))
                    finish(false, ERR_SYNTAX)
                    break
                }
                for i, retValue := range retvalues {
                    if expectedTypes[i] != "" && !isCompatibleType(retValue, expectedTypes[i], currentModule) {
                        farglock.RUnlock()
                        parser.report(inbound.SourceLine, sf("Return type mismatch at position %d: expected %s, got %T", i+1, expectedTypes[i], retValue))
                        finish(false, ERR_SYNTAX)
                        break
                    }
                }
            }
            farglock.RUnlock()
            // pf("call() #%d : rv -> [%+v]\n",ifs,retvalues)

            // If we're in a try block and executing a return, pack the return values
            // with EXCEPTION_RETURN status for the parent function to handle
            if isTryBlock {
                // Pack return values: [EXCEPTION_RETURN, retval1, retval2, ...]
                packedRetvals := make([]any, 1+int(retval_count))
                packedRetvals[0] = EXCEPTION_RETURN
                for i := 0; i < int(retval_count); i++ {
                    packedRetvals[i+1] = retvalues[i]
                }
                retvalues = packedRetvals
                retval_count = uint8(len(packedRetvals))
            }

            endFunc = true
            break

        case C_Enddef:

            if !defining {
                parser.report(inbound.SourceLine, "Not currently defining a function.")
                finish(false, ERR_SYNTAX)
                break
            }

            defining = false
            definitionName = ""
            // pf("defined new function %s.\n",definitionName)

        case C_Input:

            // INPUT <id> <type> <position> [<hint>]
            // - set variable {id} from external value or exits.

            // get C_Input arguments

            if inbound.TokenCount < 4 {
                usage := "INPUT [#i1]id[#i0] PARAM | OPTARG [#i1]field_position[#i0] [ IS [#i1]error_hint[#i0] ]\n"
                usage += "INPUT [#i1]id[#i0] ENV [#i1]env_name[#i0]"
                parser.report(inbound.SourceLine, "Incorrect arguments supplied to INPUT.\n"+usage)
                finish(false, ERR_SYNTAX)
                break
            }

            id := inbound.Tokens[1].tokText
            typ := inbound.Tokens[2].tokText
            pos := inbound.Tokens[3].tokText

            bin := bind_int(ifs, id)
            if bin >= uint64(len(*ident)) {
                newident := make([]Variable, bin+identGrowthSize)
                copy(newident, *ident)
                *ident = newident
            }

            hint := id
            noteAt := inbound.TokenCount

            if inbound.TokenCount > 5 { // must be something after the IS token too
                noteAt = findDelim(inbound.Tokens, C_Is, 4)
                if noteAt != -1 {
                    we = parser.wrappedEval(ifs, ident, ifs, ident, inbound.Tokens[noteAt+1:])
                    if !we.evalError {
                        hint = we.result.(string)
                    }
                } else {
                    noteAt = inbound.TokenCount
                }
            }

            // eval

            switch str.ToLower(typ) {
            case "param":

                we = parser.wrappedEval(ifs, ident, ifs, ident, inbound.Tokens[3:noteAt])
                if we.evalError {
                    parser.report(inbound.SourceLine, sf("could not evaluate the INPUT expression\n%+v", we.errVal))
                    finish(true, ERR_EVAL)
                    break
                }
                switch we.result.(type) {
                case int:
                default:
                    parser.report(inbound.SourceLine, "INPUT expression must evaluate to an integer")
                    finish(true, ERR_EVAL)
                    break
                }
                d := we.result.(int)

                if d < 1 {
                    parser.report(inbound.SourceLine, sf("INPUT position %d too low.", d))
                    finish(true, ERR_SYNTAX)
                    break
                }
                if d <= len(cmdargs) {

                    // remove any numSeps from literal, range is a copy of numSeps from lex.go
                    tryN := cmdargs[d-1]
                    for _, ns := range "_" {
                        tryN = str.Replace(tryN, string(ns), "", -1)
                    }

                    // if this is numeric, assign as an int
                    n, er := strconv.Atoi(tryN)
                    if er == nil {
                        vset(nil, ifs, ident, id, n)
                    } else {
                        vset(nil, ifs, ident, id, cmdargs[d-1])
                    }
                } else {
                    // parser.report(inbound.SourceLine,sf("Expected CLI parameter [%s] not provided at startup.", hint))
                    pf("Expected CLI parameter %s [%s] not provided at startup.\n", id, hint)
                    finish(true, ERR_SYNTAX)
                }

            case "optarg":

                we = parser.wrappedEval(ifs, ident, ifs, ident, inbound.Tokens[3:noteAt])
                if we.evalError {
                    parser.report(inbound.SourceLine, sf("could not evaluate the INPUT expression\n%+v", we.errVal))
                    finish(false, ERR_EVAL)
                    break
                }
                switch we.result.(type) {
                case int:
                default:
                    parser.report(inbound.SourceLine, "INPUT expression must evaluate to an integer")
                    finish(false, ERR_EVAL)
                    break
                }
                d := we.result.(int)

                if d <= len(cmdargs) {

                    // remove any numSeps from literal, range is a copy of numSeps from lex.go
                    tryN := cmdargs[d-1]
                    for _, ns := range "_" {
                        tryN = str.Replace(tryN, string(ns), "", -1)
                    }

                    // if this is numeric, assign as an int
                    n, er := strconv.Atoi(tryN)
                    if er == nil {
                        vset(nil, ifs, ident, id, n)
                    } else {
                        vset(nil, ifs, ident, id, cmdargs[d-1])
                    }
                } else {
                    if !(*ident)[bin].declared {
                        // nothing provided but var didn't exist, so create it empty
                        vset(nil, ifs, ident, id, "")
                    }
                    // showIdent(ident)
                }

            case "env":

                vset(nil, ifs, ident, id, os.Getenv(pos))

                /*
                   if os.Getenv(pos)!="" {
                       // non-empty env var so set id var to value.
                       vset(nil,ifs, ident,id, os.Getenv(pos))
                   } else {
                       // when env var empty either create the id var or
                       // leave it alone if it already exists.
                       vset(nil,ifs,ident,id,"")
                   }
                */
            }

        case C_Module:

            // MODULE str_name_or_path [ AS alias_name ]

            asAt := findDelim(inbound.Tokens, C_As, 2)
            modGivenAlias := ""
            aliased := false
            hasAuto := false
            headerPaths := []string{}

            // Check for AUTO keyword
            autoAt := int16(-1)
            for i := int16(2); i < inbound.TokenCount; i++ {
                if inbound.Tokens[i].tokType == Identifier &&
                    str.ToUpper(inbound.Tokens[i].tokText) == "AUTO" {
                    autoAt = i
                    hasAuto = true
                    break
                }
            }

            // Determine where AS clause ends
            asEndAt := inbound.TokenCount
            if asAt > 1 {
                aliased = true
                // AS keyword is at asAt, alias name is at asAt+1
                if asAt+1 >= inbound.TokenCount {
                    parser.report(inbound.SourceLine, "MODULE AS requires an alias name")
                    finish(false, ERR_MODULE)
                    break
                }
                modGivenAlias = inbound.Tokens[asAt+1].tokText
                asEndAt = asAt + 2

                // If AUTO present, it should be after AS
                if hasAuto && autoAt < asEndAt {
                    parser.report(inbound.SourceLine, "AUTO clause must appear after AS clause")
                    finish(false, ERR_MODULE)
                    break
                }
            } else {
                asAt = inbound.TokenCount
                asEndAt = inbound.TokenCount
            }

            // Parse AUTO clause if present
            if hasAuto {
                // Collect string literals after AUTO keyword (explicit header paths)
                for i := autoAt + 1; i < inbound.TokenCount; i++ {
                    if inbound.Tokens[i].tokType == StringLiteral {
                        headerPaths = append(headerPaths, inbound.Tokens[i].tokText)
                        if os.Getenv("ZA_DEBUG_AUTO") != "" {
                            fmt.Printf("[AUTO] Parsed explicit header path: %s\n", inbound.Tokens[i].tokText)
                        }
                    } else if inbound.Tokens[i].tokType != O_Comma {
                        // Stop at first non-string, non-comma token
                        break
                    }
                }
                // asAt is now end of library path + AS clause, before AUTO
                if autoAt > 0 {
                    asAt = autoAt
                }
                if os.Getenv("ZA_DEBUG_AUTO") != "" {
                    fmt.Printf("[AUTO] Parsed %d explicit header paths from AUTO clause\n", len(headerPaths))
                }
            }

            if inbound.TokenCount > 1 {
                we = parser.wrappedEval(ifs, ident, ifs, ident, inbound.Tokens[1:asAt])
                if we.evalError {
                    parser.report(inbound.SourceLine, sf("could not evaluate expression in MODULE statement\n%+v", we.errVal))
                    finish(false, ERR_MODULE)
                    break
                }
            } else {
                parser.report(inbound.SourceLine, "No module name provided.")
                finish(false, ERR_MODULE)
                break
            }

            modGivenPath := we.result.(string)

            if strcmp(modGivenPath, "") {
                parser.report(inbound.SourceLine, "Empty module name provided.")
                finish(false, ERR_MODULE)
                break
            }

            // Check if this is a C library (shared object file)
            // Supports .so (Linux/BSD), .dll (Windows), .dylib (macOS)
            isSharedLib := strings.HasSuffix(modGivenPath, ".so") ||
                strings.Contains(modGivenPath, ".so.") ||
                strings.HasSuffix(modGivenPath, ".dll") ||
                strings.HasSuffix(modGivenPath, ".dylib")

            if isSharedLib {
                // Save current namespace before potentially changing it

                var modRealAlias string
                if aliased {
                    modRealAlias = modGivenAlias
                    currentModule = modRealAlias
                } else {
                    currentModule = filepath.Base(modGivenPath)
                    // Remove library extensions and version suffixes
                    if strings.HasSuffix(currentModule, ".dll") {
                        currentModule = str.TrimSuffix(currentModule, ".dll")
                    } else if strings.HasSuffix(currentModule, ".dylib") {
                        currentModule = str.TrimSuffix(currentModule, ".dylib")
                    } else if strings.HasSuffix(currentModule, ".so") {
                        currentModule = str.TrimSuffix(currentModule, ".so")
                    } else {
                        // Remove everything after .so to handle versioned libs like .so.6
                        soIndex := strings.Index(currentModule, ".so")
                        if soIndex > 0 {
                            currentModule = currentModule[:soIndex]
                        }
                    }
                    // Strip common library prefixes
                    currentModule = str.TrimPrefix(currentModule, "lib")
                    modRealAlias = currentModule
                }

                // Handle C library loading - always try system paths for C libraries
                var err error
                var libPath string

                // Check if path contains directory separator (either / or \)
                hasPathSep := str.Contains(modGivenPath, "/") || str.Contains(modGivenPath, "\\")

                var lib *CLibrary
                if !hasPathSep {
                    // Use hybrid approach: LD_LIBRARY_PATH -> ldconfig -> comprehensive path search
                    var systemPaths []string

                    // 1. Check environment library path variables first (user override, highest priority)
                    if runtime.GOOS == "darwin" {
                        // macOS: Check DYLD_LIBRARY_PATH and DYLD_FALLBACK_LIBRARY_PATH
                        // Note: These are often disabled by SIP for protected binaries
                        if dyldPath := os.Getenv("DYLD_LIBRARY_PATH"); dyldPath != "" {
                            for _, dir := range str.Split(dyldPath, ":") {
                                if dir != "" {
                                    systemPaths = append(systemPaths, filepath.Join(dir, modGivenPath))
                                }
                            }
                        }
                        if dyldFallback := os.Getenv("DYLD_FALLBACK_LIBRARY_PATH"); dyldFallback != "" {
                            for _, dir := range str.Split(dyldFallback, ":") {
                                if dir != "" {
                                    systemPaths = append(systemPaths, filepath.Join(dir, modGivenPath))
                                }
                            }
                        }
                    } else if runtime.GOOS != "windows" {
                        // Unix/Linux/BSD: Check LD_LIBRARY_PATH
                        if ldPath := os.Getenv("LD_LIBRARY_PATH"); ldPath != "" {
                            for _, dir := range str.Split(ldPath, ":") {
                                if dir != "" {
                                    systemPaths = append(systemPaths, filepath.Join(dir, modGivenPath))
                                }
                            }
                        }

                        // 2. Try ldconfig if available (most accurate, respects system config)
                        // Note: BSD systems use ldconfig but with different output format
                        if ldconfigPath := tryLdconfigPath(modGivenPath); ldconfigPath != "" {
                            systemPaths = append(systemPaths, ldconfigPath)
                        }
                    }

                    // 3. Fallback to comprehensive path search (works everywhere)
                    systemPaths = append(systemPaths, getSystemLibraryPaths(modGivenPath)...)

                    // Try each path in order until one succeeds
                    for _, path := range systemPaths {
                        libPath = path
                        lib, err = LoadCLibraryWithAlias(path, currentModule)
                        if err == nil {
                            break
                        }
                    }

                    // Final fallback: try with just the library name, letting dlopen use its own search
                    if err != nil && runtime.GOOS != "windows" {
                        libPath = modGivenPath
                        lib, err = LoadCLibraryWithAlias(modGivenPath, currentModule)
                    }

                    if err != nil {
                        parser.report(inbound.SourceLine, sf("Failed to load C library '%s': %v", modGivenPath, err))
                        finish(false, ERR_MODULE)
                        break
                    }
                } else {
                    libPath = modGivenPath
                    lib, err = LoadCLibraryWithAlias(libPath, currentModule)

                    if err != nil {
                        parser.report(inbound.SourceLine, sf("Failed to load C library '%s': %v", modGivenPath, err))
                        finish(false, ERR_MODULE)
                        break
                    }
                }

                // Store library reference for help system
                if lib != nil {
                    loadedCLibraries[currentModule] = lib
                }

                // Discover symbols in the C library
                existingLib, _ := loadedCLibraries[currentModule]
                symbols, err := DiscoverSymbolsWithAlias(libPath, currentModule, existingLib)
                if err != nil {
                    parser.report(inbound.SourceLine, sf("Failed to discover C library symbols: %v", err))
                    finish(false, ERR_MODULE)
                    break
                }
                // Register all discovered symbols
                for _, symbol := range symbols {
                    RegisterCSymbol(symbol)
                }

                // Add C library to use chain for namespace resolution
                uc_add(currentModule)

                // Parse header files if AUTO clause was specified
                if hasAuto {
                    // Check if we already have a function space for this C module alias
                    // This allows multiple AUTO imports to merge into the same namespace
                    // Use separate cModuleAliasMap, not basemodmap (which is for Za namespaces)
                    var loc uint32
                    foundExisting := false

                    cModuleAliasMapLock.RLock()
                    existingLoc, foundExisting := cModuleAliasMap[currentModule]
                    cModuleAliasMapLock.RUnlock()

                    if foundExisting {
                        // Reuse the existing function space
                        loc = existingLoc
                    } else {
                        // Allocate a new permanent function space for this C module's constants
                        // This allows constants to be accessed via module::CONSTANT_NAME
                        loc, _ = GetNextFnSpace(true, currentModule, call_s{prepared: false})

                        calllock.Lock()
                        fspacelock.Lock()
                        functionspaces[loc] = []Phrase{}
                        basecode[loc] = []BaseCode{}
                        fspacelock.Unlock()

                        farglock.Lock()
                        functionArgs[loc].args = []string{}
                        farglock.Unlock()

                        // Setup call_s entry for this module
                        modcs := call_s{}
                        modcs.base = loc
                        modcs.caller = ifs
                        modcs.fs = currentModule
                        calltable[loc] = modcs
                        calllock.Unlock()

                        // Store in C module alias map (NOT basemodmap)
                        cModuleAliasMapLock.Lock()
                        cModuleAliasMap[currentModule] = loc
                        cModuleAliasMapLock.Unlock()
                    }

                    // Parse headers using the module's dedicated function space
                    if err := parseModuleHeaders(libPath, currentModule, headerPaths, loc); err != nil {
                        parser.report(inbound.SourceLine, sf("AUTO clause failed: %v\n\nSolution: Specify explicit header path:\n  module \"%s\" as %s auto \"/path/to/header.h\"",
                            err, libPath, currentModule))
                        finish(false, ERR_MODULE)
                        break
                    }

                    // Note: Unlike normal Za modules, we don't trigger GC or tear down this function space
                    // The constants need to persist for access via module::CONSTANT_NAME
                }

                modlist[currentModule] = true

                // pf("C library '%s' loaded with %d symbols", currentModule, len(symbols))
            } else {

                //.. set file location

                moduleloc = ""

            if str.IndexByte(modGivenPath, '/') > -1 {
                if filepath.IsAbs(modGivenPath) {
                    moduleloc = modGivenPath
                } else {
                    mdir, _ := gvget("@execpath")
                    moduleloc = mdir.(string) + "/" + modGivenPath
                }
            } else {

                // modules default path is $HOME/.za/modules
                //  unless otherwise redefined in environmental variable ZA_MODPATH

                modhome, _ := gvget("@home")
                modhome = modhome.(string) + "/.za"
                if os.Getenv("ZA_MODPATH") != "" {
                    modhome = os.Getenv("ZA_MODPATH")
                }

                moduleloc = modhome.(string) + "/modules/" + modGivenPath + ".fom"

            }

            //.. validate module exists
            f, err := os.Stat(moduleloc)
            if err != nil {
                parser.report(inbound.SourceLine, sf("Module is not accessible. (path:%v)", moduleloc))
                finish(false, ERR_MODULE)
                break
            }
            if !f.Mode().IsRegular() {
                parser.report(inbound.SourceLine, "Module is not a regular file.")
                finish(false, ERR_MODULE)
                break
            }

            //.. read in file
            mod, err := ioutil.ReadFile(moduleloc)
            if err != nil {
                parser.report(inbound.SourceLine, "Problem reading the module file.")
                finish(false, ERR_MODULE)
                break
            }

            // override module name with alias at this point, if provided
            oldModule := parser.namespace
            modRealAlias := modGivenPath
            if aliased {
                modRealAlias = modGivenAlias
                currentModule = modRealAlias
            } else {
                currentModule = path.Base(modGivenPath)
                currentModule = str.TrimSuffix(currentModule, ".mod")
                modRealAlias = currentModule
            }

            // tokenise and parse into a new function space.

            //.. error if it has already been defined
            //pf("DEBUG: permit_dupmod check with alias : %s\n",modRealAlias)
            //pf("DEBUG: -- current filemap list:\n")
            is_present := false
            fileMap.Range(func(k, v any) bool {
                // pf("  : %v (ifs:%d)  ",v,k)
                if v.(string) == moduleloc {
                    is_present = true
                    // pf("<--")
                    return false
                }
                // pf("\n")
                return true
            })

            if is_present {
                if !permit_dupmod {
                    // pf("DEBUG: -- inside !permit\n")
                    parser.report(inbound.SourceLine, "Module file "+moduleloc+" already processed once.")
                    finish(false, ERR_SYNTAX)
                    break
                } else {
                    // pf("DEBUG: -- inside permit\n")
                    // just continue, module already exists
                    // pf("DEBUG: duplicate module import for %s\n",moduleloc)
                }
            }

            if !is_present {

                loc, _ := GetNextFnSpace(true, modRealAlias, call_s{prepared: false})

                calllock.Lock()

                fspacelock.Lock()
                functionspaces[loc] = []Phrase{}
                basecode[loc] = []BaseCode{}
                fspacelock.Unlock()

                farglock.Lock()
                functionArgs[loc].args = []string{}
                farglock.Unlock()

                modlist[currentModule] = true

                /*
                   pf("(module) aliased -> %v\n",aliased)
                   pf("(module) alias   -> %s\n",modRealAlias)
                   pf("(module) given   -> %s\n",modGivenPath)
                   pf("(module) cmod    -> %s\n",currentModule)
                   pf("(module) omod    -> %s\n",oldModule)
                */

                //.. parse and execute
                basemodmap[loc] = modRealAlias

                if debugMode {
                    start := time.Now()
                    phraseParse(parser.ctx, modRealAlias, string(mod), 0, 0)
                    elapsed := time.Since(start)
                    pf("(timings-module) elapsed in mod translation for '%s' : %v\n", modRealAlias, elapsed)
                } else {
                    phraseParse(parser.ctx, modRealAlias, string(mod), 0, 0)
                }
                modcs := call_s{}
                modcs.base = loc
                modcs.caller = ifs
                modcs.fs = modRealAlias
                calltable[loc] = modcs
                calllock.Unlock()

                fileMap.Store(loc, moduleloc)

                var modident = make([]Variable, identInitialSize)

                // Set the callLine field in the calltable entry before calling the function
                // For module calls, we use the source line from the inbound phrase
                atomic.StoreInt32(&calltable[loc].callLine, int32(inbound.SourceLine))

                if debugMode {
                    start := time.Now()
                    Call(ctx, MODE_NEW, &modident, loc, ciMod, false, nil, "", []string{}, nil)
                    elapsed := time.Since(start)
                    pf("(timings-module) elapsed in mod execution for '%s' : %v\n", modRealAlias, elapsed)
                } else {
                    Call(ctx, MODE_NEW, &modident, loc, ciMod, false, nil, "", []string{}, nil)
                }

                calllock.Lock()
                calltable[ifs].gcShyness = 20
                calltable[ifs].gc = true
                calllock.Unlock()
                currentModule = oldModule
                parser.namespace = oldModule

            }

            } // end else (Za module loading)

        case C_Lib:
            // LIB namespace::function(param1:type, param2:type) -> return_type
            // TODO: Refactor to parse tokens directly instead of string reconstruction
            // (see Call/functionargs parsing for reference)

            if inbound.TokenCount < 2 {
                parser.report(inbound.SourceLine, "LIB requires a function signature")
                finish(false, ERR_SYNTAX)
                break
            }

            // Reconstruct the full signature from tokens (everything after LIB keyword)
            var sigBuilder strings.Builder
            for i := int16(1); i < inbound.TokenCount; i++ {
                if i > 1 {
                    sigBuilder.WriteString(" ")
                }
                sigBuilder.WriteString(inbound.Tokens[i].tokText)
            }
            signature := sigBuilder.String()

            // Remove spaces around special characters for easier parsing
            signature = strings.ReplaceAll(signature, " :: ", "::")
            signature = strings.ReplaceAll(signature, " ( ", "(")
            signature = strings.ReplaceAll(signature, " ) ", ")")
            signature = strings.ReplaceAll(signature, " : ", ":")
            signature = strings.ReplaceAll(signature, " , ", ",")
            signature = strings.ReplaceAll(signature, " -> ", "->")
            // Fix varargs: ". . ." or ".. ." -> "..."
            signature = strings.ReplaceAll(signature, ". . .", "...")
            signature = strings.ReplaceAll(signature, ".. .", "...")
            signature = strings.ReplaceAll(signature, ". ..", "...")

            // Parse: namespace::function(param1:type, param2:type) -> return_type

            // Find the :: separator
            nsIdx := strings.Index(signature, "::")
            if nsIdx == -1 {
                parser.report(inbound.SourceLine, "LIB signature must include namespace prefix (e.g., c::malloc)")
                finish(false, ERR_SYNTAX)
                break
            }

            libAlias := signature[:nsIdx]
            remaining := signature[nsIdx+2:]

            // Find the opening parenthesis
            parenIdx := strings.Index(remaining, "(")
            if parenIdx == -1 {
                parser.report(inbound.SourceLine, "LIB signature must include parameter list in parentheses")
                finish(false, ERR_SYNTAX)
                break
            }

            funcName := remaining[:parenIdx]

            // Find the closing parenthesis
            closeParenIdx := strings.Index(remaining, ")")
            if closeParenIdx == -1 {
                parser.report(inbound.SourceLine, "LIB signature has unclosed parameter list")
                finish(false, ERR_SYNTAX)
                break
            }

            paramsStr := remaining[parenIdx+1 : closeParenIdx]
            afterParams := remaining[closeParenIdx+1:]

            // Parse return type (after ->)
            returnTypeStr := "void"
            if strings.Contains(afterParams, "->") {
                arrowIdx := strings.Index(afterParams, "->")
                returnTypeStr = strings.TrimSpace(afterParams[arrowIdx+2:])
            }

            returnType, returnStructName, err := StringToCType(returnTypeStr)
            if err != nil {
                parser.report(inbound.SourceLine, sf("Invalid return type '%s': %v", returnTypeStr, err))
                finish(false, ERR_SYNTAX)
                break
            }

            // Parse parameters
            var paramTypes []CType
            var paramStructNames []string
            hasVarargs := false
            if paramsStr != "" {
                params := strings.Split(paramsStr, ",")
                for i, param := range params {
                    param = strings.TrimSpace(param)

                    // Check for varargs (...name)
                    if strings.HasPrefix(param, "...") {
                        if i != len(params)-1 {
                            parser.report(inbound.SourceLine, "Varargs parameter (...) must be last parameter")
                            finish(false, ERR_SYNTAX)
                            break
                        }
                        hasVarargs = true
                        // Don't add varargs to paramTypes - they're handled dynamically
                        break
                    }

                    // Extract type from "name:type" format
                    colonIdx := strings.Index(param, ":")
                    if colonIdx == -1 {
                        parser.report(inbound.SourceLine, sf("Parameter '%s' must have format name:type", param))
                        finish(false, ERR_SYNTAX)
                        break
                    }
                    typeStr := strings.TrimSpace(param[colonIdx+1:])
                    paramType, structName, err := StringToCType(typeStr)
                    if err != nil {
                        parser.report(inbound.SourceLine, sf("Invalid parameter type '%s': %v", typeStr, err))
                        finish(false, ERR_SYNTAX)
                        break
                    }
                    paramTypes = append(paramTypes, paramType)
                    paramStructNames = append(paramStructNames, structName)
                }
            }

            // Store the function signature
            DeclareCFunction(libAlias, funcName, paramTypes, paramStructNames, returnType, returnStructName, hasVarargs)

        case C_Case:

            // need to store the condition and result for the is/contains/has/or clauses
            // endcase location should be calculated in advance for a direct jump to exit

            if wccount == CASE_CAP {
                parser.report(inbound.SourceLine, sf("maximum CASE nesting reached (%d)", CASE_CAP))
                finish(true, ERR_SYNTAX)
                break
            }

            // lookahead
            endfound, enddistance, er := lookahead(source_base, parser.pc, 0, 0, C_Endcase, []int64{C_Case}, []int64{C_Endcase})

            if er {
                parser.report(inbound.SourceLine, "Lookahead dedent error!")
                finish(true, ERR_SYNTAX)
                break
            }

            if !endfound {
                parser.report(inbound.SourceLine, "Missing ENDCASE for this CASE. Maybe check for open quotes or braces in block?")
                finish(false, ERR_SYNTAX)
                break
            }

            // Check for CASE expr full syntax (exhaustive enum matching)
            isExhaustive := false
            exprEnd := inbound.TokenCount

            if inbound.TokenCount > 2 {
                lastTok := inbound.Tokens[inbound.TokenCount-1]
                if lastTok.tokType == Identifier && lastTok.tokText == "full" {
                    isExhaustive = true
                    exprEnd = inbound.TokenCount - 1
                }
            }

            // Evaluate expression (excluding "full" if present)
            if exprEnd > 1 {
                we = parser.wrappedEval(ifs, ident, ifs, ident, inbound.Tokens[1:exprEnd])
                if we.evalError {
                    parser.report(inbound.SourceLine, sf("could not evaluate the CASE condition\n%+v", we.errVal))
                    finish(false, ERR_EVAL)
                    break
                }
            }

            // create storage for CASE details and increase the nesting level

            if exprEnd == 1 {
                we.result = true
            }

            // Get enum context from WITH ENUM block (if active)
            enumName := ""
            if parser.inside_with_enum {
                enumName = parser.namespace + "::" + parser.with_enum_name
            }

            wccount += 1
            wc[wccount] = caseCarton{
                endLine:        parser.pc + enddistance,
                value:          we.result,
                performed:      false,
                dodefault:      true,
                isExhaustive:   isExhaustive,
                enumName:       enumName,
                coveredMembers: []string{},
            }
            depth += 1
            lastConstruct = append(lastConstruct, C_Case)

        case C_Is, C_Has, C_Contains, C_Or:

            if lastConstruct[len(lastConstruct)-1] != C_Case {
                parser.report(inbound.SourceLine, "Not currently in a CASE block.")
                finish(false, ERR_SYNTAX)
                break
            }

            carton := wc[wccount]

            // For C_Or with exhaustive mode: error
            if statement == C_Or && carton.isExhaustive {
                parser.report(inbound.SourceLine, "CASE 'full' modifier cannot be used with OR clause")
                finish(false, ERR_SYNTAX)
                break
            }

            // Track enum members for exhaustive checking FIRST (before performed check)
            // This ensures we track all IS clauses in source code, not just executed ones
            if statement == C_Is && carton.isExhaustive {
                memberName := ""
                detectedEnum := ""

                if parser.inside_with_enum {
                    // Inside WITH ENUM: IS clause has just member name
                    if inbound.TokenCount == 2 {
                        memberName = inbound.Tokens[1].tokText
                        detectedEnum = parser.namespace + "::" + parser.with_enum_name
                    }
                } else {
                    // Outside WITH: IS clause has EnumName.member pattern
                    // Look for pattern: IS Identifier DOT Identifier
                    if inbound.TokenCount >= 4 && inbound.Tokens[2].tokType == SYM_DOT {
                        detectedEnum = parser.namespace + "::" + inbound.Tokens[1].tokText
                        memberName = inbound.Tokens[3].tokText
                    }
                }

                if memberName != "" {
                    // Set or validate enum name
                    if carton.enumName == "" {
                        carton.enumName = detectedEnum
                    } else if carton.enumName != detectedEnum {
                        parser.report(inbound.SourceLine,
                            sf("Mixed enums in exhaustive CASE: expected '%s', got '%s'",
                                carton.enumName, detectedEnum))
                        finish(false, ERR_SYNTAX)
                        break
                    }

                    // Validate member exists and track coverage
                    globlock.RLock()
                    if enumDef, exists := enum[carton.enumName]; exists {
                        if _, memberExists := enumDef.members[memberName]; memberExists {
                            carton.coveredMembers = append(carton.coveredMembers, memberName)
                            wc[wccount] = carton
                        } else {
                            globlock.RUnlock()
                            parser.report(inbound.SourceLine,
                                sf("'%s' is not a member of enum '%s'", memberName, carton.enumName))
                            finish(false, ERR_SYNTAX)
                            break
                        }
                    }
                    globlock.RUnlock()
                }
            }

            if carton.performed {
                // already matched and executed a CASE case so jump to ENDCASE
                parser.pc = carton.endLine - 1
                break
            }

            if inbound.TokenCount > 1 { // inbound.TokenCount==1 for C_Or
                we = parser.wrappedEval(ifs, ident, ifs, ident, inbound.Tokens[1:])
                if we.evalError {
                    parser.report(inbound.SourceLine, sf("could not evaluate expression in CASE condition\n%+v", we.errVal))
                    finish(false, ERR_EVAL)
                    break
                }
            }

            ramble_on := false // assume we'll need to skip to next case clause

            // pf("case-eval: checking type : %s\n%#v\n",tokNames[statement.tokType],carton)

            switch statement {

            case C_Has: // <-- @note: this may change yet

                // build expression from rest, ignore initial condition
                switch we.result.(type) {
                case bool:
                    if we.result.(bool) { // HAS truth
                        wc[wccount].performed = true
                        wc[wccount].dodefault = false
                        ramble_on = true
                    }
                default:
                    parser.report(inbound.SourceLine, sf("HAS condition did not result in a boolean\n%+v", we.errVal))
                    finish(false, ERR_EVAL)
                }

            case C_Is:
                if we.result == carton.value { // matched IS value
                    wc[wccount].performed = true
                    wc[wccount].dodefault = false
                    ramble_on = true
                }

            case C_Contains:
                reg := sparkle(we.result.(string))
                switch carton.value.(type) {
                case string:
                    if matched, _ := regexp.MatchString(reg, carton.value.(string)); matched { // matched CONTAINS regex
                        wc[wccount].performed = true
                        wc[wccount].dodefault = false
                        ramble_on = true
                    }
                case int:
                    if matched, _ := regexp.MatchString(reg, strconv.Itoa(carton.value.(int))); matched { // matched CONTAINS regex
                        wc[wccount].performed = true
                        wc[wccount].dodefault = false
                        ramble_on = true
                    }
                }

            case C_Or: // default

                if !carton.dodefault {
                    parser.pc = carton.endLine - 1
                    ramble_on = false
                } else {
                    ramble_on = true
                }

            }

            var loc int16

            // jump to the next clause, continue to next line or skip to end.

            if ramble_on { // move on to next parser.pc statement
            } else {
                // skip to next CASE clause:
                hasfound, hasdistance, _ := lookahead(source_base, parser.pc+1, 0, 0, C_Has, []int64{C_Case}, []int64{C_Endcase})
                isfound, isdistance, _ := lookahead(source_base, parser.pc+1, 0, 0, C_Is, []int64{C_Case}, []int64{C_Endcase})
                orfound, ordistance, _ := lookahead(source_base, parser.pc+1, 0, 0, C_Or, []int64{C_Case}, []int64{C_Endcase})
                cofound, codistance, _ := lookahead(source_base, parser.pc+1, 0, 0, C_Contains, []int64{C_Case}, []int64{C_Endcase})

                // add jump distances to list
                distList := []int16{}
                if cofound {
                    distList = append(distList, codistance)
                }
                if hasfound {
                    distList = append(distList, hasdistance)
                }
                if isfound {
                    distList = append(distList, isdistance)
                }
                if orfound {
                    distList = append(distList, ordistance)
                }

                /* // debug
                   pf("case-distlist: %#v\n",distList)
                   pf("case-hasfound,hasdistance: %v,%v\n",hasfound,hasdistance)
                   pf("case-isfound,isdistance: %v,%v\n",isfound,isdistance)
                   pf("case-cofound,codistance: %v,%v\n",cofound,codistance)
                   pf("case-orfound,ordistance: %v,%v\n",orfound,ordistance)
                */

                if !(isfound || hasfound || orfound || cofound) {
                    // must be an endcase
                    loc = carton.endLine
                    // pf("@%d : direct jump to endcase at %d\n",parser.pc,loc+1)
                } else {
                    loc = parser.pc + min_int16(distList) + 1
                    // pf("@%d : direct jump from distList to %d\n",parser.pc,loc+1)
                }

                // jump to nearest following clause
                parser.pc = loc - 1
            }

        case C_Endcase:

            // if forceEnd { pf("ENDCASE force flag\n") }
            if !forceEnd && lastConstruct[len(lastConstruct)-1] != C_Case {
                parser.report(inbound.SourceLine, "Not currently in a CASE block.")
                finish(false, ERR_SYNTAX)
                break
            }

            // Check exhaustiveness if required
            carton := wc[wccount]
            if carton.isExhaustive && carton.enumName != "" {
                globlock.RLock()
                if enumDef, exists := enum[carton.enumName]; exists {
                    // Build set of covered members
                    coveredSet := make(map[string]bool)
                    for _, member := range carton.coveredMembers {
                        coveredSet[member] = true
                    }

                    // Check all enum members are covered
                    var missing []string
                    for memberName := range enumDef.members {
                        if !coveredSet[memberName] {
                            missing = append(missing, memberName)
                        }
                    }

                    if len(missing) > 0 {
                        globlock.RUnlock()
                        parser.report(inbound.SourceLine,
                            sf("Non-exhaustive CASE: missing enum members: %v", missing))
                        finish(false, ERR_SYNTAX)
                        break
                    }
                }
                globlock.RUnlock()
            }

            breakIn = Error
            forceEnd = false
            lastConstruct = lastConstruct[:depth-1]
            depth -= 1
            wc[wccount] = caseCarton{}
            wccount -= 1

            if break_count > 0 {
                break_count -= 1
                if break_count > 0 {
                    switch lastConstruct[depth-1] {
                    case C_For, C_Foreach, C_While, C_Case:
                        breakIn = lastConstruct[depth-1]
                    }
                }
                // pf("ENDCASE-BREAK: bc %d\n",break_count)
            }

            if wccount < 0 {
                parser.report(inbound.SourceLine, "Cannot reduce CASE stack below zero.")
                finish(false, ERR_SYNTAX)
            }

        case C_Struct:

            // STRUCT name
            // start structmode
            // consume identifiers sequentially, adding each to definition.
            // Format:
            // STRUCT name
            // name type [ = default_value ]
            // ...
            // ENDSTRUCT

            if structMode {
                parser.report(inbound.SourceLine, "Cannot nest a STRUCT")
                finish(false, ERR_SYNTAX)
                break
            }

            if inbound.TokenCount != 2 {
                parser.report(inbound.SourceLine, "STRUCT must contain a name.")
                finish(false, ERR_SYNTAX)
                break
            }

            structName = parser.namespace + "::" + inbound.Tokens[1].tokText
            structMode = true

        case C_Endstruct:

            // ENDSTRUCT
            // end structmode

            if !structMode {
                parser.report(inbound.SourceLine, "ENDSTRUCT without STRUCT.")
                finish(false, ERR_SYNTAX)
                break
            }

            //
            // take definition and create a structmaps entry from it:
            structmapslock.Lock()
            structmaps[structName] = structNode[:]
            structmapslock.Unlock()

            structName = ""
            structNode = []any{}
            structMode = false

        case C_Showstruct:

            // SHOWSTRUCT [filter]

            var filter string

            if inbound.TokenCount > 1 {
                cet := crushEvalTokens(inbound.Tokens[1:])
                filter = interpolate(currentModule, ifs, ident, cet.text)
            }

            structmapslock.RLock()
            for k, s := range structmaps {

                if matched, _ := regexp.MatchString(filter, k); !matched {
                    continue
                }

                pf("[#6]%v[#-]\n", k)

                for i := 0; i < len(s); i += 4 {
                    pf("[#4]%24v[#-] [#3]%v[#-]\n", s[i], s[i+1])
                }
                pf("\n")

            }
            structmapslock.RUnlock()

        case C_With:

            // WITH STRUCT|ENUM name
            if inbound.TokenCount == 3 {
                with_error := false
                switch inbound.Tokens[1].tokType {
                case C_Struct:
                    if parser.inside_with_struct {
                        parser.report(inbound.SourceLine, "Already inside a WITH STRUCT")
                        finish(false, ERR_SYNTAX)
                        with_error = true
                    } else {
                        parser.inside_with_struct = true
                        parser.with_struct_name = inbound.Tokens[2].tokText
                        // pf("set with struct name to %s\n",parser.with_struct_name)
                    }
                case C_Enum:
                    if parser.inside_with_enum {
                        parser.report(inbound.SourceLine, "Already inside a WITH ENUM")
                        finish(false, ERR_SYNTAX)
                        with_error = true
                    } else {
                        parser.inside_with_enum = true
                        parser.with_enum_name = inbound.Tokens[2].tokText
                        // pf("set with enum name to %s\n",parser.with_enum_name)
                    }
                default:
                    parser.report(inbound.SourceLine, "Unknown WITH type")
                    finish(false, ERR_SYNTAX)
                    with_error = true
                }
                if with_error {
                    break
                }
                continue
            }

            // WITH var AS file
            // get params

            if inbound.TokenCount < 4 {
                parser.report(inbound.SourceLine, "Malformed WITH statement.")
                finish(false, ERR_SYNTAX)
                break
            }

            asAt := findDelim(inbound.Tokens, C_As, 2)
            if asAt == -1 {
                parser.report(inbound.SourceLine, "AS not found in WITH")
                finish(false, ERR_SYNTAX)
                break
            }

            vname := inbound.Tokens[1].tokText
            fname := crushEvalTokens(inbound.Tokens[asAt+1:]).text
            bin := inbound.Tokens[1].bindpos

            if fname == "" || vname == "" {
                parser.report(inbound.SourceLine, "Bad arguments to provided to WITH.")
                finish(false, ERR_SYNTAX)
                break
            }

            if !(*ident)[bin].declared {
                parser.report(inbound.SourceLine, sf("Variable '%s' does not exist.", vname))
                finish(false, ERR_EVAL)
                break
            }

            tfile, err := ioutil.TempFile("", "za_with_"+sf("%d", os.Getpid())+"_")
            if err != nil {
                parser.report(inbound.SourceLine, "WITH could not create a temporary file.")
                finish(true, ERR_SYNTAX)
                break
            }

            content, _ := vget(&inbound.Tokens[1], ifs, ident, vname)

            ioutil.WriteFile(tfile.Name(), []byte(content.(string)), 0600)
            vset(nil, ifs, ident, fname, tfile.Name())
            inside_with = true
            current_with_handle = tfile

            defer func() {
                remfile := current_with_handle.Name()
                current_with_handle.Close()
                current_with_handle = nil
                err := os.Remove(remfile)
                if err != nil {
                    parser.report(inbound.SourceLine, sf("WITH could not remove temporary file '%s'", remfile))
                    finish(true, ERR_FATAL)
                }
            }()

        case C_Endwith:

            if parser.inside_with_struct {
                parser.inside_with_struct = false
                parser.with_struct_name = ""
                continue
            }

            if parser.inside_with_enum {
                parser.inside_with_enum = false
                parser.with_enum_name = ""
                continue
            }

            if !inside_with {
                parser.report(inbound.SourceLine, "ENDWITH without a WITH.")
                finish(false, ERR_SYNTAX)
                break
            }

            inside_with = false

        case C_Print:
            parser.console_output(inbound.Tokens[1:], ifs, ident, inbound.SourceLine, interactive, false, false, true)

        case C_Println:
            parser.console_output(inbound.Tokens[1:], ifs, ident, inbound.SourceLine, interactive, true, false, true)

        case C_Log:
            // Check for level prefix (e.g., "debug:", "info:", etc.)
            var logLevel int = LOG_INFO // Default level
            var startToken int = 1
            var hasExplicitLevel bool = false

            if inbound.TokenCount > 2 {
                // Check for pattern: identifier + colon + message
                levelToken := inbound.Tokens[1].tokText
                if inbound.Tokens[2].tokType == SYM_COLON {
                    hasExplicitLevel = true
                    switch strings.ToLower(levelToken) {
                    case "emerg", "emergency":
                        logLevel = LOG_EMERG
                        startToken = 3
                    case "alert":
                        logLevel = LOG_ALERT
                        startToken = 3
                    case "crit", "critical":
                        logLevel = LOG_CRIT
                        startToken = 3
                    case "err", "error":
                        logLevel = LOG_ERR
                        startToken = 3
                    case "warn", "warning":
                        logLevel = LOG_WARNING
                        startToken = 3
                    case "notice":
                        logLevel = LOG_NOTICE
                        startToken = 3
                    case "info":
                        logLevel = LOG_INFO
                        startToken = 3
                    case "debug":
                        logLevel = LOG_DEBUG
                        startToken = 3
                    }
                }
            }

            // Check for exception severity override if no explicit level provided
            if !hasExplicitLevel {
                if severity, exists := getExceptionSeverity(ifs); exists {
                    logLevel = severity
                }
            }

            // Extract message from remaining tokens
            var message string
            if startToken < int(inbound.TokenCount) {
                // Build message from tokens (handle comma-separated expressions)
                var messageParts []string
                evnest := 0
                newstart := startToken
                for term := startToken; term < int(inbound.TokenCount); term++ {
                    nt := inbound.Tokens[term]
                    if nt.tokType == LParen || nt.tokType == LeftSBrace {
                        evnest += 1
                    }
                    if nt.tokType == RParen || nt.tokType == RightSBrace {
                        evnest -= 1
                    }
                    if evnest == 0 && (term == int(inbound.TokenCount)-1 || nt.tokType == O_Comma) {
                        expr, err := parser.Eval(ifs, inbound.Tokens[newstart:term+1])
                        if err != nil {
                            parser.report(inbound.SourceLine, sf("Error in LOG expression evaluation: %s", err))
                            finish(false, ERR_EVAL)
                            break
                        }
                        newstart = term + 1
                        // Handle string interpolation
                        switch expr.(type) {
                        case string:
                            expr = interpolate(parser.namespace, ifs, ident, expr.(string))
                        }
                        messageParts = append(messageParts, sf("%v", sparkle(expr)))
                    }
                }
                message = strings.Join(messageParts, "")
            } else {
                message = ""
            }

            // Create log request with level
            request := LogRequest{
                Message:     message,
                Fields:      make(map[string]any),
                IsJSON:      jsonLoggingEnabled,
                IsError:     false,
                IsWebAccess: false,
                SourceLine:  0,
                DestFile:    "",
                HTTPStatus:  0,
                Level:       logLevel,
                Timestamp:   time.Now(),
            }

            // Copy current log fields
            for k, v := range logFields {
                request.Fields[k] = v
            }

            // Handle console output (respects @silentlog setting)
            shouldPrint := true
            if v, exists := gvget("@silentlog"); exists && v != nil {
                if silent, ok := v.(bool); ok && silent {
                    shouldPrint = false
                }
            }
            if shouldPrint && logLevel <= logMinLevel {
                if jsonLoggingEnabled {
                    // Build JSON for console display
                    logEntry := make(map[string]any)
                    logEntry["message"] = message
                    logEntry["timestamp"] = time.Now().Format(time.RFC3339)
                    logEntry["level"] = logLevelToString(logLevel)

                    // Add subject if set
                    if subj, exists := gvget("@logsubject"); exists && subj != nil {
                        if subjStr, ok := subj.(string); ok && subjStr != "" {
                            logEntry["subject"] = subjStr
                        }
                    }

                    // Add custom fields
                    for k, v := range request.Fields {
                        logEntry[k] = v
                    }

                    jsonBytes, err := json.Marshal(logEntry)
                    if err == nil {
                        pf("%s\n", string(jsonBytes))
                    } else {
                        pf("%s\n", message) // Fallback to plain text
                    }
                } else {
                    // Format plain text with timestamp and level
                    timestamp := time.Now().Format(time.RFC3339)
                    levelStr := strings.ToUpper(logLevelToString(logLevel))
                    pf("%s [%s] %s\n", timestamp, levelStr, message)
                }
            }

            // Queue for file logging
            queueLogRequest(request)

        case C_Hist:

            for h, v := range hist {
                pf("%5d : %s\n", h, v)
            }

        case C_At:

            // AT row ',' column [ ',' print_expr ... ]

            commaAt := findDelim(inbound.Tokens, O_Comma, 1)

            if commaAt == -1 || commaAt == inbound.TokenCount {
                parser.report(inbound.SourceLine, "Bad delimiter in AT.")
                finish(false, ERR_SYNTAX)
            } else {

                expr_row, err := parser.Eval(ifs, inbound.Tokens[1:commaAt])
                if expr_row == nil || err != nil {
                    parser.report(inbound.SourceLine, sf("Evaluation error in %v", expr_row))
                }

                nextCommaAt := findDelim(inbound.Tokens, O_Comma, commaAt+1)
                if nextCommaAt == -1 {
                    nextCommaAt = inbound.TokenCount
                }

                expr_col, err := parser.Eval(ifs, inbound.Tokens[commaAt+1:nextCommaAt])
                if expr_col == nil || err != nil {
                    parser.report(inbound.SourceLine, sf("Evaluation error in %v", expr_col))
                }

                row, _ = GetAsInt(expr_row)
                col, _ = GetAsInt(expr_col)

                at(row, col)

                // print surplus, no LF
                if inbound.TokenCount > nextCommaAt+1 {
                    parser.console_output(inbound.Tokens[nextCommaAt+1:], ifs, ident, inbound.SourceLine, interactive, false, false, true)
                }

            }

        case C_Prompt:

            if inbound.TokenCount < 2 {
                usage := "PROMPT [#i1]storage_variable prompt_string[#i0] [ [#i1]validator_regex[#i0] ] [ TO [#i1]width[#i0] ] [ IS [#i1]def_string[#i0] ]"
                parser.report(inbound.SourceLine, "Not enough arguments for PROMPT.\n"+usage)
                finish(false, ERR_SYNTAX)
                break
            }

            // prompt variable assignment:
            if inbound.TokenCount > 1 { // um, should not do this but...
                if inbound.Tokens[1].tokType == O_Assign {
                    we = parser.wrappedEval(ifs, ident, ifs, ident, inbound.Tokens[2:])
                    if we.evalError {
                        parser.report(inbound.SourceLine, sf("could not evaluate expression prompt assignment\n%+v", we.errVal))
                        finish(false, ERR_EVAL)
                        break
                    }
                    switch we.result.(type) {
                    case string:
                        PromptTokens = make([]Token, len(inbound.Tokens)-2)
                        copy(PromptTokens, inbound.Tokens[2:])
                    }
                } else {
                    // prompt command:
                    if str.EqualFold(inbound.Tokens[1].tokText, "colour") {
                        pcol := parser.wrappedEval(ifs, ident, ifs, ident, inbound.Tokens[2:])
                        if pcol.evalError {
                            parser.report(inbound.SourceLine, "could not evaluate prompt colour")
                            finish(false, ERR_EVAL)
                            break
                        }
                        promptColour = "[#" + sf("%v", pcol.result) + "]"
                        // pf("colour is '"+promptColour+"'\n")
                    } else {
                        if inbound.TokenCount < 3 {
                            parser.report(inbound.SourceLine, "Incorrect arguments for PROMPT command.")
                            finish(false, ERR_SYNTAX)
                            break
                        } else {
                            validator := ""

                            // capture width
                            var w_okay bool
                            var providedWidth int
                            if widthAt := findDelim(inbound.Tokens, C_To, 1); widthAt != -1 {
                                expr := parser.wrappedEval(ifs, ident, ifs, ident, inbound.Tokens[widthAt+1:widthAt+2])
                                if expr.evalError {
                                    parser.report(inbound.SourceLine, "Bad width value in PROMPT command.")
                                    finish(false, ERR_EVAL)
                                    break
                                } else {
                                    providedWidth, w_okay = GetAsInt(expr.result)
                                    if w_okay {
                                        parser.report(inbound.SourceLine, "Width value is not an integer in PROMPT command.")
                                        finish(false, ERR_EVAL)
                                        break
                                    }
                                }
                            }
                            inWidth := panes[currentpane].w - 2
                            if providedWidth > 0 {
                                inWidth = providedWidth
                            }

                            // capture default string
                            defString := ""
                            defAt := findDelim(inbound.Tokens, C_Is, 1)
                            if defAt != -1 {
                                pdefault := parser.wrappedEval(ifs, ident, ifs, ident, inbound.Tokens[defAt+1:])
                                if pdefault.evalError {
                                    parser.report(inbound.SourceLine, "Bad default string in PROMPT command.")
                                    finish(false, ERR_EVAL)
                                    break
                                } else {
                                    defString = sf("%v", pdefault.result)
                                }
                            }

                            // get prompt
                            broken := false
                            expr, prompt_ev_err := parser.Eval(ifs, inbound.Tokens[2:3])
                            if expr == nil {
                                parser.report(inbound.SourceLine, "Could not evaluate in PROMPT command.")
                                finish(false, ERR_EVAL)
                                break
                            }

                            if prompt_ev_err == nil {
                                processedPrompt := expr.(string)
                                echoMask, _ := gvget("@echomask")

                                // get validator (should be at [3:C_Is|EOTokens])
                                vposEnd := inbound.TokenCount
                                hasValidator := false
                                if defAt != -1 { // has C_Is
                                    vposEnd = defAt
                                }
                                if vposEnd > 3 {
                                    hasValidator = true
                                }

                                if hasValidator {
                                    val_ex, val_ex_error := parser.Eval(ifs, inbound.Tokens[3:vposEnd])
                                    if val_ex_error != nil {
                                        parser.report(inbound.SourceLine, "Validator invalid in PROMPT!")
                                        finish(false, ERR_EVAL)
                                        break
                                    }
                                    switch val_ex.(type) {
                                    case string:
                                        validator = val_ex.(string)
                                    }
                                    intext := ""
                                    validated := false
                                    for !validated || broken {
                                        intext, _, broken = getInput(processedPrompt, defString, currentpane, row, col, inWidth, []string{}, promptColour, false, false, echoMask.(string))
                                        intext = sanitise(intext)
                                        validated, _ = regexp.MatchString(validator, intext)
                                    }
                                    if !broken {
                                        vset(&inbound.Tokens[1], ifs, ident, inbound.Tokens[1].tokText, intext)
                                    }
                                } else {
                                    var inp string
                                    inp, _, broken = getInput(processedPrompt, defString, currentpane, row, col, inWidth, []string{}, promptColour, false, false, echoMask.(string))
                                    inp = sanitise(inp)
                                    vset(&inbound.Tokens[1], ifs, ident, inbound.Tokens[1].tokText, inp)
                                }
                                if broken {
                                    finish(false, 0)
                                }
                            }
                        }
                    }
                }
            }

        case C_Logging:

            if inbound.TokenCount < 2 { // || inbound.TokenCount > 3 {
                parser.report(inbound.SourceLine, "LOGGING command malformed.")
                finish(false, ERR_SYNTAX)
                break
            }

            switch str.ToLower(inbound.Tokens[1].tokText) {

            case "off":
                for len(logQueue) > 0 {
                    // let the queue flush
                }
                loggingEnabled = false
                stopLogWorker() // Stop the background logging worker

            case "status":
                // Show comprehensive logging status
                pf("[#bold]Logging Configuration:[#-]\n")
                pf("  State: %s\n", getLoggingStateString())
                if loggingEnabled {
                    pf("  Log file: %s\n", logFile)
                }
                pf("  Format: %s\n", getLoggingFormatString())
                pf("  Console output: %s\n", func() string {
                    if v, exists := gvget("@silentlog"); exists && v != nil {
                        if silent, ok := v.(bool); ok && silent {
                            return sparkle("[#6]QUIET[#-]")
                        }
                    }
                    return sparkle("[#4]LOUD[#-]")
                }())

                if subj, exists := gvget("@logsubject"); exists && subj != nil {
                    if subjStr, ok := subj.(string); ok && subjStr != "" {
                        pf("  Subject prefix: %s\n", subjStr)
                    }
                }

                pf("  Error logging: %s\n", getErrorLoggingStateString())

                // Enhanced queue statistics
                used, total, running, webRequests, mainRequests := getLogQueueStats()
                pf("  Queue: %d/%d requests (%s)\n", used, total, func() string {
                    if running {
                        return sparkle("[#4]RUNNING[#-]")
                    }
                    return sparkle("[#2]STOPPED[#-]")
                }())
                pf("  Queue processed: %d main, %d web access\n", mainRequests, webRequests)

                // Web access logging status
                if log_web {
                    pf("  Web access logging: [#4]ENABLED[#-] -> %s\n", web_log_file)
                } else {
                    pf("  Web access logging: [#2]DISABLED[#-]\n")
                }

                pf("  Memory reserve: %d bytes (%s)\n", emergencyReserveSize, getMemoryReserveStateString())

                if logRotateSize > 0 {
                    pf("  Log rotation: %d bytes, keep %d files\n", logRotateSize, logRotateCount)
                } else {
                    pf("  Log rotation: [#2]DISABLED[#-]\n")
                }

                if jsonLoggingEnabled && len(logFields) > 0 {
                    pf("  JSON fields: ")
                    first := true
                    for k, v := range logFields {
                        if !first {
                            pf(", ")
                        }
                        pf("%s=%v", k, v)
                        first = false
                    }
                    pf("\n")
                }

            case "on":
                loggingEnabled = true
                if inbound.TokenCount == 3 {
                    we = parser.wrappedEval(ifs, ident, ifs, ident, inbound.Tokens[2:])
                    if we.evalError {
                        parser.report(inbound.SourceLine, sf("could not evaluate destination filename in LOGGING ON statement\n%+v", we.errVal))
                        finish(false, ERR_EVAL)
                        break
                    }
                    proposedLogFile := we.result.(string)
                    // Validate log file path and get expanded path
                    expandedLogFile, err := validateLogFilePath(proposedLogFile)
                    if err != nil {
                        parser.report(inbound.SourceLine, sf("Invalid log file path: %v", err))
                        finish(false, ERR_EVAL)
                        break
                    }
                    logFile = expandedLogFile
                    gvset("@logsubject", "")
                }
                startLogWorker() // Start the background logging worker

            case "quiet":
                gvset("@silentlog", true)

            case "loud":
                gvset("@silentlog", false)

            case "testfile":
                if testMode {
                    if inbound.TokenCount > 2 {
                        we = parser.wrappedEval(ifs, ident, ifs, ident, inbound.Tokens[2:])
                        if we.evalError {
                            parser.report(inbound.SourceLine, sf("could not evaluate filename in LOGGING TESTFILE statement\n%+v", we.errVal))
                            finish(false, ERR_EVAL)
                            break
                        }
                        old_name := test_output_file
                        test_output_file = we.result.(string)
                        _, err = os.Stat(test_output_file)
                        if err == nil {
                            err = os.Remove(test_output_file)
                        }
                        err = os.Rename(old_name, test_output_file)
                        if err != nil {
                            parser.report(inbound.SourceLine, sf("Error during test file instantiation:\n%v", err))
                            finish(false, ERR_FILE)
                        }
                    } else {
                        parser.report(inbound.SourceLine, "Invalid test filename provided for LOGGING TESTFILE command.")
                        finish(false, ERR_SYNTAX)
                    }
                } // else do nothing with this command outside of test mode.

            case "accessfile":
                if inbound.TokenCount > 2 {
                    we = parser.wrappedEval(ifs, ident, ifs, ident, inbound.Tokens[2:])
                    if we.evalError {
                        parser.report(inbound.SourceLine, sf("could not evaluate filename in LOGGING ACCESSFILE statement\n%+v", we.errVal))
                        finish(false, ERR_EVAL)
                        break
                    }
                    proposedAccessFile := we.result.(string)
                    // Validate access file path and get expanded path
                    expandedAccessFile, validateErr := validateLogFilePath(proposedAccessFile)
                    if validateErr != nil {
                        parser.report(inbound.SourceLine, sf("Invalid access file path: %v", validateErr))
                        finish(false, ERR_EVAL)
                        break
                    }
                    web_log_file = expandedAccessFile
                    // pf("accessfile changed to %v\n",web_log_file)
                    web_log_handle.Close()
                    var err error
                    web_log_handle, err = os.OpenFile(web_log_file, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
                    if err != nil {
                        log.Println(err)
                    }
                } else {
                    parser.report(inbound.SourceLine, "No access file provided for LOGGING ACCESSFILE command.")
                    finish(false, ERR_SYNTAX)
                }

            case "web":
                if inbound.TokenCount > 2 {
                    switch str.ToLower(inbound.Tokens[2].tokText) {
                    case "on", "1", "enable":
                        log_web = true
                    case "off", "0", "disable":
                        log_web = false
                    default:
                        parser.report(inbound.SourceLine, "Invalid state set for LOGGING WEB.")
                        finish(false, ERR_EVAL)
                    }
                } else {
                    parser.report(inbound.SourceLine, "No state provided for LOGGING WEB command.")
                    finish(false, ERR_SYNTAX)
                }

            case "subject":
                if inbound.TokenCount == 3 {
                    we = parser.wrappedEval(ifs, ident, ifs, ident, inbound.Tokens[2:])
                    if we.evalError {
                        parser.report(inbound.SourceLine, sf("could not evaluate logging subject in LOGGING SUBJECT statement\n%+v", we.errVal))
                        finish(false, ERR_EVAL)
                        break
                    }
                    gvset("@logsubject", we.result.(string))
                } else {
                    gvset("@logsubject", "")
                }

            case "json":
                if inbound.TokenCount > 2 {
                    switch str.ToLower(inbound.Tokens[2].tokText) {
                    case "on", "1", "enable":
                        jsonLoggingEnabled = true
                    case "off", "0", "disable":
                        jsonLoggingEnabled = false
                    case "fields":
                        // Handle field management: LOGGING JSON FIELDS +field -field etc.
                        for i := int16(3); i < inbound.TokenCount; i++ {
                            token := inbound.Tokens[i]
                            switch token.tokType {
                            case O_Plus:
                                // Add field: +fieldname value
                                if i+int16(2) < inbound.TokenCount {
                                    fieldName := inbound.Tokens[i+1].tokText
                                    fieldValueExpr, err := parser.Eval(ifs, inbound.Tokens[i+2:i+3])
                                    if err != nil {
                                        parser.report(inbound.SourceLine, sf("Could not evaluate field value: %s", err))
                                        finish(false, ERR_EVAL)
                                        break
                                    }
                                    logFields[fieldName] = fieldValueExpr
                                    i += int16(2) // Skip field name and value
                                }
                            case O_Minus:
                                if i+int16(1) < inbound.TokenCount {
                                    if inbound.Tokens[i+1].tokText == "" {
                                        // Clear all fields: -
                                        logFields = make(map[string]any)
                                    } else {
                                        // Remove specific field: -fieldname
                                        fieldName := inbound.Tokens[i+1].tokText
                                        delete(logFields, fieldName)
                                        i += int16(1) // Skip field name
                                    }
                                } else {
                                    // Clear all fields if just "-"
                                    logFields = make(map[string]any)
                                }
                            case Identifier:
                                tokenLower := str.ToLower(token.tokText)
                                if tokenLower == "push" {
                                    // Push current fields to stack
                                    stackCopy := make(map[string]any)
                                    for k, v := range logFields {
                                        stackCopy[k] = v
                                    }
                                    logFieldsStack = append(logFieldsStack, stackCopy)
                                } else if tokenLower == "pop" {
                                    // Pop fields from stack
                                    if len(logFieldsStack) > 0 {
                                        logFields = logFieldsStack[len(logFieldsStack)-1]
                                        logFieldsStack = logFieldsStack[:len(logFieldsStack)-1]
                                    }
                                }
                            }
                        }
                    default:
                        parser.report(inbound.SourceLine, "Invalid JSON logging option.")
                        finish(false, ERR_SYNTAX)
                    }
                } else {
                    parser.report(inbound.SourceLine, "LOGGING JSON requires an option (on/off/fields).")
                    finish(false, ERR_SYNTAX)
                }

            case "error":
                if inbound.TokenCount > 2 {
                    switch str.ToLower(inbound.Tokens[2].tokText) {
                    case "on", "1", "enable":
                        errorLoggingEnabled = true
                    case "off", "0", "disable":
                        errorLoggingEnabled = false
                    default:
                        parser.report(inbound.SourceLine, "Invalid state for LOGGING ERROR.")
                        finish(false, ERR_SYNTAX)
                    }
                } else {
                    parser.report(inbound.SourceLine, "LOGGING ERROR requires an option (on/off).")
                    finish(false, ERR_SYNTAX)
                }

            case "reserve":
                if inbound.TokenCount > 2 {
                    switch str.ToLower(inbound.Tokens[2].tokText) {
                    case "off", "0", "disable":
                        emergencyReserveSize = 0
                        if enhancedErrorsEnabled && emergencyMemoryReserve != nil {
                            *emergencyMemoryReserve = nil
                            emergencyMemoryReserve = nil
                        }
                    default:
                        // Parse as size value
                        we = parser.wrappedEval(ifs, ident, ifs, ident, inbound.Tokens[2:])
                        if we.evalError {
                            parser.report(inbound.SourceLine, "Invalid size for LOGGING RESERVE")
                            finish(false, ERR_EVAL)
                            break
                        }
                        size, faulted := GetAsInt(we.result)
                        if faulted || size < 0 {
                            parser.report(inbound.SourceLine, "LOGGING RESERVE size must be a positive integer")
                            finish(false, ERR_EVAL)
                            break
                        }
                        emergencyReserveSize = size
                        // Reallocate reserve with new size
                        if enhancedErrorsEnabled && size > 0 {
                            reserve := make([]byte, emergencyReserveSize)
                            emergencyMemoryReserve = &reserve
                        }
                    }
                } else {
                    // No arguments - show status
                    status := "disabled"
                    if enhancedErrorsEnabled && emergencyMemoryReserve != nil {
                        status = "enabled"
                    }
                    pf("Memory reserve: %d bytes (%s)\n", emergencyReserveSize, status)
                }

            case "queue":
                if inbound.TokenCount > 2 {
                    switch str.ToLower(inbound.Tokens[2].tokText) {
                    case "size":
                        if inbound.TokenCount > 3 {
                            we = parser.wrappedEval(ifs, ident, ifs, ident, inbound.Tokens[3:])
                            if we.evalError {
                                parser.report(inbound.SourceLine, "Invalid size for LOGGING QUEUE SIZE")
                                finish(false, ERR_EVAL)
                                break
                            }
                            size, faulted := GetAsInt(we.result)
                            if faulted || size < 1 {
                                parser.report(inbound.SourceLine, "LOGGING QUEUE SIZE must be a positive integer (minimum 1)")
                                finish(false, ERR_EVAL)
                                break
                            }
                            logQueueSize = size
                            // Note: Queue resize would require restarting the log worker
                            // For now, just update the setting for next time logging starts
                        } else {
                            parser.report(inbound.SourceLine, "LOGGING QUEUE SIZE requires a size value")
                            finish(false, ERR_SYNTAX)
                        }
                    default:
                        parser.report(inbound.SourceLine, "Invalid LOGGING QUEUE option (size)")
                        finish(false, ERR_SYNTAX)
                    }
                } else {
                    // No arguments - show current queue size
                    pf("Logging queue size: %d requests\n", logQueueSize)
                }

            case "rotate":
                if inbound.TokenCount > 2 {
                    switch str.ToLower(inbound.Tokens[2].tokText) {
                    case "size":
                        if inbound.TokenCount > 3 {
                            we = parser.wrappedEval(ifs, ident, ifs, ident, inbound.Tokens[3:])
                            if we.evalError {
                                parser.report(inbound.SourceLine, "Invalid size for LOGGING ROTATE SIZE")
                                finish(false, ERR_EVAL)
                                break
                            }
                            size, faulted := GetAsInt64(we.result)
                            if faulted || size < 0 {
                                parser.report(inbound.SourceLine, "LOGGING ROTATE SIZE must be a positive integer")
                                finish(false, ERR_EVAL)
                                break
                            }
                            logRotateSize = size
                        } else {
                            parser.report(inbound.SourceLine, "LOGGING ROTATE SIZE requires a size value")
                            finish(false, ERR_SYNTAX)
                        }
                    case "count":
                        if inbound.TokenCount > 3 {
                            we = parser.wrappedEval(ifs, ident, ifs, ident, inbound.Tokens[3:])
                            if we.evalError {
                                parser.report(inbound.SourceLine, "Invalid count for LOGGING ROTATE COUNT")
                                finish(false, ERR_EVAL)
                                break
                            }
                            count, faulted := GetAsInt(we.result)
                            if faulted || count < 0 {
                                parser.report(inbound.SourceLine, "LOGGING ROTATE COUNT must be a positive integer")
                                finish(false, ERR_EVAL)
                                break
                            }
                            logRotateCount = count
                        } else {
                            parser.report(inbound.SourceLine, "LOGGING ROTATE COUNT requires a count value")
                            finish(false, ERR_SYNTAX)
                        }
                    case "off", "0", "disable":
                        logRotateSize = 0
                        logRotateCount = 0
                    default:
                        parser.report(inbound.SourceLine, "Invalid LOGGING ROTATE option (size/count/off)")
                        finish(false, ERR_SYNTAX)
                    }
                } else {
                    parser.report(inbound.SourceLine, "LOGGING ROTATE requires an option (size/count/off)")
                    finish(false, ERR_SYNTAX)
                }

            default:
                parser.report(inbound.SourceLine, "LOGGING command malformed.")
                finish(false, ERR_SYNTAX)
            }

        case C_Cls:

            if inbound.TokenCount == 1 {
                cls()
                atlock.Lock()
                row = 1
                col = 1
                atlock.Unlock()
                currentpane = "global"
            } else {
                if currentpane != "global" {
                    p := panes[currentpane]
                    for l := 1; l < p.h; l += 1 {
                        clearToEOPane(l, 2)
                    }
                    atlock.Lock()
                    row = 1
                    col = 1
                    atlock.Unlock()
                }
            }

        case C_If:

            // lookahead
            var elsefound, endfound, er bool
            var elsedistance, enddistance int16

            if !inbound.Tokens[0].la_done {
                elsefound, elsedistance, er = lookahead(source_base, parser.pc, 0, 1, C_Else, []int64{C_If}, []int64{C_Endif})
                endfound, enddistance, er = lookahead(source_base, parser.pc, 0, 0, C_Endif, []int64{C_If}, []int64{C_Endif})
                inbound.Tokens[0].la_else_distance = elsedistance
                inbound.Tokens[0].la_end_distance = enddistance
                inbound.Tokens[0].la_has_else = elsefound
                inbound.Tokens[0].la_done = true
            } else {
                endfound = true
                er = false
                elsefound = inbound.Tokens[0].la_has_else
                elsedistance = inbound.Tokens[0].la_else_distance
                enddistance = inbound.Tokens[0].la_end_distance
            }

            if er || !endfound {
                parser.report(inbound.SourceLine, "Missing ENDIF for this IF")
                finish(false, ERR_SYNTAX)
                break
            }

            // eval
            expr, err = parser.Eval(ifs, inbound.Tokens[1:])
            if err != nil {
                parser.report(inbound.SourceLine, sf("Could not evaluate expression.\n%#v\n%+v", expr, err))
                finish(false, ERR_SYNTAX)
                break
            }

            if isBool(expr.(bool)) && expr.(bool) {
                // was true
                break
            } else {
                if elsefound && (elsedistance < enddistance) {
                    parser.pc += elsedistance
                } else {
                    parser.pc += enddistance
                }
            }

        case C_Else:

            // we already jumped to else+1 to deal with a failed IF test
            // so jump straight to the endif here

            endfound, enddistance, _ := lookahead(source_base, parser.pc, 1, 0, C_Endif, []int64{C_If}, []int64{C_Endif})

            if endfound {
                parser.pc += enddistance
            } else { // this shouldn't ever occur, as endif checked during C_If, but...
                parser.report(inbound.SourceLine, "ELSE without an ENDIF\n")
                finish(false, ERR_SYNTAX)
            }

        case C_Endif:

            // ENDIF *should* just be an end-of-block marker

        case C_Debug:
            // "debug on|off|break"
            if inbound.TokenCount < 2 {
                pf("[#fred]debug statement requires an argument: on, off, or break[#-]\n")
                break
            }
            action := str.ToLower(inbound.Tokens[1].tokText)

            switch action {
            case "on":
                debugMode = true
                pf("[#fgreen]Debug mode enabled.[#-]\n")
            case "off":
                debugMode = false
                pf("[#fgreen]Debug mode disabled.[#-]\n")
            case "break":
                pf("[#fyellow]Entering debugger on explicit break command.[#-]\n")
                key := (uint64(source_base) << 32) | uint64(parser.pc)
                debugger.enterDebugger(key, functionspaces[source_base], ident, &mident, &gident)
            default:
                pf("[#fred]Unknown debug command: %s[#-]\n", action)
            }
            continue

        case C_Try:
            // Parse capture list: try USES x,y,z throws "category"
            var capturedVars []string
            var usesPos, throwsPos int = -1, -1

            // Find positions of USES and throws
            for i, tok := range inbound.Tokens {
                if tok.tokType == C_Uses {
                    usesPos = i
                } else if tok.tokType == C_Throws {
                    throwsPos = i
                }
            }

            // Error checking for capture list
            if usesPos != -1 && throwsPos != -1 && usesPos > throwsPos {
                parser.report(inbound.SourceLine, "Try USES clause must come before throws clause")
                finish(false, ERR_SYNTAX)
                break
            }

            // Parse capture list if USES found
            if usesPos != -1 {
                // Extract tokens after USES until throws (if present)
                endPos := len(inbound.Tokens)
                if throwsPos != -1 {
                    endPos = throwsPos
                }

                captureTokens := inbound.Tokens[usesPos+1 : endPos]
                if len(captureTokens) > 0 {
                    // Split by commas to get variable names
                    varArrays := parser.splitCommaArray(captureTokens)
                    for _, varTokens := range varArrays {
                        if len(varTokens) > 0 {
                            // Extract variable name and validate it's declared
                            varToken := varTokens[0]
                            varName := varToken.tokText

                            // Check if variable is declared using bindpos
                            if varToken.bindpos >= uint64(len(*ident)) || !(*ident)[varToken.bindpos].declared {
                                parser.report(inbound.SourceLine, fmt.Sprintf("Variable '%s' in USES clause is not declared", varName))
                                finish(false, ERR_SYNTAX)
                                break
                            }

                            capturedVars = append(capturedVars, varName)
                        }
                    }
                }
            }

            // Parse optional throws clause: try throws "category"
            // This sets the default exception category for throw statements in this block
            var defaultCategory any
            if inbound.TokenCount >= 3 && inbound.Tokens[1].tokType == C_Throws {
                // Parse the exception category
                // Evaluate the expression to get the category (preserve original type for enums)
                categoryTokens := inbound.Tokens[2:]
                we := parser.wrappedEval(ifs, ident, ifs, ident, categoryTokens)
                if we.evalError {
                    parser.report(inbound.SourceLine, "Could not evaluate throws expression")
                    finish(false, ERR_EVAL)
                    break
                }
                // Validate that the result is a string or integer (for enum values)
                switch we.result.(type) {
                case string, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
                    defaultCategory = we.result // Valid type
                default:
                    parser.report(inbound.SourceLine, "throws clause must be a string or integer (enum value)")
                    finish(false, ERR_EVAL)
                    break
                }
            }

            // Execute try block using enhanced registry-based approach

            // Build execution path for context tracking
            executionPath := make([]uint32, 0)
            executionPath = append(executionPath, source_base)

            // Add call chain to execution path for nested scenarios
            for _, chainEntry := range errorChain {
                executionPath = append(executionPath, chainEntry.loc)
            }

            // Find applicable try blocks using the registry
            applicableTryBlocks := findApplicableTryBlocks(ctx, source_base, executionPath)

            var matchingTryBlock *tryBlockInfo
            for _, tryInfo := range applicableTryBlocks {
                if tryInfo.startLine == inbound.SourceLine+1 {
                    matchingTryBlock = &tryInfo
                    break
                }
            }

            if matchingTryBlock != nil {

                // Execute the matching try block directly using its function space ID
                // Only execute during actual execution, not during function definition
                if matchingTryBlock.functionSpace != 0 && !defining {

                    // Store the captured variables in the actual registry entry
                    // Find the actual registry entry by function space ID and update it
                    for key, tryBlock := range tryBlockRegistry {
                        if tryBlock.functionSpace == matchingTryBlock.functionSpace {
                            tryBlock.capturedVars = capturedVars
                            tryBlockRegistry[key] = tryBlock
                            break
                        }
                    }

                    // Store the default category in the call context
                    calllock.Lock()
                    calltable[matchingTryBlock.functionSpace].defaultExceptionCategory = defaultCategory
                    calllock.Unlock()

                    // Capture variables from parent scope
                    var capturedVarsValues []any
                    if len(capturedVars) > 0 {
                        capturedVarsValues = make([]any, len(capturedVars))
                        for i, varName := range capturedVars {
                            // Get the variable's current value from parent scope
                            value, _ := vget(nil, ifs, ident, varName)
                            capturedVarsValues[i] = value
                        }
                    }

                    // Create a fresh ident table for try block execution (like regular function calls)
                    var tryIdent = make([]Variable, identInitialSize)
                    var callErr error
                    var capturedResult []any
                    // Set the callLine field in the calltable entry before calling the function
                    // For try blocks, we use the source line from the inbound phrase
                    atomic.StoreInt32(&calltable[matchingTryBlock.functionSpace].callLine, int32(inbound.SourceLine))
                    _, _, _, capturedResult, callErr = Call(ctx, MODE_NEW, &tryIdent, matchingTryBlock.functionSpace, ciEval, false, nil, "", []string{}, capturedVarsValues)
                    if callErr != nil {
                        // Handle try block execution error
                        pf("Error executing try block: %v\n", callErr)
                    }

                    // Repopulate parent scope with modified captured variables
                    if capturedResult != nil && len(capturedResult) > 0 && len(capturedVars) > 0 {
                        for i, varName := range capturedVars {
                            if i < len(capturedResult) {
                                // Update the variable in parent scope
                                vset(nil, ifs, ident, varName, capturedResult[i])
                            }
                        }
                    }

                    // Check return values from try block execution and mark for cleanup
                    calllock.Lock()
                    tryRetvals := calltable[matchingTryBlock.functionSpace].retvals
                    calltable[matchingTryBlock.functionSpace].gcShyness = 100
                    calltable[matchingTryBlock.functionSpace].gc = true
                    calllock.Unlock()

                    if tryRetvals != nil {
                        // Try block returned with values - check what happened
                        if retArray, ok := tryRetvals.([]any); ok && len(retArray) >= 1 {
                            if status, ok := retArray[0].(int); ok {
                                switch status {
                                case EXCEPTION_THROWN:
                                    // Exception was thrown - set up exception state and jump to catch blocks

                                    if len(retArray) >= 4 {
                                        // Check if we have the full exception info preserved
                                        if excPtr, ok := retArray[3].(*exceptionInfo); ok {
                                            // Use the preserved exception info with original stack trace
                                            atomic.StorePointer(&calltable[ifs].activeException, unsafe.Pointer(excPtr))
                                        } else {
                                            // Fallback: recreate exception info (shouldn't happen with new format)
                                            excInfo := &exceptionInfo{
                                                category:   retArray[1],
                                                message:    GetAsString(retArray[2]),
                                                line:       int(inbound.SourceLine) + 1,
                                                function:   fs,
                                                fs:         ifs,
                                                stackTrace: nil, // No stack trace in fallback
                                            }
                                            atomic.StorePointer(&calltable[ifs].activeException, unsafe.Pointer(excInfo))
                                        }

                                        // Jump to first catch block - look in the current function space, not the try block function space
                                        // Start looking from after the inner try block (pc+2 to skip the inner endtry)
                                        catchFound, catchDistance, err := lookahead(ifs, parser.pc+2, 0, 0, C_Catch, []int64{C_Try}, []int64{C_Endtry})
                                        if catchFound {
                                        }
                                        if err || !catchFound {
                                            // No catch blocks found - look for endtry
                                            endtryFound, endtryDistance, err := lookahead(ifs, parser.pc+2, 0, 0, C_Endtry, []int64{C_Try}, []int64{C_Endtry})
                                            if endtryFound {
                                            }
                                            if err || !endtryFound {
                                                // No endtry found in current function space - bubble exception up to parent
                                                // Get exception info atomically for bubbling
                                                if ifs < uint32(len(calltable)) {
                                                    exceptionPtr := atomic.LoadPointer(&calltable[ifs].activeException)
                                                    if exceptionPtr != nil {
                                                        excPtr := (*exceptionInfo)(exceptionPtr)
                                                        // Preserve the full exception info including stack trace
                                                        retvalues = []any{EXCEPTION_THROWN, excPtr.category, excPtr.message, excPtr}
                                                    } else {
                                                        retvalues = []any{EXCEPTION_THROWN, "unknown", "unknown error", nil}
                                                    }
                                                } else {
                                                    retvalues = []any{EXCEPTION_THROWN, "unknown", "unknown error", nil}
                                                }
                                                retval_count = 4
                                                endFunc = true
                                                // Don't clear exception state - let it bubble up
                                                break
                                            }
                                            parser.pc += endtryDistance + 1 // Jump to endtry (+1 because we looked from pc+2)
                                        } else {
                                            parser.pc += catchDistance + 1 // Jump to first catch block (+1 because we looked from pc+2)
                                        }
                                    } else {
                                        parser.report(inbound.SourceLine, "Invalid exception return format")
                                        finish(false, ERR_SYNTAX)
                                        break
                                    }
                                case EXCEPTION_HANDLED:
                                    // Exception was handled within try block - skip endtry and continue
                                    parser.pc++ // Skip the endtry statement since we already dealt with it
                                case EXCEPTION_RETURN:
                                    // Try block executed a return statement - unpack return arguments and propagate
                                    if len(retArray) > 1 {
                                        // Extract user return arguments (skip the EXCEPTION_RETURN status)
                                        userRetvals := retArray[1:]
                                        retvalues = userRetvals
                                        retval_count = uint8(len(userRetvals))
                                    } else {
                                        // Return with no arguments
                                        retvalues = nil
                                        retval_count = 0
                                    }
                                    endFunc = true
                                    // The main execution loop will check endFunc and exit properly
                                default:
                                    // Other status codes - handle as needed
                                    parser.pc++ // Skip the endtry statement since we already dealt with it
                                }
                            } else {
                                // Invalid return format - treat as normal completion
                                // Use lookahead to find the correct endtry that belongs to this try block
                                endtryFound, endtryDistance, err := lookahead(source_base, parser.pc+1, 0, 0, C_Endtry, []int64{C_Try}, []int64{C_Endtry})
                                if endtryFound && !err {
                                    parser.pc += endtryDistance // Jump to the correct endtry
                                } else {
                                    // No endtry found in current function space - treat as normal completion and continue
                                    // This can happen in nested try blocks where the endtry is in the parent function space
                                }
                            }
                        } else {
                            // Invalid return format - treat as normal completion
                            // Use lookahead to find the correct endtry that belongs to this try block
                            endtryFound, endtryDistance, err := lookahead(source_base, parser.pc+1, 0, 0, C_Endtry, []int64{C_Try}, []int64{C_Endtry})
                            if endtryFound && !err {
                                parser.pc += endtryDistance // Jump to the correct endtry
                            } else {
                                // No endtry found in current function space - treat as normal completion and continue
                                // This can happen in nested try blocks where the endtry is in the parent function space
                            }
                        }
                    } else {
                        // Try block completed normally - skip endtry and continue
                        parser.pc++ // Skip the endtry statement since we already dealt with it
                    }
                } else {
                }
            } else {
            }

            // Skip the try block content in the parent function space to prevent double execution
            // Only do this during execution, not during function definition, and only when we're at a C_Try token
            // AND there's an unhandled exception AND we're in an enclosing try block
            if !defining && functionspaces[source_base][parser.pc].Tokens[0].tokType == C_Try {
                // Check if there's an unhandled exception in the current function space
                var hasUnhandledException bool
                if ifs < uint32(len(calltable)) {
                    exceptionPtr := atomic.LoadPointer(&calltable[ifs].activeException)
                    hasUnhandledException = exceptionPtr != nil
                }

                // Check if we're in an enclosing try block by looking for a try block in the parent function space
                // that starts before the current position
                var hasEnclosingTryBlock bool
                for i := int16(0); i < parser.pc; i++ {
                    if i < int16(len(functionspaces[source_base])) && len(functionspaces[source_base][i].Tokens) > 0 {
                        if functionspaces[source_base][i].Tokens[0].tokType == C_Try {
                            hasEnclosingTryBlock = true
                            break
                        }
                    }
                }

                if hasUnhandledException && hasEnclosingTryBlock {
                    // Find the corresponding endtry and jump to it (fresh lookahead for parent function space)
                    // We're at indent=0, looking for endtry at endlevel=0 (same level, since we're in the parent function space)
                    endtryFound, endtryDistance, err := lookahead(source_base, parser.pc+1, 0, 0, C_Endtry, []int64{C_Try}, []int64{C_Endtry})
                    if endtryFound && !err {
                        parser.pc += endtryDistance // Jump to endtry to skip try block content
                    }
                }
            }

        case C_Catch:
            // Cache lookahead results if not already done
            if !inbound.Tokens[0].la_done {
                // Always find next catch (we're inside try block, so indent=1)
                nextFound, nextDistance, err := lookahead(source_base, parser.pc+1, 1, 1, C_Catch, []int64{C_Try}, []int64{C_Endtry})
                if err {
                    // Lookahead failed - likely hit endtry first, which is normal
                    nextFound = false
                    nextDistance = 0
                }

                // Always find endtry (we're inside try block at indent=1, looking for endtry at endlevel=0)
                endtryFound, endtryDistance, err := lookahead(source_base, parser.pc+1, 1, 0, C_Endtry, []int64{C_Try}, []int64{C_Endtry})
                if err || !endtryFound {
                    // No endtry found in current function space - this means we're in a nested try block
                    // and the endtry is in the parent function space. This is not a syntax error.
                    // Set reasonable defaults for the lookahead cache and let normal processing continue.
                    inbound.Tokens[0].la_has_else = nextFound
                    inbound.Tokens[0].la_else_distance = nextDistance + 1
                    inbound.Tokens[0].la_end_distance = 1 // Default to next statement
                    inbound.Tokens[0].la_done = true
                } else {
                    // Look for finally block to determine final target
                    thenFound, thenDistance, thenErr := lookahead(source_base, parser.pc+1, 1, 1, C_Then, []int64{C_Try}, []int64{C_Endtry})
                    var finalTarget int16
                    if thenFound && !thenErr {
                        finalTarget = thenDistance + 1 // +1 because we started from pc+1
                    } else {
                        finalTarget = endtryDistance + 1 // +1 because we started from pc+1
                    }

                    // Cache both
                    inbound.Tokens[0].la_has_else = nextFound
                    inbound.Tokens[0].la_else_distance = nextDistance + 1 // +1 because we started from pc+1
                    inbound.Tokens[0].la_end_distance = finalTarget
                    inbound.Tokens[0].la_done = true
                }
            }

            // Only process if there's an active exception
            // Check for active exception atomically
            var hasActiveException bool
            var isCatchMatched bool
            if ifs < uint32(len(calltable)) {
                exceptionPtr := atomic.LoadPointer(&calltable[ifs].activeException)
                hasActiveException = exceptionPtr != nil
                isCatchMatched = atomic.LoadInt32(&calltable[ifs].currentCatchMatched) == 1
            }

            if !hasActiveException || isCatchMatched {
                // No active exception or already handled - skip this catch

                // Use cached lookahead results to jump
                if isCatchMatched {
                    // Exception was already caught - always jump to endtry
                    parser.pc += inbound.Tokens[0].la_end_distance - 1 // Jump to endtry
                } else if inbound.Tokens[0].la_has_else {
                    // No active exception but there are more catch blocks - jump to next catch
                    parser.pc += inbound.Tokens[0].la_else_distance - 1 // Jump to next catch
                } else {
                    // No active exception and no more catch blocks - jump to endtry
                    parser.pc += inbound.Tokens[0].la_end_distance - 1 // Jump to endtry
                }
                break
            }

            // Parse catch statement: catch err [is|contains|in <expression>]
            if inbound.TokenCount >= 2 {
                // Expected format: catch err [operator expression]
                errVarName := inbound.Tokens[1].tokText

                if inbound.TokenCount >= 4 {
                    // Has operator and expression: catch err operator expression
                    operatorToken := inbound.Tokens[2].tokType
                    exprTokens := inbound.Tokens[3:]

                    // Evaluate the expression to get the condition value
                    if we = parser.wrappedEval(ifs, ident, ifs, ident, exprTokens); !we.evalError {
                        conditionValue := we.result

                        matched := false

                        // Get current exception atomically
                        exceptionPtr := atomic.LoadPointer(&calltable[ifs].activeException)
                        catchExcPtr := (*exceptionInfo)(exceptionPtr)

                        switch operatorToken {
                        case C_Is:
                            // Check for comma-separated patterns (OR logic)
                            // Check if there are commas in the expression tokens
                            hasCommas := false
                            for _, token := range exprTokens {
                                if token.tokType == O_Comma {
                                    hasCommas = true
                                    break
                                }
                            }

                            if hasCommas {
                                // Multiple patterns - evaluate each one and check if any match
                                patterns, errs := parser.evalCommaArray(ifs, exprTokens)

                                // Check if there are any non-nil errors
                                hasErrors := false
                                for _, err := range errs {
                                    if err != nil {
                                        hasErrors = true
                                        parser.report(inbound.SourceLine, sf("Error evaluating catch pattern: %v", err))
                                        break
                                    }
                                }
                                if hasErrors {
                                    // Don't finish the program - just skip this catch block
                                    // The exception will continue to the next catch block
                                    break
                                }

                                // Check if any pattern matches (OR logic)
                                for _, pattern := range patterns {
                                    if reflect.TypeOf(catchExcPtr.category) == reflect.TypeOf(pattern) {
                                        if catchExcPtr.category == pattern {
                                            matched = true
                                            break
                                        }
                                    } else {
                                        // Type mismatch error
                                        parser.report(inbound.SourceLine, sf("Type mismatch in catch clause: exception type %T, condition type %T", catchExcPtr.category, pattern))
                                        finish(false, ERR_EVAL)
                                        break
                                    }
                                }
                            } else {
                                // Single pattern - existing logic
                                if reflect.TypeOf(catchExcPtr.category) == reflect.TypeOf(conditionValue) {
                                    if catchExcPtr.category == conditionValue {
                                        matched = true
                                    }
                                } else {
                                    // Type mismatch error
                                    parser.report(inbound.SourceLine, sf("Type mismatch in catch clause: exception type %T, condition type %T", catchExcPtr.category, conditionValue))
                                    finish(false, ERR_EVAL)
                                    break
                                }
                            }
                        case C_Contains:
                            // Both must be strings for regex matching
                            if reflect.TypeOf(catchExcPtr.category).Kind() == reflect.String && reflect.TypeOf(conditionValue).Kind() == reflect.String {
                                regexPattern := conditionValue.(string)
                                categoryStr := catchExcPtr.category.(string)
                                if matched_regex, _ := regexp.MatchString(regexPattern, categoryStr); matched_regex {
                                    matched = true
                                }
                            } else {
                                // Type mismatch error
                                parser.report(inbound.SourceLine, sf("Type mismatch in catch contains clause: both operands must be strings, got exception type %T, condition type %T", catchExcPtr.category, conditionValue))
                                finish(false, ERR_EVAL)
                                break
                            }
                        case C_In:
                            // Check if exception category is in the condition collection
                            switch conditionValue.(type) {
                            case []any:
                                conditionArray := conditionValue.([]any)
                                for _, item := range conditionArray {
                                    if reflect.TypeOf(catchExcPtr.category) == reflect.TypeOf(item) {
                                        if catchExcPtr.category == item {
                                            matched = true
                                            break
                                        }
                                    }
                                }
                            case []string:
                                if reflect.TypeOf(catchExcPtr.category).Kind() == reflect.String {
                                    conditionArray := conditionValue.([]string)
                                    categoryStr := catchExcPtr.category.(string)
                                    for _, item := range conditionArray {
                                        if categoryStr == item {
                                            matched = true
                                            break
                                        }
                                    }
                                }
                            case []int:
                                if reflect.TypeOf(catchExcPtr.category).Kind() == reflect.Int {
                                    conditionArray := conditionValue.([]int)
                                    categoryInt := catchExcPtr.category.(int)
                                    for _, item := range conditionArray {
                                        if categoryInt == item {
                                            matched = true
                                            break
                                        }
                                    }
                                }
                            default:
                                // For non-collections, exact type and value match
                                if reflect.TypeOf(catchExcPtr.category) == reflect.TypeOf(conditionValue) {
                                    if catchExcPtr.category == conditionValue {
                                        matched = true
                                    }
                                }
                            }
                        }

                        if matched {
                            // This catch block matches - set the err variable and continue executing catch body
                            atomic.StoreInt32(&calltable[ifs].currentCatchMatched, 1)

                            // Create err variable with exception information
                            errVar := map[string]any{
                                "category":    catchExcPtr.category,
                                "message":     catchExcPtr.message,
                                "line":        int(catchExcPtr.line),
                                "function":    catchExcPtr.function,
                                "stack_trace": catchExcPtr.stackTrace,
                                "source":      catchExcPtr.source,
                            }

                            // Set the err variable in the current scope
                            vset(nil, ifs, ident, errVarName, errVar)

                            // Don't jump anywhere - continue normal execution to execute the catch block body
                        } else {
                            // This catch block doesn't match - use cached lookahead results

                            // Use cached lookahead results to jump
                            if inbound.Tokens[0].la_has_else {
                                parser.pc += inbound.Tokens[0].la_else_distance - 1 // Jump to next catch
                            } else {
                                parser.pc += inbound.Tokens[0].la_end_distance - 1 // Jump to endtry
                            }
                        }
                    } else {
                        parser.report(inbound.SourceLine, "Error evaluating catch condition")
                    }

                } else {
                    // Catch-all block: catch err (no operator/expression)
                    // Get current exception info
                    exceptionPtr := atomic.LoadPointer(&calltable[ifs].activeException)
                    catchExcPtr := (*exceptionInfo)(exceptionPtr)

                    if catchExcPtr != nil {
                        // This catch-all block matches any exception
                        atomic.StoreInt32(&calltable[ifs].currentCatchMatched, 1)

                        // Create err variable with exception information
                        errVar := map[string]any{
                            "category":    catchExcPtr.category,
                            "message":     catchExcPtr.message,
                            "line":        int(catchExcPtr.line),
                            "function":    catchExcPtr.function,
                            "stack_trace": catchExcPtr.stackTrace,
                            "source":      catchExcPtr.source,
                        }

                        // Set the err variable in the current scope
                        vset(nil, ifs, ident, errVarName, errVar)

                        // Don't jump anywhere - continue normal execution to execute the catch block body
                    } else {
                        // No active exception - use cached lookahead results to jump
                        if inbound.Tokens[0].la_has_else {
                            parser.pc += inbound.Tokens[0].la_else_distance - 1 // Jump to next catch
                        } else {
                            parser.pc += inbound.Tokens[0].la_end_distance - 1 // Jump to endtry
                        }
                    }
                }

            } else {
                parser.report(inbound.SourceLine, "Invalid catch syntax")
            }

        case C_Throw:
            // Parse throw statement: throw [exception] [with message_expression]
            // If no exception specified, use default category from try throws clause
            if inbound.TokenCount >= 1 {
                var exceptionTokens []Token
                var messageTokens []Token

                // Look for 'with' keyword to separate exception from message
                withIndex := -1
                for i := 1; i < len(inbound.Tokens); i++ {
                    if inbound.Tokens[i].tokType == C_With {
                        withIndex = i
                        break
                    }
                }

                if inbound.TokenCount == 1 {
                    // Format: throw (no exception or message - use default category)
                    exceptionTokens = nil
                    messageTokens = nil
                } else if withIndex != -1 {
                    // Format: throw exception with message_expression
                    if withIndex == 1 {
                        exceptionTokens = nil
                    } else {
                        exceptionTokens = inbound.Tokens[1:withIndex]
                    }
                    messageTokens = inbound.Tokens[withIndex+1:]
                } else {
                    // Format: throw exception (no message)
                    exceptionTokens = inbound.Tokens[1:]
                    messageTokens = nil
                }

                // Evaluate the exception expression or use default category
                var category any
                if exceptionTokens == nil {
                    // No exception specified - use default category from try throws clause
                    calllock.RLock()
                    defaultCategory := calltable[ifs].defaultExceptionCategory
                    calllock.RUnlock()
                    if defaultCategory == nil {
                        parser.report(inbound.SourceLine, "throw requires an exception category or try throws clause")
                        finish(false, ERR_EXCEPTION)
                        break
                    }
                    category = defaultCategory
                } else {
                    // Evaluate the exception expression
                    if we = parser.wrappedEval(ifs, ident, ifs, ident, exceptionTokens); !we.evalError {
                        // Store the actual value (string or integer) without conversion
                        category = we.result
                    } else {
                        parser.report(inbound.SourceLine, "Error evaluating throw exception expression")
                        finish(false, ERR_EXCEPTION)
                        break
                    }
                }

                // Evaluate message expression if present
                message := ""
                if messageTokens != nil && len(messageTokens) > 0 {
                    if we = parser.wrappedEval(ifs, ident, ifs, ident, messageTokens); !we.evalError {
                        message = GetAsString(we.result)
                    } else {
                        parser.report(inbound.SourceLine, "Error evaluating throw message expression")
                        finish(false, ERR_EXCEPTION)
                        break
                    }
                }

                // Capture stack trace at throw time using full call chain
                actualLine := inbound.SourceLine + 1
                stackTraceCopy := generateStackTrace(fs, ifs, actualLine)

                // Set up exception state atomically
                excInfo := &exceptionInfo{
                    category:   category,
                    message:    message,
                    line:       int(actualLine),
                    function:   fs,
                    fs:         ifs,
                    stackTrace: stackTraceCopy,
                    source:     "throw",
                }

                // Stack trace is already correct - no need to modify

                atomic.StorePointer(&calltable[ifs].activeException, unsafe.Pointer(excInfo))
                atomic.StoreInt32(&calltable[ifs].currentCatchMatched, 0)

                // Check if we're inside a try block by looking for C_Endtry
                endtryFound, endtryDistance, endtryErr := lookahead(source_base, parser.pc+1, 1, 0, C_Endtry, []int64{C_Try}, []int64{C_Endtry})

                if !endtryFound || endtryErr {
                    // We're not inside a try block - exception should bubble up to parent function

                    // Set return values to indicate exception bubbling
                    retvalues = []any{EXCEPTION_THROWN, category, message, excInfo}
                    retval_count = 4
                    endFunc = true

                    // Don't clear exception state - let it bubble up
                    break
                }

                // We're inside a try block - cache lookahead results if not already done
                if !inbound.Tokens[0].la_done {

                    // If we're already inside a catch block (currentCatchMatched is true),
                    // we should jump directly to endtry, not look for more catch blocks
                    if atomic.LoadInt32(&calltable[ifs].currentCatchMatched) == 1 {
                        // Look for finally block first
                        thenFound, thenDistance, thenErr := lookahead(source_base, parser.pc+1, 1, 1, C_Then, []int64{C_Try}, []int64{C_Endtry})
                        if thenFound && !thenErr {
                            // Jump to finally block
                            inbound.Tokens[0].la_has_else = false
                            inbound.Tokens[0].la_else_distance = 0
                            inbound.Tokens[0].la_end_distance = thenDistance + 1 // +1 because we started from pc+1
                        } else {
                            // No finally block, jump to endtry
                            inbound.Tokens[0].la_has_else = false
                            inbound.Tokens[0].la_else_distance = 0
                            inbound.Tokens[0].la_end_distance = endtryDistance + 1 // +1 because we started from pc+1
                        }
                    } else {
                        // Normal throw - look for catch blocks first
                        // Always find catch (we're inside try block, so indent=1)
                        catchFound, catchDistance, err := lookahead(source_base, parser.pc+1, 1, 1, C_Catch, []int64{C_Try}, []int64{C_Endtry})
                        if err {
                            // Lookahead failed - likely hit endtry first, which is normal
                            catchFound = false
                            catchDistance = 0
                        }

                        // Determine where to jump if no catch blocks match
                        var finalTarget int16
                        if catchFound {
                            // Look for finally block after catch blocks
                            thenFound, thenDistance, thenErr := lookahead(source_base, parser.pc+1, 1, 1, C_Then, []int64{C_Try}, []int64{C_Endtry})
                            if thenFound && !thenErr {
                                finalTarget = thenDistance + 1 // +1 because we started from pc+1
                            } else {
                                finalTarget = endtryDistance + 1 // +1 because we started from pc+1
                            }
                        } else {
                            // No catch blocks, look for finally block
                            thenFound, thenDistance, thenErr := lookahead(source_base, parser.pc+1, 1, 1, C_Then, []int64{C_Try}, []int64{C_Endtry})
                            if thenFound && !thenErr {
                                finalTarget = thenDistance + 1 // +1 because we started from pc+1
                            } else {
                                finalTarget = endtryDistance + 1 // +1 because we started from pc+1
                            }
                        }

                        // Cache both
                        inbound.Tokens[0].la_has_else = catchFound
                        inbound.Tokens[0].la_else_distance = catchDistance + 1 // +1 because we started from pc+1
                        inbound.Tokens[0].la_end_distance = finalTarget
                    }
                    inbound.Tokens[0].la_done = true
                }

                // Use cached lookahead results
                if inbound.Tokens[0].la_has_else {
                    // Jump to first catch block
                    parser.pc += inbound.Tokens[0].la_else_distance - 1 // -1 because loop will increment
                } else {
                    // Jump to endtry
                    parser.pc += inbound.Tokens[0].la_end_distance - 1 // -1 because loop will increment
                }

            } else {
                parser.report(inbound.SourceLine, "throw requires an exception argument")
            }

        case C_Throws:
            // Parse throws statement: throws expression
            // Sets the default exception category for subsequent throw statements
            if inbound.TokenCount >= 2 {
                // Evaluate the exception expression
                exceptionTokens := inbound.Tokens[1:]
                if we = parser.wrappedEval(ifs, ident, ifs, ident, exceptionTokens); !we.evalError {
                    // Validate that the result is a valid exception category type
                    var defaultCategory any
                    switch we.result.(type) {
                    case string, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
                        defaultCategory = we.result // Valid type
                    default:
                        parser.report(inbound.SourceLine, "throws statement requires a string or integer (enum value)")
                        finish(false, ERR_EXCEPTION)
                        break
                    }

                    // Set the default exception category for this function space
                    calllock.Lock()
                    calltable[ifs].defaultExceptionCategory = defaultCategory
                    calllock.Unlock()

                } else {
                    parser.report(inbound.SourceLine, "Error evaluating throws expression")
                    finish(false, ERR_EXCEPTION)
                }
            } else {
                parser.report(inbound.SourceLine, "throws requires an exception category")
                finish(false, ERR_EXCEPTION)
            }

        case C_Then:
            // Finally block - always executes regardless of exception state
            // Just continue normal execution - the finally code will run
            // Exception state is preserved and will be handled at endtry

        case C_Endtry:
            // Endtry - handle exception state based on whether it was caught
            // Get exception info atomically
            exceptionPtr := atomic.LoadPointer(&calltable[ifs].activeException)
            isExceptionActive := exceptionPtr != nil
            var currentExceptionInfo *exceptionInfo
            if isExceptionActive {
                currentExceptionInfo = (*exceptionInfo)(exceptionPtr)
                // pf("Active Exception is [%+v]\n",currentExceptionInfo)
            }
            isCatchMatched := atomic.LoadInt32(&calltable[ifs].currentCatchMatched) == 1

            if isExceptionActive {
                if isCatchMatched {
                    // Exception was caught and handled - set return values and clear state
                    retvalues = []any{EXCEPTION_HANDLED}
                    retval_count = 1
                    // Clear exception state atomically
                    atomic.StorePointer(&calltable[ifs].activeException, nil)
                    atomic.StoreInt32(&calltable[ifs].currentCatchMatched, 0)
                } else {
                    // Exception was not handled - need to bubble up or apply strictness policy

                    // Check if we're in a nested try block (would bubble up)
                    // Try blocks execute in separate function spaces, so check if this is a try block function space
                    if str.Contains(fs, "try_block_") {
                        // We're in a try block function space - bubble the exception up to the parent
                        retvalues = []any{EXCEPTION_THROWN, currentExceptionInfo.category, currentExceptionInfo.message, currentExceptionInfo}
                        retval_count = 4
                        endFunc = true
                        // Don't clear exception state - let parent handle it
                        break
                    }

                    // Check if we're in a user-defined function (not main) - should bubble up
                    if fs != "main" && fs != "interactive" {
                        // We're in a user-defined function - bubble the exception up to the caller
                        retvalues = []any{EXCEPTION_THROWN, currentExceptionInfo.category, currentExceptionInfo.message, currentExceptionInfo}
                        retval_count = 4
                        endFunc = true
                        // Don't clear exception state - let caller handle it
                        break
                    }

                    // We're at top level (main function) - apply strictness policy
                    handleUnhandledException(currentExceptionInfo, ifs)
                }
            }

        default:

            // local command assignment (child/parent process call)

            if inbound.TokenCount > 1 {
                // ident "=|" or "=<" check
                if statement == Identifier && (inbound.Tokens[1].tokType == O_AssCommand || inbound.Tokens[1].tokType == O_AssOutCommand) {
                    if inbound.TokenCount > 2 {

                        // get text after =| or =<
                        var startPos int
                        bc := basecode[source_base][parser.pc].borcmd

                        switch inbound.Tokens[1].tokType {
                        case O_AssCommand:
                            startPos = str.IndexByte(basecode[source_base][parser.pc].Original, '|') + 1
                            // pf("(debug) ass-command present is : ·%v·\n",basecode[source_base][parser.pc].borcmd)
                        case O_AssOutCommand:
                            startPos = str.IndexByte(basecode[source_base][parser.pc].Original, '<') + 1
                            // pf("(debug) ass-out-command present is : ·%v·\n",basecode[source_base][parser.pc].borcmd)
                        }

                        var cmd string
                        if bc == "" {
                            cmd = interpolate(currentModule, ifs, ident, basecode[source_base][parser.pc].Original[startPos:])
                        } else {
                            cmd = interpolate(currentModule, ifs, ident, bc[2:])
                        }

                        cop := system(cmd, false)
                        lhs_name := inbound.Tokens[0].tokText
                        switch inbound.Tokens[1].tokType {
                        case O_AssCommand:
                            vset(&inbound.Tokens[0], ifs, ident, lhs_name, cop)
                        case O_AssOutCommand:
                            vset(&inbound.Tokens[0], ifs, ident, lhs_name, cop.Out)
                        }
                    }
                    // skip normal eval below
                    break
                }
            }

            // try to eval and assign
            we = parser.wrappedEval(ifs, ident, ifs, ident, inbound.Tokens)

            // Check if wrappedEval returned an exception
            if we.result != nil {
                if retArray, ok := we.result.([]any); ok && len(retArray) >= 1 {
                    if status, ok := retArray[0].(int); ok && status == EXCEPTION_THROWN {
                        // Set the exception state in the current function
                        if ifs < uint32(len(calltable)) {
                            if len(retArray) >= 4 {
                                if excInfo, ok := retArray[3].(*exceptionInfo); ok {
                                    atomic.StorePointer(&calltable[ifs].activeException, unsafe.Pointer(excInfo))
                                }
                            }
                        }
                    }
                }
            }
            if we.evalError {
                if enhancedErrorsEnabled {
                    // Use enhanced error handling with typo suggestions
                    showEnhancedErrorWithCallArgs(parser, inbound.SourceLine, we.errVal, ifs, nil, "")
                    finish(false, ERR_EVAL)
                    break
                } else {
                    // Standard error reporting (no typo suggestions)
                    errmsg := ""
                    // pf("[statement-loop] received this error response from wrappedEval(): %#v\n",we)
                    if we.errVal != nil {
                        errmsg = sf("%+v\n", we.errVal)
                    }
                    if !interactive {
                        parser.report(inbound.SourceLine, sf("Error in evaluation\n%s", errmsg))
                        finish(false, ERR_EVAL)
                        break
                    } else {
                        panic("")
                    }
                }
            }

            // normalProcessing:
            // Check if we have an active exception that needs handling (atomic, no lock needed)
            var excInfo *exceptionInfo
            if ifs < uint32(len(calltable)) {
                exceptionPtr := atomic.LoadPointer(&calltable[ifs].activeException)
                if exceptionPtr != nil {
                    excInfo = (*exceptionInfo)(exceptionPtr)
                }
            }
            hasActiveException := excInfo != nil

            if hasActiveException {
                // We have an active exception - check if we're inside a try block
                endtryFound, endtryDistance, err := lookahead(source_base, parser.pc+1, 1, 0, C_Endtry, []int64{C_Try}, []int64{C_Endtry})
                if !endtryFound || err {
                    // We're not inside a try block - bubble exception up to parent function
                    if excInfo != nil {
                        retvalues = []any{EXCEPTION_THROWN, excInfo.category, excInfo.message, excInfo}
                        retval_count = 4
                    } else {
                        retvalues = []any{EXCEPTION_THROWN, "unknown", "unknown error", nil}
                        retval_count = 4
                    }
                    endFunc = true
                    break
                } else {
                    // We're inside a try block - look for catch blocks
                    catchFound, catchDistance, err := lookahead(source_base, parser.pc+1, 1, 1, C_Catch, []int64{C_Try}, []int64{C_Endtry})
                    if err {
                        catchFound = false
                        catchDistance = 0
                    }

                    if catchFound {
                        // Jump to first catch block
                        parser.pc += catchDistance
                    } else {
                        // Jump to endtry
                        parser.pc += endtryDistance
                    }
                }
                continue // Skip the rest of this iteration
            }

            if interactive && !we.assign && we.result != nil {
                pf("%+v\n", we.result)
            }

        } // end-statements-case

    } // end-pc-loop

    if structMode && !typeInvalid {
        // incomplete struct definition
        pf("Open STRUCT definition %v\n", structName)
        finish(true, ERR_SYNTAX)
    }

    lastlock.RLock()
    si = sig_int
    lastlock.RUnlock()

    if debugMode && ifs < 3 {
        pf("[#fyellow]Debugger active at program end. Entering final pause.[#-]\n")
        key := (uint64(source_base) << 32) | uint64(parser.pc)
        debugger.enterDebugger(key, functionspaces[source_base], ident, &mident, &gident)
        activeDebugContext = nil
    }

    if !si {

        // populate return variable in the caller with retvals
        calllock.Lock()
        // populate method_result
        if method {
            method_result, _ = vget(nil, ifs, ident, "self")
        }

        // populate captured_result
        if isTryBlock {
            // Find the try block info to get captured variable names
            for _, tryBlock := range tryBlockRegistry {
                if tryBlock.functionSpace == csloc {
                    captured_result = make([]any, len(tryBlock.capturedVars))
                    for i, varName := range tryBlock.capturedVars {
                        value, _ := vget(nil, ifs, ident, varName)
                        captured_result[i] = value
                    }
                    break
                }
            }
        }
        if retvalues != nil {
            calltable[ifs].retvals = retvalues
        }
        calltable[ifs].disposable = true
        calllock.Unlock()

        // clean up

        // pf("Leaving call with ifs of %d [fs:%s]\n\n",ifs,fs)
        // pf("[#2]about to delete %v[#-]\n",fs)
        // pf("about to enter call de-allocation with fs of '%s'\n",fs)

        // drop allocated names
        if varmode != MODE_STATIC {
            fnlookup.lmdelete(fs)
            numlookup.lmdelete(ifs)
        }

        // we keep a record here of recently disposed functionspace names
        //  so that mem_summary can label disposed of function allocations.
        lastlock.Lock()
        lastfunc[ifs] = fs
        lastlock.Unlock()

    }

    // Determine if this is a recursive call (same function appears more than once in the callChain)
    if enableProfiling {
        chain := getCallChain(ctx)
        if isRecursive(chain) {
            // Record or flag that this profile is recursive
            pathKey := collapseCallPath(chain)
            profileMu.Lock()
            if _, exists := profiles[pathKey]; !exists {
                profiles[pathKey] = &ProfileContext{Times: make(map[string]time.Duration)}
            }
            profiles[pathKey].Times["recursive"] = 1 // special marker
            profileMu.Unlock()
        } else {
            // Record execution time only if not a recursive call
            recordExclusiveExecutionTime(ctx, chain, time.Since(startTime))
        }
    }

    calllock.Lock()
    if len(errorChain) > 0 {
        errorChain = errorChain[:len(errorChain)-1]
        if enableProfiling {
            popCallChain(ctx)
        }
    }
    calllock.Unlock()

    return retval_count, endFunc, method_result, captured_result, callErr

}

func system(cmds string, display bool) (cop struct {
    Out  string
    Err  string
    Code int
    Okay bool
}) {

    if hasOuter(cmds, '`') {
        cmds = stripOuter(cmds, '`')
    }
    cmds = str.Trim(cmds, " \t\n")

    var cmdList []string
    lastpos := 0
    var squote, dquote, bquote bool
    var escMode bool
    var e int
    for ; e < len(cmds); e++ {
        if escMode {
            switch cmds[e] {
            case 'n':
                if !(dquote || squote || bquote) {
                    cmdList = append(cmdList, cmds[lastpos:e-1])
                    lastpos = e + 1
                }
            }
        } else {
            switch cmds[e] {
            case '"':
                dquote = !dquote
            case '\'':
                squote = !squote
            case '`':
                bquote = !bquote
            case '\\':
                if !escMode {
                    escMode = true
                    continue
                }
            }
        }
        escMode = false
    }
    cmdList = append(cmdList, cmds[lastpos:e])

    final_out := ""
    for _, cmd := range cmdList {
        cop = Copper(cmd, false)
        if display {
            pf("%s", cop.Out)
        } else {
            final_out += cop.Out + "\n"
        }
        // pf("sys: [%3d] : %s\n",k,cmd)
        // pf("cmdout: %+v\n",cop)
    }

    if !display {
        cop.Out = str.Trim(final_out, "\n")
    }

    return cop
}

// / execute a command in the shell coprocess or parent
// / used when string already interpolated and result is not required
// / currently only used by SYM_BOR statement processing.
func coprocCall(s string) {
    s = str.TrimRight(s, "\n")
    if len(s) > 0 {

        // find index of first pipe, then remove everything upto and including it
        _, cet, _ := str.Cut(s, "|")

        // strip outer quotes
        cet = str.Trim(cet, " \t\n")
        if hasOuter(cet, '`') {
            cet = stripOuter(cet, '`')
        }

        cop := Copper(cet, false)
        if !cop.Okay {
            pf("Error: [%d] in shell command '%s'\n", cop.Code, str.TrimLeft(s, " \t"))
            if interactive {
                pf(cop.Err)
            }
        } else {
            if len(cop.Out) > 0 {
                if cop.Out[len(cop.Out)-1] != '\n' {
                    cop.Out += "\n"
                }
                pf("%s", cop.Out)
            }
        }
    }
}

// / print user-defined function definition(s) to stdout
func ShowDef(fn string) bool {

    var ifn uint32
    var present bool
    if ifn, present = fnlookup.lmget(fn); !present {
        // pf("COULD NOT FIND NAME IN FNLOOKUP!\n")
        return false
    }

    // pf("(sd) ifn -> %v , max -> %v\n",ifn,len(functionspaces))
    // pf("(sd) basecode ->\n%+v\n",basecode[ifn])
    if ifn < uint32(len(functionspaces)) {

        var falist []string
        for _, fav := range functionArgs[ifn].args {
            falist = append(falist, fav)
        }

        first := true

        for q := range functionspaces[ifn] {
            strOut := "\t\t "
            if first == true {
                first = false
                fargs := str.Join(falist, ",")
                strOut = sf("\n[#4][#bold]%s", fn)
                strOut = str.Replace(strOut, "~", sf("(%v) ~ in struct ", fargs), 1)
                if str.Index(strOut, "~") == -1 {
                    strOut += sf("(%v)", fargs)
                }
                strOut += "[#boff][#-]\n\t\t "
            }
            pf(sparkle(str.ReplaceAll(sf("%s%s\n", strOut, basecode[ifn][q].Original), "%", "%%")))
        }
    }
    return true
}

// / search token list for a given delimiter string
func findDelim(tokens []Token, delim int64, start int16) (pos int16) {
    n := 0
    for p := start; p < int16(len(tokens)); p += 1 {
        if tokens[p].tokType == LParen {
            n += 1
        }
        if tokens[p].tokType == RParen {
            n -= 1
        }
        if n == 0 && tokens[p].tokType == delim {
            return p
        }
    }
    return -1
}

// getMultiarchTriplets returns possible multiarch triplets for the current architecture
func getMultiarchTriplets() []string {
    var archToTriplet = map[string][]string{
        // x86 family
        "amd64": {"x86_64-linux-gnu", "x86_64-linux-musl"},
        "386":   {"i386-linux-gnu", "i686-linux-gnu"},

        // ARM family
        "arm64": {"aarch64-linux-gnu"},
        "arm":   {"arm-linux-gnueabihf", "arm-linux-gnueabi"},

        // PowerPC family
        "ppc64":   {"powerpc64-linux-gnu"},
        "ppc64le": {"powerpc64le-linux-gnu"},
        "ppc":     {"powerpc-linux-gnu"},

        // Other architectures
        "s390x":    {"s390x-linux-gnu"},
        "mips":     {"mips-linux-gnu"},
        "mipsle":   {"mipsel-linux-gnu"},
        "mips64":   {"mips64-linux-gnuabi64"},
        "mips64le": {"mips64el-linux-gnuabi64"},
        "riscv64":  {"riscv64-linux-gnu"},
    }

    if triplets, ok := archToTriplet[runtime.GOARCH]; ok {
        return triplets
    }
    return []string{}
}

// tryLdconfigPath attempts to find a library using ldconfig -p
// Returns the full path if found, empty string otherwise
func tryLdconfigPath(libName string) string {
    cmd := exec.Command("ldconfig", "-p")
    output, err := cmd.Output()
    if err != nil {
        return ""
    }

    // Parse ldconfig output: "libname.so.X (libc6,x86-64) => /path/to/lib"
    lines := str.Split(string(output), "\n")
    for _, line := range lines {
        line = str.TrimSpace(line)
        if str.HasPrefix(line, libName+" ") || str.HasPrefix(line, libName+"(") {
            // Extract path after "=>"
            parts := str.Split(line, "=>")
            if len(parts) == 2 {
                path := str.TrimSpace(parts[1])
                if path != "" {
                    return path
                }
            }
        }
    }

    return ""
}

// getSystemLibraryPaths returns a comprehensive list of library search paths
// for the current OS and architecture
func getSystemLibraryPaths(libName string) []string {
    var paths []string

    if runtime.GOOS == "windows" {
        // Windows DLL search order (standard LoadLibrary behavior)
        // https://learn.microsoft.com/en-us/windows/win32/dlls/dynamic-link-library-search-order

        // 1. Application directory (where Za.exe is running from)
        if exePath, err := os.Executable(); err == nil {
            exeDir := filepath.Dir(exePath)
            paths = append(paths, filepath.Join(exeDir, libName))
        }

        // 2. System directory (System32 for native arch, SysWOW64 for 32-bit on 64-bit)
        // Note: On 64-bit Windows, System32 contains 64-bit DLLs, SysWOW64 contains 32-bit DLLs
        if sysDir := os.Getenv("SystemRoot"); sysDir != "" {
            paths = append(paths,
                filepath.Join(sysDir, "System32", libName),
                filepath.Join(sysDir, "SysWOW64", libName),
            )
        } else {
            // Fallback if SystemRoot not set
            paths = append(paths,
                "C:\\Windows\\System32\\"+libName,
                "C:\\Windows\\SysWOW64\\"+libName,
            )
        }

        // 3. Windows directory
        if winDir := os.Getenv("SystemRoot"); winDir != "" {
            paths = append(paths, filepath.Join(winDir, libName))
        } else {
            paths = append(paths, "C:\\Windows\\"+libName)
        }

        // 4. Current working directory (security note: can be a risk in some scenarios)
        if cwd, err := os.Getwd(); err == nil {
            paths = append(paths, filepath.Join(cwd, libName))
        }

        // 5. PATH environment variable directories
        if pathEnv := os.Getenv("PATH"); pathEnv != "" {
            for _, dir := range str.Split(pathEnv, ";") {
                if dir != "" {
                    paths = append(paths, filepath.Join(dir, libName))
                }
            }
        }

        return paths
    }

    // Platform-specific path ordering
    // Darwin (macOS), BSD variants, Linux, Solaris/illumos each have different conventions

    triplets := getMultiarchTriplets()

    if runtime.GOOS == "darwin" {
        // macOS paths - search order per dyld documentation
        // Note: /usr/local/lib may not be searched by default on newer macOS (Xcode 15+)
        // but we include it for compatibility with older systems and Homebrew
        paths = append(paths,
            "/usr/local/lib/"+libName,      // Homebrew
            "/opt/homebrew/lib/"+libName,   // Homebrew on Apple Silicon
            "/opt/local/lib/"+libName,      // MacPorts
            "/usr/lib/"+libName,            // System libraries
            "/Library/Frameworks/"+libName, // System frameworks location (rare for .dylib)
        )
        return paths
    }

    if runtime.GOOS == "freebsd" || runtime.GOOS == "dragonfly" {
        // FreeBSD and DragonFly BSD paths
        // Third-party packages install to /usr/local by convention
        paths = append(paths,
            "/usr/local/lib/"+libName,           // Primary for user-installed packages
            "/usr/lib/"+libName,                 // System libraries
            "/lib/"+libName,                     // Base system libraries
            "/usr/local/lib/compat/pkg/"+libName, // Compatibility packages
            "/usr/lib/compat/"+libName,          // Compatibility libraries
        )
        return paths
    }

    if runtime.GOOS == "openbsd" {
        // OpenBSD paths - simpler than FreeBSD
        paths = append(paths,
            "/usr/local/lib/"+libName, // User-installed packages
            "/usr/lib/"+libName,       // System libraries
        )
        return paths
    }

    if runtime.GOOS == "netbsd" {
        // NetBSD paths - uses /usr/pkg for pkgsrc packages
        paths = append(paths,
            "/usr/pkg/lib/"+libName,   // pkgsrc packages (primary)
            "/usr/local/lib/"+libName, // Local installations
            "/usr/lib/"+libName,       // System libraries
        )
        return paths
    }

    if runtime.GOOS == "solaris" || runtime.GOOS == "illumos" {
        // Solaris and illumos paths
        // 64-bit and 32-bit libraries use different subdirectories
        is64bit := runtime.GOARCH == "amd64" || runtime.GOARCH == "arm64" ||
                   runtime.GOARCH == "ppc64" || runtime.GOARCH == "ppc64le" ||
                   runtime.GOARCH == "s390x" || runtime.GOARCH == "mips64" ||
                   runtime.GOARCH == "mips64le" || runtime.GOARCH == "riscv64"

        if is64bit {
            paths = append(paths,
                "/lib/64/"+libName,       // 64-bit system libraries (primary)
                "/usr/lib/64/"+libName,   // 64-bit user libraries
                "/lib/"+libName,          // Fallback to 32-bit
                "/usr/lib/"+libName,      // Fallback to 32-bit
            )
        } else {
            paths = append(paths,
                "/lib/"+libName,         // 32-bit system libraries
                "/usr/lib/"+libName,     // 32-bit user libraries
            )
        }
        return paths
    }

    // Linux paths (various distributions)
    // Priority order: multiarch-specific -> standard lib64 -> standard lib -> BSD-style -> multilib

    // 1. Multiarch paths (Debian/Ubuntu/derivatives)
    for _, triplet := range triplets {
        paths = append(paths,
            "/usr/lib/"+triplet+"/"+libName,
            "/lib/"+triplet+"/"+libName,
            "/usr/local/lib/"+triplet+"/"+libName,
        )
    }

    // 2. Standard 64-bit paths (RHEL/Fedora/CentOS/AWS Linux 2023 primary)
    // AWS Linux 2023 is Fedora-based and uses the same lib64 structure
    paths = append(paths,
        "/usr/lib64/"+libName,
        "/lib64/"+libName,
    )

    // 3. Standard primary paths (Arch/Gentoo/Void/most systems)
    paths = append(paths,
        "/usr/lib/"+libName,
        "/lib/"+libName,
    )

    // 4. /usr/local paths (common on many systems, though not all Linux distros use it)
    paths = append(paths,
        "/usr/local/lib/"+libName,
    )

    // 5. 32-bit multilib paths (Arch and others)
    paths = append(paths,
        "/usr/lib32/"+libName,
        "/lib32/"+libName,
    )

    // Note: NixOS is intentionally not included here as it uses /nix/store/<hash>-package/lib/
    // and relies entirely on RPATH embedded in binaries or NIX_LD_LIBRARY_PATH.
    // NixOS users should either:
    // 1. Use full paths: MODULE "/nix/store/.../lib/libfoo.so"
    // 2. Set LD_LIBRARY_PATH to the nix store location
    // 3. Use the system's dlopen which will find libraries via RPATH

    return paths
}

func (parser *leparser) splitCommaArray(tokens []Token) (resu [][]Token) {

    evnest := 0
    newstart := 0
    lt := 0

    if lt = len(tokens); lt == 0 {
        return resu
    }

    for term := range tokens {
        nt := tokens[term]
        if nt.tokType == LParen {
            evnest += 1
        }
        if nt.tokType == RParen {
            evnest -= 1
        }
        if evnest == 0 {
            if nt.tokType == O_Comma {
                v := tokens[newstart:term]
                resu = append(resu, v)
                newstart = term + 1
            }
            if term == lt-1 {
                v := tokens[newstart : term+1]
                resu = append(resu, v)
                newstart = term + 1
                continue
            }
        }
    }
    return resu

}

func (parser *leparser) evalCommaArray(ifs uint32, tokens []Token) (resu []any, errs []error) {

    evnest := 0
    newstart := 0
    lt := 0

    if lt = len(tokens); lt == 0 {
        return resu, errs
    }

    for term := range tokens {
        nt := tokens[term]
        if nt.tokType == LParen {
            evnest += 1
        }
        if nt.tokType == RParen {
            evnest -= 1
        }
        if evnest == 0 {
            if term == lt-1 {
                v, e := parser.Eval(ifs, tokens[newstart:term+1])
                resu = append(resu, v)
                errs = append(errs, e)
                newstart = term + 1
                continue
            }
            if nt.tokType == O_Comma {
                v, e := parser.Eval(ifs, tokens[newstart:term])
                resu = append(resu, v)
                errs = append(errs, e)
                newstart = term + 1
            }
        }
    }
    return resu, errs

}

// print / println / log handler
// when logging, user must decide for themselves if they want a LF at end.
func (parser *leparser) console_output(tokens []Token, ifs uint32, ident *[]Variable, sourceLine int16, interactive bool, lf bool, logging bool, sparkly bool) {
    plog_out := ""
    if len(tokens) > 0 {
        evnest := 0
        newstart := 0
        for term := range tokens {
            nt := tokens[term]
            if nt.tokType == LParen || nt.tokType == LeftSBrace {
                evnest += 1
            }
            if nt.tokType == RParen || nt.tokType == RightSBrace {
                evnest -= 1
            }
            if evnest == 0 && (term == len(tokens)-1 || nt.tokType == O_Comma) {
                v, e := parser.Eval(ifs, tokens[newstart:term+1])
                if e != nil {
                    parser.report(sourceLine, sf("Error in PRINT term evaluation: %s", e))
                    finish(false, ERR_EVAL)
                    break
                }
                newstart = term + 1
                switch v.(type) {
                case string:
                    v = interpolate(parser.namespace, ifs, ident, v.(string))
                }
                // Check if pretty array formatting should be applied (for both sparkly and non-sparkly)
                if isArrayType(v) && (interactive || prettyArrays) {
                    formatted := formatArrayPretty(v)
                    if sparkly {
                        formatted = sparkle(formatted)
                    }
                    if logging {
                        plog_out += sf(`%v`, formatted)
                    } else {
                        pf(`%v`, formatted)
                    }
                } else {
                    // Use normal formatting
                    if sparkly {
                        if logging {
                            plog_out += sf(`%v`, sparkle(v))
                        } else {
                            pf(`%v`, sparkle(v))
                        }
                    } else {
                        if logging {
                            plog_out += sf(`%v`, v)
                        } else {
                            pf(`%v`, v)
                        }
                    }
                }
                continue
            }
        }
        if logging {
            plog("%v", plog_out)
            return
        }
        if interactiveFeed || lf {
            pf("\n")
        }
    } else {
        pf("\n")
    }
}

func joinTokens(tokens []Token) string {
    var sb str.Builder
    for _, t := range tokens {
        sb.WriteString(t.tokText)
    }
    return str.Trim(sb.String(), " \t")
}

func (parser *leparser) processArgumentTokens(tokens []Token, dargs *[]string, argTypes *[]string, hasDefault *[]bool, defaults *[]any, loc uint32, ifs uint32, ident *[]Variable) {
    // Find positions of : and = tokens
    colonPos := -1
    eqPos := -1
    for i, t := range tokens {
        if t.tokType == SYM_COLON && colonPos == -1 {
            colonPos = i
        }
        if t.tokType == O_Assign {
            eqPos = i
            break
        }
    }

    var argName string
    var argType string

    // Parse patterns:
    // param              -> argType = ""
    // param:type         -> argType = "type"
    // param=default      -> argType = ""
    // param:type=default -> argType = "type"

    if colonPos != -1 {
        // Type annotation present
        argName = joinTokens(tokens[0:colonPos])
        if eqPos > colonPos {
            // param:type=default
            argType = joinTokens(tokens[colonPos+1 : eqPos])
            defaultExprTokens := tokens[eqPos+1:]

            evaluated := parser.wrappedEval(ifs, ident, ifs, ident, defaultExprTokens)
            if evaluated.evalError {
                parser.report(-1, sf("Error evaluating default for argument '%s': %v", argName, evaluated.errVal))
                finish(false, ERR_EVAL)
                return
            }

            *dargs = append(*dargs, argName)
            *argTypes = append(*argTypes, argType)
            *hasDefault = append(*hasDefault, true)
            *defaults = append(*defaults, evaluated.result)
        } else {
            // param:type (no default)
            argType = joinTokens(tokens[colonPos+1:])

            *dargs = append(*dargs, argName)
            *argTypes = append(*argTypes, argType)
            *hasDefault = append(*hasDefault, false)
            *defaults = append(*defaults, nil)
        }
    } else if eqPos != -1 {
        // Default value present, no type
        argName = joinTokens(tokens[0:eqPos])
        defaultExprTokens := tokens[eqPos+1:]

        evaluated := parser.wrappedEval(ifs, ident, ifs, ident, defaultExprTokens)
        if evaluated.evalError {
            parser.report(-1, sf("Error evaluating default for argument '%s': %v", argName, evaluated.errVal))
            finish(false, ERR_EVAL)
            return
        }

        *dargs = append(*dargs, argName)
        *argTypes = append(*argTypes, "") // any/untyped
        *hasDefault = append(*hasDefault, true)
        *defaults = append(*defaults, evaluated.result)
    } else {
        // No type, no default
        argName = joinTokens(tokens)
        *dargs = append(*dargs, argName)
        *argTypes = append(*argTypes, "") // any/untyped
        *hasDefault = append(*hasDefault, false)
        *defaults = append(*defaults, nil)
    }

    // Bind argument name to local scope
    bind_int(loc, argName)
}

// setupTypedParameter initializes a typed parameter variable before assignment
// This sets up ITyped, IKind, Kind_override and IValue so that vset() can do type checking
func setupTypedParameter(fs uint32, ident *[]Variable, name string, typeStr string, namespace string) {
    bin := bind_int(fs, name)
    if bin >= uint64(len(*ident)) {
        newident := make([]Variable, bin+identGrowthSize)
        copy(newident, *ident)
        *ident = newident
    }

    t := &(*ident)[bin]
    t.IName = name
    t.ITyped = true
    t.declared = true
    t.Kind_override = typeStr

    // Check if it's a struct type (with namespace resolution)
    sname := typeStr
    if !str.Contains(typeStr, "::") {
        sname = namespace + "::" + typeStr
    }

    // Resolve struct name through use_chain
    resolvedName := uc_match_struct(sname)
    lookupName := sname
    if resolvedName != "" {
        lookupName = resolvedName + "::" + sname
    }

    structmapslock.RLock()
    _, isStruct := structmaps[lookupName]
    if !isStruct {
        _, isStruct = structmaps[sname]  // Fallback to exact lookup
    }
    structmapslock.RUnlock()

    if isStruct {
        // Struct type - Kind_override already set, ITyped handled by struct system
        t.Kind_override = sname
        t.ITyped = false // Let struct system handle this
        return
    }

    // Set IKind and initialize IValue based on type string
    if str.HasPrefix(typeStr, "[]") {
        switch typeStr {
        case "[]bool":
            t.IKind = ksbool
            t.IValue = []bool{}
        case "[]int":
            t.IKind = ksint
            t.IValue = []int{}
        case "[]int64":
            t.IKind = ksint64
            t.IValue = []int64{}
        case "[]uint":
            t.IKind = ksuint
            t.IValue = []uint{}
        case "[]uint64":
            t.IKind = ksuint64
            t.IValue = []uint64{}
        case "[]float":
            t.IKind = ksfloat
            t.IValue = []float64{}
        case "[]string":
            t.IKind = ksstring
            t.IValue = []string{}
        case "[]bigi":
            t.IKind = ksbigi
            t.IValue = []*big.Int{}
        case "[]bigf":
            t.IKind = ksbigf
            t.IValue = []*big.Float{}
        case "[]any", "[]mixed", "[]":
            t.IKind = ksany
            t.IValue = []any{}
        default:
            // Complex multi-dimensional - use dynamic type
            t.IKind = kdynamic
            reflectType := parseAndConstructType(typeStr)
            if reflectType != nil {
                t.IValue = reflect.New(reflectType).Elem().Interface()
            }
        }
        return
    }

    // Base types - set IKind and IValue
    switch typeStr {
    case "nil":
        t.IKind = knil
        t.IValue = nil
    case "bool":
        t.IKind = kbool
        t.IValue = false
    case "int":
        t.IKind = kint
        t.IValue = 0
    case "int64":
        t.IKind = kint64
        t.IValue = int64(0)
    case "uint":
        t.IKind = kuint
        t.IValue = uint(0)
    case "uint64", "uxlong":
        t.IKind = kuint64
        t.IValue = uint64(0)
    case "uint8", "byte":
        t.IKind = kbyte
        t.IValue = uint8(0)
    case "float":
        t.IKind = kfloat
        t.IValue = 0.0
    case "string":
        t.IKind = kstring
        t.IValue = ""
    case "bigi":
        t.IKind = kbigi
        t.IValue = big.NewInt(0)
    case "bigf":
        t.IKind = kbigf
        t.IValue = big.NewFloat(0)
    case "any", "mixed":
        t.IKind = kany
        t.ITyped = false // any type doesn't need type checking
    case "map":
        t.IKind = kmap
        t.IValue = make(map[string]any)
    case "pointer":
        t.IKind = kpointer
        t.IValue = nil
    default:
        // Unknown type - might be a struct or complex type
        // Don't enable strict type checking for unknown types
        t.ITyped = false
    }
}

// parseReturnTypes parses comma-separated return type tokens into a slice of type strings
func parseReturnTypes(tokens []Token) []string {
    var types []string
    var currentTokens []Token

    for _, tok := range tokens {
        if tok.tokType == O_Comma {
            if len(currentTokens) > 0 {
                types = append(types, joinTokens(currentTokens))
                currentTokens = nil
            }
        } else {
            currentTokens = append(currentTokens, tok)
        }
    }

    // Handle the last type (or single type)
    if len(currentTokens) > 0 {
        types = append(types, joinTokens(currentTokens))
    }

    return types
}

// isCompatibleType checks if a value is compatible with the expected type string
func isCompatibleType(value any, expectedType string, namespace string) bool {
    if expectedType == "" || expectedType == "any" || expectedType == "mixed" {
        return true // any type accepted
    }

    // Check for struct type
    sname := expectedType
    if !str.Contains(expectedType, "::") {
        sname = namespace + "::" + expectedType
    }

    // Resolve struct name through use_chain
    resolvedName := uc_match_struct(sname)
    lookupName := sname
    if resolvedName != "" {
        lookupName = resolvedName + "::" + sname
    }

    structmapslock.RLock()
    _, isStructType := structmaps[lookupName]
    if !isStructType {
        _, isStructType = structmaps[sname]  // Fallback to exact lookup
    }
    structmapslock.RUnlock()

    if isStructType {
        // Use struct_match to check if value matches the struct type
        matchedName, count := struct_match(value)
        if count == 1 && matchedName == sname {
            return true
        }
        return false
    }

    // Check for slice types
    if str.HasPrefix(expectedType, "[]") {
        if value == nil {
            return true // nil is valid for slice types
        }
        vt := reflect.TypeOf(value)
        if vt == nil {
            return true
        }
        if vt.Kind() != reflect.Slice && vt.Kind() != reflect.Array {
            return false
        }
        // For now, accept any slice/array for slice types
        // More specific type checking could be added later
        return true
    }

    // Check for map type
    if expectedType == "map" || str.HasPrefix(expectedType, "map[") {
        if value == nil {
            return true
        }
        vt := reflect.TypeOf(value)
        if vt == nil {
            return true
        }
        return vt.Kind() == reflect.Map
    }

    // Check built-in types
    switch expectedType {
    case "bool":
        _, ok := value.(bool)
        return ok
    case "int":
        _, ok := value.(int)
        return ok
    case "int64":
        _, ok := value.(int64)
        return ok
    case "uint":
        _, ok := value.(uint)
        return ok
    case "uint64":
        _, ok := value.(uint64)
        return ok
    case "uint8", "byte":
        _, ok := value.(uint8)
        return ok
    case "float":
        _, ok := value.(float64)
        return ok
    case "string":
        _, ok := value.(string)
        return ok
    case "bigi":
        _, ok := value.(*big.Int)
        return ok
    case "bigf":
        _, ok := value.(*big.Float)
        return ok
    case "nil":
        return value == nil
    case "pointer":
        _, ok := value.(*CPointerValue)
        return ok || value == nil
    }

    return true // Unknown type, accept
}

func handleTestResult(ifs uint32, passed bool, sourceLine int16, exprText string, msg string) {
    testlock.Lock()
    defer testlock.Unlock()

    group_name_string := ""
    if test_group != "" {
        group_name_string += test_group + "/"
    }
    if test_name != "" {
        group_name_string += test_name
    }

    var test_report string
    if passed {
        if under_test {
            test_report = sf("[#4]TEST PASSED %s (%s/line %d) : %s[#-]",
                group_name_string, getReportFunctionName(ifs, false), 1+sourceLine, msg)
            testsPassed++
            appendToTestReport(test_output_file, ifs, parser.pc, test_report)
        }
    } else {
        if under_test {
            test_report = sf("[#2]TEST FAILED %s (%s/line %d) : %s[#-]",
                group_name_string, getReportFunctionName(ifs, false), 1+sourceLine, msg)
            testsFailed++
            appendToTestReport(test_output_file, ifs, 1+sourceLine, test_report)
        }
        temp_test_assert := test_assert
        if fail_override != "" {
            temp_test_assert = fail_override
        }
        switch temp_test_assert {
        case "fail":
            parser.report(sourceLine, msg)
            finish(false, ERR_ASSERT)
        case "continue":
            parser.report(sourceLine, msg+" (but continuing)")
        }
    }
}

func isTruthy(val any) bool {
    switch v := val.(type) {
    case bool:
        return v
    case int, int32, int64:
        return v != 0
    case float32, float64:
        return v != 0.0
    case string:
        return v != ""
    default:
        return val != nil
    }
}

// parseAndConstructType dynamically constructs reflect.Type for multi-dimensional arrays, slices, and maps
func parseAndConstructType(typeStr string) reflect.Type {
    // Handle multi-dimensional slices: "[][]int", "[][][]string"
    if str.HasPrefix(typeStr, "[]") {
        innerType := parseAndConstructType(typeStr[2:])
        if innerType != nil {
            return reflect.SliceOf(innerType)
        }
    }

    // Handle fixed arrays: "[5]int", "[3][2]string"
    if str.HasPrefix(typeStr, "[") {
        rbPos := str.Index(typeStr, "]")
        if rbPos > 1 {
            sizeStr := typeStr[1:rbPos]
            if size, err := strconv.Atoi(sizeStr); err == nil && size >= 0 {
                innerType := parseAndConstructType(typeStr[rbPos+1:])
                if innerType != nil {
                    return reflect.ArrayOf(size, innerType)
                }
            }
        }
    }

    // Handle multi-dimensional maps: map[], map[][], etc.
    if str.HasPrefix(typeStr, "map[") && str.HasSuffix(typeStr, "]") {
        // Count the dimension brackets - map[], map[][], map[][][], etc.
        remaining := typeStr[4:] // Skip "map["
        bracketCount := 0
        for i := 0; i < len(remaining); i += 2 {
            if i+1 < len(remaining) && remaining[i] == '[' && remaining[i+1] == ']' {
                bracketCount++
            } else {
                break
            }
        }
        // If we parsed the entire string as bracket pairs, it's valid
        if bracketCount > 0 && remaining == str.Repeat("[]", bracketCount) {
            // All maps in Za are map[string]any regardless of nesting depth
            return reflect.TypeOf(map[string]any{})
        }
    }

    // Handle Za type aliases
    if typeStr == "mixed" {
        typeStr = "any"
    }
    if typeStr == "interface{}" {
        typeStr = "any"
    }

    // Base case: lookup in existing Typemap
    if baseType, exists := Typemap[typeStr]; exists {
        return baseType
    }

    return nil // Unknown type
}

// executeTryBlocks handles try block execution when a panic occurs
// Returns true if the exception was handled, false if it should continue to standard error handling
func executeTryBlocks(ctx context.Context, tryBlocks []tryBlockInfo, err error, ident *[]Variable, inbound *Phrase) bool {
    // For basic implementation, we'll execute all try blocks in order
    // More sophisticated logic for catch/finally blocks will be added later

    for _, tryBlock := range tryBlocks {
        // Execute the try block as a separate function call
        // The try block shares the same *ident (variable scope) as the parent function

        // Get the try block function space name
        tryFSName, exists := numlookup.lmget(tryBlock.functionSpace)
        if !exists {
            continue // Skip if function space not found
        }

        // Create a new call location for the try block execution
        // Note: @ symbol is followed by auto-generated ID, avoid conflict with exec_@
        loc, _ := GetNextFnSpace(true, tryFSName+"_handler@", call_s{
            prepared:  true,
            base:      tryBlock.functionSpace,
            caller:    tryBlock.parentFS,
            gc:        false,
            gcShyness: 100,
        })

        // Set up fileMap entry for handler function space
        // Handler function spaces should have the same file mapping as the try block function space
        if tryBlockFileMap, exists := fileMap.Load(tryBlock.functionSpace); exists {
            fileMap.Store(loc, tryBlockFileMap)
        }

        // Capture variables from parent scope if this try block has captured variables
        var capturedVarsValues []any
        if len(tryBlock.capturedVars) > 0 {
            capturedVarsValues = make([]any, len(tryBlock.capturedVars))
            for i, varName := range tryBlock.capturedVars {
                // Get the variable's current value from parent scope
                value, _ := vget(nil, tryBlock.parentFS, ident, varName)
                capturedVarsValues[i] = value
            }
        }

        // Create a fresh ident table for try block execution (like regular function calls)
        var tryIdent = make([]Variable, identInitialSize)

        // Execute the try block
        // For trap calls, we don't have parser context, so use 0
        atomic.StoreInt32(&calltable[loc].callLine, 0) // Trap calls don't have parser context
        _, _, _, capturedResult, callErr := Call(ctx, MODE_NEW, &tryIdent, loc, ciTrap, false, nil, "", []string{}, capturedVarsValues, err)

        if callErr == nil {
            // Try block executed successfully, consider the exception handled

            // Repopulate parent scope with modified captured variables
            if capturedResult != nil && len(capturedResult) > 0 && len(tryBlock.capturedVars) > 0 {
                for i, varName := range tryBlock.capturedVars {
                    if i < len(capturedResult) {
                        // Update the variable in parent scope
                        vset(nil, tryBlock.parentFS, ident, varName, capturedResult[i])
                    }
                }
            }

            return true
        }

    }

    // No try block handled the exception
    return false
}

// findApplicableTryBlocks uses the registry to find try blocks that can handle the current exception
// Returns try blocks in the correct order (innermost first) for nested scenarios
func findApplicableTryBlocks(ctx context.Context, currentFS uint32, executionPath []uint32) []tryBlockInfo {
    var applicableTryBlocks []tryBlockInfo

    tryBlockRegistryLock.RLock()
    defer tryBlockRegistryLock.RUnlock()

    // Find all try blocks that are applicable to the current execution context
    for _, tryInfo := range tryBlockRegistry {
        // Check if this try block is in the execution path
        if isTryBlockApplicable(tryInfo, currentFS, executionPath) {
            applicableTryBlocks = append(applicableTryBlocks, *tryInfo)
        }
    }

    // Sort try blocks by nesting level (innermost first)
    // Higher nest level means more deeply nested, should be handled first
    for i := 0; i < len(applicableTryBlocks); i++ {
        for j := i + 1; j < len(applicableTryBlocks); j++ {
            if applicableTryBlocks[i].nestLevel < applicableTryBlocks[j].nestLevel {
                // Swap to put higher nest level first
                applicableTryBlocks[i], applicableTryBlocks[j] = applicableTryBlocks[j], applicableTryBlocks[i]
            }
        }
    }

    return applicableTryBlocks
}

// isTryBlockApplicable determines if a try block can handle an exception in the current context
func isTryBlockApplicable(tryInfo *tryBlockInfo, currentFS uint32, executionPath []uint32) bool {

    // Check if the try block's parent function space is in the execution path
    for _, pathFS := range executionPath {
        if pathFS == tryInfo.parentFS {
            return true
        }
    }

    // Check if the current function space matches the try block's parent
    if currentFS == tryInfo.parentFS {
        return true
    }

    // For user-defined functions: check if the current function's base matches the try block's parent
    calllock.RLock()
    currentBase := calltable[currentFS].base
    calllock.RUnlock()
    if currentBase == tryInfo.parentFS {
        return true
    }

    // Enhanced logic for nested try blocks:
    // If we're executing inside a try block function space, trace back to find the original parent

    calllock.RLock()
    defer calllock.RUnlock()

    // Build a complete call chain from current execution context
    var fullCallChain []uint32
    fullCallChain = append(fullCallChain, currentFS)

    // Add execution path
    fullCallChain = append(fullCallChain, executionPath...)

    // Trace back through the call table to find all parent function spaces
    for _, fs := range fullCallChain {
        if fs < uint32(len(calltable)) {
            // Check if this function space has a caller
            caller := calltable[fs].caller
            if caller != 0 {
                // Get the caller's function space name to check if it's a try block
                callerName, exists := numlookup.lmget(caller)
                if exists {
                    // If the caller is not a try block function space, check if it matches our try block's parent
                    if !str.Contains(callerName, "try_block_") && caller == tryInfo.parentFS {
                        return true
                    }

                    // If the caller is a try block, continue tracing back
                    if str.Contains(callerName, "try_block_") {
                        // Recursively check the caller's caller
                        grandCaller := calltable[caller].caller
                        if grandCaller != 0 && grandCaller == tryInfo.parentFS {
                            return true
                        }
                    }
                }
            }

            // Direct match with try block's parent
            if fs == tryInfo.parentFS {
                return true
            }
        }
    }

    return false
}

// executeApplicableTryBlocks executes try blocks found by the registry in proper order
func executeApplicableTryBlocks(ctx context.Context, applicableTryBlocks []tryBlockInfo, err error, ident *[]Variable, inbound *Phrase) bool {
    for _, tryBlock := range applicableTryBlocks {
        // Execute the try block as a separate function call
        // The try block shares the same *ident (variable scope) as the parent function

        // Get the try block function space name
        tryFSName, exists := numlookup.lmget(tryBlock.functionSpace)
        if !exists {
            continue // Skip if function space not found
        }

        // Create a new call location for the try block execution
        loc, _ := GetNextFnSpace(true, tryFSName+"_handler@", call_s{
            prepared:  true,
            base:      tryBlock.functionSpace,
            caller:    tryBlock.parentFS,
            gc:        false,
            gcShyness: 100,
        })

        // Set up fileMap entry for handler function space
        // Handler function spaces should have the same file mapping as the try block function space
        if tryBlockFileMap, exists := fileMap.Load(tryBlock.functionSpace); exists {
            fileMap.Store(loc, tryBlockFileMap)
        }

        // Capture variables from parent scope if this try block has captured variables
        var capturedVarsValues []any
        if len(tryBlock.capturedVars) > 0 {
            capturedVarsValues = make([]any, len(tryBlock.capturedVars))
            for i, varName := range tryBlock.capturedVars {
                // Get the variable's current value from parent scope
                value, _ := vget(nil, tryBlock.parentFS, ident, varName)
                capturedVarsValues[i] = value
            }
        }

        // Create a fresh ident table for try block execution (like regular function calls)
        var tryIdent = make([]Variable, identInitialSize)

        // Execute the try block
        // Set the callLine field in the calltable entry before calling the function
        // For trap calls, we don't have parser context, so use 0
        atomic.StoreInt32(&calltable[loc].callLine, 0) // Trap calls don't have parser context
        _, _, _, capturedResult, callErr := Call(ctx, MODE_NEW, &tryIdent, loc, ciTrap, false, nil, "", []string{}, capturedVarsValues, err)

        if callErr == nil {
            // Try block executed successfully, consider the exception handled

            // Repopulate parent scope with modified captured variables
            if capturedResult != nil && len(capturedResult) > 0 && len(tryBlock.capturedVars) > 0 {
                for i, varName := range tryBlock.capturedVars {
                    if i < len(capturedResult) {
                        // Update the variable in parent scope
                        vset(nil, tryBlock.parentFS, ident, varName, capturedResult[i])
                    }
                }
            }

            return true
        }

        // If try block also failed, continue to next try block or fall through
    }

    // No try block handled the exception
    return false
}
