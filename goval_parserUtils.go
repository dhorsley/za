package main

import (
	"fmt"
	"math"
	"net/http"
    "reflect"
	"regexp"
	"strconv"
	"strings"
//    "sync"
)

func init() {
	yyErrorVerbose = false
	// yyErrorVerbose = true // make sure to get better errors than just "syntax error"
}

// ExpressionFunction can be called from within expressions.
// The returned object needs to have one of the following types: `nil`, `bool`, `int`, `float64`, `string`, `[]interface{}` or `map[string]interface{}`.
// type ExpressionFunction = func(evalfs uint64,args ...interface{}) (interface{}, error)

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

func add(val1 interface{}, val2 interface{}) interface{} {

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
        return int1+int2
    }
    /*
	int1, int1OK := val1.(int)
	int2, int2OK := val2.(int)

	if int1OK && int2OK { // int + int = int
		return int1 + int2
	}
    */

	float1, float1OK := val1.(float64)
	float2, float2OK := val2.(float64)

	// if int1OK {
	if intInOne {
		float1 = float64(int1)
		float1OK = true
	}
	// if int2OK {
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

func sub(val1 interface{}, val2 interface{}) interface{} {
	//int1, int1OK := val1.(int)
	//int2, int2OK := val2.(int)

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

	// if int1OK && int2OK {
	if intInOne && intInTwo {
		return int1 - int2
	}

	float1, float1OK := val1.(float64)
	float2, float2OK := val2.(float64)

	// if int1OK {
	if intInOne {
		float1 = float64(int1)
		float1OK = true
	}
	// if int2OK {
	if intInTwo {
		float2 = float64(int2)
		float2OK = true
	}

	if float1OK && float2OK {
		return float1 - float2
	}
	panic(fmt.Errorf("type error: cannot subtract type %s and %s", typeOf(val1), typeOf(val2)))
}

func mul(val1 interface{}, val2 interface{}) interface{} {
	//int1, int1OK := val1.(int)
	//int2, int2OK := val2.(int)

	//if int1OK && int2OK {
	//	return int1 * int2
	//}

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

	//if int1OK && int2OK {
	if intInOne && intInTwo {
		return int1 * int2
    }

	float1, float1OK := val1.(float64)
	float2, float2OK := val2.(float64)

	// if int1OK {
	if intInOne {
		float1 = float64(int1)
		float1OK = true
	}
	// if int2OK {
	if intInTwo {
		float2 = float64(int2)
		float2OK = true
	}

	if float1OK && float2OK {
		return float1 * float2
	}
	panic(fmt.Errorf("type error: cannot multiply type %s and %s", typeOf(val1), typeOf(val2)))
}

