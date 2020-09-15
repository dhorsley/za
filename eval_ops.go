
package main

import (
    "errors"
    "fmt"
    "math"
    "net/http"
    "unsafe"
//    "regexp"
    "reflect"
    "strconv"
//    "strings"
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
    case reflect.Int, reflect.Float64:
        return "number"
    case reflect.String:
        return "string"
    default:
        // pf("[ kind %#v ]\n", kind.String())
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

func asInteger(val interface{}) int {
    i, ok := val.(int)
    if ok {
        return i
    }
    f, ok := val.(float64)
    if !ok {
        panic(fmt.Errorf("type error: required number of type integer, but was %s", typeOf(val)))
    }

    i = int(f)
    return i
}


func ev_add(val1 interface{}, val2 interface{}) (interface{}) {

    intInOne:=true; intInTwo:=true
    var int1 int
    var int2 int

    switch val1.(type) {
    case int:
        int1=val1.(int)
    case int32:
        int1=int(val1.(int32))
    case int64:
        int1=int(val1.(int64))
    default:
        intInOne=false
    }
    switch val2.(type) {
    case int:
        int2=val2.(int)
    case int32:
        int2=int(val2.(int32))
    case int64:
        int2=int(val2.(int64))
    default:
        intInTwo=false
    }

    if intInOne && intInTwo {
        return int1+int2
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
		return float1 + float2
	}

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

	if arr1OK && arr2OK {
		return append(arr1, arr2...)
	}

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
    var int1 int
    var int2 int

    switch i:=val1.(type) {
    case int:
        int1=i
    case int32:
        int1=int(i)
    case int64:
        int1=int(i)
    default:
        intInOne=false
    }
    switch i:=val2.(type) {
    case int:
        int2=i
    case int32:
        int2=int(i)
    case int64:
        int2=int(i)
    default:
        intInTwo=false
    }

	if intInOne && intInTwo {
		return int1 - int2
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
    case int32:
        int1=int(i)
    case int64:
        int1=int(i)
    default:
        intInOne=false
    }
    switch i:=val2.(type) {
    case int:
        int2=i
    case int32:
        int2=int(i)
    case int64:
        int2=int(i)
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
	panic(fmt.Errorf("type error: cannot multiply type %s and %s", typeOf(val1), typeOf(val2)))
}

func ev_div(val1 interface{}, val2 interface{}) (interface{}) {

    intInOne:=true; intInTwo:=true
    var int1 int
    var int2 int

    switch i:=val1.(type) {
    case int:
        int1=i
    case int32:
        int1=int(i)
    case int64:
        int1=int(i)
    default:
        intInOne=false
    }
    switch i:=val2.(type) {
    case int:
        int2=i
    case int32:
        int2=int(i)
    case int64:
        int2=int(i)
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
    case int32:
        int1=int(i)
    case int64:
        int1=int(i)
    default:
        intInOne=false
    }
    switch i:=val2.(type) {
    case int:
        int2=i
    case int32:
        int2=int(i)
    case int64:
        int2=int(i)
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
    case int32:
        int1=int(i)
    case int64:
        int1=int(i)
    default:
        intInOne=false
    }
    switch i:=val2.(type) {
    case int:
        int2=i
    case int32:
        int2=int(i)
    case int64:
        int2=int(i)
    default:
        intInTwo=false
    }

	if intInOne && intInTwo {
		return math.Pow(float64(int1),float64(int2))
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
    var uint1 uint64
    var int1 int64
    var uint2 uint64

    switch i:=left.(type) {
    case int,int32,int64:
        int1,_=GetAsInt64(i)
    case uint,uint8,uint32,uint64:
        uint1,_=GetAsUint(i)
        uintInOne=true
    default:
        uintInOne=false
        intInOne=false
    }
    switch i:=right.(type) {
    case uint,uint8,uint32,uint64,int,int32,int64:
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
    var uint1 uint64
    var int1 int64
    var uint2 uint64

    switch i:=left.(type) {
    case int,int32,int64:
        int1,_=GetAsInt64(i)
    case uint,uint8,uint32,uint64:
        uint1,_=GetAsUint(i)
        uintInOne=true
    default:
        uintInOne=false
        intInOne=false
    }
    switch i:=right.(type) {
    case uint,uint8,uint32,uint64,int,int32,int64:
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

    panic("shift operations only work with integers")
}

func unaryNegate(val interface{}) (interface{}) {
    switch i:=val.(type) {
    case bool:
        return !i
    }
    panic("cannot negate a non-bool")
}

func unaryMinus(val interface{}) (interface{}) {

    var intVal int
    intInOne:=true
    switch i:=val.(type) {
    case int:
        intVal=int(i)
    case int32:
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


func deepEqual(val1 interface{}, val2 interface{}) (bool) {
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

func addMapMember(evalfs uint64, obj string, key, val interface{}) {
    // map key
	s := asObjectKey(key)
	vsetElement(evalfs, obj, s, val)
    return
}

func addObjectMember(evalfs uint64, obj string, key interface{}, val interface{}) {
    // normal array
	s,invalid := GetAsInt(key.(string))
    if invalid { panic(fmt.Errorf("type error: element must be an integer")) }

    switch val.(type) {
    case map[string]interface{},map[string]string,int, float64, bool, interface{}:
        vsetElement(evalfs, obj, sf("%v",s), val)
    default:
        panic(fmt.Errorf("addobjmember cannot handle type %T for %v\n",val,key))
    }
	return
}


func convertToInt(ar interface{}) []int {
    var v interface{}
    var i int
    switch ar:=ar.(type) {
    case []float32:
        newar := make([]int, len(ar))
        for i, v = range ar { newar[i],_ = GetAsInt(v) }
        return newar
    case []float64:
        newar := make([]int, len(ar))
        for i, v = range ar { newar[i],_ = GetAsInt(v) }
        return newar
    case []int32:
        newar := make([]int, len(ar))
        for i, v = range ar { newar[i],_ = GetAsInt(v) }
        return newar
    case []int:
        newar := make([]int, len(ar))
        for i, v = range ar { newar[i],_ = GetAsInt(v) }
        return newar
    case []int64:
        newar := make([]int, len(ar))
        for i, v = range ar { newar[i],_ = GetAsInt(v) }
        return newar
    default:
        panic(fmt.Errorf("type error: cannot convert from %T to int",ar))
    }
}


func convertToFloat64(ar interface{}) []float64 {
    var v interface{}
    var i int
    switch ar:=ar.(type) {
    case []float32:
        newar := make([]float64, len(ar))
        for i, v = range ar { newar[i],_ = GetAsFloat(v) }
        return newar
    case []float64:
        newar := make([]float64, len(ar))
        for i, v = range ar { newar[i],_ = GetAsFloat(v) }
        return newar
    case []int32:
        newar := make([]float64, len(ar))
        for i, v = range ar { newar[i],_ = GetAsFloat(v) }
        return newar
    case []int:
        newar := make([]float64, len(ar))
        for i, v = range ar { newar[i],_ = GetAsFloat(v) }
        return newar
    case []int64:
        newar := make([]float64, len(ar))
        for i, v = range ar { newar[i],_ = GetAsFloat(v) }
        return newar
    default:
		panic(fmt.Errorf("type error: cannot convert from %T to float",ar))
    }
}

func accessVar(evalfs uint64, varName string) (val interface{},found bool) {
	if val, found = vget(evalfs, varName); !found {
        return val,true
    }
    return val,false
}

func accessField(evalfs uint64, obj interface{}, field interface{}) (interface{}) {
    // pf("gv-af: entered accessField with [%T] %v -> [%T], %v\n",obj,obj,field,field)
    var ifield string

    switch field.(type) {
    case string:
        ifield=field.(string)
    case int:
        ifield=sf("%v",field)
    default:
        // pf("field -> [%T] %+v\n",field)
    }

	// types
    switch obj:=obj.(type) {
    case string:
        vg,_:=vgetElement(evalfs,obj,ifield)
        return vg
	case map[string]string:
		return obj[ifield]
	case map[string]float64:
		return obj[ifield]
	case map[string]int:
		return obj[ifield]
	case map[string]uint8:
		return obj[ifield]
	case map[string]interface{}:
		return obj[ifield]
    case http.Header:
        r := reflect.ValueOf(obj)
        f := reflect.Indirect(r).FieldByName(ifield)
        return f
    case webstruct:
        r := reflect.ValueOf(obj)
        f := reflect.Indirect(r).FieldByName(ifield)
        return f
    default:

        r := reflect.ValueOf(obj)

        switch r.Kind().String() {
        case "slice":
	        idx, invalid := GetAsInt(ifield)
	        if invalid || idx<0 {
		        panic(fmt.Errorf("var error: not a valid element index. (ifield:%v)",ifield))
	        }
		    switch obj:=obj.(type) {
            case []interface{}:
                if len(obj)>idx { return obj[idx] }
            case []int:
                if len(obj)>idx { return obj[idx] }
            case []bool:
                if len(obj)>idx { return obj[idx] }
            case []int64:
                if len(obj)>idx { return obj[idx] }
            case []int32:
                if len(obj)>idx { return obj[idx] }
            case []uint:
                if len(obj)>idx { return obj[idx] }
            case []uint8:
                if len(obj)>idx { return obj[idx] }
            case []uint32:
                if len(obj)>idx { return obj[idx] }
            case []uint64:
                if len(obj)>idx { return obj[idx] }
            case []string:
                if len(obj)>idx { return obj[idx] }
            case []float64:
                if len(obj)>idx { return obj[idx] }
            default:
                panic(fmt.Errorf("unhandled type %T in array access.",obj))
            }

            panic(errors.New(sf("element '%d' is out of range in '%v'",idx,obj)))

        case "struct":

            // work with mutable copy as we need to make field unsafe
            // further down in switch.

            rcopy := reflect.New(r.Type()).Elem()
            rcopy.Set(r)

            // get the required struct field and make a r/w copy
            f := rcopy.FieldByName(ifield)

            if f.IsValid() {

                if rcopy.Type().AssignableTo(f.Type()) {
                    f=reflect.NewAt(f.Type(),unsafe.Pointer(f.UnsafeAddr())).Elem()
                }

                switch f.Type().Kind() {
                case reflect.String:
                    return f.String()
                case reflect.Bool:
                    return f.Bool()
                case reflect.Int:
                    return f.Int()
                case reflect.Int32:
                    return int32(f.Int())
                case reflect.Int64:
                    return int64(f.Int())
                case reflect.Float32:
                    return float32(f.Float())
                case reflect.Float64:
                    return f.Float()
                case reflect.Uint:
                    return uint(f.Uint())
                case reflect.Uint8:
                    return uint8(f.Uint())
                case reflect.Uint16:
                    return uint16(f.Uint())
                case reflect.Uint32:
                    return uint32(f.Uint())
                case reflect.Uint64:
                    return uint64(f.Uint())

                case reflect.Slice:

                    f  = reflect.NewAt(f.Type(), unsafe.Pointer(f.UnsafeAddr())).Elem()
                    slc:=f.Slice(0,f.Len())

                    switch f.Type().Elem().Kind() {
                    case reflect.Interface:
                        return slc.Interface()
                    default:
                        return []interface{}{}
                    }

                case reflect.Interface:
                    return f.Interface()
                default:
                    pf("default type in accessField is [%+v]",f.Type().Name())
                    return f.Interface()
                }
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
    case []int32:
        isArr=true
        arl= len(v.([]int32))
    case []int64:
        isArr=true
        arl= len(v.([]int64))
    case []float32:
        isArr=true
        arl= len(v.([]float32))
    case []float64:
        isArr=true
        arl= len(v.([]float64))
    case []uint:
        isArr=true
        arl= len(v.([]uint))
    case []uint8:
        isArr=true
        arl= len(v.([]uint8))
    case []uint32:
        isArr=true
        arl= len(v.([]uint32))
    case []uint64:
        isArr=true
        arl= len(v.([]uint64))
    case []string:
        isArr=true
        arl= len(v.([]string))
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
		fromInt = asInteger(from)
	}

	if to == nil && isStr {
		toInt = len(str)
	} else if to == nil && isArr {
		toInt = arl
	} else {
		toInt = asInteger(to)
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
    case []int32:
	    return v.([]int32)[fromInt:toInt]
    case []int64:
	    return v.([]int64)[fromInt:toInt]
    case []float32:
	    return v.([]float32)[fromInt:toInt]
    case []float64:
	    return v.([]float64)[fromInt:toInt]
    case []uint:
	    return v.([]uint)[fromInt:toInt]
    case []uint8:
	    return v.([]uint8)[fromInt:toInt]
    case []uint32:
	    return v.([]uint32)[fromInt:toInt]
    case []uint64:
	    return v.([]uint64)[fromInt:toInt]
    case []string:
	    return v.([]string)[fromInt:toInt]
    case []interface{}:
	    return v.([]interface{})[fromInt:toInt]
    }
    return nil
}

func arrayContains(arr interface{}, val interface{}) bool {
	a, ok := arr.([]interface{})
	if !ok {
		panic(fmt.Errorf("syntax error: in-operator requires array, but was %s", typeOf(arr)))
	}

	for _, v := range a {
		if deepEqual(v, val) {
			return true
		}
	}
	return false
}

func callFunction(evalfs uint64, callline int, name string, args []interface{}) (res interface{}) {

	if f, ok := stdlib[name]; !ok {

		// check if exists in user defined function space
		if lmv, isFunc := fnlookup.lmget(name); isFunc {

			// make Za function call
            loc,id := GetNextFnSpace(name+"@")

            if lockSafety { calllock.Lock() }
			calltable[loc] = call_s{fs: id, base: lmv, caller: evalfs, callline: callline, retvar: "@#"}
            if lockSafety { calllock.Unlock() }

			Call(MODE_NEW, loc, ciEval, args...)

			// handle the returned result, if present.
            res, _ = vget(evalfs, "@#")
            return res

		} else {
		    // no? now panic
		    panic(fmt.Errorf("syntax error: no such function %q", name))
        }
	} else {
        // call standard function
        res, err := f(evalfs,args...)
        if err != nil {
            panic(fmt.Errorf("function error: %s", err))
        }
        return res
    }
}


