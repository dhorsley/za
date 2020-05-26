// +build windows

package main

import (
//     term "github.com/pkg/term"
    "bytes"
    "fmt"
    "io"
    "io/ioutil"
    "log"
    "os"
    "os/exec"
    "bufio"
    "errors"
    //    "golang.org/x/sys/unix"
    "strconv"
    // "path/filepath"
    "regexp"
    "syscall"
    "unsafe"
    "unicode/utf16"
    "sort"
    str "strings"
    "time"
)

var completions = []string{"ZERO", "INC", "DEC",
    "INIT", "INSTALL", "PUSH", "TRIGGER", "DOWNLOAD", "PAUSE",
    "HELP", "NOP", "DEBUG", "REQUIRE", "DEPENDS", "EXIT", "VERSION",
    "QUIET", "LOUD", "UNSET", "INPUT", "PROMPT", "INDENT", "LOG", "PRINT", "PRINTLN",
    "LOGGING", "CLS", "AT", "DEFINE", "ENDDEF", "SHOWDEF", "RETURN",
    "MODULE", "USES", "WHILE", "ENDWHILE", "FOR", "FOREACH",
    "ENDFOR", "CONTINUE", "BREAK", "ON", "DO", "IF", "ELSE", "ENDIF", "WHEN",
    "IS", "CONTAINS", "IN", "OR", "ENDWHEN", "PANE",
    "TEST", "ENDTEST", "ASSERT",
}

const ansi = "[\u001B\u009B][[\\]()#;?]*(?:(?:(?:[a-zA-Z\\d]*(?:;[a-zA-Z\\d]*)*)?\u0007)|(?:(?:\\d{1,4}(?:;\\d{0,4})*)?[\\dA-PRZcf-ntqry=><~]))"

/// generic vararg print handler. also moves cursor in interactive mode
func pf(s string, va ...interface{}) {

    s = sf(sparkle(s), va...)
    if interactive {
        c := str.Count(s, "\n")
        row += c
        col = 1
    }
    fmt.Print(s)
}


func getch() []byte {
    modkernel32 := syscall.NewLazyDLL("kernel32.dll")
    procReadConsole := modkernel32.NewProc("ReadConsoleW")
    procGetConsoleMode := modkernel32.NewProc("GetConsoleMode")
    procSetConsoleMode := modkernel32.NewProc("SetConsoleMode")

    var mode uint32
    pMode := &mode
    procGetConsoleMode.Call(uintptr(syscall.Stdin), uintptr(unsafe.Pointer(pMode)))

    var vtMode, echoMode, lineMode uint32
    echoMode = 4
    lineMode = 2
    vtMode   = 0x0200

    procSetConsoleMode.Call(uintptr(syscall.Stdin), uintptr(mode ^ (echoMode | lineMode | vtMode )))

    line := make([]uint16, 3)
    pLine := &line[0]
    var n uint16
    procReadConsole.Call(uintptr(syscall.Stdin), uintptr(unsafe.Pointer(pLine)), uintptr(len(line)), uintptr(unsafe.Pointer(&n)))

    b := []byte(string(utf16.Decode(line[:n])))

    procSetConsoleMode.Call(uintptr(syscall.Stdin), uintptr(mode))

    return b
}



