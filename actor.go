package main

import (
    "io/ioutil"
    "math"
    "log"
    "net/http"
    "os"
    "path/filepath"
    "reflect"
    "regexp"
    "sync"
    "strconv"
    "runtime"
    str "strings"
    "time"
)

var siglock = &sync.RWMutex{}

// finish : flag the machine state as okay or in error and optionally
// terminates execution.
func finish(hard bool, i int) {
    if hard {
        os.Exit(i)
    }

    if !interactive {
        os.Exit(i)
    }

    if lockSafety { siglock.Lock() }
    sig_int = true
    if lockSafety { siglock.Unlock() }

}

// GetAsFloat : converts a variety of types to a float
func GetAsFloat(unk interface{}) (float64, bool) {
    switch i := unk.(type) {
    case float64:
        return i, false
    case float32:
        return float64(i), false
    case int:
        return float64(i), false
    case int32:
        return float64(i), false
    case int64:
        return float64(i), false
    case uint8:
        return float64(i), false
    case uint:
        return float64(i), false
    case uint32:
        return float64(i), false
    case uint64:
        return float64(i), false
    case string:
        p, e := strconv.ParseFloat(i, 64)
        return p, e != nil
    default:
        return math.NaN(), true
    }
}

// GetAsInt32 : converts a variety of types to int32
func GetAsInt32(expr interface{}) (int32, bool) {
    switch i := expr.(type) {
    case float32:
        return int32(i), false
    case float64:
        return int32(i), false
    case uint:
        return int32(i), false
    case uint8:
        return int32(i), false
    case uint32:
        return int32(i), false
    case uint64:
        return int32(i), false
    case int:
        return int32(i), false
    case int32:
        return int32(i), false
    case int64:
        return int32(i), false
    case string:
        p, e := strconv.ParseFloat(i, 64)
        if e == nil {
            return int32(p), false
        }
    }
    return 0, true
}

func GetAsInt(expr interface{}) (int, bool) {
    switch i := expr.(type) {
    case float32:
        return int(i), false
    case float64:
        return int(i), false
    case uint:
        return int(i), false
    case uint8:
        return int(i), false
    case uint32:
        return int(i), false
    case uint64:
        return int(i), false
    case int:
        return int(i), false
    case int32:
        return int(i), false
    case int64:
        return int(i), false
    case string:
        p, e := strconv.ParseFloat(i, 64)
        if e == nil {
            return int(p), false
        }
    default:
        // special case: these need rationalising eventually...
        switch sf("%T",expr) {
        case "float32":
            return int(i.(float32)), false
        case "float64":
            return int(i.(float64)), false
        case "uint":
            return int(i.(uint)), false
        case "uint8":
            return int(i.(uint8)), false
        case "uint32":
            return int(i.(uint32)), false
        case "uint64":
            return int(i.(uint64)), false
        case "int":
            return int(i.(int)), false
        case "int32":
            return int(i.(int32)), false
        case "int64":
            return int(i.(int64)), false
        case "string":
            p,e:=strconv.ParseFloat(expr.(string),64)
            if e==nil {
                return int(p),false
            }
        default:
            // pf("\n\n*debug* GetAsInt default on type %T\n\n",expr)
        }
    }
    return 0, true
}

// EvalCrush* used in C_If, C_Exit, C_For and C_Debug:

// EvalCrush() : take all tokens from tok[] between tstart and tend inclusive, compact and return evaluated answer.
// if no evalError then returns a "validated" true bool
func EvalCrush(fs uint64, tok []Token, tstart int, tend int) (interface{}, bool) {
    expr,_ := wrappedEval(fs, crushEvalTokens(tok[tstart:tend+1]), false)
    if expr.evalError { return expr.result,false }
    return expr.result, true
}

// as evalCrush but operate over all remaining tokens from tstart onwards
func EvalCrushRest(fs uint64, tok []Token, tstart int) (interface{}, bool) {
    expr,_ := wrappedEval(fs, crushEvalTokens(tok[tstart:]), false)
    if expr.evalError { return expr.result,false }
    return expr.result, true
}

// check for value in slice
func InSlice(a int, list []int) bool {
    for _, b := range list {
        if b == a {
            return true
        }
    }
    return false
}

func InStringSlice(a string, list []string) bool {
    for _, b := range list {
        if b == a {
            return true
        }
    }
    return false
}

//
// LOOK-AHEAD FUNCTIONS
//

func searchToken(base uint64, start int, end int, sval string) bool {

    range_fs:=functionspaces[base][start:end]

    for _, v := range range_fs {
        if len(v.Tokens) == 0 {
            continue
        }
        for r := 0; r < len(v.Tokens); r++ {
            if v.Tokens[r].tokType == Identifier && v.Tokens[r].tokText == sval {
                return true
            }
            // check for direct reference
            if str.Contains(v.Tokens[r].tokText, sval) {
                return true
            }
            // on *any* indirect reference return true, as we can't be sure without following the interpolation.
            if str.Contains(v.Tokens[r].tokText,"{{") {
                return true
            }
        }
    }
    return false
}

// used by if..else..endif and similar constructs for nesting

func lookahead(fs uint64, startLine int, startlevel int, endlevel int, term int, indenters []int, dedenters []int) (bool, int, bool) {

    indent := startlevel
    range_fs:=functionspaces[fs][startLine:]

    for i, v := range range_fs {

        if len(v.Tokens) == 0 {
            continue
        }

        statement := v.Tokens[0].tokType

        // indents and dedents
        if InSlice(statement, indenters) {
            indent++
        }
        if InSlice(statement, dedenters) {
            indent--
        }
        if indent < endlevel {
            return false, 0, true
        }

        // found search term?
        if indent == endlevel && statement == term {
            return true, i, false
        }
    }

    // return found, distance, false
    return false, -1, false

}


// find the next available slot for a function or module
//  definition in the functionspace[] list.
func GetNextFnSpace() uint64 {
    for q := uint64(1); q < MaxUint64; q++ {
        if _, extant := numlookup.lmget(q); extant {
            continue
        }
        return q
    }
    pf("Error: no more function space available.\n")
    finish(true, ERR_FATAL)
    return 0
}


// redraw margins - called after a SIGWINCH
func pane_redef() {
    MW, MH, _ = GetSize(1)
    winching = false
}

var calllock   = &sync.RWMutex{}
var lastlock   = &sync.RWMutex{}
var fspacelock = &sync.RWMutex{}
var farglock   = &sync.RWMutex{}
var looplock   = &sync.RWMutex{}
var newfnlock  = &sync.RWMutex{}
var globlock   = &sync.RWMutex{}


