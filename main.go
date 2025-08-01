package main

//
// IMPORTS
//

import (
    "context"
    "flag"
    "fmt"
    "io"
    "io/ioutil"
    "log"
    "net/http"
    "os"
    "os/exec"
    "path"
    "path/filepath"
    "runtime"
    "strconv"
    "strings"
    str "strings"
    "sync"
    "sync/atomic"
    "time"

    term "github.com/pkg/term"
    // _ "modernc.org/sqlite" 
    _ "github.com/mattn/go-sqlite3"

    // for profiling:

    _ "net/http/pprof"
)

// F_EnableComplexAssignments provides a feature flag to disable deeply nested
// assignments, which are currently being hardened. When false, only assignments
// up to two levels of depth (e.g., `a.b.c` or `a[i].b`) are permitted.
var F_EnableComplexAssignments = true

//
// ALIASES
//

var sf = fmt.Sprintf
var pln = fmt.Println
var fpf = fmt.Fprintln
var fef = fmt.Errorf

//
// GLOBALS
//

// build-time constants made available at run-time
var BuildComment string
var BuildVersion string
var BuildDate string

// global unique name counter
var globseq uint32

// global parser init
var parser *leparser
var interparse *leparser

// list of stdlib categories.
var features = make(map[string]Feature)

// open function call info
var calltable = make([]call_s, CALL_CAP)

// enum storage
var enum = make(map[string]*enum_s)

// id of func space which points to the source which contains
// the DEFINE..ENDDEF for a defined function
var sourceMap = make(map[uint32]uint32)

// tokenised function storage
// this is where all the translated source ends up
var functionspaces = make([][]Phrase, SPACE_CAP)
var basecode = make([][]BaseCode, SPACE_CAP)
var isSource = make([]bool, SPACE_CAP)

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

// base-to-modulename mapping list
var basemodmap = make(map[uint32]string)

// global variable storage
var gident []Variable
var mident []Variable

// lookup tables for converting between function name
//
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
//
//  library calls for flagging an "update".
var firstInstallRun bool = true

// co-proc connection timeout, in milli-seconds
var MAX_TIO time.Duration = 120000

var cmdargs []string   // cli args
var interpolation bool // false to disable string interpolation

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
var bgproc *exec.Cmd  // holder for the coprocess
var pi io.WriteCloser // process input stream
var po io.ReadCloser  // process output stream
var pe io.ReadCloser  // process error stream

// Global: console related
var row, col int       // for pane + terminal use
var MW, MH int         // for pane + terminal use
var BMARGIN int        // bottom offset to stop io at
var currentpane string // for pane use
var tt *term.Term      // keystroke input receiver
var ansiMode bool      // to disable ansi colour output
var lineWrap bool      // optional pane line wrap.
var promptColour string

// Global: setup getInput() history for interactive mode
var curHist int
var lastHist int
var hist []string
var histEmpty bool

// History file management
const MAX_HISTORY_ENTRIES = 255

var historyFile string

// loadHistory loads history from the user's history file
func loadHistory() {
    home, err := os.UserHomeDir()
    if err != nil {
        return // Can't get home dir, skip history loading
    }

    historyFile = home + "/.za_history"

    // Try to read existing history file
    if data, err := os.ReadFile(historyFile); err == nil {
        lines := strings.Split(string(data), "\n")
        hist = make([]string, 0, len(lines))

        // Load non-empty lines, trimming whitespace
        for _, line := range lines {
            line = strings.TrimSpace(line)
            if line != "" {
                hist = append(hist, line)
            }
        }

        // Limit to MAX_HISTORY_ENTRIES
        if len(hist) > MAX_HISTORY_ENTRIES {
            hist = hist[len(hist)-MAX_HISTORY_ENTRIES:]
        }

        lastHist = len(hist)
        histEmpty = len(hist) == 0
    }
}

// saveHistory saves the current history to the user's history file
func saveHistory() {
    if historyFile == "" {
        return // No history file set, skip saving
    }

    // Create history content
    var content strings.Builder
    for _, entry := range hist {
        content.WriteString(entry)
        content.WriteString("\n")
    }

    // Write to file (ignore errors for now)
    os.WriteFile(historyFile, []byte(content.String()), 0600)
}

// addToHistory adds a new entry to history, maintaining the 255 entry limit
func addToHistory(entry string) {
    if entry == "" {
        return
    }

    // Don't add if it's the same as the last entry
    if len(hist) > 0 && hist[len(hist)-1] == entry {
        return
    }

    // Add new entry
    hist = append(hist, entry)
    lastHist++
    histEmpty = false

    // Trim to MAX_HISTORY_ENTRIES if needed
    if len(hist) > MAX_HISTORY_ENTRIES {
        hist = hist[len(hist)-MAX_HISTORY_ENTRIES:]
        lastHist = len(hist)
    }
}

// Global: logging related
var logFile string
var loggingEnabled bool
var log_web bool
var web_log_file string = "./za_access.log"

// Global: JSON logging related
var jsonLoggingEnabled bool
var logFields map[string]any
var logFieldsStack []map[string]any

// Global: Error logging integration
var errorLoggingEnabled bool
var emergencyReserveSize int = 1024 * 1024 // Default 1MB
var logRotateSize int64
var logRotateCount int
var logQueueSize int = 60     // Default queue size (increased for modern systems)
var webLogRequestCount int64  // Total web access log requests processed
var mainLogRequestCount int64 // Total main log requests processed

