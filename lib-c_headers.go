package main

import (
    "context"
    "errors"
    "fmt"
    "math"
    "os"
    "path/filepath"
    "regexp"
    "runtime"
    "strconv"
    "strings"
    "sync"
    "time"
    "unicode"
    "unicode/utf8"
    "unsafe"
)

// Global typedef registry
// Structure: libraryAlias → typedefName → baseTypeString
// Example: "png" → "png_structp" → "struct png_struct*"
//          "c" → "size_t" → "unsigned long"
var moduleTypedefs = make(map[string]map[string]string)
var moduleTypedefsLock sync.RWMutex

// moduleFunctionPointerSignatures stores function pointer type signatures
// Structure: libraryAlias → typedefName → CFunctionSignature
// Example: "c" → "qsort_compar_t" → CFunctionSignature for int (*)(const void*, const void*)
var moduleFunctionPointerSignatures = make(map[string]map[string]CFunctionSignature)
var moduleFunctionPointerSignaturesLock sync.RWMutex

// cModuleIdents stores ident arrays for C module macros, keyed by function space id
// These are kept separate from Za's normal ident/bindings system
var cModuleIdentsLock sync.RWMutex
var cModuleIdents = make(map[uint32][]Variable)

// cModuleAliasMap maps C module aliases to their function space IDs
// This is separate from basemodmap which is for Za namespaces
var cModuleAliasMapLock sync.RWMutex
var cModuleAliasMap = make(map[string]uint32)

// macroEvaluating tracks macros currently being evaluated to detect cycles
// alias → macroName → isEvaluating
var macroEvaluating = make(map[string]map[string]bool)
var macroEvaluatingLock = &sync.RWMutex{}

// moduleMacrosOrder tracks the order in which macros were defined
// This is critical for evaluating macros in dependency order
var moduleMacrosOrder = make(map[string][]string) // alias → ordered list of macro names
var moduleMacrosOrderLock = &sync.RWMutex{}

// macroEvalStatus tracks the evaluation status of each macro for help display
// Status values: "evaluated" (success), "skipped" (filtered), "failed" (attempted but failed)
var macroEvalStatus = make(map[string]map[string]string) // alias → macroName → status
var macroEvalStatusLock sync.RWMutex

// Pre-compiled regex patterns used by evaluateConstant() - compiled once at package init
// This eliminates O(n²) regex compilation in evaluateAllMacros() hot loops
var (
    // EXISTING - Regex patterns in evaluateConstant()
    nanPatternRe       = regexp.MustCompile(`\(\s*0\.0+\s*/\s*0\.0+\s*\)`)
    castBeforeSizeofRe = regexp.MustCompile(`\([a-zA-Z_][a-zA-Z0-9_\s]*\)\s*sizeof`)
    sizeofRe           = regexp.MustCompile(`sizeof\s*\(\s*([^)]+)\s*\)`)
    paramListPatternRe = regexp.MustCompile(`^\([A-Za-z_][A-Za-z0-9_]*\s+[A-Za-z_][A-Za-z0-9_]*`)
    simpleIdentifierRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
    cIdentPatternRe    = regexp.MustCompile(`__c_[A-Za-z_][A-Za-z0-9_]*`)
    // Single optimized regex for all C type keywords - eliminates 12 regex compilations per call
    typeKeywordRe = regexp.MustCompile(`\b(char|short|int|long|float|double|void|unsigned|signed|struct|union|enum)\b`)

    // NEW - Regex patterns in helper functions called by evaluateConstant()
    charLiteralRe      = regexp.MustCompile(`([LuU])?'((?:\\.|[^'\\])+)'`)              // convertCharacterLiterals
    stringConcatRe     = regexp.MustCompile(`"([^"]*)"[ \t]+"`)                         // transformStringConcatenation
    intSuffixRe        = regexp.MustCompile(`\b(\d+|0[xX][0-9a-fA-F]+)([uU]?[lL]{0,2}|[lL]{0,2}[uU]?)\b`) // stripCIntegerSuffixes
    floatSuffixRe      = regexp.MustCompile(`\b(\d+(?:\.\d*)?(?:[eE][+-]?\d+)?)[fFlLdD]\b`)               // stripCIntegerSuffixes
    ternaryCondRe      = regexp.MustCompile(`\b(\d+|0[xX][0-9a-fA-F]+)\s*\?`)          // convertTernaryConditions
    boolContextIntRe   = regexp.MustCompile(`(^|\(|\&\&|\|\|)\s*(\d+|0[xX][0-9a-fA-F]+)\s*(\&\&|\|\||[?:]|$)`) // convertCBooleanOps
    notIntRe           = regexp.MustCompile(`!\s*(\d+|0[xX][0-9a-fA-F]+)`)             // convertCBooleanOps
    notVarRe           = regexp.MustCompile(`!\s*([A-Za-z_][A-Za-z0-9_]*)`)            // convertCBooleanOps
    andVarRe           = regexp.MustCompile(`(^|\(|\&\&|\|\|)\s*([A-Za-z_][A-Za-z0-9_]*)\s*(\&\&)`) // convertCBooleanOps
    orVarRe            = regexp.MustCompile(`(^|\(|\&\&|\|\|)\s*([A-Za-z_][A-Za-z0-9_]*)\s*(\|\|)`) // convertCBooleanOps
)

// autoImportErrors tracks errors from AUTO module imports
// Structure: libraryAlias → list of error messages from skipped structs/unions
// Allows Za code to programmatically check for import failures
var autoImportErrors = make(map[string][]string)
var autoImportErrorsLock sync.RWMutex

// currentProgressTracker holds the active progress tracker for the current AUTO import
// This allows functions deep in the call stack to access it without passing it everywhere
var currentProgressTracker *AutoProgressTracker
var currentProgressTrackerLock sync.Mutex

// PreprocessorState tracks conditional compilation state while parsing a header
type PreprocessorState struct {
    definedMacros  map[string]string  // NAME → VALUE from #define
    conditionStack []bool              // Stack: true=include, false=skip
    chainSatisfied []bool              // Stack: true=any condition in if/elif chain was true
    includeDepth   int                 // Current #ifdef nesting level
    visitedHeaders map[string]bool     // Tracks visited headers for cycle detection
    alias          string               // Library alias for context
}

// AutoProgressTracker tracks progress for AUTO import process
type AutoProgressTracker struct {
    startTime     time.Time
    totalWeight   float64   // Always 100.0
    currentWeight float64   // Accumulated progress (0-100)
    currentPhase  string    // Current operation description
    subPhaseInfo  string    // Additional context (e.g., "Pass 3/10")
    enabled       bool      // Whether to show progress
    lastDisplayed float64   // Last displayed percentage
    messages      []string  // Buffered warning/info messages
}

// newAutoProgressTracker creates and initializes a progress tracker
func newAutoProgressTracker() *AutoProgressTracker {
    enabled := os.Getenv("ZA_NO_PROGRESS") == ""
    return &AutoProgressTracker{
        startTime:     time.Now(),
        totalWeight:   100.0,
        currentWeight: 0.0,
        enabled:        enabled,
        lastDisplayed:  -1.0,
    }
}

// update accumulates progress and displays the bar if significant change
func (pt *AutoProgressTracker) update(weight float64, phase string, subPhase string) {
    if !pt.enabled {
        return
    }

    // Accumulate weight
    pt.currentWeight += weight

    // Clamp to 100
    if pt.currentWeight > 100.0 {
        pt.currentWeight = 100.0
    }

    // Update phase info
    if phase != "" {
        pt.currentPhase = phase
    }
    pt.subPhaseInfo = subPhase

    // Display if significant change (>0.5%) or near completion
    percentage := pt.currentWeight
    if math.Abs(percentage-pt.lastDisplayed) > 0.5 || percentage >= 99.9 {
        pt.display()
    }
}

// addMessage buffers a warning or info message to display after progress completes
func (pt *AutoProgressTracker) addMessage(msg string) {
    if !pt.enabled {
        return
    }

    pt.messages = append(pt.messages, msg)
}

// addMessageToCurrentProgress adds a message to the current progress tracker (if any)
// This is used by functions deep in the call stack that don't have access to the tracker
func addMessageToCurrentProgress(msg string) {
    currentProgressTrackerLock.Lock()
    defer currentProgressTrackerLock.Unlock()

    if currentProgressTracker != nil {
        currentProgressTracker.addMessage(msg)
    } else {
        // If no progress tracker is active, just print to stderr
        fmt.Fprintf(os.Stderr, "%s\n", msg)
    }
}

// setCurrentProgressTracker sets the active progress tracker for the current AUTO import
func setCurrentProgressTracker(pt *AutoProgressTracker) {
    currentProgressTrackerLock.Lock()
    defer currentProgressTrackerLock.Unlock()

    currentProgressTracker = pt
}

// display renders the progress bar at current percentage
func (pt *AutoProgressTracker) display() {
    percentage := pt.currentWeight
    if percentage > 100.0 {
        percentage = 100.0
    }

    // Build the progress bar (30 characters)
    barLen := 30
    filled := int(percentage / 100.0 * float64(barLen))
    if filled > barLen {
        filled = barLen
    }

    var bar strings.Builder
    bar.WriteString("[")
    for i := 0; i < barLen; i++ {
        if i < filled && i < barLen-1 {
            bar.WriteString("=")
        } else if i == filled && filled < barLen {
            bar.WriteString(">")
        } else {
            bar.WriteString(" ")
        }
    }
    bar.WriteString("]")

    // Build the info string
    info := ""
    if pt.currentPhase != "" {
        info = pt.currentPhase
        if pt.subPhaseInfo != "" {
            info += " " + pt.subPhaseInfo
        }
    }

    // Print progress bar update using [#SOL] to return to start of line
    if info != "" {
        pf("[#SOL][#6][AUTO] Processing: %s %3.0f%%[#-] [#1]- %s[#CTE]", bar.String(), percentage, info)
    } else {
        pf("[#SOL][#6][AUTO] Processing: %s %3.0f%%[#CTE]", bar.String(), percentage)
    }

    pt.lastDisplayed = percentage
}

// finish displays buffered messages and completion time
func (pt *AutoProgressTracker) finish() {
    if !pt.enabled {
        return
    }

    // Print newline after the progress bar (which was using [#SOL] without \n)
    if pt.lastDisplayed >= 0 {
        pf("\n")
    }

    // Display all buffered warning/info messages
    for _, msg := range pt.messages {
        pln(msg)
    }

    // Calculate elapsed time and print completion message
    elapsed := time.Since(pt.startTime)
    pf("[#4][AUTO] Import completed in %.3fs[#CTE]\n", elapsed.Seconds())
}

// newPreprocessorState creates a new preprocessor state with platform macros
func newPreprocessorState(alias string) *PreprocessorState {
    state := &PreprocessorState{
        definedMacros:  make(map[string]string),
        conditionStack: []bool{true}, // Start with including (top level)
        includeDepth:   0,
        visitedHeaders: make(map[string]bool),
        alias:          alias,
    }

    // Define platform macros based on runtime
    if runtime.GOOS == "linux" || runtime.GOOS == "freebsd" ||
        runtime.GOOS == "openbsd" || runtime.GOOS == "netbsd" {
        state.definedMacros["__linux__"] = "1" // Generic unix-like
        state.definedMacros["__unix__"] = "1"
    }

    if runtime.GOARCH == "amd64" || runtime.GOARCH == "arm64" {
        state.definedMacros["__LP64__"] = "1" // 64-bit platform
    }

    // Always define __GNUC__ for compatibility
    state.definedMacros["__GNUC__"] = "4"       // Claim GCC 4.x
    state.definedMacros["__GNUC_MINOR__"] = "9" // GCC 4.9

    // glibc version macros
    state.definedMacros["__GLIBC__"] = "2"       // glibc major version
    state.definedMacros["__GLIBC_MINOR__"] = "31" // glibc minor version (conservative)

    // Word size and time size macros
    if runtime.GOARCH == "amd64" || runtime.GOARCH == "arm64" {
        state.definedMacros["__WORDSIZE"] = "64"
        state.definedMacros["__TIMESIZE"] = "64"
    } else {
        state.definedMacros["__WORDSIZE"] = "32"
        state.definedMacros["__TIMESIZE"] = "32"
    }
    state.definedMacros["__WORDSIZE_TIME64_COMPAT32"] = "1"

    // Common feature test macros
    state.definedMacros["__USE_MISC"] = "1"  // Misc extensions
    state.definedMacros["__USE_XOPEN"] = "1" // X/Open compliance

    // STDC macros
    state.definedMacros["__STDC__"] = "1"
    state.definedMacros["__STDC_VERSION__"] = "201710" // C17 (no L suffix)
    state.definedMacros["__STDC_HOSTED__"] = "1"

    // NOTE: __cplusplus is intentionally NOT defined (C mode, not C++)
    // NOTE: __GLIBC_USE and __GLIBC_PREREQ are function-like macros that
    // will be handled by being replaced with 0 when undefined

    return state
}

// GetModuleConstants returns a copy of the constants for a given module alias
// Returns nil if the alias doesn't exist or has no constants
func GetModuleConstants(alias string) map[string]any {
    moduleConstantsLock.RLock()
    defer moduleConstantsLock.RUnlock()

    if constants, exists := moduleConstants[alias]; exists {
        // Return a copy to avoid concurrent modification
        result := make(map[string]any, len(constants))
        for k, v := range constants {
            result[k] = v
        }
        return result
    }
    return nil
}

// GetModuleMacros returns a copy of the macros (original source text) for a given module alias
// Returns nil if the alias doesn't exist or has no macros
func GetModuleMacros(alias string) map[string]string {
    moduleMacrosLock.RLock()
    defer moduleMacrosLock.RUnlock()

    if macros, exists := moduleMacros[alias]; exists {
        // Return a copy to avoid concurrent modification
        result := make(map[string]string, len(macros))
        for k, v := range macros {
            result[k] = v
        }
        return result
    }
    return nil
}

// GetMacroEvalStatus returns the evaluation status for a macro
// Status values: "evaluated", "skipped", "failed", "unknown"
func GetMacroEvalStatus(alias, name string) string {
    macroEvalStatusLock.RLock()
    defer macroEvalStatusLock.RUnlock()
    if statuses, exists := macroEvalStatus[alias]; exists {
        if status, exists := statuses[name]; exists {
            return status
        }
    }
    return "unknown"
}

// isActive returns true if we're currently in an active (included) block
func (s *PreprocessorState) isActive() bool {
    // All conditions in stack must be true
    for _, active := range s.conditionStack {
        if !active {
            return false
        }
    }
    return true
}

// isDefined checks if a macro is defined
func (s *PreprocessorState) isDefined(name string) bool {
    _, exists := s.definedMacros[name]
    return exists
}

// pushCondition adds a new conditional level
// For #if/#elif, satisfied indicates if this condition was true
// For #ifdef/#ifndef, use the same value for both
func (s *PreprocessorState) pushCondition(include bool) {
    s.conditionStack = append(s.conditionStack, include)
    s.chainSatisfied = append(s.chainSatisfied, include)
    s.includeDepth++
}

// pushConditionInChain adds a condition that's part of an if/elif/else chain
// If alreadySatisfied is true, this condition is skipped regardless of include value
func (s *PreprocessorState) pushConditionInChain(include bool, alreadySatisfied bool) {
    effectiveInclude := include && !alreadySatisfied
    s.conditionStack = append(s.conditionStack, effectiveInclude)
    s.chainSatisfied = append(s.chainSatisfied, alreadySatisfied || include)
    s.includeDepth++
}

// popCondition removes the top conditional level
func (s *PreprocessorState) popCondition() {
    if len(s.conditionStack) > 0 {
        s.conditionStack = s.conditionStack[:len(s.conditionStack)-1]
        s.chainSatisfied = s.chainSatisfied[:len(s.chainSatisfied)-1]
        s.includeDepth--
    }
}

// replaceConditionInChain replaces the current condition (for #elif)
// Only includes the block if no previous condition in the chain was satisfied
func (s *PreprocessorState) replaceConditionInChain(include bool) {
    if len(s.conditionStack) > 0 && len(s.chainSatisfied) > 0 {
        alreadySatisfied := s.chainSatisfied[len(s.chainSatisfied)-1]
        effectiveInclude := include && !alreadySatisfied
        s.conditionStack[len(s.conditionStack)-1] = effectiveInclude
        // Update chainSatisfied if this condition is true
        if include {
            s.chainSatisfied[len(s.chainSatisfied)-1] = true
        }
    }
}

// toggleCondition flips the current condition (#else handling)
// Only activates if no previous condition in the chain was satisfied
func (s *PreprocessorState) toggleCondition() {
    if len(s.conditionStack) > 0 && len(s.chainSatisfied) > 0 {
        alreadySatisfied := s.chainSatisfied[len(s.chainSatisfied)-1]
        s.conditionStack[len(s.conditionStack)-1] = !alreadySatisfied
        s.chainSatisfied[len(s.chainSatisfied)-1] = true // #else is the final block
    }
}

// getMacrosAsIdent converts definedMacros to ident array for expression evaluation
// This allows macros to be referenced in #if expressions (e.g., #if VERSION > 1)
// Macro names are prefixed with __c_ to avoid Za keyword conflicts
// expandMacroValue recursively expands macro references in a value
// Example: if value is "__WORDSIZE" and __WORDSIZE is "64", returns "64"
func (s *PreprocessorState) expandMacroValue(value string, visited map[string]bool) string {
    // Prevent infinite recursion
    if visited == nil {
        visited = make(map[string]bool)
    }

    // Check if the value is a simple macro reference
    trimmed := strings.TrimSpace(value)
    if s.isDefined(trimmed) && !visited[trimmed] {
        visited[trimmed] = true
        macroVal := s.definedMacros[trimmed]
        return s.expandMacroValue(macroVal, visited)
    }

    return value
}

func (s *PreprocessorState) getMacrosAsIdent() []Variable {
    debugAuto := os.Getenv("ZA_DEBUG_AUTO") != ""
    if debugAuto {
        fmt.Fprintf(os.Stderr, "[AUTO] getMacrosAsIdent START: %d macros\n", len(s.definedMacros))
    }

    ident := make([]Variable, 0, len(s.definedMacros))

    for name, value := range s.definedMacros {
        if debugAuto {
            fmt.Fprintf(os.Stderr, "[AUTO]   Processing macro %s = %q\n", name, value)
        }
        // Expand macro references first
        expandedValue := s.expandMacroValue(value, nil)

        // Try to parse the value as a number and annotate with C type
        var val any = expandedValue
        var ctype string

        // First try parsing as integer literal
        if intVal, err := strconv.ParseInt(expandedValue, 0, 64); err == nil {
            val = int(intVal)
            ctype = "c_int"
        } else if floatVal, err := strconv.ParseFloat(expandedValue, 64); err == nil {
            // Then try float literal
            val = floatVal
            ctype = "c_float"
        } else {
            // Not a number literal - check if it's a numeric value using GetAsInt
            if intVal, ok := GetAsInt(expandedValue); ok {
                val = intVal
                ctype = "c_int"
            } else {
                // Not numeric - it's either a string or an expression
                // For expressions, don't add to ident - they'll be expanded inline
                if strings.ContainsAny(expandedValue, "()&|!<>=+-*/") {
                    // Skip adding expression macros to ident - they'll be expanded inline
                    continue
                }
                ctype = "c_string"
            }
        }

        ident = append(ident, Variable{
            IName:         name,
            IValue:        val,
            Kind_override: ctype,
            declared:      true,
        })
    }

    if debugAuto {
        fmt.Fprintf(os.Stderr, "[AUTO] getMacrosAsIdent: Built ident array with %d variables\n", len(ident))
    }

    return ident
}

// preprocessIfExpression replaces defined(NAME) with 1 or 0 and expands macros
// Handles: defined(NAME), defined NAME (without parens)
func (s *PreprocessorState) preprocessIfExpression(expr string) string {
    debugAuto := os.Getenv("ZA_DEBUG_AUTO") != ""
    if debugAuto {
        fmt.Printf("[AUTO] preprocessIfExpression START: %q\n", expr)
    }

    // Do multiple passes to expand nested macros, but limit to prevent infinite loops
    const maxPasses = 5
    for pass := 0; pass < maxPasses; pass++ {
        if debugAuto {
            fmt.Printf("[AUTO]   Pass %d: %q\n", pass, expr)
        }
        oldExpr := expr
        expr = s.preprocessIfExpressionPass(expr)
        if debugAuto {
            fmt.Printf("[AUTO]   After pass %d: %q\n", pass, expr)
        }
        // If nothing changed, we're done
        if expr == oldExpr {
            if debugAuto {
                fmt.Printf("[AUTO]   No change, stopping\n")
            }
            break
        }
    }

    if debugAuto {
        fmt.Printf("[AUTO] preprocessIfExpression END: %q\n", expr)
    }
    return expr
}

func (s *PreprocessorState) preprocessIfExpressionPass(expr string) string {
    debugAuto := os.Getenv("ZA_DEBUG_AUTO") != ""
    if debugAuto {
        fmt.Printf("[AUTO]     preprocessIfExpressionPass IN: %q\n", expr)
    }

    // Handle defined(NAME)
    re := regexp.MustCompile(`defined\s*\(\s*([A-Za-z_][A-Za-z0-9_]*)\s*\)`)
    expr = re.ReplaceAllStringFunc(expr, func(match string) string {
        // Extract the macro name
        name := re.FindStringSubmatch(match)[1]
        if s.isDefined(name) {
            return "1"
        }
        return "0"
    })

    // Handle defined NAME (without parentheses)
    re2 := regexp.MustCompile(`defined\s+([A-Za-z_][A-Za-z0-9_]*)`)
    expr = re2.ReplaceAllStringFunc(expr, func(match string) string {
        // Extract the macro name
        name := re2.FindStringSubmatch(match)[1]
        if s.isDefined(name) {
            return "1"
        }
        return "0"
    })

    // Handle ALL function-like macros FIRST: __GLIBC_USE(x) → 0, __GNUC_PREREQ(4,1) → 0
    // We don't have a full macro expansion engine, so we replace all function-like
    // macros with 0 (which is safe for conditionals - if they were meant to be true,
    // the header would have defined them differently)
    // Must do this before simple macro replacement to avoid "0(x)" syntax errors
    processedExpr := expr
    offset := 0
    re3 := regexp.MustCompile(`\b([A-Z_][A-Za-z0-9_]*)\s*\(`)

    for {
        loc := re3.FindStringIndex(processedExpr[offset:])
        if loc == nil {
            break
        }

        actualStart := offset + loc[0]
        actualEnd := offset + loc[1]

        // Process ALL macros (defined or undefined)
        // Function-like macros need full expansion which we don't support,
        // so we treat them all as 0 in conditionals
        // Find matching closing paren
        parenCount := 1
        i := actualEnd
        for i < len(processedExpr) && parenCount > 0 {
            if processedExpr[i] == '(' {
                parenCount++
            } else if processedExpr[i] == ')' {
                parenCount--
            }
            i++
        }

        // Replace entire function call with 0
        processedExpr = processedExpr[:actualStart] + "0" + processedExpr[i:]
        offset = actualStart + 1 // Continue from after the "0"
    }
    expr = processedExpr

    // Replace macro names with their values or 0 for undefined
    // If a macro's value is an expression (contains operators/parens), expand it inline
    // Otherwise keep it as a variable reference
    // IMPORTANT: Skip character/string literal prefixes (L, u, U, u8) followed by quotes
    re4 := regexp.MustCompile(`\b([A-Za-z_][A-Za-z0-9_]*)\b`)
    matches := re4.FindAllStringIndex(expr, -1)
    if matches != nil {
        // Build result by processing matches in reverse order to maintain indices
        result := expr
        for i := len(matches) - 1; i >= 0; i-- {
            start := matches[i][0]
            end := matches[i][1]
            match := expr[start:end]

            // Check if this is a character/string literal prefix
            // L'x', L"str", u'x', U'x', u8"str"
            isLiteralPrefix := false
            if end < len(expr) {
                nextChar := expr[end]
                if nextChar == '\'' || nextChar == '"' {
                    // Check if it's a known literal prefix
                    if match == "L" || match == "u" || match == "U" {
                        isLiteralPrefix = true
                    } else if match == "u8" {
                        isLiteralPrefix = true
                    }
                }
            }

            // Don't replace literal prefixes
            if isLiteralPrefix {
                continue
            }

            // Replace with macro value or 0
            replacement := ""
            if s.isDefined(match) {
                // Get the macro value
                value := s.definedMacros[match]
                // If the value looks like an expression (has operators or parens), expand it inline
                // This handles cases like: #define X (A && B)
                // The multi-pass loop will handle nested expansions
                if strings.ContainsAny(value, "()&|!<>=+-*/") {
                    replacement = "(" + value + ")"
                } else {
                    // Otherwise replace with the actual value (for simple integer/string macros)
                    // Empty string means macro defined without value - treat as 1 per C semantics
                    if value == "" {
                        replacement = "1"
                    } else {
                        replacement = value
                    }
                }
            } else {
                // Undefined macro - replace with 0 (C preprocessor semantics)
                replacement = "0"
            }

            result = result[:start] + replacement + result[end:]
        }
        expr = result
    }

    if debugAuto {
        fmt.Printf("[AUTO]     preprocessIfExpressionPass OUT: %q\n", expr)
    }
    return expr
}

