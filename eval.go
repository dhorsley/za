package main

import (
    "fmt"
    "reflect"
    "strconv"
    "math"
    //    "math/rand" // for uid in vgeti
    "net/http"
    "sync"
    // "sync/atomic"
    str "strings"
    "unsafe"
    "regexp"
)


func (p *leparser) reserved(token Token) (interface{}) {
    panic(fmt.Errorf("statement names cannot be used as identifiers ([%s] %v)",tokNames[token.tokType],token.tokText))
}


func (p *leparser) Eval(fs uint32, toks []Token) (interface{},error) {

    p.fs     = fs
    p.tokens = toks
    p.len    = int16(len(toks))
    p.pos    = -1

    return p.dparse(0)
}


type leparser struct {
    tokens      []Token     // the thing getting evaluated
    fs          uint32      // working function space
    ident       *[]Variable // where are the local variables at?
    force_lookup bool
    len         int16       // assigned length to save calling len() during parsing
    line        int16       // shadows lexer source line
    pc          int16       // shadows program counter (pc)
    pos         int16       // distance through parse
    prev        Token       // bodge for post-fix operations
    preprev     Token       //   and the same for assignment
    prectable   [END_STATEMENTS]int8
}



func (p *leparser) next() Token {

    if p.pos>0 { p.preprev=p.prev }
    if p.pos>-1 { p.prev=p.tokens[p.pos] }

    p.pos+=1
    return p.tokens[p.pos]

}

func (p *leparser) peek() Token {

    if p.pos+1 == p.len {
        return Token{tokType:EOF}
    }

    return p.tokens[p.pos+1]
}


