
package main

import (
    "io/ioutil"
    "math"
    "math/big"
    "reflect"
    "strconv"
    str "strings"
    "fmt"
)


func ev_slice_get_type(arr interface{}) reflect.Type {
      return reflect.TypeOf(arr).Elem()
}

func typeOf(val any) string {

    if val == nil {
        return "nil"
    }

    kind := reflect.TypeOf(val).Kind()

    if kind.String()=="map" { return "map" }

    if kind.String()=="ptr" {
        switch sf("%T",val) {
        case "*big.Int":
            return "bigi"
        case "*big.Float":
            return "bigf"
        }
    }

    if kind.String()=="slice" {
        return sf("%T",val)
    }

    switch kind {
    case reflect.Bool:
        return "bool"
    case reflect.Uint:
        return "uint"
    case reflect.Int:
        return "int"
    case reflect.Float64:
        return "float"
    case reflect.String:
        return "string"
    default:
    }

    return sf("<unhandled type [%T] ks (%s)>",val,kind.String())
}

func asBool(val any) (b bool) {
    switch v:=val.(type) {
    case bool:
        b = v
    case string:
        b = v!=""
    case int, int64, uint, uint64:
        b = v!=0
    case *big.Int:
        b = v.Cmp(GetAsBigInt(0))!=0
    case *big.Float:
        b = v.Cmp(GetAsBigFloat(0))!=0
    default:
            panic(fmt.Errorf("type error: required bool'able, but was %s", typeOf(v)))
    }
    return b
}

func as_integer(val any) int {
    switch v:=val.(type) {
    case nil:
        return int(0)
    case *big.Float:
        i64,_:=v.Int64()
        return int(i64)
    case *big.Int:
        return int(v.Int64())
    case bool:
        if !v { return int(0) }
        return int(1)
    case int:
        return int(v)
    case int64:
        return int(v)
    case uint:
        return int(v)
    case float64:
        return int(v)
    }
    panic(fmt.Errorf("type error: required number of type integer, but '%+v' was %s", val, typeOf(val)))
}


func ev_range(val1 any, val2 any) ([]int) {

    if sf("%T",val1)!=sf("%T",val2) {
        // error, types must match
        return nil
    }

    rstart,invalid:=GetAsInt(val1)
    if invalid { return nil }
    rend  ,invalid:=GetAsInt(val2)
    if invalid { return nil }

    if rstart>rend {
        // reversed
        a:=make([]int, rstart-rend+1)
        for i,_ := range a {
            a[i] = rstart-i
        }
        return a
    } else {
        a:=make([]int, rend-rstart+1)
        for i,_ := range a {
            a[i] = rstart+i
        }
        return a
    }

    // unreachable: // return nil

}

func ev_kind_compare(val1 any, val2tok Token) (bool) {

    v1:=typeOf(val1)

    switch val2tok.tokType {
    case T_Number:
        switch v1 {
        case "int","uint","float","bigi","bigf":
            return true
        }
        return false
    }

    switch val2tok.tokType {
    case T_Nil:
        return v1=="nil"
    case T_Bool:
        return v1=="bool"
    case T_Int:
        return v1=="int"
    case T_Uint:
        return v1=="uint"
    case T_Float:
        return v1=="float"
    case T_Bigi:
        return v1=="bigi"
    case T_Bigf:
        return v1=="bigf"
    case T_String:
        return v1=="string"
    case T_Map:
        return v1=="map"
    case T_Array:
        switch v1 {
        case "[]int","[]uint","[]bool","[]string","[]float","[]*big.Int","[]*big.Float","[]interface {}":
            return true
        }
        return false
    case T_Any:
        return v1=="any"
    }

    panic(fmt.Errorf("type error: Unknown type specifier on right-side of IS"))
    // pf("%s\nType [%T] on left of comparison.\n",v1,val1)

}


func ev_in(val1 any, val2 any) (bool) {
    switch vl:=val2.(type) {
    case []string:
        for _, b := range vl { if b == val1 { return true } }
    case []bool:
        for _, b := range vl { if b == val1 { return true } }
    case []int:
        for _, b := range vl { if b == val1 { return true } }
    case []uint:
        for _, b := range vl { if b == val1 { return true } }
    case []float64:
        for _, b := range vl { if b == val1 { return true } }
    case []*big.Int:
        var b *big.Int
        for _, b = range vl {
            if GetAsBigInt(val1).Cmp(b)==0 { return true }
        }
    case []*big.Float:
        var f *big.Float
        for _, f = range vl {
            if GetAsBigFloat(val1).Cmp(f)==0 { return true }
        }
    case []any:
        for _, b := range vl { if b == val1 { return true } }
    default:
        panic(fmt.Errorf("IN operator requires a list to search"))
    }
    return false
}


