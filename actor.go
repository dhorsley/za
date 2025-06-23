package main

import (
    "io/ioutil"
    "math"
    "math/big"
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
    "context"
    "errors"
)


var debugger = &Debugger{
    breakpoints: make(map[uint64]string),
    watchList:     []string{},
    listContext: 10,
}

var activeDebugContext *leparser
// var currentPC int16

func showIdent(ident *[]Variable) {
    for k,e:=range (*ident) {
        pf("%3d -- %s -> %+v -- decl -> %v\n",k,e.IName,e.IValue,e.declared)
    }
}

// populate a struct.
func fillStruct(t *Variable,structvalues []any,Typemap map[string]reflect.Type,hasAry bool,fieldNames []string) (error) {

    if len(structvalues)>0 {
        var sfields []reflect.StructField
        offset:=uintptr(0)
        nextNamePos:=0
        for svpos:=0; svpos<len(structvalues); svpos+=4 {
            // structvalues: [0] name [1] type [2] boolhasdefault [3] default_value
            nv :=structvalues[svpos].(string)
            nt :=structvalues[svpos+1].(string)

            if nt=="mixed" {
                nt="any"
            }

            newtype:=Typemap[nt]

            // override name if provided in fieldNames:
            if len(fieldNames)>0 {
                // pf("Replacing field named '%s' with '%s'\n",nv,fieldNames[nextNamePos])
                nv=fieldNames[nextNamePos]
                if nt=="any" {
                    newtype=reflect.TypeOf((*any)(nil)).Elem()
                } else if nt=="[]" || nt=="[]any" || nt=="[]mixed" {
                    newtype=reflect.TypeOf([]any{})
                } else {
                    newtype=reflect.TypeOf(structvalues[svpos+3])
                    // newtype=Typemap[nt]
                }
                nextNamePos++
            }
            // pf("  ([#2]nv %s [#6]%v[#-]) \n",nv,newtype)

            // populate struct fields:
            sfields=append(sfields,
                reflect.StructField{
                    Name:nv,PkgPath:"main",
                    Type:newtype,
                    Offset:offset,
                    Anonymous:false,
                },
            )
            /*
            pf("fillstruct::pre-offset::nt->%s\n",nt)
            pf("fillstruct::pre-offset::tme->%s\n",Typemap[nt])
            */

            if nt == "any" {
                offset += 32  // interface size
            } else if nt == "[]any" || nt == "[]" {
                offset += reflect.TypeOf([]any{}).Size()  // slice size (24 bytes)
            } else {
                offset += Typemap[nt].Size()
            }

        }
        // pf(" (inside fillStruct()) [ sf-> %#v ]\n",sfields)
        new_struct:=reflect.StructOf(sfields)
        v:=(reflect.New(new_struct).Elem()).Interface()

        if !hasAry {
            // default values setting:

            val:=reflect.ValueOf(v)
            tmp:=reflect.New(val.Type()).Elem()
            tmp.Set(val)

            nextNamePos:=0
            for svpos:=0; svpos<len(structvalues); svpos+=4 {
                // structvalues: [0] name [1] type [2] boolhasdefault [3] default_value
                nv :=structvalues[svpos].(string)
                nhd:=structvalues[svpos+2].(bool)
                ndv:=structvalues[svpos+3]

                if len(fieldNames)>0 {
                    nv=fieldNames[nextNamePos]
                    nextNamePos++
                }

                tf:=tmp.FieldByName(nv)

                // Bodge: special case assignment of bigi/bigf to coerce type:
                switch tf.Type().String() {
                case "*big.Int":
                    ndv=GetAsBigInt(ndv)
                    nhd=true
                case "*big.Float":
                    ndv=GetAsBigFloat(ndv)
                    nhd=true
                }
                // end-bodge

                if nhd {

                    var intyp reflect.Type
                    if ndv!=nil { intyp=reflect.ValueOf(ndv).Type() }

                    if intyp.AssignableTo(tf.Type()) {
                        tf=reflect.NewAt(tf.Type(),unsafe.Pointer(tf.UnsafeAddr())).Elem()
                        tf.Set(reflect.ValueOf(ndv))
                    } else {
                        return fmt.Errorf("cannot set field default (%T) for %v (%v)",ndv,nv,tf.Type())
                    }
                }
            }

            (*t).IValue=tmp.Interface()
            gob.Register((*t).IValue)
            // var tmpArray = reflect.ArrayOf(0,val.Type())
            // gob.Register(tmpArray)

        } else {
            (*t).IValue=[]any{}
        }

    } // end-len>0

    return nil
}



func task(caller uint32, base uint32, endClose bool, callname string, iargs ...any) (chan any,string) {

    r:=make(chan any)

    loc,id := GetNextFnSpace(true,callname+"@",call_s{prepared:true,base:base,caller:caller,gc:false,gcShyness:100})
    // fmt.Printf("***** [task]  loc#%d caller#%d, recv cstab: %+v\n",loc,caller,calltable[loc])

    go func() {
        if endClose { defer close(r) }
        var ident = make([]Variable,identInitialSize)

        atomic.AddInt32(&concurrent_funcs,1)

        var rcount byte
        var errVal error

        ctx := withProfilerContext(context.Background())
        if enableProfiling {
            id_for_profiling:="async_task: "+str.Replace(id,"@"," instance ",-1)
            startTime := time.Now()
            startProfile(id_for_profiling)
            pushToCallChain(ctx, id_for_profiling)
            rcount,_,_,errVal=Call(ctx, MODE_NEW, &ident, loc, ciAsyn, false, nil, "", []string{}, iargs...)
            popCallChain(ctx)
            recordExclusiveExecutionTime(ctx,[]string{id_for_profiling}, time.Since(startTime))
        } else {
            rcount,_,_,errVal=Call(ctx,MODE_NEW, &ident, loc, ciAsyn, false, nil, "", []string{}, iargs...)
        }
        if errVal!=nil {
            panic(errors.New(sf("call error in async task %s",id)))
        }

        // fmt.Printf("[task] sending into chan for key %s: %p\n", id,r)

        switch rcount {
        case 0:
            // pf("[task] [rcount==0 case] sending result for loc %v: %+v\n", loc, nil)
            r<-struct{l uint32;r any}{loc,nil}
            // pf("[#3]TASK RESULT : loc %d : no value (nil)[#-]\n",loc)
        case 1:
            calllock.RLock()
            v:=calltable[loc].retvals
            // pf("[task] [rcount==1 case] sending result for loc %v: %+v\n", loc, v)
            calllock.RUnlock()
            if v==nil {
                r<-nil
                break
            }
            // pf("[#3]TASK RESULT : loc %d : val (%+v)[#-]\n",loc,v.([]any))
            r<-struct{l uint32;r any}{loc,v.([]any)[0]}
        default:
            calllock.RLock()
            v:=calltable[loc].retvals
            // pf("[task] [default case] sending result for loc %v: %+v\n", loc, v)
            calllock.RUnlock()
            r<-struct{l uint32;r any}{loc,v}
            // pf("[#3]TASK RESULT : loc %d : val (%+v)[#-]\n",loc,v.([]any))
        }

        // Now mark for GC AFTER the send
        calllock.Lock()
        calltable[loc].gcShyness = 10000
        calltable[loc].gc = true
        calllock.Unlock()

        atomic.AddInt32(&concurrent_funcs,-1)

    }()
    return r,id
}


var testlock = &sync.Mutex{}
var atlock = &sync.Mutex{}

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
// @note: testing new version since Go 1.18 changed this.
/*
func ostrcmp(a string, b string) (bool) {
    la:=len(a)
    if la!=len(b)   { return false }
    if la==0        { return true }
    strcmp_repeat_point:
        la -= 1
        if a[la]!=b[la] { return false }
    if la>0 { goto strcmp_repeat_point }
    return true
}
*/

func strcmp(a string, b string) (bool) {
    la:=len(a)
    if la!=len(b)   { return false }
    if la==0        { return true }
    for ;la>0; {
        la-=1
        if a[la]!=b[la] { return false }
    }
    return true
}

func GetAsString(v any) (i string) {
    switch v.(type) {
    case *big.Int:
        n:=v.(*big.Int)
        i = n.String()
    case *big.Float:
        f:=v.(*big.Float)
        i = f.String()
    default:
        i = sf("%v",v)
    }
    return
}

func GetAsBigInt(i any) (*big.Int) {
    var ri big.Int
    switch i:=i.(type) {
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
        ri.SetString(i,0)
    }
    return &ri
}

