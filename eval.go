package main

import (
    "fmt"
    "io/ioutil"
    "reflect"
    "strconv"
    "math"
    "net/http"
    "sync"
    "sync/atomic"
    str "strings"
    "unsafe"
    "regexp"
)


func (p *leparser) reserved(token Token) (interface{}) {
    panic(fmt.Errorf("statement names cannot be used as identifiers ([%s] %v)",tokNames[token.tokType],token.tokText))
}

func UintPow(n, m uint64) (result uint64) {
    if m == 0 {
        return 1
    }
    result = n
    for i := uint64(2); i <= m; i+=1 {
        result *= n
    }
    return result
}


func (p *leparser) Eval(fs uint32, toks []Token) (interface{},error) {

    // short circuit pure numeric literals and const names
    if len(toks)==1 {
        if toks[0].tokType==NumericLiteral { return toks[0].tokVal,nil }
        switch toks[0].subtype {
        case subtypeConst:
            return toks[0].tokVal,nil
        }
    }

    //    pf("reached dparse: %+v\n",toks)

    p.fs     = fs
    p.tokens = toks
    p.len    = int16(len(toks))
    p.pos    = -1

    return p.dparse(0)
}


type leparser struct {
    tokens      []Token     // the thing getting evaluated
    ident       *[szIdent]Variable // where are the local variables at?
    prev        Token       // bodge for post-fix operations
    preprev     Token       //   and the same for assignment
    fs          uint32      // working function space
    mident      uint32      // fs of main() (1 or 2 depending on interactive mode)
    len         int16       // assigned length to save calling len() during parsing
    line        int16       // shadows lexer source line
    pc          int16       // shadows program counter (pc)
    pos         int16       // distance through parse
    prectable   [END_STATEMENTS]int8
}



func (p *leparser) next() Token {
    if p.pos>0 { p.preprev=p.prev }
    if p.pos>-1 { p.prev=p.tokens[p.pos] }
    p.pos+=1
    return p.tokens[p.pos]
}

func (p *leparser) peek() Token {
    if p.pos+1 == p.len { return Token{tokType:EOF} }
    return p.tokens[p.pos+1]
}


func (p *leparser) dparse(prec int8) (left interface{},err error) {

    // pf("\n\ndparse query     : %+v\n",p.tokens)

    // inlined manually:
    if p.pos>0 { p.preprev=p.prev }
    if p.pos>-1 { p.prev=p.tokens[p.pos] }
    p.pos+=1

    // unaries
    switch p.tokens[p.pos].tokType {
    case O_Comma,SYM_COLON,EOF:
        left=nil
    case RParen, RightSBrace:
        p.next()
        left=nil
    case NumericLiteral:
        left=p.tokens[p.pos].tokVal
    case StringLiteral:
        left=interpolate(p.fs,p.ident,p.tokens[p.pos].tokText)
    case Identifier:
        left=p.identifier(p.tokens[p.pos])
    case O_Sqr, O_Sqrt,O_InFile:
        left=p.unary(p.tokens[p.pos])
    case SYM_Not:
	    right,err := p.dparse(24) // don't bind negate as tightly
        if err!=nil { panic(err) }
		left=unaryNegate(right)
    case O_Slc,O_Suc,O_Sst,O_Slt,O_Srt:
        left=p.unary(p.tokens[p.pos])
    case O_Assign, O_Plus, O_Minus:      // prec variable
        left=p.unary(p.tokens[p.pos])
    case O_Multiply, SYM_Caret:         // unary pointery stuff
        left=p.unary(p.tokens[p.pos])
    case LParen:
        left=p.grouping(p.tokens[p.pos])
    case SYM_PP, SYM_MM:
        left=p.preIncDec(p.tokens[p.pos])
    case LeftSBrace:
        left=p.array_concat(p.tokens[p.pos])
    case O_Query:                       // ternary
        left=p.tern_if(p.tokens[p.pos])
    case O_Ref:
        left=p.reference(false)
    case O_Mut:
        left=p.reference(true)
    case SYM_BOR:
        left=p.command()
    }

    // binaries

    var token Token

    binloop1:
    for {

        if prec >= p.prectable[p.peek().tokType] { break }

        token = p.next()
        switch token.tokType {
        case EOF:
            break binloop1
        case SYM_PP,SYM_MM:
            left = p.postIncDec(token)
            continue
        case LeftSBrace:
            left = p.accessArray(left,token)
            continue
        case SYM_DOT:
            left = p.accessFieldOrFunc(left,p.next().tokText)
            continue
        case LParen:
            switch left.(type) {
            case string:
                left = p.callFunction(left,token)
                continue
            }
        }

        right,err := p.dparse(p.prectable[token.tokType] + 1)

        if err!=nil {
            left = nil
        }

        switch token.tokType {

        case O_Plus:
            left = ev_add(left,right)
        case O_Minus:
            left = ev_sub(left,right)
        case O_Multiply:
            left = ev_mul(left,right)
        case O_Divide:
            left = ev_div(left,right)
        case O_Percent:
            left = ev_mod(left,right)

        case SYM_EQ:
            left = deepEqual(left,right)
        case SYM_NE:
            left = !deepEqual(left,right)
        case SYM_LT:
            left = compare(left,right,"<")
        case SYM_GT:
            left = compare(left,right,">")
        case SYM_LE:
            left = compare(left,right,"<=")
        case SYM_GE:
            left = compare(left,right,">=")

        case SYM_LOR,C_Or:
            switch left.(type) {
            case string:
                switch right.(type) {
                case string:
                    if left.(string)=="" {
                        return right.(string),nil
                    }
                    return left.(string),nil
                }
            }
            left = asBool(left) || asBool(right)
        case SYM_LAND:
            left = asBool(left) && asBool(right)

        case SYM_Tilde:
            left = p.rcompare(left,right,false,false)
        case SYM_ITilde:
            left = p.rcompare(left,right,true,false)
        case SYM_FTilde:
            left = p.rcompare(left,right,false,true)

        case O_Filter:
            left = p.list_filter(left,right)
        case O_Map:
            left = p.list_map(left,right)

        case O_OutFile: // returns success/failure bool
            left = p.file_out(left,right)

        case SYM_BAND: // bitwise-and
            left = as_integer(left) & as_integer(right)
        case SYM_BOR: // bitwise-or
            left = as_integer(left) | as_integer(right)
        case SYM_LSHIFT:
            left = ev_shift_left(left,right)
        case SYM_RSHIFT:
            left = ev_shift_right(left,right)
        case SYM_Caret: // XOR
            left = as_integer(left) ^ as_integer(right)
        case SYM_POW:
            left = ev_pow(left,right)
        case SYM_RANGE:
            left = ev_range(left,right)
        case C_In:
            left = ev_in(left,right)

        case O_Assign:
            panic(fmt.Errorf("assignment is not a valid operation in expressions"))

        }

    }

    // pf("dparse result: %+v\n",left)
    // pf("dparse error : %#v\n",err)

	return left,err
}


type rule struct {
	nud func(token Token) (interface{})
	led func(left interface{}, token Token) (interface{})
	prec int8
}