func ev_add(val1 any, val2 any) (r any) {

    var intInOne, intInTwo, i641, i642, bint1, bint2, bf1, bf2 bool

    // short path integers
    switch val1.(type) {
    case int:
        intInOne=true
    case int64:
        i641=true
    case *big.Int:
        bint1=true
    case *big.Float:
        bf1=true
    }
    switch val2.(type) {
    case int:
        intInTwo=true
    case int64:
        i642=true
    case *big.Int:
        bint2=true
    case *big.Float:
        bf2=true
    }

    if intInOne && intInTwo {
        return val1.(int)+val2.(int)
    }

    if i641 && i642 {
        return val1.(int64)+val2.(int64)
    }

    if bf1 && bf2 {
        var r big.Float
        return r.Add(val1.(*big.Float),val2.(*big.Float))
    }
    if bf1 {
        var r big.Float
        return r.Add(val1.(*big.Float),GetAsBigFloat(val2))
    }
    if bf2 {
        var r big.Float
        return r.Add(GetAsBigFloat(val1),val2.(*big.Float))
    }

    if bint1 && bint2 {
        var r big.Int
        return r.Add(val1.(*big.Int),val2.(*big.Int))
    }
    if bint1 {
        var r big.Int
        return r.Add(val1.(*big.Int),GetAsBigInt(val2))
    }
    if bint2 {
        var r big.Int
        return r.Add(GetAsBigInt(val1),val2.(*big.Int))
    }

    float1, float1OK := val1.(float64)
    float2, float2OK := val2.(float64)

    // upcast int to floats
    if intInOne {
        float1 = float64(val1.(int))
        float1OK = true
    }
    if intInTwo {
        float2 = float64(val2.(int))
        float2OK = true
    }
    if i641 {
        float1 = float64(val1.(int64))
        float1OK = true
    }
    if i642 {
        float2 = float64(val2.(int64))
        float2OK = true
    }

    if float1OK && float2OK {
        return float1 + float2
    }

    // zero nils 
    if intInOne && val2==nil { return val1.(int) }
    if intInTwo && val1==nil { return val2.(int) }

    // handle string concat
    str1, str1OK := val1.(string)
    str2, str2OK := val2.(string)

    if str1OK && str2OK { // string + string = string
        return str1 + str2
    }

    // cast floats to string
    if str1OK && float2OK {
        if var_warn { panic(fmt.Errorf("type error: mixed types in addition (string and float/int)")) }
        return str1 + strconv.FormatFloat(float2, 'f', -1, 64)
    }
    if float1OK && str2OK {
        if var_warn { panic(fmt.Errorf("type error: mixed types in addition (float/int and string)")) }
        return strconv.FormatFloat(float1, 'f', -1, 64) + str2
    }

    // make nils visible
    if str1OK && val2 == nil {
        return str1 + "nil"
    }
    if val1 == nil && str2OK {
        return "nil" + str2
    }

    // cast bools to string
    bool1, bool1OK := val1.(bool)
    bool2, bool2OK := val2.(bool)

    if str1OK && bool2OK {
        if var_warn { panic(fmt.Errorf("type error: mixed types in addition (string and bool)")) }
        return str1 + strconv.FormatBool(bool2)
    }
    if bool1OK && str2OK {
        if var_warn { panic(fmt.Errorf("type error: mixed types in addition (bool and string)")) }
        return strconv.FormatBool(bool1) + str2
    }

    // array concatenation
    arr1, arr1OK := val1.([]any)
    arr2, arr2OK := val2.([]any)
    if arr1OK && arr2OK { return append(arr1, arr2...) }

    arrb1, arrb1OK := val1.([]bool)
    arrb2, arrb2OK := val2.([]bool)
    if arrb1OK && arrb2OK { return append(arrb1, arrb2...) }

    arri1, arri1OK := val1.([]int)
    arri2, arri2OK := val2.([]int)
    if arri1OK && arri2OK { return append(arri1, arri2...) }

    arru1, arru1OK := val1.([]uint)
    arru2, arru2OK := val2.([]uint)
    if arru1OK && arru2OK { return append(arru1, arru2...) }

    arrf1, arrf1OK := val1.([]float64)
    arrf2, arrf2OK := val2.([]float64)
    if arrf1OK && arrf2OK { return append(arrf1, arrf2...) }

    arrs1, arrs1OK := val1.([]string)
    arrs2, arrs2OK := val2.([]string)
    if arrs1OK && arrs2OK { return append(arrs1, arrs2...) }

    obj1, obj1OK := val1.(map[string]any)
    obj2, obj2OK := val2.(map[string]any)

    if obj1OK && obj2OK {
        sum := make(map[string]any)
        for k, v := range obj1 {
            sum[k] = v
        }
        for k, v := range obj2 {
            sum[k] = v
        }
        return sum
    }

    panic(fmt.Errorf("type error: cannot add or concatenate type %s and %s", typeOf(val1), typeOf(val2)))
}

func ev_sub(val1 any, val2 any) (any) {

    var intInOne, intInTwo, i641, i642, bint1, bint2, bf1, bf2 bool

    switch val1.(type) {
    case int:
        intInOne=true
    case int64:
        i641=true
    case *big.Int:
        bint1=true
    case *big.Float:
        bf1=true
    }
    switch val2.(type) {
    case int:
        intInTwo=true
    case int64:
        i642=true
    case *big.Int:
        bint2=true
    case *big.Float:
        bf2=true
    }

    if intInOne && intInTwo {
        return val1.(int) - val2.(int)
    }

    if i641 && i642 {
        return val1.(int64) - val2.(int64)
    }

    if bf1 && bf2 {
        var r big.Float
        return r.Sub(val1.(*big.Float),val2.(*big.Float))
    }

    if bint1 && bint2 {
        var r big.Int
        return r.Sub(val1.(*big.Int),val2.(*big.Int))
    }

    if bf1 || bf2 {
        var r big.Float
        return r.Sub(GetAsBigFloat(val1),GetAsBigFloat(val2))
    }

    if bint1 || bint2 {
        var r big.Int
        return r.Sub(GetAsBigInt(val1),GetAsBigInt(val2))
    }

    float1, float1OK := val1.(float64)
    float2, float2OK := val2.(float64)

    if intInOne {
        float1 = float64(val1.(int))
        float1OK = true
    }
    if intInTwo {
        float2 = float64(val2.(int))
        float2OK = true
    }

    if i641 {
        float1 = float64(val1.(int64))
        float1OK = true
    }
    if i642 {
        float2 = float64(val2.(int64))
        float2OK = true
    }

    if float1OK && float2OK {
        return float1 - float2
    }

    panic(fmt.Errorf("type error: cannot subtract type %T (val:%v) and %T (val:%v)", val1, val1, val2, val2))
}

