//+build !test

package main

import (
    "errors"
    "bufio"
    "bytes"
    "math"
    "reflect"
    "os"
    "strconv"
    "encoding/base64"
    "encoding/json"
    "strings"
    "unsafe"
    "encoding/gob"
    "github.com/itchyny/gojq"
)

// struct to map
func s2m(val interface{}) map[string]interface{} {

    m:=make(map[string]interface{})

    rs  := reflect.ValueOf(val)
    rt  := rs.Type()
    rs2 := reflect.New(rs.Type()).Elem()
    rs2.Set(rs)

    for i := 0; i < rs.NumField(); i++ {
        rf := rs2.Field(i)
        rf  = reflect.NewAt(rf.Type(), unsafe.Pointer(rf.UnsafeAddr())).Elem()
        name:=rt.Field(i).Name
        m[name] = rf.Interface()
    }

    return m
}


// map to struct: requires type information of receiver.

func m2s(m map[string]interface{}, rcvr interface{}) interface{} {

    // get underlying type of rcvr
    rs  := reflect.ValueOf(rcvr)
    rt  := rs.Type()

    rs2 := reflect.New(rs.Type()).Elem()
    rs2.Set(rs)

    // populate rcvr through reflection
    for i := 0; i < rs.NumField(); i++ {
        rf := rs2.Field(i)
        rf  = reflect.NewAt(rf.Type(), unsafe.Pointer(rf.UnsafeAddr())).Elem()
        name:=rt.Field(i).Name
        switch tm:=m[name].(type) {
        case bool,int,int64,uint,uint8,uint64,float64,string:
            rf.Set(reflect.ValueOf(tm))
        default:
            pf("unknown type in m2s '%T'\n",tm)
        }
    }

    return rs2.Interface()
}