func div(val1 interface{}, val2 interface{}) interface{} {
	//int1, int1OK := val1.(int)
	//int2, int2OK := val2.(int)

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

	// if int1OK && int2OK {
	if intInOne && intInTwo {
        if int2==0 { panic(fmt.Errorf("eval error: I'm afraid I can't do that! (divide by zero)")) }
		return int1 / int2
	}

	float1, float1OK := val1.(float64)
	float2, float2OK := val2.(float64)

	// if int1OK {
	if intInOne {
		float1 = float64(int1)
		float1OK = true
	}
	// if int2OK {
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

func mod(val1 interface{}, val2 interface{}) interface{} {
	//int1, int1OK := val1.(int)
	//int2, int2OK := val2.(int)

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

	// if int1OK && int2OK {
	if intInOne && intInTwo {
		return int1 % int2
	}

	float1, float1OK := val1.(float64)
	float2, float2OK := val2.(float64)

	// if int1OK {
	if intInOne {
		float1 = float64(int1)
		float1OK = true
	}
	// if int2OK {
	if intInTwo {
		float2 = float64(int2)
		float2OK = true
	}

	if float1OK && float2OK {
		return math.Mod(float1, float2)
	}
	panic(fmt.Errorf("type error: cannot perform modulo on type %s and %s", typeOf(val1), typeOf(val2)))
}

func unaryMinus(val interface{}) interface{} {

    var intVal int
    intInOne:=true
    switch i:=val.(type) {
    case int:
        intVal=i
    case int32:
        intVal=int(i)
    case int64:
        intVal=int(i)
    default:
        intInOne=false
    }

    // intVal, ok := val.(int)
	if intInOne { return -intVal }

    floatVal, ok := val.(float64)
	if ok { return -floatVal }

	panic(fmt.Errorf("type error: unary minus requires number, but was %s", typeOf(val)))
}

func deepEqual(val1 interface{}, val2 interface{}) bool {
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

func compare(val1 interface{}, val2 interface{}, operation string) bool {
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

func compareInt(val1 int, val2 int, operation string) bool {
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

func compareFloat(val1 float64, val2 float64, operation string) bool {
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

func asObjectKey(key interface{}) string {
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
        pf("addobjmember cannot handle type %T for %v\n",val,key)
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

func accessVar(evalfs uint64, varName string) (interface{},bool) {
    var EvalFail bool
    EvalFail=false
	val, found := vget(evalfs, varName)
	if !found {
        // pf("ERROR: Could not find variable '%s'\n",varName)
        EvalFail=true
    }
    return val,EvalFail
}

func accessField(evalfs uint64, obj interface{}, field interface{}) interface{} {

    var ifield string

    switch field.(type) {
    case string:
        ifield=field.(string)
    case int:
        ifield=sf("%v",field)
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
        if reflect.ValueOf(r).Kind().String()=="struct" {
            f := reflect.Indirect(r).FieldByName(ifield)
            switch f.Type().Kind() {
            case reflect.Bool:
                return f.Bool()
            case reflect.Int:
                return f.Int()
            case reflect.Int32:
                return int32(f.Int())
            case reflect.Int64:
                return int64(f.Int())
            case reflect.String:
                return f.String()
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
            default:
                return f.Interface()
            }
        } else {
            pf("unknown type in accessField: %T for obj %+v\n",obj,obj)
        }
	}

	idx, invalid := GetAsInt(ifield)
	if invalid || idx<0 {
		panic(fmt.Errorf("var error: not a valid element index. (ifield:%v)",ifield))
	}

    // pf("ev-af %v[%v]\n",obj,idx)
    switch obj:=obj.(type) {
	case []string:
		return obj[idx]
	case []bool:
		return obj[idx]
	case []int:
		return obj[idx]
	case []uint8:
		return obj[idx]
	case [][]bool:
		return obj[idx]
	case [][]string:
		return obj[idx]
	case []float64:
		return obj[idx]
	case [][]int:
		return obj[idx]
	case [][]uint8:
		return obj[idx]
	case [][]float64:
		return obj[idx]
	case [][]interface{}:
		return obj[idx]
	case []interface{}:
		return obj[idx]
	default:
		strVal := sf("%#v", obj)
		structOfString := `[]interface {}{"`
		if strings.HasPrefix(strVal, structOfString) {
			// strip start+end non-data chars
			strVal = strVal[15 : len(strVal)-1]
			// convert remnant into array
			r := regexp.MustCompile(`[^,"'\s]+|"([^"]*)"`)
			arr := r.FindAllString(strVal, -1)
			// return required index
			return stripOuterQuotes(arr[idx], 1)
		}
	}

	return nil
	// panic(fmt.Errorf("syntax error: cannot access fields in %v on type %s", s, typeOf(obj)))
}

func slice(v interface{}, from, to interface{}) interface{} {
	str, isStr := v.(string)
	arr, isArr := v.([]interface{})

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
		toInt = len(arr)
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

	if toInt < 0 || toInt > len(arr) {
		panic(fmt.Errorf("range error: end-index %d is out of range [0, %d]", toInt, len(arr)))
	}
	if fromInt > toInt {
		panic(fmt.Errorf("range error: start-index %d is greater than end-index %d", fromInt, toInt))
	}
	return arr[fromInt:toInt]
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

// var liblock = &sync.RWMutex{}

func callFunction(evalfs uint64, name string, args []interface{}) (res interface{}) {

	f, ok := stdlib[name]

	if !ok {
		// check if exists in user defined function space
		lmv, isFunc := fnlookup.lmget(name)
		if isFunc {

            var valid bool

			// make Za function call
            // debug(20,"gnfs called from callFunction()\n")
            loc,id := GetNextFnSpace(name+"@")

            if lockSafety { calllock.Lock() }
			calltable[loc] = call_s{fs: id, base: lmv, caller: evalfs, retvar: "@temp"}
            if lockSafety { calllock.Unlock() }

			Call(MODE_NEW, loc, args...)

			// handle the returned result, if present.
            res, valid = vget(evalfs, "@temp")
            if !valid {
                return nil
            }

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


