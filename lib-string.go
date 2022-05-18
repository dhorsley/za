//+build !test

package main

import (
    "errors"
    "regexp"
    "runtime"
    "strconv"
    str "strings"
)

const ( // tr_actions
    COPY int = iota
    DELETE
    SQUEEZE
    TRANSLATE
)

func tr(s string, action int, cases string, xlates string) string {

    original := []byte(s)
    var lastChar byte
    var newStr str.Builder
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
        case TRANSLATE:
            // get strpos in cases, append to new string xlates[strpos]
            if p:=str.IndexByte(cases, v); p != -1 {
                newStr.WriteString(string(xlates[p]))
            } else {
                newStr.WriteString(string(v))
            }
        case DELETE:
            // copy to new string if not found in delete list
            if str.IndexByte(cases, v) == -1 {
                newStr.WriteString(string(v))
            }
        case SQUEEZE:
            if str.IndexByte(cases, v) != -1 {
                squeezing = true
                lastChar = v
            }
            newStr.WriteString(string(v)) // only copy char on first match
        }

    }
    return newStr.String()

}


func buildStringLib() {

    // string handling

    features["string"] = Feature{version: 1, category: "text"}
    categories["string"] = []string{"pad", "field", "fields", "get_value", "has_start", "has_end", "match", "filter",
        "substr", "gsub", "replace", "trim", "lines", "count","inset",
        "next_match", "line_add", "line_delete", "line_replace", "line_add_before", "line_add_after","line_match","line_filter","grep","line_head","line_tail",
        "reverse", "tr", "lower", "upper", "format", "ccformat","pos","bg256","fg256","bgrgb","fgrgb",
        "split", "join", "collapse","strpos","stripansi","addansi","stripquotes","stripcc","clean",
    }

    replaceCompileCache:=make(map[string]regexp.Regexp)

    slhelp["replace"] = LibHelp{in: "var,regex,replacement", out: "string", action: "Replaces matches found in [#i1]var[#i0] with [#i1]regex[#i0] to [#i1]replacement[#i0]."}
    stdlib["replace"] = func(evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("replace",args,1,"3","string","string","string"); !ok { return nil,err }

        src := args[0].(string)
        regex := args[1].(string)
        repl := args[2].(string)

        var re regexp.Regexp
        if pre,found:=replaceCompileCache[regex];!found {
            re = *regexp.MustCompile(regex)
            replaceCompileCache[regex]=re
        } else
        {
            re = pre
        }

        s := re.ReplaceAllString(src, repl)
        return s, nil
    }

    slhelp["get_value"] = LibHelp{in: "string_array,key_name", out: "string_value", action: "Returns the value of the key [#i1]key_name[#i0] in [#i1]string_array[#i0]."}
    stdlib["get_value"] = func(evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("get_value",args,2,
            "2","string","string",
            "2","[]string","string"); !ok { return nil,err }

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
            return "", errors.New("unsupported data type in get_value() source")
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


    slhelp["reverse"] = LibHelp{in: "list_or_string", out: "as_input", action: "Reverse the contents of a variable."}
    stdlib["reverse"] = func(evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("reverse",args,8,
            "1","string", "1","[]int", "1","[]int64",
            "1","[]float64", "1","[]string", "1","[]uint",
            "1","[]bool", "1","[]interface {}"); !ok { return nil,err }

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
        case []int64:
            ln := len(args[0].([]int64)) - 1
            r := make([]int64, 0, ln+1)
            for i := ln; i >= 0; i-- {
                r = append(r, args[0].([]int64)[i])
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
                r = append(r, args[0].([]string)[i])
                // was: r[ln-i] = args[0].([]string)[i]
            }
            return r, nil
        case []uint:
            ln := len(args[0].([]uint)) - 1
            r := make([]uint, 0, ln+1)
            for i := ln; i >= 0; i-- {
                r = append(r, args[0].([]uint)[i])
            }
            return r, nil
        case []bool:
            ln := len(args[0].([]bool)) - 1
            r := make([]bool, 0, ln+1)
            for i := ln; i >= 0; i-- {
                r = append(r, args[0].([]bool)[i])
            }
            return r, nil
        case []any:
            ln := len(args[0].([]any)) - 1
            r := make([]any, 0, ln+1)
            for i := ln; i >= 0; i-- {
                r = append(r, args[0].([]any)[i])
            }
            return r, nil
        }
        return nil, errors.New("could not reverse()")
    }

    slhelp["ccformat"] = LibHelp{in: "string,var_args", out: "string", action: "Format the input string in the manner of fprintf(). Also processes embedded colour codes to ANSI."}
    stdlib["ccformat"] = func(evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if len(args)==0 { return "",errors.New("Bad arguments (count) in ccformat()") }
        if sf("%T",args[0])!="string" { return "",errors.New("Bad arguments (type) (arg #1 not string) in ccformat()") }
        if len(args) == 1 {
            return sparkle(args[0].(string)), nil
        }
        return sparkle(sf(args[0].(string), args[1:]...)), nil
    }

    slhelp["format"] = LibHelp{in: "string,var_args", out: "string", action: "Format the input string in the manner of fprintf()."}
    stdlib["format"] = func(evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if len(args)==0 { return "",errors.New("Bad arguments (count) in format()") }
        if !strcmp(sf("%T",args[0]),"string") { return "",errors.New("Bad arguments (type) (first argument is not a string) in format()") }
        if len(args) == 1 {
            return args[0].(string), nil
        }
        return sf(args[0].(string), args[1:]...), nil
    }

    slhelp["pos"] = LibHelp{in: "int_row,int_col", out: "string", action: "Returns a cursor positioning ANSI code string for (row,col)."}
    stdlib["pos"] = func(evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("pos",args,1,"2","int","int"); !ok { return nil,err }
        return sat(args[0].(int),args[1].(int)), nil
    }

    slhelp["bg256"] = LibHelp{in: "int_colour", out: "string", action: "Returns an ANSI code string for expressing an 8-bit background colour code."}
    stdlib["bg256"] = func(evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("bg256",args,1,"1","number"); !ok { return nil,err }
        i,_:=GetAsInt(args[0])
        if ansiMode {
            return sf("\033[48;5;%dm",i),nil
        }
        return "",nil
    }

    slhelp["fg256"] = LibHelp{in: "int_colour", out: "string", action: "Returns an ANSI code string for expressing an 8-bit foreground colour code."}
    stdlib["fg256"] = func(evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("fg256",args,1,"1","number"); !ok { return nil,err }
        i,_:=GetAsInt(args[0])
        if ansiMode {
            return sf("\033[38;5;%dm",i),nil
        }
        return "",nil
    }

    slhelp["bgrgb"] = LibHelp{in: "int_r,int_g,int_b", out: "string", action: "Returns an ANSI code string for expressing an rgb background colour code."}
    stdlib["bgrgb"] = func(evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("bgrgb",args,1,"3","number","number","number"); !ok { return nil,err }
        r,_:=GetAsInt(args[0])
        g,_:=GetAsInt(args[1])
        b,_:=GetAsInt(args[2])
        if ansiMode {
            return sf("\033[48;2;%d;%d;%dm",r,g,b),nil
        }
        return "",nil
    }

    slhelp["fgrgb"] = LibHelp{in: "int_r,int_g,int_b", out: "string", action: "Returns an ANSI code string for expressing an rgb foreground colour code."}
    stdlib["fgrgb"] = func(evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("fgrgb",args,1,"3","number","number","number"); !ok { return nil,err }
        r,_:=GetAsInt(args[0])
        g,_:=GetAsInt(args[1])
        b,_:=GetAsInt(args[2])
        if ansiMode {
            return sf("\033[38;2;%d;%d;%dm",r,g,b),nil
        }
        return "",nil
    }

    slhelp["tr"] = LibHelp{in: "string,action,case_string[,translation_string]", out: "string", action: `delete (action "d") or squeeze (action "s") extra characters (in [#i1]case_string[#i0]) from [#i1]string[#i0]. translate (action "t") can be used, along with the optional [#i1]translation_string[#i0] to specify direct replacements for existing characters. Please note: this is a very restricted subset of the tr tool.`}
    stdlib["tr"] = func(evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("tr",args,2,
            "3","string","string","string",
            "4","string","string","string","string"); !ok { return nil,err }

        translations:=""
        if len(args)==4 {
            translations=args[3].(string)
        }

        if args[1].(string) == "d" {
            return tr(args[0].(string), DELETE, args[2].(string), translations), nil
        }
        if args[1].(string) == "s" {
            return tr(args[0].(string), SQUEEZE, args[2].(string), translations), nil
        }
        if args[1].(string) == "t" {
            return tr(args[0].(string), TRANSLATE, args[2].(string), translations), nil
        }
        return tr(args[0].(string), COPY, args[2].(string), translations), nil
    }

    slhelp["addansi"] = LibHelp{in: "string", out: "ansi_string", action: "Return a string with za colour codes replaced with ANSI values."}
    stdlib["addansi"] = func(evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("addansi",args,1,"1","string"); !ok { return nil,err }
        return sparkle(args[0].(string)),nil
    }

    slhelp["stripansi"] = LibHelp{in: "string", out: "string", action: "Remove escaped ansi codes."}
    stdlib["stripansi"] = func(evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("stripansi",args,1,"1","string"); !ok { return nil,err }
        return Strip(args[0].(string)), nil
    }

    slhelp["stripcc"] = LibHelp{in: "string", out: "string", action: "Remove Za colour codes from string."}
    stdlib["stripcc"] = func(evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("stripcc",args,1,"1","string"); !ok { return nil,err }
        return StripCC(args[0].(string)), nil
    }

    slhelp["clean"] = LibHelp{in: "string", out: "string", action: "Remove curly brace nests from a string. Use this to sanitise inputs."}
    stdlib["clean"] = func(evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("clean",args,1,"1","string"); !ok { return nil,err }
        return sanitise(args[0].(string)), nil
    }

    slhelp["stripquotes"] = LibHelp{in: "string", out: "string", action: "Remove outer quotes (double, single or backtick)"}
    stdlib["stripquotes"] = func(evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("stripquotes",args,1,"1","string"); !ok { return nil,err }
        s:=args[0].(string)
        if hasOuter(s,'"') { return stripOuter(s,'"'),nil }
        if hasOuter(s,'\'') { return stripOuter(s,'\''),nil }
        if hasOuter(s,'`') { return stripOuter(s,'`'),nil }
        return s,nil
    }

    slhelp["lower"] = LibHelp{in: "string", out: "string", action: "Convert to lower-case."}
    stdlib["lower"] = func(evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("lower",args,1,"1","string"); !ok { return nil,err }
        return str.ToLower(args[0].(string)), nil
    }

    slhelp["upper"] = LibHelp{in: "string", out: "string", action: "Convert to upper-case."}
    stdlib["upper"] = func(evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("upper",args,1,"1","string"); !ok { return nil,err }
        return str.ToUpper(args[0].(string)), nil
    }

    slhelp["line_add"] = LibHelp{in: "var,string", out: "string", action: "Append a line to array string [#i1]var[#i0]."}
    stdlib["line_add"] = func(evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("line_add",args,1,"2","string","string"); !ok { return nil,err }

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
    stdlib["line_add_before"] = func(evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("line_add_before",args,1,"3","string","string","string"); !ok { return nil,err }

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
    stdlib["line_add_after"] = func(evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("line_add_after",args,1,"3","string","string","string"); !ok { return nil,err }

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
    stdlib["line_delete"] = func(evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("line_delete",args,1,"2","string","string"); !ok { return nil,err }

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
    stdlib["line_replace"] = func(evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("line_replace",args,1,"3","string","string","string"); !ok { return nil,err }

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
    stdlib["pad"] = func(evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("pad",args,2,
            "3","string","int","int",
            "4","string","int","int","string"); !ok { return nil,err }

        j := args[1].(int)
        w := args[2].(int)

        if len(args) == 4 {
            return pad(args[0].(string), j, w, args[3].(string)), err
        }

        return pad(args[0].(string), j, w, " "), err
    }



    slhelp["field"] = LibHelp{in: "input_string,position[,optional_separator]", out: "string", action: "Retrieves columnar field [#i1]position[#i0] from [#i1]input_string[#i0]. String is empty on failure."}
    stdlib["field"] = func(evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("field",args,2,
            "2","string","int",
            "3","string","int","string"); !ok { return nil,err }

        // get sep
        sep := " "
        if len(args) == 3 {
            sep = args[2].(string)
        }

        lf:="\r\n"
        fstr:=str.TrimSuffix(args[0].(string),lf)

        // get position
        pos := args[1].(int)

        // find column <position>

        f := func(c rune) bool {
            return str.ContainsRune(sep, c)
        }

        var ta []string
        if sep==" " {
            ta = str.FieldsFunc(fstr, f)
        } else {
            ta = str.Split(fstr, sep)
        }

        if pos > 0 && pos <= len(ta) {
            return ta[pos-1], nil
        }

        return "", nil

    }

    slhelp["fields"] = LibHelp{in: "input_string[,optional_separator]", out: "int", action: "Splits up [#i1]input_string[#i0] in local array [#i1]F[#i0], with fields starting at index 1. Field count is stored in [#i1]NF[#i0]. Also squeezes repeat spaces when separator is a space char (default). Returns -1 on error, or field count."}
    stdlib["fields"] = func(evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("fields",args,2,
            "1","string",
            "2","string","string"); !ok { return -1,err }

        // check arguments
        sep := " "
        if len(args) == 2 {
            sep = args[1].(string)
        }

        lf:="\r\n"
        fstr:=str.TrimRight(args[0].(string),lf)

        f := func(c rune) bool {
            return str.ContainsRune(sep, c)
        }

        var ta []string
        if sep==" " {
            ta = append([]string{""},str.FieldsFunc(fstr, f)...)
        } else {
            ta = append([]string{""},str.Split(fstr, sep)...)
        }

        c:=len(ta)-1
        vlock.Lock()
        // bin:=bind_int(evalfs,"F")
        // (*ident)[bin]=Variable{IName:"F",IValue:ta,IKind:0,ITyped:false,declared:true}
        vset(nil,evalfs,ident,"F",ta)
        // bin=bind_int(evalfs,"NF")
        // (*ident)[bin]=Variable{IName:"NF",IValue:c,IKind:0,ITyped:false,declared:true}
        vset(nil,evalfs,ident,"NF",c)
        vlock.Unlock()

        return c, err
    }

    slhelp["split"] = LibHelp{in: "string[,fs]", out: "[]list", action: "Returns [#i1]string[#i0] as a list, breaking the string on [#i1]fs[#i0]."}
    stdlib["split"] = func(evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("split",args,2,
            "1","string",
            "2","string","string"); !ok { return []string{},err }

        strIn:=args[0].(string)

        fs:=" "
        if len(args)>1 {
            fs=args[1].(string)
        }

        // all okay...
        return str.Split(strIn, fs),nil
    }

    slhelp["join"] = LibHelp{in: "[]string_list[,fs]", out: "string", action: "Returns a string with all elements of [#i1]string_list[#i0] concatenated, separated by [#i1]fs[#i0]."}
    stdlib["join"] = func(evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("join",args,4,
            "1","[]string",
            "2","[]string","string",
            "1","[]interface {}",
            "2","[]interface {}","string"); !ok { return "",err }

        var ary []string

        switch args[0].(type) {
        case []string:
            ary=args[0].([]string)
        case []any:
            for _,v:=range args[0].([]any) {
                ary=append(ary,sf("%v",v))
            }
        }

        fs:=""
        if len(args)>1 {
            fs=args[1].(string)
        }
        // all okay...
        return str.Join(ary, fs), nil
    }

    slhelp["collapse"] = LibHelp{in: "string", out: "string", action: "Turns a newline separated string into a space separated string."}
    stdlib["collapse"] = func(evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("collapse",args,1,"1","string"); !ok { return "",err }

        return str.TrimSpace(tr(str.Replace(args[0].(string), "\n", " ",-1),SQUEEZE," ","")),nil
    }


    slhelp["count"] = LibHelp{in: "string_name", out: "integer", action: "Returns the number of lines in [#i1]string_name[#i0]."}
    stdlib["count"] = func(evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("count",args,2,
            "1","string",
            "1","[]string"); !ok { return "",err }

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
    stdlib["lines"] = func(evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("lines",args,2,
            "2","string","string",
            "2","[]string","string"); !ok { return "",err }

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

    slhelp["inset"] = LibHelp{in: "nl_string,distance", out: "nl_string", action: "Left pads each line of [#i1]nl_string[#i0] with [#i1]distance[#i0] cursor right commands."}
    stdlib["inset"] = func(evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("inset",args,1,"2","string","int"); !ok { return "",err }

        s:=args[0].(string)
        dist:=args[1].(int)

        var list []string

        if dist==0 { return s,nil }

        lsep:="\n"
        if runtime.GOOS=="windows" {
            lsep="\r\n"
        }

        s+=lsep

        if runtime.GOOS!="windows" {
            list = str.Split(s,lsep)
        } else {
            list = str.Split(str.Replace(s, lsep, "\n", -1), "\n")
        }

        llen:=len(list)

        var ns str.Builder
        ns.Grow(20)
        if llen>0 {
            for k:=0; k<llen-1; k++ {
                if len(list[k])>0 {
                    ns.WriteString(sf("\033[%dC%s%s",dist,list[k],lsep))
                } else {
                    ns.WriteString("\n")
                }
            }
        }

        // always remove trailing lsep 
        s=ns.String()
        if s[len(s)-1] == '\n' {
            s = s[:len(s)-len(lsep)]
        }

        return s,nil

    }

    slhelp["line_head"] = LibHelp{in: "nl_string,count", out: "nl_string", action: "Returns the top [#i1]count[#i0] lines of [#i1]nl_string[#i0]."}
    stdlib["line_head"] = func(evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("line_head",args,1,"2","string","int"); !ok { return "",err }

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
        if llen>0 {
            for k:=0; k<count; k++ {
                ns.WriteString(list[k]+lsep)
            }
        }

        // always remove trailing lsep 
        s=ns.String()
        if s[len(s)-1] == '\n' {
            s = s[:len(s)-len(lsep)]
        }

        return s,nil

    }

    slhelp["line_tail"] = LibHelp{in: "nl_string,count", out: "nl_string", action: "Returns the last [#i1]count[#i0] lines of [#i1]nl_string[#i0]."}
    stdlib["line_tail"] = func(evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("line_tail",args,1,"2","string","int"); !ok { return "",err }

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
        if llen>0 {
            for k:=start; k<llen; k++ {
                ns.WriteString(list[k]+lsep)
            }
        }

        // always remove trailing lsep 
        s=ns.String()
        if s[len(s)-1] == '\n' {
            s = s[:len(s)-len(lsep)]
        }

        return s,nil

    }

    slhelp["line_match"] = LibHelp{in: "nl_string,regex", out: "bool", action: "Does [#i1]nl_string[#i0] contain a match for regular expression [#i1]regex[#i0] on any line?"}
    stdlib["line_match"] = func(evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("line_match",args,1,"2","string","string"); !ok { return "",err }

        val:=args[0].(string)
        reg:=args[1].(string)

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
    stdlib["next_match"] = func(evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("next_match",args,1,"3","string","string","int"); !ok { return "",err }

        val:=args[0].(string)
        reg:=args[1].(string)
        startcount:=args[2].(int)

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

    slhelp["grep"] = LibHelp{in: "nl_string,regex", out: "nl_string", action: "Alias for line_filter."}
    stdlib["grep"] = func(evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        return stdlib["line_filter"](evalfs,ident,args...)
    }

    slhelp["line_filter"] = LibHelp{in: "nl_string,regex", out: "nl_string", action: "Returns lines from [#i1]nl_string[#i0] where regular expression [#i1]regex[#i0] matches."}
    stdlib["line_filter"] = func(evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("line_filter",args,1,"2","string","string"); !ok { return "",err }

        val:=args[0].(string)
        reg:=args[1].(string)

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
    stdlib["match"] = func(evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("match",args,1,"2","string","string"); !ok { return "",err }
        return regexp.MatchString(args[1].(string), args[0].(string))
    }

    slhelp["filter"] = LibHelp{in: "string,regex[,count]", out: "string", action: "Returns a string matching the regular expression [#i1]regex[#i0] in [#i1]string[#i0]. count should be -1 for all matches."}
    stdlib["filter"] = func(evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("filter",args,2,
            "2","string","string",
            "3","string","string","int"); !ok { return "",err }

        count:=0
        if len(args)>2 {
            count=args[2].(int)
        }

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
    }

    slhelp["substr"] = LibHelp{in: "string,int_s,int_l", out: "string", action: "Returns a sub-string of [#i1]string[#i0], from position [#i1]int_s[#i0] with length [#i1]int_l[#i0]."}
    stdlib["substr"] = func(evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("substr",args,1,"3","string","int","int"); !ok { return "",err }
        if args[1].(int)>=len(args[0].(string)) || args[2].(int)>len(args[0].(string)) {
            return "",errors.New("Bad argument (range) in substr()")
        }
        return args[0].(string)[args[1].(int) : args[1].(int)+args[2].(int)], err
    }

    slhelp["strpos"] = LibHelp{in: "string,substring[,start_pos]", out: "int_position", action: "Returns the position of the next match of [#i1]substring[#i0] in [#i1]string[#i0]. Returns -1 if no match found."}
    stdlib["strpos"] = func(evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("strpos",args,2,
            "2","string","string",
            "3","string","string","int"); !ok { return "",err }

        start:=0
        if len(args)==3 {
            start=args[2].(int)
        }

        p:=str.Index(args[0].(string)[start:],args[1].(string))
        if p!=-1 { p+=start }
        return p,nil
    }


    slhelp["gsub"] = LibHelp{in: "string,string_m,string_s", out: "string", action: "Returns [#i1]string[#i0] with all matches of [#i1]string_m[#i0] replaced with [#i1]string_s[#i0]."}
    stdlib["gsub"] = func(evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("gsub",args,1,"3","string","string","string"); !ok { return "",err }
        return str.Replace(args[0].(string), args[1].(string), args[2].(string), -1), err
    }

    slhelp["trim"] = LibHelp{in: "string,int_type[,removal_list_string]", out: "string", action: "Removes whitespace from [#i1]string[#i0], depending on [#i1]int_type[#i0]. -1 ltrim, 0 both, 1 rtrim. By default, space (ASCII:32) and horizontal tabs (ASCII:9) are removed."}
    stdlib["trim"] = func(evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("trim",args,2,
            "2","string","int",
            "3","string","int","string"); !ok { return "",err }

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


    slhelp["has_start"] = LibHelp{in: "string1,string2", out: "bool", action: "Does [#i1]string1[#i0] begin with [#i1]string2[#i0]?"}
    stdlib["has_start"] = func(evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("has_start",args,1,"2","string","string"); !ok { return "",err }
        return str.HasPrefix(args[0].(string), args[1].(string)), nil

    }

    slhelp["has_end"] = LibHelp{in: "string1,string2", out: "bool", action: "Does [#i1]string1[#i0] end with [#i1]string2[#i0]?"}
    stdlib["has_end"] = func(evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("has_end",args,1,"2","string","string"); !ok { return "",err }
        return str.HasSuffix(args[0].(string), args[1].(string)), err

    }

}

