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

var sf  = fmt.Sprintf
var pln = fmt.Println
var fpf = fmt.Fprintln
var fef = fmt.Errorf


//
// GLOBALS
//

// build-time constants made available at run-time
var BuildComment string
var BuildVersion string
var BuildDate    string

// global unique name counter
var globseq uint32

// global parser init
var parser     *leparser
var interparse *leparser

// list of stdlib categories.
var features = make(map[string]Feature)

// open function call info
var calltable = make([]call_s,CALL_CAP)

// enum storage
var enum = make(map[string]*enum_s)

// func space to source file name mappings
var fileMap   = make(map[uint32]string)

// id of func space which points to the source which contains
// the DEFINE..ENDDEF for a defined function
var sourceMap = make(map[uint32]uint32)

// tokenised function storage
// this is where all the translated source ends up
var functionspaces = make([][]Phrase, SPACE_CAP)
var basecode       = make([][]BaseCode, SPACE_CAP)
var isSource       = make([]bool, SPACE_CAP)

// expected parameters for each defined function
var functionArgs = make([]fa_s, SPACE_CAP)


// defined console panes.
var panes = make(map[string]Pane)

// console cursor location and terminal dimensions.
var orow, ocol, ow, oh int

// ANSI colour code mappings (key: colour alias)
var fairydust = make(map[string]string, FAIRY_CAP)


// basename of module currently being processed.
var currentModule string

// list of read in modules
var modlist = make(map[string]bool)

// defined function list
var funcmap = make(map[string]Funcdef)

// global variable storage
var gident [szIdent]Variable
var mident [szIdent]Variable

// lookup tables for converting between function name 
//  and functionspaces[] index.
var fnlookup = lmcreate(SPACE_CAP)
var numlookup = nlmcreate(SPACE_CAP)

// tracker for recent function allocations.
var lastfunc = make(map[uint32]string)

// interactive mode and prompt handling flag
var interactive bool
var interactiveFeed bool

// for refactoring: find a var
var var_refs bool
var var_refs_name string

// for refactoring: panic on mixed additions w/strings
var var_warn bool

// storage for the standard library functions
var stdlib = make(map[string]ExpressionFunction, FUNC_CAP)

// firstInstallRun is used by the package management 
//  library calls for flagging an "update".
var firstInstallRun bool = true

// co-proc connection timeout, in milli-seconds
var MAX_TIO time.Duration = 120000

var cmdargs []string        // cli args
var interpolation bool      // false to disable string interpolation

// Global: DB related
// mysql connection variables 
// these would normally be provided in ZA_DB_* environmental
// variables and be initialised during db_init().
var dbhost string
var dbengine string
var dbport int
var dbuser string
var dbpass string

// Global: shell related
var bgproc *exec.Cmd        // holder for the coprocess
var pi io.WriteCloser       // process input stream
var po io.ReadCloser        // process output stream
var pe io.ReadCloser        // process error stream

// Global: console related
var row, col int            // for pane + terminal use
var MW, MH int              // for pane + terminal use
var BMARGIN int             // bottom offset to stop io at
var currentpane string      // for pane use
var tt * term.Term          // keystroke input receiver
var ansiMode bool           // to disable ansi colour output
var lineWrap bool           // optional pane line wrap.
var promptColour string

// Global: setup getInput() history for interactive mode
var curHist int
var lastHist int
var hist []string
var histEmpty bool

// Global: logging related
var logFile string
var loggingEnabled bool
var log_web bool
var web_log_file string = "/var/log/za_access.log"

// Global: generic flags
var sig_int       bool          // ctrl-c pressed?
var coproc_reset  bool          // for resetting locked coproc instances
var coproc_active bool          // 
var no_shell      bool          // disable sub-shell
var shellrep      bool          // enable shell command reporting

// Global: behaviours
var permit_uninit       bool    // default:false, will evaluation cause a run-time failure if it
                                //  encounters an uninitialised variable usage.
                                //  this can be altered with the permit("uninit",bool) call
var permit_dupmod       bool    // default:false, ignore (true) or error (false) when a duplicate
                                //  module import occurs.
var permit_exitquiet    bool    // default:false, squash (true) or display (false) err msg on exit
var permit_shell        bool    // default: true, when false, exit script if shell command encountered
var permit_eval         bool    // default: true, when false, exit script if eval call encountered

