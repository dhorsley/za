
package main

import (
    "io/ioutil"
    "fmt"
    "math"
    "math/big"
    "net/http"
    "unsafe"
    "reflect"
    "strconv"
    str "strings"
)


func typeOf(val interface{}) string {
    if val == nil {
        return "nil"
    }

    kind := reflect.TypeOf(val).Kind()
    if kind.String()=="map" { return "map" }

    switch kind {
    case reflect.Bool:
        return "bool"
    case reflect.Uint, reflect.Int, reflect.Float64:
        return "number"
    case reflect.String:
        return "string"
    default:
        // pf("[ kind %#v ]\n", kind.String())
    }

    if _, ok := val.([]interface{}); ok {
        pf("[ typeOf: array : %+v ]\n",val)
        return "array"
    }

    if _, ok := val.(map[string]interface{}); ok {
        return "object"
    }

    return sf("<unhandled type [%T]>",val)
}

func asBool(val interface{}) (b bool) {
    switch v:=val.(type) {
    case bool:
        b = v
    case string:
        b = v!=""
    case int, int32, int64, uint, uint32, uint64:
        b = v!=0
    case *big.Int:
        b = v.Cmp(GetAsBigInt(0))!=0
    case *big.Float:
        b = v.Cmp(GetAsBigFloat(0))!=0
    default:
            panic(fmt.Errorf("type error: required bool, but was %s", typeOf(v)))
    }
    return b
}

