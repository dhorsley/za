package main

import (
//     "errors"
    "fmt"
    "reflect"
//    "runtime"
    "strconv"
    "bytes"
    "math"
    "net/http"
//    "os"
    "sync"
    str "strings"
    "unsafe"
)




func (p *leparser) Init() {

    // precedence table
	p.table = [127]rule{
		RParen        : {-1,p.ignore, nil},
		RightSBrace   : {-1,p.ignore, nil},
        LeftSBrace    : {45,p.array_concat,p.binaryLed},// un: [x,y,z], bin: a[b] sub-scripting
        NumericLiteral: {-1,p.number, nil},
        StringLiteral : {-1,p.stringliteral,nil},
        Identifier    : {-1,p.identifier,nil},
        C_Assign      : {5,p.unary,p.binaryLed},
        C_Plus        : {30,p.unary,p.binaryLed},
		C_Minus       : {30,p.unary,p.binaryLed},       // subtraction and unary minus 
		C_Multiply    : {35,nil,p.binaryLed},
		C_Divide      : {35,nil,p.binaryLed},
        C_Percent     : {35,nil,p.binaryLed},
		C_Comma       : {-1,nil, nil},
        LParen        : {100,p.grouping, p.binaryLed},  // a(b), c(d)  calls
		SYM_COLON     : {-1,nil, nil},
        SYM_DOT       : {45,nil,p.binaryLed},           // a.b, c.d    field refs
        EOF           : {-1,nil, nil},
        SYM_EQ        : {25,nil,p.binaryLed},
        SYM_NE        : {25,nil,p.binaryLed},
        SYM_LT        : {25,nil,p.binaryLed},
        SYM_GT        : {25,nil,p.binaryLed},
        SYM_LE        : {25,nil,p.binaryLed},
        SYM_GE        : {25,nil,p.binaryLed},
        SYM_LAND      : {15,nil,p.binaryLed},           // BOOLEAN AND
        SYM_LOR       : {15,nil,p.binaryLed},           // BOOLEAN OR
        SYM_BAND      : {20,nil,p.binaryLed},           // AND
        C_Caret       : {20,nil,p.binaryLed},           // XOR
        SYM_BOR       : {20,nil,p.binaryLed},           // OR
		C_Pling       : {15,p.unary, nil},              // Logical Negation
        SYM_PP        : {45,p.unary, p.binaryLed},      // ++x x++
        SYM_MM        : {45,p.unary, p.binaryLed},      // --x x--
        SYM_PLE       : {5,nil,p.binaryLed},          // a+=b
        SYM_MIE       : {5,nil,p.binaryLed},          // a-=b
        SYM_MUE       : {5,nil,p.binaryLed},          // a*=b
        SYM_DIE       : {5,nil,p.binaryLed},          // a/=b
        SYM_MOE       : {5,nil,p.binaryLed},          // a%=b
        SYM_POW       : {40,nil,p.binaryLed},           // a**b
        SYM_LSHIFT    : {23,nil,p.binaryLed},
        SYM_RSHIFT    : {23,nil,p.binaryLed},
        O_Sqr         : {30,p.unary,nil},
        O_Sqrt        : {30,p.unary,nil},
	}

}

func (p *leparser) reserved(token Token) (interface{}) {

    // this might change in the future:

    panic(fmt.Errorf("statement names cannot be used as identifiers ([%s] %v)",tokNames[token.tokType],token.tokText))
    finish(true,ERR_SYNTAX)

    return token.tokText

}

func (p *leparser) Eval (fs uint64, toks []Token) (ans interface{},err error) {

    // pf("\n[ ev-query -> %+v p.fs -> %d ]\n",toks,p.fs)

    /*
    defer func() {
        if r := recover(); r != nil {
            p.report(sf("\n%v\n",r))
            os.Exit(ERR_EVAL)
        }
    }()
    */

    p.tokens = toks
    p.pos    = 0
    p.fs     = fs

    return p.dparse(0)

}


type leparser struct {
    table       [127]rule   // null+left rules
    tokens      []Token     // the thing getting evaluated
    fs          uint64      // working function space
    pos         int         // distance through parse
    line        int         // shadows lexer source line
    stmtline    int         // shadows program counter (pc)
    prev        Token       // bodge for post-fix operations
    preprev     Token
}



func (p *leparser) next() Token {

    if p.pos == len(p.tokens) {
        return Token{tokType:EOF}
    }

    if p.pos>1 { p.preprev=p.prev }
    if p.pos>0 { p.prev=p.tokens[p.pos-1] }

    p.pos++
    return p.tokens[p.pos-1]
}

func (p *leparser) peek() Token {

     if p.pos == len(p.tokens) {
        return Token{tokType:EOF}
    }

    return p.tokens[p.pos]
}

func (p *leparser) dparse(prec int8) (left interface{},err error) {

    // pf("dparse query tokens  : %#v\n",p.tokens)
    // pf("dparse query fs      : %+v\n",p.fs)
    // pf("dparse query position: %+v\n",p.pos)

	token:=p.next()

        if token.tokType>START_STATEMENTS {
            p.reserved(token)
        }

        left = p.table[token.tokType].nud(token)

        for prec < p.table[p.peek().tokType].prec {
            token = p.next()
            if p.table[token.tokType].led == nil {
                panic(sf("Token '%s' not defined in grammar",token.tokText))
            }
            left = p.table[token.tokType].led(left,token)
        }

    // pf("dparse result: %+v\n",left)
    // pf("dparse error : %#v\n",err)

	return left,err
}


type rule struct {
	prec int8
	nud func(token Token) (interface{})
	led func(left interface{}, token Token) (interface{})
}


func (p *leparser) getRule(token Token) rule {
	return p.table[token.tokType]
}

func (p *leparser) ignore(token Token) interface{} {
    p.next()
    return nil
}

