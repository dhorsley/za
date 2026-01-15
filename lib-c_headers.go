package main

import (
    "context"
    "fmt"
    "os"
    "path/filepath"
    "regexp"
    "runtime"
    "strings"
    "sync"
)

// Global typedef registry
// Structure: libraryAlias → typedefName → baseTypeString
// Example: "png" → "png_structp" → "struct png_struct*"
//          "c" → "size_t" → "unsigned long"
var moduleTypedefs = make(map[string]map[string]string)
var moduleTypedefsLock sync.RWMutex

// PreprocessorState tracks conditional compilation state while parsing a header
type PreprocessorState struct {
    definedMacros  map[string]string // NAME → VALUE from #define
    conditionStack []bool             // Stack: true=include, false=skip
    includeDepth   int                // Current #ifdef nesting level
}

// newPreprocessorState creates a new preprocessor state with platform macros
func newPreprocessorState() *PreprocessorState {
    state := &PreprocessorState{
        definedMacros:  make(map[string]string),
        conditionStack: []bool{true}, // Start with including (top level)
        includeDepth:   0,
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
func (s *PreprocessorState) pushCondition(include bool) {
    s.conditionStack = append(s.conditionStack, include)
    s.includeDepth++
}

// popCondition removes the top conditional level
func (s *PreprocessorState) popCondition() {
    if len(s.conditionStack) > 1 { // Keep base level
        s.conditionStack = s.conditionStack[:len(s.conditionStack)-1]
        s.includeDepth--
    }
}

// toggleCondition flips the current condition (#else handling)
func (s *PreprocessorState) toggleCondition() {
    if len(s.conditionStack) > 0 {
        s.conditionStack[len(s.conditionStack)-1] = !s.conditionStack[len(s.conditionStack)-1]
    }
}

// parseModuleHeaders finds and parses header files for a C library
func parseModuleHeaders(libraryPath string, alias string, explicitPaths []string) error {
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
        if err := parseHeaderFile(hpath, alias); err != nil {
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
func parseHeaderFile(path string, alias string) error {
    content, err := os.ReadFile(path)
    if err != nil {
        return err
    }

    text := string(content)

    // Step 0.5: Parse preprocessor conditionals FIRST
    text = parsePreprocessor(text, alias)

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

    // Step 2: Parse #define constants
    if err := parseDefines(text, alias); err != nil {
        return err
    }

    // Step 3: Parse enums
    if err := parseEnums(text, alias); err != nil {
        return err
    }

    // Step 4: Parse function signatures
    if err := parseFunctionSignatures(text, alias); err != nil {
        return err
    }

    return nil
}

// parsePreprocessor filters header text based on preprocessor conditionals
// Returns the filtered text with only active blocks included
func parsePreprocessor(text string, alias string) string {
    state := newPreprocessorState()
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
func parseDefines(text string, alias string) error {
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
        if val, ok := evaluateConstant(valueStr, &ident); ok {
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
func evaluateConstant(valueStr string, ident *[]Variable) (any, bool) {
    // Transform C string concatenation to Za syntax
    valueStr = transformStringConcatenation(valueStr)

    // Follow stdlib eval() pattern from lib-internal.go (lines 985-994)
    parser := &leparser{}
    parser.ident = ident
    parser.fs = 0
    parser.namespace = "auto_parse"
    parser.ctx = context.Background()
    parser.prectable = default_prectable

    // Pre-bind existing constants so they can be referenced
    // Only bind constants that are already declared (from earlier #defines)
    bindlock.Lock()
    for i := range *ident {
        if (*ident)[i].declared {
            name := (*ident)[i].IName
            // Only add to bindings if not already present
            if bindings[0] == nil {
                bindings[0] = make(map[string]uint64)
            }
            if _, exists := bindings[0][name]; !exists {
                bindings[0][name] = uint64(i)
            }
        }
    }
    bindlock.Unlock()

    // Use ev() to evaluate the constant expression
    result, err := ev(parser, 0, valueStr)
    if err != nil {
        // Evaluation failed - skip this constant
        return nil, false
    }

    return result, true
}

// parseEnums extracts enum definitions from header text
func parseEnums(text string, alias string) error {
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
                if val, ok := evaluateConstant(valueStr, &enumIdent); ok {
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
        if strings.Contains(baseType, "struct") || strings.Contains(baseType, "union") {
            // Count braces to detect nested structs
            openBraces := strings.Count(baseType, "{")
            closeBraces := strings.Count(baseType, "}")
            if openBraces > 1 || closeBraces > 1 {
                // Nested struct/union - skip for now
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

// parseFunctionSignatures extracts function signatures from header text
// and auto-generates LIB declarations using the existing C parser
func parseFunctionSignatures(text string, alias string) error {
    // Pattern to match function declarations:
    // Captures everything before '(' and the parameters
    // We'll parse the left part to extract return type and function name

    // Regex pattern explanation:
    // (?m) = multiline mode
    // ^[\t ]* = optional leading whitespace
    // ([^(]+) = everything before opening paren (capture group 1)
    // \( = opening paren
    // ([^)]*) = parameters (capture group 2)
    // \)\s*; = closing paren + semicolon

    re := regexp.MustCompile(`(?m)^[\t ]*([^(]+)\(([^)]*)\)\s*;`)
    matches := re.FindAllStringSubmatch(text, -1)

    if len(matches) == 0 {
        // No function signatures found - not an error
        return nil
    }

    debugAuto := os.Getenv("ZA_DEBUG_AUTO") != ""
    warnAuto := os.Getenv("ZA_WARN_AUTO") != ""
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

        // For return type struct name, check if return type is CStruct
        // For now, we don't have this info from header parsing, so leave empty
        returnStructName := ""

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