func (p *leparser) ignore(token Token) interface{} {
    p.next()
    return nil
}

func (p *leparser) binaryLed(prec int8, left interface{}, token Token) (interface{}) {
    return left
}


var cachelock = &sync.RWMutex{}


func (p *leparser) list_filter(left interface{},right interface{}) interface{} {

    switch right.(type) {
    case string:
    default:
        panic(fmt.Errorf("invalid condition string (%+v) in filter",right))
    }

    var reduceparser *leparser
    reduceparser=&leparser{}
    // calllock.RLock()
    reduceparser.prectable=default_prectable
    reduceparser.ident=p.ident
    reduceparser.fs=p.fs
    // calllock.RUnlock()

    switch left.(type) {
    case []string:
        // var new_list []string
        new_list:=make([]string,0,len(left.([]string)))
        for e:=0; e<len(left.([]string)); e+=1 {
            new_right:=str.Replace(right.(string),"#",`"`+left.([]string)[e]+`"`,-1)
            val,err:=ev(reduceparser,p.fs,new_right)
            if err!=nil { panic(err) }
            switch val.(type) {
            case bool:
                if val.(bool) { new_list=append(new_list,left.([]string)[e]) }
            default:
                panic(fmt.Errorf("invalid expression (non-boolean?) (%s) in filter",new_right))
            }
        }
        return new_list

    case []int:
        // var new_list []int
        new_list:=make([]int,0,len(left.([]int)))
        for e:=0; e<len(left.([]int)); e+=1 {
            new_right:=str.Replace(right.(string),"#",strconv.FormatInt(int64(left.([]int)[e]),10),-1)
            val,err:=ev(reduceparser,p.fs,new_right)
            if err!=nil { panic(err) }
            switch val.(type) {
            case bool:
                if val.(bool) { new_list=append(new_list,left.([]int)[e]) }
            default:
                panic(fmt.Errorf("invalid expression (non-boolean?) (%s) in filter",new_right))
            }
        }
        return new_list

    case []uint:
        // var new_list []uint
        new_list:=make([]uint,0,len(left.([]uint)))
        for e:=0; e<len(left.([]uint)); e+=1 {
            new_right:=str.Replace(right.(string),"#",strconv.FormatUint(uint64(left.([]uint)[e]),10),-1)
            val,err:=ev(reduceparser,p.fs,new_right)
            if err!=nil { panic(err) }
            switch val.(type) {
            case bool:
                if val.(bool) { new_list=append(new_list,left.([]uint)[e]) }
            default:
                panic(fmt.Errorf("invalid expression (non-boolean?) (%s) in filter",new_right))
            }
        }
        return new_list

    case []float64:
        // var new_list []float64
        new_list:=make([]float64,0,len(left.([]float64)))
        for e:=0; e<len(left.([]float64)); e+=1 {
            new_right:=str.Replace(right.(string),"#",strconv.FormatFloat(left.([]float64)[e],'g',-1,64),-1)
            val,err:=ev(reduceparser,p.fs,new_right)
            if err!=nil { panic(err) }
            switch val.(type) {
            case bool:
                if val.(bool) { new_list=append(new_list,left.([]float64)[e]) }
            default:
                panic(fmt.Errorf("invalid expression (non-boolean?) (%s) in filter",new_right))
            }
        }
        return new_list

    case []bool:
        var new_list []bool
        for e:=0; e<len(left.([]bool)); e+=1 {
            new_right:=str.Replace(right.(string),"#",strconv.FormatBool(left.([]bool)[e]),-1)
            val,err:=ev(reduceparser,p.fs,new_right)
            if err!=nil { panic(err) }
            switch val.(type) {
            case bool:
                if val.(bool) { new_list=append(new_list,left.([]bool)[e]) }
            default:
                panic(fmt.Errorf("invalid expression (non-boolean?) (%s) in filter",new_right))
            }
        }
        return new_list

    case map[string]string:
        var new_map = make(map[string]string)
        for k,v:=range left.(map[string]string) {
            new_right:=str.Replace(right.(string),"#",`"`+v+`"`,-1)
            val,err:=ev(reduceparser,p.fs,new_right)
            if err!=nil { panic(err) }
            switch val.(type) {
            case bool:
                if val.(bool) { new_map[k]=v }
            default:
                panic(fmt.Errorf("invalid expression (non-boolean?) (%s) in filter",new_right))
            }
        }
        return new_map

    case map[string]interface{}:
        var new_map = make(map[string]interface{})
        for k,v:=range left.(map[string]interface{}) {
            var new_right string
            switch v:=v.(type) {
            case string:
                new_right=str.Replace(right.(string),"#",`"`+v+`"`,-1)
            case int:
                new_right=str.Replace(right.(string),"#",strconv.FormatInt(int64(v),10),-1)
            case uint:
                new_right=str.Replace(right.(string),"#",strconv.FormatUint(uint64(v),10),-1)
            case float64:
                new_right=str.Replace(right.(string),"#",strconv.FormatFloat(v,'g',-1,64),-1)
            case bool:
                new_right=str.Replace(right.(string),"#",strconv.FormatBool(v),-1)
            default:
                new_right=str.Replace(right.(string),"#",sf("%#v",v),-1)
            }
            val,err:=ev(reduceparser,p.fs,new_right)
            if err!=nil { panic(err) }
            switch val.(type) {
            case bool:
                if val.(bool) { new_map[k]=v }
            default:
                panic(fmt.Errorf("invalid expression (non-boolean?) (%s) in filter",new_right))
            }
        }
        return new_map

    case []interface{}:
        var new_list []interface{}
        for e:=0; e<len(left.([]interface{})); e+=1 {
            var new_right string
            switch v:=left.([]interface{})[e].(type) {
            case string:
                new_right=str.Replace(right.(string),"#",`"`+v+`"`,-1)
            case int:
                new_right=str.Replace(right.(string),"#",strconv.FormatInt(int64(v),10),-1)
            case uint:
                new_right=str.Replace(right.(string),"#",strconv.FormatUint(uint64(v),10),-1)
            case float64:
                new_right=str.Replace(right.(string),"#",strconv.FormatFloat(v,'g',-1,64),-1)
            case bool:
                new_right=str.Replace(right.(string),"#",strconv.FormatBool(v),-1)
            default:
                new_right=str.Replace(right.(string),"#",sf("%#v",v),-1)
            }
            val,err:=ev(reduceparser,p.fs,new_right)
            if err!=nil { panic(err) }
            switch val.(type) {
            case bool:
                if val.(bool) { new_list=append(new_list,left.([]interface{})[e]) }
            default:
                panic(fmt.Errorf("invalid expression (non-boolean?) (%s) in filter",new_right))
            }
        }
        return new_list

    default:
        panic(fmt.Errorf("invalid list (%T) in filter",left))
    }
    // unreachable: // return nil
}