/// setup the za->ansi mappings
func setupAnsiPalette() {
    if ansiMode {
        fairydust["b0"] = "\033[40m"
        fairydust["b1"] = "\033[44m"
        fairydust["b2"] = "\033[41m"
        fairydust["b3"] = "\033[45m"
        fairydust["b4"] = "\033[42m"
        fairydust["b5"] = "\033[46m"
        fairydust["b6"] = "\033[43m"
        fairydust["b7"] = "\033[107m"
        fairydust["0"] = "\033[30m"
        fairydust["1"] = "\033[94m"
        fairydust["2"] = "\033[91m"
        fairydust["3"] = "\033[95m"
        fairydust["4"] = "\033[92m"
        fairydust["5"] = "\033[96m"
        fairydust["6"] = "\033[93m"
        fairydust["7"] = "\033[97m"
        fairydust["i1"] = "\033[3m"
        fairydust["i0"] = "\033[23m"
        fairydust["default"] = "\033[0m"
        fairydust["underline"] = "\033[4m"
        fairydust["invert"] = "\033[7m"
        fairydust["bold"] = "\033[1m"
        fairydust["fakm"] = "\033[0m"
        fairydust["fakb"] = "\033[49m"
        fairydust["bdefault"] = "\033[49m"
        fairydust["bblack"] = "\033[40m"
        fairydust["bred"] = "\033[41m"
        fairydust["bgreen"] = "\033[42m"
        fairydust["byellow"] = "\033[43m"
        fairydust["bblue"] = "\033[44m"
        fairydust["bmagenta"] = "\033[45m"
        fairydust["bcyan"] = "\033[46m"
        fairydust["bbgray"] = "\033[47m"
        fairydust["bgray"] = "\033[100m"
        fairydust["bbred"] = "\033[101m"
        fairydust["bbgreen"] = "\033[102m"
        fairydust["bbyellow"] = "\033[103m"
        fairydust["bbblue"] = "\033[104m"
        fairydust["bbmagenta"] = "\033[105m"
        fairydust["bbcyan"] = "\033[106m"
        fairydust["bwhite"] = "\033[107m"
        fairydust["fdefault"] = "\033[39m"
        fairydust["fblack"] = "\033[30m"
        fairydust["fred"] = "\033[31m"
        fairydust["fgreen"] = "\033[32m"
        fairydust["fyellow"] = "\033[33m"
        fairydust["fblue"] = "\033[34m"
        fairydust["fmagenta"] = "\033[35m"
        fairydust["fcyan"] = "\033[36m"
        fairydust["fbgray"] = "\033[37m"
        fairydust["fgray"] = "\033[90m"
        fairydust["fbred"] = "\033[91m"
        fairydust["fbgreen"] = "\033[92m"
        fairydust["fbyellow"] = "\033[93m"
        fairydust["fbblue"] = "\033[94m"
        fairydust["fbmagenta"] = "\033[95m"
        fairydust["fbcyan"] = "\033[96m"
        fairydust["fwhite"] = "\033[97m"
        fairydust["dim"] = "\033[2m"
        fairydust["blink"] = "\033[5m"
        fairydust["hidden"] = "\033[8m"
        fairydust["crossed"] = "\033[9m"
        fairydust["framed"] = "\033[51m"
        fairydust["CSI"] = "\033["
    } else {
        var ansiCodeList=[]string{"b0","b1","b2","b3","b4","b5","b6","b7","0","1","2","3","4","5","6","7","i1","i0",
                "default","underline","invert","bold","fakm","fakb","bdefault","bblack","bred",
                "bgreen","byellow","bblue","bmagenta","bcyan","bbgray","bgray","bbred","bbgreen",
                "bbyellow","bbblue","bbmagenta","bbcyan","bwhite","fdefault","fblack","fred","fgreen",
                "fyellow","fblue","fmagenta","fcyan","fbgray","fgray","fbred","fbgreen","fbyellow",
                "fbblue","fbmagenta","fbcyan","fwhite","dim","blink","hidden","crossed","framed","CSI",
        }

        for _,c:= range ansiCodeList {
            fairydust[c]=""
        }
    }
}


/// apply ansi code translation to inbound strings
func sparkle(a string) string {
    a = str.Replace(a, "[#-]", "[#fakm]", -1)
    a = str.Replace(a, "[##]", "[#fakb]", -1)
    for k, v := range fairydust {
    a = str.Replace(a, "[#"+k+"]", v, -1)
    }
    return (a)
}