func ev_mul(val1 any, val2 any) (any) {

    var intInOne, intInTwo, i641, i642, bint1, bint2, bf1, bf2 bool

    switch val1.(type) {
    case int:
        intInOne=true
    case int64:
        i641=true
    case *big.Int:
        bint1=true
    case *big.Float:
        bf1=true
    }
    switch val2.(type) {
    case int:
        intInTwo=true
    case int64:
        i642=true
    case *big.Int:
        bint2=true
    case *big.Float:
        bf2=true
    }

    if intInOne && intInTwo {
        return val1.(int) * val2.(int)
    }

    if i641 && i642 {
        return val1.(int64) * val2.(int64)
    }

    if bint1 && bint2 {
        var r big.Int
        return r.Mul(val1.(*big.Int),val2.(*big.Int))
    }

    if bf1 && bf2 {
        var r big.Float
        return r.Mul(val1.(*big.Float),val2.(*big.Float))
    }

    if bf1 || bf2 {
        var r big.Float
        return r.Mul(GetAsBigFloat(val1),GetAsBigFloat(val2))
    }

    if bint1 || bint2 {
        var r big.Int
        return r.Mul(GetAsBigInt(val1),GetAsBigInt(val2))
    }

    float1, float1OK := val1.(float64)
    float2, float2OK := val2.(float64)

    if intInOne {
        float1 = float64(val1.(int))
        float1OK = true
    }
    if intInTwo {
        float2 = float64(val2.(int))
        float2OK = true
    }
    if i641 {
        float1 = float64(val1.(int64))
        float1OK = true
    }
    if i642 {
        float2 = float64(val2.(int64))
        float2OK = true
    }

    if float1OK && float2OK {
        return float1 * float2
    }

    // int * string = repeat
    str1, str1OK := val1.(string)
    str2, str2OK := val2.(string)
    if (intInOne && str2OK) && val1.(int)>=0 { return str.Repeat(str2,val1.(int)) }
    if (intInTwo && str1OK) && val2.(int)>=0 { return str.Repeat(str1,val2.(int)) }

    // int * struct = repeat
    s1ok := reflect.ValueOf(val1).Kind() == reflect.Struct
    s2ok := reflect.ValueOf(val2).Kind() == reflect.Struct
    if (intInOne && s2ok) && val1.(int)>=0 { var ary []any; for e:=0; e<val1.(int); e+=1 { ary=append(ary,val2) }; return ary }
    if (intInTwo && s1ok) && val2.(int)>=0 { var ary []any; for e:=0; e<val2.(int); e+=1 { ary=append(ary,val1) }; return ary }

    // int * []number = multiply each element
    switch val1.(type) {
    case []int, []float64, []uint, []uint8, []uint64, []int64, []any:
        switch val2.(type) {
        case int:
            ary1,er:=stdlib["list_int"]("",0,nil,val1)
            if er==nil && val2.(int)>=0 {
                length,_:=ulen(ary1)
                var ary []int; for e:=0; e<length; e+=1 { ary=append(ary,ary1.([]int)[e]*val2.(int)) }; return ary
            }
        case float64:
            ary1,er:=stdlib["list_float"]("",0,nil,val1)
            if er==nil && val2.(float64)>=0 {
                length,_:=ulen(ary1)
                var ary []float64; for e:=0; e<length; e+=1 { ary=append(ary,ary1.([]float64)[e]*val2.(float64)) }; return ary
            }
        }
    }
    switch val2.(type) {
    case []int, []float64, []uint, []uint8, []uint64, []int64, []any:
        switch val1.(type) {
        case int:
            ary2,er:=stdlib["list_int"]("",0,nil,val2)
            if er==nil && val1.(int)>=0 {
                length,_:=ulen(ary2)
                var ary []int; for e:=0; e<length; e+=1 { ary=append(ary,ary2.([]int)[e]*val1.(int)) }; return ary
            }
        case float64:
            ary2,er:=stdlib["list_float"]("",0,nil,val2)
            if er==nil && val1.(float64)>=0 {
                length,_:=ulen(ary2)
                var ary []float64; for e:=0; e<length; e+=1 { ary=append(ary,ary2.([]float64)[e]*val1.(float64)) }; return ary
            }
        }
    }

    panic(fmt.Errorf("type error: cannot multiply type %T (val:%v) and %T (val:%v)", val1, val1, val2, val2))
}

