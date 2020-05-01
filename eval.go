package main

import (
    "reflect"
    "strconv"
    "net/http"
    "sync"
    str "strings"
)


/*
 * Replacement variable handlers.
 */

// for locking vset/vcreate/vdelete during a variable write
var vlock = &sync.RWMutex{}


// bah, why do variables have to have names!?! surely an offset would be memorable instead!
func VarLookup(fs uint64, name string) (int, bool) {

    // if lockSafety { vclock.RLock() }
    vc:=varcount[fs]-1
    // if lockSafety { vclock.RUnlock() }

    // more recent variables created should, on average, be higher numbered.
    if lockSafety { vlock.RLock() }
    for k := vc; k>=0 ; k-- {
        if ident[fs][k].iName == name {
            if lockSafety { vlock.RUnlock() }
            return k, true
        }
    }
    if lockSafety { vlock.RUnlock() }
    return 0, false
}


func vcreatetable(fs uint64, capacity int) {

    if lockSafety { vlock.Lock() }
    ident[fs] = make([]Variable, capacity)
    if lockSafety { vlock.Unlock() }

    // if lockSafety { vclock.Lock() }
    varcount[fs] = 0
    // if lockSafety { vclock.Unlock() }

}

func vunset(fs uint64, name string) {

    loc, found := VarLookup(fs, name)

    if lockSafety {
        vclock.Lock()
        vlock.Lock()
    }

    vc:=varcount[fs]
    if found {
        for pos := loc; pos < vc-1; pos++ {
            ident[fs][pos] = ident[fs][pos+1]
        }
        ident[fs][vc] = Variable{}
        varcount[fs]--
    }

    if lockSafety {
        vlock.Unlock()
        vclock.Unlock()
    }
}

func vset(fs uint64, name string, value interface{}) bool {

    if vi, ok := VarLookup(fs, name); ok {
        // set variable - needs to stay fast
        if lockSafety { vlock.Lock() }
        ident[fs][vi].iValue = value
        if lockSafety { vlock.Unlock() }
    } else {
        // instantiate variable - can be slower
        if lockSafety { vclock.Lock() }
        vcfs:=varcount[fs]

        if lockSafety { vlock.Lock() }
        if cap(ident[fs]) == vcfs {
            ident[fs] = append(ident[fs], Variable{iName: name, iValue: value})
        } else {
            ident[fs][vcfs] = Variable{iName: name, iValue: value}
        }
        if lockSafety { vlock.Unlock() }

        varcount[fs]++
        if lockSafety { vclock.Unlock() }

    }
    return true

}

func vgetElement(fs uint64, name string, el string) (interface{}, bool) {
    var v interface{}
    if _, ok := VarLookup(fs, name); ok {
        v, ok = vget(fs, name)
        switch v.(type) {
        case map[string]interface{}:
            // pf("*debug* vgetElement: ifs %v name %v v %v el %v\n",fs,name,v,el)
            // pf(" content : |%v|\n",v.(map[string]interface{})[el])
            return v.(map[string]interface{})[el], ok
        case http.Header:
            return v.(http.Header)[el], ok
        case map[string]int:
            return v.(map[string]int)[el], ok
        case map[string]float64:
            return v.(map[string]float64)[el], ok
        case map[string][]string:
            return v.(map[string][]string)[el], ok
        case map[string]string:
            return v.(map[string]string)[el], ok
        case map[string]bool:
            return v.(map[string]bool)[el], ok
        case []int:
            iel,_:=GetAsInt(el)
            return v.([]int)[iel],ok
        case []bool:
            iel,_:=GetAsInt(el)
            return v.([]bool)[iel],ok
        case []float64:
            iel,_:=GetAsInt(el)
            return v.([]float64)[iel],ok
        case []string:
            iel,_:=GetAsInt(el)
            return v.([]string)[iel],ok
        case string:
            iel,_:=GetAsInt(el)
            return string(v.(string)[iel]),ok
        case []interface{}:
            iel,_:=GetAsInt(el)
            return v.([]interface{})[iel],ok
        default:
            // pf("Unknown type in %v[%v] (%T)\n",name,el,v)
            iel,_:=GetAsInt(el)
            for _,val:=range reflect.ValueOf(v).Interface().([]interface{}) {
                if iel==0  { return val,true }
                iel--
            }
        }
    }
    return nil, false
}