// Global: test related
// test related setup, completely non thread safe
var testMode bool
var under_test bool
var test_group string
var test_name string
var test_assert string
var test_group_filter string
var test_name_filter string
var fail_override string
var test_output_file string
var testsPassed int
var testsFailed int
var testsTotal int

// - not currently used too much. may eventually be removed
var debug_level int             // 0:off, >0 max displayed debug level
var lineDebug   bool            // 

// list of keywords for lookups
// - used in interactive mode TAB completion
var keywordset map[string]struct{}

// list of struct fields per struct type
// - used by VAR when defining a struct
var structmaps map[string][]any

// compile cache for regex operator
var ifCompileCache map[string]regexp.Regexp


// repl prompt
var PromptTemplate string

var concurrent_funcs int32

var breaksig chan os.Signal

//
// MAIN
//

// default precedence table that each parser copy receives.
var default_prectable [END_STATEMENTS]int8

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

    // ... which is currently ignored in Windows
    if runtime.GOOS!="windows" {
        setWinchSignal(sigs)
    }

    BMARGIN=8

    permit_shell=true
    permit_eval=true

    go func() {
        for {
            <-sigs
            globlock.Lock()
            MW, MH, _ = GetSize(1)
            globlock.Unlock()
            shelltype, _ := gvget("@shelltype")
            if shelltype=="bash" || shelltype=="ash" {
                if MW!=-1 {
                    if runtime.GOOS=="freebsd" {
                        Copper(sf(`alias ls="COLUMNS=%d ls -C"`,MW),true)
                    } else {
                        Copper(sf(`alias ls="ls -x -w %d"`,MW),true)
                    }
                }
            }
        }
    }()


    default_prectable[EOF]          =-1
    default_prectable[O_Assign]     =5
    default_prectable[O_Map]        =7
    default_prectable[O_Filter]     =9
    default_prectable[SYM_LAND]     =15
    default_prectable[SYM_LOR]      =15
    default_prectable[C_Or]         =15
    default_prectable[SYM_BAND]     =20
    default_prectable[SYM_BOR]      =20
    default_prectable[SYM_Caret]    =20
    default_prectable[SYM_LSHIFT]   =21
    default_prectable[SYM_RSHIFT]   =21
    default_prectable[O_Query]      =23
    // unary not @ 24
    default_prectable[SYM_Tilde]    =25
    default_prectable[SYM_ITilde]   =25
    default_prectable[SYM_FTilde]   =25
    default_prectable[SYM_EQ]       =25
    default_prectable[SYM_NE]       =25
    default_prectable[SYM_LT]       =25
    default_prectable[SYM_GT]       =25
    default_prectable[SYM_LE]       =25
    default_prectable[SYM_GE]       =25
    default_prectable[C_In]         =27
    default_prectable[SYM_RANGE]    =29
    default_prectable[O_Plus]       =31
    default_prectable[O_Minus]      =31
    default_prectable[O_Divide]     =35
    default_prectable[O_Percent]    =35
    default_prectable[O_Multiply]   =35
    default_prectable[O_OutFile]    =37
    // default_prectable[O_Query]      =39
    default_prectable[SYM_POW]      =40
    default_prectable[SYM_PP]       =45
    default_prectable[SYM_MM]       =45
    default_prectable[LeftSBrace]   =45
    default_prectable[SYM_DOT]      =61
    // default_prectable[O_InFile]     =70
    default_prectable[LParen]       =100


    // generic error flag - used through main
    var err error

    // setup empty symbol tables for main
    bindlock.Lock()
    bindings[1]=make(map[string]uint64)
    bindlock.Unlock()

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

    // set this early, in case of interpol calls.
    interpolation=true

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
    structmaps = make(map[string][]any)

    // compile cache for regex operator
    ifCompileCache = make(map[string]regexp.Regexp)

    // get terminal dimensions
    MW, MH, _ = GetSize(1)

    // set default prompt colour
    promptColour=defaultPromptColour

    // turn debug mode off
    debug_level = 0
    lineDebug=false

    // start processing startup flags

    // fall back to command processing if unrecognised.
    gvset("@command_fallback",false)

    // command output unit separator
    gvset("@cmdsep",byte(0x1e))

    // run in parent - if -S opt or /bin/false specified
    //  for shell, then run commands in parent
    gvset("@runInParent",false)

    // should command output be captured?
    // - when disabled, output is sent to stdout
    gvset("@commandCapture",true)

    // like -S, but insist upon it for Windows executions.
    gvset("@runInWindowsParent",false)

    // set available build info
    gvset("@language", "Za")
    gvset("@version", BuildVersion)
    gvset("@creation_author", "D Horsley")
    gvset("@creation_date", BuildDate)

    // set interactive prompt
    gvset("@startprompt", promptStringStartup)
    gvset("@bashprompt", promptBashlike)
    PromptTemplate=promptStringStartup

    // set default behaviours

    // - don't echo logging
    gvset("@silentlog", true)

    // - don't show co-proc command progress
    gvset("mark_time", false)

    // - show user stdin input
    gvset("@echo", true)

    // - set character that can mask user stdin if enabled
    gvset("@echomask", "*")

    // read compile time arch info
    gvset("@glibc", false)
    if BuildComment == "glibc" {
        gvset("@glibc", true)
    }
    gvset("@ct_info", BuildComment)

    // initialise global parser
    parser=&leparser{}

    // interpolation parser
    interparse=&leparser{}

    // arg parsing
    var a_help         =   flag.Bool("h",false,"help page")
    var a_version      =   flag.Bool("v",false,"display the Za version")
    var a_interactive  =   flag.Bool("i",false,"run interactively")
    var a_debug        =    flag.Int("d",0,"set debug level (0:off)")
    var a_lineDebug    =    flag.Bool("D",false,"enable line debug")
    var a_profile      =   flag.Bool("p",false,"enable profiler")
    var a_trace        =   flag.Bool("P",false,"enable trace capture")
    var a_test         =   flag.Bool("t",false,"enable tests")
    var a_test_file    = flag.String("o","za_test.out","set the test output filename")
    var a_filename     = flag.String("f","","input filename, when present. default is stdin")
    var a_program      = flag.String("e","","program string")
    var a_program_loop =   flag.Bool("r",false,"wraps a program string in a stdin loop - awk-like")
    var a_program_fs   = flag.String("F","","provides a field separator for -r")
    var a_test_override= flag.String("O","continue","test override value")
    var a_test_name    = flag.String("N","","test name filter")
    var a_test_group   = flag.String("G","","test group filter")
    var a_time_out     =    flag.Int("T",0,"Co-process command time-out (ms)")
    var a_mark_time    =   flag.Bool("m",false,"Mark co-process command progress")
    var a_ansi         =   flag.Bool("c",false,"disable colour output")
    var a_ansiForce    =   flag.Bool("C",false,"enable colour output")
    var a_shell        = flag.String("s","","path to coprocess shell")
    var a_shellrep     =   flag.Bool("Q",false,"enables the shell info reporting")
    var a_noshell      =   flag.Bool("S",false,"disables the coprocess shell")
    var a_cmdsep       =    flag.Int("U",0x1e,"Command output separator byte")
    var a_var_refs     = flag.String("V","","find all references to a variable")
    var a_var_warn     =   flag.Bool("W",false,"emit errors when addition contains string mixed types")

    flag.Parse()
    cmdargs = flag.Args() // rest of the cli arguments
    exec_file_name := ""

    // mono flag
    ansiMode=true
    if !*a_ansiForce && *a_ansi {
        ansiMode = false
    }

    // prepare DLL calls
    setupDynamicCalls()

    // prepare ANSI colour mappings
    setupAnsiPalette()

    // check if interactive mode was desired
    if *a_interactive {
        interactive = true
    }

    // var refs
    var_refs=false
    if *a_var_refs != "" {
        var_refs=true
        var_refs_name=*a_var_refs
    }

    // type warnings
    var_warn=false
    if *a_var_warn {
        var_warn=true
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
    gvset("@execpath", fdir)

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
        gvset("@cmdsep",byte(*a_cmdsep))
    }

    if *a_debug != 0 {
        debug_level = *a_debug
    }

    if *a_lineDebug {
        lineDebug = *a_lineDebug
    }

    // max co-proc command timeout
    if *a_time_out != 0 {
        MAX_TIO = time.Duration(*a_time_out)
    }

    if *a_mark_time {
        gvset("mark_time", true)
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
    test_name_filter  = *a_test_name

    // disable the coprocess command
    if *a_noshell {
        no_shell=true
    }
    gvset("@noshell",no_shell)

    if *a_shellrep {
        shellrep=true
    }
    gvset("@shell_report",shellrep)

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

    gvset("@shelltype","")

    // figure out the correct shell to use, with available info.
    if runtime.GOOS!="windows" {
        if !no_shell {
            if default_shell=="" {
                coprocLoc, err = GetCommand("/usr/bin/which bash")
                if err == nil {
                    coprocLoc = coprocLoc[:len(coprocLoc)-1]
                    gvset("@shelltype","bash")
                } else {
                    if fexists("/bin/bash") {
                        coprocLoc ="/bin/bash"
                        coprocArgs=[]string{"-i"}
                        gvset("@shelltype","bash")
                    } else {
                        // try for /bin/sh then default to noshell
                        if fexists("/bin/sh") {
                            coprocLoc="/bin/sh"
                            coprocArgs=[]string{"-i"}
                        } else {
                            gvset("@noshell",no_shell)
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
                if shellname=="dash" || shellname=="ash" || shellname=="sh" {
                    // specify that NextCopper() should use external printf
                    // for generating \x1e (or other cmdsep) in output
                    gvset("@shelltype",shellname)
                }
            }
        }

    } else {

        // windows run-time. requires that commands are sent
        // individually through cmd.exe.
        // @note: windows is still an afterthought. this will need much
        // improvement if we ever take windows seriously.

        coprocLoc="C:/Windows/System32/cmd.exe"
        gvset("@noshell",true)
        gvset("@os","windows")
        gvset("@zsh_version", "")
        gvset("@bash_version", "")
        gvset("@bash_versinfo", "")
        gvset("@user", "")
        gvset("@home", "")
        gvset("@lang", "")
        gvset("@wsl", "")
        gvset("@release_id", "windows")
        gvset("@release_name", "windows")
        gvset("@release_version", "windows")
        gvset("@winterm", false)
        gvset("@runInWindowsParent",true)
    }

    shelltype, _ := gvget("@shelltype")
    gvset("@shell_location", coprocLoc)

    if runtime.GOOS=="windows" || no_shell || coprocLoc=="/bin/false" {
        gvset("@runInParent",true)
    }

    if runtime.GOOS!="windows" {

        if !no_shell {
            // create shell process
            bgproc, pi, po, pe = NewCoprocess(coprocLoc,coprocArgs...)
            gvset("@shell_pid",bgproc.Process.Pid)
        }

        // PIG
        // prepare for getInput() keyboard input (from main process)
        tt, _ = term.Open("/dev/tty")

    }


    // ctrl-c handler
    var breaksig = make(chan os.Signal, 1)
    signal.Notify(breaksig,syscall.SIGINT)

    // - name of Za function that handles ctrl-c.
    vset(nil,2,&mident,"trapInt", "")

    go func() {
        for {
            bs := <-breaksig
            // pf("Received signal : [%#v]\n",bs)
            quiet:=false

            if coproc_reset {
                // out with the old
                if bgproc != nil {
                    // pid := bgproc.Process.Pid
                    // pf("\nkilling pid %v\n", pid)
                    // drain io before killing the process:
                    pi.Close()
                    // now kill:
                    bgproc.Process.Kill()
                    bgproc.Process.Release()
                }
                // in with the new
                bgproc, pi, po, pe = NewCoprocess(coprocLoc,coprocArgs...)
                // pf("\nnew pid %v\n", bgproc.Process.Pid)
                gvset("@shell_pid",bgproc.Process.Pid)
                gvset("@last_signal",sf("%v %v",bs,bgproc.Process.Pid))
                lastlock.Lock()
                coproc_active = false
                coproc_reset  = false
                quiet = true
                lastlock.Unlock()
            }

            // user-trap handling
            userSigIntHandler,usihfound:=vget(nil,2,&mident,"trapInt")
            usih:=""
            if usihfound {
                switch userSigIntHandler.(type) {
                case string:
                    usih=userSigIntHandler.(string)
                }
            }

            if usih!="" {

                argString:=""
                if brackPos:=str.IndexByte(usih,'('); brackPos!=-1 {
                    argString=usih[brackPos:]
                    usih=usih[:brackPos]
                }

                // calc arguments from string

                var iargs []any
                if argString!="" {
                    argString = stripOuter(argString, '(')
                    argString = stripOuter(argString, ')')

                    // evaluate args
                    var argnames []string

                    var mloc uint32
                    if interactive {
                        mloc=1
                    } else {
                        mloc=2
                    }

                    // populate inbound parameters to the za function
                    // call, with evaluated versions of each.
                    if argString != "" {
                        argnames = str.Split(argString, ",")
                        for k, a := range argnames {
                            aval, err := ev(interparse,mloc,a)
                            if err != nil {
                                pf("Error: problem evaluating '%s' in function call arguments. (fs=%v,err=%v)\n", argnames[k], mloc, err)
                                finish(false, ERR_EVAL)
                                break
                            }
                            iargs = append(iargs, aval)
                        }
                    }
                }

                // build call

                lmv,_:=fnlookup.lmget(usih)
                loc, _ := GetNextFnSpace(true,usih+"@",call_s{prepared:true,base:lmv,caller:0})

                calllock.Lock()
                currentModule="main"
                calllock.Unlock()

                // execute call

                var trident [szIdent]Variable
                Call(MODE_NEW, &trident, loc, ciTrap, iargs...)
                if calltable[loc].retvals!=nil {
                    sigintreturn := calltable[loc].retvals.([]any)
                    if len(sigintreturn)>0 {
                        switch sigintreturn[0].(type) {
                        case int:
                        default:
                            finish(true,124)
                        }
                        if sigintreturn[0].(int)!=0 {
                            finish(true,sigintreturn[0].(int))
                        }
                    }
                }
                calltable[loc].gcShyness=0
                calltable[loc].gc=false
            } else {
                finish(false, 0)
                if !quiet {
                    pf("[#2]System Interrupt![#-]\n")
                    // if !interactive { pf("\n") }
                } else {
                    startupOptions()
                }
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
    //  program. It should always be at index 1.

    // Globals starting with a '@' sign are considered as nominally constant.
    //  However, the standard library functions may modify their values
    //  if needed.


    // static globals from bash
    if runtime.GOOS!="windows" {

        cop = Copper("echo -n $WSL_DISTRO_NAME", true)
        gvset("@wsl", cop.out)

        switch shelltype {
        case "zsh":
            cop = Copper("echo -n $ZSH_VERSION", true)
            gvset("@zsh_version", cop.out)
        case "bash":
            cop = Copper("echo -n $BASH_VERSION", true)
            gvset("@bash_version", cop.out)
            cop = Copper("echo -n $BASH_VERSINFO", true)
            gvset("@bash_versinfo", cop.out)
            cop = Copper("echo -n $LANG", true)
            gvset("@lang", cop.out)
        }

        cop = Copper("echo -n $USER", true)
        gvset("@user", cop.out)

        gvset("@os",runtime.GOOS)

        cop = Copper("echo -n $HOME", true)
        gvset("@home", cop.out)

        var tmp string

        gvset("@release_name", "unknown")
        gvset("@release_version", "unknown")

        if runtime.GOOS=="linux" {

            cop = Copper("cat /etc/*-release",true)
            s:=lgrep(cop.out,"^NAME=")
            s=lcut(s,2,"=")
            gvset("@release_name", stripOuterQuotes(s,1))

            cop = Copper("cat /etc/*-release",true)
            s=lgrep(cop.out,"^VERSION_ID=")
            s=lcut(s,2,"=")
            gvset("@release_version", stripOuterQuotes(s,1))
        }


        // special cases for release version:

        // case 1: centos/other non-semantic expansion
        vtmp, _ := gvget("@release_version")
        if tr(vtmp.(string),DELETE,"0123456789.","")=="" && !str.ContainsAny(vtmp.(string), ".") {
            vtmp = vtmp.(string) + ".0"
        }
        gvset("@release_version", vtmp)

        cop = Copper("cat /etc/*-release",true)
        s:=lgrep(cop.out,"^ID=")
        s=lcut(s,2,"=")

        tmp = stripOuterQuotes(s, 1)

        // special cases for release id:

        // case 1: opensuse
        if str.HasPrefix(tmp, "opensuse-") {
            tmp = "opensuse"
        }

        // case 2: ubuntu under wsl
        gvset("@winterm", false)
        wsl, _ := gvget("@wsl")
        if str.HasPrefix(wsl.(string), "Ubuntu-") {
            gvset("@winterm", true)
            tmp = "ubuntu"
        }

        gvset("@release_id", tmp)

        // get hostname
        h, _ := os.Hostname()
        gvset("@hostname", h)

    } // endif not windows


    // special case: aliases+pipefail in bash and/or ash
    startupOptions()

    if testMode {
        testStart(exec_file_name)
        defer testExit()
    }

    // for resetting the terminal to global pane
    panes["global"] = Pane{row: 0, col: 0, w: MW + 1, h: MH}
    currentpane = "global"
    orow = 0
    ocol = 0
    ow = MW + 1
    oh = MH

    // reset logging
    logFile = ""
    loggingEnabled = false


    // interactive mode support
    if (*a_program=="" && exec_file_name=="") || interactive {

        // in case we arrived here by another method:
        interactive=true
        interactiveFeed=true

        // output separator, may be unnecessary really
        eol:="\n"
        if runtime.GOOS=="windows" { eol="\r\n" }

        // term loop
        pf("\033[s") // save cursor
        row,col=GetCursorPos()
        if runtime.GOOS=="windows" { row++ ; col++ }
        pcol := defaultPromptColour

        // startup script preparation:

        hasScript:=false
        startScript:=""
        home, _ := gvget("@home")
        startScriptLoc:=home.(string)+"/.zarc"
        if f, err := os.Stat(startScriptLoc); err==nil {
            if f.Mode().IsRegular() {
                startScriptRaw, err := ioutil.ReadFile(startScriptLoc)
                startScript=string(startScriptRaw)
                if err==nil {
                    hasScript=true
                } else {
                    pf("Error: cannot read startup file.\n")
                    os.Exit(ERR_EXISTS)
                }
            } else {
                pf("Error: startup script is not a regular file.\n")
                os.Exit(ERR_EXISTS)
            }
        } else {
            // banner
            title:=sparkle("Za Interactive Mode")
            pf("\n%s\n\n", sparkle("[#bold][#ul][#6]"+title+"[#-][##]"))
        }

        if row>=MH-BMARGIN {
            if row>MH { row=MH }
            for past:=row-(MH-BMARGIN);past>0;past-- { at(MH+1,1); fmt.Print(eol) }
            row=MH-BMARGIN
        }

        // state control
        endFunc := false
        curHist = 0
        lastHist = 0
        histEmpty = true

        mainloc,_ := GetNextFnSpace(true,"main",call_s{prepared:false})
        fnlookup.lmset("main",1)
        numlookup.lmset(1,"main")

        started:=false

        for {

            functionspaces[1] = []Phrase{}
            basecode[1] = []BaseCode{}

            sig_int = false

            var emask any
            var echoMask string
            var ok bool

            if emask,ok=gvget("@echomask"); !ok {
                echoMask=""
            } else {
                echoMask=emask.(string)
            }

            nestAccept:=0
            totalInput:=""
            var eof,broken bool
            var input string

            // static call IDs
            // 0 global (system vars) // template area in interactive mode
            // 1 base template area for "main"
            // 2 execution environment for "main"
            // 3 first free template area for base sources
            // 4... combination of bases and instances

            cs := call_s{}
            cs.caller = 0
            cs.base = 1
            cs.fs = "main"
            calltable[mainloc]=cs

            // startup script processing:
            if !started && hasScript {
                phraseParse("main", startScript, 0)
                currentModule="main"
                _,endFunc = Call(MODE_STATIC, &mident, mainloc, ciRepl)
                pf("\n\n")
                if row>=MH-BMARGIN {
                    if row>MH { row=MH }
                    for past:=row-(MH-BMARGIN);past>0;past-- { at(MH+1,1); fmt.Print(eol) }
                    row=MH-BMARGIN
                }
                started=true
            }

            // multi-line input loop
            for {

                // set the prompt in the loop to ensure it updates regularly
                var tempPrompt string
                if nestAccept==0 {
                    tempPrompt=sparkle(interpolate(0,&gident,PromptTemplate))
                } else {
                    tempPrompt=promptContinuation
                }

                input, eof, broken = getInput(tempPrompt, "global", row, col, pcol, true, true, echoMask)

                if eof || broken { break }

                // getInput re-prints the prompt+input but doesn't add a newline or further at() calls
                // so, we shove the cursor along here:

                row++

                if row>=MH-BMARGIN {
                    if row>MH { row=MH }
                    for past:=row-(MH-BMARGIN);past>0;past-- { at(MH+1,1); fmt.Print(eol) }
                    row=MH-BMARGIN
                }

                at(row,1)
                col = 1

                if input == "\n" {
                    break
                }
                input += "\n"

                // collect input
                totalInput+=input

                breakOnCommand:=false
                tokenIfPresent:=false
                tokenOnPresent:=false
                helpRequest   :=false
                paneDefine    :=false

                var cl int16 // placeholder for current line

                for p := 0; p < len(input);  {

                    t := nextToken(input, 0, &cl, p)
                    if t.tokPos != -1 {
                        p = t.tokPos
                    }

                    if t.carton.tokType==C_Help  { helpRequest   =true }
                    if t.carton.tokType==C_Pane  { paneDefine    =true }
                    if t.carton.tokType==C_If    { tokenIfPresent=true }
                    if t.carton.tokType==C_On    { tokenOnPresent=true }

                    // this is hardly fool-proof, but okay for now:
                    if t.carton.tokType==SYM_BOR && (!tokenIfPresent || !tokenOnPresent) { breakOnCommand=true }
                    if t.carton.tokType==C_Break {
                        nestAccept=0
                        break
                    } // don't check as may also contain a nesting keyword

                    if !helpRequest && !paneDefine {
                        switch t.carton.tokType {
                        case C_Define, C_For, C_Foreach, C_While, C_If, C_When, C_Struct, LParen, LeftSBrace:
                            nestAccept++
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
                fileMap[0]=exec_file_name
                phraseParse("main", totalInput, 0)
                currentModule="main"

                // throw away break and continue positions in interactive mode
                // pf("[main] loc -> %d\n",mainloc)
                _,endFunc = Call(MODE_STATIC, &mident, mainloc, ciRepl)

                if row>=MH-BMARGIN {
                    if row>MH { row=MH }
                    for past:=row-(MH-BMARGIN);past>0;past-- { at(MH+1,1); fmt.Print(eol) }
                    row=MH-BMARGIN
                }

                if endFunc {
                    break
                }
            }

        }
        pln("")

        finish(true, 0)
    }

    //row,col=GetCursorPos()
    //if runtime.GOOS=="windows" { row++ ; col++ }

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

        data,err=ioutil.ReadAll(os.Stdin)
        if err!=nil {
            panic(err)
        }

        s:= `NL=0` + "\n" +
            `foreach _line in _stdin` + "\n" +
            `NL+=1` + "\n"

        if *a_program_fs=="" {
            s+=`fields(_line) `
        } else {
            s+=`fields(_line,"`+*a_program_fs+`") `
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

    row,col=GetCursorPos()
    if runtime.GOOS=="windows" { row++ ; col++ }

    // tokenise and part-parse the input
    if len(input) > 0 {
        fileMap[1]=exec_file_name
        if debug_level>10 {
            start:=time.Now()
            phraseParse("main", input, 0)
            elapsed:=time.Since(start)
            pf("(timings-main) elapsed in parse translation for main : %v\n",elapsed)
        } else {
            phraseParse("main", input, 0)
        }

        // initialise the main program

        mainloc,_ := GetNextFnSpace(true,"main",call_s{prepared:false})
        // pf("[#4]main location set to %d[#-]\n",mainloc)
        calllock.Lock()
        cs := call_s{}
        cs.caller = 0
        cs.base = 1
        cs.fs = "main"
        calltable[mainloc] = cs
        calllock.Unlock()
        currentModule="main"
        // pf("[main] loc -> %d\n",mainloc)
        if *a_program!="" {
            vset(nil,1,&mident,"_stdin", string(data))
        }
        Call(MODE_NEW, &mident, mainloc, ciMain)
        // calltable[mainloc].gcShyness=20
        // calltable[mainloc].gc=true
        calltable[mainloc].gcShyness=0
        calltable[mainloc].gc=false
    }

    // a little paranoia to finish things off...
    setEcho(true)

    if runtime.GOOS!="windows" {
        term_complete()
    }

}

