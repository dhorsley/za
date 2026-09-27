package main

import (
    "bufio"
    "bytes"
    "encoding/hex"
//    "encoding/json"
    "errors"
    "fmt"
    "io"
    "io/ioutil"
    "log"
    "os"
    "os/exec"
    "path/filepath"
    "regexp"
    "runtime"
    "sort"
    "strconv"
    str "strings"
    "sync"
    "syscall"
    "time"
    "unicode/utf8"

    "github.com/VictoriaMetrics/metrics"
)

var completions = []string{"VAR", "GLOBAL", "SETGLOB", "PAUSE",
    "HELP", "NOP", "REQUIRE", "EXIT", "VERSION",
    "QUIET", "LOUD", "UNSET", "INPUT", "PROMPT", "LOG", "PRINT", "PRINTLN",
    "LOGGING", "CLS", "AT", "DEFINE", "SHOWDEF", "ENDDEF", "RETURN", "ASYNC", "YIELD", "EMIT",
    "MODULE", "USE", "USES", "WHILE", "ENDWHILE", "FOR", "FOREACH",
    "ENDFOR", "CONTINUE", "BREAK", "ON", "DO", "IF", "ELSE", "ENDIF", "CASE",
    "IS", "CONTAINS", "HAS", "IN", "OR", "ENDCASE", "WITH", "ENDWITH",
    "STRUCT", "ENDSTRUCT", "SHOWSTRUCT",
    "TRY", "CATCH", "ENDTRY", "THEN",
    "PANE", "DOC", "TEST", "ENDTEST", "ASSERT", "TO", "STEP", "AS", "ENUM", "HIST",
}

// const ansi = "[\u001B\u009B][[\\]()#;?]*(?:(?:(?:[a-zA-Z\\d]*(?:;[a-zA-Z\\d]*)*)?\u0007)|(?:(?:\\d{1,4}(?:;\\d{0,4})*)?[\\dA-PRZcf-ntqry=><~]))"
const ansi = "[\u001B\u009B](?:[@-Z\\-_]|[[0-?]*[ -/]*[@-~])"

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
        fairydust["SOL"] = "\033[1G"
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
            "fbblue", "fbmagenta", "fbcyan", "fwhite", "dim", "blink", "hidden", "crossed", "framed", "CSI", "CTE", "ASB", "RSB", "SOL", ".",
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

func removeProcessedKeycode(ka *[]byte, count int) {
    if len(*ka) > (count - 1) {
        *ka = (*ka)[count:]
    }
}

func buildHelpPath(wordUnderCursor []rune, helpColoured *[]string, helpList *[]string, helpType *[]int, max_depth int) (fileList map[string]os.FileInfo) {

    fileList = make(map[string]os.FileInfo)

    s:=string(wordUnderCursor)
    if !str.HasPrefix(s,"/") {
        s="./"+s
    }
    onSlash:=str.HasSuffix(s,"/")
    parent    := filepath.Dir(s)
    searchName:= filepath.Base(s)

    // literal, case-insensitive prefix matching (fish-style): the typed
    // text — dots, plus signs, brackets and all — is plain text for
    // filename completion, never a regex.
    lsearch := str.ToLower(searchName)

    for _, paf := range dirplus(parent, max_depth) {

        name := paf.DirEntry.Name() // each file name

        if !onSlash && !str.HasPrefix(str.ToLower(name), lsearch) {
            continue
        }

        pan := parent+"/"+name
        if parent=="/" {
            pan = "/"+name
        }

        if _,found:=fileList[name]; found {
            continue            // this rejects multiple same dirents
        }

        f, err := os.Stat(pan)

        if err==nil {
            appendEntry := ""
            if f.IsDir() {
                appendEntry += "[#3]"
            } else {
                appendEntry += "[#4]"
            }
            appendEntry += name + "[#-]"
            *helpColoured = append(*helpColoured, appendEntry)
            *helpList = append(*helpList, name)
            *helpType = append(*helpType, HELP_DIRENT)
            fileList[name] = f
        }
    }
    return fileList
}


// ---------------------------------------------------------------------------
// Fish-style autosuggestion for the REPL line editor.
// The candidate pipeline is shared with the legacy default-string hint (see
// getInput): a "suggestion" string whose prefix is the typed input gets its
// unmatched tail rendered dim/italic after the caret, accepted with →/Ctrl-F.
// ---------------------------------------------------------------------------

// lazily built, alphabetically sorted list of stdlib function names, used as
// a catalog fallback when no history entry matches the typed prefix
var hintFuncnames []string

func buildHintFuncnames() {
	if hintFuncnames == nil {
		for k := range slhelp {
			hintFuncnames = append(hintFuncnames, k)
		}
		sort.Strings(hintFuncnames)
	}
}

// replHint returns the best suggestion for the given typed input:
//  1. the most recent history entry that starts with input (fish-style),
//  2. otherwise the first (alphabetical) stdlib function name that starts
//     with a non-empty input,
//  3. otherwise the first keyword that starts with a non-empty input.
// Empty input only suggests from history (the previous command).
func replHint(input string) string {
	buildHintFuncnames()

	for i := len(hist) - 1; i >= 0; i-- {
		h := hist[i]
		if h == input || h == "" {
			continue
		}
		if str.HasPrefix(h, input) {
			return h
		}
	}

	if input == "" {
		return ""
	}

	// path suggestions are ranked after history (fish's autosuggestion is
	// history-first) but before the function/keyword fallbacks: a matching
	// entry in the current directory — directories completing with a
	// trailing slash — usually means the user is navigating.
	if ps := pathSuggestion(input); ps != "" && ps != input {
		return ps
	}

	for _, f := range hintFuncnames {
		if f == input {
			continue
		}
		if str.HasPrefix(f, input) {
			return f
		}
	}

	for _, kw := range completions {
		if kw == input {
			continue
		}
		if str.HasPrefix(kw, input) {
			return kw
		}
	}

	return ""
}

// suggestionMarkup wraps a suggestion tail in dim+italic (optionally with the
// user-configured foreground colour) ready for console output. Colours go
// through the fairydust interpolation so the -c monochrome flag is honoured.
func suggestionMarkup(tail string) string {
	markup := sparkle("[#dim][#i1]") // dim + italic
	if len(autocompleteColours) > 0 && autocompleteColours[0] != "" {
		markup += sparkle(autocompleteColours[0])
	}
	markup += tail
	markup += sparkle("[#i0][#boff]")
	if len(autocompleteColours) > 0 {
		markup += sparkle("[#fd]")
	}
	return markup
}

// capGhostTail truncates a suggestion tail so it fits the remaining columns
// on the input's last row (runes are the unit; wide glyphs count as one
// column, matching displayedLen's convention). Returns "" when the input
// already fills the row.
func capGhostTail(tail string, icol, inputL int) string {
	startCol := icol
	if inputL > 0 {
		startCol = ((icol + inputL - 1) % MW) + 2 // column right after the last input char
	}
	if startCol > MW {
		return ""
	}
	remaining := MW - startCol + 1
	r := []rune(tail)
	if len(r) > remaining {
		r = r[:remaining]
	}
	return string(r)
}

// acceptSuggestionWord appends the first whitespace-delimited word of the
// suggestion tail to the input (fish alt-right/alt-f behaviour). The tail may
// be a continuation of the final token (path completion, e.g. "ls doc" ->
// "ls docs/") or the start of a following argument (history, e.g. "git" ->
// "git status"); a space is inserted only when the suggestion separates the
// next word with one. Returns the new input runes, the new cursor position,
// and whether anything was added.
func acceptSuggestionWord(s []rune, cand string) ([]rune, int, bool) {
	cur := string(s)
	if !str.HasPrefix(cand, cur) || cand == cur {
		return s, 0, false
	}
	tail := cand[len(cur):]
	tail = str.TrimLeft(tail, " ")
	if tail == "" {
		return s, 0, false
	}
	sep := ""
	if cur != "" && !str.HasSuffix(cur, " ") && len(cur) < len(cand) && cand[len(cur)] == ' ' {
		sep = " "
	}
	sp := str.IndexByte(tail, ' ')
	word := tail
	if sp != -1 {
		word = tail[:sp]
	}
	if word == "" {
		return s, 0, false
	}
	ns := cur + sep + word
	r := []rune(ns)
	return r, len(r), true
}

// wordStep moves the cursor forward (+1) or backward (-1) by one
// whitespace-delimited word, clamping at the input ends (fish-style word
// motion for shift/ctrl/alt arrow keys).
func wordStep(s []rune, cpos int, dir int) int {
	n := len(s)
	if dir > 0 {
		for cpos < n && s[cpos] == ' ' {
			cpos++
		}
		for cpos < n && s[cpos] != ' ' {
			cpos++
		}
	} else {
		for cpos > 0 && s[cpos-1] == ' ' {
			cpos--
		}
		for cpos > 0 && s[cpos-1] != ' ' {
			cpos--
		}
	}
	return cpos
}

// escKeyResult describes a decoded modified-arrow / function-key sequence.
type escKeyResult struct {
	dir    int  // -1 left, +1 right; 0 for accept/consume-only
	word   bool // move by word (a modifier was present) vs one character
	accept bool // word-accept of the suggestion (alt-f)
}