func (p *leparser) binaryLed(left interface{}, token Token) (interface{}) {

    switch token.tokType {
    case LeftSBrace:
        return p.accessArray(left,token)
    case SYM_DOT:
        // return p.accessField(left,token)
        return p.accessFieldOrFunc(p.fs,left,p.next().tokText)
    case LParen:
        return p.callFunction(left,token)
    case SYM_PP:
        return p.postIncDec(token)
    case SYM_MM:
        return p.postIncDec(token)
    }

	right,err := p.dparse(p.table[token.tokType].prec + 1)

    if err!=nil {
        return nil
    }

	switch token.tokType {
	case C_Plus:
        return ev_add(left,right)
	case C_Minus:
		return ev_sub(left,right)
	case C_Multiply:
        return ev_mul(left,right)
	case C_Divide:
		return ev_div(left,right)
	case C_Percent:
		return ev_mod(left,right)
	case SYM_PLE:
        left,_:=vget(p.fs,p.preprev.tokText)
        r:=ev_add(left,right)
        vset(p.fs,p.preprev.tokText,r)
        return r
	case SYM_MIE:
        left,_:=vget(p.fs,p.preprev.tokText)
        r:=ev_sub(left,right)
        vset(p.fs,p.preprev.tokText,r)
        return r
	case SYM_MUE:
        left,_:=vget(p.fs,p.preprev.tokText)
        r:=ev_mul(left,right)
        vset(p.fs,p.preprev.tokText,r)
        return r
	case SYM_DIE:
        left,_:=vget(p.fs,p.preprev.tokText)
        r:=ev_div(left,right)
        vset(p.fs,p.preprev.tokText,r)
        return r
	case SYM_MOE:
        left,_:=vget(p.fs,p.preprev.tokText)
        r:=ev_mod(left,right)
        vset(p.fs,p.preprev.tokText,r)
        return r
	case SYM_EQ:
        return deepEqual(left,right)
	case SYM_NE:
        return !deepEqual(left,right)
	case SYM_LT:
        return compare(left,right,"<")
	case SYM_GT:
        return compare(left,right,">")
	case SYM_LE:
        return compare(left,right,"<=")
	case SYM_GE:
        return compare(left,right,">=")
    case SYM_LOR:
        return asBool(left) || asBool(right)
    case SYM_LAND:
        return asBool(left) && asBool(right)
    case SYM_BAND: // bitwise-and
        return asInteger(left) & asInteger(right)
    case SYM_BOR: // bitwise-or
        return asInteger(left) | asInteger(right)
	case SYM_LSHIFT:
        return ev_shift_left(left,right)
	case SYM_RSHIFT:
        return ev_shift_right(left,right)
	case C_Caret: // XOR
		return asInteger(left) ^ asInteger(right)
    case SYM_POW:
        return ev_pow(left,right)
    case C_Assign:
        panic(fmt.Errorf("assignment unsupported"))
	}
	return left
}


func (p *leparser) accessField(left interface{},right Token) (interface{}) {

    // tok:=p.next()
    return p.accessFieldOrFunc(p.fs,left,p.next().tokText)
}

func (p *leparser) accessArray(left interface{},right Token) (interface{}) {

    var sz,start,end int
    var hasStart,hasEnd,hasRange bool

    switch left:=left.(type) {
    case []bool:
        sz=len(left)
    case []string:
        sz=len(left)
    case []int:
        sz=len(left)
    case []int64:
        sz=len(left)
    case []uint:
        sz=len(left)
    case []uint8:
        sz=len(left)
    case []uint64:
        sz=len(left)
    case []float64:
        sz=len(left)
    case []dirent:
        sz=len(left)
    case []interface{}:
        sz=len(left)
    case string:
        sz=len(left)

    case map[string]interface{},map[string]string:

        // check for key
        var mkey string
        if right.tokType==SYM_DOT {
            t:=p.next()
            mkey=t.tokText
        } else {
            if p.peek().tokType!=RightSBrace {
                dp,err:=p.dparse(0)
                if err!=nil {
                    panic(fmt.Errorf("map key could not be evaluated"))
                }
                switch dp.(type) {
                case string:
                    mkey=dp.(string)
                default:
                    mkey=sf("%v",dp)
                }
            }
            if p.peek().tokType!=RightSBrace {
                panic(fmt.Errorf("end of map key brace missing"))
            }
            // swallow right brace
            p.next()
        }
        return accessArray(p.fs,left,mkey)

        // end map case

    default:
        panic(fmt.Errorf("unknown map or array type '%T'",left))
    }

    end=sz

    if p.peek().tokType!=RightSBrace { // ! == a[] - start+end unchanged

        // check for start of range
        if p.peek().tokType!=SYM_COLON {
            dp,err:=p.dparse(0)
            if err!=nil {
                panic(fmt.Errorf("array range start could not be evaluated"))
            }
            switch dp.(type) {
            case int:
                start=dp.(int)
                hasStart=true
            default:
                panic(fmt.Errorf("start of range must be an integer"))
            }
        }

        // check for end of range
        if p.peek().tokType==SYM_COLON {
            p.next() // swallow colon
            hasRange=true
            if p.peek().tokType!=RightSBrace {
                dp,err:=p.dparse(0)
                if err!=nil {
                    panic(fmt.Errorf("array range end could not be evaluated"))
                }
                switch dp.(type) {
                case int:
                    end=dp.(int)
                    hasEnd=true
                default:
                    panic(fmt.Errorf("end of range must be an integer"))
                }
            }
        }

        if p.peek().tokType!=RightSBrace {
            panic(fmt.Errorf("end of range brace missing"))
        }

        // swallow brace
        p.next()

    }

    if !hasRange && !hasStart && !hasEnd {
        hasRange=true
    }

    switch hasRange {
    case false:
        return accessArray(p.fs,left,start)
    case true:
        return slice(left,start,end)
    }

    return nil

}

func (p *leparser) callFunction(left interface{},right Token) (interface{}) {

    name:=left.(string)

    // @note: this check may not be necessary now, check:

    // filter for functions here
    var isFunc bool
    if _, isFunc = stdlib[name]; !isFunc {
        // check if exists in user defined function space
        _, isFunc = fnlookup.lmget(name)
    }

    if !isFunc {
        panic(fmt.Errorf("'%v' is not a function",name))
    }

    iargs:=[]interface{}{}

    if p.peek().tokType!=RParen {
        for {
            dp,err:=p.dparse(0)
            if err!=nil {
                return nil
            }
            iargs=append(iargs,dp)
            if p.peek().tokType!=C_Comma {
                break
            }
            p.next()
        }
    }

    if p.peek().tokType==RParen {
        p.next() // consume rparen
    }

    return callFunction(p.fs,p.line,name,iargs)

}

func (p *leparser) unary(token Token) (interface{}) {

    switch token.tokType {
    case SYM_PP:
        return p.preIncDec(token)
    case SYM_MM:
        return p.preIncDec(token)
    }

	right,err := p.dparse(38) // between grouping and other ops
    if err!=nil {
        panic(err)
    }

	switch token.tokType {
	case C_Minus:
		return unaryMinus(right)
	case C_Plus:
		return unaryPlus(right)
	case C_Pling:
		return unaryNegate(right)
	case C_Assign:
		panic(fmt.Errorf("assignment unsupported"))
	case O_Sqr:
        return unOpSqr(right)
	case O_Sqrt:
        return unOpSqrt(right)
	}

	return nil
}

