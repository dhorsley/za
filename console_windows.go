// +build windows

package main

import (
    "bytes"
    "fmt"
    "syscall"
    "unsafe"
    "unicode/utf16"
    "sort"
    str "strings"
    "time"
)

func setEcho(s bool) {

    var mode uint32
    pMode := &mode
    procGetConsoleMode.Call(uintptr(syscall.Stdin), uintptr(unsafe.Pointer(pMode)))

    var echoMode uint32
    echoMode = 4

    if s {
        procSetConsoleMode.Call(uintptr(syscall.Stdin), uintptr(mode | echoMode))
    } else {
        procSetConsoleMode.Call(uintptr(syscall.Stdin), uintptr(mode &^ echoMode))
    }

}

func GetCursorPos() (int,int) {
    tcol,trow,e:=GetRowCol(1)
    if e==nil {
        return trow,tcol
    }
    return -1,-1
}

func term_complete() {
    // do nothing
}

/*
ENABLE_PROCESSED_INPUT          = 0x0001
ENABLE_LINE_INPUT               = 0x0002
ENABLE_ECHO_INPUT               = 0x0004
ENABLE_WINDOW_INPUT             = 0x0008
ENABLE_MOUSE_INPUT              = 0x0010
ENABLE_INSERT_MODE              = 0x0020
ENABLE_QUICK_EDIT_MODE          = 0x0040
ENABLE_EXTENDED_FLAGS           = 0x0080
ENABLE_VIRTUAL_TERMINAL_INPUT   = 0x0200
*/


func getch(timeo int) (b []byte,timeout bool) {

    var mode uint32
    pMode := &mode
    procGetConsoleMode.Call(uintptr(syscall.Stdin), uintptr(unsafe.Pointer(pMode)))

    var vtMode, echoMode uint32
    echoMode        = 4
    vtMode          = 0x0200

    waitInput      := vtMode
    nowaitInput    := vtMode

    echo, _ := vget(0,"@echo")
    if echo.(bool) {
        waitInput += echoMode
        nowaitInput += echoMode
    }

    if timeo==0 {
        procSetConsoleMode.Call(uintptr(syscall.Stdin), uintptr( waitInput ) )
    } else {
        procSetConsoleMode.Call(uintptr(syscall.Stdin), uintptr( nowaitInput ) )
    }

    line := make([]uint16, 3)
    pLine := &line[0]
    var n uint16

    c := make(chan []byte)
    closed:=false

    go func() {
        for ; ! closed ; {
            if timeo==0 {
                procReadConsole.Call(uintptr(syscall.Stdin), uintptr(unsafe.Pointer(pLine)), uintptr(len(line)), uintptr(unsafe.Pointer(&n)))
                if n>0 && !closed {
                    c <- []byte(string(utf16.Decode(line[:n])))
                    break
                }
            } else {
                n=0
                procPeekConsoleInput.Call(uintptr(syscall.Stdin),uintptr(unsafe.Pointer(pLine)),uintptr(len(line)),uintptr(unsafe.Pointer(&n)))
                if n>0 {
                    procReadConsole.Call(uintptr(syscall.Stdin), uintptr(unsafe.Pointer(pLine)), uintptr(len(line)), uintptr(unsafe.Pointer(&n)))
                    closed=true
                    c <- []byte(string(utf16.Decode(line[:n])))
                    break
                }
                if timeout { break }
            }
        }
    }()

    if timeo>0 {
        dur := time.Duration(timeo) * time.Microsecond
        select {
        case b = <-c:
            procFlushConsoleInputBuffer.Call(uintptr(syscall.Stdin))
        case <-time.After(dur):
            timeout=true
        }
    } else {
        select {
        case b = <-c:
        }
    }

    procSetConsoleMode.Call(uintptr(syscall.Stdin), uintptr(mode))

    return b,timeout
}


var modkernel32 *syscall.LazyDLL
var procSetConsoleMode *syscall.LazyProc
var procFlushConsoleInputBuffer *syscall.LazyProc
var procPeekConsoleInput *syscall.LazyProc
var procReadConsole *syscall.LazyProc
var procGetConsoleMode *syscall.LazyProc