// decodeEscKey interprets escape-sequence encodings not enumerated as
// explicit cases: SS3 plain arrows (ESC O C/D), CSI-u modified arrows
// (ESC [ k ; m u, kitty/ghostty: 1..4 = left/right/up/down) and the
// alt-f forms (ESC f and kitty-style ESC [ 102 ; 3 u). The modifier value
// is shared with xterm's CSI (2=shift, 3=alt, 5=ctrl), so xterm's own
// ESC [ 1 ; m C/D also decode here if the explicit cases ever miss them.
func decodeEscKey(c []byte) (esc escKeyResult, ok bool) {
	if len(c) < 2 || c[0] != 0x1B {
		return
	}
	if len(c) == 2 && c[1] == 'f' {
		return escKeyResult{accept: true}, true // ESC f = alt-f
	}
	if c[1] != '[' && c[1] != 'O' {
		return
	}
	final := c[len(c)-1]
	if c[1] == 'O' { // SS3: ESC O C = right, ESC O D = left, A/B = up/down
		switch final {
		case 'C':
			return escKeyResult{dir: +1}, true
		case 'D':
			return escKeyResult{dir: -1}, true
		}
		return escKeyResult{}, true // up/down handled by explicit cases elsewhere
	}
	// CSI: parse "n" or "n;m" parameters before the final byte
	body := c[2 : len(c)-1]
	sep := bytes.IndexByte(body, ';')
	var p1, p2 int
	var err error
	if sep == -1 {
		p1, err = strconv.Atoi(string(body))
		p2 = 1
	} else {
		p1, err = strconv.Atoi(string(body[:sep]))
		if err != nil {
			return
		}
		p2, err = strconv.Atoi(string(body[sep+1:]))
		if err != nil {
			return
		}
	}
	word := p2 != 1
	switch final {
	case 'C':
		return escKeyResult{dir: +1, word: word}, true
	case 'D':
		return escKeyResult{dir: -1, word: word}, true
	case 'u': // kitty/ghostty CSI-u
		switch p1 {
		case 1:
			return escKeyResult{dir: -1, word: word}, true // left
		case 2:
			return escKeyResult{dir: +1, word: word}, true // right
		case 102: // 'f'
			if p2 == 3 {
				return escKeyResult{accept: true}, true // alt-f
			}
			return escKeyResult{}, true
		}
		return escKeyResult{}, true
	}
	return
}

// fuzzyTermMatch reports whether term matches target as a case-insensitive
// substring (the classic behaviour) or, failing that, as a subsequence
// (fish-style fuzzy history search).
func fuzzyTermMatch(term, target string) bool {
	lowT := str.ToLower(term)
	lowG := str.ToLower(target)
	if lowT == "" {
		return true
	}
	if str.Contains(lowG, lowT) {
		return true
	}
	ti := 0
	for i := 0; i < len(lowG) && ti < len(lowT); i++ {
		if lowG[i] == lowT[ti] {
			ti++
		}
	}
	return ti == len(lowT)
}

// pathSuggestion builds a directory/file completion candidate for the final
// whitespace-delimited token of input and returns it as a full-line suggestion
// (so the ghost-tail prefix invariant "candidate starts with the input" holds).
// It matches literally and case-insensitively, fish-style, preferring a
// matching directory (trailing slash) over a file, and works against the
// current directory even without a '/'. "" is returned when nothing matches so
// history/function/keyword suggestions take over.
func pathSuggestion(input string) string {
	sp := str.LastIndexByte(input, ' ')
	prefix, tok := "", input
	if sp != -1 {
		prefix = input[:sp+1]
		tok = input[sp+1:]
	}
	if tok == "" || tok == "/" {
		return ""
	}

	// mirror buildHelpPath's normalisation to reconstruct completed paths
	searchName := filepath.Base(tok)
	head := ""
	if str.HasSuffix(tok, "/") {
		// user already typed the trailing slash: suggest any entry inside
		searchName = ""
		head = tok
	} else {
		if searchName == "." || searchName == ".." || searchName == "" {
			return ""
		}
		if str.HasSuffix(tok, searchName) {
			head = tok[:len(tok)-len(searchName)]
		}
	}

	// scan only the immediate level the token lives in: the current directory
	// for slash-less tokens (fish suggests the visible entries, not recursive
	// ones), otherwise the token's own parent.
	var cols []string
	var list []string
	var types []int
	fl := buildHelpPath([]rune(tok), &cols, &list, &types, 0)

	cand := ""
	for _, nm := range list {
		if searchName != "" && !str.HasPrefix(nm, searchName) {
			continue
		}
		if cand == "" {
			cand = nm
		}
		if fi, ok := fl[nm]; ok && fi.IsDir() {
			cand = nm
			break
		}
	}
	if cand == "" {
		return ""
	}
	if fi, ok := fl[cand]; ok && fi.IsDir() && !str.HasSuffix(cand, "/") {
		cand += "/"
	}
	return prefix + head + cand
}

// defaultSyntaxColours are the built-in colour classes used for live REPL
// syntax highlighting. Every class can be overridden (or disabled with an
// empty string) through the syntax_colours() stdlib call.
var defaultSyntaxColours = map[string]string{
	"keyword":      "[#bold][#6]",
	"function":     "[#5]",
	"string":       "[#2]",
	"number":       "[#1]",
	"comment":      "[#dim]",
	"variable":     "[#1]",
	"shellcommand": "[#1]",
	"error":        "[#2]",
	"operator":     "[#6]",
}

// syntaxColourOverride holds per-class overrides set by syntax_colours();
// an override of "" disables the class.
var syntaxColourOverride = struct {
	sync.Mutex
	m map[string]string
}{m: map[string]string{}}

// syntaxColourClassValid reports whether a colour-class name is recognised.
func syntaxColourClassValid(c string) bool {
	switch c {
	case "keyword", "function", "string", "number", "comment",
		"variable", "shellcommand", "error", "operator":
		return true
	}
	return false
}

// syntaxColour returns the effective markup for a colour class, or "" when
// the class has been disabled.
func syntaxColour(class string) string {
	syntaxColourOverride.Lock()
	v, ok := syntaxColourOverride.m[class]
	syntaxColourOverride.Unlock()
	if ok {
		return v
	}
	return defaultSyntaxColours[class]
}

// currentSyntaxColourMap returns the effective colour map (defaults plus any
// overrides) for the syntax_colours() stdlib call.
func currentSyntaxColourMap() map[string]any {
	syntaxColourOverride.Lock()
	defer syntaxColourOverride.Unlock()
	out := make(map[string]any, len(defaultSyntaxColours))
	for k, v := range defaultSyntaxColours {
		if ov, ok := syntaxColourOverride.m[k]; ok {
			out[k] = ov
		} else {
			out[k] = v
		}
	}
	return out
}

// setSyntaxColours applies colour-class overrides and returns the previous
// effective map.
func setSyntaxColours(m map[string]any) (map[string]any, error) {
	syntaxColourOverride.Lock()
	defer syntaxColourOverride.Unlock()
	prev := make(map[string]any, len(defaultSyntaxColours))
	for k, v := range defaultSyntaxColours {
		if ov, ok := syntaxColourOverride.m[k]; ok {
			prev[k] = ov
		} else {
			prev[k] = v
		}
	}
	for k, v := range m {
		if !syntaxColourClassValid(k) {
			return prev, fmt.Errorf("syntax_colours: unknown colour class \"%s\"", k)
		}
		s, ok := v.(string)
		if !ok {
			return prev, fmt.Errorf("syntax_colours: class \"%s\" must be a [#colour] string", k)
		}
		if s != "" && !str.HasPrefix(s, "[#") {
			return prev, fmt.Errorf("syntax_colours: invalid colour format for \"%s\", expected [#color] markup", k)
		}
		syntaxColourOverride.m[k] = s
	}
	return prev, nil
}

// resetSyntaxColours removes all overrides and returns the previous map.
func resetSyntaxColours() map[string]any {
	syntaxColourOverride.Lock()
	defer syntaxColourOverride.Unlock()
	prev := make(map[string]any, len(defaultSyntaxColours))
	for k, v := range defaultSyntaxColours {
		if ov, ok := syntaxColourOverride.m[k]; ok {
			prev[k] = ov
		} else {
			prev[k] = v
		}
	}
	syntaxColourOverride.m = map[string]string{}
	return prev
}

// colourSpan writes text wrapped in the markup for class, or plainly when the
// class is disabled.
func colourSpan(b *str.Builder, class, text string) {
	if c := syntaxColour(class); c != "" {
		b.WriteString(sparkle(c) + text + sparkle("[#-]"))
	} else {
		b.WriteString(text)
	}
}

// shellCmdCache memoises PATH lookups made by the highlighter so per-keystroke
// rendering stays cheap. Keyed on the exact word; true = found on PATH.
var shellCmdCache sync.Map

// isShellCommand reports whether word resolves to an executable on PATH. Note
// that even with -S (coprocess shell disabled) commands still run through the
// parent-process fallback, so the PATH check is always meaningful.
func isShellCommand(word string) bool {
	if v, ok := shellCmdCache.Load(word); ok {
		return v.(bool)
	}
	_, err := exec.LookPath(word)
	found := err == nil
	shellCmdCache.Store(word, found)
	return found
}

// shellCmdMarkup renders a bare word in shell context: on PATH → the
// shellcommand class, otherwise the error class.
func shellCmdMarkup(b *str.Builder, word string) {
	if isShellCommand(word) {
		colourSpan(b, "shellcommand", word)
	} else {
		colourSpan(b, "error", word)
	}
}

// isZaKnownWord reports whether word is already known za state: a declared
// global or a REPL macro name.
func isZaKnownWord(word string) bool {
	if _, ok := gvget(word); ok {
		return true
	}
	_, ok := macroMap.Load(word)
	return ok
}