/// get an input string from stdin, in raw mode
func getInput(prompt string, pane string, row int, col int, pcol string, histEnable bool, hintEnable bool) (s string, eof bool, broken bool) {

    sprompt := sparkle(prompt)

    // calculate real prompt length after ansi codes applied.
    dlen := displayedLen(prompt)

    globalPaneShiftLen := 0

    // init
    p := panes[pane]
    cpos := 0                    // cursor pos as extent of printable chars from start
    var cx, cy int               // current cursor position (row:cy,col:cx)
    os := ""                     // original string before history navigation begins
    navHist := false             // currently navigating history entries?
    startedContextHelp := false  // currently displaying auto-completion options
    contextHelpSelected := false // final selection made during auto-completion?
    selectedStar := 0            // starting word position of the current selection during auto-completion
    var starMax int              // fluctuating maximum word position for the auto-completion selector
    wordUnderCursor := ""        // maintains a copy of the word currently under amendment
    var helpColoured []string    // populated (on TAB) list of auto-completion possibilities as displayed on console
    var helpList []string        // list of remaining possibilities governed by current input word
    var helpstring string        // final compounded output string including helpColoured components
    var varnames []string        // the list of possible variable names from the local context
    var funcnames []string       // the list of possible standard library functions
    // var unproc_files []string    // raw list of files in the current directory
    // var files []string           // list of file basenames in the current directory

    icol := col + dlen + globalPaneShiftLen // input (row,col)
    irow := row

    endLine := false // input complete?

    // print prompt
    at(row, col)
    pf(sprompt)

    at(irow, icol)

    // change input colour
    pf(sparkle(pcol))

    for {

        // show input
        at(irow, icol)
        modinp := str.Replace(s, "\x1f", "\033[E\033[G", -1)
        dispL := len(modinp)
        clearToEOPane(irow, icol, 2+dispL+displayedLen(helpstring))
        fmt.Print(modinp)

        pf(helpstring)

        // print cursor
        cx = (icol + cpos) % (p.w)
        cy = row + int(float64(icol+cpos)/float64(p.w))
        at(cy, cx)

        // get key stroke
        c := getch()

        // actions
        switch {

        case bytes.Equal(c, []byte{4}): // ctrl-d
            eof = true
            break

        case bytes.Equal(c, []byte{13}): // enter

            if startedContextHelp {
                contextHelpSelected = true
                clearToEOPane(irow, icol, dispL)
                helpstring = ""
                break
            }

            endLine = true
            s = str.Replace(s, "\x1f", "\x0a", -1)
            if s != "" {
                hist = append(hist, s)
                lastHist++
                histEmpty = false
            }
            break

        case bytes.Equal(c, []byte{32}): // space

            if startedContextHelp {
                contextHelpSelected = false
                startedContextHelp = false
                if len(helpList) == 1 {
                    var newstart int
                    s,newstart = deleteWord(s, cpos)
                    add:=""
                    if newstart==-1 { newstart=0 }
                    if len(s)>0 { add=" " }
                    s = insertWord(s, newstart, add+helpList[0]+" ")
                    cpos = len(s)
                    helpstring = ""
                }
                // break
            }

            // normal space input
            s = insertAt(s, cpos, 32) // c[0])
            cpos++
            wordUnderCursor = getWord(s, cpos)

        // FINE IN WINDOWS VT MODE
        case bytes.Equal(c, []byte{27,91,49,126}): // home // from showkey -a
            cpos = 0
            wordUnderCursor = getWord(s, cpos)

        // FINE IN WINDOWS VT MODE
        case bytes.Equal(c, []byte{27,91,52,126}): // end // from showkey -a
            cpos = len(s)
            wordUnderCursor = getWord(s, cpos)


        case bytes.Equal(c, []byte{1}): // ctrl-a
            cpos = 0
            wordUnderCursor = getWord(s, cpos)

        case bytes.Equal(c, []byte{5}): // ctrl-e
            cpos = len(s)
            wordUnderCursor = getWord(s, cpos)

        case bytes.Equal(c, []byte{21}): // ctrl-u
            s = removeAllBefore(s, cpos)
            cpos = 0
            wordUnderCursor = getWord(s, cpos)
            clearToEOPane(irow, icol, dispL)

        // BS is 127 in VT MODE
        case bytes.Equal(c, []byte{127}): // windows backspace

            if startedContextHelp && len(helpstring) == 0 {
                startedContextHelp = false
            }

            if cpos > 0 {
                s = removeBefore(s, cpos)
                cpos--
                wordUnderCursor = getWord(s, cpos)
                clearToEOPane(irow, icol, dispL)
            }

        // DEL is 126 in VT mode
        // case bytes.Equal(c, []byte{0x1B, 0x5B, 0x33, 0x7E}): // DEL
        case bytes.Equal(c, []byte{126}): // windows DEL
            if cpos < len(s) {
                s = removeBefore(s, cpos+1)
                wordUnderCursor = getWord(s, cpos)
                clearToEOPane(irow, icol, displayedLen(s))
            }

        // CURSOR KEYS ARE FINE IN WIN VT INPUT MODE

        case bytes.Equal(c, []byte{0x1B, 0x5B, 0x44}): // LEFT

            // add check for LEFT during auto-completion:
            if startedContextHelp {
                if selectedStar > 0 {
                    selectedStar--
                }
                break
            }

            // normal LEFT:
            if cpos > 0 {
                cpos--
            }
            wordUnderCursor = getWord(s, cpos)


        case bytes.Equal(c, []byte{0x1B, 0x5B, 0x43}): // RIGHT

            // add check for RIGHT during auto-completion:
            if startedContextHelp {
                if selectedStar < starMax {
                    selectedStar++
                }
                break
            }

            // normal RIGHT:
            if cpos < len(s) {
                cpos++
            }
            wordUnderCursor = getWord(s, cpos)


        case bytes.Equal(c, []byte{0x1B, 0x5B, 0x41}): // UP

            if p.w<displayedLen(s) && cpos>p.w {
                cpos-=p.w
                break
            }

            if histEnable {
                if !histEmpty {
                    if !navHist {
                        navHist = true
                        curHist = lastHist
                        os = s
                    }
                    if curHist > 0 {
                        curHist--
                        s = hist[curHist]
                    }
                    cpos = len(s)
                    wordUnderCursor = getWord(s, cpos)
                    if curHist != lastHist {
                        l := displayedLen(s)
                        clearToEOPane(irow, icol, l)
                    }
                }
            }

        case bytes.Equal(c, []byte{0x1B, 0x5B, 0x42}): // DOWN

            if displayedLen(s)>p.w && cpos<p.w {
                cpos+=p.w
                break
            }

            if histEnable {
                if navHist {
                    if curHist < lastHist-1 {
                        curHist++
                        s = hist[curHist]
                    } else {
                        s = os
                        navHist = false
                    }
                    cpos = len(s)
                    wordUnderCursor = getWord(s, cpos)
                    if curHist != lastHist {
                        l := displayedLen(s)
                        clearToEOPane(irow, icol, l)
                    }
                }
            }


        // HOME AND END ARE FINE IN WIN VT INPUT MODE
        case bytes.Equal(c, []byte{0x1B, 0x5B, 0x48}): // HOME
            cpos = 0
            wordUnderCursor = getWord(s, cpos)
        case bytes.Equal(c, []byte{0x1B, 0x5B, 0x46}): // END
            cpos = len(s)
            wordUnderCursor = getWord(s, cpos)

        case bytes.Equal(c, []byte{9}): // TAB

            // completion hinting setup
            if hintEnable && !startedContextHelp {

                varnames = nil
                funcnames = nil

                startedContextHelp = true
                helpstring = ""
                selectedStar = -1 // start is off the list so that RIGHT has to be pressed to activate.

                //.. add var names
                for _, v := range ident[lastfs] {
                    if v.iName!="" {
                        varnames = append(varnames, v.iName)
                    }
                }
                sort.Strings(varnames)

                //.. add functionnames
                for k, _ := range slhelp {
                    funcnames = append(funcnames, k)
                }
                sort.Strings(funcnames)

            }

        case bytes.Equal(c, []byte{0x1B, 0x5B, 0x5A}): // SHIFT-TAB

        // ignore list
        case bytes.Equal(c, []byte{0x1B, 0x5B, 0x35}): // pgup
        case bytes.Equal(c, []byte{0x1B, 0x5B, 0x36}): // pgdown
        case bytes.Equal(c, []byte{0x1B, 0x5B, 0x32}): // insert

        default:
            if len(c) == 1 {
                if c[0] > 32 {
                    s = insertAt(s, cpos, c[0])
                    cpos++
                    wordUnderCursor = getWord(s, cpos)
                    selectedStar = -1 // also reset the selector position for auto-complete
                }
            }
        }

        // completion hinting population

        helpstring = ""

        if startedContextHelp {

            // populate helpstring
            helpList = []string{}
            helpColoured = []string{}

            for _, v := range completions {
                if str.HasPrefix(str.ToLower(v), str.ToLower(wordUnderCursor)) {
                    helpColoured = append(helpColoured, "[#6]"+v+"[#-]")
                    helpList = append(helpList, v)
                }
            }

            for _, v := range varnames {
                if v!="" {
                    if str.HasPrefix(v, wordUnderCursor) {
                        helpColoured = append(helpColoured, "[#3]"+v+"[#-]")
                        helpList = append(helpList, v)
                    }
                }
            }

            for _, v := range funcnames {
                if str.HasPrefix(str.ToLower(v), str.ToLower(wordUnderCursor)) {
                    helpColoured = append(helpColoured, "[#5]"+v+"[#-]")
                    helpList = append(helpList, v+"()")
                }
            }

            //.. build display string

            helpstring = "   << [#bgray][#6]"

            for cnt, v := range helpColoured {
                starMax = cnt
                l := displayedLen(helpstring) + displayedLen(s) + icol
                if (l + displayedLen(v) + icol +4 ) > p.w {
                    if l > 3 {
                        helpstring += "..."
                    }
                    break
                } else {
                    if cnt == selectedStar {
                        helpstring += "[#bblue]*"
                    }
                    helpstring += v + " "
                }
            }

            helpstring += "[#-][##]"

        }

        if contextHelpSelected {
            if len(helpList)>0 {
                if selectedStar > -1 {
                    helpList = []string{helpList[selectedStar]}
                }
                if len(helpList) == 1 {
                    var newstart int
                    s,newstart = deleteWord(s, cpos)
                    add:=""
                    if len(s)>0 { add=" " }
                    if newstart==-1 { newstart=0 }
                    s = insertWord(s, newstart, add+helpList[0]+" ")
                    cpos = len(s)
                    l := displayedLen(s)
                    clearToEOPane(irow, icol, l)
                    helpstring = ""
                }
            }
            contextHelpSelected = false
            startedContextHelp = false
        }

        if eof || broken || endLine {
            break
        }

    } // input loop

    at(irow, icol)
    clearToEOPane(irow, icol, displayedLen(s))
    pf("%s", sparkle(recolour+StripCC(s)+"[#-]"))

    return s, eof, broken
}


