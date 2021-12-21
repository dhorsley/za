package main

import (
    "io/ioutil"
    "math"
    "log"
    "encoding/gob"
    "os"
    "path/filepath"
    "path"
    "reflect"
    "regexp"
    "sync"
    "sync/atomic"
    "strconv"
    "runtime"
    "fmt"
    str "strings"
    "time"
    "unsafe"
)


func task(caller uint32, base uint32, endClose bool, call string, iargs ...interface{}) (chan interface{},string) {

    r:=make(chan interface{})

    loc,id := GetNextFnSpace(true,call+"@",call_s{prepared:true,base:base,caller:caller})
    // fmt.Printf("***** [task]  loc#%d caller#%d, recv cstab: %+v\n",loc,caller,calltable[loc])

    go func() {
        if endClose { defer close(r) }
        var ident [szIdent]Variable
        atomic.AddInt32(&concurrent_funcs,1)
        rcount,_:=Call(MODE_NEW, &ident, loc, ciAsyn, iargs...)

        switch rcount {
        case 0:
            r<-struct{l uint32;r interface{}}{loc,nil}
        case 1:
            calllock.RLock()
            v:=calltable[loc].retvals
            calllock.RUnlock()
            if v==nil {
                r<-nil
                break
            }
            // pf("[#3]TASK RESULT : loc %d : val (%+v)[#-]\n",loc,v.([]interface{}))
            r<-struct{l uint32;r interface{}}{loc,v.([]interface{})[0]}
        default:
            calllock.RLock()
            v:=calltable[loc].retvals
            calllock.RUnlock()
            r<-struct{l uint32;r interface{}}{loc,v}
        }

        atomic.AddInt32(&concurrent_funcs,-1)

    }()
    return r,id
}


var testlock = &sync.RWMutex{}
var atlock = &sync.RWMutex{}

