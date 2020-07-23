
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
    "encoding/hex"
    "strconv"
    "regexp"
    "unicode/utf8"
    str "strings"
    "time"
)

var completions = []string{"ZERO", "INC", "DEC",
    "INIT", "PAUSE",
    "HELP", "NOP", "DEBUG", "REQUIRE", "EXIT", "VERSION",
    "QUIET", "LOUD", "UNSET", "INPUT", "PROMPT", "LOG", "PRINT", "PRINTLN",
    "LOGGING", "CLS", "AT", "DEFINE", "ENDDEF", "SHOWDEF", "RETURN",
    "MODULE", "USES", "WHILE", "ENDWHILE", "FOR", "FOREACH",
    "ENDFOR", "CONTINUE", "BREAK", "ON", "DO", "IF", "ELSE", "ENDIF", "WHEN",
    "IS", "CONTAINS", "IN", "OR", "ENDWHEN", "PANE",
    "TEST", "ENDTEST", "ASSERT",
}

const ansi = "[\u001B\u009B][[\\]()#;?]*(?:(?:(?:[a-zA-Z\\d]*(?:;[a-zA-Z\\d]*)*)?\u0007)|(?:(?:\\d{1,4}(?:;\\d{0,4})*)?[\\dA-PRZcf-ntqry=><~]))"

var ansiReplacables []string
var fairyReplacer *str.Replacer


/// clear n chars
func clearChars(row int,col int,l int) {
    at(row,col)
    fmt.Print(str.Repeat(" ",l))
}

func min(a, b int) int {
    if a < b {
        return a
    }    
    return b
}

func max(a, b int) int {
    if a > b {
        return a
    }    
    return b
}


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

/// apply ansi code translation to inbound strings
func sparkle(a string) string {
    a=fairyReplacer.Replace(a)
    return (a)
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

func rlen(s string) int {
    return utf8.RuneCountInString(s)
}

/// calculate on-console visible string length, allowing for hidden formatting
func displayedLen(s string) int {
    // remove ansi codes
    return rlen(Strip(sparkle(s)))
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

func sat(row int,col int) string {
    return sf("\033[%d;%dH", orow+row, ocol+col)
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
    if rlen(s)<pos { return s }
    return s[pos:]
}

/// remove character at position pos
func removeBefore(s string, pos int) string {
    if rlen(s)<pos { return s }
    if pos < 1 { return s }
    s = s[:pos-1] + s[pos:]
    return s
}

/// insert a number of characters in string at position pos
func insertBytesAt(s string, pos int, c []byte) string {
    if pos == rlen(s) { // append
        s += string(c)
        return s
    }
    s = s[:pos] + string(c) + s[pos:]
    return s
}

/// insert a single byte at position pos in string s
func insertAt(s string, pos int, c byte) string {
    if pos == rlen(s) { // append
        s += string(c)
        return s
    }
    s = s[:pos] + string(c) + s[pos:]
    return s
}

/// append a string to end of string or insert it mid-string
func insertWord(s string, pos int, w string) string {
    if pos >= rlen(s) { // append
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
    end := rlen(s)

    if end<pos { return s,0 }

    for p := pos - 1; p >= 0; p-- {
        if s[p] == ' ' {
            start = p
            cpos=p
            break
        }
    }
    if cpos==end { cpos-- }

    for p := pos; p < rlen(s)-1; p++ {
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
    if end < rlen(s)-1 {
        if start!=0 { add=" " }
        endsub = s[end:]
    }

    rstring := startsub+add+endsub

    return rstring,cpos
}

/// get the word in string s under the cursor (at position c)
func getWord(s string, c int) string {

    if rlen(s)<c {
        return s
    }

    // track back
    var i int
    i = rlen(s) - 1
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
    for j = c; j < rlen(s)-1; j++ {
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

func saveCursor() {
    fmt.Printf("\033[s")
}

func restoreCursor() {
    fmt.Printf("\033[u")
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

    CMDSEP,_:=vget(0,"@cmdsep")
    cmdsep:=CMDSEP.(byte)

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
                if s[len(s)-1] == cmdsep {
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
            if s[len(s)-1] == cmdsep {
                s = s[:len(s)-1]
            }
        }

        // skip null end marker strings
        if len(s) > 0 {
            if s[0] == cmdsep {
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

    rip,_:=vget(0,"@runInParent")
    if rip.(bool) {
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
        CMDSEP,_:=vget(0,"@cmdsep")
        cmdsep:=CMDSEP.(byte)
        hexenc:=hex.EncodeToString([]byte{cmdsep})
        io.WriteString(pi, line+` 2>`+errorFile.Name()+` ; last=$? ; echo -en "\x`+hexenc+`${last}\x`+hexenc+`"`+"\n")

        // get output
        ns, commandErr = NextCopper(line, read_out)

        // get status code - cmd is not important for this, NextCopper just reads
        //  the output until the next cmdsep
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
    if lockSafety { debuglock.RLock() }
    if debug_level >= level {
        pf(sparkle(s), va...)
    }
    if lockSafety { debuglock.RUnlock() }
    /*
    if debug_level==20 {
        plog(sparkle(s),va...)
    }
    */
}

func restoreScreen() {
    pf("\033c") // reset screen
    pf("\033[u")
}

func testStart(file string) {
    vos,_:=vget(0,"@os") ; stros:=vos.(string)
    test_start := sf("\n[#6][#underline][#bold]Za Test[#-]\n\nTesting : %s on "+stros+"\n", file)
    appendToTestReport(test_output_file,0, 0, test_start)
}

func testExit() {
    test_final := sf("\n[#6]Tests Performed %d -- Tests Failed %d -- Tests Passed %d[#-]\n\n", testsPassed+testsFailed, testsFailed, testsPassed)
    appendToTestReport(test_output_file,0, 0, test_final)
}


