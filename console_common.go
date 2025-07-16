package main

import (
    "bufio"
    "bytes"
    "encoding/hex"
    "encoding/json"
    "errors"
    "fmt"
    "io"
    "io/ioutil"
    "log"
    "os"
    "os/exec"
    "regexp"
    "runtime"
    "sort"
    "strconv"
    str "strings"
    "sync"
    "syscall"
    "time"
    "unicode/utf8"
)

var completions = []string{"VAR", "SETGLOB", "PAUSE",
    "HELP", "NOP", "REQUIRE", "EXIT", "VERSION",
    "QUIET", "LOUD", "UNSET", "INPUT", "PROMPT", "LOG", "PRINT", "PRINTLN",
    "LOGGING", "CLS", "AT", "DEFINE", "SHOWDEF", "ENDDEF", "RETURN", "ASYNC",
    "MODULE", "USE", "USES", "WHILE", "ENDWHILE", "FOR", "FOREACH",
    "ENDFOR", "CONTINUE", "BREAK", "ON", "DO", "IF", "ELSE", "ENDIF", "CASE",
    "IS", "CONTAINS", "HAS", "IN", "OR", "ENDCASE", "WITH", "ENDWITH",
    "STRUCT", "ENDSTRUCT", "SHOWSTRUCT",
    "TRY", "CATCH", "ENDTRY", "THEN",
    "PANE", "DOC", "TEST", "ENDTEST", "ASSERT", "TO", "STEP", "AS", "ENUM", "HIST",
}

const ansi = "[\u001B\u009B][[\\]()#;?]*(?:(?:(?:[a-zA-Z\\d]*(?:;[a-zA-Z\\d]*)*)?\u0007)|(?:(?:\\d{1,4}(?:;\\d{0,4})*)?[\\dA-PRZcf-ntqry=><~]))"

var winmode bool
var funcnames []string

var ansiReplacables []string
var fairyReplacer *str.Replacer

// / setup the za->ansi mappings
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
        fairydust["bd"] = "\033[49m"
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
        fairydust["fd"] = "\033[39m"
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
        fairydust["ASB"] = "\033[?1049h"
        fairydust["RSB"] = "\033[?1049l"
        fairydust["."] = "\033[39m"

        ansiReplacables = []string{}

        for k, v := range fairydust {
            ansiReplacables = append(ansiReplacables, "[#"+k+"]")
            ansiReplacables = append(ansiReplacables, v)
        }
        fairyReplacer = str.NewReplacer(ansiReplacables...)

    } else {
        var ansiCodeList = []string{"b0", "b1", "b2", "b3", "b4", "b5", "b6", "b7", "0", "1", "2", "3", "4", "5", "6", "7", "i1", "i0",
            "default", "underline", "ul", "invert", "bold", "boff", "-", "#", "bd", "bdefault", "bblack", "bred",
            "bgreen", "byellow", "bblue", "bmagenta", "bcyan", "bbgray", "bgray", "bbred", "bbgreen",
            "bbyellow", "bbblue", "bbmagenta", "bbcyan", "bwhite", "fd", "fdefault", "fblack", "fred", "fgreen",
            "fyellow", "fblue", "fmagenta", "fcyan", "fbgray", "fgray", "fbred", "fbgreen", "fbyellow",
            "fbblue", "fbmagenta", "fbcyan", "fwhite", "dim", "blink", "hidden", "crossed", "framed", "CSI", "CTE", "ASB", "RSB", ".",
        }

        ansiReplacables = []string{}

        for _, c := range ansiCodeList {
            fairydust[c] = ""
        }

        for k, v := range fairydust {
            ansiReplacables = append(ansiReplacables, "[#"+k+"]")
            ansiReplacables = append(ansiReplacables, v)
        }
        fairyReplacer = str.NewReplacer(ansiReplacables...)

    }
}

func enable_mouse() {
    pf("\x1b[?1000h\x1b[?1002h\x1b[?1015h\x1b[?1006h")
}

func disable_mouse() {
    pf("\x1b[?1006l\x1b[?1015l\x1b[?1002l\x1b[?1000l")
}

func mouse_press(inp []byte) {

    // @wip: notes

    /*
       Normal tracking mode (not implemented in Linux 2.0.24) sends an
       escape sequence on both button press and release.  Modifier
       information is also sent.  It is enabled by sending ESC [ ? 1000
       h and disabled with ESC [ ? 1000 l.  On button press or release,
       xterm(1) sends ESC [ M bxy.  The low two bits of b encode button
       information: 0=MB1 pressed, 1=MB2 pressed, 2=MB3 pressed,
       3=release.  The upper bits encode what modifiers were down when
       the button was pressed and are added together: 4=Shift, 8=Meta,
       16=Control.  Again x and y are the x and y coordinates of the
       mouse event.  The upper left corner is (1,1).
    */

    // lmb down and up ➜ down : 0;69;28M up : 0;69;28m
    // rmb down and up ➜ down : 2;68;27M up : 2;68;27m
    // mmb down and up ➜ down : 1;67;27M up : 1;67;27m
    // mwheel up       ➜      : 64;67;27M
    // mwheel down     ➜      : 65;67;27M

    switch {
    // case bytes.Equal(inp, []byte{27,91,49,126}): // home // from showkey -a
    }
}

func insertAt(runes []rune, pos int, r rune) []rune {
    return append(runes[:pos], append([]rune{r}, runes[pos:]...)...)
}

func removeBefore(runes []rune, pos int) []rune {
    if pos <= 0 || pos > len(runes) {
        return runes
    }
    return append(runes[:pos-1], runes[pos:]...)
}

func drawBox(r0, c0, r1, c1 int, title string) {
    at(r0, c0)
    fmt.Print("┌" + str.Repeat("─", c1-c0-1) + "┐")
    for r := r0 + 1; r < r1; r++ {
        at(r, c0)
        fmt.Print("│" + str.Repeat(" ", c1-c0-1) + "│")
    }
    at(r1, c0)
    fmt.Print("└" + str.Repeat("─", c1-c0-1) + "┘")
    if title != "" {
        at(r0, c0+2)
        fmt.Print(title)
    }
}

func hasPrefixRunes(runes, prefix []rune) bool {
    if len(prefix) > len(runes) {
        return false
    }
    for i, r := range prefix {
        if runes[i] != r {
            return false
        }
    }
    return true
}