func unOpSqr(n interface{}) interface{} {
    switch n:=n.(type) {
    case int:
        return n*n
    case int64:
        return n*n
    case uint:
        return n*n
    case uint64:
        return n*n
    case float64:
        return n*n
    default:
        panic(fmt.Errorf("sqr does not support type '%T'",n))
    }
    return nil
}

func unOpSqrt(n interface{}) interface{} {
    switch n:=n.(type) {
    case int:
        return math.Sqrt(float64(n))
    case int64:
        return math.Sqrt(float64(n))
    case uint:
        return math.Sqrt(float64(n))
    case uint64:
        return math.Sqrt(float64(n))
    case float64:
        return math.Sqrt(n)
    default:
        panic(fmt.Errorf("sqrt does not support type '%T'",n))
    }
    return nil
}

func (p *leparser) array_concat(tok Token) (interface{}) {

	// right-associative

    ary:=[]interface{}{}

    if p.peek().tokType!=RightSBrace {
        for {
            dp,err:=p.dparse(0)
            if err!=nil {
                panic(err)
            }
            ary=append(ary,dp)
            if p.peek().tokType!=C_Comma {
                break
            }
            p.next()
        }
    }

    if p.peek().tokType==RightSBrace {
        p.next() // consume rparen
    }

    return ary

}

//this one needs changing to a binop for pre-
func (p *leparser) preIncDec(token Token) interface{} {

    // get direction
    ampl:=1
    optype:="increment"
    switch token.tokType {
    case SYM_MM:
        ampl=-1
        optype="decrement"
    }

    // move parser to varname 
    vartok:=p.next()

    // exists?
    val,there:=vget(p.fs,vartok.tokText)
    if !there {
        p.report(sf("invalid variable name in post-%s '%s'",optype,vartok.tokText))
        finish(false,ERR_EVAL)
        return nil
    }

    // act according to var type
    var n interface{}
    switch v:=val.(type) {
    case int:
        n=v+ampl
    case int64:
        n=v+int64(ampl)
    case uint:
        n=v+uint(ampl)
    case uint64:
        n=v+uint64(ampl)
    case uint8:
        n=v+uint8(ampl)
    case float64:
        n=v+float64(ampl)
    default:
        p.report(sf("post-%s not supported on type '%T' (%s)",optype,val,val))
        finish(false,ERR_EVAL)
        return nil
    }
    vset(p.fs,vartok.tokText,n)
    return n

}

func (p *leparser) postIncDec(token Token) interface{} {

    // p.postfix=true

    // get direction
    ampl:=1
    optype:="increment"
    switch token.tokType {
    case SYM_MM:
        ampl=-1
        optype="decrement"
    }

    // get var from parser context
    vartok:=p.prev

    // exists?
    val,there:=vget(p.fs,vartok.tokText)
    activeFS:=p.fs
    if !there {
        val,there=vget(globalaccess,vartok.tokText)
        if !there {
            panic(fmt.Errorf("invalid variable name in post-%s '%s'",optype,vartok.tokText))
        }
        activeFS=globalaccess
    }

    // act according to var type
    switch v:=val.(type) {
    case int:
        vset(activeFS,vartok.tokText,v+ampl)
    case int64:
        vset(activeFS,vartok.tokText,v+int64(ampl))
    case uint:
        vset(activeFS,vartok.tokText,v+uint(ampl))
    case uint64:
        vset(activeFS,vartok.tokText,v+uint64(ampl))
    case uint8:
        vset(activeFS,vartok.tokText,v+uint8(ampl))
    case float64:
        vset(activeFS,vartok.tokText,v+float64(ampl))
    default:
        panic(fmt.Errorf("post-%s not supported on type '%T' (%s)",optype,val,val))
        finish(false,ERR_EVAL)
        return nil
    }
    return val
}


func (p *leparser) grouping(tok Token) (interface{}) {

	// right-associative

    val,err:=p.dparse(0)
    if err!=nil {
        panic(err)
    }
    p.next() // consume RParen
    return val

}

func (p *leparser) number(token Token) (interface{}) {
    var num interface{}
    var err error

    if token.tokVal==nil {
        num, err = strconv.ParseInt(token.tokText, 10, 0)
        if err!=nil {
            num, err = strconv.ParseFloat(token.tokText, 0)
        } else {
            num=int(num.(int64))
        }
    } else {
        num=token.tokVal
    }

    if num==nil {
        panic(err)
    }
	return num
}

func (p *leparser) identifier(token Token) (interface{}) {

    // pf("identifier query -> [%+v]\n",token)

    switch token.tokText {
    case "true":
        return true
    case "false":
        return false
    case "nil":
        return nil
    }

    var inter string

    // filter for functions here

    if p.peek().tokType == LParen {
        var isFunc bool
        if _, isFunc = stdlib[token.tokText]; !isFunc {
            // check if exists in user defined function space
            _, isFunc = fnlookup.lmget(token.tokText)
        }

        if isFunc {
            return token.tokText
        }

        panic(fmt.Errorf("function '%v' does not exist",token.tokText))
    }

    inter=interpolate(p.fs,token.tokText)

    // local lookup:
    if val,there:=vget(p.fs,inter); there {
        return val
    }

    // global lookup:
    if val,there:=vget(globalaccess,inter); there {
        return val
    }

    return nil

}

func (p *leparser) stringliteral(token Token) (interface{}) {
    return interpolate(p.fs,stripBacktickQuotes(stripDoubleQuotes(token.tokText)))
}



/*
 * Replacement variable handlers.
 */

// for locking vset/vcreate/vdelete during a variable write
var vlock = &sync.RWMutex{}

// bah, why do variables have to have names!?! surely an offset would be memorable instead!
func VarLookup(fs uint64, name string) (int, bool) {

    if lockSafety { vlock.RLock() }

    // more recent variables created should, on average, be higher numbered.

    k:=varcount[fs]-1

    vl_repeat_point:
        if k<0 {
            if lockSafety { vlock.RUnlock() }
            return 0,false
        }
        if strcmp(ident[fs][k].IName,name) {
            if lockSafety { vlock.RUnlock() }
            return k, true
        }
        k--
    goto vl_repeat_point

}


func vcreatetable(fs uint64, vtable_maxreached * uint64, capacity int) {

    if lockSafety {
        vlock.Lock()
    }

    vtmr:=*vtable_maxreached

    if fs>=vtmr {
        *vtable_maxreached=fs
        ident[fs] = make([]Variable, capacity, capacity)
        varcount[fs] = 0
        // pf("vcreatetable: [for %s] just allocated [%d] cap:%d max:%d\n",name,fs,capacity,*vtable_maxreached)
    } else {
        // pf("vcreatetable: [for %s] skipped allocation for [%d] -> length:%v max:%v\n",name,fs,len(ident),*vtable_maxreached)
    }

    if lockSafety {
        vlock.Unlock()
    }

}