// defined function entry point
// csloc is ID in [id][]Phrase function spaces
func Call(ifs uint64, base uint64, varmode int, csloc uint64, va ...interface{}) (endFunc bool, breakOut int, continueOut int) {

    // pf("Entered call -> ifs %v : va -> %v\n",ifs,va)

    var inbound *Phrase

    defer func() {
        if r := recover(); r != nil {
            if _, ok := r.(runtime.Error); ok {
                pf("Fatal error on (%v)\n",inbound.Original)
                pf(sparkle("[#2]Details:\n%v[#-]\n"),r)
                if debug_level==0 { os.Exit(127) }
            }
            err := r.(error)
            pf("error : %v\n",err)
            panic(r)
        }
    }()

    var breakIn int
    var pc int
    var retvars []string
    var retvals []interface{}
    var finalline int
    var fs string
    var caller uint64
    var ncs call_s

    // this is used to return the phrase counter to modify to if a CONTINUE was encountered.
    continueOut = -1

    // get last call details
    // if lockSafety { calllock.RLock() }
    ncs = callstack[csloc]
    // if lockSafety { calllock.RUnlock() }

    // set up the function space
    fs = ncs.fs
    base = ncs.base
    caller = ncs.caller
    retvars = ncs.retvars

    if base==0 {
        if !interactive {
            report(ifs, "Possible race condition. Please check. Base->0")
            finish(false,ERR_EVAL)
            return
        }
    }

    // if lockSafety { looplock.Lock() }

    if varmode == MODE_NEW {

        ifs = GetNextFnSpace()

        numlookup.lmset(ifs, sf("%s@%v", fs, ifs))
        fnlookup.lmset(sf("%s@%v",fs,ifs),ifs)
        // in interactive mode, the current functionspace is 0
        // in normal exec mode, the source is treated as functionspace 1

        if !interactive && base < 2 {
            globalaccess = ifs
            vset(globalaccess, "userSigIntHandler", "")
        }

        // create the local variable storage for the function
        vcreatetable(ifs, &vtable_maxreached, VAR_CAP)

        // nesting levels in this function
        depth[ifs] = 0

        if lockSafety { vlock.Lock() }
        varcount[ifs] = 0
        if lockSafety { vlock.Unlock() }

        if lockSafety { lastlock.Lock() }
        inside_test = false
        test_group = ""
        test_name = ""
        test_assert = ""
        lastConstruct[ifs] = []int{}
        if lockSafety { lastlock.Unlock() }

    }

    // initialise condition states: WHEN stack depth
    // initialise the loop positions: FOR, FOREACH, WHILE
    wccount[ifs] = 0

    // allocate loop storage space if not a repeat ifs value.
    if ifs>=vtable_maxreached {
        loops[ifs] = make([]s_loop, MAX_LOOPS)
    }

    // if lockSafety { looplock.Unlock() }

    // assign value to local vars named in functionArgs (the call parameters) from each 
    // va value (functionArgs[] created at definition time from the call signature).

    if len(va) > len(functionArgs[base]) {
        report(ifs,"Syntax error: too many call arguments provided.")
        finish(false,ERR_SYNTAX)
        return
    }

    if functionArgs[base]!=nil {
        if len(functionArgs[base])>len(va) {
            for e:=0; e<(len(functionArgs[base])-len(va)); e++ {
                va=append(va,"")
            }
        }
    }

    if len(va) > 0 {
        for q, v := range va {
            fa:=functionArgs[base][q]
            vset(ifs,fa,v)
        }
    }

    // pre-calculate the len to save the extra overhead during the call loop
    finalline = len(functionspaces[base])

    var defining bool               // are we currently defining a function
    var definitionName string       // ... if we are, what is it called


    pc = -1 // program counter : increments to zero at start of loop

    for {

        pc++  // program counter, equates to each Phrase struct in the function

        si:=sig_int

        if pc >= finalline || endFunc || si {
            break
        }

        // race condition: winching check
        if winching {
            pane_redef()
        }

        // cache the next Phrase: for readability and performance (we read from this slice index often)
        inbound = &functionspaces[base][pc]

        tokencount := inbound.TokenCount // length of phrase


        // set these for global internal access: 
        if lockSafety { lastlock.Lock() }
        lastfs   = ifs
        lastbase = base
        lastline = inbound.Tokens[0].Line
        llcopy:=lastline
        if lockSafety { lastlock.Unlock() }

        // .. skip comments and DOC statements
        if inbound.Tokens[0].tokType == C_Doc && !testMode {
            continue
        }

        if tokencount == 1 { // if the entire line is a placeholding non-statement then skip
            switch inbound.Tokens[0].tokType {
            case C_Semicolon, EOL, EOF:
                continue
            }
        }

        lastIndex := tokencount - 1 // index of final token

        // remove trailing C_Semicolon token remnants
        if tokencount > 1 {
            if inbound.Tokens[lastIndex].tokType == C_Semicolon {
                inbound.TokenCount--
                tokencount = lastIndex
                lastIndex--
                inbound.Tokens = inbound.Tokens[:tokencount]
            }
        }
        // lastIndex not used below here!


        // finally... start processing the statement.

        ondo_reenter:

        statement := inbound.Tokens[0]

        // append statements to a function if currently inside a DEFINE block.
        if defining && statement.tokType != C_Enddef {
            //.. add to functionspace
            lmv,_:=fnlookup.lmget(definitionName)
            fspacelock.Lock()
            functionspaces[lmv] = append(functionspaces[lmv], *inbound)
            fspacelock.Unlock()
            continue
        }

        // abort this phrase if currently inside a TEST block but the test flag is not set.
        if lockSafety { lastlock.RLock() }
        if inside_test {
            if lockSafety { lastlock.RUnlock() }
            if statement.tokType != C_Endtest && !under_test {
                continue
            }
        }
        if lockSafety { lastlock.RUnlock() }


        ////////////////////////////////////////////////////////////////////////////////////////////////////////
        // BREAK here if required
        if breakIn != Error {
            // breakIn holds either Error or a token_type for ending the current construct
            if statement.tokType != breakIn {
                continue
            }
        }
        ////////////////////////////////////////////////////////////////////////////////////////////////////////


        // main parsing for statements starts here:

        switch statement.tokType {

        case C_While:

            endfound, enddistance, _ := lookahead(base, pc, 0, 0, C_Endwhile, []int{C_While}, []int{C_Endwhile})

            if !endfound {
                report(ifs, "Could not find an ENDWHILE.")
                finish(false, ERR_SYNTAX)
                break
            }

            // if cond false, then jump to end while
            // if true, stack the cond then continue

            // eval

            var res bool
            var cet ExpressionCarton
            if len(inbound.Tokens)==1 {
                cet = crushEvalTokens([]Token{Token{tokType: Expression, tokText:"true"}})
                res=true
            } else {
                cet = crushEvalTokens(inbound.Tokens[1:])
                expr,ef := wrappedEval(ifs, cet, false)
                if ef || expr.evalError { break }
                switch expr.result.(type) {
                case bool:
                    res = expr.result.(bool)
                default:
                    report(ifs,"WHILE condition must evaluate to boolean.")
                    finish(false,ERR_EVAL)
                    break
                }
            }

            if isBool(res) && res {
                // while cond is true, stack, then continue loop
                if lockSafety { looplock.Lock() }
                depth[ifs]++
                loops[ifs][depth[ifs]] = s_loop{repeatFrom: pc, whileContinueAt: pc + enddistance, repeatCond: cet, loopType: C_While}
                if lockSafety { looplock.Unlock() }

                if lockSafety { lastlock.Lock() }
                lastConstruct[ifs] = append(lastConstruct[ifs], C_While)
                if lockSafety { lastlock.Unlock() }
                break
            } else {
                // goto endwhile
                pc += enddistance
            }


        case C_Endwhile:

            // re-evaluate, on true jump back to start, on false, destack and continue

            cond := loops[ifs][depth[ifs]]

            if cond.loopType != C_While {
                report(ifs, "ENDWHILE outside of WHILE loop.")
                finish(false, ERR_SYNTAX)
                break
            }

            // time to die?
            if breakIn == C_Endwhile {

                if lockSafety { lastlock.Lock() }
                lastConstruct[ifs] = lastConstruct[ifs][:depth[ifs]-1]
                if lockSafety { lastlock.Unlock() }

                if lockSafety { looplock.Lock() }
                depth[ifs]--
                if lockSafety { looplock.Unlock() }

                breakIn = Error
                break
            }

            // eval
            expr,ef := wrappedEval(ifs, cond.repeatCond, true)
            if ef || expr.evalError { break }

            if expr.result.(bool) {
                // while still true, loop 
                pc = cond.repeatFrom
            } else {
                // was false, so leave the loop
                if lockSafety { lastlock.Lock() }
                lastConstruct[ifs] = lastConstruct[ifs][:depth[ifs]-1]
                if lockSafety { lastlock.Unlock() }
                if lockSafety { looplock.Lock() }
                depth[ifs]--
                if lockSafety { looplock.Unlock() }
            }


        case C_SetGlob: // set the value of a global variable.

           if tokencount<3 {
                // error
                report(ifs,"missing value in setglob.")
                finish(false,ERR_SYNTAX)
                break
            }

            aryRef:=false
            lhs:=""
            eqAt := findDelim(inbound.Tokens, "=", 2)
            if eqAt != -1 {
                lhsExpr := ""
                for i:=1;i<eqAt;i++ {
                    lhsExpr=lhsExpr+inbound.Tokens[i].tokText
                }
                lhs=lhsExpr
            } else {
                eqAt=1
                lhs = inbound.Tokens[1].tokText
            }

            var elementComponents string

            if lockSafety {globlock.Lock() }

            var sqPos int
            if sqPos=str.IndexByte(lhs,'['); sqPos!=-1 {
                // find token pos of "]"
                sqEndPos:=str.IndexByte(lhs,']')
                if sqEndPos==-1 { // missing end brace
                    report(ifs,sf("SETGLOB missing end brace in '%v'",lhs))
                    finish(false,ERR_SYNTAX)
                    break
                }
                if sqEndPos<sqPos { // wrong order
                    report(ifs,"SETGLOB braces out-of-order\n")
                    finish(false,ERR_SYNTAX)
                    break
                }
                elementComponents=lhs[sqPos+1:sqEndPos]
                aryRef=true
            }

            // eval rhs
            cet := crushEvalTokens(inbound.Tokens[eqAt+1:])
            expr,ef := wrappedEval(ifs, cet, true)
            if ef || expr.evalError {
                report(ifs,sf("Bad expression in SETGLOB : '%s'",expr.text))
                finish(false,ERR_EVAL)
                break
            }

            // now process variables in lhs index
            if aryRef {

                // array reference
                element, ef, _ := ev(ifs, elementComponents, true)
                if ef {
                    report(ifs,sf("Bad element in SETGLOB assignment: '%v'",elementComponents))
                    finish(false,ERR_EVAL)
                    break
                }

                aryName := lhs[:sqPos]

                if _, found := VarLookup(globalaccess, aryName); !found {
                    vset(globalaccess, aryName, make(map[string]interface{}, 31))
                }

                switch element.(type) {
                case string:
                    vsetElement(globalaccess, aryName, interpolate(ifs,element.(string)), expr.result)
                case int:
                    // error on negative element
                    if element.(int)<0 {
                        report(ifs,sf("Negative array element found in SETGLOB (%v,%v,%v)",ifs,aryName,element.(int)))
                        finish(false,ERR_EVAL)
                        break
                    }
                    // otherwise, set global array element
                    vsetElement(globalaccess, aryName, interpolate(ifs,sf("%v",element)), expr.result)
                default:
                    report(ifs,"Unknown type in SETGLOB")
                    if lockSafety {globlock.Unlock() }
                    os.Exit(125)
                }

            } else {
                vset(globalaccess, interpolate(ifs,lhs), expr.result)
            }

            if lockSafety {globlock.Unlock() }


        case C_Foreach:

            // FOREACH var IN expr
            // iterates over the result of expression expr as a list

            if tokencount<4 {
                report(ifs,"bad argument length in FOREACH.")
                finish(false,ERR_SYNTAX)
                break
            }

            if str.ToLower(inbound.Tokens[2].tokText) != "in" {
                report(ifs, "malformed FOREACH statement.")
                finish(false, ERR_SYNTAX)
                break
            }

            if inbound.Tokens[1].tokType != Identifier {
                report(ifs, "parameter 2 must be an identifier.")
                finish(false, ERR_SYNTAX)
                break
            }

            var iterType int
            var ce int

            fid := interpolate(ifs,inbound.Tokens[1].tokText)

            switch inbound.Tokens[3].tokType {

            case NumericLiteral, StringLiteral, LeftSBrace, Identifier, Expression, C_AssCommand:

                exp := crushEvalTokens(inbound.Tokens[3:])
                var validated bool
                var expr interface{}

                determinedValue, ok := vget(ifs, exp.text)

                if ok {
                    expr = determinedValue
                    validated = true
                } else {

                    wrappedEval,ef := wrappedEval(ifs, exp, false)
                    if ef || wrappedEval.evalError {
                        report(ifs,sf("error evaluating term in FOREACH statement '%v'",exp.text))
                        finish(false,ERR_EVAL)
                        break
                    }

                    validated=true
                    expr=wrappedEval.result

                }

                if expr == "" {
                    // skip empty expressions
                    endfound, enddistance, _ := lookahead(base, pc, 1, 0, C_Endfor, []int{C_Foreach}, []int{C_Endfor})
                    if !endfound {
                        report(ifs, "Cannot determine the location of a matching ENDFOR.")
                        finish(false, ERR_SYNTAX)
                        break
                    } else { //skip
                        pc += enddistance
                        break
                    }
                }

                var finalExprString string
                var finalExprArray interface{}
                var iter *reflect.MapIter

                if validated {

                    iterType = IT_LINE // default

                    switch expr.(type) {

                    case string:

                        // split and treat as array if multi-line

                        // remove a single trailing \n from string
                        elast := len(expr.(string)) - 1
                        if expr.(string)[elast] == '\n' {
                            expr = expr.(string)[:elast]
                        }

                        // split up string at \n divisions into an array
                        finalExprArray = str.Split(expr.(string), "\n")
                        if len(finalExprArray.([]string))>0 {
                            vset(ifs, "key_"+fid, 0)
                            vset(ifs, fid, finalExprArray.([]string)[0])
                            ce = len(finalExprArray.([]string)) - 1
                        }

                    case map[string]string:

                        finalExprArray = expr
                        if len(finalExprArray.(map[string]string)) > 0 {

                            // get iterator for this map
                            iter = reflect.ValueOf(finalExprArray.(map[string]string)).MapRange()

                            // set initial key and value
                            if iter.Next() {
                                vset(ifs, "key_"+fid, iter.Key().String())
                                vset(ifs, fid, iter.Value().Interface())
                            } else {
                                // empty
                            }
                            ce = len(finalExprArray.(map[string]string)) - 1
                        }

                    case map[string][]string:

                        finalExprArray = expr
                        if len(finalExprArray.(map[string][]string)) > 0 {

                            // get iterator for this map
                            iter = reflect.ValueOf(finalExprArray.(map[string][]string)).MapRange()

                            // set initial key and value
                            if iter.Next() {
                                vset(ifs, "key_"+fid, iter.Key().String())
                                vset(ifs, fid, iter.Value().Interface())
                            } else {
                                // empty
                            }
                            ce = len(finalExprArray.(map[string][]string)) - 1
                        }

                    case http.Header:

                        finalExprArray = expr
                        if len(finalExprArray.(http.Header)) > 0 {

                            // get iterator for this map
                            iter = reflect.ValueOf(finalExprArray.(http.Header)).MapRange()

                            // set initial key and value
                            if iter.Next() {
                                vset(ifs, "key_"+fid, iter.Key().String())
                                vset(ifs, fid, iter.Value().Interface())
                            } else {
                                // empty
                            }
                            ce = len(finalExprArray.(http.Header)) - 1
                        }

                    case map[string]interface{}:

                        finalExprArray = expr
                        if len(finalExprArray.(map[string]interface{})) > 0 {

                            // get iterator for this map
                            iter = reflect.ValueOf(finalExprArray.(map[string]interface{})).MapRange()

                            // set initial key and value
                            if iter.Next() {
                                vset(ifs, "key_"+fid, iter.Key().String())
                                vset(ifs, fid, iter.Value().Interface())
                            } else {
                                // empty
                            }
                            ce = len(finalExprArray.(map[string]interface{})) - 1
                        }

                    case []interface{}:

                        finalExprArray = expr
                        if len(finalExprArray.([]interface{})) > 0 {
                            vset(ifs, "key_"+fid, 0)
                            vset(ifs, fid, finalExprArray.([]interface{})[0])
                            ce = len(finalExprArray.([]interface{})) - 1
                        }

                    case []float64:

                        finalExprArray = expr
                        if len(finalExprArray.([]float64)) > 0 {
                            vset(ifs, "key_"+fid, 0)
                            vset(ifs, fid, finalExprArray.([]float64)[0])
                            ce = len(finalExprArray.([]float64)) - 1
                        }

                    case float64: // special case: float
                        finalExprArray = []float64{expr.(float64)}
                        if len(finalExprArray.([]float64)) > 0 {
                            vset(ifs, "key_"+fid, 0)
                            vset(ifs, fid, finalExprArray.([]float64)[0])
                            ce = len(finalExprArray.([]float64)) - 1
                        }

                    case []uint8:
                        finalExprArray = expr
                        if len(finalExprArray.([]uint8)) > 0 {
                            vset(ifs, "key_"+fid, 0)
                            vset(ifs, fid, finalExprArray.([]uint8)[0])
                            ce = len(finalExprArray.([]uint8)) - 1
                        }

                    case []int:
                        finalExprArray = expr
                        if len(finalExprArray.([]int)) > 0 {
                            vset(ifs, "key_"+fid, 0)
                            vset(ifs, fid, finalExprArray.([]int)[0])
                            ce = len(finalExprArray.([]int)) - 1
                        }

                    case int: // special case: int
                        finalExprArray = []int{expr.(int)}
                        if len(finalExprArray.([]int)) > 0 {
                            vset(ifs, "key_"+fid, 0)
                            vset(ifs, fid, finalExprArray.([]int)[0])
                            ce = len(finalExprArray.([]int)) - 1
                        }

                    case []int32:
                        finalExprArray = expr
                        if len(finalExprArray.([]int32)) > 0 {
                            vset(ifs, "key_"+fid, 0)
                            vset(ifs, fid, finalExprArray.([]int32)[0])
                            ce = len(finalExprArray.([]int32)) - 1
                        }

                    case []int64:
                        finalExprArray = expr
                        if len(finalExprArray.([]int64)) > 0 {
                            vset(ifs, "key_"+fid, 0)
                            vset(ifs, fid, finalExprArray.([]int64)[0])
                            ce = len(finalExprArray.([]int64)) - 1
                        }

                    case []float32:
                        finalExprArray = expr
                        if len(finalExprArray.([]float32)) > 0 {
                            vset(ifs, "key_"+fid, 0)
                            vset(ifs, fid, finalExprArray.([]float32)[0])
                            ce = len(finalExprArray.([]float32)) - 1
                        }

                    case []string:
                        finalExprArray = expr
                        if len(finalExprArray.([]string)) > 0 {
                            vset(ifs, "key_"+fid, 0)
                            vset(ifs, fid, finalExprArray.([]string)[0])
                            ce = len(finalExprArray.([]string)) - 1
                        }

                    default:
                        report(ifs,sf("Mishandled return of type '%T' from FOREACH expression '%v'\n", expr,expr))
                        finish(false,ERR_EVAL)
                        break
                    }

                    // figure end position
                    endfound, enddistance, _ := lookahead(base, pc, 0, 0, C_Endfor, []int{C_Foreach}, []int{C_Endfor})
                    if !endfound {
                        report(ifs, "Cannot determine the location of a matching ENDFOR.")
                        finish(false, ERR_SYNTAX)
                        break
                    }

                    if lockSafety { looplock.Lock() }
                    depth[ifs]++
                    // pf("ifs:%v depth:%v len_depth:%v len_loops:%v\n",ifs,depth[ifs],len(depth),len(loops))
                    loops[ifs][depth[ifs]] = s_loop{loopVar: fid, repeatFrom: pc + 1, iterOverMap: iter,
                        iterOverString: finalExprString, iterOverArray: finalExprArray,
                        ecounter: 0, econdEnd: ce, forEndPos: enddistance + pc,
                        loopType: C_Foreach, iterType: iterType,
                    }
                    if lockSafety { looplock.Unlock() }

                    if lockSafety { lastlock.Lock() }
                    lastConstruct[ifs] = append(lastConstruct[ifs], C_Foreach)
                    if lockSafety { lastlock.Unlock() }

                }
            }

        case C_For: // loop over an int64 range

            if tokencount < 5 || inbound.Tokens[2].tokText != "=" {
                report(ifs, "Malformed FOR statement.\n")
                finish(false, ERR_SYNTAX)
                break
            }

            toAt := findDelim(inbound.Tokens, "to", 2)
            if toAt == -1 {
                report(ifs, "TO not found in FOR")
                finish(false, ERR_SYNTAX)
                break
            }

            stepAt := findDelim(inbound.Tokens, "step", toAt)
            stepped := true
            if stepAt == -1 {
                stepped = false
                stepAt = tokencount
            }

            var fstart, fend, fstep int
            var expr interface{}
            var validated bool

            if toAt>3 {
                expr, validated = EvalCrush(ifs, inbound.Tokens, 3, toAt-1)
                if validated && isNumber(expr) {
                    fstart, _ = GetAsInt(expr)
                } else {
                    report(ifs, "Could not evaluate start expression in FOR")
                    finish(false, ERR_EVAL)
                    break
                }
            } else {
                report(ifs,"Missing expression in FOR statement?")
                finish(false,ERR_SYNTAX)
                break
            }

            if tokencount>toAt+1 {
                expr, validated = EvalCrush(ifs, inbound.Tokens, toAt+1, stepAt-1)
                if validated && isNumber(expr) {
                    fend, _ = GetAsInt(expr)
                } else {
                    report(ifs, "Could not evaluate end expression in FOR")
                    finish(false, ERR_EVAL)
                    break
                }
            } else {
                report(ifs,"Missing expression in FOR statement?")
                finish(false,ERR_SYNTAX)
                break
            }

            if stepped {
                if tokencount>stepAt+1 {
                    expr, validated = EvalCrushRest(ifs, inbound.Tokens, stepAt+1)
                    if validated && isNumber(expr) {
                        fstep, _ = GetAsInt(expr)
                    } else {
                        report(ifs, "Could not evaluate STEP expression")
                        finish(false, ERR_EVAL)
                        break
                    }
                } else {
                    report(ifs,"Missing expression in FOR statement?")
                    finish(false,ERR_SYNTAX)
                    break
                }
            }

            step := 1
            if stepped {
                step = fstep
            }
            if step == 0 {
                report(ifs, "This is a road to nowhere. (STEP==0)")
                finish(true, ERR_EVAL)
                break
            }

            direction := ACT_INC
            if step < 0 {
                direction = ACT_DEC
            }

            // figure end position
            endfound, enddistance, _ := lookahead(base, pc, 0, 0, C_Endfor, []int{C_For}, []int{C_Endfor})
            if !endfound {
                report(ifs, "Cannot determine the location of a matching ENDFOR.")
                finish(false, ERR_SYNTAX)
                break
            }

            // @note: if loop counter is never used between here and C_Endfor, then don't vset the local var

            // store loop data
            if lockSafety { looplock.Lock() }
            depth[ifs]++
            loops[ifs][depth[ifs]] = s_loop{
                loopVar:  interpolate(ifs,inbound.Tokens[1].tokText),
                optNoUse: Opt_LoopStart,
                loopType: C_For, forEndPos: pc + enddistance, repeatFrom: pc + 1,
                counter: fstart, condEnd: fend,
                repeatAction: direction, repeatActionStep: step,
            }
            if lockSafety { looplock.Unlock() }

            // store loop start condition
            vset(ifs, interpolate(ifs,inbound.Tokens[1].tokText), fstart)

            if lockSafety { lastlock.Lock() }
            lastConstruct[ifs] = append(lastConstruct[ifs], C_For)
            if lockSafety { lastlock.Unlock() }


        case C_Endfor: // terminate a FOR or FOREACH block

            if lockSafety { looplock.Lock() }

            var lastcon int
            if lockSafety { lastlock.RLock() }
            if depth[ifs]>0 {
                lastcon=lastConstruct[ifs][depth[ifs]-1]
            }
            if lockSafety { lastlock.RUnlock() }

            if lastcon!=C_For && lastcon!=C_Foreach {
                if lockSafety { looplock.Unlock() }
                report(ifs,"ENDFOR without a FOR or FOREACH")
                finish(false,ERR_SYNTAX)
                break
            }

            //.. take address of map entry
            thisLoop := &loops[ifs][depth[ifs]]

            var loopEnd bool

            // perform cond action and check condition

            if breakIn!=C_Endfor {

                switch (*thisLoop).loopType {

                case C_Foreach: // move through range

                    (*thisLoop).ecounter++

                    if (*thisLoop).ecounter > (*thisLoop).econdEnd {
                        loopEnd = true
                    } else {

                        // assign value back to local variable
                        switch (*thisLoop).iterType {
                        case IT_LINE:
                            switch (*thisLoop).iterOverArray.(type) {
                            case map[string]interface{},map[string]string,http.Header,map[string][]string:
                                // map ranges are randomly ordered!!
                                if (*thisLoop).iterOverMap.Next() { // true means not exhausted
                                    vset(ifs, "key_"+(*thisLoop).loopVar, (*thisLoop).iterOverMap.Key().String())
                                    vset(ifs, (*thisLoop).loopVar, (*thisLoop).iterOverMap.Value().Interface())
                                }
                            case []interface{}:
                                vset(ifs, "key_"+(*thisLoop).loopVar, (*thisLoop).ecounter)
                                vset(ifs, (*thisLoop).loopVar, (*thisLoop).iterOverArray.([]interface{})[(*thisLoop).ecounter])
                            case []bool:
                                vset(ifs, "key_"+(*thisLoop).loopVar, (*thisLoop).ecounter)
                                vset(ifs, (*thisLoop).loopVar, (*thisLoop).iterOverArray.([]bool)[(*thisLoop).ecounter])
                            case []int:
                                vset(ifs, "key_"+(*thisLoop).loopVar, (*thisLoop).ecounter)
                                vset(ifs, (*thisLoop).loopVar, (*thisLoop).iterOverArray.([]int)[(*thisLoop).ecounter])
                            case []uint8:
                                vset(ifs, "key_"+(*thisLoop).loopVar, (*thisLoop).ecounter)
                                vset(ifs, (*thisLoop).loopVar, (*thisLoop).iterOverArray.([]uint8)[(*thisLoop).ecounter])
                            case []int32:
                                vset(ifs, "key_"+(*thisLoop).loopVar, (*thisLoop).ecounter)
                                vset(ifs, (*thisLoop).loopVar, (*thisLoop).iterOverArray.([]int32)[(*thisLoop).ecounter])
                            case []int64:
                                vset(ifs, "key_"+(*thisLoop).loopVar, (*thisLoop).ecounter)
                                vset(ifs, (*thisLoop).loopVar, (*thisLoop).iterOverArray.([]int64)[(*thisLoop).ecounter])
                            case []string:
                                vset(ifs, "key_"+(*thisLoop).loopVar, (*thisLoop).ecounter)
                                vset(ifs, (*thisLoop).loopVar, (*thisLoop).iterOverArray.([]string)[(*thisLoop).ecounter])
                            case []float32:
                                vset(ifs, "key_"+(*thisLoop).loopVar, (*thisLoop).ecounter)
                                vset(ifs, (*thisLoop).loopVar, (*thisLoop).iterOverArray.([]float32)[(*thisLoop).ecounter])
                            case []float64:
                                vset(ifs, "key_"+(*thisLoop).loopVar, (*thisLoop).ecounter)
                                vset(ifs, (*thisLoop).loopVar, (*thisLoop).iterOverArray.([]float64)[(*thisLoop).ecounter])
                            default:
                                 pv,_:=vget(ifs,sf("%v",(*thisLoop).iterOverArray.([]float64)[(*thisLoop).ecounter]))
                                 pf("Unknown type [%T] in END/Foreach\n",pv)
                            }
                        case IT_CHAR:
                            vset(ifs, (*thisLoop).loopVar, (string)((*thisLoop).iterOverString.(string)[(*thisLoop).ecounter]))
                        }

                        /*
                        pv,_:=vget(ifs,(*thisLoop).loopVar)
                        pf("endfor with new val-> <<%v>>\n",pv)
                        */

                    }

                case C_For: // move through range

                    (*thisLoop).counter += (*thisLoop).repeatActionStep

                    switch (*thisLoop).repeatAction {
                    case ACT_INC:
                        if (*thisLoop).counter > (*thisLoop).condEnd {
                            (*thisLoop).counter -= (*thisLoop).repeatActionStep
                            if (*thisLoop).optNoUse == Opt_LoopIgnore {
                                vset(ifs, (*thisLoop).loopVar, (*thisLoop).counter)
                            }
                            loopEnd = true
                        }
                    case ACT_DEC:
                        if (*thisLoop).counter < (*thisLoop).condEnd {
                            (*thisLoop).counter -= (*thisLoop).repeatActionStep
                            if (*thisLoop).optNoUse == Opt_LoopIgnore {
                                vset(ifs, (*thisLoop).loopVar, (*thisLoop).counter)
                            }
                            loopEnd = true
                        }
                    }

                    if (*thisLoop).optNoUse == Opt_LoopStart {
                        (*thisLoop).optNoUse = Opt_LoopIgnore
                        // check tokens once for loop var references, then set Opt_LoopSet if found.
                        if searchToken(base, (*thisLoop).repeatFrom, pc, (*thisLoop).loopVar) {
                            (*thisLoop).optNoUse = Opt_LoopSet
                        }
                    }

                    // assign loop counter value back to local variable
                    if (*thisLoop).optNoUse == Opt_LoopSet {
                        vset(ifs, (*thisLoop).loopVar, (*thisLoop).counter)
                    }

                }

            } else {
                // time to die, mr bond? C_Break reached
                breakIn = Error // reset to unbroken
                loopEnd = true
            }

            if loopEnd {
                // leave the loop
                if lockSafety { lastlock.Lock() }
                lastConstruct[ifs] = lastConstruct[ifs][:depth[ifs]-1]
                if lockSafety { lastlock.Unlock() }
                depth[ifs]--
                breakIn = Error // reset to unbroken
            } else {
                // jump back to start of block
                pc = (*thisLoop).repeatFrom - 1 // start of loop will do pc++
            }

            if lockSafety { looplock.Unlock() }

        case C_Continue:

            // Continue should work with FOR, FOREACH or WHILE.

            // if lockSafety { looplock.RLock() }
            di:=depth[ifs]
            // if lockSafety { looplock.RUnlock() }

            if di == 0 {
                report(ifs, "Attempting to CONTINUE without a valid surrounding construct.")
                finish(false, ERR_SYNTAX)
            } else {

                var thisLoop *s_loop

                // ^^ we use indirect access here (and throughout loop code) for a minor speed bump.
                // ^^ we should periodically review this as an optimisation in Go could make this unnecessary.

                // if lockSafety { lastlock.RLock() }
                switch lastConstruct[ifs][di-1] {

                case C_For, C_Foreach:
                    thisLoop = &loops[ifs][di]
                    pc = (*thisLoop).forEndPos - 1
                    continueOut=pc

                case C_While:
                    thisLoop = &loops[ifs][di]
                    pc = (*thisLoop).whileContinueAt - 1
                    continueOut=pc
                }
                // if lockSafety { lastlock.RUnlock() }

            }

        case C_Break:

            // Break should work with either FOR, FOREACH, WHILE or WHEN.
            // We use lastConstruct to establish which is the innermost
            // of these from which we need to break out.

            // The surrounding construct should set the lastConstruct[fs][depth] on entry.

            if depth[ifs] == 0 {
                report(ifs, "Attempting to BREAK without a valid surrounding construct.")
                finish(false, ERR_SYNTAX)
            } else {

                // jump calc, depending on break context

                var thisLoop *s_loop
                thisLoop = &loops[ifs][depth[ifs]]
                bmess := ""

                // if lockSafety { lastlock.RLock() }
                switch lastConstruct[ifs][depth[ifs]-1] {

                case C_For:
                    pc = (*thisLoop).forEndPos - 1
                    breakIn = C_Endfor
                    bmess = "out of FOR:\n"

                case C_Foreach:
                    pc = (*thisLoop).forEndPos - 1
                    breakIn = C_Endfor
                    bmess = "out of FOREACH:\n"

                case C_While:
                    pc = (*thisLoop).whileContinueAt - 1
                    breakIn = C_Endwhile
                    bmess = "out of WHILE:\n"

                case C_When:
                    pc = wc[wccount[ifs]].endLine - 1
                    bmess = "out of WHEN:\n"

                default:
                    report(ifs, "A grue is attempting to BREAK out. (Breaking without a surrounding context!)")
                    finish(false, ERR_SYNTAX)
                    break
                }
                // if lockSafety { lastlock.RUnlock() }

                if breakIn != Error {
                    debug(5, "** break %v\n", bmess)
                }

            }


        case C_Unset: // remove a variable

            if tokencount != 2 {
                report(ifs, "Incorrect arguments supplied for UNSET.")
                finish(false, ERR_SYNTAX)
            } else {
                removee := inbound.Tokens[1].tokText
                if _, ok := VarLookup(ifs, removee); ok {
                    vunset(ifs, removee)
                } else {
                    report(ifs, sf("Variable %s does not exist.", removee))
                    finish(false, ERR_EVAL)
                }
            }


        case C_Pane:

            if tokencount == 1 {
                pf("Current  %-24s %3s %3s %3s %3s %s\n","Name","y","x","h","w","Title")
                for p,v:=range panes {
                    def:=""
                    if p==currentpane { def="*" }
                    pf("%6s   %-24s %3d %3d %3d %3d %s\n",def,p,v.row,v.col,v.h,v.w,v.title)
                }
                break
            }

            switch str.ToLower(inbound.Tokens[1].tokText) {
            case "off":
                if tokencount != 2 {
                    report(ifs, "Too many arguments supplied.")
                    finish(false, ERR_SYNTAX)
                    break
                }
                // disable
                panes = make(map[string]Pane)
                panes["global"] = Pane{row: 0, col: 0, h: MH, w: MW + 1}
                currentpane = "global"

            case "select":

                if tokencount != 3 {
                    report(ifs, "Invalid pane selection.")
                    finish(false, ERR_SYNTAX)
                    break
                }

                cp, _, _ := ev(ifs, inbound.Tokens[2].tokText, true)

                switch cp.(type) {
                case string:
                    setPane(cp.(string))
                    currentpane = cp.(string)

                default:
                    report(ifs, "Warning: you must provide a string value to PANE SELECT.")
                    finish(false,ERR_EVAL)
                    break
                }

            case "define":

                var title = ""
                var boxed string = "round" // box style // none,round,square,double

                // Collect the expressions for each position
                //      pane define name , y , x , h , w [ , title [ , border ] ]

                nameCommaAt := findDelim(inbound.Tokens, ",", 3)
                   YCommaAt := findDelim(inbound.Tokens, ",", nameCommaAt+1)
                   XCommaAt := findDelim(inbound.Tokens, ",", YCommaAt+1)
                   HCommaAt := findDelim(inbound.Tokens, ",", XCommaAt+1)
                   WCommaAt := findDelim(inbound.Tokens, ",", HCommaAt+1)
                   TCommaAt := findDelim(inbound.Tokens, ",", WCommaAt+1)

                if nameCommaAt==-1 || YCommaAt==-1 || XCommaAt==-1 || HCommaAt==-1 {
                    report(ifs, "Bad delimiter in PANE DEFINE.")
                    // pf("Toks -> [%+v]\n", inbound.Tokens)
                    finish(false, ERR_SYNTAX)
                    break
                }

                hasTitle:=false; hasBox:=false
                if TCommaAt>-1 {
                    hasTitle=true
                    if TCommaAt<tokencount-1 {
                        hasBox=true
                    }
                }

                var ew,etit,ebox ExpressionCarton

                ename := crushEvalTokens(inbound.Tokens[ 2             : nameCommaAt ] )
                ey    := crushEvalTokens(inbound.Tokens[ nameCommaAt+1 : YCommaAt    ] )
                ex    := crushEvalTokens(inbound.Tokens[ YCommaAt+1    : XCommaAt    ] )
                eh    := crushEvalTokens(inbound.Tokens[ XCommaAt+1    : HCommaAt    ] )
                if hasTitle {
                    ew    = crushEvalTokens(inbound.Tokens[ HCommaAt+1:WCommaAt   ] )
                } else {
                    ew    = crushEvalTokens(inbound.Tokens[ HCommaAt+1: ] )
                }

                if hasTitle && hasBox {
                    etit = crushEvalTokens(inbound.Tokens[ WCommaAt+1 : TCommaAt ] )
                    ebox = crushEvalTokens(inbound.Tokens[ TCommaAt+1 : ] )
                } else {
                    if hasTitle {
                        etit = crushEvalTokens(inbound.Tokens[ WCommaAt+1 : ] )
                    }
                }

                var ptitle, pbox ExpressionCarton
                pname,_  := wrappedEval(ifs, ename, true)
                py,_     := wrappedEval(ifs, ey, true)
                px,_     := wrappedEval(ifs, ex, true)
                ph,_     := wrappedEval(ifs, eh, true)
                pw,_     := wrappedEval(ifs, ew, true)
                if hasTitle {
                    ptitle,_ = wrappedEval(ifs, etit, true)
                }
                if hasBox   {
                    pbox,_   = wrappedEval(ifs, ebox, true)
                }

                if pname.evalError || py.evalError || px.evalError || ph.evalError || pw.evalError {
                    report(ifs, "Could not evaluate an argument in PANE DEFINE.")
                    // pf("Toks -> [%+v]\n", inbound.Tokens)
                    finish(false, ERR_EVAL)
                    break
                }

                name         := sf("%v",pname.result)
                col,invalid1 := GetAsInt(px.result)
                row,invalid2 := GetAsInt(py.result)
                w,invalid3   := GetAsInt(pw.result)
                h,invalid4   := GetAsInt(ph.result)
                if hasTitle { title = sf("%v",ptitle.result) }
                if hasBox   { boxed = sf("%v",pbox.result)   }

                if invalid1 || invalid2 || invalid3 || invalid4 {
                    report(ifs,"Could not use an argument in PANE DEFINE.")
                    // pf("Toks -> [%+v]\n", inbound.Tokens)
                    finish(false,ERR_EVAL)
                    break
                }

                if pname.result.(string) == "global" {
                    report(ifs, "Cannot redefine the global PANE.")
                    finish(false, ERR_SYNTAX)
                    break
                }

                panes[name] = Pane{row: row, col: col, w: w, h: h, title: title, boxed: boxed}
                paneBox(name)

            case "redraw":
                paneBox(currentpane)

            default:
                report(ifs, "Unknown PANE command.")
                finish(false, ERR_SYNTAX)
            }


        case C_LocalCommand:

            bashCall(ifs,inbound.Original)


        case C_Pause:

            var i string

            if tokencount<2 {
                report(ifs, "Not enough arguments in PAUSE.")
                finish(false, ERR_SYNTAX)
                break
            }

            cet := crushEvalTokens(inbound.Tokens[1:])
            expr,ef := wrappedEval(ifs, cet, true)

            if ef || !expr.evalError {

                if isNumber(expr.result) {
                    i = sf("%v", expr.result)
                } else {
                    i = expr.result.(string)
                }

                dur, err := time.ParseDuration(i + "ms")

                if err != nil {
                    report(ifs, sf("'%s' did not evaluate to a duration.", expr.text))
                    finish(false, ERR_EVAL)
                    break
                }

                time.Sleep(dur)

            } else {
                report(ifs, sf("Could not evaluate PAUSE expression."))
                finish(false, ERR_EVAL)
                break
            }


        case C_Doc:
            var badval bool
            if testMode {
                if tokencount > 1 {
                    docout := ""
                    previousterm := 1
                    for term := range inbound.Tokens[1:] {
                        if inbound.Tokens[term].tokType == C_Comma {

                            expr,ef := wrappedEval(ifs, crushEvalTokens(inbound.Tokens[previousterm:term]), true)
                            if ef || expr.evalError { badval=true; break }

                            docout += sparkle(sf("%v", expr.result))
                            previousterm = term + 1

                        }
                    }

                    if badval { break }

                    expr,ef := wrappedEval(ifs, crushEvalTokens(inbound.Tokens[previousterm:]), true)
                    if ef || expr.evalError { break }

                    if !expr.evalError {
                        docout += sparkle(sf("%v", expr.result))
                    }
                    appendToTestReport(test_output_file,ifs, pc, docout)
                }
            }


        case C_Test:

            // TEST "name" GROUP "group_name" ASSERT FAIL|CONTINUE

            inside_test = true

            if testMode {

                if !(tokencount == 4 || tokencount == 6) {
                    report(ifs, "Badly formatted TEST command.")
                    finish(false, ERR_SYNTAX)
                    break
                }

                if str.ToLower(inbound.Tokens[2].tokText) != "group" {
                    report(ifs, "Missing GROUP in TEST command.")
                    finish(false, ERR_SYNTAX)
                    break
                }

                test_assert = "fail"
                if tokencount == 6 {
                    if str.ToLower(inbound.Tokens[4].tokText) != "assert" {
                        report(ifs, "Missing ASSERT in TEST command.")
                        finish(false, ERR_SYNTAX)
                        break
                    } else {
                        switch str.ToLower(inbound.Tokens[5].tokText) {
                        case "fail":
                            test_assert = "fail"
                        case "continue":
                            test_assert = "continue"
                        default:
                            report(ifs, "Bad ASSERT type in TEST command.")
                            finish(false, ERR_SYNTAX)
                            break
                        }
                    }
                }

                test_name = stripOuterQuotes(inbound.Tokens[1].tokText, 2)
                test_group = stripOuterQuotes(inbound.Tokens[3].tokText, 2)

                under_test = false
                // if filter matches group
                if matched, _ := regexp.MatchString(test_group_filter, test_group); matched {
                    under_test = true
                }

            }


        case C_Endtest:

            under_test = false
            inside_test = false


        case C_On:
            // ON expr DO action
            // was false? - discard command tokens and continue
            // was true? - reform command without the 'ON condition' tokens and re-enter command switch

            if tokencount > 2 {

                doAt := findDelim(inbound.Tokens, "do", 2)
                if doAt == -1 {
                    report(ifs, "DO not found in ON")
                    finish(false, ERR_SYNTAX)
                } else {
                    // more tokens after the DO to form a command with?
                    if tokencount >= doAt {

                        cet := crushEvalTokens(inbound.Tokens[1:doAt])

                        expr,ef := wrappedEval(ifs, cet, true)
                        if ef || expr.evalError {
                            report(ifs,"Could not evaluate expression in ON..DO statement.")
                            finish(false,ERR_EVAL)
                            break
                        }

                        switch expr.result.(type) {
                        case bool:
                            if expr.result.(bool) {

                                // create a phrase
                                p := Phrase{}
                                p.Tokens = inbound.Tokens[doAt+1:]
                                p.TokenCount = inbound.TokenCount - (doAt + 1)
                                p.Original = inbound.Original
                                p.Text = inbound.Text
                                // we can ignore .Text and .Original for now - but shouldn't
                                // they are only used in *Command calls, and the input is chomped
                                // from the front to the first pipe symbol so the 'ON expr DO' would
                                // be consumed. However, @todo: fix this.

                                // action!
                                inbound=&p
                                goto ondo_reenter

                            }
                        default:
                            pf("Result Type -> %T\n", expr.result)
                            report(ifs, "ON cannot operate without a condition.")
                            finish(false, ERR_EVAL)
                            break
                        }

                    }
                }

            } else {
                report(ifs, "ON missing arguments.")
                finish(false, ERR_SYNTAX)
            }


        case C_Assert:

            if tokencount < 2 {

                report(ifs, "Insufficient arguments supplied to ASSERT")
                finish(false, ERR_ASSERT)

            } else {

                cet := crushEvalTokens(inbound.Tokens[1:])
                expr,ef := wrappedEval(ifs, cet, true)

                if ef || expr.evalError {
                    report(ifs, "Could not evaluate expression in ASSERT statement.")
                    finish(false,ERR_EVAL)
                    break
                }

                switch expr.result.(type) {
                case bool:
                    var test_report string

                    group_name_string := ""
                    if test_group != "" {
                        group_name_string += test_group + "/"
                    }
                    if test_name != "" {
                        group_name_string += test_name
                    }

                    if !expr.result.(bool) {
                        if !under_test {
                            report(ifs, sf("Could not assert! ( %s )", expr.text))
                            finish(false, ERR_ASSERT)
                            break
                        }
                        // under test
                        test_report = sf("[#2]TEST FAILED %s (%s/line %d) : %s[#-]", group_name_string, getReportFunctionName(ifs), llcopy, expr.text)
                        testsFailed++
                        appendToTestReport(test_output_file,ifs, llcopy, test_report)
                        temp_test_assert := test_assert
                        if fail_override != "" {
                            temp_test_assert = fail_override
                        }
                        switch temp_test_assert {
                        case "fail":
                            report(ifs, sf("Could not assert! (%s)", expr.text))
                            finish(false, ERR_ASSERT)
                        case "continue":
                            report(ifs, sf("Assert failed (%s), but continuing.", expr.text))
                        }
                    } else {
                        if under_test {
                            test_report = sf("[#4]TEST PASSED %s (%s/line %d) : %s[#-]", group_name_string, getReportFunctionName(ifs), llcopy, expr.text)
                            testsPassed++
                            appendToTestReport(test_output_file,ifs, pc, test_report)
                        }
                    }
                }

            }


        case C_Init: // initialise an array

            if tokencount<2 {
                report(ifs,"Not enough arguments in INIT.")
                finish(false,ERR_EVAL)
                break
            }

            varname := inbound.Tokens[1].tokText
            vartype := "assoc"
            if tokencount>2 {
                vartype = inbound.Tokens[2].tokText
            }

            dimensions:=1
            size:=DEFAULT_INIT_SIZE

            if tokencount>3 {

                cet := crushEvalTokens(inbound.Tokens[3:])

                expr,ef := wrappedEval(ifs, cet, true)
                if ef || expr.evalError {
                    report(ifs,"Could not evaluate expression in INIT statement.")
                    finish(false,ERR_EVAL)
                    break
                }

                switch expr.result.(type) {
                case int,int32,int64,uint8:
                    strSize,invalid:=GetAsInt(expr.result)
                    if ! invalid {
                        size=strSize
                    }
                default:
                    report(ifs,"Array width must evaluate to an integer.")
                    finish(false,ERR_EVAL)
                    break
                }

            }

            if varname != "" {
                switch dimensions {
                case 1:
                    switch vartype {
                    case "byte":
                        vset(ifs, varname, make([]uint8,size,size))
                    case "int":
                        vset(ifs, varname, make([]int,size,size))
                    case "float":
                        vset(ifs, varname, make([]float64,size,size))
                    case "bool":
                        vset(ifs, varname, make([]bool,size,size))
                    case "mixed":
                        vset(ifs, varname, make([]interface{},size,size))
                    case "string":
                        vset(ifs, varname, make([]string,size,size))
                    case "assoc":
                        vset(ifs, varname, make(map[string]interface{},size))
                    }
                default:
                    report(ifs,"Too many dimensions!")
                    finish(false,ERR_SYNTAX)
                }
            }


        case C_Help:
            hargs := ""
            if tokencount == 2 {
                hargs = inbound.Tokens[1].tokText
            }
            help(hargs)


        case C_Nop:
            time.Sleep(100 * time.Millisecond)


        case C_Debug:

            if tokencount != 2 {

                report(ifs, "Malformed DEBUG statement.")
                finish(false, ERR_SYNTAX)

            } else {

                dval, validated := EvalCrush(ifs, inbound.Tokens, 1, tokencount)
                if validated && isNumber(dval) {
                    debug_level = dval.(int)
                } else {
                    report(ifs, "Bad debug level value - could not evaluate.")
                    finish(false, ERR_EVAL)
                }

            }


        case C_Require:

            // require feat support in stdlib first. requires version-as-feat support and markup.

            if tokencount < 2 || tokencount > 3 {
                report(ifs, "Malformed REQUIRE statement.")
                finish(true, ERR_SYNTAX)
                break
            }

            var reqfeat string
            var reqvers int

            switch tokencount {
            case 2: // only by name
                reqfeat = inbound.Tokens[1].tokText
            case 3: // name + version
                reqfeat = inbound.Tokens[1].tokText
                reqvers, _ = strconv.Atoi(inbound.Tokens[2].tokText)
            }

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


        case C_Version:
            version()


        case C_Exit:
            if tokencount > 1 {
                ec, validated := EvalCrush(ifs, inbound.Tokens, 1, tokencount)
                if validated && isNumber(ec) {
                    finish(true, ec.(int))
                } else {
                    report(ifs,"Could not evaluate your EXIT expression")
                    finish(true,ERR_EVAL)
                }
            } else {
                finish(true, 0)
            }


        case C_Define:

            if tokencount > 1 {

                if defining {
                    report(ifs, "Already defining a function. Nesting not permitted.")
                    finish(true, ERR_SYNTAX)
                    break
                }

                fn := inbound.Tokens[1].tokText
                var dargs []string

                if tokencount == 3 {
                    // params supplied:
                    argString := stripOuter(inbound.Tokens[2].tokText, '(')
                    argString = stripOuter(argString, ')')
                    if len(argString)>0 {
                        dargs = str.Split(argString, ",")
                        for arg:=range dargs {
                            dargs[arg]=str.Trim(dargs[arg]," \t")
                        }
                    }
                } else {
                    if tokencount != 2 {
                        report(ifs, "Braced list of parameters not supplied!")
                        finish(true, ERR_SYNTAX)
                        break
                    }
                }

                defining = true
                definitionName = fn

                // error if it has already been defined

                if _, exists := fnlookup.lmget(definitionName); exists {
                    report(ifs, "Function "+definitionName+" already exists.")
                    finish(false, ERR_SYNTAX)
                    break
                }

                loc := GetNextFnSpace()
                // pf("allocated out %v in DEFINE %v\n",loc,definitionName)
                numlookup.lmset(loc,definitionName)
                fnlookup.lmset(definitionName,loc)

                fspacelock.Lock()
                functionspaces[loc] = []Phrase{}
                fspacelock.Unlock()

                farglock.Lock()
                functionArgs[loc] = dargs
                farglock.Unlock()
            }

        case C_Showdef:

            if tokencount == 2 {
                fn := stripOuterQuotes(inbound.Tokens[1].tokText, 2)
                if _, exists := fnlookup.lmget(fn); exists {
                    ShowDef(fn)
                } else {
                    report(ifs, "Function not found.")
                    finish(false, ERR_EVAL)
                }
            } else {
                for oq := range fnlookup.smap {
                    if fnlookup.smap[oq] < 2 {
                        continue
                    } // don't show global or main
                    ShowDef(oq)
                }
                pf("\n")
            }

        case C_Return:

            // tokens must not be braced
            if tokencount == 2 {
                if hasOuterBraces(inbound.Tokens[1].tokText) {
                    if inbound.Tokens[1].tokType == Expression {
                        report(ifs, "Cannot brace a RETURN value.")
                        finish(true, ERR_SYNTAX)
                        break
                    }
                }
            }

            if tokencount != 1 {

                cet := crushEvalTokens(inbound.Tokens[1:])
                if str.Trim(cet.text, " \t") != "" { // found something
                    expr,ef := wrappedEval(ifs, cet, true) // evaluate it
                    if !ef && !expr.evalError { // no error?
                        retvals = append(retvals, expr.result)
                        if ifs<=2 {
                            if exitCode,not_ok:=GetAsInt(expr.result); not_ok {
                                report(ifs, sf("could not evaluate RETURN parameter: %+v", cet.text))
                                finish(true, ERR_EVAL)
                                break
                            } else {
                                finish(true,exitCode)
                                break
                            }
                        }
                    } else {
                        report(ifs, sf("could not evaluate RETURN parameter: %+v", cet.text))
                        finish(true, ERR_EVAL)
                        break
                    }
                }

            }
            endFunc = true


        case C_Enddef:

            if !defining {
                report(ifs, "Not currently defining a function.")
                finish(false, ERR_SYNTAX)
                break
            }

            defining = false
            definitionName = ""


        case C_Input:

            // INPUT <id> <type> <position>                    - set variable {id} from external value or exits.

            // get C_Input arguments

            if tokencount != 4 {
                usage := "INPUT [#i1]id[#i0] PARAM | OPTARG | ENV [#i1]field_position[#i0]"
                report(ifs, "Incorrect arguments supplied to INPUT.\n"+usage)
                finish(false, ERR_SYNTAX)
                break
            }

            id := inbound.Tokens[1].tokText
            typ := inbound.Tokens[2].tokText
            pos := inbound.Tokens[3].tokText

            // eval

            switch str.ToLower(typ) {
            case "param":
                d, er := strconv.Atoi(pos)
                if er == nil {
                    if d<1 {
                        report(ifs,sf("INPUT position %d too low.",d))
                        finish(true, ERR_SYNTAX)
                        break
                    }
                    if d <= len(cmdargs) {
                        // if this is numeric, assign as an int
                        n, er := strconv.Atoi(cmdargs[d-1])
                        if er == nil {
                            vset(ifs, id, n)
                        } else {
                            vset(ifs, id, cmdargs[d-1])
                        }
                    } else {
                        report(ifs,sf("Expected CLI parameter '%s' not provided at startup.", id))
                        finish(true, ERR_SYNTAX)
                    }
                } else {
                    report(ifs, sf("That '%s' doesn't look like a number.", pos))
                    finish(true, ERR_SYNTAX)
                }

            case "optarg":
                d, er := strconv.Atoi(pos)
                if er == nil {
                    if d <= len(cmdargs) {
                        // if this is numeric, assign as an int
                        n, er := strconv.Atoi(cmdargs[d-1])
                        if er == nil {
                            vset(ifs, id, n)
                        } else {
                            vset(ifs, id, cmdargs[d-1])
                        }
                    } else {
                        // nothing provided but var didn't exist, so create it empty
                        // otherwise, just continue
                        if _, found := VarLookup(ifs,id); !found {
                            vset(ifs,id,"")
                        }
                    }
                } else {
                    report(ifs, sf("That '%s' doesn't look like a number.", pos))
                    finish(false, ERR_SYNTAX)
                }

            case "env":
                vset(ifs, id, os.Getenv(pos))
            }


        case C_Module:
            // MODULE <modname>                                - reads in state from a module file.

            var expr ExpressionCarton
            var ef bool

            if tokencount > 1 {
                cet := crushEvalTokens(inbound.Tokens[1:])
                expr,ef = wrappedEval(ifs, cet, true)
                if ef || expr.evalError {
                    report(ifs,"Could not evaluate expression in MODULE statement.")
                    finish(false,ERR_MODULE)
                    break
                }
            } else {
                report(ifs, "No module name provided.")
                finish(false, ERR_MODULE)
                break
            }

            fom := expr.result.(string)

            if fom == "" {
                report(ifs, "Empty module name provided.")
                finish(false, ERR_MODULE)
                break
            }

            //.. set file location

            var moduleloc string = ""

            if str.IndexByte(fom, '/') > -1 {
                if filepath.IsAbs(fom) {
                    moduleloc = fom
                } else {
                    mdir, _ := vget(0,"@execpath")
                    moduleloc = mdir.(string)+"/"+fom
                }
            } else {

                // modules default path is $HOME/.za/modules
                //  unless otherwise redefined in environmental variable ZA_MODPATH

                modhome, _ := vget(0, "@home")
                modhome = modhome.(string) + "/.za"
                if os.Getenv("ZA_MODPATH") != "" {
                    modhome = os.Getenv("ZA_MODPATH")
                }

                moduleloc = modhome.(string) + "/modules/" + fom + ".fom"

            }

            //.. validate module exists

            f, err := os.Stat(moduleloc)

            if err != nil {
                report(ifs, sf("Module is not accessible. (path:%v)",moduleloc))
                finish(false, ERR_MODULE)
                break
            }

            if !f.Mode().IsRegular() {
                report(ifs, "Module is not a regular file.")
                finish(false, ERR_MODULE)
                break
            }

            //.. read in file

            mod, err := ioutil.ReadFile(moduleloc)
            if err != nil {
                report(ifs, "Problem reading the module file.")
                finish(false, ERR_MODULE)
                break
            }

            // tokenise and parse into a new function space.

            //.. error if it has already been defined
            if _, exists := fnlookup.lmget("@mod_"+fom); exists {
                report(ifs, "Function @mod_"+fom+" already exists.")
                finish(false, ERR_SYNTAX)
                break
            }

            loc := GetNextFnSpace()
            // pf("allocated out %v in MODULE %v\n",loc,fom)
            numlookup.lmset(loc,"@mod_" + fom)
            fnlookup.lmset("@mod_"+fom, loc)

            fspacelock.Lock()
            functionspaces[loc] = []Phrase{}
            fspacelock.Unlock()

            farglock.Lock()
            functionArgs[loc] = []string{}
            farglock.Unlock()

            //.. parse and execute
            parse("@mod_"+fom, string(mod), 0)

            modcs := call_s{}
            modcs.base = loc
            modcs.caller = ifs
            modcs.fs = "@mod_" + fom
            calllock.Lock()
            callstack[loc] = modcs
            calllock.Unlock()
            Call(ifs, base, MODE_NEW, loc)

            // purge the module source as the code has been executed

            fspacelock.Lock()
            functionspaces[loc]=[]Phrase{}
            fspacelock.Unlock()

            calllock.Lock()
            callstack[loc]=call_s{}
            calllock.Unlock()


        case C_When:

            // need to store the condition and result for the is/contains/in/or clauses
            // endwhen location should be calculated in advance for a direct jump to exit
            // we need to calculate it anyway for nesting
            // after the above setup, we execute next source line as normal

            if tokencount==1 {
                report(ifs,"Missing expression in WHEN statement")
                finish(false,ERR_SYNTAX)
                break
            }

            // lookahead
            endfound, enddistance, er := lookahead(base, pc, 0, 0, C_Endwhen, []int{C_When}, []int{C_Endwhen})

            // debug(6,"@%d : Endwhen lookahead set to line %d\n",pc+1,pc+1+enddistance)

            if er {
                report(ifs, "Lookahead error!")
                finish(true, ERR_SYNTAX)
                break
            }

            if !endfound {
                report(ifs, "Missing ENDWHEN for this WHEN")
                finish(false, ERR_SYNTAX)
                break
            }

            cet := crushEvalTokens(inbound.Tokens[1:])
            expr,ef := wrappedEval(ifs, cet, true)

            if ef || expr.evalError {
                report(ifs, "Could not evaluate the WHEN condition")
                finish(false, ERR_EVAL)
                break
            }

            // create a whenCarton and increase the nesting level
            if lockSafety { looplock.Lock() }
            wccount[ifs]++
            wc[wccount[ifs]] = whenCarton{endLine: pc + enddistance, value: expr.result, dodefault: true, broken: false}
            depth[ifs]++
            if lockSafety { looplock.Unlock() }

            if lockSafety { lastlock.Lock() }
            lastConstruct[ifs] = append(lastConstruct[ifs], C_When)
            if lockSafety { lastlock.Unlock() }

        case C_Is, C_Contains, C_Or:

            var lcifs int

            if lockSafety { looplock.RLock() }

            d:=depth[ifs]

            if lockSafety { lastlock.RLock() }
            if d != 0 {
                lcifs=lastConstruct[ifs][d-1]
            }
            if lockSafety { lastlock.RUnlock() }

            if d == 0 || (d > 0 && lcifs != C_When) {
                if lockSafety { looplock.RUnlock() }
                report(ifs,"Not currently in a WHEN block.")
                break
            }

            carton := wc[wccount[ifs]]

            if lockSafety { looplock.RUnlock() }

            var cet, expr ExpressionCarton
            var ef bool

            if tokencount > 1 { // tokencount==1 for C_Or
                cet = crushEvalTokens(inbound.Tokens[1:])
                expr,ef = wrappedEval(ifs, cet, true)
                if ef || expr.evalError {
                    report(ifs, "Could not evaluate expression in WHEN condition.")
                    finish(false,ERR_EVAL)
                    break
                }
            }

            ramble_on := false // assume we'll need to skip to next when clause

            switch statement.tokType {

            case C_Is:
                if expr.result == carton.value { // matched IS value
                    carton.dodefault = false
                    if lockSafety { looplock.Lock() }
                    wc[wccount[ifs]] = carton
                    if lockSafety { looplock.Unlock() }
                    ramble_on = true
                }

            case C_Contains:
                reg := sparkle(expr.result.(string))
                switch carton.value.(type) {
                case string:
                    if matched, _ := regexp.MatchString(reg, carton.value.(string)); matched { // matched CONTAINS regex
                        carton.dodefault = false
                        if lockSafety { looplock.Lock() }
                        wc[wccount[ifs]] = carton
                        if lockSafety { looplock.Unlock() }
                        ramble_on = true
                    }
                case int:
                    if matched, _ := regexp.MatchString(reg, strconv.Itoa(carton.value.(int))); matched { // matched CONTAINS regex
                        carton.dodefault = false
                        if lockSafety { looplock.Lock() }
                        wc[wccount[ifs]] = carton
                        if lockSafety { looplock.Unlock() }
                        ramble_on = true
                    }
                }

            case C_Or: // default

                if carton.dodefault == false {
                    pc = carton.endLine - 1
                    ramble_on = false
                } else {
                    ramble_on = true
                }

            }

            var loc int

            // jump to the next clause, continue to next line or skip to end.

            if ramble_on { // move on to next pc statement
            } else {
                // skip to next WHEN clause:
                isfound, isdistance, _ := lookahead(base, pc+1, 0, 0, C_Is, []int{C_When}, []int{C_Endwhen})
                orfound, ordistance, _ := lookahead(base, pc+1, 0, 0, C_Or, []int{C_When}, []int{C_Endwhen})
                cofound, codistance, _ := lookahead(base, pc+1, 0, 0, C_Contains, []int{C_When}, []int{C_Endwhen})

                // add jump distances to list
                distList := []int{}
                if isfound {
                    distList = append(distList, isdistance)
                }
                if orfound {
                    distList = append(distList, ordistance)
                }
                if cofound {
                    distList = append(distList, codistance)
                }

                if !(isfound || orfound || cofound) {
                    // must be an endwhen
                    loc = carton.endLine
                } else {
                    loc = pc + min_int(distList) + 1
                }

                // jump to nearest following clause
                pc = loc - 1
            }


        case C_Endwhen:

            var lcifs int

            if lockSafety { looplock.RLock() }
            d:=depth[ifs]
            if lockSafety { looplock.RUnlock() }

            if lockSafety { lastlock.RLock() }
            if d != 0 {
                lcifs=lastConstruct[ifs][d-1]
            }
            if lockSafety { lastlock.RUnlock() }

            if d == 0 || (d > 0 && lcifs != C_When) {
                report(ifs,"Not currently in a WHEN block.")
                break
            }

            breakIn = Error

            if lockSafety { lastlock.Lock() }
            lastConstruct[ifs] = lastConstruct[ifs][:depth[ifs]-1]
            if lockSafety { lastlock.Unlock() }

            if lockSafety { looplock.Lock() }
            depth[ifs]--
            wccount[ifs]--
            if wccount[ifs] < 0 {
                report(ifs, "Cannot reduce WHEN stack below zero.")
                finish(false, ERR_SYNTAX)
            }
            if lockSafety { looplock.Unlock() }


        case C_Print:
            var badval bool
            if tokencount > 1 {
                previousterm := 1
                for term := range inbound.Tokens[1:] {
                    if inbound.Tokens[term].tokType == C_Comma {
                        expr, ef := wrappedEval(ifs, crushEvalTokens(inbound.Tokens[previousterm:term]), true)
                        if ef || expr.evalError {
                            pf(`<badval>`)
                            badval=true
                            break
                        }
                        pf(`%v`, sparkle(sf(`%v`, expr.result)))
                        previousterm = term + 1
                    }
                }

                if badval { break }

                expr, ef := wrappedEval(ifs, crushEvalTokens(inbound.Tokens[previousterm:]), true)
                if ef || expr.evalError {
                    pf(`<badval>`)
                    break
                }
                pf(`%v`, sparkle(sf(`%v`, expr.result)))
                if interactive { pf("\n") }
            } else {
                pf("\n")
            }


        case C_Println:
            var badval bool
            if tokencount > 1 {
                previousterm := 1
                for term := range inbound.Tokens[1:] {
                    if inbound.Tokens[term].tokType == C_Comma {
                        expr, ef := wrappedEval(ifs, crushEvalTokens(inbound.Tokens[previousterm:term]), true)
                        if ef || expr.evalError {
                            pf(`<badval>`)
                            badval=true
                            break
                        }
                        pf(`%v`, sparkle(sf(`%v`, expr.result)))
                        previousterm = term + 1
                    }
                }

                if badval { break }

                expr, ef := wrappedEval(ifs, crushEvalTokens(inbound.Tokens[previousterm:]), true)
                if ef || expr.evalError {
                    pf(`<badval>`)
                    break
                }
                pf(`%v`, sparkle(sf(`%v`, expr.result)))
                pf("\n")
            } else {
                pf("\n")
            }


        case C_Log:

            plog_out := ""

            var badval bool
            if tokencount > 1 {
                previousterm := 1
                for term := range inbound.Tokens[1:] {
                    if inbound.Tokens[term].tokType == C_Comma {
                        expr,ef := wrappedEval(ifs, crushEvalTokens(inbound.Tokens[previousterm:term]), true)
                        if ef || expr.evalError { badval=true; break }
                        plog_out += sparkle(sf(`%v`, expr.result))
                        previousterm = term + 1
                    }
                }
                if badval { break }
                expr,ef := wrappedEval(ifs, crushEvalTokens(inbound.Tokens[previousterm:]), true)
                if ef || expr.evalError { break }
                plog_out += sparkle(sf(`%v`, expr.result))
            }

            plog("%v", plog_out)

        case C_Hist:

            for h, v := range hist {
                pf("%5d : %s\n", h, v)
            }

        case C_At:

            // AT row ',' column

            commaAt := findDelim(inbound.Tokens, ",", 1)

            if commaAt == -1 || commaAt == tokencount {
                report(ifs, "Bad delimiter in AT.")
                finish(false, ERR_SYNTAX)
            } else {

                evrow := crushEvalTokens(inbound.Tokens[1:commaAt])
                evcol := crushEvalTokens(inbound.Tokens[commaAt+1:])

                expr_row, ef, err := ev(ifs, evrow.text, false)
                if ef || expr_row==nil || err != nil {
                    report(ifs, sf("Evaluation error in %v", expr_row))
                }

                expr_col, ef, err := ev(ifs, evcol.text, false)
                if ef || expr_col==nil || err != nil {
                    report(ifs, sf("Evaluation error in %v", expr_col))
                }

                row, _ = GetAsInt(expr_row)
                col, _ = GetAsInt(expr_col)
                at(row, col)

            }


        case C_Prompt:

            // if on windows, return an error
            if runtime.GOOS == "windows" {
                pf("PROMPT not supported on windows.\n")
                finish(false,ERR_SYNTAX)
                break
            }

            // else continue

            if tokencount < 2 {
                usage := "PROMPT [#i1]storage_variable prompt_string[#i0] [ [#i1]validator_regex[#i0] ]"
                report(ifs, "Not enough arguments for PROMPT.\n"+usage)
                finish(false, ERR_SYNTAX)
                break
            }

            // prompt variable assignment:
            if tokencount > 1 { // um, should not do this but...
                if inbound.Tokens[1].tokType == C_Assign {
                    cet := crushEvalTokens(inbound.Tokens[2:])
                    expr,ef := wrappedEval(ifs, cet, true)
                    if ef || expr.evalError { break }
                    switch expr.result.(type) {
                    case string:
                        promptTemplate = sparkle(expr.result.(string))
                    }
                } else {
                    // prompt command:
                    if tokencount < 3 || tokencount > 4 {
                        report(ifs, "Incorrect arguments for PROMPT command.\n")
                        finish(false, ERR_SYNTAX)
                        break
                    } else {
                        validator := ""
                        broken := false
                        expr, ef, prompt_ev_err := ev(ifs, inbound.Tokens[2].tokText, true)
                        if ef || expr==nil {
                            report(ifs, "Could not evaluate in PROMPT command.")
                            finish(false,ERR_EVAL)
                            break
                        }
                        if prompt_ev_err == nil {
                            // @todo: allow an expression instead of the string literal for validator
                            processedPrompt := expr.(string)
                            if tokencount == 4 {
                                validator = stripOuter(inbound.Tokens[3].tokText, '"')
                                intext := ""
                                validated := false
                                for !validated || broken {
                                    intext, _, broken = getInput(processedPrompt, currentpane, row, col, promptColour, false, false)
                                    validated, _ = regexp.MatchString(validator, intext)
                                }
                                if !broken {
                                    vset(ifs, inbound.Tokens[1].tokText, intext)
                                }
                            } else {
                                var inp string
                                inp, _, broken = getInput(processedPrompt, currentpane, row, col, promptColour, false, false)
                                vset(ifs, inbound.Tokens[1].tokText, inp)
                            }
                            if broken {
                                finish(false, 0)
                            }
                        }
                    }
                }
            }

        case C_Logging:

            if tokencount < 2 || tokencount > 3 {
                report(ifs, "LOGGING command malformed.")
                finish(false, ERR_SYNTAX)
                break
            }

            switch str.ToLower(inbound.Tokens[1].tokText) {

            case "off":
                loggingEnabled = false

            case "on":
                loggingEnabled = true
                if tokencount == 3 {
                    cet := crushEvalTokens(inbound.Tokens[2:])
                    expr,ef := wrappedEval(ifs, cet, false)
                    if ef || expr.evalError { break }
                    logFile = expr.result.(string)
                    vset(0, "@logsubject", "")
                }

            case "quiet":
                vset(globalspace, "@silentlog", true)

            case "loud":
                vset(globalspace, "@silentlog", false)

            case "accessfile":
                if tokencount > 2 {
                    cet := crushEvalTokens(inbound.Tokens[2:])
                    expr,ef := wrappedEval(ifs, cet, false)
                    if ef || expr.evalError { break }
                    web_log_file=expr.result.(string)
                    // pf("accessfile changed to %v\n",web_log_file)
                    web_log_handle.Close()
                    var err error
                    web_log_handle, err = os.OpenFile(web_log_file, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
                    if err != nil {
                        log.Println(err)
                    }
                    web_logger = log.New(web_log_handle, "", log.LstdFlags) // no prepended text
                } else {
                    report(ifs,"No access file provided for LOGGING ACCESSFILE command.")
                    finish(false, ERR_SYNTAX)
                }

            case "web":
                if tokencount > 2 {
                    switch str.ToLower(inbound.Tokens[2].tokText) {
                    case "on","1","enable":
                        log_web=true
                    case "off","0","disable":
                        log_web=false
                    default:
                        report(ifs,"Invalid state set for LOGGING WEB.")
                        finish(false, ERR_EVAL)
                    }
                } else {
                    report(ifs,"No state provided for LOGGING WEB command.")
                    finish(false, ERR_SYNTAX)
                }

            case "subject":
                if tokencount == 3 {
                    cet := crushEvalTokens(inbound.Tokens[2:])
                    expr,ef := wrappedEval(ifs, cet, false)
                    if ef || expr.evalError { break }
                    vset(0, "@logsubject", expr.result.(string))
                } else {
                    vset(0, "@logsubject", "")
                }

            default:
                report(ifs, "LOGGING command malformed.")
                finish(false, ERR_SYNTAX)
            }


        case C_Cls:

            if tokencount == 1 {
                cls()
                row = 1
                col = 1
                currentpane = "global"
            } else {
                if currentpane != "global" {
                    p := panes[currentpane]
                    for l := 1; l < p.h; l++ {
                        clearToEOPane(l, 2)
                    }
                    row = 1
                    col = 1
                }
            }


        case C_Zero:

            // similar issues to INC+DEC with array elements. needs fixing when they get done right.

            if tokencount == 2 {
                if inbound.Tokens[1].tokType == Identifier {
                    vset(ifs, inbound.Tokens[1].tokText, 0)
                } else {
                    report(ifs, "Not an identifier.")
                    finish(false, ERR_SYNTAX)
                }
            } else {
                report(ifs, "Missing identifier to reset.")
                finish(false, ERR_SYNTAX)
            }


        case C_Inc,C_Dec:

            var id string

            if tokencount > 1 {

                if inbound.Tokens[1].tokType == Identifier {
                    id = inbound.Tokens[1].tokText
                } else {
                    report(ifs, "Not an identifier.")
                    finish(false, ERR_SYNTAX)
                    break
                }

                var ampl int
                var er bool
                var endIncDec bool
                var isArray bool

                switch tokencount {
                case 2:
                    ampl = 1
                default:
                    // is a var?
                    v,ok:=vget(ifs,inbound.Tokens[2].tokText)
                    if ok {
                        switch v.(type) {
                        case uint8:
                            ampl,_ = GetAsInt(v.(uint8))
                        case int32:
                            ampl,_ = GetAsInt(v.(int32))
                        case int64:
                            ampl,_ = GetAsInt(v.(int64))
                        case int:
                            ampl = v.(int)
                        default:
                            report(ifs,sf("%s only works with integer types. (not this: %T)",str.ToUpper(inbound.Tokens[0].tokText),v))
                            finish(false,ERR_EVAL)
                            endIncDec=true
                            break
                        }
                    } else { // is an int?
                        ampl,er = GetAsInt(inbound.Tokens[2].tokText)
                        if er { // else evaluate
                            cet := crushEvalTokens(inbound.Tokens[2:])
                            expr,ef := wrappedEval(ifs, cet, false)
                            if ef || expr.evalError {
                                endIncDec=true
                                break
                            }
                            switch expr.result.(type) {
                            case int:
                                ampl = expr.result.(int)
                            default:
                                report(ifs,sf("%s does not result in an integer type.",str.ToUpper(inbound.Tokens[0].tokText)))
                                finish(false,ERR_EVAL)
                            }
                        }
                    }
                }
                if !endIncDec {

                    // look away in disgust, another bodge:
                    //   check for square brace in id. if present, use vgetElement instead.
                    //   we also remove quotes here, so only accepts literals for elements.
                    //   obviously, this is not great. need to fix this filth soon.

                    var val interface{}
                    var sqPos int
                    var elementComponents string
                    var sqEndPos int
                    var found bool

                    if sqPos=str.IndexByte(id,'['); sqPos!=-1 {
                        sqEndPos=str.IndexByte(id,']')
                        elementComponents=stripOuter(id[sqPos+1:sqEndPos],'"')
                        // pf("parts: %v [ %v ]\n",id[:sqPos],elementComponents)
                        val, found = vgetElement(ifs,id[:sqPos],elementComponents)
                        isArray = true
                    } else {
                        val, found = vget(ifs, id)
                    }

                    if !found { val=0 }

                    // pf("id      : %v\n",id)
                    // pf("preval  : %v\n",val)
                    // pf("pretype : %T\n",val)

                    if found {
                        switch val.(type) {
                        case int:
                            val,_=GetAsInt(val)
                        case int32,int64,uint8:
                            val,_=GetAsInt(val)
                        default:
                            report(ifs,sf("%s only works with integer types. (*not this: %T with id:%v)",str.ToUpper(inbound.Tokens[0].tokText),val,id))
                            finish(false,ERR_EVAL)
                            endIncDec=true
                        }
                    }

                    // pf("id       : %v\n",id)
                    // pf("postval  : %v\n",val)
                    // pf("posttype : %T\n",val)
                    // pf("ampl     : %v\n",ampl)
                    // pf("eid      : %v\n",endIncDec)
                    // pf("isarray  : %v\n",isArray)

                    // if not found then will init with 0+ampl
                    if !endIncDec {
                        switch statement.tokType {
                        case C_Inc:
                            if isArray {
                                vsetElement(ifs,id[:sqPos],elementComponents,val.(int)+ampl)
                            } else {
                                vset(ifs, id, val.(int)+ampl)
                            }
                        case C_Dec:
                            if isArray {
                                vsetElement(ifs,id[:sqPos],elementComponents,val.(int)-ampl)
                            } else {
                                vset(ifs, id, val.(int)-ampl)
                            }
                        }
                    }
                }
            } else {
                typ:="increment"
                if statement.tokType==C_Dec { typ="decrement" }
                report(ifs, "Missing identifier in "+typ+" statement.")
                finish(false, ERR_SYNTAX)
            }


        case C_If:

            // lookahead
            elsefound, elsedistance, er := lookahead(base, pc, 0, 1, C_Else, []int{C_If}, []int{C_Endif})
            endfound, enddistance, er := lookahead(base, pc, 0, 0, C_Endif, []int{C_If}, []int{C_Endif})

            if er || !endfound {
                report(ifs, "Missing ENDIF for this IF")
                finish(false, ERR_SYNTAX)
                break
            }

            // eval
            expr, validated := EvalCrushRest(ifs, inbound.Tokens, 1)
            if !validated {
                report(ifs, "Could not evaluate expression.")
                finish(false, ERR_SYNTAX)
                break
            }

            if isBool(expr.(bool)) && expr.(bool) {
                // was true
                break
            } else {
                if elsefound && (elsedistance < enddistance) {
                    pc += elsedistance
                } else {
                    pc += enddistance
                }
            }


        case C_Else:

            // we already jumped to else+1 to deal with a failed IF test
            // so jump straight to the endif here

            endfound, enddistance, _ := lookahead(base, pc, 1, 0, C_Endif, []int{C_If}, []int{C_Endif})

            if endfound {
                pc += enddistance
            } else { // this shouldn't ever occur, as endif checked during C_If, but...
                report(ifs, "ELSE without an ENDIF\n")
                finish(false, ERR_SYNTAX)
            }


        case C_Endif:

            // ENDIF *should* just be an end-of-block marker


        default:

            // local command assignment (bash call)

            if tokencount > 1 { // ident "=|"
                if statement.tokType == Identifier && inbound.Tokens[1].tokType == C_AssCommand {
                    if len(inbound.Text) > 0 {
                        startPos := str.IndexByte(inbound.Original, '|') + 1
                        cmd := interpolate(ifs, inbound.Original[startPos:])
                        out, _ := Copper(cmd, false)
                        lhs_name := interpolate(ifs, statement.tokText)
                        vset(ifs, lhs_name, out)
                    }
                    break
                }
            }

            // try to eval and assign
            cet := crushEvalTokens(inbound.Tokens)
            // tmpres,ef := wrappedEval(ifs, cet, true)
            tmpres,_ := wrappedEval(ifs, cet, true)
            if tmpres.evalError && tmpres.reason != "" {
                pf("Code %v: [#2]%s[#-]\n", tmpres.evalCode, tmpres.reason)
            }
            // if ef || tmpres.evalError { break }
            if tmpres.evalError { break }

        } // end-case

    }

    siglock.RLock()
    si:=sig_int
    siglock.RUnlock()

    if !si {

        // populate return variables in the caller with retvals
        for qv := range retvals {
            if qv>len(retvars)-1 { break }
            lhs_name := interpolate(ifs, retvars[qv])
            vset(caller, lhs_name, retvals[qv])
        }

        // clean up
        fnlookup.lmdelete(fs+"@"+string(ifs))
        numlookup.lmdelete(ifs)

        if ifs>2 { functionspaces[ifs] = []Phrase{} }

    }

    return endFunc, breakIn, continueOut

}

/// execute a command in the bash coprocess
func bashCall(ifs uint64,s string) {
    cet := ""
    if len(s) > 0 {
        // find index of first pipe, then remove everything upto and including it
        pipepos := str.IndexByte(s, '|')
        if pipepos==-1 {
            pf("syntax error in '%s'\n",s)
            // @todo: handle this type of exit more gracefully, no rush, should be uncommon.
            os.Exit(0)
        }
        cet = s[pipepos+1:]
        out, ec := Copper(interpolate(ifs, cet), false)
        if ec==-1 || ec > 0 {
            pf("Error: [%d] in shell command '%s'\n", ec, str.TrimLeft(cet, " \t"))
        } else {
            if len(out) > 0 {
                if out[len(out)-1] != '\n' {
                    out += "\n"
                }
                pf("%s", out)
            }
        }
    }
}


/// print user-defined function definition(s) to stdout
func ShowDef(fn string) bool {
    var ifn uint64
    var present bool
    if ifn, present = fnlookup.lmget(fn); !present {
        return false
    }

    if ifn < uint64(len(functionspaces)) {
        first := true
        for q := range functionspaces[ifn] {
            strOut := "\t\t "
            if first == true {
                first = false
                strOut = sf("\n%s(%v)\n\t\t ", fn, str.Join(functionArgs[ifn], ","))
            }
            pf("%s%s\n", strOut, functionspaces[ifn][q].Original)
        }
    }
    return true
}

/// search token list for a given delimiter token type
func findTokenDelim(tokens []Token, delim int, start int) (pos int) {
    for p := start; p < len(tokens); p++ {
        if tokens[p].tokType == delim {
            return p
        }
    }
    return -1
}

/// search token list for a given delimiter string
func findDelim(tokens []Token, delim string, start int) (pos int) {
    delim = str.ToLower(delim)
    for p := start; p < len(tokens); p++ {
        if str.ToLower(tokens[p].tokText) == delim {
            return p
        }
    }
    return -1
}