func (p *leparser) file_out(left interface{},right interface{}) interface{} {

    switch right.(type) {
    case string:
    default:
        panic(fmt.Errorf("$out requires a filename string on right-hand side"))
    }

    switch left.(type) {
    case string:
    default:
        panic(fmt.Errorf("$out requires an output string on left-hand side"))
    }

    err := ioutil.WriteFile(right.(string), []byte(left.(string)), 0600)
    if err != nil {
        return false
    }
    return true

}

func (p *leparser) list_map(left interface{},right interface{}) interface{} {

    switch right.(type) {
    case string:
    default:
        panic(fmt.Errorf("invalid string (%+v) in map",right))
    }

    var reduceparser *leparser
    reduceparser=&leparser{}
    reduceparser.prectable=default_prectable
    reduceparser.ident=p.ident
    reduceparser.fs=p.fs

    switch left.(type) {

    case []string:
        var new_list []string
        for e:=0; e<len(left.([]string)); e+=1 {
            new_right:=str.Replace(right.(string),"#",`"`+left.([]string)[e]+`"`,-1)
            val,err:=ev(reduceparser,p.fs,new_right)
            if err!=nil { panic(err) }
            switch val.(type) {
            case string:
                new_list=append(new_list,val.(string))
            default:
                panic(fmt.Errorf("invalid expression (%s) in map",new_right))
            }
        }
        return new_list

    case []int:
        var new_list []int
        for e:=0; e<len(left.([]int)); e+=1 {
            new_right:=str.Replace(right.(string),"#",strconv.FormatInt(int64(left.([]int)[e]),10),-1)
            val,err:=ev(reduceparser,p.fs,new_right)
            if err!=nil { panic(err) }
            switch val.(type) {
            case int:
                new_list=append(new_list,val.(int))
            default:
                // panic for now, but should maybe put a default zero type value in instead.
                panic(fmt.Errorf("invalid expression (%s) in map",new_right))
            }
        }
        return new_list

    case []uint:
        var new_list []uint
        for e:=0; e<len(left.([]uint)); e+=1 {
            new_right:=str.Replace(right.(string),"#",strconv.FormatUint(uint64(left.([]uint)[e]),10),-1)
            val,err:=ev(reduceparser,p.fs,new_right)
            if err!=nil { panic(err) }
            switch val.(type) {
            case uint:
                new_list=append(new_list,val.(uint))
            default:
                panic(fmt.Errorf("invalid expression (%s) in map",new_right))
            }
        }
        return new_list

    case []float64:
        var new_list []float64
        for e:=0; e<len(left.([]float64)); e+=1 {
            new_right:=str.Replace(right.(string),"#",strconv.FormatFloat(left.([]float64)[e],'g',-1,64),-1)
            val,err:=ev(reduceparser,p.fs,new_right)
            if err!=nil { panic(err) }
            switch val.(type) {
            case float64:
                new_list=append(new_list,val.(float64))
            default:
                panic(fmt.Errorf("invalid expression (%s) in map",new_right))
            }
        }
        return new_list

    case []bool:
        var new_list []bool
        for e:=0; e<len(left.([]bool)); e+=1 {
            new_right:=str.Replace(right.(string),"#",strconv.FormatBool(left.([]bool)[e]),-1)
            val,err:=ev(reduceparser,p.fs,new_right)
            if err!=nil { panic(err) }
            switch val.(type) {
            case bool:
                new_list=append(new_list,val.(bool))
            default:
                panic(fmt.Errorf("invalid expression (%s) in map",new_right))
            }
        }
        return new_list

    case []interface{}:
        var new_list []interface{}
        for e:=0; e<len(left.([]interface{})); e+=1 {
            var new_right string
            switch v:=left.([]interface{})[e].(type) {
            case string:
                new_right=str.Replace(right.(string),"#",`"`+v+`"`,-1)
            case int:
                new_right=str.Replace(right.(string),"#",strconv.FormatInt(int64(v),10),-1)
            case uint:
                new_right=str.Replace(right.(string),"#",strconv.FormatUint(uint64(v),10),-1)
            case float64:
                new_right=str.Replace(right.(string),"#",strconv.FormatFloat(v,'g',-1,64),-1)
            case bool:
                new_right=str.Replace(right.(string),"#",strconv.FormatBool(v),-1)
            default:
                new_right=str.Replace(right.(string),"#",sf("%#v",v),-1)
            }
            val,err:=ev(reduceparser,p.fs,new_right)
            if err!=nil { panic(err) }
            new_list=append(new_list,val)
        }
        return new_list

    case map[string]interface{}:
        var new_map = make(map[string]interface{})
        for k,v:=range left.(map[string]interface{}) {
            var new_right string
            switch v:=v.(type) {
            case string:
                new_right=str.Replace(right.(string),"#",`"`+v+`"`,-1)
            case int:
                new_right=str.Replace(right.(string),"#",strconv.FormatInt(int64(v),10),-1)
            case uint:
                new_right=str.Replace(right.(string),"#",strconv.FormatUint(uint64(v),10),-1)
            case float64:
                new_right=str.Replace(right.(string),"#",strconv.FormatFloat(v,'g',-1,64),-1)
            case bool:
                new_right=str.Replace(right.(string),"#",strconv.FormatBool(v),-1)
            default:
                new_right=str.Replace(right.(string),"#",sf("%#v",v),-1)
            }
            val,err:=ev(reduceparser,p.fs,new_right)
            if err!=nil { panic(err) }
            new_map[k]=val
        }
        return new_map

    default:
        panic(fmt.Errorf("invalid list (%T) in map",left))
    }
    // unreachable: // return nil
}


func (p *leparser) rcompare (left interface{},right interface{},insensitive bool, multi bool) interface{} {

    switch left.(type) {
    case string:
    default:
        panic(fmt.Errorf("regex comparison requires strings"))
    }

    switch right.(type) {
    case string:
    default:
        panic(fmt.Errorf("regex comparison requires strings"))
    }

    insenStr:=""
    if insensitive { insenStr="(?i)" }

    var re regexp.Regexp
    // cachelock.Lock()
    if pre,found:=ifCompileCache[right.(string)];!found {
        re = *regexp.MustCompile(insenStr+right.(string))
        ifCompileCache[right.(string)]=re
    } else {
        re = pre
    }
    // cachelock.Unlock()

    if multi { return re.FindAllString(left.(string),-1) }

	return re.MatchString(left.(string))
}

