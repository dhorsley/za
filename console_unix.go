// +build !windows

package main

import (
    "bytes"
    "fmt"
    term "github.com/pkg/term"
    "golang.org/x/sys/unix"
    "sort"
    str "strings"
    "time"
)


func setEcho(s bool) {
    if s {
        enableEcho()
    } else {
        disableEcho()
    }
}

func disableEcho() {
    termios, err := unix.IoctlGetTermios(0, ioctlReadTermios)
	if err == nil {
        newState := *termios
        newState.Lflag |= unix.ICANON | unix.ISIG
        newState.Iflag |= unix.ICRNL
        newState.Lflag &^= unix.ECHO
	    unix.IoctlSetTermios(0, ioctlWriteTermios, &newState)
    }
}

func enableEcho() {
    termios, err := unix.IoctlGetTermios(0, ioctlReadTermios)
	if err == nil {
        newState := *termios
        newState.Lflag |= unix.ICANON | unix.ISIG
        newState.Iflag |= unix.ICRNL
        newState.Lflag |= unix.ECHO
	    unix.IoctlSetTermios(0, ioctlWriteTermios, &newState)
    }
}

func term_complete() {
    if tt!=nil {
        tt.Restore()
        tt.Close()
    }
}

// not on linux:
func GetWinInfo(fd int) (i int) {
    return -1
}