// this could probably be faster. not a great idea duplicating the list like this...
func vsetElement(fs uint64, name string, el string, value interface{}) {

    var list interface{}
    if _, ok := VarLookup(fs, name); ok {
        list, _ = vget(fs, name)
    } else {
        list = make(map[string]interface{}, LIST_SIZE_CAP)
    }

    switch list.(type) {
    case map[string]interface{}:
        list.(map[string]interface{})[el] = value
        vset(fs, name, list)
    }

    numel,er:=strconv.Atoi(el)
    if er==nil {
        newend:=0
        switch list.(type) {

        case []int:
            sz:=cap(list.([]int))
            barrier:=sz/4
            if numel>=sz {
                newend=sz*2
                if numel>newend { newend=numel+barrier }
            }
            if newend!=0 {
                newar:=make([]int,newend,newend)
                copy(newar,list.([]int))
                list=newar
            }
            list.([]int)[numel] = value.(int)

        case []uint8:
            sz:=cap(list.([]uint8))
            barrier:=sz/4
            if numel>=sz {
                newend=sz*2
                if numel>newend { newend=numel+barrier }
            }
            if newend!=0 {
                newar:=make([]uint8,newend,newend)
                copy(newar,list.([]uint8))
                list=newar
            }
            list.([]uint8)[numel] = value.(uint8)

        case []bool:
            sz:=cap(list.([]bool))
            barrier:=sz/4
            if numel>=sz {
                newend=sz*2
                if numel>newend { newend=numel+barrier }
            }
            if newend!=0 {
                newar:=make([]bool,newend,newend)
                copy(newar,list.([]bool))
                list=newar
            }
            list.([]bool)[numel] = value.(bool)

        case []string:
            sz:=cap(list.([]string))
            barrier:=sz/4
            if numel>=sz {
                newend=sz*2
                if numel>newend { newend=numel+barrier }
            }
            if newend!=0 {
                newar:=make([]string,newend,newend)
                copy(newar,list.([]string))
                list=newar
            }
            list.([]string)[numel] = value.(string)

        case []float64:
            sz:=cap(list.([]float64))
            barrier:=sz/4
            if numel>=sz {
                newend=sz*2
                if numel>newend { newend=numel+barrier }
            }
            if newend!=0 {
                newar:=make([]float64,newend,newend)
                copy(newar,list.([]float64))
                list=newar
            }
            list.([]float64)[numel],_ = GetAsFloat(value) // convertToFloat64(value)

        case []interface{}:
            sz:=cap(list.([]interface{}))
            barrier:=sz/4
            if numel>=sz {
                newend=sz*2
                if numel>newend { newend=numel+barrier }
            }
            if newend!=0 {
                newar:=make([]interface{},newend,newend)
                copy(newar,list.([]interface{}))
                list=newar
            }
            list.([]interface{})[numel] = value

        case map[string]int:            // pass straight through to vset
        case map[string]float64:        // pass straight through to vset
        case map[string]bool:           // pass straight through to vset
        case map[string]interface{}:    // pass straight through to vset

        default:
            pf("DEFAULT: Unknown type %T for list %s\n",list,name)

        }
        vset(fs, name, list)
    }
}

func vget(fs uint64, name string) (interface{}, bool) {
    if vi, ok := VarLookup(fs, name); ok {
        return ident[fs][vi].iValue, true

    }
    return nil, false
}


func isBool(expr interface{}) bool {

    typeof := reflect.TypeOf(expr).Kind()
    switch typeof {
    case reflect.Bool:
        return true
    }
    return false
}

func isNumber(expr interface{}) bool {
    typeof := reflect.TypeOf(expr).Kind()
    switch typeof {
    case reflect.Float32, reflect.Float64, reflect.Int, reflect.Int64:
        return true
    }
    return false
}