func (p *leparser) accessArray(left interface{},right Token) (interface{}) {

    var sz,start,end int
    var hasStart,hasEnd,hasRange bool
    var sendNil bool

    switch left:=left.(type) {
    case []bool:
        sz=len(left)
    case []string:
        sz=len(left)
    case []int:
        sz=len(left)
    case []uint:
        sz=len(left)
    case []float64:
        sz=len(left)
    case []dirent:
        sz=len(left)
    case []alloc_info:
        sz=len(left)
    case string:
        sz=len(left)
    case [][]int:
        sz=len(left)
    case []interface{}:
        sz=len(left)

    case map[string]interface{},map[string]alloc_info,map[string]string,map[string]int:

        // check for key
        var mkey string
        if right.tokType==SYM_DOT {
            t:=p.next()
            mkey=t.tokText
        } else {
            if p.peek().tokType!=RightSBrace {
                dp,err:=p.dparse(0)
                if err!=nil {
                    // panic(fmt.Errorf("map key could not be evaluated"))
                    return nil
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
        return accessArray(p.ident,left,mkey)

        // end map case

    default:
        sendNil=true
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
                if !sendNil {
                    panic(fmt.Errorf("start of range must be an integer (%+v / %T)",dp,dp))
                }
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
                    if !sendNil {
                        panic(fmt.Errorf("end of range must be an integer"))
                    }
                }
            }
        }

        if p.peek().tokType!=RightSBrace {
            panic(fmt.Errorf("end of range brace missing"))
        }

        // swallow brace
        p.next()

    }

    if sendNil { return nil }

    if !hasRange && !hasStart && !hasEnd {
        hasRange=true
    }

    switch hasRange {
    case false:
        return accessArray(p.ident,left,start)
    case true:
        return slice(left,start,end)
    }

    return nil

}

func (p *leparser) callFunction(left interface{},right Token) (interface{}) {

    name:=left.(string)

    // filter for functions here
    var isFunc bool

    // check if exists in user defined function space
    if _, isFunc = stdlib[name]; !isFunc {
        isFunc = fnlookup.lmexists(name)
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
            if p.peek().tokType!=O_Comma {
                break
            }
            p.next()
        }
    }

    if p.peek().tokType==RParen {
        p.next() // consume rparen
    }

    // fmt.Printf("callfunction using (len:%d) ident of %v\n",len(*(p.ident)),*(p.ident))

    return callFunction(p.fs,p.ident,name,iargs)

}


// mut is currently unused and may remain so.
func (p *leparser) reference(mut bool) string {
    vartok:=p.next()
    if ! VarLookup(p.fs,p.ident,vartok.tokText) {
        vset(p.fs,p.ident,vartok.tokText,nil)
    }
    return vartok.tokText
}

func (p *leparser) unaryStringOp(right interface{},op uint8) string {
    switch right.(type) {
    case string:
        switch op {
        case O_Slc:
            return str.ToLower(right.(string))
        case O_Suc:
            return str.ToUpper(right.(string))
        case O_Sst:
            return str.Trim(right.(string)," \t\n\r")
        case O_Slt:
            return str.TrimLeft(right.(string)," \t\n\r")
        case O_Srt:
            return str.TrimRight(right.(string)," \t\n\r")
        default:
            panic(fmt.Errorf("unknown unary string operator!"))
    }
    default:
        panic(fmt.Errorf("invalid type in unary string operator"))
    }
    // unreachable: // return ""
}

func (p *leparser) unary(token Token) (interface{}) {

    switch token.tokType {
    /*
    case O_Ref:
        return p.reference(false)
    case O_Mut:
        return p.reference(true)
	*/
    case O_InFile:
	    right,err := p.dparse(70) // higher than dot op
        if err!=nil { panic(err) }
        return unaryFileInput(right)
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
    case O_Slc,O_Suc,O_Sst,O_Slt,O_Srt:
        return p.unaryStringOp(right,token.tokType)
    case O_Assign:
        panic(fmt.Errorf("unary assignment makes no sense"))
	}

	return nil
}

func unOpSqr(n interface{}) interface{} {
    switch n:=n.(type) {
    case int:
        return n*n
    case uint:
        return n*n
    case float64:
        return n*n
    default:
        panic(fmt.Errorf("sqr does not support type '%T'",n))
    }
    // unreachable: // return nil
}

func unOpSqrt(n interface{}) interface{} {
    switch n:=n.(type) {
    case int:
        return math.Sqrt(float64(n))
    case uint:
        return math.Sqrt(float64(n))
    case float64:
        return math.Sqrt(n)
    default:
        panic(fmt.Errorf("sqrt does not support type '%T'",n))
    }
    // unreachable: // return nil
}

func (p *leparser) tern_if(tok Token) (interface{}) {
    // '??' expr tv [':'|','] fv
    // tv/fv cannot be parenthesised
    dp,err1:=p.dparse(0)
    tv,err2:=p.dparse(0)
    if p.peek().tokType==SYM_COLON || p.peek().tokType==O_Comma {
        p.next()
    }
    fv,err3:=p.dparse(0)
    if err1!=nil || err2!=nil || err3!=nil {
        panic(fmt.Errorf("malformed conditional in expression"))
    }
    switch dp.(type) {
    case bool:
        if dp.(bool) { return tv }
    }
    return fv
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
    switch token.tokType {
    case SYM_MM:
        ampl=-1
    }

    // move parser position to varname 
    vartok:=p.next()

    // exists?
    var there bool
    var val interface{}

    there=VarLookup(p.fs,p.ident,vartok.tokText)
    activeFS:=p.fs
    if !there {
        if VarLookup(p.mident,&mident,vartok.tokText) {
            val,_=vget(p.mident,&mident,vartok.tokText)
            activeFS=p.mident
        }
        if !there { panic(fmt.Errorf("invalid variable name in pre-inc/dec '%s'",vartok.tokText)) }
    } else {
        val,_=vget(p.fs,p.ident,vartok.tokText)
    }

    // act according to var type
    var n interface{}
    switch v:=val.(type) {
    case int:
        n=v+ampl
    case uint:
        n=v+uint(ampl)
    case float64:
        n=v+float64(ampl)
    default:
        p.report(-1,sf("pre-inc/dec not supported on type '%T' (%s)",val,val))
        finish(false,ERR_EVAL)
        return nil
    }
    if activeFS==p.mident {
        vset(p.mident,&mident,vartok.tokText,n)
    } else {
        vset(p.fs,p.ident,vartok.tokText,n)
    }
    return n

}

func (p *leparser) postIncDec(token Token) interface{} {

    // get direction
    ampl:=1
    switch token.tokType {
    case SYM_MM:
        ampl=-1
    }

    // get var from parser context
    vartok:=p.prev

    // exists?
    var there bool
    var val interface{}

    activeFS:=p.fs

    var mloc uint32
    if interactive {
        mloc=1
    } else {
        mloc=2
    }

    activePtr:=p.ident
    if ! VarLookup(p.fs,p.ident,vartok.tokText) {
        if VarLookup(mloc,&mident,vartok.tokText) {
            val,there=vget(mloc,&mident,vartok.tokText)
            activeFS=mloc
            activePtr=&mident
        }
        if !there { panic(fmt.Errorf("invalid variable name in post-inc/dec '%s'",vartok.tokText)) }
    } else {
        val,_=vget(p.fs,p.ident,vartok.tokText)
    }

    // act according to var type
    switch v:=val.(type) {
    case int:
        vset(activeFS,activePtr,vartok.tokText,v+ampl)
    case uint:
        vset(activeFS,activePtr,vartok.tokText,v+uint(ampl))
    case float64:
        vset(activeFS,activePtr,vartok.tokText,v+float64(ampl))
    default:
        panic(fmt.Errorf("post-inc/dec not supported on type '%T' (%s)",val,val))
    }
    return val
}


