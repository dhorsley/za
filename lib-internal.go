// +build !test

package main


import (
    "encoding/binary"
    "errors"
    "fmt"
    "io/ioutil"
    "math/big"
    "os"
    "unicode/utf8"
    "net/http" // for key()
    "regexp"
    "runtime"
    "sort"
    str "strings"
    "sync/atomic"
//     "golang.org/x/sys/unix"
)


var execMode bool   // used by report() for errors
var execFs   uint32 // used by report() for errors

const (
    _AT_NULL             = 0
    _AT_CLKTCK           = 17
    _SYSTEM_CLK_TCK      = 100
    uintSize        uint = 32 << (^uint(0) >> 63)
)

func ulen(args any) (int,error) {
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
    case []*big.Int:
        return len(args),nil
    case []*big.Float:
        return len(args),nil
    case []uint8:
        return len(args),nil
    case []float64:
        return len(args),nil
    case []bool:
        return len(args),nil
    case []dirent:
        return len(args),nil
    case map[string]float64:
        return len(args),nil
    case map[string]string:
        return len(args),nil
    case map[string][]string:
        return len(args),nil
    case map[string]int:
        return len(args),nil
    case map[string]bool:
        return len(args),nil
    case map[string]int64:
        return len(args),nil
    case map[string]uint8:
        return len(args),nil
    case []map[string]any:
        return len(args),nil
    case map[string]any:
        return len(args),nil
    case [][]int:
        return len(args),nil
    case []any:
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
    if _, found := enum[e]; ! found { return l }
    if len(enum[e].members)==0 {
        return l
    }
    for _,m := range enum[e].ordered {
        l=append(l,m)
    }
    return l
}

func enum_all(e string) []any {
    globlock.RLock()
    defer globlock.RUnlock()
    l:=[]any{}
    if _, found := enum[e]; ! found { return l }
    if len(enum[e].members)==0 {
        return l
    }
    for _,m := range enum[e].ordered {
        l=append(l,enum[e].members[m])
    }
    return l
}

func GetAst(fn string) (ast string) {
    var ifn uint32
    var present bool
    if ifn, present = fnlookup.lmget(fn); !present {
        return
    }

    if ifn < uint32(len(functionspaces)) {

        if str.HasPrefix(fn,"@mod_") {
            return
        }

        var falist []string
        for _,fav:=range functionArgs[ifn].args {
            falist=append(falist,fav)
        }

        first := true

        indent:=0
        istring:=""

        for q := range functionspaces[ifn] {
            // strOut := "\t\t "
            if first == true {
                first = false
                // strOut = sf("\n[#4][#bold]%s(%v)[#boff][#-]\n\t\t ", fn, str.Join(falist, ","))
            }

            switch functionspaces[ifn][q].Tokens[0].tokType {
            case C_Endfor, C_Endwhile, C_Endif, C_Endwhen:
                indent--
            }

            istring=str.Repeat("....",indent)

                // ast+=sf("%sLine (bytes:%d)  : "+sparkle("[#1]" + basecode[ifn][q].Original + "[#-]")+"\n",istring,Of(functionspaces[ifn][q]))
                ast+=sf("%sLine (bytes:%d)  :\n",istring,Of(functionspaces[ifn][q]))

            for tk,tv:=range functionspaces[ifn][q].Tokens {
                ast+=sf("%s%6d : ",istring,1+tk)
                subast1:=sf("(%s",tokNames[tv.tokType])
                if tv.subtype!=0 {
                    subast1+=sf(",subtype:%s",subtypeNames[tv.subtype])
                }
                subast1+=sf(")")
                ast+=sf("%29s",subast1)
                show:=str.TrimSpace(tr(str.Replace(tv.tokText,"\n"," ",-1),SQUEEZE," ",""))
                ast+=sparkle(sf(" [#1]%+v[#-]",show))
                switch tv.tokVal.(type) {
                default:
                    if tv.tokVal!=nil {
                        ast+=sf(" Value : %+v (%T)",tv.tokVal,tv.tokVal)
                    }
                }
                ast+="\n"

            }
            ast+="\n"

            switch functionspaces[ifn][q].Tokens[0].tokType {
            case C_For, C_Foreach, C_While, C_If, C_When:
                indent++
            }

        }

    }

    return ast
}


/* for future use:
func sttyFlag(flags string,state bool) (okay bool) {
    termios, err := unix.IoctlGetTermios(0, ioctlReadTermios)
    newState := *termios
    if err!=nil {
        return false
    }
    for fp:=0;fp<len(flags); fp++ {
        f:=flags[fp]
        if state {
            switch f {
            case 'n':
                newState.Iflag |= unix.ICRNL
            case 'i':
                newState.Iflag |= unix.INLCR
                newState.Iflag |= unix.IGNCR
            case 'u':
                newState.Iflag |= unix.IUCLC
            case 's':
                newState.Lflag |= unix.ISIG
            case 'c':
                newState.Lflag |= unix.ICANON
            case 'e':
                newState.Lflag |= unix.ECHO
            }
        } else {
            switch f {
            case 'n':
                newState.Iflag &^= unix.ICRNL
            case 'i':
                newState.Iflag &^= unix.INLCR
                newState.Iflag &^= unix.IGNCR
            case 'u':
                newState.Iflag &^= unix.IUCLC
            case 's':
                newState.Lflag &^= unix.ISIG
            case 'c':
                newState.Lflag &^= unix.ICANON
            case 'e':
                newState.Lflag &^= unix.ECHO
            }
        }
        unix.IoctlSetTermios(0, ioctlWriteTermios, &newState)
    }
    return true
}
*/