// getInput() : get an input string from stdin, in raw mode
func getInput(prompt string, in_defaultString string, pane string, row int, col int, width int, ddopts []string, pcol string, histEnable bool, hintEnable bool, mask string) (out_s string, eof bool, broken bool) {

    BMARGIN := BMARGIN
    if !interactive {
        BMARGIN = 0
    }

    old_wrap := lineWrap
    lineWrap = false

    var ddmode bool
    if len(ddopts) > 0 {
        ddmode = true
    }

    showCursor()

    var s, defaultString []rune
    defaultString = []rune(in_defaultString)

    sprompt := sparkle(prompt)

    // calculate real prompt length after ansi codes applied.

    // init
    cpos := len(s)               // cursor pos as extent of printable chars from start
    orig_s := s                  // original string before history navigation begins
    navHist := false             // currently navigating history entries?
    startedContextHelp := false  // currently displaying auto-completion options
    contextHelpSelected := false // final selection made during auto-completion?
    selectedStar := 0            // starting word position of the current selection during auto-completion
    var starMax int              // fluctuating maximum word position for the auto-completion selector
    var wordUnderCursor []rune   // maintains a copy of the word currently under amendment
    var helpColoured []string    // populated (on TAB) list of auto-completion possibilities as displayed on console
    var helpList []string        // list of remaining possibilities governed by current input word
    var helpstring string        // final compounded output string including helpColoured components
    var funcnames []string       // the list of possible standard library functions
    var helpType []int

    // Reverse search state variables
    var reverseSearchMode bool = false
    var searchBuffer []rune
    var searchResults []int     // indices of matching history entries
    var currentSearchResult int // current position in search results
    var searchPrompt string = "(search): "
    var searchDisplayRow int
    var searchDisplayCol int

    // files in cwd for tab completion
    var fileList map[string]os.FileInfo

    // for special case differences:
    var winmode bool
    if runtime.GOOS == "windows" {
        winmode = true
    }

    // get echo status
    echo, _ := gvget("@echo")

    if mask == "" {
        mask = "*"
    }

    endLine := false // input complete?

    at(row, col)

    var srow, scol int // the start of input, including prompt position
    var irow, icol int // current start of input position

    irow = srow
    lastsrow := row
    defaultAccepted := false
    clearWidth := 0
    if width-col >= 0 {
        clearWidth = width - col
    }

    fmt.Printf(sparkle(pcol))
    clearChars(row, col, clearWidth)
    for {

        // calc new values for row,col
        srow = row
        scol = col
        promptL := displayedLen(sprompt)
        inputL := displayedLen(string(s))
        dispL := promptL + inputL

        hideCursor()

        // move start row back if multiline at bottom of window
        // @note: MH and MW are globals which may change during a SIGWINCH event.
        rowLen := int(dispL-1) / MW
        if srow > MH-BMARGIN {
            srow = MH - BMARGIN
        }
        if srow == MH-BMARGIN {
            srow = srow - rowLen
        }
        if lastsrow != srow {
            m1 := min(lastsrow, srow)
            m2 := max(lastsrow, srow)
            for r := m2; r > m1; r-- {
                at(r, col)
                clearToEOL()
            }
        }
        lastsrow = srow

        // print prompt
        at(srow, scol)
        fmt.Printf(sparkle(sprompt))

        irow = srow + (int(scol+promptL-1) / MW)
        icol = ((scol + promptL - 1) % MW) + 1

        // change input colour
        fmt.Printf(sparkle(pcol))

        cursAtCol := ((icol + inputL - 1) % MW) + 1
        rowLen = int(icol+inputL-1) / MW

        // show input
        at(irow, icol)
        if echo.(bool) {
            if len(s) > len(defaultString) {
                fmt.Print(string(s))
            } else {
                if str.HasPrefix(in_defaultString, string(s)) && !defaultAccepted {
                    // #dim + italic + string + normal
                    fmt.Print("\033[2m\033[3m" + in_defaultString + "\033[23m\033[22m")
                } else {
                    clearChars(irow, icol, len(in_defaultString))
                    at(irow, icol)
                    fmt.Print(string(s))
                }
            }
        } else {
            fmt.Printf(str.Repeat(mask, inputL))
        }
        if startedContextHelp {
            for i := irow + 1 + rowLen; i <= irow+BMARGIN; i += 1 {
                at(i, 1)
                clearToEOL()
            }
            at(irow+1, 1)
            fmt.Printf(sparkle(helpstring))
        }

        // move cursor to correct position (cpos)
        if irow == MH-BMARGIN && cursAtCol == 1 {
            srow--
            rowLen++
            fmt.Printf("\n\033M")
        }
        cposCursAtCol := ((icol + cpos - 1) % MW) + 1
        cposRowLen := int(icol+cpos-1) / MW
        at(srow+cposRowLen, cposCursAtCol)

        showCursor()

        // get key stroke
        c, _, pasted, pbuf := getch(0)

        if pasted {

            // we disallow multi-line pasted input. this is only a line editor.
            // no need to get fancy.

            // get paste buffer up to first eol
            eol := str.IndexByte(pbuf, '\r')     // from hazy memories... vte paste marks line breaks with a single CR
            alt_eol := str.IndexByte(pbuf, '\n') // just in case i didn't remember right...

            if eol != -1 {
                pbuf = pbuf[:eol]
            }

            if alt_eol != -1 {
                pbuf = pbuf[:alt_eol]
            }

            // strip ansi codes from pbuf then shove it in the input string
            pbuf = Strip(pbuf)
            s = insertWord(s, cpos, pbuf)
            cpos += rlen(pbuf)
            wordUnderCursor, _ = getWord(s, cpos)
            selectedStar = -1

        } else {
            switch {

            case bytes.Equal(c, []byte{3}): // ctrl-c
                broken = true
                break
            case bytes.Equal(c, []byte{4}): // ctrl-d
                eof = true
                break
            case bytes.Equal(c, []byte{26}): // ctrl-z
                // Send SIGTSTP to the current process group to suspend Za
                // Platform-specific implementation handles Unix vs Windows
                handleCtrlZ()
                break

            case reverseSearchMode:
                // Handle specific input during reverse search mode
                if len(c) == 1 {
                    if c[0] == 13 { // Enter - accept current result
                        reverseSearchMode = false
                        if len(searchResults) > 0 && currentSearchResult < len(searchResults) {
                            s = []rune(hist[searchResults[currentSearchResult]])
                            cpos = len(s)
                        }
                        showCursor()
                        // Clear the entire line and restore normal input display
                        clearChars(irow, icol, inputL)
                        // Clear any remaining characters on the line to the end
                        remainingWidth := width - icol
                        if remainingWidth > inputL {
                            clearChars(irow, icol+inputL, remainingWidth-inputL)
                        }
                        at(irow, icol)
                        pf(string(s))
                        break
                    } else if c[0] == 18 { // Ctrl+R - cancel search
                        reverseSearchMode = false
                        s = orig_s
                        cpos = len(s)
                        showCursor()
                        // Clear the entire line and restore normal input display
                        clearChars(irow, icol, inputL)
                        // Clear any remaining characters on the line to the end
                        remainingWidth := width - icol
                        if remainingWidth > inputL {
                            clearChars(irow, icol+inputL, remainingWidth-inputL)
                        }
                        at(irow, icol)
                        pf(string(s))
                        break
                    } else if c[0] >= 32 && c[0] <= 126 { // Printable character
                        // Add character to search buffer
                        searchBuffer = append(searchBuffer, rune(c[0]))

                        // Search through history backwards
                        searchResults = []int{}
                        searchTerm := str.ToLower(string(searchBuffer))
                        for i := len(hist) - 1; i >= 0; i-- {
                            if str.Contains(str.ToLower(hist[i]), searchTerm) {
                                searchResults = append(searchResults, i)
                            }
                        }
                        currentSearchResult = 0

                        // Update display - clear the search area and redraw
                        clearChars(searchDisplayRow, searchDisplayCol, len(searchPrompt)+len(searchBuffer))
                        // Clear any remaining characters that might be displayed
                        remainingWidth := width - searchDisplayCol
                        if remainingWidth > len(searchPrompt)+len(searchBuffer) {
                            clearChars(searchDisplayRow, searchDisplayCol+len(searchPrompt)+len(searchBuffer), remainingWidth-(len(searchPrompt)+len(searchBuffer)))
                        }
                        at(searchDisplayRow, searchDisplayCol)
                        pf("[#bold][#6]" + searchPrompt + string(searchBuffer) + "[#-][#4]▋[#-]")
                        if len(searchResults) > 0 {
                            pf(" -> [#4]" + hist[searchResults[currentSearchResult]] + "[#-]")
                        }

                    } else if c[0] == 127 { // Backspace
                        if len(searchBuffer) > 0 {
                            searchBuffer = searchBuffer[:len(searchBuffer)-1]

                            // Re-search with updated buffer
                            searchResults = []int{}
                            if len(searchBuffer) > 0 {
                                searchTerm := str.ToLower(string(searchBuffer))
                                for i := len(hist) - 1; i >= 0; i-- {
                                    if str.Contains(str.ToLower(hist[i]), searchTerm) {
                                        searchResults = append(searchResults, i)
                                    }
                                }
                            }
                            currentSearchResult = 0

                            // Update display - clear the search area and redraw
                            clearChars(searchDisplayRow, searchDisplayCol, len(searchPrompt)+len(searchBuffer))
                            // Clear any remaining characters that might be displayed
                            remainingWidth := width - searchDisplayCol
                            if remainingWidth > len(searchPrompt)+len(searchBuffer) {
                                clearChars(searchDisplayRow, searchDisplayCol+len(searchPrompt)+len(searchBuffer), remainingWidth-(len(searchPrompt)+len(searchBuffer)))
                            }
                            at(searchDisplayRow, searchDisplayCol)
                            pf("[#bold][#6]" + searchPrompt + string(searchBuffer) + "[#-][#4]▋[#-]")
                            if len(searchResults) > 0 {
                                pf(" -> [#4]" + hist[searchResults[currentSearchResult]] + "[#-]")
                            }
                        }
                    } else if c[0] == 21 { // Ctrl+U - clear search buffer
                        searchBuffer = []rune{}
                        searchResults = []int{}
                        currentSearchResult = 0

                        // Update display - clear the search area and redraw
                        clearChars(searchDisplayRow, searchDisplayCol, len(searchPrompt)+len(searchBuffer))
                        // Clear any remaining characters that might be displayed
                        remainingWidth := width - searchDisplayCol
                        if remainingWidth > len(searchPrompt)+len(searchBuffer) {
                            clearChars(searchDisplayRow, searchDisplayCol+len(searchPrompt)+len(searchBuffer), remainingWidth-(len(searchPrompt)+len(searchBuffer)))
                        }
                        at(searchDisplayRow, searchDisplayCol)
                        pf("[#bold][#6]" + searchPrompt + string(searchBuffer) + "[#-][#4]▋[#-]")
                    }
                } else if bytes.Equal(c, []byte{0x1B, 0x5B, 0x41}) { // UP arrow in search
                    if len(searchResults) > 0 {
                        currentSearchResult = (currentSearchResult + 1) % len(searchResults)
                        // Update display - clear the search area and redraw
                        clearChars(searchDisplayRow, searchDisplayCol, len(searchPrompt)+len(searchBuffer))
                        // Clear any remaining characters that might be displayed
                        remainingWidth := width - searchDisplayCol
                        if remainingWidth > len(searchPrompt)+len(searchBuffer) {
                            clearChars(searchDisplayRow, searchDisplayCol+len(searchPrompt)+len(searchBuffer), remainingWidth-(len(searchPrompt)+len(searchBuffer)))
                        }
                        at(searchDisplayRow, searchDisplayCol)
                        pf("[#bold][#6]" + searchPrompt + string(searchBuffer) + "[#-][#4]▋[#-]")
                        pf(" -> [#4]" + hist[searchResults[currentSearchResult]] + "[#-]")
                    }
                    break
                } else if bytes.Equal(c, []byte{0x1B, 0x5B, 0x42}) { // DOWN arrow in search
                    if len(searchResults) > 0 {
                        currentSearchResult = (currentSearchResult - 1 + len(searchResults)) % len(searchResults)
                        // Update display - clear the search area and redraw
                        clearChars(searchDisplayRow, searchDisplayCol, len(searchPrompt)+len(searchBuffer))
                        // Clear any remaining characters that might be displayed
                        remainingWidth := width - searchDisplayCol
                        if remainingWidth > len(searchPrompt)+len(searchBuffer) {
                            clearChars(searchDisplayRow, searchDisplayCol+len(searchPrompt)+len(searchBuffer), remainingWidth-(len(searchPrompt)+len(searchBuffer)))
                        }
                        at(searchDisplayRow, searchDisplayCol)
                        pf("[#bold][#6]" + searchPrompt + string(searchBuffer) + "[#-][#4]▋[#-]")
                        pf(" -> [#4]" + hist[searchResults[currentSearchResult]] + "[#-]")
                    }
                    break
                }
                break

            case bytes.Equal(c, []byte{18}): // ctrl-r - reverse search
                if histEnable && !histEmpty {
                    if reverseSearchMode {
                        // Second Ctrl+R press - cancel search
                        reverseSearchMode = false
                        s = orig_s
                        cpos = len(s)
                        showCursor()
                        // Clear the entire line and restore normal input display
                        clearChars(irow, icol, inputL)
                        // Clear any remaining characters on the line to the end
                        remainingWidth := width - icol
                        if remainingWidth > inputL {
                            clearChars(irow, icol+inputL, remainingWidth-inputL)
                        }
                        at(irow, icol)
                        pf(string(s))
                        break
                    } else if len(s) == 0 {
                        // Only enter reverse search mode if there's no existing input
                        // First Ctrl+R press - enter reverse search mode
                        reverseSearchMode = true
                        searchBuffer = []rune{}
                        searchResults = []int{}
                        currentSearchResult = 0

                        // Save current input state
                        if !navHist {
                            orig_s = s
                        }

                        // Clear the entire input line and show search prompt
                        clearChars(irow, icol, inputL)
                        // Clear any remaining characters on the line to the end
                        remainingWidth := width - icol
                        if remainingWidth > inputL {
                            clearChars(irow, icol+inputL, remainingWidth-inputL)
                        }
                        searchDisplayRow = irow
                        searchDisplayCol = icol

                        // Show initial search prompt
                        at(searchDisplayRow, searchDisplayCol)
                        pf("[#bold][#6]" + searchPrompt + "[#-][#4]▋[#-]")
                        break
                    }
                    // If there's existing input, ignore Ctrl+R (don't enter search mode)
                }
                break

            case bytes.Equal(c, []byte{0x0F}): // Ctrl+O for multiline editor
                result, eof, broken := multilineEditor(string(s), -1, MH-5, "", "", "Editor")
                if !broken {
                    // Replace the input buffer in getInput() with the result from the multiline editor
                    s = []rune(result)
                    cpos = len(s)
                } else if eof {
                    // If user pressed ctrl-d in multiline, treat as EOF in getInput
                    return "", true, false
                } else {
                    // User pressed ESC in multiline editor: return to input mode with original buffer unchanged
                }

            case bytes.Equal(c, []byte{13}): // enter

                if startedContextHelp {
                    contextHelpSelected = true
                    clearChars(irow, icol, inputL)
                    for i := irow + 1; i <= irow+BMARGIN; i += 1 {
                        at(i, 1)
                        clearToEOL()
                    }
                    helpstring = ""
                    break
                }

                endLine = true

                if len(s) != 0 {
                    addToHistory(string(s))
                }

                break

            case bytes.Equal(c, []byte{32}): // space

                if startedContextHelp {
                    contextHelpSelected = false
                    startedContextHelp = false
                    wordUnderCursor, _ = getWord(s, cpos)
                    cmpStr := str.ToLower(string(wordUnderCursor))
                    parenPos := str.IndexByte(cmpStr, '(')
                    if parenPos == -1 && len(helpList) == 1 {
                        var newstart int
                        s, newstart = deleteWord(s, cpos)
                        add := ""
                        if len(s) > 0 {
                            add = " "
                        }
                        if newstart == -1 {
                            newstart = 0
                        }
                        s = insertWord(s, newstart, add+helpList[0]+" ")
                        cpos = len(s) - 1
                        for i := irow + 1; i <= irow+BMARGIN; i += 1 {
                            at(i, 1)
                            clearToEOL()
                        }
                    }
                    helpstring = ""
                    for i := irow + 1; i <= irow+BMARGIN; i += 1 {
                        at(i, 1)
                        clearToEOL()
                    }
                }

                // normal space input
                s = insertAt(s, cpos, rune(c[0]))
                cpos++
                wordUnderCursor, _ = getWord(s, cpos)

            case bytes.Equal(c, []byte{27, 91, 49, 126}): // home // from showkey -a
                cpos = 0
                wordUnderCursor, _ = getWord(s, cpos)

            case bytes.Equal(c, []byte{27, 91, 52, 126}): // end // from showkey -a
                cpos = len(s)
                wordUnderCursor, _ = getWord(s, cpos)

            case bytes.Equal(c, []byte{1}): // ctrl-a
                cpos = 0
                wordUnderCursor, _ = getWord(s, cpos)

            case bytes.Equal(c, []byte{5}): // ctrl-e
                cpos = len(s)
                wordUnderCursor, _ = getWord(s, cpos)

            case bytes.Equal(c, []byte{11}): // ctrl-k
                s = s[:cpos]
                wordUnderCursor, _ = getWord(s, cpos)
                clearChars(irow, icol, inputL)

            case bytes.Equal(c, []byte{21}): // ctrl-u
                s = removeAllBefore(s, cpos)
                cpos = 0
                wordUnderCursor, _ = getWord(s, cpos)
                clearChars(irow, icol, inputL)

            case bytes.Equal(c, []byte{127}): // backspace

                if startedContextHelp && len(helpstring) == 0 {
                    startedContextHelp = false
                    helpstring = ""
                }

                for i := irow + 1; i <= irow+BMARGIN; i += 1 {
                    at(i, 1)
                    clearToEOL()
                }

                if cpos > 0 {
                    s = removeBefore(s, cpos)
                    cpos--
                    wordUnderCursor, _ = getWord(s, cpos)
                    clearChars(irow, icol, inputL)
                }

            case bytes.Equal(c, []byte{0x1B, 0x5B, 0x33, 0x7E}): // DEL
                if len(s) == 0 && len(defaultString) != 0 {
                    clearChars(irow, icol, len(defaultString))
                    defaultString = []rune{}
                }
                if cpos < len(s) {
                    s = removeBefore(s, cpos+1)
                    wordUnderCursor, _ = getWord(s, cpos)
                    clearChars(irow, icol, displayedLenUtf8(s)+1)
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
                wordUnderCursor, _ = getWord(s, cpos)

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
                wordUnderCursor, _ = getWord(s, cpos)

            case bytes.Equal(c, []byte{0x1B, 0x5B, 0x41}): // UP

                if MW < displayedLenUtf8(s) && cpos > MW {
                    cpos -= MW
                    break
                }

                if histEnable {
                    if !histEmpty {
                        if !navHist {
                            navHist = true
                            curHist = lastHist
                            orig_s = s
                        }
                        clearChars(irow, icol, inputL)
                        if curHist > 0 {
                            curHist--
                            s = []rune(hist[curHist])
                        }
                        cpos = len(s)
                        wordUnderCursor, _ = getWord(s, cpos)
                        rowLen = int(icol+cpos-1) / MW
                        if rowLen > 0 {
                            irow -= rowLen
                        }
                        if curHist != lastHist {
                        }
                    }
                }

            case bytes.Equal(c, []byte{0x1B, 0x5B, 0x42}): // DOWN

                if ddmode {

                    // input loop

                    ddpos := 0
                    selected := false
                    optslen := 0
                    // noChange:=false

                    hideCursor()
                inloopdd:
                    for {
                        absat(irow+1, cpos)
                        optslen = 0
                        for k, ddo := range ddopts {
                            if k == ddpos {
                                pf("[#invert]")
                            }
                            pf(ddo)
                            if k == ddpos {
                                pf("[#-]")
                            }
                            pf(" ")
                            optslen += 1 + len(ddo)
                        }
                        c := wrappedGetCh(0, false)

                        switch c {
                        case 9:
                            fallthrough
                        case 10:
                            if ddpos < len(ddopts)-1 {
                                ddpos += 1
                            }

                        case 11:
                            fallthrough
                        case 8:
                            if ddpos > 0 {
                                ddpos -= 1
                            }

                        case 13:
                            fallthrough
                        case 32:
                            selected = true
                            break inloopdd

                        // these cases may be removed later, they are reserved for later use
                        //  it may be the case that we allow partially typed matches.

                        case 27:
                            // noChange=true
                            break inloopdd
                        default:
                            // noChange=true
                            break inloopdd
                        }
                    }
                    clearChars(irow+1, cpos, optslen)
                    // - if escaped/broken then carry on as normal
                    if selected {
                        // populate input buffer with selection
                        s = insertWord(s, cpos, ddopts[ddpos])
                        cpos += len(ddopts[ddpos])
                        wordUnderCursor, _ = getWord(s, cpos)
                    }

                    showCursor()
                    break
                }

                // normal down key operations resume here
                if displayedLenUtf8(s) > MW && cpos < MW {
                    cpos += MW
                    break
                }

                if histEnable {
                    if navHist {
                        clearChars(irow, icol, inputL)
                        if curHist < lastHist-1 {
                            curHist++
                            s = []rune(hist[curHist])
                        } else {
                            s = orig_s
                            navHist = false
                        }
                        cpos = len(s)
                        wordUnderCursor, _ = getWord(s, cpos)
                        if curHist != lastHist {
                        }
                    }
                }

            case bytes.Equal(c, []byte{0x1B, 0x5B, 0x48}): // HOME
                cpos = 0
                wordUnderCursor, _ = getWord(s, cpos)
            case bytes.Equal(c, []byte{0x1B, 0x5B, 0x46}): // END
                cpos = len(s)
                wordUnderCursor, _ = getWord(s, cpos)

            case bytes.Equal(c, []byte{9}): // TAB

                // completion hinting setup
                if hintEnable {
                    if !startedContextHelp {
                        funcnames = nil

                        startedContextHelp = true
                        for i := irow + 1; i <= irow+BMARGIN; i++ {
                            at(i, 1)
                            clearToEOL()
                        }
                        helpstring = ""
                        selectedStar = -1 // start is off the list so that RIGHT has to be pressed to activate.

                        //.. add functionnames
                        for k, _ := range slhelp {
                            funcnames = append(funcnames, k)
                        }
                        sort.Strings(funcnames)

                    } else {
                        for i := irow + 1; i <= irow+BMARGIN; i++ {
                            at(i, 1)
                            clearToEOL()
                        }
                        helpstring = ""
                        selectedStar = -1 // start is off the list so that RIGHT has to be pressed to activate.
                        contextHelpSelected = false
                        startedContextHelp = false
                    }
                } else { // accept default
                    if hasPrefixRunes(defaultString, s) {
                        s = defaultString
                        cpos = len(s)
                        defaultAccepted = true
                        helpstring = ""
                        selectedStar = -1
                        contextHelpSelected = false
                        startedContextHelp = false
                    }
                }

            case bytes.Equal(c, []byte{0x1B, 0x5B, 0x5A}): // SHIFT-TAB

            case bytes.Equal(c, []byte{0x1B, 0x63}): // alt-c
            case bytes.Equal(c, []byte{0x1B, 0x76}): // alt-v

            // ignore list
            case bytes.Equal(c, []byte{0x1B, 0x5B, 0x35}): // pgup
            case bytes.Equal(c, []byte{0x1B, 0x5B, 0x36}): // pgdown
            case bytes.Equal(c, []byte{0x1B, 0x5B, 0x32}): // insert

            default:
                // Normal input processing (only reached when not in reverse search mode)
                if len(c) == 1 {
                    if c[0] > 32 && c[0] < 128 {
                        s = insertAt(s, cpos, rune(c[0]))
                        cpos++
                        wordUnderCursor, _ = getWord(s, cpos)
                        selectedStar = -1 // also reset the selector position for auto-complete
                    }
                } else {
                    // multi-byte, like utf8?
                    r, _ := utf8.DecodeRune(c)
                    s = insertAt(s, cpos, r)
                    cpos++
                    wordUnderCursor, _ = getWord(s, cpos)
                    selectedStar = -1
                }

                if startedContextHelp {
                    for i := irow + 1; i <= irow+BMARGIN; i += 1 {
                        at(i, 1)
                        clearToEOL()
                    }
                }
            }

        } // paste or char input end

        // completion hinting population

        if startedContextHelp {

            // populate helpstring
            helpList = []string{}
            helpColoured = []string{}
            helpType = []int{}

            for _, v := range funcnames {
                cmpStr := str.ToLower(string(wordUnderCursor))
                parenPos := str.IndexByte(cmpStr, '(')
                if parenPos != -1 {
                    cmpStr = cmpStr[:parenPos]
                }
                if str.HasPrefix(str.ToLower(v), cmpStr) {
                    helpColoured = append(helpColoured, "[#5]"+v+"[#-]")
                    helpList = append(helpList, v+"(")
                    helpType = append(helpType, HELP_FUNC)
                }
            }

            for _, v := range completions {
                if str.HasPrefix(str.ToLower(v), str.ToLower(string(wordUnderCursor))) {
                    helpColoured = append(helpColoured, "[#6]"+v+"[#-]")
                    helpList = append(helpList, v)
                    helpType = append(helpType, HELP_KEYWORD)
                }
            }

            fileList = make(map[string]os.FileInfo)

            max_depth, _ := gvget("context_dir_depth")

            for _, paf := range dirplus(".", max_depth.(int)) {

                name := paf.DirEntry.Name()
                parent := "."
                pan := name

                if len(paf.Parent) > 2 {
                    parent = paf.Parent[2:]
                    pan = parent + "/" + name
                }

                if matched, _ := regexp.MatchString("^"+string(wordUnderCursor), pan); !matched {
                    continue
                }

                f, _ := os.Stat(pan)

                if parent == "." {
                    appendEntry := ""
                    if f.IsDir() {
                        appendEntry += "[#3]"
                    } else {
                        appendEntry += "[#4]"
                    }
                    appendEntry += name + "[#-]"
                    helpColoured = append(helpColoured, appendEntry)
                    helpList = append(helpList, name)
                    helpType = append(helpType, HELP_DIRENT)
                    fileList[name] = f
                } else {
                    appendEntry := sf("[#2]%s[#-]", pan)
                    helpColoured = append(helpColoured, appendEntry)
                    helpList = append(helpList, pan)
                    helpType = append(helpType, HELP_DIRENT)
                    fileList[pan] = f
                }
            }

            //.. build display string

            helpstring = "help> [##][#6]"

            for cnt, v := range helpColoured {
                starMax = cnt
                if cnt > 29 {
                    break
                } // limit max length of options
                if cnt == selectedStar {
                    if winmode {
                        helpstring += "[#b2]*"
                    } else {
                        helpstring += "[#b1]*"
                    }
                }
                helpstring += v + " "
            }

            helpstring += "[#-][##]"

            // don't show desc+function help if current word is a keyword instead of function.
            //   otherwise, find desc+func for either remaining guess in context list
            //   or the current word.

            keynum := 0
            if selectedStar > 0 {
                keynum = selectedStar
            }

            if len(helpList) > 0 {
                if keynum < len(helpList) {
                    if _, found := keywordset[helpList[keynum]]; !found {
                        pos := keynum
                        if keynum == 0 {
                            if len(helpList) > 1 {
                                // show of desc+function help if current word completes a function (but still other completion options)
                                wuc := string(wordUnderCursor)
                                for p, v := range helpList {
                                    if wuc == v {
                                        pos = p
                                        break
                                    }
                                }
                            }
                        }
                        hla := helpList[pos]
                        switch helpType[pos] {
                        case HELP_FUNC:
                            hla = hla[:len(hla)-1]
                            helpstring += "\n[#bold]" + hla + "(" + slhelp[hla].in + ")[#boff] : [#4]" + slhelp[hla].action + "[#-]"
                        case HELP_DIRENT:
                            f := fileList[helpList[pos]]
                            helpstring += "\n" + helpList[pos]
                            if f.IsDir() {
                                helpstring += " [#bold]Directory[#boff]"
                            } else {
                                helpstring += " [#bold]File[#boff]"
                            }
                            helpstring += sf(" Size:%d Mode:%o Last Modification:%v", f.Size(), f.Mode(), f.ModTime())
                        }
                    }
                }
            }

        }

        if contextHelpSelected {
            if len(helpList) > 0 {
                if selectedStar > -1 {
                    helpList = []string{helpList[selectedStar]}
                }
                if len(helpList) == 1 {
                    var newstart int
                    s, newstart = deleteWord(s, cpos)
                    if newstart == -1 {
                        newstart = 0
                    }

                    // remove braces on selected text if expanding out from a dot
                    dpos := 0
                    if newstart > 0 {
                        dpos = newstart - 1
                    }

                    if str.IndexByte(helpList[0], '(') != -1 && dpos < len(s) && s[dpos] == '.' {
                        helpList[0] = helpList[0][:len(helpList[0])-1]
                    }

                    s = insertWord(s, newstart, helpList[0])
                    cpos = newstart + len(helpList[0])

                    for i := irow + 1; i <= irow+BMARGIN; i += 1 {
                        at(i, 1)
                        clearToEOL()
                    }
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

    if len(s) == 0 && len(defaultString) != 0 {
        s = defaultString
    }

    if echo.(bool) {
        fmt.Printf(sparkle(pcol))
        clearWidth := 0
        if width-scol >= 0 {
            clearWidth = width - scol
        }
        clearChars(srow, scol, clearWidth)
        at(srow, scol)
        fmt.Printf(sparkle(sprompt))
        fmt.Print(string(s)) // recolour const sets italics
    }

    lineWrap = old_wrap

    return string(s), eof, broken
}

func secScreenActive() bool {
    return altScreen
}

func findRunesThatFit(runes []rune, maxWidth int) int {
    if len(runes) > maxWidth {
        return maxWidth
    }
    return len(runes)
}

/* better way, when i can be bothered. using: mattn/go-runwidth.
func findRunesThatFit(runes []rune, maxWidth int) int {
    width := 0
    for i, r := range runes {
        width += runeWidth(r) // If you have custom rune width; otherwise assume 1
        if width > maxWidth {
            return i
        }
    }
    return len(runes)
}
*/

func escapeControlChars(s string) string {
    s = str.ReplaceAll(s, "\\", "\\\\") // escape backslashes first
    s = str.ReplaceAll(s, "\n", "\\n")
    s = str.ReplaceAll(s, "\r", "\\r")
    s = str.ReplaceAll(s, "\t", "\\t")
    return s
}

func escapeControlCharsInLiterals(s string) string {
    var out []rune
    inLiteral := false
    literalChar := rune(0)

    for _, r := range s {
        if !inLiteral {
            if r == '"' || r == '`' {
                inLiteral = true
                literalChar = r
            }
            out = append(out, r)
        } else {
            if r == literalChar {
                inLiteral = false
                literalChar = 0
                out = append(out, r)
            } else {
                // Escape control characters inside the literal
                switch r {
                case '\n':
                    out = append(out, []rune{'\\', 'n'}...)
                case '\r':
                    out = append(out, []rune{'\\', 'r'}...)
                case '\t':
                    out = append(out, []rune{'\\', 't'}...)
                case '\\':
                    out = append(out, []rune{'\\', '\\'}...)
                default:
                    out = append(out, r)
                }
            }
        }
    }
    return string(out)
}

var altScreen bool

func cleanPasteInput(s string) (string, int) {
    out := make([]rune, 0, len(s))
    removed := 0
    for _, r := range s {
        switch {
        case r == '\n' || r == '\t':
            out = append(out, r)
        case r >= 32 && r != 127:
            out = append(out, r)
        default:
            removed++
            // skip control characters like ESC (\x1b), bell, etc.
        }
    }
    return string(out), removed
}

func multilineEditor(defaultString string, width, height int, boxColour, inputColour, title string) (string, bool, bool) {

    if width <= 0 {
        width = MW - 20
    }
    if height <= 0 {
        height = 1
    }
    if boxColour == "" {
        boxColour = "[#1]"
    }
    if inputColour == "" {
        inputColour = "[#6]"
    }
    if title == "" {
        title = "Multiline Editor"
    }

    currentScreen := "primary"
    if secScreenActive() {
        currentScreen = "secondary"
    }
    if currentScreen == "primary" {
        secScreen()
    } else {
        priScreen()
    }

    cls()
    startRow := (MH - height) / 2
    startCol := (MW - width) / 2

    lines := [][]rune{}
    for _, l := range str.Split(defaultString, "\n") {
        lines = append(lines, []rune(l))
    }
    if len(lines) == 0 {
        lines = append(lines, []rune{})
    }
    lineIndex := len(lines) - 1
    cpos := len(lines[lineIndex])
    indent := 10

    prevHeight := -1
    maxHeight := MH - startRow - 1
    height = len(lines)

    hintOverlay := " ctrl-d to accept, ctrl-r to return top line, escape to abandon "
    spaceRunes := []rune{' ', ' ', ' ', ' '}
    removed := 0

    prevRendered := make([]string, len(lines))

    for {

        hideCursor()

        // fuzzy match
        if filterMode {
            // Draw prompt for filter
            at(startRow+height+2, startCol)
            pf("[#b3]filter> [#-]" + filterQuery + str.Repeat(" ", width-len(filterQuery)-8))

            // Get key input
            c, _, _, _ := getch(0)

            /*
               if len(c) == 1 && c[0] >= 32 && c[0] <= 126 {
                   filterQuery += string(c)
               }
            */

            switch {
            case bytes.Equal(c, []byte{13}): // Enter
                if filterIndex >= 0 && filterIndex < len(filteredLines) {
                    lineIndex = filteredLines[filterIndex]
                    cpos = len(lines[lineIndex])
                }
                filterMode = false
                cls()
                continue

            case bytes.Equal(c, []byte{27}): // ESC
                filterMode = false
                continue

            case bytes.Equal(c, []byte{127}): // backspace
                if len(filterQuery) > 0 {
                    filterQuery = filterQuery[:len(filterQuery)-1]
                }

            case bytes.Equal(c, []byte{0x1B, 0x5B, 0x42}): // DOWN
                if filterIndex < len(filteredLines)-1 {
                    filterIndex++
                }

            case bytes.Equal(c, []byte{0x1B, 0x5B, 0x41}): // UP
                if filterIndex > 0 {
                    filterIndex--
                }

            default:
                if len(c) == 1 && c[0] >= 32 && c[0] <= 126 {
                    filterQuery += string(c)
                }
            }

            // Clear old match lines before drawing new ones
            maxDisplay := min(10, MH-(startRow+height+5))
            for i := 0; i < maxDisplay; i++ {
                at(startRow+height+3+i, startCol)
                pf(str.Repeat(" ", width))
            }

            // Update matches
            filteredLines = fuzzyMatch(filterQuery, lines)
            if len(filteredLines) == 0 {
                filterIndex = 0
            } else if filterIndex >= len(filteredLines) {
                filterIndex = len(filteredLines) - 1
            }

            // Show first X matches
            for i := 0; i < min(maxDisplay, len(filteredLines)); i++ {
                idx := filteredLines[i]
                at(startRow+height+3+i, startCol+2)
                if i == filterIndex {
                    pf("[#invert]" + string(lines[idx]) + "[#-]")
                } else {
                    pf(string(lines[idx]) + str.Repeat(" ", width))
                }
            }

            continue
        }

        if !filterMode && prevHeight != height {
            if height < prevHeight {
                space := str.Repeat(" ", width+2)
                for i := height; i <= prevHeight; i++ { // height + 1 to clear box bottom line
                    at(startRow+i+1, startCol)
                    pf(space)
                }
            }
            drawBox(startRow, startCol, startRow+height+1, startCol+width+1, title)
            at(startRow+height+1, startCol+width-2-len(hintOverlay))
            pf(hintOverlay)
            prevRendered = make([]string, len(lines))
        }

        for i := 0; i < height; i++ {

            at(startRow+1+i, 1+startCol)
            pf(" [#b5][#7]%5d [#-]| ", i+1)

            rendered := string(lines[i])
            if prevRendered[i] != rendered || lineIndex == i {

                //                at(startRow+1+i, 1+startCol)
                //                pf(" [#b5][#7]%5d [#-]| ",i+1)

                visibleWidth := width - indent
                if visibleWidth < 0 {
                    visibleWidth = 0
                }
                line := lines[i]
                displayRunes := line
                indicator := ""

                // Truncate to fit within visibleWidth-1 and add "…"
                if displayedLen(string(line)) > visibleWidth {
                    cutoff := findRunesThatFit(line, visibleWidth-1)
                    displayRunes = line[:cutoff]
                    indicator = "…"
                }

                pf(str.Repeat(" ", visibleWidth))
                at(startRow+i+1, startCol+indent)
                pf(string(displayRunes) + indicator)

                prevRendered[i] = rendered
            }

        }

        if removed > 0 {
            at(startRow-1, startCol+2)
            pf("[#b2][#7]⚠ %d control characters removed from paste[#-]", removed)
            removed = 0
        }

        at(startRow+1+lineIndex, startCol+cpos+indent)
        pf("\033]12;red\a")
        showCursor()

        c, _, pasted, pbuf := getch(0)

        if pasted {
            // Strip ANSI codes
            pbuf = Strip(pbuf)
            // then clean the rest of the jank
            pbuf, removed = cleanPasteInput(pbuf)

            // Split pasted buffer into lines
            pasteLines := str.Split(pbuf, "\n")

            // Insert first pasted line into current line at cursor
            firstLineRunes := []rune(pasteLines[0])
            lines[lineIndex] = append(lines[lineIndex][:cpos], append(firstLineRunes, lines[lineIndex][cpos:]...)...)
            cpos += len(firstLineRunes)

            // Insert remaining pasted lines as new lines in editor
            for i := 1; i < len(pasteLines); i++ {
                lineIndex++
                if lineIndex >= len(lines) {
                    lines = append(lines, []rune{})
                }
                lines = append(lines[:lineIndex], append([][]rune{[]rune(pasteLines[i])}, lines[lineIndex:]...)...)
                cpos = len([]rune(pasteLines[i]))
            }
            prevHeight = height
            height = len(lines)
            if height > maxHeight {
                height = maxHeight
            }
            cls()
            continue
        }

        switch {
        case bytes.Equal(c, []byte{0x1B}): // ESC
            cls()
            if currentScreen == "primary" {
                priScreen()
            } else {
                secScreen()
            }
            return "", false, true
        case bytes.Equal(c, []byte{4}): // Ctrl+D
            cls()
            if currentScreen == "primary" {
                priScreen()
            } else {
                secScreen()
            }
            var out str.Builder
            for i, l := range lines {
                out.WriteString(string(l))
                if i != len(lines)-1 {
                    out.WriteRune('\n')
                }
            }
            // return out.String(), false, false
            return escapeControlCharsInLiterals(out.String()), false, false
        case bytes.Equal(c, []byte{0x06}): // Ctrl-F // fuzzy match
            filterMode = true
            filterQuery = ""
            filteredLines = []int{}
            filterIndex = 0
            continue
        case bytes.Equal(c, []byte{18}): // Ctrl+R
            cls()
            if currentScreen == "primary" {
                priScreen()
            } else {
                secScreen()
            }
            // if len(lines) > 0 { return string(lines[0]), true, false }
            if len(lines) > 0 {
                return escapeControlCharsInLiterals(string(lines[0])), true, false
            }
            return "", true, false

        /*
           case bytes.Equal(c, []byte{13}): // Enter
               if len(lines)+1>=MH-startRow-1 {
                   break
               }
               lines = append(lines, []rune{})
               lineIndex++
               cpos = 0
               prevHeight=height
               height=len(lines)
               if height>maxHeight {
                   height=maxHeight
               }
        */

        case bytes.Equal(c, []byte{13}): // Enter key
            if len(lines)+1 >= MH-startRow-1 {
                break
            }

            // Get current line and split at cursor position
            cur := lines[lineIndex]
            left := cur[:cpos]
            right := cur[cpos:]

            // Replace current line with the left half
            lines[lineIndex] = left

            // Insert right half as a new line below
            lines = append(lines[:lineIndex+1], append([][]rune{right}, lines[lineIndex+1:]...)...)

            // Move cursor to new line start
            lineIndex++
            cpos = 0

            // Adjust height
            prevHeight = height
            height = len(lines)
            if height > maxHeight {
                height = maxHeight
            }

        case bytes.Equal(c, []byte{0x1B, 0x5B, 0x33, 0x7E}): // DEL
            if cpos < len(lines[lineIndex]) {
                // Remove character under cursor
                lines[lineIndex] = removeBefore(lines[lineIndex], cpos+1)
            } else if len(lines[lineIndex]) == 0 && len(lines) > 1 {
                // Remove the empty line
                lines = append(lines[:lineIndex], lines[lineIndex+1:]...)
                if lineIndex >= len(lines) {
                    lineIndex = len(lines) - 1
                }
                cpos = 0
            }

        case bytes.Equal(c, []byte{0x09}): // TAB key
            lines[lineIndex] = append(lines[lineIndex][:cpos], append(spaceRunes, lines[lineIndex][cpos:]...)...)
            cpos += 4

        case bytes.Equal(c, []byte{127}): // Backspace
            if cpos > 0 {
                lines[lineIndex] = removeBefore(lines[lineIndex], cpos)
                cpos--
            } else if lineIndex > 0 {
                prevLen := len(lines[lineIndex-1])
                lines[lineIndex-1] = append(lines[lineIndex-1], lines[lineIndex]...)
                lines = append(lines[:lineIndex], lines[lineIndex+1:]...)
                lineIndex--
                // height--
                prevHeight = height
                height = len(lines)
                if height > maxHeight {
                    height = maxHeight
                }
                cpos = prevLen
            }
        case bytes.Equal(c, []byte{0x1B, 0x5B, 0x41}): // UP
            if lineIndex > 0 {
                lineIndex--
                cpos = min(cpos, len(lines[lineIndex]))
            }
        case bytes.Equal(c, []byte{0x1B, 0x5B, 0x42}): // DOWN
            if lineIndex < len(lines)-1 {
                lineIndex++
                cpos = min(cpos, len(lines[lineIndex]))
            }
        case bytes.Equal(c, []byte{0x1B, 0x5B, 0x44}): // LEFT
            if cpos > 0 {
                cpos--
            } else if lineIndex > 0 {
                lineIndex--
                cpos = len(lines[lineIndex])
            }
        case bytes.Equal(c, []byte{0x1B, 0x5B, 0x43}): // RIGHT
            if cpos < len(lines[lineIndex]) {
                cpos++
            } else if lineIndex < len(lines)-1 {
                lineIndex++
                cpos = 0
            }
        case bytes.Equal(c, []byte{1}): // Ctrl+A
            cpos = 0
        case bytes.Equal(c, []byte{5}): // Ctrl+E
            cpos = len(lines[lineIndex])
        case len(c) == 1 && c[0] >= 32 && c[0] < 127:
            r := rune(c[0])
            lines[lineIndex] = insertAt(lines[lineIndex], cpos, r)
            cpos++
        }
    }
}

// fuzzy filtering for multiline mode

var filterMode bool = false
var filterQuery string
var filteredLines []int
var filterIndex int

func fuzzyMatch(query string, lines [][]rune) []int {
    matches := []int{}
    q := str.ToLower(query)
    for i, line := range lines {
        if fuzzyScore(q, str.ToLower(string(line))) > 0 {
            matches = append(matches, i)
        }
    }
    return matches
}

func fuzzyScore(needle, haystack string) int {
    ni := 0
    for hi := 0; hi < len(haystack) && ni < len(needle); hi++ {
        if haystack[hi] == needle[ni] {
            ni++
        }
    }
    if ni == len(needle) {
        return ni
    }
    return 0
}

func clearChars(row int, col int, l int) {
    at(row, col)
    fmt.Print(str.Repeat(" ", l))
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
func printWithNLRespect(s string, p Pane) {
    var newStr str.Builder
    for i := 0; i < len(s); i++ {
        if col == p.w-1 {
            newStr.WriteString(sf("\n\033[%dG", ocol+1))
            col = 1
            row++
        }
        switch s[i] {
        case '\n':
            newStr.WriteString(sf("\n\033[%dG", ocol+1))
            col = 1
            row++
        default:
            newStr.WriteByte(s[i])
            col += 1
        }
    }
    fmt.Print(newStr.String())
}

// print with line wrap at non-global pane end
func printWithWrap(s string) {
    if currentpane != "global" {
        if p, ok := panes[currentpane]; ok {
            printWithNLRespect(s, p)
        } else {
            fmt.Print(s)
        }
    } else {
        fmt.Print(s)
    }
}

// generic vararg print handler. also moves cursor in interactive mode
// @TODO: this could use some attention to reduce the differences
//
//  between interactive/non-interactive source
func pf(s string, va ...any) {

    s = sf(sparkle(s), va...)
    sna := Strip(s)

    if interactive {
        if lineWrap {
            printWithWrap(s)
        } else {
            fmt.Print(s)
        }
        chpos := 0
        c := col
        for ; chpos < len(sna); c += 1 {
            if c%MW == 0 {
                row++
                c = 0
            }
            if sna[chpos] == '\n' {
                row++
                c = 0
            }
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
    chpos := 0
    c := col
    for ; chpos < len(sna); c += 1 {
        if c%MW == 0 {
            row++
            c = 0
        }
        if sna[chpos] == '\n' {
            row++
            c = 0
        }
        chpos++
    }
    atlock.Unlock()

}

// apply ansi code translation to inbound strings
func sparkle(a any) string {
    switch a.(type) {
    case string:
        return fairyReplacer.Replace(a.(string))
    }
    return sf(`%v`, a)
}

// stripTrailingNewlines removes trailing \n and \r\n from log messages for file output
func stripTrailingNewlines(s string) string {
    // Handle Windows CRLF first
    if runtime.GOOS == "windows" && str.HasSuffix(s, "\r\n") {
        return s[:len(s)-2]
    }
    // Handle Unix LF
    if str.HasSuffix(s, "\n") {
        return s[:len(s)-1]
    }
    return s
}

// logging output printer
func plog(s string, va ...any) {

    // print if not silent logging (default is to print)
    shouldPrint := true
    if v, exists := gvget("@silentlog"); exists && v != nil {
        if silent, ok := v.(bool); ok && silent {
            shouldPrint = false
        }
    }
    if shouldPrint {
        pf(s+"\n", va...)
    }

    // Queue log request if logging enabled
    if loggingEnabled {
        message := sf(s, va...)
        request := LogRequest{
            Message:   message,
            Fields:    nil, // Plain text logging has no fields
            IsJSON:    false,
            IsError:   false,
            Timestamp: time.Now(),
        }
        queueLogRequest(request)
    }
}

// JSON logging output printer
func plog_json(message string, fields map[string]any, va ...any) {

    // Build JSON log entry for console output
    logEntry := make(map[string]any)
    logEntry["message"] = sf(message, va...)
    logEntry["timestamp"] = time.Now().Format(time.RFC3339)

    // Add subject if set
    if subj, exists := gvget("@logsubject"); exists && subj != nil {
        if subjStr, ok := subj.(string); ok && subjStr != "" {
            logEntry["subject"] = subjStr
        }
    }

    // Add custom fields
    for k, v := range fields {
        logEntry[k] = v
    }

    // Convert to JSON
    jsonBytes, err := json.Marshal(logEntry)
    if err != nil {
        // Fallback to regular logging if JSON fails
        plog(message, va...)
        return
    }

    jsonString := string(jsonBytes)

    // Print if not silent logging
    shouldPrint := true
    if v, exists := gvget("@silentlog"); exists && v != nil {
        if silent, ok := v.(bool); ok && silent {
            shouldPrint = false
        }
    }
    if shouldPrint {
        pf("%s\n", jsonString)
    }

    // Queue log request if logging enabled
    if loggingEnabled {
        // Create a copy of fields for the queue
        fieldsCopy := make(map[string]any)
        for k, v := range fields {
            fieldsCopy[k] = v
        }

        request := LogRequest{
            Message:   sf(message, va...),
            Fields:    fieldsCopy,
            IsJSON:    true,
            IsError:   false,
            Timestamp: time.Now(),
        }
        queueLogRequest(request)
    }
}

// special case printing for global var interpolation
func gpf(ns string, s string) {
    pf("%s\n", spf(ns, 0, &gident, s))
}

// sprint with function space
func spf(ns string, fs uint32, ident *[]Variable, s string) string {
    s = interpolate(ns, fs, ident, s)
    return sf("%v", sparkle(s))
}

// clear screen
func cls() {
    isWinTerm := false
    if v, exists := gvget("@winterm"); exists && v != nil {
        if wt, ok := v.(bool); ok {
            isWinTerm = wt
        }
    }

    if !isWinTerm {
        pf("\033c")
    } else {
        pf("\033[2J")
    }
    at(1, 1)
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
var strip_re = regexp.MustCompile(ansi)

func Strip(s string) string {
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

func rsparkle(ra []rune) []rune {
    return []rune(sparkle(string(ra)))
}

func displayedLenUtf8(s []rune) int {
    return len([]rune(Strip(string(rsparkle(s)))))
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
func sat(row int, col int) string {
    return sf("\033[%d;%dH", orow+row, ocol+col)
}

// clear to end of line
func clearToEOL() {
    pf("\033[0K")
}

func clearToEOP(start int) {
    if currentpane == "global" {
        pf("\033[0K")
    } else {
        pf(str.Repeat(" ", panes[currentpane].w-panes[currentpane].col-start))
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
    pf("\033[%dG", n)
}

func removeAllBefore(runes []rune, pos int) []rune {
    if len(runes) < pos {
        return runes
    }
    return runes[pos:]
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

func insertWord(runes []rune, cpos int, word string) []rune {
    // Insert each rune of the word at cpos
    wordRunes := []rune(word)
    newRunes := append(runes[:cpos], append(wordRunes, runes[cpos:]...)...)
    return newRunes
}

func deleteWord(runes []rune, cpos int) ([]rune, int) {
    start := 0
    end := len(runes)

    if end < cpos {
        return runes, 0
    }

    // Scan backwards for the start of the word (or dot)
    for p := cpos - 1; p >= 0; p-- {
        if runes[p] == '.' {
            start = p + 1
            break
        }
        if runes[p] == ' ' {
            start = p + 1
            break
        }
    }

    // Scan forward for the end of the word (or dot)
    for p := cpos; p < len(runes); p++ {
        if runes[p] == ' ' || runes[p] == '.' {
            end = p
            break
        }
    }

    startsub := []rune{}
    endsub := []rune{}

    if start > 0 {
        startsub = runes[:start]
    }

    add := []rune{}
    if end < len(runes) {
        if start != 0 {
            add = []rune{' '}
        }
        endsub = runes[end+1:]
    }

    rstring := append(startsub, append(add, endsub...)...)

    return rstring, start
}

func getWord(runes []rune, cpos int) ([]rune, int) {
    if cpos > len(runes) {
        cpos = len(runes)
    }
    start := cpos
    for start > 0 && runes[start-1] != ' ' {
        start--
    }
    end := cpos
    for end < len(runes) && runes[end] != ' ' {
        end++
    }
    return runes[start:end], start
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
            fmt.Print(rep(" ", p.w-1))
        }
    } else {
        at(row, col)
        fmt.Print(rep(" ", int(p.w-col-2)))
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
    fmt.Print(tl)
    absat(p.row, p.col+p.w-1)
    fmt.Print(tr)
    absat(p.row+p.h, p.col+p.w-1)
    fmt.Print(br)
    absat(p.row+p.h, p.col)
    fmt.Print(bl)

    // top, bottom
    absat(p.row, p.col+1)
    fmt.Print(rep(tlr, int(p.w-2)))
    absat(p.row+p.h, p.col+1)
    fmt.Print(rep(blr, int(p.w-2)))

    // left, right
    for r := p.row + 1; r < p.row+p.h; r++ {
        absat(r, p.col)
        fmt.Print(ud)
        absat(r, p.col+p.w-1)
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
func NewCoprocess(loc string, args ...string) (process *exec.Cmd, pi io.WriteCloser, po io.ReadCloser, pe io.ReadCloser) {

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

    c = str.Trim(c, " \t\n")
    bargs := str.Split(c, " ")
    cmd := exec.Command(bargs[0], bargs[1:]...)
    var out bytes.Buffer
    cmd.Stdin = os.Stdin
    capture, _ := gvget("@commandCapture")

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

    CMDSEP, _ := gvget("@cmdsep")
    cmdsep := CMDSEP.(byte)

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
                s = append(s, v)
                if v == 10 {
                    if mt.(bool) {
                        pf("⟊")
                    }
                    t.Reset(dur)
                }
            }

            if err == io.EOF {
                if v != 0 {
                    s = append(s, v)
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

// / mutex for shell calls
// / used by Copper()+NextCopper()+GetCommand()
var cmdlock = &sync.Mutex{}

// submit a command for coprocess execution
func Copper(line string, squashErr bool) struct {
    out  string
    err  string
    code int
    okay bool
} {

    if !permit_shell {
        panic(fmt.Errorf("Shell calls not permitted!"))
    }

    // remove some bad conditions...
    if str.HasSuffix(str.TrimRight(line, " "), "|") {
        return struct {
            out  string
            err  string
            code int
            okay bool
        }{"", "", -1, false}
    }
    if tr(line, DELETE, "| ", "") == "" {
        return struct {
            out  string
            err  string
            code int
            okay bool
        }{"", "", -1, false}
    }
    line = str.TrimRight(line, "\n")

    var ns []byte
    var errout string // stderr output
    var errint int    // coprocess return code
    var err error     // generic error handle
    var commandErr error

    riwp, _ := gvget("@runInWindowsParent")
    rip, _ := gvget("@runInParent")

    // shell reporting option:
    sr, _ := gvget("@shell_report")

    if sr.(bool) == true {
        noshell, _ := gvget("@noshell")
        shelltype, _ := gvget("@shelltype")
        shellloc, _ := gvget("@shell_location")
        if !noshell.(bool) {
            pf("[#4]Shell Options: ")
            pf("%v (%v) ", shelltype, shellloc)
            if riwp.(bool) {
                pf("Windows ")
            }
            if rip.(bool) {
                pf("in parent\n[#-]")
            } else {
                pf("in coproc\n[#-]")
            }
            pf("[#4]command : [%s][#-]\n", line)
        }
    }

    gvset("@lastcmd", line)

    if riwp.(bool) || rip.(bool) {

        if riwp.(bool) {
            var ba string
            ba, err = GetCommand("cmd /c " + line)
            ns = []byte(ba)
        } else {
            var ba string
            ba, err = GetCommand(line)
            ns = []byte(ba)
        }

        gvset("@last", 0)
        gvset("@last_err", []byte{0})

        if exitError, ok := err.(*exec.ExitError); ok {
            errint = exitError.ExitCode()
            errout = err.Error()
        }
        gvset("@last", errint)
        gvset("@last_err", string(errout))

    } else {

        cmdlock.Lock()
        defer cmdlock.Unlock()

        errorFile, err := ioutil.TempFile("", "copper.*.err")
        if err != nil {
            os.Remove(errorFile.Name())
            log.Fatal(err)
        }
        defer os.Remove(errorFile.Name())
        gvset("@last", 0)

        read_out := bufio.NewReader(po)

        // issue command
        CMDSEP, _ := gvget("@cmdsep")
        cmdsep := CMDSEP.(byte)
        hexenc := hex.EncodeToString([]byte{cmdsep})
        io.WriteString(pi, "\n"+line+` 2>`+errorFile.Name()+` ; last=$? ; echo -en "\x`+hexenc+`${last}\x`+hexenc+`"`+"\n")

        // get output
        ns, commandErr = NextCopper(line, read_out)
        // pf("[copper] line -> <%s>\n", line)
        // pf("[copper] ns   -> <%s>\n", ns)

        // get status code - cmd is not important for this, NextCopper just reads
        //  the output until the next cmdsep
        code, err := NextCopper("#Status", read_out)
        // pull cwd from /proc
        childProc, _ := gvget("@shell_pid")

        cwd, _ := os.Readlink(sf("/proc/%v/cwd", childProc))
        prevdir, _ := gvget("@cwd")
        if cwd != prevdir {
            err = syscall.Chdir(cwd)
            gvset("@cwd", cwd)
        }

        if commandErr != nil {
            errint = -3
            lastlock.Lock()
            coproc_reset = true
            lastlock.Unlock()
            os.Remove(errorFile.Name())
            procKill(os.Getpid())
            return struct {
                out  string
                err  string
                code int
                okay bool
            }{"", "interrupt", -3, false}
        } else {
            if err == nil {
                errint, err = strconv.Atoi(string(code))
                if err != nil {
                    errint = -2
                }
                if !squashErr {
                    gvset("@last", errint)
                }
            } else {
                errint = -1
            }
        }

        // get stderr file
        b, err := ioutil.ReadFile(errorFile.Name())

        if len(b) > 0 {
            errout = string(b)
        } else {
            errout = ""
        }
        gvset("@last_err", errout)

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

    return struct {
        out  string
        err  string
        code int
        okay bool
    }{string(ns), errout, errint, errint == 0}
}

func restoreScreen() {
    pf("\033c") // reset screen
    pf("\033[u")
}

func testStart(file string) {
    vos, _ := gvget("@os")
    stros := vos.(string)
    test_start := sf("\n[#6][#ul][#bold]Za Test[#-]\n\nTesting : %s on "+stros+"\n", file)
    appendToTestReport(test_output_file, 0, 0, test_start)
}

func testExit() {
    test_final := sf("\n[#6]Tests Performed %d -- Tests Failed %d -- Tests Passed %d[#-]\n\n", testsPassed+testsFailed, testsFailed, testsPassed)
    appendToTestReport(test_output_file, 0, 0, test_final)
}
