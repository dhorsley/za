//+build !test

package main

/*

    General array and array-as-list functions.

    Let's never talk about this code.

*/

import (
    "errors"
    "math"
    "reflect"
    "runtime"
    "regexp"
    "sort"
    str "strings"
    "strconv"
//    "sync/atomic"
)

type sortStructInt struct {
    k string
    v int
}

type sortStructUint struct {
    k string
    v uint
}

type sortStructString struct {
    k string
    v string
}

type sortStructInterface struct {
    k string
    v interface{}
}

type sortStructFloat struct {
    k string
    v float64
}

func anyDissimilar(list []interface{}) bool {
    knd := sf("%T", list[0])
    for _, v := range list[1:] {
        if sf("%T", v) != knd {
            return true
        }
    }
    return false
}


func buildNum(a string) float64 {

    // @note: deca (da) is missing. doesn't fit the scheme and is
    //   not commonly used.
    //   kilo (K) is also aliased to lower-case k for common misuse.

    var unitOrder = make(map[rune]int)
    unitOrder['K']=3
    unitOrder['M']=6
    unitOrder['G']=9
    unitOrder['T']=12
    unitOrder['P']=15
    unitOrder['E']=18
    unitOrder['Z']=21
    unitOrder['Y']=24
    unitOrder['k']=3
    unitOrder['h']=2
    unitOrder['d']=-1
    unitOrder['c']=-2
    unitOrder['m']=-3
    unitOrder['u']=-6
    unitOrder['μ']=-6
    unitOrder['n']=-9
    unitOrder['p']=-12
    unitOrder['f']=-15
    unitOrder['a']=-18
    unitOrder['z']=-21
    unitOrder['y']=-24

    minus:=false
    digits:=""
    unit:=0
    for p,c:=range a {
        if p==0 && c=='-' { minus=!minus; continue }
        if c=='.' || (c>='0' && c<='9') { digits+=string(c); continue }
        if _,found:=unitOrder[c]; found {
            unit=unitOrder[c]
            break
        }
    }
    astr:=""
    if minus { astr="-" }
    astr+=digits
    aval,aerr:=GetAsFloat(astr)
    if aerr { return math.NaN() }
    return aval*math.Pow10(unit)
}

// naive solution. should instead do similar to strnumcmp-in.h numcompare() from coreutils.
func human_numcompare(astr,bstr string) (bool) {
    a:=buildNum(astr)
    b:=buildNum(bstr)
    return a<b
}

func human_numcompare_reverse(astr,bstr string) (bool) {
    a:=buildNum(astr)
    b:=buildNum(bstr)
    return a>b
}