func (p *leparser) grouping(tok Token) (interface{}) {

	// right-associative
    val,err:=p.dparse(0)
    if err!=nil { panic(err) }
    p.next() // consume RParen
    return val

}

func (p *leparser) number(token Token) (num interface{}) {
    var err error

    // test code:
    num=token.tokVal

    if num==nil {
        panic(err)
    }
	return num
}


func (p *leparser) command() (string) {

    dp,err:=p.dparse(65)
    if err!=nil {
        panic(fmt.Errorf("error parsing string in command operator"))
    }

    switch dp.(type) {
    case string:
    default:
        panic(fmt.Errorf("command operator only accepts strings (not %T)",dp))
    }

    // pf("command : |%s|\n",dp.(string))
    cmd:=system(interpolate(p.fs,p.ident,dp.(string)),false)

    if cmd.okay {
        return cmd.out
    }

    panic(fmt.Errorf("error in command operator (code:%d) '%s'",cmd.code,cmd.err))

}


func (p *leparser) identifier(token Token) (interface{}) {

    // pf("-- identifier query -> %#v[#CTE]\n",token)

    if token.subtype!=subtypeNone {
        switch token.subtype {
        case subtypeConst:
            return token.tokVal
        case subtypeStandard:
            return token.tokText
        case subtypeUser:
            return token.tokText
        }
    }

    // filter for functions here
    //  this also sets the subtype for funcs defined late.
    if p.pos+1!=p.len && p.tokens[p.pos+1].tokType == LParen {
        if _, isFunc := stdlib[token.tokText]; !isFunc {
            // check if exists in user defined function space
            if fnlookup.lmexists(token.tokText) {
                p.tokens[p.pos].subtype=subtypeUser
                return token.tokText
            }
        } else {
            p.tokens[p.pos].subtype=subtypeStandard
            return token.tokText
        }
        panic(fmt.Errorf("function '%v' does not exist",token.tokText))
    }


    // local variable lookup:
    bin:=bind_int(p.fs,token.tokText)
    if (*p.ident)[bin].declared {
        return (*p.ident)[bin].IValue
    }

    // global lookup:
    var val interface{}
    var there bool
    // fmt.Printf("\nglobal identifier fetching (in %d) vget for name : %s\n",p.fs,token.tokText)
    if val,there=vget(p.mident,&mident,token.tokText); there {
        return val
    }

    // permit references to uninitialised variables
    if permit_uninit {
        return nil
    }

    // permit module names
    if modlist[token.tokText]==true {
        return nil
    }

    // permit enum names
    if enum[token.tokText]!=nil {
        return nil
    }

    /*
    pf("-----------------------\n")
    pf("fs      : %d\n",p.fs)
    pf("toktext : %s\n",token.tokText)
    */

    panic(fmt.Errorf("variable '%s' is uninitialised.",token.tokText))

}


/*
 * Replacement variable handlers.
 */

// for locking vset/vcreate/vdelete during a variable write
var vlock = &sync.RWMutex{}

// bah, why do variables have to have names!?! surely an offset would be memorable instead!
func VarLookup(fs uint32, ident *[szIdent]Variable, name string) (bool) {
     // fmt.Printf("vlookup : [%d] %s -> %v\n",fs,name,(*ident)[bind_int(fs,name)].declared)
    if (*ident)[bind_int(fs,name)].declared { return true }
    return false
}



// vcreatetable: creates an empty variable store
// @note: is locked by caller
/*
func vcreatetable(fs uint32, ident *[szIdent]Variable, vtable_maxreached *uint32,sz uint16) {
    vtmr:=*vtable_maxreached
    if fs>=vtmr {
        *vtable_maxreached=fs
        var temp_ident [szIdent]Variable
        *ident=temp_ident
        // fmt.Printf("vct - temp_ident=%#v\n",temp_ident)
        // fmt.Printf("vct - ident=%#v\n",ident)
        // fmt.Printf("vct - gident=%#v\n",gident)
        // fmt.Printf("vcreatetable: just allocated [fs:%d] cap:%d max_reached:%d\n",fs,sz,*vtable_maxreached)
    } else {
        // fmt.Printf("vcreatetable: skipped allocation for [fs:%d] -> length:%v max_reached:%v\n",fs,len(ident),*vtable_maxreached)
    }
}
*/

func vunset(fs uint32, ident *[szIdent]Variable, name string) {
    vlock.Lock()
    if VarLookup(fs, ident, name) {
        (*ident)[bind_int(fs,name)] = Variable{declared:false}
    }
    vlock.Unlock()
}

func vdelete(fs uint32, ident *[szIdent]Variable, name string, ename string) {

    if VarLookup(fs, ident, name) {
        m,_:=vget(fs,ident,name)
        switch m:=m.(type) {
        case map[string][]string:
            delete(m,ename)
            vset(fs,ident,name,m)
        case map[string]string:
            delete(m,ename)
            vset(fs,ident,name,m)
        case map[string]int:
            delete(m,ename)
            vset(fs,ident,name,m)
        case map[string]float64:
            delete(m,ename)
            vset(fs,ident,name,m)
        case map[string]bool:
            delete(m,ename)
            vset(fs,ident,name,m)
        case map[string]interface{}:
            delete(m,ename)
            vset(fs,ident,name,m)
        }
    }
}

/*
func identResize(evalfs uint32,ident *[szIdent]Variable,sz uint16) {
    // fmt.Printf("resize (for %d) - ident len = %v\n",evalfs,len(*ident))
    newar:=make([szIdent]Variable,sz)
    copy(newar,*ident)
    *ident=newar
    // fmt.Printf("resize (for %d) - new_len   = %v\n",evalfs,len(*ident))
}
*/

func vset(fs uint32, ident *[szIdent]Variable, name string, value interface{}) {
    // fmt.Printf("[vs] name : %s\n",name)
    bin:=bind_int(fs,name)
    var locked bool
    if atomic.LoadInt32(&concurrent_funcs)>0 { vlock.Lock() ; locked=true }
    (*ident)[bin].IName=name
    (*ident)[bin].declared=true
    // fmt.Printf("vset bin is now %d for %s\n",bin,name)
    vseti(fs,ident,bin,value)
    if locked { vlock.Unlock() }

    // fmt.Printf("vset value is %+v\n",(*ident)[bin])
}

func vsetInteger(fs uint32, ident *[szIdent]Variable, name string, value int) {
    var ll bool
    bin:=bind_int(fs,name)
    if atomic.LoadInt32(&concurrent_funcs)>0 { ll=true; vlock.Lock() }
    (*ident)[bin]=Variable{IName:name,IValue:value,declared:true}
    /*
    (*ident)[bin].IName=name
    (*ident)[bin].IValue=value
    (*ident)[bin].declared=true
    */
    if ll { vlock.Unlock() }
}


