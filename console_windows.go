// +build windows

package main

import (
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
    //     "sort"
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


/// PLACE HOLDERS ON WINDOWS

func getInput(prompt string, pane string, row int, col int, pcol string, histEnable bool, hintEnable bool) (s string, eof bool, broken bool) {
    return "",true,false
}

func wrappedGetCh(p int) (i int) {
    return -1
}

// GetSize returns the dimensions of the given terminal.
func GetSize(fd int) (int, int, error) {
    return 0,0,nil
}



/// END OF PLACE HOLDERS

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
    s = interpolate(ns, s)
    return sf("%v", sparkle(s))
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

