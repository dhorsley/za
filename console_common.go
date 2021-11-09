
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

var completions = []string{"VAR", "SETGLOB", "PAUSE",
    "HELP", "NOP", "REQUIRE", "EXIT", "VERSION",
    "QUIET", "LOUD", "UNSET", "INPUT", "PROMPT", "LOG", "PRINT", "PRINTLN",
    "LOGGING", "CLS", "AT", "DEFINE", "SHOWDEF", "ENDDEF", "RETURN", "ASYNC",
    "MODULE", "USES", "WHILE", "ENDWHILE", "FOR", "FOREACH",
    "ENDFOR", "CONTINUE", "BREAK", "ON", "DO", "IF", "ELSE", "ENDIF", "WHEN",
    "IS", "CONTAINS", "HAS", "IN", "OR", "ENDWHEN", "WITH", "ENDWITH",
    "STRUCT", "ENDSTRUCT", "SHOWSTRUCT",
    "PANE", "DOC", "TEST", "ENDTEST", "ASSERT", "TO", "STEP", "AS", "ENUM", "HIST",
}

const ansi = "[\u001B\u009B][[\\]()#;?]*(?:(?:(?:[a-zA-Z\\d]*(?:;[a-zA-Z\\d]*)*)?\u0007)|(?:(?:\\d{1,4}(?:;\\d{0,4})*)?[\\dA-PRZcf-ntqry=><~]))"

var ansiReplacables []string
var fairyReplacer *str.Replacer


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

// row+col are globals
func printWithNLRespect(s string,p Pane) {
    var newStr str.Builder
    for i:=0; i<len(s); i++ {
        if col==p.w-1 {
            newStr.WriteString(sf("\n\033[%dG",ocol+1))
            col=1 ; row++
        }
        switch s[i] {
        case '\n':
            newStr.WriteString(sf("\n\033[%dG",ocol+1))
            col=1 ; row++
        default:
            newStr.WriteByte(s[i])
            col++
        }
    }
    fmt.Print(newStr.String())
}

// print with line wrap at non-global pane end
func printWithWrap(s string) {
    if currentpane!="global" {
        if p, ok := panes[currentpane]; ok {
             printWithNLRespect(s,p)
        } else {
            fmt.Print(s)
        }
    } else {
        fmt.Print(s)
    }
}

// generic vararg print handler. also moves cursor in interactive mode
func pf(s string, va ...interface{}) {

    s = sf(sparkle(s), va...)

    if interactive {
        if lineWrap {
            printWithWrap(s)
        } else {
            fmt.Print(s)
        }
        chpos:=0
        c:=col
        for ; chpos<len(s); c++ {
            if c%MW==0          { row++; c=0 }
            if s[chpos]=='\n'   { row++; c=0 }
            chpos++
        }
        // past:=row-(MH-BMARGIN) ; if past>1 { fmt.Printf("\033[%dS",past) }
        return
    }

    if lineWrap {
        printWithWrap(s)
        return
    }

    fmt.Print(s)

    // test row update:
        chpos:=0
        c:=col
        for ; chpos<len(s); c++ {
            if c%MW==0          { row++; c=0 }
            if s[chpos]=='\n'   { row++; c=0 }
            chpos++
        }
    // end test
}

// apply ansi code translation to inbound strings
func sparkle(a interface{}) string {
    switch a:=a.(type) {
    case string:
        return fairyReplacer.Replace(a)
    }
    return sf(`%v`,a)
}

// logging output printer
func plog(s string, va ...interface{}) {

    // print if not silent logging
    if v, _ := vget(0, &gident, "@silentlog"); v.(bool) {
        pf(s, va...)
    }

    // also write to log file
    if loggingEnabled {
        f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
        if err != nil {
            log.Println(err)
        }
        defer f.Close()
        subj, _ := vget(0, &gident, "@logsubject")
        logger := log.New(f, subj.(string), log.LstdFlags)
        logger.Printf(s, va...)
    }

}

// special case printing for global var interpolation
func gpf(s string) {
    pf("%s\n", spf(0, &gident, s))
}

// sprint with namespace
func spf(ns uint32, ident *[szIdent]Variable, s string) string {
    s = interpolate(ns,ident,s)
    return sf("%v", sparkle(s))
}

// clear screen
func cls() {
    if v, _ := vget(0, &gident, "@winterm"); !v.(bool) {
        pf("\033c")
    } else {
        pf("\033[2J")
    }
    at(1, 1)
}


// probably not used now...

// switch to secondary buffer
func secScreen() {
    pf("\033[?1049h\033[H")
}