// RFC 5424 Log Severity Levels
const (
    LOG_EMERG   = 0 // Emergency: system is unusable
    LOG_ALERT   = 1 // Alert: action must be taken immediately
    LOG_CRIT    = 2 // Critical: critical conditions
    LOG_ERR     = 3 // Error: error conditions
    LOG_WARNING = 4 // Warning: warning conditions
    LOG_NOTICE  = 5 // Notice: normal but significant condition
    LOG_INFO    = 6 // Informational: informational messages
    LOG_DEBUG   = 7 // Debug: debug-level messages
)

var logMinLevel int = LOG_DEBUG // Default: show all levels

// Global: generic flags
var sig_int bool       // ctrl-c pressed?
var coproc_reset bool  // for resetting locked coproc instances
var coproc_active bool //
var no_shell bool      // disable sub-shell
var shellrep bool      // enable shell command reporting

// Global: behaviours
var permit_uninit bool // default:false, will evaluation cause a run-time failure if it
// encounters an uninitialised variable usage.
// this can be altered with the permit("uninit",bool) call
var permit_dupmod bool // default:false, ignore (true) or error (false) when a duplicate
// module import occurs.
var permit_exitquiet bool                   // default:false, squash (true) or display (false) err msg on exit
var permit_shell bool                       // default: true, when false, exit script if shell command encountered
var permit_eval bool                        // default: true, when false, exit script if eval call encountered
var permit_permit bool                      // default: true, when false, permit function is disabled
var permit_cmd_fallback bool                // default: false, when true and in interactive mode, exec in shell as fallback
var permit_error_exit bool = true           // default: true, when false, error handler won't exit program
var permit_exception_strictness bool = true // default: true, when false, exception_strictness() function is disabled

// Global: exception handling
var exceptionStrictness string = "strict" // default: strict, options: strict, permissive, warn, disabled
var unhandledAtSource bool                // tracks if exception originated from unhandled try block
var unhandledExceptionInfo struct {
    category     string
    message      string
    location     string
    functionName string
}

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
var enforceError bool

// - not currently used too much. may eventually be removed
var debugMode bool     //  enable debugging repl
var lineDebug bool     //
var enableAsserts bool // turn assert interpretation on/off

// list of keywords for lookups
// - used in interactive mode TAB completion
var keywordset map[string]struct{}

// list of struct fields per struct type
// - used by VAR when defining a struct
var structmaps map[string][]any

// compile cache for regex operator
// var ifCompileCache map[string]regexp.Regexp
var ifCompileCache sync.Map // maps string -> *regexp.Regexp

// repl prompt
var PromptTemplate string
var PromptTokens []Token

var concurrent_funcs int32
var has_global_lock uint32

var breaksig chan os.Signal

//
// MAIN
//

// default precedence table that each parser copy receives.
var default_prectable [END_STATEMENTS]int8

const PrecedenceInvalid = -100

