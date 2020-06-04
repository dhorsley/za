//+build !test

package main


import (
    "encoding/binary"
    "errors"
    "io/ioutil"
    "os"
	// "golang.org/x/sys/unix"
    "unicode/utf8"
    "reflect"
    "regexp"
    "runtime"
    "sort"
    str "strings"
    "time"
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
        return utf8.RuneCountInString(args.(string)),nil
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
    return -1,errors.New(sf("Unknown type '%T'",args))
}


func buildInternalLib() {

    // language

    features["internal"] = Feature{version: 1, category: "debug"}
    categories["internal"] = []string{"last", "last_out", "zsh_version", "bash_version", "bash_versinfo", "user", "os", "home", "lang",
        "release_name", "release_version", "release_id", "winterm", "hostname", "argc","argv",
        "funcs", "dump", "keypress", "tokens", "key", "clear_line","pid","ppid", "system",
        "local", "clktck", "globkey", "getglob", "funcref", "thisfunc", "thisref", "commands","cursoron","cursoroff","cursorx",
        "eval", "term_w", "term_h", "pane_h", "pane_w","utf8supported","execpath","locks", "ansi", "interpol", "shellpid", "has_shell",
        "globlen","len","length","tco", "echo","getrow","getcol","unmap","await",
    }


    slhelp["utf8supported"] = LibHelp{in: "", out: "bool", action: "Is the current language utf-8 compliant."}
    stdlib["utf8supported"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
        return str.HasSuffix(str.ToLower(os.Getenv("LANG")),".utf-8") , nil
    }

    slhelp["wininfo"] = LibHelp{in: "", out: "number", action: "(windows) Returns the console geometry."}
    stdlib["wininfo"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
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
    stdlib["term_h"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
        return MH, nil
    }

    slhelp["term_w"] = LibHelp{in: "", out: "number", action: "Returns the current terminal width."}
    stdlib["term_w"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
        return MW, nil
    }

    slhelp["pane_h"] = LibHelp{in: "", out: "number", action: "Returns the current pane height."}
    stdlib["pane_h"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
        return panes[currentpane].h, nil
    }

    slhelp["pane_w"] = LibHelp{in: "", out: "number", action: "Returns the current pane width."}
    stdlib["pane_w"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
        return panes[currentpane].w, nil
    }

    slhelp["system"] = LibHelp{in: "string,bool", out: "string", action: "Executes command [#i1]string[#i0] and returns (bool=false) or displays (bool=true) the output."}
    stdlib["system"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {

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
    stdlib["argv"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
        return cmdargs, nil
    }

    slhelp["argc"] = LibHelp{in: "", out: "number", action: "CLI argument count."}
    stdlib["argc"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
        return len(cmdargs), nil
    }

    slhelp["eval"] = LibHelp{in: "string", out: "various", action: "evaluate expression in [#i1]string[#i0]."}
    stdlib["eval"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
        if len(args) == 1 {
            switch args[0].(type) {
            case string:
                ret, _, err = ev(evalfs, args[0].(string), true,true)
                return ret, err
            }
        }
        return nil, nil
    }

    slhelp["getrow"] = LibHelp{in: "", out: "number", action: "row position of console text cursor."}
    stdlib["getrow"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
        r,_:=GetCursorPos()
        if runtime.GOOS=="windows" { r++ }
        return r, nil
    }

    slhelp["getcol"] = LibHelp{in: "", out: "number", action: "column position of console text cursor."}
    stdlib["getcol"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
        _,c:=GetCursorPos()
        if runtime.GOOS=="windows" { c++ }
        return c, nil
    }

    slhelp["echo"] = LibHelp{in: "bool[,mask]", out: "bool", action: "Enable (default) or disable local echo. Optionally, set the mask character to be used during input. Current visibility state is returned."}
    stdlib["echo"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
        if len(args)>2 {
            return nil,errors.New("incorrect argument count for echo().")
        }

        se:=true
        if len(args)>0 {
            switch args[0].(type) {
            case bool:
                if args[0].(bool) {
                    se=true
                    vset(0, "@echo", true)
                } else {
                    se=false
                    vset(0, "@echo", false)
                }
            default:
                return nil,errors.New("echo() accepts a boolean value only.")
            }
        }

        mask,_:=vget(0,"@echomask")
        if len(args)>1 {
            switch args[1].(type) {
            case string:
                mask=args[1].(string)
            default:
                return nil,errors.New("echo() accepts a string value for mask.")
            }
        }

        setEcho(se)
        vset(0,"@echomask", mask)
        return se,nil
    }

    slhelp["ansi"] = LibHelp{in: "bool", out: "", action: "Enable (default) or disable ANSI colour support at runtime."}
    stdlib["ansi"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
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
    stdlib["interpol"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
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
    stdlib["locks"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
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
    stdlib["funcref"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
        if len(args)!=1 || sf("%T",args[0])!="string" { return nil,errors.New("Bad arguments provided to funcref()") }
        lmv,_:=fnlookup.lmget(args[0].(string))
        return lmv, nil
    }

    slhelp["thisfunc"] = LibHelp{in: "", out: "string", action: "Find this function's name."}
    stdlib["thisfunc"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
        nv,_:=numlookup.lmget(evalfs)
        return nv, nil
    }

    slhelp["thisref"] = LibHelp{in: "", out: "func_ref_num", action: "Find this function's handle."}
    stdlib["thisref"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
        return evalfs, nil
    }

    slhelp["tco"] = LibHelp{in: "", out: "bool", action: "are we currently in a tail call loop?"}
    stdlib["tco"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
        b,_:=vget(evalfs,"@in_tco")
        return b.(bool), nil
    }

    slhelp["local"] = LibHelp{in: "string", out: "value", action: "Return this local variable's value."}
    stdlib["local"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
        var name string
        if len(args) == 1 {
            switch args[0].(type) {
            case string:
                name = args[0].(string)
                v, _ := vget(evalfs, name)
                return v, nil
            }
        }
        return nil, errors.New(sf("'%v' does not exist!", name))
    }


    slhelp["len"] = LibHelp{in: "string", out: "integer", action: "Returns length of string or list."}
    stdlib["len"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
        if len(args) == 1 {
            return ulen(args[0])
        }
        return -1,errors.New("Bad argument in len()")
    }

    slhelp["length"] = LibHelp{in: "string", out: "integer", action: "Returns length of string or list."}
    stdlib["length"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
        if len(args) == 1 {
            return ulen(args[0])
        }
        return -1,errors.New("Bad argument in length()")
    }

    slhelp["globlen"] = LibHelp{in: "name", out: "int", action: "Get the length of a global variable. Returns -1 on not found or error."}
    stdlib["globlen"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
        if len(args) == 1 {
            switch args[0].(type) {
            case string:

                inp,_ :=interpolate(evalfs,args[0].(string),true)

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
    stdlib["getglob"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
        if len(args) == 1 {
            switch args[0].(type) {
            case string:

                inp,_ :=interpolate(evalfs,args[0].(string),true)

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


    slhelp["await"] = LibHelp{in: "handle_map,all_flag", out: "[]result", action: "Checks for async completion."}
    stdlib["await"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {

        if len(args)==0 || len(args)>2 {
            return nil,errors.New("bad argument count in await()")
        }

        var handleMap map[string]interface{}

        if len(args)>0 {
            if sf("%T",args[0])!="map[string]interface {}" {
                return nil,errors.New("argument 1 must be a map")
            }
            handleMap=args[0].(map[string]interface{})
        }

        waitForAll:=false
        if len(args)>1 {
            if sf("%T",args[1])!="bool" {
                return nil,errors.New("argument 2 must be a boolean if present")
            }
            waitForAll=args[1].(bool)
        }

        var results=make(map[string]interface{})

        keepWaiting:=true
        for ; keepWaiting ; {
            for k,v:=range handleMap {
                select {
                case retval := <-v.(<-chan interface{}):
                    results[k]=retval
                    delete(handleMap,k)
                default:
                }
            }
            keepWaiting=false
            if waitForAll {
                if len(handleMap)!=0 {
                    keepWaiting=true
                    time.Sleep(1*time.Microsecond)
                }
            }
        }

        return results,nil
    }


    slhelp["unmap"] = LibHelp{in: "ary_name,key_name", out: "bool", action: "Remove a map key"}
    stdlib["unmap"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
        if len(args) != 2 {
            return false, errors.New("bad argument count in unmap()")
        }
        if reflect.TypeOf(args[0]).Name() != "string" || reflect.TypeOf(args[1]).Name() != "string" {
            return false, errors.New("arguments to unmap() must be strings.")
        }

        var v interface{}
        var found bool

        if v, found = vget(evalfs, args[0].(string)); !found {
            return false, nil
        }
        if _, found = v.(map[string]interface{})[args[1].(string)].(interface{}); found {
            vdelete(evalfs,args[0].(string),args[1].(string))
            return true,nil
        }
        return false, nil
    }

    slhelp["key"] = LibHelp{in: "ary_name,key_name", out: "bool", action: "Does key [#i1]key_name[#i0] exist in associative array [#i1]ary_name[#i0]?"}
    stdlib["key"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {

        if len(args) != 2 {
            return false, errors.New("bad argument count in key()")
        }

        if sf("%T",args[0])!="string" || sf("%T",args[1])!="string" {
            return false, errors.New("arguments to key() must be strings.")
        }

        var v interface{}
        var found bool

        if v, found = vget(evalfs, args[0].(string)); !found {
            return false, nil
        }

        key,_:=interpolate(evalfs,args[1].(string),true)

        switch v.(type) {
        case map[string]interface{}:
            if _, found = v.(map[string]interface{})[key];   found { return true, nil }
        case map[string]float64:
            if _, found = v.(map[string]float64)[key];       found { return true, nil }
        case map[string]uint8:
            if _, found = v.(map[string]uint8) [key];        found { return true, nil }
        case map[string]int64:
            if _, found = v.(map[string]int64) [key];        found { return true, nil }
        case map[string]int32:
            if _, found = v.(map[string]int32) [key];        found { return true, nil }
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

    slhelp["globkey"] = LibHelp{in: "ary_name,key_name", out: "bool", action: "Does key [#i1]key_name[#i0] exist in the global associative array [#i1]ary_name[#i0]?"}
    stdlib["globkey"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {

        if len(args) != 2 {
            return false, errors.New("bad argument count in globkey()")
        }

        if sf("%T",args[0])!="string" || sf("%T",args[1])!="string" {
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

        key,_:=interpolate(evalfs,args[1].(string),true)

        switch v.(type) {
        case map[string]interface{}:
            if _, found = v.(map[string]interface{})[key];   found { return true, nil }
        case map[string]float64:
            if _, found = v.(map[string]float64)[key];       found { return true, nil }
        case map[string]uint8:
            if _, found = v.(map[string]uint8) [key];        found { return true, nil }
        case map[string]int64:
            if _, found = v.(map[string]int64) [key];        found { return true, nil }
        case map[string]int32:
            if _, found = v.(map[string]int32) [key];        found { return true, nil }
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
    stdlib["last"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
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
    stdlib["execpath"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
        v, _ := vget(0, "@execpath")
        return string(v.(string)), err
    }

    slhelp["last_out"] = LibHelp{in: "", out: "string", action: "Returns the last received error text from the co-process."}
    stdlib["last_out"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
        v, found := vget(0, "@last_out")
        if found {
            return string(v.([]byte)), err
        }
        return "",errors.New("No co-process error has been detected yet.")
    }

    slhelp["zsh_version"] = LibHelp{in: "", out: "string", action: "Returns the zsh version string if present."}
    stdlib["zsh_version"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
        v, _ := vget(0, "@zsh_version")
        return v.(string), err
    }

    slhelp["bash_version"] = LibHelp{in: "", out: "string", action: "Returns the full release string of the Bash co-process."}
    stdlib["bash_version"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
        v, _ := vget(0, "@bash_version")
        return v.(string), err
    }

    slhelp["bash_versinfo"] = LibHelp{in: "", out: "string", action: "Returns the major version number of the Bash co-process."}
    stdlib["bash_versinfo"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
        v, _ := vget(0, "@bash_versinfo")
        return v.(string), err
    }

/*
    slhelp["keypeek"] = LibHelp{in: "", out: "int", action: "Returns an integer corresponding with a currently held down keypress."}
    stdlib["keypeek"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
        return wrappedGetCh(-1), nil
    }
*/

    slhelp["keypress"] = LibHelp{in: "timeout", out: "int", action: "Returns an integer corresponding with a keypress."}
    stdlib["keypress"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {

        timeo := int64(0)
        if len(args) == 1 {

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
    stdlib["cursoroff"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
        hideCursor()
        return nil, nil
    }

    slhelp["cursorx"] = LibHelp{in: "n", out: "", action: "Moves cursor to horizontal position [#i1]n[#i0]."}
    stdlib["cursorx"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
        if len(args)==1 {
            switch args[0].(type) {
            case int:
                cursorX(args[0].(int))
            }
        }
        return nil, nil
    }

    slhelp["cursoron"] = LibHelp{in: "", out: "", action: "Enables cursor display."}
    stdlib["cursoron"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
        showCursor()
        return nil, nil
    }

    slhelp["ppid"] = LibHelp{in: "", out: "", action: "Return the pid of parent process."}
    stdlib["ppid"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
        return os.Getppid(), nil
    }

    slhelp["pid"] = LibHelp{in: "", out: "", action: "Return the pid of the current process."}
    stdlib["pid"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
        return os.Getpid(), nil
    }

    slhelp["clear_line"] = LibHelp{in: "row,col", out: "", action: "Clear to the end of the line, starting at row,col in the current pane."}
    stdlib["clear_line"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
        if len(args)!=2 { return nil,errors.New("Bad arguments provided to clear_line()") }
        row, rerr := GetAsInt(args[0])
        col, cerr := GetAsInt(args[1])
        if !(cerr || rerr) {
            clearToEOPane(row, col)
        }
        return nil, nil
    }

    slhelp["user"] = LibHelp{in: "", out: "string", action: "Returns the parent user of the Bash co-process."}
    stdlib["user"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
        v, _ := vget(0, "@user")
        return v.(string), err
    }

    slhelp["os"] = LibHelp{in: "", out: "string", action: "Returns the kernel version name."}
    stdlib["os"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
        v, _ := vget(0, "@os")
        return v.(string), err
    }

    slhelp["home"] = LibHelp{in: "", out: "string", action: "Returns the home directory of the user that launched Za."}
    stdlib["home"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
        v, _ := vget(0, "@home")
        return v.(string), err
    }

    slhelp["lang"] = LibHelp{in: "", out: "string", action: "Returns the locale name for the active Za session."}
    stdlib["lang"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
        v, _ := vget(0, "@lang")
        return v.(string), err
    }

    slhelp["release_name"] = LibHelp{in: "", out: "string", action: "Returns the OS release name."}
    stdlib["release_name"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
        v, _ := vget(0, "@release_name")
        return v.(string), err
    }

    slhelp["hostname"] = LibHelp{in: "", out: "string", action: "Returns the current hostname."}
    stdlib["hostname"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
        z, _ := Copper("hostname", true)
        vset(0, "@hostname", z)
        return z, err
    }

    slhelp["tokens"] = LibHelp{in: "string", out: "", action: "Returns a list of tokens in a string."}
    stdlib["tokens"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
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
    stdlib["release_version"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
        v, _ := vget(0, "@release_version")
        return v.(string), err
    }

    slhelp["release_id"] = LibHelp{in: "", out: "string", action: "Returns the /etc derived release name."}
    stdlib["release_id"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
        v, _ := vget(0, "@release_id")
        return v.(string), err
    }

    slhelp["winterm"] = LibHelp{in: "", out: "bool", action: "Is this a WSL terminal?"}
    stdlib["winterm"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
        v, _ := vget(0, "@winterm")
        return v.(bool), err
    }

    slhelp["commands"] = LibHelp{in: "", out: "", action: "Displays a list of keywords."}
    stdlib["commands"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
        commands()
        return nil, nil
    }

    slhelp["funcs"] = LibHelp{in: "[partial_match[,bool_return]]", out: "string", action: "Returns a list of standard library functions."}
    stdlib["funcs"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {

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
                show:=false

                if matched, _ := regexp.MatchString(regex, q); matched { show=true }
                if matched, _ := regexp.MatchString(regex, k); matched { show=true }

                if show {
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
    stdlib["dump"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
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
    stdlib["has_shell"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
        v, _ := vget(0,"@noshell")
        return !v.(bool), nil
    }

    slhelp["shellpid"] = LibHelp{in: "", out: "int", action: "Get process ID of the launched child co-process."}
    stdlib["shellpid"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
        v, _ := vget(0,"@shellpid")
        return v, nil
    }

    slhelp["clktck"] = LibHelp{in: "", out: "number", action: "Get clock ticks from aux file."}
    stdlib["clktck"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
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
