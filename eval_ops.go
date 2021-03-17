
package main

import (
    "io/ioutil"
    "fmt"
    "math"
    "net/http"
    "unsafe"
    "reflect"
    "strconv"
    str "strings"
//    "sync/atomic"
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
        pf("[ kind %#v ]\n", kind.String())
    }

    if _, ok := val.([]interface{}); ok {
        return "array"
    }

    if _, ok := val.(map[string]interface{}); ok {
        return "object"
    }

    return "<unknown type>"
}

func asBool(val interface{}) bool {
    b, ok := val.(bool)
    if !ok {
        panic(fmt.Errorf("type error: required bool, but was %s", typeOf(val)))
    }
    return b
}

func as_integer(val interface{}) int {
    switch v:=val.(type) {
    case nil:
        return int(0)
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

    return nil

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
    case []interface{}:
        for _, b := range vl { if b == val1 { return true } }
    default:
        panic(fmt.Errorf("IN operator requires a list to search"))
    }
    return false
}


func ev_add(val1 interface{}, val2 interface{}) (interface{}) {

    var intInOne bool
    var intInTwo bool

    switch val1.(type) {
    case int:
        intInOne=true
    }
    switch val2.(type) {
    case int:
        intInTwo=true
    }

    if intInOne && intInTwo {
        return val1.(int)+val2.(int)
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

    if float1OK && float2OK {
        return float1 + float2
    }

    if intInOne && val2==nil { return val1.(int) }
    if intInTwo && val1==nil { return val2.(int) }

    str1, str1OK := val1.(string)
    str2, str2OK := val2.(string)

    if str1OK && str2OK { // string + string = string
        return str1 + str2
    }

    if str1OK && float2OK {
        return str1 + strconv.FormatFloat(float2, 'f', -1, 64)
    }
    if float1OK && str2OK {
        return strconv.FormatFloat(float1, 'f', -1, 64) + str2
    }

    if str1OK && val2 == nil {
        return str1 + "nil"
    }
    if val1 == nil && str2OK {
        return "nil" + str2
    }

    bool1, bool1OK := val1.(bool)
    bool2, bool2OK := val2.(bool)

    if str1OK && bool2OK {
        return str1 + strconv.FormatBool(bool2)
    }
    if bool1OK && str2OK {
        return strconv.FormatBool(bool1) + str2
    }

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

    intInOne:=true; intInTwo:=true

    switch val1.(type) {
    case int:
    default:
        intInOne=false
    }
    switch val2.(type) {
    case int:
    default:
        intInTwo=false
    }

    if intInOne && intInTwo {
        return val1.(int) - val2.(int)
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

    if float1OK && float2OK {
        return float1 - float2
    }
    panic(fmt.Errorf("type error: cannot subtract type %s and %s", typeOf(val1), typeOf(val2)))
}

func ev_mul(val1 interface{}, val2 interface{}) (interface{}) {

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
        return int1 * int2
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
        return float1 * float2
    }

    // int * string = repeat
    str1, str1OK := val1.(string)
    str2, str2OK := val2.(string)
    if (intInOne && str2OK) && int1>=0 { return str.Repeat(str2,int1) }
    if (intInTwo && str1OK) && int2>=0 { return str.Repeat(str1,int2) }

    // int * struct = repeat
    s1ok := reflect.ValueOf(val1).Kind() == reflect.Struct
    s2ok := reflect.ValueOf(val2).Kind() == reflect.Struct
    if (intInOne && s2ok) && int1>=0 { var ary []interface{}; for e:=0; e<int1; e++ { ary=append(ary,val2) }; return ary }
    if (intInTwo && s1ok) && int2>=0 { var ary []interface{}; for e:=0; e<int2; e++ { ary=append(ary,val1) }; return ary }

    panic(fmt.Errorf("type error: cannot multiply type %s and %s", typeOf(val1), typeOf(val2)))
}

func ev_div(val1 interface{}, val2 interface{}) (interface{}) {

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
        if int2==0 { panic(fmt.Errorf("eval error: I'm afraid I can't do that! (divide by zero)")) }
        return int1 / int2
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
        if float2==0 { panic(fmt.Errorf("eval error: I'm afraid I can't do that! (divide by zero)")) }
        return float1 / float2
    }
    panic(fmt.Errorf("type error: cannot divide type %s and %s", typeOf(val1), typeOf(val2)))
}