// parseModuleHeaders finds and parses header files for a C library
func parseModuleHeaders(libraryPath string, alias string, explicitPaths []string, fs uint32) error {
    debugAuto := os.Getenv("ZA_DEBUG_AUTO") != ""
    if debugAuto {
        fmt.Printf("[AUTO] parseModuleHeaders START: lib=%s, alias=%s, fs=%d\n", libraryPath, alias, fs)
    }

    // Initialize progress tracker and set it as current
    progress := newAutoProgressTracker()
    setCurrentProgressTracker(progress)
    defer func() {
        setCurrentProgressTracker(nil)
        progress.finish()
    }()

    // Check if ident table already allocated for this namespace and reuse if exists
    cModuleIdentsLock.Lock()
    if cModuleIdents[fs] == nil {
        cModuleIdents[fs] = make([]Variable, 0)
    }
    // If it exists, we reuse it - allowing namespace merging
    cModuleIdentsLock.Unlock()

    // Determine header paths for cache key computation
    var headerPaths []string
    if len(explicitPaths) > 0 {
        // Explicit paths provided: MODULE "lib.so" AS name HEADERS "path1.h" "path2.h"
        headerPaths = explicitPaths
    } else {
        // Auto-discover: MODULE "libfoo.so" AS foo HEADERS
        headerPaths = discoverHeaders(libraryPath)
    }

    // CACHE: Compute cache key once and try to load from cache
    var cacheKey FFICacheKey
    var cacheKeyErr error
    cacheKey, cacheKeyErr = computeCacheKey(libraryPath, alias, headerPaths)
    if cacheKeyErr != nil {
        if debugAuto {
            fmt.Printf("[AUTO] Failed to compute cache key: %v, will continue without cache\n", cacheKeyErr)
        }
    } else {
        if cachedData, ok := tryLoadFFICache(cacheKey); ok {
            if err := populateGlobalMapsFromCache(cachedData, alias, fs); err != nil {
                // Cache load failed, continue with normal parsing
                if debugAuto {
                    fmt.Printf("[AUTO] Cache load failed: %v, will parse headers\n", err)
                }
            } else {
                // Cache loaded successfully
                progress.update(100.0, "Loaded from cache", "")
                return nil
            }
        }
    }

    if len(headerPaths) == 0 {
        // Extract library name to show what we searched for
        baseName := filepath.Base(libraryPath)
        name := strings.TrimPrefix(baseName, "lib")
        name = strings.TrimSuffix(name, ".so")
        name = strings.TrimSuffix(name, ".so.6")

        // Build list of paths that were searched
        searched := []string{
            "/usr/include/" + name + ".h",
            "/usr/local/include/" + name + ".h",
            "/usr/include/" + name + "/" + name + ".h",
        }

        // Add architecture-specific path
        if arch := runtime.GOARCH; arch == "amd64" {
            searched = append(searched, "/usr/include/x86_64-linux-gnu/"+name+".h")
        } else if arch == "arm64" {
            searched = append(searched, "/usr/include/aarch64-linux-gnu/"+name+".h")
        }

        return fmt.Errorf("no header files found for module '%s'\n  Searched: %s",
            alias, strings.Join(searched, ", "))
    }

    // Update progress: header discovery
    progress.update(2.0, "Found headers", fmt.Sprintf("(%d files)", len(headerPaths)))

    // Collect processed text from all files for enum/function parsing after macro evaluation
    allProcessedText := make([]string, 0)

    // Per-file processing: allocate 60% across all files
    perFileWeight := 60.0 / float64(len(headerPaths))
    for fileIdx, hpath := range headerPaths {
        fileName := filepath.Base(hpath)
        progress.update(0, "Processing file",
            fmt.Sprintf("(%d/%d) %s", fileIdx+1, len(headerPaths), fileName))

        processedText, err := parseHeaderFile(hpath, alias, fs, progress, perFileWeight)
        if err != nil {
            return fmt.Errorf("failed to parse %s: %w", hpath, err)
        }
        allProcessedText = append(allProcessedText, processedText)
    }

    // After all files are processed, evaluate all collected macros
    progress.update(0, "Evaluating macros", "")
    if err := evaluateAllMacros(alias, fs, progress); err != nil {
        return fmt.Errorf("failed to evaluate macros: %w", err)
    }

    // Now parse enums and functions after macros are evaluated
    // This ensures enum values that reference macros can be evaluated correctly
    combinedText := strings.Join(allProcessedText, "\n")

    progress.update(0, "Parsing enums", "")
    if err := parseEnums(combinedText, alias, fs); err != nil {
        return fmt.Errorf("failed to parse enums: %w", err)
    }
    progress.update(4.0, "Parsed enums", "")

    progress.update(0, "Parsing functions", "")
    if err := parseFunctionSignatures(combinedText, alias); err != nil {
        return fmt.Errorf("failed to parse functions: %w", err)
    }
    progress.update(4.0, "Completed", "")

    // CACHE: Save parsed data to cache for future runs (reusing the key computed at load time)
    if cacheKeyErr != nil {
        // Cache key computation failed earlier, skip save
        if debugAuto {
            fmt.Printf("[AUTO] Skipping cache save due to missing cache key\n")
        }
    } else if err := saveFFICache(cacheKey, alias); err != nil {
        // Log but don't fail compilation on cache save errors
        if debugAuto {
            fmt.Printf("[AUTO] Warning: cache save failed: %v\n", err)
        }
    }

    return nil
}

// discoverHeaders attempts to auto-discover header files for a library
func discoverHeaders(libraryPath string) []string {
    // Extract library name from path
    // libpng.so → png.h
    // Try common locations:
    //   /usr/include/
    //   /usr/local/include/
    //   /usr/include/<arch>-linux-gnu/

    baseName := filepath.Base(libraryPath)
    // Strip "lib" prefix and ".so" suffix
    name := strings.TrimPrefix(baseName, "lib")
    name = strings.TrimSuffix(name, ".so")
    name = strings.TrimSuffix(name, ".so.6") // Handle versioned libs

    headerName := name + ".h"

    searchPaths := []string{
        "/usr/include/" + headerName,
        "/usr/local/include/" + headerName,
        "/usr/include/" + name + "/" + headerName, // e.g., /usr/include/curl/curl.h
    }

    // Add architecture-specific path
    if arch := runtime.GOARCH; arch == "amd64" {
        searchPaths = append(searchPaths, "/usr/include/x86_64-linux-gnu/"+headerName)
    } else if arch == "arm64" {
        searchPaths = append(searchPaths, "/usr/include/aarch64-linux-gnu/"+headerName)
    }

    var found []string
    for _, path := range searchPaths {
        if _, err := os.Stat(path); err == nil {
            found = append(found, path)
            break // Use first found
        }
    }

    return found
}

// parseHeaderFile parses a single C header file
// parseHeaderFile parses a single C header file and returns the processed text
// Enums and functions are not parsed here - they're parsed later after macro evaluation
func parseHeaderFile(path string, alias string, fs uint32, progress *AutoProgressTracker, allocatedWeight float64) (string, error) {
    debugAuto := os.Getenv("ZA_DEBUG_AUTO") != ""
    if debugAuto {
        fmt.Printf("[AUTO] parseHeaderFile START: path=%s, alias=%s\n", path, alias)
    }

    content, err := os.ReadFile(path)
    if err != nil {
        if debugAuto {
            fmt.Printf("[AUTO] parseHeaderFile: failed to read file: %v\n", err)
        }
        return "", err
    }

    text := string(content)

    // Step 0: Strip comments FIRST (before preprocessor parsing)
    // Otherwise, preprocessor directives inside comments will be processed
    text = stripCComments(text)
    progress.update(allocatedWeight*0.05, "", "stripping comments")

    // Step 0.1a: Parse included headers for typedefs and struct definitions
    // This MUST happen on the ORIGINAL text before preprocessing, because
    // parsePreprocessor may remove or modify #include directives
    parseIncludedHeaders(text, alias, path)
    progress.update(allocatedWeight*0.10, "", "parsing includes")

    // Step 0.1b: Parse preprocessor conditionals
    // This resolves #ifdef blocks so that typedef and struct definitions are clean
    text = parsePreprocessor(text, alias, path, fs)
    progress.update(allocatedWeight*0.20, "", "preprocessing")

    // Step 0.3: Parse typedefs from main file before structs
    // Typedefs must be registered before parsing structs that use them
    if err := parseTypedefs(text, alias); err != nil {
        if os.Getenv("ZA_WARN_AUTO") != "" {
            msg := fmt.Sprintf("[AUTO] Warning: early typedef parsing failed for %s: %v", path, err)
            if progress != nil {
                progress.addMessage(msg)
            } else {
                fmt.Fprintf(os.Stderr, "%s\n", msg)
            }
        }
    }
    progress.update(allocatedWeight*0.15, "", "parsing typedefs")

    // Step 0.4: Parse plain structs AFTER typedefs and preprocessing
    // Now typedef-based types can be resolved
    if err := parsePlainStructs(text, alias); err != nil {
        if os.Getenv("ZA_WARN_AUTO") != "" {
            msg := fmt.Sprintf("[AUTO] Warning: plain struct parsing failed for %s: %v", path, err)
            if progress != nil {
                progress.addMessage(msg)
            } else {
                fmt.Fprintf(os.Stderr, "%s\n", msg)
            }
        }
        // Don't fail - continue with other parsing
    }
    progress.update(allocatedWeight*0.25, "", "parsing structs")

    // Step 0.5: Remove known empty marker macros that interfere with parsing
    // These are typically defined as empty and used for C++ compatibility
    text = removeEmptyMarkerMacros(text)

    // Step 0.6: Remove export macros with parameters (e.g., FT_EXPORT(type))
    // These interfere with function signature regex matching
    text = removeExportMacros(text)

    // Step 1.5: Normalize multiline declarations
    text = normalizeFunctionDeclarations(text)

    // Step 1.6: Parse #define macros for simple constants
    if err := parseDefines(text, alias, fs); err != nil {
        if os.Getenv("ZA_WARN_AUTO") != "" {
            msg := fmt.Sprintf("[AUTO] Warning: define parsing failed for %s: %v", path, err)
            if progress != nil {
                progress.addMessage(msg)
            } else {
                fmt.Fprintf(os.Stderr, "%s\n", msg)
            }
        }
        // Don't fail on define parsing errors - continue with other parsing
    }

    // Step 1.7: Skip duplicate typedef parsing (already done in step 0.3)
    // We already parsed typedefs early to support struct field type resolution

    // Step 1.8: Parse struct typedefs (before unions so they can reference structs)
    if err := parseStructTypedefs(text, alias); err != nil {
        if os.Getenv("ZA_WARN_AUTO") != "" {
            msg := fmt.Sprintf("[AUTO] Warning: struct parsing failed for %s: %v", path, err)
            if progress != nil {
                progress.addMessage(msg)
            } else {
                fmt.Fprintf(os.Stderr, "%s\n", msg)
            }
        }
        // Don't fail on struct parsing errors - continue with other parsing
    }

    // Note: Plain structs are now parsed early (before preprocessing) in Step 0.3 above

    // Step 1.9: Parse union typedefs (after structs so they can use struct-typed fields)
    if err := parseUnionTypedefs(text, alias); err != nil {
        if os.Getenv("ZA_WARN_AUTO") != "" {
            msg := fmt.Sprintf("[AUTO] Warning: union parsing failed for %s: %v", path, err)
            if progress != nil {
                progress.addMessage(msg)
            } else {
                fmt.Fprintf(os.Stderr, "%s\n", msg)
            }
        }
        // Don't fail on union parsing errors - continue with other parsing
    }
    progress.update(allocatedWeight*0.15, "", "parsing unions")

    // Step 2: Parse #define constants (just collect them, evaluation happens later)
    if err := parseDefines(text, alias, fs); err != nil {
        return "", err
    }
    progress.update(allocatedWeight*0.10, "", "collecting macros")

    // Step 2.5: Remove #define lines now that they've been processed
    // This prevents them from being mistaken for function signatures
    lines := strings.Split(text, "\n")
    var filteredLines []string
    for _, line := range lines {
        trimmed := strings.TrimSpace(line)
        if !strings.HasPrefix(trimmed, "#define") {
            filteredLines = append(filteredLines, line)
        }
    }
    text = strings.Join(filteredLines, "\n")

    // Return the processed text for enum/function parsing after macro evaluation
    return text, nil
}

// resolveIncludePath resolves an #include directive to an absolute file path
// Handles both #include "file.h" (local) and #include <file.h> (system)
// Returns empty string if not found
func resolveIncludePath(includeLine string, currentFile string) string {
    // Parse the include directive
    // #include "file.h" → file.h (local first)
    // #include <file.h> → file.h (system only)
    includeLine = strings.TrimSpace(includeLine)

    var filename string
    isSystemInclude := false

    if strings.HasPrefix(includeLine, "<") && strings.HasSuffix(includeLine, ">") {
        // System include: <stdio.h>
        filename = strings.TrimSuffix(strings.TrimPrefix(includeLine, "<"), ">")
        isSystemInclude = true
    } else if strings.HasPrefix(includeLine, "\"") && strings.HasSuffix(includeLine, "\"") {
        // Local include: "myheader.h"
        filename = strings.TrimSuffix(strings.TrimPrefix(includeLine, "\""), "\"")
        isSystemInclude = false
    } else {
        // Malformed include
        return ""
    }

    // Try local directory first (for "file.h" includes)
    if !isSystemInclude && currentFile != "" {
        localDir := filepath.Dir(currentFile)
        localPath := filepath.Join(localDir, filename)
        if _, err := os.Stat(localPath); err == nil {
            absPath, _ := filepath.Abs(localPath)
            return absPath
        }
    }

    // Try standard include paths
    searchPaths := []string{
        "/usr/include/" + filename,
        "/usr/local/include/" + filename,
    }

    // Add architecture-specific paths
    if runtime.GOARCH == "amd64" {
        searchPaths = append(searchPaths, "/usr/include/x86_64-linux-gnu/"+filename)
    } else if runtime.GOARCH == "arm64" {
        searchPaths = append(searchPaths, "/usr/include/aarch64-linux-gnu/"+filename)
    }

    // Also try subdirectories (e.g., /usr/include/curl/curl.h)
    parts := strings.Split(filename, "/")
    if len(parts) > 1 {
        // It's already a path like "curl/curl.h", just use it
    } else {
        // Try common subdirectory pattern: /usr/include/<name>/<name>.h
        // For "png.h" try /usr/include/png/png.h
        baseName := strings.TrimSuffix(filename, filepath.Ext(filename))
        searchPaths = append(searchPaths, "/usr/include/"+baseName+"/"+filename)
    }

    // Search for the file
    for _, path := range searchPaths {
        if _, err := os.Stat(path); err == nil {
            absPath, _ := filepath.Abs(path)
            return absPath
        }
    }

    return ""
}

// parsePreprocessor filters header text based on preprocessor conditionals
// Returns the filtered text with only active blocks included
func parsePreprocessor(text string, alias string, headerPath string, fs uint32) string {
    state := newPreprocessorState(alias)
    // Mark the initial header as visited
    if headerPath != "" {
        absPath, _ := filepath.Abs(headerPath)
        state.visitedHeaders[absPath] = true
    }
    return parsePreprocessorWithState(text, state, headerPath, fs)
}

// extractOriginalMacros extracts #define directives from text before line joining
// Returns map of macro_name → original_text (preserving backslash continuations)
func extractOriginalMacros(text string) map[string]string {
    result := make(map[string]string)
    lines := strings.Split(text, "\n")

    for i := 0; i < len(lines); i++ {
        line := lines[i]
        trimmed := strings.TrimSpace(line)

        // Look for #define directives
        if !strings.HasPrefix(trimmed, "#") {
            continue
        }

        // Parse the directive
        directive := strings.TrimPrefix(trimmed, "#")
        directive = strings.TrimSpace(directive)

        parts := strings.SplitN(directive, " ", 2)
        if len(parts) < 2 || parts[0] != "define" {
            continue
        }

        // Extract macro name and full definition with backslash continuations
        macroStart := i
        macroLines := []string{line}

        // Follow backslash continuations
        for i < len(lines)-1 {
            lineContent := strings.TrimRight(lines[i], " \t\r")
            if !strings.HasSuffix(lineContent, "\\") {
                break
            }
            i++
            macroLines = append(macroLines, lines[i])
        }

        // Join with newlines to preserve structure
        fullMacro := strings.Join(macroLines, "\n")

        // Extract macro name from the first line
        firstLine := strings.TrimSpace(macroLines[0])
        firstLine = strings.TrimPrefix(firstLine, "#")
        firstLine = strings.TrimSpace(firstLine)
        firstLine = strings.TrimPrefix(firstLine, "define")
        firstLine = strings.TrimSpace(firstLine)

        // Macro name is everything up to first space or (
        var macroName string
        if idx := strings.IndexAny(firstLine, " \t("); idx != -1 {
            macroName = firstLine[:idx]
        } else {
            macroName = firstLine
        }

        if macroName != "" {
            result[macroName] = fullMacro
        }

        _ = macroStart // unused but kept for clarity
    }

    return result
}

// joinLineContinuations handles C preprocessor line continuations (lines ending with \)
// Joins lines ending with \ with the next line, removing the backslash
func joinLineContinuations(text string) string {
    lines := strings.Split(text, "\n")
    var result []string

    for i := 0; i < len(lines); i++ {
        line := lines[i]

        // Check if line ends with backslash (after trimming trailing whitespace)
        trimmed := strings.TrimRight(line, " \t\r")
        for strings.HasSuffix(trimmed, "\\") && i+1 < len(lines) {
            // Remove the backslash and append next line
            trimmed = strings.TrimSuffix(trimmed, "\\")
            i++
            nextLine := strings.TrimLeft(lines[i], " \t")
            trimmed = trimmed + " " + nextLine
            trimmed = strings.TrimRight(trimmed, " \t\r")
        }

        result = append(result, trimmed)
    }

    return strings.Join(result, "\n")
}

