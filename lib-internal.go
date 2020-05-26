//+build !test

package main

import (
    "encoding/binary"
    "errors"
    "io/ioutil"
    "os"
    "reflect"
    "regexp"
    "runtime"
    "sort"
    str "strings"
)

const (
    _AT_NULL             = 0
    _AT_CLKTCK           = 17
    _SYSTEM_CLK_TCK      = 100
    uintSize        uint = 32 << (^uint(0) >> 63)
)

func ulen(args interface{}) (int,error) {
    switch args.(type) { // i'm getting fed up of typing these case statements!!
    case string:
        return len(args.(string)),nil
    case []string:
        return len(args.([]string)),nil
    case []interface{}:
        return len(args.([]interface{})),nil
    case []int:
        return len(args.([]int)),nil
    case []int32:
        return len(args.([]int32)),nil
    case []int64:
        return len(args.([]int64)),nil
    case []uint8:
        return len(args.([]uint8)),nil
    case []float64:
        return len(args.([]float64)),nil
    case []bool:
        return len(args.([]bool)),nil
    case map[string]float64:
        return len(args.(map[string]float64)),nil
    case map[string]interface{}:
        return len(args.(map[string]interface{})),nil
    case map[string]string:
        return len(args.(map[string]string)),nil
    case map[string]int:
        return len(args.(map[string]int)),nil
    case map[string]bool:
        return len(args.(map[string]bool)),nil
    case map[string]int32:
        return len(args.(map[string]int32)),nil
    case map[string]int64:
        return len(args.(map[string]int64)),nil
    case map[string]uint8:
        return len(args.(map[string]uint8)),nil
    }
    return -1,errors.New(sf("Unknown type in globlen '%T'",args))
}