// switch to primary buffer
func priScreen() {
    pf("\033[?1049l")
}


// search for pane by name and return its dimensions
func paneLookup(s string) (row int, col int, w int, h int, err error) {
    for p := range panes {
        q := panes[p]
        if s == p {
            return q.row, q.col, q.w, q.h, nil
        }
    }
    return 0, 0, 0, 0, nil
}

// remove ansi codes from a string
func Strip(s string) string {
    var strip_re = regexp.MustCompile(ansi)
    return strip_re.ReplaceAllString(s, "")
}

// remove za format codes from a string
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

// calculate on-console visible string length, allowing for hidden formatting
func displayedLen(s string) int {
    // remove ansi codes
    return rlen(Strip(sparkle(s)))
}

// move the console cursor
func absat(row int, col int) {
    atlock.Lock()
    if row < 0 {
        row = 0
    }
    if col < 0 {
        col = 0
    }
    atlock.Unlock()
    fmt.Printf("\033[%d;%dH", row, col)
}

// move the console cursor (relative to current pane origin [orow,ocol])
// orow+ocol are globals
func at(row int, col int) {
    fmt.Printf("\033[%d;%dH", orow+row, ocol+col)
}

// return ansi codes for moving the console cursor
func sat(row int,col int) string {
    return sf("\033[%d;%dH", orow+row, ocol+col)
}

// clear to end of line
func clearToEOL() {
    pf("\033[0K")
}

// show the console cursor
func showCursor() {
    pf("\033[?12l\033[?25h\033[?8h")
}

// hide the console cursor
func hideCursor() {
    pf("\033[?8l\033[?25l\033[?12h")
}

// move to horizontal cursor position n
func cursorX(n int) {
    pf("\033[%dG",n)
}

// remove runes in string s before position pos
func removeAllBefore(s string, pos int) string {
    if rlen(s)<pos { return s }
    return s[pos:]
}

// remove character at position pos
func removeBefore(s string, pos int) string {
    if rlen(s)<pos { return s }
    if pos < 1 { return s }
    s = s[:pos-1] + s[pos:]
    return s
}

// insert a number of characters in string at position pos
func insertBytesAt(s string, pos int, c []byte) string {
    if pos == rlen(s) { // append
        s += string(c)
        return s
    }
    s = s[:pos] + string(c) + s[pos:]
    return s
}

// insert a single byte at position pos in string s
func insertAt(s string, pos int, c byte) string {
    if pos >= rlen(s) { // append
        s += string(c)
        return s
    }
    s = s[:pos] + string(c) + s[pos:]
    return s
}

// append a string to end of string or insert it mid-string
func insertWord(s string, pos int, w string) string {
    if pos >= rlen(s) { // append
        s += w
        return s
    }
    s = s[:pos] + w + s[pos:]
    return s
}

// delete the word under the cursor
func deleteWord(s string, pos int) (string,int) {

    start:=0
    end := len(s)

    if end<pos { return s,0 }

    for p := pos - 1; p >= 0; p-- {
        if s[p]=='.' {
            start=p+1
            break
        }
        if s[p] == ' ' {
            start=p+1
            break
        }
    }

    for p := pos; p < len(s); p++ {
        if s[p] == ' ' || s[p]=='.' {
            end = p
            break
        }
    }

    startsub := ""
    endsub := ""

    if start > 0 {
        startsub = s[:start]
    }

    add:=""
    if end < len(s) {
        if start!=0 { add=" " }
        endsub = s[end+1:]
    }

    rstring := startsub+add+endsub

    return rstring,start
}

// get the word in string s under the cursor (at position c)
// using space or dot as separator
func getWord(s string, c int) (string,bool) {
    if rlen(s)<c { return s,false }
    dotted:=false

    // track back
    var i int
    i = rlen(s) - 1
    if c < i { i = c }
    if i < 0 { i = 0 }
    for ; i > 0; i-- {
        if i!=c && (s[i]==' ' || s[i]=='.') {
            if s[i]=='.' { dotted=true }
            break
        }
    }
    if i == 0 { i = -1 }

    // track forwards
    var j int
    for j = c; j < rlen(s)-1; j++ {
        if s[j] == ' ' || s[j]=='.' { break }
    }

    // select word
    if j > i { return s[i+1 : j],dotted }

    return "",dotted
}

