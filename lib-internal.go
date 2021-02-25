// +build !test

package main


import (
    "encoding/binary"
    "errors"
    "io/ioutil"
    "os"
    "unicode/utf8"
    "net/http" // for key()
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
    switch args:=args.(type) { // i'm getting fed up of typing these case statements!!
    case nil:
        return 0,nil
    case string:
        return utf8.RuneCountInString(args),nil
    case []string:
        return len(args),nil
    case []int:
        return len(args),nil
    case []int64:
        return len(args),nil
    case []uint8:
        return len(args),nil
    case []float64:
        return len(args),nil
    case []bool:
        return len(args),nil
    case map[string]float64:
        return len(args),nil
    case map[string]string:
        return len(args),nil
    case map[string]int:
        return len(args),nil
    case map[string]bool:
        return len(args),nil
    case map[string]int64:
        return len(args),nil
    case map[string]uint8:
        return len(args),nil
    case []map[string]interface{}:
        return len(args),nil
    case map[string]interface{}:
        return len(args),nil
    case []interface{}:
        return len(args),nil
    }
    return -1,errors.New(sf("Cannot determine length of unknown type '%T' in len()",args))
}

func getMemUsage() (uint64,uint64) {
        var m runtime.MemStats
        runtime.ReadMemStats(&m)
        return m.Alloc, m.Sys
}

func enum_names(e string) []string {
    globlock.RLock()
    defer globlock.RUnlock()
    l:=[]string{}
    if len(enum[e].members)==0 {
        return l
    }
    for _,m := range enum[e].ordered {
        l=append(l,m)
    }
    return l
}

func enum_all(e string) []interface{} {
    globlock.RLock()
    defer globlock.RUnlock()
    l:=[]interface{}{}
    if len(enum[e].members)==0 {
        return l
    }
    for _,m := range enum[e].ordered {
        l=append(l,enum[e].members[m])
    }
    return l
}