func buildConversionLib() {

    // conversion

    features["conversion"] = Feature{version: 1, category: "os"}
    categories["conversion"] = []string{
        "byte","int", "int64", "float", "bool", "string", "kind", "char", "asc","uint",
        "is_number","base64e","base64d","json_decode","json_format","json_query",
        "write_struct","read_struct",
        "btoi","itob",
    }

    slhelp["write_struct"] = LibHelp{in: "filename,name_of_struct", out: "size", action: "Sends a struct to file. Returns byte size written."}
    stdlib["write_struct"] = func(evalfs uint32,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("write_struct",args,1,"2","string","string"); !ok { return nil,err }

        fn:=args[0].(string)
        vn:=args[1].(string)

        // convert struct to map
        v,_:=vget(evalfs,vn)
        m:=s2m(v)

        // encode with gob
        b:=new(bytes.Buffer)
        e:=gob.NewEncoder(b)
        err=e.Encode(m)
        if err!=nil {
            return false,err
        }

        // start writer
        f, err := os.Create(fn)
        w:=bufio.NewWriter(f)
        w.Write(b.Bytes())
        w.Flush()
        f.Close()

        return true, nil

    }

    slhelp["read_struct"] = LibHelp{in: "filename,name_of_destination_struct", out: "success_flag", action: "Read a struct from a file."}
    stdlib["read_struct"] = func(evalfs uint32,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("read_struct",args,1,"2","string","string"); !ok { return nil,err }

        fn:=args[0].(string)
        vn:=args[1].(string)

        v,success:=vget(evalfs,vn)
        if !success {
            return false,errors.New(sf("could not find '%v'",vn))
        }

        r  :=reflect.ValueOf(v)

        // confirm this is a struct
        if reflect.ValueOf(r).Kind().String()!="struct" {
            return false,errors.New(sf("'%v' is not a STRUCT",vn))
        }

        // retrieve the packed file
        f,err:=os.Open(fn)
        if err!=nil {
            return nil,err
        }

        // unpack
        var m = new(map[string]interface{})
        d:=gob.NewDecoder(f)
        err=d.Decode(&m)
        f.Close()

        if err != nil {
            return false,errors.New("unpacking error")
        }

        // write to Za variable.
        vset(evalfs,vn,m2s(*m,v))

        return true,nil

    }


    slhelp["char"] = LibHelp{in: "int", out: "string", action: "Return a string representation of ASCII char [#i1]int[#i0]. Representations between 128 and 160 are empty."}
    stdlib["char"] = func(evalfs uint32,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("char",args,1,"1","int"); !ok { return nil,err }

        if args[0].(int) < 0 || args[0].(int) > 255 {
            return "", nil
        }
        c:=args[0].(int)
        if c<128 || c>160 {
            return sf("%c",c),nil
        } else {
            return "",nil
        }
    }

    // @todo: fix this up when we support runes better.
    slhelp["asc"] = LibHelp{in: "string", out: "int", action: "Return a numeric representation of the first char in [#i1]string[#i0]."}
    stdlib["asc"] = func(evalfs uint32,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("asc",args,1,"1","string"); !ok { return nil,err }
        return int([]rune(args[0].(string))[0]), nil
    }

    slhelp["itob"] = LibHelp{in: "int", out: "bool", action: "Return a boolean which is set to true when [#i1]int[#i0] is non-zero."}
    stdlib["itob"] = func(evalfs uint32,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("itob",args,1,"1","int"); !ok { return nil,err }
        return args[0].(int)!=0, nil
    }

    slhelp["btoi"] = LibHelp{in: "bool", out: "int", action: "Return an int which is either 1 when [#i1]bool[#i0] is true or else 0 when [#i1]bool[#i0] is false. This function is mainly useful for performing branchless calculations, although the efficacy is low in an interpreter such as this."}
    stdlib["btoi"] = func(evalfs uint32,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("btoi",args,1,"1","bool"); !ok { return nil,err }
        switch args[0].(bool) {
        case true:
            return 1,nil
        }
        return 0,nil
    }

    slhelp["kind"] = LibHelp{in: "var", out: "string", action: "Return a string indicating the type of the variable [#i1]var[#i0]."}
    stdlib["kind"] = func(evalfs uint32,args ...interface{}) (ret interface{}, err error) {
        if len(args) != 1 {
            return -1, errors.New("invalid arguments provided to kind()")
        }
        return sf("%T", args[0]), nil
    }

    slhelp["base64e"] = LibHelp{in: "string", out: "string", action: "Return a string of the base64 encoding of [#i1]string[#i0]"}
    stdlib["base64e"] = func(evalfs uint32,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("base64e",args,1,"1","string"); !ok { return nil,err }
        enc:=base64.StdEncoding.EncodeToString([]byte(args[0].(string)))
        return enc,nil
    }

    slhelp["base64d"] = LibHelp{in: "string", out: "string", action: "Return a string of the base64 decoding of [#i1]string[#i0]"}
    stdlib["base64d"] = func(evalfs uint32,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("base64d",args,1,"1","string"); !ok { return nil,err }
        dec,e:=base64.StdEncoding.DecodeString(args[0].(string))
        if e!=nil { return "",errors.New(sf("could not convert '%s' in base64d()",args[0].(string))) }
        return string(dec),nil
    }

    slhelp["json_decode"] = LibHelp{in: "string", out: "[]mixed", action: "Return a mixed type array representing a JSON string."}
    stdlib["json_decode"] = func(evalfs uint32,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("jason_decode",args,1,"1","string"); !ok { return nil,err }

        var v map[string]interface{}
        dec:=json.NewDecoder(strings.NewReader(args[0].(string)))

        if err := dec.Decode(&v); err!=nil {
            return "",errors.New(sf("could not convert value '%v' in json_decode()",args[0].(string)))
        }

        return v,nil

    }

    slhelp["json_format"] = LibHelp{in: "string", out: "string", action: "Return a formatted JSON representation of [#i1]string[#i0], or an empty string on error."}
    stdlib["json_format"] = func(evalfs uint32,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("jason_format",args,1,"1","string"); !ok { return nil,err }
        var pj bytes.Buffer
        if err := json.Indent(&pj,[]byte(args[0].(string)), "", "\t"); err!=nil {
            return "",errors.New(sf("could not format string in json_format()"))
        }
        return string(pj.Bytes()),nil
    }

    slhelp["json_query"] = LibHelp{in: "string,query_string[,map_flag]", out: "output_string", action: "Returns the result of processing [#i1]string[#i0] using the gojq library. [#i1]query_string[#i0] is a jq-like query to operate against [#i1]string[#i0]. If [#i1]map_flag[#i0] is false (default) then a string is returned, otherwise an iterable list is returned."}
    stdlib["json_query"] = func(evalfs uint32,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("jason_query",args,2,
            "2","string","string",
            "3","string","string","bool"); !ok { return nil,err }

        var complex bool
        if len(args)==3 {
            switch args[2].(type) {
                case bool:
                    complex=args[2].(bool)
                default:
                    return nil,errors.New("argument 3 must be a boolean when present in json_query()")
            }
        }

        // first parse query string
        q,e:=gojq.Parse(args[1].(string))
        if e!=nil {
            return "",errors.New("invalid query string in json_query()")
        }

        // then decode json to map suitable for gojq.Run
        var iv map[string]interface{}
        dec:=json.NewDecoder(strings.NewReader(args[0].(string)))
        if err := dec.Decode(&iv); err!=nil {
            return "",errors.New("could not convert JSON in json_query()")
        }

        // process query
        var ns strings.Builder
        var retlist []interface{}

        iter:=q.Run(iv)

        for {
            v,ok:=iter.Next()
            if !ok { break }
            if complex {
                retlist=append(retlist,v)
            } else {
                ns.WriteString(sf("%v\n",v))
            }
        }

        if complex { return retlist, nil }
        return ns.String(),nil

    }

    slhelp["float"] = LibHelp{in: "var", out: "float", action: "Convert [#i1]var[#i0] to a float. Returns NaN on error."}
    stdlib["float"] = func(evalfs uint32,args ...interface{}) (ret interface{}, err error) {
        if len(args) != 1 {
            return -1, errors.New("invalid arguments provided to float()")
        }
        i, e := GetAsFloat(args[0])
        if e { return math.NaN(),nil }
        return i, nil
    }

    slhelp["byte"] = LibHelp{in: "var", out: "byte", action: "Convert to a uint8 sized integer, or errors. The type is still [#bold]int[#boff] however the bounds are limited between 0-255."}
    stdlib["byte"] = func(evalfs uint32,args ...interface{}) (ret interface{}, err error) {
        if len(args) != 1 {
            return -1, errors.New("invalid arguments provided to byte()")
        }
        i, invalid := GetAsInt(args[0])
        if !invalid {
            if i>=0 && i<256 {
                return i, nil
            } else {
                return 0,errors.New("out of range value in byte()")
            }
        }
        return 0, err
    }


    slhelp["bool"] = LibHelp{in: "string", out: "bool", action: "Convert [#i1]string[#i0] to a boolean value, or errors"}
    stdlib["bool"] = func(evalfs uint32,args ...interface{}) (ret interface{}, err error) {
        if len(args) != 1 {
            return -1, errors.New("invalid arguments provided to bool()")
        }
        switch args[0].(type) {
        case bool:
            return args[0].(bool),nil
        case string:
            if args[0]=="" { args[0]="false" }
            b, err := strconv.ParseBool(args[0].(string))
            if err==nil {
                return b, nil
            }
        }
        return false, errors.New(sf("could not convert [%T] (%v) to bool in bool()",args[0],args[0]))
    }


    slhelp["int"] = LibHelp{in: "var", out: "integer", action: "Convert [#i1]var[#i0] to an integer, or errors."}
    stdlib["int"] = func(evalfs uint32,args ...interface{}) (ret interface{}, err error) {
        if len(args) != 1 {
            return -1, errors.New("invalid arguments provided to int()")
        }
        i, invalid := GetAsInt(args[0])
        if !invalid {
            return i, nil
        }
        return 0, errors.New(sf("could not convert [%T] (%v) to integer in int()",args[0],args[0]))
    }

    slhelp["uint"] = LibHelp{in: "var", out: "unsigned_integer", action: "Convert [#i1]var[#i0] to a uint type, or errors."}
    stdlib["uint"] = func(evalfs uint32,args ...interface{}) (ret interface{}, err error) {
        if len(args) != 1 {
            return -1, errors.New("invalid arguments provided to uint()")
        }
        i, invalid := GetAsUint(args[0])
        if !invalid {
            return i, nil
        }
        return uint(0), errors.New(sf("could not convert [%T] (%v) to integer in uint()",args[0],args[0]))
    }

    slhelp["int64"] = LibHelp{in: "var", out: "integer", action: "Convert [#i1]var[#i0] to an int64 type, or errors."}
    stdlib["int64"] = func(evalfs uint32,args ...interface{}) (ret interface{}, err error) {
        if len(args) != 1 {
            return -1, errors.New("invalid arguments provided to int64()")
        }
        i, invalid := GetAsInt(args[0])
        if !invalid {
            return int64(i), nil
        }
        return int64(0), errors.New(sf("could not convert [%T] (%v) to integer in int64()",args[0],args[0]))
    }

    slhelp["string"] = LibHelp{in: "var", out: "string", action: "Converts [#i1]var[#i0] to a string."}
    stdlib["string"] = func(evalfs uint32,args ...interface{}) (ret interface{}, err error) {
        if len(args) != 1 {
            return -1, errors.New("invalid arguments provided to string()")
        }
        i := sf("%v", args[0])
        return i, nil
    }

    slhelp["is_number"] = LibHelp{in: "expression", out: "bool", action: "Returns true if [#i1]expression[#i0] evaluates to a numeric value."}
    stdlib["is_number"] = func(evalfs uint32,args ...interface{}) (ret interface{}, err error) {
        if len(args) != 1 {
            return -1, errors.New("invalid arguments provided to is_number()")
        }
        switch args[0].(type) {
        case uint, uint8, uint32, uint64, int, int32, int64, float32, float64:
            return isNumber(args[0]), nil
        case string:
            if len(args[0].(string))==0 {
                return false,nil
            }
            _, invalid := GetAsFloat(args[0])
            if invalid {
                return false, nil
            } else {
                _, invalid := GetAsInt(args[0])
                if invalid {
                    return false,nil
                }
            }
            return true,nil
        default:
            return false, nil
        }
    }

}


