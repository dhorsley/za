//+build !test

package main

import (
    "errors"
    "reflect"
    "regexp"
    "unicode/utf8"
    "runtime"
    "strconv"
    str "strings"
)

const ( // tr_actions
    COPY int = iota
    DELETE
    SQUEEZE
)

func tr(s string, action int, cases string) string {

    original := []byte(s)
    var lastChar byte
    newStr := ""
    squeezing := false

    for _, v := range original {

        if squeezing {
            if v == lastChar {
                continue
            } else {
                squeezing = false
            }
        }

        switch action {
        case DELETE:
            // copy to new string if not found in delete list
            if str.IndexByte(cases, v) == -1 {
                newStr = newStr + string(v)
            }
        case SQUEEZE:
            if str.IndexByte(cases, v) != -1 {
                squeezing = true
                lastChar = v
            }
            newStr = newStr + string(v) // only copy char on first match
        }

    }
    return newStr

}


func runesToUTF8(runes []rune) []byte {
    sz := 0
    for _, r := range runes {
        sz += utf8.RuneLen(r)
    }

    buf := make([]byte, sz)

    count := 0
    for _, r := range runes {
        count += utf8.EncodeRune(buf[count:], r)
    }

    return buf
}


func buildStringLib() {

    // string handling

    features["string"] = Feature{version: 1, category: "text"}
    categories["string"] = []string{"pad", "field", "fields", "get_value", "start", "end", "match", "filter",
        "substr", "gsub", "replace", "trim", "lines", "count",
        "next_match", "line_add", "line_delete", "line_replace", "line_add_before", "line_add_after","line_match","line_filter","line_head","line_tail",
        "reverse", "tr", "lower", "upper", "format", "ccformat","at",
        "split", "join", "collapse","strpos","stripansi","addansi","stripquotes",
    }

    // part of regex caching test - may be removed later.
    compileCache:=make(map[string]regexp.Regexp)

    slhelp["replace"] = LibHelp{in: "var,regex,replacement", out: "string", action: "Replaces matches found in [#i1]var[#i0] with [#i1]regex[#i0] to [#i1]replacement[#i0]."}
    stdlib["replace"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
        if len(args) != 3 {
            return "", errors.New("Error: invalid argument count.\n")
        }
        // @todo: type checks on args.

        src := args[0].(string)
        regex := args[1].(string)
        repl := args[2].(string)
        // pf("debug : s %v , reg %v , repl %v\n",src,regex,repl)

        // pf("compiling %#v\n",regex)

        /*
        // caching added as a test. 
        // may be removed pre-release
        //
        // if it stays in it will also need a max cache size and expiry mechanism.
        //
        */

        var re regexp.Regexp
        if pre,found:=compileCache[regex];!found {
            re = *regexp.MustCompile(regex)
            compileCache[regex]=re
        } else
        {
            re = pre
        }

        // pf("compiled  %#v\n",re)

        s := re.ReplaceAllString(src, repl)
        return s, nil
    }

    slhelp["get_value"] = LibHelp{in: "string_array,key_name", out: "string_value", action: "Returns the value of the key [#i1]key_name[#i0] in [#i1]string_array[#i0]."}
    stdlib["get_value"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {

        if len(args) != 2 {
            return "", errors.New("Error: invalid argument count.\n")
        }

        var search []string

        switch args[0].(type) {
        case string:
            if runtime.GOOS!="windows" {
                search = str.Split(args[0].(string), "\n")
            } else {
                search = str.Split(str.Replace(args[0].(string), "\r\n", "\n", -1), "\n")
            }
        case []string:
            search = args[0].([]string)
        default:
            return "", errors.New("Error: unsupported data type in get_value() source.")
        }

        key := args[1].(string)

        if key=="" {
            return "", nil
        }

        fsep := func(c rune) bool { return c == '=' }
        for _, l := range search {
            ta := str.FieldsFunc(l, fsep)
            if len(ta) == 2 {
                if str.TrimSpace(ta[0]) == key {
                    return str.TrimSpace(ta[1]), nil
                }
            }
        }
        return "", nil // errors.New("Error: key '"+key+"' not found by get_value().")
    }

    // reverse()
    slhelp["reverse"] = LibHelp{in: "list_or_string", out: "as_input", action: "Reverse the contents of a variable."}
    stdlib["reverse"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
        if len(args)!=1 { return "",errors.New("Bad arguments (count) to reverse()") }
        switch args[0].(type) {
        case string:
            ln := len(args[0].(string)) - 1
            r := ""
            for i := ln; i >= 0; i-- {
                r = r + string(args[0].(string)[i])
            }
            return r, nil
        case []int:
            ln := len(args[0].([]int)) - 1
            r := make([]int, 0, ln+1)
            for i := ln; i >= 0; i-- {
                r = append(r, args[0].([]int)[i])
            }
            return r, nil
        case []int32:
            ln := len(args[0].([]int32)) - 1
            r := make([]int32, 0, ln+1)
            for i := ln; i >= 0; i-- {
                r = append(r, args[0].([]int32)[i])
            }
            return r, nil
        case []int64:
            ln := len(args[0].([]int64)) - 1
            r := make([]int64, 0, ln+1)
            for i := ln; i >= 0; i-- {
                r = append(r, args[0].([]int64)[i])
            }
            return r, nil
        case []float32:
            ln := len(args[0].([]float32)) - 1
            r := make([]float32, 0, ln+1)
            for i := ln; i >= 0; i-- {
                r = append(r, args[0].([]float32)[i])
            }
            return r, nil
        case []float64:
            ln := len(args[0].([]float64)) - 1
            r := make([]float64, 0, ln+1)
            for i := ln; i >= 0; i-- {
                r = append(r, args[0].([]float64)[i])
            }
            return r, nil
        case []string:
            ln := len(args[0].([]string)) - 1
            r := make([]string, 0, ln+1)
            for i := ln; i >= 0; i-- {
                r[ln-i] = args[0].([]string)[i]
            }
            return r, nil
        case []interface{}:
            ln := len(args[0].([]interface{})) - 1
            r := make([]interface{}, 0, ln+1)
            for i := ln; i >= 0; i-- {
                r = append(r, args[0].([]interface{})[i])
            }
            return r, nil
        }
        return nil, errors.New("could not reverse()")
    }

    slhelp["ccformat"] = LibHelp{in: "string,var_args", out: "string", action: "Format the input string in the manner of fprintf(). Also processes embedded colour codes to ANSI."}
    stdlib["ccformat"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
        if len(args)==0 { return "",errors.New("Bad arguments (count) in ccformat()") }
        if sf("%T",args[0])!="string" { return "",errors.New("Bad arguments (type) (arg #1 not string) in ccformat()") }
        if len(args) == 1 {
            return sparkle(sf(args[0].(string))), nil
        }
        return sparkle(sf(args[0].(string), args[1:]...)), nil
    }

    slhelp["format"] = LibHelp{in: "string,var_args", out: "string", action: "Format the input string in the manner of fprintf()."}
    stdlib["format"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
        if len(args)==0 { return "",errors.New("Bad arguments (count) in format()") }
        if sf("%T",args[0])!="string" { return "",errors.New("Bad arguments (type) (arg#1 not string) in format()") }
        if len(args) == 1 {
            return sf(args[0].(string)), nil
        }
        return sf(args[0].(string), args[1:]...), nil
    }

    slhelp["at"] = LibHelp{in: "int_row,int_col", out: "string", action: "Returns a cursor positioning ANSI code string for (row,col)."}
    stdlib["at"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
        if len(args)!=2 { return "",errors.New("Bad arguments (count) in at()") }
        if sf("%T",args[0])!="int" || sf("%T",args[1])!="int" { return "",errors.New("Bad arguments (type) (not int) in at()") }
        return sat(args[0].(int),args[1].(int)), nil
    }

    // tr() - bad version of tr, that doesn't actually translate :)  really needs to not append with addition nor use bytes instead of runes. Probably quite slow.
    // arg 0 -> input string
    // arg 1 -> "d" delete
    // arg 1 -> "s" squeeze
    // arg 2 -> operand char set
    // @note: we should probably add the character translate to this, needs an argument #3...

    slhelp["tr"] = LibHelp{in: "string,action,case_string", out: "string", action: "delete (action 'd') or squeeze (action 's') extra characters (in [#i1]case_string[#i0]) from [#i1]string[#i0]."}
    stdlib["tr"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
        if len(args) != 3 {
            return "", errors.New("Bad arguments to tr()")
        }
        if reflect.TypeOf(args[0]).Name() != "string" || reflect.TypeOf(args[1]).Name() != "string" || reflect.TypeOf(args[2]).Name() != "string" {
            return "", errors.New("Bad arguments to tr()")
        }
        action := COPY
        if args[1].(string) == "d" {
            action = DELETE
        }
        if args[1].(string) == "s" {
            action = SQUEEZE
        }
        cases := args[2].(string)
        return tr(args[0].(string), action, cases), nil
    }

	slhelp["addansi"] = LibHelp{in: "string", out: "ansi_string", action: "Return a string with za colour codes replaced with ANSI values."}
	stdlib["addansi"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
		if len(args) != 1 {
			return "", errors.New("invalid argument (count) provided to addansi()")
		}
		if sf("%T", args[0])!="string" {
            return "", errors.New("invalid argument (type) provided to addansi()")
        }
        return sparkle(args[0].(string)),nil
	}

    slhelp["stripansi"] = LibHelp{in: "string", out: "string", action: "Remove escaped ansi codes."}
    stdlib["stripansi"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
        if sf("%T",args[0])!="string" { return "",errors.New("Bad arguments (type) to stripansi()") }
        if len(args)!=1 { return "",errors.New("Bad arguments (count) to stripansi()") }
        return Strip(args[0].(string)), nil
    }

    slhelp["stripquotes"] = LibHelp{in: "string", out: "string", action: "Remove outer quotes (double, single or backtick)"}
    stdlib["stripquotes"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
        if len(args)!=1 { return "",errors.New("Bad arguments (count) to stripansi()") }
        if sf("%T",args[0])!="string" { return "",errors.New("Bad arguments (type) to stripquotes()") }
        s:=args[0].(string)
        if hasOuter(s,'"') { return stripOuter(s,'"'),nil }
        if hasOuter(s,'\'') { return stripOuter(s,'\''),nil }
        if hasOuter(s,'`') { return stripOuter(s,'`'),nil }
        return s,nil
    }

    slhelp["lower"] = LibHelp{in: "string", out: "string", action: "Convert to lower-case."}
    stdlib["lower"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
        if len(args)!=1 { return "",errors.New("Bad arguments (count) to lower()") }
        return str.ToLower(args[0].(string)), nil
    }

    slhelp["upper"] = LibHelp{in: "string", out: "string", action: "Convert to upper-case."}
    stdlib["upper"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
        if len(args)!=1 { return "",errors.New("Bad arguments (count) to upper()") }
        return str.ToUpper(args[0].(string)), nil
    }

    slhelp["line_add"] = LibHelp{in: "var,string", out: "string", action: "Append a line to array string [#i1]var[#i0]."}
    stdlib["line_add"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
        if len(args) != 2 {
            return "", errors.New("Error: invalid argument count.\n")
        }
        src := args[0].(string)
        app := args[1].(string)
        nl := "\n"
        if src[len(src)-1] == '\n' {
            nl = ""
        }
        src = src + nl + app
        return src, err
    }

    slhelp["line_add_before"] = LibHelp{in: "string,regex_string,string", out: "string", action: "Inserts a new line in string ahead of the first matching [#i1]regex_string[#i0]."}
    stdlib["line_add_before"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {

        if len(args) != 3 {
            return "", errors.New("invalid argument count in line_add_before()")
        }
        if sf("%T",args[0])!="string" || sf("%T",args[1])!="string" || sf("%T",args[2])!="string" {
            return "", errors.New("invalid argument types in line_add_before()")
        }

        src := args[0].(string)
        regex := args[1].(string)
        app := args[2].(string)
        elf := false

        if src[len(src)-1] == '\n' {
            elf = true
        }

        var r []string
        pastFirst := false
        lsep:="\n"
        if runtime.GOOS!="windows" {
            r = str.Split(src, "\n")
        } else {
            r = str.Split(str.Replace(src, "\r\n", "\n", -1), "\n")
            lsep="\r\n"
        }

        var s string
        for _, l := range r {
            if match, _ := regexp.MatchString(regex, l); match && !pastFirst {
                s = s + app + lsep
                pastFirst = true
            }
            s = s + l + lsep
        }
        if !elf {
            s = s[:len(s)-len(lsep)]
        }
        return s, nil
    }

    slhelp["line_add_after"] = LibHelp{in: "var,regex,string", out: "string", action: "Inserts a new line to array string [#i1]var[#i0] after the first matching [#i1]regex[#i0]."}
    stdlib["line_add_after"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
        if len(args) != 3 {
            return "", errors.New("Error: invalid argument count.\n")
        }
        src := args[0].(string)
        regex := args[1].(string)
        app := args[2].(string)
        elf := false

        if src[len(src)-1] == '\n' {
            elf = true
        }
        var s string
        pastFirst := false

        var r []string
        lsep:="\n"
        if runtime.GOOS!="windows" {
            r = str.Split(src, "\n")
        } else {
            r = str.Split(str.Replace(src, "\r\n", "\n", -1), "\n")
            lsep="\r\n"
        }

        for _, l := range r {
            s = s + l + lsep
            if match, _ := regexp.MatchString(regex, l); match && !pastFirst {
                s = s + app + lsep
                pastFirst = true
            }
        }
        if !elf {
            s = s[:len(s)-len(lsep)]
        }
        return s, nil
    }

    slhelp["line_delete"] = LibHelp{in: "var,regex", out: "string", action: "Remove lines from array string [#i1]var[#i0] which match [#i1]regex[#i0]."}
    stdlib["line_delete"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
        if len(args) != 2 {
            return "", errors.New("Error: invalid argument count.\n")
        }
        src := args[0].(string)
        regex := args[1].(string)
        elf := false

        if src[len(src)-1] == '\n' {
            elf = true
        }

        var s string
        var r []string

        lsep:="\n"
        lseplen:=1
        if runtime.GOOS!="windows" {
            r = str.Split(src, "\n")
        } else {
            r = str.Split(str.Replace(src, "\r\n", "\n", -1), "\n")
            lsep="\r\n"
            lseplen=2
        }

        for _, l := range r {
            if match, _ := regexp.MatchString(regex, l); !match {
                s = s + l + lsep
            }
        }

        // remove generated last separator
        s=s[:len(s)-lseplen]

        // add back in from original if it existed
        if elf {
            s += lsep
        }

        return s, nil
    }

    slhelp["line_replace"] = LibHelp{in: "var,regex,replacement", out: "string", action: "Replaces lines in [#i1]var[#i0] that match [#i1]regex[#i0] with [#i1]replacement[#i0]."}
    stdlib["line_replace"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {

        if len(args) != 3 {
            return "", errors.New("Error: invalid argument count.\n")
        }

        src := args[0].(string)
        regex := args[1].(string)
        repl := args[2].(string)

        // check if last char of original is a newline and remove it
        elf := false
        if src[len(src)-1] == '\n' {
            elf = true
            src = src[:len(src)-1]
        }

        // trim right-most newline from replacement
        if repl[len(repl)-1] == '\n' {
            repl = repl[:len(repl)-1]
        }

        var s string
        var r []string
        lsep:="\n"
        if runtime.GOOS!="windows" {
            r = str.Split(src, "\n")
        } else {
            r = str.Split(str.Replace(src, "\r\n", "\n", -1), "\n")
            lsep="\r\n"
        }
        for _, l := range r {
            if match, _ := regexp.MatchString(regex, l); match {
                s = s + repl + lsep
            } else {
                s = s + l + lsep
            }
        }
        // if original did not have a trailing newline then remove
        if !elf && s[len(s)-1] == '\n' {
            s = s[:len(s)-1]
        }

        return s, nil
    }

    slhelp["pad"] = LibHelp{in: "string,justify,width[,padchar]", out: "string", action: "Return left (-1), centred (0) or right (1) justified, padded string."}
    stdlib["pad"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
        if len(args) < 3 || len(args) > 4 {
            return "", errors.New("bad argument count in pad()")
        }
        j, jbad := GetAsInt(args[1])
        w, wbad := GetAsInt(args[2])
        if jbad || wbad {
            pf("[j%v,w%v] ", j, w)
            return "", errors.New("bad args")
        }
        if len(args) == 4 {
            return pad(args[0].(string), j, w, args[3].(string)), err
        }
        if len(args) == 3 {
            return pad(args[0].(string), j, w, " "), err
        }
        return "", err
    }



    slhelp["field"] = LibHelp{in: "input_string,position[,optional_separator]", out: "string", action: "Retrieves columnar field [#i1]position[#i0] from [#i1]input_string[#i0]. String is empty on failure."}
    stdlib["field"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {

        // get sep
        sep := " "
        if len(args) == 3 {
            sep = args[2].(string)
        }

        switch args[0].(type) {
        case string:
        default:
            return "",errors.New("Bad args (type) in field()")
        }

        lf:="\r\n"
        fstr:=str.TrimSuffix(args[0].(string),lf)

        if len(args) > 0 && len(args) <= 3 {
            // get position
            pos := args[1].(int)

            // find column <position>
            f := func(c rune) bool {
                return str.ContainsRune(sep, c)
            }

            ta := str.FieldsFunc(fstr, f)
            if pos > 0 && pos <= len(ta) {
                return ta[pos-1], nil
            }
        }
        return "", nil

    }

    slhelp["fields"] = LibHelp{in: "input_string[,optional_separator]", out: "int", action: "Splits up [#i1]input_string[#i0] into variables in the current namespace. Variables are named [#i1]F1[#i0] through to [#i1]Fn[#i0]. Field count is stored in [#i1]NF[#i0]. Returns -1 on error, or field count."}
    stdlib["fields"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {

        // purge previous
        vset(evalfs,"F",[]string{})
        vset(evalfs,"NF",0)

        // check arguments
        sep := " "
        if len(args) > 0 {
            if len(args) == 2 {
                sep = args[1].(string)
            }
        } else {
            return -1, err
        }

        switch args[0].(type) {
        case string:
        default:
            return "",errors.New("Bad args (type) in fields()")
        }

        lf:="\r\n"
        fstr:=str.TrimRight(args[0].(string),lf)

        f := func(c rune) bool {
            return str.ContainsRune(sep, c)
        }
        ta := str.FieldsFunc(fstr, f)

        // populate F array and F1..Fx variables
        var c int
        for c = 0; c < len(ta); c++ {
            vset(evalfs, "F"+strconv.Itoa(c+1), ta[c])
        }
        vset(evalfs, "F", ta)
        vset(evalfs, "NF", c)

        return c, err
    }

    slhelp["split"] = LibHelp{in: "string[,fs]", out: "[]list", action: "Returns [#i1]string[#i0] as a list, breaking the string on [#i1]fs[#i0]."}
    stdlib["split"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
        var strIn string
        if len(args)>0 {
            strIn=args[0].(string)
        } else {
            return nil,nil
        }
        fs:=" "
        if len(args)>1 {
            fs=args[1].(string)
        }
        // all okay...
        return str.Split(strIn, fs),nil
    }

    slhelp["join"] = LibHelp{in: "[]string_list[,fs]", out: "string", action: "Returns a string with all elements of [#i1]string_list[#i0] concatenated, separated by [#i1]fs[#i0]."}
    stdlib["join"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
        var ary []string
        if len(args)>0 {
            switch args[0].(type) {
            case []string:
                ary=args[0].([]string)
            case []interface{}:
                for _,v:=range args[0].([]interface{}) {
                    ary=append(ary,sf("%v",v))
                }
            default:
                return "",errors.New("Bad args (type) in join()")
            }
        }
        fs:=""
        if len(args)>1 {
            fs=args[1].(string)
        }
        // all okay...
        return str.Join(ary[:], fs), nil
    }

    slhelp["collapse"] = LibHelp{in: "string", out: "string", action: "Turns a newline separated string into a space separated string."}
    stdlib["collapse"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
        if len(args)!=1 { return "",errors.New("Bad args (count) in collapse()") }
        if sf("%T",args[0])!="string" {
            return "",errors.New("Bad args (type) in collapse()")
        }
        return str.TrimSpace(tr(str.Replace(args[0].(string), "\n", " ",-1),SQUEEZE," ")),nil
    }


    slhelp["count"] = LibHelp{in: "string_name", out: "integer", action: "Returns the number of lines in [#i1]string_name[#i0]."}
    stdlib["count"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {

        if len(args) != 1 {
            return 0, err
        }

        switch v := args[0].(type) {
        case []string:
            return len(v), nil
        case string:
            if args[0].(string) == "" {
                return 0, nil
            }

            var ary []string
            if runtime.GOOS!="windows" {
                ary = str.SplitAfterN(args[0].(string), "\n",-1)
            } else {
                ary = str.SplitAfterN(str.Replace(args[0].(string), "\r\n", "\n", -1), "\n",-1)
            }

            return len(ary), nil
        }
        return nil, err
    }

    slhelp["lines"] = LibHelp{in: "string_name,string_range", out: "string", action: "Returns lines from [#i1]string_name[#i0]. [#i1]string_range[#i0] is specified in the form [#i1]start:end[#i0]. Either optional term can be [#i1]last[#i0] to indicate the last line of the file. Numbering starts from 0."}
    stdlib["lines"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {

        if len(args) == 2 {

            var ary []string

            var lsep="\n"
            if runtime.GOOS!="windows" {
                lsep="\r\n"
            }

            switch args[0].(type) {
            case string:

                if runtime.GOOS!="windows" {
                    ary = str.Split(args[0].(string), "\n")
                } else {
                    ary = str.Split(str.Replace(args[0].(string), "\r\n", "\n", -1), "\n")
                }

                if ary[len(ary)-1] == "" {
                    ary = ary[0 : len(ary)-1]
                }

            case []string:
                ary = args[0].([]string)
            }

            r := str.Split(args[1].(string), ":")

            start := -1
            end := -1

            if len(r) > 0 {
                if str.ToLower(r[0]) == "last" {
                    start = len(ary) - 1
                } else {
                    if r[0] != "" {
                        start, _ = strconv.Atoi(r[0])
                    }
                }
                if len(r) > 1 {
                    if str.ToLower(r[1]) == "last" {
                        end = len(ary)
                    } else {
                        if r[1] != "" {
                            end, _ = strconv.Atoi(r[1])
                            end++
                            if end > len(ary) {
                                end = len(ary)
                            }
                        }
                    }
                } else {
                    end = start + 1
                }
            }

            if end == -1 {
                end = len(ary)
            }
            if start == -1 {
                start = 0
            }

            return str.Join(ary[start:end], lsep), err
        }

        return "", err
    }

    slhelp["line_head"] = LibHelp{in: "nl_string,count", out: "nl_string", action: "Returns the top [#i1]count[#i0] lines of [#i1]nl_string[#i0]."}
    stdlib["line_head"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {

        if len(args)!=2 { return "",errors.New("Bad args (count) to line_head()") }
        if sf("%T",args[0])!="string" || sf("%T",args[1])!="int" {
            return "",errors.New("Bad args (type) to line_head()")
        }

        s:=args[0].(string)
        var list []string

        lsep:="\n"
        if runtime.GOOS!="windows" {
            list = str.Split(s,"\n")
        } else {
            list = str.Split(str.Replace(s, "\r\n", "\n", -1), "\n")
            lsep="\r\n"
        }

        llen:=len(list)
        count:=args[1].(int)
        if count>llen { count=llen }

        var ns str.Builder
        ns.Grow(100)
        for k:=0; k<count; k++ {
            ns.WriteString(list[k]+lsep)
        }

        // always remove trailing lsep 
        s=ns.String()
        if s[len(s)-1] == '\n' {
            s = s[:len(s)-len(lsep)]
        }

        return s,nil

    }

    slhelp["line_tail"] = LibHelp{in: "nl_string,count", out: "nl_string", action: "Returns the last [#i1]count[#i0] lines of [#i1]nl_string[#i0]."}
    stdlib["line_tail"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {

        if len(args)!=2 { return "",errors.New("Bad args (count) to line_tail()") }
        if sf("%T",args[0])!="string" || sf("%T",args[1])!="int" {
            return "",errors.New("Bad args (type) to line_tail()")
        }

        s:=args[0].(string)
        var list []string

        lsep:="\n"
        if runtime.GOOS!="windows" {
            list = str.Split(s, "\n")
        } else {
            list = str.Split(str.Replace(s, "\r\n", "\n", -1), "\n")
            lsep="\r\n"
        }

        llen:=len(list)
        count:=args[1].(int)
        start:=llen-count
        if start<0 { start=0 }

        var ns str.Builder
        ns.Grow(100)
        for k:=start; k<llen; k++ {
            ns.WriteString(list[k]+lsep)
        }

        // always remove trailing lsep 
        s=ns.String()
        if s[len(s)-1] == '\n' {
            s = s[:len(s)-len(lsep)]
        }

        return s,nil

    }

    slhelp["line_match"] = LibHelp{in: "nl_string,regex", out: "bool", action: "Does [#i1]nl_string[#i0] contain a match for regular expression [#i1]regex[#i0] on any line?"}
    stdlib["line_match"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {

        if len(args)!=2 { return false,errors.New("Bad arguments (count) in line_match()") }
        var val string
        var reg string
        switch args[0].(type) {
        case string:
            val=args[0].(string)
        default:
            return false,errors.New("Bad argument #1 (type) in line_match()")
        }
        switch args[1].(type) {
        case string:
            reg=args[1].(string)
        default:
            return false,errors.New("Bad argument #2 (type) in line_match()")
        }

        var r []string
        if runtime.GOOS!="windows" {
            r = str.Split(val, "\n")
        } else {
            r = str.Split(str.Replace(val, "\r\n", "\n", -1), "\n")
        }

        for _,v:=range r {
            if m,_:=regexp.MatchString(reg, v);m { return true,nil }
        }
        return false,nil

    }


    // int=next_match(s,regex,start_line) # to return matching line number (0 based)
    slhelp["next_match"] = LibHelp{in: "nl_string,regex,start_line", out: "int", action: "Returns the next line number which contains the [#i1]regex[#i0] in [#i1]nl_string[#i0]. -1 is returned on no match."}
    stdlib["next_match"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {


        if len(args)!=3 { return false,errors.New("Bad arguments (count) in next_match()") }
        var val string
        var reg string
        switch args[0].(type) {
        case string:
            val=args[0].(string)
        default:
            return -1,errors.New("Bad argument #1 (type) in next_match()")
        }
        switch args[1].(type) {
        case string:
            reg=args[1].(string)
        default:
            return -1,errors.New("Bad argument #2 (type) in next_match()")
        }

        startcount:=0
        switch args[2].(type) {
        case int:
            startcount=args[2].(int)
        default:
            return -1,errors.New("Bad argument #3 (type) in next_match()")
        }

        var r []string
        if runtime.GOOS!="windows" {
            r = str.Split(val, "\n")
        } else {
            r = str.Split(str.Replace(val, "\r\n", "\n", -1), "\n")
        }

        for curpos,v:=range r {
            if curpos>=startcount {
                if m,_:=regexp.MatchString(reg, v);m { return curpos, nil }
            }
        }
        return -1,nil

    }

    slhelp["line_filter"] = LibHelp{in: "nl_string,regex", out: "nl_string", action: "Returns lines from [#i1]nl_string[#i0] where regular expression [#i1]regex[#i0] matches."}
    stdlib["line_filter"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {

        if len(args)!=2 { return false,errors.New("Bad arguments (count) in line_filter()") }
        var val string
        var reg string
        switch args[0].(type) {
        case string:
            val=args[0].(string)
        default:
            return false,errors.New("Bad argument #1 (type) in line_filter()")
        }
        switch args[1].(type) {
        case string:
            reg=args[1].(string)
        default:
            return false,errors.New("Bad argument #2 (type) in line_filter()")
        }

        var list []string
        lsep:="\n"
        if runtime.GOOS!="windows" {
            list = str.Split(val, "\n")
        } else {
            list = str.Split(str.Replace(val, "\r\n", "\n", -1), "\n")
            lsep="\r\n"
        }

        var ns str.Builder
        ns.Grow(100)
        for _,v:=range list {
            if m,_:=regexp.MatchString(reg,v); m { ns.WriteString(v+lsep) }
        }

        // trim right-most newline from replacement
        repl:=ns.String()
        if len(repl)>0 {
            if repl[len(repl)-1] == '\n' {
                repl = repl[:len(repl)-1]
            }
        }
        return repl,nil

    }


    slhelp["match"] = LibHelp{in: "string,regex", out: "bool", action: "Does [#i1]string[#i0] contain a match for regular expression [#i1]regex[#i0]?"}
    stdlib["match"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
        if len(args) == 2 {
            if sf("%T",args[0])=="string" && sf("%T",args[1])=="string" {
                return regexp.MatchString(args[1].(string), args[0].(string))
            } else {
                return false, errors.New("match() only accepts strings.")
            }
        }
        return false, err
    }

    slhelp["filter"] = LibHelp{in: "string,regex,count", out: "string", action: "Returns a string matching the regular expression [#i1]regex[#i0] in [#i1]string[#i0]. count should be -1 for all matches."}
    stdlib["filter"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
        if len(args) >1 {
            count:=0
            if len(args)>2 {
                if sf("%T",args[2])=="int" {
                    count=args[2].(int)
                }
            }
            if sf("%T",args[0])=="string" && sf("%T",args[1])=="string" {
                re, err := regexp.Compile(args[1].(string))
                if err == nil {
                    if count==0 {
                        m := re.FindString(args[0].(string))
                        return m, nil
                    } else {
                        m := re.FindAllString(args[0].(string), count)
                        return m, nil
                    }
                }
                return "", err
            } else {
                return false, errors.New("filter() only accepts strings.")
            }
        }
        return "", err
    }

    slhelp["substr"] = LibHelp{in: "string,int_s,int_l", out: "string", action: "Returns a sub-string of [#i1]string[#i0], from position [#i1]int_s[#i0] with length [#i1]int_l[#i0]."}
    stdlib["substr"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
        if len(args) == 3 {
            if sf("%T",args[0])!="string" || sf("%T",args[1])!="int" || sf("%T",args[2])!="int" {
                return "",errors.New("Bad arguments (type) to substr()")
            }
            if args[1].(int)>=len(args[0].(string)) || args[2].(int)>len(args[0].(string)) {
                return "",errors.New("Bad argument (range) in substr()")
            }
            return args[0].(string)[args[1].(int) : args[1].(int)+args[2].(int)], err
        }
        return false, err
    }

    // strpos(s,sub,start)
    slhelp["strpos"] = LibHelp{in: "string,substring[,start_pos]", out: "int_position", action: "Returns the position of the next match of [#i1]substring[#i0] in [#i1]string[#i0]. Returns -1 if no match found."}
    stdlib["strpos"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
        if len(args)<2 || len(args)>3 { return -1,errors.New("Bad arguments (count) in strpos()") }
        start:=0
        if len(args)==3 {
            if sf("%T",args[2])=="int" {
                start=args[2].(int)
            } else {
                return -1,errors.New("Bad arguments (type) in strpos()")
            }
        }
        if sf("%T",args[0])!="string" || sf("%T",args[1])!="string" {
            return -1,errors.New("Bad arguments (type) in strpos()")
        }
        p:=str.Index(args[0].(string)[start:],args[1].(string))
        if p!=-1 { p+=start }
        return p,nil
    }


    slhelp["gsub"] = LibHelp{in: "string,string_m,string_s", out: "string", action: "Returns [#i1]string[#i0] with all matches of [#i1]string_m[#i0] replaced with [#i1]string_s[#i0]."}
    stdlib["gsub"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
        if len(args)!=3 { return "",errors.New("Bad arguments (count) to gsub()") }
        if sf("%T",args[0])!="string" || sf("%T",args[1])!="string" || sf("%T",args[2])!="string" {
            return "",errors.New("Bad arguments (type) to gsub()")
        }
        return str.Replace(args[0].(string), args[1].(string), args[2].(string), -1), err
    }

    slhelp["trim"] = LibHelp{in: "string,int_type", out: "string", action: "Removes whitespace from [#i1]string[#i0], depending on [#i1]int_type[#i0]. -1 ltrim, 0 both, 1 rtrim."}
    stdlib["trim"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {

        if len(args) < 2 || len(args)>3 { return "",errors.New("Bad arguments (count) to trim()") }
        if sf("%T",args[0])!="string" || sf("%T",args[1])!="int" { return "",errors.New("Bad arguments (type) in trim()") }

        removals:=" \t"
        if len(args)==3 {
            if sf("%T",args[2])!="string" {
                 return "",errors.New("Bad arguments (type) in trim()")
            }
            removals=args[2].(string)
        }

        switch args[1].(int) {
        case -1:
            return str.TrimLeft(args[0].(string), removals), nil
        case 0:
            return str.Trim(args[0].(string), removals), nil
        case 1:
            return str.TrimRight(args[0].(string), removals), nil
        }

        return "", err
    }


    slhelp["start"] = LibHelp{in: "string1,string2", out: "bool", action: "Does [#i1]string1[#i0] begin with [#i1]string2[#i0]?"}
    stdlib["start"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {

        if len(args) != 2 { return "",errors.New("Bad arguments (count) to start()") }
        if sf("%T",args[0])!="string" || sf("%T",args[1])!="string" { return "",errors.New("Bad arguments (type) in start()") }

        return str.HasPrefix(args[0].(string), args[1].(string)), nil

    }

    slhelp["end"] = LibHelp{in: "string1,string2", out: "bool", action: "Does [#i1]string1[#i0] end with [#i1]string2[#i0]?"}
    stdlib["end"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {

        if len(args) != 2 { return "",errors.New("Bad arguments (count) to end()") }
        if sf("%T",args[0])!="string" || sf("%T",args[1])!="string" { return "",errors.New("Bad arguments (type) in end()") }

        return str.HasSuffix(args[0].(string), args[1].(string)), err

    }

}
