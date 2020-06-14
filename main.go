package main

//
// IMPORTS
//

import (
    "flag"
    "fmt"
    "path/filepath"
    "io"
    "io/ioutil"
    "os"
    "os/exec"
    "os/signal"
    "runtime"
    str "strings"
    "syscall"
    "time"
)

// for profiling:
import (
    "log"
    "net/http"
    _ "net/http/pprof"
    "runtime/trace"
)

//
// ALIASES
//

var sf = fmt.Sprintf
var pln = fmt.Println
var fpf = fmt.Fprintln
var fef = fmt.Errorf
//
// CONSTS AND GLOBALS
//

// connections
var MAX_TIO time.Duration = 120000 // two minutes

// build-time

var BuildComment string
var BuildVersion string
var BuildDate string

// safety

var lockSafety bool=false       // enable mutices in variable handling functions, for multi-threading.

// run-time

var calltable = make([]call_s,CALL_CAP)             // open function calls
var panes = make(map[string]Pane)                   // defined console panes.
var features = make(map[string]Feature)             // list of stdlib categories.
var orow, ocol, ow, oh int                          // console cursor location and terminal dimensions.

var functionspaces = make([][]Phrase, SPACE_CAP)    // tokenised function storage (key: function name)
var functionArgs = make([][]string, SPACE_CAP)      // expected parameters for each defined function (key: function name)
var loops = make([][]s_loop, LOOP_START_CAP)        // counters per function per loop type (keys: function, keyword-token id)
var depth = make([]int, SPACE_CAP)                  // generic nesting indentation counters (key: function id)
var fairydust = make(map[string]string, FAIRY_CAP)  // ANSI colour code mappings (key: colour alias)
var lastConstruct = make([][]int, SPACE_CAP)        // stores the active construct/loop types outer->inner for the break command
var wc = make([]whenCarton, SPACE_CAP)              // active WHEN..ENDWHEN statements
var wccount = make([]int, SPACE_CAP)                // count of active WHEN..ENDWHEN statements per function.

var globalaccess uint64                             // number of functionspace which is considered to be "global"

var varcount = make([]int, SPACE_CAP)                 // how many local variables are declared in each active function.
// var vmap = make(map[uint64]map[string]int, SPACE_CAP) // test lookup make for var names


// variable storage per function (indices: function space id for locality , table offset. offset calculated by VarLookup)
var ident = make([][]Variable, SPACE_CAP)

// lookup tables for converting between function name and functionspaces[] index.
var fnlookup = lmcreate(SPACE_CAP)
var numlookup = nlmcreate(SPACE_CAP)

// interactive mode and prompt handling
var interactive bool                 // interactive mode flag
var promptTemplate string

// storage for the standard library functions
var stdlib = make(map[string]ExpressionFunction, FUNC_CAP)

// firstInstallRun is used by the package management library calls for flagging an "update".
var firstInstallRun bool = true

// mysql connection variables - these should really be in the library (and done differently!)
var dbhost string   //
var dbengine string // these would normally be provided in ZA_DB_* environmental variables
var dbport int      // and be initialised during db_init().
var dbuser string   //
var dbpass string   //

// not thread-safe: used during debug
// var high_q uint64
var elast int                                       // mainly for debugging eval routine. should only be used when locks are 
                                                    //  disabled. it contains the last line number executed.


//
// MAIN
//

var eval *Evaluator         // declaration for math evaluator

var bgproc *exec.Cmd        // holder for the coprocess
var pi io.WriteCloser       // process in, out and error streams
var po io.ReadCloser
var pe io.ReadCloser
var row, col int            // for pane + terminal use
var MW, MH int              // for pane + terminal use
var currentpane string      // for pane use

var cmdargs []string        // cli args

var no_interpolation bool   // true: disable string interpolation.

var ansiMode bool           // defaults to true. false disables ansi colour code output

var testMode bool           // is TEST..ENDTEST functionality enabled this run? (this may change later for alternative run types)
var docGen bool             // is DOC processing enabled this run? (now deprecated?)

// setup getInput() history for interactive mode
var curHist int
var lastHist int
var hist []string // =make([]string)
var histEmpty bool

