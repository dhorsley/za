//+build !test

package main

import (
    "errors"
    "reflect"
    "regexp"
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

func ulen(args []interface{}) (int,error) {
    if len(args) == 1 {
        switch args[0].(type) {
        case []int:
            return len(args[0].([]int)), nil
        case []bool:
            return len(args[0].([]bool)), nil
        case []float64:
            return len(args[0].([]float64)), nil
        case []string:
            return len(args[0].([]string)), nil
        case map[string]interface{}:
            return len(args[0].(map[string]interface{})), nil
        case string:
            return len(args[0].(string)), nil
        case []interface{}:
            return len(args[0].([]interface{})), nil
        }
    }
    return -1, nil
}


func buildStringLib() {

    // string handling

    features["string"] = Feature{version: 1, category: "text"}
    categories["string"] = []string{"pad", "len", "length", "field", "fields", "pipesep", "get_value", "start", "end", "match", "filter",
        "substr", "gsub", "replace", "trim", "lines", "count",
        "line_add", "line_delete", "line_replace", "line_add_before", "line_add_after","line_match","line_filter","line_head","line_tail",
        "reverse", "tr", "lower", "upper", "format",
        "split", "join", "collapse","strpos",
    }

    slhelp["replace"] = LibHelp{in: "var,regex,replacement", out: "string", action: "Replaces matches found in [#i1]var[#i0] with [#i1]regex[#i0] to [#i1]replacement[#i0]."}
    stdlib["replace"] = func(args ...interface{}) (ret interface{}, err error) {
        if len(args) != 3 {
            return "", errors.New("Error: invalid argument count.\n")
        }
        src := args[0].(string)
        regex := args[1].(string)
        repl := args[2].(string)
        var re = regexp.MustCompile(regex)
        s := re.ReplaceAllString(src, repl)
        return s, nil
    }

    slhelp["get_value"] = LibHelp{in: "string_array,key_name", out: "string_value", action: "Returns the value of the key [#i1]key_name[#i0] in [#i1]string_array[#i0]."}
    stdlib["get_value"] = func(args ...interface{}) (ret interface{}, err error) {

        if len(args) != 2 {
            return "", errors.New("Error: invalid argument count.\n")
        }

        var search []string

        switch args[0].(type) {
        case string:
            search = str.Split(args[0].(string), "\n")
        case []string:
            search = args[0].([]string)
        default:
            return "", errors.New("Error: unsupported data type in get_value() source.")
        }

        key := args[1].(string)

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
    stdlib["reverse"] = func(args ...interface{}) (ret interface{}, err error) {
        if len(args)!=1 { return "",errors.New("Bad arguments (count) to reverse()") }
        switch args[0].(type) {
        case string:
            ln := len(args[0].(string)) - 1
            r := ""
            for i := ln; i >= 0; i-- {
                r = r + string(args[0].(string)[i])
            }
            return r, nil
        case []int, []int32, []int64:
            ln := len(args[0].([]int64)) - 1
            r := make([]int64, 0, ln+1)
            for i := ln; i >= 0; i-- {
                r = append(r, args[0].([]int64)[i])
            }
            return r, nil
        case []float32, []float64:
            ln := len(args[0].([]float64)) - 1
            r := make([]float64, 0, ln+1)
            for i := ln; i >= 0; i-- {
                r = append(r, args[0].([]float64)[i])
            }
            return r, nil
        case []interface{}:
            ln := len(args[0].([]interface{})) - 1
            r := make([]interface{}, 0, ln+1)
            for i := ln; i >= 0; i-- {
                r = append(r, args[0].([]interface{})[i])
            }
            return r, nil
        case []string:
            ln := len(args[0].([]string)) - 1
            r := make([]string, 0, ln+1)
            for i := ln; i >= 0; i-- {
                r[ln-i] = args[0].([]string)[i]
            }
            return r, nil
        }
        return nil, errors.New("could not reverse()")
    }

    // format() - as sprintf()
    slhelp["format"] = LibHelp{in: "string,var_args", out: "string", action: "Format the input string in the manner of fprintf()."}
    stdlib["format"] = func(args ...interface{}) (ret interface{}, err error) {
        if len(args)==0 { return "",errors.New("Bad arguments (count) in format()") }
        if sf("%T",args[0])!="string" { return "",errors.New("Bad arguments (type) (arg#1 not string) in format()") }
        if len(args) == 1 {
            return sf(args[0].(string)), nil
        }
        return sf(args[0].(string), args[1:]...), nil
    }

    // tr() - bad version of tr, that doesn't actually translate :)  really needs to not append with addition nor use bytes instead of runes. Probably quite slow.
    // arg 0 -> input string
    // arg 1 -> "d" delete
    // arg 1 -> "s" squeeze
    // arg 2 -> operand char set
    // @note: we should probably add the character translate to this, needs an argument #3...

    slhelp["tr"] = LibHelp{in: "string,action,case_string", out: "string", action: "delete (action 'd') or squeeze (action 's') extra characters (in [#i1]case_string[#i0]) from [#i1]string[#i0]."}
    stdlib["tr"] = func(args ...interface{}) (ret interface{}, err error) {
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

    // lower()
    slhelp["lower"] = LibHelp{in: "string", out: "string", action: "Convert to lower-case."}
    stdlib["lower"] = func(args ...interface{}) (ret interface{}, err error) {
        if len(args)!=1 { return "",errors.New("Bad arguments (count) to lower()") }
        return str.ToLower(args[0].(string)), nil
    }

    // upper()
    slhelp["upper"] = LibHelp{in: "string", out: "string", action: "Convert to upper-case."}
    stdlib["upper"] = func(args ...interface{}) (ret interface{}, err error) {
        if len(args)!=1 { return "",errors.New("Bad arguments (count) to upper()") }
        return str.ToUpper(args[0].(string)), nil
    }

    slhelp["line_add"] = LibHelp{in: "var,string", out: "string", action: "Append a line to array string [#i1]var[#i0]."}
    stdlib["line_add"] = func(args ...interface{}) (ret interface{}, err error) {
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

    slhelp["line_add_before"] = LibHelp{in: "var,regex,string", out: "string", action: "Inserts a new line to array string [#i1]var[#i0] ahead of the first matching [#i1]regex[#i0]."}
    stdlib["line_add_before"] = func(args ...interface{}) (ret interface{}, err error) {
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
        for _, l := range str.Split(src, "\n") {
            if match, _ := regexp.MatchString(regex, l); match && !pastFirst {
                s = s + app + "\n"
                pastFirst = true
            }
            s = s + l + "\n"
        }
        if elf {
            s = s[:len(s)-1]
        }
        return s, nil
    }

    slhelp["line_add_after"] = LibHelp{in: "var,regex,string", out: "string", action: "Inserts a new line to array string [#i1]var[#i0] after the first matching [#i1]regex[#i0]."}
    stdlib["line_add_after"] = func(args ...interface{}) (ret interface{}, err error) {
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
        for _, l := range str.Split(src, "\n") {
            s = s + l + "\n"
            if match, _ := regexp.MatchString(regex, l); match && !pastFirst {
                s = s + app + "\n"
                pastFirst = true
            }
        }
        if elf {
            s = s[:len(s)-1]
        }
        return s, nil
    }

    slhelp["line_delete"] = LibHelp{in: "var,regex", out: "string", action: "Remove lines from array string [#i1]var[#i0] which match [#i1]regex[#i0]."}
    stdlib["line_delete"] = func(args ...interface{}) (ret interface{}, err error) {
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
        for _, l := range str.Split(src, "\n") {
            if match, _ := regexp.MatchString(regex, l); !match {
                s = s + l + "\n"
            }
        }
        if elf {
            s = s[:len(s)-1]
        }
        return s, nil
    }

    slhelp["line_replace"] = LibHelp{in: "var,regex,replacement", out: "string", action: "Replaces lines in [#i1]var[#i0] that match [#i1]regex[#i0] with [#i1]replacement[#i0]."}
    stdlib["line_replace"] = func(args ...interface{}) (ret interface{}, err error) {

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
        for _, l := range str.Split(src, "\n") {
            if match, _ := regexp.MatchString(regex, l); match {
                s = s + repl + "\n"
            } else {
                s = s + l + "\n"
            }
        }
        // if original did not have a trailing newline then remove
        if !elf && s[len(s)-1] == '\n' {
            s = s[:len(s)-1]
        }

        return s, nil
    }

    slhelp["pad"] = LibHelp{in: "string,justify,width,character", out: "string", action: "Return left (-1), centred (0) or right (1) justified, padded string."}
    stdlib["pad"] = func(args ...interface{}) (ret interface{}, err error) {
        if len(args) < 3 || len(args) > 4 {
            return "", errors.New("bad argument count in pad()")
        }
        j, jbad := GetAsInt(args[1])
        w, wbad := GetAsInt(args[2])
        if jbad || wbad {
            pf("[j->%#v,w->%#v] ", j, w)
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


    slhelp["len"] = LibHelp{in: "string", out: "integer", action: "Returns length of string or list."}
    stdlib["len"] = func(args ...interface{}) (ret interface{}, err error) {
        return ulen(args)
    }
    slhelp["length"] = LibHelp{in: "string", out: "integer", action: "Returns length of string or list."}
    stdlib["length"] = func(args ...interface{}) (ret interface{}, err error) {
        return ulen(args)
    }


    slhelp["field"] = LibHelp{in: "input_string,position,optional_separator", out: "", action: "Retrieves columnar field [#i1]position[#i0] from [#i1]input_string[#i0]. String is empty on failure."}
    stdlib["field"] = func(args ...interface{}) (ret interface{}, err error) {
        // get sep
        sep := " "
        if len(args) == 3 {
            sep = args[2].(string)
        }
        if len(args) > 0 && len(args) <= 3 {
            // get position
            pos := args[1].(int)
            fstr := args[0].(string)
            // squeeze separator repeats
            new := tr(fstr, SQUEEZE, sep)
            // find column <position>
            f := func(c rune) bool {
                return str.ContainsRune(sep, c)
            }
            ta := str.FieldsFunc(new, f)
            if pos > 0 && pos <= len(ta) {
                return ta[pos-1], nil
            }
        }
        return "", nil

    }

    slhelp["fields"] = LibHelp{in: "input_string,optional_separator", out: "", action: "Splits up [#i1]input_string[#i0] into variables in the current namespace. Variables are named [#i1]F1[#i0] through to [#i1]Fn[#i0]. Field count is stored in [#i1]NF[#i0]."}
    stdlib["fields"] = func(args ...interface{}) (ret interface{}, err error) {

        lastlock.RLock()
        lfs:=lastfs
        lastlock.RUnlock()

        // purge previous - allows for 32 fields. this should be changed as part of removing arbitrary limits.
        vset(lfs,"F",[]string{})
        vset(lfs,"NF",0)

        // check arguments
        sep := " "
        if len(args) > 0 {
            if len(args) == 2 {
                sep = args[1].(string)
            }
        } else {
            return -1, err
        }

        // perform squeeze and split
        fstr := args[0].(string)
        new := tr(fstr, SQUEEZE, sep)
        f := func(c rune) bool {
            return str.ContainsRune(sep, c)
        }
        ta := str.FieldsFunc(new, f)

        // populate F array and F1..Fx variables
        // vset(lfs, "F", []string{})
        var c int
        for c = 0; c < len(ta); c++ {
            vset(lfs, "F"+strconv.Itoa(c+1), ta[c])
            v, _ := vget(lfs, "F")
            vset(lfs, "F", append(v.([]string), ta[c]))
        }
        vset(lfs, "NF", c)

        return c, err
    }

    slhelp["pipesep"] = LibHelp{in: "input_string", out: "", action: "deprecated."}
        // Splits [#i1]input_string[#i0] into variables named [#i1]F1[#i0] through to [#i1]Fn[#i0]. The split is performed at pipe (|) symbols. Field count is stored in [#i1]NF[#i0]."}
    stdlib["pipesep"] = func(args ...interface{}) (ret interface{}, err error) {

        lastlock.RLock()
        lfs:=lastfs
        lastlock.RUnlock()

        fsep := func(c rune) bool { return c == '|' }
        if len(args) == 1 {

            ta := str.FieldsFunc(args[0].(string), fsep)
            var c int
            for c = 0; c < len(ta); c++ {
                vset(lfs, "F"+strconv.Itoa(c+1), ta[c])
                v, _ := vget(lfs, "F")
                vset(lfs, "F", append(v.([]string), ta[c]))
            }
            vset(lfs, "NF", c)
        }
        return nil, err
    }


    slhelp["split"] = LibHelp{in: "string[,fs]", out: "list", action: "Returns [#i1]string[#i0] as a list, breaking the string on [#i1]fs[#i0]."}
    stdlib["split"] = func(args ...interface{}) (ret interface{}, err error) {
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

    slhelp["join"] = LibHelp{in: "string_list[,fs]", out: "string", action: "Returns a string with all elements of [#i1]string_list[#i0] concatenated, separated by [#i1]fs[#i0]."}
    stdlib["join"] = func(args ...interface{}) (ret interface{}, err error) {
        var ary []string
        if len(args)>0 {
            switch args[0].(type) {
            case []interface{}:
                for _,v:=range args[0].([]interface{}) {
                    ary=append(ary,sf("%v",v))
                }
            case []string:
                ary=args[0].([]string)
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
    stdlib["collapse"] = func(args ...interface{}) (ret interface{}, err error) {
        if len(args)!=1 { return "",errors.New("Bad args (count) in collapse()") }
        if sf("%T",args[0])!="string" {
            return "",errors.New("Bad args (type) in collapse()")
        }
        return str.TrimSpace(tr(str.Replace(args[0].(string), "\n", " ",-1),SQUEEZE," ")),nil
    }


    slhelp["count"] = LibHelp{in: "string_name", out: "integer", action: "Returns the number of lines in [#i1]string_name[#i0]."}
    stdlib["count"] = func(args ...interface{}) (ret interface{}, err error) {

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
            ary := str.SplitAfterN(args[0].(string), "\n", -1)
            return len(ary), nil
        }
        return nil, err
    }

    slhelp["lines"] = LibHelp{in: "string_name,string_range", out: "string", action: "Returns lines from [#i1]string_name[#i0]. [#i1]string_range[#i0] is specified in the form [#i1]start:end[#i0]. Either optional term can be [#i1]last[#i0] to indicate the last line of the file. Numbering starts from 0."}
    stdlib["lines"] = func(args ...interface{}) (ret interface{}, err error) {

        if len(args) == 2 {

            var ary []string

            switch args[0].(type) {
            case string:
                ary = str.Split(args[0].(string), "\n")
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

            return str.Join(ary[start:end], "\n"), err
        }

        return "", err
    }

    slhelp["line_head"] = LibHelp{in: "nl_string,count", out: "nl_string", action: "Returns the top [#i1]count[#i0] lines of [#i1]nl_string[#i0]."}
    stdlib["line_head"] = func(args ...interface{}) (ret interface{}, err error) {

        if len(args)!=2 { return "",errors.New("Bad args (count) to line_head()") }
        if sf("%T",args[0])!="string" || sf("%T",args[1])!="int" {
            return "",errors.New("Bad args (type) to line_head()")
        }

        list:=str.Split(args[0].(string),"\n")
        llen:=len(list)
        count:=args[1].(int)
        if count>llen { count=llen }

        var ns str.Builder
        ns.Grow(100)
        for k:=0; k<count; k++ {
            ns.WriteString(list[k]+"\n")
        }
        return ns.String(),nil

    }

    slhelp["line_tail"] = LibHelp{in: "nl_string,count", out: "nl_string", action: "Returns the last [#i1]count[#i0] lines of [#i1]nl_string[#i0]."}
    stdlib["line_tail"] = func(args ...interface{}) (ret interface{}, err error) {

        if len(args)!=2 { return "",errors.New("Bad args (count) to line_tail()") }
        if sf("%T",args[0])!="string" || sf("%T",args[1])!="int" {
            return "",errors.New("Bad args (type) to line_tail()")
        }

        list:=str.Split(args[0].(string),"\n")
        llen:=len(list)
        count:=args[1].(int)
        start:=llen-count
        if start<0 { start=0 }

        var ns str.Builder
        ns.Grow(100)
        for k:=start; k<llen; k++ {
            ns.WriteString(list[k]+"\n")
        }
        return ns.String(),nil

    }

    slhelp["line_match"] = LibHelp{in: "nl_string,regex", out: "bool", action: "Does [#i1]nl_string[#i0] contain a match for regular expression [#i1]regex[#i0] on any line?"}
    stdlib["line_match"] = func(args ...interface{}) (ret interface{}, err error) {

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

        for _,v:=range str.Split(val, "\n") {
            if m,_:=regexp.MatchString(reg, v);m { return true,nil }
        }
        return false,nil

    }

    slhelp["line_filter"] = LibHelp{in: "nl_string,regex", out: "nl_string", action: "Returns lines from [#i1]nl_string[#i0] where regular expression [#i1]regex[#i0] matches."}
    stdlib["line_filter"] = func(args ...interface{}) (ret interface{}, err error) {

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

        var ns str.Builder
        ns.Grow(100)
        for _,v:=range str.Split(val, "\n") {
            if m,_:=regexp.MatchString(reg,v); m { ns.WriteString(v+"\n") }
        }
        return ns.String(),nil

    }


    slhelp["match"] = LibHelp{in: "string,regex", out: "bool", action: "Does [#i1]string[#i0] contain a match for regular expression [#i1]regex[#i0]?"}
    stdlib["match"] = func(args ...interface{}) (ret interface{}, err error) {
        if len(args) == 2 {
            if sf("%T",args[0])=="string" && sf("%T",args[1])=="string" {
                return regexp.MatchString(args[1].(string), args[0].(string))
            } else {
                return false, errors.New("match() only accepts strings.")
            }
        }
        return false, err
    }

    slhelp["filter"] = LibHelp{in: "string,regex", out: "string", action: "Returns a string matching the regular expression [#i1]regex[#i0] in [#i1]string[#i0]."}
    stdlib["filter"] = func(args ...interface{}) (ret interface{}, err error) {
        if len(args) == 2 {
            if sf("%T",args[0])=="string" && sf("%T",args[1])=="string" {
                re, err := regexp.Compile(args[1].(string))
                if err == nil {
                    m := re.FindString(args[0].(string))
                    return m, err
                }
                return "", err
            } else {
                return false, errors.New("filter() only accepts strings.")
            }
        }
        return "", err
    }

    slhelp["substr"] = LibHelp{in: "string,int_s,int_l", out: "string", action: "Returns a sub-string of [#i1]string[#i0], from position [#i1]int_s[#i0] with length [#i1]int_l[#i0]."}
    stdlib["substr"] = func(args ...interface{}) (ret interface{}, err error) {
        if len(args) == 3 {
            if sf("%T",args[0])!="string" || sf("%T",args[1])!="int" || sf("%T",args[2])!="int" {
                return "",errors.New("Bad arguments (type) to substr()")
            }
            if args[1].(int)>=len(args[0].(string)) || args[2].(int)>=len(args[0].(string)) {
                return "",errors.New("Bad argument (range) in substr()")
            }
            return args[0].(string)[args[1].(int) : args[1].(int)+args[2].(int)], err
        }
        return false, err
    }

    // strpos(s,sub,start)
    slhelp["strpos"] = LibHelp{in: "string,substring[,start_pos]", out: "int_position", action: "Returns the position of the next match of [#i1]substring[#i0] in [#i1]string[#i0]. Returns -1 if no match found."}
    stdlib["strpos"] = func(args ...interface{}) (ret interface{}, err error) {
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
    stdlib["gsub"] = func(args ...interface{}) (ret interface{}, err error) {
        if len(args)!=3 { return "",errors.New("Bad arguments (count) to gsub()") }
        if sf("%T",args[0])!="string" || sf("%T",args[1])!="string" || sf("%T",args[2])!="string" {
            return "",errors.New("Bad arguments (type) to gsub()")
        }
        return str.Replace(args[0].(string), args[1].(string), args[2].(string), -1), err
    }

    slhelp["trim"] = LibHelp{in: "string,int_type", out: "string", action: "Removes whitespace from [#i1]string[#i0], depending on [#i1]int_type[#i0]. -1 ltrim, 0 both, 1 rtrim."}
    stdlib["trim"] = func(args ...interface{}) (ret interface{}, err error) {
        if len(args) == 2 {
            switch args[1].(int) {
            case -1:
                return str.TrimLeft(args[0].(string), " \t"), err
            case 0:
                return str.Trim(args[0].(string), " \t"), err
            case 1:
                return str.TrimRight(args[0].(string), " \t"), err
            }
        }
        return false, err
    }
    slhelp["start"] = LibHelp{in: "string1,string2", out: "bool", action: "Does [#i1]string1[#i0] begin with [#i1]string2[#i0]?"}
    stdlib["start"] = func(args ...interface{}) (ret interface{}, err error) {
        if len(args) == 2 {
            return str.HasPrefix(args[0].(string), args[1].(string)), err
        }
        return false, err
    }

    slhelp["end"] = LibHelp{in: "string1,string2", out: "bool", action: "Does [#i1]string1[#i0] end with [#i1]string2[#i0]?"}
    stdlib["end"] = func(args ...interface{}) (ret interface{}, err error) {
        if len(args) == 2 {
            return str.HasSuffix(args[0].(string), args[1].(string)), err
        }
        return false, err
    }

}
