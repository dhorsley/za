package main

import (
    "fmt"
    "io/ioutil"
    "reflect"
    "strconv"
    "math"
    "math/big"
    "net/http"
    "sync"
//    "github.com/davecgh/go-spew/spew"
//    "sync/atomic"
    "path/filepath"
    str "strings"
    "unsafe"
    "regexp"
    "crypto/md5"
)


func (p *leparser) reserved(token Token) (any) {
    panic(fmt.Errorf("statement names cannot be used as identifiers ([%s] %v)",tokNames[token.tokType],token.tokText))
}

func (p *leparser) Eval(fs uint32, toks []Token) (any,error,bool) {

    l:=len(toks)

    // short circuit pure numeric literals and const names
    if l==1 {
        if toks[0].tokType==NumericLiteral { return toks[0].tokVal,nil,false }
        switch toks[0].subtype {
        case subtypeConst:
            return toks[0].tokVal,nil,false
        }
    }

    p.prectable=default_prectable
    p.fs     = fs
    p.tokens = toks
    p.len    = int16(l)
    p.pos    = -1

    return p.dparse(0,false)
}


type leparser struct {
    tokens      []Token                 // the thing getting evaluated
    ident       *[]Variable             // where are the local variables at?
    prev        Token                   // bodge for post-fix operations
    preprev     Token                   //   and the same for assignment
    fs          uint32                  // working function space
    mident      uint32                  // fs of main() (1 or 2 depending on interactive mode)
                                        // @note: mident is necessary to say whether globals are stored under fs #1 or #2
    len         int16                   // assigned length to save calling len() during parsing
    line        int16                   // shadows lexer source line
    pc          int16                   // shadows program counter (pc)
    pos         int16                   // distance through parse
    prectable   [END_STATEMENTS]int8    // precedence lookup table
    namespace   string                  // optional namespace attached to next 2 tokens
    namespacing bool                    // pending namespace completion?
    namespace_pos int16                 // token position of namespace start

    try_fault   bool                    // if a try operator was encountered and it's expression returned nil
    try_err     error                   // recorded error
    try_pos     int                     // program statement counter position of error
    try_line    int                     // source line in base function of error
    try_info    string                  // error string
    try_type    string                  // fix type to apply
    try_type_override bool              // allows dparse to process rhs of tern_if operator during a try
    std_call    bool                    // if a call to stdlib has been made
    std_faulted bool                    // and if it faulted.

    in_fix      bool                    // currently in a fix block
    resume_pos  int16                   // statement position the fix was triggered from

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


func (p *leparser) dparse(prec int8,skip bool) (left any,err error,try_fault bool) {

    // @note: skip allows expression to be parsed without error in order to skip 
    // past redundant phrases. not ideal, but okay for now.

    // pf("\ndparse query with fs #%d : spos %v : %+v\n",p.fs,p.pos,p.tokens)

    if skip {
        brace_level:=0
        skiploop1:
        for {
            token := p.peek()
            switch token.tokType {
            case LParen:
                brace_level+=1
            case RParen:
                if brace_level==0 {
                    // pf("[skip breaking on token %v] ",tokNames[token.tokType])
                    break skiploop1
                }
                brace_level-=1
            case O_Comma,SYM_COLON,EOF:
                // pf("[skip breaking on token %v] ",tokNames[token.tokType])
                break skiploop1
            }
            // pf("[skip token %+v] ",token)
            p.next()
        }
        return left,err,try_fault
    }


    // inlined next() manually:
    if p.pos>0 { p.preprev=p.prev }
    if p.pos>-1 { p.prev=p.tokens[p.pos] }
    p.pos+=1

    ct:=&p.tokens[p.pos]

    // unaries
    switch (*ct).tokType {
    case O_Comma,SYM_COLON,EOF:
        left=nil
    case RParen, RightSBrace:
        p.next()
        left=nil
    case NumericLiteral:
        left=(*ct).tokVal
    case StringLiteral:
        left=interpolate(p.namespace,p.fs,p.ident,(*ct).tokText)
    case Identifier:
        left=p.identifier(ct)
    case O_Sqr, O_Sqrt,O_InFile:
        left=p.unary(ct)
    case SYM_Not:
	    right,err,_ := p.dparse(24,false) // don't bind negate as tightly
        if err!=nil { panic(err) }
		left=unaryNegate(right)
    case O_Pb,O_Pa,O_Pn,O_Pe,O_Pp:
        right,err,_ := p.dparse(10,false) // allow strings to accumulate to the right
        if err!=nil { panic(err) }
        left=p.unaryPathOp(right,(*ct).tokType)
    case O_Slc,O_Suc,O_Sst,O_Slt,O_Srt:
        left=p.unary(ct)
    case O_Assign, O_Plus, O_Minus:      // prec variable
        left=p.unary(ct)
    case LParen:
        left=p.grouping(ct)
    case SYM_PP, SYM_MM:
        left=p.preIncDec(ct)
    case LeftSBrace:
        left=p.array_concat(ct)
    case O_Ref:
        left=p.reference(false)
    case SYM_BOR:
        left=p.command()
    case Block: // ${
        _,left,_,_=p.blockCommand(ct.tokText,false)
    case AsyncBlock: // &{
        _,_,_,left=p.blockCommand(ct.tokText,true)
    case ResultBlock: // {
        _,_,left,_=p.blockCommand(ct.tokText,false)
    }

    // binaries

    binloop1:
    for {

        if !p.namespacing && prec >= p.prectable[p.peek().tokType] { break }

        token := p.next()
        // pf("binloop nt -> %v at pos %d\n",token.tokText,p.pos)

        if p.namespacing {
            // pf("  (eval) namespacing, next token %v at %d\n",token.tokText,p.pos)
            if p.pos==p.namespace_pos+1 {
                p.namespacing=false
                left=p.preprev.tokText+"::"+token.tokText
                // pf("  (eval) completed namespace -> %#v at pos %d\n",left,p.pos)
                continue
            }
        }

        switch token.tokType {
        case EOF:
            break binloop1
        case SYM_PP,SYM_MM:
            left = p.postIncDec(token)
            continue
        case LeftSBrace:
            left = p.accessArray(left,token)
            continue
        case SYM_DoubleColon:
            if ! p.namespacing {
                p.namespacing=true
                p.namespace_pos=p.pos
            } else {
                // pf(":: namespacing fault on token '%s' npos %d cpos %d?\nall toks -> %#v\n",token.tokText,p.namespace_pos,p.pos,p.tokens)
                p.namespacing=false
                p.namespace_pos=-1
                break binloop1
            }
            continue
        case SYM_DOT:
            p.std_faulted=false
            // pf("toks->%+v\n",p.tokens)
            // pf("sd prepos  -> %d\n",p.pos)
            left,_ = p.accessFieldOrFunc(left,p.next().tokText)
            // pf("sd postpos -> %d\n",p.pos)
            continue
        case C_Is:
            left = p.kind_compare(left)
            continue
        case LParen:
            switch left.(type) {
            case string:
                left,_ = p.buildStructOrFunction(left,token)
                continue
            }
        }

        var right any
        right,err,_ = p.dparse(p.prectable[token.tokType] + 1,false)

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

        case O_Query:                       // ternary and try op
            /*
            pf("(o_query) right is %v\n",right)
            pf("(o_query) tokens -> %+v\n",p.tokens)
            */
            left=p.tern_if(left,right)

        case SYM_EQ:
            left = deepEqual(left,right)
        case SYM_NE:
            left = !deepEqual(left,right)
        case SYM_LT:
            left = compare(left,right,SYM_LT)
        case SYM_GT:
            left = compare(left,right,SYM_GT)
        case SYM_LE:
            left = compare(left,right,SYM_LE)
        case SYM_GE:
            left = compare(left,right,SYM_GE)

        case SYM_LOR,C_Or: // OR and AND here for non-bool types

            switch left.(type) {
            case string:
                switch right.(type) {
                case string:
                    if left.(string)=="" {
                        return right.(string),nil,false
                    }
                    return left.(string),nil,false
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

    /*
    if err!=nil || left==nil {
      pf("[#2]dparse result: %+v[#-]\n",left)
      pf("[#2]dparse error : %#v[#-]\n",err)
    }
    */

	return left,err,p.try_fault
}


type rule struct {
	nud func(token Token) (any)
	led func(left any, token Token) (any)
	prec int8
}


func (p *leparser) ignore(token Token) any {
    p.next()
    return nil
}


func (p *leparser) list_filter(left any,right any) any {

    switch right.(type) {
    case string:
    default:
        panic(fmt.Errorf("invalid condition string (%+v) in filter",right))
    }

    var reduceparser *leparser
    reduceparser=&leparser{}
    reduceparser.ident=p.ident
    reduceparser.fs=p.fs

    switch left.(type) {

    case []dirent:

        // find # refs
        var fields []string
        var fieldpos []int

        cond:=right.(string)+" "
        for e:=0; e<len(cond)-1; e+=1 {
            if cond[e]=='#' && cond[e+1]=='.' {
                for f:=e+2; f<len(cond); f+=1 {
                    if str.IndexByte(identifier_set,cond[f])==-1 {
                        fields=append(fields,cond[e+2:f])
                        e=f
                        fieldpos=append(fieldpos,f-1)
                        break
                    }
                }
            }
        }

        // filter
        var new_list []dirent
        for e:=0; e<len(left.([]dirent)); e+=1 {
            nm:=s2m(left.([]dirent)[e])
            var new_right str.Builder
            fnum:=0
            for f:=0; f<len(cond); f+=1 {
                switch cond[f] {
                case '#':
                    new_right.WriteString(sf("%#v",nm[fields[fnum]]))
                    f=fieldpos[fnum]
                    fnum+=1
                default:
                    new_right.WriteByte(cond[f])
                }
            }
            val,err:=ev(reduceparser,p.fs,new_right.String())
            if err!=nil { panic(err) }
            switch val.(type) {
            case bool:
                if val.(bool) { new_list=append(new_list,left.([]dirent)[e]) }
            default:
                panic(fmt.Errorf("invalid expression (non-boolean?) (%s) in filter",new_right.String()))
            }
        }
        return new_list

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

    case map[string]any:
        var new_map = make(map[string]any)
        for k,v:=range left.(map[string]any) {
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

    case []any:
        var new_list []any
        for e:=0; e<len(left.([]any)); e+=1 {
            var new_right string
            switch v:=left.([]any)[e].(type) {
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
            case nil:
                new_right=str.Replace(right.(string),"#","nil",-1)
            default:
                new_right=str.Replace(right.(string),"#",sf("%#v",v),-1)
            }
            val,err:=ev(reduceparser,p.fs,new_right)
            if err!=nil { panic(err) }
            switch val.(type) {
            case bool:
                if val.(bool) { new_list=append(new_list,left.([]any)[e]) }
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

func (p *leparser) file_out(left any,right any) any {

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

func (p *leparser) list_map(left any,right any) any {

    switch right.(type) {
    case string:
    default:
        panic(fmt.Errorf("invalid string (%+v) in map",right))
    }

    var reduceparser *leparser
    reduceparser=&leparser{}
    reduceparser.ident=p.ident
    reduceparser.fs=p.fs

    // to generalise the []dirent case to all []structs we need:
    //  1. something to detect field access in the condition ahead of the switch/case.
    //  2. to move the []dirent case block out to its own generic function so that 
    //      the s2m() and outer for loop length check work.
    //  alternatively, we could combine the []any (no field access) and []dirent/struct cases
    //    and check for field access from within it.. that way the switch could instead assign
    //    the case left.(type) below to a scoped variable instead and we could possibly remove
    //    the type assertions that way. (maybe - need to check)

    switch left.(type) {

    case []dirent:

        // find # refs
        var fields []string
        var fieldpos []int

        cond:=right.(string)+" "
        for e:=0; e<len(cond)-1; e+=1 {
            if cond[e]=='#' && cond[e+1]=='.' {
                for f:=e+2; f<len(cond); f+=1 {
                    if str.IndexByte(identifier_set,cond[f])==-1 {
                        fields=append(fields,cond[e+2:f])
                        e=f
                        fieldpos=append(fieldpos,f-1)
                        break
                    }
                }
            }
        }

        var new_list []interface{}
        for e:=0; e<len(left.([]dirent)); e+=1 {
            nm:=s2m(left.([]dirent)[e])
            var new_right str.Builder
            fnum:=0
            for f:=0; f<len(cond); f+=1 {
                switch cond[f] {
                case '#':
                    new_right.WriteString(sf("%#v",nm[fields[fnum]]))
                    f=fieldpos[fnum]
                    fnum+=1
                default:
                    new_right.WriteByte(cond[f])
                }
            }
            val,err:=ev(reduceparser,p.fs,new_right.String())
            if err!=nil { panic(err) }
            new_list=append(new_list,val)
        }
        return new_list

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

    case string:
        var new_list []string
        for e:=0; e<len(left.(string)); e+=1 {
            new_right:=str.Replace(right.(string),"#",`"`+string(left.(string)[e])+`"`,-1)
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

    case []any:
        var new_list []any
        for e:=0; e<len(left.([]any)); e+=1 {
            var new_right string
            switch v:=left.([]any)[e].(type) {
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

    case map[string]any:
        var new_map = make(map[string]any)
        for k,v:=range left.(map[string]any) {
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


func (p *leparser) rcompare (left any,right any,insensitive bool, multi bool) any {

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

    if pre,found:=ifCompileCache[insenStr+right.(string)];!found {
        var ptr_re *regexp.Regexp
        var err error
        ptr_re,err = regexp.Compile(insenStr+right.(string))
        if err!=nil {
            panic(fmt.Errorf("supplied regex is invalid: %s",right.(string)))
        }
        re=*ptr_re
        ifCompileCache[insenStr+right.(string)]=*ptr_re
    } else {
        re = pre
    }

    if multi { return re.FindAllString(left.(string),-1) }

	return re.MatchString(left.(string))
}


func (p *leparser) accessArray(left any,right Token) (any) {

    var start,end any
    var hasStart,hasEnd,hasRange bool
    var sendNil bool

    switch left:=left.(type) {

    // size is checked in slice()
    case []bool:
    case []string:
    case []int:
    case []uint:
    case []float64:
    case []dirent:
    case []alloc_info:
    case string:
    case []*big.Int:
    case []*big.Float:
    case [][]int:
    case []any:

    case map[string]any,map[string]alloc_info,map[string]string,map[string]int:

        // check for key
        var mkey string
        if right.tokType==SYM_DOT {
            t:=p.next()
            mkey=t.tokText
        } else {
            if p.peek().tokType!=RightSBrace {
                dp,err,_:=p.dparse(0,false)
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

    case uint,int,float64,uint8,uint64,int64,*big.Int,*big.Float:
        // just allow these through. handled as a clamp operation later.
        // but do flag to allow missing start/end
        hasRange=true
    default:
        sendNil=true
    }

    if p.peek().tokType!=RightSBrace {

        // check for start of range
        if p.peek().tokType!=SYM_COLON {
            // pf("(aa)     ntok -> %+v\n",tokNames[p.peek().tokType])
            dp,err,_:=p.dparse(0,false)
            // pf("(aa) start dp -> %+v\n",dp)
            // pf("(aa)   err dp -> %+v\n",err)
            if err!=nil {
                panic(fmt.Errorf("array range start could not be evaluated"))
            }
            switch dp.(type) {
            case int,float64,*big.Int,*big.Float:
                start=dp
                hasStart=true
            }
        }

        // check for end of range
        if p.peek().tokType==SYM_COLON {
            p.next() // swallow colon
            hasRange=true
            if p.peek().tokType!=RightSBrace {
                dp,err,_:=p.dparse(0,false)
                if err!=nil {
                    panic(fmt.Errorf("array range end could not be evaluated"))
                }
                switch dp.(type) {
                case int,float64,*big.Int,*big.Float:
                    end=dp
                    hasEnd=true
                }
            }
        }

        // pf("[range] next token %v\n",tokNames[p.peek().tokType])
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


func (p *leparser) buildStructOrFunction(left any,right Token) (any,bool) {

    name:=left.(string)
    isStruct:=false

    // filter for enabling struct type names here:
    structvalues:=[]any{}
    found:=false
    if structvalues,found=structmaps[name];found || name=="anon" {
        isStruct=true
    }
    // end-struct-filter

    if !isStruct {
        // filter for functions here
        var isFunc bool

        // check if exists in user defined function space
        if _, isFunc = stdlib[name]; !isFunc {
            if !str.Contains(name,"::") {
                useName:=p.namespace
                if p.namespace=="" {
                    useName="main"
                }
                name=useName+"::"+name
            }
            isFunc = fnlookup.lmexists(name)
        }

        if !isFunc {
            panic(fmt.Errorf("'%v' (l:%#v,r:%#v) is not a function",name,left,right))
        }
    }

    iargs:=[]any{}
    arg_names:=[]string{}
    argpos:=1

    if p.peek().tokType!=RParen {
        for {
            switch p.peek().tokType {
            case SYM_DOT:
                p.next() // move-to-dot
                p.next() // skip-to-name-from-dot
                arg_names=append(arg_names,p.tokens[p.pos].tokText) // add name field
            case RParen,O_Comma:
                // missing/blank arg in list
                panic(fmt.Errorf("missing argument #%d",argpos))
            }
            dp,err,_:=p.dparse(0,false)
            if err!=nil {
                return nil,true
            }
            iargs=append(iargs,dp)
            if p.peek().tokType!=O_Comma {
                break
            }
            p.next()
            argpos+=1
        }
    }

    if p.peek().tokType==RParen {
        p.next() // consume rparen
    }


    // build struct literals
    if isStruct {

        // @note: this typemap set up is also in actor.go 
        // it needs reworking with even more urgency now!
        // but, leaving it here while testing:

        var tb bool
        var tu8 uint8
        var tu32 uint32
        var tu64 uint64
        var tu uint
        var ti int
        var tf64 float64
        var ts string
        var tmixed interface{}
        var tbi *big.Int
        var tbf *big.Float

        var stb     []bool
        var stu     []uint
        var stu8    []uint8
        var stu32   []uint32
        var stu64   []uint64
        var sti     []int
        var stf64   []float64
        var sts     []string
        var stbi    []*big.Int
        var stbf    []*big.Float
        var stmixed []any

        // instantiate fields with an empty expected type:
        typemap:=make(map[string]reflect.Type)
        typemap["bool"]     = reflect.TypeOf(tb)
        typemap["uint"]     = reflect.TypeOf(tu)
        typemap["uint8"]    = reflect.TypeOf(tu8)
        typemap["uint32"]   = reflect.TypeOf(tu32)
        typemap["uint64"]   = reflect.TypeOf(tu64)
        typemap["ulong"]    = reflect.TypeOf(tu32)
        typemap["uxlong"]   = reflect.TypeOf(tu64)
        typemap["byte"]     = reflect.TypeOf(tu8)
        typemap["int"]      = reflect.TypeOf(ti)
        typemap["float"]    = reflect.TypeOf(tf64)
        typemap["bigi"]     = reflect.TypeOf(tbi)
        typemap["bigf"]     = reflect.TypeOf(tbf)
        typemap["string"]   = reflect.TypeOf(ts)
        typemap["mixed"]    = reflect.TypeOf(tmixed)
        typemap["any"]      = reflect.TypeOf(tmixed)
        typemap["[]bool"]   = reflect.TypeOf(stb)
        typemap["[]uint"]   = reflect.TypeOf(stu)
        typemap["[]uint8"]  = reflect.TypeOf(stu8)
        typemap["[]byte"]   = reflect.TypeOf(stu8)
        typemap["[]int"]    = reflect.TypeOf(sti)
        typemap["[]uint32"] = reflect.TypeOf(stu32)
        typemap["[]uint64"] = reflect.TypeOf(stu64)
        typemap["[]float"]  = reflect.TypeOf(stf64)
        typemap["[]string"] = reflect.TypeOf(sts)
        typemap["[]bigi"]   = reflect.TypeOf(stbi)
        typemap["[]bigf"]   = reflect.TypeOf(stbf)
        typemap["[]mixed"]  = reflect.TypeOf(stmixed)
        typemap["[]any"]    = reflect.TypeOf(stmixed)
        typemap["[]"]       = reflect.TypeOf(stmixed)
        typemap["map"]      = nil

        // end-of-typemap-dogshit

        var t Variable

        if len(arg_names)>0 {
            // named field handling:
            //  struct_name(.name value,...,.name value)
            if len(arg_names)==len(iargs) {
                // all dotted, named fields?
                /*
                for n:=0; n<len(arg_names); n+=1 {
                    pf("s-field, loop name  #%d : %+v\n",n,arg_names[n])
                    pf("s-field, loop value #%d : %+v\n",n,iargs[n])
                }
                */
            } else {
                panic(fmt.Errorf("length mismatch of argument names [%d] to struct fields [%d]",len(arg_names),len(iargs)))
                // finish(false,ERR_EVAL)
                // return nil,true
            }
        }

        if name=="anon" {
            for n:=0; n<len(arg_names); n+=1 {
                structvalues=append(structvalues,arg_names[n])
                t:=reflect.TypeOf(iargs[n])
                typeFound:=false
                for vk,vt:=range typemap {
                    if vt==t {
                        structvalues=append(structvalues,vk)
                        typeFound=true
                        break
                    }
                }
                if !typeFound {
                    panic(fmt.Errorf("unknown type in struct(anon) field %s [%v]",arg_names[n],t))
                    // finish(false,ERR_EVAL)
                    // return nil,true
                }
                structvalues=append(structvalues,true)
                structvalues=append(structvalues,iargs[n])
            }
        } else {
            switch len(iargs) {
            case 0:
                // leave 0 args as unhandle, for a default constructor here
            case len(structvalues)/4:
                // work through iargs, populating struct fields here
                // structvalues: [0] name [1] type [2] boolhasdefault [3] default_value

                // confirm types match named arguments:
                if len(arg_names)>0 {
                    for i:=range iargs {
                        nameMatched:=false
                        for j:=0; j<len(structvalues); j+=4 {
                            if structvalues[j].(string)==arg_names[i] {
                                if typemap[structvalues[j+1].(string)] != reflect.TypeOf(iargs[i]) {
                                    panic(fmt.Errorf("type mismatch in named field '%s', should be %v",arg_names[i],structvalues[j+1]))
                                }
                                nameMatched=true
                                break // found a positive match, move on to next argument
                            }
                        }
                        if !nameMatched {
                            panic(fmt.Errorf("provided argument name '%s' not found in struct '%s'",arg_names[i],name))
                        }
                    }
                    // if we reach here, then all types matched the provided values, hopefully!
                }

                n:=0
                for i:=3; i<len(structvalues); i+=4 {
                    structvalues[i-1]=true
                    structvalues[i]=iargs[n]
                    n+=1
                }
            default:
                // error
                panic(fmt.Errorf("invalid parameter list count (%d) in struct(%s) init",len(iargs),name))
                // finish(false,ERR_EVAL)
                // return nil,true
            }
        }

        err:=fillStruct(&t,structvalues,typemap,false,arg_names)
        if err!=nil {
            panic(err.Error())
            // finish(false,ERR_EVAL)
            // return nil,true
        }

        return t.IValue,false

    }

    // if not a struct() then treat as a normal func() instead: 

    if len(arg_names)>0 { // check that arg_names tally with functionArgs list
        var ifn uint32
        var present bool
        if ifn, present = fnlookup.lmget(name); !present {
            panic(fmt.Errorf("could not find function named '%s'",name))
            // finish(false,ERR_EVAL)
            // return nil,true
        }
        farglock.RLock()
        falist:=functionArgs[ifn].args
        farglock.RUnlock()
        if len(arg_names)==len(falist) {
            for _,an:=range arg_names {
                found:=false
                for _,fa:=range falist {
                    if an==fa {
                        found=true
                        break
                    }
                }
                if !found {
                    panic(fmt.Errorf("argument '%s' not found in definition for '%s'",an,name))
                    // finish(false,ERR_EVAL)
                    // return nil,true
                }
            }
        } else {
            panic(fmt.Errorf("bad argument name count [%d] for '%s' [needs %d]",len(arg_names),name,len(falist)))
            // finish(false,ERR_EVAL)
            // return nil,true
        }
    }

    return p.callFunctionExt(p.fs,p.ident,name,self_s{},arg_names,iargs)

}


// mut is currently unused and may remain so.
func (p *leparser) reference(mut bool) string {
    vartok:=p.next()
    bin:=vartok.bindpos
    if ! (*p.ident)[bin].declared {
        vset(&vartok,p.fs,p.ident,vartok.tokText,nil)
    }
    return vartok.tokText
}

func (p *leparser) unaryPathOp(right any,op int64) string {
    switch right.(type) {
    case string:
        switch op {
        case O_Pb: // base path
            return filepath.Base(right.(string))
        case O_Pa: // abs path
            fp,e:=filepath.Abs(right.(string))
            if e!=nil { return "" }
            return fp
        case O_Pn: // base - no ext
            fp:=filepath.Base(right.(string))
            fe:=filepath.Ext(fp)
            if fe=="" { return fp }
            return fp[:len(fp)-len(fe)]
        case O_Pe: // base - only ext
            return filepath.Ext(right.(string))[1:]
        case O_Pp: // parent path
            fp,e:=filepath.Abs(right.(string))
            if e!=nil { return "" }
            return fp[:str.LastIndex(fp,"/")]
        default:
            panic(fmt.Errorf("unknown unary path operator!")) // shouldn't see this!
        }
    default:
        panic(fmt.Errorf("invalid type in unary path operator"))
    }
}


// none of this pointer stuff is live. just tinkering here. move along!
func (p *leparser) unaryPointerOp(right any,op int64) any {
    bin:=bind_int(p.fs,right.(string))
    switch op {
    case SYM_Caret:
        switch right.(type) {
        case string:
            if (*p.ident)[bin].declared {
                return &((*p.ident)[bin])
            }
        }
    case O_Multiply:
        return (*right.(*Variable)).IValue
    }
    return nil
}

func (p *leparser) unaryStringOp(right any,op int64) string {
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
}

func (p *leparser) unary(token *Token) (any) {

    switch token.tokType {
    case O_InFile:
	    right,err,_ := p.dparse(70,false) // higher than dot op
        if err!=nil { panic(err) }
        return unaryFileInput(right)
    }

	right,err,_ := p.dparse(38,false) // between grouping and other ops
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

func unOpSqr(n any) any {
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
}

func unOpSqrt(n any) any {
    switch n:=n.(type) {
    case int:
        return math.Sqrt(float64(n))
    case uint:
        return math.Sqrt(float64(n))
    case float64:
        return math.Sqrt(n)
    case *big.Int:
        var tmp big.Int
        return tmp.Sqrt(n)
    case *big.Float:
        var tmp big.Float
        return tmp.Sqrt(n)
    default:
        panic(fmt.Errorf("sqrt does not support type '%T'",n))
    }
    // unreachable: // return nil
}

func (p *leparser) tern_if(left any,tv any) (any) {
    // pf("(tern_if) tv : %+v\n",tv)
    // expr '?' string
    if p.peek().tokType != SYM_COLON {
        switch left.(type) {
        case nil:
            // set try fail state
            p.try_fault=true
            p.try_pos=int(p.pc)
            p.try_info=sf("%v",p.try_err)
        }
        switch tv.(type) {
        case string:
            // set fix type string
            p.try_type=tv.(string)
            p.try_type_override=false
            // pf("tern string type : %s\n",p.try_type)
        default:
            p.try_type=""
        }
        return left
    }
    // expr '?' tv ':' fv
    switch left.(type) {
    case bool:
    default:
        panic(fmt.Errorf("not a boolean on left of ternary"))
    }
    if p.peek().tokType==SYM_COLON {
        p.next()
    } else {
        panic(fmt.Errorf("missing colon in ternary"))
    }


    switch left.(type) {
    case bool:
        if left.(bool) {
            p.dparse(0,true)
            return tv
        }
    }
    /*
    pf("(tern_if) reached false expression. parser state->\n")
    spew.Dump(p.tokens)
    pf("parse position : ")
    spew.Dump(p.pos)
    pf("\n\n")
    */
    fv,err,_:=p.dparse(0,false)
    if err!=nil {
        panic(fmt.Errorf("malformed false expression in ternary"))
    }
    return fv
}

func (p *leparser) array_concat(tok *Token) (any) {

	// right-associative

    ary:=[]any{}

    if p.peek().tokType!=RightSBrace {
        for {
            dp,err,_:=p.dparse(0,false)
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

func (p *leparser) preIncDec(token *Token) any {

    // get direction
    ampl:=1
    switch token.tokType {
    case SYM_MM:
        ampl=-1
    }

    // move parser position to varname 
    vartok:=p.next()

    // exists?
    var val any

    bin:=vartok.bindpos

    activeFS:=p.fs
    if ! (*p.ident)[bin].declared {
        gbin:=bind_int(p.mident,vartok.tokText)
        if mident[gbin].declared {
            val,_=vget(nil,p.mident,&mident,vartok.tokText)
            activeFS=p.mident
        } else {
            panic(fmt.Errorf("invalid variable name in pre-inc/dec '%s'",vartok.tokText))
        }
    } else {
        val,_=vget(&vartok,p.fs,p.ident,vartok.tokText)
    }

    // act according to var type
    var n any
    switch v:=val.(type) {
    case int:
        n=v+ampl
    case uint:
        n=v+uint(ampl)
    case float64:
        n=v+float64(ampl)
    case *big.Int:
        n=v.Add(v,GetAsBigInt(ampl))
    case *big.Float:
        n=v.Add(v,GetAsBigFloat(ampl))
    default:
        p.report(-1,sf("pre-inc/dec not supported on type '%T' (%s)",val,val))
        finish(false,ERR_EVAL)
        return nil
    }
    if activeFS==p.mident {
        vset(&vartok,p.mident,&mident,vartok.tokText,n)
    } else {
        vset(&vartok,p.fs,p.ident,vartok.tokText,n)
    }
    return n

}

func (p *leparser) postIncDec(token Token) any {

    // get direction
    ampl:=1
    switch token.tokType {
    case SYM_MM:
        ampl=-1
    }

    // get var from parser context
    vartok:=p.prev

    // exists?
    var val any

    bin:=vartok.bindpos
    activeFS:=p.fs

    var mloc uint32
    if interactive {
        mloc=1
    } else {
        mloc=2
    }

    activePtr:=p.ident

    if strcmp((*p.ident)[bin].IName,vartok.tokText) {
        if ! (*p.ident)[bin].declared {
            gbin:=bind_int(mloc,vartok.tokText)
            if mident[gbin].declared {
                val,_=vget(&token,mloc,&mident,vartok.tokText)
                activeFS=mloc
                activePtr=&mident
            } else {
                panic(fmt.Errorf("invalid variable name in post-inc/dec '%s'",vartok.tokText))
            }
        } else {
                val,_=vget(&vartok,p.fs,p.ident,vartok.tokText)
        }
    } else {
        panic(fmt.Errorf("'%s' not a local variable.",vartok.tokText))
    }

    // act according to var type
    switch v:=val.(type) {
    case int:
        vset(&vartok,activeFS,activePtr,vartok.tokText,v+ampl)
    case uint:
        vset(&vartok,activeFS,activePtr,vartok.tokText,v+uint(ampl))
    case float64:
        vset(&vartok,activeFS,activePtr,vartok.tokText,v+float64(ampl))
    case *big.Int:
        n:=v.Add(v,GetAsBigInt(ampl))
        vset(&vartok,activeFS,activePtr,vartok.tokText,n)
    case *big.Float:
        n:=v.Add(v,GetAsBigFloat(ampl))
        vset(&vartok,activeFS,activePtr,vartok.tokText,n)
    default:
        panic(fmt.Errorf("post-inc/dec not supported on type '%T' (%s)",val,val))
    }
    return val
}


func (p *leparser) grouping(tok *Token) (any) {

	// right-associative
    val,err,_:=p.dparse(0,false)
    if err!=nil { panic(err) }
    p.next() // consume RParen
    return val

}

func (p *leparser) kind_compare(left any) bool {
    typeTok:=p.next()
    return ev_kind_compare(left,typeTok)
}

func (p *leparser) number(token Token) (num any) {
    var err error

    // test code:
    num=token.tokVal

    if num==nil {
        panic(err)
    }
	return num
}

type cmd_result struct{out string; err string; code int; okay bool}
type bg_result  struct{name string;handle chan any}

func (p *leparser) blockCommand(cmd string, async bool) (state bool, resstr string, result cmd_result, bgresult bg_result) {

    cmd=sparkle(interpolate(p.namespace,p.fs,p.ident,cmd))

    if async {

        // make a new fn name
        csumName:=sf("_bg_block_%x",md5.Sum([]byte(cmd)))

        // define fn
        stdlib["exec"](p.namespace,p.fs,p.ident,"define "+csumName+"()\nr={"+cmd+"\n}\nreturn r;end\n")

        // exec it async
        useName:=p.namespace
        if p.namespace=="" {
            useName="main"
        }
        name:=useName+"::"+csumName

        lmv, isfunc := fnlookup.lmget(name)

        if isfunc {
            // call
            h,id:=task(p.fs,lmv,false,csumName+"@",nil)
            // destroy fn def before leaving
            fnlookup.lmdelete(p.namespace+"::"+csumName)
            numlookup.lmdelete(lmv)
            // return
            return true,"",cmd_result{},bg_result{name:id,handle:h}
        }

        pf("Background process could not be generated.\n")
        return false,"",cmd_result{},bg_result{}

    }

    result=system(cmd,false)
    return result.okay,result.out,result,bg_result{}

}

func (p *leparser) command() (string) {

    dp,err,_:=p.dparse(65,false)
    if err!=nil {
        panic(fmt.Errorf("error parsing string in command operator"))
    }

    switch dp.(type) {
    case string:
    default:
        panic(fmt.Errorf("command operator only accepts strings (not %T)",dp))
    }

    // pf("command : |%s|\n",dp.(string))
    cmd:=system(interpolate(p.namespace,p.fs,p.ident,dp.(string)),false)

    if cmd.okay {
        return cmd.out
    }

    panic(fmt.Errorf("error in command operator (code:%d) '%s'",cmd.code,cmd.err))

}


func (p *leparser) identifier(token *Token) (any) {

    // pf("(id, got token -> %#v)\n",token)

    switch token.subtype {
    case subtypeConst:
        return token.tokVal
    case subtypeStandard:
        return token.tokText
    case subtypeUser:
        return token.tokText
    }

    // filter for functions here. this also sets the subtype for funcs defined late.
    if p.pos+1!=p.len && p.tokens[p.pos+1].tokType == LParen {
        if _, isFunc := stdlib[token.tokText]; !isFunc {
            useName:=p.namespace
            if p.namespace=="" {
                useName="main"
            }
            // pf("  -- checking for name %s::%s in:\n%#v\n",useName,token.tokText,fnlookup.lmshow())
            if fnlookup.lmexists(useName+"::"+token.tokText) {
                p.tokens[p.pos].subtype=subtypeUser
                return token.tokText
            }
        } else {
            p.tokens[p.pos].subtype=subtypeStandard
            return token.tokText
        }
    }

    // local variable lookup:
    bin:=token.bindpos
    if bin>=uint64(len(*p.ident)) {
        newg:=make([]Variable,bin+identGrowthSize)
        copy(newg,*p.ident)
        *p.ident=newg
    }


    if (*p.ident)[bin].declared {
        // fmt.Printf("(il) fetched %s from local ident, bin %d :: %#v\n",token.tokText,bin,(*p.ident)[bin])
        return (*p.ident)[bin].IValue
    }

    // global lookup:
    if val,there:=vget(nil,p.mident,&mident,token.tokText); there {
        // fmt.Printf("(ig) fetched %s->%v from global ident\n",token.tokText,val)
        return val
    }

    // permit module names
    if modlist[token.tokText]==true {
        // pf("(eval) permitting mod name %s\n",token.tokText)
        return nil
    }

    // permit namespace:: names
    ename:=p.namespace+"::"+token.tokText

    if enum[ename]!=nil {
        // pf("(eval) permitting enum name %s\n",ename)
        return nil
    }

    // permit references to uninitialised variables
    if permit_uninit {
        return nil
    }

    // permit struct names
    sname:="anon"
    if token.tokText!="anon" {
        sname=p.namespace+"::"+token.tokText
    }
    if _,found:=structmaps[sname];found || sname=="anon" {
        return sname
    }

    panic(fmt.Errorf("'%s' is uninitialised.",token.tokText))

}


/*
 * Replacement variable handlers.
 */

// for locking vset/vcreate/vdelete during a variable write
var glock = &sync.RWMutex{}
var vlock = &sync.RWMutex{}


func vunset(fs uint32, ident *[]Variable, name string) {
    bin:=bind_int(fs,name)
    vlock.Lock()
    if (*ident)[bin].declared {
        (*ident)[bin] = Variable{declared:false}
    }
    vlock.Unlock()
}

func vdelete(fs uint32, ident *[]Variable, name string, ename string) {

    bin:=bind_int(fs,name)
    vlock.RLock()
    decl:=(*ident)[bin].declared
    vlock.RUnlock()
    if decl {
        m,_:=vget(nil,fs,ident,name)
        switch m:=m.(type) {
        case map[string][]string:
            delete(m,ename)
            vset(nil,fs,ident,name,m)
        case map[string]string:
            delete(m,ename)
            vset(nil,fs,ident,name,m)
        case map[string]int:
            delete(m,ename)
            vset(nil,fs,ident,name,m)
        case map[string]float64:
            delete(m,ename)
            vset(nil,fs,ident,name,m)
        case map[string]bool:
            delete(m,ename)
            vset(nil,fs,ident,name,m)
        case map[string]any:
            delete(m,ename)
            vset(nil,fs,ident,name,m)
        }
    }
}

func gvset(name string, value any) {
    glock.Lock()
    bin:=bind_int(0,name)
    if bin>=uint64(len(gident)) {
        newg:=make([]Variable,bin+identGrowthSize)
        copy(newg,gident)
        gident=newg
    }
    gident[bin].IName=name
    gident[bin].IValue=value
    gident[bin].declared=true
    glock.Unlock()
}

func vset(tok *Token,fs uint32, ident *[]Variable, name string, value any) {

    var bin uint64

    if tok==nil {
        bin=bind_int(fs,name)
        if bin>=uint64(len(*ident)) {
            newident:=make([]Variable,bin+identGrowthSize)
            copy(newident,*ident)
            *ident=newident
        }
        (*ident)[bin]=Variable{IKind:0,ITyped:false}
    } else {
        bin=tok.bindpos
    }

    if bin>=uint64(len(*ident)) {
        newident:=make([]Variable,bin+identGrowthSize)
        copy(newident,*ident)
        *ident=newident
    }

    (*ident)[bin].IName=name
    (*ident)[bin].declared=true

    if (*ident)[bin].ITyped {
        var ok bool
        switch (*ident)[bin].IKind {
        case kbool:
            _,ok=value.(bool)
            if ok { (*ident)[bin].IValue=value }
        case kint:
            _,ok=value.(int)
            if ok { (*ident)[bin].IValue=value }
        case kuint:
            _,ok=value.(uint)
            if ok { (*ident)[bin].IValue=value }
        case kfloat:
            _,ok=value.(float64)
            if ok { (*ident)[bin].IValue=value }

        case kbigi:
            switch value.(type) {
            case uint, uint32, int, int64, uint64, float64, *big.Int, *big.Float, string, uint8:
                (*ident)[bin].IValue.(*big.Int).Set(GetAsBigInt(value))
                ok=true
            }
        case kbigf:
            switch value.(type) {
            case uint, uint32, int, int64, uint64, float64, *big.Int, *big.Float, string, uint8:
                (*ident)[bin].IValue.(*big.Float).Set(GetAsBigFloat(value))
                ok=true
            }

        case kstring:
            _,ok=value.(string)
            if ok { (*ident)[bin].IValue=value }
        case kbyte:
            _,ok=value.(uint8)
            if ok { (*ident)[bin].IValue=value }
        case ksbool:
            _,ok=value.([]bool)
            if ok { (*ident)[bin].IValue=value }
        case ksint:
            _,ok=value.([]int)
            if ok { (*ident)[bin].IValue=value }
        case ksuint:
            _,ok=value.([]uint)
            if ok { (*ident)[bin].IValue=value }
        case ksfloat:
            _,ok=value.([]float64)
            if ok { (*ident)[bin].IValue=value }
        case ksstring:
            _,ok=value.([]string)
            if ok { (*ident)[bin].IValue=value }
        case ksbyte:
            _,ok=value.([]uint8)
            if ok { (*ident)[bin].IValue=value }
        case ksbigi:
            _,ok=value.([]*big.Int)
            if ok { (*ident)[bin].IValue=value }
        case ksbigf:
            _,ok=value.([]*big.Float)
            if ok { (*ident)[bin].IValue=value }
        case ksany:
            _,ok=value.([]any)
            if ok { (*ident)[bin].IValue=value }
        }

        if !ok { panic(fmt.Errorf("invalid assignation : to type [%T] of [%T]", (*ident)[bin].IValue,value)) }

    } else {
        // undeclared or untyped and needs replacing
        (*ident)[bin].IValue=value
    }

    return
}


func vgetElementi(fs uint32, ident *[]Variable, name string, el string) (any, bool) {
    var v any
    var ok bool
    v, ok = vget(nil,fs,ident,name)

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
    case map[string]any:
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
    case []any:
        iel,_:=GetAsInt(el)
        return v[iel],ok
    default:
        // pf("Unknown type in %v[%v] (%T)\n",name,el,v)
        iel,_:=GetAsInt(el)
        for _,val:=range reflect.ValueOf(v).Interface().([]any) {
            if iel==0  { return val,true }
            iel-=1
        }
    }
    return nil, false
}


func vsetElement(tok *Token,fs uint32, ident *[]Variable, name string, el any, value any) {

    var list any
    var ok bool

    if tok==nil {
    list, ok = vget(nil,fs,ident,name)
    } else {
        list, ok = vget(tok,fs,ident,name)
    }

    if !ok {
        list = make(map[string]any, LIST_SIZE_CAP)
        vset(nil,fs,ident,name,list)
    }

    bin:=bind_int(fs,name)

    switch list.(type) {

    case map[string]any:
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
        (*ident)[bin].IValue.(map[string]any)[key] = value
        return
    }

    numel:=el.(int)
    var fault bool

    switch (*ident)[bin].IValue.(type) {

    case string:
        if numel<0 || numel>=len((*ident)[bin].IValue.(string)) {
            panic(fmt.Errorf("Out of bounds access [element %d] of %s",numel,name))
        }
        switch value.(type) {
        case string:
        default:
            panic(fmt.Errorf("Invalid type [%T] in string element access",value))
        }

        nv:=(*ident)[bin].IValue.(string)
        switch len(nv) {
        case 1:
            (*ident)[bin].IValue=str.Join([]string{nv[:numel]},string(value.(string)[0]))
        case 0:
            panic(fmt.Errorf("Assignee empty in element write"))
        default:
            (*ident)[bin].IValue=str.Join([]string{nv[:numel],nv[numel+1:]},string(value.(string)[0]))
        }

    case []int:
        sz:=cap((*ident)[bin].IValue.([]int))
        if numel>=sz {
            newend:=sz*2
            if sz==0 { newend=1 }
            if numel>=newend { newend=numel+1 }
            newar:=make([]int,newend,newend)
            copy(newar,(*ident)[bin].IValue.([]int))
            (*ident)[bin].IValue=newar
        }
        (*ident)[bin].IValue.([]int)[numel]=value.(int)

    case []uint8:
        sz:=cap((*ident)[bin].IValue.([]uint8))
        if numel>=sz-1 {
            newend:=sz*2
            if sz==0 { newend=1 }
            if numel>=newend { newend=numel+1 }
            newar:=make([]uint8,newend,newend)
            copy(newar,(*ident)[bin].IValue.([]uint8))
            (*ident)[bin].IValue=newar
        }
        (*ident)[bin].IValue.([]uint8)[numel]=value.(uint8)

    case []uint:
        sz:=cap((*ident)[bin].IValue.([]uint))
        if numel>=sz-1 {
            newend:=sz*2
            if sz==0 { newend=1 }
            if numel>=newend { newend=numel+1 }
            newar:=make([]uint,newend,newend)
            copy(newar,(*ident)[bin].IValue.([]uint))
            (*ident)[bin].IValue=newar
        }
        (*ident)[bin].IValue.([]uint)[numel]=value.(uint)

    case []bool:
        sz:=cap((*ident)[bin].IValue.([]bool))
        if numel>=sz {
            newend:=sz*2
            if sz==0 { newend=1 }
            if numel>=newend { newend=numel+1 }
            newar:=make([]bool,newend,newend)
            copy(newar,(*ident)[bin].IValue.([]bool))
            (*ident)[bin].IValue=newar
        }
        (*ident)[bin].IValue.([]bool)[numel]=value.(bool)

    case []string:
        sz:=cap((*ident)[bin].IValue.([]string))
        if numel>=sz {
            newend:=sz*2
            if sz==0 { newend=1 }
            if numel>=newend { newend=numel+1 }
            newar:=make([]string,newend,newend)
            copy(newar,(*ident)[bin].IValue.([]string))
            (*ident)[bin].IValue=newar
        }
        (*ident)[bin].IValue.([]string)[numel]=value.(string)

    case []float64:
        sz:=cap((*ident)[bin].IValue.([]float64))
        if numel>=sz {
            newend:=sz*2
            if sz==0 { newend=1 }
            if numel>=newend { newend=numel+1 }
            newar:=make([]float64,newend,newend)
            copy(newar,(*ident)[bin].IValue.([]float64))
            (*ident)[bin].IValue=newar
        }
        (*ident)[bin].IValue.([]float64)[numel],fault=GetAsFloat(value)
        if fault {
            panic(fmt.Errorf("Could not append to float array (ele:%v) a value '%+v' of type '%T'",numel,value,value))
        }

    case []*big.Int:
        sz:=cap((*ident)[bin].IValue.([]*big.Int))
        if numel>=sz {
            newend:=sz*2
            if sz==0 { newend=1 }
            if numel>=newend { newend=numel+1 }
            newar:=make([]*big.Int,newend,newend)
            copy(newar,(*ident)[bin].IValue.([]*big.Int))
            (*ident)[bin].IValue=newar
        }
        (*ident)[bin].IValue.([]*big.Int)[numel]=GetAsBigInt(value)

    case []*big.Float:
        sz:=cap((*ident)[bin].IValue.([]*big.Float))
        if numel>=sz {
            newend:=sz*2
            if sz==0 { newend=1 }
            if numel>=newend { newend=numel+1 }
            newar:=make([]*big.Float,newend,newend)
            copy(newar,(*ident)[bin].IValue.([]*big.Float))
            (*ident)[bin].IValue=newar
        }
        (*ident)[bin].IValue.([]*big.Float)[numel]=GetAsBigFloat(value)

    case []any:
        sz:=cap((*ident)[bin].IValue.([]any))
        if numel>=sz {
            newend:=sz*2
            if sz==0 { newend=1 }
            if numel>=newend { newend=numel+1 }
            newar:=make([]any,newend,newend)
            copy(newar,(*ident)[bin].IValue.([]any))
            (*ident)[bin].IValue=newar
        }
        if value==nil {
            (*ident)[bin].IValue.([]any)[numel]=nil
        } else {
            (*ident)[bin].IValue.([]any)[numel]=value.(any)
        }

    default:
        pf("DEFAULT: Unknown type %T for list %s\n",list,name)

    }

}

func gvget(name string) (any, bool) {
    bin:=bind_int(0,name)
    if bin<uint64(len(gident)) && gident[bin].declared {
        glock.RLock()
        tv:=gident[bin].IValue
        glock.RUnlock()
        return tv,true
    }
    return nil,false
}

func vget(token *Token,fs uint32, ident *[]Variable,name string) (any, bool) {

    var bin uint64
    if token==nil {
        bin=bind_int(fs,name)
    } else {
        bin=token.bindpos
    }

    if bin<uint64(len(*ident)) && (*ident)[bin].declared {
        return (*ident)[bin].IValue,true
    }
    // pf("[#2]-- vget miss for %s on fs %d bin %d (not declared)[#-]\n",name,fs,bin)
    return nil, false
}

func isBool(expr any) bool {
    switch reflect.TypeOf(expr).Kind() {
    case reflect.Bool:
        return true
    }
    return false
}


func isNumber(expr any) bool {
    typeof := reflect.TypeOf(expr).Kind()
    switch typeof {
    case reflect.Float64, reflect.Int, reflect.Int64, reflect.Uint, reflect.Uint8:
        return true
    }
    return false
}


/// convert variable placeholders in strings to their values
func interpolate(ns string,fs uint32, ident *[]Variable, s string) (string) {

    if !interpolation || len(s)==0 {
        return s
    }

    // should finish sooner if no curly open brace in string.
    if str.IndexByte(s, '{') == -1 {
        return s
    }

    orig:=s
    r := regexp.MustCompile(`{([^{}]*)}`)

    //   interparse.mident is set to either 1 or 2 in actor.go
    //   depending on interactive mode flag.

    var interparse *leparser
    interparse=&leparser{}
    interparse.fs=fs
    interparse.ident=ident
    interparse.namespace=ns
    if interactive {
        interparse.mident=1
    } else {
        interparse.mident=2
    }

    for {
        orig_s:=s

        // generate list of matches of {...} in s
        matches := r.FindAllStringSubmatch(s,-1)

        for _, v := range matches {

            kn:=v[1]
            if kn[0]=='=' { continue }

            if kv,there:=vget(nil,fs,ident,kn); there {
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
                case []uint, []float64, []int, []bool, []any, []string:
                    s = str.Replace(s, "{"+kn+"}", sf("%v",kv),-1)
                case any:
                    s = str.Replace(s, "{"+kn+"}", sf("%v",kv),-1)
                default:
                    s = str.Replace(s, "{"+kn+"}", sf("!%T!%v",kv,kv),-1)

                }
            }
        }

        if orig_s==s { break }
    }

    // if nothing was replaced, check if evaluation possible, then it's time to leave this infernal place
    var modified bool

    redo:=true

    for ;redo; {
        modified=false
        for p:=0;p<len(s)-1;p+=1 {
            if s[p]=='{' && s[p+1]=='=' {
                nest:=0
                var close_index int
                for close_index=p;close_index<len(s);close_index+=1 {
                    if s[close_index]=='{' { nest+=1 }
                    if s[close_index]=='}' { nest-=1 }
                    if s[close_index]=='}' && nest==0 {
                        break
                    }
                }
                if nest>0 { break }

                if aval, err := ev(interparse,fs,s[p+2:close_index]); err==nil {
                    s=s[:p]+sf("%v",aval)+s[close_index+1:]
                    modified=true
                    break
                }
                p=close_index+1
            }
        }
        if !modified { redo=false }
    }

    if s=="<nil>" { s=orig }

    return s
}


// evaluate an expression string
func ev(parser *leparser,fs uint32, ws string) (result any, err error) {

    // build token list from string 'ws'
    toks:=make([]Token,0,6)
    var cl int16
    var p int
    var t *lcstruct
    for p = 0; p < len(ws);  {
        t = nextToken(ws, fs, &cl, p)
        if t.carton.tokType==Identifier {
            t.carton.bindpos=bind_int(fs,t.carton.tokText)
            t.carton.bound=true
        }
        if t.tokPos != -1 {
            p = t.tokPos
        }
        toks = append(toks, t.carton)
        if t.eof { break }
    }

    // evaluate token list
    if len(toks)!=0 {
        result, err, _ = parser.Eval(fs,toks)
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
    // crushedOpcodes.Grow(16)

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
    var try_fault bool

    if len(tks)==2 {
        switch tks[1].tokType {
        case SYM_PP,SYM_MM:

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
            expr.result, err, try_fault = p.Eval(fs,tks[k+1:])
            break floop1
        case SYM_PLE:
            expr.result,err, try_fault =p.Eval(fs,tks[k+1:])
            if err==nil {
                eqPos=k
                newEval=make([]Token,len(tks[:k])+2)
                copy(newEval,tks[:k])
                newEval[k]=Token{tokType:O_Plus}
            }
            standardAssign=false
            break floop1
        case SYM_MIE:
            expr.result,err, try_fault =p.Eval(fs,tks[k+1:])
            if err==nil {
                eqPos=k
                newEval=make([]Token,len(tks[:k])+2)
                copy(newEval,tks[:k])
                newEval[k]=Token{tokType:O_Minus}
            }
            standardAssign=false
            break floop1
        case SYM_MUE:
            expr.result,err, try_fault =p.Eval(fs,tks[k+1:])
            if err==nil {
                eqPos=k
                newEval=make([]Token,len(tks[:k])+2)
                copy(newEval,tks[:k])
                newEval[k]=Token{tokType:O_Multiply}
            }
            standardAssign=false
            break floop1
        case SYM_DIE:
            expr.result,err, try_fault =p.Eval(fs,tks[k+1:])
            if err==nil {
                eqPos=k
                newEval=make([]Token,len(tks[:k])+2)
                copy(newEval,tks[:k])
                newEval[k]=Token{tokType:O_Divide}
            }
            standardAssign=false
            break floop1
        case SYM_MOE:
            expr.result,err, try_fault =p.Eval(fs,tks[k+1:])
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
        // pf("[#5]-- w.e. (in fs %d) calling eval on : %#v[#-]\n",fs,tks)
        expr.result, err , try_fault = p.Eval(fs,tks)
        expr.assignPos=-1
    } else {
        expr.assign=true
        expr.assignPos=eqPos

        // before eval, rewrite lhs token bindings to their lhs equivalent
        if !standardAssign {
            if lfs!=fs {
                if newEval[0].tokType==Identifier {
                    if ! (*lident)[newEval[0].bindpos].declared {
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

            expr.result, err , try_fault = p.Eval(lfs,newEval)

        }
    }

    if try_fault {
        p.try_pos=int(p.pc)
        p.try_fault=true
    }

    if err!=nil && !try_fault {
        expr.evalError=true
        expr.errVal=err
        return expr
    }

    if expr.assign {
	 // pf("[#4]Assigning : lfs %d rfs %d toks->%+v[#-]\n",lfs,fs,tks)
	 // pf("[#5]This expression box result address -> %v\n",&expr.result)
        p.doAssign(lfs,lident,fs,rident,tks,&expr,eqPos)
    }

    return expr

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

    // pf("(da) largs -> %#v\n",largs)

    var results []any

    if len(largs)==1 {
        if expr.result==nil {
            results=[]any{nil}
        } else {
            results=[]any{expr.result}
        }
    } else {
        // read results
        if expr.result!=nil {
            switch expr.result.(type) {
            case []any:
                results=expr.result.([]any)
            case any:
                results=append(results,expr.result.(any))
            default:
                pf("unknown result type [%T] in expr box %#v\n",expr.result,expr.result)
            }
        } else {
            results=[]any{nil}
        }
    }

    // figure number of l.h.s items and compare to results.
    if len(largs)>len(results) && len(results)>1 {
        expr.errVal=fmt.Errorf("not enough values to populate assignment")
        expr.evalError=true
        return
    }

    for assno := range largs {

        assignee:=largs[assno]

        /*
        pf("[#6]");
        pf("assignee #%d\n",assno)
        pf("assignee token : %#v\n",assignee)
        pf("assignee value : %+v\n",results[assno])
        pf("[#-]")
        */

        if assignee[0].tokType!=Identifier {
            expr.errVal=fmt.Errorf("Assignee must be an identifier (not '%s')",assignee[0].tokText)
            expr.evalError=true
            return
        }

        // ignore assignment to underscore
        if assignee[0].tokText=="_" { continue }

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
            // inter:=interpolate(p.namespace,rfs,rident,assignee[0].tokText)
            // @note: this is slow, mainly due to allowing interpolation.
            //  if we didn't, then we could re-use the binding value from the assignee[0] token *CHANGED*
            if lfs==rfs {
                vset(&assignee[0], lfs, lident, assignee[0].tokText, results[assno])
            } else {
                vset(nil, lfs, lident, assignee[0].tokText, results[assno])
            }

        case len(assignee)==2:


        case len(assignee)>3:

            ///////////// CHECK FOR a[e]    /////////////////////////////////////////////
            // check for lbrace and rbrace
            if assignee[1].tokType != LeftSBrace || assignee[rbAt].tokType != RightSBrace {
                // pf("\n->%d:%v",assno,assignee)
                expr.errVal=fmt.Errorf("syntax error in assignment")
                expr.evalError=true
                return
            }

            // get the element name expr, eval it. element.(type) is used in switch below.
            // pf("(da) about to eval element name. lfs %d rfs %d toks -> %+v\n",lfs,rfs,assignee[2:rbAt])
            element, err , _ := p.Eval(rfs,assignee[2:rbAt])
            // pf("element eval. element set to %+v for expression tokens : %+v\n",element,assignee[2:rbAt])
            if err!=nil {
                pf("could not evaluate index or key in assignment")
                expr.evalError=true
                expr.errVal=err
                return
            }
            // pf("element [%v] in array access is '%v'\n",element,assignee[2:rbAt])
            /////////////////////////////////////////////////////////////////////////////


            ///////////// CHECK FOR a[e].f= /////////////////////////////////////////////

            // pf("(da) lhs : element and field. element->%v\n",element)

            if dotMode {
                lhs_dotField:=""
                if dotAt!=len(assignee)-2 {
                    expr.errVal=fmt.Errorf("Too much information in field name!")
                    expr.evalError=true
                    return
                }
                lhs_dotField=assignee[dotAt+1].tokText
                // pf("(da) lhs : element and field. field->%v\n",lhs_dotField)
                // do everything here and leave other cases alone, or it will get real messy

                // have to vget from a[e] into tmp
                //  then check element type like in normal fieldless switch case
                //  then modify the tmp like we do in the eqpos==3 dotted case
                //  and then write it back to storage
                // feels like a really bad idea this...

                // find stored variable and copy it:

                var tempStore any
                var found bool
                aryName := assignee[0].tokText
                var eleName string
                switch element.(type) {
                case int:
                    eleName = strconv.FormatInt(int64(element.(int)), 10)
                case int64:
                    eleName = strconv.FormatInt(element.(int64), 10)
                case string:
                    eleName = interpolate(p.namespace,rfs,rident,element.(string))
                default:
                    eleName = sf("%v",element)
                }

                // pf("(da) about to vget-element : %v[%v]\n",aryName,eleName)
                tempStore ,found = vgetElementi(lfs,lident,aryName,eleName)

                if found {

                    // get type info about left/right side of assignment
                    val:=reflect.ValueOf(tempStore)
                    typ:=val.Type()

                    if typ.Kind()==reflect.Struct {

                        // create temp copy of struct
                        tmp:=reflect.New(val.Type()).Elem()
                        tmp.Set(val)

                        if _,exists:=typ.FieldByName(lhs_dotField); exists {

                            // get the required struct field
                            tf:=tmp.FieldByName(lhs_dotField)

                            // Bodge: special case assignments to coerce type:
                            switch tf.Type().String() {
                            case "*big.Int":
                                results[assno]=GetAsBigInt(results[assno])
                            case "*big.Float":
                                results[assno]=GetAsBigFloat(results[assno])
                            }
                            switch results[assno].(type) {
                            case uint32:
                                results[assno]=int(results[assno].(uint32))
                            }
                            // end-bodge

                            intyp:=reflect.ValueOf(results[assno]).Type()

                            if intyp.AssignableTo(tf.Type()) {

                                // make r/w then assign the new value into the copied field
                                tf=reflect.NewAt(tf.Type(),unsafe.Pointer(tf.UnsafeAddr())).Elem()
                                tf.Set(reflect.ValueOf(results[assno]))

                                ////////////////////////////////////////////////////////////////
                                // write the copy back to the 'real' variable
                                if lfs==rfs {
                                    switch element.(type) {
                                    case int:
                                        vsetElement(&assignee[0],lfs,lident,aryName,element.(int),tmp.Interface())
                                    case string:
                                        vsetElement(&assignee[0],lfs,lident,aryName,element.(string),tmp.Interface())
                                    default:
                                        vsetElement(&assignee[0],lfs,lident,aryName,element.(string),tmp.Interface())
                                    }
                                } else {
                                    switch element.(type) {
                                    case int:
                                        vsetElement(nil,lfs,lident,aryName,element.(int),tmp.Interface())
                                    case string:
                                        vsetElement(nil,lfs,lident,aryName,element.(string),tmp.Interface())
                                    default:
                                        vsetElement(nil,lfs,lident,aryName,element.(string),tmp.Interface())
                                    }
                                }
                               return
                                ////////////////////////////////////////////////////////////////

                            } else {
                                expr.errVal=fmt.Errorf("cannot assign result (%T) to %v[%v].%v (%v)",results[assno],aryName,eleName,lhs_dotField,tf.Type())
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
                element = interpolate(p.namespace,rfs,rident,element.(string))
                if lfs==rfs {
                    vsetElement(&assignee[0], lfs, lident, assignee[0].tokText, element.(string), results[assno])
                } else {
                    vsetElement(nil, lfs, lident, assignee[0].tokText, element.(string), results[assno])
                }
            case int:
                if element.(int)<0 {
                    pf("negative element index!! (%s[%v])\n",assignee[0].tokText,element)
                    expr.evalError=true
                    expr.errVal=err
                } else {
                    if lfs==rfs {
                        vsetElement(&assignee[0], lfs, lident, assignee[0].tokText, element.(int), results[assno])
                    } else {
                        vsetElement(nil, lfs, lident, assignee[0].tokText, element.(int), results[assno])
                    }
                }
            default:
                pf("unhandled element type!! [%T]\n",element)
                expr.evalError=true
                expr.errVal=err
            }

        // case eqPos==3:
        case len(assignee)==3:
            ///////////// CHECK FOR a.f=    /////////////////////////////////////////////
            // dotted
            if assignee[1].tokType == SYM_DOT {

                lhs_v:=assignee[0].tokText
                lhs_f:=assignee[2].tokText

                var ts any
                var found bool

                ts,found=vget(&assignee[0],lfs,lident,lhs_v)

                if found {

                    val:=reflect.ValueOf(ts)
                    typ:=reflect.ValueOf(ts).Type()

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
                                if lfs==rfs {
                                    vset(&assignee[0],lfs,lident,lhs_v,tmp.Interface())
                                } else {
                                    vset(nil,lfs,lident,lhs_v,tmp.Interface())
                                }

                            } else {

                                // Bodge: special case assignments to coerce type:
                                switch tf.Type().String() {
                                case "*big.Int":
                                    results[assno]=GetAsBigInt(results[assno])
                                case "*big.Float":
                                    results[assno]=GetAsBigFloat(results[assno])
                                }

                                switch results[assno].(type) {
                                case uint32:
                                    results[assno]=int(results[assno].(uint32))
                                }
                                // end-bodge

                                var intyp reflect.Type
                                // special case, nil
                                if results[assno]!=nil {
                                    intyp=reflect.ValueOf(results[assno]).Type()
                                }

                                if intyp.AssignableTo(tf.Type()) {
                                    tf=reflect.NewAt(tf.Type(),unsafe.Pointer(tf.UnsafeAddr())).Elem()
                                    tf.Set(reflect.ValueOf(results[assno]))
                                    // write the copy back to the 'real' variable
                                    if lfs==rfs {
                                        vset(&assignee[0],lfs,lident,lhs_v,tmp.Interface())
                                    } else {
                                        vset(nil,lfs,lident,lhs_v,tmp.Interface())
                                    }
                                } else {
                                    pf("cannot assign result (%T) to %v.%v (%v)",results[assno],lhs_v,lhs_f,tf.Type())
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
            pf(":\n->%d:%v\n",assno,assignee)
            expr.evalError=true
            expr.errVal=err

        }

    } // end for assno

}