/// get keypresses, filtering out undesired until a valid match found
func wrappedGetCh(p int) (i int) {

    var keychan chan int
    keychan = make(chan int, 1)

    go func() {
        var k int
        for {
            c := getch()
            if c != nil {
                switch {
                case bytes.Equal(c, []byte{3}):
                    k = 3 // ctrl-c
                case bytes.Equal(c, []byte{4}):
                    k = 4 // ctrl-d
                case bytes.Equal(c, []byte{13}):
                    k = 13 // enter
                case bytes.Equal(c, []byte{126}):
                    k = 126 // DEL
                case bytes.Equal(c, []byte{127}):
                    k = 127 // backspace
                default:
                    if len(c) == 1 {
                        if c[0] > 31 {
                            k = int(c[0])
                        }
                    }
                }
            }
            if k != 0 {
                keychan <- k
                break
            }
        }
        keychan <- 0
    }()

    select {
    case i = <-keychan:
    }

    return i
}


type (
    SHORT int16
    WORD  uint16

    SMALL_RECT struct {
        Left   SHORT
        Top    SHORT
        Right  SHORT
        Bottom SHORT
    }

    COORD struct {
        X SHORT
        Y SHORT
    }

    CONSOLE_SCREEN_BUFFER_INFO struct {
        Size              COORD
        CursorPosition    COORD
        Attributes        WORD
        Window            SMALL_RECT
        MaximumWindowSize COORD
    }
)