func GetAsBigFloat(i any) *big.Float {
    var r big.Float
    switch i:=i.(type) {
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
func GetAsInt64(expr any) (int64, bool) {
    switch i := expr.(type) {
    case float64:
        return int64(i), false
    case uint:
        return int64(i), false
    case int:
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
        if !i { return int(0), false }
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
        if i!="" {
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

    if sval=="" {
        return false
    }

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
func lookahead(fs uint32, startLine int16, indent int, endlevel int, term int64, indenters []int64, dedenters []int64) (bool, int16, bool) {

    // pf("(la) searching for %s from statement #%d\n",tokNames[term],startLine)

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
            indent-=1
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
//  definition in the functionspace[] list.
//  do_lock normally only false during recursive user-defined fn calls.
func GetNextFnSpace(do_lock bool, requiredName string, cs call_s) (uint32,string) {

    // fmt.Printf("Entered gnfs\n")
    calllock.Lock()

    // : sets up a re-use value
    var reuse,e uint32
    if (globseq % globseq_disposal_freq )==0 {
        for e=0; e<globseq; e+=1 {
            if calltable[e].gc && calltable[e].disposable {
                if calltable[e].gcShyness>0 { calltable[e].gcShyness-=1 }
                if calltable[e].gcShyness==0 {
                    reuse=e
                    // runtime.GC()
                    break
                }
            }
        }
    }

    // find a reservation
    for ; numlookup.lmexists(globseq) ; { // reserved
        globseq=(globseq+1) % gnfsModulus
        if globseq==0 { globseq=2 }
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
        // fmt.Printf("[gnfs] resized calltable.\n")
    }

    if reuse==0 {
        reuse=globseq
    }

    // generate new tagged instance name
    newName := requiredName
    if newName[len(newName)-1]=='@' {
        newName+=strconv.FormatUint(uint64(reuse), 10)
    }

    // allocate
    calltable[reuse].gc=false
    calltable[reuse].disposable=false
    calltable[reuse].gcShyness=0
    numlookup.lmset(reuse, newName)
    fnlookup.lmset(newName,reuse)
    if cs.prepared==true {
        cs.fs=newName
        cs.disposable=false
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
var globlock   = &sync.RWMutex{}  // enum access lock
var sglock     = &sync.RWMutex{}  // setglob lock


// for error reporting : keeps a list of parent->child function calls
//   will probably blow up during recursion.
//   errorChain tracks the full call stack (with caller/line info) for error reporting only.
var errorChain []chainInfo

// defined function entry point
// everything about what is to be executed is contained in calltable[csloc]
func Call(ctx context.Context, varmode uint8, ident *[]Variable, csloc uint32, registrant uint8, method bool, method_value any, kind_override string, arg_names []string, va ...any) (retval_count uint8,endFunc bool,method_result any,callErr error) {

    /*
    dispifs,_:=fnlookup.lmget(calltable[csloc].fs)
    pf("-- Call()\n  -func %s\n  - fs %d\n  - base %d\n  - ident addr : %v\n",
        calltable[csloc].fs,
        dispifs,
        calltable[csloc].base,
        &ident,
    )
    */

    display_fs,_:=numlookup.lmget(calltable[csloc].base)

    calllock.Lock()

    // register call
    caller_str,_:=numlookup.lmget(calltable[csloc].caller)
    if caller_str=="global" { caller_str="main" }

    if calltable[csloc].caller!=0 {
        errorChain=append(errorChain,chainInfo{loc:calltable[csloc].caller,name:caller_str,registrant:registrant})
    }

    // profile setup

    if enableProfiling {
        pushToCallChain(ctx,display_fs)
        startProfile(caller_str)
    }
    startTime:=time.Now()

    // set up evaluation parser - one per function
    parser:=&leparser{}
    parser.ident=ident
    parser.kind_override=kind_override
    parser.ctx = ctx

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
    var source_base uint32              // location of the translated source tokens

    // error handler
    defer func() {
        if r := recover(); r != nil {

            // fall back to shell command?
            if interactive && !parser.hard_fault && !parser.std_call && permit_cmd_fallback {
                cmd:=basecode[source_base][parser.pc].Original

                s:=interpolate(currentModule,1,&mident,cmd)
                s=str.TrimRight(s,"\n")
                if len(s) > 0 {
                    cop := Copper(s, false)
                    gvset("@last",cop.code)
                    gvset("@last_err",cop.err)
                    if ! cop.okay {
                        pf("Error: [%d] in shell command '%s'\n", cop.code, str.TrimLeft(s," \t"))
                        pf(cop.err+"\n")
                    } else {
                        if len(cop.out) > 0 {
                            if cop.out[len(cop.out)-1] != '\n' {
                                cop.out += "\n"
                            }
                            pf("%s", cop.out)
                        }
                    }
                }
                if row>=MH-BMARGIN {
                    if row>MH { row=MH }
                    for past:=row-(MH-BMARGIN);past>0;past-- { at(MH+1,1); fmt.Print("\n") }
                    row=MH-BMARGIN
                }
            } else {

                if !enforceError {
                    callErr = errors.New(sf("suppressed panic: %v",r))
                    return
                }

                parser.hard_fault=false
                if _,ok:=r.(runtime.Error); ok {
                    parser.report(inbound.SourceLine,sf("\n%v\n",r))
                    if debugMode { err:=r.(error); panic(err) }
                    finish(false,ERR_EVAL)
                }
                // err:=r.(error)
				var err error
				if errVal, ok := r.(error); ok {
					err = errVal
				} else {
					err = errors.New(sf("%v", r))
				}
                parser.report(inbound.SourceLine,sf("\n%v\n",err))
                if debugMode { panic(r) }
                setEcho(true)
                finish(false,ERR_EVAL)
            }
        }
    }()

    // some tracking variables for this function call
    var break_count int                 // usually 0. when >0 stops breakIn from resetting
                                        //  used for multi-level breaks.
    var breakIn int64                   // true during transition from break to outer.
    var forceEnd bool                   // used by BREAK for skipping context checks when
                                        //  bailing from nested constructs.
    var retvalues []any                 // return values to be passed back
    var finalline int16                 // tracks end of tokens in the function
    var fs string                       // current function space name
    var thisLoop *s_loop                // pointer to loop information. used in FOR

    // set up the function space

    // -- get call details
    calllock.Lock()
    // unique name for this execution, pre-generated before call
    fs = calltable[csloc].fs

    // where the tokens are:
    source_base = calltable[csloc].base

    currentModule=basemodmap[source_base]
    parser.namespace    = currentModule
    interparse.namespace= currentModule
    // pf("in call to %s currentModule set to : %s\n",fs,currentModule)

    // the uint32 id attached to fs name
    ifs,_:=fnlookup.lmget(fs)
    calllock.Unlock()

    // fake a filename to ifs relationship, for debugger use.
    // fileMap.Store(ifs,source_base)
    fileMap.Store(ifs,currentModule)

    // pf("Inside Call() : pre-statement-loop : current ifs=%d\n",ifs)

    // -- generate bindings

    bindlock.Lock()

    // reset bindings
    if ifs>=uint32(cap(bindings)) {
        bindResize()
    }
    if varmode==MODE_NEW {
    bindings[ifs]=make(map[string]uint64)
    }

    // copy bindings from source tokens
    for _,phrase:=range functionspaces[source_base] {
        for _,tok:=range phrase.Tokens {
            if tok.tokType==Identifier {
                bindings[ifs][tok.tokText]=tok.bindpos
            }
        }
    }
    // pf("Binding table from tokens is:\n%#v\n",bindings[ifs])

    bindlock.Unlock()


    if varmode==MODE_NEW {
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
    var lastConstruct     = []int64{}

    // initialise condition states: CASE stack depth
    // initialise the loop positions: FOR, FOREACH, WHILE

    // active CASE..ENDCASE statement meta info
    var wc     = make([]caseCarton, CASE_CAP)

    // count of active CASE..ENDCASE statements
    var wccount int

    // counters per loop type
    var loops     = make([]s_loop, MAX_LOOPS)

    // assign self from calling object
    if method {
        bin:=bind_int(ifs,"self")
        vset(nil,ifs,ident,"self", method_value)
        t:=(*ident)[bin]
        t.ITyped=false
        t.declared=true
        t.Kind_override=kind_override
        (*ident)[bin]=t
    }

tco_reentry:

    // assign value to local vars named in functionArgs (the call parameters)
    //  from each va value.
    // - functionArgs[] created at definition time from the call signature

    farglock.RLock()

    if len(va)>0 {
        if method { va=va[1:] }
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
                panic(errors.New(sf("missing required argument: %s",argName)))
            }
        }

        if s,ok:=value.(string); ok {
            value=interpolate(currentModule,ifs,ident,s)
        }
        vset(nil, ifs, ident, argName, value)
    }

    farglock.RUnlock()


    if len(functionspaces[source_base])>32767 {
        parser.report(-1,"function too long!")
        finish(true,ERR_SYNTAX)
        return
    }

    // pf("Call Args: %#v\n",va)

    finalline = int16(len(functionspaces[source_base]))

    inside_test := false      // are we currently inside a test bock
    inside_with := false      // WITH cannot be nested and remains local in scope.

    var structMode bool       // are we currently defining a struct
    var structName string     // name of struct currently being defined
    var structNode []any      // struct builder
    var defining bool         // are we currently defining a function. takes priority over structmode.
    var definitionName string // ... if we are, what is it called

    parser.pc = -1            // program counter : increments to zero at start of loop

    var si bool
    var we ExpressionCarton   // pre-allocated for general expression results eval
    var expr any              // pre-allocated for wrapped expression results eval
    var err error

    typeInvalid:=false        // used during struct building for indicating type validity.
    statement:=Error


    // debug mode stuff:

    activeDebugContext = parser

    if debugMode && ifs<3 {
        pf("[#fgreen]Debugger is active. Pausing before startup.[#-]\n")
        debugger.enterDebugger(0,functionspaces[source_base], ident, &mident, &gident)
    }


    // main statement loop:

    for {

        parser.pc+=1

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
            key:=(uint64(ifs) << 32) | uint64(parser.pc)
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

     ondo_reenter:  // on..do re-enters here because it creates the new phrase in advance and
                    //  we want to leave the program counter unaffected.

        statement=inbound.Tokens[0].tokType

        // finally... start processing the statement.


        /////// LINE DEBUG //////////////////////////////////////////////////////
        if lineDebug {
            clr:="2"
            if defining || statement==C_Define {
                clr="4"
            }
            pf("[#dim][#7]%20s: %5d : [#"+clr+"]%+v[#-]\n",display_fs,inbound.SourceLine+1,basecode[source_base][parser.pc])
        }
        /////////////////////////////////////////////////////////////////////////


        // append statements to a function if currently inside a DEFINE block.
        if defining && statement != C_Enddef {
            lmv,_:=fnlookup.lmget(definitionName)
            fspacelock.Lock()
            functionspaces[lmv] = append(functionspaces[lmv], *inbound)
            basecode_entry      = &basecode[source_base][parser.pc]
            basecode[lmv]       = append(basecode[lmv], *basecode_entry)
            // although we have added all the tokens in to the new source_base,
            // we still have to add identifier bindings in the new source_base
            // for the replicated inbound lines.
            for _,itok:=range inbound.Tokens {
                if itok.tokType==Identifier {
                    itok.bindpos=bind_int(lmv,itok.tokText)
                    itok.bound=true
                }
            }
            fspacelock.Unlock()
            continue
        }

        // struct building
        if structMode && statement!=C_Endstruct {

            if statement!=C_Define && statement!=C_Enddef {

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
                case "int","float","string","bool","uint","uint8","bigi","bigf","byte","mixed","any","[]":
                case "[]int","[]float","[]string","[]bool","[]uint","[]uint8","[]bigi","[]bigf","[]byte":
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
        }

        // show var references for -V arg
        if var_refs {
            switch statement {
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
            if (breakIn==C_For || breakIn==C_Foreach) && statement!=C_Endfor { continue }
            if breakIn==C_While && statement!=C_Endwhile { continue }
            if breakIn==C_Case && statement!=C_Endcase { continue }
        }
        ////////////////////////////////////////////////////////////////


        // main parsing for statements starts here:

        switch statement {

        case C_Var: // permit declaration with a default value

            //   'VAR' name [ ',' ... nameX ] [ '[' [size] ']' ] type [ '=' expr ]
            // | 'VAR' name struct_name
            // | 'VAR' aryname []struct_name

            var name_list []string
            var name_pos  []uint64
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
                    inter:=interpolate(currentModule,ifs,ident,inbound.Tokens[c].tokText)
                    name_list=append(name_list,inter)
                    name_pos =append(name_pos,uint64(c))
                    // pf("nl : %s , np : %d\n",inter,c)
                case O_Comma:
                    if !expectingComma {
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


            // look for ary setup or namespaced struct name

            var hasAry bool
            var size int
            found_namespace:=""

            if !varSyntaxError {
                // continue from last 'c' value

                // namespace check
                for dcpos:=c;dcpos<eqPos;dcpos+=1 {
                    if inbound.Tokens[dcpos].tokType==SYM_DoubleColon {
                        found_namespace=inbound.Tokens[dcpos-1].tokText
                        break
                    }
                }
               
                if found_namespace=="" {
                    found_namespace=parser.namespace
                    if c+1<inbound.TokenCount {
                        if found:=uc_match_func(inbound.Tokens[c+1].tokText); found!="" {
                            found_namespace=found
                        }
                    }
                }

                if inbound.Tokens[c].tokType==LeftSBrace {

                    // find RightSBrace
                    var d int16
                    for d=eqPos-1; d>c; d-=1 {
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


            // name iterations

            for nlp,vname:=range name_list {

                var sid uint64
                if strcmp(vname,inbound.Tokens[name_pos[nlp]].tokText) { // no interpol done:
                    sid=inbound.Tokens[name_pos[nlp]].bindpos
                } else {
                    sid=bind_int(ifs,vname)
                }

                // resize ident if required:
                if sid>=uint64(len(*ident)) {
                    newIdent:=make([]Variable,sid+identGrowthSize)
                    copy(newIdent,*ident)
                    *ident=newIdent
                }

                // get the required type
                var new_type_token_string string
                type_token_string := inbound.Tokens[eqPos-1].tokText

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
                if _,found:=Typemap[new_type_token_string]; found {

                    t:=Variable{}

                    if new_type_token_string!="map" {
                        t.IValue = reflect.New(Typemap[new_type_token_string]).Elem().Interface()
                    }

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
                    case "uint64","uxlong":
                        t.IKind=kuint64
                    case "mixed":
                        t.IKind=kany
                    case "any":
                        t.IKind=kany
                    case "[]bool":
                        t.IKind=ksbool
                        t.IValue=make([]bool,size,size)
                    case "[]int":
                        t.IKind=ksint
                        t.IValue=make([]int,size,size)
                    case "[]uint":
                        t.IKind=ksuint
                        t.IValue=make([]uint,size,size)
                    case "[]uint64","[]uxlong":
                        t.IKind=ksuint64
                        t.IValue=make([]uint64,size,size)
                    case "[]float":
                        t.IKind=ksfloat
                        t.IValue=make([]float64,size,size)
                    case "[]string":
                        t.IKind=ksstring
                        t.IValue=make([]string,size,size)
                    case "[]byte","[]uint8":
                        t.IKind=ksbyte
                        t.IValue=make([]uint8,size,size)
                    case "[]","[]mixed","[]any","[]interface {}":
                        t.IKind=ksany
                        t.IValue=make([]any,size,size)
                    case "map":
                        t.IKind=kmap
                        t.IValue=make(map[string]any,size)
                        gob.Register(t.IValue)
                    case "bigi":
                        t.IKind=kbigi
                        t.IValue=big.NewInt(0)
                    case "bigf":
                        t.IKind=kbigf
                        t.IValue=big.NewFloat(0)
                    case "[]bigi":
                        t.IKind=ksbigi
                        t.IValue=make([]*big.Int,size,size)
                    case "[]bigf":
                        t.IKind=ksbigf
                        t.IValue=make([]*big.Float,size,size)
                    }


                    // if we had a default value, stuff it in here...

                    if hasValue && new_type_token_string!="map" {

                        // deal with bigs first:
                        var tmp any

                        if t.IKind==kbigi || t.IKind==kbigf {
                            switch t.IKind {
                            case kbigi:
                                tmp=GetAsBigInt(we.result)
                            case kbigf:
                                tmp=GetAsBigFloat(we.result)
                            }
                            switch tmp:=tmp.(type) {
                            case *big.Int, *big.Float:
                                t.IValue=tmp
                            default:
                                parser.report(inbound.SourceLine,sf("type mismatch in VAR assignment (need a big, got %T)",tmp))
                                finish(false,ERR_EVAL)
                            }
                        } else {
                            // ... then other types:
                            new_type_token_string=str.Replace(new_type_token_string,"float","float64",-1)
                            new_type_token_string=str.Replace(new_type_token_string,"any","interface {}",-1)
                            if sf("%T",we.result)!=new_type_token_string {
                                parser.report(inbound.SourceLine,sf("type mismatch in VAR assignment (need %s, got %T)",new_type_token_string,we.result))
                                finish(false,ERR_EVAL)
                                break
                            } else {
                                t.IValue=we.result
                            }
                        }
                    }

                    // write temp to ident
                    (*ident)[sid]=t
                    // pf("wrote var: %#v\n... with sid of #%d\n",t,sid)

                } else {
                    // unknown type: check if it is a struct name

                    isStruct:=false
                    structvalues:=[]any{}

                    // handle namespace presence
                    checkstr:=type_token_string
                    sname:=found_namespace+"::"+checkstr
                    cpos:=str.IndexByte(checkstr,':')
                    if cpos!=-1 {
                        if len(checkstr)>cpos+1 {
                            if checkstr[cpos+1]==':' {
                                sname=checkstr
                            }
                        }
                    }

                    /*
                    pf("(struct check) found_namespace is %s\n",found_namespace)
                    pf("(struct check) type_token_string is %s\n",type_token_string)
                    pf("(struct check) sname is %s\n",sname)
                    pf("(struct check) structmaps:\n%#v\n",structmaps)
                    */

                    // structmap has list of field_name,field_type,... for each struct
                    // structvalues: [0] name [1] type [2] boolhasdefault [3] default_value
                    for sn, _ := range structmaps {
                        if sn==sname {
                            isStruct=true
                            structvalues=structmaps[sn]
                            break
                        }
                    }

                    if isStruct {
                        t:=(*ident)[sid]
                        err=fillStruct(&t,structvalues,Typemap,hasAry,[]string{})
                        if err!=nil {
                            parser.report(inbound.SourceLine,err.Error())
                            finish(false,ERR_EVAL)
                            break
                        }
                        t.IName=vname
                        t.ITyped=false
                        t.declared=true
                        t.Kind_override=sname
                        (*ident)[sid]=t

                    } else {
                        parser.report(inbound.SourceLine,sf("unknown data type requested '%v'",sname))
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
                arg:=inbound.Tokens[1]
                switch arg.tokType {
                case O_Minus:
                    uc_reset()
                case Identifier:
                    switch str.ToLower(arg.tokText) {
                    case "push":
                        ucs_push()
                    case "pop":
                        if ucs_pop()==false {
                            parser.report(inbound.SourceLine,sf("Cannot pop an empty stack in USE command."))
                        }
                    default:
                        parser.report(inbound.SourceLine,sf("Unknown argument in USE (%s).",arg.tokText))
                        finish(false, ERR_SYNTAX)
                    }
                default:
                    parser.report(inbound.SourceLine,sf("Unknown argument in USE (%s).",arg.tokText))
                    finish(false, ERR_SYNTAX)
                }
            case 3:
                arg1:=inbound.Tokens[1]
                arg2:=inbound.Tokens[2]
                switch arg1.tokType {
                case O_Minus:
                    uc_remove(arg2.tokText)
                case O_Plus:
                    uc_add(arg2.tokText)
                case SYM_Caret:
                    uc_top(arg2.tokText)
                default:
                    parser.report(inbound.SourceLine,sf("Unknown argument in USE (%s).",arg1.tokText))
                    finish(false, ERR_SYNTAX)
                }
            default:
                parser.report(inbound.SourceLine,sf("USE keyword has invalid arguments."))
                finish(false, ERR_SYNTAX)
            }


        // @note: use this at your own risk... (experimental)
        case C_Namespace:
            switch inbound.TokenCount {
            case 2:
                ns:=inbound.Tokens[1].tokText
                parser.namespace=ns
                interparse.namespace=ns
                currentModule=ns
            default:
                parser.report(inbound.SourceLine,sf("NAMESPACE needs a single argument."))
                finish(false, ERR_SYNTAX)
            }


        case C_While:

            var endfound bool
            var enddistance int16

            endfound, enddistance, _ = lookahead(source_base, parser.pc, 0, 0, C_Endwhile, []int64{C_While}, []int64{C_Endwhile})
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
                parser.report(inbound.SourceLine,"ENDWHILE outside of WHILE loop")
                finish(false, ERR_SYNTAX)
                break
            }

            // time to die?
            if breakIn == C_While {
                depth-=1
                lastConstruct = lastConstruct[:depth]
                breakIn = Error
                forceEnd=false
                break_count-=1
                if break_count>0 {
                    switch lastConstruct[depth-1] {
                    case C_For,C_Foreach,C_While,C_Case:
                        breakIn=lastConstruct[depth-1]
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

            // fmt.Printf("(sg) in fs %d (mident->%d) eval -> %+v\n",ifs,parser.mident,inbound.Tokens[1:])
            atomic.StoreUint32(&has_global_lock,ifs)
            sglock.Lock()
            if res:=parser.wrappedEval(parser.mident,&mident,ifs,ident,inbound.Tokens[1:]); res.evalError {
                parser.report(inbound.SourceLine,sf("Error in SETGLOB evaluation\n%+v\n",res.errVal))
                atomic.StoreUint32(&has_global_lock,0)
                sglock.Unlock()
                finish(false,ERR_EVAL)
                break
            }
            sglock.Unlock()
            atomic.StoreUint32(&has_global_lock,0)


        case C_Foreach:

            // FOREACH var [ : type ] IN expr
            // iterates over the result of expression expr as a list

            if inbound.TokenCount<4 {
                parser.report(inbound.SourceLine,"bad argument count in FOREACH.")
                finish(false,ERR_SYNTAX)
                break
            }

            skip:=0
            it_type:=""
            if inbound.Tokens[2].tokType == SYM_COLON {
                it_type=inbound.Tokens[3].tokText
                skip=2
                // valid type?
                // check if it_type is a key in either Typemap or structmaps
                // @todo: add a USE namespace lookup here
                otype:=it_type
                if !str.Contains(it_type,"::") {
                    it_type=parser.namespace+"::"+it_type
                }

                found:=false
                if _,found=structmaps[it_type]; !found {
                    _,found=Typemap[otype]
                }
                if ! found {
                    parser.report(inbound.SourceLine,sf("invalid type [%s] for iterator in FOREACH.",otype))
                    finish(false,ERR_SYNTAX)
                    break
                }
            }

            if inbound.Tokens[2+skip].tokType!=C_In {
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

            switch inbound.Tokens[3+skip].tokType {

            // cause evaluation of all terms following IN
            case SYM_BOR, O_InFile, ResultBlock, Block, NumericLiteral, StringLiteral, LeftSBrace, LParen, Identifier:

                we = parser.wrappedEval(ifs,ident,ifs,ident,inbound.Tokens[3+skip:])
                if we.evalError {
                    parser.report(inbound.SourceLine,sf("error evaluating term in FOREACH statement '%v'\n%+v\n",we.text,we.errVal))
                    finish(false,ERR_EVAL)
                    break
                }

                // ensure result block has content:
                switch we.result.(type) {
                case struct {out string; err string; code int; okay bool}:
                    // cast cmd results as their stdout string in loops
                    we.result=we.result.(struct {out string; err string; code int; okay bool}).out
                case string:
                default:
                    if inbound.Tokens[3+skip].tokType==ResultBlock {
                        parser.report(inbound.SourceLine,"system command did not return a string in FOREACH statement\n")
                        finish(false,ERR_EVAL)
                        break
                    }
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
                case []tui:
                    l=len(lv)
                case []*big.Int:
                    l=len(lv)
                case []*big.Float:
                    l=len(lv)
                case []dirent:
                    l=len(lv)
                case []alloc_info:
                    l=len(lv)
                case map[string]alloc_info:
                    l=len(lv)
                case map[string]dirent:
                    l=len(lv)
                case map[string]tui:
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
                case []map[string]any:
                    l=len(lv)
                case map[string]any:
                    l=len(lv)
                case [][]int:
                    l=len(lv)
                case []any:
                    l=len(lv)
                default:
                    pf("Unknown loop type [%T]\n",lv)
                }

                endfound, enddistance, _ := lookahead(source_base, parser.pc, 0, 0, C_Endfor, []int64{C_For,C_Foreach}, []int64{C_Endfor})
                if !endfound {
                    parser.report(inbound.SourceLine,"Cannot determine the location of a matching ENDFOR.")
                    finish(false, ERR_SYNTAX)
                    break
                }

                // skip empty expressions
                if l==0 {
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
                    if runtime.GOOS!="windows" {
                        we.result = str.Split(we.result.(string), "\n")
                    } else {
                        we.result = str.Split(str.Replace(we.result.(string), "\r\n", "\n", -1), "\n")
                    }

                    if len(we.result.([]string))>0 {
                        vset(nil, ifs, ident,"key_"+fid, 0)
                        vset(&inbound.Tokens[1], ifs, ident,fid, we.result.([]string)[0])
                        condEndPos = len(we.result.([]string)) - 1
                    }

                case map[string]float64:
                    if len(we.result.(map[string]float64)) > 0 {
                        // get iterator for this map
                        iter = reflect.ValueOf(we.result.(map[string]float64)).MapRange()

                        // set initial key and value
                        if iter.Next() {
                            vset(nil,ifs, ident,"key_"+fid, iter.Key().String())
                            vset(&inbound.Tokens[1],ifs, ident, fid, iter.Value().Interface())
                        }
                        condEndPos = len(we.result.(map[string]float64)) - 1
                    }

                case map[string]tui:
                    if len(we.result.(map[string]tui)) > 0 {
                        iter = reflect.ValueOf(we.result.(map[string]tui)).MapRange()
                        if iter.Next() {
                            vset(nil,ifs, ident,"key_"+fid, iter.Key().String())
                            vset(&inbound.Tokens[1],ifs, ident, fid, iter.Value().Interface())
                        }
                        condEndPos = len(we.result.(map[string]tui)) - 1
                    }

                case map[string]alloc_info:
                    if len(we.result.(map[string]alloc_info)) > 0 {
                        iter = reflect.ValueOf(we.result.(map[string]alloc_info)).MapRange()
                        if iter.Next() {
                            vset(nil,ifs, ident,"key_"+fid, iter.Key().String())
                            vset(&inbound.Tokens[1],ifs, ident, fid, iter.Value().Interface())
                        }
                        condEndPos = len(we.result.(map[string]alloc_info)) - 1
                    }

                case map[string]dirent:
                    if len(we.result.(map[string]dirent)) > 0 {
                        iter = reflect.ValueOf(we.result.(map[string]dirent)).MapRange()
                        if iter.Next() {
                            vset(nil,ifs, ident,"key_"+fid, iter.Key().String())
                            vset(&inbound.Tokens[1],ifs, ident, fid, iter.Value().Interface())
                        }
                        condEndPos = len(we.result.(map[string]dirent)) - 1
                    }

                case map[string]bool:
                    if len(we.result.(map[string]bool)) > 0 {
                        iter = reflect.ValueOf(we.result.(map[string]bool)).MapRange()
                        if iter.Next() {
                            vset(nil,ifs, ident,"key_"+fid, iter.Key().String())
                            vset(&inbound.Tokens[1],ifs, ident, fid, iter.Value().Interface())
                        }
                        condEndPos = len(we.result.(map[string]bool)) - 1
                    }

                case map[string]uint:
                    if len(we.result.(map[string]uint)) > 0 {
                        iter = reflect.ValueOf(we.result.(map[string]uint)).MapRange()
                        if iter.Next() {
                            vset(nil,ifs, ident,"key_"+fid, iter.Key().String())
                            vset(&inbound.Tokens[1],ifs, ident, fid, iter.Value().Interface())
                        }
                        condEndPos = len(we.result.(map[string]uint)) - 1
                    }

                case map[string]int:
                    if len(we.result.(map[string]int)) > 0 {
                        // get iterator for this map
                        iter = reflect.ValueOf(we.result.(map[string]int)).MapRange()

                        // set initial key and value
                        if iter.Next() {
                            vset(nil,ifs, ident,"key_"+fid, iter.Key().String())
                            vset(&inbound.Tokens[1],ifs, ident, fid, iter.Value().Interface())
                        }
                        condEndPos = len(we.result.(map[string]int)) - 1
                    }

                case map[string]string:

                    if len(we.result.(map[string]string)) > 0 {

                        // get iterator for this map
                        iter = reflect.ValueOf(we.result.(map[string]string)).MapRange()
                        // set initial key and value
                        if iter.Next() {
                            vset(nil,ifs, ident,"key_"+fid, iter.Key().String())
                            vset(&inbound.Tokens[1],ifs, ident, fid, iter.Value().Interface())
                        }
                        condEndPos = len(we.result.(map[string]string)) - 1
                    }

                case map[string][]string:

                    if len(we.result.(map[string][]string)) > 0 {

                        // get iterator for this map
                        iter = reflect.ValueOf(we.result.(map[string][]string)).MapRange()

                        // set initial key and value
                        if iter.Next() {
                            vset(nil,ifs, ident,"key_"+fid, iter.Key().String())
                            vset(&inbound.Tokens[1],ifs, ident, fid, iter.Value().Interface())
                        }
                        condEndPos = len(we.result.(map[string][]string)) - 1
                    }

                case []float64:

                    if len(we.result.([]float64)) > 0 {
                        vset(nil,ifs, ident,"key_"+fid, 0)
                        vset(&inbound.Tokens[1],ifs, ident, fid, we.result.([]float64)[0])
                        condEndPos = len(we.result.([]float64)) - 1
                    }

                case float64: // special case: float
                    we.result = []float64{we.result.(float64)}
                    if len(we.result.([]float64)) > 0 {
                        vset(nil,ifs, ident,"key_"+fid, 0)
                        vset(&inbound.Tokens[1],ifs, ident, fid, we.result.([]float64)[0])
                        condEndPos = len(we.result.([]float64)) - 1
                    }

                case []uint:
                    if len(we.result.([]uint)) > 0 {
                        vset(nil,ifs, ident,"key_"+fid, 0)
                        vset(&inbound.Tokens[1],ifs, ident, fid, we.result.([]uint)[0])
                        condEndPos = len(we.result.([]uint)) - 1
                    }

                case []bool:
                    if len(we.result.([]bool)) > 0 {
                        vset(nil,ifs, ident,"key_"+fid, 0)
                        vset(&inbound.Tokens[1],ifs, ident, fid, we.result.([]bool)[0])
                        condEndPos = len(we.result.([]bool)) - 1
                    }

                case []int:
                    if len(we.result.([]int)) > 0 {
                        vset(nil,ifs, ident,"key_"+fid, 0)
                        vset(&inbound.Tokens[1],ifs, ident, fid, we.result.([]int)[0])
                        condEndPos = len(we.result.([]int)) - 1
                    }

                case []*big.Int:
                    if len(we.result.([]*big.Int)) > 0 {
                        vset(nil,ifs, ident,"key_"+fid, 0)
                        vset(&inbound.Tokens[1],ifs, ident, fid, we.result.([]*big.Int)[0])
                        condEndPos = len(we.result.([]*big.Int)) - 1
                    }

                case []*big.Float:
                    if len(we.result.([]*big.Float)) > 0 {
                        vset(nil,ifs, ident,"key_"+fid, 0)
                        vset(&inbound.Tokens[1],ifs, ident, fid, we.result.([]*big.Float)[0])
                        condEndPos = len(we.result.([]*big.Float)) - 1
                    }

                case int: // special case: int
                    we.result = []int{we.result.(int)}
                    if len(we.result.([]int)) > 0 {
                        vset(nil,ifs, ident,"key_"+fid, 0)
                        vset(&inbound.Tokens[1],ifs, ident, fid, we.result.([]int)[0])
                        condEndPos = len(we.result.([]int)) - 1
                    }

                case []string:
                    if len(we.result.([]string)) > 0 {
                        vset(nil,ifs, ident,"key_"+fid, 0)
                        vset(&inbound.Tokens[1],ifs, ident, fid, we.result.([]string)[0])
                        condEndPos = len(we.result.([]string)) - 1
                    }

                case []tui:
                    if len(we.result.([]tui)) > 0 {
                        vset(nil,ifs, ident,"key_"+fid, 0)
                        vset(&inbound.Tokens[1],ifs, ident, fid, we.result.([]tui)[0])
                        condEndPos = len(we.result.([]tui)) - 1
                    }

                case []dirent:
                    if len(we.result.([]dirent)) > 0 {
                        vset(nil,ifs, ident,"key_"+fid, 0)
                        vset(&inbound.Tokens[1],ifs, ident, fid, we.result.([]dirent)[0])
                        condEndPos = len(we.result.([]dirent)) - 1
                    }

                case []alloc_info:
                    if len(we.result.([]alloc_info)) > 0 {
                        vset(nil,ifs, ident,"key_"+fid, 0)
                        vset(&inbound.Tokens[1],ifs, ident, fid, we.result.([]alloc_info)[0])
                        condEndPos = len(we.result.([]alloc_info)) - 1
                    }

                case [][]int:
                    if len(we.result.([][]int)) > 0 {
                        vset(nil,ifs, ident,"key_"+fid, 0)
                        vset(&inbound.Tokens[1],ifs, ident, fid, we.result.([][]int)[0])
                        condEndPos = len(we.result.([][]int)) - 1
                    }

                case []map[string]any:

                    if len(we.result.([]map[string]any)) > 0 {
                        vset(nil,ifs, ident,"key_"+fid, 0)
                        vset(&inbound.Tokens[1],ifs, ident, fid, we.result.([]map[string]any)[0])
                        condEndPos = len(we.result.([]map[string]any)) - 1
                    }

                case map[string]any:

                    if len(we.result.(map[string]any)) > 0 {

                        // get iterator for this map
                        iter = reflect.ValueOf(we.result.(map[string]any)).MapRange()

                        // set initial key and value
                        if iter.Next() {
                            vset(nil,ifs, ident,"key_"+fid, iter.Key().String())
                            vset(&inbound.Tokens[1],ifs, ident, fid, iter.Value().Interface())
                        }
                        condEndPos = len(we.result.(map[string]any)) - 1
                    }

                case []any:

                    if len(we.result.([]any)) > 0 {
                        vset(nil,ifs, ident,"key_"+fid, 0)
                        vset(&inbound.Tokens[1],ifs, ident, fid, we.result.([]any)[0])

                        bin:=bind_int(ifs,fid)
                        if it_type != "" {
                            t:=(*ident)[bin]
                            t.ITyped=true
                            t.declared=true
                            t.Kind_override=it_type
                            (*ident)[bin]=t
                        }

                        isStruct := reflect.TypeOf(we.result.([]any)[0]).Kind() == reflect.Struct
                        if isStruct && it_type=="" {
                            if s,count:=struct_match(we.result.([]any)[0]); count==1 {
                                (*ident)[bin].Kind_override=s
                            }
                        }

                        condEndPos = len(we.result.([]any)) - 1
                    }

                default:
                    parser.report(inbound.SourceLine,sf("Mishandled return of type '%T' from FOREACH expression '%v'\n", we.result,we.result))
                    finish(false,ERR_EVAL)
                    break
                }


                depth+=1
                lastConstruct = append(lastConstruct, C_Foreach)

                loops[depth] = s_loop{loopVar: fid, keyVar: "key_"+fid,
                    optNoUse: Opt_LoopStart,
                    repeatFrom: parser.pc + 1, iterOverMap: iter, iterOverArray: we.result,
                    counter: 0, condEnd: condEndPos, forEndPos: enddistance + parser.pc,
                    loopType: LT_FOREACH, itType: it_type,
                }

            default:
                    parser.report(inbound.SourceLine,"Unexpected expression type in FOREACH.")
                    finish(false, ERR_SYNTAX)
                    break

            }


        case C_For: // loop over an int64 range

            var iterAssignment []Token
            var iterCondition  []Token
            var iterAmendment  []Token
            customCond:=false

            // check for custom FOR setup
            // e.g. for x=0,x<10,x+=1

            commaList:=parser.splitCommaArray(inbound.Tokens[1:])
            if len(commaList)==3 {
                iterAssignment= commaList[0]
                iterCondition = commaList[1]
                iterAmendment = commaList[2]
                foundAssign:=false
                if len(iterAssignment)>0 {
                    // has an equals? then do assignment
                    for eqPos:=0; eqPos<len(iterAssignment); eqPos+=1 {
                        if inbound.Tokens[eqPos].tokType == O_Assign {
                            foundAssign=true
                            default_value := parser.wrappedEval(ifs,ident,ifs,ident,iterAssignment)
                            if default_value.evalError {
                                foundAssign=false
                            }
                            break
                        }
                    }
                    if !foundAssign {
                        parser.report(inbound.SourceLine,sf("Invalid assignment in FOR (%+v)",iterAssignment))
                        finish(false,ERR_SYNTAX)
                        break
                    }
                }
                customCond=true

                // figure end position
                endfound, enddistance, _ := lookahead(source_base, parser.pc, 0, 0, C_Endfor, []int64{C_For,C_Foreach}, []int64{C_Endfor})
                if !endfound {
                    parser.report(inbound.SourceLine,"Cannot determine the location of a matching ENDFOR.")
                    finish(false, ERR_SYNTAX)
                    break
                }

                depth+=1
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
                    expr, err = parser.Eval(ifs,inbound.Tokens[3:toAt])
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
                endfound, enddistance, _ := lookahead(source_base, parser.pc, 0, 0, C_Endfor, []int64{C_For,C_Foreach}, []int64{C_Endfor})
                if !endfound {
                    parser.report(inbound.SourceLine,"Cannot determine the location of a matching ENDFOR.")
                    finish(false, ERR_SYNTAX)
                    break
                }

                // @note: if loop counter is never used between here and C_Endfor, then don't vset the local var

                // store loop data
                fid:=inbound.Tokens[1].tokText

                // prepare loop counter binding
                bin:=inbound.Tokens[1].bindpos

                depth+=1
                loops[depth] = s_loop{
                    loopVar:  fid,
                    keyVar: "key_"+fid,
                    loopVarBinding: bin,
                    optNoUse: Opt_LoopStart,
                    loopType: LT_FOR, forEndPos: parser.pc + enddistance, repeatFrom: parser.pc + 1,
                    counter: fstart, condEnd: fend,
                    repeatAction: direction, repeatActionStep: step,
                }

                // store loop start condition
                vset(&inbound.Tokens[1],ifs, ident, fid, fstart)

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

            } // end-not-custom-cond

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

            if breakIn!=C_For && breakIn!=C_Foreach {

                switch (*thisLoop).loopType {

                case LT_FOREACH: // move through range

                    (*thisLoop).counter+=1

                    // set only on first iteration, keeps optNoUse consistent with C_For
                    if (*thisLoop).optNoUse == Opt_LoopStart {
                        (*thisLoop).optNoUse = Opt_LoopSet
                    }

                    it_type:=(*thisLoop).itType

                    if (*thisLoop).counter > (*thisLoop).condEnd {
                        loopEnd = true
                    } else {

                        // assign value back to local variable

                        switch (*thisLoop).iterOverArray.(type) {

                        // map ranges are randomly ordered!!
                        case map[string]any, map[string]alloc_info, map[string]tui, map[string]dirent, map[string]int, map[string]uint, map[string]bool, map[string]float64, map[string]string, map[string][]string:
                            if (*thisLoop).iterOverMap.Next() { // true means not exhausted
                                vset(nil,ifs, ident, (*thisLoop).keyVar, (*thisLoop).iterOverMap.Key().String())
                                vset(nil,ifs, ident, (*thisLoop).loopVar, (*thisLoop).iterOverMap.Value().Interface())
                            }

                        case []bool:
                            vset(nil,ifs, ident,(*thisLoop).keyVar, (*thisLoop).counter)
                            vset(nil,ifs, ident, (*thisLoop).loopVar, (*thisLoop).iterOverArray.([]bool)[(*thisLoop).counter])
                        case []int:
                            vset(nil,ifs, ident,(*thisLoop).keyVar, (*thisLoop).counter)
                            vset(nil,ifs, ident, (*thisLoop).loopVar, (*thisLoop).iterOverArray.([]int)[(*thisLoop).counter])
                        case []uint:
                            vset(nil,ifs, ident,(*thisLoop).keyVar, (*thisLoop).counter)
                            vset(nil,ifs, ident, (*thisLoop).loopVar, (*thisLoop).iterOverArray.([]uint8)[(*thisLoop).counter])
                        case []uint64:
                            vset(nil,ifs, ident,(*thisLoop).keyVar, (*thisLoop).counter)
                            vset(nil,ifs, ident, (*thisLoop).loopVar, (*thisLoop).iterOverArray.([]uint64)[(*thisLoop).counter])
                        case []string:
                            vset(nil,ifs, ident,(*thisLoop).keyVar, (*thisLoop).counter)
                            vset(nil,ifs, ident, (*thisLoop).loopVar, (*thisLoop).iterOverArray.([]string)[(*thisLoop).counter])
                        case []dirent:
                            vset(nil,ifs, ident,(*thisLoop).keyVar, (*thisLoop).counter)
                            vset(nil,ifs, ident, (*thisLoop).loopVar, (*thisLoop).iterOverArray.([]dirent)[(*thisLoop).counter])
                        case []tui:
                            vset(nil,ifs, ident,(*thisLoop).keyVar, (*thisLoop).counter)
                            vset(nil,ifs, ident, (*thisLoop).loopVar, (*thisLoop).iterOverArray.([]tui)[(*thisLoop).counter])
                        case []alloc_info:
                            vset(nil,ifs, ident,(*thisLoop).keyVar, (*thisLoop).counter)
                            vset(nil,ifs, ident, (*thisLoop).loopVar, (*thisLoop).iterOverArray.([]alloc_info)[(*thisLoop).counter])
                        case []float64:
                            vset(nil,ifs, ident,(*thisLoop).keyVar, (*thisLoop).counter)
                            vset(nil,ifs, ident, (*thisLoop).loopVar, (*thisLoop).iterOverArray.([]float64)[(*thisLoop).counter])
                        case []*big.Int:
                            vset(nil,ifs, ident,(*thisLoop).keyVar, (*thisLoop).counter)
                            vset(nil,ifs, ident, (*thisLoop).loopVar, (*thisLoop).iterOverArray.([]*big.Int)[(*thisLoop).counter])
                        case []*big.Float:
                            vset(nil,ifs, ident,(*thisLoop).keyVar, (*thisLoop).counter)
                            vset(nil,ifs, ident, (*thisLoop).loopVar, (*thisLoop).iterOverArray.([]*big.Float)[(*thisLoop).counter])
                        case [][]int:
                            vset(nil,ifs, ident,(*thisLoop).keyVar, (*thisLoop).counter)
                            vset(nil,ifs, ident, (*thisLoop).loopVar, (*thisLoop).iterOverArray.([][]int)[(*thisLoop).counter])
                        case []map[string]any:
                            vset(nil,ifs, ident,(*thisLoop).keyVar, (*thisLoop).counter)
                            vset(nil,ifs, ident, (*thisLoop).loopVar, (*thisLoop).iterOverArray.([]map[string]any)[(*thisLoop).counter])
                        case []any:
                            vset(nil,ifs, ident,(*thisLoop).keyVar, (*thisLoop).counter)
                            vset(nil,ifs, ident, (*thisLoop).loopVar, (*thisLoop).iterOverArray.([]any)[(*thisLoop).counter])
                        default:
                            // @note: should put a proper exit in here.
                            pv,_:=vget(nil,ifs,ident,sf("%v",(*thisLoop).iterOverArray.([]float64)[(*thisLoop).counter]))
                            pf("Unknown type [%T] in END/Foreach\n",pv)
                        }

                        bin:=bind_int(ifs,(*thisLoop).loopVar)
                        if it_type != "" {
                            t:=(*ident)[bin]
                            t.ITyped=true
                            t.Kind_override=it_type
                            (*ident)[bin]=t
                        }

                        isStruct := reflect.TypeOf((*ident)[bin].IValue).Kind() == reflect.Struct
                        if isStruct && it_type=="" {
                            if s,count:=struct_match((*ident)[bin].IValue); count==1 {
                                (*ident)[bin].Kind_override=s
                            }
                        }


                    }

                case LT_FOR: // move through range

                    if (*thisLoop).repeatCustom {

                        // amend iterator
                        if len((*thisLoop).repeatAmendment)>0 {
                            evAmendment := parser.wrappedEval(ifs,ident,ifs,ident,(*thisLoop).repeatAmendment)
                            if evAmendment.evalError {
                                parser.report(inbound.SourceLine,"Invalid expression for amendment in FOR")
                                finish(false, ERR_EVAL)
                                break
                            }
                        }

                        // check iterator
                        if len((*thisLoop).repeatCond)>0 {
                            evCond := parser.wrappedEval(ifs,ident,ifs,ident,(*thisLoop).repeatCond)
                            if evCond.evalError {
                                parser.report(inbound.SourceLine,"Invalid condition for amendment in FOR")
                                finish(false, ERR_EVAL)
                                break
                            }
                            loopEnd=true
                            switch evCond.result.(type) {
                            case bool:
                                if evCond.result.(bool) {
                                    loopEnd=false
                                }
                            default:
                                parser.report(inbound.SourceLine,"Condition does not evaluate to a bool in FOR")
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
                                    (*ident)[(*thisLoop).loopVarBinding].IValue=(*thisLoop).counter
                                }
                                loopEnd = true
                            }
                        case ACT_DEC:
                            if (*thisLoop).counter < (*thisLoop).condEnd {
                                (*thisLoop).counter -= (*thisLoop).repeatActionStep
                                if (*thisLoop).optNoUse == Opt_LoopIgnore {
                                    (*ident)[(*thisLoop).loopVarBinding].IValue=(*thisLoop).counter
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
                            (*ident)[(*thisLoop).loopVarBinding].IValue=(*thisLoop).counter
                        }

                    }

                }

            } else {
                // time to die, mr bond? C_Break reached
                if ( (*thisLoop).loopType==LT_FOR && breakIn==C_For ) || ( (*thisLoop).loopType==LT_FOREACH && breakIn==C_Foreach ) {
                    // pf("**reached break reset**\n")
                    breakIn = Error // reset to unbroken
                    forceEnd=false
                    loopEnd = true
                }
            }

            if loopEnd {
                // leave the loop
                depth-=1
                lastConstruct = lastConstruct[:depth]
                breakIn = Error // reset to unbroken
                forceEnd=false
                if break_count>0 {
                    break_count-=1
                    if break_count>0 {
                        switch lastConstruct[depth-1] {
                        case C_For,C_Foreach,C_While,C_Case:
                            breakIn=lastConstruct[depth-1]
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

                case C_Case:
                    // mark this as an error for now, as we don't currently
                    //  backtrack through lastConstruct to check the actual
                    //  loop type so that it can be properly unwound.
                    parser.report(inbound.SourceLine,"Attempting to CONTINUE inside a CASE is not permitted.")
                    finish(false,ERR_SYNTAX)

                }

            }


        case C_Break:

            // Break should work with either FOR, FOREACH, WHILE or CASE.

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
                    forceEnd=false

                    // /* @note: removed as buggy when breaking from nested for/foreach combination:

                    var efound,er bool
                    switch inbound.Tokens[1].tokType {
                    case C_Case:
                        efound,_,er=lookahead(source_base,parser.pc,1,0,C_Endcase, []int64{C_Case},[]int64{C_Endcase})
                        breakIn=C_Case
                        forceEnd=true
                        parser.pc = wc[wccount].endLine - 1
                    case C_For:
                        efound,_,er=lookahead(source_base,parser.pc,1,0,C_Endfor,[]int64{C_For,C_Foreach},[]int64{C_Endfor})
                        breakIn=C_For
                        forceEnd=true
                        parser.pc = (*thisLoop).forEndPos - 1
                    case C_Foreach:
                        efound,_,er=lookahead(source_base,parser.pc,1,0,C_Endfor,[]int64{C_For,C_Foreach},[]int64{C_Endfor})
                        breakIn=C_Foreach
                        forceEnd=true
                        parser.pc = (*thisLoop).forEndPos - 1
                    case C_While:
                        efound,_,er=lookahead(source_base,parser.pc,1,0,C_Endwhile,[]int64{C_While},[]int64{C_Endwhile})
                        breakIn=C_While
                        forceEnd=true
                        parser.pc = (*thisLoop).whileContinueAt-1
                    }
                    if er {
                        // lookahead error
                        parser.report(inbound.SourceLine,sf("BREAK [%s] cannot find end of construct",tokNames[breakIn]))
                        finish(false, ERR_SYNTAX)
                        break
                    }
                    if efound {
                        // break jump point is set, so continue pc loop 
                        continue
                    }
                    //*/
                }

                if !forceEnd {
                    // break by expression
                    break_depth:=parser.wrappedEval(ifs,ident,ifs,ident,inbound.Tokens[1:])
                    switch break_depth.result.(type) {
                    case int:
                        break_count=break_depth.result.(int)
                        // pf("-- break/expr int->%v\n",break_count)
                    default:
                        parser.report(inbound.SourceLine,"Could not evaluate BREAK depth argument")
                        finish(false,ERR_EVAL)
                        break
                    }
                }

                if forceEnd {
                    // set count of back tracking in end* statements
                    for break_count=1;break_count<=depth; break_count+=1 {
                        // pf("(cbreak) increasing break_count to %v\n",break_count)
                        lce:=lastConstruct[depth-break_count]
                        // pf("(cbreak) now processing lc type of %v\n",tokNames[lce])
                        if lce==C_Case {
                            wccount-=1
                        }
                        if lce==C_While {
                        }
                        if lce==inbound.Tokens[1].tokType {
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
                parser.report(inbound.SourceLine,"A grue is attempting to BREAK out. (Breaking without a surrounding context!)")
                // pf("breakin->%v depth->%v wccount->%v thisloop->%#v\n",breakIn,depth,wccount,thisLoop)
                // pf("breakcount->%v lastConstruct->%#v\n",break_count,lastConstruct[depth-1])
                finish(false, ERR_SYNTAX)
                break
            }


        case C_Enum:

            if inbound.TokenCount<4 || (
                ! (inbound.Tokens[2].tokType==LParen && inbound.Tokens[inbound.TokenCount-1].tokType==RParen) &&
                ! (inbound.Tokens[2].tokType==LeftCBrace && inbound.Tokens[inbound.TokenCount-1].tokType==RightCBrace)) {
                parser.report(inbound.SourceLine,"Incorrect arguments supplied for ENUM.")
                finish(false,ERR_SYNTAX)
                break
            }

            resu:=parser.splitCommaArray(inbound.Tokens[3:inbound.TokenCount-1])

            globlock.Lock()
            enum_name:=parser.namespace+"::"+inbound.Tokens[1].tokText
            enum[enum_name]=&enum_s{}
            enum[enum_name].members=make(map[string]any)
            enum[enum_name].namespace=parser.namespace
            globlock.Unlock()

            var nextVal any
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


        case C_Unset: // undeclare variables

            if inbound.TokenCount < 2 {
                parser.report(inbound.SourceLine,"Incorrect arguments supplied for UNSET.")
                finish(false, ERR_SYNTAX)
            } else {
                resu:=parser.splitCommaArray(inbound.Tokens[1:])
                for e:=0; e<len(resu); e++ {
                    if len(resu[e])==1 {
                        removee := resu[e][0].tokText
                        if (*ident)[resu[e][0].bindpos].declared {
                            vunset(ifs, ident, removee)
                        } else {
                            parser.report(inbound.SourceLine,sf("Variable %s does not exist.", removee))
                            finish(false, ERR_EVAL)
                            break
                        }
                    } else {
                        parser.report(inbound.SourceLine,sf("Invalid variable specification '%v' in UNSET.",resu[e]))
                        finish(false, ERR_EVAL)
                        break
                    }
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

            bc:=interpolate(currentModule,ifs,ident,basecode[source_base][parser.pc].borcmd)

            /*
            pf("\n")
            pf("In local command\nCalled with ifs:%d and tokens->%+v\n",ifs,inbound.Tokens)
            pf("  source_base -> %v\n",source_base)
            pf("  basecode    -> %v\n",basecode[source_base][parser.pc].Original)
            pf("  bor cmd     -> %#v\n",bc)
            pf("\n")
            */

            if inbound.TokenCount==2 && hasOuter(inbound.Tokens[1].tokText,'`') {
                s:=interpolate(currentModule,ifs,ident,stripOuter(inbound.Tokens[1].tokText,'`'))
                coprocCall(s)
            } else {
                coprocCall(bc)
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
                        // pf("(doc) term %+v nt %+v\n",term,nt)
                        if nt.tokType==LParen || nt.tokType==LeftSBrace  { evnest+=1 }
                        if nt.tokType==RParen || nt.tokType==RightSBrace { evnest-=1 }
                        if evnest==0 && (term==len(inbound.Tokens[1:])-1 || nt.tokType == O_Comma) {
                            v,_ := parser.Eval(ifs,inbound.Tokens[1+newstart:term+2])
                            newstart=term+1
                            switch v.(type) { case string: v=interpolate(currentModule,ifs,ident,v.(string)) }
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
                        testlock.Unlock()
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

                test_name = interpolate(currentModule,ifs,ident,stripOuterQuotes(inbound.Tokens[1].tokText, 2))
                test_group = interpolate(currentModule,ifs,ident,stripOuterQuotes(inbound.Tokens[3].tokText, 2))

                under_test = false
                // if filter matches group
                if test_name_filter=="" {
                    if matched, _ := regexp.MatchString(test_group_filter, test_group); matched {
                        vset(nil,ifs,ident,"_test_group",test_group)
                        vset(nil,ifs,ident,"_test_name",test_name)
                        under_test = true
                        appendToTestReport(test_output_file,ifs, parser.pc, sf("\nTest Section : [#5][#bold]%s/%s[#boff][#-]",test_group,test_name))
                    }
                } else {
                    // if filter matches name
                    if matched, _ := regexp.MatchString(test_name_filter, test_name); matched {
                        vset(nil,ifs,ident,"_test_group",test_group)
                        vset(nil,ifs,ident,"_test_name",test_name)
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

            if inbound.TokenCount > 2 {

                doAt := findDelim(inbound.Tokens, C_Do, 1)
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
                            pf("Result Type -> %T expression was -> %v\n", we.text, we.result)
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
                parser.report(inbound.SourceLine, "Insufficient arguments supplied to ASSERT")
                finish(false, ERR_ASSERT)
                break
            }

            // Determine if this is ASSERT ERROR or normal ASSERT
            isAssertError := inbound.TokenCount > 2 && inbound.Tokens[1].tokText == "ERROR"

            var exprTokens []Token
            if isAssertError {
                exprTokens = inbound.Tokens[2:]
            } else {
                exprTokens = inbound.Tokens[1:]
            }

            // Evaluate once
            oldEnforceError:=enforceError
            enforceError=false
            we := parser.wrappedEval(ifs, ident, ifs, ident, exprTokens)
            enforceError=oldEnforceError

            // Non-test mode: exit early with lightweight checks
            if !under_test {
                if isAssertError {
                    if !we.evalError {
                        parser.report(inbound.SourceLine, "ASSERT ERROR: expression did not throw an error")
                        finish(false, ERR_ASSERT)
                    }
                    // Passed: errored as expected
                    break
                }

                // Normal ASSERT
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
                    parser.report(inbound.SourceLine, "Could not assert! (assertion failed)")
                    finish(false, ERR_ASSERT)
                }
                break
            }

            // Under test: use full test reporting
            if isAssertError {
                if we.evalError {
                    handleTestResult(ifs, true, inbound.SourceLine, "ASSERT ERROR", "expression threw an error as expected")
                } else {
                    handleTestResult(ifs, false, inbound.SourceLine, "ASSERT ERROR", "expression did not throw an error")
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
                handleTestResult(ifs, false, inbound.SourceLine, cet.text, sf("Could not assert! (%s)", we.text))
            } else {
                handleTestResult(ifs, true, inbound.SourceLine, cet.text, cet.text)
            }


        case C_Help:
            hargs := ""
            if inbound.TokenCount == 2 {
                hargs = inbound.Tokens[1].tokText
            }
            ihelp(currentModule,hargs)

        case C_Nop:
            // time.Sleep(1 * time.Microsecond)

        case C_Async:

            // ASYNC IDENTIFIER (namespace :: ) IDENTIFIER LPAREN [EXPRESSION[,...]] RPAREN [IDENTIFIER]
            // async handles    (ns :: )        q          (      [e[,...]]          )      [key]

            if inbound.TokenCount<5 {
                usage := "ASYNC [#i1]handle_map function_call([args]) [next_id][#i0]"
                parser.report(inbound.SourceLine,"Invalid arguments in ASYNC\n"+usage)
                finish(false,ERR_SYNTAX)
                break
            }

            handles := inbound.Tokens[1].tokText
                
            // namespace check
            skip:=int16(0)
            found_namespace:=parser.namespace
            if inbound.Tokens[3].tokType==SYM_DoubleColon {
                found_namespace=inbound.Tokens[2].tokText
                skip=2
            }

            call := found_namespace+"::"+inbound.Tokens[2+skip].tokText

            if inbound.Tokens[3+skip].tokType!=LParen {
                parser.report(inbound.SourceLine,"could not find '(' in ASYNC function call.")
                finish(false,ERR_SYNTAX)
            }

            // get arguments

            var rightParenLoc int16
            for ap:=inbound.TokenCount-1; ap>3+skip; ap-=1 {
                if inbound.Tokens[ap].tokType==RParen {
                    rightParenLoc=ap
                    break
                }
            }

            if rightParenLoc<4 {
               parser.report(inbound.SourceLine,"could not find a valid ')' in ASYNC function call.")
                finish(false,ERR_SYNTAX)
            }

            resu,errs:=parser.evalCommaArray(ifs, inbound.Tokens[4+skip:rightParenLoc])

            // find the optional key argument, for stipulating the key name to be used in handles
            var nival any
            if rightParenLoc!=inbound.TokenCount-1 {
                var err error
                nival,err = parser.Eval(ifs,inbound.Tokens[rightParenLoc+1:])
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
                globlock.Lock()
                if handles=="nil" {
                    _,_=task(ifs,lmv,true,call,resu...)
                } else {
                    h,id:=task(ifs,lmv,false,call,resu...)
                    // assign channel h to handles map
                    if nival==nil {
                        // fmt.Printf("about to vsetElement() in ASYNC (no key name) : nival:%#v h:%#v\n",nival,h)
                        vsetElement(nil,ifs,ident,handles,sf("async_%v",id),h)
                    } else {
                        // fmt.Printf("about to vsetElement() in ASYNC : nival:%#v h:%#v\n",nival,h)
                        vsetElement(nil,ifs,ident,handles,sf("%v",nival),h)
                    }
                }
                globlock.Unlock()

            } else {
                // func not found
                parser.report(inbound.SourceLine,sf("invalid function '%s' in ASYNC call",call))
                finish(false,ERR_EVAL)
            }


            case C_Require: // @note: this keyword may be remove

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
                        resu[1]=interpolate(currentModule,ifs,ident,resu[1].(string))
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
                definitionName = parser.namespace+"::"+inbound.Tokens[1].tokText

                parent:=""
                if structMode {
                    parent=structName
                    definitionName+="~"+parent
                }

                // pf("[#4]Now defining %s[#-]\n",definitionName)

                loc, _ := GetNextFnSpace(true,definitionName,call_s{prepared:false})
                var dargs []string
                var hasDefault []bool
                var defaults []any

                if inbound.TokenCount > 2 {
                    // process tokens directly (no string splitting!)
                    tokens := inbound.Tokens[2:]

                    // remove outer parens if present
                    if tokens[0].tokType == LParen && tokens[len(tokens)-1].tokType == RParen {
                        tokens = tokens[1 : len(tokens)-1]
                    }

                    var currentArgTokens []Token
                    for _, tok := range tokens {
                        if tok.tokType == O_Comma {
                            parser.processArgumentTokens(currentArgTokens, &dargs, &hasDefault, &defaults, loc,ifs,ident)
                            currentArgTokens = nil
                        } else {
                            currentArgTokens = append(currentArgTokens, tok)
                        }
                    }
                    // process the final argument
                    if len(currentArgTokens) > 0 {
                        parser.processArgumentTokens(currentArgTokens, &dargs, &hasDefault, &defaults, loc,ifs,ident)
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
                funcmap[definitionName]=Funcdef{
                    name:definitionName,
                    module:parser.namespace,
                    fs:loc,
                    parent:parent,
                }

                basemodmap[loc]=parser.namespace
                sourceMap[loc]=source_base     // relate defined base 'loc' to parent 'ifs' instance's 'base' source

                fspacelock.Lock()
                functionspaces[loc] = []Phrase{}
                basecode[loc] = []BaseCode{}
                fspacelock.Unlock()

                farglock.Lock()
                functionArgs[loc].args   = dargs
                functionArgs[loc].hasDefault = hasDefault
                functionArgs[loc].defaults = defaults
                farglock.Unlock()

                // pf("defining new function %s (%d)\n",definitionName,loc)

            }

        case C_Showdef:

            if inbound.TokenCount == 2 {

                searchTerm:=inbound.Tokens[1].tokText
                if val,found:=modlist[searchTerm]; found {
                    if val==true {
                        pf("[#5]Module %s : Functions[#-]\n",searchTerm)
                        for _,fun:=range funcmap {
                            if fun.module==searchTerm {
                                ShowDef(fun.name)
                            }
                        }
                    }
                } else {
                    fn := stripOuterQuotes(inbound.Tokens[1].tokText, 2)
                    fn =  interpolate(currentModule,ifs,ident,fn)
                    if _, exists := fnlookup.lmget(fn); exists {
                        ShowDef(fn)
                    } else {
                        parser.report(inbound.SourceLine,"Module/function not found.")
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
                //pf("[bname:%s,toktext:%s,current:%s]",bname,inbound.Tokens[1].tokText,currentModule)
                tco_check:=false // deny tco until we check all is well

                if inbound.Tokens[1].tokType==Identifier && inbound.Tokens[2].tokType==LParen {
                    if strcmp(currentModule+"::"+inbound.Tokens[1].tokText,bname) {
                        rbraceAt := findDelim(inbound.Tokens,RParen, 2)
                        // pf("[rb@%d,tokcount:%d]",rbraceAt,inbound.TokenCount)
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
                        wccount=0
                        depth=0
                        parser.pc=-1
                        goto tco_reentry
                    }
                }
            }

            // evaluate each expr and stuff the results in an array
            var ev_er error
            retvalues=make([]any,curArg)
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
                usage:= "INPUT [#i1]id[#i0] PARAM | OPTARG [#i1]field_position[#i0] [ IS [#i1]error_hint[#i0] ]\n"
                usage+= "INPUT [#i1]id[#i0] ENV [#i1]env_name[#i0]"
                parser.report(inbound.SourceLine,"Incorrect arguments supplied to INPUT.\n"+usage)
                finish(false, ERR_SYNTAX)
                break
            }

            id := inbound.Tokens[1].tokText
            typ := inbound.Tokens[2].tokText
            pos := inbound.Tokens[3].tokText

            bin := bind_int(ifs,id)
            if bin>=uint64(len(*ident)) {
                newident:=make([]Variable,bin+identGrowthSize)
                copy(newident,*ident)
                *ident=newident
            }

            hint:=id
            noteAt:=inbound.TokenCount

            if inbound.TokenCount>5 { // must be something after the IS token too
                noteAt = findDelim(inbound.Tokens, C_Is, 4)
                if noteAt!=-1 {
                    we=parser.wrappedEval(ifs,ident,ifs,ident,inbound.Tokens[noteAt+1:])
                    if !we.evalError {
                        hint=we.result.(string)
                    }
                } else {
                    noteAt=inbound.TokenCount
                }
            }

            // eval

            switch str.ToLower(typ) {
            case "param":

                we = parser.wrappedEval(ifs,ident,ifs,ident,inbound.Tokens[3:noteAt])
                if we.evalError {
                    parser.report(inbound.SourceLine,sf("could not evaluate the INPUT expression\n%+v",we.errVal))
                    finish(true, ERR_EVAL)
                    break
                }
                switch we.result.(type) {
                case int:
                default:
                    parser.report(inbound.SourceLine,"INPUT expression must evaluate to an integer")
                    finish(true,ERR_EVAL)
                    break
                }
                d:=we.result.(int)

                if d<1 {
                    parser.report(inbound.SourceLine, sf("INPUT position %d too low.",d))
                    finish(true, ERR_SYNTAX)
                    break
                }
                if d <= len(cmdargs) {

                    // remove any numSeps from literal, range is a copy of numSeps from lex.go
                    tryN := cmdargs[d-1]
                    for _,ns:=range "_" { tryN=str.Replace(tryN,string(ns),"",-1) }

                    // if this is numeric, assign as an int
                    n, er := strconv.Atoi(tryN)
                    if er == nil {
                        vset(nil,ifs, ident, id, n)
                    } else {
                        vset(nil,ifs, ident, id, cmdargs[d-1])
                    }
                } else {
                    // parser.report(inbound.SourceLine,sf("Expected CLI parameter [%s] not provided at startup.", hint))
                    pf("Expected CLI parameter %s [%s] not provided at startup.\n",id,hint)
                    finish(true, ERR_SYNTAX)
                }

            case "optarg":

                we = parser.wrappedEval(ifs,ident,ifs,ident, inbound.Tokens[3:noteAt])
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

                    // remove any numSeps from literal, range is a copy of numSeps from lex.go
                    tryN := cmdargs[d-1]
                    for _,ns:=range "_" { tryN=str.Replace(tryN,string(ns),"",-1) }

                    // if this is numeric, assign as an int
                    n, er := strconv.Atoi(tryN)
                    if er == nil {
                        vset(nil,ifs, ident, id, n)
                    } else {
                        vset(nil,ifs, ident, id, cmdargs[d-1])
                    }
                } else {
                    if ! (*ident)[bin].declared {
                        // nothing provided but var didn't exist, so create it empty
                        vset(nil,ifs,ident,id,"")
                    }
                    // showIdent(ident)
                }

            case "env":

                vset(nil,ifs,ident,id,os.Getenv(pos))

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
            modGivenAlias:=""
            aliased:=false

            if asAt > 1 {
                // optional AS - if present, set the name of the namespace for this inclusion
                if inbound.TokenCount-asAt!=2 {
                    parser.report(inbound.SourceLine,"MODULE only accepts a single token for AS aliases")
                    finish(false,ERR_MODULE)
                    break
                }
                aliased=true
                modGivenAlias=inbound.Tokens[asAt+1].tokText
            } else {
                asAt=inbound.TokenCount
            }

            if inbound.TokenCount > 1 {
                we = parser.wrappedEval(ifs,ident,ifs,ident, inbound.Tokens[1:asAt])
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

            modGivenPath := we.result.(string)

            if strcmp(modGivenPath,"") {
                parser.report(inbound.SourceLine,"Empty module name provided.")
                finish(false, ERR_MODULE)
                break
            }

            //.. set file location

            var moduleloc string = ""

            if str.IndexByte(modGivenPath, '/') > -1 {
                if filepath.IsAbs(modGivenPath) {
                    moduleloc = modGivenPath
                } else {
                    mdir, _ := gvget("@execpath")
                    moduleloc = mdir.(string)+"/"+modGivenPath
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
            f,err:=os.Stat(moduleloc)
            if err != nil {
                parser.report(inbound.SourceLine, sf("Module is not accessible. (path:%v)",moduleloc))
                finish(false, ERR_MODULE)
                break
            }
            if !f.Mode().IsRegular() {
                parser.report(inbound.SourceLine,"Module is not a regular file.")
                finish(false, ERR_MODULE)
                break
            }

            //.. read in file
            mod, err := ioutil.ReadFile(moduleloc)
            if err != nil {
                parser.report(inbound.SourceLine,"Problem reading the module file.")
                finish(false, ERR_MODULE)
                break
            }

            // override module name with alias at this point, if provided
            oldModule:=parser.namespace
            // oldModule:=currentModule
            modRealAlias:=modGivenPath
            if aliased {
                modRealAlias=modGivenAlias
                currentModule=modRealAlias
            } else {
                currentModule=path.Base(modGivenPath)
                currentModule=str.TrimSuffix(currentModule,".mod")
                modRealAlias=currentModule
            }

            // tokenise and parse into a new function space.

            //.. error if it has already been defined
            if fnlookup.lmexists(modRealAlias) && !permit_dupmod {
                parser.report(inbound.SourceLine,"Module file "+modRealAlias+" already processed once.")
                finish(false, ERR_SYNTAX)
                break
            }

            if !fnlookup.lmexists(modRealAlias) {

                loc, _ := GetNextFnSpace(true,modRealAlias,call_s{prepared:false})

                calllock.Lock()

                fspacelock.Lock()
                functionspaces[loc] = []Phrase{}
                basecode[loc] = []BaseCode{}
                fspacelock.Unlock()

                farglock.Lock()
                functionArgs[loc].args = []string{}
                farglock.Unlock()

                modlist[currentModule]=true

                /*
                pf("(module) aliased -> %v\n",aliased)
                pf("(module) alias   -> %s\n",modRealAlias)
                pf("(module) given   -> %s\n",modGivenPath)
                pf("(module) cmod    -> %s\n",currentModule)
                pf("(module) omod    -> %s\n",oldModule)
                */ 

                //.. parse and execute
                basemodmap[loc]=modRealAlias

                if debugMode {
                    start := time.Now()
                    phraseParse(parser.ctx,modRealAlias, string(mod), 0)
                    elapsed := time.Since(start)
                    pf("(timings-module) elapsed in mod translation for '%s' : %v\n",modRealAlias,elapsed)
                } else {
                    phraseParse(parser.ctx,modRealAlias, string(mod), 0)
                }
                modcs := call_s{}
                modcs.base = loc
                modcs.caller = ifs
                modcs.fs = modRealAlias
                calltable[loc] = modcs
                calllock.Unlock()

                fileMap.Store(loc,moduleloc)

                var modident = make([]Variable,identInitialSize)

                if debugMode {
                    start := time.Now()
                    Call(ctx,MODE_NEW, &modident, loc, ciMod, false, nil, "", []string{})
                    elapsed := time.Since(start)
                    pf("(timings-module) elapsed in mod execution for '%s' : %v\n",modRealAlias,elapsed)
                } else {
                    Call(ctx,MODE_NEW, &modident, loc, ciMod, false, nil, "", []string{})
                }

                calllock.Lock()
                calltable[ifs].gcShyness=20
                calltable[ifs].gc=true
                calllock.Unlock()

                currentModule=oldModule
                parser.namespace=oldModule

            }

        case C_Case:

            // need to store the condition and result for the is/contains/has/or clauses
            // endcase location should be calculated in advance for a direct jump to exit

            if wccount==CASE_CAP {
                parser.report(inbound.SourceLine,sf("maximum CASE nesting reached (%d)",CASE_CAP))
                finish(true,ERR_SYNTAX)
                break
            }

            // make comparator True if missing.
            /*
            if inbound.TokenCount==1 {
                inbound.Tokens=append(inbound.Tokens,Token{tokType:Identifier,subtype:subtypeConst,tokVal:true,tokText:"true"})
            }
            */

            // lookahead
            endfound, enddistance, er := lookahead(source_base, parser.pc, 0, 0, C_Endcase, []int64{C_Case}, []int64{C_Endcase})

            if er {
                parser.report(inbound.SourceLine,"Lookahead dedent error!")
                finish(true, ERR_SYNTAX)
                break
            }

            if !endfound {
                parser.report(inbound.SourceLine,"Missing ENDCASE for this CASE. Maybe check for open quotes or braces in block?")
                finish(false, ERR_SYNTAX)
                break
            }

            if inbound.TokenCount>1 {
                we = parser.wrappedEval(ifs,ident,ifs,ident,inbound.Tokens[1:])
                if we.evalError {
                    parser.report(inbound.SourceLine,sf("could not evaluate the CASE condition\n%+v",we.errVal))
                    finish(false, ERR_EVAL)
                    break
                }
            }

            // create storage for CASE details and increase the nesting level

            if inbound.TokenCount==1 {
                we.result=true
            }

            wccount+=1
            wc[wccount] = caseCarton{endLine: parser.pc + enddistance, value: we.result, performed:false, dodefault: true}
            depth+=1
            lastConstruct = append(lastConstruct, C_Case)


        case C_Is, C_Has, C_Contains, C_Or:

            if lastConstruct[len(lastConstruct)-1] != C_Case {
                parser.report(inbound.SourceLine,"Not currently in a CASE block.")
                finish(false,ERR_SYNTAX)
                break
            }

            carton := wc[wccount]

            if carton.performed {
                // already matched and executed a CASE case so jump to ENDCASE
                parser.pc = carton.endLine - 1
                break
            }

            if inbound.TokenCount > 1 { // inbound.TokenCount==1 for C_Or
                we = parser.wrappedEval(ifs,ident,ifs,ident, inbound.Tokens[1:])
                if we.evalError {
                    parser.report(inbound.SourceLine,sf("could not evaluate expression in CASE condition\n%+v",we.errVal))
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
                    if we.result.(bool) {  // HAS truth
                        wc[wccount].performed = true
                        wc[wccount].dodefault = false
                        // pf("case-has (@line %d): true -> %+v == %+v\n",inbound.SourceLine,we.result,carton.value)
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
                    // pf("case-is (@line %d): true -> %+v == %+v\n",inbound.SourceLine,we.result,carton.value)
                    ramble_on = true
                }

            case C_Contains:
                // pf("case-reached-contains\ncarton: %#v\n",carton)
                reg := sparkle(we.result.(string))
                switch carton.value.(type) {
                case string:
                    if matched, _ := regexp.MatchString(reg, carton.value.(string)); matched { // matched CONTAINS regex
                        wc[wccount].performed = true
                        wc[wccount].dodefault = false
                        // pf("case-contains (@line %d): true -> %+v == %+v\n",inbound.SourceLine,we.result,carton.value)
                        ramble_on = true
                    }
                case int:
                    if matched, _ := regexp.MatchString(reg, strconv.Itoa(carton.value.(int))); matched { // matched CONTAINS regex
                        wc[wccount].performed = true
                        wc[wccount].dodefault = false
                        // pf("case-contains (@line %d): true -> %+v == %+v\n",inbound.SourceLine,we.result,carton.value)
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
                isfound, isdistance, _   := lookahead(source_base, parser.pc+1, 0, 0, C_Is, []int64{C_Case}, []int64{C_Endcase})
                orfound, ordistance, _   := lookahead(source_base, parser.pc+1, 0, 0, C_Or, []int64{C_Case}, []int64{C_Endcase})
                cofound, codistance, _   := lookahead(source_base, parser.pc+1, 0, 0, C_Contains, []int64{C_Case}, []int64{C_Endcase})

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
                break
            }

            breakIn = Error
            forceEnd=false
            lastConstruct = lastConstruct[:depth-1]
            depth-=1
            wc[wccount] = caseCarton{}
            wccount-=1

            if break_count>0 {
                break_count-=1
                if break_count>0 {
                    switch lastConstruct[depth-1] {
                    case C_For,C_Foreach,C_While,C_Case:
                        breakIn=lastConstruct[depth-1]
                    }
                }
                // pf("ENDCASE-BREAK: bc %d\n",break_count)
            }

            if wccount < 0 {
                parser.report(inbound.SourceLine,"Cannot reduce CASE stack below zero.")
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

            structName=parser.namespace+"::"+inbound.Tokens[1].tokText
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
            structNode=[]any{}
            structMode=false


        case C_Showstruct:

            // SHOWSTRUCT [filter]

            var filter string

            if inbound.TokenCount>1 {
                cet := crushEvalTokens(inbound.Tokens[1:])
                filter = interpolate(currentModule,ifs,ident,cet.text)
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

            // WITH STRUCT|ENUM name
            if inbound.TokenCount == 3 {
                with_error:=false
                switch inbound.Tokens[1].tokType {
                case C_Struct:
                    if parser.inside_with_struct {
                        parser.report(inbound.SourceLine,"Already inside a WITH STRUCT")
                        finish(false, ERR_SYNTAX)
                        with_error=true
                    } else {
                        parser.inside_with_struct=true
                        parser.with_struct_name=inbound.Tokens[2].tokText
                        // pf("set with struct name to %s\n",parser.with_struct_name)
                    }
                case C_Enum:
                    if parser.inside_with_enum {
                        parser.report(inbound.SourceLine,"Already inside a WITH ENUM")
                        finish(false, ERR_SYNTAX)
                        with_error=true
                    } else {
                        parser.inside_with_enum=true
                        parser.with_enum_name=inbound.Tokens[2].tokText
                        // pf("set with enum name to %s\n",parser.with_enum_name)
                    }
                default:
                    parser.report(inbound.SourceLine,"Unknown WITH type")
                    finish(false, ERR_SYNTAX)
                    with_error=true
                }
                if with_error { break }
                continue
            }


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
            bin:=inbound.Tokens[1].bindpos

            if fname=="" || vname=="" {
                parser.report(inbound.SourceLine,"Bad arguments to provided to WITH.")
                finish(false,ERR_SYNTAX)
                break
            }

            if ! (*ident)[bin].declared {
                parser.report(inbound.SourceLine,sf("Variable '%s' does not exist.",vname))
                finish(false,ERR_EVAL)
                break
            }

            tfile, err:= ioutil.TempFile("","za_with_"+sf("%d",os.Getpid())+"_")
            if err!=nil {
                parser.report(inbound.SourceLine,"WITH could not create a temporary file.")
                finish(true,ERR_SYNTAX)
                break
            }

            content,_:=vget(&inbound.Tokens[1],ifs,ident,vname)

            ioutil.WriteFile(tfile.Name(), []byte(content.(string)), 0600)
            vset(nil,ifs,ident,fname,tfile.Name())
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

            if parser.inside_with_struct {
                parser.inside_with_struct=false
                parser.with_struct_name=""
                continue
            }

            if parser.inside_with_enum {
                parser.inside_with_enum=false
                parser.with_enum_name=""
                continue
            }

            if !inside_with {
                parser.report(inbound.SourceLine,"ENDWITH without a WITH.")
                finish(false,ERR_SYNTAX)
                break
            }

            inside_with=false


        case C_Print:
            parser.console_output(inbound.Tokens[1:],ifs,ident,inbound.SourceLine,interactive,false,false)

        case C_Println:
            parser.console_output(inbound.Tokens[1:],ifs,ident,inbound.SourceLine,interactive,true,false)

        case C_Log:
            parser.console_output(inbound.Tokens[1:],ifs,ident,inbound.SourceLine,false,false,true)


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
                    parser.console_output(inbound.Tokens[nextCommaAt+1:],ifs,ident,inbound.SourceLine,interactive,false,false)
                }

            }


        case C_Prompt:

            if inbound.TokenCount < 2 {
                usage := "PROMPT [#i1]storage_variable prompt_string[#i0] [ [#i1]validator_regex[#i0] ] [ TO [#i1]width[#i0] ] [ IS [#i1]def_string[#i0] ]"
                parser.report(inbound.SourceLine,  "Not enough arguments for PROMPT.\n"+usage)
                finish(false, ERR_SYNTAX)
                break
            }

            // prompt variable assignment:
            if inbound.TokenCount > 1 { // um, should not do this but...
                if inbound.Tokens[1].tokType == O_Assign {
                    we = parser.wrappedEval(ifs,ident,ifs,ident,inbound.Tokens[2:])
                    if we.evalError {
                        parser.report(inbound.SourceLine,sf("could not evaluate expression prompt assignment\n%+v",we.errVal))
                        finish(false, ERR_EVAL)
                        break
                    }
                    switch we.result.(type) {
                    case string:
                        PromptTokens=make([]Token,len(inbound.Tokens)-2)
                        copy(PromptTokens,inbound.Tokens[2:])
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

                            // capture width
                            var w_okay bool
                            var providedWidth int
                            if widthAt:=findDelim(inbound.Tokens,C_To,1); widthAt != -1 {
                                expr:=parser.wrappedEval(ifs,ident,ifs,ident,inbound.Tokens[widthAt+1:widthAt+2])
                                if expr.evalError {
                                    parser.report(inbound.SourceLine, "Bad width value in PROMPT command.")
                                    finish(false, ERR_EVAL)
                                    break
                                } else {
                                    providedWidth,w_okay=GetAsInt(expr.result)
                                    if w_okay {
                                        parser.report(inbound.SourceLine, "Width value is not an integer in PROMPT command.")
                                        finish(false, ERR_EVAL)
                                        break
                                    }
                                }
                            }
                            inWidth:=panes[currentpane].w-2
                            if providedWidth>0 { inWidth=providedWidth }

                            // capture default string
                            defString := ""
                            defAt := findDelim(inbound.Tokens, C_Is, 1)
                            if defAt != -1 {
                                pdefault:=parser.wrappedEval(ifs,ident,ifs,ident,inbound.Tokens[defAt+1:])
                                if pdefault.evalError {
                                    parser.report(inbound.SourceLine, "Bad default string in PROMPT command.")
                                    finish(false, ERR_EVAL)
                                    break
                                } else {
                                    defString=sf("%v",pdefault.result)
                                }
                            }

                            // get prompt
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

                                // get validator (should be at [3:C_Is|EOTokens])
                                vposEnd:=inbound.TokenCount
                                hasValidator:=false
                                if defAt!=-1 {      // has C_Is
                                    vposEnd=defAt
                                }
                                if vposEnd>3 {
                                    hasValidator=true
                                }

                                if hasValidator {
                                    val_ex,val_ex_error := parser.Eval(ifs,inbound.Tokens[3:vposEnd])
                                    if val_ex_error != nil {
                                        parser.report(inbound.SourceLine,"Validator invalid in PROMPT!")
                                        finish(false,ERR_EVAL)
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
                                        intext=sanitise(intext)
                                        validated, _ = regexp.MatchString(validator, intext)
                                    }
                                    if !broken {
                                        vset(&inbound.Tokens[1],ifs, ident,inbound.Tokens[1].tokText, intext)
                                    }
                                } else {
                                    var inp string
                                    inp, _, broken = getInput(processedPrompt, defString, currentpane, row, col, inWidth, []string{}, promptColour, false, false, echoMask.(string))
                                    inp=sanitise(inp)
                                    vset(&inbound.Tokens[1],ifs, ident,inbound.Tokens[1].tokText, inp)
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
                parser.report(inbound.SourceLine,"LOGGING command malformed.")
                finish(false,ERR_SYNTAX)
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

            case "testfile":
                if testMode {
                    if inbound.TokenCount > 2 {
                        we = parser.wrappedEval(ifs,ident,ifs,ident, inbound.Tokens[2:])
                        if we.evalError {
                            parser.report(inbound.SourceLine, sf("could not evaluate filename in LOGGING TESTFILE statement\n%+v",we.errVal))
                            finish(false, ERR_EVAL)
                            break
                        }
                        old_name:=test_output_file
                        test_output_file=we.result.(string)
                        _,err=os.Stat(test_output_file)
                        if err==nil {
                            err=os.Remove(test_output_file)
                        }
                        err=os.Rename(old_name,test_output_file)
                        if err!=nil {
                            parser.report(inbound.SourceLine,sf("Error during test file instantiation:\n%v",err))
                            finish(false,ERR_FILE)
                        }
                    } else {
                        parser.report(inbound.SourceLine, "Invalid test filename provided for LOGGING TESTFILE command.")
                        finish(false, ERR_SYNTAX)
                    }
                } // else do nothing with this command outside of test mode.

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
                elsefound, elsedistance, er = lookahead(source_base, parser.pc, 0, 1, C_Else, []int64{C_If}, []int64{C_Endif})
                endfound, enddistance, er = lookahead(source_base, parser.pc, 0, 0, C_Endif, []int64{C_If}, []int64{C_Endif})
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
                key:=(uint64(source_base) << 32) | uint64(parser.pc)
                debugger.enterDebugger(key, functionspaces[source_base], ident, &mident, &gident)
            default:
                pf("[#fred]Unknown debug command: %s[#-]\n", action)
            }
            continue


        default:

            // local command assignment (child/parent process call)

            if inbound.TokenCount > 1 {
                // ident "=|" or "=<" check
                if statement == Identifier && ( inbound.Tokens[1].tokType == O_AssCommand || inbound.Tokens[1].tokType == O_AssOutCommand ) {
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
                            cmd = interpolate(currentModule,ifs,ident,basecode[source_base][parser.pc].Original[startPos:])
                        } else {
                            cmd = interpolate(currentModule,ifs,ident,bc[2:])
                        }

                        cop:=system(cmd,false)
                        lhs_name := inbound.Tokens[0].tokText
                        switch inbound.Tokens[1].tokType {
                        case O_AssCommand:
                            vset(&inbound.Tokens[0],ifs, ident, lhs_name, cop)
                        case O_AssOutCommand:
                            vset(&inbound.Tokens[0],ifs, ident, lhs_name, cop.out)
                        }
                    }
                    // skip normal eval below
                    break
                }
            }

            // pf("[statement-loop] about to try default eval with %#v\n",inbound.Tokens)
            // try to eval and assign
            if we=parser.wrappedEval(ifs,ident,ifs,ident,inbound.Tokens); we.evalError {
                errmsg:=""
                // pf("[statement-loop] received this error response from wrappedEval(): %#v\n",we)
                if we.errVal!=nil { errmsg=sf("%+v\n",we.errVal) }
                parser.report(inbound.SourceLine,sf("Error in evaluation\n%s",errmsg))
                finish(false,ERR_EVAL)
                break
            }
            // pf("[statement-loop] received this valid response from wrappedEval(): %#v\n",we)

            if interactive && !we.assign && we.result!=nil {
                pf("%+v\n",we.result)
            }

        } // end-statements-case

    } // end-pc-loop

    if structMode && !typeInvalid {
        // incomplete struct definition
        pf("Open STRUCT definition %v\n",structName)
        finish(true,ERR_SYNTAX)
    }

    lastlock.RLock()
    si=sig_int
    lastlock.RUnlock()

    if debugMode && ifs<3 {
        pf("[#fyellow]Debugger active at program end. Entering final pause.[#-]\n")
        key:=(uint64(source_base) << 32) | uint64(parser.pc)
        debugger.enterDebugger(key,functionspaces[source_base], ident, &mident, &gident)
        activeDebugContext=nil
    }


    if !si {

        // populate return variable in the caller with retvals
        calllock.Lock()
        // populate method_result
        if method {
            method_result,_=vget(nil,ifs,ident,"self")
        }
        if retvalues!=nil {
            calltable[ifs].retvals=retvalues
        }
        calltable[ifs].disposable=true
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
        lastfunc[ifs]=fs
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
            recordExclusiveExecutionTime(ctx,chain, time.Since(startTime))
        }
    }

    calllock.Lock()
    // fmt.Printf("Releasing fs %d (%s). Call table :\n%#v\n",ifs,fs,calltable[ifs])
    if calltable[csloc].caller!=0 {
        errorChain=errorChain[:len(errorChain)-1]
        if enableProfiling {
            popCallChain(ctx)
        }
    }
    calllock.Unlock()

    return retval_count,endFunc,method_result,callErr

}


func system(cmds string, display bool) (cop struct{out string; err string; code int; okay bool}) {

    if hasOuter(cmds,'`') { cmds=stripOuter(cmds,'`') }
    cmds = str.Trim(cmds," \t\n")

    var cmdList []string
    lastpos:=0
    var squote, dquote, bquote bool
    var escMode bool
    var e int
    for ; e<len(cmds); e++ {
        if escMode {
            switch cmds[e] {
            case 'n':
                if ! (dquote || squote || bquote) {
                    cmdList=append(cmdList,cmds[lastpos:e-1])
                    lastpos=e+1
                }
            }
        } else {
            switch cmds[e] {
            case '"':
                dquote=!dquote
            case '\'':
                squote=!squote
            case '`':
                bquote=!bquote
            case '\\':
                if !escMode {
                    escMode=true
                    continue
                }
            }
        }
        escMode=false
    }
    cmdList=append(cmdList,cmds[lastpos:e])

    final_out:=""
    for _,cmd:=range cmdList {
        cop = Copper(cmd, false)
        if display {
            pf("%s",cop.out)
        } else {
            final_out+=cop.out+"\n"
        }
        // pf("sys: [%3d] : %s\n",k,cmd)
        // pf("cmdout: %+v\n",cop)
    }

    if ! display {
        cop.out=str.Trim(final_out,"\n")
    }

    return cop
}

/// execute a command in the shell coprocess or parent
/// used when string already interpolated and result is not required
/// currently only used by SYM_BOR statement processing.
func coprocCall(s string) {
    s=str.TrimRight(s,"\n")
    if len(s) > 0 {

        // find index of first pipe, then remove everything upto and including it
        _,cet,_ := str.Cut(s,"|")

        // strip outer quotes
        cet      = str.Trim(cet," \t\n")
        if hasOuter(cet,'`') { cet=stripOuter(cet,'`') }

        cop     := Copper(cet, false)
        if ! cop.okay {
            pf("Error: [%d] in shell command '%s'\n", cop.code, str.TrimLeft(s," \t"))
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
        // pf("COULD NOT FIND NAME IN FNLOOKUP!\n")
        return false
    }

    // pf("(sd) ifn -> %v , max -> %v\n",ifn,len(functionspaces))
    // pf("(sd) basecode ->\n%+v\n",basecode[ifn])
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
                fargs:=str.Join(falist,",")
                strOut = sf("\n[#4][#bold]%s",fn)
                strOut = str.Replace(strOut,"~",sf("(%v) ~ in struct ",fargs),1)
                if str.Index(strOut,"~")==-1 {
                    strOut += sf("(%v)",fargs)
                }
                strOut += "[#boff][#-]\n\t\t "
            }
            pf(sparkle(str.ReplaceAll(sf("%s%s\n", strOut, basecode[ifn][q].Original),"%","%%")))
        }
    }
    return true
}


/// search token list for a given delimiter string
func findDelim(tokens []Token, delim int64, start int16) (pos int16) {
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


func (parser *leparser) splitCommaArray(tokens []Token) (resu [][]Token) {

    evnest:=0
    newstart:=0
    lt:=0

    if lt=len(tokens);lt==0 { return resu }

    for term := range tokens {
        nt:=tokens[term]
        if nt.tokType==LParen { evnest+=1 }
        if nt.tokType==RParen { evnest-=1 }
        if evnest==0 {
            if nt.tokType == O_Comma {
                v := tokens[newstart:term]
                resu=append(resu,v)
                newstart=term+1
            }
            if term==lt-1 {
                v := tokens[newstart:term+1]
                resu=append(resu,v)
                newstart=term+1
                continue
            }
        }
    }
    return resu

}



func (parser *leparser) evalCommaArray(ifs uint32, tokens []Token) (resu []any, errs []error) {

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
func (parser *leparser) console_output(tokens []Token,ifs uint32,ident *[]Variable,sourceLine int16,interactive bool,lf bool,logging bool) {
    plog_out := ""
    if len(tokens) > 0 {
        evnest:=0
        newstart:=0
        for term := range tokens {
            nt:=tokens[term]
            if nt.tokType==LParen || nt.tokType==LeftSBrace  { evnest+=1 }
            if nt.tokType==RParen || nt.tokType==RightSBrace { evnest-=1 }
            if evnest==0 && (term==len(tokens)-1 || nt.tokType == O_Comma) {
                v, e := parser.Eval(ifs,tokens[newstart:term+1])
                if e!=nil {
                    parser.report(sourceLine,sf("Error in PRINT term evaluation: %s",e))
                    finish(false,ERR_EVAL)
                    break
                }
                newstart=term+1
                switch v.(type) { case string: v=interpolate(parser.namespace,ifs,ident,v.(string)) }
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

func joinTokens(tokens []Token) string {
    var sb str.Builder
    for _, t := range tokens {
        sb.WriteString(t.tokText)
    }
    return str.Trim(sb.String(), " \t")
}


func (parser *leparser) processArgumentTokens(tokens []Token, dargs *[]string, hasDefault *[]bool, defaults *[]any, loc uint32,ifs uint32,ident *[]Variable) {
    eqPos := -1
    for i, t := range tokens {
        if t.tokType == O_Assign {
            eqPos = i
            break
        }
    }

    var argName string

    if eqPos != -1 {
        // Default value present
        argName = joinTokens(tokens[0:eqPos])
        defaultExprTokens := tokens[eqPos+1:]

        // Evaluate
        evaluated := parser.wrappedEval(ifs, ident, ifs, ident, defaultExprTokens)
        if evaluated.evalError {
            parser.report(-1, sf("Error evaluating default for argument '%s': %v", argName, evaluated.errVal))
            finish(false, ERR_EVAL)
            return
        }

        *dargs = append(*dargs, argName)
        *hasDefault = append(*hasDefault, true)
        *defaults = append(*defaults, evaluated.result)
    } else {
        // No default
        argName = joinTokens(tokens)
        *dargs = append(*dargs, argName)
        *hasDefault = append(*hasDefault, false)
        *defaults = append(*defaults, nil)
    }

    // Bind argument name to local scope
    bind_int(loc, argName)
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