func vunset(fs uint64, name string) {

    ///////////////////////
    return
    ///////////////////////

    // @note: if we intend to use this function then we should
    //  make sure that delete and other funcs update VarLookup
    //  correctly first.

    loc, found := VarLookup(fs, name)

    if lockSafety { vlock.Lock() }

    vc:=varcount[fs]
    if found {
        for pos := loc; pos < vc-1; pos++ {
            ident[fs][pos] = ident[fs][pos+1]
        }
        ident[fs][vc] = Variable{}
        varcount[fs]--
    }

    if lockSafety { vlock.Unlock() }

}


func vdelete(fs uint64, name string, ename string) {

    // no need for lock here as vget already locks and
    // we are working with a copy before vset writes.
    // vset also locks when required.

    if _, ok := VarLookup(fs, name); ok {
        m,_:=vget(fs,name)
        switch m:=m.(type) {
        case map[string][]string:
            delete(m,ename)
            vset(fs,name,m)
        case map[string]string:
            delete(m,ename)
            vset(fs,name,m)
        case map[string]int:
            delete(m,ename)
            vset(fs,name,m)
        case map[string]int64:
            delete(m,ename)
            vset(fs,name,m)
        case map[string]uint8:
            delete(m,ename)
            vset(fs,name,m)
        case map[string]uint64:
            delete(m,ename)
            vset(fs,name,m)
        case map[string]float64:
            delete(m,ename)
            vset(fs,name,m)
        case map[string]bool:
            delete(m,ename)
            vset(fs,name,m)
        case map[string]interface{}:
            delete(m,ename)
            vset(fs,name,m)
        }
    }
}



func vset(fs uint64, name string, value interface{}) (vi int) {

    var ok bool

    if vi, ok = VarLookup(fs, name); ok {

        // set
        if lockSafety { vlock.Lock() }

        // check for conflict with previous VAR
        if ident[fs][vi].ITyped {
            var ok bool
            switch ident[fs][vi].IKind {
            case "bool":
                _,ok=value.(bool)
                if ok { ident[fs][vi].IValue = value.(bool) }
            case "int":
                _,ok=value.(int)
                if ok { ident[fs][vi].IValue = value.(int) }
            case "int64":
                _,ok=value.(int64)
                if ok { ident[fs][vi].IValue = value.(int64) }
            case "uint":
                _,ok=value.(uint64)
                if ok { ident[fs][vi].IValue = value.(uint64) }
            case "float":
                _,ok=value.(float64)
                if ok { ident[fs][vi].IValue = value.(float64) }
            case "string":
                _,ok=value.(string)
                if ok { ident[fs][vi].IValue = value.(string) }
            }
            if !ok {
                if lockSafety { vlock.Unlock() }
                panic(fmt.Errorf("invalid assignation on '%v' [%v] of %v [%T]",name,ident[fs][vi].IKind,value,value))
            }

        } else {
            ident[fs][vi].IValue = value
        }

        if lockSafety { vlock.Unlock() }

    } else {

        // new variable instantiation

        if lockSafety { vlock.Lock() }

        vi=varcount[fs]
        if vi==len(ident[fs]) {

            // append thread safety workaround
            newary:=make([]Variable,len(ident[fs]),len(ident[fs])*2)
            copy(newary,ident[fs])
            newary=append(newary,Variable{IName: name, IValue: value})
            ident[fs]=newary

        } else {
            ident[fs][vi] = Variable{IName: name, IValue: value}
        }

        varcount[fs]++

        if lockSafety { vlock.Unlock() }

    }

    return vi

}


func vgetElement(fs uint64, name string, el string) (interface{}, bool) {
    // pf("vgetE: entered with %v[%v]\n",name,el)
    var v interface{}
    if _, ok := VarLookup(fs, name); ok {
        v, ok = vget(fs, name)
        switch v:=v.(type) {
        case map[string]int:
            return v[el], ok
        case map[string]float64:
            return v[el], ok
        case map[string][]string:
            return v[el], ok
        case map[string]string:
            return v[el], ok
        case map[string]bool:
            return v[el], ok
        case map[string]interface{}:
            return v[el], ok
        case http.Header:
            return v[el], ok
        case []int:
            iel,_:=GetAsInt(el)
            return v[iel],ok
        case []bool:
            iel,_:=GetAsInt(el)
            return v[iel],ok
        case []float64:
            iel,_:=GetAsInt(el)
            return v[iel],ok
        case []string:
            iel,_:=GetAsInt(el)
            return v[iel],ok
        case string:
            iel,_:=GetAsInt(el)
            return string(v[iel]),ok
        case []interface{}:
            iel,_:=GetAsInt(el)
            return v[iel],ok
        default:
            // pf("Unknown type in %v[%v] (%T)\n",name,el,v)
            iel,_:=GetAsInt(el)
            for _,val:=range reflect.ValueOf(v).Interface().([]interface{}) {
                if iel==0  { return val,true }
                iel--
            }
        }
    }
    // pf("vgetE: leaving %v[%v]\n",name,el)
    return nil, false
}

// this could probably be faster. not a great idea duplicating the list like this...