func ev_mod(val1 interface{}, val2 interface{}) (interface{}) {

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
        return int1 % int2
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

    // special case for ptr nil
    var nilcmp1,nilcmp2 bool
    switch v1:=val1.(type) {
    case []string:
        if len(v1)==2 && v1[0]=="nil" && v1[1]=="nil" {
            nilcmp1=true
            val1=nil
        }
    }
    switch v2:=val2.(type) {
    case []string:
        if len(v2)==2 && v2[0]=="nil" && v2[1]=="nil" {
            nilcmp2=true
            val2=nil
        }
    }
    if nilcmp1 || nilcmp2 {
        if val1==val2 {
            return true
        }
    }
    // end bodge

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
    panic(fmt.Errorf("type error: cannot compare type %s and %s", typeOf(val1), typeOf(val2)))
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

func asObjectKey(key interface{}) (string) {
    s, ok := key.(string)
    if !ok {
        panic(fmt.Errorf("type error: object key must be string, but was %s", typeOf(key)))
    }
    return s
}

func addMapMember(evalfs uint32, obj string, key, val interface{}) {
    // map key
    s := asObjectKey(key)
    vsetElement(evalfs, obj, s, val)
    return
}

func addObjectMember(evalfs uint32, obj string, key interface{}, val interface{}) {
    // normal array
    s,invalid := GetAsInt(key.(string))
    if invalid { panic(fmt.Errorf("type error: element must be an integer")) }

    switch val.(type) {
    case map[string]interface{},map[string]string,int, float64, bool, interface{}:
        vsetElement(evalfs, obj, s, val)
    default:
        panic(fmt.Errorf("addobjmember cannot handle type %T for %v\n",val,key))
    }
    return
}


func (p *leparser) accessFieldOrFunc(obj interface{}, field string) (interface{}) {

    // evalfs:=p.fs

    switch obj:=obj.(type) {

    case http.Header:
        r := reflect.ValueOf(obj)
        f := reflect.Indirect(r).FieldByName(field)
        return f

    case token_result:
        r := reflect.ValueOf(obj)
        f := reflect.Indirect(r).FieldByName(field)
        return f

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

                    f  = reflect.NewAt(f.Type(), unsafe.Pointer(f.UnsafeAddr())).Elem()
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
                    // pf("default type in accessField is [%+v]",f.Type().Name())
                    return f.Interface()
                }
            }

        default:

            // @todo: we should probably remove this outside of the switch,
            //  so that we can handle function chaining for structs as like other
            //  value types. It would mean massaging the code above a little too
            //  to allow unhandled cases to fall through and skip chaining if
            //  already handled above.
            //  We would need to do this anyway if we ever add struct methods.

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

            return callFunction(p.fs,p.line,name,iargs)

        }

    }

    return nil
}


func accessArray(evalfs uint32, obj interface{}, field interface{}) (interface{}) {

    switch obj:=obj.(type) {
    case string:
        vg,_:=vgetElement(evalfs,obj,strconv.Itoa(field.(int)))
        return vg
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
            switch obj:=obj.(type) {
            case []int:
                if len(obj)>field.(int) { return obj[field.(int)] }
            case []bool:
                if len(obj)>field.(int) { return obj[field.(int)] }
            case []uint:
                if len(obj)>field.(int) { return obj[field.(int)] }
            case []string:
                if len(obj)>field.(int) { return obj[field.(int)] }
            case string:
                if len(obj)>field.(int) { return obj[field.(int)] }
            case []float64:
                if len(obj)>field.(int) { return obj[field.(int)] }
            case []dirent:
                if len(obj)>field.(int) { return obj[field.(int)] }
            case []alloc_info:
                if len(obj)>field.(int) { return obj[field.(int)] }
            case []interface{}:
                if len(obj)>field.(int) { return obj[field.(int)] }
            default:
                panic(fmt.Errorf("unhandled type %T in array access.",obj))
            }

            panic(fmt.Errorf("element '%d' is out of range",field.(int)))

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
    case []uint:
        isArr=true
        arl= len(v.([]uint))
    case []string:
        isArr=true
        arl= len(v.([]string))
    case string:
        isArr=true
        arl= len(v.(string))
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
    case []uint:
        return v.([]uint)[fromInt:toInt]
    case []string:
        return v.([]string)[fromInt:toInt]
    case string:
        return v.(string)[fromInt:toInt]
    case []interface{}:
        return v.([]interface{})[fromInt:toInt]
    }
    return nil
}


func callFunction(evalfs uint32, callline int16, name string, args []interface{}) (res interface{}) {

    // pf("callFunction started with\nfs %v line %v fn %v\n",evalfs,callline,name)

    for a:=0; a<len(args); a++ {
        switch args[a].(type) {
        case string:
            args[a]=interpolate(evalfs,args[a].(string))
        }
    }

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
            loc,id := GetNextFnSpace(name+"@")

            calllock.Lock()
            calltable[loc] = call_s{fs: id, base: lmv, caller: evalfs, callline: callline, retvar: "@#"}
            calllock.Unlock()

            // atomic.AddInt32(&concurrent_funcs, 1)
            rcount,_:=Call(MODE_NEW, loc, ciEval, args...)
            // atomic.AddInt32(&concurrent_funcs, -1)

            // handle the returned result, if present.
            res, _ = vget(evalfs, "@#")
            switch rcount {
            case 0:
                return nil
            case 1:
                return res.([]interface{})[0]
            default:
                return res
            }

        } else {
            panic(fmt.Errorf("syntax error: no such function %q", name))
        }
    } else {
        // call standard library function
        res, err := f(evalfs,args...)
        if err != nil {
            msg:=sf("function error: in %+v %s",name,err)
            if err.Error()!="" { msg=err.Error() }
            panic(fmt.Errorf(msg))
        }
        return res
    }
}


