// +build !test

package main

import (
    "errors"
    "fmt"
    "math"
    "math/rand"
    "strconv"
)

func buildMathLib() {

    features["math"] = Feature{version: 1, category: "math"}
    categories["math"] = []string{
        "seed", "rand", "randf", "pow","abs",
        "sin", "cos", "tan", "asin", "acos", "atan",
        "sinh", "cosh", "tanh", "asinh", "acosh", "atanh",
        "floor", "ln", "logn", "log2", "log10", "round", "rad2deg", "deg2rad",
        "e", "pi", "phi", "ln2", "ln10","ibase",
        "ubin8","uhex32","numcomma",
    }

    slhelp["e"] = LibHelp{in: "", out: "float", action: "Returns the value of e."}
    stdlib["e"] = func(evalfs uint32,ident *[szIdent]Variable,args ...interface{}) (ret interface{}, err error) {
        return 2.71828182845904523536028747135266249775724709369995957496696763, nil
    }
    slhelp["pi"] = LibHelp{in: "", out: "float", action: "Returns the value of pi."}
    stdlib["pi"] = func(evalfs uint32,ident *[szIdent]Variable,args ...interface{}) (ret interface{}, err error) {
        return 3.14159265358979323846264338327950288419716939937510582097494459, nil
    }
    slhelp["phi"] = LibHelp{in: "", out: "float", action: "Returns the value of phi."}
    stdlib["phi"] = func(evalfs uint32,ident *[szIdent]Variable,args ...interface{}) (ret interface{}, err error) {
        return 1.61803398874989484820458683436563811772030917980576286213544862, nil
    }
    slhelp["ln2"] = LibHelp{in: "", out: "float", action: "Returns the value of ln2."}
    stdlib["ln2"] = func(evalfs uint32,ident *[szIdent]Variable,args ...interface{}) (ret interface{}, err error) {
        return 0.693147180559945309417232121458176568075500134360255254120680009, nil
    }
    slhelp["ln10"] = LibHelp{in: "", out: "float", action: "Returns the value of ln10."}
    stdlib["ln10"] = func(evalfs uint32,ident *[szIdent]Variable,args ...interface{}) (ret interface{}, err error) {
        return 2.30258509299404568401799145468436420760110148862877297603332790, nil
    }

    slhelp["numcomma"] = LibHelp{in: "number[,precision]", out: "string", action: "Returns formatted number."}
    stdlib["numcomma"] = func(evalfs uint32,ident *[szIdent]Variable,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("numcomma",args,6,
        "2","float64","int",
        "2","int64","int",
        "2","int","int",
        "1","float64",
        "1","int64",
        "1","int"); !ok { return nil,err }

        var precString string
        switch len(args) {
        case 1:
            precString=".######"
        case 2:
            precString="."
            for e:=args[1].(int);e>0;e-- {
                precString=precString+"#"
            }
        }

        switch args[0].(type) {
        case float64:
            return RenderFloat("#,###"+precString,args[0].(float64)),nil
        case int:
        case int64:
        default:
            return math.NaN,errors.New(sf("type '%T' is not supported by numcomma",args[0]))
        }
        r,invalid:=GetAsFloat(args[0])
        if invalid {
            return math.NaN,errors.New(sf("could not evaluate numcomma(%v)",args[0]))
        }
        return RenderFloat("#,###"+precString,r),nil
    }

    slhelp["ln"] = LibHelp{in: "number", out: "float", action: "Calculate natural logarithm of [#i1]number[#i0]."}
    stdlib["ln"] = func(evalfs uint32,ident *[szIdent]Variable,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("ln",args,1,"1","number"); !ok { return nil,err }

        var n float64
        switch args[0].(type) {
        case float64, int, int64:
            n, _ = GetAsFloat(args[0])
            n = math.Log(n)
        }
        return n, nil
    }

    slhelp["log10"] = LibHelp{in: "number", out: "float", action: "Calculate logarithm (base 10) of [#i1]number[#i0]."}
    stdlib["log10"] = func(evalfs uint32,ident *[szIdent]Variable,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("log10",args,3,"1","number"); !ok { return nil,err }
        var n float64
        switch args[0].(type) {
        case float64, int, int64:
            n, _ = GetAsFloat(args[0])
            n = math.Log10(n)
        }
        return n, nil
    }

    slhelp["log2"] = LibHelp{in: "number", out: "float", action: "Calculate logarithm (base 2) of [#i1]number[#i0]."}
    stdlib["log2"] = func(evalfs uint32,ident *[szIdent]Variable,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("log2",args,3,"1","number"); !ok { return nil,err }
        var n float64
        switch args[0].(type) {
        case float64, int, int64:
            n, _ = GetAsFloat(args[0])
            n = math.Log2(n)
        }
        return n, nil
    }

    slhelp["logn"] = LibHelp{in: "number,base", out: "float", action: "Calculate logarithm (base [#i1]base[#i0]) of [#i1]number[#i0]. FP results may be fuzzy."}
    stdlib["logn"] = func(evalfs uint32,ident *[szIdent]Variable,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("log2",args,3,
        "2","float64","number",
        "2","int64","number",
        "2","int","number"); !ok { return nil,err }

        var n, b float64
        switch args[0].(type) {
        case float64, int, int64:
            n, _ = GetAsFloat(args[0])
        }
        switch args[1].(type) {
        case float64, int, int64:
            b, _ = GetAsFloat(args[1])
            if b <= 0 {
                return 0, errors.New("Base must be positive in log()")
            }
        default:
            return 0, errors.New("Data type not supported.")
        }
        n = math.Log(n) / math.Log(b)
        return n, nil
    }

    slhelp["deg2rad"] = LibHelp{in: "number", out: "float", action: "Convert degrees to radians."}
    stdlib["deg2rad"] = func(evalfs uint32,ident *[szIdent]Variable,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("deg2rad",args,1,"1","number"); !ok { return nil,err }
        var radians float64
        switch args[0].(type) {
        case float64, int, int64:
            deg, _ := GetAsFloat(args[0])
            radians = deg * (math.Pi / 180)
        default:
            return 0, errors.New("Data type not supported.")
        }
        return radians, nil
    }

    slhelp["rad2deg"] = LibHelp{in: "number", out: "float", action: "Convert radians to degrees."}
    stdlib["rad2deg"] = func(evalfs uint32,ident *[szIdent]Variable,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("rad2deg",args,1,"1","number"); !ok { return nil,err }
        var degrees float64
        switch args[0].(type) {
        case float64, int, int64:
            rad, _ := GetAsFloat(args[0])
            degrees = rad * (180 / math.Pi)
        default:
            return 0, errors.New("Data type not supported.")
        }
        return degrees, nil
    }

    slhelp["asin"] = LibHelp{in: "number", out: "float", action: "Calculate arc sine of [#i1]number[#i0]."}
    stdlib["asin"] = func(evalfs uint32,ident *[szIdent]Variable,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("asin",args,2, "1","int", "1","float64"); !ok { return nil,err }
        var r float64
        switch args[0].(type) {
        case int:
            r = float64(args[0].(int))
        case float64:
            r = args[0].(float64)
        }
        return math.Asin(r), err
    }

    slhelp["acos"] = LibHelp{in: "number", out: "float", action: "Calculate arc cosine of [#i1]number[#i0]."}
    stdlib["acos"] = func(evalfs uint32,ident *[szIdent]Variable,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("acos",args,2, "1","int", "1","float64"); !ok { return nil,err }
        var r float64
        switch args[0].(type) {
        case int:
            r = float64(args[0].(int))
        case float64:
            r = args[0].(float64)
        }
        return math.Acos(r), err
    }

    slhelp["atan"] = LibHelp{in: "number", out: "float", action: "Calculate arc tangent of [#i1]number[#i0]."}
    stdlib["atan"] = func(evalfs uint32,ident *[szIdent]Variable,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("atan",args,2, "1","int", "1","float64"); !ok { return nil,err }
        var r float64
        switch args[0].(type) {
        case int:
            r = float64(args[0].(int))
        case float64:
            r = args[0].(float64)
        }
        return math.Atan(r), err
    }

    slhelp["sinh"] = LibHelp{in: "number", out: "float", action: "Calculate hyberbolic sine of [#i1]number[#i0]."}
    stdlib["sinh"] = func(evalfs uint32,ident *[szIdent]Variable,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("sinh",args,2, "1","int", "1","float64"); !ok { return nil,err }
        var r float64
        switch args[0].(type) {
        case int:
            r = float64(args[0].(int))
        case float64:
            r = args[0].(float64)
        }
        return math.Sinh(r), err
    }

    slhelp["asinh"] = LibHelp{in: "number", out: "float", action: "Calculate hyberbolic arc sine of [#i1]number[#i0]."}
    stdlib["asinh"] = func(evalfs uint32,ident *[szIdent]Variable,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("asinh",args,2, "1","int", "1","float64"); !ok { return nil,err }
        var r float64
        switch args[0].(type) {
        case int:
            r = float64(args[0].(int))
        case float64:
            r = args[0].(float64)
        }
        return math.Asinh(r), err
    }

    slhelp["cosh"] = LibHelp{in: "number", out: "float", action: "Calculate hyberbolic cosine of [#i1]number[#i0]."}
    stdlib["cosh"] = func(evalfs uint32,ident *[szIdent]Variable,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("cosh",args,2, "1","int", "1","float64"); !ok { return nil,err }
        var r float64
        switch args[0].(type) {
        case int:
            r = float64(args[0].(int))
        case float64:
            r = args[0].(float64)
        }
        return math.Cosh(r), err
    }

    slhelp["acosh"] = LibHelp{in: "number", out: "float", action: "Calculate hyberbolic arc cosine of [#i1]number[#i0]."}
    stdlib["acosh"] = func(evalfs uint32,ident *[szIdent]Variable,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("acosh",args,2, "1","int", "1","float64"); !ok { return nil,err }
        var r float64
        switch args[0].(type) {
        case int:
            r = float64(args[0].(int))
        case float64:
            r = args[0].(float64)
        }
        return math.Acosh(r), err
    }

    slhelp["tanh"] = LibHelp{in: "number", out: "float", action: "Calculate hyberbolic tangent of [#i1]number[#i0]."}
    stdlib["tanh"] = func(evalfs uint32,ident *[szIdent]Variable,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("tanh",args,2, "1","int", "1","float64"); !ok { return nil,err }
        var r float64
        switch args[0].(type) {
        case int:
            r = float64(args[0].(int))
        case float64:
            r = args[0].(float64)
        }
        return math.Tanh(r), err
    }

    slhelp["atanh"] = LibHelp{in: "number", out: "float", action: "Calculate hyberbolic arc tangent of [#i1]number[#i0]."}
    stdlib["atanh"] = func(evalfs uint32,ident *[szIdent]Variable,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("atanh",args,2, "1","int", "1","float64"); !ok { return nil,err }
        var r float64
        switch args[0].(type) {
        case int:
            r = float64(args[0].(int))
        case float64:
            r = args[0].(float64)
        }
        return math.Atanh(r), err
    }

    slhelp["sin"] = LibHelp{in: "number", out: "float", action: "Calculate sine of [#i1]number[#i0]."}
    stdlib["sin"] = func(evalfs uint32,ident *[szIdent]Variable,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("sin",args,2, "1","int", "1","float64"); !ok { return nil,err }
        var r float64
        switch args[0].(type) {
        case int:
            r = float64(args[0].(int))
        case float64:
            r = args[0].(float64)
        }
        return math.Sin(r), err
    }

    slhelp["cos"] = LibHelp{in: "number", out: "float", action: "Calculate cosine of [#i1]number[#i0]."}
    stdlib["cos"] = func(evalfs uint32,ident *[szIdent]Variable,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("cos",args,2, "1","int", "1","float64"); !ok { return nil,err }
        var r float64
        switch args[0].(type) {
        case int:
            r = float64(args[0].(int))
        case float64:
            r = args[0].(float64)
        }
        return math.Cos(r), err
    }

    slhelp["tan"] = LibHelp{in: "number", out: "float", action: "Calculate tangent of [#i1]number[#i0]."}
    stdlib["tan"] = func(evalfs uint32,ident *[szIdent]Variable,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("tan",args,2, "1","int", "1","float64"); !ok { return nil,err }
        var r float64
        switch args[0].(type) {
        case int:
            r = float64(args[0].(int))
        case float64:
            r = args[0].(float64)
        }
        return math.Tan(r), err
    }

    slhelp["pow"] = LibHelp{in: "number,n", out: "float", action: "Calculate [#i1]number[#i0] raised to the power [#i1]n[#i0]."}
    stdlib["pow"] = func(evalfs uint32,ident *[szIdent]Variable,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("pow",args,4,
            "2","int","int",
            "2","int","float64",
            "2","float64","int",
            "2","float64","float64"); !ok { return nil,err }

        var p1, p2 float64
        switch args[0].(type) {
        case int:
            p1 = float64(args[0].(int))
        case float64:
            p1 = args[0].(float64)
        }
        switch args[1].(type) {
        case int:
            p2 = float64(args[1].(int))
        case float64:
            p2 = args[1].(float64)
        }
        return math.Pow(p1, p2), err
    }

    slhelp["abs"] = LibHelp{in: "int", out: "positive_int", action: "Calculate absolute value of [#i1]int[#i0]."}
    stdlib["abs"] = func(evalfs uint32,ident *[szIdent]Variable,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("abs",args,1,"1","number"); !ok { return nil,err }
        switch args[0].(type) {
        case int:
            n := args[0].(int)
            y := n >> 63
            return (n ^ y) - y, nil
        case int64:
            n := args[0].(int64)
            y := n >> 63
            return (n ^ y) - y, nil
        case float64:
            return math.Abs(args[0].(float64)),nil
        default:
            return -1, errors.New("argument to abs() must be an integer or float.")
        }
    }

    slhelp["round"] = LibHelp{in: "float", out: "float", action: "Round float to nearest integer."}
    stdlib["round"] = func(evalfs uint32,ident *[szIdent]Variable,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("round",args,2,"1","float64","1","int"); !ok { return nil,err }
        switch args[0].(type) {
        case float64:
            switch len(args) {
            case 1:
                return math.Round(args[0].(float64)), nil
            }
        case int:
            switch len(args) {
            case 1:
                return math.Round(float64(args[0].(int))), nil
            }
        }
        return math.NaN(), err
    }

    slhelp["floor"] = LibHelp{in: "float", out: "float", action: "Round float down to nearest integer."}
    stdlib["floor"] = func(evalfs uint32,ident *[szIdent]Variable,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("floor",args,2,"1","float64","1","int"); !ok { return nil,err }
        switch args[0].(type) {
        case float64:
            switch len(args) {
            case 1:
                return math.Floor(args[0].(float64)), nil
            }
        case int:
            switch len(args) {
            case 1:
                return math.Floor(float64(args[0].(int))), nil
            }
        }
        return math.NaN(), err
    }

    slhelp["ubin8"] = LibHelp{in: "unsigned_binary_string", out: "int", action: "unsigned binary to decimal. (8-bit)"}
    stdlib["ubin8"] = func(evalfs uint32,ident *[szIdent]Variable,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("ubin8",args,1,"1","string"); !ok { return nil,err }
        if i,err:=strconv.ParseUint(args[0].(string), 2, 8); err==nil {
            return int(i),nil
        } else {
            return int(0),errors.New(sf("could not convert %s",args[0].(string)))
        }
    }

    slhelp["uhex32"] = LibHelp{in: "unsigned_hex_string", out: "int", action: "unsigned hexadecimal to decimal. (16-bit)"}
    stdlib["uhex32"] = func(evalfs uint32,ident *[szIdent]Variable,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("uhex32",args,1,"1","string"); !ok { return nil,err }
        if i,err:=strconv.ParseUint(args[0].(string), 16, 16); err==nil {
            return int(i),nil
        } else {
            return int(0),errors.New(sf("could not convert %s",args[0].(string)))
        }
    }

    slhelp["ibase"] = LibHelp{in: "n,int", out: "string", action: "Returns a string holding a conversion of [#i1]int[#i0] to base [#i1]n[#i0]"}
    stdlib["ibase"] = func(evalfs uint32,ident *[szIdent]Variable,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("ibase",args,1,"2","number","number"); !ok { return nil,err }
        var i int64
        var n int
        var e bool
        if n,e=GetAsInt(args[0]); e==true {
            return "",errors.New("invalid base specified in ibase()")
        }
        if i,e=GetAsInt64(args[1]); e==true {
            return "",errors.New("invalid number specified in ibase()")
        }
        return strconv.FormatInt(i, n),nil
    }

    slhelp["rand"] = LibHelp{in: "positive_max_int", out: "int", action: "Generate a random integer between 1 and [#i1]positive_max_int[#i0] inclusive."}
    stdlib["rand"] = func(evalfs uint32,ident *[szIdent]Variable,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("rand",args,1,"1","int"); !ok { return nil,err }
        if args[0].(int) <= 0 {
            pf("Error: Argument to rand() must be a positive integer.\n")
            finish(false, ERR_SYNTAX)
            return nil,err
        }
        return 1+rand.Intn(args[0].(int)), err
    }

    slhelp["randf"] = LibHelp{in: "", out: "float", action: "Generate a random float between 0 and 1."}
    stdlib["randf"] = func(evalfs uint32,ident *[szIdent]Variable,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("randf",args,2,
        "1","int",
        "0"); !ok { return nil,err }
        if len(args)==1 {
            return rand.Float64()*float64(args[0].(int)),nil
        }
        return rand.Float64(), nil
    }

    slhelp["seed"] = LibHelp{in: "number", out: "", action: "Set the random seed."}
    stdlib["seed"] = func(evalfs uint32,ident *[szIdent]Variable,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("seed",args,2,"1","int64","1","int"); !ok { return nil,err }
        var r int64
        switch args[0].(type) {
        case int:
            r = int64(args[0].(int))
        case int64:
            r = args[0].(int64)
        }
        rand.Seed(r)
        return nil, err
    }

}