// setup logging
var logFile string
var loggingEnabled bool
var log_web bool
var web_log_file string = "/var/log/za_access.log"

// trap handling (@note: this needs extending to handle user defined traps)
var sig_int bool       // ctrl-c pressed?
var coproc_active bool // for resetting co-proc if interrupted

// test related setup
var under_test bool
var test_group string
var test_name string
var test_assert string
var test_group_filter string
var fail_override string
// var inside_test bool
var test_output_file string
var testsPassed int
var testsFailed int
var testsTotal int


// for disabling the coprocess entirely:
var no_shell bool

// pane resize indicator
var winching bool

// 0:off, >0 max displayed level (not currently used too much. maybe eventually be removed).
var debug_level int

// 0:off, >0 line number to emit (used when producing source line debug output)
// var slmon, elmon int

// list of keywords for lookups (used in interactive mode TAB completion)
var keywordset map[string]struct{}

// highest numbered vtable entry created
var vtable_maxreached uint64

func main() {

    runtime.GOMAXPROCS(runtime.NumCPU())

    // setup winch handler receive channel to indicate a refresh is required, then check it in Call() before enact().
    sigs := make(chan os.Signal, 1)

    if runtime.GOOS!="windows" {
        setWinchSignal(sigs)
    }

    go func() {
        for {
            <-sigs
            if lockSafety { globlock.Lock() }
            winching = true
            if lockSafety { globlock.Unlock() }
        }
    }()

    var err error // generic error flag

    fnlookup.lmset("global",0)
    fnlookup.lmset("main",1)
    numlookup.lmset(0,"global")
    numlookup.lmset(1,"main")

    /*
    vmap[0]=make(map[string]int,VAR_CAP)
    vmap[1]=make(map[string]int,VAR_CAP)
    */

    calllock.Lock()
    calltable[0] = call_s{} // reset call stacks for global and main
    calltable[1] = call_s{}
    calllock.Unlock()

    farglock.Lock()
    functionArgs[0] = []string{} // initialise empty function argument lists for
    functionArgs[1] = []string{} // global and main, as they cannot be called by user.
    farglock.Unlock()

    // setup the funcs in the standard library. this must come before any use of vset()
    buildStandardLib()

    // create lookup list for keywords - this must come before any use of vset()
    keywordset = make(map[string]struct{})
    for keyword := range completions {
        keywordset[completions[keyword]] = struct{}{}
    }

    // global namespace
    vcreatetable(0, &vtable_maxreached, VAR_CAP)

    // get terminal dimensions
    MW, MH, _ = GetSize(1)

    // turn debug mode off
    debug_level = 0

    // run in parent flag
    vset(0,"@runInParent",false) // if -S opt or /bin/false specified for shell, then run commands in parent

    // set available build info
    vset(0, "@language", "Za")
    vset(0, "@version", BuildVersion)
    vset(0, "@creation_author", "D Horsley")
    vset(0, "@creation_date", BuildDate)

    // set interactive prompt
    vset(0, "@prompt", promptStringStartup)
    vset(0, "@startprompt", promptStringStartup)
    vset(0, "@bashprompt", promptBashlike)

    // set default behaviours
    vset(0, "@silentlog", true)
    vset(0, "mark_time", false)
    vset(0, "@echo", true)
    vset(0, "userSigIntHandler", "")// name of Za function that handles ctrl-c.
    vset(0, "@echomask", "*")

    // set global loop and nesting counters
    loops[0] = make([]s_loop, MAX_LOOPS)
    lastConstruct[globalspace] = []int{}

    // initialise math evaluator for re-use in ev()
    eval = NewEvaluator()

    // read compile time arch info
    vset(0, "@glibc", false)
    if BuildComment == "glibc" {
        vset(0, "@glibc", true)
    }
    vset(0, "@ct_info", BuildComment)

    // arg parsing
    var a_help          = flag.Bool("h", false, "help page")
    var a_version       = flag.Bool("v", false, "display the Za version")
    var a_interactive   = flag.Bool("i", false, "run interactively")
    var a_debug         = flag.Int("d", 0, "set debug level (0:off)")
    var a_profile       = flag.Bool("p", false, "enable profiler")
    var a_trace         = flag.Bool("P", false, "enable trace capture")
    var a_test          = flag.Bool("t", false, "enable tests")
    var a_test_file     = flag.String("o", "za_test.out", "set the test output filename")
    var a_docgen        = flag.Bool("g", false, "enable documentation generator")
    var a_filename      = flag.String("f", "", "input filename, when present. default is stdin")
    var a_program       = flag.String("e", "", "program string")
    var a_program_loop  = flag.Bool("r", false, "wraps a program string in a stdin loop - awk-like")
    var a_program_fs    = flag.String("F", "", "provides a field separator for -r")
    var a_test_override = flag.String("O", "continue", "test override value")
    var a_test_group    = flag.String("G", "", "test group filter")
    var a_time_out      = flag.Int("T", 0, "Co-process command time-out (ms)")
    var a_mark_time     = flag.Bool("m", false, "Mark co-process command progress")
    var a_ansi          = flag.Bool("c", false, "disable colour output")
    var a_ansiForce     = flag.Bool("C", false, "enable colour output")
    var a_lock_safety   = flag.Bool("l", false, "Enable variable mutex locking for multi-threaded use")
    var a_shell         = flag.String("s", "", "path to coprocess shell")
    var a_noshell       = flag.Bool("S", false, "disables the coprocess shell")

    flag.Parse()
    cmdargs = flag.Args() // rest of the cli arguments
    exec_file_name := ""

    // mono flag
    ansiMode=true
    if !*a_ansiForce && (runtime.GOOS=="windows" || *a_ansi) {
        ansiMode = false
    }

    // prepare ANSI colour mappings
    setupAnsiPalette()

    // safety checks
    if *a_lock_safety {
        lockSafety = true
    }

    // check if interactive mode was desired
    if *a_interactive {
        interactive = true
    }

    // filename
    if *a_filename != "" {
        exec_file_name = *a_filename
    } else {
        // try first cmdarg
        if len(cmdargs) > 0 {
            exec_file_name = cmdargs[0]
            if !interactive && *a_program=="" { cmdargs = cmdargs[1:] }
        }
    }

    fpath,_:=filepath.Abs(exec_file_name)
    fdir:=fpath

    f, err := os.Stat(fpath)
    if err == nil {
        if !f.Mode().IsDir() {
            fdir=filepath.Dir(fpath)
        }
    }
    vset(0, "@execpath", fdir)

    // help flag
    if *a_help {
        help("")
        os.Exit(0)
    }

    // version flag
    if *a_version {
        version()
        os.Exit(0)
    }

    // max timeout
    if *a_time_out != 0 {
        MAX_TIO = time.Duration(*a_time_out)
    }

    if *a_mark_time {
        vset(0, "mark_time", true)
    }

    // debug flag
    if *a_debug != 0 {
        debug_level = *a_debug
    }

    // trace capture
    if *a_trace {
        tf, err := os.Create("trace.out")
        if err != nil {
            panic(err)
        }
        err = trace.Start(tf)
        if err != nil {
            os.Exit(126)
        }
        defer func() {
            trace.Stop()
            tf.Close()
        }()
    }

    // pprof
    if *a_profile {
        go func() {
            log.Fatalln(http.ListenAndServe("localhost:6060", http.DefaultServeMux))
        }()
    }

    // test mode
    if *a_test {
        testMode = true
    }

    if *a_test_override != "" {
        fail_override = *a_test_override
    }

    test_output_file = *a_test_file
    _ = os.Remove(test_output_file)

    test_group_filter = *a_test_group

    // disable the coprocess command
    if *a_noshell!=false {
        no_shell=true
    }
    vset(0, "@noshell",no_shell)

    // set the coprocess command
    default_shell:=""
    if *a_shell!="" {
        default_shell=*a_shell
    }

    // doc gen (deprecated)
    if *a_docgen {
        docGen = true
    }


    // Primary activity below

    var data []byte // input buffering

    // start shell in co-process

    coprocLoc:=""

    if runtime.GOOS!="windows" {

        if default_shell=="" {
            coprocLoc, err = GetCommand("/usr/bin/which bash")
            if err == nil {
                coprocLoc = coprocLoc[:len(coprocLoc)-1]
            } else {
                if fexists("/bin/bash") {
                    coprocLoc="/bin/bash"
                } else {
                    // try for /bin/sh then default to noshell
                    if fexists("/bin/sh") {
                        coprocLoc="/bin/sh"
                    } else {
                        vset(0,"@noshell",true)
                        vset(0, "@noshell",no_shell)
                        coprocLoc="/bin/false"
                    }
                    // pf("Error: could not locate a Bash shell.\n")
                    // pf("Error content:\n%v\n",err)
                    // os.Exit(ERR_NOBASH)
                }
            }
        } else {
            if !fexists(default_shell) {
                pf("The chosen shell (%v) does not exist.\n",default_shell)
                os.Exit(ERR_NOBASH)
            }
            coprocLoc=default_shell
        }

    } else {
        coprocLoc="C:/Windows/System32/cmd.exe"
        // should figure out how to populate these properly:
        vset(0,"@noshell",true)
        vset(0,"@os","windows")
        vset(0, "@zsh_version", "")
        vset(0, "@bash_version", "")
        vset(0, "@bash_versinfo", "")
        vset(0, "@user", "")
        vset(0, "@home", "")
        vset(0, "@lang", "")
        vset(0, "@wsl", "")
        vset(0, "@release_name", "")
        vset(0, "@release_version", "")
        vset(0, "@winterm", false)
    }

    vset(0, "@shell_location", coprocLoc)

    if runtime.GOOS=="windows" || no_shell || coprocLoc=="/bin/false" {
        vset(0,"@runInParent",true)
    }

    // spawn a bash co-process
    if runtime.GOOS!="windows" {
        bgproc, pi, po, pe = NewCoprocess(coprocLoc)
        vset(0, "@shellpid",bgproc.Process.Pid)
    }

    // ctrl-c handler
    breaksig := make(chan os.Signal, 1)
    signal.Notify(breaksig, syscall.SIGINT)

    go func() {
        for {
            <-breaksig

            lastlock.RLock()
            caval:=coproc_active
            lastlock.RUnlock()

            if caval {
                // out with the old
                if bgproc != nil {
                    pid := bgproc.Process.Pid
                    debug(13, "\nkilling pid %v\n", pid)
                    // drain io before killing the process:
                    pi.Close()
                    // now kill:
                    bgproc.Process.Kill()
                    bgproc.Process.Release()
                }
                // in with the new
                bgproc, pi, po, pe = NewCoprocess(coprocLoc)
                debug(13, "\nnew pid %v\n", bgproc.Process.Pid)
                vset(0, "@shellpid",bgproc.Process.Pid)
                siglock.Lock()
                coproc_active = false
                siglock.Unlock()
            }

            // user-trap handling

            userSigIntHandler,usihfound:=vget(globalaccess,"userSigIntHandler")
            usih:=""
            if usihfound { usih=userSigIntHandler.(string) }

            if usih!="" {

                argString:=""
                if brackPos:=str.IndexByte(usih,'('); brackPos!=-1 {
                    argString=usih[brackPos:]
                    usih=usih[:brackPos]
                }

                // calc arguments from string

                var iargs []interface{}
                if argString!="" {
                    argString = stripOuter(argString, '(')
                    argString = stripOuter(argString, ')')

                    // evaluate args
                    var argnames []string

                    // populate inbound parameters to the za function call, with evaluated versions of each.
                    if argString != "" {
                        argnames = str.Split(argString, ",")
                        for k, a := range argnames {
                            aval, ef, err := ev(globalaccess, a, false,true)
                            if ef || err != nil {
                                pf("Error: problem evaluating '%s' in function call arguments. (fs=%v,err=%v)\n", argnames[k], globalaccess, err)
                                finish(false, ERR_EVAL)
                                break
                            }
                            iargs = append(iargs, aval)
                        }
                    }
                }

                // build call

                // vunset(globalaccess,"@temp")
                loc,id := GetNextFnSpace(usih+"@")
                lmv,_:=fnlookup.lmget(usih)
                calllock.Lock()
                calltable[loc] = call_s{fs: id, base: lmv, caller: globalaccess, retvar: "@temp"}
                calllock.Unlock()

                // execute call
                Call(MODE_NEW, loc, iargs...)

                if _, ok := VarLookup(globalaccess, "@temp"); ok {
                    sigintreturn,_ := vget(globalaccess, "@temp")
                    switch sigintreturn.(type) {
                    case int:
                    default:
                        // pf("User interrupt handler must return an int or nothing!\n")
                        // finish(true,124)
                    }
                    if sigintreturn.(int)!=0 {
                        finish(true,sigintreturn.(int))
                    }
                }
            } else {
                finish(false, 0)
                pf("\n[#2]User Interrupt![#-] ")
                if !interactive { pf("\n") }
            }
        }
    }()

    var cop string

    // @note:
    // some explanation is required here...
    // there are two "global" concepts here. First, there is an internal global space, which is used for storing
    // run-time state that may be needed by the standard library or the language itself.
    // This global is always at index 0.
    // Second, there is a user global. This one can potentially float around. It represents where global variables 
    // are stored by a running za program. It will generally be at index 1 or 2.
    // Where it is depends on if we are in interactive mode or not. 
    // This is not very elegant, but then, nothing about this whole thing is! :)
    // globals with a '@' sign are considered as nominally constant. The @ sign is not available to users in identifiers.
    // however, the standard library functions may modify their values if needed.

    // @note:
    //  all of these Copper() calls need reworking to use internal info where 
    //  possible. also need to get rid of this grep/cut dependency.

    // static globals from bash
    if runtime.GOOS!="windows" {

        cop, _ = Copper("echo -n $ZSH_VERSION", true)
        vset(0, "@zsh_version", cop)
        cop, _ = Copper("echo -n $BASH_VERSION", true)
        vset(0, "@bash_version", cop)
        cop, _ = Copper("echo -n $BASH_VERSINFO", true)
        vset(0, "@bash_versinfo", cop)
        cop, _ = Copper("echo -n $USER", true)
        vset(0, "@user", cop)
        cop, _ = Copper("echo -n $OSTYPE", true)
        vset(0, "@os", cop)
        cop, _ = Copper("echo -n $HOME", true)
        vset(0, "@home", cop)
        cop, _ = Copper("echo -n $LANG", true)
        vset(0, "@lang", cop)
        cop, _ = Copper("echo -n $WSL_DISTRO_NAME", true)
        vset(0, "@wsl", cop)

        tmp, _ := Copper("cat /etc/*-release | grep '^NAME=' | cut -d= -f2", true) // e.g. "Debian GNU/Linux"
        vset(0, "@release_name", stripOuterQuotes(tmp, 1))

        tmp, _ = Copper("cat /etc/*-release | grep '^VERSION_ID=' | cut -d= -f2", true) // e.g. "9"
        vset(0, "@release_version", stripOuterQuotes(tmp, 1))

        // special cases for release version:

        // case 1: centos/other non-semantic expansion
        vtmp, _ := vget(0, "@release_version")
        if !str.ContainsAny(vtmp.(string), ".") {
            vtmp = vtmp.(string) + ".0"
        }
        vset(0, "@release_version", vtmp)

        tmp, _ = Copper("cat /etc/*-release | grep '^ID=' | cut -d= -f2", true) // e.g. "debian"

        // special cases for release id:

        // case 1: opensuse
        tmp = stripOuterQuotes(tmp, 1)
        if str.HasPrefix(tmp, "opensuse-") {
            tmp = "opensuse"
        }

        // case 2: ubuntu under wsl
        vset(0, "@winterm", false)
        wsl, _ := vget(0, "@wsl")
        if str.HasPrefix(wsl.(string), "Ubuntu-") {
            vset(0, "@winterm", true)
            tmp = "ubuntu"
        }

        vset(0, "@release_id", tmp)


        // further globals from bash
        // cop, _ = Copper("hostname", true)
        h, _ := os.Hostname()
        vset(0, "@hostname", h)

    } // if not windows


    if testMode {
        testStart(exec_file_name)
        defer testExit()
    }


    // reset counters:
    depth[globalspace] = 0

    promptTemplate = promptStringStartup
    panes["global"] = Pane{row: 0, col: 0, w: MW + 1, h: MH}
    currentpane = "global"
    orow = 0
    ocol = 0
    ow = MW + 1
    oh = MH // for resetting the terminal to global pane


    // reset logging
    logFile = ""
    loggingEnabled = false


    // interactive mode support
    if interactive {

        // reset terminal
        cls()

        // banner
        title := sparkle(sf("Za Interactive Mode  -  (%v,%v)  ", MH, MW))
        pf("%s\n\n", sparkle("[#bblue][#7]"+pad(title, -1, MW, "·")+"[#-][##]"))

        // state control
        endFunc := false
        // breakOut:=Error

        curHist = 0
        lastHist = 0
        histEmpty = true

        // term loop
        pf("\033[s") // save cursor
        row = 3
        col = 1
        at(row, col)
        pcol := promptColour

        // simple, inelegant, probably buggy REPL
        for {

            sig_int = false
            fspacelock.Lock()
            functionspaces[globalspace] = []Phrase{}
            fspacelock.Unlock()

            pr, _ := vget(0, "@prompt")
            sparklePrompt := sparkle(pr.(string))
            echoMask,_:=vget(0,"@echomask")
            input, eof, broken := getInput(globalspace, sparklePrompt, "global", row, col, pcol, true, true, echoMask.(string))
            if eof || broken {
                break
            }

            cr,_:=GetCursorPos()
            row=cr+1
            if row>MH { row=MH ; pf("\n") }

            // row:=row+(len(input)/MW)
            col = 1
            at(row, col)

            if input == "\n" {
                continue
            }
            input += "\n"

            parse("global", input, 0)

            // throw away break and continue positions in interactive mode
            endFunc = Call(MODE_STATIC, globalspace)
            if endFunc {
                break
            }

            inter,_:=interpolate(globalspace, promptTemplate,true)
            vset(0, "@prompt", inter)

        }
        pln("")

        finish(true, 0)
    }

    row,col=GetCursorPos()
    if runtime.GOOS=="windows" { row++ ; col++ }

    // function spaces:
    //
    // global space is named 'global'
    // main (entry point) function space is named 'main'
    // defined functions are named according to their function name

    // if not in interactive mode, then get input from either file or stdin:
    if *a_program=="" {
        if exec_file_name != "" && exec_file_name != "-" {
            ok := false
            f, err := os.Stat(exec_file_name)
            if err == nil {
                if f.Mode().IsRegular() {
                    ok = true
                }
            }
            if ok {
                data, err = ioutil.ReadFile(exec_file_name)
            } else {
                pf("Error: source file not found.\n")
                os.Exit(ERR_EXISTS)
            }
        } else {
            data, err = ioutil.ReadAll(os.Stdin)
            if err != nil {
                panic(err)
            }
        }
    }

    if *a_program_loop==true {
        s:= `NL=0` + "\n" +
            `foreach _line in read_file("/dev/stdin")` + "\n" +
            `inc NL` + "\n"
        if *a_program_fs=="" {
            s+=`fields(_line); `
        } else {
            s+=`fields(_line,"`+*a_program_fs+`"); `
        }
        s += "\n" + *a_program + "\nendfor\n"
        *a_program=s
    }


    // source the program
    var input string
    if *a_program!="" {
        input=*a_program+"\n"
    } else {
        input=string(data)
    }

    // tokenise and parse the input
    if len(input) > 0 {
        parse("main", input, 0)

        // initialise the main program
        cs := call_s{}
        cs.base = 1
        cs.fs = "main"
        cs.caller = 0

        mainloc,_ := GetNextFnSpace("main")
        calllock.Lock()
        calltable[mainloc] = cs
        calllock.Unlock()
        Call(MODE_NEW, mainloc)
    }

    // a little paranoia to finish things off...
    setEcho(true)

}

