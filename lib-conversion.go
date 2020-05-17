//+build !test

package main

import (
	"errors"
    "bytes"
    "math"
    "encoding/base64"
    "encoding/json"
    "strings"
)

func buildConversionLib() {

	// conversion

	features["conversion"] = Feature{version: 1, category: "os"}
	categories["conversion"] = []string{
        "byte","int", "float", "string", "kind", "chr", "ascii",
        "is_number","base64e","base64d","json_decode","json_format",
    }

	slhelp["chr"] = LibHelp{in: "number", out: "ascii_char", action: "Return a string representation of ASCII char [#i1]number[#i0]."}
	stdlib["chr"] = func(args ...interface{}) (ret interface{}, err error) {

		if len(args) != 1 {
			return "", errors.New("invalid arguments provided to chr()")
		}

		switch args[0].(type) {
		case int:
			if args[0].(int) < 0 || args[0].(int) > 255 {
				return "", nil
			}
			if c, e := GetAsInt(args[0]); e == false {
				return sf("%c", c), nil
			} else {
				return "", errors.New("unspecified error in type conversion")
			}

		default:
			return "", errors.New(sf("unsupported type %T", args[0]))
		}
	}

	// @todo: fix this up when we support runes better.
	slhelp["ascii"] = LibHelp{in: "ascii_char", out: "integer", action: "Return a numeric representation of [#i1]ascii_char[#i0]."}
	stdlib["ascii"] = func(args ...interface{}) (ret interface{}, err error) {
		if len(args) != 1 {
			return -1, errors.New("invalid arguments provided to ascii()")
		}
		switch args[0].(type) {
		case string:
			if len(args[0].(string)) != 1 {
				return -1, errors.New("string must be 1 character long")
			}
			return int([]rune(args[0].(string))[0]), nil
		}
		return -1, err
	}

	slhelp["kind"] = LibHelp{in: "variable", out: "type_string", action: "Return a string indicating the type of the variable."}
	stdlib["kind"] = func(args ...interface{}) (ret interface{}, err error) {
		if len(args) != 1 {
			return -1, errors.New("invalid arguments provided to kind()")
		}
		return sf("%T", args[0]), nil
	}

	slhelp["base64e"] = LibHelp{in: "string", out: "base64_string", action: "Return a string of the base64 encoding of [#i1]string[#i0]"}
	stdlib["base64e"] = func(args ...interface{}) (ret interface{}, err error) {
		if len(args) != 1 { return -1, errors.New("invalid arguments (count) provided to base64e()") }
        if sf("%T",args[0])!="string" { return "",errors.New("invalid arguments (type) provided to base64e()") }
        enc:=base64.StdEncoding.EncodeToString([]byte(args[0].(string)))
		return enc,nil
	}

	slhelp["base64d"] = LibHelp{in: "base64_string", out: "string", action: "Return a string of the base64 decoding of [#i1]base64_string[#i0]"}
	stdlib["base64d"] = func(args ...interface{}) (ret interface{}, err error) {
		if len(args) != 1 { return -1, errors.New("invalid arguments (count) provided to base64d()") }
        if sf("%T",args[0])!="string" { return "",errors.New("invalid arguments (type) provided to base64d()") }
        dec,e:=base64.StdEncoding.DecodeString(args[0].(string))
        if e!=nil { return "",errors.New(sf("could not convert '%s' in base64d()",args[0].(string))) }
		return string(dec),nil
	}

	slhelp["json_decode"] = LibHelp{in: "json_string", out: "mixed", action: "Return a mixed type structure."}
	stdlib["json_decode"] = func(args ...interface{}) (ret interface{}, err error) {
		if len(args) != 1 { return -1, errors.New("invalid arguments (count) provided to json_decode()") }
        if sf("%T",args[0])!="string" { return "",errors.New("invalid arguments (type) provided to json_decode()") }

        var v map[string]interface{}
        dec:=json.NewDecoder(strings.NewReader(args[0].(string)))

        if err := dec.Decode(&v); err!=nil {
            return "",errors.New(sf("could not convert value '%v' in json_decode()",args[0].(string)))
        }

		return v,nil

	}

	slhelp["json_format"] = LibHelp{in: "json_string", out: "string", action: "Return a formatted string."}
	stdlib["json_format"] = func(args ...interface{}) (ret interface{}, err error) {
		if len(args) != 1 { return -1, errors.New("invalid arguments (count) provided to json_format()") }
        if sf("%T",args[0])!="string" { return "",errors.New("invalid arguments (type) provided to json_format()") }
        var pj bytes.Buffer
        if err := json.Indent(&pj,[]byte(args[0].(string)), "", "\t"); err!=nil {
            return "",errors.New(sf("could not format string in json_format()"))
        }
		return string(pj.Bytes()),nil
    }

	slhelp["float"] = LibHelp{in: "variable", out: "float", action: "Convert to a float."}
	stdlib["float"] = func(args ...interface{}) (ret interface{}, err error) {
		if len(args) != 1 {
			return -1, errors.New("invalid arguments provided to float()")
		}
		i, e := GetAsFloat(args[0])
        if e { return math.NaN(),nil } // errors.New(sf("could not convert '%v' in float()",args[0])) }
		return i, nil
	}

	slhelp["byte"] = LibHelp{in: "string_number", out: "byte", action: "Convert to a uint8."}
	stdlib["byte"] = func(args ...interface{}) (ret interface{}, err error) {
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

	slhelp["int"] = LibHelp{in: "string_number", out: "integer", action: "Convert to an integer."}
	stdlib["int"] = func(args ...interface{}) (ret interface{}, err error) {
		if len(args) != 1 {
			return -1, errors.New("invalid arguments provided to int()")
		}
		i, invalid := GetAsInt(args[0])
		if !invalid {
			return i, nil
		}
		return 0, err
	}

	slhelp["string"] = LibHelp{in: "some_type", out: "string", action: "Convert to a string."}
	stdlib["string"] = func(args ...interface{}) (ret interface{}, err error) {
		if len(args) != 1 {
			return -1, errors.New("invalid arguments provided to string()")
		}
		i := sf("%v", args[0])
		return i, nil
	}

	slhelp["is_number"] = LibHelp{in: "expression", out: "bool", action: "is [#i1]expression[#i0] a number?"}
	stdlib["is_number"] = func(args ...interface{}) (ret interface{}, err error) {
		if len(args) != 1 {
			return -1, errors.New("invalid arguments provided to is_number()")
		}
		switch args[0].(type) {
		case uint, uint8, uint32, uint64, int, int32, int64, float32, float64:
			return isNumber(args[0]), nil
		default:
			return false, nil
		}
	}

}