func buildInternalLib() {

    features["internal"] = Feature{version: 1, category: "debug"}
    categories["internal"] = []string{"last", "last_out", "zsh_version", "bash_version", "bash_versinfo", "user", "os", "home", "lang",
        "release_name", "release_version", "release_id", "winterm", "hostname", "argc","argv",
        "funcs", "dump", "keypress", "tokens", "key", "clear_line","pid","ppid", "system",
        "func_inputs","func_outputs","func_descriptions","func_categories",
        "local", "clktck", "globkey", "getglob", "funcref", "thisfunc", "thisref", "commands","cursoron","cursoroff","cursorx",
        "eval", "term_w", "term_h", "pane_h", "pane_w","utf8supported","execpath","coproc", "capture_shell", "ansi", "interpol", "shellpid", "has_shell",
        "globlen","len","tco", "echo","get_row","get_col","unmap","await","get_mem","zainfo","get_cores","permit","wrap",
        "enum_names","enum_all",
    }


    slhelp["enum_names"] = LibHelp{in: "enum", out: "[]string", action: "returns the name labels associated with enumeration [#i1]enum[#i0]"}
    stdlib["enum_names"] = func(evalfs uint32,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("enum_names",args,1,"1","string"); !ok { return nil,err }
        return enum_names(args[0].(string)),nil
    }

    slhelp["enum_all"] = LibHelp{in: "enum", out: "[]mixed", action: "returns the values associated with enumeration [#i1]enum[#i0]"}
    stdlib["enum_all"] = func(evalfs uint32,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("enum_all",args,1,"1","string"); !ok { return nil,err }
        return enum_all(args[0].(string)),nil
    }

    slhelp["zainfo"] = LibHelp{in: "", out: "struct", action: "internal info: [#i1].version[#i0]: semantic version number, [#i1].name[#i0]: language name, [#i1].build[#i0]: build type"}
    stdlib["zainfo"] = func(evalfs uint32,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("zainfo",args,0); !ok { return nil,err }
        v,_:=vget(0,"@version")
        l,_:=vget(0,"@language")
        c,_:=vget(0,"@ct_info")
        return zainfo{version:v.(string),name:l.(string),build:c.(string)},nil
    }

    slhelp["utf8supported"] = LibHelp{in: "", out: "bool", action: "Is the current language utf-8 compliant? This only works if the environmental variable LANG is available."}
    stdlib["utf8supported"] = func(evalfs uint32,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("utf8supported",args,0); !ok { return nil,err }
        return str.HasSuffix(str.ToLower(os.Getenv("LANG")),".utf-8") , nil
    }

    slhelp["wininfo"] = LibHelp{in: "", out: "int", action: "(windows only) Returns the console geometry."}
    stdlib["wininfo"] = func(evalfs uint32,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("wininfo",args,2,
            "1","int",
            "0"); !ok { return nil,err }
        hnd:=1
        if len(args)==1 {
            switch args[0].(type) {
            case int:
                hnd=args[0].(int)
            }
        }
        return GetWinInfo(hnd), nil
    }

    slhelp["get_mem"] = LibHelp{in: "", out: "struct", action: "Returns the current heap allocated memory and total system memory usage in MB. Structure fields are [#i1].alloc[#i0] and [#i1].system[#i0] for allocated space and total system space respectively."}
    stdlib["get_mem"] = func(evalfs uint32,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("get_mem",args,0); !ok { return nil,err }
        a,s:=getMemUsage()
        return struct{alloc uint64;system uint64}{a/1024/1024,s/1024/1024},nil
    }

    slhelp["get_cores"] = LibHelp{in: "", out: "int", action: "Returns the CPU core count."}
    stdlib["get_cores"] = func(evalfs uint32,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("get_cores",args,0); !ok { return nil,err }
        return runtime.NumCPU(),nil
    }

    slhelp["term_h"] = LibHelp{in: "", out: "int", action: "Returns the current terminal height."}
    stdlib["term_h"] = func(evalfs uint32,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("term_h",args,0); !ok { return nil,err }
        return MH, nil
    }

    slhelp["term_w"] = LibHelp{in: "", out: "int", action: "Returns the current terminal width."}
    stdlib["term_w"] = func(evalfs uint32,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("term_w",args,0); !ok { return nil,err }
        return MW, nil
    }

    slhelp["pane_h"] = LibHelp{in: "", out: "int", action: "Returns the current pane height."}
    stdlib["pane_h"] = func(evalfs uint32,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("pane_h",args,0); !ok { return nil,err }
        return panes[currentpane].h, nil
    }

    slhelp["pane_w"] = LibHelp{in: "", out: "int", action: "Returns the current pane width."}
    stdlib["pane_w"] = func(evalfs uint32,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("pane_w",args,0); !ok { return nil,err }
        return panes[currentpane].w, nil
    }

    slhelp["system"] = LibHelp{in: "string[,bool]", out: "string", action: "Executes command [#i1]string[#i0] and returns a command structure (bool==false) or displays (bool==true) the output."}
    stdlib["system"] = func(evalfs uint32,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("system",args,2,
            "2","string","bool",
            "1","string"); !ok { return nil,err }

        cmd:=args[0].(string)
        if len(args)==2 && args[1]==true {
            system(cmd,true)
            return nil,nil
        }

        return system(cmd,false),nil
    }

    slhelp["argv"] = LibHelp{in: "", out: "[]string", action: "CLI arguments as an array."}
    stdlib["argv"] = func(evalfs uint32,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("argv",args,0); !ok { return nil,err }
        return cmdargs, nil
    }

    slhelp["argc"] = LibHelp{in: "", out: "int", action: "CLI argument count."}
    stdlib["argc"] = func(evalfs uint32,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("argc",args,0); !ok { return nil,err }
        return len(cmdargs), nil
    }

    slhelp["eval"] = LibHelp{in: "string", out: "[mixed]", action: "evaluate expression in [#i1]string[#i0]."}
    stdlib["eval"] = func(evalfs uint32,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("eval",args,1,"1","string"); !ok { return nil,err }
        p:=&leparser{}
        return ev(p,evalfs, args[0].(string))
    }

    slhelp["get_row"] = LibHelp{in: "", out: "int", action: "reads the row position of console text cursor."}
    stdlib["get_row"] = func(evalfs uint32,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("get_row",args,0); !ok { return nil,err }
        r,_:=GetCursorPos()
        if runtime.GOOS=="windows" { r++ }
        return r, nil
    }

    slhelp["get_col"] = LibHelp{in: "", out: "int", action: "reads the column position of console text cursor."}
    stdlib["get_col"] = func(evalfs uint32,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("get_col",args,0); !ok { return nil,err }
        _,c:=GetCursorPos()
        if runtime.GOOS=="windows" { c++ }
        return c, nil
    }

    slhelp["echo"] = LibHelp{in: "[bool[,mask]]", out: "bool", action: "Optionally, enable or disable local echo. Optionally, set the mask character to be used during input. Current visibility state is returned."}
    stdlib["echo"] = func(evalfs uint32,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("echo",args,2,
        "2","bool","string",
        "1","bool"); !ok { return nil,err }

        se:=true
        if args[0].(bool) {
            vset(0, "@echo", true)
        } else {
            se=false
            vset(0, "@echo", false)
        }

        mask,_:=vget(0,"@echomask")
        if len(args)>1 {
            mask=args[1].(string)
        }

        setEcho(se)
        vset(0,"@echomask", mask)
        v,_:=vget(0,"@echo")

        return v,nil
    }

    slhelp["permit"] = LibHelp{in: "behaviour_string,various_types", out: "", action: "Set a run-time behaviour.\nuninit(bool): determine if execution should stop when an uninitialised variable is encountered during evaluation.\ndupmod(bool): ignore duplicate module imports."}
    stdlib["permit"] = func(evalfs uint32,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("permit",args,4,
        "2","string","bool",
        "2","string","int",
        "2","string","float64",
        "2","string","string"); !ok { return nil,err }

        lastlock.Lock()
        defer lastlock.Unlock()

        switch str.ToLower(args[0].(string)) {
        case "uninit":
            switch args[1].(type) {
            case bool:
                permit_uninit=args[1].(bool)
                return nil,nil
            default:
                return nil,errors.New("permit(uninit) accepts a boolean value only.")
            }
        case "dupmod":
            switch args[1].(type) {
            case bool:
                permit_dupmod=args[1].(bool)
                return nil,nil
            default:
                return nil,errors.New("permit(dupmod) accepts a boolean value only.")
            }
        case "exitquiet":
            switch args[1].(type) {
            case bool:
                permit_exitquiet=args[1].(bool)
                return nil,nil
            default:
                return nil,errors.New("permit(exitquiet) accepts a boolean value only.")
            }
        }
        return nil,errors.New("unrecognised behaviour provided in permit() argument 1")
    }

    slhelp["wrap"] = LibHelp{in: "bool", out: "previous_bool", action: "Enable (default) or disable line wrap in panes. Returns the previous state."}
    stdlib["wrap"] = func(evalfs uint32,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("wrap",args,1,"1","bool"); !ok { return nil,err }
        lastwrap:=lineWrap
        lastlock.Lock()
        lineWrap=args[0].(bool)
        lastlock.Unlock()
        return lastwrap,nil
    }

    slhelp["ansi"] = LibHelp{in: "bool", out: "previous_bool", action: "Enable (default) or disable ANSI colour support at runtime. Returns the previous state."}
    stdlib["ansi"] = func(evalfs uint32,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("ansi",args,1,"1","bool"); !ok { return nil,err }
        lastam:=ansiMode
        lastlock.Lock()
        ansiMode=args[0].(bool)
        lastlock.Unlock()
        setupAnsiPalette()
        return lastam,nil
    }

    slhelp["interpol"] = LibHelp{in: "bool", out: "", action: "Enable (default) or disable string interpolation at runtime. This is useful for ensuring that braced phrases remain unmolested."}
    stdlib["interpol"] = func(evalfs uint32,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("interpol",args,1,"1","bool"); !ok { return nil,err }
        lastlock.Lock()
        no_interpolation=!args[0].(bool)
        lastlock.Unlock()
        return nil, nil
    }

    slhelp["coproc"] = LibHelp{in: "bool", out: "", action: "Select if | and =| commands should execute in the coprocess (true) or the current Za process (false)."}
    stdlib["coproc"] = func(evalfs uint32,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("bool",args,1,"1","bool"); !ok { return nil,err }
        vset(0,"@runInParent",!args[0].(bool))
        return nil, nil
    }

    slhelp["capture_shell"] = LibHelp{in: "bool", out: "", action: "Select if | and =| commands should capture output."}
    stdlib["capture_shell"] = func(evalfs uint32,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("capture_shell",args,1,"1","bool"); !ok { return nil,err }
        vset(0,"@commandCapture",args[0].(bool))
        return nil, nil
    }

    slhelp["funcref"] = LibHelp{in: "name", out: "func_ref_num", action: "Find a function handle."}
    stdlib["funcref"] = func(evalfs uint32,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("funcref",args,1,"1","string"); !ok { return nil,err }
        lmv,_:=fnlookup.lmget(args[0].(string))
        return lmv, nil
    }

    slhelp["thisfunc"] = LibHelp{in: "", out: "string", action: "Find this function's name."}
    stdlib["thisfunc"] = func(evalfs uint32,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("thisfunc",args,0); !ok { return nil,err }
        nv,_:=numlookup.lmget(evalfs)
        return nv, nil
    }

    slhelp["thisref"] = LibHelp{in: "", out: "func_ref_num", action: "Find this function's handle."}
    stdlib["thisref"] = func(evalfs uint32,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("thisref",args,0); !ok { return nil,err }
        i,_:=GetAsInt(evalfs)
        return i,nil
    }

    slhelp["tco"] = LibHelp{in: "", out: "bool", action: "are we currently in a tail call loop?"}
    stdlib["tco"] = func(evalfs uint32,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("tco",args,0); !ok { return nil,err }
        b,_:=vget(evalfs,"@in_tco")
        return b.(bool), nil
    }

    slhelp["local"] = LibHelp{in: "string", out: "value", action: "Return this local variable's value."}
    stdlib["local"] = func(evalfs uint32,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("local",args,1,"1","string"); !ok { return nil,err }
        name := args[0].(string)
        v, found := vget(evalfs, name)
        if found { return v, nil }
        return nil, errors.New(sf("'%v' does not exist!", name))
    }

/*
    slhelp["sizeof"] = LibHelp{in: "var", out: "integer", action: "Returns size of object."}
    stdlib["sizeof"] = func(evalfs uint32,args ...interface{}) (ret interface{}, err error) {
        if len(args) == 1 {
            return unsafe.Sizeof(args[0]),nil
        }
        return -1,errors.New("Bad argument in sizeof()")
    }
*/

    slhelp["len"] = LibHelp{in: "various_types", out: "integer", action: "Returns length of string or list."}
    stdlib["len"] = func(evalfs uint32,args ...interface{}) (ret interface{}, err error) {
        if len(args)==1 { return ulen(args[0]) }
        return nil,errors.New("Bad argument in len()")
        /* @note: re-introduce this block (and comment line above) if len has issues with types...
        if ok,err:=expect_args("len",args,18,
            "1","nil",
            "1","string",
            "1","[]interface {}",
            "1","[]string",
            "1","[]int",
            "1","[]int64",
            "1","[]uint8",
            "1","[]float64",
            "1","[]bool",
            "1","map[string]float64",
            "1","map[string]string",
            "1","map[string]int",
            "1","map[string]bool",
            "1","map[string]int64",
            "1","map[string]uint8",
            "1","[]map[string]interface {}",
            "1","map[string]interface {}",
            "1","[]interface {}"); !ok { return nil,err }
        return ulen(args[0])
        */
    }

    slhelp["await"] = LibHelp{in: "handle_map[,all_flag]", out: "[]result", action: "Checks for async completion."}
    stdlib["await"] = func(evalfs uint32,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("await",args,2,
            "2","map[string]interface {}","bool",
            "1","map[string]interface {}"); !ok { return nil,err }

        handleMap:=args[0].(map[string]interface{})

        waitForAll:=false
        if len(args)>1 {
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


    slhelp["unmap"] = LibHelp{in: "ary_name,key_name", out: "bool", action: "Remove a map key. Returns true on successful removal."}
    stdlib["unmap"] = func(evalfs uint32,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("unmap",args,1,"2","string","string"); !ok { return nil,err }
        // @note: mut candidate

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
    stdlib["key"] = func(evalfs uint32,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("key",args,1,"2","string","string"); !ok { return nil,err }
        // @note: ref candidate

        var v interface{}
        var found bool

        if v, found = vget(evalfs, args[0].(string)); !found {
            return false, nil
        }

        key:=interpolate(evalfs,args[1].(string))

        // @todo: check if other built-in types are needed here!
        switch v:=v.(type) {
        case http.Header:
            if _, found = v[key];   found { return true, nil }
        case map[string]float64:
            if _, found = v[key];   found { return true, nil }
        case map[string]uint8:
            if _, found = v[key];   found { return true, nil }
        case map[string]int64:
            if _, found = v[key];   found { return true, nil }
        case map[string]int:
            if _, found = v[key];   found { return true, nil }
        case map[string]bool:
            if _, found = v[key];   found { return true, nil }
        case map[string]string:
            if _, found = v[key];   found { return true, nil }
        case map[string]interface{}:
            if _, found = v[key];   found { return true, nil }
        default:
            pf("unknown type: %T\n",v); os.Exit(0)
        }
        return false, nil
    }

    // may soon be unnecessary (if ref added)
    slhelp["globkey"] = LibHelp{in: "ary_name,key_name", out: "bool", action: "Does key [#i1]key_name[#i0] exist in the global associative array [#i1]ary_name[#i0]?"}
    stdlib["globkey"] = func(evalfs uint32,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("globkey",args,1,"2","string","string"); !ok { return nil,err }

        var v interface{}
        var found bool
        globlock.RLock()
        if v, found = vget(globalaccess, args[0].(string)); !found {
            globlock.RUnlock()
            return false, nil
        }
        globlock.RUnlock()

        key:=interpolate(evalfs,args[1].(string))

        // @todo: other built-in types needed here?
        switch v.(type) {
        case map[string]http.Header:
            if _, found = v.(http.Header)[key];   found { return true, nil }
        case map[string]float64:
            if _, found = v.(map[string]float64)[key];       found { return true, nil }
        case map[string]uint8:
            if _, found = v.(map[string]uint8) [key];        found { return true, nil }
        case map[string]int64:
            if _, found = v.(map[string]int64) [key];        found { return true, nil }
        case map[string]int:
            if _, found = v.(map[string]int) [key];          found { return true, nil }
        case map[string]bool:
            if _, found = v.(map[string]bool)[key];          found { return true, nil }
        case map[string]string:
            if _, found = v.(map[string]string)[key];        found { return true, nil }
        case map[string]interface{}:
            if _, found = v.(map[string]interface{})[key];   found { return true, nil }
        default:
            pf("unknown type: %T\n",v); os.Exit(0)
        }
        return false, nil
    }

    slhelp["last"] = LibHelp{in: "", out: "int", action: "Returns the last received error code from a co-process command."}
    stdlib["last"] = func(evalfs uint32,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("last",args,0); !ok { return nil,err }
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

    slhelp["execpath"] = LibHelp{in: "", out: "string", action: "Returns the initial working directory."}
    stdlib["execpath"] = func(evalfs uint32,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("execpath",args,0); !ok { return nil,err }
        v, _ := vget(0, "@execpath")
        return string(v.(string)), err
    }

    slhelp["last_out"] = LibHelp{in: "", out: "string", action: "Returns the last received error text from the co-process."}
    stdlib["last_out"] = func(evalfs uint32,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("last_out",args,0); !ok { return nil,err }
        v, found := vget(0, "@last_out")
        if found {
            return string(v.([]byte)), err
        }
        return "",errors.New("No co-process error has been detected yet.")
    }

    slhelp["zsh_version"] = LibHelp{in: "", out: "string", action: "Returns the zsh version string if present."}
    stdlib["zsh_version"] = func(evalfs uint32,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("zsh_version",args,0); !ok { return nil,err }
        v, _ := vget(0, "@zsh_version")
        return v.(string), err
    }

    slhelp["bash_version"] = LibHelp{in: "", out: "string", action: "Returns the full release string of the Bash co-process."}
    stdlib["bash_version"] = func(evalfs uint32,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("bash_version",args,0); !ok { return nil,err }
        v, _ := vget(0, "@bash_version")
        return v.(string), err
    }

    slhelp["bash_versinfo"] = LibHelp{in: "", out: "string", action: "Returns the major version number of the Bash co-process."}
    stdlib["bash_versinfo"] = func(evalfs uint32,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("bash_versinfo",args,0); !ok { return nil,err }
        v, _ := vget(0, "@bash_versinfo")
        return v.(string), err
    }

    slhelp["keypress"] = LibHelp{in: "[timeout_μs]", out: "int", action: "Returns an integer corresponding with a keypress. Internally, the minimum timeout value is currently 1 decisecond. The microsecond unit for timeout will remain in case this is revised. Lower timeout requirements should use asynchronous functionality."}
    stdlib["keypress"] = func(evalfs uint32,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("keypress",args,1,
        "1","int",
        "0"); !ok { return nil,err }
        timeo := int64(0)
        if len(args) == 1 {
            switch args[0].(type) {
            case string, int:
                ttmp, terr := GetAsInt(args[0])
                timeo = int64(ttmp)
                if terr { return "", errors.New("Invalid timeout value.") }
            }
        }

        k:=wrappedGetCh(int(timeo))

        if k==3 { // ctrl-c 
            siglock.RLock()
            sig_int=true
            siglock.RUnlock()
        }

        return k,nil
    }

    slhelp["cursoroff"] = LibHelp{in: "", out: "", action: "Disables cursor display."}
    stdlib["cursoroff"] = func(evalfs uint32,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("cursoroff",args,0); !ok { return nil,err }
        hideCursor()
        return nil, nil
    }

    slhelp["cursorx"] = LibHelp{in: "n", out: "", action: "Moves cursor to horizontal position [#i1]n[#i0]."}
    stdlib["cursorx"] = func(evalfs uint32,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("cursorx",args,1,"1","int"); !ok { return nil,err }
        cursorX(args[0].(int))
        return nil, nil
    }

    slhelp["cursoron"] = LibHelp{in: "", out: "", action: "Enables cursor display."}
    stdlib["cursoron"] = func(evalfs uint32,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("cursoron",args,0); !ok { return nil,err }
        showCursor()
        return nil, nil
    }

    slhelp["ppid"] = LibHelp{in: "", out: "int", action: "Return the pid of parent process."}
    stdlib["ppid"] = func(evalfs uint32,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("ppid",args,0); !ok { return nil,err }
        return os.Getppid(), nil
    }

    slhelp["pid"] = LibHelp{in: "", out: "int", action: "Return the pid of the current process."}
    stdlib["pid"] = func(evalfs uint32,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("pid",args,0); !ok { return nil,err }
        return os.Getpid(), nil
    }

    slhelp["clear_line"] = LibHelp{in: "row,col", out: "", action: "Clear to the end of the line, starting at row,col in the current pane."}
    stdlib["clear_line"] = func(evalfs uint32,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("clear_line",args,1,"2","int","int"); !ok { return nil,err }
        atlock.Lock()
        row, rerr := GetAsInt(args[0])
        col, cerr := GetAsInt(args[1])
        atlock.Unlock()
        if !(cerr || rerr) {
            clearToEOPane(row, col)
        }
        return nil, nil
    }

    slhelp["user"] = LibHelp{in: "", out: "string", action: "Returns the parent user of the Bash co-process."}
    stdlib["user"] = func(evalfs uint32,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("user",args,0); !ok { return nil,err }
        v, _ := vget(0, "@user")
        return v.(string), err
    }

    slhelp["os"] = LibHelp{in: "", out: "string", action: "Returns the kernel version name as reported by the coprocess."}
    stdlib["os"] = func(evalfs uint32,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("os",args,0); !ok { return nil,err }
        v, _ := vget(0, "@os")
        return v.(string), err
    }

    slhelp["home"] = LibHelp{in: "", out: "string", action: "Returns the home directory of the user that launched Za as reported by the coprocess."}
    stdlib["home"] = func(evalfs uint32,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("home",args,0); !ok { return nil,err }
        v, _ := vget(0, "@home")
        return v.(string), err
    }

    slhelp["lang"] = LibHelp{in: "", out: "string", action: "Returns the locale name used within the coprocess."}
    stdlib["lang"] = func(evalfs uint32,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("lang",args,0); !ok { return nil,err }
        if v, found := vget(0, "@lang"); found {
            return v.(string), nil
        }
        return "",nil
    }

    slhelp["release_name"] = LibHelp{in: "", out: "string", action: "Returns the OS release name as reported by the coprocess."}
    stdlib["release_name"] = func(evalfs uint32,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("release_name",args,0); !ok { return nil,err }
        v, _ := vget(0, "@release_name")
        return v.(string), err
    }

    slhelp["hostname"] = LibHelp{in: "", out: "string", action: "Returns the current hostname."}
    stdlib["hostname"] = func(evalfs uint32,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("hostname",args,0); !ok { return nil,err }
        z, _ := os.Hostname()
        vset(0, "@hostname", z)
        return z, err
    }

    slhelp["tokens"] = LibHelp{in: "string", out: "struct", action: "Returns a structure containing a list of tokens ([#i1].tokens[#i0]) in a string and a list ([#i1].types[#i0]) of token types."}
    stdlib["tokens"] = func(evalfs uint32,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("tokens",args,1,"1","string"); !ok { return nil,err }
        tt := Error
        var toks []string
        var toktypes []string
        cl := int16(1)
        for p := 0; p < len(args[0].(string)); {
            t, tokPos, eol, eof := nextToken(args[0].(string), &cl, p, tt)
            tt = t.tokType
            if tokPos != -1 {
                p = tokPos
            }
            toks = append(toks, t.tokText)
            toktypes = append(toktypes, tokNames[tt])
            if eof || eol {
                break
            }
        }
        return token_result{tokens:toks,types:toktypes}, err
    }

    slhelp["release_version"] = LibHelp{in: "", out: "string", action: "Returns the OS version number."}
    stdlib["release_version"] = func(evalfs uint32,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("release_version",args,0); !ok { return nil,err }
        v, _ := vget(0, "@release_version")
        return v.(string), err
    }

    slhelp["release_id"] = LibHelp{in: "", out: "string", action: "Returns the /etc derived release name."}
    stdlib["release_id"] = func(evalfs uint32,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("release_id",args,0); !ok { return nil,err }
        v, _ := vget(0, "@release_id")
        return v.(string), err
    }

    slhelp["winterm"] = LibHelp{in: "", out: "bool", action: "Is this a WSL terminal?"}
    stdlib["winterm"] = func(evalfs uint32,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("winterm",args,0); !ok { return nil,err }
        v, _ := vget(0, "@winterm")
        return v.(bool), err
    }

    slhelp["commands"] = LibHelp{in: "", out: "", action: "Displays a list of keywords."}
    stdlib["commands"] = func(evalfs uint32,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("commands",args,0); !ok { return nil,err }
        commands()
        return nil, nil
    }

    slhelp["func_inputs"] = LibHelp{in: "", out: "[]string", action: "Returns a list of standard library function inputs."}
    stdlib["func_inputs"] = func(evalfs uint32,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("func_inputs",args,0); !ok { return nil,err }
        var fm = make(map[string]string)
        for k,i:=range slhelp {
            fm[k]=i.in
        }
        return fm,nil
    }

    slhelp["func_outputs"] = LibHelp{in: "", out: "[]string", action: "Returns a list of standard library function outputs."}
    stdlib["func_outputs"] = func(evalfs uint32,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("func_outputs",args,0); !ok { return nil,err }
        var fm = make(map[string]string)
        for k,i:=range slhelp {
            fm[k]=i.out
        }
        return fm,nil
    }

    slhelp["func_descriptions"] = LibHelp{in: "", out: "[]string", action: "Returns a list of standard library function descriptions."}
    stdlib["func_descriptions"] = func(evalfs uint32,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("func_descriptions",args,0); !ok { return nil,err }
        var fm = make(map[string]string)
        for k,i:=range slhelp {
            fm[k]=i.action
        }
        return fm,nil
    }

    slhelp["func_categories"] = LibHelp{in: "", out: "[]string", action: "Returns a list of standard library functions."}
    stdlib["func_categories"] = func(evalfs uint32,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("func_categories",args,0); !ok { return nil,err }
        return categories,nil
    }

    slhelp["funcs"] = LibHelp{in: "[partial_match[,bool_return]]", out: "string", action: "Returns a list of standard library functions."}
    stdlib["funcs"] = func(evalfs uint32,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("funcs",args,3,
        "2","string","bool",
        "1","string",
        "0"); !ok { return nil,err }

        if len(args) == 0 {
            args = append(args, "")
        }

        retstring:=false
        if len(args)==2 { retstring=args[1].(bool) }

        regex := ""
        funclist := ""
        if args[0].(string) != "" { regex = args[0].(string) }

        // sort the keys
        var keys []string
        for k :=range categories { keys=append(keys,k) }
        sort.Strings(keys)

        for _,k := range keys {
        c := k
        v := categories[k]
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

    // @todo: review this. we probably want to only have this available in interactive mode and then only for global scope.
    slhelp["dump"] = LibHelp{in: "function_name", out: "", action: "Displays variable list, or a specific entry."}
    stdlib["dump"] = func(evalfs uint32,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("dump",args,2,
        "1","string",
        "0"); !ok { return nil,err }
        s := ""
        if len(args) == 0 { s="global" }
        if len(args) == 1 {
            s = args[0].(string)
        }
        if s != "" {
            lmv,found:=fnlookup.lmget(s)
            if found {
                vc:=int(functionidents[lmv])
                for q := 0; q < vc; q++ {
                    v := ident[lmv][q]
                    if v.IName=="" { continue }
                    if v.IName[0]=='@' { continue }
                    pf("%s = %v\n", v.IName, v.IValue)
                }
            } else {
                pf("Invalid space name provided '%v'.\n",s)
            }
        }
        return true, err
    }

    slhelp["has_shell"] = LibHelp{in: "", out: "bool", action: "Check if a child co-process has been launched."}
    stdlib["has_shell"] = func(evalfs uint32,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("has_shell",args,0); !ok { return nil,err }
        v, _ := vget(0,"@noshell")
        return !v.(bool), nil
    }

    slhelp["shellpid"] = LibHelp{in: "", out: "int", action: "Get process ID of the launched child co-process."}
    stdlib["shellpid"] = func(evalfs uint32,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("shellpid",args,0); !ok { return nil,err }
        v, _ := vget(0,"@shellpid")
        return v, nil
    }

    slhelp["clktck"] = LibHelp{in: "", out: "int", action: "Get clock ticks from aux file."}
    stdlib["clktck"] = func(evalfs uint32,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("clktck",args,0); !ok { return nil,err }
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

