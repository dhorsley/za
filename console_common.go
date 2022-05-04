
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
    "runtime"
    "unicode/utf8"
    "sort"
    str "strings"
    "sync"
    "syscall"
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


// getInput() : get an input string from stdin, in raw mode
//  it does have some issues with utf8 input when moving the cursor around.
//  not likely to fix this unless it annoys me too much.. more likely to
//  replace the input mechanism wholesale.
//  the issue is basically that we are not tracking where the code points start
//  for each char and moving the cursor to those instead of byte by byte.

func getInput(prompt string, pane string, row int, col int, pcol string, histEnable bool, hintEnable bool, mask string) (s string, eof bool, broken bool) {

    BMARGIN:=BMARGIN
    if !interactive { BMARGIN=0 }

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

    // for special case differences:
    var winmode bool
    if runtime.GOOS=="windows" { winmode = true }

    // get echo status
    echo,_:=gvget("@echo")

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
        fmt.Printf(sparkle(sprompt))

        irow=srow+(int(scol+promptL-1)/MW)
        icol=((scol+promptL-1)%MW)+1

        // change input colour
        fmt.Printf(sparkle(pcol))

        cursAtCol:=((icol+inputL-1)%MW)+1
        rowLen=int(icol+inputL-1)/MW

        // show input
        at(irow, icol)
        if echo.(bool) {
            fmt.Print(s)
        } else {
            fmt.Printf(str.Repeat(mask,inputL))
        }
        clearToEOP(cursAtCol)
        for i:=irow+1+rowLen;i<=irow+BMARGIN;i+=1 { at(i,1); clearToEOL() }
        at(irow+1,1); fmt.Printf(sparkle(helpstring))

        // move cursor to correct position (cpos)
        if irow==MH-BMARGIN && cursAtCol==1 { srow--; rowLen++; fmt.Printf("\n\033M") }
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
            wordUnderCursor,_ = getWord(s, cpos)
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
                        cpos = len(s)-1
                        for i:=irow+1;i<=irow+BMARGIN;i+=1 { at(i,1); clearToEOL() }
                    }
                    helpstring = ""
                }

                // normal space input
                s = insertAt(s, cpos, c[0])
                cpos++
                wordUnderCursor,_ = getWord(s, cpos)

            case bytes.Equal(c, []byte{27,91,49,126}): // home // from showkey -a
                cpos = 0
                wordUnderCursor,_ = getWord(s, cpos)

            case bytes.Equal(c, []byte{27,91,52,126}): // end // from showkey -a
                cpos = len(s)
                wordUnderCursor,_ = getWord(s, cpos)

            case bytes.Equal(c, []byte{1}): // ctrl-a
                cpos = 0
                wordUnderCursor,_ = getWord(s, cpos)

            case bytes.Equal(c, []byte{5}): // ctrl-e
                cpos = len(s)
                wordUnderCursor,_ = getWord(s, cpos)

            case bytes.Equal(c, []byte{21}): // ctrl-u
                s = removeAllBefore(s, cpos)
                cpos = 0
                wordUnderCursor,_ = getWord(s, cpos)
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
                    wordUnderCursor,_ = getWord(s, cpos)
                    clearChars(irow, icol, inputL)
                }

            case bytes.Equal(c, []byte{0x1B, 0x5B, 0x33, 0x7E}): // DEL
                if cpos < len(s) {
                    s = removeBefore(s, cpos+1)
                    wordUnderCursor,_ = getWord(s, cpos)
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
                wordUnderCursor,_ = getWord(s, cpos)

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
                wordUnderCursor,_ = getWord(s, cpos)

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
                        wordUnderCursor,_ = getWord(s, cpos)
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
                        wordUnderCursor,_ = getWord(s, cpos)
                        if curHist != lastHist {
                            l := displayedLen(s)
                            clearChars(irow, icol, l)
                        }
                    }
                }

            case bytes.Equal(c, []byte{0x1B, 0x5B, 0x48}): // HOME
                cpos = 0
                wordUnderCursor,_ = getWord(s, cpos)
            case bytes.Equal(c, []byte{0x1B, 0x5B, 0x46}): // END
                cpos = len(s)
                wordUnderCursor,_ = getWord(s, cpos)

            case bytes.Equal(c, []byte{9}): // TAB

                // completion hinting setup
                if hintEnable {
                    if !startedContextHelp {
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

                    } else {
                        for i:=irow+1;i<=irow+BMARGIN;i++ { at(i,1); clearToEOL() }
                        helpstring=""
                        selectedStar = -1 // start is off the list so that RIGHT has to be pressed to activate.
                        contextHelpSelected = false
                        startedContextHelp = false
                    }
                }

            case bytes.Equal(c, []byte{0x1B, 0x5B, 0x5A}): // SHIFT-TAB

            case bytes.Equal(c, []byte{0x1B, 0x63}): // alt-c
            case bytes.Equal(c, []byte{0x1B, 0x76}): // alt-v

            // specials over 128 - don't do this.. too messy with runes.
            case bytes.Equal(c, []byte{0xc2, 0xa3}): // £  194 163
                s = insertAt(s, cpos, '£')
                cpos++
                wordUnderCursor,_ = getWord(s, cpos)
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
                        wordUnderCursor,_ = getWord(s, cpos)
                        selectedStar = -1 // also reset the selector position for auto-complete
                    }
                }

                for i:=irow+1;i<=irow+BMARGIN;i+=1 { at(i,1); clearToEOL() }
            }

        } // paste or char input end

        // completion hinting population

        if startedContextHelp {

            // populate helpstring
            helpList = []string{}
            helpColoured = []string{}

            for _, v := range funcnames {
                if str.HasPrefix(str.ToLower(v), str.ReplaceAll(str.ToLower(wordUnderCursor),"(","")) {
                    helpColoured = append(helpColoured, "[#5]"+v+"[#-]")
                    helpList = append(helpList, v+"(")
                }
            }

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

            //.. build display string

            helpstring = "help> [##][#6]"

            for cnt, v := range helpColoured {
                starMax = cnt
                if cnt>29 { break } // limit max length of options
                /*
                l := displayedLen(helpstring) + displayedLen(s) + icol
                if (l + displayedLen(v) + icol + 4) > MW {
                    if l > 3 {
                        helpstring += "..."
                    }
                    break
                } else {
                */
                    if cnt == selectedStar {
                        if winmode {
                            helpstring += "[#b2]*"
                        } else {
                            helpstring += "[#b1]*"
                        }
                    }
                    helpstring += v + " "
                // }
            }

            helpstring += "[#-][##]"

            // don't show desc+function help if current word is a keyword instead of function.
            //   otherwise, find desc+func for either remaining guess in context list 
            //   or the current word.

            keynum:=0
            if selectedStar>0 { keynum=selectedStar }

            if len(helpList)>0 {
                if keynum<len(helpList) {
                    if _,found:=keywordset[helpList[keynum]]; !found {
                        pos:=0
                        if keynum==0 {
                            if len(helpList)>1 {
                                // show of desc+function help if current word completes a function (but still other completion options)
                                for p,v:=range helpList {
                                    if wordUnderCursor==v {
                                        pos=p
                                        break
                                    }
                                }
                            }
                        } else {
                            pos=keynum
                        }
                        hla:=helpList[pos]
                        hla=hla[:len(hla)-1]
                        helpstring+="\n[#bold]"+hla+"("+slhelp[hla].in+")[#boff] : [#4]"+slhelp[hla].action+"[#-]"
                    }
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
                    s,newstart = deleteWord(s,cpos)
                    if newstart==-1 { newstart=0 }

                    // remove braces on selected text if expanding out from a dot
                    dpos:=0
                    if newstart>0 { dpos=newstart-1 }

                    if str.IndexByte(helpList[0],'(')!=-1 && dpos<len(s) && s[dpos]=='.' {
                        helpList[0]=helpList[0][:len(helpList[0])-2]
                    }

                    s = insertWord(s, newstart, helpList[0])
                    cpos = newstart+len(helpList[0])

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
        at(srow, icol) ; clearToEOL()
        fmt.Print(sparkle(recolour)+s+sparkle("[#-]"))
        cposRowLen:=int(scol+cpos-1)/MW
        at(srow+cposRowLen,1)
    }

    lineWrap=old_wrap

    return s, eof, broken
}


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
// @TODO: this could use some attention to reduce the differences
//        between interactive/non-interactive source
func pf(s string, va ...any) {

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

        return
    }

    if lineWrap {
        printWithWrap(s)
        return
    }

    fmt.Print(s)

    // row update:
        atlock.Lock()
        chpos:=0
        c:=col
        for ; chpos<len(s); c++ {
            if c%MW==0          { row++; c=0 }
            if s[chpos]=='\n'   { row++; c=0 }
            chpos++
        }
        atlock.Unlock()
    // end test
}

