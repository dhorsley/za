//+build !test

package main

import (
    "errors"
    // "os"
    "math"
    "reflect"
    "runtime"
    "regexp"
    "sort"
    str "strings"
)

type sortStructInt struct {
    k string
    v int
}

type sortStructString struct {
    k string
    v string
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

var unitOrder = make(map[rune]int)

func buildNum(a string) float64 {
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

    unitOrder['K']=3
    unitOrder['M']=6
    unitOrder['G']=9
    unitOrder['T']=12
    unitOrder['P']=15
    unitOrder['E']=18
    unitOrder['Z']=21
    unitOrder['Y']=24
    unitOrder['k']=3

    features["list"] = Feature{version: 1, category: "data"}
    categories["list"] = []string{"col", "head", "tail", "sum", "fieldsort", "sort", "uniq",
        "append", "insert", "remove", "push", "deq",
        "any", "all", "concat", "esplit", "min", "max", "avg",
        "empty", "list_string", "list_float", "list_int","numcomp",
    }

    slhelp["numcomp"] = LibHelp{in: "stringa,stringb", out: "bool", action: "Is a<b?"}
    stdlib["numcomp"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
        return human_numcompare(args[0].(string),args[1].(string)),nil
    }

    slhelp["empty"] = LibHelp{in: "list", out: "bool", action: "Is list empty?"}
    stdlib["empty"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
        if len(args) != 1 {
            return nil, errors.New("Incorrect argument count for empty()")
        }
        switch args[0].(type) {
        case []interface{}:
            if len(args[0].([]interface{})) == 0 {
                return true, nil
            }
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
        case []uint8:
            if len(args[0].([]uint8)) == 0 {
                return true, nil
            }
        case []float64:
            if len(args[0].([]float64)) == 0 {
                return true, nil
            }
        case nil:
            return true, nil
        default:
            return nil, errors.New("empty() requires a list.")
        }
        return false, nil
    }

    /*
       slhelp["similar"] = LibHelp{in:"list",out:"bool",action:"Are all items in [#i1]list[#i0] of the same type?"}
       stdlib["similar"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
           if len(args)!=1 { return nil,errors.New("Bad arguments passed to similar()") }
           switch args[0].(type) {
           case []bool:
               if len(args[0].([]bool))<2 { return true,nil }
               if anyDissimilar(args[0].([]bool)) { return false,nil }
           case []int:
               if len(args[0].([]int))<2 { return true,nil }
               if anyDissimilar(args[0].([]int)) { return false,nil }
           case []string:
               if len(args[0].([]string))<2 { return true,nil }
               if anyDissimilar(args[0].([]string)) { return false,nil }
           case []float64:
               if len(args[0].([]float64))<2 { return true,nil }
               if anyDissimilar(args[0].([]float64)) { return false,nil }
           default:
               return nil,errors.New("similar() requires a list.")
           }
           return true,nil
       }
    */

    slhelp["col"] = LibHelp{in: "string_list,column,delimiter", out: "list", action: "Creates a list from a particular [#i1]column[#i0] of [#i1]string_list[#i0]."}
    stdlib["col"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {

        if len(args) != 3 {
            return nil, errors.New("Incorrect argument count for col()")
        }

        switch args[1].(type) {

        case int:

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

            var cols []string
            if len(list) > 0 {
                switch args[2].(type) {
                case string:
                    del := args[2].(string)
                    for q := range list {
                        z := str.Split(list[q], del)
                        if len(z) >= coln {
                            cols = append(cols, z[coln-1])
                        }
                    }
                default:
                    return nil, errors.New("Argument 3 (delimiter) to col() must be a string.")
                }
            }
            return cols, nil

        default:
            return nil, errors.New("Argument 2 (column) to col() must be a positive integer!")

        }

    }

    // append returns a[]+arg
    slhelp["append"] = LibHelp{in: "[list,],item", out: "new_list", action: "Returns [#i1]new_list[#i0] containing [#i1]item[#i0] appended to [#i1]list[#i0]. If [#i1]list[#i0] is omitted then a new list is created containing [#i1]item[#i0]."}
    stdlib["append"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
        if len(args) == 1 {
            switch args[0].(type) {
            case interface{}:
                l := make([]interface{}, 0, 31)
                return append(l, sf("%v", args[0].(interface{}))), nil
            case string:
                l := make([]string, 0, 31)
                return append(l, args[0].(string)), nil
            case float64:
                l := make([]float64, 0, 31)
                return append(l, args[0].(float64)), nil
            case bool:
                l := make([]bool, 0, 31)
                return append(l, args[0].(bool)), nil
            case uint8:
                l := make([]uint8, 0, 31)
                return append(l, args[0].(uint8)), nil
            case int:
                l := make([]int, 0, 31)
                return append(l, args[0].(int)), nil
            default:
                return nil, errors.New(sf("data type (%T) not supported in lists.",args[0]))
            }
        }
        if len(args) != 2 {
            return nil, errors.New("Invalid arguments to append()")
        }
        switch args[0].(type) {
        case nil:
            args[0] = make([]interface{}, 0, 31)
        }
        switch args[0].(type) {
        case []interface{}:
            // if "interface{}" != sf("%T",args[1]) { return nil,errors.New(sf("(l:<interface>,a:%T) data types must match in append()",args[1])) }
            l := append(args[0].([]interface{}), args[1].(interface{}))
            return l, nil
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
        case []uint8:
            if "uint8" != sf("%T", args[1]) {
                return nil, errors.New(sf("(l:uint8,a:%T) data types must match in append()", args[1]))
            }
            l := append(args[0].([]uint8), args[1].(uint8))
            return l, nil
        case []int:
            if "int" != sf("%T", args[1]) {
                return nil, errors.New(sf("(l:int,a:%T) data types must match in append()", args[1]))
            }
            l := append(args[0].([]int), args[1].(int))
            return l, nil
        default:
            return nil, errors.New("data type not supported in append()")
        }
        // return nil, errors.New("append() needs arguments")
    }

    slhelp["push"] = LibHelp{in: "[list,]item", out: "new_list", action: "Adds [#i1]item[#i0] to the front of [#i1]list[#i0]. If only an item is provided, then a new list is started."}
    stdlib["push"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
        if len(args) == 1 {
            switch args[0].(type) {
            case interface{}:
                l := make([]interface{}, 0, 31)
                return append(l, sf("%v", args[0].(interface{}))), nil
            case string:
                l := make([]string, 0, 31)
                return append(l, args[0].(string)), nil
            case float64:
                l := make([]float64, 0, 31)
                return append(l, args[0].(float64)), nil
            case bool:
                l := make([]bool, 0, 31)
                return append(l, args[0].(bool)), nil
            case int:
                l := make([]int, 0, 31)
                return append(l, args[0].(int)), nil
            case uint8:
                l := make([]uint8, 0, 31)
                return append(l, args[0].(uint8)), nil
            default:
                return nil, errors.New("data type not supported in lists.")
            }
        }
        if len(args) != 2 {
            return nil, errors.New("Invalid arguments to push()")
        }
        switch args[0].(type) {
        case []float64:
            if "float64" != sf("%T", args[1]) {
                return nil, errors.New("data types must match in push()")
            }
            l := make([]float64, 0, 31)
            l = append(l, args[1].(float64))
            l = append(l, args[0].([]float64)...)
            return l, nil
        case []interface{}:
            if "interface{}" != sf("%T", args[1]) {
                return nil, errors.New("data types must match in push()")
            }
            l := make([]interface{}, 0, 31)
            l = append(l, sf("%v", args[1].(interface{})))
            l = append(l, args[0].([]interface{})...)
            return l, nil
        case []bool:
            if "bool" != sf("%T", args[1]) {
                return nil, errors.New("data types must match in push()")
            }
            l := make([]bool, 0, 31)
            l = append(l, args[1].(bool))
            l = append(l, args[0].([]bool)...)
            return l, nil
        case []int:
            if "int" != sf("%T", args[1]) {
                return nil, errors.New("data types must match in push()")
            }
            l := make([]int, 0, 31)
            l = append(l, args[1].(int))
            l = append(l, args[0].([]int)...)
            return l, nil
        case []uint8:
            if "uint8" != sf("%T", args[1]) {
                return nil, errors.New("data types must match in push()")
            }
            l := make([]uint8, 0, 31)
            l = append(l, args[1].(uint8))
            l = append(l, args[0].([]uint8)...)
            return l, nil
        case []string:
            if "string" != sf("%T", args[1]) {
                return nil, errors.New("data types must match in push()")
            }
            l := make([]string, 0, 31)
            l = append(l, args[1].(string))
            l = append(l, args[0].([]string)...)
            return l, nil
        default:
            return nil, errors.New("Unknown list type provided to push()")
        }
    }

    slhelp["deq"] = LibHelp{in: "list_name", out: "item", action: "Removes and returns the last [#i1]item[#i0] in the named list [#i1]list_name[#i0]."}
    stdlib["deq"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
        if len(args) != 1 {
            return nil, errors.New("Invalid arguments to deq()")
        }
        switch args[0].(type) {
        case string:
            n := args[0].(string)
            v, _ := vget(evalfs, n)
            switch v.(type) {
            case []interface{}:
                if ln := len(v.([]interface{})); ln > 0 {
                    r := v.([]interface{})[ln-1]
                    vset(evalfs, n, v.([]interface{})[:ln-1])
                    return r, nil
                }
            case []bool:
                if ln := len(v.([]bool)); ln > 0 {
                    r := v.([]bool)[ln-1]
                    vset(evalfs, n, v.([]bool)[:ln-1])
                    return r, nil
                }
            case []int:
                if ln := len(v.([]int)); ln > 0 {
                    r := v.([]int)[ln-1]
                    vset(evalfs, n, v.([]int)[:ln-1])
                    return r, nil
                }
            case []uint8:
                if ln := len(v.([]uint8)); ln > 0 {
                    r := v.([]uint8)[ln-1]
                    vset(evalfs, n, v.([]uint8)[:ln-1])
                    return r, nil
                }
            case []float64:
                if ln := len(v.([]float64)); ln > 0 {
                    r := v.([]float64)[ln-1]
                    vset(evalfs, n, v.([]float64)[:ln-1])
                    return r, nil
                }
            case []string:
                if ln := len(v.([]string)); ln > 0 {
                    r := v.([]string)[ln-1]
                    vset(evalfs, n, v.([]string)[:ln-1])
                    return r, nil
                }
            }

            return nil, errors.New("list was empty")

        default:
            return nil, errors.New("could not evaluate list name in deq()")
        }
    }

    slhelp["insert"] = LibHelp{in: "list,pos,item", out: "new_list", action: "Returns a [#i1]new_list[#i0] with [#i1]item[#i0] inserted in [#i1]list[#i0] at position [#i1]pos[#i0]. (1-based)"}
    stdlib["insert"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
        if len(args) != 3 {
            return nil, errors.New("Invalid arguments to insert()")
        }
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
        case []uint8:
            l := make([]uint8, 0, 31)
            if pos > 0 {
                l = append(l, args[0].([]uint8)[:pos-1]...)
            }
            l = append(l, item.(uint8))
            l = append(l, args[0].([]uint8)[pos-1:]...)
            return l, nil
        case []interface{}:
            l := make([]interface{}, 0, 31)
            if pos > 0 {
                l = append(l, args[0].([]interface{})[:pos-1]...)
            }
            l = append(l, sf("%v", item.(interface{})))
            l = append(l, args[0].([]interface{})[pos-1:]...)
            return l, nil
        }
        return nil, errors.New("could not insert()")
    }

    slhelp["remove"] = LibHelp{in: "list,pos", out: "new_list", action: "Returns a [#i1]new_list[#i0] with the item at position [#i1]pos[#i0] removed."}
    stdlib["remove"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {

        pos := args[1].(int)

        if len(args) != 2 {
            return nil, errors.New("Invalid arguments to remove()")
        }
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
        case []uint8:
            if pos > len(args[0].([]uint8)) {
                return nil, errors.New(sf("position (%v) out of range (uint8/high) in remove()", pos))
            }
            l := make([]uint8, 0, 31)
            l = append(l, args[0].([]uint8)[:pos-1]...)
            l = append(l, args[0].([]uint8)[pos:]...)
            return l, nil
        case []int:
            if pos > len(args[0].([]int)) {
                return nil, errors.New(sf("position (%v) out of range (int/high) in remove()", pos))
            }
            l := make([]int, 0, 31)
            l = append(l, args[0].([]int)[:pos-1]...)
            l = append(l, args[0].([]int)[pos:]...)
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
    stdlib["head"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
        if len(args)!=1 { return nil,errors.New("Bad args (count) to head()") }
        switch args[0].(type) {
        case []bool:
            if len(args[0].([]bool)) == 0 {
                return []bool{}, nil
            }
            return args[0].([]bool)[0], nil
        case []uint8:
            if len(args[0].([]uint8)) == 0 {
                return []uint8{}, nil
            }
            return args[0].([]uint8)[0], nil
        case []int:
            if len(args[0].([]int)) == 0 {
                return []int{}, nil
            }
            return args[0].([]int)[0], nil
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
    slhelp["tail"] = LibHelp{in: "list", out: "new_list", action: "Returns a new list containing all items in [#i1]list[#i0] except the head item."}
    stdlib["tail"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
        if len(args)!=1 { return nil,errors.New("Bad args (count) to tail()") }
        switch args[0].(type) {
        case []bool:
            if len(args[0].([]bool)) == 0 {
                return []bool{}, nil
            }
            return args[0].([]bool)[1:], nil
        case []uint8:
            if len(args[0].([]uint8)) == 0 {
                return []uint8{}, nil
            }
            return args[0].([]uint8)[1:], nil
        case []int:
            if len(args[0].([]int)) == 0 {
                return []int{}, nil
            }
            return args[0].([]int)[1:], nil
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
    slhelp["all"] = LibHelp{in: "list", out: "boolean", action: "Returns true if all items in [#i1]list[#i0] evaluate to true."}
    stdlib["all"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
        if len(args)==0 { return false, errors.New("no list provided to all().") }
        switch args[0].(type) {
        case []bool:
            for _, v := range args[0].([]bool) {
                if !v {
                    return false, nil
                }
            }
            return true, nil
        default:
            return false, errors.New("not a boolean list provided to all().")
        }
    }

    // any(l) returns bool true if a[:] any true (||)
    slhelp["any"] = LibHelp{in: "list", out: "boolean", action: "Returns true if any item in [#i1]list[#i0] evaluate to true."}
    stdlib["any"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
        if len(args)==0 { return false, errors.New("no list provided to any().") }
        switch args[0].(type) {
        case []bool:
            for _, v := range args[0].([]bool) {
                if v {
                    return true, nil
                }
            }
            return false, nil
        default:
            return false, errors.New("not a boolean list provided to any().")
        }
    }

    // fieldsort(s,f,dir) ascending or descending sorted version returned. (type dependant)
    slhelp["fieldsort"] = LibHelp{in: "nl_string,field[,sort_type][,bool_reverse]", out: "new_string", action: "Sorts a newline separated string [#i1]nl_string[#i0] in ascending or descending ([#i1]bool_reverse[#i0]==true) order on key [#i1]field[#i0]."}
    stdlib["fieldsort"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {

        var s string
        if len(args)==0 || len(args)>4 { return nil,errors.New("Bad args (count) to fieldsort()") }

        // get list
        switch args[0].(type) {
        case string:
            s=args[0].(string)
        default:
            return nil, errors.New("fieldsort() can only sort strings.")
        }

        var field int
        if len(args)>1 {
            // get column number
            if sf("%T",args[1])!="int" {
                return nil,errors.New("fieldsort() must be provided with a field number.")
            }
            field=args[1].(int) - 1
        } else {
            field=0
        }

        var fssyntaxerror int

        // get type
        var stype string
        if len(args)>2 {
            if sf("%T",args[2])=="string" {
                stype=args[2].(string)
            } else {
                fssyntaxerror=3
            }
        }

        // get direction
        var reverse bool
        if len(args)>3 {
            if sf("%T",args[3])=="bool" {
                reverse=args[3].(bool)
            } else {
                fssyntaxerror=4
            }
        }

        if fssyntaxerror>0 {
            return nil,errors.New(sf("fieldsort(): type error in parameter %v.",fssyntaxerror))
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

        // pf("Starting sort of length %v on field %v (fc:%v)\n",len(list),field,len(list[0]))

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
    // the map[string]... sorts are just here for completeness. they are utterly pointless currently.
    slhelp["sort"] = LibHelp{in: "list [,bool_reverse]", out: "new_list", action: "Sorts a [#i1]list[#i0] in ascending or descending ([#i1]bool_reverse[#i0]==true) order."}
    stdlib["sort"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
        if len(args)==0 || len(args)>2 { return nil,errors.New("Bad args (count) to sort()") }

        list := args[0]
        direction := false
        if len(args) == 2 {
            direction = args[1].(bool)
        }

        // need to sort?
        switch list.(type) {
        case []int:
            if len(list.([]int)) < 2        { return list, nil }
        case []uint8:
            if len(list.([]uint8)) < 2      { return list, nil }
        case []float64:
            if len(list.([]float64)) < 2    { return list, nil }
        case []string:
            if len(list.([]string)) < 2     { return list, nil }
        case map[string]interface{}:
            if len(list.(map[string]interface{})) < 2 { return list,nil }
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

            case []uint8:
                sort.SliceStable(list, func(i, j int) bool { return list.([]uint8)[i] < list.([]uint8)[j] })
                return list, nil

            case []float64:
                sort.SliceStable(list, func(i, j int) bool { return list.([]float64)[i] < list.([]float64)[j] })
                return list, nil

            case []string:
                sort.SliceStable(list, func(i, j int) bool { return list.([]string)[i] < list.([]string)[j] })
                return list, nil

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

            case []uint8:
                sort.SliceStable(list, func(i, j int) bool { return list.([]uint8)[i] > list.([]uint8)[j] })
                return list, nil

            case []float64:
                sort.SliceStable(list, func(i, j int) bool { return list.([]float64)[i] > list.([]float64)[j] })
                return list, nil

            case []string:
                sort.SliceStable(list, func(i, j int) bool { return list.([]string)[i] > list.([]string)[j] })
                return list, nil

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
                default:
                    pf("Error: unknown type '%T' in sort()\n",list)
                    finish(false,ERR_EVAL)
                }
                return args[0], nil
            }

        }
        return args[0], nil
    }

    slhelp["list_float"] = LibHelp{in: "int_or_string_list", out: "float_list", action: "Returns [#i1]int_or_string_list[#i0] as a list of floats, with invalid items removed."}
    stdlib["list_float"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
        if len(args)!=1 { return nil,errors.New("Bad args (count) to list_float()") }
        var float_list []float64
        switch args[0].(type) {
        case []interface{}:
            for _, q := range args[0].([]interface{}) {
                v, invalid := GetAsFloat(sf("%v", q))
                if !invalid {
                    float_list = append(float_list, v)
                }
            }
        case []int:
            for _, q := range args[0].([]int) {
                v, invalid := GetAsFloat(sf("%v", q))
                if !invalid {
                    float_list = append(float_list, v)
                }
            }
        case []uint8:
            for _, q := range args[0].([]uint8) {
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
        default:
            return nil, errors.New("That's not a list of strings")
        }
        return float_list, nil
    }

    slhelp["list_int"] = LibHelp{in: "float_or_string_list", out: "int_list", action: "Returns [#i1]float_or_string_list[#i0] as a list of integers, with invalid items removed."}
    stdlib["list_int"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
        if len(args)!=1 { return nil,errors.New("Bad args (count) to list_int()") }
        var int_list []int
        switch args[0].(type) {
        case []interface{}:
            for _, q := range args[0].([]interface{}) {
                v, invalid := GetAsInt(sf("%v", q))
                if !invalid {
                    int_list = append(int_list, v)
                } else {
                    return nil, errors.New(sf("could not treat %v as an integer.", q))
                }
            }
        case []float64:
            for _, q := range args[0].([]float64) {
                v, invalid := GetAsInt(sf("%v", q))
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
        default:
            return nil, errors.New("That's not a list of strings")
        }
        return int_list, nil
    }

    slhelp["list_string"] = LibHelp{in: "list", out: "string_list", action: "Returns [#i1]list[#i0] as a list of strings."}
    stdlib["list_string"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
        var string_list []string
        switch args[0].(type) {
        case []interface{}:
            for _, q := range args[0].([]interface{}) { string_list = append(string_list, sf("%v",q)) }
        case []string:
            for _, q := range args[0].([]string) { string_list = append(string_list, sf("%v",q)) }
        case []float64:
            for _, q := range args[0].([]float64) { string_list = append(string_list, sf("%v",q)) }
        case []int:
            for _, q := range args[0].([]int) { string_list = append(string_list, sf("%v",q)) }
        case []uint8:
            for _, q := range args[0].([]uint8) { string_list = append(string_list, sf("%v",q)) }
        default:
            return nil, errors.New(sf("That's not an appropriate list type (%T) for list_string()",args[0]))
        }
        return string_list, nil
    }

    // uniq(l) returns list with duplicates removed (must be pre-sorted?)
    slhelp["uniq"] = LibHelp{in: "list", out: "new_list", action: "Returns [#i1]list[#i0] with duplicate values removed."}
    stdlib["uniq"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {

        if len(args) != 1 { return nil, errors.New("Bad arguments (count) in uniq()") }
        if args[0]==nil { return nil, errors.New("Bad arguments (type) in uniq()") }

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

        case []uint8:
            var newlist []uint8
            sort.SliceStable(args[0].([]uint8), func(i, j int) bool { return args[0].([]uint8)[i] < args[0].([]uint8)[j] })
            if len(args[0].([]uint8)) > 1 {
                newlist = append(newlist, args[0].([]uint8)[0])
                for p := 1; p < len(args[0].([]uint8)); p++ {
                    prev := args[0].([]uint8)[p-1]
                    if args[0].([]uint8)[p] == prev {
                        continue
                    }
                    newlist = append(newlist, args[0].([]uint8)[p])
                }
                return newlist, nil
            } else {
                return args[0].([]uint8), nil
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
    slhelp["concat"] = LibHelp{in: "list", out: "new_list", action: "Concatenates two lists and returns the result."}
    stdlib["concat"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
        if len(args) != 2 {
            return nil, errors.New("Invalid arguments to concat()")
        }
        if reflect.TypeOf(args[0]) != reflect.TypeOf(args[1]) {
            return nil, errors.New("Cannot concatenate dissimilar type lists.")
        }
        switch args[0].(type) {
        case []bool:
            return append(args[0].([]bool), args[1].([]bool)...), nil
        case []interface{}:
            return append(args[0].([]interface{}), args[1].([]interface{})...), nil
        case []int:
            return append(args[0].([]int), args[1].([]int)...), nil
        case []uint8:
            return append(args[0].([]uint8), args[1].([]uint8)...), nil
        case []string:
            return append(args[0].([]string), args[1].([]string)...), nil
        case []float64:
            return append(args[0].([]float64), args[1].([]float64)...), nil
        default:
            pf("type is %T\n", args[0])
        }
        return nil, errors.New("Unknown list type concatenation.")
    }

    // esplit(l,"a","b",match) recreates l with a[:match] and returns a[match:]
    slhelp["esplit"] = LibHelp{in: `list,"var1","var2",match`, out: "bool", action: "Split [#i1]list[#i0] at position [#i1]match[#i0] (1-based). Each side is put into variables [#i1]var1[#i0] and [#i1]var2[#i0]."}
    stdlib["esplit"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
        if len(args) != 4 {
            return false, errors.New("Incorrect number of arguments in esplit()")
        }
        switch args[0].(type) {
        case []bool, []string, []uint8, []int, []float64:
        default:
            return false, errors.New("Argument 1 must be a list.")
        }
        switch args[1].(type) {
        case string:
        default:
            return false, errors.New("Argument 2 must be a string.")
        }
        switch args[2].(type) {
        case string:
        default:
            return false, errors.New("Argument 3 must be a string.")
        }
        switch args[3].(type) {
        case int, int32, int64:
        default:
            return false, errors.New("Argument 4 must be an integer.")
        }
        pos := args[3].(int)
        invalidPos := false
        switch args[0].(type) {
        case []float64:
            if pos < 0 || pos > len(args[0].([]float64)) {
                invalidPos = true
                break
            }
            vset(evalfs, args[1].(string), args[0].([]float64)[:pos-1])
            vset(evalfs, args[2].(string), args[0].([]float64)[pos-1:])
        case []bool:
            if pos < 0 || pos > len(args[0].([]bool)) {
                invalidPos = true
                break
            }
            vset(evalfs, args[1].(string), args[0].([]bool)[:pos-1])
            vset(evalfs, args[2].(string), args[0].([]bool)[pos-1:])
        case []interface{}:
            if pos < 0 || pos > len(args[0].([]interface{})) {
                invalidPos = true
                break
            }
            vset(evalfs, args[1].(string), args[0].([]interface{})[:pos-1])
            vset(evalfs, args[2].(string), args[0].([]interface{})[pos-1:])
        case []int:
            if pos < 0 || pos > len(args[0].([]int)) {
                invalidPos = true
                break
            }
            vset(evalfs, args[1].(string), args[0].([]int)[:pos-1])
            vset(evalfs, args[2].(string), args[0].([]int)[pos-1:])
        case []uint8:
            if pos < 0 || pos > len(args[0].([]uint8)) {
                invalidPos = true
                break
            }
            vset(evalfs, args[1].(string), args[0].([]uint8)[:pos-1])
            vset(evalfs, args[2].(string), args[0].([]uint8)[pos-1:])
        case []string:
            if pos < 0 || pos > len(args[0].([]string)) {
                invalidPos = true
                break
            }
            vset(evalfs, args[1].(string), args[0].([]string)[:pos-1])
            vset(evalfs, args[2].(string), args[0].([]string)[pos-1:])
        }
        if invalidPos {
            return false, errors.New("List position not within a valid range.")
        }
        return true, nil
    }

    // @note: this one is deliberately removed. it has issues.
    // msplit(l,match) recreates l with a[:matching_element_pos_of(match)] and returns a[pos(match):]
    slhelp["msplit"] = LibHelp{in: `list,"var1","var2",match`, out: "bool", action: "Split [#i1]list[#i0] at first item matching [#i1]match[#i0]. Each side is put into variables [#i1]var1[#i0] and [#i1]var2[#i0]."}
    stdlib["msplit"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {

        if len(args) != 4 {
            return false, errors.New("Incorrect number of arguments in msplit()")
        }

        switch args[0].(type) {
        case []interface{}:
            if len(args[0].([]interface{})) == 0 {
                return false, errors.New("Argument 1 has no length.")
            }
        case []string:
            if len(args[0].([]string)) == 0 {
                return false, errors.New("Argument 1 has no length.")
            }
        default:
            return false, errors.New("Argument 1 must be a list of strings.")
        }

        switch args[1].(type) {
        case string:
        default:
            return false, errors.New("Argument 2 must be a string.")
        }
        switch args[2].(type) {
        case string:
        default:
            return false, errors.New("Argument 3 must be a string.")
        }
        switch args[3].(type) {
        case string:
        default:
            return false, errors.New("Argument 4 must be an regex.")
        }

        var pos int = -1
        switch args[0].(type) {
        case []interface{}:
            for q, v := range args[0].([]interface{}) {
                if match, _ := regexp.MatchString(args[3].(string), v.(string)); match {
                    pos = q
                    break
                }
            }
        case []string:
            for q, v := range args[0].([]string) {
                if match, _ := regexp.MatchString(args[3].(string), v); match {
                    pos = q
                    break
                }
            }
        }

        if pos == -1 {
            return false, errors.New("No match found.")
        }

        switch args[0].(type) {
        case []interface{}:
            if pos < 0 || pos > len(args[0].([]interface{})) {
                return false, errors.New("List position not within a valid range.")
            }
            vset(evalfs, args[1].(string), args[0].([]interface{})[:pos])
            vset(evalfs, args[2].(string), args[0].([]interface{})[pos:])
        case []string:
            if pos < 0 || pos > len(args[0].([]string)) {
                return false, errors.New("List position not within a valid range.")
            }
            vset(evalfs, args[1].(string), args[0].([]string)[:pos])
            vset(evalfs, args[2].(string), args[0].([]string)[pos:])
        }
        return true, nil

    }

    slhelp["min"] = LibHelp{in: "list", out: "number", action: "Calculate the minimum value in a [#i1]list[#i0]."}
    stdlib["min"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
        if len(args)!=1 { return 0,errors.New("Bad args (count) to min()") }
        switch args[0].(type) {
        case []interface{}:
            return min_inter(args[0].([]interface{})), nil
        case []int:
            return min_int(args[0].([]int)), nil
        case []float64:
            return min_float64(args[0].([]float64)), nil
        default:
            pf("type %T\n", args[0])
        }
        return 0, errors.New("Unknown number type")
    }

    slhelp["max"] = LibHelp{in: "list", out: "number", action: "Calculate the maximum value in a [#i1]list[#i0]."}
    stdlib["max"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
        if len(args)!=1 { return 0,errors.New("Bad args (count) to max()") }
        switch args[0].(type) {
        case []interface{}:
            return max_inter(args[0].([]interface{})), nil
        case []int:
            return max_int(args[0].([]int)), nil
        case []float64:
            return max_float64(args[0].([]float64)), nil
        default:
            pf("type %T\n", args[0])
        }
        return 0, errors.New("Unknown number type")
    }

    slhelp["avg"] = LibHelp{in: "list", out: "number", action: "Calculate the average value in a [#i1]list[#i0]."}
    stdlib["avg"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
        if len(args)!=1 { return 0,errors.New("Bad args (count) to avg()") }
        var f float64
        switch args[0].(type) {
        case []interface{}:
            f = float64(avg_inter(args[0].([]interface{})))
        case []int:
            f = float64(avg_int(args[0].([]int)))
        case []float64:
            f = avg_float64(args[0].([]float64))
        default:
            pf("type %T\n", args[0])
        }
        if f != -1 {
            return f, nil
        }
        return 0, errors.New("Divide by zero")
    }

    // sum(l)  return sum of a[:]
    slhelp["sum"] = LibHelp{in: "list", out: "number", action: "Calculate the sum of the values in [#i1]list[#i0]."}
    stdlib["sum"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
        if len(args)!=1 { return nil,errors.New("Bad args (count) to sum()") }
        var f float64
        switch args[0].(type) {
        case []interface{}:
            f = float64(sum_inter(args[0].([]interface{})))
        case []int:
            f = float64(sum_int(args[0].([]int)))
        case []float64:
            f = sum_float64(args[0].([]float64))
        default:
            pf("type %T\n", args[0])
        }
        return f, nil
    }

}