func vseti(fs uint32, ident *[szIdent]Variable, bin uint64, value interface{}) {

    // fmt.Printf("[vseti fs # %d] bind_int of %d = %v\n",fs,bin,value)

    t:=(*ident)[bin]
    t.declared=true

    if (*ident)[bin].ITyped {
        var ok bool
        // t.declared=true
        switch (*ident)[bin].IKind {
        case kbool:
            _,ok=value.(bool)
            if ok { t.IValue = value }
        case kint:
            _,ok=value.(int)
            if ok { t.IValue = value }
        case kuint:
            _,ok=value.(uint)
            if ok { t.IValue = value }
        case kfloat:
            _,ok=value.(float64)
            if ok { t.IValue = value }
        case kstring:
            _,ok=value.(string)
            if ok { t.IValue = value }
        case kbyte:
            _,ok=value.(uint8)
            if ok { t.IValue = value }
        case ksbool:
            _,ok=value.([]bool)
            if ok { t.IValue = value }
        case ksint:
            _,ok=value.([]int)
            if ok { t.IValue = value }
        case ksuint:
            _,ok=value.([]uint)
            if ok { t.IValue = value }
        case ksfloat:
            _,ok=value.([]float64)
            if ok { t.IValue = value }
        case ksstring:
            _,ok=value.([]string)
            if ok { t.IValue = value }
        case ksbyte:
            _,ok=value.([]uint8)
            if ok { t.IValue = value }
        case ksany:
            _,ok=value.([]interface{})
            if ok { t.IValue = value }
        }
        (*ident)[bin]=t

        if !ok { panic(fmt.Errorf("invalid assignation : to type [%T] of [%T]", (*ident)[bin].IValue,value)) }

    } else {
        // undeclared or untyped and needs replacing
        t.IValue=value
        (*ident)[bin]=t
    }

    return

}

func vgetElement(fs uint32, ident *[szIdent]Variable, name string, el string) (interface{}, bool) {
    if VarLookup(fs,ident,name) {
        return vgetElementi(fs,ident,name,el)
    }
    return nil,false
}

func vgetElementi(fs uint32, ident *[szIdent]Variable, name string, el string) (interface{}, bool) {
    var v interface{}
    var ok bool
    v, ok = vget(fs,ident,name)

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
        // pf("vgete-string: v  %s\n",string(v))
        // pf("vgete-string: el %s\n",iel)
        return string(v[iel]),ok
    case []interface{}:
        iel,_:=GetAsInt(el)
        return v[iel],ok
    default:
        // pf("Unknown type in %v[%v] (%T)\n",name,el,v)
        iel,_:=GetAsInt(el)
        for _,val:=range reflect.ValueOf(v).Interface().([]interface{}) {
            if iel==0  { return val,true }
            iel-=1
        }
    }
    return nil, false
}


func vsetElement(fs uint32, ident *[szIdent]Variable, name string, el interface{}, value interface{}) {
    var list interface{}

    if ! VarLookup(fs, ident, name) {
        list = make(map[string]interface{}, LIST_SIZE_CAP)
        vset(fs,ident,name,list)
    }

    vsetElementi(fs,ident,name,el,value)

}

// this could probably be faster. not a great idea duplicating the list like this...

func vsetElementi(fs uint32, ident *[szIdent]Variable, name string, el interface{}, value interface{}) {

    var list interface{}
    var ok bool

    list, ok = vget(fs,ident,name)

    if !ok {
       list = make(map[string]interface{}, LIST_SIZE_CAP)
        vset(fs,ident,name,list)
    }

    bin:=bind_int(fs,name)

    switch list.(type) {

    case map[string]interface{}:
        var key string
        switch el.(type) {
        case int:
            key=strconv.FormatInt(int64(el.(int)), 10)
        case float64:
            key=strconv.FormatFloat(el.(float64), 'f', -1, 64)
        case uint:
            key=strconv.FormatUint(uint64(el.(uint)), 10)
        case string:
            key=el.(string)
        }
        locked:=false
        if atomic.LoadInt32(&concurrent_funcs)>0 { locked=true; vlock.Lock() }
        if ok {
            (*ident)[bin].IValue.(map[string]interface{})[key] = value
        } else {
            (*ident)[bin].IValue.(map[string]interface{})[key] = value
        }
        if locked { vlock.Unlock() }
        return
    }

    numel:=el.(int)
    var fault bool

    locked:=false
    if atomic.LoadInt32(&concurrent_funcs)>0 { locked=true; vlock.Lock() }
    atype:=(*ident)[bin].IValue
    if locked { vlock.Unlock() }

    switch atype.(type) {

    case []int:
        sz:=cap((*ident)[bin].IValue.([]int))
        if numel>=sz {
            newend:=sz*2
            if sz==0 { newend=1 }
            if numel>=newend { newend=numel+1 }
            newar:=make([]int,newend,newend)
            copy(newar,(*ident)[bin].IValue.([]int))
            vset(fs,ident,name,newar)
        }
        if atomic.LoadInt32(&concurrent_funcs)>0 { vlock.Lock() }
        (*ident)[bin].IValue.([]int)[numel]=value.(int)
        if atomic.LoadInt32(&concurrent_funcs)>0 { vlock.Unlock() }

    case []uint8:
        sz:=cap((*ident)[bin].IValue.([]uint8))
        if numel>=sz-1 {
            newend:=sz*2
            if sz==0 { newend=1 }
            if numel>=newend { newend=numel+1 }
            newar:=make([]uint8,newend,newend)
            copy(newar,(*ident)[bin].IValue.([]uint8))
            vset(fs,ident,name,newar)
        }
        if atomic.LoadInt32(&concurrent_funcs)>0 { vlock.Lock() }
        (*ident)[bin].IValue.([]uint8)[numel]=value.(uint8)
        if atomic.LoadInt32(&concurrent_funcs)>0 { vlock.Unlock() }

    case []uint:
        sz:=cap((*ident)[bin].IValue.([]uint))
        if numel>=sz-1 {
            newend:=sz*2
            if sz==0 { newend=1 }
            if numel>=newend { newend=numel+1 }
            newar:=make([]uint,newend,newend)
            copy(newar,(*ident)[bin].IValue.([]uint))
            vset(fs,ident,name,newar)
        }
        if atomic.LoadInt32(&concurrent_funcs)>0 { vlock.Lock() }
        (*ident)[bin].IValue.([]uint)[numel]=value.(uint)
        if atomic.LoadInt32(&concurrent_funcs)>0 { vlock.Unlock() }

    case []bool:
        sz:=cap((*ident)[bin].IValue.([]bool))
        if numel>=sz {
            newend:=sz*2
            if sz==0 { newend=1 }
            if numel>=newend { newend=numel+1 }
            newar:=make([]bool,newend,newend)
            copy(newar,(*ident)[bin].IValue.([]bool))
            vset(fs,ident,name,newar)
        }
        if atomic.LoadInt32(&concurrent_funcs)>0 { vlock.Lock() }
        (*ident)[bin].IValue.([]bool)[numel]=value.(bool)
        if atomic.LoadInt32(&concurrent_funcs)>0 { vlock.Unlock() }

    case []string:
        sz:=cap((*ident)[bin].IValue.([]string))
        if numel>=sz {
            newend:=sz*2
            if sz==0 { newend=1 }
            if numel>=newend { newend=numel+1 }
            newar:=make([]string,newend,newend)
            copy(newar,(*ident)[bin].IValue.([]string))
            vset(fs,ident,name,newar)
        }
        if atomic.LoadInt32(&concurrent_funcs)>0 { vlock.Lock() }
        (*ident)[bin].IValue.([]string)[numel]=value.(string)
        if atomic.LoadInt32(&concurrent_funcs)>0 { vlock.Unlock() }

    case []float64:
        sz:=cap((*ident)[bin].IValue.([]float64))
        if numel>=sz {
            newend:=sz*2
            if sz==0 { newend=1 }
            if numel>=newend { newend=numel+1 }
            newar:=make([]float64,newend,newend)
            copy(newar,(*ident)[bin].IValue.([]float64))
            vset(fs,ident,name,newar)
        }
        if atomic.LoadInt32(&concurrent_funcs)>0 { vlock.Lock() }
        (*ident)[bin].IValue.([]float64)[numel],fault=GetAsFloat(value)
        if atomic.LoadInt32(&concurrent_funcs)>0 { vlock.Unlock() }
        if fault {
            panic(fmt.Errorf("Could not append to float array (ele:%v) a value '%+v' of type '%T'",numel,value,value))
        }

    case []interface{}:
        sz:=cap((*ident)[bin].IValue.([]interface{}))
        if numel>=sz {
            newend:=sz*2
            if sz==0 { newend=1 }
            if numel>=newend { newend=numel+1 }
            newar:=make([]interface{},newend,newend)
            copy(newar,(*ident)[bin].IValue.([]interface{}))
            vset(fs,ident,name,newar)
        }
        if atomic.LoadInt32(&concurrent_funcs)>0 { vlock.Lock() }
        if value==nil {
            (*ident)[bin].IValue.([]interface{})[numel]=nil
        } else {
            (*ident)[bin].IValue.([]interface{})[numel]=value.(interface{})
        }
        if atomic.LoadInt32(&concurrent_funcs)>0 { vlock.Unlock() }

    default:
        pf("DEFAULT: Unknown type %T for list %s\n",list,name)

    }

}