func ev_div(val1 any, val2 any) (any) {

    var intInOne, intInTwo, i641, i642, bint1, bint2, bf1, bf2 bool

    switch val1.(type) {
    case int:
        intInOne=true
    case int64:
        i641=true
    case *big.Int:
        bint1=true
    case *big.Float:
        bf1=true
    }
    switch val2.(type) {
    case int:
        intInTwo=true
    case int64:
        i642=true
    case *big.Int:
        bint2=true
    case *big.Float:
        bf2=true
    }

    if intInOne && intInTwo {
        if val2.(int)==0 { panic(fmt.Errorf("eval error: divide by zero")) }
        return val1.(int) / val2.(int)
    }

    if i641 && i642 {
        if val2.(int64)==0 { panic(fmt.Errorf("eval error: divide by zero")) }
        return val1.(int64) / val2.(int64)
    }

    if bf2 && val2.(*big.Float).Sign()==0 { panic(fmt.Errorf("eval error: divide by zero")) }
    if bint2 && val2.(*big.Int).Sign()==0 { panic(fmt.Errorf("eval error: divide by zero")) }

    if bf1 && bf2 {
        var r big.Float
        return r.Quo(val1.(*big.Float),val2.(*big.Float))
    }

    if bint1 && bint2 {
        var r big.Int
        return r.Div(val1.(*big.Int),val2.(*big.Int))
    }

    if bf1 || bf2 {
        var r big.Float
        return r.Quo(GetAsBigFloat(val1),GetAsBigFloat(val2))
    }

    if bint1 || bint2 {
        var r big.Int
        b:=GetAsBigInt(val2)
        if b.Sign()==0 { panic(fmt.Errorf("eval error: divide by zero")) }
        return r.Div(GetAsBigInt(val1),b)
    }

    float1, float1OK := val1.(float64)
    float2, float2OK := val2.(float64)

    if intInOne {
        float1 = float64(val1.(int))
        float1OK = true
    }
    if intInTwo {
        float2 = float64(val2.(int))
        float2OK = true
    }
    if i641 {
        float1 = float64(val1.(int64))
        float1OK = true
    }
    if i642 {
        float2 = float64(val2.(int64))
        float2OK = true
    }

    if float1OK && float2OK {
        if float2==0 { panic(fmt.Errorf("eval error: divide by zero")) }
        return float1 / float2
    }

    // int / []number = divide by/into each element
    switch val1.(type) {
    case []int, []float64, []uint, []uint8, []uint64, []int64, []any:
        switch val2.(type) {
        case int:
            if val2.(int)==0 { panic(fmt.Errorf("eval error: divide by zero")) }
            ary1,er:=stdlib["list_int"]("",0,nil,val1)
            if er==nil && val2.(int)!=0 {
                length,_:=ulen(ary1)
                var ary []int; for e:=0; e<length; e+=1 { ary=append(ary,ary1.([]int)[e]/val2.(int)) }; return ary
            } 
        case float64:
            if val2.(float64)==0 { panic(fmt.Errorf("eval error: divide by zero")) }
            ary1,er:=stdlib["list_float"]("",0,nil,val1)
            if er==nil && val2.(float64)!=0 {
                length,_:=ulen(ary1)
                var ary []float64; for e:=0; e<length; e+=1 { ary=append(ary,ary1.([]float64)[e]/val2.(float64)) }; return ary
            }
        }
    }
    switch val2.(type) {
    case []int, []float64, []uint, []uint8, []uint64, []int64, []any:
        switch val1.(type) {
        case int:
            ary2,er:=stdlib["list_int"]("",0,nil,val2)
            if er==nil {
                length,_:=ulen(ary2)
                var ary []int
                for e:=0; e<length; e+=1 {
                    if ary2.([]int)[e]!=0 {
                        ary=append(ary,val1.(int)/ary2.([]int)[e])
                    } else {
                        ary=append(ary,0)
                    }
                }
                return ary
            }
        case float64:
            ary2,er:=stdlib["list_float"]("",0,nil,val2)
            if er==nil {
                length,_:=ulen(ary2)
                var ary []float64
                for e:=0; e<length; e+=1 {
                    if ary2.([]float64)[e]!=0 {
                        ary=append(ary,val1.(float64)/ary2.([]float64)[e])
                    } else {
                        ary=append(ary,0)
                    }
                }
                return ary
            }
        }
    }

    panic(fmt.Errorf("type error: cannot divide type %s and %s", typeOf(val1), typeOf(val2)))
}

func ev_mod(val1 any, val2 any) (any) {

    var intInOne, intInTwo, i641, i642, bint1, bint2, bf1, bf2 bool

    switch val1.(type) {
    case int:
        intInOne=true
    case int64:
        i641=true
    case *big.Int:
        bint1=true
    case *big.Float:
        bf1=true
    }
    switch val2.(type) {
    case int:
        intInTwo=true
    case int64:
        i642=true
    case *big.Int:
        bint2=true
    case *big.Float:
        bf2=true
    }

    if intInOne && intInTwo {
        return val1.(int) % val2.(int)
    }
    if i641 && i642 {
        return val1.(int64) % val2.(int64)
    }

    if bint1 || bint2 {
        var r big.Int
        return r.Mod(GetAsBigInt(val1),GetAsBigInt(val2))
    }

    if bf1 || bf2 {
        panic(fmt.Errorf("type error: cannot perform modulo on type %s and %s", typeOf(val1), typeOf(val2)))
    }

    float1, float1OK := val1.(float64)
    float2, float2OK := val2.(float64)

    if intInOne {
        float1 = float64(val1.(int))
        float1OK = true
    }
    if intInTwo {
        float2 = float64(val2.(int))
        float2OK = true
    }
    if i641 {
        float1 = float64(val1.(int64))
        float1OK = true
    }
    if i642 {
        float1 = float64(val2.(int64))
        float2OK = true
    }

    if float1OK && float2OK {
        return math.Mod(float1, float2)
    }
    panic(fmt.Errorf("type error: cannot perform modulo on type %s and %s", typeOf(val1), typeOf(val2)))
}