// apply ansi code translation to inbound strings
func sparkle(a any) string {
    switch a:=a.(type) {
    case string:
        return fairyReplacer.Replace(a)
    }
    return sf(`%v`,a)
}

// logging output printer
func plog(s string, va ...any) {

    // print if not silent logging
    if v, _ := gvget("@silentlog"); v.(bool) {
        pf(s, va...)
    }

    // also write to log file
    if loggingEnabled {
        f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
        if err != nil {
            log.Println(err)
        }
        defer f.Close()
        subj, _ := gvget("@logsubject")
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
    if v, _ := gvget("@winterm"); !v.(bool) {
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

func clearToEOP(start int) {
    if currentpane=="global" {
        pf("\033[0K")
    } else {
        pf(str.Repeat(" ",panes[currentpane].w-panes[currentpane].col-start))
    }
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
    fmt.Printf("\033[0m")
    // clear line
    if (len(va) == 1) && (va[0] > p.w) {
        lines := va[0] / (p.w - 1)
        for ; lines >= 0; lines-- {
            at(row+lines-1, 1)
            fmt.Print(rep(" ",p.w))
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
    cmdlock.Lock()
    defer cmdlock.Unlock()

    c=str.Trim(c," \t\n")
    bargs := str.Split(c, " ")
    cmd := exec.Command(bargs[0], bargs[1:]...)
    var out bytes.Buffer
    cmd.Stdin  = os.Stdin
    capture,_:=gvget("@commandCapture")

    if capture.(bool) {
        cmd.Stdout = &out
        err = cmd.Run()
    } else {
        cmd.Stdout = os.Stdout
        err = cmd.Run()
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

    CMDSEP,_:=gvget("@cmdsep")
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
        mt, _ := gvget("mark_time")
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


/// mutex for shell calls
/// used by Copper()+NextCopper()+GetCommand()
var cmdlock = &sync.Mutex{}


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

    riwp,_:=gvget("@runInWindowsParent")
    rip,_ :=gvget("@runInParent")


    // shell reporting option:
    sr,_:=gvget("@shell_report")

    if sr.(bool)==true {
        noshell,_  :=gvget("@noshell")
        shelltype,_:=gvget("@shelltype")
        shellloc,_ :=gvget("@shell_location")
        if !noshell.(bool) {
            pf("[#4]Shell Options: ")
            pf("%v (%v) ",shelltype,shellloc)
            if riwp.(bool) { pf("Windows ") }
            if rip.(bool)  {
                pf("in parent\n[#-]")
            } else {
                pf("in coproc\n[#-]")
            }
            pf("[#4]command : [%s][#-]\n",line)
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

            gvset("@last",0)
            gvset("@lastout",[]byte{0})

            if !squashErr {

                if exitError, ok := err.(*exec.ExitError); ok {
                    errint=exitError.ExitCode()
                    errout=err.Error()
                    gvset("@last",errint)
                    gvset("@last_out",errout)
                } else { // probably a command not found?
                    errint=1
                    errout="Command not found."
                    gvset("@last",errint)
                    gvset("@last_out",errout)
                }

            }

        } else {
            gvset("@last",0)
            gvset("@last_out", []byte{0})
        }
    } else {

        cmdlock.Lock()
        defer cmdlock.Unlock()

        errorFile, err := ioutil.TempFile("", "copper.*.err")
        if err != nil {
            os.Remove(errorFile.Name())
            log.Fatal(err)
        }
        defer os.Remove(errorFile.Name())
        gvset("@last",0)

        read_out := bufio.NewReader(po)

        // issue command
        CMDSEP,_:=gvget("@cmdsep")
        cmdsep:=CMDSEP.(byte)
        hexenc:=hex.EncodeToString([]byte{cmdsep})
        // PIG
        io.WriteString(pi, "\n"+line+` 2>`+errorFile.Name()+` ; last=$? ; echo -en "\x`+hexenc+`${last}\x`+hexenc+`"`+"\n")

        // get output
        ns, commandErr = NextCopper(line, read_out)
        // pf("[copper] line -> <%s>\n",line)
        // pf("[copper] ns   -> <%s>\n",ns)

        // get status code - cmd is not important for this, NextCopper just reads
        //  the output until the next cmdsep
        code, err := NextCopper("#Status", read_out)
        // pull cwd from /proc
        childProc,_:=gvget("@shell_pid")

        cwd,_:=os.Readlink(sf("/proc/%v/cwd",childProc))
        prevdir,_:=gvget("@cwd")
        if cwd!=prevdir {
            err=syscall.Chdir(cwd)
            gvset("@cwd", cwd)
        }

        if commandErr != nil {
            errint = -3
        } else {
            if err == nil {
                errint, err = strconv.Atoi(string(code))
                if err != nil {
                    errint = -2
                }
                if !squashErr {
                    gvset("@last",errint)
                }
            } else {
                errint = -1
            }
        }

        // get stderr file
        b, err := ioutil.ReadFile(errorFile.Name())

        if len(b) > 0 {
            gvset("@last_out", b)
            errout=string(b)
        } else {
            gvset("@last_out", []byte{0})
            errout=""
        }

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
    vos,_:=gvget("@os") ; stros:=vos.(string)
    test_start := sf("\n[#6][#ul][#bold]Za Test[#-]\n\nTesting : %s on "+stros+"\n", file)
    appendToTestReport(test_output_file,0, 0, test_start)
}

func testExit() {
    test_final := sf("\n[#6]Tests Performed %d -- Tests Failed %d -- Tests Passed %d[#-]\n\n", testsPassed+testsFailed, testsFailed, testsPassed)
    appendToTestReport(test_output_file,0, 0, test_final)
}


