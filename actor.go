package main

import (
    "io/ioutil"
    "math"
    "log"
    "os"
    "path/filepath"
    "path"
    "reflect"
    "regexp"
    "sync"
    "sync/atomic"
    "strconv"
    "runtime"
    str "strings"
    "time"
    "unsafe"
)


func task(caller uint32, loc uint32, iargs ...interface{}) <-chan interface{} {
    r:=make(chan interface{})

    go func() {
        defer close(r)
        atomic.AddInt32(&concurrent_funcs, 1)
        rcount,_:=Call(MODE_NEW, loc, ciAsyn, iargs...)

        switch rcount {
        case 0:
            r<-nil
        case 1:
            v,_:=vget(caller,"@#@"+strconv.FormatUint(uint64(loc), 10))
            if v==nil {
                r<-nil
                break
            }
            r<-v.([]interface{})[0]
        default:
            v,_:=vget(caller,"@#@"+strconv.FormatUint(uint64(loc), 10))
            r<-v
        }
        atomic.AddInt32(&concurrent_funcs, -1)
    }()
    return r
}

var atlock = &sync.RWMutex{}
var siglock = &sync.RWMutex{}

// finish : flag the machine state as okay or in error and
// optionally terminates execution.
func finish(hard bool, i int) {
    if hard {
        os.Exit(i)
    }

    if !interactive {
        os.Exit(i)
    }

    siglock.Lock()
    sig_int = true
    siglock.Unlock()

}