func vget(fs uint32, ident *[szIdent]Variable,name string) (interface{}, bool) {
    // if atomic.LoadInt32(&concurrent_funcs)>0 { vlock.Lock() ; defer vlock.Unlock() }
    if VarLookup(fs,ident, name) {
        bin:=bind_int(fs,name)
        // pf("\n--vget-- for %s in fs %d -> bin %d value %+v\n",name,fs,bin,(*ident)[bin])
        // pf("      -- bindings : %+v\n",bindings[fs])
        // pf("      --    cache : %+v\n",lru_bind_cache)
        // most:=32 ; if int(bin)<most { most=int(bin) }
        // for e:=0; e<=most; e++ { pf("      -- idents [#%d/%d]  -> %+v\n",fs,e,(*ident)[e]) }
        // if atomic.LoadInt32(&concurrent_funcs)>0 { vlock.Lock() ; defer vlock.Unlock() }
        if (*ident)[bin].declared { return (*ident)[bin].IValue,true }
        return nil,false
    }
    return nil, false
}

func vgeti(fs uint32, ident *[szIdent]Variable,id uint64) (interface{}) {
    // if atomic.LoadInt32(&concurrent_funcs)>0 { vlock.RLock() ; defer vlock.RUnlock() }
    if (*ident)[id].declared { return (*ident)[id].IValue }
    return nil
}

func isBool(expr interface{}) bool {
    switch reflect.TypeOf(expr).Kind() {
    case reflect.Bool:
        return true
    }
    return false
}


func isNumber(expr interface{}) bool {
    typeof := reflect.TypeOf(expr).Kind()
    switch typeof {
    case reflect.Float64, reflect.Int, reflect.Int64, reflect.Uint, reflect.Uint8:
        return true
    }
    return false
}


/// convert variable placeholders in strings to their values
func interpolate(fs uint32, ident *[szIdent]Variable, s string) (string) {

    if atomic.LoadInt32(&concurrent_funcs)>0 {
        lastlock.Lock()
        defer lastlock.Unlock()
    }

    if !interpolation {
        return s
    }

    // should finish sooner if no curly open brace in string.
    if str.IndexByte(s, '{') == -1 {
        return s
    }

    orig:=s
    r := regexp.MustCompile(`{([^{}]*)}`)

    ofs:=interparse.fs
    oident:=interparse.ident
    interparse.fs=fs
    interparse.ident=ident

    for {
        os:=s

        // generate list of matches of {...} in s
        matches := r.FindAllStringSubmatch(s,-1)

        for _, v := range matches {

            kn:=v[1]
            if kn[0]=='=' { continue }

            if kv,there:=vget(fs,ident,kn); there {
                // pf("[interpol] looked up in #%d %s : value %+v[#CTE]\n",fs,kn,kv)
                switch kv.(type) {
                case int:
                    s = str.Replace(s, "{"+kn+"}", strconv.FormatInt(int64(kv.(int)), 10),-1)
                case float64:
                    s = str.Replace(s, "{"+kn+"}", strconv.FormatFloat(kv.(float64),'g',-1,64),-1)
                case bool:
                    s = str.Replace(s, "{"+kn+"}", strconv.FormatBool(kv.(bool)),-1)
                case string:
                    s = str.Replace(s, "{"+kn+"}", kv.(string),-1)
                case uint:
                    s = str.Replace(s, "{"+kn+"}", strconv.FormatUint(uint64(kv.(uint)), 10),-1)
                case []uint, []float64, []int, []bool, []interface{}, []string:
                    s = str.Replace(s, "{"+kn+"}", sf("%v",kv),-1)
                case interface{}:
                    s = str.Replace(s, "{"+kn+"}", sf("%v",kv),-1)
                default:
                    s = str.Replace(s, "{"+kn+"}", sf("!%T!%v",kv,kv),-1)

                }
            }
        }

        if os==s { break }
    }

    // if nothing was replaced, check if evaluation possible, then it's time to leave this infernal place
    var modified bool

    redo:=true

    for ;redo; {
        modified=false
        for p:=0;p<len(s)-1;p+=1 {
            if s[p]=='{' && s[p+1]=='=' {
                q:=str.IndexByte(s[p:],'}') // don't start at greater offset or have to make assumptions about len
                if q==-1 { break }
                if aval, err := ev(interparse,fs,s[p+2:p+q]); err==nil {
                    s=s[:p]+sf("%v",aval)+s[p+q+1:]
                    modified=true
                    break
                }
                p=q+1
            }
        }
        if !modified { redo=false }
    }

    interparse.ident=oident
    interparse.fs=ofs

    if s=="<nil>" { s=orig }

    return s
}