func min_int16(s []int16) (m int16) {
    for i, e := range s {
        if i == 0 || e < m {
            m = e
        }
    }
    return m
}

func min_int(s []int) (m int) {
    for i, e := range s {
        if i == 0 || e < m {
            m = e
        }
    }
    return m
}

func min_int64(s []int64) (m int64) {
    for i, e := range s {
        if i == 0 || e < m {
            m = e
        }
    }
    return m
}

func min_uint(s []uint) (m uint) {
    for i, e := range s {
        if i == 0 || e < m {
            m = e
        }
    }
    return m
}

func min_float64(s []float64) (m float64) {
    for i, e := range s {
        if i == 0 || e < m {
            m = e
        }
    }
    return m
}

func min_inter(s []interface{}) (m float64) {
    for i, e := range s {
        ee, err := GetAsFloat(sf("%v", e))
        if !err && (i == 0 || ee < m) {
            m = ee
        }
    }
    return m
}

func max_int16(s []int16) (m int16) {
    for i, e := range s {
        if i == 0 || e > m {
            m = e
        }
    }
    return m
}

func max_int(s []int) (m int) {
    for i, e := range s {
        if i == 0 || e > m {
            m = e
        }
    }
    return m
}

func max_int64(s []int64) (m int64) {
    for i, e := range s {
        if i == 0 || e > m {
            m = e
        }
    }
    return m
}