func checkError(r1, r2 uintptr, err error) error {
    // Windows APIs return non-zero to indicate success
    if r1 != 0 {
        return nil
    }

    // Return the error if provided, otherwise default to EINVAL
    if err != nil {
        return err
    }
    return syscall.EINVAL
}

func getStdHandle(stdhandle int) uintptr {
    handle, err := syscall.GetStdHandle(stdhandle)
    if err != nil {
        panic(fmt.Errorf("could not get standard io handle %d", stdhandle))
    }
    return uintptr(handle)
}

func GetConsoleScreenBufferInfo(handle uintptr) (*CONSOLE_SCREEN_BUFFER_INFO, error) {
    var info CONSOLE_SCREEN_BUFFER_INFO
    var kernel32DLL = syscall.NewLazyDLL("kernel32.dll")
    var getConsoleScreenBufferInfoProc = kernel32DLL.NewProc("GetConsoleScreenBufferInfo")
    if err := checkError(getConsoleScreenBufferInfoProc.Call(handle, uintptr(unsafe.Pointer(&info)), 0)); err != nil {
        return nil, err
    }
    return &info, nil
}

func GetWinInfo(fd int) (info *CONSOLE_SCREEN_BUFFER_INFO) {
    stdoutHandle := getStdHandle(syscall.STD_OUTPUT_HANDLE)
    info, _ = GetConsoleScreenBufferInfo(stdoutHandle)
    return info
}

func GetSize(fd int) (width, height int, err error) {

    stdoutHandle := getStdHandle(syscall.STD_OUTPUT_HANDLE)
    var info, e = GetConsoleScreenBufferInfo(stdoutHandle)

    if e != nil {
            return 0, 0, e
    }

    // we should be able to use Size.Y here, but get a nonsense
    // answer back most of the time. (probably to do with max
    // history size?)

    // so we calculate height based on the moving window size
    // in the history window instead.

    y:=int(info.Window.Bottom)-int(info.Window.Top)

    // return int(info.Size.X), int(info.Size.Y), nil
    return int(info.Size.X), y, nil

}

/*
// GetSize returns the dimensions of the given terminal.
func GetSize(fd int) (int, int, error) {
    return 0,0,nil
}
*/


/// END OF PLACE HOLDERS

/// logging output printer
func plog(s string, va ...interface{}) {

    // print if not silent logging
    if v, _ := vget(0, "@silentlog"); v.(bool) {
        pf(s, va...)
    }

    // also write to log file
    if loggingEnabled {
        f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
        if err != nil {
            log.Println(err)
        }
        defer f.Close()
        subj, _ := vget(0, "@logsubject")
        logger := log.New(f, subj.(string), log.LstdFlags)
        logger.Printf(s, va...)
    }

}

/// special case printing for global var interpolation
func gpf(s string) {
    pf("%s\n", spf(globalspace, s))
}

/// sprint with namespace
func spf(ns uint64, s string) string {
    s,_ = interpolate(ns, s,true)
    return sf("%v", sparkle(s))
}


/// clear screen
func cls() {
    if v, _ := vget(0, "@winterm"); !v.(bool) {
        pf("\033c")
    } else {
        pf("\033[2J")
    }
    at(1, 1)
}

func secScreen() {
    pf("\033[?1049h\033[H")
}

func priScreen() {
    pf("\033[?1049l")
}