// evaluate an expression string
func ev(parser *leparser,fs uint32, ws string) (result interface{}, err error) {

    // build token list from string 'ws'
    toks:=make([]Token,0,6)
    var cl int16
    var p int
    var t *lcstruct
    for p = 0; p < len(ws);  {
        t = nextToken(ws, &cl, p)
        // pf("ev: p:%2d t:%+v\n",p,t)
        if t.tokPos != -1 {
            p = t.tokPos
        }
        toks = append(toks, t.carton)
        if t.eof { break }
    }

    // evaluate token list
    if len(toks)!=0 {
        result, err = parser.Eval(fs,toks)
    }

    if result==nil { // could not eval
        if err!=nil {
            parser.report(-1,sf("Error evaluating '%s'",ws))
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

func (p *leparser) wrappedEval(lfs uint32, lident *[szIdent]Variable, fs uint32, rident *[szIdent]Variable, tks []Token) (expr ExpressionCarton) {

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

            // override p.prev value as postIncDec uses it and we will be throwing 
            //  away the p.* values shortly after this use.
            p.prev=tks[0]
            p.postIncDec(tks[1])
            expr.assign=true
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
             // pf("Assign query          : %+v\n",tks[k+1:])
            expr.result, err = p.Eval(fs,tks[k+1:])
             // pf("Assign expression box : %+v\n\n",expr)
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
        expr.assignPos=-1
    } else {
        expr.assign=true
        expr.assignPos=eqPos
        // before eval, rewrite lhs token bindings to their lhs equivalent
        if !standardAssign {
            if lfs!=fs {
                if newEval[0].tokType==Identifier {
                    if VarLookup(lfs,lident,newEval[0].tokText) {
                        if ! (*lident)[bind_int(lfs,newEval[0].tokText)].declared {
                            p.report(-1,"you may only amend declared variables outside of local scope")
                            expr.evalError=true
                            finish(false,ERR_SYNTAX)
                            return expr
                        }
                    } else {
                        p.report(-1,"you may only amend existing variables outside of local scope")
                        expr.evalError=true
                        finish(false,ERR_SYNTAX)
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

            // lots of crap here since moving to separate ident[] per func :)

            /*
            pf("\n\n\n[#1]weval | tok | %+v[#-]\n",tks)
            pf("[#1]weval | nev | %+v[#-]\n",newEval)
            pf("[#1]weval | lhs | fs %d | ident_addr %p[#-]\n",lfs,lident)
            pf("[#1]weval | rhs | fs %d | ident_addr %p[#-]\n\n\n\n",fs,rident)
            */

            // eval
            // pf("[#2]weval |  PRE RESULT[#-]\n")
            // pf("[#1]weval | exp | %#v[#-]\n",expr)

            oid:=p.ident; p.ident=lident
            expr.result, err = p.Eval(lfs,newEval)
            p.ident=oid

            // pf("[#2]weval | POST RESULT[#-]\n")
        }
    }


    if err!=nil {
        expr.evalError=true
        expr.errVal=err
        return expr
    }

    if expr.assign {
        /*
        pf("-- entering doAssign (lfs->%d,rfs->%d) with tokens : %+v\n",lfs,fs,tks)
        pf("-- entering doAssign (lfs->%d,rfs->%d) with value  : %+v\n",lfs,fs,expr.result)
        pf("-- entering doAssign (lfs->%d,rfs->%d) with lfs bindings: %+v\n",lfs,fs,bindings[lfs])
        pf("-- entering doAssign (lfs->%d,rfs->%d) with rfs bindings: %+v\n",lfs,fs,bindings[fs])
        */
        p.doAssign(lfs,lident,fs,rident,tks,&expr,eqPos)
        // pf("-- exited   doAssign (lfs->%d,rfs->%d) with idents of %+v\n",lfs,fs,lident)
        // pf("-- exited   doAssign (lfs->%d,rfs->%d) with lfs bindings: %+v\n",lfs,fs,bindings[lfs])
        // pf("-- exited   doAssign (lfs->%d,rfs->%d) with rfs bindings: %+v\n",lfs,fs,bindings[fs])
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

func (p *leparser) doAssign(lfs uint32, lident *[szIdent]Variable, rfs uint32, rident *[szIdent]Variable, tks []Token,expr *ExpressionCarton,eqPos int) {

    // (left)  lfs is the function space to assign to
    // (right) rfs is the function space to evaluate with (calculating indices expressions, etc)

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
        var scrap [16]Token
        scrapCount:=0
        for tok := range tks[:eqPos] {
            nt:=tks[tok]
            if nt.tokType==LParen || nt.tokType==LeftSBrace  { evnest+=1 }
            if nt.tokType==RParen || nt.tokType==RightSBrace { evnest-=1 }
            if nt.tokType!=O_Comma || evnest>0 {
                scrap[scrapCount]=nt
                scrapCount+=1
            }
            if evnest==0 && (tok==eqPos-1 || nt.tokType == O_Comma) {
                largs[curArg]=append(largs[curArg],scrap[:scrapCount]...)
                scrapCount=0
                curArg+=1
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
        if expr.result==nil {
            results=[]interface{}{nil}
        } else {
            results=[]interface{}{expr.result}
        }
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

        if assignee[0].tokType!=Identifier {
            expr.errVal=fmt.Errorf("Assignee must be an identifier (not '%s')",assignee[0].tokText)
            expr.evalError=true
            return
        }

        // ignore assignment to underscore
        if strcmp(assignee[0].tokText,"_") { continue }

        // then apply the shite below to each one, using the next available result from results[]

        dotAt:=-1
        rbAt :=-1
        var rbSet, dotSet bool
        for dp:=len(assignee)-1;dp>0;dp-=1 {
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
            // pf("-- normal assignment to (ifs:%d) %s of %+v [%T]\n", lfs, assignee[0].tokText, results[assno],results[assno])
            vset(lfs, lident, assignee[0].tokText, results[assno])
            /////////////////////////////////////////////////////////////////////////////

        case len(assignee)==2:

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
                    eleName = interpolate(rfs,rident,element.(string))
                default:
                    eleName = sf("%v",element)
                }

                tempStore ,found = vgetElementi(lfs,lident,aryName, eleName)

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
                                    vsetElementi(lfs,lident,aryName,element.(int),tmp.Interface())
                                case string:
                                    vsetElementi(lfs,lident,aryName,element.(string),tmp.Interface())
                                default:
                                    vsetElementi(lfs,lident,aryName,element.(string),tmp.Interface())
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
                element = interpolate(rfs,rident,element.(string))
                vsetElementi(lfs, lident, assignee[0].tokText, element.(string), results[assno])
            case int:
                if element.(int)<0 {
                    pf("negative element index!! (%s[%v])\n",assignee[0].tokText,element)
                    expr.evalError=true
                    expr.errVal=err
                }
                vsetElementi(lfs, lident, assignee[0].tokText, element.(int), results[assno])
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

                var ts interface{}
                var found bool

                ts,found=vget(lfs,lident,lhs_v)

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
                                vset(lfs,lident,lhs_v,tmp.Interface())
                            } else {
                                if intyp.AssignableTo(tf.Type()) {
                                    tf=reflect.NewAt(tf.Type(),unsafe.Pointer(tf.UnsafeAddr())).Elem()
                                    tf.Set(reflect.ValueOf(results[assno]))
                                    // write the copy back to the 'real' variable
                                    vset(lfs,lident,lhs_v,tmp.Interface())
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