/// convert variable placeholders in strings to their values
func interpolate(fs uint64, s string) string {

    // should finish sooner if no curly open brace in string.
    if str.IndexByte(s, '{') == -1 {
        return s
    }

    vc:=varcount[fs]

    // we need the extra loops to deal with embedded indirection

    // @note: the direct access to ident[fs][k] means we should
    //  probably have a read lock around this entire loop.
    //  however, that really slows things down and interpolation has
    //  not caused any race or wrong index conditions to exhibit 
    //  during a lot of load testing. 
    //  only add the lock if we really have to! i.e. make sure it is
    //  actually this part of the code and not one of the many other
    //  terrible bits which is the underlying problem.

    for {
        os := s
        for k := 0; k < vc; k++ {
            v := ident[fs][k]
            if v.iValue != nil {
                switch v.iValue.(type) {
                case int:
                    s = str.Replace(s, "{"+v.iName+"}", strconv.FormatInt(int64(v.iValue.(int)), 10), -1)
                case uint8:
                    s = str.Replace(s, "{"+v.iName+"}", strconv.FormatUint(uint64(v.iValue.(uint8)), 10), -1)
                case uint:
                    s = str.Replace(s, "{"+v.iName+"}", strconv.FormatUint(uint64(v.iValue.(uint)), 10), -1)
                case int64:
                    s = str.Replace(s, "{"+v.iName+"}", strconv.FormatInt(v.iValue.(int64), 10), -1)
                case uint64:
                    s = str.Replace(s, "{"+v.iName+"}", strconv.FormatUint(v.iValue.(uint64), 10), -1)
                case float32:
                    s = str.Replace(s, "{"+v.iName+"}", strconv.FormatFloat(v.iValue.(float64), 'f', -1, 32), -1)
                case float64:
                    s = str.Replace(s, "{"+v.iName+"}", strconv.FormatFloat(v.iValue.(float64), 'f', -1, 64), -1)
                case string:
                    s = str.Replace(s, "{"+v.iName+"}", v.iValue.(string), -1)
                default:
                }
            }
        }
        // if nothing was replaced, then it's time to leave this infernal place
        if os == s {
            break
        }
    }
    return s
}

/// find user defined functions in a token stream and evaluate them
func userDefEval(ifs uint64, tokens []Token) ([]Token,bool) {

    var splitPoint = -1
    var callOnly bool
    var lhs Token
    var termsActive bool

    // check for assignment
    for t := range tokens {
        if tokens[t].tokType == C_Assign {
            splitPoint = t
            break
        }
    }

    // searching for equality, in all the wrong places...
    if splitPoint==-1 {
        callOnly=true
    } else {
        lhs = tokens[0]
        // pf("udf: lhs is '%v'\n",lhs)
        callOnly = false
        if !callOnly && splitPoint!=1 {
            if splitPoint == 0 {
                report(ifs,lastline,"Left-hand side is missing.\n")
            }
            if splitPoint == len(tokens)-1 {
                report(ifs,lastline,"Right-hand side is missing.\n")
            }
            finish(false, ERR_SYNTAX)
            return []Token{},true
        }
    }

    // function argument lookup
    var lfa int
    if !callOnly {
        lhsnum, _ := fnlookup.lmget(lhs.tokText)
        farglock.RLock()
        lfa=len(functionArgs[ifs][lhsnum])
        farglock.RUnlock()
    }

    // now work through tokens beyond splitPoint
    // if is ident followed by paramOpen, then look for the paramClose.

    newTermList:=[]Token{}

    for t:=range tokens[splitPoint+1:] {
        if tokens[t].tokType==Identifier {
            // brace next? or leave it be.
            indent:=0
            endOfList:=0 // if still 0 at end, then term list not completed correctly
            for nt:=range tokens[t+1:] {
                if termsActive && str.IndexByte(tokens[nt].tokText,'(') != -1 {
                    indent++
                    termsActive=true
                }
                if termsActive && str.IndexByte(tokens[nt].tokText,')') != -1 {
                    if indent>0 {
                        // still nested
                        indent--
                    } else {
                        // reached end of term list
                        endOfList=nt+1 // close param position, will take tokens up to endOfList-1
                        break
                    }
                }
            }

            if indent>0 && endOfList==0 {
                // something fishy.
                report(ifs,lastline,"unterminated function call?")
                finish(false,ERR_SYNTAX)
                return []Token{},true
            }

            // once close detected, evaluate each term inside params
            // build a new list of terms

            if indent==0 && endOfList!=0 {
                // all should be well, fn found, terms found, properly terminated.
                termList:=tokens[t+1:endOfList]
                expectingComma:=false

                for nt:=range termList {
                    if nt>=lfa {
                        report(ifs,lastline,sf("%s expected %d arguments and received at least %d arguments",lhs.tokText,lfa,nt))
                        finish(false,ERR_SYNTAX)
                        return []Token{},true
                    }
                    // eval each term and ensure comma between each
                    if tokens[nt].tokType!=C_Comma {
                        if expectingComma {
                            // syntax error
                            report(ifs,lastline,"missing comma in parameter list")
                            finish(false,ERR_SYNTAX)
                            return []Token{},true
                        } else {
                            expectingComma=true
                        }
                    } else {
                        if expectingComma {
                            expectingComma=false
                        } else {
                            report(ifs,lastline,"missing a term in parameter list")
                            finish(false,ERR_SYNTAX)
                            return []Token{},true
                        }
                    }
                    // resolve down to list of terms with user functions all evaluated
                    r,e:=userDefEval(ifs,tokens[t+2:t+nt+2])
                    if e {
                        report(ifs,lastline,"deep error in user function evaluation.")
                        finish(false,ERR_SYNTAX)
                        return []Token{},true
                    }
                    newTermList=append(newTermList,r...)
                } // for
            } // if indent
            t=endOfList+1
        } else {
            newTermList=append(newTermList,tokens[t])
        } // if ident
    }

    // figure out what is on the RHS
    //   we need to distinguish za functions (rather than stdlib calls),

    var rhs []Token
    var okay bool = false

    // replace za defined function calls with their results...
    if termsActive {
        rhs, okay = buildRhs(ifs, newTermList)
        if ! okay {
            return []Token{},true
        }
    } else {
        // no sign of a func call, so use original expression
        rhs = tokens[splitPoint+1:]
    }

    // construct a result.

    var combined []Token
    if callOnly {
        combined = append(combined, rhs...)
    } else {
        combined = append(combined, lhs)
        combined = append(combined, Token{tokType: C_Assign, tokText: "="})
        combined = append(combined, rhs...)
    }

    return combined,false

}