func main() {

    // lineWrap=true // currently disabled - it breaks up ansi sequences. will re-enable when dealt with.

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
    if runtime.GOOS != "windows" {
        setWinchSignal(sigs)
    }

    if runtime.GOOS == "windows" {
        winmode = true
    }

    BMARGIN = 8

    permit_shell = true
    permit_eval = true
    permit_permit = true

    go func() {
        for {
            <-sigs
            sglock.Lock()
            MW, MH, _ = GetSize(1)
            sglock.Unlock()
            shelltype, _ := gvget("@shelltype")
            if shelltype == "bash" || shelltype == "ash" {
                if MW != -1 {
                    if runtime.GOOS == "freebsd" {
                        Copper(sf(`alias ls="COLUMNS=%d ls -C"`, MW), true)
                    } else {
                        Copper(sf(`alias ls="ls -x -w %d"`, MW), true)
                    }
                }
            }
        }
    }()

    // debug mode setup:
    var breaksig = make(chan os.Signal, 1)
    var signals = make(chan os.Signal, 1)
    setupSignalHandlers(signals, breaksig)
    // end of debug setup

    // lower number means : "binding is less tight than operators with higher number"
    // e.g. a+b>c  : + is 31 : > is 25 : a+b performed before evaluating >c
    // in general: conjunctions and comparisons should have lower number than other operators
    // assignment should be among the lowest bp.
    // map/filter are low because they act in a similar way to assignment
    // The misc_x bindings are still perhaps not in their best positions.

    // Lm notations are how most other people organise these operations
    // with L01 being the most precedent and L15 the least.

    // Set all to invalid
    for i := range default_prectable {
        default_prectable[i] = PrecedenceInvalid
    }

    default_prectable[EOF] = -1

    default_prectable[Identifier] = 1     // dummy value to stop reject in dparse()
    default_prectable[NumericLiteral] = 1 // dummy value to stop reject in dparse()
    default_prectable[StringLiteral] = 1  // dummy value to stop reject in dparse()
    default_prectable[O_Ref] = 1          // dummy value to stop reject in dparse()
    default_prectable[O_Sqr] = 1          // dummy value to stop reject in dparse()
    default_prectable[O_Sqrt] = 1         // dummy value to stop reject in dparse()
    default_prectable[O_Mut] = 1          // dummy value to stop reject in dparse()
    default_prectable[O_Slc] = 1          // dummy value to stop reject in dparse()
    default_prectable[O_Suc] = 1          // dummy value to stop reject in dparse()
    default_prectable[O_Sst] = 1          // dummy value to stop reject in dparse()
    default_prectable[O_Slt] = 1          // dummy value to stop reject in dparse()
    default_prectable[O_Pb] = 1           // dummy value to stop reject in dparse()
    default_prectable[O_Pa] = 1           // dummy value to stop reject in dparse()
    default_prectable[O_Pn] = 1           // dummy value to stop reject in dparse()
    default_prectable[O_Pe] = 1           // dummy value to stop reject in dparse()
    default_prectable[O_Pp] = 1           // dummy value to stop reject in dparse()
    default_prectable[T_Nil] = 1          // dummy value to stop reject in dparse()
    default_prectable[T_Number] = 1       // dummy value to stop reject in dparse()
    default_prectable[T_Bool] = 1         // dummy value to stop reject in dparse()
    default_prectable[T_Uint] = 1         // dummy value to stop reject in dparse()
    default_prectable[T_Int] = 1          // dummy value to stop reject in dparse()
    default_prectable[T_String] = 1       // dummy value to stop reject in dparse()
    default_prectable[T_Float] = 1        // dummy value to stop reject in dparse()
    default_prectable[T_Bigi] = 1         // dummy value to stop reject in dparse()
    default_prectable[T_Bigf] = 1         // dummy value to stop reject in dparse()
    default_prectable[T_Map] = 1          // dummy value to stop reject in dparse()
    default_prectable[T_Array] = 1        // dummy value to stop reject in dparse()
    default_prectable[T_Any] = 1          // dummy value to stop reject in dparse()
    default_prectable[Block] = 1          // dummy value to stop reject in dparse()
    default_prectable[AsyncBlock] = 1     // dummy value to stop reject in dparse()
    default_prectable[ResultBlock] = 1    // dummy value to stop reject in dparse()

    // assignment-type group
    default_prectable[O_Assign] = 5 // L09
    default_prectable[O_Map] = 7
    default_prectable[O_Filter] = 9
    default_prectable[O_Try] = 10 // ?? operator - higher than assignment, lower than logical operators

    // booleans @note: and/or + &&/|| tokenisation needs tidying
    default_prectable[SYM_LOR] = 15  // L13
    default_prectable[C_Or] = 15     // L13
    default_prectable[SYM_LAND] = 15 // L12

    // bit-wise
    default_prectable[SYM_BAND] = 20   // L07
    default_prectable[SYM_BOR] = 19    // L07
    default_prectable[SYM_Caret] = 20  // L07
    default_prectable[SYM_LSHIFT] = 21 // L07
    default_prectable[SYM_RSHIFT] = 21 // L07

    // misc 1
    default_prectable[O_Query] = 23 // tern // L14
    default_prectable[SYM_Not] = 24

    // equality type tests
    default_prectable[SYM_Tilde] = 25
    default_prectable[SYM_ITilde] = 25
    default_prectable[SYM_FTilde] = 25
    default_prectable[C_Is] = 25
    default_prectable[SYM_EQ] = 25 // L11
    default_prectable[SYM_NE] = 25 // L11
    default_prectable[SYM_LT] = 25 // L10
    default_prectable[SYM_GT] = 25 // L10
    default_prectable[SYM_LE] = 25 // L10
    default_prectable[SYM_GE] = 25 // L10
    default_prectable[C_In] = 26

    // misc 2
    default_prectable[SYM_RANGE] = 29 // L08

    // arithmetic
    default_prectable[O_Plus] = 31     // L06
    default_prectable[O_Minus] = 31    // L06
    default_prectable[O_Divide] = 35   // L05
    default_prectable[O_Percent] = 35  // mod  // L05
    default_prectable[O_Multiply] = 35 // L05
    default_prectable[SYM_POW] = 37

    default_prectable[SYM_PP] = 45 // L02
    default_prectable[SYM_MM] = 45 // L02

    // misc 3
    default_prectable[O_OutFile] = 37

    // sub-access
    default_prectable[LeftSBrace] = 45      // L02
    default_prectable[SYM_DoubleColon] = 59 //field // L02
    default_prectable[SYM_DOT] = 61         //field // L02

    default_prectable[O_InFile] = 70

    default_prectable[LParen] = 100 // L01

    // generic error flag - used through main
    var err error

    // create shared storage
    gident = make([]Variable, 64)
    mident = make([]Variable, 64)

    // Initialize emergency memory reserve for error handling
    if enhancedErrorsEnabled {
        // Allocate emergency memory reserve dynamically to avoid global layout disruption
        reserve := make([]byte, emergencyReserveSize) // Configurable reserve size
        emergencyMemoryReserve = &reserve
    }

    // setup empty symbol tables for main
    bindlock.Lock()
    bindings[1] = make(map[string]uint64)
    bindlock.Unlock()

    // create identifiers for global and main source caches
    fnlookup.lmset("global", 0)
    fnlookup.lmset("main", 1)
    numlookup.lmset(0, "global")
    numlookup.lmset(1, "main")

    basemodmap[0] = "global"
    basemodmap[1] = "main"

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
    interpolation = true

    // setup the functions in the standard library.
    // - this must come before any use of vset()
    buildStandardLib()

    // initialize the exception enum system
    // - this must come after buildStandardLib() so categories[] is populated
    initializeExceptionEnum()

    // create lookup list for keywords
    // - this must come before any use of vset()
    keywordset = make(map[string]struct{})
    for keyword := range completions {
        keywordset[completions[keyword]] = struct{}{}
    }

    // create the structure definition storage area
    structmaps = make(map[string][]any)

    // compile cache for regex operator
    // ifCompileCache = make(map[string]regexp.Regexp)

    // get terminal dimensions
    MW, MH, _ = GetSize(1)

    // set default prompt colour
    promptColour = defaultPromptColour

    lineDebug = false

    // start processing startup flags

    // command output unit separator
    gvset("@cmdsep", byte(0x1e))

    // run in parent - if -S opt or /bin/false specified
    //  for shell, then run commands in parent
    gvset("@runInParent", false)

    // should command output be captured?
    // - when disabled, output is sent to stdout
    gvset("@commandCapture", true)

    // like -S, but insist upon it for Windows executions.
    gvset("@runInWindowsParent", false)

    // set available build info
    gvset("@language", "Za")
    gvset("@version", BuildVersion)
    gvset("@creation_author", "D Horsley")
    gvset("@creation_date", BuildDate)

    // set interactive prompt
    gvset("@startprompt", promptStringStartup)
    gvset("@bashprompt", promptBashlike)
    PromptTemplate = promptStringStartup

    // set default behaviours

    // - don't echo logging (default to console output for log commands)
    gvset("@silentlog", false)

    // - don't show co-proc command progress
    gvset("mark_time", false)

    // - max depth of interactive mode dir context help
    gvset("context_dir_depth", 1)

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

    // create default context for main
    ctx := withProfilerContext(context.Background())

    // initialise global parser
    parser = &leparser{}
    parser.ctx = ctx

    // interpolation parser
    interparse = &leparser{}
    interparse.ctx = ctx

    // arg parsing
    var a_help = flag.Bool("h", false, "help page")
    var a_version = flag.Bool("v", false, "display the Za version")
    var a_interactive = flag.Bool("i", false, "run interactively")
    var a_scriptBypass = flag.Bool("b", false, "bypass startup script")
    var a_debug = flag.Bool("d", false, "set debug mode")
    var a_lineDebug = flag.Bool("D", false, "enable line debug")
    var a_profile = flag.Bool("p", false, "enable profiler")
    // var a_trace          =   flag.Bool("P",false,"enable trace capture")
    var a_test = flag.Bool("t", false, "enable tests")
    var a_test_file = flag.String("o", "za_test.out", "set the test output filename")
    var a_filename = flag.String("f", "", "input filename, when present. default is stdin")
    var a_program = flag.String("e", "", "program string")
    var a_program_loop = flag.Bool("r", false, "wraps a program string in a stdin loop - awk-like")
    var a_program_fs = flag.String("F", "", "provides a field separator for -r")
    var a_test_override = flag.String("O", "continue", "test override value")
    var a_test_name = flag.String("N", "", "test name filter")
    var a_test_group = flag.String("G", "", "test group filter")
    var a_time_out = flag.Int("T", 0, "Co-process command time-out (ms)")
    var a_mark_time = flag.Bool("m", false, "Mark co-process command progress")
    var a_ansi = flag.Bool("c", false, "disable colour output")
    var a_ansiForce = flag.Bool("C", false, "enable colour output")
    var a_shell = flag.String("s", "", "path to coprocess shell")
    var a_shellrep = flag.Bool("Q", false, "enables the shell info reporting")
    var a_noshell = flag.Bool("S", false, "disables the coprocess shell")
    var a_cmdsep = flag.Int("U", 0x1e, "Command output separator byte")
    var a_var_refs = flag.String("V", "", "find all references to a variable")
    var a_var_warn = flag.Bool("W", false, "emit errors when addition contains string mixed types")
    var a_enable_asserts = flag.Bool("a", false, "enable assertions. default is false, unless -t specified.")
    var a_enable_profiling = flag.Bool("P", false, "enable profiling of Za interpreter phases.")

    flag.Parse()
    cmdargs = flag.Args() // rest of the cli arguments
    exec_file_name := ""

    // Process ZA_LOG_LEVEL environment variable
    if envLogLevel := os.Getenv("ZA_LOG_LEVEL"); envLogLevel != "" {
        switch strings.ToLower(envLogLevel) {
        case "emerg", "emergency":
            logMinLevel = LOG_EMERG
        case "alert":
            logMinLevel = LOG_ALERT
        case "crit", "critical":
            logMinLevel = LOG_CRIT
        case "err", "error":
            logMinLevel = LOG_ERR
        case "warn", "warning":
            logMinLevel = LOG_WARNING
        case "notice":
            logMinLevel = LOG_NOTICE
        case "info":
            logMinLevel = LOG_INFO
        case "debug":
            logMinLevel = LOG_DEBUG
        default:
            // Try parsing as number
            if level, err := strconv.Atoi(envLogLevel); err == nil && level >= 0 && level <= 7 {
                logMinLevel = level
            }
        }
    }

    // Process ZA_LOG_QUEUE_SIZE environment variable
    if envQueueSize := os.Getenv("ZA_LOG_QUEUE_SIZE"); envQueueSize != "" {
        if size, err := strconv.Atoi(envQueueSize); err == nil && size >= 1 {
            logQueueSize = size
        }
    }

    // phase profiling flag
    if *a_enable_profiling {
        enableProfiling = true
    }

    // mono flag
    ansiMode = true
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
    var_refs = false
    if *a_var_refs != "" {
        var_refs = true
        var_refs_name = *a_var_refs
    }

    // type warnings
    var_warn = false
    if *a_var_warn {
        var_warn = true
    }

    // source filename
    if *a_filename != "" {
        exec_file_name = *a_filename
    } else {
        // try first cmdarg
        if len(cmdargs) > 0 {
            exec_file_name = cmdargs[0]
            if !interactive && *a_program == "" {
                cmdargs = cmdargs[1:]
            }
        }
    }

    // figure out correct source path and execution path
    fpath, _ := filepath.Abs(exec_file_name)
    fdir := fpath

    f, err := os.Stat(fpath)
    if err == nil {
        if !f.Mode().IsDir() {
            fdir = filepath.Dir(fpath)
        }
    }
    gvset("@execpath", fdir)

    // help flag
    if *a_help {
        help("main", "")
        os.Exit(0)
    }

    // version flag
    if *a_version {
        version()
        os.Exit(0)
    }

    // command separator
    if *a_cmdsep != 0 {
        gvset("@cmdsep", byte(*a_cmdsep))
    }

    if *a_debug {
        debugMode = *a_debug
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

    /*
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
    */

    gvset("@winterm", false)

    // pprof - not advertised.
    if *a_profile {
        go func() {
            // runtime.SetCPUProfileRate(1000)
            log.Fatalln(http.ListenAndServe("localhost:8008", http.DefaultServeMux))
        }()
    }

    // test mode

    enableAsserts = false
    enforceError = true

    if *a_test {
        testMode = true
        enableAsserts = true
    }

    if *a_test_override != "" {
        fail_override = *a_test_override
    }

    test_output_file = *a_test_file
    _ = os.Remove(test_output_file)

    test_group_filter = *a_test_group
    test_name_filter = *a_test_name

    if *a_enable_asserts {
        enableAsserts = *a_enable_asserts
    }

    // disable the coprocess command
    if *a_noshell {
        no_shell = true
    }
    gvset("@noshell", no_shell)

    if *a_shellrep {
        shellrep = true
    }
    gvset("@shell_report", shellrep)

    // set the coprocess command
    default_shell := ""
    if *a_shell != "" {
        default_shell = *a_shell
    }

    //
    // Primary activity below
    //

    var data []byte // input buffering

    // start shell in co-process

    coprocLoc := ""
    var coprocArgs []string

    gvset("@shelltype", "")

    // figure out the correct shell to use, with available info.
    if runtime.GOOS != "windows" {
        if !no_shell {
            if default_shell == "" {
                coprocLoc, err = GetCommand("/usr/bin/which bash")
                if err == nil {
                    coprocLoc = coprocLoc[:len(coprocLoc)-1]
                    gvset("@shelltype", "bash")
                } else {
                    if fexists("/bin/bash") {
                        coprocLoc = "/bin/bash"
                        coprocArgs = []string{"-i"}
                        gvset("@shelltype", "bash")
                    } else {
                        // try for /bin/sh then default to noshell
                        if fexists("/bin/sh") {
                            coprocLoc = "/bin/sh"
                            coprocArgs = []string{"-i"}
                        } else {
                            gvset("@noshell", no_shell)
                            coprocLoc = "/bin/false"
                        }
                    }
                }
            } else { // not default shell
                if !fexists(default_shell) {
                    pf("The chosen shell (%v) does not exist.\n", default_shell)
                    os.Exit(ERR_NOBASH)
                }
                coprocLoc = default_shell
                shellname := path.Base(coprocLoc)
                // a few common shells require use of external printf
                // for separating output using non-printables.
                if shellname == "dash" || shellname == "ash" || shellname == "sh" {
                    // specify that NextCopper() should use external printf
                    // for generating \x1e (or other cmdsep) in output
                    gvset("@shelltype", shellname)
                }
            }
        }

    } else {

        // windows run-time. requires that commands are sent
        // individually through cmd.exe.
        // @note: windows is still an afterthought. this will need much
        // improvement if we ever take windows seriously.

        coprocLoc = "C:/Windows/System32/cmd.exe"
        gvset("@noshell", true)
        gvset("@os", "windows")
        gvset("@zsh_version", "")
        gvset("@bash_version", "")
        gvset("@bash_versinfo", "")
        gvset("@cwd", ".")
        gvset("@wsl", "")
        gvset("@winterm", false)
        gvset("@runInWindowsParent", true)

        // Get Windows-specific user and locale information
        if username, err := getCurrentUsername(); err == nil {
            gvset("@user", username)
        }
        if locale, err := getCurrentLocale(); err == nil {
            gvset("@lang", locale)
        }
        if homeDir, err := getCurrentHomeDir(); err == nil {
            gvset("@home", homeDir)
        }
        if releaseName, releaseId, releaseVersion, err := getWindowsReleaseInfo(); err == nil {
            gvset("@release_name", releaseName)
            gvset("@release_id", releaseId)
            gvset("@release_version", releaseVersion)
        }

        // Set empty shell version info for Windows (no external execution)
        gvset("@powershell_version", "")
        gvset("@cmd_version", "")
    }

    shelltype, _ := gvget("@shelltype")
    gvset("@shell_location", coprocLoc)

    if runtime.GOOS == "windows" || no_shell || coprocLoc == "/bin/false" {
        gvset("@runInParent", true)
    }

    if runtime.GOOS != "windows" {

        if !no_shell {
            // create shell process
            bgproc, pi, po, pe = NewCoprocess(coprocLoc, coprocArgs...)
            gvset("@shell_pid", bgproc.Process.Pid)
        }

        // prepare for getInput() keyboard input (from main process)
        tt, _ = term.Open("/dev/tty")
        // enable_mouse()
    }

    // - name of Za function that handles ctrl-c.
    gvset("@trapInt", "")
    // - name of Za function that handles errors.
    gvset("@trapError", "")

    go func() {
        for {
            bs := <-breaksig
            // pf("Received signal : [%#v]\n",bs)
            quiet := false

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
                bgproc, pi, po, pe = NewCoprocess(coprocLoc, coprocArgs...)
                // pf("\nnew pid %v\n", bgproc.Process.Pid)
                gvset("@shell_pid", bgproc.Process.Pid)
                gvset("@last_signal", sf("%v %v", bs, bgproc.Process.Pid))
                lastlock.Lock()
                coproc_active = false
                coproc_reset = false
                quiet = true
                lastlock.Unlock()
            }

            // user-trap handling
            userSigIntHandler, usihfound := gvget("@trapInt")
            usih := ""
            if usihfound {
                switch userSigIntHandler.(type) {
                case string:
                    usih = userSigIntHandler.(string)
                }
            }

            if usih != "" {

                if !str.Contains(usih, "::") {
                    if found := uc_match_func(usih); found != "" {
                        usih = found + "::" + usih
                    } else {
                        usih = currentModule + "::" + usih
                    }
                }

                argString := ""
                if brackPos := str.IndexByte(usih, '('); brackPos != -1 {
                    argString = usih[brackPos:]
                    usih = usih[:brackPos]
                }

                // calc arguments from string

                var iargs []any
                if argString != "" {
                    argString = stripOuter(argString, '(')
                    argString = stripOuter(argString, ')')

                    // evaluate args
                    var argnames []string

                    var mloc uint32
                    if interactive {
                        mloc = 1
                    } else {
                        mloc = 2
                    }

                    // populate inbound parameters to the za function
                    // call, with evaluated versions of each.
                    if argString != "" {
                        argnames = str.Split(argString, ",")
                        for k, a := range argnames {
                            aval, err := ev(interparse, mloc, a)
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

                lmv, _ := fnlookup.lmget(usih)
                loc, _ := GetNextFnSpace(true, usih+"@", call_s{prepared: true, base: lmv, caller: 0})

                calllock.Lock()
                basemodmap[lmv] = "main"
                calllock.Unlock()

                // execute call

                var trident = make([]Variable, identInitialSize)

                // Set the callLine field in the calltable entry before calling the function
                // For trap calls, we don't have parser context, so use 0
                atomic.StoreInt32(&calltable[loc].callLine, 0) // Trap calls don't have parser context
                Call(ctx, MODE_NEW, &trident, loc, ciTrap, false, nil, "", []string{}, nil, iargs...)
                if calltable[loc].retvals != nil {
                    sigintreturn := calltable[loc].retvals.([]any)
                    if len(sigintreturn) > 0 {
                        switch sigintreturn[0].(type) {
                        case int:
                        default:
                            finish(true, 124)
                        }
                        if sigintreturn[0].(int) != 0 {
                            finish(true, sigintreturn[0].(int))
                        }
                    }
                }
                calltable[loc].gcShyness = 0
                calltable[loc].gc = false
            } else {
                finish(false, 0)
                if !quiet {
                    pf("[#2]System Interrupt![#-]\n")
                } else {
                    startupOptions()
                }
            }
        }
    }()

    var cop struct {
        out  string
        err  string
        code int
        okay bool
    }

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
    if runtime.GOOS != "windows" {

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

        gvset("@os", runtime.GOOS)

        cop = Copper("echo -n $HOME", true)
        gvset("@home", cop.out)

        gvset("@release_name", "unknown")
        gvset("@release_version", "unknown")

        release_info := Copper("cat /etc/*-release", true)
        s := lgrep(release_info.out, "^ID=")
        s = lcut(s, 2, "=")
        release_id := stripOuterQuotes(s, 1)

        if runtime.GOOS == "linux" {

            s := lgrep(release_info.out, "^NAME=")
            s = lcut(s, 2, "=")
            gvset("@release_name", stripOuterQuotes(s, 1))

            s = lgrep(release_info.out, "^VERSION_ID=")
            s = lcut(s, 2, "=")
            gvset("@release_version", stripOuterQuotes(s, 1))

        }

        // special cases for release version:

        // case 1: centos/other non-semantic expansion
        vtmp, _ := gvget("@release_version")
        if tr(vtmp.(string), DELETE, "0123456789.", "") == "" && !str.ContainsAny(vtmp.(string), ".") {
            vtmp = vtmp.(string) + ".0"
        }
        gvset("@release_version", vtmp)

        // special cases for release id:

        // case 1: opensuse
        if str.HasPrefix(release_id, "opensuse-") {
            release_id = "opensuse"
        }

        // case 2: ubuntu under wsl
        gvset("@winterm", false)
        wsl, _ := gvget("@wsl")
        if str.HasPrefix(wsl.(string), "Ubuntu-") {
            gvset("@winterm", true)
            release_id = "ubuntu"
        }

        gvset("@release_id", release_id)

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

    // Initialize JSON logging (for both interactive and non-interactive modes)
    jsonLoggingEnabled = false
    logFields = make(map[string]any)
    logFieldsStack = make([]map[string]any, 0)

    // interactive mode support
    if (*a_program == "" && exec_file_name == "") || interactive {

        // in case we arrived here by another method:
        interactive = true
        interactiveFeed = true

        // output separator, may be unnecessary really
        eol := "\n"
        if runtime.GOOS == "windows" {
            eol = "\r\n"
        }

        // term loop
        pf("\033[s") // save cursor
        row, col = GetCursorPos()
        if runtime.GOOS == "windows" {
            row++
            col++
        }
        pcol := defaultPromptColour

        // startup script preparation:
        hasScript := false
        startScript := ""
        home, _ := gvget("@home")
        startScriptLoc := home.(string) + "/.zarc"
        if f, err := os.Stat(startScriptLoc); err == nil && !*a_scriptBypass {
            if f.Mode().IsRegular() {
                startScriptRaw, err := ioutil.ReadFile(startScriptLoc)
                startScript = string(startScriptRaw)
                if err == nil {
                    hasScript = true
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
            title := sparkle("Za Interactive Mode")
            pf("\n%s", sparkle("[#bold][#ul][#6]"+title+"[#-][##]"))
            pf(str.Repeat("\n",BMARGIN+2))
            row+=2
        }


        // state control
        endFunc := false
        curHist = 0
        lastHist = 0
        histEmpty = true

        // Load permanent history file
        loadHistory()

        // Ensure history is saved on any exit (normal or abnormal)
        defer func() {
            if interactive {
                saveHistory()
            }
        }()

        mainloc, _ := GetNextFnSpace(true, "main", call_s{prepared: false})
        fnlookup.lmset("main", 1)
        numlookup.lmset(1, "main")

        started := false
        first_prompt:=true
        gvset("@lastcmd", "")

        for {

            functionspaces[1] = []Phrase{}
            basecode[1] = []BaseCode{}

            sig_int = false

            var emask any
            var echoMask string
            var ok bool

            if emask, ok = gvget("@echomask"); !ok {
                echoMask = ""
            } else {
                echoMask = emask.(string)
            }

            nestAccept := 0
            totalInput := ""
            var eof, broken bool
            var input string
            fileMap.Store(uint32(0), exec_file_name)
            fileMap.Store(uint32(1), exec_file_name)

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
            calltable[mainloc] = cs

            // startup script processing:
            var errVal error
            if !started && hasScript {
                phraseParse(ctx, "main", startScript, 0, 0)
                basemodmap[1] = "main"
                // Set the callLine field in the calltable entry before calling the function
                // For REPL calls, we use line 1 as the main entry point
                atomic.StoreInt32(&calltable[mainloc].callLine, 1)
                _, endFunc, _, _, errVal = Call(ctx, MODE_STATIC, &mident, mainloc, ciRepl, false, nil, "", []string{}, nil)
                if errVal != nil {
                    pf("error in startup script processing:%s\n", errVal)
                }
                if row >= MH-BMARGIN {
                    if row > MH {
                        row = MH
                    }
                    for past := row - (MH - BMARGIN); past > 0; past-- {
                        at(MH+1, 1)
                        fmt.Print(eol)
                    }
                    row = MH - BMARGIN
                }

                started = true
            }

            // multi-line input loop
            for {

                parser.namespace = "main"
                currentModule = "main"
                interparse.namespace = "main"

                // set the prompt in the loop to ensure it updates regularly
                var tempPrompt string
                if nestAccept == 0 {
                    if len(PromptTokens) > 0 {
                        we := interparse.wrappedEval(1, &mident, 1, &mident, PromptTokens)
                        if !we.evalError {
                            switch we.result.(type) {
                            case string:
                                tempPrompt = sparkle(interpolate("main", 1, &mident, we.result.(string)))
                            }
                        }
                    } else {
                        tempPrompt = sparkle(interpolate("main", 1, &mident, PromptTemplate))
                    }
                } else {
                    tempPrompt = promptContinuation
                }

                gvset("@last", 0)
                input, eof, broken = getInput(tempPrompt, "", "global", row, col, panes["global"].w-2, []string{}, pcol, true, true, echoMask)

                if eof || broken {
                    break
                }

                // getInput re-prints the prompt+input but doesn't add a newline or further at() calls
                // so, we shove the cursor along here:

                row++

                /*
                   if started && row>=MH-BMARGIN {
                       if row>MH { row=MH }
                       for past:=row-(MH-BMARGIN);past>0;past-- { at(MH+1,1); fmt.Print(eol) }
                       row=MH-BMARGIN
                   }
                */

                at(row, 1)
                col = 1

                if input == "\n" {
                    break
                }
                input += "\n"

                // collect input
                totalInput += input

                breakOnCommand := false
                tokenIfPresent := false
                tokenOnPresent := false
                helpRequest := false
                paneDefine := false

                var cl int16 // placeholder for current line

                for p := 0; p < len(input); {

                    t := nextToken(input, 0, &cl, p)
                    if t.tokPos != -1 {
                        p = t.tokPos
                    }

                    if t.carton.tokType == C_Help {
                        helpRequest = true
                    }
                    if t.carton.tokType == C_Pane {
                        paneDefine = true
                    }
                    if t.carton.tokType == C_If {
                        tokenIfPresent = true
                    }
                    if t.carton.tokType == C_On {
                        tokenOnPresent = true
                    }

                    // this is hardly fool-proof, but okay for now:
                    if t.carton.tokType == SYM_BOR && (!tokenIfPresent || !tokenOnPresent) {
                        breakOnCommand = true
                    }
                    if t.carton.tokType == C_Break {
                        nestAccept = 0
                        break
                    } // don't check as may also contain a nesting keyword

                    // @note: the stanza below causes issues with fallback mode, for example if you
                    //        have an element of a path name that is if, case, while... etc.
                    //        not sure of best way to deal with that yet.
                    if !helpRequest && !paneDefine {
                        switch t.carton.tokType {
                        case C_Define, C_For, C_Foreach, C_While, C_If, C_Case, C_Struct, LParen, LeftSBrace:
                            nestAccept++
                        case C_Enddef, C_Endfor, C_Endwhile, C_Endif, C_Endcase, C_Endstruct, RParen, RightSBrace:
                            nestAccept--
                        }
                    }

                }

                if nestAccept < 0 {
                    pf("Nesting error.\n")
                    break
                }
                if nestAccept == 0 || breakOnCommand {
                    break
                }

            }

            if eof || broken {
                break
            }

            // submit input

            if nestAccept == 0 {
                fileMap.Store(uint32(0), exec_file_name)
                phraseParse(ctx, "main", totalInput, 0, 0)
                currentModule = "main"
                parser.namespace = "main"

                // throw away break and continue positions in interactive mode
                // pf("[main] loc -> %d\n",mainloc)
                // Set the callLine field in the calltable entry before calling the function
                // For REPL calls, we use line 1 as the main entry point
                atomic.StoreInt32(&calltable[mainloc].callLine, 1)
                _, endFunc, _, _, _ = Call(ctx, MODE_STATIC, &mident, mainloc, ciRepl, false, nil, "", []string{}, nil)

                if row >= MH-BMARGIN {
                    if row > MH {
                        row = MH
                    }
                    for past := row - (MH - BMARGIN); past > 0; past-- {
                        at(MH+1, 1)
                        fmt.Print(eol)
                        if first_prompt {
                            past=0
                            first_prompt=false
                        }
                    }
                    row = MH - BMARGIN
                }

                if endFunc {
                    break
                }
            }

        }
        pln("")

        // Save history before exiting
        saveHistory()

        finish(true, 0)
    }

    //row,col=GetCursorPos()
    //if runtime.GOOS=="windows" { row++ ; col++ }

    // if not in interactive mode, then get input from either file or stdin:
    if *a_program == "" {
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

        data, err = ioutil.ReadAll(os.Stdin)
        if err != nil {
            panic(err)
        }

        s := `NL=0` + "\n" +
            `foreach _line in _stdin` + "\n" +
            `NL+=1` + "\n"

        if *a_program_fs == "" {
            s += `_=fields(_line) `
        } else {
            s += `_=fields(_line,"` + *a_program_fs + `") `
        }
        s += "\n" + *a_program + "\nendfor\n"
        *a_program = s

    }

    // source the program
    var input string
    if *a_program != "" {
        input = *a_program + "\n"
    } else {
        input = string(data)
    }

    row, col = GetCursorPos()
    if runtime.GOOS == "windows" {
        row++
        col++
    }

    // tokenise and part-parse the input
    if len(input) > 0 {
        fileMap.Store(uint32(0), exec_file_name)
        fileMap.Store(uint32(1), exec_file_name)
        if debugMode {
            start := time.Now()
            phraseParse(ctx, "main", input, 0, 0)
            elapsed := time.Since(start)
            pf("(timings-main) elapsed in parse translation for main : %v\n", elapsed)
        } else {
            phraseParse(ctx, "main", input, 0, 0)
        }

        // initialise the main program

        mainloc, _ := GetNextFnSpace(true, "main", call_s{prepared: false})
        calllock.Lock()
        cs := call_s{}
        cs.caller = 0
        cs.base = 1
        cs.fs = "main"
        atomic.StoreInt32(&cs.callLine, 1) // Main program entry point
        calltable[mainloc] = cs
        calllock.Unlock()
        currentModule = "main"
        if *a_program != "" {
            vset(nil, 1, &mident, "_stdin", string(data))
        }

        Call(ctx, MODE_NEW, &mident, mainloc, ciMain, false, nil, "", []string{}, nil)

        // Check if main function returned with unhandled exception
        calllock.Lock()
        mainRetvals := calltable[mainloc].retvals
        calllock.Unlock()

        if mainRetvals != nil {
            if retArray, ok := mainRetvals.([]any); ok && len(retArray) >= 1 {
                if status, ok := retArray[0].(int); ok && status == EXCEPTION_THROWN {
                    // Main function returned with unhandled exception - apply strictness policy
                    // var category any = "unknown"
                    // var message string = "unknown error"
                    var excInfo *exceptionInfo = nil
                    if len(retArray) >= 4 {
                        // category = retArray[1]
                        // message = GetAsString(retArray[2])
                        if excPtr, ok := retArray[3].(*exceptionInfo); ok {
                            excInfo = excPtr
                        }
                    } else if len(retArray) >= 3 {
                        // Fallback for old format
                        // category = retArray[1]
                        // message = GetAsString(retArray[2])
                    }
                    handleUnhandledException(excInfo, 0) // Use function space 0 for main
                } else {
                    // DEBUG: Not an exception return - let normal error handling proceed
                    pf("[#fyellow]DEBUG: Main returned non-exception values: %+v[#-]\n", retArray)
                }
            } else {
                // DEBUG: Not an array return - let normal error handling proceed
                pf("[#fyellow]DEBUG: Main returned non-array value: %+v[#-]\n", mainRetvals)
            }
        }

        calltable[mainloc].gcShyness = 0
        calltable[mainloc].gc = false

    }

    // profiling summary
    if enableProfiling {
        dumpProfileSummary()
    }

    // a little paranoia to finish things off...
    setEcho(true)

    if runtime.GOOS != "windows" {
        term_complete()
    }

}