func vsetElement(fs uint64, name string, el interface{}, value interface{}) {

    var list interface{}
    var vi int
    var ok bool

    if vi, ok = VarLookup(fs, name); ok {
        list, _ = vget(fs, name)
    } else {
        list = make(map[string]interface{}, LIST_SIZE_CAP)
        vi=vset(fs,name,list)
    }

    if lockSafety { vlock.Lock() }

    switch list.(type) {
    case map[string]interface{}:

        switch el.(type) {
        case int:
            el=strconv.FormatInt(int64(el.(int)), 10)
        case int64:
            el=strconv.FormatInt(el.(int64), 10)
        case float64:
            el=strconv.FormatFloat(el.(float64), 'f', -1, 64)
        case uint64:
            el=strconv.FormatUint(el.(uint64), 10)
        case uint8:
            el=strconv.FormatUint(uint64(el.(uint8)), 10)
        }

        if ok {
            ident[fs][vi].IValue.(map[string]interface{})[el.(string)]= value
        } else {
            ident[fs][vi].IName= name
            ident[fs][vi].IValue.(map[string]interface{})[el.(string)]= value
            if lockSafety { vlock.Unlock() }
            return
        }
        if lockSafety { vlock.Unlock() }
        return
    }

    numel:=el.(int)
    var fault bool

    switch ident[fs][vi].IValue.(type) {

    case []int:
        sz:=cap(ident[fs][vi].IValue.([]int))
        if numel>=sz {
            newend:=sz*2
            if numel>newend { newend=numel+sz }
            newar:=make([]int,newend,newend)
            copy(newar,ident[fs][vi].IValue.([]int))
            ident[fs][vi].IValue=newar
        }
        ident[fs][vi].IValue.([]int)[numel]=value.(int)

    case []uint8:
        sz:=cap(ident[fs][vi].IValue.([]uint8))
        if numel>=sz {
            newend:=sz*2
            if numel>newend { newend=numel+sz }
            newar:=make([]uint8,newend,newend)
            copy(newar,ident[fs][vi].IValue.([]uint8))
            ident[fs][vi].IValue=newar
        }
        ident[fs][vi].IValue.([]uint8)[numel]=value.(uint8)

    case []bool:
        sz:=cap(ident[fs][vi].IValue.([]bool))
        if numel>=sz {
            newend:=sz*2
            if numel>newend { newend=numel+sz }
            newar:=make([]bool,newend,newend)
            copy(newar,ident[fs][vi].IValue.([]bool))
            ident[fs][vi].IValue=newar
        }
        ident[fs][vi].IValue.([]bool)[numel]=value.(bool)

    case []string:
        sz:=cap(ident[fs][vi].IValue.([]string))
        if numel>=sz {
            newend:=sz*2
            if numel>newend { newend=numel+sz }
            newar:=make([]string,newend,newend)
            copy(newar,ident[fs][vi].IValue.([]string))
            ident[fs][vi].IValue=newar
        }
        ident[fs][vi].IValue.([]string)[numel]=value.(string)

    case []float64:
        sz:=cap(ident[fs][vi].IValue.([]float64))
        if numel>=sz {
            newend:=sz*2
            if numel>newend { newend=numel+sz }
            newar:=make([]float64,newend,newend)
            copy(newar,ident[fs][vi].IValue.([]float64))
            ident[fs][vi].IValue=newar
        }
        ident[fs][vi].IValue.([]float64)[numel],fault=GetAsFloat(value)
        if fault {
            panic(fmt.Errorf("Could not append to float array (ele:%v) a value '%+v' of type '%T'",numel,value,value))
            // finish(false,ERR_EVAL)
        }

    case []interface{}:
        sz:=cap(ident[fs][vi].IValue.([]interface{}))
        if numel>=sz {
            newend:=sz*2
            if numel>newend { newend=numel+sz }
            newar:=make([]interface{},newend,newend)
            copy(newar,ident[fs][vi].IValue.([]interface{}))
            ident[fs][vi].IValue=newar
        }
        ident[fs][vi].IValue.([]interface{})[numel]=value.(interface{})

    default:
        pf("DEFAULT: Unknown type %T for list %s\n",list,name)

    }

    // final write

    if lockSafety { vlock.Unlock() }

}

func vget(fs uint64, name string) (interface{}, bool) {

    if vi, ok := VarLookup(fs, name); ok {

        if lockSafety {
            vlock.RLock()
            defer vlock.RUnlock()
        }
        return ident[fs][vi].IValue , true
    }
    return nil, false

}

func getvtype(fs uint64, name string) (reflect.Type, bool) {
    if vi, ok := VarLookup(fs, name); ok {
        if lockSafety {
            vlock.RLock()
            defer vlock.RUnlock()
        }
        return reflect.TypeOf(ident[fs][vi].IValue) , true
    }
    return nil, false
}

func isBool(expr interface{}) bool {

    // typeof := reflect.TypeOf(expr).Kind()
    switch reflect.TypeOf(expr).Kind() {
    case reflect.Bool:
        return true
    }
    return false
}


func isNumber(expr interface{}) bool {
    typeof := reflect.TypeOf(expr).Kind()
    switch typeof {
    case reflect.Float32, reflect.Float64, reflect.Int, reflect.Int64, reflect.Int32, reflect.Uint8, reflect.Uint32, reflect.Uint64:
        return true
    }
    return false
}


func escape(str string) string {
	var buf bytes.Buffer
	for _, char := range str {
		switch char {
		case '\'', '"', '\\', '\t', '\n', '%':
			buf.WriteRune('\\')
		}
		buf.WriteRune(char)
	}
	return buf.String()
}


var interlock = &sync.RWMutex{}

/// convert variable placeholders in strings to their values
func interpolate(fs uint64, s string) (string) {

    if no_interpolation {
        return s
    }

    // should finish sooner if no curly open brace in string.
    if str.IndexByte(s, '{') == -1 {
        return s
    }

    if lockSafety { interlock.Lock() }

    orig:=s

    // we need the extra loops to deal with embedded indirection

    vc:=varcount[fs]

        // string replacer
        rs := []string{}
        typedlist:=[]int{}
        for k := 0 ; k<vc; k++ {
            if ident[fs][k].IValue!=nil && ident[fs][k].ITyped {
                typedlist=append(typedlist,k)
                if strcmp(ident[fs][k].IKind,"string") {
                    rs = append(rs, "{"+ident[fs][k].IName+"}")
                    rs = append(rs, ident[fs][k].IValue.(string))
                }
                if strcmp(ident[fs][k].IKind,"int")    {
                    rs = append(rs, "{"+ident[fs][k].IName+"}")
                    rs = append(rs, strconv.FormatInt(int64(ident[fs][k].IValue.(int)),10))
                }
                if strcmp(ident[fs][k].IKind,"float")  {
                    rs = append(rs, "{"+ident[fs][k].IName+"}")
                    rs = append(rs, strconv.FormatFloat(ident[fs][k].IValue.(float64),'g',-1,64))
                }
                if strcmp(ident[fs][k].IKind,"bool")  {
                    rs = append(rs, "{"+ident[fs][k].IName+"}")
                    rs = append(rs, strconv.FormatBool(ident[fs][k].IValue.(bool)))
                }
            }
        }
        s = str.NewReplacer(rs...).Replace(s)
        // end replacer

    var skip bool
    var i,k int
    var os string

    for {

        if lockSafety { vlock.RLock() }
        os = s

        for k = 0; k < vc; k++ {

            // already replaced above?
            skip=false
            for _,i=range typedlist {
                if i==k { skip=true; break }
            }
            if skip { continue }

            v := ident[fs][k]

            if v.IValue != nil {

                switch v.IValue.(type) {
                case int:
                    s = str.Replace(s, "{"+v.IName+"}", strconv.FormatInt(int64(v.IValue.(int)), 10),-1)
                case int64:
                    s = str.Replace(s, "{"+v.IName+"}", strconv.FormatInt(v.IValue.(int64), 10),-1)
                case float64:
                    s = str.Replace(s, "{"+v.IName+"}", strconv.FormatFloat(v.IValue.(float64),'g',-1,64),-1)
                case bool:
                    s = str.Replace(s, "{"+v.IName+"}", strconv.FormatBool(v.IValue.(bool)),-1)
                case string:
                    s = str.Replace(s, "{"+v.IName+"}", v.IValue.(string),-1)
                case uint64:
                    s = str.Replace(s, "{"+v.IName+"}", strconv.FormatUint(v.IValue.(uint64), 10),-1)
                case uint:
                    s = str.Replace(s, "{"+v.IName+"}", strconv.FormatUint(v.IValue.(uint64), 10),-1)
                case uint8:
                    s = str.Replace(s, "{"+v.IName+"}", strconv.FormatUint(uint64(v.IValue.(uint8)), 10),-1)
                case []uint8, []uint64, []int64, []float64, []int, []bool, []interface{}, []string:
                    s = str.Replace(s, "{"+v.IName+"}", sf("%v",v.IValue),-1)
                case interface{}:
                    s = str.Replace(s, "{"+v.IName+"}", sf("%v",v.IValue),-1)
                default:
                    s = str.Replace(s, "{"+v.IName+"}", sf("!%T!%v",v.IValue,v.IValue),-1)

                }
            }

        }
        if lockSafety { vlock.RUnlock() }

        if os==s { break }

    }

        // if nothing was replaced, check if evaluation possible, then it's time to leave this infernal place
        var modified bool

        redo:=true
        for ;redo; {
            modified=false
            for p:=0;p<len(s);p++ {
                if s[p]=='{' && s[p+1]=='=' {
                    q:=str.IndexByte(s[p+2:],'}')
                    if q==-1 { break }

                    if aval, err := ev(interparse,fs, s[p+2:p+q+2], false); err==nil {

                        switch val:=aval.(type) {
                        // a few special cases here which will operate faster
                        //  than waiting for fmt.sprintf() to execute.
                        case string:
                            s=s[:p]+val+s[p+q+3:]
                        case int:
                            s=s[:p]+strconv.Itoa(val)+s[p+q+3:]
                        case int64:
                            s=s[:p]+strconv.FormatInt(val,10)+s[p+q+3:]
                        case uint:
                            s=s[:p]+strconv.FormatUint(uint64(val),10)+s[p+q+3:]
                        default:
                            s=s[:p]+sf("%v",val)+s[p+q+3:]

                        }
                        modified=true
                        break
                    }
                    p=q+1
                }
            }
            if !modified { redo=false }
        }

        if s=="<nil>" { s=orig }

    if lockSafety { interlock.Unlock() }

    return s
}