func buildInternalLib() {

    features["internal"] = Feature{version: 1, category: "debug"}
    categories["internal"] = []string{"last", "last_out", "zsh_version", "bash_version", "bash_versinfo", "user", "os", "home", "lang",
        "release_name", "release_version", "release_id", "winterm", "hostname", "argc","argv",
        "funcs", "keypress", "tokens", "key", "clear_line","pid","ppid", "system",
        "func_inputs","func_outputs","func_descriptions","func_categories",
        "local", "clktck", "glob_key", "funcref", "thisfunc", "thisref","cursoron","cursoroff","cursorx",
        "eval", "exec", "term_w", "term_h", "pane_h", "pane_w","pane_r","pane_c","utf8supported","execpath","coproc",
        "capture_shell", "ansi", "interpol", "shell_pid", "has_shell", "has_term","has_colour",
        "len","echo","get_row","get_col","unmap","await","get_mem","zainfo","get_cores","permit",
        "enum_names","enum_all","dump","sysvar",
        "ast","varbind","sizeof","dup",
        // "conread","conwrite","conset","conclear", : for future use.
    }

    slhelp["gdump"] = LibHelp{in: "function_name", out: "", action: "Displays system variable list."}
    stdlib["gdump"] = func(evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("gdump",args,1,"0"); !ok { return nil,err }
        for e:=0;e<len(gident);e++ {
            if gident[e].declared {
                pf("%s = %v\n", gident[e].IName, gident[e].IValue)
            }
        }
        return nil, nil
    }

    slhelp["dump"] = LibHelp{in: "function_name", out: "", action: "Displays in-scope variable list."}
    stdlib["dump"] = func(evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("dump",args,1,"0"); !ok { return nil,err }
        for e:=0;e<len(*ident);e++ {
            if (*ident)[e].declared {
                pf("%s = %v\n", (*ident)[e].IName, (*ident)[e].IValue)
            }
        }
        return nil, nil
    }

    /*
    slhelp["symtest"] = LibHelp{in: "none", out: "none", action: "(debug)"}
    stdlib["symtest"] = func(evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        var q = make(map[uint64]int)
        start:=""
        if len(args)>1 { start=args[1].(string) }
        for e:=0; e<args[0].(int); e++ {
            bie:=bind_int(evalfs,sf("%s%d",start))
            if _,there:=q[bie]; there {
                pf("* clash on %s\n",sf("%s%d",start,e))
            }
            q[bie]++
        }
        return len(q),nil
    }
*/

    slhelp["dup"] = LibHelp{in: "map", out: "copy_of_map", action: "returns a duplicate copy of [#i1]map[#i0]."}
    stdlib["dup"] = func(evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("dup",args,17,
            "1","map[string]string",
            "1","map[string]bool",
            "1","map[string]int",
            "1","map[string]uint",
            "1","map[string]float64",
            "1","map[string]*big.Int",
            "1","map[string]*big.Float",
            "1","map[string]interface {}",
            "1","string",
            "1","[]string",
            "1","[]bool",
            "1","[]int",
            "1","[]uint",
            "1","[]float64",
            "1","[]*big.Int",
            "1","[]*big.Float",
            "1","[]interface {}"); !ok { return nil,err }

        switch m:=args[0].(type) {
        case map[string]string:
            m2:=make(map[string]string)
            for id, v := range m { m2[id] = v }
            return m2,nil
        case map[string]bool:
            m2:=make(map[string]bool)
            for id, v := range m { m2[id] = v }
            return m2,nil
        case map[string]int:
            m2:=make(map[string]int)
            for id, v := range m { m2[id] = v }
            return m2,nil
        case map[string]uint:
            m2:=make(map[string]uint)
            for id, v := range m { m2[id] = v }
            return m2,nil
        case map[string]float64:
            m2:=make(map[string]float64)
            for id, v := range m { m2[id] = v }
            return m2,nil
        case map[string]*big.Int:
            m2:=make(map[string]*big.Int)
            for id, v := range m { m2[id] = v }
            return m2,nil
        case map[string]*big.Float:
            m2:=make(map[string]*big.Float)
            for id, v := range m { m2[id] = v }
            return m2,nil
        case map[string]interface{}:
            m2:=make(map[string]interface{})
            for id, v := range m { m2[id] = v }
            return m2,nil
        case []bool:
            a2:=make([]bool,len(m),cap(m))
            copy(a2,m)
            return a2,nil
        case []int:
            a2:=make([]int,len(m),cap(m))
            copy(a2,m)
            return a2,nil
        case []uint:
            a2:=make([]uint,len(m),cap(m))
            copy(a2,m)
            return a2,nil
        case []float64:
            a2:=make([]float64,len(m),cap(m))
            copy(a2,m)
            return a2,nil
        case string:
            return str.Clone(m),nil
        case []string:
            a2:=make([]string,len(m),cap(m))
            copy(a2,m)
            return a2,nil
        case []*big.Int:
            a2:=make([]*big.Int,len(m),cap(m))
            copy(a2,m)
            return a2,nil
        case []*big.Float:
            a2:=make([]*big.Float,len(m),cap(m))
            copy(a2,m)
            return a2,nil
        case []interface{}:
            a2:=make([]interface{},len(m),cap(m))
            copy(a2,m)
            return a2,nil

        default:
            return nil,errors.New(sf("dup requires a map, not a %T",args[0]))
        }
    }
    slhelp["sizeof"] = LibHelp{in: "string", out: "uint", action: "returns the size of an object."}
    stdlib["sizeof"] = func(evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("sizeof",args,1,"1","any"); !ok { return nil,err }
        return Of(args[0]),nil
    }

    slhelp["varbind"] = LibHelp{in: "string", out: "uint", action: "returns the name binding uint for a variable."}
    stdlib["varbind"] = func(evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("varbind",args,1,"1","string"); !ok { return nil,err }
        return bind_int(evalfs,args[0].(string)),nil
    }

    slhelp["enum_names"] = LibHelp{in: "enum", out: "[]string", action: "returns the name labels associated with enumeration [#i1]enum[#i0]"}
    stdlib["enum_names"] = func(evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("enum_names",args,1,"1","string"); !ok { return nil,err }
        return enum_names(args[0].(string)),nil
    }

    slhelp["enum_all"] = LibHelp{in: "enum", out: "[]mixed", action: "returns the values associated with enumeration [#i1]enum[#i0]"}
    stdlib["enum_all"] = func(evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("enum_all",args,1,"1","string"); !ok { return nil,err }
        return enum_all(args[0].(string)),nil
    }

    /*
    slhelp["conread"] = LibHelp{in: "", out: "termios_struct", action: "reads console state struct."}
    stdlib["conread"] = func(evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("conread",args,1,"0"); !ok { return nil,err }
        termios, err := unix.IoctlGetTermios(0, ioctlReadTermios)
        if err!=nil {
            return nil,err
        }
        return termios,nil
    }

    slhelp["conwrite"] = LibHelp{in: "", out: "int", action: "writes console state struct."}
    stdlib["conwrite"] = func(evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("conwrite",args,1,"1","*unix.Termios"); !ok { return nil,err }
        return nil,unix.IoctlSetTermios(0, ioctlWriteTermios, args[0].(*unix.Termios))
    }

    slhelp["conclear"] = LibHelp{in: "string", out: "bool", action: "resets console state bits. returns success flag.\nFlags are n:ICRNL i:IGNCR u:IUCLC s:ISIG c:ICANON e:ECHO\nSee man page termios (3) for further details."}
    stdlib["conclear"] = func(evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("conclear",args,1,"1","string"); !ok { return nil,err }
        return sttyFlag(args[0].(string),false),nil
    }

    slhelp["conset"] = LibHelp{in: "string", out: "bool", action: "sets console state bits. returns success flag.\nFlags are n:ICRNL i:IGNCR u:IUCLC s:ISIG c:ICANON e:ECHO\nSee man page termios (3) for further details."}
    stdlib["conset"] = func(evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("conset",args,1,"1","string"); !ok { return nil,err }
        return sttyFlag(args[0].(string),true),nil
    }
    */

    slhelp["sysvar"] = LibHelp{in: "system_variable_name", out: "struct", action: "Returns the value of a system variable."}
    stdlib["sysvar"] = func(evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("sysvar",args,1,"1","string"); !ok { return nil,err }
        v,_:=gvget(args[0].(string))
        return v,nil
    }

    slhelp["zainfo"] = LibHelp{in: "", out: "struct", action: "internal info: [#i1].version[#i0]: semantic version number, [#i1].name[#i0]: language name, [#i1].build[#i0]: build type"}
    stdlib["zainfo"] = func(evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("zainfo",args,0); !ok { return nil,err }
        v,_:=gvget("@version")
        l,_:=gvget("@language")
        c,_:=gvget("@ct_info")
        return zainfo{version:v.(string),name:l.(string),build:c.(string)},nil
    }

    slhelp["dinfo"] = LibHelp{in: "var", out: "struct", action: "(debug) show var info."}
    stdlib["dinfo"] = func(evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("dinfo",args,0); !ok { return nil,err }
        bindlock.RLock()
        pf("EvalFS  : %d\n",evalfs)
        pf("Bindings:\n%#v\n",bindings[evalfs])
        pf("Ident   :\n")
        bindlock.RUnlock()
        for k,i:=range *ident {
            pf("%3d : %+v\n",k,i)
        }
        pf("\n")
        return nil,nil
    }

    slhelp["utf8supported"] = LibHelp{in: "", out: "bool", action: "Is the current language utf-8 compliant? This only works if the environmental variable LANG is available."}
    stdlib["utf8supported"] = func(evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("utf8supported",args,0); !ok { return nil,err }
        return str.HasSuffix(str.ToLower(os.Getenv("LANG")),".utf-8") , nil
    }

    slhelp["wininfo"] = LibHelp{in: "", out: "int", action: "(windows only) Returns the console geometry."}
    stdlib["wininfo"] = func(evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("wininfo",args,2,
            "1","int",
            "0"); !ok { return nil,err }
        hnd:=1
        if len(args)==1 {
            hnd=args[0].(int)
        }
        return GetWinInfo(hnd), nil
    }

    slhelp["get_mem"] = LibHelp{in: "", out: "struct",
        action: "Returns the current heap allocated memory and total system memory usage in MB.\n"+
        "Structure fields are [#i1].alloc[#i0] and [#i1].system[#i0] for allocated space and total system space respectively."}
    stdlib["get_mem"] = func(evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("get_mem",args,0); !ok { return nil,err }
        a,s:=getMemUsage()
        return struct{alloc uint64;system uint64}{a/1024/1024,s/1024/1024},nil
    }

    slhelp["get_cores"] = LibHelp{in: "", out: "int", action: "Returns the CPU core count."}
    stdlib["get_cores"] = func(evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("get_cores",args,0); !ok { return nil,err }
        return runtime.NumCPU(),nil
    }

    slhelp["term_h"] = LibHelp{in: "", out: "int", action: "Returns the current terminal height."}
    stdlib["term_h"] = func(evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("term_h",args,0); !ok { return nil,err }
        return MH, nil
    }

    slhelp["term_w"] = LibHelp{in: "", out: "int", action: "Returns the current terminal width."}
    stdlib["term_w"] = func(evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("term_w",args,0); !ok { return nil,err }
        return MW, nil
    }

    slhelp["pane_h"] = LibHelp{in: "", out: "int", action: "Returns the current pane height."}
    stdlib["pane_h"] = func(evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("pane_h",args,0); !ok { return nil,err }
        return panes[currentpane].h, nil
    }

    slhelp["pane_w"] = LibHelp{in: "", out: "int", action: "Returns the current pane width."}
    stdlib["pane_w"] = func(evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("pane_w",args,0); !ok { return nil,err }
        return panes[currentpane].w, nil
    }

    slhelp["pane_r"] = LibHelp{in: "", out: "int", action: "Returns the current pane start row."}
    stdlib["pane_r"] = func(evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("pane_r",args,0); !ok { return nil,err }
        return panes[currentpane].row, nil
    }

    slhelp["pane_c"] = LibHelp{in: "", out: "int", action: "Returns the current pane start column."}
    stdlib["pane_c"] = func(evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("pane_c",args,0); !ok { return nil,err }
        return panes[currentpane].col, nil
    }

    slhelp["system"] = LibHelp{in: "string[,bool]", out: "string", action: "Executes command [#i1]string[#i0] and returns a command structure (bool==false) or displays (bool==true) the output."}
    stdlib["system"] = func(evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("system",args,2,
            "2","string","bool",
            "1","string"); !ok { return nil,err }

        cmd:=interpolate(evalfs,ident,args[0].(string))
        if len(args)==2 && args[1]==true {
            system(cmd,true)
            return nil,nil
        }

        return system(cmd,false),nil
    }

    slhelp["argv"] = LibHelp{in: "", out: "[]string", action: "CLI arguments as an array."}
    stdlib["argv"] = func(evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("argv",args,0); !ok { return nil,err }
        return cmdargs, nil
    }

    slhelp["argc"] = LibHelp{in: "", out: "int", action: "CLI argument count."}
    stdlib["argc"] = func(evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("argc",args,0); !ok { return nil,err }
        return len(cmdargs), nil
    }

    slhelp["eval"] = LibHelp{in: "string", out: "[mixed]", action: "evaluate expression in [#i1]string[#i0]."}
    stdlib["eval"] = func(evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("eval",args,1,"1","string"); !ok { return nil,err }

        if !permit_eval {
            panic(fmt.Errorf("eval() not permitted!"))
        }

        p:=&leparser{}
        calllock.RLock()
        p.ident=ident
        p.fs=evalfs
        calllock.RUnlock()
        // pf("-- [eval] q:|%s|\n",args[0].(string))
        return ev(p,evalfs,args[0].(string))
    }

    slhelp["exec"] = LibHelp{in: "string", out: "return_values", action: "execute code in [#i1]string[#i0]."}
    stdlib["exec"] = func(evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {

        if !permit_eval {
            panic(fmt.Errorf("exec() not permitted!"))
        }

        execMode=true // racey: don't care, error reporting stuff
        execFs=evalfs

        var code string
        if len(args)>0 {
            switch args[0].(type) {
            case string:
                code=args[0].(string)+"\n"
            default:
                return nil,errors.New("exec requires a string to lex.")
            }
        }

        // allocate function space for source
        sloc,sfn:=GetNextFnSpace(true,"exec@",call_s{prepared:true,caller:evalfs})

        // parse
        badword,_:=phraseParse(sfn, code, 0)
        if badword {
            return nil,errors.New("exec could not lex input.")
        }

        // allocate function space for execution
        eloc,efn:=GetNextFnSpace(true,sfn+"@",call_s{prepared:true})
        cs := calltable[eloc]
        cs.caller   = evalfs
        cs.base     = sloc
        cs.retvals  = nil
        cs.fs       = efn
        calltable[eloc]=cs
        var instance_ident = make([]Variable,identInitialSize)

        // pf("[#5](debug-exec) : sloc -> %d eloc -> %d[#-]\n",sloc,eloc)
        // pf("[#5](debug-exec) : executing -> [%+v][#-]\n",code)

        // execute code
        atomic.AddInt32(&concurrent_funcs,1)
        var rcount uint8
        if len(args)>1 {
            rcount,_=Call(MODE_NEW, &instance_ident, eloc, ciEval, args[1:]...)
        } else {
            rcount,_=Call(MODE_NEW, &instance_ident, eloc, ciEval)
        }

        execMode=false
        atomic.AddInt32(&concurrent_funcs,-1)

        // get return values
        calllock.Lock()
        res := calltable[eloc].retvals
        calltable[eloc].gcShyness=50
        calltable[eloc].gc=true
        calltable[eloc].disposable=true
        calllock.Unlock()

        // throw away the tokenised source
        fnlookup.lmdelete(sfn)
        numlookup.lmdelete(sloc)

        // throw away the code instance block
        fnlookup.lmdelete(efn)
        numlookup.lmdelete(eloc)

        // parse return'ed values
        switch rcount {
        case 0:
            return nil,nil
        case 1:
            return res.([]any)[0],nil
        default:
            return res,nil
        }
        return res,nil

    }

    slhelp["get_row"] = LibHelp{in: "", out: "int", action: "reads the row position of console text cursor."}
    stdlib["get_row"] = func(evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("get_row",args,0); !ok { return nil,err }
        r,_:=GetCursorPos()
        if runtime.GOOS=="windows" { r++ }
        return r, nil
    }

    slhelp["get_col"] = LibHelp{in: "", out: "int", action: "reads the column position of console text cursor."}
    stdlib["get_col"] = func(evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("get_col",args,0); !ok { return nil,err }
        _,c:=GetCursorPos()
        if runtime.GOOS=="windows" { c++ }
        return c, nil
    }

    slhelp["echo"] = LibHelp{in: "[bool[,mask]]", out: "bool",
        action: "Enable or disable local echo. Optionally, set the mask character to be used during input.\n"+
            "Current visibility state is returned when no arguments are provided."}
    stdlib["echo"] = func(evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("echo",args,2,
        "2","bool","string",
        "1","bool"); !ok { return nil,err }

        se:=true
        if args[0].(bool) {
            gvset("@echo", true)
        } else {
            se=false
            gvset("@echo", false)
        }

        mask,_:=gvget("@echomask")
        if len(args)>1 {
            mask=args[1].(string)
        }

        setEcho(se)
        gvset("@echomask", mask)
        v,_:=gvget("@echo")

        return v,nil
    }

    slhelp["permit"] = LibHelp{in: "behaviour_string,various_types", out: "", action: "Set a run-time behaviour.\nuninit(bool): determine if execution should stop when an uninitialised variable is encountered during evaluation.\ndupmod(bool): ignore duplicate module imports.\nexitquiet(bool): shorter error message.\nshell(bool): permit shell commands,  eval(bool): permit eval() calls,  interpol(bool): permit string interpolation."}
    stdlib["permit"] = func(evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
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
        case "fallback":
            switch args[1].(type) {
            case bool:
                permit_fallback:=args[1].(bool)
                gvset("@command_fallback",permit_fallback)
                return nil,nil
            default:
                return nil,errors.New("permit(fallback) accepts a boolean value only.")
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
        case "shell":
            switch args[1].(type) {
            case bool:
                permit_shell=args[1].(bool)
                return nil,nil
            default:
                return nil,errors.New("permit(shell) accepts a boolean value only.")
            }
        case "eval":
            switch args[1].(type) {
            case bool:
                permit_eval=args[1].(bool)
                return nil,nil
            default:
                return nil,errors.New("permit(eval) accepts a boolean value only.")
            }
        case "interpol":
            switch args[1].(type) {
            case bool:
                interpolation=args[1].(bool)
                return nil,nil
            default:
                return nil,errors.New("permit(interpol) accepts a boolean value only.")
            }
        }

        return nil,errors.New("unrecognised behaviour provided in permit() argument 1")
    }

    slhelp["ansi"] = LibHelp{in: "bool", out: "previous_bool", action: "Enable (default) or disable ANSI colour support at runtime. Returns the previous state."}
    stdlib["ansi"] = func(evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("ansi",args,1,"1","bool"); !ok { return nil,err }
        lastam:=ansiMode
        lastlock.Lock()
        ansiMode=args[0].(bool)
        lastlock.Unlock()
        setupAnsiPalette()
        return lastam,nil
    }

    slhelp["feed"] = LibHelp{in: "bool", out: "bool", action: "(debug) Toggle for enforced interactive mode line feed."}
    stdlib["feed"] = func(evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("feed",args,1,"1","bool"); !ok { return nil,err }
        lastlock.Lock()
        interactiveFeed=args[0].(bool)
        lastlock.Unlock()
        return nil, nil
    }

    slhelp["interpol"] = LibHelp{in: "bool", out: "bool",
        action: "Enable (default) or disable string interpolation at runtime.\n"+
            "This is useful for ensuring that braced phrases remain unmolested. Returns the previous state."}
    stdlib["interpol"] = func(evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("interpol",args,1,"1","bool"); !ok { return nil,err }
        lastlock.Lock()
        prev:=interpolation
        interpolation=args[0].(bool)
        lastlock.Unlock()
        return prev, nil
    }

    slhelp["coproc"] = LibHelp{in: "bool", out: "", action: "Select if | and =| commands should execute in the coprocess (true) or the current Za process (false)."}
    stdlib["coproc"] = func(evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("bool",args,1,"1","bool"); !ok { return nil,err }
        gvset("@runInParent",!args[0].(bool))
        return nil, nil
    }

    slhelp["capture_shell"] = LibHelp{in: "bool", out: "", action: "Select if | and =| commands should capture output."}
    stdlib["capture_shell"] = func(evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("capture_shell",args,1,"1","bool"); !ok { return nil,err }
        gvset("@commandCapture",args[0].(bool))
        return nil, nil
    }

    slhelp["funcref"] = LibHelp{in: "name", out: "func_ref_num", action: "Find a function handle."}
    stdlib["funcref"] = func(evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("funcref",args,1,"1","string"); !ok { return nil,err }
        lmv,_:=fnlookup.lmget(args[0].(string))
        return lmv, nil
    }

    slhelp["thisfunc"] = LibHelp{in: "", out: "string", action: "Find this function's name."}
    stdlib["thisfunc"] = func(evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("thisfunc",args,0); !ok { return nil,err }
        nv,_:=numlookup.lmget(evalfs)
        return nv, nil
    }

    slhelp["thisref"] = LibHelp{in: "", out: "func_ref_num", action: "Find this function's handle."}
    stdlib["thisref"] = func(evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("thisref",args,0); !ok { return nil,err }
        i,_:=GetAsInt(evalfs)
        return i,nil
    }

    slhelp["local"] = LibHelp{in: "string", out: "value", action: "Return this local variable's value."}
    stdlib["local"] = func(evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("local",args,1,"1","string"); !ok { return nil,err }
        name := args[0].(string)
        v, found := vget(nil,evalfs,ident, name)
        if found { return v, nil }
        return nil, errors.New(sf("'%v' does not exist!", name))
    }

    slhelp["len"] = LibHelp{in: "various_types", out: "integer", action: "Returns length of string or list."}
    stdlib["len"] = func(evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if len(args)==1 { return ulen(args[0]) }
        return nil,errors.New("Bad argument in len()")
    }

    slhelp["await"] = LibHelp{in: "handle_map[,all_flag]", out: "[]result", action: "Checks for async completion."}
    stdlib["await"] = func(evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("await",args,2,
            "2","string","bool",
            "1","string"); !ok { return nil,err }

        waitForAll:=false
        if len(args)>1 {
            waitForAll=args[1].(bool)
        }

        bin:=bind_int(evalfs,args[0].(string))

        switch args[0].(type) {
        case string:
            if ! (*ident)[bin].declared {
                return nil, errors.New("await requires the name of a local handle map")
            }
        }

        var results=make(map[string]any)

        keepWaiting:=true

        for ; keepWaiting ; {

            // Have to lock this as the results may be updated
            // concurrently while this loop is running.

            vlock.Lock()
            chanTableCopy:=(*ident)[bin].IValue.(map[string]any)

            for k,v:=range chanTableCopy {

                select {
                case retval := <-v.(chan any):

                    if retval==nil { // shouldn't happen
                        pf("(k %v) is nil. still waiting for it.\n",k)
                        os.Exit(1) // but you never know!
                    }

                    loc      :=retval.(struct{l uint32;r any}).l
                    results[k]=retval.(struct{l uint32;r any}).r

                    // close the channel, yes i know, not at the client end, etc
                    close(v.(chan any))

                    calllock.Lock()

                    calltable[loc].gcShyness=100
                    calltable[loc].gc=true

                    // remove async/await pair from handle list
                    delete(chanTableCopy,k)

                    calllock.Unlock()

                default:
                }

            }

            (*ident)[bin].IValue=chanTableCopy

            keepWaiting=false
            if waitForAll {
                if len((*ident)[bin].IValue.(map[string]any))!=0 {
                    keepWaiting=true
                }
            }

            vlock.Unlock()

        }
        return results,nil
    }


    slhelp["unmap"] = LibHelp{in: "ary_name,key_name", out: "bool", action: "Remove a map key. Returns true on successful removal."}
    stdlib["unmap"] = func(evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("unmap",args,1,"2","string","string"); !ok { return nil,err }

        var v any
        var found bool

        if v, found = vget(nil,evalfs,ident, args[0].(string)); !found {
            return false, nil
        }

        switch v.(type) {
        case map[string]any,map[string]int,map[string]float64,map[string]int64:
        case map[string]bool,map[string]uint:
        default:
            return false, errors.New("unmap requires a map")
        }

        if _, found = v.(map[string]any)[args[1].(string)].(any); found {
            vdelete(evalfs,ident,args[0].(string),args[1].(string))
            return true,nil
        }
        return false, nil
    }

    slhelp["key"] = LibHelp{in: "ary_name,key_name", out: "bool", action: "Does key [#i1]key_name[#i0] exist in associative array [#i1]ary_name[#i0]?"}
    stdlib["key"] = func(evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("key",args,1,"2","string","string"); !ok { return nil,err }

        var v any
        var found bool

        if v, found = vget(nil,evalfs,ident, args[0].(string)); !found {
            return false, nil
        }

        key:=interpolate(evalfs,ident,args[1].(string))

        switch v:=v.(type) {
        case http.Header:
            if _, found = v[key]; found { return true, nil }
        case map[string]float64:
            if _, found = v[key]; found { return true, nil }
        case map[string]uint8:
            if _, found = v[key]; found { return true, nil }
        case map[string]uint:
            if _, found = v[key]; found { return true, nil }
        case map[string]uint64:
            if _, found = v[key]; found { return true, nil }
        case map[string]int64:
            if _, found = v[key]; found { return true, nil }
        case map[string]int:
            if _, found = v[key]; found { return true, nil }
        case map[string]bool:
            if _, found = v[key]; found { return true, nil }
        case map[string]string:
            if _, found = v[key]; found { return true, nil }
        case map[string][]string:
            if _, found = v[key]; found { return true, nil }
        case map[string]any:
            if _, found = v[key]; found { return true, nil }
        case map[string][]any:
            if _, found = v[key]; found { return true, nil }
        default:
            return false, errors.New("key() requires a map")
        }
        return false, nil
    }

    slhelp["glob_key"] = LibHelp{in: "ary_name,key_name", out: "bool", action: "Does key [#i1]key_name[#i0] exist in the global associative array [#i1]ary_name[#i0]?"}
    stdlib["glob_key"] = func(evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("glob_key",args,1,"2","string","string"); !ok { return nil,err }

        var v any
        var found bool

        locked:=false
        if atomic.LoadUint32(&has_global_lock)!=evalfs {
            sglock.RLock(); locked=true
        }

        var mloc uint32
        if interactive {
            mloc=1
        } else {
            mloc=2
        }
        if v, found = vget(nil,mloc,&mident,args[0].(string)); !found {
            if locked { sglock.RUnlock() }
            return false, nil
        }
        if locked { sglock.RUnlock() }

        key:=interpolate(evalfs,ident,args[1].(string))

        switch v:=v.(type) {
        case http.Header:
            if _, found = v[key]; found { return true, nil }
        case map[string]float64:
            if _, found = v[key]; found { return true, nil }
        case map[string]uint8:
            if _, found = v[key]; found { return true, nil }
        case map[string]uint:
            if _, found = v[key]; found { return true, nil }
        case map[string]uint64:
            if _, found = v[key]; found { return true, nil }
        case map[string]int64:
            if _, found = v[key]; found { return true, nil }
        case map[string]int:
            if _, found = v[key]; found { return true, nil }
        case map[string]bool:
            if _, found = v[key]; found { return true, nil }
        case map[string]string:
            if _, found = v[key]; found { return true, nil }
        case map[string][]string:
            if _, found = v[key]; found { return true, nil }
        case map[string]any:
            if _, found = v[key]; found { return true, nil }
        case map[string][]any:
            if _, found = v[key]; found { return true, nil }
        default:
            return false, errors.New("glob_key() requires a map")
        }
        return false, nil
    }

    slhelp["last"] = LibHelp{in: "", out: "int", action: "Returns the last received error code from a co-process command."}
    stdlib["last"] = func(evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("last",args,0); !ok { return nil,err }
        v, found := gvget("@last")
        if found {
            i:=v.(int)
            return i, nil
        }
        return -1,errors.New("no co-process command has been executed yet.")
    }

    slhelp["execpath"] = LibHelp{in: "", out: "string", action: "Returns the initial working directory."}
    stdlib["execpath"] = func(evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("execpath",args,0); !ok { return nil,err }
        v, _ := gvget("@execpath")
        return string(v.(string)), err
    }

    slhelp["last_out"] = LibHelp{in: "", out: "string", action: "Returns the last received error text from the co-process."}
    stdlib["last_out"] = func(evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("last_out",args,0); !ok { return nil,err }
        v, found := gvget("@last_out")
        if found {
            return v.(string), err
        }
        return "",errors.New("No co-process error has been detected yet.")
    }

    slhelp["zsh_version"] = LibHelp{in: "", out: "string", action: "Returns the zsh version string if present."}
    stdlib["zsh_version"] = func(evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("zsh_version",args,0); !ok { return nil,err }
        v, found := gvget("@zsh_version")
        if !found { v="" }
        return v.(string), err
    }

    slhelp["bash_version"] = LibHelp{in: "", out: "string", action: "Returns the full release string of the Bash co-process."}
    stdlib["bash_version"] = func(evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("bash_version",args,0); !ok { return nil,err }
        v, _ := gvget("@bash_version")
        return v.(string), err
    }

    slhelp["bash_versinfo"] = LibHelp{in: "", out: "string", action: "Returns the major version number of the Bash co-process."}
    stdlib["bash_versinfo"] = func(evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("bash_versinfo",args,0); !ok { return nil,err }
        v, _ := gvget("@bash_versinfo")
        return v.(string), err
    }

    slhelp["keypress"] = LibHelp{in: "[timeout_ms]", out: "int", action: "Returns an integer corresponding with a keypress.\n"+
        "Internally, the minimum timeout value is currently 1 decisecond.\n"+
        "See the termios(3) man page for reasoning about VMIN/VTIME."}
    stdlib["keypress"] = func(evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("keypress",args,3,
        "2","int","bool",
        "1","int",
        "0"); !ok { return nil,err }
        timeo := int64(0)
        if len(args) > 0 {
            switch args[0].(type) {
            case string, int:
                ttmp, terr := GetAsInt(args[0])
                timeo = int64(ttmp)
                if terr { return "", errors.New("Invalid timeout value.") }
            }
        }

        disp:=false
        if len(args)>1 { disp=args[1].(bool) }

        k:=wrappedGetCh(int(timeo),disp)

        if k==3 { // ctrl-c 
            lastlock.RLock()
            sig_int=true
            lastlock.RUnlock()
        }

        return k,nil
    }

    slhelp["cursoroff"] = LibHelp{in: "", out: "", action: "Disables cursor display."}
    stdlib["cursoroff"] = func(evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("cursoroff",args,0); !ok { return nil,err }
        hideCursor()
        return nil, nil
    }

    slhelp["cursorx"] = LibHelp{in: "n", out: "", action: "Moves cursor to horizontal position [#i1]n[#i0]."}
    stdlib["cursorx"] = func(evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("cursorx",args,1,"1","int"); !ok { return nil,err }
        cursorX(args[0].(int))
        return nil, nil
    }

    slhelp["cursoron"] = LibHelp{in: "", out: "", action: "Enables cursor display."}
    stdlib["cursoron"] = func(evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("cursoron",args,0); !ok { return nil,err }
        showCursor()
        return nil, nil
    }

    slhelp["ppid"] = LibHelp{in: "", out: "int", action: "Return the pid of parent process."}
    stdlib["ppid"] = func(evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("ppid",args,0); !ok { return nil,err }
        return os.Getppid(), nil
    }

    slhelp["pid"] = LibHelp{in: "", out: "int", action: "Return the pid of the current process."}
    stdlib["pid"] = func(evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("pid",args,0); !ok { return nil,err }
        return os.Getpid(), nil
    }

    slhelp["clear_line"] = LibHelp{in: "row,col", out: "", action: "Clear to the end of the line, starting at row,col in the current pane."}
    stdlib["clear_line"] = func(evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
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
    stdlib["user"] = func(evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("user",args,0); !ok { return nil,err }
        v, _ := gvget("@user")
        return v.(string), err
    }

    slhelp["os"] = LibHelp{in: "", out: "string", action: "Returns the kernel version name as reported by the coprocess."}
    stdlib["os"] = func(evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("os",args,0); !ok { return nil,err }
        v, _ := gvget("@os")
        return v.(string), err
    }

    slhelp["home"] = LibHelp{in: "", out: "string", action: "Returns the home directory of the user that launched Za as reported by the coprocess."}
    stdlib["home"] = func(evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("home",args,0); !ok { return nil,err }
        v, _ := gvget("@home")
        return v.(string), err
    }

    slhelp["lang"] = LibHelp{in: "", out: "string", action: "Returns the locale name used within the coprocess."}
    stdlib["lang"] = func(evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("lang",args,0); !ok { return nil,err }
        if v, found := gvget("@lang"); found {
            return v.(string), nil
        }
        return "",nil
    }

    slhelp["release_name"] = LibHelp{in: "", out: "string", action: "Returns the OS release name as reported by the coprocess."}
    stdlib["release_name"] = func(evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("release_name",args,0); !ok { return nil,err }
        v, _ := gvget("@release_name")
        return v.(string), err
    }

    slhelp["hostname"] = LibHelp{in: "", out: "string", action: "Returns the current hostname."}
    stdlib["hostname"] = func(evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("hostname",args,0); !ok { return nil,err }
        z, _ := os.Hostname()
        gvset("@hostname", z)
        return z, err
    }

    slhelp["tokens"] = LibHelp{in: "string", out: "struct", action: "Returns a structure containing a list of tokens ([#i1].tokens[#i0]) in a string and a list ([#i1].types[#i0]) of token types."}
    stdlib["tokens"] = func(evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("tokens",args,1,"1","string"); !ok { return nil,err }
        var toks []string
        var toktypes []string
        var cl int16
        for p := 0; p < len(args[0].(string)); {
            t := nextToken(args[0].(string), evalfs, &cl, p)
            if t.tokPos != -1 {
                p = t.tokPos
            }
            toks = append(toks, t.carton.tokText)
            toktypes = append(toktypes, tokNames[t.carton.tokType])
            if t.eof || t.eol {
                break
            }
        }
        return token_result{tokens:toks,types:toktypes}, err
    }

    slhelp["release_version"] = LibHelp{in: "", out: "string", action: "Returns the OS version number."}
    stdlib["release_version"] = func(evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("release_version",args,0); !ok { return nil,err }
        v, _ := gvget("@release_version")
        return v.(string), err
    }

    slhelp["release_id"] = LibHelp{in: "", out: "string", action: "Returns the /etc derived release name."}
    stdlib["release_id"] = func(evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("release_id",args,0); !ok { return nil,err }
        v, _ := gvget("@release_id")
        return v.(string), err
    }

    slhelp["winterm"] = LibHelp{in: "", out: "bool", action: "Is this a WSL terminal?"}
    stdlib["winterm"] = func(evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("winterm",args,0); !ok { return nil,err }
        v, _ := gvget("@winterm")
        return v.(bool), err
    }

    slhelp["func_inputs"] = LibHelp{in: "", out: "[]string", action: "Returns a list of standard library function inputs."}
    stdlib["func_inputs"] = func(evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("func_inputs",args,0); !ok { return nil,err }
        var fm = make(map[string]string)
        for k,i:=range slhelp {
            fm[k]=i.in
        }
        return fm,nil
    }

    slhelp["func_outputs"] = LibHelp{in: "", out: "[]string", action: "Returns a list of standard library function outputs."}
    stdlib["func_outputs"] = func(evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("func_outputs",args,0); !ok { return nil,err }
        var fm = make(map[string]string)
        for k,i:=range slhelp {
            fm[k]=i.out
        }
        return fm,nil
    }

    slhelp["func_descriptions"] = LibHelp{in: "", out: "[]string", action: "Returns a list of standard library function descriptions."}
    stdlib["func_descriptions"] = func(evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("func_descriptions",args,0); !ok { return nil,err }
        var fm = make(map[string]string)
        for k,i:=range slhelp {
            fm[k]=i.action
        }
        return fm,nil
    }

    slhelp["func_categories"] = LibHelp{in: "", out: "[]string", action: "Returns a list of standard library functions."}
    stdlib["func_categories"] = func(evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("func_categories",args,0); !ok { return nil,err }
        return categories,nil
    }

    slhelp["funcs"] = LibHelp{in: "[partial_match[,bool_return]]", out: "string", action: "Returns a list of standard library functions."}
    stdlib["funcs"] = func(evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
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
                        s_inset,_:=stdlib["inset"](evalfs,ident,sparkle(slhelp[q].action),8)
                        matchList += sf(sparkle("\n  [#6]Function : [#"+colour+"]%s%s(%s)[#-]\n"), lhs, q, params)
                        matchList += sf(sparkle("[#7]%s[#-]\n"),s_inset)
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

    slhelp["ast"] = LibHelp{in: "fn_name", out: "string", action: "Return AST representation."}
    stdlib["ast"] = func(evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("ast",args,1,"1","string"); !ok { return nil,err }
        fname:=args[0].(string)
        if fname=="" { return "",nil }
            _,found:=fnlookup.lmget(fname)
            if found {
                return GetAst(fname),nil
            } else {
                return "",nil
            }
    }

    slhelp["has_term"] = LibHelp{in: "", out: "bool", action: "Check if executing with a tty."}
    stdlib["has_term"] = func(evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("has_term",args,0); !ok { return false,err }
        return isatty(), nil
    }

    slhelp["has_colour"] = LibHelp{in: "", out: "bool", action: "Check if tty supports at least 16 colours."}
    stdlib["has_colour"] = func(evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("has_colour",args,0); !ok { return false,err }
        term:=os.Getenv("TERM")
        cterms:=regexp.MustCompile("(?i)^xterm|^vt100|^vt220|^rxvt|^screen|color|ansi|cygwin|linux")
        return ansiMode && cterms.MatchString(term),nil
    }

    slhelp["has_shell"] = LibHelp{in: "", out: "bool", action: "Check if a child co-process has been launched."}
    stdlib["has_shell"] = func(evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("has_shell",args,0); !ok { return nil,err }
        v, _ := gvget("@noshell")
        return !v.(bool), nil
    }

    slhelp["shell_pid"] = LibHelp{in: "", out: "int", action: "Get process ID of the launched child co-process."}
    stdlib["shell_pid"] = func(evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("shell_pid",args,0); !ok { return nil,err }
        v, _ := gvget("@shell_pid")
        return v, nil
    }

    slhelp["clktck"] = LibHelp{in: "", out: "int", action: "Get clock ticks from aux file."}
    stdlib["clktck"] = func(evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
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