func ev_pow(val1 any, val2 any) (any) {

    var intInOne, intInTwo, bf1, bf2, bint1, bint2 bool
    var int1 int
    var int2 int

    switch i:=val1.(type) {
    case int:
        int1=i
        intInOne=true
    case *big.Int:
        bint1=true
    case *big.Float:
        bf1=true
    }
    switch i:=val2.(type) {
    case int:
        int2=i
        intInTwo=true
    case *big.Int:
        bint2=true
    case *big.Float:
        bf2=true
    }

    if intInOne && intInTwo {
        return int(math.Pow(float64(int1),float64(int2)))
    }

    if bint1 || bint2 {
        var r big.Int
        return r.Exp(GetAsBigInt(val1),GetAsBigInt(val2),nil)
    }

    if bf1 || bf2 {
        panic(fmt.Errorf("type error: cannot perform power operation on type %s and %s", typeOf(val1), typeOf(val2)))
    }

    float1, float1OK := val1.(float64)
    float2, float2OK := val2.(float64)

    if intInOne {
        float1 = float64(int1)
        float1OK = true
    }
    if intInTwo {
        float2 = float64(int2)
        float2OK = true
    }

    if float1OK && float2OK {
        return math.Pow(float1,float2)
    }
    panic(fmt.Errorf("type error: cannot perform exponentiation on type %s and %s", typeOf(val1), typeOf(val2)))
}

func ev_shift_left(left,right any) (any) {
    // both must be integers
    intInOne:=true; uintInTwo:=true; uintInOne:=false
    var uint1 uint
    var int1 int64
    var uint2 uint

    switch i:=left.(type) {
    case int,int64:
        int1,_=GetAsInt64(i)
    case uint,uint8,uint64:
        uint1,_=GetAsUint(i)
        uintInOne=true
    default:
        uintInOne=false
        intInOne=false
    }
    switch i:=right.(type) {
    case uint,uint8,uint64,int,int64:
        uint2,_=GetAsUint(i)
    default:
        uintInTwo=false
    }

    if uintInOne && uintInTwo {
        return uint1 << uint2
    }

    if intInOne && uintInTwo {
        return int1 << uint2
    }

    panic(fmt.Errorf("shift operations only work with integers"))

}

func ev_shift_right(left,right any) (any) {
    // both must be integers
    intInOne:=true; uintInTwo:=true; uintInOne:=false
    var uint1 uint
    var int1 int64
    var uint2 uint

    switch i:=left.(type) {
    case int,int64:
        int1,_=GetAsInt64(i)
    case uint,uint8,uint64:
        uint1,_=GetAsUint(i)
        uintInOne=true
    default:
        uintInOne=false
        intInOne=false
    }
    switch i:=right.(type) {
    case uint,uint8,uint64,int,int64:
        uint2,_=GetAsUint(i)
    default:
        uintInTwo=false
    }

    if uintInOne && uintInTwo {
        return uint1 << uint2
    }

    if intInOne && uintInTwo {
        return int1 >> uint2
    }

    panic(fmt.Errorf("shift operations only work with integers"))
}

func unaryNegate(val any) (any) {
    switch i:=val.(type) {
    case bool:
        // pf("returning negative\n")
        return !i
    }
    panic(fmt.Errorf("cannot negate a non-bool"))
}

func unaryPlus(val any) (any) {

    var intVal int
    intInOne:=true

    switch i:=val.(type) {
    case int:
        intVal=int(i)
    case int64:
        intVal=int(i)
    case *big.Int,*big.Float:
        return i
    default:
        intInOne=false
    }

    if intInOne { return intVal }

    floatVal, ok := val.(float64)
    if ok { return floatVal }

    panic(fmt.Errorf("type error: unary positive requires number, but was %s", typeOf(val)))
}

func unaryMinus(val any) (any) {

    switch i:=val.(type) {
    case int:
        return -int(i)
    case int64:
        return -int(i)
    case *big.Int:
        var r big.Int
        r.Neg(GetAsBigInt(i))
        return &r
    case *big.Float:
        var r big.Float
        r.Neg(GetAsBigFloat(i))
        return &r
    }

    floatVal, ok := val.(float64)
    if ok { return -floatVal }

    panic(fmt.Errorf("type error: unary minus requires number, but was %s", typeOf(val)))
}


func unaryFileInput(i any) (string) {
    switch i.(type) {
    case string:
        s, err := ioutil.ReadFile(i.(string))
        if err!=nil {
            return "" // panic(fmt.Errorf("error importing file '%s' as string",i.(string)))
        }
        if len(s)>0 && s[len(s)-1]==10 { s=s[:len(s)-1] }
        return string(s)
    }
    panic(fmt.Errorf("error importing file as string"))
}


