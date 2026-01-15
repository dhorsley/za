package main

import (
    "context"
    "fmt"
    "os"
    "path/filepath"
    "regexp"
    "runtime"
    "strconv"
    "strings"
    "sync"
    "unicode"
    "unicode/utf8"
)

// Global typedef registry
// Structure: libraryAlias → typedefName → baseTypeString
// Example: "png" → "png_structp" → "struct png_struct*"
//          "c" → "size_t" → "unsigned long"
var moduleTypedefs = make(map[string]map[string]string)
var moduleTypedefsLock sync.RWMutex

// PreprocessorState tracks conditional compilation state while parsing a header
type PreprocessorState struct {
    definedMacros  map[string]string  // NAME → VALUE from #define
    conditionStack []bool              // Stack: true=include, false=skip
    chainSatisfied []bool              // Stack: true=any condition in if/elif chain was true
    includeDepth   int                 // Current #ifdef nesting level
    visitedHeaders map[string]bool     // Tracks visited headers for cycle detection
    alias          string               // Library alias for context
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
    state.definedMacros["__GNUC__"] = "4" // Claim GCC 4.x

    // Common feature test macros
    state.definedMacros["__USE_MISC"] = "1"  // Misc extensions
    state.definedMacros["__USE_XOPEN"] = "1" // X/Open compliance

    return state
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
func (s *PreprocessorState) getMacrosAsIdent() []Variable {
    ident := make([]Variable, 0, len(s.definedMacros))
    for name, value := range s.definedMacros {
        // Try to parse the value as a number
        var val any = value
        if intVal, err := strconv.ParseInt(value, 0, 64); err == nil {
            val = int(intVal)
        } else if floatVal, err := strconv.ParseFloat(value, 64); err == nil {
            val = floatVal
        }
        ident = append(ident, Variable{
            IName:    "__c_" + name,  // Prefix to avoid Za keyword conflicts
            IValue:   val,
            declared: true,
        })
    }
    return ident
}

// preprocessIfExpression replaces defined(NAME) with 1 or 0 and prefixes macro names
// to avoid conflicts with Za keywords (e.g., VERSION is a Za keyword)
// Handles: defined(NAME), defined NAME (without parens)
func (s *PreprocessorState) preprocessIfExpression(expr string) string {
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

    // Replace macro names with prefixed versions or 0 for undefined
    // Defined macros: VERSION → __c_VERSION (to avoid Za keyword conflicts)
    // Undefined macros: UNDEFINED → 0 (C preprocessor semantics)
    re3 := regexp.MustCompile(`\b([A-Z_][A-Z0-9_]*)\b`)
    expr = re3.ReplaceAllStringFunc(expr, func(match string) string {
        if s.isDefined(match) {
            // Defined macro - prefix to avoid keyword conflicts
            return "__c_" + match
        }
        // Undefined macro - replace with 0 (C preprocessor semantics)
        return "0"
    })

    return expr
}

// parseModuleHeaders finds and parses header files for a C library
func parseModuleHeaders(libraryPath string, alias string, explicitPaths []string, fs uint32) error {
    var headerPaths []string

    if len(explicitPaths) > 0 {
        // Explicit paths provided: MODULE "lib.so" AS name HEADERS "path1.h" "path2.h"
        headerPaths = explicitPaths
    } else {
        // Auto-discover: MODULE "libfoo.so" AS foo HEADERS
        headerPaths = discoverHeaders(libraryPath)
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

    for _, hpath := range headerPaths {
        if err := parseHeaderFile(hpath, alias, fs); err != nil {
            return fmt.Errorf("failed to parse %s: %w", hpath, err)
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
func parseHeaderFile(path string, alias string, fs uint32) error {
    content, err := os.ReadFile(path)
    if err != nil {
        return err
    }

    text := string(content)

    // Step 0.5: Parse preprocessor conditionals FIRST
    text = parsePreprocessor(text, alias, path, fs)

    // Step 1: Strip comments
    text = stripCComments(text)

    // Step 1.5: Normalize multiline declarations
    text = normalizeFunctionDeclarations(text)

    // Step 1.7: Parse typedefs FIRST so they're available for function parsing
    if err := parseTypedefs(text, alias); err != nil {
        if os.Getenv("ZA_WARN_AUTO") != "" {
            fmt.Fprintf(os.Stderr, "[AUTO] Warning: typedef parsing failed for %s: %v\n",
                path, err)
        }
        // Don't fail on typedef parsing errors - continue with other parsing
    }

    // Step 1.8: Parse union typedefs
    if err := parseUnionTypedefs(text, alias); err != nil {
        if os.Getenv("ZA_WARN_AUTO") != "" {
            fmt.Fprintf(os.Stderr, "[AUTO] Warning: union parsing failed for %s: %v\n",
                path, err)
        }
        // Don't fail on union parsing errors - continue with other parsing
    }

    // Step 1.9: Parse struct typedefs
    if err := parseStructTypedefs(text, alias); err != nil {
        if os.Getenv("ZA_WARN_AUTO") != "" {
            fmt.Fprintf(os.Stderr, "[AUTO] Warning: struct parsing failed for %s: %v\n",
                path, err)
        }
        // Don't fail on struct parsing errors - continue with other parsing
    }

    // Step 2: Parse #define constants
    if err := parseDefines(text, alias, fs); err != nil {
        return err
    }

    // Step 3: Parse enums
    if err := parseEnums(text, alias, fs); err != nil {
        return err
    }

    // Step 4: Parse function signatures
    if err := parseFunctionSignatures(text, alias); err != nil {
        return err
    }

    return nil
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

// parsePreprocessorWithState filters header text using an existing preprocessor state
// This allows recursive #include processing with shared state
func parsePreprocessorWithState(text string, state *PreprocessorState, currentFile string, fs uint32) string {
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
                    fmt.Printf("[AUTO] Line %d: #ifndef %s → %v (depth %d)\n",
                        lineNum+1, arg, condition, state.includeDepth)
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
                result, ok := evaluateConstant(preprocessed, &ident, fs)
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
                // Populate ident with defined macros so they can be referenced in the expression
                ident := state.getMacrosAsIdent()

                // Evaluate the expression - if it fails, treat as false
                condition := false
                result, ok := evaluateConstant(preprocessed, &ident, fs)
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
                state.popCondition()
                if debugAuto {
                    fmt.Printf("[AUTO] Line %d: #endif (depth %d)\n",
                        lineNum+1, state.includeDepth)
                }
                continue

            case "define":
                // #define NAME VALUE - Track for conditionals
                if state.isActive() {
                    parts := strings.Fields(arg)
                    if len(parts) >= 1 {
                        name := parts[0]
                        value := ""
                        if len(parts) > 1 {
                            value = strings.Join(parts[1:], " ")
                        }
                        state.definedMacros[name] = value
                        if debugAuto {
                            fmt.Printf("[AUTO] Line %d: #define %s %s\n", lineNum+1, name, value)
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

                        // Recursively process the included file
                        processedInclude := parsePreprocessorWithState(string(includeContent), state, includePath, fs)

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
        } else if debugAuto && trimmed != "" {
            fmt.Printf("[AUTO] Line %d: SKIPPED (inactive): %s\n", lineNum+1, trimmed)
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

// parseDefines extracts #define constants from header text
func parseDefines(text string, alias string, fs uint32) error {
    // Match: #define NAME VALUE
    // Support: integers, hex, floats, strings, and expressions using ev()
    // Note: Use [ \t]+ instead of \s+ to avoid matching newlines (which would match across lines)

    re := regexp.MustCompile(`(?m)^\s*#define\s+([A-Z_][A-Z0-9_]*)[ \t]+(.+)$`)
    matches := re.FindAllStringSubmatch(text, -1)

    moduleConstantsLock.Lock()
    defer moduleConstantsLock.Unlock()

    if moduleConstants[alias] == nil {
        moduleConstants[alias] = make(map[string]any)
    }

    // TODO: Future enhancement - maintain ident table to allow constants to reference earlier ones
    // Currently disabled due to variable lookup issues in ev()
    var ident []Variable

    for _, match := range matches {
        name := match[1]
        valueStr := strings.TrimSpace(match[2])

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
                    continue // Skip function-like macros
                }
            }
        }

        // Use ev() to evaluate the constant - handles all types automatically
        // Note: String concatenation ("str1" "str2") is transformed to Za syntax ("str1" + "str2")
        if val, ok := evaluateConstant(valueStr, &ident, fs); ok {
            moduleConstants[alias][name] = val

            // Add constant to ident table so later constants can reference it
            ident = append(ident, Variable{
                IName:    name,
                IValue:   val,
                declared: true,
            })
        }
        // If evaluation fails, skip silently (complex macros, undefined symbols, etc.)
    }

    return nil
}

// transformStringConcatenation converts C string concatenation to Za syntax
// Transforms: "str1" "str2" → "str1" + "str2"
func transformStringConcatenation(s string) string {
    // Pattern: quoted string followed by whitespace and another quoted string
    // Replace whitespace between strings with " + "
    re := regexp.MustCompile(`"([^"]*)"[ \t]+"`)
    for re.MatchString(s) {
        s = re.ReplaceAllString(s, `"$1" + "`)
    }
    return s
}

// evaluateConstant uses Za's expression evaluator to parse #define values
// Handles integers, floats, strings, and constant expressions automatically
// The ident parameter allows constants to reference previously-defined constants
// The fs parameter is the function space from the calling context (passed from Call())
func evaluateConstant(valueStr string, ident *[]Variable, fs uint32) (any, bool) {
    // Transform C string concatenation to Za syntax
    valueStr = transformStringConcatenation(valueStr)

    // Follow stdlib eval() pattern from lib-internal.go (lines 1023-1032)
    // Use the caller's fs (from Call() context via parser.fs)
    parser := &leparser{}
    parser.ident = ident
    parser.fs = fs
    parser.namespace = "auto_parse"
    parser.ctx = context.Background()
    parser.prectable = default_prectable

    // Pre-bind existing constants so they can be referenced
    // Clear previous bindings for this fs to avoid stale index mappings
    // (ident array order can change between evaluations due to map iteration)
    debugAuto := os.Getenv("ZA_DEBUG_AUTO") != ""
    bindlock.Lock()
    // Clear previous bindings for macro names to avoid index mismatches
    if bindings[fs] != nil {
        for name := range bindings[fs] {
            if strings.HasPrefix(name, "__c_") {
                delete(bindings[fs], name)
            }
        }
    }
    if bindings[fs] == nil {
        bindings[fs] = make(map[string]uint64)
    }
    // Bind all ident variables for this evaluation
    for i := range *ident {
        if (*ident)[i].declared {
            name := (*ident)[i].IName
            bindings[fs][name] = uint64(i)
            if debugAuto {
                fmt.Printf("[AUTO]     → Bound %s = %v (index %d) in fs=%d\n", name, (*ident)[i].IValue, i, fs)
            }
        }
    }
    bindlock.Unlock()

    if debugAuto {
        fmt.Printf("[AUTO]     → Calling ev() with expr=%q, fs=%d\n", valueStr, fs)
    }

    // Use ev() to evaluate the constant expression
    result, err := ev(parser, fs, valueStr)

    if debugAuto {
        fmt.Printf("[AUTO]     → ev() returned: result=%v, err=%v\n", result, err)
    }
    if err != nil {
        // Evaluation failed - skip this constant
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
                // Create a temporary ident table (enums don't usually reference each other across members)
                var enumIdent []Variable
                if val, ok := evaluateConstant(valueStr, &enumIdent, fs); ok {
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
                }
            } else {
                // Auto-increment
                memberName := line
                enum[fullName].members[memberName] = currentValue
                enum[fullName].ordered = append(enum[fullName].ordered, memberName)
                currentValue++
            }
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
            buffer += trimmed + " "
            if strings.Contains(trimmed, ");") || strings.Contains(trimmed, ") ;") {
                // Declaration complete
                normalized = append(normalized, buffer)
                buffer = ""
                inDeclaration = false
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

    // Pattern 1: Simple typedef
    // typedef unsigned int uint32_t;
    // typedef const char* string_t;
    reSimple := regexp.MustCompile(`typedef\s+([^;]+?)\s+(\w+)\s*;`)

    // Match and store
    for _, match := range reSimple.FindAllStringSubmatch(text, -1) {
        baseType := strings.TrimSpace(match[1])
        newName := match[2]

        // Skip function pointer typedefs (have (* in them)
        // typedef int (*callback_t)(void*);
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
                continue
            }
        }

        moduleTypedefs[alias][newName] = baseType

        if os.Getenv("ZA_DEBUG_AUTO") != "" {
            fmt.Printf("[AUTO] Typedef: %s → %s\n", newName, baseType)
        }
    }

    return nil
}

// resolveTypedef recursively resolves a typedef name to its base type
// Returns empty string if the type is not a typedef
func resolveTypedef(typeName string, alias string, depth int) string {
    // Prevent infinite recursion
    if depth > 10 {
        return ""
    }

    moduleTypedefsLock.RLock()
    defer moduleTypedefsLock.RUnlock()

    if moduleTypedefs[alias] == nil {
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

    // Check if this is a typedef
    if baseType, exists := moduleTypedefs[alias][cleanType]; exists {
        // Recursively resolve
        if resolved := resolveTypedef(baseType, alias, depth+1); resolved != "" {
            return resolved
        }
        return baseType
    }

    return ""
}

// parseUnionTypedefs extracts union typedef declarations from header text
// and stores them in the FFI struct registry with IsUnion=true
func parseUnionTypedefs(text string, alias string) error {
    // Pattern to match: typedef union { fields } name;
    // Also handles: typedef union name { fields } name;
    // Matches both multiline and single-line declarations

    re := regexp.MustCompile(`(?s)typedef\s+union\s+(?:[A-Za-z_][A-Za-z0-9_]*)?\s*\{([^}]+)\}\s*([A-Za-z_][A-Za-z0-9_]*)\s*;`)
    matches := re.FindAllStringSubmatch(text, -1)

    debugAuto := os.Getenv("ZA_DEBUG_AUTO") != ""

    for _, match := range matches {
        fieldBlock := match[1]  // Content between { }
        unionName := match[2]   // Type name after }

        if debugAuto {
            fmt.Printf("[AUTO] Found union typedef: %s\n", unionName)
        }

        // Parse fields from the field block
        fields, maxSize, err := parseUnionFields(fieldBlock, alias, debugAuto)
        if err != nil {
            if debugAuto {
                fmt.Printf("[AUTO] Warning: failed to parse union %s: %v\n", unionName, err)
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
        // Also store without namespace for easier lookup
        ffiStructDefinitions[unionName] = unionStruct
        ffiStructLock.Unlock()

        // ALSO register as typed Za struct (makes AUTO unions available in Za code)
        registerStructInZa(alias, unionName, unionStruct)

        if debugAuto {
            fmt.Printf("[AUTO] Registered union %s (size: %d bytes, %d fields)\n",
                unionName, maxSize, len(fields))
        }
    }

    return nil
}

// parseUnionFields parses the field declarations inside a union definition
// Returns the fields, the max size, and any error
func parseUnionFields(fieldBlock string, alias string, debug bool) ([]StructField, uintptr, error) {
    var fields []StructField
    var maxSize uintptr = 0

    // Split by semicolons to get individual field declarations
    declarations := strings.Split(fieldBlock, ";")

    for _, decl := range declarations {
        decl = strings.TrimSpace(decl)
        if decl == "" {
            continue
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
                size, err := strconv.Atoi(arraySizeStr)
                if err != nil {
                    if debug {
                        fmt.Printf("[AUTO] Warning: invalid array size in union field: %s\n", decl)
                    }
                    continue
                }
                arraySize = size

                // Extract type and name before [
                beforeBracket := strings.TrimSpace(decl[:openBracket])
                parts := strings.Fields(beforeBracket)
                if len(parts) < 2 {
                    if debug {
                        fmt.Printf("[AUTO] Warning: invalid array field declaration: %s\n", decl)
                    }
                    continue
                }

                // Last part is field name, everything else is type
                fieldName = parts[len(parts)-1]
                typeStr := strings.Join(parts[:len(parts)-1], " ")

                // Parse element type
                elemType, elemSize := parseCTypeString(typeStr, alias)
                if elemType == CVoid {
                    if debug {
                        fmt.Printf("[AUTO] Warning: unsupported array element type: %s\n", typeStr)
                    }
                    continue
                }

                elementType = elemType
                fieldType = elemType // For arrays, store element type
                fieldSize = elemSize * uintptr(arraySize)
            }
        } else {
            // Regular field (non-array)
            parts := strings.Fields(decl)
            if len(parts) < 2 {
                if debug {
                    fmt.Printf("[AUTO] Warning: invalid field declaration: %s\n", decl)
                }
                continue
            }

            // Last part is field name, everything else is type
            fieldName = parts[len(parts)-1]
            typeStr := strings.Join(parts[:len(parts)-1], " ")

            // Parse type
            fType, fSize := parseCTypeString(typeStr, alias)
            if fType == CVoid {
                if debug {
                    fmt.Printf("[AUTO] Warning: unsupported field type: %s\n", typeStr)
                }
                continue
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
                fmt.Printf("[AUTO]   Field: %s %s[%d] (size: %d bytes, offset: 0)\n",
                    fieldType, fieldName, arraySize, fieldSize)
            } else {
                fmt.Printf("[AUTO]   Field: %s %s (size: %d bytes, offset: 0)\n",
                    fieldType, fieldName, fieldSize)
            }
        }
    }

    return fields, maxSize, nil
}

// parseStructTypedefs extracts struct typedef declarations from header text
// and stores them in the FFI struct registry with IsUnion=false
func parseStructTypedefs(text string, alias string) error {
    // Pattern to match: typedef struct { fields } name;
    // Also handles: typedef struct name { fields } name;
    // Matches both multiline and single-line declarations

    re := regexp.MustCompile(`(?s)typedef\s+struct\s+(?:[A-Za-z_][A-Za-z0-9_]*)?\s*\{([^}]+)\}\s*([A-Za-z_][A-Za-z0-9_]*)\s*;`)
    matches := re.FindAllStringSubmatch(text, -1)

    debugAuto := os.Getenv("ZA_DEBUG_AUTO") != ""

    for _, match := range matches {
        fieldBlock := match[1]  // Content between { }
        structName := match[2]  // Type name after }

        if debugAuto {
            fmt.Printf("[AUTO] Found struct typedef: %s\n", structName)
        }

        // Parse fields from the field block
        fields, totalSize, err := parseStructFields(fieldBlock, alias, debugAuto)
        if err != nil {
            if debugAuto {
                fmt.Printf("[AUTO] Warning: failed to parse struct %s: %v\n", structName, err)
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
        // Also store without namespace for easier lookup
        ffiStructDefinitions[structName] = structDef
        ffiStructLock.Unlock()

        // ALSO register as typed Za struct (makes AUTO structs available in Za code)
        registerStructInZa(alias, structName, structDef)

        if debugAuto {
            fmt.Printf("[AUTO] Registered struct %s (size: %d bytes, %d fields)\n",
                structName, totalSize, len(fields))
        }
    }

    return nil
}

// parseStructFields parses the field declarations inside a struct definition
// Returns the fields, the total size, and any error
// Unlike unions, struct fields have sequential offsets
func parseStructFields(fieldBlock string, alias string, debug bool) ([]StructField, uintptr, error) {
    var fields []StructField
    var currentOffset uintptr = 0

    // Split by semicolons to get individual field declarations
    declarations := strings.Split(fieldBlock, ";")

    for _, decl := range declarations {
        decl = strings.TrimSpace(decl)
        if decl == "" {
            continue
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
                size, err := strconv.Atoi(arraySizeStr)
                if err != nil {
                    if debug {
                        fmt.Printf("[AUTO] Warning: invalid array size in struct field: %s\n", decl)
                    }
                    continue
                }
                arraySize = size

                // Extract type and name before [
                beforeBracket := strings.TrimSpace(decl[:openBracket])
                parts := strings.Fields(beforeBracket)
                if len(parts) < 2 {
                    if debug {
                        fmt.Printf("[AUTO] Warning: invalid array field declaration: %s\n", decl)
                    }
                    continue
                }

                // Last part is field name, everything else is type
                fieldName = parts[len(parts)-1]
                typeStr := strings.Join(parts[:len(parts)-1], " ")

                // Parse element type
                elemType, elemSize := parseCTypeString(typeStr, alias)
                if elemType == CVoid {
                    if debug {
                        fmt.Printf("[AUTO] Warning: unsupported array element type: %s\n", typeStr)
                    }
                    continue
                }

                elementType = elemType
                fieldType = elemType // For arrays, store element type
                fieldSize = elemSize * uintptr(arraySize)
            }
        } else {
            // Regular field (non-array)
            parts := strings.Fields(decl)
            if len(parts) < 2 {
                if debug {
                    fmt.Printf("[AUTO] Warning: invalid field declaration: %s\n", decl)
                }
                continue
            }

            // Last part is field name, everything else is type
            fieldName = parts[len(parts)-1]
            typeStr := strings.Join(parts[:len(parts)-1], " ")

            // Parse type
            fType, fSize := parseCTypeString(typeStr, alias)
            if fType == CVoid {
                if debug {
                    fmt.Printf("[AUTO] Warning: unsupported field type: %s\n", typeStr)
                }
                continue
            }

            fieldType = fType
            fieldSize = fSize
        }

        // Struct fields have sequential offsets (not overlapping like unions)
        field := StructField{
            Name:        fieldName,
            Type:        fieldType,
            Offset:      currentOffset,
            ArraySize:   arraySize,
            ElementType: elementType,
        }

        fields = append(fields, field)

        // Update offset for next field
        currentOffset += fieldSize

        if debug {
            if arraySize > 0 {
                fmt.Printf("[AUTO]   Field: %s %s[%d] (size: %d bytes, offset: %d)\n",
                    fieldType, fieldName, arraySize, fieldSize, field.Offset)
            } else {
                fmt.Printf("[AUTO]   Field: %s %s (size: %d bytes, offset: %d)\n",
                    fieldType, fieldName, fieldSize, field.Offset)
            }
        }
    }

    return fields, currentOffset, nil
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
    // Also register without namespace for backward compatibility
    structmaps[structName] = fields
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
    typeStr = strings.ToLower(typeStr)

    // Remove qualifiers
    typeStr = strings.ReplaceAll(typeStr, "const ", "")
    typeStr = strings.ReplaceAll(typeStr, "volatile ", "")
    typeStr = strings.ReplaceAll(typeStr, "restrict ", "")
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
    case "long long", "long long int", "signed long long":
        return CInt64, 8
    case "unsigned long long", "unsigned long long int":
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
    default:
        // Unknown type
        return CVoid, 0
    }
}

// parseFunctionSignatures extracts function signatures from header text
// and auto-generates LIB declarations using the existing C parser
func parseFunctionSignatures(text string, alias string) error {
    // Pattern to match function declarations:
    // Captures everything before '(' and the parameters
    // We'll parse the left part to extract return type and function name

    // Regex pattern explanation:
    // ([a-zA-Z_][a-zA-Z0-9_\s\*]*) = return type + function name (capture group 1)
    // \( = opening paren
    // ([^)]*) = parameters (capture group 2)
    // \)\s*; = closing paren + semicolon
    // Note: removed ^ anchor since preprocessor may collapse lines

    re := regexp.MustCompile(`([a-zA-Z_][a-zA-Z0-9_\s\*]*)\(([^)]*)\)\s*;`)
    matches := re.FindAllStringSubmatch(text, -1)

    if len(matches) == 0 {
        // No function signatures found - not an error
        return nil
    }

    debugAuto := os.Getenv("ZA_DEBUG_AUTO") != ""
    warnAuto := os.Getenv("ZA_WARN_AUTO") != ""

    if debugAuto {
        fmt.Fprintf(os.Stderr, "Found %d potential function declarations\n", len(matches))
        for i, match := range matches {
            fmt.Fprintf(os.Stderr, "  Match %d: %s(%s)\n", i+1, match[1], match[2])
        }
    }
    discoveredCount := 0

    for _, match := range matches {
        leftPart := strings.TrimSpace(match[1])
        params := strings.TrimSpace(match[2])

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
                fmt.Fprintf(os.Stderr, "Warning: skipped unparseable signature: %s (error: %v)\n", signature, err)
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

        discoveredCount++

        if debugAuto {
            fmt.Fprintf(os.Stderr, "  Auto-discovered: %s\n", signature)
        }
    }

    if debugAuto && discoveredCount > 0 {
        fmt.Fprintf(os.Stderr, "Auto-discovered %d function signatures for module %s\n", discoveredCount, alias)
    }

    return nil
}