func max_uint(s []uint) (m uint) {
    for i, e := range s {
        if i == 0 || e > m {
            m = e
        }
    }
    return m
}

func max_float64(s []float64) (m float64) {
    for i, e := range s {
        if i == 0 || e > m {
            m = e
        }
    }
    return m
}

func max_inter(s []interface{}) (m float64) {
    for i, e := range s {
        ee, err := GetAsFloat(sf("%v", e))
        if !err && (i == 0 || ee > m) {
            m = ee
        }
    }
    return m
}

func avg_int(s []int) (m int) {
    c := float64(0)
    sum := float64(0)
    for _, e := range s {
        sum += float64(e)
        c++
    }
    if c != 0 {
        return int(sum / c)
    }
    panic(fmt.Errorf("divide by zero generating an average"))
}

func avg_int64(s []int64) (m int64) {
    c := float64(0)
    sum := float64(0)
    for _, e := range s {
        sum += float64(e)
        c++
    }
    if c != 0 {
        return int64(sum / c)
    }
    panic(fmt.Errorf("divide by zero generating an average"))
}

func avg_uint(s []uint) (m uint) {
    c := float64(0)
    sum := float64(0)
    for _, e := range s {
        sum += float64(e)
        c++
    }
    if c != 0 {
        return uint(sum / c)
    }
    panic(fmt.Errorf("divide by zero generating an average"))
}