/// search for pane by name and return its dimensions
func paneLookup(s string) (row int, col int, w int, h int, err error) {
    for p := range panes {
        q := panes[p]
        if s == p {
            return q.row, q.col, q.w, q.h, nil
        }
    }
    return 0, 0, 0, 0, nil
}

/// remove ansi codes from a string
func Strip(s string) string {
    var strip_re = regexp.MustCompile(ansi)
    return strip_re.ReplaceAllString(s, "")
}

/// remove za format codes from a string
func StripCC(s string) string {
    s = Strip(s)
    rs := []string{}
    for k, _ := range fairydust {
        rs = append(rs, sf("[#%v]", k))
        rs = append(rs, "")
    }
    rs = append(rs, "[#-]", "")
    rs = append(rs, "[##]", "")
    r := str.NewReplacer(rs...)
    return r.Replace(s)
}

/// calculate on-console visible string length, allowing for hidden formatting
func displayedLen(s string) int {
    // remove ansi codes
    return len(Strip(sparkle(s)))
}

/// move the console cursor
func absat(row int, col int) {
    if row < 0 {
        row = 0
    }
    if col < 0 {
        col = 0
    }
    pf("\033[%d;%dH", row, col)
}

/// move the console cursor
func at(row int, col int) {
    pf("\033[%d;%dH", orow+row, ocol+col)
}

/// clear to end of line
func clearToEOL() {
    pf("\033[0K")
}

/// show the console cursor
func showCursor() {
    pf("\033[?12l\033[?25h\033[?8h")
}

/// hide the console cursor
func hideCursor() {
    pf("\033[?8l\033[?25l\033[?12h")
}

func cursorX(n int) {
    pf("\033[%dG",n)
}

/// remove runes in string s before position pos
func removeAllBefore(s string, pos int) string {
    if len(s)<pos { return s }
    return s[pos:]
}

/// remove character at position pos
func removeBefore(s string, pos int) string {
    if len(s)<pos { return s }
    if pos < 1 { return s }
    s = s[:pos-1] + s[pos:]
    return s
}

/// insert a number of characters in string at position pos
func insertBytesAt(s string, pos int, c []byte) string {
    if pos == len(s) { // append
        s += string(c)
        return s
    }
    s = s[:pos] + string(c) + s[pos:]
    return s
}

/// insert a single byte at position pos in string s
func insertAt(s string, pos int, c byte) string {
    if pos == len(s) { // append
        s += string(c)
        return s
    }
    s = s[:pos] + string(c) + s[pos:]
    return s
}

/// append a string to end of string or insert it mid-string
func insertWord(s string, pos int, w string) string {
    if pos >= len(s) { // append
        s += w
        return s
    }
    s = s[:pos] + w + s[pos:]
    return s
}

/// delete the word under the cursor
func deleteWord(s string, pos int) (string,int) {

    start := 0
    cpos:=0
    end := len(s)

    if end<pos { return s,0 }

    for p := pos - 1; p >= 0; p-- {
        if s[p] == ' ' {
            start = p
            cpos=p
            break
        }
    }
    if cpos==end { cpos-- }

    for p := pos; p < len(s)-1; p++ {
        if s[p] == ' ' {
            end = p + 1
            break
        }
    }

    startsub := ""
    endsub := ""

    if start > 0 {
        startsub = s[:start]
    }

    add:=""
    if end < len(s)-1 {
        if start!=0 { add=" " }
        endsub = s[end:]
    }

    rstring := startsub+add+endsub

    return rstring,cpos
}

/// get the word in string s under the cursor (at position c)
func getWord(s string, c int) string {

    if len(s)<c {
        return s
    }

    // track back
    var i int
    i = len(s) - 1
    if c < i {
        i = c
    }
    if i < 0 {
        i = 0
    }
    for ; i > 0; i-- {
        if s[i] == ' ' {
            break
        }
    }
    if i == 0 {
        i = -1
    }

    // track forwards
    var j int
    for j = c; j < len(s)-1; j++ {
        if s[j] == ' ' {
            break
        }
    }

    // select word
    if j > i {
        return s[i+1 : j]
    }

    return ""

}


/// clear to end of current window pane
func clearToEOPane(row int, col int, va ...int) {
    p := panes[currentpane]
    // save cursor pos
    fmt.Printf("\033[s")
    // clear line
    if (len(va) == 1) && (va[0] > p.w) {
        lines := va[0] / (p.w - 1)
        for ; lines >= 0; lines-- {
            at(row+lines-1, 1)
            fmt.Print(rep(" ", int(p.w)))
        }
    } else {
        at(row, col)
        fmt.Print(rep(" ", int(p.w-col-1)))
    }
    // restore cursor pos
    fmt.Printf("\033[u")
}