// parsePreprocessorWithState filters header text using an existing preprocessor state
// This allows recursive #include processing with shared state
func parsePreprocessorWithState(text string, state *PreprocessorState, currentFile string, fs uint32) string {
    // Strip comments FIRST (before any preprocessing)
    text = stripCComments(text)

    // Extract original macro definitions (before line joining) for display purposes
    if state.alias != "" {
        originalMacros := extractOriginalMacros(text)
        if len(originalMacros) > 0 {
            moduleMacrosOriginalLock.Lock()
            if moduleMacrosOriginal[state.alias] == nil {
                moduleMacrosOriginal[state.alias] = make(map[string]string)
            }
            for name, originalText := range originalMacros {
                // Only store if not already present (first definition wins)
                if _, exists := moduleMacrosOriginal[state.alias][name]; !exists {
                    moduleMacrosOriginal[state.alias][name] = originalText
                }
            }
            moduleMacrosOriginalLock.Unlock()
        }
    }

    // Handle line continuations
    text = joinLineContinuations(text)

    lines := strings.Split(text, "\n")
    var result []string

    debugAuto := os.Getenv("ZA_DEBUG_AUTO") != ""

    for lineNum, line := range lines {
        trimmed := strings.TrimSpace(line)

        // Handle preprocessor directives
        if strings.HasPrefix(trimmed, "#") {
            directive, arg := parseDirective(trimmed)

            switch directive {
            case "ifdef":
                // #ifdef NAME
                condition := state.isDefined(arg)
                state.pushCondition(condition)
                if debugAuto {
                    fmt.Printf("[AUTO] Line %d: #ifdef %s → %v (depth %d)\n",
                        lineNum+1, arg, condition, state.includeDepth)
                }
                continue // Don't include directive itself

            case "ifndef":
                // #ifndef NAME
                condition := !state.isDefined(arg)
                state.pushCondition(condition)
                if debugAuto {
                    isStructStat := strings.Contains(currentFile, "struct_stat")
                    if isStructStat || arg == "_BITS_STRUCT_STAT_H" {
                        fmt.Printf("[AUTO] Line %d: #ifndef %s → %v (depth %d) in struct_stat file\n",
                            lineNum+1, arg, condition, state.includeDepth)
                    } else {
                        fmt.Printf("[AUTO] Line %d: #ifndef %s → %v (depth %d)\n",
                            lineNum+1, arg, condition, state.includeDepth)
                    }
                }
                continue

            case "if":
                // #if expression
                // Preprocess defined() operators, then evaluate the expression using Za's ev() evaluator
                preprocessed := state.preprocessIfExpression(arg)
                // Populate ident with defined macros so they can be referenced in the expression
                ident := state.getMacrosAsIdent()

                if debugAuto {
                    fmt.Printf("[AUTO] Line %d: #if %s → preprocessed: %s\n", lineNum+1, arg, preprocessed)
                    fmt.Printf("[AUTO]   Macros available: ")
                    for i, v := range ident {
                        fmt.Printf("%s=%v(%T) ", v.IName, v.IValue, v.IValue)
                        if i > 5 {
                            fmt.Printf("...")
                            break
                        }
                    }
                    fmt.Printf("\n")
                }

                // Evaluate the expression - if it fails, treat as false
                condition := false
                if debugAuto {
                    fmt.Printf("[AUTO]   → About to call evaluateConstant() with expr=%q, fs=%d\n", preprocessed, fs)
                }
                result, ok := evaluateConstant(preprocessed, state, state.alias, fs, true, nil) // #if expression
                if debugAuto {
                    fmt.Printf("[AUTO]   → evaluateConstant() returned: result=%v, ok=%v\n", result, ok)
                }
                if ok {
                    condition = isTruthy(result)
                } else {
                    if debugAuto {
                        fmt.Printf("[AUTO]   → evaluation failed, treating as false\n")
                    }
                }

                state.pushCondition(condition)
                if debugAuto {
                    fmt.Printf("[AUTO]   → result=%v, ok=%v, condition=%v, depth=%d\n",
                        result, ok, condition, state.includeDepth)
                }
                continue

            case "elif":
                // #elif expression
                // Only evaluate and activate if no previous condition in chain was satisfied
                preprocessed := state.preprocessIfExpression(arg)

                // Evaluate the expression - if it fails, treat as false
                condition := false
                result, ok := evaluateConstant(preprocessed, state, state.alias, fs, true, nil) // #elif expression
                if ok {
                    condition = isTruthy(result)
                }

                state.replaceConditionInChain(condition)
                if debugAuto {
                    fmt.Printf("[AUTO] Line %d: #elif %s → preprocessed: %s → %v (result=%v, ok=%v, depth %d, active=%v)\n",
                        lineNum+1, arg, preprocessed, condition, result, ok, state.includeDepth, state.isActive())
                }
                continue

            case "else":
                // #else
                state.toggleCondition()
                if debugAuto {
                    fmt.Printf("[AUTO] Line %d: #else (depth %d, active=%v)\n",
                        lineNum+1, state.includeDepth, state.isActive())
                }
                continue

            case "endif":
                // #endif
                if debugAuto {
                    fmt.Printf("[AUTO] Line %d: #endif (depth before=%d, stack size=%d)\n",
                        lineNum+1, state.includeDepth, len(state.conditionStack))
                }
                state.popCondition()
                if debugAuto {
                    fmt.Printf("[AUTO]   After pop: depth=%d, isActive=%v\n", state.includeDepth, state.isActive())
                }
                continue

            case "define":
                // #define NAME[(params)] VALUE - Track for conditionals
                if state.isActive() {
                    // Use regex to properly parse #define directives
                    // This preserves function-like macro parameter lists for display
                    defineRegex := regexp.MustCompile(`^(\w+)(\([^)]*\))?\s*(.*)$`)
                    matches := defineRegex.FindStringSubmatch(arg)

                    if matches != nil && len(matches) >= 4 {
                        name := matches[1]           // Macro name (e.g., "isless")
                        params := matches[2]         // Parameter list with parens (e.g., "(x, y)") or empty
                        body := matches[3]           // Macro body/value
                        isFunctionLike := params != ""

                        // Strip C integer/float suffixes from the body
                        body = stripCIntegerSuffixes(body)

                        // For conditional evaluation (#if expressions), store just the body
                        state.definedMacros[name] = body

                        // For help plugin display, store with full parameter list if function-like
                        if state.alias != "" {
                            moduleMacrosLock.Lock()
                            if moduleMacros[state.alias] == nil {
                                moduleMacros[state.alias] = make(map[string]string)
                            }

                            // Store display value
                            var displayValue string
                            if isFunctionLike {
                                // Function-like: store as "name(params) body"
                                displayValue = name + params + " " + body
                            } else {
                                // Object-like: store just the body
                                displayValue = body
                            }

                            if _, exists := moduleMacros[state.alias][name]; !exists {
                                moduleMacros[state.alias][name] = displayValue
                            }

                            // Record the order for evaluation
                            // Skip function-like macros and system/compiler macros (those starting with __)
                            // Also skip macros whose values reference system macros
                            moduleMacrosOrderLock.Lock()
                            if moduleMacrosOrder[state.alias] == nil {
                                moduleMacrosOrder[state.alias] = make([]string, 0)
                            }

                            // Only add evaluatable macros: object-like macros that:
                            // 1. Don't start with __
                            // 2. Don't reference system macros (containing __)
                            // 3. Don't contain sizeof, typedef, extern, etc.
                            // 4. Aren't self-referential (value != name)
                            // 5. Don't contain complex operators (?, :, ~)
                            // 6. Aren't simple macro aliases (single UPPERCASE_WORD)
                            // 7. Don't contain struct/array initializers (causes ev() to hang)
                            // 8. Don't contain function-like macro calls
                            // 9. Don't contain C keywords
                            // 10. Don't contain type keywords
                            // 11. Don't contain cast expressions

                            hasStructInit := strings.Contains(body, "{") || strings.Contains(body, "}")

                            // Filter function-like macro calls in body
                            hasFunctionCall := regexp.MustCompile(`\w+\s*\(`).MatchString(body)

                            // Filter C keywords
                            cKeywords := []string{
                                "inline", "__inline", "__inline__",
                                "static", "const", "volatile", "restrict",
                                "_Noreturn", "__noreturn__",
                                "register", "auto", "_Thread_local",
                            }
                            hasKeyword := false
                            for _, kw := range cKeywords {
                                // Match as whole word: exact match, or with spaces around it
                                if body == kw ||
                                   strings.HasPrefix(body, kw+" ") ||
                                   strings.HasSuffix(body, " "+kw) ||
                                   strings.Contains(body, " "+kw+" ") {
                                    hasKeyword = true
                                    break
                                }
                            }

                            // Filter type keywords
                            typeKeywords := []string{
                                "void", "char", "short", "int", "long", "float", "double",
                                "signed", "unsigned",
                                "size_t", "ssize_t", "ptrdiff_t", "wchar_t",
                                "int8_t", "int16_t", "int32_t", "int64_t",
                                "uint8_t", "uint16_t", "uint32_t", "uint64_t",
                                "intptr_t", "uintptr_t",
                            }
                            hasTypeKeyword := false
                            for _, tk := range typeKeywords {
                                // Check if body is exactly the type, or contains it
                                if body == tk || strings.Contains(body, tk) {
                                    hasTypeKeyword = true
                                    break
                                }
                            }

                            // Filter cast expressions (C-style casts)
                            hasCast := regexp.MustCompile(`\([a-zA-Z_][a-zA-Z0-9_]*\)\s*\(`).MatchString(body)

                            isSimpleAlias := regexp.MustCompile(`^[A-Z_][A-Z0-9_]*$`).MatchString(body)

                            // Allow __SIZEOF_* and __WORDSIZE macros (common size constants)
                            allowedSystemMacro := false
                            if strings.HasPrefix(name, "__SIZEOF_") || name == "__WORDSIZE" {
                                // These are numeric constants, not system references
                                if _, err := strconv.Atoi(body); err == nil {
                                    allowedSystemMacro = true
                                }
                            }

                            // Allow sizeof expressions in specific common system macros
                            // These are evaluated by replacing sizeof() with actual sizes
                            allowedSizeofMacro := false
                            if name == "_SIGSET_NWORDS" || name == "__NFDBITS" ||
                               name == "__FD_SETSIZE" || (strings.HasPrefix(name, "__") &&
                               strings.Contains(name, "SIZE")) {
                                // Check if this is a simple arithmetic expression with sizeof
                                // Examples: (1024 / (8 * sizeof(...))), (8 * sizeof(...))
                                if strings.Contains(body, "sizeof") &&
                                   !strings.Contains(body, "typedef") &&
                                   !strings.Contains(body, "struct") &&
                                   !strings.Contains(body, "union") &&
                                   !strings.Contains(body, "enum") {
                                    allowedSizeofMacro = true
                                }
                            }

                            shouldEvaluate := !isFunctionLike &&
                                              (!strings.HasPrefix(name, "__") || allowedSystemMacro || allowedSizeofMacro) &&
                                              (!strings.Contains(body, "__") || allowedSizeofMacro) &&
                                              (!strings.Contains(body, "sizeof") || allowedSizeofMacro) &&
                                              !strings.Contains(body, "typedef") &&
                                              !strings.Contains(body, "extern") &&
                                              !strings.Contains(body, "?") &&
                                              !strings.Contains(body, "~") &&
                                              !hasStructInit &&
                                              (!hasFunctionCall || allowedSizeofMacro) &&
                                              !hasKeyword &&
                                              (!hasTypeKeyword || allowedSizeofMacro) &&
                                              !hasCast &&
                                              !isSimpleAlias &&
                                              body != name

                            if shouldEvaluate {
                                moduleMacrosOrder[state.alias] = append(moduleMacrosOrder[state.alias], name)
                            } else if !isFunctionLike {
                                // Only mark object-like macros as skipped
                                // Function-like macros don't get status tracking (will show "?" unknown)
                                macroEvalStatusLock.Lock()
                                if macroEvalStatus[state.alias] == nil {
                                    macroEvalStatus[state.alias] = make(map[string]string)
                                }
                                macroEvalStatus[state.alias][name] = "skipped"
                                macroEvalStatusLock.Unlock()
                            }
                            moduleMacrosOrderLock.Unlock()

                            moduleMacrosLock.Unlock()
                        }

                        if debugAuto {
                            if isFunctionLike {
                                fmt.Printf("[AUTO] Line %d: #define %s%s %s\n", lineNum+1, name, params, body)
                            } else {
                                fmt.Printf("[AUTO] Line %d: #define %s %s\n", lineNum+1, name, body)
                            }
                        }
                    }
                }
                // Include line so parseDefines can process it
                // (but only if in active block - handled by isActive() check below)

            case "include":
                // #include "file.h" or #include <file.h>
                if state.isActive() {
                    includePath := resolveIncludePath(arg, currentFile)
                    if includePath != "" {
                        // Check for cycles
                        if state.visitedHeaders[includePath] {
                            if debugAuto {
                                fmt.Printf("[AUTO] Line %d: #include %s SKIPPED (already visited)\n", lineNum+1, arg)
                            }
                            continue
                        }

                        // Mark as visited
                        state.visitedHeaders[includePath] = true

                        if debugAuto {
                            fmt.Printf("[AUTO] Line %d: #include %s → %s\n", lineNum+1, arg, includePath)
                        }

                        // Read and process the included file
                        includeContent, err := os.ReadFile(includePath)
                        if err != nil {
                            if debugAuto {
                                fmt.Printf("[AUTO] Line %d: Failed to read %s: %v\n", lineNum+1, includePath, err)
                            }
                            continue
                        }

                        // Strip comments from included file BEFORE recursive processing
                        // This ensures function declarations aren't obscured by comments
                        includeText := stripCComments(string(includeContent))

                        // Recursively process the included file
                        processedInclude := parsePreprocessorWithState(includeText, state, includePath, fs)

                        // CRITICAL: Process included content the same way as main file
                        // Remove empty marker macros and export macros, then normalize function declarations
                        processedInclude = removeEmptyMarkerMacros(processedInclude)
                        processedInclude = removeExportMacros(processedInclude)
                        processedInclude = normalizeFunctionDeclarations(processedInclude)

                        // Append the processed content to results
                        if processedInclude != "" {
                            result = append(result, processedInclude)
                        }
                    } else if debugAuto {
                        fmt.Printf("[AUTO] Line %d: #include %s NOT FOUND\n", lineNum+1, arg)
                    }
                }
                continue // Don't include the #include directive itself
            }
        }

        // Include line only if in active block
        if state.isActive() {
            result = append(result, line)
            if debugAuto && (strings.Contains(trimmed, "struct stat") || strings.Contains(trimmed, "_BITS_STRUCT_STAT_H")) {
                fmt.Printf("[AUTO] Line %d: INCLUDED (active): %s\n", lineNum+1, trimmed)
            }
        } else if debugAuto && trimmed != "" {
            if strings.Contains(trimmed, "struct stat") || strings.Contains(trimmed, "_BITS_STRUCT_STAT_H") {
                fmt.Printf("[AUTO] Line %d: SKIPPED (inactive) [IMPORTANT]: %s\n", lineNum+1, trimmed)
            } else {
                fmt.Printf("[AUTO] Line %d: SKIPPED (inactive): %s\n", lineNum+1, trimmed)
            }
        }
    }

    return strings.Join(result, "\n")
}

// parseDirective splits a preprocessor directive into command and argument
// "#ifdef FOO" → ("ifdef", "FOO")
// "#define BAR 42" → ("define", "BAR 42")
func parseDirective(line string) (string, string) {
    line = strings.TrimSpace(line)
    if !strings.HasPrefix(line, "#") {
        return "", ""
    }

    line = strings.TrimPrefix(line, "#")
    line = strings.TrimSpace(line)

    // Split on first whitespace
    parts := strings.Fields(line)
    if len(parts) == 0 {
        return "", ""
    }

    directive := parts[0]
    arg := ""
    if len(parts) > 1 {
        arg = strings.Join(parts[1:], " ")
    }

    return directive, arg
}

// stripCComments removes C-style comments from text
func stripCComments(text string) string {
    // Remove /* ... */ comments (including multiline)
    re1 := regexp.MustCompile(`(?s)/\*.*?\*/`)
    text = re1.ReplaceAllString(text, "")

    // Remove // ... comments
    re2 := regexp.MustCompile(`//.*`)
    text = re2.ReplaceAllString(text, "")

    return text
}

// removeEmptyMarkerMacros removes known empty marker macros that interfere with parsing
// These macros are typically defined as empty and used for C++ compatibility or attributes
func removeEmptyMarkerMacros(text string) string {
    // List of known empty marker macros to remove
    emptyMacros := []string{
        "__BEGIN_DECLS",
        "__END_DECLS",
        "__THROW",
        "__THROWNL",
        "__nonnull",
        "__wur",
        "__attribute_const__",
        "__attribute_pure__",
        "__attribute_malloc__",
        "__attribute_artificial__",
        "__attribute_maybe_unused__",
        "__attribute_warn_unused_result__",
        "__returns_nonnull",
        "__attribute_deprecated__",
        "__COLD",
        "__LEAF",
        "__LEAF_ATTR",
    }

    // Remove each macro as a standalone word
    // Use word boundaries to avoid removing parts of other identifiers
    for _, macro := range emptyMacros {
        pattern := `\b` + regexp.QuoteMeta(macro) + `\b`
        re := regexp.MustCompile(pattern)
        text = re.ReplaceAllString(text, "")
    }

    // Clean up any resulting double spaces
    text = regexp.MustCompile(`  +`).ReplaceAllString(text, " ")

    return text
}

// removeExportMacros handles export/visibility macros that have parameters
// These interfere with function signature regex matching because the regex
// matches the first parenthesis it encounters, which would be the macro's
// parameter list instead of the function's parameter list.
//
// For export macros like FT_EXPORT(type), the parameter IS the return type,
// so we extract it. For attribute macros like __declspec(dllexport), we
// remove the entire macro.
func removeExportMacros(text string) string {
    // Export macros where the parameter IS the return type
    // Extract the type from MACRO(type) -> type
    returnTypeMacros := []string{
        "FT_EXPORT",
        "FT_EXPORT_DEF",
        "FT_EXPORT_FUNC",
        "FT_BASE",
        "FT_BASE_DEF",
        "PNG_EXPORT",
        "PNGAPI",
        "ZEXPORT",
        "ZEXPORTVA",
    }

    for _, macro := range returnTypeMacros {
        // Match MACRO(...) and capture the content inside parentheses
        // The captured group is the return type
        pattern := regexp.MustCompile(regexp.QuoteMeta(macro) + `\s*\(([^)]*)\)\s*`)
        // Replace with the captured content (the return type)
        text = pattern.ReplaceAllString(text, "$1 ")
    }

    // Visibility/attribute macros where the parameter should be discarded
    // Remove the entire macro including its parameter
    attributeMacros := []string{
        "__declspec",
        "__attribute__",
    }

    for _, macro := range attributeMacros {
        // Remove entire macro including its parameter
        pattern := regexp.MustCompile(regexp.QuoteMeta(macro) + `\s*\([^)]*\)\s*`)
        text = pattern.ReplaceAllString(text, " ")
    }

    // Clean up any resulting double spaces
    text = regexp.MustCompile(`  +`).ReplaceAllString(text, " ")

    return text
}