/// find user defined functions in a token stream and evaluate them
func userDefEval(p *leparser,ifs uint64, tokens []Token) ([]Token,bool) {

    var splitPoint int
    var callOnly bool
    var lhs Token
    var termsActive bool

    // return immediately if malformed with = at start
    if tokens[0].tokType == C_Assign {
        return []Token{},true
    }

    // check for assignment
    for t := range tokens {
        if tokens[t].tokType == C_Assign {
            splitPoint = t
            break
        }
    }

    // searching for equality, in all the wrong places...
    if splitPoint==0 {
        callOnly=true
        splitPoint--  // reduce so that all of expr is used in for loop below
    } else {
        lhs = tokens[0]
        callOnly = false
        if !callOnly && splitPoint!=1 {
            if splitPoint == len(tokens)-1 {
                p.report("Right-hand side is missing.")
            }
            finish(false, ERR_SYNTAX)
            return []Token{},true
        }
    }

    // function argument lookup
    var lfa int
    if !callOnly {
        lhsnum, _ := fnlookup.lmget(lhs.tokText)
        lfa=len(functionArgs[ifs][lhsnum])
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
                p.report("unterminated function call?")
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
                        p.report(sf("%s expected %d arguments and received at least %d arguments",lhs.tokText,lfa,nt))
                        finish(false,ERR_SYNTAX)
                        return []Token{},true
                    }
                    // eval each term and ensure comma between each
                    if tokens[nt].tokType!=C_Comma {
                        if expectingComma {
                            // syntax error
                            p.report("missing comma in parameter list")
                            finish(false,ERR_SYNTAX)
                            return []Token{},true
                        } else {
                            expectingComma=true
                        }
                    } else {
                        if expectingComma {
                            expectingComma=false
                        } else {
                            p.report("missing a term in parameter list")
                            finish(false,ERR_SYNTAX)
                            return []Token{},true
                        }
                    }
                    // resolve down to list of terms with user functions all evaluated
                    r,e:=userDefEval(p,ifs,tokens[t+2:t+nt+2])
                    if e {
                        p.report("deep error in user function evaluation.")
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
        rhs, okay = buildRhs(p,ifs, newTermList)
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


func buildRhs(parser *leparser,ifs uint64, rhs []Token) ([]Token, bool) {

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
                            aval, err := ev(parser,ifs, a, false)
                            // pf("brhs - ev : %v -> %v\n",a,aval)
                            if err != nil {
                                pf("Error: problem evaluating '%s' in function call arguments. (fs=%v,err=%v)\n", argnames[k], ifs, err)
                                finish(false, ERR_EVAL)
                                return []Token{},false
                            }
                            iargs = append(iargs, aval)
                        }
                    }

                    // make Za function call

                    // debug(20,"gnfs called from buildRhs()\n")
                    loc,id := GetNextFnSpace(previous.tokText+"@")
                    if lockSafety { calllock.Lock() }
                    lmv,_:=fnlookup.lmget(previous.tokText)

                    calltable[loc] = call_s{fs: id, base: lmv, caller: ifs, callline:parser.line, retvar: "@#"}
                    if lockSafety { calllock.Unlock() }

                    Call(MODE_NEW, loc, ciRhsb, iargs...)

                    // handle the returned result
                    if _, ok := VarLookup(ifs, "@#"); ok {

                        new_tok := Token{}

                        // replace the expression
                        temp,_ := vget(ifs, "@#")
                        switch temp.(type) {
                        case bool:
                            // true and false are both treated as identifiers.
                            new_tok.tokType = Identifier
                        }

                        new_tok.tokVal = temp

                        // replace tail with result, don't add expression to end.
                        rhs_tail--
                        new_rhs[rhs_tail-1] = new_tok

                    } else {
                        rhs_tail--
                    }

                }
            }
        }

        previous = p

    }

    return new_rhs[:rhs_tail], true

}


func fastConv(s string) interface{} {

    if len(s)==0 { return nil }

    isfloat:=false
    isneg:=false

    if len(s)>1 && s[0]=='-' { isneg=true; s=s[1:] }

    // this is not 100% effective, it's just meant to filter
    // out some easy return values.

    for _, v := range s {
        if v=='.' { isfloat=true; continue }
        if v=='e' { continue }
        if v<'0' || v>'9' { break }
    }

    pn,e := strconv.ParseFloat(s,64)

    // @note: not checking if is string here..
    if e==nil {
        if !isfloat {
            if isneg { return int(-pn) }
            return int(pn)
        }
        return pn
    }
    return s
}