func as_integer(val interface{}) int {
    switch v:=val.(type) {
    case nil:
        return int(0)
    case *big.Float:
        i64,_:=v.Int64()
        return int(i64)
    case *big.Int:
        return int(v.Int64())
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


func ev_range(val1 interface{}, val2 interface{}) ([]int) {

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

func ev_in(val1 interface{}, val2 interface{}) (bool) {
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
    case []interface{}:
        for _, b := range vl { if b == val1 { return true } }
    default:
        panic(fmt.Errorf("IN operator requires a list to search"))
    }
    return false
}


func ev_add(val1 interface{}, val2 interface{}) (r interface{}) {

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
    arr1, arr1OK := val1.([]interface{})
    arr2, arr2OK := val2.([]interface{})
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

    obj1, obj1OK := val1.(map[string]interface{})
    obj2, obj2OK := val2.(map[string]interface{})

    if obj1OK && obj2OK {
        sum := make(map[string]interface{})
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

func ev_sub(val1 interface{}, val2 interface{}) (interface{}) {

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

func ev_mul(val1 interface{}, val2 interface{}) (interface{}) {

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
    if (intInOne && s2ok) && val1.(int)>=0 { var ary []interface{}; for e:=0; e<val1.(int); e+=1 { ary=append(ary,val2) }; return ary }
    if (intInTwo && s1ok) && val2.(int)>=0 { var ary []interface{}; for e:=0; e<val2.(int); e+=1 { ary=append(ary,val1) }; return ary }

    panic(fmt.Errorf("type error: cannot multiply type %T (val:%v) and %T (val:%v)", val1, val1, val2, val2))
}

func ev_div(val1 interface{}, val2 interface{}) (interface{}) {

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
        if val2.(int)==0 { panic(fmt.Errorf("eval error: divide by zero")) }
        return val1.(int) / val2.(int)
    }

    if bf1 || bf2 {
        var r big.Float
        return r.Quo(GetAsBigFloat(val1),GetAsBigFloat(val2))
    }

    if bint1 || bint2 {
        var r big.Int
        return r.Div(GetAsBigInt(val1),GetAsBigInt(val2))
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
    panic(fmt.Errorf("type error: cannot divide type %s and %s", typeOf(val1), typeOf(val2)))
}

func ev_mod(val1 interface{}, val2 interface{}) (interface{}) {

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

func ev_pow(val1 interface{}, val2 interface{}) (interface{}) {

    intInOne:=true; intInTwo:=true
    var int1 int
    var int2 int

    switch i:=val1.(type) {
    case int:
        int1=i
    default:
        intInOne=false
    }
    switch i:=val2.(type) {
    case int:
        int2=i
    default:
        intInTwo=false
    }

    if intInOne && intInTwo {
        return int(math.Pow(float64(int1),float64(int2)))
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

func ev_shift_left(left,right interface{}) (interface{}) {
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

func ev_shift_right(left,right interface{}) (interface{}) {
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

func unaryNegate(val interface{}) (interface{}) {
    switch i:=val.(type) {
    case bool:
        // pf("returning negative\n")
        return !i
    }
    panic(fmt.Errorf("cannot negate a non-bool"))
}

func unaryPlus(val interface{}) (interface{}) {

    var intVal int
    intInOne:=true

    switch i:=val.(type) {
    case int:
        intVal=int(i)
    case int64:
        intVal=int(i)
    case big.Int,big.Float:
        return i
    default:
        intInOne=false
    }

    if intInOne { return intVal }

    floatVal, ok := val.(float64)
    if ok { return floatVal }

    panic(fmt.Errorf("type error: unary positive requires number, but was %s", typeOf(val)))
}

func unaryMinus(val interface{}) (interface{}) {

    var intVal int
    intInOne:=true
    switch i:=val.(type) {
    case int:
        intVal=int(i)
    case int64:
        intVal=int(i)
    case big.Int:
        var r big.Int
        return *r.Neg(GetAsBigInt(i))
    case big.Float:
        var r big.Float
        return *r.Neg(GetAsBigFloat(i))
    default:
        intInOne=false
    }

    if intInOne { return -intVal }

    floatVal, ok := val.(float64)
    if ok { return -floatVal }

    panic(fmt.Errorf("type error: unary minus requires number, but was %s", typeOf(val)))
}


func unaryFileInput(i interface{}) (string) {
    switch i.(type) {
    case string:
        s, err := ioutil.ReadFile(i.(string))
        if err!=nil {
            panic(fmt.Errorf("error importing file '%s' as string",i.(string)))
        }
        if len(s)>0 && s[len(s)-1]==10 { s=s[:len(s)-1] }
        return string(s)
    }
    panic(fmt.Errorf("error importing file as string"))
}


func deepEqual(val1 interface{}, val2 interface{}) (bool) {

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

    case []interface{}:

        typ2, ok := val2.([]interface{})
        if !ok || len(typ1) != len(typ2) {
            return false
        }

        for idx := range typ1 {
            if !deepEqual(typ1[idx], typ2[idx]) {
                return false
            }
        }
        return true

    case map[string]interface{}:
        typ2, ok := val2.(map[string]interface{})
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

    case int64:
        int2, ok := val2.(int)
        if ok {
            return typ1 == int64(int2)
        }
        intsixfour, ok := val2.(int64)
        if ok {
            return typ1 == intsixfour
        }
        float2, ok := val2.(float64)
        if ok {
            return float64(typ1) == float2
        }
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

func compare(val1 interface{}, val2 interface{}, operation string) (bool) {

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

    var bf1,bf2,bi1,bi2 bool
    switch val1.(type) {
    case *big.Float:
        bf1=true
    case *big.Int:
        bi1=true
    }
    switch val2.(type) {
    case *big.Float:
        bf2=true
    case *big.Int:
        bi2=true
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

    str1, str1OK := val1.(string)
    str2, str2OK := val2.(string)

    if str1OK && str2OK {
        return compareString(str1, str2, operation)
    }

    panic(fmt.Errorf("type error: cannot compare type %s and %s", typeOf(val1), typeOf(val2)))
}

func compareString(val1 string, val2 string, operation string) (bool) {
    switch operation {
    case "<":
        return val1 < val2
    case "<=":
        return val1 <= val2
    case ">":
        return val1 > val2
    case ">=":
        return val1 >= val2
    }
    panic(fmt.Errorf("syntax error: unsupported operation %q", operation))
}

func compareInt(val1 int, val2 int, operation string) (bool) {
    switch operation {
    case "<":
        return val1 < val2
    case "<=":
        return val1 <= val2
    case ">":
        return val1 > val2
    case ">=":
        return val1 >= val2
    }
    panic(fmt.Errorf("syntax error: unsupported operation %q", operation))
}

func compareFloat(val1 float64, val2 float64, operation string) (bool) {
    switch operation {
    case "<":
        return val1 < val2
    case "<=":
        return val1 <= val2
    case ">":
        return val1 > val2
    case ">=":
        return val1 >= val2
    }
    panic(fmt.Errorf("syntax error: unsupported operation %q", operation))
}

func compareBigFloat(val1 *big.Float, val2 *big.Float, operation string) (bool) {
    switch operation {
    case "<":
        return val1.Cmp(val2)==-1
    case "<=":
        return val1.Cmp(val2)<1
    case ">":
        return val1.Cmp(val2)==1
    case ">=":
        return val1.Cmp(val2)>-1
    }
    panic(fmt.Errorf("syntax error: unsupported operation %q", operation))
}

func compareBigInt(val1 *big.Int, val2 *big.Int, operation string) (bool) {
    switch operation {
    case "<":
        return val1.Cmp(val2)==-1
    case "<=":
        return val1.Cmp(val2)<1
    case ">":
        return val1.Cmp(val2)==1
    case ">=":
        return val1.Cmp(val2)>-1
    }
    panic(fmt.Errorf("syntax error: unsupported operation %q", operation))
}

func asObjectKey(key interface{}) (string) {
    s, ok := key.(string)
    if !ok {
        panic(fmt.Errorf("type error: object key must be string, but was %s", typeOf(key)))
    }
    return s
}

func (p *leparser) accessFieldOrFunc(obj interface{}, field string) (interface{}) {

    switch obj:=obj.(type) {

    case http.Header:
        r := reflect.ValueOf(obj)
        f := reflect.Indirect(r).FieldByName(field)
        return f

        /*
    case token_result:
        r := reflect.ValueOf(obj)
        f := reflect.Indirect(r).FieldByName(field)
        pf("[TR] : r %#v : f %#v\n",r,f)
        return f
        */

    default:

        r := reflect.ValueOf(obj)

        switch r.Kind() {

        case reflect.Struct:

            // work with mutable copy as we need to make field unsafe
            // further down in switch.

            rcopy := reflect.New(r.Type()).Elem()
            rcopy.Set(r)

            // get the required struct field and make a r/w copy
            f := rcopy.FieldByName(field)

            if f.IsValid() {

                switch f.Type().Kind() {
                case reflect.String:
                    return f.String()
                case reflect.Bool:
                    return f.Bool()
                case reflect.Int:
                    return int(f.Int())
                case reflect.Int64:
                    return int(f.Int())
                case reflect.Float64:
                    return f.Float()
                case reflect.Uint:
                    return uint(f.Uint())
                case reflect.Uint8:
                    return uint8(f.Uint())
                case reflect.Uint32:
                    return uint32(f.Uint())
                case reflect.Uint64:
                    return uint64(f.Uint())

                case reflect.Slice:

                    f = reflect.NewAt(f.Type(), unsafe.Pointer(f.UnsafeAddr())).Elem()
                    slc:=f.Slice(0,f.Len())

                    switch f.Type().Elem().Kind() {
                    case reflect.Interface,reflect.String:
                        return slc.Interface()
                    default:
                        return []interface{}{}
                    }

                case reflect.Interface:
                    return f.Interface()

                default:
                    f = reflect.NewAt(f.Type(), unsafe.Pointer(f.UnsafeAddr())).Elem()
                    return f.Interface()
                }
            }

        default:

            // try a function call..
            // lhs_v would become the first argument of func lhs_f

            var isFunc bool
            name:=field

            // parse the function call as module '.' funcname
            nonlocal:=false
            var there bool

            if _,there=funcmap[p.preprev.tokText+"."+name] ; there {
                nonlocal=true
                name=p.preprev.tokText+"."+name
                isFunc=true
            }

            // check if stdlib or user-defined function
            if !isFunc {
                if _, isFunc = stdlib[name]; !isFunc {
                    isFunc = fnlookup.lmexists(name)
                }
            }

            if !isFunc {
                // before failing, check if this is a valid enum reference
                globlock.RLock()
                defer globlock.RUnlock()
                if enum[p.preprev.tokText]!=nil {
                    return enum[p.preprev.tokText].members[name]
                }
                // pf("\n\nobject: %v\n\n",obj)
                panic(fmt.Errorf("no function, enum or record field found for %v", field))
            }

            // user-defined or stdlib call 

            var iargs []interface{}
            if !nonlocal {
                iargs=[]interface{}{obj}
            }

            if p.peek().tokType==LParen {
                p.next()
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
            }

            return callFunction(p.fs,p.ident,name,iargs)

        }

    }

    return nil
}


func accessArray(ident *[szIdent]Variable, obj interface{}, field interface{}) (interface{}) {

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
    case map[string]interface{}:
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
                case []interface{}:
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

func slice(v interface{}, from, to interface{}) interface{} {
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
    case []interface{}:
        isArr=true
        arl= len(v.([]interface{}))
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
    case []interface{}:
        return v.([]interface{})[fromInt:toInt]
    }
    return nil
}


func callFunction(evalfs uint32, ident *[szIdent]Variable, name string, args []interface{}) (res interface{}) {

    /*
    pf("callFunction started with\nfs %v fn %v\n",evalfs,name)
    pf("+ident of : %v\n",*ident)
    */

    /* // test removal - should probably not be interpolating arguments
    for a:=0; a<len(args); a++ {
        switch args[a].(type) {
        case string:
            args[a]=interpolate(evalfs,args[a].(string))
        }
    }
    */

    if f, ok := stdlib[name]; !ok {

        var lmv uint32
        var isFunc bool
        var fm Funcdef

        if str.Contains(name,".") {
            fm,isFunc = funcmap[name]
            name=fm.name
            if isFunc { lmv=fm.fs }
        } else {
            lmv, isFunc = fnlookup.lmget(name)
        }

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

            var ident [szIdent]Variable

            rcount,_:=Call(MODE_NEW, &ident, loc, ciEval, args...)

            // handle the returned result, if present.

            calllock.Lock()
            res = calltable[loc].retvals
            calltable[loc].gcShyness=100
            calltable[loc].gc=true
            calllock.Unlock()

            switch rcount {
            case 0:
                return nil
            case 1:
                return res.([]interface{})[0]
            default:
                return res
            }
            return res
        } else {
            panic(fmt.Errorf("syntax error: no such function %q", name))
        }
    } else {
        // call standard library function
        res, err := f(evalfs,ident,args...)
        if err != nil {
            msg:=sf("function error: in %+v %s",name,err)
            if err.Error()!="" { msg=err.Error() }
            panic(fmt.Errorf(msg))
        }
        return res
    }
}