// parseDefines extracts #define constants from header text
func parseDefines(text string, alias string, fs uint32) error {
    // Match: #define NAME VALUE
    // Support: integers, hex, floats, strings, and expressions using ev()
    // Note: Use [ \t]+ instead of \s+ to avoid matching newlines (which would match across lines)
    // Changed to match any identifier (not just uppercase) to catch Bool, True, etc.

    re := regexp.MustCompile(`(?m)^\s*#define\s+([A-Za-z_][A-Za-z0-9_]*)[ \t]+(.+)$`)
    matches := re.FindAllStringSubmatch(text, -1)

    debugAuto := os.Getenv("ZA_DEBUG_AUTO") != ""
    if debugAuto {
        fmt.Printf("[AUTO] parseDefines found %d #define statements\n", len(matches))
    }

    // Initialize moduleConstants map if needed
    moduleConstantsLock.Lock()
    if moduleConstants[alias] == nil {
        moduleConstants[alias] = make(map[string]any)
    }
    moduleConstantsLock.Unlock()

    for _, match := range matches {
        name := match[1]
        valueStr := strings.TrimSpace(match[2])

        if debugAuto {
            fmt.Printf("[AUTO] parseDefines processing: %s = %s\n", name, valueStr)
        }

        // Skip function-like macros: #define NAME(params) value
        // These look like: valueStr starts with "(identifier)" or "(a,b)"
        if strings.HasPrefix(valueStr, "(") {
            // Check if it looks like a parameter list: (x) or (a,b) followed by something
            closeIdx := strings.Index(valueStr, ")")
            if closeIdx > 0 && closeIdx < len(valueStr)-1 {
                // Has content after the closing paren - likely a function-like macro
                paramPart := valueStr[1:closeIdx]
                // Simple check: contains only letters, digits, underscores, spaces, commas
                isMacro := true
                for _, ch := range paramPart {
                    if !(ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z' ||
                        ch >= '0' && ch <= '9' || ch == '_' || ch == ' ' || ch == ',') {
                        isMacro = false
                        break
                    }
                }
                if isMacro {
                    if debugAuto := os.Getenv("ZA_DEBUG_AUTO") != ""; debugAuto {
                        fmt.Printf("[AUTO]   → Skipping function-like macro: %s = %s\n", name, valueStr)
                    }
                    continue // Skip function-like macros
                }
            }
        }

        // Apply the same filtering as the preprocessor to avoid adding
        // simple macro aliases that reference undefined identifiers
        // This prevents ev() from hanging on expressions like:
        // #define ft_encoding_gb2312 FT_ENCODING_PRC
        // where FT_ENCODING_PRC is not defined

        // Check for simple alias (single identifier referencing another undefined macro)
        // Example: #define ft_encoding_gb2312 FT_ENCODING_PRC
        isSimpleAlias := regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`).MatchString(valueStr)

        // Also apply other common filters from the preprocessor
        hasStructInit := strings.Contains(valueStr, "{") || strings.Contains(valueStr, "}")
        hasFunctionCall := regexp.MustCompile(`\w+\s*\(`).MatchString(valueStr)
        hasSystemRef := strings.Contains(valueStr, "__")
        hasComplexOps := strings.Contains(valueStr, "?") || strings.Contains(valueStr, "~")

        // Check for type aliases like #define Bool int
        // These should be added to typedef registry, not as constants
        typeKeywords := []string{
            "void", "char", "short", "int", "long", "float", "double",
            "signed", "unsigned",
            "size_t", "ssize_t", "ptrdiff_t", "wchar_t",
        }
        isTypeAlias := false
        for _, tk := range typeKeywords {
            if valueStr == tk {
                // Exact match - this is a type alias
                isTypeAlias = true
                moduleTypedefsLock.Lock()
                if moduleTypedefs[alias] == nil {
                    moduleTypedefs[alias] = make(map[string]string)
                }
                moduleTypedefs[alias][name] = valueStr
                moduleTypedefsLock.Unlock()
                if debugAuto {
                    fmt.Printf("[AUTO] Type alias (via #define): %s → %s\n", name, valueStr)
                }
                break
            }
        }

        if isTypeAlias {
            continue // Skip to next define
        }

        // Check for type/cast keywords in expressions (not type aliases)
        hasTypeKeyword := false
        for _, tk := range typeKeywords {
            if strings.Contains(valueStr, tk) {
                hasTypeKeyword = true
                break
            }
        }

        shouldSkip := isSimpleAlias || hasStructInit || hasFunctionCall ||
            hasSystemRef || hasComplexOps || hasTypeKeyword ||
            strings.Contains(valueStr, "sizeof") ||
            strings.Contains(valueStr, "typedef") ||
            strings.Contains(valueStr, "extern")

        debugAuto := os.Getenv("ZA_DEBUG_AUTO") != ""

        // Store macro in moduleMacros for display (always, even if filtered)
        moduleMacrosLock.Lock()
        if moduleMacros[alias] == nil {
            moduleMacros[alias] = make(map[string]string)
        }
        if _, exists := moduleMacros[alias][name]; !exists {
            // Only add if not already present from preprocessor
            moduleMacros[alias][name] = valueStr
        }
        moduleMacrosLock.Unlock()

        if shouldSkip {
            if debugAuto {
                fmt.Printf("[AUTO]   → Skipping macro (filtered): %s = %s\n", name, valueStr)
            }

            // Mark as skipped for help display
            macroEvalStatusLock.Lock()
            if macroEvalStatus[alias] == nil {
                macroEvalStatus[alias] = make(map[string]string)
            }
            macroEvalStatus[alias][name] = "skipped"
            macroEvalStatusLock.Unlock()

            continue // Don't add to evaluation order
        }

        // For non-filtered macros, add to evaluation order
        if debugAuto {
            fmt.Printf("[AUTO] Stored macro for later evaluation: %s = %q\n", name, valueStr)
        }

        // Record the order
        moduleMacrosOrderLock.Lock()
        if moduleMacrosOrder[alias] == nil {
            moduleMacrosOrder[alias] = make([]string, 0)
        }
        moduleMacrosOrder[alias] = append(moduleMacrosOrder[alias], name)
        moduleMacrosOrderLock.Unlock()

        // Check if we should abort (interactive mode error signaled via sig_int)
        if sig_int {
            return fmt.Errorf("AUTO import aborted due to evaluation error")
        }
    }

    return nil
}

// evaluateAllMacros evaluates all macros in moduleMacros after all files have been processed
// Uses multiple passes to handle dependencies between macros
// progress can be nil for early evaluations or in contexts without progress tracking
func evaluateAllMacros(alias string, fs uint32, progress *AutoProgressTracker) error {
    debugAuto := os.Getenv("ZA_DEBUG_AUTO") != ""

    if debugAuto {
        fmt.Printf("[AUTO] evaluateAllMacros: Processing all macros for alias=%s\n", alias)
    }

    // Initialize moduleConstants map if needed
    moduleConstantsLock.Lock()
    if moduleConstants[alias] == nil {
        moduleConstants[alias] = make(map[string]any)
    }
    moduleConstantsLock.Unlock()

    // Get macros in the order they were defined
    moduleMacrosOrderLock.RLock()
    orderedMacros := moduleMacrosOrder[alias]
    moduleMacrosOrderLock.RUnlock()

    moduleMacrosLock.RLock()
    allMacros := moduleMacros[alias]
    moduleMacrosLock.RUnlock()

    if debugAuto {
        fmt.Printf("[AUTO] evaluateAllMacros: Found %d macros to evaluate in definition order\n", len(orderedMacros))
    }

    // Use multiple passes to handle dependencies
    // Evaluate macros in the order they were defined - this naturally handles most dependencies
    // Additional passes catch any remaining dependencies (e.g., forward references)

    // Set flag to prevent ev() from calling report()/finish() on errors during AUTO processing
    autoProcessingLock.Lock()
    inAutoProcessing = true
    autoProcessingLock.Unlock()
    defer func() {
        autoProcessingLock.Lock()
        inAutoProcessing = false
        autoProcessingLock.Unlock()
    }()

    maxPasses := 10
    macroWeight := 30.0
    for pass := 0; pass < maxPasses; pass++ {
        evaluated := 0

        if debugAuto {
            fmt.Printf("[AUTO] evaluateAllMacros: Pass %d\n", pass+1)
        }

        // Update progress at start of pass
        passInfo := fmt.Sprintf("(Pass %d/%d)", pass+1, maxPasses)
        if progress != nil {
            progress.update(0, "Evaluating macros", passInfo)
        }

        // Build cached tempIdent ONCE per pass to avoid rebuilding it for each macro evaluation
        // This is a critical optimization: without caching, vset() gets called O(macros * constants) times
        cachedTempIdent := make([]Variable, identInitialSize)

        // Add already-evaluated constants with __c_ prefix
        moduleConstantsLock.RLock()
        if constants, exists := moduleConstants[alias]; exists {
            for name, value := range constants {
                vset(nil, 0, &cachedTempIdent, "__c_"+name, value)
            }
        }
        moduleConstantsLock.RUnlock()

        // Iterate in definition order
        for _, name := range orderedMacros {
            valueStr := allMacros[name]

            // Skip if already evaluated
            moduleConstantsLock.RLock()
            _, alreadyEvaluated := moduleConstants[alias][name]
            moduleConstantsLock.RUnlock()

            if alreadyEvaluated {
                continue
            }

            // Skip if already marked as failed (don't waste time re-evaluating)
            macroEvalStatusLock.RLock()
            status, hasStatus := macroEvalStatus[alias][name]
            macroEvalStatusLock.RUnlock()

            if hasStatus && status == "failed" {
                continue
            }

            // Check if user pressed Ctrl-C BEFORE this evaluation
            lastlock.Lock()
            wasInterrupted := sig_int
            lastlock.Unlock()
            if wasInterrupted {
                // User pressed Ctrl-C before we even started this macro
                return fmt.Errorf("AUTO import interrupted by user")
            }

            // Try to evaluate using cached tempIdent
            if val, ok := evaluateConstant(valueStr, nil, alias, fs, false, &cachedTempIdent); ok {
                if debugAuto {
                    fmt.Printf("[AUTO] evaluateAllMacros: Evaluated %s = %v (type %T)\n", name, val, val)
                }

                // Store in moduleConstants
                moduleConstantsLock.Lock()
                if moduleConstants[alias] == nil {
                    moduleConstants[alias] = make(map[string]any)
                }
                moduleConstants[alias][name] = val
                moduleConstantsLock.Unlock()

                // Mark as evaluated
                macroEvalStatusLock.Lock()
                if macroEvalStatus[alias] == nil {
                    macroEvalStatus[alias] = make(map[string]string)
                }
                macroEvalStatus[alias][name] = "evaluated"
                macroEvalStatusLock.Unlock()

                // Store in cModuleIdents
                cModuleIdentsLock.Lock()
                cModuleIdents[fs] = append(cModuleIdents[fs], Variable{
                    IName:    name,
                    IValue:   val,
                    declared: true,
                })
                cModuleIdentsLock.Unlock()

                evaluated++
            } else {
                // Evaluation failed - this is expected for non-constant identifiers
                // (function names, type names, etc.)
                // Mark as failed
                macroEvalStatusLock.Lock()
                if macroEvalStatus[alias] == nil {
                    macroEvalStatus[alias] = make(map[string]string)
                }
                macroEvalStatus[alias][name] = "failed"
                macroEvalStatusLock.Unlock()
            }
        }

        if debugAuto {
            fmt.Printf("[AUTO] evaluateAllMacros: Pass %d evaluated %d macros\n", pass+1, evaluated)
        }

        // Update progress after this pass
        if evaluated > 0 && progress != nil {
            passWeight := macroWeight / float64(maxPasses)
            progress.update(passWeight, "", passInfo)
        }

        // If no macros were evaluated in this pass, we're done (or stuck on cycles)
        if evaluated == 0 {
            // Allocate remaining weight from skipped passes (including current pass)
            if progress != nil {
                remainingPasses := maxPasses - pass  // Include current pass
                if remainingPasses > 0 {
                    skipWeight := macroWeight * float64(remainingPasses) / float64(maxPasses)
                    progress.update(skipWeight, "", "")
                }
            }
            break
        }
    }

    if debugAuto {
        moduleConstantsLock.RLock()
        totalEvaluated := 0
        if constants, exists := moduleConstants[alias]; exists {
            totalEvaluated = len(constants)
        }
        moduleConstantsLock.RUnlock()
        fmt.Printf("[AUTO] evaluateAllMacros: Completed. Total evaluated: %d out of %d\n", totalEvaluated, len(allMacros))
    }

    return nil
}

// convertCharacterLiterals converts C character literals to their numeric values
// Examples: '\0' → 0, 'A' → 65, '\n' → 10, L'\0' → 0
func convertCharacterLiterals(expr string) string {
    // Match character literals with optional L/u/U prefix
    // Pattern: optional prefix (L, u, U) + single quote + content + single quote
    expr = charLiteralRe.ReplaceAllStringFunc(expr, func(match string) string {
        // Extract the character content (skip prefix and quotes)
        submatches := charLiteralRe.FindStringSubmatch(match)
        if len(submatches) < 3 {
            return match // Keep as-is if parsing fails
        }

        content := submatches[2]

        // Handle escape sequences
        var value int
        if len(content) > 0 && content[0] == '\\' {
            if len(content) < 2 {
                return match // Invalid escape sequence
            }
            switch content[1] {
            case '0':
                value = 0 // Null character
            case 'n':
                value = 10 // Newline
            case 't':
                value = 9 // Tab
            case 'r':
                value = 13 // Carriage return
            case '\\':
                value = 92 // Backslash
            case '\'':
                value = 39 // Single quote
            case '"':
                value = 34 // Double quote
            default:
                // For other escape sequences, keep the original
                return match
            }
        } else if len(content) == 1 {
            // Single character literal
            value = int(content[0])
        } else {
            // Multi-character literal or unknown format
            return match
        }

        return fmt.Sprintf("%d", value)
    })

    return expr
}

// addCPrefixToIdentifiers adds __c_ prefix to identifiers in expressions
// to prevent conflicts with Za keywords during evaluation
func addCPrefixToIdentifiers(expr string) string {
    var result strings.Builder
    inString := false
    escaped := false
    i := 0

    for i < len(expr) {
        ch := expr[i]

        // Handle string literal tracking
        if ch == '"' && !escaped {
            inString = !inString
            result.WriteByte(ch)
            escaped = false
            i++
            continue
        }

        // Track escape sequences
        if ch == '\\' && inString && !escaped {
            escaped = true
            result.WriteByte(ch)
            i++
            continue
        } else {
            escaped = false
        }

        // If we're in a string, don't transform identifiers
        if inString {
            result.WriteByte(ch)
            i++
            continue
        }

        // Check if we have the start of an identifier
        if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || ch == '_' {
            // Check if this is a hex/binary literal prefix (0x, 0X, 0b, 0B)
            // These should NOT be treated as identifiers
            if (ch == 'x' || ch == 'X' || ch == 'b' || ch == 'B') && i > 0 && expr[i-1] == '0' {
                result.WriteByte(ch)
                i++
                // Continue copying hex/binary digits
                for i < len(expr) {
                    c := expr[i]
                    // Always accept decimal digits and underscores
                    if (c >= '0' && c <= '9') || c == '_' {
                        result.WriteByte(c)
                        i++
                    } else if (ch == 'x' || ch == 'X') && ((c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
                        // Accept hex letters for hex literals only
                        result.WriteByte(c)
                        i++
                    } else {
                        // Stop when we hit a non-digit character
                        break
                    }
                }
                continue
            }

            // Extract the full identifier
            start := i
            for i < len(expr) && ((expr[i] >= 'a' && expr[i] <= 'z') ||
                (expr[i] >= 'A' && expr[i] <= 'Z') ||
                (expr[i] >= '0' && expr[i] <= '9') ||
                expr[i] == '_') {
                i++
            }
            match := expr[start:i]

            // Check if this is a character/string literal prefix
            // L'x', L"str", u'x', U'x', u8"str"
            isLiteralPrefix := false
            if i < len(expr) {
                nextChar := expr[i]
                if nextChar == '\'' || nextChar == '"' {
                    // Check if it's a known literal prefix
                    if match == "L" || match == "u" || match == "U" {
                        isLiteralPrefix = true
                    } else if match == "u8" {
                        isLiteralPrefix = true
                    }
                }
            }

            // Don't prefix literal prefixes or Za keywords
            if isLiteralPrefix {
                result.WriteString(match)
                continue
            }

            // Skip Za keywords and C keywords (except sizeof, which is handled separately)
            if match == "true" || match == "false" ||
                match == "typeof" || match == "alignof" ||
                match == "_Alignof" || match == "__alignof__" || match == "__typeof__" {
                result.WriteString(match)
                continue
            }

            // Add __c_ prefix
            result.WriteString("__c_" + match)
        } else {
            result.WriteByte(ch)
            i++
        }
    }

    return result.String()
}

// getCTypeSize returns the size in bytes of a C type
// Handles base types, resolves macros from moduleConstants, and resolves typedefs using moduleTypedefs
func getCTypeSize(typeName string, alias string) (int, bool) {
    // Remove qualifiers (const, volatile, restrict)
    typeName = strings.TrimSpace(typeName)
    typeName = strings.ReplaceAll(typeName, "const ", "")
    typeName = strings.ReplaceAll(typeName, "volatile ", "")
    typeName = strings.ReplaceAll(typeName, "restrict ", "")
    typeName = strings.TrimSpace(typeName)

    // First, try to resolve as a macro from moduleMacros
    // This handles cases like #define __ss_aligntype unsigned long int
    moduleMacrosLock.RLock()
    if macros, exists := moduleMacros[alias]; exists {
        if macroValue, found := macros[typeName]; found {
            moduleMacrosLock.RUnlock()
            // Recursively resolve the macro value
            return getCTypeSize(macroValue, alias)
        }
    }
    moduleMacrosLock.RUnlock()

    // Base C types (64-bit architecture)
    baseTypes := map[string]int{
        "char":                1,
        "signed char":         1,
        "unsigned char":       1,
        "short":               2,
        "short int":           2,
        "signed short":        2,
        "signed short int":    2,
        "unsigned short":      2,
        "unsigned short int":  2,
        "int":                 4,
        "signed":              4,
        "signed int":          4,
        "unsigned":            4,
        "unsigned int":        4,
        "long":                8,
        "long int":            8,
        "signed long":         8,
        "signed long int":     8,
        "unsigned long":       8,
        "unsigned long int":   8,
        "long long":           8,
        "long long int":       8,
        "signed long long":    8,
        "unsigned long long":  8,
        "float":               4,
        "double":              8,
        "long double":         16,
        "void *":              8,
        "_Bool":               1,
    }

    // Check base types first
    if size, ok := baseTypes[typeName]; ok {
        return size, true
    }

    // Try typedef resolution
    moduleTypedefsLock.RLock()
    defer moduleTypedefsLock.RUnlock()

    if typedefs, exists := moduleTypedefs[alias]; exists {
        if resolvedType, found := typedefs[typeName]; found {
            // Recursively resolve
            return getCTypeSize(resolvedType, alias)
        }
    }

    // Unknown type
    return 0, false
}

// transformStringConcatenation converts C string concatenation to Za syntax
// Transforms: "str1" "str2" → "str1" + "str2"
func transformStringConcatenation(s string) string {
    // Pattern: quoted string followed by whitespace and another quoted string
    // Replace whitespace between strings with " + "
    for stringConcatRe.MatchString(s) {
        s = stringConcatRe.ReplaceAllString(s, `"$1" + "`)
    }
    return s
}

// stripCIntegerSuffixes removes C integer and float literal suffixes
// Integer examples: 123L → 123, 0xFFULL → 0xFF, 42ul → 42
// Float examples: 1e10000L → 1e10000, 3.14f → 3.14, 3.40282347e+38F → 3.40282347e+38
func stripCIntegerSuffixes(s string) string {
    // First strip integer literals with suffixes (U/L combinations)
    // Pattern: digit or hex number followed by optional U/L suffixes
    s = intSuffixRe.ReplaceAllString(s, "$1")

    // Then strip float literals with suffixes (f/F/l/L/d/D)
    // Patterns to match:
    // - 1e10000L → 1e10000
    // - 3.14f → 3.14
    // - 3.40282347e+38F → 3.40282347e+38
    // - 1.0e-5l → 1.0e-5
    // Match: digits, optional decimal, optional exponent, followed by f/F/l/L/d/D
    s = floatSuffixRe.ReplaceAllString(s, "$1")

    return s
}

// convertTernaryConditions converts integer literals in ternary conditions to booleans
// Za requires boolean conditions for ternary operators, not integers
// Examples: 0 ? x : y → false ? x : y, 1 ? x : y → true ? x : y
func convertTernaryConditions(expr string) string {
    // Match: <integer> followed by ?
    // Replace the integer with true/false based on C truthiness
    expr = ternaryCondRe.ReplaceAllStringFunc(expr, func(match string) string {
        // Extract the number
        numPart := strings.TrimRight(match, " \t?")

        // Parse as integer
        var num int64
        var err error
        if strings.HasPrefix(numPart, "0x") || strings.HasPrefix(numPart, "0X") {
            num, err = strconv.ParseInt(numPart[2:], 16, 64)
        } else {
            num, err = strconv.ParseInt(numPart, 10, 64)
        }

        if err != nil {
            return match // Keep original if parse fails
        }

        // C semantics: 0 is false, non-zero is true
        if num == 0 {
            return "false ?"
        }
        return "true ?"
    })

    return expr
}

// convertCBooleanOps converts C-style boolean operations to Za-compatible form
// In C: !0 is true, !<nonzero> is false, and integers can be used in boolean contexts
// In Za: ! operator and logical operators require boolean operands
// Examples: !0 → true, 0 && x → false && x, 1 || x → true || x
func convertCBooleanOps(expr string) string {
    // First, replace integers used in logical contexts (after &&, ||, or at start)
    // Only match when the integer is followed by a logical operator or end of expression
    // NOT when followed by comparison operators like >=, <=, >, <, ==, !=
    // or arithmetic operators like +, -, *, /, %
    //
    // Modified regex to not match when ) is followed by comparison/arithmetic operators
    // This prevents matching numbers like 30600 in expressions like ((30600)) > x
    expr = boolContextIntRe.ReplaceAllStringFunc(expr, func(match string) string {
        // Extract parts
        submatch := boolContextIntRe.FindStringSubmatch(match)
        if len(submatch) < 4 {
            return match
        }

        prefix := submatch[1]
        numStr := submatch[2]
        suffix := submatch[3]

        // Parse the number
        var num int64
        var err error
        if strings.HasPrefix(numStr, "0x") || strings.HasPrefix(numStr, "0X") {
            num, err = strconv.ParseInt(numStr[2:], 16, 64)
        } else {
            num, err = strconv.ParseInt(numStr, 10, 64)
        }

        if err != nil {
            return match
        }

        // Convert to boolean
        var boolStr string
        if num == 0 {
            boolStr = "false"
        } else {
            boolStr = "true"
        }

        // Build result with proper spacing
        // Note: "^" and "$" are anchors, not literals
        result := ""
        if prefix != "" {
            result += prefix + " "
        }
        result += boolStr
        if suffix != "" {
            result += " " + suffix
        }
        return result
    })

    // Replace !<integer> with true/false based on C truthiness (handle optional whitespace)
    expr = notIntRe.ReplaceAllStringFunc(expr, func(match string) string {
        // Extract the number (from the first capture group)
        submatch := notIntRe.FindStringSubmatch(match)
        if len(submatch) < 2 {
            return match // Should not happen, but be safe
        }
        numStr := submatch[1]

        // Parse as integer
        var num int64
        var err error
        if strings.HasPrefix(numStr, "0x") || strings.HasPrefix(numStr, "0X") {
            num, err = strconv.ParseInt(numStr[2:], 16, 64)
        } else {
            num, err = strconv.ParseInt(numStr, 10, 64)
        }

        if err != nil {
            return match // Keep original if parse fails
        }

        // C semantics: !0 is true, !<nonzero> is false
        if num == 0 {
            return "true"
        }
        return "false"
    })

    // Replace variables in boolean contexts with itob() wrapper
    // This converts C integer semantics to Za boolean semantics

    // Handle !<variable> → !(itob(<variable>))
    // Skip boolean literals (true, false)
    expr = notVarRe.ReplaceAllStringFunc(expr, func(match string) string {
        submatch := notVarRe.FindStringSubmatch(match)
        if len(submatch) < 2 {
            return match
        }
        varName := submatch[1]
        // Don't wrap boolean literals
        if varName == "true" || varName == "false" {
            return match
        }
        return "!(itob(" + varName + "))"
    })

    // Handle <variable> && ... and ... && <variable>
    // Only wrap if not followed by comparison operators (==, !=, <, >, <=, >=)
    // Skip boolean literals (true, false)
    expr = andVarRe.ReplaceAllStringFunc(expr, func(match string) string {
        submatch := andVarRe.FindStringSubmatch(match)
        if len(submatch) < 4 {
            return match
        }
        prefix := submatch[1]
        varName := submatch[2]
        suffix := submatch[3]
        // Don't wrap boolean literals
        if varName == "true" || varName == "false" {
            return match
        }
        return prefix + " itob(" + varName + ") " + suffix
    })

    // Handle <variable> || ... and ... || <variable>
    expr = orVarRe.ReplaceAllStringFunc(expr, func(match string) string {
        submatch := orVarRe.FindStringSubmatch(match)
        if len(submatch) < 4 {
            return match
        }
        prefix := submatch[1]
        varName := submatch[2]
        suffix := submatch[3]
        // Don't wrap boolean literals
        if varName == "true" || varName == "false" {
            return match
        }
        return prefix + " itob(" + varName + ") " + suffix
    })

    return expr
}

// evaluateSimpleValue attempts to evaluate a simple constant value from a string
// Used for evaluating macro values from moduleMacros when building tempIdent
// Returns (value, true) if successful, (nil, false) otherwise
func evaluateSimpleValue(valueStr string) (any, bool) {
    valueStr = strings.TrimSpace(valueStr)

    // Try parsing as integer
    if intVal, err := strconv.ParseInt(valueStr, 0, 64); err == nil {
        return int(intVal), true
    }

    // Try parsing as float
    if floatVal, err := strconv.ParseFloat(valueStr, 64); err == nil {
        return floatVal, true
    }

    // Try parsing as string literal
    if strings.HasPrefix(valueStr, "\"") && strings.HasSuffix(valueStr, "\"") {
        return valueStr[1 : len(valueStr)-1], true
    }

    // Not a simple constant value
    return nil, false
}

// evaluateMacroLazily evaluates a macro on-demand with cycle detection
// Returns (value, true) if successful, (nil, false) if evaluation fails or cycle detected
func evaluateMacroLazily(name string, alias string, fs uint32, debugAuto bool) (any, bool) {
    // Check if already evaluated
    moduleConstantsLock.RLock()
    if constants, exists := moduleConstants[alias]; exists {
        if val, found := constants[name]; found {
            moduleConstantsLock.RUnlock()
            return val, true
        }
    }
    moduleConstantsLock.RUnlock()

    // Check if macro was marked as skipped (filtered out during parsing)
    // These macros should not be evaluated as they likely reference undefined identifiers
    macroEvalStatusLock.RLock()
    if status, exists := macroEvalStatus[alias]; exists {
        if status[name] == "skipped" {
            macroEvalStatusLock.RUnlock()
            if debugAuto {
                fmt.Printf("[AUTO]   → Skipping lazy evaluation of filtered macro: %s\n", name)
            }
            return nil, false
        }
    }
    macroEvalStatusLock.RUnlock()

    // Check for evaluation cycle
    macroEvaluatingLock.Lock()
    if macroEvaluating[alias] == nil {
        macroEvaluating[alias] = make(map[string]bool)
    }
    if macroEvaluating[alias][name] {
        // Cycle detected
        macroEvaluatingLock.Unlock()
        if debugAuto {
            fmt.Printf("[AUTO]   → Cycle detected evaluating macro %s\n", name)
        }
        return nil, false
    }
    macroEvaluating[alias][name] = true
    macroEvaluatingLock.Unlock()

    // Cleanup cycle guard on exit
    defer func() {
        macroEvaluatingLock.Lock()
        delete(macroEvaluating[alias], name)
        macroEvaluatingLock.Unlock()
    }()

    // Get raw macro value
    moduleMacrosLock.RLock()
    macroValue, exists := moduleMacros[alias][name]
    moduleMacrosLock.RUnlock()

    if !exists {
        return nil, false
    }

    if debugAuto {
        fmt.Printf("[AUTO]   → Lazy evaluating macro %s = %q\n", name, macroValue)
    }

    // Evaluate the macro (this may recursively trigger more lazy evaluations)
    val, ok := evaluateConstant(macroValue, nil, alias, fs, false, nil)
    if !ok {
        return nil, false
    }

    // Store result in moduleConstants
    moduleConstantsLock.Lock()
    if moduleConstants[alias] == nil {
        moduleConstants[alias] = make(map[string]any)
    }
    moduleConstants[alias][name] = val
    moduleConstantsLock.Unlock()

    // Store in cModuleIdents for future reference
    cModuleIdentsLock.Lock()
    cModuleIdents[fs] = append(cModuleIdents[fs], Variable{
        IName:    name,
        IValue:   val,
        declared: true,
    })
    cModuleIdentsLock.Unlock()

    if debugAuto {
        fmt.Printf("[AUTO]   → Lazy evaluated %s = %v (type %T)\n", name, val, val)
    }

    return val, true
}

// evaluateConstant uses Za's expression evaluator to parse #define values
// Handles integers, floats, strings, and constant expressions automatically
// The ident parameter allows constants to reference previously-defined constants
// The fs parameter is the function space from the calling context (passed from Call())
// The alias parameter is the module alias for typedef resolution in sizeof expressions
// The isConditional parameter indicates if this is a #if expression (needs boolean conversions)
func evaluateConstant(valueStr string, state *PreprocessorState, alias string, fs uint32, isConditional bool, cachedTempIdent *[]Variable) (any, bool) {
    debugAuto := os.Getenv("ZA_DEBUG_AUTO") != ""

    // Transform C string concatenation to Za syntax
    valueStr = transformStringConcatenation(valueStr)

    // Strip C integer suffixes (L, LL, U, UL, ULL)
    valueStr = stripCIntegerSuffixes(valueStr)

    // Convert character literals to numeric values (must be done before evaluation)
    // Examples: '\0' → 0, 'A' → 65, L'\0' → 0
    valueStr = convertCharacterLiterals(valueStr)

    // Special case: Detect divide-by-zero expressions that create NaN
    // Patterns: (0.0 / 0.0), (0.0f / 0.0f), etc.
    // These create NaN in C but can't be evaluated as expressions
    // Return math.NaN() directly
    if nanPatternRe.MatchString(valueStr) {
        if debugAuto {
            fmt.Printf("[AUTO]     → Detected NaN expression: %s, returning math.NaN()\n", valueStr)
        }
        return math.NaN(), true
    }

    // Handle sizeof(type) by replacing with actual size EARLY
    // This must be done BEFORE type keyword checking to avoid skipping expressions like "sizeof(int)"

    // First, strip C-style casts before sizeof: (int) sizeof(...) → sizeof(...)
    // This handles cases like (int) sizeof(__fd_mask) or (unsigned) sizeof(long int)
    valueStr = castBeforeSizeofRe.ReplaceAllString(valueStr, "sizeof")

    valueStr = sizeofRe.ReplaceAllStringFunc(valueStr, func(match string) string {
        // Extract type name
        submatch := sizeofRe.FindStringSubmatch(match)
        if len(submatch) < 2 {
            return match // Keep original if parsing fails
        }
        typeName := strings.TrimSpace(submatch[1])

        // First, try to resolve as a macro
        if state != nil {
            if macroValue, found := state.definedMacros[typeName]; found {
                typeName = macroValue
                if debugAuto {
                    fmt.Printf("[AUTO]     → Resolved sizeof macro %s → %s\n", submatch[1], typeName)
                }
            }
        }

        // Get C type size
        if size, ok := getCTypeSize(typeName, alias); ok {
            if debugAuto {
                fmt.Printf("[AUTO]     → Replaced sizeof(%s) with %d\n", typeName, size)
            }
            return strconv.Itoa(size)
        }

        // Unknown type - keep original (will fail later with helpful error)
        if debugAuto {
            fmt.Printf("[AUTO]     → Warning: sizeof(%s) has unknown type, keeping original\n", typeName)
        }
        return match
    })

    // Skip parameter lists like (_Marg_ __x) or (double __x, int __y)
    // These come from macro definitions that represent function parameters, not expressions
    if paramListPatternRe.MatchString(strings.TrimSpace(valueStr)) {
        if debugAuto {
            fmt.Printf("[AUTO]     → Skipping parameter list: %s\n", valueStr)
        }
        return nil, false
    }

    // Check if this is likely a reference to a function-like macro, type declaration, or undefined identifier
    // Examples:
    //   #define __REDIRECT_FORTIFY __REDIRECT  (function-like macro alias)
    //   #define __U_CHAR unsigned char  (type declaration)
    // Skip these as they're not evaluatable constants
    trimmed := strings.TrimSpace(valueStr)
    isSimpleIdentifier := simpleIdentifierRe.MatchString(trimmed)

    // Skip type declarations (contains C type keywords)
    // Note: This check happens AFTER sizeof replacement, so "sizeof(int)" will have been replaced with a number
    if typeKeywordRe.MatchString(trimmed) {
        if debugAuto {
            // Find which keyword matched for debug output
            keywords := []string{"char", "short", "int", "long", "float", "double", "void", "unsigned", "signed", "struct", "union", "enum"}
            for _, keyword := range keywords {
                if strings.Contains(trimmed, keyword) {
                    fmt.Printf("[AUTO]     → Skipping type declaration: %s (contains keyword %s)\n", trimmed, keyword)
                    break
                }
            }
        }
        return nil, false
    }

    // For simple identifiers, check if they're known function-like macros
    // by checking if they start with common prefixes (__) or are all uppercase
    if isSimpleIdentifier && !isConditional {
        // Skip identifiers that look like function-like macros
        // (start with __ or are macro-style names)
        if strings.HasPrefix(trimmed, "__") {
            if debugAuto {
                fmt.Printf("[AUTO]     → Skipping identifier %s (looks like macro)\n", trimmed)
            }
            return nil, false
        }
    }

    // Only apply boolean conversions for conditional expressions (#if), not for constant values
    if isConditional {
        // Check if expression contains ternary operator - if so, treat as false for now
        // TODO: Fix ternary operator support in Za's ev()
        if strings.Contains(valueStr, "?") {
            if debugAuto {
                fmt.Printf("[AUTO]     → Skipping ternary expression: %q, treating as false\n", valueStr)
            }
            return false, true  // Treat as false instead of failing
        }

        // Convert ternary conditions from integers to booleans
        valueStr = convertTernaryConditions(valueStr)

        // Convert C boolean operations to Za-compatible form
        valueStr = convertCBooleanOps(valueStr)

        // Trim any extra whitespace that may have been added
        valueStr = strings.TrimSpace(valueStr)
    }

    // Create parser for ev()
    parser := &leparser{}
    parser.fs = 0  // Use fs=0 for evaluation to avoid bind_int() conflicts
    parser.namespace = "auto_parse"
    parser.ctx = context.Background()
    parser.prectable = default_prectable

    // Build a temporary ident with previously defined C constants
    // Use vset with fs=0 to ensure identifiers are at correct binding positions
    // Add __c_ prefix to avoid conflicts with Za keywords
    //
    // NOTE: If cachedTempIdent is provided, use it instead of rebuilding from scratch
    // This is a critical optimization: without caching, vset() gets called O(macros * constants) times
    var tempIdent []Variable
    var constantNames []string

    if cachedTempIdent != nil {
        // Clone the cached tempIdent to avoid modifications
        tempIdent = make([]Variable, len(*cachedTempIdent))
        copy(tempIdent, *cachedTempIdent)
    } else {
        // Fall-back: build from scratch (used for non-evaluateAllMacros calls)
        tempIdent = make([]Variable, identInitialSize)

        // Copy existing C constants from cModuleIdents[fs] using vset for proper binding
        // Add __c_ prefix to prevent keyword conflicts
        cModuleIdentsLock.RLock()
        constantNames = make([]string, 0)
        if cIdent, exists := cModuleIdents[fs]; exists {
            for _, v := range cIdent {
                if v.declared {
                    vset(nil, 0, &tempIdent, "__c_"+v.IName, v.IValue)
                    constantNames = append(constantNames, v.IName)
                }
            }
        }
        cModuleIdentsLock.RUnlock()

        // Add preprocessor macros if available with __c_ prefix
        if state != nil {
            macros := state.getMacrosAsIdent()
            for _, m := range macros {
                if m.declared {
                    vset(nil, 0, &tempIdent, "__c_"+m.IName, m.IValue)
                }
            }
        } else {
            // When state is nil (e.g., in parseDefines), use moduleConstants directly
            // Only include macros that have already been evaluated to avoid circular dependencies
            moduleConstantsLock.RLock()
            if constants, exists := moduleConstants[alias]; exists {
                for name, value := range constants {
                    vset(nil, 0, &tempIdent, "__c_"+name, value)
                }
            }
            moduleConstantsLock.RUnlock()
        }
    }

    // Transform valueStr to use __c_ prefixed identifiers
    if debugAuto {
        fmt.Printf("[AUTO]     → Before prefix transform: %q\n", valueStr)

        // Show which C constants appear in this expression
        foundConstants := make([]string, 0)
        for _, cname := range constantNames {
            if strings.Contains(valueStr, cname) {
                foundConstants = append(foundConstants, cname)
            }
        }
        if len(foundConstants) > 0 {
            fmt.Printf("[AUTO]     → Expression references C constants: %v\n", foundConstants)
            // Show their values
            cModuleIdentsLock.RLock()
            if cIdent, exists := cModuleIdents[fs]; exists {
                for _, v := range cIdent {
                    for _, fname := range foundConstants {
                        if v.IName == fname && v.declared {
                            fmt.Printf("[AUTO]       %s = %v (type %T)\n", fname, v.IValue, v.IValue)
                        }
                    }
                }
            }
            cModuleIdentsLock.RUnlock()
        }
    }

    valueStr = addCPrefixToIdentifiers(valueStr)
    if debugAuto {
        fmt.Printf("[AUTO]     → After prefix transform: %q\n", valueStr)
    }

    // Add true/false for conditional evaluation
    if isConditional {
        vset(nil, 0, &tempIdent, "true", true)
        vset(nil, 0, &tempIdent, "false", false)
    }

    if debugAuto {
        fmt.Printf("[AUTO]     → Using tempIdent with length %d (from cModuleIdents[%d])\n", len(tempIdent), fs)
        // Show if the constants we need are in tempIdent
        if strings.Contains(valueStr, "_SS_SIZE") || strings.Contains(valueStr, "SOCKADDR_COMMON") {
            for _, v := range tempIdent {
                if strings.Contains(v.IName, "_SS_SIZE") || strings.Contains(v.IName, "SOCKADDR_COMMON") {
                    fmt.Printf("[AUTO]       tempIdent has: %s = %v (type %T)\n", v.IName, v.IValue, v.IValue)
                }
            }
        }
        fmt.Printf("[AUTO]     → About to call ev(parser, fs=0, valueStr=%q)\n", valueStr)
    }

    // Check if expression contains __c_ prefixed identifiers that don't exist in tempIdent
    // This prevents ev() from hanging when trying to evaluate undefined identifiers
    referencedIdents := cIdentPatternRe.FindAllString(valueStr, -1)
    if len(referencedIdents) > 0 {
        for _, ident := range referencedIdents {
            found := false
            for _, v := range tempIdent {
                if v.IName == ident && v.declared {
                    found = true
                    break
                }
            }
            if !found {
                if debugAuto {
                    fmt.Printf("[AUTO]     → Expression references undefined identifier %s, skipping evaluation\n", ident)
                }
                return nil, false
            }
        }
    }

    parser.ident = &tempIdent

    // Evaluate using fs=0 (global) to avoid polluting C module bindings
    result, err := ev(parser, 0, valueStr)

    if debugAuto {
        fmt.Printf("[AUTO]     → ev() returned: result=%v, err=%v\n", result, err)
    }

    if err != nil {
        if debugAuto {
            fmt.Printf("[AUTO]     → ev() error for expression %q: %v\n", valueStr, err)
            // Report eval errors only in debug mode
            fmt.Fprintf(os.Stderr, "Error evaluating '%s'\n", valueStr)
        }
        return nil, false
    }

    return result, true
}

// parseEnums extracts enum definitions from header text
func parseEnums(text string, alias string, fs uint32) error {
    // Match: enum Name { ... } or enum { ... }
    // Handle both single-line and multiline

    // Pattern: enum (name)? { members }
    re := regexp.MustCompile(`(?s)enum\s+([A-Za-z_][A-Za-z0-9_]*)?\s*\{([^}]+)\}`)
    matches := re.FindAllStringSubmatch(text, -1)

    globlock.Lock()
    defer globlock.Unlock()

    for idx, match := range matches {
        enumName := match[1]
        if enumName == "" {
            enumName = fmt.Sprintf("anon_enum_%d", idx)
        }

        fullName := alias + "::" + enumName
        members := match[2]

        // Parse members: NAME, NAME = VALUE, NAME = 0x123
        memberLines := strings.Split(members, ",")

        enum[fullName] = &enum_s{
            members:   make(map[string]any),
            ordered:   []string{},
            namespace: alias,
        }

        currentValue := 0
        for _, line := range memberLines {
            line = strings.TrimSpace(line)
            if line == "" {
                continue
            }

            // Check for explicit value: NAME = VALUE
            if strings.Contains(line, "=") {
                parts := strings.SplitN(line, "=", 2)
                memberName := strings.TrimSpace(parts[0])
                valueStr := strings.TrimSpace(parts[1])

                // For enum values, we expect integers
                if val, ok := evaluateConstant(valueStr, nil, alias, fs, false, nil); ok { // enum value
                    // Accept any numeric type (int, int64, float64) and convert to int
                    intVal := 0
                    switch v := val.(type) {
                    case int:
                        intVal = v
                    case int64:
                        intVal = int(v)
                    case float64:
                        intVal = int(v)
                    default:
                        continue // Skip non-numeric enum values
                    }
                    enum[fullName].members[memberName] = intVal
                    enum[fullName].ordered = append(enum[fullName].ordered, memberName)
                    currentValue = intVal + 1

                    // Add to cModuleIdents so later enum values can reference it
                    cModuleIdentsLock.Lock()
                    cModuleIdents[fs] = append(cModuleIdents[fs], Variable{
                        IName:    memberName,
                        IValue:   intVal,
                        declared: true,
                    })
                    cModuleIdentsLock.Unlock()
                }
            } else {
                // Auto-increment
                memberName := line
                enum[fullName].members[memberName] = currentValue
                enum[fullName].ordered = append(enum[fullName].ordered, memberName)

                // Add to cModuleIdents so later enum values can reference it
                cModuleIdentsLock.Lock()
                cModuleIdents[fs] = append(cModuleIdents[fs], Variable{
                    IName:    memberName,
                    IValue:   currentValue,
                    declared: true,
                })
                cModuleIdentsLock.Unlock()

                currentValue++
            }
        }

        // Check if we should abort (interactive mode error signaled via sig_int)
        if sig_int {
            return fmt.Errorf("AUTO import aborted during enum parsing")
        }
    }

    return nil
}

// normalizeFunctionDeclarations preprocesses header text to convert multiline
// function declarations into single lines for easier regex matching
func normalizeFunctionDeclarations(text string) string {
    lines := strings.Split(text, "\n")
    var normalized []string
    inDeclaration := false
    var buffer string

    for _, line := range lines {
        trimmed := strings.TrimSpace(line)

        // Skip empty lines and preprocessor directives
        if trimmed == "" || strings.HasPrefix(trimmed, "#") {
            if !inDeclaration {
                normalized = append(normalized, line)
            }
            continue
        }

        if !inDeclaration {
            // Check if this line might be the start of a function declaration
            // It could be:
            // 1. Contains '(' and doesn't end with ';' or '{' (incomplete declaration)
            // 2. Looks like a type declaration (extern, const, type name) but no '('
            //    (might be followed by function name on next line)

            if strings.Contains(trimmed, "(") {
                if !strings.HasSuffix(trimmed, ";") && !strings.HasSuffix(trimmed, "{") {
                    // Incomplete declaration - start buffering
                    inDeclaration = true
                    buffer = trimmed + " "
                } else {
                    // Complete single-line declaration
                    normalized = append(normalized, line)
                }
            } else if strings.HasPrefix(trimmed, "extern ") ||
                       strings.HasPrefix(trimmed, "const ") ||
                       (len(trimmed) > 0 && !strings.Contains(trimmed, "{") && !strings.Contains(trimmed, "}") &&
                        !strings.HasPrefix(trimmed, "typedef ") && !strings.HasSuffix(trimmed, ";")) {
                // Might be a return type line for multiline declaration
                // Start buffering tentatively (but exclude typedef and complete statements ending with ;)
                inDeclaration = true
                buffer = trimmed + " "
            } else {
                normalized = append(normalized, line)
            }
        } else {
            // We're in a declaration - keep buffering until we find the end
            if strings.Contains(trimmed, ")") {
                buffer += trimmed + " "
                // Check if this line also contains the final semicolon
                if strings.Contains(trimmed, ";") {
                    // Declaration complete
                    normalized = append(normalized, buffer)
                    buffer = ""
                    inDeclaration = false
                }
                // Otherwise, continue buffering (attributes follow on next lines)
            } else {
                buffer += trimmed + " "
            }
        }
    }

    // If we have leftover buffer (incomplete declaration), don't include it
    // (it's likely not a valid function declaration)

    return strings.Join(normalized, "\n")
}

// parseTypedefs extracts typedef declarations from header text
// and stores them in the global typedef registry for later resolution
func parseTypedefs(text string, alias string) error {
    moduleTypedefsLock.Lock()
    defer moduleTypedefsLock.Unlock()

    if moduleTypedefs[alias] == nil {
        moduleTypedefs[alias] = make(map[string]string)
    }

    if os.Getenv("ZA_DEBUG_AUTO") != "" {
        fmt.Printf("[AUTO] parseTypedefs START for alias=%s (text length=%d)\n", alias, len(text))
        if strings.Contains(text, "GLFWframebuffersizefun") {
            fmt.Printf("[AUTO] *** Text CONTAINS GLFWframebuffersizefun typedef\n")
            // Find and show the context
            idx := strings.Index(text, "GLFWframebuffersizefun")
            start := idx - 50
            if start < 0 {
                start = 0
            }
            end := idx + 100
            if end > len(text) {
                end = len(text)
            }
            fmt.Printf("[AUTO] Context: %q\n", text[start:end])
        }
    }

    // Pattern 1: Function pointer typedefs FIRST
    // typedef int (*callback_t)(void*);
    // typedef int (*binary_op_t)(int, int);
    reFuncPtr := regexp.MustCompile(`typedef\s+(\S+\s+\(\*\s*(\w+)\s*\)[^;]*);`)

    funcPtrMatches := reFuncPtr.FindAllStringSubmatch(text, -1)

    debugAuto := os.Getenv("ZA_DEBUG_AUTO") != ""
    if debugAuto && strings.Contains(text, "GLFWframebuffersizefun") {
        fmt.Printf("[AUTO] Found %d function pointer typedef matches (text contains GLFWframebuffersizefun)\n", len(funcPtrMatches))
        // List the extracted typedef names
        for i, match := range funcPtrMatches {
            if i < 5 {
                fmt.Printf("[AUTO]   Match %d: newName='%s'\n", i, match[2])
            }
        }
    }

    for _, match := range funcPtrMatches {
        baseType := strings.TrimSpace(match[1])
        newName := match[2]

        if os.Getenv("ZA_DEBUG_AUTO") != "" {
            if strings.Contains(newName, "Framebuffer") {
                fmt.Printf("[AUTO] *** Found Framebuffer typedef: newName='%s'\n", newName)
            }
        }

        // Parse the function pointer signature
        _, sig, err := parseFunctionPointerSignature(baseType)
        if err != nil {
            if os.Getenv("ZA_WARN_AUTO") != "" {
                msg := fmt.Sprintf("[AUTO] Warning: Failed to parse function pointer typedef %s: %v", newName, err)
                addMessageToCurrentProgress(msg)
            }
            continue
        }
        // Store in function pointer registry
        moduleFunctionPointerSignaturesLock.Lock()
        if moduleFunctionPointerSignatures[alias] == nil {
            moduleFunctionPointerSignatures[alias] = make(map[string]CFunctionSignature)
        }
        moduleFunctionPointerSignatures[alias][newName] = sig
        if os.Getenv("ZA_DEBUG_AUTO") != "" && strings.Contains(newName, "Framebuffer") {
            fmt.Printf("[AUTO] Stored function pointer typedef: alias='%s', newName='%s'\n", alias, newName)
        }
        moduleFunctionPointerSignaturesLock.Unlock()
    }

    // Pattern 2: Simple typedef
    // typedef unsigned int uint32_t;
    // typedef const char* string_t;
    // Handle multiline typedefs by collapsing newlines around *
    // e.g., "typedef struct _XGC\n*GC;" becomes "typedef struct _XGC *GC;"
    normalizedText := regexp.MustCompile(`\n\s*\*`).ReplaceAllString(text, " *")

    // Match typedef patterns - use \s* instead of \s+ to handle cases where
    // the pointer star is directly followed by identifier (e.g., "struct _XGC*GC")
    reSimple := regexp.MustCompile(`typedef\s+([^;]+?)\s*(\w+)\s*;`)

    matches := reSimple.FindAllStringSubmatch(normalizedText, -1)

    // Match and store
    for _, match := range matches {
        baseType := strings.TrimSpace(match[1])
        newName := match[2]

        // Skip function pointer typedefs - they were already handled above
        // These have (* in them
        if strings.Contains(baseType, "(*") {
            continue
        }

        // Skip function-like typedefs without pointers
        // These are complex macros or forward declarations
        if strings.Contains(baseType, "(") && !strings.Contains(baseType, "*") {
            continue
        }

        // Skip struct/union definitions in typedef (keep simple form)
        // typedef struct { int x; } Point; is OK
        // typedef struct Point Point; is OK
        // But skip: typedef struct { struct { int x; } nested; } Complex;
        // Also skip: partial matches like "union { int" (unmatched braces)
        if strings.Contains(baseType, "struct") || strings.Contains(baseType, "union") {
            // Count braces to detect incomplete or nested struct/union definitions
            openBraces := strings.Count(baseType, "{")
            closeBraces := strings.Count(baseType, "}")
            if openBraces != closeBraces || openBraces > 1 {
                // Unmatched braces or nested struct/union - skip
                if debugAuto && newName == "GC" {
                    fmt.Printf("[AUTO] SKIPPING typedef %s (unmatched braces: %d open, %d close)\n", newName, openBraces, closeBraces)
                }
                continue
            }
        }

        moduleTypedefs[alias][newName] = baseType

        if debugAuto {
            if newName == "GC" || strings.Contains(newName, "GLFW") || strings.Contains(newName, "GC") {
                fmt.Printf("[AUTO] Typedef: %s → %s\n", newName, baseType)
            }
        }
    }

    return nil
}

// resolveTypedef recursively resolves a typedef name to its base type
// Returns empty string if the type is not a typedef
// Respects the use chain - if not found in the specified alias, searches through used libraries
func resolveTypedef(typeName string, alias string, depth int) string {
    // Prevent infinite recursion
    if depth > 10 {
        return ""
    }

    // Strip qualifiers and keywords
    cleanType := strings.TrimSpace(typeName)
    cleanType = strings.TrimPrefix(cleanType, "const ")
    cleanType = strings.TrimPrefix(cleanType, "volatile ")
    cleanType = strings.TrimPrefix(cleanType, "restrict ")
    cleanType = strings.TrimPrefix(cleanType, "struct ")
    cleanType = strings.TrimPrefix(cleanType, "union ")
    cleanType = strings.TrimPrefix(cleanType, "enum ")
    cleanType = strings.TrimSpace(cleanType)

    // First, try the specified alias
    moduleTypedefsLock.RLock()
    if moduleTypedefs[alias] != nil {
        if baseType, exists := moduleTypedefs[alias][cleanType]; exists {
            moduleTypedefsLock.RUnlock()
            // Recursively resolve
            if resolved := resolveTypedef(baseType, alias, depth+1); resolved != "" {
                return resolved
            }
            return baseType
        }
    }
    moduleTypedefsLock.RUnlock()

    // Not found in the specified alias - search through the use chain
    // using uc_match_typedef to find which library has this typedef
    if resolverAlias := uc_match_typedef(cleanType); resolverAlias != "" && resolverAlias != alias {
        moduleTypedefsLock.RLock()
        baseType, exists := moduleTypedefs[resolverAlias][cleanType]
        moduleTypedefsLock.RUnlock()

        if exists {
            // Found it in a used library
            if resolved := resolveTypedef(baseType, resolverAlias, depth+1); resolved != "" {
                return resolved
            }
            return baseType
        }
    }

    return ""
}

// parseFunctionPointerSignature parses a function pointer typedef
// Format: "returnType (*name)(param1, param2, ...)"
// Example: "int (*compare_t)(const void*, const void*)"
// Returns: (name, signature, error)
func parseFunctionPointerSignature(decl string) (string, CFunctionSignature, error) {
    // Trim whitespace
    decl = strings.TrimSpace(decl)

    // Find the opening paren containing the *
    startIdx := strings.Index(decl, "(*")
    if startIdx == -1 {
        return "", CFunctionSignature{}, fmt.Errorf("invalid function pointer format")
    }

    // Extract return type (everything before "(*")
    returnTypeStr := strings.TrimSpace(decl[:startIdx])

    // Find the closing paren of the name
    endIdx := strings.Index(decl[startIdx:], ")")
    if endIdx == -1 {
        return "", CFunctionSignature{}, fmt.Errorf("unclosed parenthesis in function pointer name")
    }
    endIdx += startIdx

    // Extract name (between * and the closing paren)
    name := strings.TrimSpace(decl[startIdx+2 : endIdx])

    // Find the parameter list (opening paren after the name paren)
    paramStart := strings.Index(decl[endIdx:], "(")
    if paramStart == -1 {
        return "", CFunctionSignature{}, fmt.Errorf("no parameter list in function pointer")
    }
    paramStart += endIdx

    paramEnd := strings.LastIndex(decl, ")")
    if paramEnd <= paramStart {
        return "", CFunctionSignature{}, fmt.Errorf("unclosed parameter list in function pointer")
    }

    // Extract parameter string
    paramStr := strings.TrimSpace(decl[paramStart+1 : paramEnd])

    // Parse return type
    returnType, returnStructName, err := StringToCType(returnTypeStr)
    if err != nil {
        return "", CFunctionSignature{}, fmt.Errorf("invalid return type in function pointer: %w", err)
    }

    // Parse parameters
    var paramTypes []CType
    var paramStructNames []string

    if paramStr != "" && paramStr != "void" {
        // Split parameters by comma (but be careful about nested types)
        params := strings.Split(paramStr, ",")
        for _, param := range params {
            param = strings.TrimSpace(param)
            // Remove type qualifiers
            param = strings.TrimPrefix(param, "const ")
            param = strings.TrimPrefix(param, "volatile ")
            param = strings.TrimPrefix(param, "restrict ")
            param = strings.TrimSpace(param)

            // Handle "..." for variadic parameters
            if param == "..." {
                // For now, we don't support variadic function pointers fully
                continue
            }

            // Remove parameter names (if present)
            // E.g., "int a" -> "int", "const char *str" -> "const char *"
            parts := strings.Fields(param)
            if len(parts) > 1 {
                param = strings.Join(parts[:len(parts)-1], " ")
            }

            ptype, pstruct, err := StringToCType(param)
            if err != nil {
                return "", CFunctionSignature{}, fmt.Errorf("invalid parameter type '%s': %w", param, err)
            }
            paramTypes = append(paramTypes, ptype)
            paramStructNames = append(paramStructNames, pstruct)
        }
    }

    sig := CFunctionSignature{
        ParamTypes:       paramTypes,
        ParamStructNames: paramStructNames,
        ReturnType:       returnType,
        ReturnStructName: returnStructName,
        HasVarargs:       false,
        FixedArgCount:    len(paramTypes),
    }

    return name, sig, nil
}

// extractUnionTypedefMatches extracts union typedef declarations using brace-counting
// to handle nested braces (like inline unions) correctly
// Returns: (fieldBlock, unionName) pairs
func extractUnionTypedefMatches(text string) []struct{ fieldBlock, unionName string } {
    var matches []struct{ fieldBlock, unionName string }

    // Find all "typedef union" occurrences
    pattern := regexp.MustCompile(`typedef\s+union\s+(?:[A-Za-z_][A-Za-z0-9_]*)?\s*\{`)
    positions := pattern.FindAllStringIndex(text, -1)

    for _, pos := range positions {
        braceStart := pos[1] - 1 // Position of the opening brace

        // Find matching closing brace using brace counting
        braceCount := 0
        braceEnd := -1
        for i := braceStart; i < len(text); i++ {
            if text[i] == '{' {
                braceCount++
            } else if text[i] == '}' {
                braceCount--
                if braceCount == 0 {
                    braceEnd = i
                    break
                }
            }
        }

        if braceEnd == -1 {
            // No matching brace found
            continue
        }

        // Extract field block content (between braces)
        fieldBlock := text[braceStart+1 : braceEnd]

        // Find union name after the closing brace
        afterBrace := text[braceEnd+1:]
        unionNameMatch := regexp.MustCompile(`^\s*([A-Za-z_][A-Za-z0-9_]*)\s*;`).FindStringSubmatch(afterBrace)
        if len(unionNameMatch) < 2 {
            continue
        }
        unionName := unionNameMatch[1]

        matches = append(matches, struct{ fieldBlock, unionName string }{fieldBlock, unionName})
    }

    return matches
}

// parseUnionTypedefs extracts union typedef declarations from header text
// and stores them in the FFI struct registry with IsUnion=true
func parseUnionTypedefs(text string, alias string) error {
    // Pattern to match: typedef union { fields } name;
    // Also handles: typedef union name { fields } name;
    // Matches both multiline and single-line declarations
    // Uses brace-counting to handle nested braces (inline unions)

    debugAuto := os.Getenv("ZA_DEBUG_AUTO") != ""

    // Use brace-counting extraction instead of regex
    matches := extractUnionTypedefMatches(text)

    for _, match := range matches {
        fieldBlock := match.fieldBlock
        unionName := match.unionName

        if debugAuto {
            fmt.Printf("[AUTO] Found union typedef: %s\n", unionName)
        }

        // Parse fields from the field block
        fields, maxSize, err := parseUnionFields(fieldBlock, alias, unionName, debugAuto)
        if err != nil {
            // Production warning - always visible
            errMsg := fmt.Sprintf("skipped union %s: %v", unionName, err)
            msg := fmt.Sprintf("[AUTO] Warning: %s", errMsg)
            addMessageToCurrentProgress(msg)

            // Track error for programmatic access
            autoImportErrorsLock.Lock()
            autoImportErrors[alias] = append(autoImportErrors[alias], errMsg)
            autoImportErrorsLock.Unlock()

            if debugAuto {
                fmt.Printf("[AUTO] Debug: union %s parse error details: %v\n", unionName, err)
            }
            continue // Skip this union but continue parsing others
        }

        // Create CLibraryStruct for the union
        unionStruct := &CLibraryStruct{
            Name:    unionName,
            Fields:  fields,
            Size:    maxSize,
            IsUnion: true,
        }

        // Store in FFI struct registry (from lib-c.go)
        ffiStructLock.Lock()
        fullName := alias + "::" + unionName
        ffiStructDefinitions[fullName] = unionStruct
        ffiStructLock.Unlock()

        // ALSO register as typed Za struct (makes AUTO unions available in Za code)
        registerStructInZa(alias, unionName, unionStruct)

        if debugAuto {
            fmt.Printf("[AUTO] Registered union %s (size: %d bytes, %d fields)\n",
                unionName, maxSize, len(fields))
        }

        // Check if we should abort (interactive mode error signaled via sig_int)
        if sig_int {
            return fmt.Errorf("AUTO import aborted during union parsing")
        }
    }

    return nil
}

// parseUnionFields parses the field declarations inside a union definition
// Returns the fields, the max size, and any error
func parseUnionFields(fieldBlock string, alias string, unionName string, debug bool) ([]StructField, uintptr, error) {
    var fields []StructField
    var maxSize uintptr = 0

    // Split by semicolons to get individual field declarations
    declarations := splitFieldDeclarations(fieldBlock)

    for _, decl := range declarations {
        decl = strings.TrimSpace(decl)
        if decl == "" {
            continue
        }

        // Handle function pointer fields
        if isFunctionPointerField(decl) {
            fieldName, sig, err := parseFunctionPointerField(decl)
            if err != nil {
                return nil, 0, fmt.Errorf("failed to parse function pointer field: %w", err)
            }

            // Create function pointer field (all union fields at offset 0)
            field := StructField{
                Name:              fieldName,
                Type:              CPointer, // Function pointers are pointers
                Offset:            0,  // All union fields at offset 0
                IsFunctionPtr:     true,
                FunctionSignature: &sig,
            }

            fields = append(fields, field)
            // Function pointers are sizeof(void*) on all platforms
            ptrSize := unsafe.Sizeof(unsafe.Pointer(nil))
            if ptrSize > maxSize {
                maxSize = ptrSize
            }

            if debug && os.Getenv("ZA_DEBUG_AUTO") != "" {
                fmt.Printf("[AUTO] Function pointer field: %s at offset %d\n", fieldName, field.Offset)
            }
            continue
        }

        // Check for inline struct/union definitions
        if isNestedStructOrUnion(decl) {
            // Parse the inline union/struct
            fieldName, inlineUnionDef, inlineSize, err := parseInlineUnion(decl, alias, debug)
            if err != nil {
                return nil, 0, fmt.Errorf("failed to parse inline union in field: %v", err)
            }

            // Create field with inline union (all union fields have offset 0)
            field := StructField{
                Name:     fieldName,
                Type:     CStruct, // Treat as struct type for now
                Offset:   0,  // All union fields at offset 0
                IsUnion:  inlineUnionDef.IsUnion,    // ← POPULATE THIS FIELD!
                UnionDef: inlineUnionDef,             // ← POPULATE THIS FIELD!
            }

            fields = append(fields, field)

            // Update max size (for unions, all fields are the same size at offset 0)
            if inlineSize > maxSize {
                maxSize = inlineSize
            }

            if debug {
                unionOrStruct := "struct"
                if inlineUnionDef.IsUnion {
                    unionOrStruct = "union"
                }
                fmt.Printf("[AUTO]   Inline %s field: %s (size: %d bytes, offset: 0)\n",
                    unionOrStruct, fieldName, inlineSize)
            }

            continue  // Move to next field declaration
        }

        // Fail if field is a bitfield
        if isBitfieldDeclaration(decl) {
            return nil, 0, fmt.Errorf("contains bitfield (not supported): %s", decl)
        }

        // Fail if field declaration is malformed
        if !isValidFieldDeclaration(decl) {
            return nil, 0, fmt.Errorf("contains malformed field declaration: %s", decl)
        }

        // Parse field declaration: type field_name or type field_name[size]
        // Examples: "int x", "float values[4]", "unsigned char bytes[16]"

        // Handle array fields: type name[size]
        var fieldName string
        var fieldType CType
        var fieldSize uintptr
        var arraySize int = 0
        var elementType CType

        if strings.Contains(decl, "[") && strings.HasSuffix(decl, "]") {
            // Array field
            openBracket := strings.Index(decl, "[")
            closeBracket := strings.LastIndex(decl, "]")

            if openBracket > 0 && closeBracket > openBracket {
                // Extract size
                arraySizeStr := strings.TrimSpace(decl[openBracket+1 : closeBracket])

                // Try to resolve as a constant macro first
                resolved := false
                moduleConstantsLock.RLock()
                if constants, exists := moduleConstants[alias]; exists {
                    if val, found := constants[arraySizeStr]; found {
                        switch v := val.(type) {
                        case int:
                            arraySize = v
                            resolved = true
                        case int64:
                            arraySize = int(v)
                            resolved = true
                        case float64:
                            arraySize = int(v)
                            resolved = true
                        }
                    }
                }
                moduleConstantsLock.RUnlock()

                // Fallback to literal integer or expression evaluation
                if !resolved {
                    size, err := strconv.Atoi(arraySizeStr)
                    if err != nil {
                        // Try to evaluate as an expression (e.g., "1024 / 64" or "__FD_SETSIZE / __NFDBITS")
                        if val, ok := evaluateConstant(arraySizeStr, nil, alias, 0, false, nil); ok {
                            switch v := val.(type) {
                            case int:
                                arraySize = v
                                resolved = true
                            case int64:
                                arraySize = int(v)
                                resolved = true
                            case float64:
                                arraySize = int(v)
                                resolved = true
                            }
                        }
                        if !resolved {
                            return nil, 0, fmt.Errorf("contains array field with invalid size: %s", decl)
                        }
                    } else {
                        arraySize = size
                    }
                }

                // Extract type and name before [
                beforeBracket := strings.TrimSpace(decl[:openBracket])
                parts := strings.Fields(beforeBracket)
                if len(parts) < 2 {
                    return nil, 0, fmt.Errorf("contains invalid array field declaration: %s", decl)
                }

                // Last part is field name, everything else is type
                fieldName = parts[len(parts)-1]
                typeStr := strings.Join(parts[:len(parts)-1], " ")

                // Parse element type
                elemType, elemSize := parseCTypeString(typeStr, alias)
                if elemType == CVoid {
                    return nil, 0, fmt.Errorf("contains array field with unsupported element type '%s': %s", typeStr, decl)
                }

                elementType = elemType
                fieldType = elemType // For arrays, store element type
                fieldSize = elemSize * uintptr(arraySize)
            }
        } else {
            // Regular field (non-array)
            parts := strings.Fields(decl)
            if len(parts) < 2 {
                return nil, 0, fmt.Errorf("contains invalid field declaration: %s", decl)
            }

            // Last part is field name, everything else is type
            fieldName = parts[len(parts)-1]
            typeStr := strings.Join(parts[:len(parts)-1], " ")

            // Parse type
            fType, fSize := parseCTypeString(typeStr, alias)
            if fType == CVoid {
                // Check if this is a struct/union type (same as parseStructFields logic)
                var nestedStruct *CLibraryStruct
                possibleNames := []string{typeStr, alias + "::" + typeStr}
                for _, name := range possibleNames {
                    ffiStructLock.RLock()
                    if structDef, exists := ffiStructDefinitions[name]; exists {
                        nestedStruct = structDef
                        ffiStructLock.RUnlock()
                        break
                    }
                    ffiStructLock.RUnlock()
                }

                if nestedStruct != nil {
                    // It's a nested struct/union
                    fieldType = CStruct
                    fieldSize = nestedStruct.Size
                    // Store the struct definition for marshaling/unmarshaling
                    field := StructField{
                        Name:       fieldName,
                        Type:       CStruct,
                        Offset:     0,  // All union fields at offset 0
                        StructName: typeStr,
                        StructDef:  nestedStruct,
                    }
                    fields = append(fields, field)
                    if nestedStruct.Size > maxSize {
                        maxSize = nestedStruct.Size
                    }

                    if debug {
                        fmt.Printf("[AUTO]   Field: CStruct %s (nested %s, size: %d bytes, offset: 0)\n",
                            fieldName, typeStr, nestedStruct.Size)
                    }
                    continue  // Skip the normal field append below
                } else {
                    // Try heuristic: is it likely an opaque pointer?
                    if isLikelyOpaquePointer(typeStr) {
                        fieldType = CPointer
                        fieldSize = 8  // Pointer size on x86-64
                        if debug {
                            fmt.Printf("[AUTO] Treating unknown type %s as opaque pointer\n", typeStr)
                        }
                    } else {
                        // Cannot resolve type - fail union parsing
                        return nil, 0, fmt.Errorf("contains unresolved type '%s' in field: %s", typeStr, decl)
                    }
                }
            }

            fieldType = fType
            fieldSize = fSize
        }

        // All union fields have offset 0 (they overlap)
        field := StructField{
            Name:        fieldName,
            Type:        fieldType,
            Offset:      0, // All union fields start at offset 0
            ArraySize:   arraySize,
            ElementType: elementType,
        }

        fields = append(fields, field)

        // Update max size
        if fieldSize > maxSize {
            maxSize = fieldSize
        }

        if debug {
            if arraySize > 0 {
                fmt.Printf("[AUTO]   Field: %+v %s[%d] (size: %d bytes, offset: 0)\n",
                    fieldType, fieldName, arraySize, fieldSize)
            } else {
                fmt.Printf("[AUTO]   Field: %+v %s (size: %d bytes, offset: 0)\n",
                    fieldType, fieldName, fieldSize)
            }
        }

        // Check if we should abort (interactive mode error signaled via sig_int)
        if sig_int {
            return nil, 0, fmt.Errorf("AUTO import aborted during union field parsing")
        }
    }

    return fields, maxSize, nil
}

// extractStructTypedefMatches extracts struct typedef declarations using brace-counting
// to handle nested braces (like inline unions) correctly
// Returns: (fieldBlock, structName) pairs
func extractStructTypedefMatches(text string) []struct{ fieldBlock, structName string } {
    var matches []struct{ fieldBlock, structName string }

    // Find all "typedef struct" occurrences
    pattern := regexp.MustCompile(`typedef\s+struct\s+(?:[A-Za-z_][A-Za-z0-9_]*)?\s*\{`)
    positions := pattern.FindAllStringIndex(text, -1)

    for _, pos := range positions {
        braceStart := pos[1] - 1 // Position of the opening brace

        // Find matching closing brace using brace counting
        braceCount := 0
        braceEnd := -1
        for i := braceStart; i < len(text); i++ {
            if text[i] == '{' {
                braceCount++
            } else if text[i] == '}' {
                braceCount--
                if braceCount == 0 {
                    braceEnd = i
                    break
                }
            }
        }

        if braceEnd == -1 {
            // No matching brace found
            continue
        }

        // Extract field block content (between braces)
        fieldBlock := text[braceStart+1 : braceEnd]

        // Find struct name after the closing brace
        afterBrace := text[braceEnd+1:]
        structNameMatch := regexp.MustCompile(`^\s*([A-Za-z_][A-Za-z0-9_]*)\s*;`).FindStringSubmatch(afterBrace)
        if len(structNameMatch) < 2 {
            continue
        }
        structName := structNameMatch[1]

        matches = append(matches, struct{ fieldBlock, structName string }{fieldBlock, structName})
    }

    return matches
}

// parseStructTypedefs extracts struct typedef declarations from header text
// and stores them in the FFI struct registry with IsUnion=false
func parseStructTypedefs(text string, alias string) error {
    // Pattern to match: typedef struct { fields } name;
    // Also handles: typedef struct name { fields } name;
    // Matches both multiline and single-line declarations
    // Uses brace-counting to handle nested braces (inline unions)

    debugAuto := os.Getenv("ZA_DEBUG_AUTO") != ""
    if debugAuto {
        fmt.Printf("[AUTO] parseStructTypedefs called for alias=%s, text length=%d\n", alias, len(text))
        if strings.Contains(text, "struct stat") {
            fmt.Printf("[AUTO] DEBUG parseStructTypedefs: text contains 'struct stat'\n")
        }
    }

    // Use brace-counting extraction instead of regex
    matches := extractStructTypedefMatches(text)

    for _, match := range matches {
        fieldBlock := match.fieldBlock
        structName := match.structName

        if debugAuto {
            fmt.Printf("[AUTO] Found struct typedef: %s\n", structName)
        }

        // Parse fields from the field block
        fields, totalSize, err := parseStructFields(fieldBlock, alias, structName, debugAuto)
        if err != nil {
            // Production warning - always visible
            errMsg := fmt.Sprintf("skipped struct %s: %v", structName, err)
            msg := fmt.Sprintf("[AUTO] Warning: %s", errMsg)
            addMessageToCurrentProgress(msg)

            // Track error for programmatic access
            autoImportErrorsLock.Lock()
            autoImportErrors[alias] = append(autoImportErrors[alias], errMsg)
            autoImportErrorsLock.Unlock()

            if debugAuto {
                fmt.Printf("[AUTO] Debug: struct %s parse error details: %v\n", structName, err)
            }
            continue // Skip this struct but continue parsing others
        }

        // Create CLibraryStruct for the struct
        structDef := &CLibraryStruct{
            Name:    structName,
            Fields:  fields,
            Size:    totalSize,
            IsUnion: false,
        }

        // Store in FFI struct registry (from lib-c.go)
        ffiStructLock.Lock()
        fullName := alias + "::" + structName
        ffiStructDefinitions[fullName] = structDef
        if debugAuto {
            fmt.Printf("[AUTO] Stored struct typedef in ffiStructDefinitions: fullName=%s, size=%d\n", fullName, totalSize)
        }
        ffiStructLock.Unlock()

        // ALSO register as typed Za struct (makes AUTO structs available in Za code)
        registerStructInZa(alias, structName, structDef)

        if debugAuto {
            fmt.Printf("[AUTO] Registered struct %s (size: %d bytes, %d fields)\n",
                structName, totalSize, len(fields))
        }

        // Check if we should abort (interactive mode error signaled via sig_int)
        if sig_int {
            return fmt.Errorf("AUTO import aborted during struct parsing")
        }
    }

    return nil
}

// removeConditionalDirectives removes #ifdef, #else, #endif, and #include directives from text
// Used to clean struct field blocks before parsing
func removeConditionalDirectives(text string) string {
    lines := strings.Split(text, "\n")
    var cleaned []string

    for _, line := range lines {
        trimmed := strings.TrimSpace(line)
        // Skip empty lines and conditional directives
        if trimmed == "" {
            cleaned = append(cleaned, "") // Preserve empty lines for structure
            continue
        }

        // Check if line starts with # directive (may have leading spaces)
        if strings.HasPrefix(trimmed, "#") {
            // Skip any # directive
            continue
        }

        // Keep non-directive lines
        cleaned = append(cleaned, line)
    }

    return strings.Join(cleaned, "\n")
}

// parseIncludedHeaders reads #include directives and parses included files for struct definitions
func parseIncludedHeaders(text string, alias string, currentFile string) {
    debugAuto := os.Getenv("ZA_DEBUG_AUTO") != ""

    if debugAuto {
        fmt.Printf("[AUTO] parseIncludedHeaders: scanning for #include directives in %s\n", currentFile)
    }

    // Find all #include directives
    // Pattern: #include <file.h> or #include "file.h"
    re := regexp.MustCompile(`#include\s+(?:<([^>]+)>|"([^"]+)")`)
    matches := re.FindAllStringSubmatch(text, -1)

    if debugAuto {
        fmt.Printf("[AUTO] parseIncludedHeaders: found %d #include directives\n", len(matches))

        // Debug: check if #include directives are visible in text
        if strings.Contains(text, "#include") {
            fmt.Printf("[AUTO] DEBUG: Text contains '#include' but regex found %d matches\n", len(matches))
            // Show where #include appears
            idx := strings.Index(text, "#include")
            if idx >= 0 {
                start := idx
                if start > 50 {
                    start -= 50
                }
                end := idx + 100
                if end > len(text) {
                    end = len(text)
                }
                snippet := text[start:end]
                fmt.Printf("[AUTO] DEBUG: Context around first #include:\n%q\n", snippet)
            }
        }
    }

    for _, match := range matches {
        var includePath string
        isSystem := false
        if match[1] != "" {
            // System include: <file.h>
            includePath = match[1]
            isSystem = true
        } else {
            // Local include: "file.h"
            includePath = match[2]
        }

        var resolvedPath string

        if isSystem {
            // For system includes like <bits/struct_stat.h>, try common system paths
            systemPaths := []string{
                "/usr/include/" + includePath,
                "/usr/local/include/" + includePath,
            }

            // Add architecture-specific paths for Linux systems
            if runtime.GOOS == "linux" {
                if runtime.GOARCH == "amd64" {
                    systemPaths = append(systemPaths, "/usr/include/x86_64-linux-gnu/"+includePath)
                } else if runtime.GOARCH == "arm64" {
                    systemPaths = append(systemPaths, "/usr/include/aarch64-linux-gnu/"+includePath)
                }
            }

            for _, tryPath := range systemPaths {
                if _, err := os.Stat(tryPath); err == nil {
                    resolvedPath = tryPath
                    break
                }
            }
        } else {
            // For local includes, resolve relative to current file
            resolvedPath = resolveIncludePath("#include \""+includePath+"\"", currentFile)
        }

        if resolvedPath == "" {
            if debugAuto {
                fmt.Printf("[AUTO] Warning: could not resolve include path: %s\n", includePath)
            }
            continue
        }

        // Read the included file
        content, err := os.ReadFile(resolvedPath)
        if err != nil {
            if debugAuto {
                fmt.Printf("[AUTO] Warning: could not read included file %s: %v\n", resolvedPath, err)
            }
            continue
        }

        includedText := string(content)
        includedText = stripCComments(includedText)

        // Save the unpreprocessed text for fallback struct extraction
        // Some structs have internal #ifdef directives that may cause them to be stripped by the preprocessor
        unpreprocessedText := includedText

        // THIRD: Recursively process includes BEFORE preprocessing
        // This must happen on the original (non-preprocessed) text so #include directives are still visible
        // Do this FIRST so we can find nested includes like <bits/struct_stat.h> from <bits/stat.h>
        if debugAuto {
            fmt.Printf("[AUTO] Recursively processing includes from: %s\n", resolvedPath)
        }
        parseIncludedHeaders(includedText, alias, resolvedPath)

        // PREPROCESSOR: Run preprocessor on included file to resolve #ifdef blocks
        // This reveals struct definitions that may be hidden behind conditionals
        if debugAuto {
            fmt.Printf("[AUTO] Running preprocessor on included file: %s\n", resolvedPath)
        }
        includedText = parsePreprocessor(includedText, alias, resolvedPath, 0)

        if debugAuto {
            fmt.Printf("[AUTO] Parsing typedefs from included file: %s\n", resolvedPath)
        }

        // FIRST: Parse typedefs from the included file
        // This must happen before struct parsing so that typedef'd types in struct fields are resolved
        if err := parseTypedefs(includedText, alias); err != nil {
            if debugAuto {
                fmt.Printf("[AUTO] Warning: failed to parse typedefs from %s: %v\n", resolvedPath, err)
            }
        }

        if debugAuto {
            fmt.Printf("[AUTO] Parsing structs from included file: %s\n", resolvedPath)
        }

        // SECOND: Parse structs from the included file
        // Now that typedefs are registered, struct fields can be resolved properly
        // Pass both preprocessed and unpreprocessed text; will use unpreprocessed as fallback
        if err := parsePlainStructsWithFallback(includedText, unpreprocessedText, alias); err != nil {
            if debugAuto {
                fmt.Printf("[AUTO] Warning: failed to parse structs from %s: %v\n", resolvedPath, err)
            }
        }
    }
}

// parsePlainStructsWithFallback parses plain struct definitions from preprocessed text,
// and falls back to unpreprocessed text if no structs are found
// This handles cases where internal #ifdef directives cause structs to be stripped by the preprocessor
func parsePlainStructsWithFallback(preprocessedText, unpreprocessedText, alias string) error {
    debugAuto := os.Getenv("ZA_DEBUG_AUTO") != ""

    // First try parsing from the preprocessed text
    structs, err := parsePlainStructsInternal(preprocessedText, alias, true)
    if err != nil {
        return err
    }

    // If no structs found in preprocessed text, try unpreprocessed as fallback
    // This handles structs with internal preprocessor directives like #ifdef inside the struct body
    if len(structs) == 0 && preprocessedText != unpreprocessedText {
        if debugAuto {
            fmt.Printf("[AUTO] No structs found in preprocessed text, trying unpreprocessed fallback...\n")
        }
        structs, err = parsePlainStructsInternal(unpreprocessedText, alias, false)
        if err != nil {
            return err
        }
    }

    return nil
}

// parsePlainStructs parses plain struct definitions (not typedefs)
// Pattern: struct name { fields };
// This complements parseStructTypedefs which only handles: typedef struct name { fields } name;
func parsePlainStructs(text string, alias string) error {
    _, err := parsePlainStructsInternal(text, alias, true)
    return err
}

// parsePlainStructsInternal does the actual struct parsing
// isPreprocessed indicates whether the text has been preprocessed (affects how we handle field parsing)
func parsePlainStructsInternal(text string, alias string, isPreprocessed bool) ([]struct{ name string; fieldBlock string }, error) {
    debugAuto := os.Getenv("ZA_DEBUG_AUTO") != ""
    if debugAuto {
        fmt.Printf("[AUTO] parsePlainStructs called for alias=%s, text length=%d\n", alias, len(text))

        // Debug: Check if "struct stat" appears in text at all
        if strings.Contains(text, "struct stat") {
            fmt.Printf("[AUTO]   Text contains 'struct stat'\n")
            // Find and show the context around "struct stat"
            idx := strings.Index(text, "struct stat")
            start := idx
            if start > 50 {
                start -= 50
            }
            end := idx + len("struct stat") + 150
            if end > len(text) {
                end = len(text)
            }
            snippet := text[start:end]
            fmt.Printf("[AUTO]   Context: %q\n", snippet)
        } else {
            fmt.Printf("[AUTO]   WARNING: Text does NOT contain 'struct stat'\n")
        }
    }

    // Find struct definitions using proper brace matching (not regex)
    // This handles nested #ifdef blocks and other complexities
    var structs []struct {
        name      string
        fieldBlock string
    }

    // Pattern to find struct declarations: "struct Name {"
    // Note: \s includes newlines in Go's regexp package
    startRe := regexp.MustCompile(`\bstruct\s+([A-Za-z_][A-Za-z0-9_]*)\s*\{`)
    matches := startRe.FindAllStringSubmatchIndex(text, -1)

    if debugAuto && len(matches) == 0 {
        // Debug: check if "struct stat" appears at all
        if strings.Contains(text, "struct stat") {
            fmt.Printf("[AUTO] DEBUG: Text contains 'struct stat' but regex didn't match\n")
            // Show the context around "struct stat"
            idx := strings.Index(text, "struct stat")
            start := idx
            if start > 100 {
                start -= 100
            }
            end := idx + len("struct stat") + 200
            if end > len(text) {
                end = len(text)
            }
            snippet := text[start:end]
            fmt.Printf("[AUTO] DEBUG: Context around 'struct stat':\n%q\n", snippet)
        }
    }

    for _, matchIdx := range matches {
        // matchIdx format: [start0, end0, start1, end1]
        // Group 1: struct name
        structName := text[matchIdx[2]:matchIdx[3]]
        openBracePos := matchIdx[1] - 1  // Position of the '{'

        // Find matching closing brace using brace counting
        braceCount := 1
        closePos := openBracePos + 1

        for closePos < len(text) && braceCount > 0 {
            ch := text[closePos]
            if ch == '{' {
                braceCount++
            } else if ch == '}' {
                braceCount--
                if braceCount == 0 {
                    break
                }
            } else if ch == '"' {
                // Skip string literals
                closePos++
                for closePos < len(text) && text[closePos] != '"' {
                    if text[closePos] == '\\' && closePos+1 < len(text) {
                        closePos += 2
                    } else {
                        closePos++
                    }
                }
            }
            closePos++
        }

        if braceCount == 0 {
            // Extract field block (content between { and })
            fieldBlock := text[openBracePos+1 : closePos]
            structs = append(structs, struct {
                name      string
                fieldBlock string
            }{structName, fieldBlock})

            if debugAuto {
                fmt.Printf("[AUTO] Found struct %s with field block size %d\n", structName, len(fieldBlock))
            }
        }
    }

    if debugAuto {
        fmt.Printf("[AUTO] parsePlainStructs: found %d struct definitions\n", len(structs))
    }

    // Return the found structs (internal helper returns array before processing)
    if !isPreprocessed {
        // For unpreprocessed text, just return the found structs without processing
        return structs, nil
    }

    for _, s := range structs {
        structName := s.name
        fieldBlock := s.fieldBlock
        fullName := alias + "::" + structName

        if debugAuto {
            fmt.Printf("[AUTO] Found plain struct: %s\n", structName)
            // Show first part of field block for debugging
            blockPreview := fieldBlock
            if len(blockPreview) > 200 {
                blockPreview = blockPreview[:200]
            }
            fmt.Printf("[AUTO]   Field block preview (first 200 chars): %q\n", blockPreview)
        }

        // Skip if this struct was already parsed as a typedef (only for preprocessed text)
        if isPreprocessed {
            ffiStructLock.RLock()
            alreadyExists := ffiStructDefinitions[fullName] != nil
            ffiStructLock.RUnlock()

            if alreadyExists {
                if debugAuto {
                    fmt.Printf("[AUTO] Struct %s already registered (from typedef), skipping plain struct\n", structName)
                }
                continue
            }
        }

        // Remove #ifdef directives from field block before parsing
        // This ensures the field block contains only actual field declarations
        cleanedFieldBlock := removeConditionalDirectives(fieldBlock)

        if debugAuto {
            fmt.Printf("[AUTO]   After removing conditionals, field block size: %d chars\n", len(cleanedFieldBlock))
            // Show first part of cleaned field block
            cleanedPreview := cleanedFieldBlock
            if len(cleanedPreview) > 200 {
                cleanedPreview = cleanedPreview[:200]
            }
            fmt.Printf("[AUTO]   Cleaned field block preview: %q\n", cleanedPreview)
        }

        // Parse fields from the cleaned field block
        fields, totalSize, err := parseStructFields(cleanedFieldBlock, alias, structName, debugAuto)
        if err != nil {
            // Production warning - always visible
            errMsg := fmt.Sprintf("skipped struct %s: %v", structName, err)
            msg := fmt.Sprintf("[AUTO] Warning: %s", errMsg)
            addMessageToCurrentProgress(msg)

            // Track error for programmatic access
            autoImportErrorsLock.Lock()
            autoImportErrors[alias] = append(autoImportErrors[alias], errMsg)
            autoImportErrorsLock.Unlock()

            if debugAuto {
                fmt.Printf("[AUTO] Debug: struct %s parse error details: %v\n", structName, err)
            }
            continue // Skip this struct but continue parsing others
        }

        if debugAuto {
            fmt.Printf("[AUTO] Parsed %d fields from struct %s, total size: %d bytes\n", len(fields), structName, totalSize)
        }

        // Create CLibraryStruct for the struct
        structDef := &CLibraryStruct{
            Name:    structName,
            Fields:  fields,
            Size:    totalSize,
            IsUnion: false,
        }

        // Store in FFI struct registry (from lib-c.go)
        ffiStructLock.Lock()
        ffiStructDefinitions[fullName] = structDef
        ffiStructLock.Unlock()

        // ALSO register as typed Za struct (makes AUTO structs available in Za code)
        registerStructInZa(alias, structName, structDef)

        if debugAuto {
            fmt.Printf("[AUTO] Registered plain struct %s (size: %d bytes, %d fields)\n",
                structName, totalSize, len(fields))
        }

        // Check if we should abort (interactive mode error signaled via sig_int)
        if sig_int {
            return nil, fmt.Errorf("AUTO import aborted during plain struct parsing")
        }
    }

    return structs, nil
}

// splitFieldDeclarations splits field declarations by semicolon while respecting brace/paren nesting
// This prevents splitting inside function pointers or nested structs
func splitFieldDeclarations(fieldBlock string) []string {
    var declarations []string
    var current strings.Builder
    var parenDepth, braceDepth int

    for _, ch := range fieldBlock {
        switch ch {
        case '(':
            parenDepth++
            current.WriteRune(ch)
        case ')':
            parenDepth--
            current.WriteRune(ch)
        case '{':
            braceDepth++
            current.WriteRune(ch)
        case '}':
            braceDepth--
            current.WriteRune(ch)
        case ';':
            if parenDepth == 0 && braceDepth == 0 {
                // This is a declaration boundary
                declarations = append(declarations, current.String())
                current.Reset()
            } else {
                // Inside parens or braces, treat as part of declaration
                current.WriteRune(ch)
            }
        default:
            current.WriteRune(ch)
        }
    }

    // Add any remaining declaration
    if current.Len() > 0 {
        declarations = append(declarations, current.String())
    }

    return declarations
}

// isFunctionPointerField detects function pointer fields like "int (*callback)(void*)"
func isFunctionPointerField(decl string) bool {
    // Quick check first
    if !strings.Contains(decl, "(*") {
        return false
    }

    // If declaration contains braces, it's likely an inline struct/union
    // containing function pointers, not a direct function pointer field
    if strings.Contains(decl, "{") || strings.Contains(decl, "}") {
        return false
    }

    return true
}

// parseFunctionPointerField parses a function pointer field declaration
// Example: "int (*compare)(int, int)" -> fieldName="compare", signature for "int,int->int"
func parseFunctionPointerField(decl string) (fieldName string, sig CFunctionSignature, err error) {
    decl = strings.TrimSpace(decl)

    // Find the opening paren containing the *
    startIdx := strings.Index(decl, "(*")
    if startIdx == -1 {
        return "", CFunctionSignature{}, fmt.Errorf("invalid function pointer field format")
    }

    // Extract return type (everything before "(*")
    returnTypeStr := strings.TrimSpace(decl[:startIdx])

    // Find the closing paren of the name
    endIdx := strings.Index(decl[startIdx:], ")")
    if endIdx == -1 {
        return "", CFunctionSignature{}, fmt.Errorf("unclosed parenthesis in function pointer field name")
    }
    endIdx += startIdx

    // Extract name (between * and the closing paren)
    fieldName = strings.TrimSpace(decl[startIdx+2 : endIdx])

    // Find the parameter list (opening paren after the name paren)
    paramStart := strings.Index(decl[endIdx:], "(")
    if paramStart == -1 {
        return "", CFunctionSignature{}, fmt.Errorf("no parameter list in function pointer field")
    }
    paramStart += endIdx

    paramEnd := strings.LastIndex(decl, ")")
    if paramEnd <= paramStart {
        return "", CFunctionSignature{}, fmt.Errorf("unclosed parameter list in function pointer field")
    }

    // Extract parameter string
    paramStr := strings.TrimSpace(decl[paramStart+1 : paramEnd])

    // Parse return type
    returnType, returnStructName, err := StringToCType(returnTypeStr)
    if err != nil {
        return "", CFunctionSignature{}, fmt.Errorf("invalid return type in function pointer field: %w", err)
    }

    // Parse parameters
    var paramTypes []CType
    var paramStructNames []string

    if paramStr != "" && paramStr != "void" {
        params := strings.Split(paramStr, ",")
        for _, param := range params {
            param = strings.TrimSpace(param)
            // Remove type qualifiers
            param = strings.TrimPrefix(param, "const ")
            param = strings.TrimPrefix(param, "volatile ")
            param = strings.TrimPrefix(param, "restrict ")
            param = strings.TrimSpace(param)

            if param == "..." {
                continue
            }

            // Remove parameter names (if present)
            parts := strings.Fields(param)
            if len(parts) > 1 {
                param = strings.Join(parts[:len(parts)-1], " ")
            }

            ptype, pstruct, err := StringToCType(param)
            if err != nil {
                return "", CFunctionSignature{}, fmt.Errorf("invalid parameter type '%s': %w", param, err)
            }
            paramTypes = append(paramTypes, ptype)
            paramStructNames = append(paramStructNames, pstruct)
        }
    }

    sig = CFunctionSignature{
        ParamTypes:       paramTypes,
        ParamStructNames: paramStructNames,
        ReturnType:       returnType,
        ReturnStructName: returnStructName,
        HasVarargs:       false,
        FixedArgCount:    len(paramTypes),
    }

    return fieldName, sig, nil
}

// isNestedStructOrUnion detects inline struct/union definitions like "struct { int x; } nested;"
func isNestedStructOrUnion(decl string) bool {
    hasStruct := strings.Contains(decl, "struct")
    hasUnion := strings.Contains(decl, "union")
    hasBrace := strings.Contains(decl, "{")
    return (hasStruct || hasUnion) && hasBrace
}

// parseInlineUnion extracts and parses an inline union/struct definition from a field declaration
// Example input: "union { char *a; wchar_t *b; } field_name"
// Returns: field name, union definition, size, or error
func parseInlineUnion(decl string, alias string, debug bool) (fieldName string, unionDef *CLibraryStruct, size uintptr, err error) {
    decl = strings.TrimSpace(decl)

    // 1. Find the opening brace position
    openBrace := strings.Index(decl, "{")
    if openBrace == -1 {
        return "", nil, 0, fmt.Errorf("no opening brace found in inline union/struct: %s", decl)
    }

    // 2. Find matching closing brace using brace counting
    braceCount := 0
    closeBrace := -1
    for i := openBrace; i < len(decl); i++ {
        if decl[i] == '{' {
            braceCount++
        } else if decl[i] == '}' {
            braceCount--
            if braceCount == 0 {
                closeBrace = i
                break
            }
        }
    }

    if closeBrace == -1 {
        return "", nil, 0, fmt.Errorf("no matching closing brace in inline union/struct (found %d braces): %s", braceCount, decl)
    }

    // 3. Extract field block (between braces)
    fieldBlock := decl[openBrace+1 : closeBrace]

    // 4. Extract field name (after closing brace)
    afterBrace := strings.TrimSpace(decl[closeBrace+1:])
    parts := strings.Fields(afterBrace)
    if len(parts) == 0 {
        return "", nil, 0, fmt.Errorf("no field name after inline union definition")
    }
    fieldName = parts[0]

    // 5. Determine if it's a union or struct
    isUnion := strings.Contains(decl[:openBrace], "union")

    // 6. Parse the fields
    var fields []StructField
    var fieldSize uintptr

    if isUnion {
        // Generate a unique name for the anonymous union
        anonName := fmt.Sprintf("__anon_union_%s", fieldName)
        fields, fieldSize, err = parseUnionFields(fieldBlock, alias, anonName, debug)
    } else {
        // It's an inline struct
        anonName := fmt.Sprintf("__anon_struct_%s", fieldName)
        fields, fieldSize, err = parseStructFields(fieldBlock, alias, anonName, debug)
    }

    if err != nil {
        return "", nil, 0, fmt.Errorf("failed to parse inline union/struct: %v", err)
    }

    // 7. Create CLibraryStruct for the inline union/struct
    unionDef = &CLibraryStruct{
        Name:    fieldName, // Use field name as union name
        Fields:  fields,
        Size:    fieldSize,
        IsUnion: isUnion,
    }

    return fieldName, unionDef, fieldSize, nil
}

// isBitfieldDeclaration detects bitfield syntax like "unsigned int flags : 8;"
func isBitfieldDeclaration(decl string) bool {
    return strings.Contains(decl, ":")
}

// isLikelyOpaquePointer returns true if a type name looks like an opaque pointer typedef
// Examples: GC, Display, Window, Visual, Screen, etc.
// Heuristic: Single word, uppercase first letter, no spaces, looks like a typedef'd handle
func isLikelyOpaquePointer(typeName string) bool {
    // Reject multi-word types
    if strings.Contains(typeName, " ") {
        return false
    }
    if len(typeName) == 0 {
        return false
    }
    // Check if it starts with uppercase (suggests typedef'd opaque type)
    firstChar := typeName[0]
    if firstChar >= 'A' && firstChar <= 'Z' {
        return true // Uppercase start suggests typedef'd opaque handle
    }
    return false
}

// isValidFieldDeclaration validates that a declaration looks like an actual field
// Rejects lone punctuation, bare keywords, and requires valid identifier as field name
func isValidFieldDeclaration(decl string) bool {
    // Trim and check if empty
    decl = strings.TrimSpace(decl)
    if decl == "" {
        return false
    }

    // Check for lone punctuation
    if decl == "(" || decl == ")" || decl == "{" || decl == "}" {
        return false
    }

    // Split by whitespace and check parts
    parts := strings.Fields(decl)
    if len(parts) < 2 {
        return false
    }

    // Last part should be valid identifier (field name)
    // A valid identifier starts with letter or underscore
    fieldName := parts[len(parts)-1]

    // Strip leading * for pointer declarations like "Type *field"
    fieldName = strings.TrimLeft(fieldName, "*")

    if len(fieldName) == 0 {
        return false
    }

    firstChar := fieldName[0]
    if !((firstChar >= 'a' && firstChar <= 'z') ||
        (firstChar >= 'A' && firstChar <= 'Z') ||
        firstChar == '_') {
        return false
    }

    // Reject if last part is a bare keyword (type without field)
    bareKeywords := []string{"unsigned", "signed", "struct", "union", "int", "char", "short", "long", "double", "float", "void"}
    for _, kw := range bareKeywords {
        if fieldName == kw {
            return false
        }
    }

    return true
}

// warnStructField outputs a production warning for skipped struct/union fields
// Does not require debug flags - always outputs to stderr
func warnStructField(structName, fieldDecl, reason string) {
    msg := fmt.Sprintf("[AUTO] Warning: skipping field in struct %s: %s (reason: %s)",
        structName, strings.TrimSpace(fieldDecl), reason)
    addMessageToCurrentProgress(msg)
}

// parseStructFields parses the field declarations inside a struct definition
// Returns the fields, the total size, and any error
// Unlike unions, struct fields have sequential offsets
func parseStructFields(fieldBlock string, alias string, structName string, debug bool) ([]StructField, uintptr, error) {
    var fields []StructField
    var currentOffset uintptr = 0
    var maxAlignment uintptr = 1  // Track maximum alignment requirement

    // Split by semicolons to get individual field declarations
    declarations := splitFieldDeclarations(fieldBlock)

    for _, decl := range declarations {
        decl = strings.TrimSpace(decl)
        if decl == "" {
            continue
        }

        // Handle function pointer fields
        if isFunctionPointerField(decl) {
            fieldName, sig, err := parseFunctionPointerField(decl)
            if err != nil {
                return nil, 0, fmt.Errorf("failed to parse function pointer field: %w", err)
            }

            // Create function pointer field
            field := StructField{
                Name:              fieldName,
                Type:              CPointer, // Function pointers are pointers
                Offset:            currentOffset,
                IsFunctionPtr:     true,
                FunctionSignature: &sig,
            }

            fields = append(fields, field)
            // Function pointers are sizeof(void*) on all platforms
            ptrSize := unsafe.Sizeof(unsafe.Pointer(nil))
            currentOffset += ptrSize
            if ptrSize > maxAlignment {
                maxAlignment = ptrSize
            }

            if debug && os.Getenv("ZA_DEBUG_AUTO") != "" {
                fmt.Printf("[AUTO] Function pointer field: %s at offset %d\n", fieldName, field.Offset)
            }
            continue
        }

        // Check for inline struct/union definitions
        if isNestedStructOrUnion(decl) {
            if debug {
                fmt.Printf("[AUTO]   parseStructFields: Detected inline union/struct in field declaration: %q\n", decl)
            }
            // Parse the inline union/struct
            fieldName, inlineUnionDef, inlineSize, err := parseInlineUnion(decl, alias, debug)
            if err != nil {
                if debug {
                    fmt.Printf("[AUTO]   parseStructFields: ERROR parsing inline union: %v\n", err)
                }
                return nil, 0, fmt.Errorf("failed to parse inline union in field: %v", err)
            }

            // Calculate alignment
            fieldAlignment := inlineSize
            if fieldAlignment > 8 {
                fieldAlignment = 8 // Cap at 8 bytes
            }
            if fieldAlignment > 1 && currentOffset%fieldAlignment != 0 {
                currentOffset = ((currentOffset + fieldAlignment - 1) / fieldAlignment) * fieldAlignment
            }

            // Create field with inline union
            field := StructField{
                Name:     fieldName,
                Type:     CStruct, // Treat as struct type for now
                Offset:   currentOffset,
                IsUnion:  inlineUnionDef.IsUnion,    // ← POPULATE THIS FIELD!
                UnionDef: inlineUnionDef,             // ← POPULATE THIS FIELD!
            }

            fields = append(fields, field)

            // Track maximum alignment
            if fieldAlignment > maxAlignment {
                maxAlignment = fieldAlignment
            }

            // Update offset for next field
            currentOffset += inlineSize

            if debug {
                unionOrStruct := "struct"
                if inlineUnionDef.IsUnion {
                    unionOrStruct = "union"
                }
                fmt.Printf("[AUTO]   Inline %s field: %s (size: %d bytes, offset: %d)\n",
                    unionOrStruct, fieldName, inlineSize, field.Offset)
            }

            continue  // Move to next field declaration
        }

        // Fail if field is a bitfield
        if isBitfieldDeclaration(decl) {
            return nil, 0, fmt.Errorf("contains bitfield (not supported): %s", decl)
        }

        // Fail if field declaration is malformed
        if !isValidFieldDeclaration(decl) {
            return nil, 0, fmt.Errorf("contains malformed field declaration: %s", decl)
        }

        // Parse field declaration: type field_name or type field_name[size]
        // Examples: "int x", "float values[4]", "unsigned char bytes[16]"

        // Handle array fields: type name[size]
        var fieldName string
        var fieldType CType
        var fieldSize uintptr
        var arraySize int = 0
        var elementType CType
        var typeStr string
        var nestedStruct *CLibraryStruct

        if strings.Contains(decl, "[") && strings.HasSuffix(decl, "]") {
            // Array field
            openBracket := strings.Index(decl, "[")
            closeBracket := strings.LastIndex(decl, "]")

            if openBracket > 0 && closeBracket > openBracket {
                // Extract size
                arraySizeStr := strings.TrimSpace(decl[openBracket+1 : closeBracket])

                // Try to resolve as a constant macro first
                resolved := false
                moduleConstantsLock.RLock()
                if constants, exists := moduleConstants[alias]; exists {
                    if val, found := constants[arraySizeStr]; found {
                        switch v := val.(type) {
                        case int:
                            arraySize = v
                            resolved = true
                        case int64:
                            arraySize = int(v)
                            resolved = true
                        case float64:
                            arraySize = int(v)
                            resolved = true
                        }
                    }
                }
                moduleConstantsLock.RUnlock()

                // Fallback to literal integer or expression evaluation
                if !resolved {
                    size, err := strconv.Atoi(arraySizeStr)
                    if err != nil {
                        // Try to evaluate as an expression (e.g., "1024 / 64" or "__FD_SETSIZE / __NFDBITS")
                        if val, ok := evaluateConstant(arraySizeStr, nil, alias, 0, false, nil); ok {
                            switch v := val.(type) {
                            case int:
                                arraySize = v
                                resolved = true
                            case int64:
                                arraySize = int(v)
                                resolved = true
                            case float64:
                                arraySize = int(v)
                                resolved = true
                            }
                        }
                        if !resolved {
                            return nil, 0, fmt.Errorf("contains array field with invalid size: %s", decl)
                        }
                    } else {
                        arraySize = size
                    }
                }

                // Extract type and name before [
                beforeBracket := strings.TrimSpace(decl[:openBracket])
                parts := strings.Fields(beforeBracket)
                if len(parts) < 2 {
                    return nil, 0, fmt.Errorf("contains invalid array field declaration: %s", decl)
                }

                // Last part is field name, everything else is type
                fieldName = parts[len(parts)-1]
                typeStr = strings.Join(parts[:len(parts)-1], " ")

                // Parse element type
                elemType, elemSize := parseCTypeString(typeStr, alias)
                if elemType == CVoid {
                    return nil, 0, fmt.Errorf("contains array field with unsupported element type '%s': %s", typeStr, decl)
                }

                elementType = elemType
                fieldType = elemType // For arrays, store element type
                fieldSize = elemSize * uintptr(arraySize)
            }
        } else {
            // Regular field (non-array)
            // Handle multi-declaration like "int x, y;" or "int x_root, y_root;"
            parts := strings.Fields(decl)
            if len(parts) < 2 {
                return nil, 0, fmt.Errorf("contains invalid field declaration: %s", decl)
            }

            // Check for comma-separated field names (multi-declaration)
            // Example: "int x, y" becomes ["int", "x,", "y"]
            var fieldNames []string
            typeEndIndex := -1

            for i, part := range parts {
                if strings.Contains(part, ",") || (i > 0 && typeEndIndex >= 0) {
                    // This is a field name (possibly with comma)
                    if typeEndIndex < 0 {
                        typeEndIndex = i - 1
                    }
                    fieldNames = append(fieldNames, strings.TrimSuffix(part, ","))
                } else if i == len(parts)-1 {
                    // Last part - could be single field name or last in multi-declaration
                    if typeEndIndex < 0 {
                        // Single field
                        typeEndIndex = i - 1
                    }
                    fieldNames = append(fieldNames, part)
                }
            }

            if typeEndIndex < 0 || len(fieldNames) == 0 {
                return nil, 0, fmt.Errorf("contains field declaration that could not be parsed: %s", decl)
            }

            // Type is everything before the field names
            typeStr = strings.Join(parts[:typeEndIndex+1], " ")

            // Parse type once for all fields in this declaration
            fType, fSize := parseCTypeString(typeStr, alias)

            // Process each field name
            for _, fname := range fieldNames {
                fieldName = fname

                // Check if fieldName starts with * (pointer notation like "*name")
                if strings.HasPrefix(fieldName, "*") {
                    // Move the * from field name to type
                    fieldName = strings.TrimPrefix(fieldName, "*")
                    typeStr = strings.TrimSpace(typeStr) + " *"
                    // Re-parse with pointer
                    fType, fSize = parseCTypeString(typeStr, alias)
                }

                if fType == CVoid {
                    // Check if this is a struct type
                    possibleNames := []string{typeStr, alias + "::" + typeStr}
                    var foundStruct *CLibraryStruct
                    for _, name := range possibleNames {
                        if structDef, exists := ffiStructDefinitions[name]; exists {
                            foundStruct = structDef
                            break
                        }
                    }

                    if foundStruct != nil {
                        fieldType = CStruct
                        fieldSize = foundStruct.Size
                        nestedStruct = foundStruct
                    } else {
                        // Try heuristic: is it likely an opaque pointer (like GC, Display, Window)?
                        if isLikelyOpaquePointer(typeStr) {
                            fieldType = CPointer
                            fieldSize = 8  // Pointer size on x86-64
                            nestedStruct = nil
                            if debug {
                                fmt.Printf("[AUTO] Treating unknown type %s as opaque pointer\n", typeStr)
                            }
                        } else {
                            // Cannot resolve type - fail struct parsing
                            return nil, 0, fmt.Errorf("contains unresolved type '%s' in field: %s", typeStr, decl)
                        }
                    }
                } else {
                    fieldType = fType
                    fieldSize = fSize
                    nestedStruct = nil
                }

                // Align field to its natural alignment (up to 8 bytes on x86-64)
                fieldAlignment := fieldSize
                if fieldAlignment > 8 {
                    fieldAlignment = 8 // Cap at 8 bytes (x86-64 maximum)
                }
                if fieldAlignment > 1 && currentOffset%fieldAlignment != 0 {
                    currentOffset = ((currentOffset + fieldAlignment - 1) / fieldAlignment) * fieldAlignment
                }

                // Create field for this name
                field := StructField{
                    Name:        fieldName,
                    Type:        fieldType,
                    Offset:      currentOffset,
                    ArraySize:   arraySize,
                    ElementType: elementType,
                }

                // If this is a nested struct field, populate StructName and StructDef
                if fieldType == CStruct && nestedStruct != nil {
                    field.StructName = typeStr
                    field.StructDef = nestedStruct
                }

                fields = append(fields, field)

                // Track maximum alignment requirement
                if fieldAlignment > maxAlignment {
                    maxAlignment = fieldAlignment
                }

                // Update offset for next field
                currentOffset += fieldSize
            }

            // Skip the normal field append since we handled it in the loop
            continue
        }

        // Align field to its natural alignment (up to 8 bytes on x86-64)
        fieldAlignment := fieldSize
        if arraySize > 0 {
            // For arrays, alignment is the element size
            elementSize := fieldSize / uintptr(arraySize)
            fieldAlignment = elementSize
        }
        if fieldAlignment > 8 {
            fieldAlignment = 8 // Cap at 8 bytes (x86-64 maximum)
        }
        if fieldAlignment > 1 && currentOffset%fieldAlignment != 0 {
            currentOffset = ((currentOffset + fieldAlignment - 1) / fieldAlignment) * fieldAlignment
        }

        // Struct fields have sequential offsets (not overlapping like unions)
        field := StructField{
            Name:        fieldName,
            Type:        fieldType,
            Offset:      currentOffset,
            ArraySize:   arraySize,
            ElementType: elementType,
        }

        // If this is a nested struct field, populate StructName and StructDef
        if fieldType == CStruct && nestedStruct != nil {
            field.StructName = typeStr
            field.StructDef = nestedStruct
        }

        fields = append(fields, field)

        // Track maximum alignment requirement
        if fieldAlignment > maxAlignment {
            maxAlignment = fieldAlignment
        }

        // Update offset for next field
        currentOffset += fieldSize

        if debug {
            if arraySize > 0 {
                fmt.Printf("[AUTO]   Field: %+v %s[%d] (size: %d bytes, offset: %d)\n",
                    fieldType, fieldName, arraySize, fieldSize, field.Offset)
            } else {
                fmt.Printf("[AUTO]   Field: %+v %s (size: %d bytes, offset: %d)\n",
                    fieldType, fieldName, fieldSize, field.Offset)
            }
        }

        // Check if we should abort (interactive mode error signaled via sig_int)
        if sig_int {
            return nil, 0, fmt.Errorf("AUTO import aborted during struct field parsing")
        }
    }

    // Add padding to align struct size to maximum field alignment
    // This matches C struct padding rules (struct size is multiple of max alignment)
    paddedSize := currentOffset
    if maxAlignment > 1 && currentOffset%maxAlignment != 0 {
        paddedSize = ((currentOffset + maxAlignment - 1) / maxAlignment) * maxAlignment
    }

    if debug {
        fmt.Printf("[AUTO]   Struct size before padding: %d, max alignment: %d, final size: %d\n",
            currentOffset, maxAlignment, paddedSize)
    }

    return fields, paddedSize, nil
}

// capitalizeFieldName capitalizes the first letter of a field name for Go reflection
// This matches Za's renameSF() function behavior
func capitalizeFieldName(name string) string {
    r, i := utf8.DecodeRuneInString(name)
    return string(unicode.ToTitle(r)) + name[i:]
}

// cTypeToZaType converts a CType enum to Za type string
// Used for registering C structs in Za's structmaps
func cTypeToZaType(ctype CType) string {
    switch ctype {
    case CInt, CInt8, CInt16, CInt64:
        return "int"
    case CUInt, CUInt8, CUInt16, CUInt64:
        return "int" // Za uses int for unsigned types
    case CFloat, CDouble:
        return "float"
    case CString:
        return "string"
    case CPointer:
        return "any" // Pointers map to any type (opaque handles)
    case CStruct:
        return "any" // Nested struct (full support in future phase)
    case CChar:
        return "int" // char maps to int in Za
    case CBool:
        return "bool"
    default:
        return "any"
    }
}

// cTypeToZaTypeString converts CType to Za type string with array handling
// For arrays, uses "any" type to avoid strict type checking issues
func cTypeToZaTypeString(ctype CType, arraySize int, elemType CType) string {
    if arraySize > 0 {
        // For array fields, use "any" type
        // Za's type system doesn't strictly distinguish array element types in struct instantiation
        return "any"
    }
    return cTypeToZaType(ctype)
}

// registerStructInZa registers a C struct from ffiStructDefinitions into Za's structmaps
// This makes AUTO-parsed structs available as typed Za structs
// Za structs are namespace-scoped, so we register with module alias prefix
func registerStructInZa(alias string, structName string, structDef *CLibraryStruct) {
    // Convert CLibraryStruct.Fields to structmaps format
    // Format: [name1, type1, hasDefault1, default1, name2, type2, ...]
    var fields []any

    for _, field := range structDef.Fields {
        // [0] field name (capitalize first letter for Go reflection)
        fields = append(fields, capitalizeFieldName(field.Name))

        // [1] field type string
        zaType := cTypeToZaTypeString(field.Type, field.ArraySize, field.ElementType)
        fields = append(fields, zaType)

        // [2] has default (always false for C structs)
        fields = append(fields, false)

        // [3] default value (nil for C structs)
        fields = append(fields, nil)
    }

    // Register in structmaps with namespace prefix (like Za-defined structs)
    structmapslock.Lock()
    fullName := alias + "::" + structName
    structmaps[fullName] = fields
    structmapslock.Unlock()

    if os.Getenv("ZA_DEBUG_AUTO") != "" {
        fmt.Printf("[AUTO] Registered %s as Za struct type (%d fields)\n",
            structName, len(structDef.Fields))
    }
}

// parseCTypeString converts a C type string to CType and returns its size
// Handles types like "int", "float", "unsigned char", "unsigned int", etc.
func parseCTypeString(typeStr string, alias string) (CType, uintptr) {
    // Normalize type string
    typeStr = strings.TrimSpace(typeStr)

    // Check for pointer types first (before lowercasing)
    // Handle both "Type*" and "Type *" formats
    if strings.Contains(typeStr, "*") {
        return CPointer, unsafe.Sizeof(uintptr(0))
    }

    // Check typedef registry BEFORE lowercasing (typedefs are case-sensitive)
    // First try the specific alias, then fall back to use chain
    moduleTypedefsLock.RLock()
    resolved := false
    var baseType string

    if aliasMap, hasAlias := moduleTypedefs[alias]; hasAlias {
        if bt, ok := aliasMap[typeStr]; ok {
            baseType = bt
            resolved = true
        }
    }

    // If not found in specific alias, search through use chain
    if !resolved {
        moduleTypedefsLock.RUnlock()
        if resolverAlias := uc_match_typedef(typeStr); resolverAlias != "" {
            moduleTypedefsLock.RLock()
            if aliasMap, hasAlias := moduleTypedefs[resolverAlias]; hasAlias {
                if bt, ok := aliasMap[typeStr]; ok {
                    baseType = bt
                    resolved = true
                }
            }
        }
        moduleTypedefsLock.RLock()
    }
    moduleTypedefsLock.RUnlock()

    if resolved {
        // Recursively parse the base type
        return parseCTypeString(baseType, alias)
    }

    // Debug logging for unresolved int64_t and similar types
    if os.Getenv("ZA_DEBUG_AUTO") != "" && (typeStr == "int64_t" || typeStr == "uint64_t" || typeStr == "int32_t" || typeStr == "uint32_t") {
        fmt.Printf("[AUTO] [DEBUG] Typedef not found for %s (alias=%s)\n", typeStr, alias)
        moduleTypedefsLock.RLock()
        if aliasMap, hasAlias := moduleTypedefs[alias]; hasAlias {
            fmt.Printf("[AUTO] [DEBUG] Available typedefs for %s: %d entries\n", alias, len(aliasMap))
            // Show a sample of available typedefs
            count := 0
            for k := range aliasMap {
                if count < 5 {
                    fmt.Printf("[AUTO] [DEBUG]   - %s\n", k)
                    count++
                }
            }
            if len(aliasMap) > 5 {
                fmt.Printf("[AUTO] [DEBUG]   ... and %d more\n", len(aliasMap)-5)
            }
        } else {
            fmt.Printf("[AUTO] [DEBUG] No typedefs registered for alias %s\n", alias)
        }
        moduleTypedefsLock.RUnlock()
    }

    // Handle glibc-specific types before lowercasing
    // These are special types used in glibc headers that may not resolve via typedefs
    // On 64-bit Linux, these are typically 8-byte or 4-byte integer types
    switch typeStr {
    case "__dev_t", "__ino_t", "__ino64_t", "__off_t", "__off64_t":
        // Device number, inode number, file offset - typically 8 bytes on 64-bit
        return CUInt64, 8
    case "__mode_t", "__nlink_t", "__uid_t", "__gid_t":
        // Mode (permissions), link count, user id, group id - typically 4 bytes on 64-bit
        return CUInt, 4
    case "__time_t":
        // Time value - 8 bytes on 64-bit with __TIMESIZE=64
        return CInt64, 8
    case "__blksize_t", "__blkcnt_t", "__blkcnt64_t":
        // Block size, block count - typically 8 bytes on 64-bit
        return CInt64, 8
    case "__syscall_ulong_t", "__syscall_slong_t":
        // Syscall long types - 8 bytes on 64-bit
        return CInt64, 8
    case "__int64_t", "__int_fast64_t", "__int_least64_t":
        // 64-bit signed integer types from stdint.h
        // Fix #9: Support __int64_t and variants from system headers
        return CInt64, 8
    case "__uint64_t", "__uint_fast64_t", "__uint_least64_t":
        // 64-bit unsigned integer types from stdint.h
        // Fix #9: Support __uint64_t and variants from system headers
        return CUInt64, 8
    case "__int32_t", "__int_fast32_t", "__int_least32_t":
        // 32-bit signed integer types from stdint.h
        return CInt, 4
    case "__uint32_t", "__uint_fast32_t", "__uint_least32_t":
        // 32-bit unsigned integer types from stdint.h
        return CUInt, 4
    case "__int16_t", "__int_fast16_t", "__int_least16_t":
        // 16-bit signed integer types from stdint.h
        return CInt16, 2
    case "__uint16_t", "__uint_fast16_t", "__uint_least16_t":
        // 16-bit unsigned integer types from stdint.h
        return CUInt16, 2
    case "__int8_t", "__int_fast8_t", "__int_least8_t":
        // 8-bit signed integer types from stdint.h
        return CInt8, 1
    case "__uint8_t", "__uint_fast8_t", "__uint_least8_t":
        // 8-bit unsigned integer types from stdint.h
        return CUInt8, 1
    case "size_t":
        // size_t is typically unsigned long on 64-bit, unsigned int on 32-bit
        // Assume 64-bit for modern systems
        return CUInt64, 8
    }

    typeStr = strings.ToLower(typeStr)

    // Remove qualifiers
    typeStr = strings.ReplaceAll(typeStr, "const ", "")
    typeStr = strings.ReplaceAll(typeStr, "volatile ", "")
    typeStr = strings.ReplaceAll(typeStr, "restrict ", "")
    typeStr = strings.ReplaceAll(typeStr, "__extension__ ", "")
    typeStr = strings.TrimSpace(typeStr)

    // Map C types to CType and size
    switch typeStr {
    case "int", "signed int":
        return CInt, 4
    case "unsigned int", "unsigned":
        return CUInt, 4
    case "float":
        return CFloat, 4
    case "double":
        return CDouble, 8
    case "char", "signed char":
        return CChar, 1
    case "unsigned char":
        return CUInt8, 1
    case "short", "short int", "signed short":
        return CInt16, 2
    case "unsigned short", "unsigned short int":
        return CUInt16, 2
    case "long", "long int", "signed long":
        // On x86-64, long is 8 bytes; on x86-32, it's 4 bytes
        // We'll assume 64-bit for now (most modern systems)
        return CInt64, 8
    case "unsigned long", "unsigned long int":
        return CUInt64, 8
    case "long long", "long long int", "signed long long", "signed long long int":
        return CInt64, 8
    case "unsigned long long", "unsigned long long int":
        return CUInt64, 8
    case "long unsigned int", "long unsigned":
        // Alternative word order for "unsigned long"
        return CUInt64, 8
    case "long long unsigned int", "long long unsigned":
        // Alternative word order for "unsigned long long"
        return CUInt64, 8
    case "int8_t":
        return CInt8, 1
    case "uint8_t":
        return CUInt8, 1
    case "int16_t":
        return CInt16, 2
    case "uint16_t":
        return CUInt16, 2
    case "int32_t":
        return CInt, 4
    case "uint32_t":
        return CUInt, 4
    case "int64_t":
        return CInt64, 8
    case "uint64_t":
        return CUInt64, 8
    case "wchar_t":
        // Platform-dependent size detected at init
        if wcharSize == 2 {
            return CUInt16, 2
        } else if wcharSize == 4 {
            return CUInt, 4 // CUInt is 4 bytes, unsigned
        }
        // Fallback (shouldn't happen)
        return CVoid, 0
    default:
        // Unknown type - return CVoid to signal caller to check if it's a struct type
        // Note: typedef'd types were already checked before lowercasing
        // #define'd types (like X11's Bool) won't be recognized
        return CVoid, 0
    }
}

// extractBalancedParentheses extracts the content between balanced parentheses
// starting from the position right after an opening '('.
// It returns the content (between the parentheses), the position after the closing ')', and any error.
func extractBalancedParentheses(text string, startPos int) (content string, endPos int, err error) {
    if startPos < 0 || startPos >= len(text) {
        return "", -1, errors.New("invalid start position")
    }

    depth := 1
    for i := startPos; i < len(text); i++ {
        switch text[i] {
        case '(':
            depth++
        case ')':
            depth--
            if depth == 0 {
                content = text[startPos:i]
                endPos = i + 1
                return content, endPos, nil
            }
        }
    }

    return "", -1, errors.New("unclosed parentheses")
}

// expandDeclarationMacros expands macro calls in function declaration text
// Uses macro definitions from moduleTypedefs (populated during Phase 1)
// Recursively resolves nested macros
// Follows C preprocessor semantics: preprocessor directives in expansions are NOT re-processed
func expandDeclarationMacros(text string, alias string) string {
    result := text
    maxIterations := 500 // Allow enough iterations for large headers with many macros
    iteration := 0
    debugAuto := os.Getenv("ZA_DEBUG_AUTO") != ""

    if debugAuto {
        // Try both moduleTypedefs and moduleMacros
        moduleTypedefsLock.RLock()
        typedefCount := len(moduleTypedefs[alias])
        moduleTypedefsLock.RUnlock()

        moduleMacrosLock.RLock()
        macroCount := len(moduleMacros[alias])
        moduleMacrosLock.RUnlock()

        fmt.Fprintf(os.Stderr, "[AUTO] expandDeclarationMacros: alias=%s, typedefs=%d, macros=%d\n",
            alias, typedefCount, macroCount)
    }

    for iteration < maxIterations {
        iteration++
        changed := false

        // Pattern: detect function-like macro calls
        // MACRO_NAME(...) where MACRO_NAME starts with uppercase
        re := regexp.MustCompile(`\b([A-Z][A-Z0-9_]*)\s*\(`)

        matches := re.FindAllStringSubmatchIndex(result, -1)
        if debugAuto && iteration == 1 && strings.Contains(result, "gdImageCreateTrueColor") {
            fmt.Fprintf(os.Stderr, "[AUTO] Macro expansion iteration %d: found %d macro calls\n", iteration, len(matches))
            // Find BGD_DECLARE in the matches
            for _, m := range matches {
                macroName := result[m[2]:m[3]]
                if macroName == "BGD_DECLARE" {
                    endPos := m[1] + 30
                    if endPos > len(result) {
                        endPos = len(result)
                    }
                    fmt.Fprintf(os.Stderr, "[AUTO]   Found BGD_DECLARE at offset %d: %q\n", m[0], result[m[0]:endPos])
                }
            }
        }
        if len(matches) == 0 {
            // No function-like macros found; look for object-like macros (bare identifiers)
            // First, remove known attribute/calling convention macros that don't help with type parsing
            knownAttrs := []string{"BGD_EXPORT_DATA_PROT", "BGD_STDCALL", "WINAPI", "__stdcall", "__cdecl"}
            for _, attr := range knownAttrs {
                // Match the macro with surrounding whitespace and remove it
                reAttr := regexp.MustCompile(`\s*\b` + regexp.QuoteMeta(attr) + `\b\s*`)
                if reAttr.MatchString(result) {
                    result = reAttr.ReplaceAllString(result, " ")
                    changed = true
                }
            }

            if changed {
                continue // Continue to next iteration to process remaining macros
            }

            // Pattern: uppercase identifier surrounded by word boundaries
            reObj := regexp.MustCompile(`\b([A-Z][A-Z0-9_]*)\b`)
            objMatches := reObj.FindAllStringSubmatchIndex(result, -1)
            if len(objMatches) == 0 {
                break
            }

            // Process matches in reverse to maintain string indices
            found := false
            for i := len(objMatches) - 1; i >= 0; i-- {
                match := objMatches[i]
                macroStart := match[0]
                macroEnd := match[1]
                macroNameStart := match[2]
                macroNameEnd := match[3]

                macroName := result[macroNameStart:macroNameEnd]

                // Skip keywords that look like macros but aren't
                if len(macroName) <= 2 || strings.HasPrefix(macroName, "_") {
                    continue
                }

                // Look up macro definition
                moduleTypedefsLock.RLock()
                macroBody, exists := moduleTypedefs[alias][macroName]
                moduleTypedefsLock.RUnlock()

                // If not in moduleTypedefs, try moduleMacros
                if !exists {
                    moduleMacrosLock.RLock()
                    macroBody, exists = moduleMacros[alias][macroName]
                    moduleMacrosLock.RUnlock()
                }

                if !exists || macroBody == "" {
                    continue
                }

                // Extract the body part from function-like macro definitions
                expanded := macroBody
                if strings.Contains(macroBody, macroName+"(") {
                    // Function-like macro: extract the body after the parameter list
                    reStart := regexp.MustCompile(regexp.QuoteMeta(macroName) + `\s*\([^)]*\)\s*`)
                    matchIdxes := reStart.FindStringIndex(macroBody)
                    if matchIdxes != nil {
                        expanded = macroBody[matchIdxes[1]:]
                    }
                }

                if debugAuto {
                    fmt.Fprintf(os.Stderr, "[AUTO] Expanding object-like macro %s to: %q\n", macroName, expanded)
                }

                result = result[:macroStart] + expanded + result[macroEnd:]
                changed = true
                found = true
                break // Restart from beginning due to index changes
            }

            if !found {
                break
            }
        }

        // Process matches in reverse to maintain string indices
        for i := len(matches) - 1; i >= 0; i-- {
            match := matches[i]
            macroStart := match[0]
            macroEnd := match[1]
            macroNameStart := match[2]
            macroNameEnd := match[3]

            macroName := result[macroNameStart:macroNameEnd]

            // Skip macros that are commonly attributes/empty (on Linux)
            // These don't contribute to type information
            if macroName == "BGD_STDCALL" || macroName == "__GNUC__" ||
                macroName == "__attribute__" || macroName == "__declspec" {
                continue
            }

            // Find matching closing paren
            parenCount := 1
            j := macroEnd
            argStart := macroEnd

            for j < len(result) && parenCount > 0 {
                if result[j] == '(' {
                    parenCount++
                } else if result[j] == ')' {
                    parenCount--
                }
                j++
            }

            if parenCount != 0 {
                continue // Unmatched parens, skip
            }

            // Extract arguments
            argText := strings.TrimSpace(result[argStart : j-1])

            // Look up macro definition in moduleTypedefs or moduleMacros
            moduleTypedefsLock.RLock()
            macroBody, exists := moduleTypedefs[alias][macroName]
            moduleTypedefsLock.RUnlock()

            // If not in moduleTypedefs, try moduleMacros (function-like macros)
            if !exists {
                moduleMacrosLock.RLock()
                macroBody, exists = moduleMacros[alias][macroName]
                moduleMacrosLock.RUnlock()
            }

            if debugAuto && macroName == "BGD_DECLARE" && iteration == 1 {
                fmt.Fprintf(os.Stderr, "[AUTO] Processing BGD_DECLARE: argText=%q, exists=%v, body=%q\n",
                    argText, exists, macroBody)
            }

            if !exists || macroBody == "" {
                continue // Not a known macro or empty, skip
            }

            // Extract the body part from the macro definition
            // For function-like macros stored as "name(params) body"
            // For object-like macros stored as just "body"
            expanded := macroBody

            // Check if this is a function-like macro by looking for the macro name
            if strings.Contains(macroBody, macroName+"(") {
                // Function-like macro: extract the body after the parameter list
                // Pattern: name(params) body
                reStart := regexp.MustCompile(regexp.QuoteMeta(macroName) + `\s*\([^)]*\)\s*`)
                matches := reStart.FindStringIndex(macroBody)
                if matches != nil {
                    expanded = macroBody[matches[1]:]
                }
            }

            // Try to identify parameter names in macro body
            // Common patterns: rt, type, x, n, etc.
            // For BGD_DECLARE: the parameter is "rt" (return type)
            // Simple approach: replace bare word-boundary matches
            tokens := strings.FieldsFunc(expanded, func(r rune) bool {
                return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_'
            })

            // Replace parameter-like tokens in macro body
            // Check for single-letter parameters or common names
            for _, token := range tokens {
                if len(token) == 1 && unicode.IsLetter(rune(token[0])) && token != "r" && token != "d" && token != "i" && token != "f" && token != "s" && token != "p" && token != "v" && token != "t" && token != "m" {
                    // Single letter parameter like 'x' but avoid common letters in words
                    // Replace as whole word
                    repl := regexp.MustCompile(`\b` + regexp.QuoteMeta(token) + `\b`)
                    expanded = repl.ReplaceAllString(expanded, argText)
                } else if token == "rt" || token == "type" || token == "x" || token == "n" {
                    // Common parameter names
                    repl := regexp.MustCompile(`\b` + regexp.QuoteMeta(token) + `\b`)
                    expanded = repl.ReplaceAllString(expanded, argText)
                }
            }

            if debugAuto {
                fmt.Fprintf(os.Stderr, "[AUTO] Expanding macro %s(%s) from %q to: %q\n", macroName, argText, macroBody, expanded)
            }

            // Replace macro call with expanded result
            result = result[:macroStart] + expanded + result[j:]
            changed = true
            break // Restart from beginning due to index changes
        }

        if !changed {
            break
        }
    }

    return result
}

// parseFunctionSignatures extracts function signatures from header text
// and auto-generates LIB declarations using the existing C parser
func parseFunctionSignatures(text string, alias string) error {
    // Preprocess: expand declaration macros
    // This handles macros like BGD_DECLARE(type), resolves to actual type names
    text = expandDeclarationMacros(text, alias)

    debugAuto := os.Getenv("ZA_DEBUG_AUTO") != ""
    if debugAuto && strings.Contains(text, "gdImageCreateTrueColor") {
        idx := strings.Index(text, "gdImageCreateTrueColor")
        start := idx - 100
        if start < 0 {
            start = 0
        }
        end := idx + 50
        if end > len(text) {
            end = len(text)
        }
        fmt.Fprintf(os.Stderr, "[AUTO] After macro expansion, gdImageCreateTrueColor context: %q\n",
            text[start:end])
    }

    // Pattern to match function declarations with simple regex:
    // Find return type + function name + opening paren
    // Then use character-by-character parser to handle nested parentheses in function pointers

    // Regex pattern explanation:
    // ([a-zA-Z_][a-zA-Z0-9_ \t\*]*) = return type + function name (capture group 1)
    // \( = opening paren (starting position for parameter extraction)
    // We use a simpler regex that just finds the start of potential declarations

    re := regexp.MustCompile(`([a-zA-Z_][a-zA-Z0-9_ \t\*]*)\(`)
    matches := re.FindAllStringSubmatchIndex(text, -1)

    if len(matches) == 0 {
        // No function signatures found - not an error
        return nil
    }

    warnAuto := os.Getenv("ZA_WARN_AUTO") != ""

    if debugAuto {
        fmt.Fprintf(os.Stderr, "Found %d potential function declarations\n", len(matches))
    }

    discoveredCount := 0

    // Process each match
    for _, matchIdx := range matches {
        // matchIdx contains: [fullStart, fullEnd, group1Start, group1End]
        leftPartStart := matchIdx[2]
        leftPartEnd := matchIdx[3]
        openParenEnd := matchIdx[1]

        leftPart := strings.TrimSpace(text[leftPartStart:leftPartEnd])

        // Extract parameters using parenthesis-aware parser
        // openParenEnd points to the character after '('
        if openParenEnd >= len(text) {
            continue
        }

        params, endPos, err := extractBalancedParentheses(text, openParenEnd)
        if err != nil {
            if debugAuto {
                fmt.Fprintf(os.Stderr, "  Error extracting parameters for %s: %v\n", leftPart, err)
            }
            continue
        }

        // Check if this looks like a complete function declaration by looking for semicolon
        // Skip past any GCC attributes after the closing paren
        restOfLine := text[endPos:]
        semiIdx := strings.IndexByte(restOfLine, ';')
        if semiIdx == -1 {
            // No semicolon found on this line, might be multi-line or not a function declaration
            continue
        }

        params = strings.TrimSpace(params)

        // Skip unwanted patterns by checking if leftPart contains exclusion keywords
        if strings.Contains(leftPart, "typedef") ||
            strings.Contains(leftPart, "#define") ||
            strings.Contains(leftPart, "static") && strings.Contains(leftPart, "inline") ||
            strings.Contains(leftPart, "extern") && strings.Contains(leftPart, "inline") ||
            strings.HasPrefix(leftPart, "__attribute__") {
            continue
        }

        // Parse leftPart to extract return type and function name
        // The function name is the last identifier (word) in leftPart
        // Everything before it is the return type (which may include *)

        // Split by whitespace and * to find tokens
        // For "void *malloc", "char* strcpy", etc.
        tokens := strings.FieldsFunc(leftPart, func(r rune) bool {
            return r == ' ' || r == '\t' || r == '*'
        })

        if len(tokens) == 0 {
            continue
        }

        // Last token is the function name
        funcName := tokens[len(tokens)-1]

        // Skip internal/private functions (those starting with underscore)
        if strings.HasPrefix(funcName, "_") {
            continue
        }

        // Skip C language keywords (control flow, type keywords)
        keywordSkips := map[string]bool{
            "if": true, "else": true, "while": true, "for": true, "do": true,
            "switch": true, "case": true, "default": true, "return": true,
            "break": true, "continue": true, "sizeof": true, "define": true,
            "defined": true, "include": true, "ifdef": true, "ifndef": true,
            "endif": true, "pragma": true, "error": true,
            "warning": true, "line": true, "undef": true,
        }
        if keywordSkips[funcName] {
            if debugAuto {
                fmt.Fprintf(os.Stderr, "  Skipping keyword: %s\n", funcName)
            }
            continue
        }

        // Everything before the function name is the return type
        // We need to reconstruct it from leftPart by removing the function name
        funcNameIdx := strings.LastIndex(leftPart, funcName)
        if funcNameIdx == -1 {
            continue
        }
        returnType := strings.TrimSpace(leftPart[:funcNameIdx])

        // Reconstruct C signature format expected by parseCFunctionSignature
        signature := returnType + " " + funcName + "(" + params + ")"

        if debugAuto {
            fmt.Fprintf(os.Stderr, "[AUTO] Parsing signature: %s\n", signature)
        }

        // Call existing parser from help plugin (lib-c.go)
        sig, err := parseCFunctionSignature(signature, funcName, alias)
        if err != nil {
            // Skip unparseable signatures (function pointers, complex types)
            if warnAuto {
                msg := fmt.Sprintf("Warning: skipped unparseable signature: %s (error: %v)", signature, err)
                addMessageToCurrentProgress(msg)
            }
            continue
        }

        // Extract parameter types and struct names from parsed signature
        paramTypes := make([]CType, len(sig.Parameters))
        paramStructNames := make([]string, len(sig.Parameters))
        for i, param := range sig.Parameters {
            paramTypes[i] = param.Type
            paramStructNames[i] = param.StructTypeName
        }

        // Use return struct name from parsed signature (for unions/structs)
        returnStructName := sig.ReturnStructName

        /*
        if warnAuto {
            fmt.Fprintf(os.Stderr, "about to declare c function : %s -> %s\n",alias,funcName)
        }
        */

        // Store in global function registry
        DeclareCFunction(
            alias,
            funcName,
            paramTypes,
            paramStructNames,
            sig.ReturnType,
            returnStructName,
            sig.IsVariadic,
        )

        // Also add to library's Symbols so help plugin can display it
        if lib, exists := loadedCLibraries[alias]; exists {
            if lib.Symbols == nil {
                lib.Symbols = make(map[string]*CSymbol)
            }
            lib.Symbols[funcName] = &CSymbol{
                Name:       funcName,
                ReturnType: sig.ReturnType,
                Parameters: sig.Parameters,
                IsFunction: true,
                Library:    alias,
            }
        }

        discoveredCount++

        if debugAuto {
            fmt.Fprintf(os.Stderr, "  Auto-discovered: %s\n", signature)
            fmt.Fprintf(os.Stderr, "  Registered as: %s::%s with %d parameters\n", alias, funcName, len(paramTypes))
        }
    }

    if debugAuto && discoveredCount > 0 {
        fmt.Fprintf(os.Stderr, "Auto-discovered %d function signatures for module %s\n", discoveredCount, alias)
    }

    return nil
}