// evaluate an expression string using a modified version of the third-party goval lib
func ev(parser *leparser,fs uint64, ws string, interpol bool) (result interface{}, err error) {

    // pf("ev: received: %v\n",ws)

    // replace interpreted RHS vars with ident[fs] values
    // if interpol {
    //     ws = interpolate(fs, ws)
    // }

    // build token list from string 'ws'
    tt := Error
    toks:=make([]Token,0,6)
    cl := 1
    var p int
    for p = 0; p < len(ws);  {
        t, tokPos, _, _ := nextToken(ws, &cl, p, tt)
        tt = t.tokType
        if tokPos != -1 {
            p = tokPos
        }
        toks = append(toks, t)
    }

    // evaluate token list

    // pf("\n\n->> ev calling with '%v'\n : '%+v'\n",ws,toks)
    if len(toks)!=0 {
        result, err = parser.Eval(fs,toks)
    }
    // pf("returned result [%T] '%+v'\n",result,result)

    if result==nil { // could not eval
        if err!=nil {
            parser.report(sf("Error evaluating '%s'",ws))
            finish(false,ERR_EVAL)
        }
    }

    if err!=nil {
        if isNumber(ws) {
            var ierr bool
            result,ierr=GetAsInt(ws)
            if ierr {
                result,_=GetAsFloat(ws)
            }
        }
    }

    return result, err

}


/// convert a token stream into a single expression struct
func crushEvalTokens(intoks []Token) ExpressionCarton {

    // token := intoks[0]

    /* should never happen
    if token.tokType == SingleComment {
        return ExpressionCarton{}
    }
    */

    // var id str.Builder
    // id.Grow(16)
    var crushedOpcodes str.Builder
    crushedOpcodes.Grow(16)

    // var assign bool

        for t:=range intoks {
            crushedOpcodes.WriteString(intoks[t].tokText)
        }

        return ExpressionCarton{text: crushedOpcodes.String(), assign: false, assignVar: ""}

}

/*
    switch {
    case tc == 1:
        // definitely trying as an expression only
        if token.tokVal==nil {
            crushedOpcodes.WriteString(token.tokText)
        } else {
            crushedOpcodes.WriteString(sf("%v",token.tokVal))
        }

    case tc == 2:
        // reform arg and try as expression
        for t := range intoks[0:] {
            token := intoks[t]
            if token.tokVal==nil {
                crushedOpcodes.WriteString(token.tokText)
            } else {
                crushedOpcodes.WriteString(sf("%v",token.tokVal))
            }
        }

    case tc > 2:
        // find assign pos
        var eqPos int
        for e:=1;e<tc;e++ {
            if intoks[e].tokType==C_Assign {
                eqPos=e
                break
            }
        }

        // check for identifier c_equals expression
        // if eqPos>0 && intoks[eqPos].tokType == C_Assign {
        if eqPos>0 {
            assign = true
            for t:=0;t<eqPos; t++ {
                id.WriteString(intoks[t].tokText)
            }
            for t := range intoks[eqPos+1:] {
                token := intoks[eqPos+1+t]
                if token.tokVal==nil {
                    crushedOpcodes.WriteString(token.tokText)
                } else {
                    crushedOpcodes.WriteString(sf("%v",token.tokVal))
                }
            }
        } else {
            for t := range intoks[0:] {
                token := intoks[t]
                if token.tokVal==nil {
                    crushedOpcodes.WriteString(token.tokText)
                } else {
                    crushedOpcodes.WriteString(sf("%v",token.tokVal))
                }
            }
        }
    }

    return ExpressionCarton{text: crushedOpcodes.String(), assign: assign, assignVar: id.String()}

}

*/


// currently unused?
func tokenise(s string) (toks []Token) {
    tt := Error
    cl := 1
    for p := 0; p < len(s); p++ {
        t, tokPos, eol, eof := nextToken(s, &cl, p, tt)
        tt = t.tokType
        if tokPos != -1 {
            p = tokPos
        }
        toks = append(toks, Token{tokType: tt, tokText: t.tokText})
        if eof || eol {
            break
        }
    }
    return toks
}


/// the main call point for actor.go evaluation.
/// this function handles boxing the ev() call

func wrappedEval(p *leparser,fs uint64, tks []Token) (expr ExpressionCarton) {

    // another bodge while testing new evaluator:
    // .Eval not currently returning an assignment flag, so check manually for it
    // will change when assignment moved into the evaluator.

    eqPos:=-1
    for k,_:=range tks {
        if tks[k].tokType==C_Assign {
            expr.assign=true
            eqPos=k
            break
        }
    }

    // end of bodge #7005

    var err error
    expr.result, err = p.Eval(fs,tks[eqPos+1:])
    if err!=nil {
        expr.evalError=true
        expr.errVal=err
        return expr
    }

    if expr.assign {
        p.doAssign(fs,fs,tks,&expr,eqPos)
    }

    return expr

}