// buildRhs does not generate any result. it populates the original expression with
// evaluated results from za functions. the final expression still needs to be evaluated
// by the normal evaluator.


func buildRhs(ifs uint64, rhs []Token) ([]Token, bool) {

    var new_rhs = [31]Token{}
    rhs_tail := 0

    var isfunc bool
    var previous = Token{}
    var argString string
    for _, p := range rhs {

        new_rhs[rhs_tail] = p
        rhs_tail++

        if p.tokType == Expression {
            if previous.tokType == Identifier {
                _, isfunc = fnlookup.lmget(previous.tokText)

                if isfunc {

                    if !hasOuterBraces(p.tokText) {
                        pf("Error: functions must be called with a braced argument set.\n")
                        finish(false, ERR_SYNTAX)
                        return []Token{},false
                    }

                    argString = stripOuter(p.tokText, '(')
                    argString = stripOuter(argString, ')')

                    // evaluate args
                    var iargs []interface{}
                    var argnames []string

                    // populate inbound parameters to the za function call, with evaluated versions of each.
                    if argString != "" {
                        argnames = str.Split(argString, ",")
                        for k, a := range argnames {
                            aval, ef, err := ev(ifs, a, false)
                            if ef || err != nil {
                                pf("Error: problem evaluating '%s' in function call arguments. (fs=%v,err=%v)\n", argnames[k], ifs, err)
                                finish(false, ERR_EVAL)
                                return []Token{},false
                            }
                            iargs = append(iargs, aval)
                        }
                    }

                    // make Za function call

                    newfnlock.Lock()
                    loc := GetNextFnSpace()
                    lmv,_:=fnlookup.lmget(previous.tokText)
                    newfnlock.Unlock()

                    calllock.Lock()
                    callstack[loc] = call_s{fs: previous.tokText, base: lmv, caller: ifs, retvars: []string{"@temp"}}
                    calllock.Unlock()

                    // this Call() should not race. lmv refers to the original source of the function, not the instance. 

                    Call(MODE_CALL, ifs, lmv, MODE_NEW, Phrase{}, loc, iargs...)


                    // @todo: multiple return values...
                    // this would mean we have to populate retvars properly by building the list of l.h.s. variables and passing that instead 
                    // of {"@temp"}. we used to do this, but removed it! :) will have to write it again.
                    // we can pass in a full retvars to buildrhs() from the caller. 
                    // don't need to do this in main.go or actor.go.
                    // we are currently only getting a single lhs value instead of a []lhs list. need to change this too.
                    // fill in missing lhs or rhs variables/results

                    // .. less rhs than lhs:
                    // if len(retvars)<len(iargs) {
                    //     report(ifs,lastline,sf("Syntax error: less RETURN values than expected in call to %v\n",lmv))
                    //     finish(false,ERR_SYNTAX)
                    //     return nil,true
                    // }

                    // .. less lhs than rhs:
                    // for e:=len(iargs); e<=len(retvars); e++ {
                    //     iargs=append(iargs,"@throw")
                    // }

                    // retcount:=0

                    //for kr,vr:=range iargs {
                        // retcount++
                        // if kr=="@throw" {
                        //     pf("Throwing away arg #%v: %v\n",retcount,vr)
                        //     continue
                        // }

                        // handle the returned result
                        if _, ok := VarLookup(ifs, "@temp"); ok {

                            new_tok := Token{}

                            // replace the expression
                            temp,_ := vget(ifs, "@temp")
                            switch temp.(type) {
                            case map[string]interface{}:
                                new_tok.tokVal = temp
                            case string:
                                new_tok.tokVal = temp
                            case float32:
                                new_tok.tokVal = temp
                            case float64:
                                new_tok.tokVal = temp
                            case int64:
                                new_tok.tokVal = temp
                            case uint8:
                                new_tok.tokVal = temp
                            case []bool:
                                new_tok.tokVal = temp
                            case []uint8:
                                new_tok.tokVal = temp
                            case []string:
                                new_tok.tokVal = temp
                            case []float64:
                                new_tok.tokVal = temp
                            case []int:
                                new_tok.tokVal = temp
                            case int:
                                new_tok.tokVal = temp
                            case webstruct:
                                new_tok.tokVal = temp
                            case http.Header:
                                new_tok.tokVal = temp
                            case bool:
                                // true and false are both treated as identifiers.
                                new_tok.tokType = Identifier
                                new_tok.tokVal = temp
                            default:
                                pf("DEFAULT : Did not handle '%+v' in buildRhs().\n",temp)
                            }

                            // replace tail with result, don't add expression to end.
                            rhs_tail--
                            new_rhs[rhs_tail-1] = new_tok

                        } else {
                            rhs_tail--
                        }

                    // } // was end of iargs loop for multi-return values. disabled.
                }
            }
        }

        previous = p

    }

    return new_rhs[:rhs_tail], true

}