func deepEqual(val1 any, val2 any) (bool) {

    // special case for nil
    if val1==nil && val2==nil {
        return true
    } else {
        if val1==nil || val2==nil {
            return false
        }
    }

    // special case for NaN and big num
    var nt1,nt2 bool
    var bi1, bi2, bf1, bf2 bool

    switch val1.(type) {
    case *big.Int:
        bi1=true
    case *big.Float:
        bf1=true
    case float64:
        if math.IsNaN(val1.(float64)) { nt1=true }
    }
    switch val2.(type) {
    case *big.Int:
        bi2=true
    case *big.Float:
        bf2=true
    case float64:
        if math.IsNaN(val2.(float64)) { nt2=true }
    }
    if nt1 && nt2 { return true }
    if nt1 || nt2 { return false }

    // big num equality check
    // float comparisons are most likely limited in precision
    // because of this autocasting below.
    if bf1 || bf2 {
        v1:=GetAsBigFloat(val1)
        v2:=GetAsBigFloat(val2)
        return v1.Cmp(v2)==0
    }
    if bi1 || bi2 {
        v1:=GetAsBigInt(val1)
        v2:=GetAsBigInt(val2)
        return v1.Cmp(v2)==0
    }

    switch typ1 := val1.(type) {

    case []any:

        typ2, ok := val2.([]any)
        if !ok || len(typ1) != len(typ2) {
            return false
        }

        for idx := range typ1 {
            if !deepEqual(typ1[idx], typ2[idx]) {
                return false
            }
        }
        return true

    case map[string]any:
        typ2, ok := val2.(map[string]any)
        if !ok || len(typ1) != len(typ2) {
            return false
        }
        for idx := range typ1 {
            if !deepEqual(typ1[idx], typ2[idx]) {
                return false
            }
        }
        return true

    case int:
        int2, ok := val2.(int)
        if ok {
            return typ1 == int2
        }
        intsixfour, ok := val2.(int64)
        if ok {
            return int64(typ1) == intsixfour
        }
        float2, ok := val2.(float64)
        if ok {
            return float64(typ1) == float2
        }
        return false

    case uint64:
        int2, ok := val2.(int)
        if ok { return typ1 == uint64(int2) }
        uintsixfour, ok := val2.(uint64)
        if ok { return typ1 == uintsixfour }
        intsixfour, ok := val2.(int64)
        if ok { return typ1 == uint64(intsixfour) }
        float2, ok := val2.(float64)
        if ok { return float64(typ1) == float2 }
        return false

    case int64:
        int2, ok := val2.(int)
        if ok { return typ1 == int64(int2) }
        uintsixfour, ok := val2.(uint64)
        if ok { return typ1 == int64(uintsixfour) }
        intsixfour, ok := val2.(int64)
        if ok { return typ1 == intsixfour }
        float2, ok := val2.(float64)
        if ok { return float64(typ1) == float2 }
        return false

    case float64:
        float2, ok := val2.(float64)
        if ok {
            return typ1 == float2
        }
        int2, ok := val2.(int)
        if ok {
            return typ1 == float64(int2)
        }
        return false
    }
    // pf("D.E. default compare of (%v) against (%v)\n",val1,val2)
    return val1 == val2
}

func compare(val1 any, val2 any, operation int64) (bool) {

    int1, int1OK := val1.(int)
    int2, int2OK := val2.(int)
    if int1OK && int2OK {
        return compareInt(int1, int2, operation)
    }

    float1, float1OK := val1.(float64)
    float2, float2OK := val2.(float64)

    if int1OK {
        float1 = float64(int1)
        float1OK = true
    }
    if int2OK {
        float2 = float64(int2)
        float2OK = true
    }

    if float1OK && float2OK {
        return compareFloat(float1, float2, operation)
    }

    // big num equality check
    // float comparisons are most likely limited in precision
    // because of this autocasting below.

    var i164,i264,bf1,bf2,bi1,bi2 bool
    switch val1.(type) {
    case *big.Float:
        bf1=true
    case *big.Int:
        bi1=true
    case int64:
        i164=true
    }
    switch val2.(type) {
    case *big.Float:
        bf2=true
    case *big.Int:
        bi2=true
    case int64:
        i264=true
    }

    if bf1 || bf2 {
        v1:=GetAsBigFloat(val1)
        v2:=GetAsBigFloat(val2)
        return compareBigFloat(v1,v2,operation)
    }
    if bi1 || bi2 {
        v1:=GetAsBigInt(val1)
        v2:=GetAsBigInt(val2)
        return compareBigInt(v1,v2,operation)
    }

    if i164 || i264 {
        i1,e1:=GetAsInt64(val1)
        i2,e2:=GetAsInt64(val2)
        if e1 || e2 {
            return false
        }
        return compareI64(i1,i2,operation)
    }

    str1, str1OK := val1.(string)
    str2, str2OK := val2.(string)

    if str1OK && str2OK {
        return compareString(str1, str2, operation)
    }

    if val1==nil && val2==nil {
        return true
    }

    panic(fmt.Errorf("type error: cannot compare type %s and %s", typeOf(val1), typeOf(val2)))
}

func compareString(val1 string, val2 string, operation int64) (bool) {
    switch operation {
    case SYM_LT:
        return val1 < val2
    case SYM_LE:
        return val1 <= val2
    case SYM_GT:
        return val1 > val2
    case SYM_GE:
        return val1 >= val2
    }
    panic(fmt.Errorf("syntax error: unsupported operation %q", operation))
}