func paneBox(c string) {

    p := panes[c]

    var tl, tr, bl, br, tlr, blr, ud string

    switch p.boxed {
    case "none":
        tl = " "
        tr = " "
        bl = " "
        br = " "
        tlr = " "
        blr = " "
        ud = " "
    case "rounddot":
        tl = "╭"
        tr = "╮"
        bl = "╰"
        br = "╯"
        tlr = "┈"
        blr = "┈"
        ud = "┊"
    case "round":
        tl = "╭"
        tr = "╮"
        bl = "╰"
        br = "╯"
        tlr = "─"
        blr = "─"
        ud = "│"
    case "square":
        tl = "┌"
        tr = "┐"
        bl = "└"
        br = "┘"
        tlr = "─"
        blr = "─"
        ud = "│"
    case "double":
        tl = "╔"
        tr = "╗"
        bl = "╚"
        br = "╝"
        tlr = "═"
        blr = "═"
        ud = "║"
    case "sparse":
        tl = "┏"
        tr = "┓"
        bl = "┗"
        br = "┛"
        tlr = " "
        blr = " "
        ud = " "
    case "topline":
        tl = "╞"
        tr = "╡"
        bl = " "
        br = " "
        tlr = "═"
        blr = " "
        ud = " "
    default:
        // pf("Box was : '%s'\n",p.boxed)
    }

    // corners
    absat(p.row, p.col)
    pf(tl)
    absat(p.row, p.col+p.w-1)
    pf(tr)
    absat(p.row+p.h, p.col+p.w-1)
    pf(br)
    absat(p.row+p.h, p.col)
    pf(bl)

    // top, bottom
    absat(p.row, p.col+1)
    pf(rep(tlr, int(p.w-2)))
    absat(p.row+p.h, p.col+1)
    pf(rep(blr, int(p.w-2)))

    // left, right
    for r := p.row + 1; r < p.row+p.h; r++ {
        absat(r, p.col)
        pf(ud)
        absat(r, p.col+p.w-1)
        pf(ud)
    }

    // title
    if p.title != "" {
        absat(p.row, p.col+3)
        pf(p.title)
    }

}

func rep(s string, i int) string {
    if i < 0 {
        i = 0
    }
    return str.Repeat(s, i)
}

func paneUnbox(c string) {
    bg := " "
    p := panes[c]
    absat(p.row, p.col)
    pf(bg)
    absat(p.row, p.col+p.w-1)
    pf(bg)
    absat(p.row+p.h, p.col+p.w-1)
    pf(bg)
    absat(p.row+p.h, p.col)
    pf(bg)
    absat(p.row, p.col+1)
    pf(rep(bg, int(p.w-2)))
    absat(p.row+p.h, p.col+1)
    pf(rep(bg, int(p.w-2)))
    for r := p.row + 1; r < p.row+p.h; r++ {
        absat(r, p.col)
        pf(bg)
        absat(r, p.col+p.w-1)
        pf(bg)
    }
}

func setPane(c string) {
    if p, ok := panes[c]; ok {
        orow = p.row
        ocol = p.col
        oh = p.h
        ow = p.w
    } else {
        pf("Pane '%s' not found! Ignoring.\n", c)
    }
}

// build-a-bash
func NewCoprocess(loc string) (process *exec.Cmd, pi io.WriteCloser, po io.ReadCloser, pe io.ReadCloser) {

    var err error

    process = exec.Command(loc)

    pi, err = process.StdinPipe()
    if err != nil {
        log.Fatal(err)
    }

    po, err = process.StdoutPipe()
    if err != nil {
        log.Fatal(err)
    }

    pe, err = process.StderrPipe()
    if err != nil {
        log.Fatal(err)
    }

    if err = process.Start(); err != nil {
        pf("Error: could not launch the coprocess.\n")
        os.Exit(ERR_NOBASH)
    }

    return process, pi, po, pe

}


// synchronous execution and capture
func GetCommand(c string) (string, error) {
    c=str.Trim(c," \t")
    bargs := str.Split(c, " ")
    cmd := exec.Command(bargs[0], bargs[1:]...)
    var out bytes.Buffer
    cmd.Stdout = &out
    err := cmd.Run()
    return out.String(), err
}

type BashRead struct {
    S string
    E error
}