func buildInternalLib() {

    // language

    features["internal"] = Feature{version: 1, category: "debug"}
    categories["internal"] = []string{"last", "last_out", "zsh_version", "bash_version", "bash_versinfo", "user", "os", "home", "lang",
        "release_name", "release_version", "release_id", "winterm", "hostname", "argc","argv",
        "funcs", "dump", "key_press", "tokens", "key", "clear_line","pid","ppid", "system",
        "local", "clktck", "globkey", "getglob", "funcref", "thisfunc", "thisref", "commands","cursoron","cursoroff","cursorx",
        "eval", "term_w", "term_h", "pane_h", "pane_w","utf8supported","execpath","locks", "ansi", "interpol", "shellpid", "noshell",
        "globlen","len","length",
    }


    slhelp["utf8supported"] = LibHelp{in: "", out: "bool", action: "Is the current language utf-8 compliant."}
    stdlib["utf8supported"] = func(args ...interface{}) (ret interface{}, err error) {
        return str.HasSuffix(str.ToLower(os.Getenv("LANG")),".utf-8") , nil
    }

    slhelp["wininfo"] = LibHelp{in: "", out: "number", action: "(windows) Returns the console geometry."}
    stdlib["wininfo"] = func(args ...interface{}) (ret interface{}, err error) {
        hnd:=1
        if len(args)==1 {
            switch args[0].(type) {
            case int:
                hnd=args[0].(int)
            }
        }
        return GetWinInfo(hnd), nil
    }

    slhelp["term_h"] = LibHelp{in: "", out: "number", action: "Returns the current terminal height."}
    stdlib["term_h"] = func(args ...interface{}) (ret interface{}, err error) {
        return MH, nil
    }

    slhelp["term_w"] = LibHelp{in: "", out: "number", action: "Returns the current terminal width."}
    stdlib["term_w"] = func(args ...interface{}) (ret interface{}, err error) {
        return MW, nil
    }

    slhelp["pane_h"] = LibHelp{in: "", out: "number", action: "Returns the current pane height."}
    stdlib["pane_h"] = func(args ...interface{}) (ret interface{}, err error) {
        return panes[currentpane].h, nil
    }

    slhelp["pane_w"] = LibHelp{in: "", out: "number", action: "Returns the current pane width."}
    stdlib["pane_w"] = func(args ...interface{}) (ret interface{}, err error) {
        return panes[currentpane].w, nil
    }

    slhelp["system"] = LibHelp{in: "string,bool", out: "string", action: "Executes command [#i1]string[#i0] and returns (bool=false) or displays (bool=true) the output."}
    stdlib["system"] = func(args ...interface{}) (ret interface{}, err error) {

        cmd:=""
        display:=false

        if len(args)>0 {
            switch args[0].(type) {
            case string:
                cmd=args[0].(string)
            }
        }

        if len(args)>1 {
            switch args[1].(type) {
            case bool:
                display=args[1].(bool)
            }
        }

        return system(cmd,display),nil

    }

    slhelp["argv"] = LibHelp{in: "", out: "arg_list", action: "CLI arguments."}
    stdlib["argv"] = func(args ...interface{}) (ret interface{}, err error) {
        return cmdargs, nil
    }

    slhelp["argc"] = LibHelp{in: "", out: "number", action: "CLI argument count."}
    stdlib["argc"] = func(args ...interface{}) (ret interface{}, err error) {
        return len(cmdargs), nil
    }

    slhelp["eval"] = LibHelp{in: "string", out: "various", action: "evaluate expression in [#i1]string[#i0]."}
    stdlib["eval"] = func(args ...interface{}) (ret interface{}, err error) {

        if len(args) == 1 {
            lastlock.RLock()
            lfs:=lastfs
            lastlock.RUnlock()
            switch args[0].(type) {
            case string:
                ret, _, err = ev(lfs, args[0].(string), true,true)
                return ret, err
            }
        }
        return nil, nil
    }

    slhelp["ansi"] = LibHelp{in: "bool", out: "", action: "Enable (default) or disable ANSI colour support at runtime."}
    stdlib["ansi"] = func(args ...interface{}) (ret interface{}, err error) {
        if len(args)!=1 {
            return nil,errors.New("ansi() accepts a boolean value only.")
        }
        switch args[0].(type) {
        case bool:
            lastlock.Lock()
            ansiMode=args[0].(bool)
            lastlock.Unlock()
        default:
            return nil,errors.New("ansi() accepts a boolean value only.")
        }
        setupAnsiPalette()
        return nil, nil
    }

    slhelp["interpol"] = LibHelp{in: "bool", out: "", action: "Enable (default) or disable string interpolation at runtime."}
    stdlib["interpol"] = func(args ...interface{}) (ret interface{}, err error) {
        if len(args)!=1 {
            return nil,errors.New("interpol() accepts a boolean value only.")
        }
        switch args[0].(type) {
        case bool:
            lastlock.Lock()
            no_interpolation=!args[0].(bool)
            lastlock.Unlock()
        default:
            return nil,errors.New("interpol() accepts a boolean value only.")
        }
        return nil, nil
    }

    slhelp["locks"] = LibHelp{in: "bool", out: "", action: "Enable or disable locks at runtime."}
    stdlib["locks"] = func(args ...interface{}) (ret interface{}, err error) {
        if len(args)!=1 {
            return nil,errors.New("locks() accepts a boolean value only.")
        }
        switch args[0].(type) {
        case bool:
            globlock.Lock()
            lockSafety=args[0].(bool)
            globlock.Unlock()
        default:
            return nil,errors.New("locks() accepts a boolean value only.")
        }
        return nil, nil
    }

    slhelp["funcref"] = LibHelp{in: "name", out: "func_ref_num", action: "Find a function handle."}
    stdlib["funcref"] = func(args ...interface{}) (ret interface{}, err error) {
        if len(args)!=1 || sf("%T",args[0])!="string" { return nil,errors.New("Bad arguments provided to funcref()") }
        lmv,_:=fnlookup.lmget(args[0].(string))
        return lmv, nil
    }

    slhelp["thisfunc"] = LibHelp{in: "", out: "string", action: "Find this function's name."}
    stdlib["thisfunc"] = func(args ...interface{}) (ret interface{}, err error) {
        nv,_:=numlookup.lmget(lastfs)
        return nv, nil
    }

    slhelp["thisref"] = LibHelp{in: "", out: "func_ref_num", action: "Find this function's handle."}
    stdlib["thisref"] = func(args ...interface{}) (ret interface{}, err error) {
        return lastfs, nil
    }

    slhelp["local"] = LibHelp{in: "string", out: "value", action: "Return this local variable's value."}
    stdlib["local"] = func(args ...interface{}) (ret interface{}, err error) {
        var name string
        if len(args) == 1 {
            switch args[0].(type) {
            case string:
                name = args[0].(string)
                v, _ := vget(lastfs, name)
                return v, nil
            }
        }
        return nil, errors.New(sf("'%v' does not exist!", name))
    }


    slhelp["len"] = LibHelp{in: "string", out: "integer", action: "Returns length of string or list."}
    stdlib["len"] = func(args ...interface{}) (ret interface{}, err error) {
        if len(args) == 1 {
            return ulen(args[0])
        }
        return -1,errors.New("Bad argument in len()")
    }

    slhelp["length"] = LibHelp{in: "string", out: "integer", action: "Returns length of string or list."}
    stdlib["length"] = func(args ...interface{}) (ret interface{}, err error) {
        if len(args) == 1 {
            return ulen(args[0])
        }
        return -1,errors.New("Bad argument in length()")
    }

    slhelp["globlen"] = LibHelp{in: "name", out: "int", action: "Get the length of a global variable. Returns -1 on not found or error."}
    stdlib["globlen"] = func(args ...interface{}) (ret interface{}, err error) {
        if len(args) == 1 {
            switch args[0].(type) {
            case string:

                lastlock.RLock()
                lfs:=lastfs
                lastlock.RUnlock()

                inp,_ :=interpolate(lfs,args[0].(string),true)

                globlock.RLock()
                res,_,err:=ev(globalaccess,inp,true,true)
                globlock.RUnlock()

                if err==nil {
                    return ulen(res)
                }

            default:
                return -1,errors.New(sf("Global variable must be expressed as a string."))
            }
        }
        return -1, errors.New("Bad args to globlen()")
    }


    slhelp["getglob"] = LibHelp{in: "name", out: "var", action: "Read a global variable."}
    stdlib["getglob"] = func(args ...interface{}) (ret interface{}, err error) {
        if len(args) == 1 {
            switch args[0].(type) {
            case string:

                lastlock.RLock()
                lfs:=lastfs
                lastlock.RUnlock()
                inp,_ :=interpolate(lfs,args[0].(string),true)

                globlock.RLock()

                res,_,err:=ev(globalaccess,inp,true,true)

                globlock.RUnlock()

                if err==nil {
                    return res,nil
                } else {
                    return nil,errors.New(sf("Bad evaluation of '%s'",args[0].(string)))
                }
            default:
                return nil,nil
            }
        }
        return nil, errors.New("Bad args to getglob()")
    }

    slhelp["key"] = LibHelp{in: "ary_name,key_name", out: "bool", action: "Does key [#i1]key_name[#i0] exist in associative array [#i1]ary_name[#i0]?"}
    stdlib["key"] = func(args ...interface{}) (ret interface{}, err error) {
        if len(args) != 2 {
            return false, errors.New("bad argument count in key()")
        }
        if reflect.TypeOf(args[0]).Name() != "string" || reflect.TypeOf(args[1]).Name() != "string" {
            return false, errors.New("arguments to key() must be strings.")
        }

        var v interface{}
        var found bool

        if v, found = vget(lastfs, args[0].(string)); !found {
            return false, nil
        }
        if _, found = v.(map[string]interface{})[args[1].(string)].(interface{}); found {
            return true, nil
        }
        return false, nil
    }

    slhelp["globkey"] = LibHelp{in: "ary_name,key_name", out: "bool", action: "Does key [#i1]key_name[#i0] exist in the global associative array [#i1]ary_name[#i0]?"}
    stdlib["globkey"] = func(args ...interface{}) (ret interface{}, err error) {
        if len(args) != 2 {
            return false, errors.New("bad argument count in globkey()")
        }
        if reflect.TypeOf(args[0]).Name() != "string" || reflect.TypeOf(args[1]).Name() != "string" {
            return false, errors.New("arguments to globkey() must be strings.")
        }
        var v interface{}
        var found bool
        globlock.RLock()
        if v, found = vget(globalaccess, args[0].(string)); !found {
            globlock.RUnlock()
            return false, nil
        }
        globlock.RUnlock()

        if lockSafety { lastlock.RLock() }
        key,_:=interpolate(lastfs,args[1].(string),true)
        if lockSafety { lastlock.RUnlock() }

        switch v.(type) {
        case map[string]interface{}:
            if _, found = v.(map[string]interface{})[key];   found { return true, nil }
        case map[string]float64:
            if _, found = v.(map[string]float64)[key];       found { return true, nil }
        case map[string]int:
            if _, found = v.(map[string]int) [key];          found { return true, nil }
        case map[string]bool:
            if _, found = v.(map[string]bool)[key];          found { return true, nil }
        case map[string]string:
            if _, found = v.(map[string]string)[key];        found { return true, nil }
        default:
            pf("unknown type: %T\n",v); os.Exit(0)
        }
        return false, nil
    }

    slhelp["last"] = LibHelp{in: "", out: "int", action: "Returns the last received error code from a co-process command."}
    stdlib["last"] = func(args ...interface{}) (ret interface{}, err error) {
        v, found := vget(0, "@last")
        if found {
            i, bool_err := GetAsInt(v.(string))
            if !bool_err {
                return i, nil
            }
            return i, errors.New("could not convert last status to integer.")
        }
        return -1,errors.New("no co-process command has been executed yet.")
    }

    slhelp["execpath"] = LibHelp{in: "", out: "string", action: "Returns the current working directory."}
    stdlib["execpath"] = func(args ...interface{}) (ret interface{}, err error) {
        v, _ := vget(0, "@execpath")
        return string(v.(string)), err
    }

    slhelp["last_out"] = LibHelp{in: "", out: "string", action: "Returns the last received error text from the co-process."}
    stdlib["last_out"] = func(args ...interface{}) (ret interface{}, err error) {
        v, found := vget(0, "@last_out")
        if found {
            return string(v.([]byte)), err
        }
        return "",errors.New("No co-process error has been detected yet.")
    }

    slhelp["zsh_version"] = LibHelp{in: "", out: "string", action: "Returns the zsh version string if present."}
    stdlib["zsh_version"] = func(args ...interface{}) (ret interface{}, err error) {
        v, _ := vget(0, "@zsh_version")
        return v.(string), err
    }

    slhelp["bash_version"] = LibHelp{in: "", out: "string", action: "Returns the full release string of the Bash co-process."}
    stdlib["bash_version"] = func(args ...interface{}) (ret interface{}, err error) {
        v, _ := vget(0, "@bash_version")
        return v.(string), err
    }

    slhelp["bash_versinfo"] = LibHelp{in: "", out: "string", action: "Returns the major version number of the Bash co-process."}
    stdlib["bash_versinfo"] = func(args ...interface{}) (ret interface{}, err error) {
        v, _ := vget(0, "@bash_versinfo")
        return v.(string), err
    }

    slhelp["key_press"] = LibHelp{in: "", out: "int", action: "Returns an integer corresponding with a keypress."}
    stdlib["key_press"] = func(args ...interface{}) (ret interface{}, err error) {
        timeo := int64(0)
        if len(args) == 1 {

            if runtime.GOOS=="windows" {
                return "",errors.New("Timeout value cannot be specified in Windows.")
            }

            switch args[0].(type) {
            case string, int:
                ttmp, terr := GetAsInt(args[0])
                timeo = int64(ttmp)
                if terr {
                    return "", errors.New("Invalid timeout value.")
                }
            }
        }
        return wrappedGetCh(int(timeo)), nil
    }

    slhelp["cursoroff"] = LibHelp{in: "", out: "", action: "Disables cursor display."}
    stdlib["cursoroff"] = func(args ...interface{}) (ret interface{}, err error) {
        hideCursor()
        return nil, nil
    }

    slhelp["cursorx"] = LibHelp{in: "n", out: "", action: "Moves cursor to horizontal position [#i1]n[#i0]."}
    stdlib["cursorx"] = func(args ...interface{}) (ret interface{}, err error) {
        if len(args)==1 {
            switch args[0].(type) {
            case int:
                cursorX(args[0].(int))
            }
        }
        return nil, nil
    }

    slhelp["cursoron"] = LibHelp{in: "", out: "", action: "Enables cursor display."}
    stdlib["cursoron"] = func(args ...interface{}) (ret interface{}, err error) {
        showCursor()
        return nil, nil
    }

    slhelp["ppid"] = LibHelp{in: "", out: "", action: "Return the pid of parent process."}
    stdlib["ppid"] = func(args ...interface{}) (ret interface{}, err error) {
        return os.Getppid(), nil
    }

    slhelp["pid"] = LibHelp{in: "", out: "", action: "Return the pid of the current process."}
    stdlib["pid"] = func(args ...interface{}) (ret interface{}, err error) {
        return os.Getpid(), nil
    }

    slhelp["clear_line"] = LibHelp{in: "row,col", out: "", action: "Clear to the end of the line, starting at row,col in the current pane."}
    stdlib["clear_line"] = func(args ...interface{}) (ret interface{}, err error) {
        if len(args)!=2 { return nil,errors.New("Bad arguments provided to clear_line()") }
        row, rerr := GetAsInt(args[0])
        col, cerr := GetAsInt(args[1])
        if !(cerr || rerr) {
            clearToEOPane(row, col)
        }
        return nil, nil
    }

    slhelp["user"] = LibHelp{in: "", out: "string", action: "Returns the parent user of the Bash co-process."}
    stdlib["user"] = func(args ...interface{}) (ret interface{}, err error) {
        v, _ := vget(0, "@user")
        return v.(string), err
    }

    slhelp["os"] = LibHelp{in: "", out: "string", action: "Returns the kernel version name."}
    stdlib["os"] = func(args ...interface{}) (ret interface{}, err error) {
        v, _ := vget(0, "@os")
        return v.(string), err
    }

    slhelp["home"] = LibHelp{in: "", out: "string", action: "Returns the home directory of the user that launched Za."}
    stdlib["home"] = func(args ...interface{}) (ret interface{}, err error) {
        v, _ := vget(0, "@home")
        return v.(string), err
    }

    slhelp["lang"] = LibHelp{in: "", out: "string", action: "Returns the locale name for the active Za session."}
    stdlib["lang"] = func(args ...interface{}) (ret interface{}, err error) {
        v, _ := vget(0, "@lang")
        return v.(string), err
    }

    slhelp["release_name"] = LibHelp{in: "", out: "string", action: "Returns the OS release name."}
    stdlib["release_name"] = func(args ...interface{}) (ret interface{}, err error) {
        v, _ := vget(0, "@release_name")
        return v.(string), err
    }

    slhelp["hostname"] = LibHelp{in: "", out: "string", action: "Returns the current hostname."}
    stdlib["hostname"] = func(args ...interface{}) (ret interface{}, err error) {
        z, _ := Copper("hostname", true)
        vset(0, "@hostname", z)
        return z, err
    }

    slhelp["tokens"] = LibHelp{in: "string", out: "", action: "Returns a list of tokens in a string."}
    stdlib["tokens"] = func(args ...interface{}) (ret interface{}, err error) {
        if len(args)==0 { return []string{},errors.New("No argument provided to tokens()") }
        if sf("%T",args[0])!="string" {
            return []string{},errors.New("Invalid argument provided to tokens()")
        }
        tt := Error
        var toks []string
        var toktypes []string
        cl := 1
        for p := 0; p < len(args[0].(string)); p++ {
            t, eol, eof := nextToken(args[0].(string), &cl, p, tt)
            tt = t.tokType
            if t.tokPos != -1 {
                p = t.tokPos
            }
            toks = append(toks, t.tokText)
            toktypes = append(toktypes, tokNames[tt])
            if eof || eol {
                break
            }
        }
        return append(toktypes,toks...), err
    }

    slhelp["release_version"] = LibHelp{in: "", out: "string", action: "Returns the OS version number."}
    stdlib["release_version"] = func(args ...interface{}) (ret interface{}, err error) {
        v, _ := vget(0, "@release_version")
        return v.(string), err
    }

    slhelp["release_id"] = LibHelp{in: "", out: "string", action: "Returns the /etc derived release name."}
    stdlib["release_id"] = func(args ...interface{}) (ret interface{}, err error) {
        v, _ := vget(0, "@release_id")
        return v.(string), err
    }

    slhelp["winterm"] = LibHelp{in: "", out: "bool", action: "Is this a WSL terminal?"}
    stdlib["winterm"] = func(args ...interface{}) (ret interface{}, err error) {
        v, _ := vget(0, "@winterm")
        return v.(bool), err
    }

    slhelp["commands"] = LibHelp{in: "", out: "", action: "Displays a list of keywords."}
    stdlib["commands"] = func(args ...interface{}) (ret interface{}, err error) {
        commands()
        return nil, nil
    }

    slhelp["funcs"] = LibHelp{in: "partial_match (optional)", out: "string", action: "Returns a list of standard library functions."}
    stdlib["funcs"] = func(args ...interface{}) (ret interface{}, err error) {

        if len(args) == 0 {
            args = append(args, "")
        }
        if len(args) > 2 {
            return false, nil
        }

        retstring:=false
        if len(args)==2 {
            switch args[1].(type) {
            case bool:
                retstring=args[1].(bool)
            default:
                return "",errors.New("Argument 2 in funcs() must be a boolean if present.")
            }
        }

        regex := ""
        funclist := ""
        if args[0].(string) != "" {
            regex = args[0].(string)
        }

        // sort the keys
        var keys []string
        for k :=range categories { keys=append(keys,k) }
        sort.Strings(keys)

        for _,k := range keys {
        c := k
        v := categories[k]
        // for c, v := range categories {
            matchList := ""
            foundOne := false
            for _, q := range v {
                if matched, _ := regexp.MatchString(regex, q); matched {
                    if _, ok := slhelp[q]; ok {
                        lhs := slhelp[q].out
                        colour := "2"
                        if slhelp[q].out != "" {
                            lhs += " = "
                            colour = "3"
                        }
                        params := slhelp[q].in
                        matchList += sf(sparkle("\n  [#6]Function : [#"+colour+"]%s%s(%s)[#-]\n"), lhs, q, params)
                        matchList += sf(sparkle("           [#6]:[#-] %s\n"), sparkle(slhelp[q].action))
                    }
                    foundOne = true
                }
            }
            if foundOne {
                funclist += sf("Category: %s\n%s\n", c, matchList)
            }
        }
        if !retstring {
            pf(funclist)
            return nil, nil
        }
        return funclist,nil
    }

    slhelp["dump"] = LibHelp{in: "function_name", out: "none", action: "Displays variable list, or a specific entry."}
    stdlib["dump"] = func(args ...interface{}) (ret interface{}, err error) {
        s := ""
        if len(args) == 0 { s="global" }
        if len(args) == 1 {
            switch args[0].(type) {
            case string:
                s = args[0].(string)
            default:
                return false, err
            }
        }
        if s != "" {
            lmv,found:=fnlookup.lmget(s)
            if found {
            vc:=varcount[lmv]
            for q := 0; q < vc; q++ {
                v := ident[lmv][q]
                if v.iName[0]=='@' { continue }
                    pf("%s = %v\n", v.iName, v.iValue)
                }
            } else {
                pf("Invalid space name provided '%v'.\n",s)
            }
        }
        return true, err
    }

    slhelp["has_shell"] = LibHelp{in: "", out: "bool", action: "Check if there is a child co-process has been launched."}
    stdlib["has_shell"] = func(args ...interface{}) (ret interface{}, err error) {
        v, _ := vget(0,"@noshell")
        return !v.(bool), nil
    }

    slhelp["shellpid"] = LibHelp{in: "", out: "int", action: "Get process ID of the launched child co-process."}
    stdlib["shellpid"] = func(args ...interface{}) (ret interface{}, err error) {
        v, _ := vget(0,"@shellpid")
        return v, nil
    }

    slhelp["clktck"] = LibHelp{in: "", out: "number", action: "Get clock ticks from aux file."}
    stdlib["clktck"] = func(args ...interface{}) (ret interface{}, err error) {
        return getclktck(), nil
    }

}

func getclktck() int {

    if runtime.GOOS=="windows" {
        return -1
    }

    buf, err := ioutil.ReadFile("/proc/self/auxv")
    if err == nil {
        pb := int(uintSize / 8)
        for i := 0; i < len(buf)-pb*2; i += pb * 2 {
            var tag, val uint
            switch uintSize {
            case 32:
                tag = uint(binary.LittleEndian.Uint32(buf[i:]))
                val = uint(binary.LittleEndian.Uint32(buf[i+pb:]))
            case 64:
                tag = uint(binary.LittleEndian.Uint64(buf[i:]))
                val = uint(binary.LittleEndian.Uint64(buf[i+pb:]))
            }

            switch tag {
            case _AT_CLKTCK:
                if val != 0 {
                    return int(val)
                }
            }
        }
    }
    return int(_SYSTEM_CLK_TCK)
}