// slightly faster string comparison.
// have to use gotos here as loops can't be inlined
// @todo: keep this under review
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
    case int:
        return float64(i), false
    case int64:
        return float64(i), false
    case uint:
        return float64(i), false
    case uint8:
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
func searchToken(base uint32, start int16, end int16, sval string) bool {

    range_fs:=functionspaces[base][start:end]

    for _, v := range range_fs {
        if v.TokenCount == 0 {
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
func lookahead(fs uint32, startLine int16, indent int, endlevel int, term uint8, indenters []uint8, dedenters []uint8) (bool, int16, bool) {

    range_fs:=functionspaces[fs][startLine:]

    if range_fs[0].pairLA !=0 {
        // return cached lookahead
        return true,range_fs[0].pairLA,false
    }

    for i, v := range range_fs {

        if len(v.Tokens) == 0 {
            continue
        }

        // indents and dedents
        if InSlice(v.Tokens[0].tokType, indenters) {
            indent++
        }
        if InSlice(v.Tokens[0].tokType, dedenters) {
            indent--
        }
        if indent < endlevel {
            return false, 0, true
        }

        // found search term?
        if indent == endlevel && v.Tokens[0].tokType == term {
            // cache lookahead then return value
            range_fs[0].pairLA=int16(i)
            return true, int16(i), false
        }
    }

    // return found, distance, nesting_fault_status
    return false, -1, false

}


func Uint32n(maxN uint32) uint32 {
    x := Uint32()
    return uint32((uint64(x) * uint64(maxN)) >> 32)
}

type RNG struct {
    x uint32
}

var rngPool sync.Pool

func (r *RNG) Uint32n(maxN uint32) uint32 {
    x := r.Uint32()
    return uint32((uint64(x) * uint64(maxN)) >> 32)
}


func Uint32() uint32 {
    v := rngPool.Get()
    if v == nil {
        v = &RNG{}
    }
    r := v.(*RNG)
    x := r.Uint32()
    rngPool.Put(r)
    return x
}

func getRandomUint32() uint32 {
    x := time.Now().UnixNano()
    return uint32((x >> 32) ^ x)
}

func (r *RNG) Uint32() uint32 {
    for r.x == 0 {
        r.x = getRandomUint32()
    }
    x := r.x
    x ^= x << 13
    x ^= x >> 17
    x ^= x << 5
    r.x = x
    return x
}

func formatUint32(n uint32) string {
    return strconv.FormatUint(uint64(n), 10)
}

func formatInt32(n int32) string {
    return strconv.FormatInt(int64(n), 10)
}


// find the next available slot for a function or module
//  definition in the functionspace[] list.
func GetNextFnSpace(requiredName string) (uint32,string) {

    calllock.Lock()

    // find highest in list
    top:=uint32(cap(calltable))
    highest:=top-1
    ccap:=uint32(CALL_CAP)
    deallow:=top>uint32(ccap*2)

    for ; highest>(ccap*2) && highest>(top/2)-ccap; highest-- {
        if calltable[highest]!=(call_s{}) { break }
    }

    // de-alloc
    if deallow {
        if highest<((top/2)-(ccap/2)-1) {
            ncs:=make([]call_s,len(calltable)/2,cap(calltable)/2)
            copy(ncs,calltable)
            calltable=ncs
            top=uint32(cap(calltable))
        }
    }

    // we know at this point that if a dealloc occurred then highest was
    // already below new cap and a fresh alloc should not occur below

    for q := uint32(1); q < top+1 ; q++ {

        if numlookup.lmexists(q) {
            continue
        }

        for ; q>=uint32(cap(calltable)) ; {
            ncs:=make([]call_s,len(calltable)*2,cap(calltable)*2)
            copy(ncs,calltable)
            calltable=ncs
        }

        // pf("-- entered reserving code--\n")
        var r RNG
        for  ; ; {

            newName := requiredName

            if newName[len(newName)-1]=='@' {
                newName+=formatUint32(r.Uint32n(1e7))
            }

            if !numlookup.lmexists(q) { // unreserved
                numlookup.lmset(q, newName)
                fnlookup.lmset(newName,q)
                // place a reservation in calltable:
                // if we don't do this, then there is a small chance that the id [q]
                //  will get re-used between the calls to GetNextFnSpace() and Call()
                //  by fast spawning async tasks.
                calltable[q]=call_s{fs:"@@reserved",caller:0,base:0,retvar:""}
                calllock.Unlock()
                return q,newName
            }

        }
    }

    pf("Error: no more function space available.\n")
    finish(true, ERR_FATAL)
    calllock.Unlock()
    return 0, ""
}


// setup mutex locks
var calllock   = &sync.RWMutex{}  // function call related
var lastlock   = &sync.RWMutex{}  // cached globals, loops[] and depth[]
var fspacelock = &sync.RWMutex{}  // token storage related
var farglock   = &sync.RWMutex{}  // function argument related
var globlock   = &sync.RWMutex{}  // generic global related


// identify the source storage id related to a specific instance id
func baseof(fs uint32) (base uint32) {
    calllock.RLock()
    base = calltable[fs].base
    calllock.RUnlock()
    return
}

// for error reporting : keeps a list of parent->child function calls
//   will probably blow up during recursion.

var callChain []chainInfo

// defined function entry point
// everything about what is to be executed is contained in calltable[csloc]
func Call(varmode uint8, csloc uint32, registrant uint8, va ...interface{}) (retval_count uint8,endFunc bool) {

    // register call
    calllock.Lock()
    // pf("Entered call -> %#v : va -> %#v\n",calltable[csloc],va)
    // pf(" with new ifs of -> %v fs-> %v\n",csloc,calltable[csloc].fs)
    caller_str,_:=numlookup.lmget(calltable[csloc].caller)
    callChain=append(callChain,chainInfo{loc:calltable[csloc].caller,name:caller_str,line:calltable[csloc].callline,registrant:registrant})
    calllock.Unlock()

    var inbound *Phrase
    var current_with_handle *os.File

    // set up evaluation parser - one per function
    parser:=&leparser{}

    // error handler
    defer func() {
        if r := recover(); r != nil {
            if _,ok:=r.(runtime.Error); ok {
                parser.report(sf("\n%v\n",r))
                finish(false,ERR_EVAL)
                if debug_level>0 { panic(r) }
            }
            err:=r.(error)
            parser.report(sf("\n%v\n",err))
            setEcho(true)
            finish(false,ERR_EVAL)
        }
    }()

    // some tracking variables for this function call
    var breakIn uint8               // true during transition from break to outer.
    var pc int16                    // program counter
    var retvar string               // variables to allocate return vars to
    var retvalues []interface{}     // return values to be passed back
    var finalline int16             // tracks end of tokens in the function
    var fs string                   // current function space
    var caller uint32               // function space which placed the call
    var base uint32                 // location of the translated source tokens
    var thisLoop *s_loop            // pointer to loop information. used in FOR

    // set up the function space

    // ..get call details
    calllock.RLock()
    ncs := &calltable[csloc]

    // unique name for this execution, pre-generated before call
    fs = (*ncs).fs

    // the source code to be read for this function
    base = (*ncs).base

    // which func id called this code
    caller = (*ncs).caller

    // usually @#, the return variable name
    retvar = (*ncs).retvar

    // the uint32 id attached to fs name
    ifs,_:=fnlookup.lmget(fs)

    calllock.RUnlock()

    // @todo: remove this check at some point - should be redundant
    if base==0 {
        if !interactive {
            parser.report("Possible race condition. Please check. Base->0")
            finish(false,ERR_EVAL)
            return
        }
    }


    // create a local symbol table. this excludes references in strings,
    //  these have to be looked up manually.

    //  this section rewrites the tokens in the base source (one time only)
    //  to a contiguous set of integers which represent the array element
    //  to be looked up in ident[fs][uint16] to locate a Variable{}

    // reset the variable mappings if the source hasn't been parsed yet

    ll:=false
    if atomic.LoadInt32(&concurrent_funcs)>0 { vlock.Lock() ; ll=true }

    if !identParsed[base] && varmode==MODE_NEW {
        functionidents[base]=0
        vmap[base]  =make(map[string]uint16,0)
        unvmap[base]=make(map[uint16]string,0)
    }

    // set the location of the next available slot for new variables
    nextVarId:=uint16(0)
    if varmode==MODE_STATIC {
        nextVarId=functionidents[base]
    }

    // range over all the tokens, adding offsets to the source tokens.
    // this must be done every call for MODE_STATIC, but only if unparsed for MODE_NEW
    if varmode==MODE_STATIC || !identParsed[base] {

        defnest:=0
        for kph,ph:= range functionspaces[base] {
            for kt,t := range ph.Tokens {
                // @ 0 is statement, not subsequent identifier in line.
                if kt==0 && t.tokType==C_Define { defnest++; continue }
                if kt==0 && t.tokType==C_Enddef { defnest--; continue }
                if defnest==0 && t.tokType==Identifier {
                    if pos,found:=vmap[base][t.tokText] ; found {
                        // replace token
                        t.offset=pos
                        vmap[base][t.tokText]=pos
                        unvmap[base][pos]=t.tokText
                        // pf("act-stat-replace base %d : %s pos %d\n",base,t.tokText,pos)
                    } else {
                        // append token
                        t.offset=nextVarId
                        vmap[base][t.tokText]=nextVarId
                        unvmap[base][nextVarId]=t.tokText
                        // pf("act-stat-add base %d : %s pos %d\n",base,t.tokText,nextVarId)
                        nextVarId++
                    }
                    functionspaces[base][kph].Tokens[kt]=t
                }
            }
        }

        if defnest!=0 {
            parser.report("definition nesting error!")
            if ll { vlock.Unlock() }
            finish(true,ERR_SYNTAX)
            return
        }
        identParsed[base]=true

        //if varmode==MODE_NEW {
            // set the base index for variables in new instances of this base
            functionidents[base]=nextVarId
        //}

    } else {
        nextVarId=functionidents[base]
    }

    if ll { vlock.Unlock() }

    // in source vars processed, can now reserve a minimum space quota
    //  for this instance of the routine.

    if varmode==MODE_NEW {
        // create the local variable storage for the function

        ll=false
        if atomic.LoadInt32(&concurrent_funcs)>0 { vlock.RLock() ; ll=true }
        var vtm uint32
        vtm=vtable_maxreached
        minvar:=nextVarId
        if ll { vlock.RUnlock() }

        if VAR_CAP>minvar { minvar=VAR_CAP }
        if ifs>=vtm {
            vcreatetable(ifs, &vtable_maxreached, minvar)
            // pf("-- Created variable table [ifs:%d] with length of %d\n",ifs,minvar)
        } else {
            // reset existing ifs storage area
            ll=false
            if atomic.LoadInt32(&concurrent_funcs)>0 { vlock.Lock() ; ll=true }
            identResize(ifs,0)
            if ll { vlock.Unlock() }
        }

        globlock.Lock()
        test_group = ""
        test_name = ""
        test_assert = ""
        globlock.Unlock()

        // copy the base var mapping to this instance
        ll=false
        if atomic.LoadInt32(&concurrent_funcs)>0 { vlock.Lock() ; ll=true }

        unvmap[ifs] =make(map[uint16]string,len(unvmap[base]))
        vmap[ifs]   =make(map[string]uint16,len(unvmap[base]))
        for e:=uint16(0); e<uint16(len(unvmap[base])); e++ {
            unvmap[ifs][e] = unvmap[base][e]
            vmap  [ifs][unvmap[base][e]] = vmap[base][unvmap[base][e]]
        }
        if ll { vlock.Unlock() }

        // add the call parameters as available variable mappings
        //  to the current function call
        farglock.RLock()
        for e:=0; e<len(va); e++ {
            nextFaArg:=functionArgs[base].args[e]
            if vi,found:=vmap[ifs][nextFaArg] ; found {
                vseti(ifs,nextFaArg,vi,va[e])
                // pf("On entry to %v set local var %v to [%T] %+v\n", fs, nextFaArg, va[e], va[e])
            } else {
                vseti(ifs,nextFaArg,nextVarId,va[e])
                // pf("On entry to %v created local var %v with value [%T] %+v\n", fs, nextFaArg, va[e], va[e])
                nextVarId++
            }
        }
        farglock.RUnlock()

        ll=false
        if atomic.LoadInt32(&concurrent_funcs)>0 { vlock.Lock() ; ll=true }
        functionidents[ifs]=nextVarId
        if ll { vlock.Unlock() }

    }


    // missing varargs in call result in empty string assignments:
    farglock.RLock()
    if len(functionArgs[base].args)>len(va) {
        for e:=0; e<(len(functionArgs[base].args)-len(va)); e++ {
            va=append(va,nil) // "")
        }
    }
    farglock.RUnlock()


    if varmode == MODE_NEW {

        // in interactive mode, the top-level current functionspace is 0
        // in normal exec mode, the source is treated as functionspace 1
        if base < 2 {
            globalaccess = ifs
            vset(globalaccess, "trapInt", "")
        }

        // nesting levels in this function

        lastlock.Lock()
        depth[ifs] = 0
        lastConstruct[ifs] = []uint8{}
        lastlock.Unlock()

        vset(ifs,"@in_tco",false)

    }

    // initialise condition states: WHEN stack depth
    // initialise the loop positions: FOR, FOREACH, WHILE

    lastlock.Lock()

    // active WHEN..ENDWHEN statement meta info
    var wc = make([]whenCarton, WHEN_CAP)

    // count of active WHEN..ENDWHEN statements
    var wccount int

    // allocate loop storage space if not a repeat ifs value.

    ll=false
    if atomic.LoadInt32(&concurrent_funcs)>0 { vlock.RLock() ; ll=true }

    var top,highest,lscap uint32

    top=uint32(cap(loops))
    highest=top
    lscap=LOOP_START_CAP
    deallow:=top>uint32(lscap*2)

    for q:=top-1; q>(lscap*2) && q>(top/2)-lscap; q-- {
        if loops[q]!=nil { highest=q; break }
    }

    // dealloc
    if deallow {
        if highest<((top/2)-(lscap/2)-1) {
            nloops:=make([][]s_loop,len(loops)/2,cap(loops)/2)
            copy(nloops,loops)
            loops=nloops
            top=uint32(cap(loops))
        }
    }

    for ; ifs>=uint32(cap(loops)) ; {
            // realloc with increased cap
            nloops:=make([][]s_loop,len(loops)*2,cap(loops)*2)
            copy(nloops,loops)
            loops=nloops
    }

    loops[ifs] = make([]s_loop, MAX_LOOPS)

    if ll { vlock.RUnlock() }
    lastlock.Unlock()


tco_reentry:

    // assign value to local vars named in functionArgs (the call parameters)
    //  from each va value.
    // - functionArgs[] created at definition time from the call signature

    farglock.RLock()
    if len(va) > 0 {
        for q, v := range va {
            fa:=functionArgs[base].args[q]
            // pf("-- setting va-to-var variable %s with %+v\n",fa,v)
            vset(ifs,fa,v)
        }
    }
    farglock.RUnlock()

    if len(functionspaces[base])>32767 {
        parser.report("function too long!")
        finish(true,ERR_SYNTAX)
        return
    }

    finalline = int16(len(functionspaces[base]))

    inside_test := false      // are we currently inside a test bock
    inside_with := false      // WITH cannot be nested and remains local in scope.

    var structMode bool       // are we currently defining a struct
    var structName string     // name of struct currently being defined
    var structNode []interface{}   // struct builder
    var defining bool         // are we currently defining a function. takes priority over structmode.
    var definitionName string // ... if we are, what is it called

    pc = -1                   // program counter : increments to zero at start of loop

    var si bool
    var statement *Token
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
        //  i guess it isn't optimising away at compile-time as easily as it should.

        // pc += 1  // optimises correctly, but no better than explicit inc below

        pc = pc + 1             // program counter, equates to each Phrase struct in the function
        parser.stmtline=pc      // reflects the pc for use in the evaluator

        if pc >= finalline || endFunc || sig_int {
            break
        }

        // get the next Phrase
        inbound = &functionspaces[base][pc]

                    //
     ondo_reenter:  // on..do re-enters here because it creates the new phrase in advance and
                    //  we want to leave the program counter unaffected.


        parser.line=inbound.SourceLine

        /////////////////////////////////////////////////////////////////////////

        // finally... start processing the statement.

        statement = &inbound.Tokens[0]

        // .. skip comments and DOC statements
        if statement.tokType == C_Doc && !testMode {
            continue
        }

        /////// LINE ////////////////////////////////////////////////////////////
            // pf("(%20s) [#b7][#2]%5d : %+v[##][#-]\n",fs,pc,inbound.Tokens)
        /////////////////////////////////////////////////////////////////////////

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

            // check for default value assignment:
            var eqPos int16
            var hasValue bool
            for eqPos=2;eqPos<inbound.TokenCount;eqPos++ {
                if inbound.Tokens[eqPos].tokType==O_Assign {
                    hasValue=true
                    break
                }
            }

            var default_value ExpressionCarton
            if hasValue {
                default_value = parser.wrappedEval(ifs,ifs,inbound.Tokens[eqPos+1:])
                // pf(" : set default_value in hasValue ( %#v )\n",default_value)
                if default_value.evalError {
                    parser.report(sf("Invalid default value in STRUCT '%s'",statement.tokText))
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
            case "int","float","string","bool","uint","uint8","byte","mixed","[]":
            default:
                parser.report(sf("Invalid type in STRUCT '%s'",cet.text))
                finish(false,ERR_SYNTAX)
                typeInvalid=true
                break
            }

            if typeInvalid {
                break
            }

            structNode=append(structNode,statement.tokText,cet.text,hasValue,default_value.result)
            // pf("current struct node build at :\n%#v\n",structNode)

            continue
        }

        // abort this phrase if currently inside a TEST block but the test flag is not set.
        if inside_test {
            if statement.tokType != C_Endtest && !under_test {
                continue
            }
        }


        ////////////////////////////////////////////////////////////////
        // BREAK here if required
        if breakIn != Error {
            // breakIn holds either Error or a token_type for ending the current construct
            if statement.tokType != breakIn {
                continue
            }
        }
        ////////////////////////////////////////////////////////////////


        // main parsing for statements starts here:

        switch statement.tokType {

        case C_Var: // permit declaration with a default value

            // expand to this:
            // 'VAR' name1 [ ',' ... nameX ] [ '[' [size] ']' ] type [ '=' expr ]

            //  and var ary_s []struct_name

            var name_list []string
            var expectingComma bool
            var flagVarSynError bool
            var c int16

          var_comma_loop:
            for c=int16(1); c<inbound.TokenCount; c++ {
                switch inbound.Tokens[c].tokType {
                case Identifier:
                    if expectingComma { // syntax error
                        break var_comma_loop
                    }
                    name_list=append(name_list,inbound.Tokens[c].tokText)
                case O_Comma:
                    if !expectingComma { // syntax error
                        flagVarSynError=true
                        break var_comma_loop
                    }
                default:
                    break var_comma_loop
                } 
                expectingComma=!expectingComma
            }

            if len(name_list)==0 {
                flagVarSynError=true
            }

            // set eqpos to either location of first equals sign
            var eqPos int16
            var hasEqu bool
            for eqPos=c; eqPos<inbound.TokenCount; eqPos++ {
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

            if !flagVarSynError {
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
                        se := parser.wrappedEval(ifs,ifs,inbound.Tokens[c+1:d])
                        if se.evalError {
                            parser.report("could not evaluate size expression in VAR")
                            finish(false,ERR_EVAL)
                            break
                        }
                        switch se.result.(type) {
                        case int:
                            size=se.result.(int)
                        default:
                            parser.report("size expression must evaluate to an integer")
                            finish(false,ERR_EVAL)
                            break
                        }
                    }
                }

                // pf("hasAry?  %v\n",hasAry)
                // pf("size is  %d\n",size)

            } else {
                parser.report("invalid VAR syntax\nUsage: VAR varname1 [#i1][,...varnameX][#i0] [#i1][optional_size][#i0] type [#i1][=expression][#i0]")
                finish(false,ERR_SYNTAX)
            }

            if flagVarSynError {
                break
            }

            // eval the terms to assign to new vars
            hasValue := false
            if hasEqu {
                hasValue=true
                we = parser.wrappedEval(ifs,ifs,inbound.Tokens[eqPos+1:])
                if we.evalError {
                    parser.report("could not evaluate VAR assignment expression")
                    finish(false,ERR_EVAL)
                    break
                }
            }

                // this needs reworking:
                //   if we pulled them out to global scope then will cause parallelism problems

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
                typemap["[]mixed"]  = reflect.TypeOf(stmixed)
                typemap["[]"]       = reflect.TypeOf(stmixed)
                typemap["assoc"]    = nil
                // --

            // name iterations

            for _,vname:=range name_list {

                var vi uint16
                var there bool

                if vi,there=VarLookup(ifs,vname); there {
                    ll:=false
                    if atomic.LoadInt32(&concurrent_funcs)>0 { vlock.RLock() ; ll=true }
                    // if !interactive && ident[ifs][vi].declared==true {
                    if vi>0 && ident[ifs][vi].declared {
                        if ll { vlock.RUnlock() }
                        parser.report(sf("variable '%s' already exists",vname))
                        finish(false, ERR_SYNTAX)
                        break
                    } 
                    if ll { vlock.RUnlock() }
                }

                // get the required type
                type_token_string := inbound.Tokens[eqPos-1].tokText
                new_type_token_string := type_token_string
                if hasAry {
                    if type_token_string!="[]" {
                        new_type_token_string="[]"+type_token_string
                    }
                }

                // declaration and initialisation
                if _,found:=typemap[new_type_token_string]; found {

                    if new_type_token_string!="assoc" {
                        vset(ifs,vname,reflect.New(typemap[new_type_token_string]).Elem().Interface())
                    }

                    ll:=false
                    if atomic.LoadInt32(&concurrent_funcs)>0 { vlock.Lock() ; ll=true }

                    ident[ifs][vi].ITyped=true
                    ident[ifs][vi].declared=true

                    switch new_type_token_string {
                    case "nil":
                        ident[ifs][vi].IKind=knil
                    case "bool":
                        ident[ifs][vi].IKind=kbool
                    case "int":
                        ident[ifs][vi].IKind=kint
                    case "uint":
                        ident[ifs][vi].IKind=kuint
                    case "float":
                        ident[ifs][vi].IKind=kfloat
                    case "string":
                        ident[ifs][vi].IKind=kstring
                    case "uint8","byte":
                        ident[ifs][vi].IKind=kbyte
                    case "[]bool":
                        ident[ifs][vi].IKind=ksbool
                        ident[ifs][vi].IValue=make([]bool,size,size)
                    case "[]int":
                        ident[ifs][vi].IKind=ksint
                        ident[ifs][vi].IValue=make([]int,size,size)
                    case "[]uint":
                        ident[ifs][vi].IKind=ksuint
                        ident[ifs][vi].IValue=make([]uint,size,size)
                    case "[]float":
                        ident[ifs][vi].IKind=ksfloat
                        ident[ifs][vi].IValue=make([]float64,size,size)
                    case "[]string":
                        ident[ifs][vi].IKind=ksstring
                        ident[ifs][vi].IValue=make([]string,size,size)
                    case "[]byte","[]uint8":
                        ident[ifs][vi].IKind=ksbyte
                        ident[ifs][vi].IValue=make([]uint8,size,size)
                    case "[]","[]mixed":
                        ident[ifs][vi].IKind=ksany
                        ident[ifs][vi].IValue=make([]interface{},size,size)
                    case "assoc":
                        ident[ifs][vi].IKind=kmap
                        ident[ifs][vi].IValue=make(map[string]interface{},size)
                    }

                    // if we had a default value, stuff it in here...
                    if new_type_token_string!="assoc" && hasValue {
                        if sf("%T",we.result)!=new_type_token_string {
                            parser.report("type mismatch in VAR assignment")
                            finish(false,ERR_EVAL)
                            if ll { vlock.Unlock() }
                            break
                        }
                        ident[ifs][vi].IValue=we.result
                    }
                    if ll { vlock.Unlock() }

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

                        ll:=false
                        if atomic.LoadInt32(&concurrent_funcs)>0 { vlock.Lock() ; ll=true }

                        // deal with var name [n]struct_type
                        if len(structvalues)>0 {
                            var sfields []reflect.StructField
                            offset:=uintptr(0)
                            for svpos:=0; svpos<len(structvalues); svpos+=4 {
                                // structvalues: [0] name [1] type [2] boolhasdefault [3] default_value
                                nv :=structvalues[svpos].(string)
                                nt :=structvalues[svpos+1].(string)
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

                            ident[ifs][vi].ITyped=false
                            ident[ifs][vi].declared=true


                            if !hasAry {
                                // default values setting:

                                val:=reflect.ValueOf(v)
                                // typ:=reflect.ValueOf(v).Type()
                                tmp:=reflect.New(val.Type()).Elem()
                                tmp.Set(val)

                                allSet:=true

                                for svpos:=0; svpos<len(structvalues); svpos+=4 {
                                    // structvalues: [0] name [1] type [2] boolhasdefault [3] default_value
                                    nv :=structvalues[svpos].(string)
                                    nhd:=structvalues[svpos+2].(bool)
                                    ndv:=structvalues[svpos+3]
                                    // pf("nv : %v  hasdef : %v  value : %v\n",nv,nhd,ndv)
                                    if nhd {
                                        var intyp reflect.Type
                                        if ndv!=nil { intyp=reflect.ValueOf(ndv).Type() }

                                        tf:=tmp.FieldByName(nv)
                                        if intyp.AssignableTo(tf.Type()) {
                                            tf=reflect.NewAt(tf.Type(),unsafe.Pointer(tf.UnsafeAddr())).Elem()
                                            tf.Set(reflect.ValueOf(ndv))
                                        } else {
                                            parser.report(sf("cannot set field default (%T) for %v (%v)",ndv,nv,tf.Type()))
                                            finish(false,ERR_EVAL)
                                            allSet=false
                                            break
                                        }
                                    }
                                }

                                if allSet {
                                    ident[ifs][vi].IValue=tmp.Interface()
                                }

                            } else {
                                // don't do this for now, as Za slices are currently just []interface{}:
                                // slice:=reflect.MakeSlice(reflect.SliceOf(new_struct),0,size)
                                // ident[ifs][vi].IValue=slice.Interface()
                                ident[ifs][vi].IValue=[]interface{}{}
                            }

                        } // end-len>0

                        if ll { vlock.Unlock() }

                    } else {
                        parser.report(sf("unknown data type requested '%v'",type_token_string))
                        finish(false, ERR_SYNTAX)
                        break
                    }

                } // end-type-or-struct

            } // end-of-name-list


        case C_While:

            var endfound bool
            var enddistance int16

            endfound, enddistance, _ = lookahead(base, pc, 0, 0, C_Endwhile, []uint8{C_While}, []uint8{C_Endwhile})
            if !endfound {
                parser.report("could not find an ENDWHILE")
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

                we = parser.wrappedEval(ifs,ifs,etoks)
                if we.evalError {
                    parser.report("could not evaluate WHILE condition")
                    finish(false,ERR_EVAL)
                    break
                }

                switch we.result.(type) {
                case bool:
                    res = we.result.(bool)
                default:
                    parser.report("WHILE condition must evaluate to boolean")
                    finish(false,ERR_EVAL)
                    break
                }

            }

            if isBool(res) && res {
                // while cond is true, stack, then continue loop
                lastlock.Lock()
                depth[ifs]++
                loops[ifs][depth[ifs]] = s_loop{repeatFrom: pc, whileContinueAt: pc + enddistance, repeatCond: etoks, loopType: C_While}
                lastConstruct[ifs] = append(lastConstruct[ifs], C_While)
                lastlock.Unlock()
                break
            } else {
                // -> endwhile
                pc += enddistance
            }


        case C_Endwhile:

            // re-evaluate, on true jump back to start, on false, destack and continue

            lastlock.Lock()
            cond := loops[ifs][depth[ifs]]

            if cond.loopType != C_While {
                parser.report("ENDWHILE outside of WHILE loop.")
                finish(false, ERR_SYNTAX)
                lastlock.Unlock()
                break
            }

            // time to die?
            if breakIn == C_Endwhile {
                lastConstruct[ifs] = lastConstruct[ifs][:depth[ifs]-1]
                depth[ifs]--
                breakIn = Error
                lastlock.Unlock()
                break
            }

            // evaluate condition
            we = parser.wrappedEval(ifs,ifs,cond.repeatCond)
            if we.evalError {
                parser.report(sf("eval fault in ENDWHILE\n%+v\n",we.errVal))
                finish(false,ERR_EVAL)
                lastlock.Unlock()
                break
            }

            if we.result.(bool) {
                // while still true, loop
                pc = cond.repeatFrom
            } else {
                // was false, so leave the loop
                lastConstruct[ifs] = lastConstruct[ifs][:depth[ifs]-1]
                depth[ifs]--
            }

            lastlock.Unlock()



        case C_SetGlob: // set the value of a global variable.

           if inbound.TokenCount<3 {
                parser.report("missing value in setglob.")
                finish(false,ERR_SYNTAX)
                break
            }

            if res:=parser.wrappedEval(globalaccess,ifs,inbound.Tokens[1:]); res.evalError {
                parser.report(sf("Error in SETGLOB evaluation\n%+v\n",res.errVal))
                finish(false,ERR_EVAL)
                break
            }


        case C_Foreach:

            // FOREACH var IN expr
            // iterates over the result of expression expr as a list

            if inbound.TokenCount<4 {
                parser.report("bad argument length in FOREACH.")
                finish(false,ERR_SYNTAX)
                break
            }

            if str.ToLower(inbound.Tokens[2].tokText) != "in" {
                parser.report("malformed FOREACH statement.")
                finish(false, ERR_SYNTAX)
                break
            }

            if inbound.Tokens[1].tokType != Identifier {
                parser.report("parameter 2 must be an identifier.")
                finish(false, ERR_SYNTAX)
                break
            }

            var condEndPos int

            fid := inbound.Tokens[1].tokText
            fno := inbound.Tokens[1].offset

            switch inbound.Tokens[3].tokType {

            // cause evaluation of all terms following IN
            case O_InFile, NumericLiteral, StringLiteral, LeftSBrace, LParen, Identifier:

                we = parser.wrappedEval(ifs,ifs, inbound.Tokens[3:])
                if we.evalError {
                    parser.report(sf("error evaluating term in FOREACH statement '%v'\n%+v\n",we.text,we.errVal))
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
                case []interface{}:
                    l=len(lv)
                default:
                    // pf("Unknown loop type [%T]\n",lv)
                }

                if l==0 {
                    // skip empty expressions
                    endfound, enddistance, _ := lookahead(base, pc, 0, 0, C_Endfor, []uint8{C_For,C_Foreach}, []uint8{C_Endfor})
                    if !endfound {
                        parser.report("Cannot determine the location of a matching ENDFOR.")
                        finish(false, ERR_SYNTAX)
                        break
                    } else { //skip
                        pc += enddistance
                        break
                    }
                }

                var iter *reflect.MapIter
                var fkno uint16

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
                        fkno=vset(ifs, "key_"+fid, 0)
                        vseti(ifs, fid, fno, we.result.([]string)[0])
                        condEndPos = len(we.result.([]string)) - 1
                    }

                case map[string]float64:
                    if len(we.result.(map[string]float64)) > 0 {
                        // get iterator for this map
                        iter = reflect.ValueOf(we.result.(map[string]float64)).MapRange()

                        // set initial key and value
                        if iter.Next() {
                            fkno=vset(ifs, "key_"+fid, iter.Key().String())
                            vseti(ifs, fid, fno, iter.Value().Interface())
                        }
                        condEndPos = len(we.result.(map[string]float64)) - 1
                    }

                case map[string]alloc_info:
                    if len(we.result.(map[string]alloc_info)) > 0 {
                        iter = reflect.ValueOf(we.result.(map[string]alloc_info)).MapRange()
                        if iter.Next() {
                            fkno=vset(ifs, "key_"+fid, iter.Key().String())
                            vseti(ifs, fid, fno, iter.Value().Interface())
                        }
                        condEndPos = len(we.result.(map[string]alloc_info)) - 1
                    }

                case map[string]bool:
                    if len(we.result.(map[string]bool)) > 0 {
                        iter = reflect.ValueOf(we.result.(map[string]bool)).MapRange()
                        if iter.Next() {
                            fkno=vset(ifs, "key_"+fid, iter.Key().String())
                            vseti(ifs, fid, fno, iter.Value().Interface())
                        }
                        condEndPos = len(we.result.(map[string]bool)) - 1
                    }

                case map[string]uint:
                    if len(we.result.(map[string]uint)) > 0 {
                        iter = reflect.ValueOf(we.result.(map[string]uint)).MapRange()
                        if iter.Next() {
                            fkno=vset(ifs, "key_"+fid, iter.Key().String())
                            vseti(ifs, fid, fno, iter.Value().Interface())
                        }
                        condEndPos = len(we.result.(map[string]uint)) - 1
                    }

                case map[string]int:
                    if len(we.result.(map[string]int)) > 0 {
                        // get iterator for this map
                        iter = reflect.ValueOf(we.result.(map[string]int)).MapRange()

                        // set initial key and value
                        if iter.Next() {
                            fkno=vset(ifs, "key_"+fid, iter.Key().String())
                            vseti(ifs, fid, fno, iter.Value().Interface())
                        }
                        condEndPos = len(we.result.(map[string]int)) - 1
                    }

                case map[string]string:

                    if len(we.result.(map[string]string)) > 0 {

                        // get iterator for this map
                        iter = reflect.ValueOf(we.result.(map[string]string)).MapRange()
                        // set initial key and value
                        if iter.Next() {
                            fkno=vset(ifs, "key_"+fid, iter.Key().String())
                            vseti(ifs, fid, fno, iter.Value().Interface())
                        }
                        condEndPos = len(we.result.(map[string]string)) - 1
                    }

                case map[string][]string:

                    if len(we.result.(map[string][]string)) > 0 {

                        // get iterator for this map
                        iter = reflect.ValueOf(we.result.(map[string][]string)).MapRange()

                        // set initial key and value
                        if iter.Next() {
                            fkno=vset(ifs, "key_"+fid, iter.Key().String())
                            vseti(ifs, fid, fno, iter.Value().Interface())
                        }
                        condEndPos = len(we.result.(map[string][]string)) - 1
                    }

                case []float64:

                    if len(we.result.([]float64)) > 0 {
                        fkno=vset(ifs, "key_"+fid, 0)
                        vseti(ifs, fid, fno, we.result.([]float64)[0])
                        condEndPos = len(we.result.([]float64)) - 1
                    }

                case float64: // special case: float
                    we.result = []float64{we.result.(float64)}
                    if len(we.result.([]float64)) > 0 {
                        fkno=vset(ifs, "key_"+fid, 0)
                        vseti(ifs, fid, fno, we.result.([]float64)[0])
                        condEndPos = len(we.result.([]float64)) - 1
                    }

                case []uint:
                    if len(we.result.([]uint)) > 0 {
                        fkno=vset(ifs, "key_"+fid, 0)
                        vseti(ifs, fid, fno, we.result.([]uint)[0])
                        condEndPos = len(we.result.([]uint)) - 1
                    }

                case []bool:
                    if len(we.result.([]bool)) > 0 {
                        fkno=vset(ifs, "key_"+fid, 0)
                        vseti(ifs, fid, fno, we.result.([]bool)[0])
                        condEndPos = len(we.result.([]bool)) - 1
                    }

                case []int:
                    if len(we.result.([]int)) > 0 {
                        fkno=vset(ifs, "key_"+fid, 0)
                        vseti(ifs, fid, fno, we.result.([]int)[0])
                        condEndPos = len(we.result.([]int)) - 1
                    }

                case int: // special case: int
                    we.result = []int{we.result.(int)}
                    if len(we.result.([]int)) > 0 {
                        fkno=vset(ifs, "key_"+fid, 0)
                        vseti(ifs, fid, fno, we.result.([]int)[0])
                        condEndPos = len(we.result.([]int)) - 1
                    }

                case []string:
                    if len(we.result.([]string)) > 0 {
                        fkno=vset(ifs, "key_"+fid, 0)
                        vseti(ifs, fid, fno, we.result.([]string)[0])
                        condEndPos = len(we.result.([]string)) - 1
                    }

                case []dirent:
                    if len(we.result.([]dirent)) > 0 {
                        fkno=vset(ifs, "key_"+fid, 0)
                        vseti(ifs, fid, fno, we.result.([]dirent)[0])
                        condEndPos = len(we.result.([]dirent)) - 1
                    }

                case []alloc_info:
                    if len(we.result.([]alloc_info)) > 0 {
                        fkno=vset(ifs, "key_"+fid, 0)
                        vseti(ifs, fid, fno, we.result.([]alloc_info)[0])
                        condEndPos = len(we.result.([]alloc_info)) - 1
                    }

                case []map[string]interface{}:

                    if len(we.result.([]map[string]interface{})) > 0 {
                        fkno=vset(ifs, "key_"+fid, 0)
                        vseti(ifs, fid, fno, we.result.([]map[string]interface{})[0])
                        condEndPos = len(we.result.([]map[string]interface{})) - 1
                    }

                case map[string]interface{}:

                    if len(we.result.(map[string]interface{})) > 0 {

                        // get iterator for this map
                        iter = reflect.ValueOf(we.result.(map[string]interface{})).MapRange()

                        // set initial key and value
                        if iter.Next() {
                            fkno=vset(ifs, "key_"+fid, iter.Key().String())
                            vseti(ifs, fid, fno, iter.Value().Interface())
                        }
                        condEndPos = len(we.result.(map[string]interface{})) - 1
                    }

                case []interface{}:

                    if len(we.result.([]interface{})) > 0 {
                        fkno=vset(ifs, "key_"+fid, 0)
                        vseti(ifs, fid, fno, we.result.([]interface{})[0])
                        condEndPos = len(we.result.([]interface{})) - 1
                    }

                default:
                    parser.report(sf("Mishandled return of type '%T' from FOREACH expression '%v'\n", we.result,we.result))
                    finish(false,ERR_EVAL)
                    break
                }


                // figure end position
                endfound, enddistance, _ := lookahead(base, pc, 0, 0, C_Endfor, []uint8{C_For,C_Foreach}, []uint8{C_Endfor})
                if !endfound {
                    parser.report("Cannot determine the location of a matching ENDFOR.")
                    finish(false, ERR_SYNTAX)
                    break
                }

                lastlock.Lock()

                depth[ifs]++
                lastConstruct[ifs] = append(lastConstruct[ifs], C_Foreach)

                loops[ifs][depth[ifs]] = s_loop{loopVar: fid, varoffset:fno, varkeyoffset:fkno,
                    optNoUse: Opt_LoopStart,
                    repeatFrom: pc + 1, iterOverMap: iter, iterOverArray: we.result,
                    counter: 0, condEnd: condEndPos, forEndPos: enddistance + pc,
                    loopType: C_Foreach,
                }

                lastlock.Unlock()

            }


        case C_For: // loop over an int64 range

            if inbound.TokenCount < 5 || inbound.Tokens[2].tokText != "=" {
                parser.report("Malformed FOR statement.")
                finish(false, ERR_SYNTAX)
                break
            }

            toAt := findDelim(inbound.Tokens, C_To, 2)
            if toAt == -1 {
                parser.report("TO not found in FOR")
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
                    parser.report("Could not evaluate start expression in FOR")
                    finish(false, ERR_EVAL)
                    break
                }
            } else {
                parser.report("Missing expression in FOR statement?")
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
                    parser.report("Could not evaluate end expression in FOR")
                    finish(false, ERR_EVAL)
                    break
                }
            } else {
                parser.report("Missing expression in FOR statement?")
                finish(false,ERR_SYNTAX)
                break
            }

            if stepped {
                if inbound.TokenCount>stepAt+1 {
                    expr, err = parser.Eval(ifs, inbound.Tokens[stepAt+1:])
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
                parser.report("This is a road to nowhere. (STEP==0)")
                finish(true, ERR_EVAL)
                break
            }

            direction := ACT_INC
            if step < 0 {
                direction = ACT_DEC
            }

            // figure end position
            endfound, enddistance, _ := lookahead(base, pc, 0, 0, C_Endfor, []uint8{C_For,C_Foreach}, []uint8{C_Endfor})
            if !endfound {
                parser.report("Cannot determine the location of a matching ENDFOR.")
                finish(false, ERR_SYNTAX)
                break
            }

            // @note: if loop counter is never used between here and
            //  C_Endfor, then don't vset the local var

            // store loop data
            inter:=inbound.Tokens[1].tokText
            fno:=inbound.Tokens[1].offset

            lastlock.Lock()

            depth[ifs]++
            loops[ifs][depth[ifs]] = s_loop{
                loopVar:  inter,
                varoffset: fno,
                optNoUse: Opt_LoopStart,
                loopType: C_For, forEndPos: pc + enddistance, repeatFrom: pc + 1,
                counter: fstart, condEnd: fend,
                repeatAction: direction, repeatActionStep: step,
            }

            // store loop start condition
            vseti(ifs, inter, fno, fstart)

            lastConstruct[ifs] = append(lastConstruct[ifs], C_For)

            lastlock.Unlock()

            // make sure start is not more than end, if it is, send it to the endfor
            switch direction {
            case ACT_INC:
                if fstart>fend {
                    pc=pc+enddistance-1
                    break
                }
            case ACT_DEC:
                if fstart<fend {
                    pc=pc+enddistance-1
                    break
                }
            }


        case C_Endfor: // terminate a FOR or FOREACH block

            ll:=false
            if atomic.LoadInt32(&concurrent_funcs)>0 { lastlock.Lock() ; ll=true }

            //.. take address of loop info store entry
            thisLoop = &loops[ifs][depth[ifs]]

            if (*thisLoop).optNoUse == Opt_LoopStart {
                if lastConstruct[ifs][depth[ifs]-1]!=C_Foreach && lastConstruct[ifs][depth[ifs]-1]!=C_For {
                    parser.report("ENDFOR without a FOR or FOREACH")
                    finish(false,ERR_SYNTAX)
                    if ll { lastlock.Unlock() }
                    break
                }
            }

            var loopEnd bool

            // perform cond action and check condition

            if breakIn!=C_Endfor {

                switch (*thisLoop).loopType {

                case C_Foreach: // move through range

                    (*thisLoop).counter++

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
                                vseti(ifs, "key_"+(*thisLoop).loopVar, (*thisLoop).varkeyoffset, (*thisLoop).iterOverMap.Key().String())
                                vseti(ifs, (*thisLoop).loopVar, (*thisLoop).varoffset, (*thisLoop).iterOverMap.Value().Interface())
                            }

                        case []bool:
                            vseti(ifs, "key_"+(*thisLoop).loopVar, (*thisLoop).varkeyoffset, (*thisLoop).counter)
                            vseti(ifs, (*thisLoop).loopVar, (*thisLoop).varoffset, (*thisLoop).iterOverArray.([]bool)[(*thisLoop).counter])
                        case []int:
                            vseti(ifs, "key_"+(*thisLoop).loopVar, (*thisLoop).varkeyoffset, (*thisLoop).counter)
                            vseti(ifs, (*thisLoop).loopVar, (*thisLoop).varoffset, (*thisLoop).iterOverArray.([]int)[(*thisLoop).counter])
                        case []uint:
                            vseti(ifs, "key_"+(*thisLoop).loopVar, (*thisLoop).varkeyoffset, (*thisLoop).counter)
                            vseti(ifs, (*thisLoop).loopVar, (*thisLoop).varoffset, (*thisLoop).iterOverArray.([]uint8)[(*thisLoop).counter])
                        case []string:
                            vseti(ifs, "key_"+(*thisLoop).loopVar, (*thisLoop).varkeyoffset, (*thisLoop).counter)
                            vseti(ifs, (*thisLoop).loopVar, (*thisLoop).varoffset, (*thisLoop).iterOverArray.([]string)[(*thisLoop).counter])
                        case []dirent:
                            vseti(ifs, "key_"+(*thisLoop).loopVar, (*thisLoop).varkeyoffset, (*thisLoop).counter)
                            vseti(ifs, (*thisLoop).loopVar, (*thisLoop).varoffset, (*thisLoop).iterOverArray.([]dirent)[(*thisLoop).counter])
                        case []alloc_info:
                            vseti(ifs, "key_"+(*thisLoop).loopVar, (*thisLoop).varkeyoffset, (*thisLoop).counter)
                            vseti(ifs, (*thisLoop).loopVar, (*thisLoop).varoffset, (*thisLoop).iterOverArray.([]alloc_info)[(*thisLoop).counter])
                        case []float64:
                            vseti(ifs, "key_"+(*thisLoop).loopVar, (*thisLoop).varkeyoffset, (*thisLoop).counter)
                            vseti(ifs, (*thisLoop).loopVar, (*thisLoop).varoffset, (*thisLoop).iterOverArray.([]float64)[(*thisLoop).counter])
                        case []map[string]interface{}:
                            vseti(ifs, "key_"+(*thisLoop).loopVar, (*thisLoop).varkeyoffset, (*thisLoop).counter)
                            vseti(ifs, (*thisLoop).loopVar, (*thisLoop).varoffset, (*thisLoop).iterOverArray.([]map[string]interface{})[(*thisLoop).counter])
                        case []interface{}:
                            vseti(ifs, "key_"+(*thisLoop).loopVar, (*thisLoop).varkeyoffset, (*thisLoop).counter)
                            vseti(ifs, (*thisLoop).loopVar, (*thisLoop).varoffset, (*thisLoop).iterOverArray.([]interface{})[(*thisLoop).counter])
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
                                vseti(ifs, (*thisLoop).loopVar, (*thisLoop).varoffset, (*thisLoop).counter)
                            }
                            loopEnd = true
                        }
                    case ACT_DEC:
                        if (*thisLoop).counter < (*thisLoop).condEnd {
                            (*thisLoop).counter -= (*thisLoop).repeatActionStep
                            if (*thisLoop).optNoUse == Opt_LoopIgnore {
                                vseti(ifs, (*thisLoop).loopVar, (*thisLoop).varoffset, (*thisLoop).counter)
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
                        vseti(ifs, (*thisLoop).loopVar, (*thisLoop).varoffset, (*thisLoop).counter)
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

            if ll { lastlock.Unlock() }


        case C_Continue:

            // Continue should work with FOR, FOREACH or WHILE.

            lastlock.RLock()

            if depth[ifs] == 0 {
                parser.report("Attempting to CONTINUE without a valid surrounding construct.")
                finish(false, ERR_SYNTAX)
            } else {

                // @note:
                //  we use indirect access with thisLoop here (and throughout
                //  loop code) for a minor speed bump. we should periodically
                //  review this as an optimisation in Go could make this unnecessary.

                lastlock.RLock()
                switch lastConstruct[ifs][depth[ifs]-1] {

                case C_For, C_Foreach:
                    thisLoop = &loops[ifs][depth[ifs]]
                    pc = (*thisLoop).forEndPos - 1

                case C_While:
                    thisLoop = &loops[ifs][depth[ifs]]
                    pc = (*thisLoop).whileContinueAt - 1
                }
                lastlock.RUnlock()

            }

            lastlock.RUnlock()


        case C_Break:

            // Break should work with either FOR, FOREACH, WHILE or WHEN.

            // We use lastConstruct to establish which is the innermost
            //  of these from which we need to break out.

            // The surrounding construct should set the
            //  lastConstruct[fs][depth] on entry.

            lastlock.RLock()

            if depth[ifs] == 0 {
                parser.report("Attempting to BREAK without a valid surrounding construct.")
                finish(false, ERR_SYNTAX)
            } else {

                // jump calc, depending on break context

                thisLoop = &loops[ifs][depth[ifs]]

                switch lastConstruct[ifs][depth[ifs]-1] {

                case C_For:
                    pc = (*thisLoop).forEndPos - 1
                    breakIn = C_Endfor

                case C_Foreach:
                    pc = (*thisLoop).forEndPos - 1
                    breakIn = C_Endfor

                case C_While:
                    pc = (*thisLoop).whileContinueAt - 1
                    breakIn = C_Endwhile

                case C_When:
                    pc = wc[wccount].endLine - 1

                default:
                    parser.report("A grue is attempting to BREAK out. (Breaking without a surrounding context!)")
                    finish(false, ERR_SYNTAX)
                    lastlock.RUnlock()
                    break
                }

            }

            lastlock.RUnlock()


        case C_Enum:

            if inbound.TokenCount<4 || (
                ! (inbound.Tokens[2].tokType==LParen && inbound.Tokens[inbound.TokenCount-1].tokType==RParen) &&
                ! (inbound.Tokens[2].tokType==LeftCBrace && inbound.Tokens[inbound.TokenCount-1].tokType==RightCBrace)) {
                parser.report("Incorrect arguments supplied for ENUM.")
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
                        parser.report("Cannot increment default value in ENUM")
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

                            evEnum := parser.wrappedEval(ifs,ifs,resu[ea][2:])

                            if evEnum.evalError {
                                parser.report("Invalid expression for assignment in ENUM")
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
                            parser.report("Missing assignment in ENUM")
                            finish(false,ERR_SYNTAX)
                            break enum_loop
                        }
                    }
                }
            }


        case C_Unset: // remove a variable

            if inbound.TokenCount != 2 {
                parser.report("Incorrect arguments supplied for UNSET.")
                finish(false, ERR_SYNTAX)
            } else {
                removee := inbound.Tokens[1].tokText
                if _, ok := VarLookup(ifs, removee); ok {
                    vunset(ifs, removee)
                } else {
                    parser.report(sf("Variable %s does not exist.", removee))
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
                    parser.report("Too many arguments supplied.")
                    finish(false, ERR_SYNTAX)
                    break
                }
                // disable
                panes = make(map[string]Pane)
                panes["global"] = Pane{row: 0, col: 0, h: MH, w: MW + 1}
                currentpane = "global"

            case "select":

                if inbound.TokenCount != 3 {
                    parser.report("Invalid pane selection.")
                    finish(false, ERR_SYNTAX)
                    break
                }

                cp,_ := parser.Eval(ifs,inbound.Tokens[2:3])

                switch cp:=cp.(type) {
                case string:

                    setPane(cp)
                    currentpane = cp

                default:
                    parser.report("Warning: you must provide a string value to PANE SELECT.")
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
                    parser.report("Bad delimiter in PANE DEFINE.")
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
                pname  := parser.wrappedEval(ifs,ifs, inbound.Tokens[2:nameCommaAt])
                py     := parser.wrappedEval(ifs,ifs, inbound.Tokens[nameCommaAt+1:YCommaAt])
                px     := parser.wrappedEval(ifs,ifs, inbound.Tokens[YCommaAt+1:XCommaAt])
                ph     := parser.wrappedEval(ifs,ifs, inbound.Tokens[XCommaAt+1:HCommaAt])
                pw     := parser.wrappedEval(ifs,ifs, ew)
                if hasTitle {
                    ptitle = parser.wrappedEval(ifs,ifs, etit)
                }
                if hasBox   {
                    pbox   = parser.wrappedEval(ifs,ifs, ebox)
                }

                if pname.evalError || py.evalError || px.evalError || ph.evalError || pw.evalError {
                    parser.report("could not evaluate an argument in PANE DEFINE")
                    finish(false, ERR_EVAL)
                    break
                }

                name         := sf("%v",pname.result)
                atlock.Lock()
                col,invalid1 := GetAsInt(px.result)
                row,invalid2 := GetAsInt(py.result)
                atlock.Unlock()
                w,invalid3   := GetAsInt(pw.result)
                h,invalid4   := GetAsInt(ph.result)
                if hasTitle { title = sf("%v",ptitle.result) }
                if hasBox   { boxed = sf("%v",pbox.result)   }

                if invalid1 || invalid2 || invalid3 || invalid4 {
                    parser.report("Could not use an argument in PANE DEFINE.")
                    finish(false,ERR_EVAL)
                    break
                }

                if pname.result.(string) == "global" {
                    parser.report("Cannot redefine the global PANE.")
                    finish(false, ERR_SYNTAX)
                    break
                }

                panes[name] = Pane{row: row, col: col, w: w, h: h, title: title, boxed: boxed}
                paneBox(name)

            case "redraw":
                paneBox(currentpane)

            default:
                parser.report("Unknown PANE command.")
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
                parser.report("Not enough arguments in PAUSE.")
                finish(false, ERR_SYNTAX)
                break
            }

            we = parser.wrappedEval(ifs,ifs, inbound.Tokens[1:])

            if !we.evalError {

                if isNumber(we.result) {
                    i = sf("%v", we.result)
                } else {
                    i = we.result.(string)
                }

                dur, err := time.ParseDuration(i + "ms")

                if err != nil {
                    parser.report(sf("'%s' did not evaluate to a duration.", we.text))
                    finish(false, ERR_EVAL)
                    break
                }

                time.Sleep(dur)

            } else {
                parser.report(sf("could not evaluate PAUSE expression\n%+v",we.errVal))
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
                        if nt.tokType==LParen || nt.tokType==LeftSBrace  { evnest++ }
                        if nt.tokType==RParen || nt.tokType==RightSBrace { evnest-- }
                        if evnest==0 && (term==len(inbound.Tokens[1:])-1 || nt.tokType == O_Comma) {
                            v,_ := parser.Eval(ifs,inbound.Tokens[1+newstart:term+2])
                            newstart=term+1
                            switch v.(type) { case string: v=interpolate(ifs,v.(string)) }
                            docout += sparkle(sf(`%v`, v))
                            continue
                        }
                    }

                    appendToTestReport(test_output_file,ifs, pc, docout)

                }
            }


        case C_Test:

            // TEST "name" GROUP "group_name" ASSERT FAIL|CONTINUE

            inside_test = true

            if testMode {

                if !(inbound.TokenCount == 4 || inbound.TokenCount == 6) {
                    parser.report("Badly formatted TEST command.")
                    finish(false, ERR_SYNTAX)
                    break
                }

                if str.ToLower(inbound.Tokens[2].tokText) != "group" {
                    parser.report("Missing GROUP in TEST command.")
                    finish(false, ERR_SYNTAX)
                    break
                }

                test_assert = "fail"
                if inbound.TokenCount == 6 {
                    if str.ToLower(inbound.Tokens[4].tokText) != "assert" {
                        parser.report("Missing ASSERT in TEST command.")
                        finish(false, ERR_SYNTAX)
                        break
                    } else {
                        switch str.ToLower(inbound.Tokens[5].tokText) {
                        case "fail":
                            test_assert = "fail"
                        case "continue":
                            test_assert = "continue"
                        default:
                            parser.report("Bad ASSERT type in TEST command.")
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

                doAt := findDelim(inbound.Tokens, C_Do, 2)
                if doAt == -1 {
                    parser.report("DO not found in ON")
                    finish(false, ERR_SYNTAX)
                } else {
                    // more tokens after the DO to form a command with?
                    if inbound.TokenCount >= doAt {

                        we = parser.wrappedEval(ifs,ifs, inbound.Tokens[1:doAt])
                        if we.evalError {
                            parser.report( sf("Could not evaluate expression '%v' in ON..DO statement.\n%+v",we.text,we.errVal))
                            finish(false,ERR_EVAL)
                            break
                        }

                        switch we.result.(type) {
                        case bool:
                            if we.result.(bool) {

                                // create a phrase
                                p := Phrase{}
                                p.Tokens = inbound.Tokens[doAt+1:]
                                p.TokenCount = inbound.TokenCount - (doAt + 1)
                                p.Original = inbound.Original

                                // action!
                                inbound=&p
                                goto ondo_reenter

                            }
                        default:
                            pf("Result Type -> %T expression was -> %s\n", we.text, we.result)
                            parser.report("ON cannot operate without a condition.")
                            finish(false, ERR_EVAL)
                            break
                        }

                    }
                }

            } else {
                parser.report("ON missing arguments.")
                finish(false, ERR_SYNTAX)
            }


        case C_Assert:

            if inbound.TokenCount < 2 {

                parser.report("Insufficient arguments supplied to ASSERT")
                finish(false, ERR_ASSERT)

            } else {

                cet := crushEvalTokens(inbound.Tokens[1:])
                we = parser.wrappedEval(ifs,ifs, inbound.Tokens[1:])

                if we.assign {
                    // someone typo'ed a condition 99.9999% of the time
                    parser.report(
                        sf("[#2][#bold]Warning! Assert contained an assignment![#-][#boff]\n  [#6]%v = %v[#-]\n",
                            cet.assignVar,cet.text))
                    finish(false,ERR_ASSERT)
                    break
                }

                if we.evalError {
                    parser.report("Could not evaluate expression in ASSERT statement.")
                    finish(false,ERR_EVAL)
                    break
                }

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
                            parser.report(sf("Could not assert! ( %s )", we.text))
                            finish(false, ERR_ASSERT)
                            break
                        }
                        // under test
                        test_report = sf("[#2]TEST FAILED %s (%s/line %d) : %s[#-]",
                            group_name_string, getReportFunctionName(ifs,false), parser.line, we.text)
                        testsFailed++
                        appendToTestReport(test_output_file,ifs, parser.line, test_report)
                        temp_test_assert := test_assert
                        if fail_override != "" {
                            temp_test_assert = fail_override
                        }
                        switch temp_test_assert {
                        case "fail":
                            parser.report(sf("Could not assert! (%s)", we.text))
                            finish(false, ERR_ASSERT)
                        case "continue":
                            parser.report(sf("Assert failed (%s), but continuing.", we.text))
                        }
                    } else {
                        if under_test {
                            test_report = sf("[#4]TEST PASSED %s (%s/line %d) : %s[#-]",
                                group_name_string, getReportFunctionName(ifs,false), parser.line, we.text)
                            testsPassed++
                            appendToTestReport(test_output_file,ifs, pc, test_report)
                        }
                    }
                }

            }


        case C_Init: // initialise an array

            /*
            if inbound.TokenCount<2 {
                parser.report("Not enough arguments in INIT.")
                finish(false,ERR_EVAL)
                break
            }

            varname := interpolate(ifs,inbound.Tokens[1].tokText)

            vartype := "assoc"
            if inbound.TokenCount>2 {
                vartype = inbound.Tokens[2].tokText
            }

            size:=DEFAULT_INIT_SIZE

            if inbound.TokenCount>3 {

                we = parser.wrappedEval(ifs,ifs, inbound.Tokens[3:])
                if we.evalError {
                    parser.report(sf("could not evaluate expression in INIT statement\n%+v",we.errVal))
                    finish(false,ERR_EVAL)
                    break
                }

                switch we.result.(type) {
                case int,int64:
                    strSize,invalid:=GetAsInt(we.result)
                    if ! invalid {
                        size=strSize
                    }
                default:
                    parser.report("Array width must evaluate to an integer.")
                    finish(false,ERR_EVAL)
                    break
                }

            }

            if varname != "" {

                // TODO: these cases need moving into C_Var: (DONE?)
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
                    var tu uint
                    var ti int
                    var tf64 float64
                    var ts string
                    var atint   []interface{}
                    var ats     []string

                    // instantiate fields with an empty expected type:
                    typemap:=make(map[string]reflect.Type)
                    typemap["bool"]     = reflect.TypeOf(tb)
                    typemap["byte"]     = reflect.TypeOf(tu8)
                    typemap["uint8"]    = reflect.TypeOf(tu8)
                    typemap["uint"]     = reflect.TypeOf(tu)
                    typemap["int"]      = reflect.TypeOf(ti)
                    typemap["float"]    = reflect.TypeOf(tf64)
                    typemap["float64"]  = reflect.TypeOf(tf64)
                    typemap["string"]   = reflect.TypeOf(ts)
                    typemap["[]string"] = reflect.TypeOf(ats)
                    typemap["[]"]       = reflect.TypeOf(atint)
                    typemap["^"]        = reflect.TypeOf(ats)
                    //


                }
            }
            */

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
                parser.report("Invalid arguments in ASYNC\n"+usage)
                finish(false,ERR_SYNTAX)
                break
            }

            handles := inbound.Tokens[1].tokText
            call    := inbound.Tokens[2].tokText

            if inbound.Tokens[3].tokType!=LParen {
                parser.report("could not find '(' in ASYNC function call.")
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
               parser.report("could not find a valid ')' in ASYNC function call.")
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
                    parser.report(sf("could not evaluate handle key argument '%+v' in ASYNC.",inbound.Tokens[rightParenLoc+1:]))
                    finish(false,ERR_EVAL)
                    break
                }
            }

            // build task call
            lmv, isfunc := fnlookup.lmget(call)

            if isfunc {

                errClear:=true
                for e:=0; e<len(errs); e++ {
                    if errs[e]!=nil {
                        // error
                        pf("- arg %d: %+v\n",errs[e])
                        errClear=false
                    }
                }

                if !errClear {
                    parser.report(sf("problem evaluating arguments in function call. (fs=%v)\n", ifs))
                    finish(false, ERR_EVAL)
                    break
                }

                // make Za function call
                loc,id := GetNextFnSpace(call+"@")
                calllock.Lock()

                // @note: @todo:
                // vset below is commented out as, currently, short-lived async tasks
                // can possibly have their previously reserved 'loc' value re-used
                // by subsequent launches of the parallel task.
                // we should keep a recent-list (bad) or work out a way to hold on to
                // the reservation until after await() has collected it.

                // vset(ifs,"@#@"+strconv.Itoa(int(loc)),nil)

                calltable[loc] = call_s{fs: id, base: lmv, caller: ifs, callline: pc, retvar: sf("@#@%v",loc)}
                calllock.Unlock()

                // construct a go call that includes a normal Call
                h:=task(ifs,loc,resu...)

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


        case C_Require:

            // require feat support in stdlib first. requires version-as-feat support and markup.

            if inbound.TokenCount < 2 {
                parser.report("Malformed REQUIRE statement.")
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
                    lver,_ :=vget(0,"@version")
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
                        parser.report("Could not evaluate your EXIT expression")
                        finish(true,ERR_EVAL)
                    }
                }
            } else {
                finish(true, 0)
            }


        case C_Define:

            if inbound.TokenCount > 1 {

                if defining {
                    parser.report("Already defining a function. Nesting not permitted.")
                    finish(true, ERR_SYNTAX)
                    break
                }

                defining = true
                definitionName = inbound.Tokens[1].tokText

                loc, _ := GetNextFnSpace(definitionName)
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
                        parser.report("A library function already exists with the name '"+definitionName+"'")
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

                sourceMap[loc]=base     // relate defined base 'loc' to parent 'ifs' instance's 'base' source
                fspacelock.Lock()
                functionspaces[loc] = []Phrase{}
                fspacelock.Unlock()

                farglock.Lock()
                functionArgs[loc].args   = dargs
                farglock.Unlock()

            }

        case C_Showdef:

            if inbound.TokenCount == 2 {
                fn := stripOuterQuotes(inbound.Tokens[1].tokText, 2)
                if _, exists := fnlookup.lmget(fn); exists {
                    ShowDef(fn)
                } else {
                    parser.report("Function not found.")
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
            for tok := range argtoks {
                nt:=argtoks[tok]
                if nt.tokType==LParen { evnest++ }
                if nt.tokType==RParen { evnest-- }
                if nt.tokType==LeftSBrace { evnest++ }
                if nt.tokType==RightSBrace { evnest-- }
                if nt.tokType!=O_Comma || evnest>0 {
                    rargs[curArg]=append(rargs[curArg],nt)
                }
                if evnest==0 && (tok==len(argtoks)-1 || nt.tokType == O_Comma) {
                    curArg++
                    if int(curArg)>=len(rargs) {
                        rargs=append(rargs,[]Token{})
                    }
                }
            }
            retval_count=curArg
            // pf("retval_count : %d\n",retval_count)

            // SECTION MOVED FROM HERE
            // MOVED SECTION ENDED HERE

            // tail call recursion handling:
            if inbound.TokenCount > 2 {

                var bname string
                bname, _ = numlookup.lmget(base)

                tco_check:=false // deny tco until we check all is well

                if inbound.Tokens[1].tokType==Identifier && inbound.Tokens[2].tokType==LParen {
                    if strcmp(inbound.Tokens[1].tokText,bname) {
                        // pf("passed func same name check\n")
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
                    for q:=0; q<len(errs); q++ {
                        va[q]=resu[q]
                        if errs[q]!=nil { skip_reentry=true; break }
                    }
                    // no args/wrong arg count check
                    if len(errs)!=len(va) {
                        skip_reentry=true
                    }

                    // set tco flag if required, and perform.
                    if !skip_reentry {
                        vset(ifs,"@in_tco",true)
                        pc=-1
                        goto tco_reentry
                    }
                }
            }

            // evaluate each expr and stuff the results in an array
            var ev_er error
            retvalues=make([]interface{},curArg)
            for q:=0;q<int(curArg);q++ {
                retvalues[q], ev_er = parser.Eval(ifs,rargs[q])
                if ev_er!=nil {
                    parser.report("Could not evaluate RETURN arguments")
                    finish(true,ERR_EVAL)
                    break
                }
            }

            endFunc = true
            break

        case C_Enddef:

            if !defining {
                parser.report("Not currently defining a function.")
                finish(false, ERR_SYNTAX)
                break
            }

            defining = false
            definitionName = ""


        case C_Input:

            // INPUT <id> <type> <position> [<hint>]
            // - set variable {id} from external value or exits.

            // get C_Input arguments

            if inbound.TokenCount < 4 {
                usage:= "INPUT [#i1]id[#i0] PARAM | OPTARG [#i1]field_position[#i0] [ [#i1]error_hint[#i0] ]\n"
                usage+= "INPUT [#i1]id[#i0] ENV [#i1]env_name[#i0]"
                parser.report("Incorrect arguments supplied to INPUT.\n"+usage)
                finish(false, ERR_SYNTAX)
                break
            }

            id := inbound.Tokens[1].tokText
            typ := inbound.Tokens[2].tokText
            pos := inbound.Tokens[3].tokText

            hint:=id
            if inbound.TokenCount==5 {
                we=parser.wrappedEval(ifs,ifs,inbound.Tokens[4:])
                if !we.evalError {
                    hint=we.result.(string)
                }
            }

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
                        parser.report(sf("Expected CLI parameter [%s] not provided at startup.", hint))
                        finish(true, ERR_SYNTAX)
                    }
                } else {
                    parser.report(sf("That '%s' doesn't look like a number.", pos))
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
                        if vid, found := VarLookup(ifs,id); !found || (found && ! ident[ifs][vid].declared) {
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
                    if vid, found := VarLookup(ifs,id); !found || (found && ! ident[ifs][vid].declared) {
                        vset(ifs,id,"")
                    }
                }
            }


        case C_Module:

            if inbound.TokenCount > 1 {
                we = parser.wrappedEval(ifs,ifs, inbound.Tokens[1:])
                if we.evalError {
                    parser.report(sf("could not evaluate expression in MODULE statement\n%+v",we.errVal))
                    finish(false,ERR_MODULE)
                    break
                }
            } else {
                parser.report("No module name provided.")
                finish(false, ERR_MODULE)
                break
            }

            fom := we.result.(string)

            if strcmp(fom,"") {
                parser.report("Empty module name provided.")
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
            if fnlookup.lmexists("@mod_"+fom) && !permit_dupmod {
                parser.report("Module file "+fom+" already processed once.")
                finish(false, ERR_SYNTAX)
                break
            }

            if !fnlookup.lmexists("@mod_"+fom) {

                loc, _ := GetNextFnSpace("@mod_"+fom)

                calllock.Lock()

                fspacelock.Lock()
                functionspaces[loc] = []Phrase{}
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
                phraseParse("@mod_"+fom, string(mod), 0)

                modcs := call_s{}
                modcs.base = loc
                modcs.caller = ifs
                modcs.fs = "@mod_" + fom
                modcs.callline = pc
                calltable[loc] = modcs

                calllock.Unlock()

                atomic.AddInt32(&concurrent_funcs, 1)
                Call(MODE_NEW, loc, ciMod)
                atomic.AddInt32(&concurrent_funcs, -1)

                currentModule=oldModule

            }

        case C_When:

            // need to store the condition and result for the is/contains/has/or clauses
            // endwhen location should be calculated in advance for a direct jump to exit

            if wccount==WHEN_CAP {
                parser.report(sf("maximum WHEN nesting reached (%d)",WHEN_CAP))
                finish(true,ERR_SYNTAX)
                break
            }

            if inbound.TokenCount==1 {
                inbound.Tokens=append(inbound.Tokens,Token{tokType:Identifier,subtype:subtypeConst,tokVal:true,tokText:"true"})
            }

            // lookahead
            endfound, enddistance, er := lookahead(base, pc, 0, 0, C_Endwhen, []uint8{C_When}, []uint8{C_Endwhen})

            if er {
                parser.report("Lookahead error!")
                finish(true, ERR_SYNTAX)
                break
            }

            if !endfound {
                parser.report("Missing ENDWHEN for this WHEN. Maybe check for open quotes or braces in block?")
                finish(false, ERR_SYNTAX)
                break
            }

            we = parser.wrappedEval(ifs,ifs, inbound.Tokens[1:])
            if we.evalError {
                parser.report(sf("could not evaluate the WHEN condition\n%+v",we.errVal))
                finish(false, ERR_EVAL)
                break
            }

            // create storage for WHEN details and increase the nesting level

            lastlock.Lock()

            wccount++
            wc[wccount] = whenCarton{endLine: pc + enddistance, value: we.result, dodefault: true}
            depth[ifs]++
            lastConstruct[ifs] = append(lastConstruct[ifs], C_When)

            lastlock.Unlock()


        case C_Is, C_Has, C_Contains, C_Or:

            lastlock.RLock()

            if depth[ifs] == 0 || (depth[ifs] > 0 && lastConstruct[ifs][depth[ifs]-1] != C_When) {
                parser.report("Not currently in a WHEN block.")
                finish(false,ERR_SYNTAX)
                lastlock.RUnlock()
                break
            }

            carton := wc[wccount]

            lastlock.RUnlock()

            if inbound.TokenCount > 1 { // inbound.TokenCount==1 for C_Or
                we = parser.wrappedEval(ifs,ifs, inbound.Tokens[1:])
                if we.evalError {
                    parser.report(sf("could not evaluate expression in WHEN condition\n%+v",we.errVal))
                    finish(false, ERR_EVAL)
                    break
                }
            }

            ramble_on := false // assume we'll need to skip to next when clause

            switch statement.tokType {

            case C_Has: // <-- @note: this may change yet

                // build expression from rest, ignore initial condition
                switch we.result.(type) {
                case bool:
                    if we.result.(bool) {  // HAS truth
                        wc[wccount].dodefault = false
                        ramble_on = true
                    }
                default:
                    parser.report(sf("HAS condition did not result in a boolean\n%+v",we.errVal))
                    finish(false, ERR_EVAL)
                }

            case C_Is:
                if we.result == carton.value { // matched IS value
                    wc[wccount].dodefault = false
                    ramble_on = true
                }

            case C_Contains:
                reg := sparkle(we.result.(string))
                switch carton.value.(type) {
                case string:
                    if matched, _ := regexp.MatchString(reg, carton.value.(string)); matched { // matched CONTAINS regex
                        wc[wccount].dodefault = false
                        ramble_on = true
                    }
                case int:
                    if matched, _ := regexp.MatchString(reg, strconv.Itoa(carton.value.(int))); matched { // matched CONTAINS regex
                        wc[wccount].dodefault = false
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

            var loc int16

            // jump to the next clause, continue to next line or skip to end.

            if ramble_on { // move on to next pc statement
            } else {
                // skip to next WHEN clause:
                hasfound, hasdistance, _ := lookahead(base, pc+1, 0, 0, C_Has, []uint8{C_When}, []uint8{C_Endwhen})
                isfound, isdistance, _   := lookahead(base, pc+1, 0, 0, C_Is, []uint8{C_When}, []uint8{C_Endwhen})
                orfound, ordistance, _   := lookahead(base, pc+1, 0, 0, C_Or, []uint8{C_When}, []uint8{C_Endwhen})
                cofound, codistance, _   := lookahead(base, pc+1, 0, 0, C_Contains, []uint8{C_When}, []uint8{C_Endwhen})

                // add jump distances to list
                distList := []int16{}
                if hasfound {
                    distList = append(distList, hasdistance)
                }
                if isfound {
                    distList = append(distList, isdistance)
                }
                if orfound {
                    distList = append(distList, ordistance)
                }
                if cofound {
                    distList = append(distList, codistance)
                }

                if !(isfound || hasfound || orfound || cofound) {
                    // must be an endwhen
                    loc = carton.endLine
                    // pf("@%d : direct jump to endwhen at %d\n",pc,loc+1)
                } else {
                    loc = pc + min_int16(distList) + 1
                    // pf("@%d : direct jump from distList to %d\n",pc,loc+1)
                }

                // jump to nearest following clause
                pc = loc - 1
            }


        case C_Endwhen:

            lastlock.Lock()

            if depth[ifs] == 0 || (depth[ifs] > 0 && lastConstruct[ifs][depth[ifs]-1] != C_When) {
                parser.report( "Not currently in a WHEN block.")
                lastlock.Unlock()
                break
            }

            breakIn = Error
            lastConstruct[ifs] = lastConstruct[ifs][:depth[ifs]-1]
            depth[ifs]--
            wccount--

            if wccount < 0 {
                parser.report("Cannot reduce WHEN stack below zero.")
                finish(false, ERR_SYNTAX)
            }

            lastlock.Unlock()


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

            structName=""
            structNode=[]interface{}{}
            structMode=false


        case C_Showstruct:

            // SHOWSTRUCT [filter]

            var filter string

            if inbound.TokenCount>1 {
                cet := crushEvalTokens(inbound.Tokens[1:])
                filter = interpolate(ifs,cet.text)
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
                parser.report("Malformed WITH statement.")
                finish(false, ERR_SYNTAX)
                break
            }

            asAt := findDelim(inbound.Tokens, C_As, 2)
            if asAt == -1 {
                parser.report("AS not found in WITH")
                finish(false, ERR_SYNTAX)
                break
            }

            vid  :=inbound.Tokens[1].offset
            vname:=inbound.Tokens[1].tokText
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
            ll:=false
            if atomic.LoadInt32(&concurrent_funcs)>0 { vlock.RLock() ; ll=true }
            content,_:=vgeti(ifs,vid)
            if ll { vlock.RUnlock() }

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
        // we should only need to worry about parens when scanning for commas
        // as strings should be single string literal tokens.
        case C_Print:

            if inbound.TokenCount > 1 {
                evnest:=0
                newstart:=0
                for term := range inbound.Tokens[1:] {
                    nt:=inbound.Tokens[1+term]
                    if nt.tokType==LParen || nt.tokType==LeftSBrace  { evnest++ }
                    if nt.tokType==RParen || nt.tokType==RightSBrace { evnest-- }
                    if evnest==0 && (term==len(inbound.Tokens[1:])-1 || nt.tokType == O_Comma) {
                        v, _ := parser.Eval(ifs,inbound.Tokens[1+newstart:term+2])
                        newstart=term+1
                        switch v.(type) { case string: v=interpolate(ifs,v.(string)) }
                        pf(`%v`,sparkle(v))
                        continue
                    }
                }
                if interactive { pf("\n") }
            } else {
                pf("\n")
            }


        case C_Println:
            if inbound.TokenCount > 1 {
                evnest:=0
                newstart:=0
                for term := range inbound.Tokens[1:] {
                    nt:=inbound.Tokens[1+term]
                    if nt.tokType==LParen || nt.tokType==LeftSBrace  { evnest++ }
                    if nt.tokType==RParen || nt.tokType==RightSBrace { evnest-- }
                    if evnest==0 && (term==len(inbound.Tokens[1:])-1 || nt.tokType == O_Comma) {
                        v, _ := parser.Eval(ifs,inbound.Tokens[1+newstart:term+2])
                        newstart=term+1
                        switch v.(type) { case string: v=interpolate(ifs,v.(string)) }
                        pf(`%v`,sparkle(v))
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
                evnest:=0
                newstart:=0
                for term := range inbound.Tokens[1:] {
                    nt:=inbound.Tokens[1+term]
                    if nt.tokType==LParen || nt.tokType==LeftSBrace  { evnest++ }
                    if nt.tokType==RParen || nt.tokType==RightSBrace { evnest-- }
                    if evnest==0 && (term==len(inbound.Tokens[1:])-1 || nt.tokType == O_Comma) {
                        v, _ := parser.Eval(ifs,inbound.Tokens[1+newstart:term+2])
                        newstart=term+1
                        switch v.(type) { case string: v=interpolate(ifs,v.(string)) }
                        plog_out += sf(`%v`,sparkle(v))
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

            commaAt := findDelim(inbound.Tokens, O_Comma, 1)

            if commaAt == -1 || commaAt == inbound.TokenCount {
                parser.report(  "Bad delimiter in AT.")
                finish(false, ERR_SYNTAX)
            } else {

                expr_row, err := parser.Eval(ifs,inbound.Tokens[1:commaAt])
                if expr_row==nil || err != nil {
                    parser.report( sf("Evaluation error in %v", expr_row))
                }

                expr_col, err := parser.Eval(ifs,inbound.Tokens[commaAt+1:])
                if expr_col==nil || err != nil {
                    parser.report(  sf("Evaluation error in %v", expr_col))
                }

                atlock.Lock()
                row, _ = GetAsInt(expr_row)
                col, _ = GetAsInt(expr_col)
                atlock.Unlock()

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
                if inbound.Tokens[1].tokType == O_Assign {
                    we = parser.wrappedEval(ifs,ifs, inbound.Tokens[2:])
                    if we.evalError {
                        parser.report(sf("could not evaluate expression prompt assignment\n%+v",we.errVal))
                        finish(false, ERR_EVAL)
                        break
                    }
                    switch we.result.(type) {
                    case string:
                        PromptTemplate=stripOuterQuotes(inbound.Tokens[2].tokText,1)
                    }
                } else {
                    // prompt command:
                    if inbound.TokenCount < 3 {
                        parser.report( "Incorrect arguments for PROMPT command.")
                        finish(false, ERR_SYNTAX)
                        break
                    } else {
                        validator := ""
                        broken := false
                        expr, prompt_ev_err := parser.Eval(ifs,inbound.Tokens[2:3])
                        if expr==nil {
                            parser.report( "Could not evaluate in PROMPT command.")
                            finish(false,ERR_EVAL)
                            break
                        }
                        if prompt_ev_err == nil {
                            processedPrompt := expr.(string)
                            echoMask,_:=vget(0,"@echomask")
                            if inbound.TokenCount > 3 {
                                val_ex,val_ex_error := parser.Eval(ifs,inbound.Tokens[3:])
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
                    we = parser.wrappedEval(ifs,ifs, inbound.Tokens[2:])
                    if we.evalError {
                        parser.report( sf("could not evaluate destination filename in LOGGING ON statement\n%+v",we.errVal))
                        finish(false, ERR_EVAL)
                        break
                    }
                    logFile = we.result.(string)
                    vset(0, "@logsubject", "")
                }

            case "quiet":
                vset(0, "@silentlog", true)

            case "loud":
                vset(0, "@silentlog", false)

            case "accessfile":
                if inbound.TokenCount > 2 {
                    we = parser.wrappedEval(ifs,ifs, inbound.Tokens[2:])
                    if we.evalError {
                        parser.report( sf("could not evaluate filename in LOGGING ACCESSFILE statement\n%+v",we.errVal))
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
                    we = parser.wrappedEval(ifs,ifs, inbound.Tokens[2:])
                    if we.evalError {
                        parser.report( sf("could not evaluate logging subject in LOGGING SUBJECT statement\n%+v",we.errVal))
                        finish(false, ERR_EVAL)
                        break
                    }
                    vset(0, "@logsubject", we.result.(string))
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
                atlock.Lock()
                row = 1
                col = 1
                atlock.Unlock()
                currentpane = "global"
            } else {
                if currentpane != "global" {
                    p := panes[currentpane]
                    for l := 1; l < p.h; l++ {
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
            elsefound, elsedistance, er := lookahead(base, pc, 0, 1, C_Else, []uint8{C_If}, []uint8{C_Endif})
            endfound, enddistance, er := lookahead(base, pc, 0, 0, C_Endif, []uint8{C_If}, []uint8{C_Endif})

            if er || !endfound {
                parser.report("Missing ENDIF for this IF")
                finish(false, ERR_SYNTAX)
                break
            }

            // eval
            // pf("IF EXPR TOKENS : [%+v]\n",inbound.Tokens[1:])
            expr, err = parser.Eval(ifs, inbound.Tokens[1:])
            // pf("Expr result -> %+v\n",expr)
            if err!=nil {
                parser.report("Could not evaluate expression.")
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

            // local command assignment (child/parent process call)

            if inbound.TokenCount > 1 { // ident "=|"
                if statement.tokType == Identifier && inbound.Tokens[1].tokType == O_AssCommand {
                    if inbound.TokenCount > 2 {
                        // get text after =|
                        startPos := str.IndexByte(inbound.Original, '|') + 1
                        cmd := interpolate(ifs, inbound.Original[startPos:])
                        cop:=system(cmd,false)
                        lhs_name := statement.tokText
                        vset(ifs, lhs_name, cop)
                    }
                    // skip normal eval below
                    break
                }
            }

            // try to eval and assign

            if we=parser.wrappedEval(ifs,ifs,inbound.Tokens); we.evalError {
                parser.report(sf("Error in evaluation\n%+v\n",we.errVal))
                finish(false,ERR_EVAL)
                break
            } else {
                if interactive && !we.assign && we.result!=nil {
                    pf("%#v\n",we.result)
                }
            }

        } // end-statements-case

    } // end-pc-loop


    siglock.RLock()
    si=sig_int
    siglock.RUnlock()

    if structMode && !typeInvalid {
        // incomplete struct definition
        pf("Open STRUCT definition %v\n",structName)
        finish(true,ERR_SYNTAX)
    }

    if !si {

        // populate return variable in the caller with retvals
        if retvalues!=nil {
            // pf("call-end (%v) (caller:%v,retvar:%v) with retvalues : %+v\n",caller,retvar,fs,retvalues)
            vset(caller, retvar, retvalues)
        }

        // clean up

        // pf("Leaving call with ifs of %d [fs:%s]\n\n",ifs,fs)

        // pf("[#2]about to delete %v[#-]\n",fs)

        // pf("about to enter call de-allocation with fs of '%s'\n",fs)
        if !str.HasPrefix(fs,"@mod_") {

            if atomic.LoadInt32(&concurrent_funcs)>0 { lastlock.Lock() ; ll=true }
            depth[ifs]=0
            loops[ifs]=nil
            if ll { lastlock.Unlock() }

            if atomic.LoadInt32(&concurrent_funcs)>0 { calllock.Lock(); ll=true }
            calltable[ifs]=call_s{}
            fnlookup.lmdelete(fs)
            numlookup.lmdelete(ifs)

            // we keep a record here of recently disposed functionspace names
            //  so that mem_summary can label disposed of function allocations.
            lastfunc[ifs]=fs

            if ll { calllock.Unlock() }

            if ifs>2 {
                if atomic.LoadInt32(&concurrent_funcs)>0 { fspacelock.Lock() ; ll=true }
                functionspaces[ifs] = []Phrase{}
                if ll { fspacelock.Unlock() }
            }

        }

    }

    if atomic.LoadInt32(&concurrent_funcs)>0 { calllock.Lock() ; ll=true }
    callChain=callChain[:len(callChain)-1]
    if ll { calllock.Unlock() }

    return retval_count,endFunc

}

func system(cmd string, display bool) (cop struct{out string; err string; code int; okay bool}) {
    cmd = str.Trim(cmd," \t\n")
    if hasOuter(cmd,'`') {
        cmd=stripOuter(cmd,'`')
    }
    cop = Copper(cmd, false)
    if display { pf("%s",cop.out) }
    return cop
}

/// execute a command in the shell coprocess or parent
func coprocCall(ifs uint32,s string) {
    cet := ""
    s=str.TrimRight(s,"\n")
    if len(s) > 0 {
        // find index of first pipe, then remove everything upto and including it
        pipepos := str.IndexByte(s, '|')
        cet = s[pipepos+1:]
        inter   := interpolate(ifs,cet)
        cop := Copper(inter, false)
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
    var ifn uint32
    var present bool
    if ifn, present = fnlookup.lmget(fn); !present {
        return false
    }

    if ifn < uint32(len(functionspaces)) {

        if str.HasPrefix(fn,"@mod_") {
            return false
        }

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
            // pf("%s%s\n", strOut, functionspaces[ifn][q].Original)
            pf(sparkle(str.ReplaceAll(sf("%s%s\n", strOut, functionspaces[ifn][q].Original),"%","%%")))
        }
    }
    return true
}


/// search token list for a given delimiter string
func findDelim(tokens []Token, delim uint8, start int16) (pos int16) {
    n:=0
    for p := start; p < int16(len(tokens)); p++ {
        if tokens[p].tokType==LParen { n++ }
        if tokens[p].tokType==RParen { n-- }
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
        if nt.tokType==LParen { evnest++ }
        if nt.tokType==RParen { evnest-- }
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
        if nt.tokType==LParen { evnest++ }
        if nt.tokType==RParen { evnest-- }
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