func compareInt(val1 int, val2 int, operation int64) (bool) {
    switch operation {
    case SYM_LT:
        return val1 < val2
    case SYM_LE:
        return val1 <= val2
    case SYM_GT:
        return val1 > val2
    case SYM_GE:
        return val1 >= val2
    }
    panic(fmt.Errorf("syntax error: unsupported operation %q", operation))
}

func compareFloat(val1 float64, val2 float64, operation int64) (bool) {
    switch operation {
    case SYM_LT:
        return val1 < val2
    case SYM_LE:
        return val1 <= val2
    case SYM_GT:
        return val1 > val2
    case SYM_GE:
        return val1 >= val2
    }
    panic(fmt.Errorf("syntax error: unsupported operation %q", operation))
}

func compareBigFloat(val1 *big.Float, val2 *big.Float, operation int64) (bool) {
    switch operation {
    case SYM_LT:
        return val1.Cmp(val2)==-1
    case SYM_LE:
        return val1.Cmp(val2)<1
    case SYM_GT:
        return val1.Cmp(val2)==1
    case SYM_GE:
        return val1.Cmp(val2)>-1
    }
    panic(fmt.Errorf("syntax error: unsupported operation %q", operation))
}

func compareBigInt(val1 *big.Int, val2 *big.Int, operation int64) (bool) {
    switch operation {
    case SYM_LT:
        return val1.Cmp(val2)==-1
    case SYM_LE:
        return val1.Cmp(val2)<1
    case SYM_GT:
        return val1.Cmp(val2)==1
    case SYM_GE:
        return val1.Cmp(val2)>-1
    }
    panic(fmt.Errorf("syntax error: unsupported operation %q", operation))
}

func compareI64(val1 int64, val2 int64, operation int64) (bool) {
    switch operation {
    case SYM_LT:
        return val1<val2
    case SYM_LE:
        return val1<=val2
    case SYM_GT:
        return val1>val2
    case SYM_GE:
        return val1>=val2
    }
    panic(fmt.Errorf("syntax error: unsupported operation %q", operation))
}

func asObjectKey(key any) (string) {
    s, ok := key.(string)
    if !ok {
        panic(fmt.Errorf("type error: object key must be string, but was %s", typeOf(key)))
    }
    return s
}


func accessArray(ident *[]Variable, obj any, field any) (any) {

     // pf("aa-typ : (%T)\n",obj)
     // pf("aa-obj : (%T) %+v\n",obj,obj)
     // pf("aa-fld : (%T) %+v\n",field,field)

    switch obj:=obj.(type) {
    case string: // string[n] access
        ifield,invalid:=GetAsInt(field)
        if !invalid {
            // pf("obj-if : (%T) %+v\n",obj[ifield],obj[ifield])
            if ifield>=0 && ifield<len(obj) {
                return string(obj[ifield])
            } else {
                panic(fmt.Errorf("out-of-bounds access to string sub-script %d",ifield))
            }
        }
        panic(fmt.Errorf("string sub-script '%v' must be a number",field))
    case map[string]alloc_info:
        return obj[field.(string)]
    case map[string]string:
        return obj[field.(string)]
    case map[string]float64:
        return obj[field.(string)]
    case map[string]int:
        return obj[field.(string)]
    case map[string]uint:
        return obj[field.(string)]
    case map[string]any:
        return obj[field.(string)]
    default:

        r := reflect.ValueOf(obj)

        switch r.Kind().String() {
        case "slice":
            ifield,invalid:=GetAsInt(field)
            if !invalid {
                if ifield<0 {
                    panic(fmt.Errorf("out-of-bounds access to sub-script %d in %T",ifield,obj))
                }
                switch obj:=obj.(type) {
                case []int:
                    if len(obj)>ifield { return obj[ifield] }
                case []bool:
                    if len(obj)>ifield { return obj[ifield] }
                case []uint:
                    if len(obj)>ifield { return obj[ifield] }
                case []string:
                    if len(obj)>ifield { return obj[ifield] }
                case string:
                    if len(obj)>ifield { return obj[ifield] }
                case []float64:
                    if len(obj)>ifield { return obj[ifield] }
                case []*big.Int:
                    if len(obj)>ifield { return obj[ifield] }
                case []*big.Float:
                    if len(obj)>ifield { return obj[ifield] }
                case []dirent:
                    if len(obj)>ifield { return obj[ifield] }
                case []alloc_info:
                    if len(obj)>ifield { return obj[ifield] }
                case [][]int:
                    if len(obj)>ifield { return obj[ifield] }
                case []any:
                    if len(obj)>ifield { return obj[ifield] }
                default:
                    panic(fmt.Errorf("unhandled type %T in array access.",obj))
                }
                panic(fmt.Errorf("out-of-bounds access to sub-script %d in %T",ifield,obj))
            } else {
                panic(fmt.Errorf("array sub-script '%v' must be a number",field))
            }

        }

    } // end default case

    return nil

}