func avg_float64(s []float64) (m float64) {
    c := float64(0)
    sum := float64(0)
    for _, e := range s {
        sum += float64(e)
        c++
    }
    if c != 0 {
        return sum / c
    }
    panic(fmt.Errorf("divide by zero generating an average"))
}

func avg_inter(s []interface{}) (m float64) {
    c := float64(0)
    sum := float64(0)
    for _, e := range s {
        ee, _ := GetAsFloat(sf("%v", e))
        sum += ee
        c++
    }
    if c != 0 {
        return sum / c
    }
    panic(fmt.Errorf("divide by zero generating an average"))
}

func sum_uint(s []uint) (m uint) {
    sum := float64(0)
    for _, e := range s {
        sum += float64(e)
    }
    return uint(sum)
}

func sum_int(s []int) (m int) {
    sum := float64(0)
    for _, e := range s {
        sum += float64(e)
    }
    return int(sum)
}

func sum_int64(s []int64) (m int64) {
    sum := float64(0)
    for _, e := range s {
        sum += float64(e)
    }
    return int64(sum)
}

func sum_float64(s []float64) (m float64) {
    sum := float64(0)
    for _, e := range s {
        sum += float64(e)
    }
    return sum
}

func sum_inter(s []interface{}) (m float64) {
    sum := float64(0)
    for _, e := range s {
        ee, _ := GetAsFloat(sf("%v", e))
        sum += ee
    }
    return sum
}

