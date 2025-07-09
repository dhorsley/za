package main

import (
	"context"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"strings"
	str "strings"
	"sync/atomic"
)

// Global variable to hold current error context for library functions
var currentErrorContext *ErrorContext

func startupOptions() {
	shelltype, _ := gvget("@shelltype")
	if shelltype == "bash" {
		Copper("shopt -s expand_aliases", true)
		Copper("set -o pipefail", true)
	}
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

type parent_and_file struct {
	Parent   string
	Depth    int
	DirEntry fs.DirEntry
}

func dirplus(path string, depth int) (flist []parent_and_file) {
	if depth < 0 {
		return
	}
	sep := "/"
	if runtime.GOOS == "windows" {
		sep = "\\"
	}
	files, err := os.ReadDir(path)
	if err != nil {
		return
	}
	for _, v := range files {
		// pf("  on path %v : cf -> %v\n",path,v.Name())
		f, err := os.Stat(path + sep + v.Name())
		if err == nil {
			flist = append(flist, parent_and_file{Parent: path, Depth: depth, DirEntry: v})
			if f.IsDir() {
				flist = append(flist, dirplus(path+sep+v.Name(), depth-1)...)
			}
		}
	}
	// pf("dp->returning list of %+v\n",flist)
	return flist
}

func dir(filter string) []dirent {

	cdir, _ := gvget("@cwd")
	f, err := os.Open(cdir.(string))
	if err != nil {
		return []dirent{}
	}
	files, err := f.Readdir(-1)
	f.Close()
	if err != nil {
		return []dirent{}
	}

	re := regexp.MustCompile(filter)

	var dl []dirent
	for _, file := range files {
		// if match, _ := regexp.MatchString(filter, file.Name()); !match { continue }
		if !re.MatchString(file.Name()) {
			continue
		}
		var fs dirent
		fs.name = file.Name()
		fs.size = file.Size()
		fs.mode = int(file.Mode())
		fs.mtime = file.ModTime().Unix()
		fs.is_dir = file.IsDir()
		dl = append(dl, fs)
	}
	return dl

}

// does file exist?
func fexists(fp string) bool {
	f, err := os.Stat(fp)
	if err == nil {
		return f.Mode().IsRegular()
	}
	return false
}

func getReportFunctionName(ifs uint32, full bool) string {
	nl, _ := numlookup.lmget(ifs)
	if !full && str.IndexByte(nl, '@') > -1 {
		nl = nl[:str.IndexByte(nl, '@')]
	}
	return nl
}

func showCallChain(base string) {

	// show chain
	evalChainTotal := 0
	pf("[#CTE][#5]")
	calllock.RLock()
	for k, v := range errorChain {
		if k == 0 {
			continue
		}
		if v.registrant == ciEval {
			evalChainTotal++
		}
		if evalChainTotal > 5 {
			pf("-> ABORTED EVALUATION CHAIN (>5) ")
			break
		}
		v.name = getReportFunctionName(v.loc, false)
		// pf("-> %s (%d) (%s) ",v.name,v.line,lookupChainName(v.registrant))
		pf("-> %s(#%d) ", v.name, v.line)
	}
	calllock.RUnlock()
	pf("-> [#6]" + base + "[#-]\n[#CTE]")

}

func lookupChainName(n uint8) string {
	//  ciTrap ciCall ciEval ciMod ciAsyn ciRepl ciRhsb ciLnet ciMain ciErr
	return [10]string{"0-Trap Handler", "1-Call", "2-Evaluator",
		"3-Module Definition", "4-Async Handler", "5-Interactive Mode",
		"6-UDF Builder", "7-Net Library", "8-Main Function", "9-Error Handling"}[n]
}

// extractUnknownWordFromError parses error messages to find the unknown word
func extractUnknownWordFromError(errorMsg string) string {
	// Pattern 1: "Unknown statement 'word'" or similar
	if str.Contains(errorMsg, "Unknown statement") {
		start := str.Index(errorMsg, "'")
		if start != -1 {
			end := str.Index(errorMsg[start+1:], "'")
			if end != -1 {
				return errorMsg[start+1 : start+1+end]
			}
		}
	}

	// Pattern 2: "'word' is uninitialised." - potential keyword typos
	if str.Contains(errorMsg, "is uninitialised") {
		start := str.Index(errorMsg, "'")
		if start != -1 {
			end := str.Index(errorMsg[start+1:], "'")
			if end != -1 {
				return errorMsg[start+1 : start+1+end]
			}
		}
	}

	// Add more patterns as needed for different error types
	return ""
}

// Global flag to prevent recursion in suggestion system
var inSuggestionProcessing = false

func suggestFunction(unknownWord string) string {
	if len(unknownWord) < 4 {
		return "" // Skip short user inputs
	}

	// Safety check to prevent recursion in suggestion processing
	if inSuggestionProcessing {
		return "" // Don't suggest while already processing suggestions
	}
	inSuggestionProcessing = true
	defer func() { inSuggestionProcessing = false }()

	bestMatch := ""
	minDistance := 3 // Maximum useful distance

	// Check stdlib functions first
	for funcName := range slhelp {
		distance := calculateLevenshteinDistance(
			str.ToLower(unknownWord),
			str.ToLower(funcName))

		if distance <= 2 && distance < minDistance {
			minDistance = distance
			bestMatch = funcName
		}
	}

	// Check user-defined functions from numlookup
	// These are stored as "namespace::function_name", we want just the function name

	numlookup.m.Range(func(key, value interface{}) bool {
		// Check if the value is a usable string type
		fullName, ok := value.(string)
		if !ok {
			return true // continue iteration if not a string
		}

		// Skip if it doesn't contain namespace separator
		if !str.Contains(fullName, "::") {
			return true // continue iteration
		}

		// Extract function name part (after "::")
		parts := str.Split(fullName, "::")
		if len(parts) < 2 {
			return true // continue iteration
		}

		funcName := parts[len(parts)-1] // Get the last part (function name)

		// Skip if it's a generated function (contains "@")
		if str.Contains(funcName, "@") {
			return true // continue iteration
		}

		distance := calculateLevenshteinDistance(
			str.ToLower(unknownWord),
			str.ToLower(funcName))

		if distance <= 2 && distance < minDistance {
			minDistance = distance
			bestMatch = funcName
		}

		return true // continue iteration
	})

	if bestMatch != "" {
		return fmt.Sprintf(" [#6]Did you mean '%s()'?[#-]", bestMatch)
	}

	return ""
}

func suggestKeyword(unknownWord string) string {
	if len(unknownWord) < 4 {
		return "" // Skip short user inputs
	}

	// First try keywords
	bestMatch := ""
	minDistance := 3 // Maximum useful distance

	for _, keyword := range completions {
		distance := calculateLevenshteinDistance(
			str.ToLower(unknownWord),
			str.ToLower(keyword))

		if distance <= 2 && distance < minDistance {
			minDistance = distance
			bestMatch = keyword
		}
	}

	if bestMatch != "" {
		return fmt.Sprintf(" [#6]Did you mean '%s'?[#-]", str.ToLower(bestMatch))
	}

	// If no keyword match found, try functions
	return "" // suggestFunction(unknownWord)
}

func suggestVariable(unknownWord string, parser *leparser) string {
	if len(unknownWord) < 3 {
		return "" // Allow shorter names for variables than functions
	}

	// Safety check to prevent recursion in suggestion processing
	if inSuggestionProcessing {
		return ""
	}
	inSuggestionProcessing = true
	defer func() { inSuggestionProcessing = false }()

	var localMatches []string
	var globalMatches []string
	minDistance := 3

	// Check local variables
	if parser.ident != nil {
		for _, v := range *parser.ident {
			if v.declared && v.IName != "" {
				distance := calculateLevenshteinDistance(
					str.ToLower(unknownWord),
					str.ToLower(v.IName))

				if distance <= 2 && distance < minDistance {
					minDistance = distance
					localMatches = []string{v.IName}
				} else if distance <= 2 && distance == minDistance {
					localMatches = append(localMatches, v.IName)
				}
			}
		}
	}

	// Check global variables
	if mident != nil {
		for _, v := range mident {
			if v.declared && v.IName != "" {
				distance := calculateLevenshteinDistance(
					str.ToLower(unknownWord),
					str.ToLower(v.IName))

				if distance <= 2 && distance < minDistance {
					minDistance = distance
					globalMatches = []string{v.IName}
					// Don't reset localMatches - we want both if same distance
				} else if distance <= 2 && distance == minDistance {
					globalMatches = append(globalMatches, v.IName)
				}
			}
		}
	}

	return formatVariableSuggestions(localMatches, globalMatches)
}

func formatVariableSuggestions(localMatches, globalMatches []string) string {
	totalLocal := len(localMatches)
	totalGlobal := len(globalMatches)

	if totalLocal == 0 && totalGlobal == 0 {
		return ""
	}

	var suggestions []string

	// Add local suggestions (prioritized)
	if totalLocal > 0 {
		if totalLocal == 1 {
			suggestions = append(suggestions, fmt.Sprintf("'%s' (local)", localMatches[0]))
		} else if totalLocal == 2 {
			suggestions = append(suggestions, fmt.Sprintf("'%s' or '%s' (local)", localMatches[0], localMatches[1]))
		} else {
			suggestions = append(suggestions, fmt.Sprintf("'%s', '%s', or %d other local variables", localMatches[0], localMatches[1], totalLocal-2))
		}
	}

	// Add global suggestions
	if totalGlobal > 0 {
		if totalGlobal == 1 {
			suggestions = append(suggestions, fmt.Sprintf("'@%s' (global)", globalMatches[0]))
		} else if totalGlobal == 2 {
			suggestions = append(suggestions, fmt.Sprintf("'@%s' or '@%s' (global)", globalMatches[0], globalMatches[1]))
		} else {
			suggestions = append(suggestions, fmt.Sprintf("'@%s', '@%s', or %d other global variables", globalMatches[0], globalMatches[1], totalGlobal-2))
		}
	}

	// Combine suggestions
	if len(suggestions) == 1 {
		return fmt.Sprintf(" [#6]Did you mean %s?[#-]", suggestions[0])
	} else {
		return fmt.Sprintf(" [#6]Did you mean %s or %s?[#-]", suggestions[0], suggestions[1])
	}
}

func (parser *leparser) report(line int16, s string) {

	// Log error to file if error logging is enabled
	if errorLoggingEnabled {
		logError(line, s, parser)
		// Continue with console output if not in quiet mode
	}

	// Simple error ID tracking to allow re-entry from error handler itself
	//currentErrorID := fmt.Sprintf("%p-%d", parser, line) // Unique ID based on parser + line

	/*
	   // Check for attempted re-entry to error handler
	   if enhancedErrorsEnabled && globalErrorContext.InErrorHandler {
	       if globalErrorContext.CurrentErrorID != currentErrorID {
	           return
	       } else {
	           // Allow re-entry from same error handler (recursive call)
	           globalErrorContext.InErrorHandler = false // Temporarily allow processing
	       }
	   }
	*/

	/*
	   // Check if enhanced error handling is enabled and we're not already in an error handler
	   if enhancedErrorsEnabled && !globalErrorContext.InErrorHandler {
	       globalErrorContext.CurrentErrorID = currentErrorID
	       globalErrorContext.InErrorHandler = true

	       // Create a synthetic error and use enhanced error handling
	       err := errors.New(strings.TrimSpace(s))
	       currentCallArgs := make(map[string]any)
	       currentFunctionName := "main"

	       // Use enhanced error handler
	       showEnhancedErrorWithCallArgs(parser, line, err, parser.fs, currentCallArgs, currentFunctionName)
	       return
	   }
	*/

	// Original report() logic for standard error handling
	baseId := parser.fs
	if parser.fs == 2 {
		baseId = 1
	} else {
		funcName := getReportFunctionName(parser.fs, false)
		baseId, _ = fnlookup.lmget(funcName)
	}
	if execMode {
		// pf("\nexecMode : switched fs from %d to %d\n", baseId, execFs)
		baseId = execFs
	}
	baseName, _ := numlookup.lmget(baseId) //      -> name of base func

	var line_content string
	if len(functionspaces[baseId]) > 0 {
		if baseId != 0 {
			line_content = basecode[baseId][parser.pc].Original
		} else {
			line_content = "Interactive Mode"
		}
	}

	moduleName := "main"
	for _, fun := range funcmap {
		if fun.fs == baseId && fun.name == baseName {
			moduleName = fun.module
			break
		}
	}

	var submsg string
	if interactive {
		submsg = "[#7]Error (interactive) : "
	} else {
		submsg = sf("[#7]Error in %+v/%s (line #%d) : ", moduleName, baseName, line+1)
	}

	var msg string
	if !permit_exitquiet {
		msg = sparkle("[#bred]\n[#CTE]"+submsg) +
			line_content + "\n" +
			sparkle("[##][#-][#CTE]") +
			sparkle(sf("%s\n", s)) +
			sparkle("[#CTE]")
	} else {
		msg = sparkle(sf("%s\n", s)) + sparkle("[#CTE]")
	}

	fmt.Print(msg)

	msgna := Strip(msg)
	if interactive {
		chpos := 0
		c := col
		for ; chpos < len(msgna); c++ {
			if c%MW == 0 {
				row++
				c = 0
			}
			if msgna[chpos] == '\n' {
				row++
				c = 0
			}
			chpos++
		}
	}

	if !interactive && !permit_exitquiet {
		showCallChain(baseName)
		pf("\n[#CTE]")
	}

}

func appendToTestReport(test_output_file string, ifs uint32, pos int16, s string) {

	s = sparkle(s) + "\n"

	f, err := os.OpenFile(test_output_file, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0640)
	if err != nil {
		log.Fatal(err)
	}
	if _, err := f.Write([]byte(s)); err != nil {
		log.Fatal(err)
	}
	if err := f.Close(); err != nil {
		log.Fatal(err)
	}

}

// I'm so lazy... snippet below for calculating byte size of interface
// DmitriyVTitov @ https://github.com/DmitriyVTitov/size/blob/master/size.go

func Of(v any) int {
	cache := make(map[uintptr]bool) // cache with every visited Pointer for recursion detection
	return sizeOf(reflect.Indirect(reflect.ValueOf(v)), cache)
}

func sizeOf(v reflect.Value, cache map[uintptr]bool) int {

	switch v.Kind() {

	case reflect.Array:
		fallthrough
	case reflect.Slice:
		// return 0 if this node has been visited already (infinite recursion)
		if v.Kind() != reflect.Array && cache[v.Pointer()] {
			return 0
		}
		if v.Kind() != reflect.Array {
			cache[v.Pointer()] = true
		}
		sum := 0
		for i := 0; i < v.Len(); i++ {
			s := sizeOf(v.Index(i), cache)
			if s < 0 {
				return -1
			}
			sum += s
		}
		return sum + int(v.Type().Size())

	case reflect.Struct:
		sum := 0
		for i, n := 0, v.NumField(); i < n; i++ {
			s := sizeOf(v.Field(i), cache)
			if s < 0 {
				return -1
			}
			sum += s
		}
		return sum

	case reflect.String:
		return len(v.String()) + int(v.Type().Size())

	case reflect.Ptr:
		// return Ptr size if this node has been visited already (infinite recursion)
		if cache[v.Pointer()] {
			return int(v.Type().Size())
		}
		cache[v.Pointer()] = true
		if v.IsNil() {
			return int(reflect.New(v.Type()).Type().Size())
		}
		s := sizeOf(reflect.Indirect(v), cache)
		if s < 0 {
			return -1
		}
		return s + int(v.Type().Size())

	case reflect.Bool,
		reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Int,
		reflect.Chan,
		reflect.Float32, reflect.Float64, reflect.Complex64, reflect.Complex128:
		return int(v.Type().Size())

	case reflect.Map:
		// return 0 if this node has been visited already (infinite recursion)
		if cache[v.Pointer()] {
			return 0
		}
		cache[v.Pointer()] = true
		sum := 0
		keys := v.MapKeys()
		for i := range keys {
			val := v.MapIndex(keys[i])
			// calculate size of key and value separately
			sv := sizeOf(val, cache)
			if sv < 0 {
				return -1
			}
			sum += sv
			sk := sizeOf(keys[i], cache)
			if sk < 0 {
				return -1
			}
			sum += sk
		}
		return sum + int(v.Type().Size())

	case reflect.Interface:
		return sizeOf(v.Elem(), cache) + int(v.Type().Size())
	}

	return -1
}

func version() {
	v, _ := gvget("@ct_info")
	add := ""
	if v.(string) != "glibc" {
		add = v.(string) + ", "
	}
	la, _ := gvget("@language")
	ve, _ := gvget("@version")
	cd, _ := gvget("@creation_date")
	pf("[#6][#bold]%s version %s[#boff][#-]\n", la, ve)
	pf("[#1]Last build:[#-] %s%s\n", add, cd)
}

func help_commands(ns string) {
	commands(ns)
}

func help_colour(ns string) {

	colourpage := `
Some of the codes are demonstrated below. They can be activated by placing the
code inside [# and ] in output strings.

bdefault    / bd   Return background to default colour.
[#fgray][#b0]bblack[#-]   / b0   Set background colour to black.
[#fgray][#b1]bblue[#-]    / b1   Set background colour to blue.
[#fgray][#b2]bred[#-]     / b2   Set background colour to red.
[#fgray][#b3]bmagenta[#-] / b3   Set background colour to magenta.
[#fgray][#b4]bgreen[#-]   / b4   Set background colour to green.
[#fgray][#b5]bcyan[#-]    / b5   Set background colour to cyan.
[#fgray][#b6]byellow[#-]  / b6   Set background colour to yellow.
[#fgray][#b7]bwhite[#-]   / b7   Set background colour to white.
bbgray          Set background colour to bright gray.
bgray           Set background colour to gray.
bbred           Set background colour to bright red.
bbgreen         Set background colour to bright green.
bbyellow        Set background colour to bright yellow.
bbblue          Set background colour to bright blue.
bbmagenta       Set background colour to bright magenta.
bbcyan          Set background colour to bright cyan.

fdefault   / fd   Return the foreground colour to the default.
fblack[#-]    / 0   Set foreground colour to black.
fblue           Set foreground colour to blue.
[#1]fbblue[#-]    / 1   Set foreground colour to bright blue.
fred            Set foreground colour to red.
[#2]fbred[#-]     / 2   Set foreground colour to bright red.
fmagenta        Set foreground colour to magenta.
[#3]fbmagenta[#-] / 3   Set foreground colour to bright magenta.
fgreen          Set foreground colour to green.
[#4]fbgreen[#-]   / 4   Set foreground colour to bright green.
fcyan           Set foreground colour to cyan.
[#5]fbcyan[#-]    / 5   Set foreground colour to bright cyan.
fyellow         Set foreground colour to yellow.
[#6]fbyellow[#-]  / 6   Set foreground colour to bright yellow.
fgray           Set foreground colour to gray.
[#7]fwhite[#-]    / 7   Set foreground colour to white.

-               Return foreground colour to the default.
#               Return background colour to the default.
CTE             Clear to end of line.
default         Turn off all currently raised codes.
[#bold]bold[#-]            Enable bold text.
[#dim]dim[#-]             Enable low-lighting of text.
[#i1]i1[#i0]              Enable italicised text.
i0              Disable italicised text.
[#ul]ul | underline[#-]  Enable underlined text.
[#blink]blink[#-]           Enable flashing text (where supported.)
[#invert]invert[#-]          Enable reverse video text. (where supported.)
hidden          Enable hidden text. (where supported.)
[#crossed]crossed[#-]         Enable single strike-through text. (where supported.)
[#framed]framed[#-]          Enable framed text. (where supported.)
`

	gpf(ns, colourpage)
}

func help_ops(ns string) {

	opspage := `
[#1][#bold]Supported Operators[#boff][#-]

[#1]Prefix Operators[#-]
[#4]--n[#-]         pre-decrement               [#4]++n[#-]         pre-increment
[#4]sqr n[#-]       square (n*n)                [#4]sqrt n[#-]      square root
[#4]-n[#-]          unary negative              [#4]+n[#-]          unary positive
[#4]!b[#-]          boolean negation   ( or not b )

[#4]$uc s[#-]       upper case string s         [#4]$lc s[#-]       lower case string s
[#4]$lt s[#-]       left trim leading whitespace from string s [\t\ \n\r]
[#4]$rt s[#-]       right trim trailing whitespace from string s
[#4]$st s[#-]       trim whitespace from both sides of string s
[#4]$pa s[#-]       absolute path from string s  [#4]$pp s[#-]       parent path of string s
[#4]$pb s[#-]       base file name from string s [#4]$pn s[#-]       base file name without extension
[#4]$pe s[#-]       extension only from string s
[#4]$in f[#-]       read file 'f' in as string literal
[#4]b ? t : f[#-]   if expression b is true then t else f
[#4]| s[#-]         return successful command output (of s) as a string

[#1]Infix Operators[#-]
[#4]a - b[#-]       subtraction                 [#4]a + b[#-]       addition
[#4]a * b[#-]       numeric multiplication      [#4]str_a * b[#-]   string repetition
[#4]a / b[#-]       division                    [#4]a % b[#-]       modulo
[#4]a ** b[#-]      power

[#4]a -= b[#-]      subtractive assignment      [#4]a += b[#-]      additive assignment
[#4]a *= b[#-]      multiplicative assignment   [#4]a /= b[#-]      divisive assignment
[#4]a %= b[#-]      modulo assignment

[#4]a || b[#-]      boolean OR  ( or a or b  )  [#4]a && b[#-]      boolean AND ( or a and b )
[#4]a << b[#-]      bitwise left shift          [#4]a >> b[#-]      bitwise right shift
[#4]a | b[#-]       bitwise OR                  [#4]a & b[#-]       bitwise AND
[#4]a ^ b[#-]       bitwise XOR                 

[#4]a ~f b[#-]      array of matches from string a using regex b[#-]
[#4]s.f[#-]         field access                [#4]s .. e[#-]      builds an array of values in the range s to e
[#4]s $out f[#-]    write string 's' to file 'f'
[#4]ns::struct[#-]  apply namespace to struct name
[#4]ns::enum[#-]    apply namespace to enum name

[#4]array|map ?> "bool_expr"[#-]
[#4]array|map -> "expression"[#-]
  Filters (?>) or maps (->) matches of 'bool_expr' (?>) or values of 'expression' (->) against elements in 
  an array/map to return a new array/map. Each # in bool_expr/expression is replaced by each array/map value in turn.

[#1]Comparisons[#-]
[#4]a == b[#-]      equality                    [#4]a != b[#-]      inequality
[#4]a < b[#-]       less than                   [#4]a > b[#-]       greater than
[#4]a <= b[#-]      less than or equal to       [#4]a >= b[#-]      greater than or equal to
[#4]a ~ b[#-]       string a matches regex b    [#4]a ~i b[#-]      string a matches regex b (case insensitive)
[#4]a in b[#-]      array b contains value a    [#4]a is <type>[#-] expression a has underlying type of:
                                                           bool|int|uint|float|bigi|bigf|number|string|map|array|nil

[#1]Postfix Operators[#-]
[#4]n--[#-]         post-decrement (local scope only, command not expression)
[#4]n++[#-]         post-increment (local scope only, command not expression)
`
	gpf(ns, opspage)
}

// cli help
func help(ns string, hargs string) {

	helppage := `
[#1]za [-v] [-h] [-i] [-b] [-m] [-c] [-C] [-Q] [-S] [-W] [-P] [-d] [-a] [-D]  \
    [-s [#i1]path[#i0]] [-V [#i1]varname[#i0]]                                      \
    [-t] [-O [#i1]tval[#i0]] [-N [#i1]name_filter[#i0]]                             \
    [-G [#i1]group_filter[#i0]]  [-o [#i1]output_file[#i0]]                         \
    [-r] [-F "[#i1]sep[#i0]"] [-e [#i1]program_string[#i0]]                         \
    [-T [#i1]time-out[#i0]] [-U [#i1]sep[#i0]] [[-f] [#i1]input_file[#i0]][#-]

    [#4]-v[#-] : Version
    [#4]-h[#-] : Help
    [#4]-f[#-] : Process script [#i1]input_file[#i0]
    [#4]-e[#-] : Provide source code in a string for interpretation. Stdin becomes available for data input
    [#4]-S[#-] : Disable the co-process shell
    [#4]-s[#-] : Provide an alternative path for the co-process shell
    [#4]-i[#-] : Interactive mode (default if no script provided)
    [#4]-b[#-] : Bypass startup script
    [#4]-c[#-] : Ignore colour code macros at startup
    [#4]-C[#-] : Enable colour code macros at startup
    [#4]-r[#-] : Wraps a -e argument in a loop iterating standard input. Each line is automatically split into fields
    [#4]-F[#-] : Provides a field separator character for -r
    [#4]-t[#-] : Test mode
    [#4]-O[#-] : Test override value [#i1]tval[#i0]
    [#4]-o[#-] : Name the test file [#i1]output_file[#i0]
    [#4]-G[#-] : Test group filter [#i1]group_filter[#i0]
    [#4]-N[#-] : Test name filter [#i1]name_filter[#i0]
    [#4]-a[#-] : Enable assertions. default is false, unless -t specified.
    [#4]-T[#-] : Sets the [#i1]time-out[#i0] duration, in milliseconds, for calls to the co-process shell
    [#4]-W[#-] : Emit errors when addition contains strings mixed with other types
    [#4]-V[#-] : find all references to a variable
    [#4]-m[#-] : Mark co-process command progress
    [#4]-U[#-] : Specify system command separator byte (default 30)
    [#4]-D[#-] : Enable line debug output
    [#4]-d[#-] : Enable full debugger
    [#4]-P[#-] : Enable function profiling output
    [#4]-Q[#-] : Show shell command options
    

`
	gpf(ns, helppage)

}

// interactive mode help
func ihelp(ns string, hargs string) {

	switch len(hargs) {
	case 0:

		helppage := `
[#4]help command    [#-]: available statements
[#4]help op         [#-]: show operator info
[#4]help colour     [#-]: show colour codes
[#4]help <string>   [#-]: show specific statement/function info
[#4]funcs()         [#-]: all functions
[#4]funcs(<string>) [#-]: finds matching categories or functions
`
		gpf(ns, helppage)

	default:

		foundCommand := false
		foundFunction := false

		cmd := str.ToLower(hargs)
		var cmdMatchList []string
		funcMatchList := ""

		switch cmd {

		case "command":
			fallthrough
		case "commands":
			commands(ns)

		case "op":
			fallthrough
		case "ops":
			help_ops(ns)

		case "colour":
			fallthrough
		case "colours":
			help_colour(ns)

		default:

			// check for keyword first:
			re, err := regexp.Compile(`(?m)^{1}\[#[0-9]\]` + cmd + `.*?$`)

			if err == nil {
				cmdMatchList = re.FindAllString(str.ToLower(cmdpage), -1)
				if len(cmdMatchList) > 0 {
					foundCommand = true
				}
			}

			// check for stdlib if not a keyword.
			if _, exists := slhelp[cmd]; exists {
				lhs := slhelp[cmd].out
				colour := "2"
				if slhelp[cmd].out != "" {
					lhs += " = "
					colour = "3"
				}
				params := slhelp[cmd].in
				funcMatchList += sf(sparkle("[#"+colour+"]%s%s(%s)[#-]\n"), lhs, cmd, params)
				funcMatchList += sparkle(sf("[#4]%s[#-]", slhelp[cmd].action))
				foundFunction = true
			} else {
				// pf("(no match):%s\n",cmd)
			}

			if foundFunction || foundCommand {
				if foundCommand {
					remspace := regexp.MustCompile(`[ ]+`)
					for _, nextCmd := range cmdMatchList {
						nextCmd = sparkle(str.TrimSpace(remspace.ReplaceAllString(nextCmd, " ")))
						pf("keyword  : %v\n", nextCmd)
					}
				}
				if foundFunction {
					pf("function : %v\n", funcMatchList)
				}
			}

		}

	}
}

/* bit verbose with these ones listed too:
[#5]SHOWDEF[#-]                                         - list function definitions.
[#2]LOG [#i1]expression[#i0][#-]                                  - local echo plus pre-named destination log file.
[#2]LOGGING [WEB] OFF|ON [#i1]name[#i0][#-]                        - disable or enable logging and specify the log file name.
[#2]LOGGING ACCESSFILE|TESTFILE [#i1]filename[#i0][#-]                    - option to squash console echo of LOG messages.
[#2]LOGGING QUIET | LOUD[#-]                            - option to squash console echo of LOG messages.
[#6]REQUIRE [#i1]feature[#i0] [ [#i1]num[#i0] ][#-]                         - assert feature availability and optional version level, or exit.
[#7]SHOWSTRUCT[#-]                                      - display structure definitions.
[#7]WITH [#i1]var[#i0] AS [#i1]name[#i0][#-]                                - starts a WITH construct.
[#7]ENDWITH[#-]                                         - ends a WITH construct.
[#7]NOP[#-]                                             - no operation - dummy command.
[#7]VERSION[#-]                                         - show Za version.
[#7]HELP[#-]                                            - this page.

*/

var cmdpage string = `
Available commands:
[#5]DEFINE [#i1]name[#i0] ([#i1]arg1,...,argN[#i0])[#-]                     - create a function.
[#5]RETURN [#i1]retval[#i0][#-]                                   - return from function, with value.
[#5]END[#-]                                             - end a function definition.
[#5]ASYNC [#i1]handle_map f(...)[#i0] [[#i1]handle_id[#i0]][#-]             - run a function asynchronously.
[#4]ON [#i1]condition[#i0] DO [#i1]command[#i0][#-]                         - perform a single command if condition evaluates to true.
[#4]IF [#i1]condition[#i0][#-] ... [#4]ELSE[#-] ... [#4]ENDIF[#-]                 - conditional execution.
[#4]WHILE [#i1]condition[#i0][#-]                                 - start while...end loop block.
[#4]ENDWHILE[#-]                                        - end of while...end loop block.
[#4]FOR [#i1]var[#i0] = [#i1]start[#i0] TO [#i1]end[#i0] [ STEP [#i1]step[#i0] ][#-]            - start FOR loop block. (integer iteration only)
[#4]FOREACH [#i1]var[#i0] IN [#i1]var[#i0] | [#i1]fn(expr)[#i0] | [#i1]"literal"[#i0][#-]       - iterate over variable content lines.
[#4]ENDFOR[#-]                                          - terminate FOR execution block.
[#4]CASE [#i1]expr[#i0][#-]                                       - switch-like structure.
[#4]IS | HAS | CONTAINS [#i1]expr[#-][#i0]                        - when [#i1]expr[#i0] matches value, expression or regex.
[#4]OR[#-]                                              - default case.
[#4]ENDCASE[#-]                                         - terminates the CASE block.
[#4]BREAK [ expr ][#-]                                  - exit a loop or CASE clause immediately.
[#4]CONTINUE[#-]                                        - proceed to next loop iteration immediately.
[#4]EXIT [#i1]code[#i0] [,[#i1]error_string[#i0] ][#-]                      - exit script with status code.
[#2]PRINT[LN] [#i1]expression [ , expression ][#i0][#-]           - local echo. (PRINTLN adds a trailing newline character.)
[#2]CLS [ [#i1]pane_id[#i0] ][#-]                                 - clear console screen/pane.
[#2]AT [#i1]row,column[#i0] [ , [#i1]expression[#i0] ][#-]                  - move cursor to [#i1]row,column[#i0]. Optionally display result of [#i1]expression[#i0].
[#2]PANE DEFINE [#i1]name,row,col,h,w[,title[,border]][#i0][#-]   - Define a new coordinate pane.
[#2]PANE SELECT [#i1]name[#i0][#-]                                - Select a defined pane as active.
[#2]PANE OFF[#-]                                        - Disable panes.
[#6]INPUT [#i1]id[#i0] [#i1](PARAM|OPTARG)[#i0] [#i1]position[#i0] [ IS [#i1]hint[#i0] ][#-]    - set variable [#i1]id[#i0] from argument.
[#6]INPUT [#i1]id[#i0] ENV [#i1]env_name[#i0][#-]                           - set variable [#i1]id[#i0] from environmental variable.         
[#6]PROMPT [#i1]var prompt[#i0] [ [#i1]validator[#i0] ][#-]                 - set [#i1]var[#i0] from stdin. loops until [#i1]validator[#i0] satisfied.
[#3]MODULE [#i1]modname[#i0] [ AS [#i1]alias[#i0] ][#-]                     - reads in state from a module file.
[#3]TEST [#i1]name[#i0] GROUP [#i1]gname[#i0] [ ASSERT FAIL|CONTINUE ][#-]  - Define a test
[#3]ENDTEST[#-]                                         - End a test definition
[#3]ASSERT [#i1]condition[#i0][#-]                                - Confirm condition is true, or exit.
[#3]DOC [ [#i1]function_name[#i0] ] [#i1]comment[#i0][#-]                   - Create an exportable comment, for test mode.
[#7]VAR[#-] [#i1]var type[#i0]                                    - declare an optional type or dimension an array.
[#7]ENUM[#-] [#i1]name[#i0] ( member[=val][,...,memberN[=val]] )  - declare an enumeration.
[#7]PAUSE[#-] [#i1]timer_ms[#i0]                                  - delay [#i1]timer_ms[#i0] milliseconds.
[#7]STRUCT[#-] [#i1]name[#i0]                                     - begin structure definition.
[#7]ENDSTRUCT[#-]                                       - end structure definition.
[#7]USE [-|+|^|POP|PUSH] [[#i1]name[#i0]][#-]                     - namespace chain rule configuration.
[#7]|[#-] [#i1]command[#i0]                                       - execute shell command.
[#i1]name[#i0][#i1](params)[#i0]                                    - call a function, with parameters <params>
[#i1]var[#i0] = [#i1]value[#i0]                                     - assign to variable.
[#i1]var[#i0] =| [#i1]expression[#i0]                               - store result of a local shell command to variable.
# comment                                       - comment to end of line.
`

func commands(ns string) {
	gpf(ns, cmdpage)
}

func showEnhancedExpectArgsError(parser *leparser, line int16, enhancedErr *EnhancedExpectArgsError, ifs uint32) {
	showEnhancedExpectArgsErrorWithCallArgs(parser, line, enhancedErr, ifs, nil, "")
}

func showEnhancedExpectArgsErrorWithCallArgs(parser *leparser, line int16, enhancedErr *EnhancedExpectArgsError, ifs uint32, currentCallArgs map[string]any, currentFunctionName string) {
	// Free emergency memory reserve immediately
	if enhancedErrorsEnabled && emergencyMemoryReserve != nil {
		*emergencyMemoryReserve = nil
		emergencyMemoryReserve = nil
		runtime.GC()
	}

	// Populate global error context for library functions
	populateErrorContext(parser, line, enhancedErr.OriginalError.Error(), ifs, currentCallArgs, currentFunctionName)

	// First show the standard error (maintain compatibility)
	parser.report(line, sf("\n%v\n", enhancedErr.OriginalError))

	// Then show enhanced context
	pf("\n[#6]=== Enhanced Error Context ===[#-]\n")

	// For stdlib functions, use the original Args array to preserve order
	if len(enhancedErr.Args) > 0 {
		showCallChainContextWithOrderedArgs(enhancedErr.Args, enhancedErr.FunctionName)
	} else {
		// Show call chain context with current call arguments (fallback)
		showCallChainContextWithCurrentCall(currentCallArgs, currentFunctionName)
	}

	// Show source context
	showSourceContext(parser, line, ifs)

	// Show function information
	showFunctionInfo(enhancedErr.FunctionName)

	// Show variable state
	showVariableState(parser, ifs)

	pf("[#CTE]")

	// Reset CLI cursor and handle debug mode (same as original error handler)
	if debugMode {
		panic(enhancedErr.OriginalError)
	}
	setEcho(true)

	// Check if we're in a deep recursive call chain - if so, exit immediately
	// to prevent corrupted state propagation
	evalChainCount := len(errorChain)
	if evalChainCount > 3 {
		os.Exit(ERR_EVAL)
	}

	// Clear error context when exiting error handler
	clearErrorContext()
}

func showEnhancedErrorWithCallArgs(parser *leparser, line int16, err error, ifs uint32, currentCallArgs map[string]any, currentFunctionName string) {
	// Free emergency memory reserve immediately
	if enhancedErrorsEnabled && emergencyMemoryReserve != nil {
		*emergencyMemoryReserve = nil
		emergencyMemoryReserve = nil
		runtime.GC()
	}

	// Add typo suggestions to error message if enabled (before setting InErrorHandler)
	errorMessage := err.Error()
	if enhancedErrorsEnabled {
		unknownWord := extractUnknownWordFromError(errorMessage)
		if suggestion := suggestKeyword(unknownWord); suggestion != "" {
			errorMessage = errorMessage + suggestion
		} else if suggestion := suggestFunction(unknownWord); suggestion != "" {
			errorMessage = errorMessage + suggestion
		} else if suggestion := suggestVariable(unknownWord, parser); suggestion != "" {
			errorMessage = errorMessage + suggestion
		}
	}

	// Populate global error context for library functions
	populateErrorContext(parser, line, errorMessage, ifs, currentCallArgs, currentFunctionName)

	// First show the standard error (maintain compatibility)
	parser.report(line, sf("\n%v\n", errorMessage))

	// Then show enhanced context
	pf("\n[#6]=== Enhanced Error Context ===[#-]\n")

	// Show call chain context with current call arguments
	evalChainCount := showCallChainContextWithCurrentCall(currentCallArgs, currentFunctionName)

	// Show source context
	showSourceContext(parser, line, ifs)

	// Show variable state
	showVariableState(parser, ifs)

	pf("[#CTE]")

	// Reset CLI cursor and handle debug mode (same as original error handler)
	if debugMode {
		panic(err)
	}
	setEcho(true)

	// Check if we're in a deep recursive call chain - if so, exit immediately
	// to prevent corrupted state propagation (this overrides permit_error_exit)
	if evalChainCount > 3 {
		os.Exit(ERR_EVAL)
	}

	// Clear error context when exiting error handler
	clearErrorContext()
}

func showSourceContext(parser *leparser, line int16, ifs uint32) {
	// Get base function space for source code (same logic as report())
	baseId := parser.fs
	if parser.fs == 2 {
		baseId = 1
	} else {
		funcName := getReportFunctionName(parser.fs, false)
		baseId, _ = fnlookup.lmget(funcName)
	}

	// Get filename from fileMap using parser.fs (the current function space)
	filename := ""
	if fileMapValue, exists := fileMap.Load(parser.fs); exists {
		storedPath := fileMapValue.(string)

		// Convert to absolute path if it's relative
		if filepath.IsAbs(storedPath) {
			filename = storedPath
		} else {
			// Get current working directory and make absolute path
			if cwd, err := os.Getwd(); err == nil {
				filename = filepath.Join(cwd, storedPath)
			} else {
				// Fallback to stored path if we can't get cwd
				filename = storedPath
			}
		}
	}

	// Show header with filename if available
	if filename != "" {
		pf("[#7]Source Context (%s):[#-]\n", filename)
	} else {
		pf("[#7]Source Context:[#-]\n")
	}

	// Show source lines using existing basecode[] and functionspaces[]
	if len(functionspaces[baseId]) > 0 && baseId != 0 {
		// Use parser.pc to index into the statement arrays, but get actual source line numbers from SourceLine
		startStmt := max(0, int(parser.pc)-2)
		endStmt := min(len(basecode[baseId])-1, int(parser.pc)+2)

		for i := startStmt; i <= endStmt; i++ {
			marker := "  "
			if i == int(parser.pc) {
				marker = "→ "
			}
			// Get the actual source line number from the phrase
			actualLineNum := functionspaces[baseId][i].SourceLine + 1 // Convert 0-based to 1-based
			pf("  %s[#b5][#7]%5d[#-] | %s\n", marker, actualLineNum, basecode[baseId][i].Original)
		}
	}
}

func showFunctionInfo(functionName string) {
	pf("\n[#7]Function Information:[#-]\n")

	// Look up function info using existing slhelp[]
	if helpInfo, exists := slhelp[functionName]; exists {
		pf("  %s(%s) -> %s\n", functionName, helpInfo.in, helpInfo.out)
		pf("  Description: %s\n", helpInfo.action)
	} else {
		pf("  %s (user-defined function)\n", functionName)
	}
}

func showVariableState(parser *leparser, ifs uint32) {
	pf("\n[#7]Variable State:[#-]\n")

	// Get the source line that failed
	var sourceLine string
	if len(functionspaces[ifs]) > 0 && ifs != 0 && int(parser.pc) < len(basecode[ifs]) {
		sourceLine = basecode[ifs][parser.pc].Original
	}

	// In interactive mode, only show variables referenced in the failing expression
	if interactive && sourceLine != "" {
		referencedVars := extractVariableNames(sourceLine)
		shown := 0

		for _, varName := range referencedVars {
			if parser.ident != nil {
				for i := 0; i < len(*parser.ident); i++ {
					v := (*parser.ident)[i]
					if v.declared && v.IName == varName {
						valueStr := formatVariableValue(v.IValue)
						pf("  • %s = %s (%T)\n", v.IName, valueStr, v.IValue)
						shown++
						break
					}
				}
			}
		}

		if shown == 0 {
			pf("  (no variables referenced in failing expression)\n")
		}
	} else {
		// Non-interactive mode: show up to 3 declared variables
		if parser.ident != nil {
			shown := 0
			maxVars := 3

			// Show declared variables
			for i := 0; i < len(*parser.ident) && shown < maxVars; i++ {
				v := (*parser.ident)[i]
				if v.declared && v.IName != "" {
					valueStr := formatVariableValue(v.IValue)
					pf("  • %s = %s (%T)\n", v.IName, valueStr, v.IValue)
					shown++
				}
			}

			if shown == 0 {
				pf("  (no declared variables in scope)\n")
			}
		}
	}
}

func formatVariableValue(value any) string {
	if value == nil {
		return "nil"
	}

	valueStr := sf("%v", value)

	// Truncate very long values
	if len(valueStr) > 50 {
		return valueStr[:47] + "..."
	}

	return valueStr
}

// extractVariableNames extracts variable names from a source line using Za's tokens stdlib function
func extractVariableNames(sourceLine string) []string {
	var variables []string
	seen := make(map[string]bool)

	// Call the stdlib tokens function to get parsed tokens
	if tokensFunc, exists := stdlib["tokens"]; exists {
		result, err := tokensFunc("main", 0, nil, sourceLine)
		if err == nil {
			if tokenResult, ok := result.(token_result); ok {
				// Filter for IDENTIFIER token types
				for i, tokenType := range tokenResult.types {
					if tokenType == "IDENTIFIER" && i < len(tokenResult.tokens) {
						varName := tokenResult.tokens[i]
						if !seen[varName] {
							variables = append(variables, varName)
							seen[varName] = true
						}
					}
				}
			}
		}
	}

	return variables
}

// cleanFunctionName cleans manufactured function names while preserving namespace info
// e.g., "main::main_func@5" -> "main_func" (strips main namespace)
// e.g., "mymodule::helper@3" -> "mymodule::helper" (keeps other namespaces)
func cleanFunctionName(manufacturedName string) string {
	// Remove the instance suffix (@number)
	if atPos := strings.LastIndex(manufacturedName, "@"); atPos != -1 {
		manufacturedName = manufacturedName[:atPos]
	}

	// Only remove "main::" prefix, keep other namespaces for clarity
	if strings.HasPrefix(manufacturedName, "main::") {
		return manufacturedName[6:] // Remove "main::" prefix
	}

	return manufacturedName
}

func showCallChainContext() {
	showCallChainContextWithCurrentCall(nil, "")
}

// showCallChainContextWithOrderedArgs displays call chain using ordered argument arrays
// This preserves the original argument order for stdlib functions
func showCallChainContextWithOrderedArgs(args []any, functionName string) {
	pf("\n[#7]Call Chain:[#-]\n")

	// Show current call with ordered arguments
	if len(args) > 0 && functionName != "" {
		cleanCurrentName := cleanFunctionName(functionName)
		pf("  [#3]Current:[#-] [#5]%s[#-]\n", cleanCurrentName)
		pf("    [#7]Arguments:[#-] ")

		// Display arguments in their original order
		for i, argValue := range args {
			if i > 0 {
				pf(", ")
			}
			valueStr := formatVariableValue(argValue)
			pf("[#2]arg%d[#-]=[#4]%s[#-]", i+1, valueStr)
		}
		pf("\n")
	}

	// Show parent calls from error chain (same as before)
	if len(errorChain) == 0 {
		if len(args) == 0 {
			pf("  (no call chain available)\n")
		}
		return
	}

	// Walk through the errorChain to show call context (with curtailment for deep recursion)
	evalChainTotal := 0
	for i, chainInfo := range errorChain {
		// Count evaluation chains and abort if too many (prevent recursion spam)
		if chainInfo.registrant == ciEval {
			evalChainTotal++
		}
		if evalChainTotal > 5 {
			indent := strings.Repeat("  ", i+1)
			pf("%s[#3]...[#-] [#5]ABORTED CALL CHAIN (>5 evaluation levels)[#-]\n", indent)
			break
		}

		calllock.RLock()

		// Get the function space name from the chain info
		caller_str, exists := numlookup.lmget(chainInfo.loc)
		if !exists {
			calllock.RUnlock()
			continue
		}

		// Get the call table entry for this location
		callEntry := calltable[chainInfo.loc]

		calllock.RUnlock()

		// Get module name
		moduleName := "main"
		if callEntry.base < uint32(len(basemodmap)) {
			if mod, modExists := basemodmap[callEntry.base]; modExists {
				moduleName = mod
			}
		}

		// Get filename for this call location - try chainInfo first, then fileMap
		filename := chainInfo.filename
		if filename == "" {
			if fileMapValue, exists := fileMap.Load(chainInfo.loc); exists {
				filename = fileMapValue.(string)
			}
		}

		// Show the call information
		indent := strings.Repeat("  ", i+1)
		cleanCallerName := cleanFunctionName(caller_str)
		pf("%s[#3]%d.[#-] [#5]%s[#-] in [#6]%s[#-]", indent, i+1, cleanCallerName, moduleName)

		// Show filename:line if available
		if filename != "" {
			if chainInfo.line > 0 {
				pf(" ([#7]%s:%d[#-])", filename, chainInfo.line+1) // Convert 0-based to 1-based
			} else {
				pf(" ([#7]%s[#-])", filename)
			}
		} else if chainInfo.line > 0 {
			pf(" (line %d)", chainInfo.line+1) // Convert 0-based to 1-based
		}

		// Show arguments if they were captured in the error chain
		if len(chainInfo.argNames) > 0 && len(chainInfo.argValues) > 0 {
			pf("\n%s    [#7]Arguments:[#-] ", indent)
			argCount := 0

			// The error chain preserves argument order in the arrays, so just iterate through them
			for j, argName := range chainInfo.argNames {
				if j < len(chainInfo.argValues) {
					if argCount > 0 {
						pf(", ")
					}
					valueStr := formatVariableValue(chainInfo.argValues[j])
					pf("[#2]%s[#-]=[#4]%s[#-]", argName, valueStr)
					argCount++
				}
			}
		}

		pf("\n")
	}
}

func showCallChainContextWithCurrentCall(currentCallArgs map[string]any, currentFunctionName string) int {
	pf("\n[#7]Call Chain:[#-]\n")

	// Show current call arguments from errorChain (last entry) if available
	if len(errorChain) > 0 {
		currentCall := errorChain[len(errorChain)-1]
		if len(currentCall.argNames) > 0 && len(currentCall.argValues) > 0 {
			cleanCurrentName := cleanFunctionName(currentCall.name)
			pf("  [#3]Current:[#-] [#5]%s[#-]\n", cleanCurrentName)
			pf("    [#7]Arguments:[#-] ")

			// Show arguments in their original order from argNames/argValues arrays
			for i, argName := range currentCall.argNames {
				if i < len(currentCall.argValues) {
					if i > 0 {
						pf(", ")
					}
					valueStr := formatVariableValue(currentCall.argValues[i])
					pf("[#2]%s[#-]=[#4]%s[#-]", argName, valueStr)
				}
			}
			pf("\n")
		}
	}

	// Show parent calls from error chain
	if len(errorChain) == 0 {
		if len(currentCallArgs) == 0 {
			pf("  (no call chain available)\n")
		}
		return 0
	}

	// Walk through the errorChain to show call context (with curtailment for deep recursion)
	evalChainTotal := 0
	for i, chainInfo := range errorChain {
		// Count evaluation chains and abort if too many (prevent recursion spam)
		if chainInfo.registrant == ciEval {
			evalChainTotal++
		}
		if evalChainTotal > 5 {
			indent := strings.Repeat("  ", i+1)
			pf("%s[#3]...[#-] [#5]ABORTED CALL CHAIN (>5 evaluation levels)[#-]\n", indent)
			break
		}

		calllock.RLock()

		// Get the function space name from the chain info
		caller_str, exists := numlookup.lmget(chainInfo.loc)
		if !exists {
			calllock.RUnlock()
			continue
		}

		// Get the call table entry for this location
		callEntry := calltable[chainInfo.loc]

		calllock.RUnlock()

		// Get module name
		moduleName := "main"
		if callEntry.base < uint32(len(basemodmap)) {
			if mod, modExists := basemodmap[callEntry.base]; modExists {
				moduleName = mod
			}
		}

		// Get filename for this call location - try chainInfo first, then fileMap
		filename := chainInfo.filename
		if filename == "" {
			if fileMapValue, exists := fileMap.Load(chainInfo.loc); exists {
				filename = fileMapValue.(string)
			}
		}

		// Show the call information
		indent := strings.Repeat("  ", i+1)
		cleanCallerName := cleanFunctionName(caller_str)
		pf("%s[#3]%d.[#-] [#5]%s[#-] in [#6]%s[#-]", indent, i+1, cleanCallerName, moduleName)

		// Show filename:line if available
		if filename != "" {
			if chainInfo.line > 0 {
				pf(" ([#7]%s:%d[#-])", filename, chainInfo.line+1) // Convert 0-based to 1-based
			} else {
				pf(" ([#7]%s[#-])", filename)
			}
		} else if chainInfo.line > 0 {
			pf(" (line %d)", chainInfo.line+1) // Convert 0-based to 1-based
		}

		// Show arguments if they were captured in the error chain
		if len(chainInfo.argNames) > 0 && len(chainInfo.argValues) > 0 {
			pf("\n%s    [#7]Arguments:[#-] ", indent)
			argCount := 0

			// The error chain preserves argument order in the arrays, so just iterate through them
			for j, argName := range chainInfo.argNames {
				if j < len(chainInfo.argValues) {
					if argCount > 0 {
						pf(", ")
					}
					valueStr := formatVariableValue(chainInfo.argValues[j])
					pf("[#2]%s[#-]=[#4]%s[#-]", argName, valueStr)
					argCount++
				}
			}
		}

		pf("\n")
	}

	return evalChainTotal
}

// populateErrorContext gathers error information and stores it in globalErrorContext
func populateErrorContext(parser *leparser, line int16, errorMessage string, ifs uint32, currentCallArgs map[string]any, currentFunctionName string) {
	globalErrorContext.Message = errorMessage
	globalErrorContext.SourceLine = line
	globalErrorContext.FunctionName = currentFunctionName
	globalErrorContext.InErrorHandler = true

	// Get module name
	baseId := ifs
	if parser.fs == 2 {
		baseId = 1
	} else {
		funcName := getReportFunctionName(ifs, false)
		baseId, _ = fnlookup.lmget(funcName)
	}
	globalErrorContext.ModuleName = basemodmap[baseId]

	// Collect source lines
	globalErrorContext.SourceLines = []string{}
	if len(functionspaces[baseId]) > 0 && baseId != 0 {
		startLine := max(0, int(parser.pc)-2)
		endLine := min(len(basecode[baseId])-1, int(parser.pc)+2)
		for i := startLine; i <= endLine; i++ {
			globalErrorContext.SourceLines = append(globalErrorContext.SourceLines, basecode[baseId][i].Original)
		}
	}

	// Build call chain
	globalErrorContext.CallChain = []map[string]any{}
	globalErrorContext.CallStack = []string{}

	// Add current call to chain
	if currentFunctionName != "" {
		callInfo := map[string]any{
			"function": currentFunctionName,
			"args":     currentCallArgs,
		}

		// For stdlib functions with enhanced errors, store arguments as ordered array
		if len(currentCallArgs) == 0 && currentErrorContext != nil && currentErrorContext.EnhancedError != nil {
			// Store arguments as a simple ordered array with indexed keys
			orderedArgs := make([]string, 0)
			for i, arg := range currentErrorContext.EnhancedError.Args {
				orderedArgs = append(orderedArgs, sf("arg%d:%#v", i+1, arg))
			}
			callInfo["args"] = orderedArgs
		}

		globalErrorContext.CallChain = append(globalErrorContext.CallChain, callInfo)
		globalErrorContext.CallStack = append(globalErrorContext.CallStack, currentFunctionName)
	}

	// Add error chain with enhanced argument information
	for i := len(errorChain) - 1; i >= 0; i-- {
		chainEntry := errorChain[i]

		// Get function name - prefer the captured name over the registrant lookup
		functionName := chainEntry.name
		if functionName == "" {
			functionName = lookupChainName(chainEntry.registrant)
		}

		globalErrorContext.CallStack = append(globalErrorContext.CallStack, functionName)

		// Build enhanced call info with arguments if available
		callInfo := map[string]any{
			"function": functionName,
			"type":     chainEntry.registrant,
		}

		// Add arguments if they were captured (when enhancedErrorsEnabled was true)
		if len(chainEntry.argNames) > 0 && len(chainEntry.argValues) > 0 {
			argsMap := make(map[string]any)
			for j, argName := range chainEntry.argNames {
				if j < len(chainEntry.argValues) {
					argsMap[argName] = chainEntry.argValues[j]
				}
			}
			callInfo["args"] = argsMap
		}

		globalErrorContext.CallChain = append(globalErrorContext.CallChain, callInfo)
	}

	// Collect local variables
	globalErrorContext.LocalVars = make(map[string]any)
	if parser.ident != nil {
		for i := 0; i < len(*parser.ident); i++ {
			v := (*parser.ident)[i]
			if v.declared && v.IName != "" {
				globalErrorContext.LocalVars[v.IName] = v.IValue
			}
		}
	}

	// Collect user-defined global variables (filter out system variables starting with @)
	globalErrorContext.GlobalVars = make(map[string]any)
	for i := 0; i < len(gident); i++ {
		v := gident[i]
		if v.declared && v.IName != "" && !str.HasPrefix(v.IName, "@") {
			globalErrorContext.GlobalVars[v.IName] = v.IValue
		}
	}
}

// clearErrorContext resets the error context when exiting error handling
func clearErrorContext() {
	globalErrorContext = ErrorContext{InErrorHandler: false}
}

// callCustomErrorHandler calls the user-defined error handler function
func callCustomErrorHandler(handlerName string, namespace string, evalfs uint32) {
	// Ensure we have a complete handler name with namespace
	if !str.Contains(handlerName, "::") {
		if found := uc_match_func(handlerName); found != "" {
			handlerName = found + "::" + handlerName
		} else {
			handlerName = namespace + "::" + handlerName
		}
	}

	// Remove any argument parentheses for now (simple implementation)
	if brackPos := str.IndexByte(handlerName, '('); brackPos != -1 {
		handlerName = handlerName[:brackPos]
	}

	// Look up the function
	lmv, found := fnlookup.lmget(handlerName)
	if !found {
		// Fallback to standard error display if handler not found
		pf("[#1]Error: Custom error handler '%s' not found[#-]\n", handlerName)
		if currentErrorContext != nil && currentErrorContext.EnhancedError != nil {
			showEnhancedExpectArgsError(currentErrorContext.Parser, int16(currentErrorContext.SourceLocation.Line), currentErrorContext.EnhancedError, currentErrorContext.EvalFS)
		}
		return
	}

	// Populate globalErrorContext with current error information for library functions
	if currentErrorContext != nil {
		globalErrorContext.Message = currentErrorContext.Message
		globalErrorContext.SourceLine = int16(currentErrorContext.SourceLocation.Line)
		globalErrorContext.FunctionName = currentErrorContext.SourceLocation.Function
		globalErrorContext.ModuleName = currentErrorContext.SourceLocation.Module
		globalErrorContext.InErrorHandler = true

		// Populate source lines, call chain, and variables
		populateErrorContext(currentErrorContext.Parser, globalErrorContext.SourceLine,
			globalErrorContext.Message, currentErrorContext.EvalFS, nil, globalErrorContext.FunctionName)
	}

	// Allocate function space for the error handler
	loc, _ := GetNextFnSpace(true, handlerName+"@", call_s{prepared: true, base: lmv, caller: evalfs})

	calllock.Lock()
	basemodmap[lmv] = namespace
	calllock.Unlock()

	// Create a new variable space for the handler
	var handlerIdent = make([]Variable, identInitialSize)

	// Call the error handler (no arguments for now)
	ctx := context.Background()
	// Set the callLine field in the calltable entry before calling the function
	// For error handlers, we use the source line from the error context
	atomic.StoreInt32(&calltable[loc].callLine, int32(globalErrorContext.SourceLine))
	_, _, _, callErr := Call(ctx, MODE_NEW, &handlerIdent, loc, ciTrap, false, nil, "", []string{})

	// Clean up after the call
	calllock.Lock()
	calltable[loc].gcShyness = 0
	calltable[loc].gc = true
	calllock.Unlock()

	// Clear the error context after handling
	clearErrorContext()
	currentErrorContext = nil

	if callErr != nil {
		pf("[#1]Error in custom error handler: %s[#-]\n", callErr)
	}
}