func (p *leparser) doAssign(lfs,rfs uint64,tks []Token,expr *ExpressionCarton,eqPos int) {

    // (left)  lfs is the function space to assign to
    // (right) rfs is the function space to evaluate with (calculating indices expressions, etc)

    // pf("doAssign called with tokens: %+v\n",tks)

    var err error

    /*
    determine where last rbrace and last dot are
    if lastdot pos > last rbrace pos and rbrace exists, then mode is i[e].f=
        in this case do the stuff below relative to dot position instead of equpos
        then either assign or get field name and assign
    */

    dotAt:=-1
    rbAt :=-1
    var rbSet, dotSet bool
    for dp:=eqPos-1;dp>0;dp-- {
        if !rbSet  && tks[dp].tokType == RightSBrace    { rbAt=dp  ; rbSet=true }
        if !dotSet && tks[dp].tokType == SYM_DOT        { dotAt=dp ; dotSet=true}
    }

    var dotMode bool
    checkPos:=eqPos

    if dotAt>rbAt && rbAt>0 {
        dotMode=true
        checkPos=dotAt
    }

    // pull out pre eqPos tokens

    switch {
    case eqPos==1:

        ///////////// CHECK FOR a       /////////////////////////////////////////////
        // normal assignment
        vset(lfs, interpolate(rfs,tks[0].tokText), expr.result)
        /////////////////////////////////////////////////////////////////////////////

    case eqPos>3:

        ///////////// CHECK FOR a[e]    /////////////////////////////////////////////
        // check for lbrace and rbrace
        if tks[1].tokType != LeftSBrace || tks[checkPos-1].tokType != RightSBrace {
            expr.errVal=fmt.Errorf("syntax error in assignment")
            expr.evalError=true
            return
        }

        // get the element name expr, eval it. element.(type) is used in switch below.
        element, err := p.Eval(rfs,tks[2:checkPos-1])
        if err!=nil {
            pf("could not evaluate index or key in assignment")
            expr.evalError=true
            expr.errVal=err
            return
        }
        /////////////////////////////////////////////////////////////////////////////


        ///////////// CHECK FOR a[e].f= /////////////////////////////////////////////
        if dotMode {
            lhs_dotField:=""
            if dotAt!=eqPos-2 {
                expr.errVal=fmt.Errorf("Too much information in field name!")
                expr.evalError=true
                return
            }
            lhs_dotField=interpolate(rfs,tks[dotAt+1].tokText)

            // do everything here and leave other cases alone, or it will get real messy

            // have to vget from a[e] into tmp
            //  then check element type like in normal fieldless switch case
            //  then modify the tmp like we do in the eqpos==3 dotted case
            //  and then write it back to storage
            // feels like a really bad idea this...

            // ( reckon i should be finding a memref of the base of the var, then
            //   an offset for the ary element, then similar for field, but that would
            //   mean writing proper variable handling, and proper memory management, and
            //   i ain't going to that length for a prototype :D

            // find stored variable and copy it:

            var tempStore interface{}
            var found bool
            aryName := interpolate(rfs,tks[0].tokText)
            var eleName string
            switch element.(type) {
            case int:
                eleName = strconv.FormatInt(int64(element.(int)), 10)
            case int64:
                eleName = strconv.FormatInt(element.(int64), 10)
            case string:
                eleName = element.(string)
            default:
                eleName = sf("%v",element)
            }

            tempStore ,found = vgetElement(lfs,aryName,eleName)

            if found {

                // get type info about left/right side of assignment
                val:=reflect.ValueOf(tempStore)
                typ:=val.Type()
                intyp:=reflect.ValueOf(expr.result).Type()

                if typ.Kind()==reflect.Struct {

                    // create temp copy of struct
                    tmp:=reflect.New(val.Type()).Elem()
                    tmp.Set(val)

                    if _,exists:=typ.FieldByName(lhs_dotField); exists {

                        // get the required struct field
                        tf:=tmp.FieldByName(lhs_dotField)

                        if intyp.AssignableTo(tf.Type()) {

                            // make r/w then assign the new value into the copied field
                            tf=reflect.NewAt(tf.Type(),unsafe.Pointer(tf.UnsafeAddr())).Elem()
                            tf.Set(reflect.ValueOf(expr.result))

                            ////////////////////////////////////////////////////////////////
                            // write the copy back to the 'real' variable
                            switch element.(type) {
                            case int:
                                vsetElement(lfs,aryName,element.(int),tmp.Interface())
                            case string:
                                vsetElement(lfs,aryName,element.(string),tmp.Interface())
                            default:
                                vsetElement(lfs,aryName,element.(string),tmp.Interface())
                            }
                            return
                            ////////////////////////////////////////////////////////////////

                        } else {
                            expr.errVal=fmt.Errorf("cannot assign result (%T) to %v[%v] (%v)",expr.result,aryName,lhs_dotField,tf.Type())
                            expr.evalError=true
                            return
                        }
                    } else {
                        expr.errVal=fmt.Errorf("STRUCT field %v not found in %v[%v]",lhs_dotField,aryName,eleName)
                        expr.evalError=true
                        return
                    }
                } else {
                    expr.errVal=fmt.Errorf("variable %v[%v] is not a STRUCT (it's a %T)",aryName,eleName,typ.Kind())
                    expr.evalError=true
                    return
                }
            } else {
                expr.errVal=fmt.Errorf("record variable %v[%v] not found",aryName,eleName)
                expr.evalError=true
                return
            }

        }
        /////////////////////////////////////////////////////////////////////////////


        switch element.(type) {
        case string:
            vsetElement(lfs, interpolate(rfs,tks[0].tokText), element.(string), expr.result)
        case int:
            if element.(int)<0 {
                pf("negative element index!! (%s[%v])\n",tks[0].tokText,element)
                expr.evalError=true
                expr.errVal=err
            }
            vsetElement(lfs, interpolate(rfs,tks[0].tokText), element.(int), expr.result)
        default:
            pf("unhandled element type!! [%T]\n",element)
            expr.evalError=true
            expr.errVal=err
        }

    case eqPos==3:
        ///////////// CHECK FOR a.f=    /////////////////////////////////////////////

        // dotted
        if tks[1].tokType == SYM_DOT {

            lhs_v:=interpolate(rfs,tks[0].tokText)
            lhs_f:=interpolate(rfs,tks[2].tokText)

            var ts interface{}
            var found bool

            ts,found=vget(lfs,lhs_v)

            if found {

                val:=reflect.ValueOf(ts)
                typ:=reflect.ValueOf(ts).Type()
                intyp:=reflect.ValueOf(expr.result).Type()

                if typ.Kind()==reflect.Struct {

                    // create temp copy of struct
                    tmp:=reflect.New(val.Type()).Elem()
                    tmp.Set(val)

                    // get the required struct field and make a r/w copy
                    // then assign the new value into the copied field
                    if _,exists:=typ.FieldByName(lhs_f); exists {
                        tf:=tmp.FieldByName(lhs_f)
                        if intyp.AssignableTo(tf.Type()) {
                            tf=reflect.NewAt(tf.Type(),unsafe.Pointer(tf.UnsafeAddr())).Elem()
                            tf.Set(reflect.ValueOf(expr.result))
                            // write the copy back to the 'real' variable
                            vset(lfs,lhs_v,tmp.Interface())
                        } else {
                            pf("cannot assign result (%T) to %v (%v)",expr.result,interpolate(rfs,tks[0].tokText),tf.Type())
                            expr.evalError=true
                            expr.errVal=err
                            // return expr
                        }
                    } else {
                        pf("STRUCT field %v not found in %v",lhs_f,lhs_v)
                        expr.evalError=true
                        expr.errVal=err
                        // return expr
                    }

                } else {
                    pf("variable %v is not a STRUCT",lhs_v)
                    expr.evalError=true
                    expr.errVal=err
                    // return expr
                }

            } else {

                pf("record variable %v not found",lhs_v)
                expr.evalError=true
                expr.errVal=err
                // return expr
            }

        } else {
            pf("assignment looks like it was missing a dot, or you broke it in another way")
            expr.evalError=true
            expr.errVal=err
            // return expr
        }
        /////////////////////////////////////////////////////////////////////////////

    default:
        pf("syntax error in assignment")
        expr.evalError=true
        expr.errVal=err
        // return expr

    }

    // return expr
}