func floor(x float64) float64 {
    return math.Floor(x)
}

func round(x float64) float64 {
    return math.Round(x)
}

/*

Author: https://github.com/gorhill
Source: https://gist.github.com/gorhill/5285193

A Go function to render a number to a string based on
the following user-specified criteria:

* thousands separator
* decimal separator
* decimal precision

Usage: s := RenderFloat(format, n)

The format parameter tells how to render the number n.

http://play.golang.org/p/LXc1Ddm1lJ

Examples of format strings, given n = 12345.6789:

"#,###.##" => "12,345.67"
"#,###." => "12,345"
"#,###" => "12345,678"
"#\u202F###,##" => "12 345,67"
"#.###,###### => 12.345,678900
"" (aka default format) => 12,345.67

The highest precision allowed is 9 digits after the decimal symbol.
There is also a version for integer number, RenderInteger(),
which is convenient for calls within template.

I didn't feel it was worth to publish a library just for this piece
of code, hence the snippet. Feel free to reuse as you wish.

Source Modified: 

*/

var renderFloatPrecisionMultipliers = [10]float64{
    1,
    10,
    100,
    1000,
    10000,
    100000,
    1000000,
    10000000,
    100000000,
    1000000000,
}

var renderFloatPrecisionRounders = [10]float64{
    0.5,
    0.05,
    0.005,
    0.0005,
    0.00005,
    0.000005,
    0.0000005,
    0.00000005,
    0.000000005,
    0.0000000005,
}

