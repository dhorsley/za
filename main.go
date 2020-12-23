package main

//
// IMPORTS
//

import (
    "flag"
    "fmt"
    "path"
    "path/filepath"
    term "github.com/pkg/term"
    "io"
    "io/ioutil"
    "os"
    "os/exec"
    "os/signal"
    "regexp"
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

// co-proc connection timeout, in milli-seconds
var MAX_TIO time.Duration = 120000

// build-time constants made available at run-time
var BuildComment string
var BuildVersion string
var BuildDate string

// thread-safety

// @deprecated: leaving in place as not removed locks() func yet
// enable mutices in variable handling functions
var lockSafety bool=false


// initialise parser used by the interpolate function
// this (and interlock) in ev() prevent recursive interpolation from working
// @todo: need to see if there's a work around for this.
var interparse *leparser

// run-time declarations

// open function calls
var calltable = make([]call_s,CALL_CAP)

// defined console panes.
var panes = make(map[string]Pane)

// list of stdlib categories.
var features = make(map[string]Feature)

// console cursor location and terminal dimensions.
var orow, ocol, ow, oh int

// for converting vmap id to fn id (for debugging/errors)
var vmap    = make([]map[string]uint16,MAX_FUNCS)

// for converting fn id to vmap id (for debugging/errors)
var unvmap  = make([]map[uint16]string,MAX_FUNCS)

// flag to indicate if source vars have been processed once
var identParsed = make([]bool,MAX_FUNCS)

// func space to source file name mappings
var fileMap   = make(map[uint32]string)

// id of func space which points to the source which contains
// the DEFINE..ENDDEF for a defined function
var sourceMap = make(map[uint32]uint32)

// tokenised function storage
// this is where all the translated source ends up
var functionspaces = make([][]Phrase, SPACE_CAP)

// expected parameters for each defined function
var functionArgs = make([]fa_s, SPACE_CAP)

// for storing found identifier counts in parsing
var functionidents  [MAX_FUNCS]uint16

// marks pre-processed function spaces
var parsed          [MAX_FUNCS]bool

// counters per function per loop type
var loops = make([][]s_loop, LOOP_START_CAP)

// generic nesting indentation counters
var depth = make([]int, SPACE_CAP)

// ANSI colour code mappings (key: colour alias)
var fairydust = make(map[string]string, FAIRY_CAP)

// stores the active construct/loop types outer->inner
//  for the break and continue statements
var lastConstruct = make([][]uint8, SPACE_CAP)

// number of functionspace which is considered to be "global"
var globalaccess uint32

// basename of module currently being processed.
var currentModule string

// defined function list
var funcmap = make(map[string]Funcdef)

// variable storage per function
//  indices: function space id for locality, table offset.
//  offset calculated by VarLookup
var ident = make([][]Variable, SPACE_CAP)

// lookup tables for converting between function name 
//  and functionspaces[] index.
var fnlookup = lmcreate(SPACE_CAP)
var numlookup = nlmcreate(SPACE_CAP)

// interactive mode and prompt handling
// interactive mode flag
var interactive bool

// interactive mode prompt
var promptTemplate string

// storage for the standard library functions
var stdlib = make(map[string]ExpressionFunction, FUNC_CAP)

// firstInstallRun is used by the package management 
//  library calls for flagging an "update".
var firstInstallRun bool = true

// mysql connection variables 
// - these should really be in the library 
// these would normally be provided in ZA_DB_* environmental
// variables and be initialised during db_init().
var dbhost string
var dbengine string
var dbport int
var dbuser string
var dbpass string

// for debugging eval routines.
// should only be used when locks are disabled.
// it contains the last line number executed.
var elast int


//
// MAIN
//

var bgproc *exec.Cmd        // holder for the coprocess
var pi io.WriteCloser       // process input stream
var po io.ReadCloser        // process output stream
var pe io.ReadCloser        // process error stream

var row, col int            // for pane + terminal use
var MW, MH int              // for pane + terminal use
var currentpane string      // for pane use

var cmdargs []string        // cli args

var no_interpolation bool   // to disable string interpolation
var tt * term.Term          // keystroke input receiver
var ansiMode bool           // to disable ansi colour output

// setup getInput() history for interactive mode
var curHist int
var lastHist int
var hist []string
var histEmpty bool

// setup logging - could use better defaults
var logFile string
var loggingEnabled bool
var log_web bool
var web_log_file string = "/var/log/za_access.log"

// trap handling
var sig_int bool       // ctrl-c pressed?
var coproc_active bool // for resetting co-proc if interrupted

// test related setup, completely non thread safe
var testMode bool
var under_test bool
var test_group string
var test_name string
var test_assert string
var test_group_filter string
var fail_override string
var test_output_file string
var testsPassed int
var testsFailed int
var testsTotal int

// for disabling the coprocess entirely:
var no_shell bool
var shellrep bool

// pane resize indicator
var winching bool

// 0:off, >0 max displayed debug level
// - not currently used too much. may eventually be removed
var debug_level int

// list of keywords for lookups
// - used in interactive mode TAB completion
var keywordset map[string]struct{}

// list of struct fields per struct type
// - used by INIT when defining a struct
var structmaps map[string][]string

// compile cache for regex operator
var ifCompileCache map[string]regexp.Regexp

// highest numbered variable table entry created
var vtable_maxreached uint32


func main() {

    // time zone handling
    if tz := os.Getenv("TZ"); tz != "" {
        var err error
        time.Local, err = time.LoadLocation(tz)
        if err != nil {
            log.Printf("error loading location '%s': %v\n", tz, err)
        }
    }

    // set available CPUs
    runtime.GOMAXPROCS(runtime.NumCPU())

    // setup winch handler receive channel to indicate a refresh
    //  is required, then check it in Call()
    sigs := make(chan os.Signal, 1)

    if runtime.GOOS!="windows" {
        setWinchSignal(sigs)
    }

    go func() {
        for {
            <-sigs
            globlock.Lock()
            winching = true
            globlock.Unlock()
        }
    }()

    // instantiate parser for interpolation
    interparse=&leparser{}

    // generic error flag - used through main
    var err error

    // global forward name resolution map
    vmap[0]=make(map[string]uint16,0)

    // main func forward name resolution map
    vmap[1]=make(map[string]uint16,0)

    // global reverse name resolution map
    unvmap[0]=make(map[uint16]string,0)

    // main func reverse name resolution map
    unvmap[1]=make(map[uint16]string,0)

    // create identifiers for global and main source caches
    fnlookup.lmset("global",0)
    fnlookup.lmset("main",1)
    numlookup.lmset(0,"global")
    numlookup.lmset(1,"main")

    // reset call stacks for global and main
    calllock.Lock()
    calltable[0] = call_s{}
    calltable[1] = call_s{}
    calllock.Unlock()

    // initialise empty function argument lists for
    // global and main, as they cannot be called by user.
    farglock.Lock()
    functionArgs[0].args = []string{}
    functionArgs[1].args = []string{}
    farglock.Unlock()

    // setup the functions in the standard library.
    // - this must come before any use of vset()
    buildStandardLib()

    // create lookup list for keywords
    // - this must come before any use of vset()
    keywordset = make(map[string]struct{})
    for keyword := range completions {
        keywordset[completions[keyword]] = struct{}{}
    }

    // create the structure definition storage area
    structmaps = make(map[string][]string)

    // compile cache for regex operator
    // - usage requires a lock around it
    ifCompileCache = make(map[string]regexp.Regexp)

    // global function space setup
    minvar:=functionidents[0]
    if VAR_CAP>minvar { minvar=VAR_CAP }
    vcreatetable(0, &vtable_maxreached, minvar)


    // get terminal dimensions
    MW, MH, _ = GetSize(1)

    // turn debug mode off
    debug_level = 0

    // start processing startup flags

    // command output unit separator
    vset(0,"@cmdsep",byte(0x1e))

    // run in parent - if -S opt or /bin/false specified
    //  for shell, then run commands in parent
    vset(0,"@runInParent",false)

    // should command output be captured?
    // - when disabled, output is sent to stdout
    vset(0,"@commandCapture",true)

    // like -S, but insist upon it for Windows executions.
    vset(0,"@runInWindowsParent",false)

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

    // - don't echo logging
    vset(0, "@silentlog", true)

    // - don't show co-proc command progress
    vset(0, "mark_time", false)

    // - name of Za function that handles ctrl-c.
    vset(0, "trapInt", "")

    // - show user stdin input
    vset(0, "@echo", true)

    // - set character that can mask user stdin if enabled
    vset(0, "@echomask", "*")


    // set global loop and nesting counters
    loops[0] = make([]s_loop, MAX_LOOPS)
    lastConstruct[globalspace] = []uint8{}


    // read compile time arch info
    vset(0, "@glibc", false)
    if BuildComment == "glibc" {
        vset(0, "@glibc", true)
    }
    vset(0, "@ct_info", BuildComment)

    // arg parsing
    var a_help         =   flag.Bool("h",false,"help page")
    var a_version      =   flag.Bool("v",false,"display the Za version")
    var a_interactive  =   flag.Bool("i",false,"run interactively")
    var a_debug        =    flag.Int("d",0,"set debug level (0:off)")
    var a_profile      =   flag.Bool("p",false,"enable profiler")
    var a_trace        =   flag.Bool("P",false,"enable trace capture")
    var a_test         =   flag.Bool("t",false,"enable tests")
    var a_test_file    = flag.String("o","za_test.out","set the test output filename")
    var a_filename     = flag.String("f","","input filename, when present. default is stdin")
    var a_program      = flag.String("e","","program string")
    var a_program_loop =   flag.Bool("r",false,"wraps a program string in a stdin loop - awk-like")
    var a_program_fs   = flag.String("F","","provides a field separator for -r")
    var a_test_override= flag.String("O","continue","test override value")
    var a_test_group   = flag.String("G","","test group filter")
    var a_time_out     =    flag.Int("T",0,"Co-process command time-out (ms)")
    var a_mark_time    =   flag.Bool("m",false,"Mark co-process command progress")
    var a_ansi         =   flag.Bool("c",false,"disable colour output")
    var a_ansiForce    =   flag.Bool("C",false,"enable colour output")
    var a_lock_safety  =   flag.Bool("l",false,"Enable variable mutex locking for multi-threaded use")
    var a_shell        = flag.String("s","","path to coprocess shell")
    var a_shellrep     =   flag.Bool("Q",false,"enables the shell info reporting")
    var a_noshell      =   flag.Bool("S",false,"disables the coprocess shell")
    var a_cmdsep       =    flag.Int("U",0x1e,"Command output separator byte.")

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

    // thread safety checks
    if *a_lock_safety {
        locks(true)
    }

    // check if interactive mode was desired
    if *a_interactive {
        interactive = true
    }

    // source filename
    if *a_filename != "" {
        exec_file_name = *a_filename
    } else {
        // try first cmdarg
        if len(cmdargs) > 0 {
            exec_file_name = cmdargs[0]
            if !interactive && *a_program=="" { cmdargs = cmdargs[1:] }
        }
    }

    // figure out correct source path and execution path
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

    // command separator
    if *a_cmdsep != 0 {
        vset(0,"@cmdsep",byte(*a_cmdsep))
    }

    // max co-proc command timeout
    if *a_time_out != 0 {
        MAX_TIO = time.Duration(*a_time_out)
    }

    if *a_mark_time {
        vset(0, "mark_time", true)
    }

    if *a_debug != 0 {
        debug_level = *a_debug
    }

    // trace capture - not advertised.
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

    // pprof - not advertised.
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
    if *a_noshell {
        no_shell=true
    }
    vset(0, "@noshell",no_shell)

    if *a_shellrep {
        shellrep=true
    }
    vset(0, "@shell_report",shellrep)

    // set the coprocess command
    default_shell:=""
    if *a_shell!="" {
        default_shell=*a_shell
    }


    //
    // Primary activity below
    //

    var data []byte // input buffering

    // start shell in co-process

    coprocLoc:=""
    var coprocArgs []string

    vset(0,"@shelltype","")

    // figure out the correct shell to use, with available info.
    if runtime.GOOS!="windows" {
        if !no_shell {
            if default_shell=="" {
                coprocLoc, err = GetCommand("/usr/bin/which bash")
                if err == nil {
                    coprocLoc = coprocLoc[:len(coprocLoc)-1]
                    vset(0,"@shelltype","bash")
                } else {
                    if fexists("/bin/bash") {
                        coprocLoc ="/bin/bash"
                        coprocArgs=[]string{"-i"}
                        vset(0,"@shelltype","bash")
                    } else {
                        // try for /bin/sh then default to noshell
                        if fexists("/bin/sh") {
                            coprocLoc="/bin/sh"
                            coprocArgs=[]string{"-i"}
                        } else {
                            vset(0,"@noshell",true)
                            vset(0, "@noshell",no_shell)
                            coprocLoc="/bin/false"
                        }
                    }
                }
            } else { // not default shell
                if !fexists(default_shell) {
                    pf("The chosen shell (%v) does not exist.\n",default_shell)
                    os.Exit(ERR_NOBASH)
                }
                coprocLoc=default_shell
                shellname:=path.Base(coprocLoc)
                // a few common shells require use of external printf
                // for separating output using non-printables.
                // - @todo: we should find a better way than this.
                if shellname=="dash" || shellname=="ash" || shellname=="sh" {
                    // specify that NextCopper() should use external printf
                    // for generating \x1e (or other cmdsep) in output
                    vset(0,"@shelltype",shellname)
                }
            }
        }

    } else {

        // windows run-time. requires that commands are sent
        // individually through cmd.exe.
        // @note: windows is still an afterthought. this will need much
        // improvement if we ever take windows seriously.

        coprocLoc="C:/Windows/System32/cmd.exe"
        vset(0,"@noshell",true)
        vset(0,"@os","windows")
        vset(0, "@zsh_version", "")
        vset(0, "@bash_version", "")
        vset(0, "@bash_versinfo", "")
        vset(0, "@user", "")
        vset(0, "@home", "")
        vset(0, "@lang", "")
        vset(0, "@wsl", "")
        vset(0, "@release_id", "windows")
        vset(0, "@release_name", "windows")
        vset(0, "@release_version", "windows")
        vset(0, "@winterm", false)
        vset(0,"@runInWindowsParent",true)
    }

    shelltype, _ := vget(0, "@shelltype")
    vset(0, "@shell_location", coprocLoc)

    if runtime.GOOS=="windows" || no_shell || coprocLoc=="/bin/false" {
        vset(0,"@runInParent",true)
    }

    if runtime.GOOS!="windows" {

        if !no_shell {
            // create shell process
            bgproc, pi, po, pe = NewCoprocess(coprocLoc,coprocArgs...)
            vset(0, "@shellpid",bgproc.Process.Pid)
        }

        // prepare for getInput() keyboard input (from main process)
        tt, _ = term.Open("/dev/tty")

    }


    // initialise global parser
    parser:=&leparser{}

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
                bgproc, pi, po, pe = NewCoprocess(coprocLoc,coprocArgs...)
                debug(13, "\nnew pid %v\n", bgproc.Process.Pid)
                vset(0, "@shellpid",bgproc.Process.Pid)
                siglock.Lock()
                coproc_active = false
                siglock.Unlock()
            }

            // user-trap handling

            userSigIntHandler,usihfound:=vget(globalaccess,"trapInt")
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

                    // populate inbound parameters to the za function
                    // call, with evaluated versions of each.
                    if argString != "" {
                        argnames = str.Split(argString, ",")
                        for k, a := range argnames {
                            aval, err := ev(parser,globalaccess,a)
                            if err != nil {
                                pf("Error: problem evaluating '%s' in function call arguments. (fs=%v,err=%v)\n", argnames[k], globalaccess, err)
                                finish(false, ERR_EVAL)
                                break
                            }
                            iargs = append(iargs, aval)
                        }
                    }
                }

                // build call

                loc,id := GetNextFnSpace(usih+"@")
                lmv,_:=fnlookup.lmget(usih)
                calllock.Lock()
                currentModule="main"
                calltable[loc] = call_s{
                    fs: id,
                    base: lmv,
                    caller: globalaccess,
                    callline: 0,
                    retvar: "@#",
                }
                calllock.Unlock()

                // execute call
                Call(MODE_NEW, loc, ciTrap, iargs...)

                if _, ok := VarLookup(globalaccess, "@#"); ok {
                    sigintreturn,_ := vget(globalaccess, "@#")
                    switch sigintreturn.(type) {
                    case int:
                    default:
                        finish(true,124)
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

    var cop struct{out string; err string; code int; okay bool}

    // @note:
    // some explanation is required here..

    // There are two "global" concepts here. First, there is an internal
    //  global space, which is used for storing run-time state that may 
    //  be needed by the standard library or the language itself. This global
    //  is always at index 0.

    // Second, there is a user global. This one can potentially float around.
    //  It represents where global variables are stored by a running Za 
    //  program. It will generally be at index 1 or 2. Where it is depends
    //  on if we are in interactive mode or not. 

    // This is not very elegant, but then, nothing about this whole thing is!

    // Globals starting with a '@' sign are considered as nominally constant.
    //  However, the standard library functions may modify their values
    //  if needed.


    // static globals from bash
    if runtime.GOOS!="windows" {

        cop = Copper("echo -n $WSL_DISTRO_NAME", true)
        vset(0, "@wsl", cop.out)

        switch shelltype {
        case "zsh":
            cop = Copper("echo -n $ZSH_VERSION", true)
            vset(0, "@zsh_version", cop.out)
        case "bash":
            cop = Copper("echo -n $BASH_VERSION", true)
            vset(0, "@bash_version", cop.out)
            cop = Copper("echo -n $BASH_VERSINFO", true)
            vset(0, "@bash_versinfo", cop.out)
            cop = Copper("echo -n $LANG", true)
            vset(0, "@lang", cop.out)
        }

        cop = Copper("echo -n $USER", true)
        vset(0, "@user", cop.out)

        vset(0,"@os",runtime.GOOS)

        cop = Copper("echo -n $HOME", true)
        vset(0, "@home", cop.out)

        var tmp string

        vset(0, "@release_name", "unknown")
        vset(0, "@release_version", "unknown")

        // @todo: these ones *really* need re-doing. should not be calling
        // grep/cut however common they are. i was just being lazy. we can
        // do all of the work involved with stuff already available.

        if runtime.GOOS=="linux" {
            cop = Copper("cat /etc/*-release | grep '^NAME=' | cut -d= -f2", true)
            vset(0, "@release_name", stripOuterQuotes(cop.out, 1))
            cop = Copper("cat /etc/*-release | grep '^VERSION_ID=' | cut -d= -f2", true)
            vset(0, "@release_version", stripOuterQuotes(cop.out, 1))
        }

        // special cases for release version:

        // case 1: centos/other non-semantic expansion
        vtmp, _ := vget(0, "@release_version")
        if tr(vtmp.(string),DELETE,"0123456789.","")=="" && !str.ContainsAny(vtmp.(string), ".") {
            vtmp = vtmp.(string) + ".0"
        }
        vset(0, "@release_version", vtmp)

        cop = Copper("cat /etc/*-release | grep '^ID=' | cut -d= -f2", true)
        tmp = stripOuterQuotes(cop.out, 1)

        // special cases for release id:

        // case 1: opensuse
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
        h, _ := os.Hostname()
        vset(0, "@hostname", h)

    } // endif not windows

    // special case: aliases in bash
    if shelltype=="bash" {
        Copper("shopt -s expand_aliases",true)
    }

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

            var echoMask interface{}
            var ok bool

            if echoMask,ok=vget(0,"@echomask"); !ok {
                echoMask=""
            }

            nestAccept:=0
            totalInput:=""
            var eof,broken bool
            var input string

            // multi-line input loop
            for {

                // set the prompt in the loop to ensure it updates regularly
                var tempPrompt string
                if nestAccept==0 {
                    pr, _ := vget(0, "@prompt")
                    sparklePrompt := sparkle(interpolate(globalspace,pr.(string)))
                    tempPrompt=sparklePrompt
                } else {
                    tempPrompt=promptContinuation
                }

                input, eof, broken = getInput(globalspace, tempPrompt, "global", row, col, pcol, true, true, echoMask.(string))
                if eof || broken { break }

                row++
                if row>MH { row=MH ; pf("\n") }
                col = 1
                at(row, col)

                if input == "\n" {
                    break
                }
                input += "\n"

                // collect input
                totalInput+=input

                temptok:=Error
                cl:=0
                breakOnCommand:=false
                tokenIfPresent:=false
                tokenOnPresent:=false
                helpRequest   :=false

                for p := 0; p < len(input);  {
                    t, tokPos, _, _ := nextToken(input, &cl, p, temptok)
                    temptok = t.tokType
                    if tokPos != -1 {
                        p = tokPos
                    }
                    if t.tokType==C_Help  { helpRequest   =true }
                    if t.tokType==C_If    { tokenIfPresent=true }
                    if t.tokType==C_On    { tokenOnPresent=true }

                    // this is hardly fool-proof, but okay for now:
                    if t.tokType==SYM_BOR && (!tokenIfPresent || !tokenOnPresent) { breakOnCommand=true }

                    if !helpRequest {
                        switch t.tokType {
                        // adders
                        case C_Define, C_For, C_Foreach, C_While, C_If, C_When, C_Struct, LParen, LeftSBrace:
                            nestAccept++
                        // ladders
                        case C_Enddef, C_Endfor, C_Endwhile, C_Endif, C_Endwhen, C_Endstruct, RParen, RightSBrace:
                            nestAccept--
                        }
                    }

                }

                if nestAccept<0 { pf("Nesting error.\n") ; break }

                if nestAccept==0 || breakOnCommand { break }

            }

            if eof || broken { break }

            // submit input

            if nestAccept==0 {
                fileMap[globalspace]=exec_file_name
                phraseParse("global", totalInput, 0)
                currentModule="main"

                // throw away break and continue positions in interactive mode
                _,endFunc = Call(MODE_STATIC, globalspace, ciRepl)
                if endFunc {
                    break
                }
            }

            inter:=interpolate(globalspace, promptTemplate)
            vset(0, "@prompt", inter)

        }
        pln("")

        finish(true, 0)
    }

    row,col=GetCursorPos()
    if runtime.GOOS=="windows" { row++ ; col++ }

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

    // awk-like mode
    if *a_program_loop {
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

    // tokenise and part-parse the input
    if len(input) > 0 {
        fileMap[1]=exec_file_name
        phraseParse("main", input, 0)

        // initialise the main program
        cs := call_s{}
        cs.base = 1
        cs.fs = "main"
        cs.caller = 0
        cs.callline = 0

        mainloc,_ := GetNextFnSpace("main")
        calllock.Lock()
        calltable[mainloc] = cs
        calllock.Unlock()
        currentModule="main"
        Call(MODE_NEW, mainloc, ciMain)
    }

    // a little paranoia to finish things off...
    setEcho(true)

    if runtime.GOOS!="windows" {
        term_complete()
    }

}