/// setup the za->ansi mappings
func setupAnsiPalette() {

    modkernel32 = syscall.NewLazyDLL("kernel32.dll")
    procSetConsoleMode = modkernel32.NewProc("SetConsoleMode")
    procFlushConsoleInputBuffer = modkernel32.NewProc("FlushConsoleInputBuffer")
    procPeekConsoleInput = modkernel32.NewProc("PeekConsoleInputW")
    procReadConsole = modkernel32.NewProc("ReadConsoleW")
    procGetConsoleMode = modkernel32.NewProc("GetConsoleMode")

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
        fairydust["boff"] = "\033[22m"
        fairydust["-"] = "\033[0m"
        fairydust["#"] = "\033[49m"
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
        fairydust["CTE"] = "\033[0K"

        ansiReplacables=[]string{}

        for k,v := range fairydust {
            ansiReplacables=append(ansiReplacables,"[#"+k+"]")
            ansiReplacables=append(ansiReplacables,v)
        }

        fairyReplacer=str.NewReplacer(ansiReplacables...)

    } else {
        var ansiCodeList=[]string{"b0","b1","b2","b3","b4","b5","b6","b7","0","1","2","3","4","5","6","7","i1","i0",
                "default","underline","invert","bold","boff","-","#","bdefault","bblack","bred",
                "bgreen","byellow","bblue","bmagenta","bcyan","bbgray","bgray","bbred","bbgreen",
                "bbyellow","bbblue","bbmagenta","bbcyan","bwhite","fdefault","fblack","fred","fgreen",
                "fyellow","fblue","fmagenta","fcyan","fbgray","fgray","fbred","fbgreen","fbyellow",
                "fbblue","fbmagenta","fbcyan","fwhite","dim","blink","hidden","crossed","framed","CSI","CTE",
        }

        for _,c:= range ansiCodeList {
            fairydust[c]=""
        }

        ansiReplacables=[]string{}

        for k,v := range fairydust {
            ansiReplacables=append(ansiReplacables,"[#"+k+"]")
            ansiReplacables=append(ansiReplacables,v)
        }
        fairyReplacer=str.NewReplacer(ansiReplacables...)

    }
}

/// get an input string from stdin, in raw mode
func getInput(evalfs uint32, prompt string, pane string, row int, col int, pcol string, histEnable bool, hintEnable bool, mask string) (s string, eof bool, broken bool) {

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

    icol := col + dlen + globalPaneShiftLen // input (row,col)
    irow := row

    endLine := false // input complete?

    // print prompt
    at(row, col)
    pf(sprompt)

    at(irow, icol)

    // change input colour
    pf(sparkle(pcol))

    // get echo status
    echo,_:=vget(0,"@echo")
    if mask=="" { mask="*" }

    for {

        dispL := rlen(s)

        // show input
        at(irow, icol)
        clearToEOPane(irow, icol, 2+dispL+displayedLen(helpstring))
        if echo.(bool) {
            pf(s)
        } else {
            l:=rlen(s)
            pf(str.Repeat(mask,l))
        }
        pf(helpstring)

        // print cursor
        cx = (icol + cpos) % (p.w)
        cy = row + int(float64(icol+cpos)/float64(p.w))
        at(cy, cx)

        // get key stroke
        c,_ := getch(0)

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
                    if rlen(s)>0 { add=" " }
                    s = insertWord(s, newstart, add+helpList[0]+" ")
                    cpos = rlen(s)
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
            cpos = rlen(s)
            wordUnderCursor = getWord(s, cpos)


        case bytes.Equal(c, []byte{1}): // ctrl-a
            cpos = 0
            wordUnderCursor = getWord(s, cpos)

        case bytes.Equal(c, []byte{5}): // ctrl-e
            cpos = rlen(s)
            wordUnderCursor = getWord(s, cpos)

        case bytes.Equal(c, []byte{21}): // ctrl-u
            s = removeAllBefore(s, cpos)
            cpos = 0
            wordUnderCursor = getWord(s, cpos)
            clearToEOPane(irow, icol, dispL)

        // BS is 127 in VT MODE
        case bytes.Equal(c, []byte{127}): // windows backspace

            if startedContextHelp && rlen(helpstring) == 0 {
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
            if cpos < rlen(s) {
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
            if cpos < rlen(s) {
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
                    cpos = rlen(s)
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
                    cpos = rlen(s)
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
            cpos = rlen(s)
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
                for _, v := range ident[evalfs] {
                    if v.IName!="" {
                        varnames = append(varnames, v.IName)
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
                    if rlen(s)>0 { add=" " }
                    if newstart==-1 { newstart=0 }
                    s = insertWord(s, newstart, add+helpList[0]+" ")
                    cpos = rlen(s)
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
    if echo.(bool) { pf("%s", sparkle(recolour+StripCC(s)+"[#-]")) }

    return s, eof, broken
}

/// get a keypress
func wrappedGetCh(p int) (k int) {

    c,tout := getch(p)

    if !tout {

        if c != nil {
            switch {
            case bytes.Equal(c, []byte{3}):
                k = 3 // ctrl-c
            case bytes.Equal(c, []byte{4}):
                k = 4 // ctrl-d
            case bytes.Equal(c, []byte{13}):
                k = 13 // enter
            case bytes.Equal(c, []byte{0xc2, 0xa3}):
                k = 163 // £ 
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

    }    

    return k
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

func GetRowCol(fd int) (int, int, error) {
    stdoutHandle := getStdHandle(syscall.STD_OUTPUT_HANDLE)
    var info, e = GetConsoleScreenBufferInfo(stdoutHandle)

    if e != nil {
            return 0, 0, e
    }
    return int(info.CursorPosition.X), int(info.CursorPosition.Y), nil
}