// shellishPosition reports whether the byte after the word at index j
// (jumping over whitespace) marks a za-language position rather than a shell
// command: an assignment target ('='), a call ('(') or an index ('[').
func shellishPosition(input string, j int) bool {
	n := len(input)
	k := j
	for k < n && (input[k] == ' ' || input[k] == '\t') {
		k++
	}
	if k >= n {
		return false
	}
	switch input[k] {
	case '=', '(', '[':
		return true
	}
	return false
}

// colourSyntax returns an ANSI-coloured rendering of a Za source line for live
// syntax highlighting in the REPL editor. It is a deliberately small, tolerant
// scanner: the user is frequently mid-token, so it never fails on partial
// input and never runs the real parser.
//
// Colours come from syntaxColour() classes and are configurable via the
// syntax_colours() stdlib call. Beyond lexical classes (keywords, functions,
// strings, numbers, comments, variables) it also marks SHELL command positions,
// gated to stay useful in a language REPL:
//   - the first word of the line is a command position; when it resolves to a
//     real executable on PATH the line is treated as shell/mixed and words
//     after `|`, `;`, `&`, `&&`, `||` become further command positions
//     (the separators themselves get the operator class);
//   - a command-position word that is neither za syntax nor on PATH gets the
//     error class;
//   - `${...}` regions are always treated as shell command substitutions;
//   - unknowns are not flagged when they sit in za-ish positions (assignment
//     targets, or followed by '(' or '[').
func colourSyntax(input string) string {

	// sparkle() relies on the ANSI palette replacer built by setupAnsiPalette()
	// (normally called from main()); build it lazily so the highlighter is
	// safe in any context, including tests.
	if fairyReplacer == nil {
		setupAnsiPalette()
	}

	keywordSet := func(w string) bool {
		low := str.ToLower(w)
		for _, k := range completions {
			if str.ToLower(k) == low {
				return true
			}
		}
		return false
	}
	funcSet := func(w string) bool {
		_, ok := slhelp[w]
		if !ok {
			_, ok = slhelp[str.ToLower(w)]
		}
		return ok
	}

	var b str.Builder
	i := 0
	n := len(input)

	shellConfirmed := false // a command position resolved to a real shell command
	expectCmd := true       // the next word is at a command position

	for i < n {
		ch := input[i]

		// comments run to the end of the line
		if ch == '#' {
			j := str.IndexByte(input[i:], '\n')
			if j == -1 {
				colourSpan(&b, "comment", input[i:])
				break
			}
			colourSpan(&b, "comment", input[i:i+j])
			i += j
			continue
		}

		// string literals: "..." `...` '...'
		if ch == '"' || ch == '`' || ch == '\'' {
			j := i + 1
			for j < n && input[j] != ch {
				j++
			}
			if j < n {
				j++ // include the closing quote
			} else {
				j = n
			}
			colourSpan(&b, "string", input[i:j])
			i = j
			expectCmd = false
			continue
		}

		// numbers (hex/binary/octal/floats all covered by the alphanumeric
		// scan; the digit start anchor keeps identifiers out of this branch)
		if ch >= '0' && ch <= '9' {
			j := i
			for j < n {
				c := input[j]
				if (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F') ||
					c == 'x' || c == 'X' || c == 'o' || c == 'O' || c == 'b' || c == 'B' || c == '.' || c == '_' {
					j++
				} else {
					break
				}
			}
			colourSpan(&b, "number", input[i:j])
			i = j
			expectCmd = false
			continue
		}

		// ${...} shell command substitution
		if ch == '$' && i+1 < n && input[i+1] == '{' {
			colourSpan(&b, "operator", "${")
			close := str.IndexByte(input[i+2:], '}')
			hasClose := close != -1
			inner := input[i+2:]
			after := ""
			if hasClose {
				inner = input[i+2 : i+2+close]
				after = input[i+2+close:]
			}
			// colour only the first word as the shell command and preserve
			// the inner text (including any leading spaces) exactly, in the
			// original order: the render must never drop or shift characters
			lead := 0
			for lead < len(inner) && (inner[lead] == ' ' || inner[lead] == '\t') {
				lead++
			}
			rest := inner[lead:]
			sp := 0
			for sp < len(rest) && rest[sp] != ' ' && rest[sp] != '\t' {
				sp++
			}
			if lead > 0 {
				b.WriteString(inner[:lead])
			}
			if sp > 0 {
				shellCmdMarkup(&b, rest[:sp])
			}
			b.WriteString(rest[sp:])
			if hasClose {
				colourSpan(&b, "operator", after[:1]) // the '}'
				b.WriteString(after[1:])
				i = i + 2 + close + 1
			} else {
				i = n
			}
			expectCmd = false
			continue
		}

		// identifiers, keywords, function names, variables, globals and
		// shell command positions
		if ch == '_' || (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || ch == '$' || ch == '@' {
			j := i + 1
			for j < n {
				c := input[j]
				if c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') {
					j++
				} else {
					break
				}
			}
			w := input[i:j]
			atCmd := expectCmd
			expectCmd = false

			if ch == '$' || ch == '@' {
				colourSpan(&b, "variable", w)
			} else if keywordSet(w) {
				colourSpan(&b, "keyword", w)
			} else if funcSet(w) {
				colourSpan(&b, "function", w)
			} else if atCmd {
				// command position: za-first, then PATH, then error
				if shellishPosition(input, j) || isZaKnownWord(w) {
					colourSpan(&b, "variable", w)
				} else if isShellCommand(w) {
					colourSpan(&b, "shellcommand", w)
					shellConfirmed = true
				} else {
					colourSpan(&b, "error", w)
				}
			} else {
				b.WriteString(w)
			}
			i = j
			continue
		}

		// operator / separator handling once a real shell command anchors the
		// line as shell/mixed
		if shellConfirmed {
			switch ch {
			case '|':
				if i+1 < n && input[i+1] == '|' {
					colourSpan(&b, "operator", "||")
					i += 2
				} else {
					colourSpan(&b, "operator", "|")
					i++
				}
				expectCmd = true
				continue
			case '&':
				if i+1 < n && input[i+1] == '&' {
					colourSpan(&b, "operator", "&&")
					i += 2
				} else {
					colourSpan(&b, "operator", "&")
					i++
				}
				expectCmd = true
				continue
			case ';':
				colourSpan(&b, "operator", ";")
				expectCmd = true
				i++
				continue
			case '>', '<':
				// redirection: the following token is an argument, not a command
				if i+1 < n && (input[i+1] == ch || input[i+1] == '&' || input[i+1] == '|') {
					colourSpan(&b, "operator", input[i:i+2])
					i += 2
				} else {
					colourSpan(&b, "operator", string(ch))
					i++
				}
				expectCmd = false
				continue
			}
		}

		b.WriteByte(ch)
		i++
	}

	return b.String()
}

// ---------------------------------------------------------------------------
// TAB completion pager (opt-in via tabpager()): renders the completion pane
// as a column grid with a status row and a clipped description block. The pane
// is a single multi-line string separated by '\n' so the existing draw code
// (clear the rows below the input, print at irow+1) needs no geometry changes,
// and the total line count never exceeds HELP_SIZE.
// ---------------------------------------------------------------------------

const pagerDetailRowsClosed = 3
const pagerDetailRowsOpen = 7

// splitDescriptionLines splits an slhelp action string into render lines.
// Both '\n' and the "[#SOL]" marker start a new line, but the conventional
// "\n[#SOL]" pair (LF to drop a row + ESC[1G back to column 1) is a SINGLE
// line break — the pair is collapsed first so it never yields a phantom
// blank line.
func splitDescriptionLines(s string) []string {
	s = str.ReplaceAll(s, "\n[#SOL]", "\n")
	s = str.ReplaceAll(s, "[#SOL]", "\n")
	return str.Split(s, "\n")
}

// pagerColWidth returns the TAB-pager column width: the longest visible
// candidate name plus padding, capped.
func pagerColWidth(helpList []string) int {
	w := 8
	for _, h := range helpList {
		if L := rlen(h); L > w {
			w = L
		}
	}
	w += 2
	if w > 30 {
		w = 30
	}
	return w
}

// pagerGridCols returns the number of grid columns for the given pane width.
func pagerGridCols(helpList []string) int {
	cols := MW / pagerColWidth(helpList)
	if cols < 1 {
		cols = 1
	}
	return cols
}

// pagerMetrics returns the grid geometry used by pager key handling: column
// count and page size, computed with the collapsed detail budget (good enough
// for navigation; the exact grid is recomputed at render time).
func pagerMetrics(helpList []string) (cols, pageSize int) {
	cols = pagerGridCols(helpList)
	gridRows := 11 - 1 - pagerDetailRowsClosed // HELP_SIZE is local to getInput
	if gridRows < 1 {
		gridRows = 1
	}
	return cols, cols * gridRows
}

// pagerDetailLines builds the raw description lines for the entry at sel.
func pagerDetailLines(helpList []string, helpType []int, fileList map[string]os.FileInfo, sel int) []string {
	switch helpType[sel] {
	case HELP_FUNC:
		name := helpList[sel]
		if str.HasSuffix(name, "(") {
			name = name[:len(name)-1]
		}
		var lines []string
		sig := name
		if h, ok := slhelp[name]; ok {
			sig = name + "(" + h.in + ")"
			lines = append(lines, "[#bold]"+sig+"[#boff]")
			lines = append(lines, splitDescriptionLines(h.action)...)
			return lines
		}
		return []string{"[#bold]" + sig + "[#boff]"}
	case HELP_DIRENT:
		if f, ok := fileList[helpList[sel]]; ok {
			kind := "File"
			if f.IsDir() {
				kind = "Directory"
			}
			return []string{helpList[sel] + " " + kind + sf("  Size:%d Mode:%o Mod:%v", f.Size(), f.Mode(), f.ModTime())}
		}
		return []string{helpList[sel]}
	case HELP_KEYWORD:
		return []string{"[#6]" + helpList[sel] + "[#-]"}
	}
	return nil
}

// clampLine truncates a rendered line to maxCols visible runes.
func clampLine(s string, maxCols int) string {
	r := []rune(s)
	if maxCols > 0 && len(r) > maxCols {
		return string(r[:maxCols])
	}
	return s
}

// buildPagerPane renders the TAB completion pane for tabpager mode. paneRows
// is the number of rows the pane owns (HELP_SIZE in getInput); the pane is
// split between the grid, one status row and the (clipped) description block.
func buildPagerPane(helpList []string, helpType []int, fileList map[string]os.FileInfo, wordUnderCursor []rune, selectedStar int, expanded bool, descScroll int, paneRows int) string {
	n := len(helpList)
	if n == 0 {
		return "[#dim]no matches[#-]"
	}
	sel := selectedStar
	if sel < 0 {
		sel = 0
	}
	if sel >= n {
		sel = n - 1
	}

	detailRows := pagerDetailRowsClosed
	if expanded {
		detailRows = pagerDetailRowsOpen
	}
	if detailRows > paneRows-2 {
		detailRows = paneRows - 2
	}
	gridRows := paneRows - 1 - detailRows
	if gridRows < 1 {
		gridRows = 1
	}

	colWidth := pagerColWidth(helpList)
	cols := MW / colWidth
	if cols < 1 {
		cols = 1
	}
	pageSize := cols * gridRows
	page := sel / pageSize
	pageCount := (n + pageSize - 1) / pageSize
	pageTop := page * pageSize

	pfxR := []rune(str.ToLower(string(wordUnderCursor)))

	lines := make([]string, 0, gridRows+1+detailRows)
	for r := 0; r < gridRows; r++ {
		var b str.Builder
		for c := 0; c < cols; c++ {
			idx := pageTop + r*cols + c
			if idx >= n {
				continue
			}
			full := helpList[idx]
			disp := full
			if helpType[idx] == HELP_DIRENT {
				if f, ok := fileList[full]; ok && f.IsDir() {
					disp = full + "/"
				}
			}
			if rr := []rune(disp); len(rr) > colWidth-1 {
				disp = string(rr[:colWidth-1])
			}
			// underline the portion matching the typed prefix
			lr := []rune(str.ToLower(disp))
			m := 0
			for m < len(lr) && m < len(pfxR) && lr[m] == pfxR[m] {
				m++
			}
			body := disp
			if m > 0 {
				dr := []rune(disp)
				body = sparkle("[#ul]") + string(dr[:m]) + sparkle("[#-]") + string(dr[m:])
			}
			for displayedLen(body) < colWidth-1 {
				body += " "
			}
			if idx == sel {
				b.WriteString("[#b7][#0]" + body + "[#-]")
				continue
			}
			switch helpType[idx] {
			case HELP_FUNC:
				b.WriteString(sparkle("[#5]") + body + sparkle("[#-]"))
			case HELP_KEYWORD:
				b.WriteString(sparkle("[#6]") + body + sparkle("[#-]"))
			case HELP_DIRENT:
				if f, ok := fileList[full]; ok && f.IsDir() {
					b.WriteString(sparkle("[#3]") + body + sparkle("[#-]"))
				} else {
					b.WriteString(sparkle("[#4]") + body + sparkle("[#-]"))
				}
			default:
				b.WriteString(body)
			}
		}
		lines = append(lines, b.String())
	}

	// status row
	plural := "es"
	if n == 1 {
		plural = ""
	}
	status := "[#dim]" + sf("%d match%s", n, plural)
	if pageCount > 1 {
		status += sf(" · page %d/%d", page+1, pageCount)
	}
	if expanded {
		status += " · full description"
		if total := len(pagerDetailLines(helpList, helpType, fileList, sel)); total > detailRows {
			visEnd := descScroll + detailRows
			if visEnd > total {
				visEnd = total
			}
			status += sf(" (%d-%d/%d)", descScroll+1, visEnd, total)
		}
		status += " · [?] grid[#-]"
	} else {
		status += " · [?] full[#-]"
	}
	lines = append(lines, status)

	// clipped description block (the expanded view scrolls through it)
	dlines := pagerDetailLines(helpList, helpType, fileList, sel)
	if max := len(dlines) - detailRows; descScroll > max {
		descScroll = max
	}
	if descScroll < 0 {
		descScroll = 0
	}
	extra := len(dlines) - (descScroll + detailRows)
	detail := make([]string, 0, detailRows)
	for i := 0; i < detailRows; i++ {
		j := descScroll + i
		if j < len(dlines) && dlines[j] != "" {
			detail = append(detail, clampLine(dlines[j], MW-1))
		} else {
			detail = append(detail, "")
		}
	}
	if extra > 0 {
		detail[detailRows-1] = clampLine(dlines[descScroll+detailRows-1], MW-1) + "  " +
			sparkle("[#dim]") + sf("… +%d more [PgDn]", extra) + sparkle("[#-]")
	}
	if descScroll > 0 {
		detail[0] = sparkle("[#dim]") + sf("… [-] %d lines above [PgUp]", descScroll) + sparkle("[#-]") +
			" " + detail[0]
	}
	lines = append(lines, detail...)

	// join the pane rows with an explicit column-1 reset: za runs the tty in
	// raw mode (OPOST off), so a bare '\n' moves down WITHOUT a carriage
	// return, and every subsequent line would start at the previous line's
	// ending column. "[#SOL]" (= ESC[1G) anchors each row back to column 1.
	return "[#SOL]" + str.Join(lines, "\n[#SOL]")
}

// getInput() : get an input string from stdin, in raw mode
func getInput(prompt string, in_defaultString string, pane string, girow int, gicol int, width int, ddopts []string, pcol string, histEnable bool, hintEnable bool, mask string, replEditor bool) (out_s string, eof bool, broken bool, cancelled bool) {

    if runtime.GOOS != "windows" {
        startRaw(0)
        defer endRaw()
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
    HELP_SIZE := 11

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

    at(girow, gicol)

    var srow, scol int // the start of input, including prompt position
    var irow, icol int // current start of input position
    var rowLen int
    baseRow := girow // the prompt's anchor row; may be pushed by popup/cancel handling

    irow = srow
    lastsrow := girow
    defaultAccepted := false
    clearWidth := 0
    if width-gicol >= 0 {
        clearWidth = width - gicol
    }

    // used to avoid repainting an unchanged prompt+input on pure cursor moves
        var prevInput string
        var prevPrompt string
        var prevHelp bool
        var prevIrow int
        var prevRowLen int
        var prevStar int
        // forces a repaint on the iteration after the multi-line editor
        // returns (ESC-abandon or accept), so the prompt block is redrawn
        // after the editor restores (or clears) the screen
        editForceRepaint := false

        // fish-style autosuggestion state (REPL only). The candidate pipeline
        // is shared with the legacy default-string hint: suggestion is the
        // candidate string, suggestionRunes its runes, and defaultAccepted is
        // the shared "hint already accepted" flag (legacy TAB / RIGHT accept).
        useSuggestion := replEditor && autocompleteEnabled
        suggestion := in_defaultString
        suggestionRunes := defaultString
        buildHintFuncnames()
        // the previously drawn ghost tail, cleared before the next repaint
        var prevGhostRow, prevGhostCol, prevGhostLen int

        // TAB-pager state (opt-in): the description block starts collapsed and
        // '?' expands it; opening a big description expands automatically
        pagerDetailExpanded := false
        pagerLastSel := -2
        prevPagerExpanded := false
        // vertical offset into the expanded description (PgUp/PgDn scroll it)
        pagerDescScroll := 0
        prevPagerDescScroll := 0

        // set when an unrecognised ESC-prefixed byte has been seen, so that
        // the rest of its control sequence (which may arrive fragmented over
        // the following reads) is discarded instead of leaking into the input
        dropSeq := false

        // rebuilds the reverse-history-search (Ctrl-R) result list from the
        // current search buffer (fuzzy: substring or subsequence match) and
        // redraws the search prompt + preview line
        rescan := func() {
            searchResults = []int{}
            if len(searchBuffer) > 0 {
                searchTerm := str.ToLower(string(searchBuffer))
                for i := len(hist) - 1; i >= 0; i-- {
                    if fuzzyTermMatch(searchTerm, str.ToLower(hist[i])) {
                        searchResults = append(searchResults, i)
                    }
                }
            }
            currentSearchResult = 0
            clearChars(searchDisplayRow, searchDisplayCol, len(searchPrompt)+len(searchBuffer))
            if remainingWidth := width - searchDisplayCol; remainingWidth > len(searchPrompt)+len(searchBuffer) {
                clearChars(searchDisplayRow, searchDisplayCol+len(searchPrompt)+len(searchBuffer), remainingWidth-(len(searchPrompt)+len(searchBuffer)))
            }
            at(searchDisplayRow, searchDisplayCol)
            pf("[#bold][#6]" + searchPrompt + string(searchBuffer) + "[#-][#4]▋[#-]")
            if len(searchResults) > 0 {
                pf(" -> [#4]" + hist[searchResults[currentSearchResult]] + "[#-]")
            }
        }

    fmt.Print(sparkle(pcol))
    clearChars(girow, gicol, clearWidth)
    for {

        // calc new values for row,col
        srow = baseRow
        scol = gicol
        promptL := displayedLen(sprompt)
        inputL := displayedLen(string(s))
        dispL := promptL + inputL

        hideCursor()

        // Position the whole prompt+input block so it never extends past the
        // bottom of the window. All geometry is decided up front, before any
        // drawing, so the cursor is placed using the same rows the text was
        // drawn on. Note this never lifts a single-line prompt off the last
        // row: after a full-window command output the last row is a blank
        // scroll line, so the prompt must be drawn there (not one row up,
        // which would overwrite the last output line).
        // @note: MH and MW are globals which may change during a SIGWINCH event.
        if srow > MH {
            srow = MH
        }
        height := int(scol+dispL-2) / MW // screen rows spanned by the block (>= 0)
        if srow+height > MH {
            srow = MH - height
        }
        if srow < 1 {
            srow = 1
        }

        // when the completion popup is open it occupies the rows below the
        // input block (HELP_SIZE rows starting below the last input row,
        // irow+rowLen+1); lift the block by exactly the overflow so the popup
        // fits on-screen. Recomputed every iteration off the stable baseRow,
        // so it never drifts. `height` already includes the wrapped input rows.
        if startedContextHelp {
            if over := srow + height + HELP_SIZE - MH; over > 0 {
                srow -= over
                if srow < 1 {
                    srow = 1
                }
            }
        }

        if lastsrow != srow {
            m1 := min(lastsrow, srow)
            m2 := max(lastsrow, srow)
            for r := m2; r > m1; r-- {
                at(r, gicol)
                clearToEOL()
            }
            lastsrow = srow
        }

        irow = srow + (int(scol+promptL-1) / MW)
        icol = ((scol + promptL - 1) % MW) + 1
        rowLen = int(icol+inputL-1) / MW

        if startedContextHelp {
            // this will need more conditions added (e.g. input crossing a slash, etc)
            // to cause it to recalc less often
            max_depth, _ := gvget("context_dir_depth")
            fileList = buildHelpPath(wordUnderCursor, &helpColoured, &helpList, &helpType, max_depth.(int))
        }

        // recompute the fish-style suggestion whenever the input changed;
        // the ghost is only shown with the caret at the end of the input
        if useSuggestion {
            if cand := replHint(string(s)); cand != "" && cpos == len(s) {
                suggestion = cand
                suggestionRunes = []rune(cand)
                defaultAccepted = false
            } else {
                suggestion = ""
                suggestionRunes = nil
            }
        }

        // repaint only when content or layout actually changed; pure cursor
        // moves (arrows, home/end) just reposition the cursor.
        sinput := string(s)
        sdispprompt := string(sprompt)
        // live syntax highlighting: colour the typed input for this paint;
        // all length/cursor math uses displayedLen() so the zero-width ANSI
        // codes inserted here never disturb the layout
        var colored string
        if echo.(bool) {
            colored = colourSyntax(sinput)
        }
        needPaint := sinput != prevInput || sdispprompt != prevPrompt || startedContextHelp != prevHelp ||
            irow != prevIrow || rowLen != prevRowLen || selectedStar != prevStar ||
            pagerDetailExpanded != prevPagerExpanded || pagerDescScroll != prevPagerDescScroll || editForceRepaint
        editForceRepaint = false
        prevInput = sinput
        prevPrompt = sdispprompt
        prevHelp = startedContextHelp
        prevIrow = irow
        prevRowLen = rowLen
        prevStar = selectedStar
        prevPagerExpanded = pagerDetailExpanded
        prevPagerDescScroll = pagerDescScroll

        if needPaint {
            // clear the previously drawn ghost tail before redrawing
            if prevGhostLen > 0 {
                at(prevGhostRow, prevGhostCol)
                clearToEOL()
                prevGhostLen = 0
            }

            // print prompt
            at(srow, scol)
            fmt.Print(sprompt)

            // change input colour
            fmt.Print(sparkle(pcol))

            // show input
            at(irow, icol)
            if echo.(bool) {
                if len(s) > len(suggestionRunes) {
                    fmt.Print(colored)
                } else if str.HasPrefix(suggestion, string(s)) && !defaultAccepted {
                    if useSuggestion && cpos == len(s) {
                        // render the typed prefix normally and the suggestion
                        // tail dimmed/coloured (fish-style ghost)
                        tail := capGhostTail(suggestion[len(string(s)):], icol, inputL)
                        fmt.Print(colored)
                        if tail != "" {
                            fmt.Print(suggestionMarkup(tail))
                            prevGhostRow = irow + rowLen
                            if inputL == 0 {
                                prevGhostCol = icol
                            } else {
                                prevGhostCol = ((icol + inputL - 1) % MW) + 2
                                if prevGhostCol > MW {
                                    prevGhostCol = MW
                                }
                            }
                            prevGhostLen = rlen(tail)
                        }
                    } else {
                        // legacy default hint (or REPL caret not at end):
                        // dimmed+italic whole candidate, unchanged behaviour
                        fmt.Print(sparkle("[#dim][#i1]") + suggestion + sparkle("[#i0][#boff]"))
                    }
                } else {
                    clearChars(irow, icol, len(suggestionRunes))
                    at(irow, icol)
                    fmt.Print(colored)
                }
            } else {
                fmt.Print(str.Repeat(mask, inputL))
            }
            if startedContextHelp {
                // the pane begins below the LAST input row: with a wrapped
                // (multi-row) input the old irow+1 start overlapped the input's
                // continuation lines
                paneStart := irow + rowLen + 1
                for i := paneStart; i <= paneStart+HELP_SIZE-1; i += 1 {
                    at(i, 1)
                    clearToEOL()
                }
                at(paneStart, 1)
                fmt.Print(sparkle(helpstring))
            }
        }

        // move cursor to the position of cpos within the input region. the
        // base row is irow (where the input starts), not srow, so wrapped
        // prompts (irow > srow) still land the cursor on the right line.
        cposCursAtCol := ((icol + cpos - 1) % MW) + 1
        cposRowLen := int(icol+cpos-1) / MW
        at(irow+cposRowLen, cposCursAtCol)

        showCursor()

        // get key stroke
        c, timeout, pasted, pbuf := getch(0)

        if c == nil || timeout {
            continue
        }

        if pasted {

            // strip ansi codes from the paste buffer
            pbuf = Strip(pbuf)

            if replEditor {
                // vte paste marks line breaks with a single CR.
                // A paste that carries more than one line is routed to the
                // multi-line editor instead of silently discarding everything
                // after the first line. A single trailing terminator (common
                // when pasting one line) is dropped and stays in this editor.
                trimmed := str.TrimRight(pbuf, "\r\n")
                if str.Contains(trimmed, "\r") || str.Contains(trimmed, "\n") {
                    // normalise line endings first (VTE sends a single CR
                    // between lines); cleanPasteInput would otherwise strip
                    // the CRs and merge every line into one.
                    trimmed = normalizePasteLines(trimmed)
                    trimmed, _ = cleanPasteInput(trimmed)
                    combined := string(append(append([]rune(s[:cpos]), []rune(trimmed)...), s[cpos:]...))
                    result, reof, rbroken := multilineEditor(combined, -1, MH-5, "", "", "Editor")
                    if !rbroken {
                        s = []rune(result)
                        cpos = len(s)
                        editForceRepaint = true
                    } else if reof {
                        return "", true, false, false
                    } else {
                        // ESC: abandon the editor; repaint the prompt block
                        editForceRepaint = true
                    }
                } else {
                    s = insertWord(s, cpos, trimmed)
                    cpos += rlen(trimmed)
                }
                wordUnderCursor, _ = getWord(s, cpos)
                selectedStar = -1
            } else {
                // single-line editor: no multi-line pasted input
                eol := str.IndexByte(pbuf, '\r')
                alt_eol := str.IndexByte(pbuf, '\n')
                if eol != -1 {
                    pbuf = pbuf[:eol]
                }
                if alt_eol != -1 {
                    pbuf = pbuf[:alt_eol]
                }
                s = insertWord(s, cpos, pbuf)
                cpos += rlen(pbuf)
                wordUnderCursor, _ = getWord(s, cpos)
                selectedStar = -1
            }

        } else {

            for len(c) > 0 {

                switch {

                case bytes.Equal(c, []byte{3}): // ctrl-c
                    removeProcessedKeycode(&c, 1)
                    if replEditor {
                        // REPL line editor: echo the caret at the cursor and
                        // report the cancel to the caller, which abandons the
                        // line/collection and re-prompts on a fresh line.
                        at(irow+cposRowLen, cposCursAtCol)
                        fmt.Print("^C")
                        cancelled = true
                        // clear any completion popup belonging to this line
                        for i := irow + 1; i <= irow+HELP_SIZE; i += 1 {
                            at(i, 1)
                            clearToEOL()
                        }
                        startedContextHelp = false
                        helpstring = ""
                        break
                    }
                    broken = true
                    break
                case bytes.Equal(c, []byte{4}): // ctrl-d
                    removeProcessedKeycode(&c, 1)
                    if replEditor && len(s) > 0 {
                        // REPL: EOF only on an empty line (readline style)
                        break
                    }
                    eof = true
                    break
                case bytes.Equal(c, []byte{26}): // ctrl-z
                    // Send SIGTSTP to the current process group to suspend Za
                    // Platform-specific implementation handles Unix vs Windows
                    removeProcessedKeycode(&c, 1)
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
                            removeProcessedKeycode(&c, 1)
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
                            removeProcessedKeycode(&c, 1)
                            break
                        } else if c[0] >= 32 && c[0] <= 126 { // Printable character
                            // Add character to search buffer
                            searchBuffer = append(searchBuffer, rune(c[0]))
                            removeProcessedKeycode(&c, 1)
                            rescan()

                        } else if c[0] == 127 { // Backspace
                            removeProcessedKeycode(&c, 1)
                            if len(searchBuffer) > 0 {
                                searchBuffer = searchBuffer[:len(searchBuffer)-1]
                                rescan()
                            }
                        } else if c[0] == 21 { // Ctrl+U - clear search buffer
                            removeProcessedKeycode(&c, 1)
                            searchBuffer = []rune{}
                            rescan()
                        }
                    } else if bytes.Equal(c, []byte{0x1B, 0x5B, 0x41}) { // UP arrow in search
                        removeProcessedKeycode(&c, 3)
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
                        removeProcessedKeycode(&c, 3)
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
                    removeProcessedKeycode(&c, 1)
                    if histEnable && !histEmpty {
                        if reverseSearchMode {
                            // Second Ctrl+R press - cancel search
                            reverseSearchMode = false
                            s = orig_s
                            cpos = len(s)
                            showCursor()
                            // Clear the entire line and restore normal input display
                            clearChars(irow, icol, len(orig_s))
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
                            clearChars(irow, icol, len(s))
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
                    removeProcessedKeycode(&c, 1)
                    result, eof, broken := multilineEditor(string(s), -1, MH-5, "", "", "Editor")
                    if !broken {
                        // Replace the input buffer in getInput() with the result from the multiline editor
                        s = []rune(result)
                        cpos = len(s)
                        editForceRepaint = true
                    } else if eof {
                        // If user pressed ctrl-d in multiline, treat as EOF in getInput
                        return "", true, false, false
                    } else {
                        // User pressed ESC in multiline editor: return to input mode with original buffer unchanged
                        editForceRepaint = true
                    }

                case bytes.Equal(c, []byte{13}): // enter
                    removeProcessedKeycode(&c, 1)

                    if startedContextHelp {

                        if len(helpList) > 0 && selectedStar>-1 && helpType[selectedStar]==HELP_DIRENT {
                            f := fileList[helpList[selectedStar]]
                            if f.IsDir() {
                                // update word under cursor but continue in help mode
                                s = replaceWord(s, cpos, helpList[selectedStar]+"/")
                                // cpos = cpos + len(helpList[selectedStar])+1
                                cpos=len(s)
                                wordUnderCursor, _ = getWord(s, cpos)
                                selectedStar=-1
                                break
                            }
                        }

                        contextHelpSelected = true
                        helpstring = ""

                        clearChars(irow, icol, len(s))
                        for i := irow + 1; i <= irow+HELP_SIZE; i += 1 {
                            at(i, 1)
                            clearToEOL()
                        }
                        break
                    }

                    endLine = true

                    if len(s) != 0 {
                        addToHistory(string(s))
                    }

                    break

                case bytes.Equal(c, []byte{32}): // space
                    removeProcessedKeycode(&c, 1)

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
                            for i := irow + 1; i <= irow+HELP_SIZE; i += 1 {
                                at(i, 1)
                                clearToEOL()
                            }
                        }
                        helpstring = ""
                        for i := irow + 1; i <= irow+HELP_SIZE; i += 1 {
                            at(i, 1)
                            clearToEOL()
                        }
                    }

                    // normal space input
                    s = insertAt(s, cpos, rune(' ')) // rune(c[0]))
                    cpos++
                    wordUnderCursor, _ = getWord(s, cpos)

                case bytes.Equal(c, []byte{27, 91, 49, 126}): // home // from showkey -a
                    cpos = 0
                    removeProcessedKeycode(&c, 4)
                    wordUnderCursor, _ = getWord(s, cpos)

                case bytes.Equal(c, []byte{27, 91, 52, 126}): // end // from showkey -a
                    cpos = len(s)
                    removeProcessedKeycode(&c, 4)
                    wordUnderCursor, _ = getWord(s, cpos)

                case bytes.Equal(c, []byte{1}): // ctrl-a
                    cpos = 0
                    removeProcessedKeycode(&c, 1)
                    wordUnderCursor, _ = getWord(s, cpos)

                case bytes.Equal(c, []byte{5}): // ctrl-e
                    cpos = len(s)
                    removeProcessedKeycode(&c, 1)
                    wordUnderCursor, _ = getWord(s, cpos)

                case bytes.Equal(c, []byte{11}): // ctrl-k
                    clearChars(irow, icol, len(s))
                    s = s[:cpos]
                    removeProcessedKeycode(&c, 1)
                    wordUnderCursor, _ = getWord(s, cpos)

                case bytes.Equal(c, []byte{21}): // ctrl-u
                    clearChars(irow, icol, len(s))
                    s = removeAllBefore(s, cpos)
                    cpos = 0
                    removeProcessedKeycode(&c, 1)
                    wordUnderCursor, _ = getWord(s, cpos)

                case bytes.Equal(c, []byte{127}): // backspace
                    removeProcessedKeycode(&c, 1)

                    if startedContextHelp && len(helpstring) == 0 {
                        startedContextHelp = false
                        helpstring = ""
                    }

                    for i := irow + 1; i <= irow+HELP_SIZE; i += 1 {
                        at(i, 1)
                        clearToEOL()
                    }

                    if cpos > 0 {
                        clearChars(irow, icol, len(s))
                        s = removeBefore(s, cpos)
                        cpos--
                        wordUnderCursor, _ = getWord(s, cpos)
                    }

                case bytes.Equal(c, []byte{0x1B, 0x5B, 0x33, 0x7E}): // DEL
                    removeProcessedKeycode(&c, 4)
                    if len(s) == 0 && len(defaultString) != 0 {
                        clearChars(irow, icol, len(defaultString))
                        defaultString = []rune{}
                    }
                    if cpos < len(s) {
                        clearChars(irow, icol, len(s))
                        s = removeBefore(s, cpos+1)
                        wordUnderCursor, _ = getWord(s, cpos)
                    }

                case bytes.Equal(c, []byte{0x1B, 0x5B, 0x44}): // LEFT
                    removeProcessedKeycode(&c, 3)
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
                    removeProcessedKeycode(&c, 3)
                    // add check for RIGHT during auto-completion:
                    if startedContextHelp {
                        if selectedStar < starMax {
                            selectedStar++
                        }
                        break
                    }

                    // fish-style: at the end of the input with a suggestion
                    // shown, RIGHT accepts it
                    if useSuggestion && cpos == len(s) {
                        if cand := replHint(string(s)); cand != "" && str.HasPrefix(cand, string(s)) && cand != string(s) {
                            s = []rune(cand)
                            cpos = len(s)
                            defaultAccepted = true
                            break
                        }
                    }

                    // normal RIGHT:
                    if cpos < len(s) {
                        cpos++
                    }
                    wordUnderCursor, _ = getWord(s, cpos)

                case bytes.Equal(c, []byte{0x06}): // Ctrl-F: accept the suggestion (fish-style)
                    removeProcessedKeycode(&c, 1)
                    if useSuggestion && !startedContextHelp && cpos == len(s) {
                        if cand := replHint(string(s)); cand != "" && str.HasPrefix(cand, string(s)) && cand != string(s) {
                            s = []rune(cand)
                            cpos = len(s)
                            defaultAccepted = true
                            break
                        }
                    }
                    break

                case bytes.Equal(c, []byte{0x1B, 0x5B, 0x41}): // UP
                    removeProcessedKeycode(&c, 3)
                    // the TAB pager owns UP while the pane is open
                    if startedContextHelp && tabpagerEnabled {
                        if len(helpList) > 0 {
                            cols := pagerGridCols(helpList)
                            selectedStar -= cols
                            if selectedStar < 0 {
                                selectedStar = 0
                            }
                        }
                        break
                    }
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
                            clearChars(irow, icol, len(s))
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
                        }
                    }

                case bytes.Equal(c, []byte{0x1B, 0x5B, 0x42}): // DOWN
                    removeProcessedKeycode(&c, 3)
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
                    if startedContextHelp && tabpagerEnabled {
                        if len(helpList) > 0 {
                            cols := pagerGridCols(helpList)
                            selectedStar += cols
                            if m := len(helpList) - 1; selectedStar > m {
                                selectedStar = m
                            }
                        }
                        break
                    }
                    if displayedLenUtf8(s) > MW && cpos < MW {
                        cpos += MW
                        break
                    }

                    if histEnable {
                        if navHist {
                            clearChars(irow, icol, len(s))
                            if curHist < lastHist-1 {
                                curHist++
                                s = []rune(hist[curHist])
                            } else {
                                s = orig_s
                                navHist = false
                            }
                            cpos = len(s)
                            wordUnderCursor, _ = getWord(s, cpos)
                        }
                    }

                case bytes.Equal(c, []byte{0x1B, 0x5B, 0x48}): // HOME
                    cpos = 0
                    removeProcessedKeycode(&c, 3)
                    wordUnderCursor, _ = getWord(s, cpos)
                case bytes.Equal(c, []byte{0x1B, 0x5B, 0x46}): // END
                    cpos = len(s)
                    removeProcessedKeycode(&c, 3)
                    wordUnderCursor, _ = getWord(s, cpos)

                case bytes.Equal(c, []byte{9}): // TAB

                    removeProcessedKeycode(&c, 1)

                    // completion hinting setup
                    if hintEnable {
                        if !startedContextHelp {
                            funcnames = nil

                            // the popup occupies the HELP_SIZE rows below the
                            // input; when it would run off the bottom, scroll
                            // the window up enough to make room on every entry
                            // (not just the first), and move the block anchor so
                            // the loop-top geometry stays aligned.
                            if over := irow + HELP_SIZE - MH; over > 0 {
                                for i := 0; i < over; i++ {
                                    at(MH+1, 1)
                                    fmt.Println()
                                }
                                baseRow -= over
                                if baseRow < 1 {
                                    baseRow = 1
                                }
                            }

                            startedContextHelp = true
                            helpstring = ""
                            pagerDetailExpanded = false
                            pagerDescScroll = 0
                            if tabpagerEnabled {
                                selectedStar = 0 // pager: first match is live
                            } else {
                                selectedStar = -1 // start is off the list so that RIGHT has to be pressed to activate.
                            }

                            //.. add functionnames
                            for k, _ := range slhelp {
                                funcnames = append(funcnames, k)
                            }
                            sort.Strings(funcnames)

                        } else {
                            for i := irow + 1; i <= irow+HELP_SIZE; i++ {
                                at(i, 1)
                                clearToEOL()
                            }
                            helpstring = ""
                            selectedStar = -1 // start is off the list so that RIGHT has to be pressed to activate.
                            contextHelpSelected = false
                            pagerDetailExpanded = false
                            pagerDescScroll = 0
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
                    removeProcessedKeycode(&c, 3)
                    if startedContextHelp && tabpagerEnabled {
                        if len(helpList) > 0 {
                            _, pageSize := pagerMetrics(helpList)
                            selectedStar -= pageSize
                            if selectedStar < 0 {
                                selectedStar = 0
                            }
                        }
                    }

                case bytes.Equal(c, []byte{0x1B, 0x63}): // alt-c
                    removeProcessedKeycode(&c, 2)
                case bytes.Equal(c, []byte{0x1B, 0x76}): // alt-v
                    removeProcessedKeycode(&c, 2)

                case bytes.Equal(c, []byte{0x1B, 0x66}): // alt-f: accept the first word of the suggestion (fish-style)
                    removeProcessedKeycode(&c, 2)
                    if useSuggestion && !startedContextHelp && cpos == len(s) {
                        if cand := replHint(string(s)); cand != "" && str.HasPrefix(cand, string(s)) {
                            if ns, npos, ok := acceptSuggestionWord(s, cand); ok {
                                s = ns
                                cpos = npos
                                defaultAccepted = true
                            }
                        }
                    }

                case bytes.Equal(c, []byte{0x1B, 0x5B, 0x31, 0x3B, 0x33, 0x43}): // alt-right: accept the first word (fish-style)
                    removeProcessedKeycode(&c, 6)
                    acceptedWord := false
                    if useSuggestion && !startedContextHelp && cpos == len(s) {
                        if cand := replHint(string(s)); cand != "" && str.HasPrefix(cand, string(s)) {
                            if ns, npos, ok := acceptSuggestionWord(s, cand); ok {
                                s = ns
                                cpos = npos
                                defaultAccepted = true
                                acceptedWord = true
                            }
                        }
                    }
                    // otherwise move forward one word (fish-style)
                    if !acceptedWord {
                        cpos = wordStep(s, cpos, +1)
                        wordUnderCursor, _ = getWord(s, cpos)
                    }

                case bytes.Equal(c, []byte{0x1B, 0x5B, 0x31, 0x3B, 0x32, 0x44}): // shift-left: one word left
                    removeProcessedKeycode(&c, 6)
                    cpos = wordStep(s, cpos, -1)
                    wordUnderCursor, _ = getWord(s, cpos)

                case bytes.Equal(c, []byte{0x1B, 0x5B, 0x31, 0x3B, 0x32, 0x43}): // shift-right: one word right
                    removeProcessedKeycode(&c, 6)
                    cpos = wordStep(s, cpos, +1)
                    wordUnderCursor, _ = getWord(s, cpos)

                case bytes.Equal(c, []byte{0x1B, 0x5B, 0x31, 0x3B, 0x35, 0x44}): // ctrl-left: one word left
                    removeProcessedKeycode(&c, 6)
                    cpos = wordStep(s, cpos, -1)
                    wordUnderCursor, _ = getWord(s, cpos)

                case bytes.Equal(c, []byte{0x1B, 0x5B, 0x31, 0x3B, 0x35, 0x43}): // ctrl-right: one word right
                    removeProcessedKeycode(&c, 6)
                    cpos = wordStep(s, cpos, +1)
                    wordUnderCursor, _ = getWord(s, cpos)

                case bytes.Equal(c, []byte{0x1B, 0x5B, 0x31, 0x3B, 0x33, 0x44}): // alt-left: one word left
                    removeProcessedKeycode(&c, 6)
                    cpos = wordStep(s, cpos, -1)
                    wordUnderCursor, _ = getWord(s, cpos)

                // ignore list
                case bytes.Equal(c, []byte{0x1B, 0x5B, 0x35, 0x7E}): // pgup
                    removeProcessedKeycode(&c, 4)
                    if startedContextHelp && tabpagerEnabled {
                        if len(helpList) > 0 {
                            if pagerDetailExpanded {
                                // scroll back through the expanded description
                                if pagerDescScroll > 0 {
                                    pagerDescScroll--
                                }
                            } else {
                                _, pageSize := pagerMetrics(helpList)
                                selectedStar -= pageSize
                                if selectedStar < 0 {
                                    selectedStar = 0
                                }
                            }
                        }
                        break
                    }
                case bytes.Equal(c, []byte{0x1B, 0x5B, 0x36, 0x7E}): // pgdown
                    removeProcessedKeycode(&c, 4)
                    if startedContextHelp && tabpagerEnabled {
                        if len(helpList) > 0 {
                            if pagerDetailExpanded {
                                // scroll forward through the expanded description
                                total := len(pagerDetailLines(helpList, helpType, fileList, selectedStar))
                                max := total - pagerDetailRowsOpen
                                if pagerDetailRowsOpen > total {
                                    max = 0
                                }
                                if pagerDescScroll < max {
                                    pagerDescScroll++
                                }
                            } else {
                                _, pageSize := pagerMetrics(helpList)
                                selectedStar += pageSize
                                if m := len(helpList) - 1; selectedStar > m {
                                    selectedStar = m
                                }
                            }
                        }
                        break
                    }
                case bytes.Equal(c, []byte{0x1B, 0x5B, 0x32, 0x7E}): // insert
                    removeProcessedKeycode(&c, 3)

                default:

                    // '?' toggles the TAB-pager's full-description view
                    if len(c) == 1 && c[0] == '?' && startedContextHelp && tabpagerEnabled {
                        pagerDetailExpanded = !pagerDetailExpanded
                        removeProcessedKeycode(&c, 1)
                    } else {

                    // Decode escape sequences the explicit byte cases above do
                    // not enumerate (SS3 arrows, CSI-u arrows and alt-f):
                    // apply the movement / word-accept they describe.
                    if res, rok := decodeEscKey(c); rok {
                        if res.accept {
                            if useSuggestion && !startedContextHelp && cpos == len(s) {
                                if cand := replHint(string(s)); cand != "" && str.HasPrefix(cand, string(s)) {
                                    if ns, npos, aok := acceptSuggestionWord(s, cand); aok {
                                        s = ns
                                        cpos = npos
                                        defaultAccepted = true
                                    }
                                }
                            }
                        } else if res.dir != 0 {
                            if res.word {
                                cpos = wordStep(s, cpos, res.dir)
                            } else if res.dir > 0 {
                                if cpos < len(s) {
                                    cpos++
                                }
                            } else if cpos > 0 {
                                cpos--
                            }
                            wordUnderCursor, _ = getWord(s, cpos)
                        }
                    } else {

                    // Normal input processing (only reached when not in reverse search mode)
                    // multi-byte, like utf8? decode a fresh rune per iteration
                    // so burst input chunks (fast typing) insert every
                    // character, and a line terminator inside the chunk
                    // submits the line just like the Enter key.
                    for len(c) > 0 {
                        if dropSeq {
                            // consume the tail of an unrecognised escape
                            // sequence: the CSI/SS3 introducer '[' / 'O' is
                            // not a final byte, so keep dropping until the
                            // true final byte (0x40-0x7E).
                            b := c[0]
                            removeProcessedKeycode(&c, 1)
                            if b == 0x5B || b == 0x4F {
                                continue
                            }
                            if b >= 0x40 && b <= 0x7E {
                                dropSeq = false
                            }
                            continue
                        }
                        r, sz := utf8.DecodeRune(c)
                        if r != utf8.RuneError {
                            if r == 0x1B { // ESC: start of a control sequence
                                dropSeq = true
                                removeProcessedKeycode(&c, 1)
                                continue
                            }
                            if r == '\r' || r == '\n' {
                                removeProcessedKeycode(&c, sz)
                                if len(s) != 0 {
                                    addToHistory(string(s))
                                }
                                endLine = true
                                break
                            }
                            if sz != 0 {
                                if r > 31 {
                                    s = insertAt(s, cpos, r)
                                    cpos++
                                }
                            }
                            removeProcessedKeycode(&c, sz)
                        } else {
                            // avoid stalling on a malformed byte
                            removeProcessedKeycode(&c, 1)
                        }
                    }
                    } // end decodeEscKey else (normal text insertion)
                    wordUnderCursor, _ = getWord(s, cpos)
                    selectedStar = -1 // also reset the selector position for auto-complete

                    if startedContextHelp {
                        for i := irow + 1; i <= irow+HELP_SIZE; i += 1 {
                            at(i, 1)
                            clearToEOL()
                        }
                    }
                    } // end pager-'?' else (decoded key / normal text)
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

            //.. get file list
            max_depth, _ := gvget("context_dir_depth")
            fileList=buildHelpPath(wordUnderCursor, &helpColoured, &helpList, &helpType, max_depth.(int))

            if tabpagerEnabled {

                // the pager keeps a live selection on the first match and
                // resets the expanded-description view when it moves
                if len(helpList) == 0 {
                    selectedStar = -1
                } else if selectedStar < 0 || selectedStar >= len(helpList) {
                    selectedStar = 0
                }
                starMax = len(helpList) - 1
                if selectedStar != pagerLastSel {
                    pagerLastSel = selectedStar
                    pagerDetailExpanded = false
                    pagerDescScroll = 0
                }
                helpstring = buildPagerPane(helpList, helpType, fileList, wordUnderCursor, selectedStar, pagerDetailExpanded, pagerDescScroll, HELP_SIZE)
            } else {

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
                            helpstring += "\n[#SOL][#bold]" + hla + "(" + slhelp[hla].in + ")[#boff] : [#4]" + slhelp[hla].action + "[#-]"
                        case HELP_DIRENT:
                            f := fileList[helpList[pos]]
                            helpstring += "\n[#SOL]" + helpList[pos]
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
            } // end tabpager-else (classic completion list)

        }

        /*
        if contextHelpSelected && len(helpList) > 0 && selectedStar>-1 && helpType[selectedStar]==HELP_DIRENT {
            f := fileList[helpList[selectedStar]]
            if f.IsDir() {
                // update word under cursor but continue in help mode
                s = insertWord(s, cpos, helpList[selectedStar]+"/")
                cpos = cpos + len(helpList[selectedStar])+1
                contextHelpSelected=false
                startedContextHelp=true
                at(10,10); pf("[#6]BLAH![#-]")
                continue
            }
        }
        */

        if contextHelpSelected {
            if len(helpList) > 0 {
                if selectedStar > -1 {
                    helpList = []string{helpList[selectedStar]}
                }
                if len(helpList) == 1 {
                    // a sole directory completion gets a trailing slash,
                    // matching shell-editor behaviour (fish etc.)
                    hi := 0
                    if selectedStar > -1 {
                        hi = selectedStar
                    }
                    if hi < len(helpType) && helpType[hi] == HELP_DIRENT {
                        if fi, ok := fileList[helpList[0]]; ok && fi.IsDir() && !str.HasSuffix(helpList[0], "/") {
                            helpList[0] = helpList[0] + "/"
                        }
                    }
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

                    for i := irow + 1; i <= irow+HELP_SIZE; i += 1 {
                        at(i, 1)
                        clearToEOL()
                    }
                }
            }
            helpstring = ""
            contextHelpSelected = false
            startedContextHelp = false
        }

        if eof || broken || endLine || cancelled {
            break
        }

    } // input loop

    if len(s) == 0 && len(defaultString) != 0 {
        s = defaultString
    }

    // a cancelled (^C) line is left on screen with its caret marker; skip the
    // normal end-of-line echo so the abandoned text is not redrawn
    if echo.(bool) && !cancelled {
        fmt.Print(sparkle(pcol))
        clearWidth := 0
        if width-scol >= 0 {
            clearWidth = width - scol
        }
        clearChars(srow, scol, clearWidth)
        at(srow, scol)
        fmt.Print(sparkle(sprompt))
        fmt.Print(colourSyntax(string(s)))
    }

    lineWrap = old_wrap

    inputL := displayedLen(string(s))
    promptL := displayedLen(sprompt)
    dispL := promptL + inputL
    rowLen = int(dispL) / MW
    row = baseRow + rowLen + 1
    at(row, 1)
    return string(s), eof, broken, cancelled
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

func escapeQuotes(s string) string {
    s = str.ReplaceAll(s, "`", "\\`")
    s = str.ReplaceAll(s, `"`, `\"`)
    return s
}

/*
func escapeControlChars(s string) string {
    s = str.ReplaceAll(s, "\\", "\\\\") // escape backslashes first
    s = str.ReplaceAll(s, "\n", "\\n")
    s = str.ReplaceAll(s, "\r", "\\r")
    s = str.ReplaceAll(s, "\t", "\\t")
    return s
}
*/

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

// normalizePasteLines converts the common paste line separators to "\n" so
// later processing (cleanPasteInput, editor line splitting) sees real
// newlines regardless of how the terminal delivered them.
func normalizePasteLines(s string) string {
    s = str.ReplaceAll(s, "\r\n", "\n")
    s = str.ReplaceAll(s, "\r", "\n")
    return s
}

func cleanPasteInput(s string) (string, int) {
    out := make([]rune, 0, len(s))
    removed := 0
    for _, r := range s {
        switch {
        case r == '\n' || r == '\t':
            out = append(out, r)
        case r == '\r':
            // a lone CR is a paste line break; keep it as a newline rather
            // than discarding it
            out = append(out, '\n')
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

    // local "clear screen" for the editor: a plain ED2+home. Do not use the
    // package cls() here, which emits \033c (a full terminal RESET) that
    // destroys the primary-buffer snapshot ?1049l would otherwise restore on
    // exit — that is what left the screen and prompt missing after ESC.
    cls := func() {
        pf("\033[2J\033[H")
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
            // normalise the line endings first (VTE sends a single CR between
            // lines) so cleanPasteInput/splitting see real newlines
            pbuf = normalizePasteLines(pbuf)
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
func pf(s string, va ...any) {

    s = sparkle(s)
    var ns string
    if len(va) > 0 {
        ns = sf(s, va...)
    } else {
        ns = s
    }
    sna := Strip(ns)

    if interactive {
        if lineWrap {
            printWithWrap(ns)
        } else {
            fmt.Print(ns)
        }
        chpos := 0
        c := col

        for ; chpos < len(sna); c += 1 {
            if MW > 0 && c%MW == 0 {
                row++
                c = 0
            }
            if sna[chpos] == '\n' {
                row++
                c = 0
            }
            chpos++
        }

        col = c
        return
    }

    if lineWrap {
        printWithWrap(ns)
        return
    }

    fmt.Print(ns)

    // row update:
    atlock.Lock()
    chpos := 0
    c := col
    for ; chpos < len(sna); c += 1 {
        if MW > 0 && c%MW == 0 {
            row++
            c = 0
        }
        if sna[chpos] == '\n' {
            row++
            c = 0
        }
        chpos++
    }
    col = c
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

/*
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
*/

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

func replaceWord(runes []rune, cpos int, word string) []rune {
    runes,cpos=deleteWord(runes,cpos)
    runes=insertWord(runes,cpos,word)
    return runes
}

func deleteWord(runes []rune, cpos int) ([]rune, int) {
    start := 0
    end := len(runes)

    if end < cpos {
        return runes, 0
    }

    // Scan backwards for the start of the word (or dot)
    for p := cpos - 1; p >= 0; p-- {
        if runes[p] == ' ' || runes[p]=='.' || runes[p]=='/' {
            start = p + 1
            break
        }
    }

    // Scan forward for the end of the word (or dot)
    for p := cpos; p < len(runes); p++ {
        if runes[p] == ' ' || runes[p] == '.' || runes[p]=='/' {
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

    c := make(chan BashRead, 1)

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
        if closed {
            plog("Shell command timed out: '%s' (duration: %vms, max_tio: %dms)", cmd, dur.Milliseconds(), MAX_TIO)
            result.E = errors.New("Command '" + cmd + "' timed-out after " +
                dur.String() + " (see logs for details)")
        }
    }

    // Drain any pending result before closing to prevent "send on closed channel" panic
    select {
    case <-c:
        // Result was pending, drain it
    default:
        // No result pending, safe to close
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
func Copper(line string, squashErr bool) (result struct {
    Out  string
    Err  string
    Code int
    Okay bool
}) {
    var start time.Time
    if enableMetrics {
        start = time.Now()
    }

    defer func() {
        if !enableMetrics {
            return
        }
        ms := float64(time.Since(start).Milliseconds())
        metrics.GetOrCreateCounter(`za_shell_calls_total`).Inc()
        if result.Code == 0 {
            metrics.GetOrCreateCounter(`za_shell_success_total`).Inc()
        } else {
            metrics.GetOrCreateCounter(`za_shell_errors_total`).Inc()
        }
        metrics.GetOrCreateSummary(`za_shell_duration_ms`).Update(ms)
    }()

    if !permit_shell {
        panic(fmt.Errorf("Shell calls not permitted!"))
    }

    // remove some bad conditions...
    if str.HasSuffix(str.TrimRight(line, " "), "|") {
        result.Out = ""
        result.Err = ""
        result.Code = -1
        result.Okay = false
        return
    }
    if tr(line, DELETE, "| ", "") == "" {
        result.Out = ""
        result.Err = ""
        result.Code = -1
        result.Okay = false
        return
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
            result.Out = ""
            result.Err = "interrupt"
            result.Code = -3
            result.Okay = false
            return
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

    result.Out = string(ns)
    result.Err = errout
    result.Code = errint
    result.Okay = errint == 0
    return
}

func restoreScreen() {
    pf("\033c") // reset screen
    pf("\033[u")
}

func testStart(file string) {
	if test_tap {
		appendToTestReportRaw(test_output_file, "TAP version 13")
		return
	}
	vos, _ := gvget("@os")
	stros := vos.(string)
	test_start := sf("\n[#6][#ul][#bold]Za Test[#-]\n\nTesting : %s on "+stros+"\n", file)
	appendToTestReport(test_output_file, 0, 0, test_start)
}

func testExit() {
	if test_tap {
		return
	}
	test_final := sf("\n[#6]Tests Performed %d -- Tests Failed %d -- Tests Passed %d[#-]\n\n", testsPassed+testsFailed, testsFailed, testsPassed)
	appendToTestReport(test_output_file, 0, 0, test_final)
}