// finish : flag the machine state as okay or in error and
// optionally terminates execution.
func finish(hard bool, i int) {
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


// slightly faster string comparison.
// have to use gotos here as loops can't be inlined
func strcmp(a string, b string) (bool) {
    la:=len(a)
    if la!=len(b)   { return false }
    if la==0        { return true }
    strcmp_repeat_point:
        la -= 1
        if a[la]!=b[la] { return false }
    if la>0 { goto strcmp_repeat_point }
    return true
}

func strcmpFrom1(a string, b string) (bool) {
    la:=len(a)
    if la!=len(b)   { return false }
    // if la==0        { return true }
    strcmp_repeat_point:
        la -= 1
        if a[la]!=b[la] { return false }
    if la>1 { goto strcmp_repeat_point }
    return true
}

// GetAsFloat : converts a variety of types to a float
func GetAsFloat(unk interface{}) (float64, bool) {
    switch i := unk.(type) {
    case int:
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
    case float64:
        return int64(i), false
    case uint:
        return int64(i), false
    case int:
        return int64(i), false
    case int64:
        return i, false
    case uint64:
        return int64(i), false
    case uint32:
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


func GetAsInt(expr interface{}) (int, bool) {
    switch i := expr.(type) {
    case float64:
        return int(i), false
    case uint:
        return int(i), false
    case int64:
        return int(i), false
    case uint64:
        return int(i), false
    case uint32:
        return int(i), false
    case uint8:
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
    }
    return 0, true
}

func GetAsUint(expr interface{}) (uint, bool) {
    switch i := expr.(type) {
    case float64:
        return uint(i), false
    case int:
        return uint(i), false
    case int64:
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


// check for value in slice - used by lookahead()
// @note: not inlined!
func InSlice(a uint8, list []uint8) bool {
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

// searchToken is used by FOR to check for occurrences of the loop variable.
func searchToken(source_base uint32, start int16, end int16, sval string) bool {

    range_fs:=functionspaces[source_base][start:end]

    for _, v := range range_fs {
        if v.TokenCount == 0 {
            continue
        }
        for r := 0; r < len(v.Tokens); r+=1 {
            if v.Tokens[r].tokType == Identifier && v.Tokens[r].tokText == sval {
                return true
            }
            // check for direct reference
            if str.Contains(v.Tokens[r].tokText, sval) {
                return true
            }
            // on *any* indirect reference return true, as we can't be
            // sure without following the interpolation.
            if str.Contains(v.Tokens[r].tokText,"{{") {
                return true
            }
        }
    }
    return false
}


// lookahead used by if..else..endif and similar constructs for nesting
//  @note: lookahead only returns _,_,true when over dedented.
func lookahead(fs uint32, startLine int16, indent int, endlevel int, term uint8, indenters []uint8, dedenters []uint8) (bool, int16, bool) {

    range_fs:=functionspaces[fs][startLine:]

    for i, v := range range_fs {

        if len(v.Tokens) == 0 {
            continue
        }

        // indents and dedents
        if InSlice(v.Tokens[0].tokType, indenters) {
            indent+=1
        }
        if InSlice(v.Tokens[0].tokType, dedenters) {
            indent--
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
    return false, -1, false

}


// find the next available slot for a function or module
//  definition in the functionspace[] list.
func GetNextFnSpace(do_lock bool, requiredName string, cs call_s) (uint32,string) {

    // do_lock not currently used!

    // fmt.Printf("Entered gnfs\n")
    calllock.Lock()

    // : sets up a re-use value
    var reuse,e uint32
    if globseq<gcModulus*2 || (globseq % gcModulus) < 2 {
        for e=0; e<globseq; e+=1 {
            if calltable[e].gc {
                if calltable[e].gcShyness>0 { calltable[e].gcShyness-=1 }
                if calltable[e].gcShyness==0 { break }
            }
        }
        if e<globseq { reuse=e }
    }

    // find a reservation
    for ; numlookup.lmexists(globseq) ; { // reserved
        globseq=(globseq+1) % gnfsModulus
        if globseq==0 { globseq=1 }
    }

    // resize calltable if needed
    for ; globseq>=uint32(cap(calltable)) ; {
        if cap(calltable)>=gnfsModulus {
            fmt.Printf("call table overgrown\n")
            finish(true, ERR_FATAL)
            calllock.Unlock()
            return 0,""
        }
        ncs:=make([]call_s,len(calltable)*2,cap(calltable)*2)
        copy(ncs,calltable)
        calltable=ncs
        fmt.Printf("[gnfs] resized calltable.\n")
    }

    // generate new tagged instance name
    newName := requiredName
    if newName[len(newName)-1]=='@' {
        newName+=strconv.FormatUint(uint64(globseq), 10)
    }

    // allocate
    if reuse==0 { reuse=globseq } // else { fmt.Printf("** reusing fs %d\n",reuse) }
    numlookup.lmset(reuse, newName)
    fnlookup.lmset(newName,reuse)
    if cs.prepared==true {
        cs.fs=newName
        calltable[reuse]=cs
        // fmt.Printf("[gnfs] populated call table entry # %d with: %+v\n",reuse,calltable[globseq]) 
    }

    // fmt.Printf("(gnf) allocated for %v with %d\n",newName,reuse)

    calllock.Unlock()
    return reuse,newName

}

// setup mutex locks
var calllock   = &sync.RWMutex{}  // function call related
var lastlock   = &sync.RWMutex{}  // cached globals
var farglock   = &sync.RWMutex{}  // function args manipulation
var fspacelock = &sync.RWMutex{}  // token storage related
var globlock   = &sync.RWMutex{}  // generic global related


// for error reporting : keeps a list of parent->child function calls
//   will probably blow up during recursion.

var callChain []chainInfo

var symbolised = make(map[uint32]bool)

// defined function entry point
// everything about what is to be executed is contained in calltable[csloc]
func Call(varmode uint8, ident *[szIdent]Variable, csloc uint32, registrant uint8, va ...interface{}) (retval_count uint8,endFunc bool) {

    // register call
    calllock.Lock()

    /*
    pf("\n[#1]Entered call (csloc#%d) -> %#v[#-]\n",csloc,calltable[csloc])
    pf(" with caller of  -> %v\n",calltable[csloc].caller)
    pf(" with new ifs of -> %v fs-> %v\n",csloc,calltable[csloc].fs)
    */

    caller_str,_:=numlookup.lmget(calltable[csloc].caller)
    callChain=append(callChain,chainInfo{loc:calltable[csloc].caller,name:caller_str,registrant:registrant})

    // set up evaluation parser - one per function
    parser:=&leparser{}
    parser.prectable=default_prectable
    parser.ident=ident
    calllock.Unlock()

    lastlock.Lock()
    interparse.ident=ident
    if interactive {
        parser.mident=1
        interparse.mident=1
    } else {
        parser.mident=2
        interparse.mident=2
    }
    lastlock.Unlock()

    var inbound *Phrase
    var basecode_entry *BaseCode
    var current_with_handle *os.File

    // error handler
    defer func() {
        if r := recover(); r != nil {
            if _,ok:=r.(runtime.Error); ok {
                parser.report(inbound.SourceLine,sf("\n%v\n",r))
                if debug_level>0 { err:=r.(error); panic(err) }
                finish(false,ERR_EVAL)
            }
            err:=r.(error)
            parser.report(inbound.SourceLine,sf("\n%v\n",err))
            if debug_level>0 { panic(r) }
            setEcho(true)
            finish(false,ERR_EVAL)
        }
    }()

    // some tracking variables for this function call
    var break_count int             // usually 0. when >0 stops breakIn from resetting
                                    //  used for multi-level breaks.
    var breakIn uint8               // true during transition from break to outer.
    var forceEnd bool               // used by BREAK for skipping context checks when
                                    //  bailing from nested constructs.
    var retvalues []interface{}     // return values to be passed back
    var finalline int16             // tracks end of tokens in the function
    var fs string                   // current function space
    var source_base uint32          // location of the translated source tokens
    var thisLoop *s_loop            // pointer to loop information. used in FOR

    // set up the function space

    // ..get call details
    calllock.RLock()

    // unique name for this execution, pre-generated before call
    fs = calltable[csloc].fs

    // the source code to be read for this function
    /*
    fmt.Printf("[call] csloc: %d\n",csloc)
    fmt.Printf("[call] cstab: %+v\n",calltable[csloc])
    */

    source_base = calltable[csloc].base

    // the uint32 id attached to fs name
    ifs,_:=fnlookup.lmget(fs)

    calllock.RUnlock()

    // pf("CALL ENTRY BINDINGS - #%d\n",ifs)
    // pf("%+v\n",bindings[ifs])

    // -- generate bindings

    var locked bool
    if atomic.LoadInt32(&concurrent_funcs)>0 { locked=true ; lastlock.Lock() }
    if !symbolised[source_base] && source_base>1 {
        bindlock.Lock()
        bindings[source_base]=make(map[string]uint64)
        bindlock.Unlock()
        symbolised[source_base]=true
    }
    if locked { lastlock.Unlock() }

    bindlock.Lock()
    if bindings[ifs]==nil && ifs>1 {
        // pf("-- RESETTING BINDINGS FOR IFS %d\n",ifs)
        bindings[ifs]=make(map[string]uint64)
    }

    for k,v:=range bindings[source_base] {
        bindings[ifs][k]=v
        // pf("[call #ifs %d] BASE COPYING FROM #%d : bound %s to id %d\n",ifs,source_base,k,bindings[ifs][k])
    }
    /*
     fmt.Printf("Just copied symbols for ifs #%d\n",ifs)
     fmt.Printf("%+v\n",bindings[ifs])
    */
    bindlock.Unlock()


    if varmode==MODE_NEW {
        // create the local variable storage for the function

        testlock.Lock()
        test_group = ""
        test_name = ""
        test_assert = ""
        testlock.Unlock()

    }

    // missing varargs in call result in nil assignments back to caller:
    farglock.Lock()
    /*
    fmt.Printf("[call-fa] ifs#%d source_base -> %+v\n",ifs,source_base)
    fmt.Printf("[call-fa] ifs#%d fargs       -> %+v\n",ifs,functionArgs[source_base].args)
    */
    if len(functionArgs[source_base].args)>len(va) {
        for e:=0; e<(len(functionArgs[source_base].args)-len(va)); e+=1 {
            va=append(va,nil)
        }
    }
    farglock.Unlock()

    // generic nesting indentation counters
    // this being local prevents re-entrance i guess
    var depth int

    // stores the active construct/loop types outer->inner
    //  for the break and continue statements
    var lastConstruct = []uint8{}

    /*
    if varmode == MODE_NEW {
        // in_tco: currently in an iterative tail-call.
        in_tco=false
        // vset(ifs,ident, "@in_tco",false)
    }
    */

    // initialise condition states: WHEN stack depth
    // initialise the loop positions: FOR, FOREACH, WHILE

    // active WHEN..ENDWHEN statement meta info
    var wc = make([]whenCarton, WHEN_CAP)

    // count of active WHEN..ENDWHEN statements
    var wccount int

    // counters per loop type
    var loops = make([]s_loop, MAX_LOOPS)

tco_reentry:

    // assign value to local vars named in functionArgs (the call parameters)
    //  from each va value.
    // - functionArgs[] created at definition time from the call signature

    farglock.RLock()
    if len(va) > 0 {
        for q, v := range va {
            if q>=len(functionArgs[source_base].args) { break }
            fa:=functionArgs[source_base].args[q]
            /*
            pf("-- setting va-to-var (fargs-sb) : %+v\n",functionArgs[source_base])
            pf("-- setting va-to-var in ifs#%d source_base: %d\n",csloc,source_base)
            pf("-- setting va-to-var in ifs#%d ifs        : %d\n",csloc,ifs)
            pf("-- setting va-to-var in ifs#%d variable   : %s\n",csloc,fa)
            pf("-- setting va-to-var in ifs#%d with val   : %+v\n",csloc,v)
            */
            vset(ifs,ident,fa,v)
        }
    }
    farglock.RUnlock()

    if len(functionspaces[source_base])>32767 {
        parser.report(-1,"function too long!")
        finish(true,ERR_SYNTAX)
        return
    }

    finalline = int16(len(functionspaces[source_base]))

    inside_test := false      // are we currently inside a test bock
    inside_with := false      // WITH cannot be nested and remains local in scope.

    var structMode bool       // are we currently defining a struct
    var structName string     // name of struct currently being defined
    var structNode []interface{}   // struct builder
    var defining bool         // are we currently defining a function. takes priority over structmode.
    var definitionName string // ... if we are, what is it called

    parser.pc = -1            // program counter : increments to zero at start of loop

    var si bool
    var we ExpressionCarton   // pre-allocated for general expression results eval
    var expr interface{}      // pre-llocated for wrapped expression results eval
    var err error

    typeInvalid:=false          // used during struct building for indicating type validity.

    for {

        // *sigh*
        //  turns out, pc++ is much slower than pc = pc + 1, at least on x64 test build

        //  pprof didn't capture this reliably (probably to do with sample rate)
        //  however, if you check against eg/long_loop or eg/addition_loop over a sufficiently
        //  large repetition count you will see there is a big difference in performance.

        parser.pc+=1

        // @note: sig_int can be a race condition. alternatives?
        // if sig_int removed from below then user ctrl-c handler cannot
        // return a custom error code (unless it exits instead of returning maybe)
        // also, having this cond check every iteration slows down execution.

        if parser.pc >= finalline || endFunc || sig_int {
            break
        }

        // get the next Phrase
        inbound = &functionspaces[source_base][parser.pc]

                    //
     ondo_reenter:  // on..do re-enters here because it creates the new phrase in advance and
                    //  we want to leave the program counter unaffected.


        /////////////////////////////////////////////////////////////////////////

        // finally... start processing the statement.


        /////// LINE ////////////////////////////////////////////////////////////
               // pf("(%20s) (line:%5d) [#b7][#2]%5d : %+v[##][#-]\n",fs,inbound.SourceLine,parser.pc,inbound.Tokens)
        /////////////////////////////////////////////////////////////////////////

        // append statements to a function if currently inside a DEFINE block.
        if defining && inbound.Tokens[0].tokType != C_Enddef {
            lmv,_:=fnlookup.lmget(definitionName)
            fspacelock.Lock()
            functionspaces[lmv] = append(functionspaces[lmv], *inbound)
            basecode_entry      = &basecode[source_base][parser.pc]
            basecode[lmv]       = append(basecode[lmv], *basecode_entry)
            fspacelock.Unlock()
            continue
        }

        // struct building
        if structMode && inbound.Tokens[0].tokType!=C_Endstruct {
            // consume the statement as an identifier
            // as we are only accepting simple types currently, restrict validity
            //  to single type token.
            if inbound.TokenCount<2 {
                parser.report(inbound.SourceLine,sf("Invalid STRUCT entry '%v'",inbound.Tokens[0].tokText))
                finish(false,ERR_SYNTAX)
                break
            }

            // check for default value assignment:
            var eqPos int16
            var hasValue bool
            for eqPos=2;eqPos<inbound.TokenCount;eqPos+=1 {
                if inbound.Tokens[eqPos].tokType==O_Assign {
                    hasValue=true
                    break
                }
            }

            var default_value ExpressionCarton
            if hasValue {
                default_value = parser.wrappedEval(ifs,ident,ifs,ident,inbound.Tokens[eqPos+1:])
                // pf(" : set default_value in hasValue ( %#v )\n",default_value)
                if default_value.evalError {
                    parser.report(inbound.SourceLine,sf("Invalid default value in STRUCT '%s'",inbound.Tokens[0].tokText))
                    finish(false,ERR_SYNTAX)
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
            switch str.ToLower(cet.text) {
            case "int","float","string","bool","uint","uint8","byte","mixed","any","[]":
            default:
                parser.report(inbound.SourceLine,sf("Invalid type in STRUCT '%s'",cet.text))
                finish(false,ERR_SYNTAX)
                typeInvalid=true
                break
            }

            if typeInvalid {
                break
            }

            structNode=append(structNode,inbound.Tokens[0].tokText,cet.text,hasValue,default_value.result)
            // pf("current struct node build at :\n%#v\n",structNode)

            continue
        }

        // show var references for -V arg
        if var_refs {
            switch inbound.Tokens[0].tokType {
            case C_Module,C_Define,C_Enddef:
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
            if inbound.Tokens[0].tokType != C_Endtest && !under_test {
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

        if breakIn != Error {
            // breakIn holds either Error or a token_type for ending the current construct
            if inbound.Tokens[0].tokType != breakIn {
                continue
            }
        }
        ////////////////////////////////////////////////////////////////


        // main parsing for statements starts here:

        switch inbound.Tokens[0].tokType {

        case C_Var: // permit declaration with a default value

            // expand to this:
            // 'VAR' name1 [ ',' ... nameX ] [ '[' [size] ']' ] type [ '=' expr ]
            //  and var ary_s []struct_name

            var name_list []string
            var expectingComma bool
            var varSyntaxError bool
            var c int16

          var_comma_loop:
            for c=int16(1); c<inbound.TokenCount; c+=1 {
                switch inbound.Tokens[c].tokType {
                case Identifier:
                    if expectingComma { // syntax error
                        break var_comma_loop
                    }
                    name_list=append(name_list,inbound.Tokens[c].tokText)
                case O_Comma:
                    if !expectingComma { // syntax error
                        varSyntaxError=true
                        break var_comma_loop
                    }
                default:
                    break var_comma_loop
                }
                expectingComma=!expectingComma
            }

            if len(name_list)==0 {
                varSyntaxError=true
            }

            // set eqpos to either location of first equals sign
            // or zero, as well as bool to indicate success
            var eqPos int16
            var hasEqu bool
            for eqPos=c; eqPos<inbound.TokenCount; eqPos+=1 {
                if inbound.Tokens[eqPos].tokType == O_Assign {
                    hasEqu=true
                    break
                }
            }
            // eqPos remains as last token index on natural loop exit

            // pf("hasEqu? %v\n",hasEqu)
            // pf("eqpos @ %d\n",eqPos)

            // look for ary setup

            var hasAry bool
            var size int

            if !varSyntaxError {
                // continue from last 'c' value
                if inbound.Tokens[c].tokType==LeftSBrace {

                    // find RightSBrace
                    var d int16
                    for d=eqPos-1; d>c; d-- {
                        if inbound.Tokens[d].tokType==RightSBrace {
                            hasAry=true
                            break
                        }
                    }
                    if hasAry && d>(c+1) {
                        // not an empty [] term, but includes a size expression
                        se := parser.wrappedEval(ifs,ident,ifs,ident,inbound.Tokens[c+1:d])
                        if se.evalError {
                            parser.report(inbound.SourceLine,"could not evaluate size expression in VAR")
                            finish(false,ERR_EVAL)
                            break
                        }
                        switch se.result.(type) {
                        case int:
                            size=se.result.(int)
                        case int32:
                            size=int(se.result.(int32))
                        case int64:
                            size=int(se.result.(int64))
                        case uint:
                            size=int(se.result.(uint))
                        case uint64:
                            size=int(se.result.(uint64))
                        default:
                            parser.report(inbound.SourceLine,"size expression must evaluate to an integer")
                            finish(false,ERR_EVAL)
                            break
                        }
                    }
                }

                // pf("hasAry?  %v\n",hasAry)
                // pf("size is  %d\n",size)

            } else {
                parser.report(inbound.SourceLine,"invalid VAR syntax\nUsage: VAR varname1 [#i1][,...varnameX][#i0] [#i1][optional_size][#i0] type [#i1][=expression][#i0]")
                finish(false,ERR_SYNTAX)
            }

            if varSyntaxError {
                break
            }

            // eval the terms to assign to new vars
            hasValue := false
            if hasEqu {
                hasValue=true
                we = parser.wrappedEval(ifs,ident,ifs,ident,inbound.Tokens[eqPos+1:])
                if we.evalError {
                    parser.report(inbound.SourceLine,"could not evaluate VAR assignment expression")
                    finish(false,ERR_EVAL)
                    break
                }
            }

                // this needs reworking:
                //   if we pulled them out to global scope then will cause parallelism problems,
                //   but could probably just stick a lock around this process given it's
                //   expected infrequency. (or find a better way to initialise types dynamically)

                // ++
                var tb bool
                var tu8 uint8
                var tu uint
                var ti int
                var tf64 float64
                var ts string

                var stb     []bool
                var stu     []uint
                var stu8    []uint8
                var sti     []int
                var stf64   []float64
                var sts     []string
                var stmixed []interface{}

                // *sigh* - really need to move this stuff out of here:
                gob.Register(stb)
                gob.Register(stu)
                gob.Register(stu8)
                gob.Register(sti)
                gob.Register(stf64)
                gob.Register(stmixed)

                // instantiate fields with an empty expected type:
                typemap:=make(map[string]reflect.Type)
                typemap["bool"]     = reflect.TypeOf(tb)
                typemap["uint"]     = reflect.TypeOf(tu)
                typemap["uint8"]    = reflect.TypeOf(tu8)
                typemap["byte"]     = reflect.TypeOf(tu8)
                typemap["int"]      = reflect.TypeOf(ti)
                typemap["float"]    = reflect.TypeOf(tf64)
                typemap["string"]   = reflect.TypeOf(ts)
                typemap["[]bool"]   = reflect.TypeOf(stb)
                typemap["[]uint"]   = reflect.TypeOf(stu)
                typemap["[]uint8"]  = reflect.TypeOf(stu8)
                typemap["[]byte"]   = reflect.TypeOf(stu8)
                typemap["[]int"]    = reflect.TypeOf(sti)
                typemap["[]float"]  = reflect.TypeOf(stf64)
                typemap["[]string"] = reflect.TypeOf(sts)
                typemap["[]interface {}"] = reflect.TypeOf(stmixed)
                typemap["[]"]       = reflect.TypeOf(stmixed)
                typemap["assoc"]    = nil
                // --

            // name iterations

            for _,vname:=range name_list {

                sid:=bind_int(ifs,vname) // sid_list[k]

                // get the required type
                var new_type_token_string string
                type_token_string := inbound.Tokens[eqPos-1].tokText

                // @note: this only allows for []any not just any (as an interface{})
                //   will revise all this when i get my head around generics in Go 1.18

                if type_token_string=="]" || type_token_string=="mixed" || type_token_string=="any" {
                    type_token_string="[]"
                }
                if type_token_string!="[]" {
                    new_type_token_string = type_token_string
                }
                if hasAry {
                    if type_token_string!="[]" {
                        new_type_token_string="[]"+type_token_string
                    } else {
                        new_type_token_string="[]"
                    }
                }

                // declaration and initialisation
                if _,found:=typemap[new_type_token_string]; found {

                    t:=Variable{}

                    if new_type_token_string!="assoc" {
                        t.IValue = reflect.New(typemap[new_type_token_string]).Elem().Interface()
                    }

                    vlock.Lock()

                    t.IName=vname
                    t.ITyped=true
                    t.declared=true

                    switch new_type_token_string {
                    case "nil":
                        t.IKind=knil
                    case "bool":
                        t.IKind=kbool
                    case "int":
                        t.IKind=kint
                    case "uint":
                        t.IKind=kuint
                    case "float":
                        t.IKind=kfloat
                    case "string":
                        t.IKind=kstring
                    case "uint8","byte":
                        t.IKind=kbyte
                    case "[]bool":
                        t.IKind=ksbool
                        t.IValue=make([]bool,size,size)
                    case "[]int":
                        t.IKind=ksint
                        t.IValue=make([]int,size,size)
                    case "[]uint":
                        t.IKind=ksuint
                        t.IValue=make([]uint,size,size)
                    case "[]float":
                        t.IKind=ksfloat
                        t.IValue=make([]float64,size,size)
                    case "[]string":
                        t.IKind=ksstring
                        t.IValue=make([]string,size,size)
                    case "[]byte","[]uint8":
                        t.IKind=ksbyte
                        t.IValue=make([]uint8,size,size)
                    case "[]","[]mixed","[]any":
                        t.IKind=ksany
                        t.IValue=make([]interface{},size,size)
                    case "assoc":
                        t.IKind=kmap
                        t.IValue=make(map[string]interface{},size)
                        gob.Register(t.IValue)
                    }


                    // if we had a default value, stuff it in here...
                    if new_type_token_string!="assoc" && hasValue {
                        if sf("%T",we.result)!=new_type_token_string {
                            parser.report(inbound.SourceLine,"type mismatch in VAR assignment")
                            finish(false,ERR_EVAL)
                            vlock.Unlock()
                            break
                        } else {
                            t.IValue=we.result
                        }
                    }

                    // write temp to ident
                    // @note: have to write all to retain the ITyped flag!
                    (*ident)[sid]=t
                    vlock.Unlock()

                } else {
                    // unknown type: check if it is a struct name

                    isStruct:=false
                    structvalues:=[]interface{}{}

                    // structmap has list of field_name,field_type,... for each struct
                    for sn, snv := range structmaps {
                        if sn==type_token_string {
                            isStruct=true
                            structvalues=snv
                            break
                        }
                    }

                    if isStruct {
                        vlock.Lock()

                        // holding temp var
                        t:=(*ident)[sid]

                        // deal with var name [n]struct_type
                        if len(structvalues)>0 {
                            var sfields []reflect.StructField
                            offset:=uintptr(0)
                            for svpos:=0; svpos<len(structvalues); svpos+=4 {
                                // structvalues: [0] name [1] type [2] boolhasdefault [3] default_value
                                nv :=structvalues[svpos].(string)
                                nt :=structvalues[svpos+1].(string)
                                if nt=="any" || nt=="mixed" { nt="[]" }
                                sfields=append(sfields,
                                    reflect.StructField{
                                        Name:nv,PkgPath:"main",
                                        Type:typemap[nt],
                                        Offset:offset,
                                        Anonymous:false,
                                    },
                                )
                                offset+=typemap[nt].Size()
                            }
                            new_struct:=reflect.StructOf(sfields)
                            v:=(reflect.New(new_struct).Elem()).Interface()

                            t.IName=vname
                            t.ITyped=false
                            t.declared=true

                            if !hasAry {
                                // default values setting:

                                val:=reflect.ValueOf(v)
                                tmp:=reflect.New(val.Type()).Elem()
                                tmp.Set(val)

                                allSet:=true

                                for svpos:=0; svpos<len(structvalues); svpos+=4 {
                                    // structvalues: [0] name [1] type [2] boolhasdefault [3] default_value
                                    nv :=structvalues[svpos].(string)
                                    nhd:=structvalues[svpos+2].(bool)
                                    ndv:=structvalues[svpos+3]
                                    if nhd {
                                        var intyp reflect.Type
                                        if ndv!=nil { intyp=reflect.ValueOf(ndv).Type() }

                                        tf:=tmp.FieldByName(nv)
                                        if intyp.AssignableTo(tf.Type()) {
                                            tf=reflect.NewAt(tf.Type(),unsafe.Pointer(tf.UnsafeAddr())).Elem()
                                            tf.Set(reflect.ValueOf(ndv))
                                        } else {
                                            parser.report(inbound.SourceLine,sf("cannot set field default (%T) for %v (%v)",ndv,nv,tf.Type()))
                                            finish(false,ERR_EVAL)
                                            allSet=false
                                            break
                                        }
                                    }
                                }

                                if allSet {
                                    t.IValue=tmp.Interface()
                                }

                            } else {
                                t.IValue=[]interface{}{}
                            }

                        } // end-len>0

                        // write temp to ident
                        (*ident)[sid]=t

                        vlock.Unlock()

                    } else {
                        parser.report(inbound.SourceLine,sf("unknown data type requested '%v'",type_token_string))
                        finish(false, ERR_SYNTAX)
                        break
                    }

                } // end-type-or-struct

            } // end-of-name-list


        case C_While:

            var endfound bool
            var enddistance int16

            endfound, enddistance, _ = lookahead(source_base, parser.pc, 0, 0, C_Endwhile, []uint8{C_While}, []uint8{C_Endwhile})
            // pf("(while debug) -> on line %v : end_found %+v : distance %+v\n",inbound.SourceLine,endfound,enddistance)
            if !endfound {
                parser.report(inbound.SourceLine,"could not find an ENDWHILE")
                finish(false, ERR_SYNTAX)
                break
            }

            // if cond false, then jump to end while
            // if true, stack the cond then continue

            // eval

            var res bool
            var etoks []Token

            if inbound.TokenCount==1 {
                etoks=[]Token{Token{tokType:Identifier,tokText:"true",subtype:subtypeConst,tokVal:true}}
                res=true
            } else {

                etoks=inbound.Tokens[1:]

                we = parser.wrappedEval(ifs,ident,ifs,ident,etoks)
                if we.evalError {
                    parser.report(inbound.SourceLine,"could not evaluate WHILE condition")
                    finish(false,ERR_EVAL)
                    break
                }

                switch we.result.(type) {
                case bool:
                    res = we.result.(bool)
                default:
                    parser.report(inbound.SourceLine,"WHILE condition must evaluate to boolean")
                    finish(false,ERR_EVAL)
                    break
                }

            }

            if isBool(res) && res {
                // while cond is true, stack, then continue loop
                depth+=1
                loops[depth] = s_loop{repeatFrom: parser.pc, whileContinueAt: parser.pc + enddistance, repeatCond: etoks, loopType: C_While}
                lastConstruct = append(lastConstruct, C_While)
                break
            } else {
                // -> endwhile
                parser.pc += enddistance
            }


        case C_Endwhile:

            // re-evaluate, on true jump back to start, on false, destack and continue

            cond := loops[depth]

            if !forceEnd && cond.loopType != C_While {
                parser.report(inbound.SourceLine,"ENDWHILE outside of WHILE loop")
                finish(false, ERR_SYNTAX)
                break
            }

            // time to die?
            if breakIn == C_Endwhile {
                depth-=1
                lastConstruct = lastConstruct[:depth]
                breakIn = Error
                forceEnd=false
                break_count-=1
                if break_count>0 {
                    switch lastConstruct[depth-1] {
                    case C_For,C_Foreach:
                        breakIn=C_Endfor
                    case C_While:
                        breakIn=C_Endwhile
                    case C_When:
                        breakIn=C_Endwhen
                    }
                }
                // pf("ENDWHILE-BREAK: bc %d\n",break_count)
                break
            }

            // evaluate condition
            we = parser.wrappedEval(ifs,ident,ifs,ident,cond.repeatCond)
            if we.evalError {
                parser.report(inbound.SourceLine,sf("eval fault in ENDWHILE\n%+v\n",we.errVal))
                finish(false,ERR_EVAL)
                break
            }

            if we.result.(bool) {
                // while still true, loop
                parser.pc = cond.repeatFrom
            } else {
                // was false, so leave the loop
                depth-=1
                lastConstruct = lastConstruct[:depth]
            }


        case C_SetGlob: // set the value of a global variable.

           if inbound.TokenCount<3 {
                parser.report(inbound.SourceLine,"missing value in setglob.")
                finish(false,ERR_SYNTAX)
                break
            }

            if res:=parser.wrappedEval(parser.mident,&mident,ifs,ident,inbound.Tokens[1:]); res.evalError {
                parser.report(inbound.SourceLine,sf("Error in SETGLOB evaluation\n%+v\n",res.errVal))
                finish(false,ERR_EVAL)
                break
            }


        case C_Foreach:

            // FOREACH var IN expr
            // iterates over the result of expression expr as a list

            if inbound.TokenCount<4 {
                parser.report(inbound.SourceLine,"bad argument length in FOREACH.")
                finish(false,ERR_SYNTAX)
                break
            }

            if ! str.EqualFold(inbound.Tokens[2].tokText,"in") {
                parser.report(inbound.SourceLine,"malformed FOREACH statement.")
                finish(false, ERR_SYNTAX)
                break
            }

            if inbound.Tokens[1].tokType != Identifier {
                parser.report(inbound.SourceLine,"parameter 2 must be an identifier.")
                finish(false, ERR_SYNTAX)
                break
            }

            var condEndPos int

            fid := inbound.Tokens[1].tokText

            switch inbound.Tokens[3].tokType {

            // cause evaluation of all terms following IN
            case SYM_BOR, O_InFile, NumericLiteral, StringLiteral, LeftSBrace, LParen, Identifier:

                we = parser.wrappedEval(ifs,ident,ifs,ident,inbound.Tokens[3:])
                if we.evalError {
                    parser.report(inbound.SourceLine,sf("error evaluating term in FOREACH statement '%v'\n%+v\n",we.text,we.errVal))
                    finish(false,ERR_EVAL)
                    break
                }
                var l int
                switch lv:=we.result.(type) {
                case string:
                    l=len(lv)
                case []string:
                    l=len(lv)
                case []uint:
                    l=len(lv)
                case []int:
                    l=len(lv)
                case []float64:
                    l=len(lv)
                case []bool:
                    l=len(lv)
                case []dirent:
                    l=len(lv)
                case []alloc_info:
                    l=len(lv)
                case map[string]alloc_info:
                    l=len(lv)
                case map[string]string:
                    l=len(lv)
                case map[string]uint:
                    l=len(lv)
                case map[string]int:
                    l=len(lv)
                case map[string]float64:
                    l=len(lv)
                case map[string]bool:
                    l=len(lv)
                case map[string][]string:
                    l=len(lv)
                case map[string][]uint:
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
                case [][]int:
                    l=len(lv)
                case []interface{}:
                    l=len(lv)
                default:
                    pf("Unknown loop type [%T]\n",lv)
                }

                if l==0 {
                    // skip empty expressions
                    endfound, enddistance, _ := lookahead(source_base, parser.pc, 0, 0, C_Endfor, []uint8{C_For,C_Foreach}, []uint8{C_Endfor})
                    if !endfound {
                        parser.report(inbound.SourceLine,"Cannot determine the location of a matching ENDFOR.")
                        finish(false, ERR_SYNTAX)
                        break
                    } else { //skip
                        parser.pc += enddistance
                        break
                    }
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
                    if runtime.GOOS!="windows" {
                        we.result = str.Split(we.result.(string), "\n")
                    } else {
                        we.result = str.Split(str.Replace(we.result.(string), "\r\n", "\n", -1), "\n")
                    }

                    if len(we.result.([]string))>0 {
                        vset(ifs, ident,"key_"+fid, 0)
                        vset(ifs, ident,fid, we.result.([]string)[0])
                        condEndPos = len(we.result.([]string)) - 1
                    }

                case map[string]float64:
                    if len(we.result.(map[string]float64)) > 0 {
                        // get iterator for this map
                        iter = reflect.ValueOf(we.result.(map[string]float64)).MapRange()

                        // set initial key and value
                        if iter.Next() {
                            vset(ifs, ident,"key_"+fid, iter.Key().String())
                            vset(ifs, ident, fid, iter.Value().Interface())
                        }
                        condEndPos = len(we.result.(map[string]float64)) - 1
                    }

                case map[string]alloc_info:
                    if len(we.result.(map[string]alloc_info)) > 0 {
                        iter = reflect.ValueOf(we.result.(map[string]alloc_info)).MapRange()
                        if iter.Next() {
                            vset(ifs, ident,"key_"+fid, iter.Key().String())
                            vset(ifs, ident, fid, iter.Value().Interface())
                        }
                        condEndPos = len(we.result.(map[string]alloc_info)) - 1
                    }

                case map[string]bool:
                    if len(we.result.(map[string]bool)) > 0 {
                        iter = reflect.ValueOf(we.result.(map[string]bool)).MapRange()
                        if iter.Next() {
                            vset(ifs, ident,"key_"+fid, iter.Key().String())
                            vset(ifs, ident, fid, iter.Value().Interface())
                        }
                        condEndPos = len(we.result.(map[string]bool)) - 1
                    }

                case map[string]uint:
                    if len(we.result.(map[string]uint)) > 0 {
                        iter = reflect.ValueOf(we.result.(map[string]uint)).MapRange()
                        if iter.Next() {
                            vset(ifs, ident,"key_"+fid, iter.Key().String())
                            vset(ifs, ident, fid, iter.Value().Interface())
                        }
                        condEndPos = len(we.result.(map[string]uint)) - 1
                    }

                case map[string]int:
                    if len(we.result.(map[string]int)) > 0 {
                        // get iterator for this map
                        iter = reflect.ValueOf(we.result.(map[string]int)).MapRange()

                        // set initial key and value
                        if iter.Next() {
                            vset(ifs, ident,"key_"+fid, iter.Key().String())
                            vset(ifs, ident, fid, iter.Value().Interface())
                        }
                        condEndPos = len(we.result.(map[string]int)) - 1
                    }

                case map[string]string:

                    if len(we.result.(map[string]string)) > 0 {

                        // get iterator for this map
                        iter = reflect.ValueOf(we.result.(map[string]string)).MapRange()
                        // set initial key and value
                        if iter.Next() {
                            vset(ifs, ident,"key_"+fid, iter.Key().String())
                            vset(ifs, ident, fid, iter.Value().Interface())
                        }
                        condEndPos = len(we.result.(map[string]string)) - 1
                    }

                case map[string][]string:

                    if len(we.result.(map[string][]string)) > 0 {

                        // get iterator for this map
                        iter = reflect.ValueOf(we.result.(map[string][]string)).MapRange()

                        // set initial key and value
                        if iter.Next() {
                            vset(ifs, ident,"key_"+fid, iter.Key().String())
                            vset(ifs, ident, fid, iter.Value().Interface())
                        }
                        condEndPos = len(we.result.(map[string][]string)) - 1
                    }

                case []float64:

                    if len(we.result.([]float64)) > 0 {
                        vset(ifs, ident,"key_"+fid, 0)
                        vset(ifs, ident, fid, we.result.([]float64)[0])
                        condEndPos = len(we.result.([]float64)) - 1
                    }

                case float64: // special case: float
                    we.result = []float64{we.result.(float64)}
                    if len(we.result.([]float64)) > 0 {
                        vset(ifs, ident,"key_"+fid, 0)
                        vset(ifs, ident, fid, we.result.([]float64)[0])
                        condEndPos = len(we.result.([]float64)) - 1
                    }

                case []uint:
                    if len(we.result.([]uint)) > 0 {
                        vset(ifs, ident,"key_"+fid, 0)
                        vset(ifs, ident, fid, we.result.([]uint)[0])
                        condEndPos = len(we.result.([]uint)) - 1
                    }

                case []bool:
                    if len(we.result.([]bool)) > 0 {
                        vset(ifs, ident,"key_"+fid, 0)
                        vset(ifs, ident, fid, we.result.([]bool)[0])
                        condEndPos = len(we.result.([]bool)) - 1
                    }

                case []int:
                    if len(we.result.([]int)) > 0 {
                        vset(ifs, ident,"key_"+fid, 0)
                        vset(ifs, ident, fid, we.result.([]int)[0])
                        condEndPos = len(we.result.([]int)) - 1
                    }

                case int: // special case: int
                    we.result = []int{we.result.(int)}
                    if len(we.result.([]int)) > 0 {
                        vset(ifs, ident,"key_"+fid, 0)
                        vset(ifs, ident, fid, we.result.([]int)[0])
                        condEndPos = len(we.result.([]int)) - 1
                    }

                case []string:
                    if len(we.result.([]string)) > 0 {
                        vset(ifs, ident,"key_"+fid, 0)
                        vset(ifs, ident, fid, we.result.([]string)[0])
                        condEndPos = len(we.result.([]string)) - 1
                    }

                case []dirent:
                    if len(we.result.([]dirent)) > 0 {
                        vset(ifs, ident,"key_"+fid, 0)
                        vset(ifs, ident, fid, we.result.([]dirent)[0])
                        condEndPos = len(we.result.([]dirent)) - 1
                    }

                case []alloc_info:
                    if len(we.result.([]alloc_info)) > 0 {
                        vset(ifs, ident,"key_"+fid, 0)
                        vset(ifs, ident, fid, we.result.([]alloc_info)[0])
                        condEndPos = len(we.result.([]alloc_info)) - 1
                    }

                case [][]int:
                    if len(we.result.([][]int)) > 0 {
                        vset(ifs, ident,"key_"+fid, 0)
                        vset(ifs, ident, fid, we.result.([][]int)[0])
                        condEndPos = len(we.result.([][]int)) - 1
                    }

                case []map[string]interface{}:

                    if len(we.result.([]map[string]interface{})) > 0 {
                        vset(ifs, ident,"key_"+fid, 0)
                        vset(ifs, ident, fid, we.result.([]map[string]interface{})[0])
                        condEndPos = len(we.result.([]map[string]interface{})) - 1
                    }

                case map[string]interface{}:

                    if len(we.result.(map[string]interface{})) > 0 {

                        // get iterator for this map
                        iter = reflect.ValueOf(we.result.(map[string]interface{})).MapRange()

                        // set initial key and value
                        if iter.Next() {
                            vset(ifs, ident,"key_"+fid, iter.Key().String())
                            vset(ifs, ident, fid, iter.Value().Interface())
                        }
                        condEndPos = len(we.result.(map[string]interface{})) - 1
                    }

                case []interface{}:

                    if len(we.result.([]interface{})) > 0 {
                        vset(ifs, ident,"key_"+fid, 0)
                        vset(ifs, ident, fid, we.result.([]interface{})[0])
                        condEndPos = len(we.result.([]interface{})) - 1
                    }

                default:
                    parser.report(inbound.SourceLine,sf("Mishandled return of type '%T' from FOREACH expression '%v'\n", we.result,we.result))
                    finish(false,ERR_EVAL)
                    break
                }


                // figure end position
                endfound, enddistance, _ := lookahead(source_base, parser.pc, 0, 0, C_Endfor, []uint8{C_For,C_Foreach}, []uint8{C_Endfor})
                if !endfound {
                    parser.report(inbound.SourceLine,"Cannot determine the location of a matching ENDFOR.")
                    finish(false, ERR_SYNTAX)
                    break
                }

                depth+=1
                lastConstruct = append(lastConstruct, C_Foreach)

                loops[depth] = s_loop{loopVar: fid,
                    optNoUse: Opt_LoopStart,
                    repeatFrom: parser.pc + 1, iterOverMap: iter, iterOverArray: we.result,
                    counter: 0, condEnd: condEndPos, forEndPos: enddistance + parser.pc,
                    loopType: C_Foreach,
                }

            }


        case C_For: // loop over an int64 range

            if inbound.TokenCount < 5 || inbound.Tokens[2].tokText != "=" {
                parser.report(inbound.SourceLine,"Malformed FOR statement.")
                finish(false, ERR_SYNTAX)
                break
            }

            toAt := findDelim(inbound.Tokens, C_To, 2)
            if toAt == -1 {
                parser.report(inbound.SourceLine,"TO not found in FOR")
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

            if toAt>3 {
                expr, err = parser.Eval(ifs, inbound.Tokens[3:toAt])
                if err==nil && isNumber(expr) {
                    fstart, _ = GetAsInt(expr)
                } else {
                    parser.report(inbound.SourceLine,"Could not evaluate start expression in FOR")
                    finish(false, ERR_EVAL)
                    break
                }
            } else {
                parser.report(inbound.SourceLine,"Missing expression in FOR statement?")
                finish(false,ERR_SYNTAX)
                break
            }

            if inbound.TokenCount>toAt+1 {
                if stepAt>0 {
                    expr, err = parser.Eval(ifs, inbound.Tokens[toAt+1:stepAt])
                } else {
                    expr, err = parser.Eval(ifs, inbound.Tokens[toAt+1:])
                }
                if err==nil && isNumber(expr) {
                    fend, _ = GetAsInt(expr)
                } else {
                    parser.report(inbound.SourceLine,"Could not evaluate end expression in FOR")
                    finish(false, ERR_EVAL)
                    break
                }
            } else {
                parser.report(inbound.SourceLine,"Missing expression in FOR statement?")
                finish(false,ERR_SYNTAX)
                break
            }

            if stepped {
                if inbound.TokenCount>stepAt+1 {
                    expr, err = parser.Eval(ifs, inbound.Tokens[stepAt+1:])
                    if err==nil && isNumber(expr) {
                        fstep, _ = GetAsInt(expr)
                    } else {
                        parser.report(inbound.SourceLine,"Could not evaluate STEP expression")
                        finish(false, ERR_EVAL)
                        break
                    }
                } else {
                    parser.report(inbound.SourceLine, "Missing expression in FOR statement?")
                    finish(false,ERR_SYNTAX)
                    break
                }
            }

            step := 1
            if stepped {
                step = fstep
            }
            if step == 0 {
                parser.report(inbound.SourceLine,"This is a road to nowhere. (STEP==0)")
                finish(true, ERR_EVAL)
                break
            }

            direction := ACT_INC
            if step < 0 {
                direction = ACT_DEC
            }

            // figure end position
            endfound, enddistance, _ := lookahead(source_base, parser.pc, 0, 0, C_Endfor, []uint8{C_For,C_Foreach}, []uint8{C_Endfor})
            if !endfound {
                parser.report(inbound.SourceLine,"Cannot determine the location of a matching ENDFOR.")
                finish(false, ERR_SYNTAX)
                break
            }

            // @note: if loop counter is never used between here and C_Endfor, then don't vset the local var

            // store loop data
            fid:=inbound.Tokens[1].tokText

            depth+=1
            loops[depth] = s_loop{
                loopVar:  fid,
                optNoUse: Opt_LoopStart,
                loopType: C_For, forEndPos: parser.pc + enddistance, repeatFrom: parser.pc + 1,
                counter: fstart, condEnd: fend,
                repeatAction: direction, repeatActionStep: step,
            }

            // store loop start condition
            vset(ifs, ident, fid, fstart)

            lastConstruct = append(lastConstruct, C_For)

            // make sure start is not more than end, if it is, send it to the endfor
            switch direction {
            case ACT_INC:
                if fstart>fend {
                    parser.pc=parser.pc+enddistance-1
                    break
                }
            case ACT_DEC:
                if fstart<fend {
                    parser.pc=parser.pc+enddistance-1
                    break
                }
            }


        case C_Endfor: // terminate a FOR or FOREACH block

            //.. take address of loop info store entry
            thisLoop = &loops[depth]

            if (*thisLoop).optNoUse == Opt_LoopStart {
                if !forceEnd && lastConstruct[depth-1]!=C_Foreach && lastConstruct[depth-1]!=C_For {
                    parser.report(inbound.SourceLine,"ENDFOR without a FOR or FOREACH")
                    finish(false,ERR_SYNTAX)
                    break
                }
            }

            var loopEnd bool

            // perform cond action and check condition

            if breakIn!=C_Endfor {

                switch (*thisLoop).loopType {

                case C_Foreach: // move through range

                    (*thisLoop).counter+=1

                    // set only on first iteration, keeps optNoUse consistent with C_For
                    if (*thisLoop).optNoUse == Opt_LoopStart {
                        (*thisLoop).optNoUse = Opt_LoopSet
                    }

                    if (*thisLoop).counter > (*thisLoop).condEnd {
                        loopEnd = true
                    } else {

                        // assign value back to local variable

                        switch (*thisLoop).iterOverArray.(type) {

                        // map ranges are randomly ordered!!
                        case map[string]interface{}, map[string]alloc_info, map[string]int, map[string]uint, map[string]bool, map[string]float64, map[string]string, map[string][]string:
                            if (*thisLoop).iterOverMap.Next() { // true means not exhausted
                                vset(ifs, ident,"key_"+(*thisLoop).loopVar, (*thisLoop).iterOverMap.Key().String())
                                vset(ifs, ident, (*thisLoop).loopVar, (*thisLoop).iterOverMap.Value().Interface())
                            }

                        case []bool:
                            vset(ifs, ident,"key_"+(*thisLoop).loopVar, (*thisLoop).counter)
                            vset(ifs, ident, (*thisLoop).loopVar, (*thisLoop).iterOverArray.([]bool)[(*thisLoop).counter])
                        case []int:
                            vset(ifs, ident,"key_"+(*thisLoop).loopVar, (*thisLoop).counter)
                            vset(ifs, ident, (*thisLoop).loopVar, (*thisLoop).iterOverArray.([]int)[(*thisLoop).counter])
                        case []uint:
                            vset(ifs, ident,"key_"+(*thisLoop).loopVar, (*thisLoop).counter)
                            vset(ifs, ident, (*thisLoop).loopVar, (*thisLoop).iterOverArray.([]uint8)[(*thisLoop).counter])
                        case []string:
                            vset(ifs, ident,"key_"+(*thisLoop).loopVar, (*thisLoop).counter)
                            vset(ifs, ident, (*thisLoop).loopVar, (*thisLoop).iterOverArray.([]string)[(*thisLoop).counter])
                        case []dirent:
                            vset(ifs, ident,"key_"+(*thisLoop).loopVar, (*thisLoop).counter)
                            vset(ifs, ident, (*thisLoop).loopVar, (*thisLoop).iterOverArray.([]dirent)[(*thisLoop).counter])
                        case []alloc_info:
                            vset(ifs, ident,"key_"+(*thisLoop).loopVar, (*thisLoop).counter)
                            vset(ifs, ident, (*thisLoop).loopVar, (*thisLoop).iterOverArray.([]alloc_info)[(*thisLoop).counter])
                        case []float64:
                            vset(ifs, ident,"key_"+(*thisLoop).loopVar, (*thisLoop).counter)
                            vset(ifs, ident, (*thisLoop).loopVar, (*thisLoop).iterOverArray.([]float64)[(*thisLoop).counter])
                        case [][]int:
                            vset(ifs, ident,"key_"+(*thisLoop).loopVar, (*thisLoop).counter)
                            vset(ifs, ident, (*thisLoop).loopVar, (*thisLoop).iterOverArray.([][]int)[(*thisLoop).counter])
                        case []map[string]interface{}:
                            vset(ifs, ident,"key_"+(*thisLoop).loopVar, (*thisLoop).counter)
                            vset(ifs, ident, (*thisLoop).loopVar, (*thisLoop).iterOverArray.([]map[string]interface{})[(*thisLoop).counter])
                        case []interface{}:
                            vset(ifs, ident,"key_"+(*thisLoop).loopVar, (*thisLoop).counter)
                            vset(ifs, ident, (*thisLoop).loopVar, (*thisLoop).iterOverArray.([]interface{})[(*thisLoop).counter])
                        default:
                            // @note: should put a proper exit in here.
                            pv,_:=vget(ifs,ident,sf("%v",(*thisLoop).iterOverArray.([]float64)[(*thisLoop).counter]))
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
                                vset(ifs, ident, (*thisLoop).loopVar, (*thisLoop).counter)
                            }
                            loopEnd = true
                        }
                    case ACT_DEC:
                        if (*thisLoop).counter < (*thisLoop).condEnd {
                            (*thisLoop).counter -= (*thisLoop).repeatActionStep
                            if (*thisLoop).optNoUse == Opt_LoopIgnore {
                                vset(ifs, ident, (*thisLoop).loopVar, (*thisLoop).counter)
                            }
                            loopEnd = true
                        }
                    }

                    if (*thisLoop).optNoUse == Opt_LoopStart {
                        (*thisLoop).optNoUse = Opt_LoopIgnore
                        // check tokens once for loop var references, then set Opt_LoopSet if found.
                        if searchToken(source_base, (*thisLoop).repeatFrom, parser.pc, (*thisLoop).loopVar) {
                            (*thisLoop).optNoUse = Opt_LoopSet
                        }
                    }

                    // assign loop counter value back to local variable
                    if (*thisLoop).optNoUse == Opt_LoopSet {
                        // assign directly as already declared and removes the fn call
                        vset(ifs,ident,(*thisLoop).loopVar,(*thisLoop).counter)
                    }

                }

            } else {
                // time to die, mr bond? C_Break reached
                breakIn = Error // reset to unbroken
                forceEnd=false
                loopEnd = true
            }

            // @note: this is bad. should really be a list of break contexts instead of
            //   just a count.

            if loopEnd {
                // leave the loop
                depth-=1
                lastConstruct = lastConstruct[:depth]
                breakIn = Error // reset to unbroken
                forceEnd=false
                if break_count>0 {
                    break_count-=1
                    // doh: testing removal of these:
                    // breakIn=Error
                    // forceEnd=false
                    if break_count>0 {
                        switch lastConstruct[depth-1] {
                        case C_For,C_Foreach:
                            breakIn=C_Endfor
                        case C_While:
                            breakIn=C_Endwhile
                        case C_When:
                            breakIn=C_Endwhen
                        }
                    }
                    // pf("ENDFOR-BREAK: bc %d - new break type is %s\n",break_count,tokNames[breakIn])
                }
            } else {
                // jump back to start of block
                parser.pc = (*thisLoop).repeatFrom - 1 // start of loop will do pc++
            }


        case C_Continue:

            // Continue should work with FOR, FOREACH or WHILE.

            if depth == 0 {
                parser.report(inbound.SourceLine,"Attempting to CONTINUE without a valid surrounding construct.")
                finish(false, ERR_SYNTAX)
            } else {

                // @note:
                //  we use indirect access with thisLoop here (and throughout
                //  loop code) for a minor speed bump. we should periodically
                //  review this as an optimisation in Go could make this unnecessary.

                switch lastConstruct[depth-1] {
                case C_For, C_Foreach:
                    thisLoop = &loops[depth]
                    parser.pc = (*thisLoop).forEndPos - 1

                case C_While:
                    thisLoop = &loops[depth]
                    parser.pc = (*thisLoop).whileContinueAt - 1

                case C_When:
                    // mark this as an error for now, as we don't currently
                    //  backtrack through lastConstruct to check the actual
                    //  loop type so that it can be properly unwound.
                    parser.report(inbound.SourceLine,"Attempting to CONTINUE inside a WHEN is not permitted.")
                    finish(false,ERR_SYNTAX)

                }

            }


        case C_Break:

            // Break should work with either FOR, FOREACH, WHILE or WHEN.

            // We use lastConstruct to establish which is the innermost
            //  of these from which we need to break out.

            // The surrounding construct should set the
            //  lastConstruct[depth] on entry.

            // check for break depth argument

            break_count=0

            if inbound.TokenCount>1 {

                // break by construct type
                if inbound.TokenCount==2 {
                    thisLoop = &loops[depth]
                    lookingForEnd:=false
                    var efound,er bool
                    forceEnd=false
                    switch inbound.Tokens[1].tokType {
                    case C_When:
                        lookingForEnd=true
                        efound,_,er=lookahead(source_base,parser.pc,1,0,C_Endwhen, []uint8{C_When},    []uint8{C_Endwhen})
                        breakIn=C_Endwhen
                        forceEnd=true
                        parser.pc = wc[wccount].endLine - 1
                    case C_For:
                        lookingForEnd=true
                        efound,_,er=lookahead(source_base,parser.pc,1,0,C_Endfor,[]uint8{C_For,C_Foreach},[]uint8{C_Endfor})
                        breakIn=C_Endfor
                        forceEnd=true
                        parser.pc = (*thisLoop).forEndPos - 1
                    case C_Foreach:
                        lookingForEnd=true
                        efound,_,er=lookahead(source_base,parser.pc,1,0,C_Endfor,  []uint8{C_Foreach}, []uint8{C_Endfor})
                        breakIn=C_Endfor
                        forceEnd=true
                        parser.pc = (*thisLoop).forEndPos - 1
                    case C_While:
                        lookingForEnd=true
                        efound,_,er=lookahead(source_base,parser.pc,1,0,C_Endwhile,[]uint8{C_While},   []uint8{C_Endwhile})
                        breakIn=C_Endwhile
                        forceEnd=true
                        parser.pc = (*thisLoop).whileContinueAt - 1
                    }
                    if lookingForEnd {
                        // pf("(debug) efound : %v  er : %v\n",efound,er)
                        if er {
                            // lookahead error
                            parser.report(inbound.SourceLine,sf("BREAK [%s] cannot find end of construct",tokNames[breakIn]))
                            finish(false, ERR_SYNTAX)
                            break
                        }
                        if ! efound {
                            // nesting error
                            parser.report(inbound.SourceLine,sf("BREAK [%s] without surrounding construct",tokNames[breakIn]))
                            finish(false, ERR_SYNTAX)
                            break
                        } else {
                            // break jump point is set, so continue pc loop 
                            // pf("(debug) continuing at statement %d\n",parser.pc+1)
                            continue
                        }

                    }
                }

                // break by expression
                break_depth:=parser.wrappedEval(ifs,ident,ifs,ident,inbound.Tokens[1:])
                switch break_depth.result.(type) {
                case int:
                    break_count=break_depth.result.(int)
                default:
                    parser.report(inbound.SourceLine,"Could not evaluate BREAK depth argument")
                    finish(false,ERR_EVAL)
                    break
                }
            }

            if depth < break_count {
                parser.report(inbound.SourceLine,"Attempting to BREAK without a valid surrounding construct.")
                finish(false, ERR_SYNTAX)
            } else {

                // jump calc, depending on break context

                thisLoop = &loops[depth]

                switch lastConstruct[depth-1] {

                case C_For:
                    parser.pc = (*thisLoop).forEndPos - 1
                    breakIn = C_Endfor

                case C_Foreach:
                    parser.pc = (*thisLoop).forEndPos - 1
                    breakIn = C_Endfor

                case C_While:
                    parser.pc = (*thisLoop).whileContinueAt - 1
                    breakIn = C_Endwhile

                case C_When:
                    parser.pc = wc[wccount].endLine - 1

                default:
                    parser.report(inbound.SourceLine,"A grue is attempting to BREAK out. (Breaking without a surrounding context!)")
                    finish(false, ERR_SYNTAX)
                    break
                }

            }

        case C_Enum:

            if inbound.TokenCount<4 || (
                ! (inbound.Tokens[2].tokType==LParen && inbound.Tokens[inbound.TokenCount-1].tokType==RParen) &&
                ! (inbound.Tokens[2].tokType==LeftCBrace && inbound.Tokens[inbound.TokenCount-1].tokType==RightCBrace)) {
                parser.report(inbound.SourceLine,"Incorrect arguments supplied for ENUM.")
                finish(false,ERR_SYNTAX)
                break
            }

            resu:=parser.splitCommaArray(ifs, inbound.Tokens[3:inbound.TokenCount-1])

            globlock.Lock()
            enum_name:=inbound.Tokens[1].tokText
            enum[enum_name]=&enum_s{}
            enum[enum_name].members=make(map[string]interface{})
            globlock.Unlock()

            var nextVal interface{}
            nextVal=0           // auto incs to 1 for first default value
            var member string
          enum_loop:
            for ea:=range resu {

                if len(resu[ea])==1 {
                    switch nextVal.(type) {
                    case int:
                        nextVal=nextVal.(int)+1
                    case uint:
                        nextVal=nextVal.(uint)+1
                    case int64:
                        nextVal=nextVal.(int64)+1
                    case float64:
                        nextVal=nextVal.(float64)+1
                    default:
                        // non-incremental error
                        parser.report(inbound.SourceLine,"Cannot increment default value in ENUM")
                        finish(false,ERR_EVAL)
                        break enum_loop
                    }

                    globlock.Lock()
                    member=resu[ea][0].tokText
                    enum[enum_name].members[member]=nextVal
                    enum[enum_name].ordered=append(enum[enum_name].ordered,member)
                    globlock.Unlock()

                } else {
                    //   member = constant
                    // | member = expr
                    if len(resu[ea])>2 {
                        if resu[ea][1].tokType==O_Assign {

                            evEnum := parser.wrappedEval(ifs,ident,ifs,ident,resu[ea][2:])

                            if evEnum.evalError {
                                parser.report(inbound.SourceLine,"Invalid expression for assignment in ENUM")
                                finish(false, ERR_EVAL)
                                break enum_loop
                            }

                            nextVal=evEnum.result

                            globlock.Lock()
                            member=resu[ea][0].tokText
                            enum[enum_name].members[member]=nextVal
                            enum[enum_name].ordered=append(enum[enum_name].ordered,member)
                            globlock.Unlock()


                        } else {
                            // error
                            parser.report(inbound.SourceLine,"Missing assignment in ENUM")
                            finish(false,ERR_SYNTAX)
                            break enum_loop
                        }
                    }
                }
            }


        case C_Unset: // remove a variable

            if inbound.TokenCount != 2 {
                parser.report(inbound.SourceLine,"Incorrect arguments supplied for UNSET.")
                finish(false, ERR_SYNTAX)
            } else {
                removee := inbound.Tokens[1].tokText
                // should have a lock around varlookup really:
                if VarLookup(ifs, ident,removee) {
                    vunset(ifs, ident, removee)
                } else {
                    parser.report(inbound.SourceLine,sf("Variable %s does not exist.", removee))
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
                    parser.report(inbound.SourceLine,"Too many arguments supplied.")
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
                    parser.report(inbound.SourceLine,"Invalid pane selection.")
                    finish(false, ERR_SYNTAX)
                    break
                }

                cp,_ := parser.Eval(ifs,inbound.Tokens[2:3])

                switch cp:=cp.(type) {
                case string:

                    setPane(cp)
                    currentpane = cp

                default:
                    parser.report(inbound.SourceLine,"Warning: you must provide a string value to PANE SELECT.")
                    finish(false,ERR_EVAL)
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

                if nameCommaAt==-1 || YCommaAt==-1 || XCommaAt==-1 || HCommaAt==-1 {
                    parser.report(inbound.SourceLine,"Bad delimiter in PANE DEFINE.")
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
                pname  := parser.wrappedEval(ifs,ident,ifs,ident, inbound.Tokens[2:nameCommaAt])
                py     := parser.wrappedEval(ifs,ident,ifs,ident, inbound.Tokens[nameCommaAt+1:YCommaAt])
                px     := parser.wrappedEval(ifs,ident,ifs,ident, inbound.Tokens[YCommaAt+1:XCommaAt])
                ph     := parser.wrappedEval(ifs,ident,ifs,ident, inbound.Tokens[XCommaAt+1:HCommaAt])
                pw     := parser.wrappedEval(ifs,ident,ifs,ident, ew)
                if hasTitle {
                    ptitle = parser.wrappedEval(ifs,ident,ifs,ident, etit)
                }
                if hasBox   {
                    pbox   = parser.wrappedEval(ifs,ident,ifs,ident, ebox)
                }

                if pname.evalError || py.evalError || px.evalError || ph.evalError || pw.evalError {
                    parser.report(inbound.SourceLine,"could not evaluate an argument in PANE DEFINE")
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
                    parser.report(inbound.SourceLine,sf("Could not use an argument in PANE DEFINE. [%T %T %T %T]",px.result,py.result,pw.result,ph.result))
                    finish(false,ERR_EVAL)
                    break
                }

                if pname.result.(string) == "global" {
                    parser.report(inbound.SourceLine,"Cannot redefine the global PANE.")
                    finish(false, ERR_SYNTAX)
                    break
                }

                panes[name] = Pane{row: row, col: col, w: w, h: h, title: title, boxed: boxed}
                paneBox(name)

            case "title":
                if inbound.TokenCount>2 {
                    etit := inbound.Tokens[2:]
                    ptitle := parser.wrappedEval(ifs,ident,ifs,ident,etit)
                    p:=panes[currentpane]
                    p.title=sf("%v",ptitle.result)
                    panes[currentpane]=p
                    paneBox(currentpane)
                }

            case "redraw":
                paneBox(currentpane)

            default:
                parser.report(inbound.SourceLine,"Unknown PANE command.")
                finish(false, ERR_SYNTAX)
            }


        case SYM_BOR: // Local Command

            bc:=basecode[source_base][parser.pc].borcmd

            /*
            pf("\n")
            pf("In local command\nCalled with ifs:%d and tokens->%+v\n",ifs,inbound.Tokens)
            pf("  source_base -> %v\n",source_base)
            pf("  basecode    -> %v\n",basecode[source_base][parser.pc].Original)
            pf("  bor cmd     -> %#v\n",bc)
            pf("\n")
            */

            if inbound.TokenCount==2 && hasOuter(inbound.Tokens[1].tokText,'`') {
                s:=stripOuter(inbound.Tokens[1].tokText,'`')
                coprocCall(parser,ifs,ident,s)
            } else {
                coprocCall(parser,ifs,ident,bc)
            }


        case C_Pause:

            var i string

            if inbound.TokenCount<2 {
                parser.report(inbound.SourceLine,"Not enough arguments in PAUSE.")
                finish(false, ERR_SYNTAX)
                break
            }

            we = parser.wrappedEval(ifs,ident,ifs,ident, inbound.Tokens[1:])

            if !we.evalError {

                if isNumber(we.result) {
                    i = sf("%v", we.result)
                } else {
                    i = we.result.(string)
                }

                dur, err := time.ParseDuration(i + "ms")

                if err != nil {
                    parser.report(inbound.SourceLine,sf("'%s' did not evaluate to a duration.", we.text))
                    finish(false, ERR_EVAL)
                    break
                }

                time.Sleep(dur)

            } else {
                parser.report(inbound.SourceLine,sf("could not evaluate PAUSE expression\n%+v",we.errVal))
                finish(false, ERR_EVAL)
                break
            }


        case C_Doc:

            if testMode {
                if inbound.TokenCount > 1 {
                    evnest:=0
                    newstart:=0
                    docout := ""
                    for term := range inbound.Tokens[1:] {
                        nt:=inbound.Tokens[1+term]
                        if nt.tokType==LParen || nt.tokType==LeftSBrace  { evnest+=1 }
                        if nt.tokType==RParen || nt.tokType==RightSBrace { evnest-=1 }
                        if evnest==0 && (term==len(inbound.Tokens[1:])-1 || nt.tokType == O_Comma) {
                            v,_ := parser.Eval(ifs,inbound.Tokens[1+newstart:term+2])
                            newstart=term+1
                            switch v.(type) { case string: v=interpolate(ifs,ident,v.(string)) }
                            docout += sparkle(sf(`%v`, v))
                            continue
                        }
                    }

                    appendToTestReport(test_output_file,ifs, parser.pc, docout)

                }
            }


        case C_Test:

            // TEST "name" GROUP "group_name" ASSERT FAIL|CONTINUE

            testlock.Lock()
            inside_test = true

            if testMode {

                if !(inbound.TokenCount == 4 || inbound.TokenCount == 6) {
                    parser.report(inbound.SourceLine,"Badly formatted TEST command.")
                    finish(false, ERR_SYNTAX)
                    testlock.Unlock()
                    break
                }

                if ! str.EqualFold(inbound.Tokens[2].tokText,"group") {
                    parser.report(inbound.SourceLine,"Missing GROUP in TEST command.")
                    finish(false, ERR_SYNTAX)
                    testlock.Unlock()
                    break
                }

                test_assert = "fail"
                if inbound.TokenCount == 6 {
                    if ! str.EqualFold(inbound.Tokens[4].tokText,"assert") {
                        parser.report(inbound.SourceLine,"Missing ASSERT in TEST command.")
                        finish(false, ERR_SYNTAX)
                        break
                    } else {
                        switch str.ToLower(inbound.Tokens[5].tokText) {
                        case "fail":
                            test_assert = "fail"
                        case "continue":
                            test_assert = "continue"
                        default:
                            parser.report(inbound.SourceLine,"Bad ASSERT type in TEST command.")
                            finish(false, ERR_SYNTAX)
                            testlock.Unlock()
                            break
                        }
                    }
                }

                test_name = interpolate(ifs,ident,stripOuterQuotes(inbound.Tokens[1].tokText, 2))
                test_group = interpolate(ifs,ident,stripOuterQuotes(inbound.Tokens[3].tokText, 2))

                under_test = false
                // if filter matches group
                if test_name_filter=="" {
                    if matched, _ := regexp.MatchString(test_group_filter, test_group); matched {
                        vset(ifs,ident,"_test_group",test_group)
                        vset(ifs,ident,"_test_name",test_name)
                        under_test = true
                        appendToTestReport(test_output_file,ifs, parser.pc, sf("\nTest Section : [#5][#bold]%s/%s[#boff][#-]",test_group,test_name))
                    }
                } else {
                    // if filter matches name
                    if matched, _ := regexp.MatchString(test_name_filter, test_name); matched {
                        vset(ifs,ident,"_test_group",test_group)
                        vset(ifs,ident,"_test_name",test_name)
                        under_test = true
                        appendToTestReport(test_output_file,ifs, parser.pc, sf("\nTest Section : [#5][#bold]%s/%s[#boff][#-]",test_group,test_name))
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

            // > print tokens("on int(diff_{i})<0 do print")
            //  on int        (      diff_42    )      <      0         do         print
            //  ON IDENTIFIER LPAREN IDENTIFIER RPAREN SYM_LT N_LITERAL IDENTIFIER PRINT
            //  0  1          2      3          4      5      6         7          8...

            if inbound.TokenCount > 2 {

                doAt := findDelim(inbound.Tokens, C_Do, 2)
                if doAt == -1 {
                    parser.report(inbound.SourceLine,"DO not found in ON")
                    finish(false, ERR_SYNTAX)
                } else {
                    // more tokens after the DO to form a command with?
                    if inbound.TokenCount >= doAt {

                        we = parser.wrappedEval(ifs,ident,ifs,ident, inbound.Tokens[1:doAt])
                        if we.evalError {
                            parser.report(inbound.SourceLine, sf("Could not evaluate expression '%v' in ON..DO statement.\n%+v",we.text,we.errVal))
                            finish(false,ERR_EVAL)
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
                                inbound=&p
                                basecode_entry=&b
                                goto ondo_reenter

                            }
                        default:
                            pf("Result Type -> %T expression was -> %s\n", we.text, we.result)
                            parser.report(inbound.SourceLine,"ON cannot operate without a condition.")
                            finish(false, ERR_EVAL)
                            break
                        }

                    }
                }

            } else {
                parser.report(inbound.SourceLine,"ON missing arguments.")
                finish(false, ERR_SYNTAX)
            }


        case C_Assert:

            if inbound.TokenCount < 2 {
                parser.report(inbound.SourceLine,"Insufficient arguments supplied to ASSERT")
                finish(false, ERR_ASSERT)
            } else {

                cet := crushEvalTokens(inbound.Tokens[1:])
                we = parser.wrappedEval(ifs,ident,ifs,ident,inbound.Tokens[1:])

                if we.assign {
                    // someone typo'ed a condition 99.9999% of the time
                    parser.report(inbound.SourceLine,"[#2][#bold]Warning! Assert contained an assignment![#-][#boff]")
                    finish(false,ERR_ASSERT)
                    break
                }

                if we.evalError {
                    parser.report(inbound.SourceLine,"Could not evaluate expression in ASSERT statement")
                    finish(false,ERR_EVAL)
                    break
                }

                testlock.Lock()
                switch we.result.(type) {
                case bool:
                    var test_report string

                    group_name_string := ""
                    if test_group != "" {
                        group_name_string += test_group + "/"
                    }
                    if test_name != "" {
                        group_name_string += test_name
                    }

                    if !we.result.(bool) {
                        if !under_test {
                            parser.report(inbound.SourceLine,sf("Could not assert! ( %s )", cet.text))
                            finish(false, ERR_ASSERT)
                            break
                        }
                        // under test
                        test_report = sf("[#2]TEST FAILED %s (%s/line %d) : %s[#-]",
                            group_name_string, getReportFunctionName(ifs,false), 1+inbound.SourceLine, we.text)
                        testsFailed+=1
                        appendToTestReport(test_output_file,ifs, 1+inbound.SourceLine, test_report)
                        temp_test_assert := test_assert
                        if fail_override != "" {
                            temp_test_assert = fail_override
                        }
                        switch temp_test_assert {
                        case "fail":
                            parser.report(inbound.SourceLine,sf("Could not assert! (%s)", we.text))
                            finish(false, ERR_ASSERT)
                        case "continue":
                            parser.report(inbound.SourceLine,sf("Assert failed (%s), but continuing.", we.text))
                        }
                    } else {
                        if under_test {
                            test_report = sf("[#4]TEST PASSED %s (%s/line %d) : %s[#-]",
                                group_name_string, getReportFunctionName(ifs,false), 1+inbound.SourceLine,cet.text)
                            testsPassed+=1
                            appendToTestReport(test_output_file,ifs, parser.pc, test_report)
                        }
                    }
                }
                testlock.Unlock()

            }

        case C_Help:
            hargs := ""
            if inbound.TokenCount == 2 {
                hargs = inbound.Tokens[1].tokText
            }
            ihelp(hargs)

        case C_Nop:
            // time.Sleep(1 * time.Microsecond)

        case C_Async:

            // ASYNC IDENTIFIER IDENTIFIER LPAREN [EXPRESSION[,...]] RPAREN [IDENTIFIER]
            // async handles    q          (      [e[,...]]          )      [key]
            // 0     1          2          3      4

            if inbound.TokenCount<5 {
                usage := "ASYNC [#i1]handle_map function_call([args]) [next_id][#i0]"
                parser.report(inbound.SourceLine,"Invalid arguments in ASYNC\n"+usage)
                finish(false,ERR_SYNTAX)
                break
            }

            handles := inbound.Tokens[1].tokText
            call    := inbound.Tokens[2].tokText

            if inbound.Tokens[3].tokType!=LParen {
                parser.report(inbound.SourceLine,"could not find '(' in ASYNC function call.")
                finish(false,ERR_SYNTAX)
            }

            // get arguments

            var rightParenLoc int16
            for ap:=inbound.TokenCount-1; ap>3; ap-- {
                if inbound.Tokens[ap].tokType==RParen {
                    rightParenLoc=ap
                    break
                }
            }

            if rightParenLoc<4 {
               parser.report(inbound.SourceLine,"could not find a valid ')' in ASYNC function call.")
                finish(false,ERR_SYNTAX)
            }

            resu,errs:=parser.evalCommaArray(ifs, inbound.Tokens[4:rightParenLoc])

            // find the optional key argument, for stipulating the key name to be used in handles
            var nival interface{}
            if rightParenLoc!=inbound.TokenCount-1 {
                var err error
                nival,err = parser.Eval(ifs,inbound.Tokens[rightParenLoc+1:])
                nival=sf("%v",nival)
                if err!=nil {
                    parser.report(inbound.SourceLine,sf("could not evaluate handle key argument '%+v' in ASYNC.",inbound.Tokens[rightParenLoc+1:]))
                    finish(false,ERR_EVAL)
                    break
                }
            }

            lmv, isfunc := fnlookup.lmget(call)

            if isfunc {

                errClear:=true
                for e:=0; e<len(errs); e+=1 {
                    if errs[e]!=nil {
                        // error
                        pf("- arg %d: %+v\n",errs[e])
                        errClear=false
                    }
                }

                if !errClear {
                    parser.report(inbound.SourceLine,sf("problem evaluating arguments in function call. (fs=%v)\n", ifs))
                    finish(false, ERR_EVAL)
                    break
                }

                // make Za function call

                // construct a go call that includes a normal Call
                if handles=="nil" {
                    _,_=task(ifs,lmv,true,call,resu...)
                } else {
                    h,id:=task(ifs,lmv,false,call,resu...)
                    // time.Sleep(1 * time.Millisecond)
                    // assign channel h to handles map
                    if nival==nil {
                        vsetElement(ifs,ident,handles,sf("async_%v",id),h)
                    } else {
                        vsetElement(ifs,ident,handles,sf("%v",nival),h)
                    }
                }

            } else {
                // func not found
                parser.report(inbound.SourceLine,sf("invalid function '%s' in ASYNC call",call))
                finish(false,ERR_EVAL)
            }


        case C_Require:

            // require feat support in stdlib first. requires version-as-feat support and markup.

            if inbound.TokenCount < 2 {
                parser.report(inbound.SourceLine,"Malformed REQUIRE statement.")
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
                required=str.Replace(required," ","",-1)
                _, e := vconvert(required)
                if e==nil {
                    // sem ver provided / compare to language version
                    lver,_ :=gvget("@version")
                    lcmp,_ :=vcmp(lver.(string),required)
                    if lcmp==-1 { // lang ver is lower than required ver
                        // error
                        pf("Language version of '%s' is too low (%s<%s). Quitting.\n", lver, lver, required)
                        finish(true, ERR_REQUIRE)
                    }
                    reqEnd=true
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
                resu,errs:=parser.evalCommaArray(ifs,inbound.Tokens[1:])
                errmsg:=""
                if len(resu)>1 && errs[1]==nil {
                    switch resu[1].(type) {
                    case string:
                        errmsg=sf("%v\n",resu[1])
                    }
                }
                if len(resu)>0 && errs[0]==nil {
                    ec:=resu[0]
                    pf(errmsg)
                    if isNumber(ec) {
                        finish(true, ec.(int))
                    } else {
                        parser.report(inbound.SourceLine,"Could not evaluate your EXIT expression")
                        finish(true,ERR_EVAL)
                    }
                }
            } else {
                finish(true, 0)
            }


        case C_Define:

            if inbound.TokenCount > 1 {

                if defining {
                    parser.report(inbound.SourceLine,"Already defining a function. Nesting not permitted.")
                    finish(true, ERR_SYNTAX)
                    break
                }

                defining = true
                definitionName = inbound.Tokens[1].tokText

                loc, _ := GetNextFnSpace(true,definitionName,call_s{prepared:false})
                var dargs []string

                if inbound.TokenCount > 2 {
                    // params supplied:
                    argString := crushEvalTokens(inbound.Tokens[2:]).text
                    argString = stripOuter(argString, '(')
                    argString = stripOuter(argString, ')')

                    if len(argString)>0 {
                        dargs = str.Split(argString, ",")
                        for karg,_:=range dargs {
                            dargs[karg]=str.Trim(dargs[karg]," \t")
                            // pf("-- set darg in ifs %d of %d with '%+v'\n",loc,karg,dargs[karg])
                        }
                    }
                }

                // error if it clashes with a stdlib name
                exMatchStdlib:=false
                for n,_:=range slhelp {
                    if n==definitionName {
                        parser.report(inbound.SourceLine,"A library function already exists with the name '"+definitionName+"'")
                        finish(false,ERR_SYNTAX)
                        exMatchStdlib=true
                        break
                    }
                }
                if exMatchStdlib { break }

                // register new func in funcmap
                funcmap[currentModule+"."+definitionName]=Funcdef{
                    name:definitionName,
                    module:currentModule,
                    fs:loc,
                }

                sourceMap[loc]=source_base     // relate defined base 'loc' to parent 'ifs' instance's 'base' source
                // pf("[sm] loc %d -> %v\n",loc,source_base)
                fspacelock.Lock()
                functionspaces[loc] = []Phrase{}
                basecode[loc] = []BaseCode{}
                fspacelock.Unlock()

                farglock.Lock()
                functionArgs[loc].args   = dargs
                farglock.Unlock()

                // pf("defining new function %s (%d)\n",definitionName,loc)

            }

        case C_Showdef:

            if inbound.TokenCount == 2 {
                fn := stripOuterQuotes(inbound.Tokens[1].tokText, 2)
                if _, exists := fnlookup.lmget(fn); exists {
                    ShowDef(fn)
                } else {
                    parser.report(inbound.SourceLine,"Function not found.")
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

            // split return args by comma in evaluable lumps
            var rargs=make([][]Token,1)
            var curArg uint8
            evnest:=0
            argtoks:=inbound.Tokens[1:]

            rargs[0]=make([]Token,0)
            ppos:=0
            for tok := range argtoks {
                nt:=argtoks[tok]
                if nt.tokType==LParen { evnest+=1 }
                if nt.tokType==RParen { evnest-=1 }
                if nt.tokType==LeftSBrace { evnest+=1 }
                if nt.tokType==RightSBrace { evnest-=1 }
                if evnest==0 && (tok==len(argtoks)-1 || nt.tokType == O_Comma) {
                    rargs[curArg]=argtoks[ppos:tok+1]
                    ppos=tok+1
                    curArg+=1
                    if int(curArg)>=len(rargs) {
                        rargs=append(rargs,[]Token{})
                    }
                }
            }
            retval_count=curArg
            // pf("call() %d : args -> [%+v]\n",ifs,rargs)

            // tail call recursion handling:
            if inbound.TokenCount > 2 {

                var bname string
                bname, _ = numlookup.lmget(source_base)

                tco_check:=false // deny tco until we check all is well

                if inbound.Tokens[1].tokType==Identifier && inbound.Tokens[2].tokType==LParen {
                    if strcmp(inbound.Tokens[1].tokText,bname) {
                        rbraceAt := findDelim(inbound.Tokens,RParen, 2)
                        if rbraceAt==inbound.TokenCount-1 {
                            tco_check=true
                        }
                    }
                }

                if tco_check {
                    skip_reentry:=false
                    resu,errs:=parser.evalCommaArray(ifs,rargs[0][2:len(rargs[0])-1])
                    // populate var args for re-entry. should check errs here too...
                    for q:=0; q<len(errs); q+=1 {
                        va[q]=resu[q]
                        if errs[q]!=nil { skip_reentry=true; break }
                    }
                    // no args/wrong arg count check
                    if len(errs)!=len(va) {
                        skip_reentry=true
                    }

                    // set tco flag if required, and perform.
                    if !skip_reentry {
                        // vset(ifs,ident,"@in_tco",true)
                        // in_tco=true
                        parser.pc=-1
                        goto tco_reentry
                    }
                }
            }

            // evaluate each expr and stuff the results in an array
            var ev_er error
            retvalues=make([]interface{},curArg)
            for q:=0;q<int(curArg);q+=1 {
                retvalues[q], ev_er = parser.Eval(ifs,rargs[q])
                if ev_er!=nil {
                    parser.report(inbound.SourceLine,"Could not evaluate RETURN arguments")
                    finish(true,ERR_EVAL)
                    break
                }
            }
            // pf("call() #%d : rv -> [%+v]\n",ifs,retvalues)

            endFunc = true
            break

        case C_Enddef:

            if !defining {
                parser.report(inbound.SourceLine,"Not currently defining a function.")
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
                usage:= "INPUT [#i1]id[#i0] PARAM | OPTARG [#i1]field_position[#i0] [ [#i1]error_hint[#i0] ]\n"
                usage+= "INPUT [#i1]id[#i0] ENV [#i1]env_name[#i0]"
                parser.report(inbound.SourceLine,"Incorrect arguments supplied to INPUT.\n"+usage)
                finish(false, ERR_SYNTAX)
                break
            }

            id := inbound.Tokens[1].tokText
            typ := inbound.Tokens[2].tokText
            pos := inbound.Tokens[3].tokText

            hint:=id
            if inbound.TokenCount==5 {
                we=parser.wrappedEval(ifs,ident,ifs,ident,inbound.Tokens[4:])
                if !we.evalError {
                    hint=we.result.(string)
                }
            }

            // eval

            switch str.ToLower(typ) {
            case "param":

                we = parser.wrappedEval(ifs,ident,ifs,ident, inbound.Tokens[3:])
                if we.evalError {
                    parser.report(inbound.SourceLine,sf("could not evaluate the INPUT expression\n%+v",we.errVal))
                    finish(false, ERR_EVAL)
                    break
                }
                switch we.result.(type) {
                case int:
                default:
                    parser.report(inbound.SourceLine,"INPUT expression must evaluate to an integer")
                    finish(false,ERR_EVAL)
                    break
                }
                d:=we.result.(int)

                if d<1 {
                    parser.report(inbound.SourceLine, sf("INPUT position %d too low.",d))
                    finish(true, ERR_SYNTAX)
                    break
                }
                if d <= len(cmdargs) {
                    // if this is numeric, assign as an int
                    n, er := strconv.Atoi(cmdargs[d-1])
                    if er == nil {
                        vset(ifs, ident, id, n)
                    } else {
                        vset(ifs, ident, id, cmdargs[d-1])
                    }
                } else {
                    parser.report(inbound.SourceLine,sf("Expected CLI parameter [%s] not provided at startup.", hint))
                    finish(true, ERR_SYNTAX)
                }

            case "optarg":

                we = parser.wrappedEval(ifs,ident,ifs,ident, inbound.Tokens[3:])
                if we.evalError {
                    parser.report(inbound.SourceLine,sf("could not evaluate the INPUT expression\n%+v",we.errVal))
                    finish(false, ERR_EVAL)
                    break
                }
                switch we.result.(type) {
                case int:
                default:
                    parser.report(inbound.SourceLine,"INPUT expression must evaluate to an integer")
                    finish(false,ERR_EVAL)
                    break
                }
                d:=we.result.(int)

                if d <= len(cmdargs) {
                    // if this is numeric, assign as an int
                    n, er := strconv.Atoi(cmdargs[d-1])
                    if er == nil {
                        vset(ifs, ident, id, n)
                    } else {
                        vset(ifs, ident, id, cmdargs[d-1])
                    }
                } else {
                    // nothing provided but var didn't exist, so create it empty
                    // otherwise, just continue
                    if ! VarLookup(ifs,ident,id) {
                        vset(ifs,ident,id,"")
                    }
                }

            case "env":

                if os.Getenv(pos)!="" {
                    // non-empty env var so set id var to value.
                    vset(ifs, ident,id, os.Getenv(pos))
                } else {
                    // when env var empty either create the id var or
                    // leave it alone if it already exists.
                    if ! VarLookup(ifs,ident,id) {
                        vset(ifs,ident,id,"")
                    }
                }
            }


        case C_Module:

            if inbound.TokenCount > 1 {
                we = parser.wrappedEval(ifs,ident,ifs,ident, inbound.Tokens[1:])
                if we.evalError {
                    parser.report(inbound.SourceLine,sf("could not evaluate expression in MODULE statement\n%+v",we.errVal))
                    finish(false,ERR_MODULE)
                    break
                }
            } else {
                parser.report(inbound.SourceLine,"No module name provided.")
                finish(false, ERR_MODULE)
                break
            }

            fom := we.result.(string)

            if strcmp(fom,"") {
                parser.report(inbound.SourceLine,"Empty module name provided.")
                finish(false, ERR_MODULE)
                break
            }

            //.. set file location

            var moduleloc string = ""

            if str.IndexByte(fom, '/') > -1 {
                if filepath.IsAbs(fom) {
                    moduleloc = fom
                } else {
                    mdir, _ := gvget("@execpath")
                    moduleloc = mdir.(string)+"/"+fom
                }
            } else {

                // modules default path is $HOME/.za/modules
                //  unless otherwise redefined in environmental variable ZA_MODPATH

                modhome, _ := gvget("@home")
                modhome = modhome.(string) + "/.za"
                if os.Getenv("ZA_MODPATH") != "" {
                    modhome = os.Getenv("ZA_MODPATH")
                }

                moduleloc = modhome.(string) + "/modules/" + fom + ".fom"

            }

            //.. validate module exists

            f, err := os.Stat(moduleloc)

            if err != nil {
                parser.report(inbound.SourceLine, sf("Module is not accessible. (path:%v)",moduleloc))
                finish(false, ERR_MODULE)
                break
            }

            if !f.Mode().IsRegular() {
                parser.report(inbound.SourceLine,  "Module is not a regular file.")
                finish(false, ERR_MODULE)
                break
            }

            //.. read in file

            mod, err := ioutil.ReadFile(moduleloc)
            if err != nil {
                parser.report(inbound.SourceLine,  "Problem reading the module file.")
                finish(false, ERR_MODULE)
                break
            }

            // tokenise and parse into a new function space.

            //.. error if it has already been defined
            if fnlookup.lmexists("@mod_"+fom) && !permit_dupmod {
                parser.report(inbound.SourceLine,"Module file "+fom+" already processed once.")
                finish(false, ERR_SYNTAX)
                break
            }

            if !fnlookup.lmexists("@mod_"+fom) {

                loc, _ := GetNextFnSpace(true,"@mod_"+fom,call_s{prepared:false})

                calllock.Lock()

                fspacelock.Lock()
                functionspaces[loc] = []Phrase{}
                basecode[loc] = []BaseCode{}
                fspacelock.Unlock()

                farglock.Lock()
                functionArgs[loc].args  = []string{}
                farglock.Unlock()

                oldModule:=currentModule
                currentModule=path.Base(fom)
                currentModule=str.TrimSuffix(currentModule,".mod")
                modlist[currentModule]=true

                //.. parse and execute
                fileMap[loc]=moduleloc
                // pf("[fm] loc %d -> %v\n",loc,moduleloc)

                if debug_level>10 {
                    start := time.Now()
                    phraseParse("@mod_"+fom, string(mod), 0)
                    elapsed := time.Since(start)
                    pf("(timings-module) elapsed in mod translation for '%s' : %v\n",fom,elapsed)
                } else {
                    phraseParse("@mod_"+fom, string(mod), 0)
                }
                modcs := call_s{}
                modcs.base = loc
                modcs.caller = ifs
                modcs.fs = "@mod_" + fom
                calltable[loc] = modcs

                calllock.Unlock()

                var modident [szIdent]Variable

                // pf("[mod] loc -> %d\n",loc)
                if debug_level>10 {
                    start := time.Now()
                    Call(MODE_NEW, &modident, loc, ciMod)
                    elapsed := time.Since(start)
                    pf("(timings-module) elapsed in mod execution for '%s' : %v\n",fom,elapsed)
                } else {
                    Call(MODE_NEW, &modident, loc, ciMod)
                }

                calllock.Lock()
                calltable[ifs].gcShyness=20
                calltable[ifs].gc=true
                calllock.Unlock()

                currentModule=oldModule

            }

        case C_When:

            // need to store the condition and result for the is/contains/has/or clauses
            // endwhen location should be calculated in advance for a direct jump to exit

            if wccount==WHEN_CAP {
                parser.report(inbound.SourceLine,sf("maximum WHEN nesting reached (%d)",WHEN_CAP))
                finish(true,ERR_SYNTAX)
                break
            }

            // make comparator True if missing.
            if inbound.TokenCount==1 {
                inbound.Tokens=append(inbound.Tokens,Token{tokType:Identifier,subtype:subtypeConst,tokVal:true,tokText:"true"})
            }

            // lookahead
            endfound, enddistance, er := lookahead(source_base, parser.pc, 0, 0, C_Endwhen, []uint8{C_When}, []uint8{C_Endwhen})

            if er {
                parser.report(inbound.SourceLine,"Lookahead dedent error!")
                finish(true, ERR_SYNTAX)
                break
            }

            if !endfound {
                parser.report(inbound.SourceLine,"Missing ENDWHEN for this WHEN. Maybe check for open quotes or braces in block?")
                finish(false, ERR_SYNTAX)
                break
            }

            we = parser.wrappedEval(ifs,ident,ifs,ident,inbound.Tokens[1:])
            if we.evalError {
                parser.report(inbound.SourceLine,sf("could not evaluate the WHEN condition\n%+v",we.errVal))
                finish(false, ERR_EVAL)
                break
            }

            // create storage for WHEN details and increase the nesting level

            wccount+=1
            wc[wccount] = whenCarton{endLine: parser.pc + enddistance, value: we.result, performed:false, dodefault: true}
            depth+=1
            lastConstruct = append(lastConstruct, C_When)


        case C_Is, C_Has, C_Contains, C_Or:

            if lastConstruct[len(lastConstruct)-1] != C_When {
                parser.report(inbound.SourceLine,"Not currently in a WHEN block.")
                finish(false,ERR_SYNTAX)
                break
            }

            carton := wc[wccount]

            if carton.performed {
                // already matched and executed a WHEN case so jump to ENDWHEN
                parser.pc = carton.endLine - 1
                break
            }

            if inbound.TokenCount > 1 { // inbound.TokenCount==1 for C_Or
                we = parser.wrappedEval(ifs,ident,ifs,ident, inbound.Tokens[1:])
                if we.evalError {
                    parser.report(inbound.SourceLine,sf("could not evaluate expression in WHEN condition\n%+v",we.errVal))
                    finish(false, ERR_EVAL)
                    break
                }
            }

            ramble_on := false // assume we'll need to skip to next when clause

            // pf("when-eval: checking type : %s\n%#v\n",tokNames[statement.tokType],carton)

            switch inbound.Tokens[0].tokType {

            case C_Has: // <-- @note: this may change yet

                // build expression from rest, ignore initial condition
                switch we.result.(type) {
                case bool:
                    if we.result.(bool) {  // HAS truth
                        wc[wccount].performed = true
                        wc[wccount].dodefault = false
                        // pf("when-has (@line %d): true -> %+v == %+v\n",inbound.SourceLine,we.result,carton.value)
                        ramble_on = true
                    }
                default:
                    parser.report(inbound.SourceLine,sf("HAS condition did not result in a boolean\n%+v",we.errVal))
                    finish(false, ERR_EVAL)
                }

            case C_Is:
                if we.result == carton.value { // matched IS value
                    wc[wccount].performed = true
                    wc[wccount].dodefault = false
                    // pf("when-is (@line %d): true -> %+v == %+v\n",inbound.SourceLine,we.result,carton.value)
                    ramble_on = true
                }

            case C_Contains:
                // pf("when-reached-contains\ncarton: %#v\n",carton)
                reg := sparkle(we.result.(string))
                switch carton.value.(type) {
                case string:
                    if matched, _ := regexp.MatchString(reg, carton.value.(string)); matched { // matched CONTAINS regex
                        wc[wccount].performed = true
                        wc[wccount].dodefault = false
                        // pf("when-contains (@line %d): true -> %+v == %+v\n",inbound.SourceLine,we.result,carton.value)
                        ramble_on = true
                    }
                case int:
                    if matched, _ := regexp.MatchString(reg, strconv.Itoa(carton.value.(int))); matched { // matched CONTAINS regex
                        wc[wccount].performed = true
                        wc[wccount].dodefault = false
                        // pf("when-contains (@line %d): true -> %+v == %+v\n",inbound.SourceLine,we.result,carton.value)
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
                // skip to next WHEN clause:
                hasfound, hasdistance, _ := lookahead(source_base, parser.pc+1, 0, 0, C_Has, []uint8{C_When}, []uint8{C_Endwhen})
                isfound, isdistance, _   := lookahead(source_base, parser.pc+1, 0, 0, C_Is, []uint8{C_When}, []uint8{C_Endwhen})
                orfound, ordistance, _   := lookahead(source_base, parser.pc+1, 0, 0, C_Or, []uint8{C_When}, []uint8{C_Endwhen})
                cofound, codistance, _   := lookahead(source_base, parser.pc+1, 0, 0, C_Contains, []uint8{C_When}, []uint8{C_Endwhen})

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
                pf("when-distlist: %#v\n",distList)
                pf("when-hasfound,hasdistance: %v,%v\n",hasfound,hasdistance)
                pf("when-isfound,isdistance: %v,%v\n",isfound,isdistance)
                pf("when-cofound,codistance: %v,%v\n",cofound,codistance)
                pf("when-orfound,ordistance: %v,%v\n",orfound,ordistance)
                */

                if !(isfound || hasfound || orfound || cofound) {
                    // must be an endwhen
                    loc = carton.endLine
                    // pf("@%d : direct jump to endwhen at %d\n",parser.pc,loc+1)
                } else {
                    loc = parser.pc + min_int16(distList) + 1
                    // pf("@%d : direct jump from distList to %d\n",parser.pc,loc+1)
                }

                // jump to nearest following clause
                parser.pc = loc - 1
            }


        case C_Endwhen:

            if !forceEnd && lastConstruct[len(lastConstruct)-1] != C_When {
                parser.report(inbound.SourceLine, "Not currently in a WHEN block.")
                break
            }

            breakIn = Error
            forceEnd=false
            lastConstruct = lastConstruct[:depth-1]
            depth-=1
            wccount-=1

            if break_count>0 {
                break_count-=1
                if break_count>0 {
                    switch lastConstruct[depth-1] {
                    case C_For,C_Foreach:
                        breakIn=C_Endfor
                    case C_While:
                        breakIn=C_Endwhile
                    case C_When:
                        breakIn=C_Endwhen
                    }
                }
                // pf("ENDWHEN-BREAK: bc %d\n",break_count)
            }

            if wccount < 0 {
                parser.report(inbound.SourceLine,"Cannot reduce WHEN stack below zero.")
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
                parser.report(inbound.SourceLine,"Cannot nest a STRUCT")
                finish(false,ERR_SYNTAX)
                break
            }

            if inbound.TokenCount!=2 {
                parser.report(inbound.SourceLine,"STRUCT must contain a name.")
                finish(false,ERR_SYNTAX)
                break
            }

            structName=inbound.Tokens[1].tokText
            structMode=true

        case C_Endstruct:

            // ENDSTRUCT
            // end structmode

            if ! structMode {
                parser.report(inbound.SourceLine,"ENDSTRUCT without STRUCT.")
                finish(false,ERR_SYNTAX)
                break
            }

            //
            // take definition and create a structmaps entry from it:
            structmaps[structName]=structNode[:]

            structName=""
            structNode=[]interface{}{}
            structMode=false


        case C_Showstruct:

            // SHOWSTRUCT [filter]

            var filter string

            if inbound.TokenCount>1 {
                cet := crushEvalTokens(inbound.Tokens[1:])
                filter = interpolate(ifs,ident,cet.text)
            }

            for k,s:=range structmaps {

                if matched, _ := regexp.MatchString(filter, k); !matched { continue }

                pf("[#6]%v[#-]\n",k)

                for i:=0; i<len(s); i+=4 {
                    pf("[#4]%24v[#-] [#3]%v[#-]\n",s[i],s[i+1])
                }
                pf("\n")

            }


        case C_With:
            // WITH var AS file
            // get params

            if inbound.TokenCount < 4 {
                parser.report(inbound.SourceLine,"Malformed WITH statement.")
                finish(false, ERR_SYNTAX)
                break
            }

            asAt := findDelim(inbound.Tokens, C_As, 2)
            if asAt == -1 {
                parser.report(inbound.SourceLine,"AS not found in WITH")
                finish(false, ERR_SYNTAX)
                break
            }

            vname:=inbound.Tokens[1].tokText
            fname:=crushEvalTokens(inbound.Tokens[asAt+1:]).text

            if fname=="" || vname=="" {
                parser.report(inbound.SourceLine,"Bad arguments to provided to WITH.")
                finish(false,ERR_SYNTAX)
                break
            }

            vlock.RLock()
            if ! VarLookup(ifs,ident,vname) {
                vlock.RUnlock()
                parser.report(inbound.SourceLine,sf("Variable '%s' does not exist.",vname))
                finish(false,ERR_EVAL)
                break
            }
            vlock.RUnlock()

            tfile, err:= ioutil.TempFile("","za_with_"+sf("%d",os.Getpid())+"_")
            if err!=nil {
                parser.report(inbound.SourceLine,"WITH could not create a temporary file.")
                finish(true,ERR_SYNTAX)
                break
            }

            content,_:=vget(ifs,ident,vname)

            ioutil.WriteFile(tfile.Name(), []byte(content.(string)), 0600)
            vset(ifs,ident,fname,tfile.Name())
            inside_with=true
            current_with_handle=tfile

            defer func() {
                remfile:=current_with_handle.Name()
                current_with_handle.Close()
                current_with_handle=nil
                err:=os.Remove(remfile)
                if err!=nil {
                    parser.report(inbound.SourceLine,sf("WITH could not remove temporary file '%s'",remfile))
                    finish(true,ERR_FATAL)
                }
            }()

        case C_Endwith:
            if !inside_with {
                parser.report(inbound.SourceLine,"ENDWITH without a WITH.")
                finish(false,ERR_SYNTAX)
                break
            }

            inside_with=false


        case C_Print:
            parser.console_output(inbound.Tokens[1:],ifs,ident,interactive,false,false)

        case C_Println:
            parser.console_output(inbound.Tokens[1:],ifs,ident,interactive,true,false)

        case C_Log:
            parser.console_output(inbound.Tokens[1:],ifs,ident,false,false,true)


        case C_Hist:

            for h, v := range hist {
                pf("%5d : %s\n", h, v)
            }

        case C_At:

            // AT row ',' column [ ',' print_expr ... ]

            commaAt := findDelim(inbound.Tokens, O_Comma, 1)

            if commaAt == -1 || commaAt == inbound.TokenCount {
                parser.report(inbound.SourceLine,"Bad delimiter in AT.")
                finish(false, ERR_SYNTAX)
            } else {

                expr_row, err := parser.Eval(ifs,inbound.Tokens[1:commaAt])
                if expr_row==nil || err != nil {
                    parser.report(inbound.SourceLine,sf("Evaluation error in %v", expr_row))
                }

                nextCommaAt := findDelim(inbound.Tokens, O_Comma, commaAt+1)
                if nextCommaAt==-1 {
                    nextCommaAt=inbound.TokenCount
                }

                expr_col, err := parser.Eval(ifs,inbound.Tokens[commaAt+1:nextCommaAt])
                if expr_col==nil || err != nil {
                    parser.report(inbound.SourceLine,sf("Evaluation error in %v", expr_col))
                }

                row, _ = GetAsInt(expr_row)
                col, _ = GetAsInt(expr_col)

                at(row, col)

                // print surplus, no LF
                if inbound.TokenCount>nextCommaAt+1 {
                    parser.console_output(inbound.Tokens[nextCommaAt+1:],ifs,ident,interactive,false,false)
                }

            }


        case C_Prompt:

            // else continue

            if inbound.TokenCount < 2 {
                usage := "PROMPT [#i1]storage_variable prompt_string[#i0] [ [#i1]validator_regex[#i0] ]"
                parser.report(inbound.SourceLine,  "Not enough arguments for PROMPT.\n"+usage)
                finish(false, ERR_SYNTAX)
                break
            }

            // prompt variable assignment:
            if inbound.TokenCount > 1 { // um, should not do this but...
                if inbound.Tokens[1].tokType == O_Assign {
                    we = parser.wrappedEval(ifs,ident,ifs,ident, inbound.Tokens[2:])
                    if we.evalError {
                        parser.report(inbound.SourceLine,sf("could not evaluate expression prompt assignment\n%+v",we.errVal))
                        finish(false, ERR_EVAL)
                        break
                    }
                    switch we.result.(type) {
                    case string:
                        PromptTemplate=stripOuterQuotes(inbound.Tokens[2].tokText,1)
                    }
                } else {
                    // prompt command:
                    if str.EqualFold(inbound.Tokens[1].tokText,"colour") {
                        pcol:=parser.wrappedEval(ifs,ident,ifs,ident,inbound.Tokens[2:])
                        if pcol.evalError {
                            parser.report(inbound.SourceLine,"could not evaluate prompt colour")
                            finish(false,ERR_EVAL)
                            break
                        }
                        promptColour="[#"+sf("%v",pcol.result)+"]"
                        // pf("colour is '"+promptColour+"'\n")
                    } else {
                        if inbound.TokenCount < 3 {
                            parser.report(inbound.SourceLine, "Incorrect arguments for PROMPT command.")
                            finish(false, ERR_SYNTAX)
                            break
                        } else {
                            validator := ""
                            broken := false
                            expr, prompt_ev_err := parser.Eval(ifs,inbound.Tokens[2:3])
                            if expr==nil {
                                parser.report(inbound.SourceLine, "Could not evaluate in PROMPT command.")
                                finish(false,ERR_EVAL)
                                break
                            }
                            if prompt_ev_err == nil {
                                processedPrompt := expr.(string)
                                echoMask,_:=gvget("@echomask")
                                if inbound.TokenCount > 3 {
                                    val_ex,val_ex_error := parser.Eval(ifs,inbound.Tokens[3:])
                                    if val_ex_error != nil {
                                        parser.report(inbound.SourceLine,"Validator invalid in PROMPT!")
                                        finish(false,ERR_EVAL)
                                        break
                                    }
                                    validator = val_ex.(string)
                                    intext := ""
                                    validated := false
                                    for !validated || broken {
                                        intext, _, broken = getInput(processedPrompt, currentpane, row, col, promptColour, false, false, echoMask.(string))
                                        intext=sanitise(intext)
                                        validated, _ = regexp.MatchString(validator, intext)
                                    }
                                    if !broken {
                                        vset(ifs, ident,inbound.Tokens[1].tokText, intext)
                                    }
                                } else {
                                    var inp string
                                    inp, _, broken = getInput(processedPrompt, currentpane, row, col, promptColour, false, false, echoMask.(string))
                                    inp=sanitise(inp)
                                    vset(ifs, ident,inbound.Tokens[1].tokText, inp)
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

            if inbound.TokenCount < 2 || inbound.TokenCount > 3 {
                parser.report(inbound.SourceLine,  "LOGGING command malformed.")
                finish(false, ERR_SYNTAX)
                break
            }

            switch str.ToLower(inbound.Tokens[1].tokText) {

            case "off":
                loggingEnabled = false

            case "on":
                loggingEnabled = true
                if inbound.TokenCount == 3 {
                    we = parser.wrappedEval(ifs,ident,ifs,ident, inbound.Tokens[2:])
                    if we.evalError {
                        parser.report(inbound.SourceLine, sf("could not evaluate destination filename in LOGGING ON statement\n%+v",we.errVal))
                        finish(false, ERR_EVAL)
                        break
                    }
                    logFile = we.result.(string)
                    gvset("@logsubject", "")
                }

            case "quiet":
                gvset("@silentlog", true)

            case "loud":
                gvset("@silentlog", false)

            case "accessfile":
                if inbound.TokenCount > 2 {
                    we = parser.wrappedEval(ifs,ident,ifs,ident, inbound.Tokens[2:])
                    if we.evalError {
                        parser.report(inbound.SourceLine, sf("could not evaluate filename in LOGGING ACCESSFILE statement\n%+v",we.errVal))
                        finish(false, ERR_EVAL)
                        break
                    }
                    web_log_file=we.result.(string)
                    // pf("accessfile changed to %v\n",web_log_file)
                    web_log_handle.Close()
                    var err error
                    web_log_handle, err = os.OpenFile(web_log_file, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
                    if err != nil {
                        log.Println(err)
                    }
                    web_logger = log.New(web_log_handle, "", log.LstdFlags) // no prepended text
                } else {
                    parser.report(inbound.SourceLine, "No access file provided for LOGGING ACCESSFILE command.")
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
                        parser.report(inbound.SourceLine, "Invalid state set for LOGGING WEB.")
                        finish(false, ERR_EVAL)
                    }
                } else {
                    parser.report(inbound.SourceLine, "No state provided for LOGGING WEB command.")
                    finish(false, ERR_SYNTAX)
                }

            case "subject":
                if inbound.TokenCount == 3 {
                    we = parser.wrappedEval(ifs,ident,ifs,ident, inbound.Tokens[2:])
                    if we.evalError {
                        parser.report(inbound.SourceLine, sf("could not evaluate logging subject in LOGGING SUBJECT statement\n%+v",we.errVal))
                        finish(false, ERR_EVAL)
                        break
                    }
                    gvset("@logsubject", we.result.(string))
                } else {
                    gvset("@logsubject", "")
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
                    for l := 1; l < p.h; l+=1 {
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

            if ! inbound.Tokens[0].la_done {
                elsefound, elsedistance, er = lookahead(source_base, parser.pc, 0, 1, C_Else, []uint8{C_If}, []uint8{C_Endif})
                endfound, enddistance, er = lookahead(source_base, parser.pc, 0, 0, C_Endif, []uint8{C_If}, []uint8{C_Endif})
                inbound.Tokens[0].la_else_distance=elsedistance
                inbound.Tokens[0].la_end_distance=enddistance
                inbound.Tokens[0].la_has_else=elsefound
                inbound.Tokens[0].la_done=true
            } else {
                endfound=true; er=false
                elsefound=inbound.Tokens[0].la_has_else
                elsedistance=inbound.Tokens[0].la_else_distance
                enddistance=inbound.Tokens[0].la_end_distance
            }

            if er || !endfound {
                parser.report(inbound.SourceLine,"Missing ENDIF for this IF")
                finish(false, ERR_SYNTAX)
                break
            }

            // eval
            expr, err = parser.Eval(ifs, inbound.Tokens[1:])
            if err!=nil {
                parser.report(inbound.SourceLine,"Could not evaluate expression.")
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

            endfound, enddistance, _ := lookahead(source_base, parser.pc, 1, 0, C_Endif, []uint8{C_If}, []uint8{C_Endif})

            if endfound {
                parser.pc += enddistance
            } else { // this shouldn't ever occur, as endif checked during C_If, but...
                parser.report(inbound.SourceLine, "ELSE without an ENDIF\n")
                finish(false, ERR_SYNTAX)
            }


        case C_Endif:

            // ENDIF *should* just be an end-of-block marker


        default:

            // local command assignment (child/parent process call)

            if inbound.TokenCount > 1 { // ident "=|"
                if inbound.Tokens[0].tokType == Identifier && ( inbound.Tokens[1].tokType == O_AssCommand || inbound.Tokens[1].tokType == O_AssOutCommand ) {
                    if inbound.TokenCount > 2 {

                        // get text after =| or =<
                        var startPos int
                        bc:=basecode[source_base][parser.pc].borcmd

                        switch inbound.Tokens[1].tokType {
                        case O_AssCommand:
                            startPos = str.IndexByte(basecode[source_base][parser.pc].Original, '|') + 1
                            // pf("(debug) ass-command present is : ·%v·\n",basecode[source_base][parser.pc].borcmd)
                        case O_AssOutCommand:
                            startPos = str.IndexByte(basecode[source_base][parser.pc].Original, '<') + 1
                            // pf("(debug) ass-out-command present is : ·%v·\n",basecode[source_base][parser.pc].borcmd)
                        }

                        var cmd string
                        if bc=="" {
                            cmd = interpolate(ifs,ident,basecode[source_base][parser.pc].Original[startPos:])
                        } else {
                            cmd = interpolate(ifs,ident,bc[2:])

                        }

                        cop:=system(cmd,false)
                        lhs_name := inbound.Tokens[0].tokText
                        switch inbound.Tokens[1].tokType {
                        case O_AssCommand:
                            vset(ifs, ident, lhs_name, cop)
                        case O_AssOutCommand:
                            vset(ifs, ident, lhs_name, cop.out)
                        }
                    }
                    // skip normal eval below
                    break
                }
            }

            // try to eval and assign
              //  pf("[act-eval] ifs %d, ident ptr %p\n",ifs,ident)
              //  pf("[act-eval] toks %+v\n",inbound.Tokens)
            if we=parser.wrappedEval(ifs,ident,ifs,ident,inbound.Tokens); we.evalError {
                parser.report(inbound.SourceLine,sf("Error in evaluation\n%+v\n",we.errVal))
                finish(false,ERR_EVAL)
                break
            }

            if interactive && !we.assign && we.result!=nil {
                pf("%+v\n",we.result)
            }

        } // end-statements-case

    } // end-pc-loop

    lastlock.RLock()
    si=sig_int
    lastlock.RUnlock()

    if structMode && !typeInvalid {
        // incomplete struct definition
        pf("Open STRUCT definition %v\n",structName)
        finish(true,ERR_SYNTAX)
    }

    if !si {

        // populate return variable in the caller with retvals
        if retvalues!=nil {
            calllock.Lock()
            calltable[ifs].retvals=retvalues
            calllock.Unlock()
        }

        // clean up

        // pf("Leaving call with ifs of %d [fs:%s]\n\n",ifs,fs)
        // pf("[#2]about to delete %v[#-]\n",fs)
        // pf("about to enter call de-allocation with fs of '%s'\n",fs)

        if !str.HasPrefix(fs,"@mod_") {

            // drop allocated names
            if varmode != MODE_STATIC {
                fnlookup.lmdelete(fs)
                numlookup.lmdelete(ifs)
            }

            // we keep a record here of recently disposed functionspace names
            //  so that mem_summary can label disposed of function allocations.
            lastlock.Lock()
            lastfunc[ifs]=fs
            lastlock.Unlock()

            if ifs>2 {
                fspacelock.Lock()
                functionspaces[ifs] = []Phrase{}
                basecode[ifs] = []BaseCode{}
                fspacelock.Unlock()
            }

        }

    }

    calllock.Lock()
    callChain=callChain[:len(callChain)-1]
    calllock.Unlock()
    // fmt.Printf("Releasing fs %d (%s)\n",ifs,fs)

    return retval_count,endFunc

}

func system(cmd string, display bool) (cop struct{out string; err string; code int; okay bool}) {
    cmd = str.Trim(cmd," \t\n")
    if hasOuter(cmd,'`') { cmd=stripOuter(cmd,'`') }
    cop = Copper(cmd, false)
    if display { pf("%s",cop.out) }
    return cop
}

/// execute a command in the shell coprocess or parent
func coprocCall(parser *leparser, ifs uint32,ident *[szIdent]Variable, s string) {
    cet := ""
    s=str.TrimRight(s,"\n")
    if len(s) > 0 {

        // find index of first pipe, then remove everything upto and including it
        pipepos := str.IndexByte(s, '|')
        cet      = s[pipepos+1:]

        // strip outer quotes
        cet      = str.Trim(cet," \t\n")
        if hasOuter(cet,'`') { cet=stripOuter(cet,'`') }

        inter   := interpolate(ifs,ident,cet)
        cop     := Copper(inter, false)
        if ! cop.okay {
            pf("Error: [%d] in shell command '%s'\n", cop.code, str.TrimLeft(inter," \t"))
            if interactive {
                pf(cop.err)
            }
        } else {
            if len(cop.out) > 0 {
                if cop.out[len(cop.out)-1] != '\n' {
                    cop.out += "\n"
                }
                pf("%s", cop.out)
            }
        }
    }
}



/// print user-defined function definition(s) to stdout
func ShowDef(fn string) bool {

    if str.HasPrefix(fn,"@mod_") {
        return false
    }

    var ifn uint32
    var present bool
    if ifn, present = fnlookup.lmget(fn); !present {
        ifn = calltable[ifn].base
        return false
    }

    if ifn < uint32(len(functionspaces)) {

        var falist []string
        for _,fav:=range functionArgs[ifn].args {
            falist=append(falist,fav)
        }

        first := true

        for q := range functionspaces[ifn] {
            strOut := "\t\t "
            if first == true {
                first = false
                strOut = sf("\n[#4][#bold]%s(%v)[#boff][#-]\n\t\t ", fn, str.Join(falist, ","))
            }
            pf(sparkle(str.ReplaceAll(sf("%s%s\n", strOut, basecode[ifn][q].Original),"%","%%")))
        }
    }
    return true
}


/// search token list for a given delimiter string
func findDelim(tokens []Token, delim uint8, start int16) (pos int16) {
    n:=0
    for p := start; p < int16(len(tokens)); p+=1 {
        if tokens[p].tokType==LParen { n+=1 }
        if tokens[p].tokType==RParen { n-=1 }
        if n==0 && tokens[p].tokType == delim {
            return p
        }
    }
    return -1
}


func (parser *leparser) splitCommaArray(ifs uint32, tokens []Token) (resu [][]Token) {

    evnest:=0
    newstart:=0
    lt:=0

    if lt=len(tokens);lt==0 { return resu }

    for term := range tokens {
        nt:=tokens[term]
        if nt.tokType==LParen { evnest+=1 }
        if nt.tokType==RParen { evnest-=1 }
        if evnest==0 {
            if term==lt-1 {
                v := tokens[newstart:term+1]
                resu=append(resu,v)
                newstart=term+1
                continue
            }
            if nt.tokType == O_Comma {
                v := tokens[newstart:term]
                resu=append(resu,v)
                newstart=term+1
            }
        }
    }
    return resu

}



func (parser *leparser) evalCommaArray(ifs uint32, tokens []Token) (resu []interface{}, errs []error) {

    evnest:=0
    newstart:=0
    lt:=0

    if lt=len(tokens);lt==0 { return resu,errs }

    for term := range tokens {
        nt:=tokens[term]
        if nt.tokType==LParen { evnest+=1 }
        if nt.tokType==RParen { evnest-=1 }
        if evnest==0 {
            if term==lt-1 {
                v, e := parser.Eval(ifs,tokens[newstart:term+1])
                resu=append(resu,v)
                errs=append(errs,e)
                newstart=term+1
                continue
            }
            if nt.tokType == O_Comma {
                v, e := parser.Eval(ifs,tokens[newstart:term])
                resu=append(resu,v)
                errs=append(errs,e)
                newstart=term+1
            }
        }
    }
    return resu,errs

}

// print / println / log handler
// when logging, user must decide for themselves if they want a LF at end.
func (parser *leparser) console_output(tokens []Token,ifs uint32,ident *[szIdent]Variable,interactive bool,lf bool,logging bool) {
    plog_out := ""
    if len(tokens) > 0 {
        evnest:=0
        newstart:=0
        for term := range tokens {
            nt:=tokens[term]
            if nt.tokType==LParen || nt.tokType==LeftSBrace  { evnest+=1 }
            if nt.tokType==RParen || nt.tokType==RightSBrace { evnest-=1 }
            if evnest==0 && (term==len(tokens)-1 || nt.tokType == O_Comma) {
                v, _ := parser.Eval(ifs,tokens[newstart:term+1])
                newstart=term+1
                switch v.(type) { case string: v=interpolate(ifs,ident,v.(string)) }
                if logging {
                    plog_out += sf(`%v`,sparkle(v))
                } else {
                    pf(`%v`,sparkle(v))
                }
                continue
            }
        }
        if logging {
            plog("%v", plog_out)
            return
        }
        if interactiveFeed || lf { pf("\n") }
    } else {
        pf("\n")
    }
}