func slice(v any, from, to any) any {
    str, isStr := v.(string)
    isArr:=false
    var arl int
    switch v.(type) {
    case []bool:
        isArr=true
        arl= len(v.([]bool))
    case []int:
        isArr=true
        arl= len(v.([]int))
    case []float64:
        isArr=true
        arl= len(v.([]float64))
    case []*big.Int:
        isArr=true
        arl= len(v.([]*big.Int))
    case []*big.Float:
        isArr=true
        arl= len(v.([]*big.Float))
    case []uint:
        isArr=true
        arl= len(v.([]uint))
    case []string:
        isArr=true
        arl= len(v.([]string))
    case string:
        isArr=true
        arl= len(v.(string))
    case [][]int:
        isArr=true
        arl= len(v.([][]int))
    case []any:
        isArr=true
        arl= len(v.([]any))
    case int, uint, int64, uint64, uint8, float64, *big.Int, *big.Float:
        // clamp operator
        if from==nil && to!=nil { // only expressing upper limit
            return num_min(v,to)
        }
        if to==nil && from!=nil { // only expressing lower limit
            return num_max(v,from)
        }
        if from==nil && to==nil {
            return v
        }
        return num_max(from,num_min(v,to))
    default:
        panic(fmt.Errorf("syntax error: unknown array type '%T'",v))
    }

    if !isStr && !isArr {
        panic(fmt.Errorf("syntax error: slicing requires an array or string, but was %s", typeOf(v)))
    }

    var fromInt, toInt int
    if from == nil {
        fromInt = 0
    } else {
        fromInt = as_integer(from)
    }

    if to == nil && isStr {
        toInt = len(str)
    } else if to == nil && isArr {
        toInt = arl
    } else {
        toInt = as_integer(to)
    }

    if fromInt < 0 {
        panic(fmt.Errorf("range error: start-index %d is negative", fromInt))
    }

    if isStr {
        if toInt < 0 || toInt > len(str) {
            panic(fmt.Errorf("range error: end-index %d is out of range [0, %d]", toInt, len(str)))
        }
        if fromInt > toInt {
            panic(fmt.Errorf("range error: start-index %d is greater than end-index %d", fromInt, toInt))
        }
        return str[fromInt:toInt]
    }

    if toInt < 0 || toInt > arl {
        panic(fmt.Errorf("range error: end-index %d is out of range [0, %d]", toInt, arl))
    }
    if fromInt > toInt {
        panic(fmt.Errorf("range error: start-index %d is greater than end-index %d", fromInt, toInt))
    }

    switch v.(type) {
    case []bool:
        return v.([]bool)[fromInt:toInt]
    case []int:
        return v.([]int)[fromInt:toInt]
    case []float64:
        return v.([]float64)[fromInt:toInt]
    case []*big.Int:
        return v.([]*big.Int)[fromInt:toInt]
    case []*big.Float:
        return v.([]*big.Float)[fromInt:toInt]
    case []uint:
        return v.([]uint)[fromInt:toInt]
    case []string:
        return v.([]string)[fromInt:toInt]
    case string:
        return v.(string)[fromInt:toInt]
    case [][]int:
        return v.([][]int)[fromInt:toInt]
    case []any:
        return v.([]any)[fromInt:toInt]
    }
    return nil
}


func (p *leparser) callFunctionExt(evalfs uint32, ident *[]Variable, name string, method bool, method_value any, kind_override string, arg_names []string, args []any) (res any,hasError bool,method_result any) {

    // pf("(cfe) kind_override -> %s\n",kind_override)

    if f, ok := stdlib[name]; !ok {

        var lmv uint32
        var isFunc bool

        // var fm Funcdef
        // @note: is this still correct? why are we checking for dot here
        /*
        if str.Contains(name,".") {
            fm,isFunc = funcmap[name]
            name=fm.name
            if isFunc { lmv=fm.fs }
        } else {
            lmv, isFunc = fnlookup.lmget(name)
        }
        */
        lmv, isFunc = fnlookup.lmget(name)

        // check if exists in user defined function space
        if isFunc {

            // make Za function call

            // don't lock in space allocator on recursive calls
            var do_lock bool
            evname,_:=numlookup.lmget(evalfs)
            if len(evname)>=len(name) && evname[:len(name)] != name {
                do_lock=true
            }
            // pf("(in call) do_lock %v - name %v - evalfs_name %v\n",do_lock,name,evname)

            loc,_ := GetNextFnSpace(do_lock,name+"@",call_s{prepared:true,base: lmv, caller: evalfs})

            var ident = make([]Variable,identInitialSize)

            var rcount uint8
            rcount,_,method_result=Call(MODE_NEW, &ident, loc, ciEval, method, method_value, kind_override, arg_names, args...)

            // handle the returned result, if present.
            calllock.Lock()
            res = calltable[loc].retvals
            calltable[loc].gcShyness=10
            calltable[loc].gc=true
            calllock.Unlock()

            switch rcount {
            case 0:
                return nil,false,method_result
            case 1:
                switch res.(type) {
                case []any:
                    return res.([]any)[0],false,method_result
                }
                return nil,false,method_result
            default:
                return res,false,method_result
            }

        } else {
            panic(fmt.Errorf("syntax error: no such function %q", name))
        }
    } else {
        // call standard library function
        p.std_call=true

        // hijack kind() calls here
        if name == "kind" {
            res,err := kind(kind_override,args...)
            return res,err!=nil,method_result
        } else {
            // normal stdlib call
            res, err := f(p.namespace,evalfs,ident,args...)
            if err != nil {
                p.std_faulted=true
                p.try_err=err
                pf("%s\n",err)
            }
            return res,err!=nil,method_result
        }
    }
}