func RenderFloat(format string, n float64) string {

    if math.IsNaN(n) {
        return "NaN"
    }
    if n > math.MaxFloat64 {
        return "Infinity"
    }
    if n < -math.MaxFloat64 {
        return "-Infinity"
    }

    // default format
    precision := 2
    decimalStr := "."
    thousandStr := ","
    positiveStr := ""
    negativeStr := "-"

    if len(format) > 0 {
        // If there is an explicit format directive,
        // then default values are these:
        precision = 9
        thousandStr = ""

        // collect indices of meaningful formatting directives
        formatDirectiveChars := []rune(format)
        formatDirectiveIndices := make([]int, 0)
        for i, char := range formatDirectiveChars {
            if char != '#' && char != '0' {
                formatDirectiveIndices = append(formatDirectiveIndices, i)
            }
        }

        if len(formatDirectiveIndices) > 0 {
            // Directive at index 0:
            //   Must be a '+'
            //   Raise an error if not the case
            // index: 0123456789
            //        +0.000,000
            //        +000,000.0
            //        +0000.00
            //        +0000
            if formatDirectiveIndices[0] == 0 {
                if formatDirectiveChars[formatDirectiveIndices[0]] != '+' {
                    panic("RenderFloat(): invalid positive sign directive")
                }
                positiveStr = "+"
                formatDirectiveIndices = formatDirectiveIndices[1:]
            }

            // Two directives:
            //   First is thousands separator
            //   Raise an error if not followed by 3-digit
            // 0123456789
            // 0.000,000
            // 000,000.00
            if len(formatDirectiveIndices) == 2 {
                if (formatDirectiveIndices[1] - formatDirectiveIndices[0]) != 4 {
                    panic("RenderFloat(): thousands separator directive must be followed by 3 digit-specifiers")
                }
                thousandStr = string(formatDirectiveChars[formatDirectiveIndices[0]])
                formatDirectiveIndices = formatDirectiveIndices[1:]
            }

            // One directive:
            //   Directive is decimal separator
            //   The number of digit-specifier following the separator indicates wanted precision
            // 0123456789
            // 0.00
            // 000,0000
            if len(formatDirectiveIndices) == 1 {
                decimalStr = string(formatDirectiveChars[formatDirectiveIndices[0]])
                precision = len(formatDirectiveChars) - formatDirectiveIndices[0] - 1
            }
        }
    }

    // generate sign part
    var signStr string
    if n >= 0.000000001 {
        signStr = positiveStr
    } else if n <= -0.000000001 {
        signStr = negativeStr
        n = -n
    } else {
        signStr = ""
        n = 0.0
    }

    // split number into integer and fractional parts
    intf, fracf := math.Modf(n + renderFloatPrecisionRounders[precision])

    // generate integer part string
    intStr := strconv.Itoa(int(intf))
    // some systems may need this instead, x32 compiles on x64, arm?
    // intStr := strconv.FormatInt(int64(intf),10)

    // add thousand separator if required
    if len(thousandStr) > 0 {
        for i := len(intStr); i > 3; {
            i -= 3
            intStr = intStr[:i] + thousandStr + intStr[i:]
        }
    }

    // no fractional part, we can leave now
    if precision == 0 {
        return signStr + intStr
    }

    // generate fractional part
    fracStr := strconv.Itoa(int(fracf * renderFloatPrecisionMultipliers[precision]))
    // may need padding
    if len(fracStr) < precision {
        fracStr = "000000000000000"[:precision-len(fracStr)] + fracStr
    }

    return signStr + intStr + decimalStr + fracStr
}

func RenderInteger(format string, n int) string {
    return RenderFloat(format, float64(n))
}