var lastreval *[]Token
var lastws string

// evaluate an expression string using the third-party goval lib
func ev(fs uint64, ws string, interpol bool) (result interface{}, ef bool, err error) {

    // before tokens are crushed, search for za functions
    // and execute them, replacing the relevant found terms
    // with the result to reduce the expression.

    // replace interpreted RHS vars with ident[fs] values
    if interpol {
        ws = interpolate(fs, ws)
    }

    // check for potential user-defined functions
    var cl int
    var maybeFunc bool

    // retokenise string, while substituting udf results for udf calls.

    var reval []Token
        var valcount int

    if lastws==ws {
        // reuse previous token string
        reval=*lastreval
    } else {
        // create new token string
        reval = []Token{}
        var t Token
        var eol,eof bool
        lws:=len(ws)

        for p := 0; p < lws; p++ {
            t, eol , eof = nextToken(ws, &cl, p, t.tokType)
            if t.tokPos != -1 {
                p = t.tokPos
            }
            if str.IndexByte(t.tokText, '(') != -1 { maybeFunc=true }
            reval=append(reval,t)
            valcount++
            if eol||eof { break }
        }
        // update cache
        lastws=ws
        lastreval=&reval
    }

    // eval the user defined functions if it looks like there are any

    if maybeFunc {
        // crush to get an ExpressionCarton. .text holds a string version
        r,e:=userDefEval(fs,reval[:valcount])
        if e {
            report(fs,lastline,sf("Could not evaluate the call '%v'\n",reval[:valcount]))
            finish(false,ERR_EVAL)
            return nil,true,nil
        }
        result, ef, err = Evaluate( crushEvalTokens(r).text , fs )
    } else {
        // normal evaluation
        result, ef, err = Evaluate(ws, fs)
    }

    if ef || err != nil {
        lastlock.RLock()
        nv,_:=numlookup.lmget(lastbase)
        if nv!="" {
            pf("Evaluation Error @ Function %v, line %v\n", nv, lastline)
        } else {
            pf("Evaluation Error, line %v\n", lastline)
        }
        lastlock.RUnlock()
        if err!=nil { pf("[#6]%v[#-]\n", err) }
        return nil, ef, err
    }

    return result, ef, nil

}


// single cache line for crusher

// currently disabled until all race conditions have been dealt with.
// This one can be resolved later. It would probably be okay if we locked for the
// full duration of the function, but that would introduce a lot of slow downs.

// var precrushed ExpressionCarton
// var precrushedTokens []Token

// var crushlock deadlock.RWMutex