/// get keypresses, filtering out undesired until a valid match found
func wrappedGetCh(p int) (i int) {

    var keychan chan int
    keychan = make(chan int, 1)

    go func() {
        var k int
        for {
            c, tout, pasted, _ := getch(p)
            if tout {
                break
            }
            if pasted {
                break
            }
            if c != nil {
                switch {
                case bytes.Equal(c, []byte{2}):
                    k = 2 // ctrl-b
                case bytes.Equal(c, []byte{3}):
                    k = 3 // ctrl-c
                case bytes.Equal(c, []byte{4}):
                    k = 4 // ctrl-d
                case bytes.Equal(c, []byte{13}):
                    k = 13 // enter
                case bytes.Equal(c, []byte{0xc2, 0xa3}): // 194 163
                    k = 163
                case bytes.Equal(c, []byte{127}):
                    k = 127 // backspace
                case bytes.Equal(c, []byte{27,91,53,126}): // pgup
                    k = 15 // replaces Shift In (SI)
                case bytes.Equal(c, []byte{27,91,54,126}): // pgdown
                    k = 14 // replaces Shift Out (SO)
                case bytes.Equal(c, []byte{0x1B, 0x5B, 0x42}): // DOWN
                    k = 10
                case bytes.Equal(c, []byte{0x1B, 0x5B, 0x41}): // UP
                    k = 11
                case bytes.Equal(c, []byte{0x1B, 0x5B, 0x44}): // LEFT
                    k = 8
                case bytes.Equal(c, []byte{0x1B, 0x5B, 0x43}): // RIGHT
                    k = 9
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
        fairydust["ul"] = "\033[4m"
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
                "default","underline","ul","invert","bold","boff","-","#","bdefault","bblack","bred",
                "bgreen","byellow","bblue","bmagenta","bcyan","bbgray","bgray","bbred","bbgreen",
                "bbyellow","bbblue","bbmagenta","bbcyan","bwhite","fdefault","fblack","fred","fgreen",
                "fyellow","fblue","fmagenta","fcyan","fbgray","fgray","fbred","fbgreen","fbyellow",
                "fbblue","fbmagenta","fbcyan","fwhite","dim","blink","hidden","crossed","framed","CSI","CTE",
        }

        ansiReplacables=[]string{}

        for _,c:= range ansiCodeList {
            fairydust[c]=""
        }

        for k,v := range fairydust {
            ansiReplacables=append(ansiReplacables,"[#"+k+"]")
            ansiReplacables=append(ansiReplacables,v)
        }
        fairyReplacer=str.NewReplacer(ansiReplacables...)

    }
}


// race condition, yes... but who arranges concurrent keyboard access?
var bigbytelist = make([]byte,6*4096)


// get a key press
func getch(timeo int) ( []byte, bool, bool, string ) {

    term.RawMode(tt)

    tt.SetOption(term.ReadTimeout(time.Duration(timeo) * time.Microsecond))
    numRead, err := tt.Read(bigbytelist)

    tt.Restore()

    // deal with mass input (pasting?)
    if numRead>6 {
        return []byte{0},false,true,string(bigbytelist[0:numRead])
    }

    // numRead can be up to 6 chars for special input stroke.

    if err != nil {
        // treat as timeout.. separate later, but timeout is buried in here
        return nil, true, false, ""
    }
    return bigbytelist[0:numRead], false, false, ""
}


// @todo: deprecate GetCursorPos() soon
// @note: don't use this if you can avoid it. better to track the cursor yourself
// than rely on this if you require even modest performance. reads the cursor
// position from the vt console itself using output commands. of course, speed is
// also externally dependant upon the vt emulation of the terminal software the
// program is executed within!

func GetCursorPos() (int,int) {

    if tt==nil {
        return 0,0
    }

    buf:=make([]byte,15,15)
    var r,c int

    term.RawMode(tt)

    tt.Write([]byte("\033[6n"))

    n,_:=tt.Read(buf)

    if n>0 {
        endpos:=str.IndexByte(string(buf),'R')
        if endpos==-1 {
            r=-1; c=-1
        } else {
            op:=string(buf[2:endpos])
            parts:=str.Split(op,";")
            r,_=GetAsInt(parts[0])
            c,_=GetAsInt(parts[1])
        }
    }

    tt.Restore()

    return r,c

}


// getInput() : get an input string from stdin, in raw mode
//  it does have some issues with utf8 input when moving the cursor around.
//  not likely to fix this unless it annoys me too much.. more likely to
//  replace the input mechanism wholesale.
//  the issue is basically that we are not tracking where the code points start
//  for each char and moving the cursor to those instead of byte by byte.

func getInput(prompt string, pane string, row int, col int, pcol string, histEnable bool, hintEnable bool, mask string) (s string, eof bool, broken bool) {

    old_wrap := lineWrap
    lineWrap = false

    sprompt := sparkle(prompt)

    // calculate real prompt length after ansi codes applied.
    // init
    cpos := 0                    // cursor pos as extent of printable chars from start
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
    var funcnames []string       // the list of possible standard library functions

    // get echo status
    echo,_:=vget(0,&gident,"@echo")
    // pf("echo status is [%v]\n",echo)

    if mask=="" { mask="*" }

    endLine := false // input complete?

    at(row,col)

    var srow,scol int // the start of input, including prompt position
    var irow,icol int // current start of input position

    irow=srow
    lastsrow:=row

    for {

        // calc new values for row,col
        srow=row; scol=col
        promptL := displayedLen(sprompt)
        inputL  := displayedLen(s)
        dispL   :=promptL+inputL

        // move start row back if multiline at bottom of window
        // @note: MH and MW are globals which may change during a SIGWINCH event.
        rowLen:=int(dispL-1)/MW
        if srow>MH-BMARGIN { srow=MH-BMARGIN }
        if srow==MH-BMARGIN { srow=srow-rowLen }
        if lastsrow!=srow {
            m1:=min(lastsrow,srow)
            m2:=max(lastsrow,srow)
            for r:=m2; r>m1; r-- {
                at(r,col); clearToEOL()
            }
        }
        lastsrow=srow

        // print prompt
        at(srow, scol)
        pf(sprompt)

        irow=srow+(int(scol+promptL-1)/MW)
        icol=((scol+promptL-1)%MW)+1

        // change input colour
        pf(sparkle(pcol))

        cursAtCol:=((icol+inputL-1)%MW)+1
        rowLen=int(icol+inputL-1)/MW

        // show input
        at(irow, icol)
        if echo.(bool) {
            fmt.Print(s)
        } else {
            pf(str.Repeat(mask,inputL))
        }
        clearToEOL()
        at(irow+1,1); pf(helpstring); clearToEOL()

        // move cursor to correct position (cpos)
        if irow==MH-BMARGIN && cursAtCol==1 { srow--; rowLen++; pf("\n\033M") }
        cposCursAtCol:=((icol+cpos-1)%MW)+1
        cposRowLen:=int(icol+cpos-1)/MW
        at(srow+cposRowLen, cposCursAtCol)

        // get key stroke
        c, _ , pasted, pbuf := getch(0)

        if pasted {

            // we disallow multi-line pasted input. this is only a line editor.
            // no need to get fancy.

            // get paste buffer up to first eol
            eol:=str.IndexByte(pbuf,'\r')       // from hazy memories... vte paste marks line breaks with a single CR
            alt_eol:=str.IndexByte(pbuf,'\n')   // just in case i didn't remember right...

            if eol!=-1 {
                pbuf=pbuf[:eol]
            }

            if alt_eol!=-1 {
                pbuf=pbuf[:alt_eol]
            }

            // strip ansi codes from pbuf then shove it in the input string
            s = insertWord(s, cpos, Strip(pbuf))
            cpos+=len(pbuf)
            wordUnderCursor = getWord(s, cpos)
            selectedStar = -1

        } else {

            switch {

            case bytes.Equal(c, []byte{3}): // ctrl-c
                broken = true
                break
            case bytes.Equal(c, []byte{4}): // ctrl-d
                eof = true
                break
            case bytes.Equal(c, []byte{13}): // enter

                if startedContextHelp {
                    contextHelpSelected = true
                    clearChars(irow, icol, inputL)
                    for i:=irow+1;i<=irow+BMARGIN;i+=1 { at(i,1); clearToEOL() }
                    helpstring = ""
                    break
                }

                endLine = true

                if s != "" {
                    if len(hist)==0 || (len(hist)>0 && s!=hist[len(hist)-1]) {
                        hist = append(hist, s)
                        lastHist++
                        histEmpty = false
                    }
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
                        if len(s)>0 { add=" " }
                        if newstart==-1 { newstart=0 }
                        s = insertWord(s, newstart, add+helpList[0]+" ")
                        cpos = len(s)
                        for i:=irow+1;i<=irow+BMARGIN;i+=1 { at(i,1); clearToEOL() }
                    }
                    helpstring = ""
                }

                // normal space input
                s = insertAt(s, cpos, c[0])
                cpos++
                wordUnderCursor = getWord(s, cpos)

            case bytes.Equal(c, []byte{27,91,49,126}): // home // from showkey -a
                cpos = 0
                wordUnderCursor = getWord(s, cpos)

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
                clearChars(irow, icol, inputL)

            case bytes.Equal(c, []byte{127}): // backspace

                if startedContextHelp && len(helpstring) == 0 {
                    startedContextHelp = false
                    helpstring=""
                }

                for i:=irow+1;i<=irow+BMARGIN;i+=1 { at(i,1); clearToEOL() }

                if cpos > 0 {
                    s = removeBefore(s, cpos)
                    cpos--
                    wordUnderCursor = getWord(s, cpos)
                    clearChars(irow, icol, inputL)
                }

            case bytes.Equal(c, []byte{0x1B, 0x5B, 0x33, 0x7E}): // DEL
                if cpos < len(s) {
                    s = removeBefore(s, cpos+1)
                    wordUnderCursor = getWord(s, cpos)
                    clearChars(irow, icol, displayedLen(s))
                }

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

                if MW<displayedLen(s) && cpos>MW {
                    cpos-=MW
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
                        rowLen=int(icol+cpos-1)/MW
                        if rowLen>0 { irow-=rowLen }
                        if curHist != lastHist {
                        }
                    }
                }

            case bytes.Equal(c, []byte{0x1B, 0x5B, 0x42}): // DOWN

                if displayedLen(s)>MW && cpos<MW {
                    cpos+=MW
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
                            clearChars(irow, icol, l)
                        }
                    }
                }

            case bytes.Equal(c, []byte{0x1B, 0x5B, 0x48}): // HOME
                cpos = 0
                wordUnderCursor = getWord(s, cpos)
            case bytes.Equal(c, []byte{0x1B, 0x5B, 0x46}): // END
                cpos = len(s)
                wordUnderCursor = getWord(s, cpos)

            case bytes.Equal(c, []byte{9}): // TAB

                // completion hinting setup
                if hintEnable && !startedContextHelp {

                    funcnames = nil

                    startedContextHelp = true
                    for i:=irow+1;i<=irow+BMARGIN;i++ { at(i,1); clearToEOL() }
                    helpstring = ""
                    selectedStar = -1 // start is off the list so that RIGHT has to be pressed to activate.

                    //.. add functionnames
                    for k, _ := range slhelp {
                        funcnames = append(funcnames, k)
                    }
                    sort.Strings(funcnames)

                }

            case bytes.Equal(c, []byte{0x1B, 0x5B, 0x5A}): // SHIFT-TAB

            case bytes.Equal(c, []byte{0x1B, 0x63}): // alt-c
            case bytes.Equal(c, []byte{0x1B, 0x76}): // alt-v

            // specials over 128 - don't do this.. too messy with runes.

            case bytes.Equal(c, []byte{0xc2, 0xa3}): // £  194 163
                s = insertAt(s, cpos, '£')
                cpos++
                wordUnderCursor = getWord(s, cpos)
                selectedStar = -1

            // ignore list
            case bytes.Equal(c, []byte{0x1B, 0x5B, 0x35}): // pgup
            case bytes.Equal(c, []byte{0x1B, 0x5B, 0x36}): // pgdown
            case bytes.Equal(c, []byte{0x1B, 0x5B, 0x32}): // insert

            default:
                if len(c) == 1 {
                    if c[0] > 32 && c[0]<128 {
                        s = insertAt(s, cpos, c[0])
                        cpos++
                        wordUnderCursor = getWord(s, cpos)
                        selectedStar = -1 // also reset the selector position for auto-complete
                    }
                }

                // @todo(dh): this is lazy. clears context help after an input change
                //   needs improving throughout.
                for i:=irow+1;i<=irow+BMARGIN;i+=1 { at(i,1); clearToEOL() }
            }

        } // paste or char input end


        // completion hinting population

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

            /*
            for _, v := range varnames {
                if v!="" {
                    if str.HasPrefix(v, wordUnderCursor) {
                        helpColoured = append(helpColoured, "[#3]"+v+"[#-]")
                        helpList = append(helpList, v)
                    }
                }
            }
            */

            for _, v := range funcnames {
                if str.HasPrefix(str.ToLower(v), str.ToLower(wordUnderCursor)) {
                    helpColoured = append(helpColoured, "[#5]"+v+"[#-]")
                    helpList = append(helpList, v+"()")
                }
            }


            //.. build display string

            helpstring = "help> [#bgray][#6]"

            for cnt, v := range helpColoured {
                starMax = cnt
                l := displayedLen(helpstring) + displayedLen(s) + icol
                if (l + displayedLen(v) + icol + 4) > MW {
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

            // don't show desc+function help if current word is a keyword instead of function.
            //   otherwise, find desc+func for either remaining guess in context list 
            //   or the current word.

            if len(helpList)>0 {
                if _,found:=keywordset[helpList[0]]; !found {
                    pos:=0
                    if len(helpList)>1 {
                        // show of desc+function help if current word completes a function (but still other completion options)
                        for p,v:=range helpList {
                            if wordUnderCursor==v {
                                pos=p
                                break
                            }
                        }
                    }
                    hla:=helpList[pos]
                    hla=hla[:len(hla)-2]
                    helpstring+="\n[#bold]"+hla+"("+slhelp[hla].in+")[#boff] : [#4]"+slhelp[hla].action+"[#-]"
                }
            }

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
                    s = insertWord(s, newstart, add+helpList[0]) // +" ")
                    if bpos:=str.IndexByte(s,'('); bpos!=-1 {
                        // inserting a func so move cpos
                        cpos = newstart+len(helpList[0])+1
                    } else {
                        cpos = len(s)
                    }
                    for i:=irow+1;i<=irow+BMARGIN;i+=1 { at(i,1); clearToEOL() }
                }
            }
            helpstring = ""
            contextHelpSelected = false
            startedContextHelp = false
        }

        if eof || broken || endLine {
            break
        }

    } // input loop

    if echo.(bool) {
        at(irow, icol) ; clearToEOL()
        fmt.Print(sparkle(recolour)+s+sparkle("[#-]"))
        cposRowLen:=int(icol+cpos-1)/MW
        at(irow+cposRowLen-1,1)
    }

    lineWrap=old_wrap

    return s, eof, broken
}

// GetSize returns the dimensions of the given terminal.
func GetSize(fd int) (int, int, error) {
    ws, err := unix.IoctlGetWinsize(fd, unix.TIOCGWINSZ)
    if err != nil {
        return -1, -1, err
    }
    return int(ws.Col), int(ws.Row), nil
}


