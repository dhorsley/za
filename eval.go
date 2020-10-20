package main

import (
    "fmt"
    "reflect"
    "strconv"
    "bytes"
    "math"
    "net/http"
    "sync"
    str "strings"
    "unsafe"
    "regexp"
)


func (p *leparser) reserved(token Token) (interface{}) {
    panic(fmt.Errorf("statement names cannot be used as identifiers ([%s] %v)",tokNames[token.tokType],token.tokText))
    finish(true,ERR_SYNTAX)
    return token.tokText
}

func (p *leparser) Eval (fs uint32, toks []Token) (ans interface{},err error) {
    p.tokens = toks
    // pf("tokens -> %+v\n",p.tokens)
    p.pos    = 0
    p.fs     = fs
    return p.dparse(0)
}


type leparser struct {
    tokens      []Token     // the thing getting evaluated
    fs          uint32      // working function space
    pos         int         // distance through parse
    line        int         // shadows lexer source line
    stmtline    int         // shadows program counter (pc)
    prev        Token       // bodge for post-fix operations
    preprev     Token       //   and the same for assignment
}



func (p *leparser) next() Token {

    if p.pos>1 { p.preprev=p.prev }
    if p.pos>0 { p.prev=p.tokens[p.pos-1] }

    if p.pos == len(p.tokens) {
        return Token{tokType:EOF}
    }

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

    // pf("\n\ndparse query     : %+v\n",p.tokens)

	token:=p.next()

        if token.tokType>START_STATEMENTS {
            p.reserved(token)
        }

            // unaries

        switch token.tokType {
		case O_Comma,SYM_COLON,EOF:         // nil rules, prec -1
            left=nil
        case RParen, RightSBrace:           // ignore, prec -1
            left=p.ignore(token)
        case NumericLiteral:                // prec -1
            left=p.number(token)
        case StringLiteral:                 // prec -1
            left=p.stringliteral(token)
        case Identifier:                    // prec -1
            left=p.identifier(token)
        case SYM_Pling, O_Sqr, O_Sqrt,O_Assign, O_Plus, O_Minus:      // prec variable
            left=p.unary(token)
        case O_Multiply, SYM_Caret:         // unary pointery stuff
            left=p.unary(token)
        case LParen:
            left=p.grouping(token)
        case SYM_PP, SYM_MM:
            left=p.unary(token)
        case LeftSBrace:
            left=p.array_concat(token)
        }

            // binaries

        if p.peek().tokType!=EOF {

            for {

                ruleprec:=int8(-1)

                switch p.peek().tokType {
                case O_Assign:
                    ruleprec=5
                case SYM_LAND, SYM_LOR:
                    ruleprec=15
                case SYM_BAND, SYM_BOR, SYM_Caret:
                    ruleprec=20
                case SYM_LSHIFT, SYM_RSHIFT:
                    ruleprec=23
                case SYM_Tilde, SYM_ITilde, SYM_FTilde:
                    ruleprec=25
                case C_In:
                    ruleprec=27
                case SYM_EQ, SYM_NE, SYM_LT, SYM_GT, SYM_LE, SYM_GE:
                    ruleprec=25
                case O_Plus, O_Minus:
                    ruleprec=30
                case O_Divide, O_Percent, O_Multiply:
                    ruleprec=35
                case SYM_POW:
                    ruleprec=40
                case SYM_PP, SYM_MM, LeftSBrace:
                    ruleprec=45
                case SYM_DOT:
                    ruleprec=47
                case LParen:
                    ruleprec=100
                }

                if prec >= ruleprec {
                    break
                }

                token = p.next()
                left = p.binaryLed(ruleprec,left,token)

            }
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



func (p *leparser) ignore(token Token) interface{} {
    p.next()
    return nil
}

func (p *leparser) binaryLed(prec int8, left interface{}, token Token) (interface{}) {

    // pf("current parser state at start of binaryLed is (%#v)\n",p)
    // pf("entered binaryLed with l->%+v and t->%+v\n",left,token)

    switch token.tokType {
    case SYM_PP:
        return p.postIncDec(token)
    case SYM_MM:
        return p.postIncDec(token)
    case LeftSBrace:
        return p.accessArray(left,token)
    case SYM_DOT:
        return p.accessFieldOrFunc(p.fs,left,p.next().tokText)
    case LParen:
        return p.callFunction(left,token)
    }

    // pf("binary: current token list (len:%d) -> (%+v)\n",len(p.tokens),p.tokens)
    // pf("binary sending tokens from position %d in (%+v) to right parse.\n",p.pos,p.tokens[p.pos:])
	right,err := p.dparse(prec + 1)
    // pf("binary returned from right parse with '%+v'\n",right)

    if err!=nil {
        return nil
    }

	switch token.tokType {

	case O_Plus:
        return ev_add(left,right)
	case O_Minus:
		return ev_sub(left,right)
	case O_Multiply:
        return ev_mul(left,right)
	case O_Divide:
		return ev_div(left,right)
	case O_Percent:
		return ev_mod(left,right)

    case O_Assign:
        panic(fmt.Errorf("assignment is not a valid operation in expressions"))

	case SYM_EQ:
        return deepEqual(left,right)
	case SYM_NE:
        // pf("SYM_NE calling deepequal with l:(%#v) and r:(%#v)\n",left,right)
        return !deepEqual(left,right)
	case SYM_LT:
        return compare(left,right,"<")
	case SYM_GT:
        return compare(left,right,">")
	case SYM_LE:
        return compare(left,right,"<=")
	case SYM_GE:
        return compare(left,right,">=")

    case SYM_Tilde:
        return p.rcompare(left,right,false,false)
    case SYM_ITilde:
        return p.rcompare(left,right,true,false)
    case SYM_FTilde:
        return p.rcompare(left,right,false,true)

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
	case SYM_Caret: // XOR
		return asInteger(left) ^ asInteger(right)
    case SYM_POW:
        return ev_pow(left,right)
	case C_In:
		return ev_in(left,right)
	}
	return left
}


var cachelock = &sync.RWMutex{}

func (p *leparser) rcompare (left interface{},right interface{},insensitive bool, multi bool) interface{} {

    switch left.(type) {
    case string:
    default:
        panic(fmt.Errorf("regex comparision requires strings"))
    }

    switch right.(type) {
    case string:
    default:
        panic(fmt.Errorf("regex comparision requires strings"))
    }

    insenStr:=""
    if insensitive { insenStr="(?i)" }

    var re regexp.Regexp
    cachelock.Lock()
    if pre,found:=ifCompileCache[right.(string)];!found {
        re = *regexp.MustCompile(insenStr+right.(string))
        ifCompileCache[right.(string)]=re
        // @note: yes, yes. i know. we aren't releasing 
        //   these. still need to set an ejection policy.
    } else {
        re = pre
    }
    cachelock.Unlock()

    if multi { return re.FindAllString(left.(string),-1) }

	return re.MatchString(left.(string))
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

    case map[string]interface{},map[string]string,map[string]int,map[int]interface{},map[int]int,map[int]string,map[int][]int,map[int][]string,map[int][]interface{}:

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
        panic(fmt.Errorf("unknown map or array type '%T' (val : %#v) with %+v",left,left,right))
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

    // filter for functions here
    var isFunc bool
    if _, isFunc = stdlib[name]; !isFunc {
        // check if exists in user defined function space
        // _, isFunc = fnlookup.lmget(name)
        isFunc = fnlookup.lmexists(name)
    }

    if !isFunc {
        panic(fmt.Errorf("'%v' is not a function",name))
    }

    iargs:=[]interface{}{}

    if p.peek().tokType!=RParen {
        for {
            // pf("cf:nexttok->%#v\n",p.peek())
            dp,err:=p.dparse(0)
            if err!=nil {
                return nil
            }
            iargs=append(iargs,dp)
            if p.peek().tokType!=O_Comma {
                break
            }
            p.next()
        }
    }

    if p.peek().tokType==RParen {
        p.next() // consume rparen
    }

    // pf("callfunc args -> %#v\n",iargs)
    return callFunction(p.fs,p.line,name,iargs)

}


func (p *leparser) unary(token Token) (interface{}) {

    switch token.tokType {
    case SYM_PP:
        return p.preIncDec(token)
    case SYM_MM:
        return p.preIncDec(token)
    case SYM_Caret:
        return p.unAddrOf(token)
    case O_Multiply:
        return p.unDeref(token)
    }

	switch token.tokType {
    case SYM_Pling:
	    right,err := p.dparse(24) // don't bind negate as tightly
        if err!=nil { panic(err) }
		return unaryNegate(right)
    }

	right,err := p.dparse(38) // between grouping and other ops
    if err!=nil { panic(err) }

	switch token.tokType {
	case O_Minus:
		return unaryMinus(right)
	case O_Plus:
		return unaryPlus(right)
	case O_Sqr:
        return unOpSqr(right)
	case O_Sqrt:
        return unOpSqrt(right)
    case O_Assign:
        panic(fmt.Errorf("unary assignment makes no sense"))
	}

	return nil
}

func (p *leparser) unAddrOf(tok Token) interface{} {
    fsnum:=p.fs
    vartok:=p.next()
    // is this a var?
    inter:=vartok.tokText
    if _,there:=vgeti(p.fs,tok.offset); !there {
        if _,there:=vgeti(globalaccess,tok.offset); !there {
            return nil // no var to reference
        }
        fsnum=globalaccess
    }
    // build reference to var
    fs, _ := numlookup.lmget(fsnum)
    if fs=="" { fs="global" }
    // return ref
    return []string{fs,inter}
}

func (p *leparser) unDeref(tok Token) interface{} {

    vartok:=p.next()

    // is this an array?
    var ref interface{}
    var there bool
    inter:=vartok.tokText
    if ref,there=vgeti(p.fs,tok.offset); !there {
        panic(fmt.Errorf("pointer '%v' does not exist",inter))
    }
    switch ref.(type) {
    case []string:
    default:
        panic(fmt.Errorf("invalid reference (type) in '%v'",ref))
        return nil
    }

    // ... with len 2?
    if len(ref.([]string))!=2 {
        panic(fmt.Errorf("invalid reference (length) in '%v'",ref))
        return nil
    }

    // ... with valid fs->fsid? @ ary[0]
    var fsid uint32
    var valid bool
    if ref.([]string)[0]=="nil" && ref.([]string)[1]=="nil" {
        return nil
    }

    if ref.([]string)[0]=="global" {
        fsid=0
    } else {
        fsid,valid=fnlookup.lmget(ref.([]string)[0])
        if !valid {
            panic(fmt.Errorf("invalid space reference in '%v'",ref))
        }
    }

    // ... with active backing variable @ ary[1]
    if val,there:=vget(fsid,ref.([]string)[1]); there {
        return val
    }
    panic(fmt.Errorf("invalid name reference in '%v'",ref))
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
            if p.peek().tokType!=O_Comma {
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

func (p *leparser) preIncDec(token Token) interface{} {

    // get direction
    ampl:=1
    optype:="increment"
    switch token.tokType {
    case SYM_MM:
        ampl=-1
        optype="decrement"
    }

    // move parser position to varname 
    vartok:=p.next()

    // exists?
    val,there:=vgeti(p.fs,vartok.offset)
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
    val,there:=vgeti(p.fs,vartok.offset)
    activeFS:=p.fs
    if !there {
        val,there=vgeti(globalaccess,vartok.offset)
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

func (p *leparser) number(token Token) (num interface{}) {
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

    // pf("-- identifier query -> [%+v]\n",token)

    if strcmp(token.tokText,"true")  { return true }
    if strcmp(token.tokText,"false") { return false }
    if strcmp(token.tokText,"nil")   { return nil }

    // pf("-- post constant checks\n")

    // filter for functions here
    if p.peek().tokType == LParen {
        var isFunc bool
        if _, isFunc = stdlib[token.tokText]; !isFunc {
            // check if exists in user defined function space
            isFunc = fnlookup.lmexists(token.tokText)
        }

        if isFunc {
            return token.tokText
        }

        panic(fmt.Errorf("function '%v' does not exist",token.tokText))
    }

    // local lookup:
    // pf("-- local name for fs %d vi %d is : '%s'\n",p.fs,token.offset, unvmap[p.fs][token.offset])
    if val,there:=vgeti(p.fs,token.offset); there {
        // pf("-- local check in fs %d for %s (%d) - got result %+v\n",p.fs,token.tokText,token.offset,val)
        return val
    }

    // global lookup:
    if vi,there:=VarLookup(globalaccess,token.tokText); there {
        // pf("gc:vl:ga:fs %d - name %s - vi %d\n",globalaccess,token.tokText,vi)
        if v,ok:=vgeti(globalaccess,vi); ok {
            // pf("gc:vl:ga:result %+v\n",v)
            return v
        }
    }

    // pf("gc:vl:ga:nil-end\n")
    return nil

}

func (p *leparser) stringliteral(token Token) (interface{}) {
    // pf("checked a string literal '%v'\n",token.tokText)
    return interpolate(p.fs,stripBacktickQuotes(stripDoubleQuotes(token.tokText)))
}



/*
 * Replacement variable handlers.
 */

// for locking vset/vcreate/vdelete during a variable write
var vlock = &sync.RWMutex{}

// bah, why do variables have to have names!?! surely an offset would be memorable instead!
func VarLookup(fs uint32, name string) (uint16, bool) {

    if vi,found:=vmap[fs][name]; found {
        if vi>functionidents[fs] {
            // pf("vl:read_overflow:%d:%s:%d of %d\n",fs,name,vi,functionidents[fs])
            return 0,false
        }
        // fmt.Printf("vl:found:%d/%s:%d\n",fs,name,vmap[fs][name])
        return vi,true
    }
    // fmt.Printf("vl:notfound:%d/%s\n",fs,name)
    return 0,false

}


func vcreatetable(fs uint32, vtable_maxreached * uint32,sz uint16) {

    if lockSafety {
        vlock.Lock()
    }

    vtmr:=*vtable_maxreached

    if fs>=vtmr {
        *vtable_maxreached=fs
        ident[fs] = make([]Variable, 0, sz)
        // fmt.Printf("vcreatetable: just allocated [fs:%d] cap:%d max_reached:%d\n",fs,sz,*vtable_maxreached)
    } else {
        // fmt.Printf("vcreatetable: skipped allocation for [fs:%d] -> length:%v max_reached:%v\n",fs,len(ident),*vtable_maxreached)
    }

    if lockSafety {
        vlock.Unlock()
    }

}

func vunset(fs uint32, name string) {

    // @note: this is obviously bad as it leaves a hole in the
    //  variables list.
    //  works for now, but needs improvement.

    loc, found := VarLookup(fs, name)

    if lockSafety { vlock.Lock() }
    if found { ident[fs][loc] = Variable{declared:false} }
    if lockSafety { vlock.Unlock() }

}

func vdelete(fs uint32, name string, ename string) {

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

func identResize(fs uint32,sz uint16) {
    newar:=make([]Variable,sz,sz)
    copy(newar,ident[fs])
    ident[fs]=newar
}


func vset(fs uint32, name string, value interface{}) (vi uint16) {

    // create mapping entries for this name if it does not already exist
    if _,found:=vmap[fs][name]; !found {
        vmap[fs][name]=functionidents[fs]
        unvmap[fs][functionidents[fs]]=name
        identResize(fs,functionidents[fs]+1)
        functionidents[fs]++
        // fmt.Printf("-- vset fs %d - name %s - val %+v\n",fs,name,value)
    }

    // ... then forward to vseti
    return vseti(fs, name, vmap[fs][name], value)
}

func vseti(fs uint32, name string, vi uint16, value interface{}) (uint16) {

     // fmt.Printf("** vset %s %+v\n",vi,value)
     // fmt.Printf("  -- len ident fs -> %d ident count of %d\n",len(ident[fs]), functionidents[fs])

    if len(ident[fs])>=int(vi) {
                // && vi < functionidents[fs] {
        // set
        if lockSafety { vlock.Lock() }

        // fmt.Printf("vset:type checking:fs %d/%s (vl:%d):len %d:fi %d\n",fs,unvmap[fs][vi],vi,len(ident[fs]),functionidents[fs])

        if len(ident[fs])<=int(vi) {
            identResize(fs,vi+1)
            functionidents[fs]=vi+1
        }

        // check for conflict with previous VAR
        if ident[fs][vi].ITyped {
            var ok bool
            switch ident[fs][vi].IKind {
            case kbool:
                _,ok=value.(bool)
                if ok { ident[fs][vi].IValue = value.(bool) }
            case kint:
                _,ok=value.(int)
                if ok { ident[fs][vi].IValue = value.(int) }
            case kuint:
                _,ok=value.(uint)
                if ok { ident[fs][vi].IValue = value.(uint) }
            case kfloat:
                _,ok=value.(float64)
                if ok { ident[fs][vi].IValue = value.(float64) }
            case kstring:
                _,ok=value.(string)
                if ok { ident[fs][vi].IValue = value.(string) }
            }
            if !ok {
                if lockSafety { vlock.Unlock() }
                panic(fmt.Errorf("invalid assignation on '%v' of %v [%T]",vi,value,value))
            }

        } else {
            if !ident[fs][vi].declared { // exists, but not in use
                if len(ident[fs])<=int(vi) { identResize(fs,vi+1) ; functionidents[fs]=vi+1 }
                ident[fs][vi]=Variable{IName:name,IValue:value,declared:true}
                vmap[fs][name]=vi
                unvmap[fs][vi]=name
                // fmt.Printf("-- vseti, !declared - fs %d - vi %d - name %s - val %+v\n",fs,vi,name,value)
            } else { // declared so alter
                ident[fs][vi].IValue = value
                // fmt.Printf("vset:assign:existing:vi->%d:fs->%d:val->%+v\nVariable -> %#v\n",vi,fs,value,ident[fs][vi])
            }
        }

        if lockSafety { vlock.Unlock() }

    } else {

        // fmt.Printf("vseti: new var %v\n",name)

        // new variable instantiation
        if lockSafety { vlock.Lock() }

        // vi=functionidents[fs]
        if len(ident[fs])<=int(vi) {
           identResize(fs,vi+1)
        }
        ident[fs][vi]=Variable{IName:name,IValue:value,declared:true}
        // fmt.Printf("vseti:assign:new:vi->%d:fs->%d:val->%+v\nVariable -> %#v\n",vi,fs,value,ident[fs][vi])
        vmap[fs][name]=vi
        unvmap[fs][vi]=name
        functionidents[fs]++

        if lockSafety { vlock.Unlock() }

    }

    return vi

}


func vgetElement(fs uint32, name string, el string) (interface{}, bool) {
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
func vsetElement(fs uint32, name string, el interface{}, value interface{}) {

    var list interface{}
    var vi uint16
    var ok bool

    if vi, ok = VarLookup(fs, name); ok {
        if list, ok = vgeti(fs, vi); ok {
            // pf("vse:gotlist:%s:%#v\n",name,list)
        } else {
            list = make(map[string]interface{}, LIST_SIZE_CAP)
            vi=vset(fs,name,list)
            // pf("vse:undec_newlist:%s\n",name)
        }
    } else {
        list = make(map[string]interface{}, LIST_SIZE_CAP)
        vi=vset(fs,name,list)
        // pf("vse:newlist:%s\n",name)
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
        case uint:
            el=strconv.FormatUint(uint64(el.(uint)), 10)
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

    case []uint:
        sz:=cap(ident[fs][vi].IValue.([]uint))
        if numel>=sz {
            newend:=sz*2
            if numel>newend { newend=numel+sz }
            newar:=make([]uint,newend,newend)
            copy(newar,ident[fs][vi].IValue.([]uint))
            ident[fs][vi].IValue=newar
        }
        ident[fs][vi].IValue.([]uint)[numel]=value.(uint)

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

func vget(fs uint32, name string) (interface{}, bool) {

    if vi, ok := VarLookup(fs, name); ok {

        if lockSafety {
            vlock.RLock()
            defer vlock.RUnlock()
        }
         // pf("-- vget returning value '%v' for %s\n",ident[fs][vi].IValue,name)
        if ident[fs][vi].declared {
            return ident[fs][vi].IValue , true
        }
    }
     // pf("-- vget did not find %s in fs %d\n",name,fs)
    return nil, false

}


func vgeti(fs uint32, vi uint16) (interface{}, bool) {

    if lockSafety {
        vlock.RLock()
        defer vlock.RUnlock()
    }

    if int(vi)>=len(ident[fs]) {
        // pf("-- vgeti returning early.\n");
        return nil,false
    }

    if ident[fs][vi].declared {
        // pf("-- vgeti returning value '%v' for %d\n",ident[fs][vi].IValue,vi)
        return ident[fs][vi].IValue , true
    }
    // pf("-- vgeti did not find %d in fs %d\n",vi,fs)
    return nil, false

}


func getvtype(fs uint32, name string) (reflect.Type, bool) {
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
func interpolate(fs uint32, s string) (string) {

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

    vc:=int(functionidents[fs])

    // string replacer
    rs := []string{}
    typedlist:=[]int{}
    for k := 0 ; k<vc; k++ {
        if ident[fs][k].declared && ident[fs][k].ITyped {
            typedlist=append(typedlist,k)
            if ident[fs][k].IKind==kstring {
                rs = append(rs, "{"+ident[fs][k].IName+"}")
                rs = append(rs, ident[fs][k].IValue.(string))
            }
            if ident[fs][k].IKind==kint    {
                rs = append(rs, "{"+ident[fs][k].IName+"}")
                rs = append(rs, strconv.FormatInt(int64(ident[fs][k].IValue.(int)),10))
            }
            if ident[fs][k].IKind==kuint    {
                rs = append(rs, "{"+ident[fs][k].IName+"}")
                rs = append(rs, strconv.FormatUint(uint64(ident[fs][k].IValue.(uint)),10))
            }
            if ident[fs][k].IKind==kfloat  {
                rs = append(rs, "{"+ident[fs][k].IName+"}")
                rs = append(rs, strconv.FormatFloat(ident[fs][k].IValue.(float64),'g',-1,64))
            }
            if ident[fs][k].IKind==kbool  {
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

                        // pf("( eval interpolation of %s ) ",s[p+2:p+q+2])
                        if aval, err := ev(interparse,fs, s[p+2:p+q+2]); err==nil {

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


// evaluate an expression string
func ev(parser *leparser,fs uint32, ws string) (result interface{}, err error) {

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
        if t.tokType==Identifier {
            loc, _ := VarLookup(fs, t.tokText)
            t.offset=loc
        }
        toks = append(toks, t)
    }

    // evaluate token list
    // pf("ev will send -> %+v\n",toks)
    if len(toks)!=0 {
        result, err = parser.Eval(fs,toks)
    }
    // pf("ev got back  -> %+v\n",result)

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

    var crushedOpcodes str.Builder
    crushedOpcodes.Grow(16)

    for t:=range intoks {
        crushedOpcodes.WriteString(intoks[t].tokText)
    }

    return ExpressionCarton{text: crushedOpcodes.String(), assign: false, assignVar: ""}

}


/// the main call point for actor.go evaluation.
/// this function handles boxing the ev() call

func (p *leparser) wrappedEval(lfs uint32, fs uint32, tks []Token) (expr ExpressionCarton) {

    // search for any assignment operator +=,-=,*=,/=,%=
    // compound the terms beyond the assignment symbol and eval them.

    eqPos:=-1
    var newEval []Token
    var err error

    if len(tks)==2 {
        switch tks[1].tokType {
        case SYM_PP,SYM_MM:

            // @note: naive, just takes the previous token, so cannot
            //  address field or array operations on l.h.s.
            //  okay for now, but could improve. e.g. if p.preprev 
            //  == SYM_DOT || RBrace then work backwards to capture components.
            //  would also depend on length of tks.

            // ++ and -- DO NOT WORK WITH SETGLOB CURRENTLY

            // override p.prev value as postIncDec uses it and we will be throwing 
            //  away the p.* values shortly after this use.
            p.prev=tks[0]
            p.postIncDec(tks[1])
            return expr
        }
    }

    standardAssign:=true

  floop1:
    for k,_:=range tks {
        switch tks[k].tokType {
        // use whichever is encountered first
        case O_Assign:
            eqPos=k
            expr.result, err = p.Eval(fs,tks[k+1:])
            break floop1
        case SYM_PLE:
            expr.result,err=p.Eval(fs,tks[k+1:])
            if err==nil {
                eqPos=k
                newEval=make([]Token,len(tks[:k])+2)
                copy(newEval,tks[:k])
                newEval[k]=Token{tokType:O_Plus}
            }
            standardAssign=false
            break floop1
        case SYM_MIE:
            expr.result,err=p.Eval(fs,tks[k+1:])
            if err==nil {
                eqPos=k
                newEval=make([]Token,len(tks[:k])+2)
                copy(newEval,tks[:k])
                newEval[k]=Token{tokType:O_Minus}
            }
            standardAssign=false
            break floop1
        case SYM_MUE:
            expr.result,err=p.Eval(fs,tks[k+1:])
            if err==nil {
                eqPos=k
                newEval=make([]Token,len(tks[:k])+2)
                copy(newEval,tks[:k])
                newEval[k]=Token{tokType:O_Multiply}
            }
            standardAssign=false
            break floop1
        case SYM_DIE:
            expr.result,err=p.Eval(fs,tks[k+1:])
            if err==nil {
                eqPos=k
                newEval=make([]Token,len(tks[:k])+2)
                copy(newEval,tks[:k])
                newEval[k]=Token{tokType:O_Divide}
            }
            standardAssign=false
            break floop1
        case SYM_MOE:
            expr.result,err=p.Eval(fs,tks[k+1:])
            if err==nil {
                eqPos=k
                newEval=make([]Token,len(tks[:k])+2)
                copy(newEval,tks[:k])
                newEval[k]=Token{tokType:O_Percent}
            }
            standardAssign=false
            break floop1
        }
    }

    if eqPos==-1 {
        expr.result, err = p.Eval(fs,tks)
    } else {
        expr.assign=true
        // before eval, rewrite lhs token offsets to their lhs equivalent
        if !standardAssign {
            if lfs!=fs {
                if newEval[0].tokType==Identifier {
                    if off,found:=vmap[lfs][newEval[0].tokText]; found {
                        vlock.RLock()
                        if ident[lfs][off].declared {
                            newEval[0].offset=off
                        } else {
                            p.report("you may only amend declared variables outside of local scope")
                            expr.evalError=true
                            finish(false,ERR_SYNTAX)
                            vlock.RUnlock()
                            return expr
                        }
                        vlock.RUnlock()
                    } else {
                        p.report("you may only amend existing variables outside of local scope")
                        expr.evalError=true
                        finish(false,ERR_SYNTAX)
                        vlock.RUnlock()
                        return expr
                    }
                }
            }
            switch expr.result.(type) {
            case string:
                newEval[eqPos+1]=Token{tokType:StringLiteral,tokText:expr.result.(string), tokVal:expr.result}
            default:
                newEval[eqPos+1]=Token{tokType:NumericLiteral,tokText:"", tokVal:expr.result}
            }
            // eval
            expr.result, err = p.Eval(lfs,newEval)
        }
    }

    if err!=nil {
        expr.evalError=true
        expr.errVal=err
        return expr
    }

    if expr.assign {
        // pf("-- entering doAssign (lfs->%d,rfs->%d) with tokens : %+v\n",lfs,fs,tks)
        // pf("-- entering doAssign (lfs->%d,rfs->%d) with value  : %+v\n",lfs,fs,expr.result)
        p.doAssign(lfs,fs,tks,&expr,eqPos)
        // pf("-- exited   doAssign (lfs->%d,rfs->%d) with idents of %+v\n",lfs,fs,ident[lfs])
    }

    return expr

}

func getExpressionType(e interface{}) uint8 {
    switch e.(type) {
    case string:
        return StringLiteral
    }
    return NumericLiteral
}

func (p *leparser) doAssign(lfs,rfs uint32,tks []Token,expr *ExpressionCarton,eqPos int) {

    // (left)  lfs is the function space to assign to
    // (right) rfs is the function space to evaluate with (calculating indices expressions, etc)

    // pf("doAssign called with tokens: %#v\n",tks)
    // pf("doAssign inbound assign?   : %+v\n",expr.assign)
    // pf("doAssign inbound results   : %#v\n",expr.result)

    var err error

    // split tks into assignees, splitting on commas

    doMulti:=false
    for tok := range tks[:eqPos] {
        if tks[tok].tokType==O_Comma { doMulti=true; break }
    }

    var largs=make([][]Token,1)

    if doMulti {
        curArg:=0
        evnest:=0
        var scrap [7]Token
        scrapCount:=0
        for tok := range tks[:eqPos] {
            nt:=tks[tok]
            if nt.tokType==LParen || nt.tokType==LeftSBrace  { evnest++ }
            if nt.tokType==RParen || nt.tokType==RightSBrace { evnest-- }
            if nt.tokType!=O_Comma || evnest>0 {
                scrap[scrapCount]=nt
                scrapCount++
            }
            if evnest==0 && (tok==eqPos-1 || nt.tokType == O_Comma) {
                largs[curArg]=append(largs[curArg],scrap[:scrapCount]...)
                scrapCount=0
                curArg++
                if curArg>=len(largs) {
                    largs=append(largs,[]Token{})
                }
            }
        }
        largs=largs[:curArg]
    } else {
        largs[0]=tks[:eqPos]
    }

    var results []interface{}

    if len(largs)==1 {
        results=[]interface{}{expr.result}
    } else {
        // read results
        if expr.result!=nil {
            switch expr.result.(type) {
            case []interface{}:
                results=expr.result.([]interface{})
            case interface{}:
                results=append(results,expr.result.(interface{}))
            default:
                pf("unknown result type [%T] in expr box %#v\n",expr.result,expr.result)
            }
        } else {
            results=[]interface{}{nil}
        }
    }

    // figure number of l.h.s items and compare to results.
    if len(largs)>len(results) && len(results)>1 {
        expr.errVal=fmt.Errorf("not enough values to populate assignment")
        expr.evalError=true
        return
    }

    for assno := range largs {

        // pf("assignee #%d -> %+v\n",assno,largs[assno])
        assignee:=largs[assno]

        // then apply the shite below to each one, using the next available result from results[]

        dotAt:=-1
        rbAt :=-1
        var rbSet, dotSet bool
        for dp:=len(assignee)-1;dp>0;dp-- {
            if !rbSet  && assignee[dp].tokType == RightSBrace    { rbAt=dp  ; rbSet=true }
            if !dotSet && assignee[dp].tokType == SYM_DOT        { dotAt=dp ; dotSet=true}
        }

        var dotMode bool

        if dotAt>rbAt && rbAt>0 {
            dotMode=true
        }

        switch {
        case len(assignee)==1:
            ///////////// CHECK FOR a       /////////////////////////////////////////////
            // normal assignment
            var vi uint16
            var there bool
            if lfs!=rfs {
                if vi,there=VarLookup(lfs,assignee[0].tokText); !there {
                    vi=vset(lfs,assignee[0].tokText,nil)
                }
            } else {
                vi=assignee[0].offset
            }
            // pf("-- normal assignment to (ifs:%d) %s (offset:%d) of %+v [%T]\n", lfs, assignee[0].tokText, vi, results[assno],results[assno])
            vseti(lfs, assignee[0].tokText, vi, results[assno])
            // pf("--  content of fs %d vi %d -> %+v [%T]\n",lfs,vi,ident[lfs][vi].IValue,ident[lfs][vi].IValue)
            /////////////////////////////////////////////////////////////////////////////

        case len(assignee)==2:
            // currently only *p pointer assignment, but check...
            switch assignee[0].tokText {
            case "*":

                // ... check assignee[1] is a local var
                if _,there:=VarLookup(rfs,assignee[1].tokText); !there {
                    expr.errVal=fmt.Errorf("cannot find local pointer in assignment")
                    expr.evalError=true
                    return
                }

                // ... check it is also a pointer
                val,_:=vgeti(rfs,assignee[1].offset)
                switch val.(type) {
                case []string:
                    if len(val.([]string))!=2 {
                        expr.errVal=fmt.Errorf("'%+v' doesn't look like a pointer",val)
                        expr.evalError=true
                        return
                    }
                case nil:
                    var v [2]string
                    v[0]="nil"
                    v[1]="nil"
                    val=v[:]
                default:
                    expr.errVal=fmt.Errorf("'%+v' is not a pointer",val)
                    expr.evalError=true
                    return
                }

                // ... deref target fsid and varname from pointer[0] and pointer[1]
                var fsid uint32
                var valid bool
                if len(val.([]string))==2 && val.([]string)[0]=="nil" && val.([]string)[1]=="nil" {
                } else {
                    if val.([]string)[0]!="global" {
                        fsid,valid=fnlookup.lmget(val.([]string)[0])
                        if !valid {
                            expr.errVal=fmt.Errorf("'%v' is not a valid function space",val.([]string)[0])
                            expr.evalError=true
                            return
                        }
                    }
                }
                // @note: not checking validity of val[1] var name, may change this later.

                // ... vset targets
                vset(fsid,val.([]string)[1],results[assno])

            }

        case len(assignee)>3:

            ///////////// CHECK FOR a[e]    /////////////////////////////////////////////
            // check for lbrace and rbrace
            if assignee[1].tokType != LeftSBrace || assignee[rbAt].tokType != RightSBrace {
                expr.errVal=fmt.Errorf("syntax error in assignment")
                expr.evalError=true
                return
            }

            // get the element name expr, eval it. element.(type) is used in switch below.
            element, err := p.Eval(rfs,assignee[2:rbAt])
            if err!=nil {
                pf("could not evaluate index or key in assignment")
                expr.evalError=true
                expr.errVal=err
                return
            }
            // pf("element [%v] in array access is '%v'\n",element,assignee[2:rbAt])
            /////////////////////////////////////////////////////////////////////////////


            ///////////// CHECK FOR a[e].f= /////////////////////////////////////////////
            if dotMode {
                lhs_dotField:=""
                if dotAt!=len(assignee)-2 {
                    expr.errVal=fmt.Errorf("Too much information in field name!")
                    expr.evalError=true
                    return
                }
                lhs_dotField=assignee[dotAt+1].tokText

                // do everything here and leave other cases alone, or it will get real messy

                // have to vget from a[e] into tmp
                //  then check element type like in normal fieldless switch case
                //  then modify the tmp like we do in the eqpos==3 dotted case
                //  and then write it back to storage
                // feels like a really bad idea this...

                // find stored variable and copy it:

                var tempStore interface{}
                var found bool
                aryName := assignee[0].tokText
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

                // pf("doassign:making tempStore\n")
                tempStore ,found = vgetElement(lfs,aryName, eleName)
                // pf("doassign:tempStore:%#v\n",tempStore)

                if found {

                    // get type info about left/right side of assignment
                    val:=reflect.ValueOf(tempStore)
                    typ:=val.Type()
                    intyp:=reflect.ValueOf(results[assno]).Type()

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
                                tf.Set(reflect.ValueOf(results[assno]))

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
                                expr.errVal=fmt.Errorf("cannot assign result (%T) to %v[%v] (%v)",results[assno],aryName,lhs_dotField,tf.Type())
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
                // pf("-- setting array element : %s [ %v ] with '%v'\n",assignee[0].tokText, element, results[assno])
                vsetElement(lfs, assignee[0].tokText, element.(string), results[assno])
            case int:
                if element.(int)<0 {
                    pf("negative element index!! (%s[%v])\n",assignee[0].tokText,element)
                    expr.evalError=true
                    expr.errVal=err
                }
                vsetElement(lfs, assignee[0].tokText, element.(int), results[assno])
            default:
                pf("unhandled element type!! [%T]\n",element)
                expr.evalError=true
                expr.errVal=err
            }

        case eqPos==3:
            ///////////// CHECK FOR a.f=    /////////////////////////////////////////////
            // dotted
            if assignee[1].tokType == SYM_DOT {

                lhs_v:=assignee[0].tokText
                lhs_f:=assignee[2].tokText
                lhs_o:=assignee[0].offset

                var ts interface{}
                var found bool

                ts,found=vgeti(lfs,lhs_o)

                if found {

                    val:=reflect.ValueOf(ts)
                    typ:=reflect.ValueOf(ts).Type()

                    var intyp reflect.Type
                    // special case, nil
                    if results[assno]!=nil {
                        intyp=reflect.ValueOf(results[assno]).Type()
                    }

                    if typ.Kind()==reflect.Struct {

                        // create temp copy of struct
                        tmp:=reflect.New(val.Type()).Elem()
                        tmp.Set(val)

                        // get the required struct field and make a r/w copy
                        // then assign the new value into the copied field
                        if _,exists:=typ.FieldByName(lhs_f); exists {
                            tf:=tmp.FieldByName(lhs_f)
                            if results[assno]==nil {
                                tf=reflect.NewAt(tf.Type(),unsafe.Pointer(tf.UnsafeAddr())).Elem()
                                tf.Set(reflect.ValueOf([]string{"nil","nil"}))
                                vset(lfs,lhs_v,tmp.Interface())
                            } else {
                                if intyp.AssignableTo(tf.Type()) {
                                    tf=reflect.NewAt(tf.Type(),unsafe.Pointer(tf.UnsafeAddr())).Elem()
                                    tf.Set(reflect.ValueOf(results[assno]))
                                    // write the copy back to the 'real' variable
                                    vset(lfs,lhs_v,tmp.Interface())
                                } else {
                                    pf("cannot assign result (%T) to %v (%v)",results[assno],assignee[0].tokText,tf.Type())
                                    expr.evalError=true
                                    expr.errVal=err
                                }
                            }
                        } else {
                            pf("STRUCT field %v not found in %v",lhs_f,lhs_v)
                            expr.evalError=true
                            expr.errVal=err
                        }

                    } else {
                        pf("variable %v is not a STRUCT",lhs_v)
                        expr.evalError=true
                        expr.errVal=err
                    }

                } else {

                    pf("record variable %v not found",lhs_v)
                    expr.evalError=true
                    expr.errVal=err
                }

            } else {
                pf("assignment looks like it was missing a dot, or you broke it in another way")
                expr.evalError=true
                expr.errVal=err
            }
            /////////////////////////////////////////////////////////////////////////////

        default:
            pf("syntax error in assignment")
            expr.evalError=true
            expr.errVal=err

        }

    } // end for assno

}