func buildListLib() {

    features["list"] = Feature{version: 1, category: "data"}
    categories["list"] = []string{"col", "head", "tail", "sum", "fieldsort", "sort", "uniq",
        "append", "append_to", "insert", "remove", "push_front", "pop", "peek",
        "any", "all", "concat", "esplit", "min", "max", "avg",
        "empty", "list_string", "list_float", "list_int",
        "scan_left","zip",
    }

    slhelp["scan_left"] = LibHelp{in: "numeric_list,op_string,start_seed", out: "list", action: "Creates a list from the intermediary values of processing [#i1]op_string[#i0] while iterating over [#i1]list[#i0]."}
    stdlib["scan_left"] = func(evalfs uint32,ident *[szIdent]Variable,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("scan_left",args,3,
            "3","[]int","string","number",
            "3","[]float64","string","number",
            "3","[]interface {}","string","number"); !ok { return nil,err }

        op_string:=args[1].(string)

        var reduceparser *leparser
        reduceparser=&leparser{}
        calllock.RLock()
        reduceparser.prectable=default_prectable
        reduceparser.ident=ident
        reduceparser.fs=evalfs
        calllock.RUnlock()

        switch args[0].(type) {
        case []int:
            var seed int
            switch args[2].(type) {
            case int:
                seed=args[2].(int)
            default:
                return nil, errors.New("seed must be an int")
            }
            var new_list []int
            for q:=range args[0].([]int) {
                expr:=strconv.Itoa(seed)+op_string+strconv.Itoa(args[0].([]int)[q])
                res,err:=ev(reduceparser,evalfs,expr)
                if err!=nil {
                    return nil,errors.New("could not process list")
                }
                seed=res.(int)
                new_list=append(new_list,res.(int))
            }
            return new_list,nil

        case []float64:
            var seed float64
            switch args[2].(type) {
            case float64:
                seed=args[2].(float64)
            default:
                return nil, errors.New("seed must be a float64")
            }
            var new_list []float64
            for q:=range args[0].([]float64) {
                expr:=strconv.FormatFloat(seed,'f',-1,64)+op_string+strconv.FormatFloat(args[0].([]float64)[q],'f',-1,64)
                res,err:=ev(reduceparser,evalfs,expr)
                if err!=nil {
                    return nil,errors.New("could not process list")
                }
                seed=res.(float64)
                new_list=append(new_list,res.(float64))
            }
            return new_list,nil

        case []interface{}:
            var seed interface{}
            var ok bool
            switch args[2].(type) {
            case string,uint,int:
                seed,ok=GetAsFloat(args[2])
                if !ok {
                    return nil,errors.New("could not convert seed")
                }
            case float64:
                seed=args[2].(float64)
            default:
                return nil, errors.New("unknown seed type")
            }
            var new_list []interface{}
            switch args[0].(type) {
            case []float64:
                for q:=range args[0].([]interface{}) {
                    gf,ok:=GetAsFloat(seed)
                    if !ok {
                        return nil,errors.New("bad seed")
                    }
                    gf2,ok:=GetAsFloat(args[0].([]interface{})[q])
                    if !ok {
                        return nil,errors.New("bad element")
                    }
                    expr:=strconv.FormatFloat(gf,'f',-1,64)+op_string+strconv.FormatFloat(gf2,'f',-1,64)
                    res,err:=ev(reduceparser,evalfs,expr)
                    if err!=nil {
                        return nil,errors.New("could not process list")
                    }
                    seed=res
                    new_list=append(new_list,res)
                }
            }
            return new_list,nil
        }

        return nil,nil
    }

    slhelp["zip"] = LibHelp{in: "list1,list2", out: "list", action: "Creates a list by combining each element of [#i1]list1[#i0] and [#i1]list2[#i0]."}
    stdlib["zip"] = func(evalfs uint32,ident *[szIdent]Variable,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("zip",args,6,
            "2","[]int","[]int",
            "2","[]float64","[]float64",
            "2","[]string","[]string",
            "2","[]bool","[]bool",
            "2","[]uint","[]uint",
            "2","[]interface {}","[]interface {}"); !ok { return nil,err }

        switch args[0].(type) {
        case []bool:
            mx:=max_int([]int{len(args[0].([]bool)),len(args[1].([]bool))})
            var new_list []bool
            for q:=0; q<mx; q++ {
                var a bool
                var b bool
                if q<len(args[0].([]bool)) {
                    a=args[0].([]bool)[q]
                }
                if q<len(args[1].([]bool)) {
                    b=args[1].([]bool)[q]
                }
                new_list=append(new_list,a,b)
            }
            return new_list,nil
        case []int:
            mx:=max_int([]int{len(args[0].([]int)),len(args[1].([]int))})
            var new_list []int
            for q:=0; q<mx; q++ {
                var a int
                var b int
                if q<len(args[0].([]int)) {
                    a=args[0].([]int)[q]
                }
                if q<len(args[1].([]int)) {
                    b=args[1].([]int)[q]
                }
                new_list=append(new_list,a,b)
            }
            return new_list,nil
        case []uint:
            mx:=max_int([]int{len(args[0].([]uint)),len(args[1].([]uint))})
            var new_list []uint
            for q:=0; q<mx; q++ {
                var a uint
                var b uint
                if q<len(args[0].([]uint)) {
                    a=args[0].([]uint)[q]
                }
                if q<len(args[1].([]uint)) {
                    b=args[1].([]uint)[q]
                }
                new_list=append(new_list,a,b)
            }
            return new_list,nil
        case []float64:
            mx:=max_int([]int{len(args[0].([]float64)),len(args[1].([]float64))})
            var new_list []float64
            for q:=0; q<mx; q++ {
                var a float64
                var b float64
                if q<len(args[0].([]float64)) {
                    a=args[0].([]float64)[q]
                }
                if q<len(args[1].([]float64)) {
                    b=args[1].([]float64)[q]
                }
                new_list=append(new_list,a,b)
            }
            return new_list,nil
        case []string:
            mx:=max_int([]int{len(args[0].([]string)),len(args[1].([]string))})
            var new_list []string
            for q:=0; q<mx; q++ {
                var a string
                var b string
                if q<len(args[0].([]string)) {
                    a=args[0].([]string)[q]
                }
                if q<len(args[1].([]string)) {
                    b=args[1].([]string)[q]
                }
                new_list=append(new_list,a,b)
            }
            return new_list,nil
        case []interface{}:
            mx:=max_int([]int{len(args[0].([]interface{})),len(args[1].([]interface{}))})
            var new_list []interface{}
            for q:=0; q<mx; q++ {
                var a interface{}
                var b interface{}
                if q<len(args[0].([]interface{})) {
                    a=args[0].([]interface{})[q]
                }
                if q<len(args[1].([]interface{})) {
                    b=args[1].([]interface{})[q]
                }
                new_list=append(new_list,a,b)
            }
            return new_list,nil
        }

    return nil,errors.New("unspecified error in zip()")
    }

    slhelp["empty"] = LibHelp{in: "list", out: "bool", action: "Is list empty?"}
    stdlib["empty"] = func(evalfs uint32,ident *[szIdent]Variable,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("empty",args,8,
            "1","[]int",
            "1","[]string",
            "1","[]bool",
            "1","[]int64",
            "1","[]uint",
            "1","[]float64",
            "1","[]interface {}",
            "1","nil"); !ok { return nil,err }

        switch args[0].(type) {
        case []string:
            if len(args[0].([]string)) == 0 {
                return true, nil
            }
        case []bool:
            if len(args[0].([]bool)) == 0 {
                return true, nil
            }
        case []int:
            if len(args[0].([]int)) == 0 {
                return true, nil
            }
        case []int64:
            if len(args[0].([]int64)) == 0 {
                return true, nil
            }
        case []uint:
            if len(args[0].([]uint)) == 0 {
                return true, nil
            }
        case []float64:
            if len(args[0].([]float64)) == 0 {
                return true, nil
            }
        case []interface{}:
            if len(args[0].([]interface{})) == 0 {
                return true, nil
            }
        case nil:
            return true, nil
        }
        return false, nil
    }

    slhelp["col"] = LibHelp{in: "string,column[,delimiter]", out: "[]string", action: "Creates a list from a particular [#i1]column[#i0] of line separated [#i1]string[#i0]."}
    stdlib["col"] = func(evalfs uint32,ident *[szIdent]Variable,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("col",args,2,
            "3","string","int","string",
            "2","string","int"); !ok { return nil,err }

        coln := args[1].(int)
        if coln < 1 {
            return nil, errors.New("Argument 2 (column) to col() must be a positive integer!")
        }

        var list []string
        if runtime.GOOS!="windows" {
            list = str.Split(args[0].(string), "\n")
        } else {
            list = str.Split(str.Replace(args[0].(string), "\r\n", "\n", -1), "\n")
        }

        del:=" "
        if len(args)==3 {
            del=args[2].(string)
        }

        var cols []string
        if len(list) > 0 {
            for q := range list {
                z := str.Split(list[q], del)
                if len(z) >= coln {
                    cols = append(cols, z[coln-1])
                }
            }
        }
        return cols, nil
    }


    slhelp["append_to"] = LibHelp{in: "list_name,item", out: "bool_success", action: "Appends [#i1]item[#i0] to [#i1]list_name[#i0]. Returns [#i1]bool_success[#i0] depending on success."}
    stdlib["append_to"] = func(evalfs uint32,ident *[szIdent]Variable,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("append_to",args,1,"2","string","any"); !ok { return nil, err }

        // check args[0] exists in &ident
        if args[0]==nil {
            return nil,errors.New("argument is not a list")
        }
        // @note: should have a mutex around this?
        name:=args[0].(string)
        if ! VarLookup(evalfs,ident,name) {
            return nil, errors.New(sf("list %s does not exist",args[0]))
            // @todo: initialise the var automatically later on
        }

        // check type is compatible

        set:=false
        switch (*ident)[bind_int(evalfs,name)].IValue.(type) {
        case []string:
            (*ident)[bind_int(evalfs,name)].IValue=append((*ident)[bind_int(evalfs,name)].IValue.([]string),sf("%v",args[1]))
            set=true
        case []int:
            switch args[1].(type) {
            case int:
                (*ident)[bind_int(evalfs,name)].IValue=append((*ident)[bind_int(evalfs,name)].IValue.([]int),args[1].(int))
                set=true
            }
        case []uint:
            switch args[1].(type) {
            case uint:
                (*ident)[bind_int(evalfs,name)].IValue=append((*ident)[bind_int(evalfs,name)].IValue.([]uint),args[1].(uint))
                set=true
            }
        case []float64:
            switch args[1].(type) {
            case float64:
                (*ident)[bind_int(evalfs,name)].IValue=append((*ident)[bind_int(evalfs,name)].IValue.([]float64),args[1].(float64))
                set=true
            }
        case []bool:
            switch args[1].(type) {
            case bool:
                (*ident)[bind_int(evalfs,name)].IValue=append((*ident)[bind_int(evalfs,name)].IValue.([]bool),args[1].(bool))
                set=true
            }
        case []interface{}:
            (*ident)[bind_int(evalfs,name)].IValue=append((*ident)[bind_int(evalfs,name)].IValue.([]interface{}),args[1])
            set=true
        /*
        default:
            (*ident)[bind_int(evalfs,name)].IValue=append((*ident)[bind_int(evalfs,name)].IValue.([]interface{}),args[1])
            set=true
        */
        }

        if !set {
            return false,errors.New(sf("unsupported list type in append_to()"))
        }

        return true,nil

    }


    // append returns a[]+arg
    slhelp["append"] = LibHelp{in: "[list,]item", out: "[]mixed", action: "Returns [#i1]new_list[#i0] containing [#i1]item[#i0] appended to [#i1]list[#i0]. If [#i1]list[#i0] is omitted then a new list is created containing [#i1]item[#i0]."}
    stdlib["append"] = func(evalfs uint32,ident *[szIdent]Variable,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("append",args,1,"2","any","any"); !ok { return nil,err }

        // should really do some kind of implicit conversion here (and elsewhere)
        // but not a high-priority, as with most things. 
        if len(args) == 1 {
            switch args[0].(type) {
            case string:
                l := make([]string, 0, 31)
                return append(l, args[0].(string)), nil
            case float64:
                l := make([]float64, 0, 31)
                return append(l, args[0].(float64)), nil
            case bool:
                l := make([]bool, 0, 31)
                return append(l, args[0].(bool)), nil
            case uint:
                l := make([]uint, 0, 31)
                return append(l, args[0].(uint)), nil
            case int:
                l := make([]int, 0, 31)
                return append(l, args[0].(int)), nil
            case nil:
                l := make([]interface{}, 0, 31)
                return l,nil
            case interface{}:
                l := make([]interface{}, 0, 31)
                return append(l, sf("%v", args[0].(interface{}))), nil
            default:
                return nil, errors.New(sf("data type (%T) not supported in lists.",args[0]))
            }
        }

        switch args[0].(type) {
        case nil:
            switch args[1].(type) {
            case float64:
                 args[0] = make([]float64, 0, 31)
            case int:
                 args[0] = make([]int, 0, 31)
            case uint:
                 args[0] = make([]uint, 0, 31)
            case bool:
                 args[0] = make([]bool, 0, 31)
            case string:
                 args[0] = make([]string, 0, 31)
            case interface{}:
                 args[0] = make([]interface{}, 0, 31)
            default:
                 args[0] = make([]interface{}, 0, 31)
            }
        }

        switch args[0].(type) {
        case []string:
            l := append(args[0].([]string), sf("%v", args[1]))
            return l, nil
        case []float64:
            if "float64" != sf("%T", args[1]) {
                return nil, errors.New(sf("(l:float64,a:%T) data types must match in append()", args[1]))
            }
            l := append(args[0].([]float64), args[1].(float64))
            return l, nil
        case []bool:
            if "bool" != sf("%T", args[1]) {
                return nil, errors.New(sf("(l:bool,a:%T) data types must match in append()", args[1]))
            }
            l := append(args[0].([]bool), args[1].(bool))
            return l, nil
        case []int:
            if "int" != sf("%T", args[1]) {
                return nil, errors.New(sf("(l:int,a:%T) data types must match in append()", args[1]))
            }
            l := append(args[0].([]int), args[1].(int))
            return l, nil
        case []interface{}:
            l := append(args[0].([]interface{}), args[1].(interface{}))
            return l, nil
        default:
            return nil, errors.New(sf("data type [%T] not supported in append()",args[0]))
        }
    }

    slhelp["push_front"] = LibHelp{in: "[list,]item", out: "[]mixed", action: "Adds [#i1]item[#i0] to the front of [#i1]list[#i0]. If only an item is provided, then a new list is started."}
    stdlib["push_front"] = func(evalfs uint32,ident *[szIdent]Variable,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("push_front",args,2,
            "2","any","any",
            "1","any"); !ok { return nil,err }

        if len(args) == 1 {
            switch args[0].(type) {
            case string:
                l := make([]string, 0, 31)
                return append(l, args[0].(string)), nil
            case float64:
                l := make([]float64, 0, 31)
                return append(l, args[0].(float64)), nil
            case bool:
                l := make([]bool, 0, 31)
                return append(l, args[0].(bool)), nil
            case uint:
                l := make([]uint, 0, 31)
                return append(l, args[0].(uint)), nil
            case int:
                l := make([]int, 0, 31)
                return append(l, args[0].(int)), nil
            case interface{}:
                l := make([]interface{}, 0, 31)
                return append(l, sf("%v", args[0].(interface{}))), nil
            default:
                return nil, errors.New("data type not supported in lists.")
            }
        }

        switch args[0].(type) {
        case []float64:
            if "float64" != sf("%T", args[1]) {
                return nil, errors.New("data types must match in push_front()")
            }
            l := make([]float64, 0, 31)
            l = append(l, args[1].(float64))
            l = append(l, args[0].([]float64)...)
            return l, nil
        case []bool:
            if "bool" != sf("%T", args[1]) {
                return nil, errors.New("data types must match in push_front()")
            }
            l := make([]bool, 0, 31)
            l = append(l, args[1].(bool))
            l = append(l, args[0].([]bool)...)
            return l, nil
        case []uint:
            if "uint" != sf("%T", args[1]) {
                return nil, errors.New("data types must match in push_front()")
            }
            l := make([]uint, 0, 31)
            l = append(l, args[1].(uint))
            l = append(l, args[0].([]uint)...)
            return l, nil
        case []int:
            if "int" != sf("%T", args[1]) {
                return nil, errors.New("data types must match in push_front()")
            }
            l := make([]int, 0, 31)
            l = append(l, args[1].(int))
            l = append(l, args[0].([]int)...)
            return l, nil
        case []string:
            if "string" != sf("%T", args[1]) {
                return nil, errors.New("data types must match in push_front()")
            }
            l := make([]string, 0, 31)
            l = append(l, args[1].(string))
            l = append(l, args[0].([]string)...)
            return l, nil
        case []interface{}:
            l := make([]interface{}, 0, 31)
            l = append(l, sf("%v", args[1].(interface{})))
            l = append(l, args[0].([]interface{})...)
            return l, nil
        default:
            return nil, errors.New("Unknown list type provided to push_front()")
        }
    }

    slhelp["peek"] = LibHelp{in: "list_name", out: "item", action: "Returns a copy of the last [#i1]item[#i0] in the list [#i1]list_name[#i0]. Returns an error if the list is empty."}
    stdlib["peek"] = func(evalfs uint32,ident *[szIdent]Variable,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("peek",args,6,
            "1","[]string",
            "1","[]int",
            "1","[]uint",
            "1","[]float64",
            "1","[]bool",
            "1","[]interface {}"); !ok { return nil,err }

        switch a:=args[0].(type) {
        case []string:
            if len(a)==0 { break }
            return a[len(a)-1],nil
        case []int:
            if len(a)==0 { break }
            return a[len(a)-1],nil
        case []uint:
            if len(a)==0 { break }
            return a[len(a)-1],nil
        case []float64:
            if len(a)==0 { break }
            return a[len(a)-1],nil
        case []bool:
            if len(a)==0 { break }
            return a[len(a)-1],nil
        case []interface{}:
            if len(a)==0 { break }
            return a[len(a)-1],nil
        }
        return nil,errors.New("No values available to peek()")
    }

    // @note: mut candidate
    slhelp["pop"] = LibHelp{in: "list_name", out: "item", action: "Removes and returns the last [#i1]item[#i0] in the named list [#i1]list_name[#i0]."}
    stdlib["pop"] = func(evalfs uint32,ident *[szIdent]Variable,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("pop",args,1,
            "1","string"); !ok { return nil,err }

            n := args[0].(string)
            v, _ := vget(evalfs,ident, n)
            switch v.(type) {
            case []bool:
                if ln := len(v.([]bool)); ln > 0 {
                    r := v.([]bool)[ln-1]
                    vset(evalfs,ident, n, v.([]bool)[:ln-1])
                    return r, nil
                }
            case []int:
                if ln := len(v.([]int)); ln > 0 {
                    r := v.([]int)[ln-1]
                    vset(evalfs,ident, n, v.([]int)[:ln-1])
                    return r, nil
                }
            case []uint:
                if ln := len(v.([]uint)); ln > 0 {
                    r := v.([]uint)[ln-1]
                    vset(evalfs,ident, n, v.([]uint)[:ln-1])
                    return r, nil
                }
            case []float64:
                if ln := len(v.([]float64)); ln > 0 {
                    r := v.([]float64)[ln-1]
                    vset(evalfs,ident, n, v.([]float64)[:ln-1])
                    return r, nil
                }
            case []string:
                if ln := len(v.([]string)); ln > 0 {
                    r := v.([]string)[ln-1]
                    vset(evalfs,ident, n, v.([]string)[:ln-1])
                    return r, nil
                }
            case []interface{}:
                if ln := len(v.([]interface{})); ln > 0 {
                    r := v.([]interface{})[ln-1]
                    vset(evalfs,ident, n, v.([]interface{})[:ln-1])
                    return r, nil
                }
            }

            return nil, nil

    }

    slhelp["insert"] = LibHelp{in: "list,pos,item", out: "[]new_list", action: "Returns a [#i1]new_list[#i0] with [#i1]item[#i0] inserted in [#i1]list[#i0] at position [#i1]pos[#i0]. (1-based)"}
    stdlib["insert"] = func(evalfs uint32,ident *[szIdent]Variable,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("insert",args,1,"3","any","int","any"); !ok { return nil,err }

        pos := args[1].(int)
        item := args[2]

        switch args[0].(type) {
        case []float64:
            l := make([]float64, 0, 31)
            if pos > 0 {
                l = append(l, args[0].([]float64)[:pos-1]...)
            }
            l = append(l, item.(float64))
            l = append(l, args[0].([]float64)[pos-1:]...)
            return l, nil
        case []string:
            l := make([]string, 0, 31)
            if pos > 0 {
                l = append(l, args[0].([]string)[:pos-1]...)
            }
            l = append(l, sf("%v", item))
            l = append(l, args[0].([]string)[pos-1:]...)
            return l, nil
        case []bool:
            l := make([]bool, 0, 31)
            if pos > 0 {
                l = append(l, args[0].([]bool)[:pos-1]...)
            }
            l = append(l, item.(bool))
            l = append(l, args[0].([]bool)[pos-1:]...)
            return l, nil
        case []int:
            l := make([]int, 0, 31)
            if pos > 0 {
                l = append(l, args[0].([]int)[:pos-1]...)
            }
            l = append(l, item.(int))
            l = append(l, args[0].([]int)[pos-1:]...)
            return l, nil
        case []uint:
            l := make([]uint, 0, 31)
            if pos > 0 {
                l = append(l, args[0].([]uint)[:pos-1]...)
            }
            l = append(l, item.(uint))
            l = append(l, args[0].([]uint)[pos-1:]...)
            return l, nil
        case []interface{}:
            l := make([]interface{}, 0, 31)
            if pos > 0 {
                l = append(l, args[0].([]interface{})[:pos-1]...)
            }
            l = append(l, item.(interface{}))
            l = append(l, args[0].([]interface{})[pos-1:]...)
            return l, nil
        }
        return nil, errors.New("could not insert()")
    }

    slhelp["remove"] = LibHelp{in: "list,pos", out: "[]new_list", action: "Returns a [#i1]new_list[#i0] with the item at position [#i1]pos[#i0] removed. 1-based."}
    stdlib["remove"] = func(evalfs uint32,ident *[szIdent]Variable,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("remove",args,1,"2","any","int"); !ok { return nil,err }

        pos := args[1].(int)

        if pos < 1 {
            return nil, errors.New(sf("position (%v) out of range (low) in remove()", pos))
        }

        switch args[0].(type) {
        case []string:
            if pos > len(args[0].([]string)) {
                return nil, errors.New(sf("position (%v) out of range (string/high) in remove()", pos))
            }
            l := make([]string, 0, 31)
            l = append(l, args[0].([]string)[:pos-1]...)
            l = append(l, args[0].([]string)[pos:]...)
            return l, nil
        case []float64:
            if pos > len(args[0].([]float64)) {
                return nil, errors.New(sf("position (%v) out of range (float/high) in remove()", pos))
            }
            l := make([]float64, 0, 31)
            l = append(l, args[0].([]float64)[:pos-1]...)
            l = append(l, args[0].([]float64)[pos:]...)
            return l, nil
        case []bool:
            if pos > len(args[0].([]bool)) {
                return nil, errors.New(sf("position (%v) out of range (bool/high) in remove()", pos))
            }
            l := make([]bool, 0, 31)
            l = append(l, args[0].([]bool)[:pos-1]...)
            l = append(l, args[0].([]bool)[pos:]...)
            return l, nil
        case []int:
            if pos > len(args[0].([]int)) {
                return nil, errors.New(sf("position (%v) out of range (int/high) in remove()", pos))
            }
            l := make([]int, 0, 31)
            l = append(l, args[0].([]int)[:pos-1]...)
            l = append(l, args[0].([]int)[pos:]...)
            return l, nil
        case []uint:
            if pos > len(args[0].([]uint)) {
                return nil, errors.New(sf("position (%v) out of range (int/high) in remove()", pos))
            }
            l := make([]uint, 0, 31)
            l = append(l, args[0].([]uint)[:pos-1]...)
            l = append(l, args[0].([]uint)[pos:]...)
            return l, nil
        case []interface{}:
            if pos > len(args[0].([]interface{})) {
                return nil, errors.New(sf("position (%v) out of range (interface/high) in remove()", pos))
            }
            l := make([]interface{}, 0, 31)
            l = append(l, args[0].([]interface{})[:pos-1]...)
            l = append(l, args[0].([]interface{})[pos:]...)
            return l, nil
        }
        return nil, errors.New("could not remove()")
    }

    // head(l) returns a[0]
    slhelp["head"] = LibHelp{in: "list", out: "item", action: "Returns the head element of a list."}
    stdlib["head"] = func(evalfs uint32,ident *[szIdent]Variable,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("head",args,1,"1","any"); !ok { return nil,err }

        switch args[0].(type) {
        case []bool:
            if len(args[0].([]bool)) == 0 {
                return []bool{}, nil
            }
            return args[0].([]bool)[0], nil
        case []int:
            if len(args[0].([]int)) == 0 {
                return []int{}, nil
            }
            return args[0].([]int)[0], nil
        case []uint:
            if len(args[0].([]uint)) == 0 {
                return []uint{}, nil
            }
            return args[0].([]uint)[0], nil
        case []float64:
            if len(args[0].([]float64)) == 0 {
                return []float64{}, nil
            }
            return args[0].([]float64)[0], nil
        case []string:
            if len(args[0].([]string)) == 0 {
                return []string{}, nil
            }
            return args[0].([]string)[0], nil
        case []interface{}:
            if len(args[0].([]interface{})) == 0 {
                return []interface{}{}, nil
            }
            return args[0].([]interface{})[0], nil
        }
        return nil, err
    }

    // tail(l) returns a[1:]
    slhelp["tail"] = LibHelp{in: "list", out: "[]new_list", action: "Returns a new list containing all items in [#i1]list[#i0] except the head item."}
    stdlib["tail"] = func(evalfs uint32,ident *[szIdent]Variable,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("tail",args,1,"1","any"); !ok { return nil,err }

        switch args[0].(type) {
        case []bool:
            if len(args[0].([]bool)) == 0 {
                return []bool{}, nil
            }
            return args[0].([]bool)[1:], nil
        case []int:
            if len(args[0].([]int)) == 0 {
                return []int{}, nil
            }
            return args[0].([]int)[1:], nil
        case []uint:
            if len(args[0].([]uint)) == 0 {
                return []uint{}, nil
            }
            return args[0].([]uint)[1:], nil
        case []float64:
            if len(args[0].([]float64)) == 0 {
                return []float64{}, nil
            }
            return args[0].([]float64)[1:], nil
        case []string:
            if len(args[0].([]string)) == 0 {
                return []string{}, nil
            }
            return args[0].([]string)[1:], nil
        case []interface{}:
            if len(args[0].([]interface{})) == 0 {
                return []interface{}{}, nil
            }
            return args[0].([]interface{})[1:], nil
        }
        return nil, errors.New(sf("tail() could not evaluate type %T on %#v", args[0], args[0]))
    }

    // all(l) returns bool true if a[:] all true (&&)
    slhelp["all"] = LibHelp{in: "bool_list", out: "bool", action: "Returns true if all items in [#i1]bool_list[#i0] evaluate to true."}
    stdlib["all"] = func(evalfs uint32,ident *[szIdent]Variable,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("all",args,1,"1","[]bool"); !ok { return nil,err }
        for _, v := range args[0].([]bool) {
            if !v {
                return false, nil
            }
        }
        return true, nil
    }

    // any(l) returns bool true if a[:] any true (||)
    slhelp["any"] = LibHelp{in: "list", out: "boolean", action: "Returns true if any item in [#i1]list[#i0] evaluates to true."}
    stdlib["any"] = func(evalfs uint32,ident *[szIdent]Variable,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("any",args,1,"1","[]bool"); !ok { return nil,err }
        for _, v := range args[0].([]bool) {
            if v {
                return true, nil
            }
        }
        return false, nil
    }


    // fieldsort(s,f,dir) ascending or descending sorted version returned. (type dependant)
    slhelp["fieldsort"] = LibHelp{in: "nl_string,field[,sort_type][,bool_reverse]", out: "new_string", action: "Sorts a newline separated string [#i1]nl_string[#i0] in ascending or descending ([#i1]bool_reverse[#i0]==true) order on key [#i1]field[#i0]."}
    stdlib["fieldsort"] = func(evalfs uint32,ident *[szIdent]Variable,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("fieldsort",args,3,
            "4","string","int","string","bool",
            "3","string","int","string",
            "2","string","int"); !ok { return nil,err }

        // get list
        s:=args[0].(string)

        // get column number
        var field int
        if sf("%T",args[1])!="int" {
            return nil,errors.New("fieldsort() must be provided with a field number.")
        }
        field=args[1].(int) - 1

        // get type
        var stype string
        if len(args)>2 {
           stype=args[2].(string)
        }

        // get direction
        var reverse bool
        if len(args)>3 {
            reverse=args[3].(bool)
        }

        // convert string to list
        var list [][]string
        var r []string

        if runtime.GOOS!="windows" {
            r = str.Split(s, "\n")
        } else {
            r = str.Split(str.Replace(s, "\r\n", "\n", -1), "\n")
        }

        for _,l:= range r {
            if l=="" { continue }
            list=append(list,str.Split(l," "))
        }

        if field<0 || field>len(list[0])-1 {
            return nil,errors.New(sf("Field out of range in fieldsort()\n#%v > %v\n",field,list[0]))
        }

        // build a comparison func
        var f func(int,int) bool
        switch str.ToLower(stype) {
        case "n":
            if !reverse {
                f=func(i, j int) bool {
                    ni,_:=GetAsFloat(list[i][field])
                    nj,_:=GetAsFloat(list[j][field])
                    return ni < nj
                }
            } else {
                f=func(i, j int) bool {
                    ni,_:=GetAsFloat(list[i][field])
                    nj,_:=GetAsFloat(list[j][field])
                    return ni > nj
                }
            }
        case "s":
            if !reverse {
                f=func(i, j int) bool { return list[i][field] < list[j][field] }
            } else {
                f=func(i, j int) bool { return list[i][field] > list[j][field] }
            }
        case "h":
            if !reverse {
                f=func(i,j int) bool {
                    return buildNum(list[i][field]) < buildNum(list[j][field]) }
            } else {
                f=func(i,j int) bool {
                    return buildNum(list[i][field]) > buildNum(list[j][field]) }
            }
        default:
            // string sort
            if !reverse {
                f=func(i, j int) bool { return list[i][field] < list[j][field] }
            } else {
                f=func(i, j int) bool { return list[i][field] > list[j][field] }
            }
        }

        sort.SliceStable(list,f)

        // build a string
        lsep:="\n"
        if runtime.GOOS=="windows" {
            lsep="\r\n"
        }
        var ns str.Builder
        ns.Grow(100)
        for _,l:=range list { ns.WriteString(str.Join(l," ")+lsep) }

        return ns.String(),nil

    }


    // sort(l,[ud]) ascending or descending sorted version returned. (type dependant)
    slhelp["sort"] = LibHelp{in: "list[,bool_reverse]", out: "[]new_list", action: "Sorts a [#i1]list[#i0] in ascending or descending ([#i1]bool_reverse[#i0]==true) order."}
    stdlib["sort"] = func(evalfs uint32,ident *[szIdent]Variable,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("sort",args,2,
            "2","any","bool",
            "1","any"); !ok { return nil,err }

        list := args[0]
        direction := false
        if len(args) == 2 {
            direction = args[1].(bool)
        }

        // need to sort?
        switch list.(type) {
        case []int:
            if len(list.([]int)) < 2        { return list, nil }
        case []uint:
            if len(list.([]uint)) < 2       { return list, nil }
        case []float64:
            if len(list.([]float64)) < 2    { return list, nil }
        case []string:
            if len(list.([]string)) < 2     { return list, nil }
        case map[string]interface{}:
            if len(list.(map[string]interface{})) < 2 { return list,nil }
        case []interface{}:
            if len(list.([]interface{})) < 2 { return list, nil }
        default:
            return nil, errors.New(sf("Can only sort list of type int, float or string. (This is a '%T')", list))
        }

        // sort
        switch direction {
        case false:

            switch list.(type) {

            case []int:
                sort.SliceStable(list, func(i, j int) bool { return list.([]int)[i] < list.([]int)[j] })
                return list, nil

            case []uint:
                sort.SliceStable(list, func(i, j int) bool { return list.([]uint)[i] < list.([]uint)[j] })
                return list, nil

            case []float64:
                sort.SliceStable(list, func(i, j int) bool { return list.([]float64)[i] < list.([]float64)[j] })
                return list, nil

            case []string:
                sort.SliceStable(list, func(i, j int) bool { return list.([]string)[i] < list.([]string)[j] })
                return list, nil

            case []interface{}:
                sort.SliceStable(list, func(i, j int) bool { return sf("%v",list.([]interface{})[i]) < sf("%v",list.([]interface{})[j]) })
                return list, nil

            // @note: ignore this, placeholder until we can do something useful here...
            case map[string]interface{}:

                var iter *reflect.MapIter
                iter = reflect.ValueOf(list.(map[string]interface{})).MapRange()
                iter.Next()
                switch iter.Value().Interface().(type) {
                case int:
                    kv:=make([]sortStructInt,0,len(list.(map[string]interface{})))
                    for k,v:=range list.(map[string]interface{}) { kv=append(kv,sortStructInt{k:k,v:v.(int)}) }
                    sort.Slice(kv,func(i,j int) bool { return kv[i].v < kv[j].v })
                    l:=make(map[string]int); for _,v:=range kv { l[v.k]=v.v }
                    return l,nil
                case uint:
                    kv:=make([]sortStructUint,0,len(list.(map[string]interface{})))
                    for k,v:=range list.(map[string]interface{}) { kv=append(kv,sortStructUint{k:k,v:v.(uint)}) }
                    sort.Slice(kv,func(i,j int) bool { return kv[i].v < kv[j].v })
                    l:=make(map[string]uint); for _,v:=range kv { l[v.k]=v.v }
                    return l,nil
                case float64:
                    kv:=make([]sortStructFloat,0,len(list.(map[string]interface{})))
                    for k,v:=range list.(map[string]interface{}) { kv=append(kv,sortStructFloat{k:k,v:v.(float64)}) }
                    sort.Slice(kv,func(i,j int) bool { return kv[i].v < kv[j].v })
                    l:=make(map[string]float64); for _,v:=range kv { l[v.k]=v.v }
                    return l,nil
                case string:
                    kv:=make([]sortStructString,0,len(list.(map[string]interface{})))
                    for k,v:=range list.(map[string]interface{}) { kv=append(kv,sortStructString{k:k,v:v.(string)}) }
                    sort.Slice(kv,func(i,j int) bool { return kv[i].v < kv[j].v })
                    l:=make(map[string]string); for _,v:=range kv { l[v.k]=v.v }
                    return l,nil
                case interface{}:
                    kv:=make([]sortStructInterface,0,len(list.(map[string]interface{})))
                    for k,v:=range list.(map[string]interface{}) { kv=append(kv,sortStructInterface{k:k,v:v}) }
                    sort.Slice(kv,func(i,j int) bool { return kv[i].k < kv[j].k })
                    l:=make(map[string]interface{}); for _,v:=range kv { l[v.k]=v.v }
                    return l,nil
                default:
                    pf("Error: unknown type '%T' in sort()\n",list)
                    finish(false,ERR_EVAL)
                }
                return args[0], nil
            }

        case true: // descending

            switch list.(type) {

            case []int:
                sort.SliceStable(list, func(i, j int) bool { return list.([]int)[i] > list.([]int)[j] })
                return list, nil

            case []uint:
                sort.SliceStable(list, func(i, j int) bool { return list.([]uint)[i] > list.([]uint)[j] })
                return list, nil

            case []float64:
                sort.SliceStable(list, func(i, j int) bool { return list.([]float64)[i] > list.([]float64)[j] })
                return list, nil

            case []string:
                sort.SliceStable(list, func(i, j int) bool { return list.([]string)[i] > list.([]string)[j] })
                return list, nil

            case []interface{}:
                sort.SliceStable(list, func(i, j int) bool { return sf("%v",list.([]interface{})[i]) > sf("%v",list.([]interface{})[j]) })
                return list, nil

            // placeholders again...
            case map[string]interface{}:
                var iter *reflect.MapIter
                iter = reflect.ValueOf(list.(map[string]interface{})).MapRange()
                iter.Next()
                switch iter.Value().Interface().(type) {
                case int:
                    kv:=make([]sortStructInt,0,len(list.(map[string]interface{})))
                    for k,v:=range list.(map[string]interface{}) { kv=append(kv,sortStructInt{k:k,v:v.(int)}) }
                    sort.Slice(kv,func(i,j int) bool { return kv[i].v > kv[j].v })
                    l:=make(map[string]int); for _,v:=range kv { l[v.k]=v.v }
                    return kv,nil
                case uint:
                    kv:=make([]sortStructUint,0,len(list.(map[string]interface{})))
                    for k,v:=range list.(map[string]interface{}) { kv=append(kv,sortStructUint{k:k,v:v.(uint)}) }
                    sort.Slice(kv,func(i,j int) bool { return kv[i].v > kv[j].v })
                    l:=make(map[string]uint); for _,v:=range kv { l[v.k]=v.v }
                    return kv,nil
                case float64:
                    kv:=make([]sortStructFloat,0,len(list.(map[string]interface{})))
                    for k,v:=range list.(map[string]interface{}) { kv=append(kv,sortStructFloat{k:k,v:v.(float64)}) }
                    sort.Slice(kv,func(i,j int) bool { return kv[i].v > kv[j].v })
                    l:=make(map[string]float64); for _,v:=range kv { l[v.k]=v.v }
                    return kv,nil
                case string:
                    kv:=make([]sortStructString,0,len(list.(map[string]interface{})))
                    for k,v:=range list.(map[string]interface{}) { kv=append(kv,sortStructString{k:k,v:v.(string)}) }
                    sort.Slice(kv,func(i,j int) bool { return kv[i].v > kv[j].v })
                    l:=make(map[string]string); for _,v:=range kv { l[v.k]=v.v }
                    return l,nil
                case interface{}:
                    kv:=make([]sortStructInterface,0,len(list.(map[string]interface{})))
                    for k,v:=range list.(map[string]interface{}) { kv=append(kv,sortStructInterface{k:k,v:v}) }
                    sort.Slice(kv,func(i,j int) bool { return kv[i].k > kv[j].k })
                    l:=make(map[string]interface{}); for _,v:=range kv { l[v.k]=v.v }
                    return l,nil
                default:
                    pf("Error: unknown type '%T' in sort()\n",list)
                    finish(false,ERR_EVAL)
                }
                return args[0], nil
            }

        }
        return args[0], nil
    }

    slhelp["list_float"] = LibHelp{in: "int_or_string_list", out: "[]float_list", action: "Returns [#i1]int_or_string_list[#i0] as a list of floats, with invalid items removed."}
    stdlib["list_float"] = func(evalfs uint32,ident *[szIdent]Variable,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("list_float",args,4,
            "1","[]int",
            "1","[]uint",
            "1","[]string",
            "1","[]interface {}"); !ok { return nil,err }

        var float_list []float64
        switch args[0].(type) {
        case []int:
            for _, q := range args[0].([]int) {
                v, invalid := GetAsFloat(sf("%v", q))
                if !invalid {
                    float_list = append(float_list, v)
                }
            }
        case []uint:
            for _, q := range args[0].([]uint) {
                v, invalid := GetAsFloat(sf("%v", q))
                if !invalid {
                    float_list = append(float_list, v)
                }
            }
        case []string:
            for _, q := range args[0].([]string) {
                v, invalid := GetAsFloat(sf("%v", q))
                if !invalid {
                    float_list = append(float_list, v)
                }
            }
        case []interface{}:
            for _, q := range args[0].([]interface{}) {
                v, invalid := GetAsFloat(sf("%v", q))
                if !invalid {
                    float_list = append(float_list, v)
                }
            }
        }
        return float_list, nil
    }

    slhelp["list_int"] = LibHelp{in: "float_or_string_list", out: "[]int_list", action: "Returns [#i1]float_or_string_list[#i0] as a list of integers. Invalid items will generate an error."}
    stdlib["list_int"] = func(evalfs uint32,ident *[szIdent]Variable,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("list_int",args,6,
            "1","[]int",
            "1","[]uint",
            "1","[]int64",
            "1","[]float64",
            "1","[]string",
            "1","[]interface {}"); !ok { return nil,err }

        var int_list []int
        switch args[0].(type) {
        case []int:
            return args[0].([]int),nil
        case []int64:
            return args[0].([]int64),nil
        case []uint:
            for _, q := range args[0].([]uint) {
                v, invalid := GetAsInt(q)
                if !invalid {
                    int_list = append(int_list, v)
                } else {
                    return nil, errors.New(sf("could not treat %v as an integer.", q))
                }
            }
        case []float64:
            for _, q := range args[0].([]float64) {
                v, invalid := GetAsInt(q)
                if !invalid {
                    int_list = append(int_list, v)
                } else {
                    return nil, errors.New(sf("could not treat %v as an integer.", q))
                }
            }
        case []string:
            for _, q := range args[0].([]string) {
                v, invalid := GetAsInt(sf("%v", q))
                if !invalid {
                    int_list = append(int_list, v)
                } else {
                    return nil, errors.New(sf("could not treat %v as an integer.", q))
                }
            }
        case []interface{}:
            for _, q := range args[0].([]interface{}) {
                v, invalid := GetAsInt(sf("%v", q))
                if !invalid {
                    int_list = append(int_list, v)
                } else {
                    return nil, errors.New(sf("could not treat %v as an integer.", q))
                }
            }
        }
        return int_list, nil
    }

    // @todo: change sprintf for strconv funcs
    slhelp["list_string"] = LibHelp{in: "list", out: "[]string_list", action: "Returns [#i1]list[#i0] of numbers as a list of strings."}
    stdlib["list_string"] = func(evalfs uint32,ident *[szIdent]Variable,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("list_int",args,6,
            "1","[]int",
            "1","[]uint",
            "1","[]int64",
            "1","[]float64",
            "1","[]string",
            "1","[]interface {}"); !ok { return nil,err }
        var string_list []string
        switch args[0].(type) {
        case []string:
            return args[0].([]string),nil
        case []float64:
            for _, q := range args[0].([]float64) { string_list = append(string_list, sf("%v",q)) }
        case []int:
            for _, q := range args[0].([]int) { string_list = append(string_list, sf("%v",q)) }
        case []int64:
            for _, q := range args[0].([]int) { string_list = append(string_list, sf("%v",q)) }
        case []uint:
            for _, q := range args[0].([]uint) { string_list = append(string_list, sf("%v",q)) }
        case []interface{}:
            for _, q := range args[0].([]interface{}) { string_list = append(string_list, sf("%v",q)) }
        }
        return string_list, nil
    }

    // uniq(l) returns a sorted list with duplicates removed
    slhelp["uniq"] = LibHelp{in: "[]list", out: "[]new_list", action: "Returns [#i1]list[#i0] sorted with duplicate values removed."}
    stdlib["uniq"] = func(evalfs uint32,ident *[szIdent]Variable,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("uniq",args,5,
            "1","string",
            "1","[]string",
            "1","[]int",
            "1","[]float64",
            "1","[]uint"); !ok { return nil,err }

        switch args[0].(type) {
        case string:
            var ns str.Builder
            ns.Grow(100)

            var first bool = true
            var prev string

            lsep:="\n"
            var r []string
            if runtime.GOOS!="windows" {
                r = str.Split(args[0].(string), "\n")
            } else {
                r = str.Split(str.Replace(args[0].(string), "\r\n", "\n", -1), "\n")
                lsep="\r\n"
            }

            for _,v:=range r {
                if first {
                    first=false
                    ns.WriteString(v+lsep)
                    prev=v
                    continue
                }
                if v==prev { continue }
                ns.WriteString(v+lsep)
                prev=v
            }

            return ns.String(),nil

        case []float64:
            var newlist []float64
            sort.SliceStable(args[0].([]float64), func(i, j int) bool { return args[0].([]float64)[i] < args[0].([]float64)[j] })
            if len(args[0].([]float64)) > 1 {
                newlist = append(newlist, args[0].([]float64)[0])
                for p := 1; p < len(args[0].([]float64)); p++ {
                    prev := args[0].([]float64)[p-1]
                    if args[0].([]float64)[p] == prev {
                        continue
                    }
                    newlist = append(newlist, args[0].([]float64)[p])
                }
                return newlist, nil
            } else {
                return args[0].([]float64), nil
            }

        case []int:
            var newlist []int
            sort.SliceStable(args[0].([]int), func(i, j int) bool { return args[0].([]int)[i] < args[0].([]int)[j] })
            if len(args[0].([]int)) > 1 {
                newlist = append(newlist, args[0].([]int)[0])
                for p := 1; p < len(args[0].([]int)); p++ {
                    prev := args[0].([]int)[p-1]
                    if args[0].([]int)[p] == prev {
                        continue
                    }
                    newlist = append(newlist, args[0].([]int)[p])
                }
                return newlist, nil
            } else {
                return args[0].([]int), nil
            }

        case []uint:
            var newlist []uint
            sort.SliceStable(args[0].([]uint), func(i, j int) bool { return args[0].([]uint)[i] < args[0].([]uint)[j] })
            if len(args[0].([]uint)) > 1 {
                newlist = append(newlist, args[0].([]uint)[0])
                for p := 1; p < len(args[0].([]uint)); p++ {
                    prev := args[0].([]uint)[p-1]
                    if args[0].([]uint)[p] == prev {
                        continue
                    }
                    newlist = append(newlist, args[0].([]uint)[p])
                }
                return newlist, nil
            } else {
                return args[0].([]uint), nil
            }

        case []string:
            var newlist []string
            sort.SliceStable(args[0].([]string), func(i, j int) bool { return args[0].([]string)[i] < args[0].([]string)[j] })
            if len(args[0].([]string)) > 1 {
                newlist = append(newlist, args[0].([]string)[0])
                for p := 1; p < len(args[0].([]string)); p++ {
                    prev := args[0].([]string)[p-1]
                    if args[0].([]string)[p] == prev {
                        continue
                    }
                    newlist = append(newlist, args[0].([]string)[p])
                }
                return newlist, nil
            } else {
                return args[0].([]string), nil
            }

        default:
            return args[0].([]interface{}), errors.New("uniq() can only operate upon lists of type float, int or string.")
        }
    }

    // concat(l1,l2) returns concatenated list of l1,l2
    slhelp["concat"] = LibHelp{in: "list,list", out: "[]new_list", action: "Concatenates two lists and returns the result."}
    stdlib["concat"] = func(evalfs uint32,ident *[szIdent]Variable,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("concat",args,1,"2","any","any"); !ok { return nil,err }

        if reflect.TypeOf(args[0]) != reflect.TypeOf(args[1]) {
            return nil, errors.New("Cannot concatenate dissimilar type lists.")
        }

        switch args[0].(type) {
        case []bool:
            return append(args[0].([]bool), args[1].([]bool)...), nil
        case []int:
            return append(args[0].([]int), args[1].([]int)...), nil
        case []uint:
            return append(args[0].([]uint), args[1].([]uint)...), nil
        case []string:
            return append(args[0].([]string), args[1].([]string)...), nil
        case []float64:
            return append(args[0].([]float64), args[1].([]float64)...), nil
        case []interface{}:
            return append(args[0].([]interface{}), args[1].([]interface{})...), nil
        }
        return nil, errors.New(sf("Unknown list type concatenation (%T+%T)",args[0],args[1]))
    }

    // esplit(l,"a","b",match) recreates l with a[:match] and returns a[pos:]
    slhelp["esplit"] = LibHelp{in: `[]list,"var1","var2",pos`, out: "bool", action: "Split [#i1]list[#i0] at position [#i1]pos[#i0] (1-based). Each side is put into variables [#i1]var1[#i0] and [#i1]var2[#i0]."}
    stdlib["esplit"] = func(evalfs uint32,ident *[szIdent]Variable,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("esplit",args,1,"4","any","string","string","int"); !ok { return nil,err }

        // pf("in esplit : arg 1 : %s\n",args[1].(string))
        // pf("in esplit : arg 2 : %s\n",args[2].(string))

        switch args[0].(type) {
        case []bool, []string, []uint8, []int, []uint, []float64:
        default:
            return false, errors.New("Argument 1 must be a list.")
        }
        pos := args[3].(int)

        invalidPos := false
        switch args[0].(type) {
        case []float64:
            if pos < 0 || pos > len(args[0].([]float64)) {
                invalidPos = true
                break
            }
            vset(evalfs,ident, args[1].(string), args[0].([]float64)[:pos-1])
            vset(evalfs,ident, args[2].(string), args[0].([]float64)[pos-1:])
        case []bool:
            if pos < 0 || pos > len(args[0].([]bool)) {
                invalidPos = true
                break
            }
            vset(evalfs,ident, args[1].(string), args[0].([]bool)[:pos-1])
            vset(evalfs,ident, args[2].(string), args[0].([]bool)[pos-1:])
        case []int:
            if pos < 0 || pos > len(args[0].([]int)) {
                invalidPos = true
                break
            }
            vset(evalfs,ident, args[1].(string), args[0].([]int)[:pos-1])
            vset(evalfs,ident, args[2].(string), args[0].([]int)[pos-1:])
        case []uint:
            if pos < 0 || pos > len(args[0].([]uint)) {
                invalidPos = true
                break
            }
            vset(evalfs,ident, args[1].(string), args[0].([]uint)[:pos-1])
            vset(evalfs,ident, args[2].(string), args[0].([]uint)[pos-1:])
        case []string:
            if pos < 0 || pos > len(args[0].([]string)) {
                invalidPos = true
                break
            }
            vset(evalfs,ident, args[1].(string), args[0].([]string)[:pos-1])
            vset(evalfs,ident, args[2].(string), args[0].([]string)[pos-1:])
        case []interface{}:
            if pos < 0 || pos > len(args[0].([]interface{})) {
                invalidPos = true
                break
            }
            vset(evalfs,ident, args[1].(string), args[0].([]interface{})[:pos-1])
            vset(evalfs,ident, args[2].(string), args[0].([]interface{})[pos-1:])
        }
        if invalidPos {
            return false, errors.New("List position not within a valid range.")
        }
        return true, nil
    }

    // @note: this one is deliberately removed. it has issues.
    // msplit(l,match) recreates l with a[:matching_element_pos_of(match)] and returns status
    slhelp["msplit"] = LibHelp{in: `[]list,"var1","var2",match`, out: "bool", action: "Split [#i1]list[#i0] at first item matching [#i1]match[#i0]. Each side is put into variables [#i1]var1[#i0] and [#i1]var2[#i0]. Returns success flag."}
    stdlib["msplit"] = func(evalfs uint32,ident *[szIdent]Variable,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("msplit",args,2,
            "4","[]string","string","string","string",
            "4","[]interface {}","string","string","string"); !ok { return nil,err }

        var pos int = -1
        switch args[0].(type) {
        case []string:
            for q, v := range args[0].([]string) {
                if match, _ := regexp.MatchString(args[3].(string), v); match {
                    pos = q
                    break
                }
            }
        case []interface{}:
            for q, v := range args[0].([]interface{}) {
                if match, _ := regexp.MatchString(args[3].(string), v.(string)); match {
                    pos = q
                    break
                }
            }
        }

        if pos == -1 {
            return false, nil
        }

        switch args[0].(type) {
        case []string:
            if pos < 0 || pos > len(args[0].([]string)) {
                return false, errors.New("List position not within a valid range.")
            }
            vset(evalfs,ident, args[1].(string), args[0].([]string)[:pos])
            vset(evalfs,ident, args[2].(string), args[0].([]string)[pos:])
        case []interface{}:
            if pos < 0 || pos > len(args[0].([]interface{})) {
                return false, errors.New("List position not within a valid range.")
            }
            vset(evalfs,ident, args[1].(string), args[0].([]interface{})[:pos])
            vset(evalfs,ident, args[2].(string), args[0].([]interface{})[pos:])
        }
        return true, nil

    }

    slhelp["min"] = LibHelp{in: "list", out: "number", action: "Calculate the minimum value in a [#i1]list[#i0]."}
    stdlib["min"] = func(evalfs uint32,ident *[szIdent]Variable,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("min",args,1,"1","any"); !ok { return nil,err }
        switch args[0].(type) {
        case []int:
            return min_int(args[0].([]int)), nil
        case []uint:
            return min_uint(args[0].([]uint)), nil
        case []float64:
            return min_float64(args[0].([]float64)), nil
        case []interface{}:
            return min_inter(args[0].([]interface{})), nil
        default:
            return nil,errors.New(sf("Unknown number type in min(), type %T\n", args[0]))
        }
    }

    slhelp["max"] = LibHelp{in: "list", out: "number", action: "Calculate the maximum value in a [#i1]list[#i0]."}
    stdlib["max"] = func(evalfs uint32,ident *[szIdent]Variable,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("max",args,1,"1","any"); !ok { return nil,err }
        switch args[0].(type) {
        case []int:
            return max_int(args[0].([]int)), nil
        case []uint:
            return max_uint(args[0].([]uint)), nil
        case []float64:
            return max_float64(args[0].([]float64)), nil
        case []interface{}:
            return max_inter(args[0].([]interface{})), nil
        default:
            return nil,errors.New(sf("Unknown number type in max(), type %T\n", args[0]))
        }
    }

    slhelp["avg"] = LibHelp{in: "list", out: "number", action: "Calculate the average value in a [#i1]list[#i0]."}
    stdlib["avg"] = func(evalfs uint32,ident *[szIdent]Variable,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("avg",args,1,"1","any"); !ok { return nil,err }
        var f float64
        switch args[0].(type) {
        case []int:
            f = float64(avg_int(args[0].([]int)))
        case []uint:
            f = float64(avg_uint(args[0].([]uint)))
        case []float64:
            f = avg_float64(args[0].([]float64))
        case []interface{}:
            f = float64(avg_inter(args[0].([]interface{})))
        default:
            return nil,errors.New(sf("Unknown number type in avg(), type %T\n", args[0]))
        }
        if f != -1 {
            return f, nil
        }
        return 0, errors.New("Divide by zero in avg()")
    }

    slhelp["sum"] = LibHelp{in: "list", out: "number", action: "Calculate the sum of the values in [#i1]list[#i0]."}
    stdlib["sum"] = func(evalfs uint32,ident *[szIdent]Variable,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("sum",args,1,"1","any"); !ok { return nil,err }
        var f float64
        switch args[0].(type) {
        case []int:
            f = float64(sum_int(args[0].([]int)))
        case []uint:
            f = float64(sum_uint(args[0].([]uint)))
        case []float64:
            f = sum_float64(args[0].([]float64))
        case []interface{}:
            f = float64(sum_inter(args[0].([]interface{})))
        default:
            return nil,errors.New(sf("Unknown number type in sum(), type %T\n", args[0]))
        }
        return f, nil
    }

}