func (p *leparser) dparse(prec int8) (left interface{},err error) {

    // pf("\n\ndparse query     : %+v\n",p.tokens)

    current:=p.next()

    // unaries
    switch current.tokType {
    case O_Comma,SYM_COLON,EOF:
        left=nil
    case RParen, RightSBrace:
        p.next()
        left=nil
    case NumericLiteral:
        left=current.tokVal
    case StringLiteral:
        left=interpolate(p.fs,p.ident,current.tokText)
    case Identifier:
        left=p.identifier(current)
    case O_Sqr, O_Sqrt, O_InFile:
        left=p.unary(current)
    case SYM_Not:
	    right,err := p.dparse(24) // don't bind negate as tightly
        if err!=nil { panic(err) }
		left=unaryNegate(right)
    case O_Slc,O_Suc,O_Sst,O_Slt,O_Srt:
        left=p.unary(current)
    case O_Assign, O_Plus, O_Minus:      // prec variable
        left=p.unary(current)
    case O_Multiply, SYM_Caret:         // unary pointery stuff
        left=p.unary(current)
    case LParen:
        left=p.grouping(current)
    case SYM_PP, SYM_MM:
        left=p.preIncDec(current)
    case LeftSBrace:
        left=p.array_concat(current)
    case O_Query:                       // ternary
        left=p.tern_if(current)
    case O_Ref:
        left=p.reference(false)
    case O_Mut:
        left=p.reference(true)
    }

    // binaries

    var token Token

    binloop1:
    for {

        if prec >= p.prectable[p.peek().tokType] { break }

        // token := p.next()
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
            /*
            if p.peek().tokText=="format" {
                fmt.Printf("afof with prev of      : %+v\n",p.prev)
                fmt.Printf("afof with preprev of   : %+v\n",p.preprev)
                fmt.Printf("afof entry param 1 (%T): %+v\n",left)
                fmt.Printf("afof entry param 2     : %+v\n",p.peek().tokText)
            }
            */
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

func (p *leparser) list_map(left interface{},right interface{}) interface{} {

    switch right.(type) {
    case string:
    default:
        panic(fmt.Errorf("invalid string (%+v) in map",right))
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
    case []interface{}:
        sz=len(left)

    case map[string]interface{},map[string]alloc_info,map[string]string,map[string]int,map[int]interface{},map[int]int,map[int]string,map[int][]int,map[int][]string,map[int][]interface{}:

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
    if _,there:=VarLookup(p.fs,vartok.tokText); there {
        return vartok.tokText
    } else {
        panic(fmt.Errorf("reference to unknown variable"))
    }
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

    /*
    switch token.tokType {
    case O_Ref:
        return p.reference(false)
    case O_Mut:
        return p.reference(true)
    }
    */

    /*
	switch token.tokType {
    case SYM_Not:
	    right,err := p.dparse(24) // don't bind negate as tightly
        if err!=nil { panic(err) }
		return unaryNegate(right)
    }
    */

	right,err := p.dparse(38) // between grouping and other ops
    if err!=nil { panic(err) }

	switch token.tokType {
	case O_Minus:
		return unaryMinus(right)
	case O_Plus:
		return unaryPlus(right)
	case O_InFile:
        return unaryFileInput(right)
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
    var vi uint16
    var there bool
    var val interface{}

    vi,there=VarLookup(p.fs,vartok.tokText)
    activeFS:=p.fs
    if !there {
        if vi,there=VarLookup(1,vartok.tokText); there {
            val,_=vgeti(1,&mident,vi)
            activeFS=1
        }
        if !there { panic(fmt.Errorf("invalid variable name in pre-inc/dec '%s'",vartok.tokText)) }
    } else {
        val,_=vgeti(p.fs,p.ident,vartok.offset)
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
    if activeFS==1 {
        vset(1,&mident,vartok.tokText,n)
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
    var vi uint16
    var there bool
    var val interface{}

    vi,there=VarLookup(p.fs,vartok.tokText)
    activeFS:=p.fs
    activePtr:=p.ident
    if !there {
        if vi,there=VarLookup(1,vartok.tokText); there {
            val,_=vgeti(1,&mident,vi)
            activeFS=1
            activePtr=&mident
        }
        if !there { panic(fmt.Errorf("invalid variable name in post-inc/dec '%s'",vartok.tokText)) }
    } else {
        val,_=vgeti(p.fs,p.ident,vartok.offset)
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

    /* TEST REMOVAL (dh):
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
    */

    if num==nil {
        panic(err)
    }
	return num
}

func (p *leparser) identifier(token Token) (interface{}) {

    // pf("-- identifier query -> [%+v]\n",token)

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
    //  @note: still have to do this, even though we sometimes set this 
    //  in phraser.go, as user function definitions may appear after 
    //  a reference to them.

    if p.peek().tokType == LParen {
        if _, isFunc := stdlib[token.tokText]; !isFunc {
            // check if exists in user defined function space
            if fnlookup.lmexists(token.tokText) {
                return token.tokText
            }
        } else {
            return token.tokText
        }

        panic(fmt.Errorf("function '%v' does not exist",token.tokText))
    }


    // local variable lookup:

    if interactive || p.force_lookup {
        if val,there:=vget(p.fs,p.ident,token.tokText); there {
            return val
        }
    } else {
        // pf("identifier fetching (in %d) (ident ptr : %p) vgeti for name : %s\n",p.fs,p.ident,token.tokText)
        if val,there:=vgeti(p.fs,p.ident,token.offset); there {
            return val
        }
    }

    // global lookup:

    if !interactive {
        // fmt.Printf("\nglobal identifier fetching (in %d) vgeti for name : %s\n",p.fs,token.tokText)
        if vi,there:=VarLookup(1,token.tokText); there {
            // uid:=rand.Intn(10000)
            // fmt.Printf("\n\ngi %d - post varlookup - query = %s    \n\n",uid,token.tokText)
            if v,ok:=vgeti(1,&mident,vi); ok {
                // fmt.Printf("\n\ngi %d - post vgeti : found value -> %+v\n\n",uid,v)
                return v
            } else {
                // fmt.Printf("\n\ngi %d - var %d not found in mident\n\n",uid,vi)
            }
        }
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
    // globlock.RLock()
    // defer globlock.RUnlock()
    if enum[token.tokText]!=nil {
        return nil
    }

    panic(fmt.Errorf("variable '%s' is uninitialised.",token.tokText))

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
            return 0,false
        }
        return vi,true
    }
    return 0,false

}


// vcreatetable: creates an empty variable store
// @note: is locked by caller
func vcreatetable(fs uint32, ident *[]Variable, vtable_maxreached *uint32,sz uint16) {

    vtmr:=*vtable_maxreached

    if fs>=vtmr {
        *vtable_maxreached=fs
        var temp_ident = make([]Variable, 0, sz)
        *ident=temp_ident
        /*
        fmt.Printf("vct - temp_ident=%#v\n",temp_ident)
        fmt.Printf("vct - ident=%#v\n",ident)
        fmt.Printf("vct - gident=%#v\n",gident)
        */
        // fmt.Printf("vcreatetable: just allocated [fs:%d] cap:%d max_reached:%d\n",fs,sz,*vtable_maxreached)
    } else {
        // fmt.Printf("vcreatetable: skipped allocation for [fs:%d] -> length:%v max_reached:%v\n",fs,len(ident),*vtable_maxreached)
    }

}

func vunset(fs uint32, ident *[]Variable, name string) {
    vlock.Lock()
    loc, found := VarLookup(fs, name)
    if found { (*ident)[loc] = Variable{declared:false} }
    vlock.Unlock()
}

func vdelete(fs uint32, ident *[]Variable, name string, ename string) {

    if _, ok := VarLookup(fs, name); ok {
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

func identResize(evalfs uint32,ident *[]Variable,sz uint16) {
    // fmt.Printf("resize (for %d) - ident len = %v\n",evalfs,len(*ident))
    newar:=make([]Variable,sz,sz)
    copy(newar,*ident)
    *ident=newar
    // fmt.Printf("resize (for %d) - new_len   = %v\n",evalfs,len(*ident))
}


func vset(fs uint32, ident *[]Variable, name string, value interface{}) (uint16) {

    // create mapping entries for this name if it does not already exist
    // vlock.Lock()
    // fmt.Printf("vset (to fs of %d)-  pre-gidcheck ident=%v\nmident=%v\n",fs,ident,mident)
    // fmt.Printf("vset - post-gidcheck ident=%#v - mident=%#v\n",ident,mident)
    if _,found:=vmap[fs][name]; !found {
        vmap[fs][name]=functionidents[fs]+1
        unvmap[fs][functionidents[fs]]=name
        identResize(fs,ident,functionidents[fs]+1)
        functionidents[fs]+=1
    }
    ovi:=vmap[fs][name]
    // fmt.Printf("vset - post-resize   ident=%v\nmident=%v\n",ident,mident)
    // vlock.Unlock()

    // ... then forward to vseti
    return vseti(fs, ident, name, ovi, value)
}

func vseti(fs uint32, ident *[]Variable, name string, vi uint16, value interface{}) (uint16) {

    // @note: should probably still have a lock for functionidents, vmap and unvmap access

    // fmt.Printf("\n ** vseti : ident addr %p : mident addr %p **\n",ident,mident)

    if len(*ident)>=int(vi) {

        if len(*ident)==int(vi) {
            identResize(fs,ident,vi+1)
            functionidents[fs]=vi+1
        }

        // check for conflict with previous VAR
        if (*ident)[vi].ITyped {
            var ok bool
            switch (*ident)[vi].IKind {
            case kbool:
                _,ok=value.(bool)
                if ok { (*ident)[vi].IValue = value }
            case kint:
                _,ok=value.(int)
                if ok { (*ident)[vi].IValue = value }
            case kuint:
                _,ok=value.(uint)
                if ok { (*ident)[vi].IValue = value }
            case kfloat:
                _,ok=value.(float64)
                if ok { (*ident)[vi].IValue = value }
            case kstring:
                _,ok=value.(string)
                if ok { (*ident)[vi].IValue = value }
            case kbyte:
                _,ok=value.(uint8)
                if ok { (*ident)[vi].IValue = value }
            case ksbool:
                _,ok=value.([]bool)
                if ok { (*ident)[vi].IValue = value }
            case ksint:
                _,ok=value.([]int)
                if ok { (*ident)[vi].IValue = value }
            case ksuint:
                _,ok=value.([]uint)
                if ok { (*ident)[vi].IValue = value }
            case ksfloat:
                _,ok=value.([]float64)
                if ok { (*ident)[vi].IValue = value }
            case ksstring:
                _,ok=value.([]string)
                if ok { (*ident)[vi].IValue = value }
            case ksbyte:
                _,ok=value.([]uint8)
                if ok { (*ident)[vi].IValue = value }
            }

            if !ok {
                panic(fmt.Errorf("invalid assignation to '%v' [%T] of %v [%T]",
                    name,(*ident)[vi].IValue,value,value),
                )
            }

        } else {
            if !(*ident)[vi].declared {
                // exists, but not in use
                (*ident)[vi]=Variable{IName:name,IValue:value,declared:true}
                vmap[fs][name]=vi
                unvmap[fs][vi]=name
            } else {
                // declared so alter
                // fmt.Printf("altered pre-declared variable %s (vi %d) (in %d) with value: %v\n",name,vi,fs,value)
                (*ident)[vi].IValue = value
            }
        }

    } else {

        // new variable instantiation
        // fmt.Printf("vseti -  pre-resize ident = %#v\n",ident)
        if len(*ident)<int(vi) {
            identResize(fs,ident,vi+1)
        }
        // fmt.Printf("vseti - post-resize ident = %#v\n",ident)
        (*ident)[vi]=Variable{IName:name,IValue:value,declared:true}
        vmap[fs][name]=vi
        unvmap[fs][vi]=name
        functionidents[fs]+=1

    }


    return vi

}

func vgetElement(fs uint32, ident *[]Variable, name string, el string) (interface{}, bool) {
    if vi,ok := VarLookup(fs,name); ok {
        return vgetElementi(fs,ident,name,vi,el)
    }
    return nil,false
}

func vgetElementi(fs uint32, ident *[]Variable, name string, vi uint16, el string) (interface{}, bool) {
    var v interface{}
    var ok bool
    v, ok = vgeti(fs,ident,vi)

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


func vsetElement(fs uint32, ident *[]Variable, name string, el interface{}, value interface{}) {
    var list interface{}

    vlock.RLock()
    vi, declared := VarLookup(fs, name)
    vlock.RUnlock()

    if !declared {
        list = make(map[string]interface{}, LIST_SIZE_CAP)
        vi=vset(fs,ident,name,list)
    }

    vsetElementi(fs,ident,name,vi,el,value)

}

// this could probably be faster. not a great idea duplicating the list like this...

func vsetElementi(fs uint32, ident *[]Variable, name string, vi uint16, el interface{}, value interface{}) {

    var list interface{}
    var ok bool

    list, ok = vgeti(fs,ident,vi)

    if !ok {
       list = make(map[string]interface{}, LIST_SIZE_CAP)
        vi=vset(fs,ident,name,list)
    }


    // vlock.Lock()
    switch list.(type) {

    case map[int]interface{}:
        var key int
        switch el.(type) {
        case int:
            key=el.(int)
        case uint:
            key=int(el.(uint))
        }
        if ok {
            (*ident)[vi].IValue.(map[int]interface{})[key] = value
        } else {
            (*ident)[vi].IName = name
            (*ident)[vi].IValue.(map[int]interface{})[key] = value
            // vlock.Unlock()
            return
        }
        // vlock.Unlock()
        return

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
        if ok {
            (*ident)[vi].IValue.(map[string]interface{})[key] = value
        } else {
            (*ident)[vi].IName = name
            (*ident)[vi].IValue.(map[string]interface{})[key] = value
            // vlock.Unlock()
            return
        }
        // vlock.Unlock()
        return
    }

    numel:=el.(int)
    var fault bool

    switch (*ident)[vi].IValue.(type) {

    case []int:
        sz:=cap((*ident)[vi].IValue.([]int))
        if numel>=sz {
            newend:=sz*2
            if sz==0 { newend=1 }
            if numel>=newend { newend=numel+1 }
            newar:=make([]int,newend,newend)
            copy(newar,(*ident)[vi].IValue.([]int))
            (*ident)[vi].IValue=newar
        }
        (*ident)[vi].IValue.([]int)[numel]=value.(int)

    case []uint8:
        sz:=cap((*ident)[vi].IValue.([]uint8))
        if numel>=sz-1 {
            newend:=sz*2
            if sz==0 { newend=1 }
            if numel>=newend { newend=numel+1 }
            newar:=make([]uint8,newend,newend)
            copy(newar,(*ident)[vi].IValue.([]uint8))
            (*ident)[vi].IValue=newar
        }
        (*ident)[vi].IValue.([]uint8)[numel]=value.(uint8)

    case []uint:
        sz:=cap((*ident)[vi].IValue.([]uint))
        if numel>=sz-1 {
            newend:=sz*2
            if sz==0 { newend=1 }
            if numel>=newend { newend=numel+1 }
            newar:=make([]uint,newend,newend)
            copy(newar,(*ident)[vi].IValue.([]uint))
            (*ident)[vi].IValue=newar
        }
        (*ident)[vi].IValue.([]uint)[numel]=value.(uint)

    case []bool:
        sz:=cap((*ident)[vi].IValue.([]bool))
        if numel>=sz {
            newend:=sz*2
            if sz==0 { newend=1 }
            if numel>=newend { newend=numel+1 }
            newar:=make([]bool,newend,newend)
            copy(newar,(*ident)[vi].IValue.([]bool))
            (*ident)[vi].IValue=newar
        }
        (*ident)[vi].IValue.([]bool)[numel]=value.(bool)

    case []string:
        sz:=cap((*ident)[vi].IValue.([]string))
        if numel>=sz {
            newend:=sz*2
            if sz==0 { newend=1 }
            if numel>=newend { newend=numel+1 }
            newar:=make([]string,newend,newend)
            copy(newar,(*ident)[vi].IValue.([]string))
            (*ident)[vi].IValue=newar
        }
        (*ident)[vi].IValue.([]string)[numel]=value.(string)

    case []float64:
        sz:=cap((*ident)[vi].IValue.([]float64))
        if numel>=sz {
            newend:=sz*2
            if sz==0 { newend=1 }
            if numel>=newend { newend=numel+1 }
            newar:=make([]float64,newend,newend)
            copy(newar,(*ident)[vi].IValue.([]float64))
            (*ident)[vi].IValue=newar
        }
        (*ident)[vi].IValue.([]float64)[numel],fault=GetAsFloat(value)
        if fault {
            panic(fmt.Errorf("Could not append to float array (ele:%v) a value '%+v' of type '%T'",numel,value,value))
        }

    case []interface{}:
        sz:=cap((*ident)[vi].IValue.([]interface{}))
        if numel>=sz {
            newend:=sz*2
            if sz==0 { newend=1 }
            if numel>=newend { newend=numel+1 }
            newar:=make([]interface{},newend,newend)
            copy(newar,(*ident)[vi].IValue.([]interface{}))
            (*ident)[vi].IValue=newar
        }
        if value==nil {
            (*ident)[vi].IValue.([]interface{})[numel]=nil
        } else {
            (*ident)[vi].IValue.([]interface{})[numel]=value.(interface{})
        }

    default:
        pf("DEFAULT: Unknown type %T for list %s\n",list,name)

    }

    // vlock.Unlock()

}

func vget(fs uint32, ident *[]Variable,name string) (interface{}, bool) {
    // vlock.RLock()
    // fmt.Printf("vget - req: fs->%d name->%s\n",fs,name)

    if vi, ok := VarLookup(fs,name); ok {

        // workaround, hopefully temporary, for split ident tables
        // if ident==nil                 { return nil,false }
        // if len(*ident)<=int(vi)       { return nil,false }
        // if (*ident)[vi].IName != name { return nil,false }
        // end-bodge

        v:=(*ident)[vi].IValue
        // fmt.Printf("vget - ans from : %+v\n",(*ident)[vi])
        if (*ident)[vi].declared {
            // vlock.RUnlock()
            return v,true
        }

    }
    // vlock.RUnlock()
    // fmt.Printf("vget - no answer (nil)\n")
    return nil, false
}


func vgeti(fs uint32, ident *[]Variable, vi uint16) (v interface{}, s bool) {
    v=(*ident)[vi].IValue
    if !(*ident)[vi].declared {
        v=nil
    } else {
        s=true
    }
    return v, s
}


/*
func getvtype(fs uint32, ident *[]Variable, name string) (reflect.Type, bool) {
    if vi, ok := VarLookup(fs, name); ok {
        vlock.RLock()
        defer vlock.RUnlock()
        return reflect.TypeOf((*ident)[vi].IValue) , true
    }
    return nil, false
}
*/

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
func interpolate(fs uint32, ident *[]Variable, s string) (string) {

    if !interpolation {
        return s
    }

    // should finish sooner if no curly open brace in string.
    if str.IndexByte(s, '{') == -1 {
        return s
    }

    orig:=s
    r := regexp.MustCompile(`{([^{}]*)}`)

    // @note: may still need the lock for vmap access

    for {
        os:=s

        // generate list of matches of {...} in s
        matches := r.FindAllStringSubmatch(s,-1)

        for _, v := range matches {

            //  lookup in vmap
            if k,there:=vmap[fs][v[1]]; there {

                if (*ident)[k].declared && (*ident)[k].IValue != nil {

                    switch (*ident)[k].IValue.(type) {
                    case int:
                        s = str.Replace(s, "{"+(*ident)[k].IName+"}", strconv.FormatInt(int64((*ident)[k].IValue.(int)), 10),-1)
                    case float64:
                        s = str.Replace(s, "{"+(*ident)[k].IName+"}", strconv.FormatFloat((*ident)[k].IValue.(float64),'g',-1,64),-1)
                    case bool:
                        s = str.Replace(s, "{"+(*ident)[k].IName+"}", strconv.FormatBool((*ident)[k].IValue.(bool)),-1)
                    case string:
                        s = str.Replace(s, "{"+(*ident)[k].IName+"}", (*ident)[k].IValue.(string),-1)
                    case uint:
                        s = str.Replace(s, "{"+(*ident)[k].IName+"}", strconv.FormatUint(uint64((*ident)[k].IValue.(uint)), 10),-1)
                    case []uint, []float64, []int, []bool, []interface{}, []string:
                        s = str.Replace(s, "{"+(*ident)[k].IName+"}", sf("%v",(*ident)[k].IValue),-1)
                    case interface{}:
                        s = str.Replace(s, "{"+(*ident)[k].IName+"}", sf("%v",(*ident)[k].IValue),-1)
                    default:
                        s = str.Replace(s, "{"+(*ident)[k].IName+"}", sf("!%T!%v",(*ident)[k].IValue,(*ident)[k].IValue),-1)

                    }
                }
            }
        }

        if os==s { break }
    }

    // if nothing was replaced, check if evaluation possible, then it's time to leave this infernal place
    var modified bool

    redo:=true
    calllock.Lock()
    ofs:=interparse.fs
    oident:=interparse.ident
    interparse.fs=fs
    interparse.ident=ident
    calllock.Unlock()

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

    calllock.Lock()
    interparse.ident=oident
    interparse.fs=ofs
    calllock.Unlock()

    if s=="<nil>" { s=orig }
    // pf("\n\ns=%s\n",s)

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
        if t.tokPos != -1 {
            p = t.tokPos
        }
        if t.carton.tokType==Identifier {
            loc, _ := VarLookup(fs, t.carton.tokText)
            t.carton.offset=loc
        }
        toks = append(toks, t.carton)
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

func (p *leparser) wrappedEval(lfs uint32, lident *[]Variable, fs uint32, rident *[]Variable, tks []Token) (expr ExpressionCarton) {

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
        expr.assignPos=-1
    } else {
        expr.assign=true
        expr.assignPos=eqPos
        // before eval, rewrite lhs token offsets to their lhs equivalent
        if !standardAssign {
            if lfs!=fs {
                if newEval[0].tokType==Identifier {
                    if off,found:=vmap[lfs][newEval[0].tokText]; found {
                        if (*lident)[off].declared {
                            newEval[0].offset=off
                        } else {
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

            /*
            // lots of crap here since moving to separate ident[] per func :)

            cls()
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
        // pf("-- entering doAssign (lfs->%d,rfs->%d) with tokens : %+v\n",lfs,fs,tks)
        // pf("-- entering doAssign (lfs->%d,rfs->%d) with value  : %+v\n",lfs,fs,expr.result)
        p.doAssign(lfs,lident,fs,rident,tks,&expr,eqPos)
        // pf("-- exited   doAssign (lfs->%d,rfs->%d) with idents of %+v\n",lfs,fs,lident)
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

func (p *leparser) doAssign(lfs uint32, lident *[]Variable, rfs uint32, rident *[]Variable, tks []Token,expr *ExpressionCarton,eqPos int) {

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
        var scrap [7]Token
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
            var vi uint16
            var there bool
            if lfs!=rfs {
                if vi,there=VarLookup(lfs,assignee[0].tokText); !there {
                    vi=vset(lfs,lident,assignee[0].tokText,nil)
                }
            } else {
                vi=assignee[0].offset
            }
            // pf("-- normal assignment to (ifs:%d) %s (offset:%d) of %+v [%T]\n", lfs, assignee[0].tokText, vi, results[assno],results[assno])
            vseti(lfs, lident, assignee[0].tokText, vi, results[assno])
            // pf("--  content of fs %d vi %d -> [%T] %#v\n",lfs,vi,lident[vi].IValue,lident[vi])
            /////////////////////////////////////////////////////////////////////////////

        case len(assignee)==2:
            // currently only *p pointer assignment, but check...
            /*
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
            */

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
            var vi uint16

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
                var there bool
                aryName := assignee[0].tokText
                if lfs!=rfs {
                    if vi,there=VarLookup(lfs,assignee[0].tokText); !there {
                        vi=vset(lfs,lident,assignee[0].tokText,nil)
                    }
                } else {
                    vi=assignee[0].offset
                }
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

                tempStore ,found = vgetElementi(lfs,lident,aryName, vi, eleName)

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
                                    vsetElementi(lfs,lident,aryName,vi,element.(int),tmp.Interface())
                                case string:
                                    vsetElementi(lfs,lident,aryName,vi,element.(string),tmp.Interface())
                                default:
                                    vsetElementi(lfs,lident,aryName,vi,element.(string),tmp.Interface())
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


            var there bool
            if lfs!=rfs {
                if vi,there=VarLookup(lfs,assignee[0].tokText); !there {
                    vi=vset(lfs,lident,assignee[0].tokText,nil)
                }
            } else {
                vi=assignee[0].offset
            }

            switch element.(type) {
            case string:
                element = interpolate(rfs,rident,element.(string))
                vsetElementi(lfs, lident, assignee[0].tokText, vi, element.(string), results[assno])
            case int:
                if element.(int)<0 {
                    pf("negative element index!! (%s[%v])\n",assignee[0].tokText,element)
                    expr.evalError=true
                    expr.errVal=err
                }
                vsetElementi(lfs, lident, assignee[0].tokText, vi, element.(int), results[assno])
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

                ts,found=vgeti(lfs,lident,lhs_o)

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