/// convert a token stream into a single expression struct
func crushEvalTokens(intoks []Token) ExpressionCarton {

    // crushlock.Lock()
    // defer crushlock.Unlock()

    token := intoks[0]

    if token.tokType == EOL || token.tokType == SingleComment {
        return ExpressionCarton{}
    }

/*
    // check for cached repeat
    if len(intoks)==len(precrushedTokens) {
        var eq bool=true
        for i, v := range intoks {
            if v != precrushedTokens[i] { eq=false;break }
        }
        if eq { return precrushed }
    }
*/

    var id str.Builder
    id.Grow(20)
    var crushedOpcodes str.Builder
    crushedOpcodes.Grow(256)

    var assign bool
    tc := len(intoks)

    switch {
    case tc == 1:
        // definitely trying as an expression only
        if token.tokVal==nil {
            crushedOpcodes.WriteString(token.tokText)
        } else {
            crushedOpcodes.WriteString(sf("%#v",token.tokVal))
        }

    case tc == 2:
        // reform arg and try as expression
        for t := range intoks[0:] {
            token := intoks[t]
            if token.tokVal==nil {
                crushedOpcodes.WriteString(token.tokText)
            } else {
                crushedOpcodes.WriteString(sf("%#v",token.tokVal))
            }
        }

    case tc > 2:
        // find assign pos
        var eqPos int
        for e:=1;e<tc;e++ {
            if intoks[e].tokType==C_Assign {
                eqPos=e
            }
        }

        // check for identifier c_equals expression
        if eqPos>0 && intoks[eqPos].tokType == C_Assign {
            assign = true
            for t:=0;t<eqPos; t++ {
                id.WriteString(intoks[t].tokText)
            }
            for t := range intoks[eqPos+1:] {
                token := intoks[eqPos+1+t]
                if token.tokVal==nil {
                    crushedOpcodes.WriteString(token.tokText)
                } else {
                    crushedOpcodes.WriteString(sf("%#v",token.tokVal))
                }
            }
        } else {
            for t := range intoks[0:] {
                token := intoks[t]
                if token.tokVal==nil {
                    crushedOpcodes.WriteString(token.tokText)
                } else {
                    crushedOpcodes.WriteString(sf("%#v",token.tokVal))
                }
            }
        }
    }

//    precrushedTokens=intoks
    precrushed:=ExpressionCarton{text: crushedOpcodes.String(), assign: assign, assignVar: id.String()}

    return precrushed

}

// currently unused?
func tokenise(s string) (toks []Token) {
    tt := Error
    cl := 1
    for p := 0; p < len(s); p++ {
        t, eol, eof := nextToken(s, &cl, p, tt)
        tt = t.tokType
        if t.tokPos != -1 {
            p = t.tokPos
        }
        toks = append(toks, Token{tokType: tt, tokText: t.tokText})
        if eof || eol {
            break
        }
    }
    return toks
}

/// the main call point for actor.go evaluation.
/// this function handles boxing/unboxing around the ev() call
func wrappedEval(fs uint64, expr ExpressionCarton, interpol bool) (result ExpressionCarton, ef bool) {

    v, ef, err := ev(fs, expr.text, interpol)

    if ef || err!=nil {
        expr.evalError=true
        return expr,ef
    }

    expr.result = v

    // @note: this section is allowing commas through on l.h.s. of assignment. 
    // we may want to permit this eventually for multiple assignment.
    // however, it is currently permitting all kinds of dodgy identifier names through.
    // we should have caught them earlier than this, and they are silently succeeding.
    // eg. a,b=release_version()
    // that ^^ works. you can only read the value through interpolation "{a,b}", but it 
    // should really have errored.

    if expr.assign {
        pos := str.IndexByte(expr.assignVar, '[')
        epos := str.IndexByte(expr.assignVar, ']')
        if pos != -1 && epos != -1 {
            // handle array reference
            element, ef, err := ev(fs, expr.assignVar[pos+1:epos], true)
            if ef || err!=nil {
                expr.evalError=true
                return expr, ef
            }
            switch element.(type) {
            case string:
                vsetElement(fs, expr.assignVar[:pos], element.(string), expr.result)
            case int:
                if element.(int)<0 {
                    pf("**debug** negative array element found (%v,%v,%v)\n",fs,expr.assignVar[:pos],element.(int))
                    expr.evalError=true
                    return expr,true
                }
                vsetElement(fs, expr.assignVar[:pos], sf("%v",element.(int)), expr.result)
            }
        } else {
            // non indexed
            vset(fs, interpolate(fs,expr.assignVar), expr.result)
        }
    }

    return expr, ef

}