// get the word in string s under the cursor (at position c)
// using only space as separator
func getWordStrict(s string, c int) string {
    if rlen(s)<c { return s }

    // track back
    var i int
    i = rlen(s) - 1
    if c < i { i = c }
    if i < 0 { i = 0 }
    for ; i > 0; i-- {
        if i!=c && s[i]==' ' {
            break
        }
    }
    if i == 0 { i = -1 }

    // track forwards
    var j int
    for j = c; j < rlen(s)-1; j++ {
        if s[j] == ' ' { break }
    }

    // select word
    if j > i { return s[i+1 : j] }

    return ""
}

func saveCursor() {
    fmt.Printf("\033[s")
}

func restoreCursor() {
    fmt.Printf("\033[u")
}

// clear to end of current window pane
func clearToEOPane(row int, col int, va ...int) {
    p := panes[currentpane]
    // save cursor pos
    fmt.Printf("\033[s")
    // clear line
    if (len(va) == 1) && (va[0] > p.w) {
        lines := va[0] / (p.w - 1)
        for ; lines >= 0; lines-- {
            at(row+lines-1, 1)
            fmt.Print(rep(" ",p.w-2))
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
    // pf(tl)
    fmt.Print(tl)
    absat(p.row, p.col+p.w-1)
    // pf(tr)
    fmt.Print(tr)
    absat(p.row+p.h, p.col+p.w-1)
    // pf(br)
    fmt.Print(br)
    absat(p.row+p.h, p.col)
    // pf(bl)
    fmt.Print(bl)

    // top, bottom
    absat(p.row, p.col+1)
    // pf(rep(tlr, int(p.w-2)))
    fmt.Print(rep(tlr, int(p.w-2)))
    absat(p.row+p.h, p.col+1)
    // pf(rep(blr, int(p.w-2)))
    fmt.Print(rep(blr, int(p.w-2)))

    // left, right
    for r := p.row + 1; r < p.row+p.h; r++ {
        absat(r, p.col)
        // pf(ud)
        fmt.Print(ud)
        absat(r, p.col+p.w-1)
        // pf(ud)
        fmt.Print(ud)
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
        atlock.Lock()
        orow = p.row
        ocol = p.col
        oh = p.h
        ow = p.w
        atlock.Unlock()
    } else {
        pf("Pane '%s' not found! Ignoring.\n", c)
    }
}

// build-a-bash
func NewCoprocess(loc string,args ...string) (process *exec.Cmd, pi io.WriteCloser, po io.ReadCloser, pe io.ReadCloser) {

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
func GetCommand(c string) (s string, err error) {
    c=str.Trim(c," \t\n")
    bargs := str.Split(c, " ")
    cmd := exec.Command(bargs[0], bargs[1:]...)
    var out bytes.Buffer
    cmd.Stdin  = os.Stdin
    // cmd.Stderr = os.Stderr
    capture,_:=vget(0,&gident,"@commandCapture")
    if capture.(bool) {
        cmd.Stdout = &out
        err = cmd.Run()
    } else {
        cmd.Stdout = os.Stdout
        err := cmd.Run()
        return "", err
    }
    return out.String(), err
}


type BashRead struct {
    S []byte
    E error
}

// execute a command in the coprocess, return output.
func NextCopper(cmd string, r *bufio.Reader) (s []byte, err error) {

    var result BashRead

    CMDSEP,_:=vget(0,&gident,"@cmdsep")
    cmdsep:=CMDSEP.(byte)

    lastlock.Lock()
    coproc_active = true
    lastlock.Unlock()

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
        mt, _ := vget(0,&gident, "mark_time")
        if mt.(bool) {
            pf("[#CSI]s[#CSI]1G")
        }

        for {

            v, err = r.ReadByte()

            if err == nil {
                s = append(s,v)
                if v == 10 {
                    if mt.(bool) {
                        pf("⟊")
                    }
                    t.Reset(dur)
                }
            }

            if err == io.EOF {
                if v != 0 {
                    s = append(s,v)
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
                s = []byte{}
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

    lastlock.Lock()
    coproc_active = false
    lastlock.Unlock()

    return result.S, result.E

}


// submit a command for coprocess execution
func Copper(line string, squashErr bool) struct{out string; err string; code int; okay bool} {

    if !permit_shell {
        panic(fmt.Errorf("Shell calls not permitted!"))
    }

    // remove some bad conditions...
    if str.HasSuffix(str.TrimRight(line," "),"|") {
        return struct{out string;err string;code int;okay bool}{"","",-1,false}
    }
    if tr(line,DELETE,"| ","") == "" {
        return struct{out string;err string;code int;okay bool}{"","",-1,false}
    }
    line=str.TrimRight(line,"\n")

    var ns []byte
    var errout string   // stderr output
    var errint int      // coprocess return code
    var err error       // generic error handle
    var commandErr error

    riwp,_:=vget(0,&gident,"@runInWindowsParent")
    rip,_ :=vget(0,&gident,"@runInParent")


    // shell reporting option:
    sr,_:=vget(0,&gident,"@shell_report")

    if sr.(bool)==true {
        noshell,_  :=vget(0,&gident,"@noshell")
        shelltype,_:=vget(0,&gident,"@shelltype")
        shellloc,_ :=vget(0,&gident,"@shell_location")
        if !noshell.(bool) {
            pf("[#4]Shell Options: ")
            pf("%v (%v) ",shelltype,shellloc)
            if riwp.(bool) { pf("Windows ") }
            if rip.(bool)  {
                pf("in parent\n[#-]")
            } else {
                pf("in coproc\n[#-]")
            }
        }
    }

    if riwp.(bool) || rip.(bool) {

        if riwp.(bool) {
            var ba string
            ba,err = GetCommand("cmd /c "+line)
            ns = []byte(ba)
        } else {
            var ba string
            ba,err = GetCommand(line)
            ns = []byte(ba)
        }

        if err != nil {

            vset(0,&gident,"@last","0")
            vset(0,&gident,"@lastout",[]byte{0})

            if !squashErr {

                if exitError, ok := err.(*exec.ExitError); ok {
                    vset(0,&gident,"@last",sf("%v",exitError.ExitCode()))
                    vset(0,&gident, "@last_out", []byte(err.Error()))
                } else { // probably a command not found?
                    vset(0,&gident,"@last","1")
                    vset(0,&gident,"@last_out", []byte("Command not found."))
                }

            }

        } else {
            vset(0,&gident,"@last", "0")
            vset(0,&gident,"@last_out", []byte{0})
        }
    } else {

        errorFile, err := ioutil.TempFile("", "copper.*.err")
        if err != nil {
            os.Remove(errorFile.Name())
            log.Fatal(err)
        }
        defer os.Remove(errorFile.Name())
        vset(0,&gident,"@last", "0")

        read_out := bufio.NewReader(po)

        // issue command
        CMDSEP,_:=vget(0,&gident,"@cmdsep")
        cmdsep:=CMDSEP.(byte)
        hexenc:=hex.EncodeToString([]byte{cmdsep})
        io.WriteString(pi, line+` 2>`+errorFile.Name()+` ; last=$? ; echo -en "\x`+hexenc+`${last}\x`+hexenc+`"`+"\n")

        // get output
        ns, commandErr = NextCopper(line, read_out)
        // pf("[copper] line -> <%s>\n",line)
        // pf("[copper] ns   -> <%s>\n",ns)

        // get status code - cmd is not important for this, NextCopper just reads
        //  the output until the next cmdsep
        code, err := NextCopper("#Status", read_out)
        // pull cwd from /proc
        childProc,_:=vget(0,&gident,"@shellpid")
        pwd,_:=os.Readlink(sf("/proc/%v/cwd",childProc))
        vset(0,&gident,"@pwd", pwd)

        if commandErr != nil {
            errint = -3
        } else {
            if err == nil {
                errint, err = strconv.Atoi(string(code))
                if err != nil {
                    errint = -2
                }
                if !squashErr {
                    vset(0,&gident,"@last", string(code))
                }
            } else {
                errint = -1
            }
        }

        // get stderr file
        b, err := ioutil.ReadFile(errorFile.Name())

        if len(b) > 0 {
            vset(0,&gident, "@last_out", b)
            errout=string(b)
        } else {
            vset(0,&gident,"@last_out", []byte{0})
            errout=""
        }

        // os.Remove(errorFile.Name())

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

    return struct{out string;err string;code int;okay bool}{string(ns),errout,errint,errint==0}
}

func restoreScreen() {
    pf("\033c") // reset screen
    pf("\033[u")
}

func testStart(file string) {
    vos,_:=vget(0,&gident,"@os") ; stros:=vos.(string)
    test_start := sf("\n[#6][#ul][#bold]Za Test[#-]\n\nTesting : %s on "+stros+"\n", file)
    appendToTestReport(test_output_file,0, 0, test_start)
}

func testExit() {
    test_final := sf("\n[#6]Tests Performed %d -- Tests Failed %d -- Tests Passed %d[#-]\n\n", testsPassed+testsFailed, testsFailed, testsPassed)
    appendToTestReport(test_output_file,0, 0, test_final)
}