// execute a command in the coprocess, return output.
func NextCopper(cmd string, r *bufio.Reader) (s string, err error) {

    var result BashRead

    siglock.Lock()
    coproc_active = true
    siglock.Unlock()

    c := make(chan BashRead)

    dur := time.Duration(MAX_TIO * time.Millisecond)
    t := time.NewTimer(dur)

    go func() {

        var err error
        var v byte

        // get char by char. if LF then reset timeout timer
        // otherwise poke it to end of output string
        // if EOF then end with what we have accumulated so far

        // save cursor - move to start of row
        mt, _ := vget(0, "mark_time")
        if mt.(bool) {
            pf("[#CSI]s[#CSI]1G")
        }

        for {

            v, err = r.ReadByte()

            if err == nil {
                s += string(v)
                if v == 10 {
                    if mt.(bool) {
                        pf("⟊")
                    }
                    t.Reset(dur)
                }
            }

            if err == io.EOF {
                if v != 0 {
                    s += string(v)
                }
                break
            }

            if len(s) > 0 {
                if s[len(s)-1] == 0x1e {
                    break
                }
                if !t.Stop() {
                    <-t.C
                }
                t.Reset(dur)
            }

        }

        // restore cursor
        if mt.(bool) {
            pf("[#CSI]u")
        }

        // remove trailing end marker
        if len(s) > 0 {
            if s[len(s)-1] == 0x1e {
                s = s[:len(s)-1]
            }
        }

        // skip null end marker strings
        if len(s) > 0 {
            if s[0] == 0x1e {
                s = ""
            }
        }

        c <- BashRead{S: s, E: err}

    }()

    select {
    case result = <-c:
    case _, closed := <-t.C:
        if !closed {
            result.E = errors.New("Command '" + cmd + "' timed-out.")
        }
    }

    close(c)

    siglock.Lock()
    coproc_active = false
    siglock.Unlock()

    return result.S, result.E

}


// submit a command for coprocess execution
func Copper(line string, squashErr bool) (string, int) {

    // line         command to execute
    // squashErr    ignore errors in output

    // remove some bad conditions...
    if str.HasSuffix(str.TrimRight(line," "),"|") {
        return "",-1
    }
    if tr(line,DELETE,"| ") == "" {
        return "",-1
    }

    var ns string  // output from coprocess
    var errint int // coprocess return code
    var err error  // generic error handle
    var commandErr error

    if runInParent {
        ns,err = GetCommand("cmd /c "+line)
        if err != nil {

            vset(0,"@last","0")
            vset(0,"@lastout",[]byte{0})

            if !squashErr {

                if exitError, ok := err.(*exec.ExitError); ok {
                    vset(0,"@last",sf("%v",exitError.ExitCode()))
                    vset(0, "@last_out", []byte(err.Error()))
                } else { // probably a command not found?
                    vset(0,"@last","1")
                    vset(0,"@last_out", []byte("Command not found."))
                }

            }

        } else {
            vset(0, "@last", "0")
            vset(0, "@last_out", []byte{0})
        }
    } else {

        errorFile, err := ioutil.TempFile("", "copper.*.err")
        if err != nil {
            log.Fatal(err)
        }
        defer os.Remove(errorFile.Name())

        read_out := bufio.NewReader(po)

        // issue command
        io.WriteString(pi, line+` 2>`+errorFile.Name()+` ; last=$? ; echo -en "\x1e${last}\x1e"`+"\n")

        // get output
        ns, commandErr = NextCopper(line, read_out)

        // get status code - cmd is not important for this, NextCopper just reads
        //  the output until the next 0x1e
        code, err := NextCopper("#Status", read_out)

        // pull cwd from /proc
        childProc,_:=vget(0,"@shellpid")
        pwd,_:=os.Readlink(sf("/proc/%v/cwd",childProc))
        vset(0, "@pwd", pwd)

        if commandErr != nil {
            errint = -3
        } else {
            if err == nil {
                errint, err = strconv.Atoi(code)
                if err != nil {
                    errint = -2
                }
                if !squashErr {
                    vset(0, "@last", code)
                }
            } else {
                errint = -1
            }
        }

        // get stderr file
        b, err := ioutil.ReadFile(errorFile.Name())

        if len(b) > 0 {
            vset(0, "@last_out", b)
        } else {
            vset(0, "@last_out", []byte{0})
        }

        os.Remove(errorFile.Name())

    }

    // remove trailing slash-n
    if len(ns) > 0 {
        for q := len(ns) - 1; q > 0; q-- {
            if ns[q] == '\n' {
                ns = ns[:q]
            } else {
                break
            }
        }
    }

    return ns, errint
}

func debug(level int, s string, va ...interface{}) {
    if debug_level >= level {
        pf(sparkle(s), va...)
    }
    if debug_level==20 {
        plog(sparkle(s),va...)
    }
}

func restoreScreen() {
    pf("\033c") // reset screen
    pf("\033[u")
}

func testStart(file string) {
    test_start := sf("\n[#6][#underline][#bold]Za Test[#-]\nTesting : %s\n", file)
    appendToTestReport(test_output_file,0, 0, test_start)
}

func testExit() {
    test_final := sf("\n[#6]Tests Performed %d -- Tests Failed %d -- Tests Passed %d[#-]\n\n", testsPassed+testsFailed, testsFailed, testsPassed)
    appendToTestReport(test_output_file,0, 0, test_final)
}

