package main

import (
    "bytes"
    "encoding/gob"
    "io/ioutil"
    "math"
    "math/rand"
    "log"
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

func getRealSizeOf(v interface{}) (int, error) {
    b := new(bytes.Buffer)
    if err := gob.NewEncoder(b).Encode(v); err != nil {
        return 0, err
    }
    return b.Len(), nil
}

func task(caller uint64, loc uint64, iargs ...interface{}) <-chan interface{} {
    r:=make(chan interface{})
    go func() {
        defer close(r)
        Call(MODE_NEW, loc, ciAsyn, iargs...)
        v,_:=vget(caller,sf("@#@%v",loc))
        r<-v
    }()
    return r
}

var debuglock = &sync.RWMutex{}
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


// slightly faster string comparison.
// have to use gotos here as loops can't be inlined
func strcmp(a string, b string) (bool) {
    la:=len(a)
    if la!=len(b)           { return false }
    if la==0 && len(b)==0   { return true }
    strcmp_repeat_point:
        la--
        if a[la]!=b[la] { return false }
    if la>0 { goto strcmp_repeat_point }
    return true
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

// GetAsInt64 : converts a variety of types to int64
func GetAsInt64(expr interface{}) (int64, bool) {
    switch i := expr.(type) {
    case float32:
        return int64(i), false
    case float64:
        return int64(i), false
    case uint:
        return int64(i), false
    case uint8:
        return int64(i), false
    case uint32:
        return int64(i), false
    case uint64:
        return int64(i), false
    case int:
        return int64(i), false
    case int32:
        return int64(i), false
    case int64:
        return i, false
    case string:
        p, e := strconv.ParseFloat(i, 64)
        if e == nil {
            return int64(p), false
        }
    }
    return 0, true
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
    case int64:
        return int32(i), false
    case int32:
        return i, false
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
    case int32:
        return int(i), false
    case int64:
        return int(i), false
    case int:
        return i, false
    case string:
        if i!="" {
            p, e := strconv.ParseFloat(i, 64)
            if e == nil {
                return int(p), false
            }
        }
    default:
        /*
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
        */
    }
    return 0, true
}

func GetAsUint(expr interface{}) (uint64, bool) {
    switch i := expr.(type) {
    case float32:
        return uint64(i), false
    case float64:
        return uint64(i), false
    case int:
        return uint64(i), false
    case uint8:
        return uint64(i), false
    case uint32:
        return uint64(i), false
    case uint64:
        return i, false
    case int32:
        return uint64(i), false
    case int64:
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

// EvalCrush* used in C_If, C_Exit, C_For and C_Debug:

// EvalCrush() : take all tokens from tok[] between tstart and tend inclusive, compact and return evaluated answer.
// if no evalError then returns a "validated" true bool
func EvalCrush(p *leparser, fs uint64, tok []Token, tstart int, tend int) (interface{}, error) {
    /*
    for k,v:=range tok {
        pf("(%d) %+v\n",k,v)
    }
    */
    return p.Eval(fs,tok[tstart:tend+1])
}

// as evalCrush but operate over all remaining tokens from tstart onwards
func EvalCrushRest(p *leparser, fs uint64, tok []Token, tstart int) (interface{}, error) {
    return p.Eval(fs,tok[tstart:])
}

// check for value in slice
func InSlice(a uint8, list []uint8) bool {
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
            // c,heck for direct reference
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

func lookahead(fs uint64, startLine int, startlevel int, endlevel int, term uint8, indenters []uint8, dedenters []uint8) (bool, int, bool) {

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
func GetNextFnSpace(requiredName string) (uint64,string) {

    calllock.Lock()
    defer calllock.Unlock()

    // find highest in list

    top:=uint64(cap(calltable))
    highest:=top
    ccap:=uint64(CALL_CAP)
    deallow:=top>uint64(ccap*2)

    for q:=top-1; q>(ccap*2) && q>(top/2)-ccap; q-- {
        if calltable[q]!=(call_s{}) { highest=q; break }
    }

    // dealloc

    if deallow {
        if highest<((top/2)-(ccap/2)-1) {
            ncs:=make([]call_s,len(calltable)/2,cap(calltable)/2)
            copy(ncs,calltable)
            calltable=ncs
            top=uint64(cap(calltable))
        }
    }

    // we know at this point that if a dealloc occurred then highest was
    // already below new cap and a fresh alloc should not occur below

    for q := uint64(1); q < top+1 ; q++ {

        if _, found := numlookup.lmget(q); found {
            continue
        }

        for ; q>=uint64(cap(calltable)) ; {
            ncs:=make([]call_s,len(calltable)*2,cap(calltable)*2)
            copy(ncs,calltable)
            calltable=ncs
        }

        var suf string

        // pf("-- entered reserving code--\n")
        for  ; ; {

            newName := requiredName

            if newName[len(newName)-1]=='@' {
                suf=sf("%d",rand.Int())
                newName+=suf
            }

            if _, found := numlookup.lmget(q); !found { // unreserved
                numlookup.lmset(q, newName)
                fnlookup.lmset(newName,q)
                // place a reservation in calltable:
                // if we don't do this, then there is a small chance that the id [q]
                //  will get re-used between the calls to GetNextFnSpace() and Call()
                //  by fast spawning async tasks.
                calltable[q]=call_s{fs:"@@reserved",caller:0,base:0,retvar:""}
                // pf("-- leaving gnfs with %v,%v --\n\n",q,suf)
                // calllock.Unlock()
                return q,newName
            }

        }
    }

    pf("Error: no more function space available.\n")
    finish(true, ERR_FATAL)
    return 0, ""
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
var globlock   = &sync.RWMutex{}


// test var for error reporting
//   will probably blow up during recursion.

var callChain []chainInfo


// defined function entry point
// everything about what is to be executed is contained in calltable[csloc]
func Call(varmode int, csloc uint64, registrant uint8, va ...interface{}) (endFunc bool) {

    // if lockSafety { calllock.RLock() }
    // pf("Entered call -> %#v : va -> %+v\n",calltable[csloc],va)
    // pf(" with new ifs of -> %v fs-> %v\n",csloc,calltable[csloc].fs)
    // if lockSafety { calllock.RUnlock() }

    // register call
    caller_str,_:=numlookup.lmget(calltable[csloc].caller)
    callChain=append(callChain,chainInfo{loc:calltable[csloc].caller,name:caller_str,line:calltable[csloc].callline,registrant:registrant})

    var inbound *Phrase
    var current_with_handle *os.File

    defer func() {
        if r := recover(); r != nil {

            if _, ok := r.(runtime.Error); ok {
                pf("Fatal error on: %v\n",inbound.Original)
                pf(sparkle("[#2]Details:\n%v[#-]\n"),r)
                if debug_level==0 {
                    os.Exit(127)
                }
            }
            setEcho(true)
            err := r.(error)
            pf("error : %v\n",err)
            panic(r)
        }
    }()

    // set up evaluation parser
    parser:=&leparser{}
    parser.Init()

    var breakIn uint8
    var pc int
    var retvar string
    var retval interface{}
    var finalline int
    var fs string
    var caller uint64
    var base uint64

    // set up the function space

    // ..get call details
    calllock.RLock()
    ncs := &calltable[csloc]
    fs = (*ncs).fs                          // unique name for this execution, pre-generated before call
    base = (*ncs).base                      // the source code to be read for this function
    caller = (*ncs).caller                  // which func id called this code
    retvar = (*ncs).retvar                  // usually @#, the return variable name
    ifs,_:=fnlookup.lmget(fs)               // the uint64 id attached to fs name
    calllock.RUnlock()

    if base==0 {
        if !interactive {
            parser.report("Possible race condition. Please check. Base->0")
            finish(false,ERR_EVAL)
            return
        }
    }

    if lockSafety { farglock.RLock() }

    // pf("va->%#v\n",va...)
    // pf("fa->%#v\n",functionArgs[base])
    /*
    if len(va) > len(functionArgs[base]) {
        parser.report("Syntax error: too many call arguments provided.")
        finish(false,ERR_SYNTAX)
        return
    }
    */

    // missing varargs in call result in empty string assignments:
    if functionArgs[base]!=nil {
        if len(functionArgs[base])>len(va) {
            for e:=0; e<(len(functionArgs[base])-len(va)); e++ {
                va=append(va,"")
            }
        }
    }
    if lockSafety { farglock.RUnlock() }


    tco:=false

    //
    // re-entry point for recursive tail calls
    //

tco_reentry:


    if varmode == MODE_NEW {

        // create the local variable storage for the function
        var vtm uint64
        if ! tco {

            vlock.RLock()
            vtm=vtable_maxreached
            vlock.RUnlock()

            if ifs>=vtm { vcreatetable(ifs, &vtable_maxreached, VAR_CAP) }

            globlock.Lock()
            test_group = ""
            test_name = ""
            test_assert = ""
            globlock.Unlock()

        }

        // in interactive mode, the current functionspace is 0
        // in normal exec mode, the source is treated as functionspace 1
        if base < 2 {
            globalaccess = ifs
            vset(globalaccess, "trapInt", "")
        }

        // nesting levels in this function
        looplock.Lock()
        depth[ifs] = 0
        looplock.Unlock()

        vlock.Lock()
        varcount[ifs] = 0
        vlock.Unlock()

        lastlock.Lock()
        lastConstruct[ifs] = []uint8{}
        lastlock.Unlock()

        vset(ifs,"@in_tco",false)

    }

    // initialise condition states: WHEN stack depth
    // initialise the loop positions: FOR, FOREACH, WHILE

    if lockSafety { looplock.Lock() }
    wccount[ifs] = 0

    // allocate loop storage space if not a repeat ifs value.
    if lockSafety { vlock.RLock() }

    var top,highest,lscap uint64

    top=uint64(cap(loops))
    highest=top
    lscap=LOOP_START_CAP
    deallow:=top>uint64(lscap*2)

    for q:=top-1; q>(lscap*2) && q>(top/2)-lscap; q-- {
        if loops[q]!=nil { highest=q; break }
    }

    // dealloc
    if deallow {
        if highest<((top/2)-(lscap/2)-1) {
            nloops:=make([][]s_loop,len(loops)/2,cap(loops)/2)
            copy(nloops,loops)
            loops=nloops
            // pf("[#1]--[#-] loops-pre-dec  highest %d,len %d, cap %d\n",highest,lscap,top)
            top=uint64(cap(loops))
            // pf("[#1]--[#-] loops-post-dec highest %d,len %d, cap %d\n",highest,lscap,top)
        }
    }

    for ; ifs>=uint64(cap(loops)) ; {
            // increase
            // pf("[#2]++[#-] loops-pre-inc highest %d,len %d, cap %d\n",highest,lscap,top)
            nloops:=make([][]s_loop,len(loops)*2,cap(loops)*2)
            copy(nloops,loops)
            loops=nloops
    }

    loops[ifs] = make([]s_loop, MAX_LOOPS)

    if lockSafety { vlock.RUnlock() }
    if lockSafety { looplock.Unlock() }

   /* 
    pf("in %v \n",fs)
    pf("base  -> %v\n",base)
    pf("va    -> %#v\n",va)
    pf("fargs -> %#v\n",functionArgs[base])
   */

    // assign value to local vars named in functionArgs (the call parameters) from each 
    // va value (functionArgs[] created at definition time from the call signature).

    if lockSafety { farglock.RLock() }
    if len(va) > 0 {
        for q, v := range va {
            fa:=functionArgs[base][q]
            vset(ifs,fa,v)
        }
    }
    if lockSafety { farglock.RUnlock() }


    finalline = len(functionspaces[base])

    inside_test := false            // are we currently inside a test bock
    inside_with := false            // WITH cannot be nested and remains local in scope.

    var structMode bool             // are we currently defining a struct
    var structName string           // name of struct currently being defined
    var structNode []string         // struct builder
    var defining bool               // are we currently defining a function. takes priority over structmode.
    var definitionName string       // ... if we are, what is it called
    var ampl int                    // sets amplitude of change in INC/DEC statements
    var vid int                     // VarLookup ID cache for FOR loops when !lockSafety

    pc = -1                         // program counter : increments to zero at start of loop

    var si bool
    /*
    grso,_:=getRealSizeOf(functionspaces)
    pf(">> fs[] sz : %d len %d\n",grso,len(functionspaces))
    */

    var lastline int
    var statement Token

    for {

        pc++  // program counter, equates to each Phrase struct in the function
        parser.line=pc

        si=sig_int

        if pc >= finalline || endFunc || si {
            break
        }

        // race condition: winching check
        if !lockSafety && winching {
            pane_redef()
        }

        // get the next Phrase
        inbound     = &functionspaces[base][pc]
        lastline    = inbound.Tokens[0].Line

        // .. skip comments and DOC statements
        if !testMode && inbound.Tokens[0].tokType == C_Doc {
            continue
        }

        // tokencount  = inbound.TokenCount // length of phrase
/*
        if tokencount == 1 { // if the entire line is a placeholding non-statement then skip
            switch inbound.Tokens[0].tokType {
            case C_Semicolon, EOL, EOF:
                continue
            }
        }

        // remove trailing C_Semicolon token remnants
        // if tokencount > 1 {
            if inbound.Tokens[tokencount-1].tokType == C_Semicolon {
                inbound.TokenCount--
                tokencount--
                inbound.Tokens = inbound.Tokens[:tokencount]
            }
        // }
*/

        // finally... start processing the statement.
   ondo_reenter:

        statement = inbound.Tokens[0]

        // append statements to a function if currently inside a DEFINE block.
        if defining && statement.tokType != C_Enddef {
            lmv,_:=fnlookup.lmget(definitionName)
            fspacelock.Lock()
            functionspaces[lmv] = append(functionspaces[lmv], *inbound)
            fspacelock.Unlock()
            continue
        }

        // struct building
        if structMode && statement.tokType!=C_Endstruct {
            // consume the statement as an identifier
            // as we are only accepting simple types currently, restrict validity
            //  to single type token.
            if inbound.TokenCount<2 {
                parser.report(sf("Invalid STRUCT entry '%v'",statement.tokText))
                finish(false,ERR_SYNTAX)
                break
            }
            // @todo: add a check here for syntax. for example, placing type before name will result
            //  in an error during INIT. need to raise error here instead. (on order and type validity).
            cet :=crushEvalTokens(inbound.Tokens[1:])
            structNode=append(structNode,statement.tokText,cet.text)
            continue
        }

        // abort this phrase if currently inside a TEST block but the test flag is not set.
        if inside_test {
            if statement.tokType != C_Endtest && !under_test {
                continue
            }
        }


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

            endfound, enddistance, _ := lookahead(base, pc, 0, 0, C_Endwhile, []uint8{C_While}, []uint8{C_Endwhile})

            if !endfound {
                parser.report( "could not find an ENDWHILE")
                finish(false, ERR_SYNTAX)
                break
            }

            // if cond false, then jump to end while
            // if true, stack the cond then continue

            // eval

            var res bool
            var etoks []Token

            if inbound.TokenCount==1 {
                etoks=[]Token{Token{tokType:Identifier,tokText:"true"}}
                res=true
            } else {

                etoks=inbound.Tokens[1:]

                expr := wrappedEval(parser,ifs, etoks, true)
                if expr.evalError {
                    parser.report( "could not evaluate WHILE condition")
                    finish(false,ERR_EVAL)
                    break
                }

                switch expr.result.(type) {
                case bool:
                    res = expr.result.(bool)
                default:
                    parser.report( "WHILE condition must evaluate to boolean")
                    finish(false,ERR_EVAL)
                    break
                }

            }

            if isBool(res) && res {
                // while cond is true, stack, then continue loop
                if lockSafety { looplock.Lock() }
                if lockSafety { lastlock.Lock() }
                depth[ifs]++
                loops[ifs][depth[ifs]] = s_loop{repeatFrom: pc, whileContinueAt: pc + enddistance, repeatCond: etoks, loopType: C_While}
                lastConstruct[ifs] = append(lastConstruct[ifs], C_While)
                if lockSafety { lastlock.Unlock() }
                if lockSafety { looplock.Unlock() }
                break
            } else {
                // goto endwhile
                pc += enddistance
            }


        case C_Endwhile:

            // re-evaluate, on true jump back to start, on false, destack and continue

            if lockSafety { looplock.Lock() }

            cond := loops[ifs][depth[ifs]]

            if cond.loopType != C_While {
                parser.report(  "ENDWHILE outside of WHILE loop.")
                finish(false, ERR_SYNTAX)
                if lockSafety { looplock.Unlock() }
                break
            }

            // time to die?
            if breakIn == C_Endwhile {

                if lockSafety { lastlock.Lock() }
                lastConstruct[ifs] = lastConstruct[ifs][:depth[ifs]-1]
                depth[ifs]--
                if lockSafety { lastlock.Unlock() }
                if lockSafety { looplock.Unlock() }
                breakIn = Error
                break
            }

            if lockSafety { looplock.Unlock() }

            // eval
            expr := wrappedEval(parser,ifs, cond.repeatCond, true)
            if expr.evalError {
                parser.report(sf("eval fault in ENDWHILE\n%+v\n",expr.errVal))
                finish(false,ERR_EVAL)
                break
            }

            if expr.result.(bool) {
                // while still true, loop 
                pc = cond.repeatFrom
            } else {
                // was false, so leave the loop
                if lockSafety { looplock.Lock() }
                if lockSafety { lastlock.Lock() }
                lastConstruct[ifs] = lastConstruct[ifs][:depth[ifs]-1]
                depth[ifs]--
                if lockSafety { lastlock.Unlock() }
                if lockSafety { looplock.Unlock() }
            }


        case C_SetGlob: // set the value of a global variable.

           if inbound.TokenCount<4 {
                parser.report( "missing value in setglob.")
                finish(false,ERR_SYNTAX)
                break
            }

            pos:=-1
            for t:=1; t<inbound.TokenCount; t++ {
                if inbound.Tokens[t].tokType==C_Assign { pos=t; break }
            }

            if pos==-1 || pos==inbound.TokenCount-1 || pos==1 {
                parser.report("SETGLOB syntax error")
                finish(false,ERR_SYNTAX)
                break
            }

            var expr ExpressionCarton
            var err error

            expr.result, err = parser.Eval(ifs,inbound.Tokens[pos+1:])
            if err!=nil {
                parser.report("could not evaluate expression in SETGLOB")
                finish(false,ERR_SYNTAX)
                break
            }

            // pos-1 because we have removed the setglob token at front:
            expr=parser.doAssign(globalaccess,ifs,inbound.Tokens[1:],expr,pos-1)

            if expr.evalError {
                parser.report(sf("error in SETGLOB assignment\n%+v\n",expr.errVal))
                finish(false,ERR_EVAL)
                break
            }


        case C_Foreach:

            // FOREACH var IN expr
            // iterates over the result of expression expr as a list

            if inbound.TokenCount<4 {
                parser.report( "bad argument length in FOREACH.")
                finish(false,ERR_SYNTAX)
                break
            }

            if str.ToLower(inbound.Tokens[2].tokText) != "in" {
                parser.report(  "malformed FOREACH statement.")
                finish(false, ERR_SYNTAX)
                break
            }

            if inbound.Tokens[1].tokType != Identifier {
                parser.report(  "parameter 2 must be an identifier.")
                finish(false, ERR_SYNTAX)
                break
            }

            var ce int

            fid,_ := interpolate(ifs,inbound.Tokens[1].tokText,true)

            switch inbound.Tokens[3].tokType {

            case NumericLiteral, StringLiteral, LeftSBrace, Identifier, Expression, C_AssCommand:

                expr := wrappedEval(parser,ifs, inbound.Tokens[3:], true)
                if expr.evalError {
                    parser.report( sf("error evaluating term in FOREACH statement '%v'\n%+v\n",expr.text,expr.errVal))
                    finish(false,ERR_EVAL)
                    break
                }

                var l int
                switch lv:=expr.result.(type) {
                case string:
                    l=len(lv)
                case []string:
                    l=len(lv)
                case []int:
                    l=len(lv)
                case []int32:
                    l=len(lv)
                case []int64:
                    l=len(lv)
                case []float64:
                    l=len(lv)
                case []bool:
                    l=len(lv)
                case []uint8:
                    l=len(lv)
                case map[string]string:
                    l=len(lv)
                case map[string][]string:
                    l=len(lv)
                case map[string][]int:
                    l=len(lv)
                case map[string][]bool:
                    l=len(lv)
                case map[string][]float64:
                    l=len(lv)
                case []map[string]interface{}:
                    l=len(lv)
                case map[string]interface{}:
                    l=len(lv)
                case []interface{}:
                    l=len(lv)
                default:
                    // pf("Unknown loop type [%T]\n",lv)
                }

                if l==0 {
                    // skip empty expressions
                    endfound, enddistance, _ := lookahead(base, pc, 0, 0, C_Endfor, []uint8{C_Foreach}, []uint8{C_Endfor})
                    if !endfound {
                        parser.report(  "Cannot determine the location of a matching ENDFOR.")
                        finish(false, ERR_SYNTAX)
                        break
                    } else { //skip
                        pc += enddistance
                        break
                    }
                }

                var iter *reflect.MapIter

                switch expr.result.(type) {

                case string:

                    // split and treat as array if multi-line

                    // remove a single trailing \n from string
                    elast := len(expr.result.(string)) - 1
                    if expr.result.(string)[elast] == '\n' {
                        expr.result = expr.result.(string)[:elast]
                    }

                    // split up string at \n divisions into an array
                    if runtime.GOOS!="windows" {
                        expr.result = str.Split(expr.result.(string), "\n")
                    } else {
                        expr.result = str.Split(str.Replace(expr.result.(string), "\r\n", "\n", -1), "\n")
                    }

                    if len(expr.result.([]string))>0 {
                        vset(ifs, "key_"+fid, 0)
                        vset(ifs, fid, expr.result.([]string)[0])
                        ce = len(expr.result.([]string)) - 1
                    }

                case map[string]float64:
                    if len(expr.result.(map[string]float64)) > 0 {
                        // get iterator for this map
                        iter = reflect.ValueOf(expr.result.(map[string]float64)).MapRange()

                        // set initial key and value
                        if iter.Next() {
                            vset(ifs, "key_"+fid, iter.Key().String())
                            vset(ifs, fid, iter.Value().Interface())
                        }
                        ce = len(expr.result.(map[string]float64)) - 1
                    }

                case map[string]int:
                    if len(expr.result.(map[string]int)) > 0 {
                        // get iterator for this map
                        iter = reflect.ValueOf(expr.result.(map[string]int)).MapRange()

                        // set initial key and value
                        if iter.Next() {
                            vset(ifs, "key_"+fid, iter.Key().String())
                            vset(ifs, fid, iter.Value().Interface())
                        }
                        ce = len(expr.result.(map[string]int)) - 1
                    }

                case map[string]string:

                    if len(expr.result.(map[string]string)) > 0 {

                        // get iterator for this map
                        iter = reflect.ValueOf(expr.result.(map[string]string)).MapRange()

                        // set initial key and value
                        if iter.Next() {
                            vset(ifs, "key_"+fid, iter.Key().String())
                            vset(ifs, fid, iter.Value().Interface())
                        } else {
                            // empty
                        }
                        ce = len(expr.result.(map[string]string)) - 1
                    }

                case map[string][]string:

                    if len(expr.result.(map[string][]string)) > 0 {

                        // get iterator for this map
                        iter = reflect.ValueOf(expr.result.(map[string][]string)).MapRange()

                        // set initial key and value
                        if iter.Next() {
                            vset(ifs, "key_"+fid, iter.Key().String())
                            vset(ifs, fid, iter.Value().Interface())
                        } else {
                            // empty
                        }
                        ce = len(expr.result.(map[string][]string)) - 1
                    }

                case []float64:

                    if len(expr.result.([]float64)) > 0 {
                        vset(ifs, "key_"+fid, 0)
                        vset(ifs, fid, expr.result.([]float64)[0])
                        ce = len(expr.result.([]float64)) - 1
                    }

                case float64: // special case: float
                    expr.result = []float64{expr.result.(float64)}
                    if len(expr.result.([]float64)) > 0 {
                        vset(ifs, "key_"+fid, 0)
                        vset(ifs, fid, expr.result.([]float64)[0])
                        ce = len(expr.result.([]float64)) - 1
                    }

                case []uint8:
                    if len(expr.result.([]uint8)) > 0 {
                        vset(ifs, "key_"+fid, 0)
                        vset(ifs, fid, expr.result.([]uint8)[0])
                        ce = len(expr.result.([]uint8)) - 1
                    }

                case []bool:
                    if len(expr.result.([]bool)) > 0 {
                        vset(ifs, "key_"+fid, 0)
                        vset(ifs, fid, expr.result.([]bool)[0])
                        ce = len(expr.result.([]bool)) - 1
                    }

                case []int:
                    if len(expr.result.([]int)) > 0 {
                        vset(ifs, "key_"+fid, 0)
                        vset(ifs, fid, expr.result.([]int)[0])
                        ce = len(expr.result.([]int)) - 1
                    }

                case int: // special case: int
                    expr.result = []int{expr.result.(int)}
                    if len(expr.result.([]int)) > 0 {
                        vset(ifs, "key_"+fid, 0)
                        vset(ifs, fid, expr.result.([]int)[0])
                        ce = len(expr.result.([]int)) - 1
                    }

                case []int32:
                    if len(expr.result.([]int32)) > 0 {
                        vset(ifs, "key_"+fid, 0)
                        vset(ifs, fid, expr.result.([]int32)[0])
                        ce = len(expr.result.([]int32)) - 1
                    }

                case []int64:
                    if len(expr.result.([]int64)) > 0 {
                        vset(ifs, "key_"+fid, 0)
                        vset(ifs, fid, expr.result.([]int64)[0])
                        ce = len(expr.result.([]int64)) - 1
                    }

                case []float32:
                    if len(expr.result.([]float32)) > 0 {
                        vset(ifs, "key_"+fid, 0)
                        vset(ifs, fid, expr.result.([]float32)[0])
                        ce = len(expr.result.([]float32)) - 1
                    }

                case []string:
                    if len(expr.result.([]string)) > 0 {
                        vset(ifs, "key_"+fid, 0)
                        vset(ifs, fid, expr.result.([]string)[0])
                        ce = len(expr.result.([]string)) - 1
                    }

                case []map[string]interface{}:

                    if len(expr.result.([]map[string]interface{})) > 0 {
                        vset(ifs, "key_"+fid, 0)
                        vset(ifs, fid, expr.result.([]map[string]interface{})[0])
                        ce = len(expr.result.([]map[string]interface{})) - 1
                    }

                case map[string]interface{}:

                    if len(expr.result.(map[string]interface{})) > 0 {

                        // get iterator for this map
                        iter = reflect.ValueOf(expr.result.(map[string]interface{})).MapRange()

                        // set initial key and value
                        if iter.Next() {
                            vset(ifs, "key_"+fid, iter.Key().String())
                            vset(ifs, fid, iter.Value().Interface())
                        } else {
                            // empty
                        }
                        ce = len(expr.result.(map[string]interface{})) - 1
                    }

                case []interface{}:

                    if len(expr.result.([]interface{})) > 0 {
                        vset(ifs, "key_"+fid, 0)
                        vset(ifs, fid, expr.result.([]interface{})[0])
                        ce = len(expr.result.([]interface{})) - 1
                    }

                default:
                    parser.report( sf("Mishandled return of type '%T' from FOREACH expression '%v'\n", expr.result,expr.result))
                    finish(false,ERR_EVAL)
                    break
                }

                // figure end position
                endfound, enddistance, _ := lookahead(base, pc, 0, 0, C_Endfor, []uint8{C_Foreach}, []uint8{C_Endfor})
                if !endfound {
                    parser.report(  "Cannot determine the location of a matching ENDFOR.")
                    finish(false, ERR_SYNTAX)
                    break
                }

                if lockSafety { looplock.Lock() }
                if lockSafety { lastlock.Lock() }

                depth[ifs]++
                lastConstruct[ifs] = append(lastConstruct[ifs], C_Foreach)

                // pf("ifs:%v depth:%v len_depth:%v len_loops:%v\n",ifs,depth[ifs],len(depth),len(loops))
                // pf("loop ifs:\n%#v\n",loops[ifs])

                loops[ifs][depth[ifs]] = s_loop{loopVar: fid, repeatFrom: pc + 1,
                    iterOverMap: iter, iterOverArray: expr.result,
                    counter: 0, condEnd: ce, forEndPos: enddistance + pc,
                    loopType: C_Foreach,
                }


                if lockSafety { lastlock.Unlock() }
                if lockSafety { looplock.Unlock() }

            }

        case C_For: // loop over an int64 range

            if inbound.TokenCount < 5 || inbound.Tokens[2].tokText != "=" {
                parser.report(  "Malformed FOR statement.")
                finish(false, ERR_SYNTAX)
                break
            }

            toAt := findDelim(inbound.Tokens, "to", 2)
            if toAt == -1 {
                parser.report(  "TO not found in FOR")
                finish(false, ERR_SYNTAX)
                break
            }

            stepAt := findDelim(inbound.Tokens, "step", toAt)
            stepped := true
            if stepAt == -1 {
                stepped = false
            }

            var fstart, fend, fstep int
            var expr interface{}
            // var validated bool
            var err error

            if toAt>3 {
                expr, err = EvalCrush(parser,ifs, inbound.Tokens, 3, toAt-1)
                if err==nil && isNumber(expr) {
                    fstart, _ = GetAsInt(expr)
                } else {
                    parser.report(  "Could not evaluate start expression in FOR")
                    finish(false, ERR_EVAL)
                    break
                }
            } else {
                parser.report( "Missing expression in FOR statement?")
                finish(false,ERR_SYNTAX)
                break
            }

            if inbound.TokenCount>toAt+1 {
                if stepAt>0 {
                    expr, err = EvalCrush(parser,ifs, inbound.Tokens, toAt+1, stepAt-1)
                } else {
                    expr, err = EvalCrushRest(parser,ifs, inbound.Tokens, toAt+1)
                }
                if err==nil && isNumber(expr) {
                    fend, _ = GetAsInt(expr)
                } else {
                    parser.report(  "Could not evaluate end expression in FOR")
                    finish(false, ERR_EVAL)
                    break
                }
            } else {
                parser.report( "Missing expression in FOR statement?")
                finish(false,ERR_SYNTAX)
                break
            }

            if stepped {
                if inbound.TokenCount>stepAt+1 {
                    expr, err := EvalCrushRest(parser,ifs, inbound.Tokens, stepAt+1)
                    if err==nil && isNumber(expr) {
                        fstep, _ = GetAsInt(expr)
                    } else {
                        parser.report(  "Could not evaluate STEP expression")
                        finish(false, ERR_EVAL)
                        break
                    }
                } else {
                    parser.report( "Missing expression in FOR statement?")
                    finish(false,ERR_SYNTAX)
                    break
                }
            }

            step := 1
            if stepped {
                step = fstep
            }
            if step == 0 {
                parser.report(  "This is a road to nowhere. (STEP==0)")
                finish(true, ERR_EVAL)
                break
            }

            direction := ACT_INC
            if step < 0 {
                direction = ACT_DEC
            }

            // figure end position
            endfound, enddistance, _ := lookahead(base, pc, 0, 0, C_Endfor, []uint8{C_For}, []uint8{C_Endfor})
            if !endfound {
                parser.report(  "Cannot determine the location of a matching ENDFOR.")
                finish(false, ERR_SYNTAX)
                break
            }

            // @note: if loop counter is never used between here and C_Endfor, then don't vset the local var

            // store loop data
            inter,_:=interpolate(ifs,inbound.Tokens[1].tokText,true)

            if lockSafety { lastlock.Lock() }
            if lockSafety { looplock.Lock() }

            depth[ifs]++
            loops[ifs][depth[ifs]] = s_loop{
                loopVar:  inter,
                optNoUse: Opt_LoopStart,
                loopType: C_For, forEndPos: pc + enddistance, repeatFrom: pc + 1,
                counter: fstart, condEnd: fend,
                repeatAction: direction, repeatActionStep: step,
            }

            // store loop start condition
            vset(ifs, inter, fstart)

            lastConstruct[ifs] = append(lastConstruct[ifs], C_For)

            if lockSafety { looplock.Unlock() }
            if lockSafety { lastlock.Unlock() }


        case C_Endfor: // terminate a FOR or FOREACH block

            if lockSafety { looplock.Lock() }
            if lockSafety { lastlock.Lock() }

            if depth[ifs]==0 {
                pf("*debug* trying to get lastConstruct when there isn't one in ifs->%v!\n",ifs)
                finish(true,ERR_FATAL)
                break
            }

            if lastConstruct[ifs][depth[ifs]-1]!=C_For && lastConstruct[ifs][depth[ifs]-1]!=C_Foreach {
                parser.report( "ENDFOR without a FOR or FOREACH")
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

                    (*thisLoop).counter++

                    if (*thisLoop).counter > (*thisLoop).condEnd {
                        loopEnd = true
                    } else {

                        // assign value back to local variable

                        switch (*thisLoop).iterOverArray.(type) {

                        // map ranges are randomly ordered!!
                        case map[string]interface{}, map[string]int, map[string]float64, map[string]string, map[string][]string:
                            if (*thisLoop).iterOverMap.Next() { // true means not exhausted
                                vset(ifs, "key_"+(*thisLoop).loopVar, (*thisLoop).iterOverMap.Key().String())
                                vset(ifs, (*thisLoop).loopVar, (*thisLoop).iterOverMap.Value().Interface())
                            }

                        case []bool:
                            vset(ifs, "key_"+(*thisLoop).loopVar, (*thisLoop).counter)
                            vset(ifs, (*thisLoop).loopVar, (*thisLoop).iterOverArray.([]bool)[(*thisLoop).counter])
                        case []int:
                            vset(ifs, "key_"+(*thisLoop).loopVar, (*thisLoop).counter)
                            vset(ifs, (*thisLoop).loopVar, (*thisLoop).iterOverArray.([]int)[(*thisLoop).counter])
                        case []uint8:
                            vset(ifs, "key_"+(*thisLoop).loopVar, (*thisLoop).counter)
                            vset(ifs, (*thisLoop).loopVar, (*thisLoop).iterOverArray.([]uint8)[(*thisLoop).counter])
                        case []int32:
                            vset(ifs, "key_"+(*thisLoop).loopVar, (*thisLoop).counter)
                            vset(ifs, (*thisLoop).loopVar, (*thisLoop).iterOverArray.([]int32)[(*thisLoop).counter])
                        case []int64:
                            vset(ifs, "key_"+(*thisLoop).loopVar, (*thisLoop).counter)
                            vset(ifs, (*thisLoop).loopVar, (*thisLoop).iterOverArray.([]int64)[(*thisLoop).counter])
                        case []string:
                            vset(ifs, "key_"+(*thisLoop).loopVar, (*thisLoop).counter)
                            vset(ifs, (*thisLoop).loopVar, (*thisLoop).iterOverArray.([]string)[(*thisLoop).counter])
                        case []float32:
                            vset(ifs, "key_"+(*thisLoop).loopVar, (*thisLoop).counter)
                            vset(ifs, (*thisLoop).loopVar, (*thisLoop).iterOverArray.([]float32)[(*thisLoop).counter])
                        case []float64:
                            vset(ifs, "key_"+(*thisLoop).loopVar, (*thisLoop).counter)
                            vset(ifs, (*thisLoop).loopVar, (*thisLoop).iterOverArray.([]float64)[(*thisLoop).counter])
                        case []map[string]interface{}:
                            vset(ifs, "key_"+(*thisLoop).loopVar, (*thisLoop).counter)
                            vset(ifs, (*thisLoop).loopVar, (*thisLoop).iterOverArray.([]map[string]interface{})[(*thisLoop).counter])
                        case []interface{}:
                            vset(ifs, "key_"+(*thisLoop).loopVar, (*thisLoop).counter)
                            vset(ifs, (*thisLoop).loopVar, (*thisLoop).iterOverArray.([]interface{})[(*thisLoop).counter])
                        default:
                            // @note: should put a proper exit in here.
                            pv,_:=vget(ifs,sf("%v",(*thisLoop).iterOverArray.([]float64)[(*thisLoop).counter]))
                            pf("Unknown type [%T] in END/Foreach\n",pv)
                        }

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
                        if !lockSafety {
                            vid, _ = VarLookup(ifs,(*thisLoop).loopVar)
                            ident[ifs][vid].IValue = (*thisLoop).counter
                        } else {
                            vset(ifs, (*thisLoop).loopVar, (*thisLoop).counter)
                        }
                    }

                }

            } else {
                // time to die, mr bond? C_Break reached
                breakIn = Error // reset to unbroken
                loopEnd = true
            }

            if loopEnd {
                // leave the loop
                lastConstruct[ifs] = lastConstruct[ifs][:depth[ifs]-1]
                depth[ifs]--
                breakIn = Error // reset to unbroken
            } else {
                // jump back to start of block
                pc = (*thisLoop).repeatFrom - 1 // start of loop will do pc++
            }

            if lockSafety { lastlock.Unlock() }
            if lockSafety { looplock.Unlock() }

        case C_Continue:

            // Continue should work with FOR, FOREACH or WHILE.

            if lockSafety { looplock.RLock() }

            if depth[ifs] == 0 {
                parser.report(  "Attempting to CONTINUE without a valid surrounding construct.")
                finish(false, ERR_SYNTAX)
            } else {

                var thisLoop *s_loop

                // ^^ we use indirect access here (and throughout loop code) for a minor speed bump.
                // ^^ we should periodically review this as an optimisation in Go could make this unnecessary.

                if lockSafety { lastlock.RLock() }
                switch lastConstruct[ifs][depth[ifs]-1] {

                case C_For, C_Foreach:
                    thisLoop = &loops[ifs][depth[ifs]]
                    pc = (*thisLoop).forEndPos - 1

                case C_While:
                    thisLoop = &loops[ifs][depth[ifs]]
                    pc = (*thisLoop).whileContinueAt - 1
                }
                if lockSafety { lastlock.RUnlock() }

            }
            if lockSafety { looplock.RUnlock() }

        case C_Break:

            // Break should work with either FOR, FOREACH, WHILE or WHEN.
            // We use lastConstruct to establish which is the innermost
            // of these from which we need to break out.

            // The surrounding construct should set the lastConstruct[fs][depth] on entry.

            if lockSafety { looplock.RLock() }
            if lockSafety { lastlock.RLock() }

            if depth[ifs] == 0 {
                parser.report(  "Attempting to BREAK without a valid surrounding construct.")
                finish(false, ERR_SYNTAX)
            } else {

                // jump calc, depending on break context

                // var thisLoop *s_loop
                thisLoop := &loops[ifs][depth[ifs]]
                bmess := ""

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
                    parser.report(  "A grue is attempting to BREAK out. (Breaking without a surrounding context!)")
                    finish(false, ERR_SYNTAX)
                    if lockSafety { lastlock.RUnlock() }
                    if lockSafety { looplock.RUnlock() }
                    break
                }

                if breakIn != Error {
                    debug(5, "** break %v\n", bmess)
                }

            }

            if lockSafety { lastlock.RUnlock() }
            if lockSafety { looplock.RUnlock() }

        case C_Unset: // remove a variable

            // @note: need to look at this...
            //  unset on a map entry would be fine.
            //  unset on an array element is pointless.
            //  unset on a scalar should be fine.
            //  basically, need to add more intelligence to this.
            //  just keeping it from attempting the deletion for now.

            if inbound.TokenCount != 2 {
                parser.report(  "Incorrect arguments supplied for UNSET.")
                finish(false, ERR_SYNTAX)
            } else {
                removee := inbound.Tokens[1].tokText
                if _, ok := VarLookup(ifs, removee); ok {
                    // vunset(ifs, removee)
                } else {
                    parser.report( sf("Variable %s does not exist.", removee))
                    finish(false, ERR_EVAL)
                }
            }


        case C_Pane:

            if inbound.TokenCount == 1 {
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
                if inbound.TokenCount != 2 {
                    parser.report(  "Too many arguments supplied.")
                    finish(false, ERR_SYNTAX)
                    break
                }
                // disable
                panes = make(map[string]Pane)
                panes["global"] = Pane{row: 0, col: 0, h: MH, w: MW + 1}
                currentpane = "global"

            case "select":

                if inbound.TokenCount != 3 {
                    parser.report(  "Invalid pane selection.")
                    finish(false, ERR_SYNTAX)
                    break
                }

                cp, _ := ev(parser,ifs, inbound.Tokens[2].tokText, true,true)

                switch cp:=cp.(type) {
                case string:
                    setPane(cp)
                    currentpane = cp

                default:
                    parser.report( "Warning: you must provide a string value to PANE SELECT.")
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
                    parser.report(  "Bad delimiter in PANE DEFINE.")
                    // pf("Toks -> [%+v]\n", inbound.Tokens)
                    finish(false, ERR_SYNTAX)
                    break
                }

                hasTitle:=false; hasBox:=false
                if TCommaAt>-1 {
                    hasTitle=true
                    if TCommaAt<inbound.TokenCount-1 {
                        hasBox=true
                    }
                }

                // var ew,etit,ebox ExpressionCarton
                var ew,etit,ebox []Token

                if hasTitle {
                    ew    = inbound.Tokens[ HCommaAt+1:WCommaAt   ]
                } else {
                    ew    = inbound.Tokens[ HCommaAt+1: ]
                }

                if hasTitle && hasBox {
                    etit = inbound.Tokens[ WCommaAt+1 : TCommaAt ]
                    ebox = inbound.Tokens[ TCommaAt+1 : ]
                } else {
                    if hasTitle {
                        etit = inbound.Tokens[ WCommaAt+1 : ]
                    }
                }

                var ptitle, pbox ExpressionCarton
                pname  := wrappedEval(parser,ifs, inbound.Tokens[2:nameCommaAt], true)
                py     := wrappedEval(parser,ifs, inbound.Tokens[nameCommaAt+1:YCommaAt], true)
                px     := wrappedEval(parser,ifs, inbound.Tokens[YCommaAt+1:XCommaAt], true)
                ph     := wrappedEval(parser,ifs, inbound.Tokens[XCommaAt+1:HCommaAt], true)
                pw     := wrappedEval(parser,ifs, ew, true)
                if hasTitle {
                    ptitle = wrappedEval(parser,ifs, etit, true)
                }
                if hasBox   {
                    pbox   = wrappedEval(parser,ifs, ebox, true)
                }

                if pname.evalError || py.evalError || px.evalError || ph.evalError || pw.evalError {
                    parser.report( "could not evaluate an argument in PANE DEFINE")
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
                    parser.report( "Could not use an argument in PANE DEFINE.")
                    // pf("Toks -> [%+v]\n", inbound.Tokens)
                    finish(false,ERR_EVAL)
                    break
                }

                if pname.result.(string) == "global" {
                    parser.report( "Cannot redefine the global PANE.")
                    finish(false, ERR_SYNTAX)
                    break
                }

                panes[name] = Pane{row: row, col: col, w: w, h: h, title: title, boxed: boxed}
                paneBox(name)

            case "redraw":
                paneBox(currentpane)

            default:
                parser.report(  "Unknown PANE command.")
                finish(false, ERR_SYNTAX)
            }


        case SYM_BOR: // Local Command

            if inbound.TokenCount==2 && hasOuter(inbound.Tokens[1].tokText,'`') {
                s:=stripOuter(inbound.Tokens[1].tokText,'`')
                coprocCall(ifs,"|"+s)
            } else {
                coprocCall(ifs,inbound.Original)
            }

        case C_Pause:

            var i string

            if inbound.TokenCount<2 {
                parser.report(  "Not enough arguments in PAUSE.")
                finish(false, ERR_SYNTAX)
                break
            }

            expr := wrappedEval(parser,ifs, inbound.Tokens[1:], true)

            if !expr.evalError {

                if isNumber(expr.result) {
                    i = sf("%v", expr.result)
                } else {
                    i = expr.result.(string)
                }

                dur, err := time.ParseDuration(i + "ms")

                if err != nil {
                    parser.report(  sf("'%s' did not evaluate to a duration.", expr.text))
                    finish(false, ERR_EVAL)
                    break
                }

                time.Sleep(dur)

            } else {
                parser.report(  sf("could not evaluate PAUSE expression\n%+v",expr.errVal))
                finish(false, ERR_EVAL)
                break
            }


        case C_Doc:
            var badval bool
            if testMode {
                if inbound.TokenCount > 1 {
                    docout := ""
                    previousterm := 1
                    for term := range inbound.Tokens[1:] {
                        if inbound.Tokens[term].tokType == C_Comma {

                            expr := wrappedEval(parser,ifs, inbound.Tokens[previousterm:term], true)
                            if expr.evalError {
                                parser.report( sf("bad value in DOC command\n%+v",expr.errVal))
                                finish(false,ERR_EVAL)
                                badval=true
                                break
                            }

                            docout += sparkle(sf(`%v`, expr.result))
                            previousterm = term + 1

                        }
                    }

                    if badval { break }

                    expr := wrappedEval(parser,ifs, inbound.Tokens[previousterm:], true)
                    if expr.evalError {
                        parser.report( sf("bad value in DOC command\n%+v",expr.errVal))
                        finish(false,ERR_EVAL)
                    } else {
                        docout += sparkle(sf(`%v`, expr.result))
                    }

                    appendToTestReport(test_output_file,ifs, pc, docout)

                }
            }


        case C_Test:

            // TEST "name" GROUP "group_name" ASSERT FAIL|CONTINUE

            inside_test = true

            if testMode {

                if !(inbound.TokenCount == 4 || inbound.TokenCount == 6) {
                    parser.report(  "Badly formatted TEST command.")
                    finish(false, ERR_SYNTAX)
                    break
                }

                if str.ToLower(inbound.Tokens[2].tokText) != "group" {
                    parser.report(  "Missing GROUP in TEST command.")
                    finish(false, ERR_SYNTAX)
                    break
                }

                test_assert = "fail"
                if inbound.TokenCount == 6 {
                    if str.ToLower(inbound.Tokens[4].tokText) != "assert" {
                        parser.report(  "Missing ASSERT in TEST command.")
                        finish(false, ERR_SYNTAX)
                        break
                    } else {
                        switch str.ToLower(inbound.Tokens[5].tokText) {
                        case "fail":
                            test_assert = "fail"
                        case "continue":
                            test_assert = "continue"
                        default:
                            parser.report(  "Bad ASSERT type in TEST command.")
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
                    vset(ifs,"_test_group",test_group)
                    vset(ifs,"_test_name",test_name)
                    under_test = true
                    appendToTestReport(test_output_file,ifs, pc, sf("\nTest Section : [#5][#bold]%s/%s[#boff][#-]",test_group,test_name))
                }

            }


        case C_Endtest:

            under_test = false
            inside_test = false


        case C_On:
            // ON expr DO action
            // was false? - discard command tokens and continue
            // was true? - reform command without the 'ON condition' tokens and re-enter command switch

            // > print tokens("on int(diff_{i})<0 do print")
            //  on int        (      diff_42    )      <      0         do         print
            //  ON IDENTIFIER LPAREN IDENTIFIER RPAREN SYM_LT N_LITERAL IDENTIFIER PRINT
            //  0  1          2      3          4      5      6         7          8...

            if inbound.TokenCount > 2 {

                doAt := findDelim(inbound.Tokens, "do", 2)
                if doAt == -1 {
                    parser.report(  "DO not found in ON")
                    finish(false, ERR_SYNTAX)
                } else {
                    // more tokens after the DO to form a command with?
                    if inbound.TokenCount >= doAt {

                        expr := wrappedEval(parser,ifs, inbound.Tokens[1:doAt], true)
                        if expr.evalError {
                            parser.report( sf("Could not evaluate expression '%v' in ON..DO statement.\n%+v",expr.text,expr.errVal))
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
                                // we can ignore .Text and .Original for now - but shouldn't.
                                // they are only used in *Command calls, and the input is chomped
                                // from the front to the first pipe symbol so the 'ON expr DO' would
                                // be consumed. However, @todo: fix this.

                                // action!
                                inbound=&p
                                goto ondo_reenter

                            }
                        default:
                            pf("Result Type -> %T\n", expr.result)
                            parser.report( "ON cannot operate without a condition.")
                            finish(false, ERR_EVAL)
                            break
                        }

                    }
                }

            } else {
                parser.report( "ON missing arguments.")
                finish(false, ERR_SYNTAX)
            }


        case C_Assert:

            if inbound.TokenCount < 2 {

                parser.report(  "Insufficient arguments supplied to ASSERT")
                finish(false, ERR_ASSERT)

            } else {

                cet := crushEvalTokens(inbound.Tokens[1:])
                expr := wrappedEval(parser,ifs, inbound.Tokens[1:], true)

                if expr.assign {
                    // someone typo'ed a condition 99.9999% of the time
                    parser.report(
                        sf("[#2][#bold]Warning! Assert contained an assignment![#-][#boff]\n  [#6]%v = %v[#-]\n",cet.assignVar,cet.text))
                    finish(false,ERR_ASSERT)
                    break
                }

                if expr.evalError {
                    parser.report(  "Could not evaluate expression in ASSERT statement.")
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
                            parser.report(  sf("Could not assert! ( %s )", expr.text))
                            finish(false, ERR_ASSERT)
                            break
                        }
                        // under test
                        test_report = sf("[#2]TEST FAILED %s (%s/line %d) : %s[#-]", group_name_string, getReportFunctionName(ifs,false), lastline, expr.text)
                        testsFailed++
                        appendToTestReport(test_output_file,ifs, lastline, test_report)
                        temp_test_assert := test_assert
                        if fail_override != "" {
                            temp_test_assert = fail_override
                        }
                        switch temp_test_assert {
                        case "fail":
                            parser.report(  sf("Could not assert! (%s)", expr.text))
                            finish(false, ERR_ASSERT)
                        case "continue":
                            parser.report(  sf("Assert failed (%s), but continuing.", expr.text))
                        }
                    } else {
                        if under_test {
                            test_report = sf("[#4]TEST PASSED %s (%s/line %d) : %s[#-]", group_name_string, getReportFunctionName(ifs,false), lastline, expr.text)
                            testsPassed++
                            appendToTestReport(test_output_file,ifs, pc, test_report)
                        }
                    }
                }

            }


        case C_Init: // initialise an array

            if inbound.TokenCount<2 {
                parser.report( "Not enough arguments in INIT.")
                finish(false,ERR_EVAL)
                break
            }

            varname,_ := interpolate(ifs,inbound.Tokens[1].tokText,true)
            vartype := "assoc"
            if inbound.TokenCount>2 {
                vartype = inbound.Tokens[2].tokText
            }

            size:=DEFAULT_INIT_SIZE

            if inbound.TokenCount>3 {

                expr := wrappedEval(parser,ifs, inbound.Tokens[3:], true)
                if expr.evalError {
                    parser.report( sf("could not evaluate expression in INIT statement\n%+v",expr.errVal))
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
                    parser.report( "Array width must evaluate to an integer.")
                    finish(false,ERR_EVAL)
                    break
                }

            }

            if varname != "" {
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
                default:
                    //
                    // move this later:
                    var tb bool
                    var tu8 uint8
                    var tu32 uint32
                    var tu64 uint64
                    var ti int
                    var ti32 int32
                    var ti64 int64
                    var tf32 float32
                    var tf64 float64
                    var ts string
                    var atint   []interface{}
                    /* not supported yet:
                    var ats     []string
                    var ati     []int
                    var atf     []float64
                    var atb     []bool
                    */

                    // instantiate fields with an empty expected type:
                    typemap:=make(map[string]reflect.Type)
                    typemap["bool"]     = reflect.TypeOf(tb)
                    typemap["byte"]     = reflect.TypeOf(tu8)
                    typemap["uint8"]    = reflect.TypeOf(tu8)
                    typemap["uint32"]   = reflect.TypeOf(tu32)
                    typemap["uint64"]   = reflect.TypeOf(tu64)
                    typemap["int"]      = reflect.TypeOf(ti)
                    typemap["int32"]    = reflect.TypeOf(ti32)
                    typemap["int64"]    = reflect.TypeOf(ti64)
                    typemap["float"]    = reflect.TypeOf(tf64)
                    typemap["float64"]  = reflect.TypeOf(tf64)
                    typemap["float32"]  = reflect.TypeOf(tf32)
                    typemap["string"]   = reflect.TypeOf(ts)
                    /* only interface{} currently supported.
                    typemap["[]string"] = reflect.TypeOf(ats)
                    typemap["[]int"]    = reflect.TypeOf(ati)
                    typemap["[]float"]  = reflect.TypeOf(atf)
                    typemap["[]bool"]   = reflect.TypeOf(atb)
                    */
                    typemap["[]"]       = reflect.TypeOf(atint)
                    //

                    // check here for struct init by name
                    found:=false
                    structvalues:=[]string{}

                    // structmap has list of field_name,field_type,... for each struct
                    for sn, snv := range structmaps {
                        if sn==vartype {
                            found=true
                            structvalues=snv
                            break
                        }
                    }

                    if found {
                        // deal with init name struct_type
                        if len(structvalues)>0 {
                            var sf []reflect.StructField
                            offset:=uintptr(0)
                            for svpos:=0; svpos<len(structvalues); svpos+=2 {
                                nv:=structvalues[svpos]
                                nt:=structvalues[svpos+1]
                                sf=append(sf,
                                    reflect.StructField{
                                        Name:nv,PkgPath:"main",
                                        Type:typemap[nt],
                                        Offset:offset,
                                        Anonymous:false,
                                    },
                                )
                                offset+=typemap[nt].Size()
                            }
                            typ:=reflect.StructOf(sf)
                            v:=(reflect.New(typ).Elem()).Interface()
                            vset(ifs,varname,v)
                            // also register value of type with gob in case of serialisation
                            // gob.Register(v)
                        }
                    } else {
                        // handle unknown type error
                    }
                }
            }


        case C_Help:
            hargs := ""
            if inbound.TokenCount == 2 {
                hargs = inbound.Tokens[1].tokText
            }
            help(hargs)


        case C_Nop:
            time.Sleep(1 * time.Microsecond)


        case C_Async:

            // ASYNC IDENTIFIER IDENTIFIER LPAREN [EXPRESSION[,...]] RPAREN [IDENTIFIER]
            // async handles    q          (      [e[,...]]          )      [key]
            // 0     1          2          3      4

            if inbound.TokenCount<5 {
                usage := "ASYNC [#i1]handle_map function_call([args]) [next_id][#i0]"
                parser.report("Invalid arguments in ASYNC\n"+usage)
                finish(false,ERR_SYNTAX)
                break
            }

            handles,_ := interpolate(ifs,inbound.Tokens[1].tokText,true)
            call      := inbound.Tokens[2].tokText

            if inbound.Tokens[3].tokType!=LParen {
                parser.report("could not find '(' in ASYNC function call.")
                finish(false,ERR_SYNTAX)
            }

            // get arguments
            var argString str.Builder
            var rparenloc int
            for ap:=4; ap<inbound.TokenCount; ap++ {
                if inbound.Tokens[ap].tokType==RParen {
                    rparenloc=ap
                    break
                }
                if inbound.Tokens[ap].tokType==C_Comma {
                    argString.WriteString(",")
                    continue
                }
                argString.WriteString(inbound.Tokens[ap].tokText)
            }

            if rparenloc<4 {
               parser.report("could not find a valid ')' in ASYNC function call.")
                finish(false,ERR_SYNTAX)
            }

            // find the optional key argument, for stipulating the key name to be used in handles
            var nival interface{}
            if rparenloc!=inbound.TokenCount-1 {
                var err error
                keyString := crushEvalTokens(inbound.Tokens[rparenloc+1:]).text
                nival,err = ev(parser,ifs,keyString,true,true)
                if err!=nil {
                    parser.report(sf("could not evaluate handle key argument '%s' in ASYNC.",keyString))
                    finish(false,ERR_EVAL)
                    break
                }
            }

            // build task call
            lmv, isfunc := fnlookup.lmget(call)

            if isfunc {

                // evaluate args
                var iargs []interface{}
                var argnames []string

                // populate inbound parameters to the za function call, with evaluated versions of each.
                fullBreak:=false
                if argString.String() != "" {
                    argnames = str.Split(argString.String(), ",")
                    for k, a := range argnames {
                        aval, err := ev(parser,ifs, a, false, true)
                        if err != nil {
                            parser.report(sf("problem evaluating '%s' in function call arguments. (fs=%v,err=%v)\n", argnames[k], ifs, err))
                            finish(false, ERR_EVAL)
                            fullBreak=true
                            break
                        }
                        iargs = append(iargs, aval)
                    }
                }
                if fullBreak { break }

                // make Za function call
                loc,id := GetNextFnSpace(call+"@")
                calllock.Lock()
                vset(ifs,sf("@#@%v",loc),nil)
                calltable[loc] = call_s{fs: id, base: lmv, caller: ifs, callline: pc, retvar: sf("@#@%v",loc)}
                calllock.Unlock()

                // construct a go call that includes a normal Call
                h:=task(ifs,loc,iargs...)

                // pf("task returned channel id : %+v\n",h)

                // assign h to handles map
                if nival==nil {
                    vsetElement(ifs,handles,sf("async_%v",id),h)
                } else {
                    vsetElement(ifs,handles,sf("%v",nival),h)
                }

            } else {
                // func not found
                parser.report(sf("invalid function '%s' in ASYNC call",call))
                finish(false,ERR_EVAL)
            }

        case C_Debug:

            if inbound.TokenCount != 2 {

                parser.report(  "Malformed DEBUG statement.")
                finish(false, ERR_SYNTAX)

            } else {

                dval, err := EvalCrush(parser,ifs, inbound.Tokens, 1, inbound.TokenCount)
                if err==nil && isNumber(dval) {
                    debug_level = dval.(int)
                } else {
                    parser.report(  "Bad debug level value - could not evaluate.")
                    finish(false, ERR_EVAL)
                }

            }


        case C_Require:

            // require feat support in stdlib first. requires version-as-feat support and markup.

            if inbound.TokenCount < 2 || inbound.TokenCount > 3 {
                parser.report(  "Malformed REQUIRE statement.")
                finish(true, ERR_SYNTAX)
                break
            }

            var reqfeat string
            var reqvers int

            switch inbound.TokenCount {
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
            if inbound.TokenCount > 1 {
                ec, err := EvalCrush(parser,ifs, inbound.Tokens, 1, inbound.TokenCount)
                if err==nil && isNumber(ec) {
                    finish(true, ec.(int))
                } else {
                    parser.report( "Could not evaluate your EXIT expression")
                    finish(true,ERR_EVAL)
                }
            } else {
                finish(true, 0)
            }


        case C_Define:

            if inbound.TokenCount > 1 {

                if defining {
                    parser.report( "Already defining a function. Nesting not permitted.")
                    finish(true, ERR_SYNTAX)
                    break
                }

                fn := inbound.Tokens[1].tokText
                var dargs []string

                if inbound.TokenCount > 2 {
                    // params supplied:
                    argString := crushEvalTokens(inbound.Tokens[2:]).text
                    argString = stripOuter(argString, '(')
                    argString = stripOuter(argString, ')')

                    if len(argString)>0 {
                        dargs = str.Split(argString, ",")
                        for arg:=range dargs {
                            dargs[arg]=str.Trim(dargs[arg]," \t")
                        }
                    }
                } else {
                    /*
                    if inbound.TokenCount != 2 {
                        parser.report(  "Braced list of parameters not supplied!")
                        finish(true, ERR_SYNTAX)
                        break
                    }
                    */
                }

                defining = true
                definitionName = fn

                // error if it clashes with a stdlib name
                exMatchStdlib:=false
                for n,_:=range slhelp {
                    if n==definitionName {
                        parser.report("A library function already exists with the name '"+definitionName+"'")
                        finish(false,ERR_SYNTAX)
                        exMatchStdlib=true
                        break
                    }
                }
                if exMatchStdlib { break }

                // error if it has already been user defined
                if _, exists := fnlookup.lmget(definitionName); exists {
                    parser.report(  "Function "+definitionName+" already exists.")
                    finish(false, ERR_SYNTAX)
                    break
                }

                // debug(20,"[#3]DEFINE taking a space[#-]\n")
                loc, _ := GetNextFnSpace(definitionName)

                fspacelock.Lock()
                functionspaces[loc] = []Phrase{}
                fspacelock.Unlock()

                farglock.Lock()
                functionArgs[loc] = dargs
                farglock.Unlock()
            }

        case C_Showdef:

            if inbound.TokenCount == 2 {
                fn := stripOuterQuotes(inbound.Tokens[1].tokText, 2)
                if _, exists := fnlookup.lmget(fn); exists {
                    ShowDef(fn)
                } else {
                    parser.report( "Function not found.")
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

            /*
                this is kind of invalid now we don't pass (...) as a single expression token.
                we could still do with a bounds check here of some form, but this is not it...

            if inbound.TokenCount == 2 {
                if hasOuterBraces(inbound.Tokens[1].tokText) {
                    if inbound.Tokens[1].tokType == Expression {
                        parser.report( "Cannot brace a RETURN value.")
                        finish(true, ERR_SYNTAX)
                        break
                    }
                }
            }

            */


            if inbound.TokenCount != 1 {

                // @todo: this should still work, but needs some updating to allow for full tokenisation
                // needs to use Eval() or wrappedEval() instead of ev and process tokens instead of splitting
                // strings all over the place.

                cet := crushEvalTokens(inbound.Tokens[1:])
                if str.Trim(cet.text, " \t") != "" { // found something

                    // tco goes here...
                    //  this only deals with calls to same function we can do other options
                    //  later. i think only difference would be recalculating the ifs+base 
                    //  args from call. may be some other changes needed too.

                    // if tokens had thisFunc in calls, *and*
                    // no other tokens except the call params
                    // then....

                    tco_check:=false // disable until we check all is well
                    bname, _ := numlookup.lmget(base)

                    if inbound.TokenCount > 2 {
                        // 0:RETURN 1:fn/var_name 2+:(expression)
                        // if only a var_name, then inbound.TokenCount must be 2

                        // pf("This func is  -> <%v>\n",fs)
                        // pf("Base func is  -> <%v>\n",bname)
                        // pf("Call token is -> <%v>\n",inbound.Tokens[1].tokText)

                        if strcmp(bname,inbound.Tokens[1].tokText) {
                            tco_check=true
                        }
                    }

                    if tco_check {

                        skip_reentry:=false

                        r,_:=userDefEval(parser,ifs,inbound.Tokens[1:])

                        // until we have more logic in here, also skip if
                        // there's *anything* left in the expression after
                        // the initial func call
                        cet := crushEvalTokens(r[2:])
                        if cet.text != "" { skip_reentry=true }

                        // this will be re-enabled when we do more complex checking:

                        // check no more calls with same func name:
                        // if str.Contains(cet.text,bname) {
                        //     skip_reentry=true
                        // }

                        // now pick through r, setting each va in turn...

                        if !skip_reentry {

                            // strip paren
                            ex := stripOuter(r[1].tokText,'(')
                            ex  = stripOuter(ex, ')')

                            // split by comma
                            var dargs []string

                            if len(ex)>0 {
                                dargs = str.Split(ex, ",")
                                for arg:=range dargs {
                                    dargs[arg]=str.Trim(dargs[arg]," \t")
                                }
                            } else {
                                skip_reentry=true // no args
                            }

                            // repopulate va with expression results from the return expressions

                            full_break:=false

                            if len(va) == len(dargs) {

                                for q, _ := range va {
                                    expr, err := ev(parser,ifs, dargs[q], false, true)
                                    if expr==nil || err != nil {
                                        parser.report("Could not evaluate RETURN expression")
                                        finish(true,ERR_EVAL)
                                        full_break=true
                                        break
                                    }
                                    va[q]=expr
                                }

                            } else {
                                skip_reentry=true
                            }

                            if full_break { break }

                        }

                        if !skip_reentry {
                            vset(ifs,"@in_tco",true)
                            pc=-1
                            goto tco_reentry
                        }

                    }

                    // normal return (non tco)

                    expr := wrappedEval(parser,ifs, inbound.Tokens[1:], true) // evaluate it
                    if !expr.evalError {
                        retval = expr.result
                        if ifs<=2 {
                            if exitCode,not_ok:=GetAsInt(expr.result); not_ok {
                                parser.report( sf("could not evaluate RETURN parameter: %+v\n%+v", cet.text,expr.errVal))
                                finish(true, ERR_EVAL)
                                break
                            } else {
                                finish(true,exitCode)
                                break
                            }
                        }
                    } else {
                        parser.report(  sf("could not evaluate RETURN parameter: %+v", cet.text))
                        finish(true, ERR_EVAL)
                        break
                    }
                }

            }
            endFunc = true


        case C_Enddef:

            if !defining {
                parser.report(  "Not currently defining a function.")
                finish(false, ERR_SYNTAX)
                break
            }

            defining = false
            definitionName = ""


        case C_Input:

            // INPUT <id> <type> <position>                    - set variable {id} from external value or exits.

            // get C_Input arguments

            if inbound.TokenCount != 4 {
                usage  :=         "INPUT [#i1]id[#i0] PARAM | OPTARG [#i1]field_position[#i0]\n"
                usage   = usage + "INPUT [#i1]id[#i0] ENV [#i1]env_name[#i0]"
                parser.report( "Incorrect arguments supplied to INPUT.\n"+usage)
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
                        parser.report( sf("INPUT position %d too low.",d))
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
                        parser.report( sf("Expected CLI parameter '%s' not provided at startup.", id))
                        finish(true, ERR_SYNTAX)
                    }
                } else {
                    parser.report( sf("That '%s' doesn't look like a number.", pos))
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
                    parser.report( sf("That '%s' doesn't look like a number.", pos))
                    finish(false, ERR_SYNTAX)
                }

            case "env":
                if os.Getenv(pos)!="" {
                    // non-empty env var so set id var to value.
                    vset(ifs, id, os.Getenv(pos))
                } else {
                    // when env var empty either create the id var or
                    // leave it alone if it already exists.
                    if _, found := VarLookup(ifs,id); !found {
                        vset(ifs,id,"")
                    }
                }
            }


        case C_Module:
            // MODULE <modname>                                - reads in state from a module file.

            var expr ExpressionCarton

            if inbound.TokenCount > 1 {
                expr = wrappedEval(parser,ifs, inbound.Tokens[1:], true)
                if expr.evalError {
                    parser.report( sf("could not evaluate expression in MODULE statement\n%+v",expr.errVal))
                    finish(false,ERR_MODULE)
                    break
                }
            } else {
                parser.report( "No module name provided.")
                finish(false, ERR_MODULE)
                break
            }

            fom := expr.result.(string)

            if strcmp(fom,"") {
                parser.report(  "Empty module name provided.")
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
                parser.report( sf("Module is not accessible. (path:%v)",moduleloc))
                finish(false, ERR_MODULE)
                break
            }

            if !f.Mode().IsRegular() {
                parser.report(  "Module is not a regular file.")
                finish(false, ERR_MODULE)
                break
            }

            //.. read in file

            mod, err := ioutil.ReadFile(moduleloc)
            if err != nil {
                parser.report(  "Problem reading the module file.")
                finish(false, ERR_MODULE)
                break
            }

            // tokenise and parse into a new function space.

            //.. error if it has already been defined
            if _, exists := fnlookup.lmget("@mod_"+fom); exists {
                parser.report(  "Function @mod_"+fom+" already exists.")
                finish(false, ERR_SYNTAX)
                break
            }

            // debug(20,"[#3]MODULE taking a space[#-]\n")
            loc, _ := GetNextFnSpace("@mod_"+fom)

            calllock.Lock()

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
            modcs.callline = pc
            calltable[loc] = modcs

            calllock.Unlock()

            Call(MODE_NEW, loc, ciMod)

            calllock.Lock()
            calltable[loc]=call_s{}
            calllock.Unlock()


            // purge the module source as the code has been executed
            fspacelock.Lock()
            functionspaces[loc]=[]Phrase{}
            fspacelock.Unlock()


        case C_When:

            // need to store the condition and result for the is/contains/in/or clauses
            // endwhen location should be calculated in advance for a direct jump to exit
            // we need to calculate it anyway for nesting
            // after the above setup, we execute next source line as normal

            if inbound.TokenCount==1 {
                parser.report( "Missing expression in WHEN statement")
                finish(false,ERR_SYNTAX)
                break
            }

            // lookahead
            endfound, enddistance, er := lookahead(base, pc, 0, 0, C_Endwhen, []uint8{C_When}, []uint8{C_Endwhen})

            // debug(6,"@%d : Endwhen lookahead set to line %d\n",pc+1,pc+1+enddistance)

            if er {
                parser.report(  "Lookahead error!")
                finish(true, ERR_SYNTAX)
                break
            }

            if !endfound {
                parser.report(  "Missing ENDWHEN for this WHEN")
                finish(false, ERR_SYNTAX)
                break
            }

            expr := wrappedEval(parser,ifs, inbound.Tokens[1:], true)
            if expr.evalError {
                parser.report( sf("could not evaluate the WHEN condition\n%+v",expr.errVal))
                finish(false, ERR_EVAL)
                break
            }

            // create a whenCarton and increase the nesting level

            if lockSafety { lastlock.Lock() }
            if lockSafety { looplock.Lock() }

            wccount[ifs]++
            wc[wccount[ifs]] = whenCarton{endLine: pc + enddistance, value: expr.result, dodefault: true}
            depth[ifs]++
            lastConstruct[ifs] = append(lastConstruct[ifs], C_When)

            if lockSafety { looplock.Unlock() }
            if lockSafety { lastlock.Unlock() }

        case C_Is, C_Contains, C_Or:

            if lockSafety { lastlock.RLock() }
            if lockSafety { looplock.RLock() }

            if depth[ifs] == 0 || (depth[ifs] > 0 && lastConstruct[ifs][depth[ifs]-1] != C_When) {
                parser.report( "Not currently in a WHEN block.")
                finish(false,ERR_SYNTAX)
                if lockSafety { looplock.RUnlock() }
                if lockSafety { lastlock.RUnlock() }
                break
            }

            carton := wc[wccount[ifs]]

            if lockSafety { looplock.RUnlock() }
            if lockSafety { lastlock.RUnlock() }

            // var cet, expr ExpressionCarton
            var expr ExpressionCarton

            if inbound.TokenCount > 1 { // inbound.TokenCount==1 for C_Or
                expr = wrappedEval(parser,ifs, inbound.Tokens[1:], true)
                if expr.evalError {
                    parser.report( sf("could not evaluate expression in WHEN condition\n%+v",expr.errVal))
                    finish(false, ERR_EVAL)
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

                if !carton.dodefault {
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
                isfound, isdistance, _ := lookahead(base, pc+1, 0, 0, C_Is, []uint8{C_When}, []uint8{C_Endwhen})
                orfound, ordistance, _ := lookahead(base, pc+1, 0, 0, C_Or, []uint8{C_When}, []uint8{C_Endwhen})
                cofound, codistance, _ := lookahead(base, pc+1, 0, 0, C_Contains, []uint8{C_When}, []uint8{C_Endwhen})

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

            if lockSafety { looplock.Lock() }
            if lockSafety { lastlock.Lock() }

            if depth[ifs] == 0 || (depth[ifs] > 0 && lastConstruct[ifs][depth[ifs]-1] != C_When) {
                parser.report( "Not currently in a WHEN block.")
                if lockSafety { lastlock.Unlock() }
                if lockSafety { looplock.Unlock() }
                break
            }

            breakIn = Error

            lastConstruct[ifs] = lastConstruct[ifs][:depth[ifs]-1]
            depth[ifs]--
            wccount[ifs]--

            if wccount[ifs] < 0 {
                parser.report("Cannot reduce WHEN stack below zero.")
                finish(false, ERR_SYNTAX)
            }

            if lockSafety { lastlock.Unlock() }
            if lockSafety { looplock.Unlock() }


        case C_Struct:

            // STRUCT name
            // start structmode
            // consume identifiers sequentially, adding each to definition.
            // Format:
            // STRUCT name
            // a type; b type;
            // c type;
            // d type; e type;
            // ...
            // ENDSTRUCT

            if structMode {
                parser.report("Cannot nest a STRUCT")
                finish(false,ERR_SYNTAX)
                break
            }

            if inbound.TokenCount!=2 {
                parser.report("STRUCT must contain a name.")
                finish(false,ERR_SYNTAX)
                break
            }

            structName=inbound.Tokens[1].tokText
            structMode=true
            // pf("Building struct %v\n",structName)

        case C_Endstruct:

            // ENDSTRUCT
            // end structmode

            if ! structMode {
                parser.report("ENDSTRUCT without STRUCT.")
                finish(false,ERR_SYNTAX)
                break
            }

            // 
            // take definition and create a structmaps entry from it:
            structmaps[structName]=structNode[:]
            // pf("Completing struct %v\n",structName)
            // pf("structNode -> %v\n",structNode)
            //

            structName=""
            structNode=[]string{}
            structMode=false


        case C_Showstruct:

            // SHOWSTRUCT [filter]

            var filter string

            if inbound.TokenCount>1 {
                cet := crushEvalTokens(inbound.Tokens[1:])
                filter,_ = interpolate(ifs,cet.text,true)
            }

            for k,s:=range structmaps {

                if matched, _ := regexp.MatchString(filter, k); !matched { continue }

                pf("[#6]%v[#-]\n",k)

                for i:=0; i<len(s); i+=2 {
                    pf("[#4]%24v[#-] [#3]%v[#-]\n",s[i],s[i+1])
                }
                pf("\n")

            }


        case C_With:
            // WITH var AS file
            // get params

            if inbound.TokenCount < 4 {
                parser.report("Malformed WITH statement.")
                finish(false, ERR_SYNTAX)
                break
            }

            asAt := findDelim(inbound.Tokens, "as", 2)
            if asAt == -1 {
                parser.report("AS not found in WITH")
                finish(false, ERR_SYNTAX)
                break
            }

            vname:=crushEvalTokens(inbound.Tokens[1:asAt]).text
            fname:=crushEvalTokens(inbound.Tokens[asAt+1:]).text

            if fname=="" || vname=="" {
                parser.report("Bad arguments to provided to WITH.")
                finish(false,ERR_SYNTAX)
                break
            }

            if _, found := VarLookup(ifs,vname); !found {
                parser.report(sf("Variable '%s' does not exist.",vname))
                finish(false,ERR_EVAL)
                break
            }

            tfile, err:= ioutil.TempFile("","za_with_"+sf("%d",os.Getpid())+"_")
            if err!=nil {
                parser.report("WITH could not create a temporary file.")
                finish(true,ERR_SYNTAX)
                break
            }
            content,_:=vget(ifs,vname)
		    ioutil.WriteFile(tfile.Name(), []byte(content.(string)), 0600)
            vset(ifs,fname,tfile.Name())
            inside_with=true
            current_with_handle=tfile

            defer func() {
                remfile:=current_with_handle.Name()
                current_with_handle.Close()
                current_with_handle=nil
                err:=os.Remove(remfile)
                if err!=nil {
                    parser.report(sf("WITH could not remove temporary file '%s'",remfile))
                    finish(true,ERR_FATAL)
                }
            }()

        case C_Endwith:
            if !inside_with {
                parser.report("ENDWITH without a WITH.")
                finish(false,ERR_SYNTAX)
                break
            }

            inside_with=false


        // parsing for these is a mess, will clean up when new evaluator stable.
        // i think we only need to worry about parens when scanning for commas
        // as string expressions should be single string literal tokens.
        case C_Print:
            if inbound.TokenCount > 1 {
                evphrase:=""
                evnest:=0
                for term := range inbound.Tokens[1:] {
                    nt:=inbound.Tokens[1+term]
                    if nt.tokType==LParen { evnest++ }
                    if nt.tokType==RParen { evnest-- }
                    if nt.tokType!=C_Comma {
                        evphrase+=nt.tokText
                    } else {
                        if evnest>0 { evphrase+=nt.tokText }
                    }
                    if evnest==0 && (term==len(inbound.Tokens[1:])-1 || nt.tokType == C_Comma) {
                        v,_:=ev(parser,ifs,evphrase,true,false)
                        pf(sparkle(sf(`%v`,v)))
                        evphrase=""
                        continue
                    }
                    // should do something about evnest>0 here, but all this will
                    // be cleansed eventually.
                }
                if interactive { pf("\n") }
            } else {
                pf("\n")
            }


        case C_Println:
            if inbound.TokenCount > 1 {
                evphrase:=""
                evnest:=0
                for term := range inbound.Tokens[1:] {
                    nt:=inbound.Tokens[1+term]
                    if nt.tokType==LParen { evnest++ }
                    if nt.tokType==RParen { evnest-- }
                    if evnest>0 || nt.tokType!=C_Comma {
                        evphrase+=nt.tokText
                    } else {
                        if evnest>0 { evphrase+=nt.tokText }
                    }
                    if evnest==0 && (term==len(inbound.Tokens[1:])-1 || nt.tokType == C_Comma) {
                        v,_:=ev(parser,ifs,evphrase,true,false)
                        pf( sf("%v",v) ) // sparkle( sf("%v",v) ) )
                        // pf(sparkle(sf(`%v`,v)))
                        evphrase=""
                        continue
                    }
                }
                pf("\n")
            } else {
                pf("\n")
            }


        case C_Log:

            plog_out := ""
            if inbound.TokenCount > 1 {
                evphrase:=""
                evnest:=0
                for term := range inbound.Tokens[1:] {
                    nt:=inbound.Tokens[1+term]
                    if nt.tokType==LParen { evnest++ }
                    if nt.tokType==RParen { evnest-- }
                    if nt.tokType!=C_Comma {
                        evphrase+=nt.tokText
                    } else {
                        if evnest>0 { evphrase+=nt.tokText }
                    }
                    if evnest==0 && (term==len(inbound.Tokens[1:])-1 || nt.tokType == C_Comma) {
                        v,_:=ev(parser,ifs,evphrase,true,false)
                        plog_out += sparkle(sf(`%v`,v))
                        evphrase=""
                        continue
                    }
                }
            }
            plog("%v", plog_out)


        case C_Hist:

            for h, v := range hist {
                pf("%5d : %s\n", h, v)
            }

        case C_At:

            // AT row ',' column

            commaAt := findDelim(inbound.Tokens, ",", 1)

            if commaAt == -1 || commaAt == inbound.TokenCount {
                parser.report(  "Bad delimiter in AT.")
                finish(false, ERR_SYNTAX)
            } else {

                evrow := crushEvalTokens(inbound.Tokens[1:commaAt])
                evcol := crushEvalTokens(inbound.Tokens[commaAt+1:])

                expr_row, err := ev(parser,ifs, evrow.text, false,true)
                if expr_row==nil || err != nil {
                    parser.report( sf("Evaluation error in %v", expr_row))
                }

                expr_col, err := ev(parser,ifs, evcol.text, false,true)
                if expr_col==nil || err != nil {
                    parser.report(  sf("Evaluation error in %v", expr_col))
                }

                row, _ = GetAsInt(expr_row)
                col, _ = GetAsInt(expr_col)
                at(row, col)

            }


        case C_Prompt:

            // else continue

            if inbound.TokenCount < 2 {
                usage := "PROMPT [#i1]storage_variable prompt_string[#i0] [ [#i1]validator_regex[#i0] ]"
                parser.report(  "Not enough arguments for PROMPT.\n"+usage)
                finish(false, ERR_SYNTAX)
                break
            }

            // prompt variable assignment:
            if inbound.TokenCount > 1 { // um, should not do this but...
                if inbound.Tokens[1].tokType == C_Assign {
                    expr := wrappedEval(parser,ifs, inbound.Tokens[2:], true)
                    if expr.evalError {
                        parser.report( sf("could not evaluate expression prompt assignment\n%+v",expr.errVal))
                        finish(false, ERR_EVAL)
                        break
                    }
                    switch expr.result.(type) {
                    case string:
                        promptTemplate = sparkle(expr.result.(string))
                    }
                } else {
                    // prompt command:
                    if inbound.TokenCount < 3 || inbound.TokenCount > 4 {
                        parser.report( "Incorrect arguments for PROMPT command.")
                        finish(false, ERR_SYNTAX)
                        break
                    } else {
                        validator := ""
                        broken := false
                        expr, prompt_ev_err := ev(parser,ifs, inbound.Tokens[2].tokText, true, true)
                        if expr==nil {
                            parser.report( "Could not evaluate in PROMPT command.")
                            finish(false,ERR_EVAL)
                            break
                        }
                        if prompt_ev_err == nil {
                            // @todo: allow an expression instead of the string literal for validator
                            processedPrompt := expr.(string)
                            echoMask,_:=vget(0,"@echomask")
                            if inbound.TokenCount == 4 {
                                val_ex,val_ex_error := ev(parser,ifs, inbound.Tokens[3].tokText, true, true)
                                if val_ex_error != nil {
                                    parser.report("Validator invalid in PROMPT!")
                                    finish(false,ERR_EVAL)
                                    break
                                }
                                validator = val_ex.(string)
                                intext := ""
                                validated := false
                                for !validated || broken {
                                    intext, _, broken = getInput(ifs,processedPrompt, currentpane, row, col, promptColour, false, false, echoMask.(string))
                                    validated, _ = regexp.MatchString(validator, intext)
                                }
                                if !broken {
                                    vset(ifs, inbound.Tokens[1].tokText, intext)
                                }
                            } else {
                                var inp string
                                inp, _, broken = getInput(ifs,processedPrompt, currentpane, row, col, promptColour, false, false, echoMask.(string))
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

            if inbound.TokenCount < 2 || inbound.TokenCount > 3 {
                parser.report(  "LOGGING command malformed.")
                finish(false, ERR_SYNTAX)
                break
            }

            switch str.ToLower(inbound.Tokens[1].tokText) {

            case "off":
                loggingEnabled = false

            case "on":
                loggingEnabled = true
                if inbound.TokenCount == 3 {
                    expr := wrappedEval(parser,ifs, inbound.Tokens[2:], false)
                    if expr.evalError {
                        parser.report( sf("could not evaluate destination filename in LOGGING ON statement\n%+v",expr.errVal))
                        finish(false, ERR_EVAL)
                        break
                    }
                    logFile = expr.result.(string)
                    vset(0, "@logsubject", "")
                }

            case "quiet":
                vset(globalspace, "@silentlog", true)

            case "loud":
                vset(globalspace, "@silentlog", false)

            case "accessfile":
                if inbound.TokenCount > 2 {
                    expr := wrappedEval(parser,ifs, inbound.Tokens[2:], true)
                    if expr.evalError {
                        parser.report( sf("could not evaluate filename in LOGGING ACCESSFILE statement\n%+v",expr.errVal))
                        finish(false, ERR_EVAL)
                        break
                    }
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
                    parser.report( "No access file provided for LOGGING ACCESSFILE command.")
                    finish(false, ERR_SYNTAX)
                }

            case "web":
                if inbound.TokenCount > 2 {
                    switch str.ToLower(inbound.Tokens[2].tokText) {
                    case "on","1","enable":
                        log_web=true
                    case "off","0","disable":
                        log_web=false
                    default:
                        parser.report( "Invalid state set for LOGGING WEB.")
                        finish(false, ERR_EVAL)
                    }
                } else {
                    parser.report( "No state provided for LOGGING WEB command.")
                    finish(false, ERR_SYNTAX)
                }

            case "subject":
                if inbound.TokenCount == 3 {
                    expr := wrappedEval(parser,ifs, inbound.Tokens[2:], false)
                    if expr.evalError {
                        parser.report( sf("could not evaluate logging subject in LOGGING SUBJECT statement\n%+v",expr.errVal))
                        finish(false, ERR_EVAL)
                        break
                    }
                    vset(0, "@logsubject", expr.result.(string))
                } else {
                    vset(0, "@logsubject", "")
                }

            default:
                parser.report( "LOGGING command malformed.")
                finish(false, ERR_SYNTAX)
            }


        case C_Cls:

            if inbound.TokenCount == 1 {
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

            if inbound.TokenCount == 2 {
                if inbound.Tokens[1].tokType == Identifier {
                    vset(ifs, inbound.Tokens[1].tokText, 0)
                } else {
                    parser.report(  "Not an identifier.")
                    finish(false, ERR_SYNTAX)
                }
            } else {
                parser.report(  "Missing identifier to reset.")
                finish(false, ERR_SYNTAX)
            }


        case C_Inc,C_Dec:

            var id string

            if inbound.TokenCount > 1 {

                if inbound.Tokens[1].tokType == Identifier {
                    id = inbound.Tokens[1].tokText
                } else {
                    parser.report(  "Not an identifier.")
                    finish(false, ERR_SYNTAX)
                    break
                }

                // var ampl int
                // var er bool
                var endIncDec bool
                var isArray bool

                switch inbound.TokenCount {
                case 2:
                    ampl = 1
                default:
                    // is a var?
                    v,ok:=vget(ifs,inbound.Tokens[2].tokText)
                    if ok {
                        switch v:=v.(type) {
                        case uint8,int32,int64,uint32,uint64:
                            ampl,_ = GetAsInt(v)
                        case int:
                            ampl = v
                        default:
                            parser.report( sf("%s only works with integer types. (not this: %T)",str.ToUpper(inbound.Tokens[0].tokText),v))
                            finish(false,ERR_EVAL)
                            endIncDec=true
                            break
                        }
                    } else { // is an int?
                        var er bool
                        ampl,er = GetAsInt(inbound.Tokens[2].tokText)
                        if er { // else evaluate

                            expr := wrappedEval(parser,ifs, inbound.Tokens[2:], false)
                            typ:="increment"
                            if statement.tokType==C_Dec { typ="decrement" }
                            if expr.evalError {
                                parser.report( sf("could not evaluate amplitude in %v statement\n%+v",typ,expr.errVal))
                                finish(false, ERR_EVAL)
                                break
                            }

                            switch expr.result.(type) {
                            case int:
                                ampl = expr.result.(int)
                            default:
                                parser.report( sf("%s does not result in an integer type.",str.ToUpper(inbound.Tokens[0].tokText)))
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
                        val, found = vgetElement(ifs,id[:sqPos],elementComponents)
                        isArray = true
                    } else {
                        val, found = vget(ifs, id)
                    }

                    var ival int
                    if found {
                        switch val.(type) {
                        case int:
                            ival=val.(int)
                        case uint64:
                            ival=int(val.(uint64))
                        case int32:
                            ival=int(val.(int32))
                        case int64:
                            ival=int(val.(int64))
                        case uint8:
                            ival=int(val.(uint8))
                        default:
                            parser.report( sf("%s only works with integer types. (*not this: %T with id:%v)",str.ToUpper(inbound.Tokens[0].tokText),val,id))
                            finish(false,ERR_EVAL)
                            endIncDec=true
                        }
                    }


                    // if not found then will init with 0+ampl
                    if !endIncDec {
                        switch statement.tokType {
                        case C_Inc:
                            if isArray {
                                vsetElement(ifs,id[:sqPos],elementComponents,ival+ampl)
                            } else {
                                if lockSafety {
                                    vset(ifs, id, ival+ampl)
                                } else {
                                    vid, _ = VarLookup(ifs,id)
                                    ident[ifs][vid].IValue = ival+ampl
                                }
                            }
                        case C_Dec:
                            if isArray {
                                vsetElement(ifs,id[:sqPos],elementComponents,ival-ampl)
                            } else {
                                if lockSafety {
                                    vset(ifs, id, ival-ampl)
                                } else {
                                    vid, _ = VarLookup(ifs,id)
                                    ident[ifs][vid].IValue = ival-ampl
                                }
                            }
                        }
                    }
                }
            } else {
                typ:="increment"
                if statement.tokType==C_Dec { typ="decrement" }
                parser.report( "Missing identifier in "+typ+" statement.")
                finish(false, ERR_SYNTAX)
            }


        case C_If:

            // lookahead
            elsefound, elsedistance, er := lookahead(base, pc, 0, 1, C_Else, []uint8{C_If}, []uint8{C_Endif})
            endfound, enddistance, er := lookahead(base, pc, 0, 0, C_Endif, []uint8{C_If}, []uint8{C_Endif})

            if er || !endfound {
                parser.report(  "Missing ENDIF for this IF")
                finish(false, ERR_SYNTAX)
                break
            }

            // eval
            expr, err := EvalCrushRest(parser,ifs, inbound.Tokens, 1)
            if err!=nil {
                parser.report(  "Could not evaluate expression.")
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

            endfound, enddistance, _ := lookahead(base, pc, 1, 0, C_Endif, []uint8{C_If}, []uint8{C_Endif})

            if endfound {
                pc += enddistance
            } else { // this shouldn't ever occur, as endif checked during C_If, but...
                parser.report( "ELSE without an ENDIF\n")
                finish(false, ERR_SYNTAX)
            }


        case C_Endif:

            // ENDIF *should* just be an end-of-block marker


        default:

            // local command assignment (child process call)

            if inbound.TokenCount > 1 { // ident "=|"
                if statement.tokType == Identifier && inbound.Tokens[1].tokType == C_AssCommand {
                    if len(inbound.Text) > 0 {
                        // get text after =|
                        startPos := str.IndexByte(inbound.Original, '|') + 1
                        cmd,_ := interpolate(ifs, inbound.Original[startPos:],true)
                        out:=system(cmd,false)
                        lhs_name,_ := interpolate(ifs, statement.tokText,true)
                        vset(ifs, lhs_name, out)
                    }
                    // skip normal eval below
                    break
                }
            }

            //
            //
            // try to eval and assign

            if we:=wrappedEval(parser,ifs, inbound.Tokens, true); we.evalError {
                parser.report(sf("Error in evaluation\n%+v\n",we.errVal))
                finish(false,ERR_EVAL)
                break
            }

            //
            //
            //

        } // end-statements-case

    } // end-pc-loop


    siglock.RLock()
    si=sig_int
    siglock.RUnlock()

    if structMode {
        // incomplete struct definition
        pf("Open STRUCT definition %v\n",structName)
        finish(true,ERR_SYNTAX)
    }

    if !si {

        // populate return variable in the caller with retvals

        if retval!=nil {
            vset(caller, retvar, retval)
        }

        // clean up

        // pf("Leaving call with ifs of %d [fs:%s]\n\n",ifs,fs)

        // pf("[#2]about to delete %v[#-]\n",fs)
        if lockSafety { calllock.Lock() }

        calltable[ifs]=call_s{}
        fnlookup.lmdelete(fs)
        numlookup.lmdelete(ifs)
        // pf("call disposing of ifs : %d\n",ifs)

        looplock.Lock()
        depth[ifs]=0
        loops[ifs]=nil
        looplock.Unlock()

        fspacelock.Lock()
        if ifs>2 { functionspaces[ifs] = []Phrase{} }
        fspacelock.Unlock()

        if lockSafety { calllock.Unlock() }

    }

    callChain=callChain[:len(callChain)-1]

    return endFunc

}

func system(cmd string, display bool) (string) {
    cmd = str.Trim(cmd," \t")
    if hasOuter(cmd,'`') {
        cmd=stripOuter(cmd,'`')
    }
    out, _ := Copper(cmd, false)
    if display { pf("%s",out) }
    return out
}

/// execute a command in the shell coprocess or parent
func coprocCall(ifs uint64,s string) {
    cet := ""
    if len(s) > 0 {
        // find index of first pipe, then remove everything upto and including it
        pipepos := str.IndexByte(s, '|')
        /*
        if pipepos==-1 {
            pf("syntax error in '%s'\n",s)
            // @todo: handle this type of exit more gracefully, no rush, should be uncommon.
            os.Exit(0)
        }
        */
        cet = s[pipepos+1:]
        inter,_ := interpolate(ifs,cet,true)
        out, ec := Copper(inter, false)
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
            if first {
                first = false
                strOut = sf("\n%s(%v)\n\t\t ", fn, str.Join(functionArgs[ifn], ","))
            }
            pf("%s%s\n", strOut, functionspaces[ifn][q].Original)
        }
    }
    return true
}

/// search token list for a given delimiter token type
func findTokenDelim(tokens []Token, delim uint8, start int) (pos int) {
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


